package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/wgmesh/wgmesh/internal/i18n"
)

func renderFormModalBox(m Model, width, height int) string {
	return renderFormModalBoxWithColor(m, width, height, lipgloss.Color("86"))
}

func renderFormModalBoxWithColor(m Model, width, height int, borderColor lipgloss.Color) string {
	innerWidth := width - 4
	if innerWidth < 10 {
		innerWidth = 10
	}
	body := renderModalBodyContent(m, innerWidth)
	rendered := modalBoxStyle.Width(width).BorderForeground(borderColor).Render(body)

	lines := strings.Split(rendered, "\n")
	if len(lines) > height {
		lines = lines[:height]
	}
	return strings.Join(lines, "\n")
}

func renderModalOverlay(m Model) string {
	boxWidth := min(86, m.Width-4)
	innerWidth := boxWidth - 4
	if innerWidth < 30 {
		innerWidth = 30
	}

	body := renderModalBodyContent(m, innerWidth)
	if body == "" {
		return ""
	}

	return modalBoxStyle.
		Width(boxWidth).
		Render(body)
}

func renderModalBodyContent(m Model, innerWidth int) string {
	switch m.Modal.Type {
	case ModalHelp:
		return renderHelpContent(innerWidth)
	case ModalBootstrapNode:
		return renderFormModal(i18n.T("modal_bootstrap"), m.Modal, innerWidth)
	case ModalAddNode:
		return renderFormModal(i18n.T("modal_add_node"), m.Modal, innerWidth)
	case ModalEditNode:
		return renderFormModal(i18n.T("modal_edit_node"), m.Modal, innerWidth)
	case ModalAddRoute:
		return renderFormModal(i18n.T("modal_add_route"), m.Modal, innerWidth)
	case ModalEditRoute:
		return renderFormModal(i18n.T("modal_edit_route"), m.Modal, innerWidth)
	case ModalAddClient:
		return renderFormModal(i18n.T("modal_add_client"), m.Modal, innerWidth)
	case ModalEditClient:
		return renderFormModal(i18n.T("modal_edit_client"), m.Modal, innerWidth)
	case ModalAddList:
		return renderFormModal(i18n.T("modal_add_list"), m.Modal, innerWidth)
	case ModalEditList:
		return renderFormModal(i18n.T("modal_edit_list"), m.Modal, innerWidth)
	case ModalConfirm:
		return renderConfirmModal(m.Modal, innerWidth)
	case ModalDoctor:
		return renderDoctorModal(m.Modal, innerWidth)
	case ModalExport:
		return renderExportModal(m.Modal, innerWidth)
	case ModalCapabilities:
		return renderCapModal(m.Modal, innerWidth)
	case ModalGit:
		return renderGitModal(m.Modal, innerWidth)
	case ModalUpdate:
		return renderUpdateModal(m.Modal, innerWidth)
	default:
		return ""
	}
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

func renderFormModal(title string, state ModalState, innerWidth int) string {
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
				sb.WriteString(activeFieldStyle.Width(innerWidth).Render(fmt.Sprintf("► %-26s: %s", f.Label, val)) + "\n")
			} else {
				sb.WriteString(activeFieldStyle.Width(innerWidth).Render(fmt.Sprintf("► %-26s: [%s_]", f.Label, val)) + "\n")
			}
		} else {
			if len(f.Options) > 0 && f.OptionIdx >= 0 && f.OptionIdx < len(f.Options) {
				val = f.Options[f.OptionIdx]
			}
			sb.WriteString(fieldLabelStyle.Width(innerWidth).Render(fmt.Sprintf("  %-26s: %s", f.Label, val)) + "\n")
		}
	}

	noteStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("242")).Width(innerWidth)
	if hasStar {
		sb.WriteString("\n" + noteStyle.Render("* Пароль SSH используется только для автозагрузки ключа и нигде не сохраняется"))
	}
	if hasStarStar {
		sb.WriteString("\n" + noteStyle.Render("** При включении защиты перед удалением или редактированием потребуется подтверждение"))
	}

	hintStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("243")).Width(innerWidth)
	sb.WriteString("\n\n" + hintStyle.Render(
		"[Tab/Enter/↓] Навигация | [←/→] Варианты | [Enter] Сохранить | [Esc] Отмена",
	))

	return sb.String()
}

func renderConfirmModal(state ModalState, innerWidth int) string {
	var sb strings.Builder
	sb.WriteString(warnStyle.Width(innerWidth).Render(i18n.T("modal_confirm_prot")) + "\n\n")
	sb.WriteString(state.ConfirmPrompt + "\n\n")
	sb.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("86")).Width(innerWidth).Render(
		i18n.T("modal_confirm_hint"),
	))
	return sb.String()
}

