// Package mesh — core-логика: валидация конфигурации и менеджер применения.
package mesh

import (
	"fmt"
	"net"
	"sort"
	"strings"
	"time"

	"github.com/meshctl/meshctl/internal/config"
	"github.com/meshctl/meshctl/internal/wg"
)

// Validate выполняет полную валидацию mesh-конфига:
// имена нод/маршрутов, типы, ключи WG, конфликты портов и адресов,
// ссылки маршрутов на несуществующие ноды, циклы в path, exit_node.
func Validate(m *config.Mesh) error {
	var errs []string

	if strings.TrimSpace(m.Name) == "" {
		errs = append(errs, "mesh: поле 'name' обязательно")
	}
	if _, _, err := net.ParseCIDR(m.CIDROrDefault()); err != nil {
		errs = append(errs, fmt.Sprintf("mesh: некорректный cidr %q: %v", m.CIDROrDefault(), err))
	}

	names := map[string]bool{}
	ports := map[string][]string{} // interface+port -> node names
	for i := range m.Nodes {
		n := &m.Nodes[i]
		if n.Name == "" {
			errs = append(errs, fmt.Sprintf("node[%d]: пустое имя", i))
			continue
		}
		if n.Name == config.ClientHop {
			errs = append(errs, fmt.Sprintf("node %q: имя 'client' зарезервировано", n.Name))
		}
		if names[n.Name] {
			errs = append(errs, fmt.Sprintf("node %q: дублируется имя ноды", n.Name))
		}
		names[n.Name] = true

		switch n.Type {
		case config.TypeLinux, config.TypeMikrotik, config.TypeOpenWRT:
		default:
			errs = append(errs, fmt.Sprintf("node %q: неизвестный тип %q (linux|mikrotik|openwrt)", n.Name, n.Type))
		}
		if n.Host == "" {
			errs = append(errs, fmt.Sprintf("node %q: не задан host", n.Name))
		}
		if n.WireGuard.Interface == "" {
			errs = append(errs, fmt.Sprintf("node %q: не задано wireguard.interface", n.Name))
		}
		if n.WireGuard.ListenPort != 0 && (n.WireGuard.ListenPort < 1024 || n.WireGuard.ListenPort > 65535) {
			errs = append(errs, fmt.Sprintf("node %q: listen_port %d вне диапазона 1024-65535", n.Name, n.WireGuard.ListenPort))
		}
		if n.WireGuard.PrivateKey != "" && !wg.ValidKey(n.WireGuard.PrivateKey) {
			errs = append(errs, fmt.Sprintf("node %q: некорректный wireguard.private_key", n.Name))
		}
		if n.WireGuard.PublicKey != "" && !wg.ValidKey(n.WireGuard.PublicKey) {
			errs = append(errs, fmt.Sprintf("node %q: некорректный wireguard.public_key", n.Name))
		}
		if n.MeshIP != "" && net.ParseIP(n.MeshIP) == nil {
			errs = append(errs, fmt.Sprintf("node %q: некорректный mesh_ip %q", n.Name, n.MeshIP))
		}
		key := fmt.Sprintf("%s:%d", n.WireGuard.Interface, n.WireGuard.ListenPort)
		ports[key] = append(ports[key], n.Name)
	}
	for key, list := range ports {
		if len(list) > 1 {
			sort.Strings(list)
			errs = append(errs, fmt.Sprintf("конфликт interface/port %s: используется нодами %s", key, strings.Join(list, ", ")))
		}
	}

	routeNames := map[string]bool{}
	for i := range m.Routes {
		r := &m.Routes[i]
		if r.Name == "" {
			errs = append(errs, fmt.Sprintf("route[%d]: пустое имя", i))
			continue
		}
		if routeNames[r.Name] {
			errs = append(errs, fmt.Sprintf("route %q: дублируется имя маршрута", r.Name))
		}
		routeNames[r.Name] = true

		if len(r.Path) < 2 || r.Path[0] != config.ClientHop {
			errs = append(errs, fmt.Sprintf("route %q: path должен начинаться с 'client' и содержать хотя бы одну ноду", r.Name))
			continue
		}
		seen := map[string]bool{}
		for _, p := range r.Path {
			if p == config.ClientHop {
				continue
			}
			if !names[p] {
				errs = append(errs, fmt.Sprintf("route %q: путь ссылается на неизвестную ноду %q", r.Name, p))
			}
			if seen[p] {
				errs = append(errs, fmt.Sprintf("route %q: цикл в пути — нода %q встречается более одного раза", r.Name, p))
			}
			seen[p] = true
		}
		if r.ExitNode == "" {
			errs = append(errs, fmt.Sprintf("route %q: не задан exit_node", r.Name))
		} else if !names[r.ExitNode] {
			errs = append(errs, fmt.Sprintf("route %q: exit_node %q — неизвестная нода", r.Name, r.ExitNode))
		} else if r.Path[len(r.Path)-1] != r.ExitNode {
			errs = append(errs, fmt.Sprintf("route %q: exit_node %q должен быть последним хопом пути", r.Name, r.ExitNode))
		}
	}

	if len(errs) == 0 {
		return nil
	}
	return fmt.Errorf("валидация не пройдена:\n  - %s", strings.Join(errs, "\n  - "))
}

// CheckReachability пытается TCP-подключиться к SSH-порту каждой ноды
// (используется перед `apply`, чтобы сразу показать недоступные хосты).
// Возвращает карту имя_ноды -> ошибка (пустая, если все достижимы).
func CheckReachability(m *config.Mesh, timeout time.Duration) map[string]error {
	res := map[string]error{}
	for i := range m.Nodes {
		n := &m.Nodes[i]
		addr := net.JoinHostPort(n.Host, fmt.Sprint(n.SSHPortOrDefault()))
		conn, err := net.DialTimeout("tcp", addr, timeout)
		if err != nil {
			res[n.Name] = err
		} else {
			conn.Close()
		}
	}
	return res
}
