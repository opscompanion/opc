package cmd

import (
	"fmt"
	"strings"

	"github.com/opscompanion/opc/internal/api"
	"github.com/opscompanion/opc/internal/config"
	"github.com/spf13/cobra"
)

var recallCmd = &cobra.Command{
	Use:   "recall <query>",
	Short: "Search stored memories by keyword or semantic similarity",
	Args:  cobra.MinimumNArgs(1),
	RunE:  runRecall,
}

func init() {
	rootCmd.AddCommand(recallCmd)
}

func runRecall(cmd *cobra.Command, args []string) error {
	cfg, err := config.RequireConfig()
	if err != nil {
		return err
	}

	query := strings.Join(args, " ")
	client := api.New(cfg)
	result, err := client.SearchMemories(query)
	if err != nil {
		return fmt.Errorf("searching memories: %w", err)
	}

	fmt.Printf("# Memory Search Results\n\n")
	fmt.Printf("**Query**: %s\n", query)
	fmt.Printf("**Matches**: %d (in %dms)\n\n", result.TotalMatches, result.QueryTimeMS)

	if len(result.Results) == 0 {
		fmt.Println("No matching memories found.")
		return nil
	}

	for i, mem := range result.Results {
		fmt.Printf("## Match %d — %s (relevance: %.2f)\n\n", i+1, mem.Type, mem.Relevance)
		fmt.Printf("> %s\n\n", mem.Content)
		fmt.Printf("- **Tags**: %s\n", strings.Join(mem.Tags, ", "))
		fmt.Printf("- **Author**: %s\n", mem.User)
		fmt.Printf("- **Date**: %s\n", mem.CreatedAt)
		fmt.Println()
	}
	return nil
}
