package provider

import (
	"fmt"
	"sync"
	"time"
)

// CircuitState represents the state of a circuit breaker.
type CircuitState int

const (
	CircuitClosed   CircuitState = iota // normal operation
	CircuitOpen                         // failing, reject calls
	CircuitHalfOpen                     // testing if recovery happened
)

func (s CircuitState) String() string {
	switch s {
	case CircuitClosed:
		return "closed"
	case CircuitOpen:
		return "open"
	case CircuitHalfOpen:
		return "half-open"
	default:
		return "unknown"
	}
}

// CircuitBreakerConfig configures a circuit breaker.
type CircuitBreakerConfig struct {
	// FailureThreshold is the number of consecutive failures before opening.
	FailureThreshold int
	// ResetTimeout is how long the circuit stays open before half-opening.
	ResetTimeout time.Duration
	// HalfOpenMaxAttempts is the number of test requests in half-open state.
	HalfOpenMaxAttempts int
}

// DefaultCircuitBreakerConfig returns sensible defaults.
func DefaultCircuitBreakerConfig() CircuitBreakerConfig {
	return CircuitBreakerConfig{
		FailureThreshold:    5,
		ResetTimeout:        30 * time.Second,
		HalfOpenMaxAttempts: 1,
	}
}

// CircuitBreaker implements the circuit breaker pattern for API calls.
type CircuitBreaker struct {
	mu     sync.Mutex
	config CircuitBreakerConfig

	state          CircuitState
	failures       int
	successes      int
	lastFailure    time.Time
	lastStateChange time.Time
}

// NewCircuitBreaker creates a new circuit breaker.
func NewCircuitBreaker(cfg CircuitBreakerConfig) *CircuitBreaker {
	if cfg.FailureThreshold <= 0 {
		cfg.FailureThreshold = 5
	}
	if cfg.ResetTimeout <= 0 {
		cfg.ResetTimeout = 30 * time.Second
	}
	if cfg.HalfOpenMaxAttempts <= 0 {
		cfg.HalfOpenMaxAttempts = 1
	}
	return &CircuitBreaker{
		config:          cfg,
		state:           CircuitClosed,
		lastStateChange: time.Now(),
	}
}

// Allow checks if a request should be allowed through.
func (cb *CircuitBreaker) Allow() error {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	switch cb.state {
	case CircuitClosed:
		return nil
	case CircuitOpen:
		if time.Since(cb.lastFailure) >= cb.config.ResetTimeout {
			cb.state = CircuitHalfOpen
			cb.successes = 0
			cb.lastStateChange = time.Now()
			return nil
		}
		return fmt.Errorf("circuit breaker open: %d consecutive failures, retry after %v",
			cb.failures, cb.config.ResetTimeout-time.Since(cb.lastFailure))
	case CircuitHalfOpen:
		return nil
	}
	return nil
}

// RecordSuccess records a successful call.
func (cb *CircuitBreaker) RecordSuccess() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	switch cb.state {
	case CircuitHalfOpen:
		cb.successes++
		if cb.successes >= cb.config.HalfOpenMaxAttempts {
			cb.state = CircuitClosed
			cb.failures = 0
			cb.lastStateChange = time.Now()
		}
	case CircuitClosed:
		cb.failures = 0
	}
}

// RecordFailure records a failed call.
func (cb *CircuitBreaker) RecordFailure() {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	cb.lastFailure = time.Now()

	switch cb.state {
	case CircuitClosed:
		cb.failures++
		if cb.failures >= cb.config.FailureThreshold {
			cb.state = CircuitOpen
			cb.lastStateChange = time.Now()
		}
	case CircuitHalfOpen:
		cb.state = CircuitOpen
		cb.lastStateChange = time.Now()
	}
}

// State returns the current circuit state.
func (cb *CircuitBreaker) State() CircuitState {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	return cb.state
}

// Reset forces the circuit back to closed state.
func (cb *CircuitBreaker) Reset() {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	cb.state = CircuitClosed
	cb.failures = 0
	cb.successes = 0
	cb.lastStateChange = time.Now()
}

// Stats returns current circuit breaker statistics.
func (cb *CircuitBreaker) Stats() CircuitBreakerStats {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	return CircuitBreakerStats{
		State:           cb.state,
		Failures:        cb.failures,
		LastFailure:     cb.lastFailure,
		LastStateChange: cb.lastStateChange,
	}
}

// CircuitBreakerStats holds circuit breaker statistics.
type CircuitBreakerStats struct {
	State           CircuitState
	Failures        int
	LastFailure     time.Time
	LastStateChange time.Time
}
