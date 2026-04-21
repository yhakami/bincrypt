package main

import (
	"context"
	"embed"
	"io/fs"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/gorilla/mux"
	configpkg "github.com/yhakami/bincrypt/src/config"
	"github.com/yhakami/bincrypt/src/logger"
	"github.com/yhakami/bincrypt/src/services"
)

// Config holds HTTP server configuration.
// InvoiceService can be nil to disable payments.
// StorageService is required.
type Config struct {
	Port           string
	StorageService *services.StorageService
	StaticFiles    embed.FS
	EnableMetrics  bool
	InvoiceService services.InvoiceService
}

// Server represents the HTTP server with all dependencies.
type Server struct {
	config           *Config
	storageService   *services.StorageService
	rateLimitService services.RateLimiter
	invoiceService   services.InvoiceService
	httpServer       *http.Server
	deletionQueue    *services.DeletionQueue
	proxyConfig      *configpkg.TrustedProxyConfig
}

// New creates a new HTTP server with the provided configuration.
// Falls back to in-memory rate limiter if Redis/Firestore unavailable.
func New(config *Config) *Server {
	rateLimiter, err := services.NewRateLimiterFromEnv()
	if err != nil {
		logger.Default().Warn("rate_limiter_fallback_to_memory", logger.Fields{"error": err.Error()})
		rateLimiter = services.NewRateLimitService()
	}

	// Initialize deletion queue with configurable workers
	deletionWorkers := 10 // Default
	if workersStr := os.Getenv("DELETION_WORKERS"); workersStr != "" {
		if w, err := strconv.Atoi(workersStr); err == nil && w > 0 {
			deletionWorkers = w
		}
	}

	deletionQueue := services.NewDeletionQueue(config.StorageService, deletionWorkers)

	// Load trusted proxy configuration
	proxyConfig := configpkg.LoadTrustedProxyConfig()

	return &Server{
		config:           config,
		storageService:   config.StorageService,
		rateLimitService: rateLimiter,
		invoiceService:   config.InvoiceService,
		deletionQueue:    deletionQueue,
		proxyConfig:      proxyConfig,
	}
}

func (s *Server) Start() error {
	r := mux.NewRouter()

	r.Use(Recovery)
	r.Use(Logging)
	r.Use(s.GlobalRateLimit(300))
	r.Use(RequestSizeLimit(5 << 20))
	r.Use(InputValidation)
	r.Use(CORS)
	r.Use(CSP)
	r.Use(InjectNonceMiddleware)

	api := r.PathPrefix("/api").Subrouter()

	api.Handle("/paste",
		s.RateLimiter("create_paste", 10, time.Minute)(
			http.HandlerFunc(s.CreatePaste),
		),
	).Methods("POST")

	api.Handle("/paste/{id}",
		s.RateLimiter("get_paste", 100, time.Minute)(
			http.HandlerFunc(s.GetPasteAPI),
		),
	).Methods("GET")

	api.HandleFunc("/health", s.HealthCheck).Methods("GET")
	if s.config.EnableMetrics {
		api.HandleFunc("/metrics", logger.MetricsHandler()).Methods("GET")
	}

	api.Handle("/config",
		s.RateLimiter("api_config", 60, time.Minute)(
			http.HandlerFunc(s.GetClientConfig),
		),
	).Methods("GET")

	if s.invoiceService != nil {
		api.Handle("/invoice",
			s.RateLimiter("create_invoice", 10, time.Minute)(
				http.HandlerFunc(s.CreateInvoice),
			),
		).Methods("POST")

		api.HandleFunc("/payhook", s.PaymentWebhook).Methods("POST")
	}

	r.Handle("/p/{id}",
		s.RateLimiter("view_paste", 100, time.Minute)(
			http.HandlerFunc(s.ViewPaste),
		),
	).Methods("GET")

	staticFS, _ := fs.Sub(s.config.StaticFiles, "static")
	r.PathPrefix("/static/").Handler(http.StripPrefix("/static/", http.FileServer(http.FS(staticFS))))

	r.HandleFunc("/", s.servePage("static/index.html")).Methods("GET")
	r.HandleFunc("/news", s.servePage("static/news.html")).Methods("GET")
	r.HandleFunc("/about", s.servePage("static/about.html")).Methods("GET")

	logger.Info("server_listening", logger.Fields{"port": s.config.Port})

	s.httpServer = &http.Server{
		Addr:              ":" + s.config.Port,
		Handler:           r,
		ReadTimeout:       10 * time.Second,
		ReadHeaderTimeout: 5 * time.Second,
		WriteTimeout:      15 * time.Second,
		IdleTimeout:       60 * time.Second,
		MaxHeaderBytes:    1 << 20,
	}

	return s.httpServer.ListenAndServe()
}

// Shutdown gracefully shuts down the HTTP server and cleans up resources.
// This method is idempotent and can be called multiple times safely.
func (s *Server) Shutdown(ctx context.Context) error {
	log := logger.Default()

	if s.httpServer != nil {
		log.Info("shutdown_http_server", logger.Fields{})
		if err := s.httpServer.Shutdown(ctx); err != nil {
			log.Error("shutdown_http_server_failed", logger.Fields{"error": err.Error()})
			return err
		}
	}

	if s.deletionQueue != nil {
		log.Info("shutdown_deletion_queue", logger.Fields{})
		// Calculate remaining time from context deadline, default to 10 seconds
		timeout := 10 * time.Second
		if deadline, ok := ctx.Deadline(); ok {
			timeout = time.Until(deadline)
			if timeout < 0 {
				timeout = time.Second // Minimum 1 second
			}
		}
		if err := s.deletionQueue.Shutdown(timeout); err != nil {
			log.Error("shutdown_deletion_queue_failed", logger.Fields{"error": err.Error()})
			// Continue with other shutdowns even if deletion queue fails
		}
	}

	if s.rateLimitService != nil {
		log.Info("shutdown_rate_limiter", logger.Fields{})
		s.rateLimitService.Stop()
	}

	if s.storageService != nil {
		log.Info("shutdown_storage_service", logger.Fields{})
		if err := s.storageService.Close(); err != nil {
			log.Error("shutdown_storage_failed", logger.Fields{"error": err.Error()})
			return err
		}
	}

	return nil
}

func (s *Server) servePage(path string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		content, err := s.config.StaticFiles.ReadFile(path)
		if err != nil {
			http.Error(w, "Page not found", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write(content)
	}
}
