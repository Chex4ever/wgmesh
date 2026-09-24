package tui

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/pem"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/skip2/go-qrcode"
	"golang.org/x/crypto/ssh"

	"github.com/meshctl/meshctl/internal/config"
	"github.com/meshctl/meshctl/internal/drivers"
	"github.com/meshctl/meshctl/internal/export"
	"github.com/meshctl/meshctl/internal/mesh"
	"github.com/meshctl/meshctl/internal/wg"
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
	ModalBootstrapNode
	ModalAddRoute
	ModalAddClient
	ModalAddList
	ModalConfirm
	ModalDoctor
	ModalExport
	ModalCapabilities
	ModalGit
)

type FormField struct {
	Label       string
	Value       string
	Placeholder string
	Mask        bool
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
}

type Model struct {
	Mesh       *config.Mesh
	ConfigPath string
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

	LogMsg   string
	Applying bool
}

func NewModel(m *config.Mesh, configPath string) Model {
	return Model{
		Mesh:           m,
		ConfigPath:     configPath,
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
		key := msg.String()

		// 1. Если открыто модальное окно — обработка модального ввода
		if m.Modal.Type != ModalNone {
			return m.handleModalKey(key)
		}

		// 2. Глобальные и панельные горячие клавиши
		switch key {
		case "q", "ctrl+c":
			return m, tea.Quit

		case "?":
			m.Modal = ModalState{Type: ModalHelp}
			return m, nil

		case "tab":
			m.ActivePane = (m.ActivePane + 1) % 6

		case "shift+tab":
			m.ActivePane = (m.ActivePane + 5) % 6

		case "s":
			if err := config.Save(m.ConfigPath, m.Mesh); err != nil {
				m.LogMsg = fmt.Sprintf("Ошибка сохранения: %v", err)
			} else {
				m.IsDirty = false
				m.LogMsg = fmt.Sprintf("✔ Конфигурация успешно сохранена в %s", m.ConfigPath)
			}

		case "a":
			m.LogMsg = "→ Запуск полного применения (Apply) по SSH…"
			m.runApply()

		case "d":
			m.runDoctor()

		case "b":
			m.openBootstrapModal()

		case "n":
			m.openAddNodeModal()

		case "r":
			m.openAddRouteModal()

		case "c":
			m.openAddClientModal()

		case "l":
			m.openAddListModal()

		case "e":
			m.openEditRouteModal()

		case "x", "delete":
			m.handleDeleteCurrent()

		case "t":
			m.handleTeardownNode()

		case "k":
			m.handleCapabilities()

		case "g":
			m.openGitModal()

		case "up":
			m.moveSelection(-1)

		case "down":
			m.moveSelection(1)

		case "+", "=":
			m.addHopToSelectedRoute()

		case "-", "_":
			m.removeHopFromSelectedRoute()
		}
	}

	return m, nil
}

func (m *Model) moveSelection(delta int) {
	switch m.ActivePane {
	case PaneTopology, PaneRoutes:
		if len(m.Mesh.Routes) > 0 {
			m.SelectedRoute = (m.SelectedRoute + delta + len(m.Mesh.Routes)) % len(m.Mesh.Routes)
			m.Editor.RouteIdx = m.SelectedRoute
		}
	case PaneNodes:
		if len(m.Mesh.Nodes) > 0 {
			m.SelectedNode = (m.SelectedNode + delta + len(m.Mesh.Nodes)) % len(m.Mesh.Nodes)
		}
	case PaneClients:
		if len(m.Mesh.Clients) > 0 {
			m.SelectedClient = (m.SelectedClient + delta + len(m.Mesh.Clients)) % len(m.Mesh.Clients)
		}
	case PaneLists:
		if len(m.Mesh.Lists) > 0 {
			m.SelectedList = (m.SelectedList + delta + len(m.Mesh.Lists)) % len(m.Mesh.Lists)
		}
	}
}

func (m *Model) addHopToSelectedRoute() {
	if len(m.Mesh.Routes) > 0 && len(m.Mesh.Nodes) > 0 {
		r := &m.Mesh.Routes[m.SelectedRoute]
		if r.Protected {
			m.LogMsg = fmt.Sprintf("⚠️ Маршрут %q защищён (protected: true) — редактирование запрещено", r.Name)
			return
		}
		nodeToAdd := m.Mesh.Nodes[0].Name
		r.Path = append(r.Path, nodeToAdd)
		r.ExitNode = nodeToAdd
		m.IsDirty = true
		m.LogMsg = fmt.Sprintf("✔ Добавлен хоп %s в маршрут %s", nodeToAdd, r.Name)
	}
}

