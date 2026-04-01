package server

import (
	"log/slog"
	"net/http"
	"os"
	"runtime/debug"
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

// loggingResponseWriter captures the HTTP status code for logging.
type loggingResponseWriter struct {
	http.ResponseWriter
	status int
}

func (lw *loggingResponseWriter) WriteHeader(code int) {
	lw.status = code
	lw.ResponseWriter.WriteHeader(code)
}
