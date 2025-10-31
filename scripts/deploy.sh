#!/bin/bash

set -e  # Exit on error

echo "=========================================="
echo "  Fermi API Gateway - Deployment"
echo "=========================================="
echo ""

# Configuration
APP_DIR="/opt/fermi-api-gateway"
APP_NAME="fermi-api-gateway"
SERVICE_NAME="fermi-gateway"
BUILD_BINARY="$APP_DIR/bin/$APP_NAME"
LOG_FILE="$APP_DIR/logs/deploy.log"

# Check if running as root
if [ "$EUID" -ne 0 ]; then
    echo "Please run as root (use sudo)"
    exit 1
fi

# Get the actual user (not root when using sudo)
ACTUAL_USER=${SUDO_USER:-$USER}

# Ensure logs directory exists
mkdir -p "$APP_DIR/logs"
chown "$ACTUAL_USER:$ACTUAL_USER" "$APP_DIR/logs" 2>/dev/null || true

# Log function
log() {
    echo "[$(date '+%Y-%m-%d %H:%M:%S')] $1" | tee -a "$LOG_FILE"
}

log "Starting deployment..."

# Check if app directory exists
if [ ! -d "$APP_DIR" ]; then
    log "ERROR: Application directory $APP_DIR does not exist"
    log "Please run setup.sh first"
    exit 1
fi

# Navigate to app directory
cd "$APP_DIR"

# Check if .env file exists
if [ ! -f "$APP_DIR/.env" ]; then
    log "WARNING: .env file not found. Creating from .env.example"
    if [ -f "$APP_DIR/.env.example" ]; then
        cp "$APP_DIR/.env.example" "$APP_DIR/.env"
        log "Created .env file. Please update it with your configuration."
    else
        log "ERROR: Neither .env nor .env.example found"
        exit 1
    fi
fi

# Pull latest code (if this is a git repo)
if [ -d ".git" ]; then
    log "Pulling latest code from git..."
    sudo -u "$ACTUAL_USER" git pull origin main || log "Warning: Git pull failed or not needed"
else
    log "Not a git repository, skipping git pull"
fi

# Install/update Go dependencies
log "Installing Go dependencies..."
sudo -u "$ACTUAL_USER" bash << 'EOF'
export PATH=$PATH:/usr/local/go/bin
export GOPATH=$HOME/go
go mod download
go mod tidy
EOF

# Regenerate protobuf files if proto directory exists
if [ -d "proto" ] && [ -f "proto/continuum.proto" ]; then
    log "Regenerating protobuf files..."
    sudo -u "$ACTUAL_USER" bash << 'EOF'
export PATH=$PATH:/usr/local/go/bin:$HOME/go/bin
export GOPATH=$HOME/go
cd proto
protoc --go_out=. --go_opt=paths=source_relative \
       --go-grpc_out=. --go-grpc_opt=paths=source_relative \
       continuum.proto
EOF
    log "Protobuf files regenerated"
fi

# Run tests
log "Running tests..."
sudo -u "$ACTUAL_USER" bash << 'EOF'
export PATH=$PATH:/usr/local/go/bin
export GOPATH=$HOME/go
if go test ./... > /tmp/test-output.log 2>&1; then
    echo "All tests passed"
else
    echo "WARNING: Some tests failed. Check /tmp/test-output.log"
    cat /tmp/test-output.log
fi
EOF

# Build the application
log "Building application..."
sudo -u "$ACTUAL_USER" bash << 'EOF'
export PATH=$PATH:/usr/local/go/bin
export GOPATH=$HOME/go
go build -o bin/fermi-api-gateway -ldflags="-s -w" ./cmd/gateway
EOF

if [ ! -f "$BUILD_BINARY" ]; then
    log "ERROR: Build failed - binary not found at $BUILD_BINARY"
    exit 1
fi

log "Build successful: $(ls -lh $BUILD_BINARY)"

# Install systemd service file if not already installed
if [ ! -f "/etc/systemd/system/$SERVICE_NAME.service" ]; then
    log "Installing systemd service file..."
    if [ -f "$APP_DIR/deployments/fermi-gateway.service" ]; then
        cp "$APP_DIR/deployments/fermi-gateway.service" "/etc/systemd/system/$SERVICE_NAME.service"
        systemctl daemon-reload
        log "Systemd service file installed"
    else
        log "WARNING: Systemd service file not found at $APP_DIR/deployments/fermi-gateway.service"
    fi
fi

# Stop the service if it's running
log "Stopping service..."
if systemctl is-active --quiet "$SERVICE_NAME"; then
    systemctl stop "$SERVICE_NAME"
    log "Service stopped"
else
    log "Service is not running"
fi

# Set proper permissions
log "Setting permissions..."
chown -R "$ACTUAL_USER:$ACTUAL_USER" "$APP_DIR"
chmod +x "$BUILD_BINARY"

# Start the service
log "Starting service..."
systemctl daemon-reload
systemctl enable "$SERVICE_NAME"
systemctl start "$SERVICE_NAME"

# Wait a moment for the service to start
sleep 3

# Check service status
if systemctl is-active --quiet "$SERVICE_NAME"; then
    log "Service started successfully"

    # Test health endpoint
    log "Testing health endpoint..."
    sleep 2
    if curl -s http://localhost:8080/health > /dev/null; then
        log "Health check passed!"
        curl -s http://localhost:8080/health | jq '.'
    else
        log "WARNING: Health check failed"
    fi
else
    log "ERROR: Service failed to start"
    log "Checking service status:"
    systemctl status "$SERVICE_NAME" --no-pager
    exit 1
fi

# Show logs
log "Recent logs:"
journalctl -u "$SERVICE_NAME" -n 20 --no-pager

echo ""
echo "=========================================="
echo "  Deployment Complete!"
echo "=========================================="
echo ""
echo "Service status: $(systemctl is-active $SERVICE_NAME)"
echo "View logs: journalctl -u $SERVICE_NAME -f"
echo "Restart: sudo systemctl restart $SERVICE_NAME"
echo "Stop: sudo systemctl stop $SERVICE_NAME"
echo ""
