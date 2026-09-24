package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/meshctl/meshctl/internal/config"
	"github.com/meshctl/meshctl/internal/i18n"
)

// RenderListsPane отображает список доменов/IP для селективного туннелирования и кнопку добавления.
func RenderListsPane(m *config.Mesh, selectedIdx, hoverIdx int, isPressed bool, isActive bool, width, height int) string {
	var sb strings.Builder
	sb.WriteString(i18n.T("pane_lists"))
	sb.WriteString("\n\n")

	listCount := len(m.Lists)
	for i, l := range m.Lists {
		countD := len(l.Domains)
		countIP := len(l.IPs)
		line := fmt.Sprintf("%-10s (Domains: %d, IPs: %d)", l.Name, countD, countIP)

		st := itemNormalStyle
		prefix := "  "
		if i == selectedIdx {
			st = itemSelectedStyle
			prefix = "> "
		}
		if i == hoverIdx {
			if isPressed {
				st = itemPressedStyle
			} else {
				st = st.Copy().Underline(true)
			}
		}
		sb.WriteString(st.Render(fmt.Sprintf("%s%s", prefix, line)) + "\n")
	}

	addBtnIdx := listCount
	addBtnText := i18n.T("btn_add_list")
	addSt := lipgloss.NewStyle().Foreground(lipgloss.Color("244"))
	addPrefix := "  "
	if selectedIdx == addBtnIdx {
		addSt = itemSelectedStyle
		addPrefix = "> "
	}
	if hoverIdx == addBtnIdx {
		if isPressed {
			addSt = itemPressedStyle
		} else {
			addSt = addSt.Copy().Underline(true)
		}
	}
	sb.WriteString(addSt.Render(fmt.Sprintf("%s%s", addPrefix, addBtnText)) + "\n")

	style := paneStyle.Width(width)
	if isActive {
		style = activePaneStyle.Width(width)
	}

	return style.Render(sb.String())
}
