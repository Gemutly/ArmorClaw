# ArmorClaw Two-Plane Test Harness

## TL;DR

> **Quick Summary**: Wire DeviceStore/InviteStore in main.go (minimal wiring fix — store init + config fields + possible import), then build a two-plane test suite: **Plane A** (local governance RPCs via socat, no external service deps) and **Plane B** (VPS integration via SSH + curl for health/auth + matrix-commander for Matrix messaging).
> 
> **Deliverables**:
> - Minimal wiring fix in `bridge/cmd/bridge/main.go`
> - `tests/test-governance-rpc.sh` — self-contained local harness (5 test groups, socat, no external services)
> - `tests/test-vps-smoke.sh` — VPS integration tests (SSH + curl + auth)
> - `tests/test-matrix-plane.sh` — Matrix plane tests (matrix-commander)
> 
> **Estimated Effort**: Large
> **Parallel Execution**: YES - 2 waves
> **Critical Path**: T1 (wire stores) → T2+T3+T4 (all parallel) → F1-F4

---

## Context

### Original Request
ArmorChat Android client testing is not going well. User needs ArmorChat-independent testing of all ArmorClaw functions: governance RPCs, end-to-end flows, browser automation, and auth/token flow.

> **Milestone scope note**: This plan covers governance RPCs, auth/token flow, and Matrix transport first. Browser automation and full workflow/HITL end-to-end testing are intentionally deferred to a later harness milestone.

### Interview Summary
**Key Discussions**:
- Two-plane architecture: Matrix plane (sync, messaging, media, assistant) + Bridge admin plane (governance RPCs, auth, health)
- Local governance testing (socat, no deps) + VPS integration testing (SSH, curl, matrix-commander)
- `matrix-commander` as primary Matrix CLI — install via Docker or pip (requires `libolm`)
- Auth testing via HTTP `/api` endpoint (curl with `Authorization: Bearer` header)
- Environment variables for all secrets — never hardcode passwords
- No fake prerequisites URLs, no guessed Docker image names

**Research Findings**:
- **Critical**: `main.go` line 2501 creates `rpc.New()` but does NOT pass `DeviceStore` or `InviteStore` — nil stores → all governance handlers return "not configured"
- Fix is minimal: initialize both stores from `ks.GetDB()` and pass them into `rpc.Config`
- Unix socket bypasses auth; HTTP `/api` enforces auth — both paths must be tested
- Exact error codes from source: `-32602` (InvalidParams), `-32603` (InternalError), `-32000` (NotFound), `-32601` (MethodNotFound)
- `invite.create` expiration whitelist: `"1h"`, `"6h"`, `"1d"`, `"7d"`, `"30d"`, `"never"` only
- `device.reject` `reason` field is optional — only `device_id` and `rejected_by` are validated
- Bridge health endpoint: `http://localhost:8080/health` (JSON, no auth required)
- Bridge RPC endpoint: `http://localhost:8080/api` (JSON-RPC 2.0, auth required for governance methods)
- `matrix-commander` Docker image: `matrixcommander/matrix-commander` — verified on Docker Hub
- `matrix-commander` pip install requires `libolm-dev` on Debian/Ubuntu

### Metis Review
**Identified Gaps** (all addressed):
- Store wiring gap → T1
- Two invite systems → targeting SQL-backed InviteStore only
- Auth requires HTTP → T3 covers curl-based auth testing
- Device creation via provisioning (not RPC) → T2 tests list/get with empty/not-found

---

## Work Objectives

### Core Objective
Enable comprehensive ArmorChat-independent testing across two planes: Bridge admin (governance RPCs, auth, health) and Matrix (messaging, sync, media, assistant round-trips).

### Concrete Deliverables
- `bridge/cmd/bridge/main.go` — minimal store wiring fix
- `tests/test-governance-rpc.sh` — local governance test (5 test groups, socat)
- `tests/test-vps-smoke.sh` — VPS integration test (SSH + curl + auth; auto-sources `.env` for VPS_IP/VPS_USER/BRIDGE_PORT)
- `tests/test-matrix-plane.sh` — Matrix plane test (matrix-commander; auto-sources `.env` and constructs HOMESERVER from VPS_IP+MATRIX_PORT)

