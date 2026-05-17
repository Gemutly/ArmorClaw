# Harness Hardening & Operational Trust

## TL;DR

> **Quick Summary**: Fix 5 issues in the ArmorClaw test harness to make it operationally trustworthy — dual-transport VPS testing, curl-based Matrix messaging, port conflict resolution, WebSocket broadcaster wiring, and restart persistence verification.
>
> **Deliverables**:
> - test-vps-smoke.sh: Dual transport (Unix socket over SSH + HTTP) with auto-detection
> - test-matrix-plane.sh: Curl-based send/receive replacing flaky matrix-commander
> - test-governance-rpc.sh: Discovery disabled in temp config (no port 8080 conflict)
> - test-persistence.sh: New script testing state survival across bridge restart
> - bridge/cmd/bridge/main.go: SetBroadcaster wire fix (~5 lines)
> - Updated bridge binary deployed to VPS, all tests passing live
>
> **Estimated Effort**: Medium
> **Parallel Execution**: YES — 2 waves + final verification
> **Critical Path**: T1-T5 (parallel) → T6 (build+deploy) → T7 (live run) → F1-F4 (verify)

---

## Context

### Original Request
User prioritized 5 items in order: (1) Fix T3 to test real admin path on VPS, (2) Stabilize Matrix harness, (3) Make T2 robust on hosts running bridge, (4) Wire WebSocket broadcaster, (5) Add restart/persistence tests. Items 1+2 are "harness truthfulness" — highest priority.

### Interview Summary
**Key Discussions**:
- T3 approach: User chose "both paths" — socket-over-SSH as default, HTTP if bridge detected in TCP/HTTPS mode
- T4 approach: User chose curl-based send/receive (keep matrix-commander for file upload/assistant only)
- Scope: Fix scripts + deploy + verify live on VPS (end-to-end)
- User's framing: "the fastest path from 'features are finally connected' to 'the deployed system is operationally trustworthy'"

**Research Findings**:
- Discovery HTTP server does NOT have /api RPC — only HTTPS bridge server (`[http] enabled=true`) does
- HTTPS server auto-generates self-signed TLS cert — use `curl -k`
- WebSocket broadcaster crash: `eventBus.Start()` at main.go:~2201 before `httpsServer` at main.go:~2659, SetBroadcaster never called
- T2 port conflict: temp TOML config missing `[discovery]` section → defaults to enabled=true, port=8080
- matrix-commander E151: client-side send bug, direct Conduit API curl works perfectly
- Bridge on VPS: systemd service `armorclaw-bridge.service`, binary via symlink chain
- VPS has CGO deps (libsqlcipher-dev, libyara-dev) and Go 1.22.2 for building
- Local machine CANNOT build bridge (missing CGO deps) — must build on VPS

### Metis Review
**Identified Gaps** (addressed):
- Transport detection probe order: socket first (`test -S`), HTTP fallback — incorporated into T3
- socat availability on VPS: graceful skip if missing — incorporated into T3
- Restart method: confirmed systemd (`systemctl restart armorclaw-bridge`) — incorporated into T5
- Matrix txnId collision: unique IDs via `openssl rand -hex 8` — incorporated into T2
- Token expiry: fresh login per test run — incorporated into T2
- Race condition on restart: poll for socket readiness — incorporated into T5
- WebSocketEnabled currently false on VPS: item 4 wires a dormant feature — plan acknowledges
- **Metis correction noted**: Metis said "don't move eventBus.Start()" but this contradicts the crash analysis. Start() MUST move to after SetBroadcaster wire. Correct approach implemented.

---

## Work Objectives

### Core Objective
Make the ArmorClaw test harness truthful and trustworthy — every test that passes means something real works on the deployed VPS, and every test that fails catches a real problem.

### Concrete Deliverables
- `tests/test-vps-smoke.sh` — rewritten with dual transport + auto-detection
- `tests/test-matrix-plane.sh` — curl-based send/receive, commander for file upload only
- `tests/test-governance-rpc.sh` — `[discovery] enabled = false` in temp config
- `tests/test-persistence.sh` — new file, invite lifecycle across bridge restart
- `bridge/cmd/bridge/main.go` — SetBroadcaster wire (~5 lines added)
- Updated VPS binary with all tests passing

### Definition of Done
- [ ] `bash tests/test-governance-rpc.sh` → exit 0 (on host where CGO available, or on VPS)
- [ ] `bash tests/test-vps-smoke.sh` → exit 0, reports which transport(s) tested
- [ ] `bash tests/test-matrix-plane.sh` → exit 0, no matrix-commander dependency for send/receive
- [ ] `bash tests/test-persistence.sh` → exit 0, state survives restart
- [ ] `grep -n "SetBroadcaster" bridge/cmd/bridge/main.go` returns exactly 1 match
- [ ] VPS bridge running new binary, `systemctl is-active armorclaw-bridge` = active

### Must Have
- Dual transport in T3 with auto-detection (socket-first, HTTP fallback)
- Curl-based Matrix send/receive in T4 (no matrix-commander for messaging)
- Discovery disabled in T2 temp config
- SetBroadcaster wire in main.go with proper guard (`if eventBus != nil && httpsServer != nil`)
- Fresh Matrix login per T4 test run (no stale tokens)
- Unique txnId per Matrix message (`openssl rand -hex 8`)
- Poll for bridge readiness after restart in T5 (max 30s)
- Graceful skip when transport unavailable (not hard fail)
- All scripts exit 0 on all-pass, exit 1 on any failure
- All scripts auto-source `.env` for connection details

### Must NOT Have (Guardrails)
- NO changes to bridge business logic, RPC handlers, or config defaults beyond item 4
- NO modifications to discovery server, mDNS, or Matrix client code
- NO new RPC methods or changes to existing ones
- NO touching Android, admin-panel, or browser-service code
- NO modifying production TOML config permanently (test window only for HTTP mode)
- NO installing new packages on the VPS
- NO changing Dockerfile or docker-compose files
- NO new Go packages or refactoring
- NO hardcoded VPS credentials in test scripts (source from .env)
- NO test timeout exceeding 120 seconds total per script
- NO removing existing test categories from T3/T4 (add only)
- NO matrix-commander removal from T4 (still needed for file upload/assistant tests)
- NO moving persistence tests into existing scripts (separate file)
- NO AI slop: excessive comments, over-abstraction, unnecessary variables

---

## Verification Strategy (MANDATORY)

> **ZERO HUMAN INTERVENTION** — ALL verification is agent-executed. No exceptions.

### Test Decision
- **Infrastructure exists**: YES (test scripts ARE the test infrastructure)
- **Automated tests**: The scripts themselves are the tests
- **Framework**: Bash + curl + jq + socat
- **No separate unit tests** — the scripts are tested by running them live on VPS

### QA Policy
Every task includes agent-executed QA scenarios.
Evidence saved to `.sisyphus/evidence/task-{N}-{scenario-slug}.{ext}`.

