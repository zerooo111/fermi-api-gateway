# Cloudflare + Nginx Deployment Troubleshooting

## Current Status
- ✅ Nginx deployed and running on server
- ✅ Local proxy working (`localhost/health` responds)
- ❌ Cloudflare showing HTTP 522 error (connection timeout)
- ℹ️  Domain: api.fermi.trade (proxied through Cloudflare)

## The 522 Error Explained

HTTP 522 means **Cloudflare cannot connect to your origin server**. This is a Cloudflare-specific error indicating that the connection to your server is timing out.

## Fix Steps

### Step 1: Verify Gateway is Running on Server

SSH into your server and run:
```bash
# Check if gateway is running
sudo systemctl status fermi-gateway

# If not running, start it
sudo systemctl start fermi-gateway

# Test locally on server
curl http://localhost:8080/health
curl http://localhost/health
```

**Expected:** Both should return `{"status":"healthy",...}`

---

### Step 2: Check AWS Security Group Rules

Your EC2 instance **must allow** incoming traffic from Cloudflare's IP ranges.

#### Option A: Allow All HTTP Traffic (Simple but less secure)
In AWS Console → EC2 → Security Groups:
```
Type: HTTP
Protocol: TCP
Port: 80
Source: 0.0.0.0/0
```

#### Option B: Allow Only Cloudflare IPs (Recommended)
Get Cloudflare's IP ranges and add them:
```bash
# On your local machine, get Cloudflare IPs
curl https://www.cloudflare.com/ips-v4
curl https://www.cloudflare.com/ips-v6
```

Add each range to your security group inbound rules (Port 80):
```
Example ranges (check link for current list):
173.245.48.0/20
103.21.244.0/22
103.22.200.0/22
... etc
```

**Current Cloudflare IPs:** https://www.cloudflare.com/ips/

---

### Step 3: Configure Cloudflare SSL/TLS Mode

In Cloudflare Dashboard → SSL/TLS → Overview:

**Choose the right mode for your setup:**

| Mode | When to Use | Your Setup |
|------|-------------|------------|
| **Flexible** | Server only has HTTP (no SSL certificate) | ✅ **Use this now** |
| Off | No encryption at all | ❌ Don't use |
| Full | Server has self-signed SSL | ❌ Not yet |
| Full (Strict) | Server has valid SSL from Let's Encrypt | ⏳ Use after SSL setup |

**For now:** Set to **Flexible** since you only have HTTP configured.

**After setting up SSL:** Switch to **Full (Strict)** and use the full nginx.conf.

---

### Step 4: Check Nginx is Listening on Port 80

On your server:
```bash
# Check if nginx is listening
sudo lsof -i :80
sudo ss -tlnp | grep :80

# Check nginx status
sudo systemctl status nginx

# Check nginx logs for errors
sudo tail -f /var/log/nginx/fermi-gateway-error.log
```

---

### Step 5: Test Direct Connection (Bypass Cloudflare)

Find your server's real IP address:
```bash
# On server
curl -4 ifconfig.co
# or
curl https://api.ipify.org
```

Then from your local machine:
```bash
# Replace YOUR_SERVER_IP with actual IP
curl -H "Host: api.fermi.trade" http://YOUR_SERVER_IP/health
```

If this works, the issue is definitely Cloudflare → Server connectivity.

---

### Step 6: Temporarily Disable Cloudflare Proxy (Diagnostic)

In Cloudflare Dashboard → DNS:
1. Find the `api.fermi.trade` A record
2. Click the **orange cloud** icon to turn it **gray** (DNS only)
3. Wait 1-2 minutes for DNS propagation
4. Test: `curl http://api.fermi.trade/health`

**If this works:** The issue is Cloudflare configuration (likely SSL/TLS mode or firewall)
**If this fails:** The issue is on your server side (check steps 1 & 4)

**Important:** Turn the proxy back on (orange cloud) after testing.

---

## Common Fixes Summary

| Symptom | Fix |
|---------|-----|
| 522 Error | Check security group allows Cloudflare IPs or 0.0.0.0/0:80 |
| Works with gray cloud, fails with orange | Set SSL/TLS mode to **Flexible** |
| Gateway not running | `sudo systemctl start fermi-gateway` |
| Nginx not listening | `sudo systemctl restart nginx` |
| Direct IP works, domain doesn't | DNS propagation (wait 5 minutes) |

---

## Complete Testing Checklist

Run these commands **on your server**:

```bash
# 1. Check gateway is running
curl http://localhost:8080/health

# 2. Check nginx proxy works locally
curl http://localhost/health

# 3. Check nginx is listening
sudo ss -tlnp | grep :80

# 4. Get your server's public IP
curl -4 ifconfig.co

# 5. Check nginx error logs
sudo tail -20 /var/log/nginx/fermi-gateway-error.log
```

Run these **from your local machine**:

```bash
# 1. Test direct (replace with real IP from step 4 above)
curl -H "Host: api.fermi.trade" http://YOUR_SERVER_IP/health

# 2. Test through Cloudflare
curl http://api.fermi.trade/health

# 3. Check DNS
dig api.fermi.trade
nslookup api.fermi.trade
```

---

## After Fixing 522 Error

Once you can access `http://api.fermi.trade/health`:

### Next Steps:
1. **Set up SSL/TLS** - Run Let's Encrypt setup (coming in TODO.md Step 6)
2. **Change Cloudflare to Full (Strict)** - After SSL is working
3. **Enable HTTP → HTTPS redirect** - Force all traffic to HTTPS
4. **Deploy full nginx.conf** - Use the production config with SSL

---

## Quick Reference

### Cloudflare SSL/TLS Modes
```
Flexible:      Client ←HTTPS→ Cloudflare ←HTTP→ Server (use now)
Full:          Client ←HTTPS→ Cloudflare ←HTTPS→ Server (self-signed OK)
Full (Strict): Client ←HTTPS→ Cloudflare ←HTTPS→ Server (valid cert required)
```

### Essential Commands
```bash
# On server
sudo systemctl status fermi-gateway nginx
curl http://localhost/health
sudo tail -f /var/log/nginx/fermi-gateway-error.log

# Local machine
curl http://api.fermi.trade/health
curl -I http://api.fermi.trade/health
```

---

## Need Help?

If none of these steps work:
1. Check AWS EC2 Security Groups (most common issue)
2. Check Cloudflare SSL/TLS mode is "Flexible"
3. Verify gateway is actually running: `systemctl status fermi-gateway`
4. Check nginx logs: `sudo journalctl -u nginx -f`

**Most likely fix:** Security group not allowing inbound traffic on port 80.
