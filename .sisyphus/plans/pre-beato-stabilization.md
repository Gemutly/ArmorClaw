# Pre-BEATO Stabilization & Initial BEATO Validation

## TL;DR

> **Quick Summary**: Fix 7 FAIL + 3 SKIP test results caused by the new Docker image (v4.6.0) switching to mandatory HTTPS, missing Matrix bridge user, SSH self-reference, and missing VPS tools. Reach Pre-BEATO exit gates (>=80% PASS, <15% SKIP) before starting BEATO feature validation.
>
> **Deliverables**:
> - Fixed shared test libraries (`transport.sh`, `load_env.sh`, `restart_bridge.sh`) supporting HTTPS bridge
> - Re-registered Matrix bridge user in fresh Conduit DB
> - SSH key on VPS for self-referencing test scripts
> - Missing tools installed on VPS (`websocat`, `socat`, `jq`)
> - Fixed `test-yara-smoke.sh` container image filter
> - All 49 test scripts re-run with updated baseline
> - Pre-BEATO exit gate report
>
> **Estimated Effort**: Medium
> **Parallel Execution**: YES - 3 waves + final verification
> **Critical Path**: T0.4 (HTTPS transport fix) -> T0.1 (Matrix re-register) -> W1.1 (Matrix CLI coverage) -> W2.1 (Resilience tests) -> W3.1 (Pre-BEATO gates)

---

## Context

### Original Request
Fix current test failures (7 FAIL + 3 SKIP out of 49 scripts) caused by the new Docker image (mikegemut/armorclaw:latest v4.6.0 binary v1.1.0) which introduced mandatory TLS (`ListenAndServeTLS` unconditional) and stricter validation. Reach Pre-BEATO exit criteria before starting BEATO feature validation.

### Interview Summary
**Key Discussions**:
- **Root cause**: Bridge HTTP server (`bridge/pkg/http/server.go:230`) calls `ListenAndServeTLS` unconditionally — there is NO HTTP mode. All shared test libraries hardcode `http://localhost`.
- **Matrix bridge user**: Fresh Conduit DB after new image deploy, `bridge` user login returns 403 — need to re-register.
- **SSH self-reference**: `check_bridge_running()` fixed but individual test assertions still use `ssh_vps` for local checks; need SSH key on VPS for `ssh root@localhost` to work.
- **Missing tools**: `websocat`, `socat` not installed on VPS, `jq` fails via SSH pipe.
- **YARA container filter**: `ancestor=mikegemut/armorclaw:latest` may not match the actual running container.
- **governance-rpc test**: Self-contained (builds its own bridge binary) — may need different fix strategy than transport fix.

**Research Findings**:
- `transport.sh` line 24: `BRIDGE_HTTP_URL` defaults to `http://localhost:8080` — needs HTTPS + `-k` flag
- `load_env.sh` line 64: `check_bridge_running()` uses `curl -sf http://localhost` — needs HTTPS + `-k`
- `restart_bridge.sh` lines 27, 61: `_bridge_is_local()` and readiness polling use `http://localhost` — needs HTTPS + `-k`
- TLS cert fingerprint: `177751f0beadfba62eff910d6fe6cb8b6cd2a3462fe57292d1ff83559dfb00b71`
- Bridge healthy on HTTPS: `curl -ksS https://localhost:8080/health` returns `{"status":"ok","version":"4.6.0"}`

### Metis Review
**Identified Gaps** (addressed):
- `load_env.sh` also broken by HTTPS but wasn't in original Wave 0 scope — NOW included in T0.4
- `restart_bridge.sh` readiness polling also uses HTTP — NOW included in T0.4
- `governance-rpc` test builds its own bridge — uses Go RPC directly, not affected by HTTPS transport fix, but may need binary path fix
- TLS certificate is self-signed — all curl calls need `-k` (insecure) flag for localhost testing
- Don't over-upgrade test scripts for unimplemented BEATO features

---

## Work Objectives

### Core Objective
Fix all infrastructure-level test failures so the harness produces reliable PASS/FAIL results reflecting actual product behavior, not transport or environment issues. Reach Pre-BEATO exit gates (>=80% PASS, <15% SKIP).

### Concrete Deliverables
- `tests/lib/transport.sh` — HTTPS-aware with auto-detection
- `tests/lib/load_env.sh` — HTTPS-aware `check_bridge_running()`
- `tests/lib/restart_bridge.sh` — HTTPS-aware `_bridge_is_local()` and readiness polling
- `tests/test-yara-smoke.sh` — Fixed container filter
- VPS: Matrix bridge user re-registered and verified
- VPS: SSH key copied to `~/.ssh/authorized_keys`
- VPS: `websocat`, `socat`, `jq` installed
- Full test suite re-run report with Pre-BEATO gate assessment

### Definition of Done
- [ ] `curl -ksS https://localhost:8080/health` returns 200 on VPS
- [ ] `source tests/lib/transport.sh && detect_transport` reports `mode=http` (HTTPS detected)
- [ ] All 49 test scripts run without transport errors
- [ ] Pre-BEATO exit gates: >=80% PASS, <15% SKIP
- [ ] No FAIL caused by infrastructure (transport, SSH, missing tools)

### Must Have
- HTTPS support in all 3 shared test libraries
- Matrix bridge user registered and login working
- VPS tooling installed
- Full test suite re-run with evidence

### Must NOT Have (Guardrails)
- Do NOT modify `container-setup.sh` or `deploy/container-setup.sh`
- Do NOT change `bridge/pkg/yara/scanner.go` internal logic
- Do NOT add new RPC methods
- Do NOT add structured logging library — stdlib `log` only
- Do NOT touch `test-element-x-flow.sh` (out of scope)
- Do NOT change production handler behavior to fix tests — fix assertions instead
- Do NOT treat feature-gated voice/e2ee/keystore responses as product bugs
- Do NOT refactor production code to make it testable — only add test files
- Do NOT log or print password values anywhere
- Do NOT touch `cmd/` directory
- Do NOT remove SQLCipher, Matrix control plane, or approval flow
- Do NOT change HEALTHCHECK in Dockerfile.quickstart
- Do NOT delete `deploy/quickstart-entrypoint.sh` from the repo
- Do NOT add config versioning/migration logic
- Do NOT over-upgrade test scripts for unimplemented BEATO features
- Do NOT modify `transport.sh` to hardcode `https://` — use env var with HTTP fallback

---

## Verification Strategy

> **ZERO HUMAN INTERVENTION** - ALL verification is agent-executed. No exceptions.

### Test Decision
- **Infrastructure exists**: YES (existing test harness from Phase 1)
- **Automated tests**: Tests-after (fix infrastructure, then re-run full suite)
- **Framework**: Bash test harness (49 scripts)

### QA Policy
Every task MUST include agent-executed QA scenarios.
Evidence saved to `.sisyphus/evidence/pre-beato-{task-name}/`.

- **VPS operations**: SSH commands via `ssh_vps()` wrapper
- **Transport verification**: `curl` with `-k` flag against HTTPS bridge
- **Matrix verification**: `curl` against Conduit API
- **Test re-runs**: Full bash harness execution with evidence capture

---

## Execution Strategy

### Parallel Execution Waves

