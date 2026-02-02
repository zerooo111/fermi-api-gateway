#!/bin/bash
# Deploy Fermi API Gateway to Cloud Run (local build method)

set -e

PROJECT_ID="${GCP_PROJECT_ID:-your-project-id}"
REGION="${GCP_REGION:-us-central1}"
SERVICE_NAME="fermi-gateway"
IMAGE_NAME="gcr.io/${PROJECT_ID}/${SERVICE_NAME}"

GREEN='\033[0;32m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
NC='\033[0m'

echo -e "${GREEN}=== Deploying Fermi Gateway (Local Build) ===${NC}"

if [ "$PROJECT_ID" = "your-project-id" ]; then
    echo -e "${RED}Error: Please set GCP_PROJECT_ID${NC}"
    exit 1
fi

echo -e "${YELLOW}Project: ${PROJECT_ID}${NC}"
gcloud config set project "${PROJECT_ID}"

echo -e "${YELLOW}Configuring Docker for GCR...${NC}"
gcloud auth configure-docker gcr.io --quiet

echo -e "${YELLOW}Building image locally...${NC}"
docker build -t "${IMAGE_NAME}:latest" .

echo -e "${YELLOW}Pushing to GCR...${NC}"
docker push "${IMAGE_NAME}:latest"

echo -e "${YELLOW}Deploying to Cloud Run...${NC}"
gcloud run deploy "${SERVICE_NAME}" \
    --image "${IMAGE_NAME}:latest" \
    --platform managed \
    --region "${REGION}" \
    --allow-unauthenticated \
    --min-instances 1 \
    --max-instances 10 \
    --cpu 1 \
    --memory 512Mi \
    --concurrency 1000 \
    --timeout 300 \
    --port 8080 \
    --set-env-vars "ENV=production,LOG_LEVEL=info"

SERVICE_URL=$(gcloud run services describe "${SERVICE_NAME}" \
    --platform managed \
    --region "${REGION}" \
    --format 'value(status.url)')

echo -e "\n${GREEN}✓ Deployment successful!${NC}"
echo -e "${GREEN}Service URL: ${SERVICE_URL}${NC}"
echo -e "\nTest: curl ${SERVICE_URL}/health"