### Definition of Done
- [ ] `bash tests/test-governance-rpc.sh` exits 0, all tests passing, no VPS/Matrix/ArmorChat deps
- [ ] `ADMIN_TOKEN=aat_xxx bash tests/test-vps-smoke.sh` exits 0 (VPS_IP/VPS_USER/BRIDGE_PORT sourced from `.env`), tests health + auth (asserted via JSON-RPC error body) + governance
- [ ] `MATRIX_USER=... MATRIX_PASSWORD=... ROOM_ID=... bash tests/test-matrix-plane.sh` exits 0 (VPS_IP/MATRIX_PORT sourced from `.env`, HOMESERVER auto-constructed), tests sync + messaging + media

### Must Have
- All 8 governance methods callable and returning valid JSON-RPC responses (device.list, device.get, device.approve, device.reject, invite.create, invite.list, invite.revoke, invite.validate — all exist as handlers and are cheap to verify)
- Parameter validation tests for all required fields
- Auth rejection test (no token → JSON-RPC error with `-32001 "unauthorized"`, not HTTP status code — assert the JSON-RPC error body)
- Auth acceptance test (valid `aat_` token → governance methods return `"result"` not `"error"`)
- Bridge health endpoint returns valid JSON
- Matrix sync succeeds
- Message send succeeds
- Clear PASS/FAIL output per test
- Exit code 0 = all pass, 1 = any fail
- Environment variables for all secrets, never hardcoded

### Explicitly Out of Scope (Confirmed)
- Audit log verification — intentionally deferred to separate verification lane
- WebSocket event emission testing
- Browser automation testing
- Full workflow/HITL path testing
- ArmorChat testing
- Manual prompts or interactive test modes

### Must NOT Have (Guardrails)
- NO ArmorChat dependency
- NO hardcoded passwords or tokens in scripts
- NO fake or unverified prerequisite URLs
- NO guessed Docker image names
- NO interactive prompts in test scripts (CI-safe)
- NO log-grep as the only success criterion (assert actual API responses)
- NO modifications to governance handler logic (only wiring fix)
- NO WebSocket event testing
- NO audit log verification (deferred — see Explicitly Out of Scope)
- NO changes to existing `tests/test-rpc-methods.sh`
- NO VPS, Matrix, or ArmorChat dependency for the local governance test (`test-governance-rpc.sh`)

---

## Verification Strategy (MANDATORY)

> **ZERO HUMAN INTERVENTION** - ALL verification is agent-executed.

### Test Decision
- **Infrastructure exists**: YES (bash + socat for local, SSH + curl for VPS, matrix-commander for Matrix)
- **Automated tests**: THIS IS THE TEST
- **Framework**: bash scripts with socat + curl + jq + matrix-commander

### QA Policy
Every task includes agent-executed QA scenarios.
Evidence saved to `.sisyphus/evidence/task-{N}-{scenario-slug}.{ext}`.

- **Local Governance**: Bash (socat) → send JSON-RPC, parse response, assert
- **VPS Health/Auth**: Bash (SSH + curl) → check endpoints, assert status codes + JSON
- **Matrix Plane**: Bash (matrix-commander Docker) → sync, send, poll, assert

---

## Execution Strategy

### Parallel Execution Waves

```
Wave 1 (Start Immediately — prerequisite):
└── Task 1: Wire DeviceStore + InviteStore in main.go [quick]

Wave 2 (After Wave 1 — parallel test scripts):
├── Task 2: Local governance test harness (socat) [unspecified-high]
├── Task 3: VPS integration test harness (SSH + curl) [unspecified-high]
└── Task 4: Matrix plane test harness (matrix-commander) [unspecified-high]

Wave FINAL (After ALL tasks — 4 parallel reviews):
├── Task F1: Plan compliance audit (oracle)
├── Task F2: Code quality review (unspecified-high)
├── Task F3: Automated execution audit (unspecified-high)
└── Task F4: Scope fidelity check (deep)
-> Present results -> Get explicit user okay

Critical Path: T1 → T2/T3/T4 (parallel) → F1-F4 → user okay
Max Concurrent: 3 (Wave 2)
```

### Dependency Matrix

| Task | Depends On | Blocks | Wave |
|------|-----------|--------|------|
| T1   | -         | T2, T3, T4 | 1 |
| T2   | T1        | F1-F4  | 2    |
| T3   | T1        | F1-F4  | 2    |
| T4   | T1        | F1-F4  | 2    |
| F1-F4| T2,T3,T4  | user   | FINAL|

### Agent Dispatch Summary

- **Wave 1**: **1** — T1 → `quick`
- **Wave 2**: **3** — T2 → `unspecified-high`, T3 → `unspecified-high`, T4 → `unspecified-high`
- **FINAL**: **4** — F1 → `oracle`, F2 → `unspecified-high`, F3 → `unspecified-high`, F4 → `deep`

