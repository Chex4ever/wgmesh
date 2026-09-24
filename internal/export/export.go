package export

import (
	"fmt"

	"github.com/meshctl/meshctl/internal/config"
)

// Exporter — интерфейс генератора клиентских профилей.
type Exporter interface {
	Name() string
	Render(m *config.Mesh, route *config.Route, client *config.Client) ([]byte, error)
}

var registry = map[string]Exporter{}

func Register(e Exporter) {
	registry[e.Name()] = e
}

func Get(name string) (Exporter, error) {
	e, ok := registry[name]
	if !ok {
		return nil, fmt.Errorf("неизвестный формат экспорта: %q", name)
	}
	return e, nil
}

func Formats() []string {
	var list []string
	for k := range registry {
		list = append(list, k)
	}
	return list
}
