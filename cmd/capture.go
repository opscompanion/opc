package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/opscompanion/opc/internal/capture"
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
