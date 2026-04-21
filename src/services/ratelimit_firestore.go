//go:build gcp

package services

import (
	"context"
	"fmt"
	"os"
	"time"

	"cloud.google.com/go/firestore"
	"google.golang.org/api/option"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// FirestoreRateLimitService implements RateLimiter using Firestore
// Uses documents with timestamp arrays for sliding window rate limiting
type FirestoreRateLimitService struct {
	client     *firestore.Client
	collection string
}

// NewFirestoreRateLimitService creates a new Firestore-based rate limiter
func NewFirestoreRateLimitService(projectID string, collection string) (RateLimiter, error) {
	if projectID == "" {
		return nil, fmt.Errorf("project ID required")
	}
	if collection == "" {
		collection = "ratelimits"
	}

	ctx := context.Background()

	// Check for emulator
	var opts []option.ClientOption
	if emulatorHost := getFirestoreEmulatorHost(); emulatorHost != "" {
		opts = append(opts, option.WithoutAuthentication())
	}

	client, err := firestore.NewClient(ctx, projectID, opts...)
	if err != nil {
		return nil, fmt.Errorf("firestore client creation failed: %w", err)
	}

	return &FirestoreRateLimitService{
		client:     client,
		collection: collection,
	}, nil
}

// CheckRateLimit implements sliding window rate limiting using Firestore
func (f *FirestoreRateLimitService) CheckRateLimit(ctx context.Context, key string, limit int, window time.Duration) (allowed bool, remaining int, resetAt time.Time, retryAfter time.Duration, err error) {
	if limit <= 0 || window <= 0 {
		return false, 0, time.Time{}, 0, fmt.Errorf("invalid limit or window")
	}

	now := time.Now()
	windowStart := now.Add(-window)
	docRef := f.client.Collection(f.collection).Doc(key)

	// Use transaction for atomicity
	err = f.client.RunTransaction(ctx, func(ctx context.Context, tx *firestore.Transaction) error {
		doc, err := tx.Get(docRef)

		var timestamps []time.Time
		if err != nil {
			if status.Code(err) != codes.NotFound {
				return fmt.Errorf("failed to get document: %w", err)
			}
			// Document doesn't exist, this is the first request
			timestamps = []time.Time{}
		} else {
			// Get existing timestamps
			if data, ok := doc.Data()["timestamps"].([]interface{}); ok {
				for _, ts := range data {
					if t, ok := ts.(time.Time); ok && t.After(windowStart) {
						timestamps = append(timestamps, t)
					}
				}
			}
		}

		// Filter out old timestamps (sliding window)
		var validTimestamps []time.Time
		for _, ts := range timestamps {
			if ts.After(windowStart) {
				validTimestamps = append(validTimestamps, ts)
			}
		}

		// Check if under limit
		if len(validTimestamps) >= limit {
			// Rate limit exceeded
			allowed = false
			remaining = 0
			if len(validTimestamps) > 0 {
				// Calculate when the oldest request expires
				oldest := validTimestamps[0]
				for _, ts := range validTimestamps {
					if ts.Before(oldest) {
						oldest = ts
					}
				}
				resetAt = oldest.Add(window)
				retryAfter = time.Until(resetAt)
				if retryAfter < 0 {
					retryAfter = 0
				}
			}
			return nil
		}

		// Add current request
		validTimestamps = append(validTimestamps, now)
		allowed = true
		remaining = limit - len(validTimestamps)
		resetAt = now.Add(window)

		// Update document with TTL
		return tx.Set(docRef, map[string]interface{}{
			"timestamps": validTimestamps,
			"updatedAt":  now,
			"expiresAt":  now.Add(window * 2), // Keep for 2x window for cleanup
		})
	})

	if err != nil {
		return false, 0, time.Time{}, 0, fmt.Errorf("transaction failed: %w", err)
	}

	return allowed, remaining, resetAt, retryAfter, nil
}

// Stop gracefully shuts down the Firestore rate limiter
func (f *FirestoreRateLimitService) Stop() {
	if f.client != nil {
		_ = f.client.Close()
	}
}

// getFirestoreEmulatorHost returns the Firestore emulator host if configured
func getFirestoreEmulatorHost() string {
	// Firestore emulator uses FIRESTORE_EMULATOR_HOST env var
	if host := os.Getenv("FIRESTORE_EMULATOR_HOST"); host != "" {
		return host
	}
	return ""
}
