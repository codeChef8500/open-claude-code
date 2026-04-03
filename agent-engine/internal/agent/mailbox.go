package agent

import (
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
)

// ────────────────────────────────────────────────────────────────────────────
// Mailbox — persistent inbox for teammate agents in swarm sessions.
// Aligned with claude-code-main's teammate mailbox / inbox pattern.
// ────────────────────────────────────────────────────────────────────────────

// MailboxMessage is a single message stored in a teammate's mailbox.
type MailboxMessage struct {
	ID        string    `json:"id"`
	From      string    `json:"from"`
	To        string    `json:"to"`
	Text      string    `json:"text"`
	Timestamp time.Time `json:"timestamp"`
	Status    MailboxMessageStatus `json:"status"`
	Priority  MailboxPriority      `json:"priority,omitempty"`
	ReplyTo   string               `json:"reply_to,omitempty"` // ID of message being replied to
}

// MailboxMessageStatus tracks the lifecycle of a mailbox message.
type MailboxMessageStatus string

const (
	MailboxStatusPending    MailboxMessageStatus = "pending"
	MailboxStatusRead       MailboxMessageStatus = "read"
	MailboxStatusProcessing MailboxMessageStatus = "processing"
	MailboxStatusProcessed  MailboxMessageStatus = "processed"
	MailboxStatusExpired    MailboxMessageStatus = "expired"
)

// MailboxPriority controls message ordering.
type MailboxPriority string

const (
	MailboxPriorityNormal MailboxPriority = "normal"
	MailboxPriorityHigh   MailboxPriority = "high"
	MailboxPriorityLow    MailboxPriority = "low"
)

// Mailbox is a thread-safe per-agent inbox that supports read, ack, and
// expiration semantics.
type Mailbox struct {
	mu       sync.Mutex
	agentID  string
	messages []MailboxMessage
	maxSize  int
	ttl      time.Duration // 0 = no expiry
}

// NewMailbox creates a mailbox for the given agent.
func NewMailbox(agentID string, maxSize int, ttl time.Duration) *Mailbox {
	if maxSize <= 0 {
		maxSize = 256
	}
	return &Mailbox{
		agentID: agentID,
		maxSize: maxSize,
		ttl:     ttl,
	}
}

// Deliver adds a message to the mailbox. Returns error if the mailbox is full.
func (mb *Mailbox) Deliver(from, text string, priority MailboxPriority, replyTo string) (string, error) {
	mb.mu.Lock()
	defer mb.mu.Unlock()

	// Expire old messages first.
	mb.expireLocked()

	if len(mb.messages) >= mb.maxSize {
		return "", fmt.Errorf("mailbox for agent %q is full (%d messages)", mb.agentID, mb.maxSize)
	}

	if priority == "" {
		priority = MailboxPriorityNormal
	}

	msg := MailboxMessage{
		ID:        uuid.New().String(),
		From:      from,
		To:        mb.agentID,
		Text:      text,
		Timestamp: time.Now(),
		Status:    MailboxStatusPending,
		Priority:  priority,
		ReplyTo:   replyTo,
	}
	mb.messages = append(mb.messages, msg)
	return msg.ID, nil
}

// Peek returns all pending (unread) messages without changing their status.
func (mb *Mailbox) Peek() []MailboxMessage {
	mb.mu.Lock()
	defer mb.mu.Unlock()
	mb.expireLocked()

	var pending []MailboxMessage
	for _, m := range mb.messages {
		if m.Status == MailboxStatusPending {
			pending = append(pending, m)
		}
	}
	return pending
}

// Read returns all pending messages and marks them as read.
func (mb *Mailbox) Read() []MailboxMessage {
	mb.mu.Lock()
	defer mb.mu.Unlock()
	mb.expireLocked()

	var pending []MailboxMessage
	for i := range mb.messages {
		if mb.messages[i].Status == MailboxStatusPending {
			mb.messages[i].Status = MailboxStatusRead
			pending = append(pending, mb.messages[i])
		}
	}
	return pending
}

