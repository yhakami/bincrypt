package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"time"

	"github.com/gorilla/mux"
	"github.com/yhakami/bincrypt/src/logger"
	"github.com/yhakami/bincrypt/src/models"
	"github.com/yhakami/bincrypt/src/services"
	"github.com/yhakami/bincrypt/src/utils"
)

// CreatePaste handles paste creation with automatic deduplication and ban checking.
// Free tier is limited to 7-day max expiry.
func (s *Server) CreatePaste(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	log := logger.WithContext(ctx)
	auditLog := logger.GetAuditLogger()
	clientIP := utils.GetClientIP(r, s.proxyConfig)
	logger.GetGlobalMetrics().IncrementPasteCreated()

	var req models.CreatePasteRequest
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&req); err != nil {
		log.Warn("invalid_paste_request", logger.Fields{"error": err.Error(), "client_ip": clientIP})
		auditLog.LogInvalidRequest(ctx, clientIP, "/api/paste", "invalid JSON", r.Header.Get("User-Agent"))
		WriteSimpleError(w, "Invalid JSON request", http.StatusBadRequest)
		return
	}

	if errs := validateCreatePasteRequest(&req); len(errs) > 0 {
		log.Warn("paste_validation_failed", logger.Fields{"errors": errs, "client_ip": clientIP})
		auditLog.LogInvalidRequest(ctx, clientIP, "/api/paste", "validation failed", r.Header.Get("User-Agent"))
		WriteValidationError(w, errs)
		return
	}

	if req.ExpirySeconds > maxFreeTierExpirySeconds {
		log.Debug("adjusting_expiry", logger.Fields{"requested": req.ExpirySeconds, "adjusted": maxFreeTierExpirySeconds})
		req.ExpirySeconds = maxFreeTierExpirySeconds
	}

	var contentToStore string
	var isPlaintext bool
	var contentSize int

	if req.Ciphertext != "" {
		isPlaintext = false
		contentToStore = req.Ciphertext
		decoded, err := base64.StdEncoding.DecodeString(req.Ciphertext)
		if err != nil {
			log.Warn("invalid_base64_ciphertext", logger.Fields{"error": err.Error(), "client_ip": clientIP})
			auditLog.LogInvalidRequest(ctx, clientIP, "/api/paste", "invalid base64", r.Header.Get("User-Agent"))
			WriteSimpleError(w, "Invalid base64 encoding", http.StatusBadRequest)
			return
		}
		contentSize = len(decoded)
		if contentSize > MaxCiphertextSizeEncrypted {
			log.Warn("encrypted_paste_too_large", logger.Fields{"size": contentSize, "client_ip": clientIP})
			auditLog.LogInvalidRequest(ctx, clientIP, "/api/paste", "encrypted paste too large", r.Header.Get("User-Agent"))
			WriteSimpleError(w, "Encrypted paste too large (max 384KB)", http.StatusBadRequest)
			return
		}
	} else {
		isPlaintext = true
		contentSize = len(req.Plaintext)
		if contentSize > MaxCiphertextSizeUnencrypted {
			log.Warn("plaintext_paste_too_large", logger.Fields{"size": contentSize, "client_ip": clientIP})
			auditLog.LogInvalidRequest(ctx, clientIP, "/api/paste", "plaintext paste too large", r.Header.Get("User-Agent"))
			WriteSimpleError(w, "Plaintext paste too large (max 512KB)", http.StatusBadRequest)
			return
		}
		if req.Metadata != nil {
			if _, err := json.Marshal(req.Metadata); err != nil {
				log.Warn("invalid_metadata", logger.Fields{"error": err.Error(), "client_ip": clientIP})
				WriteSimpleError(w, "Invalid metadata", http.StatusBadRequest)
				return
			}
		}
		plaintextData := map[string]interface{}{
			"content":  req.Plaintext,
			"metadata": req.Metadata,
			"version":  "2.0",
		}
		plaintextJSON, err := json.Marshal(plaintextData)
		if err != nil {
			log.Error("plaintext_json_marshal_failed", logger.Fields{"error": err.Error()})
			WriteSimpleError(w, "Failed to prepare content", http.StatusInternalServerError)
			return
		}
		contentToStore = base64.StdEncoding.EncodeToString(plaintextJSON)
	}

	id, err := utils.GenerateID()
	if err != nil {
		log.Error("id_generation_failed", logger.Fields{"error": err.Error()})
		WriteSimpleError(w, "Failed to generate ID", http.StatusInternalServerError)
		return
	}

	saveResult, err := s.storageService.SavePaste(ctx, id, contentToStore, req.ExpirySeconds, req.BurnAfterRead, isPlaintext)
	if err != nil {
		if err == services.ErrBannedContent {
			log.Warn("banned_content_attempt", logger.Fields{"client_ip": clientIP})
			auditLog.LogInvalidRequest(ctx, clientIP, "/api/paste", "banned content", r.Header.Get("User-Agent"))
			WriteSimpleError(w, "Content not permitted", http.StatusForbidden)
			return
		}
		log.Error("paste_save_failed", logger.Fields{"error": err.Error(), "paste_id_hash": logger.HashPasteID(id)})
		WriteSimpleError(w, "Failed to save paste", http.StatusInternalServerError)
		return
	}

	if saveResult.WasDeduplicated {
		log.Info("paste_deduplicated", logger.Fields{
			"paste_id_hash":   logger.HashPasteID(id),
			"references_hash": logger.HashPasteID(saveResult.ReferencesID),
			"size":            contentSize,
			"is_plaintext":    isPlaintext,
			"expiry_seconds":  req.ExpirySeconds,
			"burn_after_read": req.BurnAfterRead,
		})
		auditLog.LogPasteDeduplicated(ctx, id, saveResult.ReferencesID, saveResult.ContentHash, clientIP, contentSize)
	} else {
		log.Info("paste_created", logger.Fields{
			"paste_id_hash":   logger.HashPasteID(id),
			"size":            contentSize,
			"is_plaintext":    isPlaintext,
			"expiry_seconds":  req.ExpirySeconds,
			"burn_after_read": req.BurnAfterRead,
		})
		auditLog.LogPasteCreated(ctx, id, contentSize, int64(req.ExpirySeconds), req.BurnAfterRead, clientIP)
	}

	if err := saveResult.Paste.Validate(); err != nil {
		log.Error("paste_metadata_validation_failed", logger.Fields{"error": err.Error(), "paste_id_hash": logger.HashPasteID(id)})
		WriteSimpleError(w, "Internal validation error", http.StatusInternalServerError)
		return
	}

	response := models.PasteResponse{ID: saveResult.Paste.ID, ExpiresAt: saveResult.Paste.ExpiresAt}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store, no-cache, must-revalidate")
	_ = json.NewEncoder(w).Encode(response)
}

