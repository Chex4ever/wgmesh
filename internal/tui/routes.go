package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/meshctl/meshctl/internal/config"
)

// RenderRoutesPane отображает список маршрутов.
func RenderRoutesPane(m *config.Mesh, selectedIdx int, isActive bool, height int) string {
	var sb strings.Builder
	sb.WriteString("Список маршрутов (Routes):\n\n")

	if len(m.Routes) == 0 {
		sb.WriteString("  (Нет маршрутов. Нажмите 'r' для создания)")
	} else {
		for i, r := range m.Routes {
			hopsStr := strings.Join(r.Path, " ──▶ ")
			protBadge := ""
			if r.Protected {
				protBadge = lipgloss.NewStyle().Foreground(lipgloss.Color("208")).Render(" [PROTECTED]")
			}
			line := fmt.Sprintf("%-14s [%s] (Exit: %s)%s", r.Name, hopsStr, r.ExitNode, protBadge)

			if i == selectedIdx {
				sb.WriteString(itemSelectedStyle.Render(fmt.Sprintf("► %s", line)))
			} else {
				sb.WriteString(itemNormalStyle.Render(fmt.Sprintf("  %s", line)))
			}
			sb.WriteString("\n")
		}
	}

	style := paneStyle
	if isActive {
		style = activePaneStyle
	}

	return style.Render(sb.String())
}
