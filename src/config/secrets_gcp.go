//go:build gcp

package config

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"
	"sync"

	secretmanager "cloud.google.com/go/secretmanager/apiv1"
	"cloud.google.com/go/secretmanager/apiv1/secretmanagerpb"
)

// GCPSecretProvider retrieves secrets from Google Secret Manager.
type GCPSecretProvider struct {
	projectID string
	client    *secretmanager.Client
	cache     map[string]string
	mu        sync.RWMutex
}

// NewGCPSecretProvider creates a provider that reads from Google Secret Manager.
func NewGCPSecretProvider(projectID string) (*GCPSecretProvider, error) {
	ctx := context.Background()
	client, err := secretmanager.NewClient(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to create secret manager client: %w", err)
	}

	return &GCPSecretProvider{
		projectID: projectID,
		client:    client,
		cache:     make(map[string]string),
	}, nil
}

func (g *GCPSecretProvider) LoadSecrets(ctx context.Context) error {
	return nil
}

func (g *GCPSecretProvider) GetSecret(ctx context.Context, key string) (string, error) {
	g.mu.RLock()
	if value, ok := g.cache[key]; ok {
		g.mu.RUnlock()
		return value, nil
	}
	g.mu.RUnlock()

	secretName := strings.ToLower(strings.ReplaceAll(key, "_", "-"))
	name := fmt.Sprintf("projects/%s/secrets/%s/versions/latest", g.projectID, secretName)

	req := &secretmanagerpb.AccessSecretVersionRequest{
		Name: name,
	}

	result, err := g.client.AccessSecretVersion(ctx, req)
	if err != nil {
		return "", fmt.Errorf("failed to access secret %s: %w", key, err)
	}

	value := string(result.Payload.Data)

	g.mu.Lock()
	g.cache[key] = value
	g.mu.Unlock()

	return value, nil
}

func (g *GCPSecretProvider) Close() error {
	return g.client.Close()
}

// initGCPProvider initializes the GCP Secret Manager provider.
func initGCPProvider(ctx context.Context) error {
	projectID := os.Getenv("GOOGLE_CLOUD_PROJECT")
	if projectID == "" {
		return fmt.Errorf("GOOGLE_CLOUD_PROJECT not set")
	}

	provider, err := NewGCPSecretProvider(projectID)
	if err != nil {
		return fmt.Errorf("failed to initialize GCP provider: %w", err)
	}
	secretProvider = provider
	log.Println("INFO: Using Google Secret Manager for secrets")
	return nil
}