func (m *Model) removeHopFromSelectedRoute() {
	if len(m.Mesh.Routes) > 0 {
		r := &m.Mesh.Routes[m.SelectedRoute]
		if r.Protected {
			m.LogMsg = fmt.Sprintf("⚠️ Маршрут %q защищён (protected: true) — редактирование запрещено", r.Name)
			return
		}
		if len(r.Path) > 2 {
			removed := r.Path[len(r.Path)-1]
			r.Path = r.Path[:len(r.Path)-1]
			r.ExitNode = r.Path[len(r.Path)-1]
			m.IsDirty = true
			m.LogMsg = fmt.Sprintf("✔ Удалён хоп %s из маршрута %s", removed, r.Name)
		}
	}
}

func (m *Model) handleModalKey(key string) (Model, tea.Cmd) {
	switch key {
	case "esc":
		m.Modal = ModalState{Type: ModalNone}
		return *m, nil

	case "y", "Y":
		if m.Modal.Type == ModalConfirm && m.Modal.OnConfirm != nil {
			m.Modal.OnConfirm(m)
			m.Modal = ModalState{Type: ModalNone}
		}
		return *m, nil

	case "n", "N":
		if m.Modal.Type == ModalConfirm {
			m.Modal = ModalState{Type: ModalNone}
		}

	case "tab", "down":
		if len(m.Modal.Fields) > 0 {
			m.Modal.ActiveField = (m.Modal.ActiveField + 1) % len(m.Modal.Fields)
		}

	case "shift+tab", "up":
		if len(m.Modal.Fields) > 0 {
			m.Modal.ActiveField = (m.Modal.ActiveField + len(m.Modal.Fields) - 1) % len(m.Modal.Fields)
		}

	case "backspace":
		if len(m.Modal.Fields) > 0 {
			f := &m.Modal.Fields[m.Modal.ActiveField]
			if len(f.Value) > 0 {
				f.Value = f.Value[:len(f.Value)-1]
			}
		}

	case "enter":
		m.submitCurrentModal()

	case "w", "a", "u", "q":
		if m.Modal.Type == ModalExport {
			m.handleExportFormat(key)
		}

	default:
		if len(key) == 1 && len(m.Modal.Fields) > 0 {
			f := &m.Modal.Fields[m.Modal.ActiveField]
			f.Value += key
		}
	}

	return *m, nil
}

