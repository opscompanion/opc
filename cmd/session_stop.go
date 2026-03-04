package cmd

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/opscompanion/opsctl/internal/api"
	"github.com/opscompanion/opsctl/internal/capture"
	"github.com/opscompanion/opsctl/internal/config"
	"github.com/spf13/cobra"
)

var sessionStopCmd = &cobra.Command{
	Use:   "stop [session-id]",
	Short: "Stop a session and extract memories",
	RunE:  runSessionStop,
}

func init() {
	sessionCmd.AddCommand(sessionStopCmd)
}

func runSessionStop(cmd *cobra.Command, args []string) error {
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

	// Log stop event
	capture.AppendEvent(sessionID, capture.Event{
		ID:   fmt.Sprintf("evt_%d_stop", time.Now().UnixNano()),
		Type: "hook",
		Data: map[string]interface{}{
			"hook":    "session-stop",
			"stopped": time.Now().UTC(),
		},
	})

	client := api.New(cfg)
	memories, err := client.StopSession(sessionID)
	if err != nil {
		return fmt.Errorf("stopping session: %w", err)
	}

	fmt.Printf("# Session Stopped: `%s`\n\n", sessionID)
	fmt.Printf("## Auto-Extracted Memories\n\n")
	for _, mem := range memories {
		fmt.Printf("- **%s** [%s]: %s\n", mem.Type, strings.Join(mem.Tags, ", "), mem.Content)
	}
	fmt.Printf("\n%d memories saved to org knowledge base.\n", len(memories))
	return nil
}
