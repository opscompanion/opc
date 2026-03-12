package api

import (
	"fmt"
	"os/user"
	"strings"
	"time"

	"github.com/opscompanion/opc/internal/models"
)

// MockClient returns realistic mock responses for all API operations.
type MockClient struct {
	cfg *models.Config
}

func NewMockClient(cfg *models.Config) *MockClient {
	return &MockClient{cfg: cfg}
}

func (m *MockClient) currentUser() string {
	if u, err := user.Current(); err == nil {
		return u.Username
	}
	return "unknown"
}

func (m *MockClient) org() string {
	return "acme-corp"
}

func (m *MockClient) Verify() error {
	return nil
}

func (m *MockClient) StartSession(session models.Session) (*models.FullContext, error) {
	return m.GetContext()
}

func (m *MockClient) StopSession(sessionID string) ([]models.Memory, error) {
	now := time.Now().UTC().Format(time.RFC3339)
	return []models.Memory{
		{
			ID:        "mem_auto_001",
			Type:      "Decision",
			Content:   "Use sliding-window rate limiting with Redis backend instead of token bucket",
			Tags:      []string{"api-gateway", "rate-limiting", "architecture"},
			Org:       m.org(),
			User:      m.currentUser(),
			CreatedAt: now,
			SessionID: sessionID,
		},
		{
			ID:        "mem_auto_002",
			Type:      "Discovery",
			Content:   "Datadog agent drops traces when payload exceeds 10MB — need to batch large spans",
			Tags:      []string{"observability", "datadog", "debugging"},
			Org:       m.org(),
			User:      m.currentUser(),
			CreatedAt: now,
			SessionID: sessionID,
		},
	}, nil
}

func (m *MockClient) ResumeSession(sessionID string) (*models.SessionResumeContext, error) {
	return &models.SessionResumeContext{
		LastActive: time.Now().Add(-2 * time.Hour).UTC().Format(time.RFC3339),
		WorkingOn:  "rate-limiter middleware for api-gateway",
		Branch:     "feat/rate-limiter",
		ModifiedFiles: []string{
			"src/middleware/rate-limiter.ts",
			"src/config/limits.ts",
			"tests/rate-limiter.test.ts",
		},
		Decisions: []string{
			"Sliding window algorithm chosen over token bucket for better burst handling",
			"Rate limit config stored in config-service, not hardcoded",
			"Using Redis for shared state across gateway instances",
		},
		OpenThreads: []string{
			"Need to add integration tests for rate limiter",
			"PR #47 pending review from @sarah",
		},
	}, nil
}

func (m *MockClient) Checkpoint(sessionID string) (*models.Checkpoint, error) {
	return &models.Checkpoint{
		SessionID:     sessionID,
		Compressed:    "Working on rate-limiter middleware. Decided on sliding window + Redis. Config via config-service. Integration tests pending.",
		Decisions:     []string{
			"Use sliding window over token bucket",
			"Store rate limit config in config-service",
			"Redis for cross-instance shared state",
			"Default limits: 1000 req/min auth, 100 req/min anon",
		},
		FilesModified: []string{
			"src/middleware/rate-limiter.ts",
			"src/config/limits.ts",
		},
	}, nil
}

func (m *MockClient) SaveMemory(content string, tags []string) (*models.Memory, error) {
	if len(tags) == 0 {
		tags = []string{"general", "manual-save"}
	}
	return &models.Memory{
		ID:        fmt.Sprintf("mem_%d", time.Now().Unix()),
		Type:      "Decision",
		Content:   content,
		Tags:      tags,
		Org:       m.org(),
		User:      m.currentUser(),
		CreatedAt: time.Now().UTC().Format(time.RFC3339),
	}, nil
}

func (m *MockClient) SearchMemories(query string) (*models.SearchResult, error) {
	q := strings.ToLower(query)

	switch {
	case strings.Contains(q, "rate") || strings.Contains(q, "limit"):
		return m.rateLimitResults(), nil
	case strings.Contains(q, "auth") || strings.Contains(q, "token") || strings.Contains(q, "jwt") || strings.Contains(q, "paseto"):
		return m.authResults(), nil
	case strings.Contains(q, "k8s") || strings.Contains(q, "kubernetes") || strings.Contains(q, "cluster") || strings.Contains(q, "upgrade"):
		return m.k8sResults(), nil
	default:
		return &models.SearchResult{
			Results:      nil,
			TotalMatches: 0,
			QueryTimeMS:  12,
		}, nil
	}
}