```
Wave 0 (Start Immediately — critical VPS infrastructure fixes):
├── T0.1: Re-register Matrix bridge user [quick]
├── T0.2: Fix SSH self-reference (copy key to VPS) [quick]
├── T0.3: Install missing tools on VPS [quick]
├── T0.4: Fix shared libraries for HTTPS (transport.sh, load_env.sh, restart_bridge.sh) [deep]
└── T0.5: Fix test-yara-smoke.sh container image filter [quick]

Wave 1 (After Wave 0 — validation + coverage expansion):
├── W1.1: Matrix CLI coverage — register, login, send, receive [unspecified-high]
├── W1.2: WebSocket reconnection test [unspecified-high]
└── W1.3: Full validation re-run (all 49 scripts) [deep]

Wave 2 (After Wave 1 — resilience + E2E):
├── W2.1: Bridge restart resilience tests [unspecified-high]
├── W2.2: Studio/Secretary E2E via Matrix [deep]
└── W2.3: Browser/Email pipeline E2E [unspecified-high]

Wave 3 (After Wave 2 — Pre-BEATO gates + report):
├── W3.1: Pre-BEATO gate assessment [quick]
└── W3.2: Coverage baseline report + CI gate config [writing]

Wave FINAL (After ALL tasks — 4 parallel reviews):
├── F1: Plan compliance audit (oracle)
├── F2: Code quality review (unspecified-high)
├── F3: Real manual QA (unspecified-high)
└── F4: Scope fidelity check (deep)
-> Present results -> Get explicit user okay

Critical Path: T0.4 -> T0.1 -> W1.1 -> W2.1 -> W3.1 -> F1-F4
Parallel Speedup: ~60% faster than sequential
Max Concurrent: 5 (Wave 0)
```

### Dependency Matrix

| Task | Depends On | Blocks |
|------|-----------|--------|
| T0.1 | None | W1.1, W1.3, W2.2 |
| T0.2 | None | W1.3 |
| T0.3 | None | W1.2, W1.3 |
| T0.4 | None | W1.3, W2.1 |
| T0.5 | None | W1.3 |
| W1.1 | T0.1 | W2.2 |
| W1.2 | T0.3 | W2.1 |
| W1.3 | T0.1-T0.5 | W2.1-W2.3, W3.1 |
| W2.1 | T0.4, W1.2, W1.3 | W3.1 |
| W2.2 | W1.1, W1.3 | W3.1 |
| W2.3 | W1.3 | W3.1 |
| W3.1 | W2.1-W2.3 | W3.2 |
| W3.2 | W3.1 | F1-F4 |

### Agent Dispatch Summary

- **Wave 0**: **5 tasks** - T0.1-T0.3,T0.5 -> `quick`, T0.4 -> `deep`
- **Wave 1**: **3 tasks** - W1.1-W1.2 -> `unspecified-high`, W1.3 -> `deep`
- **Wave 2**: **3 tasks** - W2.1,W2.3 -> `unspecified-high`, W2.2 -> `deep`
- **Wave 3**: **2 tasks** - W3.1 -> `quick`, W3.2 -> `writing`
- **FINAL**: **4 tasks** - F1 -> `oracle`, F2-F3 -> `unspecified-high`, F4 -> `deep`

---

## TODOs

- [x] T0.1. **Re-register Matrix Bridge User** — `quick`

  **What to do**:
  - SSH to VPS and register a new `bridge` user in the fresh Conduit DB
  - Use Conduit's shared-secret registration API: `POST http://localhost:6167/_matrix/client/v3/register`
  - Store the credentials in `/opt/armorclaw/.env` as `ARMORCLAW_MATRIX_USER` and `ARMORCLAW_MATRIX_PASSWORD`
  - Update `/opt/armorclaw/data/config.toml` `[matrix]` section with new credentials
  - Restart the bridge container to pick up new Matrix credentials
  - Verify bridge reports Matrix as `logged_in` via `curl -ksS https://localhost:8080/api` RPC `status` method

  **Must NOT do**:
  - Do NOT log or print password values in test output or evidence files
  - Do NOT change Conduit's `allow_registration` to `true` permanently
  - Do NOT use the admin token as the bridge user password

  **Recommended Agent Profile**:
  - **Category**: `quick`
    - Reason: Single-purpose VPS operation, well-defined steps
  - **Skills**: []
  - **Skills Evaluated but Omitted**:
    - `claw-ssh`: Would be useful but ssh_vps pattern already established

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 0 (with T0.2, T0.3, T0.4, T0.5)
  - **Blocks**: W1.1, W1.3, W2.2
  - **Blocked By**: None (can start immediately)

  **References**:

  **Pattern References**:
  - `tests/test-system-health-baseline.sh:40-60` — Existing Matrix health check pattern (how scripts verify Matrix is alive)
  - `/opt/armorclaw/.env` (on VPS) — Current env file format for credential storage

  **API/Type References**:
  - Matrix v3 registration API: `POST /_matrix/client/v3/register` with `{"type":"m.login.dummy","username":"bridge","password":"..."}` — Conduit's registration format
  - Bridge RPC `status` method response: `{"result":{"matrix":{"status":"logged_in",...}}}`

  **External References**:
  - Conduit registration: `https://conduit.rs/configuration.html` — Shared-secret registration

  **WHY Each Reference Matters**:
  - `test-system-health-baseline.sh`: Shows how existing tests check Matrix status — match this pattern for verification
  - VPS `.env`: Must follow existing env file format for credential storage (key=value, no quotes)

  **Acceptance Criteria**:

  **QA Scenarios (MANDATORY):**

  ```
  Scenario: Matrix bridge user registration succeeds
    Tool: Bash (ssh_vps)
    Preconditions: Conduit running on localhost:6167, bridge container running
    Steps:
      1. ssh_vps "curl -sS -X POST http://localhost:6167/_matrix/client/v3/register -H 'Content-Type: application/json' -d '{\"username\":\"bridge\",\"password\":\"c4f30bd263d3b36753a89d36b7540fb4\",\"auth\":{\"type\":\"m.login.dummy\"}}'"
      2. Assert HTTP 200 response contains "access_token" and "user_id"
      3. ssh_vps "curl -sS -X POST http://localhost:6167/_matrix/client/v3/login -H 'Content-Type: application/json' -d '{\"type\":\"m.login.password\",\"identifier\":{\"type\":\"m.id.user\",\"user\":\"bridge\"},\"password\":\"c4f30bd263d3b36753a89d36b7540fb4\"}'"
      4. Assert login returns 200 with access_token
    Expected Result: Both registration and login return 200 with valid access tokens
    Failure Indicators: 403 (registration disabled/failed), 400 (bad request format)
    Evidence: .sisyphus/evidence/pre-beato-t01/matrix-registration.txt

  Scenario: Bridge reports Matrix as logged_in after registration
    Tool: Bash (ssh_vps)
    Preconditions: Bridge user registered, bridge restarted with new credentials
    Steps:
      1. ssh_vps "curl -ksS https://localhost:8080/api -H 'Content-Type: application/json' -H 'Authorization: Bearer c8762f64e1921c5b46ff470f3ee37aae7aa2ad5d01acd1fbcbddd3d5d2e066b9' -d '{\"jsonrpc\":\"2.0\",\"id\":1,\"method\":\"status\"}'"
      2. Assert response contains "logged_in" in matrix.status field
    Expected Result: Bridge status RPC returns matrix.status = "logged_in"
    Failure Indicators: "not_configured", "login_failed", or connection error
    Evidence: .sisyphus/evidence/pre-beato-t01/bridge-matrix-status.txt
  ```

  **Commit**: YES (groups with Wave 0)
  - Message: `fix(tests): HTTPS transport for shared libraries + VPS infra fixes`
  - Files: `.env` (VPS only, not committed), `config.toml` (VPS only)
  - Pre-commit: N/A (VPS-only changes, no repo commit needed for this task)

