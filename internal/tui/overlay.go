package tui

import (
	"regexp"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type ModalTickMsg time.Time

func animateModalCmd() tea.Cmd {
	return tea.Tick(16*time.Millisecond, func(t time.Time) tea.Msg {
		return ModalTickMsg(t)
	})
}

var ansiRegex = regexp.MustCompile(`\x1b\[[0-9;]*[a-zA-Z]`)

func stripANSI(s string) string {
	return ansiRegex.ReplaceAllString(s, "")
}

func dimBackground(screen string) string {
	lines := strings.Split(screen, "\n")
	dimStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("238"))
	var dimmed []string
	for _, l := range lines {
		plain := stripANSI(l)
		dimmed = append(dimmed, dimStyle.Render(plain))
	}
	return strings.Join(dimmed, "\n")
}

func renderAnimatedModalOverlay(m Model, mainScreen string) string {
	if m.Modal.Type == ModalNone {
		return mainScreen
	}

	dimmedScreen := dimBackground(mainScreen)

	modalBody := renderModalOverlay(m)
	if modalBody == "" {
		return mainScreen
	}

	targetW := lipgloss.Width(modalBody)
	targetH := lipgloss.Height(modalBody)

	targetX := (m.Width - targetW) / 2
	targetY := (m.Height - targetH) / 2
	if targetX < 0 {
		targetX = 0
	}
	if targetY < 0 {
		targetY = 0
	}

	currX := targetX
	currY := targetY
	renderedBox := modalBody

	if m.Modal.IsAnimating && m.Modal.AnimProgress < 1.0 {
		p := m.Modal.AnimProgress

		if m.Modal.IsClosing {
			// Warp Hyper-Jump Closing Animation (Zoom-Through expansion into camera)
			easedP := p * p
			scale := 1.0 + 0.5*easedP

			currW := int(float64(targetW) * scale)
			currH := int(float64(targetH) * scale)

			currX = targetX - (currW-targetW)/2
			currY = targetY - (currH-targetH)/2

			var borderColor lipgloss.Color
			switch {
			case p < 0.35:
				borderColor = lipgloss.Color("51") // Electric Cyan
			case p < 0.70:
				borderColor = lipgloss.Color("201") // Neon Purple / Warp Glow
			default:
				borderColor = lipgloss.Color("231") // Hyper White Flash
			}

			renderedBox = renderFormModalBoxWithColor(m, currW, currH, borderColor)
		} else {
			// Opening Animation (Expanding from click origin)
			easedP := 1.0 - (1.0-p)*(1.0-p)

			origX := m.Modal.OriginX
			origY := m.Modal.OriginY
			if origX <= 0 {
				origX = m.Width / 2
			}
			if origY <= 0 {
				origY = m.Height / 2
			}

			currX = int(float64(origX)*(1.0-easedP) + float64(targetX)*easedP)
			currY = int(float64(origY)*(1.0-easedP) + float64(targetY)*easedP)

			currW := int(float64(targetW) * (0.3 + 0.7*easedP))
			currH := int(float64(targetH) * (0.3 + 0.7*easedP))

			if currW < 16 {
				currW = 16
			}
			if currH < 4 {
				currH = 4
			}

			borderColor := lipgloss.Color("51") // Cyan energy flash on open
			if p > 0.7 {
				borderColor = lipgloss.Color("86") // Smooth transition to standard theme color
			}

			renderedBox = renderFormModalBoxWithColor(m, currW, currH, borderColor)
		}
	}

	return compositeOverlay(dimmedScreen, renderedBox, currX, currY)
}

func compositeOverlay(bgScreen, fgBox string, posX, posY int) string {
	bgLines := strings.Split(bgScreen, "\n")
	fgLines := strings.Split(fgBox, "\n")

	outLines := make([]string, len(bgLines))
	copy(outLines, bgLines)

	for i, fgLine := range fgLines {
		destY := posY + i
		if destY < 0 || destY >= len(outLines) {
			continue
		}

		bgPlain := stripANSI(outLines[destY])
		bgRunes := []rune(bgPlain)
		bgLen := len(bgRunes)
		fgVisWidth := lipgloss.Width(fgLine)
		if fgVisWidth == 0 {
			continue
		}

		endX := posX + fgVisWidth
		if endX <= 0 {
			continue
		}

		var leftPart, rightPart string
		dimStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("238"))

		if posX <= 0 {
			leftPart = ""
		} else if posX < bgLen {
			leftPart = dimStyle.Render(string(bgRunes[:posX]))
		} else {
			pad := strings.Repeat(" ", posX-bgLen)
			leftPart = dimStyle.Render(string(bgRunes)) + pad
		}

		if endX >= 0 && endX < bgLen {
			rightPart = dimStyle.Render(string(bgRunes[endX:]))
		} else {
			rightPart = ""
		}

		outLines[destY] = leftPart + fgLine + rightPart
	}

	return strings.Join(outLines, "\n")
}
