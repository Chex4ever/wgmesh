package drivers

import (
	"bytes"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"time"

	"golang.org/x/crypto/ssh"

	"github.com/meshctl/meshctl/internal/config"
	"github.com/meshctl/meshctl/internal/wg"
)

// sshRunner — минимальная обёртка над SSH-сессией.
type sshRunner struct {
	client *ssh.Client
}

func dialSSH(node *config.Node) (*sshRunner, error) {
	cfg := &ssh.ClientConfig{
		User:            node.SSHUser,
		HostKeyCallback: ssh.InsecureIgnoreHostKey(), // TODO(фаза 2): known_hosts
		Timeout:         15 * time.Second,
	}
	switch {
	case node.SSHKey != "":
		keyPath := expandHome(node.SSHKey)
		data, err := os.ReadFile(keyPath)
		if err != nil {
			return nil, fmt.Errorf("drivers: не удалось прочитать ключ %s: %w", keyPath, err)
		}
		var signer ssh.Signer
		if node.SSHPass != "" {
			signer, err = ssh.ParsePrivateKeyWithPassphrase(data, []byte(node.SSHPass))
		} else {
			signer, err = ssh.ParsePrivateKey(data)
		}
		if err != nil {
			return nil, fmt.Errorf("drivers: не удалось разобрать ключ: %w", err)
		}
		cfg.Auth = []ssh.AuthMethod{ssh.PublicKeys(signer)}
	case node.SSHPass != "":
		cfg.Auth = []ssh.AuthMethod{ssh.Password(node.SSHPass)}
	default:
		return nil, fmt.Errorf("drivers: для ноды %q не задан ни ssh_key, ни ssh_password", node.Name)
	}

	addr := net.JoinHostPort(node.Host, fmt.Sprint(node.SSHPortOrDefault()))
	c, err := ssh.Dial("tcp", addr, cfg)
	if err != nil {
		return nil, fmt.Errorf("drivers: ssh к %q (%s): %w", node.Name, addr, err)
	}
	return &sshRunner{client: c}, nil
}

// run выполняет команду на ноде, возвращает stdout+stderr.
func (r *sshRunner) run(cmd string) (string, error) {
	sess, err := r.client.NewSession()
	if err != nil {
		return "", err
	}
	defer sess.Close()
	var buf bytes.Buffer
	sess.Stdout = &buf
	sess.Stderr = &buf
	err = sess.Run(cmd)
	return buf.String(), err
}

// write uploads content to a remote path via heredoc (0600).
func (r *sshRunner) write(path, content string) error {
	cmd := fmt.Sprintf("umask 077; cat > %s <<'MESHCTL_EOF'\n%s\nMESHCTL_EOF", path, content)
	_, err := r.run(cmd)
	return err
}

func expandHome(p string) string {
	if strings.HasPrefix(p, "~/") {
		if home, err := os.UserHomeDir(); err == nil {
			return filepath.Join(home, p[2:])
		}
	}
	return p
}

// LinuxDriver настраивает Linux-ноду по SSH: wg-quick + iptables.
type LinuxDriver struct {
	node *config.Node
}

// NewLinux создаёт драйвер Linux.
func NewLinux(node *config.Node) *LinuxDriver { return &LinuxDriver{node: node} }

// Name — linux.
func (d *LinuxDriver) Name() string { return "linux" }

