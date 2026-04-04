package agent

import (
	"encoding/xml"
	"fmt"
	"strings"
	"sync"
	"time"
)

// Notification system aligned with claude-code-main's notification injection
// mechanism. Notifications are XML-formatted messages injected into the parent
// agent's conversation to inform it about child agent events.

// NotificationType enumerates the kinds of notifications.
type NotificationType string

const (
	NotificationTypeProgress NotificationType = "progress"
	NotificationTypeComplete NotificationType = "complete"
	NotificationTypeError    NotificationType = "error"
	NotificationTypeMessage  NotificationType = "message"
	NotificationTypeStatus   NotificationType = "status"
)

// Notification represents a single notification from a child agent to its parent.
type Notification struct {
	Type      NotificationType `json:"type"`
	AgentID   string           `json:"agent_id"`
	AgentType string           `json:"agent_type,omitempty"`
	Message   string           `json:"message"`
	Timestamp time.Time        `json:"timestamp"`
}

// NotificationQueue is a bounded, thread-safe queue of notifications.
type NotificationQueue struct {
	mu       sync.Mutex
	items    []Notification
	capacity int
}

// NewNotificationQueue creates a queue with the given capacity.
func NewNotificationQueue(capacity int) *NotificationQueue {
	return &NotificationQueue{
		items:    make([]Notification, 0, capacity),
		capacity: capacity,
	}
}

// Push adds a notification to the queue. If at capacity, the oldest is dropped.
func (q *NotificationQueue) Push(n Notification) {
	q.mu.Lock()
	defer q.mu.Unlock()

	if n.Timestamp.IsZero() {
		n.Timestamp = time.Now()
	}

	if len(q.items) >= q.capacity {
		// Drop oldest.
		q.items = q.items[1:]
	}
	q.items = append(q.items, n)
}

// DrainAll returns all queued notifications and clears the queue.
func (q *NotificationQueue) DrainAll() []Notification {
	q.mu.Lock()
	defer q.mu.Unlock()

	if len(q.items) == 0 {
		return nil
	}

	result := make([]Notification, len(q.items))
	copy(result, q.items)
	q.items = q.items[:0]
	return result
}

// Peek returns the most recent notification without removing it.
func (q *NotificationQueue) Peek() *Notification {
	q.mu.Lock()
	defer q.mu.Unlock()

	if len(q.items) == 0 {
		return nil
	}
	n := q.items[len(q.items)-1]
	return &n
}

// Len returns the current number of queued notifications.
func (q *NotificationQueue) Len() int {
	q.mu.Lock()
	defer q.mu.Unlock()
	return len(q.items)
}

// ── XML Formatting ──────────────────────────────────────────────────────────
// claude-code-main injects notifications as XML blocks into the conversation.

// xmlNotification is the XML structure for a single notification.
type xmlNotification struct {
	XMLName xml.Name `xml:"notification"`
	Type    string   `xml:"type,attr"`
	AgentID string   `xml:"agent_id,attr"`
	Time    string   `xml:"time,attr,omitempty"`
	Body    string   `xml:",chardata"`
}

// xmlNotifications wraps multiple notifications.
type xmlNotifications struct {
	XMLName       xml.Name          `xml:"agent_notifications"`
	Notifications []xmlNotification `xml:"notification"`
}

// FormatNotificationsXML formats a slice of notifications as an XML block
// suitable for injection into the conversation as a system message.
// Aligned with claude-code-main's notification XML format.
func FormatNotificationsXML(notifications []Notification) string {
	if len(notifications) == 0 {
		return ""
	}

	xns := xmlNotifications{
		Notifications: make([]xmlNotification, len(notifications)),
	}

	for i, n := range notifications {
		xns.Notifications[i] = xmlNotification{
			Type:    string(n.Type),
			AgentID: truncID(n.AgentID),
			Time:    n.Timestamp.Format(time.RFC3339),
			Body:    n.Message,
		}
	}

	data, err := xml.MarshalIndent(xns, "", "  ")
	if err != nil {
		// Fallback to plain text.
		return formatNotificationsPlain(notifications)
	}

	return string(data)
}

// formatNotificationsPlain is a fallback when XML marshalling fails.
func formatNotificationsPlain(notifications []Notification) string {
	var sb strings.Builder
	sb.WriteString("--- Agent Notifications ---\n")
	for _, n := range notifications {
		sb.WriteString(fmt.Sprintf("[%s] %s (%s): %s\n",
			n.Type, truncID(n.AgentID), n.AgentType, n.Message))
	}
	sb.WriteString("--- End Notifications ---\n")
	return sb.String()
}

// FormatSingleNotificationXML formats a single notification as XML.
func FormatSingleNotificationXML(n Notification) string {
	xn := xmlNotification{
		Type:    string(n.Type),
		AgentID: truncID(n.AgentID),
		Time:    n.Timestamp.Format(time.RFC3339),
		Body:    n.Message,
	}

	data, err := xml.MarshalIndent(xn, "", "  ")
	if err != nil {
		return fmt.Sprintf("[%s] %s: %s", n.Type, truncID(n.AgentID), n.Message)
	}
	return string(data)
}

// NotificationAggregator collects notifications from multiple agents
// and provides a unified drain interface for the parent.
type NotificationAggregator struct {
	mu     sync.Mutex
	queues map[string]*NotificationQueue
}

// NewNotificationAggregator creates a new aggregator.
func NewNotificationAggregator() *NotificationAggregator {
	return &NotificationAggregator{
		queues: make(map[string]*NotificationQueue),
	}
}

// RegisterAgent creates a notification queue for an agent.
func (a *NotificationAggregator) RegisterAgent(agentID string, capacity int) *NotificationQueue {
	a.mu.Lock()
	defer a.mu.Unlock()

	q := NewNotificationQueue(capacity)
	a.queues[agentID] = q
	return q
}

// DrainAllAgents returns all pending notifications from all agents.
func (a *NotificationAggregator) DrainAllAgents() []Notification {
	a.mu.Lock()
	defer a.mu.Unlock()

	var all []Notification
	for _, q := range a.queues {
		all = append(all, q.DrainAll()...)
	}
	return all
}

// DrainAgent returns pending notifications for a specific agent.
func (a *NotificationAggregator) DrainAgent(agentID string) []Notification {
	a.mu.Lock()
	q, ok := a.queues[agentID]
	a.mu.Unlock()

	if !ok {
		return nil
	}
	return q.DrainAll()
}

// RemoveAgent removes an agent's queue from the aggregator.
func (a *NotificationAggregator) RemoveAgent(agentID string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	delete(a.queues, agentID)
}
