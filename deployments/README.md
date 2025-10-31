# Nginx Configuration Files

This directory contains production-grade nginx configurations for the Fermi API Gateway.

## Available Configurations

### 1. nginx-http-only.conf
**Purpose:** HTTP-only configuration for initial setup and testing

**Use when:**
- Setting up for the first time
- Testing domain and DNS configuration
- Before SSL certificates are obtained

**Features:**
- ✅ Rate limiting (3 tiers)
- ✅ Security headers
- ✅ Compression
- ✅ Metrics access control
- ✅ SSE streaming support
- ✅ Error handling
- ❌ No SSL/HTTPS (insecure for production)

**Deployment:**
```bash
sudo bash scripts/deploy-nginx.sh
```

### 2. nginx.conf
**Purpose:** Full production configuration with SSL/HTTPS

**Use when:**
- Ready for production deployment
- SSL certificates obtained from Let's Encrypt
- Need HTTPS encryption

**Features:**
- ✅ All features from HTTP-only config
- ✅ SSL/TLS 1.2 & 1.3
- ✅ HTTP to HTTPS redirect
- ✅ HSTS with preload
- ✅ OCSP stapling
- ✅ Modern cipher suites
- ✅ Content Security Policy

**Deployment:**
```bash
# First obtain SSL certificate (will be automated in setup-ssl.sh)
# Then deploy this config
sudo cp /opt/fermi-api-gateway/deployments/nginx.conf /etc/nginx/conf.d/fermi-gateway.conf
sudo nginx -t && sudo systemctl reload nginx
```

## Configuration Comparison

| Feature | HTTP-only | HTTPS (Full) |
|---------|-----------|--------------|
| Domain | api.fermi.trade | api.fermi.trade |
| SSL/HTTPS | ❌ | ✅ |
| HTTP to HTTPS redirect | N/A | ✅ |
| Rate Limiting | ✅ 3 tiers | ✅ 3 tiers |
| Security Headers | ✅ Basic | ✅ Enhanced + HSTS |
| Compression | ✅ | ✅ |
| Metrics Protection | ✅ | ✅ |
| SSE Streaming | ✅ | ✅ |
| Error Handling | ✅ JSON | ✅ JSON |
| Hidden File Block | ✅ | ✅ |
| Client Limits | ✅ 10MB/10s | ✅ 10MB/10s |
| Keepalive | ✅ Optimized | ✅ Optimized |
| OCSP Stapling | N/A | ✅ |
| Content Security Policy | ❌ | ✅ |

## Rate Limiting Configuration

Both configurations use identical rate limiting:

```nginx
# Rate limiting zones
limit_req_zone $binary_remote_addr zone=general:10m rate=100r/s;
limit_req_zone $binary_remote_addr zone=api:10m rate=50r/s;
limit_req_zone $binary_remote_addr zone=api_strict:10m rate=20r/s;

# Connection limiting
limit_conn_zone $binary_remote_addr zone=addr:10m;
```

**Limits per endpoint:**
- `/` (root): 100 req/sec, burst 50
- `/api/*`: 50 req/sec, burst 20
- `/api/v1/continuum/stream-ticks`: 20 req/sec, burst 10
- All: 10 concurrent connections per IP

## Security Features

### Both Configs Include:

1. **Rate Limiting**
   - Protects against DoS attacks
   - Per-IP based
   - Custom limits per endpoint type

2. **Connection Limits**
   - Max 10 concurrent connections per IP
   - Prevents connection exhaustion

3. **Client Protections**
   - 10MB max body size
   - 10s client timeouts
   - Prevents slowloris attacks

4. **Metrics Protection**
   - Restricted to internal IPs only
   - Blocks external access

5. **Hidden File Access**
   - Blocks `/.git`, `/.env`, etc.
   - Returns 404 to hide existence

6. **Error Handling**
   - Upstream retry logic (2 attempts)
   - Custom JSON error pages
   - Graceful degradation

7. **Security Headers**
   - X-Frame-Options: DENY
   - X-Content-Type-Options: nosniff
   - X-XSS-Protection: 1; mode=block
   - Referrer-Policy: strict-origin-when-cross-origin
   - Permissions-Policy

### HTTPS Config Additional Features:

1. **SSL/TLS Security**
   - TLS 1.2 and 1.3 only (no outdated protocols)
   - Modern cipher suites
   - OCSP stapling for cert validation

2. **HSTS (HTTP Strict Transport Security)**
   - 1 year max-age
   - includeSubDomains
   - preload ready

3. **Content Security Policy**
   - Restricts resource loading
   - Prevents XSS attacks

4. **Automatic HTTPS Redirect**
   - All HTTP traffic → HTTPS
   - Except Let's Encrypt challenges

## Upstream Configuration

