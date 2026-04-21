// Package services provides deletion queue with bounded worker pool.
// This prevents unbounded goroutine creation during high traffic or attacks.
package services

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/yhakami/bincrypt/src/logger"
)

// DeletionQueue manages a bounded worker pool for asynchronous paste deletion.
// It prevents resource exhaustion from unbounded goroutine creation during attacks.
type DeletionQueue struct {
	storage interface {
		DeletePaste(ctx context.Context, id string) error
	}
	tasks   chan string // Buffered channel for paste IDs to delete
	workers int
	ctx     context.Context
	cancel  context.CancelFunc
	wg      sync.WaitGroup

	mu     sync.Mutex
	closed bool
}

// NewDeletionQueue creates a deletion queue with specified number of workers.
// The queue has a fixed buffer of 1000 pending deletions to provide backpressure.
//
// Parameters:
//   - storage: The storage service to perform deletions (StorageService or StorageBackend)
//   - workers: Number of concurrent worker goroutines (recommended: 10-50)
//
// Returns a running DeletionQueue that must be stopped via Shutdown().
func NewDeletionQueue(storage interface {
	DeletePaste(ctx context.Context, id string) error
}, workers int) *DeletionQueue {
	if workers <= 0 {
		workers = 10 // Default to 10 workers
	}

	ctx, cancel := context.WithCancel(context.Background())
	q := &DeletionQueue{
		storage: storage,
		tasks:   make(chan string, 1000), // Buffer 1000 pending deletions
		workers: workers,
		ctx:     ctx,
		cancel:  cancel,
	}

	// Start worker goroutines
	for i := 0; i < workers; i++ {
		q.wg.Add(1)
		go q.worker(i)
	}

	logger.Default().Info("deletion_queue_started", logger.Fields{
		"workers":     workers,
		"buffer_size": 1000,
	})

	return q
}

// worker processes deletion tasks from the queue.
// It runs until the context is cancelled.
func (q *DeletionQueue) worker(id int) {
	defer q.wg.Done()

	log := logger.Default()
	log.Debug("deletion_worker_started", logger.Fields{"worker_id": id})

	for pasteID := range q.tasks {
		// Process deletion with timeout.
		// Use q.ctx as the parent so Shutdown(timeout) can cancel in-flight work on timeout.
		ctx, cancel := context.WithTimeout(q.ctx, 5*time.Second)

		if err := q.storage.DeletePaste(ctx, pasteID); err != nil {
			// Log error but don't fail - paste might already be deleted.
			log.Error("deletion_failed", logger.Fields{
				"worker_id":     id,
				"paste_id_hash": logger.HashPasteID(pasteID),
				"error":         err.Error(),
			})
		} else {
			log.Debug("deletion_success", logger.Fields{
				"worker_id":     id,
				"paste_id_hash": logger.HashPasteID(pasteID),
			})
		}

		cancel()
	}

	log.Debug("deletion_worker_stopping", logger.Fields{"worker_id": id})
}

// QueueDeletion adds a paste ID to the deletion queue.
// Returns an error if the queue is full (backpressure mechanism).
//
// This is a non-blocking operation that provides backpressure when the queue
// is at capacity, preventing memory exhaustion during attacks.
func (q *DeletionQueue) QueueDeletion(pasteID string) error {
	q.mu.Lock()
	if q.closed {
		q.mu.Unlock()
		return errors.New("deletion queue shutting down")
	}
	select {
	case q.tasks <- pasteID:
		q.mu.Unlock()
		return nil
	default:
		q.mu.Unlock()
		logger.Default().Warn("deletion_queue_full", logger.Fields{
			"paste_id_hash": logger.HashPasteID(pasteID),
		})
		return errors.New("deletion queue full")
	}
}

// Shutdown gracefully stops the deletion queue.
// It signals workers to stop and waits for them to finish processing current tasks.
//
// Parameters:
//   - timeout: Maximum time to wait for workers to finish
//
// Returns an error if workers don't stop within the timeout.
func (q *DeletionQueue) Shutdown(timeout time.Duration) error {
	log := logger.Default()
	log.Info("deletion_queue_shutdown_initiated", logger.Fields{
		"timeout_seconds": timeout.Seconds(),
	})

	// Stop accepting new tasks and let workers drain the queue.
	q.mu.Lock()
	if !q.closed {
		q.closed = true
		close(q.tasks)
	}
	q.mu.Unlock()

	// Wait for workers to finish with timeout
	done := make(chan struct{})
	go func() {
		q.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		log.Info("deletion_queue_shutdown_completed", logger.Fields{})
		q.cancel()
		return nil
	case <-time.After(timeout):
		// Force-cancel in-flight deletions and allow workers to exit quickly.
		q.cancel()
		log.Error("deletion_queue_shutdown_timeout", logger.Fields{
			"timeout_seconds": timeout.Seconds(),
		})
		return errors.New("shutdown timeout exceeded")
	}
}