- [x] T0.2. **Fix SSH Self-Reference** — `quick`

  **What to do**:
  - Copy the SSH key `~/.ssh/openclaw_win` (public key) to VPS `~/.ssh/authorized_keys`
  - Verify `ssh -i ~/.ssh/openclaw_win -o StrictHostKeyChecking=no root@localhost "echo ok"` works from the VPS itself
  - This fixes test scripts that use `ssh_vps` for local bridge checks — they can now reach `root@localhost` on the VPS

  **Must NOT do**:
  - Do NOT leave SSH keys in insecure locations
  - Do NOT change SSH daemon configuration (Port, PermitRootLogin, etc.)
  - Do NOT copy private keys to the VPS

  **Recommended Agent Profile**:
  - **Category**: `quick`
    - Reason: Single SSH key copy operation
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 0 (with T0.1, T0.3, T0.4, T0.5)
  - **Blocks**: W1.3
  - **Blocked By**: None

  **References**:

  **Pattern References**:
  - `tests/lib/load_env.sh:52-53` — `ssh_vps()` function that tests call — this is what needs to work from VPS to itself
  - `~/.ssh/openclaw_win` (local) — SSH key path used by all test scripts

  **WHY Each Reference Matters**:
  - `ssh_vps()`: Understanding its exact invocation (`ssh -i $SSH_KEY_PATH -o StrictHostKeyChecking=no -o ConnectTimeout=10 ${VPS_USER}@${VPS_IP}`) tells us the key must work for `root@5.183.11.149` AND `root@localhost`

  **Acceptance Criteria**:

  **QA Scenarios (MANDATORY):**

  ```
  Scenario: SSH self-reference works from VPS
    Tool: Bash (ssh_vps)
    Preconditions: SSH public key exists locally at ~/.ssh/openclaw_win.pub
    Steps:
      1. Extract public key: cat ~/.ssh/openclaw_win.pub
      2. ssh_vps "echo '<pubkey>' >> ~/.ssh/authorized_keys"
      3. ssh_vps "ssh -i /root/.ssh/id_rsa -o StrictHostKeyChecking=no root@localhost 'echo ssh-ok'" 2>/dev/null || ssh_vps "ssh -o StrictHostKeyChecking=no root@localhost 'echo ssh-ok'"
      4. Assert output contains "ssh-ok"
    Expected Result: SSH from VPS to itself (localhost) succeeds
    Failure Indicators: "Permission denied", "Connection refused", timeout
    Evidence: .sisyphus/evidence/pre-beato-t02/ssh-self-reference.txt

  Scenario: ssh_vps function works when VPS_IP=localhost on VPS
    Tool: Bash (ssh_vps)
    Preconditions: Key installed, SSH daemon running
    Steps:
      1. ssh_vps "VPS_IP=127.0.0.1 SSH_KEY_PATH=/root/.ssh/id_rsa source tests/lib/load_env.sh && ssh_vps 'echo ok'"
      2. Assert output contains "ok"
    Expected Result: ssh_vps() resolves and executes remote command
    Failure Indicators: VPS_IP error, SSH connection failure
    Evidence: .sisyphus/evidence/pre-beato-t02/ssh-vps-function.txt
  ```

  **Commit**: NO (VPS-only infrastructure change, no repo files modified)

- [x] T0.3. **Install Missing Tools on VPS** — `quick`

  **What to do**:
  - SSH to VPS and install: `websocat`, `socat`, `jq`
  - Use `apt-get install -y jq socat` for jq and socat
  - For websocat: check if available via apt, otherwise download binary from GitHub releases
  - Verify each tool is in PATH and functional

  **Must NOT do**:
  - Do NOT install unnecessary packages
  - Do NOT modify VPS apt sources or add untrusted PPAs
  - Do NOT install development headers or build tools

  **Recommended Agent Profile**:
  - **Category**: `quick`
    - Reason: Simple package installation
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 0 (with T0.1, T0.2, T0.4, T0.5)
  - **Blocks**: W1.2, W1.3
  - **Blocked By**: None

  **References**:

  **Pattern References**:
  - `tests/lib/event_subscriber_helper.sh` — Uses `websocat` for WebSocket event subscription; shows what features are needed (self-signed TLS via `-k` flag)
  - `tests/lib/transport.sh:121` — Uses `socat` for Unix socket JSON-RPC; shows exact invocation pattern
  - Various test scripts — Use `jq` for JSON parsing via `ssh_vps "curl ... | jq .field"`

  **WHY Each Reference Matters**:
  - `event_subscriber_helper.sh`: Confirms websocat must support `-k` flag for self-signed certs
  - `transport.sh`: Confirms socat is needed for Unix socket bridge communication
  - jq usage: Multiple tests pipe curl output through jq via SSH, which fails without jq on VPS

  **Acceptance Criteria**:

  **QA Scenarios (MANDATORY):**

  ```
  Scenario: All required tools installed and functional
    Tool: Bash (ssh_vps)
    Preconditions: VPS accessible, apt-get available
    Steps:
      1. ssh_vps "jq --version"
      2. Assert output contains "jq-" (version string)
      3. ssh_vps "socat -V | head -1"
      4. Assert output contains "socat version"
      5. ssh_vps "websocat --version"
      6. Assert output contains version string or exits 0
    Expected Result: All 3 tools return version info without errors
    Failure Indicators: "command not found", exit code non-zero
    Evidence: .sisyphus/evidence/pre-beato-t03/tools-installed.txt

  Scenario: jq works via SSH pipe (reproducing test-platform-adapters failure)
    Tool: Bash (ssh_vps)
    Preconditions: jq installed on VPS
    Steps:
      1. ssh_vps "echo '{\"status\":\"ok\"}' | jq .status"
      2. Assert output is exactly "ok" (with quotes)
    Expected Result: jq parses JSON correctly through SSH pipe
    Failure Indicators: "command not found", parse error
    Evidence: .sisyphus/evidence/pre-beato-t03/jq-via-ssh.txt
  ```

  **Commit**: NO (VPS-only infrastructure change)

