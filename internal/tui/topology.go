package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/meshctl/meshctl/internal/config"
)

var (
	selectedRouteStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("86")).
				Bold(true)

	normalRouteStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("240"))

	boxStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			Padding(0, 1)

	selectedBoxStyle = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(lipgloss.Color("86")).
				Foreground(lipgloss.Color("86")).
				Bold(true).
				Padding(0, 1)

	arrowStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("244"))
)

// RenderTopology строит ASCII-визуализацию маршрутов и топологии сети.
func RenderTopology(m *config.Mesh, activeRouteIdx int, width int) string {
	if len(m.Routes) == 0 {
		if len(m.Nodes) == 0 {
			return lipgloss.NewStyle().
				Foreground(lipgloss.Color("214")).
				Render(
					"💡 Карта топологии пока пуста. Для старта:\n" +
						"   1. Нажмите [b] для подключения первого сервера/роутера по SSH (Bootstrap)\n" +
						"   2. Нажмите [r] для создания первого exit-маршрута\n" +
						"   3. Нажмите [a] для быстрой настройки узлов по SSH (Apply)",
				)
		}
		return lipgloss.NewStyle().
			Foreground(lipgloss.Color("86")).
			Render(
				"💡 Узлы добавлены, но маршруты еще не настроены:\n" +
					"   1. Нажмите [r] для создания первого маршрута через узел\n" +
					"   2. Нажмите [a] для применения конфигурации по SSH",
			)
	}

	var sb strings.Builder
	for i, r := range m.Routes {
		isSelected := (i == activeRouteIdx)
		chainStr := renderRouteChain(m, &r, isSelected)
		if isSelected {
			sb.WriteString(selectedRouteStyle.Render(fmt.Sprintf("► %s", r.Name)))
		} else {
			sb.WriteString(normalRouteStyle.Render(fmt.Sprintf("  %s", r.Name)))
		}
		sb.WriteString("\n")
		sb.WriteString(chainStr)
		if i < len(m.Routes)-1 {
			sb.WriteString("\n\n")
		}
	}

	return sb.String()
}

func renderRouteChain(m *config.Mesh, r *config.Route, isSelected bool) string {
	var parts []string
	parts = append(parts, renderBox("Client", isSelected))

	for _, hopName := range r.Hops() {
		node := m.NodeByName(hopName)
		label := hopName
		if node != nil {
			switch node.Type {
			case config.TypeMikrotik:
				label = fmt.Sprintf("◆ %s", hopName)
			case config.TypeOpenWRT:
				label = fmt.Sprintf("▲ %s", hopName)
			default:
				label = fmt.Sprintf("● %s", hopName)
			}
		}
		parts = append(parts, renderBox(label, isSelected))
	}

	parts = append(parts, renderBox("🌐 Internet", isSelected))

	arrow := arrowStyle.Render(" ──▶ ")
	return "   " + strings.Join(parts, arrow)
}

func renderBox(text string, isSelected bool) string {
	if isSelected {
		return selectedBoxStyle.Render(text)
	}
	return boxStyle.Render(text)
}
