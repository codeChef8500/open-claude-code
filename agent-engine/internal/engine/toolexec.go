package engine

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"time"
)

// ToolExecError classifies tool execution failures by category.
type ToolExecError struct {
	ToolName string
	Category ToolExecErrorCategory
	Cause    error
}

func (e *ToolExecError) Error() string {
	return fmt.Sprintf("tool %s (%s): %v", e.ToolName, e.Category, e.Cause)
}

func (e *ToolExecError) Unwrap() error { return e.Cause }

// ToolExecErrorCategory classifies why a tool call failed.
type ToolExecErrorCategory string

const (
	ToolExecErrValidation  ToolExecErrorCategory = "validation"
	ToolExecErrPermission  ToolExecErrorCategory = "permission"
	ToolExecErrExecution   ToolExecErrorCategory = "execution"
	ToolExecErrSizeExceeded ToolExecErrorCategory = "size_exceeded"
	ToolExecErrTimeout     ToolExecErrorCategory = "timeout"
)

// ToolExecRequest is the input to ExecuteTool.
type ToolExecRequest struct {
	ToolUseID string
	Tool      Tool
	Input     json.RawMessage
	UseCtx    *UseContext
}

// ToolExecResult is the output of ExecuteTool.
type ToolExecResult struct {
	ToolUseID string
	ToolName  string
	Blocks    []*ContentBlock
	IsError   bool
	Duration  time.Duration
	// ErrorCategory is set when IsError=true.
	ErrorCategory ToolExecErrorCategory
}

// ToolExecHooks are optional callbacks fired at each lifecycle stage.
// Any nil hook is silently skipped.
type ToolExecHooks struct {
	// PreValidate fires before input validation.
	PreValidate func(req *ToolExecRequest)
	// PrePermission fires after validation, before permission checks.
	PrePermission func(req *ToolExecRequest)
	// PreExecute fires after permissions are granted.
	PreExecute func(req *ToolExecRequest)
	// PostExecute fires after the tool returns its result.
	PostExecute func(req *ToolExecRequest, result *ToolExecResult)
	// OnError fires when any lifecycle stage returns an error.
	OnError func(req *ToolExecRequest, category ToolExecErrorCategory, err error)
}

// ToolExecutor runs a single tool through the full lifecycle:
// ValidateInput → CheckPermissions → [preHooks] → Call → [postHooks] → cap size.
type ToolExecutor struct {
	hooks *ToolExecHooks
}

// NewToolExecutor creates a ToolExecutor.  Pass nil hooks for no-op callbacks.
func NewToolExecutor(hooks *ToolExecHooks) *ToolExecutor {
	if hooks == nil {
		hooks = &ToolExecHooks{}
	}
	return &ToolExecutor{hooks: hooks}
}

// Execute runs the full tool lifecycle and returns a ToolExecResult.
// It never returns an error itself; failures are encoded in result.IsError.
func (te *ToolExecutor) Execute(ctx context.Context, req *ToolExecRequest) *ToolExecResult {
	start := time.Now()
	result := &ToolExecResult{
		ToolUseID: req.ToolUseID,
		ToolName:  req.Tool.Name(),
	}

	fail := func(cat ToolExecErrorCategory, err error) *ToolExecResult {
		result.IsError = true
		result.ErrorCategory = cat
		result.Duration = time.Since(start)
		result.Blocks = []*ContentBlock{{
			Type:    ContentTypeText,
			Text:    fmt.Sprintf("Tool %s failed (%s): %v", req.Tool.Name(), cat, err),
			IsError: true,
		}}
		if te.hooks.OnError != nil {
			te.hooks.OnError(req, cat, err)
		}
		slog.Debug("tool exec error",
			slog.String("tool", req.Tool.Name()),
			slog.String("category", string(cat)),
			slog.Any("err", err))
		return result
	}

	// ── Stage 1: ValidateInput ────────────────────────────────────────────
	if te.hooks.PreValidate != nil {
		te.hooks.PreValidate(req)
	}
	if err := req.Tool.ValidateInput(ctx, req.Input); err != nil {
		return fail(ToolExecErrValidation, err)
	}

	// ── Stage 2: CheckPermissions ─────────────────────────────────────────
	if te.hooks.PrePermission != nil {
		te.hooks.PrePermission(req)
	}
	if err := req.Tool.CheckPermissions(ctx, req.Input, req.UseCtx); err != nil {
		return fail(ToolExecErrPermission, err)
	}

	// ── Stage 3: Execute ──────────────────────────────────────────────────
	if te.hooks.PreExecute != nil {
		te.hooks.PreExecute(req)
	}
	blockCh, err := req.Tool.Call(ctx, req.Input, req.UseCtx)
	if err != nil {
		return fail(ToolExecErrExecution, err)
	}

	// Drain the result channel.
	var blocks []*ContentBlock
	for b := range blockCh {
		if b != nil {
			blocks = append(blocks, b)
		}
	}
	if len(blocks) == 0 {
		blocks = []*ContentBlock{{Type: ContentTypeText, Text: "(no output)"}}
	}

	// ── Stage 4: Cap output size ──────────────────────────────────────────
	maxChars := req.Tool.MaxResultSizeChars()
	if req.UseCtx != nil && req.UseCtx.MaxResultChars > 0 {
		maxChars = req.UseCtx.MaxResultChars
	}
	if maxChars > 0 {
		blocks = capBlockSize(blocks, maxChars)
	}

	result.Blocks = blocks
	result.Duration = time.Since(start)
	for _, b := range blocks {
		if b.IsError {
			result.IsError = true
			break
		}
	}

	if te.hooks.PostExecute != nil {
		te.hooks.PostExecute(req, result)
	}

	slog.Debug("tool exec complete",
		slog.String("tool", req.Tool.Name()),
		slog.Duration("duration", result.Duration),
		slog.Bool("error", result.IsError))

	return result
}

// capBlockSize truncates text blocks so total character count ≤ maxChars.
func capBlockSize(blocks []*ContentBlock, maxChars int) []*ContentBlock {
	total := 0
	out := make([]*ContentBlock, 0, len(blocks))
	for _, b := range blocks {
		if b.Type != ContentTypeText {
			out = append(out, b)
			continue
		}
		remaining := maxChars - total
		if remaining <= 0 {
			out = append(out, &ContentBlock{
				Type:    ContentTypeText,
				Text:    "[... output truncated — size limit reached ...]",
				IsError: false,
			})
			break
		}
		if len(b.Text) > remaining {
			out = append(out, &ContentBlock{
				Type:    b.Type,
				Text:    b.Text[:remaining] + "\n[... truncated ...]",
				IsError: b.IsError,
			})
			total = maxChars
		} else {
			out = append(out, b)
			total += len(b.Text)
		}
	}
	return out
}

// ClassifyToolError maps a raw error to a ToolExecErrorCategory.
func ClassifyToolError(err error) ToolExecErrorCategory {
	if err == nil {
		return ""
	}
	msg := strings.ToLower(err.Error())
	switch {
	case strings.Contains(msg, "permission") || strings.Contains(msg, "denied") || strings.Contains(msg, "not permitted"):
		return ToolExecErrPermission
	case strings.Contains(msg, "timeout") || strings.Contains(msg, "deadline"):
		return ToolExecErrTimeout
	case strings.Contains(msg, "invalid") || strings.Contains(msg, "validation"):
		return ToolExecErrValidation
	default:
		return ToolExecErrExecution
	}
}
