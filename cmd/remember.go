package cmd

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/opscompanion/opc/internal/api"
	"github.com/opscompanion/opc/internal/config"
	"github.com/opscompanion/opc/internal/models"
	"github.com/spf13/cobra"
)

var rememberTags []string

var rememberCmd = &cobra.Command{
	Use:   "remember <text>",
	Short: "Save a decision or discovery to organization knowledge",
	Args:  cobra.MinimumNArgs(1),
	RunE:  runRemember,
}

func init() {
	rememberCmd.Flags().StringSliceVarP(&rememberTags, "tags", "t", nil, "tags to include in the saved note")
	rootCmd.AddCommand(rememberCmd)
}

func runRemember(cmd *cobra.Command, args []string) error {
	cfg, err := config.RequireConfig()
	if err != nil {
		return err
	}

	client := api.New(cfg)
	content := strings.Join(args, " ")
	path := fmt.Sprintf("opc/remember/%s.md", time.Now().UTC().Format("2006-01"))
	entry := renderRememberEntry(content, rememberTags)

	existingContent := ""
	doc, err := client.GetKnowledgeByPath(path)
	if err != nil {
		var apiErr *api.APIError
		if !errors.As(err, &apiErr) || apiErr.StatusCode != 404 {
			return fmt.Errorf("loading existing knowledge note: %w", err)
		}
	} else {
		existingContent = strings.TrimSpace(doc.Content)
	}

	combined := entry
	if existingContent != "" {
		combined = existingContent + "\n\n" + entry
	}

	saved, err := client.PutKnowledgeByPath(path, models.KnowledgePathUpsertRequest{
		Content: combined,
	})
	if err != nil {
		return fmt.Errorf("saving memory: %w", err)
	}

	fmt.Println("Knowledge entry saved.")
	fmt.Printf("  File: %s/%s\n", saved.File.Path, saved.File.Name)
	fmt.Printf("  ID:   %s\n", saved.File.PublicID)
	if saved.Version != nil {
		fmt.Printf("  Ver:  %s\n", saved.Version.PublicID)
	}
	return nil
}

func renderRememberEntry(content string, tags []string) string {
	timestamp := time.Now().UTC().Format(time.RFC3339)
	lines := []string{
		fmt.Sprintf("## %s", timestamp),
		"",
		content,
	}
	if len(tags) > 0 {
		lines = append(lines, "", "Tags: "+strings.Join(tags, ", "))
	}
	return strings.Join(lines, "\n")
}
