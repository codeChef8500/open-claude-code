package statusline

import (
	"sync"
	"time"

	"github.com/charmbracelet/lipgloss"
)

// NotificationType classifies a notification.
type NotificationType int

const (
	NotifyInfo    NotificationType = iota
	NotifySuccess
	NotifyWarning
	NotifyError
)

// Notification is a temporary message displayed in the status area.
type Notification struct {
	Type      NotificationType
	Text      string
	CreatedAt time.Time
	Duration  time.Duration
}

// IsExpired returns true if the notification has exceeded its display duration.
func (n *Notification) IsExpired() bool {
	return time.Since(n.CreatedAt) > n.Duration
}

// NotificationManager manages a queue of temporary notifications.
type NotificationManager struct {
	mu            sync.Mutex
	notifications []*Notification
	maxItems      int
	defaultTTL    time.Duration
}

// NewNotificationManager creates a notification manager.
func NewNotificationManager() *NotificationManager {
	return &NotificationManager{
		maxItems:   5,
		defaultTTL: 5 * time.Second,
	}
}

// Push adds a notification.
func (nm *NotificationManager) Push(typ NotificationType, text string) {
	nm.mu.Lock()
	defer nm.mu.Unlock()

	n := &Notification{
		Type:      typ,
		Text:      text,
		CreatedAt: time.Now(),
		Duration:  nm.defaultTTL,
	}

	nm.notifications = append(nm.notifications, n)
	if len(nm.notifications) > nm.maxItems {
		nm.notifications = nm.notifications[len(nm.notifications)-nm.maxItems:]
	}
}

// PushWithDuration adds a notification with a custom duration.
func (nm *NotificationManager) PushWithDuration(typ NotificationType, text string, dur time.Duration) {
	nm.mu.Lock()
	defer nm.mu.Unlock()

	n := &Notification{
		Type:      typ,
		Text:      text,
		CreatedAt: time.Now(),
		Duration:  dur,
	}

	nm.notifications = append(nm.notifications, n)
	if len(nm.notifications) > nm.maxItems {
		nm.notifications = nm.notifications[len(nm.notifications)-nm.maxItems:]
	}
}

// Active returns all non-expired notifications.
func (nm *NotificationManager) Active() []*Notification {
	nm.mu.Lock()
	defer nm.mu.Unlock()

	var active []*Notification
	for _, n := range nm.notifications {
		if !n.IsExpired() {
			active = append(active, n)
		}
	}

	// Prune expired
	nm.notifications = active
	return active
}

// HasActive returns true if there are visible notifications.
func (nm *NotificationManager) HasActive() bool {
	return len(nm.Active()) > 0
}

// Latest returns the most recent non-expired notification, or nil.
func (nm *NotificationManager) Latest() *Notification {
	active := nm.Active()
	if len(active) == 0 {
		return nil
	}
	return active[len(active)-1]
}

// Clear removes all notifications.
func (nm *NotificationManager) Clear() {
	nm.mu.Lock()
	defer nm.mu.Unlock()
	nm.notifications = nil
}

// RenderNotification renders a single notification with appropriate styling.
func RenderNotification(n *Notification, theme Theme) string {
	switch n.Type {
	case NotifySuccess:
		return theme.Context.Render("✓ " + n.Text)
	case NotifyWarning:
		return theme.Mode.Render("⚠ " + n.Text)
	case NotifyError:
		return theme.Warning.Render("✗ " + n.Text)
	default:
		return theme.Dim.Render("▶ " + n.Text)
	}
}

// RenderNotifications renders all active notifications as a single line.
func RenderNotifications(nm *NotificationManager, width int, theme Theme) string {
	active := nm.Active()
	if len(active) == 0 {
		return ""
	}

	// Show only the latest notification in the status bar
	n := active[len(active)-1]
	rendered := RenderNotification(n, theme)

	// Truncate to width
	if lipgloss.Width(rendered) > width {
		rendered = rendered[:width]
	}

	return rendered
}
