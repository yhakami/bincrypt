package utils

import (
	"crypto/rand"
	"encoding/base64"
)

// GenerateID generates a secure random ID
func GenerateID() (string, error) {
	b := make([]byte, 12)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.URLEncoding.EncodeToString(b), nil
}