- **Test scripts**: Run on VPS via SSH, assert exit code + output content
- **Bridge code**: Verify with grep + build + deploy + live test
- **Persistence**: Run full lifecycle, assert state survives restart

---

## Execution Strategy

### Parallel Execution Waves

```
Wave 1 (Start Immediately — all independent script + code fixes):
├── T1: Fix T2 discovery port conflict [quick]
├── T2: Rewrite T4 Matrix send/receive to curl [unspecified-high]
├── T3: Add dual transport to T3 with auto-detect [unspecified-high]
├── T4: Wire SetBroadcaster in main.go [quick]
└── T5: Create test-persistence.sh [unspecified-high]

Wave 2 (After Wave 1 — build, deploy, verify):
├── T6: Build on VPS + deploy + configure HTTP mode [unspecified-high]
└── T7: Run all test suites live on VPS [deep]

Wave FINAL (After ALL tasks — 4 parallel reviews):
├── F1: Plan compliance audit [oracle]
├── F2: Code quality review [unspecified-high]
├── F3: Real manual QA [unspecified-high]
└── F4: Scope fidelity check [deep]
→ Present results → Get explicit user okay

Critical Path: T4 → T6 → T7 → F1-F4 → user okay
Parallel Speedup: ~60% faster than sequential
Max Concurrent: 5 (Wave 1)
```

### Dependency Matrix

| Task | Depends On | Blocks | Wave |
|------|-----------|--------|------|
| T1 | — | T7 | 1 |
| T2 | — | T7 | 1 |
| T3 | — | T7 | 1 |
| T4 | — | T6 | 1 |
| T5 | — | T7 | 1 |
| T6 | T4 | T7 | 2 |
| T7 | T1, T2, T3, T5, T6 | F1-F4 | 2 |
| F1-F4 | T7 | user okay | FINAL |

### Agent Dispatch Summary

- **Wave 1**: **5** — T1 → `quick`, T2 → `unspecified-high`, T3 → `unspecified-high`, T4 → `quick`, T5 → `unspecified-high`
- **Wave 2**: **2** — T6 → `unspecified-high`, T7 → `deep`
- **FINAL**: **4** — F1 → `oracle`, F2 → `unspecified-high`, F3 → `unspecified-high`, F4 → `deep`

---

## TODOs

- [x] 1. Fix T2: Disable discovery in temp config to avoid port 8080 conflict

  **What to do**:
  - In `tests/test-governance-rpc.sh`, locate the temp TOML config heredoc (lines 43-55)
  - Add a `[discovery]` section with `enabled = false` to prevent the bridge from attempting to bind port 8080 for the discovery HTTP server during governance tests
  - The temp config currently has `[keystore]`, `[server]`, and `[error_system]` sections but no `[discovery]` — this causes defaults to activate (`enabled: true`, `port: 8080`)
  - This is a 2-line addition inside the existing heredoc, no other changes needed

  **Must NOT do**:
  - Do NOT add other config sections beyond `[discovery]`
  - Do NOT change any test logic or assertions
  - Do NOT modify the bridge binary or any Go code

  **Recommended Agent Profile**:
  - **Category**: `quick`
    - Reason: Single file, 2-line addition to existing heredoc
  - **Skills**: []
  - **Skills Evaluated but Omitted**:
    - `git-master`: Not needed — trivial change, no complex git operations

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 1 (with T2, T3, T4, T5)
  - **Blocks**: T7 (live run)
  - **Blocked By**: None (can start immediately)

  **References**:

  **Pattern References**:
  - `tests/test-governance-rpc.sh:43-55` — The temp TOML config heredoc where the `[discovery]` section must be added. Currently has `[keystore]`, `[server]` (socket_path only), `[error_system]`.

  **API/Type References**:
  - `bridge/pkg/config/config.go:954-957` — Discovery config defaults: `Enabled: true`, `Port: 8080`. These activate when `[discovery]` section is absent.

  **WHY Each Reference Matters**:
  - The heredoc location is where the fix goes — agent needs to see the exact format
  - The config defaults explain why the bug exists (missing section → defaults activate)

  **Acceptance Criteria**:

  **QA Scenarios (MANDATORY):**

  ```
  Scenario: Discovery section present in temp config
    Tool: Bash (grep)
    Preconditions: tests/test-governance-rpc.sh exists
    Steps:
      1. grep -A1 '\[discovery\]' tests/test-governance-rpc.sh
      2. Assert output contains "enabled = false"
    Expected Result: Exactly one [discovery] section with enabled = false
    Failure Indicators: No match, or enabled = true
    Evidence: .sisyphus/evidence/task-1-discovery-config.txt

  Scenario: No other config sections added
    Tool: Bash (grep)
    Preconditions: tests/test-governance-rpc.sh modified
    Steps:
      1. diff <(git show HEAD:tests/test-governance-rpc.sh) tests/test-governance-rpc.sh
      2. Assert only lines added are [discovery] and enabled = false
    Expected Result: Exactly 2 new lines in the heredoc, nothing else changed
    Failure Indicators: More than 2 lines changed, changes outside heredoc
    Evidence: .sisyphus/evidence/task-1-diff.txt
  ```

  **Commit**: YES
  - Message: `fix(tests): disable discovery in T2 temp config to avoid port 8080 conflict`
  - Files: `tests/test-governance-rpc.sh`

