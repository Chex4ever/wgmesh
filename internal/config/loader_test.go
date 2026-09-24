package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSaveLoadRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "mesh.yaml")

	m := DefaultMesh()
	m.Name = "Test Mesh"
	m.Nodes = []Node{
		{
			Name: "kz-server", Type: TypeLinux, Host: "109.248.198.55",
			SSHUser: "root", MeshIP: "10.66.0.10",
			WireGuard: WG{Interface: "wg0", ListenPort: 51820},
		},
	}
	m.Routes = []Route{{Name: "via-kz", Path: []string{"client", "kz-server"}, ExitNode: "kz-server"}}

	if err := Save(path, m); err != nil {
		t.Fatalf("Save: %v", err)
	}
	got, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got.Name != m.Name || len(got.Nodes) != 1 || len(got.Routes) != 1 {
		t.Fatalf("round-trip потерял данные: %+v", got)
	}
	n := got.Nodes[0]
	if n.Name != "kz-server" || n.Host != "109.248.198.55" || n.WireGuard.ListenPort != 51820 {
		t.Fatalf("поля ноды не совпали: %+v", n)
	}
	if r := got.Routes[0]; r.ExitNode != "kz-server" || len(r.Hops()) != 1 {
		t.Fatalf("маршрут не сохранился: %+v", r)
	}
}

func TestLoadMissingFile(t *testing.T) {
	if _, err := Load(filepath.Join(t.TempDir(), "nope.yaml")); err == nil {
		t.Fatal("ожидалась ошибка для несуществующего файла")
	}
}

func TestExists(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "x.yaml")
	if Exists(p) {
		t.Fatal("Exists вернул true для несуществующего файла")
	}
	if err := os.WriteFile(p, []byte("name: x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if !Exists(p) {
		t.Fatal("Exists вернул false для существующего файла")
	}
}

func TestHelpers(t *testing.T) {
	m := &Mesh{
		Nodes:  []Node{{Name: "a"}, {Name: "b"}},
		Routes: []Route{{Name: "r1", Path: []string{"client", "a", "b"}, ExitNode: "b"}},
	}
	if m.NodeByName("b") == nil || m.NodeByName("zz") != nil {
		t.Fatal("NodeByName работает неверно")
	}
	if m.RouteByName("r1") == nil || m.RouteByName("qq") != nil {
		t.Fatal("RouteByName работает неверно")
	}
	if hops := m.Routes[0].Hops(); len(hops) != 2 || hops[0] != "a" || hops[1] != "b" {
		t.Fatalf("Hops: %v", hops)
	}
	if m.CIDROrDefault() != DefaultCIDR {
		t.Fatalf("CIDROrDefault: %v", m.CIDROrDefault())
	}
	n := &Node{}
	if n.SSHPortOrDefault() != 22 {
		t.Fatal("SSHPortOrDefault должен давать 22")
	}
}
