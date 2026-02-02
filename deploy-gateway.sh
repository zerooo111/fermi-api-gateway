#!/bin/bash
# Deploy Fermi API Gateway to Google Cloud Run

set -e

# Configuration
PROJECT_ID="${GCP_PROJECT_ID:-your-project-id}"
REGION="${GCP_REGION:-us-central1}"
SERVICE_NAME="fermi-gateway"
IMAGE_NAME="gcr.io/${PROJECT_ID}/${SERVICE_NAME}"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

echo -e "${GREEN}=== Deploying Fermi API Gateway to Cloud Run ===${NC}"

# Check if GCP_PROJECT_ID is set
if [ "$PROJECT_ID" = "your-project-id" ]; then
    echo -e "${RED}Error: Please set GCP_PROJECT_ID environment variable${NC}"
    echo "Example: export GCP_PROJECT_ID=my-gcp-project"
    exit 1
fi

# Check if gcloud is installed
if ! command -v gcloud &> /dev/null; then
    echo -e "${RED}Error: gcloud CLI not found${NC}"
    echo "Install from: https://cloud.google.com/sdk/docs/install"
    exit 1
fi

# Set the project
echo -e "${YELLOW}Setting GCP project: ${PROJECT_ID}${NC}"
gcloud config set project "${PROJECT_ID}"

# Enable required APIs
echo -e "${YELLOW}Enabling required GCP APIs...${NC}"
gcloud services enable \
    cloudbuild.googleapis.com \
    run.googleapis.com \
    containerregistry.googleapis.com

# Build and push using Cloud Build (faster than local builds)
echo -e "${YELLOW}Building container image with Cloud Build...${NC}"
gcloud builds submit --tag "${IMAGE_NAME}:latest"

# Deploy to Cloud Run
echo -e "${YELLOW}Deploying to Cloud Run...${NC}"
gcloud run deploy "${SERVICE_NAME}" \
    --image "${IMAGE_NAME}:latest" \
    --platform managed \
    --region "${REGION}" \
    --allow-unauthenticated \
    --min-instances 2 \
    --max-instances 50 \
    --cpu 2 \
    --memory 512Mi \
    --concurrency 1000 \
    --timeout 300 \
    --port 8080 \
    --set-env-vars "ENV=production" \
    --set-env-vars "LOG_LEVEL=info"

# Get the service URL
SERVICE_URL=$(gcloud run services describe "${SERVICE_NAME}" \
    --platform managed \
    --region "${REGION}" \
    --format 'value(status.url)')

echo -e "${GREEN}✓ Deployment successful!${NC}"
echo -e "${GREEN}Service URL: ${SERVICE_URL}${NC}"
echo ""
echo -e "${YELLOW}Test the deployment:${NC}"
echo "curl ${SERVICE_URL}/health"
echo ""
echo -e "${YELLOW}View logs:${NC}"
echo "gcloud run services logs read ${SERVICE_NAME} --region ${REGION} --limit 50"
echo ""
echo -e "${YELLOW}Set environment variables:${NC}"
echo "gcloud run services update ${SERVICE_NAME} --region ${REGION} \\"
echo "  --set-env-vars KEY1=value1,KEY2=value2"
