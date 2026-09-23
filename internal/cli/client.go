package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/skip2/go-qrcode"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	"github.com/meshctl/meshctl/internal/config"
	"github.com/meshctl/meshctl/internal/mesh"
	"github.com/meshctl/meshctl/internal/wg"
)

// clientState — кэш ключей клиентов в mesh.yaml (поле extension).
type clientState struct {
	Clients map[string]clientKeys `yaml:"clients,omitempty"`
}

type clientKeys struct {
	PrivateKey string `yaml:"private_key"`
	PublicKey  string `yaml:"public_key"`
	MeshIP     string `yaml:"mesh_ip"`
}

// rawMesh — «сырой» YAML для сохранения неизвестных полей (clients:).
type rawMesh struct {
	config.Mesh
	Extra map[string]interface{} `yaml:",inline"`
}

func clientConfigCmd() *cobra.Command {
	var (
		qr      bool
		outPath string
		dns     []string
	)
	cmd := &cobra.Command{
		Use:   "client-config <route>",
		Short: "Сгенерировать WireGuard-конфиг клиента для маршрута",
		Long: `Клиент — это виртуальный пир на первом хопе маршрута.
Ключи клиента кэшируются в mesh.yaml (секция clients), повторный вызов
возвращает тот же конфиг. Для multihop-маршрутов клиент подключается
к первому хопу, а default route берётся с exit-ноды (маршрутизация
настроена на нодах через apply).`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			m, err := loadMesh()
			if err != nil {
				return err
			}
			routeName := args[0]
			r := m.RouteByName(routeName)
			if r == nil {
				return fmt.Errorf("маршрут %q не найден", routeName)
			}
			hops := r.Hops()
			if len(hops) == 0 {
				return fmt.Errorf("маршрут %q не содержит нод", routeName)
			}
			first := m.NodeByName(hops[0])
			if first.WireGuard.PublicKey == "" {
				return fmt.Errorf("нода %q не инициализирована — сначала выполните `meshctl apply`", first.Name)
			}

			// ключи клиента: из кэша или новые
			state, err := loadClientState(cfgPath)
			if err != nil {
				return err
			}
			keys, ok := state.Clients[routeName]
			if !ok {
				kp, err := wg.GenerateKey()
				if err != nil {
					return err
				}
				mgr := mesh.NewManager(m)
				idx := routeIndex(m, routeName)
				keys = clientKeys{PrivateKey: kp.PrivateKey, PublicKey: kp.PublicKey, MeshIP: mgr.ClientAddressFor(idx)}
				if state.Clients == nil {
					state.Clients = map[string]clientKeys{}
				}
				state.Clients[routeName] = keys
				if err := saveClientState(cfgPath, m, state); err != nil {
					return err
				}
			}

			cc := &wg.ClientConfig{
				Name:       routeName,
				PrivateKey: keys.PrivateKey,
				Address:    keys.MeshIP + "/32",
				DNS:        dns,
				Peer: wg.PeerConf{
					PublicKey:           first.WireGuard.PublicKey,
					AllowedIPs:          []string{"0.0.0.0/0", "::/0"},
					Endpoint:            fmt.Sprintf("%s:%d", first.Host, first.WireGuard.ListenPort),
					PersistentKeepalive: keepalive(first),
				},
			}
			text := wg.RenderClient(cc)

			if outPath != "" {
				if err := os.WriteFile(outPath, []byte(text), 0o600); err != nil {
					return err
				}
				fmt.Printf("✔ Конфиг сохранён: %s\n", outPath)
			} else {
				fmt.Print(text)
			}
			if qr {
				png, err := qrcode.Encode(text, qrcode.Medium, 512)
				if err != nil {
					return err
				}
				name := outPath + ".png"
				if outPath == "" {
					name = routeName + ".qr.png"
				}
				if err := os.WriteFile(name, png, 0o644); err != nil {
					return err
				}
				fmt.Printf("✔ QR-код: %s\n", filepath.Base(name))
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&qr, "qr", false, "сохранить QR-код (PNG) для импорта в мобильный клиент")
	cmd.Flags().StringVarP(&outPath, "out", "o", "", "файл для записи .conf (по умолчанию stdout)")
	cmd.Flags().StringSliceVar(&dns, "dns", nil, "DNS-серверы через запятую (например 1.1.1.1,9.9.9.9)")
	return cmd
}

func keepalive(n *config.Node) int {
	if n.WireGuard.PersistentKeepalive > 0 {
		return n.WireGuard.PersistentKeepalive
	}
	return 25
}

// clientPublicKey — публичный ключ клиента маршрута из кэша, если он есть.
func clientPublicKey(path, routeName string) (string, error) {
	st, err := loadClientState(path)
	if err != nil {
		return "", err
	}
	k, ok := st.Clients[routeName]
	if !ok {
		return "", fmt.Errorf("клиент для маршрута %q ещё не сгенерирован", routeName)
	}
	return k.PublicKey, nil
}

// loadClientState читает секцию clients из mesh.yaml без потери остальных полей.
func loadClientState(path string) (*clientState, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	raw := map[string]interface{}{}
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return nil, err
	}
	st := &clientState{}
	if c, ok := raw["clients"]; ok {
		buf, _ := yaml.Marshal(c)
		if err := yaml.Unmarshal(buf, st); err != nil {
			return nil, fmt.Errorf("повреждена секция clients в %s: %w", path, err)
		}
	}
	return st, nil
}

// saveClientState дописывает clients в существующий YAML, сохраняя поля пользователя.
func saveClientState(path string, m *config.Mesh, st *clientState) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	raw := map[string]interface{}{}
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return err
	}
	raw["clients"] = st.Clients
	out, err := yaml.Marshal(raw)
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, out, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

var _ = rawMesh{} // резонанс структуры для будущих расширений
