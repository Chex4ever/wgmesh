package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/wgmesh/wgmesh/internal/config"
)

// initCmd — `wgmesh init`: создаёт mesh.yaml с дефолтной конфигурацией.
func initCmd() *cobra.Command {
	var name string
	cmd := &cobra.Command{
		Use:   "init",
		Short: "Создать новый mesh-конфиг (mesh.yaml)",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if config.Exists(cfgPath) {
				return fmt.Errorf("файл %s уже существует — не перезаписываю", cfgPath)
			}
			m := config.DefaultMesh()
			if name != "" {
				m.Name = name
			}
			if err := config.Save(cfgPath, m); err != nil {
				return err
			}
			fmt.Printf("✔ Создан %s (сеть %q). Дальше:\n", cfgPath, m.Name)
			fmt.Println("  wgmesh node bootstrap <name> --host <ip>")
			fmt.Println("  wgmesh route add via-1 --path client,<name> --exit <name>")
			fmt.Println("  wgmesh apply && wgmesh export client via-1 --qr")
			return nil
		},
	}
	cmd.Flags().StringVar(&name, "name", "", "название сети")
	return cmd
}

