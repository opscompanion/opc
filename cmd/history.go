package cmd

import (
	"fmt"
	"strings"

	"github.com/opscompanion/opc/internal/api"
	"github.com/opscompanion/opc/internal/config"
	"github.com/spf13/cobra"
)

var historyCmd = &cobra.Command{
	Use:   "history",
	Short: "View session history with linked events and decisions",
	RunE:  runHistory,
}

func init() {
	rootCmd.AddCommand(historyCmd)
}

func runHistory(cmd *cobra.Command, args []string) error {
	cfg, err := config.RequireConfig()
	if err != nil {
		return err
	}

	client := api.New(cfg)
	entries, err := client.GetHistory()
	if err != nil {
		return fmt.Errorf("fetching history: %w", err)
	}

	fmt.Println("# Session History")
	fmt.Println()
	fmt.Printf("%-12s | %-45s | %-12s | %s\n", "Trigger", "Summary", "Agent", "Session")
	fmt.Println(strings.Repeat("-", 100))

	for _, e := range entries {
		fmt.Printf("%-12s | %-45s | %-12s | %s\n",
			e.Trigger,
			truncate(e.Summary, 45),
			e.Agent,
			e.SessionID,
		)
	}

	// Decisions summary
	fmt.Println()
	fmt.Println("## Recent Decisions")
	fmt.Println()
	for _, e := range entries {
		if len(e.Decisions) > 0 {
			fmt.Printf("**%s** (%s):\n", e.SessionID, e.Summary)
			for _, d := range e.Decisions {
				fmt.Printf("  - %s\n", d)
			}
		}
	}
	return nil
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max-3] + "..."
}
