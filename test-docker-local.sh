#!/bin/bash
# Test the Docker image locally before deploying

set -e

# Colors
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

IMAGE_NAME="fermi-gateway-local"
CONTAINER_NAME="fermi-gateway-test"

echo -e "${GREEN}=== Testing Fermi Gateway Docker Image Locally ===${NC}"

# Build the image
echo -e "${YELLOW}Building Docker image...${NC}"
docker build -t "${IMAGE_NAME}:latest" .

# Stop and remove existing container if running
docker stop "${CONTAINER_NAME}" 2>/dev/null || true
docker rm "${CONTAINER_NAME}" 2>/dev/null || true

# Run the container
echo -e "${YELLOW}Starting container...${NC}"
docker run -d \
    --name "${CONTAINER_NAME}" \
    -p 8080:8080 \
    --env-file .env \
    "${IMAGE_NAME}:latest"

# Wait for container to be ready
echo -e "${YELLOW}Waiting for service to be ready...${NC}"
sleep 3

# Test health endpoint
echo -e "${YELLOW}Testing health endpoint...${NC}"
if curl -f http://localhost:8080/health; then
    echo -e "\n${GREEN}✓ Health check passed!${NC}"
else
    echo -e "\n${RED}✗ Health check failed${NC}"
    docker logs "${CONTAINER_NAME}"
    exit 1
fi

# Test ready endpoint
echo -e "\n${YELLOW}Testing ready endpoint...${NC}"
if curl -f http://localhost:8080/ready; then
    echo -e "\n${GREEN}✓ Ready check passed!${NC}"
else
    echo -e "\n${RED}✗ Ready check failed${NC}"
fi

# Show container info
echo -e "\n${GREEN}Container is running successfully!${NC}"
echo ""
echo "View logs:"
echo "  docker logs -f ${CONTAINER_NAME}"
echo ""
echo "Stop container:"
echo "  docker stop ${CONTAINER_NAME}"
echo ""
echo "Remove container:"
echo "  docker rm ${CONTAINER_NAME}"
echo ""
echo "Test endpoints:"
echo "  curl http://localhost:8080/health"
echo "  curl http://localhost:8080/metrics"
