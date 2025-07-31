package middleware

import (
	"log"
	"net/http"
	"strconv"
	"time"

	"bincrypt/src/services"
	"bincrypt/src/utils"
)

// RateLimiter creates rate limiting middleware
func RateLimiter(rateLimitService *services.RateLimitService, action string, limit int, window time.Duration) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			identifier := utils.GetClientIP(r)
			ctx := r.Context()
			
			allowed, remaining, resetAt, err := rateLimitService.CheckRateLimit(ctx, identifier, action, limit, window)
			if err != nil {
				requestID := ctx.Value("request_id")
				log.Printf("[%s] Rate limit check error: %v", requestID, err)
				// Allow on error
				next.ServeHTTP(w, r)
				return
			}
			
			// Set rate limit headers
			w.Header().Set("X-RateLimit-Limit", strconv.Itoa(limit))
			w.Header().Set("X-RateLimit-Remaining", strconv.Itoa(remaining))
			w.Header().Set("X-RateLimit-Reset", strconv.FormatInt(resetAt.Unix(), 10))
			
			if !allowed {
				http.Error(w, "Rate limit exceeded", http.StatusTooManyRequests)
				return
			}
			
			next.ServeHTTP(w, r)
		})
	}
}