- [x] 2. Rewrite T4 Matrix send/receive to use direct Conduit API curl

  **What to do**:
  - In `tests/test-matrix-plane.sh`, replace matrix-commander-based send/receive in Category B (lines 93-153) with direct Conduit API curl calls
  - Keep `mc()` wrapper and matrix-commander for: Category A login/room listing, Category C file upload, Category D assistant round-trip
  - Implement a new `matrix_login()` function that logs in via curl, stores access_token:
    ```bash
    matrix_login() {
      local response=$(ssh_vps_or_local "curl -s -X POST 'http://localhost:${MATRIX_PORT}/_matrix/client/v3/login' \
        -H 'Content-Type: application/json' \
        -d '{\"type\":\"m.login.password\",\"identifier\":{\"type\":\"m.id.user\",\"user\":\"${MATRIX_USER}\"},\"password\":\"${MATRIX_PASSWORD}\"}'")
      MATRIX_TOKEN=$(echo "$response" | jq -r '.access_token')
      MATRIX_USER_ID=$(echo "$response" | jq -r '.user_id')
      # Assert token is not null/empty
    }
    ```
  - Implement `matrix_send()` function:
    ```bash
    matrix_send() {
      local room_id="$1" message="$2"
      local txn_id=$(openssl rand -hex 8)
      local response=$(curl_or_ssh "curl -s -X PUT 'http://localhost:${MATRIX_PORT}/_matrix/client/v3/rooms/${room_id}/send/m.room.message/${txn_id}' \
        -H 'Authorization: Bearer ${MATRIX_TOKEN}' \
        -H 'Content-Type: application/json' \
        -d '{\"msgtype\":\"m.text\",\"body\":\"${message}\"}'")
      echo "$response" | jq -r '.event_id'
    }
    ```
  - Implement `matrix_receive()` function:
    ```bash
    matrix_receive() {
      local room_id="$1" since_token="$2"
      curl_or_ssh "curl -s 'http://localhost:${MATRIX_PORT}/_matrix/client/v3/rooms/${room_id}/messages?dir=b&limit=20' \
        -H 'Authorization: Bearer ${MATRIX_TOKEN}'"
    }
    ```
  - Category B tests become: B1 (login via curl), B2 (send via curl), B3 (verify received via curl), B4 (round-trip via curl)
  - Each message uses a unique token: `UNIQUE_TOKEN="ARMORCLAW-$(openssl rand -hex 4)"`
  - The `curl_or_ssh` helper sends curl directly if MATRIX_HOST is localhost, or via ssh_vps for VPS testing
  - All API calls go through the existing VPS_IP via SSH (same pattern as T3's `ssh_vps`)
  - The script should work both locally (direct curl) and via SSH (remote curl) based on env vars

  **Must NOT do**:
  - Do NOT remove matrix-commander entirely — still needed for file upload (Category C) and assistant (Category D)
  - Do NOT change the script's exit code contract or counter structure
  - Do NOT hardcode credentials — use MATRIX_USER, MATRIX_PASSWORD from .env
  - Do NOT increase total test timeout beyond 120s

  **Recommended Agent Profile**:
  - **Category**: `unspecified-high`
    - Reason: Moderate complexity — rewriting core messaging logic with new curl functions, handling SSH/direct transport, preserving test structure
  - **Skills**: []
  - **Skills Evaluated but Omitted**:
    - `playwright`: Not needed — no browser interaction

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 1 (with T1, T3, T4, T5)
  - **Blocks**: T7 (live run)
  - **Blocked By**: None (can start immediately)

  **References**:

  **Pattern References**:
  - `tests/test-vps-smoke.sh:37-62` — SSH helper pattern (`ssh_vps()`, `rpc_vps()`) to follow for the `ssh_vps_or_local` curl wrapper
  - `tests/test-matrix-plane.sh:93-153` — Current Category B messaging tests being replaced. Follow the same test counter/group structure.

  **API/Type References**:
  - Matrix Client-Server API v3: `POST /_matrix/client/v3/login` — Login with `m.login.password`, returns `access_token`, `user_id`, `device_id`
  - Matrix Client-Server API v3: `PUT /_matrix/client/v3/rooms/{roomId}/send/m.room.message/{txnId}` — Send message, returns `event_id`
  - Matrix Client-Server API v3: `GET /_matrix/client/v3/rooms/{roomId}/messages?dir=b&limit=20` — Get messages, returns `chunk` array with `content.body`

  **Test References**:
  - `tests/test-matrix-plane.sh:40-49` — Existing `mc()` wrapper pattern to preserve for Categories C and D

  **External References**:
  - Conduit Matrix API: Compatible with Matrix v3 Client-Server spec. Base URL: `http://localhost:6167`

  **WHY Each Reference Matters**:
  - T3 SSH pattern shows how to properly tunnel curl through SSH to VPS
  - Category B structure shows the test organization to preserve
  - Matrix API endpoints are the exact contracts being implemented
  - mc() wrapper must be preserved for file upload/assistant tests

  **Acceptance Criteria**:

  **QA Scenarios (MANDATORY):**

  ```
  Scenario: No matrix-commander for message sending
    Tool: Bash (grep)
    Preconditions: tests/test-matrix-plane.sh modified
    Steps:
      1. grep -n 'mc.*--message' tests/test-matrix-plane.sh
      2. Assert 0 matches (no mc --message calls in the file)
    Expected Result: grep returns exit code 1 (no matches)
    Failure Indicators: Any mc --message calls found
    Evidence: .sisyphus/evidence/task-2-no-mc-send.txt

  Scenario: Curl login function exists
    Tool: Bash (grep)
    Preconditions: tests/test-matrix-plane.sh modified
    Steps:
      1. grep -c 'matrix_login\|/_matrix/client/v3/login' tests/test-matrix-plane.sh
      3. Assert ≥ 2 matches (function definition + usage)
    Expected Result: Login function defined and called
    Failure Indicators: No curl-based login found
    Evidence: .sisyphus/evidence/task-2-curl-login.txt

  Scenario: Unique txnId for each message
    Tool: Bash (grep)
    Preconditions: tests/test-matrix-plane.sh modified
    Steps:
      1. grep -c 'openssl rand -hex 8\|txn_id\|txnId' tests/test-matrix-plane.sh
      2. Assert ≥ 1 match (txnId generation present)
    Expected Result: Transaction ID generation with sufficient randomness
    Failure Indicators: Hardcoded or sequential txnId
    Evidence: .sisyphus/evidence/task-2-txnid.txt

  Scenario: Matrix send/receive works via curl on VPS
    Tool: Bash (curl via SSH)
    Preconditions: VPS Conduit running, .env has MATRIX_USER/MATRIX_PASSWORD
    Steps:
      1. ssh -i ~/.ssh/openclaw_win root@5.183.11.149 "curl -s -X POST 'http://localhost:6167/_matrix/client/v3/login' -H 'Content-Type: application/json' -d '{\"type\":\"m.login.password\",\"identifier\":{\"type\":\"m.id.user\",\"user\":\"armorclaw-bridge\"},\"password\":\"jIa0vwprzlBZJwZMEawVI59h7DlQ76bT\"}' | jq -r '.access_token'"
      2. Assert non-empty token returned
      3. Use token to send message to known room
      4. Use token to GET /messages, grep for sent content
    Expected Result: Login returns token, send returns event_id, receive contains message body
    Failure Indicators: Empty token, no event_id, message not in received chunk
    Evidence: .sisyphus/evidence/task-2-matrix-curl-roundtrip.txt
  ```

  **Commit**: YES
  - Message: `fix(tests): rewrite T4 Matrix send/receive to use direct Conduit API curl`
  - Files: `tests/test-matrix-plane.sh`

- [x] 3. Add dual transport to T3 with Unix socket + HTTP auto-detection

  **What to do**:
  - In `tests/test-vps-smoke.sh`, add transport auto-detection and a socket-based RPC path alongside the existing HTTP path
  - Add a `detect_transport()` function that:
    1. SSH to VPS, checks if `/run/armorclaw/bridge.sock` exists: `ssh_vps "test -S /run/armorclaw/bridge.sock && echo SOCKET_EXISTS"`
    2. SSH to VPS, checks if HTTP /api responds: `ssh_vps "curl -kfsS -o /dev/null -w '%{http_code}' http://localhost:${BRIDGE_PORT}/api 2>/dev/null || echo NO_HTTP"`
    3. Sets TRANSPORT_MODE variable: "socket" or "http" or "both" or "none"
    4. Reports detected mode in test output
  - Add a `rpc_socket()` function (alongside existing `rpc_vps()` which becomes `rpc_http()`):
    ```bash
    rpc_socket() {
      local method="$1"
      local params="${2:-{}}"
      local payload="{\"jsonrpc\":\"2.0\",\"id\":1,\"method\":\"${method}\",\"params\":${params},\"auth\":\"${ADMIN_TOKEN}\"}"
      ssh_vps "echo '${payload}' | socat - UNIX-CONNECT:/run/armorclaw/bridge.sock"
    }
    ```
  - Add a `check_socat()` function that verifies socat is available on VPS: `ssh_vps "command -v socat >/dev/null 2>&1 && echo YES || echo NO"`
  - Restructure Category B (Auth) and Category C (Governance) to run against detected transport:
    - If socket available: run socket-based auth tests (send RPC without auth → expect error, with auth → expect success)
    - If HTTP available: run existing HTTP-based auth tests (existing code)
    - If both available: run both, compare results
    - If neither available: FAIL with clear message about which transports were tried
  - For Category A (Bridge Health):
    - A2 (Health endpoint): Skip if no HTTP transport (socket has no /health)
    - D1 (Discovery): Skip if no HTTP transport
  - Graceful degradation: if socat not on VPS, skip socket tests (print `[SKIP] socket tests: socat not available`), do NOT fail
  - The script should report at the end: "Transport tested: socket (N tests), http (N tests)"

  **Must NOT do**:
  - Do NOT remove the existing HTTP-based tests — only add socket variants
  - Do NOT change the script's exit code contract or counter structure
  - Do NOT hardcode VPS credentials
  - Do NOT require socat — make it optional with graceful skip

  **Recommended Agent Profile**:
  - **Category**: `unspecified-high`
    - Reason: Significant script restructuring — adding transport detection, socket RPC helper, conditional test execution, while preserving existing test structure
  - **Skills**: []
  - **Skills Evaluated but Omitted**:
    - `playwright`: Not needed — no browser interaction

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 1 (with T1, T2, T4, T5)
  - **Blocks**: T7 (live run)
  - **Blocked By**: None (can start immediately)

  **References**:

  **Pattern References**:
  - `tests/test-vps-smoke.sh:37-62` — Existing helpers: `ssh_vps()` at line 38, `rpc_vps()` at line 40-47. New `rpc_socket()` follows same pattern but uses socat over SSH instead of curl over SSH.
  - `tests/test-governance-rpc.sh:95-122` — `rpc_call()` and `raw_rpc()` functions show how to format JSON-RPC for Unix socket transport. Use same JSON-RPC structure.

  **API/Type References**:
  - `bridge/pkg/socket/server.go` — Unix socket server that accepts raw JSON-RPC. No HTTP wrapping. Auth is via the `"auth"` field in the JSON-RPC params (not HTTP headers).
  - `bridge/pkg/http/server.go:378-414` — HTTP `/api` handleRPC. Auth is via `Authorization: Bearer` header.

  **Test References**:
  - `tests/test-vps-smoke.sh:111-157` — Category B auth tests (B1 no auth, B2 valid auth, B3 invalid token). Socket variants must test the same auth enforcement but via `"auth"` field in JSON-RPC body.

  **External References**:
  - JSON-RPC 2.0 spec: Auth extension — the `"auth"` field in params is an ArmorClaw extension for socket-based auth

  **WHY Each Reference Matters**:
  - The existing ssh_vps/rpc_vps pattern is the template for the new functions
  - The governance RPC script shows working socket RPC format
  - Socket auth uses JSON-RPC body field, HTTP auth uses header — agent must handle both
  - Existing test structure must be preserved while adding conditional execution

  **Acceptance Criteria**:

  **QA Scenarios (MANDATORY):**

  ```
  Scenario: Transport detection functions exist
    Tool: Bash (grep)
    Preconditions: tests/test-vps-smoke.sh modified
    Steps:
      1. grep -c 'detect_transport\|rpc_socket\|check_socat' tests/test-vps-smoke.sh
      2. Assert ≥ 3 matches
    Expected Result: All three new functions defined
    Failure Indicators: Missing functions
    Evidence: .sisyphus/evidence/task-3-transport-functions.txt

  Scenario: Socket-based RPC works on VPS
    Tool: Bash (SSH + socat)
    Preconditions: VPS bridge running, socat available on VPS
    Steps:
      1. ssh -i ~/.ssh/openclaw_win root@5.183.11.149 "echo '{\"jsonrpc\":\"2.0\",\"id\":1,\"method\":\"device.list\",\"params\":{},\"auth\":\"aat_57f59b6eec6fdab12d545f6718ecf4b1ab14cb90c601bf94\"}' | socat - UNIX-CONNECT:/run/armorclaw/bridge.sock"
      2. Assert JSON response contains "result" key
    Expected Result: {"jsonrpc":"2.0","id":1,"result":[]}
    Failure Indicators: Empty response, connection refused, "error" in response
    Evidence: .sisyphus/evidence/task-3-socket-rpc.txt

  Scenario: Graceful skip when socat missing
    Tool: Bash (grep)
    Preconditions: tests/test-vps-smoke.sh modified
    Steps:
      1. grep -c 'check_socat\|socat not available\|\[SKIP\]' tests/test-vps-smoke.sh
      2. Assert ≥ 1 match (graceful skip logic present)
    Expected Result: Script handles missing socat without hard failure
    Failure Indicators: No skip logic, script would crash if socat missing
    Evidence: .sisyphus/evidence/task-3-socat-skip.txt

  Scenario: Transport mode reported in output
    Tool: Bash (grep)
    Preconditions: tests/test-vps-smoke.sh modified
    Steps:
      1. grep -c 'Transport tested\|TRANSPORT_MODE\|transport' tests/test-vps-smoke.sh
      2. Assert ≥ 2 matches
    Expected Result: Script reports which transport was tested in summary
    Failure Indicators: No transport reporting
    Evidence: .sisyphus/evidence/task-3-transport-report.txt
  ```

  **Commit**: YES
  - Message: `feat(tests): add dual transport to T3 with Unix socket + HTTP auto-detection`
  - Files: `tests/test-vps-smoke.sh`

- [x] 4. Wire SetBroadcaster in main.go for WebSocket event distribution

  **What to do**:
  - In `bridge/cmd/bridge/main.go`, fix the EventBroadcaster wiring so the bridge boots cleanly with `websocket_enabled = true`
  - **Root cause**: `eventBus.Start()` is called at ~line 2201 BEFORE `httpsServer` is created at ~line 2659. `Start()` calls `websocketServer.Start()` which checks `broadcaster == nil` and returns `errNoBroadcaster`, which triggers `log.Fatalf` — crashing the bridge.
  - **The fix** (3 changes in main.go):

  **Change 1**: At ~line 2200-2210, REMOVE the `eventBus.Start()` call. Keep the `eventBus = eventbus.NewEventBus(...)` creation. Replace Start() with a comment:
    ```go
    // Event bus will be started after HTTPS server creation (needs broadcaster wire)
    ```

  **Change 2**: At ~line 2670-2680 (AFTER `httpsServer = bridgeHTTP.NewServer(...)` and BEFORE `go httpsServer.Start()`), ADD:
    ```go
    // Wire HTTP server as WebSocket broadcaster into event bus
    if eventBus != nil && httpsServer != nil {
        eventBus.SetBroadcaster(httpsServer)
    }
    // Start event bus (after broadcaster is wired)
    if eventBus != nil {
        if err := eventBus.Start(); err != nil {
            log.Printf("Warning: Failed to start event bus: %v", err)
        } else {
            log.Println("Event bus started")
        }
    }
    ```

  **Change 3**: Before the eventBus creation (~line 2198), ADD a config validation guard:
    ```go
    // Validate: WebSocket requires HTTP server (broadcaster lives in HTTPS server)
    if cfg.EventBus.WebSocketEnabled && !cfg.HTTP.Enabled {
        log.Fatalf("Configuration error: websocket_enabled=true requires [http] enabled=true (the HTTP server hosts the WebSocket endpoint)")
    }
    ```

  **Must NOT do**:
  - Do NOT modify eventbus.go, websocket.go, or any other Go files
  - Do NOT change the log.Fatalf crash contract — it should still crash if broadcaster is nil at Start time
  - Do NOT touch the existing eventBus creation logic
  - Do NOT refactor or reorganize any surrounding code
  - Do NOT change more than ~12 lines total (3 deletions + ~9 additions)

  **Recommended Agent Profile**:
  - **Category**: `quick`
    - Reason: Precise 3-point edit in a single file, exactly specified. No design decisions needed.
  - **Skills**: []
  - **Skills Evaluated but Omitted**:
    - `git-master`: Not needed — simple change, standard commit

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 1 (with T1, T2, T3, T5)
  - **Blocks**: T6 (build requires this change)
  - **Blocked By**: None (can start immediately)

  **References**:

  **Pattern References**:
  - `bridge/cmd/bridge/main.go:2651-2679` — HTTPS server creation block. The SetBroadcaster call goes AFTER `httpsServer = bridgeHTTP.NewServer(...)` here.
  - `bridge/cmd/bridge/main.go:2198-2210` — Event bus creation and Start block. The Start() call must be REMOVED from here and moved to after the wire.

  **API/Type References**:
  - `bridge/pkg/eventbus/eventbus.go:165-169` — `SetBroadcaster(broadcaster websocket.EventBroadcaster)` method. This is what must be called before Start().
  - `bridge/pkg/websocket/websocket.go:66-75` — `Start()` method that checks `s.broadcaster == nil` and returns `errNoBroadcaster()`. This is why the bridge crashes.
  - `bridge/pkg/http/server.go:1122` — `Server.BroadcastEvent(eventType string, payload []byte)` — the method that satisfies the `websocket.EventBroadcaster` interface.

  **External References**:
  - Go interface satisfaction: `bridgeHTTP.Server` implicitly satisfies `websocket.EventBroadcaster` via the `BroadcastEvent` method.

  **WHY Each Reference Matters**:
  - The exact line numbers for insertion/deletion are critical — wrong placement causes compilation errors or runtime crashes
  - The interface contract shows why httpsServer can be passed to SetBroadcaster
  - The crash location shows what must be prevented

  **Acceptance Criteria**:

  **QA Scenarios (MANDATORY):**

  ```
  Scenario: SetBroadcaster called exactly once in main.go
    Tool: Bash (grep)
    Preconditions: bridge/cmd/bridge/main.go modified
    Steps:
      1. grep -n 'SetBroadcaster' bridge/cmd/bridge/main.go
      2. Assert exactly 1 match, line number > 2650 (after httpsServer creation)
    Expected Result: Single match at line ~2670-2680
    Failure Indicators: 0 matches (not wired), 2+ matches (over-wired), line < 2650 (before httpsServer exists)
    Evidence: .sisyphus/evidence/task-4-setbroadcaster.txt

  Scenario: Config validation guard present
    Tool: Bash (grep)
    Preconditions: bridge/cmd/bridge/main.go modified
    Steps:
      1. grep -c 'websocket_enabled.*http.*enabled\|WebSocketEnabled.*HTTP.Enabled' bridge/cmd/bridge/main.go
      2. Assert ≥ 1 match
    Expected Result: Guard that prevents WebSocketEnabled without HTTP enabled
    Failure Indicators: No guard found
    Evidence: .sisyphus/evidence/task-4-config-guard.txt

  Scenario: Bridge compiles successfully
    Tool: Bash (go build)
    Preconditions: CGO deps available (run on VPS or machine with libsqlcipher-dev/libyara-dev)
    Steps:
      1. cd bridge && go build -o /dev/null ./cmd/bridge/
      2. Assert exit code 0
    Expected Result: Clean compilation, no errors
    Failure Indicators: Compilation errors, undefined references
    Evidence: .sisyphus/evidence/task-4-build.txt

  Scenario: No changes to other Go files
    Tool: Bash (git diff)
    Preconditions: Changes committed
    Steps:
      1. git diff --name-only HEAD~1
      2. Assert only bridge/cmd/bridge/main.go in diff
    Expected Result: Single file changed
    Failure Indicators: Multiple files, files outside bridge/
    Evidence: .sisyphus/evidence/task-4-diff-files.txt
  ```

  **Commit**: YES
  - Message: `fix(bridge): wire EventBroadcaster for WebSocket event distribution`
  - Files: `bridge/cmd/bridge/main.go`

- [x] 5. Create test-persistence.sh for invite lifecycle across bridge restart

  **What to do**:
  - Create a new file `tests/test-persistence.sh` that tests invite state survival across bridge restart
  - Follow the script structure pattern from `test-governance-rpc.sh`: shebang + strict mode, env sourcing, counters, cleanup trap, summary
  - Auto-source `.env` for VPS_IP, VPS_USER, ADMIN_TOKEN, SSH_KEY_PATH
  - The script runs entirely against the VPS (no local bridge needed)
  - Test sequence:
    1. **P0: Prerequisites** — Verify SSH connectivity, bridge running, socat available (or curl for HTTP mode). Store detected transport method.
    2. **P1: Create invite** — Send `invite.create` RPC via detected transport (socket or HTTP). Capture `invite_id` and `code` from response.
    3. **P2: Pre-restart validation** — `invite.validate` with the code → expect `"status":"active"`. `invite.list` → expect invite_id present.
    4. **P3: Restart bridge** — `ssh_vps "systemctl restart armorclaw-bridge"`. Poll for bridge readiness (check socket or HTTP health, max 30s with 2s intervals).
    5. **P4: Post-restart validation** — `invite.validate` with same code → expect `"status":"active"`. `invite.list` → expect invite_id still present.
    6. **P5: Revoke after restart** — `invite.revoke` with invite_id → expect `"success":true`. `invite.validate` → expect `"revoked"` or error code.
    7. **P6: Cleanup** — Revoke any invites created during the test (best-effort, don't fail if already revoked).
  - Use unique identifiers per run: `TEST_RUN_ID="persist-$(date +%s)-$$"`
  - The `invite.create` params should include `created_by: "persistence-test"` and `role: "admin"` for identification
  - All RPC calls use the detected transport (socket via socat over SSH, or HTTP via curl over SSH)
  - Print summary: total tests, passed, failed, per-group counts

  **Must NOT do**:
  - Do NOT append to existing test scripts — this is a separate file
  - Do NOT test device persistence, keystore persistence, or other state (invite-only scope)
  - Do NOT modify any bridge code or config permanently
  - Do NOT hardcode credentials — source from .env

  **Recommended Agent Profile**:
  - **Category**: `unspecified-high`
    - Reason: Creating a new test script from scratch with transport detection, restart handling, state verification, and proper bash structure
  - **Skills**: []
  - **Skills Evaluated but Omitted**:
    - `git-master`: Not needed — simple file creation

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 1 (with T1, T2, T3, T4)
  - **Blocks**: T7 (live run)
  - **Blocked By**: None (can start immediately)

  **References**:

  **Pattern References**:
  - `tests/test-governance-rpc.sh:1-28` — Script structure pattern: shebang, strict mode, path setup, counter initialization. Follow this exact structure.
  - `tests/test-governance-rpc.sh:30-40` — Cleanup trap pattern: kill process, remove temp files, report counts.
  - `tests/test-vps-smoke.sh:37-47` — SSH and RPC helper patterns. Use `ssh_vps()` and `rpc_vps()`/`rpc_socket()` pattern.

  **API/Type References**:
  - `invite.create` params: `{"role": "admin", "expiration": "24h", "max_uses": 1, "created_by": "persistence-test", "welcome_message": "test invite"}`
  - `invite.validate` params: `{"code": "<code>"}` → `{"status": "active", "invite_id": "...", ...}`
  - `invite.list` params: `{}` → `{"result": [{"invite_id": "...", ...}]}`
  - `invite.revoke` params: `{"invite_id": "<id>"}` → `{"success": true}`

  **Test References**:
  - `tests/test-governance-rpc.sh:192-259` — GROUP 2 invite happy path: create → validate → list → revoke → verify revoked. Follow this exact test flow but add restart between steps.

  **External References**:
  - systemd: `systemctl restart armorclaw-bridge` — restarts the bridge service on VPS
  - systemd: `systemctl is-active armorclaw-bridge` — checks if service is running

  **WHY Each Reference Matters**:
  - The governance test script is the closest structural pattern for a new bash test script
  - The invite lifecycle in GROUP 2 is the exact sequence being tested, plus a restart step
  - SSH helpers show how to execute commands on VPS
  - RPC params are the exact JSON shapes the test must send

  **Acceptance Criteria**:

  **QA Scenarios (MANDATORY):**

  ```
  Scenario: Script structure follows pattern
    Tool: Bash (grep)
    Preconditions: tests/test-persistence.sh exists
    Steps:
      1. head -2 tests/test-persistence.sh | grep -c 'set -euo pipefail'
      2. grep -c 'cleanup\|trap.*EXIT' tests/test-persistence.sh
      3. grep -c 'TOTAL\|PASSED\|FAILED' tests/test-persistence.sh
    Expected Result: Strict mode, cleanup trap, and counter variables present
    Failure Indicators: Missing any of the three structural elements
    Evidence: .sisyphus/evidence/task-5-structure.txt

  Scenario: All test groups present
    Tool: Bash (grep)
    Preconditions: tests/test-persistence.sh exists
    Steps:
      1. grep -c 'P[1-6]\|Create invite\|Pre-restart\|Restart\|Post-restart\|Revoke\|Cleanup' tests/test-persistence.sh
      2. Assert ≥ 6 matches (6 test groups)
    Expected Result: All 6 test phases (P1-P6) present
    Failure Indicators: Missing test phases
    Evidence: .sisyphus/evidence/task-5-groups.txt

  Scenario: Restart logic polls for readiness
    Tool: Bash (grep)
    Preconditions: tests/test-persistence.sh exists
    Steps:
      1. grep -c 'systemctl restart\|poll\|ready\|sleep\|timeout\|is-active' tests/test-persistence.sh
      2. Assert ≥ 3 matches (restart command + polling logic)
    Expected Result: Bridge restart with readiness polling (not instant assumption)
    Failure Indicators: No polling, assumes immediate availability
    Evidence: .sisyphus/evidence/task-5-restart-logic.txt

  Scenario: File is executable
    Tool: Bash (ls)
    Preconditions: tests/test-persistence.sh created
    Steps:
      1. ls -la tests/test-persistence.sh
      2. Assert permissions include execute bit
    Expected Result: -rwxr-xr-x
    Failure Indicators: No execute bit
    Evidence: .sisyphus/evidence/task-5-executable.txt
  ```

  **Commit**: YES
  - Message: `test(persistence): add invite lifecycle test across bridge restart`
  - Files: `tests/test-persistence.sh`

- [x] 6. Build on VPS + deploy + configure HTTP mode for test window

  **What to do**:
  - SSH to VPS and build the updated bridge binary (with SetBroadcaster fix from T4)
  - Deploy the binary using the existing symlink chain
  - Enable HTTP mode in config for T3's HTTP transport test path
  - Restart the bridge service and verify it comes up cleanly

  **Step-by-step**:
  1. Push all commits to origin/main (T1-T5 should be committed first)
  2. SSH to VPS, pull latest code: `ssh_vps "cd /opt/armorclaw && git pull origin main"`
  3. Build on VPS: `ssh_vps "cd /opt/armorclaw/bridge && CGO_ENABLED=1 go build -o build/armorclaw-bridge ./cmd/bridge/"`
  4. Verify binary: `ssh_vps "ls -la /opt/armorclaw/bridge/build/armorclaw-bridge"`
  5. Stop bridge: `ssh_vps "systemctl stop armorclaw-bridge"` (may need `kill -9` if slow shutdown)
  6. Enable HTTP mode in config — add to `/etc/armorclaw/config.toml`:
     ```toml
     [http]
     enabled = true
     port = 8080
     hostname = "0.0.0.0"
     ```
     Note: This enables the HTTPS server on port 8080 with auto-generated self-signed cert. Discovery is automatically suppressed (served via HTTPS server instead). Keep `websocket_enabled = false` for now (item 4 wiring tested via code review, not live WebSocket — that's a future milestone).
  7. Start bridge: `ssh_vps "systemctl start armorclaw-bridge"`
  8. Verify: `ssh_vps "systemctl is-active armorclaw-bridge"` → "active"
  9. Verify HTTP: `ssh_vps "curl -kfsS https://localhost:8080/health"` → `{"status":"ok"}`
  10. Verify socket still works: `ssh_vps "echo '{\"jsonrpc\":\"2.0\",\"id\":1,\"method\":\"device.list\",\"params\":{}}' | socat - UNIX-CONNECT:/run/armorclaw/bridge.sock"` → should return auth error (no token in body)

  **Must NOT do**:
  - Do NOT permanently change the VPS config — the [http] section can stay but must be documented as test-mode
  - Do NOT modify the systemd service file
  - Do NOT enable websocket_enabled=true on VPS (future milestone)
  - Do NOT install new packages on VPS
  - Do NOT delete the old binary (symlink chain handles this)

  **Recommended Agent Profile**:
  - **Category**: `unspecified-high`
    - Reason: Multi-step deployment with config changes, service management, and verification. Requires careful sequencing.
  - **Skills**: []
  - **Skills Evaluated but Omitted**:
    - `git-master`: Could be useful for push, but standard git commands suffice

  **Parallelization**:
  - **Can Run In Parallel**: NO
  - **Parallel Group**: Wave 2 (sequential, after T4)
  - **Blocks**: T7 (live run)
  - **Blocked By**: T4 (must include SetBroadcaster fix in build)

  **References**:

  **Pattern References**:
  - Previous deployment session (from conversation context): The exact steps used to deploy the previous binary — `cd /opt/armorclaw && git pull`, `go build`, symlink chain, systemctl restart.

  **API/Type References**:
  - `bridge/pkg/config/config.go:HTTPConfig` — `[http]` config section: `Enabled`, `Port`, `Hostname`, `CertDir`. Setting `enabled=true, port=8080, hostname="0.0.0.0"` starts the HTTPS server.
  - `/etc/armorclaw/config.toml` — Current VPS config. Already has `[server]`, `[keystore]`, `[event_bus] websocket_enabled=false`. Need to add `[http]` section.

  **WHY Each Reference Matters**:
  - Previous deployment steps are proven to work
  - Config fields show exactly what to add to enable HTTP mode
  - The VPS config file is where the change goes

  **Acceptance Criteria**:

  **QA Scenarios (MANDATORY):**

  ```
  Scenario: New binary deployed and running
    Tool: Bash (SSH)
    Preconditions: T4 committed and pushed, VPS reachable
    Steps:
      1. ssh -i ~/.ssh/openclaw_win root@5.183.11.149 "systemctl is-active armorclaw-bridge"
      2. Assert output is "active"
      3. ssh -i ~/.ssh/openclaw_win root@5.183.11.149 "curl -kfsS https://localhost:8080/health"
      4. Assert output contains "ok"
    Expected Result: Bridge running with HTTPS server on port 8080
    Failure Indicators: Service inactive, health check fails
    Evidence: .sisyphus/evidence/task-6-deploy.txt

  Scenario: Both transports available
    Tool: Bash (SSH)
    Preconditions: Bridge running with new config
    Steps:
      1. ssh -i ~/.ssh/openclaw_win root@5.183.11.149 "test -S /run/armorclaw/bridge.sock && echo SOCKET_OK"
      2. Assert "SOCKET_OK"
      3. ssh -i ~/.ssh/openclaw_win root@5.183.11.149 "curl -kfsS https://localhost:8080/health 2>/dev/null && echo HTTP_OK"
      4. Assert "HTTP_OK"
    Expected Result: Both Unix socket and HTTPS server running
    Failure Indicators: One or both transports unavailable
    Evidence: .sisyphus/evidence/task-6-dual-transport.txt

  Scenario: SetBroadcaster fix in deployed binary
    Tool: Bash (SSH + strings)
    Preconditions: New binary deployed
    Steps:
      1. ssh -i ~/.ssh/openclaw_win root@5.183.11.149 "strings /opt/armorclaw/bridge/build/armorclaw-bridge | grep SetBroadcaster"
      2. Assert at least 1 match (symbol exists in binary)
    Expected Result: SetBroadcaster symbol present in compiled binary
    Failure Indicators: No match (old binary without the fix)
    Evidence: .sisyphus/evidence/task-6-broadcaster-symbol.txt
  ```

  **Commit**: NO (deployment step, no code changes)

- [x] 7. Run all test suites live on VPS and capture evidence

  **What to do**:
  - Execute all test scripts against the live VPS deployment and capture results
  - This is the verification step that proves everything works end-to-end

  **Step-by-step**:
  1. Run T2 (governance RPC) on VPS (since local machine can't build bridge):
     ```bash
     ssh -i ~/.ssh/openclaw_win root@5.183.11.149 "cd /opt/armorclaw && bash tests/test-governance-rpc.sh"
     ```
     Note: T2 may not run on VPS either if go build is needed — in that case, verify the temp config fix via grep and run the governance tests via the socket path on the running bridge.
  2. Run T3 (VPS smoke test with dual transport):
     ```bash
     bash tests/test-vps-smoke.sh
     ```
     Should test both socket and HTTP paths. Capture output showing which transport(s) were tested.
  3. Run T4 (Matrix plane with curl-based messaging):
     ```bash
     bash tests/test-matrix-plane.sh
     ```
     Should complete without matrix-commander send failures.
  4. Run T5 (persistence):
     ```bash
     bash tests/test-persistence.sh
     ```
     Should show invite surviving bridge restart.
  5. Save all outputs as evidence files.
  6. If any test fails: diagnose, fix script (NOT product code), re-run. If product issue found, report to user.

  **Must NOT do**:
  - Do NOT fix product bugs during this step — only fix test scripts
  - Do NOT skip failing tests — investigate and fix or report
  - Do NOT modify VPS config during testing (already set up in T6)

  **Recommended Agent Profile**:
  - **Category**: `deep`
    - Reason: Requires investigation and debugging if tests fail, systematic evidence collection, potential script fixes
  - **Skills**: []
  - **Skills Evaluated but Omitted**:
    - `playwright`: Not needed — no browser testing

  **Parallelization**:
  - **Can Run In Parallel**: NO (sequential test execution)
  - **Parallel Group**: Wave 2 (sequential, after T6)
  - **Blocks**: F1-F4 (verification wave)
  - **Blocked By**: T6 (binary must be deployed first)

  **References**:

  **Pattern References**:
  - `.env` — Contains VPS_IP=5.183.11.149, VPS_USER=root, BRIDGE_PORT=8080, MATRIX_PORT=6167, SSH_KEY_PATH, ADMIN_TOKEN, MATRIX_USER, MATRIX_PASSWORD
  - All test scripts auto-source `.env` on startup

  **API/Type References**:
  - VPS: `root@5.183.11.149`, SSH key: `~/.ssh/openclaw_win`
  - Bridge: systemd service `armorclaw-bridge`, Unix socket at `/run/armorclaw/bridge.sock`, HTTPS at `https://localhost:8080`
  - Conduit: Docker container `armorclaw-conduit`, port 6167
  - Admin token: `aat_57f59b6eec6fdab12d545f6718ecf4b1ab14cb90c601bf94`
  - Matrix user: `@armorclaw-bridge:5.183.11.149`, password: `jIa0vwprzlBZJwZMEawVI59h7DlQ76bT`

  **WHY Each Reference Matters**:
  - .env provides all connection details scripts need
  - VPS details for SSH execution
  - Service names for restart in persistence test

  **Acceptance Criteria**:

  **QA Scenarios (MANDATORY):**

  ```
  Scenario: T3 passes with dual transport
    Tool: Bash (script execution)
    Preconditions: T6 complete, bridge running with HTTP+socket
    Steps:
      1. bash tests/test-vps-smoke.sh 2>&1 | tee .sisyphus/evidence/task-7-t3-output.txt
      2. Assert exit code 0
      3. grep -c 'Transport tested' .sisyphus/evidence/task-7-t3-output.txt
      4. Assert transport report present
    Expected Result: All tests pass, both socket and HTTP reported
    Failure Indicators: Exit code 1, transport not reported, tests skipped
    Evidence: .sisyphus/evidence/task-7-t3-output.txt

  Scenario: T4 passes with curl-based messaging
    Tool: Bash (script execution)
    Preconditions: T6 complete, Conduit running
    Steps:
      1. bash tests/test-matrix-plane.sh 2>&1 | tee .sisyphus/evidence/task-7-t4-output.txt
      2. Assert exit code 0
      3. grep -c 'PASS\|FAIL' .sisyphus/evidence/task-7-t4-output.txt
      4. Assert more PASS than FAIL
    Expected Result: All messaging tests pass via curl, no E151 errors
    Failure Indicators: Exit code 1, E151 error in output, curl failures
    Evidence: .sisyphus/evidence/task-7-t4-output.txt

  Scenario: T5 persistence test passes
    Tool: Bash (script execution)
    Preconditions: T6 complete, bridge running
    Steps:
      1. bash tests/test-persistence.sh 2>&1 | tee .sisyphus/evidence/task-7-t5-output.txt
      2. Assert exit code 0
      3. grep 'Post-restart' .sisyphus/evidence/task-7-t5-output.txt
      4. Assert PASS for post-restart validation
    Expected Result: Invite state survives bridge restart
    Failure Indicators: Exit code 1, post-restart validation fails
    Evidence: .sisyphus/evidence/task-7-t5-output.txt

  Scenario: All evidence files captured
    Tool: Bash (ls)
    Preconditions: All tests executed
    Steps:
      1. ls .sisyphus/evidence/task-7-*.txt
      2. Assert ≥ 3 files (T3, T4, T5 outputs)
    Expected Result: Evidence for each test suite
    Failure Indicators: Missing evidence files
    Evidence: .sisyphus/evidence/task-7-evidence-listing.txt
  ```

  **Commit**: NO (verification step, no code changes — script fixes committed separately if needed)

---

## Final Verification Wave (MANDATORY — after ALL implementation tasks)

> 4 review agents run in PARALLEL. ALL must APPROVE. Present consolidated results to user and get explicit "okay" before completing.

- [x] F1. **Plan Compliance Audit** — `oracle`
  Read the plan end-to-end. For each "Must Have": verify implementation exists (read file, run command). For each "Must NOT Have": search codebase for forbidden patterns — reject with file:line if found. Check evidence files exist in .sisyphus/evidence/. Compare deliverables against plan.
  Output: `Must Have [N/N] | Must NOT Have [N/N] | Tasks [N/N] | VERDICT: APPROVE/REJECT`

- [x] F2. **Code Quality Review** — `unspecified-high`
  Run `go vet ./bridge/...` in the bridge directory. Review all changed files for: `as any`/`@ts-ignore`, empty catches, console.log in prod, commented-out code, unused imports. Check AI slop: excessive comments, over-abstraction, generic names. Verify test scripts follow bash best practices (set -euo pipefail, proper quoting, no word splitting bugs).
  Output: `Build [PASS/FAIL] | Lint [PASS/FAIL] | Files [N clean/N issues] | VERDICT`

- [x] F3. **Real Manual QA** — `unspecified-high`
  Start from clean state. Execute EVERY QA scenario from EVERY task — follow exact steps, capture evidence. Test cross-script integration (all scripts pass on same VPS deployment). Test edge cases: missing socat, bridge in different transport mode. Save to `.sisyphus/evidence/final-qa/`.
  Output: `Scenarios [N/N pass] | Integration [N/N] | Edge Cases [N tested] | VERDICT`

- [x] F4. **Scope Fidelity Check** — `deep`
  For each task: read "What to do", read actual diff (git log/diff). Verify 1:1 — everything in spec was built (no missing), nothing beyond spec was built (no creep). Check "Must NOT do" compliance. Verify only bridge/cmd/bridge/main.go and tests/ were modified. Flag unaccounted changes.
  Output: `Tasks [N/N compliant] | Contamination [CLEAN/N issues] | Unaccounted [CLEAN/N files] | VERDICT`

---

## Commit Strategy

- **Commit 1**: `fix(tests): disable discovery in T2 temp config to avoid port 8080 conflict` — tests/test-governance-rpc.sh
- **Commit 2**: `fix(tests): rewrite T4 Matrix send/receive to use direct Conduit API curl` — tests/test-matrix-plane.sh
- **Commit 3**: `feat(tests): add dual transport to T3 with Unix socket + HTTP auto-detection` — tests/test-vps-smoke.sh
- **Commit 4**: `fix(bridge): wire EventBroadcaster for WebSocket event distribution` — bridge/cmd/bridge/main.go
- **Commit 5**: `test(persistence): add invite lifecycle test across bridge restart` — tests/test-persistence.sh

---

## Success Criteria

### Verification Commands
```bash
# T2 fix: discovery disabled in temp config
grep -A1 '\[discovery\]' tests/test-governance-rpc.sh
# Expected: "enabled = false"

# T4 fix: no matrix-commander for send
grep -c 'mc.*--message' tests/test-matrix-plane.sh
# Expected: 0

# T3 fix: dual transport functions exist
grep -c 'rpc_socket\|rpc_http\|detect_transport' tests/test-vps-smoke.sh
# Expected: ≥ 3

# T4 code fix: SetBroadcaster wired
grep -c 'SetBroadcaster' bridge/cmd/bridge/main.go
# Expected: 1

# T5: persistence script exists
test -f tests/test-persistence.sh && echo "EXISTS"
# Expected: EXISTS

# All scripts executable
ls -la tests/test-*.sh | awk '{print $1}'
# Expected: all show -rwxr-xr-x

# VPS bridge running new binary
ssh -i ~/.ssh/openclaw_win root@5.183.11.149 "systemctl is-active armorclaw-bridge"
# Expected: active
```

### Final Checklist
- [ ] All "Must Have" present
- [ ] All "Must NOT Have" absent
- [ ] All test scripts pass on VPS
- [ ] Bridge boots cleanly with websocket_enabled=true (if tested)
- [ ] Persistence test confirms state survives restart
- [ ] No files modified outside tests/ and bridge/cmd/bridge/main.go
