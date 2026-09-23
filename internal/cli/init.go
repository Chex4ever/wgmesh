package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/meshctl/meshctl/internal/config"
)

// initCmd — `meshctl init`: создаёт mesh.yaml с дефолтной конфигурацией.
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
			fmt.Println("  meshctl node add <name> --type linux --host <ip> --user root")
			fmt.Println("  meshctl route add via-1 --path client,<name> --exit <name>")
			fmt.Println("  meshctl apply && meshctl client-config via-1 --qr")
			return nil
		},
	}
	cmd.Flags().StringVar(&name, "name", "", "название сети")
	return cmd
}
