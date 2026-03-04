package capture

import (
	"bufio"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const baseDir = "/tmp/opsctl/sessions"

// Event represents a single captured event in a session.
type Event struct {
	ID        string      `json:"id"`
	ParentID  string      `json:"parent_id,omitempty"`
	Type      string      `json:"type"` // user_message, assistant_message, tool_use, tool_result, thinking, hook
	Timestamp string      `json:"timestamp"`
	SessionID string      `json:"session_id"`
	Role      string      `json:"role,omitempty"` // user, assistant, system
	Tokens    *TokenUsage `json:"tokens,omitempty"`
	Data      interface{} `json:"data"`
}

// TokenUsage tracks token consumption for an event.
type TokenUsage struct {
	Input  int `json:"input"`
	Output int `json:"output"`
}

// CheckpointMeta is the metadata saved alongside a checkpoint (metadata.json).
type CheckpointMeta struct {
	CheckpointID    string            `json:"checkpoint_id"`
	SessionID       string            `json:"session_id"`
	CheckpointIndex int               `json:"checkpoint_index"`
	CommitHash      string            `json:"commit_hash,omitempty"`
	CommitMessage   string            `json:"commit_message,omitempty"`
	Branch          string            `json:"branch,omitempty"`
	Author          string            `json:"author,omitempty"`
	FilesModified   []string          `json:"files_modified,omitempty"`
	EventCount      int               `json:"event_count"`
	TokenUsage      TokenUsage        `json:"token_usage"`
	Attribution     AgentAttribution  `json:"attribution"`
	CreatedAt       string            `json:"created_at"`
}

// AgentAttribution tracks how much of the committed code was agent-written.
type AgentAttribution struct {
	Agent            string  `json:"agent"`
	TotalLines       int     `json:"total_lines_committed"`
	AgentLines       int     `json:"agent_lines"`
	AgentPercent     float64 `json:"agent_percent"`
}

// SessionDir returns the directory for a session's captured data.
func SessionDir(sessionID string) string {
	return filepath.Join(baseDir, sessionID)
}

// checkpointDir returns the sharded directory for a checkpoint.
// Uses Entire's convention: <first2>/<next10>/<index>/
func checkpointDir(sessionID, checkpointID string, index int) string {
	prefix := checkpointID[:2]
	rest := checkpointID[2:]
	return filepath.Join(SessionDir(sessionID), prefix, rest, fmt.Sprintf("%d", index))
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

// CreateCheckpoint takes a snapshot of the current session state, linked to a commit.
// Produces the full Entire.io-style checkpoint folder:
//
//	<prefix2>/<rest10>/<index>/
//	  full.jsonl        — complete session log up to this point
//	  context.md        — clean summary of user prompts
//	  prompt.txt        — raw user-facing prompts
//	  metadata.json     — stats: files, tokens, attribution
//	  content_hash.txt  — SHA-256 of full.jsonl for integrity
func CreateCheckpoint(sessionID, commitHash, commitMsg, branch, author string, filesModified []string, linesAdded int) (*CheckpointMeta, error) {
	// Derive 12-char hex checkpoint ID from commit hash
	h := sha256.Sum256([]byte(commitHash + sessionID))
	cpID := fmt.Sprintf("%x", h[:6])

	// Determine checkpoint index (how many checkpoints already exist for this ID)
	index := nextCheckpointIndex(sessionID, cpID)

	cpDir := checkpointDir(sessionID, cpID, index)
	if err := os.MkdirAll(cpDir, 0755); err != nil {
		return nil, fmt.Errorf("creating checkpoint dir: %w", err)
	}

	// Read the full session log
	srcLog := filepath.Join(SessionDir(sessionID), "full.jsonl")
	logData, err := os.ReadFile(srcLog)
	if err != nil && !os.IsNotExist(err) {
		return nil, fmt.Errorf("reading session log: %w", err)
	}

	// --- full.jsonl ---
	if len(logData) > 0 {
		if err := os.WriteFile(filepath.Join(cpDir, "full.jsonl"), logData, 0644); err != nil {
			return nil, fmt.Errorf("writing full.jsonl: %w", err)
		}
	}

	// Parse events for stats
	events := parseEvents(logData)
	eventCount := len(events)
	totalTokens := sumTokens(events)
	prompts := extractUserPrompts(events)

	// --- content_hash.txt ---
	contentHash := sha256.Sum256(logData)
	os.WriteFile(
		filepath.Join(cpDir, "content_hash.txt"),
		[]byte(fmt.Sprintf("sha256:%x\n", contentHash)),
		0644,
	)

	// --- prompt.txt ---
	var promptBuf strings.Builder
	for _, p := range prompts {
		promptBuf.WriteString(p)
		promptBuf.WriteString("\n\n---\n\n")
	}
	os.WriteFile(filepath.Join(cpDir, "prompt.txt"), []byte(promptBuf.String()), 0644)

	// --- context.md ---
	var ctxBuf strings.Builder
	ctxBuf.WriteString(fmt.Sprintf("# Session Context\n\n"))
	ctxBuf.WriteString(fmt.Sprintf("**Session**: `%s`\n", sessionID))
	ctxBuf.WriteString(fmt.Sprintf("**Commit**: `%s` — %s\n", commitHash[:12], commitMsg))
	ctxBuf.WriteString(fmt.Sprintf("**Branch**: `%s`\n", branch))
	ctxBuf.WriteString(fmt.Sprintf("**Author**: %s\n\n", author))
	ctxBuf.WriteString("## User Prompts\n\n")
	for i, p := range prompts {
		ctxBuf.WriteString(fmt.Sprintf("### Prompt %d\n\n%s\n\n", i+1, p))
	}
	if len(filesModified) > 0 {
		ctxBuf.WriteString("## Files Modified\n\n")
		for _, f := range filesModified {
			ctxBuf.WriteString(fmt.Sprintf("- `%s`\n", f))
		}
	}
	os.WriteFile(filepath.Join(cpDir, "context.md"), []byte(ctxBuf.String()), 0644)

	// --- metadata.json ---
	agentLines := linesAdded
	agentPct := 100.0
	if agentLines == 0 {
		agentPct = 0
	}

	meta := &CheckpointMeta{
		CheckpointID:    cpID,
		SessionID:       sessionID,
		CheckpointIndex: index,
		CommitHash:      commitHash,
		CommitMessage:   commitMsg,
		Branch:          branch,
		Author:          author,
		FilesModified:   filesModified,
		EventCount:      eventCount,
		TokenUsage:      totalTokens,
		Attribution: AgentAttribution{
			Agent:        "Claude Code",
			TotalLines:   agentLines,
			AgentLines:   agentLines,
			AgentPercent: agentPct,
		},
		CreatedAt: time.Now().UTC().Format(time.RFC3339),
	}

	metaData, err := json.MarshalIndent(meta, "", "  ")
	if err != nil {
		return nil, err
	}
	if err := os.WriteFile(filepath.Join(cpDir, "metadata.json"), metaData, 0644); err != nil {
		return nil, fmt.Errorf("writing metadata.json: %w", err)
	}

	return meta, nil
}

// nextCheckpointIndex counts how many checkpoint snapshots already exist under this ID.
func nextCheckpointIndex(sessionID, cpID string) int {
	prefix := cpID[:2]
	rest := cpID[2:]
	parent := filepath.Join(SessionDir(sessionID), prefix, rest)
	entries, err := os.ReadDir(parent)
	if err != nil {
		return 0
	}
	max := -1
	for _, e := range entries {
		if e.IsDir() {
			var n int
			if _, err := fmt.Sscanf(e.Name(), "%d", &n); err == nil && n > max {
				max = n
			}
		}
	}
	return max + 1
}

// parseEvents reads JSONL data and returns parsed events.
func parseEvents(data []byte) []Event {
	var events []Event
	scanner := bufio.NewScanner(strings.NewReader(string(data)))
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}
		var ev Event
		if err := json.Unmarshal([]byte(line), &ev); err == nil {
			events = append(events, ev)
		}
	}
	return events
}

