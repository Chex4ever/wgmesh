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
	"github.com/meshctl/meshctl/internal/updater"
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

	HoverX int
	HoverY int

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

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.Width = msg.Width
		m.Height = msg.Height

	case tea.MouseMsg:
		return m.handleMouseMsg(msg)

	case tea.KeyMsg:
		rawKey := msg.String()

		// 1. Если открыто модальное окно — обработка модального ввода
		if m.Modal.Type != ModalNone {
			return m.handleModalKey(rawKey)
		}

		// 2. Нормализация русской раскладки в латинские хоткеи
		key := normalizeKey(rawKey)

		// 3. Глобальные и панельные горячие клавиши
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

		case "g":
			m.openGitModal()

		case "u":
			m.openUpdateModal()

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
			switch m.ActivePane {
			case PaneNodes:
				m.openEditNodeModal()
			case PaneRoutes, PaneEditor, PaneTopology:
				m.openEditRouteModal()
			case PaneClients:
				m.openEditClientModal()
			case PaneLists:
				m.openEditListModal()
			}

		case "enter", "space":
			switch m.ActivePane {
			case PaneNodes:
				if m.SelectedNode == len(m.Mesh.Nodes) {
					m.openAddNodeModal()
				} else if m.SelectedNode == len(m.Mesh.Nodes)+1 {
					m.openBootstrapModal()
				} else {
					m.openEditNodeModal()
				}
			case PaneRoutes:
				if m.SelectedRoute == len(m.Mesh.Routes) {
					m.openAddRouteModal()
				} else {
					m.openEditRouteModal()
				}
			case PaneClients:
				if m.SelectedClient == len(m.Mesh.Clients) {
					m.openAddClientModal()
				} else {
					m.openEditClientModal()
				}
			case PaneLists:
				if m.SelectedList == len(m.Mesh.Lists) {
					m.openAddListModal()
				} else {
					m.openEditListModal()
				}
			}

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

func normalizeKey(key string) string {
	cyrToLat := map[rune]rune{
		'й': 'q', 'ц': 'w', 'у': 'e', 'к': 'r', 'е': 't', 'н': 'y', 'г': 'u', 'ш': 'i', 'щ': 'o', 'з': 'p',
		'ф': 'a', 'ы': 's', 'в': 'd', 'а': 'f', 'п': 'g', 'р': 'h', 'о': 'j', 'л': 'k', 'д': 'l',
		'я': 'z', 'ч': 'x', 'с': 'c', 'м': 'v', 'и': 'b', 'т': 'n', 'ь': 'm',
		'Й': 'q', 'Ц': 'w', 'У': 'e', 'К': 'r', 'Е': 't', 'Н': 'y', 'Г': 'u', 'Ш': 'i', 'Щ': 'o', 'З': 'p',
		'Ф': 'a', 'Ы': 's', 'В': 'd', 'А': 'f', 'П': 'g', 'Р': 'h', 'О': 'j', 'Л': 'k', 'Д': 'l',
		'Я': 'z', 'Ч': 'x', 'С': 'c', 'М': 'v', 'И': 'b', 'Т': 'n', 'Ь': 'm',
	}

	runes := []rune(key)
	if len(runes) == 1 {
		if lat, ok := cyrToLat[runes[0]]; ok {
			return string(lat)
		}
	}
	return key
}

func (m *Model) moveSelection(delta int) {
	switch m.ActivePane {
	case PaneTopology, PaneRoutes:
		maxItems := len(m.Mesh.Routes) + 1
		m.SelectedRoute = (m.SelectedRoute + delta + maxItems) % maxItems
		if m.SelectedRoute < len(m.Mesh.Routes) {
			m.Editor.RouteIdx = m.SelectedRoute
		}
	case PaneNodes:
		maxItems := len(m.Mesh.Nodes) + 2
		m.SelectedNode = (m.SelectedNode + delta + maxItems) % maxItems
	case PaneClients:
		maxItems := len(m.Mesh.Clients) + 1
		m.SelectedClient = (m.SelectedClient + delta + maxItems) % maxItems
	case PaneLists:
		maxItems := len(m.Mesh.Lists) + 1
		m.SelectedList = (m.SelectedList + delta + maxItems) % maxItems
	}
}

