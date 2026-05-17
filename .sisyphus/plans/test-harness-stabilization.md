# Post-Deploy Test Harness Stabilization + Bridge Coverage Expansion

## TL;DR

> **Quick Summary**: Fix the VPS test harness (Docker transport detection, ADMIN_TOKEN, result categories) and close zero-test gaps in 5 high-risk Bridge packages (security, websocket, matrix, executor, socket).
>
> **Deliverables**:
> - Shared bridge transport detector (`tests/lib/transport.sh`)
> - Docker-aware `check_bridge_running()` and `restart_bridge()`
> - Result categories extended to PASS/FAIL/SKIP/GATED_EXPECTED/ENV_MISSING
> - `matrix.status` contract defined and asserted
> - Go test suites for `pkg/security`, `pkg/websocket`, `pkg/matrix`, `internal/executor`, `pkg/socket`
> - 14 harness scripts migrated to shared transport detector
> - ADMIN_TOKEN + BRIDGE_SOCKET configured on VPS
>
> **Estimated Effort**: XL
> **Parallel Execution**: YES - 5 waves
> **Critical Path**: T1(transport lib) → T3(patch scripts) → T13(VPS deploy) → T22-26(Go tests) → F1-F4

---

## Context

### Original Request
User's rewritten plan critique identified that the post-deploy validation report mixed two separate problems: (1) VPS shell harness infrastructure (systemd vs Docker detection, missing ADMIN_TOKEN, missing BRIDGE_SOCKET) and (2) Bridge control-plane spine with zero-test high-risk packages. The plan must fix both streams independently.

### Interview Summary
**Key Discussions**:
- Voice returning "feature disabled" is EXPECTED behavior, not a product defect — must be GATED_EXPECTED, not FAIL
- `matrix.status` returns null on VPS but shell test expects a real response — contract ambiguity, not necessarily a bug
- `pkg/security` has 1005 lines of access control logic with zero tests and HIGH mockability — highest ROI
- 20+ scripts duplicate transport detection code — consolidation target
- Test result categories need GATED_EXPECTED and ENV_MISSING to distinguish real failures from expected states

**Research Findings**:
- `tests/lib/load_env.sh:57-62` — `check_bridge_running()` is SYSTEMD-ONLY
- `tests/e2e/common.sh:218-232` — `rpc_call()` is SOCKET-ONLY via socat
- `tests/lib/restart_bridge.sh:26` — `systemctl restart` only
- NO shared transport detector exists anywhere in `tests/lib/`
- 3 canonical detection patterns duplicated across 20+ scripts
- Auth injection differs: socket puts `"auth":"$ADMIN_TOKEN"` in JSON body; HTTP uses `Authorization: Bearer` header
- 5 zero-test Go packages totaling 3100 lines of production code

### Metis Review
**Identified Gaps** (addressed):
- `pkg/security` underweighted in original plan — now Wave 4 priority 1
- Voice mis-prioritized as product defect — now explicitly GATED_EXPECTED
- `matrix.status` null response ambiguity — now Wave 2 contract definition task
- Wave 0 re-baseline missing — now first wave before any changes
- Missing ADMIN_TOKEN on VPS conflated with script bugs — now separate W1.2 task

---

## Work Objectives

### Core Objective
Convert the VPS validation suite from "partially trustworthy" to "release-gating" while closing zero-test gaps in the Bridge control plane.

### Concrete Deliverables
- `tests/lib/transport.sh` — shared Docker-aware bridge transport detector
- `tests/lib/common_output.sh` updated with GATED_EXPECTED and ENV_MISSING result categories
- `tests/lib/load_env.sh` updated with Docker-aware `check_bridge_running()`
- `tests/lib/restart_bridge.sh` updated with Docker container restart support
- `bridge/pkg/security/categories_test.go` + `website_guard_test.go`
- `bridge/pkg/websocket/websocket_test.go`
- `bridge/pkg/matrix/client_test.go`
- `bridge/internal/executor/engine_test.go`
- `bridge/pkg/socket/server_test.go`
- `test-matrix-integration.sh` rewritten with correct contract assertion
- VPS `.env` configured with ADMIN_TOKEN and BRIDGE_SOCKET

### Definition of Done
- [ ] `bash tests/test-vps-smoke.sh` produces 0 false failures on Docker-hosted Bridge
- [ ] `go test ./pkg/security/... ./pkg/websocket/... ./pkg/matrix/... ./internal/executor/... ./pkg/socket/...` — all PASS
- [ ] All 14 migrated harness scripts detect Docker transport correctly
- [ ] `matrix.status` returns documented contract on live VPS
- [ ] Voice/e2ee/keystore feature-gated methods report GATED_EXPECTED, not FAIL

### Must Have
- Shared transport detector in `tests/lib/transport.sh`
- Docker-aware `check_bridge_running()` in `tests/lib/load_env.sh`
- GATED_EXPECTED result category in `tests/lib/common_output.sh`
- At least 1 test file per zero-test package (security, websocket, matrix, executor, socket)
- `matrix.status` contract documented and tested
- ADMIN_TOKEN configured on VPS

### Must NOT Have (Guardrails)
- Do NOT modify `container-setup.sh` or `deploy/container-setup.sh`
- Do NOT touch `bridge/pkg/yara/scanner.go` internal logic
- Do NOT add structured logging library
- Do NOT change production handler behavior to fix tests — fix assertions instead
- Do NOT treat feature-gated voice/e2ee/keystore responses as product bugs
- Do NOT refactor production code to make it testable — only add test files
- Do NOT modify `test-element-x-flow.sh` (out of scope)
- Do NOT add new RPC methods
- Do NOT break existing passing tests
- Do NOT change AGENTS.md security constraints

---

## Verification Strategy (MANDATORY)

> **ZERO HUMAN INTERVENTION** - ALL verification is agent-executed.

### Test Decision
- **Infrastructure exists**: YES (shell harness + Go test framework)
- **Automated tests**: YES (Tests-after for Go packages, harness fix for shell scripts)
- **Framework**: Go `testing` + bash shell test scripts

### QA Policy
Every task includes agent-executed QA scenarios.
Evidence saved to `.sisyphus/evidence/task-{N}-{scenario-slug}.{ext}`.

