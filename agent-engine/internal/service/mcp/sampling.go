package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
)

// ── Sampling types (MCP spec: sampling/createMessage) ───────────────────────

// SamplingMessage is a single message in a sampling request/result.
type SamplingMessage struct {
	Role    string      `json:"role"` // "user" or "assistant"
	Content ContentItem `json:"content"`
}

// ModelPreferences expresses the client's model selection preferences.
type ModelPreferences struct {
	Hints                []ModelHint `json:"hints,omitempty"`
	CostPriority         float64     `json:"costPriority,omitempty"`
	SpeedPriority        float64     `json:"speedPriority,omitempty"`
	IntelligencePriority float64     `json:"intelligencePriority,omitempty"`
}

// ModelHint suggests a model name or partial pattern.
type ModelHint struct {
	Name string `json:"name,omitempty"`
}

// SamplingRequest is the params for sampling/createMessage.
type SamplingRequest struct {
	Messages         []SamplingMessage `json:"messages"`
	ModelPreferences *ModelPreferences `json:"modelPreferences,omitempty"`
	SystemPrompt     string            `json:"systemPrompt,omitempty"`
	IncludeContext   string            `json:"includeContext,omitempty"` // "none"|"thisServer"|"allServers"
	Temperature      *float64          `json:"temperature,omitempty"`
	MaxTokens        int               `json:"maxTokens"`
	StopSequences    []string          `json:"stopSequences,omitempty"`
	Metadata         json.RawMessage   `json:"metadata,omitempty"`
}

// SamplingResult is the response for sampling/createMessage.
type SamplingResult struct {
	Role    string      `json:"role"`
	Content ContentItem `json:"content"`
	Model   string      `json:"model"`
	// StopReason indicates why the model stopped generating.
	StopReason string `json:"stopReason,omitempty"`
}

// ── Sampling handler ────────────────────────────────────────────────────────

// SamplingHandler processes sampling/createMessage requests from MCP servers.
type SamplingHandler interface {
	// HandleSamplingRequest processes a sampling request and returns a result.
	HandleSamplingRequest(ctx context.Context, serverName string, req *SamplingRequest) (*SamplingResult, error)
}

// NoopSamplingHandler rejects all sampling requests.
type NoopSamplingHandler struct{}

// HandleSamplingRequest rejects the request.
func (NoopSamplingHandler) HandleSamplingRequest(_ context.Context, serverName string, _ *SamplingRequest) (*SamplingResult, error) {
	return nil, fmt.Errorf("sampling not supported (server: %s)", serverName)
}

// SamplingHandlerFunc is an adapter to use a function as a SamplingHandler.
type SamplingHandlerFunc func(ctx context.Context, serverName string, req *SamplingRequest) (*SamplingResult, error)

// HandleSamplingRequest calls the underlying function.
func (f SamplingHandlerFunc) HandleSamplingRequest(ctx context.Context, serverName string, req *SamplingRequest) (*SamplingResult, error) {
	return f(ctx, serverName, req)
}

// RegisterSamplingHandler wires a SamplingHandler into a Client so that
// incoming sampling/createMessage requests are routed through it.
func RegisterSamplingHandler(client *Client, handler SamplingHandler) {
	serverName := client.Name()
	client.SetSamplingHandler(func(ctx context.Context, req *Request) (*Response, error) {
		// Parse the sampling request params.
		var samplingReq SamplingRequest
		if req.Params != nil {
			if err := json.Unmarshal(req.Params, &samplingReq); err != nil {
				return makeErrorResponse(req.ID, -32602, "invalid sampling params: "+err.Error()), nil
			}
		}

		slog.Debug("mcp: sampling request received",
			slog.String("server", serverName),
			slog.Int("messages", len(samplingReq.Messages)),
			slog.Int("maxTokens", samplingReq.MaxTokens))

		result, err := handler.HandleSamplingRequest(ctx, serverName, &samplingReq)
		if err != nil {
			slog.Warn("mcp: sampling handler error",
				slog.String("server", serverName),
				slog.Any("err", err))
			return makeErrorResponse(req.ID, -32603, err.Error()), nil
		}

		resultJSON, err := json.Marshal(result)
		if err != nil {
			return makeErrorResponse(req.ID, -32603, "marshal sampling result: "+err.Error()), nil
		}

		return &Response{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result:  resultJSON,
		}, nil
	})
}

// RegisterSamplingHandlerOnManager registers a SamplingHandler on all current
// and future connected clients in the Manager.
func RegisterSamplingHandlerOnManager(m *Manager, handler SamplingHandler) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	for _, c := range m.clients {
		RegisterSamplingHandler(c, handler)
	}
}

// makeErrorResponse builds a JSON-RPC error response.
func makeErrorResponse(id JSONRPCID, code int, message string) *Response {
	errData, _ := json.Marshal(&RPCError{
		Code:    code,
		Message: message,
	})
	return &Response{
		JSONRPC: "2.0",
		ID:      id,
		Error:   &RPCError{Code: code, Message: message},
		Result:  errData,
	}
}
