package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"

	"github.com/wall-ai/agent-engine/internal/server"
	"github.com/wall-ai/agent-engine/internal/session"
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
	// ── CLI flags ──────────────────────────────────────────────────────────
	serve := flag.Bool("serve", false, "Start in HTTP server mode")
	prompt := flag.String("p", "", "Non-interactive: run a single prompt and exit")
	model := flag.String("model", "", "Override model name")
	verbose := flag.Bool("verbose", false, "Enable verbose logging")
	workDir := flag.String("C", "", "Working directory")
	flag.Parse()

	// 1. Load configuration (Viper: env > config file > defaults)
	if err := util.InitConfig(); err != nil {
		return fmt.Errorf("init config: %w", err)
	}

	// 2. Initialise logger
	isVerbose := *verbose || util.GetBoolConfig("verbose")
	util.InitLogger(isVerbose)

	// 3. Resolve working directory
	wd := *workDir
	if wd == "" {
		wd, _ = os.Getwd()
	}

	// 4. Context with graceful-shutdown support
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	// 5. Register cleanup handlers
	util.RegisterCleanup(func() {
		slog.Info("cleanup complete")
	})

	// ── HTTP server mode ───────────────────────────────────────────────────
	if *serve {
		fmt.Print(banner)
		port := util.GetInt("http_port")
		if portEnv := os.Getenv("PORT"); portEnv != "" {
			if p, err := strconv.Atoi(portEnv); err == nil {
				port = p
			}
		}
		addr := fmt.Sprintf(":%d", port)
		slog.Info("starting agent engine (HTTP)", slog.String("addr", addr))
		srv := server.New(addr)
		return srv.Start(ctx)
	}

	// ── Interactive / one-shot mode ────────────────────────────────────────
	appCfg, err := util.LoadAppConfig(wd)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}
	if *model != "" {
		appCfg.Model = *model
	}
	appCfg.VerboseMode = isVerbose

	result, err := session.Bootstrap(ctx, session.BootstrapConfig{
		AppConfig: appCfg,
		WorkDir:   wd,
	})
	if err != nil {
		return fmt.Errorf("bootstrap: %w", err)
	}
	defer session.Shutdown(result)

	runner := session.NewRunner(result)

	// Wire output callbacks.
	runner.OnTextDelta = func(text string) {
		fmt.Print(text)
	}
	runner.OnToolStart = func(id, name, input string) {
		fmt.Fprintf(os.Stderr, "\n⚙ %s %s\n", name, input)
	}
	runner.OnToolDone = func(id, output string, isError bool) {
		if isError {
			fmt.Fprintf(os.Stderr, "✗ tool error: %s\n", output)
		}
	}
	runner.OnDone = func() {
		fmt.Println()
	}
	runner.OnError = func(err error) {
		fmt.Fprintf(os.Stderr, "⚠ %v\n", err)
	}
	runner.OnSystem = func(text string) {
		fmt.Fprintf(os.Stderr, "▶ %s\n", text)
	}

	// One-shot mode: -p "prompt"
	if *prompt != "" {
		runner.HandleInput(ctx, *prompt)
		return nil
	}

	// Interactive REPL.
	fmt.Print(banner)
	fmt.Fprintf(os.Stderr, "Model: %s | Work dir: %s\n", appCfg.Model, wd)
	fmt.Fprintf(os.Stderr, "Type /help for commands, /quit to exit.\n\n")

	scanner := bufio.NewScanner(os.Stdin)
	for {
		fmt.Fprint(os.Stderr, "> ")
		if !scanner.Scan() {
			break
		}
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		if !runner.HandleInput(ctx, line) {
			break
		}

		// Check context cancellation.
		if ctx.Err() != nil {
			break
		}
	}

	return nil
}
