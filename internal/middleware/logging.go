package middleware

import (
	"net/http"
	"time"

	"go.uber.org/zap"
)

// responseWriter wraps http.ResponseWriter to capture status code
type responseWriter struct {
	http.ResponseWriter
	statusCode int
	written    bool
}

func (rw *responseWriter) WriteHeader(code int) {
	if !rw.written {
		rw.statusCode = code
		rw.written = true
		rw.ResponseWriter.WriteHeader(code)
	}
}

func (rw *responseWriter) Write(b []byte) (int, error) {
	if !rw.written {
		rw.WriteHeader(http.StatusOK)
	}
	return rw.ResponseWriter.Write(b)
}

// Flush implements http.Flusher interface for SSE support
func (rw *responseWriter) Flush() {
	if flusher, ok := rw.ResponseWriter.(http.Flusher); ok {
		flusher.Flush()
	}
}

// Logging middleware logs HTTP requests with structured logging
func Logging(logger *zap.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()

			// Wrap response writer to capture status code
			wrapped := &responseWriter{
				ResponseWriter: w,
				statusCode:     http.StatusOK,
				written:        false,
			}

			// Process request
			next.ServeHTTP(wrapped, r)

			// Calculate duration
			duration := time.Since(start)

			// Build log fields
			fields := []zap.Field{
				zap.String("method", r.Method),
				zap.String("path", r.URL.Path),
				zap.Int("status", wrapped.statusCode),
				zap.Duration("duration", duration),
				zap.String("remote_addr", r.RemoteAddr),
			}

			// Add request ID if available
			if requestID := r.Header.Get("X-Request-ID"); requestID != "" {
				fields = append(fields, zap.String("request_id", requestID))
			}

			// Add user agent
			if userAgent := r.Header.Get("User-Agent"); userAgent != "" {
				fields = append(fields, zap.String("user_agent", userAgent))
			}

			// Log at appropriate level based on status code
			// Use shorter, cleaner messages for common requests
			if wrapped.statusCode >= 500 {
				logger.Error("HTTP request", fields...)
			} else if wrapped.statusCode >= 400 {
				// Suppress verbose logging for 404s - they're usually not critical
				if wrapped.statusCode == 404 {
					// Don't include stack trace info for 404s
					logger.Debug("HTTP request", fields...)
				} else {
					logger.Warn("HTTP request", fields...)
				}
			} else {
				logger.Info("HTTP request", fields...)
			}
		})
	}
}
