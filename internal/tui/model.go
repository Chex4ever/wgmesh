package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/meshctl/meshctl/internal/config"
	"github.com/meshctl/meshctl/internal/i18n"
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

func (m Model) View() string {
	if m.Modal.Type != ModalNone {
		return renderModalOverlay(m)
	}

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

	topBox := paneStyle.Width(totalWidth).Render(
		header + "\n\n" + topView,
	)

	paneHeight := 6
	testNodesView := RenderNodesPane(m.Mesh, m.SelectedNode, -1, false, m.ActivePane == PaneNodes, colWidth, paneHeight)
	testRoutesView := RenderRoutesPane(m.Mesh, m.SelectedRoute, -1, false, m.ActivePane == PaneRoutes, colWidth, paneHeight)
	testClientsView := RenderClientsPane(m.Mesh, m.SelectedClient, -1, false, m.ActivePane == PaneClients, colWidth, paneHeight)
	testListsView := RenderListsPane(m.Mesh, m.SelectedList, -1, false, m.ActivePane == PaneLists, colWidth, paneHeight)
	testEditorView := RenderEditorPane(m.Mesh, &m.Editor, -1, -1, false, m.ActivePane == PaneEditor, totalWidth)

	midUpperTest := lipgloss.JoinHorizontal(lipgloss.Top, testNodesView, " ", testRoutesView)
	midLowerTest := lipgloss.JoinHorizontal(lipgloss.Top, testClientsView, " ", testListsView)

	topHeight := lipgloss.Height(topBox)
	nodesWidth := lipgloss.Width(testNodesView)
	midUpperHeight := lipgloss.Height(midUpperTest)
	midLowerHeight := lipgloss.Height(midLowerTest)
	editorHeight := lipgloss.Height(testEditorView)

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
		if y >= topHeight && y < topHeight+midUpperHeight {
			yRel := y - topHeight - 1
			if x < nodesWidth {
				hoverNodesIdx = yRel - 2
			} else {
				hoverRoutesIdx = yRel - 2
			}
		} else if y >= topHeight+midUpperHeight && y < topHeight+midUpperHeight+midLowerHeight {
			yRel := y - (topHeight + midUpperHeight) - 1
			if x < nodesWidth {
				hoverClientsIdx = yRel - 2
			} else {
				hoverListsIdx = yRel - 2
			}
		} else if y >= topHeight+midUpperHeight+midLowerHeight && y < topHeight+midUpperHeight+midLowerHeight+editorHeight {
			yRel := y - (topHeight + midUpperHeight + midLowerHeight) - 1
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
		} else if y >= topHeight+midUpperHeight+midLowerHeight+editorHeight {
			yRelBar := y - (topHeight + midUpperHeight + midLowerHeight + editorHeight)
			if yRelBar == 1 || yRelBar == 2 {
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

	nodesView := RenderNodesPane(m.Mesh, m.SelectedNode, hoverNodesIdx, m.IsMousePressed, m.ActivePane == PaneNodes, colWidth, paneHeight)
	routesView := RenderRoutesPane(m.Mesh, m.SelectedRoute, hoverRoutesIdx, m.IsMousePressed, m.ActivePane == PaneRoutes, colWidth, paneHeight)
	clientsView := RenderClientsPane(m.Mesh, m.SelectedClient, hoverClientsIdx, m.IsMousePressed, m.ActivePane == PaneClients, colWidth, paneHeight)
	listsView := RenderListsPane(m.Mesh, m.SelectedList, hoverListsIdx, m.IsMousePressed, m.ActivePane == PaneLists, colWidth, paneHeight)
	editorView := RenderEditorPane(m.Mesh, &m.Editor, hoverHopIdx, hoverBtnIdx, m.IsMousePressed, m.ActivePane == PaneEditor, totalWidth)

	middleUpper := lipgloss.JoinHorizontal(
		lipgloss.Top,
		nodesView,
		" ",
		routesView,
	)

	middleLower := lipgloss.JoinHorizontal(
		lipgloss.Top,
		clientsView,
		" ",
		listsView,
	)

	bottomLog := ""
	if m.LogMsg != "" {
		bottomLog = lipgloss.NewStyle().
			Foreground(lipgloss.Color("214")).
			Render(fmt.Sprintf(" Лог: %s", m.LogMsg)) + "\n"
	}

	statusBar := RenderStatusBar(m.Mesh, m.ConfigPath, m.IsDirty, "", hoverHintIdx, m.IsMousePressed, m.Width)

	return lipgloss.JoinVertical(
		lipgloss.Left,
		topBox,
		middleUpper,
		middleLower,
		editorView,
		bottomLog,
		statusBar,
	)
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
