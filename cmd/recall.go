package cmd

import (
	"github.com/spf13/cobra"
)

var recallCmd = &cobra.Command{
	Use:     "search <query>",
	Aliases: []string{"recall"},
	Short:   "Search stored memories by keyword or semantic similarity",
	Args:    cobra.MinimumNArgs(1),
	RunE:    runRecall,
}

func init() {
	rootCmd.AddCommand(recallCmd)
}
