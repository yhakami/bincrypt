package tests

import (
	"context"
	"os"
	"testing"

	"github.com/yhakami/bincrypt/src/config"
)

func TestEnvSecretProvider(t *testing.T) {
	// Set a test environment variable
	testKey := "TEST_SECRET_KEY"
	testValue := "test-secret-value"
	os.Setenv(testKey, testValue)
	defer os.Unsetenv(testKey)

	provider := config.NewEnvSecretProvider("")
	ctx := context.Background()

	// Test retrieving the secret
	value, err := provider.GetSecret(ctx, testKey)
	if err != nil {
		t.Errorf("Failed to get secret: %v", err)
	}
	if value != testValue {
		t.Errorf("Expected %s, got %s", testValue, value)
	}

	// Test missing secret
	_, err = provider.GetSecret(ctx, "NONEXISTENT_KEY")
	if err == nil {
		t.Error("Expected error for nonexistent key")
	}
}

// TestLoadEnvFile was removed because it tests the private loadEnvFile function
// which is not exported from the config package. The functionality is tested
// indirectly through the EnvSecretProvider.LoadSecrets() method.

func TestGetSecretOrDefault(t *testing.T) {
	ctx := context.Background()

	// Test with existing key
	testKey := "TEST_DEFAULT_KEY"
	testValue := "actual-value"
	os.Setenv(testKey, testValue)
	defer os.Unsetenv(testKey)

	// Initialize the config system to use environment variables
	// This replaces the direct access to unexported variables
	if err := config.Initialize(ctx); err != nil {
		t.Fatalf("Failed to initialize config: %v", err)
	}

	result := config.GetSecretOrDefault(ctx, testKey, "default-value")
	if result != testValue {
		t.Errorf("Expected %s, got %s", testValue, result)
	}

	// Test with missing key
	result = config.GetSecretOrDefault(ctx, "MISSING_KEY", "default-value")
	if result != "default-value" {
		t.Errorf("Expected default-value, got %s", result)
	}
}