- **Shell scripts**: Bash (syntax check + sourcing test)
- **Go tests**: `go test -v -run` + `go vet`
- **VPS deployment**: SSH + curl/socat for live verification

---

## Execution Strategy

### Parallel Execution Waves

```
Wave 0 (Start Immediately — foundation):
├── T1: Create tests/lib/transport.sh (shared detector) [quick]
├── T2: Extend result categories in common_output.sh [quick]
└── T3: Update load_env.sh + restart_bridge.sh for Docker [quick]

Wave 1 (After Wave 0 — migrate harness scripts):
├── T4: Patch Tier A scripts (10 scripts) [unspecified-high]
├── T5: Patch Tier C cross-subsystem scripts (6 scripts) [unspecified-high]
└── T6: Fix test-matrix-integration.sh contract [deep]

Wave 2 (After Wave 1 — VPS deploy + restore coverage):
├── T7: Configure VPS env (ADMIN_TOKEN + BRIDGE_SOCKET) [quick]
├── T8: Sync missing scripts to VPS [quick]
└── T9: Run full validation suite on VPS [unspecified-high]

Wave 3 (Parallel with Wave 2 — Go test suites):
├── T10: pkg/security tests (categories + website_guard) [deep]
├── T11: pkg/websocket tests [quick]
├── T12: pkg/matrix tests (httptest-based) [deep]
├── T13: internal/executor tests (ToolPool + ToolExecutor) [deep]
└── T14: pkg/socket tests (JSON-RPC handlers) [unspecified-high]

Wave FINAL (After ALL tasks):
├── F1: Plan compliance audit (oracle)
├── F2: Code quality review (unspecified-high)
├── F3: Harness correctness verification (unspecified-high)
└── F4: Scope fidelity check (deep)
→ Present results → Get explicit user okay

Critical Path: T1 → T4/T5 → T7/T8 → T9 → F1-F4
Parallel Speedup: Wave 3 fully parallel (5 Go test tasks)
Max Concurrent: 5 (Wave 3)
```

### Dependency Matrix

| Task | Depends On | Blocks | Wave |
|------|-----------|--------|------|
| T1 | - | T4, T5, T6 | 0 |
| T2 | - | T4, T5 | 0 |
| T3 | - | T4, T5, T9 | 0 |
| T4 | T1, T2 | T9 | 1 |
| T5 | T1, T2 | T9 | 1 |
| T6 | T1 | T9 | 1 |
| T7 | - | T9 | 2 |
| T8 | - | T9 | 2 |
| T9 | T4, T5, T6, T7, T8 | F1-F4 | 2 |
| T10 | - | F1-F4 | 3 |
| T11 | - | F1-F4 | 3 |
| T12 | - | F1-F4 | 3 |
| T13 | - | F1-F4 | 3 |
| T14 | - | F1-F4 | 3 |

### Agent Dispatch Summary

- **Wave 0**: 3 tasks — T1 `quick`, T2 `quick`, T3 `quick`
- **Wave 1**: 3 tasks — T4 `unspecified-high`, T5 `unspecified-high`, T6 `deep`
- **Wave 2**: 3 tasks — T7 `quick`, T8 `quick`, T9 `unspecified-high`
- **Wave 3**: 5 tasks — T10 `deep`, T11 `quick`, T12 `deep`, T13 `deep`, T14 `unspecified-high`
- **FINAL**: 4 tasks — F1 `oracle`, F2 `unspecified-high`, F3 `unspecified-high`, F4 `deep`

---

## TODOs

- [x] 1. Create `tests/lib/transport.sh` — shared Docker-aware bridge transport detector

  **What to do**:
  - Create `tests/lib/transport.sh` as a new shared library
  - Export `detect_transport()` function that checks in order:
    1. Explicit env override: `BRIDGE_TRANSPORT` (socket|http|both|none)
    2. Socket mode: `test -S $BRIDGE_SOCKET` (default `/run/armorclaw/bridge.sock`)
    3. Docker health: `docker ps --filter name=armorclaw --format '{{.Status}}'` as advisory
    4. HTTP health: `curl -sf http://localhost:${BRIDGE_PORT:-8080}/health`
  - Export `rpc_call()` that auto-selects socket or HTTP based on detected transport
    - Socket: `echo payload | socat - UNIX-CONNECT:$BRIDGE_SOCKET`
    - HTTP: `curl -sf http://localhost:${BRIDGE_PORT}/api -H "Authorization: Bearer $ADMIN_TOKEN"`
  - Export `rpc_call_auth()` that injects auth correctly per transport:
    - Socket: `"auth":"$ADMIN_TOKEN"` in JSON-RPC body
    - HTTP: `Authorization: Bearer $ADMIN_TOKEN` header
  - Export `require_bridge()` that exits with ENV_MISSING if no transport found
  - Export `optional_bridge()` that sets `BRIDGE_AVAILABLE=false` and returns 1 (no exit)
  - Set `BRIDGE_SOCKET` default to `/run/armorclaw/bridge.sock`
  - Set `BRIDGE_HTTP_URL` default to `http://localhost:${BRIDGE_PORT:-8080}`
  - Print detected mode on first call (info logging)
  - Pattern source: `test-vps-smoke.sh` detect_transport (most complete existing implementation)

  **Must NOT do**:
  - Do NOT modify any existing test scripts in this task (T4/T5 handle migration)
  - Do NOT add dependencies beyond socat, curl, jq (already in CI image)
  - Do NOT change behavior of existing `tests/e2e/common.sh` rpc_call

  **Recommended Agent Profile**:
  - **Category**: `quick`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 0 (with T2, T3)
  - **Blocks**: T4, T5, T6
  - **Blocked By**: None

  **References**:
  **Pattern References**:
  - `tests/test-vps-smoke.sh:30-90` — Most complete existing `detect_transport()` with HAS_SOCKET/HAS_HTTP flags and TRANSPORT_MODE
  - `tests/test-persistence.sh:40-80` — Alternative detect_transport with BRIDGE_SOCKET env support
  - `tests/test-trust-layer.sh:50-100` — Typical `rpc_http()` + `rpc_socket()` + unified `rpc_call()` pattern
  - `tests/test-provisioning.sh:40-90` — detect_transport with SSH-based socket check

  **API/Type References**:
  - `tests/lib/load_env.sh` — Env vars to respect: VPS_IP, VPS_USER, BRIDGE_PORT, ADMIN_TOKEN, SSH_KEY_PATH
  - `tests/e2e/common.sh:218-232` — Existing `rpc_call()` to remain compatible with (socket path param)

  **Acceptance Criteria**:
  - [ ] `bash -n tests/lib/transport.sh` — syntax OK
  - [ ] `source tests/lib/transport.sh; type detect_transport` — function exists
  - [ ] `source tests/lib/transport.sh; type rpc_call` — function exists
  - [ ] `source tests/lib/transport.sh; type rpc_call_auth` — function exists

  **QA Scenarios**:

  ```
  Scenario: Library sources without error
    Tool: Bash
    Preconditions: tests/lib/transport.sh exists
    Steps:
      1. bash -n tests/lib/transport.sh
      2. source tests/lib/transport.sh
      3. type detect_transport rpc_call rpc_call_auth require_bridge optional_bridge
    Expected Result: All 5 functions found, exit code 0
    Evidence: .sisyphus/evidence/task-1-source-test.txt

  Scenario: Detect transport with BRIDGE_TRANSPORT=socket override
    Tool: Bash
    Preconditions: BRIDGE_SOCKET env set to existing file
    Steps:
      1. BRIDGE_TRANSPORT=socket BRIDGE_SOCKET=/run/armorclaw/bridge.sock source tests/lib/transport.sh
      2. detect_transport
      3. echo $TRANSPORT_MODE
    Expected Result: TRANSPORT_MODE contains "socket"
    Evidence: .sisyphus/evidence/task-1-socket-override.txt
  ```

  **Commit**: YES (groups with T2, T3)
  - Message: `fix(tests): add shared transport detector and normalize result categories`

