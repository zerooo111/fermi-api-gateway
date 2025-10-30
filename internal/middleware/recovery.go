package middleware

import (
	"encoding/json"
	"fmt"
	"net/http"
	"runtime/debug"
)

// Recovery middleware recovers from panics and returns a 500 error
func Recovery(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if err := recover(); err != nil {
				// Log the panic and stack trace
				fmt.Printf("PANIC: %v\n", err)
				fmt.Printf("Stack trace:\n%s\n", debug.Stack())

				// Set content type to JSON
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusInternalServerError)

				// Create error response
				response := map[string]interface{}{
					"error":   "Internal Server Error",
					"message": "An unexpected error occurred",
				}

				// Include request ID if available
				if requestID := r.Header.Get("X-Request-ID"); requestID != "" {
					response["request_id"] = requestID
				}

				// Write JSON response
				json.NewEncoder(w).Encode(response)
			}
		}()

		next.ServeHTTP(w, r)
	})
}
