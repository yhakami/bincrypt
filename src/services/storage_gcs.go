//go:build gcp

// Package services provides GCS storage backend implementation.
package services

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"time"

	"cloud.google.com/go/storage"
	"github.com/yhakami/bincrypt/src/logger"
	"github.com/yhakami/bincrypt/src/models"
)

type GCSBackend struct {
	bucket *storage.BucketHandle
}

func NewGCSBackend(bucket *storage.BucketHandle) *GCSBackend {
	return &GCSBackend{bucket: bucket}
}

func (s *GCSBackend) SavePaste(ctx context.Context, id string, ciphertext string, expirySeconds int, burnAfterRead bool, isPlaintext bool) (*models.SavePasteResult, error) {
	var contentHash string
	var plaintextHash string

	if isPlaintext {
		decoded, err := base64.StdEncoding.DecodeString(ciphertext)
		if err != nil {
			return nil, fmt.Errorf("failed to decode plaintext content: %w", err)
		}

		var plaintextData map[string]interface{}
		if err := json.Unmarshal(decoded, &plaintextData); err == nil {
			if content, ok := plaintextData["content"].(string); ok {
				hash := sha256.Sum256([]byte(content))
				plaintextHash = hex.EncodeToString(hash[:])
			}
		}

		hash := sha256.Sum256([]byte(ciphertext))
		contentHash = hex.EncodeToString(hash[:])

		if plaintextHash != "" {
			if banned, err := s.isPlaintextHashBanned(ctx, plaintextHash); err != nil {
				return nil, fmt.Errorf("failed to check plaintext banned status: %w", err)
			} else if banned {
				return nil, ErrBannedContent
			}
		}
	} else {
		hash := sha256.Sum256([]byte(ciphertext))
		contentHash = hex.EncodeToString(hash[:])
	}

	if banned, err := s.isHashBanned(ctx, contentHash); err != nil {
		return nil, fmt.Errorf("failed to check banned status: %w", err)
	} else if banned {
		return nil, ErrBannedContent
	}

	existingID, err := s.findByContentHash(ctx, contentHash)
	if err != nil {
		return nil, fmt.Errorf("failed to check for existing content: %w", err)
	}

	now := time.Now().UTC()
	expiresAt := now.Add(time.Duration(expirySeconds) * time.Second)

	if existingID != "" {
		paste, err := s.createReferencePaste(ctx, id, existingID, contentHash, expiresAt, burnAfterRead, len(ciphertext), now, isPlaintext, plaintextHash)
		if err != nil {
			return nil, err
		}
		return &models.SavePasteResult{
			Paste:           paste,
			WasDeduplicated: true,
			ReferencesID:    existingID,
			ContentHash:     contentHash,
		}, nil
	}

	obj := s.bucket.Object(s.pasteObjectName(id))
	writer := obj.NewWriter(ctx)

	writer.Metadata = map[string]string{
		"bincrypt-id":              id,
		"bincrypt-expires-at":      strconv.FormatInt(expiresAt.Unix(), 10),
		"bincrypt-burn-after-read": strconv.FormatBool(burnAfterRead),
		"bincrypt-size-bytes":      strconv.Itoa(len(ciphertext)),
		"bincrypt-created-at":      strconv.FormatInt(now.Unix(), 10),
		"bincrypt-is-burned":       "false",
		"bincrypt-content-hash":    contentHash,
		"bincrypt-is-plaintext":    strconv.FormatBool(isPlaintext),
		"bincrypt-plaintext-hash":  plaintextHash,
	}
	writer.ContentType = "application/octet-stream"

	if _, err := writer.Write([]byte(ciphertext)); err != nil {
		return nil, fmt.Errorf("failed to write paste: %w", err)
	}
	if err := writer.Close(); err != nil {
		return nil, fmt.Errorf("failed to save paste: %w", err)
	}

	if err := s.createHashIndex(ctx, contentHash, id); err != nil {
		logger.WithContext(ctx).Warn("hash_index_creation_failed", logger.Fields{"error": err.Error(), "paste_id_hash": logger.HashPasteID(id)})
	}

	paste := &models.Paste{
		ID:            id,
		ExpiresAt:     expiresAt,
		BurnAfterRead: burnAfterRead,
		SizeBytes:     len(ciphertext),
		CreatedAt:     now,
		IsBurned:      false,
	}

	return &models.SavePasteResult{
		Paste:           paste,
		WasDeduplicated: false,
		ContentHash:     contentHash,
	}, nil
}

