package agent

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"
)

// PoolConfig configures the agent pool.
type PoolConfig struct {
	// MaxConcurrent is the maximum number of agents running simultaneously.
	MaxConcurrent int
	// MaxQueued is the maximum number of agents waiting in the queue.
	MaxQueued int
	// DefaultTimeout is the default timeout for each agent run.
	DefaultTimeout time.Duration
}

// DefaultPoolConfig returns sensible defaults.
func DefaultPoolConfig() PoolConfig {
	return PoolConfig{
		MaxConcurrent:  4,
		MaxQueued:      16,
		DefaultTimeout: 10 * time.Minute,
	}
}

// Pool manages a bounded pool of concurrent sub-agents with a work queue.
type Pool struct {
	mu     sync.Mutex
	config PoolConfig
	coord  *Coordinator
	tasks  *TaskManager
	bus    *MessageBus
	sem    chan struct{}
	queue  chan poolJob
	wg     sync.WaitGroup
	closed bool
}

type poolJob struct {
	ctx context.Context
	cfg AgentConfig
	ch  chan AgentResult
}

// NewPool creates an agent pool backed by the given coordinator.
func NewPool(coord *Coordinator, tasks *TaskManager, bus *MessageBus, cfg PoolConfig) *Pool {
	if cfg.MaxConcurrent <= 0 {
		cfg.MaxConcurrent = 4
	}
	if cfg.MaxQueued <= 0 {
		cfg.MaxQueued = 16
	}
	if cfg.DefaultTimeout <= 0 {
		cfg.DefaultTimeout = 10 * time.Minute
	}

	p := &Pool{
		config: cfg,
		coord:  coord,
		tasks:  tasks,
		bus:    bus,
		sem:    make(chan struct{}, cfg.MaxConcurrent),
		queue:  make(chan poolJob, cfg.MaxQueued),
	}

	// Start dispatcher.
	p.wg.Add(1)
	go p.dispatch()

	return p
}

// Submit queues an agent for execution. Returns a channel that will receive the result.
func (p *Pool) Submit(ctx context.Context, cfg AgentConfig) (<-chan AgentResult, error) {
	p.mu.Lock()
	if p.closed {
		p.mu.Unlock()
		return nil, fmt.Errorf("pool is closed")
	}
	p.mu.Unlock()

	ch := make(chan AgentResult, 1)
	select {
	case p.queue <- poolJob{ctx: ctx, cfg: cfg, ch: ch}:
		return ch, nil
	default:
		return nil, fmt.Errorf("agent pool queue is full (%d/%d)", len(p.queue), p.config.MaxQueued)
	}
}

// Close shuts down the pool, waiting for all in-flight agents.
func (p *Pool) Close() {
	p.mu.Lock()
	p.closed = true
	p.mu.Unlock()
	close(p.queue)
	p.wg.Wait()
}

// Stats returns current pool utilization.
func (p *Pool) Stats() PoolStats {
	return PoolStats{
		MaxConcurrent: p.config.MaxConcurrent,
		Active:        len(p.sem),
		Queued:        len(p.queue),
	}
}

// PoolStats holds pool utilization info.
type PoolStats struct {
	MaxConcurrent int `json:"max_concurrent"`
	Active        int `json:"active"`
	Queued        int `json:"queued"`
}

func (p *Pool) dispatch() {
	defer p.wg.Done()
	for job := range p.queue {
		// Acquire semaphore slot.
		p.sem <- struct{}{}
		p.wg.Add(1)
		go func(j poolJob) {
			defer func() {
				<-p.sem
				p.wg.Done()
			}()
			p.runJob(j)
		}(job)
	}
}

func (p *Pool) runJob(j poolJob) {
	timeout := p.config.DefaultTimeout
	ctx, cancel := context.WithTimeout(j.ctx, timeout)
	defer cancel()

	// Register task.
	if p.tasks != nil {
		p.tasks.Create(AgentDefinition{
			AgentID:  j.cfg.AgentID,
			Task:     j.cfg.Task,
			WorkDir:  j.cfg.WorkDir,
			MaxTurns: j.cfg.MaxTurns,
			ParentID: j.cfg.ParentID,
		})
		_ = p.tasks.MarkRunning(j.cfg.AgentID)
	}

	agentID, err := p.coord.SpawnAgent(ctx, j.cfg)
	if err != nil {
		result := AgentResult{AgentID: j.cfg.AgentID, Error: err}
		if p.tasks != nil {
			_ = p.tasks.MarkFailed(j.cfg.AgentID, err.Error())
		}
		j.ch <- result
		close(j.ch)
		return
	}

	slog.Info("agent pool: agent started", slog.String("agent_id", agentID))

	result, err := p.coord.WaitAgent(ctx, agentID)
	if err != nil {
		result = AgentResult{AgentID: agentID, Error: err}
		if p.tasks != nil {
			_ = p.tasks.MarkFailed(agentID, err.Error())
		}
	} else if result.Error != nil {
		if p.tasks != nil {
			_ = p.tasks.MarkFailed(agentID, result.Error.Error())
		}
	} else {
		if p.tasks != nil {
			_ = p.tasks.MarkDone(agentID, result.Output)
		}
	}

	j.ch <- result
	close(j.ch)
}
