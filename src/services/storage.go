package services

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"

	"bincrypt/src/models"
	"cloud.google.com/go/storage"
	"google.golang.org/api/iterator"
)

// StorageService handles all GCS operations
type StorageService struct {
	bucket *storage.BucketHandle
}

// NewStorageService creates a new storage service
func NewStorageService(bucket *storage.BucketHandle) *StorageService {
	return &StorageService{bucket: bucket}
}

// SavePaste saves encrypted paste content and metadata
func (s *StorageService) SavePaste(ctx context.Context, id string, ciphertext string, expirySeconds int, burnAfterRead bool) (*models.Paste, error) {
	// Save encrypted content
	obj := s.bucket.Object(s.pasteObjectName(id))
	writer := obj.NewWriter(ctx)
	
	expiresAt := time.Now().Add(time.Duration(expirySeconds) * time.Second).UTC()
	writer.Metadata = map[string]string{
		"expiry": strconv.FormatInt(expiresAt.Unix(), 10),
		"size":   strconv.Itoa(len(ciphertext)),
		"burn":   strconv.FormatBool(burnAfterRead),
	}
	writer.ContentType = "application/octet-stream"
	
	if _, err := writer.Write([]byte(ciphertext)); err != nil {
		return nil, fmt.Errorf("failed to write paste: %w", err)
	}
	if err := writer.Close(); err != nil {
		return nil, fmt.Errorf("failed to save paste: %w", err)
	}
	
	// Save metadata JSON
	paste := &models.Paste{
		ID:            id,
		ExpiresAt:     expiresAt,
		BurnAfterRead: burnAfterRead,
		SizeBytes:     len(ciphertext),
		CreatedAt:     time.Now().UTC(),
		IsBurned:      false,
	}
	
	metaObj := s.bucket.Object(s.pasteMetaObjectName(id))
	metaWriter := metaObj.NewWriter(ctx)
	metaWriter.ContentType = "application/json"
	if err := json.NewEncoder(metaWriter).Encode(paste); err != nil {
		_ = metaWriter.Close()
		return nil, fmt.Errorf("failed to write metadata: %w", err)
	}
	if err := metaWriter.Close(); err != nil {
		return nil, fmt.Errorf("failed to save metadata: %w", err)
	}
	
	return paste, nil
}

// GetPaste retrieves paste content and metadata
func (s *StorageService) GetPaste(ctx context.Context, id string) (string, *models.Paste, error) {
	obj := s.bucket.Object(s.pasteObjectName(id))
	
	// Check if paste exists and get metadata
	attrs, err := obj.Attrs(ctx)
	if err == storage.ErrObjectNotExist {
		return "", nil, storage.ErrObjectNotExist
	} else if err != nil {
		return "", nil, fmt.Errorf("failed to get paste attributes: %w", err)
	}
	
	// Check expiry
	if expiryStr := attrs.Metadata["expiry"]; expiryStr != "" {
		if expiry, err := strconv.ParseInt(expiryStr, 10, 64); err == nil {
			if time.Now().Unix() > expiry {
				// Delete expired paste
				_ = obj.Delete(ctx)
				_ = s.bucket.Object(s.pasteMetaObjectName(id)).Delete(ctx)
				return "", nil, storage.ErrObjectNotExist
			}
		}
	}
	
	// Read content
	reader, err := obj.NewReader(ctx)
	if err != nil {
		return "", nil, fmt.Errorf("failed to read paste: %w", err)
	}
	defer reader.Close()
	
	content, err := io.ReadAll(reader)
	if err != nil {
		return "", nil, fmt.Errorf("failed to read paste content: %w", err)
	}
	
	// Read metadata
	var paste models.Paste
	metaObj := s.bucket.Object(s.pasteMetaObjectName(id))
	metaReader, err := metaObj.NewReader(ctx)
	if err == nil {
		json.NewDecoder(metaReader).Decode(&paste)
		metaReader.Close()
	} else {
		// Fallback to attrs metadata
		paste = models.Paste{
			ID:            id,
			BurnAfterRead: attrs.Metadata["burn"] == "true",
			SizeBytes:     len(content),
		}
	}
	
	return string(content), &paste, nil
}

// MarkBurned marks a paste as burned
func (s *StorageService) MarkBurned(ctx context.Context, id string) error {
	metaObj := s.bucket.Object(s.pasteMetaObjectName(id))
	
	// Read current metadata
	attrs, err := metaObj.Attrs(ctx)
	if err != nil {
		return err
	}
	
	reader, err := metaObj.NewReader(ctx)
	if err != nil {
		return err
	}
	
	var paste models.Paste
	if err := json.NewDecoder(reader).Decode(&paste); err != nil {
		reader.Close()
		return err
	}
	reader.Close()
	
	if paste.IsBurned {
		return nil // Already burned
	}
	
	// Update with generation match to ensure atomicity
	paste.IsBurned = true
	writer := metaObj.If(storage.Conditions{GenerationMatch: attrs.Generation}).NewWriter(ctx)
	writer.ContentType = "application/json"
	if err := json.NewEncoder(writer).Encode(&paste); err != nil {
		writer.Close()
		return err
	}
	return writer.Close()
}

// DeletePaste deletes paste content and metadata
func (s *StorageService) DeletePaste(ctx context.Context, id string) error {
	if err := s.bucket.Object(s.pasteObjectName(id)).Delete(ctx); err != nil && err != storage.ErrObjectNotExist {
		return err
	}
	if err := s.bucket.Object(s.pasteMetaObjectName(id)).Delete(ctx); err != nil && err != storage.ErrObjectNotExist {
		return err
	}
	return nil
}

// CleanupExpiredPastes removes expired pastes
func (s *StorageService) CleanupExpiredPastes(ctx context.Context) error {
	it := s.bucket.Objects(ctx, &storage.Query{Prefix: "pastes/", Delimiter: ""})
	now := time.Now().Unix()
	
	for {
		attrs, err := it.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return fmt.Errorf("error listing objects: %w", err)
		}
		
		if strings.HasSuffix(attrs.Name, ".bin") {
			if expiryStr := attrs.Metadata["expiry"]; expiryStr != "" {
				expiry, _ := strconv.ParseInt(expiryStr, 10, 64)
				if now > expiry {
					if err := s.bucket.Object(attrs.Name).Delete(ctx); err != nil {
						// Log but continue
						continue
					}
					// Delete metadata too
					metaName := strings.TrimSuffix(attrs.Name, ".bin") + ".json"
					_ = s.bucket.Object(metaName).Delete(ctx)
				}
			}
		}
	}
	
	return nil
}

// Object name helpers
func (s *StorageService) pasteObjectName(id string) string {
	return fmt.Sprintf("pastes/%s.bin", id)
}

func (s *StorageService) pasteMetaObjectName(id string) string {
	return fmt.Sprintf("pastes/%s.json", id)
}