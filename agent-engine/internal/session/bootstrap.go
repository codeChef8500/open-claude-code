package session

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"github.com/wall-ai/agent-engine/internal/analytics"
	"github.com/wall-ai/agent-engine/internal/command"
	"github.com/wall-ai/agent-engine/internal/engine"
	"github.com/wall-ai/agent-engine/internal/memory"
	"github.com/wall-ai/agent-engine/internal/permission"
	"github.com/wall-ai/agent-engine/internal/prompt"
	"github.com/wall-ai/agent-engine/internal/provider"
	"github.com/wall-ai/agent-engine/internal/toolset"
	"github.com/wall-ai/agent-engine/internal/util"
)

// BootstrapConfig holds all the inputs needed to set up a new interactive session.
type BootstrapConfig struct {
	AppConfig *util.AppConfig
	WorkDir   string
	SessionID string
}

// BootstrapResult holds the fully initialised components ready for use.
type BootstrapResult struct {
	Engine         *engine.Engine
	Provider       provider.Provider
	Checker        *permission.Checker
	PermStore      *permission.PermissionStore
	CmdExecutor    *command.Executor
	MemoryStore    *memory.Store
	SessionMemory  *memory.SessionMemoryManager
	CostTracker    *provider.CostTracker
	SessionTracker *analytics.SessionTracker
	SystemPrompt   *prompt.BuiltSystemPrompt
}

// Bootstrap wires together all subsystems for an interactive session.
func Bootstrap(ctx context.Context, cfg BootstrapConfig) (*BootstrapResult, error) {
	appCfg := cfg.AppConfig
	if appCfg == nil {
		var err error
		appCfg, err = util.LoadAppConfig(cfg.WorkDir)
		if err != nil {
			return nil, fmt.Errorf("load config: %w", err)
		}
	}

	workDir := cfg.WorkDir
	if workDir == "" {
		workDir, _ = os.Getwd()
	}
	sessionID := cfg.SessionID
	if sessionID == "" {
		sessionID = generateSessionID()
	}

	result := &BootstrapResult{}

	// ── 1. Provider ────────────────────────────────────────────────────────
	prov, err := provider.New(provider.Config{
		Type:    appCfg.Provider,
		APIKey:  appCfg.APIKey,
		Model:   appCfg.Model,
		BaseURL: appCfg.BaseURL,
	})
	if err != nil {
		return nil, fmt.Errorf("create provider: %w", err)
	}
	result.Provider = prov

	// ── 2. Permission checker ──────────────────────────────────────────────
	permStorePath := permission.DefaultPermissionStorePath(workDir)
	permStore := permission.NewPermissionStore(permStorePath)
	_ = permStore.Load()
	allow, deny := permStore.ToRules()
	checker := permission.NewChecker(
		permission.Mode(appCfg.PermissionMode),
		allow, deny,
		appCfg.AllowedDirs,
		appCfg.DeniedCommands,
	)
	result.Checker = checker
	result.PermStore = permStore

	// ── 3. Memory ──────────────────────────────────────────────────────────
	home, _ := os.UserHomeDir()
	memDir := filepath.Join(home, ".claude", "memory", sessionID)
	memStore := memory.NewStore(memDir)
	_ = memStore.Load()
	result.MemoryStore = memStore

	smCfg := memory.DefaultSessionMemoryConfig()
	smCfg.Enabled = appCfg.SessionMemoryEnabled
	sessionMem := memory.NewSessionMemoryManager(smCfg, memStore, prov, sessionID)
	result.SessionMemory = sessionMem

	// ── 4. System prompt ───────────────────────────────────────────────────
	memInjection, _ := memory.ReadClaudeMd(workDir)
	memContent := ""
	if memInjection != nil && memInjection.HasContent() {
		memContent = memInjection.MergedContent()
	}

	sysPrompt := prompt.BuildEffectiveSystemPrompt(prompt.BuildOptions{
		WorkDir:       workDir,
		MemoryContent: memContent,
	})
	result.SystemPrompt = sysPrompt

	// ── 5. Engine ──────────────────────────────────────────────────────────
	eng, err := engine.New(engine.EngineConfig{
		Provider:           appCfg.Provider,
		APIKey:             appCfg.APIKey,
		BaseURL:            appCfg.BaseURL,
		Model:              appCfg.Model,
		MaxTokens:          appCfg.MaxTokens,
		ThinkingBudget:     appCfg.ThinkingBudget,
		WorkDir:            workDir,
		SessionID:          sessionID,
		AutoMode:           appCfg.AutoMode,
		MaxCostUSD:         appCfg.MaxCostUSD,
		PermissionMode:     appCfg.PermissionMode,
		CustomSystemPrompt: sysPrompt.Text,
		Verbose:            appCfg.VerboseMode,
	}, prov, toolset.DefaultTools(nil))
	if err != nil {
		return nil, fmt.Errorf("create engine: %w", err)
	}
	result.Engine = eng

	// Wire optional integrations into the engine (same as SDK path).
	eng.SetMemoryLoader(memory.NewAdapter())
	eng.SetSessionWriter(NewAdapter())
	eng.SetPromptBuilder(prompt.NewAdapter())
	eng.SetPermissionChecker(permission.NewAdapter())

	// ── 6. Command executor ────────────────────────────────────────────────
	cmdExec := command.NewExecutor(command.Default())
	result.CmdExecutor = cmdExec

	// ── 7. Cost + session tracking ─────────────────────────────────────────
	costTracker := provider.NewCostTracker()
	result.CostTracker = costTracker

	sessionTracker := analytics.NewSessionTracker(sessionID, appCfg.Model, workDir)
	result.SessionTracker = sessionTracker

	// ── 8. Analytics ───────────────────────────────────────────────────────
	analytics.SetSessionID(sessionID)
	analyticsPath := analytics.DefaultAnalyticsPath()
	if analyticsPath != "" {
		sink, err := analytics.NewFileSink(analyticsPath)
		if err != nil {
			slog.Warn("analytics sink disabled", slog.String("error", err.Error()))
		} else {
			analytics.AttachSink(sink)
		}
	}

	analytics.LogEvent("session_start", analytics.EventMetadata{
		"model":      appCfg.Model,
		"provider":   appCfg.Provider,
		"work_dir":   workDir,
		"session_id": sessionID,
	})

	slog.Info("session bootstrapped",
		slog.String("session_id", sessionID),
		slog.String("model", appCfg.Model),
		slog.String("work_dir", workDir),
	)

	return result, nil
}

// Shutdown performs graceful cleanup for a session.
func Shutdown(result *BootstrapResult) {
	if result == nil {
		return
	}
	if result.SessionTracker != nil {
		result.SessionTracker.EmitSessionEnd()
	}
	_ = analytics.Flush()
	_ = analytics.Close()
}

func generateSessionID() string {
	return fmt.Sprintf("sess_%d_%d", os.Getpid(), time.Now().UnixMilli())
}