// ApplySpec подключается по SSH и применяет конфигурацию WireGuard:
//  1. проверяет наличие wireguard-tools (wg-quick), при отсутствии ставит через apt/yum/apk;
//  2. пишет /etc/wireguard/<iface>.conf;
//  3. включает ip_forward (для relay/exit);
//  4. включает NAT (masquerade) только на exit-ноде;
//  5. systemctl restart wg-quick@<iface>.
func (d *LinuxDriver) ApplySpec(spec *NodeApplySpec) error {
	r, err := dialSSH(d.node)
	if err != nil {
		return err
	}
	defer r.client.Close()

	// 1. наличие wg-quick
	if _, err := r.run("command -v wg-quick"); err != nil {
		out, err := r.run(installWGCmd())
		if err != nil {
			return fmt.Errorf("drivers/linux: установка wireguard-tools: %w\n%s", err, out)
		}
	}

	// 2. конфиг интерфейса
	confText := wg.RenderNode(spec.Config)
	confPath := fmt.Sprintf("/etc/wireguard/%s.conf", spec.Node.WireGuard.Interface)
	if err := r.write(confPath, confText); err != nil {
		return fmt.Errorf("drivers/linux: запись %s: %w", confPath, err)
	}

	// 3. forwarding
	if spec.Forward || spec.NAT {
		if _, err := r.run("sysctl -w net.ipv4.ip_forward=1 && sed -i 's/^#*net.ipv4.ip_forward.*/net.ipv4.ip_forward=1/' /etc/sysctl.conf || true"); err != nil {
			return fmt.Errorf("drivers/linux: ip_forward: %w", err)
		}
	}

	// 4. NAT на exit-ноде (идемпотентно: -C перед -t nat ... -A)
	if spec.NAT {
		natRule := fmt.Sprintf("iptables -t nat -C POSTROUTING -o eth0 -j MASQUERADE 2>/dev/null || "+
			"iptables -t nat -A POSTROUTING -o eth0 -j MASQUERADE; "+
			"iptables -C FORWARD -i %s -o eth0 -j ACCEPT 2>/dev/null || "+
			"iptables -A FORWARD -i %s -o eth0 -j ACCEPT; "+
			"iptables -C FORWARD -i eth0 -o %s -m state --state RELATED,ESTABLISHED -j ACCEPT 2>/dev/null || "+
			"iptables -A FORWARD -i eth0 -o %s -m state --state RELATED,ESTABLISHED -j ACCEPT",
			spec.Node.WireGuard.Interface, spec.Node.WireGuard.Interface,
			spec.Node.WireGuard.Interface, spec.Node.WireGuard.Interface)
		if out, err := r.run(natRule); err != nil {
			return fmt.Errorf("drivers/linux: NAT: %w\n%s", err, out)
		}
	}

	// 5. перезапуск интерфейса
	out, err := r.run(fmt.Sprintf("systemctl enable --now wg-quick@%s && systemctl restart wg-quick@%s",
		spec.Node.WireGuard.Interface, spec.Node.WireGuard.Interface))
	if err != nil {
		// fallback для систем без systemd
		out2, err2 := r.run(fmt.Sprintf("wg-quick up %s 2>/dev/null; wg syncconf %s %s",
			spec.Node.WireGuard.Interface, spec.Node.WireGuard.Interface, confPath))
		if err2 != nil {
			return fmt.Errorf("drivers/linux: запуск wg-quick@%s: %w\n%s\n%s", spec.Node.WireGuard.Interface, err, out, out2)
		}
	}
	return nil
}

// RemoveConfig останавливает и удаляет WG-интерфейс.
func (d *LinuxDriver) RemoveConfig(node *config.Node) error {
	r, err := dialSSH(node)
	if err != nil {
		return err
	}
	defer r.client.Close()
	_, err = r.run(fmt.Sprintf("systemctl stop wg-quick@%s; rm -f /etc/wireguard/%s.conf",
		node.WireGuard.Interface, node.WireGuard.Interface))
	return err
}

// GetStatus выводит `wg show`.
func (d *LinuxDriver) GetStatus(node *config.Node) (string, error) {
	r, err := dialSSH(node)
	if err != nil {
		return "", err
	}
	defer r.client.Close()
	return r.run("wg show")
}

func installWGCmd() string {
	return `(command -v apt-get && apt-get update -y && apt-get install -y wireguard-tools) \
|| (command -v yum && yum install -y wireguard-tools) \
|| (command -v apk && apk add --no-cache wireguard-tools) \
|| (command -v dnf && dnf install -y wireguard-tools)`
}
