package worktree

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"

	"github.com/wall-ai/agent-engine/internal/engine"
	"github.com/wall-ai/agent-engine/internal/tool"
)

// EnterInput is the input schema for EnterWorktreeTool.
type EnterInput struct {
	Branch string `json:"branch"`
	Path   string `json:"path,omitempty"`
	Create bool   `json:"create,omitempty"`
}

// ExitInput is the input schema for ExitWorktreeTool.
type ExitInput struct {
	Path   string `json:"path"`
	Remove bool   `json:"remove,omitempty"`
}

// ── EnterWorktreeTool ─────────────────────────────────────────────────────────

// EnterWorktreeTool creates or switches to a git worktree.
type EnterWorktreeTool struct{ tool.BaseTool }

func NewEnter() *EnterWorktreeTool { return &EnterWorktreeTool{} }

func (t *EnterWorktreeTool) Name() string           { return "EnterWorktree" }
func (t *EnterWorktreeTool) UserFacingName() string { return "enter_worktree" }
func (t *EnterWorktreeTool) Description() string {
	return "Create or switch to a git worktree for isolated branch work."
}
func (t *EnterWorktreeTool) IsReadOnly() bool        { return false }
func (t *EnterWorktreeTool) IsConcurrencySafe() bool { return false }
func (t *EnterWorktreeTool) MaxResultSizeChars() int { return 4096 }
func (t *EnterWorktreeTool) IsEnabled(_ *tool.UseContext) bool { return true }
func (t *EnterWorktreeTool) IsDestructive() bool { return false }

func (t *EnterWorktreeTool) InputSchema() json.RawMessage {
	return json.RawMessage(`{
		"type":"object",
		"properties":{
			"branch":{"type":"string","description":"Branch name for the worktree."},
			"path":{"type":"string","description":"Directory path for the worktree. Auto-generated if omitted."},
			"create":{"type":"boolean","description":"Create the branch if it does not exist (default false)."}
		},
		"required":["branch"]
	}`)
}

func (t *EnterWorktreeTool) Prompt(_ *tool.UseContext) string { return "" }

func (t *EnterWorktreeTool) CheckPermissions(_ context.Context, input json.RawMessage, _ *tool.UseContext) error {
	var in EnterInput
	if err := json.Unmarshal(input, &in); err != nil {
		return fmt.Errorf("invalid input: %w", err)
	}
	if in.Branch == "" {
		return fmt.Errorf("branch must not be empty")
	}
	return nil
}

func (t *EnterWorktreeTool) Call(ctx context.Context, input json.RawMessage, uctx *tool.UseContext) (<-chan *engine.ContentBlock, error) {
	var in EnterInput
	if err := json.Unmarshal(input, &in); err != nil {
		return nil, fmt.Errorf("invalid input: %w", err)
	}

	ch := make(chan *engine.ContentBlock, 2)
	go func() {
		defer close(ch)
		worktreePath := in.Path
		if worktreePath == "" {
			worktreePath = ".worktrees/" + sanitizeName(in.Branch)
		}

		var args []string
		if in.Create {
			args = []string{"worktree", "add", "-b", in.Branch, worktreePath}
		} else {
			args = []string{"worktree", "add", worktreePath, in.Branch}
		}

		cmd := exec.CommandContext(ctx, "git", args...)
		if uctx != nil && uctx.WorkDir != "" {
			cmd.Dir = uctx.WorkDir
		}
		out, err := cmd.CombinedOutput()
		if err != nil {
			ch <- &engine.ContentBlock{
				Type:    engine.ContentTypeText,
				Text:    fmt.Sprintf("git worktree add failed: %s\n%s", err, string(out)),
				IsError: true,
			}
			return
		}
		ch <- &engine.ContentBlock{
			Type: engine.ContentTypeText,
			Text: fmt.Sprintf("Worktree created at %s on branch %s.\n%s", worktreePath, in.Branch, string(out)),
		}
	}()
	return ch, nil
}

// ── ExitWorktreeTool ──────────────────────────────────────────────────────────

// ExitWorktreeTool removes a git worktree.
type ExitWorktreeTool struct{ tool.BaseTool }

func NewExit() *ExitWorktreeTool { return &ExitWorktreeTool{} }

func (t *ExitWorktreeTool) Name() string           { return "ExitWorktree" }
func (t *ExitWorktreeTool) UserFacingName() string { return "exit_worktree" }
func (t *ExitWorktreeTool) Description() string {
	return "Remove a git worktree when done with isolated branch work."
}
func (t *ExitWorktreeTool) IsReadOnly() bool        { return false }
func (t *ExitWorktreeTool) IsConcurrencySafe() bool { return false }
func (t *ExitWorktreeTool) MaxResultSizeChars() int { return 4096 }
func (t *ExitWorktreeTool) IsEnabled(_ *tool.UseContext) bool { return true }
func (t *ExitWorktreeTool) IsDestructive() bool { return true }

func (t *ExitWorktreeTool) InputSchema() json.RawMessage {
	return json.RawMessage(`{
		"type":"object",
		"properties":{
			"path":{"type":"string","description":"Path of the worktree to remove."},
			"remove":{"type":"boolean","description":"Also delete the worktree directory (default false)."}
		},
		"required":["path"]
	}`)
}

func (t *ExitWorktreeTool) Prompt(_ *tool.UseContext) string { return "" }

func (t *ExitWorktreeTool) CheckPermissions(_ context.Context, input json.RawMessage, _ *tool.UseContext) error {
	var in ExitInput
	if err := json.Unmarshal(input, &in); err != nil {
		return fmt.Errorf("invalid input: %w", err)
	}
	if in.Path == "" {
		return fmt.Errorf("path must not be empty")
	}
	return nil
}

func (t *ExitWorktreeTool) Call(ctx context.Context, input json.RawMessage, uctx *tool.UseContext) (<-chan *engine.ContentBlock, error) {
	var in ExitInput
	if err := json.Unmarshal(input, &in); err != nil {
		return nil, fmt.Errorf("invalid input: %w", err)
	}

	ch := make(chan *engine.ContentBlock, 2)
	go func() {
		defer close(ch)
		args := []string{"worktree", "remove"}
		if in.Remove {
			args = append(args, "--force")
		}
		args = append(args, in.Path)

		cmd := exec.CommandContext(ctx, "git", args...)
		if uctx != nil && uctx.WorkDir != "" {
			cmd.Dir = uctx.WorkDir
		}
		out, err := cmd.CombinedOutput()
		if err != nil {
			ch <- &engine.ContentBlock{
				Type:    engine.ContentTypeText,
				Text:    fmt.Sprintf("git worktree remove failed: %s\n%s", err, string(out)),
				IsError: true,
			}
			return
		}
		ch <- &engine.ContentBlock{
			Type: engine.ContentTypeText,
			Text: fmt.Sprintf("Worktree at %s removed.\n%s", in.Path, string(out)),
		}
	}()
	return ch, nil
}

func sanitizeName(s string) string {
	return strings.NewReplacer("/", "-", " ", "-", ".", "-").Replace(s)
}
