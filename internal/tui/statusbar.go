package tui

import (
	"fmt"

	"github.com/charmbracelet/lipgloss"
	"github.com/meshctl/meshctl/internal/config"
)

var (
	statusBarStyle = lipgloss.NewStyle().
			Background(lipgloss.Color("236")).
			Foreground(lipgloss.Color("252")).
			Padding(0, 1)

	unsavedStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("208")).
			Bold(true)

	keyHintStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("243"))
)

// RenderStatusBar формирует нижнюю информационную панель TUI.
func RenderStatusBar(m *config.Mesh, configPath string, isDirty bool, activePane string, width int) string {
	nodeCount := len(m.Nodes)
	routeCount := len(m.Routes)

	dirtyText := ""
	if isDirty {
		dirtyText = unsavedStyle.Render(" * [не сохранено]")
	}

	leftInfo := fmt.Sprintf("Ноды: %d | Маршруты: %d | Конфиг: %s%s",
		nodeCount, routeCount, configPath, dirtyText)

	hints := keyHintStyle.Render("[?] Help | [Tab] Панель | [a] Apply | [s] Save | [q] Quit")

	totalWidth := width - 4
	if totalWidth < 40 {
		totalWidth = 40
	}

	content := lipgloss.JoinHorizontal(
		lipgloss.Left,
		leftInfo,
	)

	fullBar := lipgloss.JoinHorizontal(
		lipgloss.Left,
		content,
		"    ",
		hints,
	)

	return statusBarStyle.Width(totalWidth).Render(fullBar)
}
