// Package services provides PostgreSQL storage backend implementation.
package services

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/yhakami/bincrypt/src/models"
)

type PostgresBackend struct {
	pool *pgxpool.Pool
}

func NewPostgresBackend(pool *pgxpool.Pool) *PostgresBackend {
	return &PostgresBackend{pool: pool}
}

func (s *PostgresBackend) SavePaste(ctx context.Context, id string, ciphertext string, expirySeconds int, burnAfterRead bool, isPlaintext bool) (*models.SavePasteResult, error) {
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
			banned, err := s.isPlaintextHashBanned(ctx, plaintextHash)
			if err != nil {
				return nil, fmt.Errorf("failed to check plaintext banned status: %w", err)
			}
			if banned {
				return nil, ErrBannedContent
			}
		}
	} else {
		hash := sha256.Sum256([]byte(ciphertext))
		contentHash = hex.EncodeToString(hash[:])
	}

	banned, err := s.isHashBanned(ctx, contentHash)
	if err != nil {
		return nil, fmt.Errorf("failed to check banned status: %w", err)
	}
	if banned {
		return nil, ErrBannedContent
	}

	now := time.Now().UTC()
	expiresAt := now.Add(time.Duration(expirySeconds) * time.Second)

	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{
		IsoLevel: pgx.Serializable,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	var existingID string
	err = tx.QueryRow(ctx, "SELECT paste_id FROM hash_index WHERE content_hash = $1", contentHash).Scan(&existingID)
	if err != nil && err != pgx.ErrNoRows {
		return nil, fmt.Errorf("failed to check for existing content: %w", err)
	}

	if existingID != "" {
		_, err = tx.Exec(ctx, `
			INSERT INTO pastes (id, references_id, content_hash, plaintext_hash, expires_at, burn_after_read, size_bytes, is_plaintext, created_at)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`,
			id, existingID, contentHash, nullString(plaintextHash), expiresAt, burnAfterRead, len(ciphertext), isPlaintext, now)
		if err != nil {
			return nil, fmt.Errorf("failed to create reference paste: %w", err)
		}

		if err := tx.Commit(ctx); err != nil {
			return nil, fmt.Errorf("failed to commit transaction: %w", err)
		}

		return &models.SavePasteResult{
			Paste: &models.Paste{
				ID:            id,
				ExpiresAt:     expiresAt,
				BurnAfterRead: burnAfterRead,
				SizeBytes:     len(ciphertext),
				CreatedAt:     now,
				IsBurned:      false,
			},
			WasDeduplicated: true,
			ReferencesID:    existingID,
			ContentHash:     contentHash,
		}, nil
	}

	_, err = tx.Exec(ctx, `
		INSERT INTO pastes (id, content, content_hash, plaintext_hash, expires_at, burn_after_read, size_bytes, is_plaintext, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`,
		id, ciphertext, contentHash, nullString(plaintextHash), expiresAt, burnAfterRead, len(ciphertext), isPlaintext, now)
	if err != nil {
		return nil, fmt.Errorf("failed to save paste: %w", err)
	}

	_, err = tx.Exec(ctx, "INSERT INTO hash_index (content_hash, paste_id) VALUES ($1, $2) ON CONFLICT DO NOTHING", contentHash, id)
	if err != nil {
		return nil, fmt.Errorf("failed to create hash index: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("failed to commit transaction: %w", err)
	}

	return &models.SavePasteResult{
		Paste: &models.Paste{
			ID:            id,
			ExpiresAt:     expiresAt,
			BurnAfterRead: burnAfterRead,
			SizeBytes:     len(ciphertext),
			CreatedAt:     now,
			IsBurned:      false,
		},
		WasDeduplicated: false,
		ContentHash:     contentHash,
	}, nil
}

func (s *PostgresBackend) GetPaste(ctx context.Context, id string) (string, *models.Paste, error) {
	// For burn-after-read pastes, we need atomicity to prevent race conditions.
	// Use a transaction with serializable isolation to ensure only one reader succeeds.
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{
		IsoLevel: pgx.Serializable,
	})
	if err != nil {
		return "", nil, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	// Check if already burned (atomic within transaction).
	// Burned pastes are treated as "gone" (returned as IsBurned=true with empty content).
	var isBurned bool
	err = tx.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM burned_pastes WHERE paste_id = $1)", id).Scan(&isBurned)
	if err != nil {
		return "", nil, fmt.Errorf("failed to check burn status: %w", err)
	}
	if isBurned {
		return "", &models.Paste{ID: id, IsBurned: true}, nil
	}

	// Fetch paste metadata with expiry check in SQL (atomic within transaction).
	// For burn-after-read pastes, we claim the burn marker BEFORE reading the content.
	var referencesID *string
	var expiresAt time.Time
	var burnAfterRead bool
	var sizeBytes int
	var createdAt time.Time

	err = tx.QueryRow(ctx, `
		SELECT references_id, expires_at, burn_after_read, size_bytes, created_at
		FROM pastes
		WHERE id = $1 AND (expires_at > NOW() OR expires_at IS NULL)`,
		id).Scan(&referencesID, &expiresAt, &burnAfterRead, &sizeBytes, &createdAt)

	if err == pgx.ErrNoRows {
		return "", nil, ErrNotFound
	}
	if err != nil {
		return "", nil, fmt.Errorf("failed to get paste: %w", err)
	}

	paste := &models.Paste{
		ID:            id,
		ExpiresAt:     expiresAt,
		BurnAfterRead: burnAfterRead,
		SizeBytes:     sizeBytes,
		CreatedAt:     createdAt,
		IsBurned:      false,
	}

	// Atomically claim burn-after-read pastes before reading content.
	// This matches the GCS behavior: first reader succeeds and receives the content,
	// subsequent readers receive IsBurned=true with empty content.
	if burnAfterRead {
		tag, err := tx.Exec(ctx, "INSERT INTO burned_pastes (paste_id, burned_at) VALUES ($1, NOW()) ON CONFLICT DO NOTHING", id)
		if err != nil {
			return "", nil, fmt.Errorf("failed to mark paste as burned: %w", err)
		}
		if tag.RowsAffected() == 0 {
			return "", &models.Paste{ID: id, IsBurned: true}, nil
		}
		paste.IsBurned = true
	}

	// Resolve content (handle references, atomic within transaction)
	var finalContent string
	contentID := id
	if referencesID != nil && *referencesID != "" {
		contentID = *referencesID
	}

	var content *string
	err = tx.QueryRow(ctx, "SELECT content FROM pastes WHERE id = $1", contentID).Scan(&content)
	if err == pgx.ErrNoRows {
		return "", nil, ErrNotFound
	}
	if err != nil {
		return "", nil, fmt.Errorf("failed to get paste content: %w", err)
	}
	if content == nil {
		return "", nil, ErrNotFound
	}
	finalContent = *content

	// Commit transaction (makes burn marking permanent)
	if err := tx.Commit(ctx); err != nil {
		return "", nil, fmt.Errorf("failed to commit transaction: %w", err)
	}

	return finalContent, paste, nil
}

