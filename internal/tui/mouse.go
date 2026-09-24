package tui

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/meshctl/meshctl/internal/config"
)

func (m *Model) handleMouseMsg(msg tea.MouseMsg) (Model, tea.Cmd) {
	// Всегда обновляем координаты ховера при любом перемещении или клике мыши
	m.HoverX = msg.X
	m.HoverY = msg.Y

	// 1. Колесо мыши для скроллинга навигации
	if msg.Type == tea.MouseWheelUp {
		m.moveSelection(-1)
		return *m, nil
	}
	if msg.Type == tea.MouseWheelDown {
		m.moveSelection(1)
		return *m, nil
	}

	// ВАЖНО: Выполнение действий происходит ИСКЛЮЧИТЕЛЬНО при отпускании кнопки мыши (Release)!
	// Это предотвращает дублирование и "безумие" при зажатии или движении мыши.
	isRelease := (msg.Type == tea.MouseRelease || msg.Action == tea.MouseActionRelease)
	if !isRelease {
		return *m, nil
	}

	x := msg.X
	y := msg.Y

	// 2. Если открыто модальное окно — обработка кликов внутри модалки
	if m.Modal.Type != ModalNone {
		return m.handleModalMouseClick(x, y)
	}

	// 3. Вычисление геометрии экрана как в View()
	totalWidth := m.Width - 4
	if totalWidth < 40 {
		totalWidth = 40
	}

	colWidth := (totalWidth - 4) / 2
	if colWidth < 20 {
		colWidth = 20
	}

	header := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("86")).
		Render("═══ " + m.Mesh.Name + " (wgmesh TUI) ═══")

	topView := RenderTopology(m.Mesh, m.SelectedRoute, totalWidth)
	topBox := paneStyle.Width(totalWidth).Render(header + "\n\n" + topView)
	topHeight := lipgloss.Height(topBox)

	paneHeight := 6
	nodesView := RenderNodesPane(m.Mesh, m.SelectedNode, -1, m.ActivePane == PaneNodes, colWidth, paneHeight)
	routesView := RenderRoutesPane(m.Mesh, m.SelectedRoute, -1, m.ActivePane == PaneRoutes, colWidth, paneHeight)
	clientsView := RenderClientsPane(m.Mesh, m.SelectedClient, -1, m.ActivePane == PaneClients, colWidth, paneHeight)
	listsView := RenderListsPane(m.Mesh, m.SelectedList, -1, m.ActivePane == PaneLists, colWidth, paneHeight)
	editorView := RenderEditorPane(m.Mesh, &m.Editor, -1, -1, m.ActivePane == PaneEditor, totalWidth)

	middleUpper := lipgloss.JoinHorizontal(lipgloss.Top, nodesView, " ", routesView)
	middleLower := lipgloss.JoinHorizontal(lipgloss.Top, clientsView, " ", listsView)

	midUpperHeight := lipgloss.Height(middleUpper)
	midLowerHeight := lipgloss.Height(middleLower)
	editorHeight := lipgloss.Height(editorView)

	nodesWidth := lipgloss.Width(nodesView)

	// --- ОПРЕДЕЛЕНИЕ ПАНЕЛИ ПО Y КООРДИНАТЕ ---

	// A. Верхняя панель (PaneTopology)
	if y >= 0 && y < topHeight {
		m.ActivePane = PaneTopology
		yRel := y - 1
		topoLine := yRel - 2
		if len(m.Mesh.Routes) == 0 {
			if topoLine >= 0 && topoLine <= 4 {
				if topoLine == 1 {
					m.openBootstrapModal()
				} else if topoLine == 2 {
					m.openAddRouteModal()
				} else if topoLine == 3 {
					m.LogMsg = "→ Запуск полного применения (Apply) по SSH…"
					m.runApply()
				}
			}
		} else {
			if topoLine >= 0 {
				rIdx := topoLine / 5
				if rIdx < len(m.Mesh.Routes) {
					m.SelectedRoute = rIdx
					m.Editor.RouteIdx = rIdx
				}
			}
		}
		return *m, nil
	}

	// B. Средняя верхняя секция (Nodes & Routes)
	if y >= topHeight && y < topHeight+midUpperHeight {
		yRel := y - topHeight - 1

		if x < nodesWidth {
			m.ActivePane = PaneNodes
			nodeCount := len(m.Mesh.Nodes)
			if yRel >= 2 && yRel < 2+nodeCount {
				m.SelectedNode = yRel - 2
			} else if yRel == 2+nodeCount {
				m.SelectedNode = nodeCount
				m.openAddNodeModal()
			} else if yRel == 3+nodeCount {
				m.SelectedNode = nodeCount + 1
				m.openBootstrapModal()
			}
		} else {
			m.ActivePane = PaneRoutes
			routeCount := len(m.Mesh.Routes)
			if yRel >= 2 && yRel < 2+routeCount {
				m.SelectedRoute = yRel - 2
				m.Editor.RouteIdx = m.SelectedRoute
			} else if yRel == 2+routeCount {
				m.SelectedRoute = routeCount
				m.openAddRouteModal()
			}
		}
		return *m, nil
	}

	// C. Средняя нижняя секция (Clients & Lists)
	if y >= topHeight+midUpperHeight && y < topHeight+midUpperHeight+midLowerHeight {
		yRel := y - (topHeight + midUpperHeight) - 1

		if x < nodesWidth {
			m.ActivePane = PaneClients
			clientCount := len(m.Mesh.Clients)
			if yRel >= 2 && yRel < 2+clientCount {
				m.SelectedClient = yRel - 2
			} else if yRel == 2+clientCount {
				m.SelectedClient = clientCount
				m.openAddClientModal()
			}
		} else {
			m.ActivePane = PaneLists
			listCount := len(m.Mesh.Lists)
			if yRel >= 2 && yRel < 2+listCount {
				m.SelectedList = yRel - 2
			} else if yRel == 2+listCount {
				m.SelectedList = listCount
				m.openAddListModal()
			}
		}
		return *m, nil
	}

	// D. Панель Редактора Маршрута (PaneEditor)
	if y >= topHeight+midUpperHeight+midLowerHeight && y < topHeight+midUpperHeight+midLowerHeight+editorHeight {
		m.ActivePane = PaneEditor
		yRel := y - (topHeight + midUpperHeight + midLowerHeight) - 1

		if yRel >= 3 && yRel <= 5 {
			if len(m.Mesh.Routes) > 0 && m.SelectedRoute < len(m.Mesh.Routes) {
				r := &m.Mesh.Routes[m.SelectedRoute]
				hopWidth := 15
				if x >= 3 {
					hIdx := (x - 3) / hopWidth
					if hIdx >= 0 && hIdx < len(r.Path) {
						m.Editor.SelectedHop = hIdx
					}
				}
			}
		} else if yRel >= 6 {
			if x >= 0 && x < 22 {
				m.addHopToSelectedRoute()
			} else if x >= 22 && x < 42 {
				m.removeHopFromSelectedRoute()
			} else if x >= 42 {
				m.openEditRouteModal()
			}
		}
		return *m, nil
	}

	// E. Нижняя статусный строка (Status Bar)
	if y >= topHeight+midUpperHeight+midLowerHeight+editorHeight {
		m.handleStatusBarClick(x)
		return *m, nil
	}

	return *m, nil
}

