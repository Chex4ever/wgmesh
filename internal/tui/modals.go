package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

func renderModalOverlay(m Model) string {
	var body string

	switch m.Modal.Type {
	case ModalHelp:
		body = renderHelpContent()
	case ModalBootstrapNode:
		body = renderFormModal("🚀 Zero-Touch SSH Bootstrap Ноды", m.Modal)
	case ModalAddNode:
		body = renderFormModal("➕ Добавление Ноды", m.Modal)
	case ModalEditNode:
		body = renderFormModal("✏️ Настройки и Редактирование Ноды", m.Modal)
	case ModalAddRoute:
		body = renderFormModal("➕ Создание Маршрута", m.Modal)
	case ModalEditRoute:
		body = renderFormModal("✏️ Редактирование Маршрута", m.Modal)
	case ModalAddClient:
		body = renderFormModal("👤 Добавление Клиента", m.Modal)
	case ModalEditClient:
		body = renderFormModal("✏️ Настройки Клиента", m.Modal)
	case ModalAddList:
		body = renderFormModal("🌐 Добавление Доменного Списка", m.Modal)
	case ModalEditList:
		body = renderFormModal("✏️ Редактирование Доменного Списка", m.Modal)
	case ModalConfirm:
		body = renderConfirmModal(m.Modal)
	case ModalDoctor:
		body = renderDoctorModal(m.Modal)
	case ModalExport:
		body = renderExportModal(m.Modal)
	case ModalCapabilities:
		body = renderCapModal(m.Modal)
	case ModalGit:
		body = renderGitModal(m.Modal)
	default:
		return ""
	}

	return modalBoxStyle.
		Width(min(86, m.Width-4)).
		Render(body)
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

var (
	modalBoxStyle = lipgloss.NewStyle().
			Border(lipgloss.DoubleBorder()).
			BorderForeground(lipgloss.Color("86")).
			Padding(1, 2).
			Background(lipgloss.Color("234"))

	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("86"))

	fieldLabelStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("250"))

	activeFieldStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("86")).
				Bold(true)

	warnStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("208")).
			Bold(true)
)

func renderFormModal(title string, state ModalState) string {
	var sb strings.Builder
	sb.WriteString(titleStyle.Render(title) + "\n\n")

	hasStar := false
	hasStarStar := false

	for i, f := range state.Fields {
		if strings.Contains(f.Label, "**") {
			hasStarStar = true
		} else if strings.Contains(f.Label, "*") {
			hasStar = true
		}

		val := f.Value
		if len(f.Options) > 0 {
			if f.OptionIdx >= 0 && f.OptionIdx < len(f.Options) {
				val = fmt.Sprintf("◄ %s ►  (стрелки ←/→)", f.Options[f.OptionIdx])
			}
		} else if f.Mask {
			val = strings.Repeat("*", len(f.Value))
		} else if val == "" && f.Placeholder != "" {
			val = lipgloss.NewStyle().Foreground(lipgloss.Color("242")).Render(f.Placeholder)
		}

		if i == state.ActiveField {
			if len(f.Options) > 0 {
				sb.WriteString(activeFieldStyle.Render(fmt.Sprintf("► %-26s: %s", f.Label, val)) + "\n")
			} else {
				sb.WriteString(activeFieldStyle.Render(fmt.Sprintf("► %-26s: [%s_]", f.Label, val)) + "\n")
			}
		} else {
			if len(f.Options) > 0 && f.OptionIdx >= 0 && f.OptionIdx < len(f.Options) {
				val = f.Options[f.OptionIdx]
			}
			sb.WriteString(fieldLabelStyle.Render(fmt.Sprintf("  %-26s: %s", f.Label, val)) + "\n")
		}
	}

	noteStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("242"))
	if hasStar {
		sb.WriteString("\n" + noteStyle.Render("* Пароль SSH используется только для автозагрузки ключа и нигде не сохраняется"))
	}
	if hasStarStar {
		sb.WriteString("\n" + noteStyle.Render("** При включении защиты перед удалением или редактированием потребуется подтверждение"))
	}

	sb.WriteString("\n\n" + lipgloss.NewStyle().Foreground(lipgloss.Color("243")).Render(
		"[Tab/Enter/↓] Навигация | [←/→] Варианты | [Enter на посл. поле] Сохранить | [Esc] Отмена",
	))

	return sb.String()
}

func renderConfirmModal(state ModalState) string {
	var sb strings.Builder
	sb.WriteString(warnStyle.Render("⚠️  ЗАЩИТА ЭЛЕМЕНТА (protected: true)") + "\n\n")
	sb.WriteString(state.ConfirmPrompt + "\n\n")
	sb.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("86")).Render(
		"Нажмите [y] для подтверждения  |  [n / Esc] для отмены",
	))
	return sb.String()
}

