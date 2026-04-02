package provider

import (
	"testing"
	"time"
)

func TestCircuitBreaker_StartsClosedAndAllows(t *testing.T) {
	cb := NewCircuitBreaker(DefaultCircuitBreakerConfig())
	if cb.State() != CircuitClosed {
		t.Errorf("expected closed, got %v", cb.State())
	}
	if err := cb.Allow(); err != nil {
		t.Errorf("expected nil, got %v", err)
	}
}

func TestCircuitBreaker_OpensAfterThreshold(t *testing.T) {
	cfg := CircuitBreakerConfig{FailureThreshold: 3, ResetTimeout: time.Second}
	cb := NewCircuitBreaker(cfg)

	for i := 0; i < 3; i++ {
		cb.RecordFailure()
	}
	if cb.State() != CircuitOpen {
		t.Errorf("expected open after 3 failures, got %v", cb.State())
	}
	if err := cb.Allow(); err == nil {
		t.Error("expected error when circuit is open")
	}
}

func TestCircuitBreaker_HalfOpenAfterTimeout(t *testing.T) {
	cfg := CircuitBreakerConfig{FailureThreshold: 1, ResetTimeout: 50 * time.Millisecond}
	cb := NewCircuitBreaker(cfg)

	cb.RecordFailure()
	if cb.State() != CircuitOpen {
		t.Fatal("expected open")
	}

	time.Sleep(60 * time.Millisecond)
	if err := cb.Allow(); err != nil {
		t.Errorf("expected allow after timeout, got %v", err)
	}
	if cb.State() != CircuitHalfOpen {
		t.Errorf("expected half-open, got %v", cb.State())
	}
}

func TestCircuitBreaker_ClosesOnSuccessInHalfOpen(t *testing.T) {
	cfg := CircuitBreakerConfig{FailureThreshold: 1, ResetTimeout: 10 * time.Millisecond, HalfOpenMaxAttempts: 1}
	cb := NewCircuitBreaker(cfg)

	cb.RecordFailure()
	time.Sleep(15 * time.Millisecond)
	_ = cb.Allow() // transitions to half-open
	cb.RecordSuccess()

	if cb.State() != CircuitClosed {
		t.Errorf("expected closed after success in half-open, got %v", cb.State())
	}
}

func TestCircuitBreaker_ReopensOnFailureInHalfOpen(t *testing.T) {
	cfg := CircuitBreakerConfig{FailureThreshold: 1, ResetTimeout: 10 * time.Millisecond}
	cb := NewCircuitBreaker(cfg)

	cb.RecordFailure()
	time.Sleep(15 * time.Millisecond)
	_ = cb.Allow() // transitions to half-open
	cb.RecordFailure()

	if cb.State() != CircuitOpen {
		t.Errorf("expected open after failure in half-open, got %v", cb.State())
	}
}

func TestCircuitBreaker_Reset(t *testing.T) {
	cfg := CircuitBreakerConfig{FailureThreshold: 1, ResetTimeout: time.Minute}
	cb := NewCircuitBreaker(cfg)
	cb.RecordFailure()
	cb.Reset()
	if cb.State() != CircuitClosed {
		t.Errorf("expected closed after reset, got %v", cb.State())
	}
}

func TestCircuitBreaker_SuccessResetsFailureCount(t *testing.T) {
	cfg := CircuitBreakerConfig{FailureThreshold: 3, ResetTimeout: time.Minute}
	cb := NewCircuitBreaker(cfg)

	cb.RecordFailure()
	cb.RecordFailure()
	cb.RecordSuccess() // resets failures
	cb.RecordFailure()
	cb.RecordFailure()

	if cb.State() != CircuitClosed {
		t.Errorf("expected closed (success should reset count), got %v", cb.State())
	}
}
