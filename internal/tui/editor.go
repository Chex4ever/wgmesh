package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/meshctl/meshctl/internal/config"
	"github.com/meshctl/meshctl/internal/i18n"
)

type RouteEditor struct {
	RouteIdx    int
	SelectedHop int
}

// RenderEditorPane отображает интерактивный редактор выбранного маршрута.
func RenderEditorPane(m *config.Mesh, ed *RouteEditor, hoverHopIdx, hoverBtn int, isPressed bool, isActive bool, width int) string {
	style := paneStyle.Width(width)
	if isActive {
		style = activePaneStyle.Width(width)
	}

	if len(m.Routes) == 0 || ed.RouteIdx < 0 || ed.RouteIdx >= len(m.Routes) {
		return style.Render(i18n.T("pane_editor"))
	}

	r := &m.Routes[ed.RouteIdx]
	var sb strings.Builder
	sb.WriteString(i18n.T("editor_title", r.Name))
	sb.WriteString("\n\n")

	sb.WriteString(i18n.T("editor_hops"))
	sb.WriteString("\n")
	var elements []string
	arrow := arrowStyle.Render(" ---> ")

	for i, hop := range r.Path {
		box := ""
		st := boxStyle
		if i == ed.SelectedHop {
			st = selectedBoxStyle
		}
		if i == hoverHopIdx {
			if isPressed {
				st = st.Copy().Foreground(lipgloss.Color("214")).Bold(true).Underline(true)
			} else {
				st = st.Copy().Underline(true)
			}
		}

		if i == ed.SelectedHop {
			box = st.Render(fmt.Sprintf("[%s]", hop))
		} else {
			box = st.Render(hop)
		}

		if i > 0 {
			elements = append(elements, arrow)
		}
		elements = append(elements, box)
	}

	chain := lipgloss.JoinHorizontal(lipgloss.Center, elements...)
	sb.WriteString(indentBlock(chain, 3))
	sb.WriteString("\n\n")

	sb.WriteString(i18n.T("editor_exit_node", r.ExitNode))
	sb.WriteString("\n\n")

	btn0 := i18n.T("editor_add_hop")
	btn1 := i18n.T("editor_remove_hop")
	btn2 := i18n.T("editor_edit_route")

	st0 := lipgloss.NewStyle().Foreground(lipgloss.Color("243"))
	st1 := lipgloss.NewStyle().Foreground(lipgloss.Color("243"))
	st2 := lipgloss.NewStyle().Foreground(lipgloss.Color("243"))

	if hoverBtn == 0 {
		if isPressed {
			st0 = st0.Foreground(lipgloss.Color("214")).Bold(true).Underline(true)
		} else {
			st0 = st0.Underline(true)
		}
	}
	if hoverBtn == 1 {
		if isPressed {
			st1 = st1.Foreground(lipgloss.Color("214")).Bold(true).Underline(true)
		} else {
			st1 = st1.Underline(true)
		}
	}
	if hoverBtn == 2 {
		if isPressed {
			st2 = st2.Foreground(lipgloss.Color("214")).Bold(true).Underline(true)
		} else {
			st2 = st2.Underline(true)
		}
	}

	hints := fmt.Sprintf("%s | %s | %s", st0.Render(btn0), st1.Render(btn1), st2.Render(btn2))
	sb.WriteString(hints)

	return style.Render(sb.String())
}
