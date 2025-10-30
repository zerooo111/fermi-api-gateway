package ratelimit

import (
	"encoding/json"
	"fmt"
	"net/http"
)

// Middleware creates a rate limiting middleware
func Middleware(limiter *IPRateLimiter) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Extract client IP
			ip := ExtractIP(r)

			// Get limiter for this IP
			l := limiter.GetLimiter(ip)

			// Check if request is allowed
			if !l.Allow() {
				// Set rate limit headers
				w.Header().Set("X-RateLimit-Limit", fmt.Sprintf("%v", limiter.burst))
				w.Header().Set("X-RateLimit-Remaining", "0")
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusTooManyRequests)

				// Return error response
				response := map[string]interface{}{
					"error":   "Rate Limit Exceeded",
					"message": "Too many requests. Please try again later.",
				}

				// Include request ID if available
				if requestID := r.Header.Get("X-Request-ID"); requestID != "" {
					response["request_id"] = requestID
				}

				json.NewEncoder(w).Encode(response)
				return
			}

			// Request allowed - set rate limit headers
			tokens := int(l.Tokens())
			w.Header().Set("X-RateLimit-Limit", fmt.Sprintf("%d", limiter.burst))
			w.Header().Set("X-RateLimit-Remaining", fmt.Sprintf("%d", tokens))

			// Continue to next handler
			next.ServeHTTP(w, r)
		})
	}
}
