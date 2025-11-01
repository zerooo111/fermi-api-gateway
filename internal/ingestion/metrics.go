package ingestion

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// Metrics holds all Prometheus metrics for the tick ingestion service.
type Metrics struct {
	// Ticks ingested
	TicksTotal *prometheus.CounterVec

	// Buffer utilization
	BufferSize prometheus.Gauge

	// Write performance
	WriteDuration prometheus.Histogram
	BatchSize     prometheus.Histogram

	// Stream reconnections
	StreamReconnects prometheus.Counter

	// Parse errors
	ParseErrors prometheus.Counter

	// Write errors
	WriteErrors prometheus.Counter
}

// NewMetrics creates and registers all Prometheus metrics.
func NewMetrics(namespace string) *Metrics {
	return &Metrics{
		TicksTotal: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: namespace,
				Name:      "ticks_total",
				Help:      "Total number of ticks processed, labeled by status (success/error)",
			},
			[]string{"status"},
		),

		BufferSize: promauto.NewGauge(
			prometheus.GaugeOpts{
				Namespace: namespace,
				Name:      "buffer_size",
				Help:      "Current number of ticks in the buffer",
			},
		),

		WriteDuration: promauto.NewHistogram(
			prometheus.HistogramOpts{
				Namespace: namespace,
				Name:      "write_duration_seconds",
				Help:      "Duration of database write operations in seconds",
				Buckets:   prometheus.DefBuckets, // 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10
			},
		),

		BatchSize: promauto.NewHistogram(
			prometheus.HistogramOpts{
				Namespace: namespace,
				Name:      "batch_size",
				Help:      "Number of ticks per batch write",
				Buckets:   []float64{10, 50, 100, 250, 500, 1000, 2500, 5000},
			},
		),

		StreamReconnects: promauto.NewCounter(
			prometheus.CounterOpts{
				Namespace: namespace,
				Name:      "stream_reconnects_total",
				Help:      "Total number of gRPC stream reconnections",
			},
		),

		ParseErrors: promauto.NewCounter(
			prometheus.CounterOpts{
				Namespace: namespace,
				Name:      "parse_errors_total",
				Help:      "Total number of tick parse errors",
			},
		),

		WriteErrors: promauto.NewCounter(
			prometheus.CounterOpts{
				Namespace: namespace,
				Name:      "write_errors_total",
				Help:      "Total number of database write errors",
			},
		),
	}
}

// RecordTickSuccess increments the success counter.
func (m *Metrics) RecordTickSuccess(count int) {
	m.TicksTotal.WithLabelValues("success").Add(float64(count))
}

// RecordTickError increments the error counter.
func (m *Metrics) RecordTickError() {
	m.TicksTotal.WithLabelValues("error").Inc()
}

// RecordParseError increments the parse error counter.
func (m *Metrics) RecordParseError() {
	m.ParseErrors.Inc()
}

// RecordWriteError increments the write error counter.
func (m *Metrics) RecordWriteError() {
	m.WriteErrors.Inc()
}

// RecordStreamReconnect increments the reconnection counter.
func (m *Metrics) RecordStreamReconnect() {
	m.StreamReconnects.Inc()
}

// SetBufferSize sets the current buffer size.
func (m *Metrics) SetBufferSize(size int) {
	m.BufferSize.Set(float64(size))
}

// ObserveWriteDuration records a write duration.
func (m *Metrics) ObserveWriteDuration(seconds float64) {
	m.WriteDuration.Observe(seconds)
}

// ObserveBatchSize records a batch size.
func (m *Metrics) ObserveBatchSize(size int) {
	m.BatchSize.Observe(float64(size))
}
