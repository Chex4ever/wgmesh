package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/wgmesh/wgmesh/internal/updater"
)

func updateCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "update",
		Short: "Проверить и установить обновление wgmesh",
		Long:  `Проверяет наличие новых релизов на GitHub и автоматически обновляет бинарный файл.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			cmd.Printf("🔍 Проверка обновлений (текущая версия: %s)...\n", Version)
			hasUpdate, latestVersion, downloadURL, err := updater.CheckForUpdate(Version)
			if err != nil {
				return fmt.Errorf("ошибка проверки обновлений: %w", err)
			}

			if !hasUpdate {
				cmd.Printf("✔ У вас установлена самая актуальная версия программы (%s).\n", Version)
				return nil
			}

			cmd.Printf("🆕 Найдена новая версия: %s. Скачивание и установка...\n", latestVersion)
			if err := updater.PerformUpdate(downloadURL); err != nil {
				return fmt.Errorf("ошибка обновления: %w", err)
			}

			cmd.Println("✔ Обновление успешно выполнено!")
			return nil
		},
	}
}

