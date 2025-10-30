package middleware

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"net/http"
)

// ContextKey is a type for context keys to avoid collisions
type ContextKey string

// RequestIDKey is the context key for request IDs
const RequestIDKey ContextKey = "request-id"

// RequestID middleware generates or extracts a request ID for tracking
// If X-Request-ID header exists, it uses that, otherwise generates a new one
func RequestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Check if request ID already exists in header
		requestID := r.Header.Get("X-Request-ID")

		// Generate new ID if none exists
		if requestID == "" {
			requestID = generateRequestID()
		}

		// Set request ID in header for downstream handlers
		r.Header.Set("X-Request-ID", requestID)

		// Set request ID in response header
		w.Header().Set("X-Request-ID", requestID)

		// Add request ID to context
		ctx := context.WithValue(r.Context(), RequestIDKey, requestID)
		r = r.WithContext(ctx)

		// Continue to next handler
		next.ServeHTTP(w, r)
	})
}

// generateRequestID creates a random request ID
func generateRequestID() string {
	bytes := make([]byte, 16)
	if _, err := rand.Read(bytes); err != nil {
		// Fallback to a simpler ID if random generation fails
		return "fallback-request-id"
	}
	return hex.EncodeToString(bytes)
}
