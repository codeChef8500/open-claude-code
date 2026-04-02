package agent

import (
	"testing"
	"time"
)

func TestPool_Stats(t *testing.T) {
	cfg := DefaultPoolConfig()
	if cfg.MaxConcurrent != 4 {
		t.Errorf("expected MaxConcurrent=4, got %d", cfg.MaxConcurrent)
	}
	if cfg.MaxQueued != 16 {
		t.Errorf("expected MaxQueued=16, got %d", cfg.MaxQueued)
	}
	if cfg.DefaultTimeout != 10*time.Minute {
		t.Errorf("expected DefaultTimeout=10m, got %v", cfg.DefaultTimeout)
	}
}

func TestPool_SubmitAfterClose(t *testing.T) {
	// Create a pool with nil coordinator (we won't actually run agents).
	p := &Pool{
		config: DefaultPoolConfig(),
		sem:    make(chan struct{}, 4),
		queue:  make(chan poolJob, 16),
	}
	p.closed = true

	_, err := p.Submit(nil, AgentConfig{AgentID: "test"})
	if err == nil {
		t.Error("expected error when submitting to closed pool")
	}
}

func TestPool_StatsReflectsCapacity(t *testing.T) {
	p := &Pool{
		config: PoolConfig{MaxConcurrent: 8, MaxQueued: 32, DefaultTimeout: 5 * time.Minute},
		sem:    make(chan struct{}, 8),
		queue:  make(chan poolJob, 32),
	}

	stats := p.Stats()
	if stats.MaxConcurrent != 8 {
		t.Errorf("expected MaxConcurrent=8, got %d", stats.MaxConcurrent)
	}
	if stats.Active != 0 {
		t.Errorf("expected Active=0, got %d", stats.Active)
	}
	if stats.Queued != 0 {
		t.Errorf("expected Queued=0, got %d", stats.Queued)
	}
}

func TestTaskManager_Lifecycle(t *testing.T) {
	tm := NewTaskManager()

	def := AgentDefinition{
		AgentID: "agent-1",
		Task:    "test task",
		WorkDir: "/tmp",
	}
	task := tm.Create(def)
	if task.Status != AgentStatusPending {
		t.Errorf("expected pending, got %s", task.Status)
	}

	if err := tm.MarkRunning("agent-1"); err != nil {
		t.Fatalf("MarkRunning: %v", err)
	}
	got, ok := tm.Get("agent-1")
	if !ok {
		t.Fatal("expected task to exist")
	}
	if got.Status != AgentStatusRunning {
		t.Errorf("expected running, got %s", got.Status)
	}
	if got.StartedAt.IsZero() {
		t.Error("expected StartedAt to be set")
	}

	if err := tm.MarkDone("agent-1", "output text"); err != nil {
		t.Fatalf("MarkDone: %v", err)
	}
	got, _ = tm.Get("agent-1")
	if got.Status != AgentStatusDone {
		t.Errorf("expected done, got %s", got.Status)
	}
	if got.Output != "output text" {
		t.Errorf("expected output, got %q", got.Output)
	}
}

func TestTaskManager_Active(t *testing.T) {
	tm := NewTaskManager()
	tm.Create(AgentDefinition{AgentID: "a1", Task: "t1", WorkDir: "/tmp"})
	tm.Create(AgentDefinition{AgentID: "a2", Task: "t2", WorkDir: "/tmp"})
	_ = tm.MarkRunning("a1")
	_ = tm.MarkDone("a2", "done")

	active := tm.Active()
	if len(active) != 1 {
		t.Errorf("expected 1 active, got %d", len(active))
	}
	if active[0].Definition.AgentID != "a1" {
		t.Errorf("expected a1, got %q", active[0].Definition.AgentID)
	}
}

func TestTaskManager_MarkFailed(t *testing.T) {
	tm := NewTaskManager()
	tm.Create(AgentDefinition{AgentID: "fail-1", Task: "t", WorkDir: "/tmp"})
	_ = tm.MarkRunning("fail-1")
	_ = tm.MarkFailed("fail-1", "something went wrong")

	got, _ := tm.Get("fail-1")
	if got.Status != AgentStatusFailed {
		t.Errorf("expected failed, got %s", got.Status)
	}
	if got.Error != "something went wrong" {
		t.Errorf("expected error msg, got %q", got.Error)
	}
}

func TestTaskManager_MarkCancelled(t *testing.T) {
	tm := NewTaskManager()
	tm.Create(AgentDefinition{AgentID: "cancel-1", Task: "t", WorkDir: "/tmp"})
	_ = tm.MarkRunning("cancel-1")
	_ = tm.MarkCancelled("cancel-1")

	got, _ := tm.Get("cancel-1")
	if got.Status != AgentStatusCancelled {
		t.Errorf("expected cancelled, got %s", got.Status)
	}
}

func TestTaskManager_Delete(t *testing.T) {
	tm := NewTaskManager()
	tm.Create(AgentDefinition{AgentID: "del-1", Task: "t", WorkDir: "/tmp"})
	tm.Delete("del-1")

	_, ok := tm.Get("del-1")
	if ok {
		t.Error("expected task to be deleted")
	}
}

func TestMessageBus_SubscribeAndSend(t *testing.T) {
	bus := NewMessageBus()

	ch, err := bus.Subscribe("agent-A", 10)
	if err != nil {
		t.Fatalf("Subscribe: %v", err)
	}

	msg := AgentMessage{FromAgentID: "agent-B", ToAgentID: "agent-A", Content: "hello"}
	if err := bus.Send(msg); err != nil {
		t.Fatalf("Send: %v", err)
	}

	select {
	case got := <-ch:
		if got.Content != "hello" {
			t.Errorf("expected 'hello', got %v", got.Content)
		}
	default:
		t.Error("expected message in channel")
	}
}

func TestMessageBus_SendToUnsubscribed(t *testing.T) {
	bus := NewMessageBus()
	err := bus.Send(AgentMessage{ToAgentID: "nobody"})
	if err == nil {
		t.Error("expected error sending to unsubscribed agent")
	}
}

func TestMessageBus_Broadcast(t *testing.T) {
	bus := NewMessageBus()
	chA, _ := bus.Subscribe("A", 10)
	chB, _ := bus.Subscribe("B", 10)
	_, _ = bus.Subscribe("C", 10) // sender

	bus.Broadcast("C", "ping")

	select {
	case got := <-chA:
		if got.Content != "ping" {
			t.Errorf("A: expected 'ping', got %v", got.Content)
		}
	default:
		t.Error("A: expected broadcast message")
	}

	select {
	case got := <-chB:
		if got.Content != "ping" {
			t.Errorf("B: expected 'ping', got %v", got.Content)
		}
	default:
		t.Error("B: expected broadcast message")
	}
}

func TestMessageBus_Unsubscribe(t *testing.T) {
	bus := NewMessageBus()
	_, _ = bus.Subscribe("X", 10)
	bus.Unsubscribe("X")

	err := bus.Send(AgentMessage{ToAgentID: "X"})
	if err == nil {
		t.Error("expected error after unsubscribe")
	}
}
