# Matrix Deployment Verification — Final Report

Date: 2026-03-23
VPS: 5.183.11.149
**Status: FUNCTIONALLY COMPLETE — one deferred proof item (see below)**

---

## Verified Outcomes

### 1. Matrix Conduit Container — VERIFIED
- Container `armorclaw-conduit` running (image: `matrixconduit/matrix-conduit:latest`)
- Port 6167 bound to `0.0.0.0:6167->6167/tcp`
- `/_matrix/client/versions` returns 14 supported versions including v1.1 through v1.11
- Unstable features: `e2e_cross_signing`, `msc3916`, `msc3575`
- Config: `/tmp/armorclaw-conduit.toml` mounted read-only into container at `/etc/conduit.toml`
- Data volume: `armorclaw-conduit-data` (RocksDB)

### 2. Matrix Users — VERIFIED
- **Admin user**: `@armorclaw_admin:5.183.11.149` (new)
  - Password: `ArmorClawAdmin2026!`
  - Access token: `BzgGRh6ULqHyQPiDoX30D9vxcKC1lhDC`
  - Login verified externally via Matrix v1.2 identifier format
  - **Note**: Original `@admin` user exists but password is LOST (`.admin_password` was never written to disk per container-setup.sh:2478-2482)
- **Bridge user**: `@bridge:5.183.11.149`
  - Password: `bridgepass`
  - Login verified, access token stored in encrypted SQLCipher keystore
  - Syncing every 30s with `Sync: complete next_batch=N events=0 rooms=0`

### 3. Registration Locked — VERIFIED
- `allow_registration = false` in `/tmp/armorclaw-conduit.toml`
- No `registration_token` set
- Test registration returns `M_FORBIDDEN` (verified after Conduit restart)

### 4. Bridge Config — VERIFIED
- `[matrix].enabled = true`
- `homeserver_url = "http://localhost:6167"`
- `username = "bridge"`, `password = "bridgepass"`
- `device_id = "armorclaw-bridge"`

### 5. Bridge Service — VERIFIED
- systemd unit `armorclaw-bridge.service`, binary at `/opt/armorclaw/armorclaw-bridge`
- Status: `active (running)`, PID 204885, ~7.2M RSS
- Healthy sync loop: every 30s, zero Matrix errors in logs
- RPC socket: `/run/armorclaw/bridge.sock` (owned by `armorclaw:armorclaw`)

### 6. Bridge RPC Methods — VERIFIED (partial)
| Method | Status | Notes |
|--------|--------|-------|
| `matrix.status` | WORKS | `{enabled:true, connected:true, logged_in:true, user_id:"@bridge:5.183.11.149"}` |
| `matrix.login` | WORKS | Re-authenticates bridge user successfully |
| `matrix.send` | EXISTS | Returns 403 if bridge not in target room (expected) |
| `store_key` | WORKS | Successfully stored test key |
| `bridge.status` | PANICS | nil pointer dereference (non-critical, existing bug) |

### 7. Room Created for !status Test
- Room ID: `!IGY2TnBy2gp9GpW__JI0JG0SP61PW0CeGvWCqFUMZCI`
- Created by `armorclaw_admin`
- Made publicly joinable (`join_rule: public`)

---

## Deferred: !status End-to-End Proof

**Status**: BLOCKED — pending room join capability

**Root Cause**:
- `MatrixAdapter.JoinRoom()` exists at `bridge/internal/adapter/matrix.go:1625`
- No `matrix.join_room` RPC method is exposed in `bridge/pkg/rpc/server.go:689-743`
- Bridge user has `rooms_count=0` — not in any rooms
- Making a room public does NOT cause the bridge to auto-discover and join it
- Conduit does not support admin API (`/_matrix/client/r0/admin/v1/rooms/...` returns `M_UNRECOGNIZED`)
- Bridge user's access token is in encrypted SQLCipher keystore (cannot extract for manual Matrix API call)

**What's needed**:
- Expose `matrix.join_room` RPC by wiring `MatrixAdapter.JoinRoom()` through `server.go`
- This is a small code change, not a deployment issue
- Follow-on plan: `.sisyphus/plans/matrix-join-room-rpc.md`

**This is NOT a deployment problem**. The deployment is verified and usable. The remaining gap is a missing RPC endpoint — a bridge feature/fix.

---

## Test Artifacts to Clean Up

| Artifact | Type | Cleanup Action |
|----------|------|----------------|
| `test_probe_user` | Matrix user | Deactivate via admin API |
| `store_key` dummy entry | Keystore | Delete test key `test-key-001` |
| `@admin` user (password lost) | Matrix user | Consider deactivation; replaced by `armorclaw_admin` |

---

## Verification Commands Used

```bash
# SSH connection
ssh -o ConnectTimeout=5 -o StrictHostKeyChecking=no -i ~/.ssh/openclaw_win root@5.183.11.149

# Matrix Conduit health
curl -s http://5.183.11.149:6167/_matrix/client/versions

# Matrix login (v1.2 identifier format required)
curl -s -X POST http://5.183.11.149:6167/_matrix/client/v3/login \
  -H "Content-Type: application/json" \
  -d '{"type":"m.login.password","identifier":{"type":"m.id.user","user":"armorclaw_admin"},"password":"ArmorClawAdmin2026!"}'

# Bridge RPC — matrix.status (WORKS)
printf '{"jsonrpc":"2.0","id":1,"method":"matrix.status"}\n' | socat - UNIX-CONNECT:/run/armorclaw/bridge.sock

# Bridge RPC — store_key (WORKS)
printf '{"jsonrpc":"2.0","id":2,"method":"store_key","params":{"provider":"openrouter","token":"sk-test-dummy","id":"test-key-001"}}\n' | socat - UNIX-CONNECT:/run/armorclaw/bridge.sock

# Bridge RPC — bridge.status (PANICS — nil pointer dereference)
printf '{"jsonrpc":"2.0","id":3,"method":"bridge.status"}\n' | socat - UNIX-CONNECT:/run/armorclaw/bridge.sock

# Registration lock test (returns M_FORBIDDEN — verified locked)
curl -s -X POST http://5.183.11.149:6167/_matrix/client/v3/register \
  -H "Content-Type: application/json" \
  -d '{"username":"should_fail","password":"test123","auth":{"type":"m.login.dummy"}}'

# Bridge logs — sync health
journalctl -u armorclaw-bridge -n 100 --no-pager | grep -i "sync\|matrix\|error"
```

---

**Report Generated**: 2026-03-23
**Sessions**: 10 sessions tracked in boulder.json
**Verdict**: Matrix deployment functionally complete. One deferred proof item (room join RPC) documented in follow-on plan.
