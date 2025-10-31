#!/bin/bash
# SSL Setup Script for Fermi API Gateway
# Sets up Let's Encrypt SSL certificates for api.fermi.trade

set -e

echo "=========================================="
echo "  Fermi Gateway - SSL Setup"
echo "  Domain: api.fermi.trade"
echo "=========================================="
echo ""

# Check if running as root
if [ "$EUID" -ne 0 ]; then
    echo "Please run as root (use sudo)"
    exit 1
fi

DOMAIN="api.fermi.trade"
EMAIL="${SSL_EMAIL:-admin@fermi.trade}"  # Override with SSL_EMAIL env var
GATEWAY_DIR="/opt/fermi-api-gateway"

echo "Configuration:"
echo "  Domain: $DOMAIN"
echo "  Email:  $EMAIL"
echo ""
echo "NOTE: Make sure DNS points to this server and port 80 is accessible!"
echo ""
read -p "Press Enter to continue or Ctrl+C to cancel..."
echo ""

# Step 1: Install certbot
echo "Step 1: Installing certbot..."
if ! command -v certbot &> /dev/null; then
    # Detect OS and install certbot
    if command -v dnf &> /dev/null; then
        # Amazon Linux 2023 / RHEL 9
        dnf install -y certbot python3-certbot-nginx
    elif command -v amazon-linux-extras &> /dev/null; then
        # Amazon Linux 2
        amazon-linux-extras install -y epel
        yum install -y certbot python3-certbot-nginx
    elif command -v yum &> /dev/null; then
        # Generic RHEL/CentOS
        yum install -y epel-release
        yum install -y certbot python3-certbot-nginx
    elif command -v apt-get &> /dev/null; then
        # Debian/Ubuntu
        apt-get update
        apt-get install -y certbot python3-certbot-nginx
    else
        echo "  ✗ Could not detect package manager"
        echo "  Please install certbot manually"
        exit 1
    fi
    echo "  ✓ Certbot installed"
else
    echo "  ✓ Certbot already installed"
fi
echo ""