func (m *Model) addHopToSelectedRoute() {
	if len(m.Mesh.Nodes) == 0 {
		m.LogMsg = "ℹ️ Для создания маршрута сначала добавьте хотя бы один сервер/роутер (нажмите 'b' для Bootstrap или 'n')"
		return
	}
	if len(m.Mesh.Routes) > 0 && m.SelectedRoute < len(m.Mesh.Routes) {
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
	} else {
		m.openAddRouteModal()
	}
}

func (m *Model) removeHopFromSelectedRoute() {
	if len(m.Mesh.Routes) > 0 && m.SelectedRoute < len(m.Mesh.Routes) {
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
	normKey := normalizeKey(key)

	if len(m.Modal.Fields) > 0 && m.Modal.ActiveField < len(m.Modal.Fields) {
		f := &m.Modal.Fields[m.Modal.ActiveField]
		if len(f.Options) > 0 {
			switch key {
			case "left":
				f.OptionIdx = (f.OptionIdx - 1 + len(f.Options)) % len(f.Options)
				f.Value = f.Options[f.OptionIdx]
				return *m, nil
			case "right", "space":
				f.OptionIdx = (f.OptionIdx + 1) % len(f.Options)
				f.Value = f.Options[f.OptionIdx]
				return *m, nil
			}
		}
	}

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
			if len(f.Options) == 0 && len(f.Value) > 0 {
				runes := []rune(f.Value)
				f.Value = string(runes[:len(runes)-1])
			}
		}

	case "enter":
		m.submitCurrentModal()

	default:
		if m.Modal.Type == ModalExport {
			switch normKey {
			case "w", "a", "s", "u", "q":
				m.handleExportFormat(normKey)
				return *m, nil
			}
		}

		if len(key) == 1 && len(m.Modal.Fields) > 0 {
			f := &m.Modal.Fields[m.Modal.ActiveField]
			if len(f.Options) == 0 {
				f.Value += key
			}
		}
	}

	return *m, nil
}

