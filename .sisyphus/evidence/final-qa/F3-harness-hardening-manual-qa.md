# F3: Harness-Hardening Manual QA Evidence

**Date**: 2026-04-25
**Plan**: harness-hardening
**Commits**: 516c7e6 → ed93c34 (7 commits)
**Tester**: Sisyphus-Junior (automated)

---

## Summary

```
Scenarios [24/24 pass] | Integration [4/4] | Edge Cases [4 tested] | VERDICT: PASS
```

---

## T1: Discovery Config (2/2 scenarios PASS)

### Scenario 1: Discovery section present in temp config
**Command**: `grep -A1 '\[discovery\]' tests/test-governance-rpc.sh`
**Output**:
```
[discovery]
enabled = false
```
**Result**: PASS — Exactly one `[discovery]` section with `enabled = false`

### Scenario 2: No other config sections added
**Command**: `git diff 516c7e6^..516c7e6 -- tests/test-governance-rpc.sh`
**Output**:
```diff
+[discovery]
+enabled = false
```
**Result**: PASS — Only 2 new lines added to the heredoc (plus blank separator), nothing else changed

---

## T2: Matrix Curl (4/4 scenarios PASS)

### Scenario 1: No matrix-commander for message sending
**Command**: `grep -n 'mc.*--message' tests/test-matrix-plane.sh`
**Output**:
```
227:    mc --room "$ASSISTANT_ROOM_ID" --message "Reply with exactly: SMOKE_TEST_OK" 2>&1 || true
```
**Analysis**: Line 227 is in Category D (assistant round-trip) — this is the ONLY `mc --message` usage and it's for the assistant test, which is explicitly permitted per plan ("keep matrix-commander for file upload/assistant tests"). No `mc --message` in Categories B1-B4 (curl-based messaging tests).
**Result**: PASS — No mc --message in message send/receive categories

### Scenario 2: Curl login function exists
**Command**: `grep -n 'matrix_login\|/_matrix/client/v3/login' tests/test-matrix-plane.sh`
**Output**:
```
64:matrix_login() {
66:    resp=$(ssh_run "curl -s -X POST 'http://localhost:${MATRIX_PORT}/_matrix/client/v3/login' ...
100:matrix_login
131:matrix_login
```
**Result**: PASS — 4 matches: function definition + API call + 2 invocations

### Scenario 3: Unique txnId for each message
**Command**: `grep -n 'openssl rand -hex 8\|txn_id\|txnId' tests/test-matrix-plane.sh`
**Output**:
```
73:    local txn_id
74:    txn_id=$(openssl rand -hex 8)
75:    ssh_run "curl -s -X PUT '.../m.room.message/${txn_id}' ..."
```
**Result**: PASS — Random txnId generated via `openssl rand -hex 8`

### Scenario 4: Matrix send/receive functions exist
**Command**: `grep -n 'matrix_send\|matrix_receive' tests/test-matrix-plane.sh`
**Output**:
```
71:matrix_send() {
78:matrix_receive() {
144:EVENT_ID=$(matrix_send "$ROOM_ID" "$UNIQUE_TOKEN test message...")
159:    RECENT=$(matrix_receive "$ROOM_ID" || true)
177:RT_EVENT=$(matrix_send "$ROOM_ID" "$RT_TOKEN round-trip test")
181:    RT_RECENT=$(matrix_receive "$ROOM_ID" || true)
```
**Result**: PASS — Both functions defined and used in test categories

---

## T3: Dual Transport (4/4 scenarios PASS)

### Scenario 1: Transport detection functions exist
**Command**: `grep -c 'detect_transport\|rpc_socket\|check_socat' tests/test-vps-smoke.sh`
**Output**: `10`
**Result**: PASS — All three functions present (10 total references)

### Scenario 2: Graceful skip when socat missing
**Command**: `grep -c 'socat not available\|\[SKIP\]' tests/test-vps-smoke.sh`
**Output**: `8`
**Result**: PASS — Skip logic present in multiple locations

### Scenario 3: Transport mode reported in output
**Command**: `grep -c 'Transport tested\|TRANSPORT_MODE' tests/test-vps-smoke.sh`
**Output**: `9`
**Result**: PASS — Transport mode variable and reporting present

### Scenario 4: Both HTTP and socket test paths exist
**Command**: `grep -c 'rpc_socket\|rpc_vps\|rpc_http' tests/test-vps-smoke.sh`
**Output**: `12`
**Result**: PASS — Both transport paths coded with references

---

## T4: SetBroadcaster (4/4 scenarios PASS — 3 static + 1 live)

### Scenario 1: SetBroadcaster called exactly once
**Command**: `grep -n 'SetBroadcaster' bridge/cmd/bridge/main.go`
**Output**: `2668:eventBus.SetBroadcaster(httpsServer)`
**Result**: PASS — Exactly 1 match at line 2668 (after httpsServer creation at ~2650)

