package handlers

import (
	"embed"
	"encoding/json"
	"net/http"
	"time"

	"bincrypt/src/models"
	"bincrypt/src/services"
	"bincrypt/src/utils"
	"cloud.google.com/go/storage"
	"github.com/gorilla/mux"
)

// PasteHandler handles paste operations
type PasteHandler struct {
	storage *services.StorageService
}

// NewPasteHandler creates a new paste handler
func NewPasteHandler(storage *services.StorageService) *PasteHandler {
	return &PasteHandler{storage: storage}
}

// CreatePaste handles paste creation
func (h *PasteHandler) CreatePaste(w http.ResponseWriter, r *http.Request) {
	// Limit request body size
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20) // 1MB max
	
	var req models.CreatePasteRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}
	
	// Validate expiry (max 7 days for free tier)
	if req.ExpirySeconds <= 0 || req.ExpirySeconds > 604800 {
		req.ExpirySeconds = 604800
	}
	
	// Validate size (max 512KB for free tier)
	if len(req.Ciphertext) > 524288 {
		http.Error(w, "Paste too large (max 512KB for free tier)", http.StatusBadRequest)
		return
	}
	
	// Generate ID
	id, err := utils.GenerateID()
	if err != nil {
		http.Error(w, "Failed to generate ID", http.StatusInternalServerError)
		return
	}
	
	// Save paste
	ctx := r.Context()
	paste, err := h.storage.SavePaste(ctx, id, req.Ciphertext, req.ExpirySeconds, req.BurnAfterRead)
	if err != nil {
		http.Error(w, "Failed to save paste", http.StatusInternalServerError)
		return
	}
	
	// Return response
	response := models.PasteResponse{
		ID:        paste.ID,
		ExpiresAt: paste.ExpiresAt,
	}
	
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// GetPasteAPI handles paste data retrieval via API (returns JSON)
func (h *PasteHandler) GetPasteAPI(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id := vars["id"]
	
	ctx := r.Context()
	
	// Get paste
	ciphertext, paste, err := h.storage.GetPaste(ctx, id)
	if err == storage.ErrObjectNotExist {
		http.Error(w, "Paste not found", http.StatusNotFound)
		return
	} else if err != nil {
		http.Error(w, "Failed to retrieve paste", http.StatusInternalServerError)
		return
	}
	
	// Handle burn-after-read
	if paste.BurnAfterRead {
		// Mark as burned
		_ = h.storage.MarkBurned(ctx, id)
		
		// Delete after response
		defer func() {
			go func() {
				// Small delay to ensure response is sent
				time.Sleep(100 * time.Millisecond)
				_ = h.storage.DeletePaste(ctx, id)
			}()
		}()
	}
	
	// Return paste data
	response := map[string]interface{}{
		"id":            paste.ID,
		"ciphertext":    ciphertext,
		"burnAfterRead": paste.BurnAfterRead,
		"expiresAt":     paste.ExpiresAt,
	}
	
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// ViewPaste handles paste viewing page (serves HTML)
func (h *PasteHandler) ViewPaste(w http.ResponseWriter, r *http.Request, staticFiles embed.FS) {
	// Serve the viewer HTML page
	content, err := staticFiles.ReadFile("static/viewer.html")
	if err != nil {
		http.Error(w, "Viewer page not found", http.StatusInternalServerError)
		return
	}
	
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Content-Security-Policy", "default-src 'self' https://cdn.jsdelivr.net https://unpkg.com https://www.gstatic.com; script-src 'self' 'unsafe-inline' 'unsafe-eval' https://cdn.jsdelivr.net https://unpkg.com https://www.gstatic.com; style-src 'self' 'unsafe-inline' https://cdn.jsdelivr.net")
	w.Header().Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
	w.Write(content)
}