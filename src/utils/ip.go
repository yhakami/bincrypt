package utils

import (
	"net"
	"net/http"
)

// GetClientIP extracts the client IP from the request
func GetClientIP(r *http.Request) string {
	// Trust X-Real-IP header set by NGINX after real_ip processing
	if ip := r.Header.Get("X-Real-IP"); ip != "" {
		return ip
	}

	// Fall back to RemoteAddr
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}
