package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var sessionResumeCmd = &cobra.Command{
	Use:        "resume <session-id>",
	Short:      "Deprecated session resume command",
	Hidden:     true,
	Deprecated: "session resume is no longer supported by the public API",
	Args:       cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return fmt.Errorf("session resume is no longer supported by the public API")
	},
}

func init() {
	sessionCmd.AddCommand(sessionResumeCmd)
}
