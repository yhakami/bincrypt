package models

import "time"

// RateLimitEntry represents a rate limit counter
type RateLimitEntry struct {
	Count       int       `json:"count"`
	WindowStart time.Time `json:"window_start"`
	ExpiresAt   time.Time `json:"expires_at"`
}
