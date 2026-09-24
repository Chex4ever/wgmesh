package cli

import (
	"fmt"
	"os"

	"github.com/skip2/go-qrcode"
	"github.com/spf13/cobra"

	"github.com/meshctl/meshctl/internal/config"
	"github.com/meshctl/meshctl/internal/export"
)

func exportCmd() *cobra.Command {
	var (
		clientName string
		formatName string
		outPath    string
		showQR     bool
	)
	cmd := &cobra.Command{
		Use:   "export <route>",
		Short: "Экспорт профиля клиента в форматы WireGuard, Amnezia, Sing-box, URI, QR",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			m, err := loadMesh()
			if err != nil {
				return err
			}
			routeName := args[0]
			route := m.RouteByName(routeName)
			if route == nil {
				return fmt.Errorf("маршрут %q не найден", routeName)
			}

			var client *config.Client
			if clientName != "" {
				client = m.ClientByName(clientName)
				if client == nil {
					return fmt.Errorf("клиент %q не найден", clientName)
				}
			}

			exp, err := export.Get(formatName)
			if err != nil {
				return err
			}

			data, err := exp.Render(m, route, client)
			if err != nil {
				return fmt.Errorf("ошибка экспорта: %w", err)
			}

			if outPath != "" {
				if err := os.WriteFile(outPath, data, 0o600); err != nil {
					return fmt.Errorf("не удалось записать в %s: %w", outPath, err)
				}
				fmt.Printf("✔ Профиль успешно экспортирован в %s\n", outPath)
			} else if !showQR {
				fmt.Print(string(data))
			}

			if showQR {
				qr, err := qrcode.New(string(data), qrcode.Medium)
				if err != nil {
					return fmt.Errorf("не удалось сгенерировать QR-код: %w", err)
				}
				fmt.Println(qr.ToSmallString(false))
			}

			return nil
		},
	}
	cmd.Flags().StringVar(&clientName, "client", "", "имя клиента из раздела clients")
	cmd.Flags().StringVar(&formatName, "format", "wireguard", "формат экспорта: wireguard | amnezia | sing-box | uri")
	cmd.Flags().StringVarP(&outPath, "out", "o", "", "сохранить вывод в файл")
	cmd.Flags().BoolVar(&showQR, "qr", false, "показать ASCII QR-код в терминале")
	return cmd
}
