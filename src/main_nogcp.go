//go:build !gcp

package main

import (
	"context"
	"fmt"

	"github.com/yhakami/bincrypt/src/logger"
	"github.com/yhakami/bincrypt/src/services"
)

// initGCSBackend returns an error when GCP support is not compiled in.
func initGCSBackend(ctx context.Context, log logger.Logger) (*services.StorageService, error) {
	return nil, fmt.Errorf("GCS backend not available (build with -tags gcp)")
}

// closeGCSClient is a no-op when GCP support is not compiled in.
func closeGCSClient() {}
