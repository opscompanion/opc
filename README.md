# OPC — OpsCompanion CLI

Persistent, shared memory for AI coding agents. OPC captures decisions and discoveries from your agent sessions (Claude Code, Cursor, Codex) and makes them searchable across tools, sessions, and teammates.

## Install

```bash
brew tap opscompanion/opc
brew install opc
```

To upgrade:

```bash
brew update && brew upgrade opc
```

### From source

```bash
git clone https://github.com/opscompanion/opc.git
cd opc
make install
```

Requires Go 1.25+.

## Quick start

```bash
# 1. Configure your API key
opc init

# 2. Set up Claude Code hooks
opc hooks

# 3. Start a session (happens automatically via hooks)
opc session start

# 4. Save a decision
opc remember "Use connection pooling for Redis — single connections hit latency issues under load" --tags redis,performance

# 5. Search memories
opc recall "redis connection"

# 6. View org/team context
opc context
```

## How it works

OPC integrates with AI coding agents via **hooks** — lightweight event handlers that fire on tool calls, prompts, and session lifecycle events.

```
Agent (Claude Code, Cursor, etc.)
  → fires hook events
    → opc capture (writes to local JSONL buffer)
      → checkpoint triggers (git commit, every 50 events, session stop)
        → sync to OpsCompanion API
          → extraction pipeline → shared memories
```

Memories extracted from one agent are available to all agents via `opc recall`. No MCP required — hooks are deterministic and don't bloat context with tool schemas.

## Commands

| Command | Description |
|---------|-------------|
| `opc init` | Configure API key and endpoint |
| `opc hooks` | Generate Claude Code hook configuration |
| `opc context` | Display org, team, and user context |
| `opc session start` | Start a new session with context loading |
| `opc session stop` | End session and extract memories |
| `opc session resume <id>` | Resume a previous session |
| `opc session list` | List local sessions |
| `opc session inspect <id>` | Inspect session events and checkpoints |
| `opc session checkpoint` | Create a manual checkpoint |
| `opc remember <text>` | Save a decision or discovery (`--tags` to add tags) |
| `opc recall <query>` | Search stored memories |
| `opc history` | View session history with decisions |
| `opc capture` | Capture hook events from stdin (used by hooks) |
| `opc version` | Print version info |

## Architecture

OPC follows a **thin client, smart server** design:

- **Client** — captures events, buffers locally as JSONL, syncs to API
- **Server** — handles extraction, dedup, semantic search, ranking
- **No local database** — `/tmp/opc/sessions/` is a temporary buffer

This means intelligence improves server-side without shipping binary updates.

### Multi-agent memory sharing

Multiple agents can share knowledge in near-real-time:

```
Agent A discovers a bug → checkpoint → API extracts memory
                                              ↓
Agent B asks about the same area → opc recall → gets Agent A's discovery
```

### Memory layers

| Layer | Purpose | Shared? |
|-------|---------|---------|
| **Raw events** | Audit log of every tool call | No |
| **Memories** | Extracted decisions and discoveries | Yes |

An hour-long session might produce 500 events but only 3–5 memories.

## Supported agents

| Agent | Integration | Status |
|-------|-------------|--------|
| Claude Code | `.claude/settings.local.json` hooks | Supported |
| Cursor | `.cursor/hooks.json` | Planned |
| Codex | Config hooks | Planned |
| OpenCode | TypeScript plugin | Planned |

## Development

```bash
make build      # Build binary
make install    # Install to $GOPATH/bin
make snapshot   # GoReleaser local build
make clean      # Remove binary
```

### Releasing

```bash
git tag v0.3.0
git push origin v0.3.0
# GitHub Actions → GoReleaser → GitHub Release → Homebrew formula auto-updates
```

Builds target darwin and linux on amd64 and arm64.

## Contributing

Contributions welcome! Please open an issue or pull request.

## License

See [LICENSE](LICENSE) for details.
