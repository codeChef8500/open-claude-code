package util

import (
	"log/slog"
	"os"
	"os/signal"
	"sync"
	"syscall"
)

var (
	cleanupMu  sync.Mutex
	cleanupFns []func()
)

// RegisterCleanup registers a function to be called during graceful shutdown.
func RegisterCleanup(fn func()) {
	cleanupMu.Lock()
	defer cleanupMu.Unlock()
	cleanupFns = append(cleanupFns, fn)
}

// RunCleanup executes all registered cleanup functions in LIFO order.
func RunCleanup() {
	cleanupMu.Lock()
	fns := make([]func(), len(cleanupFns))
	copy(fns, cleanupFns)
	cleanupMu.Unlock()

	for i := len(fns) - 1; i >= 0; i-- {
		fns[i]()
	}
}

// WaitForShutdown blocks until SIGINT or SIGTERM is received, then runs all
// registered cleanup functions and returns.
func WaitForShutdown() {
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, syscall.SIGINT, syscall.SIGTERM)
	sig := <-ch
	slog.Info("received signal, shutting down", slog.String("signal", sig.String()))
	RunCleanup()
}
