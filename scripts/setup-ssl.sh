#!/bin/bash

set -e  # Exit on error

echo "=========================================="
echo "  Fermi API Gateway - SSL Setup"
echo "=========================================="
echo ""

# Check if running as root
if [ "$EUID" -ne 0 ]; then
    echo "Please run as root (use sudo)"
    exit 1
fi

# Check arguments
if [ $# -lt 1 ]; then
    echo "Usage: sudo bash scripts/setup-ssl.sh <domain> [email]"
    echo ""
    echo "Example: sudo bash scripts/setup-ssl.sh api.yourdomain.com admin@yourdomain.com"
    echo ""
    echo "The email is optional but recommended for Let's Encrypt notifications."
    exit 1
fi

DOMAIN=$1
EMAIL=${2:-""}
APP_DIR="/opt/fermi-api-gateway"
NGINX_AVAILABLE="/etc/nginx/sites-available"
NGINX_ENABLED="/etc/nginx/sites-enabled"

# Create nginx config directories if they don't exist (for Amazon Linux)
mkdir -p "$NGINX_AVAILABLE"
mkdir -p "$NGINX_ENABLED"

echo "Domain: $DOMAIN"
if [ -n "$EMAIL" ]; then
    echo "Email: $EMAIL"
fi
echo ""

# Check if nginx is installed
if ! command -v nginx &> /dev/null; then
    echo "ERROR: Nginx is not installed. Please run setup.sh first."
    exit 1
fi

# Check if certbot is installed
if ! command -v certbot &> /dev/null; then
    echo "ERROR: Certbot is not installed. Please run setup.sh first."
    exit 1
fi

# Create certbot webroot directory
echo "[1/7] Creating certbot webroot directory..."
mkdir -p /var/www/certbot
# Use nginx user for Amazon Linux (www-data for Ubuntu)
NGINX_USER="nginx"
chown -R $NGINX_USER:$NGINX_USER /var/www/certbot

# Copy nginx configuration
echo "[2/7] Installing Nginx configuration..."
if [ ! -f "$APP_DIR/deployments/nginx.conf" ]; then
    echo "ERROR: Nginx configuration not found at $APP_DIR/deployments/nginx.conf"
    exit 1
fi

cp "$APP_DIR/deployments/nginx.conf" "$NGINX_AVAILABLE/fermi-gateway"

# Replace placeholder domain with actual domain
sed -i "s/yourdomain.com/$DOMAIN/g" "$NGINX_AVAILABLE/fermi-gateway"

echo "Nginx configuration installed"

# Create temporary HTTP-only configuration for initial certificate
echo "[3/7] Creating temporary HTTP configuration..."
cat > "$NGINX_AVAILABLE/fermi-gateway-temp" << EOF
server {
    listen 80;
    listen [::]:80;
    server_name $DOMAIN www.$DOMAIN;

    location /.well-known/acme-challenge/ {
        root /var/www/certbot;
    }

    location / {
        return 200 'OK';
        add_header Content-Type text/plain;
    }
}
EOF

# Disable default nginx site
if [ -L "$NGINX_ENABLED/default" ]; then
    rm "$NGINX_ENABLED/default"
fi

# Enable temporary configuration
ln -sf "$NGINX_AVAILABLE/fermi-gateway-temp" "$NGINX_ENABLED/fermi-gateway-temp"

# Test nginx configuration
echo "[4/7] Testing Nginx configuration..."
nginx -t

# Reload nginx
echo "Reloading Nginx..."
systemctl reload nginx

# Obtain SSL certificate
echo "[5/7] Obtaining SSL certificate from Let's Encrypt..."
if [ -n "$EMAIL" ]; then
    certbot certonly \
        --webroot \
        --webroot-path=/var/www/certbot \
        --email "$EMAIL" \
        --agree-tos \
        --no-eff-email \
        --force-renewal \
        -d "$DOMAIN" \
        -d "www.$DOMAIN"
else
    certbot certonly \
        --webroot \
        --webroot-path=/var/www/certbot \
        --register-unsafely-without-email \
        --agree-tos \
        --force-renewal \
        -d "$DOMAIN" \
        -d "www.$DOMAIN"
fi

# Check if certificate was obtained
if [ ! -f "/etc/letsencrypt/live/$DOMAIN/fullchain.pem" ]; then
    echo "ERROR: Failed to obtain SSL certificate"
    exit 1
fi

echo "SSL certificate obtained successfully!"

# Switch to full HTTPS configuration
echo "[6/7] Enabling full HTTPS configuration..."
rm "$NGINX_ENABLED/fermi-gateway-temp"
ln -sf "$NGINX_AVAILABLE/fermi-gateway" "$NGINX_ENABLED/fermi-gateway"

# Test nginx configuration again
echo "Testing Nginx configuration with SSL..."
nginx -t

# Reload nginx
echo "Reloading Nginx with SSL..."
systemctl reload nginx

# Setup automatic certificate renewal
echo "[7/7] Setting up automatic certificate renewal..."
cat > /etc/cron.d/certbot-renewal << 'EOF'
# Renew Let's Encrypt certificates twice daily
0 0,12 * * * root certbot renew --quiet --post-hook "systemctl reload nginx"
EOF

chmod 644 /etc/cron.d/certbot-renewal

echo ""
echo "=========================================="
echo "  SSL Setup Complete!"
echo "=========================================="
echo ""
echo "Certificate information:"
certbot certificates
echo ""
echo "Your site is now accessible at:"
echo "  https://$DOMAIN"
echo "  https://www.$DOMAIN"
echo ""
echo "Certificate will auto-renew twice daily via cron."
echo ""
echo "Useful commands:"
echo "  - Test renewal: sudo certbot renew --dry-run"
echo "  - View certificates: sudo certbot certificates"
echo "  - Reload nginx: sudo systemctl reload nginx"
echo ""
