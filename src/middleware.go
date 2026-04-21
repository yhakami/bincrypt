package main

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gorilla/mux"
	configpkg "github.com/yhakami/bincrypt/src/config"
	"github.com/yhakami/bincrypt/src/logger"
	"github.com/yhakami/bincrypt/src/utils"
)

type responseWriter struct {
	http.ResponseWriter
	statusCode int
	size       int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}
func (rw *responseWriter) Write(b []byte) (int, error) {
	n, err := rw.ResponseWriter.Write(b)
	rw.size += n
	return n, err
}

func Logging(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		logPath := sanitizePathForLogs(r.URL.Path)
		endpoint := r.Method + " " + logPath
		logger.GetGlobalMetrics().IncrementRequest(endpoint)
		requestID := r.Header.Get("X-Request-ID")
		if requestID == "" {
			if id, err := utils.GenerateID(); err == nil {
				requestID = id
			}
		}
		traceHeader := r.Header.Get("X-Cloud-Trace-Context")
		var traceID, spanID string
		if traceHeader != "" {
			parts := strings.Split(traceHeader, "/")
			if len(parts) > 0 {
				traceID = parts[0]
			}
			if len(parts) > 1 {
				sp := strings.Split(parts[1], ";")
				if len(sp) > 0 {
					spanID = sp[0]
				}
			}
		}
		clientIP := utils.GetClientIP(r, nil)
		userAgent := r.Header.Get("User-Agent")
		ctx := context.WithValue(r.Context(), logger.RequestIDKey, requestID)
		ctx = context.WithValue(ctx, logger.ClientIPKey, clientIP)
		ctx = context.WithValue(ctx, logger.UserAgentKey, userAgent)
		if traceID != "" {
			ctx = context.WithValue(ctx, logger.TraceIDKey, traceID)
		}
		if spanID != "" {
			ctx = context.WithValue(ctx, logger.SpanIDKey, spanID)
		}
		r = r.WithContext(ctx)

		wrapped := &responseWriter{ResponseWriter: w, statusCode: http.StatusOK}
		wrapped.Header().Set("X-Request-ID", requestID)

		if !isSensitivePath(r.URL.Path) {
			logger.WithContext(ctx).Info("request_started", logger.Fields{
				"method": r.Method, "path": logPath, "query": sanitizeQuery(r.URL.RawQuery),
				"remote_addr": r.RemoteAddr, "client_ip": clientIP, "user_agent": userAgent,
				"referer": r.Header.Get("Referer"), "content_type": r.Header.Get("Content-Type"),
			})
		}

		next.ServeHTTP(wrapped, r)

		duration := time.Since(start)
		logFunc := logger.WithContext(ctx).Info
		if wrapped.statusCode >= 500 {
			logFunc = logger.WithContext(ctx).Error
		} else if wrapped.statusCode >= 400 {
			logFunc = logger.WithContext(ctx).Warn
		}
		fields := logger.Fields{"method": r.Method, "path": logPath, "status": wrapped.statusCode, "duration_ms": duration.Milliseconds(), "bytes_written": wrapped.size, "client_ip": clientIP}
		if wrapped.statusCode >= 400 {
			logger.GetGlobalMetrics().IncrementError(endpoint)
			fields["user_agent"] = userAgent
			fields["referer"] = r.Header.Get("Referer")
		}
		logFunc("request_completed", fields)
		if wrapped.statusCode == 429 {
			logger.GetAuditLogger().LogRateLimitHit(ctx, clientIP, logPath, 0, "")
		}
	})
}

func sanitizeQuery(query string) string {
	if query == "" {
		return ""
	}
	sens := []string{"token", "key", "secret", "password", "auth"}
	parts := strings.Split(query, "&")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		kv := strings.SplitN(part, "=", 2)
		if len(kv) == 2 {
			k := strings.ToLower(kv[0])
			for _, s := range sens {
				if strings.Contains(k, s) {
					kv[1] = "<redacted>"
					break
				}
			}
			out = append(out, kv[0]+"="+kv[1])
		} else {
			out = append(out, part)
		}
	}
	return strings.Join(out, "&")
}

func sanitizePathForLogs(path string) string {
	const redactedID = "{paste_id}"
	if strings.HasPrefix(path, "/p/") {
		parts := strings.Split(path, "/")
		if len(parts) == 3 && ValidatePasteID(parts[2]) == nil {
			return "/p/" + redactedID
		}
	}
	if strings.HasPrefix(path, "/api/paste/") {
		parts := strings.Split(path, "/")
		if len(parts) == 4 && ValidatePasteID(parts[3]) == nil {
			return "/api/paste/" + redactedID
		}
	}
	return path
}

func isSensitivePath(path string) bool {
	return path == "/api/health"
}