- [x] T0.4. **Fix Shared Libraries for HTTPS** — `deep`

  **What to do**:
  - **`tests/lib/transport.sh`**: Make `BRIDGE_HTTP_URL` aware of HTTPS. Add `BRIDGE_HTTPS_MODE` env var (default: auto-detect). When auto-detecting, try HTTPS first (`curl -ksS`), fall back to HTTP. Update `detect_transport()` health check to use `curl -ksS` for HTTPS. Update `rpc_call()` and `rpc_call_auth()` to use `curl -ksS` for HTTPS URLs. Keep HTTP as fallback for older deployments.
  - **`tests/lib/load_env.sh`**: Update `check_bridge_running()` line 64 to try HTTPS first with `-k` flag, then fall back to HTTP. Pattern: `curl -ksSf --max-time 2 "https://localhost:${BRIDGE_PORT}/health" || curl -sf --max-time 2 "http://localhost:${BRIDGE_PORT}/health"`
  - **`tests/lib/restart_bridge.sh`**: Update `_bridge_is_local()` (line 27) and readiness polling (line 61) to try HTTPS first with `-k` flag, then fall back to HTTP. Same dual-try pattern.
  - Add `BRIDGE_HTTPS_MODE` env var support: `auto` (default, try HTTPS first), `https` (force HTTPS), `http` (force HTTP).
  - Ensure backward compatibility: existing deployments with HTTP-only bridge continue to work.

  **Must NOT do**:
  - Do NOT hardcode `https://` — must auto-detect with HTTP fallback
  - Do NOT remove HTTP support entirely
  - Do NOT change the function signatures of `rpc_call`, `rpc_call_auth`, `detect_transport`
  - Do NOT add new dependencies

  **Recommended Agent Profile**:
  - **Category**: `deep`
    - Reason: Multi-file change with backward compatibility requirement and edge cases (cert validation, fallback logic)
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 0 (with T0.1, T0.2, T0.3, T0.5)
  - **Blocks**: W1.3, W2.1
  - **Blocked By**: None

  **References**:

  **Pattern References**:
  - `tests/lib/transport.sh:24` — `BRIDGE_HTTP_URL` default: `http://localhost:${BRIDGE_PORT}` — this is the hardcoded HTTP that breaks
  - `tests/lib/transport.sh:78` — Health check: `curl -sf "${BRIDGE_HTTP_URL}/health"` — needs HTTPS + `-k`
  - `tests/lib/transport.sh:139-142` — HTTP transport RPC: `curl -sf "${BRIDGE_HTTP_URL}/api"` — needs HTTPS + `-k`
  - `tests/lib/transport.sh:193-199` — Auth RPC: same HTTP transport pattern
  - `tests/lib/load_env.sh:64` — `check_bridge_running()` health check: `curl -sf --max-time 2 "http://localhost:${BRIDGE_PORT}/health"`
  - `tests/lib/restart_bridge.sh:27` — `_bridge_is_local()`: `curl -sf --max-time 2 "http://localhost:${BRIDGE_PORT}/health"`
  - `tests/lib/restart_bridge.sh:61` — Readiness polling: same HTTP pattern via `_bridge_is_local`

  **API/Type References**:
  - `bridge/pkg/http/server.go:230` — `ListenAndServeTLS` unconditional — confirms HTTPS is always-on, no HTTP mode exists

  **Test References**:
  - `tests/test-trust-layer.sh` — Uses `rpc_call` and `rpc_call_auth` — verify these still work after change

  **WHY Each Reference Matters**:
  - `transport.sh:24,78,139,193`: Every curl call in this file uses `BRIDGE_HTTP_URL` which defaults to HTTP — all must support HTTPS
  - `load_env.sh:64`: The bridge health check used by every test that calls `check_bridge_running()` — must detect HTTPS
  - `restart_bridge.sh:27,61`: Bridge restart verification — must poll HTTPS health endpoint correctly
  - `server.go:230`: The ground truth — bridge is HTTPS-only, confirming our fix direction

  **Acceptance Criteria**:

  **QA Scenarios (MANDATORY):**

  ```
  Scenario: transport.sh detects HTTPS bridge automatically
    Tool: Bash
    Preconditions: Bridge running on VPS with HTTPS on port 8080
    Steps:
      1. source tests/lib/transport.sh
      2. BRIDGE_PORT=8080 detect_transport
      3. Assert output contains "mode=http" (HTTPS detected as transport mode)
      4. Assert $HAS_HTTP == true
    Expected Result: Transport library detects HTTPS bridge and sets HAS_HTTP=true
    Failure Indicators: "mode=none", HAS_HTTP=false, curl errors
    Evidence: .sisyphus/evidence/pre-beato-t04/transport-https-detect.txt

  Scenario: rpc_call works over HTTPS
    Tool: Bash
    Preconditions: Bridge running on VPS with HTTPS, ADMIN_TOKEN set
    Steps:
      1. source tests/lib/transport.sh
      2. ADMIN_TOKEN=c8762f64e1921c5b46ff470f3ee37aae7aa2ad5d01acd1fbcbddd3d5d2e066b9
      3. BRIDGE_PORT=8080 rpc_call status
      4. Assert response contains "jsonrpc":"2.0" and "result"
    Expected Result: RPC call succeeds over HTTPS transport
    Failure Indicators: "no bridge transport", curl exit code 35/60 (SSL error)
    Evidence: .sisyphus/evidence/pre-beato-t04/rpc-call-https.txt

  Scenario: check_bridge_running detects HTTPS bridge
    Tool: Bash (ssh_vps)
    Preconditions: Bridge running on VPS with HTTPS
    Steps:
      1. ssh_vps "cd /opt/armorclaw && source tests/lib/load_env.sh && check_bridge_running"
      2. Assert exit code 0
    Expected Result: check_bridge_running returns 0 for HTTPS bridge
    Failure Indicators: Exit code 1 (bridge not detected)
    Evidence: .sisyphus/evidence/pre-beato-t04/check-bridge-https.txt

  Scenario: HTTP fallback works when HTTPS unavailable
    Tool: Bash
    Preconditions: No bridge running locally (simulates old deployment)
    Steps:
      1. source tests/lib/transport.sh
      2. BRIDGE_HTTPS_MODE=http detect_transport
      3. Assert no errors about HTTPS failures
      4. BRIDGE_HTTP_URL="http://localhost:9999" detect_transport
      5. Assert mode=none (HTTP tried, no bridge)
    Expected Result: HTTP fallback path doesn't crash or hang
    Failure Indicators: Timeout, SSL errors on HTTP attempt
    Evidence: .sisyphus/evidence/pre-beato-t04/http-fallback.txt

  Scenario: restart_bridge works with HTTPS bridge
    Tool: Bash (ssh_vps)
    Preconditions: Bridge running in Docker on VPS with HTTPS
    Steps:
      1. ssh_vps "cd /opt/armorclaw && source tests/lib/load_env.sh && source tests/lib/restart_bridge.sh && restart_bridge 60"
      2. Assert output contains "Bridge ready after"
      3. Assert exit code 0
    Expected Result: Bridge restarts and readiness poll detects HTTPS health endpoint
    Failure Indicators: "Bridge not ready after 60s", timeout
    Evidence: .sisyphus/evidence/pre-beato-t04/restart-bridge-https.txt
  ```

  **Commit**: YES (groups with Wave 0)
  - Message: `fix(tests): HTTPS transport for shared libraries + VPS infra fixes`
  - Files: `tests/lib/transport.sh`, `tests/lib/load_env.sh`, `tests/lib/restart_bridge.sh`
  - Pre-commit: `bash -n tests/lib/transport.sh && bash -n tests/lib/load_env.sh && bash -n tests/lib/restart_bridge.sh`

- [x] T0.5. **Fix test-yara-smoke.sh Container Filter** — `quick`

  **What to do**:
  - Update `CONTAINER_FILTER` in `test-yara-smoke.sh` line 25 from `ancestor=mikegemut/armorclaw:latest` to use `name=armorclaw` (matching the container name, not image ancestry)
  - The current filter fails because Docker image ancestry matching is fragile — the running container may not match the exact image tag
  - Alternative: use `--filter "name=armorclaw"` which matches the container name used in deployment

  **Must NOT do**:
  - Do NOT change the YARA test logic or test files
  - Do NOT modify the container deployment naming

  **Recommended Agent Profile**:
  - **Category**: `quick`
    - Reason: Single-line change in one test script
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 0 (with T0.1-T0.4)
  - **Blocks**: W1.3
  - **Blocked By**: None

  **References**:

  **Pattern References**:
  - `tests/test-yara-smoke.sh:25` — `CONTAINER_FILTER="ancestor=mikegemut/armorclaw:latest"` — the broken filter
  - `tests/test-yara-smoke.sh:33` — `ssh_vps "docker ps --filter ${CONTAINER_FILTER} --format '{{.ID}}' | head -1"` — how it's used
  - `tests/lib/transport.sh:70-74` — Shows correct Docker filter pattern: `docker ps --filter name=armorclaw`

  **WHY Each Reference Matters**:
  - Line 25: The exact line to change
  - Line 33: How the filter is consumed — confirms it's used directly in `docker ps --filter`
  - `transport.sh:70`: Shows the established pattern in other parts of the codebase using `name=armorclaw`

  **Acceptance Criteria**:

  **QA Scenarios (MANDATORY):**

  ```
  Scenario: YARA smoke test discovers container correctly
    Tool: Bash (ssh_vps)
    Preconditions: ArmorClaw container running on VPS
    Steps:
      1. ssh_vps "docker ps --filter name=armorclaw --format '{{.ID}}' | head -1"
      2. Assert output is a non-empty container ID (12 hex chars)
    Expected Result: Container ID found using name filter
    Failure Indicators: Empty output (no container found)
    Evidence: .sisyphus/evidence/pre-beato-t05/yara-container-filter.txt

  Scenario: YARA smoke test passes end-to-end
    Tool: Bash
    Preconditions: T0.3 (jq installed), T0.4 (transport fixed), container running with YARA rules
    Steps:
      1. bash tests/test-yara-smoke.sh
      2. Assert exit code 0
      3. Assert output contains "YARA Smoke:" and PASS count > 0
    Expected Result: At least 3/5 YARA tests pass (compile, detection, clean file)
    Failure Indicators: "No running ArmorClaw container found", YARA compile errors
    Evidence: .sisyphus/evidence/pre-beato-t05/yara-smoke-e2e.txt
  ```

  **Commit**: YES (groups with Wave 0)
  - Message: `fix(tests): HTTPS transport for shared libraries + VPS infra fixes`
  - Files: `tests/test-yara-smoke.sh`
  - Pre-commit: `bash -n tests/test-yara-smoke.sh`

