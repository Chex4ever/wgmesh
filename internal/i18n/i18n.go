// Package i18n предоставляет локализацию для TUI и CLI интерфейсов wgmesh.
package i18n

import (
	"fmt"
	"os"
	"runtime"
	"strings"
	"sync"
	"syscall"
)

var (
	mu          sync.RWMutex
	currentLang = "en"
)

// SetLanguage устанавливает текущий язык локализации ("en", "ru" или "auto").
func SetLanguage(lang string) {
	mu.Lock()
	defer mu.Unlock()

	clean := strings.ToLower(strings.TrimSpace(lang))
	if clean == "" || clean == "auto" {
		currentLang = DetectOSLanguage()
		return
	}

	if clean == "ru" || strings.HasPrefix(clean, "ru") {
		currentLang = "ru"
	} else {
		currentLang = "en"
	}
}

// CurrentLanguage возвращает текущий код языка ("en" или "ru").
func CurrentLanguage() string {
	mu.RLock()
	defer mu.RUnlock()
	return currentLang
}

// DetectOSLanguage определяет язык системы. Если язык русский (СНГ) — возвращает "ru", иначе "en".
func DetectOSLanguage() string {
	for _, env := range []string{"LANG", "LC_ALL", "LC_MESSAGES", "SYSTEMLOCALE"} {
		val := strings.ToLower(os.Getenv(env))
		if val != "" {
			if strings.HasPrefix(val, "ru") {
				return "ru"
			}
			if strings.HasPrefix(val, "en") {
				return "en"
			}
		}
	}

	if runtime.GOOS == "windows" {
		for _, env := range []string{"LANG", "SYSTEMLOCALE"} {
			val := strings.ToLower(os.Getenv(env))
			if strings.Contains(val, "ru") {
				return "ru"
			}
		}

		modKernel32 := syscall.NewLazyDLL("kernel32.dll")
		procGetUserDefaultUILang := modKernel32.NewProc("GetUserDefaultUILanguage")
		if procGetUserDefaultUILang.Find() == nil {
			r1, _, _ := procGetUserDefaultUILang.Call()
			langID := uint16(r1)
			primaryLangID := langID & 0x3ff
			// 0x19 (Russian), 0x22 (Ukrainian), 0x23 (Belarusian), 0x3f (Kazakh)
			if primaryLangID == 0x19 || primaryLangID == 0x22 || primaryLangID == 0x23 || primaryLangID == 0x3f {
				return "ru"
			}
		}

		procGetSystemDefaultUILang := modKernel32.NewProc("GetSystemDefaultUILanguage")
		if procGetSystemDefaultUILang.Find() == nil {
			r1, _, _ := procGetSystemDefaultUILang.Call()
			langID := uint16(r1)
			primaryLangID := langID & 0x3ff
			if primaryLangID == 0x19 || primaryLangID == 0x22 || primaryLangID == 0x23 || primaryLangID == 0x3f {
				return "ru"
			}
		}
	}

	return "en"
}

// T возвращает переведённую строку по ключу для текущего языка.
func T(key string, args ...interface{}) string {
	mu.RLock()
	lang := currentLang
	mu.RUnlock()

	dict, ok := translations[lang]
	if !ok {
		dict = translations["en"]
	}

	msg, found := dict[key]
	if !found {
		// Fallback to English dictionary if key missing in target language
		msg, found = translations["en"][key]
		if !found {
			msg = key
		}
	}

	if len(args) > 0 {
		return fmt.Sprintf(msg, args...)
	}
	return msg
}