func (m *Model) submitCurrentModal() {
	switch m.Modal.Type {
	case ModalBootstrapNode:
		name := m.getFieldValue("Имя ноды")
		host := m.getFieldValue("IP / Хост")
		pass := m.getFieldValue("Пароль SSH")
		user := m.getFieldValue("SSH Пользователь")
		nType := m.getFieldValue("Тип ноды")
		protStr := strings.ToLower(m.getFieldValue("Защита (y/n)"))

		if name == "" || host == "" {
			m.LogMsg = "Ошибка: укажите имя и IP ноды"
			return
		}
		if user == "" {
			user = "root"
		}
		if nType == "" {
			nType = config.TypeLinux
		}

		keyPath, pubStr, err := ensureDefaultSSHKeyTUI()
		if err != nil {
			m.LogMsg = fmt.Sprintf("Ошибка ключа SSH: %v", err)
			return
		}

		if err := installRemoteKeyTUI(host, 22, user, pass, keyPath, pubStr, nType); err != nil {
			m.LogMsg = fmt.Sprintf("Ошибка провижининга SSH: %v", err)
			return
		}

		isProt := protStr == "y" || protStr == "yes"
		m.Mesh.Nodes = append(m.Mesh.Nodes, config.Node{
			Name:      name,
			Type:      nType,
			Host:      host,
			SSHUser:   user,
			SSHKey:    "~/.config/wgmesh/keys/id_ed25519",
			SSHPort:   22,
			Protected: isProt,
			WireGuard: config.WG{Interface: "wg0", ListenPort: 51820},
		})
		m.IsDirty = true
		_ = config.Save(m.ConfigPath, m.Mesh)
		m.LogMsg = fmt.Sprintf("✔ Нода %q успешно подключена по SSH ключу и добавлена!", name)
		m.Modal = ModalState{Type: ModalNone}

	case ModalAddNode:
		name := m.getFieldValue("Имя ноды")
		host := m.getFieldValue("IP / Хост")
		nType := m.getFieldValue("Тип ноды")
		user := m.getFieldValue("SSH Пользователь")
		protStr := strings.ToLower(m.getFieldValue("Защита (y/n)"))
		if name != "" && host != "" {
			if nType == "" {
				nType = config.TypeLinux
			}
			if user == "" {
				user = "root"
			}
			m.Mesh.Nodes = append(m.Mesh.Nodes, config.Node{
				Name:      name,
				Type:      nType,
				Host:      host,
				SSHUser:   user,
				SSHPort:   22,
				Protected: protStr == "y" || protStr == "yes",
				WireGuard: config.WG{Interface: "wg0", ListenPort: 51820},
			})
			m.IsDirty = true
			_ = config.Save(m.ConfigPath, m.Mesh)
			m.LogMsg = fmt.Sprintf("✔ Нода %q добавлена", name)
		}
		m.Modal = ModalState{Type: ModalNone}

	case ModalAddRoute:
		name := m.getFieldValue("Имя маршрута")
		pathStr := m.getFieldValue("Путь (через запятую)")
		exit := m.getFieldValue("Exit нода")
		protStr := strings.ToLower(m.getFieldValue("Защита (y/n)"))
		if name != "" && pathStr != "" {
			hops := splitTrimTUI(pathStr)
			if len(hops) > 0 && hops[0] != config.ClientHop {
				hops = append([]string{config.ClientHop}, hops...)
			}
			if exit == "" && len(hops) > 1 {
				exit = hops[len(hops)-1]
			}
			r := config.Route{
				Name:      name,
				Path:      hops,
				ExitNode:  exit,
				Protected: protStr == "y" || protStr == "yes",
			}
			m.Mesh.Routes = append(m.Mesh.Routes, r)
			m.IsDirty = true
			_ = config.Save(m.ConfigPath, m.Mesh)
			m.LogMsg = fmt.Sprintf("✔ Маршрут %q создан", name)
		}
		m.Modal = ModalState{Type: ModalNone}

	case ModalGit:
		msg := m.getFieldValue("Сообщение коммита")
		if msg == "" {
			msg = "update meshctl configuration"
		}
		out, err := exec.Command("git", "commit", "-am", msg).CombinedOutput()
		if err != nil {
			m.LogMsg = fmt.Sprintf("Git commit ошибка: %v (%s)", err, string(out))
		} else {
			outPush, errP := exec.Command("git", "push").CombinedOutput()
			if errP != nil {
				m.LogMsg = fmt.Sprintf("✔ Коммит создан. Git push ошибка: %v (%s)", errP, string(outPush))
			} else {
				m.LogMsg = "✔ Изменения успешно закоммичены и отправлены в Git (push)!"
			}
		}
		m.Modal = ModalState{Type: ModalNone}
	}
}

func (m *Model) getFieldValue(label string) string {
	for _, f := range m.Modal.Fields {
		if f.Label == label {
			return strings.TrimSpace(f.Value)
		}
	}
	return ""
}

func (m *Model) openBootstrapModal() {
	m.Modal = ModalState{
		Type: ModalBootstrapNode,
		Fields: []FormField{
			{Label: "Имя ноды", Placeholder: "kz-server"},
			{Label: "IP / Хост", Placeholder: "109.248.198.55"},
			{Label: "Пароль SSH", Mask: true},
			{Label: "SSH Пользователь", Value: "root"},
			{Label: "Тип ноды", Value: "linux"},
			{Label: "Защита (y/n)", Value: "n"},
		},
	}
}

func (m *Model) openAddNodeModal() {
	m.Modal = ModalState{
		Type: ModalAddNode,
		Fields: []FormField{
			{Label: "Имя ноды", Placeholder: "de-server"},
			{Label: "IP / Хост", Placeholder: "194.87.71.7"},
			{Label: "Тип ноды", Value: "linux"},
			{Label: "SSH Пользователь", Value: "root"},
			{Label: "Защита (y/n)", Value: "n"},
		},
	}
}

