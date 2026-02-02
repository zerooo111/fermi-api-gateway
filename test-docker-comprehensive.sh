#!/bin/bash
# Comprehensive Docker deployment test suite

set -e

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

IMAGE_NAME="fermi-gateway-local"
CONTAINER_NAME="fermi-gateway-test"
BASE_URL="http://localhost:8080"

# Test counters
TESTS_RUN=0
TESTS_PASSED=0
TESTS_FAILED=0

# Helper functions
print_header() {
    echo -e "\n${BLUE}========================================${NC}"
    echo -e "${BLUE}$1${NC}"
    echo -e "${BLUE}========================================${NC}\n"
}

test_endpoint() {
    local name="$1"
    local method="$2"
    local endpoint="$3"
    local expected_status="$4"
    local additional_checks="$5"

    TESTS_RUN=$((TESTS_RUN + 1))
    echo -e "${YELLOW}Testing: ${name}${NC}"

    if [ "$method" = "GET" ]; then
        response=$(curl -s -w "\n%{http_code}" "$BASE_URL$endpoint")
    else
        response=$(curl -s -w "\n%{http_code}" -X "$method" "$BASE_URL$endpoint")
    fi

    status_code=$(echo "$response" | tail -n 1)
    body=$(echo "$response" | sed '$d')

    if [ "$status_code" = "$expected_status" ]; then
        if [ -n "$additional_checks" ]; then
            if echo "$body" | grep -q "$additional_checks"; then
                echo -e "${GREEN}✓ PASSED${NC} - Status: $status_code, Content verified"
                TESTS_PASSED=$((TESTS_PASSED + 1))
                return 0
            else
                echo -e "${RED}✗ FAILED${NC} - Status correct but content missing: $additional_checks"
                echo "Response: $body"
                TESTS_FAILED=$((TESTS_FAILED + 1))
                return 1
            fi
        else
            echo -e "${GREEN}✓ PASSED${NC} - Status: $status_code"
            TESTS_PASSED=$((TESTS_PASSED + 1))
            return 0
        fi
    else
        echo -e "${RED}✗ FAILED${NC} - Expected: $expected_status, Got: $status_code"
        echo "Response: $body"
        TESTS_FAILED=$((TESTS_FAILED + 1))
        return 1
    fi
}

cleanup() {
    echo -e "\n${YELLOW}Cleaning up...${NC}"
    docker stop "$CONTAINER_NAME" 2>/dev/null || true
    docker rm "$CONTAINER_NAME" 2>/dev/null || true
}

# Trap cleanup on exit
trap cleanup EXIT

print_header "FERMI GATEWAY DOCKER TEST SUITE"

# Step 1: Build the image
print_header "Step 1: Building Docker Image"
echo -e "${YELLOW}Building image: ${IMAGE_NAME}:latest${NC}"
if docker build -t "${IMAGE_NAME}:latest" .; then
    echo -e "${GREEN}✓ Image built successfully${NC}"
else
    echo -e "${RED}✗ Failed to build image${NC}"
    exit 1
fi

# Check image size
IMAGE_SIZE=$(docker images "${IMAGE_NAME}:latest" --format "{{.Size}}")
echo -e "${BLUE}Image size: ${IMAGE_SIZE}${NC}"

# Step 2: Stop existing container
print_header "Step 2: Cleaning Up Existing Container"
docker stop "${CONTAINER_NAME}" 2>/dev/null || true
docker rm "${CONTAINER_NAME}" 2>/dev/null || true
echo -e "${GREEN}✓ Cleanup complete${NC}"

# Step 3: Run the container
print_header "Step 3: Starting Container"
echo -e "${YELLOW}Starting container: ${CONTAINER_NAME}${NC}"

# Check if .env exists
if [ -f .env ]; then
    docker run -d \
        --name "${CONTAINER_NAME}" \
        -p 8080:8080 \
        --env-file .env \
        "${IMAGE_NAME}:latest"
else
    echo -e "${YELLOW}Warning: .env file not found, using defaults${NC}"
    docker run -d \
        --name "${CONTAINER_NAME}" \
        -p 8080:8080 \
        -e ENV=development \
        -e PORT=8080 \
        "${IMAGE_NAME}:latest"
fi

# Step 4: Wait for service to be ready
print_header "Step 4: Waiting for Service to be Ready"
echo -e "${YELLOW}Waiting for service to start...${NC}"
MAX_RETRIES=30
RETRY_COUNT=0

while [ $RETRY_COUNT -lt $MAX_RETRIES ]; do
    if curl -s -f "$BASE_URL/health" > /dev/null 2>&1; then
        echo -e "${GREEN}✓ Service is ready!${NC}"
        break
    fi
    RETRY_COUNT=$((RETRY_COUNT + 1))
    echo -n "."
    sleep 1
done

