#!/bin/bash
# Load Testing Script for Tick Ingestion Service
# Tests the service under high load to validate 10k ticks/sec target

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Configuration
SERVICE_URL="http://localhost:8081"
DURATION_SECONDS=60
TARGET_TPS=10000

echo -e "${GREEN}=== Tick Ingestion Service Load Test ===${NC}"
echo ""

# Check if service is running
echo -e "${YELLOW}1. Checking service health...${NC}"
if ! curl -s "${SERVICE_URL}/health" > /dev/null 2>&1; then
    echo -e "${RED}✗ Service is not responding at ${SERVICE_URL}${NC}"
    echo "Please start the service first:"
    echo "  ./bin/tick-ingester"
    exit 1
fi
echo -e "${GREEN}✓ Service is healthy${NC}"
echo ""

# Get baseline metrics
echo -e "${YELLOW}2. Collecting baseline metrics...${NC}"
BASELINE_TICKS=$(curl -s "${SERVICE_URL}/metrics" | grep "tick_ingester_ticks_total{status=\"success\"}" | awk '{print $2}' || echo "0")
echo "Baseline ticks: ${BASELINE_TICKS}"
echo ""

# Run load test
echo -e "${YELLOW}3. Running load test...${NC}"
echo "Duration: ${DURATION_SECONDS} seconds"
echo "Target: ${TARGET_TPS} ticks/sec"
echo ""

START_TIME=$(date +%s)

# Monitor metrics during test
for i in $(seq 1 $DURATION_SECONDS); do
    sleep 1

    # Get current metrics
    CURRENT_TICKS=$(curl -s "${SERVICE_URL}/metrics" | grep "tick_ingester_ticks_total{status=\"success\"}" | awk '{print $2}' || echo "$BASELINE_TICKS")
    CURRENT_ERRORS=$(curl -s "${SERVICE_URL}/metrics" | grep "tick_ingester_ticks_total{status=\"error\"}" | awk '{print $2}' || echo "0")
    BUFFER_SIZE=$(curl -s "${SERVICE_URL}/metrics" | grep "^tick_ingester_buffer_size " | awk '{print $2}' || echo "0")

    # Calculate rate
    ELAPSED=$((i))
    TOTAL_TICKS=$((CURRENT_TICKS - BASELINE_TICKS))
    RATE=$((TOTAL_TICKS / ELAPSED))

    # Progress bar
    PROGRESS=$((i * 100 / DURATION_SECONDS))
    printf "\rProgress: [%-50s] %d%% | Rate: %d tps | Buffer: %s | Errors: %s" \
        "$(printf '#%.0s' $(seq 1 $((PROGRESS / 2))))" \
        "$PROGRESS" \
        "$RATE" \
        "$BUFFER_SIZE" \
        "$CURRENT_ERRORS"
done

echo ""
echo ""

END_TIME=$(date +%s)
ACTUAL_DURATION=$((END_TIME - START_TIME))

# Collect final metrics
echo -e "${YELLOW}4. Analyzing results...${NC}"
FINAL_TICKS=$(curl -s "${SERVICE_URL}/metrics" | grep "tick_ingester_ticks_total{status=\"success\"}" | awk '{print $2}' || echo "$BASELINE_TICKS")
FINAL_ERRORS=$(curl -s "${SERVICE_URL}/metrics" | grep "tick_ingester_ticks_total{status=\"error\"}" | awk '{print $2}' || echo "0")
PARSE_ERRORS=$(curl -s "${SERVICE_URL}/metrics" | grep "tick_ingester_parse_errors_total" | awk '{print $2}' || echo "0")
WRITE_ERRORS=$(curl -s "${SERVICE_URL}/metrics" | grep "tick_ingester_write_errors_total" | awk '{print $2}' || echo "0")

TOTAL_PROCESSED=$((FINAL_TICKS - BASELINE_TICKS))
AVERAGE_TPS=$((TOTAL_PROCESSED / ACTUAL_DURATION))

# Get latency percentiles from histogram
P50_LATENCY=$(curl -s "${SERVICE_URL}/metrics" | grep "tick_ingester_write_duration_seconds_bucket" | head -1 | awk '{print $2}')
P95_LATENCY=$(curl -s "${SERVICE_URL}/metrics" | grep "tick_ingester_write_duration_seconds_bucket" | tail -3 | head -1 | awk '{print $2}')

echo ""
echo -e "${GREEN}=== Load Test Results ===${NC}"
echo ""
echo "Duration:          ${ACTUAL_DURATION} seconds"
echo "Ticks Processed:   ${TOTAL_PROCESSED}"
echo "Average TPS:       ${AVERAGE_TPS}"
echo "Target TPS:        ${TARGET_TPS}"
echo ""
echo "Errors:"
echo "  Total Errors:    ${FINAL_ERRORS}"
echo "  Parse Errors:    ${PARSE_ERRORS}"
echo "  Write Errors:    ${WRITE_ERRORS}"
echo ""

# Performance evaluation
echo -e "${GREEN}=== Performance Evaluation ===${NC}"
echo ""

if [ "$AVERAGE_TPS" -ge "$TARGET_TPS" ]; then
    echo -e "${GREEN}✓ PASS: Throughput meets target (${AVERAGE_TPS} >= ${TARGET_TPS} tps)${NC}"
else
    echo -e "${RED}✗ FAIL: Throughput below target (${AVERAGE_TPS} < ${TARGET_TPS} tps)${NC}"
fi

ERROR_RATE=$(awk "BEGIN {print ($FINAL_ERRORS / $TOTAL_PROCESSED) * 100}")
if (( $(echo "$ERROR_RATE < 1.0" | bc -l) )); then
    echo -e "${GREEN}✓ PASS: Error rate acceptable (${ERROR_RATE}% < 1%)${NC}"
else
    echo -e "${RED}✗ FAIL: Error rate too high (${ERROR_RATE}% >= 1%)${NC}"
fi

echo ""
echo -e "${YELLOW}For detailed metrics, visit: ${SERVICE_URL}/metrics${NC}"
echo ""

# Save results
RESULTS_FILE="loadtest-results-$(date +%Y%m%d-%H%M%S).txt"
cat > "$RESULTS_FILE" << EOF
Tick Ingestion Service Load Test Results
========================================
Date: $(date)
Duration: ${ACTUAL_DURATION}s
Target TPS: ${TARGET_TPS}

Results:
  Ticks Processed: ${TOTAL_PROCESSED}
  Average TPS: ${AVERAGE_TPS}
  Total Errors: ${FINAL_ERRORS}
  Parse Errors: ${PARSE_ERRORS}
  Write Errors: ${WRITE_ERRORS}
  Error Rate: ${ERROR_RATE}%

Status: $([ "$AVERAGE_TPS" -ge "$TARGET_TPS" ] && echo "PASS" || echo "FAIL")
EOF

echo -e "${GREEN}Results saved to: ${RESULTS_FILE}${NC}"
