package tests

import (
	"net"
	"net/http"
	"testing"

	"github.com/yhakami/bincrypt/src/config"
	"github.com/yhakami/bincrypt/src/utils"
)

func testTrustedProxyConfig() *config.TrustedProxyConfig {
	_, allIPv4, _ := net.ParseCIDR("0.0.0.0/0")
	_, allIPv6, _ := net.ParseCIDR("::/0")
	return &config.TrustedProxyConfig{
		Enabled:        true,
		TrustedCIDRs:   []*net.IPNet{allIPv4, allIPv6},
		TrustedHeaders: []string{"X-Forwarded-For", "X-Real-IP"},
	}
}

func TestGetClientIP(t *testing.T) {
	tests := []struct {
		name       string
		headers    map[string]string
		remoteAddr string
		expected   string
	}{
		{
			name: "Cloud Run with typical X-Forwarded-For",
			headers: map[string]string{
				"X-Forwarded-For": "203.0.113.7, 203.0.113.8",
			},
			remoteAddr: "10.0.0.1:1234",
			expected:   "203.0.113.7",
		},
		{
			name: "Cloud Run with multiple proxies",
			headers: map[string]string{
				"X-Forwarded-For": "192.168.1.1, 203.0.113.7, 35.191.0.1, 10.0.0.1",
			},
			remoteAddr: "10.0.0.1:1234",
			expected:   "192.168.1.1", // First IP is original client
		},
		{
			name: "Spoofed X-Forwarded-For with invalid first IP",
			headers: map[string]string{
				"X-Forwarded-For": "not-an-ip, 203.0.113.7, 35.191.0.1",
			},
			remoteAddr: "10.0.0.1:1234",
			expected:   "10.0.0.1", // Invalid first IP ignored, fall back to RemoteAddr
		},
		{
			name: "Only X-Real-IP header",
			headers: map[string]string{
				"X-Real-IP": "203.0.113.7",
			},
			remoteAddr: "10.0.0.1:1234",
			expected:   "203.0.113.7",
		},
		{
			name: "X-Forwarded-For with single IP",
			headers: map[string]string{
				"X-Forwarded-For": "203.0.113.7",
			},
			remoteAddr: "10.0.0.1:1234",
			expected:   "203.0.113.7",
		},
		{
			name:       "No headers, only RemoteAddr with port",
			headers:    map[string]string{},
			remoteAddr: "203.0.113.7:1234",
			expected:   "203.0.113.7",
		},
		{
			name:       "No headers, RemoteAddr without port",
			headers:    map[string]string{},
			remoteAddr: "203.0.113.7",
			expected:   "203.0.113.7",
		},
		{
			name: "Empty X-Forwarded-For",
			headers: map[string]string{
				"X-Forwarded-For": "",
			},
			remoteAddr: "203.0.113.7:1234",
			expected:   "203.0.113.7",
		},
		{
			name: "X-Forwarded-For with spaces",
			headers: map[string]string{
				"X-Forwarded-For": " 192.168.1.1 , 203.0.113.7 , 35.191.0.1 ",
			},
			remoteAddr: "10.0.0.1:1234",
			expected:   "192.168.1.1",
		},
		{
			name: "Invalid X-Real-IP falls back to RemoteAddr",
			headers: map[string]string{
				"X-Real-IP": "not-an-ip",
			},
			remoteAddr: "203.0.113.7:1234",
			expected:   "203.0.113.7",
		},
		{
			name: "IPv6 addresses",
			headers: map[string]string{
				"X-Forwarded-For": "2001:db8::1, 2001:db8::2",
			},
			remoteAddr: "[::1]:1234",
			expected:   "2001:db8::1",
		},
		{
			name: "Mixed IPv4 and IPv6",
			headers: map[string]string{
				"X-Forwarded-For": "192.168.1.1, 2001:db8::1, 203.0.113.7",
			},
			remoteAddr: "10.0.0.1:1234",
			expected:   "192.168.1.1",
		},
		{
			name:       "Invalid RemoteAddr",
			headers:    map[string]string{},
			remoteAddr: "invalid-addr",
			expected:   "127.0.0.1",
		},
		{
			name: "All invalid IPs in X-Forwarded-For",
			headers: map[string]string{
				"X-Forwarded-For": "not-ip-1, not-ip-2",
			},
			remoteAddr: "203.0.113.7:1234",
			expected:   "203.0.113.7",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := &http.Request{
				Header:     make(http.Header),
				RemoteAddr: tt.remoteAddr,
			}

			for k, v := range tt.headers {
				req.Header.Set(k, v)
			}

			got := utils.GetClientIP(req, testTrustedProxyConfig())
			if got != tt.expected {
				t.Errorf("GetClientIP() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func BenchmarkGetClientIP(b *testing.B) {
	req := &http.Request{
		Header:     make(http.Header),
		RemoteAddr: "10.0.0.1:1234",
	}
	req.Header.Set("X-Forwarded-For", "192.168.1.1, 203.0.113.7, 35.191.0.1, 10.0.0.1")
	proxyConfig := testTrustedProxyConfig()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		utils.GetClientIP(req, proxyConfig)
	}
}
