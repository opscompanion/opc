package agent

import (
	"fmt"
	"os"
	"strings"
)

// Name is a known agent identifier.
type Name string

const (
	Auto     Name = "auto"
	Claude   Name = "claude"
	Codex    Name = "codex"
	Cursor   Name = "cursor"
	OpenClaw Name = "openclaw"
	Unknown  Name = "unknown"
)

// Info holds agent-specific metadata used to normalize behavior across agents.
type Info struct {
	Name          Name
	SessionEnvVar string // env var that holds the agent's session ID
	HookFormat    string // hook integration style: "claude-hooks", "codex-hooks", "generic"
}

var registry = map[Name]Info{
	Claude: {
		Name:          Claude,
		SessionEnvVar: "CLAUDE_SESSION_ID",
		HookFormat:    "claude-hooks",
	},
	Codex: {
		Name:          Codex,
		SessionEnvVar: "CODEX_SESSION_ID",
		HookFormat:    "codex-hooks",
	},
	Cursor: {
		Name:          Cursor,
		SessionEnvVar: "CURSOR_SESSION_ID",
		HookFormat:    "generic",
	},
	OpenClaw: {
		Name:          OpenClaw,
		SessionEnvVar: "OPENCLAW_SESSION_ID",
		HookFormat:    "generic",
	},
}

// Supported returns the list of valid agent names for flag help text.
func Supported() []string {
	return []string{
		string(Auto),
		string(Claude),
		string(Codex),
		string(Cursor),
		string(OpenClaw),
	}
}

// Resolve takes the --agent flag value and returns the resolved agent Info.
// When flag is "auto" (default), it probes environment variables.
func Resolve(flag string) Info {
	flag = strings.ToLower(strings.TrimSpace(flag))

	if flag != "" && flag != string(Auto) {
		name := Name(flag)
		if info, ok := registry[name]; ok {
			return info
		}
		// Unknown agent name — still usable, just no env detection
		return Info{
			Name:       Name(flag),
			HookFormat: "generic",
		}
	}

	// Auto-detect from environment
	for _, info := range []Info{
		registry[Claude],
		registry[Codex],
		registry[Cursor],
		registry[OpenClaw],
	} {
		if os.Getenv(info.SessionEnvVar) != "" {
			return info
		}
	}

	return Info{Name: Unknown, HookFormat: "generic"}
}

// SessionID returns the active session ID for this agent, checking the
// agent-specific env var. Returns empty string if not set.
func (i Info) SessionID() string {
	if i.SessionEnvVar != "" {
		return os.Getenv(i.SessionEnvVar)
	}
	return ""
}

// Validate checks that the --agent flag value is recognized.
// Returns an error for unrecognized values (except "auto").
func Validate(flag string) error {
	flag = strings.ToLower(strings.TrimSpace(flag))
	if flag == "" || flag == string(Auto) {
		return nil
	}
	if _, ok := registry[Name(flag)]; ok {
		return nil
	}
	return fmt.Errorf("unknown agent %q (supported: %s)", flag, strings.Join(Supported(), ", "))
}