- [x] W1.1. **Matrix CLI Coverage — Register, Login, Send, Receive** — `unspecified-high`

  **What to do**:
  - Verify Matrix bridge user can send and receive messages
  - Test Matrix room creation via bridge RPC
  - Test message sending from bridge to Matrix room
  - Test message receiving from Matrix room to bridge
  - Verify Matrix event stream works (sync endpoint)
  - Create/update test script `tests/test-matrix-client-flow.sh` with these scenarios
  - Fix any assertions in existing Matrix test scripts that expect wrong response shapes

  **Must NOT do**:
  - Do NOT add new RPC methods
  - Do NOT modify Matrix Conduit configuration beyond what T0.1 set up
  - Do NOT treat feature-gated voice/e2ee/keystore responses as bugs

  **Recommended Agent Profile**:
  - **Category**: `unspecified-high`
    - Reason: Multi-scenario E2E testing across Matrix + Bridge
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES (with W1.2)
  - **Parallel Group**: Wave 1
  - **Blocks**: W2.2
  - **Blocked By**: T0.1 (Matrix user registered)

  **References**:

  **Pattern References**:
  - `tests/test-system-health-baseline.sh` — Existing Matrix health check patterns
  - `tests/test-platform-adapters.sh` — Matrix adapter test patterns (may be partially working)
  - `/opt/armorclaw/data/config.toml` (on VPS) — Matrix configuration with credentials

  **API/Type References**:
  - Matrix v3 sync: `GET /_matrix/client/v3/sync?timeout=0`
  - Matrix v3 send: `PUT /_matrix/client/v3/rooms/{roomId}/send/m.room.message/{txnId}`
  - Bridge RPC `status`: Returns `matrix.status` field

  **WHY Each Reference Matters**:
  - `test-system-health-baseline.sh`: Shows working Matrix health check pattern to follow
  - `test-platform-adapters.sh`: May have existing Matrix adapter tests that need fixing
  - Bridge RPC `status`: The canonical way to verify bridge-to-Matrix connectivity

  **Acceptance Criteria**:

  **QA Scenarios (MANDATORY):**

  ```
  Scenario: Matrix user login succeeds after T0.1 registration
    Tool: Bash (ssh_vps)
    Preconditions: T0.1 completed, bridge user registered
    Steps:
      1. ssh_vps "curl -sS -X POST http://localhost:6167/_matrix/client/v3/login -H 'Content-Type: application/json' -d '{\"type\":\"m.login.password\",\"identifier\":{\"type\":\"m.id.user\",\"user\":\"bridge\"},\"password\":\"c4f30bd263d3b36753a89d36b7540fb4\"}'"
      2. Assert HTTP 200 and response contains "access_token"
    Expected Result: Login returns valid access token
    Failure Indicators: 403 (forbidden), 400 (bad request), "M_FORBIDDEN"
    Evidence: .sisyphus/evidence/pre-beato-w11/matrix-login.txt

  Scenario: Matrix sync returns valid response
    Tool: Bash (ssh_vps)
    Preconditions: Bridge user logged in with access token
    Steps:
      1. ACCESS_TOKEN=$(ssh_vps "curl -sS -X POST http://localhost:6167/_matrix/client/v3/login -H 'Content-Type: application/json' -d '...' | jq -r .access_token")
      2. ssh_vps "curl -sS 'http://localhost:6167/_matrix/client/v3/sync?timeout=0' -H 'Authorization: Bearer $ACCESS_TOKEN'"
      3. Assert HTTP 200 and response contains "next_batch"
    Expected Result: Sync returns valid initial state with next_batch token
    Failure Indicators: 401 (unauthorized), timeout
    Evidence: .sisyphus/evidence/pre-beato-w11/matrix-sync.txt

  Scenario: Bridge RPC status shows logged_in
    Tool: Bash (ssh_vps)
    Preconditions: Bridge running with Matrix credentials configured
    Steps:
      1. ssh_vps "curl -ksS https://localhost:8080/api -H 'Content-Type: application/json' -H 'Authorization: Bearer c8762f64e1921c5b46ff470f3ee37aae7aa2ad5d01acd1fbcbddd3d5d2e066b9' -d '{\"jsonrpc\":\"2.0\",\"id\":1,\"method\":\"status\"}'"
      2. Assert response contains "logged_in" in matrix section
    Expected Result: Bridge confirms Matrix connection is active
    Failure Indicators: "not_configured", "login_failed", connection error
    Evidence: .sisyphus/evidence/pre-beato-w11/bridge-status.txt
  ```

  **Commit**: YES
  - Message: `test(matrix): Matrix CLI coverage + WebSocket reconnection tests`
  - Files: `tests/test-matrix-client-flow.sh` (updated)
  - Pre-commit: `bash -n tests/test-matrix-client-flow.sh`

