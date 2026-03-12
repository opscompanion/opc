package api

import (
	"os"

	"github.com/opscompanion/opc/internal/models"
)

// Client is the interface for the OpsCompanion API.
type Client interface {
	// Auth
	Verify() error

	// Sessions
	StartSession(session models.Session) (*models.FullContext, error)
	StopSession(sessionID string) ([]models.Memory, error)
	ResumeSession(sessionID string) (*models.SessionResumeContext, error)
	Checkpoint(sessionID string) (*models.Checkpoint, error)

	// Memories
	SaveMemory(content string, tags []string) (*models.Memory, error)
	SearchMemories(query string) (*models.SearchResult, error)

	// History
	GetHistory() ([]models.HistoryEntry, error)

	// Context
	GetContext() (*models.FullContext, error)
}

// New returns the appropriate client implementation based on config.
// Uses mock client when api_key is "mock-key" or OPSCOMPANION_MOCK=true.
func New(cfg *models.Config) Client {
	if cfg.APIKey == "mock-key" || os.Getenv("OPSCOMPANION_MOCK") == "true" {
		return NewMockClient(cfg)
	}
	return NewHTTPClient(cfg)
}
