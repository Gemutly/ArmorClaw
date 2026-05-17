# Matrix Deployment on VPS

> **Status:** FUNCTIONALLY COMPLETE — one deferred item (!status proof blocked by missing matrix.join_room RPC)
> **Priority:** Critical (security/communication)
> **Depends on:** fix-keystore-provider-validation (DONE - commit 9cd3a1d)

## TL;DR

> **Goal**: Enable Matrix communication on VPS so users can securely interact with ArmorClaw via ArmorChat mobile app.
> 
> **Deliverables**:
> - Matrix Conduit server running and accessible
> - Bridge connected to Matrix
> - Admin user created
> - E2E encrypted communication verified
> 
> **Estimated Effort**: Medium
> **Parallel Execution**: NO - sequential deployment steps

---

## Context

### Current State
- VPS at `5.183.11.149` has bridge binary running (systemd service)
- Provider validation fix deployed (commit 9cd3a1d)
- Matrix is **disabled** in quickstart config (`/etc/armorclaw/config.toml`)
- No Matrix containers running
- Docker is available on VPS

### Why Matrix Matters
Matrix is the control plane for secure E2E encrypted communication between:
- ArmorChat mobile app ↔ Bridge ↔ AI agents
- Human-in-the-loop approval flows
- All agent communications

---

## Work Objectives

### Core Objective
Deploy Matrix Conduit server and configure bridge to connect, enabling secure E2E communication.

### Concrete Deliverables
- Matrix Conduit container running on port 6167
- Bridge config updated with Matrix settings
- Admin Matrix user created
- Bridge Matrix user created
- Communication verified

### Definition of Done
- [x] `curl http://localhost:6167/_matrix/client/versions` returns valid response on VPS
- [x] Bridge logs show healthy Matrix sync (every 30s, zero errors)
- [x] Admin user can login via Matrix client (`armorclaw_admin` / v1.2 identifier format)
- [x] `!status` test attempted — confirmed M_FORBIDDEN (bridge not in room) *(deferred — blocked by missing `matrix.join_room` RPC, see follow-on plan `matrix-join-room-rpc.md`)*

### Must Have
- Matrix Conduit server running
- Bridge connected to Matrix
- Admin user created

### Must NOT Have (Guardrails)
- Do NOT expose Matrix to public internet without TLS (local only for now)
- Do NOT store API keys in config files
- Do NOT break existing bridge functionality

---

## Verification Strategy

### Test Decision
- **Automated tests**: None (infrastructure deployment)
- **Agent-executed QA**: SSH commands to verify services

### QA Scenarios
1. Matrix server health check
2. Bridge Matrix connection check
3. Admin user login test

---

## Execution Strategy

### Sequential Steps (cannot parallelize)

```
Step 1: Deploy Matrix Conduit container
├── Pull matrixconduit/matrix-conduit:latest
├── Create conduit config
├── Start container on port 6167
└── Wait for healthy status

Step 2: Create Matrix users
├── Create admin user via Conduit API
├── Create bridge user via Conduit API
└── Record credentials

Step 3: Update bridge config
├── Enable Matrix in config.toml
├── Set homeserver_url, username, password
└── Restart bridge service

Step 4: Verify communication
├── Check bridge logs for Matrix connection
├── Test Matrix API from bridge
└── Verify E2E encryption working

Step 5: Final verification
├── curl Matrix versions endpoint
├── Check bridge socket still works
└── Verify provider validation still works
```

---

## TODOs

- [x] 1. Deploy Matrix Conduit Container

  **What to do**:
  - SSH to VPS
  - Pull matrixconduit/matrix-conduit:latest image
  - Create Conduit config with server_name
  - Start container with proper volume mounts
  - Wait for health check to pass

  **References**:
  - `deploy/quickstart-entrypoint.sh:257-288` - Container creation pattern
  - `configs/conduit.toml` - Config template

  **QA Scenario**:
  ```
  Scenario: Matrix server health check
    Tool: Bash (SSH)
    Steps:
      1. SSH to VPS
      2. curl -s http://localhost:6167/_matrix/client/versions
    Expected: {"versions":["v1.0","v1.1",...]}
    Failure: Connection refused or empty response
  ```

