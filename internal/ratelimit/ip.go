package ratelimit

import (
	"net"
	"net/http"
	"strings"
)

// ExtractIP extracts the client IP address from the request
// Priority: X-Forwarded-For > X-Real-IP > RemoteAddr
func ExtractIP(r *http.Request) string {
	// Try X-Forwarded-For header first (for requests behind proxies)
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		// X-Forwarded-For can contain multiple IPs: "client, proxy1, proxy2"
		// We want the first one (the original client)
		ips := strings.Split(xff, ",")
		if len(ips) > 0 {
			ip := strings.TrimSpace(ips[0])
			if ip != "" {
				return ip
			}
		}
	}

	// Try X-Real-IP header
	if xri := r.Header.Get("X-Real-IP"); xri != "" {
		ip := strings.TrimSpace(xri)
		if ip != "" {
			return ip
		}
	}

	// Fall back to RemoteAddr
	ip := r.RemoteAddr

	// RemoteAddr includes port, strip it
	if host, _, err := net.SplitHostPort(ip); err == nil {
		return host
	}

	// If SplitHostPort fails, return as-is (might be IP without port)
	if ip != "" {
		return ip
	}

	// Fallback for empty RemoteAddr
	return "unknown"
}
