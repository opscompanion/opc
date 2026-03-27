package agent

import (
	"strings"
	"testing"
)

func TestResolveExplicitAgent(t *testing.T) {
	got := Resolve(" Codex ")
	if got.Name != Codex {
		t.Fatalf("Resolve(Codex).Name = %q, want %q", got.Name, Codex)
	}
	if got.SessionEnvVar != "CODEX_SESSION_ID" {
		t.Fatalf("Resolve(Codex).SessionEnvVar = %q", got.SessionEnvVar)
	}
}

func TestResolveUnknownAgentFallsBackToGeneric(t *testing.T) {
	got := Resolve("custom-agent")
	if got.Name != Name("custom-agent") {
		t.Fatalf("Resolve(custom-agent).Name = %q", got.Name)
	}
	if got.HookFormat != "generic" {
		t.Fatalf("Resolve(custom-agent).HookFormat = %q", got.HookFormat)
	}
}

func TestResolveAutoUsesEnvironmentPriority(t *testing.T) {
	t.Setenv("CLAUDE_SESSION_ID", "")
	t.Setenv("CODEX_SESSION_ID", "")
	t.Setenv("CURSOR_SESSION_ID", "")
	t.Setenv("OPENCLAW_SESSION_ID", "")

	if got := Resolve("auto"); got.Name != Unknown {
		t.Fatalf("Resolve(auto).Name = %q, want unknown", got.Name)
	}

	t.Setenv("CURSOR_SESSION_ID", "cursor-1")
	t.Setenv("CODEX_SESSION_ID", "codex-1")

	got := Resolve("")
	if got.Name != Codex {
		t.Fatalf("Resolve(\"\").Name = %q, want %q", got.Name, Codex)
	}
}

func TestSessionIDUsesConfiguredEnvVar(t *testing.T) {
	t.Setenv("CODEX_SESSION_ID", "session-123")

	got := registry[Codex].SessionID()
	if got != "session-123" {
		t.Fatalf("SessionID() = %q, want %q", got, "session-123")
	}
}

func TestValidate(t *testing.T) {
	if err := Validate("auto"); err != nil {
		t.Fatalf("Validate(auto): %v", err)
	}
	if err := Validate("cursor"); err != nil {
		t.Fatalf("Validate(cursor): %v", err)
	}
	if err := Validate("bad-agent"); err == nil || !strings.Contains(err.Error(), "supported:") {
		t.Fatalf("Validate(bad-agent) error = %v, want supported list", err)
	}
}