func (m *Model) openAddRouteModal() {
	m.Modal = ModalState{
		Type: ModalAddRoute,
		Fields: []FormField{
			{Label: "Имя маршрута", Placeholder: "via-kz-de"},
			{Label: "Путь (через запятую)", Placeholder: "client,kz-server,de-server"},
			{Label: "Exit нода", Placeholder: "de-server"},
			{Label: "Защита (y/n)", Value: "n"},
		},
	}
}

func (m *Model) openAddClientModal() {
	m.Modal = ModalState{
		Type: ModalAddClient,
		Fields: []FormField{
			{Label: "Имя клиента", Placeholder: "alice-phone"},
			{Label: "Ingress Нода", Placeholder: "kz-server"},
		},
	}
}

func (m *Model) openAddListModal() {
	m.Modal = ModalState{
		Type: ModalAddList,
		Fields: []FormField{
			{Label: "Имя списка", Placeholder: "youtube-list"},
			{Label: "Домены (через запятую)", Placeholder: "youtube.com, *.googlevideo.com"},
		},
	}
}

func (m *Model) openEditRouteModal() {
	if len(m.Mesh.Routes) == 0 || m.SelectedRoute >= len(m.Mesh.Routes) {
		return
	}
	r := &m.Mesh.Routes[m.SelectedRoute]
	if r.Protected {
		m.Modal = ModalState{
			Type:          ModalConfirm,
			ConfirmPrompt: fmt.Sprintf("Маршрут %q помечен как защищённый (protected: true)! Вы действительно хотите его отредактировать?", r.Name),
			OnConfirm: func(mod *Model) {
				mod.openAddRouteModal()
			},
		}
		return
	}
	m.openAddRouteModal()
}

func (m *Model) handleDeleteCurrent() {
	switch m.ActivePane {
	case PaneNodes:
		if len(m.Mesh.Nodes) == 0 || m.SelectedNode >= len(m.Mesh.Nodes) {
			return
		}
		node := &m.Mesh.Nodes[m.SelectedNode]
		idx := m.SelectedNode
		if node.Protected {
			m.Modal = ModalState{
				Type:          ModalConfirm,
				ConfirmPrompt: fmt.Sprintf("Нода %q помечена как защищённая (protected: true)! Вы действительно хотите её удалить?", node.Name),
				OnConfirm: func(mod *Model) {
					mod.removeNodeByIdx(idx)
				},
			}
			return
		}
		m.removeNodeByIdx(idx)

	case PaneRoutes:
		if len(m.Mesh.Routes) == 0 || m.SelectedRoute >= len(m.Mesh.Routes) {
			return
		}
		r := &m.Mesh.Routes[m.SelectedRoute]
		idx := m.SelectedRoute
		if r.Protected {
			m.Modal = ModalState{
				Type:          ModalConfirm,
				ConfirmPrompt: fmt.Sprintf("Маршрут %q помечен как защищённый (protected: true)! Вы действительно хотите его удалить?", r.Name),
				OnConfirm: func(mod *Model) {
					mod.removeRouteByIdx(idx)
				},
			}
			return
		}
		m.removeRouteByIdx(idx)

	case PaneClients:
		if len(m.Mesh.Clients) > 0 && m.SelectedClient < len(m.Mesh.Clients) {
			cName := m.Mesh.Clients[m.SelectedClient].Name
			m.Mesh.Clients = append(m.Mesh.Clients[:m.SelectedClient], m.Mesh.Clients[m.SelectedClient+1:]...)
			m.IsDirty = true
			_ = config.Save(m.ConfigPath, m.Mesh)
			m.LogMsg = fmt.Sprintf("✔ Клиент %q удалён", cName)
		}

	case PaneLists:
		if len(m.Mesh.Lists) > 0 && m.SelectedList < len(m.Mesh.Lists) {
			lName := m.Mesh.Lists[m.SelectedList].Name
			m.Mesh.Lists = append(m.Mesh.Lists[:m.SelectedList], m.Mesh.Lists[m.SelectedList+1:]...)
			m.IsDirty = true
			_ = config.Save(m.ConfigPath, m.Mesh)
			m.LogMsg = fmt.Sprintf("✔ Список %q удалён", lName)
		}
	}
}

