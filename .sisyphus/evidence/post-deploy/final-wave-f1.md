# F1: Startup Audit — Final Verification Wave

**Date:** $(date -u +%Y-%m-%dT%H:%M:%SZ)
**Commit Reference:** a563703 (Wave 1 baseline)
**Auditor:** Oracle (automated)

---

## Check 1: Config Preservation — PASS

**File:** `deploy/setup-quick.sh`, lines 1025-1029
**What was verified:** `generate_config()` contains a guard that skips config generation if the file already exists, unless `FORCE_REGEN=true`.

```bash
# Guard: preserve existing config unless FORCE_REGEN is set
if [[ -f "$config_file" ]] && [[ "${FORCE_REGEN:-false}" != "true" ]]; then
    print_done "Config already exists, preserving: $config_file"
    return 0
fi
```

**Verdict:** Config is preserved across container restarts. Only `FORCE_REGEN=true` triggers overwrite.

---

## Check 2: CI Mode — ensure_matrix() BEFORE tail — PASS

**File:** `deploy/setup-quick.sh`, `main()` function (lines 1679-1801)
**What was verified:** In first-run CI mode, the execution order is:

1. `check_prerequisites` (line 1709)
2. `install_bridge` (line 1718)
3. `create_user` (line 1719)
4. `generate_config` (line 1720)
5. `init_keystore` (line 1721)
6. `setup_systemd` (line 1722)
7. `start_bridge` (line 1723)
8. `verify_health` (line 1724)
9. `touch .bootstrapped` (line 1726)
10. **`ensure_matrix`** (line 1736, inside Matrix block lines 1728-1746)
11. **`exec tail -f /dev/null`** (line 1750)

**Verdict:** `ensure_matrix()` runs BEFORE `exec tail -f /dev/null` in CI mode.

---

## Check 3: Bridge PID 1 in Docker — PASS

**File:** `deploy/setup-quick.sh`, two code paths
**What was verified:**

**Bootstrapped path** (lines 1696-1703):
- Detects Docker via `/.dockerenv`, `/run/.containerenv`, or `$container`
- Kills background bridge process
- `exec $INSTALL_DIR/armorclaw-bridge -config ...` replaces shell as PID 1

**First-run path** (lines 1792-1800):
- `DOCKER_MODE` flag set in `start_bridge()` (line 1211) when Docker detected
- After setup completes, kills background bridge, then `exec` bridge as PID 1
- CI mode correctly takes priority: `exec tail` for smoke tests, not bridge

**Verdict:** Bridge correctly becomes PID 1 via `exec` in Docker mode.

---

## Check 4: .bootstrapped Flag — PASS

**File:** `deploy/setup-quick.sh`, `main()` function
**What was verified:**

**Flag check** (lines 1686-1706):
- `bootstrap_flag="$INSTALL_DIR/.bootstrapped"`
- If flag exists: skip all setup, start bridge directly, exit

**Flag creation** (line 1726):
- Created immediately after `verify_health` succeeds (line 1724)
- Before Matrix setup, API key, QR code, and tunnel

**Verdict:** `.bootstrapped` prevents re-setup on container restart.

**Note:** Flag is created before Matrix setup. If Matrix fails on first run, subsequent restarts skip Matrix. Acceptable: bridge is functional without Matrix, and Matrix containers persist independently.

---

## Final Verdict

| Check | Result |
|-------|--------|
| Startup [4/4] | ✅ |
| CI Mode | PASS |
| Config Preservation | PASS |
| Restart (.bootstrapped) | PASS |

**VERDICT: APPROVE**

All 4 "Must Have" startup requirements are correctly implemented in `deploy/setup-quick.sh`.

### Minor Observations (non-blocking)

1. **Bootstrapped flag before Matrix**: `.bootstrapped` is touched at line 1726 (after `verify_health`) but before `ensure_matrix()` at line 1736. If Matrix setup fails, restart skips Matrix. This is intentional — bridge works without Matrix.
2. **CI + Docker first run**: In CI mode on first run, `exec tail` fires instead of bridge PID 1. This is correct for CI smoke tests.
3. **Dockerfile.quickstart**: Dead `quickstart-entrypoint.sh` COPY is removed (confirmed by line 154 comment). No dead code in Docker path.
