//go:build !gcp

package services

import (
	"fmt"
)

// NewFirestoreRateLimitService returns an error when GCP support is not compiled in.
func NewFirestoreRateLimitService(projectID string, collection string) (RateLimiter, error) {
	return nil, fmt.Errorf("Firestore rate limiter not available (build with -tags gcp)")
}
