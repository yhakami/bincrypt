package main

import (
	"context"
	"embed"
	"flag"
	"log"
	"os"

	"bincrypt/src/server"
	"cloud.google.com/go/storage"
	"google.golang.org/api/option"
)

//go:embed static/*
var staticFiles embed.FS

func main() {
	// Parse flags
	var (
		port = flag.String("port", getEnv("PORT", "8080"), "Server port")
	)
	flag.Parse()
	
	// Get configuration from environment
	bucketName := getEnv("BUCKET_NAME", "")
	if bucketName == "" {
		log.Fatal("BUCKET_NAME environment variable is required")
	}
	
	btcpayURL := getEnv("BTCPAY_ENDPOINT", "")
	if btcpayURL == "" {
		log.Fatal("BTCPAY_ENDPOINT environment variable is required")
	}
	
	btcpayAPIKey := getEnv("BTCPAY_APIKEY", "")
	if btcpayAPIKey == "" {
		log.Fatal("BTCPAY_APIKEY environment variable is required")
	}
	
	// Create storage client
	ctx := context.Background()
	var storageClient *storage.Client
	var err error
	
	if emulatorHost := os.Getenv("STORAGE_EMULATOR_HOST"); emulatorHost != "" {
		log.Printf("Using Storage Emulator at %s", emulatorHost)
		storageClient, err = storage.NewClient(ctx, option.WithoutAuthentication())
	} else {
		storageClient, err = storage.NewClient(ctx)
	}
	
	if err != nil {
		log.Fatalf("Failed to create storage client: %v", err)
	}
	
	// Create server configuration
	config := &server.Config{
		Port:          *port,
		BucketName:    bucketName,
		BTCPayURL:     btcpayURL,
		BTCPayAPIKey:  btcpayAPIKey,
		StorageClient: storageClient,
		StaticFiles:   staticFiles,
	}
	
	// Create and start server
	srv := server.New(config)
	if err := srv.Start(); err != nil {
		log.Fatalf("Server failed to start: %v", err)
	}
}

// getEnv gets an environment variable with a default value
func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}