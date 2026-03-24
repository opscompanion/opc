package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var sessionCheckpointCmd = &cobra.Command{
	Use:        "checkpoint [session-id]",
	Short:      "Deprecated session checkpoint command",
	Hidden:     true,
	Deprecated: "session checkpoint is no longer supported by the public API",
	RunE: func(cmd *cobra.Command, args []string) error {
		return fmt.Errorf("session checkpoint is no longer supported by the public API")
	},
}

func init() {
	sessionCmd.AddCommand(sessionCheckpointCmd)
}
