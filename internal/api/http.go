package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/opscompanion/opc/internal/config"
	"github.com/opscompanion/opc/internal/models"
)

// APIError is returned for non-2xx responses.
type APIError struct {
	StatusCode int
	Body       string
}

func (e *APIError) Error() string {
	return fmt.Sprintf("API error %d: %s", e.StatusCode, e.Body)
}

// HTTPClient talks to the real OpsCompanion API.
type HTTPClient struct {
	baseURL    string
	apiKey     string
	httpClient *http.Client
}

func NewHTTPClient(cfg *models.Config) *HTTPClient {
	return &HTTPClient{
		baseURL: config.ResolveAPIURL(cfg),
		apiKey:  cfg.APIKey,
		httpClient: &http.Client{
			Timeout: 15 * time.Second,
		},
	}
}

func (c *HTTPClient) do(ctx context.Context, method, path string, query url.Values, body any) ([]byte, error) {
	var reqBody io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("marshaling request: %w", err)
		}
		reqBody = bytes.NewReader(data)
	}

	u := c.baseURL + path
	if len(query) > 0 {
		u += "?" + query.Encode()
	}

	req, err := http.NewRequestWithContext(ctx, method, u, reqBody)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

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
		return nil, &APIError{StatusCode: resp.StatusCode, Body: string(respBody)}
	}

	return respBody, nil
}

func (c *HTTPClient) Verify() (*models.WhoAmIResponse, error) {
	data, err := c.do(context.Background(), http.MethodGet, "/whoami", nil, nil)
	if err != nil {
		return nil, err
	}
	var resp models.WhoAmIResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, fmt.Errorf("parsing response: %w", err)
	}
	return &resp, nil
}

func (c *HTTPClient) GetContext(includeComputedLinks bool) (*models.FullContext, error) {
	query := url.Values{}
	if includeComputedLinks {
		query.Set("includeComputedLinks", "true")
	}
	data, err := c.do(context.Background(), http.MethodGet, "/context", query, nil)
	if err != nil {
		return nil, err
	}

	type rawUnauthorized = models.UnauthorizedSection
	type rawMemory struct {
		Organization json.RawMessage `json:"organization"`
		User         json.RawMessage `json:"user"`
	}
	type rawContext struct {
		Organization models.OrganizationContext `json:"organization"`
		User         *models.UserContext        `json:"user"`
		Integrations json.RawMessage            `json:"integrations"`
		Workspaces   json.RawMessage            `json:"workspaces"`
		Memory       rawMemory                  `json:"memory"`
	}

	var raw rawContext
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("parsing response: %w", err)
	}

	ctx := &models.FullContext{
		Organization: raw.Organization,
		User:         raw.User,
	}

	if unauthorized, ok := decodeUnauthorized(raw.Integrations); ok {
		ctx.IntegrationsUnauthorized = unauthorized
	} else if err := json.Unmarshal(raw.Integrations, &ctx.Integrations); err != nil {
		return nil, fmt.Errorf("parsing integrations: %w", err)
	}

	if unauthorized, ok := decodeUnauthorized(raw.Workspaces); ok {
		ctx.WorkspacesUnauthorized = unauthorized
	} else if err := json.Unmarshal(raw.Workspaces, &ctx.Workspaces); err != nil {
		return nil, fmt.Errorf("parsing workspaces: %w", err)
	}

	if unauthorized, ok := decodeUnauthorized(raw.Memory.Organization); ok {
		ctx.Memory.Organization.Unauthorized = unauthorized
	} else if isJSONNull(raw.Memory.Organization) {
		ctx.Memory.Organization.Content = nil
	} else {
		var content string
		if err := json.Unmarshal(raw.Memory.Organization, &content); err != nil {
			return nil, fmt.Errorf("parsing organization memory: %w", err)
		}
		ctx.Memory.Organization.Content = &content
	}

	if unauthorized, ok := decodeUnauthorized(raw.Memory.User); ok {
		ctx.Memory.User.Unauthorized = unauthorized
	} else if isJSONNull(raw.Memory.User) {
		ctx.Memory.User.Content = nil
	} else {
		var content string
		if err := json.Unmarshal(raw.Memory.User, &content); err != nil {
			return nil, fmt.Errorf("parsing user memory: %w", err)
		}
		ctx.Memory.User.Content = &content
	}

	return ctx, nil
}

