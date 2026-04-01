package provider

import "github.com/wall-ai/agent-engine/internal/engine"

// Provider is an alias for engine.ModelCaller — the interface all LLM backend
// adapters must satisfy. Keeping the alias here lets existing code in this
// package compile without changes while the authoritative definition lives in
// the engine package (breaking the engine ↔ provider import cycle).
type Provider = engine.ModelCaller

// CallParams is an alias for engine.CallParams.
type CallParams = engine.CallParams

// ToolDefinition is an alias for engine.ToolDefinition.
type ToolDefinition = engine.ToolDefinition