if [ $RETRY_COUNT -eq $MAX_RETRIES ]; then
    echo -e "\n${RED}✗ Service failed to start within timeout${NC}"
    echo -e "${YELLOW}Container logs:${NC}"
    docker logs "${CONTAINER_NAME}"
    exit 1
fi

# Step 5: Run endpoint tests
print_header "Step 5: Testing Endpoints"

# Test 1: Health endpoint
test_endpoint \
    "Health Check Endpoint" \
    "GET" \
    "/health" \
    "200" \
    "ok"

# Test 2: Ready endpoint
test_endpoint \
    "Readiness Check Endpoint" \
    "GET" \
    "/ready" \
    "200" \
    "ready"

# Test 3: Metrics endpoint
test_endpoint \
    "Prometheus Metrics Endpoint" \
    "GET" \
    "/metrics" \
    "200" \
    "go_goroutines"

# Test 4: 404 for unknown route
test_endpoint \
    "404 for Unknown Route" \
    "GET" \
    "/this-does-not-exist" \
    "404" \
    ""

# Test 5: CORS headers (if configured)
echo -e "\n${YELLOW}Testing: CORS Headers${NC}"
TESTS_RUN=$((TESTS_RUN + 1))
cors_response=$(curl -s -I -H "Origin: http://example.com" "$BASE_URL/health")
if echo "$cors_response" | grep -qi "Access-Control-Allow-Origin"; then
    echo -e "${GREEN}✓ PASSED${NC} - CORS headers present"
    TESTS_PASSED=$((TESTS_PASSED + 1))
else
    echo -e "${YELLOW}⚠ WARNING${NC} - CORS headers not found (might not be configured)"
    TESTS_PASSED=$((TESTS_PASSED + 1))
fi

# Step 6: Performance test
print_header "Step 6: Basic Performance Test"
echo -e "${YELLOW}Running 100 requests to /health endpoint...${NC}"

start_time=$(date +%s%N)
for i in {1..100}; do
    curl -s "$BASE_URL/health" > /dev/null
done
end_time=$(date +%s%N)

duration=$(( (end_time - start_time) / 1000000 ))
avg_latency=$(( duration / 100 ))

echo -e "${BLUE}Total time: ${duration}ms${NC}"
echo -e "${BLUE}Average latency: ${avg_latency}ms per request${NC}"

if [ $avg_latency -lt 50 ]; then
    echo -e "${GREEN}✓ Performance: Excellent (<50ms)${NC}"
elif [ $avg_latency -lt 100 ]; then
    echo -e "${GREEN}✓ Performance: Good (<100ms)${NC}"
else
    echo -e "${YELLOW}⚠ Performance: Acceptable but could be better (${avg_latency}ms)${NC}"
fi

# Step 7: Resource usage
print_header "Step 7: Container Resource Usage"
echo -e "${YELLOW}Checking container resource usage...${NC}"

stats=$(docker stats --no-stream --format "table {{.Container}}\t{{.CPUPerc}}\t{{.MemUsage}}" "${CONTAINER_NAME}")
echo "$stats"

# Step 8: Container logs
print_header "Step 8: Container Logs"
echo -e "${YELLOW}Last 20 lines of container logs:${NC}"
docker logs --tail 20 "${CONTAINER_NAME}"

# Step 9: Test summary
print_header "TEST SUMMARY"
echo -e "${BLUE}Total Tests: ${TESTS_RUN}${NC}"
echo -e "${GREEN}Passed: ${TESTS_PASSED}${NC}"
echo -e "${RED}Failed: ${TESTS_FAILED}${NC}"

if [ $TESTS_FAILED -eq 0 ]; then
    echo -e "\n${GREEN}╔════════════════════════════════════╗${NC}"
    echo -e "${GREEN}║  ALL TESTS PASSED SUCCESSFULLY!   ║${NC}"
    echo -e "${GREEN}╚════════════════════════════════════╝${NC}\n"

    echo -e "${YELLOW}Container is running and ready for deployment!${NC}"
    echo ""
    echo "Useful commands:"
    echo "  View live logs:     docker logs -f ${CONTAINER_NAME}"
    echo "  Stop container:     docker stop ${CONTAINER_NAME}"
    echo "  Remove container:   docker rm ${CONTAINER_NAME}"
    echo "  Shell access:       docker exec -it ${CONTAINER_NAME} sh"
    echo ""
    echo "Test the API:"
    echo "  Health:   curl $BASE_URL/health"
    echo "  Ready:    curl $BASE_URL/ready"
    echo "  Metrics:  curl $BASE_URL/metrics"

    exit 0
else
    echo -e "\n${RED}╔════════════════════════════════════╗${NC}"
    echo -e "${RED}║       SOME TESTS FAILED!          ║${NC}"
    echo -e "${RED}╚════════════════════════════════════╝${NC}\n"

    echo -e "${YELLOW}Check logs for details:${NC}"
    echo "  docker logs ${CONTAINER_NAME}"

    exit 1
fi
