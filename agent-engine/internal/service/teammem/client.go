package teammem

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"time"
)

// ────────────────────────────────────────────────────────────────────────────
// Team memory HTTP client — aligned with claude-code-main
// src/services/teamMemorySync/index.ts (HTTP layer)
// ────────────────────────────────────────────────────────────────────────────

const (
	defaultBaseURL   = "https://api.claude.ai"
	teamMemEndpoint  = "/api/claude_code/team_memory"
	defaultTimeoutMs = 15_000
)

// TokenProvider returns an OAuth token for API authentication.
type TokenProvider func(ctx context.Context) (string, error)

// TeamMemClient is the HTTP client for team memory sync API.
type TeamMemClient struct {
	baseURL       string
	httpClient    *http.Client
	tokenProvider TokenProvider
}

// ClientOption configures a TeamMemClient.
type ClientOption func(*TeamMemClient)

// WithBaseURL sets a custom API base URL.
func WithBaseURL(url string) ClientOption {
	return func(c *TeamMemClient) { c.baseURL = url }
}

// WithHTTPClient sets a custom http.Client.
func WithHTTPClient(hc *http.Client) ClientOption {
	return func(c *TeamMemClient) { c.httpClient = hc }
}

// NewTeamMemClient creates a new team memory API client.
func NewTeamMemClient(tokenProvider TokenProvider, opts ...ClientOption) *TeamMemClient {
	c := &TeamMemClient{
		baseURL:       defaultBaseURL,
		tokenProvider: tokenProvider,
		httpClient: &http.Client{
			Timeout: time.Duration(defaultTimeoutMs) * time.Millisecond,
		},
	}
	for _, opt := range opts {
		opt(c)
	}
	return c
}

// FetchTeamMemory fetches all team memory entries from the server.
// Supports ETag-based caching (If-None-Match).
func (c *TeamMemClient) FetchTeamMemory(ctx context.Context, state *SyncState) (*FetchResult, error) {
	u := c.buildURL(state.RepoSlug)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, &SyncError{Kind: SyncErrorNetwork, Message: "build request", Cause: err}
	}

	if err := c.setAuth(ctx, req); err != nil {
		return nil, err
	}

	if state.ETag != "" {
		req.Header.Set("If-None-Match", state.ETag)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, &SyncError{Kind: SyncErrorNetwork, Message: "fetch team memory", Cause: err}
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusOK:
		var result FetchResult
		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			return nil, &SyncError{Kind: SyncErrorServer, Message: "decode response", Cause: err}
		}
		result.ETag = resp.Header.Get("ETag")
		return &result, nil

	case http.StatusNotModified:
		return &FetchResult{ETag: state.ETag}, nil

	case http.StatusNotFound:
		return &FetchResult{NotFound: true}, nil

	case http.StatusUnauthorized, http.StatusForbidden:
		return nil, &SyncError{Kind: SyncErrorAuth, Message: fmt.Sprintf("auth error: %d", resp.StatusCode)}

	default:
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return nil, &SyncError{
			Kind:    SyncErrorServer,
			Message: fmt.Sprintf("server error %d: %s", resp.StatusCode, string(body)),
		}
	}
}

// FetchTeamMemoryHashes fetches only per-key checksums from the server.
// Used for conflict detection (412 probe).
func (c *TeamMemClient) FetchTeamMemoryHashes(ctx context.Context, state *SyncState) (map[string]string, error) {
	u := c.buildURL(state.RepoSlug) + "&hashes_only=true"

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, &SyncError{Kind: SyncErrorNetwork, Message: "build hash request", Cause: err}
	}

	if err := c.setAuth(ctx, req); err != nil {
		return nil, err
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, &SyncError{Kind: SyncErrorNetwork, Message: "fetch hashes", Cause: err}
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, &SyncError{
			Kind:    SyncErrorServer,
			Message: fmt.Sprintf("fetch hashes: status %d", resp.StatusCode),
		}
	}

	var result struct {
		Hashes map[string]string `json:"hashes"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, &SyncError{Kind: SyncErrorServer, Message: "decode hashes", Cause: err}
	}
	return result.Hashes, nil
}

// UploadTeamMemory uploads team memory entries to the server.
// Uses If-Match for optimistic concurrency control.
func (c *TeamMemClient) UploadTeamMemory(ctx context.Context, state *SyncState, entries []DeltaEntry, etag string) (*PushResult, error) {
	u := c.buildURL(state.RepoSlug)

	body, err := json.Marshal(map[string]interface{}{
		"entries": entries,
	})
	if err != nil {
		return nil, &SyncError{Kind: SyncErrorLocal, Message: "marshal entries", Cause: err}
	}

	if len(body) > MaxPutBodySize {
		return nil, &SyncError{
			Kind:    SyncErrorLocal,
			Message: fmt.Sprintf("payload too large: %d bytes (max %d)", len(body), MaxPutBodySize),
		}
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPut, u, bytes.NewReader(body))
	if err != nil {
		return nil, &SyncError{Kind: SyncErrorNetwork, Message: "build upload request", Cause: err}
	}

	req.Header.Set("Content-Type", "application/json")
	if etag != "" {
		req.Header.Set("If-Match", etag)
	}

	if err := c.setAuth(ctx, req); err != nil {
		return nil, err
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, &SyncError{Kind: SyncErrorNetwork, Message: "upload team memory", Cause: err}
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusOK, http.StatusCreated, http.StatusNoContent:
		newETag := resp.Header.Get("ETag")
		return &PushResult{Success: true, ETag: newETag}, nil

	case http.StatusPreconditionFailed:
		slog.Info("team memory upload: 412 conflict")
		return &PushResult{Conflict: true}, nil

	case http.StatusUnauthorized, http.StatusForbidden:
		return nil, &SyncError{Kind: SyncErrorAuth, Message: fmt.Sprintf("auth error: %d", resp.StatusCode)}

	default:
		respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return nil, &SyncError{
			Kind:    SyncErrorServer,
			Message: fmt.Sprintf("upload error %d: %s", resp.StatusCode, string(respBody)),
		}
	}
}

// ── Helpers ────────────────────────────────────────────────────────────────

func (c *TeamMemClient) buildURL(repoSlug string) string {
	return fmt.Sprintf("%s%s?repo=%s", c.baseURL, teamMemEndpoint, url.QueryEscape(repoSlug))
}

func (c *TeamMemClient) setAuth(ctx context.Context, req *http.Request) error {
	if c.tokenProvider == nil {
		return nil
	}
	token, err := c.tokenProvider(ctx)
	if err != nil {
		return &SyncError{Kind: SyncErrorAuth, Message: "get oauth token", Cause: err}
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	return nil
}
