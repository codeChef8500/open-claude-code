package engine

import (
	"context"
	"encoding/json"
)

// Tool is the contract every tool implementation must satisfy.
// Defining it here (rather than in the tool package) breaks the
// engine ↔ tool import cycle.
type Tool interface {
	Name() string
	UserFacingName() string
	Description() string
	InputSchema() json.RawMessage
	Call(ctx context.Context, input json.RawMessage, uctx *UseContext) (<-chan *ContentBlock, error)
	CheckPermissions(ctx context.Context, input json.RawMessage, uctx *UseContext) error
	Prompt(uctx *UseContext) string
	IsEnabled(uctx *UseContext) bool
	IsReadOnly() bool
	IsConcurrencySafe() bool
	MaxResultSizeChars() int
}

// UseContext carries per-request context that tools may need.
type UseContext struct {
	WorkDir          string
	SessionID        string
	AutoMode         bool
	AgentID          string
	PermittedDirs    []string
	DeniedCommands   []string
	AskPermission    func(ctx context.Context, tool, desc string) (bool, error)
	SendNotification func(msg string)
}
