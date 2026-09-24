package tui

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/pem"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/skip2/go-qrcode"
	"golang.org/x/crypto/ssh"

	"github.com/meshctl/meshctl/internal/config"
	"github.com/meshctl/meshctl/internal/drivers"
	"github.com/meshctl/meshctl/internal/export"
	"github.com/meshctl/meshctl/internal/i18n"
	"github.com/meshctl/meshctl/internal/mesh"
	"github.com/meshctl/meshctl/internal/updater"
	"github.com/meshctl/meshctl/internal/wg"
)

type checkUpdateMsg struct {
	hasUpdate     bool
	latestVersion string
	downloadURL   string
	err           error
}

type performUpdateMsg struct {
	err error
}

func checkUpdateCmd(version string) tea.Cmd {
	return func() tea.Msg {
		hasUpdate, latestVer, downloadURL, err := updater.CheckForUpdate(version)
		return checkUpdateMsg{
			hasUpdate:     hasUpdate,
			latestVersion: latestVer,
			downloadURL:   downloadURL,
			err:           err,
		}
	}
}

func performUpdateCmd(downloadURL string) tea.Cmd {
	return func() tea.Msg {
		err := updater.PerformUpdate(downloadURL)
		return performUpdateMsg{err: err}
	}
}

func (m *Model) openUpdateModal() tea.Cmd {
	m.Modal = ModalState{
		Type:         ModalUpdate,
		UpdateStatus: i18n.T("update_checking", m.Version),
		IsUpdating:   false,
		HasUpdate:    false,
	}
	return checkUpdateCmd(m.Version)
}

func (m *Model) runApply() {
	mgr := mesh.NewManager(m.Mesh)
	keys, err := mgr.EnsureKeys()
	if err != nil {
		m.LogMsg = fmt.Sprintf("Ошибка генерации ключей: %v", err)
		return
	}
	ips, err := mgr.EnsureIPs()
	if err != nil {
		m.LogMsg = fmt.Sprintf("Ошибка назначения IP: %v", err)
		return
	}
	if keys > 0 || ips > 0 {
		_ = config.Save(m.ConfigPath, m.Mesh)
	}

	if err := mesh.Validate(m.Mesh); err != nil {
		m.LogMsg = fmt.Sprintf("Ошибка валидации: %v", err)
		return
	}

	plans := mgr.BuildPlan()
	var failed []string
	for _, p := range plans {
		d, err := drivers.New(p.Node)
		if err != nil {
			failed = append(failed, p.Node.Name)
			continue
		}
		spec := &drivers.NodeApplySpec{
			Node:    p.Node,
			Config:  &wg.NodeConfig{Name: p.Node.Name, PrivateKey: p.Node.WireGuard.PrivateKey, Address: p.Node.MeshIP + "/24", ListenPort: p.Node.WireGuard.ListenPort},
			NAT:     p.IsExit,
			Forward: p.IsRelay || p.IsExit,
		}
		if err := d.ApplySpec(spec); err != nil {
			failed = append(failed, p.Node.Name)
		}
	}

	if len(failed) > 0 {
		m.LogMsg = fmt.Sprintf("[FAIL] Ошибка применения на нодах: %v", failed)
	} else {
		m.LogMsg = "[OK] Конфигурация успешно применена ко всем участвующим нодам по SSH!"
	}
}

func (m *Model) runDoctor() {
	var lines []string
	hasFail := false
	hasWarn := false

	// Config
	if err := mesh.Validate(m.Mesh); err != nil {
		lines = append(lines, fmt.Sprintf("[FAIL] Config       | Синтаксис и граф: %v", err))
		hasFail = true
	} else {
		lines = append(lines, fmt.Sprintf("[OK] Config       | Синтаксис и граф: ОК (%d нод, %d маршрутов)", len(m.Mesh.Nodes), len(m.Mesh.Routes)))
	}

	// Security
	var secretWarns []string
	for _, n := range m.Mesh.Nodes {
		if n.WireGuard.PrivateKey != "" {
			secretWarns = append(secretWarns, n.Name)
		}
	}
	if len(secretWarns) > 0 {
		lines = append(lines, fmt.Sprintf("[WARN] Security     | Секреты в YAML: Приватный ключ сохранён на %s", strings.Join(secretWarns, ", ")))
		hasWarn = true
	} else {
		lines = append(lines, "[OK] Security     | Секреты в YAML: В mesh.yaml нет открытых приватных ключей")
	}

	// SSH reachability
	reachability := mesh.CheckReachability(m.Mesh, 3*time.Second)
	for _, n := range m.Mesh.Nodes {
		if rErr, failed := reachability[n.Name]; failed {
			lines = append(lines, fmt.Sprintf("[FAIL] SSH          | %s (%s): %v", n.Name, n.Host, rErr))
			hasFail = true
		} else {
			lines = append(lines, fmt.Sprintf("[OK] SSH          | %s (%s): TCP соединение установлено", n.Name, n.Host))
		}
	}

	status := "GREEN"
	if hasFail {
		status = "RED"
	} else if hasWarn {
		status = "YELLOW"
	}

	m.Modal = ModalState{
		Type:         ModalDoctor,
		DoctorOutput: lines,
		DoctorStatus: status,
	}
}

