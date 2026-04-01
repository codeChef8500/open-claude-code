package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"strconv"
	"syscall"

	"github.com/wall-ai/agent-engine/internal/server"
	"github.com/wall-ai/agent-engine/internal/util"
)

const banner = `
 ┌─────────────────────────────────┐
 │  Agent Engine  –  Wall AI       │
 │  Go rewrite of Claude Code core │
 └─────────────────────────────────┘
`

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "fatal: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	// 1. Load configuration (Viper: env > config file > defaults)
	if err := util.InitConfig(); err != nil {
		return fmt.Errorf("init config: %w", err)
	}

	// 2. Initialise logger
	verbose := util.GetBoolConfig("verbose")
	util.InitLogger(verbose)

	fmt.Print(banner)

	// 3. Determine HTTP listen address
	port := util.GetInt("http_port")
	if portEnv := os.Getenv("PORT"); portEnv != "" {
		if p, err := strconv.Atoi(portEnv); err == nil {
			port = p
		}
	}
	addr := fmt.Sprintf(":%d", port)

	// 4. Context with graceful-shutdown support
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	// 5. Register cleanup handlers
	util.RegisterCleanup(func() {
		slog.Info("cleanup complete")
	})

	// 6. Start HTTP server
	slog.Info("starting agent engine", slog.String("addr", addr))
	srv := server.New(addr)
	return srv.Start(ctx)
}
