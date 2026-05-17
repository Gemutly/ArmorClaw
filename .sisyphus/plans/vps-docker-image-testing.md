# Plan: VPS Docker Image Testing

## TL;DR

> **Quick Summary**: Test the newly pushed Docker image on production VPS (5.183.11.149) with z.ai provider.
> 
> **Deliverables**: 
> - Pull latest Docker image on VPS
> - Run quickstart container
> - Store z.ai API key
> - Verify AI chat functionality
> 
> **Estimated Effort**: Medium (multiple verification steps)
> **Parallel Execution**: NO - sequential testing
> **Critical Path**: Pull → Run → Store Key → Test

---

## Context

### VPS Details
- **Server**: 5.183.11.149
- **SSH**: `ssh -L 4096:127.0.0.1:4096 -o IdentityAgent=none -i ~/.ssh/openclaw_win root@5.183.11.149`
- **Docker Image**: `mikegemut/armorclaw:latest`

### API Key (NOT FOR REPO)
- **Provider**: zhipu (z.ai)
- **Key ID**: `zhipu-main`
- **Token**: `cff2e899ebec4c6ab917e13946ff1f05.yZrEJ8uZFh15GeCJ`
- **Base URL**: `https://api.z.ai/api/paas/v4`

---

## TODOs

- [ ] 1. SSH to VPS and Pull Latest Docker Image

  **What to do**:
  - SSH to VPS
  - Pull `mikegemut/armorclaw:latest`
  - Verify image digest

  **Commands**:
  ```bash
  ssh -L 4096:127.0.0.1:4096 -o IdentityAgent=none -i ~/.ssh/openclaw_win root@5.183.11.149
  docker pull mikegemut/armorclaw:latest
  docker images mikegemut/armorclaw
  ```

- [ ] 2. Stop Existing Containers and Clean Up

  **What to do**:
  - Stop any running ArmorClaw containers
  - Remove old containers to start fresh

  **Commands**:
  ```bash
  docker ps -a | grep armorclaw
  docker rm -f $(docker ps -aq --filter name=armorclaw) 2>/dev/null || true
  ```

- [ ] 3. Run Quickstart Container

  **What to do**:
  - Run the container with proper port mappings
  - Mount volumes for persistence
  - Set environment variables

  **Commands**:
  ```bash
  docker run -d \
    --name armorclaw \
    --restart unless-stopped \
    -p 8443:8443 \
    -p 6167:6167 \
    -v armorclaw-config:/etc/armorclaw \
    -v armorclaw-data:/var/lib/armorclaw \
    -v armorclaw-logs:/var/log/armorclaw \
    mikegemut/armorclaw:latest
  ```

- [ ] 4. Wait for Container to Initialize

  **What to do**:
  - Wait 30 seconds for setup to complete
  - Check container logs

  **Commands**:
  ```bash
  sleep 30
  docker logs armorclaw 2>&1 | tail -50
  ```

- [ ] 5. Store z.ai API Key

  **What to do**:
  - Use RPC to store the API key
  - Key will be encrypted in SQLCipher keystore

  **Commands**:
  ```bash
  docker exec armorclaw armorclaw-bridge add-key \
    --provider zhipu \
    --key-id zhipu-main \
    --token "cff2e899ebec4c6ab917e13946ff1f05.yZrEJ8uZFh15GeCJ" \
    --base-url "https://api.z.ai/api/paas/v4"
  ```

- [ ] 6. Verify Key Storage

  **What to do**:
  - List stored keys
  - Verify key appears in list

  **Commands**:
  ```bash
  docker exec armorclaw armorclaw-bridge list-keys
  ```

- [ ] 7. Test Bridge RPC Health

  **What to do**:
  - Call bridge.status RPC method
  - Verify bridge is responsive

  **Commands**:
  ```bash
  docker exec armorclaw armorclaw-bridge status
  # Or via RPC:
  echo '{"jsonrpc":"2.0","id":1,"method":"bridge.status"}' | docker exec -i armorclaw socat - UNIX-CONNECT:/run/armorclaw/bridge.sock
  ```

- [ ] 8. Test AI Chat with z.ai

  **What to do**:
  - Send a test message to AI chat
  - Verify response comes back

  **Commands**:
  ```bash
  echo '{"jsonrpc":"2.0","id":1,"method":"ai.chat","params":{"key_id":"zhipu-main","message":"Hello, respond with just the word OK"}}' | docker exec -i armorclaw socat - UNIX-CONNECT:/run/armorclaw/bridge.sock
  ```

- [ ] 9. Test Matrix Server (Optional)

  **What to do**:
  - Check if Matrix/Conduit is running
  - Verify Matrix health endpoint

  **Commands**:
  ```bash
  docker ps | grep conduit
  curl -s http://localhost:6167/_matrix/client/versions
  ```

- [ ] 10. Document Test Results

  **What to do**:
  - Record all test results
  - Note any issues or errors
  - Update review.md with findings

---

## Success Criteria

- [x] Docker image pulled successfully
- [x] Container running and healthy
- [x] API key stored in keystore
- [x] Bridge RPC responding
- [x] AI chat returns response from z.ai provider
- [x] CI smoke test passes
- [x] All tests documented

---

## Guardrails

- **DO NOT** save API key to repo
- **DO NOT** commit any files containing the API key
- **DO** clean up any command history containing the key after testing