func (m *MockClient) rateLimitResults() *models.SearchResult {
	return &models.SearchResult{
		Results: []models.Memory{
			{
				ID:        "mem_rl_001",
				Type:      "Decision",
				Content:   "Use sliding-window rate limiting with Redis backend. Token bucket rejected — worse burst handling for our use case. Config lives in config-service, not hardcoded.",
				Tags:      []string{"api-gateway", "rate-limiting", "architecture"},
				Org:       m.org(),
				User:      "kenneth",
				CreatedAt: "2026-02-28T14:30:00Z",
				Relevance: 0.94,
			},
			{
				ID:        "mem_rl_002",
				Type:      "Discovery",
				Content:   "In-memory rate limit counters don't work with multiple gateway pods. Must use Redis for shared state across instances.",
				Tags:      []string{"api-gateway", "rate-limiting", "debugging"},
				Org:       m.org(),
				User:      "kenneth",
				CreatedAt: "2026-02-27T10:15:00Z",
				Relevance: 0.87,
			},
			{
				ID:        "mem_rl_003",
				Type:      "Decision",
				Content:   "Default rate limits: 1000 req/min for authenticated users, 100 req/min for anonymous. Configurable per-tenant via config-service.",
				Tags:      []string{"api-gateway", "rate-limiting", "config"},
				Org:       m.org(),
				User:      "sarah",
				CreatedAt: "2026-02-26T16:45:00Z",
				Relevance: 0.72,
			},
		},
		TotalMatches: 3,
		QueryTimeMS:  23,
	}
}

func (m *MockClient) authResults() *models.SearchResult {
	return &models.SearchResult{
		Results: []models.Memory{
			{
				ID:        "mem_auth_001",
				Type:      "Decision",
				Content:   "Migrating auth-service from JWT to PASETO v4 tokens. PASETO chosen for mandatory encryption and no algorithm confusion attacks. Target: 2026-03-01.",
				Tags:      []string{"auth-service", "security", "migration"},
				Org:       m.org(),
				User:      "kenneth",
				CreatedAt: "2026-02-15T09:00:00Z",
				Relevance: 0.96,
			},
			{
				ID:        "mem_auth_002",
				Type:      "Discovery",
				Content:   "PASETO v4 library (github.com/vk-rv/pvx) has no support for custom claims map. Need to use raw payload with manual marshaling.",
				Tags:      []string{"auth-service", "paseto", "debugging"},
				Org:       m.org(),
				User:      "kenneth",
				CreatedAt: "2026-02-20T11:30:00Z",
				Relevance: 0.89,
			},
			{
				ID:        "mem_auth_003",
				Type:      "Context",
				Content:   "Dual-token rollout: accept both JWT and PASETO during 2-week migration window. Feature flag: auth.paseto.enabled in config-service.",
				Tags:      []string{"auth-service", "migration", "feature-flags"},
				Org:       m.org(),
				User:      "sarah",
				CreatedAt: "2026-02-22T14:00:00Z",
				Relevance: 0.81,
			},
		},
		TotalMatches: 3,
		QueryTimeMS:  18,
	}
}

func (m *MockClient) k8sResults() *models.SearchResult {
	return &models.SearchResult{
		Results: []models.Memory{
			{
				ID:        "mem_k8s_001",
				Type:      "Decision",
				Content:   "K8s upgrade path: 1.28 → 1.29 → 1.30. Skip versions not supported by EKS. Control plane first, then node groups in us-east-1, then us-west-2.",
				Tags:      []string{"kubernetes", "infrastructure", "migration"},
				Org:       m.org(),
				User:      "kenneth",
				CreatedAt: "2026-02-10T08:00:00Z",
				Relevance: 0.92,
			},
			{
				ID:        "mem_k8s_002",
				Type:      "Discovery",
				Content:   "PodDisruptionBudget for auth-service was blocking node drain. Fixed by setting maxUnavailable to 1 instead of 0.",
				Tags:      []string{"kubernetes", "auth-service", "debugging"},
				Org:       m.org(),
				User:      "sarah",
				CreatedAt: "2026-02-18T16:20:00Z",
				Relevance: 0.85,
			},
		},
		TotalMatches: 2,
		QueryTimeMS:  15,
	}
}

