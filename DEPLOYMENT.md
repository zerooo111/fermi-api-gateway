# Fermi API Gateway - Deployment Guide

## Quick Start - Deploy to Production

### Prerequisites
- EC2 instance with public IP: `3.128.173.48`
- Domain: `api.fermi.trade` pointing to your server IP
- AWS Security Group allowing ports 80, 443, 8080

### 1. Upload Files to Server

From your local machine:

```bash
cd /Users/zeroo111/Developer/fermi-api-gateway

# Upload deployment files
scp deployments/nginx-http-only.conf ec2-user@3.128.173.48:/opt/fermi-api-gateway/deployments/
scp scripts/deploy-nginx.sh ec2-user@3.128.173.48:/opt/fermi-api-gateway/scripts/

# Make script executable
ssh ec2-user@3.128.173.48 "chmod +x /opt/fermi-api-gateway/scripts/deploy-nginx.sh"
```

### 2. Deploy Nginx

SSH into your server and run:

```bash
cd /opt/fermi-api-gateway
sudo bash scripts/deploy-nginx.sh
```

The script will:
- ✅ Install nginx (if needed)
- ✅ Verify DNS configuration
- ✅ Backup existing configs (to `/etc/nginx/backups/`)
- ✅ Remove conflicting configs
- ✅ Deploy production-grade configuration
- ✅ Test configuration syntax
- ✅ Configure firewall
- ✅ Restart nginx
- ✅ Run health checks
- ✅ Auto-rollback on failure

### 3. Verify Deployment

Test these endpoints:

```bash
# From server
curl http://localhost/health
curl http://localhost/api/v1/continuum/status

# From your local machine or browser
curl http://api.fermi.trade/health
curl http://api.fermi.trade/
```

In browser:
- http://api.fermi.trade/health
- http://api.fermi.trade/metrics (should be blocked from external IPs)

## Security Features

### Rate Limiting
- **General endpoints**: 100 req/sec (burst: 50)
- **API endpoints** (`/api/`): 50 req/sec (burst: 20)
- **Streaming** (`/stream-ticks`): 20 req/sec (burst: 10)
- Returns `429` JSON response when exceeded

### Connection Limits
- **10 concurrent connections per IP** to prevent connection exhaustion

### Security Headers
```
X-Frame-Options: DENY
X-Content-Type-Options: nosniff
X-XSS-Protection: 1; mode=block
Referrer-Policy: strict-origin-when-cross-origin
Permissions-Policy: geolocation=(), microphone=(), camera=()
```

### Metrics Protection
- `/metrics` endpoint restricted to internal IPs only:
  - `127.0.0.1` (localhost)
  - `10.0.0.0/8` (private network)
  - `172.16.0.0/12` (private network)
  - `192.168.0.0/16` (private network)

### Client Limits
- **Max body size**: 10MB
- **Client timeouts**: 10s (prevents slowloris attacks)
- **Hidden files blocked**: Returns 404 for `/.git`, `/.env`, etc.

### Error Handling
- Upstream retry logic (2 attempts, 10s timeout)
- Custom JSON error pages:
  - `429` - Rate limit exceeded
  - `403` - Forbidden
  - `404` - Not found
  - `502/503/504` - Service unavailable

## Configuration Details

### Upstream Configuration
```nginx
upstream fermi_gateway {
    server 127.0.0.1:8080 max_fails=3 fail_timeout=30s;
    keepalive 32;
    keepalive_requests 1000;
    keepalive_timeout 60s;
}
```

### SSE Streaming Settings
Special configuration for `/api/v1/continuum/stream-ticks`:
- `proxy_buffering off` - Real-time streaming
- `proxy_read_timeout 24h` - Long-running connections
- `chunked_transfer_encoding on`
- `X-Accel-Buffering: no`

### Compression
Gzip compression enabled for:
- `text/plain`, `text/css`, `text/xml`
- `application/json`, `application/javascript`
- `application/xml+rss`, `application/atom+xml`

## Useful Commands

### Nginx Management
```bash
# Test configuration
sudo nginx -t

# Reload (without downtime)
sudo systemctl reload nginx

# Restart
sudo systemctl restart nginx

# Check status
sudo systemctl status nginx

# View logs (live)
sudo journalctl -u nginx -f

# View error log
sudo tail -f /var/log/nginx/fermi-gateway-error.log

# View access log
sudo tail -f /var/log/nginx/fermi-gateway-access.log
```

