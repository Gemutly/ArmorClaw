# Task 10: VPS Deployment and Integration Verification

**Date:** 2026-05-06
**VPS:** 5.183.11.149 (root)

## Summary

Successfully deployed the autoheal bridge binary to VPS and verified full integration including recovery cycle testing.

## Step 1: Source Code Sync
- Used `rsync` to sync bridge source (excluding binaries) from local to `/opt/armorclaw/bridge/`
- Key files transferred: `internal/adapter/matrix.go`, `pkg/eventbus/events.go`, `pkg/rpc/server.go`, `pkg/systemd/notify.go`

## Step 2: Build on VPS
- **Issue:** `go-sqlcipher/v4 v4.4.2` CGO compilation error: `unknown type name 'sqlite3_filename'`
  - Root cause: Bundled `sqlite3.h` in go-sqlcipher v4.4.2 lacks the `sqlite3_filename` typedef
  - Fix: Patched module cache header: `typedef const char *sqlite3_filename;`
- **Go version:** 1.26.1 (system had been upgraded from original 1.22)
- **Build command:** `go build -o armorclaw-bridge-new ./cmd/bridge/`
- **Result:** 44MB binary built successfully

## Step 3: Deployment
- Old binary backed up: `armorclaw-bridge.bak-20260506022424`
- New binary deployed to `/opt/armorclaw/bridge/build/armorclaw-bridge`
- systemd service file already had `Type=notify`, `NotifyAccess=main`, `WatchdogSec=60`
- Service restarted successfully

## Step 4: Verification Results

### Service Status
```
Active: active (running)
Main PID: 41911
Status: "Matrix: connected"
Memory: 7.7M
```

### Health Check
```json
{"bridge_ready":true,"is_new_server":true,"provisioning_available":false,
 "server_name":"0.0.0.0","status":"ok","timestamp":"2026-05-06T02:28:29Z","version":"4.6.0"}
```

### Key Log Lines
- `Systemd watchdog enabled (ping every 30s)`
- `Matrix login successful via password`
- `Matrix sync loop started`
- `Matrix adapter initialized: @armorclaw-bridge:5.183.11.149`

## Step 5: Recovery Cycle Test

### Conduit Stop → Bridge Backoff
1. `docker stop armorclaw-conduit` at 02:29:06 UTC
2. Bridge detected sync failures with exponential backoff:
   - `consecutive_failures=1 backoff=1s`
   - `consecutive_failures=2 backoff=2s`
   - `consecutive_failures=3 backoff=4s` → auto re-login triggered
   - `consecutive_failures=4 backoff=8s`
3. Systemd status: `Matrix: reconnecting (backoff: 8s)`
4. Bridge remained `active` throughout

### Conduit Start → Bridge Recovery
1. `docker start armorclaw-conduit` at 02:29:56 UTC
2. Bridge recovered at 02:30:27: `matrix sync recovered, previous_failures=5`
3. Systemd status: `Matrix: connected`
4. Health check: `{"status":"ok", "bridge_ready":true}`

### Recovery Summary
| Phase | Time | Detail |
|-------|------|--------|
| Conduit stopped | 02:29:06 | Bridge detects `connection refused` |
| First failure | 02:29:14 | `backoff=1s` |
| Auto re-login | 02:29:23 | `re-login successful` |
| Max backoff | 02:29:30 | `backoff=8s` |
| Conduit restarted | 02:29:56 | Docker container started |
| Sync recovered | 02:30:27 | `previous_failures=5`, status → `connected` |

## Build Gotcha (for future deploys)
The `go-sqlcipher/v4@v4.4.2` module has a CGO compatibility issue with modern Go versions due to missing `sqlite3_filename` typedef in its bundled `sqlite3.h`. Patch required:
```bash
SQLCIPHER_DIR=/root/go/pkg/mod/github.com/mutecomm/go-sqlcipher/v4@v4.4.2
chmod u+w "$SQLCIPHER_DIR/sqlite3.h"
sed -i "/^typedef struct sqlite3 sqlite3;$/a typedef const char *sqlite3_filename;" "$SQLCIPHER_DIR/sqlite3.h"
```

## Conduit Status (verified running at end)
```
armorclaw-conduit: running (Docker)
armorclaw-bridge: active (running), Matrix: connected
```
