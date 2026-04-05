package daemon

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"time"
)

// ─── Session History ────────────────────────────────────────────────────────
// Ported from claude-code-main assistant/sessionHistory.ts.
// Fetches historical conversation events from the API for context restoration
// across KAIROS daemon restarts.

const (
	historyPageSize    = 50
	historyHTTPTimeout = 30 * time.Second
)

// HistoryAuthCtx holds authentication context for history fetching.
type HistoryAuthCtx struct {
	// BaseURL is the API base URL (e.g. https://api.claude.ai).
	BaseURL string
	// SessionID is the conversation session to fetch history for.
	SessionID string
	// AuthToken is the OAuth/API token.
	AuthToken string
	// HTTPClient overrides the default HTTP client.
	HTTPClient *http.Client
}

// HistoryEvent represents a single conversation event from history.
type HistoryEvent struct {
	ID        string                 `json:"id"`
	Type      string                 `json:"type"`
	Timestamp int64                  `json:"timestamp"`
	Data      map[string]interface{} `json:"data,omitempty"`
}

// HistoryPage is a paginated response of history events.
type HistoryPage struct {
	Events  []HistoryEvent `json:"events"`
	HasMore bool           `json:"has_more"`
	// OldestID is the ID of the oldest event in this page (for pagination).
	OldestID string `json:"oldest_id,omitempty"`
}

// CreateHistoryAuthCtx creates an auth context for history fetching.
func CreateHistoryAuthCtx(baseURL, sessionID, authToken string) *HistoryAuthCtx {
	return &HistoryAuthCtx{
		BaseURL:   baseURL,
		SessionID: sessionID,
		AuthToken: authToken,
		HTTPClient: &http.Client{
			Timeout: historyHTTPTimeout,
		},
	}
}

// FetchLatestEvents fetches the most recent events for the session.
func FetchLatestEvents(ctx context.Context, authCtx *HistoryAuthCtx, limit int) (*HistoryPage, error) {
	if limit <= 0 {
		limit = historyPageSize
	}

	endpoint := fmt.Sprintf("%s/sessions/%s/events?limit=%d",
		authCtx.BaseURL, url.PathEscape(authCtx.SessionID), limit)

	return fetchEvents(ctx, authCtx, endpoint)
}

// FetchOlderEvents fetches events older than the given event ID for pagination.
func FetchOlderEvents(ctx context.Context, authCtx *HistoryAuthCtx, beforeID string, limit int) (*HistoryPage, error) {
	if limit <= 0 {
		limit = historyPageSize
	}

	endpoint := fmt.Sprintf("%s/sessions/%s/events?limit=%d&before=%s",
		authCtx.BaseURL, url.PathEscape(authCtx.SessionID), limit,
		url.QueryEscape(beforeID))

	return fetchEvents(ctx, authCtx, endpoint)
}

// FetchAllEvents fetches all events by paginating backwards from the latest.
// Returns events in chronological order (oldest first).
func FetchAllEvents(ctx context.Context, authCtx *HistoryAuthCtx) ([]HistoryEvent, error) {
	var allEvents []HistoryEvent

	page, err := FetchLatestEvents(ctx, authCtx, historyPageSize)
	if err != nil {
		return nil, err
	}
	if page == nil {
		return nil, nil
	}

	allEvents = append(allEvents, page.Events...)

	for page.HasMore && page.OldestID != "" {
		if ctx.Err() != nil {
			return allEvents, ctx.Err()
		}

		page, err = FetchOlderEvents(ctx, authCtx, page.OldestID, historyPageSize)
		if err != nil {
			slog.Warn("history: pagination failed, returning partial results",
				slog.Any("err", err))
			break
		}
		if page == nil || len(page.Events) == 0 {
			break
		}
		allEvents = append(allEvents, page.Events...)
	}

	// Reverse to chronological order
	for i, j := 0, len(allEvents)-1; i < j; i, j = i+1, j-1 {
		allEvents[i], allEvents[j] = allEvents[j], allEvents[i]
	}

	return allEvents, nil
}

// ─── Internal ───────────────────────────────────────────────────────────────

func fetchEvents(ctx context.Context, authCtx *HistoryAuthCtx, endpoint string) (*HistoryPage, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Accept", "application/json")
	if authCtx.AuthToken != "" {
		req.Header.Set("Authorization", "Bearer "+authCtx.AuthToken)
	}

	client := authCtx.HTTPClient
	if client == nil {
		client = &http.Client{Timeout: historyHTTPTimeout}
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch events: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, nil // session not found — fresh start
	}

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("history API %d: %s", resp.StatusCode, string(body))
	}

	var page HistoryPage
	if err := json.NewDecoder(resp.Body).Decode(&page); err != nil {
		return nil, fmt.Errorf("decode history: %w", err)
	}

	return &page, nil
}
