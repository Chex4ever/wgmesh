package drivers

import (
	"fmt"

	"github.com/meshctl/meshctl/internal/config"
)

// MikrotikDriver — заглушка до Фазы 3 (SSH + RouterOS CLI).
type MikrotikDriver struct{ node *config.Node }

// NewMikrotik создаёт драйвер Mikrotik.
func NewMikrotik(node *config.Node) *MikrotikDriver { return &MikrotikDriver{node: node} }

// Name — mikrotik.
func (d *MikrotikDriver) Name() string { return "mikrotik" }

// ApplySpec пока не реализован (Фаза 3): /interface wireguard, /ip route, откат.
func (d *MikrotikDriver) ApplySpec(spec *NodeApplySpec) error {
	return fmt.Errorf("drivers/mikrotik: поддержка ноды %q будет добавлена в Фазе 3 (см. Plan-001.md)", d.node.Name)
}

// RemoveConfig — Фаза 3.
func (d *MikrotikDriver) RemoveConfig(node *config.Node) error {
	return fmt.Errorf("drivers/mikrotik: будет реализован в Фазе 3")
}

// GetStatus — Фаза 3.
func (d *MikrotikDriver) GetStatus(node *config.Node) (string, error) {
	return "", fmt.Errorf("drivers/mikrotik: будет реализован в Фазе 3")
}

// OpenWRtdriver — заглушка до Фазы 3 (UCI: /etc/config/wireguard).
type OpenWRtDriver struct{ node *config.Node }

// NewOpenWRT создаёт драйвер OpenWRT.
func NewOpenWRT(node *config.Node) *OpenWRtDriver { return &OpenWRtDriver{node: node} }

// Name — openwrt.
func (d *OpenWRtDriver) Name() string { return "openwrt" }

// ApplySpec — Фаза 3.
func (d *OpenWRtDriver) ApplySpec(spec *NodeApplySpec) error {
	return fmt.Errorf("drivers/openwrt: будет реализован в Фазе 3")
}

// RemoveConfig — Фаза 3.
func (d *OpenWRtDriver) RemoveConfig(node *config.Node) error {
	return fmt.Errorf("drivers/openwrt: будет реализован в Фазе 3")
}

// GetStatus — Фаза 3.
func (d *OpenWRtDriver) GetStatus(node *config.Node) (string, error) {
	return "", fmt.Errorf("drivers/openwrt: будет реализован в Фазе 3")
}
