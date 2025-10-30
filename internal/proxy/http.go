package proxy

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// HTTPProxy handles HTTP reverse proxying to backend services
type HTTPProxy struct {
	target  string
	timeout time.Duration
	client  *http.Client
}

// NewHTTPProxy creates a new HTTP reverse proxy
func NewHTTPProxy(targetURL string, timeout time.Duration) *HTTPProxy {
	// Create HTTP client with connection pooling and timeout
	client := &http.Client{
		Timeout: timeout,
		Transport: &http.Transport{
			// Connection pooling settings
			MaxIdleConns:        100,
			MaxIdleConnsPerHost: 10,
			IdleConnTimeout:     90 * time.Second,

			// Dial settings
			DialContext: (&net.Dialer{
				Timeout:   5 * time.Second,
				KeepAlive: 30 * time.Second,
			}).DialContext,

			// TLS and other settings
			TLSHandshakeTimeout:   10 * time.Second,
			ResponseHeaderTimeout: timeout,
			ExpectContinueTimeout: 1 * time.Second,
		},
	}

	return &HTTPProxy{
		target:  strings.TrimSuffix(targetURL, "/"),
		timeout: timeout,
		client:  client,
	}
}

// Handler returns an http.Handler that proxies requests to the backend
func (p *HTTPProxy) Handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p.proxyRequest(w, r)
	})
}

// proxyRequest handles the actual proxying logic
func (p *HTTPProxy) proxyRequest(w http.ResponseWriter, r *http.Request) {
	// Build target URL
	targetURL, err := url.Parse(p.target)
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"invalid backend URL: %v"}`, err), http.StatusInternalServerError)
		return
	}

	// Concatenate the base path with the request path
	// If target is "http://backend.com/api/v1", and request is "/health"
	// Result should be "http://backend.com/api/v1/health"
	basePath := strings.TrimSuffix(targetURL.Path, "/")
	requestPath := r.URL.Path
	targetURL.Path = basePath + requestPath
	targetURL.RawQuery = r.URL.RawQuery

	// Create new request to backend
	proxyReq, err := http.NewRequestWithContext(r.Context(), r.Method, targetURL.String(), r.Body)
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"failed to create backend request: %v"}`, err), http.StatusInternalServerError)
		return
	}

	// Copy headers from original request
	copyHeaders(proxyReq.Header, r.Header)

	// Set/override important headers
	proxyReq.Header.Set("X-Forwarded-For", getClientIP(r))
	proxyReq.Header.Set("X-Forwarded-Proto", getScheme(r))
	proxyReq.Header.Set("X-Forwarded-Host", r.Host)

	// Make request to backend
	resp, err := p.client.Do(proxyReq)
	if err != nil {
		// Check if it's a timeout error (and not a connection error)
		if isTimeoutError(err) && !isConnectionError(err) {
			http.Error(w, `{"error":"gateway timeout"}`, http.StatusGatewayTimeout)
			return
		}

		// Other errors (connection refused, DNS failure, etc.)
		http.Error(w, `{"error":"bad gateway"}`, http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	// Copy response headers
	copyHeaders(w.Header(), resp.Header)

	// Copy status code
	w.WriteHeader(resp.StatusCode)

	// Copy response body
	io.Copy(w, resp.Body)
}

// copyHeaders copies HTTP headers from src to dst
func copyHeaders(dst, src http.Header) {
	for key, values := range src {
		// Skip hop-by-hop headers
		if isHopByHopHeader(key) {
			continue
		}
		for _, value := range values {
			dst.Add(key, value)
		}
	}
}

// isHopByHopHeader checks if a header is hop-by-hop (shouldn't be forwarded)
func isHopByHopHeader(header string) bool {
	hopByHop := []string{
		"Connection",
		"Keep-Alive",
		"Proxy-Authenticate",
		"Proxy-Authorization",
		"Te",
		"Trailers",
		"Transfer-Encoding",
		"Upgrade",
	}

	header = strings.ToLower(header)
	for _, h := range hopByHop {
		if strings.ToLower(h) == header {
			return true
		}
	}
	return false
}

// getClientIP extracts the client IP from the request
func getClientIP(r *http.Request) string {
	// Check X-Forwarded-For header first
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		return xff
	}

	// Check X-Real-IP header
	if xri := r.Header.Get("X-Real-IP"); xri != "" {
		return xri
	}

	// Fall back to RemoteAddr
	ip, _, _ := net.SplitHostPort(r.RemoteAddr)
	return ip
}

// getScheme returns the request scheme (http or https)
func getScheme(r *http.Request) string {
	if r.TLS != nil {
		return "https"
	}
	if scheme := r.Header.Get("X-Forwarded-Proto"); scheme != "" {
		return scheme
	}
	return "http"
}

// isTimeoutError checks if an error is a timeout error
func isTimeoutError(err error) bool {
	if err == nil {
		return false
	}

	// Check for net.Error with Timeout() method
	if netErr, ok := err.(net.Error); ok {
		return netErr.Timeout()
	}

	// Check for context deadline exceeded
	return err == context.DeadlineExceeded
}

// isConnectionError checks if an error is a connection error (not timeout)
func isConnectionError(err error) bool {
	if err == nil {
		return false
	}

	// Check for common connection errors
	errStr := err.Error()
	return strings.Contains(errStr, "connection refused") ||
		strings.Contains(errStr, "no such host") ||
		strings.Contains(errStr, "connection reset") ||
		strings.Contains(errStr, "dial tcp")
}
