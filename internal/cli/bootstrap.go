package cli

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/pem"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"golang.org/x/crypto/ssh"

	"github.com/wgmesh/wgmesh/internal/config"
)

const defaultSSHKeyRelPath = "~/.config/wgmesh/keys/id_ed25519"

func nodeBootstrapCmd() *cobra.Command {
	var (
		host, user, password, nType string
		sshPort                     int
		protected                   bool
	)

	cmd := &cobra.Command{
		Use:   "bootstrap <name>",
		Short: "Zero-touch подключение ноды (сброс SSH-ключа и добавление в mesh.yaml)",
		Long: `node bootstrap загружает выделенный SSH-ключ mesh-сети (~/.config/wgmesh/keys/id_ed25519)
на новую ноду по временному паролю, прописывает его в authorized_keys или RouterOS SSH keys,
и сохраняет ноду в mesh.yaml.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]
			if host == "" {
				return fmt.Errorf("обязательно укажите --host <ip/domain>")
			}

			keyPath, pubKeyStr, err := ensureDefaultSSHKey()
			if err != nil {
				return fmt.Errorf("не удалось подготовить SSH-ключ: %w", err)
			}

			fmt.Printf("→ Провижининг SSH-ключа (%s.pub) на %s@%s:%d…\n", keyPath, user, host, sshPort)

			if err := installRemoteSSHKey(host, sshPort, user, password, keyPath, pubKeyStr, nType); err != nil {
				return fmt.Errorf("ошибка установки SSH-ключа на удаленный хост: %w", err)
			}

			m, err := loadMesh()
			if err != nil {
				// Если конфиг еще не создан, создадим дефолтный
				m = &config.Mesh{
					Name:    "mesh",
					Version: 1,
					CIDR:    config.DefaultCIDR,
				}
			}

			existing := m.NodeByName(name)
			if existing != nil {
				existing.Host = host
				existing.SSHUser = user
				existing.SSHKey = defaultSSHKeyRelPath
				existing.SSHPort = sshPort
				existing.Type = nType
				existing.Protected = protected
			} else {
				node := config.Node{
					Name:      name,
					Type:      nType,
					Host:      host,
					SSHUser:   user,
					SSHKey:    defaultSSHKeyRelPath,
					SSHPort:   sshPort,
					Protected: protected,
					WireGuard: config.WG{
						Interface:  "wg0",
						ListenPort: 51820,
					},
				}
				m.Nodes = append(m.Nodes, node)
			}

			if err := saveMesh(m); err != nil {
				return fmt.Errorf("не удалось сохранить %s: %w", cfgPath, err)
			}

			fmt.Printf("✔ Нода %q (%s) успешно подготовлена и сохранена в %s!\n", name, host, cfgPath)
			return nil
		},
	}

	cmd.Flags().StringVar(&host, "host", "", "публичный IP или имя хоста ноды (обязательно)")
	cmd.Flags().StringVar(&user, "user", "root", "SSH-пользователь")
	cmd.Flags().StringVar(&password, "password", "", "пароль SSH для первичной установки ключа")
	cmd.Flags().StringVar(&nType, "type", config.TypeLinux, "тип ноды: linux|mikrotik|openwrt")
	cmd.Flags().IntVar(&sshPort, "ssh-port", 22, "порт SSH")
	cmd.Flags().BoolVar(&protected, "protected", false, "пометить ноду как защищённую (protected: true)")

	return cmd
}

// ensureDefaultSSHKey проверяет или генерирует ED25519 SSH-ключ в ~/.config/wgmesh/keys/id_ed25519.
func ensureDefaultSSHKey() (string, string, error) {
	keyPath := expandHome(defaultSSHKeyRelPath)
	pubPath := keyPath + ".pub"

	if _, err := os.Stat(keyPath); err == nil {
		pubData, err := os.ReadFile(pubPath)
		if err == nil {
			return keyPath, strings.TrimSpace(string(pubData)), nil
		}
	}

	dir := filepath.Dir(keyPath)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return "", "", fmt.Errorf("создание директории %s: %w", dir, err)
	}

	pubKey, privKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return "", "", fmt.Errorf("генерация ed25519: %w", err)
	}

	sshPrivBlock, err := ssh.MarshalPrivateKey(cryptoPrivateKey(privKey), "")
	if err != nil {
		return "", "", fmt.Errorf("маршалинг приватного ключа: %w", err)
	}

	if err := os.WriteFile(keyPath, pem.EncodeToMemory(sshPrivBlock), 0600); err != nil {
		return "", "", fmt.Errorf("запись приватного ключа: %w", err)
	}

	sshPubKey, err := ssh.NewPublicKey(pubKey)
	if err != nil {
		return "", "", fmt.Errorf("конвертация публичного ключа: %w", err)
	}

	pubBytes := ssh.MarshalAuthorizedKey(sshPubKey)
	pubStr := strings.TrimSpace(string(pubBytes))
	if err := os.WriteFile(pubPath, pubBytes, 0644); err != nil {
		return "", "", fmt.Errorf("запись публичного ключа: %w", err)
	}

	return keyPath, pubStr, nil
}

func cryptoPrivateKey(k ed25519.PrivateKey) interface{} {
	return k
}

func installRemoteSSHKey(host string, port int, user, password, keyPath, pubKeyStr, nType string) error {
	var authMethods []ssh.AuthMethod
	if password != "" {
		authMethods = append(authMethods, ssh.Password(password))
	}
	if keyData, err := os.ReadFile(keyPath); err == nil {
		if signer, err := ssh.ParsePrivateKey(keyData); err == nil {
			authMethods = append(authMethods, ssh.PublicKeys(signer))
		}
	}

	if len(authMethods) == 0 {
		return fmt.Errorf("укажите --password для подключения по SSH")
	}

	cfg := &ssh.ClientConfig{
		User:            user,
		Auth:            authMethods,
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         15 * time.Second,
	}

	addr := net.JoinHostPort(host, fmt.Sprint(port))
	client, err := ssh.Dial("tcp", addr, cfg)
	if err != nil {
		return fmt.Errorf("подключение SSH к %s: %w", addr, err)
	}
	defer client.Close()

	sess, err := client.NewSession()
	if err != nil {
		return err
	}
	defer sess.Close()

	cleanPubKey := strings.TrimSpace(pubKeyStr)
	switch nType {
	case config.TypeMikrotik:
		cmd := fmt.Sprintf(
			"/file print file=id_ed25519.pub; /file set id_ed25519.pub contents=%q; /user ssh-keys import public-key-file=id_ed25519.pub user=%s",
			cleanPubKey, user,
		)
		out, err := sess.CombinedOutput(cmd)
		if err != nil {
			// Alt try ROS 7.15+ command
			sess2, err2 := client.NewSession()
			if err2 == nil {
				defer sess2.Close()
				cmd2 := fmt.Sprintf("/user ssh-public-keys add user=%s key=%q", user, cleanPubKey)
				out2, err2 := sess2.CombinedOutput(cmd2)
				if err2 != nil {
					return fmt.Errorf("RouterOS import failed: %s / %s", string(out), string(out2))
				}
			}
		}
	default: // linux / openwrt
		cmd := fmt.Sprintf(
			"mkdir -p ~/.ssh && chmod 700 ~/.ssh && (grep -q -F %q ~/.ssh/authorized_keys 2>/dev/null || echo %q >> ~/.ssh/authorized_keys) && chmod 600 ~/.ssh/authorized_keys",
			cleanPubKey, cleanPubKey,
		)
		out, err := sess.CombinedOutput(cmd)
		if err != nil {
			return fmt.Errorf("команда установки authorized_keys завершилась с ошибкой: %w\n%s", err, string(out))
		}
	}

	return nil
}