// Ack marks a message as processed by its ID.
func (mb *Mailbox) Ack(msgID string) bool {
	mb.mu.Lock()
	defer mb.mu.Unlock()

	for i := range mb.messages {
		if mb.messages[i].ID == msgID {
			mb.messages[i].Status = MailboxStatusProcessed
			return true
		}
	}
	return false
}

// MarkProcessing marks a message as being actively worked on.
func (mb *Mailbox) MarkProcessing(msgID string) bool {
	mb.mu.Lock()
	defer mb.mu.Unlock()

	for i := range mb.messages {
		if mb.messages[i].ID == msgID {
			mb.messages[i].Status = MailboxStatusProcessing
			return true
		}
	}
	return false
}

// PendingCount returns the number of unread messages.
func (mb *Mailbox) PendingCount() int {
	mb.mu.Lock()
	defer mb.mu.Unlock()
	mb.expireLocked()

	count := 0
	for _, m := range mb.messages {
		if m.Status == MailboxStatusPending {
			count++
		}
	}
	return count
}

// All returns all messages regardless of status.
func (mb *Mailbox) All() []MailboxMessage {
	mb.mu.Lock()
	defer mb.mu.Unlock()
	out := make([]MailboxMessage, len(mb.messages))
	copy(out, mb.messages)
	return out
}

// Clear removes all messages from the mailbox.
func (mb *Mailbox) Clear() {
	mb.mu.Lock()
	defer mb.mu.Unlock()
	mb.messages = nil
}

func (mb *Mailbox) expireLocked() {
	if mb.ttl <= 0 {
		return
	}
	cutoff := time.Now().Add(-mb.ttl)
	filtered := mb.messages[:0]
	for _, m := range mb.messages {
		if m.Timestamp.After(cutoff) || m.Status == MailboxStatusProcessing {
			filtered = append(filtered, m)
		}
	}
	mb.messages = filtered
}

// ────────────────────────────────────────────────────────────────────────────
// MailboxRegistry — manages mailboxes for all agents in a swarm.
// ────────────────────────────────────────────────────────────────────────────

// MailboxRegistry holds all agent mailboxes.
type MailboxRegistry struct {
	mu        sync.RWMutex
	mailboxes map[string]*Mailbox
	maxSize   int
	ttl       time.Duration
}

// NewMailboxRegistry creates a registry with default mailbox settings.
func NewMailboxRegistry(maxSize int, ttl time.Duration) *MailboxRegistry {
	return &MailboxRegistry{
		mailboxes: make(map[string]*Mailbox),
		maxSize:   maxSize,
		ttl:       ttl,
	}
}

// GetOrCreate returns the mailbox for an agent, creating it if needed.
func (mr *MailboxRegistry) GetOrCreate(agentID string) *Mailbox {
	mr.mu.Lock()
	defer mr.mu.Unlock()
	if mb, ok := mr.mailboxes[agentID]; ok {
		return mb
	}
	mb := NewMailbox(agentID, mr.maxSize, mr.ttl)
	mr.mailboxes[agentID] = mb
	return mb
}

// Send delivers a message from one agent to another.
func (mr *MailboxRegistry) Send(from, to, text string, priority MailboxPriority, replyTo string) (string, error) {
	mb := mr.GetOrCreate(to)
	return mb.Deliver(from, text, priority, replyTo)
}

// Remove destroys a mailbox for an agent.
func (mr *MailboxRegistry) Remove(agentID string) {
	mr.mu.Lock()
	defer mr.mu.Unlock()
	if mb, ok := mr.mailboxes[agentID]; ok {
		mb.Clear()
		delete(mr.mailboxes, agentID)
	}
}

// AllPendingCounts returns pending message counts per agent.
func (mr *MailboxRegistry) AllPendingCounts() map[string]int {
	mr.mu.RLock()
	defer mr.mu.RUnlock()
	counts := make(map[string]int, len(mr.mailboxes))
	for id, mb := range mr.mailboxes {
		counts[id] = mb.PendingCount()
	}
	return counts
}