func (m *Model) handleStatusBarClick(x int) {
	nodeCount := len(m.Mesh.Nodes)
	routeCount := len(m.Mesh.Routes)
	dirtyText := ""
	if m.IsDirty {
		dirtyText = " * [НЕ СОХРАНЕНО — нажмите 's']"
	}
	leftInfo := "Узлы: " + string(rune('0'+nodeCount)) + " | Маршруты: " + string(rune('0'+routeCount)) + " | " + m.ConfigPath + dirtyText
	leftWidth := len(leftInfo) + 4

	if x < leftWidth {
		if m.IsDirty {
			if err := config.Save(m.ConfigPath, m.Mesh); err != nil {
				m.LogMsg = "Ошибка сохранения: " + err.Error()
			} else {
				m.IsDirty = false
				m.LogMsg = "✔ Конфигурация успешно сохранена в " + m.ConfigPath
			}
		}
		return
	}

	hintsX := x - leftWidth
	switch {
	case hintsX >= 0 && hintsX <= 10:
		if err := config.Save(m.ConfigPath, m.Mesh); err != nil {
			m.LogMsg = "Ошибка сохранения: " + err.Error()
		} else {
			m.IsDirty = false
			m.LogMsg = "✔ Конфигурация успешно сохранена в " + m.ConfigPath
		}
	case hintsX > 10 && hintsX <= 22:
		m.LogMsg = "→ Запуск полного применения (Apply) по SSH…"
		m.runApply()
	case hintsX > 22 && hintsX <= 38:
		m.openBootstrapModal()
	case hintsX > 38 && hintsX <= 51:
		m.runDoctor()
	case hintsX > 51 && hintsX <= 64:
		if len(m.Mesh.Routes) > 0 && m.SelectedRoute < len(m.Mesh.Routes) {
			m.handleExportFormat("w")
		} else {
			m.LogMsg = "ℹ️ Выберите маршрут для экспорта"
		}
	case hintsX > 64 && hintsX <= 74:
		m.openGitModal()
	case hintsX > 74 && hintsX <= 85:
		m.Modal = ModalState{Type: ModalHelp}
	}
}

