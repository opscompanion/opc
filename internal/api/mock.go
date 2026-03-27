package api

import (
	"fmt"
	"os/user"
	"strings"
	"time"

	"github.com/opscompanion/opc/internal/models"
)

// MockClient returns realistic mock responses for supported public API operations.
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

func (m *MockClient) Verify() (*models.WhoAmIResponse, error) {
	return &models.WhoAmIResponse{
		APIKey: models.APIKeyIdentity{
			PublicID:  "key_mock_public",
			KeyPrefix: "opc_mock",
			OwnerType: "user",
			Scopes: []string{
				"organization:integration:read",
				"organization:workspace:read",
				"organization:memory:read",
				"organization:knowledge:read",
				"organization:knowledge:write",
				"user:memory:read",
				"user:memory:write",
			},
		},
		Organization: models.PublicIdentity{PublicID: "org_mock_public"},
		User:         &models.PublicIdentity{PublicID: "user_mock_public"},
	}, nil
}

func (m *MockClient) GetContext(includeComputedLinks bool) (*models.FullContext, error) {
	orgName := "OpsCompanion"
	first := m.currentUser()
	email := fmt.Sprintf("%s@example.com", first)
	ctx := &models.FullContext{
		Organization: models.OrganizationContext{
			PublicID: "org_mock_public",
			Name:     &orgName,
		},
		User: &models.UserContext{
			PublicID:  "user_mock_public",
			FirstName: &first,
			LastName:  nil,
			Email:     &email,
		},
		Integrations: []models.Integration{
			{PublicID: "int_aws", Provider: "amazon-web-services", IsActive: true},
			{PublicID: "int_github", Provider: "github", IsActive: true},
		},
		Workspaces: []models.Workspace{
			{
				PublicID:  "ws_prod",
				Name:      "Production",
				Color:     "#0285FF",
				NodeCount: 2,
				Nodes: []models.WorkspaceNode{
					{PublicID: "node_ec2_a", ExternalID: "us-east-2/vm/i-123", Provider: "amazon-web-services", X: 0, Y: 60},
					{PublicID: "node_ec2_b", ExternalID: "us-east-2/vm/i-456", Provider: "amazon-web-services", X: 600, Y: 80},
				},
				Links: []models.WorkspaceLink{
					{PublicID: "link_1", NodeAPublicID: "node_ec2_a", NodeBPublicID: "node_ec2_b"},
				},
			},
		},
	}
	if includeComputedLinks {
		ctx.Workspaces[0].ComputedLinks = []models.ComputedLink{
			{
				SourcePublicID:      "node_ec2_a",
				TargetPublicID:      "node_ec2_b",
				Reason:              "Shared VPC vpc-123",
				Provider:            "amazon-web-services",
				IntegrationPublicID: "int_aws",
			},
		}
	}
	orgMemory := "# Mock org memory\n\nRemember the release checklist."
	userMemory := "# Mock user memory\n\nPrefers concise answers."
	ctx.Memory.Organization.Content = &orgMemory
	ctx.Memory.User.Content = &userMemory
	return ctx, nil
}

func (m *MockClient) SearchKnowledge(req models.KnowledgeSearchRequest) (*models.SearchResult, error) {
	items := []models.SearchResultItem{
		{
			SourceType: "knowledge",
			Scope:      maxString(req.Scope, "organization"),
			Path:       "runbooks/deploy.md",
			Name:       "deploy.md",
			MatchCount: 2,
			Snippet:    "Use canary first, then promote after health checks pass.",
		},
	}
	if strings.EqualFold(req.Scope, "both") {
		items = append(items, models.SearchResultItem{
			SourceType: "memory",
			Scope:      "current",
			Path:       "memory.md",
			MatchCount: 1,
			Snippet:    "Deployment notes mention the canary sequence.",
		})
	}
	return &models.SearchResult{Results: items}, nil
}

func (m *MockClient) SearchMemory(req models.MemorySearchRequest) (*models.SearchResult, error) {
	return &models.SearchResult{
		Results: []models.SearchResultItem{
			{
				SourceType: "memory",
				Scope:      "current",
				Path:       "memory.md",
				MatchCount: 1,
				Snippet:    fmt.Sprintf("Mock memory hit for %q", req.Query),
			},
		},
	}, nil
}

func (m *MockClient) GetKnowledgeByPath(path string) (*models.KnowledgeDocument, error) {
	if strings.Contains(path, "missing") {
		return nil, &APIError{StatusCode: 404, Body: `{"error":"not found"}`}
	}
	return &models.KnowledgeDocument{
		File: models.KnowledgeFile{
			PublicID:  "file_mock_public",
			Path:      pathDir(path),
			Name:      pathBase(path),
			CreatedAt: time.Now().Add(-24 * time.Hour).UTC().Format(time.RFC3339),
			UpdatedAt: time.Now().UTC().Format(time.RFC3339),
		},
		Version: &models.KnowledgeVersion{
			PublicID:    "ver_mock_public",
			SHA256Hash:  "mock",
			ByteSize:    64,
			TokenCount:  12,
			ContentType: "text/markdown",
			CreatedAt:   time.Now().UTC().Format(time.RFC3339),
			UpdatedAt:   time.Now().UTC().Format(time.RFC3339),
		},
		Content: "## Previous entry\n\nMock knowledge content.",
	}, nil
}

func (m *MockClient) PutKnowledgeByPath(path string, req models.KnowledgePathUpsertRequest) (*models.KnowledgeDocument, error) {
	doc, _ := m.GetKnowledgeByPath(path)
	if doc == nil {
		doc = &models.KnowledgeDocument{}
	}
	doc.File = models.KnowledgeFile{
		PublicID:  "file_mock_public",
		Path:      pathDir(path),
		Name:      pathBase(path),
		CreatedAt: time.Now().Add(-24 * time.Hour).UTC().Format(time.RFC3339),
		UpdatedAt: time.Now().UTC().Format(time.RFC3339),
	}
	doc.Version = &models.KnowledgeVersion{
		PublicID:    "ver_mock_public",
		SHA256Hash:  "mock",
		ByteSize:    len(req.Content),
		TokenCount:  12,
		ContentType: "text/markdown",
		CreatedAt:   time.Now().UTC().Format(time.RFC3339),
		UpdatedAt:   time.Now().UTC().Format(time.RFC3339),
	}
	doc.Content = req.Content
	return doc, nil
}

func maxString(v, fallback string) string {
	if strings.TrimSpace(v) == "" {
		return fallback
	}
	return v
}

func pathDir(path string) string {
	if i := strings.LastIndex(path, "/"); i >= 0 {
		return path[:i]
	}
	return ""
}

func pathBase(path string) string {
	if i := strings.LastIndex(path, "/"); i >= 0 {
		return path[i+1:]
	}
	return path
}
