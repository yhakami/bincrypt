//go:build gcp

package main

import (
	"context"
	"fmt"

	"cloud.google.com/go/storage"
	"github.com/yhakami/bincrypt/src/config"
	"github.com/yhakami/bincrypt/src/logger"
	"github.com/yhakami/bincrypt/src/services"
	"google.golang.org/api/option"
)

// gcsClient holds the GCS client when using GCS backend.
var gcsClient *storage.Client

// initGCSBackend initializes Google Cloud Storage backend.
func initGCSBackend(ctx context.Context, log logger.Logger) (*services.StorageService, error) {
	bucketName := config.MustGetSecret(ctx, "BUCKET_NAME")

	var err error
	if emulatorHost := config.GetSecretOrDefault(ctx, "STORAGE_EMULATOR_HOST", ""); emulatorHost != "" {
		log.Info("storage_emulator_enabled", logger.Fields{"host": emulatorHost})
		gcsClient, err = storage.NewClient(ctx, option.WithoutAuthentication())
	} else {
		log.Info("storage_production_mode", logger.Fields{})
		gcsClient, err = storage.NewClient(ctx)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to create storage client: %w", err)
	}

	bucket := gcsClient.Bucket(bucketName)
	log.Info("gcs_storage_initialized", logger.Fields{"bucket": bucketName})

	return services.NewStorageServiceWithBackend(services.NewGCSBackend(bucket)), nil
}

// closeGCSClient closes the GCS client if it was initialized.
func closeGCSClient() {
	if gcsClient != nil {
		gcsClient.Close()
	}
}