- [x] W1.2. **WebSocket Reconnection Test** — `unspecified-high`

  **What to do**:
  - Verify WebSocket event subscription works with `websocat` (installed in T0.3)
  - Test WebSocket connection to bridge event stream
  - Test reconnection after bridge restart (WebSocket should reconnect)
  - Update `tests/lib/event_subscriber_helper.sh` if needed for HTTPS WSS support
  - Create evidence of WebSocket stability for >=30 seconds

  **Must NOT do**:
  - Do NOT add new WebSocket endpoints
  - Do NOT modify production WebSocket handler code
  - Do NOT require specific event content — just verify connection stability

  **Recommended Agent Profile**:
  - **Category**: `unspecified-high`
    - Reason: WebSocket testing with timing sensitivity and tool dependencies
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES (with W1.1)
  - **Parallel Group**: Wave 1
  - **Blocks**: W2.1
  - **Blocked By**: T0.3 (websocat installed), T0.4 (transport HTTPS)

  **References**:

  **Pattern References**:
  - `tests/lib/event_subscriber_helper.sh` — Existing WebSocket helper with `websocat` patterns
  - `tests/test-eventbus-streaming.sh` — Existing event bus test with WebSocket scenarios

  **WHY Each Reference Matters**:
  - `event_subscriber_helper.sh`: Shows established pattern for WebSocket subscription — must match this
  - `test-eventbus-streaming.sh`: Has working WebSocket test scenarios to build on

  **Acceptance Criteria**:

  **QA Scenarios (MANDATORY):**

  ```
  Scenario: WebSocket connection stable for 30 seconds
    Tool: Bash (ssh_vps)
    Preconditions: websocat installed (T0.3), bridge running with HTTPS
    Steps:
      1. ssh_vps "timeout 35 websocat -k wss://localhost:8080/ws/events -H 'Authorization: Bearer c8762f64e1921c5b46ff470f3ee37aae7aa2ad5d01acd1fbcbddd3d5d2e066b9' 2>&1 | head -20"
      2. Assert connection was established (no "connection refused" or SSL error)
      3. Assert no disconnection within 30 seconds
    Expected Result: WebSocket connection maintained for >=30 seconds
    Failure Indicators: "connection refused", SSL handshake failure, immediate disconnect
    Evidence: .sisyphus/evidence/pre-beato-w12/websocket-stability.txt

  Scenario: WebSocket reconnects after bridge restart
    Tool: Bash (ssh_vps)
    Preconditions: WebSocket connection established, bridge restartable
    Steps:
      1. Start background WebSocket listener
      2. Restart bridge (docker restart)
      3. Wait for bridge to become ready (HTTPS health check)
      4. Attempt new WebSocket connection
      5. Assert new connection succeeds
    Expected Result: New WebSocket connection succeeds after bridge restart
    Failure Indicators: Persistent "connection refused" after bridge ready
    Evidence: .sisyphus/evidence/pre-beato-w12/websocket-reconnect.txt
  ```

  **Commit**: YES (groups with W1.1)
  - Message: `test(matrix): Matrix CLI coverage + WebSocket reconnection tests`
  - Files: `tests/lib/event_subscriber_helper.sh` (if modified), new test evidence
  - Pre-commit: `bash -n tests/lib/event_subscriber_helper.sh`

- [x] W1.3. **Full Validation Re-run (All 49 Scripts)** — `deep`

  **What to do**:
  - After ALL Wave 0 tasks complete, run the full 49-script test suite on VPS
  - Capture complete output with timestamps
  - Classify results: PASS, FAIL, SKIP, GATED_EXPECTED, ENV_MISSING
  - Compare against Phase 1 baseline (39 PASS, 7 FAIL, 3 SKIP)
  - Generate comparison report showing improvement/digression per script
  - Identify any NEW failures introduced by Wave 0 changes
  - Save evidence to `.sisyphus/evidence/pre-beato-w13/full-suite/`

  **Must NOT do**:
  - Do NOT skip any test scripts
  - Do NOT modify test scripts to make them pass
  - Do NOT count GATED_EXPECTED or ENV_MISSING as failures

  **Recommended Agent Profile**:
  - **Category**: `deep`
    - Reason: Full suite execution with analysis and comparison reporting
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: NO — sequential full suite run
  - **Parallel Group**: Wave 1 (after W1.1, W1.2)
  - **Blocks**: W2.1, W2.2, W2.3, W3.1
  - **Blocked By**: T0.1, T0.2, T0.3, T0.4, T0.5 (ALL Wave 0 tasks)

  **References**:

  **Pattern References**:
  - `.sisyphus/reports/vps-test-results-2026-05-15.md` — Phase 1 baseline report (39 PASS, 7 FAIL, 3 SKIP)
  - `/tmp/vps-test-results-full.txt` (on VPS) — Previous raw test output

  **WHY Each Reference Matters**:
  - Phase 1 baseline: Must compare against this to measure improvement
  - Previous raw output: Shows the exact format and command used for the previous run

  **Acceptance Criteria**:

  **QA Scenarios (MANDATORY):**

  ```
  Scenario: Full test suite produces results file
    Tool: Bash (ssh_vps)
    Preconditions: All Wave 0 tasks complete, bridge healthy on HTTPS
    Steps:
      1. ssh_vps "cd /opt/armorclaw && for f in tests/test-*.sh; do echo '=== \$f ==='; bash \$f 2>&1 | tail -5; done" > /tmp/pre-beato-w13-results.txt
      2. Assert file is non-empty and contains output from >=40 scripts
    Expected Result: Complete test output captured for all 49 scripts
    Failure Indicators: Empty file, fewer than 40 scripts executed
    Evidence: .sisyphus/evidence/pre-beato-w13/full-suite.txt

  Scenario: PASS rate improved over Phase 1 baseline
    Tool: Bash
    Preconditions: Full suite results captured
    Steps:
      1. Parse results: count PASS, FAIL, SKIP across all scripts
      2. Compare against baseline: 39 PASS, 7 FAIL, 3 SKIP
      3. Assert PASS count >= 39 (no regression)
      4. Assert FAIL count < 7 (improvement)
    Expected Result: PASS count >= 39, FAIL count < 7
    Failure Indicators: PASS decreased, FAIL increased
    Evidence: .sisyphus/evidence/pre-beato-w13/comparison-report.md
  ```

  **Commit**: YES
  - Message: `test(validation): Full Pre-BEATO suite re-run with results`
  - Files: `.sisyphus/reports/pre-beato-w13-results.md`
  - Pre-commit: N/A (report only)

- [x] W2.1. **Bridge Restart Resilience Tests** — `unspecified-high`

  **What to do**:
  - Test that bridge state survives restart (config, Matrix connection, agent state)
  - Verify bridge health returns to OK within 30 seconds of restart
  - Verify Matrix reconnection after bridge restart
  - Test rapid restart (restart twice within 10 seconds) — bridge should stabilize
  - Use updated `restart_bridge.sh` from T0.4 for the restart operations

  **Must NOT do**:
  - Do NOT modify production bridge restart logic
  - Do NOT add persistence mechanisms
  - Do NOT leave bridge in stopped state after tests

  **Recommended Agent Profile**:
  - **Category**: `unspecified-high`
    - Reason: State verification across restarts, timing-sensitive operations
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES (with W2.2, W2.3)
  - **Parallel Group**: Wave 2
  - **Blocks**: W3.1
  - **Blocked By**: T0.4 (restart_bridge.sh HTTPS fix), W1.2 (WebSocket), W1.3 (baseline results)

  **References**:

  **Pattern References**:
  - `tests/lib/restart_bridge.sh` — The updated restart helper with HTTPS support
  - `tests/test-trust-layer.sh` — Shows pattern for testing bridge state persistence

  **WHY Each Reference Matters**:
  - `restart_bridge.sh`: The tool under test — must use the HTTPS-aware version from T0.4
  - `test-trust-layer.sh`: Shows how to verify bridge state before and after operations

  **Acceptance Criteria**:

  **QA Scenarios (MANDATORY):**

  ```
  Scenario: Bridge health returns within 30s after restart
    Tool: Bash (ssh_vps)
    Preconditions: Bridge running, restart_bridge.sh updated
    Steps:
      1. Record pre-restart state: ssh_vps "curl -ksS https://localhost:8080/health"
      2. ssh_vps "docker restart armorclaw"
      3. Poll health every 2s for up to 30s
      4. Assert health returns 200 within 30s
    Expected Result: Bridge healthy within 30 seconds of restart
    Failure Indicators: Health check fails after 30s, bridge crash loop
    Evidence: .sisyphus/evidence/pre-beato-w21/restart-resilience.txt

  Scenario: Matrix reconnection after bridge restart
    Tool: Bash (ssh_vps)
    Preconditions: Bridge connected to Matrix (T0.1 complete)
    Steps:
      1. Check pre-restart Matrix status via RPC status method
      2. Restart bridge
      3. Wait for health OK
      4. Check post-restart Matrix status via RPC status method
      5. Assert matrix.status == "logged_in"
    Expected Result: Bridge reconnects to Matrix after restart
    Failure Indicators: "not_configured", "login_failed" after restart
    Evidence: .sisyphus/evidence/pre-beato-w21/matrix-reconnect.txt
  ```

  **Commit**: YES
  - Message: `test(resilience): Bridge restart + Studio/Secretary/Browser E2E`
  - Files: New evidence files
  - Pre-commit: N/A (evidence capture)

