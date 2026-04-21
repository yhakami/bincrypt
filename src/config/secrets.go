// Package config provides secret management via environment variables.
// For Google Secret Manager support, build with -tags gcp.
package config

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// SecretProvider defines the interface for secret retrieval.
type SecretProvider interface {
	GetSecret(ctx context.Context, key string) (string, error)
	LoadSecrets(ctx context.Context) error
}

// EnvSecretProvider reads secrets from environment variables.
type EnvSecretProvider struct {
	envFile string
	loaded  bool
	mu      sync.RWMutex
}

// NewEnvSecretProvider creates a provider that reads from env vars and optionally an env file.
func NewEnvSecretProvider(envFile string) *EnvSecretProvider {
	return &EnvSecretProvider{
		envFile: envFile,
	}
}

func (e *EnvSecretProvider) LoadSecrets(ctx context.Context) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.loaded {
		return nil
	}

	if e.envFile != "" {
		if _, err := os.Stat(e.envFile); err == nil {
			if err := loadEnvFile(e.envFile); err != nil {
				return fmt.Errorf("failed to load env file: %w", err)
			}
			log.Printf("INFO: Loaded environment from %s", e.envFile)
		}
	}

	e.loaded = true
	return nil
}

func (e *EnvSecretProvider) GetSecret(ctx context.Context, key string) (string, error) {
	e.mu.RLock()
	needsLoad := !e.loaded
	e.mu.RUnlock()

	if needsLoad {
		if err := e.LoadSecrets(ctx); err != nil {
			return "", err
		}
	}

	e.mu.RLock()
	defer e.mu.RUnlock()

	value := os.Getenv(key)
	if value == "" {
		return "", fmt.Errorf("secret %s not found in environment", key)
	}

	return value, nil
}

func loadEnvFile(filename string) error {
	data, err := os.ReadFile(filename)
	if err != nil {
		return err
	}

	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}

		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])

		if len(value) >= 2 && ((value[0] == '"' && value[len(value)-1] == '"') ||
			(value[0] == '\'' && value[len(value)-1] == '\'')) {
			value = value[1 : len(value)-1]
		}

		os.Setenv(key, value)
	}

	return nil
}

var (
	secretProvider SecretProvider
	providerMu     sync.RWMutex
)

// Initialize sets up the secret provider. Uses env vars by default.
// Build with -tags gcp to enable Google Secret Manager support.
func Initialize(ctx context.Context) error {
	providerMu.Lock()
	defer providerMu.Unlock()

	if secretProvider != nil {
		return nil
	}

	// Check if GCP Secret Manager is requested
	if os.Getenv("USE_SECRET_MANAGER") == "true" {
		if err := initGCPProvider(ctx); err != nil {
			log.Printf("WARN: GCP Secret Manager unavailable: %v", err)
			// Fall through to env provider
		} else {
			return nil
		}
	}

	// Default: use environment variables
	envFile := ".env"
	if customEnvFile := os.Getenv("ENV_FILE"); customEnvFile != "" {
		envFile = customEnvFile
	}

	if _, err := os.Stat(envFile); err != nil {
		parentEnv := filepath.Join("..", envFile)
		if _, err := os.Stat(parentEnv); err == nil {
			envFile = parentEnv
		}
	}

	provider := NewEnvSecretProvider(envFile)
	if err := provider.LoadSecrets(ctx); err != nil {
		log.Printf("WARN: Failed to load env file: %v", err)
	}
	secretProvider = provider
	log.Println("INFO: Using environment variables for secrets")

	return nil
}

// GetSecret retrieves a secret by key.
func GetSecret(ctx context.Context, key string) (string, error) {
	providerMu.RLock()
	defer providerMu.RUnlock()

	if secretProvider == nil {
		return "", fmt.Errorf("secret provider not initialized")
	}

	return secretProvider.GetSecret(ctx, key)
}

// GetSecretOrDefault retrieves a secret or returns a default value.
func GetSecretOrDefault(ctx context.Context, key, defaultValue string) string {
	value, err := GetSecret(ctx, key)
	if err != nil {
		return defaultValue
	}
	return value
}

// MustGetSecret retrieves a secret or terminates the program.
func MustGetSecret(ctx context.Context, key string) string {
	value, err := GetSecret(ctx, key)
	if err != nil {
		log.Fatalf("Required secret %s not found: %v", key, err)
	}
	return value
}

// Close releases resources held by the secret provider.
func Close() error {
	providerMu.Lock()
	defer providerMu.Unlock()

	if closer, ok := secretProvider.(interface{ Close() error }); ok {
		return closer.Close()
	}
	return nil
}
