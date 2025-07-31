package handlers

import (
	"encoding/json"
	"net/http"
	"os"
)

// ConfigHandler handles configuration endpoints
type ConfigHandler struct{}

// NewConfigHandler creates a new config handler
func NewConfigHandler() *ConfigHandler {
	return &ConfigHandler{}
}

// GetClientConfig returns client-side configuration
func (h *ConfigHandler) GetClientConfig(w http.ResponseWriter, r *http.Request) {
	// Get Coinzilla zone IDs from environment
	coinzillaZones := map[string]string{
		"banner728x90":    os.Getenv("COINZILLA_BANNER_LEADERBOARD"),
		"sidebar300x600":  os.Getenv("COINZILLA_SIDEBAR_SKYSCRAPER"),
		"rectangle300x250": os.Getenv("COINZILLA_RECTANGLE_MEDIUM"),
		"mobile320x50":    os.Getenv("COINZILLA_MOBILE_BANNER"),
	}
	
	// Check if any zones are configured
	hasZones := false
	for _, zoneID := range coinzillaZones {
		if zoneID != "" {
			hasZones = true
			break
		}
	}
	
	config := map[string]interface{}{}
	
	// Only include zones if at least one is configured
	if hasZones {
		config["coinzillaZones"] = coinzillaZones
	}
	
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(config)
}