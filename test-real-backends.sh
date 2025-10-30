#!/bin/bash

# Colors
GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

BASE_URL="http://localhost:8080"

echo -e "${BLUE}========================================${NC}"
echo -e "${BLUE}  Real Backend Integration Test Report ${NC}"
echo -e "${BLUE}========================================${NC}\n"

# Test function
test_api() {
    local name="$1"
    local method="$2"
    local url="$3"
    local data="$4"
    local expected_status="$5"

    echo -e "${YELLOW}Testing:${NC} $name"

    if [ "$method" == "POST" ]; then
        response=$(curl -s -w "\n%{http_code}\n%{time_total}" -X POST \
            -H "Content-Type: application/json" \
            -d "$data" \
            "$url" 2>&1)
    else
        response=$(curl -s -w "\n%{http_code}\n%{time_total}" -X GET "$url" 2>&1)
    fi

    http_code=$(echo "$response" | tail -2 | head -1)
    time_total=$(echo "$response" | tail -1)
    body=$(echo "$response" | head -n -2)

    latency_ms=$(echo "$time_total * 1000" | bc)

    if [ "$http_code" == "$expected_status" ]; then
        echo -e "  ${GREEN}✓ PASS${NC} HTTP $http_code (${latency_ms}ms)"
        echo -e "  Response: ${body:0:150}..."
    else
        echo -e "  ${RED}✗ FAIL${NC} Expected $expected_status, got $http_code (${latency_ms}ms)"
        echo -e "  Response: $body"
    fi
    echo ""
}

echo -e "${BLUE}=== Core Gateway Endpoints ===${NC}\n"

test_api "Health Check" "GET" "$BASE_URL/health" "" "200"
test_api "Ready Check" "GET" "$BASE_URL/ready" "" "200"
test_api "Root Info" "GET" "$BASE_URL/" "" "200"

echo -e "${BLUE}=== Continuum REST Proxy ===${NC}\n"

test_api "REST: Health" "GET" "$BASE_URL/api/continuum/rest/health" "" "200"
test_api "REST: Status" "GET" "$BASE_URL/api/continuum/rest/status" "" "200"
test_api "REST: Sequencer Status" "GET" "$BASE_URL/api/continuum/rest/sequencer/status" "" "200"
test_api "REST: Recent Ticks" "GET" "$BASE_URL/api/continuum/rest/ticks/recent" "" "200"
test_api "REST: Recent Transactions" "GET" "$BASE_URL/api/continuum/rest/tx/recent" "" "200"
test_api "REST: Chain State" "GET" "$BASE_URL/api/continuum/rest/chain/state" "" "200"

echo -e "${BLUE}=== Continuum gRPC Proxy ===${NC}\n"

test_api "gRPC: Get Status" "GET" "$BASE_URL/api/continuum/grpc/status" "" "200"
test_api "gRPC: Get Chain State (default)" "GET" "$BASE_URL/api/continuum/grpc/chain-state" "" "200"
test_api "gRPC: Get Chain State (limit=5)" "GET" "$BASE_URL/api/continuum/grpc/chain-state?tick_limit=5" "" "200"
test_api "gRPC: Get Tick #1" "GET" "$BASE_URL/api/continuum/grpc/tick?number=1" "" "200"
test_api "gRPC: Get Tick #100" "GET" "$BASE_URL/api/continuum/grpc/tick?number=100" "" "200"

echo -e "${BLUE}========================================${NC}"
echo -e "${GREEN}Test Complete!${NC}"
echo -e "${BLUE}========================================${NC}"
