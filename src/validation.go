package main

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"strings"

	"github.com/yhakami/bincrypt/src/models"
)

const (
	MaxCiphertextSizeEncrypted   = 393216
	MaxCiphertextSizeUnencrypted = 524288
	MaxCiphertextSize            = 10 * 1024 * 1024
	MaxBase64Size                = (MaxCiphertextSize * 4) / 3
	MinCiphertextSize            = 1
	MaxExpirySeconds             = 30 * 24 * 60 * 60
	MinExpirySeconds             = 60
	maxFreeTierExpirySeconds     = 7 * 24 * 60 * 60
)

var idRegex = regexp.MustCompile(`^[A-Za-z0-9_-]{43}$`)

func ValidateBase64(ciphertext string, fieldName string) error {
	if ciphertext == "" {
		return fmt.Errorf("%s is required", fieldName)
	}
	if len(ciphertext) > MaxBase64Size {
		return fmt.Errorf("%s exceeds maximum size of 10MB", fieldName)
	}
	decoded, err := base64.StdEncoding.DecodeString(ciphertext)
	if err != nil {
		if decoded, err = base64.URLEncoding.DecodeString(ciphertext); err != nil {
			if decoded, err = base64.RawStdEncoding.DecodeString(ciphertext); err != nil {
				if decoded, err = base64.RawURLEncoding.DecodeString(ciphertext); err != nil {
					return fmt.Errorf("%s contains invalid base64 encoding", fieldName)
				}
			}
		}
	}
	if len(decoded) < MinCiphertextSize {
		return fmt.Errorf("%s is too small", fieldName)
	}
	if len(decoded) > MaxCiphertextSize {
		return fmt.Errorf("%s exceeds maximum decoded size of 10MB", fieldName)
	}
	return nil
}

func ValidateExpirySeconds(expiry int) error {
	if expiry < MinExpirySeconds {
		return fmt.Errorf("expiry must be at least %d seconds", MinExpirySeconds)
	}
	if expiry > MaxExpirySeconds {
		return fmt.Errorf("expiry cannot exceed %d seconds (30 days)", MaxExpirySeconds)
	}
	return nil
}

func ValidatePasteID(id string) error {
	if id == "" {
		return fmt.Errorf("paste ID is required")
	}
	if !idRegex.MatchString(id) {
		return fmt.Errorf("invalid paste ID format")
	}
	return nil
}

func SanitizeString(s string) string {
	s = strings.Map(func(r rune) rune {
		if r < 32 && r != '\t' && r != '\n' && r != '\r' {
			return -1
		}
		return r
	}, s)
	return strings.TrimSpace(s)
}

func WriteValidationError(w http.ResponseWriter, errors []models.ValidationError) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusBadRequest)
	_ = json.NewEncoder(w).Encode(models.ValidationErrors{Errors: errors})
}

func WriteSimpleError(w http.ResponseWriter, message string, statusCode int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": message})
}

func validateCreatePasteRequest(r *models.CreatePasteRequest) []models.ValidationError {
	var errs []models.ValidationError
	if r.Ciphertext == "" && r.Plaintext == "" {
		errs = append(errs, models.ValidationError{Field: "content", Message: "either ciphertext or plaintext is required"})
		return errs
	}
	if r.Ciphertext != "" && r.Plaintext != "" {
		errs = append(errs, models.ValidationError{Field: "content", Message: "cannot provide both ciphertext and plaintext"})
	}
	if r.Ciphertext != "" {
		if err := ValidateBase64(r.Ciphertext, "ciphertext"); err != nil {
			errs = append(errs, models.ValidationError{Field: "ciphertext", Message: err.Error()})
		}
	}
	if r.Plaintext != "" {
		if len(r.Plaintext) > MaxCiphertextSizeUnencrypted {
			errs = append(errs, models.ValidationError{Field: "plaintext", Message: "plaintext exceeds 512KB limit"})
		}
	}
	if err := ValidateExpirySeconds(r.ExpirySeconds); err != nil {
		errs = append(errs, models.ValidationError{Field: "expiry_seconds", Message: err.Error()})
	}
	return errs
}

func validateCreateInvoiceRequest(r *models.CreateInvoiceRequest) []models.ValidationError {
	var errs []models.ValidationError

	if r.Amount <= 0 {
		errs = append(errs, models.ValidationError{Field: "amount", Message: "amount must be greater than zero"})
	}
	if r.Amount > 1000000 {
		errs = append(errs, models.ValidationError{Field: "amount", Message: "amount exceeds maximum supported value"})
	}

	if r.Currency == "" {
		errs = append(errs, models.ValidationError{Field: "currency", Message: "currency is required"})
	} else if len(r.Currency) != 3 {
		errs = append(errs, models.ValidationError{Field: "currency", Message: "currency must be a 3-letter ISO code"})
	}

	if r.ExpirationMinutes < minInvoiceExpirationMinutes || r.ExpirationMinutes > maxInvoiceExpirationMinutes {
		errs = append(errs, models.ValidationError{
			Field:   "expiration_minutes",
			Message: fmt.Sprintf("expiration_minutes must be between %d and %d", minInvoiceExpirationMinutes, maxInvoiceExpirationMinutes),
		})
	}

	if r.Metadata != nil {
		metadataBytes, err := json.Marshal(r.Metadata)
		if err != nil {
			errs = append(errs, models.ValidationError{Field: "metadata", Message: "metadata must be JSON serializable"})
		} else if len(metadataBytes) > maxInvoiceMetadataBytes {
			errs = append(errs, models.ValidationError{Field: "metadata", Message: "metadata exceeds maximum size"})
		}
	}

	return errs
}
