package logger

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/yhakami/bincrypt/src/config"
)

func TestMetricsHandlerRequiresConfiguredToken(t *testing.T) {
	t.Setenv("METRICS_TOKEN", "")
	if err := config.Initialize(context.Background()); err != nil {
		t.Fatalf("failed to initialize config: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/metrics", nil)
	rec := httptest.NewRecorder()

	MetricsHandler().ServeHTTP(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("metrics without token returned %d, want %d", rec.Code, http.StatusServiceUnavailable)
	}
}

func TestMetricsHandlerRequiresMatchingBearerToken(t *testing.T) {
	t.Setenv("METRICS_TOKEN", "test-token")
	if err := config.Initialize(context.Background()); err != nil {
		t.Fatalf("failed to initialize config: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/metrics", nil)
	req.Header.Set("Authorization", "Bearer wrong-token")
	rec := httptest.NewRecorder()

	MetricsHandler().ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("metrics with wrong token returned %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestMetricsHandlerAcceptsMatchingBearerToken(t *testing.T) {
	t.Setenv("METRICS_TOKEN", "test-token")
	if err := config.Initialize(context.Background()); err != nil {
		t.Fatalf("failed to initialize config: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/metrics", nil)
	req.Header.Set("Authorization", "Bearer test-token")
	rec := httptest.NewRecorder()

	MetricsHandler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("metrics with correct token returned %d, want %d", rec.Code, http.StatusOK)
	}
}
