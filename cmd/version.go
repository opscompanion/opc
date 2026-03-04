package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var (
	Version = "dev"
	Commit  = "none"
	Date    = "unknown"
)

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print the version of opsctl",
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Printf("opsctl %s (commit: %s, built: %s)\n", Version, Commit, Date)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)
}
