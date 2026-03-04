package cmd

import (
	"github.com/spf13/cobra"
)

var commitCmd = &cobra.Command{
	Use:   "commit",
	Short: "Git commit operations linked to sessions",
}

func init() {
	rootCmd.AddCommand(commitCmd)
}
