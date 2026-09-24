package tui

import (
	"fmt"
	"strings"

	"github.com/meshctl/meshctl/internal/config"
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

func (m *Model) openAddRouteModal() {
	if len(m.Mesh.Nodes) == 0 {
		m.LogMsg = "[!] Для создания маршрута сначала добавьте хотя бы один узел (нажмите 'b' для Bootstrap или 'n')"
		return
	}
	nodeNames := m.nodeNamesList()
	defaultPath := "client," + nodeNames[0]
	m.Modal = ModalState{
		Type: ModalAddRoute,
		Fields: []FormField{
			{Label: "Имя маршрута", Value: m.nextDefaultRouteName()},
			{Label: "Путь (через запятую)", Value: defaultPath},
			{Label: "Выходной сервер (Exit)", Options: nodeNames, OptionIdx: 0, Value: nodeNames[0]},
			{Label: "Защита маршрута **", Options: []string{"нет", "да (protected: true)"}, OptionIdx: 0, Value: "нет"},
		},
	}
}

func (m *Model) openEditRouteModal() {
	if len(m.Mesh.Routes) == 0 || m.SelectedRoute < 0 || m.SelectedRoute >= len(m.Mesh.Routes) {
		return
	}
	r := &m.Mesh.Routes[m.SelectedRoute]
	if r.Protected && m.Modal.Type != ModalConfirm {
		m.Modal = ModalState{
			Type:          ModalConfirm,
			ConfirmPrompt: fmt.Sprintf("Маршрут %q помечен как защищённый (protected: true)! Вы действительно хотите его отредактировать?", r.Name),
			OnConfirm: func(mod *Model) {
				mod.showEditRouteForm()
			},
		}
		return
	}
	m.showEditRouteForm()
}

func (m *Model) showEditRouteForm() {
	if len(m.Mesh.Routes) == 0 || m.SelectedRoute < 0 || m.SelectedRoute >= len(m.Mesh.Routes) {
		return
	}
	r := &m.Mesh.Routes[m.SelectedRoute]
	nodeNames := m.nodeNamesList()
	exitOpt := 0
	for i, name := range nodeNames {
		if name == r.ExitNode {
			exitOpt = i
			break
		}
	}
	protOpt := 0
	if r.Protected {
		protOpt = 1
	}
	m.Modal = ModalState{
		Type: ModalEditRoute,
		Fields: []FormField{
			{Label: "Имя маршрута", Value: r.Name},
			{Label: "Путь (через запятую)", Value: strings.Join(r.Path, ", ")},
			{Label: "Выходной сервер (Exit)", Options: nodeNames, OptionIdx: exitOpt, Value: nodeNames[exitOpt]},
			{Label: "Защита маршрута **", Options: []string{"нет", "да (protected: true)"}, OptionIdx: protOpt, Value: []string{"нет", "да (protected: true)"}[protOpt]},
		},
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
