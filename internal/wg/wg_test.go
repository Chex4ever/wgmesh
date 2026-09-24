package wg

import (
	"strings"
	"testing"
)

func TestGenerateKeyAndPublicKey(t *testing.T) {
	kp, err := GenerateKey()
	if err != nil {
		t.Fatalf("GenerateKey: %v", err)
	}
	if !ValidKey(kp.PrivateKey) || !ValidKey(kp.PublicKey) {
		t.Fatalf("сгенерированные ключи невалидны: %+v", kp)
	}
	// публичный ключ должен детерминированно вычисляться из приватного
	pub, err := PublicKey(kp.PrivateKey)
	if err != nil {
		t.Fatalf("PublicKey: %v", err)
	}
	if pub != kp.PublicKey {
		t.Fatalf("PublicKey(%s) = %s, ожидалось %s", kp.PrivateKey, pub, kp.PublicKey)
	}
	// две генерации дают разные ключи
	kp2, _ := GenerateKey()
	if kp2.PrivateKey == kp.PrivateKey {
		t.Fatal("два вызова GenerateKey вернули одинаковый ключ")
	}
}

func TestValidKey(t *testing.T) {
	if ValidKey("not-base64!!") || ValidKey("") || ValidKey("c2hvcnQ=") {
		t.Fatal("ValidKey пропускает некорректные ключи")
	}
}

func TestRenderClient(t *testing.T) {
	c := &ClientConfig{
		Name:       "via-kz",
		PrivateKey: "priv",
		Address:    "10.66.0.101/32",
		DNS:        []string{"1.1.1.1"},
		Peer: PeerConf{
			PublicKey:           "pub",
			AllowedIPs:          []string{"0.0.0.0/0", "::/0"},
			Endpoint:            "1.2.3.4:51820",
			PersistentKeepalive: 25,
		},
	}
	out := RenderClient(c)
	for _, want := range []string{"[Interface]", "[Peer]", "PrivateKey = priv", "Address = 10.66.0.101/32",
		"PublicKey = pub", "Endpoint = 1.2.3.4:51820", "PersistentKeepalive = 25", "DNS = 1.1.1.1"} {
		if !strings.Contains(out, want) {
			t.Fatalf("в рендере клиентского конфига нет %q:\n%s", want, out)
		}
	}
}

func TestRenderNode(t *testing.T) {
	n := &NodeConfig{
		Name:       "kz-server",
		PrivateKey: "priv",
		Address:    "10.66.0.10/24",
		ListenPort: 51820,
		Peers: []PeerConf{
			{PublicKey: "peerpub", AllowedIPs: []string{"10.66.0.11/32"}, Endpoint: "5.6.7.8:51821"},
		},
	}
	out := RenderNode(n)
	for _, want := range []string{"[Interface]", "ListenPort = 51820", "Address = 10.66.0.10/24",
		"[Peer]", "PublicKey = peerpub", "AllowedIPs = 10.66.0.11/32", "Endpoint = 5.6.7.8:51821"} {
		if !strings.Contains(out, want) {
			t.Fatalf("в рендере серверного конфига нет %q:\n%s", want, out)
		}
	}
	// пустой endpoint не должен рендериться как "Endpoint = "
	if strings.Contains(out, "Endpoint = \n") {
		t.Fatalf("рендер содержит пустой Endpoint:\n%s", out)
	}
}
