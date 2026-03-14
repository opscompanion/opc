# OPC Architecture & Design Decisions

## Current State (v0.2.1)

### What exists
- CLI binary (`opc`) distributed via Homebrew (`brew tap opscompanion/opc && brew install opc`)
- Hook-based integration with Claude Code (deterministic, no MCP)
- Session lifecycle: start, checkpoint, stop, resume, inspect, list
- Memory CRUD: `remember` (save) and `recall` (search)
- Full event capture to local JSONL (`/tmp/opc/sessions/`)
- Mock API client for development, real HTTP client wired up
- Agent detection from captured event data (claude-code, codex, cursor)
- GoReleaser builds for darwin/linux (amd64/arm64)
- `homebrew-opc` repo auto-updates formula via GitHub Actions polling

### Architecture
```
coding tool hooks/plugins
    → opc CLI binary
        → hosted API (memory storage, retrieval, extraction)
```

Hooks shell out to `opc`, `opc` talks to the API, the API handles the smart stuff (extraction, dedup, conflict resolution, semantic search, ranking). Client stays thin, intelligence stays server-side.

---

## Data Flow

### The full pipeline
```
1. Hook fires (PreToolUse, PostToolUse, Stop, etc.)
         ↓
2. opc capture → writes raw event to local JSONL (buffer)
         ↓
3. Checkpoint triggers (git commit, every 50 events, stop)
         ↓
4. opc spawns detached `opc sync` subprocess (fire-and-forget)
         ↓
5. opc sync:
     - pushes raw events to API
     - API stores events in Postgres
     - API runs extraction → creates memories
     - opc sync returns
         ↓
6. opc recall → hits API directly → returns memories
```

### Key principles
- **Client is thin.** No extraction, no SQLite, no local intelligence. Just capture and sync.
- **Local JSONL is a buffer.** Events queue up locally between checkpoints, get bulk-pushed to the API, cleaned up after confirmed upload.
- **API does all the work.** Extraction, dedup, semantic search, ranking — all server-side. Iterate on intelligence without shipping binary updates.
- **Agents require internet.** Claude Code (and similar tools) don't work offline, so the API is always reachable. No need for offline support or local caching.

### Sync mechanism: fire-and-forget subprocess
No daemon. No PID files. On checkpoint or session stop, `opc` spawns a detached child process to push events to the API. The parent returns immediately.

```go
cmd := exec.Command(os.Args[0], "sync", "--session", sessionID)
cmd.Stdout = nil
cmd.Stderr = nil
cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true} // detach from parent
cmd.Start() // don't wait
```

**When sync fires:**
- On checkpoint (git commit, every 50 events, manual)
- On session stop
- NOT on every event — too expensive

**If the API is down:** child fails silently, local JSONL is safe, next checkpoint retries.

---

## Multi-Agent Memory Sharing

### The vision
5 Claude Code agents (or any mix of supported agents) share session knowledge in near-real-time. Agent A discovers Redis connection pooling is broken → Agent B gets that injected automatically before it adds Redis caching.

### How it works
1. Each agent runs `opc session start` → registers with the API
2. On checkpoint/key events, each agent pushes raw events to the API via `opc sync`
3. API extracts memories from events
4. On `UserPromptSubmit` (every prompt), each agent calls `opc recall` → hits API → gets memories from all active sessions
5. On `PreCompact`, critical shared context gets preserved

### Cross-agent flow
```
Agent A (Claude Code) → opc capture → JSONL → opc sync → API
                                                           ↓
                                                   extraction pipeline
                                                           ↓
                                                   memories stored
                                                           ↓
Agent B (Cursor) → opc recall → API → memories (including Agent A's)
```

---

## Memory Layering

### Problem
Raw event capture from 5 agents = thousands of events per hour. Can't inject all of that.

### Two layers

| Layer | What | Storage | Shared? | Retention |
|-------|------|---------|---------|-----------|
| **Raw events** | Every tool call, every output | Postgres (partitioned by time, moved to cold storage) | No — audit only | Long-term, compliance-driven |
| **Memories** | Extracted decisions, discoveries, context | Postgres with vector search | Yes — between agents | TTL, dedup, merge |

### Raw events (audit log)
- Stored in the API, never queried by agents
- Retained for compliance/audit
- Local JSONL in `/tmp/opc/sessions/` is a buffer until synced
- Bulk upload to API on checkpoint/stop
- Delete local after confirmed upload
- Raw events let you rebuild memories later — change extraction logic, re-run against the log

### Memories (shared knowledge)
- Extracted from events server-side (API runs the extraction pipeline)
- An hour-long session might produce 500 events but only 3-5 memories
- Server-side controls: TTL, dedup, relevance threshold, compression
- Client-side controls: token budget, scope (repo/branch/org), recency bias

### Injection budget
- What gets injected on `UserPromptSubmit`: ~5 results max, ~200 tokens
- Scoped: recent from active sessions (last 30 min, same repo) + semantically relevant to the prompt

---

## Database Schema

### Postgres (API)

