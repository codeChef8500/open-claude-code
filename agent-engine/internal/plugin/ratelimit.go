package plugin

import (
	"fmt"
	"sync"
	"time"
)

// RateLimitConfig defines rate limits for plugin hook invocations.
type RateLimitConfig struct {
	// MaxPerMinute is the maximum number of invocations per minute (0 = unlimited).
	MaxPerMinute int
	// MaxPerHour is the maximum number of invocations per hour (0 = unlimited).
	MaxPerHour int
	// BurstSize is the maximum burst allowed above the per-minute limit (default = MaxPerMinute).
	BurstSize int
}

// RateLimiter is a sliding-window rate limiter for plugin hook calls.
type RateLimiter struct {
	cfg    RateLimitConfig
	mu     sync.Mutex
	minute []time.Time
	hour   []time.Time
}

// NewRateLimiter creates a RateLimiter from config.
func NewRateLimiter(cfg RateLimitConfig) *RateLimiter {
	return &RateLimiter{cfg: cfg}
}

// Allow reports whether the next invocation is permitted.
// It records the invocation if permitted.
func (r *RateLimiter) Allow() (bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	now := time.Now()
	r.prune(now)

	if r.cfg.MaxPerMinute > 0 && len(r.minute) >= r.cfg.MaxPerMinute {
		return false, fmt.Errorf("rate limit exceeded: %d calls/minute", r.cfg.MaxPerMinute)
	}
	if r.cfg.MaxPerHour > 0 && len(r.hour) >= r.cfg.MaxPerHour {
		return false, fmt.Errorf("rate limit exceeded: %d calls/hour", r.cfg.MaxPerHour)
	}

	r.minute = append(r.minute, now)
	r.hour = append(r.hour, now)
	return true, nil
}

// Stats returns current usage counts within the sliding windows.
func (r *RateLimiter) Stats() (perMinute, perHour int) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.prune(time.Now())
	return len(r.minute), len(r.hour)
}

// prune removes timestamps outside the sliding windows.
// Must be called with r.mu held.
func (r *RateLimiter) prune(now time.Time) {
	cutMinute := now.Add(-time.Minute)
	cutHour := now.Add(-time.Hour)

	m := r.minute[:0]
	for _, t := range r.minute {
		if t.After(cutMinute) {
			m = append(m, t)
		}
	}
	r.minute = m

	h := r.hour[:0]
	for _, t := range r.hour {
		if t.After(cutHour) {
			h = append(h, t)
		}
	}
	r.hour = h
}

// PluginRateLimiterMap manages per-plugin rate limiters.
type PluginRateLimiterMap struct {
	mu       sync.RWMutex
	limiters map[string]*RateLimiter
}

// NewPluginRateLimiterMap creates an empty map.
func NewPluginRateLimiterMap() *PluginRateLimiterMap {
	return &PluginRateLimiterMap{limiters: make(map[string]*RateLimiter)}
}

// Set registers a rate limiter for a named plugin.
func (m *PluginRateLimiterMap) Set(pluginName string, cfg RateLimitConfig) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.limiters[pluginName] = NewRateLimiter(cfg)
}

// Allow checks and records an invocation for the named plugin.
// Returns true and nil if no limiter is registered for that plugin.
func (m *PluginRateLimiterMap) Allow(pluginName string) (bool, error) {
	m.mu.RLock()
	rl, ok := m.limiters[pluginName]
	m.mu.RUnlock()
	if !ok {
		return true, nil
	}
	return rl.Allow()
}