### Gateway Management
```bash
# Check gateway status
sudo systemctl status fermi-gateway

# View gateway logs
sudo journalctl -u fermi-gateway -f

# Restart gateway
sudo systemctl restart fermi-gateway
```

### Diagnostics
```bash
# Check what's listening on port 80
sudo ss -tlnp | grep :80

# Check what's listening on port 8080
sudo ss -tlnp | grep :8080

# Test health endpoint
curl -v http://localhost/health

# Check DNS resolution
dig +short api.fermi.trade

# Get public IP
curl -s http://169.254.169.254/latest/meta-data/public-ipv4
```

## Rollback

If something goes wrong, backups are stored in `/etc/nginx/backups/`:

```bash
# List backups
ls -lh /etc/nginx/backups/

# Restore from backup
BACKUP_FILE=$(ls -t /etc/nginx/backups/nginx-configs-backup-*.tar.gz | head -1)
sudo rm -f /etc/nginx/conf.d/fermi-gateway.conf
sudo tar -xzf $BACKUP_FILE -C /etc/nginx/conf.d/
sudo nginx -t && sudo systemctl reload nginx
```

## Next Steps - SSL/HTTPS Setup

Once HTTP is working, set up SSL with Let's Encrypt:

1. Ensure DNS points to your server
2. Ensure ports 80 and 443 are open in AWS Security Group
3. Run SSL setup script (coming soon)

```bash
# (Not yet available - will be created if needed)
sudo bash scripts/setup-ssl.sh
```

This will:
- Install certbot
- Obtain SSL certificate from Let's Encrypt
- Deploy full HTTPS configuration (`deployments/nginx.conf`)
- Set up auto-renewal (90-day certificates)

## Monitoring

### Key Metrics to Watch
1. **Request rate** - Monitor for unusual spikes
2. **429 errors** - Rate limit hits
3. **5xx errors** - Backend issues
4. **Response times** - Performance degradation

### Access Metrics
```bash
# View real-time requests
sudo tail -f /var/log/nginx/fermi-gateway-access.log

# Count requests by status code
sudo awk '{print $9}' /var/log/nginx/fermi-gateway-access.log | sort | uniq -c

# Find IPs hitting rate limits
sudo grep "429" /var/log/nginx/fermi-gateway-access.log | awk '{print $1}' | sort | uniq -c | sort -rn
```

## Troubleshooting

### Issue: 502 Bad Gateway
```bash
# Check if gateway is running
sudo systemctl status fermi-gateway

# Check gateway logs
sudo journalctl -u fermi-gateway -n 50

# Restart gateway
sudo systemctl restart fermi-gateway
```

### Issue: Connection Refused
```bash
# Check if nginx is running
sudo systemctl status nginx

# Check if ports are listening
sudo ss -tlnp | grep -E ':(80|8080)'

# Check firewall
sudo firewall-cmd --list-all
```

### Issue: Domain Not Accessible
```bash
# Verify DNS
dig +short api.fermi.trade

# Check AWS Security Group allows port 80
# (Must be done in AWS Console)

# Test from server
curl http://localhost/health

# If localhost works but domain doesn't = DNS or firewall issue
```

### Issue: Rate Limiting Too Strict
Edit `/etc/nginx/conf.d/fermi-gateway.conf`:
```nginx
# Increase rate limits at the top of the file:
limit_req_zone $binary_remote_addr zone=general:10m rate=200r/s;  # Was 100r/s
limit_req_zone $binary_remote_addr zone=api:10m rate=100r/s;      # Was 50r/s
```

Then reload:
```bash
sudo nginx -t && sudo systemctl reload nginx
```

## AWS Security Group Configuration

Your EC2 Security Group must allow:

| Type | Protocol | Port Range | Source | Description |
|------|----------|------------|--------|-------------|
| HTTP | TCP | 80 | 0.0.0.0/0 | Public HTTP access |
| HTTPS | TCP | 443 | 0.0.0.0/0 | Public HTTPS access (for SSL) |
| Custom TCP | TCP | 8080 | 0.0.0.0/0 | Gateway direct access (optional) |
| SSH | TCP | 22 | Your IP | SSH access |

## Support

For issues:
1. Check logs: `sudo journalctl -u nginx -n 50`
2. Check config: `sudo nginx -t`
3. Review this guide
4. Check AWS Security Group settings
5. Verify DNS with `dig api.fermi.trade`
