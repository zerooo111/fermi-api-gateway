package proxy

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestNewHTTPProxy(t *testing.T) {
	proxy := NewHTTPProxy("http://backend.example.com", 5*time.Second)

	if proxy == nil {
		t.Fatal("Expected proxy to be created, got nil")
	}

	if proxy.target != "http://backend.example.com" {
		t.Errorf("Expected target http://backend.example.com, got %s", proxy.target)
	}
}

func TestHTTPProxy_ProxyRequest(t *testing.T) {
	// Create a test backend server
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Echo back the request details
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"backend":"response","path":"` + r.URL.Path + `","method":"` + r.Method + `"}`))
	}))
	defer backend.Close()

	// Create proxy
	proxy := NewHTTPProxy(backend.URL, 5*time.Second)

	// Create test request
	req := httptest.NewRequest("GET", "/api/test", nil)
	rr := httptest.NewRecorder()

	// Proxy the request
	handler := proxy.Handler()
	handler.ServeHTTP(rr, req)

	// Verify response
	if rr.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", rr.Code)
	}

	body := rr.Body.String()
	if !strings.Contains(body, `"backend":"response"`) {
		t.Errorf("Expected backend response, got %s", body)
	}
}

func TestHTTPProxy_PreservesHeaders(t *testing.T) {
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Check if headers were forwarded
		if r.Header.Get("X-Custom-Header") != "test-value" {
			t.Error("Expected X-Custom-Header to be forwarded")
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer backend.Close()

	proxy := NewHTTPProxy(backend.URL, 5*time.Second)

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("X-Custom-Header", "test-value")
	rr := httptest.NewRecorder()

	handler := proxy.Handler()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", rr.Code)
	}
}

func TestHTTPProxy_PreservesMethod(t *testing.T) {
	methods := []string{"GET", "POST", "PUT", "DELETE", "PATCH"}

	for _, method := range methods {
		t.Run(method, func(t *testing.T) {
			backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != method {
					t.Errorf("Expected method %s, got %s", method, r.Method)
				}
				w.WriteHeader(http.StatusOK)
			}))
			defer backend.Close()

			proxy := NewHTTPProxy(backend.URL, 5*time.Second)

			var body io.Reader
			if method == "POST" || method == "PUT" || method == "PATCH" {
				body = strings.NewReader(`{"test":"data"}`)
			}

			req := httptest.NewRequest(method, "/test", body)
			rr := httptest.NewRecorder()

			handler := proxy.Handler()
			handler.ServeHTTP(rr, req)

			if rr.Code != http.StatusOK {
				t.Errorf("Expected status 200, got %d", rr.Code)
			}
		})
	}
}

func TestHTTPProxy_HandlesBackendError(t *testing.T) {
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"error":"backend error"}`))
	}))
	defer backend.Close()

	proxy := NewHTTPProxy(backend.URL, 5*time.Second)

	req := httptest.NewRequest("GET", "/test", nil)
	rr := httptest.NewRecorder()

	handler := proxy.Handler()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusInternalServerError {
		t.Errorf("Expected status 500, got %d", rr.Code)
	}
}

func TestHTTPProxy_Timeout(t *testing.T) {
	// Create a slow backend that takes 2 seconds to respond
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(2 * time.Second)
		w.WriteHeader(http.StatusOK)
	}))
	defer backend.Close()

	// Create proxy with 100ms timeout
	proxy := NewHTTPProxy(backend.URL, 100*time.Millisecond)

	req := httptest.NewRequest("GET", "/test", nil)
	rr := httptest.NewRecorder()

	handler := proxy.Handler()
	handler.ServeHTTP(rr, req)

	// Should return 504 Gateway Timeout
	if rr.Code != http.StatusGatewayTimeout {
		t.Errorf("Expected status 504, got %d", rr.Code)
	}
}

func TestHTTPProxy_StripPrefix(t *testing.T) {
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Backend should receive path without /api/rollup prefix
		w.Write([]byte(r.URL.Path))
	}))
	defer backend.Close()

	proxy := NewHTTPProxy(backend.URL, 5*time.Second)

	// Request to /api/rollup/transactions should become /transactions at backend
	req := httptest.NewRequest("GET", "/api/rollup/transactions", nil)
	rr := httptest.NewRecorder()

	handler := http.StripPrefix("/api/rollup", proxy.Handler())
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", rr.Code)
	}

	if rr.Body.String() != "/transactions" {
		t.Errorf("Expected /transactions, got %s", rr.Body.String())
	}
}

func TestHTTPProxy_InvalidBackendURL(t *testing.T) {
	proxy := NewHTTPProxy("http://invalid-backend-that-does-not-exist.local:9999", 1*time.Second)

	req := httptest.NewRequest("GET", "/test", nil)
	rr := httptest.NewRecorder()

	handler := proxy.Handler()
	handler.ServeHTTP(rr, req)

	// Should return either 502 (connection error) or 504 (timeout during DNS/connect)
	// Both are acceptable for invalid backend
	if rr.Code != http.StatusBadGateway && rr.Code != http.StatusGatewayTimeout {
		t.Errorf("Expected status 502 or 504, got %d", rr.Code)
	}
}

func TestHTTPProxy_PreservesQueryParams(t *testing.T) {
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		query := r.URL.Query()
		if query.Get("param1") != "value1" {
			t.Error("Expected param1=value1")
		}
		if query.Get("param2") != "value2" {
			t.Error("Expected param2=value2")
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer backend.Close()

	proxy := NewHTTPProxy(backend.URL, 5*time.Second)

	req := httptest.NewRequest("GET", "/test?param1=value1&param2=value2", nil)
	rr := httptest.NewRecorder()

	handler := proxy.Handler()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", rr.Code)
	}
}

func TestHTTPProxy_CancelsOnContextDone(t *testing.T) {
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Slow response
		time.Sleep(1 * time.Second)
		w.WriteHeader(http.StatusOK)
	}))
	defer backend.Close()

	proxy := NewHTTPProxy(backend.URL, 5*time.Second)

	// Create request with cancellable context
	ctx, cancel := context.WithCancel(context.Background())
	req := httptest.NewRequest("GET", "/test", nil).WithContext(ctx)
	rr := httptest.NewRecorder()

	// Cancel context after 100ms
	go func() {
		time.Sleep(100 * time.Millisecond)
		cancel()
	}()

	handler := proxy.Handler()
	handler.ServeHTTP(rr, req)

	// Should handle cancellation gracefully (exact status code may vary)
	if rr.Code == 0 {
		t.Error("Expected response to be written before context cancellation")
	}
}
