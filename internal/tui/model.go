package tui

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/wgmesh/wgmesh/internal/config"
	"github.com/wgmesh/wgmesh/internal/i18n"
)

type Pane int

const (
	PaneTopology Pane = iota
	PaneNodes
	PaneRoutes
	PaneClients
	PaneLists
	PaneEditor
)

type ModalType int

const (
	ModalNone ModalType = iota
	ModalHelp
	ModalAddNode
	ModalEditNode
	ModalBootstrapNode
	ModalAddRoute
	ModalEditRoute
	ModalAddClient
	ModalEditClient
	ModalAddList
	ModalEditList
	ModalConfirm
	ModalDoctor
	ModalExport
	ModalCapabilities
	ModalGit
	ModalUpdate
)

type FormField struct {
	Label       string
	Value       string
	Placeholder string
	Mask        bool
	Options     []string
	OptionIdx   int
}

type ModalState struct {
	Type          ModalType
	Title         string
	Fields        []FormField
	ActiveField   int
	ConfirmPrompt string
	OnConfirm     func(*Model)
	DoctorOutput  []string
	DoctorStatus  string
	ExportData    string
	ShowQR        bool
	QRString      string
	GitStatus     string
	CapText       string
	UpdateStatus  string
	UpdateVersion string
	DownloadURL   string
	HasUpdate     bool
	IsUpdating    bool

	OriginX       int
	OriginY       int
	AnimStartTime time.Time
	AnimDuration  time.Duration
	IsAnimating   bool
	IsClosing     bool
	AnimProgress  float64
}

type Model struct {
	Mesh       *config.Mesh
	ConfigPath string
	Version    string
	IsDirty    bool
	ActivePane Pane

	SelectedNode   int
	SelectedRoute  int
	SelectedClient int
	SelectedList   int
	Editor         RouteEditor

	Modal ModalState

	Width  int
	Height int

	HoverX         int
	HoverY         int
	IsMousePressed bool

	LogMsg   string
	Applying bool
}

func NewModel(m *config.Mesh, configPath string, version ...string) Model {
	ver := "dev"
	if len(version) > 0 && version[0] != "" {
		ver = version[0]
	}
	return Model{
		Mesh:           m,
		ConfigPath:     configPath,
		Version:        ver,
		ActivePane:     PaneTopology,
		SelectedNode:   0,
		SelectedRoute:  0,
		SelectedClient: 0,
		SelectedList:   0,
		Editor: RouteEditor{
			RouteIdx:    0,
			SelectedHop: 0,
		},
		Width:  100,
		Height: 30,
		HoverX: -1,
		HoverY: -1,
	}
}

func (m Model) Init() tea.Cmd {
	return nil
}

type layoutGeometry struct {
	totalWidth     int
	colWidth       int
	paneHeight     int
	topHeight      int
	nodesWidth     int
	midUpperHeight int
	midLowerHeight int
	editorHeight   int
	padLines       int
	statusBarTop   int
}

