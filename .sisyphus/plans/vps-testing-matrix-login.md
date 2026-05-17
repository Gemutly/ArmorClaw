# VPS Testing Plan: Matrix Login Fix Verification

## TL;DR

> **Goal**: Verify the Matrix v3 login format fix works on VPS 5.183.11.149
>
> **Deliverables**:
> - Pulled latest Docker image with Matrix login fix
> - Running ArmorClaw bridge container
> - Verified Matrix login with Conduit
> - Tested z.ai provider integration
> - All RPC endpoints responding correctly
>
> **Estimated Time**: ~10 minutes
> **VPS**: 5.183.11.149

---

## Context

### What Was Fixed
- **Matrix Login Format**: Changed from deprecated r0 format to v3 identifier format
- **Files Modified**:
  - `bridge/pkg/matrix/client.go` - Login endpoint + payload format
  - `bridge/internal/adapter/matrix.go` - Login payload format
- **Docker Image**: `mikegemut/armorclaw:latest` rebuilt with fix

### VPS Details
- **IP**: 5.183.11.149
- **SSH**: `ssh -L 4096:127.0.0.1:4096 -o IdentityAgent=none -i /mnt/c/Users/Micha/.ssh/openclaw_win root@5.183.11.149`
- **Existing Services**:
  - Conduit (Matrix homeserver) on port 6167
  - Admin user: `@admin:5.183.11.149` (password: `adminpass`)

---

## Prerequisites

- [ ] Docker available on VPS
- [ ] SSH key accessible at `/mnt/c/Users/Micha/.ssh/openclaw_win`
- [ ] Port 4096 forwarded for local access
- [ ] z.ai API key ready: `cff2e899ebec4c6ab917e13946ff1f05.yZrEJ8uZFh15GeCJ`

---

## Execution Steps

### Step 1: Connect to VPS

```bash
ssh -L 4096:127.0.0.1:4096 -o IdentityAgent=none -i /mnt/c/Users/Micha/.ssh/openclaw_win root@5.183.11.149
```

### Step 2: Pull Latest Docker Image

```bash
# Pull the new image with Matrix login fix
docker pull mikegemut/armorclaw:latest

# Verify image pulled
docker images | grep armorclaw
```

### Step 3: Stop Existing Container (if running)

```bash
# Check if container exists
docker ps -a | grep armorclaw

# Stop and remove if exists
docker stop armorclaw 2>/dev/null || true
docker rm armorclaw 2>/dev/null || true
```

### Step 4: Start Bridge Container

```bash
# Run bridge with z.ai provider
docker run -d --name armorclaw \
  --restart unless-stopped \
  --network host \
  -e ZAI_API_KEY='cff2e899ebec4c6ab917e13946ff1f05.yZrEJ8uZFh15GeCJ' \
  -v /var/run/docker.sock:/var/run/docker.sock \
  -v armorclaw-config:/etc/armorclaw \
  -v armorclaw-data:/var/lib/armorclaw \
  --entrypoint '' \
  mikegemut/armorclaw:latest \
  /opt/armorclaw/armorclaw-bridge -config /etc/armorclaw/config.toml
```

### Step 5: Verify Container Started

```bash
# Check container status
docker ps --filter name=armorclaw

# Wait for startup (5 seconds)
sleep 5

# Check logs for startup
docker logs armorclaw 2>&1 | tail -30
```

### Step 6: Configure Matrix (if needed)

```bash
# Update config with Matrix settings
cat > /var/lib/docker/volumes/armorclaw-config/_data/config.toml << 'EOF'
[server]
socket_path = "/run/armorclaw/bridge.sock"
daemonize = false

[keystore]
db_path = "/var/lib/armorclaw/keystore.db"

[matrix]
enabled = true
homeserver_url = "http://127.0.0.1:6167"
username = "admin"
password = "adminpass"
device_id = "BRIDGE"

[providers.default]
provider = "zai"
api_key = "cff2e899ebec4c6ab917e13946ff1f05.yZrEJ8uZFh15GeCJ"

[budget]
daily_limit_usd = 5.0
monthly_limit_usd = 100.0
hard_stop = true

[logging]
level = "info"
format = "text"
output = "stdout"

[discovery]
enabled = true
port = 8080
tls = false
EOF

# Restart to apply config
docker restart armorclaw
sleep 5
```

---

## Verification Tests

### Test 1: Conduit (Matrix Server) Health

