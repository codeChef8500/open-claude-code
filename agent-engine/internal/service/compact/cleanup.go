package compact

import (
	"context"
	"log/slog"

	"github.com/wall-ai/agent-engine/internal/hooks"
)

// PostCompactCleanupConfig holds options for post-compact cleanup.
type PostCompactCleanupConfig struct {
	// HookExecutor fires PostCompact hooks if configured.
	HookExecutor *hooks.Executor
	// QuerySource identifies the compaction trigger (e.g. "auto", "manual", "session_memory").
	QuerySource string
	// ResetFileCache, if non-nil, is called to clear the file state cache
	// so that post-compact file re-reads fetch fresh content.
	ResetFileCache func()
	// ResetNestedMemoryPaths, if non-nil, clears loaded nested memory paths
	// so they are re-injected on the next turn.
	ResetNestedMemoryPaths func()
}

// RunPostCompactCleanup performs cleanup tasks after a successful compaction:
//   - Fires PostCompact hooks
//   - Resets file state cache
//   - Resets nested memory paths
func RunPostCompactCleanup(ctx context.Context, cfg PostCompactCleanupConfig) {
	// Fire PostCompact hook.
	if cfg.HookExecutor != nil && cfg.HookExecutor.HasHooksFor(hooks.EventPostCompact) {
		resp := cfg.HookExecutor.RunSync(ctx, hooks.EventPostCompact, &hooks.HookInput{})
		if resp.FailureReason != "" {
			slog.Warn("compact: PostCompact hook reported failure",
				slog.String("reason", resp.FailureReason))
		}
	}

	// Reset file state cache.
	if cfg.ResetFileCache != nil {
		cfg.ResetFileCache()
	}

	// Reset nested memory paths.
	if cfg.ResetNestedMemoryPaths != nil {
		cfg.ResetNestedMemoryPaths()
	}

	slog.Debug("compact: post-compact cleanup complete",
		slog.String("source", cfg.QuerySource))
}
