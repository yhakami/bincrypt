package utils

import (
	"net"
	"net/http"
	"strings"

	configpkg "github.com/yhakami/bincrypt/src/config"
)

// GetClientIP extracts the real client IP with proxy trust validation.
// This prevents IP spoofing attacks by only trusting X-Forwarded-For headers
// from known proxy IP ranges. When running behind untrusted proxies or directly
// exposed, always uses RemoteAddr to prevent header injection attacks.
//
// Security model:
// - If proxyConfig is nil or disabled: Always use RemoteAddr (secure default)
// - If request from untrusted IP: Use RemoteAddr (prevents spoofing)
// - If request from trusted proxy: Trust X-Forwarded-For header
func GetClientIP(r *http.Request, proxyConfig *configpkg.TrustedProxyConfig) string {
	// Extract remote address (direct connection IP)
	remoteIP := parseIP(r.RemoteAddr)

	// If proxy support disabled, always use RemoteAddr (fail secure)
	if proxyConfig == nil || !proxyConfig.Enabled {
		return remoteIP
	}

	// Check if request came from trusted proxy
	if !proxyConfig.IsTrustedProxy(remoteIP) {
		// Not from trusted proxy - use RemoteAddr to prevent spoofing
		return remoteIP
	}

	// Request from trusted proxy - check forwarding headers
	for _, header := range proxyConfig.TrustedHeaders {
		value := r.Header.Get(header)
		if value == "" {
			continue
		}

		// For X-Forwarded-For, take the FIRST IP (original client)
		// Format: client, proxy1, proxy2, ...
		if header == "X-Forwarded-For" {
			ips := strings.Split(value, ",")
			if len(ips) > 0 {
				clientIP := strings.TrimSpace(ips[0])
				if ip := net.ParseIP(clientIP); ip != nil {
					return clientIP
				}
			}
		} else {
			// For other headers (X-Real-IP), use value directly
			clientIP := strings.TrimSpace(value)
			if ip := net.ParseIP(clientIP); ip != nil {
				return clientIP
			}
		}
	}

	// No valid headers found, fall back to RemoteAddr
	return remoteIP
}

// parseIP extracts IP from addr in "IP:port" format
func parseIP(addr string) string {
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		// RemoteAddr might be just an IP without port
		if net.ParseIP(addr) != nil {
			return addr
		}
		return "127.0.0.1" // Safe fallback
	}
	return host
}
