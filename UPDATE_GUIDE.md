# Update & Redeploy Guide

## Updating Environment Variables

### Quick Method (Recommended)

1. **Edit your local `.env` file**
   ```bash
   nano .env
   # or
   vim .env
   ```

2. **Run the update script**
   ```bash
   # For Frankfurt instance
   export GCP_PROJECT_ID="fermi-testnet"
   ./update-env.sh europe-west3-a fermi-gateway

   # For other regions
   ./update-env.sh [zone] [instance-name]
   ```

3. **Done!** Services automatically restart with new config

### Manual Method

```bash
# 1. Transfer .env file
gcloud compute scp .env fermi-gateway:/tmp/.env \
  --project=fermi-testnet --zone=europe-west3-a

# 2. Update and restart
gcloud compute ssh fermi-gateway --project=fermi-testnet --zone=europe-west3-a --command='
  sudo cp /opt/fermi/.env /opt/fermi/.env.backup
  sudo mv /tmp/.env /opt/fermi/.env
  sudo systemctl restart fermi-gateway fermi-price-sync
'

# 3. Verify
curl http://34.107.54.90:9080/health
```

---

## Updating Application Code

### Option 1: Update Binaries Only (Fast)

When you've only changed Go code (no config changes):

```bash
# 1. Build new binaries
GOOS=linux GOARCH=amd64 make build

# 2. Transfer binaries
gcloud compute scp bin/gateway bin/price-sync fermi-gateway:/tmp/ \
  --project=fermi-testnet --zone=europe-west3-a

# 3. Deploy and restart
gcloud compute ssh fermi-gateway --project=fermi-testnet --zone=europe-west3-a --command='
  sudo systemctl stop fermi-gateway fermi-price-sync
  sudo cp /tmp/gateway /tmp/price-sync /opt/fermi/
  sudo chmod +x /opt/fermi/gateway /opt/fermi/price-sync
  sudo systemctl start fermi-gateway fermi-price-sync
'

# 4. Verify
curl http://34.107.54.90:9080/health
```

**Time:** ~30 seconds

### Option 2: Full Redeploy (Complete)

When you want to ensure everything is fresh:

```bash
# Delete old instance
gcloud compute instances delete fermi-gateway \
  --project=fermi-testnet --zone=europe-west3-a --quiet

# Deploy fresh instance
export GCP_PROJECT_ID="fermi-testnet"
./deploy-gce-simple.sh europe-west3 europe-west3-a fermi-gateway
```

**Time:** ~5 minutes

---

## Common Update Scenarios

### Scenario 1: Fix Price-Sync Endpoint

```bash
# Edit .env
vim .env
# Change: MARKET_ENDPOINT_FOR_PRICE_SYNC=http://new-endpoint.com

# Update
./update-env.sh europe-west3-a
```

### Scenario 2: Update Rate Limits

```bash
# Edit .env
vim .env
# Change: RATE_LIMIT_CONTINUUM_GRPC=1000

# Update
./update-env.sh europe-west3-a
```

### Scenario 3: Add New Backend Service

```bash
# Edit .env
vim .env
# Add: NEW_SERVICE_URL=https://new-service.com

# Update code to use new service
vim internal/proxy/handler.go

# Build and deploy
GOOS=linux GOARCH=amd64 make build
gcloud compute scp bin/gateway fermi-gateway:/tmp/ \
  --project=fermi-testnet --zone=europe-west3-a

gcloud compute ssh fermi-gateway --project=fermi-testnet --zone=europe-west3-a --command='
  sudo systemctl stop fermi-gateway
  sudo mv /tmp/gateway /opt/fermi/gateway
  sudo chmod +x /opt/fermi/gateway
  sudo systemctl start fermi-gateway
'
```

### Scenario 4: Database Configuration Change

```bash
# Edit .env
vim .env
# Change DB_HOST, DB_PORT, etc.

# Update both services
./update-env.sh europe-west3-a

# Check logs to ensure connection works
gcloud compute ssh fermi-gateway --project=fermi-testnet --zone=europe-west3-a \
  --command='sudo journalctl -u fermi-gateway -f'
```

---

## Rollback Procedure

If something goes wrong:

### Quick Rollback (Environment Variables)

