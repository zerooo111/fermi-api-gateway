package middleware

import (
	"net/http"
	"strconv"
	"time"

	"github.com/fermilabs/fermi-api-gateway/internal/metrics"
)

// metricsResponseWriter wraps http.ResponseWriter to capture response size and status
type metricsResponseWriter struct {
	http.ResponseWriter
	statusCode   int
	bytesWritten int
}

func (mrw *metricsResponseWriter) WriteHeader(code int) {
	mrw.statusCode = code
	mrw.ResponseWriter.WriteHeader(code)
}

func (mrw *metricsResponseWriter) Write(b []byte) (int, error) {
	n, err := mrw.ResponseWriter.Write(b)
	mrw.bytesWritten += n
	return n, err
}

// Metrics middleware records HTTP metrics
func Metrics(m *metrics.Metrics) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()

			// Wrap response writer
			mrw := &metricsResponseWriter{
				ResponseWriter: w,
				statusCode:     http.StatusOK, // Default status
				bytesWritten:   0,
			}

			// Record request size
			requestSize := float64(r.ContentLength)
			if requestSize > 0 {
				m.RequestSize.WithLabelValues(r.Method, r.URL.Path).Observe(requestSize)
			}

			// Process request
			next.ServeHTTP(mrw, r)

			// Calculate duration
			duration := time.Since(start).Seconds()

			// Convert status code to string
			statusCode := strconv.Itoa(mrw.statusCode)

			// Record metrics
			m.RequestsTotal.WithLabelValues(r.Method, r.URL.Path, statusCode).Inc()
			m.RequestDuration.WithLabelValues(r.Method, r.URL.Path, statusCode).Observe(duration)
			m.ResponseSize.WithLabelValues(r.Method, r.URL.Path, statusCode).Observe(float64(mrw.bytesWritten))
		})
	}
}
