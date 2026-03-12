package cmd

import (
	"fmt"
	"strings"

	"github.com/opscompanion/opc/internal/capture"
	"github.com/spf13/cobra"
)

var sessionListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all captured sessions in /tmp/opc/sessions",
	RunE:  runSessionList,
}

func init() {
	sessionCmd.AddCommand(sessionListCmd)
}

func runSessionList(cmd *cobra.Command, args []string) error {
	sessions, err := capture.ListSessions()
	if err != nil {
		return fmt.Errorf("listing sessions: %w", err)
	}

	if len(sessions) == 0 {
		fmt.Println("No captured sessions found.")
		fmt.Printf("Sessions are stored in: %s\n", capture.SessionDir(""))
		return nil
	}

	fmt.Printf("# Captured Sessions (%d)\n\n", len(sessions))
	fmt.Printf("%-40s | %s\n", "Session ID", "Checkpoints")
	fmt.Println(strings.Repeat("-", 60))

	for _, sid := range sessions {
		cps, _ := capture.ListCheckpoints(sid)
		fmt.Printf("%-40s | %d\n", sid, len(cps))
	}

	fmt.Printf("\nStorage: %s\n", capture.SessionDir(""))
	return nil
}
