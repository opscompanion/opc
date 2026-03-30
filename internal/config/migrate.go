package config

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"

	"github.com/BurntSushi/toml"
	"github.com/opscompanion/opc/internal/models"
)

// migrateJSONToTOML reads a legacy config.json, writes config.toml, and
// renames the old file to config.json.bak. Returns the parsed TOMLConfig.
func migrateJSONToTOML(jsonPath, tomlPath string) (*TOMLConfig, error) {
	data, err := os.ReadFile(jsonPath)
	if err != nil {
		return nil, fmt.Errorf("reading legacy config: %w", err)
	}

	var cfg models.Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parsing legacy config: %w", err)
	}

	tc := &TOMLConfig{
		Default: ProfileEntry{
			APIURL: cfg.APIURL,
			APIKey: cfg.APIKey,
		},
	}

	var buf bytes.Buffer
	if err := toml.NewEncoder(&buf).Encode(tc); err != nil {
		return nil, fmt.Errorf("encoding TOML config: %w", err)
	}

	if err := os.WriteFile(tomlPath, buf.Bytes(), 0600); err != nil {
		return nil, fmt.Errorf("writing TOML config: %w", err)
	}

	// Preserve the old file as a backup.
	if err := os.Rename(jsonPath, jsonPath+".bak"); err != nil {
		// Non-fatal: the TOML file was written successfully.
		fmt.Fprintf(os.Stderr, "warning: could not rename %s to .bak: %v\n", jsonPath, err)
	}

	return tc, nil
}
