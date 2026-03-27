package cmd

import (
	"fmt"
	"sort"
	"strings"

	"github.com/opscompanion/opc/internal/api"
	"github.com/opscompanion/opc/internal/config"
	"github.com/opscompanion/opc/internal/models"
	"github.com/spf13/cobra"
)

var contextIncludeComputedLinks bool
var contextVerbose bool
var contextFullMemory bool

var contextCmd = &cobra.Command{
	Use:   "context",
	Short: "Display org, user, integration, and workspace context",
	RunE:  runContext,
}

func init() {
	contextCmd.Flags().BoolVar(&contextIncludeComputedLinks, "computed-links", false, "include computed workspace links")
	contextCmd.Flags().BoolVarP(&contextVerbose, "verbose", "v", false, "show detailed nodes and integration entries")
	contextCmd.Flags().BoolVar(&contextFullMemory, "full-memory", false, "show full memory bodies instead of excerpts")
	rootCmd.AddCommand(contextCmd)
}

func runContext(cmd *cobra.Command, args []string) error {
	cfg, err := config.RequireConfig()
	if err != nil {
		return err
	}

	client := api.New(cfg)
	ctx, err := client.GetContext(contextIncludeComputedLinks)
	if err != nil {
		return fmt.Errorf("fetching context: %w", err)
	}

	fmt.Println("## Identity")
	fmt.Println()
	fmt.Printf("- **Organization**: %s\n", displayString(ctx.Organization.Name, "(unknown)"))
	fmt.Printf("  - Public ID: %s\n", ctx.Organization.PublicID)
	if ctx.User == nil {
		fmt.Println("- **User**: none")
	} else {
		name := strings.TrimSpace(strings.Join([]string{
			displayString(ctx.User.FirstName, ""),
			displayString(ctx.User.LastName, ""),
		}, " "))
		if name == "" {
			name = "(unnamed)"
		}
		fmt.Printf("- **User**: %s\n", name)
		fmt.Printf("  - Public ID: %s\n", ctx.User.PublicID)
		fmt.Printf("  - Email: %s\n", displayString(ctx.User.Email, "(hidden)"))
	}

	fmt.Println()
	fmt.Println("## Integrations")
	fmt.Println()
	if ctx.IntegrationsUnauthorized != nil {
		printUnauthorized(ctx.IntegrationsUnauthorized)
	} else if len(ctx.Integrations) == 0 {
		fmt.Println("- No integrations available.")
	} else {
		providers := summarizeIntegrations(ctx.Integrations)
		fmt.Printf("- %d configured: %s\n", len(ctx.Integrations), strings.Join(providers, ", "))
		if contextVerbose {
			for _, integration := range ctx.Integrations {
				fmt.Printf("  - %s (%s) active=%t\n", integration.Provider, integration.PublicID, integration.IsActive)
			}
		}
	}

	fmt.Println()
	fmt.Println("## Workspaces")
	fmt.Println()
	if ctx.WorkspacesUnauthorized != nil {
		printUnauthorized(ctx.WorkspacesUnauthorized)
	} else if len(ctx.Workspaces) == 0 {
		fmt.Println("- No workspaces available.")
	} else {
		for _, workspace := range ctx.Workspaces {
			fmt.Printf("### %s\n\n", workspace.Name)
			fmt.Printf("- **Public ID**: %s\n", workspace.PublicID)
			fmt.Printf("- **Nodes**: %d\n", workspace.NodeCount)
			fmt.Printf("- **Saved links**: %d\n", len(workspace.Links))
			if contextIncludeComputedLinks {
				fmt.Printf("- **Computed links**: %d\n", len(workspace.ComputedLinks))
			}
			fmt.Printf("- **Providers**: %s\n", summarizeWorkspaceProviders(workspace))
			if contextVerbose && len(workspace.Nodes) > 0 {
				fmt.Println("- **Nodes**:")
				for _, node := range workspace.Nodes {
					fmt.Printf("  - %s · %s · %s\n", node.PublicID, node.Provider, truncate(node.ExternalID, 72))
				}
			}
			fmt.Println()
		}
	}

	fmt.Println("## Memory")
	fmt.Println()
	fmt.Println("### Organization")
	fmt.Println()
	printMemorySection("Organization", ctx.Memory.Organization)
	fmt.Println()
	fmt.Println("### User")
	fmt.Println()
	printMemorySection("User", ctx.Memory.User)

	return nil
}

func printUnauthorized(section *models.UnauthorizedSection) {
	fmt.Printf("- Unauthorized. Required scopes: %s\n", strings.Join(section.RequiredScopes, ", "))
}

func printMemorySection(label string, section models.MemorySection) {
	switch {
	case section.Unauthorized != nil:
		printUnauthorized(section.Unauthorized)
	case section.Content == nil:
		fmt.Println("- No memory available.")
	default:
		content := strings.TrimSpace(*section.Content)
		if contextFullMemory {
			fmt.Printf("%s\n", content)
			return
		}
		excerpt := excerptMemory(content)
		lines := strings.Count(content, "\n") + 1
		fmt.Printf("- %s memory available (%d lines)\n", strings.ToLower(label), lines)
		fmt.Println()
		fmt.Println(excerpt)
		if excerpt != content {
			fmt.Println()
			fmt.Println("...")
			fmt.Println()
			fmt.Println("Use `opc context --full-memory` to show the full body.")
		}
	}
}

func displayString(value *string, fallback string) string {
	if value == nil || strings.TrimSpace(*value) == "" {
		return fallback
	}
	return *value
}

func summarizeIntegrations(integrations []models.Integration) []string {
	providers := make([]string, 0, len(integrations))
	for _, integration := range integrations {
		providers = append(providers, integration.Provider)
	}
	sort.Strings(providers)
	return providers
}

func summarizeWorkspaceProviders(workspace models.Workspace) string {
	set := map[string]struct{}{}
	for _, node := range workspace.Nodes {
		if strings.TrimSpace(node.Provider) != "" {
			set[node.Provider] = struct{}{}
		}
	}
	if len(set) == 0 {
		return "(none)"
	}
	providers := make([]string, 0, len(set))
	for provider := range set {
		providers = append(providers, provider)
	}
	sort.Strings(providers)
	return strings.Join(providers, ", ")
}

func excerptMemory(content string) string {
	lines := strings.Split(content, "\n")
	if len(lines) <= 8 {
		return content
	}
	return strings.Join(lines[:8], "\n")
}

func truncate(value string, max int) string {
	if max <= 3 || len(value) <= max {
		return value
	}
	return value[:max-3] + "..."
}
