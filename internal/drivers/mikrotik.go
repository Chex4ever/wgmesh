package drivers

import (
	"fmt"
	"strings"

	"github.com/wgmesh/wgmesh/internal/config"
)

// MikrotikDriver реализует провижининг роутеров Mikrotik (RouterOS 7) по SSH.
type MikrotikDriver struct {
	node *config.Node
}

// NewMikrotik создаёт драйвер Mikrotik.
func NewMikrotik(node *config.Node) *MikrotikDriver {
	return &MikrotikDriver{node: node}
}

// Name — mikrotik.
func (d *MikrotikDriver) Name() string { return "mikrotik" }

// ApplySpec настраивает WireGuard, пиры, адрес и NAT/маршруты на RouterOS.
func (d *MikrotikDriver) ApplySpec(spec *NodeApplySpec) error {
	r, err := dialSSH(d.node)
	if err != nil {
		return fmt.Errorf("drivers/mikrotik: ошибка подключения SSH: %w", err)
	}
	defer r.client.Close()

	iface := spec.Node.WireGuard.Interface
	if iface == "" {
		iface = "wg0"
	}
	port := spec.Node.WireGuard.ListenPort
	if port == 0 {
		port = 51820
	}

	// 1. Создание/обновление WireGuard интерфейса
	cmdIface := fmt.Sprintf(
		":if ([:len [/interface wireguard find name=%s]] = 0) do={ /interface wireguard add name=%s listen-port=%d private-key=%q comment=\"wgmesh:%s\" } else={ /interface wireguard set [find name=%s] listen-port=%d private-key=%q }",
		iface, iface, port, spec.Config.PrivateKey, d.node.Name, iface, port, spec.Config.PrivateKey,
	)
	if out, err := r.run(cmdIface); err != nil {
		return fmt.Errorf("drivers/mikrotik: не удалось настроить интерфейс %s: %w\n%s", iface, err, out)
	}

	// 2. Добавление/обновление пиров
	for _, p := range spec.Config.Peers {
		allowedStr := strings.Join(p.AllowedIPs, ",")
		cmdPeer := fmt.Sprintf(
			":if ([:len [/interface wireguard peers find public-key=%q interface=%s]] = 0) do={ /interface wireguard peers add interface=%s public-key=%q allowed-address=%q endpoint-address=%q endpoint-port=%d persistent-keepalive=%d comment=\"wgmesh:peer\" } else={ /interface wireguard peers set [find public-key=%q interface=%s] allowed-address=%q endpoint-address=%q endpoint-port=%d persistent-keepalive=%d }",
			p.PublicKey, iface, iface, p.PublicKey, allowedStr, p.EndpointHost(), p.EndpointPort(), p.PersistentKeepalive,
			p.PublicKey, iface, allowedStr, p.EndpointHost(), p.EndpointPort(), p.PersistentKeepalive,
		)
		if out, err := r.run(cmdPeer); err != nil {
			return fmt.Errorf("drivers/mikrotik: ошибка добавления пира %s: %w\n%s", p.PublicKey, err, out)
		}
	}

	// 3. Назначение IP адреса на интерфейс
	cmdAddr := fmt.Sprintf(
		":if ([:len [/ip address find interface=%s]] = 0) do={ /ip address add address=%q interface=%s comment=\"wgmesh:%s\" }",
		iface, spec.Config.Address, iface, d.node.Name,
	)
	if out, err := r.run(cmdAddr); err != nil {
		return fmt.Errorf("drivers/mikrotik: не удалось назначить IP адрес: %w\n%s", err, out)
	}

	// 4. Настройка NAT / Forwarding если нода — exit или relay
	if spec.NAT {
		wan := "ether1"
		cmdNAT := fmt.Sprintf(
			":if ([:len [/ip firewall nat find comment=\"wgmesh-nat\"]] = 0) do={ /ip firewall nat add chain=srcnat out-interface=%s action=masquerade comment=\"wgmesh-nat\" }",
			wan,
		)
		r.run(cmdNAT)

		cmdFwd := fmt.Sprintf(
			":if ([:len [/ip firewall filter find comment=\"wgmesh-fwd\"]] = 0) do={ /ip firewall filter add chain=forward in-interface=%s action=accept comment=\"wgmesh-fwd\" }",
			iface,
		)
		r.run(cmdFwd)
	}

	return nil
}

// RemoveConfig удаляет конфигурацию wgmesh с ноды Mikrotik.
func (d *MikrotikDriver) RemoveConfig(node *config.Node) error {
	r, err := dialSSH(node)
	if err != nil {
		return err
	}
	defer r.client.Close()

	iface := node.WireGuard.Interface
	if iface == "" {
		iface = "wg0"
	}

	cmd := fmt.Sprintf(
		"/interface wireguard peers remove [find interface=%s]; /ip address remove [find interface=%s]; /interface wireguard remove [find name=%s]",
		iface, iface, iface,
	)
	_, err = r.run(cmd)
	return err
}

// GetStatus возвращает состояние wireguard интерфейса и пиров.
func (d *MikrotikDriver) GetStatus(node *config.Node) (string, error) {
	r, err := dialSSH(node)
	if err != nil {
		return "", err
	}
	defer r.client.Close()

	out1, _ := r.run("/interface wireguard print detail")
	out2, _ := r.run("/interface wireguard peers print detail")

	return fmt.Sprintf("=== Interface ===\n%s\n=== Peers ===\n%s", out1, out2), nil
}

