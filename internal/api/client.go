package api

import (
	"github.com/opscompanion/opsctl/internal/models"
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
	LinkCommit(sessionID string, commit models.CommitRecord) error

	// Memories
	SaveMemory(content string, tags []string) (*models.Memory, error)
	SearchMemories(query string) (*models.SearchResult, error)

	// History
	GetHistory() ([]models.HistoryEntry, error)

	// Context
	GetContext() (*models.FullContext, error)
}

// New returns the appropriate client implementation based on config.
func New(cfg *models.Config) Client {
	// TODO: return real HTTP client when backend exists
	return NewMockClient(cfg)
}
