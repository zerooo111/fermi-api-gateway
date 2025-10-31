# Cloudflare Setup Guide

This guide explains how to use Cloudflare as your SSL/TLS proxy in front of the Fermi API Gateway.

## Architecture

```
Client (HTTPS) → Cloudflare (SSL/TLS Termination) → Your Server (HTTP:80) → Nginx → Gateway (HTTP:8080)
```

**Benefits:**
- ✅ Free SSL/TLS certificates (managed by Cloudflare)
- ✅ DDoS protection
- ✅ CDN caching
- ✅ Web Application Firewall (WAF)
- ✅ Bot protection
- ✅ Analytics
- ✅ No certificate renewal needed

## Cloudflare Dashboard Setup

### 1. SSL/TLS Settings

Go to **SSL/TLS** in Cloudflare dashboard:

**Set encryption mode to: Flexible**
- This allows Cloudflare to use HTTPS with clients
- While connecting to your origin server via HTTP

```
SSL/TLS → Overview → Encryption mode: Flexible
```

**Why Flexible?**
- Your server doesn't need SSL certificates
- Cloudflare handles all SSL/TLS
- Simpler setup and maintenance

### 2. DNS Configuration

Go to **DNS** → **Records**:

**Ensure proxy is ENABLED (orange cloud):**
```
Type: A
Name: api
Content: YOUR_SERVER_IP
Proxy status: Proxied (🟠 orange cloud)
TTL: Auto
```

**Important:** The orange cloud must be ON for SSL to work!

### 3. Firewall Rules (Optional but Recommended)

Go to **Security** → **WAF**:

Create a rule to block non-Cloudflare traffic:
```
Field: IP Source Address
Operator: does not match
Value: Cloudflare IPs
Action: Block
```

This ensures only Cloudflare can reach your server.

### 4. Page Rules (Optional)

Go to **Rules** → **Page Rules**:

**Cache API responses** (if needed):
```
URL: api.fermi.trade/api/*
Cache Level: Standard
Edge Cache TTL: 2 hours
```

**Note:** Be careful with caching API responses!

## Nginx Configuration

The nginx config has been updated to work with Cloudflare:

### Real IP Restoration

Nginx now trusts Cloudflare IPs and extracts the real client IP from `CF-Connecting-IP` header:

```nginx
set_real_ip_from 173.245.48.0/20;
set_real_ip_from 103.21.244.0/22;
# ... (all Cloudflare IP ranges)
real_ip_header CF-Connecting-IP;
real_ip_recursive on;
```

**Why this matters:**
- Rate limiting works per actual client (not Cloudflare proxy IP)
- Logs show real client IPs
- Security rules work correctly

### Cloudflare Headers Forwarded

Nginx passes these Cloudflare headers to your backend:

```nginx
proxy_set_header CF-Connecting-IP $http_cf_connecting_ip;
proxy_set_header CF-Ray $http_cf_ray;
proxy_set_header X-Forwarded-Proto $http_cf_visitor;
```

Your Go application can use these headers to:
- Get real client IP: `r.Header.Get("CF-Connecting-IP")`
- Get Cloudflare Ray ID for debugging: `r.Header.Get("CF-Ray")`
- Detect HTTPS requests: `r.Header.Get("X-Forwarded-Proto")`

## Deploy Updated Configuration

Upload and deploy the Cloudflare-ready config:

```bash
# From local machine
scp deployments/nginx.conf ec2-user@YOUR_SERVER_IP:/opt/fermi-api-gateway/deployments/

# On server
ssh ec2-user@YOUR_SERVER_IP
cd /opt/fermi-api-gateway

# Deploy the updated config
sudo bash scripts/deploy-nginx.sh
```

## Verification

### 1. Test HTTP (from server)
```bash
curl http://localhost/health
# Should work
```

### 2. Test via Cloudflare (from anywhere)
```bash
curl https://api.fermi.trade/health
# Should work with HTTPS!
```

### 3. Check Headers
```bash
curl -I https://api.fermi.trade/health

# Should see:
# - CF-Ray: ... (Cloudflare request ID)
# - CF-Cache-Status: ...
# - Server: cloudflare
```

### 4. Verify Real IP Logging
```bash
# On server, check nginx logs
sudo tail -f /var/log/nginx/fermi-gateway-access.log

# Should show real client IPs, not Cloudflare IPs
```

## Cloudflare Security Features

### Enable Bot Fight Mode
```
Security → Bots → Configure → Enable Bot Fight Mode
```

### Enable HTTPS Only
```
SSL/TLS → Edge Certificates → Always Use HTTPS: ON
```

### Enable HSTS
```
SSL/TLS → Edge Certificates → HTTP Strict Transport Security (HSTS)
- Enable HSTS
- Max Age: 1 year (31536000)
- Include subdomains: ON
- Preload: ON (optional)
```

### Enable Automatic HTTPS Rewrites
```
SSL/TLS → Edge Certificates → Automatic HTTPS Rewrites: ON
```

## Rate Limiting Strategy

**Two layers of rate limiting:**

1. **Cloudflare Rate Limiting** (first layer)
   - Go to Security → WAF → Rate limiting rules
   - Set global limits (e.g., 1000 req/min per IP)

2. **Nginx Rate Limiting** (second layer)
   - Already configured: 100/50/20 req/sec per endpoint
   - Works with real client IPs (thanks to CF-Connecting-IP)

**Recommended Cloudflare limits:**
```
Rate limiting rule:
- URL: api.fermi.trade/api/*
- Method: ALL
- Requests: 1000 requests per 60 seconds
- Action: Block
- Duration: 600 seconds (10 min ban)
```

