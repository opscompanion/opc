package capture

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const baseDir = "/tmp/opc/sessions"

// Event represents a single captured event in a session.
// Data is stored as raw JSON — zero parsing, whatever Claude Code sends.
type Event struct {
	ID        string          `json:"id"`
	Timestamp string          `json:"timestamp"`
	SessionID string          `json:"session_id"`
	HookType  string          `json:"hook_type"`           // PreToolUse, PostToolUse, Stop, SessionStart, etc.
	ToolName  string          `json:"tool_name,omitempty"`  // Bash, Write, Edit, Read, Grep, Glob, etc.
	Data      json.RawMessage `json:"data"`                 // Raw hook payload from Claude Code
}

// CheckpointTrigger describes why a checkpoint should be created.
type CheckpointTrigger struct {
	Reason string         `json:"reason"` // git_commit, session_stop, periodic
	Detail map[string]any `json:"detail,omitempty"`
}

// CheckpointMeta is the metadata saved alongside a checkpoint (metadata.json).
type CheckpointMeta struct {
	CheckpointID  string         `json:"checkpoint_id"`
	SessionID     string         `json:"session_id"`
	Agent         string         `json:"agent"`
	Trigger       string         `json:"trigger"`
	TriggerDetail map[string]any `json:"trigger_detail,omitempty"`
	EventCount    int            `json:"event_count"`
	CreatedAt     string         `json:"created_at"`
}

// SessionDir returns the directory for a session's captured data.
func SessionDir(sessionID string) string {
	return filepath.Join(baseDir, sessionID)
}

// EnsureSessionDir creates the session directory if it doesn't exist.
func EnsureSessionDir(sessionID string) error {
	return os.MkdirAll(SessionDir(sessionID), 0755)
}

// AppendEvent writes a single event to the session's JSONL log.
func AppendEvent(sessionID string, event Event) error {
	if err := EnsureSessionDir(sessionID); err != nil {
		return err
	}

	event.SessionID = sessionID
	if event.Timestamp == "" {
		event.Timestamp = time.Now().UTC().Format(time.RFC3339Nano)
	}

	data, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("marshaling event: %w", err)
	}
	data = append(data, '\n')

	logPath := filepath.Join(SessionDir(sessionID), "full.jsonl")
	f, err := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("opening session log: %w", err)
	}
	defer f.Close()

	_, err = f.Write(data)
	return err
}

// EvaluateTrigger examines an event to decide if a checkpoint should be created.
func EvaluateTrigger(sessionID string, event Event) *CheckpointTrigger {
	// Git commit detection: PostToolUse + Bash + output looks like a commit
	if event.HookType == "PostToolUse" && event.ToolName == "Bash" {
		if detail := detectGitCommit(event.Data); detail != nil {
			return &CheckpointTrigger{Reason: "git_commit", Detail: detail}
		}
	}

	// Session stop
	if event.HookType == "Stop" {
		return &CheckpointTrigger{Reason: "session_stop"}
	}

	// Periodic: every 50 events
	count := countEvents(sessionID)
	if count > 0 && count%50 == 0 {
		return &CheckpointTrigger{
			Reason: "periodic",
			Detail: map[string]any{"event_count": count},
		}
	}

	return nil
}

// detectGitCommit checks if a Bash PostToolUse event looks like a git commit.
func detectGitCommit(data json.RawMessage) map[string]any {
	var payload struct {
		ToolInput struct {
			Command string `json:"command"`
		} `json:"tool_input"`
		ToolOutput struct {
			Stdout string `json:"stdout"`
		} `json:"tool_output"`
	}
	if err := json.Unmarshal(data, &payload); err != nil {
		return nil
	}

	cmd := payload.ToolInput.Command
	out := payload.ToolOutput.Stdout

	if !strings.Contains(cmd, "git commit") {
		return nil
	}

	detail := map[string]any{
		"command": cmd,
		"output":  out,
	}

	// Try to extract commit hash from output like "[main abc1234] message"
	if idx := strings.Index(out, "["); idx >= 0 {
		if end := strings.Index(out[idx:], "]"); end >= 0 {
			bracket := out[idx+1 : idx+end]
			parts := strings.SplitN(bracket, " ", 2)
			if len(parts) == 2 {
				detail["branch"] = parts[0]
				detail["short_hash"] = parts[1]
			}
		}
	}

	return detail
}