func renderDoctorModal(state ModalState) string {
	var sb strings.Builder
	sb.WriteString(titleStyle.Render("🏥 wgmesh Doctor — Глубокая Диагностика") + "\n")
	sb.WriteString("------------------------------------------------------------------\n\n")

	for _, line := range state.DoctorOutput {
		sb.WriteString(line + "\n")
	}

	sb.WriteString("\n------------------------------------------------------------------\n")
	if state.DoctorStatus == "GREEN" {
		sb.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("82")).Bold(true).Render("🟢 Итоговый статус: GREEN (Сеть полностью здорова)"))
	} else if state.DoctorStatus == "YELLOW" {
		sb.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("214")).Bold(true).Render("🟡 Итоговый статус: YELLOW (Есть предупреждения)"))
	} else {
		sb.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("196")).Bold(true).Render("🔴 Итоговый статус: RED (Критические ошибки)"))
	}
	sb.WriteString("\n\n" + lipgloss.NewStyle().Foreground(lipgloss.Color("243")).Render(
		"[r] Запустить повторно | [Esc] Закрыть",
	))

	return sb.String()
}

func renderExportModal(state ModalState) string {
	var sb strings.Builder
	sb.WriteString(titleStyle.Render("📦 Экспорт Клиентского Профиля & QR-Код") + "\n\n")

	if state.ShowQR && state.QRString != "" {
		sb.WriteString(state.QRString + "\n\n")
	} else if state.ExportData != "" {
		lines := strings.Split(state.ExportData, "\n")
		if len(lines) > 15 {
			lines = append(lines[:15], "… (конфиг усечён для вывода)")
		}
		sb.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("252")).Render(strings.Join(lines, "\n")) + "\n\n")
	}

	sb.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("243")).Render(
		"[w] WireGuard | [a] Amnezia | [s] Sing-box | [u] URI | [q] QR-код | [Esc] Закрыть",
	))

	return sb.String()
}

func renderCapModal(state ModalState) string {
	var sb strings.Builder
	sb.WriteString(titleStyle.Render("💻 Возможности Платформы Драйвера") + "\n\n")
	sb.WriteString(state.CapText + "\n\n")
	sb.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("243")).Render(
		"[Esc] Закрыть",
	))
	return sb.String()
}

func renderGitModal(state ModalState) string {
	var sb strings.Builder
	sb.WriteString(titleStyle.Render("🐙 Git Синхронизация Конфигурации") + "\n\n")
	sb.WriteString("Статус репозитория:\n")
	sb.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("250")).Render(state.GitStatus) + "\n\n")

	for i, f := range state.Fields {
		val := f.Value
		if val == "" && f.Placeholder != "" {
			val = lipgloss.NewStyle().Foreground(lipgloss.Color("242")).Render(f.Placeholder)
		}
		if i == state.ActiveField {
			sb.WriteString(activeFieldStyle.Render(fmt.Sprintf("► %-24s: [%s_]", f.Label, val)) + "\n")
		} else {
			sb.WriteString(fieldLabelStyle.Render(fmt.Sprintf("  %-24s: %s", f.Label, val)) + "\n")
		}
	}

	sb.WriteString("\n" + lipgloss.NewStyle().Foreground(lipgloss.Color("243")).Render(
		"[Enter] Commit & Push  |  [Esc] Закрыть",
	))
	return sb.String()
}

func renderHelpContent() string {
	return titleStyle.Render("═══ Справка по клавишам wgmesh TUI ═══") + `

  [Tab] / [Shift+Tab]   — Навигация по панелям (Топология, Узлы, Маршруты, Клиенты, Списки, Редактор)
  [↑ / ↓] или [j / k]   — Перемещение по элементам текущей панели
  [+] / [-]             — Быстрое добавление/удаление хопа в цепочке маршрута

Команды управления:
  [b]                   — 🚀 Zero-Touch SSH Bootstrap новой ноды (с автоподключением по паролю)
  [n]                   — ➕ Добавить ноду в конфиг
  [r]                   — ➕ Создать новый exit-маршрут
  [c]                   — 👤 Добавить клиента (с интерактивным выбором ноды подключения)
  [l]                   — 🌐 Добавить список доменов/IP (Split Tunneling)
  [e]                   — ✏️ Изменить выбранный маршрут
  [Del] / [x]           — 🗑️ Удалить выбранный узел / маршрут / клиент
  [t]                   — 🧼 Teardown WG-конфига с ноды по SSH
  [k]                   — 💻 Показать технологические возможности ноды

Система и Экспорт:
  [a]                   — ⚡ Применить всю конфигурацию на сервера по SSH (Apply)
  [d]                   — 🏥 wgmesh Doctor (Полная диагностика сети)
  [x]                   — 📦 Экспорт профиля / Генерация ASCII QR-кода
  [g]                   — 🐙 Git status, Commit & Push в репозиторий
  [s]                   — 💾 Сохранить изменения в mesh.yaml
  [?]                   — ❓ Справка по горячим клавишам
  [q] / [Ctrl+C]        — 🚪 Выход из TUI
`
}
