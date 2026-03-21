package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/opscompanion/opc/internal/agent"
	"github.com/spf13/cobra"
)

var hooksCmd = &cobra.Command{
	Use:   "hooks",
	Short: "Generate agent hook configuration",
	Long: `Generates hook configuration for the active agent runtime.

For Claude Code (default), writes .claude/settings.local.json with hooks:
  - PreToolUse  → captures every tool invocation before execution
  - PostToolUse → captures every tool result (Bash, Write, Edit, Read, etc.)
  - Stop        → captures session end + creates final checkpoint
  - SessionStart → loads org/team/user context

For Codex, writes .codex/hooks.json with the equivalent event bindings.
For other agents, prints a generic shell snippet.

All hook configs include --agent <name> so opc normalizes behavior
regardless of which agent runtime is calling it.`,
	RunE: runHooks,
}

var hooksDryRun bool

func init() {
	hooksCmd.Flags().BoolVar(&hooksDryRun, "dry-run", false, "print the config without writing it")
	rootCmd.AddCommand(hooksCmd)
}

func runHooks(cmd *cobra.Command, args []string) error {
	binary, err := os.Executable()
	if err != nil {
		binary = "opc"
	}

	ag := ActiveAgent
	// Default to claude when auto-detecting and no agent env is set
	if ag.Name == agent.Unknown || ag.Name == agent.Auto {
		ag = agent.Resolve(string(agent.Claude))
	}

	switch ag.HookFormat {
	case "claude-hooks":
		return generateClaudeHooks(binary, ag)
	case "codex-hooks":
		return generateCodexHooks(binary, ag)
	default:
		return generateGenericHooks(binary, ag)
	}
}

func agentFlagArg(ag agent.Info) string {
	return " --agent " + string(ag.Name)
}

func generateClaudeHooks(binary string, ag agent.Info, apiKey ...string) error {
	af := agentFlagArg(ag)
	settings := map[string]interface{}{
		"hooks": map[string]interface{}{
			"PreToolUse": []map[string]interface{}{
				{
					"hooks": []map[string]string{
						{"type": "command", "command": binary + af + " capture --hook-type PreToolUse"},
					},
				},
			},
			"PostToolUse": []map[string]interface{}{
				{
					"hooks": []map[string]string{
						{"type": "command", "command": binary + af + " capture --hook-type PostToolUse"},
					},
				},
			},
			"SessionStart": []map[string]interface{}{
				{
					"matcher": "startup",
					"hooks": []map[string]string{
						{"type": "command", "command": binary + af + " session start"},
					},
				},
				{
					"matcher": "compact",
					"hooks": []map[string]string{
						{"type": "command", "command": binary + af + " session checkpoint"},
					},
				},
				{
					"matcher": "resume",
					"hooks": []map[string]string{
						{"type": "command", "command": binary + af + " session resume $CLAUDE_SESSION_ID"},
					},
				},
			},
			"Stop": []map[string]interface{}{
				{
					"hooks": []map[string]string{
						{"type": "command", "command": binary + af + " capture --hook-type Stop"},
						{"type": "command", "command": binary + af + " session stop"},
					},
				},
			},
		},
	}

	// Wire up Claude Code's native OTEL pipeline to OpsCompanion
	key := ""
	if len(apiKey) > 0 {
		key = apiKey[0]
	}
	if key != "" && key != "mock-key" {
		settings["env"] = map[string]string{
			"CLAUDE_CODE_ENABLE_TELEMETRY":     "1",
			"OTEL_LOGS_EXPORTER":               "otlp",
			"OTEL_EXPORTER_OTLP_PROTOCOL":      "http/json",
			"OTEL_EXPORTER_OTLP_LOGS_ENDPOINT": "https://otel.opscompanion.ai/v1/logs",
			"OTEL_EXPORTER_OTLP_HEADERS":       "Authorization=Bearer " + key,
		}
	}

	data, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return err
	}

	if hooksDryRun {
		fmt.Println(string(data))
		return nil
	}

	claudeDir := filepath.Join(".", ".claude")
	if err := os.MkdirAll(claudeDir, 0755); err != nil {
		return fmt.Errorf("creating .claude directory: %w", err)
	}

	settingsPath := filepath.Join(claudeDir, "settings.local.json")
	if err := os.WriteFile(settingsPath, data, 0644); err != nil {
		return fmt.Errorf("writing settings: %w", err)
	}

	fmt.Printf("  Hooks written to %s\n\n", settingsPath)
	fmt.Println("  Claude Code will now:")
	fmt.Println("    - PreToolUse   → log before each tool runs")
	fmt.Println("    - PostToolUse  → log after each tool completes")
	fmt.Println("    - SessionStart → load org/team/user context")
	fmt.Println("    - Stop         → final checkpoint + extract memories")
	if key != "" && key != "mock-key" {
		fmt.Println("    - OTEL         → export logs to OpsCompanion")
	}
	return nil
}

