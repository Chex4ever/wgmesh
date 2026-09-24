package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/meshctl/meshctl/internal/config"
)

var (
	paneStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("240")).
			Padding(0, 1)

	activePaneStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("86")).
			Padding(0, 1)

	itemSelectedStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("86")).
				Bold(true)

	itemNormalStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("252"))
)

// RenderNodesPane отображает список нод сети.
func RenderNodesPane(m *config.Mesh, selectedIdx int, isActive bool, height int) string {
	var sb strings.Builder
	sb.WriteString("Список узлов (Nodes):\n\n")

	if len(m.Nodes) == 0 {
		sb.WriteString("  (Нет узлов. Нажмите 'n' для добавления)")
	} else {
		for i, n := range m.Nodes {
			icon := "●"
			switch n.Type {
			case config.TypeMikrotik:
				icon = "◆"
			case config.TypeOpenWRT:
				icon = "▲"
			}

			ip := n.MeshIP
			if ip == "" {
				ip = "авто"
			}

			line := fmt.Sprintf("%s %-14s [%-8s] %s (%s)", icon, n.Name, n.Type, n.Host, ip)

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
