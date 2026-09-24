package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/meshctl/meshctl/internal/config"
)

// RenderRoutesPane отображает список маршрутов и интерактивную кнопку создания.
func RenderRoutesPane(m *config.Mesh, selectedIdx, hoverIdx int, isActive bool, width, height int) string {
	var sb strings.Builder
	sb.WriteString("Список маршрутов (Routes):\n\n")

	routeCount := len(m.Routes)
	for i, r := range m.Routes {
		hopsStr := strings.Join(r.Path, " ──▶ ")
		protBadge := ""
		if r.Protected {
			protBadge = lipgloss.NewStyle().Foreground(lipgloss.Color("208")).Render(" [PROT]")
		}
		line := fmt.Sprintf("%-10s [%s] (%s)%s", r.Name, hopsStr, r.ExitNode, protBadge)

		st := itemNormalStyle
		prefix := "  "
		if i == selectedIdx {
			st = itemSelectedStyle
			prefix = "► "
		}
		if i == hoverIdx {
			st = st.Copy().Underline(true)
		}
		sb.WriteString(st.Render(fmt.Sprintf("%s%s", prefix, line)) + "\n")
	}

	addBtnIdx := routeCount
	addBtnText := "➕ [+ Создать маршрут ('r')]"
	addSt := lipgloss.NewStyle().Foreground(lipgloss.Color("244"))
	addPrefix := "  "
	if selectedIdx == addBtnIdx {
		addSt = itemSelectedStyle
		addPrefix = "► "
	}
	if hoverIdx == addBtnIdx {
		addSt = addSt.Copy().Underline(true)
	}
	sb.WriteString(addSt.Render(fmt.Sprintf("%s%s", addPrefix, addBtnText)) + "\n")

	style := paneStyle.Width(width)
	if isActive {
		style = activePaneStyle.Width(width)
	}

	return style.Render(sb.String())
}