func (m *Model) handleExportFormat(fmtKey string) {
	if len(m.Mesh.Routes) == 0 || m.SelectedRoute >= len(m.Mesh.Routes) {
		return
	}
	r := &m.Mesh.Routes[m.SelectedRoute]
	var c *config.Client
	if len(m.Mesh.Clients) > 0 && m.SelectedClient < len(m.Mesh.Clients) {
		c = &m.Mesh.Clients[m.SelectedClient]
	}

	fmtName := "wireguard"
	showQR := false
	switch fmtKey {
	case "a":
		fmtName = "amnezia"
	case "s":
		fmtName = "sing-box"
	case "u":
		fmtName = "uri"
	case "q":
		fmtName = "wireguard"
		showQR = true
	}

	exp, err := export.Get(fmtName)
	if err != nil {
		m.LogMsg = fmt.Sprintf("Ошибка экспорта: %v", err)
		return
	}
	data, err := exp.Render(m.Mesh, r, c)
	if err != nil {
		m.LogMsg = fmt.Sprintf("Ошибка рендеринга профиля: %v", err)
		return
	}

	qrStr := ""
	if showQR {
		qr, err := qrcode.New(string(data), qrcode.Medium)
		if err == nil {
			qrStr = qr.ToSmallString(false)
		}
	}

	m.Modal = ModalState{
		Type:       ModalExport,
		ExportData: string(data),
		ShowQR:     showQR,
		QRString:   qrStr,
	}
}

func (m *Model) openGitModal() {
	out, _ := exec.Command("git", "status", "-s").CombinedOutput()
	m.Modal = ModalState{
		Type:      ModalGit,
		GitStatus: string(out),
		Fields: []FormField{
			{Label: "Сообщение коммита", Value: "update mesh configuration"},
		},
	}
}

func ensureDefaultSSHKeyTUI() (string, string, error) {
	keyPath := expandHomeTUI("~/.config/wgmesh/keys/id_ed25519")
	pubPath := keyPath + ".pub"

	if _, err := os.Stat(keyPath); err == nil {
		pubData, err := os.ReadFile(pubPath)
		if err == nil {
			return keyPath, strings.TrimSpace(string(pubData)), nil
		}
	}

	dir := filepath.Dir(keyPath)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return "", "", err
	}

	pubKey, privKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return "", "", err
	}

	sshPrivBlock, err := ssh.MarshalPrivateKey(privKey, "")
	if err != nil {
		return "", "", err
	}

	if err := os.WriteFile(keyPath, pem.EncodeToMemory(sshPrivBlock), 0600); err != nil {
		return "", "", err
	}

	sshPubKey, err := ssh.NewPublicKey(pubKey)
	if err != nil {
		return "", "", err
	}

	pubBytes := ssh.MarshalAuthorizedKey(sshPubKey)
	pubStr := strings.TrimSpace(string(pubBytes))
	_ = os.WriteFile(pubPath, pubBytes, 0644)

	return keyPath, pubStr, nil
}

func installRemoteKeyTUI(host string, port int, user, password, keyPath, pubKeyStr, nType string) error {
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
		return fmt.Errorf("укажите пароль для подключения")
	}

	cfg := &ssh.ClientConfig{
		User:            user,
		Auth:            authMethods,
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         10 * time.Second,
	}

	addr := net.JoinHostPort(host, fmt.Sprint(port))
	client, err := ssh.Dial("tcp", addr, cfg)
	if err != nil {
		return err
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
		cmd := fmt.Sprintf("/file print file=id_ed25519.pub; /file set id_ed25519.pub contents=%q; /user ssh-keys import public-key-file=id_ed25519.pub user=%s", cleanPubKey, user)
		_, _ = sess.CombinedOutput(cmd)
	default:
		cmd := fmt.Sprintf("mkdir -p ~/.ssh && chmod 700 ~/.ssh && (grep -q -F %q ~/.ssh/authorized_keys 2>/dev/null || echo %q >> ~/.ssh/authorized_keys) && chmod 600 ~/.ssh/authorized_keys", cleanPubKey, cleanPubKey)
		_, err = sess.CombinedOutput(cmd)
		if err != nil {
			return err
		}
	}
	return nil
}

func expandHomeTUI(p string) string {
	if strings.HasPrefix(p, "~/") {
		if home, err := os.UserHomeDir(); err == nil {
			return filepath.Join(home, p[2:])
		}
	}
	return p
}
