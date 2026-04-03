package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"strconv"
	"syscall"

	"github.com/spf13/cobra"
	"github.com/wall-ai/agent-engine/internal/util"
)

// Version is set at build time via -ldflags.
var Version = "dev"

const banner = `
 ┌─────────────────────────────────┐
 │  Agent Engine  –  Wall AI       │
 │  Go rewrite of Claude Code core │
 └─────────────────────────────────┘
`

// cliOpts holds all parsed CLI flags.
type cliOpts struct {
	// Mode flags
	Print  string
	Serve  bool
	Resume string

	// Model / provider
	Model    string
	Provider string
	BaseURL  string

	// Behaviour
	PermissionMode    string
	OutputFormat      string
	SystemPrompt      string
	AppendSystemPrompt string
	AllowedTools      []string
	DisallowedTools   []string
	MCPConfig         []string
	MaxTurns          int
	MaxCostUSD        float64

	// Session
	Continue  bool
	SessionID string

	// Misc
	Verbose bool
	Debug   bool
	WorkDir string
}

var opts cliOpts

func newRootCmd() *cobra.Command {
	rootCmd := &cobra.Command{
		Use:   "agent-engine [prompt]",
		Short: "Agent Engine – AI coding assistant",
		Long:  "A Go implementation of an AI-powered coding assistant with interactive CLI.",
		Args:  cobra.ArbitraryArgs,
		RunE:  runRoot,
		SilenceUsage:  true,
		SilenceErrors: true,
	}

	// ── Mode flags ──────────────────────────────────────────────────────
	f := rootCmd.Flags()
	f.StringVarP(&opts.Print, "print", "p", "", "Non-interactive: run a single prompt and exit")
	f.BoolVar(&opts.Serve, "serve", false, "Start in HTTP server mode")

	// ── Model / provider ────────────────────────────────────────────────
	f.StringVar(&opts.Model, "model", "", "Override model name")
	f.StringVar(&opts.Provider, "provider", "", "Override provider (openai, anthropic, ...)")
	f.StringVar(&opts.BaseURL, "base-url", "", "Override API base URL")

	// ── Behaviour ───────────────────────────────────────────────────────
	f.StringVar(&opts.PermissionMode, "permission-mode", "", "Permission mode: default|auto|bypass|plan|acceptEdits|dontAsk")
	f.StringVar(&opts.OutputFormat, "output-format", "", "Output format: text|json|stream-json")
	f.StringVar(&opts.SystemPrompt, "system-prompt", "", "Custom system prompt (replaces default)")
	f.StringVar(&opts.AppendSystemPrompt, "append-system-prompt", "", "Append to default system prompt")
	f.StringSliceVar(&opts.AllowedTools, "allowed-tools", nil, "Tool allowlist (comma-separated)")
	f.StringSliceVar(&opts.DisallowedTools, "disallowed-tools", nil, "Tool denylist (comma-separated)")
	f.StringSliceVar(&opts.MCPConfig, "mcp-config", nil, "MCP config file paths")
	f.IntVar(&opts.MaxTurns, "max-turns", 0, "Maximum conversation turns (0 = unlimited)")
	f.Float64Var(&opts.MaxCostUSD, "max-cost", 0, "Maximum session cost in USD (0 = unlimited)")

	// ── Session ─────────────────────────────────────────────────────────
	f.BoolVarP(&opts.Continue, "continue", "c", false, "Continue most recent conversation")
	f.StringVarP(&opts.Resume, "resume", "r", "", "Resume conversation by session ID")

	// ── Misc ────────────────────────────────────────────────────────────
	f.BoolVarP(&opts.Verbose, "verbose", "v", false, "Enable verbose logging")
	f.BoolVarP(&opts.Debug, "debug", "d", false, "Enable debug mode")
	f.StringVarP(&opts.WorkDir, "work-dir", "C", "", "Working directory")

	// ── Subcommands ─────────────────────────────────────────────────────
	rootCmd.AddCommand(newVersionCmd())
	rootCmd.AddCommand(newServeCmd())
	rootCmd.AddCommand(newDoctorCmd())

	return rootCmd
}

// runRoot is the main entry point. It dispatches to interactive, print, or serve mode.
func runRoot(cmd *cobra.Command, args []string) error {
	// 1. Load configuration
	if err := util.InitConfig(); err != nil {
		return fmt.Errorf("init config: %w", err)
	}

	// 2. Initialise logger
	isVerbose := opts.Verbose || opts.Debug || util.GetBoolConfig("verbose")
	util.InitLogger(isVerbose)

	// 3. Resolve working directory
	wd := opts.WorkDir
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

	// 6. Dispatch to serve mode
	if opts.Serve {
		return runServeMode(ctx, wd)
	}

	// 7. Build AppConfig from flags + config files
	appCfg, err := buildAppConfig(wd)
	if err != nil {
		return err
	}

	// 8. Non-interactive print mode
	if opts.Print != "" {
		return runPrintMode(ctx, appCfg, wd, opts.Print)
	}

	// 9. Check for inline prompt: agent-engine "do something"
	if len(args) > 0 && opts.Print == "" {
		prompt := args[0]
		return runPrintMode(ctx, appCfg, wd, prompt)
	}

	// 10. Interactive REPL
	return runInteractiveMode(ctx, appCfg, wd)
}

// buildAppConfig creates an AppConfig from layered sources + CLI overrides.
func buildAppConfig(wd string) (*util.AppConfig, error) {
	appCfg, err := util.LoadAppConfig(wd)
	if err != nil {
		return nil, fmt.Errorf("load config: %w", err)
	}

	// CLI flag overrides (highest priority)
	if opts.Model != "" {
		appCfg.Model = opts.Model
	}
	if opts.Provider != "" {
		appCfg.Provider = opts.Provider
	}
	if opts.BaseURL != "" {
		appCfg.BaseURL = opts.BaseURL
	}
	if opts.PermissionMode != "" {
		appCfg.PermissionMode = opts.PermissionMode
	}
	if opts.OutputFormat != "" {
		appCfg.OutputFormat = opts.OutputFormat
	}
	if opts.MaxCostUSD > 0 {
		appCfg.MaxCostUSD = opts.MaxCostUSD
	}
	if opts.Verbose || opts.Debug {
		appCfg.VerboseMode = true
	}

	return appCfg, nil
}

// runServeMode starts the HTTP server.
func runServeMode(ctx context.Context, wd string) error {
	fmt.Print(banner)
	port := util.GetInt("http_port")
	if portEnv := os.Getenv("PORT"); portEnv != "" {
		if p, err := strconv.Atoi(portEnv); err == nil {
			port = p
		}
	}
	addr := fmt.Sprintf(":%d", port)
	slog.Info("starting agent engine (HTTP)", slog.String("addr", addr))

	// Import server inline to keep the root command light.
	// The server package is already available.
	srv := newHTTPServer(addr)
	return srv.Start(ctx)
}
