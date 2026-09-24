package export

import (
	"encoding/json"
	"fmt"

	"github.com/meshctl/meshctl/internal/config"
	"github.com/meshctl/meshctl/internal/wg"
)

type AmneziaExporter struct{}

func init() {
	Register(&AmneziaExporter{})
}

func (e *AmneziaExporter) Name() string { return "amnezia" }

type amneziaProfile struct {
	Type          string   `json:"type"`
	Name          string   `json:"name"`
	HostName      string   `json:"hostName"`
	Port          int      `json:"port"`
	ClientIP      string   `json:"clientIp"`
	ClientPrivKey string   `json:"clientPrivKey"`
	ServerPubKey  string   `json:"serverPubKey"`
	AllowedIPs    []string `json:"allowedIps"`
	H1            uint32   `json:"h1,omitempty"`
	H2            uint32   `json:"h2,omitempty"`
	H3            uint32   `json:"h3,omitempty"`
	H4            uint32   `json:"h4,omitempty"`
	S1            uint16   `json:"s1,omitempty"`
	S2            uint16   `json:"s2,omitempty"`
	Jc            uint16   `json:"jc,omitempty"`
	Jmin          uint16   `json:"jmin,omitempty"`
	Jmax          uint16   `json:"jmax,omitempty"`
}

func (e *AmneziaExporter) Render(m *config.Mesh, route *config.Route, client *config.Client) ([]byte, error) {
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

	clientIP := "10.66.0.100"
	if client != nil && client.IP != "" {
		clientIP = client.IP
	}

	prof := amneziaProfile{
		Type:          "amnezia-wireguard",
		Name:          route.Name,
		HostName:      firstHop.Host,
		Port:          firstHop.WireGuard.ListenPort,
		ClientIP:      clientIP,
		ClientPrivKey: kp.PrivateKey,
		ServerPubKey:  firstHop.WireGuard.PublicKey,
		AllowedIPs:    []string{"0.0.0.0/0"},
	}

	if route.LinkObfuscation != nil {
		linkKey := fmt.Sprintf("client->%s", firstHopName)
		if presetName, ok := route.LinkObfuscation[linkKey]; ok {
			p, err := wg.PresetParams(presetName)
			if err == nil {
				prof.H1 = p.H1
				prof.H2 = p.H2
				prof.H3 = p.H3
				prof.H4 = p.H4
				prof.S1 = p.S1
				prof.S2 = p.S2
				prof.Jc = p.Jc
				prof.Jmin = p.Jmin
				prof.Jmax = p.Jmax
			}
		}
	}

	return json.MarshalIndent(prof, "", "  ")
}