func RequestSizeLimit(maxBytes int64) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			r.Body = http.MaxBytesReader(w, r.Body, maxBytes)
			if ct := r.Header.Get("Content-Type"); ct == "application/x-www-form-urlencoded" || ct == "multipart/form-data" {
				_ = r.ParseMultipartForm(maxBytes)
			}
			next.ServeHTTP(w, r)
		})
	}
}

func InputValidation(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		validMethods := map[string]bool{"GET": true, "POST": true, "OPTIONS": true, "HEAD": true}
		if !validMethods[r.Method] {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		for name, values := range r.Header {
			if len(name) > 128 {
				http.Error(w, "Invalid header name", http.StatusBadRequest)
				return
			}

			for _, value := range values {
				if len(value) > 8192 {
					http.Error(w, "Header value too long", http.StatusBadRequest)
					return
				}

				if strings.HasPrefix(strings.ToLower(name), "x-forwarded-") && strings.Contains(value, "\n") {
					http.Error(w, "Invalid header value", http.StatusBadRequest)
					return
				}
			}
		}

		vars := mux.Vars(r)
		for key, value := range vars {
			if key == "id" {
				if len(value) > 100 || len(value) < 1 {
					http.Error(w, "Invalid paste ID", http.StatusBadRequest)
					return
				}

				for _, ch := range value {
					if !((ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') || (ch >= '0' && ch <= '9') || ch == '-' || ch == '_') {
						http.Error(w, "Invalid paste ID format", http.StatusBadRequest)
						return
					}
				}
			}
		}

		query := r.URL.Query()
		for key, values := range query {
			if len(key) > 128 {
				http.Error(w, "Invalid query parameter", http.StatusBadRequest)
				return
			}

			for _, value := range values {
				if len(value) > 1024 {
					http.Error(w, "Query parameter too long", http.StatusBadRequest)
					return
				}
			}
		}

		if r.Method == "POST" {
			contentType := r.Header.Get("Content-Type")
			if contentType == "" {
				contentType = "application/octet-stream"
			}

			mediaType := contentType
			if idx := strings.Index(contentType, ";"); idx != -1 {
				mediaType = strings.TrimSpace(contentType[:idx])
			}

			allowedTypes := map[string]bool{
				"application/json":                  true,
				"application/x-www-form-urlencoded": true,
				"multipart/form-data":               true,
				"text/plain":                        true,
			}

			if !allowedTypes[mediaType] {
				http.Error(w, "Unsupported content type", http.StatusUnsupportedMediaType)
				return
			}
		}

		if len(r.URL.Path) > 2048 {
			http.Error(w, "URL path too long", http.StatusBadRequest)
			return
		}

		if strings.Contains(r.URL.Path, "..") || strings.Contains(r.URL.Path, "./") {
			http.Error(w, "Invalid path", http.StatusBadRequest)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func (s *Server) RateLimiter(action string, limit int, window time.Duration) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			clientIP := utils.GetClientIP(r, s.proxyConfig)
			key := fmt.Sprintf("%s:%s", action, clientIP)
			ctx := r.Context()
			allowed, remaining, resetAt, retryAfter, err := s.rateLimitService.CheckRateLimit(ctx, key, limit, window)
			if err != nil {
				logger.WithContext(ctx).Error("rate_limit_check_failed", logger.Fields{
					"key":   key,
					"error": err.Error(),
				})
				w.Header().Set("X-RateLimit-Limit", strconv.Itoa(limit))
				w.Header().Set("X-RateLimit-Remaining", "0")
				w.Header().Set("X-RateLimit-Reset", strconv.FormatInt(time.Now().Add(window).Unix(), 10))
				w.Header().Set("Retry-After", strconv.Itoa(int(window.Seconds())))
				http.Error(w, "Service temporarily unavailable", http.StatusServiceUnavailable)
				return
			}
			w.Header().Set("X-RateLimit-Limit", strconv.Itoa(limit))
			w.Header().Set("X-RateLimit-Remaining", strconv.Itoa(remaining))
			w.Header().Set("X-RateLimit-Reset", strconv.FormatInt(resetAt.Unix(), 10))
			if !allowed {
				if retryAfter > 0 {
					w.Header().Set("Retry-After", strconv.Itoa(int(retryAfter.Seconds())))
				}
				logger.WithContext(ctx).Warn("rate_limit_exceeded", logger.Fields{
					"client_ip": clientIP,
					"action":    action,
				})
				http.Error(w, "Rate limit exceeded. Please try again later.", http.StatusTooManyRequests)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func (s *Server) GlobalRateLimit(requestsPerMinute int) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			clientIP := utils.GetClientIP(r, s.proxyConfig)
			key := fmt.Sprintf("global:%s", clientIP)
			allowed, _, _, retryAfter, err := s.rateLimitService.CheckRateLimit(r.Context(), key, requestsPerMinute, time.Minute)
			if err != nil {
				logger.WithContext(r.Context()).Error("global_rate_limit_check_failed", logger.Fields{
					"client_ip": clientIP,
					"error":     err.Error(),
				})
				next.ServeHTTP(w, r)
				return
			}
			if !allowed {
				if retryAfter > 0 {
					w.Header().Set("Retry-After", strconv.Itoa(int(retryAfter.Seconds())))
				}
				http.Error(w, "Too many requests", http.StatusTooManyRequests)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// Recovery middleware
func Recovery(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if err := recover(); err != nil {
				logger.WithContext(r.Context()).Error("panic_recovered", logger.Fields{
					"error":      fmt.Sprintf("%v", err),
					"panic_type": fmt.Sprintf("%T", err),
				})
				http.Error(w, "Internal server error", http.StatusInternalServerError)
			}
		}()
		next.ServeHTTP(w, r)
	})
}

type contextKey string

const nonceKey contextKey = "csp-nonce"

func contextWithNonce(ctx context.Context, nonce string) context.Context {
	return context.WithValue(ctx, nonceKey, nonce)
}
func getNonceFromContext(ctx context.Context) string { v, _ := ctx.Value(nonceKey).(string); return v }

func generateCSPNonce() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(b), nil
}

func CSP(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		nonce, err := generateCSPNonce()
		if err != nil {
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}
		r = r.WithContext(contextWithNonce(r.Context(), nonce))

		firebaseDomain := configpkg.GetSecretOrDefault(context.Background(), "FIREBASE_AUTH_DOMAIN", "")
		storageDomain := "https://firebasestorage.googleapis.com"
		connectSrcDomains := fmt.Sprintf("'self' %s %s https://challenges.cloudflare.com", firebaseDomain, storageDomain)
		if firebaseDomain != "" {
			connectSrcDomains = fmt.Sprintf("'self' https://%s %s https://challenges.cloudflare.com", firebaseDomain, storageDomain)
		}

		csp := fmt.Sprintf(
			"default-src 'self'; "+
				"script-src 'self' 'nonce-%s' https://challenges.cloudflare.com; "+
				"style-src 'self' 'nonce-%s'; "+
				"font-src 'self'; "+
				"img-src 'self' data:; "+
				"connect-src %s; "+
				"frame-src https://challenges.cloudflare.com; "+
				"object-src 'none'; "+
				"base-uri 'self'; "+
				"form-action 'self'; "+
				"frame-ancestors 'none'; "+
				"block-all-mixed-content; "+
				"upgrade-insecure-requests",
			nonce, nonce, connectSrcDomains,
		)
		w.Header().Set("Content-Security-Policy", csp)
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("X-XSS-Protection", "1; mode=block")
		w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
		w.Header().Set("Permissions-Policy", "accelerometer=(), camera=(), geolocation=(), gyroscope=(), magnetometer=(), microphone=(), payment=(), usb=()")
		w.Header().Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains; preload")
		next.ServeHTTP(w, r)
	})
}

