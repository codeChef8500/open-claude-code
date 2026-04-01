package server

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

// Server wraps the HTTP router and lifecycle management.
type Server struct {
	addr   string
	router *chi.Mux
	http   *http.Server
}

// New creates a new HTTP server bound to addr (e.g. ":8080").
func New(addr string) *Server {
	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Timeout(120 * time.Second))

	s := &Server{
		addr:   addr,
		router: r,
		http: &http.Server{
			Addr:         addr,
			Handler:      r,
			ReadTimeout:  30 * time.Second,
			WriteTimeout: 300 * time.Second, // long for SSE streams
			IdleTimeout:  120 * time.Second,
		},
	}

	registerRoutes(r)
	return s
}

// Start begins accepting connections. Blocks until ctx is cancelled.
func (s *Server) Start(ctx context.Context) error {
	errCh := make(chan error, 1)
	go func() {
		if err := s.http.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errCh <- err
		}
	}()

	select {
	case err := <-errCh:
		return fmt.Errorf("server: %w", err)
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		return s.http.Shutdown(shutdownCtx)
	}
}