```nginx
upstream fermi_gateway {
    server 127.0.0.1:8080 max_fails=3 fail_timeout=30s;
    keepalive 32;
    keepalive_requests 1000;
    keepalive_timeout 60s;
}
```

- **Backend:** API Gateway on port 8080
- **Health checks:** Fails after 3 attempts, 30s timeout
- **Keepalive:** 32 connections, 1000 requests, 60s timeout
- **Performance:** Reduces connection overhead

## SSE Streaming Configuration

Special handling for `/api/v1/continuum/stream-ticks`:

```nginx
proxy_buffering off;           # Real-time streaming
proxy_cache off;               # No caching
proxy_read_timeout 24h;        # Long-running connections
chunked_transfer_encoding on;  # Required for SSE
```

This ensures proper Server-Sent Events (SSE) support for real-time data streams.

## Error Pages

Custom JSON error responses:

- **429** - Rate limit exceeded
- **403** - Forbidden (metrics from external IP)
- **404** - Not found
- **502/503/504** - Service unavailable

Example:
```json
{
  "error": "rate_limit_exceeded",
  "message": "Too many requests. Please slow down and try again later."
}
```

## Deployment Path

### Phase 1: HTTP Setup (Testing)
```bash
1. Deploy HTTP-only config
2. Verify DNS points to server
3. Test endpoints work
4. Check rate limiting
```

### Phase 2: SSL Setup (Production)
```bash
1. Obtain SSL certificate (Let's Encrypt)
2. Deploy HTTPS config
3. Verify HTTPS works
4. Check HTTP→HTTPS redirect
5. Test all endpoints with HTTPS
```

## Monitoring & Logs

Both configurations log to:
- **Access log:** `/var/log/nginx/fermi-gateway-access.log`
- **Error log:** `/var/log/nginx/fermi-gateway-error.log`

Health checks and metrics scraping are not logged (access_log off).

## Performance Optimization

Both configs include:

1. **Gzip Compression**
   - Enabled for JSON, JS, CSS, XML
   - Min size: 1KB
   - Reduces bandwidth by ~70%

2. **Keepalive Connections**
   - Reuses connections
   - Reduces handshake overhead
   - Up to 1000 requests per connection

3. **Buffering**
   - Enabled for regular requests (4KB buffers)
   - Disabled for SSE streaming
   - Optimized buffer sizes

4. **HTTP/2**
   - Enabled on HTTPS config
   - Multiplexing support
   - Header compression

## Customization

### Adjust Rate Limits

Edit the top of the config file:
```nginx
# Increase limits
limit_req_zone $binary_remote_addr zone=general:10m rate=200r/s;  # Was 100r/s
limit_req_zone $binary_remote_addr zone=api:10m rate=100r/s;      # Was 50r/s
```

Then reload:
```bash
sudo nginx -t && sudo systemctl reload nginx
```

### Allow Metrics from External IP

Edit metrics location block:
```nginx
location /metrics {
    allow 127.0.0.1;
    allow YOUR_PROMETHEUS_IP;  # Add your IP here
    deny all;
    ...
}
```

### Increase Client Body Size

```nginx
client_max_body_size 50m;  # Was 10m
```

## Testing

### Test HTTP Config
```bash
curl http://api.fermi.trade/health
curl http://api.fermi.trade/api/v1/continuum/status
```

### Test HTTPS Config
```bash
curl https://api.fermi.trade/health
curl -I https://api.fermi.trade/  # Check headers

# Verify SSL
openssl s_client -connect api.fermi.trade:443 -servername api.fermi.trade
```

### Test Rate Limiting
```bash
# Spam requests to trigger rate limit
for i in {1..200}; do curl -s http://api.fermi.trade/health; done

# Should see 429 error after limit exceeded
```

### Test Metrics Protection
```bash
# From server (should work)
curl http://localhost/metrics

# From external (should get 403)
curl http://api.fermi.trade/metrics
```

## Troubleshooting

### Config Test Fails
```bash
sudo nginx -t
# Read error message carefully
# Common issues: syntax errors, missing semicolons
```

### Rate Limit Too Strict
Check logs for 429 errors:
```bash
sudo grep "429" /var/log/nginx/fermi-gateway-access.log | wc -l
```

### SSL Certificate Issues
```bash
# Check certificate
sudo certbot certificates

# Renew manually
sudo certbot renew --force-renewal
```

## References

- [Nginx Rate Limiting](https://www.nginx.com/blog/rate-limiting-nginx/)
- [Nginx SSL Configuration](https://ssl-config.mozilla.org/)
- [Security Headers](https://securityheaders.com/)
- [Let's Encrypt](https://letsencrypt.org/)

## Support

For detailed deployment instructions, see [DEPLOYMENT.md](../DEPLOYMENT.md)
