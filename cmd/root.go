package cmd

import (
	"fmt"
	"os"

	"github.com/opscompanion/opc/internal/update"
	"github.com/spf13/cobra"
)

var cfgFile string

// updateResult receives the async update check result.
var updateResult chan string

var rootCmd = &cobra.Command{
	Use:   "opc",
	Short: "OpsCompanion CLI for platform operations",
	Long: `opc is a CLI companion for platform engineers. It provides persistent
context, memory, and session management for agent-assisted workflows.`,
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		if !shouldCheckUpdate(cmd) {
			return
		}
		updateResult = make(chan string, 1)
		go func() {
			updateResult <- update.Check(Version, false)
		}()
	},
	PersistentPostRun: func(cmd *cobra.Command, args []string) {
		if updateResult == nil {
			return
		}
		select {
		case msg := <-updateResult:
			if msg != "" {
				fmt.Fprintln(os.Stderr, msg)
			}
		default:
		}
	},
}

func shouldCheckUpdate(cmd *cobra.Command) bool {
	if os.Getenv("OPC_NO_UPDATE_CHECK") != "" {
		return false
	}
	name := cmd.Name()
	return name != "version" && name != "completion"
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func init() {
	rootCmd.CompletionOptions.DisableDefaultCmd = true
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is ~/.config/opscompanion/config.json)")
}
