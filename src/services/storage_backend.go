// Package services provides storage backend abstractions for BinCrypt.
//
// StorageBackend: interface for paste storage with content deduplication
// GCSBackend: Google Cloud Storage implementation
// PostgresBackend: PostgreSQL implementation with pgx/v5
package services

import (
	"context"
	"errors"

	"github.com/yhakami/bincrypt/src/models"
)

var (
	ErrNotFound      = errors.New("paste not found")
	ErrAlreadyBurned = errors.New("paste already burned")
	ErrBannedContent = errors.New("content not permitted")
)

// StorageBackend defines the interface for paste storage operations.
// Implementations must support:
//   - Atomic paste creation with deduplication
//   - Burn-after-read with race-free marking
//   - Content and plaintext hash ban checking
//   - Reference-based storage for duplicates
type StorageBackend interface {
	SavePaste(ctx context.Context, id string, ciphertext string, expirySeconds int, burnAfterRead bool, isPlaintext bool) (*models.SavePasteResult, error)
	GetPaste(ctx context.Context, id string) (string, *models.Paste, error)
	MarkBurned(ctx context.Context, id string) error
	DeletePaste(ctx context.Context, id string) error
	BanContentHash(ctx context.Context, contentHash string, reason string) error
	BanPlaintextHash(ctx context.Context, plaintextHash string, reason string) error
	Ping(ctx context.Context) error
	Close() error
}

// StorageService is a facade over storage backends with unified error handling.
type StorageService struct {
	backend StorageBackend
}

func NewStorageServiceWithBackend(backend StorageBackend) *StorageService {
	return &StorageService{backend: backend}
}

func (s *StorageService) SavePaste(ctx context.Context, id string, ciphertext string, expirySeconds int, burnAfterRead bool, isPlaintext bool) (*models.SavePasteResult, error) {
	return s.backend.SavePaste(ctx, id, ciphertext, expirySeconds, burnAfterRead, isPlaintext)
}

func (s *StorageService) GetPaste(ctx context.Context, id string) (string, *models.Paste, error) {
	return s.backend.GetPaste(ctx, id)
}

func (s *StorageService) MarkBurned(ctx context.Context, id string) error {
	return s.backend.MarkBurned(ctx, id)
}

func (s *StorageService) DeletePaste(ctx context.Context, id string) error {
	return s.backend.DeletePaste(ctx, id)
}

func (s *StorageService) BanContentHash(ctx context.Context, contentHash string, reason string) error {
	return s.backend.BanContentHash(ctx, contentHash, reason)
}

func (s *StorageService) BanPlaintextHash(ctx context.Context, plaintextHash string, reason string) error {
	return s.backend.BanPlaintextHash(ctx, plaintextHash, reason)
}

func (s *StorageService) Ping(ctx context.Context) error {
	return s.backend.Ping(ctx)
}

func (s *StorageService) Close() error {
	if s.backend != nil {
		return s.backend.Close()
	}
	return nil
}
