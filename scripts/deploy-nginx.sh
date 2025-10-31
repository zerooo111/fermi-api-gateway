#!/bin/bash
# Deploy Nginx Configuration for Fermi API Gateway
# Domain: api.fermi.trade

set -e

echo "=========================================="
echo "  Fermi Gateway - Nginx Deployment"
echo "  Domain: api.fermi.trade"
echo "=========================================="
echo ""

# Check if running as root
if [ "$EUID" -ne 0 ]; then
    echo "Please run as root (use sudo)"
    exit 1
fi

DOMAIN="api.fermi.trade"
GATEWAY_DIR="/opt/fermi-api-gateway"
BACKUP_DIR="/etc/nginx/backups"
TIMESTAMP=$(date +%Y%m%d_%H%M%S)

# Rollback function in case of failure
rollback() {
    echo ""
    echo "=========================================="
    echo "  Deployment Failed - Rolling Back"
    echo "=========================================="

    LATEST_BACKUP=$(ls -t $BACKUP_DIR/nginx-configs-backup-*.tar.gz 2>/dev/null | head -1)

    if [ -n "$LATEST_BACKUP" ] && [ -f "$LATEST_BACKUP" ]; then
        echo "  Restoring from: $LATEST_BACKUP"
        rm -f /etc/nginx/conf.d/fermi-gateway.conf
        tar -xzf "$LATEST_BACKUP" -C /etc/nginx/conf.d/

        if nginx -t 2>/dev/null; then
            systemctl reload nginx
            echo "  ✓ Rollback successful"
        else
            echo "  ⚠ Warning: Backup config also has errors"
        fi
    else
        echo "  No backup found to restore"
    fi

    echo "  Check logs: sudo journalctl -u nginx -n 50"
    exit 1
}

# Step 1: Install nginx if needed
echo "Step 1: Checking Nginx installation..."
if ! command -v nginx &> /dev/null; then
    echo "  Installing Nginx..."
    yum install -y nginx
    echo "  ✓ Nginx installed"
else
    echo "  ✓ Nginx already installed"
fi
echo ""