// GetPasteAPI retrieves paste data by ID. For burn-after-read pastes,
// marks as burned atomically before serving to prevent races.
func (s *Server) GetPasteAPI(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	log := logger.WithContext(ctx)
	auditLog := logger.GetAuditLogger()
	clientIP := utils.GetClientIP(r, s.proxyConfig)

	vars := mux.Vars(r)
	id := SanitizeString(vars["id"])
	if err := ValidatePasteID(id); err != nil {
		log.Warn("invalid_paste_id", logger.Fields{"paste_id_hash": logger.HashPasteID(id), "id_length": len(id), "error": err.Error(), "client_ip": clientIP})
		auditLog.LogInvalidRequest(ctx, clientIP, "/api/paste/{id}", "invalid paste ID", r.Header.Get("User-Agent"))
		WriteSimpleError(w, err.Error(), http.StatusBadRequest)
		return
	}

	ciphertext, paste, err := s.storageService.GetPaste(ctx, id)
	if err == services.ErrNotFound {
		log.Info("paste_not_found", logger.Fields{"paste_id_hash": logger.HashPasteID(id), "client_ip": clientIP})
		WriteSimpleError(w, "Paste not found", http.StatusNotFound)
		return
	} else if err != nil {
		log.Error("paste_retrieval_failed", logger.Fields{"paste_id_hash": logger.HashPasteID(id), "error": err.Error()})
		WriteSimpleError(w, "Failed to retrieve paste", http.StatusInternalServerError)
		return
	}

	if err := ValidateBase64(ciphertext, "stored ciphertext"); err != nil {
		WriteSimpleError(w, "Paste data corrupted", http.StatusInternalServerError)
		return
	}

	if err := paste.Validate(); err != nil {
		log.Error("invalid_paste_metadata", logger.Fields{"paste_id_hash": logger.HashPasteID(id), "error": err.Error()})
		WriteSimpleError(w, "Invalid paste metadata", http.StatusInternalServerError)
		return
	}

	if time.Now().After(paste.ExpiresAt) {
		log.Info("paste_expired_accessed", logger.Fields{"paste_id_hash": logger.HashPasteID(id), "expired_at": paste.ExpiresAt, "client_ip": clientIP})
		_ = s.storageService.DeletePaste(ctx, id)
		WriteSimpleError(w, "Paste has expired", http.StatusGone)
		return
	}

	if paste.IsBurned {
		// Paste was already burned or was just atomically burned in GetPaste().
		if paste.BurnAfterRead {
			logger.GetGlobalMetrics().IncrementPasteBurned()
			log.Info("paste_burned", logger.Fields{"paste_id_hash": logger.HashPasteID(id), "client_ip": clientIP})
			auditLog.LogPasteBurned(ctx, id, clientIP)

			// Schedule async cleanup of paste content via deletion queue.
			if err := s.deletionQueue.QueueDeletion(id); err != nil {
				log.Warn("deletion_queue_failed", logger.Fields{"paste_id_hash": logger.HashPasteID(id), "error": err.Error()})
			}
		} else {
			log.Warn("burned_paste_accessed", logger.Fields{"paste_id_hash": logger.HashPasteID(id), "client_ip": clientIP})
			WriteSimpleError(w, "Paste has been burned", http.StatusGone)
			return
		}
	}

	logger.GetGlobalMetrics().IncrementPasteViewed()
	log.Info("paste_viewed", logger.Fields{"paste_id_hash": logger.HashPasteID(id), "burn_after_read": paste.BurnAfterRead, "expires_at": paste.ExpiresAt})
	auditLog.LogPasteViewed(ctx, id, clientIP, paste.BurnAfterRead)

	response := map[string]interface{}{
		"id":              paste.ID,
		"ciphertext":      ciphertext,
		"burn_after_read": paste.BurnAfterRead,
		"expires_at":      paste.ExpiresAt,
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store, no-cache, must-revalidate")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	_ = json.NewEncoder(w).Encode(response)
}

func (s *Server) ViewPaste(w http.ResponseWriter, r *http.Request) {
	content, err := s.config.StaticFiles.ReadFile("static/viewer.html")
	if err != nil {
		http.Error(w, "Viewer page not found", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write(content)
}

func (s *Server) HealthCheck(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	storageStatus := "healthy"
	if err := s.storageService.Ping(ctx); err != nil {
		storageStatus = "unhealthy"
	}

	overallStatus := "healthy"
	if storageStatus != "healthy" {
		overallStatus = "degraded"
	}

	health := map[string]interface{}{
		"status":    overallStatus,
		"timestamp": time.Now().UTC().Format(time.RFC3339),
		"services": map[string]string{
			"storage": storageStatus,
		},
	}

	w.Header().Set("Content-Type", "application/json")
	if overallStatus != "healthy" {
		w.WriteHeader(http.StatusServiceUnavailable)
	} else {
		w.WriteHeader(http.StatusOK)
	}
	_ = json.NewEncoder(w).Encode(health)
}
