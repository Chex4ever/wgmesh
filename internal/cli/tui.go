package cli

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"

	"github.com/meshctl/meshctl/internal/config"
	"github.com/meshctl/meshctl/internal/tui"
)

func runTUICmd(cmd *cobra.Command, args []string) error {
	m, err := loadMesh()
	if err != nil {
		m = config.DefaultMesh()
		_ = saveMesh(m)
	}

	model := tui.NewModel(m, cfgPath, Version)
	p := tea.NewProgram(model, tea.WithAltScreen(), tea.WithMouseCellMotion())

	if _, err := p.Run(); err != nil {
		return fmt.Errorf("ошибка TUI: %w", err)
	}
	return nil
}

func tuiCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "tui",
		Short: "Запустить интерактивный TUI-интерфейс",
		Long:  `Интерактивный TUI-режим для полного управления mesh-сетью.`,
		Args:  cobra.NoArgs,
		RunE:  runTUICmd,
	}
}
