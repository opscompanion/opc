package config

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/opscompanion/opc/internal/models"
)

func TestDeleteRemovesConfigFile(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	if err := Save(&models.Config{APIKey: "test-key", APIURL: DefaultAPIURL}); err != nil {
		t.Fatalf("Save: %v", err)
	}

	path, err := Delete()
	if err != nil {
		t.Fatalf("Delete: %v", err)
	}

	if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("config still exists after delete, err = %v", err)
	}
}

func TestDeleteMissingConfigIsAllowed(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	path, err := Delete()
	if err != nil {
		t.Fatalf("Delete: %v", err)
	}

	want := filepath.Join(home, configDir, configFile)
	if path != want {
		t.Fatalf("Delete path = %q, want %q", path, want)
	}
}
