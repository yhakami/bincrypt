package logger

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"net/http"
	"runtime"
	"sync"
	"time"

	"github.com/yhakami/bincrypt/src/config"
)

// MetricsCollector collects application metrics
type MetricsCollector struct {
	mu            sync.RWMutex
	requestCounts map[string]int64
	errorCounts   map[string]int64
	pastesCreated int64
	pastesViewed  int64
	pastesBurned  int64
	startTime     time.Time
}

// NewMetricsCollector creates a new metrics collector
func NewMetricsCollector() *MetricsCollector {
	return &MetricsCollector{
		requestCounts: make(map[string]int64),
		errorCounts:   make(map[string]int64),
		startTime:     time.Now(),
	}
}

// IncrementRequest increments request count for an endpoint
func (m *MetricsCollector) IncrementRequest(endpoint string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.requestCounts[endpoint]++
}

// IncrementError increments error count for an endpoint
func (m *MetricsCollector) IncrementError(endpoint string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.errorCounts[endpoint]++
}

// IncrementPasteCreated increments paste creation counter
func (m *MetricsCollector) IncrementPasteCreated() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.pastesCreated++
}

// IncrementPasteViewed increments paste view counter
func (m *MetricsCollector) IncrementPasteViewed() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.pastesViewed++
}

// IncrementPasteBurned increments paste burn counter
func (m *MetricsCollector) IncrementPasteBurned() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.pastesBurned++
}

// GetMetrics returns current metrics
func (m *MetricsCollector) GetMetrics() map[string]interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// Calculate uptime
	uptime := time.Since(m.startTime)

	// Get memory stats
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)

	// Copy maps to avoid holding lock during JSON encoding
	requestCounts := make(map[string]int64)
	for k, v := range m.requestCounts {
		requestCounts[k] = v
	}

	errorCounts := make(map[string]int64)
	for k, v := range m.errorCounts {
		errorCounts[k] = v
	}

	return map[string]interface{}{
		"uptime_seconds":  uptime.Seconds(),
		"uptime_human":    uptime.String(),
		"requests":        requestCounts,
		"errors":          errorCounts,
		"pastes_created":  m.pastesCreated,
		"pastes_viewed":   m.pastesViewed,
		"pastes_burned":   m.pastesBurned,
		"memory_alloc_mb": float64(memStats.Alloc) / 1024 / 1024,
		"memory_sys_mb":   float64(memStats.Sys) / 1024 / 1024,
		"goroutines":      runtime.NumGoroutine(),
		"gc_runs":         memStats.NumGC,
	}
}

// Global metrics collector
var globalMetrics = NewMetricsCollector()

// GetGlobalMetrics returns the global metrics collector
func GetGlobalMetrics() *MetricsCollector {
	return globalMetrics
}

// MetricsHandler returns an HTTP handler for metrics endpoint
func MetricsHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		log := WithContext(ctx)

		// Require authorization whenever metrics are enabled.
		authHeader := r.Header.Get("Authorization")
		metricsToken := getMetricsToken()
		if metricsToken == "" {
			log.Error("metrics_token_missing", Fields{
				"client_ip": r.RemoteAddr,
			})
			http.Error(w, "Metrics token not configured", http.StatusServiceUnavailable)
			return
		}

		expectedToken := "Bearer " + metricsToken
		if subtle.ConstantTimeCompare([]byte(authHeader), []byte(expectedToken)) != 1 {
			log.Warn("unauthorized_metrics_access", Fields{
				"client_ip": r.RemoteAddr,
			})
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		metrics := globalMetrics.GetMetrics()

		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")

		if err := json.NewEncoder(w).Encode(metrics); err != nil {
			log.Error("metrics_encode_error", Fields{
				"error": err.Error(),
			})
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}

		log.Debug("metrics_served", Fields{
			"client_ip": r.RemoteAddr,
		})
	}
}

// getMetricsToken returns the metrics authorization token
func getMetricsToken() string {
	// Pull from centralized secrets (env or Secret Manager)
	return config.GetSecretOrDefault(context.Background(), "METRICS_TOKEN", "")
}

// LogMetricsPeriodically logs metrics at regular intervals
func LogMetricsPeriodically(ctx context.Context, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	log := Default()

	for {
		select {
		case <-ticker.C:
			metrics := globalMetrics.GetMetrics()
			log.Info("metrics_snapshot", Fields{
				"metrics": metrics,
			})
		case <-ctx.Done():
			return
		}
	}
}
