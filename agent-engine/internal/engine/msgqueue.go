package engine

import "sync"

// MessagePriority controls when a queued message is delivered relative to the
// current query loop.
type MessagePriority int

const (
	// PriorityNow causes the message to be injected into the current turn
	// immediately (before any more tool calls are executed).
	PriorityNow MessagePriority = iota
	// PriorityNext causes the message to be delivered at the start of the
	// next model turn (after the current tool calls complete).
	PriorityNext
	// PriorityLater causes the message to be appended after the current
	// query loop completes, as the first message of the subsequent loop.
	PriorityLater
)

// QueuedMessage is a pending user or system message waiting to be injected
// into the query loop.
type QueuedMessage struct {
	// Role is the message role (RoleUser or RoleSystem).
	Role MessageRole
	// Text is the content to inject.
	Text string
	// Priority controls when the message is delivered.
	Priority MessagePriority
	// ToolUseID, if set, makes the message a tool-result injection rather
	// than a standalone user message.
	ToolUseID string
}

// MessageQueue is a thread-safe priority queue for messages that need to be
// injected into the query loop out-of-band (e.g. hook responses, interrupt
// notifications, plan-mode approvals).
type MessageQueue struct {
	mu   sync.Mutex
	msgs []*QueuedMessage
}

// Enqueue adds a message to the queue.
func (q *MessageQueue) Enqueue(msg *QueuedMessage) {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.msgs = append(q.msgs, msg)
}

// DrainPriority removes and returns all messages at or above the given
// priority level, in FIFO order.
func (q *MessageQueue) DrainPriority(maxPrio MessagePriority) []*QueuedMessage {
	q.mu.Lock()
	defer q.mu.Unlock()

	var out, remaining []*QueuedMessage
	for _, m := range q.msgs {
		if m.Priority <= maxPrio {
			out = append(out, m)
		} else {
			remaining = append(remaining, m)
		}
	}
	q.msgs = remaining
	return out
}

// DrainAll removes and returns every queued message in FIFO order.
func (q *MessageQueue) DrainAll() []*QueuedMessage {
	q.mu.Lock()
	defer q.mu.Unlock()
	out := q.msgs
	q.msgs = nil
	return out
}

// Len returns the number of pending messages.
func (q *MessageQueue) Len() int {
	q.mu.Lock()
	defer q.mu.Unlock()
	return len(q.msgs)
}

// HasPriority reports whether any message at or above the given priority is
// waiting, without removing it.
func (q *MessageQueue) HasPriority(maxPrio MessagePriority) bool {
	q.mu.Lock()
	defer q.mu.Unlock()
	for _, m := range q.msgs {
		if m.Priority <= maxPrio {
			return true
		}
	}
	return false
}
