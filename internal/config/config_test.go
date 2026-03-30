package config

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/BurntSushi/toml"
	"github.com/opscompanion/opc/internal/models"
)

func TestLoadTOML(t *testing.T) {
	dir := t.TempDir()
	tomlPath := filepath.Join(dir, configFileTOML)

	content := `
[default]
api_url = "https://api.example.com/v1"
api_key = "key-default"
`
	if err := os.WriteFile(tomlPath, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}

	tc, err := loadFullFrom(tomlPath, "")
	if err != nil {
		t.Fatal(err)
	}
	if tc.Default.APIKey != "key-default" {
		t.Errorf("got api_key %q, want %q", tc.Default.APIKey, "key-default")
	}
	if tc.Default.APIURL != "https://api.example.com/v1" {
		t.Errorf("got api_url %q, want %q", tc.Default.APIURL, "https://api.example.com/v1")
	}
}

func TestLoadTOMLWithProfiles(t *testing.T) {
	dir := t.TempDir()
	tomlPath := filepath.Join(dir, configFileTOML)

	content := `
[default]
api_url = "https://default.example.com/v1"
api_key = "key-default"

[profile.acme]
api_url = "https://acme.example.com/v1"
api_key = "key-acme"
paths = ["~/code/acme-*"]

[profile.personal]
api_key = "key-personal"
`
	if err := os.WriteFile(tomlPath, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}

	tc, err := loadFullFrom(tomlPath, "")
	if err != nil {
		t.Fatal(err)
	}

	if len(tc.Profiles) != 2 {
		t.Fatalf("got %d profiles, want 2", len(tc.Profiles))
	}

	acme := tc.Profiles["acme"]
	if acme.APIKey != "key-acme" {
		t.Errorf("acme api_key = %q, want %q", acme.APIKey, "key-acme")
	}
	if len(acme.Paths) != 1 || acme.Paths[0] != "~/code/acme-*" {
		t.Errorf("acme paths = %v, want [~/code/acme-*]", acme.Paths)
	}

	personal := tc.Profiles["personal"]
	if personal.APIKey != "key-personal" {
		t.Errorf("personal api_key = %q, want %q", personal.APIKey, "key-personal")
	}
}

func TestResolveProfileDefault(t *testing.T) {
	tc := &TOMLConfig{
		Default: ProfileEntry{APIURL: "https://default.com", APIKey: "key-d"},
	}
	cfg := resolveProfile(tc, "")
	if cfg.APIKey != "key-d" {
		t.Errorf("got %q, want %q", cfg.APIKey, "key-d")
	}
}

func TestResolveProfileNamed(t *testing.T) {
	tc := &TOMLConfig{
		Default: ProfileEntry{APIURL: "https://default.com", APIKey: "key-d"},
		Profiles: map[string]ProfileEntry{
			"acme": {APIURL: "https://acme.com", APIKey: "key-acme"},
		},
	}
	cfg := resolveProfile(tc, "acme")
	if cfg.APIKey != "key-acme" || cfg.APIURL != "https://acme.com" {
		t.Errorf("got key=%q url=%q, want key-acme / https://acme.com", cfg.APIKey, cfg.APIURL)
	}
}

func TestResolveProfileInheritsURL(t *testing.T) {
	tc := &TOMLConfig{
		Default: ProfileEntry{APIURL: "https://default.com", APIKey: "key-d"},
		Profiles: map[string]ProfileEntry{
			"personal": {APIKey: "key-personal"},
		},
	}
	cfg := resolveProfile(tc, "personal")
	if cfg.APIURL != "https://default.com" {
		t.Errorf("got %q, want inherited %q", cfg.APIURL, "https://default.com")
	}
}

func TestResolveProfileUnknownFallsToDefault(t *testing.T) {
	tc := &TOMLConfig{
		Default: ProfileEntry{APIURL: "https://default.com", APIKey: "key-d"},
	}
	cfg := resolveProfile(tc, "nonexistent")
	if cfg.APIKey != "key-d" {
		t.Errorf("got %q, want %q", cfg.APIKey, "key-d")
	}
}

func TestMigrateJSONToTOML(t *testing.T) {
	dir := t.TempDir()
	jsonPath := filepath.Join(dir, configFileJSON)
	tomlPath := filepath.Join(dir, configFileTOML)

	oldCfg := models.Config{APIURL: "https://old.com/v1", APIKey: "old-key"}
	data, _ := json.Marshal(oldCfg)
	if err := os.WriteFile(jsonPath, data, 0600); err != nil {
		t.Fatal(err)
	}

	tc, err := migrateJSONToTOML(jsonPath, tomlPath)
	if err != nil {
		t.Fatal(err)
	}

	if tc.Default.APIKey != "old-key" {
		t.Errorf("migrated key = %q, want %q", tc.Default.APIKey, "old-key")
	}
	if tc.Default.APIURL != "https://old.com/v1" {
		t.Errorf("migrated url = %q, want %q", tc.Default.APIURL, "https://old.com/v1")
	}

	// TOML file should exist.
	if _, err := os.Stat(tomlPath); err != nil {
		t.Error("TOML file not created")
	}
	// JSON should be renamed to .bak.
	if _, err := os.Stat(jsonPath + ".bak"); err != nil {
		t.Error("JSON backup not created")
	}
	if _, err := os.Stat(jsonPath); !os.IsNotExist(err) {
		t.Error("original JSON file should have been renamed")
	}
}

