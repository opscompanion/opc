package cmd

import (
	"fmt"
	"strings"

	"github.com/opscompanion/opc/internal/api"
	"github.com/opscompanion/opc/internal/config"
	"github.com/spf13/cobra"
)

var rememberTags []string

var rememberCmd = &cobra.Command{
	Use:   "remember <text>",
	Short: "Save a decision or discovery to the org knowledge base",
	Args:  cobra.MinimumNArgs(1),
	RunE:  runRemember,
}

func init() {
	rememberCmd.Flags().StringSliceVarP(&rememberTags, "tags", "t", nil, "tags for the memory (comma-separated)")
	rootCmd.AddCommand(rememberCmd)
}

func runRemember(cmd *cobra.Command, args []string) error {
	cfg, err := config.RequireConfig()
	if err != nil {
		return err
	}

	content := strings.Join(args, " ")
	client := api.New(cfg)
	mem, err := client.SaveMemory(content, rememberTags)
	if err != nil {
		return fmt.Errorf("saving memory: %w", err)
	}

	fmt.Println("Memory saved.")
	fmt.Printf("  ID:      %s\n", mem.ID)
	fmt.Printf("  User:    %s\n", mem.User)
	fmt.Printf("  Tags:    %s\n", strings.Join(mem.Tags, ", "))
	fmt.Printf("  Saved:   %s\n", mem.CreatedAt)
	fmt.Printf("  Content: %s\n", mem.Content)
	return nil
}
