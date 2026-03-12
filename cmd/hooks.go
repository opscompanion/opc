package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
)

var hooksCmd = &cobra.Command{
	Use:   "hooks",
	Short: "Generate Claude Code hook configuration",
	Long: `Generates a .claude/settings.local.json that wires opc into
Claude Code's hook system to capture every action:

  - PreToolUse  → captures every tool invocation before execution
  - PostToolUse → captures every tool result (Bash, Write, Edit, Read, etc.)
  - Stop        → captures session end + creates final checkpoint
  - SessionStart → loads org/team/user context

This enables full session replay — every file edit, every shell command,
every search, every action is logged to /tmp/opc/sessions/.`,
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

	hooks := map[string]interface{}{
		"hooks": map[string]interface{}{
			"PreToolUse": []map[string]interface{}{
				{
					"hooks": []map[string]string{
						{"type": "command", "command": binary + " capture --hook-type PreToolUse"},
					},
				},
			},
			"PostToolUse": []map[string]interface{}{
				{
					"hooks": []map[string]string{
						{"type": "command", "command": binary + " capture --hook-type PostToolUse"},
					},
				},
			},
			"SessionStart": []map[string]interface{}{
				{
					"matcher": "startup",
					"hooks": []map[string]string{
						{"type": "command", "command": binary + " session start"},
					},
				},
				{
					"matcher": "compact",
					"hooks": []map[string]string{
						{"type": "command", "command": binary + " session checkpoint"},
					},
				},
				{
					"matcher": "resume",
					"hooks": []map[string]string{
						{"type": "command", "command": binary + " session resume $CLAUDE_SESSION_ID"},
					},
				},
			},
			"Stop": []map[string]interface{}{
				{
					"hooks": []map[string]string{
						{"type": "command", "command": binary + " capture --hook-type Stop"},
						{"type": "command", "command": binary + " session stop"},
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

	fmt.Printf("Hooks written to %s\n\n", settingsPath)
	fmt.Println("Claude Code will now capture every action:")
	fmt.Println("  - PreToolUse   → log before each tool runs")
	fmt.Println("  - PostToolUse  → log after each tool completes")
	fmt.Println("  - SessionStart → load org/team/user context")
	fmt.Println("  - Stop         → final checkpoint + extract memories")
	fmt.Println()
	fmt.Println("Restart your Claude Code session for hooks to take effect.")
	return nil
}