func (m *Model) removeNodeByIdx(idx int) {
	if idx < len(m.Mesh.Nodes) {
		name := m.Mesh.Nodes[idx].Name
		m.Mesh.Nodes = append(m.Mesh.Nodes[:idx], m.Mesh.Nodes[idx+1:]...)
		m.IsDirty = true
		_ = config.Save(m.ConfigPath, m.Mesh)
		m.LogMsg = fmt.Sprintf("✔ Нода %q удалена", name)
		if m.SelectedNode > 0 {
			m.SelectedNode--
		}
	}
}

func (m *Model) removeRouteByIdx(idx int) {
	if idx < len(m.Mesh.Routes) {
		name := m.Mesh.Routes[idx].Name
		m.Mesh.Routes = append(m.Mesh.Routes[:idx], m.Mesh.Routes[idx+1:]...)
		m.IsDirty = true
		_ = config.Save(m.ConfigPath, m.Mesh)
		m.LogMsg = fmt.Sprintf("✔ Маршрут %q удалён", name)
		if m.SelectedRoute > 0 {
			m.SelectedRoute--
		}
	}
}

func (m *Model) handleTeardownNode() {
	if len(m.Mesh.Nodes) == 0 || m.SelectedNode >= len(m.Mesh.Nodes) {
		return
	}
	node := &m.Mesh.Nodes[m.SelectedNode]
	m.LogMsg = fmt.Sprintf("→ Удаляю WG-конфигурацию с %q (%s)…", node.Name, node.Host)
	d, err := drivers.New(node)
	if err != nil {
		m.LogMsg = fmt.Sprintf("Ошибка драйвера: %v", err)
		return
	}
	if err := d.RemoveConfig(node); err != nil {
		m.LogMsg = fmt.Sprintf("Ошибка teardown: %v", err)
	} else {
		m.LogMsg = fmt.Sprintf("✔ WG конфигурация с %q удалена!", node.Name)
	}
}

func (m *Model) handleCapabilities() {
	if len(m.Mesh.Nodes) == 0 || m.SelectedNode >= len(m.Mesh.Nodes) {
		return
	}
	node := &m.Mesh.Nodes[m.SelectedNode]
	caps := drivers.Capabilities(node.Type)
	m.Modal = ModalState{
		Type: ModalCapabilities,
		CapText: fmt.Sprintf(
			"Нода: %s (%s)\n\n"+
				"  - PSK Support:         %v\n"+
				"  - Keepalive Support:   %v\n"+
				"  - NAT/Masquerade:      %v\n"+
				"  - Multi-Route:         %v\n"+
				"  - AmneziaWG Obfusc:    %v\n"+
				"  - Auto Package Inst:   %v",
			node.Name, node.Type,
			caps.SupportsPSK, caps.SupportsKeepalive, caps.SupportsNAT,
			caps.SupportsMultiRoute, caps.SupportsObfuscation, caps.NeedsPackageInstall,
		),
	}
}

func (m *Model) runApply() {
	mgr := mesh.NewManager(m.Mesh)
	keys, err := mgr.EnsureKeys()
	if err != nil {
		m.LogMsg = fmt.Sprintf("Ошибка генерации ключей: %v", err)
		return
	}
	ips, err := mgr.EnsureIPs()
	if err != nil {
		m.LogMsg = fmt.Sprintf("Ошибка назначения IP: %v", err)
		return
	}
	if keys > 0 || ips > 0 {
		_ = config.Save(m.ConfigPath, m.Mesh)
	}

	if err := mesh.Validate(m.Mesh); err != nil {
		m.LogMsg = fmt.Sprintf("Ошибка валидации: %v", err)
		return
	}

	plans := mgr.BuildPlan()
	var failed []string
	for _, p := range plans {
		d, err := drivers.New(p.Node)
		if err != nil {
			failed = append(failed, p.Node.Name)
			continue
		}
		spec := &drivers.NodeApplySpec{
			Node:    p.Node,
			Config:  &wg.NodeConfig{Name: p.Node.Name, PrivateKey: p.Node.WireGuard.PrivateKey, Address: p.Node.MeshIP + "/24", ListenPort: p.Node.WireGuard.ListenPort},
			NAT:     p.IsExit,
			Forward: p.IsRelay || p.IsExit,
		}
		if err := d.ApplySpec(spec); err != nil {
			failed = append(failed, p.Node.Name)
		}
	}

	if len(failed) > 0 {
		m.LogMsg = fmt.Sprintf("✖ Ошибка применения на нодах: %v", failed)
	} else {
		m.LogMsg = "✔ Конфигурация успешно применена ко всем участвующим нодам по SSH!"
	}
}

