package services

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// RedisRateLimitService implements RateLimiter using Redis sorted sets
// Uses sliding window algorithm for accurate rate limiting
type RedisRateLimitService struct {
	client *redis.Client
	prefix string // key prefix for namespace isolation
}

// NewRedisRateLimitService creates a new Redis-based rate limiter
func NewRedisRateLimitService(redisURL string, keyPrefix string) (RateLimiter, error) {
	if redisURL == "" {
		return nil, fmt.Errorf("redis URL required")
	}

	opt, err := redis.ParseURL(redisURL)
	if err != nil {
		return nil, fmt.Errorf("invalid redis URL: %w", err)
	}

	// Connection pool settings for production
	opt.MaxRetries = 3
	opt.MinIdleConns = 2
	opt.MaxIdleConns = 10
	opt.ConnMaxIdleTime = 5 * time.Minute

	client := redis.NewClient(opt)

	// Verify connection with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("redis connection failed: %w", err)
	}

	if keyPrefix == "" {
		keyPrefix = "ratelimit:"
	}

	return &RedisRateLimitService{
		client: client,
		prefix: keyPrefix,
	}, nil
}

// CheckRateLimit implements sliding window rate limiting using Redis sorted sets
func (r *RedisRateLimitService) CheckRateLimit(ctx context.Context, key string, limit int, window time.Duration) (allowed bool, remaining int, resetAt time.Time, retryAfter time.Duration, err error) {
	if limit <= 0 || window <= 0 {
		return false, 0, time.Time{}, 0, fmt.Errorf("invalid limit or window")
	}

	now := time.Now()
	windowStart := now.Add(-window)
	redisKey := r.prefix + key

	// Use Lua script for atomicity and to reduce round trips
	script := redis.NewScript(`
		local key = KEYS[1]
		local now = tonumber(ARGV[1])
		local window_start = tonumber(ARGV[2])
		local limit = tonumber(ARGV[3])
		local window_ms = tonumber(ARGV[4])
		
		-- Remove old entries outside window
		redis.call('ZREMRANGEBYSCORE', key, 0, window_start)
		
		-- Count current entries
		local count = redis.call('ZCARD', key)
		
		if count < limit then
			-- Add current request
			redis.call('ZADD', key, now, now)
			redis.call('EXPIRE', key, math.ceil(window_ms / 1000))
			return {1, limit - count - 1}
		else
			-- Get oldest entry to calculate retry time
			local oldest = redis.call('ZRANGE', key, 0, 0, 'WITHSCORES')
			if #oldest > 0 then
				return {0, 0, oldest[2]}
			else
				return {0, 0, 0}
			end
		end
	`)

	result, err := script.Run(ctx, r.client, []string{redisKey},
		now.UnixNano(), windowStart.UnixNano(), limit, window.Milliseconds()).Result()

	if err != nil {
		if err == redis.Nil {
			// First request for this key
			return r.checkRateLimitFallback(ctx, redisKey, limit, window, now)
		}
		return false, 0, time.Time{}, 0, fmt.Errorf("redis error: %w", err)
	}

	vals, ok := result.([]interface{})
	if !ok || len(vals) < 2 {
		return false, 0, time.Time{}, 0, fmt.Errorf("unexpected redis response")
	}

	allowedInt, _ := vals[0].(int64)
	allowed = allowedInt == 1

	if allowed {
		if v, ok := vals[1].(int64); ok {
			remaining = int(v)
		}
		resetAt = now.Add(window)
		retryAfter = 0
	} else {
		remaining = 0
		if len(vals) > 2 && vals[2] != nil {
			if oldestNano, ok := vals[2].(int64); ok && oldestNano > 0 {
				oldestTime := time.Unix(0, oldestNano)
				resetAt = oldestTime.Add(window)
				retryAfter = time.Until(resetAt)
				if retryAfter < 0 {
					retryAfter = 0
				}
			}
		}
	}

	return allowed, remaining, resetAt, retryAfter, nil
}

// checkRateLimitFallback handles the case when Lua script fails or first request
func (r *RedisRateLimitService) checkRateLimitFallback(ctx context.Context, key string, limit int, window time.Duration, now time.Time) (bool, int, time.Time, time.Duration, error) {
	// Simple fallback using pipeline
	pipe := r.client.Pipeline()

	windowStart := now.Add(-window).UnixNano()
	pipe.ZRemRangeByScore(ctx, key, "0", fmt.Sprintf("%d", windowStart))
	pipe.ZAdd(ctx, key, redis.Z{Score: float64(now.UnixNano()), Member: now.UnixNano()})
	pipe.ZCard(ctx, key)
	pipe.Expire(ctx, key, window)

	cmds, err := pipe.Exec(ctx)
	if err != nil {
		return false, 0, time.Time{}, 0, err
	}

	count := cmds[2].(*redis.IntCmd).Val()
	if count > int64(limit) {
		// Remove the just-added entry since we're over limit
		r.client.ZRem(ctx, key, now.UnixNano())
		return false, 0, now.Add(window), window, nil
	}

	return true, limit - int(count), now.Add(window), 0, nil
}

// Stop gracefully shuts down the Redis rate limiter
func (r *RedisRateLimitService) Stop() {
	if r.client != nil {
		_ = r.client.Close()
	}
}
