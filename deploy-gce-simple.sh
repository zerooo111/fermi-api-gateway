#!/bin/bash
# Deploy Fermi Gateway + Price-Sync to GCE (Ubuntu, no Docker)

set -e

PROJECT_ID="${GCP_PROJECT_ID:-fermi-testnet}"
ZONE="${GCP_ZONE:-us-central1-a}"
INSTANCE_NAME="fermi-gateway"
MACHINE_TYPE="e2-medium"

GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

echo -e "${GREEN}=== Deploying Fermi Services to GCE ===${NC}"

# Check if binaries exist
if [ ! -f bin/gateway ] || [ ! -f bin/price-sync ]; then
    echo -e "${YELLOW}Building binaries...${NC}"
    make build
fi

# Check if .env exists
if [ ! -f .env ]; then
    echo "Error: .env file not found"
    exit 1
fi

# Create firewall rules
echo -e "${YELLOW}Creating firewall rules...${NC}"
gcloud compute firewall-rules create allow-gateway-9080 \
    --project="${PROJECT_ID}" \
    --allow=tcp:9080 \
    --source-ranges=0.0.0.0/0 \
    --target-tags=gateway \
    --description="Allow traffic to Fermi Gateway" 2>/dev/null || echo "Firewall rule exists"

# Create startup script
cat > /tmp/startup-script.sh << 'EOF'
#!/bin/bash
# This runs once when instance boots

# Wait for network
sleep 10

# Install required packages
apt-get update
apt-get install -y curl jq

# Create service directory
mkdir -p /opt/fermi
chmod 755 /opt/fermi

echo "Startup complete"
EOF

# Create instance with Ubuntu
echo -e "${YELLOW}Creating Ubuntu instance...${NC}"
gcloud compute instances create "${INSTANCE_NAME}" \
    --project="${PROJECT_ID}" \
    --zone="${ZONE}" \
    --machine-type="${MACHINE_TYPE}" \
    --image-family=ubuntu-2204-lts \
    --image-project=ubuntu-os-cloud \
    --boot-disk-size=20GB \
    --boot-disk-type=pd-standard \
    --tags=gateway \
    --metadata-from-file=startup-script=/tmp/startup-script.sh \
    --scopes=https://www.googleapis.com/auth/cloud-platform

echo -e "${YELLOW}Waiting for instance to be ready (30 seconds)...${NC}"
sleep 30

# Transfer files
echo -e "${YELLOW}Transferring binaries and config...${NC}"
gcloud compute scp bin/gateway bin/price-sync .env ${INSTANCE_NAME}:/tmp/ \
    --project=${PROJECT_ID} --zone=${ZONE}

# Deploy services
echo -e "${YELLOW}Setting up services...${NC}"
gcloud compute ssh ${INSTANCE_NAME} --project=${PROJECT_ID} --zone=${ZONE} --command='
set -e

# Move files to /opt/fermi
sudo mv /tmp/gateway /tmp/price-sync /tmp/.env /opt/fermi/
sudo chmod +x /opt/fermi/gateway /opt/fermi/price-sync

# Create systemd service for gateway
sudo tee /etc/systemd/system/fermi-gateway.service > /dev/null << "SERVICE"
[Unit]
Description=Fermi API Gateway
After=network.target

[Service]
Type=simple
User=root
WorkingDirectory=/opt/fermi
EnvironmentFile=/opt/fermi/.env
ExecStart=/opt/fermi/gateway
Restart=always
RestartSec=5

[Install]
WantedBy=multi-user.target
SERVICE

# Create systemd service for price-sync
sudo tee /etc/systemd/system/fermi-price-sync.service > /dev/null << "SERVICE"
[Unit]
Description=Fermi Price Sync Service
After=network.target

[Service]
Type=simple
User=root
WorkingDirectory=/opt/fermi
EnvironmentFile=/opt/fermi/.env
ExecStart=/opt/fermi/price-sync
Restart=always
RestartSec=5

[Install]
WantedBy=multi-user.target
SERVICE

# Reload systemd and start services
sudo systemctl daemon-reload
sudo systemctl enable fermi-gateway fermi-price-sync
sudo systemctl start fermi-gateway fermi-price-sync

# Wait for services to start
sleep 3

echo ""
echo "=== Service Status ==="
sudo systemctl status fermi-gateway --no-pager -l | head -15
echo ""
sudo systemctl status fermi-price-sync --no-pager -l | head -15
'

# Get external IP
EXTERNAL_IP=$(gcloud compute instances describe ${INSTANCE_NAME} \
    --project=${PROJECT_ID} \
    --zone=${ZONE} \
    --format='get(networkInterfaces[0].accessConfigs[0].natIP)')

# Cleanup
rm -f /tmp/startup-script.sh

echo -e "\n${GREEN}✓ Deployment complete!${NC}"
echo -e "${GREEN}Instance: ${INSTANCE_NAME} (${MACHINE_TYPE})${NC}"
echo -e "${GREEN}External IP: ${EXTERNAL_IP}${NC}"
echo -e "${GREEN}Gateway URL: http://${EXTERNAL_IP}:9080${NC}"

echo -e "\n${YELLOW}Testing gateway...${NC}"
sleep 5
curl -s http://${EXTERNAL_IP}:9080/health | jq . || curl -s http://${EXTERNAL_IP}:9080/health

echo -e "\n${YELLOW}Useful Commands:${NC}"
echo "View logs:       gcloud compute ssh ${INSTANCE_NAME} --project=${PROJECT_ID} --zone=${ZONE} --command='sudo journalctl -u fermi-gateway -f'"
echo "Price-sync logs: gcloud compute ssh ${INSTANCE_NAME} --project=${PROJECT_ID} --zone=${ZONE} --command='sudo journalctl -u fermi-price-sync -f'"
echo "Restart gateway: gcloud compute ssh ${INSTANCE_NAME} --project=${PROJECT_ID} --zone=${ZONE} --command='sudo systemctl restart fermi-gateway'"
echo "Service status:  gcloud compute ssh ${INSTANCE_NAME} --project=${PROJECT_ID} --zone=${ZONE} --command='sudo systemctl status fermi-gateway fermi-price-sync'"
echo "Stop instance:   gcloud compute instances stop ${INSTANCE_NAME} --project=${PROJECT_ID} --zone=${ZONE}"
echo "Delete instance: gcloud compute instances delete ${INSTANCE_NAME} --project=${PROJECT_ID} --zone=${ZONE}"