func generateCodexHooks(binary string, ag agent.Info, apiKey ...string) error {
	af := agentFlagArg(ag)
	sessionVar := "$CODEX_SESSION_ID"

	hooks := map[string]interface{}{
		"hooks": map[string]interface{}{
			"PreToolUse": []map[string]interface{}{
				{
					"hooks": []map[string]string{
						{"type": "command", "command": binary + af + " capture --hook-type PreToolUse"},
					},
				},
			},
			"PostToolUse": []map[string]interface{}{
				{
					"hooks": []map[string]string{
						{"type": "command", "command": binary + af + " capture --hook-type PostToolUse"},
					},
				},
			},
			"SessionStart": []map[string]interface{}{
				{
					"matcher": "startup",
					"hooks": []map[string]string{
						{"type": "command", "command": binary + af + " session start"},
					},
				},
				{
					"matcher": "compact",
					"hooks": []map[string]string{
						{"type": "command", "command": binary + af + " session checkpoint"},
					},
				},
				{
					"matcher": "resume",
					"hooks": []map[string]string{
						{"type": "command", "command": binary + af + " session resume " + sessionVar},
					},
				},
			},
			"Stop": []map[string]interface{}{
				{
					"hooks": []map[string]string{
						{"type": "command", "command": binary + af + " capture --hook-type Stop"},
						{"type": "command", "command": binary + af + " session stop"},
					},
				},
			},
		},
	}

	data, err := json.MarshalIndent(hooks, "", "  ")
	if err != nil {
		return err
	}

	if hooksDryRun {
		fmt.Println(string(data))
		return nil
	}

	codexDir := filepath.Join(".", ".codex")
	if err := os.MkdirAll(codexDir, 0755); err != nil {
		return fmt.Errorf("creating .codex directory: %w", err)
	}

	hooksPath := filepath.Join(codexDir, "hooks.json")
	if err := os.WriteFile(hooksPath, data, 0644); err != nil {
		return fmt.Errorf("writing hooks: %w", err)
	}

	fmt.Printf("  Hooks written to %s\n", hooksPath)

	// Write OTEL config to ~/.codex/config.toml
	key := ""
	if len(apiKey) > 0 {
		key = apiKey[0]
	}
	if key != "" && key != "mock-key" {
		home, err := os.UserHomeDir()
		if err != nil {
			return fmt.Errorf("cannot determine home directory: %w", err)
		}
		codexHome := filepath.Join(home, ".codex")
		if err := os.MkdirAll(codexHome, 0755); err != nil {
			return fmt.Errorf("creating ~/.codex directory: %w", err)
		}

		configPath := filepath.Join(codexHome, "config.toml")
		otelConfig := fmt.Sprintf(`[otel]
exporter = { otlp-http = {
  endpoint = "https://otel.opscompanion.ai/v1/logs",
  protocol = "http/json",
  headers = { "Authorization" = "Bearer %s" }
}}
`, key)

		if err := os.WriteFile(configPath, []byte(otelConfig), 0600); err != nil {
			return fmt.Errorf("writing config.toml: %w", err)
		}
		fmt.Printf("  OTEL config written to %s\n", configPath)
	}

	fmt.Println()
	fmt.Println("  Codex will now:")
	fmt.Println("    - PreToolUse   → log before each tool runs")
	fmt.Println("    - PostToolUse  → log after each tool completes")
	fmt.Println("    - SessionStart → load org/team/user context")
	fmt.Println("    - Stop         → final checkpoint + extract memories")
	if key != "" && key != "mock-key" {
		fmt.Println("    - OTEL         → export logs to OpsCompanion")
	}
	return nil
}

func generateGenericHooks(binary string, ag agent.Info) error {
	af := agentFlagArg(ag)
	fmt.Printf("# opc hook commands for agent: %s\n", ag.Name)
	fmt.Printf("# Add these to your agent's hook/plugin configuration:\n\n")
	fmt.Printf("# Session lifecycle\n")
	fmt.Printf("on_session_start: %s%s session start\n", binary, af)
	fmt.Printf("on_session_stop:  %s%s session stop\n", binary, af)
	fmt.Printf("\n# Tool capture\n")
	fmt.Printf("on_tool_pre:  %s%s capture --hook-type PreToolUse\n", binary, af)
	fmt.Printf("on_tool_post: %s%s capture --hook-type PostToolUse\n", binary, af)
	fmt.Println()
	fmt.Println("Pipe tool invocation JSON to stdin for capture commands.")
	return nil
}
