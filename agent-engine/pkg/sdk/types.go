// Package sdk exports the public types that consumers of the Agent Engine SDK need.
// All core types are aliases of the internal engine types to preserve type identity.
package sdk

import "github.com/wall-ai/agent-engine/internal/engine"

// Re-export StreamEvent and StreamEventType so callers need only import this package.
type StreamEvent = engine.StreamEvent
type StreamEventType = engine.StreamEventType

// Re-export event type constants.
const (
	EventTextDelta     = engine.EventTextDelta
	EventTextComplete  = engine.EventTextComplete
	EventToolUse       = engine.EventToolUse
	EventToolResult    = engine.EventToolResult
	EventThinking      = engine.EventThinking
	EventUsage         = engine.EventUsage
	EventError         = engine.EventError
	EventDone          = engine.EventDone
	EventSystemMessage = engine.EventSystemMessage
)

// Re-export message and content types.
type Message = engine.Message
type MessageRole = engine.MessageRole
type ContentBlock = engine.ContentBlock
type ContentType = engine.ContentType
type UsageStats = engine.UsageStats

// Re-export config types.
type EngineConfig = engine.EngineConfig
type QueryParams = engine.QueryParams
