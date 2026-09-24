package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/meshctl/meshctl/internal/i18n"
)

func renderModalOverlay(m Model) string {
	var body string

	switch m.Modal.Type {
	case ModalHelp:
		body = renderHelpContent()
	case ModalBootstrapNode:
		body = renderFormModal(i18n.T("modal_bootstrap"), m.Modal)
	case ModalAddNode:
		body = renderFormModal(i18n.T("modal_add_node"), m.Modal)
	case ModalEditNode:
		body = renderFormModal(i18n.T("modal_edit_node"), m.Modal)
	case ModalAddRoute:
		body = renderFormModal(i18n.T("modal_add_route"), m.Modal)
	case ModalEditRoute:
		body = renderFormModal(i18n.T("modal_edit_route"), m.Modal)
	case ModalAddClient:
		body = renderFormModal(i18n.T("modal_add_client"), m.Modal)
	case ModalEditClient:
		body = renderFormModal(i18n.T("modal_edit_client"), m.Modal)
	case ModalAddList:
		body = renderFormModal(i18n.T("modal_add_list"), m.Modal)
	case ModalEditList:
		body = renderFormModal(i18n.T("modal_edit_list"), m.Modal)
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
	case ModalUpdate:
		body = renderUpdateModal(m.Modal)
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
	sb.WriteString(warnStyle.Render(i18n.T("modal_confirm_prot")) + "\n\n")
	sb.WriteString(state.ConfirmPrompt + "\n\n")
	sb.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("86")).Render(
		i18n.T("modal_confirm_hint"),
	))
	return sb.String()
}

func renderDoctorModal(state ModalState) string {
	var sb strings.Builder
	sb.WriteString(titleStyle.Render(i18n.T("modal_doctor_title")) + "\n")
	sb.WriteString("------------------------------------------------------------------\n\n")

	for _, line := range state.DoctorOutput {
		sb.WriteString(line + "\n")
	}

	sb.WriteString("\n------------------------------------------------------------------\n")
	if state.DoctorStatus == "GREEN" {
		sb.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("82")).Bold(true).Render(i18n.T("doctor_green")))
	} else if state.DoctorStatus == "YELLOW" {
		sb.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("214")).Bold(true).Render(i18n.T("doctor_yellow")))
	} else {
		sb.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("196")).Bold(true).Render(i18n.T("doctor_red")))
	}
	sb.WriteString("\n\n" + lipgloss.NewStyle().Foreground(lipgloss.Color("243")).Render(
		"[r] Retry | [Esc] Close",
	))

	return sb.String()
}

func renderExportModal(state ModalState) string {
	var sb strings.Builder
	sb.WriteString(titleStyle.Render(i18n.T("modal_export_title")) + "\n\n")

	if state.ShowQR && state.QRString != "" {
		sb.WriteString(state.QRString + "\n\n")
	} else if state.ExportData != "" {
		lines := strings.Split(state.ExportData, "\n")
		if len(lines) > 15 {
			lines = append(lines[:15], "…")
		}
		sb.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("252")).Render(strings.Join(lines, "\n")) + "\n\n")
	}

	sb.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("243")).Render(
		"[w] WireGuard | [a] Amnezia | [s] Sing-box | [u] URI | [q] QR-code | [Esc] Close",
	))

	return sb.String()
}

func renderCapModal(state ModalState) string {
	var sb strings.Builder
	sb.WriteString(titleStyle.Render(i18n.T("modal_caps_title")) + "\n\n")
	sb.WriteString(state.CapText + "\n\n")
	sb.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("243")).Render(
		"[Esc] Close",
	))
	return sb.String()
}

func renderGitModal(state ModalState) string {
	var sb strings.Builder
	sb.WriteString(titleStyle.Render(i18n.T("modal_git_title")) + "\n\n")
	sb.WriteString("Status:\n")
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
		"[Enter] Commit & Push  |  [Esc] Close",
	))
	return sb.String()
}

func renderUpdateModal(state ModalState) string {
	var sb strings.Builder
	sb.WriteString(titleStyle.Render(i18n.T("modal_update_title")) + "\n\n")

	if state.UpdateStatus != "" {
		sb.WriteString(state.UpdateStatus + "\n\n")
	}

	if state.IsUpdating {
		sb.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("214")).Bold(true).Render(i18n.T("update_installing")) + "\n\n")
	} else if state.HasUpdate {
		sb.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("86")).Render(
			"[Enter] Install Update  |  [Esc] Cancel",
		))
	} else {
		sb.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("243")).Render(
			"[Esc] Close",
		))
	}

	return sb.String()
}

func renderHelpContent() string {
	return titleStyle.Render(i18n.T("modal_help_title")) + "\n\n" +
		i18n.T("help_nav") + "\n\n" +
		i18n.T("help_ctrl") + "\n\n" +
		i18n.T("help_sys") + "\n"
}
