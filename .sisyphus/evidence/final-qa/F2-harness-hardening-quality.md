# F2: Code Quality Review — harness-hardening

**Date:** 2026-04-24
**Reviewer:** Sisyphus-Junior (automated)
**Commits reviewed:** 7 (ed93c34 → 516c7e6)

## Files Changed (6 total)

| # | File | Lines Changed | Type |
|---|------|---------------|------|
| 1 | `tests/test-governance-rpc.sh` | +3 | Bash test |
| 2 | `tests/test-matrix-plane.sh` | ~264 lines (rewritten) | Bash test |
| 3 | `tests/test-vps-smoke.sh` | ~459 lines (dual transport) | Bash test |
| 4 | `bridge/cmd/bridge/main.go` | ~28 lines | Go (EventBroadcaster) |
| 5 | `tests/test-persistence.sh` | +288 (new file) | Bash test |
| 6 | `bridge/pkg/http/server.go` | ~20 lines | Go (TLS ciphers) |

---

## Automated Checks

### Build Check: `go vet`
```
$ cd bridge && go vet ./cmd/bridge/ ./pkg/http/
EXIT=0
```
**Result: PASS** (CGO deps were available)

### Lint Check: `bash -n`
```
$ bash -n tests/test-governance-rpc.sh  → PASS
$ bash -n tests/test-matrix-plane.sh    → PASS
$ bash -n tests/test-vps-smoke.sh       → PASS
$ bash -n tests/test-persistence.sh     → PASS
```
**Result: PASS** (all 4 scripts parse cleanly)

### Anti-Pattern Scan

| Check | Result |
|-------|--------|
| `console.log` in changed files | None found |
| `as any` / `@ts-ignore` | N/A (Go/Bash) |
| `TODO`/`FIXME`/`HACK` in bash scripts | None found |
| `TODO` in Go files | Pre-existing only (lines 56, 1426, 1986, 2352-2353, 2756) — none introduced by this PR |
| `fmt.Println` in Go files | Pre-existing CLI output only — appropriate for user-facing setup/daemon CLI |
| `# TODO`/`# FIXME` in test scripts | None found |
| Commented-out code | None found in changed regions |

---

## Per-File Manual Review

### 1. `tests/test-governance-rpc.sh` — T1: Discovery disable
**Change:** Added `[discovery] enabled = false` to temp TOML config (3 lines).

- ✅ `set -euo pipefail` present (line 2)
- ✅ Proper quoting (`"${BASH_SOURCE[0]}"`, `"${SCRIPT_DIR}/../bridge"`)
- ✅ Cleanup trap with `rm -f`/`rm -rf`
- ✅ No AI slop (minimal, targeted change)
- **Verdict: CLEAN**

### 2. `tests/test-matrix-plane.sh` — T2: Rewritten Category B with curl
**Change:** Replaced matrix-commander messaging with direct Conduit API curl functions.

- ✅ `set -euo pipefail` present (line 2)
- ✅ Auto-sources `.env` via `set -a; source .env; set +a`
- ✅ Proper required-variable validation (`:?missing` pattern)
- ✅ `matrix_login()`, `matrix_send()`, `matrix_receive()` are well-structured
- ✅ Unique tokens for message verification (`ARMORCLAW-$(openssl rand -hex 4)`)
- ✅ Polling with bounded attempts (6 iterations × 5s = 30s max)
- ✅ Proper error handling — `|| true` on poll loops to avoid `set -e` exit
- ✅ No word-splitting bugs — all variables properly quoted
- ✅ No AI slop — concise, purposeful code
- **Verdict: CLEAN**

### 3. `tests/test-vps-smoke.sh` — T3: Dual transport
**Change:** Added `detect_transport()`, `rpc_socket()`, dual-socket/HTTP test execution.

- ✅ `set -euo pipefail` present (line 2)
- ✅ Auto-sources `.env`
- ✅ `detect_transport()` properly checks both socat+socket AND HTTP health endpoint
- ✅ `rpc_socket()` correctly uses `"auth"` field in JSON-RPC body (per conventions)
- ✅ `rpc_vps()` correctly uses `Authorization: Bearer` header (per conventions)
- ✅ Tests conditionally skip when transport unavailable (`[SKIP]` messages)
- ✅ Proper quoting throughout — `${VPS_USER}@${VPS_IP}`, `"${ADMIN_TOKEN}"`
- ✅ Transport counters (`SOCKET_TESTS`, `HTTP_TESTS`) for summary
- ⚠️ Minor: `rpc_socket()` line 107 has defensive check `[ "$params" = "\{\}" ]` — overly cautious but harmless
- ✅ No AI slop
- **Verdict: CLEAN**

