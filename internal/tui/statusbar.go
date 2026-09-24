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
		dirtyText = unsavedStyle.Render(" * [НЕ СОХРАНЕНО — нажмите 's']")
	}

	leftInfo := fmt.Sprintf("Узлы: %d | Маршруты: %d | %s%s",
		nodeCount, routeCount, configPath, dirtyText)

	hints := keyHintStyle.Render("[s] Save | [a] Apply | [b] Bootstrap | [d] Doctor | [x] Export | [g] Git | [?] Help | [q] Quit")

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