// Словарь переводов: en (по умолчанию) и ru
var translations = map[string]map[string]string{
	"en": {
		// Header & Panes
		"topology_title":        "=== Network Topology: %s (version %s) ===",
		"pane_nodes":           "Node List (Nodes):",
		"pane_routes":          "Route List (Routes):",
		"pane_clients":         "Client List (Clients):",
		"pane_lists":           "Domain/IP Lists (Split Tunneling):",
		"pane_editor":          "Route Editor:",
		"editor_title":         "Route Editing: %s",
		"editor_hops":          "Hop Chain:",
		"editor_exit_node":     "Exit Node: %s",
		"editor_add_hop":       "[+] Add Hop",
		"editor_remove_hop":    "[-] Remove Hop",
		"editor_edit_route":    "[e] Edit Route",

		// Buttons
		"btn_add_node":         "[+ Add Node ('n')]",
		"btn_bootstrap_node":   "[SSH Bootstrap Node ('b')]",
		"btn_add_route":        "[+ Create Route ('r')]",
		"btn_add_client":       "[+ Add Client ('c')]",
		"btn_add_list":         "[+ Add List ('l')]",

		// Status Bar
		"status_info":          "Nodes: %d | Routes: %d | %s%s",
		"status_unsaved":       " * [UNSAVED - press 's']",

		// Log Messages
		"log_idle":             "System ready. Awaiting commands.",
		"log_saved":            "[OK] Configuration successfully saved to %s",
		"log_save_err":         "Save error: %v",
		"log_applying":         "-> Running full SSH Apply...",
		"log_node_added":       "[OK] Node %q (%s) successfully added!",
		"log_node_renamed":     "[OK] Node renamed: %q -> %q (updated in all routes & clients)!",
		"log_node_saved":       "[OK] Node settings for %q saved!",
		"log_route_created":    "[OK] Route %q created",
		"log_route_saved":      "[OK] Route settings for %q saved!",
		"log_client_added":     "[OK] Client %q added",
		"log_client_saved":     "[OK] Client settings for %q saved!",
		"log_list_added":       "[OK] Domain list %q added",
		"log_list_saved":       "[OK] List settings for %q saved!",
		"log_client_deleted":   "[OK] Client %q deleted",
		"log_list_deleted":     "[OK] List %q deleted",
		"log_node_deleted":     "[OK] Node %q deleted",
		"log_route_deleted":    "[OK] Route %q deleted",
		"log_teardown_start":   "-> Removing WG config from %q (%s)...",
		"log_teardown_done":    "[OK] WG config removed from %q!",
		"log_apply_done":       "[OK] Configuration successfully applied to all participating nodes via SSH!",
		"log_apply_fail":       "[FAIL] Apply error on nodes: %v",
		"log_hop_added":        "[OK] Added hop %s to route %s",
		"log_hop_removed":      "[OK] Removed hop %s from route %s",
		"log_route_protected":  "[!] Route %q is protected (protected: true) - editing denied",

		// Validation errors
		"err_name_ip_req":      "[!] Error: specify node name and IP",
		"err_node_taken":       "[!] Error: Node name %q is already taken!",
		"err_route_taken":      "[!] Error: Route name %q is already taken!",
		"err_client_taken":     "[!] Error: Client name %q is already taken!",
		"err_list_taken":       "[!] Error: List name %q is already taken!",
		"err_name_path_req":    "[!] Error: route name and path cannot be empty!",
		"err_client_name_req":  "[!] Error: client name cannot be empty!",
		"err_list_name_req":    "[!] Error: list name cannot be empty!",

		// Modals
		"modal_bootstrap":      "=== Zero-Touch SSH Bootstrap Node ===",
		"modal_add_node":       "=== Add Node ===",
		"modal_edit_node":      "=== Node Settings & Editing ===",
		"modal_add_route":      "=== Create Route ===",
		"modal_edit_route":     "=== Edit Route ===",
		"modal_add_client":     "=== Add Client ===",
		"modal_edit_client":    "=== Client Settings ===",
		"modal_add_list":       "=== Add Domain List ===",
		"modal_edit_list":      "=== Edit Domain List ===",
		"modal_confirm_prot":   "[!] ELEMENT PROTECTION (protected: true)",
		"modal_confirm_hint":   "Press [y] to confirm  |  [n / Esc] to cancel",
		"modal_doctor_title":   "=== wgmesh Doctor - Deep Diagnostics ===",
		"modal_export_title":   "=== Export Client Profile & QR Code ===",
		"modal_caps_title":     "=== Driver Platform Capabilities ===",
		"modal_git_title":      "=== Git Configuration Sync ===",
		"modal_update_title":   "=== wgmesh Auto-Update ===",
		"modal_help_title":     "=== wgmesh TUI Keymap Reference ===",

		// Help Modal Text
		"help_nav":             "  [Tab] / [Shift+Tab]   - Navigate panes (Topology, Nodes, Routes, Clients, Lists, Editor)\n  [Up / Down] or [j/k] - Move items in current pane\n  [+] / [-]             - Quick add/remove hop in route chain",
		"help_ctrl":            "Control Commands:\n  [b]                   - Zero-Touch SSH Bootstrap new node (with auto SSH key copy)\n  [n]                   - Add node to config\n  [r]                   - Create new exit route\n  [c]                   - Add client (with interactive ingress node selection)\n  [l]                   - Add domain/IP list (Split Tunneling)\n  [e]                   - Edit selected item\n  [Del] / [x]           - Delete selected node / route / client\n  [t]                   - Teardown WG config from node via SSH\n  [k]                   - Show platform capabilities of node",
		"help_sys":             "System & Export:\n  [a]                   - Apply configuration to servers via SSH (Apply)\n  [d]                   - wgmesh Doctor (Full network diagnostics)\n  [x]                   - Export profile / Render ASCII QR code\n  [g]                   - Git status, Commit & Push to repository\n  [u]                   - Check and install auto-update\n  [s]                   - Save changes to mesh.yaml\n  [?]                   - Show keymap help\n  [q] / [Ctrl+C]        - Quit TUI",

		// Doctor & Update Statuses
		"doctor_green":         "[GREEN] Overall status: GREEN (Network is fully healthy)",
		"doctor_yellow":        "[YELLOW] Overall status: YELLOW (There are warnings)",
		"doctor_red":           "[RED] Overall status: RED (Critical errors)",
		"update_checking":      "[...] Checking updates (current version: %s)...",
		"update_latest":        "[OK] You have the latest version (%s).",
		"update_new":           "[NEW] New version available: %s (current: %s)\n\nPress [Enter] to install update.",
		"update_installing":    "[...] Downloading and launching update script...",
	},
	"ru": {
		// Header & Panes
		"topology_title":        "=== Топология сети: %s (версия %s) ===",
		"pane_nodes":           "Список узлов (Nodes):",
		"pane_routes":          "Список маршрутов (Routes):",
		"pane_clients":         "Список клиентов (Clients):",
		"pane_lists":           "Списки доменов/IP (Split Tunneling):",
		"pane_editor":          "Редактор маршрута:",
		"editor_title":         "Редактирование маршрута: %s",
		"editor_hops":          "Цепочка хопов:",
		"editor_exit_node":     "Exit Node: %s",
		"editor_add_hop":       "[+] добавить хоп",
		"editor_remove_hop":    "[-] удалить хоп",
		"editor_edit_route":    "[e] изменить маршрут",

		// Buttons
		"btn_add_node":         "[+ Добавить ноду ('n')]",
		"btn_bootstrap_node":   "[SSH Bootstrap ноды ('b')]",
		"btn_add_route":        "[+ Создать маршрут ('r')]",
		"btn_add_client":       "[+ Добавить клиента ('c')]",
		"btn_add_list":         "[+ Добавить список ('l')]",

		// Status Bar
		"status_info":          "Узлы: %d | Маршруты: %d | %s%s",
		"status_unsaved":       " * [НЕ СОХРАНЕНО — нажмите 's']",

		// Log Messages
		"log_idle":             "Система готова к работе. Ожидание команд.",
		"log_saved":            "[OK] Конфигурация успешно сохранена в %s",
		"log_save_err":         "Ошибка сохранения: %v",
		"log_applying":         "-> Запуск полного применения (Apply) по SSH...",
		"log_node_added":       "[OK] Нода %q (%s) успешно добавлена!",
		"log_node_renamed":     "[OK] Нода переименована: %q -> %q (обновлена во всех маршрутах и клиентах)!",
		"log_node_saved":       "[OK] Настройки ноды %q сохранены!",
		"log_route_created":    "[OK] Маршрут %q создан",
		"log_route_saved":      "[OK] Настройки маршрута %q сохранены!",
		"log_client_added":     "[OK] Клиент %q добавлен",
		"log_client_saved":     "[OK] Настройки клиента %q сохранены!",
		"log_list_added":       "[OK] Список доменов %q добавлен",
		"log_list_saved":       "[OK] Настройки списка %q сохранены!",
		"log_client_deleted":   "[OK] Клиент %q удалён",
		"log_list_deleted":     "[OK] Список %q удалён",
		"log_node_deleted":     "[OK] Нода %q удалена",
		"log_route_deleted":    "[OK] Маршрут %q удалён",
		"log_teardown_start":   "-> Удаляю WG-конфигурацию с %q (%s)...",
		"log_teardown_done":    "[OK] WG конфигурация с %q удалена!",
		"log_apply_done":       "[OK] Конфигурация успешно применена ко всем участвующим нодам по SSH!",
		"log_apply_fail":       "[FAIL] Ошибка применения на нодах: %v",
		"log_hop_added":        "[OK] Добавлен хоп %s в маршрут %s",
		"log_hop_removed":      "[OK] Удалён хоп %s из маршрута %s",
		"log_route_protected":  "[!] Маршрут %q защищён (protected: true) - редактирование запрещено",

		// Validation errors
		"err_name_ip_req":      "[!] Ошибка: укажите имя и IP ноды",
		"err_node_taken":       "[!] Ошибка: Имя ноды %q уже занято!",
		"err_route_taken":      "[!] Ошибка: Имя маршрута %q уже занято!",
		"err_client_taken":     "[!] Ошибка: Имя клиента %q уже занято!",
		"err_list_taken":       "[!] Ошибка: Имя списка %q уже занято!",
		"err_name_path_req":    "[!] Ошибка: имя и путь маршрута не могут быть пустыми!",
		"err_client_name_req":  "[!] Ошибка: имя клиента не может быть пустым!",
		"err_list_name_req":    "[!] Ошибка: имя списка не может быть пустым!",

		// Modals
		"modal_bootstrap":      "=== Zero-Touch SSH Bootstrap Ноды ===",
		"modal_add_node":       "=== Добавление Ноды ===",
		"modal_edit_node":      "=== Настройки и Редактирование Ноды ===",
		"modal_add_route":      "=== Создание Маршрута ===",
		"modal_edit_route":     "=== Редактирование Маршрута ===",
		"modal_add_client":     "=== Добавление Клиента ===",
		"modal_edit_client":    "=== Настройки Клиента ===",
		"modal_add_list":       "=== Добавление Доменного Списка ===",
		"modal_edit_list":      "=== Редактирование Доменного Списка ===",
		"modal_confirm_prot":   "[!] ЗАЩИТА ЭЛЕМЕНТА (protected: true)",
		"modal_confirm_hint":   "Нажмите [y] для подтверждения  |  [n / Esc] для отмены",
		"modal_doctor_title":   "=== wgmesh Doctor - Глубокая Диагностика ===",
		"modal_export_title":   "=== Экспорт Клиентского Профиля & QR-Код ===",
		"modal_caps_title":     "=== Возможности Платформы Драйвера ===",
		"modal_git_title":      "=== Git Синхронизация Конфигурации ===",
		"modal_update_title":   "=== Авто-обновление wgmesh ===",
		"modal_help_title":     "=== Справка по клавишам wgmesh TUI ===",

		// Help Modal Text
		"help_nav":             "  [Tab] / [Shift+Tab]   - Навигация по панелям (Топология, Узлы, Маршруты, Клиенты, Списки, Редактор)\n  [Up / Down] или [j/k] - Перемещение по элементам текущей панели\n  [+] / [-]             - Быстрое добавление/удаление хопа в цепочке маршрута",
		"help_ctrl":            "Команды управления:\n  [b]                   - Zero-Touch SSH Bootstrap новой ноды (с автоподключением по паролю)\n  [n]                   - Добавить ноду в конфиг\n  [r]                   - Создать новый exit-маршрут\n  [c]                   - Добавить клиента (с интерактивным выбором ноды подключения)\n  [l]                   - Добавить список доменов/IP (Split Tunneling)\n  [e]                   - Изменить выбранный маршрут\n  [Del] / [x]           - Удалить выбранный узел / маршрут / клиент\n  [t]                   - Teardown WG-конфига с ноды по SSH\n  [k]                   - Показать технологические возможности ноды",
		"help_sys":             "Система и Экспорт:\n  [a]                   - Применить всю конфигурацию на сервера по SSH (Apply)\n  [d]                   - wgmesh Doctor (Полная диагностика сети)\n  [x]                   - Экспорт профиля / Генерация ASCII QR-кода\n  [g]                   - Git status, Commit & Push в репозиторий\n  [u]                   - Проверить и установить авто-обновление\n  [s]                   - Сохранить изменения в mesh.yaml\n  [?]                   - Справка по горячим клавишам\n  [q] / [Ctrl+C]        - Выход из TUI",

		// Doctor & Update Statuses
		"doctor_green":         "[GREEN] Итоговый статус: GREEN (Сеть полностью здорова)",
		"doctor_yellow":        "[YELLOW] Итоговый статус: YELLOW (Есть предупреждения)",
		"doctor_red":           "[RED] Итоговый статус: RED (Критические ошибки)",
		"update_checking":      "[...] Проверка обновлений (текущая версия: %s)...",
		"update_latest":        "[OK] У вас установлена самая актуальная версия программы (%s).",
		"update_new":           "[NEW] Найдена новая версия: %s (текущая: %s)\n\nНажмите [Enter] для установки обновления.",
		"update_installing":    "[...] Скачивание и запуск скрипта обновления...",
	},
}
