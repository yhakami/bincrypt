package services

import (
	"context"
	"sync"
	"time"
)

// RateLimiter defines the interface for rate limiting implementations
// This allows easy swapping between in-memory and Redis implementations
type RateLimiter interface {
	// CheckRateLimit checks if a request is allowed and returns:
	// - allowed: whether the request should be allowed
	// - remaining: number of requests remaining in current window
	// - resetAt: when the current window resets
	// - retryAfter: duration until next request is allowed (only set when rate limited)
	CheckRateLimit(ctx context.Context, key string, limit int, window time.Duration) (allowed bool, remaining int, resetAt time.Time, retryAfter time.Duration, err error)

	// Stop gracefully shuts down the rate limiter
	Stop()
}

// RateLimitService handles rate limiting using in-memory storage
// This implementation uses a simple token bucket algorithm
type RateLimitService struct {
	mu      sync.RWMutex
	buckets map[string]*rateBucket
	// Cleanup old entries periodically
	cleanupInterval time.Duration
	stopCleanup     chan bool
	// Track cleanup goroutine
	cleanupDone chan bool
}

type rateBucket struct {
	// Token bucket implementation
	tokens       int       // Current number of tokens
	lastRefill   time.Time // Last time tokens were refilled
	lastAccessed time.Time // Last time bucket was accessed (for cleanup)
}

// NewRateLimitService creates a new in-memory rate limit service
func NewRateLimitService() RateLimiter {
	r := &RateLimitService{
		buckets:         make(map[string]*rateBucket),
		cleanupInterval: 5 * time.Minute, // Less frequent cleanup needed for token bucket
		stopCleanup:     make(chan bool, 1),
		cleanupDone:     make(chan bool, 1),
	}

	// Start cleanup goroutine
	go r.cleanupRoutine()

	return r
}

// Stop stops the cleanup routine
func (r *RateLimitService) Stop() {
	select {
	case r.stopCleanup <- true:
		// Wait for cleanup to finish
		<-r.cleanupDone
	default:
		// Already stopped
	}
}

// CheckRateLimit implements token bucket rate limiting
func (r *RateLimitService) CheckRateLimit(ctx context.Context, key string, limit int, window time.Duration) (allowed bool, remaining int, resetAt time.Time, retryAfter time.Duration, err error) {
	now := time.Now()

	// Calculate refill rate (tokens per second)
	refillRate := float64(limit) / window.Seconds()

	r.mu.Lock()
	bucket, exists := r.buckets[key]
	if !exists {
		// Create new bucket with full tokens
		bucket = &rateBucket{
			tokens:       limit,
			lastRefill:   now,
			lastAccessed: now,
		}
		r.buckets[key] = bucket
	} else {
		// Refill tokens based on time elapsed
		elapsed := now.Sub(bucket.lastRefill).Seconds()
		tokensToAdd := int(elapsed * refillRate)

		if tokensToAdd > 0 {
			bucket.tokens += tokensToAdd
			if bucket.tokens > limit {
				bucket.tokens = limit
			}
			bucket.lastRefill = now
		}
		bucket.lastAccessed = now
	}

	// Check if request is allowed
	if bucket.tokens > 0 {
		bucket.tokens--
		remaining = bucket.tokens
		allowed = true
		resetAt = now.Add(window)
	} else {
		remaining = 0
		allowed = false
		// Calculate when next token will be available
		timeToNextToken := 1.0 / refillRate
		retryAfter = time.Duration(timeToNextToken * float64(time.Second))
		resetAt = now.Add(retryAfter)
	}

	r.mu.Unlock()

	return allowed, remaining, resetAt, retryAfter, nil
}

// cleanupRoutine removes expired entries periodically
func (r *RateLimitService) cleanupRoutine() {
	defer func() {
		r.cleanupDone <- true
	}()

	ticker := time.NewTicker(r.cleanupInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			r.cleanup()
		case <-r.stopCleanup:
			return
		}
	}
}

// cleanup removes buckets that haven't been used recently
func (r *RateLimitService) cleanup() {
	now := time.Now()
	expiredKeys := make([]string, 0)

	r.mu.Lock()
	for key, bucket := range r.buckets {
		// Remove buckets that haven't been accessed in 2 hours
		if now.Sub(bucket.lastAccessed) > 2*time.Hour {
			expiredKeys = append(expiredKeys, key)
		}
	}

	// Remove expired buckets
	for _, key := range expiredKeys {
		delete(r.buckets, key)
	}
	r.mu.Unlock()
}