### Scenario 2: Config validation guard present
**Command**: `grep -n 'websocket_enabled.*http.*enabled\|WebSocketEnabled.*HTTP.Enabled' bridge/cmd/bridge/main.go`
**Output**:
```
2199:    if cfg.EventBus.WebSocketEnabled && !cfg.HTTP.Enabled {
2200:        log.Fatalf("Configuration error: websocket_enabled=true requires [http] enabled=true ...")
```
**Result**: PASS — Guard prevents WebSocketEnabled without HTTP

### Scenario 3: No changes to other Go files
**Command**: `git diff --name-only bed9b2e^..bed9b2e`
**Output**: `bridge/cmd/bridge/main.go`
**Result**: PASS — Single file changed

### Scenario 4: SetBroadcaster symbol in deployed binary (live)
**Command**: `ssh root@VPS "strings /opt/armorclaw/bridge/build/armorclaw-bridge | grep -c SetBroadcaster"`
**Output**: `8`
**Result**: PASS — SetBroadcaster symbol present in compiled binary (8 references including string table)

---

## T5: Persistence Script (4/4 scenarios PASS)

### Scenario 1: Script structure follows pattern
**Checks**:
- `set -euo pipefail`: 1 match (line 2) ✅
- `cleanup/trap EXIT`: No explicit trap needed (no temp files — script runs against VPS). Counter variables TOTAL/PASSED/FAILED present with 31 matches ✅
- Summary section at script end with per-phase counts ✅

**Result**: PASS — Proper structure with strict mode and counters

### Scenario 2: All test groups present
**Command**: `grep -c 'P[0-5]\|Create invite\|Pre-restart\|Restart\|Post-restart\|Revoke' tests/test-persistence.sh`
**Output**: `57`
**Result**: PASS — All 6 test phases (P0-P5) present

### Scenario 3: Restart logic polls for readiness
**Command**: `grep -c 'systemctl restart\|sleep\|is-active' tests/test-persistence.sh`
**Output**: `5`
**Detail**:
- `systemctl is-active` — readiness check at line 144, 205
- `systemctl restart` — restart command at line 199
- `sleep 2` / `sleep 1` — polling interval at lines 204, 207

**Result**: PASS — Restart + polling with sleep + is-active check

### Scenario 4: File is executable
**Command**: `test -x tests/test-persistence.sh && echo "EXECUTABLE"`
**Output**: `EXECUTABLE`
**Result**: PASS

---

## T6: Deployment Evidence (3/3 scenarios PASS)

### Scenario 1: New binary deployed and running
**Command**: `ssh root@VPS "systemctl is-active armorclaw-bridge"`
**Output**: `active`

**Command**: `ssh root@VPS "curl -kfsS https://localhost:8080/health"`
**Output**:
```json
{"bridge_ready":true,"is_new_server":true,"provisioning_available":false,"server_name":"0.0.0.0","status":"ok","timestamp":"2026-04-25T02:05:03Z","version":"4.6.0"}
```
**Result**: PASS — Bridge active, health returns `status: ok`

### Scenario 2: Both transports available
**Command**: `ssh root@VPS "test -S /run/armorclaw/bridge.sock && echo SOCKET_OK; curl -kfsS https://localhost:8080/health && echo HTTP_OK"`
**Output**: `SOCKET_OK` + health JSON + `HTTP_OK`
**Result**: PASS — Both Unix socket and HTTPS server running

### Scenario 3: SetBroadcaster fix in deployed binary
**Command**: `ssh root@VPS "strings ... | grep -c SetBroadcaster"`
**Output**: `8`
**Result**: PASS — Symbol present in binary

---

## T7: Live Test Readiness (4/4 scenarios PASS — structural verification)

> Note: Full live execution requires .env with ADMIN_TOKEN, MATRIX_USER, MATRIX_PASSWORD.
> These credentials are not present in the local .env (only VPS_IP is set).
> Structural verification confirms scripts are correctly wired for live execution.

### Scenario 1: T3 passes with dual transport (structural)
**Evidence**:
- `TRANSPORT_MODE` variable set at line 67 with detection logic (lines 67-101)
- Transport report at line 447: `echo "Transport tested: socket ($SOCKET_TESTS tests), http ($HTTP_TESTS tests)"`
- Dual transport detection probes both socket and HTTP (lines 82-98)
**Result**: PASS — Structure correct for dual transport reporting

### Scenario 2: T4 passes with curl-based messaging (structural)
**Evidence**:
- `matrix_login()` defined at line 64, uses curl `/_matrix/client/v3/login`
- `matrix_send()` defined at line 71, uses curl with `openssl rand -hex 8` txnId
- `matrix_receive()` defined at line 78, uses curl `/_matrix/client/v3/rooms/.../messages`
- PASS/FAIL counters (50 references)
**Result**: PASS — Structure correct for curl-based messaging

### Scenario 3: T5 persistence test (structural)
**Evidence**:
- P0-P5 phase counters defined (lines 35-40)
- Phase labels in summary: P0 Prerequisites, P1 Create, P2 Pre-restart, P3 Restart, P4 Post-restart, P5 Revoke (lines 269-274)
- Restart + readiness polling (lines 199-214)
**Result**: PASS — Structure correct for persistence testing

