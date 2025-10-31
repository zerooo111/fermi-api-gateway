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

# Detect Amazon Linux version
if [ -f /etc/os-release ]; then
    . /etc/os-release
    OS_VERSION_ID=${VERSION_ID%%.*}
fi

# Determine package manager
if command -v dnf &> /dev/null; then
    PKG_MANAGER="dnf"
    PKG_INSTALL="dnf install -y"
    PKG_UPDATE="dnf update -y"
elif command -v yum &> /dev/null; then
    PKG_MANAGER="yum"
    PKG_INSTALL="yum install -y"
    PKG_UPDATE="yum update -y"
else
    echo "ERROR: Neither yum nor dnf found. This script requires Amazon Linux."
    exit 1
fi

# Update system packages
echo "[1/8] Updating system packages..."
$PKG_UPDATE

# Handle curl conflict on Amazon Linux 2023 (curl-minimal vs curl)
# Remove curl-minimal if present and install full curl package
if rpm -q curl-minimal &> /dev/null; then
    echo "Removing curl-minimal to install full curl package..."
    $PKG_MANAGER remove -y curl-minimal 2>/dev/null || true
fi

# Install curl (or skip if already installed)
if ! command -v curl &> /dev/null; then
    echo "Installing curl..."
    $PKG_INSTALL curl || echo "Note: curl installation skipped (may already be present)"
fi

# Install essential tools and development packages
echo "[2/8] Installing essential tools..."
$PKG_INSTALL \
    wget \
    unzip \
    git \
    gcc \
    make \
    firewalld \
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

# Install Nginx (Amazon Linux Extras for AL2, or regular package for AL2023)
echo "[5/8] Installing Nginx..."
if ! command -v nginx &> /dev/null; then
    if [ "$PKG_MANAGER" = "yum" ]; then
        # Amazon Linux 2
        amazon-linux-extras install -y nginx1 || $PKG_INSTALL nginx
    else
        # Amazon Linux 2023+
        $PKG_INSTALL nginx
    fi
    
    # Create sites-available and sites-enabled directories (for Amazon Linux)
    mkdir -p /etc/nginx/sites-available
    mkdir -p /etc/nginx/sites-enabled
    
    # Ensure nginx main config includes sites-enabled
    NGINX_CONF="/etc/nginx/nginx.conf"
    if [ -f "$NGINX_CONF" ] && ! grep -q "sites-enabled" "$NGINX_CONF"; then
        # Try to add include directive near existing include statements
        if grep -q "include.*conf.d" "$NGINX_CONF"; then
            # Add after conf.d include line
            sed -i '/include.*conf.d/a\    include /etc/nginx/sites-enabled/*;' "$NGINX_CONF"
        elif grep -q "http {" "$NGINX_CONF"; then
            # Add after http { line
            sed -i '/http {/a\    include /etc/nginx/sites-enabled/*;' "$NGINX_CONF"
        fi
    fi
    
    systemctl enable nginx
    systemctl start nginx
    echo "Nginx installed: $(nginx -v 2>&1)"
else
    echo "Nginx already installed: $(nginx -v 2>&1)"
    # Ensure directories exist even if nginx is already installed
    mkdir -p /etc/nginx/sites-available
    mkdir -p /etc/nginx/sites-enabled
fi

# Configure firewall
echo "[6/8] Configuring firewall..."
systemctl enable firewalld
systemctl start firewalld
firewall-cmd --permanent --add-service=ssh
firewall-cmd --permanent --add-service=http
firewall-cmd --permanent --add-service=https
firewall-cmd --permanent --add-port=8080/tcp
firewall-cmd --reload
firewall-cmd --list-all

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
