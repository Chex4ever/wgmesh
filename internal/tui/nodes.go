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

// RenderNodesPane отображает список нод сети и интерактивные кнопки добавления.
func RenderNodesPane(m *config.Mesh, selectedIdx int, isActive bool, width, height int) string {
	var sb strings.Builder
	sb.WriteString("Список узлов (Nodes):\n\n")

	nodeCount := len(m.Nodes)
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

		protBadge := ""
		if n.Protected {
			protBadge = lipgloss.NewStyle().Foreground(lipgloss.Color("208")).Render(" [PROT]")
		}

		line := fmt.Sprintf("%s %-10s [%-7s] %s%s", icon, n.Name, n.Type, n.Host, protBadge)

		if i == selectedIdx {
			sb.WriteString(itemSelectedStyle.Render(fmt.Sprintf("► %s", line)))
		} else {
			sb.WriteString(itemNormalStyle.Render(fmt.Sprintf("  %s", line)))
		}
		sb.WriteString("\n")
	}

	addBtnIdx := nodeCount
	bootBtnIdx := nodeCount + 1

	addBtnText := "➕ [+ Добавить ноду ('n')]"
	if selectedIdx == addBtnIdx {
		sb.WriteString(itemSelectedStyle.Render(fmt.Sprintf("► %s", addBtnText)))
	} else {
		sb.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("244")).Render(fmt.Sprintf("  %s", addBtnText)))
	}
	sb.WriteString("\n")

	bootBtnText := "🚀 [Bootstrap по SSH ('b')]"
	if selectedIdx == bootBtnIdx {
		sb.WriteString(itemSelectedStyle.Render(fmt.Sprintf("► %s", bootBtnText)))
	} else {
		sb.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("244")).Render(fmt.Sprintf("  %s", bootBtnText)))
	}
	sb.WriteString("\n")

	style := paneStyle.Width(width)
	if isActive {
		style = activePaneStyle.Width(width)
	}

	return style.Render(sb.String())
}
