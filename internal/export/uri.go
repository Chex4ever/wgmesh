package export

import (
	"fmt"
	"net/url"

	"github.com/wgmesh/wgmesh/internal/config"
	"github.com/wgmesh/wgmesh/internal/wg"
)

type URIExporter struct{}

func init() {
	Register(&URIExporter{})
}

func (e *URIExporter) Name() string { return "uri" }

func (e *URIExporter) Render(m *config.Mesh, route *config.Route, client *config.Client) ([]byte, error) {
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

	// Формат wireguard://privateKey@host:port?publickey=...#name
	uriStr := fmt.Sprintf("wireguard://%s@%s:%d?publickey=%s&address=%s#%s",
		url.QueryEscape(kp.PrivateKey),
		firstHop.Host,
		firstHop.WireGuard.ListenPort,
		url.QueryEscape(firstHop.WireGuard.PublicKey),
		url.QueryEscape(clientIP),
		url.QueryEscape(route.Name),
	)

	return []byte(uriStr + "\n"), nil
}