- [x] 2. Extend result categories in `tests/lib/common_output.sh`

  **What to do**:
  - Add `FULL_SYSTEM_GATED_EXPECTED` counter (initial 0)
  - Add `FULL_SYSTEM_ENV_MISSING` counter (initial 0)
  - Add `log_gated_expected()` — increments counter, prints `[GATED]` in CYAN
  - Add `log_env_missing()` — increments counter, prints `[ENV_MISSING]` in MAGENTA
  - Update `harness_summary()` to print all 5 categories
  - Update return logic: 0 if FAILED==0 (GATED_EXPECTED and ENV_MISSING are not failures)

  **Must NOT do**:
  - Do NOT change existing PASS/FAIL/SKIP behavior
  - Do NOT change existing counter variable names

  **Recommended Agent Profile**:
  - **Category**: `quick`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 0 (with T1, T3)
  - **Blocks**: T4, T5
  - **Blocked By**: None

  **References**:
  **Pattern References**:
  - `tests/lib/common_output.sh` — Current PASS/FAIL/SKIP implementation (61 lines)
  - `tests/test-exploits.sh` — Has custom EXPECTED_FAILURE/EXPECTED_CONTAINMENT labels (reference for category design)

  **Acceptance Criteria**:
  - [ ] `bash -n tests/lib/common_output.sh` — syntax OK
  - [ ] `source tests/lib/common_output.sh; type log_gated_expected` — function exists
  - [ ] `source tests/lib/common_output.sh; type log_env_missing` — function exists
  - [ ] `harness_summary` output includes "Gated:" and "Env Missing:" lines

  **QA Scenarios**:

  ```
  Scenario: New result categories work
    Tool: Bash
    Steps:
      1. source tests/lib/common_output.sh
      2. log_gated_expected "voice intentionally disabled"
      3. log_env_missing "ADMIN_TOKEN not set"
      4. harness_summary
    Expected Result: Output shows "Gated: 1" and "Env Missing: 1", return code 0
    Evidence: .sisyphus/evidence/task-2-result-categories.txt
  ```

  **Commit**: YES (groups with T1, T3)

- [x] 3. Update `load_env.sh` + `restart_bridge.sh` for Docker awareness

  **What to do**:
  - In `tests/lib/load_env.sh`:
    - Replace `check_bridge_running()` (lines 56-63) with Docker-aware version:
      1. Check `docker ps --filter name=armorclaw --format '{{.Status}}'` — if "healthy" or "Up", return 0
      2. Fallback: check `systemctl is-active armorclaw-bridge.service` (preserve existing)
      3. Fallback: check socket file exists `test -S /run/armorclaw/bridge.sock`
    - Add `BRIDGE_SOCKET` env var with default `/run/armorclaw/bridge.sock`
    - Add `BRIDGE_TRANSPORT` env var documentation comment
  - In `tests/lib/restart_bridge.sh`:
    - Replace systemd-only `systemctl restart` (line 26) with:
      1. Try `docker restart armorclaw` first
      2. Fallback: `systemctl restart armorclaw-bridge.service`
    - Replace health poll `systemctl is-active` (line 36) with:
      1. `curl -sf http://localhost:${BRIDGE_PORT:-8080}/health`
      2. Fallback: `systemctl is-active`
    - Add `BRIDGE_SOCKET` to readiness poll: `test -S $BRIDGE_SOCKET`

  **Must NOT do**:
  - Do NOT remove systemctl checks (must support both Docker and systemd deployments)
  - Do NOT change function signatures or return codes

  **Recommended Agent Profile**:
  - **Category**: `quick`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 0 (with T1, T2)
  - **Blocks**: T4, T5, T9
  - **Blocked By**: None

  **References**:
  **Pattern References**:
  - `tests/lib/load_env.sh:56-63` — Current systemd-only `check_bridge_running()`
  - `tests/lib/restart_bridge.sh:26,36` — Current systemd-only restart and poll

  **Acceptance Criteria**:
  - [ ] `bash -n tests/lib/load_env.sh` — syntax OK
  - [ ] `bash -n tests/lib/restart_bridge.sh` — syntax OK
  - [ ] `check_bridge_running` detects Docker container when `docker ps` shows armorclaw running
  - [ ] `restart_bridge` uses `docker restart` when Docker container detected

  **QA Scenarios**:

  ```
  Scenario: check_bridge_running detects Docker
    Tool: Bash (SSH to VPS)
    Preconditions: armorclaw Docker container running on VPS
    Steps:
      1. source tests/lib/load_env.sh
      2. check_bridge_running
      3. echo $?
    Expected Result: Exit code 0
    Evidence: .sisyphus/evidence/task-3-docker-detect.txt

  Scenario: restart_bridge uses Docker
    Tool: Bash (SSH to VPS)
    Preconditions: armorclaw Docker container running on VPS
    Steps:
      1. source tests/lib/load_env.sh
      2. source tests/lib/restart_bridge.sh
      3. restart_bridge 30
    Expected Result: "Restarting armorclaw container..." message, bridge becomes healthy
    Evidence: .sisyphus/evidence/task-3-docker-restart.txt
  ```

  **Commit**: YES (groups with T1, T2)

