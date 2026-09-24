package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/meshctl/meshctl/internal/config"
)

// RenderRoutesPane отображает список маршрутов и интерактивную кнопку создания.
func RenderRoutesPane(m *config.Mesh, selectedIdx int, isActive bool, width, height int) string {
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

		if i == selectedIdx {
			sb.WriteString(itemSelectedStyle.Render(fmt.Sprintf("► %s", line)))
		} else {
			sb.WriteString(itemNormalStyle.Render(fmt.Sprintf("  %s", line)))
		}
		sb.WriteString("\n")
	}

	addBtnIdx := routeCount
	addBtnText := "➕ [+ Создать маршрут ('r')]"
	if selectedIdx == addBtnIdx {
		sb.WriteString(itemSelectedStyle.Render(fmt.Sprintf("► %s", addBtnText)))
	} else {
		sb.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("244")).Render(fmt.Sprintf("  %s", addBtnText)))
	}
	sb.WriteString("\n")

	style := paneStyle.Width(width)
	if isActive {
		style = activePaneStyle.Width(width)
	}

	return style.Render(sb.String())
}