---

## TODOs

- [x] 1. Wire DeviceStore + InviteStore in main.go

  **What to do**:
  - In `bridge/cmd/bridge/main.go`, locate the `hardeningStore` initialization (search for `trust.NewKeystoreHardeningStore(ks.GetDB())` — currently near line 2495). **After** that line, add:
    ```go
    // Initialize governance stores (shared keystore DB)
    deviceStore, err := trust.NewDeviceStore(ks.GetDB())
    if err != nil {
        log.Fatalf("Failed to initialize device store: %v", err)
    }
    inviteStore, err := invite.NewInviteStore(ks.GetDB())
    if err != nil {
        log.Fatalf("Failed to initialize invite store: %v", err)
    }
    ```
  - Locate the `rpc.Config{}` struct literal (search for `rpc.New(` or `Config{` near the end of `main()` — currently around line 2520). Add these fields alongside the existing `HardeningStore` field:
    ```go
    DeviceStore:     deviceStore,
    InviteStore:     inviteStore,
    ```
  - Add import for `"github.com/armorclaw/bridge/pkg/invite"` if not already present

  **Must NOT do**:
  - Do NOT modify handler logic, store logic, or config structures
  - Do NOT add new config file fields
  - Do NOT change the existing `hardeningStore` line

  **Recommended Agent Profile**:
  - **Category**: `quick`
  - **Skills**: `[]`

  **Parallelization**:
  - **Can Run In Parallel**: NO
  - **Blocks**: T2, T3, T4
  - **Blocked By**: None

  **References**:
  - `bridge/cmd/bridge/main.go` — Search for `trust.NewKeystoreHardeningStore(ks.GetDB())` (store init pattern) and `rpc.New(` / `Config{` (rpc.Config block near end of main())
  - `bridge/pkg/trust/device_store.go:35` — `NewDeviceStore(db *sql.DB) (*DeviceStore, error)`
  - `bridge/pkg/invite/store.go:42` — `NewInviteStore(db *sql.DB) (*InviteStore, error)`
  - `bridge/pkg/rpc/server.go:187-188` — Config fields: `DeviceStore`, `InviteStore`

  **Acceptance Criteria**:

  **QA Scenarios (MANDATORY):**

  ```
  Scenario: Bridge compiles with store wiring
    Tool: Bash
    Steps:
      1. cd bridge && go build -o /dev/null ./cmd/bridge
    Expected Result: Exit code 0, no compilation errors
    Evidence: .sisyphus/evidence/task-1-compile.txt

  Scenario: Stores initialize when bridge starts
    Tool: Bash (socat)
    Steps:
      1. Build bridge, create temp config, start bridge
      2. Send device.list via socat
      3. Assert response contains "result" NOT "device store not configured"
    Expected Result: `{"jsonrpc":"2.0","id":1,"result":[]}`
    Evidence: .sisyphus/evidence/task-1-store-init.txt
  ```

  **Commit**: YES
  - Message: `fix(bridge): wire DeviceStore and InviteStore in RPC server`
  - Files: `bridge/cmd/bridge/main.go`