- [x] 4. Patch Tier A harness scripts to use shared transport detector

  **What to do**:
  - Migrate these 10 scripts to `source tests/lib/transport.sh`:
    1. `test-system-health-baseline.sh` — replace `check_bridge_running()` call with `detect_transport`
    2. `test-trust-layer.sh` — replace local `rpc_http()`/`rpc_socket()`/`detect_transport()` with sourced lib
    3. `test-secretary-workflow-core.sh` — replace local transport code
    4. `test-email-pipeline.sh` — replace local transport code
    5. `test-license-enforcement.sh` — replace local transport code
    6. `test-sidecar-docs.sh` — replace local transport code
    7. `test-jetski-sidecar.sh` — replace local transport code
    8. `test-voice-stack.sh` — replace local `voice_rpc()` with `rpc_call_auth`; change voice "disabled" to `log_gated_expected`
    9. `test-platform-adapters.sh` — replace local HAS_SOCKET/HAS_HTTP with `detect_transport`
    10. `test-eventbus-streaming.sh` — replace `check_bridge_running()` with `detect_transport`
  - For each script:
    - Remove local `rpc_http()`, `rpc_socket()`, `detect_transport()`, `rpc_call()` function definitions
    - Add `source "$(dirname "$0")/../lib/transport.sh"` after existing lib sources
    - Replace `log_fail "ADMIN_TOKEN not set"` with `log_env_missing "ADMIN_TOKEN not set"`
    - Replace feature-disabled checks (voice, e2ee, keystore) with `log_gated_expected`

  **Must NOT do**:
  - Do NOT change test assertions or test logic
  - Do NOT change the order of test scenarios
  - Do NOT modify scripts not in the list above

  **Recommended Agent Profile**:
  - **Category**: `unspecified-high`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES (with T5, T6)
  - **Parallel Group**: Wave 1
  - **Blocks**: T9
  - **Blocked By**: T1, T2

  **References**:
  **Pattern References**:
  - `tests/test-trust-layer.sh:50-100` — Typical local dual-transport code to remove
  - `tests/test-platform-adapters.sh:20-50` — HAS_SOCKET/HAS_HTTP pattern to replace
  - `tests/test-voice-stack.sh` — voice_rpc() to replace; feature-gated responses to convert to log_gated_expected
  - `tests/lib/transport.sh` — New shared library (from T1)

  **Acceptance Criteria**:
  - [ ] All 10 scripts pass `bash -n` syntax check
  - [ ] All 10 scripts source `tests/lib/transport.sh`
  - [ ] No script contains local `rpc_http()` or `rpc_socket()` function definitions
  - [ ] Voice/e2ee/keystore feature-disabled outcomes use `log_gated_expected` not `log_fail`

  **QA Scenarios**:

  ```
  Scenario: Migrated scripts syntax-check clean
    Tool: Bash
    Steps:
      1. for f in test-system-health-baseline test-trust-layer test-secretary-workflow-core test-email-pipeline test-license-enforcement test-sidecar-docs test-jetski-sidecar test-voice-stack test-platform-adapters test-eventbus-streaming; do bash -n tests/$f.sh && echo "OK: $f"; done
    Expected Result: All 10 print "OK"
    Evidence: .sisyphus/evidence/task-4-syntax-check.txt

  Scenario: No duplicate transport functions
    Tool: Bash
    Steps:
      1. grep -l "rpc_http()" tests/test-system-health-baseline.sh tests/test-trust-layer.sh tests/test-secretary-workflow-core.sh tests/test-email-pipeline.sh tests/test-license-enforcement.sh tests/test-sidecar-docs.sh tests/test-jetski-sidecar.sh tests/test-voice-stack.sh tests/test-platform-adapters.sh tests/test-eventbus-streaming.sh || echo "CLEAN"
    Expected Result: "CLEAN" (no local rpc_http definitions found)
    Evidence: .sisyphus/evidence/task-4-no-dupes.txt
  ```

  **Commit**: YES (groups with T5)
  - Message: `fix(tests): migrate 16 harness scripts to shared transport detector`

- [x] 5. Patch Tier C cross-subsystem scripts to use shared transport detector

  **What to do**:
  - Migrate these 6 cross-subsystem scripts to `source tests/lib/transport.sh`:
    1. `test-cross-browser-trust.sh`
    2. `test-cross-event-truth.sh`
    3. `test-cross-workflow-docs.sh`
    4. `test-cross-workflow-email.sh`
    5. `test-navchart-security.sh`
    6. `test-navchart-pipeline.sh`
  - Same pattern as T4: remove local transport code, source shared lib, convert ADMIN_TOKEN skips to `log_env_missing`

  **Must NOT do**:
  - Do NOT change test logic or assertions
  - Do NOT modify scripts not in the list

  **Recommended Agent Profile**:
  - **Category**: `unspecified-high`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES (with T4, T6)
  - **Parallel Group**: Wave 1
  - **Blocks**: T9
  - **Blocked By**: T1, T2

  **References**:
  **Pattern References**:
  - `tests/test-cross-browser-trust.sh` — Cross-subsystem script with local rpc_http/rpc_socket
  - `tests/lib/transport.sh` — New shared library (from T1)

  **Acceptance Criteria**:
  - [ ] All 6 scripts pass `bash -n` syntax check
  - [ ] All 6 scripts source `tests/lib/transport.sh`
  - [ ] No script contains local `rpc_http()` or `rpc_socket()` definitions

  **QA Scenarios**:

  ```
  Scenario: Cross-subsystem scripts syntax-check clean
    Tool: Bash
    Steps:
      1. for f in test-cross-browser-trust test-cross-event-truth test-cross-workflow-docs test-cross-workflow-email test-navchart-security test-navchart-pipeline; do bash -n tests/$f.sh && echo "OK: $f"; done
    Expected Result: All 6 print "OK"
    Evidence: .sisyphus/evidence/task-5-syntax-check.txt
  ```

  **Commit**: YES (groups with T4)