func TestSaveProfile(t *testing.T) {
	dir := t.TempDir()
	tomlPath := filepath.Join(dir, configFileTOML)

	// Write initial default.
	cfg := &models.Config{APIURL: "https://default.com", APIKey: "key-d"}
	if err := saveProfileTo(cfg, "", tomlPath); err != nil {
		t.Fatal(err)
	}

	// Write a named profile.
	acmeCfg := &models.Config{APIURL: "https://acme.com", APIKey: "key-acme"}
	if err := saveProfileTo(acmeCfg, "acme", tomlPath); err != nil {
		t.Fatal(err)
	}

	// Reload and verify both exist.
	tc, err := loadFullFrom(tomlPath, "")
	if err != nil {
		t.Fatal(err)
	}
	if tc.Default.APIKey != "key-d" {
		t.Errorf("default key = %q, want %q", tc.Default.APIKey, "key-d")
	}
	if tc.Profiles["acme"].APIKey != "key-acme" {
		t.Errorf("acme key = %q, want %q", tc.Profiles["acme"].APIKey, "key-acme")
	}
}

func TestParseDotfile(t *testing.T) {
	tests := []struct {
		content string
		want    string
	}{
		{"profile = acme-corp", "acme-corp"},
		{"# comment\nprofile = acme-corp\n", "acme-corp"},
		{"profile=acme", "acme"},
		{"", ""},
		{"something = else", ""},
	}
	for _, tt := range tests {
		got := parseDotfile(tt.content)
		if got != tt.want {
			t.Errorf("parseDotfile(%q) = %q, want %q", tt.content, got, tt.want)
		}
	}
}

func TestExpandHome(t *testing.T) {
	got := expandHome("~/code/acme", "/home/user")
	if got != "/home/user/code/acme" {
		t.Errorf("got %q, want %q", got, "/home/user/code/acme")
	}
	got = expandHome("/absolute/path", "/home/user")
	if got != "/absolute/path" {
		t.Errorf("got %q, want %q", got, "/absolute/path")
	}
}

func TestDotfileWalkup(t *testing.T) {
	// Create a nested directory structure with a dotfile at the root.
	dir := t.TempDir()
	nested := filepath.Join(dir, "a", "b", "c")
	if err := os.MkdirAll(nested, 0755); err != nil {
		t.Fatal(err)
	}
	dotfile := filepath.Join(dir, "a", ".opscompanion")
	if err := os.WriteFile(dotfile, []byte("profile = myprofile"), 0644); err != nil {
		t.Fatal(err)
	}

	// Change to the deeply nested directory.
	origDir, _ := os.Getwd()
	defer os.Chdir(origDir)
	os.Chdir(nested)

	name := findDotfile()
	if name != "myprofile" {
		t.Errorf("findDotfile() = %q, want %q", name, "myprofile")
	}
}

// loadFullFrom is a test helper that loads a TOML config from a specific path.
func loadFullFrom(tomlPath, jsonPath string) (*TOMLConfig, error) {
	if _, err := os.Stat(tomlPath); err == nil {
		var tc TOMLConfig
		if _, err := toml.DecodeFile(tomlPath, &tc); err != nil {
			return nil, err
		}
		return &tc, nil
	}
	if jsonPath != "" {
		if _, err := os.Stat(jsonPath); err == nil {
			return migrateJSONToTOML(jsonPath, tomlPath)
		}
	}
	return nil, nil
}

// saveProfileTo is a test helper that saves to a specific TOML path.
func saveProfileTo(cfg *models.Config, profileName, tomlPath string) error {
	var tc TOMLConfig
	if _, err := os.Stat(tomlPath); err == nil {
		if _, err := toml.DecodeFile(tomlPath, &tc); err != nil {
			return err
		}
	}

	entry := ProfileEntry{APIURL: cfg.APIURL, APIKey: cfg.APIKey}
	if profileName == "" {
		tc.Default = entry
	} else {
		if tc.Profiles == nil {
			tc.Profiles = make(map[string]ProfileEntry)
		}
		if existing, ok := tc.Profiles[profileName]; ok {
			entry.Paths = existing.Paths
		}
		tc.Profiles[profileName] = entry
	}

	var buf bytes.Buffer
	if err := toml.NewEncoder(&buf).Encode(tc); err != nil {
		return err
	}
	return os.WriteFile(tomlPath, buf.Bytes(), 0600)
}
