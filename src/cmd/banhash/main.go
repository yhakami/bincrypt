//go:build gcp

package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"

	"cloud.google.com/go/storage"
	"github.com/yhakami/bincrypt/src/config"
	"github.com/yhakami/bincrypt/src/services"
)

func main() {
	var (
		contentHash = flag.String("hash", "", "Content hash to ban")
		reason      = flag.String("reason", "", "Reason for banning")
		bucketName  = flag.String("bucket", "", "GCS bucket name (defaults to BUCKET_NAME env var)")
	)
	flag.Parse()

	if *contentHash == "" {
		fmt.Println("Usage: banhash -hash <SHA256_HASH> -reason <REASON> [-bucket <BUCKET>]")
		fmt.Println("\nThis bans the ciphertext hash (for encrypted pastes)")
		fmt.Println("To ban plaintext content, use banplaintext command")
		os.Exit(1)
	}

	if *reason == "" {
		fmt.Println("Error: -reason is required")
		os.Exit(1)
	}

	// Initialize secrets provider (env/.env or Secret Manager)
	ctx := context.Background()
	if err := config.Initialize(ctx); err != nil {
		log.Fatalf("Failed to initialize config: %v", err)
	}
	defer config.Close()

	// Get bucket name
	bucket := *bucketName
	if bucket == "" {
		bucket = config.GetSecretOrDefault(ctx, "BUCKET_NAME", "")
		if bucket == "" {
			fmt.Println("Error: BUCKET_NAME not set and -bucket not provided")
			os.Exit(1)
		}
	}

	// Create storage client
	client, err := storage.NewClient(ctx)
	if err != nil {
		log.Fatalf("Failed to create storage client: %v", err)
	}
	defer client.Close()

	// Create storage service
	bucketHandle := client.Bucket(bucket)
	storageService := services.NewStorageService(bucketHandle)

	// Ban the hash
	if err := storageService.BanContentHash(ctx, *contentHash, *reason); err != nil {
		log.Fatalf("Failed to ban hash: %v", err)
	}

	fmt.Printf("Successfully banned ciphertext hash: %s\n", *contentHash)
	fmt.Printf("Reason: %s\n", *reason)
	fmt.Println("\nNote: This will block encrypted pastes with this exact ciphertext")
}
