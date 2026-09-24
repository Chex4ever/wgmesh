package cli

import (
	"encoding/json"
	"fmt"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/meshctl/meshctl/internal/config"
	"github.com/meshctl/meshctl/internal/drivers"
)

// nodeCmd — `meshctl node ...`: add / list / remove / teardown / capabilities.
func nodeCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "node",
		Short: "Управление нодами mesh-сети",
	}
	cmd.AddCommand(nodeAddCmd(), nodeListCmd(), nodeRemoveCmd(), nodeTeardownCmd(), nodeCapsCmd())
	return cmd
}

func nodeAddCmd() *cobra.Command {
	var (
		nType, host, user, key, iface string
		port, wgPort                  int
	)
	cmd := &cobra.Command{
		Use:   "add <name>",
		Short: "Добавить ноду",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			m, err := loadMesh()
			if err != nil {
				return err
			}
			name := args[0]
			if m.NodeByName(name) != nil {
				return fmt.Errorf("нода %q уже существует", name)
			}
			node := config.Node{
				Name:    name,
				Type:    nType,
				Host:    host,
				SSHUser: user,
				SSHKey:  key,
				SSHPort: port,
				WireGuard: config.WG{
					Interface:  iface,
					ListenPort: wgPort,
				},
			}
			m.Nodes = append(m.Nodes, node)
			if node.Host == "" {
				return fmt.Errorf("нода %q: не задан --host", name)
			}
			if err := saveMesh(m); err != nil {
				return err
			}
			fmt.Printf("✔ Нода %q (%s, %s) добавлена в %s\n", name, nType, host, cfgPath)
			return nil
		},
	}
	cmd.Flags().StringVar(&nType, "type", config.TypeLinux, "тип ноды: linux|mikrotik|openwrt")
	cmd.Flags().StringVar(&host, "host", "", "публичный IP/домен ноды (обязательно)")
	cmd.Flags().StringVar(&user, "user", "root", "SSH-пользователь")
	cmd.Flags().StringVar(&key, "key", "", "путь к SSH-ключу (по умолчанию ~/.ssh/id_ed25519 или id_rsa на ноде не проверяется)")
	cmd.Flags().IntVar(&port, "ssh-port", 22, "порт SSH")
	cmd.Flags().StringVar(&iface, "wg-interface", "wg0", "имя WireGuard-интерфейса")
	cmd.Flags().IntVar(&wgPort, "wg-port", 51820, "порт прослушивания WireGuard")
	return cmd
}

func nodeListCmd() *cobra.Command {
	var jsonOut bool
	cmd := &cobra.Command{
		Use:   "list",
		Short: "Список нод",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			m, err := loadMesh()
			if err != nil {
				return err
			}
			if jsonOut {
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(m.Nodes)
			}
			w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 4, 2, ' ', 0)
			fmt.Fprintln(w, "ИМЯ\tТИП\tHOST\tWG IFACE\tPORT\tMESH IP\tКЛЮЧ")
			for _, n := range m.Nodes {
				keyState := "—"
				if n.SSHKey != "" {
					keyState = "key"
				} else if n.SSHPass != "" {
					keyState = "password"
				}
				pub := "нет"
				if n.WireGuard.PublicKey != "" {
					pub = "есть"
				}
				fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%d\t%s\t%s/%s\n",
					n.Name, n.Type, n.Host, n.WireGuard.Interface, n.WireGuard.ListenPort,
					orDash(n.MeshIP), keyState, pub)
			}
			if len(m.Nodes) == 0 {
				fmt.Fprintln(w, "(нет нод — добавьте через: meshctl node add <name> --host <ip>)")
			}
			return w.Flush()
		},
	}
	cmd.Flags().BoolVar(&jsonOut, "json", false, "вывод в формате JSON")
	return cmd
}

func nodeRemoveCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "remove <name>",
		Short: "Удалить ноду (и ссылки из маршрутов)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			m, err := loadMesh()
			if err != nil {
				return err
			}
			name := args[0]
			if m.NodeByName(name) == nil {
				return fmt.Errorf("нода %q не найдена", name)
			}
			// запрещаем удаление, если нода участвует в маршрутах, без явного согласия
			var affected []string
			for _, r := range m.Routes {
				for _, p := range r.Path {
					if p == name {
						affected = append(affected, r.Name)
						break
					}
				}
			}
			if len(affected) > 0 {
				return fmt.Errorf("нода %q участвует в маршрутах %v — сначала удалите/поправьте их (meshctl route remove <name>)", name, affected)
			}
			var nodes []config.Node
			for _, n := range m.Nodes {
				if n.Name != name {
					nodes = append(nodes, n)
				}
			}
			m.Nodes = nodes
			if err := saveMesh(m); err != nil {
				return err
			}
			fmt.Printf("✔ Нода %q удалена\n", name)
			return nil
		},
	}
}

func nodeTeardownCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "teardown <name>",
		Short: "Удалить WG конфигурацию с ноды по SSH",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			m, err := loadMesh()
			if err != nil {
				return err
			}
			name := args[0]
			node := m.NodeByName(name)
			if node == nil {
				return fmt.Errorf("нода %q не найдена", name)
			}
			d, err := drivers.New(node)
			if err != nil {
				return err
			}
			fmt.Printf("→ Удаляю WG конфигурацию с %q (%s)…\n", node.Name, d.Name())
			if err := d.RemoveConfig(node); err != nil {
				return fmt.Errorf("не удалось удалить конфигурацию: %w", err)
			}
			fmt.Printf("✔ Конфигурация с ноды %q успешно удалена\n", node.Name)
			return nil
		},
	}
}

func nodeCapsCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "capabilities <name>",
		Short: "Показать технологические возможности платформы ноды",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			m, err := loadMesh()
			if err != nil {
				return err
			}
			name := args[0]
			node := m.NodeByName(name)
			if node == nil {
				return fmt.Errorf("нода %q не найдена", name)
			}
			caps := drivers.Capabilities(node.Type)
			fmt.Printf("Возможности платформы %q (%s):\n", node.Name, node.Type)
			fmt.Printf("  - PSK Support:         %v\n", caps.SupportsPSK)
			fmt.Printf("  - Keepalive Support:   %v\n", caps.SupportsKeepalive)
			fmt.Printf("  - NAT/Masquerade:      %v\n", caps.SupportsNAT)
			fmt.Printf("  - Multi-Route:         %v\n", caps.SupportsMultiRoute)
			fmt.Printf("  - AmneziaWG Obfusc:    %v\n", caps.SupportsObfuscation)
			fmt.Printf("  - Auto Package Inst:   %v\n", caps.NeedsPackageInstall)
			return nil
		},
	}
}

func orDash(s string) string {
	if s == "" {
		return "—"
	}
	return s
}