- [x] 2. Create local governance test harness (Plane A — socat)

  **What to do**:
  - Create `tests/test-governance-rpc.sh` — self-contained, no VPS/Matrix/ArmorChat deps
  - Follow EXACT pattern from `tests/test-rpc-methods.sh`: cleanup trap, bridge build, temp config, socket wait, `rpc_call()`, `test_method()`
  - Add `test_method_with_params()` for param-based assertions
  - ~40 tests across 5 groups (count is approximate; what matters is comprehensive grouped coverage):

  **Group 1: Method Registration (8 tests)** — all 8 governance methods return valid JSON-RPC:
  - `device.list` → returns result (empty array `[]`)
  - `device.get` → returns error (no device_id → validation error)
  - `device.approve` → returns error (param validation only — no device to approve)
  - `device.reject` → returns error (param validation only — no device to reject)
  - `invite.list` → returns result (empty array `[]`)
  - `invite.create` → returns result with invite record
  - `invite.revoke` → returns error (nothing to revoke initially)
  - `invite.validate` → returns error (no valid code yet)

  **Group 2: Happy Path Flow for Invites (sequential, ~8 tests)**:
  - Create invite via `invite.create` with `{"role":"admin","expiration":"1d","max_uses":5,"created_by":"test-admin"}` → capture invite ID and code
  - Validate invite via `invite.validate` with `{"code":"<captured_code>"}` → assert status "active"
  - List invites via `invite.list` → assert array has length ≥ 1
  - Revoke invite via `invite.revoke` with `{"invite_id":"<captured_id>","revoked_by":"test-admin"}` → assert success
  - Validate revoked invite → assert "revoked" or error indicating revoked status

  **Device happy-path NOTE**: Device creation happens via provisioning flow, not RPC. The local harness has no way to seed a pending device into the temp store. Therefore:
  - `device.approve` and `device.reject` happy-path tests are **never run locally** — parameter validation only
  - Full device lifecycle testing happens exclusively in T3 (VPS smoke) where provisioning creates real devices and `PENDING_DEVICE_ID` gating applies

  **Group 3: Parameter Validation (~16 tests)** — every required field missing → `-32602`:
  - `device.get` with no `device_id` → `-32602 "device_id is required"`
  - `device.get` with empty `device_id` → `-32602 "device_id is required"`
  - `device.get` with non-existent `device_id` → `-32000 "device not found"`
  - `device.approve` with no params → `-32602 "device_id is required"`
  - `device.approve` with missing `approved_by` → `-32602 "approved_by is required"`
  - `device.reject` with no params → `-32602 "device_id is required"`
  - `device.reject` with missing `rejected_by` → `-32602 "rejected_by is required"`
  - `device.reject` WITHOUT `reason` → should NOT error (reason is optional — verify this)
  - `invite.create` with no `role` → `-32602 "role is required"`
  - `invite.create` with invalid `role` → `-32602 "invalid role: superuser"`
  - `invite.create` with no `expiration` → `-32602 "expiration is required"`
  - `invite.create` with invalid `expiration` → `-32602 "invalid expiration: ..."`
  - `invite.create` with no `created_by` → `-32602 "created_by is required"`
  - `invite.revoke` with no `invite_id` → `-32602 "invite_id is required"`
  - `invite.validate` with no `code` → `-32602 "code is required"`
  - `invite.validate` with invalid `code` → not-found error

  **Group 4: Protocol Errors (~4 tests)**:
  - Non-existent method `device.delete` → `-32601` (Method not found)
  - Invalid JSON-RPC version `{"jsonrpc":"1.0",...}` → `-32600` (Invalid request)
  - Missing method field → `-32600` (Invalid request)
  - Invalid JSON body → `-32700` (Parse error)

  **Group 5: Invite Lifecycle Edge Cases (~4 tests)**:
  - `invite.create` with `max_uses: 0` → success (unlimited uses)
  - `invite.create` with `expiration: "never"` → success
  - `invite.create` with `welcome_message: "Welcome!"` → success
  - `invite.validate` with revoked invite code → error indicating revoked status

  **Summary Block (MANDATORY)**: After all tests complete, print a summary block to stdout:
  ```
  =========================================
  GOVERNANCE RPC TEST SUMMARY
  =========================================
  Total:  N  |  Passed:  N  |  Failed:  N
  Groups: Registration(8/8) Invites(8/8) Validation(16/16) Protocol(4/4) EdgeCases(4/4)
  =========================================
  FAILURES (if any):
  - [test-name]: [exact command] → [actual response snippet]
  =========================================
  ```
  This summary IS the audit report for this plane. It is captured in the evidence file by the QA runner. No separate report task needed.

  **Must NOT do**:
  - Do NOT require VPS, Matrix, ArmorChat, or any external service
  - Do NOT modify any Go source files
  - Do NOT create a test framework — plain bash functions only
  - Do NOT test device.approve/reject happy-path locally (no seeding mechanism)
  - Do NOT modify the existing `tests/test-rpc-methods.sh`

  **Recommended Agent Profile**:
  - **Category**: `unspecified-high`
  - **Skills**: `[]`

  **Parallelization**:
  - **Can Run In Parallel**: YES (with T3 and T4)
  - **Blocks**: F1-F4
  - **Blocked By**: T1

  **References**:
  - `tests/test-rpc-methods.sh:1-126` — Complete pattern to follow EXACTLY
  - `bridge/pkg/rpc/governance_types.go` — Request/response structs with field names
  - `bridge/pkg/rpc/device_handlers.go` — Parameter validation and error messages
  - `bridge/pkg/rpc/invite_handlers.go` — Parameter validation and error messages
  - `bridge/pkg/invite/store.go:230-260` — `ParseExpiration()` whitelist
  - `bridge/pkg/rpc/device_handlers_test.go` — Expected error messages
  - `bridge/pkg/rpc/invite_handlers_test.go` — Expected error messages

  **Acceptance Criteria**:

  **QA Scenarios (MANDATORY):**

  ```
  Scenario: Full run exits 0
    Tool: Bash
    Steps:
      1. bash tests/test-governance-rpc.sh
      2. Check exit code
    Expected Result: Exit 0, all test groups pass
    Evidence: .sisyphus/evidence/task-2-full-run.txt

  Scenario: Parameter validation works
    Tool: Bash (socat)
    Steps:
      1. device.get with {} → assert "-32602"
      2. invite.create with {} → assert "-32602"
    Expected Result: InvalidParams errors
    Evidence: .sisyphus/evidence/task-2-param-validation.txt

  Scenario: Cleanup is thorough
    Tool: Bash
    Steps:
      1. After script exits, check no temp files/processes remain
    Expected Result: Clean system
    Evidence: .sisyphus/evidence/task-2-cleanup.txt
  ```

  **Commit**: YES
  - Message: `test(governance): add comprehensive RPC test harness`
  - Files: `tests/test-governance-rpc.sh`