func (s *GCSBackend) GetPaste(ctx context.Context, id string) (string, *models.Paste, error) {
	// Step 1: Get paste object and its current generation number
	// This generation number is immutable for this version of the object and provides
	// a consistent snapshot we can use for atomic operations.
	obj := s.bucket.Object(s.pasteObjectName(id))
	attrs, err := obj.Attrs(ctx)
	if err == storage.ErrObjectNotExist {
		return "", nil, ErrNotFound
	} else if err != nil {
		return "", nil, fmt.Errorf("failed to get paste attributes: %w", err)
	}

	// Step 2: Check if burn marker already exists (early exit for already-burned pastes)
	// This prevents reading content for pastes that were already consumed.
	burnObj := s.bucket.Object(s.burnObjectName(id))
	if _, err := burnObj.Attrs(ctx); err == nil {
		// Burn marker exists - paste was already consumed
		return "", &models.Paste{
			ID:       id,
			IsBurned: true,
		}, nil
	}

	// Step 3: Parse paste metadata
	paste := &models.Paste{
		ID: id,
	}

	if expiresAt, err := strconv.ParseInt(attrs.Metadata["bincrypt-expires-at"], 10, 64); err == nil {
		paste.ExpiresAt = time.Unix(expiresAt, 0).UTC()
	}
	if createdAt, err := strconv.ParseInt(attrs.Metadata["bincrypt-created-at"], 10, 64); err == nil {
		paste.CreatedAt = time.Unix(createdAt, 0).UTC()
	}
	paste.BurnAfterRead = attrs.Metadata["bincrypt-burn-after-read"] == "true"
	if sizeBytes, err := strconv.Atoi(attrs.Metadata["bincrypt-size-bytes"]); err == nil {
		paste.SizeBytes = sizeBytes
	}

	// Step 4: Follow reference if this is a deduplicated paste
	if referencesID := attrs.Metadata["bincrypt-references"]; referencesID != "" {
		refObj := s.bucket.Object(s.pasteObjectName(referencesID))
		refAttrs, err := refObj.Attrs(ctx)
		if err != nil {
			return "", nil, fmt.Errorf("referenced content not found: %w", err)
		}

		// For referenced content, we use the reference's generation for reading
		// but the original paste's ID for burn markers
		obj = refObj
		attrs = refAttrs
	}

	// Step 5: Check expiration
	if !paste.ExpiresAt.IsZero() && time.Now().After(paste.ExpiresAt) {
		_ = obj.Delete(ctx)
		_ = burnObj.Delete(ctx)
		return "", nil, ErrNotFound
	}

	// Step 6: For burn-after-read pastes, atomically claim the burn marker BEFORE reading content
	// This is the critical race-condition fix: we use GCS DoesNotExist precondition to ensure
	// only ONE concurrent reader can successfully create the burn marker. Others will fail
	// and return ErrNotFound.
	if paste.BurnAfterRead {
		// Try to atomically create the burn marker with DoesNotExist precondition
		// Only the first request will succeed; all others will get a precondition failure
		burnWriter := burnObj.If(storage.Conditions{DoesNotExist: true}).NewWriter(ctx)
		burnWriter.ContentType = "text/plain"
		burnWriter.Metadata = map[string]string{
			"burned-at": strconv.FormatInt(time.Now().Unix(), 10),
			"paste-id":  id,
		}

		// Write burn timestamp
		if _, err := burnWriter.Write([]byte(time.Now().Format(time.RFC3339))); err != nil {
			return "", nil, fmt.Errorf("failed to write burn marker: %w", err)
		}

		// Close will fail if another request already created the burn marker
		if err := burnWriter.Close(); err != nil {
			// Precondition failed means someone else won the race - this paste is burned
			if err.Error() == "googleapi: Error 412: Precondition Failed" || err.Error() == "googleapi: Error 412: At least one of the pre-conditions you specified did not hold., conditionNotMet" {
				return "", &models.Paste{
					ID:       id,
					IsBurned: true,
				}, nil
			}
			return "", nil, fmt.Errorf("failed to create burn marker: %w", err)
		}

		// Success! We won the race and can safely read the content
		paste.IsBurned = true
	}

	// Step 7: Read paste content with generation match precondition
	// This ensures we read the exact object version we examined, preventing TOCTOU issues
	reader, err := obj.If(storage.Conditions{GenerationMatch: attrs.Generation}).NewReader(ctx)
	if err == storage.ErrObjectNotExist {
		return "", nil, ErrNotFound
	} else if err != nil {
		return "", nil, fmt.Errorf("failed to read paste: %w", err)
	}
	defer reader.Close()

	content, err := io.ReadAll(reader)
	if err != nil {
		return "", nil, fmt.Errorf("failed to read paste content: %w", err)
	}

	if paste.SizeBytes == 0 {
		paste.SizeBytes = len(content)
	}

	return string(content), paste, nil
}