- [x] W2.2. **Studio/Secretary E2E via Matrix** — `deep`

  **What to do**:
  - Test Secretary workflow creation via Matrix command (`!agent create`)
  - Test Secretary step execution and progress streaming
  - Verify events appear in Matrix room during execution
  - Test blocker resolution via Matrix (user sends approval)
  - Verify Secretary state persists across bridge restart (W2.1 dependency)

  **Must NOT do**:
  - Do NOT add new Secretary features or Matrix commands
  - Do NOT modify production Secretary orchestration code
  - Do NOT test voice/e2ee/keystore features (feature-gated, out of scope)

  **Recommended Agent Profile**:
  - **Category**: `deep`
    - Reason: Multi-step E2E workflow across Matrix + Bridge + Secretary
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES (with W2.1, W2.3)
  - **Parallel Group**: Wave 2
  - **Blocks**: W3.1
  - **Blocked By**: W1.1 (Matrix coverage), W1.3 (baseline)

  **References**:

  **Pattern References**:
  - `tests/test-secretary-workflow-core.sh` — Existing Secretary workflow test (7 scenarios)
  - `tests/test-cross-workflow-email.sh` — Cross-subsystem Secretary + Email pattern

  **WHY Each Reference Matters**:
  - `test-secretary-workflow-core.sh`: Has the Secretary state machine tests — extend these for Matrix E2E
  - `test-cross-workflow-email.sh`: Shows cross-subsystem test pattern to follow

  **Acceptance Criteria**:

  **QA Scenarios (MANDATORY):**

  ```
  Scenario: Secretary workflow created via Matrix command
    Tool: Bash (ssh_vps)
    Preconditions: Bridge connected to Matrix, user has Matrix access token
    Steps:
      1. Create Matrix room via Conduit API
      2. Invite bridge user to room
      3. Send "!agent create name=test-agent" to room
      4. Poll room for bridge response (m.notice message)
      5. Assert bridge acknowledges agent creation
    Expected Result: Bridge creates agent and responds in Matrix room
    Failure Indicators: No response within 10s, error message
    Evidence: .sisyphus/evidence/pre-beato-w22/secretary-e2e-create.txt

  Scenario: Secretary progress streamed to Matrix room
    Tool: Bash (ssh_vps)
    Preconditions: Agent created, workflow running
    Steps:
      1. Trigger workflow execution via Matrix command
      2. Poll Matrix room messages for 30 seconds
      3. Assert progress events (STEP, PROGRESS, etc.) appear as m.notice
    Expected Result: Progress messages appear in Matrix room during execution
    Failure Indicators: No messages received, only error messages
    Evidence: .sisyphus/evidence/pre-beato-w22/secretary-progress.txt
  ```

  **Commit**: YES (groups with W2)
  - Message: `test(resilience): Bridge restart + Studio/Secretary/Browser E2E`
  - Files: Evidence files
  - Pre-commit: N/A

- [x] W2.3. **Browser/Email Pipeline E2E** — `unspecified-high`

  **What to do**:
  - Test browser automation pipeline (Jetski sidecar status, CDP proxy)
  - Test email pipeline RPC boundary
  - Verify Trust Layer integration with browser operations
  - These are "B" and "E" from BEATO — initial validation, not full coverage

  **Must NOT do**:
  - Do NOT test audio/text/office (A/T/O) features in this wave
  - Do NOT modify browser service or email pipeline production code
  - Do NOT add new browser skills or email endpoints

  **Recommended Agent Profile**:
  - **Category**: `unspecified-high`
    - Reason: Multi-component E2E testing across browser and email subsystems
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES (with W2.1, W2.2)
  - **Parallel Group**: Wave 2
  - **Blocks**: W3.1
  - **Blocked By**: W1.3 (baseline results)

  **References**:

  **Pattern References**:
  - `tests/test-email-pipeline.sh` — Existing email pipeline test (7 scenarios)
  - `tests/test-cross-browser-trust.sh` — Browser + Trust Layer cross-test pattern
  - `tests/test-jetski-sidecar.sh` — Jetski sidecar test patterns

  **WHY Each Reference Matters**:
  - `test-email-pipeline.sh`: Shows email RPC boundary testing pattern
  - `test-cross-browser-trust.sh`: Browser + Trust integration pattern
  - `test-jetski-sidecar.sh`: Jetski sidecar lifecycle testing

  **Acceptance Criteria**:

  **QA Scenarios (MANDATORY):**

  ```
  Scenario: Browser sidecar status query succeeds
    Tool: Bash (ssh_vps)
    Preconditions: Bridge running with browser service enabled
    Steps:
      1. ssh_vps "curl -ksS https://localhost:8080/api -H 'Content-Type: application/json' -H 'Authorization: Bearer $ADMIN_TOKEN' -d '{\"jsonrpc\":\"2.0\",\"id\":1,\"method\":\"jetski.status\"}'"
      2. Assert response is valid JSON with "result" or expected error for "not_running"
    Expected Result: Jetski status returns structured response (running or gracefully not running)
    Failure Indicators: "method not found", connection error, malformed JSON
    Evidence: .sisyphus/evidence/pre-beato-w23/browser-status.txt

  Scenario: Email pipeline RPC boundary test
    Tool: Bash (ssh_vps)
    Preconditions: Bridge running with email config
    Steps:
      1. ssh_vps "curl -ksS https://localhost:8080/api -H 'Content-Type: application/json' -H 'Authorization: Bearer $ADMIN_TOKEN' -d '{\"jsonrpc\":\"2.0\",\"id\":1,\"method\":\"email.status\"}'"
      2. Assert response is valid JSON (success or expected error)
    Expected Result: Email status RPC returns structured response
    Failure Indicators: "method not found", panic, connection error
    Evidence: .sisyphus/evidence/pre-beato-w23/email-rpc.txt
  ```

  **Commit**: YES (groups with W2)
  - Message: `test(resilience): Bridge restart + Studio/Secretary/Browser E2E`
  - Files: Evidence files
  - Pre-commit: N/A

