#!/bin/bash

# Kill old servers
lsof -ti:8080 2>/dev/null | xargs kill -9 2>/dev/null
sleep 2

# Start server with correct env vars
CONTINUUM_GRPC_URL="100.24.216.168:9090" \
CONTINUUM_REST_URL="http://100.24.216.168:8080/api/v1" \
go run ./cmd/gateway/main.go > /tmp/gateway.log 2>&1 &

SERVER_PID=$!
sleep 6

echo "======================================"
echo "  API Gateway Real Backend Test"
echo "======================================"
echo ""
echo "Server PID: $SERVER_PID"
echo ""

# Run the comprehensive test
./test-real-backends.sh

# Show server logs
echo ""
echo "======================================"
echo "  Server Logs (last 20 lines)"
echo "======================================"
tail -20 /tmp/gateway.log

# Kill server
kill $SERVER_PID 2>/dev/null
