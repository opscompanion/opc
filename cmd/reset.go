package cmd

import (
	"fmt"

	"github.com/opscompanion/opc/internal/config"
	"github.com/spf13/cobra"
)

var resetCmd = &cobra.Command{
	Use:   "reset",
	Short: "Clear saved OpsCompanion config",
	Long:  "Deletes the saved OpsCompanion config file after an explicit confirmation prompt.",
	RunE:  runReset,
}

func init() {
	rootCmd.AddCommand(resetCmd)
}

func runReset(cmd *cobra.Command, args []string) error {
	existing, err := config.Load()
	if err != nil {
		return err
	}

	path, err := config.Path()
	if err != nil {
		return err
	}

	if existing == nil {
		fmt.Printf("No saved config found at %s\n", path)
		return nil
	}

	fmt.Printf("This will delete the saved config at %s\n", path)
	confirmed, err := promptConfirmAction("confirm", "Are you sure you want to permanently delete the saved config? This cannot be undone.", false)
	if err != nil {
		return err
	}
	if !confirmed {
		fmt.Println("Aborted.")
		return nil
	}

	deletedPath, err := config.Delete()
	if err != nil {
		return err
	}

	fmt.Printf("Deleted config: %s\n", deletedPath)
	fmt.Println("Run `opc setup` to configure OpsCompanion again.")
	return nil
}
