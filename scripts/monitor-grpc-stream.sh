#!/bin/bash

# Monitor gRPC stream over time
# Usage: ./scripts/monitor-grpc-stream.sh [duration_in_seconds]

DURATION=${1:-60}  # Default 60 seconds
SERVER="${CONTINUUM_GRPC_URL:-100.24.216.168:9090}"
BINARY="./bin/test-grpc-stream"

# Build if not exists
if [ ! -f "$BINARY" ]; then
    echo "Building test tool..."
    go build -o "$BINARY" ./cmd/test-grpc-stream
fi

echo "════════════════════════════════════════════════════════════════"
echo "  gRPC Stream Monitor"
echo "════════════════════════════════════════════════════════════════"
echo "Server:   $SERVER"
echo "Duration: ${DURATION}s"
echo "Started:  $(date '+%Y-%m-%d %H:%M:%S')"
echo "════════════════════════════════════════════════════════════════"
echo ""

ITERATION=1
START_TIME=$(date +%s)

while true; do
    CURRENT_TIME=$(date +%s)
    ELAPSED=$((CURRENT_TIME - START_TIME))

    if [ $ELAPSED -ge $DURATION ]; then
        echo ""
        echo "════════════════════════════════════════════════════════════════"
        echo "Monitoring complete after ${ELAPSED}s"
        echo "Total iterations: $ITERATION"
        break
    fi

    echo "[$(date '+%H:%M:%S')] Iteration #$ITERATION (elapsed: ${ELAPSED}s)"
    echo "─────────────────────────────────────────────────────────────────"

    # Run test and capture output
    OUTPUT=$($BINARY -server "$SERVER" 2>&1)

    # Extract key metrics
    TICKS=$(echo "$OUTPUT" | grep "Ticks received:" | awk '{print $3}')
    RATE=$(echo "$OUTPUT" | grep "Rate:" | awk '{print $2}')
    TICK_RANGE=$(echo "$OUTPUT" | grep "Tick range:" | awk '{print $3, $4, $5}')
    SEQUENTIAL=$(echo "$OUTPUT" | grep "Sequential")

    echo "  Ticks: $TICKS | Rate: $RATE tps | Range: $TICK_RANGE"

    if echo "$OUTPUT" | grep -q "Skipped ticks"; then
        GAPS=$(echo "$OUTPUT" | grep "Skipped ticks" | awk '{print $5}')
        echo "  ⚠️  WARNING: $GAPS gaps detected!"
    elif echo "$OUTPUT" | grep -q "Sequential"; then
        echo "  ✓ All sequential"
    fi

    # Check for errors
    if echo "$OUTPUT" | grep -q "ERROR"; then
        echo "  ❌ ERROR detected:"
        echo "$OUTPUT" | grep "ERROR" | head -3
    fi

    echo ""

    ITERATION=$((ITERATION + 1))

    # Small delay between iterations
    sleep 2
done

echo "════════════════════════════════════════════════════════════════"
