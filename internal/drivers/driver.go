// Package drivers — абстракции платформ для применения конфигурации к нодам.
// Каждый драйвер реализует интерфейс Driver: ApplyConfig / RemoveConfig / GetStatus.
package drivers

import (
	"fmt"

	"github.com/wgmesh/wgmesh/internal/config"
	"github.com/wgmesh/wgmesh/internal/wg"
)

// NodeApplySpec — всё, что драйверу нужно знать про ноду для применения конфига.
type NodeApplySpec struct {
	Node   *config.Node
	Config *wg.NodeConfig // готовый wg-quick конфиг (текст рендерится драйвером/вызывающим)
	// NAT включает masquerade на exit-ноде; Forward включает ip_forward на relay.
	NAT     bool
	Forward bool
}

// Driver — универсальный интерфейс платформы.
type Driver interface {
	// Name возвращает имя платформы ("linux", "mikrotik", "openwrt").
	Name() string
	// ApplyConfig настраивает ноду согласно спеку (idempotent).
	ApplySpec(spec *NodeApplySpec) error
	// RemoveConfig удаляет WG-интерфейс с ноды.
	RemoveConfig(node *config.Node) error
	// GetStatus возвращает краткую сводку состояния (wg show).
	GetStatus(node *config.Node) (string, error)
}

// New возвращает драйвер по типу ноды.
func New(node *config.Node) (Driver, error) {
	switch node.Type {
	case config.TypeLinux:
		return NewLinux(node), nil
	case config.TypeMikrotik:
		return NewMikrotik(node), nil
	case config.TypeOpenWRT:
		return NewOpenWRT(node), nil
	default:
		return nil, fmt.Errorf("drivers: неизвестный тип ноды %q", node.Type)
	}
}

