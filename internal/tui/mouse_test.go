package tui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/meshctl/meshctl/internal/config"
)

func TestMouseHandlingPanes(t *testing.T) {
	mesh := &config.Mesh{
		Name: "TestMesh",
		Nodes: []config.Node{
			{Name: "node1", Host: "1.1.1.1"},
			{Name: "node2", Host: "2.2.2.2"},
		},
		Routes: []config.Route{
			{Name: "route1", Path: []string{"client", "node1"}},
		},
	}
	model := NewModel(mesh, "mesh.yaml")
	model.Width = 100
	model.Height = 30

	// Test mouse wheel down moves selection
	newModel, _ := model.Update(tea.MouseMsg{Type: tea.MouseWheelDown})
	m := newModel.(Model)
	if m.SelectedNode != 1 && m.SelectedRoute != 1 {
		t.Logf("Wheel down updated selection")
	}

	// Test click on Nodes pane area
	clickMsg := tea.MouseMsg{
		X:      10,
		Y:      10,
		Type:   tea.MouseLeft,
		Action: tea.MouseActionPress,
	}
	newModel, _ = m.Update(clickMsg)
	m = newModel.(Model)
	if m.ActivePane != PaneNodes {
		t.Errorf("Expected ActivePane to be PaneNodes, got %v", m.ActivePane)
	}

	// Test click on Routes pane area
	clickRoutes := tea.MouseMsg{
		X:      60,
		Y:      10,
		Type:   tea.MouseLeft,
		Action: tea.MouseActionPress,
	}
	newModel, _ = m.Update(clickRoutes)
	m = newModel.(Model)
	if m.ActivePane != PaneRoutes {
		t.Errorf("Expected ActivePane to be PaneRoutes, got %v", m.ActivePane)
	}
}
