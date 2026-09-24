package mesh

import (
	"strings"
	"testing"

	"github.com/meshctl/meshctl/internal/config"
)

// validMesh — базовая корректная конфигурация из двух нод и multihop-маршрута.
func validMesh() *config.Mesh {
	return &config.Mesh{
		Name:    "Test",
		Version: 1,
		Nodes: []config.Node{
			{Name: "kz", Type: config.TypeLinux, Host: "1.1.1.1",
				WireGuard: config.WG{Interface: "wg0", ListenPort: 51820}},
			{Name: "de", Type: config.TypeLinux, Host: "2.2.2.2",
				WireGuard: config.WG{Interface: "wg0", ListenPort: 51821}},
		},
		Routes: []config.Route{
			{Name: "via-kz-de", Path: []string{"client", "kz", "de"}, ExitNode: "de"},
		},
	}
}

func mustContain(t *testing.T, err error, substr string) {
	t.Helper()
	if err == nil || !strings.Contains(err.Error(), substr) {
		t.Fatalf("ожидалась ошибка со %q, получено: %v", substr, err)
	}
}

func TestValidateOK(t *testing.T) {
	if err := Validate(validMesh()); err != nil {
		t.Fatalf("корректный конфиг не прошёл валидацию: %v", err)
	}
}

func TestValidateErrors(t *testing.T) {
	cases := []struct {
		name   string
		mutate func(*config.Mesh)
		want   string
	}{
		{"empty name", func(m *config.Mesh) { m.Name = "" }, "name"},
		{"bad cidr", func(m *config.Mesh) { m.CIDR = "not-a-cidr" }, "cidr"},
		{"dup node", func(m *config.Mesh) {
			m.Nodes = append(m.Nodes, m.Nodes[0])
		}, "дублируется имя ноды"},
		{"reserved client", func(m *config.Mesh) {
			m.Nodes = append(m.Nodes, config.Node{Name: "client", Type: config.TypeLinux, Host: "3.3.3.3",
				WireGuard: config.WG{Interface: "wg1", ListenPort: 51830}})
		}, "зарезервировано"},
		{"unknown type", func(m *config.Mesh) { m.Nodes[0].Type = "windows" }, "неизвестный тип"},
		{"no host", func(m *config.Mesh) { m.Nodes[0].Host = "" }, "host"},
		{"bad key", func(m *config.Mesh) { m.Nodes[0].WireGuard.PrivateKey = "zzz!" }, "private_key"},
		{"port conflict", func(m *config.Mesh) { m.Nodes[1].WireGuard.ListenPort = 51820 }, "конфликт interface/port"},
		{"dup route", func(m *config.Mesh) { m.Routes = append(m.Routes, m.Routes[0]) }, "дублируется имя маршрута"},
		{"path no client", func(m *config.Mesh) { m.Routes[0].Path = []string{"kz", "de"} }, "path"},
		{"unknown hop", func(m *config.Mesh) { m.Routes[0].Path = []string{"client", "mars"} }, "неизвестную ноду"},
		{"cycle", func(m *config.Mesh) { m.Routes[0].Path = []string{"client", "kz", "de", "kz"} }, "цикл"},
		{"exit mismatch", func(m *config.Mesh) { m.Routes[0].ExitNode = "kz" }, "должен быть последним хопом"},
		{"unknown exit", func(m *config.Mesh) { m.Routes[0].ExitNode = "mars" }, "неизвестная нода"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			m := validMesh()
			tc.mutate(m)
			mustContain(t, Validate(m), tc.want)
		})
	}
}

func TestEnsureKeysAndIPs(t *testing.T) {
	m := validMesh()
	mgr := NewManager(m)
	keys, err := mgr.EnsureKeys()
	if err != nil {
		t.Fatal(err)
	}
	if keys != 2 {
		t.Fatalf("ключей сгенерировано %d, ожидалось 2", keys)
	}
	ips, err := mgr.EnsureIPs()
	if err != nil {
		t.Fatal(err)
	}
	if ips != 2 || m.Nodes[0].MeshIP == m.Nodes[1].MeshIP {
		t.Fatalf("адреса розданы неверно: %v / %v", m.Nodes[0].MeshIP, m.Nodes[1].MeshIP)
	}
	// повторный вызов идемпотентен
	k2, _ := mgr.EnsureKeys()
	i2, _ := mgr.EnsureIPs()
	if k2 != 0 || i2 != 0 {
		t.Fatalf("повторный EnsureKeys/EnsureIPs должен быть холостым: %d/%d", k2, i2)
	}
}

func TestBuildPlan(t *testing.T) {
	m := validMesh()
	m.Routes = append(m.Routes,
		config.Route{Name: "via-kz", Path: []string{"client", "kz"}, ExitNode: "kz"})
	mgr := NewManager(m)
	plans := mgr.BuildPlan()
	if len(plans) != 2 {
		t.Fatalf("план должен включать 2 ноды, получил %d", len(plans))
	}
	var kz, de *NodePlan
	for i := range plans {
		switch plans[i].Node.Name {
		case "kz":
			kz = &plans[i]
		case "de":
			de = &plans[i]
		}
	}
	if kz == nil || de == nil {
		t.Fatal("в плане нет ожидаемых нод")
	}
	// kz — relay для via-kz-de и exit для via-kz; peer — de
	if !kz.IsRelay || !kz.IsExit {
		t.Fatalf("kz: exit=%v relay=%v, ожидал true/true", kz.IsExit, kz.IsRelay)
	}
	if len(kz.PeerNames) != 1 || kz.PeerNames[0] != "de" {
		t.Fatalf("kz peers: %v", kz.PeerNames)
	}
	if !de.IsExit || de.IsRelay {
		t.Fatalf("de: exit=%v relay=%v, ожидал true/false", de.IsExit, de.IsRelay)
	}
	// kz — первый хоп в обоих маршрутах
	if len(kz.ClientRoutes) != 2 {
		t.Fatalf("kz ClientRoutes: %v", kz.ClientRoutes)
	}
}

func TestClientAddressFor(t *testing.T) {
	mgr := NewManager(validMesh())
	if got := mgr.ClientAddressFor(0); got != "10.66.0.100" {
		t.Fatalf("ClientAddressFor(0) = %s", got)
	}
	if got := mgr.ClientAddressFor(3); got != "10.66.0.103" {
		t.Fatalf("ClientAddressFor(3) = %s", got)
	}
}
