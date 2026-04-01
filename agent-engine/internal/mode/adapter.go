package mode

import (
	"context"

	"github.com/wall-ai/agent-engine/internal/engine"
	"github.com/wall-ai/agent-engine/internal/provider"
)

// ClassifierAdapter wraps RunYoloClassifier to implement engine.AutoModeClassifier.
// Wired at SDK construction time to avoid circular imports.
type ClassifierAdapter struct {
	prov      provider.Provider
	userRules []AutoModeRule
}

// NewClassifierAdapter creates an AutoModeClassifier adapter.
// prov must be the same provider used by the engine (typically Anthropic).
func NewClassifierAdapter(prov provider.Provider, extraRules []AutoModeRule) *ClassifierAdapter {
	return &ClassifierAdapter{
		prov:      prov,
		userRules: extraRules,
	}
}

// Classify satisfies engine.AutoModeClassifier.
func (a *ClassifierAdapter) Classify(
	ctx interface{ Done() <-chan struct{} },
	toolName string,
	toolInput interface{},
) (engine.PermissionVerdict, string, error) {
	goCtx, ok := ctx.(context.Context)
	if !ok {
		goCtx = context.Background()
	}

	verdict, reason, err := RunYoloClassifier(goCtx, a.prov, toolName, toolInput, a.userRules)
	if err != nil {
		return engine.PermissionAllow, "", err
	}

	switch verdict {
	case VerdictDeny:
		return engine.PermissionDeny, reason, nil
	case VerdictSoftDeny:
		return engine.PermissionSoftDeny, reason, nil
	default:
		return engine.PermissionAllow, reason, nil
	}
}
