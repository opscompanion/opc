package cmd

import (
	"github.com/spf13/cobra"
)

var sessionCmd = &cobra.Command{
	Use:   "session",
	Short: "Manage agent sessions (start, stop, resume, checkpoint)",
}

func init() {
	rootCmd.AddCommand(sessionCmd)
}