func computeLayoutGeometry(m Model) layoutGeometry {
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
		Render(i18n.T("topology_title", m.Mesh.Name, m.Version))

	topView := RenderTopology(m.Mesh, m.SelectedRoute, totalWidth)
	topBox := paneStyle.Width(totalWidth).Render(header + "\n\n" + topView)
	topHeight := lipgloss.Height(topBox)

	editorView := RenderEditorPane(m.Mesh, &m.Editor, -1, -1, false, m.ActivePane == PaneEditor, totalWidth)
	editorHeight := lipgloss.Height(editorView)

	fixedH := topHeight + editorHeight + 3 // topBox + editorView + 1 log + 2 statusBar
	availForMiddle := m.Height - fixedH

	paneHeight := 6
	if availForMiddle >= 12 {
		paneHeight = (availForMiddle - 4) / 2
		if paneHeight < 5 {
			paneHeight = 5
		}
	}

	testNodesView := RenderNodesPane(m.Mesh, m.SelectedNode, -1, false, m.ActivePane == PaneNodes, colWidth, paneHeight)
	testRoutesView := RenderRoutesPane(m.Mesh, m.SelectedRoute, -1, false, m.ActivePane == PaneRoutes, colWidth, paneHeight)
	testClientsView := RenderClientsPane(m.Mesh, m.SelectedClient, -1, false, m.ActivePane == PaneClients, colWidth, paneHeight)
	testListsView := RenderListsPane(m.Mesh, m.SelectedList, -1, false, m.ActivePane == PaneLists, colWidth, paneHeight)

	midUpperTest := lipgloss.JoinHorizontal(lipgloss.Top, testNodesView, " ", testRoutesView)
	midLowerTest := lipgloss.JoinHorizontal(lipgloss.Top, testClientsView, " ", testListsView)

	nodesWidth := lipgloss.Width(testNodesView)
	midUpperHeight := lipgloss.Height(midUpperTest)
	midLowerHeight := lipgloss.Height(midLowerTest)

	contentH := topHeight + midUpperHeight + midLowerHeight + editorHeight + 3
	padLines := 0
	if m.Height > contentH {
		padLines = m.Height - contentH
	}

	statusBarTop := topHeight + midUpperHeight + midLowerHeight + editorHeight + padLines + 1

	return layoutGeometry{
		totalWidth:     totalWidth,
		colWidth:       colWidth,
		paneHeight:     paneHeight,
		topHeight:      topHeight,
		nodesWidth:     nodesWidth,
		midUpperHeight: midUpperHeight,
		midLowerHeight: midLowerHeight,
		editorHeight:   editorHeight,
		padLines:       padLines,
		statusBarTop:   statusBarTop,
	}
}

