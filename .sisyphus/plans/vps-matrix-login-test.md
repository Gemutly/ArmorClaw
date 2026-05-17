# VPS Testing Plan: Matrix Login Fix Verification

## TL;DR

> **Goal**: Verify the fixed Matrix login (v3 identifier format) works on VPS 5.183.11.149
>
> **Deliverables**:
> - Pull new Docker image `mikegemut/armorclaw:latest`
> - Configure z.ai provider with provided API key
> - Verify Matrix login succeeds with Conduit
> - Run comprehensive health checks
>
> **Estimated Time**: 15-20 minutes
> **VPS**: 5.183.11.149
> **SSH**: `ssh -L 4096:127.0.0.1:4096 -o IdentityAgent=none -i /mnt/c/Users/Micha/.ssh/openclaw_win root@5.183.11.149`

---

## Prerequisites

- Docker Hub image: `mikegemut/armorclaw:latest` (contains v3 login fix)
- VPS access: SSH key at `/mnt/c/Users/Micha/.ssh/openclaw_win`
- z.ai API key: `cff2e899ebec4c6ab917e13946ff1f05.yZrEJ8uZFh15GeCJ`
- Conduit running on port 6167

---

## Execution Plan

### Phase 1: Connect and Pull Image (5 min)

**Task 1.1: SSH to VPS**
```bash
ssh -L 4096:127.0.0.1:4096 -o IdentityAgent=none -i /mnt/c/Users/Micha/.ssh/openclaw_win root@5.183.11.149
```

**Task 1.2: Pull latest image**
```bash
docker pull mikegemut/armorclaw:latest
```

**Task 1.3: Verify image pulled**
```bash
docker images mikegemut/armorclaw:latest
```
Expected: Image listed with latest tag and recent timestamp

---

### Phase 2: Stop Existing Container (2 min)

**Task 2.1: Check current container**
```bash
docker ps -a | grep armorclaw
```

**Task 2.2: Stop and remove old container**
```bash
docker stop armorclaw 2>/dev/null || true
docker rm armorclaw 2>/dev/null || true
```

**Task 2.3: Clean old socket**
```bash
rm -f /run/armorclaw/bridge.sock 2>/dev/null || true
```

---

### Phase 3: Start New Container with z.ai (5 min)

**Task 3.1: Run bridge container**
```bash
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

**Task 3.2: Wait for startup**
```bash
sleep 5
docker logs armorclaw --tail 30
```

Expected: Bridge starts, shows "Matrix: http://127.0.0.1:6167 (enabled)" and "z.ai provider configured"

---

### Phase 4: Configure z.ai Provider (3 min)

**Task 4.1: Check current config**
```bash
docker exec armorclaw cat /etc/armorclaw/config.toml
```

**Task 4.2: Update config if needed**

If z.ai is not configured, add to config:
```bash
docker exec armorclaw sh -c 'cat >> /etc/armorclaw/config.toml << EOF

[providers.default]
provider = "zai"
api_key = "cff2e899ebec4c6ab917e13946ff1f05.yZrEJ8uZFh15GeCJ"
EOF'
```

**Task 4.3: Restart container**
```bash
docker restart armorclaw
sleep 5
```

---

### Phase 5: Verify Matrix Login (5 min)

**Task 5.1: Check bridge logs for Matrix login**
```bash
docker logs armorclaw 2>&1 | grep -E "(Matrix|login|z\.ai)"
```

Expected output:
```
Matrix: http://127.0.0.1:6167 (enabled)
Configuring z.ai provider...
z.ai provider configured correctly
```

**Task 5.2: Check for login errors**
```bash
docker logs armorclaw 2>&1 | grep -i "login failed\|error\|failed"
```

Expected: No "login failed" errors (or specific explanation if Conduit user doesn't exist)

**Task 5.3: Test Matrix status via RPC**
```bash
echo '{"jsonrpc":"2.0","id":1,"method":"matrix.status"}' | \
  socat - UNIX-CONNECT:/run/armorclaw/bridge.sock 2>/dev/null || \
  echo '{"jsonrpc":"2.0","id":1,"method":"matrix.status"}' | nc -U /run/armorclaw/bridge.sock
