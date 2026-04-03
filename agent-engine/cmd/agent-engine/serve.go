package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"strconv"

	"github.com/spf13/cobra"
	"github.com/wall-ai/agent-engine/internal/server"
	"github.com/wall-ai/agent-engine/internal/util"
)

func newServeCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "serve",
		Short: "Start the HTTP API server",
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := util.InitConfig(); err != nil {
				return fmt.Errorf("init config: %w", err)
			}
			util.InitLogger(opts.Verbose)

			ctx, cancel := context.WithCancel(cmd.Context())
			defer cancel()

			wd, _ := os.Getwd()
			return runServeMode(ctx, wd)
		},
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	return cmd
}

// newHTTPServer creates the HTTP server instance.
func newHTTPServer(addr string) *server.Server {
	return server.New(addr)
}

// runServeModeStandalone is for the serve subcommand pathway.
func runServeModeFromCmd(ctx context.Context) error {
	port := util.GetInt("http_port")
	if portEnv := os.Getenv("PORT"); portEnv != "" {
		if p, err := strconv.Atoi(portEnv); err == nil {
			port = p
		}
	}
	addr := fmt.Sprintf(":%d", port)
	slog.Info("starting agent engine (HTTP)", slog.String("addr", addr))
	srv := newHTTPServer(addr)
	return srv.Start(ctx)
}
