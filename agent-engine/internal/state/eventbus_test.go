package state

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestEventBus_SubscribeAndPublish(t *testing.T) {
	bus := NewEventBus()
	var received string
	bus.Subscribe("test.event", func(e Event) {
		received = e.Type
	})
	bus.Publish("test.event", nil)
	if received != "test.event" {
		t.Errorf("expected test.event, got %q", received)
	}
}

func TestEventBus_WildcardSubscription(t *testing.T) {
	bus := NewEventBus()
	var count int
	bus.SubscribeAll(func(e Event) {
		count++
	})
	bus.Publish("a", nil)
	bus.Publish("b", nil)
	bus.Publish("c", nil)
	if count != 3 {
		t.Errorf("expected 3 events, got %d", count)
	}
}

func TestEventBus_Unsubscribe(t *testing.T) {
	bus := NewEventBus()
	var count int
	id := bus.Subscribe("x", func(e Event) {
		count++
	})
	bus.Publish("x", nil)
	bus.Unsubscribe(id)
	bus.Publish("x", nil)
	if count != 1 {
		t.Errorf("expected 1, got %d", count)
	}
}

func TestEventBus_PublishAsync(t *testing.T) {
	bus := NewEventBus()
	var called atomic.Int32
	bus.Subscribe("async", func(e Event) {
		called.Add(1)
	})
	bus.PublishAsync("async", nil)
	time.Sleep(50 * time.Millisecond)
	if called.Load() != 1 {
		t.Errorf("expected 1, got %d", called.Load())
	}
}

func TestEventBus_Concurrent(t *testing.T) {
	bus := NewEventBus()
	var total atomic.Int64
	bus.Subscribe("inc", func(e Event) {
		total.Add(1)
	})

	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			bus.Publish("inc", nil)
		}()
	}
	wg.Wait()
	if total.Load() != 100 {
		t.Errorf("expected 100, got %d", total.Load())
	}
}
