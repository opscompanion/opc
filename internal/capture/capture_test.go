package capture

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDetectGitCommit(t *testing.T) {
	data := json.RawMessage(`{
		"tool_input":{"command":"git commit -m test"},
		"tool_output":{"stdout":"[main abc1234] test commit"}
	}`)

	got := detectGitCommit(data)
	if got == nil {
		t.Fatal("detectGitCommit() = nil, want detail")
	}
	if got["branch"] != "main" {
		t.Fatalf("branch = %v, want main", got["branch"])
	}
	if got["short_hash"] != "abc1234" {
		t.Fatalf("short_hash = %v, want abc1234", got["short_hash"])
	}
}

func TestEvaluateTriggerForCommitAndStop(t *testing.T) {
	commit := EvaluateTrigger("unused", Event{
		HookType: "PostToolUse",
		ToolName: "Bash",
		Data: json.RawMessage(`{
			"tool_input":{"command":"git commit -m test"},
			"tool_output":{"stdout":"[main abc1234] test commit"}
		}`),
	})
	if commit == nil || commit.Reason != "git_commit" {
		t.Fatalf("commit trigger = %#v", commit)
	}

	stop := EvaluateTrigger("unused", Event{HookType: "Stop"})
	if stop == nil || stop.Reason != "session_stop" {
		t.Fatalf("stop trigger = %#v", stop)
	}
}

func TestEvaluateTriggerPeriodic(t *testing.T) {
	sessionID := "capture-periodic"
	sessionDir := SessionDir(sessionID)
	t.Cleanup(func() {
		_ = os.RemoveAll(sessionDir)
	})

	if err := os.MkdirAll(sessionDir, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	logData := make([]byte, 0, 100)
	for i := 0; i < 50; i++ {
		logData = append(logData, '\n')
	}
	if err := os.WriteFile(filepath.Join(sessionDir, "full.jsonl"), logData, 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	got := EvaluateTrigger(sessionID, Event{HookType: "PreToolUse"})
	if got == nil || got.Reason != "periodic" {
		t.Fatalf("periodic trigger = %#v", got)
	}
	if got.Detail["event_count"] != 50 {
		t.Fatalf("event_count = %v, want 50", got.Detail["event_count"])
	}
}

func TestListCheckpointsMissingReturnsNil(t *testing.T) {
	sessionID := "capture-missing"
	_ = os.RemoveAll(SessionDir(sessionID))

	got, err := ListCheckpoints(sessionID)
	if err != nil {
		t.Fatalf("ListCheckpoints: %v", err)
	}
	if got != nil {
		t.Fatalf("ListCheckpoints() = %#v, want nil", got)
	}
}

func TestListSessionsReturnsDirectoriesOnly(t *testing.T) {
	baseCleanup := filepath.Join(baseDir, "capture-test-a")
	baseCleanup2 := filepath.Join(baseDir, "capture-test-b")
	fileCleanup := filepath.Join(baseDir, "capture-test-file")
	t.Cleanup(func() {
		_ = os.RemoveAll(baseCleanup)
		_ = os.RemoveAll(baseCleanup2)
		_ = os.Remove(fileCleanup)
	})

	if err := os.MkdirAll(baseCleanup, 0o755); err != nil {
		t.Fatalf("MkdirAll a: %v", err)
	}
	if err := os.MkdirAll(baseCleanup2, 0o755); err != nil {
		t.Fatalf("MkdirAll b: %v", err)
	}
	if err := os.MkdirAll(baseDir, 0o755); err != nil {
		t.Fatalf("MkdirAll base: %v", err)
	}
	if err := os.WriteFile(fileCleanup, []byte("x"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	got, err := ListSessions()
	if err != nil {
		t.Fatalf("ListSessions: %v", err)
	}

	foundA := false
	foundB := false
	for _, session := range got {
		if session == "capture-test-a" {
			foundA = true
		}
		if session == "capture-test-b" {
			foundB = true
		}
		if session == "capture-test-file" {
			t.Fatalf("ListSessions included file entry: %q", session)
		}
	}
	if !foundA || !foundB {
		t.Fatalf("ListSessions() missing expected sessions: %#v", got)
	}
}

func TestAppendEventSetsSessionAndTimestamp(t *testing.T) {
	sessionID := "capture-append"
	sessionDir := SessionDir(sessionID)
	t.Cleanup(func() {
		_ = os.RemoveAll(sessionDir)
	})

	err := AppendEvent(sessionID, Event{
		ID:       "evt-1",
		HookType: "PreToolUse",
		ToolName: "Bash",
	})
	if err != nil {
		t.Fatalf("AppendEvent: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(sessionDir, "full.jsonl"))
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if !strings.Contains(string(data), `"session_id":"capture-append"`) {
		t.Fatalf("event log missing session id: %s", data)
	}
	if !strings.Contains(string(data), `"timestamp":"`) {
		t.Fatalf("event log missing timestamp: %s", data)
	}
}

func TestCreateCheckpointWritesMetadataAndDetectsAgent(t *testing.T) {
	sessionID := "capture-checkpoint"
	sessionDir := SessionDir(sessionID)
	t.Cleanup(func() {
		_ = os.RemoveAll(sessionDir)
	})

	if err := os.MkdirAll(sessionDir, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	logLine := "{\"transcript_path\":\"/tmp/.codex/sessions/abc\"}\n"
	if err := os.WriteFile(filepath.Join(sessionDir, "full.jsonl"), []byte(logLine), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	meta, err := CreateCheckpoint(sessionID, &CheckpointTrigger{Reason: "session_stop"})
	if err != nil {
		t.Fatalf("CreateCheckpoint: %v", err)
	}
	if meta.CheckpointID == "" {
		t.Fatal("CheckpointID should be set")
	}
	if meta.Agent != "codex" {
		t.Fatalf("Agent = %q, want codex", meta.Agent)
	}

	data, err := os.ReadFile(filepath.Join(sessionDir, "metadata.json"))
	if err != nil {
		t.Fatalf("ReadFile metadata: %v", err)
	}
	if !strings.Contains(string(data), `"checkpoint_id"`) {
		t.Fatalf("metadata missing checkpoint id: %s", data)
	}
}