func (m *Model) submitCurrentModal() {
	switch m.Modal.Type {
	case ModalBootstrapNode, ModalAddNode:
		name := m.getFieldValue("Имя ноды")
		host := m.getFieldValue("IP / Хост ноды")
		pass := m.getFieldValue("Пароль SSH (для автозагрузки ключа)")
		user := m.getFieldValue("SSH Пользователь")
		nType := m.getFieldValue("Тип платформы")
		protStr := strings.ToLower(m.getFieldValue("Защита от удаления"))

		if name == "" || host == "" {
			m.LogMsg = "⚠️ Ошибка: укажите имя и IP ноды"
			return
		}
		if m.isNodeNameTaken(name, -1) {
			m.LogMsg = fmt.Sprintf("⚠️ Ошибка: Имя ноды %q уже занято! Укажите уникальное имя.", name)
			return
		}
		if user == "" {
			user = "root"
		}
		if nType == "" {
			nType = config.TypeLinux
		}

		keyPath := "~/.config/wgmesh/keys/id_ed25519"
		if pass != "" || m.Modal.Type == ModalBootstrapNode {
			kp, pubStr, err := ensureDefaultSSHKeyTUI()
			if err != nil {
				m.LogMsg = fmt.Sprintf("Ошибка ключа SSH: %v", err)
				return
			}
			keyPath = kp
			if pass != "" {
				m.LogMsg = fmt.Sprintf("→ Провижининг SSH-ключа на %s@%s…", user, host)
				if err := installRemoteKeyTUI(host, 22, user, pass, keyPath, pubStr, nType); err != nil {
					m.LogMsg = fmt.Sprintf("Ошибка установки SSH-ключа на удаленный хост: %v", err)
					return
				}
			}
		}

		isProt := protStr == "да (protected: true)" || protStr == "y" || protStr == "yes"
		m.Mesh.Nodes = append(m.Mesh.Nodes, config.Node{
			Name:      name,
			Type:      nType,
			Host:      host,
			SSHUser:   user,
			SSHKey:    keyPath,
			SSHPort:   22,
			Protected: isProt,
			WireGuard: config.WG{Interface: "wg0", ListenPort: 51820},
		})
		m.IsDirty = true
		_ = config.Save(m.ConfigPath, m.Mesh)
		m.LogMsg = fmt.Sprintf("✔ Нода %q (%s) успешно добавлена!", name, host)
		m.Modal = ModalState{Type: ModalNone}

	case ModalEditNode:
		if m.SelectedNode < 0 || m.SelectedNode >= len(m.Mesh.Nodes) {
			m.Modal = ModalState{Type: ModalNone}
			return
		}
		oldNode := &m.Mesh.Nodes[m.SelectedNode]
		oldName := oldNode.Name

		newName := m.getFieldValue("Имя ноды")
		host := m.getFieldValue("IP / Хост ноды")
		pass := m.getFieldValue("Пароль SSH (для автозагрузки ключа)")
		user := m.getFieldValue("SSH Пользователь")
		nType := m.getFieldValue("Тип платформы")
		protStr := strings.ToLower(m.getFieldValue("Защита от удаления"))

		if newName == "" || host == "" {
			m.LogMsg = "⚠️ Ошибка: имя и IP ноды не могут быть пустыми!"
			return
		}
		if m.isNodeNameTaken(newName, m.SelectedNode) {
			m.LogMsg = fmt.Sprintf("⚠️ Ошибка: Имя ноды %q уже занято!", newName)
			return
		}
		if user == "" {
			user = "root"
		}
		if nType == "" {
			nType = config.TypeLinux
		}

		if pass != "" {
			kp, pubStr, err := ensureDefaultSSHKeyTUI()
			if err == nil {
				m.LogMsg = fmt.Sprintf("→ Провижининг SSH-ключа на %s@%s…", user, host)
				if err := installRemoteKeyTUI(host, 22, user, pass, kp, pubStr, nType); err != nil {
					m.LogMsg = fmt.Sprintf("⚠️ Ошибка установки SSH-ключа: %v", err)
					return
				}
			}
		}

		isProt := protStr == "да (protected: true)" || protStr == "y" || protStr == "yes"

		if newName != oldName {
			m.renameNode(oldName, newName)
		}

		oldNode.Name = newName
		oldNode.Host = host
		oldNode.SSHUser = user
		oldNode.Type = nType
		oldNode.Protected = isProt

		m.IsDirty = true
		_ = config.Save(m.ConfigPath, m.Mesh)

		if newName != oldName {
			m.LogMsg = fmt.Sprintf("✔ Нода переименована: %q ➔ %q (обновлена во всех маршрутах и клиентах)!", oldName, newName)
		} else {
			m.LogMsg = fmt.Sprintf("✔ Настройки ноды %q сохранены!", newName)
		}
		m.Modal = ModalState{Type: ModalNone}

	case ModalAddRoute:
		name := m.getFieldValue("Имя маршрута")
		pathStr := m.getFieldValue("Путь (через запятую)")
		exit := m.getFieldValue("Выходной сервер (Exit)")
		protStr := strings.ToLower(m.getFieldValue("Защита маршрута"))
		if name != "" && pathStr != "" {
			if m.isRouteNameTaken(name, -1) {
				m.LogMsg = fmt.Sprintf("⚠️ Ошибка: Имя маршрута %q уже занято!", name)
				return
			}
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
				Protected: protStr == "да (protected: true)" || protStr == "y" || protStr == "yes",
			}
			m.Mesh.Routes = append(m.Mesh.Routes, r)
			m.IsDirty = true
			_ = config.Save(m.ConfigPath, m.Mesh)
			m.LogMsg = fmt.Sprintf("✔ Маршрут %q создан", name)
		}
		m.Modal = ModalState{Type: ModalNone}

	case ModalEditRoute:
		if m.SelectedRoute < 0 || m.SelectedRoute >= len(m.Mesh.Routes) {
			m.Modal = ModalState{Type: ModalNone}
			return
		}
		oldRoute := &m.Mesh.Routes[m.SelectedRoute]
		newName := m.getFieldValue("Имя маршрута")
		pathStr := m.getFieldValue("Путь (через запятую)")
		exit := m.getFieldValue("Выходной сервер (Exit)")
		protStr := strings.ToLower(m.getFieldValue("Защита маршрута"))

		if newName == "" || pathStr == "" {
			m.LogMsg = "⚠️ Ошибка: имя и путь маршрута не могут быть пустыми!"
			return
		}
		if m.isRouteNameTaken(newName, m.SelectedRoute) {
			m.LogMsg = fmt.Sprintf("⚠️ Ошибка: Имя маршрута %q уже занято!", newName)
			return
		}

		hops := splitTrimTUI(pathStr)
		if len(hops) > 0 && hops[0] != config.ClientHop {
			hops = append([]string{config.ClientHop}, hops...)
		}
		if exit == "" && len(hops) > 1 {
			exit = hops[len(hops)-1]
		}

		oldRoute.Name = newName
		oldRoute.Path = hops
		oldRoute.ExitNode = exit
		oldRoute.Protected = protStr == "да (protected: true)" || protStr == "y" || protStr == "yes"

		m.IsDirty = true
		_ = config.Save(m.ConfigPath, m.Mesh)
		m.LogMsg = fmt.Sprintf("✔ Настройки маршрута %q сохранены!", newName)
		m.Modal = ModalState{Type: ModalNone}

	case ModalAddClient:
		name := m.getFieldValue("Имя клиента (устройства)")
		ingress := m.getFieldValue("Нода подключения (Ingress)")
		if name != "" {
			if m.isClientNameTaken(name, -1) {
				m.LogMsg = fmt.Sprintf("⚠️ Ошибка: Имя клиента %q уже занято!", name)
				return
			}
			if strings.HasPrefix(ingress, "(") {
				ingress = ""
			}
			c := config.Client{
				Name:    name,
				Ingress: ingress,
			}
			m.Mesh.Clients = append(m.Mesh.Clients, c)
			m.IsDirty = true
			_ = config.Save(m.ConfigPath, m.Mesh)
			m.LogMsg = fmt.Sprintf("✔ Клиент %q добавлен", name)
		}
		m.Modal = ModalState{Type: ModalNone}

	case ModalEditClient:
		if m.SelectedClient < 0 || m.SelectedClient >= len(m.Mesh.Clients) {
			m.Modal = ModalState{Type: ModalNone}
			return
		}
		oldClient := &m.Mesh.Clients[m.SelectedClient]
		newName := m.getFieldValue("Имя клиента (устройства)")
		ingress := m.getFieldValue("Нода подключения (Ingress)")

		if newName == "" {
			m.LogMsg = "⚠️ Ошибка: имя клиента не может быть пустым!"
			return
		}
		if m.isClientNameTaken(newName, m.SelectedClient) {
			m.LogMsg = fmt.Sprintf("⚠️ Ошибка: Имя клиента %q уже занято!", newName)
			return
		}
		if strings.HasPrefix(ingress, "(") {
			ingress = ""
		}

		oldClient.Name = newName
		oldClient.Ingress = ingress
		m.IsDirty = true
		_ = config.Save(m.ConfigPath, m.Mesh)
		m.LogMsg = fmt.Sprintf("✔ Настройки клиента %q сохранены!", newName)
		m.Modal = ModalState{Type: ModalNone}

	case ModalAddList:
		name := m.getFieldValue("Имя списка")
		domStr := m.getFieldValue("Домены (через запятую)")
		if name != "" {
			if m.isListNameTaken(name, -1) {
				m.LogMsg = fmt.Sprintf("⚠️ Ошибка: Имя списка %q уже занято!", name)
				return
			}
			doms := splitTrimTUI(domStr)
			l := config.DomainList{
				Name:    name,
				Domains: doms,
			}
			m.Mesh.Lists = append(m.Mesh.Lists, l)
			m.IsDirty = true
			_ = config.Save(m.ConfigPath, m.Mesh)
			m.LogMsg = fmt.Sprintf("✔ Список доменов %q добавлен", name)
		}
		m.Modal = ModalState{Type: ModalNone}

	case ModalEditList:
		if m.SelectedList < 0 || m.SelectedList >= len(m.Mesh.Lists) {
			m.Modal = ModalState{Type: ModalNone}
			return
		}
		oldList := &m.Mesh.Lists[m.SelectedList]
		newName := m.getFieldValue("Имя списка")
		domStr := m.getFieldValue("Домены (через запятую)")

		if newName == "" {
			m.LogMsg = "⚠️ Ошибка: имя списка не может быть пустым!"
			return
		}
		if m.isListNameTaken(newName, m.SelectedList) {
			m.LogMsg = fmt.Sprintf("⚠️ Ошибка: Имя списка %q уже занято!", newName)
			return
		}

		oldList.Name = newName
		oldList.Domains = splitTrimTUI(domStr)
		m.IsDirty = true
		_ = config.Save(m.ConfigPath, m.Mesh)
		m.LogMsg = fmt.Sprintf("✔ Настройки списка %q сохранены!", newName)
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

	case ModalUpdate:
		if m.Modal.HasUpdate && !m.Modal.IsUpdating {
			m.Modal.IsUpdating = true
			m.Modal.UpdateStatus = fmt.Sprintf("⏳ Скачивание и запуск скрипта обновления версии %s...", m.Modal.UpdateVersion)
			downloadURL := m.Modal.DownloadURL
			if err := updater.PerformUpdate(downloadURL); err != nil {
				m.Modal.IsUpdating = false
				m.Modal.UpdateStatus = fmt.Sprintf("❌ Ошибка обновления: %v", err)
			}
		}
	}
}

