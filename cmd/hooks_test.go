package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/opscompanion/opc/internal/agent"
	"github.com/opscompanion/opc/internal/config"
)

func TestWriteClaudeHooksUsesDevOTELEndpointForDevAPIURL(t *testing.T) {
	tmp := t.TempDir()
	prev, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd: %v", err)
	}
	defer func() {
		_ = os.Chdir(prev)
	}()
	if err := os.Chdir(tmp); err != nil {
		t.Fatalf("Chdir(%q): %v", tmp, err)
	}

	_, _, err = writeClaudeHooks("opc", agent.Resolve(string(agent.Claude)), true, "test-key", config.DevAPIURL)
	if err != nil {
		t.Fatalf("writeClaudeHooks() error = %v", err)
	}

	data, err := os.ReadFile(filepath.Join(tmp, ".claude", "settings.local.json"))
	if err != nil {
		t.Fatalf("ReadFile(settings.local.json): %v", err)
	}
	content := string(data)
	if !strings.Contains(content, config.DevOTELURL) {
		t.Fatalf("settings.local.json missing dev OTEL endpoint: %s", content)
	}
	if strings.Contains(content, config.DefaultOTELURL) {
		t.Fatalf("settings.local.json should not contain default OTEL endpoint: %s", content)
	}
}

func TestWriteCodexHooksUsesDevOTELEndpointForDevAPIURL(t *testing.T) {
	tmp := t.TempDir()
	prev, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd: %v", err)
	}
	defer func() {
		_ = os.Chdir(prev)
	}()
	if err := os.Chdir(tmp); err != nil {
		t.Fatalf("Chdir(%q): %v", tmp, err)
	}
	t.Setenv("HOME", tmp)

	_, configPath, err := writeCodexHooks("opc", agent.Resolve(string(agent.Codex)), true, "test-key", config.DevAPIURL)
	if err != nil {
		t.Fatalf("writeCodexHooks() error = %v", err)
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("ReadFile(%q): %v", configPath, err)
	}
	content := string(data)
	if !strings.Contains(content, `environment = "dev"`) {
		t.Fatalf("config.toml missing dev environment: %s", content)
	}
	if !strings.Contains(content, config.DevOTELURL) {
		t.Fatalf("config.toml missing dev OTEL endpoint: %s", content)
	}
	if strings.Contains(content, config.DefaultOTELURL) {
		t.Fatalf("config.toml should not contain default OTEL endpoint: %s", content)
	}
}

func TestWriteCodexHooksUsesProdOTELEndpointForDefaultAPIURL(t *testing.T) {
	tmp := t.TempDir()
	prev, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd: %v", err)
	}
	defer func() {
		_ = os.Chdir(prev)
	}()
	if err := os.Chdir(tmp); err != nil {
		t.Fatalf("Chdir(%q): %v", tmp, err)
	}
	t.Setenv("HOME", tmp)

	_, configPath, err := writeCodexHooks("opc", agent.Resolve(string(agent.Codex)), true, "test-key", config.DefaultAPIURL)
	if err != nil {
		t.Fatalf("writeCodexHooks() error = %v", err)
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("ReadFile(%q): %v", configPath, err)
	}
	content := string(data)
	if !strings.Contains(content, `environment = "prod"`) {
		t.Fatalf("config.toml missing prod environment: %s", content)
	}
	if !strings.Contains(content, config.DefaultOTELURL) {
		t.Fatalf("config.toml missing default OTEL endpoint: %s", content)
	}
	if strings.Contains(content, config.DevOTELURL) {
		t.Fatalf("config.toml should not contain dev OTEL endpoint: %s", content)
	}
}