## Monitoring

### Cloudflare Analytics

Dashboard → Analytics → Traffic

Monitor:
- Total requests
- Bandwidth
- Threats blocked
- Cache hit ratio
- Status codes

### Cloudflare Logs (Enterprise only)

If you have Cloudflare Enterprise, enable Logpush to send logs to:
- S3
- Google Cloud Storage
- Azure Blob Storage
- Elasticsearch

### Your Server Logs

Nginx logs now show real client IPs:
```bash
sudo tail -f /var/log/nginx/fermi-gateway-access.log
```

## Troubleshooting

### Issue: 521 Error (Web server is down)
```
Cloudflare can't connect to your origin server
```

**Solution:**
1. Check if nginx is running: `sudo systemctl status nginx`
2. Check if port 80 is open in AWS Security Group
3. Check firewall: `sudo firewall-cmd --list-all`

### Issue: 522 Error (Connection timed out)
```
Cloudflare connected but origin didn't respond in time
```

**Solution:**
1. Check if gateway is running: `sudo systemctl status fermi-gateway`
2. Increase nginx timeouts if needed
3. Check backend logs: `sudo journalctl -u fermi-gateway -n 50`

### Issue: 525 Error (SSL handshake failed)
```
Should not happen with "Flexible" mode
```

**Solution:**
- Verify SSL mode is set to **Flexible** in Cloudflare
- Not "Full" or "Full (strict)"

### Issue: Rate limiting not working correctly

**Check if real IP restoration is working:**
```bash
# On server
curl -H "CF-Connecting-IP: 1.2.3.4" http://localhost/health

# Check logs - should show 1.2.3.4, not 127.0.0.1
sudo tail -n 1 /var/log/nginx/fermi-gateway-access.log
```

### Issue: Infinite redirect loop

**Solution:**
- Make sure Cloudflare SSL mode is **Flexible** (not Full)
- Remove any HTTPS redirect rules from nginx (already done)

## Security Best Practices

### 1. Firewall Rules (AWS Security Group)

**Only allow Cloudflare IPs:**
```
Inbound Rules:
- Type: HTTP, Port: 80, Source: Cloudflare IP ranges
- Type: SSH, Port: 22, Source: Your IP
```

Get Cloudflare IP ranges:
- https://www.cloudflare.com/ips-v4
- https://www.cloudflare.com/ips-v6

**Important:** This prevents direct access to your server, forcing all traffic through Cloudflare.

### 2. Block Non-Cloudflare Traffic

Add to nginx config (if not using Security Group restriction):
```nginx
# At the top of server block
if ($http_cf_connecting_ip = "") {
    return 403;
}
```

This blocks any requests not coming from Cloudflare.

### 3. Validate CF-Connecting-IP

Cloudflare guarantees `CF-Connecting-IP` is the real client IP only when:
- Request comes from Cloudflare IP ranges (which we trust in nginx)
- No one can spoof this header from outside Cloudflare

### 4. Use Cloudflare WAF

Enable managed rulesets:
```
Security → WAF → Managed rules
- Cloudflare Managed Ruleset: ON
- Cloudflare OWASP Core Ruleset: ON
```

### 5. Enable Under Attack Mode (if needed)

If under DDoS attack:
```
Quick Actions → Under Attack Mode: ON
```

This shows an interstitial page to verify visitors are human.

## Performance Optimization

### Enable Argo Smart Routing (Paid)
```
Speed → Optimization → Argo Smart Routing
```

Routes traffic through Cloudflare's fastest paths.

### Enable HTTP/3 (QUIC)
```
Network → HTTP/3 (with QUIC): ON
```

Faster connection establishment.

### Enable 0-RTT Connection Resumption
```
Network → 0-RTT Connection Resumption: ON
```

Reduces latency for repeat visitors.

### Enable Brotli Compression
```
Speed → Optimization → Brotli: ON
```

Better compression than gzip.

## Cost Considerations

**Free Plan includes:**
- Unlimited DDoS protection
- Global CDN
- Free SSL certificates
- Basic WAF
- 100k requests/month

**Pro Plan ($20/month) adds:**
- More WAF rules
- Advanced DDoS protection
- Image optimization
- Mobile optimization

**For high traffic (50k-200k req/sec):**
- Consider Business or Enterprise plan
- Or use Pro plan + AWS scaling

## Migration Checklist

- [ ] Domain DNS pointing to your server
- [ ] Cloudflare proxy enabled (orange cloud)
- [ ] SSL mode set to **Flexible**
- [ ] Deploy updated nginx config with Cloudflare IPs
- [ ] Test HTTPS endpoint: `https://api.fermi.trade/health`
- [ ] Verify real client IPs in logs
- [ ] Enable HTTPS-only mode
- [ ] Enable HSTS
- [ ] Set up rate limiting rules
- [ ] Enable Bot Fight Mode
- [ ] Enable WAF managed rules
- [ ] Configure AWS Security Group (Cloudflare IPs only)
- [ ] Monitor Cloudflare Analytics

## Summary

With Cloudflare proxy:
- ✅ Free SSL/TLS (no certificates to manage)
- ✅ DDoS protection
- ✅ Global CDN
- ✅ WAF and bot protection
- ✅ Real client IPs preserved for rate limiting
- ✅ Cloudflare headers forwarded to backend
- ✅ Simple setup (just deploy nginx config)

Your API is production-ready with enterprise-grade security! 🚀