func (m *Model) getFieldValue(prefix string) string {
	for _, f := range m.Modal.Fields {
		if strings.HasPrefix(f.Label, prefix) {
			if len(f.Options) > 0 && f.OptionIdx >= 0 && f.OptionIdx < len(f.Options) {
				return f.Options[f.OptionIdx]
			}
			return strings.TrimSpace(f.Value)
		}
	}
	return ""
}

func (m *Model) nextDefaultNodeName() string {
	for i := 1; i <= 99; i++ {
		name := fmt.Sprintf("node_%02d", i)
		if !m.isNodeNameTaken(name, -1) {
			return name
		}
	}
	return "node_99"
}

func (m *Model) nextDefaultRouteName() string {
	for i := 1; i <= 99; i++ {
		name := fmt.Sprintf("route_%02d", i)
		if !m.isRouteNameTaken(name, -1) {
			return name
		}
	}
	return "route_99"
}

func (m *Model) nextDefaultClientName() string {
	for i := 1; i <= 99; i++ {
		name := fmt.Sprintf("client_%02d", i)
		if !m.isClientNameTaken(name, -1) {
			return name
		}
	}
	return "client_99"
}

func (m *Model) nextDefaultListName() string {
	for i := 1; i <= 99; i++ {
		name := fmt.Sprintf("list_%02d", i)
		if !m.isListNameTaken(name, -1) {
			return name
		}
	}
	return "list_99"
}

