package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/meshctl/meshctl/internal/config"
)

type RouteEditor struct {
	RouteIdx    int
	SelectedHop int
}

// RenderEditorPane отображает интерактивный редактор выбранного маршрута.
func RenderEditorPane(m *config.Mesh, ed *RouteEditor, isActive bool, width int) string {
	style := paneStyle.Width(width)
	if isActive {
		style = activePaneStyle.Width(width)
	}

	if len(m.Routes) == 0 || ed.RouteIdx < 0 || ed.RouteIdx >= len(m.Routes) {
		return style.Render("Редактор маршрута: Выберите маршрут для редактирования")
	}

	r := &m.Routes[ed.RouteIdx]
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Редактирование маршрута: %s\n\n", r.Name))

	sb.WriteString("Цепочка хопов:\n")
	var elements []string
	arrow := arrowStyle.Render(" ──▶ ")

	for i, hop := range r.Path {
		box := ""
		if i == ed.SelectedHop {
			box = selectedBoxStyle.Render(fmt.Sprintf("[%s]", hop))
		} else {
			box = boxStyle.Render(hop)
		}
		if i > 0 {
			elements = append(elements, arrow)
		}
		elements = append(elements, box)
	}

	chain := lipgloss.JoinHorizontal(lipgloss.Center, elements...)
	sb.WriteString("   " + chain + "\n\n")

	sb.WriteString(fmt.Sprintf("Exit Node: %s\n\n", r.ExitNode))

	hints := lipgloss.NewStyle().Foreground(lipgloss.Color("243")).Render(
		"[+] добавить хоп | [-] удалить хоп | [e] изменить маршрут")
	sb.WriteString(hints)

	return style.Render(sb.String())
}
