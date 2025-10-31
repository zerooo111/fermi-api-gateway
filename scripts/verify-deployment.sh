#!/bin/bash

echo "=========================================="
echo "  Fermi API Gateway - Deployment Verification"
echo "=========================================="
echo ""

# Colors for output
GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Configuration
SERVICE_NAME="fermi-gateway"
APP_DIR="/opt/fermi-api-gateway"
HEALTH_URL="http://localhost:8080/health"
METRICS_URL="http://localhost:8080/metrics"

echo "1. Checking systemd service status..."
if systemctl is-active --quiet "$SERVICE_NAME"; then
    echo -e "${GREEN}✓ Service is running${NC}"
    systemctl status "$SERVICE_NAME" --no-pager -l | head -n 10
else
    echo -e "${RED}✗ Service is NOT running${NC}"
    systemctl status "$SERVICE_NAME" --no-pager -l | head -n 15
fi
echo ""

echo "2. Checking service is enabled (starts on boot)..."
if systemctl is-enabled --quiet "$SERVICE_NAME"; then
    echo -e "${GREEN}✓ Service is enabled${NC}"
else
    echo -e "${YELLOW}⚠ Service is not enabled (won't start on boot)${NC}"
    echo "  Run: sudo systemctl enable $SERVICE_NAME"
fi
echo ""

echo "3. Checking application binary..."
if [ -f "$APP_DIR/bin/fermi-api-gateway" ]; then
    echo -e "${GREEN}✓ Binary exists${NC}"
    ls -lh "$APP_DIR/bin/fermi-api-gateway"
else
    echo -e "${RED}✗ Binary not found at $APP_DIR/bin/fermi-api-gateway${NC}"
fi
echo ""

echo "4. Checking environment file..."
if [ -f "$APP_DIR/.env" ]; then
    echo -e "${GREEN}✓ Environment file exists${NC}"
    echo "  Checking required variables..."
    if grep -q "^PORT=" "$APP_DIR/.env"; then
        echo -e "  ${GREEN}✓ PORT configured${NC}"
    else
        echo -e "  ${YELLOW}⚠ PORT not set${NC}"
    fi
    if grep -q "^ALLOWED_ORIGINS=" "$APP_DIR/.env" && ! grep -q "yourdomain.com" "$APP_DIR/.env"; then
        echo -e "  ${GREEN}✓ ALLOWED_ORIGINS configured${NC}"
    else
        echo -e "  ${YELLOW}⚠ ALLOWED_ORIGINS not configured or using default${NC}"
    fi
    if grep -q "^ROLLUP_URL=" "$APP_DIR/.env" && ! grep -q "localhost:3000" "$APP_DIR/.env"; then
        echo -e "  ${GREEN}✓ ROLLUP_URL configured${NC}"
    else
        echo -e "  ${YELLOW}⚠ ROLLUP_URL not configured or using default${NC}"
    fi
else
    echo -e "${RED}✗ Environment file not found${NC}"
fi
echo ""

echo "5. Checking health endpoint..."
if curl -s -f "$HEALTH_URL" > /dev/null 2>&1; then
    echo -e "${GREEN}✓ Health endpoint is responding${NC}"
    echo "  Response:"
    curl -s "$HEALTH_URL" | jq '.' 2>/dev/null || curl -s "$HEALTH_URL"
else
    echo -e "${RED}✗ Health endpoint is NOT responding${NC}"
    echo "  URL: $HEALTH_URL"
    echo "  Check logs: sudo journalctl -u $SERVICE_NAME -n 50"
fi
echo ""

echo "6. Checking metrics endpoint (should be accessible)..."
if curl -s -f "$METRICS_URL" > /dev/null 2>&1; then
    echo -e "${GREEN}✓ Metrics endpoint is responding${NC}"
    METRICS_COUNT=$(curl -s "$METRICS_URL" | grep -c "^[^#]" || echo "0")
    echo "  Found $METRICS_COUNT metric lines"
else
    echo -e "${YELLOW}⚠ Metrics endpoint check failed (this might be normal if restricted)${NC}"
    echo "  URL: $METRICS_URL"
fi
echo ""

echo "7. Checking network ports..."
if netstat -tuln 2>/dev/null | grep -q ":8080 " || ss -tuln 2>/dev/null | grep -q ":8080 "; then
    echo -e "${GREEN}✓ Port 8080 is listening${NC}"
    netstat -tuln 2>/dev/null | grep ":8080 " || ss -tuln 2>/dev/null | grep ":8080 "
else
    echo -e "${RED}✗ Port 8080 is NOT listening${NC}"
fi
echo ""

echo "8. Checking recent logs (last 10 lines)..."
if journalctl -u "$SERVICE_NAME" -n 10 --no-pager > /dev/null 2>&1; then
    echo "  Recent log entries:"
    journalctl -u "$SERVICE_NAME" -n 10 --no-pager | tail -n 5
    ERROR_COUNT=$(journalctl -u "$SERVICE_NAME" --since "5 minutes ago" --no-pager | grep -ci "error" || echo "0")
    if [ "$ERROR_COUNT" -gt 0 ]; then
        echo -e "  ${YELLOW}⚠ Found $ERROR_COUNT error messages in last 5 minutes${NC}"
    else
        echo -e "  ${GREEN}✓ No recent errors${NC}"
    fi
else
    echo -e "${RED}✗ Cannot access logs${NC}"
fi
echo ""

echo "9. Checking systemd service file..."
if [ -f "/etc/systemd/system/$SERVICE_NAME.service" ]; then
    echo -e "${GREEN}✓ Service file exists${NC}"
    if grep -q "User=ec2-user" "/etc/systemd/system/$SERVICE_NAME.service"; then
        echo -e "  ${GREEN}✓ Running as ec2-user${NC}"
    else
        USER=$(grep "^User=" "/etc/systemd/system/$SERVICE_NAME.service" | cut -d= -f2)
        echo -e "  ${YELLOW}⚠ Running as: $USER${NC}"
    fi
else
    echo -e "${RED}✗ Service file not found${NC}"
fi
echo ""

echo "10. Checking disk space..."
DISK_USAGE=$(df -h "$APP_DIR" | tail -1 | awk '{print $5}' | sed 's/%//')
if [ "$DISK_USAGE" -lt 80 ]; then
    echo -e "${GREEN}✓ Disk usage: ${DISK_USAGE}%${NC}"
elif [ "$DISK_USAGE" -lt 90 ]; then
    echo -e "${YELLOW}⚠ Disk usage: ${DISK_USAGE}% (getting high)${NC}"
else
    echo -e "${RED}✗ Disk usage: ${DISK_USAGE}% (critical)${NC}"
fi
df -h "$APP_DIR" | tail -1
echo ""

echo "=========================================="
echo "  Verification Complete"
echo "=========================================="
echo ""
echo "Useful commands:"
echo "  View logs:       sudo journalctl -u $SERVICE_NAME -f"
echo "  Restart service: sudo systemctl restart $SERVICE_NAME"
echo "  Check status:    sudo systemctl status $SERVICE_NAME"
echo "  Test health:     curl $HEALTH_URL"
echo ""

