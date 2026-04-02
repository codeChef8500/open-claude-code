package server

import (
	"log/slog"
	"net/http"
	"os"
	"runtime/debug"
	"sync"
	"time"
)

// withRecovery wraps a handler with panic recovery, returning 500 on any panic.
func withRecovery(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rec := recover(); rec != nil {
				slog.Error("http handler panic",
					slog.Any("error", rec),
					slog.String("path", r.URL.Path),
					slog.String("stack", string(debug.Stack())),
				)
				http.Error(w, `{"error":"internal server error"}`, http.StatusInternalServerError)
			}
		}()
		next.ServeHTTP(w, r)
	})
}

// withRequestLogging logs method, path, status, and duration for each request.
func withRequestLogging(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		lw := &loggingResponseWriter{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(lw, r)
		slog.Info("http",
			slog.String("method", r.Method),
			slog.String("path", r.URL.Path),
			slog.Int("status", lw.status),
			slog.Duration("duration", time.Since(start)),
		)
	})
}

// withAPIKeyAuth enforces Bearer token authentication when AGENT_ENGINE_API_KEY
// is set in the environment.  If the env var is unset, auth is skipped.
func withAPIKeyAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Health endpoint is always public.
		if r.URL.Path == "/health" {
			next.ServeHTTP(w, r)
			return
		}

		required := os.Getenv("AGENT_ENGINE_API_KEY")
		if required == "" {
			next.ServeHTTP(w, r)
			return
		}

		token := r.Header.Get("Authorization")
		if len(token) > 7 && token[:7] == "Bearer " {
			token = token[7:]
		}
		if token != required {
			writeError(w, http.StatusUnauthorized, "invalid or missing API key")
			return
		}
		next.ServeHTTP(w, r)
	})
}

// withCORS adds Cross-Origin Resource Sharing headers.
func withCORS(allowedOrigins string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origin := allowedOrigins
			if origin == "" {
				origin = "*"
			}
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Request-ID")
			w.Header().Set("Access-Control-Max-Age", "86400")

			if r.Method == http.MethodOptions {
				w.WriteHeader(http.StatusNoContent)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// withRateLimit is a simple per-IP rate limiter using a sliding window.
// maxRequests is the max number of requests per window from a single IP.
func withRateLimit(maxRequests int, window time.Duration) func(http.Handler) http.Handler {
	type entry struct {
		count   int
		resetAt time.Time
	}
	var mu sync.Mutex
	clients := make(map[string]*entry)

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ip := r.RemoteAddr
			now := time.Now()

			mu.Lock()
			e, ok := clients[ip]
			if !ok || now.After(e.resetAt) {
				e = &entry{count: 0, resetAt: now.Add(window)}
				clients[ip] = e
			}
			e.count++
			over := e.count > maxRequests
			mu.Unlock()

			if over {
				w.Header().Set("Retry-After", "60")
				writeError(w, http.StatusTooManyRequests, "rate limit exceeded")
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// loggingResponseWriter captures the HTTP status code for logging.
type loggingResponseWriter struct {
	http.ResponseWriter
	status int
}

func (lw *loggingResponseWriter) WriteHeader(code int) {
	lw.status = code
	lw.ResponseWriter.WriteHeader(code)
}
