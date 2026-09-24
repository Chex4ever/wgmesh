package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/meshctl/meshctl/internal/config"
)

// RenderListsPane отображает список доменов/IP для селективного туннелирования и кнопку добавления.
func RenderListsPane(m *config.Mesh, selectedIdx int, isActive bool, width, height int) string {
	var sb strings.Builder
	sb.WriteString("Списки доменов/IP (Split Tunneling):\n\n")

	listCount := len(m.Lists)
	for i, l := range m.Lists {
		countD := len(l.Domains)
		countIP := len(l.IPs)
		line := fmt.Sprintf("🌐 %-10s (Domains: %d, IPs: %d)", l.Name, countD, countIP)

		if i == selectedIdx {
			sb.WriteString(itemSelectedStyle.Render(fmt.Sprintf("► %s", line)))
		} else {
			sb.WriteString(itemNormalStyle.Render(fmt.Sprintf("  %s", line)))
		}
		sb.WriteString("\n")
	}

	addBtnIdx := listCount
	addBtnText := "🌐 [+ Добавить список ('l')]"
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
