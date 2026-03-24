package cmd

import (
	"fmt"
	"strings"

	"github.com/opscompanion/opc/internal/api"
	"github.com/opscompanion/opc/internal/config"
	"github.com/opscompanion/opc/internal/models"
	"github.com/spf13/cobra"
)

var (
	searchScope          string
	searchMode           string
	searchLimit          int
	searchIncludeContent bool
	searchCaseSensitive  bool
)

var searchCmd = &cobra.Command{
	Use:   "search <query>",
	Short: "Search stored organization knowledge or user memory",
	Args:  cobra.MinimumNArgs(1),
	RunE:  runSearch,
}

func init() {
	searchCmd.Flags().StringVar(&searchScope, "scope", "organization", "search scope: organization, user, or both")
	searchCmd.Flags().StringVar(&searchMode, "mode", "keyword", "search mode: keyword or regex")
	searchCmd.Flags().IntVar(&searchLimit, "limit", 10, "maximum number of results")
	searchCmd.Flags().BoolVar(&searchIncludeContent, "include-content", false, "include full matched content")
	searchCmd.Flags().BoolVar(&searchCaseSensitive, "case-sensitive", false, "enable case-sensitive search")
	rootCmd.AddCommand(searchCmd)
}

func runSearch(cmd *cobra.Command, args []string) error {
	cfg, err := config.RequireConfig()
	if err != nil {
		return err
	}

	switch searchScope {
	case "organization", "user", "both":
	default:
		return fmt.Errorf("invalid --scope %q (expected organization, user, or both)", searchScope)
	}
	switch searchMode {
	case "keyword", "regex":
	default:
		return fmt.Errorf("invalid --mode %q (expected keyword or regex)", searchMode)
	}

	query := strings.Join(args, " ")
	client := api.New(cfg)

	var result *models.SearchResult
	switch searchScope {
	case "user":
		result, err = client.SearchMemory(models.MemorySearchRequest{
			Query:          query,
			Mode:           searchMode,
			Limit:          searchLimit,
			CaseSensitive:  searchCaseSensitive,
			IncludeContent: searchIncludeContent,
		})
	default:
		result, err = client.SearchKnowledge(models.KnowledgeSearchRequest{
			Scope:          searchScope,
			Query:          query,
			Mode:           searchMode,
			Limit:          searchLimit,
			CaseSensitive:  searchCaseSensitive,
			IncludeContent: searchIncludeContent,
		})
	}
	if err != nil {
		return fmt.Errorf("searching: %w", err)
	}

	fmt.Printf("# Search Results\n\n")
	fmt.Printf("**Query**: %s\n", query)
	fmt.Printf("**Scope**: %s\n", searchScope)
	fmt.Printf("**Matches**: %d\n", len(result.Results))
	if result.Truncated {
		fmt.Printf("**Note**: results were truncated by the API\n")
	}
	fmt.Println()

	if len(result.Results) == 0 {
		fmt.Println("No matching results found.")
		return nil
	}

	for i, item := range result.Results {
		fmt.Printf("## Match %d — %s/%s\n\n", i+1, item.SourceType, item.Scope)
		if item.Name != "" {
			fmt.Printf("- **Name**: %s\n", item.Name)
		}
		fmt.Printf("- **Path**: %s\n", item.Path)
		if item.Date != nil {
			fmt.Printf("- **Date**: %s\n", *item.Date)
		}
		fmt.Printf("- **Match count**: %d\n", item.MatchCount)
		fmt.Printf("- **Snippet**: %s\n", item.Snippet)
		if searchIncludeContent && strings.TrimSpace(item.Content) != "" {
			fmt.Printf("\n%s\n", item.Content)
		}
		fmt.Println()
	}

	return nil
}
