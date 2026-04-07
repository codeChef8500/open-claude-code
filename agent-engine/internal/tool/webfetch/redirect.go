package webfetch

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

const maxRedirects = 10

// RedirectInfo is returned when a cross-host redirect is detected that
// the tool cannot follow automatically.
type RedirectInfo struct {
	OriginalURL string `json:"original_url"`
	RedirectURL string `json:"redirect_url"`
	StatusCode  int    `json:"status_code"`
}

// isPermittedRedirect checks if a redirect is safe to follow automatically.
// Allows: same-origin, adding/removing "www." prefix, path/query changes.
func isPermittedRedirect(originalURL, redirectURL string) bool {
	orig, err := url.Parse(originalURL)
	if err != nil {
		return false
	}
	redir, err := url.Parse(redirectURL)
	if err != nil {
		return false
	}
	if redir.Scheme != orig.Scheme {
		return false
	}
	if redir.Port() != orig.Port() {
		return false
	}
	if redir.User != nil {
		return false
	}
	stripWWW := func(h string) string { return strings.TrimPrefix(h, "www.") }
	return stripWWW(orig.Hostname()) == stripWWW(redir.Hostname())
}

// fetchWithRedirects performs an HTTP GET with manual redirect handling.
// Safe same-host redirects are followed automatically (up to maxRedirects).
// Cross-host redirects return a *RedirectInfo for the caller to handle.
func fetchWithRedirects(client *http.Client, rawURL string, headers map[string]string, maxBodySize int64) (*http.Response, []byte, *RedirectInfo, error) {
	// Build a client that does NOT follow redirects automatically.
	noRedirectClient := *client
	noRedirectClient.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		return http.ErrUseLastResponse
	}

	currentURL := rawURL
	for depth := 0; depth < maxRedirects; depth++ {
		req, err := http.NewRequest(http.MethodGet, currentURL, nil)
		if err != nil {
			return nil, nil, nil, err
		}
		for k, v := range headers {
			req.Header.Set(k, v)
		}
		req.Header.Set("Accept", "text/markdown, text/html, */*")

		resp, err := noRedirectClient.Do(req)
		if err != nil {
			return nil, nil, nil, err
		}

		// Check for redirect status codes.
		if resp.StatusCode == 301 || resp.StatusCode == 302 || resp.StatusCode == 307 || resp.StatusCode == 308 {
			resp.Body.Close()
			loc := resp.Header.Get("Location")
			if loc == "" {
				return nil, nil, nil, fmt.Errorf("redirect missing Location header")
			}
			// Resolve relative URLs.
			base, _ := url.Parse(currentURL)
			resolved, err := base.Parse(loc)
			if err != nil {
				return nil, nil, nil, fmt.Errorf("invalid redirect Location: %w", err)
			}
			redirectURL := resolved.String()

			if isPermittedRedirect(currentURL, redirectURL) {
				currentURL = redirectURL
				continue
			}
			// Cross-host redirect — report back to caller.
			return nil, nil, &RedirectInfo{
				OriginalURL: rawURL,
				RedirectURL: redirectURL,
				StatusCode:  resp.StatusCode,
			}, nil
		}

		// Not a redirect — read body and return.
		body, err := io.ReadAll(io.LimitReader(resp.Body, maxBodySize))
		resp.Body.Close()
		if err != nil {
			return resp, nil, nil, err
		}
		return resp, body, nil, nil
	}
	return nil, nil, nil, fmt.Errorf("too many redirects (exceeded %d)", maxRedirects)
}
