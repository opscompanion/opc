package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var cfgFile string

var rootCmd = &cobra.Command{
	Use:   "opsctl",
	Short: "OpsCompanion CLI for platform operations",
	Long: `opsctl is a CLI tool for managing platform infrastructure, deployments,
and operational workflows.

Commands:
  init              Configure API key, org, and endpoint
  context           Show org/team/user context
  recall <query>    Search stored memories
  remember <text>   Save a decision or discovery
  history           View commit history with linked sessions
  session           Manage agent sessions (start, stop, resume, checkpoint)
  commit capture    Link a git commit to the current session`,
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func init() {
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is ~/.config/opscompanion/config.json)")
}
