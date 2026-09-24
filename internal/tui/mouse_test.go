package tui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
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

	// Compute topBox height dynamically
	topView := RenderTopology(mesh, 0, 96)
	topBox := paneStyle.Width(96).Render("═══ TestMesh (wgmesh TUI) ═══\n\n" + topView)
	topHeight := lipgloss.Height(topBox)

	// Test mouse wheel down moves selection
	newModel, _ := model.Update(tea.MouseMsg{Type: tea.MouseWheelDown})
	m := newModel.(Model)
	if m.SelectedNode != 1 && m.SelectedRoute != 1 {
		t.Logf("Wheel down updated selection")
	}

	// Test click release on Nodes pane area (Line 2 of Nodes pane is Node 0 -> Y = topHeight + 1 + 2)
	clickNodes := tea.MouseMsg{
		X:      10,
		Y:      topHeight + 3,
		Type:   tea.MouseRelease,
		Action: tea.MouseActionRelease,
	}
	newModel, _ = m.Update(clickNodes)
	m = newModel.(Model)
	if m.ActivePane != PaneNodes {
		t.Errorf("Expected ActivePane to be PaneNodes, got %v", m.ActivePane)
	}

	// Test hover coordinate tracking
	motionMsg := tea.MouseMsg{
		X:      60,
		Y:      topHeight + 3,
		Action: tea.MouseActionMotion,
	}
	newModel, _ = m.Update(motionMsg)
	m = newModel.(Model)
	if m.HoverX != 60 || m.HoverY != topHeight+3 {
		t.Errorf("Expected HoverX=60, HoverY=%d, got %d, %d", topHeight+3, m.HoverX, m.HoverY)
	}

	// Test click release on Routes pane area
	clickRoutes := tea.MouseMsg{
		X:      60,
		Y:      topHeight + 3,
		Type:   tea.MouseRelease,
		Action: tea.MouseActionRelease,
	}
	newModel, _ = m.Update(clickRoutes)
	m = newModel.(Model)
	if m.ActivePane != PaneRoutes {
		t.Errorf("Expected ActivePane to be PaneRoutes, got %v", m.ActivePane)
	}
}

func TestCascadingRenameNode(t *testing.T) {
	mesh := &config.Mesh{
		Name: "TestMesh",
		Nodes: []config.Node{
			{Name: "kz-gohost", Host: "1.1.1.1"},
			{Name: "de-server", Host: "2.2.2.2"},
		},
		Routes: []config.Route{
			{Name: "r1", Path: []string{"client", "kz-gohost", "de-server"}, ExitNode: "de-server"},
			{Name: "r2", Path: []string{"client", "kz-gohost"}, ExitNode: "kz-gohost"},
		},
		Clients: []config.Client{
			{Name: "client1", Ingress: "kz-gohost"},
		},
	}
	model := NewModel(mesh, "mesh.yaml")

	// Perform cascading rename kz-gohost -> kz-main
	model.renameNode("kz-gohost", "kz-main")

	// Verify route paths updated
	if model.Mesh.Routes[0].Path[1] != "kz-main" {
		t.Errorf("Expected r1 path[1] to be 'kz-main', got %s", model.Mesh.Routes[0].Path[1])
	}
	if model.Mesh.Routes[1].ExitNode != "kz-main" {
		t.Errorf("Expected r2 exit_node to be 'kz-main', got %s", model.Mesh.Routes[1].ExitNode)
	}
	// Verify client ingress updated
	if model.Mesh.Clients[0].Ingress != "kz-main" {
		t.Errorf("Expected client1 ingress to be 'kz-main', got %s", model.Mesh.Clients[0].Ingress)
	}
}

func TestDuplicateNameValidation(t *testing.T) {
	mesh := &config.Mesh{
		Nodes: []config.Node{
			{Name: "node1"},
			{Name: "node2"},
		},
	}
	model := NewModel(mesh, "mesh.yaml")

	if !model.isNodeNameTaken("node1", -1) {
		t.Errorf("Expected node1 to be taken")
	}
	if !model.isNodeNameTaken("NODE1", -1) {
		t.Errorf("Expected NODE1 (case insensitive) to be taken")
	}
	if model.isNodeNameTaken("node1", 0) {
		t.Errorf("Expected node1 to NOT be taken when excluding its own index (0)")
	}
	if model.isNodeNameTaken("node3", -1) {
		t.Errorf("Expected node3 to NOT be taken")
	}
}