```bash
gcloud compute ssh fermi-gateway --project=fermi-testnet --zone=europe-west3-a --command='
  # Restore backup
  sudo cp /opt/fermi/.env.backup /opt/fermi/.env

  # Restart services
  sudo systemctl restart fermi-gateway fermi-price-sync
'
```

### Full Rollback (Binaries)

```bash
# Keep old binaries before updating
gcloud compute ssh fermi-gateway --project=fermi-testnet --zone=europe-west3-a --command='
  sudo cp /opt/fermi/gateway /opt/fermi/gateway.backup
  sudo cp /opt/fermi/price-sync /opt/fermi/price-sync.backup
'

# To rollback
gcloud compute ssh fermi-gateway --project=fermi-testnet --zone=europe-west3-a --command='
  sudo systemctl stop fermi-gateway fermi-price-sync
  sudo mv /opt/fermi/gateway.backup /opt/fermi/gateway
  sudo mv /opt/fermi/price-sync.backup /opt/fermi/price-sync
  sudo systemctl start fermi-gateway fermi-price-sync
'
```

---

## Zero-Downtime Updates (Advanced)

For production, use blue-green deployment:

### Step 1: Create Second Instance

```bash
# Deploy new instance with updated code
./deploy-gce-simple.sh europe-west3 europe-west3-a fermi-gateway-v2
```

### Step 2: Test New Instance

```bash
# Get new IP
NEW_IP=$(gcloud compute instances describe fermi-gateway-v2 \
  --project=fermi-testnet --zone=europe-west3-a \
  --format='get(networkInterfaces[0].accessConfigs[0].natIP)')

# Test thoroughly
curl http://${NEW_IP}:9080/health
```

### Step 3: Switch Traffic

Update your DNS/load balancer to point to new instance.

### Step 4: Cleanup Old Instance

```bash
# After verification, delete old instance
gcloud compute instances delete fermi-gateway \
  --project=fermi-testnet --zone=europe-west3-a
```

---

## Monitoring After Updates

### Check Service Status

```bash
gcloud compute ssh fermi-gateway --project=fermi-testnet --zone=europe-west3-a --command='
  sudo systemctl status fermi-gateway fermi-price-sync
'
```

### Check Logs

```bash
# Gateway logs
gcloud compute ssh fermi-gateway --project=fermi-testnet --zone=europe-west3-a \
  --command='sudo journalctl -u fermi-gateway -n 50 --no-pager'

# Price-sync logs
gcloud compute ssh fermi-gateway --project=fermi-testnet --zone=europe-west3-a \
  --command='sudo journalctl -u fermi-price-sync -n 50 --no-pager'

# Live tail
gcloud compute ssh fermi-gateway --project=fermi-testnet --zone=europe-west3-a \
  --command='sudo journalctl -u fermi-gateway -f'
```

### Health Checks

```bash
# Quick health check
curl http://34.107.54.90:9080/health
curl http://34.107.54.90:9080/ready

# Detailed metrics
curl http://34.107.54.90:9080/metrics
```

---

## Best Practices

1. **Always test locally first**
   ```bash
   make test
   make run
   ```

2. **Build for correct architecture**
   ```bash
   GOOS=linux GOARCH=amd64 go build
   ```

3. **Keep backups before updates**
   - `.env` files are auto-backed up
   - Manually backup binaries for major changes

4. **Update during low traffic**
   - Services restart causes brief downtime (~2 seconds)

5. **Monitor after deployment**
   - Check logs for errors
   - Verify health endpoints
   - Monitor metrics

6. **Document changes**
   - Git commit messages
   - Update CHANGELOG
   - Note in deployment log

---

## Troubleshooting

**Service won't start after update:**
```bash
# Check logs
sudo journalctl -u fermi-gateway -n 100 --no-pager

# Check .env syntax
sudo cat /opt/fermi/.env | grep "^[^#]"

# Restore backup
sudo cp /opt/fermi/.env.backup /opt/fermi/.env
sudo systemctl restart fermi-gateway
```

**Binary not executable:**
```bash
sudo chmod +x /opt/fermi/gateway /opt/fermi/price-sync
```

**Wrong architecture:**
```bash
# Check binary
file /opt/fermi/gateway
# Should show: "ELF 64-bit LSB executable, x86-64"

# Rebuild correctly
GOOS=linux GOARCH=amd64 go build -o bin/gateway ./cmd/gateway
```
