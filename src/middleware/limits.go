package middleware

import (
	"net/http"
	"strings"
	"time"
)

// Timeout middleware times out requests after 25s.
func Timeout(next http.Handler) http.Handler {
	return http.TimeoutHandler(next, 25*time.Second, "request timed out")
}

// BodyLimit sets max body size depending on content type.
func BodyLimit(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var limit int64 = 2 << 20 // 2MiB
		ct := r.Header.Get("Content-Type")
		if strings.Contains(ct, "multipart/form-data") || strings.Contains(ct, "octet-stream") {
			limit = 10 << 20
		}
		r.Body = http.MaxBytesReader(w, r.Body, limit)
		next.ServeHTTP(w, r)
	})
}