- [x] 3. Create VPS integration test harness (Plane B — SSH + curl)

  **What to do**:
  - Create `tests/test-vps-smoke.sh` — tests against a running ArmorClaw VPS instance
  - **Auto-source `.env` at script start** for VPS connection details:
    ```bash
    set -a
    source .env 2>/dev/null || true
    set +a
    ```
  - Uses environment variables for all config (no hardcoded secrets). VPS_IP, VPS_USER, BRIDGE_PORT come from `.env`; ADMIN_TOKEN and PENDING_DEVICE_ID must be set on CLI:

    ```bash
    : "${VPS_IP:?missing VPS_IP (set in .env or CLI)}"
    : "${VPS_USER:=root}"
    : "${BRIDGE_PORT:=8080}"
    : "${ADMIN_TOKEN:?missing ADMIN_TOKEN (pass via CLI)}"
    : "${CI_MODE:=0}"
    ```

  - Define SSH wrapper:
    ```bash
    ssh_vps() { ssh -o ConnectTimeout=10 -o StrictHostKeyChecking=no "${VPS_USER}@${VPS_IP}" "$@"; }
    ```

  - Define curl wrapper for admin RPC:
    ```bash
    rpc_vps() {
      local method="$1" params="${2:-{}}"
      ssh_vps "curl -fsS -H 'Authorization: Bearer ${ADMIN_TOKEN}' -H 'Content-Type: application/json' -d '{\"jsonrpc\":\"2.0\",\"id\":1,\"method\":\"${method}\",\"params\":${params}}' http://localhost:${BRIDGE_PORT}/api"
    }
    ```

  - Test categories:

  **Category A: Bridge Health (3 tests)**
  - SSH connectivity: `ssh_vps "echo ok"` → exit 0
  - Health endpoint: `ssh_vps "curl -fsS http://localhost:${BRIDGE_PORT}/health | jq .status"` → `"ok"`
  - Bridge process running: `ssh_vps "docker ps --filter name=armorclaw --format '{{.Names}}'"` → non-empty

  **Category B: Auth Enforcement (3 tests)**
  - No auth → JSON-RPC error body: `RESP=$(ssh_vps "curl -s -H 'Content-Type: application/json' -d '{\"jsonrpc\":\"2.0\",\"id\":1,\"method\":\"device.list\"}' http://localhost:${BRIDGE_PORT}/api")` → assert `echo "$RESP" | jq -e '.error.code == -32001'` and `echo "$RESP" | jq -e '.error.message == "unauthorized"'`. HTTP status is informational only (often 200). Do NOT discard body with `-o /dev/null`.
  - Valid auth → succeeds: `rpc_vps "device.list"` → contains `"result"` not `"error"`
  - Invalid token → JSON-RPC error body: send with `Authorization: Bearer invalid-token` → assert `.error.code == -32001` and `.error.message == "unauthorized"` in the captured body

  **Category C: Governance Methods via HTTP (4 tests)**
  - `device.list` with auth → returns `{"jsonrpc":"2.0","id":1,"result":[]}`
  - `invite.list` with auth → returns result array
  - `invite.create` with auth and valid params → returns invite with code
  - `invite.revoke` with auth → returns success or appropriate error

  **Device approve/reject happy-path NOTE**: These tests require a pending device to exist on the VPS. Two options:
  - If `PENDING_DEVICE_ID` env var is set → test `device.approve` and `device.reject` against it (happy-path)
  - If not set → skip device approve/reject happy-path, only test parameter validation (consistent with T2)
  - Document which mode ran in the test output

  **Category D: Discovery Endpoint (1 test, OPTIONAL — does not block pass/fail)**
  - `curl http://localhost:${BRIDGE_PORT}/api/discovery` → returns JSON with version, mode, matrix_homeserver
  - This is a smoke check, not a governance requirement

  **Implementation Notes**:
  - Every test asserts actual API response content, NOT just exit codes
  - `jq` used for JSON parsing — fail clearly if `jq` not installed
  - Print step-by-step output: `[1/N] test name ... ✅`
  - On failure, print the full response body for debugging
  - **Summary Block (MANDATORY)**: After all tests complete, print a summary block to stdout:
    ```
    =========================================
    VPS SMOKE TEST SUMMARY
    =========================================
    Total:  N  |  Passed:  N  |  Failed:  N
    Groups: Health(N/N) Auth(N/N) Governance(N/N) Discovery(N/N)
    =========================================
    FAILURES (if any):
    - [test-name]: [exact command] → [actual response snippet]
    =========================================
    ```
  - Total: ~11 tests

  **Must NOT do**:
  - Do NOT hardcode passwords or tokens
  - Do NOT use interactive prompts (CI-safe)
  - Do NOT use `docker logs | grep` as the only success criterion
  - Do NOT install anything on the VPS
  - Do NOT modify the VPS configuration

  **Recommended Agent Profile**:
  - **Category**: `unspecified-high`
  - **Skills**: `[]`

  **Parallelization**:
  - **Can Run In Parallel**: YES (with T2)
  - **Blocks**: F1-F4
  - **Blocked By**: T1

  **References**:
  - `bridge/pkg/http/server.go:378-414` — handleRPC with auth middleware
  - `bridge/pkg/http/server.go:416-433` — handleHealth endpoint returns `{"status":"ok",...}`
  - `bridge/pkg/http/server.go:435-457` — handleDiscovery endpoint
  - `bridge/pkg/auth/matrix_auth.go` — authenticateAdminToken() for `aat_` prefix tokens
  - `applications/admin-panel/src/services/bridgeApi.ts:154-187` — BridgeAPIClient.rpc() shows exact curl format
  - `docs/reference/rpc-api.md:1-120` — JSON-RPC request/response format

  **Acceptance Criteria**:

  **QA Scenarios (MANDATORY):**

  ```
  Scenario: VPS smoke test runs against real deployment
    Tool: Bash (SSH + curl)
    Preconditions: .env contains VPS_IP/VPS_USER/BRIDGE_PORT; ArmorClaw deployed on VPS
    Steps:
      1. ADMIN_TOKEN=aat_xxx bash tests/test-vps-smoke.sh
      2. Check exit code
    Expected Result: Exit 0, all tests pass
    Evidence: .sisyphus/evidence/task-3-vps-smoke.txt

  Scenario: Auth is enforced on admin RPC (JSON-RPC body assertion)
    Tool: Bash (curl via SSH)
    Steps:
      1. Send device.list without Authorization header → capture full response body
      2. Assert response contains `.error.code == -32001` and `.error.message == "unauthorized"`
      3. Send device.list with valid ADMIN_TOKEN → assert response contains `"result"` not `"error"`
    Expected Result: No-auth returns JSON-RPC error (-32001), valid-auth returns result
    Evidence: .sisyphus/evidence/task-3-auth-test.txt
  ```

  **Commit**: YES
  - Message: `test(vps): add SSH + curl integration smoke test`
  - Files: `tests/test-vps-smoke.sh`

