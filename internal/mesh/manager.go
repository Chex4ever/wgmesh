package mesh

import (
	"fmt"
	"net"

	"github.com/meshctl/meshctl/internal/config"
	"github.com/meshctl/meshctl/internal/wg"
)

// Manager — core-логика применения конфигурации к нодам.
type Manager struct {
	Mesh *config.Mesh
}

// NewManager создаёт менеджер для загруженного конфига.
func NewManager(m *config.Mesh) *Manager {
	return &Manager{Mesh: m}
}

// EnsureKeys генерирует WireGuard-ключи для всех нод, у которых они ещё не заданы.
// Возвращает количество сгенерированных пар. Конфиг вызывающий обязан сохранить.
func (mgr *Manager) EnsureKeys() (int, error) {
	n := 0
	for i := range mgr.Mesh.Nodes {
		node := &mgr.Mesh.Nodes[i]
		if node.WireGuard.PrivateKey != "" && node.WireGuard.PublicKey != "" {
			continue
		}
		kp, err := wg.GenerateKey()
		if err != nil {
			return n, fmt.Errorf("нода %q: %w", node.Name, err)
		}
		node.WireGuard.PrivateKey = kp.PrivateKey
		node.WireGuard.PublicKey = kp.PublicKey
		n++
	}
	return n, nil
}

// EnsureIPs раздаёт каждой ноде mesh_ip из CIDR-подсети (детерминированно: .10 + индекс).
// Клиентам маршрутов адреса выдаются отдельно (см. ClientAddressFor).
// Возвращает количество назначенных адресов.
func (mgr *Manager) EnsureIPs() (int, error) {
	_, ipnet, err := net.ParseCIDR(mgr.Mesh.CIDROrDefault())
	if err != nil {
		return 0, err
	}
	base := ipnet.IP.To4()
	if base == nil {
		return 0, fmt.Errorf("meshctl: пока поддерживаются только IPv4 CIDR")
	}
	used := map[string]bool{}
	for _, node := range mgr.Mesh.Nodes {
		if node.MeshIP != "" {
			used[node.MeshIP] = true
		}
	}
	assigned := 0
	next := byte(10) // .10, .11, ... — чтобы не коллизировать с ручными назначениями
	for i := range mgr.Mesh.Nodes {
		node := &mgr.Mesh.Nodes[i]
		if node.MeshIP != "" {
			continue
		}
		for used[fmt.Sprintf("%d.%d.%d.%d", base[0], base[1], base[2], next)] {
			next++
			if next == 0 {
				return assigned, fmt.Errorf("meshctl: в подсети закончились свободные адреса")
			}
		}
		node.MeshIP = fmt.Sprintf("%d.%d.%d.%d", base[0], base[1], base[2], next)
		used[node.MeshIP] = true
		next++
		assigned++
	}
	return assigned, nil
}

// ClientAddressFor возвращает mesh-адрес клиента для конкретного маршрута.
// Клиенты занимают диапазон .100+.
func (mgr *Manager) ClientAddressFor(routeIdx int) string {
	base, _ := parseIPv4Prefix(mgr.Mesh.CIDROrDefault())
	return fmt.Sprintf("%d.%d.%d.%d", base[0], base[1], base[2], byte(100+routeIdx))
}

// Plan describes what apply should do for each node involved in routes.
type NodePlan struct {
	Node        *config.Node
	PeerNames   []string // имена пиров (ноды/клиенты маршрутов), которые должны быть настроены на этой ноде
	IsExit      bool     // является ли exit-нодой хотя бы для одного маршрута
	IsRelay     bool     // промежуточный хоп (нужен ip_forward без NAT)
	ClientRoutes []string // маршруты, где эта нода — первый хоп после client (настраивает peer для клиента)
}

// BuildPlan строит план применения: какие ноды участвуют, кто relay, кто exit.
func (mgr *Manager) BuildPlan() []NodePlan {
	byName := map[string]*NodePlan{}
	order := []string{}
	get := func(name string) *NodePlan {
		if p, ok := byName[name]; ok {
			return p
		}
		p := &NodePlan{Node: mgr.Mesh.NodeByName(name)}
		byName[name] = p
		order = append(order, name)
		return p
	}

	for _, r := range mgr.Mesh.Routes {
		hops := r.Hops()
		for idx, h := range hops {
			p := get(h)
			switch {
			case h == r.ExitNode:
				p.IsExit = true
			default:
				p.IsRelay = true
			}
			// соседство в цепочке: двусторонние пиры
			if idx > 0 {
				prev := hops[idx-1]
				p.PeerNames = appendUnique(p.PeerNames, prev)
				get(prev).PeerNames = appendUnique(get(prev).PeerNames, h)
			}
		}
		if len(hops) > 0 {
			first := get(hops[0])
			first.ClientRoutes = appendUnique(first.ClientRoutes, r.Name)
		}
	}

	plans := make([]NodePlan, 0, len(order))
	for _, name := range order {
		plans = append(plans, *byName[name])
	}
	return plans
}

// AllowedIPsForHop возвращает набор AllowedIPs для peer'а, соответствующего ноде node,
// на стороне её соседа: адрес самой ноды + mesh-подсеть, если нода ретранслирует дальше,
// плюс 0.0.0.0/0 для exit-нод со стороны клиента.
func AllowedIPsForHop(node *config.Node, asExit bool) []string {
	ips := []string{node.MeshIP + "/32"}
	if asExit {
		ips = append(ips, "0.0.0.0/0")
	}
	return ips
}

func appendUnique(list []string, v string) []string {
	for _, x := range list {
		if x == v {
			return list
		}
	}
	return append(list, v)
}

func parseIPv4Prefix(cidr string) (net.IP, error) {
	ip, _, err := net.ParseCIDR(cidr)
	if err != nil {
		return nil, err
	}
	return ip.To4(), nil
}
