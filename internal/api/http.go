package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/opscompanion/opc/internal/models"
)

// HTTPClient talks to the real OpsCompanion API.
type HTTPClient struct {
	baseURL    string
	apiKey     string
	httpClient *http.Client
}

func NewHTTPClient(cfg *models.Config) *HTTPClient {
	return &HTTPClient{
		baseURL: cfg.APIURL,
		apiKey:  cfg.APIKey,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// do executes an HTTP request with auth headers and returns the response body.
func (c *HTTPClient) do(ctx context.Context, method, path string, body any) ([]byte, error) {
	var reqBody io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("marshaling request: %w", err)
		}
		reqBody = bytes.NewReader(data)
	}

	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, reqBody)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading response: %w", err)
	}

	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("API error %d: %s", resp.StatusCode, string(respBody))
	}

	return respBody, nil
}

func (c *HTTPClient) Verify() error {
	_, err := c.do(context.Background(), http.MethodGet, "/verify", nil)
	return err
}

func (c *HTTPClient) StartSession(session models.Session) (*models.FullContext, error) {
	data, err := c.do(context.Background(), http.MethodPost, "/sessions", session)
	if err != nil {
		return nil, err
	}
	var fc models.FullContext
	if err := json.Unmarshal(data, &fc); err != nil {
		return nil, fmt.Errorf("parsing response: %w", err)
	}
	return &fc, nil
}

func (c *HTTPClient) StopSession(sessionID string) ([]models.Memory, error) {
	data, err := c.do(context.Background(), http.MethodPost, "/sessions/"+sessionID+"/stop", nil)
	if err != nil {
		return nil, err
	}
	var memories []models.Memory
	if err := json.Unmarshal(data, &memories); err != nil {
		return nil, fmt.Errorf("parsing response: %w", err)
	}
	return memories, nil
}

func (c *HTTPClient) ResumeSession(sessionID string) (*models.SessionResumeContext, error) {
	data, err := c.do(context.Background(), http.MethodPost, "/sessions/"+sessionID+"/resume", nil)
	if err != nil {
		return nil, err
	}
	var rc models.SessionResumeContext
	if err := json.Unmarshal(data, &rc); err != nil {
		return nil, fmt.Errorf("parsing response: %w", err)
	}
	return &rc, nil
}

func (c *HTTPClient) Checkpoint(sessionID string) (*models.Checkpoint, error) {
	data, err := c.do(context.Background(), http.MethodPost, "/sessions/"+sessionID+"/checkpoint", nil)
	if err != nil {
		return nil, err
	}
	var cp models.Checkpoint
	if err := json.Unmarshal(data, &cp); err != nil {
		return nil, fmt.Errorf("parsing response: %w", err)
	}
	return &cp, nil
}

func (c *HTTPClient) SaveMemory(content string, tags []string) (*models.Memory, error) {
	payload := map[string]any{"content": content, "tags": tags}
	data, err := c.do(context.Background(), http.MethodPost, "/memories", payload)
	if err != nil {
		return nil, err
	}
	var mem models.Memory
	if err := json.Unmarshal(data, &mem); err != nil {
		return nil, fmt.Errorf("parsing response: %w", err)
	}
	return &mem, nil
}

func (c *HTTPClient) SearchMemories(query string) (*models.SearchResult, error) {
	payload := map[string]string{"query": query}
	data, err := c.do(context.Background(), http.MethodPost, "/memories/search", payload)
	if err != nil {
		return nil, err
	}
	var sr models.SearchResult
	if err := json.Unmarshal(data, &sr); err != nil {
		return nil, fmt.Errorf("parsing response: %w", err)
	}
	return &sr, nil
}

func (c *HTTPClient) GetHistory() ([]models.HistoryEntry, error) {
	data, err := c.do(context.Background(), http.MethodGet, "/history", nil)
	if err != nil {
		return nil, err
	}
	var entries []models.HistoryEntry
	if err := json.Unmarshal(data, &entries); err != nil {
		return nil, fmt.Errorf("parsing response: %w", err)
	}
	return entries, nil
}

func (c *HTTPClient) GetContext() (*models.FullContext, error) {
	data, err := c.do(context.Background(), http.MethodGet, "/context", nil)
	if err != nil {
		return nil, err
	}
	var fc models.FullContext
	if err := json.Unmarshal(data, &fc); err != nil {
		return nil, fmt.Errorf("parsing response: %w", err)
	}
	return &fc, nil
}
