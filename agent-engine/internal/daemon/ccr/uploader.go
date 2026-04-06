package ccr

import (
	"context"
	"log/slog"
	"sync"
	"time"
)

// ─── BatchUploader ──────────────────────────────────────────────────────────
// Generic batched uploader with queue, retry, and exponential backoff.
// Aligned with claude-code-main ccrClient.ts batch upload logic.

const (
	defaultBatchSize    = 50
	defaultFlushMs      = 100
	maxUploadRetries    = 10
	retryBaseMs         = 500
	retryMaxMs          = 30_000
	retryJitterFraction = 0.1
)

// UploadFunc is called to upload a batch of items. Returns an error on failure.
type UploadFunc[T any] func(ctx context.Context, batch []T) error

// BatchUploader accumulates items and uploads them in batches with retry.
type BatchUploader[T any] struct {
	mu       sync.Mutex
	queue    []T
	upload   UploadFunc[T]
	batchSz  int
	flushDur time.Duration
	ticker   *time.Ticker
	stopCh   chan struct{}
}

// NewBatchUploader creates a new uploader.
func NewBatchUploader[T any](upload UploadFunc[T]) *BatchUploader[T] {
	return &BatchUploader[T]{
		queue:    make([]T, 0, defaultBatchSize),
		upload:   upload,
		batchSz:  defaultBatchSize,
		flushDur: time.Duration(defaultFlushMs) * time.Millisecond,
		stopCh:   make(chan struct{}),
	}
}

// Start begins the periodic flush loop.
func (u *BatchUploader[T]) Start(ctx context.Context) {
	u.ticker = time.NewTicker(u.flushDur)
	go func() {
		for {
			select {
			case <-ctx.Done():
				u.Flush(context.Background())
				return
			case <-u.stopCh:
				u.Flush(context.Background())
				return
			case <-u.ticker.C:
				u.Flush(ctx)
			}
		}
	}()
}

// Enqueue adds an item to the upload queue.
func (u *BatchUploader[T]) Enqueue(item T) {
	u.mu.Lock()
	u.queue = append(u.queue, item)
	overBatch := len(u.queue) >= u.batchSz
	u.mu.Unlock()

	if overBatch {
		go u.Flush(context.Background())
	}
}

// Flush drains the queue and uploads in batches with retry.
func (u *BatchUploader[T]) Flush(ctx context.Context) {
	u.mu.Lock()
	if len(u.queue) == 0 {
		u.mu.Unlock()
		return
	}
	batch := make([]T, len(u.queue))
	copy(batch, u.queue)
	u.queue = u.queue[:0]
	u.mu.Unlock()

	for attempt := 0; attempt <= maxUploadRetries; attempt++ {
		if ctx.Err() != nil {
			// Re-queue on context cancellation
			u.mu.Lock()
			u.queue = append(batch, u.queue...)
			u.mu.Unlock()
			return
		}

		err := u.upload(ctx, batch)
		if err == nil {
			return
		}

		slog.Debug("ccr uploader: retry",
			slog.Int("attempt", attempt+1),
			slog.Any("err", err))

		delay := retryDelay(attempt)
		select {
		case <-ctx.Done():
			return
		case <-time.After(delay):
		}
	}

	slog.Warn("ccr uploader: dropping batch after max retries",
		slog.Int("size", len(batch)))
}

// Stop stops the periodic flush loop.
func (u *BatchUploader[T]) Stop() {
	if u.ticker != nil {
		u.ticker.Stop()
	}
	select {
	case <-u.stopCh:
	default:
		close(u.stopCh)
	}
}

func retryDelay(attempt int) time.Duration {
	ms := retryBaseMs * (1 << attempt)
	if ms > retryMaxMs {
		ms = retryMaxMs
	}
	return time.Duration(ms) * time.Millisecond
}