```sql
-- Active/past sessions
CREATE TABLE sessions (
    id TEXT PRIMARY KEY,
    org_id TEXT NOT NULL,
    user_id TEXT NOT NULL,
    agent TEXT,                  -- claude-code, cursor, codex, etc.
    repo TEXT,
    branch TEXT,
    started_at TIMESTAMPTZ NOT NULL,
    stopped_at TIMESTAMPTZ,
    status TEXT DEFAULT 'active'
);

-- Raw audit log, append-only, never queried by agents
CREATE TABLE events (
    id TEXT PRIMARY KEY,
    session_id TEXT REFERENCES sessions(id),
    agent TEXT,
    hook_type TEXT,              -- PreToolUse, PostToolUse, Stop, etc.
    tool_name TEXT,              -- Bash, Write, Edit, Read, etc.
    data JSONB NOT NULL,
    created_at TIMESTAMPTZ NOT NULL
);

-- Extracted knowledge, searchable, shared between agents
CREATE TABLE memories (
    id TEXT PRIMARY KEY,
    type TEXT NOT NULL,          -- Decision, Discovery, Context
    content TEXT NOT NULL,
    tags TEXT[],
    repo TEXT,
    branch TEXT,
    agent TEXT,
    session_id TEXT REFERENCES sessions(id),
    org_id TEXT NOT NULL,
    user_id TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL,
    expires_at TIMESTAMPTZ,
    source_event_ids TEXT[]     -- links back to raw events
);
```

### Org/user context
Not in Postgres. Stored as markdown files in R2, keyed by org. The API reads from R2 and returns on `GET /v1/context`. Derived from the API key.

### Local (client)
No database. Just JSONL files in `/tmp/opc/sessions/` as a buffer until synced to the API.

---

## Hook Integration (No MCP)

### Why hooks over MCP
- Hooks are deterministic — memory gets injected whether the agent "feels like it" or not
- No tool definitions bloating context
- No token cost for tool schemas
- No reliance on the agent deciding to call a search tool

### Hook coverage

| Hook | Job | Status |
|------|-----|--------|
| `SessionStart` (startup) | Load org/team/user context + recall repo-scoped memories | Partial — context loads, no memory recall yet |
| `SessionStart` (compact) | Preserve critical context before compaction | Partial — local checkpoint only |
| `SessionStart` (resume) | Reload session context | Implemented |
| `PreToolUse` | Capture every tool invocation | Implemented |
| `PostToolUse` | Capture every tool result | Implemented |
| `UserPromptSubmit` | Inject relevant memories per-prompt (including from other agents) | **Not implemented** |
| `Stop` | Upload event log, extract memories | Partial — mock extraction only |

### Per-agent adapters
One binary, every adapter calls it the same way. Adapters are thin JSON configs.

| Agent | Adapter | Integration surface |
|-------|---------|-------------------|
| Claude Code | `.claude/settings.local.json` hooks | Richest — 15+ hook events |
| Codex | Config + hooks | SessionStart/Stop, thin |
| Cursor | `.cursor/hooks.json` | session-start, stop, afterFileEdit |
| OpenCode | TypeScript plugin via npm | Event bus, full lifecycle |
| Copilot CLI | `.github/hooks/*.json` | Committable to repo |

Setup: `opc hooks` (currently Claude Code only, expand to `opc enable --agent <name>`)

---

## API Design (TODO)

### Endpoints needed

**Sessions:**
- `POST /v1/sessions` — register active session (agent, repo, branch, user)
- `POST /v1/sessions/{id}/stop` — end session, trigger extraction
- `POST /v1/sessions/{id}/events` — bulk upload raw events from JSONL
- `GET /v1/sessions/active` — list active sessions (for multi-agent awareness)

**Memories:**
- `POST /v1/memories` — save a memory
- `POST /v1/memories/search` — search with filters (tags, type, since, repo, scope)
- `PUT /v1/memories/{id}` — update
- `DELETE /v1/memories/{id}` — delete
- `GET /v1/memories/{id}` — get single memory

**Context:**
- `GET /v1/context` — org/team/user context (markdown from R2, derived from API key)

### Missing CLI commands
- `opc forget <id>` — delete a memory
- `opc update <id>` — update a memory
- `opc recall` with filters: `--tags`, `--type`, `--since`, `--limit`, `--scope`
- `opc enable --agent <name>` — replace `opc hooks` with multi-agent support
- `opc sync` — push buffered events to API (normally spawned automatically)

---

## Distribution

### Homebrew
- Code lives in `opscompanion/opc` (open source)
- Formula lives in `opscompanion/homebrew-opc`
- `homebrew-opc` has a GitHub Actions workflow that polls for new releases every 15 minutes and auto-updates the formula with correct sha256s
- No PAT required — `homebrew-opc` uses its own `GITHUB_TOKEN`
- GoReleaser in `opc` builds release tarballs on tagged pushes (`v*`)

### Install
```bash
brew tap opscompanion/opc
brew install opc
```

### Release flow
```bash
git tag v0.3.0
git push origin v0.3.0
# GitHub Actions runs GoReleaser → builds tarballs → creates release
# homebrew-opc workflow detects new release → updates formula → users get it on next brew update
```
