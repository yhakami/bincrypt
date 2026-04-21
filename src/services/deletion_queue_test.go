package services

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// mockStorage implements the DeletePaste interface for testing
type mockStorage struct {
	deletions   []string
	mu          sync.Mutex
	deleteDelay time.Duration
	failOnID    string
	deleteCount atomic.Int32
	shouldBlock atomic.Bool
	blockCh     chan struct{}
}

func newMockStorage() *mockStorage {
	return &mockStorage{
		deletions: make([]string, 0),
		blockCh:   make(chan struct{}),
	}
}

func (m *mockStorage) DeletePaste(ctx context.Context, id string) error {
	// Increment counter
	m.deleteCount.Add(1)

	// Check if we should block (for shutdown testing)
	if m.shouldBlock.Load() {
		select {
		case <-m.blockCh:
		case <-ctx.Done():
			return ctx.Err()
		}
	}

	// Simulate processing time
	if m.deleteDelay > 0 {
		time.Sleep(m.deleteDelay)
	}

	// Simulate failure for specific ID
	if m.failOnID != "" && id == m.failOnID {
		return errors.New("simulated deletion failure")
	}

	// Record deletion
	m.mu.Lock()
	m.deletions = append(m.deletions, id)
	m.mu.Unlock()

	return nil
}

func (m *mockStorage) getDeletions() []string {
	m.mu.Lock()
	defer m.mu.Unlock()
	result := make([]string, len(m.deletions))
	copy(result, m.deletions)
	return result
}

func (m *mockStorage) getDeleteCount() int {
	return int(m.deleteCount.Load())
}

func (m *mockStorage) unblock() {
	close(m.blockCh)
}

// TestDeletionQueueBasic tests basic queuing and deletion
func TestDeletionQueueBasic(t *testing.T) {
	storage := newMockStorage()
	queue := NewDeletionQueue(storage, 2)
	defer queue.Shutdown(5 * time.Second)

	// Queue some deletions
	ids := []string{"paste1", "paste2", "paste3"}
	for _, id := range ids {
		if err := queue.QueueDeletion(id); err != nil {
			t.Fatalf("Failed to queue deletion: %v", err)
		}
	}

	// Wait for processing
	time.Sleep(100 * time.Millisecond)

	// Verify all deletions were processed
	deletions := storage.getDeletions()
	if len(deletions) != len(ids) {
		t.Errorf("Expected %d deletions, got %d", len(ids), len(deletions))
	}

	// Verify all IDs were deleted (order doesn't matter)
	deletionMap := make(map[string]bool)
	for _, id := range deletions {
		deletionMap[id] = true
	}
	for _, id := range ids {
		if !deletionMap[id] {
			t.Errorf("ID %s was not deleted", id)
		}
	}
}

// TestDeletionQueueBackpressure tests queue full behavior
func TestDeletionQueueBackpressure(t *testing.T) {
	storage := newMockStorage()
	storage.shouldBlock.Store(true) // Block all deletions

	queue := NewDeletionQueue(storage, 1)
	defer func() {
		storage.unblock()
		queue.Shutdown(5 * time.Second)
	}()

	// Fill the queue (1000 capacity)
	successCount := 0
	for i := 0; i < 1010; i++ {
		if err := queue.QueueDeletion("paste"); err == nil {
			successCount++
		}
	}

	// Should have accepted 1000 (buffer size) + 1 (worker processing)
	// But since worker is blocked, we'll get exactly 1000 in buffer
	if successCount < 1000 || successCount > 1001 {
		t.Errorf("Expected ~1000 successful queues, got %d", successCount)
	}

	// Additional attempts should fail immediately
	if err := queue.QueueDeletion("paste"); err == nil {
		t.Error("Expected queue full error, got nil")
	}
}

// TestDeletionQueueConcurrency tests concurrent queuing
func TestDeletionQueueConcurrency(t *testing.T) {
	storage := newMockStorage()
	storage.deleteDelay = 1 * time.Millisecond

	queue := NewDeletionQueue(storage, 10)
	defer queue.Shutdown(5 * time.Second)

	// Queue deletions concurrently
	var wg sync.WaitGroup
	numGoroutines := 50
	deletionsPerGoroutine := 20

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < deletionsPerGoroutine; j++ {
				_ = queue.QueueDeletion("paste")
			}
		}(i)
	}

	wg.Wait()
	time.Sleep(500 * time.Millisecond)

	// Verify all deletions were processed
	expected := numGoroutines * deletionsPerGoroutine
	actual := storage.getDeleteCount()
	if actual != expected {
		t.Errorf("Expected %d deletions, got %d", expected, actual)
	}
}

