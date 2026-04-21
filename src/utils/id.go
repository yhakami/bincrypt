// Package utils provides utility functions for BinCrypt.
//
// This package includes:
//   - ID generation with cryptographic randomness
//   - Client IP extraction for rate limiting and audit logs
//
// All utilities are designed to be secure by default and work correctly
// in cloud environments like Google Cloud Run.
package utils

import (
	"crypto/rand"
	"encoding/base64"
)

// GenerateID generates a cryptographically secure random ID with 256 bits of entropy
// This prevents enumeration attacks by making IDs unpredictable
func GenerateID() (string, error) {
	// Use 32 bytes (256 bits) of entropy for security
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	// Use URL-safe base64 encoding without padding
	// This produces a 43-character string that is safe for URLs
	return base64.RawURLEncoding.EncodeToString(b), nil
}
