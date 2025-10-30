package ratelimit

import (
	"net/http"
	"testing"
)

func TestExtractIP(t *testing.T) {
	tests := []struct {
		name            string
		remoteAddr      string
		xForwardedFor   string
		xRealIP         string
		expectedIP      string
	}{
		{
			name:       "extracts from RemoteAddr",
			remoteAddr: "192.168.1.1:1234",
			expectedIP: "192.168.1.1",
		},
		{
			name:          "prefers X-Forwarded-For header",
			remoteAddr:    "192.168.1.1:1234",
			xForwardedFor: "203.0.113.1",
			expectedIP:    "203.0.113.1",
		},
		{
			name:          "handles X-Forwarded-For with multiple IPs",
			remoteAddr:    "192.168.1.1:1234",
			xForwardedFor: "203.0.113.1, 198.51.100.1, 192.0.2.1",
			expectedIP:    "203.0.113.1",
		},
		{
			name:       "prefers X-Real-IP over RemoteAddr",
			remoteAddr: "192.168.1.1:1234",
			xRealIP:    "203.0.113.2",
			expectedIP: "203.0.113.2",
		},
		{
			name:          "prefers X-Forwarded-For over X-Real-IP",
			remoteAddr:    "192.168.1.1:1234",
			xForwardedFor: "203.0.113.1",
			xRealIP:       "203.0.113.2",
			expectedIP:    "203.0.113.1",
		},
		{
			name:       "handles IPv6 address",
			remoteAddr: "[2001:db8::1]:1234",
			expectedIP: "2001:db8::1",
		},
		{
			name:       "handles RemoteAddr without port",
			remoteAddr: "192.168.1.1",
			expectedIP: "192.168.1.1",
		},
		{
			name:          "trims whitespace from X-Forwarded-For",
			remoteAddr:    "192.168.1.1:1234",
			xForwardedFor: "  203.0.113.1  ",
			expectedIP:    "203.0.113.1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create request with headers
			req, err := http.NewRequest("GET", "/test", nil)
			if err != nil {
				t.Fatalf("Failed to create request: %v", err)
			}

			req.RemoteAddr = tt.remoteAddr

			if tt.xForwardedFor != "" {
				req.Header.Set("X-Forwarded-For", tt.xForwardedFor)
			}

			if tt.xRealIP != "" {
				req.Header.Set("X-Real-IP", tt.xRealIP)
			}

			// Extract IP
			ip := ExtractIP(req)

			// Verify
			if ip != tt.expectedIP {
				t.Errorf("Expected IP %s, got %s", tt.expectedIP, ip)
			}
		})
	}
}

func TestExtractIP_EmptyRemoteAddr(t *testing.T) {
	req, err := http.NewRequest("GET", "/test", nil)
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}

	req.RemoteAddr = ""

	ip := ExtractIP(req)

	// Should return some fallback value (like "unknown" or empty)
	if ip == "" {
		t.Error("Expected non-empty IP even with empty RemoteAddr")
	}
}
