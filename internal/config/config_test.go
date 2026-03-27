package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/opscompanion/opc/internal/models"
)

func TestPathUsesHomeDirectory(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	got, err := Path()
	if err != nil {
		t.Fatalf("Path: %v", err)
	}

	want := filepath.Join(home, configDir, configFile)
	if got != want {
		t.Fatalf("Path() = %q, want %q", got, want)
	}
}

func TestLoadMissingConfigReturnsNil(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg != nil {
		t.Fatalf("Load() = %#v, want nil", cfg)
	}
}

func TestSaveAndLoadRoundTrip(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	want := &models.Config{
		APIURL: "https://example.test/v1",
		APIKey: "secret",
	}
	if err := Save(want); err != nil {
		t.Fatalf("Save: %v", err)
	}

	got, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got == nil || *got != *want {
		t.Fatalf("Load() = %#v, want %#v", got, want)
	}

	path, err := Path()
	if err != nil {
		t.Fatalf("Path: %v", err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("Stat: %v", err)
	}
	if perms := info.Mode().Perm(); perms != 0o600 {
		t.Fatalf("config mode = %o, want 600", perms)
	}
}

func TestLoadInvalidJSONReturnsError(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	path, err := Path()
	if err != nil {
		t.Fatalf("Path: %v", err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := os.WriteFile(path, []byte("{"), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	_, err = Load()
	if err == nil || !strings.Contains(err.Error(), "parsing config") {
		t.Fatalf("Load error = %v, want parsing config error", err)
	}
}

func TestRequireConfigErrorsWhenMissing(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	_, err := RequireConfig()
	if err == nil || !strings.Contains(err.Error(), "opc is not configured") {
		t.Fatalf("RequireConfig error = %v, want unconfigured error", err)
	}
}

func TestRequireConfigReturnsLoadedConfig(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	want := &models.Config{APIKey: "k"}
	if err := Save(want); err != nil {
		t.Fatalf("Save: %v", err)
	}

	got, err := RequireConfig()
	if err != nil {
		t.Fatalf("RequireConfig: %v", err)
	}
	if got == nil || got.APIKey != want.APIKey {
		t.Fatalf("RequireConfig() = %#v, want %#v", got, want)
	}
}

func TestIsMock(t *testing.T) {
	t.Setenv("OPSCOMPANION_MOCK", "")
	if IsMock(nil) {
		t.Fatal("IsMock(nil) should be false")
	}
	if IsMock(&models.Config{APIKey: "real"}) {
		t.Fatal("IsMock(real key) should be false")
	}
	if !IsMock(&models.Config{APIKey: "mock-key"}) {
		t.Fatal("IsMock(mock-key) should be true")
	}

	t.Setenv("OPSCOMPANION_MOCK", "true")
	if !IsMock(&models.Config{APIKey: "real"}) {
		t.Fatal("env override should enable mock mode")
	}
}

func TestResolveAPIURLPrecedence(t *testing.T) {
	t.Setenv("OPSCOMPANION_API_URL", "")

	if got := ResolveAPIURL(nil); got != DefaultAPIURL {
		t.Fatalf("ResolveAPIURL(nil) = %q, want %q", got, DefaultAPIURL)
	}

	cfg := &models.Config{APIURL: " https://cfg.example/v1/ "}
	if got := ResolveAPIURL(cfg); got != "https://cfg.example/v1" {
		t.Fatalf("ResolveAPIURL(cfg) = %q", got)
	}

	t.Setenv("OPSCOMPANION_API_URL", " https://env.example/v1/ ")
	if got := ResolveAPIURL(cfg); got != "https://env.example/v1" {
		t.Fatalf("ResolveAPIURL(env) = %q", got)
	}
}
