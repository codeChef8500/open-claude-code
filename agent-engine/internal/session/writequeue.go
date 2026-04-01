package session

import (
	"log/slog"
	"sync"
)

// writeRequest is a single unit of work for the write queue.
type writeRequest struct {
	writeFn func()
}

// WriteQueue serialises writes to session storage through a buffered channel
// and a single background goroutine, guaranteeing FIFO ordering without
// holding a mutex during I/O.
type WriteQueue struct {
	ch   chan writeRequest
	once sync.Once
	wg   sync.WaitGroup
}

// NewWriteQueue creates and starts a WriteQueue with the given buffer size.
// If bufSize <= 0 it defaults to 256.
func NewWriteQueue(bufSize int) *WriteQueue {
	if bufSize <= 0 {
		bufSize = 256
	}
	wq := &WriteQueue{ch: make(chan writeRequest, bufSize)}
	wq.wg.Add(1)
	go wq.worker()
	return wq
}

// Enqueue adds a write function to the queue.  It never blocks as long as
// the buffer is not full; if the buffer is full it logs a warning and drops.
func (wq *WriteQueue) Enqueue(fn func()) {
	req := writeRequest{writeFn: fn}
	select {
	case wq.ch <- req:
	default:
		slog.Warn("session write queue full — dropping write")
	}
}

// Close drains the queue and waits for the worker to finish.
func (wq *WriteQueue) Close() {
	wq.once.Do(func() { close(wq.ch) })
	wq.wg.Wait()
}

func (wq *WriteQueue) worker() {
	defer wq.wg.Done()
	for req := range wq.ch {
		func() {
			defer func() {
				if r := recover(); r != nil {
					slog.Error("session write queue panic", slog.Any("panic", r))
				}
			}()
			req.writeFn()
		}()
	}
}
