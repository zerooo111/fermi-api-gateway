# GCP Permissions Setup for fermi-testnet

## Required Permissions for Deployment

Your GCP project admin needs to run these commands to enable Cloud Run deployment:

```bash
PROJECT_ID="fermi-testnet"
PROJECT_NUMBER="902189515653"
SERVICE_ACCOUNT="${PROJECT_NUMBER}-compute@developer.gserviceaccount.com"
USER_EMAIL="zero@fermilabs.xyz"

# 1. Grant user permissions
gcloud projects add-iam-policy-binding ${PROJECT_ID} \
    --member="user:${USER_EMAIL}" \
    --role="roles/run.admin"

gcloud projects add-iam-policy-binding ${PROJECT_ID} \
    --member="user:${USER_EMAIL}" \
    --role="roles/artifactregistry.admin"

gcloud projects add-iam-policy-binding ${PROJECT_ID} \
    --member="user:${USER_EMAIL}" \
    --role="roles/iam.serviceAccountUser"

# 2. Grant Cloud Build service account permissions
gcloud projects add-iam-policy-binding ${PROJECT_ID} \
    --member="serviceAccount:${SERVICE_ACCOUNT}" \
    --role="roles/storage.admin"

gcloud projects add-iam-policy-binding ${PROJECT_ID} \
    --member="serviceAccount:${SERVICE_ACCOUNT}" \
    --role="roles/artifactregistry.writer"

gcloud projects add-iam-policy-binding ${PROJECT_ID} \
    --member="serviceAccount:${SERVICE_ACCOUNT}" \
    --role="roles/run.admin"
```

## After Permissions are Granted

Once permissions are set up, deployment is simple:

```bash
export GCP_PROJECT_ID="fermi-testnet"
./deploy-gateway-cloudbuild.sh
```

## Testing the Deployment

After successful deployment:

```bash
# Get the service URL
SERVICE_URL=$(gcloud run services describe fermi-gateway \
    --platform managed \
    --region us-central1 \
    --format 'value(status.url)')

# Test health endpoint
curl ${SERVICE_URL}/health

# Test ready endpoint
curl ${SERVICE_URL}/ready
```

## Current Status

- ✅ All deployment scripts ready
- ✅ Dockerfile optimized (28.1MB)
- ✅ Tests passing
- ✅ APIs enabled
- ⏳ Waiting for IAM permissions
