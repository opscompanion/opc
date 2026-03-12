package cmd

import (
	"fmt"
	"strings"

	"github.com/opscompanion/opc/internal/api"
	"github.com/opscompanion/opc/internal/config"
	"github.com/spf13/cobra"
)

var sessionResumeCmd = &cobra.Command{
	Use:   "resume <session-id>",
	Short: "Resume a previous session and reload context",
	Args:  cobra.ExactArgs(1),
	RunE:  runSessionResume,
}

func init() {
	sessionCmd.AddCommand(sessionResumeCmd)
}

func runSessionResume(cmd *cobra.Command, args []string) error {
	cfg, err := config.RequireConfig()
	if err != nil {
		return err
	}

	sessionID := args[0]
	client := api.New(cfg)
	ctx, err := client.ResumeSession(sessionID)
	if err != nil {
		return fmt.Errorf("resuming session: %w", err)
	}

	fmt.Printf("# Session Resumed: `%s`\n\n", sessionID)
	fmt.Printf("- **Last active**: %s\n", ctx.LastActive)
	fmt.Printf("- **Working on**: %s\n", ctx.WorkingOn)
	fmt.Printf("- **Branch**: %s\n", ctx.Branch)
	fmt.Printf("- **Modified files**: %s\n", strings.Join(ctx.ModifiedFiles, ", "))

	fmt.Println("\n## Previous Decisions\n")
	for _, d := range ctx.Decisions {
		fmt.Printf("- %s\n", d)
	}

	fmt.Println("\n## Open Threads\n")
	for _, t := range ctx.OpenThreads {
		fmt.Printf("- %s\n", t)
	}
	return nil
}
