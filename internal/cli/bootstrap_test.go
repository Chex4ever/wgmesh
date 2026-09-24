package cli

import (
	"os"
	"strings"
	"testing"
)

func TestEnsureDefaultSSHKey(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv("HOME", tempDir)
	t.Setenv("USERPROFILE", tempDir)

	kPath, pubStr, err := ensureDefaultSSHKey()
	if err != nil {
		t.Fatalf("ensureDefaultSSHKey вернул ошибку: %v", err)
	}

	if !strings.HasPrefix(pubStr, "ssh-ed25519 ") {
		t.Fatalf("некорректный публичный SSH ключ: %s", pubStr)
	}

	if _, err := os.Stat(kPath); err != nil {
		t.Fatalf("файл приватного ключа не найден: %v", err)
	}

	pubPath := kPath + ".pub"
	if _, err := os.Stat(pubPath); err != nil {
		t.Fatalf("файл публичного ключа не найден: %v", err)
	}

	// Повторный вызов должен переиспользовать уже созданный ключ
	kPath2, pubStr2, err2 := ensureDefaultSSHKey()
	if err2 != nil {
		t.Fatalf("повторный вызов вернул ошибку: %v", err2)
	}
	if pubStr != pubStr2 || kPath != kPath2 {
		t.Fatalf("повторный вызов не сгенерировал тот же самый ключ: %s vs %s", pubStr, pubStr2)
	}
}
