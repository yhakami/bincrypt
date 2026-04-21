//go:build gcp

package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
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
		content     = flag.String("content", "", "Plaintext content to ban")
		contentFile = flag.String("file", "", "File containing plaintext content to ban")
		hash        = flag.String("hash", "", "SHA-256 hash of plaintext to ban (if you already have it)")
		reason      = flag.String("reason", "", "Reason for banning")
		bucketName  = flag.String("bucket", "", "GCS bucket name (defaults to BUCKET_NAME env var)")
	)
	flag.Parse()

	if *content == "" && *contentFile == "" && *hash == "" {
		fmt.Println("Usage: banplaintext -content <CONTENT> | -file <FILE> | -hash <HASH> -reason <REASON> [-bucket <BUCKET>]")
		fmt.Println("\nThis bans the plaintext content (for unencrypted pastes)")
		os.Exit(1)
	}

	if *reason == "" {
		fmt.Println("Error: -reason is required")
		os.Exit(1)
	}

	// Calculate hash if not provided
	plaintextHash := *hash
	if plaintextHash == "" {
		var contentBytes []byte

		if *contentFile != "" {
			// Read from file
			data, err := os.ReadFile(*contentFile)
			if err != nil {
				log.Fatalf("Failed to read file: %v", err)
			}
			contentBytes = data
		} else {
			// Use provided content
			contentBytes = []byte(*content)
		}

		// Calculate SHA-256 hash
		hash := sha256.Sum256(contentBytes)
		plaintextHash = hex.EncodeToString(hash[:])
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

	// Ban the plaintext hash
	if err := storageService.BanPlaintextHash(ctx, plaintextHash, *reason); err != nil {
		log.Fatalf("Failed to ban plaintext hash: %v", err)
	}

	fmt.Printf("Successfully banned plaintext hash: %s\n", plaintextHash)
	fmt.Printf("Reason: %s\n", *reason)
	fmt.Println("\nNote: This will block unencrypted pastes with this exact content")
}
