package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestEnsureCodexOPCRuleCreatesFile(t *testing.T) {
	home := t.TempDir()

	path, added, err := ensureCodexOPCRule(home)
	if err != nil {
		t.Fatalf("ensureCodexOPCRule: %v", err)
	}
	if !added {
		t.Fatal("ensureCodexOPCRule() added = false, want true")
	}

	wantPath := filepath.Join(home, ".codex", "rules", "default.rules")
	if path != wantPath {
		t.Fatalf("path = %q, want %q", path, wantPath)
	}

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if string(got) != codexOPCRule+"\n" {
		t.Fatalf("default.rules = %q, want exact rule block", string(got))
	}
}

func TestEnsureCodexOPCRuleIsIdempotent(t *testing.T) {
	home := t.TempDir()

	if _, _, err := ensureCodexOPCRule(home); err != nil {
		t.Fatalf("first ensureCodexOPCRule: %v", err)
	}
	path, added, err := ensureCodexOPCRule(home)
	if err != nil {
		t.Fatalf("second ensureCodexOPCRule: %v", err)
	}
	if added {
		t.Fatal("second ensureCodexOPCRule() added = true, want false")
	}

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if count := strings.Count(string(got), codexOPCRule); count != 1 {
		t.Fatalf("rule count = %d, want 1", count)
	}
}

func TestEnsureCodexOPCRulePreservesExistingContent(t *testing.T) {
	home := t.TempDir()
	path := filepath.Join(home, ".codex", "rules", "default.rules")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}

	existing := `prefix_rule(
  pattern = ["git", "status"],
  decision = "allow",
  justification = "Allow git status",
)`
	if err := os.WriteFile(path, []byte(existing), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	_, added, err := ensureCodexOPCRule(home)
	if err != nil {
		t.Fatalf("ensureCodexOPCRule: %v", err)
	}
	if !added {
		t.Fatal("ensureCodexOPCRule() added = false, want true")
	}

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	want := existing + "\n" + codexOPCRule + "\n"
	if string(got) != want {
		t.Fatalf("default.rules = %q, want %q", string(got), want)
	}
}
