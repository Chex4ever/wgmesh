package cli

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"

	"github.com/meshctl/meshctl/internal/tui"
)

func tuiCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "tui",
		Short: "Запустить интерактивный TUI-интерфейс",
		Long:  `Интерактивный TUI-режим для просмотра топологии, редактирования маршрутов и применения изменений.`,
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			m, err := loadMesh()
			if err != nil {
				return err
			}

			model := tui.NewModel(m, cfgPath)
			p := tea.NewProgram(model, tea.WithAltScreen())

			if _, err := p.Run(); err != nil {
				return fmt.Errorf("ошибка TUI: %w", err)
			}
			return nil
		},
	}
}
