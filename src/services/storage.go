//go:build gcp

// Package services provides storage service compatibility layer.
// This file maintains backward compatibility with existing code while delegating to backend implementations.
package services

import (
	"cloud.google.com/go/storage"
)

// NewStorageService creates a StorageService with GCS backend for backward compatibility.
// New code should use NewStorageServiceWithBackend with explicit backend choice.
func NewStorageService(bucket *storage.BucketHandle) *StorageService {
	return &StorageService{backend: NewGCSBackend(bucket)}
}
