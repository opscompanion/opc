package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/opscompanion/opc/internal/api"
	"github.com/opscompanion/opc/internal/capture"
	"github.com/opscompanion/opc/internal/config"
	"github.com/opscompanion/opc/internal/models"
	"github.com/spf13/cobra"
)

var sessionStartCmd = &cobra.Command{
	Use:   "start",
	Short: "Start a new agent session and load context",
	RunE:  runSessionStart,
}

func init() {
	sessionCmd.AddCommand(sessionStartCmd)
}

func runSessionStart(cmd *cobra.Command, args []string) error {
	cfg, err := config.RequireConfig()
	if err != nil {
		return err
	}

	ag := ActiveAgent
	sessionID := ag.SessionID()
	if sessionID == "" {
		sessionID = fmt.Sprintf("ses_%d_%s", time.Now().Unix(), "local")
	}

	branch := gitOutput("branch", "--show-current")
	repo := gitOutput("remote", "get-url", "origin")

	session := models.Session{
		ID:      sessionID,
		User:    currentUser(),
		Started: time.Now().UTC(),
		Repo:    repo,
		Branch:  branch,
	}

	// Initialize session capture
	hookData, _ := json.Marshal(map[string]interface{}{
		"hook":    "session-start",
		"agent":   string(ag.Name),
		"repo":    repo,
		"branch":  branch,
		"user":    session.User,
		"started": session.Started,
	})
	capture.AppendEvent(sessionID, capture.Event{
		ID:       fmt.Sprintf("evt_%d_start", time.Now().UnixNano()),
		HookType: "SessionStart",
		Data:     json.RawMessage(hookData),
	})

	client := api.New(cfg)
	ctx, err := client.StartSession(session)
	if err != nil {
		return fmt.Errorf("starting session: %w", err)
	}

	fmt.Printf("# OpsCompanion Session Started\n\n")
	fmt.Printf("**Session**: `%s`\n", sessionID)
	fmt.Printf("**Org**: %s · **User**: %s\n", ctx.Org.Name, ctx.User.Name)
	fmt.Printf("**Started**: %s\n\n", session.Started.Format(time.RFC3339))
	fmt.Printf("## Org: %s\n\n", ctx.Org.Name)
	fmt.Printf("- **Cloud provider**: %s\n", ctx.Org.CloudProvider)
	fmt.Printf("- **IaC**: %s\n", ctx.Org.IaC)
	fmt.Printf("- **CI/CD**: %s\n", ctx.Org.CICD)
	fmt.Printf("- **Observability**: %s\n", ctx.Org.Observability)
	fmt.Printf("\n## Team: %s\n\n", ctx.Team.Name)
	fmt.Printf("- **Services owned**: %s\n", strings.Join(ctx.Team.Services, ", "))
	fmt.Printf("- **On-call rotation**: %s\n", ctx.Team.OnCallRotation)
	fmt.Printf("- **Active projects**:\n")
	for _, p := range ctx.Team.ActiveProjects {
		fmt.Printf("  - %s\n", p)
	}
	fmt.Printf("\n## User: %s\n\n", ctx.User.Name)
	fmt.Printf("- **Role**: %s\n", ctx.User.Role)
	fmt.Printf("- **Recent work**: %s\n", strings.Join(ctx.User.RecentWork, ", "))

	fmt.Printf("\nSession log: %s\n", capture.SessionDir(sessionID))
	return nil
}

func gitOutput(args ...string) string {
	out, err := exec.Command("git", args...).Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

func currentUser() string {
	if u := os.Getenv("USER"); u != "" {
		return u
	}
	return "unknown"
}
