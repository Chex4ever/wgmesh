package tui

import (
	"fmt"
	"strings"

	"github.com/meshctl/meshctl/internal/config"
)

func (m *Model) isClientNameTaken(name string, excludeIdx int) bool {
	for i, c := range m.Mesh.Clients {
		if i != excludeIdx && strings.EqualFold(c.Name, name) {
			return true
		}
	}
	return false
}

func (m *Model) nextDefaultClientName() string {
	for i := 1; i <= 99; i++ {
		name := fmt.Sprintf("client_%02d", i)
		if !m.isClientNameTaken(name, -1) {
			return name
		}
	}
	return "client_99"
}

func (m *Model) openAddClientModal() {
	nodeNames := m.nodeNamesList()
	m.Modal = ModalState{
		Type: ModalAddClient,
		Fields: []FormField{
			{Label: "Имя клиента (устройства)", Value: m.nextDefaultClientName()},
			{Label: "Нода подключения (Ingress)", Options: nodeNames, OptionIdx: 0, Value: nodeNames[0]},
		},
	}
}

func (m *Model) openEditClientModal() {
	if len(m.Mesh.Clients) == 0 || m.SelectedClient < 0 || m.SelectedClient >= len(m.Mesh.Clients) {
		return
	}
	c := &m.Mesh.Clients[m.SelectedClient]
	nodeNames := m.nodeNamesList()
	ingOpt := 0
	for i, name := range nodeNames {
		if name == c.Ingress {
			ingOpt = i
			break
		}
	}
	m.Modal = ModalState{
		Type: ModalEditClient,
		Fields: []FormField{
			{Label: "Имя клиента (устройства)", Value: c.Name},
			{Label: "Нода подключения (Ingress)", Options: nodeNames, OptionIdx: ingOpt, Value: nodeNames[ingOpt]},
		},
	}
}

func (m *Model) isListNameTaken(name string, excludeIdx int) bool {
	for i, l := range m.Mesh.Lists {
		if i != excludeIdx && strings.EqualFold(l.Name, name) {
			return true
		}
	}
	return false
}

func (m *Model) nextDefaultListName() string {
	for i := 1; i <= 99; i++ {
		name := fmt.Sprintf("list_%02d", i)
		if !m.isListNameTaken(name, -1) {
			return name
		}
	}
	return "list_99"
}

func (m *Model) openAddListModal() {
	m.Modal = ModalState{
		Type: ModalAddList,
		Fields: []FormField{
			{Label: "Имя списка", Value: m.nextDefaultListName()},
			{Label: "Домены (через запятую)", Placeholder: "youtube.com, *.googlevideo.com"},
		},
	}
}

func (m *Model) openEditListModal() {
	if len(m.Mesh.Lists) == 0 || m.SelectedList < 0 || m.SelectedList >= len(m.Mesh.Lists) {
		return
	}
	l := &m.Mesh.Lists[m.SelectedList]
	m.Modal = ModalState{
		Type: ModalEditList,
		Fields: []FormField{
			{Label: "Имя списка", Value: l.Name},
			{Label: "Домены (через запятую)", Value: strings.Join(l.Domains, ", ")},
		},
	}
}

func (m *Model) handleDeleteCurrent() {
	switch m.ActivePane {
	case PaneNodes:
		if len(m.Mesh.Nodes) == 0 || m.SelectedNode >= len(m.Mesh.Nodes) {
			return
		}
		node := &m.Mesh.Nodes[m.SelectedNode]
		idx := m.SelectedNode
		if node.Protected {
			m.Modal = ModalState{
				Type:          ModalConfirm,
				ConfirmPrompt: fmt.Sprintf("Нода %q помечена как защищённая (protected: true)! Вы действительно хотите её удалить?", node.Name),
				OnConfirm: func(mod *Model) {
					mod.removeNodeByIdx(idx)
				},
			}
			return
		}
		m.removeNodeByIdx(idx)

	case PaneRoutes:
		if len(m.Mesh.Routes) == 0 || m.SelectedRoute >= len(m.Mesh.Routes) {
			return
		}
		r := &m.Mesh.Routes[m.SelectedRoute]
		idx := m.SelectedRoute
		if r.Protected {
			m.Modal = ModalState{
				Type:          ModalConfirm,
				ConfirmPrompt: fmt.Sprintf("Маршрут %q помечен как защищённый (protected: true)! Вы действительно хотите его удалить?", r.Name),
				OnConfirm: func(mod *Model) {
					mod.removeRouteByIdx(idx)
				},
			}
			return
		}
		m.removeRouteByIdx(idx)

	case PaneClients:
		if len(m.Mesh.Clients) > 0 && m.SelectedClient < len(m.Mesh.Clients) {
			cName := m.Mesh.Clients[m.SelectedClient].Name
			m.Mesh.Clients = append(m.Mesh.Clients[:m.SelectedClient], m.Mesh.Clients[m.SelectedClient+1:]...)
			m.IsDirty = true
			_ = config.Save(m.ConfigPath, m.Mesh)
			m.LogMsg = fmt.Sprintf("[OK] Клиент %q удалён", cName)
		}

	case PaneLists:
		if len(m.Mesh.Lists) > 0 && m.SelectedList < len(m.Mesh.Lists) {
			lName := m.Mesh.Lists[m.SelectedList].Name
			m.Mesh.Lists = append(m.Mesh.Lists[:m.SelectedList], m.Mesh.Lists[m.SelectedList+1:]...)
			m.IsDirty = true
			_ = config.Save(m.ConfigPath, m.Mesh)
			m.LogMsg = fmt.Sprintf("[OK] Список %q удалён", lName)
		}
	}
}
