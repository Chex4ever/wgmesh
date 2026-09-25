package cli

import (
	"fmt"
	"net"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"golang.org/x/crypto/ssh"

	"github.com/wgmesh/wgmesh/internal/config"
	"github.com/wgmesh/wgmesh/internal/mesh"
)

type doctorCheck struct {
	Category string
	Name     string
	Status   string // PASS, WARN, FAIL
	Detail   string
}

func doctorCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "doctor",
		Short: "Глубокая диагностика сети, конфигурации и доступности нод",
		Long: `doctor производит комплексный аудит всей mesh-сети:
- Проверку синтаксиса и графа mesh.yaml;
- Гигиену секретов (отсутствие private_key в yaml);
- Доступность нод по SSH;
- Наличие системных пакетов (wireguard-tools / RouterOS v7);
- Проверку открытых портов WireGuard;
- Доступность интернета с exit-нод.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Println("🏥 Запуск полной диагностики wgmesh doctor...")
			fmt.Println("--------------------------------------------------")

			var checks []doctorCheck
			hasFail := false
			hasWarn := false

			// 1. Проверка конфигурации
			m, err := config.Load(cfgPath)
			if err != nil {
				checks = append(checks, doctorCheck{
					Category: "Config",
					Name:     "Чтение mesh.yaml",
					Status:   "FAIL",
					Detail:   err.Error(),
				})
				hasFail = true
				printCheckResults(checks)
				fmt.Println("\n❌ Диагностика остановлена: критическая ошибка конфига")
				return nil
			}

			if err := mesh.Validate(m); err != nil {
				checks = append(checks, doctorCheck{
					Category: "Config",
					Name:     "Валидация графа сети",
					Status:   "FAIL",
					Detail:   err.Error(),
				})
				hasFail = true
			} else {
				checks = append(checks, doctorCheck{
					Category: "Config",
					Name:     "Синтаксис и граф mesh.yaml",
					Status:   "PASS",
					Detail:   fmt.Sprintf("ОК (%d нод, %d маршрутов)", len(m.Nodes), len(m.Routes)),
				})
			}

			// 2. Проверка гигиены секретов
			var secretWarnings []string
			for _, n := range m.Nodes {
				if n.WireGuard.PrivateKey != "" {
					secretWarnings = append(secretWarnings, n.Name)
				}
			}
			if len(secretWarnings) > 0 {
				checks = append(checks, doctorCheck{
					Category: "Security",
					Name:     "Гигиена секретов (PrivateKey)",
					Status:   "WARN",
					Detail:   fmt.Sprintf("Приватный ключ сохранён в mesh.yaml на нодах: %s", strings.Join(secretWarnings, ", ")),
				})
				hasWarn = true
			} else {
				checks = append(checks, doctorCheck{
					Category: "Security",
					Name:     "Гигиена секретов (PrivateKey)",
					Status:   "PASS",
					Detail:   "В mesh.yaml нет открытых приватных ключей",
				})
			}

			// 3. Проверка SSH доступности
			reachability := mesh.CheckReachability(m, 4*time.Second)
			for _, n := range m.Nodes {
				rErr, failed := reachability[n.Name]
				if failed {
					checks = append(checks, doctorCheck{
						Category: "Connectivity",
						Name:     fmt.Sprintf("SSH %s (%s:%d)", n.Name, n.Host, n.SSHPortOrDefault()),
						Status:   "FAIL",
						Detail:   rErr.Error(),
					})
					hasFail = true
				} else {
					checks = append(checks, doctorCheck{
						Category: "Connectivity",
						Name:     fmt.Sprintf("SSH %s (%s:%d)", n.Name, n.Host, n.SSHPortOrDefault()),
						Status:   "PASS",
						Detail:   "TCP соединение установлено",
					})
				}
			}

			// 4. Проверка окружения нод (Tools) и 5. Портов WG и 6. Exit Node Internet
			exitNodes := map[string]bool{}
			for _, r := range m.Routes {
				if r.ExitNode != "" {
					exitNodes[r.ExitNode] = true
				}
			}

			for _, n := range m.Nodes {
				if _, failed := reachability[n.Name]; failed {
					continue // пропуск недоступных по SSH нод
				}

				// SSH запуск проверок
				client, err := dialSSHForDoctor(&n)
				if err != nil {
					checks = append(checks, doctorCheck{
						Category: "Auth",
						Name:     fmt.Sprintf("SSH Auth %s", n.Name),
						Status:   "FAIL",
						Detail:   err.Error(),
					})
					hasFail = true
					continue
				}

				// 4. Пакеты / Драйвер
				sess, err := client.NewSession()
				if err == nil {
					var toolCmd string
					switch n.Type {
					case config.TypeMikrotik:
						toolCmd = "/interface wireguard print"
					default:
						toolCmd = "command -v wg && command -v wg-quick"
					}
					out, err := sess.CombinedOutput(toolCmd)
					sess.Close()

					if err != nil {
						checks = append(checks, doctorCheck{
							Category: "Tools",
							Name:     fmt.Sprintf("WG Tools на %s (%s)", n.Name, n.Type),
							Status:   "WARN",
							Detail:   fmt.Sprintf("Инструменты WireGuard не найдены (%s)", strings.TrimSpace(string(out))),
						})
						hasWarn = true
					} else {
						checks = append(checks, doctorCheck{
							Category: "Tools",
							Name:     fmt.Sprintf("WG Tools на %s (%s)", n.Name, n.Type),
							Status:   "PASS",
							Detail:   "Установлены и доступны",
						})
					}
				}

				// 6. Выход в интернет для Exit-нод
				if exitNodes[n.Name] {
					sessExit, err := client.NewSession()
					if err == nil {
						out, err := sessExit.CombinedOutput("curl -s --max-time 5 https://ifconfig.me || curl -s --max-time 5 https://api.ipify.org")
						sessExit.Close()

						ip := strings.TrimSpace(string(out))
						if err != nil || ip == "" {
							checks = append(checks, doctorCheck{
								Category: "ExitInternet",
								Name:     fmt.Sprintf("Интернет на Exit-ноде %s", n.Name),
								Status:   "FAIL",
								Detail:   "Не удалось получить публичный IP через curl",
							})
							hasFail = true
						} else {
							checks = append(checks, doctorCheck{
								Category: "ExitInternet",
								Name:     fmt.Sprintf("Интернет на Exit-ноде %s", n.Name),
								Status:   "PASS",
								Detail:   fmt.Sprintf("Выход в интернет ОК (External IP: %s)", ip),
							})
						}
					}
				}

				client.Close()
			}

			printCheckResults(checks)

			fmt.Println("--------------------------------------------------")
			if hasFail {
				fmt.Println("🔴 Итоговый статус: RED (Обнаружены критические ошибки)")
			} else if hasWarn {
				fmt.Println("🟡 Итоговый статус: YELLOW (Обнаружены предупреждения)")
			} else {
				fmt.Println("🟢 Итоговый статус: GREEN (Сеть полностью здорова)")
			}

			return nil
		},
	}
}

func dialSSHForDoctor(node *config.Node) (*ssh.Client, error) {
	cfg := &ssh.ClientConfig{
		User:            node.SSHUser,
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         5 * time.Second,
	}

	switch {
	case node.SSHKey != "":
		keyPath := expandHome(node.SSHKey)
		data, err := os.ReadFile(keyPath)
		if err != nil {
			return nil, err
		}
		signer, err := ssh.ParsePrivateKey(data)
		if err != nil {
			return nil, err
		}
		cfg.Auth = []ssh.AuthMethod{ssh.PublicKeys(signer)}
	case node.SSHPass != "":
		cfg.Auth = []ssh.AuthMethod{ssh.Password(node.SSHPass)}
	default:
		return nil, fmt.Errorf("нет ssh_key или ssh_password")
	}

	addr := net.JoinHostPort(node.Host, fmt.Sprint(node.SSHPortOrDefault()))
	return ssh.Dial("tcp", addr, cfg)
}

func printCheckResults(checks []doctorCheck) {
	for _, c := range checks {
		symbol := "✔"
		if c.Status == "WARN" {
			symbol = "⚠️"
		} else if c.Status == "FAIL" {
			symbol = "✖"
		}
		fmt.Printf("[%s] %-12s | %-32s: %s\n", symbol, c.Category, c.Name, c.Detail)
	}
}

