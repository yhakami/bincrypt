package models

import "time"

// Paste represents paste metadata
type Paste struct {
	ID            string    `json:"id"`
	ExpiresAt     time.Time `json:"expires_at"`
	BurnAfterRead bool      `json:"burn_after_read"`
	SizeBytes     int       `json:"size_bytes"`
	CreatedAt     time.Time `json:"created_at"`
	IsBurned      bool      `json:"is_burned"`
}

// CreatePasteRequest represents a paste creation request
type CreatePasteRequest struct {
	Ciphertext    string `json:"ciphertext"`
	ExpirySeconds int    `json:"expiry_seconds"`
	BurnAfterRead bool   `json:"burn_after_read"`
}

// PasteResponse represents the API response for paste creation
type PasteResponse struct {
	ID        string    `json:"id"`
	ExpiresAt time.Time `json:"expires_at"`
}