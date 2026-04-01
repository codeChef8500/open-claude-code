package agent

import (
	"fmt"
	"sync"
)

// MessageBus routes AgentMessages between agents via per-agent buffered channels.
type MessageBus struct {
	mu    sync.RWMutex
	queues map[string]chan AgentMessage
}

// NewMessageBus creates an empty MessageBus.
func NewMessageBus() *MessageBus {
	return &MessageBus{queues: make(map[string]chan AgentMessage)}
}

// Subscribe registers a receive channel for agentID. Returns error if already subscribed.
func (mb *MessageBus) Subscribe(agentID string, bufSize int) (<-chan AgentMessage, error) {
	if bufSize <= 0 {
		bufSize = 64
	}
	mb.mu.Lock()
	defer mb.mu.Unlock()
	if _, exists := mb.queues[agentID]; exists {
		return mb.queues[agentID], nil
	}
	ch := make(chan AgentMessage, bufSize)
	mb.queues[agentID] = ch
	return ch, nil
}

// Unsubscribe removes and closes the channel for agentID.
func (mb *MessageBus) Unsubscribe(agentID string) {
	mb.mu.Lock()
	defer mb.mu.Unlock()
	if ch, ok := mb.queues[agentID]; ok {
		close(ch)
		delete(mb.queues, agentID)
	}
}

// Send routes a message to the destination agent's channel.
// Returns an error if the destination has no channel or the channel is full.
func (mb *MessageBus) Send(msg AgentMessage) error {
	mb.mu.RLock()
	ch, ok := mb.queues[msg.ToAgentID]
	mb.mu.RUnlock()
	if !ok {
		return fmt.Errorf("agent %q has no message channel", msg.ToAgentID)
	}
	select {
	case ch <- msg:
		return nil
	default:
		return fmt.Errorf("message queue for agent %q is full", msg.ToAgentID)
	}
}

// Broadcast sends a message to all subscribed agents except the sender.
func (mb *MessageBus) Broadcast(fromAgentID string, content interface{}) {
	mb.mu.RLock()
	defer mb.mu.RUnlock()
	for id, ch := range mb.queues {
		if id == fromAgentID {
			continue
		}
		select {
		case ch <- AgentMessage{FromAgentID: fromAgentID, ToAgentID: id, Content: content}:
		default:
		}
	}
}
