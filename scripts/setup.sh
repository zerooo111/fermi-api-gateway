#!/bin/bash

set -e  # Exit on error

echo "=========================================="
echo "  Fermi API Gateway - EC2 Setup"
echo "=========================================="
echo ""

# Check if running as root
if [ "$EUID" -ne 0 ]; then
    echo "Please run as root (use sudo)"
    exit 1
fi

# Get the actual user (not root when using sudo)
ACTUAL_USER=${SUDO_USER:-$USER}
USER_HOME=$(getent passwd "$ACTUAL_USER" | cut -d: -f6)

echo "Setting up for user: $ACTUAL_USER"
echo "Home directory: $USER_HOME"
echo ""

# Update system packages
echo "[1/8] Updating system packages..."
apt-get update
apt-get upgrade -y

# Install essential tools
echo "[2/8] Installing essential tools..."
apt-get install -y \
    curl \
    wget \
    unzip \
    git \
    build-essential \
    ufw \
    certbot \
    python3-certbot-nginx \
    jq

# Install Go
echo "[3/8] Installing Go..."
GO_VERSION="1.24.5"
if ! command -v go &> /dev/null; then
    cd /tmp
    wget "https://go.dev/dl/go${GO_VERSION}.linux-amd64.tar.gz"
    rm -rf /usr/local/go
    tar -C /usr/local -xzf "go${GO_VERSION}.linux-amd64.tar.gz"
    rm "go${GO_VERSION}.linux-amd64.tar.gz"

    # Add Go to PATH for all users
    echo 'export PATH=$PATH:/usr/local/go/bin' > /etc/profile.d/go.sh
    echo 'export GOPATH=$HOME/go' >> /etc/profile.d/go.sh
    chmod +x /etc/profile.d/go.sh

    # Source for current session
    export PATH=$PATH:/usr/local/go/bin
    export GOPATH=$USER_HOME/go

    echo "Go installed: $(go version)"
else
    echo "Go already installed: $(go version)"
fi

# Install Protocol Buffers compiler
echo "[4/8] Installing protoc and Go plugins..."
if ! command -v protoc &> /dev/null; then
    PROTOC_VERSION="28.3"
    cd /tmp
    wget "https://github.com/protocolbuffers/protobuf/releases/download/v${PROTOC_VERSION}/protoc-${PROTOC_VERSION}-linux-x86_64.zip"
    unzip "protoc-${PROTOC_VERSION}-linux-x86_64.zip" -d /usr/local
    rm "protoc-${PROTOC_VERSION}-linux-x86_64.zip"
    echo "protoc installed: $(protoc --version)"
else
    echo "protoc already installed: $(protoc --version)"
fi

# Install protoc-gen-go and protoc-gen-go-grpc as actual user
echo "Installing Go protobuf plugins for user $ACTUAL_USER..."
sudo -u "$ACTUAL_USER" bash << 'EOF'
export PATH=$PATH:/usr/local/go/bin
export GOPATH=$HOME/go
go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest
echo "export PATH=\$PATH:\$GOPATH/bin" >> ~/.bashrc
EOF

# Install Nginx
echo "[5/8] Installing Nginx..."
if ! command -v nginx &> /dev/null; then
    apt-get install -y nginx
    systemctl enable nginx
    echo "Nginx installed: $(nginx -v 2>&1)"
else
    echo "Nginx already installed: $(nginx -v 2>&1)"
fi

# Configure firewall
echo "[6/8] Configuring firewall..."
ufw --force enable
ufw allow 22/tcp    # SSH
ufw allow 80/tcp    # HTTP
ufw allow 443/tcp   # HTTPS
ufw allow 8080/tcp  # Gateway (for internal access)
ufw status verbose

# Create application directory
echo "[7/8] Creating application directory..."
APP_DIR="/opt/fermi-api-gateway"
mkdir -p "$APP_DIR"
mkdir -p "$APP_DIR/logs"
mkdir -p "$APP_DIR/bin"
chown -R "$ACTUAL_USER:$ACTUAL_USER" "$APP_DIR"

# Create environment file template
echo "[8/8] Creating environment file template..."
cat > "$APP_DIR/.env" << 'ENVEOF'
# Server Configuration
PORT=8080
ENV=production

# CORS Configuration
ALLOWED_ORIGINS=https://yourdomain.com,https://app.yourdomain.com

# Backend URLs (UPDATE THESE)
ROLLUP_URL=http://localhost:3000
CONTINUUM_GRPC_URL=localhost:9090
CONTINUUM_REST_URL=http://localhost:8081

# Rate Limiting (requests per minute)
RATE_LIMIT_ROLLUP=1000
RATE_LIMIT_CONTINUUM_GRPC=500
RATE_LIMIT_CONTINUUM_REST=2000
ENVEOF

chown "$ACTUAL_USER:$ACTUAL_USER" "$APP_DIR/.env"

echo ""
echo "=========================================="
echo "  Setup Complete!"
echo "=========================================="
echo ""
echo "Next steps:"
echo "  1. Clone your repository to $APP_DIR"
echo "  2. Update $APP_DIR/.env with your configuration"
echo "  3. Run the deploy script: sudo bash scripts/deploy.sh"
echo "  4. Run SSL setup: sudo bash scripts/setup-ssl.sh yourdomain.com"
echo ""
echo "Application directory: $APP_DIR"
echo "Logs directory: $APP_DIR/logs"
echo ""