func (s *GCSBackend) MarkBurned(ctx context.Context, id string) error {
	burnObj := s.bucket.Object(s.burnObjectName(id))

	if _, err := burnObj.Attrs(ctx); err == nil {
		return nil
	}

	writer := burnObj.NewWriter(ctx)
	writer.ContentType = "application/json"
	writer.Metadata = map[string]string{
		"burned-at": strconv.FormatInt(time.Now().Unix(), 10),
		"paste-id":  id,
	}

	if err := writer.Close(); err != nil {
		return fmt.Errorf("failed to mark paste as burned: %w", err)
	}

	return nil
}

func (s *GCSBackend) DeletePaste(ctx context.Context, id string) error {
	if err := s.bucket.Object(s.pasteObjectName(id)).Delete(ctx); err != nil && err != storage.ErrObjectNotExist {
		return err
	}

	_ = s.bucket.Object(s.burnObjectName(id)).Delete(ctx)

	return nil
}

func (s *GCSBackend) BanContentHash(ctx context.Context, contentHash string, reason string) error {
	obj := s.bucket.Object(s.bannedHashObjectName(contentHash))
	writer := obj.NewWriter(ctx)

	writer.Metadata = map[string]string{
		"banned-at": strconv.FormatInt(time.Now().Unix(), 10),
		"reason":    reason,
		"type":      "ciphertext",
	}
	writer.ContentType = "application/octet-stream"

	if err := writer.Close(); err != nil {
		return fmt.Errorf("failed to ban hash: %w", err)
	}

	return nil
}

func (s *GCSBackend) BanPlaintextHash(ctx context.Context, plaintextHash string, reason string) error {
	obj := s.bucket.Object(s.bannedPlaintextHashObjectName(plaintextHash))
	writer := obj.NewWriter(ctx)

	writer.Metadata = map[string]string{
		"banned-at": strconv.FormatInt(time.Now().Unix(), 10),
		"reason":    reason,
		"type":      "plaintext",
	}
	writer.ContentType = "application/octet-stream"

	if err := writer.Close(); err != nil {
		return fmt.Errorf("failed to ban plaintext hash: %w", err)
	}

	return nil
}

func (s *GCSBackend) Ping(ctx context.Context) error {
	// Check if bucket exists and is accessible
	_, err := s.bucket.Attrs(ctx)
	return err
}

func (s *GCSBackend) Close() error {
	return nil
}

func (s *GCSBackend) pasteObjectName(id string) string {
	return fmt.Sprintf("pastes/%s", id)
}

