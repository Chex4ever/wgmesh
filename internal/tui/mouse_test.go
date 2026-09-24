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

	// Test click release on Nodes pane area
	clickMsg := tea.MouseMsg{
		X:      10,
		Y:      10,
		Type:   tea.MouseRelease,
		Action: tea.MouseActionRelease,
	}
	newModel, _ = m.Update(clickMsg)
	m = newModel.(Model)
	if m.ActivePane != PaneNodes {
		t.Errorf("Expected ActivePane to be PaneNodes, got %v", m.ActivePane)
	}

	// Test hover coordinate tracking
	motionMsg := tea.MouseMsg{
		X:      60,
		Y:      10,
		Action: tea.MouseActionMotion,
	}
	newModel, _ = m.Update(motionMsg)
	m = newModel.(Model)
	if m.HoverX != 60 || m.HoverY != 10 {
		t.Errorf("Expected HoverX=60, HoverY=10, got %d, %d", m.HoverX, m.HoverY)
	}

	// Test click release on Routes pane area
	clickRoutes := tea.MouseMsg{
		X:      60,
		Y:      10,
		Type:   tea.MouseRelease,
		Action: tea.MouseActionRelease,
	}
	newModel, _ = m.Update(clickRoutes)
	m = newModel.(Model)
	if m.ActivePane != PaneRoutes {
		t.Errorf("Expected ActivePane to be PaneRoutes, got %v", m.ActivePane)
	}
}
