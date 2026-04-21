package services

import (
	"context"
	"fmt"

	"github.com/yhakami/bincrypt/src/config"
)

// RateLimiterType represents the type of rate limiter to use
type RateLimiterType string

const (
	RateLimiterTypeMemory    RateLimiterType = "memory"
	RateLimiterTypeRedis     RateLimiterType = "redis"
	RateLimiterTypeFirestore RateLimiterType = "firestore"
)

// NewRateLimiterFromEnv creates a rate limiter based on environment configuration
// This makes it easy to switch between implementations without code changes
func NewRateLimiterFromEnv() (RateLimiter, error) {
	ctx := context.Background()
	limiterType := config.GetSecretOrDefault(ctx, "RATE_LIMITER_TYPE", "")
	if limiterType == "" {
		limiterType = string(RateLimiterTypeMemory)
	}

	switch RateLimiterType(limiterType) {
	case RateLimiterTypeMemory:
		return NewRateLimitService(), nil

	case RateLimiterTypeRedis:
		redisURL := config.GetSecretOrDefault(ctx, "REDIS_URL", "")
		if redisURL == "" {
			return nil, fmt.Errorf("REDIS_URL required for Redis rate limiter")
		}

		keyPrefix := config.GetSecretOrDefault(ctx, "RATE_LIMIT_KEY_PREFIX", "bincrypt:ratelimit:")
		return NewRedisRateLimitService(redisURL, keyPrefix)

	case RateLimiterTypeFirestore:
		projectID := config.GetSecretOrDefault(ctx, "FIREBASE_PROJECT_ID", "")
		if projectID == "" {
			return nil, fmt.Errorf("FIREBASE_PROJECT_ID required for Firestore rate limiter")
		}

		collection := config.GetSecretOrDefault(ctx, "RATE_LIMIT_COLLECTION", "ratelimits")
		return NewFirestoreRateLimitService(projectID, collection)

	default:
		return nil, fmt.Errorf("unknown rate limiter type: %s", limiterType)
	}
}