func (m *Model) nodeNamesList() []string {
	var names []string
	for _, n := range m.Mesh.Nodes {
		names = append(names, n.Name)
	}
	if len(names) == 0 {
		names = []string{"(сначала добавьте ноду)"}
	}
	return names
}

func (m *Model) openBootstrapModal() {
	m.Modal = ModalState{
		Type: ModalBootstrapNode,
		Fields: []FormField{
			{Label: "Имя ноды", Value: m.nextDefaultNodeName()},
			{Label: "IP / Хост ноды", Placeholder: "109.248.198.55"},
			{Label: "Пароль SSH *", Mask: true},
			{Label: "Тип платформы", Options: []string{"linux", "mikrotik", "openwrt"}, OptionIdx: 0, Value: "linux"},
			{Label: "SSH Пользователь", Value: "root"},
			{Label: "Защита от удаления **", Options: []string{"нет", "да (protected: true)"}, OptionIdx: 0, Value: "нет"},
		},
	}
}

func (m *Model) openAddNodeModal() {
	m.Modal = ModalState{
		Type: ModalAddNode,
		Fields: []FormField{
			{Label: "Имя ноды", Value: m.nextDefaultNodeName()},
			{Label: "IP / Хост ноды", Placeholder: "194.87.71.7"},
			{Label: "Пароль SSH *", Mask: true},
			{Label: "Тип платформы", Options: []string{"linux", "mikrotik", "openwrt"}, OptionIdx: 0, Value: "linux"},
			{Label: "SSH Пользователь", Value: "root"},
			{Label: "Защита от удаления **", Options: []string{"нет", "да (protected: true)"}, OptionIdx: 0, Value: "нет"},
		},
	}
}