```bash
curl -s http://127.0.0.1:6167/_matrix/client/versions
```

**Expected**:
```json
{"versions":["r0.5.0","r0.6.0","v1.1",...]}
```

### Test 2: Bridge Socket Exists

```bash
ls -la /run/armorclaw/bridge.sock
```

**Expected**: Socket file exists with `srw-rw----` permissions

### Test 3: Matrix Status via RPC

```bash
echo '{"jsonrpc":"2.0","method":"matrix.status","id":1}' | nc -U /run/armorclaw/bridge.sock
```

**Expected**:
```json
{
  "jsonrpc": "2.0",
  "id": 1,
  "result": {
    "enabled": true,
    "connected": true,
    "logged_in": true,
    "homeserver": "http://127.0.0.1:6167"
  }
}
```

### Test 4: Bridge Status via RPC

```bash
echo '{"jsonrpc":"2.0","method":"bridge.status","id":1}' | nc -U /run/armorclaw/bridge.sock
```

**Expected**: Valid JSON response with status info

### Test 5: Check Logs for Matrix Login Success

```bash
docker logs armorclaw 2>&1 | grep -E "(Matrix|login|logged)"
```

**Expected**: 
- `Matrix: http://127.0.0.1:6167 (enabled)`
- NO "Matrix login failed" errors
- NO "M_INVALID_USERNAME" errors

### Test 6: z.ai Provider Configured

```bash
docker logs armorclaw 2>&1 | grep -i "z.ai"
```

**Expected**: `z.ai provider configured` or `z.ai provider already configured`

### Test 7: Container Health

```bash
docker ps --filter name=armorclaw --format '{{.Status}}'
```

**Expected**: `Up X minutes` or `Up X minutes (healthy)`

---

## Success Criteria

- [ ] Docker image pulled successfully
- [ ] Container running (not restarting/exited)
- [ ] Socket file exists at `/run/armorclaw/bridge.sock`
- [ ] Matrix Conduit responding on port 6167
- [ ] `matrix.status` RPC returns `"logged_in": true`
- [ ] No Matrix login errors in logs
- [ ] z.ai provider configured

---

## Troubleshooting

### If Matrix Login Still Fails

1. **Check Conduit has admin user**:
   ```bash
   # Register if needed
   curl -X POST 'http://127.0.0.1:6167/_matrix/client/v3/register' \
     -H 'Content-Type: application/json' \
     -d '{"username":"admin","password":"adminpass","auth":{"type":"m.login.dummy"}}'
   ```

2. **Test direct login to Conduit**:
   ```bash
   curl -X POST 'http://127.0.0.1:6167/_matrix/client/v3/login' \
     -H 'Content-Type: application/json' \
     -d '{"type":"m.login.password","identifier":{"type":"m.id.user","user":"admin"},"password":"adminpass"}'
   ```

3. **Check bridge logs for specific error**:
   ```bash
   docker logs armorclaw 2>&1 | grep -i "M_INVALID"
   ```

### If Container Keeps Restarting

1. **Check full logs**:
   ```bash
   docker logs armorclaw 2>&1
   ```

2. **Check config exists**:
   ```bash
   docker exec armorclaw cat /etc/armorclaw/config.toml
   ```

### If Socket Not Created

1. **Check runtime directory**:
   ```bash
   ls -la /run/armorclaw/
   ```

2. **Check permissions**:
   ```bash
   docker exec armorclaw ls -la /run/armorclaw/
   ```

---

## One-Liner Quick Test

Run all tests at once:
```bash
echo "=== Conduit ===" && \
curl -s http://127.0.0.1:6167/_matrix/client/versions && \
echo -e "\n=== Socket ===" && \
ls -la /run/armorclaw/bridge.sock && \
echo -e "\n=== Matrix Status ===" && \
echo '{"jsonrpc":"2.0","method":"matrix.status","id":1}' | nc -U /run/armorclaw/bridge.sock && \
echo -e "\n=== Container ===" && \
docker ps --filter name=armorclaw --format "table {{.Names}}\t{{.Status}}"
```

---

## Notes

- **API Key Security**: The z.ai key is visible in this plan for testing. Rotate after testing.
- **Port Forwarding**: SSH `-L 4096:127.0.0.1:4096` allows local access to VPS services
- **Config Persistence**: Docker volumes `armorclaw-config` and `armorclaw-data` persist between restarts
