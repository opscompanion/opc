package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var sessionStopCmd = &cobra.Command{
	Use:        "stop [session-id]",
	Short:      "Deprecated session stop command",
	Hidden:     true,
	Deprecated: "session stop is no longer supported by the public API",
	RunE: func(cmd *cobra.Command, args []string) error {
		return fmt.Errorf("session stop is no longer supported by the public API")
	},
}

func init() {
	sessionCmd.AddCommand(sessionStopCmd)
}
