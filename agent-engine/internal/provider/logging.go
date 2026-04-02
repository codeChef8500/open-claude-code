package provider

import (
	"context"
	"log/slog"
	"time"

	"github.com/wall-ai/agent-engine/internal/engine"
)

// LoggingProvider wraps any ModelCaller and logs each API call's metadata.
type LoggingProvider struct {
	inner  engine.ModelCaller
	logger *slog.Logger
}

// NewLoggingProvider creates a provider that logs every model call.
// If logger is nil, slog.Default() is used.
func NewLoggingProvider(inner engine.ModelCaller, logger *slog.Logger) *LoggingProvider {
	if logger == nil {
		logger = slog.Default()
	}
	return &LoggingProvider{inner: inner, logger: logger}
}

// CallModel forwards the call to the inner provider, logging start/end and
// aggregated usage stats.
func (lp *LoggingProvider) CallModel(ctx context.Context, params engine.CallParams) (<-chan *engine.StreamEvent, error) {
	start := time.Now()
	lp.logger.Debug("provider: calling model",
		slog.String("model", params.Model),
		slog.Int("messages", len(params.Messages)),
		slog.Int("max_tokens", params.MaxTokens))

	ch, err := lp.inner.CallModel(ctx, params)
	if err != nil {
		lp.logger.Warn("provider: call failed",
			slog.String("model", params.Model),
			slog.Any("err", err),
			slog.Duration("elapsed", time.Since(start)))
		return nil, err
	}

	// Wrap the channel to intercept events for logging.
	out := make(chan *engine.StreamEvent, cap(ch))
	go func() {
		defer close(out)
		var (
			inputTokens  int
			outputTokens int
			gotError     bool
		)
		for ev := range ch {
			out <- ev
			if ev == nil {
				continue
			}
			switch ev.Type {
			case engine.EventUsage:
				if ev.Usage != nil {
					inputTokens = ev.Usage.InputTokens
					outputTokens = ev.Usage.OutputTokens
				}
			case engine.EventError:
				gotError = true
				lp.logger.Warn("provider: stream error",
					slog.String("model", params.Model),
					slog.String("error", ev.Error))
			}
		}
		level := slog.LevelDebug
		if gotError {
			level = slog.LevelWarn
		}
		lp.logger.Log(ctx, level, "provider: call complete",
			slog.String("model", params.Model),
			slog.Int("input_tokens", inputTokens),
			slog.Int("output_tokens", outputTokens),
			slog.Duration("elapsed", time.Since(start)))
	}()
	return out, nil
}
