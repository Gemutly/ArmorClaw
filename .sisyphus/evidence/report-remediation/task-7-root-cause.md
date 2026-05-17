# Task 7: Bridge Token Invalidation Root Cause Analysis

**Date**: 2026-05-13
**Investigator**: Automated investigation
**Status**: ROOT CAUSE IDENTIFIED

---

## Root Cause

**DUPLICATE BRIDGE PROCESS (PID 120193) RUNNING ALONGSIDE SYSTEMD-MANAGED BRIDGE (PID 120207)**

Two instances of `armorclaw-bridge` are running simultaneously, both using the same Matrix credentials (`armorclaw-bridge`). Each instance independently performs Matrix login, which invalidates the previous session's access token. This creates a tight loop:

1. Process A logs in → gets new token
2. Process B's sync fails with `M_UNKNOWN_TOKEN` (token was invalidated by A's login)
3. Process B re-logs in → gets new token (invalidates A's token)
4. Process A's sync fails → re-logs in → invalidates B's token
5. Repeat every ~6 seconds

This is NOT an identity collision between different Matrix accounts. It is TWO INSTANCES of the same bridge binary fighting over ONE Matrix account.

---

## Evidence

### 1. Two Bridge Processes Confirmed

```
PID 120193 (root)       - Started 07:30:53 UTC - zombie/manual process
  exe -> /opt/armorclaw/bridge/build/armorclaw-bridge
  stdout/stderr -> /var/log/armorclaw/bridge.log (direct file write)

PID 120207 (armorclaw)  - Started 07:32:27 UTC - systemd-managed
  exe -> /opt/armorclaw/bridge/build/armorclaw-bridge
  stdout/stderr -> journald (via systemd)
```

**Systemd MainPID**: 120207 (the legitimate service)

The zombie process (PID 120193) was launched manually as root, likely from the nohup restart script visible in `ps aux` output:
```
bash -c kill 113813 2>/dev/null; sleep 2; ... nohup ./armorclaw-bridge -config /etc/armorclaw/config.toml >> /var/log/armorclaw/bridge.log 2>&1 &
```

### 2. Token Invalidation Cycle - Exact 6-Second Interval

Timestamps from `grep 'matrix token invalidated, triggering' /var/log/armorclaw/bridge.log`:

```
23:26:04 → 23:26:10 → 23:26:16 → 23:26:22 → 23:26:28 → 23:26:34 → 23:26:40 → 23:26:46 → 23:26:52 → 23:26:58 → 23:27:05 → 23:27:11 → 23:27:17 → 23:27:23 → 23:27:29 → 23:27:35 → 23:27:41 → 23:27:47 → 23:27:53 → 23:27:59 → 23:28:05 → 23:28:12 → 23:28:18
```

**Interval: consistently 6 seconds** (with occasional 7s). This matches the pattern of two sync loops interleaving.

### 3. Log Sequence Per Cycle

Each cycle shows the same 3-message pattern:
```
WARN  msg="matrix token invalidated, triggering immediate re-login" error="matrix token invalidated: M_UNKNOWN_TOKEN"
INFO  msg="security event" event_type=bridge_event_published event_type=bridge.status websocket_enabled=true
INFO  msg="matrix re-login successful"
WARN  msg="matrix sync error" consecutive_failures=1 backoff=1s error="matrix token invalidated: M_UNKNOWN_TOKEN"
```

After each re-login, processEvents fires for 5 rooms, then the OTHER process's login immediately invalidates the token.

### 4. Systemd Service Configuration

The legitimate service is properly configured:
```
[Unit]
Description=ArmorClaw Bridge Service
After=network-online.target docker.service

[Service]
Type=notify
User=armorclaw
ExecStart=/opt/armorclaw/armorclaw-bridge -config /etc/armorclaw/config.toml
Restart=always
StandardOutput=journal
StandardError=journal
```

The systemd service correctly writes to journald, NOT to `/var/log/armorclaw/bridge.log`.

### 5. Zombie Process Origin

PID 120193 was spawned by a bash command visible in `ps aux`:
```
bash -c kill 113813 2>/dev/null; sleep 2; ps aux | grep armorclaw-bridge | grep -v grep; echo '---RESTART---'; cd /opt/armorclaw && nohup ./armorclaw-bridge -config /etc/armorclaw/config.toml >> /var/log/armorclaw/bridge.log 2>&1 & sleep 3; tail -10 /var/log/armorclaw/bridge.log; echo '---HEALTH---'; curl -sk 'https://localhost:8080/health' 2>&1
```

This was a manual restart script (likely from a debugging session) that:
1. Killed the old process (PID 113813)
2. Started a new bridge in background with nohup, logging to `/var/log/armorclaw/bridge.log`
3. Did NOT kill itself after spawning the background process
4. The systemd service then also started its own instance (PID 120207) at 07:32

### 6. Conduit Logs - No Server-Side Issues

Conduit shows no token/session errors. Only standard warnings about missing admin API routes and occasional 500s:
```
WARN  Not found: /_matrix/client/r0/admin/register
WARN  Not found: /_matrix/client/r0/admin/users?access_token=&limit=10
ERROR response failed classification=Status code: 500 Internal Server Error
```

No evidence of Conduit-side session limits or token expiration. The Conduit container has been stable for 37 hours.

### 7. Bridge Config - Single Account, No Device ID Conflict

```toml
[matrix]
enabled = true
homeserver_url = "http://localhost:6167"
username = "armorclaw-bridge"
password = "jIa0vwprzlBZJwZMEawVI59h7DlQ76bT"
device_id = "armorclaw-bridge"
```

Both processes use identical config → same username + password + device_id. When Conduit sees a login with the same device_id, it **invalidates the previous token** (standard Matrix behavior).

---

## Fix Recommendation

### Immediate Fix (Safe)

Kill the zombie process (PID 120193, running as root) and verify only the systemd-managed process remains:

```bash
# Kill the zombie
kill 120193

# Verify only one process remains
ps aux | grep armorclaw-bridge | grep -v grep

# Should show only PID 120207 (armorclaw user, systemd-managed)
# If not, restart the service cleanly:
# systemctl restart armorclaw-bridge.service

# Monitor for 30 seconds to confirm cycle stops
journalctl -u armorclaw-bridge.service -f --since "now"
```

### Prevention

1. **Remove manual restart scripts**: The nohup-based restart script should not be used. Always use `systemctl restart armorclaw-bridge.service`.

2. **Add PID file locking**: Consider adding a PID file to the bridge binary so a second instance refuses to start if one is already running.

3. **Add process guard to systemd unit**: Add `ExecStartPre=/bin/bash -c 'pkill -f armorclaw-bridge || true'` is NOT recommended (too aggressive). Instead, rely on systemd's `Type=notify` + `Restart=always` for proper lifecycle management.

4. **Clean up `/var/log/armorclaw/bridge.log`**: The systemd service writes to journald. The bridge.log file is only written to by the zombie process. After killing the zombie, this file will stop growing. It can be archived/cleaned.

### Expected State After Fix

- Single bridge process (PID 120207, managed by systemd)
- Token remains valid across sync cycles
- No more `M_UNKNOWN_TOKEN` errors
- Bridge log (journald) shows stable sync without re-login cycles