func (m Model) View() string {
	g := computeLayoutGeometry(m)

	header := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("86")).
		Render(i18n.T("topology_title", m.Mesh.Name, m.Version))

	topView := RenderTopology(m.Mesh, m.SelectedRoute, g.totalWidth)
	topBox := paneStyle.Width(g.totalWidth).Render(header + "\n\n" + topView)

	hoverNodesIdx := -1
	hoverRoutesIdx := -1
	hoverClientsIdx := -1
	hoverListsIdx := -1
	hoverHopIdx := -1
	hoverBtnIdx := -1
	hoverHintIdx := -1

	x := m.HoverX
	y := m.HoverY

	if x >= 0 && y >= 0 {
		if y >= g.topHeight && y < g.topHeight+g.midUpperHeight {
			yRel := y - g.topHeight - 1
			if x < g.nodesWidth {
				hoverNodesIdx = yRel - 2
			} else {
				hoverRoutesIdx = yRel - 2
			}
		} else if y >= g.topHeight+g.midUpperHeight && y < g.topHeight+g.midUpperHeight+g.midLowerHeight {
			yRel := y - (g.topHeight + g.midUpperHeight) - 1
			if x < g.nodesWidth {
				hoverClientsIdx = yRel - 2
			} else {
				hoverListsIdx = yRel - 2
			}
		} else if y >= g.topHeight+g.midUpperHeight+g.midLowerHeight && y < g.topHeight+g.midUpperHeight+g.midLowerHeight+g.editorHeight {
			yRel := y - (g.topHeight + g.midUpperHeight + g.midLowerHeight) - 1
			if yRel >= 3 && yRel <= 5 {
				if len(m.Mesh.Routes) > 0 && m.SelectedRoute < len(m.Mesh.Routes) {
					if x >= 3 {
						hoverHopIdx = (x - 3) / 15
					}
				}
			} else if yRel >= 6 {
				if x < 22 {
					hoverBtnIdx = 0
				} else if x < 42 {
					hoverBtnIdx = 1
				} else {
					hoverBtnIdx = 2
				}
			}
		} else {
			if y >= g.statusBarTop {
				yRelBar := y - g.statusBarTop
				if yRelBar == 1 {
					switch {
					case x <= 9:
						hoverHintIdx = 0
					case x <= 21:
						hoverHintIdx = 1
					case x <= 37:
						hoverHintIdx = 2
					case x <= 50:
						hoverHintIdx = 3
					case x <= 63:
						hoverHintIdx = 4
					case x <= 73:
						hoverHintIdx = 5
					case x <= 86:
						hoverHintIdx = 6
					case x <= 97:
						hoverHintIdx = 7
					default:
						hoverHintIdx = 8
					}
				}
			}
		}
	}

	nodesView := RenderNodesPane(m.Mesh, m.SelectedNode, hoverNodesIdx, m.IsMousePressed, m.ActivePane == PaneNodes, g.colWidth, g.paneHeight)
	routesView := RenderRoutesPane(m.Mesh, m.SelectedRoute, hoverRoutesIdx, m.IsMousePressed, m.ActivePane == PaneRoutes, g.colWidth, g.paneHeight)
	clientsView := RenderClientsPane(m.Mesh, m.SelectedClient, hoverClientsIdx, m.IsMousePressed, m.ActivePane == PaneClients, g.colWidth, g.paneHeight)
	listsView := RenderListsPane(m.Mesh, m.SelectedList, hoverListsIdx, m.IsMousePressed, m.ActivePane == PaneLists, g.colWidth, g.paneHeight)
	editorView := RenderEditorPane(m.Mesh, &m.Editor, hoverHopIdx, hoverBtnIdx, m.IsMousePressed, m.ActivePane == PaneEditor, g.totalWidth)

	middleUpper := lipgloss.JoinHorizontal(lipgloss.Top, nodesView, " ", routesView)
	middleLower := lipgloss.JoinHorizontal(lipgloss.Top, clientsView, " ", listsView)

	logText := m.LogMsg
	logColor := "214"
	if logText == "" {
		logText = i18n.T("log_idle")
		logColor = "244"
	}
	bottomLog := lipgloss.NewStyle().
		Foreground(lipgloss.Color(logColor)).
		Render(fmt.Sprintf(" Лог: %s", logText))

	statusBar := RenderStatusBar(m.Mesh, m.ConfigPath, m.IsDirty, "", hoverHintIdx, m.IsMousePressed, m.Width)

	var viewElements []string
	viewElements = append(viewElements, topBox, middleUpper, middleLower, editorView)
	if g.padLines > 0 {
		spacer := strings.Repeat("\n", g.padLines-1)
		viewElements = append(viewElements, spacer)
	}
	viewElements = append(viewElements, bottomLog, statusBar)

	mainView := lipgloss.JoinVertical(lipgloss.Left, viewElements...)

	return renderAnimatedModalOverlay(m, mainView)
}

func (m *Model) openModal(state ModalState) tea.Cmd {
	origX := m.HoverX
	origY := m.HoverY
	if origX <= 0 || origY <= 0 {
		origX = m.Width / 2
		origY = m.Height / 2
	}

	state.OriginX = origX
	state.OriginY = origY
	state.AnimStartTime = time.Now()
	state.AnimDuration = 250 * time.Millisecond
	state.IsAnimating = true
	state.AnimProgress = 0.0

	m.Modal = state
	return animateModalCmd()
}

func (m *Model) closeModal() tea.Cmd {
	if m.Modal.Type == ModalNone || m.Modal.IsClosing {
		return nil
	}
	m.Modal.IsClosing = true
	m.Modal.IsAnimating = true
	m.Modal.AnimStartTime = time.Now()
	m.Modal.AnimDuration = 250 * time.Millisecond
	m.Modal.AnimProgress = 0.0
	return animateModalCmd()
}

func splitTrimTUI(s string) []string {
	var out []string
	for _, p := range strings.Split(s, ",") {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

