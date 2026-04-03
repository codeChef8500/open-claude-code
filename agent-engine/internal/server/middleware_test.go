package server

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func okHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"ok":true}`))
	})
}

func panicHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic("test panic")
	})
}

// ── Recovery middleware ─────────────────────────────────────────────────────

func TestWithRecovery_NoPanic(t *testing.T) {
	h := withRecovery(okHandler())
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, httptest.NewRequest("GET", "/test", nil))
	if rr.Code != 200 {
		t.Errorf("expected 200, got %d", rr.Code)
	}
}

func TestWithRecovery_Panic(t *testing.T) {
	h := withRecovery(panicHandler())
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, httptest.NewRequest("GET", "/test", nil))
	if rr.Code != 500 {
		t.Errorf("expected 500 on panic, got %d", rr.Code)
	}
}

// ── Request logging middleware ───────────────────────────────────────────────

func TestWithRequestLogging(t *testing.T) {
	h := withRequestLogging(okHandler())
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, httptest.NewRequest("GET", "/test", nil))
	if rr.Code != 200 {
		t.Errorf("expected 200, got %d", rr.Code)
	}
}

// ── API Key auth middleware ─────────────────────────────────────────────────

func TestWithAPIKeyAuth_NoEnvVar(t *testing.T) {
	t.Setenv("AGENT_ENGINE_API_KEY", "")
	h := withAPIKeyAuth(okHandler())
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, httptest.NewRequest("GET", "/api/test", nil))
	if rr.Code != 200 {
		t.Errorf("expected 200 when no key required, got %d", rr.Code)
	}
}

func TestWithAPIKeyAuth_HealthBypass(t *testing.T) {
	t.Setenv("AGENT_ENGINE_API_KEY", "secret")
	h := withAPIKeyAuth(okHandler())
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, httptest.NewRequest("GET", "/health", nil))
	if rr.Code != 200 {
		t.Errorf("expected 200 for /health, got %d", rr.Code)
	}
}

func TestWithAPIKeyAuth_ValidKey(t *testing.T) {
	t.Setenv("AGENT_ENGINE_API_KEY", "secret123")
	h := withAPIKeyAuth(okHandler())
	req := httptest.NewRequest("GET", "/api/test", nil)
	req.Header.Set("Authorization", "Bearer secret123")
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != 200 {
		t.Errorf("expected 200 with valid key, got %d", rr.Code)
	}
}

func TestWithAPIKeyAuth_InvalidKey(t *testing.T) {
	t.Setenv("AGENT_ENGINE_API_KEY", "secret123")
	h := withAPIKeyAuth(okHandler())
	req := httptest.NewRequest("GET", "/api/test", nil)
	req.Header.Set("Authorization", "Bearer wrong")
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != 401 {
		t.Errorf("expected 401 with wrong key, got %d", rr.Code)
	}
}

func TestWithAPIKeyAuth_MissingKey(t *testing.T) {
	t.Setenv("AGENT_ENGINE_API_KEY", "secret123")
	h := withAPIKeyAuth(okHandler())
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, httptest.NewRequest("GET", "/api/test", nil))
	if rr.Code != 401 {
		t.Errorf("expected 401 with missing key, got %d", rr.Code)
	}
}

// ── CORS middleware ─────────────────────────────────────────────────────────

func TestWithCORS_DefaultOrigin(t *testing.T) {
	h := withCORS("")(okHandler())
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, httptest.NewRequest("GET", "/test", nil))
	if got := rr.Header().Get("Access-Control-Allow-Origin"); got != "*" {
		t.Errorf("expected '*', got %q", got)
	}
}

func TestWithCORS_CustomOrigin(t *testing.T) {
	h := withCORS("https://example.com")(okHandler())
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, httptest.NewRequest("GET", "/test", nil))
	if got := rr.Header().Get("Access-Control-Allow-Origin"); got != "https://example.com" {
		t.Errorf("expected 'https://example.com', got %q", got)
	}
}

func TestWithCORS_Preflight(t *testing.T) {
	h := withCORS("")(okHandler())
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, httptest.NewRequest("OPTIONS", "/test", nil))
	if rr.Code != http.StatusNoContent {
		t.Errorf("expected 204 for preflight, got %d", rr.Code)
	}
	if rr.Header().Get("Access-Control-Allow-Methods") == "" {
		t.Error("expected Allow-Methods header")
	}
}

// ── Rate limit middleware ───────────────────────────────────────────────────

func TestWithRateLimit_AllowsUnder(t *testing.T) {
	h := withRateLimit(5, time.Minute)(okHandler())
	for i := 0; i < 5; i++ {
		rr := httptest.NewRecorder()
		h.ServeHTTP(rr, httptest.NewRequest("GET", "/test", nil))
		if rr.Code != 200 {
			t.Errorf("request %d: expected 200, got %d", i, rr.Code)
		}
	}
}

func TestWithRateLimit_BlocksOver(t *testing.T) {
	h := withRateLimit(2, time.Minute)(okHandler())
	for i := 0; i < 3; i++ {
		rr := httptest.NewRecorder()
		h.ServeHTTP(rr, httptest.NewRequest("GET", "/test", nil))
		if i < 2 && rr.Code != 200 {
			t.Errorf("request %d: expected 200, got %d", i, rr.Code)
		}
		if i == 2 && rr.Code != 429 {
			t.Errorf("request %d: expected 429, got %d", i, rr.Code)
		}
	}
}

func TestWithRateLimit_RetryAfterHeader(t *testing.T) {
	h := withRateLimit(1, time.Minute)(okHandler())
	// First request OK.
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, httptest.NewRequest("GET", "/test", nil))
	// Second request should be rate limited.
	rr = httptest.NewRecorder()
	h.ServeHTTP(rr, httptest.NewRequest("GET", "/test", nil))
	if rr.Header().Get("Retry-After") != "60" {
		t.Errorf("expected Retry-After header, got %q", rr.Header().Get("Retry-After"))
	}
}

// ── loggingResponseWriter ───────────────────────────────────────────────────

func TestLoggingResponseWriter_CapturesStatus(t *testing.T) {
	rr := httptest.NewRecorder()
	lw := &loggingResponseWriter{ResponseWriter: rr, status: http.StatusOK}
	lw.WriteHeader(http.StatusNotFound)
	if lw.status != 404 {
		t.Errorf("expected 404, got %d", lw.status)
	}
}
