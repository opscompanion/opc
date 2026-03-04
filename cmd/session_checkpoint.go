package cmd

import (
	"fmt"
	"os"

	"github.com/opscompanion/opsctl/internal/api"
	"github.com/opscompanion/opsctl/internal/config"
	"github.com/spf13/cobra"
)

var sessionCheckpointCmd = &cobra.Command{
	Use:   "checkpoint [session-id]",
	Short: "Save a checkpoint of the current session state",
	RunE:  runSessionCheckpoint,
}

func init() {
	sessionCmd.AddCommand(sessionCheckpointCmd)
}

func runSessionCheckpoint(cmd *cobra.Command, args []string) error {
	cfg, err := config.RequireConfig()
	if err != nil {
		return err
	}

	sessionID := os.Getenv("CLAUDE_SESSION_ID")
	if len(args) > 0 {
		sessionID = args[0]
	}
	if sessionID == "" {
		return fmt.Errorf("session ID required (set CLAUDE_SESSION_ID or pass as argument)")
	}

	client := api.New(cfg)
	cp, err := client.Checkpoint(sessionID)
	if err != nil {
		return fmt.Errorf("creating checkpoint: %w", err)
	}

	fmt.Printf("# Checkpoint Saved\n\n")
	fmt.Printf("**Session**: `%s`\n\n", cp.SessionID)
	fmt.Printf("## Compressed Summary\n\n%s\n\n", cp.Compressed)
	fmt.Printf("## Extracted Decisions\n\n")
	for _, d := range cp.Decisions {
		fmt.Printf("- %s\n", d)
	}
	fmt.Printf("\n## Files Modified\n\n")
	for _, f := range cp.FilesModified {
		fmt.Printf("- %s\n", f)
	}
	return nil
}
