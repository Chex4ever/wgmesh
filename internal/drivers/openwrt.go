package drivers

import (
	"fmt"
	"strings"

	"github.com/meshctl/meshctl/internal/config"
)

// OpenWRtDriver реализует провижининг роутеров OpenWRT через SSH и UCI.
type OpenWRtDriver struct {
	node *config.Node
}

// NewOpenWRT создаёт драйвер OpenWRT.
func NewOpenWRT(node *config.Node) *OpenWRtDriver {
	return &OpenWRtDriver{node: node}
}

// Name — openwrt.
func (d *OpenWRtDriver) Name() string { return "openwrt" }

// ApplySpec применит конфигурацию WireGuard через UCI (Unified Configuration Interface).
func (d *OpenWRtDriver) ApplySpec(spec *NodeApplySpec) error {
	r, err := dialSSH(d.node)
	if err != nil {
		return fmt.Errorf("drivers/openwrt: ошибка подключения SSH: %w", err)
	}
	defer r.client.Close()

	iface := spec.Node.WireGuard.Interface
	if iface == "" {
		iface = "mesh_wg"
	}
	port := spec.Node.WireGuard.ListenPort
	if port == 0 {
		port = 51820
	}

	// 1. Проверка наличия пакета wireguard
	if _, err := r.run("opkg list-installed | grep kmod-wireguard"); err != nil {
		r.run("opkg update && opkg install wireguard-tools kmod-wireguard")
	}

	// 2. Генерация UCI команд для сети
	var uciCmds []string
	uciCmds = append(uciCmds,
		fmt.Sprintf("uci set network.%s=interface", iface),
		fmt.Sprintf("uci set network.%s.proto='wireguard'", iface),
		fmt.Sprintf("uci set network.%s.private_key=%q", iface, spec.Config.PrivateKey),
		fmt.Sprintf("uci set network.%s.listen_port='%d'", iface, port),
		fmt.Sprintf("uci set network.%s.addresses=%q", iface, spec.Config.Address),
	)

	// 3. Добавление пиров через UCI
	for _, p := range spec.Config.Peers {
		allowedStr := strings.Join(p.AllowedIPs, " ")
		uciCmds = append(uciCmds,
			"uci add network wireguard_"+iface,
			fmt.Sprintf("uci set network.@wireguard_%s[-1].public_key=%q", iface, p.PublicKey),
			fmt.Sprintf("uci set network.@wireguard_%s[-1].allowed_ips=%q", iface, allowedStr),
			fmt.Sprintf("uci set network.@wireguard_%s[-1].endpoint_host=%q", iface, p.EndpointHost()),
			fmt.Sprintf("uci set network.@wireguard_%s[-1].endpoint_port='%d'", iface, p.EndpointPort()),
			fmt.Sprintf("uci set network.@wireguard_%s[-1].persistent_keepalive='%d'", iface, p.PersistentKeepalive),
		)
	}

	uciCmds = append(uciCmds, "uci commit network", fmt.Sprintf("ifup %s", iface))

	batchCmd := strings.Join(uciCmds, " && ")
	if out, err := r.run(batchCmd); err != nil {
		return fmt.Errorf("drivers/openwrt: не удалось применить UCI конфигурацию: %w\n%s", err, out)
	}

	// 4. Настройка файрвола (fw4 / nftables)
	if spec.NAT {
		fwCmds := []string{
			"uci set firewall.mesh_zone=zone",
			"uci set firewall.mesh_zone.name='mesh'",
			fmt.Sprintf("uci set firewall.mesh_zone.network='%s'", iface),
			"uci set firewall.mesh_zone.input='ACCEPT'",
			"uci set firewall.mesh_zone.output='ACCEPT'",
			"uci set firewall.mesh_zone.forward='ACCEPT'",
			"uci set firewall.mesh_zone.masq='1'",
			"uci commit firewall",
			"/etc/init.d/firewall reload",
		}
		r.run(strings.Join(fwCmds, " && "))
	}

	return nil
}

// RemoveConfig удаляет конфигурацию интерфейса с роутера OpenWRT.
func (d *OpenWRtDriver) RemoveConfig(node *config.Node) error {
	r, err := dialSSH(node)
	if err != nil {
		return err
	}
	defer r.client.Close()

	iface := node.WireGuard.Interface
	if iface == "" {
		iface = "mesh_wg"
	}

	cmd := fmt.Sprintf("ifdown %s; uci delete network.%s; uci commit network", iface, iface)
	_, err = r.run(cmd)
	return err
}

// GetStatus выводит состояние wireguard интерфейса через ubus или wg show.
func (d *OpenWRtDriver) GetStatus(node *config.Node) (string, error) {
	r, err := dialSSH(node)
	if err != nil {
		return "", err
	}
	defer r.client.Close()

	out, err := r.run("wg show || ubus call network.interface.mesh_wg status")
	return out, err
}