func (m *MockClient) GetHistory() ([]models.HistoryEntry, error) {
	return []models.HistoryEntry{
		{
			SessionID: "ses_1739448821_a3f1",
			Agent:     "Claude Code",
			Summary:   "Fix flaky TestHTTPServer by increasing timeout",
			Trigger:   "git_commit",
			Detail:    map[string]any{"short_hash": "b68df0d", "branch": "feat/rate-limiter"},
			Decisions: []string{"Increased test timeout from 5s to 15s to handle CI cold starts"},
		},
		{
			SessionID: "ses_1739362410_c8e2",
			Agent:     "Claude Code",
			Summary:   "Add webhook retry logic with exponential backoff",
			Trigger:   "git_commit",
			Detail:    map[string]any{"short_hash": "f92e4d1", "branch": "feat/rate-limiter"},
			Decisions: []string{"Exponential backoff with jitter, max 3 retries, 30s cap"},
		},
		{
			SessionID: "ses_1739189605_9b4d",
			Agent:     "Claude Code",
			Summary:   "Expanded t.Parallel exception list for DB-dependent tests",
			Trigger:   "git_commit",
			Detail:    map[string]any{"short_hash": "a3c17e8", "branch": "main"},
		},
		{
			SessionID: "ses_1739103200_d7f3",
			Agent:     "Claude Code",
			Summary:   "Migrate config-service to Postgres adapter",
			Trigger:   "git_commit",
			Detail:    map[string]any{"short_hash": "e71a0b3", "branch": "feat/postgres-migration"},
			Decisions: []string{"Postgres over DynamoDB for config — need transactions and joins"},
		},
		{
			SessionID: "ses_1739016800_e5a2",
			Agent:     "Claude Code",
			Summary:   "File edits: auth-service token generation + tests",
			Trigger:   "periodic",
			Detail:    map[string]any{"event_count": 50},
			Decisions: []string{"PASETO v4 with local encryption, 1h expiry, refresh via /auth/refresh"},
		},
		{
			SessionID: "ses_1738930400_f6b3",
			Agent:     "Claude Code",
			Summary:   "Session ended after K8s Helm values update",
			Trigger:   "session_stop",
			Decisions: []string{"Pin kube-proxy image to v1.29.2, update API server flags"},
		},
	}, nil
}

func (m *MockClient) GetContext() (*models.FullContext, error) {
	return &models.FullContext{
		Org: models.OrgContext{
			Name:            m.org(),
			CloudProvider:   "AWS (us-east-1 primary, us-west-2 failover)",
			IaC:             "Terraform modules in infra/ monorepo",
			CICD:            "GitHub Actions → ArgoCD for k8s deploys",
			Observability:   "Datadog APM + Prometheus/Grafana for infra metrics",
			SecretsManager:  "AWS Secrets Manager, rotated via scheduled Lambda",
			IncidentProcess: "PagerDuty → Slack #incidents → Jira post-mortem",
		},
		Team: models.TeamContext{
			Name:              "platform-eng",
			Services:          []string{"api-gateway", "auth-service", "config-service", "internal-tools"},
			OnCallRotation:    "weekly, Sun 09:00 UTC handoff",
			DeploymentCadence: "continuous to staging, daily promote to prod (11:00 UTC)",
			ActiveProjects: []string{
				"Migrating auth-service from JWT to PASETO tokens (target: 2026-03-01)",
				"K8s cluster upgrade 1.28 → 1.30 (in progress, 60% complete)",
				"New rate-limiting middleware for api-gateway",
			},
		},
		User: models.UserContext{
			Name:       m.currentUser(),
			Role:       "Senior Platform Engineer",
			Prefs:      "prefers Terraform over Pulumi, uses bun for JS tooling",
			RecentWork: []string{"auth-service token migration", "api-gateway rate limiter prototype"},
			Editor:     "VS Code + Claude Code CLI",
			Shell:      "zsh with starship prompt",
		},
		Env: models.EnvContext{
			Branch:  "main",
			WorkDir: ".",
		},
	}, nil
}