func (s *PostgresBackend) MarkBurned(ctx context.Context, id string) error {
	_, err := s.pool.Exec(ctx, "INSERT INTO burned_pastes (paste_id, burned_at) VALUES ($1, NOW()) ON CONFLICT DO NOTHING", id)
	if err != nil {
		return fmt.Errorf("failed to mark paste as burned: %w", err)
	}
	return nil
}

func (s *PostgresBackend) DeletePaste(ctx context.Context, id string) error {
	_, err := s.pool.Exec(ctx, "DELETE FROM pastes WHERE id = $1", id)
	if err != nil {
		return fmt.Errorf("failed to delete paste: %w", err)
	}

	_, _ = s.pool.Exec(ctx, "DELETE FROM burned_pastes WHERE paste_id = $1", id)

	return nil
}

func (s *PostgresBackend) BanContentHash(ctx context.Context, contentHash string, reason string) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO banned_hashes (hash, reason, hash_type, banned_at)
		VALUES ($1, $2, 'ciphertext', NOW())
		ON CONFLICT (hash) DO UPDATE SET reason = $2, banned_at = NOW()`,
		contentHash, reason)
	if err != nil {
		return fmt.Errorf("failed to ban content hash: %w", err)
	}
	return nil
}

func (s *PostgresBackend) BanPlaintextHash(ctx context.Context, plaintextHash string, reason string) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO banned_hashes (hash, reason, hash_type, banned_at)
		VALUES ($1, $2, 'plaintext', NOW())
		ON CONFLICT (hash) DO UPDATE SET reason = $2, banned_at = NOW()`,
		plaintextHash, reason)
	if err != nil {
		return fmt.Errorf("failed to ban plaintext hash: %w", err)
	}
	return nil
}

func (s *PostgresBackend) Ping(ctx context.Context) error {
	return s.pool.Ping(ctx)
}

func (s *PostgresBackend) Close() error {
	s.pool.Close()
	return nil
}

func (s *PostgresBackend) isHashBanned(ctx context.Context, contentHash string) (bool, error) {
	var exists bool
	err := s.pool.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM banned_hashes WHERE hash = $1 AND hash_type = 'ciphertext')", contentHash).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("failed to check banned status: %w", err)
	}
	return exists, nil
}

func (s *PostgresBackend) isPlaintextHashBanned(ctx context.Context, plaintextHash string) (bool, error) {
	var exists bool
	err := s.pool.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM banned_hashes WHERE hash = $1 AND hash_type = 'plaintext')", plaintextHash).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("failed to check plaintext banned status: %w", err)
	}
	return exists, nil
}

func nullString(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
