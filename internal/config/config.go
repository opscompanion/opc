package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/opscompanion/opsctl/internal/models"
)

const (
	configDir  = ".config/opscompanion"
	configFile = "config.json"
)

// Path returns the full path to the config file.
func Path() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("cannot determine home directory: %w", err)
	}
	return filepath.Join(home, configDir, configFile), nil
}

// Load reads the config from disk. Returns nil config and no error if the
// file does not exist (unconfigured state).
func Load() (*models.Config, error) {
	p, err := Path()
	if err != nil {
		return nil, err
	}

	data, err := os.ReadFile(p)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, fmt.Errorf("reading config: %w", err)
	}

	var cfg models.Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parsing config: %w", err)
	}
	return &cfg, nil
}

// Save writes the config to disk with restrictive permissions.
func Save(cfg *models.Config) error {
	p, err := Path()
	if err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(p), 0700); err != nil {
		return fmt.Errorf("creating config directory: %w", err)
	}

	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling config: %w", err)
	}

	if err := os.WriteFile(p, data, 0600); err != nil {
		return fmt.Errorf("writing config: %w", err)
	}
	return nil
}

// IsMock returns true if mock mode is active.
func IsMock(cfg *models.Config) bool {
	if cfg == nil {
		return false
	}
	return cfg.APIKey == "mock-key" || os.Getenv("OPSCOMPANION_MOCK") == "true"
}

// RequireConfig loads config and returns an error if not configured.
// Used by commands that need a valid config to operate.
func RequireConfig() (*models.Config, error) {
	cfg, err := Load()
	if err != nil {
		return nil, err
	}
	if cfg == nil {
		return nil, fmt.Errorf("opsctl is not configured — run `opsctl init` first")
	}
	return cfg, nil
}
