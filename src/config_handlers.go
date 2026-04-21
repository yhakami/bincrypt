package main

import (
	"encoding/json"
	"net/http"

	configpkg "github.com/yhakami/bincrypt/src/config"
)

func (s *Server) GetClientConfig(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	firebaseConfig := map[string]string{
		"apiKey":            configpkg.GetSecretOrDefault(ctx, "FIREBASE_API_KEY", ""),
		"authDomain":        configpkg.GetSecretOrDefault(ctx, "FIREBASE_AUTH_DOMAIN", ""),
		"projectId":         configpkg.GetSecretOrDefault(ctx, "FIREBASE_PROJECT_ID", ""),
		"storageBucket":     configpkg.GetSecretOrDefault(ctx, "FIREBASE_STORAGE_BUCKET", ""),
		"messagingSenderId": configpkg.GetSecretOrDefault(ctx, "FIREBASE_MESSAGING_SENDER_ID", ""),
		"appId":             configpkg.GetSecretOrDefault(ctx, "FIREBASE_APP_ID", ""),
	}
	turnstileSiteKey := configpkg.GetSecretOrDefault(ctx, "TURNSTILE_SITE_KEY", "")
	devBypass := configpkg.GetSecretOrDefault(ctx, "TURNSTILE_DEV_BYPASS", "false")
	configData := map[string]interface{}{
		"limits": map[string]interface{}{
			"maxSizeBytesEncrypted":   MaxCiphertextSizeEncrypted,
			"maxSizeBytesUnencrypted": MaxCiphertextSizeUnencrypted,
			"maxSizeKBEncrypted":      MaxCiphertextSizeEncrypted / 1024,
			"maxSizeKBUnencrypted":    MaxCiphertextSizeUnencrypted / 1024,
		},
	}
	if firebaseConfig["apiKey"] != "" ||
		firebaseConfig["authDomain"] != "" ||
		firebaseConfig["projectId"] != "" ||
		firebaseConfig["storageBucket"] != "" ||
		firebaseConfig["messagingSenderId"] != "" ||
		firebaseConfig["appId"] != "" {
		configData["firebase"] = firebaseConfig
	}
	if turnstileSiteKey != "" && devBypass != "true" {
		configData["turnstile"] = map[string]string{"siteKey": turnstileSiteKey}
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(configData)
}
