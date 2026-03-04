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
	Short: "Generate Claude Code hook configuration for local testing",
	Long: `Generates a .claude/settings.local.json with hooks that wire opsctl
into your Claude Code session lifecycle:

  - SessionStart (startup): opsctl session start
  - SessionStart (compact): opsctl session checkpoint
  - Stop: opsctl session stop
  - PostToolUse (Bash with git commit): opsctl commit capture

Run this in any project directory to enable session capture.`,
	RunE: runHooks,
}

var hooksDryRun bool

func init() {
	hooksCmd.Flags().BoolVar(&hooksDryRun, "dry-run", false, "print the config without writing it")
	rootCmd.AddCommand(hooksCmd)
}

func runHooks(cmd *cobra.Command, args []string) error {
	// Get absolute path to opsctl binary
	binary, err := os.Executable()
	if err != nil {
		binary = "opsctl"
	}

	hooks := map[string]interface{}{
		"hooks": map[string]interface{}{
			"SessionStart": []map[string]interface{}{
				{
					"matcher":  "startup",
					"hooks": []map[string]string{
						{"type": "command", "command": binary + " session start"},
					},
				},
				{
					"matcher":  "compact",
					"hooks": []map[string]string{
						{"type": "command", "command": binary + " session checkpoint"},
					},
				},
				{
					"matcher":  "resume",
					"hooks": []map[string]string{
						{"type": "command", "command": binary + " session resume $CLAUDE_SESSION_ID"},
					},
				},
			},
			"Stop": []map[string]interface{}{
				{
					"hooks": []map[string]string{
						{"type": "command", "command": binary + " session stop"},
					},
				},
			},
			"PostToolUse": []map[string]interface{}{
				{
					"matcher":  "Bash",
					"hooks": []map[string]string{
						{"type": "command", "command": binary + " commit capture"},
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

	// Write to .claude/settings.local.json in current directory
	claudeDir := filepath.Join(".", ".claude")
	if err := os.MkdirAll(claudeDir, 0755); err != nil {
		return fmt.Errorf("creating .claude directory: %w", err)
	}

	settingsPath := filepath.Join(claudeDir, "settings.local.json")
	if err := os.WriteFile(settingsPath, data, 0644); err != nil {
		return fmt.Errorf("writing settings: %w", err)
	}

	fmt.Printf("Hooks written to %s\n\n", settingsPath)
	fmt.Println("Claude Code will now run opsctl hooks automatically:")
	fmt.Println("  - Session start  → opsctl session start")
	fmt.Println("  - Context compact → opsctl session checkpoint")
	fmt.Println("  - Session resume → opsctl session resume")
	fmt.Println("  - Session stop   → opsctl session stop")
	fmt.Println("  - After git commit → opsctl commit capture")
	fmt.Println()
	fmt.Println("Restart your Claude Code session for hooks to take effect.")
	return nil
}