- [x] 6. Define `matrix.status` contract and fix integration test

  **What to do**:
  - Read `bridge/pkg/rpc/server.go` to find the `matrix.status` handler and understand its response when adapter is nil vs active
  - Define the canonical contract as a comment at the top of `test-matrix-integration.sh`:
    ```
    # matrix.status contract:
    # When adapter is active: {"connected":bool, "logged_in":bool, ...}
    # When adapter is nil: {"status":"not_configured","enabled":false}
    # When error: {"error":{"code":-32xxx,"message":"..."}}
    ```
  - Rewrite `test-matrix-integration.sh` to:
    1. Source `tests/lib/transport.sh` (from T1)
    2. Assert `matrix.status` returns a valid JSON response (not necessarily connected)
    3. Handle null response explicitly — if null, `log_gated_expected "Matrix adapter not initialized"`
    4. If non-null, validate response structure has expected fields
  - Add a comment in `bridge/pkg/rpc/server.go` near the `matrix.status` handler documenting the expected response shape

  **Must NOT do**:
  - Do NOT change the handler's behavior — fix the assertion, not the code
  - Do NOT add new RPC methods

  **Recommended Agent Profile**:
  - **Category**: `deep`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES (with T4, T5)
  - **Parallel Group**: Wave 1
  - **Blocks**: T9
  - **Blocked By**: T1

  **References**:
  **Pattern References**:
  - `tests/test-matrix-integration.sh` — Current test that expects real response and gets null
  - `bridge/pkg/rpc/server.go:919-925` — Matrix handler that returns status

  **Acceptance Criteria**:
  - [ ] `test-matrix-integration.sh` sources `tests/lib/transport.sh`
  - [ ] Contract documented in test file header
  - [ ] Test handles null response as `GATED_EXPECTED` instead of FAIL
  - [ ] `bash -n tests/test-matrix-integration.sh` — syntax OK

  **QA Scenarios**:

  ```
  Scenario: Matrix integration test handles null gracefully
    Tool: Bash (SSH to VPS)
    Preconditions: Bridge running on VPS
    Steps:
      1. VPS_IP=5.183.11.149 ADMIN_TOKEN=xxx bash tests/test-matrix-integration.sh
      2. Check output for GATED_EXPECTED or PASS, not FAIL
    Expected Result: No FAIL, either PASS or GATED_EXPECTED for matrix.status
    Evidence: .sisyphus/evidence/task-6-matrix-test.txt
  ```

  **Commit**: YES
  - Message: `fix(tests): define matrix.status contract and fix integration test`

- [x] 7. Configure VPS env (ADMIN_TOKEN + BRIDGE_SOCKET)

  **What to do**:
  - SSH to VPS (5.183.11.149)
  - Generate a strong ADMIN_TOKEN: `openssl rand -hex 32`
  - Add to `/opt/armorclaw/.env`:
    ```
    ADMIN_TOKEN=<generated-token>
    BRIDGE_SOCKET=/run/armorclaw/bridge.sock
    BRIDGE_TRANSPORT=socket
    ```
  - Inject ADMIN_TOKEN into bridge config: add `admin_token = "<token>"` to `/etc/armorclaw/config.toml` `[server]` section (inside the container)
  - Restart bridge container: `docker restart armorclaw`
  - Verify: `echo '{"jsonrpc":"2.0","id":1,"method":"bridge.status","params":{"auth":"<token>"}}' | socat - UNIX-CONNECT:/run/armorclaw/bridge.sock` returns valid response

  **Must NOT do**:
  - Do NOT log the token value in any committed file
  - Do NOT expose the token in git history

  **Recommended Agent Profile**:
  - **Category**: `quick`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES (with T8)
  - **Parallel Group**: Wave 2
  - **Blocks**: T9
  - **Blocked By**: None

  **References**:
  - `bridge/pkg/config/config.go` — `AdminToken` field, `toml:"admin_token"` tag
  - VPS: 5.183.11.149, user root, SSH key `~/.ssh/openclaw_win`

  **Acceptance Criteria**:
  - [ ] `/opt/armorclaw/.env` on VPS contains ADMIN_TOKEN, BRIDGE_SOCKET, BRIDGE_TRANSPORT
  - [ ] Bridge container restarted and healthy
  - [ ] RPC call with auth token returns valid response

  **QA Scenarios**:

  ```
  Scenario: Admin token configured on VPS
    Tool: Bash (SSH to VPS)
    Steps:
      1. ssh root@5.183.11.149 "grep ADMIN_TOKEN /opt/armorclaw/.env | wc -c"
      2. ssh root@5.183.11.149 "docker ps --filter name=armorclaw --format '{{.Status}}'"
    Expected Result: ADMIN_TOKEN line has content, container shows "Up" and "healthy"
    Evidence: .sisyphus/evidence/task-7-vps-env.txt
  ```

  **Commit**: NO (VPS env change, not code)

- [x] 8. Sync missing test scripts to VPS

  **What to do**:
  - SSH to VPS and run `cd /opt/armorclaw && git pull origin main` to get latest test scripts
  - Verify `tests/test-studio-lifecycle.sh` exists on VPS
  - Verify all patched scripts (from T4, T5, T6) are present
  - Install `socat` if missing: `apt-get install -y socat`

  **Must NOT do**:
  - Do NOT modify scripts on VPS directly — all changes come from git

  **Recommended Agent Profile**:
  - **Category**: `quick`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES (with T7)
  - **Parallel Group**: Wave 2
  - **Blocks**: T9
  - **Blocked By**: None

  **References**:
  - VPS: 5.183.11.149, `/opt/armorclaw/` repo

  **Acceptance Criteria**:
  - [ ] `test-studio-lifecycle.sh` exists on VPS
  - [ ] `socat` installed on VPS
  - [ ] Git HEAD on VPS matches local HEAD (within 1 commit)

  **QA Scenarios**:

  ```
  Scenario: VPS test scripts up to date
    Tool: Bash (SSH)
    Steps:
      1. ssh root@5.183.11.149 "cd /opt/armorclaw && git log -1 --format='%h %s' && ls tests/test-studio-lifecycle.sh"
    Expected Result: Latest commit hash shown, test-studio-lifecycle.sh exists
    Evidence: .sisyphus/evidence/task-8-vps-sync.txt
  ```

  **Commit**: NO (VPS deployment, not code)

