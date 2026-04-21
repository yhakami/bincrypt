// Package services provides SQLite storage backend implementation.
package services

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"time"

	_ "github.com/mattn/go-sqlite3"
	"github.com/yhakami/bincrypt/src/models"
)

type SQLiteBackend struct {
	db *sql.DB
}

func NewSQLiteBackend(dbPath string) (*SQLiteBackend, error) {
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open SQLite database: %w", err)
	}

	// Configure SQLite for better performance and concurrency
	db.SetMaxOpenConns(1) // SQLite works best with a single writer
	db.SetMaxIdleConns(1)
	db.SetConnMaxLifetime(0)

	// Enable WAL mode for better concurrency
	if _, err := db.Exec("PRAGMA journal_mode=WAL"); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to enable WAL mode: %w", err)
	}

	// Enable foreign keys
	if _, err := db.Exec("PRAGMA foreign_keys=ON"); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to enable foreign keys: %w", err)
	}

	backend := &SQLiteBackend{db: db}

	// Initialize schema
	if err := backend.initSchema(); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to initialize schema: %w", err)
	}

	return backend, nil
}

func (s *SQLiteBackend) initSchema() error {
	schema := `
	-- Pastes table: stores paste content and metadata
	CREATE TABLE IF NOT EXISTS pastes (
		id TEXT PRIMARY KEY,
		content TEXT,
		references_id TEXT,
		content_hash TEXT NOT NULL,
		plaintext_hash TEXT,
		expires_at DATETIME NOT NULL,
		burn_after_read INTEGER NOT NULL DEFAULT 0,
		size_bytes INTEGER NOT NULL,
		is_plaintext INTEGER NOT NULL DEFAULT 0,
		created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
		FOREIGN KEY (references_id) REFERENCES pastes(id) ON DELETE SET NULL
	);

	-- Hash index: maps content hash to original paste ID for deduplication
	CREATE TABLE IF NOT EXISTS hash_index (
		content_hash TEXT PRIMARY KEY,
		paste_id TEXT NOT NULL,
		created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
		FOREIGN KEY (paste_id) REFERENCES pastes(id) ON DELETE CASCADE
	);

	-- Burned pastes: tracks burn-after-read pastes that have been viewed
	CREATE TABLE IF NOT EXISTS burned_pastes (
		paste_id TEXT PRIMARY KEY,
		burned_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
		FOREIGN KEY (paste_id) REFERENCES pastes(id) ON DELETE CASCADE
	);

	-- Banned hashes: block content by ciphertext or plaintext hash
	CREATE TABLE IF NOT EXISTS banned_hashes (
		hash TEXT PRIMARY KEY,
		hash_type TEXT NOT NULL CHECK (hash_type IN ('ciphertext', 'plaintext')),
		reason TEXT NOT NULL,
		banned_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
	);

	-- Indexes for performance
	CREATE INDEX IF NOT EXISTS idx_pastes_expires_at ON pastes(expires_at);
	CREATE INDEX IF NOT EXISTS idx_pastes_content_hash ON pastes(content_hash);
	CREATE INDEX IF NOT EXISTS idx_pastes_references_id ON pastes(references_id) WHERE references_id IS NOT NULL;
	CREATE INDEX IF NOT EXISTS idx_banned_hashes_type ON banned_hashes(hash_type);
	`

	_, err := s.db.Exec(schema)
	return err
}

