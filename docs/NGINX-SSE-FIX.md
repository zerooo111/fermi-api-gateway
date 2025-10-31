# Nginx SSE Configuration Fix for stream-ticks Endpoint

## Problem

The `/api/v1/continuum/stream-ticks` endpoint uses Server-Sent Events (SSE) for real-time streaming. The default nginx configuration has settings that break SSE:

1. `proxy_buffering on` - Buffers responses, preventing real-time streaming
2. `proxy_read_timeout 30s` - Too short for long-lived SSE connections
3. Missing SSE-specific headers - Need `X-Accel-Buffering: no` and cache control headers

## Solution

Add a specific location block for the stream-ticks endpoint with SSE-optimized settings.

---

## Quick Fix (For EC2)

### Step 1: Connect to Your EC2 Instance

```bash
ssh -i your-key.pem ec2-user@your-ec2-ip
```

### Step 2: Run the SSE Configuration Checker

```bash
cd /opt/fermi-api-gateway
sudo bash scripts/check-nginx-sse.sh
```

This script will:
- Check your current nginx configuration for SSE issues
- Create a backup of your current config
- Guide you through the update process

### Step 3: Manual Update (If Needed)

If the automated script doesn't work, follow these manual steps:

#### 3.1 Backup Current Config

```bash
sudo cp /etc/nginx/sites-available/fermi-gateway \
       /etc/nginx/sites-available/fermi-gateway.backup.$(date +%Y%m%d_%H%M%S)
```

#### 3.2 Edit Nginx Config

```bash
sudo nano /etc/nginx/sites-available/fermi-gateway
```

#### 3.3 Add SSE Location Block

Find the line that says `# API endpoints (with rate limiting)` (around line 106-107).

**BEFORE** that line, add this new location block:

```nginx
    # SSE streaming endpoint (stream-ticks) - special handling
    # MUST come BEFORE the general /api/ location to take precedence
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
```

**Important**: This block MUST come BEFORE the general `/api/` location block. Nginx processes location blocks in order of specificity, so more specific paths should be defined first.

#### 3.4 Save and Exit

Press `Ctrl+X`, then `Y`, then `Enter` to save and exit nano.

#### 3.5 Test Nginx Configuration

```bash
sudo nginx -t
```

You should see:
```
nginx: the configuration file /etc/nginx/nginx.conf syntax is ok
nginx: configuration file /etc/nginx/nginx.conf test is successful
```

#### 3.6 Reload Nginx

If the test passes:

```bash
sudo systemctl reload nginx
```

If the test fails:
```bash
# Restore backup
sudo cp /etc/nginx/sites-available/fermi-gateway.backup.* \
       /etc/nginx/sites-available/fermi-gateway
sudo systemctl reload nginx
```

---

## Verification

### Test the SSE Endpoint

From your local machine:

```bash
# Test with curl (-N disables buffering on client side)
curl -N https://yourdomain.com/api/v1/continuum/stream-ticks
```

You should see:
- Immediate connection (no 30-second delay)
- Real-time streaming data in SSE format:
  ```
  data: {"tick_number":3776518724,"vdf_proof":{...}}

  data: {"tick_number":3776518725,"vdf_proof":{...}}
  ```

From the EC2 server:

```bash
# Check nginx access logs
sudo tail -f /var/log/nginx/fermi-gateway-access.log

# Check nginx error logs for any issues
sudo tail -f /var/log/nginx/fermi-gateway-error.log

# Check gateway logs
sudo journalctl -u fermi-gateway -f
```

### Verify Browser Behavior

Open browser dev tools (F12) → Network tab → Navigate to your frontend

Check the `stream-ticks` request:
- Status: 200 OK
- Type: `text/event-stream`
- Transfer: `chunked`
- No `ERR_ABORTED` or 500 errors

---

## Technical Explanation

### Why These Settings Matter

1. **`proxy_buffering off`**
   - Nginx by default buffers responses for efficiency
   - SSE requires immediate forwarding of each event
   - Buffering causes events to be delayed until buffer is full

