//go:build gcp

package tests

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/yhakami/bincrypt/src/services"
)

func skipIfNoFirestore(t *testing.T) {
	if os.Getenv("FIRESTORE_EMULATOR_HOST") == "" && os.Getenv("TEST_FIRESTORE") == "" {
		t.Skip("Skipping Firestore tests: FIRESTORE_EMULATOR_HOST or TEST_FIRESTORE not set")
	}
}

func getTestProjectID() string {
	if id := os.Getenv("FIREBASE_PROJECT_ID"); id != "" {
		return id
	}
	return "test-project"
}

func TestFirestoreRateLimiter_Basic(t *testing.T) {
	skipIfNoFirestore(t)

	rl, err := services.NewFirestoreRateLimitService(getTestProjectID(), "test_ratelimits")
	if err != nil {
		t.Fatalf("Failed to create Firestore rate limiter: %v", err)
	}
	defer rl.Stop()

	ctx := context.Background()
	key := "test_key_" + time.Now().Format("20060102150405")
	limit := 5
	window := 2 * time.Second

	// Should allow first requests up to limit
	for i := 0; i < limit; i++ {
		allowed, remaining, _, _, err := rl.CheckRateLimit(ctx, key, limit, window)
		if err != nil {
			t.Fatalf("Request %d failed: %v", i+1, err)
		}
		if !allowed {
			t.Fatalf("Request %d should be allowed", i+1)
		}
		if remaining != limit-i-1 {
			t.Errorf("Request %d: expected remaining=%d, got=%d", i+1, limit-i-1, remaining)
		}
	}

	// Should deny request over limit
	allowed, remaining, resetAt, retryAfter, err := rl.CheckRateLimit(ctx, key, limit, window)
	if err != nil {
		t.Fatalf("Over-limit request failed: %v", err)
	}
	if allowed {
		t.Error("Request over limit should be denied")
	}
	if remaining != 0 {
		t.Errorf("Expected remaining=0, got=%d", remaining)
	}
	if retryAfter <= 0 {
		t.Error("Expected positive retry after duration")
	}
	if resetAt.IsZero() {
		t.Error("Expected non-zero reset time")
	}
}

func TestFirestoreRateLimiter_SlidingWindow(t *testing.T) {
	skipIfNoFirestore(t)

	rl, err := services.NewFirestoreRateLimitService(getTestProjectID(), "test_ratelimits")
	if err != nil {
		t.Fatalf("Failed to create Firestore rate limiter: %v", err)
	}
	defer rl.Stop()

	ctx := context.Background()
	key := "test_sliding_" + time.Now().Format("20060102150405")
	limit := 3
	window := 1 * time.Second

	// Use up the limit
	for i := 0; i < limit; i++ {
		allowed, _, _, _, _ := rl.CheckRateLimit(ctx, key, limit, window)
		if !allowed {
			t.Fatalf("Initial request %d should be allowed", i+1)
		}
	}

	// Should be blocked immediately
	allowed, _, _, _, _ := rl.CheckRateLimit(ctx, key, limit, window)
	if allowed {
		t.Error("Should be rate limited")
	}

	// Wait for half window
	time.Sleep(window / 2)

	// Still should be blocked (sliding window)
	allowed, _, _, _, _ = rl.CheckRateLimit(ctx, key, limit, window)
	if allowed {
		t.Error("Should still be rate limited after half window")
	}

	// Wait for full window from first request
	time.Sleep(window/2 + 200*time.Millisecond)

	// Should allow one request now
	allowed, _, _, _, _ = rl.CheckRateLimit(ctx, key, limit, window)
	if !allowed {
		t.Error("Should allow request after window expires for oldest entry")
	}
}

func TestFirestoreRateLimiter_Concurrent(t *testing.T) {
	skipIfNoFirestore(t)

	rl, err := services.NewFirestoreRateLimitService(getTestProjectID(), "test_ratelimits")
	if err != nil {
		t.Fatalf("Failed to create Firestore rate limiter: %v", err)
	}
	defer rl.Stop()

	ctx := context.Background()
	key := "test_concurrent_" + time.Now().Format("20060102150405")
	limit := 10
	window := 2 * time.Second
	attempts := limit * 2

	type result struct {
		allowed bool
		err     error
	}

	results := make(chan result, attempts)

	// Launch concurrent requests
	for i := 0; i < attempts; i++ {
		go func() {
			allowed, _, _, _, err := rl.CheckRateLimit(ctx, key, limit, window)
			results <- result{allowed, err}
		}()
	}

	// Collect results
	allowedCount := 0
	for i := 0; i < attempts; i++ {
		r := <-results
		if r.err != nil {
			t.Errorf("Request failed: %v", r.err)
		}
		if r.allowed {
			allowedCount++
		}
	}

	// Should allow exactly the limit (or close due to Firestore transaction timing)
	if allowedCount < limit-1 || allowedCount > limit+1 {
		t.Errorf("Expected around %d allowed requests, got %d", limit, allowedCount)
	}
}

func TestFirestoreRateLimiter_InvalidParams(t *testing.T) {
	skipIfNoFirestore(t)

	rl, err := services.NewFirestoreRateLimitService(getTestProjectID(), "test_ratelimits")
	if err != nil {
		t.Fatalf("Failed to create Firestore rate limiter: %v", err)
	}
	defer rl.Stop()

	ctx := context.Background()

	// Test zero limit
	_, _, _, _, err = rl.CheckRateLimit(ctx, "key", 0, time.Second)
	if err == nil {
		t.Error("Should error on zero limit")
	}

	// Test zero window
	_, _, _, _, err = rl.CheckRateLimit(ctx, "key", 10, 0)
	if err == nil {
		t.Error("Should error on zero window")
	}

	// Test negative values
	_, _, _, _, err = rl.CheckRateLimit(ctx, "key", -1, time.Second)
	if err == nil {
		t.Error("Should error on negative limit")
	}
}
