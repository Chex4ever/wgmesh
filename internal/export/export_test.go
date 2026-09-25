package export

import (
	"strings"
	"testing"

	"github.com/wgmesh/wgmesh/internal/config"
)

func sampleMesh() *config.Mesh {
	return &config.Mesh{
		Name:    "ExportTest",
		Version: 2,
		Nodes: []config.Node{
			{Name: "kz", Type: "linux", Host: "1.1.1.1", WireGuard: config.WG{Interface: "wg0", ListenPort: 51820, PublicKey: "pubkeykz"}},
			{Name: "de", Type: "linux", Host: "2.2.2.2", WireGuard: config.WG{Interface: "wg0", ListenPort: 51821, PublicKey: "pubkeyde"}},
		},
		Lists: []config.DomainList{
			{Name: "youtube", Domains: []string{"youtube.com"}},
		},
		Routes: []config.Route{
			{Name: "via-de", Path: []string{"client", "kz", "de"}, ExitNode: "de", Match: config.TrafficMatch{List: "youtube"}},
		},
	}
}

func TestExporters(t *testing.T) {
	m := sampleMesh()
	route := &m.Routes[0]

	formats := []string{"wireguard", "amnezia", "sing-box", "uri"}
	for _, fmtName := range formats {
		t.Run(fmtName, func(t *testing.T) {
			exp, err := Get(fmtName)
			if err != nil {
				t.Fatalf("Get(%s) failed: %v", fmtName, err)
			}
			out, err := exp.Render(m, route, nil)
			if err != nil {
				t.Fatalf("Render(%s) failed: %v", fmtName, err)
			}
			if len(out) == 0 {
				t.Fatalf("Render(%s) returned empty byte slice", fmtName)
			}

			if fmtName == "uri" && !strings.HasPrefix(string(out), "wireguard://") {
				t.Fatalf("uri export should start with wireguard://, got %s", string(out))
			}
		})
	}
}

