package services

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"bincrypt/src/models"
	"cloud.google.com/go/storage"
)

// RateLimitService handles rate limiting using GCS
type RateLimitService struct {
	bucket *storage.BucketHandle
	mu     sync.Mutex // Serialize updates to reduce contention
}

// NewRateLimitService creates a new rate limit service
func NewRateLimitService(bucket *storage.BucketHandle) *RateLimitService {
	return &RateLimitService{bucket: bucket}
}

// CheckRateLimit checks if a request is allowed
func (r *RateLimitService) CheckRateLimit(ctx context.Context, identifier, action string, limit int, window time.Duration) (allowed bool, remaining int, resetAt time.Time, err error) {
	// Serialize updates to reduce GCS contention
	r.mu.Lock()
	defer r.mu.Unlock()
	
	key, windowStart, expiresAt := r.rateLimitKey(action, identifier, window)
	obj := r.bucket.Object(key)
	
	// Try to read existing entry
	var entry models.RateLimitEntry
	var generation int64
	
	attrs, err := obj.Attrs(ctx)
	if err == nil {
		generation = attrs.Generation
		reader, err := obj.NewReader(ctx)
		if err != nil {
			// Allow on read error
			return true, limit, expiresAt, nil
		}
		defer reader.Close()
		
		if err := json.NewDecoder(reader).Decode(&entry); err != nil {
			// Corrupted entry, reset
			entry = models.RateLimitEntry{
				Count:       0,
				WindowStart: windowStart,
				ExpiresAt:   expiresAt,
			}
		}
	} else if err == storage.ErrObjectNotExist {
		// New entry
		entry = models.RateLimitEntry{
			Count:       0,
			WindowStart: windowStart,
			ExpiresAt:   expiresAt,
		}
	} else {
		// Storage error, allow request
		return true, limit, expiresAt, nil
	}
	
	// Check if window has expired
	if time.Now().After(entry.ExpiresAt) || !entry.WindowStart.Equal(windowStart) {
		entry.Count = 0
		entry.WindowStart = windowStart
		entry.ExpiresAt = expiresAt
	}
	
	// Check limit
	if entry.Count >= limit {
		return false, 0, entry.ExpiresAt, nil
	}
	
	// Increment counter
	entry.Count++
	
	// Write back with generation match for atomicity
	var writer *storage.Writer
	if generation == 0 {
		writer = obj.If(storage.Conditions{DoesNotExist: true}).NewWriter(ctx)
	} else {
		writer = obj.If(storage.Conditions{GenerationMatch: generation}).NewWriter(ctx)
	}
	writer.ContentType = "application/json"
	
	if err := json.NewEncoder(writer).Encode(&entry); err != nil {
		writer.Close()
		// Allow on write error
		return true, limit - 1, expiresAt, nil
	}
	
	if err := writer.Close(); err != nil {
		// Allow on close error
		return true, limit - 1, expiresAt, nil
	}
	
	remaining = limit - entry.Count
	return true, remaining, entry.ExpiresAt, nil
}

// rateLimitKey generates the GCS object key for rate limit entry
func (r *RateLimitService) rateLimitKey(action, identifier string, window time.Duration) (string, time.Time, time.Time) {
	now := time.Now().UTC()
	windowStart := now.Truncate(window)
	expiresAt := windowStart.Add(window)
	
	// Sanitize identifier for use in path
	safeIdentifier := strings.ReplaceAll(identifier, "/", "_")
	safeIdentifier = strings.ReplaceAll(safeIdentifier, ":", "_")
	
	key := fmt.Sprintf("ratelimit/%s/%s/%s.json",
		action,
		safeIdentifier,
		windowStart.Format(time.RFC3339))
	
	return key, windowStart, expiresAt
}