func (s *GCSBackend) burnObjectName(id string) string {
	return fmt.Sprintf("burns/%s", id)
}

func (s *GCSBackend) hashIndexObjectName(hash string) string {
	return fmt.Sprintf("hashes/%s", hash)
}

func (s *GCSBackend) bannedHashObjectName(hash string) string {
	return fmt.Sprintf("banned-hashes/ciphertext/%s", hash)
}

func (s *GCSBackend) bannedPlaintextHashObjectName(hash string) string {
	return fmt.Sprintf("banned-hashes/plaintext/%s", hash)
}

func (s *GCSBackend) findByContentHash(ctx context.Context, contentHash string) (string, error) {
	obj := s.bucket.Object(s.hashIndexObjectName(contentHash))
	attrs, err := obj.Attrs(ctx)
	if err == storage.ErrObjectNotExist {
		return "", nil
	} else if err != nil {
		return "", fmt.Errorf("failed to check hash index: %w", err)
	}

	return attrs.Metadata["paste-id"], nil
}

func (s *GCSBackend) createHashIndex(ctx context.Context, contentHash string, pasteID string) error {
	obj := s.bucket.Object(s.hashIndexObjectName(contentHash))
	writer := obj.NewWriter(ctx)

	writer.Metadata = map[string]string{
		"paste-id":   pasteID,
		"created-at": strconv.FormatInt(time.Now().Unix(), 10),
	}
	writer.ContentType = "application/octet-stream"

	if err := writer.Close(); err != nil {
		return fmt.Errorf("failed to create hash index: %w", err)
	}

	return nil
}

func (s *GCSBackend) createReferencePaste(ctx context.Context, id string, referencesID string, contentHash string, expiresAt time.Time, burnAfterRead bool, sizeBytes int, now time.Time, isPlaintext bool, plaintextHash string) (*models.Paste, error) {
	obj := s.bucket.Object(s.pasteObjectName(id))
	writer := obj.NewWriter(ctx)

	writer.Metadata = map[string]string{
		"bincrypt-id":              id,
		"bincrypt-expires-at":      strconv.FormatInt(expiresAt.Unix(), 10),
		"bincrypt-burn-after-read": strconv.FormatBool(burnAfterRead),
		"bincrypt-size-bytes":      strconv.Itoa(sizeBytes),
		"bincrypt-created-at":      strconv.FormatInt(now.Unix(), 10),
		"bincrypt-is-burned":       "false",
		"bincrypt-content-hash":    contentHash,
		"bincrypt-references":      referencesID,
		"bincrypt-is-plaintext":    strconv.FormatBool(isPlaintext),
		"bincrypt-plaintext-hash":  plaintextHash,
	}
	writer.ContentType = "application/octet-stream"

	if err := writer.Close(); err != nil {
		return nil, fmt.Errorf("failed to create reference paste: %w", err)
	}

	return &models.Paste{
		ID:            id,
		ExpiresAt:     expiresAt,
		BurnAfterRead: burnAfterRead,
		SizeBytes:     sizeBytes,
		CreatedAt:     now,
		IsBurned:      false,
	}, nil
}

func (s *GCSBackend) isHashBanned(ctx context.Context, contentHash string) (bool, error) {
	obj := s.bucket.Object(s.bannedHashObjectName(contentHash))
	_, err := obj.Attrs(ctx)
	if err == storage.ErrObjectNotExist {
		return false, nil
	} else if err != nil {
		return false, fmt.Errorf("failed to check banned status: %w", err)
	}
	return true, nil
}

func (s *GCSBackend) isPlaintextHashBanned(ctx context.Context, plaintextHash string) (bool, error) {
	obj := s.bucket.Object(s.bannedPlaintextHashObjectName(plaintextHash))
	_, err := obj.Attrs(ctx)
	if err == storage.ErrObjectNotExist {
		return false, nil
	} else if err != nil {
		return false, fmt.Errorf("failed to check plaintext banned status: %w", err)
	}
	return true, nil
}
