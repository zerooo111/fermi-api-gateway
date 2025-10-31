package middleware

import (
	"encoding/json"
	"net/http"
	"runtime/debug"
	"strings"

	"go.uber.org/zap"
)

// Recovery middleware recovers from panics and returns a 500 error
func Recovery(logger *zap.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if err := recover(); err != nil {
					stack := string(debug.Stack())
					
					// Parse stack trace to get first few relevant lines
					stackLines := strings.Split(stack, "\n")
					var relevantStack []string
					relevantStack = append(relevantStack, stackLines[0]) // goroutine info
					if len(stackLines) > 1 {
						relevantStack = append(relevantStack, stackLines[1])
					}
					
					// Include only first 10 lines of stack trace
					maxStackLines := 10
					for i := 4; i < len(stackLines) && i < 4+maxStackLines; i++ {
						if strings.TrimSpace(stackLines[i]) != "" {
							relevantStack = append(relevantStack, stackLines[i])
						}
					}
					
					stackSummary := strings.Join(relevantStack, "\n")
					
					// Log with structured logging
					fields := []zap.Field{
						zap.String("method", r.Method),
						zap.String("path", r.URL.Path),
						zap.String("remote_addr", r.RemoteAddr),
						zap.String("panic", toString(err)),
						zap.String("stack", stackSummary),
					}
					
					if requestID := r.Header.Get("X-Request-ID"); requestID != "" {
						fields = append(fields, zap.String("request_id", requestID))
					}
					
					logger.Error("PANIC recovered", fields...)

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
}

// toString safely converts panic value to string
func toString(v interface{}) string {
	switch val := v.(type) {
	case string:
		return val
	case error:
		return val.Error()
	default:
		return "unknown panic type"
	}
}