- [x] 9. Run full validation suite on VPS and capture results

  **What to do**:
  - SSH to VPS and run ALL test scripts with ADMIN_TOKEN configured
  - Run Tier A scripts: `for f in test-trust-layer test-system-health-baseline test-secretary-workflow-core test-eventbus-streaming test-email-pipeline test-voice-stack test-sidecar-docs test-jetski-sidecar test-license-enforcement test-platform-adapters test-agent-runtime test-studio-lifecycle test-discovery test-rpc-methods; do bash tests/$f.sh >> /tmp/test-results.txt 2>&1; done`
  - Run Tier C scripts: `for f in test-cross-browser-trust test-cross-event-truth test-cross-workflow-docs test-cross-workflow-email; do bash tests/$f.sh >> /tmp/test-results.txt 2>&1; done`
  - Run matrix integration: `bash tests/test-matrix-integration.sh >> /tmp/test-results.txt 2>&1`
  - Capture and save full output to `.sisyphus/evidence/task-9-vps-validation.txt`
  - Parse results: count PASS, FAIL, SKIP, GATED_EXPECTED, ENV_MISSING

  **Must NOT do**:
  - Do NOT fix failures in this task — just capture results
  - Do NOT modify any scripts on VPS

  **Recommended Agent Profile**:
  - **Category**: `unspecified-high`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: NO (depends on T4-T8)
  - **Parallel Group**: Sequential (after Wave 1 + Wave 2)
  - **Blocks**: F1-F4
  - **Blocked By**: T4, T5, T6, T7, T8

  **References**:
  - VPS: 5.183.11.149
  - All test scripts in `/opt/armorclaw/tests/`

  **Acceptance Criteria**:
  - [ ] Full test output saved to `.sisyphus/evidence/task-9-vps-validation.txt`
  - [ ] Result summary documented: PASS/FAIL/SKIP/GATED_EXPECTED/ENV_MISSING counts
  - [ ] 0 false failures (Docker detection, systemd checks)

  **QA Scenarios**:

  ```
  Scenario: VPS validation runs to completion
    Tool: Bash (SSH)
    Steps:
      1. ssh root@5.183.11.149 "cd /opt/armorclaw && wc -l /tmp/test-results.txt"
      2. Parse for FAIL count
    Expected Result: Results file has content, FAIL count = 0 real failures
    Evidence: .sisyphus/evidence/task-9-vps-validation.txt
  ```

  **Commit**: NO (evidence capture only)

- [x] 10. Add `pkg/security` tests — categories + website_guard

  **What to do**:
  - Create `bridge/pkg/security/categories_test.go`:
    - `TestAllCategories` — verify AllCategories returns 8 categories
    - `TestNewSecurityConfigDefaults` — verify default tier is balanced
    - `TestSetTierParanoid` — all data categories blocked
    - `TestSetTierOpen` — all data categories allowed
    - `TestIsDataAllowed_BlockedList` — blocked domain takes priority
    - `TestIsDataAllowed_AllowlistFallback` — allowlist permits when not blocked
    - `TestIsDataAllowed_PermissionAllowAll` — PermissionAllowAll bypasses all checks
    - `TestCategoryConfig_Clone` — deep copy produces independent copy
    - `TestLoadSecurityConfig_MissingFile` — fallback to defaults
    - `TestGetCategory_InvalidReturnsNil` — unknown category returns zero-value
    - `TestToJSON` — produces valid JSON output
  - Create `bridge/pkg/security/website_guard_test.go`:
    - `TestNewWebsiteGuard` — empty guard permits nothing by default
    - `TestCheckAccess_AllowedDomain` — allowed domain returns permitted
    - `TestCheckAccess_BlockedDomain` — blocked domain returns denied
    - `TestValidateURL_HTTPSRequired` — HTTP URLs rejected
    - `TestValidateURL_SuspiciousPastebin` — pastebin/ngrok URLs flagged
    - `TestExtractDomain_WithPort` — strips port
    - `TestExtractDomain_Subdomain` — extracts correct domain
    - `TestAllowlist_MatchDomain` — exact and wildcard matching
    - `TestAllowlist_AddRemoveDomain` — list mutation
    - Mock `AuditLogger` interface using struct with function fields (existing codebase pattern)

  **Must NOT do**:
  - Do NOT modify production code in `categories.go` or `website_guard.go`
  - Do NOT add testify or gomock — use existing codebase mock pattern (struct with function fields)

  **Recommended Agent Profile**:
  - **Category**: `deep`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 3 (with T11-T14)
  - **Blocks**: F1-F4
  - **Blocked By**: None

  **References**:
  **Pattern References**:
  - `bridge/pkg/agent/agent_test.go` — Existing mock pattern with struct function fields
  - `bridge/pkg/secretary/blocker_test_helper.go` — Shared test helper pattern

  **API/Type References**:
  - `bridge/pkg/security/categories.go` — `SecurityConfig`, `CategoryConfig`, `DataCategory` constants, `AllCategories()`, `NewSecurityConfig()`, `SetTier()`, `IsDataAllowed()`, `Clone()`
  - `bridge/pkg/security/website_guard.go` — `WebsiteGuard`, `AuditLogger` interface, `CheckAccess()`, `ValidateURL()`, `ExtractDomain()`, `WebsiteAllowlist`
  - `bridge/pkg/security/website_guard.go:20-30` — `AuditLogger` interface definition (mockable)

  **Acceptance Criteria**:
  - [ ] `go test -v ./pkg/security/...` — all tests PASS
  - [ ] `go vet ./pkg/security/...` — no issues
  - [ ] Both `categories_test.go` and `website_guard_test.go` created
  - [ ] At least 10 test functions per file

  **QA Scenarios**:

  ```
  Scenario: Security tests pass
    Tool: Bash
    Steps:
      1. cd bridge && go test -v -count=1 ./pkg/security/...
    Expected Result: All tests PASS, 0 failures
    Evidence: .sisyphus/evidence/task-10-security-tests.txt
  ```

  **Commit**: YES
  - Message: `test(security): add categories and website_guard tests`