// sumTokens totals token usage across all events.
func sumTokens(events []Event) TokenUsage {
	var total TokenUsage
	for _, ev := range events {
		if ev.Tokens != nil {
			total.Input += ev.Tokens.Input
			total.Output += ev.Tokens.Output
		}
	}
	return total
}

// extractUserPrompts pulls out user_message content from events.
func extractUserPrompts(events []Event) []string {
	var prompts []string
	for _, ev := range events {
		if ev.Type == "user_message" || ev.Role == "user" {
			if s, ok := ev.Data.(string); ok {
				prompts = append(prompts, s)
			} else if m, ok := ev.Data.(map[string]interface{}); ok {
				if content, ok := m["content"].(string); ok {
					prompts = append(prompts, content)
				}
			}
		}
	}
	return prompts
}

// ListCheckpoints returns all checkpoints for a session by scanning for the
// sharded directory structure: <2char>/<10char>/<index>/metadata.json
func ListCheckpoints(sessionID string) ([]CheckpointMeta, error) {
	sDir := SessionDir(sessionID)
	var checkpoints []CheckpointMeta

	// Walk the session dir looking for metadata.json files
	err := filepath.WalkDir(sDir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil // skip errors
		}
		if d.Name() == "metadata.json" && !d.IsDir() {
			data, err := os.ReadFile(path)
			if err != nil {
				return nil
			}
			var meta CheckpointMeta
			if err := json.Unmarshal(data, &meta); err != nil {
				return nil
			}
			if meta.CheckpointID != "" {
				checkpoints = append(checkpoints, meta)
			}
		}
		return nil
	})
	if err != nil && !os.IsNotExist(err) {
		return nil, err
	}
	return checkpoints, nil
}

// ListSessions returns all session IDs with basic info.
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

// CheckpointPath returns the path for a checkpoint given its metadata.
func CheckpointPath(meta CheckpointMeta) string {
	return checkpointDir(meta.SessionID, meta.CheckpointID, meta.CheckpointIndex)
}
