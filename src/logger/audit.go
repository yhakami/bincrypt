package logger

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"time"
)

// AuditEventType represents types of audit events
type AuditEventType string

const (
	// Paste operations
	AuditPasteCreated      AuditEventType = "PASTE_CREATED"
	AuditPasteViewed       AuditEventType = "PASTE_VIEWED"
	AuditPasteBurned       AuditEventType = "PASTE_BURNED"
	AuditPasteExpired      AuditEventType = "PASTE_EXPIRED"
	AuditPasteDeleted      AuditEventType = "PASTE_DELETED"
	AuditPasteDeduplicated AuditEventType = "PASTE_DEDUPLICATED"
	AuditHashBanned        AuditEventType = "HASH_BANNED"
	AuditBannedContent     AuditEventType = "BANNED_CONTENT_ATTEMPT"

	// Security events
	AuditRateLimitHit   AuditEventType = "RATE_LIMIT_HIT"
	AuditInvalidRequest AuditEventType = "INVALID_REQUEST"
	AuditCSPViolation   AuditEventType = "CSP_VIOLATION"
	AuditAuthFailure    AuditEventType = "AUTH_FAILURE"

	// Payment events
	AuditInvoiceCreated  AuditEventType = "INVOICE_CREATED"
	AuditPaymentReceived AuditEventType = "PAYMENT_RECEIVED"
	AuditWebhookReceived AuditEventType = "WEBHOOK_RECEIVED"
	AuditWebhookInvalid  AuditEventType = "WEBHOOK_INVALID"

	// System events
	AuditSystemStart  AuditEventType = "SYSTEM_START"
	AuditSystemStop   AuditEventType = "SYSTEM_STOP"
	AuditConfigChange AuditEventType = "CONFIG_CHANGE"
)

// AuditLogger provides security audit logging
type AuditLogger struct {
	logger Logger
}

// NewAuditLogger creates a new audit logger
func NewAuditLogger(logger Logger) *AuditLogger {
	return &AuditLogger{logger: logger}
}

// LogEvent logs an audit event
func (a *AuditLogger) LogEvent(ctx context.Context, event AuditEventType, fields Fields) {
	auditFields := Fields{
		"event_type": string(event),
		"timestamp":  time.Now().UTC().Format(time.RFC3339),
	}

	// Add all provided fields
	for k, v := range fields {
		auditFields[k] = v
	}

	// Extract context values
	if ctx != nil {
		if requestID, ok := ctx.Value(RequestIDKey).(string); ok {
			auditFields["request_id"] = requestID
		}
		if clientIP, ok := ctx.Value(ClientIPKey).(string); ok {
			auditFields["client_ip"] = clientIP
		}
		if userAgent, ok := ctx.Value(UserAgentKey).(string); ok {
			auditFields["user_agent"] = userAgent
		}
	}

	a.logger.WithContext(ctx).Audit(string(event), auditFields)
}

// LogPasteCreated logs paste creation
func (a *AuditLogger) LogPasteCreated(ctx context.Context, pasteID string, size int, expirySeconds int64, burnAfterRead bool, clientIP string) {
	// Hash the paste ID for privacy
	hashedID := hashPasteID(pasteID)

	a.LogEvent(ctx, AuditPasteCreated, Fields{
		"paste_id_hash":   hashedID,
		"size_bytes":      size,
		"expiry_seconds":  expirySeconds,
		"burn_after_read": burnAfterRead,
		"client_ip":       clientIP,
	})
}

// LogPasteDeduplicated logs when a paste is deduplicated
func (a *AuditLogger) LogPasteDeduplicated(ctx context.Context, pasteID string, referencesID string, contentHash string, clientIP string, savedBytes int) {
	hashedID := hashPasteID(pasteID)
	hashedRefID := hashPasteID(referencesID)

	a.LogEvent(ctx, AuditPasteDeduplicated, Fields{
		"paste_id_hash":       hashedID,
		"references_id_hash":  hashedRefID,
		"content_hash_prefix": contentHash[:16], // Log only prefix for privacy
		"client_ip":           clientIP,
		"saved_bytes":         savedBytes,
		"dedup_efficiency":    "100%", // Full deduplication
	})
}

// LogHashBanned logs when a content hash is banned
func (a *AuditLogger) LogHashBanned(ctx context.Context, contentHash string, reason string, adminID string) {
	a.LogEvent(ctx, AuditHashBanned, Fields{
		"content_hash_prefix": contentHash[:16], // Log only prefix
		"reason":              reason,
		"admin_id":            adminID,
		"timestamp":           time.Now().UTC(),
	})
}

// LogBannedContentAttempt logs when someone tries to upload banned content
func (a *AuditLogger) LogBannedContentAttempt(ctx context.Context, contentHash string, clientIP string) {
	a.LogEvent(ctx, AuditBannedContent, Fields{
		"content_hash_prefix": contentHash[:16], // Log only prefix
		"client_ip":           clientIP,
		"action":              "rejected",
		"timestamp":           time.Now().UTC(),
	})
}

// LogPasteViewed logs paste viewing
func (a *AuditLogger) LogPasteViewed(ctx context.Context, pasteID string, clientIP string, burnAfterRead bool) {
	hashedID := hashPasteID(pasteID)

	a.LogEvent(ctx, AuditPasteViewed, Fields{
		"paste_id_hash":   hashedID,
		"client_ip":       clientIP,
		"burn_after_read": burnAfterRead,
	})
}