// TestDeletionQueueErrorHandling tests that errors don't crash workers
func TestDeletionQueueErrorHandling(t *testing.T) {
	storage := newMockStorage()
	storage.failOnID = "fail"

	queue := NewDeletionQueue(storage, 2)
	defer queue.Shutdown(5 * time.Second)

	// Queue both successful and failing deletions
	ids := []string{"paste1", "fail", "paste2", "fail", "paste3"}
	for _, id := range ids {
		if err := queue.QueueDeletion(id); err != nil {
			t.Fatalf("Failed to queue deletion: %v", err)
		}
	}

	// Wait for processing
	time.Sleep(100 * time.Millisecond)

	// Verify successful deletions were processed
	deletions := storage.getDeletions()
	if len(deletions) != 3 {
		t.Errorf("Expected 3 successful deletions, got %d", len(deletions))
	}

	// Verify total attempts includes failures
	if storage.getDeleteCount() != 5 {
		t.Errorf("Expected 5 total deletion attempts, got %d", storage.getDeleteCount())
	}
}

// TestDeletionQueueShutdown tests graceful shutdown
func TestDeletionQueueShutdown(t *testing.T) {
	storage := newMockStorage()
	storage.deleteDelay = 50 * time.Millisecond

	queue := NewDeletionQueue(storage, 2)

	// Queue some deletions
	for i := 0; i < 5; i++ {
		if err := queue.QueueDeletion("paste"); err != nil {
			t.Fatalf("Failed to queue deletion: %v", err)
		}
	}

	// Shutdown should wait for workers to finish
	start := time.Now()
	if err := queue.Shutdown(5 * time.Second); err != nil {
		t.Fatalf("Shutdown failed: %v", err)
	}
	elapsed := time.Since(start)

	// Should have waited for some deletions to complete
	if elapsed < 50*time.Millisecond {
		t.Error("Shutdown returned too quickly, workers may not have finished")
	}

	// Verify deletions were processed
	if storage.getDeleteCount() == 0 {
		t.Error("No deletions were processed before shutdown")
	}
}

// TestDeletionQueueShutdownTimeout tests shutdown timeout behavior
func TestDeletionQueueShutdownTimeout(t *testing.T) {
	storage := newMockStorage()
	storage.shouldBlock.Store(true) // Block all deletions

	queue := NewDeletionQueue(storage, 2)

	// Queue a deletion that will block
	if err := queue.QueueDeletion("paste"); err != nil {
		t.Fatalf("Failed to queue deletion: %v", err)
	}

	// Give worker time to pick up the task
	time.Sleep(10 * time.Millisecond)

	// Shutdown with short timeout should fail
	start := time.Now()
	err := queue.Shutdown(100 * time.Millisecond)
	elapsed := time.Since(start)

	if err == nil {
		t.Error("Expected shutdown timeout error, got nil")
	}

	// Should have waited approximately the timeout duration
	if elapsed < 90*time.Millisecond || elapsed > 200*time.Millisecond {
		t.Errorf("Expected ~100ms timeout, got %v", elapsed)
	}

	// Cleanup
	storage.unblock()
	time.Sleep(50 * time.Millisecond)
}

// TestDeletionQueueWorkerCount verifies correct number of workers
func TestDeletionQueueWorkerCount(t *testing.T) {
	testCases := []struct {
		name     string
		workers  int
		expected int
	}{
		{"default", 0, 10},
		{"single", 1, 1},
		{"multiple", 20, 20},
		{"negative", -5, 10},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			storage := newMockStorage()
			queue := NewDeletionQueue(storage, tc.workers)
			defer queue.Shutdown(1 * time.Second)

			if queue.workers != tc.expected {
				t.Errorf("Expected %d workers, got %d", tc.expected, queue.workers)
			}
		})
	}
}
