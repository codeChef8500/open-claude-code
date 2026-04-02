package state

import (
	"sync"
	"time"
)

// Event represents a typed event in the event bus.
type Event struct {
	Type      string
	Payload   interface{}
	Timestamp time.Time
}

// EventHandler is a callback for a specific event type.
type EventHandler func(Event)

// EventBus is a simple publish-subscribe event bus for decoupling
// components within the agent engine. It supports typed event channels
// and wildcard subscriptions.
type EventBus struct {
	mu       sync.RWMutex
	handlers map[string][]eventSub
	nextID   int
}

type eventSub struct {
	id      int
	handler EventHandler
}

// NewEventBus creates an empty event bus.
func NewEventBus() *EventBus {
	return &EventBus{
		handlers: make(map[string][]eventSub),
	}
}

// Subscribe registers a handler for the given event type.
// Returns a subscription ID that can be used to unsubscribe.
func (b *EventBus) Subscribe(eventType string, handler EventHandler) int {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.nextID++
	id := b.nextID
	b.handlers[eventType] = append(b.handlers[eventType], eventSub{
		id:      id,
		handler: handler,
	})
	return id
}

// SubscribeAll registers a handler for all event types (wildcard).
func (b *EventBus) SubscribeAll(handler EventHandler) int {
	return b.Subscribe("*", handler)
}

// Unsubscribe removes a handler by its subscription ID.
func (b *EventBus) Unsubscribe(id int) {
	b.mu.Lock()
	defer b.mu.Unlock()
	for eventType, subs := range b.handlers {
		filtered := subs[:0]
		for _, s := range subs {
			if s.id != id {
				filtered = append(filtered, s)
			}
		}
		b.handlers[eventType] = filtered
	}
}

// Publish sends an event to all matching subscribers synchronously.
func (b *EventBus) Publish(eventType string, payload interface{}) {
	ev := Event{
		Type:      eventType,
		Payload:   payload,
		Timestamp: time.Now(),
	}

	b.mu.RLock()
	// Collect matching handlers: exact match + wildcard.
	var handlers []EventHandler
	for _, s := range b.handlers[eventType] {
		handlers = append(handlers, s.handler)
	}
	for _, s := range b.handlers["*"] {
		handlers = append(handlers, s.handler)
	}
	b.mu.RUnlock()

	for _, h := range handlers {
		h(ev)
	}
}

// PublishAsync sends an event to all matching subscribers asynchronously.
func (b *EventBus) PublishAsync(eventType string, payload interface{}) {
	ev := Event{
		Type:      eventType,
		Payload:   payload,
		Timestamp: time.Now(),
	}

	b.mu.RLock()
	var handlers []EventHandler
	for _, s := range b.handlers[eventType] {
		handlers = append(handlers, s.handler)
	}
	for _, s := range b.handlers["*"] {
		handlers = append(handlers, s.handler)
	}
	b.mu.RUnlock()

	for _, h := range handlers {
		go h(ev)
	}
}

// ── Well-known event types ──────────────────────────────────────────────────

const (
	EventSessionStart    = "session.start"
	EventSessionEnd      = "session.end"
	EventTurnStart       = "turn.start"
	EventTurnEnd         = "turn.end"
	EventToolStart       = "tool.start"
	EventToolEnd         = "tool.end"
	EventCompactStart    = "compact.start"
	EventCompactEnd      = "compact.end"
	EventCostUpdate      = "cost.update"
	EventTokenWarning    = "token.warning"
	EventPermissionAsk   = "permission.ask"
	EventPermissionGrant = "permission.grant"
	EventPermissionDeny  = "permission.deny"
	EventConfigChange    = "config.change"
	EventError           = "error"
)

// ── Typed payloads ──────────────────────────────────────────────────────────

// TurnPayload is the payload for turn start/end events.
type TurnPayload struct {
	TurnNumber int
	SessionID  string
}

// ToolPayload is the payload for tool start/end events.
type ToolPayload struct {
	ToolID   string
	ToolName string
	Input    string
	Output   string
	IsError  bool
	Duration time.Duration
}

// CostPayload is the payload for cost update events.
type CostPayload struct {
	TurnCostUSD  float64
	TotalCostUSD float64
	InputTokens  int
	OutputTokens int
}

// ErrorPayload is the payload for error events.
type ErrorPayload struct {
	Error   error
	Code    string
	Context string
}
