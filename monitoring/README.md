# Tick Ingestion Service Monitoring

## Metrics Endpoint

The service exposes Prometheus metrics on the health check port (default: 8081):

```bash
curl http://localhost:8081/metrics
```

## Available Metrics

### Throughput Metrics

**`tick_ingester_ticks_total{status}`**
- Counter of total ticks processed
- Labels: `status="success"` or `status="error"`
- Use `rate()` to get ticks/second

### Buffer Metrics

**`tick_ingester_buffer_size`**
- Gauge of current tick buffer size
- Monitor for backpressure (should stay below buffer capacity)

### Performance Metrics

**`tick_ingester_write_duration_seconds`**
- Histogram of database write latencies
- Buckets: 5ms, 10ms, 25ms, 50ms, 100ms, 250ms, 500ms, 1s, 2.5s, 5s, 10s
- Use `histogram_quantile()` for percentiles

**`tick_ingester_batch_size`**
- Histogram of batch sizes
- Buckets: 10, 50, 100, 250, 500, 1000, 2500, 5000
- Monitor batch efficiency

### Error Metrics

**`tick_ingester_parse_errors_total`**
- Counter of tick parse failures
- Should be very low (<0.01% of total ticks)

**`tick_ingester_write_errors_total`**
- Counter of database write failures
- Indicates DB connectivity or performance issues

**`tick_ingester_stream_reconnects_total`**
- Counter of gRPC stream reconnections
- Monitor for network instability

## Prometheus Setup

### 1. Install Prometheus

```bash
# macOS
brew install prometheus

# Linux
wget https://github.com/prometheus/prometheus/releases/download/v2.45.0/prometheus-2.45.0.linux-amd64.tar.gz
tar xvf prometheus-*.tar.gz
cd prometheus-*
```

### 2. Configure Prometheus

Use the provided configuration:

```bash
cp monitoring/prometheus.yml /path/to/prometheus/
prometheus --config.file=prometheus.yml
```

### 3. Access Prometheus UI

Open http://localhost:9090

## Grafana Dashboard

### 1. Install Grafana

```bash
# macOS
brew install grafana
brew services start grafana

# Linux
sudo systemctl start grafana-server
```

### 2. Add Prometheus Data Source

1. Open Grafana: http://localhost:3000 (admin/admin)
2. Configuration → Data Sources → Add data source
3. Select "Prometheus"
4. URL: `http://localhost:9090`
5. Save & Test

### 3. Import Dashboard

1. Dashboards → Import
2. Upload `monitoring/grafana-dashboard.json`
3. Select Prometheus data source
4. Import

## Key Queries

### Throughput

```promql
# Ticks ingested per second
rate(tick_ingester_ticks_total{status="success"}[1m])

# Total ticks processed
sum(tick_ingester_ticks_total)
```

### Latency

```promql
# p50 write latency
histogram_quantile(0.50, rate(tick_ingester_write_duration_seconds_bucket[5m]))

# p95 write latency
histogram_quantile(0.95, rate(tick_ingester_write_duration_seconds_bucket[5m]))

# p99 write latency
histogram_quantile(0.99, rate(tick_ingester_write_duration_seconds_bucket[5m]))
```

### Errors

```promql
# Error rate
rate(tick_ingester_ticks_total{status="error"}[1m])

# Parse error percentage
rate(tick_ingester_parse_errors_total[5m]) / rate(tick_ingester_ticks_total[5m]) * 100

# Write error rate
rate(tick_ingester_write_errors_total[1m])
```

### Buffer Health

```promql
# Current buffer size
tick_ingester_buffer_size

# Buffer utilization percentage (if max is 10000)
tick_ingester_buffer_size / 10000 * 100
```

### Batch Efficiency

```promql
# Average batch size
rate(tick_ingester_batch_size_sum[5m]) / rate(tick_ingester_batch_size_count[5m])

# p95 batch size
histogram_quantile(0.95, rate(tick_ingester_batch_size_bucket[5m]))
```

## Alerting Rules

### High Error Rate

```yaml
- alert: HighTickErrorRate
  expr: rate(tick_ingester_ticks_total{status="error"}[5m]) > 10
  for: 2m
  annotations:
    summary: "High tick ingestion error rate"
    description: "Error rate is {{ $value }} ticks/sec"
```

### High Write Latency

```yaml
- alert: HighWriteLatency
  expr: histogram_quantile(0.95, rate(tick_ingester_write_duration_seconds_bucket[5m])) > 1.0
  for: 5m
  annotations:
    summary: "High database write latency"
    description: "p95 latency is {{ $value }}s"
```

### Buffer Near Capacity

```yaml
- alert: BufferNearCapacity
  expr: tick_ingester_buffer_size > 9000
  for: 1m
  annotations:
    summary: "Tick buffer near capacity"
    description: "Buffer size is {{ $value }} (max 10000)"
```

### Frequent Reconnections

```yaml
- alert: FrequentStreamReconnects
  expr: rate(tick_ingester_stream_reconnects_total[5m]) > 0.1
  for: 5m
  annotations:
    summary: "Frequent gRPC stream reconnections"
    description: "Reconnecting {{ $value }} times/sec"
```

## Performance Targets

| Metric | Target | Alert Threshold |
|--------|--------|-----------------|
| Throughput | 10,000 ticks/sec | < 9,000 ticks/sec |
| Write Latency (p95) | < 100ms | > 500ms |
| Error Rate | < 0.01% | > 1% |
| Buffer Utilization | < 50% | > 90% |
| Parse Errors | 0 | > 10/sec |
| Write Errors | 0 | > 1/sec |

## Troubleshooting with Metrics

### Symptom: Low Throughput

```promql
# Check if buffer is empty (upstream issue)
tick_ingester_buffer_size

# Check write latency (database issue)
histogram_quantile(0.95, rate(tick_ingester_write_duration_seconds_bucket[5m]))

# Check batch sizes (tuning issue)
rate(tick_ingester_batch_size_sum[5m]) / rate(tick_ingester_batch_size_count[5m])
```

### Symptom: High Memory Usage

```promql
# Check buffer size
tick_ingester_buffer_size

# If buffer is full, check write errors
rate(tick_ingester_write_errors_total[1m])
```

### Symptom: Data Loss

```promql
# Check error rates
rate(tick_ingester_ticks_total{status="error"}[5m])

# Check reconnections (stream interruptions)
rate(tick_ingester_stream_reconnects_total[5m])

# Check write errors
rate(tick_ingester_write_errors_total[1m])
```

## Metrics Export

Export metrics for analysis:

```bash
# Export current metrics
curl -s http://localhost:8081/metrics > metrics-$(date +%Y%m%d-%H%M%S).txt

# Monitor in real-time
watch -n 1 'curl -s http://localhost:8081/metrics | grep tick_ingester'
```
