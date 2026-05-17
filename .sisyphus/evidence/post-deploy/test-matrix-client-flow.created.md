# test-matrix-client-flow.sh — Created

**File:** `tests/test-matrix-client-flow.sh`
**Syntax check:** PASS (`bash -n`)
**Date:** 2026-05-14

## Tests Implemented

| # | Test | API Endpoint | Method |
|---|------|-------------|--------|
| 1 | Login via m.login.password | `/_matrix/client/v3/login` | POST |
| 2 | Create room | `/_matrix/client/v3/createRoom` | POST |
| 3 | Send message (unique token) | `/_matrix/client/v3/rooms/{roomId}/send/m.room.message/{txnId}` | PUT |
| 4 | Sync and verify message | `/_matrix/client/v3/sync?timeout=5000` | GET |
| 5 | Cleanup — leave room | `/_matrix/client/v3/rooms/{roomId}/leave` | POST |

## Pattern

Follows `test-matrix-plane.sh` conventions:
- Sources `tests/lib/load_env.sh` (provides `ssh_vps()`, VPS_IP, MATRIX_PORT)
- Sources `tests/lib/common_output.sh` (provides `log_pass/log_fail/harness_summary`)
- SSH to VPS, curl against Conduit at `http://localhost:6167`
- Unique test message via `openssl rand -hex 8`
- `jq` for all JSON parsing
- Output: `Matrix Client Flow: N/5 PASS`

## Not tested (intentional)

- Bridge RPC methods — covered by `test-matrix-e2e-rpc.sh`
- matrix-commander — uses curl only
- socat / bridge.sock — not used