- [x] 4. Create Matrix plane test harness (Plane B — matrix-commander)

  **What to do**:
  - Create `tests/test-matrix-plane.sh` — tests Matrix messaging plane via matrix-commander
  - **Auto-source `.env` at script start** for VPS connection details:
    ```bash
    set -a
    source .env 2>/dev/null || true
    set +a
    ```
  - Uses environment variables. VPS_IP and MATRIX_PORT come from `.env`. HOMESERVER auto-constructed if not set:

    ```bash
    : "${MATRIX_PORT:=6167}"
    : "${HOMESERVER:=https://${VPS_IP}:${MATRIX_PORT}}"
    : "${MATRIX_USER:?missing MATRIX_USER (pass via CLI)}"
    : "${MATRIX_PASSWORD:?missing MATRIX_PASSWORD (pass via CLI)}"
    : "${ROOM_ID:?missing ROOM_ID (pass via CLI)}"
    : "${MATRIX_STORE:=$HOME/.matrix-commander}"
    : "${MC_MODE:=docker}"  # docker | local
    : "${CI_MODE:=0}"
    ```

  - Define matrix-commander wrapper:
    ```bash
    mc() {
      if [[ "$MC_MODE" == "docker" ]]; then
        docker run --rm \
          -v "$MATRIX_STORE:/data" \
          matrixcommander/matrix-commander "$@"
      else
        matrix-commander "$@"
      fi
    }
    ```

  - Prerequisite check at script start: verify `docker` or `matrix-commander` is available

  - Test categories:

  **Category A: Login & Sync (2 tests)**
  - First-time login: `mc --login --homeserver "$HOMESERVER" --user "$MATRIX_USER" --password "$MATRIX_PASSWORD"` → exit 0, credentials saved
  - Sync succeeds: `mc --sync --timeout 30` → exit 0

  **Category B: Messaging (3 tests)**
  - Send message: `mc --room "$ROOM_ID" --message "ARMORCLAW_SMOKE_$(date +%s) [unique-token]"` → exit 0. Use a unique token (timestamp-based) in every message for reliable matching.
  - Verify message landed: use `mc --room "$ROOM_ID" --sync` (or `--tail N` mode) to get recent messages, then `grep` for the unique token. Timeout: 30 seconds (poll every 5s, 6 attempts). Failure = unique token not found after timeout.
  - Send and receive round-trip: send unique message, poll for it using the same token-matching strategy, assert it appears within 30s

  **Category C: File Upload (1 test)**
  - Upload file: create temp file, `mc --room "$ROOM_ID" --file /tmp/armorclaw-smoke.txt` → exit 0

  **Category D: Assistant Round-Trip (1 test, optional/flag-driven)**
  - Only runs when `ASSISTANT_ROOM_ID` is set
  - Send prompt: `mc --room "$ASSISTANT_ROOM_ID" --message "Reply with exactly: SMOKE_TEST_OK"`
  - Poll for response: loop up to 60s checking for "SMOKE_TEST_OK" in room messages
  - Assert response received or timeout with explicit failure

  **HITL Mode (flag-driven, no interactive prompts)**:
    ```bash
    HITL_MODE="${HITL_MODE:-skip}"  # skip | auto-approve | timeout
    ```

  **Implementation Notes**:
  - All secrets via environment variables, never hardcoded
  - `matrix-commander` store directory persisted between runs (avoids re-login)
  - CI-safe: no interactive prompts, all flags via env vars
  - Print clear step-by-step output
  - **Summary Block (MANDATORY)**: After all tests complete, print a summary block to stdout:
    ```
    =========================================
    MATRIX PLANE TEST SUMMARY
    =========================================
    Total:  N  |  Passed:  N  |  Failed:  N
    Groups: LoginSync(N/N) Messaging(N/N) FileUpload(N/N) Assistant(N/N)
    =========================================
    FAILURES (if any):
    - [test-name]: [exact command] → [actual response snippet]
    =========================================
    ```
  - Total: ~7 tests (8 with assistant round-trip)

  **Must NOT do**:
  - Do NOT hardcode Matrix credentials
  - Do NOT use `gomuks` (experimental terminal frontend, not suitable for automation)
  - Do NOT use interactive prompts
  - Do NOT use `matrix-commander-rs` as the primary client (optional alternative only)
  - Do NOT guess Docker image names — use verified `matrixcommander/matrix-commander`

  **Recommended Agent Profile**:
  - **Category**: `unspecified-high`
  - **Skills**: `[]`

  **Parallelization**:
  - **Can Run In Parallel**: YES (with T2 and T3)
  - **Blocks**: F1-F4
  - **Blocked By**: T1 (store wiring needed for full integration context)

  **References**:
  - `matrixcommander/matrix-commander` Docker Hub — verified image: `docker pull matrixcommander/matrix-commander`
  - `matrix-commander` pip install: `pip3 install matrix-commander` (requires `libolm-dev`)
  - Bridge health endpoint: `bridge/pkg/http/server.go:416-433`
  - Matrix Conduit homeserver config: `deploy/matrix/docker-compose.matrix.yml`

  **Acceptance Criteria**:

  **QA Scenarios (MANDATORY):**

  ```
  Scenario: Matrix sync and messaging works
    Tool: Bash (matrix-commander Docker)
    Preconditions: HOMESERVER, MATRIX_USER, MATRIX_PASSWORD, ROOM_ID set
    Steps:
      1. mc --login → exit 0
      2. mc --sync → exit 0
      3. mc --room $ROOM_ID --message "test" → exit 0
    Expected Result: Login, sync, and message send all succeed
    Evidence: .sisyphus/evidence/task-4-matrix-sync.txt

  Scenario: Assistant round-trip works
    Tool: Bash (matrix-commander Docker)
    Preconditions: ASSISTANT_ROOM_ID set, assistant running
    Steps:
      1. Send "Reply with: SMOKE_TEST_OK"
      2. Poll room for response up to 60s
      3. Assert "SMOKE_TEST_OK" appears
    Expected Result: Response received within timeout
    Evidence: .sisyphus/evidence/task-4-assistant-roundtrip.txt
  ```

  **Commit**: YES
  - Message: `test(matrix): add Matrix plane integration test`
  - Files: `tests/test-matrix-plane.sh`