2. **`proxy_read_timeout 24h`**
   - SSE connections stay open continuously
   - Default 30s timeout would close the connection prematurely
   - 24h allows for long-lived streams

3. **`X-Accel-Buffering: no`**
   - Nginx-specific header
   - Ensures nginx doesn't buffer even if other modules try to enable it
   - Extra insurance against buffering

4. **`Cache-Control: no-cache`**
   - SSE responses should never be cached
   - Each connection needs fresh, real-time data

5. **`chunked_transfer_encoding on`**
   - SSE uses HTTP/1.1 chunked encoding
   - Allows server to send data in chunks without knowing total size
   - Required for streaming

6. **Location Block Order**
   - More specific paths (`/api/v1/continuum/stream-ticks`) must come BEFORE less specific ones (`/api/`)
   - Nginx matches locations in the order: exact match → longest prefix match → regex
   - Our specific block will be matched before the general `/api/` block

---

## Rollback Procedure

If something goes wrong:

```bash
# List backups
ls -la /etc/nginx/sites-available/fermi-gateway.backup.*

# Restore specific backup (replace timestamp)
sudo cp /etc/nginx/sites-available/fermi-gateway.backup.20251031_120000 \
       /etc/nginx/sites-available/fermi-gateway

# Test and reload
sudo nginx -t && sudo systemctl reload nginx
```

---

## Additional Notes

### If You're Using a Different Reverse Proxy

**Apache**: Use `ProxyPass` with these directives:
```apache
ProxyPass /api/v1/continuum/stream-ticks http://localhost:8080/api/v1/continuum/stream-ticks disablereuse=on flushpackets=on
ProxyPassReverse /api/v1/continuum/stream-ticks http://localhost:8080/api/v1/continuum/stream-ticks
```

**Traefik**: Add labels:
```yaml
labels:
  - "traefik.http.middlewares.sse-nobuffer.buffering.maxRequestBodyBytes=0"
  - "traefik.http.middlewares.sse-nobuffer.buffering.memRequestBodyBytes=0"
  - "traefik.http.middlewares.sse-nobuffer.buffering.maxResponseBodyBytes=0"
  - "traefik.http.middlewares.sse-nobuffer.buffering.memResponseBodyBytes=0"
```

**Caddy**: It handles SSE correctly by default, but you can be explicit:
```
reverse_proxy /api/v1/continuum/stream-ticks localhost:8080 {
    flush_interval -1
}
```

### Monitoring SSE Connections

Check active SSE connections:

```bash
# Count active connections to gateway
netstat -an | grep :8080 | grep ESTABLISHED | wc -l

# Check nginx connection status
curl http://localhost/nginx_status  # if stub_status module is enabled
```

---

## Troubleshooting

### Connection Still Timing Out After 30 Seconds

**Symptom**: SSE connection closes after exactly 30 seconds

**Cause**: The general `/api/` location block is being matched instead of the SSE-specific block

**Fix**: Ensure the SSE location block comes BEFORE the `/api/` block in the config

### Events Are Delayed/Batched

**Symptom**: Events arrive in groups instead of real-time

**Cause**: Buffering is still enabled somewhere

**Check**:
```bash
# Check if our settings are actually being used
sudo nginx -T | grep -A 20 "location.*stream-ticks"
```

Look for `proxy_buffering off` in the output

### 502 Bad Gateway Errors

**Symptom**: 502 errors when accessing stream-ticks

**Cause**: Gateway service is down or port mismatch

**Fix**:
```bash
# Check gateway is running
sudo systemctl status fermi-gateway

# Check it's listening on the right port
sudo lsof -i :8080

# Check nginx upstream config matches
sudo grep "upstream fermi_gateway" /etc/nginx/sites-available/fermi-gateway
```

---

## Related Files

- Updated nginx config: [deployments/nginx.conf](../deployments/nginx.conf)
- SSE checker script: [scripts/check-nginx-sse.sh](../scripts/check-nginx-sse.sh)
- Middleware flusher implementation: [internal/middleware/logging.go](../internal/middleware/logging.go)

---

**Last Updated**: 2025-10-31
