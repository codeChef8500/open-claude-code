package permission

import (
	"context"

	"github.com/wall-ai/agent-engine/internal/engine"
)

// Adapter wraps Checker to implement engine.GlobalPermissionChecker.
// Wired at SDK construction time to avoid circular imports.
type Adapter struct {
	checker *Checker
}

// NewAdapter creates a permission Adapter backed by a default Checker.
func NewAdapter() *Adapter {
	return &Adapter{
		checker: NewChecker(ModeDefault, nil, nil, nil, nil),
	}
}

// NewAdapterWithChecker creates a permission Adapter backed by a custom Checker.
func NewAdapterWithChecker(c *Checker) *Adapter {
	return &Adapter{checker: c}
}

// CheckTool satisfies engine.GlobalPermissionChecker.
func (a *Adapter) CheckTool(ctx interface{ Done() <-chan struct{} }, toolName string, toolInput interface{}, workDir string) (engine.PermissionVerdict, string) {
	goCtx, ok := ctx.(context.Context)
	if !ok {
		goCtx = context.Background()
	}

	req := CheckRequest{
		ToolName:  toolName,
		ToolInput: toolInput,
		WorkDir:   workDir,
		Mode:      a.checker.mode,
	}

	if err := a.checker.Check(goCtx, req); err != nil {
		return engine.PermissionDeny, err.Error()
	}
	return engine.PermissionAllow, ""
}