// LogPasteBurned logs paste burning
func (a *AuditLogger) LogPasteBurned(ctx context.Context, pasteID string, clientIP string) {
	hashedID := hashPasteID(pasteID)

	a.LogEvent(ctx, AuditPasteBurned, Fields{
		"paste_id_hash": hashedID,
		"client_ip":     clientIP,
		"action":        "burned_after_read",
	})
}

// LogRateLimitHit logs rate limit violations
func (a *AuditLogger) LogRateLimitHit(ctx context.Context, clientIP string, endpoint string, limit int64, window string) {
	a.LogEvent(ctx, AuditRateLimitHit, Fields{
		"client_ip": clientIP,
		"endpoint":  endpoint,
		"limit":     limit,
		"window":    window,
		"action":    "blocked",
	})
}

// LogInvalidRequest logs invalid/malicious requests
func (a *AuditLogger) LogInvalidRequest(ctx context.Context, clientIP string, endpoint string, reason string, userAgent string) {
	a.LogEvent(ctx, AuditInvalidRequest, Fields{
		"client_ip":  clientIP,
		"endpoint":   endpoint,
		"reason":     reason,
		"user_agent": userAgent,
		"action":     "rejected",
	})
}

// LogCSPViolation logs Content Security Policy violations
func (a *AuditLogger) LogCSPViolation(ctx context.Context, violationReport map[string]interface{}, clientIP string) {
	a.LogEvent(ctx, AuditCSPViolation, Fields{
		"client_ip": clientIP,
		"violation": violationReport,
		"action":    "reported",
	})
}

// LogInvoiceCreated logs payment invoice creation
func (a *AuditLogger) LogInvoiceCreated(ctx context.Context, invoiceID string, amount float64, currency string, clientIP string) {
	a.LogEvent(ctx, AuditInvoiceCreated, Fields{
		"invoice_id": invoiceID,
		"amount":     amount,
		"currency":   currency,
		"client_ip":  clientIP,
	})
}

// LogPaymentReceived logs successful payment
func (a *AuditLogger) LogPaymentReceived(ctx context.Context, invoiceID string, pasteID string, amount float64, currency string) {
	hashedPasteID := hashPasteID(pasteID)

	a.LogEvent(ctx, AuditPaymentReceived, Fields{
		"invoice_id":    invoiceID,
		"paste_id_hash": hashedPasteID,
		"amount":        amount,
		"currency":      currency,
		"status":        "completed",
	})
}

// LogWebhookReceived logs webhook reception
func (a *AuditLogger) LogWebhookReceived(ctx context.Context, webhookType string, valid bool, clientIP string, signature string) {
	event := AuditWebhookReceived
	if !valid {
		event = AuditWebhookInvalid
	}

	a.LogEvent(ctx, event, Fields{
		"webhook_type":     webhookType,
		"signature_valid":  valid,
		"client_ip":        clientIP,
		"signature_prefix": truncateSignature(signature),
		"action":           determineWebhookAction(valid),
	})
}

// LogSystemStart logs system startup
func (a *AuditLogger) LogSystemStart(version string, config map[string]interface{}) {
	// Sanitize config to remove sensitive values
	sanitizedConfig := make(map[string]interface{})
	for k, v := range config {
		if isSensitiveConfigKey(k) {
			sanitizedConfig[k] = "<redacted>"
		} else {
			sanitizedConfig[k] = v
		}
	}

	a.LogEvent(context.Background(), AuditSystemStart, Fields{
		"version": version,
		"config":  sanitizedConfig,
	})
}

// Helper functions

// hashPasteID creates a privacy-preserving hash of paste ID
func hashPasteID(pasteID string) string {
	return HashPasteID(pasteID)
}

// HashPasteID returns a stable, privacy-preserving paste ID hash for logs.
func HashPasteID(pasteID string) string {
	hash := sha256.Sum256([]byte(pasteID))
	return hex.EncodeToString(hash[:8]) // First 8 bytes for brevity
}

// truncateSignature returns first 8 chars of signature for logging
func truncateSignature(signature string) string {
	if len(signature) > 8 {
		return signature[:8] + "..."
	}
	return signature
}

// determineWebhookAction returns action based on validity
func determineWebhookAction(valid bool) string {
	if valid {
		return "processed"
	}
	return "rejected"
}

// isSensitiveConfigKey checks if a config key contains sensitive data
func isSensitiveConfigKey(key string) bool {
	sensitive := []string{
		"key", "secret", "password", "token", "apikey", "api_key",
		"private", "credential", "auth", "btcpay_apikey", "webhook_secret",
	}

	lowerKey := strings.ToLower(key)
	for _, s := range sensitive {
		if strings.Contains(lowerKey, s) {
			return true
		}
	}
	return false
}

// Global audit logger instance
var globalAuditLogger *AuditLogger

// InitAuditLogger initializes the global audit logger
func InitAuditLogger(logger Logger) {
	globalAuditLogger = NewAuditLogger(logger)
}

// GetAuditLogger returns the global audit logger
func GetAuditLogger() *AuditLogger {
	if globalAuditLogger == nil {
		InitAuditLogger(Default())
	}
	return globalAuditLogger
}
