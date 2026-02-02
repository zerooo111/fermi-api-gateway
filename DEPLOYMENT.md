# GCP Deployment Guide

This guide covers deploying the Fermi API Gateway and Price-Sync service to Google Cloud Platform.

## Architecture

**Gateway**: Cloud Run (auto-scaling HTTP service)
**Price-Sync**: Compute Engine e2-micro (24/7 background worker)

## Prerequisites

1. **GCP Project**: Create or select a project at [console.cloud.google.com](https://console.cloud.google.com)
2. **gcloud CLI**: Install from [cloud.google.com/sdk/docs/install](https://cloud.google.com/sdk/docs/install)
3. **Docker**: Install from [docker.com/get-started](https://www.docker.com/get-started)
4. **Authentication**: Run `gcloud auth login`

## Gateway Deployment (Cloud Run)

### Step 1: Test Locally (Recommended)

```bash
# Test the Docker image locally
./test-docker-local.sh

# Manual test
docker build -t fermi-gateway:latest .
docker run -p 8080:8080 --env-file .env fermi-gateway:latest

# Test endpoints
curl http://localhost:8080/health
curl http://localhost:8080/metrics
```

### Step 2: Deploy to Cloud Run

```bash
# Set your GCP project ID
export GCP_PROJECT_ID="your-project-id"
export GCP_REGION="us-central1"  # Optional, defaults to us-central1

# Deploy using the script
./deploy-gateway.sh
```

### Step 3: Configure Environment Variables

Set your production environment variables:

```bash
gcloud run services update fermi-gateway \
  --region us-central1 \
  --set-env-vars "\
ENV=production,\
PORT=8080,\
ALLOWED_ORIGINS=https://yourdomain.com,\
CONTINUUM_GRPC_URL=your-grpc-url:9090,\
CONTINUUM_REST_URL=https://your-rest-api.com,\
ROLLUP_URL=https://your-rollup-api.com,\
DB_HOST=your-db-host,\
DB_PORT=5432,\
DB_USER=your-db-user,\
DB_NAME=your-db-name"
```

**For sensitive data** (passwords, API keys), use Secret Manager:

```bash
# Create secret
echo -n "your-db-password" | gcloud secrets create db-password --data-file=-

# Grant Cloud Run access
gcloud secrets add-iam-policy-binding db-password \
  --member="serviceAccount:YOUR-PROJECT-NUMBER-compute@developer.gserviceaccount.com" \
  --role="roles/secretmanager.secretAccessor"

# Update Cloud Run to use secret
gcloud run services update fermi-gateway \
  --region us-central1 \
  --set-secrets=DB_PASSWORD=db-password:latest
```

### Step 4: Configure Custom Domain (Optional)

```bash
# Map custom domain
gcloud run domain-mappings create \
  --service fermi-gateway \
  --domain api.yourdomain.com \
  --region us-central1

# Follow DNS instructions to add CNAME record
```

### Step 5: Enable Cloud CDN (Optional, for caching)

Cloud Run integrates with Cloud CDN via Cloud Load Balancer. See [Cloud Run CDN docs](https://cloud.google.com/run/docs/configuring/cdn).

---

## Price-Sync Deployment (Compute Engine)

### Step 1: Create VM Instance

```bash
# Create e2-micro instance (most cost-effective)
gcloud compute instances create price-sync-vm \
  --project="${GCP_PROJECT_ID}" \
  --zone=us-central1-a \
  --machine-type=e2-micro \
  --image-family=debian-12 \
  --image-project=debian-cloud \
  --boot-disk-size=10GB \
  --boot-disk-type=pd-standard \
  --tags=price-sync \
  --metadata=startup-script='#!/bin/bash
# Install basic dependencies
apt-get update
apt-get install -y curl ca-certificates'
```

### Step 2: Build and Upload Binary

```bash
# Build for Linux
make build-price-sync

# Or manually
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o bin/price-sync ./cmd/price-sync

# Copy to VM
gcloud compute scp bin/price-sync price-sync-vm:~/price-sync --zone=us-central1-a
```

### Step 3: Create systemd Service

SSH into the VM and create a systemd service:

```bash
# SSH to VM
gcloud compute ssh price-sync-vm --zone=us-central1-a

# Create service file
sudo tee /etc/systemd/system/price-sync.service > /dev/null <<'EOF'
[Unit]
Description=Price Sync Service
After=network.target

[Service]
Type=simple
User=nobody
Group=nogroup
WorkingDirectory=/opt/price-sync
ExecStart=/opt/price-sync/price-sync
Restart=always
RestartSec=10
StandardOutput=journal
StandardError=journal

# Environment variables
Environment="MARKET_ENDPOINT_FOR_PRICE_SYNC=http://your-market-endpoint:8080"
Environment="PRICE_SYNC_POLL_INTERVAL=1s"
Environment="PRICE_SYNC_HEARTBEAT_INTERVAL=30s"
Environment="PRICE_SYNC_HTTP_TIMEOUT=3s"
Environment="DB_HOST=your-db-host"
Environment="DB_PORT=5432"
Environment="DB_USER=your-db-user"
Environment="DB_PASSWORD=your-db-password"
Environment="DB_NAME=your-db-name"

[Install]
WantedBy=multi-user.target
EOF

# Create directory and move binary
sudo mkdir -p /opt/price-sync
sudo mv ~/price-sync /opt/price-sync/
sudo chmod +x /opt/price-sync/price-sync
sudo chown -R nobody:nogroup /opt/price-sync

# Enable and start service
sudo systemctl daemon-reload
sudo systemctl enable price-sync
sudo systemctl start price-sync

# Check status
sudo systemctl status price-sync

# View logs
sudo journalctl -u price-sync -f
```

### Step 4: Configure Firewall (if needed)

Price-sync doesn't expose any ports, so no firewall rules needed.

### Step 5: Monitor the Service

```bash
# SSH to VM
gcloud compute ssh price-sync-vm --zone=us-central1-a

# View logs
sudo journalctl -u price-sync -f

# Check status
sudo systemctl status price-sync

# Restart service
sudo systemctl restart price-sync
```

---

## Cost Estimates

### Gateway (Cloud Run)
- **Low traffic** (1k req/min): ~$10-20/month
- **Medium traffic** (10k req/min): ~$50-100/month
- **High traffic** (100k req/min): ~$300-500/month

### Price-Sync (Compute Engine e2-micro)
- **e2-micro**: ~$7-8/month (24/7)
- **e2-small**: ~$12-15/month (if more resources needed)

**Total baseline**: ~$17-28/month + gateway usage

---

## Monitoring & Logging

### View Gateway Logs
```bash
# Real-time logs
gcloud run services logs read fermi-gateway --region us-central1 --limit 50 --follow

# Logs in Cloud Console
https://console.cloud.google.com/run/detail/us-central1/fermi-gateway/logs
```

### View Price-Sync Logs
```bash
# SSH and view logs
gcloud compute ssh price-sync-vm --zone=us-central1-a
sudo journalctl -u price-sync -f
```

### Metrics & Dashboards
```bash
# Gateway metrics (Prometheus)
curl https://your-gateway-url/metrics

# Cloud Monitoring
https://console.cloud.google.com/monitoring
```

---

## Troubleshooting

### Gateway Issues

**Container won't start:**
```bash
# Check logs
gcloud run services logs read fermi-gateway --region us-central1 --limit 100

# Test locally first
./test-docker-local.sh
```

**Environment variables not working:**
```bash
# List current env vars
gcloud run services describe fermi-gateway --region us-central1 --format="get(spec.template.spec.containers[0].env)"
```

### Price-Sync Issues

**Service not starting:**
```bash
# SSH to VM
gcloud compute ssh price-sync-vm --zone=us-central1-a

# Check service status
sudo systemctl status price-sync

# View detailed logs
sudo journalctl -u price-sync --no-pager

# Check if binary is executable
ls -la /opt/price-sync/price-sync
```

**Database connection errors:**
- Verify DB_HOST, DB_PORT are correct
- Check database firewall rules allow VM IP
- Test connection: `telnet $DB_HOST $DB_PORT`

---

## Updating Deployments

### Update Gateway
```bash
# Make code changes, then redeploy
./deploy-gateway.sh

# Or update just env vars
gcloud run services update fermi-gateway \
  --region us-central1 \
  --set-env-vars KEY=value
```

### Update Price-Sync
```bash
# Build new binary
make build-price-sync

# Copy to VM
gcloud compute scp bin/price-sync price-sync-vm:/tmp/price-sync --zone=us-central1-a

# SSH and update
gcloud compute ssh price-sync-vm --zone=us-central1-a
sudo mv /tmp/price-sync /opt/price-sync/price-sync
sudo chmod +x /opt/price-sync/price-sync
sudo systemctl restart price-sync
sudo systemctl status price-sync
```

---

## CI/CD with Cloud Build (Optional)

Create `cloudbuild.yaml`:

```yaml
steps:
  # Build the container image
  - name: 'gcr.io/cloud-builders/docker'
    args: ['build', '-t', 'gcr.io/$PROJECT_ID/fermi-gateway:$COMMIT_SHA', '.']

  # Push to Container Registry
  - name: 'gcr.io/cloud-builders/docker'
    args: ['push', 'gcr.io/$PROJECT_ID/fermi-gateway:$COMMIT_SHA']

  # Deploy to Cloud Run
  - name: 'gcr.io/google.com/cloudsdktool/cloud-sdk'
    entrypoint: gcloud
    args:
      - 'run'
      - 'deploy'
      - 'fermi-gateway'
      - '--image'
      - 'gcr.io/$PROJECT_ID/fermi-gateway:$COMMIT_SHA'
      - '--region'
      - 'us-central1'
      - '--platform'
      - 'managed'

images:
  - 'gcr.io/$PROJECT_ID/fermi-gateway:$COMMIT_SHA'
```

Connect to GitHub:
```bash
gcloud builds triggers create github \
  --repo-name=fermi-api-gateway \
  --repo-owner=your-org \
  --branch-pattern="^main$" \
  --build-config=cloudbuild.yaml
```

---

## Security Best Practices

1. **Never commit `.env` files** - Already in `.gitignore`
2. **Use Secret Manager** for sensitive data (DB passwords, API keys)
3. **Enable VPC connector** if accessing private resources
4. **Use IAM roles** instead of service account keys
5. **Enable Cloud Armor** for DDoS protection on production
6. **Set up alerts** in Cloud Monitoring for errors/latency

---

## Next Steps

1. ✅ Deploy gateway to Cloud Run
2. ✅ Deploy price-sync to Compute Engine
3. 🔲 Set up Cloud Monitoring dashboards
4. 🔲 Configure alerts (error rates, latency, downtime)
5. 🔲 Set up Cloud CDN for caching (if needed)
6. 🔲 Configure custom domain
7. 🔲 Set up CI/CD pipeline

---

## Support

- **GCP Documentation**: [cloud.google.com/docs](https://cloud.google.com/docs)
- **Cloud Run Docs**: [cloud.google.com/run/docs](https://cloud.google.com/run/docs)
- **Compute Engine Docs**: [cloud.google.com/compute/docs](https://cloud.google.com/compute/docs)