# Step 2: Check DNS configuration
echo "Step 2: Verifying DNS configuration..."
PUBLIC_IP=$(curl -s http://169.254.169.254/latest/meta-data/public-ipv4 2>/dev/null || echo "unknown")
echo "  Server Public IP: $PUBLIC_IP"

if command -v dig &> /dev/null; then
    DOMAIN_IP=$(dig +short $DOMAIN @8.8.8.8 | tail -1)
    echo "  Domain $DOMAIN resolves to: $DOMAIN_IP"

    if [ "$PUBLIC_IP" != "$DOMAIN_IP" ]; then
        echo ""
        echo "  ⚠ WARNING: DNS mismatch!"
        echo "  Your domain does not point to this server."
        echo "  Please update your DNS A record:"
        echo "    Record: api.fermi.trade"
        echo "    Type: A"
        echo "    Value: $PUBLIC_IP"
        echo ""
        read -p "  Continue anyway? (y/N) " -n 1 -r
        echo
        if [[ ! $REPLY =~ ^[Yy]$ ]]; then
            echo "Exiting. Please fix DNS first."
            exit 1
        fi
    else
        echo "  ✓ DNS configured correctly"
    fi
else
    echo "  ⚠ dig not available, skipping DNS check"
fi
echo ""

# Step 3: Clean and backup existing configs
echo "Step 3: Cleaning and backing up existing configurations..."
mkdir -p $BACKUP_DIR

# Backup ALL existing nginx conf.d files
echo "  Creating backup of all nginx configs..."
if [ "$(ls -A /etc/nginx/conf.d/*.conf 2>/dev/null)" ]; then
    tar -czf "$BACKUP_DIR/nginx-configs-backup-$TIMESTAMP.tar.gz" -C /etc/nginx/conf.d . 2>/dev/null || true
    echo "  ✓ Backup created: $BACKUP_DIR/nginx-configs-backup-$TIMESTAMP.tar.gz"
else
    echo "  No existing configs to backup"
fi

# Remove ALL conflicting config files
echo "  Removing existing nginx configs..."
rm -f /etc/nginx/conf.d/default.conf
rm -f /etc/nginx/conf.d/fermi-gateway.conf
rm -f /etc/nginx/conf.d/fermi-gateway-temp.conf
rm -f /etc/nginx/conf.d/*.conf.disabled
rm -f /etc/nginx/sites-enabled/default 2>/dev/null || true

# List removed files for transparency
if [ -f "$BACKUP_DIR/nginx-configs-backup-$TIMESTAMP.tar.gz" ]; then
    echo "  ✓ Cleaned all existing nginx configs"
    echo "    (Backup available at: $BACKUP_DIR/nginx-configs-backup-$TIMESTAMP.tar.gz)"
else
    echo "  ✓ No conflicting configs found"
fi
echo ""

# Step 4: Deploy new configuration
echo "Step 4: Deploying new Nginx configuration..."
if [ -f "$GATEWAY_DIR/deployments/nginx-http-only.conf" ]; then
    cp "$GATEWAY_DIR/deployments/nginx-http-only.conf" /etc/nginx/conf.d/fermi-gateway.conf
    echo "  ✓ Configuration deployed: /etc/nginx/conf.d/fermi-gateway.conf"
else
    echo "  ✗ Error: Source config not found at $GATEWAY_DIR/deployments/nginx-http-only.conf"
    exit 1
fi
echo ""

# Step 5: Test configuration
echo "Step 5: Testing Nginx configuration..."
if nginx -t 2>&1; then
    echo "  ✓ Configuration syntax is valid"
else
    echo "  ✗ Configuration test failed!"
    rollback
fi
echo ""

# Step 6: Configure firewall
echo "Step 6: Configuring firewall..."
if command -v firewall-cmd &> /dev/null; then
    if firewall-cmd --state 2>/dev/null | grep -q "running"; then
        echo "  Opening ports 80 and 443..."
        firewall-cmd --permanent --add-service=http 2>/dev/null || echo "  HTTP already allowed"
        firewall-cmd --permanent --add-service=https 2>/dev/null || echo "  HTTPS already allowed"
        firewall-cmd --reload
        echo "  ✓ Firewall configured"
    else
        echo "  Firewalld not running"
    fi
else
    echo "  Firewalld not installed"
fi
echo ""

# Step 7: Enable and restart nginx
echo "Step 7: Starting Nginx..."
systemctl enable nginx
systemctl restart nginx

# Wait a moment for nginx to start
sleep 2

if systemctl is-active --quiet nginx; then
    echo "  ✓ Nginx is running"
else
    echo "  ✗ Nginx failed to start"
    rollback
fi
echo ""

# Step 8: Verify gateway is running
echo "Step 8: Verifying API Gateway..."
if curl -sf http://localhost:8080/health > /dev/null; then
    echo "  ✓ Gateway is responding on port 8080"
else
    echo "  ⚠ Warning: Gateway not responding on port 8080"
    echo "  Check gateway status: sudo systemctl status fermi-gateway"
fi
echo ""

# Step 9: Test nginx proxy
echo "Step 9: Testing Nginx proxy..."
echo ""
echo "  Testing localhost..."
if curl -sf http://localhost/health > /dev/null; then
    echo "  ✓ Nginx proxy works on localhost"
    echo "    Response:"
    curl -s http://localhost/health | jq '.' 2>/dev/null || curl -s http://localhost/health
else
    echo "  ✗ Nginx proxy test failed"
fi
echo ""

# Step 10: Summary
echo "=========================================="
echo "  Deployment Complete!"
echo "=========================================="
echo ""
echo "Configuration Details:"
echo "  Domain:       $DOMAIN"
echo "  Server IP:    $PUBLIC_IP"
echo "  Config file:  /etc/nginx/conf.d/fermi-gateway.conf"
echo ""
echo "Test Commands (run these):"
echo "  # From server:"
echo "  curl http://localhost/health"
echo "  curl http://localhost/api/v1/continuum/status"
echo ""
echo "  # From your local machine:"
echo "  curl http://$DOMAIN/health"
echo "  curl http://$PUBLIC_IP/health"
echo ""
echo "In Browser:"
echo "  http://$DOMAIN/health"
echo "  http://$DOMAIN/metrics"
echo ""
echo "Next Steps:"
echo "  1. Verify DNS is pointing correctly"
echo "  2. Check AWS Security Group allows port 80"
echo "  3. Test from browser: http://$DOMAIN/health"
echo "  4. Once working, set up SSL with Let's Encrypt"
echo ""
echo "Useful Commands:"
echo "  View logs:       sudo journalctl -u nginx -f"
echo "  Restart nginx:   sudo systemctl restart nginx"
echo "  Test config:     sudo nginx -t"
echo "  Check status:    sudo systemctl status nginx"
echo ""
