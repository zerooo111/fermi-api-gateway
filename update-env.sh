#!/bin/bash
# Update environment variables on running instance
# Usage: ./update-env.sh [zone] [instance-name]

set -e

ZONE="${1:-${GCP_ZONE:-europe-west3-a}}"
INSTANCE_NAME="${2:-fermi-gateway}"
PROJECT_ID="${GCP_PROJECT_ID:-fermi-testnet}"

GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

echo -e "${GREEN}=== Updating Environment Variables ===${NC}"
echo -e "${YELLOW}Instance: ${INSTANCE_NAME}${NC}"
echo -e "${YELLOW}Zone: ${ZONE}${NC}"

# Check if .env exists
if [ ! -f .env ]; then
    echo "Error: .env file not found"
    exit 1
fi

echo -e "\n${YELLOW}Transferring updated .env file...${NC}"
gcloud compute scp .env ${INSTANCE_NAME}:/tmp/.env \
    --project=${PROJECT_ID} --zone=${ZONE}

echo -e "\n${YELLOW}Updating and restarting services...${NC}"
gcloud compute ssh ${INSTANCE_NAME} --project=${PROJECT_ID} --zone=${ZONE} --command='
set -e

# Backup old .env
sudo cp /opt/fermi/.env /opt/fermi/.env.backup

# Move new .env
sudo mv /tmp/.env /opt/fermi/.env
sudo chmod 644 /opt/fermi/.env

# Restart services to pick up new env vars
echo "Restarting services..."
sudo systemctl restart fermi-gateway fermi-price-sync

# Wait for services to start
sleep 3

# Check service status
echo ""
echo "=== Service Status ==="
sudo systemctl status fermi-gateway fermi-price-sync --no-pager | grep -E "(Active:|Main PID:)"

echo ""
echo "=== Gateway Health Check ==="
curl -s http://localhost:9080/health || echo "Gateway not responding yet"
'

echo -e "\n${GREEN}✓ Environment variables updated!${NC}"
echo -e "${YELLOW}Services restarted and running with new configuration${NC}"

# Test the endpoint
EXTERNAL_IP=$(gcloud compute instances describe ${INSTANCE_NAME} \
    --project=${PROJECT_ID} \
    --zone=${ZONE} \
    --format='get(networkInterfaces[0].accessConfigs[0].natIP)')

echo -e "\n${YELLOW}Testing endpoint...${NC}"
sleep 2
curl -s http://${EXTERNAL_IP}:9080/health | jq . || curl -s http://${EXTERNAL_IP}:9080/health

echo -e "\n${GREEN}Update complete!${NC}"