---

## Final Verification Wave (MANDATORY — after ALL implementation tasks)

> 4 review agents run in PARALLEL. ALL must APPROVE. Present consolidated results to user and get explicit "okay" before completing.

- [x] F1. **Plan Compliance Audit** — `oracle`
  Read the plan end-to-end. For each "Must Have": verify implementation exists (read file, run command). For each "Must NOT Have": search for forbidden patterns. Check evidence files.
  Output: `Must Have [10/10] | Must NOT Have [10/10] | VERDICT: APPROVE`

- [x] F2. **Code Quality Review** — `unspecified-high`
  Review all 4 files: bash best practices, quoting, error handling, no hardcoded paths, no hardcoded secrets, proper cleanup. Review main.go: minimal change, correct imports.
  Output: `Scripts [CLEAN/3] | main.go [CLEAN/1] | VERDICT: APPROVE`

- [x] F3. **Automated Execution Audit** — `unspecified-high`
  Start from clean state. Run `bash tests/test-governance-rpc.sh` and verify every test passes. Run `bash -n` syntax check on all 3 test scripts. Verify output format (✅/❌ per test, summary block at end with Total/Passed/Failed counts and per-group breakdown). Verify summary block contains `FAILURES` section (empty if none). Verify cleanup (no leftover temp dirs or processes). Save all output to `.sisyphus/evidence/final-qa/`.
  Output: `Tests [3/3 pass] | Summaries [PRESENT/3] | Cleanup [CLEAN] | Syntax [CLEAN] | VERDICT: APPROVE`

