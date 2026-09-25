package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/wgmesh/wgmesh/internal/config"
)

func (m *Model) isRouteNameTaken(name string, excludeIdx int) bool {
	for i, r := range m.Mesh.Routes {
		if i != excludeIdx && strings.EqualFold(r.Name, name) {
			return true
		}
	}
	return false
}

func (m *Model) nextDefaultRouteName() string {
	for i := 1; i <= 99; i++ {
		name := fmt.Sprintf("route_%02d", i)
		if !m.isRouteNameTaken(name, -1) {
			return name
		}
	}
	return "route_99"
}

func (m *Model) addHopToSelectedRoute() {
	if len(m.Mesh.Nodes) == 0 {
		m.LogMsg = "[INFO] Для создания маршрута сначала добавьте хотя бы один сервер/роутер (нажмите 'b' для Bootstrap или 'n')"
		return
	}
	if len(m.Mesh.Routes) > 0 && m.SelectedRoute < len(m.Mesh.Routes) {
		r := &m.Mesh.Routes[m.SelectedRoute]
		if r.Protected {
			m.LogMsg = fmt.Sprintf("[!] Маршрут %q защищён (protected: true) - редактирование запрещено", r.Name)
			return
		}
		nodeToAdd := m.Mesh.Nodes[0].Name
		r.Path = append(r.Path, nodeToAdd)
		r.ExitNode = nodeToAdd
		m.IsDirty = true
		m.LogMsg = fmt.Sprintf("[OK] Добавлен хоп %s в маршрут %s", nodeToAdd, r.Name)
	} else {
		m.openAddRouteModal()
	}
}

func (m *Model) removeHopFromSelectedRoute() {
	if len(m.Mesh.Routes) > 0 && m.SelectedRoute < len(m.Mesh.Routes) {
		r := &m.Mesh.Routes[m.SelectedRoute]
		if r.Protected {
			m.LogMsg = fmt.Sprintf("[!] Маршрут %q защищён (protected: true) - редактирование запрещено", r.Name)
			return
		}
		if len(r.Path) > 2 {
			removed := r.Path[len(r.Path)-1]
			r.Path = r.Path[:len(r.Path)-1]
			r.ExitNode = r.Path[len(r.Path)-1]
			m.IsDirty = true
			m.LogMsg = fmt.Sprintf("[OK] Удалён хоп %s из маршрута %s", removed, r.Name)
		}
	}
}

func (m *Model) cycleSelectedHopNode(delta int) {
	if len(m.Mesh.Nodes) == 0 || len(m.Mesh.Routes) == 0 || m.SelectedRoute >= len(m.Mesh.Routes) {
		return
	}
	r := &m.Mesh.Routes[m.SelectedRoute]
	if r.Protected {
		m.LogMsg = fmt.Sprintf("[!] Маршрут %q защищён (protected: true)", r.Name)
		return
	}
	hopIdx := m.Editor.SelectedHop
	if hopIdx <= 0 || hopIdx >= len(r.Path) {
		return
	}

	nodeNames := m.nodeNamesList()
	currNode := r.Path[hopIdx]
	currOpt := 0
	for i, name := range nodeNames {
		if name == currNode {
			currOpt = i
			break
		}
	}

	newOpt := (currOpt + delta + len(nodeNames)) % len(nodeNames)
	r.Path[hopIdx] = nodeNames[newOpt]
	if hopIdx == len(r.Path)-1 {
		r.ExitNode = nodeNames[newOpt]
	}
	m.IsDirty = true
	_ = config.Save(m.ConfigPath, m.Mesh)
	m.LogMsg = fmt.Sprintf("[OK] Узел хопа %d изменён на %s", hopIdx, nodeNames[newOpt])
}

