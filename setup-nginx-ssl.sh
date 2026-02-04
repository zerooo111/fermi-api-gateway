#!/bin/bash
# Setup Nginx with SSL for Fermi Gateway

set -e

PROJECT_ID="${GCP_PROJECT_ID:-fermi-testnet}"
ZONE="${GCP_ZONE:-us-central1-a}"
INSTANCE_NAME="fermi-gateway"
DOMAIN="${1}"
EMAIL="${2:-admin@fermilabs.xyz}"

GREEN='\033[0;32m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
NC='\033[0m'

if [ -z "$DOMAIN" ]; then
    echo -e "${RED}Error: Domain name required${NC}"
    echo "Usage: $0 <domain> [email]"
    echo "Example: $0 testnet.fermi.trade admin@fermilabs.xyz"
    exit 1
fi

# Get instance IP
EXTERNAL_IP=$(gcloud compute instances describe ${INSTANCE_NAME} \
    --project=${PROJECT_ID} \
    --zone=${ZONE} \
    --format='get(networkInterfaces[0].accessConfigs[0].natIP)')

echo -e "${GREEN}=== Setting up Nginx + SSL ===${NC}"
echo -e "${YELLOW}Domain: ${DOMAIN}${NC}"
echo -e "${YELLOW}Instance IP: ${EXTERNAL_IP}${NC}"
echo -e "${YELLOW}Email: ${EMAIL}${NC}"

# Create Nginx configuration
echo -e "\n${YELLOW}Configuring Nginx reverse proxy...${NC}"
gcloud compute ssh ${INSTANCE_NAME} --project=${PROJECT_ID} --zone=${ZONE} --command="
set -e

# Create Nginx config
sudo tee /etc/nginx/sites-available/fermi-gateway > /dev/null << 'EOF'
server {
    listen 80;
    server_name ${DOMAIN};

    # Let's Encrypt validation
    location /.well-known/acme-challenge/ {
        root /var/www/html;
    }

    # Redirect all other HTTP traffic to HTTPS
    location / {
        return 301 https://\$server_name\$request_uri;
    }
}

server {
    listen 443 ssl http2;
    server_name ${DOMAIN};

    # SSL certificates (will be configured by certbot)
    ssl_certificate /etc/letsencrypt/live/${DOMAIN}/fullchain.pem;
    ssl_certificate_key /etc/letsencrypt/live/${DOMAIN}/privkey.pem;

    # SSL configuration
    ssl_protocols TLSv1.2 TLSv1.3;
    ssl_ciphers HIGH:!aNULL:!MD5;
    ssl_prefer_server_ciphers on;

    # Security headers
    add_header Strict-Transport-Security \"max-age=31536000; includeSubDomains\" always;
    add_header X-Frame-Options DENY always;
    add_header X-Content-Type-Options nosniff always;
    add_header X-XSS-Protection \"1; mode=block\" always;

    # Proxy settings
    location / {
        proxy_pass http://localhost:9080;
        proxy_http_version 1.1;

        # Preserve client information
        proxy_set_header Host \$host;
        proxy_set_header X-Real-IP \$remote_addr;
        proxy_set_header X-Forwarded-For \$proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto \$scheme;

        # Timeouts
        proxy_connect_timeout 60s;
        proxy_send_timeout 60s;
        proxy_read_timeout 60s;

        # Buffering
        proxy_buffering off;
        proxy_request_buffering off;
    }

    # Health check endpoint
    location /health {
        proxy_pass http://localhost:9080/health;
        access_log off;
    }

    # Logging
    access_log /var/log/nginx/fermi-gateway-access.log;
    error_log /var/log/nginx/fermi-gateway-error.log;
}
EOF

# Enable the site
sudo ln -sf /etc/nginx/sites-available/fermi-gateway /etc/nginx/sites-enabled/
sudo rm -f /etc/nginx/sites-enabled/default

# Test Nginx configuration
sudo nginx -t

# Reload Nginx
sudo systemctl reload nginx

echo 'Nginx configured successfully'
"

echo -e "\n${YELLOW}DNS Check:${NC}"
echo -e "Before obtaining SSL certificate, ensure ${DOMAIN} points to ${EXTERNAL_IP}"
echo -e "Current DNS resolution:"
nslookup ${DOMAIN} 2>/dev/null | grep -A1 "Name:" || echo "Domain not yet resolving to this IP"

read -p "Does ${DOMAIN} resolve to ${EXTERNAL_IP}? (y/N): " -n 1 -r
echo
if [[ ! $REPLY =~ ^[Yy]$ ]]; then
    echo -e "${YELLOW}Please update your DNS records and run:${NC}"
    echo -e "  Update DNS: ${DOMAIN} -> A record -> ${EXTERNAL_IP}"
    echo -e "  Then run: sudo certbot --nginx -d ${DOMAIN} --non-interactive --agree-tos --email ${EMAIL}"
    echo -e "\n${YELLOW}Or SSH into instance and run manually:${NC}"
    echo -e "  gcloud compute ssh ${INSTANCE_NAME} --project=${PROJECT_ID} --zone=${ZONE}"
    echo -e "  sudo certbot --nginx -d ${DOMAIN} --email ${EMAIL}"
    exit 0
fi

# Obtain SSL certificate
echo -e "\n${YELLOW}Obtaining SSL certificate from Let's Encrypt...${NC}"
gcloud compute ssh ${INSTANCE_NAME} --project=${PROJECT_ID} --zone=${ZONE} --command="
sudo certbot --nginx -d ${DOMAIN} --non-interactive --agree-tos --email ${EMAIL} --redirect
"

echo -e "\n${GREEN}✓ SSL Setup Complete!${NC}"
echo -e "${GREEN}Gateway URL: https://${DOMAIN}${NC}"
echo -e "\n${YELLOW}Testing HTTPS endpoint...${NC}"
sleep 3
curl -s https://${DOMAIN}/health | jq . || curl -s https://${DOMAIN}/health

echo -e "\n${YELLOW}Certificate auto-renewal:${NC}"
echo "Certbot will automatically renew certificates before expiry"
echo "Test renewal: gcloud compute ssh ${INSTANCE_NAME} --project=${PROJECT_ID} --zone=${ZONE} --command='sudo certbot renew --dry-run'"
