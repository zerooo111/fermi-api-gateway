package ratelimit

import (
	"sync"
	"time"

	"golang.org/x/time/rate"
)

// ipLimiter holds a rate limiter and the last time it was used
type ipLimiter struct {
	limiter  *rate.Limiter
	lastSeen time.Time
}

// IPRateLimiter manages rate limiters for different IP addresses
type IPRateLimiter struct {
	mu              sync.RWMutex
	limiters        map[string]*ipLimiter
	rate            rate.Limit
	burst           int
	cleanupInterval time.Duration
}

// NewIPRateLimiter creates a new IP-based rate limiter
// rate: requests per second
// burst: maximum burst size
func NewIPRateLimiter(r float64, b int) *IPRateLimiter {
	limiter := &IPRateLimiter{
		limiters:        make(map[string]*ipLimiter),
		rate:            rate.Limit(r),
		burst:           b,
		cleanupInterval: 5 * time.Minute,
	}

	// Start cleanup goroutine
	go limiter.cleanup(make(chan struct{}))

	return limiter
}

// GetLimiter returns the rate limiter for the given IP
func (i *IPRateLimiter) GetLimiter(ip string) *rate.Limiter {
	i.mu.Lock()
	defer i.mu.Unlock()

	limiterInfo, exists := i.limiters[ip]
	if !exists {
		limiterInfo = &ipLimiter{
			limiter:  rate.NewLimiter(i.rate, i.burst),
			lastSeen: time.Now(),
		}
		i.limiters[ip] = limiterInfo
	} else {
		// Update last seen time
		limiterInfo.lastSeen = time.Now()
	}

	return limiterInfo.limiter
}

// Allow checks if a request from the given IP should be allowed
func (i *IPRateLimiter) Allow(ip string) bool {
	limiter := i.GetLimiter(ip)
	return limiter.Allow()
}

// cleanup removes old unused limiters to prevent memory leaks
func (i *IPRateLimiter) cleanup(stop chan struct{}) {
	ticker := time.NewTicker(i.cleanupInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			i.mu.Lock()
			for ip, limiter := range i.limiters {
				// Remove limiters not used in the last hour
				if time.Since(limiter.lastSeen) > 1*time.Hour {
					delete(i.limiters, ip)
				}
			}
			i.mu.Unlock()
		case <-stop:
			return
		}
	}
}
