package server

import (
	"context"
	"embed"
	"io/fs"
	"log"
	"net/http"
	"time"

	"bincrypt/src/handlers"
	"bincrypt/src/middleware"
	"bincrypt/src/services"
	"cloud.google.com/go/storage"
	"github.com/gorilla/mux"
)

// Config holds server configuration
type Config struct {
	Port             string
	BucketName       string
	BTCPayURL        string
	BTCPayAPIKey     string
	StorageClient    *storage.Client
	StaticFiles      embed.FS
}

// Server represents the HTTP server
type Server struct {
	config           *Config
	bucket           *storage.BucketHandle
	storageService   *services.StorageService
	rateLimitService *services.RateLimitService
	invoiceService   *services.InvoiceService
}

// New creates a new server
func New(config *Config) *Server {
	bucket := config.StorageClient.Bucket(config.BucketName)
	
	return &Server{
		config:           config,
		bucket:           bucket,
		storageService:   services.NewStorageService(bucket),
		rateLimitService: services.NewRateLimitService(bucket),
		invoiceService:   services.NewInvoiceService(bucket, config.BTCPayURL, config.BTCPayAPIKey),
	}
}

// Start starts the HTTP server
func (s *Server) Start() error {
	// Start cleanup routine
	go s.cleanupRoutine()
	
	// Create router
	r := mux.NewRouter()
	
	// Apply global middleware
	r.Use(middleware.Recovery)
	r.Use(middleware.Logging)
	
	// Create handlers
	pasteHandler := handlers.NewPasteHandler(s.storageService)
	invoiceHandler := handlers.NewInvoiceHandler(s.invoiceService)
	healthHandler := handlers.NewHealthHandler(s.bucket)
	configHandler := handlers.NewConfigHandler()
	
	// API routes with rate limiting
	api := r.PathPrefix("/api").Subrouter()
	
	// Paste endpoints
	api.Handle("/paste", 
		middleware.RateLimiter(s.rateLimitService, "create_paste", 10, time.Hour)(
			http.HandlerFunc(pasteHandler.CreatePaste),
		)).Methods("POST")
	
	// API endpoint for retrieving paste data (JSON)
	api.Handle("/paste/{id}",
		middleware.RateLimiter(s.rateLimitService, "get_paste", 100, time.Hour)(
			http.HandlerFunc(pasteHandler.GetPasteAPI),
		)).Methods("GET")
	
	// Invoice endpoints
	api.Handle("/invoice",
		middleware.RateLimiter(s.rateLimitService, "create_invoice", 5, time.Hour)(
			http.HandlerFunc(invoiceHandler.CreateInvoice),
		)).Methods("POST")
	
	api.HandleFunc("/payhook", invoiceHandler.PaymentWebhook).Methods("POST")
	
	// Health endpoints
	api.HandleFunc("/health", healthHandler.HealthCheck).Methods("GET")
	api.HandleFunc("/metrics", healthHandler.Metrics).Methods("GET")
	
	// Config endpoint
	api.HandleFunc("/config", configHandler.GetClientConfig).Methods("GET")
	
	// View paste with rate limiting
	r.Handle("/p/{id}",
		middleware.RateLimiter(s.rateLimitService, "view_paste", 100, time.Hour)(
			http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				pasteHandler.ViewPaste(w, r, s.config.StaticFiles)
			}),
		)).Methods("GET")
	
	// Static files
	staticFS, _ := fs.Sub(s.config.StaticFiles, "static")
	r.PathPrefix("/static/").Handler(
		http.StripPrefix("/static/", 
			http.FileServer(http.FS(staticFS))))
	
	// Serve pages
	r.HandleFunc("/", s.servePage("static/index.html")).Methods("GET")
	r.HandleFunc("/news", s.servePage("static/news.html")).Methods("GET")
	r.HandleFunc("/about", s.servePage("static/about.html")).Methods("GET")
	r.HandleFunc("/premium", s.servePage("static/premium.html")).Methods("GET")
	
	log.Printf("Starting server on port %s", s.config.Port)
	return http.ListenAndServe(":"+s.config.Port, r)
}

// servePage returns a handler that serves a static HTML page
func (s *Server) servePage(path string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		content, err := s.config.StaticFiles.ReadFile(path)
		if err != nil {
			http.Error(w, "Page not found", http.StatusNotFound)
			return
		}
		
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Content-Security-Policy", "default-src 'self' https://cdn.jsdelivr.net https://unpkg.com https://www.gstatic.com; script-src 'self' 'unsafe-inline' 'unsafe-eval' https://cdn.jsdelivr.net https://unpkg.com https://www.gstatic.com; style-src 'self' 'unsafe-inline' https://cdn.jsdelivr.net")
		w.Header().Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
		w.Write(content)
	}
}

// cleanupRoutine periodically cleans up expired pastes
func (s *Server) cleanupRoutine() {
	ticker := time.NewTicker(1 * time.Hour)
	defer ticker.Stop()
	
	ctx := context.Background()
	for range ticker.C {
		if err := s.storageService.CleanupExpiredPastes(ctx); err != nil {
			log.Printf("Cleanup error: %v", err)
		}
	}
}