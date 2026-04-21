//go:build gcp

package logger

import (
	"context"
	"fmt"
	"io"
	"os"

	"cloud.google.com/go/logging"
	"google.golang.org/api/option"
)

// CloudLoggerConfig contains configuration for Google Cloud Logging
type CloudLoggerConfig struct {
	ProjectID      string
	LogName        string
	ServiceName    string
	ServiceVersion string
}

// NewCloudLogger creates a logger that writes to Google Cloud Logging
// This can be used in production to send logs to GCP
func NewCloudLogger(ctx context.Context, config CloudLoggerConfig, level LogLevel) (Logger, error) {
	// Check if running in Google Cloud environment
	projectID := config.ProjectID
	if projectID == "" {
		projectID = os.Getenv("GOOGLE_CLOUD_PROJECT")
	}
	if projectID == "" {
		projectID = os.Getenv("GCP_PROJECT")
	}

	if projectID == "" {
		return nil, fmt.Errorf("no Google Cloud project ID found")
	}

	// Create logging client
	var client *logging.Client
	var err error

	// Support for local development with emulator
	if emulatorHost := os.Getenv("LOGGING_EMULATOR_HOST"); emulatorHost != "" {
		client, err = logging.NewClient(ctx, projectID, option.WithoutAuthentication())
	} else {
		client, err = logging.NewClient(ctx, projectID)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to create logging client: %w", err)
	}

	// Create logger with service labels
	logName := config.LogName
	if logName == "" {
		logName = "bincrypt"
	}

	gcpLogger := client.Logger(logName, logging.CommonLabels(map[string]string{
		"service_name":    config.ServiceName,
		"service_version": config.ServiceVersion,
	}))

	// Create a writer that forwards to Cloud Logging
	writer := &cloudLogWriter{
		logger:  gcpLogger,
		context: ctx,
	}

	// Return a structured logger that writes to Cloud Logging
	return NewLogger(writer, level, true), nil
}

// cloudLogWriter implements io.Writer for Google Cloud Logging
type cloudLogWriter struct {
	logger  *logging.Logger
	context context.Context
}

func (w *cloudLogWriter) Write(p []byte) (n int, err error) {
	// Parse the JSON log entry and convert to Cloud Logging entry
	// For now, just write as text - in production, you'd parse JSON
	// and create structured entries
	w.logger.Log(logging.Entry{
		Payload:  string(p),
		Severity: logging.Info, // You'd parse this from the JSON
	})
	return len(p), nil
}

// InitCloudLogging initializes the global logger with Google Cloud Logging
// Call this instead of Init() when running in GCP
func InitCloudLogging(ctx context.Context, config CloudLoggerConfig, level LogLevel) error {
	cloudLogger, err := NewCloudLogger(ctx, config, level)
	if err != nil {
		return err
	}

	// Replace the default logger
	defaultLogger = cloudLogger.(*StructuredLogger)

	// Also initialize audit logger
	InitAuditLogger(cloudLogger)

	return nil
}

// MultiWriter creates a logger that writes to multiple destinations
// Useful for writing to both stdout and Cloud Logging
func MultiWriter(writers ...io.Writer) io.Writer {
	return io.MultiWriter(writers...)
}
