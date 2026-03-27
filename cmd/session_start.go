package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var sessionStartCmd = &cobra.Command{
	Use:        "start",
	Short:      "Deprecated session start command",
	Hidden:     true,
	Deprecated: "session start is no longer supported by the public API",
	RunE: func(cmd *cobra.Command, args []string) error {
		return fmt.Errorf("session start is no longer supported by the public API")
	},
}

func init() {
	sessionCmd.AddCommand(sessionStartCmd)
}
