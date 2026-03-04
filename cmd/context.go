package cmd

import (
	"fmt"
	"strings"

	"github.com/opscompanion/opsctl/internal/api"
	"github.com/opscompanion/opsctl/internal/config"
	"github.com/spf13/cobra"
)

var contextCmd = &cobra.Command{
	Use:   "context",
	Short: "Display org, team, user, and environment context",
	RunE:  runContext,
}

func init() {
	rootCmd.AddCommand(contextCmd)
}

func runContext(cmd *cobra.Command, args []string) error {
	cfg, err := config.RequireConfig()
	if err != nil {
		return err
	}

	client := api.New(cfg)
	ctx, err := client.GetContext()
	if err != nil {
		return fmt.Errorf("fetching context: %w", err)
	}

	fmt.Printf("## Org: %s\n\n", ctx.Org.Name)
	fmt.Printf("- **Cloud provider**: %s\n", ctx.Org.CloudProvider)
	fmt.Printf("- **IaC**: %s\n", ctx.Org.IaC)
	fmt.Printf("- **CI/CD**: %s\n", ctx.Org.CICD)
	fmt.Printf("- **Observability**: %s\n", ctx.Org.Observability)
	fmt.Printf("- **Secrets**: %s\n", ctx.Org.SecretsManager)
	fmt.Printf("- **Incident process**: %s\n", ctx.Org.IncidentProcess)

	fmt.Printf("\n## Team: %s\n\n", ctx.Team.Name)
	fmt.Printf("- **Services owned**: %s\n", strings.Join(ctx.Team.Services, ", "))
	fmt.Printf("- **On-call rotation**: %s\n", ctx.Team.OnCallRotation)
	fmt.Printf("- **Deployment cadence**: %s\n", ctx.Team.DeploymentCadence)
	fmt.Println("- **Active projects**:")
	for _, p := range ctx.Team.ActiveProjects {
		fmt.Printf("  - %s\n", p)
	}

	fmt.Printf("\n## User: %s\n\n", ctx.User.Name)
	fmt.Printf("- **Role**: %s\n", ctx.User.Role)
	fmt.Printf("- **Preferences**: %s\n", ctx.User.Prefs)
	fmt.Printf("- **Recent work**: %s\n", strings.Join(ctx.User.RecentWork, ", "))
	fmt.Printf("- **Editor**: %s\n", ctx.User.Editor)
	fmt.Printf("- **Shell**: %s\n", ctx.User.Shell)

	fmt.Printf("\n## Environment\n\n")
	fmt.Printf("- **Current branch**: %s\n", ctx.Env.Branch)
	fmt.Printf("- **Working directory**: %s\n", ctx.Env.WorkDir)

	return nil
}
