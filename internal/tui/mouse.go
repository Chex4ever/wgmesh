package tui

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/wgmesh/wgmesh/internal/config"
	"github.com/wgmesh/wgmesh/internal/i18n"
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

	// Разделение состояний взаимодействия мыши:
	// - Hover (движение): элементы подчёркиваются
	// - Press (зажатие): элементы выделяются подсветкой (isPressed = true)
	// - Release (отпускание): выполняется действие
	isPress := (msg.Type == tea.MouseLeft || msg.Action == tea.MouseActionPress) && msg.Type != tea.MouseRelease && msg.Action != tea.MouseActionRelease
	isRelease := (msg.Type == tea.MouseRelease || msg.Action == tea.MouseActionRelease)

	if isPress {
		m.IsMousePressed = true
		return *m, nil
	}

	if isRelease {
		m.IsMousePressed = false
	} else {
		m.IsMousePressed = false
		return *m, nil
	}

	x := msg.X
	y := msg.Y

	// 2. Если открыто модальное окно — обработка кликов внутри модалки
	if m.Modal.Type != ModalNone {
		if m.Modal.IsClosing {
			return *m, nil
		}
		return m.handleModalMouseClick(x, y)
	}

	// 3. Вычисление геометрии экрана
	g := computeLayoutGeometry(*m)

	// --- ОПРЕДЕЛЕНИЕ ПАНЕЛИ ПО Y КООРДИНАТЕ ---

	// A. Верхняя панель (PaneTopology)
	if y >= 0 && y < g.topHeight {
		m.ActivePane = PaneTopology
		yRel := y - 1
		topoLine := yRel - 2
		if len(m.Mesh.Routes) == 0 {
			if topoLine >= 0 && topoLine <= 4 {
				if topoLine == 1 {
					cmd := m.openBootstrapModal()
					return *m, cmd
				} else if topoLine == 2 {
					cmd := m.openAddRouteModal()
					return *m, cmd
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
	if y >= g.topHeight && y < g.topHeight+g.midUpperHeight {
		yRel := y - g.topHeight - 1

		if x < g.nodesWidth {
			nodeCount := len(m.Mesh.Nodes)
			if yRel >= 2 && yRel < 2+nodeCount {
				clickedIdx := yRel - 2
				if m.ActivePane == PaneNodes && m.SelectedNode == clickedIdx {
					cmd := m.openEditNodeModal()
					return *m, cmd
				} else {
					m.ActivePane = PaneNodes
					m.SelectedNode = clickedIdx
				}
			} else if yRel == 2+nodeCount {
				m.ActivePane = PaneNodes
				m.SelectedNode = nodeCount
				cmd := m.openAddNodeModal()
				return *m, cmd
			} else if yRel == 3+nodeCount {
				m.ActivePane = PaneNodes
				m.SelectedNode = nodeCount + 1
				cmd := m.openBootstrapModal()
				return *m, cmd
			}
		} else {
			routeCount := len(m.Mesh.Routes)
			if yRel >= 2 && yRel < 2+routeCount {
				clickedIdx := yRel - 2
				if m.ActivePane == PaneRoutes && m.SelectedRoute == clickedIdx {
					cmd := m.openEditRouteModal()
					return *m, cmd
				} else {
					m.ActivePane = PaneRoutes
					m.SelectedRoute = clickedIdx
					m.Editor.RouteIdx = clickedIdx
				}
			} else if yRel == 2+routeCount {
				m.ActivePane = PaneRoutes
				m.SelectedRoute = routeCount
				cmd := m.openAddRouteModal()
				return *m, cmd
			}
		}
		return *m, nil
	}

	// C. Средняя нижняя секция (Clients & Lists)
	if y >= g.topHeight+g.midUpperHeight && y < g.topHeight+g.midUpperHeight+g.midLowerHeight {
		yRel := y - (g.topHeight + g.midUpperHeight) - 1

		if x < g.nodesWidth {
			clientCount := len(m.Mesh.Clients)
			if yRel >= 2 && yRel < 2+clientCount {
				clickedIdx := yRel - 2
				if m.ActivePane == PaneClients && m.SelectedClient == clickedIdx {
					cmd := m.openEditClientModal()
					return *m, cmd
				} else {
					m.ActivePane = PaneClients
					m.SelectedClient = clickedIdx
				}
			} else if yRel == 2+clientCount {
				m.ActivePane = PaneClients
				m.SelectedClient = clientCount
				cmd := m.openAddClientModal()
				return *m, cmd
			}
		} else {
			listCount := len(m.Mesh.Lists)
			if yRel >= 2 && yRel < 2+listCount {
				clickedIdx := yRel - 2
				if m.ActivePane == PaneLists && m.SelectedList == clickedIdx {
					cmd := m.openEditListModal()
					return *m, cmd
				} else {
					m.ActivePane = PaneLists
					m.SelectedList = clickedIdx
				}
			} else if yRel == 2+listCount {
				m.ActivePane = PaneLists
				m.SelectedList = listCount
				cmd := m.openAddListModal()
				return *m, cmd
			}
		}
		return *m, nil
	}

	// D. Панель Редактора Маршрута (PaneEditor)
	if y >= g.topHeight+g.midUpperHeight+g.midLowerHeight && y < g.topHeight+g.midUpperHeight+g.midLowerHeight+g.editorHeight {
		m.ActivePane = PaneEditor
		yRel := y - (g.topHeight + g.midUpperHeight + g.midLowerHeight) - 1

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
				cmd := m.openEditRouteModal()
				return *m, cmd
			}
		}
		return *m, nil
	}

	// E. Нижняя статусная строка (Status Bar)
	if y >= g.statusBarTop {
		yRelBar := y - g.statusBarTop
		cmd := m.handleStatusBarClick(x, yRelBar)
		return *m, cmd
	}

	return *m, nil
}

func (m *Model) handleStatusBarClick(x, yRel int) tea.Cmd {
	if yRel == 0 {
		if m.IsDirty {
			if err := config.Save(m.ConfigPath, m.Mesh); err != nil {
				m.LogMsg = i18n.T("log_save_err", err)
			} else {
				m.IsDirty = false
				m.LogMsg = i18n.T("log_saved", m.ConfigPath)
			}
		}
		return nil
	}

	switch {
	case x <= 9: // [s] Save
		if err := config.Save(m.ConfigPath, m.Mesh); err != nil {
			m.LogMsg = i18n.T("log_save_err", err)
		} else {
			m.IsDirty = false
			m.LogMsg = i18n.T("log_saved", m.ConfigPath)
		}
	case x > 9 && x <= 21: // [a] Apply
		m.LogMsg = i18n.T("log_applying")
		m.runApply()
	case x > 21 && x <= 37: // [b] Bootstrap
		return m.openBootstrapModal()
	case x > 37 && x <= 50: // [d] Doctor
		return m.runDoctor()
	case x > 50 && x <= 63: // [x] Export
		if len(m.Mesh.Routes) > 0 && m.SelectedRoute < len(m.Mesh.Routes) {
			return m.handleExportFormat("w")
		} else {
			m.LogMsg = i18n.T("log_route_protected")
		}
	case x > 63 && x <= 73: // [g] Git
		return m.openGitModal()
	case x > 73 && x <= 86: // [u] Update
		return m.openUpdateModal()
	case x > 86 && x <= 97: // [?] Help
		return m.openModal(ModalState{Type: ModalHelp})
	}
	return nil
}

func (m *Model) handleModalMouseClick(x, y int) (Model, tea.Cmd) {
	modalBody := renderModalOverlay(*m)
	mWidth := lipgloss.Width(modalBody)
	mHeight := lipgloss.Height(modalBody)

	top := (m.Height - mHeight) / 2
	left := (m.Width - mWidth) / 2

	if x < left || x >= left+mWidth || y < top || y >= top+mHeight {
		// Клик мимо рамок модального окна больше не закрывает окно без сохранения!
		return *m, nil
	}

	yRel := y - top - 1
	xRel := x - left - 2

	switch m.Modal.Type {
	case ModalHelp, ModalCapabilities:
		cmd := m.closeModal()
		return *m, cmd

	case ModalDoctor:
		if yRel >= mHeight-3 {
			if xRel < 25 {
				m.runDoctor()
			} else {
				cmd := m.closeModal()
				return *m, cmd
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
				cmd := m.closeModal()
				return *m, cmd
			}
		}

	case ModalConfirm:
		if yRel >= 3 {
			if xRel < 30 {
				if m.Modal.OnConfirm != nil {
					m.Modal.OnConfirm(m)
				}
				cmd := m.closeModal()
				return *m, cmd
			} else {
				cmd := m.closeModal()
				return *m, cmd
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
					cmd := m.submitCurrentModal()
					return *m, cmd
				} else {
					cmd := m.closeModal()
					return *m, cmd
				}
			}
		}
	}

	return *m, nil
}

