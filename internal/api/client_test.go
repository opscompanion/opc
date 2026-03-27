package api

import (
	"testing"

	"github.com/opscompanion/opc/internal/models"
)

func TestNewChoosesMockOrHTTPClient(t *testing.T) {
	t.Setenv("OPSCOMPANION_MOCK", "")

	if _, ok := New(&models.Config{APIKey: "mock-key"}).(*MockClient); !ok {
		t.Fatal("mock-key should return MockClient")
	}
	if _, ok := New(&models.Config{APIKey: "real"}).(*HTTPClient); !ok {
		t.Fatal("real key should return HTTPClient")
	}

	t.Setenv("OPSCOMPANION_MOCK", "true")
	if _, ok := New(&models.Config{APIKey: "real"}).(*MockClient); !ok {
		t.Fatal("env override should return MockClient")
	}
}

func TestMockHelpersAndSearchKnowledge(t *testing.T) {
	client := NewMockClient(&models.Config{})

	result, err := client.SearchKnowledge(models.KnowledgeSearchRequest{Scope: "both"})
	if err != nil {
		t.Fatalf("SearchKnowledge: %v", err)
	}
	if len(result.Results) != 2 {
		t.Fatalf("len(SearchKnowledge.Results) = %d, want 2", len(result.Results))
	}
	if maxString("", "fallback") != "fallback" {
		t.Fatal("maxString should use fallback for blank value")
	}
	if pathDir("runbooks/deploy.md") != "runbooks" {
		t.Fatalf("pathDir returned %q", pathDir("runbooks/deploy.md"))
	}
	if pathBase("runbooks/deploy.md") != "deploy.md" {
		t.Fatalf("pathBase returned %q", pathBase("runbooks/deploy.md"))
	}
}