func (m *Model) runDoctor() {
	var lines []string
	hasFail := false
	hasWarn := false

	// Config
	if err := mesh.Validate(m.Mesh); err != nil {
		lines = append(lines, fmt.Sprintf("[✖] Config       | Синтаксис и граф: %v", err))
		hasFail = true
	} else {
		lines = append(lines, fmt.Sprintf("[✔] Config       | Синтаксис и граф: ОК (%d нод, %d маршрутов)", len(m.Mesh.Nodes), len(m.Mesh.Routes)))
	}

	// Security
	var secretWarns []string
	for _, n := range m.Mesh.Nodes {
		if n.WireGuard.PrivateKey != "" {
			secretWarns = append(secretWarns, n.Name)
		}
	}
	if len(secretWarns) > 0 {
		lines = append(lines, fmt.Sprintf("[⚠️] Security     | Секреты в YAML: Приватный ключ сохранён на %s", strings.Join(secretWarns, ", ")))
		hasWarn = true
	} else {
		lines = append(lines, "[✔] Security     | Секреты в YAML: В mesh.yaml нет открытых приватных ключей")
	}

	// SSH reachability
	reachability := mesh.CheckReachability(m.Mesh, 3*time.Second)
	for _, n := range m.Mesh.Nodes {
		if rErr, failed := reachability[n.Name]; failed {
			lines = append(lines, fmt.Sprintf("[✖] SSH          | %s (%s): %v", n.Name, n.Host, rErr))
			hasFail = true
		} else {
			lines = append(lines, fmt.Sprintf("[✔] SSH          | %s (%s): TCP соединение установлено", n.Name, n.Host))
		}
	}

	status := "GREEN"
	if hasFail {
		status = "RED"
	} else if hasWarn {
		status = "YELLOW"
	}

	m.Modal = ModalState{
		Type:         ModalDoctor,
		DoctorOutput: lines,
		DoctorStatus: status,
	}
}

func (m *Model) handleExportFormat(fmtKey string) {
	if len(m.Mesh.Routes) == 0 || m.SelectedRoute >= len(m.Mesh.Routes) {
		return
	}
	r := &m.Mesh.Routes[m.SelectedRoute]
	var c *config.Client
	if len(m.Mesh.Clients) > 0 && m.SelectedClient < len(m.Mesh.Clients) {
		c = &m.Mesh.Clients[m.SelectedClient]
	}

	fmtName := "wireguard"
	showQR := false
	switch fmtKey {
	case "a":
		fmtName = "amnezia"
	case "s":
		fmtName = "sing-box"
	case "u":
		fmtName = "uri"
	case "q":
		fmtName = "wireguard"
		showQR = true
	}

	exp, err := export.Get(fmtName)
	if err != nil {
		m.LogMsg = fmt.Sprintf("Ошибка экспорта: %v", err)
		return
	}
	data, err := exp.Render(m.Mesh, r, c)
	if err != nil {
		m.LogMsg = fmt.Sprintf("Ошибка рендеринга профиля: %v", err)
		return
	}

	qrStr := ""
	if showQR {
		qr, err := qrcode.New(string(data), qrcode.Medium)
		if err == nil {
			qrStr = qr.ToSmallString(false)
		}
	}

	m.Modal = ModalState{
		Type:       ModalExport,
		ExportData: string(data),
		ShowQR:     showQR,
		QRString:   qrStr,
	}
}

func (m *Model) openGitModal() {
	out, _ := exec.Command("git", "status", "-s").CombinedOutput()
	m.Modal = ModalState{
		Type:      ModalGit,
		GitStatus: string(out),
		Fields: []FormField{
			{Label: "Сообщение коммита", Value: "update mesh configuration"},
		},
	}
}