func (m *Model) openAddRouteModal() {
	if len(m.Mesh.Nodes) == 0 {
		m.LogMsg = "ℹ️ Для создания маршрута сначала добавьте хотя бы один сервер/роутер (нажмите 'b' для Bootstrap или 'n')"
		return
	}
	nodeNames := m.nodeNamesList()
	defaultPath := "client," + nodeNames[0]
	m.Modal = ModalState{
		Type: ModalAddRoute,
		Fields: []FormField{
			{Label: "Имя маршрута", Value: m.nextDefaultRouteName()},
			{Label: "Путь (через запятую)", Value: defaultPath},
			{Label: "Выходной сервер (Exit)", Options: nodeNames, OptionIdx: 0, Value: nodeNames[0]},
			{Label: "Защита маршрута **", Options: []string{"нет", "да (protected: true)"}, OptionIdx: 0, Value: "нет"},
		},
	}
}

func (m *Model) openAddClientModal() {
	nodeNames := m.nodeNamesList()
	m.Modal = ModalState{
		Type: ModalAddClient,
		Fields: []FormField{
			{Label: "Имя клиента (устройства)", Value: m.nextDefaultClientName()},
			{Label: "Нода подключения (Ingress)", Options: nodeNames, OptionIdx: 0, Value: nodeNames[0]},
		},
	}
}

func (m *Model) openAddListModal() {
	m.Modal = ModalState{
		Type: ModalAddList,
		Fields: []FormField{
			{Label: "Имя списка", Value: m.nextDefaultListName()},
			{Label: "Домены (через запятую)", Placeholder: "youtube.com, *.googlevideo.com"},
		},
	}
}

func (m *Model) isNodeNameTaken(name string, excludeIdx int) bool {
	for i, n := range m.Mesh.Nodes {
		if i != excludeIdx && strings.EqualFold(n.Name, name) {
			return true
		}
	}
	return false
}

func (m *Model) isRouteNameTaken(name string, excludeIdx int) bool {
	for i, r := range m.Mesh.Routes {
		if i != excludeIdx && strings.EqualFold(r.Name, name) {
			return true
		}
	}
	return false
}

func (m *Model) isClientNameTaken(name string, excludeIdx int) bool {
	for i, c := range m.Mesh.Clients {
		if i != excludeIdx && strings.EqualFold(c.Name, name) {
			return true
		}
	}
	return false
}

func (m *Model) isListNameTaken(name string, excludeIdx int) bool {
	for i, l := range m.Mesh.Lists {
		if i != excludeIdx && strings.EqualFold(l.Name, name) {
			return true
		}
	}
	return false
}

func (m *Model) renameNode(oldName, newName string) {
	if oldName == "" || newName == "" || oldName == newName {
		return
	}
	for i := range m.Mesh.Routes {
		r := &m.Mesh.Routes[i]
		for j := range r.Path {
			if r.Path[j] == oldName {
				r.Path[j] = newName
			}
		}
		if r.ExitNode == oldName {
			r.ExitNode = newName
		}
	}
	for i := range m.Mesh.Clients {
		c := &m.Mesh.Clients[i]
		if c.Ingress == oldName {
			c.Ingress = newName
		}
	}
}

