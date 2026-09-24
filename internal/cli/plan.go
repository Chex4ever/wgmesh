package cli

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/meshctl/meshctl/internal/mesh"
)

func planCmd() *cobra.Command {
	var (
		routeFilter string
		jsonOut     bool
	)
	cmd := &cobra.Command{
		Use:   "plan",
		Short: "Показать план применения маршрутизации (dry-plan)",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			m, err := loadMesh()
			if err != nil {
				return err
			}
			if err := validateMesh(m); err != nil {
				return err
			}

			mgr := mesh.NewManager(m)
			plans := mgr.BuildPlan()

			if jsonOut {
				enc := json.NewEncoder(os.Stdout)
				enc.SetIndent("", "  ")
				return enc.Encode(plans)
			}

			fmt.Printf("План применения mesh %q:\n\n", m.Name)
			for _, p := range plans {
				if routeFilter != "" {
					matched := false
					for _, rName := range p.ClientRoutes {
						if rName == routeFilter {
							matched = true
							break
						}
					}
					if !matched {
						continue
					}
				}
				fmt.Printf("— Нода %q (%s, IP: %s):\n", p.Node.Name, p.Node.Type, p.Node.MeshIP)
				fmt.Printf("    Пиры: %v\n", p.PeerNames)
				fmt.Printf("    Роль: Exit=%v, Relay=%v\n", p.IsExit, p.IsRelay)
				if len(p.ClientRoutes) > 0 {
					fmt.Printf("    Клиентские маршруты: %v\n", p.ClientRoutes)
				}
				fmt.Println()
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&routeFilter, "route", "", "фильтр по конкретному маршруту")
	cmd.Flags().BoolVar(&jsonOut, "json", false, "вывод в формате JSON")
	return cmd
}
