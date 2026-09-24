package cli

import (
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"

	"github.com/meshctl/meshctl/internal/config"
	"github.com/meshctl/meshctl/internal/drivers"
	"github.com/meshctl/meshctl/internal/mesh"
	"github.com/meshctl/meshctl/internal/wg"
)

// validateMesh — общая точка валидации конфига.
func validateMesh(m *config.Mesh) error { return mesh.Validate(m) }

// applyCmd — `meshctl apply`: генерирует ключи/адреса, строит конфигы WG
// и применяет их к нодам через драйверы. Флаг --dry-plan печатает план без SSH.
func applyCmd() *cobra.Command {
	var (
		dry      bool
		timeout  time.Duration
		checkNet bool
	)
	cmd := &cobra.Command{
		Use:   "apply",
		Short: "Применить конфигурацию ко всем участвующим нодам",
		Long: `1. Валидация mesh.yaml
2. Генерация недостающих WG-ключей и mesh_ip (сохраняются обратно в YAML)
3. Построение wg-quick конфигов для каждой ноды из маршрутов
4. Применение через драйвер платформы (пока linux; mikrotik/openwrt — Фаза 3)`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			m, err := loadMesh()
			if err != nil {
				return err
			}
			if err := validateMesh(m); err != nil {
				return err
			}
			fmt.Println("✔ Конфигурация валидна")

			mgr := mesh.NewManager(m)
			keys, err := mgr.EnsureKeys()
			if err != nil {
				return err
			}
			ips, err := mgr.EnsureIPs()
			if err != nil {
				return err
			}
			if keys > 0 || ips > 0 {
				fmt.Printf("✔ Сгенерировано ключей: %d, назначено mesh_ip: %d\n", keys, ips)
				if err := saveMesh(m); err != nil {
					return err
				}
			}

			if checkNet {
				unreach := mesh.CheckReachability(m, timeout)
				for name, e := range unreach {
					fmt.Fprintf(os.Stderr, "⚠ Нода %s недоступна по SSH: %v\n", name, e)
				}
				if len(unreach) == len(m.Nodes) && len(m.Nodes) > 0 {
					return fmt.Errorf("ни одна нода недоступна — проверьте сеть/файрвол")
				}
			}

			plans := mgr.BuildPlan()
			specs := make([]*drivers.NodeApplySpec, 0, len(plans))
			for _, p := range plans {
				spec := buildNodeSpec(mgr, m, p)
				specs = append(specs, spec)
				fmt.Printf("— нода %q (%s): пиры=%v exit=%v relay=%v\n",
					p.Node.Name, p.Node.Type, p.PeerNames, p.IsExit, p.IsRelay)
			}
			if dry {
				fmt.Println("✔ Dry-run завершён (изменения не применялись)")
				return nil
			}

			var failed []string
			for _, s := range specs {
				d, err := drivers.New(s.Node)
				if err != nil {
					failed = append(failed, s.Node.Name)
					fmt.Fprintf(os.Stderr, "✖ %s: %v\n", s.Node.Name, err)
					continue
				}
				fmt.Printf("→ Применяю %q через драйвер %s…\n", s.Node.Name, d.Name())
				if err := d.ApplySpec(s); err != nil {
					failed = append(failed, s.Node.Name)
					fmt.Fprintf(os.Stderr, "✖ %s: %v\n", s.Node.Name, err)
					continue
				}
				fmt.Printf("✔ %s настроена\n", s.Node.Name)
			}
			if len(failed) > 0 {
				return fmt.Errorf("не удалось применить на нодах: %v", failed)
			}
			fmt.Println("✔ Mesh применён. Клиентские конфиги: meshctl client-config <route>")
			return nil
		},
	}
	cmd.Flags().BoolVar(&dry, "dry-run", false, "только показать план, ничего не применять")
	cmd.Flags().BoolVar(&checkNet, "check", false, "проверить доступность нод по SSH перед применением")
	cmd.Flags().DurationVar(&timeout, "timeout", 5*time.Second, "таймаут проверки доступности")
	return cmd
}

// buildNodeSpec собирает wg.NodeConfig для одной ноды по плану.
func buildNodeSpec(mgr *mesh.Manager, m *config.Mesh, p mesh.NodePlan) *drivers.NodeApplySpec {
	node := p.Node
	nc := &wg.NodeConfig{
		Name:       node.Name,
		PrivateKey: node.WireGuard.PrivateKey,
		Address:    node.MeshIP + "/24",
		ListenPort: node.WireGuard.ListenPort,
	}
	// peers: соседние ноды цепочек.
	// Если сосед — exit-нода хотя бы одного маршрута, добавляем в его AllowedIPs
	// mesh-подсеть целиком: трафик клиентов (0.0.0.0/0 на клиенте и на relay-хопах
	// предыдущего звена) должен доставляться до exit-ноды через промежуточные хопы.
	for _, peerName := range p.PeerNames {
		peer := m.NodeByName(peerName)
		if peer == nil {
			continue
		}
		allowed := []string{peer.MeshIP + "/32"}
		if isExitNode(m, peer.Name) {
			allowed = append(allowed, m.CIDROrDefault())
		}
		nc.Peers = append(nc.Peers, wg.PeerConf{
			PublicKey:  peer.WireGuard.PublicKey,
			AllowedIPs: allowed,
			Endpoint:   fmt.Sprintf("%s:%d", peer.Host, peer.WireGuard.ListenPort),
		})
	}
	// peers: клиенты маршрутов, где эта нода — первый хоп.
	// Реальный публичный ключ клиента подставляется из секции clients (mesh.yaml),
	// если он уже сгенерирован (`client-config`); иначе peer будет добавлен
	// автоматически при следующем apply после генерации конфига клиента.
	for _, routeName := range p.ClientRoutes {
		r := m.RouteByName(routeName)
		if r == nil {
			continue
		}
		idx := routeIndex(m, routeName)
		clientIP := mgr.ClientAddressFor(idx)
		allowed := []string{clientIP + "/32"}
		if r.ExitNode == node.Name {
			allowed = append(allowed, "0.0.0.0/0")
		}
		pubKey, _ := clientPublicKey(cfgPath, routeName) // луч-effort: не блокируем apply
		nc.Peers = append(nc.Peers, wg.PeerConf{
			PublicKey:           pubKey,
			AllowedIPs:          allowed,
			PersistentKeepalive: 25,
		})
	}
	return &drivers.NodeApplySpec{
		Node:    node,
		Config:  nc,
		NAT:     p.IsExit,
		Forward: p.IsRelay || p.IsExit,
	}
}

func routeIndex(m *config.Mesh, name string) int {
	for i := range m.Routes {
		if m.Routes[i].Name == name {
			return i
		}
	}
	return 0
}

// isExitNode — является ли нода exit_node хотя бы для одного маршрута.
func isExitNode(m *config.Mesh, name string) bool {
	for _, r := range m.Routes {
		if r.ExitNode == name {
			return true
		}
	}
	return false
}
