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

func generateClaudeHooks(binary string, ag agent.Info) error {
	af := agentFlagArg(ag)
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

	data, err := json.MarshalIndent(hooks, "", "  ")
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

	fmt.Printf("Hooks written to %s (agent: %s)\n\n", settingsPath, ag.Name)
	fmt.Println("Claude Code will now capture every action:")
	fmt.Println("  - PreToolUse   → log before each tool runs")
	fmt.Println("  - PostToolUse  → log after each tool completes")
	fmt.Println("  - SessionStart → load org/team/user context")
	fmt.Println("  - Stop         → final checkpoint + extract memories")
	fmt.Println()
	fmt.Println("Restart your Claude Code session for hooks to take effect.")
	return nil
}

func generateCodexHooks(binary string, ag agent.Info) error {
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

	fmt.Printf("Hooks written to %s (agent: %s)\n\n", hooksPath, ag.Name)
	fmt.Println("Codex will now capture every action:")
	fmt.Println("  - PreToolUse   → log before each tool runs")
	fmt.Println("  - PostToolUse  → log after each tool completes")
	fmt.Println("  - SessionStart → load org/team/user context")
	fmt.Println("  - Stop         → final checkpoint + extract memories")
	fmt.Println()
	fmt.Println("Restart your Codex session for hooks to take effect.")
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
