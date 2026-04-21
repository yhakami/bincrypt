//go:build !gcp

package config

import (
	"context"
	"fmt"
)

// initGCPProvider is a stub when building without GCP support.
func initGCPProvider(ctx context.Context) error {
	return fmt.Errorf("GCP Secret Manager not available (build with -tags gcp)")
}
