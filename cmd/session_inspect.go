package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/opscompanion/opsctl/internal/capture"
	"github.com/spf13/cobra"
)

var sessionInspectCmd = &cobra.Command{
	Use:   "inspect <session-id>",
	Short: "Inspect a captured session's events and checkpoints",
	Args:  cobra.ExactArgs(1),
	RunE:  runSessionInspect,
}

func init() {
	sessionCmd.AddCommand(sessionInspectCmd)
}

func runSessionInspect(cmd *cobra.Command, args []string) error {
	sessionID := args[0]
	dir := capture.SessionDir(sessionID)

	if _, err := os.Stat(dir); os.IsNotExist(err) {
		return fmt.Errorf("session %s not found at %s", sessionID, dir)
	}

	fmt.Printf("# Session: `%s`\n\n", sessionID)
	fmt.Printf("**Directory**: %s\n\n", dir)

	// Show event log stats
	logPath := filepath.Join(dir, "full.jsonl")
	if info, err := os.Stat(logPath); err == nil {
		data, _ := os.ReadFile(logPath)
		lines := 0
		for _, b := range data {
			if b == '\n' {
				lines++
			}
		}
		fmt.Printf("## Event Log\n\n")
		fmt.Printf("- **Events**: %d\n", lines)
		fmt.Printf("- **Size**: %s\n", formatBytes(info.Size()))
		fmt.Printf("- **Path**: %s\n", logPath)
	} else {
		fmt.Println("No event log found.")
	}

	// Show checkpoints
	cps, err := capture.ListCheckpoints(sessionID)
	if err != nil {
		return err
	}

	fmt.Printf("\n## Checkpoints (%d)\n\n", len(cps))
	if len(cps) == 0 {
		fmt.Println("No checkpoints yet.")
		return nil
	}

	for _, cp := range cps {
		cpPath := capture.CheckpointPath(cp)
		fmt.Printf("### `%s/%d` — %s\n\n", cp.CheckpointID, cp.CheckpointIndex, cp.CommitMessage)
		fmt.Printf("- **Commit**: `%s`\n", cp.CommitHash[:12])
		fmt.Printf("- **Branch**: %s\n", cp.Branch)
		fmt.Printf("- **Events**: %d\n", cp.EventCount)
		fmt.Printf("- **Tokens**: %d input / %d output\n", cp.TokenUsage.Input, cp.TokenUsage.Output)
		fmt.Printf("- **Attribution**: %s — %d/%d lines (%.0f%%)\n",
			cp.Attribution.Agent, cp.Attribution.AgentLines, cp.Attribution.TotalLines, cp.Attribution.AgentPercent)

		// List checkpoint files
		fmt.Printf("- **Files**:\n")
		entries, _ := os.ReadDir(cpPath)
		for _, e := range entries {
			info, _ := e.Info()
			size := int64(0)
			if info != nil {
				size = info.Size()
			}
			fmt.Printf("  - `%s` (%s)\n", e.Name(), formatBytes(size))
		}

		if len(cp.FilesModified) > 0 {
			fmt.Printf("- **Modified**: %s\n", strings.Join(cp.FilesModified, ", "))
		}
		fmt.Println()
	}
	return nil
}

func formatBytes(b int64) string {
	switch {
	case b >= 1<<20:
		return fmt.Sprintf("%.1f MB", float64(b)/float64(1<<20))
	case b >= 1<<10:
		return fmt.Sprintf("%.1f KB", float64(b)/float64(1<<10))
	default:
		return fmt.Sprintf("%d B", b)
	}
}