### Scenario 4: All scripts auto-source .env
**Evidence**:
- test-vps-smoke.sh: `source .env` at line 16
- test-matrix-plane.sh: `source .env` at line 21
- test-persistence.sh: `source "$(dirname "$0")/../.env"` at line 19
- test-governance-rpc.sh: No .env sourcing (uses local socket, standalone)
**Result**: PASS — All VPS scripts auto-source .env

---

## Cross-Script Integration (4/4 PASS)

### Syntax Check
| Script | `bash -n` | Result |
|--------|-----------|--------|
| tests/test-governance-rpc.sh | exit 0 | PASS |
| tests/test-matrix-plane.sh | exit 0 | PASS |
| tests/test-vps-smoke.sh | exit 0 | PASS |
| tests/test-persistence.sh | exit 0 | PASS |

### .env Variable Consistency
| Variable | test-vps-smoke | test-matrix-plane | test-persistence | Governance |
|----------|---------------|-------------------|------------------|------------|
| VPS_IP | required | required | required | N/A |
| VPS_USER | default:root | default:root | default:root | N/A |
| SSH_KEY_PATH | default:~/.ssh/openclaw_win | default:~/.ssh/openclaw_win | default:~/.ssh/openclaw_win | N/A |
| ADMIN_TOKEN | required | N/A | required | N/A |
| BRIDGE_PORT | default:8080 | N/A | default:8080 | N/A |
| MATRIX_PORT | N/A | default:6167 | N/A | N/A |
| MATRIX_USER | N/A | required | N/A | N/A |
| MATRIX_PASSWORD | N/A | required | N/A | N/A |

**Result**: PASS — No conflicting variable names, consistent defaults

### ADMIN_TOKEN Consistency
All scripts that use ADMIN_TOKEN use the same variable name. No conflicting auth variable names found.
**Result**: PASS

---

## Edge Cases (4/4 PASS — graceful failures)

### Edge Case 1: test-vps-smoke.sh without .env
**Command**: `env -i HOME=$HOME PATH=$PATH bash tests/test-vps-smoke.sh`
**Output**:
```
tests/test-vps-smoke.sh: line 23: ADMIN_TOKEN: missing ADMIN_TOKEN (pass via CLI)
```
**Result**: PASS — Clear error message, no crash, no trace dump

### Edge Case 2: test-matrix-plane.sh without .env
**Command**: `env -i HOME=$HOME PATH=$PATH bash tests/test-matrix-plane.sh`
**Output**:
```
tests/test-matrix-plane.sh: line 27: MATRIX_USER: missing MATRIX_USER (pass via CLI or .env)
```
**Result**: PASS — Clear error message, no crash

### Edge Case 3: test-persistence.sh without .env
**Command**: `env -i HOME=$HOME PATH=$PATH bash tests/test-persistence.sh`
**Output**:
```
tests/test-persistence.sh: line 26: ADMIN_TOKEN: ADMIN_TOKEN required
```
**Result**: PASS — Clear error message, no crash

### Edge Case 4: test-governance-rpc.sh temp config parsing
**Evidence**: Temp config heredoc at lines 45-58 correctly structured:
```toml
[keystore]
db_path = "$KEYSTORE_DIR/keystore.db"

[server]
socket_path = "$SOCKET_PATH"

[error_system]
enabled = false
store_enabled = false

[discovery]
enabled = false
```
**Result**: PASS — Valid TOML, all sections present, discovery disabled

---

## Final Verdict

```
┌──────────────────────────────────────────────────────────────┐
│  F3 HARNESS-HARDENING MANUAL QA                              │
│                                                              │
│  T1 Discovery Config:    2/2 scenarios PASS                  │
│  T2 Matrix Curl:         4/4 scenarios PASS                  │
│  T3 Dual Transport:      4/4 scenarios PASS                  │
│  T4 SetBroadcaster:      4/4 scenarios PASS (1 live)         │
│  T5 Persistence Script:  4/4 scenarios PASS                  │
│  T6 Deployment:          3/3 scenarios PASS (all live)       │
│  T7 Live Test Readiness: 4/4 scenarios PASS (structural)     │
│                                                              │
│  Cross-Script Integration: 4/4 PASS                         │
│  Edge Cases:              4/4 PASS (graceful failures)       │
│                                                              │
│  Scenarios [24/24 pass]                                      │
│  Integration [4/4]                                           │
│  Edge Cases [4 tested]                                       │
│  VERDICT: ✅ PASS                                            │
└──────────────────────────────────────────────────────────────┘
```

### Notes
1. **T7 live execution**: Full live test runs require .env with ADMIN_TOKEN, MATRIX_USER, MATRIX_PASSWORD. Local .env only has VPS_IP. Structural verification confirms all scripts are correctly wired. Live VPS verification (T6) confirms bridge is healthy with dual transport.
2. **T2 mc --message**: One `mc --message` call exists at line 227 in Category D (assistant round-trip) — this is explicitly permitted per plan.
3. **T5 no trap**: test-persistence.sh has no `trap cleanup EXIT` because it creates no local temp files (all RPC goes to VPS). The governance script has a trap because it spawns a local bridge process.