### 4. `bridge/cmd/bridge/main.go` — T4: SetBroadcaster wire
**Change:** Commit `bed9b2e` restored correct EventBroadcaster wiring flow: create event bus → create HTTPS server → `SetBroadcaster(httpsServer)` → `eventBus.Start()`.

- ✅ Correct ordering: broadcaster set before start (line 2668 before 2671)
- ✅ Nil checks before SetBroadcaster (`eventBus != nil && httpsServer != nil`)
- ✅ Graceful error handling on Start failure (warning, not fatal)
- ✅ Validation check for WebSocket+HTTP dependency restored (lines 2198-2201)
- ✅ No commented-out code in changed regions
- ✅ No unused imports in changed regions
- ℹ️ Pre-existing `fmt.Println` calls are for CLI output — appropriate for this context
- **Verdict: CLEAN**

### 5. `tests/test-persistence.sh` — T5: New persistence test
**Change:** New 288-line script testing invite lifecycle across bridge restart.

- ✅ `set -euo pipefail` present (line 2)
- ✅ Auto-sources `.env` via relative path (`"$(dirname "$0")/../.env"`)
- ✅ Required-variable validation (`:?required` pattern)
- ✅ Phase-based structure (P0-P5) with per-phase pass/fail counters
- ✅ `detect_transport()` with socket preference and HTTP fallback
- ✅ `rpc()` wrapper delegates to detected transport
- ✅ Bridge restart with polling readiness check (15 × 2s = 30s max)
- ✅ Re-detects transport after restart (handles socket recreation delay)
- ✅ Proper cleanup — no temp files left
- ✅ Unique test run ID for traceability (`persist-$(date +%s)-$$`)
- ✅ No AI slop — focused, purposeful test phases
- ✅ Exit 0/1 at end for CI integration
- **Verdict: CLEAN**

### 6. `bridge/pkg/http/server.go` — T6: TLS cipher suite fix
**Change:** Added missing `TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256` and `TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256` ciphers required by Go's HTTP/2 implementation.

- ✅ Correct cipher suite selection — all AEAD, no CBC modes
- ✅ TLS 1.2+ minimum enforced (`MinVersion: tls.VersionTLS12`)
- ✅ Forward-secret ciphers only (all ECDHE)
- ✅ Fixed indentation in CipherSuites block (was misaligned with sibling keys)
- ✅ 9 cipher suites total: 3 TLS 1.3 + 6 TLS 1.2 (covers both ECDSA and RSA certs)
- ✅ No commented-out code
- ✅ No debug logging added
- **Verdict: CLEAN**

---

## AI Slop Assessment

| Indicator | Found? |
|-----------|--------|
| Excessive comments (>30% of file) | No — comments are section headers and brief explanations |
| Over-abstraction (unnecessary interfaces/wrappers) | No — functions are purposeful (rpc_socket, detect_transport) |
| Generic variable names (data, result, thing) | No — names are descriptive (INVITE_ID, TRANSPORT_MODE, RPC_CMD) |
| Unnecessary variables | No — all variables serve a purpose |
| Boilerplate patterns | No — each file has distinct structure |
| Comment explaining obvious code | No |

**AI Slop Rating: NONE DETECTED**

---

## Summary

```
Build [PASS] | Lint [PASS] | Files [6 clean / 0 issues] | VERDICT: PASS
```

### Pre-existing Items (NOT from this PR, not blocking)
- 5 `TODO` markers in `main.go` (lines 56, 1426, 1986, 2352-2353, 2756) — voice package, container start, AppService wiring
- `fmt.Println`/`fmt.Printf` used for CLI user output in `main.go` — appropriate for setup/daemon commands

### Observations
- All 4 test scripts follow consistent conventions: `set -euo pipefail`, `.env` auto-source, `:?required` validation, proper quoting, exit 0/1
- The EventBroadcaster wiring (commit bed9b2e) correctly restored the SetBroadcaster-before-Start ordering after an earlier commit had it wrong
- TLS cipher fix properly addresses Go HTTP/2 requirements for AES_128_GCM ciphers
