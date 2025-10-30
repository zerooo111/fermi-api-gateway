package ratelimit

import (
	"testing"
	"time"
)

func TestIPRateLimiter_GetLimiter(t *testing.T) {
	// Create limiter: 10 requests per second, burst of 20
	limiter := NewIPRateLimiter(10, 20)

	// Get limiter for IP
	ip := "192.168.1.1"
	l1 := limiter.GetLimiter(ip)

	if l1 == nil {
		t.Fatal("Expected limiter, got nil")
	}

	// Getting same IP should return same limiter
	l2 := limiter.GetLimiter(ip)
	if l1 != l2 {
		t.Error("Expected same limiter instance for same IP")
	}

	// Different IP should get different limiter
	l3 := limiter.GetLimiter("192.168.1.2")
	if l1 == l3 {
		t.Error("Expected different limiter for different IP")
	}
}

func TestIPRateLimiter_Allow(t *testing.T) {
	// Create strict limiter: 2 requests per second, burst of 2
	limiter := NewIPRateLimiter(2, 2)
	ip := "192.168.1.1"

	// First 2 requests should be allowed (burst)
	if !limiter.Allow(ip) {
		t.Error("Expected first request to be allowed")
	}
	if !limiter.Allow(ip) {
		t.Error("Expected second request to be allowed")
	}

	// Third request should be denied (exceeded burst)
	if limiter.Allow(ip) {
		t.Error("Expected third request to be denied")
	}

	// Wait for rate limit to refill (500ms = 1 request at 2 req/sec)
	time.Sleep(550 * time.Millisecond)

	// Should allow one more request
	if !limiter.Allow(ip) {
		t.Error("Expected request to be allowed after wait")
	}
}

func TestIPRateLimiter_DifferentIPs(t *testing.T) {
	// Create limiter: 1 request per second
	limiter := NewIPRateLimiter(1, 1)

	ip1 := "192.168.1.1"
	ip2 := "192.168.1.2"

	// First request from IP1 should be allowed
	if !limiter.Allow(ip1) {
		t.Error("Expected first request from IP1 to be allowed")
	}

	// First request from IP2 should also be allowed (separate limit)
	if !limiter.Allow(ip2) {
		t.Error("Expected first request from IP2 to be allowed")
	}

	// Second request from IP1 should be denied
	if limiter.Allow(ip1) {
		t.Error("Expected second request from IP1 to be denied")
	}

	// Second request from IP2 should be denied
	if limiter.Allow(ip2) {
		t.Error("Expected second request from IP2 to be denied")
	}
}

func TestIPRateLimiter_Cleanup(t *testing.T) {
	// Create limiter
	limiter := NewIPRateLimiter(10, 10)

	// Add some limiters
	limiter.GetLimiter("192.168.1.1")
	limiter.GetLimiter("192.168.1.2")
	limiter.GetLimiter("192.168.1.3")

	// Check initial count
	limiter.mu.RLock()
	initialCount := len(limiter.limiters)
	limiter.mu.RUnlock()

	if initialCount != 3 {
		t.Errorf("Expected 3 limiters initially, got %d", initialCount)
	}

	// Cleanup is already running in background from NewIPRateLimiter
	// Just verify the limiters exist
	limiter.mu.RLock()
	afterCheck := len(limiter.limiters)
	limiter.mu.RUnlock()

	if afterCheck != 3 {
		t.Errorf("Expected 3 limiters to still exist, got %d", afterCheck)
	}
}

func TestIPRateLimiter_ConcurrentAccess(t *testing.T) {
	limiter := NewIPRateLimiter(100, 100)
	ip := "192.168.1.1"

	// Concurrent access from multiple goroutines
	done := make(chan bool)
	for i := 0; i < 10; i++ {
		go func() {
			for j := 0; j < 10; j++ {
				limiter.Allow(ip)
			}
			done <- true
		}()
	}

	// Wait for all goroutines
	for i := 0; i < 10; i++ {
		<-done
	}

	// Test should complete without panic or race conditions
}