type CSPResponseWriter struct {
	http.ResponseWriter
	nonce       string
	wroteHeader bool
}

func (w *CSPResponseWriter) Write(data []byte) (int, error) {
	ct := w.Header().Get("Content-Type")
	if ct == "text/html; charset=utf-8" || ct == "text/html" {
		md := injectNonce(data, w.nonce)
		return w.ResponseWriter.Write(md)
	}
	return w.ResponseWriter.Write(data)
}
func (w *CSPResponseWriter) WriteHeader(statusCode int) {
	if !w.wroteHeader {
		w.wroteHeader = true
		w.ResponseWriter.WriteHeader(statusCode)
	}
}
func InjectNonceMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		nonce := getNonceFromContext(r.Context())
		if nonce == "" {
			next.ServeHTTP(w, r)
			return
		}
		cspWriter := &CSPResponseWriter{ResponseWriter: w, nonce: nonce}
		next.ServeHTTP(cspWriter, r)
	})
}
func injectNonce(html []byte, nonce string) []byte {
	s := string(html)
	s = strings.ReplaceAll(s, "{{CSP_NONCE}}", nonce)
	return []byte(s)
}

// CORS middleware
func CORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		allowedOrigins := configpkg.GetSecretOrDefault(context.Background(), "CORS_ALLOWED_ORIGINS", "")
		if allowedOrigins == "" {
			next.ServeHTTP(w, r)
			return
		}
		origins := strings.Split(allowedOrigins, ",")

		origin := r.Header.Get("Origin")
		for _, allowed := range origins {
			if origin == strings.TrimSpace(allowed) || strings.TrimSpace(allowed) == "*" {
				w.Header().Set("Access-Control-Allow-Origin", origin)
				break
			}
		}

		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}

		next.ServeHTTP(w, r)
	})
}
