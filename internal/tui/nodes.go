package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/meshctl/meshctl/internal/config"
	"github.com/meshctl/meshctl/internal/i18n"
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

	itemPressedStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("214")).
				Bold(true).
				Underline(true)
)

// RenderNodesPane отображает список нод сети и интерактивные кнопки добавления.
func RenderNodesPane(m *config.Mesh, selectedIdx, hoverIdx int, isPressed bool, isActive bool, width, height int) string {
	var sb strings.Builder
	sb.WriteString(i18n.T("pane_nodes") + "\n\n")

	nodeCount := len(m.Nodes)
	for i, n := range m.Nodes {
		icon := "*"
		switch n.Type {
		case config.TypeMikrotik:
			icon = "#"
		case config.TypeOpenWRT:
			icon = "^"
		}

		ip := n.MeshIP
		if ip == "" {
			ip = "auto"
		}

		protBadge := ""
		if n.Protected {
			protBadge = lipgloss.NewStyle().Foreground(lipgloss.Color("208")).Render(" [PROT]")
		}

		line := fmt.Sprintf("%s %-10s [%-7s] %s%s", icon, n.Name, n.Type, n.Host, protBadge)

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

	addBtnIdx := nodeCount
	bootBtnIdx := nodeCount + 1

	addBtnText := i18n.T("btn_add_node")
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

	bootBtnText := i18n.T("btn_bootstrap_node")
	bootSt := lipgloss.NewStyle().Foreground(lipgloss.Color("244"))
	bootPrefix := "  "
	if selectedIdx == bootBtnIdx {
		bootSt = itemSelectedStyle
		bootPrefix = "> "
	}
	if hoverIdx == bootBtnIdx {
		if isPressed {
			bootSt = itemPressedStyle
		} else {
			bootSt = bootSt.Copy().Underline(true)
		}
	}
	sb.WriteString(bootSt.Render(fmt.Sprintf("%s%s", bootPrefix, bootBtnText)) + "\n")

	style := paneStyle.Width(width)
	if isActive {
		style = activePaneStyle.Width(width)
	}

	return style.Render(sb.String())
}
