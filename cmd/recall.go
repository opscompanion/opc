package cmd

import (
	"github.com/spf13/cobra"
)

var recallCmd = &cobra.Command{
	Use:        "recall <query>",
	Short:      "Deprecated alias for search",
	Args:       cobra.MinimumNArgs(1),
	Hidden:     true,
	Deprecated: "use `opc search`",
	RunE:       runSearch,
}

func init() {
	rootCmd.AddCommand(recallCmd)
}
