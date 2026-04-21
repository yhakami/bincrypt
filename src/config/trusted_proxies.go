package config

import (
	"net"
	"os"
	"strings"
)

// TrustedProxyConfig defines which proxies we trust for IP forwarding headers.
// When enabled, only requests from trusted proxy IPs will have their X-Forwarded-For
// headers trusted. This prevents IP spoofing attacks from untrusted sources.
type TrustedProxyConfig struct {
	Enabled        bool
	TrustedCIDRs   []*net.IPNet
	TrustedHeaders []string
}

// LoadTrustedProxyConfig loads trusted proxy configuration from environment.
// Automatically configures for Cloud Run when K_SERVICE is detected.
// For manual configuration, set TRUSTED_PROXIES_ENABLED=true and provide
// a comma-separated list of CIDR ranges in TRUSTED_PROXIES.
func LoadTrustedProxyConfig() *TrustedProxyConfig {
	// Auto-configure for Cloud Run environment
	if os.Getenv("K_SERVICE") != "" {
		return &TrustedProxyConfig{
			Enabled: true,
			TrustedCIDRs: []*net.IPNet{
				mustParseCIDR("35.191.0.0/16"),  // Google Cloud Load Balancer range 1
				mustParseCIDR("130.211.0.0/22"), // Google Cloud Load Balancer range 2
			},
			TrustedHeaders: []string{"X-Forwarded-For"},
		}
	}

	// Manual configuration via environment variables
	enabled := os.Getenv("TRUSTED_PROXIES_ENABLED") == "true"
	if !enabled {
		return &TrustedProxyConfig{Enabled: false}
	}

	proxiesStr := os.Getenv("TRUSTED_PROXIES")
	if proxiesStr == "" {
		return &TrustedProxyConfig{Enabled: false}
	}

	// Parse proxy CIDR ranges
	cidrs := make([]*net.IPNet, 0)
	for _, cidr := range strings.Split(proxiesStr, ",") {
		cidr = strings.TrimSpace(cidr)
		_, network, err := net.ParseCIDR(cidr)
		if err != nil {
			// Try parsing as individual IP
			ip := net.ParseIP(cidr)
			if ip != nil {
				// Convert to /32 (IPv4) or /128 (IPv6) network
				if ip.To4() != nil {
					network = &net.IPNet{IP: ip, Mask: net.CIDRMask(32, 32)}
				} else {
					network = &net.IPNet{IP: ip, Mask: net.CIDRMask(128, 128)}
				}
			}
		}
		if network != nil {
			cidrs = append(cidrs, network)
		}
	}

	// Parse trusted headers (default to X-Forwarded-For)
	headersStr := os.Getenv("TRUSTED_HEADERS")
	headers := []string{"X-Forwarded-For"}
	if headersStr != "" {
		headers = strings.Split(headersStr, ",")
		for i := range headers {
			headers[i] = strings.TrimSpace(headers[i])
		}
	}

	return &TrustedProxyConfig{
		Enabled:        true,
		TrustedCIDRs:   cidrs,
		TrustedHeaders: headers,
	}
}

// IsTrustedProxy checks if an IP address is in the trusted proxy ranges.
// Returns false if proxy trust is disabled or IP is not in any trusted range.
func (c *TrustedProxyConfig) IsTrustedProxy(ipStr string) bool {
	if !c.Enabled {
		return false
	}

	ip := net.ParseIP(ipStr)
	if ip == nil {
		return false
	}

	for _, cidr := range c.TrustedCIDRs {
		if cidr.Contains(ip) {
			return true
		}
	}

	return false
}

// mustParseCIDR parses a CIDR string and panics on error.
// Used for hardcoded trusted ranges that should never fail.
func mustParseCIDR(cidr string) *net.IPNet {
	_, network, err := net.ParseCIDR(cidr)
	if err != nil {
		panic("invalid CIDR: " + cidr)
	}
	return network
}