// countEvents counts the number of events in a session's JSONL log.
func countEvents(sessionID string) int {
	logPath := filepath.Join(SessionDir(sessionID), "full.jsonl")
	data, err := os.ReadFile(logPath)
	if err != nil {
		return 0
	}
	count := 0
	for _, b := range data {
		if b == '\n' {
			count++
		}
	}
	return count
}

// CreateCheckpoint takes a snapshot of the current session state.
// Everything is written into the session directory — no subdirectories.
// The checkpoint ID is the SHA-256 hash of metadata.json.
//
// Files written to <session-dir>/:
//
//	full.jsonl        — already exists (live log)
//	context.md        — clean summary of user prompts + tools
//	prompt.txt        — raw user-facing prompts
//	metadata.json     — checkpoint metadata (ID = hash of this file)
//	content_hash.txt  — SHA-256 of full.jsonl for integrity
func CreateCheckpoint(sessionID string, trigger *CheckpointTrigger) (*CheckpointMeta, error) {
	sDir := SessionDir(sessionID)
	if err := os.MkdirAll(sDir, 0755); err != nil {
		return nil, fmt.Errorf("creating session dir: %w", err)
	}

	eventCount := countEvents(sessionID)

	// --- metadata.json ---
	// Write without checkpoint_id first, then hash the file to get the ID.
	meta := &CheckpointMeta{
		SessionID:     sessionID,
		Agent:         detectAgent(sessionID),
		Trigger:       trigger.Reason,
		TriggerDetail: trigger.Detail,
		EventCount:    eventCount,
		CreatedAt:     time.Now().UTC().Format(time.RFC3339),
	}

	metaData, err := json.MarshalIndent(meta, "", "  ")
	if err != nil {
		return nil, err
	}

	// Checkpoint ID = first 12 hex chars of SHA-256 of metadata.json content
	h := sha256.Sum256(metaData)
	meta.CheckpointID = fmt.Sprintf("%x", h[:6])

	// Re-marshal with the ID included
	metaData, err = json.MarshalIndent(meta, "", "  ")
	if err != nil {
		return nil, err
	}

	if err := os.WriteFile(filepath.Join(sDir, "metadata.json"), metaData, 0644); err != nil {
		return nil, fmt.Errorf("writing metadata.json: %w", err)
	}

	return meta, nil
}

// detectAgent infers the calling agent from captured event data.
func detectAgent(sessionID string) string {
	logPath := filepath.Join(SessionDir(sessionID), "full.jsonl")
	data, err := os.ReadFile(logPath)
	if err != nil {
		return "unknown"
	}
	// Check the first event for transcript_path hints
	content := string(data)
	if idx := strings.IndexByte(content, '\n'); idx > 0 {
		line := content[:idx]
		if strings.Contains(line, ".claude/") {
			return "claude-code"
		}
		if strings.Contains(line, ".codex/") || strings.Contains(line, "codex") {
			return "codex"
		}
		if strings.Contains(line, ".cursor/") {
			return "cursor"
		}
	}
	return "unknown"
}

// ListCheckpoints returns all checkpoints for a session by reading metadata.json.
func ListCheckpoints(sessionID string) ([]CheckpointMeta, error) {
	metaPath := filepath.Join(SessionDir(sessionID), "metadata.json")
	data, err := os.ReadFile(metaPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var meta CheckpointMeta
	if err := json.Unmarshal(data, &meta); err != nil {
		return nil, err
	}
	return []CheckpointMeta{meta}, nil
}

// ListSessions returns all session IDs.
func ListSessions() ([]string, error) {
	entries, err := os.ReadDir(baseDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var sessions []string
	for _, e := range entries {
		if e.IsDir() {
			sessions = append(sessions, e.Name())
		}
	}
	return sessions, nil
}
