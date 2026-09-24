package tui

import (
	"fmt"
	"strings"

	"github.com/meshctl/meshctl/internal/config"
)

// RenderClientsPane отображает список клиентских устройств.
func RenderClientsPane(m *config.Mesh, selectedIdx int, isActive bool, width, height int) string {
	var sb strings.Builder
	sb.WriteString("Список клиентов (Clients):\n\n")

	if len(m.Clients) == 0 {
		sb.WriteString("  (Нет клиентов. Нажмите 'c')")
	} else {
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
	}

	style := paneStyle.Width(width)
	if isActive {
		style = activePaneStyle.Width(width)
	}

	return style.Render(sb.String())
}