func (s *SQLiteBackend) SavePaste(ctx context.Context, id string, ciphertext string, expirySeconds int, burnAfterRead bool, isPlaintext bool) (*models.SavePasteResult, error) {
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

	// Use serializable isolation to prevent deduplication race conditions
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{
		Isolation: sql.LevelSerializable,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	var existingID string
	err = tx.QueryRowContext(ctx, "SELECT paste_id FROM hash_index WHERE content_hash = ?", contentHash).Scan(&existingID)
	if err != nil && err != sql.ErrNoRows {
		return nil, fmt.Errorf("failed to check for existing content: %w", err)
	}

	if existingID != "" {
		_, err = tx.ExecContext(ctx, `
			INSERT INTO pastes (id, references_id, content_hash, plaintext_hash, expires_at, burn_after_read, size_bytes, is_plaintext, created_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			id, existingID, contentHash, sqliteNullString(plaintextHash), expiresAt.Format(time.RFC3339), boolToInt(burnAfterRead), len(ciphertext), boolToInt(isPlaintext), now.Format(time.RFC3339))
		if err != nil {
			return nil, fmt.Errorf("failed to create reference paste: %w", err)
		}

		if err := tx.Commit(); err != nil {
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

	_, err = tx.ExecContext(ctx, `
		INSERT INTO pastes (id, content, content_hash, plaintext_hash, expires_at, burn_after_read, size_bytes, is_plaintext, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		id, ciphertext, contentHash, sqliteNullString(plaintextHash), expiresAt.Format(time.RFC3339), boolToInt(burnAfterRead), len(ciphertext), boolToInt(isPlaintext), now.Format(time.RFC3339))
	if err != nil {
		return nil, fmt.Errorf("failed to save paste: %w", err)
	}

	_, err = tx.ExecContext(ctx, "INSERT OR IGNORE INTO hash_index (content_hash, paste_id) VALUES (?, ?)", contentHash, id)
	if err != nil {
		return nil, fmt.Errorf("failed to create hash index: %w", err)
	}

	if err := tx.Commit(); err != nil {
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

func (s *SQLiteBackend) GetPaste(ctx context.Context, id string) (string, *models.Paste, error) {
	// For burn-after-read pastes, we need atomicity to prevent race conditions.
	// Use a transaction with serializable isolation to ensure only one reader succeeds.
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{
		Isolation: sql.LevelSerializable,
	})
	if err != nil {
		return "", nil, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Check if already burned (atomic within transaction).
	// Burned pastes are treated as "gone" (returned as IsBurned=true with empty content).
	var isBurned int
	err = tx.QueryRowContext(ctx, "SELECT EXISTS(SELECT 1 FROM burned_pastes WHERE paste_id = ?)", id).Scan(&isBurned)
	if err != nil {
		return "", nil, fmt.Errorf("failed to check burn status: %w", err)
	}
	if isBurned == 1 {
		return "", &models.Paste{ID: id, IsBurned: true}, nil
	}

	// Fetch paste metadata with expiry check in SQL (matches PostgreSQL behavior).
	// For burn-after-read pastes, we claim the burn marker BEFORE reading the content.
	var referencesID sql.NullString
	var expiresAtStr string
	var burnAfterReadInt int
	var sizeBytes int
	var createdAtStr string

	err = tx.QueryRowContext(ctx, `
		SELECT references_id, expires_at, burn_after_read, size_bytes, created_at
		FROM pastes
		WHERE id = ? AND (datetime(expires_at) > datetime('now') OR expires_at IS NULL)`,
		id).Scan(&referencesID, &expiresAtStr, &burnAfterReadInt, &sizeBytes, &createdAtStr)

	if err == sql.ErrNoRows {
		return "", nil, ErrNotFound
	}
	if err != nil {
		return "", nil, fmt.Errorf("failed to get paste: %w", err)
	}

	expiresAt, err := time.Parse(time.RFC3339, expiresAtStr)
	if err != nil {
		return "", nil, fmt.Errorf("failed to parse expires_at: %w", err)
	}

	createdAt, err := time.Parse(time.RFC3339, createdAtStr)
	if err != nil {
		return "", nil, fmt.Errorf("failed to parse created_at: %w", err)
	}

	burnAfterRead := intToBool(burnAfterReadInt)

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
		result, err := tx.ExecContext(ctx, "INSERT OR IGNORE INTO burned_pastes (paste_id, burned_at) VALUES (?, datetime('now'))", id)
		if err != nil {
			return "", nil, fmt.Errorf("failed to mark paste as burned: %w", err)
		}
		affected, err := result.RowsAffected()
		if err != nil {
			return "", nil, fmt.Errorf("failed to read burn insert result: %w", err)
		}
		if affected == 0 {
			return "", &models.Paste{ID: id, IsBurned: true}, nil
		}
		paste.IsBurned = true
	}

	// Resolve content (handle references).
	var finalContent string
	contentID := id
	if referencesID.Valid && referencesID.String != "" {
		contentID = referencesID.String
	}

	var content sql.NullString
	err = tx.QueryRowContext(ctx, "SELECT content FROM pastes WHERE id = ?", contentID).Scan(&content)
	if err == sql.ErrNoRows {
		return "", nil, ErrNotFound
	}
	if err != nil {
		return "", nil, fmt.Errorf("failed to get paste content: %w", err)
	}
	if !content.Valid {
		return "", nil, ErrNotFound
	}
	finalContent = content.String

	// Commit transaction (makes burn marking permanent)
	if err := tx.Commit(); err != nil {
		return "", nil, fmt.Errorf("failed to commit transaction: %w", err)
	}

	return finalContent, paste, nil
}

func (s *SQLiteBackend) MarkBurned(ctx context.Context, id string) error {
	_, err := s.db.ExecContext(ctx, "INSERT OR IGNORE INTO burned_pastes (paste_id, burned_at) VALUES (?, datetime('now'))", id)
	if err != nil {
		return fmt.Errorf("failed to mark paste as burned: %w", err)
	}
	return nil
}

func (s *SQLiteBackend) DeletePaste(ctx context.Context, id string) error {
	_, err := s.db.ExecContext(ctx, "DELETE FROM pastes WHERE id = ?", id)
	if err != nil {
		return fmt.Errorf("failed to delete paste: %w", err)
	}

	// Burned pastes will be cleaned up by CASCADE

	return nil
}

func (s *SQLiteBackend) BanContentHash(ctx context.Context, contentHash string, reason string) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO banned_hashes (hash, reason, hash_type, banned_at)
		VALUES (?, ?, 'ciphertext', datetime('now'))
		ON CONFLICT (hash) DO UPDATE SET reason = ?, banned_at = datetime('now')`,
		contentHash, reason, reason)
	if err != nil {
		return fmt.Errorf("failed to ban content hash: %w", err)
	}
	return nil
}

func (s *SQLiteBackend) BanPlaintextHash(ctx context.Context, plaintextHash string, reason string) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO banned_hashes (hash, reason, hash_type, banned_at)
		VALUES (?, ?, 'plaintext', datetime('now'))
		ON CONFLICT (hash) DO UPDATE SET reason = ?, banned_at = datetime('now')`,
		plaintextHash, reason, reason)
	if err != nil {
		return fmt.Errorf("failed to ban plaintext hash: %w", err)
	}
	return nil
}

func (s *SQLiteBackend) Close() error {
	return s.db.Close()
}

// Ping checks if the database connection is alive
func (s *SQLiteBackend) Ping(ctx context.Context) error {
	return s.db.PingContext(ctx)
}

func (s *SQLiteBackend) isHashBanned(ctx context.Context, contentHash string) (bool, error) {
	var exists int
	err := s.db.QueryRowContext(ctx, "SELECT EXISTS(SELECT 1 FROM banned_hashes WHERE hash = ? AND hash_type = 'ciphertext')", contentHash).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("failed to check banned status: %w", err)
	}
	return exists == 1, nil
}

func (s *SQLiteBackend) isPlaintextHashBanned(ctx context.Context, plaintextHash string) (bool, error) {
	var exists int
	err := s.db.QueryRowContext(ctx, "SELECT EXISTS(SELECT 1 FROM banned_hashes WHERE hash = ? AND hash_type = 'plaintext')", plaintextHash).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("failed to check plaintext banned status: %w", err)
	}
	return exists == 1, nil
}

// Helper function to convert empty string to NULL for SQLite driver
// Note: SQLite driver expects interface{} with nil, unlike PostgreSQL which uses *string
func sqliteNullString(s string) interface{} {
	if s == "" {
		return nil
	}
	return s
}

// Helper function to convert bool to int for SQLite
func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

// Helper function to convert int to bool from SQLite
func intToBool(i int) bool {
	return i != 0
}