- [x] 11. Add `pkg/websocket` tests

  **What to do**:
  - Create `bridge/pkg/websocket/websocket_test.go`:
    - `TestNewServer` — verify default config applied
    - `TestStart_NoBroadcaster_ReturnsError` — `errNoBroadcaster` when no broadcaster wired
    - `TestStart_WithBroadcaster_Success` — starts without error when broadcaster set
    - `TestBroadcast_NoBroadcaster_ReturnsError` — broadcast fails without broadcaster
    - `TestBroadcast_WithBroadcaster_Delegates` — broadcast calls injected broadcaster
    - `TestStop_SetsStartedFalse` — stop changes internal state
    - `TestAddr_Path` — returns configured address and path
    - Mock `EventBroadcaster` with struct implementing the interface

  **Must NOT do**:
  - Do NOT modify `websocket.go`

  **Recommended Agent Profile**:
  - **Category**: `quick`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 3 (with T10, T12-T14)
  - **Blocks**: F1-F4
  - **Blocked By**: None

  **References**:
  **API/Type References**:
  - `bridge/pkg/websocket/websocket.go` — `Server`, `Config`, `EventBroadcaster` interface, `noBroadcasterError`, `NewServer()`, `Start()`, `Stop()`, `Broadcast()`

  **Acceptance Criteria**:
  - [ ] `go test -v ./pkg/websocket/...` — all PASS
  - [ ] `go vet ./pkg/websocket/...` — no issues

  **QA Scenarios**:

  ```
  Scenario: WebSocket tests pass
    Tool: Bash
    Steps:
      1. cd bridge && go test -v -count=1 ./pkg/websocket/...
    Expected Result: All tests PASS
    Evidence: .sisyphus/evidence/task-11-websocket-tests.txt
  ```

  **Commit**: YES
  - Message: `test(websocket): add websocket adapter tests`

- [x] 12. Add `pkg/matrix` tests — httptest-based

  **What to do**:
  - Create `bridge/pkg/matrix/client_test.go`:
    - `TestNew_ValidatesHomeserverURL` — rejects empty/invalid URLs
    - `TestLogin_Success` — mock Matrix v3 login response, verify token stored
    - `TestLogin_InvalidCredentials` — mock 403 response, verify error
    - `TestSendMessage_Success` — mock PUT /_matrix/client/v3/rooms/{roomId}/send/m.room.message/{txnId}
    - `TestSendMessage_TooLarge` — content >65536 bytes rejected client-side
    - `TestGetMessages_Success` — mock /messages response with chunk parsing
    - `TestJoinRoom_Success` — mock /join response
    - `TestSync_Success` — mock /sync with filter, verify sinceToken
    - `TestDefaultSyncFilter` — verify filter defaults (limit=50, lazy_load_members=true)
    - Use `httptest.NewServer` for all HTTP mocking

  **Must NOT do**:
  - Do NOT modify `client.go`
  - Do NOT add external test dependencies

  **Recommended Agent Profile**:
  - **Category**: `deep`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 3 (with T10, T11, T13, T14)
  - **Blocks**: F1-F4
  - **Blocked By**: None

  **References**:
  **API/Type References**:
  - `bridge/pkg/matrix/client.go` — `Client`, `Config`, `Login()`, `SendMessage()`, `GetMessages()`, `JoinRoom()`, `Sync()`, `DefaultSyncFilter()`, `ErrNotLoggedIn`, `ErrMessageTooLarge`

  **Acceptance Criteria**:
  - [ ] `go test -v ./pkg/matrix/...` — all PASS
  - [ ] At least 8 test functions

  **QA Scenarios**:

  ```
  Scenario: Matrix client tests pass
    Tool: Bash
    Steps:
      1. cd bridge && go test -v -count=1 ./pkg/matrix/...
    Expected Result: All tests PASS
    Evidence: .sisyphus/evidence/task-12-matrix-tests.txt
  ```

  **Commit**: YES
  - Message: `test(matrix): add client httptest-based tests`

- [x] 13. Add `internal/executor` tests — ToolPool + ToolExecutor

  **What to do**:
  - Create `bridge/internal/executor/engine_test.go`:
    - `TestNewToolExecutor` — verify config applied
    - `TestExecute_DirectSkill` — mock SkillRegistry returning a skill, verify execution
    - `TestExecute_UnknownSkill_Error` — registry returns nil skill
    - `TestExecuteWithTimeout_Success` — completes within timeout
    - `TestExecuteWithTimeout_Exceeded` — context cancelled
    - `TestToolPool_Execute` — single task through pool
    - `TestToolPool_ExecuteBatch` — multiple tasks in parallel
    - `TestToolPool_ExecuteBatch_ErrorAggregation` — some fail, some succeed
    - `TestToolPool_Close` — workers shut down cleanly
    - `TestParseCommand` — quote handling edge cases
    - `TestIsValidJSON` — valid/invalid JSON detection
    - `TestTruncateOutput` — output size limiting
    - Mock `SkillRegistry` interface with struct implementing GetSkill method
    - Pass nil for PETG gateway (nil-checked in production code)

  **Must NOT do**:
  - Do NOT modify `engine.go`
  - Do NOT extract interfaces from PETG/cache/router — just use nil values

  **Recommended Agent Profile**:
  - **Category**: `deep`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 3 (with T10-T12, T14)
  - **Blocks**: F1-F4
  - **Blocked By**: None

  **References**:
  **API/Type References**:
  - `bridge/internal/executor/engine.go` — `ToolExecutor`, `ToolExecutorConfig`, `SkillRegistry` interface, `Skill`, `ToolCall`, `ToolResult`, `ToolPool`, `ToolPoolConfig`, `NewToolExecutor()`, `Execute()`, `ExecuteWithTimeout()`, `NewToolPool()`, `ToolPool.Execute()`, `ExecuteBatch()`, `Close()`

  **Acceptance Criteria**:
  - [ ] `go test -v ./internal/executor/...` — all PASS
  - [ ] At least 10 test functions

  **QA Scenarios**:

  ```
  Scenario: Executor tests pass
    Tool: Bash
    Steps:
      1. cd bridge && go test -v -count=1 ./internal/executor/...
    Expected Result: All tests PASS
    Evidence: .sisyphus/evidence/task-13-executor-tests.txt
  ```

  **Commit**: YES
  - Message: `test(executor): add ToolPool and ToolExecutor tests`