func (m *Model) openAddRouteModal() tea.Cmd {
	if len(m.Mesh.Nodes) == 0 {
		m.LogMsg = "[!] Для создания маршрута сначала добавьте хотя бы один узел (нажмите 'b' для Bootstrap или 'n')"
		return nil
	}
	nodeNames := m.nodeNamesList()
	return m.openModal(ModalState{
		Type: ModalAddRoute,
		Fields: []FormField{
			{Label: "Имя маршрута", Value: m.nextDefaultRouteName()},
			{Label: "Узел 1 (Hop 1)", Options: nodeNames, OptionIdx: 0, Value: nodeNames[0]},
			{Label: "[+] Добавить хоп в цепочку", Value: "(Enter чтобы вставить узел)"},
			{Label: "Выходной узел (Exit)", Options: nodeNames, OptionIdx: 0, Value: nodeNames[0]},
			{Label: "Защита маршрута **", Options: []string{"нет", "да (protected: true)"}, OptionIdx: 0, Value: "нет"},
		},
	})
}

func (m *Model) openEditRouteModal() tea.Cmd {
	if len(m.Mesh.Routes) == 0 || m.SelectedRoute < 0 || m.SelectedRoute >= len(m.Mesh.Routes) {
		return nil
	}
	r := &m.Mesh.Routes[m.SelectedRoute]
	if r.Protected && m.Modal.Type != ModalConfirm {
		return m.openModal(ModalState{
			Type:          ModalConfirm,
			ConfirmPrompt: fmt.Sprintf("Маршрут %q помечен как защищённый (protected: true)! Вы действительно хотите его отредактировать?", r.Name),
			OnConfirm: func(mod *Model) {
				mod.showEditRouteForm()
			},
		})
	}
	return m.showEditRouteForm()
}

func (m *Model) showEditRouteForm() tea.Cmd {
	if len(m.Mesh.Routes) == 0 || m.SelectedRoute < 0 || m.SelectedRoute >= len(m.Mesh.Routes) {
		return nil
	}
	r := &m.Mesh.Routes[m.SelectedRoute]
	nodeNames := m.nodeNamesList()

	hops := r.Hops()
	if len(hops) == 0 {
		hops = []string{nodeNames[0]}
	}

	var fields []FormField
	fields = append(fields, FormField{Label: "Имя маршрута", Value: r.Name})

	for i, hName := range hops {
		idx := 0
		for k, nName := range nodeNames {
			if nName == hName {
				idx = k
				break
			}
		}
		fields = append(fields, FormField{
			Label:     fmt.Sprintf("Узел %d (Hop %d)", i+1, i+1),
			Options:   nodeNames,
			OptionIdx: idx,
			Value:     nodeNames[idx],
		})
	}

	fields = append(fields, FormField{Label: "[+] Добавить хоп в цепочку", Value: "(Enter чтобы вставить узел)"})
	if len(hops) > 1 {
		fields = append(fields, FormField{Label: "[-] Удалить последний хоп", Value: "(Enter чтобы удалить)"})
	}

	exitOpt := 0
	for k, nName := range nodeNames {
		if nName == r.ExitNode {
			exitOpt = k
			break
		}
	}

	protOpt := 0
	if r.Protected {
		protOpt = 1
	}

	fields = append(fields, FormField{Label: "Выходной узел (Exit)", Options: nodeNames, OptionIdx: exitOpt, Value: nodeNames[exitOpt]})
	fields = append(fields, FormField{Label: "Защита маршрута **", Options: []string{"нет", "да (protected: true)"}, OptionIdx: protOpt, Value: []string{"нет", "да (protected: true)"}[protOpt]})

	return m.openModal(ModalState{
		Type:   ModalEditRoute,
		Fields: fields,
	})
}

