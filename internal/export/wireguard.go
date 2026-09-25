package export

import (
	"fmt"
	"strings"

	"github.com/wgmesh/wgmesh/internal/config"
	"github.com/wgmesh/wgmesh/internal/wg"
)

type WireGuardExporter struct{}

func init() {
	Register(&WireGuardExporter{})
}

func (e *WireGuardExporter) Name() string { return "wireguard" }

func (e *WireGuardExporter) Render(m *config.Mesh, route *config.Route, client *config.Client) ([]byte, error) {
	if len(route.Hops()) == 0 {
		return nil, fmt.Errorf("маршрут %q не содержит нод", route.Name)
	}

	firstHopName := route.Hops()[0]
	firstHop := m.NodeByName(firstHopName)
	if firstHop == nil {
		return nil, fmt.Errorf("нода %q не найдена", firstHopName)
	}

	clientIP := "10.66.0.100/32"
	if client != nil && client.IP != "" {
		clientIP = client.IP + "/32"
	}

	// Генерируем тестовую пару для клиенто-конфига если еще нет
	kp, err := wg.GenerateKey()
	if err != nil {
		return nil, err
	}

	c := &wg.ClientConfig{
		PrivateKey: kp.PrivateKey,
		Address:    clientIP,
		DNS:        []string{"1.1.1.1", "8.8.8.8"},
		Name:       route.Name,
		Peer: wg.PeerConf{
			PublicKey:           firstHop.WireGuard.PublicKey,
			AllowedIPs:          []string{"0.0.0.0/0"},
			Endpoint:            fmt.Sprintf("%s:%d", firstHop.Host, firstHop.WireGuard.ListenPort),
			PersistentKeepalive: 25,
		},
	}

	out := wg.RenderClient(c)

	// Добавляем AmneziaWG параметры если задана обфускация на первом плече
	if route.LinkObfuscation != nil {
		linkKey := fmt.Sprintf("client->%s", firstHopName)
		if presetName, ok := route.LinkObfuscation[linkKey]; ok {
			p, err := wg.PresetParams(presetName)
			if err == nil {
				out += "\n" + p.RenderAmneziaBlock()
			}
		}
	}

	return []byte(strings.TrimSpace(out) + "\n"), nil
}

