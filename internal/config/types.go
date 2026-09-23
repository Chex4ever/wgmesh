// Package config описывает YAML-структуры конфигурации mesh-сети:
// ноды, маршруты, WireGuard-параметры.
package config

// DefaultConfigFile — имя главного конфигурационного файла по умолчанию.
const DefaultConfigFile = "mesh.yaml"

// Mesh — главный конфигурационный файл (mesh.yaml).
type Mesh struct {
	Name    string  `yaml:"name"`
	Version int     `yaml:"version"`
	CIDR    string  `yaml:"cidr,omitempty"` // mesh-подсеть, по умолчанию 10.66.0.0/24
	Nodes   []Node  `yaml:"nodes"`
	Routes  []Route `yaml:"routes"`
}

// Node — участник mesh-сети (сервер/роутер).
type Node struct {
	Name      string `yaml:"name"`
	Type      string `yaml:"type"` // linux | mikrotik | openwrt
	Host      string `yaml:"host"`
	SSHUser   string `yaml:"ssh_user,omitempty"`
	SSHKey    string `yaml:"ssh_key,omitempty"`
	SSHPort   int    `yaml:"ssh_port,omitempty"`    // по умолчанию 22
	SSHPass   string `yaml:"ssh_password,omitempty"` // не рекомендуется: лучше ssh_key
	WireGuard WG     `yaml:"wireguard"`
	MeshIP    string `yaml:"mesh_ip,omitempty"` // внутренний адрес в mesh-подсети (назначается автоматически)
}

// WG — параметры WireGuard-интерфейса ноды.
type WG struct {
	Interface         string `yaml:"interface"`
	ListenPort        int    `yaml:"listen_port"`
	PrivateKey        string `yaml:"private_key,omitempty"` // генерируется автоматически
	PublicKey         string `yaml:"public_key,omitempty"`  // генерируется автоматически
	Address           string `yaml:"address,omitempty"`     // префикс адреса внутри интерфейса
	PersistentKeepalive int  `yaml:"persistent_keepalive,omitempty"`
}

// Route — маршрут (цепочка хопов) от клиента до exit-ноды.
// path[0] всегда "client", далее — имена нод по порядку.
type Route struct {
	Name     string   `yaml:"name"`
	Path     []string `yaml:"path"`
	ExitNode string   `yaml:"exit_node"`
}

// NodeType — допустимые типы нод.
const (
	TypeLinux    = "linux"
	TypeMikrotik = "mikrotik"
	TypeOpenWRT  = "openwrt"
)

// ClientHop — зарезервированное имя первого хопа в path.
const ClientHop = "client"

// SSHPortOrDefault возвращает порт SSH с дефолтом 22.
func (n *Node) SSHPortOrDefault() int {
	if n.SSHPort > 0 {
		return n.SSHPort
	}
	return 22
}

// NodeByName ищет ноду по имени (nil, если не найдена).
func (m *Mesh) NodeByName(name string) *Node {
	for i := range m.Nodes {
		if m.Nodes[i].Name == name {
			return &m.Nodes[i]
		}
	}
	return nil
}

// RouteByName ищет маршрут по имени (nil, если не найден).
func (m *Mesh) RouteByName(name string) *Route {
	for i := range m.Routes {
		if m.Routes[i].Name == name {
			return &m.Routes[i]
		}
	}
	return nil
}

// Hops возвращает список нод маршрута без "client".
func (r *Route) Hops() []string {
	var hops []string
	for _, p := range r.Path {
		if p != ClientHop {
			hops = append(hops, p)
		}
	}
	return hops
}

// CIDROrDefault возвращает mesh-подсеть с дефолтом.
func (m *Mesh) CIDROrDefault() string {
	if m.CIDR != "" {
		return m.CIDR
	}
	return DefaultCIDR
}

// DefaultCIDR — подсеть по умолчанию для внутренних адресов mesh.
const DefaultCIDR = "10.66.0.0/24"
