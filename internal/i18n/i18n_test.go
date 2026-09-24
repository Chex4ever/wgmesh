package i18n

import (
	"os"
	"testing"
)

func TestI18n(t *testing.T) {
	// Test default language
	SetLanguage("en")
	if curr := CurrentLanguage(); curr != "en" {
		t.Errorf("expected current language 'en', got '%s'", curr)
	}

	title := T("topology_title", "MyMesh", "v0.1")
	expectedEn := "=== Network Topology: MyMesh (version v0.1) ==="
	if title != expectedEn {
		t.Errorf("expected %q, got %q", expectedEn, title)
	}

	// Switch to Russian
	SetLanguage("ru")
	if curr := CurrentLanguage(); curr != "ru" {
		t.Errorf("expected current language 'ru', got '%s'", curr)
	}

	titleRu := T("topology_title", "MyMesh", "v0.1")
	expectedRu := "=== Топология сети: MyMesh (версия v0.1) ==="
	if titleRu != expectedRu {
		t.Errorf("expected %q, got %q", expectedRu, titleRu)
	}

	// Fallback to key if missing
	missing := T("non_existing_key_xyz")
	if missing != "non_existing_key_xyz" {
		t.Errorf("expected fallback to key name, got %q", missing)
	}
}

func TestDetectOSLanguage(t *testing.T) {
	origLang := os.Getenv("LANG")
	defer os.Setenv("LANG", origLang)

	os.Setenv("LANG", "ru_RU.UTF-8")
	if lang := DetectOSLanguage(); lang != "ru" {
		t.Errorf("expected 'ru' for LANG=ru_RU.UTF-8, got %q", lang)
	}

	os.Setenv("LANG", "en_US.UTF-8")
	if lang := DetectOSLanguage(); lang != "en" {
		t.Errorf("expected 'en' for LANG=en_US.UTF-8, got %q", lang)
	}
}
