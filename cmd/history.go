package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var historyCmd = &cobra.Command{
	Use:        "history",
	Short:      "Deprecated history command",
	Hidden:     true,
	Deprecated: "history is not available from the public API",
	RunE: func(cmd *cobra.Command, args []string) error {
		return fmt.Errorf("history is not available from the public API")
	},
}

func init() {
	rootCmd.AddCommand(historyCmd)
}
