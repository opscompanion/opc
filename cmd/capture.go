package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/opscompanion/opc/internal/api"
	"github.com/opscompanion/opc/internal/capture"
	"github.com/opscompanion/opc/internal/config"
	"github.com/opscompanion/opc/internal/scan"
	"github.com/spf13/cobra"
)

var captureHookType string

var captureCmd = &cobra.Command{
	Use:   "capture",
	Short: "Capture an agent hook event to the session log",
	Long: `Reads a hook event from stdin (JSON from agent hooks) and appends
it to the active session's JSONL log. Automatically triggers checkpoints
on git commits, session stops, or every 50 events.

Works with any agent runtime via --agent:
  opc --agent claude capture --hook-type PreToolUse
  opc --agent codex  capture --hook-type PreToolUse

Hook types:
  PreToolUse  → captures every tool invocation before execution
  PostToolUse → captures every tool result after execution
  Stop        → captures session end and creates a checkpoint`,
	SilenceUsage: true,
	RunE:         runCapture,
}

func init() {
	captureCmd.Flags().StringVar(&captureHookType, "hook-type", "", "hook type (PreToolUse, PostToolUse, Stop, etc.)")
	rootCmd.AddCommand(captureCmd)
}

func runCapture(cmd *cobra.Command, args []string) error {
	// Governance hooks — print to stdout, no capture
	switch captureHookType {
	case "SessionStart":
		return runSessionStart()
	case "UserPromptSubmit":
		return runUserPromptSubmit()
	}

	// Read all of stdin
	raw, err := io.ReadAll(os.Stdin)
	if err != nil {
		return fmt.Errorf("reading stdin: %w", err)
	}
	if len(raw) == 0 {
		return nil // nothing to capture
	}

	// Validate JSON
	if !json.Valid(raw) {
		return fmt.Errorf("invalid JSON on stdin")
	}

	// Best-effort extract session_id and tool_name from payload
	var envelope struct {
		SessionID string `json:"session_id"`
		ToolName  string `json:"tool_name"`
	}
	json.Unmarshal(raw, &envelope)

	sessionID := envelope.SessionID
	if sessionID == "" {
		sessionID = ActiveAgent.SessionID()
	}
	if sessionID == "" {
		sessionID = fmt.Sprintf("ses_%d_local", time.Now().Unix())
	}

	event := capture.Event{
		ID:        fmt.Sprintf("evt_%d", time.Now().UnixNano()),
		Timestamp: time.Now().UTC().Format(time.RFC3339Nano),
		SessionID: sessionID,
		HookType:  captureHookType,
		ToolName:  envelope.ToolName,
		Data:      json.RawMessage(raw),
	}

	if err := capture.AppendEvent(sessionID, event); err != nil {
		fmt.Fprintf(os.Stderr, "opc capture: %v\n", err)
		os.Exit(1)
	}

	// Evaluate checkpoint triggers
	if trigger := capture.EvaluateTrigger(sessionID, event); trigger != nil {
		cp, err := capture.CreateCheckpoint(sessionID, trigger)
		if err != nil {
			fmt.Fprintf(os.Stderr, "opc capture: checkpoint failed: %v\n", err)
		} else {
			fmt.Fprintf(os.Stderr, "opc: checkpoint %s (%s)\n", cp.CheckpointID, trigger.Reason)
		}
	}

	return nil
}

func runSessionStart() error {
	var sections []string

	// Git state
	if git := gitSummary(); git != "" {
		sections = append(sections, git)
	}

	// Code map (tree-sitter scan with caching)
	if wd, err := os.Getwd(); err == nil {
		if codeMap, err := scan.Repo(wd); err == nil && codeMap != "" {
			sections = append(sections, codeMap)
		}
	}

	// API context (best-effort)
	if ctx := compactContext(); ctx != "" {
		sections = append(sections, ctx)
	}

	// Available skills
	sections = append(sections, skillsReminder())

	fmt.Println(strings.Join(sections, "\n\n"))
	return nil
}

func gitSummary() string {
	var b strings.Builder
	b.WriteString("## Workspace\n")

	branch, err := exec.Command("git", "branch", "--show-current").Output()
	if err != nil {
		return ""
	}
	b.WriteString(fmt.Sprintf("\n- **Branch**: %s", strings.TrimSpace(string(branch))))

	status, err := exec.Command("git", "status", "--short").Output()
	if err == nil {
		lines := strings.TrimSpace(string(status))
		if lines == "" {
			b.WriteString("\n- **Working tree**: clean")
		} else {
			count := len(strings.Split(lines, "\n"))
			b.WriteString(fmt.Sprintf("\n- **Uncommitted changes**: %d files", count))
		}
	}

	return b.String()
}

func compactContext() string {
	cfg, err := config.Load()
	if err != nil || cfg == nil {
		return ""
	}
	if config.IsMock(cfg) {
		return ""
	}

	client := api.New(cfg)
	ctx, err := client.GetContext(false)
	if err != nil {
		return ""
	}

	var b strings.Builder
	b.WriteString("## Org Context\n")
	b.WriteString(fmt.Sprintf("\n- **Organization**: %s", ptrOr(ctx.Organization.Name, "(unknown)")))
	if ctx.User != nil {
		name := strings.TrimSpace(fmt.Sprintf("%s %s", ptrOr(ctx.User.FirstName, ""), ptrOr(ctx.User.LastName, "")))
		if name != "" {
			b.WriteString(fmt.Sprintf("\n- **User**: %s", name))
		}
	}

	if ctx.Memory.Organization.Content != nil {
		content := strings.TrimSpace(*ctx.Memory.Organization.Content)
		if content != "" {
			lines := strings.Split(content, "\n")
			if len(lines) > 6 {
				lines = lines[:6]
			}
			b.WriteString("\n\n### Organization Memory\n\n")
			b.WriteString(strings.Join(lines, "\n"))
		}
	}

	return b.String()
}

func ptrOr(s *string, fallback string) string {
	if s == nil {
		return fallback
	}
	return *s
}

func skillsReminder() string {
	return `## Available Skills

- **/opscompanion-context** — Load org, team, and environment context
- **/opscompanion-recall** — Search stored knowledge and memory
- **/opscompanion-remember** — Save a decision or discovery
- **/opscompanion-history** — View commit history with agent sessions`
}

func runUserPromptSubmit() error {
	return nil
}
