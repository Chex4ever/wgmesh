package tui

import (
	"fmt"
	"strings"

	"github.com/meshctl/meshctl/internal/config"
)

// RenderListsPane отображает список доменов/IP для селективного туннелирования.
func RenderListsPane(m *config.Mesh, selectedIdx int, isActive bool, height int) string {
	var sb strings.Builder
	sb.WriteString("Списки доменов/IP (Split Tunneling):\n\n")

	if len(m.Lists) == 0 {
		sb.WriteString("  (Нет списков. Нажмите 'l' для создания)")
	} else {
		for i, l := range m.Lists {
			countD := len(l.Domains)
			countIP := len(l.IPs)
			line := fmt.Sprintf("🌐 %-14s (Доменов: %d, IPs: %d)", l.Name, countD, countIP)

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
