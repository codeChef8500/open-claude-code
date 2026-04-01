package test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/wall-ai/agent-engine/internal/command"
)

func TestCommandHelp(t *testing.T) {
	ectx := &command.ExecContext{SessionID: "sess-1"}
	result, err := command.Execute(context.Background(), "help", nil, ectx)
	require.NoError(t, err)
	assert.Contains(t, result, "Available commands")
}

func TestCommandClear(t *testing.T) {
	ectx := &command.ExecContext{SessionID: "sess-1"}
	result, err := command.Execute(context.Background(), "clear", nil, ectx)
	require.NoError(t, err)
	assert.Equal(t, "__clear_history__", result)
}

func TestCommandCompact(t *testing.T) {
	ectx := &command.ExecContext{SessionID: "sess-1"}
	result, err := command.Execute(context.Background(), "compact", nil, ectx)
	require.NoError(t, err)
	assert.Equal(t, "__compact__", result)
}

func TestCommandUnknown(t *testing.T) {
	ectx := &command.ExecContext{SessionID: "sess-1"}
	_, err := command.Execute(context.Background(), "nonexistent_command_xyz", nil, ectx)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unknown command")
}

func TestCommandModel(t *testing.T) {
	ectx := &command.ExecContext{SessionID: "sess-1"}
	result, err := command.Execute(context.Background(), "model", []string{"claude-opus-4-5"}, ectx)
	require.NoError(t, err)
	assert.Contains(t, result, "claude-opus-4-5")
}
