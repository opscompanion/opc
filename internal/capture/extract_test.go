package capture

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestReadTranscriptParsesSupportedFormats(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	path := filepath.Join(home, "transcript.ndjson")
	longText := strings.Repeat("x", 2100)
	lines := []string{
		`{"message":{"role":"user","content":"hello"}}`,
		`{"role":"assistant","content":[{"text":"alpha"},"beta"]}`,
		`not json`,
		`{"message":{"role":"assistant","content":"` + longText + `"}}`,
		`{"role":"system","content":null}`,
	}
	if err := os.WriteFile(path, []byte(strings.Join(lines, "\n")), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	got, err := ReadTranscript("~/transcript.ndjson")
	if err != nil {
		t.Fatalf("ReadTranscript: %v", err)
	}

	if !strings.Contains(got, "**user**: hello") {
		t.Fatalf("missing wrapped message content: %q", got)
	}
	if !strings.Contains(got, "**assistant**: alpha beta") {
		t.Fatalf("missing flattened array content: %q", got)
	}
	if strings.Contains(got, "not json") {
		t.Fatalf("invalid json line should be skipped: %q", got)
	}
	if !strings.Contains(got, strings.Repeat("x", 2000)+"...") {
		t.Fatalf("long content should be truncated: %q", got)
	}
}

func TestExtractTurnContent(t *testing.T) {
	raw := map[string]json.RawMessage{
		"message": json.RawMessage(`{"role":"assistant","content":[{"text":"one"},"two"]}`),
	}

	role, content := extractTurnContent(raw)
	if role != "assistant" || content != "one two" {
		t.Fatalf("extractTurnContent() = (%q, %q)", role, content)
	}
}

func TestFlattenContentUnsupportedType(t *testing.T) {
	if got := flattenContent(map[string]any{"text": "ignored"}); got != "" {
		t.Fatalf("flattenContent(map) = %q, want empty", got)
	}
}