- [x] 2. Create Matrix Users

  **What to do**:
  - Create admin user via Matrix API
  - Create bridge user via Matrix API
  - Both with `m.login.dummy` auth type

  **References**:
  - `deploy/quickstart-entrypoint.sh:313-368` - Bootstrap pattern

  **QA Scenario**:
  ```
  Scenario: Admin user exists
    Tool: Bash (SSH)
    Steps:
      1. SSH to VPS
      2. curl -X POST http://localhost:6167/_matrix/client/v3/register
         with admin credentials
    Expected: Access token returned
    Failure: User does not exist or wrong password
  ```

- [x] 3. Update Bridge Config for Matrix

  **What to do**:
  - Edit `/etc/armorclaw/config.toml` on VPS
  - Set `[matrix].enabled = true`
  - Set `homeserver_url = "http://localhost:6167"`
  - Set `username = "bridge"`
  - Set `password = "bridgepass"`
  - Restart armorclaw-bridge service

  **References**:
  - Current config at `/etc/armorclaw/config.toml` on VPS
  - `docker-compose-full.yml:117-122` - Environment variable pattern

  **QA Scenario**:
  ```
  Scenario: Bridge config updated
    Tool: Bash (SSH)
    Steps:
      1. SSH to VPS
      2. grep -A5 "\[matrix\]" /etc/armorclaw/config.toml
    Expected: enabled = true, homeserver_url set
    Failure: enabled = false or missing fields
  ```

- [x] 4. Restart Bridge and Verify Matrix Connection

  **What to do**:
  - systemctl restart armorclaw-bridge
  - Check logs for Matrix connection
  - Verify bridge socket still works
  - Test provider validation still works (zhipu key storage)

  **References**:
  - Service at `/etc/systemd/system/armorclaw-bridge.service`

  **QA Scenario**:
  ```
  Scenario: Bridge connected to Matrix
    Tool: Bash (SSH)
    Steps:
      1. SSH to VPS
      2. journalctl -u armorclaw-bridge -n 50 | grep -i matrix
    Expected: "Matrix connected" or similar success message
    Failure: "Matrix disabled" or connection errors
  ```

- [x] 5. Final Verification

  **What to do**:
  - Verify Matrix API accessible
  - Verify bridge RPC still works
  - Verify provider validation fix still works
  - Document any issues

  **QA Scenario**:
  ```
  Scenario: Full stack verification
    Tool: Bash (SSH)
    Steps:
      1. curl http://localhost:6167/_matrix/client/versions
      2. echo '{"method":"status"}' | socat - UNIX-CONNECT:/run/armorclaw/bridge.sock
      3. echo '{"method":"store_key","params":{"provider":"zhipu",...}}' | socat ...
    Expected: All three return success
    Failure: Any component not responding
  ```

---

## Commit Strategy

No code changes expected - infrastructure only.

If config templates need updates:
```
feat(config): add Matrix enablement documentation

- Document Matrix setup steps for quickstart
- No code changes, deployment-only
```

---

## Success Criteria

### Verification Commands
```bash
# On VPS:
curl http://localhost:6167/_matrix/client/versions  # Matrix running
systemctl status armorclaw-bridge                   # Bridge running
echo '{"method":"store_key",...}' | socat ...       # Provider validation works
```

### Final Checklist
- [x] Matrix Conduit server running on port 6167
- [x] Admin user created and can login (`armorclaw_admin`)
- [x] Bridge user created and syncing
- [x] Bridge config updated with Matrix settings
- [x] Bridge service restarted successfully
- [x] Matrix connection verified in bridge logs (healthy 30s sync loop)
- [x] Existing bridge functionality intact (`matrix.status` ✅, `store_key` ✅, `matrix.login` ✅)
- [x] Registration locked (`allow_registration = false`, verified M_FORBIDDEN)
- [x] `!status` test confirmed M_FORBIDDEN (bridge not in room) — *(deferred to `matrix-join-room-rpc.md`, now complete)*
- [x] Test artifact cleanup documented (test_probe_user, dummy `store_key` entry) — *(deferred to `matrix-join-room-rpc.md`, artifacts remain but !status E2E proof complete)*
