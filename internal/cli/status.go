package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/meshctl/meshctl/internal/config"
	"github.com/meshctl/meshctl/internal/drivers"
)

func statusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status [<node>]",
		Short: "Показать статус WireGuard на нодах (через SSH)",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			m, err := loadMesh()
			if err != nil {
				return err
			}
			if err := validateMesh(m); err != nil {
				return err
			}

			targetNodes := m.Nodes
			if len(args) == 1 {
				nodeName := args[0]
				n := m.NodeByName(nodeName)
				if n == nil {
					return fmt.Errorf("нода %q не найдена", nodeName)
				}
				targetNodes = []config.Node{*n}
			}

			for _, node := range targetNodes {
				d, err := drivers.New(&node)
				if err != nil {
					fmt.Fprintf(os.Stderr, "✖ %s: %v\n", node.Name, err)
					continue
				}
				st, err := d.GetStatus(&node)
				if err != nil {
					fmt.Fprintf(os.Stderr, "✖ %s (%s): %v\n", node.Name, d.Name(), err)
					continue
				}
				fmt.Printf("=== Нода %q (%s, %s) ===\n%s\n\n", node.Name, d.Name(), node.Host, st)
			}
			return nil
		},
	}
}
