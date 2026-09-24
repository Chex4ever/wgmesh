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
func RenderEditorPane(m *config.Mesh, ed *RouteEditor, hoverHopIdx, hoverBtn int, isActive bool, width int) string {
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
		st := boxStyle
		if i == ed.SelectedHop {
			st = selectedBoxStyle
		}
		if i == hoverHopIdx {
			st = st.Copy().Underline(true)
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
	sb.WriteString(indentBlock(chain, 3) + "\n\n")

	sb.WriteString(fmt.Sprintf("Exit Node: %s\n\n", r.ExitNode))

	btn0 := "[+] добавить хоп"
	btn1 := "[-] удалить хоп"
	btn2 := "[e] изменить маршрут"

	st0 := lipgloss.NewStyle().Foreground(lipgloss.Color("243"))
	st1 := lipgloss.NewStyle().Foreground(lipgloss.Color("243"))
	st2 := lipgloss.NewStyle().Foreground(lipgloss.Color("243"))

	if hoverBtn == 0 {
		st0 = st0.Underline(true)
	}
	if hoverBtn == 1 {
		st1 = st1.Underline(true)
	}
	if hoverBtn == 2 {
		st2 = st2.Underline(true)
	}

	hints := fmt.Sprintf("%s | %s | %s", st0.Render(btn0), st1.Render(btn1), st2.Render(btn2))
	sb.WriteString(hints)

	return style.Render(sb.String())
}