func renderDoctorModal(state ModalState, innerWidth int) string {
	var sb strings.Builder
	sb.WriteString(titleStyle.Render(i18n.T("modal_doctor_title")) + "\n")
	sb.WriteString(strings.Repeat("-", innerWidth) + "\n\n")

	for _, line := range state.DoctorOutput {
		sb.WriteString(line + "\n")
	}

	sb.WriteString("\n" + strings.Repeat("-", innerWidth) + "\n")
	if state.DoctorStatus == "GREEN" {
		sb.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("82")).Bold(true).Width(innerWidth).Render(i18n.T("doctor_green")))
	} else if state.DoctorStatus == "YELLOW" {
		sb.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("214")).Bold(true).Width(innerWidth).Render(i18n.T("doctor_yellow")))
	} else {
		sb.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("196")).Bold(true).Width(innerWidth).Render(i18n.T("doctor_red")))
	}
	sb.WriteString("\n\n" + lipgloss.NewStyle().Foreground(lipgloss.Color("243")).Width(innerWidth).Render(
		"[r] Retry | [Esc] Close",
	))

	return sb.String()
}

func renderExportModal(state ModalState, innerWidth int) string {
	var sb strings.Builder
	sb.WriteString(titleStyle.Render(i18n.T("modal_export_title")) + "\n\n")

	if state.ShowQR && state.QRString != "" {
		sb.WriteString(state.QRString + "\n\n")
	} else if state.ExportData != "" {
		lines := strings.Split(state.ExportData, "\n")
		if len(lines) > 15 {
			lines = append(lines[:15], "…")
		}
		sb.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("252")).Width(innerWidth).Render(strings.Join(lines, "\n")) + "\n\n")
	}

	sb.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("243")).Width(innerWidth).Render(
		"[w] WireGuard | [a] Amnezia | [s] Sing-box | [u] URI | [q] QR-code | [Esc] Close",
	))

	return sb.String()
}

func renderCapModal(state ModalState, innerWidth int) string {
	var sb strings.Builder
	sb.WriteString(titleStyle.Render(i18n.T("modal_caps_title")) + "\n\n")
	sb.WriteString(state.CapText + "\n\n")
	sb.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("243")).Width(innerWidth).Render(
		"[Esc] Close",
	))
	return sb.String()
}

func renderGitModal(state ModalState, innerWidth int) string {
	var sb strings.Builder
	sb.WriteString(titleStyle.Render(i18n.T("modal_git_title")) + "\n\n")
	sb.WriteString("Status:\n")
	sb.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("250")).Width(innerWidth).Render(state.GitStatus) + "\n\n")

	for i, f := range state.Fields {
		val := f.Value
		if val == "" && f.Placeholder != "" {
			val = lipgloss.NewStyle().Foreground(lipgloss.Color("242")).Render(f.Placeholder)
		}
		if i == state.ActiveField {
			sb.WriteString(activeFieldStyle.Width(innerWidth).Render(fmt.Sprintf("► %-24s: [%s_]", f.Label, val)) + "\n")
		} else {
			sb.WriteString(fieldLabelStyle.Width(innerWidth).Render(fmt.Sprintf("  %-24s: %s", f.Label, val)) + "\n")
		}
	}

	sb.WriteString("\n" + lipgloss.NewStyle().Foreground(lipgloss.Color("243")).Width(innerWidth).Render(
		"[Enter] Commit & Push  |  [Esc] Close",
	))
	return sb.String()
}

func renderUpdateModal(state ModalState, innerWidth int) string {
	var sb strings.Builder
	sb.WriteString(titleStyle.Render(i18n.T("modal_update_title")) + "\n\n")

	if state.UpdateStatus != "" {
		sb.WriteString(state.UpdateStatus + "\n\n")
	}

	if state.IsUpdating {
		sb.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("214")).Bold(true).Width(innerWidth).Render(i18n.T("update_installing")) + "\n\n")
	} else if state.HasUpdate {
		sb.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("86")).Width(innerWidth).Render(
			"[Enter] Install Update  |  [Esc] Cancel",
		))
	} else {
		sb.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("243")).Width(innerWidth).Render(
			"[Esc] Close",
		))
	}

	return sb.String()
}

func renderHelpContent(innerWidth int) string {
	return titleStyle.Render(i18n.T("modal_help_title")) + "\n\n" +
		i18n.T("help_nav") + "\n\n" +
		i18n.T("help_ctrl") + "\n\n" +
		i18n.T("help_sys") + "\n"
}
