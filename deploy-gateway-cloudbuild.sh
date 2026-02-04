#!/bin/bash
# Deploy using Cloud Build (no local Docker needed)

set -e

PROJECT_ID="${GCP_PROJECT_ID:-fermi-testnet}"
REGION="${GCP_REGION:-us-central1}"
SERVICE_NAME="fermi-gateway"

echo -e "\033[0;32m=== Deploying via Cloud Build ===\033[0m"
echo -e "\033[1;33mProject: ${PROJECT_ID}\033[0m"

gcloud config set project "${PROJECT_ID}"

echo -e "\033[1;33mEnabling required APIs...\033[0m"
gcloud services enable cloudbuild.googleapis.com run.googleapis.com artifactregistry.googleapis.com

echo -e "\033[1;33mDeploying to Cloud Run...\033[0m"
gcloud run deploy "${SERVICE_NAME}" \
    --source . \
    --platform managed \
    --region "${REGION}" \
    --allow-unauthenticated \
    --min-instances 1 \
    --max-instances 10 \
    --cpu 1 \
    --memory 512Mi \
    --port 8080

SERVICE_URL=$(gcloud run services describe "${SERVICE_NAME}" \
    --platform managed \
    --region "${REGION}" \
    --format 'value(status.url)')

echo -e "\n\033[0;32m✓ Deployment successful!\033[0m"
echo -e "\033[0;32mService URL: ${SERVICE_URL}\033[0m"
echo -e "\nTest: curl ${SERVICE_URL}/health"