func (m *Model) addHopFieldToRouteModal() {
	nodeNames := m.nodeNamesList()

	var currentHops []string
	insertIdx := -1
	for i, f := range m.Modal.Fields {
		if strings.HasPrefix(f.Label, "Узел ") {
			if len(f.Options) > 0 && f.OptionIdx >= 0 && f.OptionIdx < len(f.Options) {
				currentHops = append(currentHops, f.Options[f.OptionIdx])
			}
		} else if strings.Contains(f.Label, "[+] Добавить хоп") {
			insertIdx = i
		}
	}

	if insertIdx == -1 {
		return
	}

	newHopNum := len(currentHops) + 1
	newHopField := FormField{
		Label:     fmt.Sprintf("Узел %d (Hop %d)", newHopNum, newHopNum),
		Options:   nodeNames,
		OptionIdx: 0,
		Value:     nodeNames[0],
	}

	newFields := make([]FormField, 0, len(m.Modal.Fields)+1)
	newFields = append(newFields, m.Modal.Fields[:insertIdx]...)
	newFields = append(newFields, newHopField)
	newFields = append(newFields, m.Modal.Fields[insertIdx:]...)

	hasDeleteBtn := false
	for _, f := range newFields {
		if strings.Contains(f.Label, "[-] Удалить") {
			hasDeleteBtn = true
			break
		}
	}

	if !hasDeleteBtn {
		for i, f := range newFields {
			if strings.Contains(f.Label, "[+] Добавить хоп") {
				delBtn := FormField{Label: "[-] Удалить последний хоп", Value: "(Enter чтобы удалить)"}
				after := make([]FormField, 0, len(newFields)+1)
				after = append(after, newFields[:i+1]...)
				after = append(after, delBtn)
				after = append(after, newFields[i+1:]...)
				newFields = after
				break
			}
		}
	}

	for i := range newFields {
		if strings.HasPrefix(newFields[i].Label, "Выходной узел") {
			newFields[i].OptionIdx = 0
			newFields[i].Value = nodeNames[0]
		}
	}

	m.Modal.Fields = newFields
	m.Modal.ActiveField = insertIdx
}

func (m *Model) removeHopFieldFromRouteModal() {
	var hopIndices []int
	for i, f := range m.Modal.Fields {
		if strings.HasPrefix(f.Label, "Узел ") {
			hopIndices = append(hopIndices, i)
		}
	}

	if len(hopIndices) <= 1 {
		return
	}

	lastHopIdx := hopIndices[len(hopIndices)-1]
	newFields := append(m.Modal.Fields[:lastHopIdx], m.Modal.Fields[lastHopIdx+1:]...)

	remainingHopCount := len(hopIndices) - 1
	if remainingHopCount <= 1 {
		for i, f := range newFields {
			if strings.Contains(f.Label, "[-] Удалить") {
				newFields = append(newFields[:i], newFields[i+1:]...)
				break
			}
		}
	}

	var lastNodeName string
	nodeNames := m.nodeNamesList()
	for _, f := range newFields {
		if strings.HasPrefix(f.Label, "Узел ") {
			if len(f.Options) > 0 && f.OptionIdx >= 0 && f.OptionIdx < len(f.Options) {
				lastNodeName = f.Options[f.OptionIdx]
			}
		}
	}

	if lastNodeName != "" {
		for i := range newFields {
			if strings.HasPrefix(newFields[i].Label, "Выходной узел") {
				for k, nName := range nodeNames {
					if nName == lastNodeName {
						newFields[i].OptionIdx = k
						newFields[i].Value = nodeNames[k]
						break
					}
				}
			}
		}
	}

	m.Modal.Fields = newFields
	if m.Modal.ActiveField >= len(m.Modal.Fields) {
		m.Modal.ActiveField = len(m.Modal.Fields) - 1
	}
}

func (m *Model) removeRouteByIdx(idx int) {
	if idx < len(m.Mesh.Routes) {
		name := m.Mesh.Routes[idx].Name
		m.Mesh.Routes = append(m.Mesh.Routes[:idx], m.Mesh.Routes[idx+1:]...)
		m.IsDirty = true
		_ = config.Save(m.ConfigPath, m.Mesh)
		m.LogMsg = fmt.Sprintf("[OK] Маршрут %q удалён", name)
		if m.SelectedRoute > 0 {
			m.SelectedRoute--
		}
	}
}

