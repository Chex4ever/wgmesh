package tui

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/meshctl/meshctl/internal/config"
	"github.com/meshctl/meshctl/internal/mesh"
)

type Pane int

const (
	PaneTopology Pane = iota
	PaneNodes
	PaneRoutes
	PaneEditor
)

type Model struct {
	Mesh       *config.Mesh
	ConfigPath string
	IsDirty    bool
	ActivePane Pane

	SelectedNode  int
	SelectedRoute int
	Editor        RouteEditor

	Width  int
	Height int

	LogMsg   string
	ShowHelp bool
	Applying bool
}

func NewModel(m *config.Mesh, configPath string) Model {
	return Model{
		Mesh:          m,
		ConfigPath:    configPath,
		ActivePane:    PaneTopology,
		SelectedNode:  0,
		SelectedRoute: 0,
		Editor: RouteEditor{
			RouteIdx:    0,
			SelectedHop: 0,
		},
		Width:  100,
		Height: 30,
	}
}

func (m Model) Init() tea.Cmd {
	return nil
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.Width = msg.Width
		m.Height = msg.Height

	case tea.KeyMsg:
		if m.ShowHelp {
			if msg.String() == "?" || msg.String() == "q" || msg.String() == "esc" {
				m.ShowHelp = false
			}
			return m, nil
		}

		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit

		case "?":
			m.ShowHelp = true
			return m, nil

		case "tab":
			m.ActivePane = (m.ActivePane + 1) % 4

		case "s":
			if err := config.Save(m.ConfigPath, m.Mesh); err != nil {
				m.LogMsg = fmt.Sprintf("Ошибка сохранения: %v", err)
			} else {
				m.IsDirty = false
				m.LogMsg = fmt.Sprintf("Конфигурация успешно сохранена в %s", m.ConfigPath)
			}

		case "a":
			m.LogMsg = "Запуск применения (Apply)..."
			mgr := mesh.NewManager(m.Mesh)
			if _, err := mgr.EnsureKeys(); err != nil {
				m.LogMsg = fmt.Sprintf("Ошибка генерации ключей: %v", err)
				return m, nil
			}
			if _, err := mgr.EnsureIPs(); err != nil {
				m.LogMsg = fmt.Sprintf("Ошибка назначения IP: %v", err)
				return m, nil
			}
			if err := mesh.Validate(m.Mesh); err != nil {
				m.LogMsg = fmt.Sprintf("Ошибка валидации: %v", err)
				return m, nil
			}
			m.LogMsg = "✔ Валидация пройдена. План применения построен."

		case "up", "k":
			switch m.ActivePane {
			case PaneTopology, PaneRoutes:
				if m.SelectedRoute > 0 {
					m.SelectedRoute--
					m.Editor.RouteIdx = m.SelectedRoute
				}
			case PaneNodes:
				if m.SelectedNode > 0 {
					m.SelectedNode--
				}
			}

		case "down", "j":
			switch m.ActivePane {
			case PaneTopology, PaneRoutes:
				if m.SelectedRoute < len(m.Mesh.Routes)-1 {
					m.SelectedRoute++
					m.Editor.RouteIdx = m.SelectedRoute
				}
			case PaneNodes:
				if m.SelectedNode < len(m.Mesh.Nodes)-1 {
					m.SelectedNode++
				}
			}

		case "+", "=":
			if len(m.Mesh.Routes) > 0 && len(m.Mesh.Nodes) > 0 {
				r := &m.Mesh.Routes[m.SelectedRoute]
				// Добавляем первую доступную ноду
				nodeToAdd := m.Mesh.Nodes[0].Name
				r.Path = append(r.Path, nodeToAdd)
				r.ExitNode = nodeToAdd
				m.IsDirty = true
				m.LogMsg = fmt.Sprintf("Добавлен хоп %s в маршрут %s", nodeToAdd, r.Name)
			}

		case "-", "_":
			if len(m.Mesh.Routes) > 0 {
				r := &m.Mesh.Routes[m.SelectedRoute]
				if len(r.Path) > 2 { // Не удаляем client
					removed := r.Path[len(r.Path)-1]
					r.Path = r.Path[:len(r.Path)-1]
					r.ExitNode = r.Path[len(r.Path)-1]
					m.IsDirty = true
					m.LogMsg = fmt.Sprintf("Удалён хоп %s из маршрута %s", removed, r.Name)
				}
			}
		}
	}

	return m, nil
}

func (m Model) View() string {
	if m.ShowHelp {
		return renderHelpOverlay(m.Width, m.Height)
	}

	header := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("86")).
		Render(fmt.Sprintf("═══ %s (meshctl TUI) ═══", m.Mesh.Name))

	topView := RenderTopology(m.Mesh, m.SelectedRoute, m.Width)

	topBox := paneStyle.Width(m.Width - 4).Render(
		header + "\n\n" + topView,
	)

	nodesView := RenderNodesPane(m.Mesh, m.SelectedNode, m.ActivePane == PaneNodes, m.Height)
	routesView := RenderRoutesPane(m.Mesh, m.SelectedRoute, m.ActivePane == PaneRoutes, m.Height)
	editorView := RenderEditorPane(m.Mesh, &m.Editor, m.ActivePane == PaneEditor)

	middle := lipgloss.JoinHorizontal(
		lipgloss.Top,
		nodesView,
		routesView,
	)

	bottomLog := ""
	if m.LogMsg != "" {
		bottomLog = lipgloss.NewStyle().
			Foreground(lipgloss.Color("214")).
			Render(fmt.Sprintf(" Лог: %s", m.LogMsg)) + "\n"
	}

	statusBar := RenderStatusBar(m.Mesh, m.ConfigPath, m.IsDirty, "", m.Width)

	return lipgloss.JoinVertical(
		lipgloss.Left,
		topBox,
		middle,
		editorView,
		bottomLog,
		statusBar,
	)
}

func renderHelpOverlay(width, height int) string {
	helpText := `
═══ Справка по горячим клавишам meshctl TUI ═══

  [↑ / ↓] или [j / k]   — Навигация по выбранной панели
  [Tab]                — Переключение активной панели
  [+] / [-]            — Добавить / удалить хоп в маршруте
  [s]                  — Сохранить конфигурацию в mesh.yaml
  [a]                  — Проверить и применить конфигурацию (Apply)
  [?]                  — Закрыть эту справку
  [q] / [Ctrl+C]       — Выход из TUI
`
	return lipgloss.NewStyle().
		Width(width - 6).
		Height(height - 4).
		Border(lipgloss.DoubleBorder()).
		BorderForeground(lipgloss.Color("86")).
		Padding(1, 2).
		Render(helpText)
}