func (m *Model) openEditNodeModal() {
	if len(m.Mesh.Nodes) == 0 || m.SelectedNode < 0 || m.SelectedNode >= len(m.Mesh.Nodes) {
		return
	}
	node := &m.Mesh.Nodes[m.SelectedNode]
	typeOpt := 0
	switch node.Type {
	case config.TypeMikrotik:
		typeOpt = 1
	case config.TypeOpenWRT:
		typeOpt = 2
	}
	protOpt := 0
	if node.Protected {
		protOpt = 1
	}

	m.Modal = ModalState{
		Type: ModalEditNode,
		Fields: []FormField{
			{Label: "Имя ноды", Value: node.Name},
			{Label: "IP / Хост ноды", Value: node.Host},
			{Label: "Пароль SSH *", Mask: true},
			{Label: "Тип платформы", Options: []string{"linux", "mikrotik", "openwrt"}, OptionIdx: typeOpt, Value: node.Type},
			{Label: "SSH Пользователь", Value: node.SSHUser},
			{Label: "Защита от удаления **", Options: []string{"нет", "да (protected: true)"}, OptionIdx: protOpt, Value: []string{"нет", "да (protected: true)"}[protOpt]},
		},
	}
}

func (m *Model) openEditClientModal() {
	if len(m.Mesh.Clients) == 0 || m.SelectedClient < 0 || m.SelectedClient >= len(m.Mesh.Clients) {
		return
	}
	c := &m.Mesh.Clients[m.SelectedClient]
	nodeNames := m.nodeNamesList()
	ingOpt := 0
	for i, name := range nodeNames {
		if name == c.Ingress {
			ingOpt = i
			break
		}
	}
	m.Modal = ModalState{
		Type: ModalEditClient,
		Fields: []FormField{
			{Label: "Имя клиента (устройства)", Value: c.Name},
			{Label: "Нода подключения (Ingress)", Options: nodeNames, OptionIdx: ingOpt, Value: nodeNames[ingOpt]},
		},
	}
}

func (m *Model) openEditListModal() {
	if len(m.Mesh.Lists) == 0 || m.SelectedList < 0 || m.SelectedList >= len(m.Mesh.Lists) {
		return
	}
	l := &m.Mesh.Lists[m.SelectedList]
	m.Modal = ModalState{
		Type: ModalEditList,
		Fields: []FormField{
			{Label: "Имя списка", Value: l.Name},
			{Label: "Домены (через запятую)", Value: strings.Join(l.Domains, ", ")},
		},
	}
}

func (m *Model) openEditRouteModal() {
	if len(m.Mesh.Routes) == 0 || m.SelectedRoute < 0 || m.SelectedRoute >= len(m.Mesh.Routes) {
		return
	}
	r := &m.Mesh.Routes[m.SelectedRoute]
	if r.Protected && m.Modal.Type != ModalConfirm {
		m.Modal = ModalState{
			Type:          ModalConfirm,
			ConfirmPrompt: fmt.Sprintf("Маршрут %q помечен как защищённый (protected: true)! Вы действительно хотите его отредактировать?", r.Name),
			OnConfirm: func(mod *Model) {
				mod.showEditRouteForm()
			},
		}
		return
	}
	m.showEditRouteForm()
}