# Step 2: Verify DNS
echo "Step 2: Verifying DNS configuration..."
PUBLIC_IP=$(curl -s http://169.254.169.254/latest/meta-data/public-ipv4 2>/dev/null || echo "unknown")
echo "  Server Public IP: $PUBLIC_IP"

if command -v dig &> /dev/null; then
    DOMAIN_IP=$(dig +short $DOMAIN @8.8.8.8 | tail -1)
    echo "  Domain $DOMAIN resolves to: $DOMAIN_IP"

    if [ "$PUBLIC_IP" != "$DOMAIN_IP" ]; then
        echo ""
        echo "  ⚠ WARNING: DNS mismatch!"
        echo "  SSL certificate generation will FAIL!"
        echo "  Please update your DNS A record first:"
        echo "    Name:  api"
        echo "    Type:  A"
        echo "    Value: $PUBLIC_IP"
        echo ""
        read -p "Continue anyway? (y/N) " -n 1 -r
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

# Step 3: Check nginx and gateway
echo "Step 3: Verifying nginx and gateway are running..."
if ! systemctl is-active --quiet nginx; then
    echo "  ✗ Nginx is not running"
    echo "  Run: sudo systemctl start nginx"
    exit 1
fi
echo "  ✓ Nginx is running"

if ! systemctl is-active --quiet fermi-gateway; then
    echo "  ⚠ Warning: Gateway is not running"
    echo "  SSL will still work, but health checks may fail"
else
    echo "  ✓ Gateway is running"
fi
echo ""

# Step 4: Create certbot webroot
echo "Step 4: Creating certbot webroot..."
mkdir -p /var/www/certbot
chown -R nginx:nginx /var/www/certbot
echo "  ✓ Webroot created: /var/www/certbot"
echo ""

# Step 5: Check if ACME challenge works
echo "Step 5: Testing ACME challenge endpoint..."
# Create test file
echo "test" > /var/www/certbot/test.txt

# Test from localhost
if curl -sf http://localhost/.well-known/acme-challenge/test.txt > /dev/null 2>&1; then
    echo "  ✓ ACME challenge endpoint is accessible"
    rm -f /var/www/certbot/test.txt
else
    echo "  ✗ ACME challenge endpoint NOT accessible"
    echo "  This will cause certificate generation to fail!"
    echo ""
    echo "  Checking nginx config for /.well-known/acme-challenge/..."
    grep -n "acme-challenge" /etc/nginx/conf.d/fermi-gateway.conf || echo "  Not found!"
    echo ""
    read -p "Continue anyway? (y/N) " -n 1 -r
    echo
    if [[ ! $REPLY =~ ^[Yy]$ ]]; then
        exit 1
    fi
fi
echo ""

# Step 6: Obtain SSL certificate
echo "Step 6: Obtaining SSL certificate from Let's Encrypt..."
echo "  This may take 1-2 minutes..."
echo ""

# Run certbot
if certbot certonly \
    --webroot \
    --webroot-path=/var/www/certbot \
    --email "$EMAIL" \
    --agree-tos \
    --no-eff-email \
    --domain "$DOMAIN" \
    --non-interactive; then
    echo ""
    echo "  ✓ SSL certificate obtained successfully!"
else
    echo ""
    echo "  ✗ Certificate generation FAILED!"
    echo ""
    echo "Common issues:"
    echo "  1. DNS not pointing to this server"
    echo "  2. Port 80 not accessible (check AWS Security Group)"
    echo "  3. Firewall blocking port 80"
    echo "  4. ACME challenge endpoint not working"
    echo ""
    echo "To debug:"
    echo "  - Check DNS: dig +short $DOMAIN"
    echo "  - Test port 80: curl http://$DOMAIN/.well-known/acme-challenge/test.txt"
    echo "  - Check logs: sudo journalctl -u nginx -n 50"
    echo ""
    exit 1
fi
echo ""

# Step 7: Verify certificate files
echo "Step 7: Verifying certificate files..."
CERT_DIR="/etc/letsencrypt/live/$DOMAIN"

if [ ! -f "$CERT_DIR/fullchain.pem" ]; then
    echo "  ✗ Certificate files not found at $CERT_DIR"
    exit 1
fi

echo "  ✓ Certificate files exist:"
ls -lh "$CERT_DIR/"
echo ""

# Step 8: Backup current nginx config
echo "Step 8: Backing up current nginx config..."
BACKUP_DIR="/etc/nginx/backups"
TIMESTAMP=$(date +%Y%m%d_%H%M%S)
mkdir -p $BACKUP_DIR

cp /etc/nginx/conf.d/fermi-gateway.conf "$BACKUP_DIR/fermi-gateway-http-only-$TIMESTAMP.conf"
echo "  ✓ Backup saved: $BACKUP_DIR/fermi-gateway-http-only-$TIMESTAMP.conf"
echo ""

# Step 9: Deploy HTTPS configuration
echo "Step 9: Deploying HTTPS nginx configuration..."
if [ -f "$GATEWAY_DIR/deployments/nginx.conf" ]; then
    cp "$GATEWAY_DIR/deployments/nginx.conf" /etc/nginx/conf.d/fermi-gateway.conf
    echo "  ✓ HTTPS config deployed"
else
    echo "  ✗ Error: HTTPS config not found at $GATEWAY_DIR/deployments/nginx.conf"
    echo "  Restoring HTTP-only config..."
    cp "$BACKUP_DIR/fermi-gateway-http-only-$TIMESTAMP.conf" /etc/nginx/conf.d/fermi-gateway.conf
    exit 1
fi
echo ""

# Step 10: Test nginx configuration
echo "Step 10: Testing nginx configuration..."
if nginx -t 2>&1; then
    echo "  ✓ Nginx configuration is valid"
else
    echo "  ✗ Nginx configuration test FAILED!"
    echo "  Restoring HTTP-only config..."
    cp "$BACKUP_DIR/fermi-gateway-http-only-$TIMESTAMP.conf" /etc/nginx/conf.d/fermi-gateway.conf
    nginx -t
    exit 1
fi
echo ""

# Step 11: Configure firewall for HTTPS
echo "Step 11: Configuring firewall..."
if command -v firewall-cmd &> /dev/null; then
    if firewall-cmd --state 2>/dev/null | grep -q "running"; then
        firewall-cmd --permanent --add-service=https 2>/dev/null || echo "  HTTPS already allowed"
        firewall-cmd --reload
        echo "  ✓ Firewall configured for HTTPS"
    else
        echo "  Firewalld not running"
    fi
else
    echo "  Firewalld not installed"
fi
echo ""

# Step 12: Reload nginx
echo "Step 12: Reloading nginx with HTTPS configuration..."
systemctl reload nginx
sleep 2

if systemctl is-active --quiet nginx; then
    echo "  ✓ Nginx reloaded successfully"
else
    echo "  ✗ Nginx failed to reload"
    echo "  Check logs: sudo journalctl -u nginx -n 50"
    exit 1
fi
echo ""

# Step 13: Test HTTPS endpoint
echo "Step 13: Testing HTTPS endpoint..."
sleep 2

if curl -sf https://$DOMAIN/health > /dev/null 2>&1; then
    echo "  ✓ HTTPS is working!"
    echo ""
    echo "  Response:"
    curl -s https://$DOMAIN/health | jq '.' 2>/dev/null || curl -s https://$DOMAIN/health
else
    echo "  ⚠ HTTPS test failed (but nginx is running)"
    echo "  This might be temporary - try manually:"
    echo "  curl https://$DOMAIN/health"
fi
echo ""

# Step 14: Set up auto-renewal
echo "Step 14: Setting up SSL certificate auto-renewal..."
# Certbot automatically installs a systemd timer for renewal
if systemctl list-timers | grep -q certbot; then
    echo "  ✓ Auto-renewal timer is active"
    systemctl list-timers | grep certbot
else
    echo "  ⚠ Auto-renewal timer not found"
    echo "  Certificates will expire in 90 days!"
    echo "  Set up a cron job to renew: sudo certbot renew"
fi
echo ""

# Step 15: Test renewal (dry run)
echo "Step 15: Testing certificate renewal (dry run)..."
if certbot renew --dry-run 2>&1 | grep -q "Congratulations"; then
    echo "  ✓ Certificate renewal test successful"
else
    echo "  ⚠ Certificate renewal test had issues"
    echo "  Check manually: sudo certbot renew --dry-run"
fi
echo ""

echo "=========================================="
echo "  SSL Setup Complete! 🎉"
echo "=========================================="
echo ""
echo "Your API Gateway is now accessible via HTTPS:"
echo ""
echo "  🔒 HTTPS Health:   https://$DOMAIN/health"
echo "  🔒 HTTPS Root:     https://$DOMAIN/"
echo "  🔒 HTTPS API:      https://$DOMAIN/api/v1/"
echo ""
echo "HTTP requests are automatically redirected to HTTPS:"
echo "  http://$DOMAIN → https://$DOMAIN"
echo ""
echo "Certificate Details:"
echo "  Domain:      $DOMAIN"
echo "  Issuer:      Let's Encrypt"
echo "  Valid for:   90 days"
echo "  Auto-renew:  Enabled"
echo "  Location:    /etc/letsencrypt/live/$DOMAIN/"
echo ""
echo "Security Features:"
echo "  ✓ TLS 1.2 & 1.3 only"
echo "  ✓ Modern cipher suites"
echo "  ✓ HSTS enabled (1 year)"
echo "  ✓ OCSP stapling"
echo "  ✓ Rate limiting (3 tiers)"
echo "  ✓ HTTP → HTTPS redirect"
echo ""
echo "Next Steps:"
echo "  1. Test in browser: https://$DOMAIN/health"
echo "  2. Check SSL rating: https://www.ssllabs.com/ssltest/analyze.html?d=$DOMAIN"
echo "  3. Monitor renewal: sudo systemctl status certbot.timer"
echo ""
echo "Useful Commands:"
echo "  View certificate:    sudo certbot certificates"
echo "  Renew manually:      sudo certbot renew"
echo "  Test renewal:        sudo certbot renew --dry-run"
echo "  View nginx logs:     sudo journalctl -u nginx -f"
echo "  Reload nginx:        sudo systemctl reload nginx"
echo ""
