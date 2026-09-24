package cli

import (
	"encoding/json"
	"fmt"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/meshctl/meshctl/internal/config"
)

// routeCmd — `meshctl route ...`: add / edit / list / remove.
func routeCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "route",
		Short: "Управление exit-маршрутами (цепочками хопов)",
	}
	cmd.AddCommand(routeAddCmd(), routeEditCmd(), routeListCmd(), routeRemoveCmd())
	return cmd
}

func routeAddCmd() *cobra.Command {
	var pathStr, exit string
	cmd := &cobra.Command{
		Use:   "add <name>",
		Short: "Создать маршрут: --path client,nodeA,nodeB --exit nodeB",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			m, err := loadMesh()
			if err != nil {
				return err
			}
			name := args[0]
			if m.RouteByName(name) != nil {
				return fmt.Errorf("маршрут %q уже существует", name)
			}
			path := splitTrim(pathStr)
			if len(path) == 0 || path[0] != config.ClientHop {
				return fmt.Errorf("--path должен начинаться с 'client', например: client,%s", strings.Join(nodeNames(m), ","))
			}
			r := config.Route{Name: name, Path: path, ExitNode: exit}
			if r.ExitNode == "" && len(path) > 1 {
				r.ExitNode = path[len(path)-1] // по умолчанию — последний хоп
			}
			m.Routes = append(m.Routes, r)
			// валидация всего конфига с новым маршрутом
			if err := validateMesh(m); err != nil {
				return err
			}
			if err := saveMesh(m); err != nil {
				return err
			}
			fmt.Printf("✔ Маршрут %q: %s → 🌐 (%s)\n", name, strings.Join(path, " → "), r.ExitNode)
			return nil
		},
	}
	cmd.Flags().StringVar(&pathStr, "path", "", "цепочка через запятую, начиная с client")
	cmd.Flags().StringVar(&exit, "exit", "", "exit-нода (по умолчанию последняя в --path)")
	return cmd
}

func routeListCmd() *cobra.Command {
	var jsonOut bool
	cmd := &cobra.Command{
		Use:   "list",
		Short: "Список маршрутов",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			m, err := loadMesh()
			if err != nil {
				return err
			}
			if jsonOut {
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(m.Routes)
			}
			w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 4, 2, ' ', 0)
			fmt.Fprintln(w, "ИМЯ\tПУТЬ\tEXIT")
			for _, r := range m.Routes {
				fmt.Fprintf(w, "%s\t%s\t%s\n", r.Name, strings.Join(r.Path, " → "), r.ExitNode)
			}
			if len(m.Routes) == 0 {
				fmt.Fprintln(w, "(нет маршрутов — добавьте: meshctl route add via-1 --path client,<node>)")
			}
			return w.Flush()
		},
	}
	cmd.Flags().BoolVar(&jsonOut, "json", false, "вывод в формате JSON")
	return cmd
}

func routeRemoveCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "remove <name>",
		Short: "Удалить маршрут",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			m, err := loadMesh()
			if err != nil {
				return err
			}
			name := args[0]
			if m.RouteByName(name) == nil {
				return fmt.Errorf("маршрут %q не найден", name)
			}
			var routes []config.Route
			for _, r := range m.Routes {
				if r.Name != name {
					routes = append(routes, r)
				}
			}
			m.Routes = routes
			if err := saveMesh(m); err != nil {
				return err
			}
			fmt.Printf("✔ Маршрут %q удалён\n", name)
			return nil
		},
	}
}

func routeEditCmd() *cobra.Command {
	var pathStr, exit string
	cmd := &cobra.Command{
		Use:   "edit <name>",
		Short: "Изменить существующий маршрут: --path client,nodeA,nodeB --exit nodeB",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			m, err := loadMesh()
			if err != nil {
				return err
			}
			name := args[0]
			r := m.RouteByName(name)
			if r == nil {
				return fmt.Errorf("маршрут %q не найден", name)
			}
			if pathStr != "" {
				path := splitTrim(pathStr)
				if len(path) == 0 || path[0] != config.ClientHop {
					return fmt.Errorf("--path должен начинаться с 'client', например: client,%s", strings.Join(nodeNames(m), ","))
				}
				r.Path = path
			}
			if exit != "" {
				r.ExitNode = exit
			} else if len(r.Path) > 1 {
				r.ExitNode = r.Path[len(r.Path)-1]
			}
			if err := validateMesh(m); err != nil {
				return err
			}
			if err := saveMesh(m); err != nil {
				return err
			}
			fmt.Printf("✔ Маршрут %q обновлён: %s → 🌐 (%s)\n", name, strings.Join(r.Path, " → "), r.ExitNode)
			return nil
		},
	}
	cmd.Flags().StringVar(&pathStr, "path", "", "цепочка через запятую, начиная с client")
	cmd.Flags().StringVar(&exit, "exit", "", "exit-нода (по умолчанию последняя в --path)")
	return cmd
}

func splitTrim(s string) []string {
	var out []string
	for _, p := range strings.Split(s, ",") {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

func nodeNames(m *config.Mesh) []string {
	var out []string
	for _, n := range m.Nodes {
		out = append(out, n.Name)
	}
	return out
}
