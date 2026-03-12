package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/opscompanion/opc/internal/capture"
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
		toolCounts := map[string]int{}
		for _, line := range strings.Split(string(data), "\n") {
			if line == "" {
				continue
			}
			lines++
			var ev struct {
				ToolName string `json:"tool_name"`
			}
			if err := json.Unmarshal([]byte(line), &ev); err == nil && ev.ToolName != "" {
				toolCounts[ev.ToolName]++
			}
		}
		fmt.Printf("## Event Log\n\n")
		fmt.Printf("- **Events**: %d\n", lines)
		fmt.Printf("- **Size**: %s\n", formatBytes(info.Size()))
		if len(toolCounts) > 0 {
			parts := []string{}
			for tool, count := range toolCounts {
				parts = append(parts, fmt.Sprintf("%s(%d)", tool, count))
			}
			fmt.Printf("- **Tools**: %s\n", strings.Join(parts, ", "))
		}
	} else {
		fmt.Println("No event log found.")
	}

	// Show checkpoint (metadata.json in same dir)
	cps, err := capture.ListCheckpoints(sessionID)
	if err != nil {
		return err
	}

	if len(cps) == 0 {
		fmt.Println("\nNo checkpoint yet.")
		return nil
	}

	cp := cps[0]
	fmt.Printf("\n## Checkpoint `%s`\n\n", cp.CheckpointID)
	fmt.Printf("- **Trigger**: %s\n", cp.Trigger)
	fmt.Printf("- **Events**: %d\n", cp.EventCount)
	fmt.Printf("- **Created**: %s\n", cp.CreatedAt)
	if len(cp.TriggerDetail) > 0 {
		for k, v := range cp.TriggerDetail {
			fmt.Printf("- **%s**: %v\n", k, v)
		}
	}

	// List all files in session dir
	fmt.Printf("\n## Files\n\n")
	entries, _ := os.ReadDir(dir)
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		info, _ := e.Info()
		size := int64(0)
		if info != nil {
			size = info.Size()
		}
		fmt.Printf("- `%s` (%s)\n", e.Name(), formatBytes(size))
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
