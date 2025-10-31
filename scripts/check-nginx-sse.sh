#!/bin/bash

# Script to check and update nginx configuration for SSE support
# Run on EC2: sudo bash scripts/check-nginx-sse.sh

set -e

echo "=========================================="
echo "  Nginx SSE Configuration Checker"
echo "=========================================="
echo ""

# Check if running as root
if [ "$EUID" -ne 0 ]; then
    echo "Please run as root (use sudo)"
    exit 1
fi

# Check if nginx is installed
if ! command -v nginx &> /dev/null; then
    echo "ERROR: Nginx is not installed"
    exit 1
fi

# Backup current config
NGINX_CONFIG="/etc/nginx/sites-available/fermi-gateway"
if [ -f "$NGINX_CONFIG" ]; then
    echo "Creating backup of current nginx config..."
    cp "$NGINX_CONFIG" "$NGINX_CONFIG.backup.$(date +%Y%m%d_%H%M%S)"
    echo "Backup created: $NGINX_CONFIG.backup.$(date +%Y%m%d_%H%M%S)"
else
    echo "WARNING: Nginx config not found at $NGINX_CONFIG"
    echo "Please ensure nginx is properly configured"
    exit 1
fi

echo ""
echo "Checking for SSE-related issues in nginx config..."
echo ""

# Check for proxy_buffering
if grep -q "proxy_buffering on" "$NGINX_CONFIG"; then
    echo "⚠️  ISSUE FOUND: proxy_buffering is ON (breaks SSE streaming)"
else
    echo "✅ proxy_buffering is not explicitly set to ON"
fi

# Check for SSE location block
if grep -q "location.*stream-ticks" "$NGINX_CONFIG"; then
    echo "✅ Found stream-ticks location block"
else
    echo "⚠️  WARNING: No specific location block for stream-ticks"
fi

# Check for appropriate timeouts
if grep -q "proxy_read_timeout.*30s" "$NGINX_CONFIG"; then
    echo "⚠️  ISSUE FOUND: proxy_read_timeout is 30s (too short for SSE)"
else
    echo "✅ proxy_read_timeout is acceptable"
fi

echo ""
echo "=========================================="
echo "  Recommended Fix"
echo "=========================================="
echo ""
echo "Add this location block BEFORE the general /api/ location block:"
echo ""
cat << 'EOF'
    # SSE streaming endpoint (stream-ticks) - special handling
    location /api/v1/continuum/stream-ticks {
        # Rate limiting
        limit_req zone=api burst=20 nodelay;
        limit_req_status 429;

        # Proxy to backend
        proxy_pass http://fermi_gateway;
        proxy_http_version 1.1;

        # Headers
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
        proxy_set_header X-Forwarded-Host $host;
        proxy_set_header Connection "";

        # SSE-specific settings - CRITICAL for streaming
        proxy_buffering off;                    # Disable buffering for real-time streaming
        proxy_cache off;                        # Disable caching
        proxy_read_timeout 24h;                 # Keep connection alive for long-running streams
        proxy_connect_timeout 10s;
        proxy_send_timeout 24h;

        # SSE headers
        add_header Cache-Control "no-cache, no-store, must-revalidate";
        add_header X-Accel-Buffering "no";      # Nginx-specific: disable buffering

        # Chunked transfer encoding (required for SSE)
        chunked_transfer_encoding on;
    }
EOF

echo ""
echo "=========================================="
echo "  Manual Steps"
echo "=========================================="
echo ""
echo "1. Edit the nginx config:"
echo "   sudo nano $NGINX_CONFIG"
echo ""
echo "2. Add the location block shown above BEFORE the /api/ location"
echo ""
echo "3. Test the configuration:"
echo "   sudo nginx -t"
echo ""
echo "4. If test passes, reload nginx:"
echo "   sudo systemctl reload nginx"
echo ""
echo "5. Test the stream endpoint:"
echo "   curl -N https://yourdomain.com/api/v1/continuum/stream-ticks"
echo ""

# Offer to create updated config
echo ""
read -p "Would you like to automatically update the nginx config? (y/N): " -n 1 -r
echo
if [[ $REPLY =~ ^[Yy]$ ]]; then
    echo ""
    echo "Creating updated nginx configuration..."

    # Create updated config with SSE support
    cat > /tmp/nginx-sse-update.conf << 'EOFCONFIG'
    # SSE streaming endpoint (stream-ticks) - special handling
    location /api/v1/continuum/stream-ticks {
        # Rate limiting
        limit_req zone=api burst=20 nodelay;
        limit_req_status 429;

        # Proxy to backend
        proxy_pass http://fermi_gateway;
        proxy_http_version 1.1;

        # Headers
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
        proxy_set_header X-Forwarded-Host $host;
        proxy_set_header Connection "";

        # SSE-specific settings - CRITICAL for streaming
        proxy_buffering off;                    # Disable buffering for real-time streaming
        proxy_cache off;                        # Disable caching
        proxy_read_timeout 24h;                 # Keep connection alive for long-running streams
        proxy_connect_timeout 10s;
        proxy_send_timeout 24h;

        # SSE headers
        add_header Cache-Control "no-cache, no-store, must-revalidate";
        add_header X-Accel-Buffering "no";      # Nginx-specific: disable buffering

        # Chunked transfer encoding (required for SSE)
        chunked_transfer_encoding on;
    }

EOFCONFIG

    echo "SSE config block created at /tmp/nginx-sse-update.conf"
    echo ""
    echo "You need to manually insert this block into your nginx config"
    echo "at line ~106 (BEFORE the general /api/ location block)"
    echo ""
    echo "Opening the config file for you..."
    sleep 2
    nano "$NGINX_CONFIG"

    # Test config after manual edit
    echo ""
    echo "Testing nginx configuration..."
    if nginx -t; then
        echo ""
        echo "✅ Nginx configuration is valid!"
        echo ""
        read -p "Reload nginx to apply changes? (y/N): " -n 1 -r
        echo
        if [[ $REPLY =~ ^[Yy]$ ]]; then
            systemctl reload nginx
            echo "✅ Nginx reloaded successfully!"
            echo ""
            echo "Test the SSE endpoint:"
            echo "curl -N https://yourdomain.com/api/v1/continuum/stream-ticks"
        fi
    else
        echo ""
        echo "❌ Nginx configuration test failed!"
        echo "Please review the errors above and fix them"
        echo ""
        echo "To restore backup:"
        echo "sudo cp $NGINX_CONFIG.backup.* $NGINX_CONFIG"
        echo "sudo systemctl reload nginx"
    fi
fi

echo ""
echo "Done!"
