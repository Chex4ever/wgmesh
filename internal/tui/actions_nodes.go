package tui

import (
	"fmt"
	"strings"

	"github.com/meshctl/meshctl/internal/config"
	"github.com/meshctl/meshctl/internal/drivers"
)

func (m *Model) isNodeNameTaken(name string, excludeIdx int) bool {
	for i, n := range m.Mesh.Nodes {
		if i != excludeIdx && strings.EqualFold(n.Name, name) {
			return true
		}
	}
	return false
}

func (m *Model) nextDefaultNodeName() string {
	for i := 1; i <= 99; i++ {
		name := fmt.Sprintf("node_%02d", i)
		if !m.isNodeNameTaken(name, -1) {
			return name
		}
	}
	return "node_99"
}

func (m *Model) nodeNamesList() []string {
	var names []string
	for _, n := range m.Mesh.Nodes {
		names = append(names, n.Name)
	}
	if len(names) == 0 {
		names = []string{"(сначала добавьте ноду)"}
	}
	return names
}

func (m *Model) renameNode(oldName, newName string) {
	if oldName == "" || newName == "" || oldName == newName {
		return
	}
	for i := range m.Mesh.Routes {
		r := &m.Mesh.Routes[i]
		for j := range r.Path {
			if r.Path[j] == oldName {
				r.Path[j] = newName
			}
		}
		if r.ExitNode == oldName {
			r.ExitNode = newName
		}
	}
	for i := range m.Mesh.Clients {
		c := &m.Mesh.Clients[i]
		if c.Ingress == oldName {
			c.Ingress = newName
		}
	}
}

func (m *Model) openBootstrapModal() {
	m.Modal = ModalState{
		Type: ModalBootstrapNode,
		Fields: []FormField{
			{Label: "Имя ноды", Value: m.nextDefaultNodeName()},
			{Label: "IP / Хост ноды", Placeholder: "109.248.198.55"},
			{Label: "Пароль SSH *", Mask: true},
			{Label: "Тип платформы", Options: []string{"linux", "mikrotik", "openwrt"}, OptionIdx: 0, Value: "linux"},
			{Label: "SSH Пользователь", Value: "root"},
			{Label: "Защита от удаления **", Options: []string{"нет", "да (protected: true)"}, OptionIdx: 0, Value: "нет"},
		},
	}
}

func (m *Model) openAddNodeModal() {
	m.Modal = ModalState{
		Type: ModalAddNode,
		Fields: []FormField{
			{Label: "Имя ноды", Value: m.nextDefaultNodeName()},
			{Label: "IP / Хост ноды", Placeholder: "194.87.71.7"},
			{Label: "Пароль SSH *", Mask: true},
			{Label: "Тип платформы", Options: []string{"linux", "mikrotik", "openwrt"}, OptionIdx: 0, Value: "linux"},
			{Label: "SSH Пользователь", Value: "root"},
			{Label: "Защита от удаления **", Options: []string{"нет", "да (protected: true)"}, OptionIdx: 0, Value: "нет"},
		},
	}
}

func (m *Model) openEditNodeModal() {
	if len(m.Mesh.Nodes) == 0 || m.SelectedNode < 0 || m.SelectedNode >= len(m.Mesh.Nodes) {
		return
	}
	node := &m.Mesh.Nodes[m.SelectedNode]
	typeOpt := 0
	switch node.Type {
	case config.TypeMikrotik:
		typeOpt = 1
	case config.TypeOpenWRT:
		typeOpt = 2
	}
	protOpt := 0
	if node.Protected {
		protOpt = 1
	}

	m.Modal = ModalState{
		Type: ModalEditNode,
		Fields: []FormField{
			{Label: "Имя ноды", Value: node.Name},
			{Label: "IP / Хост ноды", Value: node.Host},
			{Label: "Пароль SSH *", Mask: true},
			{Label: "Тип платформы", Options: []string{"linux", "mikrotik", "openwrt"}, OptionIdx: typeOpt, Value: node.Type},
			{Label: "SSH Пользователь", Value: node.SSHUser},
			{Label: "Защита от удаления **", Options: []string{"нет", "да (protected: true)"}, OptionIdx: protOpt, Value: []string{"нет", "да (protected: true)"}[protOpt]},
		},
	}
}

func (m *Model) removeNodeByIdx(idx int) {
	if idx < len(m.Mesh.Nodes) {
		name := m.Mesh.Nodes[idx].Name
		m.Mesh.Nodes = append(m.Mesh.Nodes[:idx], m.Mesh.Nodes[idx+1:]...)
		m.IsDirty = true
		_ = config.Save(m.ConfigPath, m.Mesh)
		m.LogMsg = fmt.Sprintf("[OK] Нода %q удалена", name)
		if m.SelectedNode > 0 {
			m.SelectedNode--
		}
	}
}

func (m *Model) handleTeardownNode() {
	if len(m.Mesh.Nodes) == 0 || m.SelectedNode >= len(m.Mesh.Nodes) {
		return
	}
	node := &m.Mesh.Nodes[m.SelectedNode]
	m.LogMsg = fmt.Sprintf("-> Удаляю WG-конфигурацию с %q (%s)...", node.Name, node.Host)
	d, err := drivers.New(node)
	if err != nil {
		m.LogMsg = fmt.Sprintf("Ошибка драйвера: %v", err)
		return
	}
	if err := d.RemoveConfig(node); err != nil {
		m.LogMsg = fmt.Sprintf("Ошибка teardown: %v", err)
	} else {
		m.LogMsg = fmt.Sprintf("[OK] WG конфигурация с %q удалена!", node.Name)
	}
}

func (m *Model) handleCapabilities() {
	if len(m.Mesh.Nodes) == 0 || m.SelectedNode >= len(m.Mesh.Nodes) {
		return
	}
	node := &m.Mesh.Nodes[m.SelectedNode]
	caps := drivers.Capabilities(node.Type)
	m.Modal = ModalState{
		Type: ModalCapabilities,
		CapText: fmt.Sprintf(
			"Нода: %s (%s)\n\n"+
				"  - PSK Support:         %v\n"+
				"  - Keepalive Support:   %v\n"+
				"  - NAT/Masquerade:      %v\n"+
				"  - Multi-Route:         %v\n"+
				"  - AmneziaWG Obfusc:    %v\n"+
				"  - Auto Package Inst:   %v",
			node.Name, node.Type,
			caps.SupportsPSK, caps.SupportsKeepalive, caps.SupportsNAT,
			caps.SupportsMultiRoute, caps.SupportsObfuscation, caps.NeedsPackageInstall,
		),
	}
}
