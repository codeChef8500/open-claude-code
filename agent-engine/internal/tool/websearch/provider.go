package websearch

import "context"

// SearchProvider is the interface for pluggable search backends.
// The default implementation uses DuckDuckGo's JSON API.
type SearchProvider interface {
	// Search performs a web search and returns structured results.
	Search(ctx context.Context, query string, maxResults int, allowedDomains, blockedDomains []string) ([]SearchHit, error)
	// Name returns the provider's human-readable name.
	Name() string
}