func (m *Model) handleModalMouseClick(x, y int) (Model, tea.Cmd) {
	modalBody := renderModalOverlay(*m)
	mWidth := lipgloss.Width(modalBody)
	mHeight := lipgloss.Height(modalBody)

	top := (m.Height - mHeight) / 2
	left := (m.Width - mWidth) / 2

	if x < left || x >= left+mWidth || y < top || y >= top+mHeight {
		m.Modal = ModalState{Type: ModalNone}
		return *m, nil
	}

	yRel := y - top - 1
	xRel := x - left - 2

	switch m.Modal.Type {
	case ModalHelp, ModalCapabilities:
		m.Modal = ModalState{Type: ModalNone}

	case ModalDoctor:
		if yRel >= mHeight-3 {
			if xRel < 25 {
				m.runDoctor()
			} else {
				m.Modal = ModalState{Type: ModalNone}
			}
		}

	case ModalExport:
		if yRel >= mHeight-3 {
			if xRel >= 0 && xRel < 15 {
				m.handleExportFormat("w")
			} else if xRel >= 15 && xRel < 28 {
				m.handleExportFormat("a")
			} else if xRel >= 28 && xRel < 41 {
				m.handleExportFormat("s")
			} else if xRel >= 41 && xRel < 50 {
				m.handleExportFormat("u")
			} else if xRel >= 50 && xRel < 62 {
				m.handleExportFormat("q")
			} else {
				m.Modal = ModalState{Type: ModalNone}
			}
		}

	case ModalConfirm:
		if yRel >= 3 {
			if xRel < 30 {
				if m.Modal.OnConfirm != nil {
					m.Modal.OnConfirm(m)
				}
				m.Modal = ModalState{Type: ModalNone}
			} else {
				m.Modal = ModalState{Type: ModalNone}
			}
		}

	default:
		if len(m.Modal.Fields) > 0 {
			fieldIdx := yRel - 2
			if fieldIdx >= 0 && fieldIdx < len(m.Modal.Fields) {
				m.Modal.ActiveField = fieldIdx
				f := &m.Modal.Fields[fieldIdx]
				if len(f.Options) > 0 {
					f.OptionIdx = (f.OptionIdx + 1) % len(f.Options)
					f.Value = f.Options[f.OptionIdx]
				}
			} else if yRel >= 2+len(m.Modal.Fields) {
				if xRel < 40 {
					m.submitCurrentModal()
				} else {
					m.Modal = ModalState{Type: ModalNone}
				}
			}
		}
	}

	return *m, nil
}