```

Expected response:
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

---

### Phase 6: Verify Conduit Integration (3 min)

**Task 6.1: Test Conduit API**
```bash
curl http://localhost:6167/_matrix/client/versions
```

Expected: JSON with Matrix version array

**Task 6.2: Check socket exists**
```bash
ls -la /run/armorclaw/bridge.sock
```

Expected: Socket file exists with proper permissions

**Task 6.3: Test bridge status RPC**
```bash
echo '{"jsonrpc":"2.0","id":1,"method":"bridge.status"}' | nc -U /run/armorclaw/bridge.sock
```

Expected: JSON response with bridge status

---

### Phase 7: z.ai Provider Verification (2 min)

**Task 7.1: Verify z.ai provider configured**
```bash
docker logs armorclaw 2>&1 | grep -i "z.ai"
```

Expected: "z.ai provider configured" or similar

**Task 7.2: Check provider registry**
```bash
docker exec armorclaw cat /etc/armorclaw/providers.json 2>/dev/null | grep -i zhipu || \
  echo "providers.json not found (using env var)"
```

---

### Phase 8: Final Health Check (3 min)

**Task 8.1: Container health**
```bash
docker ps --filter name=armorclaw --format "{{.Status}}"
```

Expected: "Up X minutes" (not "Restarting" or "Exited")

**Task 8.2: Resource usage**
```bash
docker stats armorclaw --no-stream --no-trunc
```

Expected: Low CPU, reasonable memory usage

**Task 8.3: Test AI chat (optional)**
```bash
echo '{"jsonrpc":"2.0","id":1,"method":"ai.chat","params":{"message":"Hello"}}' | nc -U /run/armorclaw/bridge.sock
```

Expected: Response from z.ai API (or error if API key invalid)

---

## Success Criteria

| Criterion | Expected | Verify Command |
|-----------|----------|----------------|
| Docker image pulled | `mikegemut/armorclaw:latest` exists | `docker images \| grep armorclaw` |
| Container running | Status "Up" | `docker ps --filter name=armorclaw` |
| Matrix enabled | Logs show "Matrix: ... (enabled)" | `docker logs armorclaw 2>&1 \| grep Matrix` |
| z.ai configured | Logs show "z.ai provider" | `docker logs armorclaw 2>&1 \| grep z.ai` |
| No login errors | No "M_INVALID_USERNAME" | `docker logs armorclaw 2>&1 \| grep -i error` |
| RPC responding | Socket accepts JSON-RPC | `echo '{...}' \| nc -U /run/armorclaw/bridge.sock` |
| Conduit accessible | `/_matrix/client/versions` returns JSON | `curl http://localhost:6167/_matrix/client/versions` |

---

## Troubleshooting

### Issue: Matrix Login Still Fails

**Symptom**: "Matrix login failed: M_INVALID_USERNAME"

**Cause**: Conduit user doesn't exist

**Fix**:
```bash
# Create admin user on Conduit
curl -X POST http://localhost:6167/_matrix/client/v3/register \
  -H "Content-Type: application/json" \
  -d '{"username":"admin","password":"adminpass","auth":{"type":"m.login.dummy"}}'

# Restart bridge
docker restart armorclaw
```

### Issue: z.ai API Key Not Working

**Symptom**: "API key invalid" or 401 errors

**Cause**: Wrong API key format

**Fix**: Verify API key format - should be UUID-like

### Issue: Socket Not Created

**Symptom**: `/run/armorclaw/bridge.sock` missing

**Cause**: Bridge failed to start

**Fix**:
```bash
docker logs armorclaw 2>&1 | tail -50
# Look for startup errors
```

---

## Key Files

- `/etc/armorclaw/config.toml` - Main configuration
- `/etc/armorclaw/providers.json` - Provider registry
- `/run/armorclaw/bridge.sock` - Unix socket for RPC
- `/var/lib/armorclaw/keystore.db` - Encrypted keystore

---

## Notes

1. **SSH Key Path**: User specified `/mnt/c/Users/Micha/.ssh/openclaw_win` (Windows WSL path)
2. **Port Forwarding**: `-L 4096:127.0.0.1:4096` for local access to VPS services
3. **API Key Security**: Key is passed via environment variable, never written to disk
4. **Conduit User**: Admin user may need to be created if it doesn't exist
5. **v3 Login Format**: The fix changes payload from `{"user":"..."}` to `{"identifier":{"type":"m.id.user","user":"..."}}`

---

## Rollback Plan

If testing fails:
```bash
# Stop new container
docker stop armorclaw
docker rm armorclaw

# Restore previous image (if backed up)
docker tag mikegemut/armorclaw:previous mikegemut/armorclaw:latest

# Or pull known working version
docker pull mikegemut/armorclaw:v4.5.0
```
