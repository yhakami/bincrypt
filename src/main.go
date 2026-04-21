// Package main is the entry point for the BinCrypt server.
//
// BinCrypt is a pastebin service with browser-side encryption for
// password-protected pastes. No-password pastes are stored as unencrypted paste
// data by design.
//
// Storage Backends:
//   - SQLite (default): Embedded database, no external dependencies
//   - PostgreSQL: Enterprise-grade, requires DATABASE_URL
//   - GCS: Google Cloud Storage (requires -tags gcp build flag)
//
// Rate Limiting Backends:
//   - Memory (default): In-process, no external dependencies
//   - Redis: Distributed, requires REDIS_URL
//   - Firestore: Google Cloud (requires -tags gcp build flag)
//
// Build:
//
//	go build -o bincrypt .                    # SQLite + Memory (vendor-neutral)
//	go build -tags gcp -o bincrypt .          # All backends including GCS/Firestore
package main

import (
	"context"
	"embed"
	"flag"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/yhakami/bincrypt/src/config"
	"github.com/yhakami/bincrypt/src/logger"
	"github.com/yhakami/bincrypt/src/services"
)

// Version information (set via ldflags during build)
var (
	version = "dev"
	commit  = "unknown"
)

//go:embed static/*
var staticFiles embed.FS

func main() {
	ctx := context.Background()

	// Initialize logging
	logLevel := logger.INFO
	if os.Getenv("DEBUG") == "true" {
		logLevel = logger.DEBUG
	}
	logger.Init(logLevel, true)
	logger.InitAuditLogger(logger.Default())

	auditLog := logger.GetAuditLogger()
	log := logger.Default()

	auditLog.LogSystemStart(version, map[string]interface{}{
		"version": version,
		"commit":  commit,
		"port":    os.Getenv("PORT"),
	})

	log.Info("bincrypt_starting", logger.Fields{
		"version": version,
		"commit":  commit,
	})

	// Initialize secrets provider
	if err := config.Initialize(ctx); err != nil {
		log.Error("secrets_init_failed", logger.Fields{"error": err.Error()})
		os.Exit(1)
	}
	defer config.Close()

	// Parse flags
	port := flag.String("port", config.GetSecretOrDefault(ctx, "PORT", "8080"), "Server port")
	flag.Parse()

	// Determine storage backend (default: sqlite for vendor-neutral operation)
	storageBackend := config.GetSecretOrDefault(ctx, "STORAGE_BACKEND", "sqlite")
	log.Info("storage_backend_selected", logger.Fields{"backend": storageBackend})

	var storageService *services.StorageService
	var err error

	switch storageBackend {
	case "sqlite":
		dbPath := config.GetSecretOrDefault(ctx, "SQLITE_PATH", "./bincrypt.db")
		backend, err := services.NewSQLiteBackend(dbPath)
		if err != nil {
			log.Error("sqlite_init_failed", logger.Fields{"error": err.Error()})
			os.Exit(1)
		}

		if err := backend.Ping(ctx); err != nil {
			log.Error("sqlite_ping_failed", logger.Fields{"error": err.Error()})
			os.Exit(1)
		}

		log.Info("sqlite_storage_initialized", logger.Fields{"path": dbPath})
		storageService = services.NewStorageServiceWithBackend(backend)

	case "postgres":
		dbURL := config.MustGetSecret(ctx, "DATABASE_URL")
		pool, err := pgxpool.New(ctx, dbURL)
		if err != nil {
			log.Error("postgres_connection_failed", logger.Fields{"error": err.Error()})
			os.Exit(1)
		}

		if err := pool.Ping(ctx); err != nil {
			log.Error("postgres_ping_failed", logger.Fields{"error": err.Error()})
			os.Exit(1)
		}

		log.Info("postgres_storage_initialized", logger.Fields{})
		storageService = services.NewStorageServiceWithBackend(services.NewPostgresBackend(pool))

	case "gcs":
		storageService, err = initGCSBackend(ctx, log)
		if err != nil {
			log.Error("gcs_init_failed", logger.Fields{"error": err.Error()})
			os.Exit(1)
		}
		defer closeGCSClient()

	default:
		log.Error("invalid_storage_backend", logger.Fields{"backend": storageBackend})
		os.Exit(1)
	}

	// Configure optional BTCPay integration
	btcpayEndpoint := config.GetSecretOrDefault(ctx, "BTCPAY_ENDPOINT", "")
	btcpayAPIKey := config.GetSecretOrDefault(ctx, "BTCPAY_APIKEY", "")
	btcpayStoreID := config.GetSecretOrDefault(ctx, "BTCPAY_STORE_ID", "")
	btcpayWebhookSecret := config.GetSecretOrDefault(ctx, "BTCPAY_WEBHOOK_SECRET", "")

	var invoiceService services.InvoiceService
	if btcpayEndpoint != "" || btcpayAPIKey != "" || btcpayStoreID != "" || btcpayWebhookSecret != "" {
		client, err := services.NewBTCPayClient(btcpayEndpoint, btcpayAPIKey, btcpayStoreID, btcpayWebhookSecret, nil)
		if err != nil {
			log.Warn("btcpay_client_disabled", logger.Fields{"reason": err.Error()})
		} else {
			invoiceService = client
			log.Info("btcpay_client_enabled", logger.Fields{"endpoint": btcpayEndpoint})
		}
	}

	// Create server configuration
	srvConfig := &Config{
		Port:           *port,
		StorageService: storageService,
		StaticFiles:    staticFiles,
		EnableMetrics:  config.GetSecretOrDefault(ctx, "METRICS_ENABLED", "false") == "true",
		InvoiceService: invoiceService,
	}

	// Create server
	srv := New(srvConfig)

	// Setup graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)

	// Start server in goroutine
	serverErr := make(chan error, 1)
	go func() {
		log.Info("server_starting", logger.Fields{
			"port":    *port,
			"backend": storageBackend,
		})
		serverErr <- srv.Start()
	}()

	// Wait for shutdown signal or server error
	select {
	case sig := <-sigChan:
		log.Info("shutdown_signal_received", logger.Fields{"signal": sig.String()})
		auditLog.LogEvent(context.Background(), logger.AuditSystemStop, logger.Fields{
			"signal": sig.String(),
			"reason": "shutdown signal received",
		})

		shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		log.Info("shutdown_initiated", logger.Fields{"timeout_seconds": 30})

		if err := srv.Shutdown(shutdownCtx); err != nil {
			log.Error("shutdown_http_failed", logger.Fields{"error": err.Error()})
		}

		config.Close()
		log.Info("shutdown_completed", logger.Fields{})

	case err := <-serverErr:
		if err != nil {
			log.Error("server_start_failed", logger.Fields{"error": err.Error()})
			os.Exit(1)
		}
	}
}
