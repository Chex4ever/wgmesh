package tui

import (
	"fmt"
	"os/exec"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/wgmesh/wgmesh/internal/config"
	"github.com/wgmesh/wgmesh/internal/i18n"
)

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case checkUpdateMsg:
		if msg.err != nil {
			m.Modal.UpdateStatus = fmt.Sprintf("[FAIL] Update check error: %v", msg.err)
			return m, nil
		}
		if !msg.hasUpdate {
			m.Modal.UpdateStatus = i18n.T("update_latest", m.Version)
			m.Modal.HasUpdate = false
			return m, nil
		}
		m.Modal.HasUpdate = true
		m.Modal.UpdateVersion = msg.latestVersion
		m.Modal.DownloadURL = msg.downloadURL
		m.Modal.UpdateStatus = i18n.T("update_new", msg.latestVersion, m.Version)
		return m, nil

	case performUpdateMsg:
		if msg.err != nil {
			m.Modal.IsUpdating = false
			m.Modal.UpdateStatus = fmt.Sprintf("[FAIL] Update error: %v", msg.err)
		}
		return m, nil

	case ModalTickMsg:
		if m.Modal.Type != ModalNone && m.Modal.IsAnimating {
			elapsed := time.Since(m.Modal.AnimStartTime)
			progress := float64(elapsed) / float64(m.Modal.AnimDuration)
			if progress >= 1.0 {
				m.Modal.AnimProgress = 1.0
				m.Modal.IsAnimating = false
				if m.Modal.IsClosing {
					m.Modal = ModalState{Type: ModalNone}
				}
				return m, nil
			}
			m.Modal.AnimProgress = progress
			return m, animateModalCmd()
		}
		return m, nil

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
			cmd := m.openModal(ModalState{Type: ModalHelp})
			return m, cmd

		case "tab":
			m.ActivePane = (m.ActivePane + 1) % 6

		case "shift+tab":
			m.ActivePane = (m.ActivePane + 5) % 6

		case "s":
			if err := config.Save(m.ConfigPath, m.Mesh); err != nil {
				m.LogMsg = fmt.Sprintf("Ошибка сохранения: %v", err)
			} else {
				m.IsDirty = false
				m.LogMsg = fmt.Sprintf("[OK] Конфигурация успешно сохранена в %s", m.ConfigPath)
			}

		case "a":
			m.LogMsg = "-> Запуск полного применения (Apply) по SSH..."
			m.runApply()

		case "d":
			cmd := m.runDoctor()
			return m, cmd

		case "g":
			cmd := m.openGitModal()
			return m, cmd

		case "u":
			cmd := m.openUpdateModal()
			return m, cmd

		case "b":
			cmd := m.openBootstrapModal()
			return m, cmd

		case "n":
			cmd := m.openAddNodeModal()
			return m, cmd

		case "r":
			cmd := m.openAddRouteModal()
			return m, cmd

		case "c":
			cmd := m.openAddClientModal()
			return m, cmd

		case "l":
			cmd := m.openAddListModal()
			return m, cmd

		case "e":
			var cmd tea.Cmd
			switch m.ActivePane {
			case PaneNodes:
				cmd = m.openEditNodeModal()
			case PaneRoutes, PaneEditor, PaneTopology:
				cmd = m.openEditRouteModal()
			case PaneClients:
				cmd = m.openEditClientModal()
			case PaneLists:
				cmd = m.openEditListModal()
			}
			return m, cmd

		case "delete", "x":
			cmd := m.handleDeleteCurrent()
			return m, cmd

		case "t":
			m.handleTeardownNode()

		case "k":
			cmd := m.handleCapabilities()
			return m, cmd

		case "enter", "space":
			var cmd tea.Cmd
			switch m.ActivePane {
			case PaneNodes:
				if m.SelectedNode == len(m.Mesh.Nodes) {
					cmd = m.openAddNodeModal()
				} else if m.SelectedNode == len(m.Mesh.Nodes)+1 {
					cmd = m.openBootstrapModal()
				} else {
					cmd = m.openEditNodeModal()
				}
			case PaneRoutes:
				if m.SelectedRoute == len(m.Mesh.Routes) {
					cmd = m.openAddRouteModal()
				} else {
					cmd = m.openEditRouteModal()
				}
			case PaneClients:
				if m.SelectedClient == len(m.Mesh.Clients) {
					cmd = m.openAddClientModal()
				} else {
					cmd = m.openEditClientModal()
				}
			case PaneLists:
				if m.SelectedList == len(m.Mesh.Lists) {
					cmd = m.openAddListModal()
				} else {
					cmd = m.openEditListModal()
				}
			}
			return m, cmd

		case "up":
			m.moveSelection(-1)

		case "down":
			m.moveSelection(1)

		case "left":
			if m.ActivePane == PaneEditor {
				m.cycleSelectedHopNode(-1)
			}

		case "right":
			if m.ActivePane == PaneEditor {
				m.cycleSelectedHopNode(1)
			}

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

