package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/opscompanion/opsctl/internal/api"
	"github.com/opscompanion/opsctl/internal/capture"
	"github.com/opscompanion/opsctl/internal/config"
	"github.com/opscompanion/opsctl/internal/models"
	"github.com/spf13/cobra"
)

var commitCaptureCmd = &cobra.Command{
	Use:   "capture",
	Short: "Link the most recent git commit to the current session",
	Long: `Captures the latest git commit and links it to the active session.
Creates a checkpoint with the session's event log at the time of the commit.

Typically called automatically via a post-commit hook.`,
	RunE: runCommitCapture,
}

func init() {
	commitCmd.AddCommand(commitCaptureCmd)
}

func runCommitCapture(cmd *cobra.Command, args []string) error {
	cfg, err := config.RequireConfig()
	if err != nil {
		return err
	}

	sessionID := os.Getenv("CLAUDE_SESSION_ID")
	if sessionID == "" {
		sessionID = fmt.Sprintf("ses_%d_local", time.Now().Unix())
	}

	// Extract latest commit info
	hash := gitLog("--format=%H", "-1")
	short := gitLog("--format=%h", "-1")
	msg := gitLog("--format=%s", "-1")
	branch := gitOutput("branch", "--show-current")
	author := gitLog("--format=%an", "-1")

	if hash == "" {
		return fmt.Errorf("no commits found in this repository")
	}

	commit := models.CommitRecord{
		SessionID: sessionID,
		Hash:      hash,
		Short:     short,
		Message:   msg,
		Branch:    branch,
		Author:    author,
		Timestamp: time.Now().UTC().Format(time.RFC3339),
	}

	// Get modified files from the commit
	diffOut := gitLog("--format=", "--name-only", "-1")
	var filesModified []string
	for _, f := range strings.Split(diffOut, "\n") {
		f = strings.TrimSpace(f)
		if f != "" {
			filesModified = append(filesModified, f)
		}
	}

	// Count lines added in this commit
	linesAdded := countLinesAdded()

	// Log the commit event to session capture
	capture.AppendEvent(sessionID, capture.Event{
		ID:   fmt.Sprintf("evt_%d_commit", time.Now().UnixNano()),
		Type: "tool_result",
		Data: map[string]interface{}{
			"tool":        "git-commit",
			"hash":        hash,
			"short":       short,
			"message":     msg,
			"branch":      branch,
			"author":      author,
			"files":       filesModified,
			"lines_added": linesAdded,
		},
	})

	// Create checkpoint (Entire.io-style snapshot)
	// Produces: <prefix2>/<rest10>/<index>/{full.jsonl, context.md, prompt.txt, metadata.json, content_hash.txt}
	cp, err := capture.CreateCheckpoint(sessionID, hash, msg, branch, author, filesModified, linesAdded)
	if err != nil {
		fmt.Fprintf(os.Stderr, "warning: checkpoint creation failed: %v\n", err)
	}

	// Link commit to session via API
	client := api.New(cfg)
	if err := client.LinkCommit(sessionID, commit); err != nil {
		return fmt.Errorf("linking commit: %w", err)
	}

	fmt.Printf("Commit linked to session.\n")
	fmt.Printf("  Session:  %s\n", sessionID)
	fmt.Printf("  Commit:   %s (%s)\n", short, msg)
	fmt.Printf("  Branch:   %s\n", branch)
	fmt.Printf("  Author:   %s\n", author)
	if len(filesModified) > 0 {
		fmt.Printf("  Files:    %s\n", strings.Join(filesModified, ", "))
	}
	if cp != nil {
		fmt.Printf("  Checkpoint: %s\n", cp.CheckpointID)
		fmt.Printf("  Events:     %d\n", cp.EventCount)
	}
	return nil
}

func gitLog(args ...string) string {
	fullArgs := append([]string{"log"}, args...)
	out, err := exec.Command("git", fullArgs...).Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

// countLinesAdded returns the number of lines added in the last commit.
func countLinesAdded() int {
	// git diff --stat of the last commit: "X insertions(+)"
	out, err := exec.Command("git", "diff", "--shortstat", "HEAD~1", "HEAD").Output()
	if err != nil {
		return 0
	}
	var insertions int
	for _, part := range strings.Split(string(out), ",") {
		part = strings.TrimSpace(part)
		if strings.Contains(part, "insertion") {
			fmt.Sscanf(part, "%d", &insertions)
		}
	}
	return insertions
}
