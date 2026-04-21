// Package models defines data structures and validation logic for BinCrypt.
//
// This package contains all domain models including:
//   - Paste: Core paste metadata and content references
//   - Invoice: Payment-related models for BTCPay integration
//   - Validation: Request validation and error structures
//   - RateLimit: Rate limiting configuration models
//
// All models include validation methods to ensure data integrity before
// storage or transmission. The Validate() methods return structured errors
// that can be directly serialized to API responses.
package models

import (
	"encoding/json"
	"fmt"
	"regexp"
	"time"
)

// Paste represents paste metadata
type Paste struct {
	ID            string    `json:"id"`
	ExpiresAt     time.Time `json:"expires_at"`
	BurnAfterRead bool      `json:"burn_after_read"`
	SizeBytes     int       `json:"size_bytes"`
	CreatedAt     time.Time `json:"created_at"`
	IsBurned      bool      `json:"is_burned"`
}

// SavePasteResult contains the result of saving a paste, including deduplication info
type SavePasteResult struct {
	Paste           *Paste
	WasDeduplicated bool
	ReferencesID    string
	ContentHash     string
}

// CreatePasteRequest represents a paste creation request
type CreatePasteRequest struct {
	Ciphertext     string      `json:"ciphertext,omitempty"`
	Plaintext      string      `json:"plaintext,omitempty"`
	Metadata       interface{} `json:"metadata,omitempty"`
	ExpirySeconds  int         `json:"expiry_seconds"`
	BurnAfterRead  bool        `json:"burn_after_read"`
	TurnstileToken string      `json:"turnstile_token"`
}

// PasteResponse represents the API response for paste creation
type PasteResponse struct {
	ID        string    `json:"id"`
	ExpiresAt time.Time `json:"expires_at"`
}

// Validate validates the Paste metadata
func (p *Paste) Validate() error {
	// Validate ID - must be 43-character base64url
	idPattern := regexp.MustCompile(`^[A-Za-z0-9_-]{43}$`)
	if !idPattern.MatchString(p.ID) {
		return fmt.Errorf("invalid paste ID format")
	}

	// Validate metadata size (1KB limit)
	if data, err := json.Marshal(p); err != nil {
		return fmt.Errorf("invalid paste metadata: %w", err)
	} else if len(data) > 1024 {
		return fmt.Errorf("paste metadata exceeds 1KB limit")
	}

	return nil
}