func (m *Model) handleModalKey(key string) (Model, tea.Cmd) {
	if m.Modal.IsClosing {
		return *m, nil
	}

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

	switch normKey {
	case "esc":
		cmd := m.closeModal()
		return *m, cmd

	case "q":
		if len(m.Modal.Fields) == 0 {
			cmd := m.closeModal()
			return *m, cmd
		}

	case "y":
		if m.Modal.Type == ModalConfirm && m.Modal.OnConfirm != nil {
			m.Modal.OnConfirm(m)
		}
		cmd := m.closeModal()
		return *m, cmd

	case "n":
		if m.Modal.Type == ModalConfirm {
			cmd := m.closeModal()
			return *m, cmd
		}
	}

	switch key {
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
		if m.Modal.Type == ModalAddRoute || m.Modal.Type == ModalEditRoute {
			if m.Modal.ActiveField < len(m.Modal.Fields) {
				lbl := m.Modal.Fields[m.Modal.ActiveField].Label
				if strings.Contains(lbl, "[+] Добавить хоп") {
					m.addHopFieldToRouteModal()
					return *m, nil
				}
				if strings.Contains(lbl, "[-] Удалить") {
					m.removeHopFieldFromRouteModal()
					return *m, nil
				}
			}
		}
		cmd := m.submitCurrentModal()
		return *m, cmd

	default:
		if m.Modal.Type == ModalExport {
			switch normKey {
			case "w", "a", "s", "u":
				m.handleExportFormat(normKey)
				return *m, nil
			case "q":
				cmd := m.closeModal()
				return *m, cmd
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

func (m *Model) submitCurrentModal() tea.Cmd {
	if m.Modal.Type == ModalUpdate {
		if m.Modal.HasUpdate && !m.Modal.IsUpdating {
			m.Modal.IsUpdating = true
			m.Modal.UpdateStatus = i18n.T("update_installing")
			return performUpdateCmd(m.Modal.DownloadURL)
		}
		return m.closeModal()
	}
	return m.submitFormModal()
}

func (m *Model) submitFormModal() tea.Cmd {
	switch m.Modal.Type {
	case ModalBootstrapNode, ModalAddNode:
		name := m.getFieldValue("Имя ноды")
		host := m.getFieldValue("IP / Хост ноды")
		pass := m.getFieldValue("Пароль SSH (для автозагрузки ключа)")
		user := m.getFieldValue("SSH Пользователь")
		nType := m.getFieldValue("Тип платформы")
		protStr := strings.ToLower(m.getFieldValue("Защита от удаления"))

		if name == "" || host == "" {
			m.LogMsg = "[!] Ошибка: укажите имя и IP ноды"
			return nil
		}
		if m.isNodeNameTaken(name, -1) {
			m.LogMsg = fmt.Sprintf("[!] Ошибка: Имя ноды %q уже занято! Укажите уникальное имя.", name)
			return nil
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
				return nil
			}
			keyPath = kp
			if pass != "" {
				m.LogMsg = fmt.Sprintf("-> Провижининг SSH-ключа на %s@%s...", user, host)
				if err := installRemoteKeyTUI(host, 22, user, pass, keyPath, pubStr, nType); err != nil {
					m.LogMsg = fmt.Sprintf("Ошибка установки SSH-ключа на удаленный хост: %v", err)
					return nil
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
		m.LogMsg = fmt.Sprintf("[OK] Нода %q (%s) успешно добавлена!", name, host)
		return m.closeModal()

	case ModalEditNode:
		if m.SelectedNode < 0 || m.SelectedNode >= len(m.Mesh.Nodes) {
			return m.closeModal()
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
			m.LogMsg = "[!] Ошибка: имя и IP ноды не могут быть пустыми!"
			return nil
		}
		if m.isNodeNameTaken(newName, m.SelectedNode) {
			m.LogMsg = fmt.Sprintf("[!] Ошибка: Имя ноды %q уже занято!", newName)
			return nil
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
				m.LogMsg = fmt.Sprintf("-> Провижининг SSH-ключа на %s@%s...", user, host)
				if err := installRemoteKeyTUI(host, 22, user, pass, kp, pubStr, nType); err != nil {
					m.LogMsg = fmt.Sprintf("[!] Ошибка установки SSH-ключа: %v", err)
					return nil
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
			m.LogMsg = fmt.Sprintf("[OK] Нода переименована: %q -> %q (обновлена во всех маршрутах и клиентах)!", oldName, newName)
		} else {
			m.LogMsg = fmt.Sprintf("[OK] Настройки ноды %q сохранены!", newName)
		}
		return m.closeModal()

	case ModalAddRoute, ModalEditRoute:
		name := m.getFieldValue("Имя маршрута")
		if name == "" {
			m.LogMsg = "[!] Ошибка: имя маршрута не может быть пустым!"
			return nil
		}
		excludeIdx := -1
		if m.Modal.Type == ModalEditRoute {
			excludeIdx = m.SelectedRoute
		}
		if m.isRouteNameTaken(name, excludeIdx) {
			m.LogMsg = fmt.Sprintf("[!] Ошибка: Имя маршрута %q уже занято!", name)
			return nil
		}

		var hops []string
		hops = append(hops, config.ClientHop)

		for _, f := range m.Modal.Fields {
			if strings.HasPrefix(f.Label, "Узел ") {
				if len(f.Options) > 0 && f.OptionIdx >= 0 && f.OptionIdx < len(f.Options) {
					hops = append(hops, f.Options[f.OptionIdx])
				}
			}
		}

		if len(hops) == 1 {
			m.LogMsg = "[!] Ошибка: выберите хотя бы один узел в маршруте!"
			return nil
		}

		exit := m.getFieldValue("Выходной узел")
		if exit == "" {
			exit = hops[len(hops)-1]
		}
		protStr := strings.ToLower(m.getFieldValue("Защита маршрута"))
		isProt := protStr == "да (protected: true)" || protStr == "y" || protStr == "yes"

		if m.Modal.Type == ModalAddRoute {
			r := config.Route{
				Name:      name,
				Path:      hops,
				ExitNode:  exit,
				Protected: isProt,
			}
			m.Mesh.Routes = append(m.Mesh.Routes, r)
			m.LogMsg = fmt.Sprintf("[OK] Маршрут %q создан", name)
		} else {
			if m.SelectedRoute >= 0 && m.SelectedRoute < len(m.Mesh.Routes) {
				oldRoute := &m.Mesh.Routes[m.SelectedRoute]
				oldRoute.Name = name
				oldRoute.Path = hops
				oldRoute.ExitNode = exit
				oldRoute.Protected = isProt
				m.LogMsg = fmt.Sprintf("[OK] Настройки маршрута %q сохранены!", name)
			}
		}
		m.IsDirty = true
		_ = config.Save(m.ConfigPath, m.Mesh)
		return m.closeModal()

	case ModalAddClient:
		name := m.getFieldValue("Имя клиента (устройства)")
		ingress := m.getFieldValue("Нода подключения (Ingress)")
		if name != "" {
			if m.isClientNameTaken(name, -1) {
				m.LogMsg = fmt.Sprintf("[!] Ошибка: Имя клиента %q уже занято!", name)
				return nil
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
			m.LogMsg = fmt.Sprintf("[OK] Клиент %q добавлен", name)
		}
		return m.closeModal()

	case ModalEditClient:
		if m.SelectedClient < 0 || m.SelectedClient >= len(m.Mesh.Clients) {
			return m.closeModal()
		}
		oldClient := &m.Mesh.Clients[m.SelectedClient]
		newName := m.getFieldValue("Имя клиента (устройства)")
		ingress := m.getFieldValue("Нода подключения (Ingress)")

		if newName == "" {
			m.LogMsg = "[!] Ошибка: имя клиента не может быть пустым!"
			return nil
		}
		if m.isClientNameTaken(newName, m.SelectedClient) {
			m.LogMsg = fmt.Sprintf("[!] Ошибка: Имя клиента %q уже занято!", newName)
			return nil
		}
		if strings.HasPrefix(ingress, "(") {
			ingress = ""
		}

		oldClient.Name = newName
		oldClient.Ingress = ingress
		m.IsDirty = true
		_ = config.Save(m.ConfigPath, m.Mesh)
		m.LogMsg = fmt.Sprintf("[OK] Настройки клиента %q сохранены!", newName)
		return m.closeModal()

	case ModalAddList:
		name := m.getFieldValue("Имя списка")
		domStr := m.getFieldValue("Домены (через запятую)")
		if name != "" {
			if m.isListNameTaken(name, -1) {
				m.LogMsg = fmt.Sprintf("[!] Ошибка: Имя списка %q уже занято!", name)
				return nil
			}
			doms := splitTrimTUI(domStr)
			l := config.DomainList{
				Name:    name,
				Domains: doms,
			}
			m.Mesh.Lists = append(m.Mesh.Lists, l)
			m.IsDirty = true
			_ = config.Save(m.ConfigPath, m.Mesh)
			m.LogMsg = fmt.Sprintf("[OK] Список доменов %q добавлен", name)
		}
		return m.closeModal()

	case ModalEditList:
		if m.SelectedList < 0 || m.SelectedList >= len(m.Mesh.Lists) {
			return m.closeModal()
		}
		oldList := &m.Mesh.Lists[m.SelectedList]
		newName := m.getFieldValue("Имя списка")
		domStr := m.getFieldValue("Домены (через запятую)")

		if newName == "" {
			m.LogMsg = "[!] Ошибка: имя списка не может быть пустым!"
			return nil
		}
		if m.isListNameTaken(newName, m.SelectedList) {
			m.LogMsg = fmt.Sprintf("[!] Ошибка: Имя списка %q уже занято!", newName)
			return nil
		}

		oldList.Name = newName
		oldList.Domains = splitTrimTUI(domStr)
		m.IsDirty = true
		_ = config.Save(m.ConfigPath, m.Mesh)
		m.LogMsg = fmt.Sprintf("[OK] Настройки списка %q сохранены!", newName)
		return m.closeModal()

	case ModalGit:
		msg := m.getFieldValue("Сообщение коммита")
		if msg == "" {
			msg = "update wgmesh configuration"
		}
		out, err := exec.Command("git", "commit", "-am", msg).CombinedOutput()
		if err != nil {
			m.LogMsg = fmt.Sprintf("Git commit ошибка: %v (%s)", err, string(out))
		} else {
			outPush, errP := exec.Command("git", "push").CombinedOutput()
			if errP != nil {
				m.LogMsg = fmt.Sprintf("[OK] Коммит создан. Git push ошибка: %v (%s)", errP, string(outPush))
			} else {
				m.LogMsg = "[OK] Изменения успешно закоммичены и отправлены в Git (push)!"
			}
		}
		return m.closeModal()
	}
	return m.closeModal()
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

