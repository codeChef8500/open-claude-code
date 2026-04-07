package session

import (
	"sync"
	"testing"
)

func TestSessionState_Basic(t *testing.T) {
	s := NewSessionState()
	if s.Get() != StateIdle {
		t.Errorf("initial state should be idle, got %q", s.Get())
	}

	s.SetState(StateProcessing)
	if s.Get() != StateProcessing {
		t.Errorf("state should be processing, got %q", s.Get())
	}

	meta := s.GetMetadata()
	if meta.State != StateProcessing {
		t.Errorf("metadata state should be processing, got %q", meta.State)
	}
}

func TestSessionState_PendingAction(t *testing.T) {
	s := NewSessionState()
	s.SetPendingAction(&RequiresActionDetails{
		ToolUseID: "tu_123",
		ToolName:  "bash",
	})

	if s.Get() != StatePendingAction {
		t.Errorf("state should be pending_action, got %q", s.Get())
	}

	meta := s.GetMetadata()
	if meta.PendingAction == nil || meta.PendingAction.ToolName != "bash" {
		t.Error("pending action should have tool name 'bash'")
	}

	s.ClearPendingAction()
	if s.Get() != StateProcessing {
		t.Errorf("state should be processing after clear, got %q", s.Get())
	}
}

func TestSessionState_Counters(t *testing.T) {
	s := NewSessionState()
	s.RecordTurn()
	s.RecordTurn()
	s.RecordTokenUsage(100, 50)
	s.RecordTokenUsage(200, 100)
	s.RecordCost(0.01)
	s.RecordCompaction()

	meta := s.GetMetadata()
	if meta.TurnCount != 2 {
		t.Errorf("turn count = %d, want 2", meta.TurnCount)
	}
	if meta.TotalInputTokens != 300 {
		t.Errorf("input tokens = %d, want 300", meta.TotalInputTokens)
	}
	if meta.TotalOutputTokens != 150 {
		t.Errorf("output tokens = %d, want 150", meta.TotalOutputTokens)
	}
	if meta.CompactCount != 1 {
		t.Errorf("compact count = %d, want 1", meta.CompactCount)
	}
}

func TestSessionState_Listener(t *testing.T) {
	s := NewSessionState()

	var mu sync.Mutex
	var received []SessionStateKind

	s.OnChange(func(meta SessionExternalMetadata) {
		mu.Lock()
		received = append(received, meta.State)
		mu.Unlock()
	})

	s.SetState(StateProcessing)
	s.SetTitle("Test Session")
	s.SetState(StateCompleted)

	mu.Lock()
	defer mu.Unlock()
	if len(received) < 2 {
		t.Errorf("expected at least 2 listener calls, got %d", len(received))
	}
}

func TestSessionState_Title(t *testing.T) {
	s := NewSessionState()
	s.SetTitle("My Session")
	s.SetModel("claude-3-opus")

	meta := s.GetMetadata()
	if meta.Title != "My Session" {
		t.Errorf("title = %q, want 'My Session'", meta.Title)
	}
	if meta.Model != "claude-3-opus" {
		t.Errorf("model = %q, want 'claude-3-opus'", meta.Model)
	}
}