- [x] W3.1. **Pre-BEATO Gate Assessment** — `quick`

  **What to do**:
  - Evaluate all Pre-BEATO exit gates against current test results
  - Gate checklist:
    - >=80% PASS across all 49 test scripts
    - <15% SKIP
    - `matrix.status` shows `logged_in`
    - WebSocket connection stable for >=30 seconds
    - Studio/Secretary E2E workflow completes via Matrix
    - Bridge restart test passing
    - All 5 high-risk Go packages covered by tests
    - Zero infrastructure-caused failures
  - Generate go/no-go recommendation
  - If gates NOT met, list specific gaps with remediation steps

  **Must NOT do**:
  - Do NOT inflate PASS counts
  - Do NOT downgrade FAIL to SKIP without justification
  - Do NOT proceed to BEATO if gates are not met

  **Recommended Agent Profile**:
  - **Category**: `quick`
    - Reason: Gate evaluation against existing data
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: NO — depends on all Wave 2 results
  - **Parallel Group**: Wave 3
  - **Blocks**: W3.2
  - **Blocked By**: W2.1, W2.2, W2.3

  **References**:

  **Pattern References**:
  - `.sisyphus/evidence/pre-beato-w13/comparison-report.md` — Full suite results from W1.3
  - `.sisyphus/reports/vps-test-results-2026-05-15.md` — Phase 1 baseline for comparison

  **WHY Each Reference Matters**:
  - W1.3 comparison report: The data source for gate evaluation
  - Phase 1 baseline: Shows starting point for measuring improvement

  **Acceptance Criteria**:

  **QA Scenarios (MANDATORY):**

  ```
  Scenario: Pre-BEATO gate report generated
    Tool: Bash
    Preconditions: All Wave 2 tasks complete, evidence available
    Steps:
      1. Read evidence files from W1.3, W2.1, W2.2, W2.3
      2. Count PASS/FAIL/SKIP across all scripts
      3. Evaluate each gate criterion
      4. Generate report to .sisyphus/reports/pre-beato-gate-assessment.md
    Expected Result: Gate assessment report with pass/fail per criterion and overall go/no-go
    Failure Indicators: Missing evidence, incomplete gate evaluation
    Evidence: .sisyphus/reports/pre-beato-gate-assessment.md
  ```

  **Commit**: YES
  - Message: `docs(reports): Pre-BEATO gate assessment + coverage baseline`
  - Files: `.sisyphus/reports/pre-beato-gate-assessment.md`
  - Pre-commit: N/A (report only)

- [x] W3.2. **Coverage Baseline Report + CI Gate Config** — `writing`

  **What to do**:
  - Generate comprehensive coverage baseline report from all test results
  - Document which BEATO features (B/E/A/T/O) are covered and at what level
  - Create CI gate configuration recommendations
  - Document known gaps for future BEATO phases
  - Save to `.sisyphus/reports/pre-beato-coverage-baseline.md`

  **Must NOT do**:
  - Do NOT create actual CI config files (that's a future task)
  - Do NOT commit coverage data as passing if it isn't

  **Recommended Agent Profile**:
  - **Category**: `writing`
    - Reason: Documentation and report generation
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: NO — depends on W3.1 gate assessment
  - **Parallel Group**: Wave 3
  - **Blocks**: F1-F4
  - **Blocked By**: W3.1

  **References**:

  **Pattern References**:
  - `.sisyphus/reports/pre-beato-gate-assessment.md` — Gate assessment from W3.1
  - `.sisyphus/reports/test-harness-stabilization-report.md` — Phase 1 report format to follow

  **WHY Each Reference Matters**:
  - Gate assessment: Data source for coverage baseline
  - Phase 1 report: Format template for consistent reporting

  **Acceptance Criteria**:

  **QA Scenarios (MANDATORY):**

  ```
  Scenario: Coverage baseline report is complete
    Tool: Bash
    Preconditions: W3.1 gate assessment complete
    Steps:
      1. Read .sisyphus/reports/pre-beato-coverage-baseline.md
      2. Assert file contains sections: BEATO coverage matrix, known gaps, CI gate recommendations
      3. Assert each BEATO letter has coverage level (none/minimal/partial/full)
    Expected Result: Complete coverage baseline report with actionable recommendations
    Failure Indicators: Missing sections, incomplete BEATO matrix
    Evidence: .sisyphus/reports/pre-beato-coverage-baseline.md
  ```

  **Commit**: YES (groups with W3)
  - Message: `docs(reports): Pre-BEATO gate assessment + coverage baseline`
  - Files: `.sisyphus/reports/pre-beato-coverage-baseline.md`
  - Pre-commit: N/A

---

## Final Verification Wave (MANDATORY — after ALL implementation tasks)

> 4 review agents run in PARALLEL. ALL must APPROVE. Present consolidated results to user and get explicit "okay" before completing.
>
> **Do NOT auto-proceed after verification. Wait for user's explicit approval before marking work complete.**
> **Never mark F1-F4 as checked before getting user's okay.**

- [x] F1. **Plan Compliance Audit** — `oracle`
  Read the plan end-to-end. For each "Must Have": verify implementation exists (read file, curl endpoint, run command). For each "Must NOT Have": search codebase for forbidden patterns — reject with file:line if found. Check evidence files exist in .sisyphus/evidence/. Compare deliverables against plan.
  Output: `Must Have [N/N] | Must NOT Have [N/N] | Tasks [N/N] | VERDICT: APPROVE/REJECT`

- [x] F2. **Code Quality Review** — `unspecified-high`
  Review all changed files for: `as any`/type assertions, empty catches, console.log in prod, commented-out code, unused imports. Check AI slop: excessive comments, over-abstraction, generic names. Run `bash -n` on all modified shell scripts.
  Output: `Scripts [N clean/N issues] | Pattern Compliance [PASS/FAIL] | VERDICT`

- [x] F3. **Real Manual QA** — `unspecified-high`
  SSH to VPS. Execute EVERY QA scenario from EVERY task — follow exact steps, capture evidence. Test cross-task integration (Matrix + HTTPS + tools all working together). Test edge cases: bridge down, invalid token, expired cert. Save to `.sisyphus/evidence/final-qa/`.
  Output: `Scenarios [N/N pass] | Integration [N/N] | Edge Cases [N tested] | VERDICT`

- [x] F4. **Scope Fidelity Check** — `deep`
  For each task: read "What to do", read actual diff (git log/diff). Verify 1:1 — everything in spec was built (no missing), nothing beyond spec was built (no creep). Check "Must NOT do" compliance. Detect cross-task contamination. Flag unaccounted changes.
  Output: `Tasks [N/N compliant] | Contamination [CLEAN/N issues] | Unaccounted [CLEAN/N files] | VERDICT`

---

## Commit Strategy

| Wave | Commit Message | Files |
|------|---------------|-------|
| 0 | `fix(tests): HTTPS transport for shared libraries + VPS infra fixes` | `tests/lib/transport.sh`, `tests/lib/load_env.sh`, `tests/lib/restart_bridge.sh`, `tests/test-yara-smoke.sh` |
| 1 | `test(matrix): Matrix CLI coverage + WebSocket reconnection tests` | New/modified test scripts |
| 2 | `test(resilience): Bridge restart + Studio/Secretary/Browser E2E` | New/modified test scripts |
| 3 | `docs(reports): Pre-BEATO gate assessment + coverage baseline` | Report files in `.sisyphus/reports/` |

---

## Success Criteria

### Verification Commands
```bash
# Transport library detects HTTPS correctly
source tests/lib/transport.sh && detect_transport
# Expected: mode=http (HTTPS detected and working)

# Bridge health via HTTPS
ssh_vps "curl -ksS https://localhost:8080/health"
# Expected: {"status":"ok","version":"4.6.0"}

# Matrix bridge user login
ssh_vps "curl -sS http://localhost:6167/_matrix/client/v3/login -d '{...}'"
# Expected: 200 with access_token

# Full test suite
ssh_vps "cd /opt/armorclaw && for f in tests/test-*.sh; do bash \$f 2>&1; done"
# Expected: >=80% PASS, <15% SKIP, 0 infra FAIL
```

### Pre-BEATO Exit Gates
- [ ] >=80% PASS across all 49 test scripts
- [ ] <15% SKIP
- [x] `matrix.status` shows `logged_in`
- [x] WebSocket connection stable for >=30 seconds
- [x] Studio/Secretary E2E workflow completes via Matrix
- [x] Bridge restart test passes (restart + readiness verified)
- [x] All 5 high-risk Go packages covered by tests
- [ ] Zero infrastructure-caused failures

### Final Checklist
- [x] All "Must Have" present
- [x] All "Must NOT Have" absent
- [ ] All tests pass Pre-BEATO gates
- [x] Evidence files captured for every task