- [x] F4. **Scope Fidelity Check** — `deep`
  For each task: read spec, read diff. Verify 1:1. Check "Must NOT do" compliance. Flag unaccounted changes.
  Output: `Tasks [4/4 compliant] | Unaccounted [CLEAN] | VERDICT: APPROVE`

---

## Commit Strategy

- **T1**: `fix(bridge): wire DeviceStore and InviteStore in RPC server` - bridge/cmd/bridge/main.go
- **T2**: `test(governance): add comprehensive RPC test harness` - tests/test-governance-rpc.sh
- **T3**: `test(vps): add SSH + curl integration smoke test` - tests/test-vps-smoke.sh
- **T4**: `test(matrix): add Matrix plane integration test` - tests/test-matrix-plane.sh

---

## Success Criteria

### Verification Commands
```bash
# Local governance (no VPS/Matrix/ArmorChat deps)
bash tests/test-governance-rpc.sh
# Expected: exit 0, all test groups pass

# VPS integration (auto-sources .env for VPS_IP/USER/PORT; only secrets needed on CLI)
ADMIN_TOKEN=aat_xxx bash tests/test-vps-smoke.sh
# Expected: exit 0, all tests pass (auth assertions via JSON-RPC body, not HTTP status)

# Matrix plane (auto-sources .env for VPS_IP/MATRIX_PORT; constructs HOMESERVER automatically)
MATRIX_USER=@test:domain MATRIX_PASSWORD=xxx ROOM_ID='!xxx:domain' bash tests/test-matrix-plane.sh
# Expected: exit 0, all tests pass

# Bridge compiles
cd bridge && go build -o /dev/null ./cmd/bridge
# Expected: exit 0
```

### Final Checklist
- [ ] All "Must Have" present
- [ ] All "Must NOT Have" absent
- [ ] No hardcoded secrets in any script
- [ ] All scripts exit 0 on success, 1 on failure
- [ ] No interactive prompts
