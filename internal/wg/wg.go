// Package wg — генерация ключей WireGuard и рендеринг клиентских/серверных конфигов.
// Реализация на curve25519 из golang.org/x/crypto — не требует бинарника `wg keygen`.
package wg

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"net"
	"strings"

	"golang.org/x/crypto/curve25519"
)

// KeyPair — пара ключей WireGuard в base64-кодировке.
type KeyPair struct {
	PrivateKey string
	PublicKey  string
}

// GenerateKey создаёт новый случайный ключ WireGuard.
func GenerateKey() (*KeyPair, error) {
	var priv [32]byte
	if _, err := rand.Read(priv[:]); err != nil {
		return nil, fmt.Errorf("wg: не удалось получить энтропию: %w", err)
	}
	// клампим приватный ключ согласно RFC 7748 (требование WireGuard)
	priv[0] &= 248
	priv[31] &= 127
	priv[31] |= 64

	pub, err := curve25519.X25519(priv[:], curve25519.Basepoint)
	if err != nil {
		return nil, fmt.Errorf("wg: ошибка вычисления публичного ключа: %w", err)
	}
	return &KeyPair{
		PrivateKey: base64.StdEncoding.EncodeToString(priv[:]),
		PublicKey:  base64.StdEncoding.EncodeToString(pub),
	}, nil
}

// PublicKey вычисляет публичный ключ из приватного (base64).
func PublicKey(privateB64 string) (string, error) {
	priv, err := base64.StdEncoding.DecodeString(privateB64)
	if err != nil || len(priv) != 32 {
		return "", fmt.Errorf("wg: некорректный приватный ключ")
	}
	priv = append([]byte{}, priv...)
	priv[0] &= 248
	priv[31] &= 127
	priv[31] |= 64
	pub, err := curve25519.X25519(priv, curve25519.Basepoint)
	if err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(pub), nil
}

// ValidKey проверяет формат ключа WireGuard (base64, 32 байта).
func ValidKey(k string) bool {
	b, err := base64.StdEncoding.DecodeString(k)
	return err == nil && len(b) == 32
}

// PeerConf — параметры peer-секции в конфиге.
type PeerConf struct {
	PublicKey           string
	AllowedIPs          []string
	Endpoint            string // host:port (для исходящих пиеров; пусто на серверах)
	PersistentKeepalive int
}

// EndpointHost возвращает имя хоста/IP из Endpoint (без порта).
func (p *PeerConf) EndpointHost() string {
	if p.Endpoint == "" {
		return ""
	}
	host, _, err := net.SplitHostPort(p.Endpoint)
	if err != nil {
		return p.Endpoint
	}
	return host
}

// EndpointPort возвращает порт из Endpoint (или 51820 по умолчанию).
func (p *PeerConf) EndpointPort() int {
	if p.Endpoint == "" {
		return 51820
	}
	_, portStr, err := net.SplitHostPort(p.Endpoint)
	if err != nil {
		return 51820
	}
	var port int
	fmt.Sscanf(portStr, "%d", &port)
	if port == 0 {
		return 51820
	}
	return port
}

// ClientConfig — данные для рендеринга клиентского .conf.
type ClientConfig struct {
	PrivateKey string
	Address    string   // IP/префикс клиента
	DNS        []string
	Peer       PeerConf
	Name       string
}

// RenderClient generates a standard wg-quick client config.
func RenderClient(c *ClientConfig) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# meshctl: клиентский конфиг маршрута %s\n", c.Name)
	b.WriteString("[Interface]\n")
	fmt.Fprintf(&b, "PrivateKey = %s\n", c.PrivateKey)
	fmt.Fprintf(&b, "Address = %s\n", c.Address)
	if len(c.DNS) > 0 {
		fmt.Fprintf(&b, "DNS = %s\n", strings.Join(c.DNS, ", "))
	}
	b.WriteString("\n[Peer]\n")
	fmt.Fprintf(&b, "PublicKey = %s\n", c.Peer.PublicKey)
	fmt.Fprintf(&b, "AllowedIPs = %s\n", strings.Join(c.Peer.AllowedIPs, ", "))
	if c.Peer.Endpoint != "" {
		fmt.Fprintf(&b, "Endpoint = %s\n", c.Peer.Endpoint)
	}
	if c.Peer.PersistentKeepalive > 0 {
		fmt.Fprintf(&b, "PersistentKeepalive = %d\n", c.Peer.PersistentKeepalive)
	}
	return b.String()
}

// NodeConfig — данные для рендеринга серверного wg-quick конфига ноды.
type NodeConfig struct {
	PrivateKey string
	Address    string // IP/prefix ноды внутри mesh
	ListenPort int
	Peers      []PeerConf
	Name       string
}

// RenderNode генерирует wg-quick конфиг для ноды (server-side).
func RenderNode(n *NodeConfig) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# meshctl: конфиг ноды %s (сгенерирован автоматически)\n", n.Name)
	b.WriteString("[Interface]\n")
	fmt.Fprintf(&b, "PrivateKey = %s\n", n.PrivateKey)
	fmt.Fprintf(&b, "Address = %s\n", n.Address)
	fmt.Fprintf(&b, "ListenPort = %d\n", n.ListenPort)
	for _, p := range n.Peers {
		b.WriteString("\n[Peer]\n")
		fmt.Fprintf(&b, "PublicKey = %s\n", p.PublicKey)
		fmt.Fprintf(&b, "AllowedIPs = %s\n", strings.Join(p.AllowedIPs, ", "))
		if p.Endpoint != "" {
			fmt.Fprintf(&b, "Endpoint = %s\n", p.Endpoint)
		}
		if p.PersistentKeepalive > 0 {
			fmt.Fprintf(&b, "PersistentKeepalive = %d\n", p.PersistentKeepalive)
		}
	}
	return b.String()
}