func (m *Model) showEditRouteForm() {
	if len(m.Mesh.Routes) == 0 || m.SelectedRoute < 0 || m.SelectedRoute >= len(m.Mesh.Routes) {
		return
	}
	r := &m.Mesh.Routes[m.SelectedRoute]
	nodeNames := m.nodeNamesList()
	exitOpt := 0
	for i, name := range nodeNames {
		if name == r.ExitNode {
			exitOpt = i
			break
		}
	}
	protOpt := 0
	if r.Protected {
		protOpt = 1
	}
	m.Modal = ModalState{
		Type: ModalEditRoute,
		Fields: []FormField{
			{Label: "Имя маршрута", Value: r.Name},
			{Label: "Путь (через запятую)", Value: strings.Join(r.Path, ", ")},
			{Label: "Выходной сервер (Exit)", Options: nodeNames, OptionIdx: exitOpt, Value: nodeNames[exitOpt]},
			{Label: "Защита маршрута **", Options: []string{"нет", "да (protected: true)"}, OptionIdx: protOpt, Value: []string{"нет", "да (protected: true)"}[protOpt]},
		},
	}
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

func (m *Model) openUpdateModal() {
	m.Modal = ModalState{
		Type:         ModalUpdate,
		UpdateStatus: fmt.Sprintf("🔍 Проверка обновлений (текущая версия: %s)...", m.Version),
	}

	hasUpdate, latestVer, downloadURL, err := updater.CheckForUpdate(m.Version)
	if err != nil {
		m.Modal.UpdateStatus = fmt.Sprintf("❌ Ошибка проверки обновлений: %v", err)
		return
	}

	if !hasUpdate {
		m.Modal.UpdateStatus = fmt.Sprintf("✔ У вас установлена самая актуальная версия программы (%s).", m.Version)
		m.Modal.HasUpdate = false
		return
	}

	m.Modal.HasUpdate = true
	m.Modal.UpdateVersion = latestVer
	m.Modal.DownloadURL = downloadURL
	m.Modal.UpdateStatus = fmt.Sprintf("🆕 Найдена новая версия: %s (текущая: %s)\n\nНажмите [Enter] для установки обновления.", latestVer, m.Version)
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
		Render(fmt.Sprintf("═══ %s (wgmesh TUI) ═══", m.Mesh.Name))

	topView := RenderTopology(m.Mesh, m.SelectedRoute, totalWidth)

	topBox := paneStyle.Width(totalWidth).Render(
		header + "\n\n" + topView,
	)

	paneHeight := 6
	testNodesView := RenderNodesPane(m.Mesh, m.SelectedNode, -1, m.ActivePane == PaneNodes, colWidth, paneHeight)
	testRoutesView := RenderRoutesPane(m.Mesh, m.SelectedRoute, -1, m.ActivePane == PaneRoutes, colWidth, paneHeight)
	testClientsView := RenderClientsPane(m.Mesh, m.SelectedClient, -1, m.ActivePane == PaneClients, colWidth, paneHeight)
	testListsView := RenderListsPane(m.Mesh, m.SelectedList, -1, m.ActivePane == PaneLists, colWidth, paneHeight)
	testEditorView := RenderEditorPane(m.Mesh, &m.Editor, -1, -1, m.ActivePane == PaneEditor, totalWidth)

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
			nodeCount := len(m.Mesh.Nodes)
			routeCount := len(m.Mesh.Routes)
			dirtyText := ""
			if m.IsDirty {
				dirtyText = " * [НЕ СОХРАНЕНО — нажмите 's']"
			}
			leftInfo := fmt.Sprintf("Узлы: %d | Маршруты: %d | %s%s", nodeCount, routeCount, m.ConfigPath, dirtyText)
			leftWidth := len(leftInfo) + 4
			hintsX := x - leftWidth
			if hintsX >= 0 {
				switch {
				case hintsX <= 10:
					hoverHintIdx = 0
				case hintsX <= 22:
					hoverHintIdx = 1
				case hintsX <= 38:
					hoverHintIdx = 2
				case hintsX <= 51:
					hoverHintIdx = 3
				case hintsX <= 64:
					hoverHintIdx = 4
				case hintsX <= 74:
					hoverHintIdx = 5
				case hintsX <= 85:
					hoverHintIdx = 6
				default:
					hoverHintIdx = 7
				}
			}
		}
	}

	nodesView := RenderNodesPane(m.Mesh, m.SelectedNode, hoverNodesIdx, m.ActivePane == PaneNodes, colWidth, paneHeight)
	routesView := RenderRoutesPane(m.Mesh, m.SelectedRoute, hoverRoutesIdx, m.ActivePane == PaneRoutes, colWidth, paneHeight)
	clientsView := RenderClientsPane(m.Mesh, m.SelectedClient, hoverClientsIdx, m.ActivePane == PaneClients, colWidth, paneHeight)
	listsView := RenderListsPane(m.Mesh, m.SelectedList, hoverListsIdx, m.ActivePane == PaneLists, colWidth, paneHeight)
	editorView := RenderEditorPane(m.Mesh, &m.Editor, hoverHopIdx, hoverBtnIdx, m.ActivePane == PaneEditor, totalWidth)

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

	statusBar := RenderStatusBar(m.Mesh, m.ConfigPath, m.IsDirty, "", hoverHintIdx, m.Width)

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
