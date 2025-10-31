# Fermi API Gateway - Deployment Guide

This guide walks you through deploying the Fermi API Gateway to an EC2 instance with SSL and monitoring.

## Table of Contents

1. [Prerequisites](#prerequisites)
2. [Initial Server Setup](#initial-server-setup)
3. [Application Deployment](#application-deployment)
4. [SSL Configuration](#ssl-configuration)
5. [Monitoring Setup](#monitoring-setup)
6. [Maintenance](#maintenance)
7. [Troubleshooting](#troubleshooting)

---

## Prerequisites

### EC2 Instance Requirements

- **Instance Type**: t3.medium or better (2 vCPU, 4GB RAM minimum)
- **OS**: Amazon Linux 2 or Amazon Linux 2023
- **Storage**: 20GB+ EBS volume
- **Security Group**: Ports 22 (SSH), 80 (HTTP), 443 (HTTPS) open

### Domain Configuration

- Domain name pointing to your EC2 instance's public IP
- DNS A records configured:
  - `yourdomain.com` → EC2 Public IP
  - `www.yourdomain.com` → EC2 Public IP

### Backend Services

- Rollup service accessible from EC2 instance
- Continuum gRPC service accessible from EC2 instance
- Continuum REST service accessible from EC2 instance

---

## Initial Server Setup

### Step 1: Connect to Your EC2 Instance

```bash
ssh -i your-key.pem ec2-user@your-ec2-ip
```

### Step 2: Install Git (Required for Cloning)

Amazon Linux doesn't include git by default. Install it first:

```bash
# For Amazon Linux 2
sudo yum install -y git

# For Amazon Linux 2023+
sudo dnf install -y git
```

### Step 3: Clone the Repository

```bash
# Clone to a temporary location first
cd /tmp
git clone https://github.com/fermilabs/fermi-api-gateway.git
cd fermi-api-gateway
```

**Alternative: Download as ZIP** (if git installation fails):

```bash
# Install unzip if not already available
sudo yum install -y unzip  # For Amazon Linux 2
# OR
sudo dnf install -y unzip  # For Amazon Linux 2023+

cd /tmp
curl -L https://github.com/fermilabs/fermi-api-gateway/archive/refs/heads/main.zip -o fermi-api-gateway.zip
unzip fermi-api-gateway.zip
mv fermi-api-gateway-main fermi-api-gateway
cd fermi-api-gateway
```

### Step 4: Run Initial Setup

This script installs all dependencies (Go, protobuf, Nginx, certbot, etc.)

```bash
sudo bash scripts/setup.sh
```

**What this script does:**

- Updates system packages
- Installs Go 1.24.5
- Installs protobuf compiler and Go plugins
- Installs Nginx (via Amazon Linux Extras for AL2, or regular package for AL2023)
- Installs certbot for SSL
- Configures firewall (firewalld)
- Creates `/opt/fermi-api-gateway` directory
- Creates environment file template

**Duration**: ~5-10 minutes

**Note**: Git is already included in the setup script's package installation, so if you skipped Step 2, it will be installed here.

### Step 5: Move Application to Installation Directory

```bash
# Remove existing directory if any
sudo rm -rf /opt/fermi-api-gateway

# Move application to installation directory
sudo mv /tmp/fermi-api-gateway /opt/fermi-api-gateway

# Set correct ownership
sudo chown -R ec2-user:ec2-user /opt/fermi-api-gateway

# Navigate to app directory
cd /opt/fermi-api-gateway
```

### Step 6: Configure Environment Variables

Edit the environment file with your actual configuration:

```bash
nano /opt/fermi-api-gateway/.env
```

**Required Configuration:**

```bash
# Server Configuration
PORT=8080
ENV=production

# CORS - UPDATE WITH YOUR DOMAINS
ALLOWED_ORIGINS=https://yourdomain.com,https://app.yourdomain.com

# Backend URLs - UPDATE WITH YOUR SERVICES
ROLLUP_URL=http://your-rollup-service:3000
CONTINUUM_GRPC_URL=your-continuum-host:9090
CONTINUUM_REST_URL=http://your-continuum-host:8080/api/v1

# Rate Limiting (adjust based on capacity)
RATE_LIMIT_ROLLUP=1000
RATE_LIMIT_CONTINUUM_GRPC=500
RATE_LIMIT_CONTINUUM_REST=2000
```

**Important Notes:**

- For `CONTINUUM_GRPC_URL`: Use `host:port` format (no protocol)
- For `CONTINUUM_REST_URL`: Include full URL with base path
- For `ALLOWED_ORIGINS`: Use comma-separated list, no spaces

Save and exit: `Ctrl+X`, then `Y`, then `Enter`

---

## Application Deployment

### Step 7: Deploy the Application

Run the deployment script:

```bash
sudo bash scripts/deploy.sh
```

**What this script does:**

1. Pulls latest code from git (if repository)
2. Installs/updates Go dependencies
3. Regenerates protobuf files if needed
4. Runs tests
5. Builds the binary
6. Installs systemd service
7. Starts the service
8. Verifies health endpoint

**Duration**: ~2-5 minutes

### Step 8: Verify Deployment

Check service status:

```bash
sudo systemctl status fermi-gateway
```

Test the health endpoint:

```bash
curl http://localhost:8080/health
```

Expected response:

```json
{
  "status": "healthy",
  "timestamp": "2025-10-30T...",
  "version": "1.0.0"
}
```

View logs:

```bash
# Follow logs in real-time
sudo journalctl -u fermi-gateway -f

# View last 50 lines
sudo journalctl -u fermi-gateway -n 50

# View logs from last hour
sudo journalctl -u fermi-gateway --since "1 hour ago"
```

---

## SSL Configuration

### Step 9: Obtain SSL Certificate

Run the SSL setup script with your domain:

```bash
sudo bash scripts/setup-ssl.sh api.yourdomain.com admin@yourdomain.com
```

**Arguments:**

- First argument: Your domain name (required)
- Second argument: Your email for Let's Encrypt notifications (optional but recommended)

**What this script does:**

1. Creates certbot webroot directory
2. Installs Nginx configuration
3. Sets up temporary HTTP configuration
4. Obtains SSL certificate from Let's Encrypt
5. Switches to full HTTPS configuration
6. Sets up automatic certificate renewal (cron job)

**Duration**: ~2-5 minutes

### Step 10: Verify SSL Configuration

Test HTTPS access:

```bash
curl https://api.yourdomain.com/health
```

Check certificate:

```bash
sudo certbot certificates
```

Test SSL configuration:

```bash
# Check SSL Labs rating (from your local machine)
# Visit: https://www.ssllabs.com/ssltest/analyze.html?d=api.yourdomain.com
```

Verify Nginx configuration:

```bash
sudo nginx -t
sudo systemctl status nginx
```

---

## Monitoring Setup

### Step 11: Verify Metrics Endpoint

The `/metrics` endpoint is restricted to private networks for security.

Test from the server:

```bash
curl http://localhost:8080/metrics
```

You should see Prometheus metrics output.

### Step 12: Configure External Monitoring (Optional)

If you have a Prometheus server, add this target to your `prometheus.yml`:

```yaml
scrape_configs:
  - job_name: "fermi-gateway"
    static_configs:
      - targets: ["your-ec2-private-ip:8080"]
    metrics_path: "/metrics"
```

---

## Maintenance

### Updating the Application

To deploy new code:

```bash
cd /opt/fermi-api-gateway
git pull origin main
sudo bash scripts/deploy.sh
```

The deploy script will automatically:

- Pull latest code
- Run tests
- Build new binary
- Restart service with zero downtime (graceful restart)

### Restarting the Service

```bash
# Restart service
sudo systemctl restart fermi-gateway

# Check status
sudo systemctl status fermi-gateway

# View recent logs
sudo journalctl -u fermi-gateway -n 50
```

### Updating Environment Variables

1. Edit `.env` file:

   ```bash
   sudo nano /opt/fermi-api-gateway/.env
   ```

2. Restart service to apply changes:
   ```bash
   sudo systemctl restart fermi-gateway
   ```

### Certificate Renewal

Certificates automatically renew via cron job. To manually test renewal:

```bash
# Dry run (test only, doesn't actually renew)
sudo certbot renew --dry-run

# Force renewal (if needed)
sudo certbot renew --force-renewal
sudo systemctl reload nginx
```

### Log Management

Logs are managed by systemd's journald. To configure log rotation:

```bash
# View current journal size
sudo journalctl --disk-usage

# Rotate logs manually
sudo journalctl --rotate

# Vacuum logs older than 7 days
sudo journalctl --vacuum-time=7d
```

#### Configure automatic rotation limits (systemd-journald)

Systemd's journald handles rotation automatically. Set sensible caps to prevent unbounded growth:

```bash
# Open journald config
sudo nano /etc/systemd/journald.conf
```

Uncomment and adjust these options as needed (examples shown):

```
SystemMaxUse=1G          # Max total size on disk
SystemMaxFileSize=200M   # Max size per journal file
SystemMaxFiles=10        # Max number of files retained
# Optionally enforce time/size vacuums in addition to the above caps:
# RuntimeMaxUse=500M
```

Apply changes:

```bash
sudo systemctl restart systemd-journald
```

You can also vacuum on demand by size or time:

```bash
sudo journalctl --vacuum-size=500M   # keep latest 500MB
sudo journalctl --vacuum-time=14d    # keep last 14 days
```

---

## Troubleshooting

### Service Won't Start

**Check logs for errors:**

```bash
sudo journalctl -u fermi-gateway -n 100 --no-pager
```

**Common issues:**

- Missing `.env` file → Run `sudo bash scripts/deploy.sh` again
- Port 8080 already in use → Check with `sudo lsof -i :8080`
- Backend services unreachable → Verify URLs in `.env`

### Package Installation Issues

**curl-minimal conflict (Amazon Linux 2023):**

If you see a conflict error when installing curl:

```bash
# Remove curl-minimal first
sudo dnf remove -y curl-minimal
# Then install curl
sudo dnf install -y curl
```

The setup script handles this automatically, but if you're installing packages manually, use the above.

### SSL Certificate Issues

**Certificate not obtained:**

- Verify DNS records point to correct IP
- Check firewall allows port 80
- Verify domain is accessible: `curl http://yourdomain.com`

**Certificate expired:**

```bash
sudo certbot renew --force-renewal
sudo systemctl reload nginx
```

### High CPU/Memory Usage

**Check resource usage:**

```bash
top
htop  # if installed
```

**Check connection counts:**

```bash
netstat -an | grep :8080 | wc -l
```

**Adjust rate limits in `.env` if needed, then restart:**

```bash
sudo systemctl restart fermi-gateway
```

### Nginx Issues

**Test configuration:**

```bash
sudo nginx -t
```

**View error logs:**

```bash
sudo tail -f /var/log/nginx/fermi-gateway-error.log
```

**Restart Nginx:**

```bash
sudo systemctl restart nginx
```

### Backend Connection Issues

**Test backend connectivity from server:**

```bash
# Test Continuum REST
curl http://your-continuum-host:8080/api/v1/health

# Test Continuum gRPC (requires grpcurl)
grpcurl -plaintext your-continuum-host:9090 list
```

**Check gateway logs for backend errors:**

```bash
sudo journalctl -u fermi-gateway | grep -i error
```

---

## Quick Reference

### Important Paths

- Application: `/opt/fermi-api-gateway`
- Binary: `/opt/fermi-api-gateway/bin/fermi-api-gateway`
- Environment: `/opt/fermi-api-gateway/.env`
- Logs: `/opt/fermi-api-gateway/logs/`
- Systemd service: `/etc/systemd/system/fermi-gateway.service`
- Nginx config: `/etc/nginx/sites-available/fermi-gateway`
- SSL certificates: `/etc/letsencrypt/live/yourdomain.com/`

### Common Commands

```bash
# Service management
sudo systemctl start fermi-gateway
sudo systemctl stop fermi-gateway
sudo systemctl restart fermi-gateway
sudo systemctl status fermi-gateway

# Logs
sudo journalctl -u fermi-gateway -f          # Follow logs
sudo journalctl -u fermi-gateway -n 100      # Last 100 lines
sudo journalctl -u fermi-gateway --since "1 hour ago"

# Nginx
sudo nginx -t                                 # Test config
sudo systemctl reload nginx                   # Reload config
sudo systemctl restart nginx                  # Restart nginx

# SSL
sudo certbot certificates                     # View certificates
sudo certbot renew --dry-run                 # Test renewal
sudo certbot renew                           # Renew certificates

# Deployment
sudo bash scripts/deploy.sh                  # Deploy new version
```

### Health Checks

```bash
# Local health check
curl http://localhost:8080/health

# HTTPS health check
curl https://yourdomain.com/health

# Metrics
curl http://localhost:8080/metrics

# Full API test
curl https://yourdomain.com/api/continuum/rest/health
```

---

## Security Best Practices

1. **Keep system updated:**

   ```bash
   # For Amazon Linux 2
   sudo yum update -y

   # For Amazon Linux 2023+
   sudo dnf update -y
   ```

2. **Use SSH keys only** (disable password authentication)

3. **Configure fail2ban** for SSH brute force protection (optional, Amazon Linux may not include it by default)

4. **Regularly review logs** for suspicious activity

5. **Monitor certificate expiration** (auto-renewal should handle this)

6. **Backup `.env` file** securely (contains configuration)

7. **Use security groups** to restrict access to necessary ports only

8. **Enable AWS CloudWatch** for additional monitoring

---

## Support

For issues, questions, or contributions:

- GitHub Issues: https://github.com/fermilabs/fermi-api-gateway/issues
- Documentation: https://github.com/fermilabs/fermi-api-gateway/docs

---

**Last Updated**: 2025-10-30
