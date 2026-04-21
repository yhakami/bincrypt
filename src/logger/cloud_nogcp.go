//go:build !gcp

package logger

import (
	"context"
	"fmt"
)

// CloudLoggerConfig contains configuration for Google Cloud Logging.
type CloudLoggerConfig struct {
	ProjectID      string
	LogName        string
	ServiceName    string
	ServiceVersion string
}

// InitCloudLogging returns an error when GCP support is not compiled in.
func InitCloudLogging(ctx context.Context, config CloudLoggerConfig, level LogLevel) error {
	return fmt.Errorf("Cloud Logging not available (build with -tags gcp)")
}
