# Nginx + SSL Setup Guide

## Current Status

✅ **Nginx installed and configured as reverse proxy**  
✅ **HTTP access working** (port 80)  
✅ **Firewall rules configured** (ports 80, 443)  
⏳ **SSL certificate** (pending domain configuration)

## Access Points

**Without SSL (current):**
- HTTP: `http://34.56.180.89/health`
- Direct: `http://34.56.180.89:9080/health` (still works)

**With SSL (after domain setup):**
- HTTPS: `https://yourdomain.com/health`

## Adding SSL Certificate

### Step 1: Update DNS

Point your domain to the instance IP:
```
Domain: your-domain.com
Type: A Record
Value: 34.56.180.89
TTL: 300 (or default)
```

**Available domains from your config:**
- `testnet.fermi.trade`
- `staging.fermi.trade`
- `staging.fermilabs.xyz`

### Step 2: Run SSL Setup Script

```bash
# Wait for DNS to propagate (5-10 minutes)
# Verify DNS: nslookup your-domain.com

# Run setup script
./setup-nginx-ssl.sh your-domain.com admin@fermilabs.xyz
```

### Step 3: Manual SSL Setup (Alternative)

If the script doesn't work, set up manually:

```bash
# SSH into instance
gcloud compute ssh fermi-gateway --project=fermi-testnet --zone=us-central1-a

# Obtain certificate
sudo certbot --nginx -d your-domain.com --email admin@fermilabs.xyz

# Certbot will automatically:
# - Obtain certificate
# - Configure Nginx for HTTPS
# - Set up HTTP -> HTTPS redirect
# - Enable auto-renewal
```

## Nginx Configuration

**Location:** `/etc/nginx/sites-available/fermi-gateway`

**Features:**
- Reverse proxy to gateway on localhost:9080
- Proper headers forwarding (X-Real-IP, X-Forwarded-For)
- 60s timeouts for long-running requests
- Access and error logging

## Useful Commands

```bash
# View Nginx logs
gcloud compute ssh fermi-gateway --project=fermi-testnet --zone=us-central1-a \
  --command='sudo tail -f /var/log/nginx/access.log'

# Check Nginx status
gcloud compute ssh fermi-gateway --project=fermi-testnet --zone=us-central1-a \
  --command='sudo systemctl status nginx'

# Reload Nginx (after config changes)
gcloud compute ssh fermi-gateway --project=fermi-testnet --zone=us-central1-a \
  --command='sudo systemctl reload nginx'

# Test SSL renewal
gcloud compute ssh fermi-gateway --project=fermi-testnet --zone=us-central1-a \
  --command='sudo certbot renew --dry-run'

# View SSL certificate info
gcloud compute ssh fermi-gateway --project=fermi-testnet --zone=us-central1-a \
  --command='sudo certbot certificates'
```

## Security Features (After SSL)

- **TLS 1.2 & 1.3** only
- **HSTS** (HTTP Strict Transport Security)
- **Security headers** (X-Frame-Options, X-Content-Type-Options, etc.)
- **Auto HTTP -> HTTPS redirect**
- **Auto certificate renewal** (certbot timer runs twice daily)

## Architecture

```
Internet (Port 443/HTTPS)
    ↓
GCP Firewall Rules
    ↓
Nginx (Port 80/443)
    ↓  Reverse Proxy
Gateway Service (Port 9080)
    ↓
TimescaleDB + Backend Services
```

## Troubleshooting

**Issue:** SSL certificate fails to obtain  
**Solution:** Ensure DNS is propagated and port 80 is accessible

**Issue:** 502 Bad Gateway  
**Solution:** Check if gateway service is running:
```bash
gcloud compute ssh fermi-gateway --project=fermi-testnet --zone=us-central1-a \
  --command='sudo systemctl status fermi-gateway'
```

**Issue:** Connection timeout  
**Solution:** Check firewall rules allow ports 80 and 443