func (m Model) View() string {
	if m.Modal.Type != ModalNone {
		return renderModalOverlay(m)
	}

	header := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("86")).
		Render(fmt.Sprintf("═══ %s (wgmesh TUI) ═══", m.Mesh.Name))

	topView := RenderTopology(m.Mesh, m.SelectedRoute, m.Width)

	topBox := paneStyle.Width(m.Width - 4).Render(
		header + "\n\n" + topView,
	)

	nodesView := RenderNodesPane(m.Mesh, m.SelectedNode, m.ActivePane == PaneNodes, m.Height)
	routesView := RenderRoutesPane(m.Mesh, m.SelectedRoute, m.ActivePane == PaneRoutes, m.Height)
	clientsView := RenderClientsPane(m.Mesh, m.SelectedClient, m.ActivePane == PaneClients, m.Height)
	listsView := RenderListsPane(m.Mesh, m.SelectedList, m.ActivePane == PaneLists, m.Height)
	editorView := RenderEditorPane(m.Mesh, &m.Editor, m.ActivePane == PaneEditor)

	middleUpper := lipgloss.JoinHorizontal(
		lipgloss.Top,
		nodesView,
		routesView,
	)

	middleLower := lipgloss.JoinHorizontal(
		lipgloss.Top,
		clientsView,
		listsView,
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
		middleUpper,
		middleLower,
		editorView,
		bottomLog,
		statusBar,
	)
}

func fmtPrintfStr(format string, a ...interface{}) string {
	return fmt.Sprintf(format, a...)
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

func ensureDefaultSSHKeyTUI() (string, string, error) {
	keyPath := expandHomeTUI("~/.config/wgmesh/keys/id_ed25519")
	pubPath := keyPath + ".pub"

	if _, err := os.Stat(keyPath); err == nil {
		pubData, err := os.ReadFile(pubPath)
		if err == nil {
			return keyPath, strings.TrimSpace(string(pubData)), nil
		}
	}

	dir := filepath.Dir(keyPath)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return "", "", err
	}

	pubKey, privKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return "", "", err
	}

	sshPrivBlock, err := ssh.MarshalPrivateKey(privKey, "")
	if err != nil {
		return "", "", err
	}

	if err := os.WriteFile(keyPath, pem.EncodeToMemory(sshPrivBlock), 0600); err != nil {
		return "", "", err
	}

	sshPubKey, err := ssh.NewPublicKey(pubKey)
	if err != nil {
		return "", "", err
	}

	pubBytes := ssh.MarshalAuthorizedKey(sshPubKey)
	pubStr := strings.TrimSpace(string(pubBytes))
	_ = os.WriteFile(pubPath, pubBytes, 0644)

	return keyPath, pubStr, nil
}

func installRemoteKeyTUI(host string, port int, user, password, keyPath, pubKeyStr, nType string) error {
	var authMethods []ssh.AuthMethod
	if password != "" {
		authMethods = append(authMethods, ssh.Password(password))
	}
	if keyData, err := os.ReadFile(keyPath); err == nil {
		if signer, err := ssh.ParsePrivateKey(keyData); err == nil {
			authMethods = append(authMethods, ssh.PublicKeys(signer))
		}
	}

	if len(authMethods) == 0 {
		return fmt.Errorf("укажите пароль для подключения")
	}

	cfg := &ssh.ClientConfig{
		User:            user,
		Auth:            authMethods,
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         10 * time.Second,
	}

	addr := net.JoinHostPort(host, fmt.Sprint(port))
	client, err := ssh.Dial("tcp", addr, cfg)
	if err != nil {
		return err
	}
	defer client.Close()

	sess, err := client.NewSession()
	if err != nil {
		return err
	}
	defer sess.Close()

	cleanPubKey := strings.TrimSpace(pubKeyStr)
	switch nType {
	case config.TypeMikrotik:
		cmd := fmt.Sprintf("/file print file=id_ed25519.pub; /file set id_ed25519.pub contents=%q; /user ssh-keys import public-key-file=id_ed25519.pub user=%s", cleanPubKey, user)
		_, _ = sess.CombinedOutput(cmd)
	default:
		cmd := fmt.Sprintf("mkdir -p ~/.ssh && chmod 700 ~/.ssh && (grep -q -F %q ~/.ssh/authorized_keys 2>/dev/null || echo %q >> ~/.ssh/authorized_keys) && chmod 600 ~/.ssh/authorized_keys", cleanPubKey, cleanPubKey)
		_, err = sess.CombinedOutput(cmd)
		if err != nil {
			return err
		}
	}
	return nil
}

func expandHomeTUI(p string) string {
	if strings.HasPrefix(p, "~/") {
		if home, err := os.UserHomeDir(); err == nil {
			return filepath.Join(home, p[2:])
		}
	}
	return p
}
