package tests

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/yhakami/bincrypt/src/services"
)

func TestSQLiteBackend(t *testing.T) {
	// Create temporary database file
	dbPath := "/tmp/bincrypt_test.db"
	defer os.Remove(dbPath)

	backend, err := services.NewSQLiteBackend(dbPath)
	if err != nil {
		t.Fatalf("Failed to create SQLite backend: %v", err)
	}
	defer backend.Close()

	ctx := context.Background()

	t.Run("SaveAndGetPaste", func(t *testing.T) {
		id := "test_paste_123456789012345678901234567890123"
		ciphertext := "SGVsbG8gV29ybGQK" // Base64 encoded content
		expirySeconds := 3600
		burnAfterRead := false
		isPlaintext := false

		// Save paste
		result, err := backend.SavePaste(ctx, id, ciphertext, expirySeconds, burnAfterRead, isPlaintext)
		if err != nil {
			t.Fatalf("Failed to save paste: %v", err)
		}

		if result.Paste.ID != id {
			t.Errorf("Expected ID %s, got %s", id, result.Paste.ID)
		}

		if result.WasDeduplicated {
			t.Error("First paste should not be deduplicated")
		}

		// Get paste
		content, paste, err := backend.GetPaste(ctx, id)
		if err != nil {
			t.Fatalf("Failed to get paste: %v", err)
		}

		if content != ciphertext {
			t.Errorf("Expected content %s, got %s", ciphertext, content)
		}

		if paste.ID != id {
			t.Errorf("Expected ID %s, got %s", id, paste.ID)
		}

		if paste.IsBurned {
			t.Error("Paste should not be burned")
		}
	})

	t.Run("Deduplication", func(t *testing.T) {
		id1 := "test_dedup_1234567890123456789012345678901"
		id2 := "test_dedup_2234567890123456789012345678901"
		ciphertext := "RGVkdXBsaWNhdGVkQ29udGVudAo=" // Same content
		expirySeconds := 3600
		burnAfterRead := false
		isPlaintext := false

		// Save first paste
		result1, err := backend.SavePaste(ctx, id1, ciphertext, expirySeconds, burnAfterRead, isPlaintext)
		if err != nil {
			t.Fatalf("Failed to save first paste: %v", err)
		}

		if result1.WasDeduplicated {
			t.Error("First paste should not be deduplicated")
		}

		contentHash1 := result1.ContentHash

		// Save second paste with same content
		result2, err := backend.SavePaste(ctx, id2, ciphertext, expirySeconds, burnAfterRead, isPlaintext)
		if err != nil {
			t.Fatalf("Failed to save second paste: %v", err)
		}

		if !result2.WasDeduplicated {
			t.Error("Second paste should be deduplicated")
		}

		if result2.ReferencesID != id1 {
			t.Errorf("Expected reference to %s, got %s", id1, result2.ReferencesID)
		}

		if result2.ContentHash != contentHash1 {
			t.Error("Content hashes should match")
		}

		// Verify both pastes return the same content
		content1, _, err := backend.GetPaste(ctx, id1)
		if err != nil {
			t.Fatalf("Failed to get first paste: %v", err)
		}

		content2, _, err := backend.GetPaste(ctx, id2)
		if err != nil {
			t.Fatalf("Failed to get second paste: %v", err)
		}

		if content1 != content2 {
			t.Error("Deduplicated pastes should return same content")
		}
	})

	t.Run("BurnAfterRead", func(t *testing.T) {
		id := "test_burn_12345678901234567890123456789012"
		ciphertext := "QnVybkFmdGVyUmVhZAo="
		expirySeconds := 3600
		burnAfterRead := true
		isPlaintext := false

		// Save paste
		_, err := backend.SavePaste(ctx, id, ciphertext, expirySeconds, burnAfterRead, isPlaintext)
		if err != nil {
			t.Fatalf("Failed to save paste: %v", err)
		}

		// First read should succeed and atomically burn the paste.
		content, paste, err := backend.GetPaste(ctx, id)
		if err != nil {
			t.Fatalf("Failed to get paste: %v", err)
		}
		if content != ciphertext {
			t.Errorf("Expected content %s, got %s", ciphertext, content)
		}
		if !paste.BurnAfterRead {
			t.Error("Expected paste to be burn-after-read")
		}
		if !paste.IsBurned {
			t.Error("Expected paste to be marked burned on first read")
		}

		// Second read should return burned marker (no content).
		content, paste, err = backend.GetPaste(ctx, id)
		if err != nil {
			t.Fatalf("Expected burned paste to return nil error, got %v", err)
		}
		if content != "" {
			t.Errorf("Expected empty content for burned paste, got %s", content)
		}
		if paste == nil || !paste.IsBurned {
			t.Error("Expected paste.IsBurned=true for burned paste")
		}
	})

	t.Run("DeletePaste", func(t *testing.T) {
		id := "test_delete_123456789012345678901234567890"
		ciphertext := "RGVsZXRlVGVzdAo="
		expirySeconds := 3600
		burnAfterRead := false
		isPlaintext := false

		// Save paste
		_, err := backend.SavePaste(ctx, id, ciphertext, expirySeconds, burnAfterRead, isPlaintext)
		if err != nil {
			t.Fatalf("Failed to save paste: %v", err)
		}

		// Delete paste
		err = backend.DeletePaste(ctx, id)
		if err != nil {
			t.Fatalf("Failed to delete paste: %v", err)
		}

		// Try to get deleted paste
		_, _, err = backend.GetPaste(ctx, id)
		if err != services.ErrNotFound {
			t.Errorf("Expected ErrNotFound, got %v", err)
		}
	})

	t.Run("BanContentHash", func(t *testing.T) {
		id := "test_banned_123456789012345678901234567890"
		ciphertext := "QmFubmVkQ29udGVudAo="
		expirySeconds := 3600
		burnAfterRead := false
		isPlaintext := false

		// Save paste first time
		result, err := backend.SavePaste(ctx, id, ciphertext, expirySeconds, burnAfterRead, isPlaintext)
		if err != nil {
			t.Fatalf("Failed to save paste: %v", err)
		}

		contentHash := result.ContentHash

		// Ban the content hash
		err = backend.BanContentHash(ctx, contentHash, "test ban")
		if err != nil {
			t.Fatalf("Failed to ban content hash: %v", err)
		}

		// Try to save same content again
		id2 := "test_banned_223456789012345678901234567890"
		_, err = backend.SavePaste(ctx, id2, ciphertext, expirySeconds, burnAfterRead, isPlaintext)
		if err != services.ErrBannedContent {
			t.Errorf("Expected ErrBannedContent, got %v", err)
		}
	})

	t.Run("ExpiredPaste", func(t *testing.T) {
		id := "test_expired_123456789012345678901234567890"
		ciphertext := "RXhwaXJlZFBhc3RlCg=="
		expirySeconds := 1 // 1 second
		burnAfterRead := false
		isPlaintext := false

		// Save paste
		_, err := backend.SavePaste(ctx, id, ciphertext, expirySeconds, burnAfterRead, isPlaintext)
		if err != nil {
			t.Fatalf("Failed to save paste: %v", err)
		}

		// Wait for expiration
		time.Sleep(2 * time.Second)

		// Try to get expired paste
		_, _, err = backend.GetPaste(ctx, id)
		if err != services.ErrNotFound {
			t.Errorf("Expected ErrNotFound for expired paste, got %v", err)
		}
	})
}
