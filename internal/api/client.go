package api

import (
	"os"

	"github.com/opscompanion/opc/internal/models"
)

// Client is the interface for the OpsCompanion public API.
type Client interface {
	Verify() (*models.WhoAmIResponse, error)
	GetContext(includeComputedLinks bool) (*models.FullContext, error)
	SearchKnowledge(req models.KnowledgeSearchRequest) (*models.SearchResult, error)
	SearchMemory(req models.MemorySearchRequest) (*models.SearchResult, error)
	GetKnowledgeByPath(path string) (*models.KnowledgeDocument, error)
	PutKnowledgeByPath(path string, req models.KnowledgePathUpsertRequest) (*models.KnowledgeDocument, error)
}

// New returns the appropriate client implementation based on config.
// Uses mock client when api_key is "mock-key" or OPSCOMPANION_MOCK=true.
func New(cfg *models.Config) Client {
	if cfg.APIKey == "mock-key" || os.Getenv("OPSCOMPANION_MOCK") == "true" {
		return NewMockClient(cfg)
	}
	return NewHTTPClient(cfg)
}
