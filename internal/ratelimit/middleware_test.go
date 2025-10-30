package ratelimit

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRateLimitMiddleware(t *testing.T) {
	// Create strict limiter: 2 requests per second, burst of 2
	limiter := NewIPRateLimiter(2, 2)

	// Create test handler
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("success"))
	})

	// Wrap with middleware
	handler := Middleware(limiter)(testHandler)

	// First two requests should succeed
	for i := 0; i < 2; i++ {
		req := httptest.NewRequest("GET", "/test", nil)
		req.RemoteAddr = "192.168.1.1:1234"

		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("Request %d: expected status 200, got %d", i+1, rr.Code)
		}
	}

	// Third request should be rate limited
	req := httptest.NewRequest("GET", "/test", nil)
	req.RemoteAddr = "192.168.1.1:1234"

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusTooManyRequests {
		t.Errorf("Expected status 429, got %d", rr.Code)
	}

	// Check response body
	if rr.Header().Get("Content-Type") != "application/json" {
		t.Error("Expected JSON content type")
	}
}

func TestRateLimitMiddleware_DifferentIPs(t *testing.T) {
	// Create limiter: 1 request per second
	limiter := NewIPRateLimiter(1, 1)

	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	handler := Middleware(limiter)(testHandler)

	// Request from IP1
	req1 := httptest.NewRequest("GET", "/test", nil)
	req1.RemoteAddr = "192.168.1.1:1234"

	rr1 := httptest.NewRecorder()
	handler.ServeHTTP(rr1, req1)

	if rr1.Code != http.StatusOK {
		t.Errorf("Expected IP1 first request to succeed, got %d", rr1.Code)
	}

	// Request from IP2 (should also succeed - separate limit)
	req2 := httptest.NewRequest("GET", "/test", nil)
	req2.RemoteAddr = "192.168.1.2:1234"

	rr2 := httptest.NewRecorder()
	handler.ServeHTTP(rr2, req2)

	if rr2.Code != http.StatusOK {
		t.Errorf("Expected IP2 first request to succeed, got %d", rr2.Code)
	}
}

func TestRateLimitMiddleware_Headers(t *testing.T) {
	limiter := NewIPRateLimiter(10, 10)

	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	handler := Middleware(limiter)(testHandler)

	req := httptest.NewRequest("GET", "/test", nil)
	req.RemoteAddr = "192.168.1.1:1234"

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	// Should have rate limit headers
	if rr.Header().Get("X-RateLimit-Limit") == "" {
		t.Error("Expected X-RateLimit-Limit header")
	}

	if rr.Header().Get("X-RateLimit-Remaining") == "" {
		t.Error("Expected X-RateLimit-Remaining header")
	}
}

func TestRateLimitMiddleware_XForwardedFor(t *testing.T) {
	// Create strict limiter
	limiter := NewIPRateLimiter(1, 1)

	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	handler := Middleware(limiter)(testHandler)

	// First request with X-Forwarded-For
	req1 := httptest.NewRequest("GET", "/test", nil)
	req1.RemoteAddr = "192.168.1.1:1234"
	req1.Header.Set("X-Forwarded-For", "203.0.113.1")

	rr1 := httptest.NewRecorder()
	handler.ServeHTTP(rr1, req1)

	if rr1.Code != http.StatusOK {
		t.Error("Expected first request to succeed")
	}

	// Second request with same X-Forwarded-For (should be limited)
	req2 := httptest.NewRequest("GET", "/test", nil)
	req2.RemoteAddr = "192.168.1.2:1234" // Different RemoteAddr
	req2.Header.Set("X-Forwarded-For", "203.0.113.1") // Same forwarded IP

	rr2 := httptest.NewRecorder()
	handler.ServeHTTP(rr2, req2)

	if rr2.Code != http.StatusTooManyRequests {
		t.Errorf("Expected second request to be rate limited, got %d", rr2.Code)
	}
}
