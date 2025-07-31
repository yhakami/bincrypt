package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"bincrypt/src/models"
	"cloud.google.com/go/storage"
	"google.golang.org/api/iterator"
)

// HealthHandler handles health and metrics endpoints
type HealthHandler struct {
	bucket *storage.BucketHandle
}

// NewHealthHandler creates a new health handler
func NewHealthHandler(bucket *storage.BucketHandle) *HealthHandler {
	return &HealthHandler{bucket: bucket}
}

// HealthCheck handles health check requests
func (h *HealthHandler) HealthCheck(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	
	health := map[string]interface{}{
		"status":    "healthy",
		"timestamp": time.Now().UTC().Format(time.RFC3339),
		"services":  map[string]string{},
	}
	
	// Check storage health
	if _, err := h.bucket.Attrs(ctx); err != nil {
		health["status"] = "unhealthy"
		health["services"].(map[string]string)["storage"] = fmt.Sprintf("unhealthy: %v", err)
	} else {
		health["services"].(map[string]string)["storage"] = "healthy"
	}
	
	w.Header().Set("Content-Type", "application/json")
	if health["status"] == "healthy" {
		w.WriteHeader(http.StatusOK)
	} else {
		w.WriteHeader(http.StatusServiceUnavailable)
	}
	json.NewEncoder(w).Encode(health)
}

// Metrics handles metrics requests
func (h *HealthHandler) Metrics(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	
	metrics := map[string]interface{}{
		"timestamp": time.Now().UTC().Format(time.RFC3339),
		"pastes": map[string]interface{}{
			"total":  0,
			"burned": 0,
			"active": 0,
		},
	}
	
	// Count pastes
	it := h.bucket.Objects(ctx, &storage.Query{Prefix: "pastes/", Delimiter: ""})
	for {
		attrs, err := it.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			break
		}
		
		if strings.HasSuffix(attrs.Name, ".json") && !strings.Contains(attrs.Name, "/.") {
			// Try to read metadata
			obj := h.bucket.Object(attrs.Name)
			reader, err := obj.NewReader(ctx)
			if err != nil {
				continue
			}
			
			var paste models.Paste
			if err := json.NewDecoder(reader).Decode(&paste); err == nil {
				metrics["pastes"].(map[string]interface{})["total"] = metrics["pastes"].(map[string]interface{})["total"].(int) + 1
				if paste.IsBurned {
					metrics["pastes"].(map[string]interface{})["burned"] = metrics["pastes"].(map[string]interface{})["burned"].(int) + 1
				} else {
					metrics["pastes"].(map[string]interface{})["active"] = metrics["pastes"].(map[string]interface{})["active"].(int) + 1
				}
			}
			reader.Close()
		}
	}
	
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(metrics)
}