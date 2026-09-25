package export

import (
	"encoding/json"
	"fmt"

	"github.com/wgmesh/wgmesh/internal/config"
	"github.com/wgmesh/wgmesh/internal/wg"
)

type SingboxExporter struct{}

func init() {
	Register(&SingboxExporter{})
}

func (e *SingboxExporter) Name() string { return "sing-box" }

type singboxConfig struct {
	Log       map[string]string        `json:"log"`
	Inbounds  []map[string]interface{} `json:"inbounds"`
	Outbounds []map[string]interface{} `json:"outbounds"`
	Route     singboxRoute             `json:"route"`
}

type singboxRoute struct {
	Rules []singboxRule `json:"rules"`
}

type singboxRule struct {
	Domain   []string `json:"domain,omitempty"`
	Outbound string   `json:"outbound"`
}

func (e *SingboxExporter) Render(m *config.Mesh, route *config.Route, client *config.Client) ([]byte, error) {
	if len(route.Hops()) == 0 {
		return nil, fmt.Errorf("маршрут %q не содержит нод", route.Name)
	}

	firstHopName := route.Hops()[0]
	firstHop := m.NodeByName(firstHopName)
	if firstHop == nil {
		return nil, fmt.Errorf("нода %q не найдена", firstHopName)
	}

	kp, err := wg.GenerateKey()
	if err != nil {
		return nil, err
	}

	clientIP := "10.66.0.100/32"
	if client != nil && client.IP != "" {
		clientIP = client.IP + "/32"
	}

	outbound := map[string]interface{}{
		"type":        "wireguard",
		"tag":         route.Name,
		"server":      firstHop.Host,
		"server_port": firstHop.WireGuard.ListenPort,
		"local_address": []string{clientIP},
		"private_key": kp.PrivateKey,
		"peer_public_key": firstHop.WireGuard.PublicKey,
	}

	rules := []singboxRule{}
	if route.Match.List != "" {
		list := m.ListByName(route.Match.List)
		if list != nil && len(list.Domains) > 0 {
			rules = append(rules, singboxRule{
				Domain:   list.Domains,
				Outbound: route.Name,
			})
		}
	}

	cfg := singboxConfig{
		Log: map[string]string{"level": "info"},
		Inbounds: []map[string]interface{}{
			{"type": "tun", "inet4_address": "172.19.0.1/30", "auto_route": true},
		},
		Outbounds: []map[string]interface{}{
			outbound,
			{"type": "direct", "tag": "direct"},
		},
		Route: singboxRoute{Rules: rules},
	}

	return json.MarshalIndent(cfg, "", "  ")
}