- [x] 14. Add `pkg/socket` tests — JSON-RPC handlers

  **What to do**:
  - Create `bridge/pkg/socket/server_test.go`:
    - `TestNew_NilKeystore_ReturnsError` — validates keystore requirement
    - `TestNew_DefaultSocketPath` — default path applied correctly
    - `TestServer_StartStop` — server starts, socket created, server stops, socket cleaned
    - `TestHandleMessage_InvalidVersion` — non-2.0 JSON-RPC rejected
    - `TestHandleMessage_UnknownMethod` — unknown method returns error
    - `TestHandleStatus` — status method returns server info
    - `TestHandleHealth` — health method returns healthy
    - `TestAcceptConnections_Concurrency` — multiple clients connect simultaneously
    - Note: `handleStart`, `handleGetCredential` require real keystore — test with temporary SQLCipher DB or skip with comment
    - Use `t.TempDir()` for socket paths to avoid conflicts
    - For keystore-dependent tests: create temp DB with SQLCipher if available, otherwise skip with `t.Skip("requires libsqlcipher-dev")`

  **Must NOT do**:
  - Do NOT refactor `server.go` to extract interfaces — just test what's testable without keystore
  - Do NOT add CGO dependencies to CI

  **Recommended Agent Profile**:
  - **Category**: `unspecified-high`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 3 (with T10-T13)
  - **Blocks**: F1-F4
  - **Blocked By**: None

  **References**:
  **API/Type References**:
  - `bridge/pkg/socket/server.go` — `Server`, `Config`, `New()`, `Start()`, `Stop()`, `GetSocketPath()`, `ErrServerClosed`, `ErrInvalidMessage`

  **Acceptance Criteria**:
  - [ ] `go test -v ./pkg/socket/...` — all PASS (non-keystore tests)
  - [ ] At least 6 test functions

  **QA Scenarios**:

  ```
  Scenario: Socket tests pass
    Tool: Bash
    Steps:
      1. cd bridge && go test -v -count=1 ./pkg/socket/...
    Expected Result: All tests PASS (some may skip if SQLCipher unavailable)
    Evidence: .sisyphus/evidence/task-14-socket-tests.txt
  ```

  **Commit**: YES
  - Message: `test(socket): add JSON-RPC handler tests`

---

## Final Verification Wave (MANDATORY — after ALL implementation tasks)

- [x] F1. **Plan Compliance Audit** — `oracle`
  Read the plan end-to-end. For each "Must Have": verify implementation exists. For each "Must NOT Have": search codebase for forbidden patterns. Compare deliverables against plan.
  Output: `Must Have [N/N] | Must NOT Have [N/N] | Tasks [N/N] | VERDICT: APPROVE/REJECT`

- [x] F2. **Code Quality Review** — `unspecified-high`
  Run `go vet` + `bash -n` on all modified scripts. Review changed files for: unused imports, commented-out code, copy-paste errors from transport detector duplication.
  Output: `Build [PASS/FAIL] | Scripts [N clean/N issues] | Tests [N pass/N fail] | VERDICT`

- [x] F3. **Harness Correctness Verification** — `unspecified-high`
  SSH to VPS, run ALL patched test scripts. Verify: 0 false failures, GATED_EXPECTED appears for voice/e2ee/keystore, ENV_MISSING appears when ADMIN_TOKEN absent. Compare before/after results.
  Output: `Scripts [N/N pass] | False Failures [0] | GATED_EXPECTED [N expected] | VERDICT`

- [x] F4. **Scope Fidelity Check** — `deep`
  For each task: read "What to do", read actual diff. Verify 1:1 — no missing, no creep. Check "Must NOT do" compliance. Flag unaccounted changes.
  Output: `Tasks [N/N compliant] | Creep [CLEAN/N issues] | VERDICT`

---

## Commit Strategy

- **T1-T3**: `fix(tests): add shared transport detector and normalize result categories` — tests/lib/transport.sh, tests/lib/common_output.sh, tests/lib/load_env.sh, tests/lib/restart_bridge.sh
- **T4-T5**: `fix(tests): migrate 16 harness scripts to shared transport detector` — tests/test-*.sh (16 files)
- **T6**: `fix(tests): define matrix.status contract and fix integration test` — tests/test-matrix-integration.sh
- **T7-T8**: `fix(deploy): configure VPS env and sync test scripts` — VPS .env changes
- **T10**: `test(security): add categories and website_guard tests` — bridge/pkg/security/*_test.go
- **T11**: `test(websocket): add websocket adapter tests` — bridge/pkg/websocket/websocket_test.go
- **T12**: `test(matrix): add client httptest-based tests` — bridge/pkg/matrix/client_test.go
- **T13**: `test(executor): add ToolPool and ToolExecutor tests` — bridge/internal/executor/engine_test.go
- **T14**: `test(socket): add JSON-RPC handler tests` — bridge/pkg/socket/server_test.go

---

## Success Criteria

### Harness Correctness
- 0 false failures on Docker-hosted Bridge deployments
- All admin-gated Tier A/Tier C suites run when ADMIN_TOKEN is set
- Feature-gated subsystems (voice, e2ee, keystore) report GATED_EXPECTED, not FAIL

### Coverage Expansion
- 0 high-risk Bridge packages left with zero test files
- Each zero-test package has at least 1 failure-mode test, not just happy-path

### Matrix Contract
- `matrix.status` returns documented, deterministic response
- `test-matrix-integration.sh` and `test-rpc-methods.sh` agree on expected output

### Verification Commands
```bash
bash -n tests/lib/transport.sh                              # Syntax OK
source tests/lib/transport.sh; detect_transport              # Exports functions
go test -v ./pkg/security/... ./pkg/websocket/... ./pkg/matrix/... ./internal/executor/... ./pkg/socket/...  # All PASS
ssh root@5.183.11.149 "source /opt/armorclaw/.env && bash /opt/armorclaw/tests/test-vps-smoke.sh"  # 0 false failures
```

### Final Checklist
- [ ] All "Must Have" present
- [ ] All "Must NOT Have" absent
- [ ] All Go tests pass
- [ ] All shell scripts syntax-check clean
- [ ] VPS validation suite produces 0 false failures
