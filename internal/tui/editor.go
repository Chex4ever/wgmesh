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
func RenderEditorPane(m *config.Mesh, ed *RouteEditor, isActive bool) string {
	if len(m.Routes) == 0 || ed.RouteIdx < 0 || ed.RouteIdx >= len(m.Routes) {
		return paneStyle.Render("Редактор маршрута:Выберите маршрут для редактирования")
	}

	r := &m.Routes[ed.RouteIdx]
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Редактирование маршрута: %s\n\n", r.Name))

	sb.WriteString("Цепочка хопов:\n   ")
	for i, hop := range r.Path {
		box := hop
		if i == ed.SelectedHop {
			box = selectedBoxStyle.Render(fmt.Sprintf("[%s]", hop))
		} else {
			box = boxStyle.Render(hop)
		}
		sb.WriteString(box)
		if i < len(r.Path)-1 {
			sb.WriteString(" ──▶ ")
		}
	}

	sb.WriteString("\n\n")
	sb.WriteString(fmt.Sprintf("Exit Node: %s\n\n", r.ExitNode))

	hints := lipgloss.NewStyle().Foreground(lipgloss.Color("243")).Render(
		"[←/→] навигация | [+] добавить хоп | [-] удалить | [e] установить exit")
	sb.WriteString(hints)

	style := paneStyle
	if isActive {
		style = activePaneStyle
	}

	return style.Render(sb.String())
}
