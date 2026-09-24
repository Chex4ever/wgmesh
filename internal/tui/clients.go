package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/meshctl/meshctl/internal/config"
)

// RenderClientsPane отображает список клиентских устройств и кнопку добавления.
func RenderClientsPane(m *config.Mesh, selectedIdx int, isActive bool, width, height int) string {
	var sb strings.Builder
	sb.WriteString("Список клиентов (Clients):\n\n")

	clientCount := len(m.Clients)
	for i, c := range m.Clients {
		ip := c.IP
		if ip == "" {
			ip = "авто"
		}
		ingress := c.Ingress
		if ingress == "" {
			ingress = "не назначен"
		}
		line := fmt.Sprintf("📱 %-10s [%s] -> %s", c.Name, ip, ingress)

		if i == selectedIdx {
			sb.WriteString(itemSelectedStyle.Render(fmt.Sprintf("► %s", line)))
		} else {
			sb.WriteString(itemNormalStyle.Render(fmt.Sprintf("  %s", line)))
		}
		sb.WriteString("\n")
	}

	addBtnIdx := clientCount
	addBtnText := "📱 [+ Добавить клиента ('c')]"
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