func (c *HTTPClient) SearchKnowledge(req models.KnowledgeSearchRequest) (*models.SearchResult, error) {
	data, err := c.do(context.Background(), http.MethodPost, "/knowledge/search", nil, req)
	if err != nil {
		return nil, err
	}
	var resp models.SearchResult
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, fmt.Errorf("parsing response: %w", err)
	}
	return &resp, nil
}

func (c *HTTPClient) SearchMemory(req models.MemorySearchRequest) (*models.SearchResult, error) {
	data, err := c.do(context.Background(), http.MethodPost, "/memory/search", nil, req)
	if err != nil {
		return nil, err
	}
	var resp models.SearchResult
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, fmt.Errorf("parsing response: %w", err)
	}
	return &resp, nil
}

func (c *HTTPClient) SearchLogs(req models.ObservabilitySearchRequest) (*models.LogsResult, error) {
	data, err := c.do(context.Background(), http.MethodPost, "/logs/search", nil, req)
	if err != nil {
		return nil, err
	}
	var resp models.LogsResult
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, fmt.Errorf("parsing response: %w", err)
	}
	return &resp, nil
}

func (c *HTTPClient) TailLogs(req models.ObservabilitySearchRequest) (*models.LogsResult, error) {
	data, err := c.do(context.Background(), http.MethodPost, "/logs/tail", nil, req)
	if err != nil {
		return nil, err
	}
	var resp models.LogsResult
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, fmt.Errorf("parsing response: %w", err)
	}
	return &resp, nil
}

func (c *HTTPClient) SearchTraces(req models.ObservabilitySearchRequest) (*models.TracesResult, error) {
	data, err := c.do(context.Background(), http.MethodPost, "/traces/search", nil, req)
	if err != nil {
		return nil, err
	}
	var resp models.TracesResult
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, fmt.Errorf("parsing response: %w", err)
	}
	return &resp, nil
}

func (c *HTTPClient) GetKnowledgeByPath(path string) (*models.KnowledgeDocument, error) {
	data, err := c.do(context.Background(), http.MethodGet, "/knowledge/path/"+escapePath(path), nil, nil)
	if err != nil {
		return nil, err
	}
	var resp models.KnowledgeDocument
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, fmt.Errorf("parsing response: %w", err)
	}
	return &resp, nil
}

func (c *HTTPClient) PutKnowledgeByPath(path string, req models.KnowledgePathUpsertRequest) (*models.KnowledgeDocument, error) {
	data, err := c.do(context.Background(), http.MethodPut, "/knowledge/path/"+escapePath(path), nil, req)
	if err != nil {
		return nil, err
	}
	var resp models.KnowledgeDocument
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, fmt.Errorf("parsing response: %w", err)
	}
	return &resp, nil
}

func decodeUnauthorized(raw json.RawMessage) (*models.UnauthorizedSection, bool) {
	var unauthorized models.UnauthorizedSection
	if err := json.Unmarshal(raw, &unauthorized); err == nil && unauthorized.Unauthorized {
		return &unauthorized, true
	}
	return nil, false
}

func isJSONNull(raw json.RawMessage) bool {
	return strings.TrimSpace(string(raw)) == "null"
}

func escapePath(path string) string {
	parts := strings.Split(strings.Trim(path, "/"), "/")
	for i := range parts {
		parts[i] = url.PathEscape(parts[i])
	}
	return strings.Join(parts, "/")
}
