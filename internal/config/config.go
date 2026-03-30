package config

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/BurntSushi/toml"
	"github.com/opscompanion/opc/internal/models"
)

const (
	configDir      = ".config/opscompanion"
	configFileJSON = "config.json"
	configFileTOML = "config.toml"
	DefaultAPIURL  = "https://api.opscompanion.ai/v1"
)

// Path returns the full path to the TOML config file.
func Path() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("cannot determine home directory: %w", err)
	}
	return filepath.Join(home, configDir, configFileTOML), nil
}

// LegacyPath returns the full path to the old JSON config file.
func LegacyPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("cannot determine home directory: %w", err)
	}
	return filepath.Join(home, configDir, configFileJSON), nil
}

// LoadFull reads the TOML config from disk, auto-migrating from JSON if
// needed. Returns nil if unconfigured.
func LoadFull() (*TOMLConfig, error) {
	tomlPath, err := Path()
	if err != nil {
		return nil, err
	}

	// Try TOML first.
	if _, err := os.Stat(tomlPath); err == nil {
		var tc TOMLConfig
		if _, err := toml.DecodeFile(tomlPath, &tc); err != nil {
			return nil, fmt.Errorf("parsing config: %w", err)
		}
		return &tc, nil
	}

	// Fall back to legacy JSON and auto-migrate.
	jsonPath, err := LegacyPath()
	if err != nil {
		return nil, err
	}
	if _, err := os.Stat(jsonPath); err == nil {
		return migrateJSONToTOML(jsonPath, tomlPath)
	}

	return nil, nil
}

// Load reads the config and resolves the active profile, returning the
// same *models.Config that all downstream consumers expect.
func Load() (*models.Config, error) {
	tc, err := LoadFull()
	if err != nil {
		return nil, err
	}
	if tc == nil {
		return nil, nil
	}

	profileName := ResolveProfileName(tc)
	return resolveProfile(tc, profileName), nil
}

// Save writes a config to the [default] section.
func Save(cfg *models.Config) error {
	return SaveProfile(cfg, "")
}

// SaveProfile writes a config to a named profile (or [default] if name is
// empty). Preserves all other profiles in the file.
func SaveProfile(cfg *models.Config, profileName string) error {
	tomlPath, err := Path()
	if err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(tomlPath), 0700); err != nil {
		return fmt.Errorf("creating config directory: %w", err)
	}

	// Load existing TOML to preserve other profiles.
	var tc TOMLConfig
	if _, statErr := os.Stat(tomlPath); statErr == nil {
		if _, err := toml.DecodeFile(tomlPath, &tc); err != nil {
			return fmt.Errorf("parsing existing config: %w", err)
		}
	}

	entry := ProfileEntry{
		APIURL: cfg.APIURL,
		APIKey: cfg.APIKey,
	}

	if profileName == "" {
		tc.Default = entry
	} else {
		if tc.Profiles == nil {
			tc.Profiles = make(map[string]ProfileEntry)
		}
		// Preserve existing paths if present.
		if existing, ok := tc.Profiles[profileName]; ok {
			entry.Paths = existing.Paths
		}
		tc.Profiles[profileName] = entry
	}

	var buf bytes.Buffer
	if err := toml.NewEncoder(&buf).Encode(tc); err != nil {
		return fmt.Errorf("encoding config: %w", err)
	}

	if err := os.WriteFile(tomlPath, buf.Bytes(), 0600); err != nil {
		return fmt.Errorf("writing config: %w", err)
	}
	return nil
}

// AddProfilePath adds a directory path to a named profile's paths list.
func AddProfilePath(profileName, dirPath string) error {
	tomlPath, err := Path()
	if err != nil {
		return err
	}

	var tc TOMLConfig
	if _, statErr := os.Stat(tomlPath); statErr == nil {
		if _, err := toml.DecodeFile(tomlPath, &tc); err != nil {
			return fmt.Errorf("parsing config: %w", err)
		}
	}

	if tc.Profiles == nil {
		return fmt.Errorf("profile %q not found", profileName)
	}
	p, ok := tc.Profiles[profileName]
	if !ok {
		return fmt.Errorf("profile %q not found", profileName)
	}

	// Normalize path: replace home dir prefix with ~.
	home, _ := os.UserHomeDir()
	normalized := dirPath
	if home != "" && strings.HasPrefix(dirPath, home) {
		normalized = "~" + strings.TrimPrefix(dirPath, home)
	}

	// Don't add duplicates.
	for _, existing := range p.Paths {
		if existing == normalized {
			return nil
		}
	}
	p.Paths = append(p.Paths, normalized)
	tc.Profiles[profileName] = p

	var buf bytes.Buffer
	if err := toml.NewEncoder(&buf).Encode(tc); err != nil {
		return fmt.Errorf("encoding config: %w", err)
	}
	return os.WriteFile(tomlPath, buf.Bytes(), 0600)
}

// IsMock returns true if mock mode is active.
func IsMock(cfg *models.Config) bool {
	if cfg == nil {
		return false
	}
	return cfg.APIKey == "mock-key" || os.Getenv("OPSCOMPANION_MOCK") == "true"
}

// ResolveAPIURL returns the active API URL using env override, config, then default.
func ResolveAPIURL(cfg *models.Config) string {
	if envURL := strings.TrimSpace(os.Getenv("OPSCOMPANION_API_URL")); envURL != "" {
		return strings.TrimRight(envURL, "/")
	}
	if cfg != nil && strings.TrimSpace(cfg.APIURL) != "" {
		return strings.TrimRight(strings.TrimSpace(cfg.APIURL), "/")
	}
	return DefaultAPIURL
}

// RequireConfig loads config and returns an error if not configured.
func RequireConfig() (*models.Config, error) {
	cfg, err := Load()
	if err != nil {
		return nil, err
	}
	if cfg == nil {
		return nil, errors.New("opc is not configured — run `opc init` first")
	}
	return cfg, nil
}
