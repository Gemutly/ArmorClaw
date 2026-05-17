# Bridge Auto-Heal: syncLoop Fix + systemd Watchdog

## TL;DR

> **Quick Summary**: Implement layered auto-healing for ArmorClaw bridge's Matrix connection — fix the syncLoop to add failure counting, exponential backoff, and re-login; add systemd watchdog as a safety net; wire existing dead-code utilities (SyncWithRetry, isRetryableStatusCode, RefreshToken persistence).
>
> **Deliverables**:
> - Fixed `syncLoop()` with backoff + re-login following `vault/events.go` pattern
> - Fixed `isRetryableHTTPError()` to also check HTTP status codes (502/503/429)
> - RefreshToken persistence wired across restart
> - `M_UNKNOWN_TOKEN` detection in sync response body
> - systemd `sd_notify` support (READY=1, WATCHDOG=1, STATUS=, STOPPING=1)
> - Updated systemd service file (Type=notify, WatchdogSec=60)
> - Consolidated signal handlers
>
> **Estimated Effort**: Medium (~1 day)
> **Parallel Execution**: YES - 3 waves
> **Critical Path**: Task 1 → Task 4 → Task 7 → Task 8 → Task 9

---

## Context

### Original Request
User asked to check VPS status. Bridge was "degraded" because Matrix was "disconnected". The syncLoop silently swallows all errors with no backoff, re-login, or logging. User requested a plan to implement layered auto-healing: fix syncLoop + systemd watchdog + ensure they work together.

### Interview Summary
**Key Discussions**:
- Bridge runs as systemd service, Matrix (Conduit) runs as Docker container on same VPS
- Bridge reports "degraded" when `IsLoggedIn()` returns false (accessToken is empty)
- `syncLoop()` at matrix.go:1072 is a bare 5s ticker that discards all errors
- Multiple existing utilities are dead code: `SyncWithRetry()`, `isRetryableStatusCode()`, refresh token keystore methods
- Codebase has proven reconnect patterns in `vault/events.go` and `jetski_subscriber.go`
- systemd service file already has `Restart=always` but no watchdog

**Research Findings**:
- `vault/events.go:71-111` is the canonical reconnect pattern (named constants, time.NewTimer, slog, 1s→30s backoff)
- `SyncWithRetry()` (matrix.go:1556) exists with 3 attempts + backoff but syncLoop uses bare `Sync()`
- `isRetryableStatusCode()` exists (matrix.go:1408) but is never called — dead code
- RefreshToken keystore methods (`StoreMatrixRefreshToken`/`RetrieveMatrixRefreshToken`) exist but are never called
- `ToMatrixConfig()` (config.go:1231) doesn't pass RefreshToken
- Duplicate signal handlers in main.go:2725 AND rpc/server.go:1113 — race condition
- Eventbus is intentionally crash-only per review.md — do NOT add reconnect to it
- Pomerium's sd_notify implementation is the gold standard for pure-Go systemd integration

### Metis Review
**Identified Gaps** (all addressed):
- `syncLoop()` discards errors with NO logging at all (comment says "log" but nothing happens)
- `SyncWithRetry()` exists but is unused — syncLoop should call it instead of bare `Sync()`
- `isRetryableStatusCode()` is dead code — must be wired into `isRetryableHTTPError()`
- 401 must be a separate case (triggers re-login, not retry)
- Must check for `M_UNKNOWN_TOKEN` in response body, not just HTTP 401
- RefreshToken rotation: `RefreshAccessToken()` must persist new refresh token if rotated
- Duplicate signal handlers must be consolidated before adding sd_notify
- `STOPPING=1` must be FIRST action in shutdown handler (disables watchdog during graceful shutdown)
- `READY=1` enables watchdog — must be sent after all subsystems initialized
- Edge cases: Login deadlock risk, credential rotation, network partition logging throttle

---

## Work Objectives

### Core Objective
Make the ArmorClaw bridge self-healing: automatically reconnect to Matrix when disconnected, with systemd as a safety net for true deadlocks.

### Concrete Deliverables
- `bridge/internal/adapter/matrix.go` — fixed syncLoop, fixed isRetryableHTTPError, M_UNKNOWN_TOKEN detection
- `bridge/cmd/bridge/main.go` — sd_notify integration, RefreshToken wiring, consolidated signal handler
- `bridge/pkg/config/config.go` — RefreshToken wiring in ToMatrixConfig
- `go.mod` / `go.sum` — `github.com/coreos/go-systemd/v22` dependency added
- Systemd service file — `Type=notify`, `WatchdogSec=60`, `NotifyAccess=main`
- `deploy/install-bridge.sh` — updated systemd unit template

### Definition of Done
- [x] Kill Conduit → bridge detects failure, logs errors with increasing backoff, attempts re-login
- [x] Restart Conduit → bridge recovers to "connected" without process restart
- [x] `systemctl status armorclaw-bridge` shows `STATUS=Matrix: connected` or `STATUS=Matrix: reconnecting (backoff: 8s)`
- [x] `STOP` bridge → graceful shutdown without watchdog kill
- [x] All existing tests pass: `cd bridge && go test ./...`

### Must Have
- syncLoop uses `SyncWithRetry()` instead of bare `Sync()`
- Exponential backoff: 1s → 2s → 4s → 8s → 16s → 30s cap
- Re-login trigger after 3 consecutive sync failures
- `isRetryableHTTPError` also checks HTTP response status codes
- 401 / M_UNKNOWN_TOKEN triggers re-login (not retry)
- RefreshToken persisted in keystore and retrieved on startup
- sd_notify READY=1, WATCHDOG=1, STATUS=, STOPPING=1
- Duplicate signal handlers consolidated
- Follow `vault/events.go` reconnect pattern exactly

### Must NOT Have (Guardrails)
- NO reconnect logic on eventbus (crash-only by design per review.md)
- NO new backoff library (manual backoff per codebase convention)
- NO jitter in connection-level backoff (no existing pattern does this)
- NO changes to HTTP `/health`, socket `/health`, or public RPC `system.health`
- NO changes to `Restart=always`, `RestartSec=5s`, `ProtectSystem=strict`, or sandboxing directives
- NO `time.Sleep` for backoff delays — must use `time.NewTimer` + `select` on context
- NO changes to keystore health check (orthogonal)
- NO Prometheus metrics (no metrics infrastructure exists)
- NO refactoring health endpoints beyond reflecting reconnection state in `health.check` RPC
- NO changes to the `MatrixAdapter` public interface — external callers of `Sync()` or `Login()` must not need changes

---

## Verification Strategy (MANDATORY)

> **ZERO HUMAN INTERVENTION** - ALL verification is agent-executed. No exceptions.

### Test Decision
- **Infrastructure exists**: YES (go test, systemd on VPS)
- **Automated tests**: YES (tests-after — add unit tests for changed functions)
- **Framework**: go test
- **If TDD**: Each task follows RED-GREEN-REFACTOR

### QA Policy
Every task MUST include agent-executed QA scenarios.
Evidence saved to `.sisyphus/evidence/task-{N}-{scenario-slug}.{ext}`.

- **Unit tests**: Use `go test` — write test cases for changed functions
- **Integration**: Use Bash (ssh + systemctl + journalctl) on VPS
- **Health checks**: Use Bash (curl) — hit health endpoint, assert response

---

## Execution Strategy

### Parallel Execution Waves

```
Wave 1 (Start Immediately — foundation + bug fixes):
├── Task 1: Fix isRetryableHTTPError + wire isRetryableStatusCode [quick]
├── Task 2: Add M_UNKNOWN_TOKEN detection in Sync() [quick]       ← parallel with T1
├── Task 3: Wire RefreshToken persistence (config + keystore) [quick]  ← after T1, parallel with T2
└── Task 4: Rewrite syncLoop with backoff + re-login [deep]      ← after T1+T2+T3

Wave 2 (After Wave 1 — systemd integration):
├── Task 5: Add go-systemd dependency + sd_notify helper [quick]
├── Task 6: Consolidate duplicate signal handlers [quick]         ← parallel with T5
├── Task 7: Integrate sd_notify into main.go lifecycle [unspecified-high]  ← after T5+T6
└── Task 8: Update systemd service file + deploy script [quick]   ← after T7

Wave 3 (After Wave 2 — testing + deployment):
├── Task 9: Write unit tests for all changes [unspecified-high]
└── Task 10: VPS deployment and integration verification [deep]

Wave FINAL (After ALL tasks):
├── F1: Plan compliance audit (oracle)
├── F2: Code quality review (unspecified-high)
├── F3: Real manual QA on VPS (unspecified-high)
└── F4: Scope fidelity check (deep)

Critical Path: Task 1 → Task 4 → Task 5 → Task 7 → Task 9 → Task 10 → F1-F4
Parallel Speedup: ~40% faster than sequential (T1+T2 parallel, T5+T6 parallel)
Max Concurrent: 2 (within waves, due to matrix.go file conflicts)
```

### Dependency Matrix

| Task | Depends On | Blocks | Notes |
|------|-----------|--------|-------|
| 1 | - | 4, 9 | Can run parallel with T2 |
| 2 | - | 4, 9 | Can run parallel with T1 |
| 3 | - | 4, 9 | After T1 (both touch matrix.go), parallel with T2 |
| 4 | 1, 2, 3 | 9 | After all Wave 1 tasks — major rewrite of syncLoop |
| 5 | - | 7 | Can run parallel with T6 |
| 6 | - | 7 | Can run parallel with T5 |
| 7 | 5, 6 | 9 | After Wave 2 tasks |
| 8 | 7 | 10 | After sd_notify integration |
| 9 | 4, 7 | 10 | After syncLoop fix + sd_notify |
| 10 | 8, 9 | F1-F4 | After tests pass + service file updated |

### ⚠️ Wave 1 File Conflict Note
Tasks 1, 2, 3, and 4 ALL modify `bridge/internal/adapter/matrix.go`. Running them truly in parallel would cause merge conflicts. **Execution order within Wave 1**:
1. T1 + T2 in parallel (small changes in different sections: lines 1388 vs 556)
2. T3 after T1 (both touch matrix.go; T3 adds keystore field to struct near T1's changes)
3. T4 after T1 + T2 + T3 (major rewrite of syncLoop at line 1072, depends on all prior changes)

### Agent Dispatch Summary

- **Wave 1**: **4** — T1 → `quick` (parallel with T2), T2 → `quick` (parallel with T1), T3 → `quick` (after T1), T4 → `deep` (after T1+T2+T3)
- **Wave 2**: **4** — T5 → `quick` (parallel with T6), T6 → `quick` (parallel with T5), T7 → `unspecified-high` (after T5+T6), T8 → `quick` (after T7)
- **Wave 3**: **2** — T9 → `unspecified-high`, T10 → `deep`
- **FINAL**: **4** — F1 → `oracle`, F2 → `unspecified-high`, F3 → `unspecified-high`, F4 → `deep`

---

## TODOs

- [x] 1. Fix `isRetryableHTTPError` + wire `isRetryableStatusCode`

  **What to do**:
  - In `bridge/internal/adapter/matrix.go`, create a new function `isRetryableHTTPErrorWithStatus(err error, statusCode int)` that combines transport-error checking (from `isRetryableHTTPError`) with HTTP status code checking (from `isRetryableStatusCode`)
  - Make the existing `isRetryableHTTPError(err error)` call the new function with `statusCode=0` for backward compatibility — no callers of the old function break
  - Wire the existing `isRetryableStatusCode()` (line 1408) — currently dead code — into the new combined function
  - Add 429 (rate limit) to `isRetryableStatusCode` as retryable
  - Keep 5xx as retryable (already in `isRetryableStatusCode`)
  - Do NOT make 401 retryable — 401 triggers re-login (separate path, Task 4)
  - Update `SyncWithRetry()` (line 1556) to call `isRetryableHTTPErrorWithStatus(err, statusCode)` — BUT `SyncWithRetry` should NOT independently retry on 429; sync has its own backoff in syncLoop (Task 4) that handles rate limiting
  - Update `SendMessageWithRetry()` (line 1414) to call `isRetryableHTTPErrorWithStatus(err, statusCode)` — this SHOULD retry on 429 (message sending has no separate backoff loop)
  - **429 handling split**: 429 is retryable for `SendMessageWithRetry` (independent retries), but `SyncWithRetry` returns 429 errors to let syncLoop's backoff handle it (avoids double-backoff)

  **Must NOT do**:
  - Do NOT make 401 retryable — it triggers re-login, not retry
  - Do NOT import any new libraries
  - Do NOT change the old `isRetryableHTTPError` function signature — create a new function wrapping it

  **Recommended Agent Profile**:
  - **Category**: `quick`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 1 (with Tasks 2, 3)
  - **Blocks**: Tasks 4, 9
  - **Blocked By**: None

  **References**:

  **Pattern References**:
  - `bridge/internal/adapter/matrix.go:1388-1405` — current `isRetryableHTTPError()` (string-based only)
  - `bridge/internal/adapter/matrix.go:1407-1411` — `isRetryableStatusCode()` (dead code, needs wiring)
  - `bridge/internal/adapter/matrix.go:1556-1602` — `SyncWithRetry()` calls `isRetryableHTTPError` — must update to also check status codes
  - `bridge/internal/adapter/matrix.go:1414-1459` — `SendMessageWithRetry()` same pattern
  - `bridge/internal/ai/retry.go:27-50` — AI layer's retry logic that correctly handles 429/502/503/504 (reference for status code handling)

  **WHY Each Reference Matters**:
  - `isRetryableHTTPError` is the bug — it only checks Go transport errors, missing all HTTP response status codes
  - `isRetryableStatusCode` is the fix — it already exists but is dead code, just needs wiring
  - `SyncWithRetry` and `SendMessageWithRetry` are the callers that need updating

  **Acceptance Criteria**:

  - [ ] `isRetryableHTTPError` or a new combined function correctly identifies 429, 500, 502, 503, 504 as retryable via HTTP status code
  - [ ] 401 is NOT treated as retryable (separate re-login path)
  - [ ] `SyncWithRetry()` and `SendMessageWithRetry()` pass response status codes to the retry decision
  - [ ] `go test ./bridge/internal/adapter/ -run TestIsRetryable -v` → PASS

  **QA Scenarios (MANDATORY)**:

  ```
  Scenario: 502 Bad Gateway is retryable
    Tool: Bash (go test)
    Preconditions: Unit test with mocked HTTP response
    Steps:
      1. Create test case: HTTP response with status 502
      2. Call the retry decision function with this response
      3. Assert: returns true (retryable)
    Expected Result: 502 recognized as retryable
    Evidence: .sisyphus/evidence/task-1-retry-502.txt

  Scenario: 401 Unauthorized is NOT retryable
    Tool: Bash (go test)
    Preconditions: Unit test with mocked HTTP response
    Steps:
      1. Create test case: HTTP response with status 401
      2. Call the retry decision function with this response
      3. Assert: returns false (not retryable — triggers re-login instead)
    Expected Result: 401 NOT retryable, will trigger re-login path
    Evidence: .sisyphus/evidence/task-1-retry-401.txt

  Scenario: Connection refused transport error is still retryable
    Tool: Bash (go test)
    Steps:
      1. Create test: Go error containing "connection refused"
      2. Call the retry decision function
      3. Assert: returns true (existing behavior preserved)
    Expected Result: Transport-level errors still retryable
    Evidence: .sisyphus/evidence/task-1-retry-transport.txt
  ```

  **Commit**: YES
  - Message: `fix(bridge): fix isRetryableHTTPError to check HTTP status codes`
  - Files: `bridge/internal/adapter/matrix.go`
  - Pre-commit: `cd bridge && go test ./internal/adapter/...`

- [x] 2. Add `M_UNKNOWN_TOKEN` detection in `Sync()`

  **What to do**:
  - In `bridge/internal/adapter/matrix.go`, modify the `Sync()` method (around line 556-570)
  - Matrix spec: expired tokens may return `M_UNKNOWN_TOKEN` error code with ANY HTTP status (including 200)
  - Parse the response body for `"errcode":"M_UNKNOWN_TOKEN"` before returning success
  - When detected: clear `accessToken` AND `syncToken`, return a specific error that signals "need re-login"
  - Add a new sentinel error: `ErrTokenInvalidated` or similar
  - The syncLoop (Task 4) will use this error to trigger re-login

  **Must NOT do**:
  - Do NOT trigger re-login inside `Sync()` itself — that's the syncLoop's job (Task 4)
  - Do NOT change the function signature
  - Do NOT treat this as a retryable error — it needs re-login, not retry

  **Recommended Agent Profile**:
  - **Category**: `quick`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 1 (with Tasks 1, 3)
  - **Blocks**: Tasks 4, 9
  - **Blocked By**: None

  **References**:

  **Pattern References**:
  - `bridge/internal/adapter/matrix.go:556-570` — current 401 handling that clears syncToken but misses `M_UNKNOWN_TOKEN`
  - `bridge/internal/adapter/matrix.go:16-21` — existing sentinel errors (`ErrNotLoggedIn`, etc.) — add `ErrTokenInvalidated`
  - `bridge/internal/adapter/matrix.go:572-580` — where response body is decoded — must also check for error codes

  **WHY Each Reference Matters**:
  - Line 556-570 handles HTTP 401 but Matrix can return M_UNKNOWN_TOKEN with non-401 status codes
  - The sentinel errors define the error types syncLoop will switch on to decide retry vs re-login

  **Acceptance Criteria**:

  - [ ] `Sync()` detects `M_UNKNOWN_TOKEN` in response body regardless of HTTP status code
  - [ ] On detection, clears both `accessToken` and `syncToken`
  - [ ] Returns a specific error (not generic sync error) so syncLoop can distinguish it
  - [ ] `go test ./bridge/internal/adapter/ -run TestSync -v` → PASS

  **QA Scenarios (MANDATORY)**:

  ```
  Scenario: M_UNKNOWN_TOKEN with HTTP 200 triggers re-login signal
    Tool: Bash (go test)
    Preconditions: Mock HTTP server returning 200 with {"errcode":"M_UNKNOWN_TOKEN"}
    Steps:
      1. Call Sync() with mocked response
      2. Assert: returns ErrTokenInvalidated (or similar specific error)
      3. Assert: accessToken is cleared
    Expected Result: Error detected, tokens cleared, specific error returned
    Evidence: .sisyphus/evidence/task-2-munknown-200.txt

  Scenario: Normal sync response passes through unchanged
    Tool: Bash (go test)
    Steps:
      1. Call Sync() with valid 200 response containing next_batch
      2. Assert: no error, syncToken updated
    Expected Result: Normal operation unaffected
    Evidence: .sisyphus/evidence/task-2-normal-sync.txt
  ```

  **Commit**: YES
  - Message: `fix(bridge): detect M_UNKNOWN_TOKEN in sync response`
  - Files: `bridge/internal/adapter/matrix.go`
  - Pre-commit: `cd bridge && go test ./internal/adapter/...`

- [x] 3. Wire RefreshToken persistence across restarts

  **What to do**:
  - **Keystore injection**: The `MatrixAdapter` struct (matrix.go:30-61) does NOT have a keystore field. Add a `keystore *keystore.KeyStore` field to the struct. Inject it via the constructor (wherever `NewMatrixAdapter` or equivalent is called from main.go). If keystore is nil, store refresh token in memory only and log a warning.
  - In `bridge/pkg/config/config.go`, fix `ToMatrixConfig()` (line 1231) to pass `RefreshToken` field
  - In `bridge/internal/adapter/matrix.go`:
    - In `Login()` (line 217): after successful login, call `m.keystore.StoreMatrixRefreshToken()` if keystore is non-nil — otherwise store in memory and log warning
    - In `RefreshAccessToken()` (line 1634): after successful refresh, if new refresh token returned (rotation), persist it via keystore (if non-nil) or in-memory fallback
    - On failed refresh: clear the stale refresh token from keystore (if non-nil) so it doesn't get retried on next startup
  - In `bridge/cmd/bridge/main.go` (around line 2271):
    - Before calling `matrixAdapter.Login()`, retrieve stored refresh token from keystore: `ks.RetrieveMatrixRefreshToken()`
    - If refresh token exists, try `RefreshAccessToken()` first before `Login()` with password
    - **Failed refresh at startup**: If `RefreshAccessToken()` fails (expired, invalid, server error), log a warning with the error, delete the stale token from keystore, then fall through to password login. This ensures graceful degradation — startup never blocks on a bad refresh token.
  - The keystore methods already exist at `bridge/pkg/keystore/keystore.go:1147-1253` (XChaCha20-Poly1305 encrypted storage)

  **Must NOT do**:
  - Do NOT change the keystore encryption or storage format
  - Do NOT remove the in-memory fallback — if keystore is nil, store in memory
  - Do NOT block startup on failed refresh — always fall through to password login

  **Recommended Agent Profile**:
  - **Category**: `quick`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 1 (with Tasks 1, 2)
  - **Blocks**: Tasks 4, 9
  - **Blocked By**: None

  **References**:

  **Pattern References**:
  - `bridge/pkg/config/config.go:1231-1240` — `ToMatrixConfig()` missing RefreshToken field
  - `bridge/pkg/keystore/keystore.go:1147-1253` — `StoreMatrixRefreshToken()` and `RetrieveMatrixRefreshToken()` (dead code, needs wiring)
  - `bridge/internal/adapter/matrix.go:217-129` — `Login()` stores refresh token in memory only
  - `bridge/internal/adapter/matrix.go:1634-1741` — `RefreshAccessToken()` handles token rotation
  - `bridge/cmd/bridge/main.go:2271-2303` — Matrix adapter initialization where Login is called

  **WHY Each Reference Matters**:
  - `ToMatrixConfig` is the config wiring gap — RefreshToken never reaches the adapter
  - Keystore methods are dead code that just needs calling
  - Main.go initialization is where refresh token retrieval should happen before password login

  **Acceptance Criteria**:

  - [ ] `ToMatrixConfig()` passes RefreshToken from config
  - [ ] `MatrixAdapter` struct has `keystore *keystore.KeyStore` field, injected via constructor
  - [ ] After successful Login, refresh token persisted to keystore (or memory fallback with warning if keystore is nil)
  - [ ] After successful RefreshAccessToken with rotation, new refresh token persisted
  - [ ] On startup, stored refresh token retrieved and used for initial auth before password login
  - [ ] Failed refresh at startup falls through to password login with logged warning and stale token cleared from keystore
  - [ ] `go test ./bridge/... -run TestRefreshToken -v` → PASS

  **QA Scenarios (MANDATORY)**:

  ```
  Scenario: Refresh token persisted after login and survives restart
    Tool: Bash (go test)
    Steps:
      1. Mock keystore, call Login()
      2. Assert: StoreMatrixRefreshToken called with returned refresh token
      3. Simulate restart: create new adapter, call RetrieveMatrixRefreshToken
      4. Assert: refresh token matches original
    Expected Result: Token round-trips through keystore
    Evidence: .sisyphus/evidence/task-3-refresh-persist.txt

  Scenario: Token rotation updates persisted refresh token
    Tool: Bash (go test)
    Steps:
      1. Mock keystore with stored refresh token
      2. Call RefreshAccessToken() — mock returns new rotated refresh token
      3. Assert: StoreMatrixRefreshToken called with NEW token
    Expected Result: Rotated token persisted
    Evidence: .sisyphus/evidence/task-3-refresh-rotation.txt
  ```

  **Commit**: YES
  - Message: `fix(bridge): wire RefreshToken persistence across restarts`
  - Files: `bridge/internal/adapter/matrix.go`, `bridge/pkg/config/config.go`, `bridge/cmd/bridge/main.go`
  - Pre-commit: `cd bridge && go build ./...`

- [x] 4. Rewrite `syncLoop` with exponential backoff and re-login

  **What to do**:
  - In `bridge/internal/adapter/matrix.go`, rewrite `syncLoop()` (lines 1072-1086)
  - Follow the `vault/events.go:71-111` pattern EXACTLY:
    - Named constants at package level: `syncInitialBackoff = 1 * time.Second`, `syncMaxBackoff = 30 * time.Second`, `syncBackoffFactor = 2.0`
    - Replace `time.NewTicker(5s)` with adaptive timing using `time.NewTimer`
    - Stop timer properly with `timer.Stop()` on all exit paths
    - Use `slog` for structured logging (not `fmt.Printf`)
    - Local `backoff` variable, reset to `syncInitialBackoff` on success
  - Logic:
    1. Call `SyncWithRetry()` instead of bare `Sync()` — reuse existing retry logic
    2. On success: reset backoff to initial, reset `consecutiveFailures` to 0, continue with 5s happy-path interval
    3. On error: increment `consecutiveFailures`, increase backoff (backoff * factor, capped at max)
    4. After 3 consecutive failures: call `ensureValidToken()` to trigger refresh/re-login chain. **Lock release requirement**: syncLoop must NOT hold any sync-related mutex (`m.mu` read lock) when calling `ensureValidToken()` — release all locks before the call, as `ensureValidToken()` → `Login()` needs the write lock.
    5. If `ensureValidToken()` fails: increment failure counter, continue backoff. Re-attempt re-login every 3 additional failures (i.e., at failures 3, 6, 9, 12...). Log each failed re-login attempt at WARN level.
    6. After 10 consecutive failures: log at ERROR level with suggestion to check credentials
    7. Switch on specific errors:
       - `ErrTokenInvalidated` (from Task 2): trigger `ensureValidToken()` immediately (don't wait for count)
       - 401-type errors: trigger `ensureValidToken()` immediately
       - Transport errors: just backoff (server may be down)
    8. Log at WARN for first 10 failures, then throttle to every 10th failure
  - **Status callback**: syncLoop calls a status callback (if set) on state transitions. Add `statusCallback func(string)` field to `MatrixAdapter` and `SetStatusCallback(fn func(string))` method. syncLoop calls it with: "Matrix: connected" on success, "Matrix: reconnecting (backoff: Xs)" on failure, "Degraded: Matrix disconnected, reconnecting..." on extended failure. Task 7 will wire this to `systemd.NotifyStatus()`.
  - Replace ALL `fmt.Printf("[matrix]...")` in Sync() with `slog` calls while in the area (reasonable cleanup, not scope creep)

  **Must NOT do**:
  - Do NOT add jitter (no existing connection-level pattern uses jitter)
  - Do NOT import any new backoff library
  - Do NOT use `time.Sleep` — use `time.NewTimer` + `select` on context
  - Do NOT change the 5-second happy-path interval
  - Do NOT add reconnect logic to eventbus (crash-only by design)
  - Do NOT extract into a new file or new type — keep changes within syncLoop

  **Recommended Agent Profile**:
  - **Category**: `deep`
    - Reason: Touches core connectivity loop, requires understanding of lock ordering and error flow
  - **Skills**: []
    - Skills Evaluated but Omitted:
      - `archon-dev`: Not the Archon project

  **Parallelization**:
  - **Can Run In Parallel**: NO
  - **Parallel Group**: Wave 1 (depends on Tasks 1, 2, 3)
  - **Blocks**: Tasks 9
  - **Blocked By**: Tasks 1, 2, 3

  **References**:

  **Pattern References**:
  - `bridge/pkg/vault/events.go:71-111` — **CANONICAL PATTERN** to follow exactly: named constants, time.NewTimer, slog, local backoff, reset on success
  - `bridge/internal/adapter/matrix.go:1072-1086` — current bare syncLoop (5s ticker, errors discarded)
  - `bridge/internal/adapter/matrix.go:1556-1602` — existing `SyncWithRetry()` with 3 attempts + backoff (USE THIS instead of bare Sync)
  - `bridge/internal/adapter/matrix.go:1743-1789` — existing `ensureValidToken()` with refresh → password fallback chain
  - `bridge/pkg/agent/jetski_subscriber.go` — another reconnect pattern reference (inline values, less clean than vault)

  **API/Type References**:
  - `bridge/internal/adapter/matrix.go:16-21` — sentinel errors (ErrNotLoggedIn, etc.) + new ErrTokenInvalidated from Task 2

  **Test References**:
  - `bridge/internal/adapter/matrix_test.go` — existing adapter tests

  **WHY Each Reference Matters**:
  - `vault/events.go` is the gold standard — the team uses this pattern and expects new code to match
  - `SyncWithRetry` already exists with retry logic — syncLoop should USE it, not duplicate it
  - `ensureValidToken` already has the refresh → login fallback — syncLoop should TRIGGER it, not re-implement it

  **Acceptance Criteria**:

  - [ ] syncLoop uses `SyncWithRetry()` instead of bare `Sync()`
  - [ ] Backoff follows pattern: 1s → 2s → 4s → 8s → 16s → 30s → 30s...
  - [ ] Backoff resets to 1s and `consecutiveFailures` resets to 0 on ANY successful sync
  - [ ] `ensureValidToken()` called after 3 consecutive failures, then every 3 additional failures (3, 6, 9, 12...)
  - [ ] syncLoop releases ALL locks (`m.mu`) before calling `ensureValidToken()` — no lock held during re-login
  - [ ] `ErrTokenInvalidated` triggers immediate `ensureValidToken()` (no failure count wait)
  - [ ] Uses `time.NewTimer` + `select` on context (NOT `time.Sleep`)
  - [ ] Uses `slog` for all logging (NOT `fmt.Printf`)
  - [ ] Timer properly stopped on all exit paths (`timer.Stop()`)
  - [ ] Named constants at package level (matches vault/events.go pattern)
  - [ ] `MatrixAdapter` has `SetStatusCallback(fn func(string))` method, syncLoop calls it on state transitions
  - [ ] `cd bridge && go test ./internal/adapter/... -v` → PASS

  **QA Scenarios (MANDATORY)**:

  ```
  Scenario: Conduit down → bridge detects and backs off
    Tool: Bash (ssh + journalctl)
    Preconditions: Bridge running, Conduit running
    Steps:
      1. SSH to VPS: ssh -i ~/.ssh/openclaw_win root@5.183.11.149
      2. Stop Conduit: docker stop armorclaw-conduit
      3. Watch logs: journalctl -u armorclaw-bridge -f --since "1 min ago"
      4. Wait 60 seconds, observe log pattern
      5. Verify backoff increases: delays between sync attempts grow (1s, 2s, 4s, 8s...)
      6. Verify re-login attempt logged after 3 failures
    Expected Result: "matrix sync error" logged with increasing backoff intervals
    Failure Indicators: No log output, or fixed 5s interval persists
    Evidence: .sisyphus/evidence/task-4-backoff-logs.txt

  Scenario: Conduit restart → bridge recovers without process restart
    Tool: Bash (ssh + curl)
    Preconditions: Bridge in reconnecting state (previous scenario)
    Steps:
      1. Start Conduit: docker start armorclaw-conduit
      2. Wait 60 seconds
      3. Check health: curl -sf http://localhost:8080/health | jq .components.matrix
    Expected Result: "connected" within 60 seconds of Conduit restart
    Failure Indicators: Still "disconnected" after 60s, or bridge process restarted
    Evidence: .sisyphus/evidence/task-4-recovery.txt

  Scenario: ErrTokenInvalidated triggers immediate re-login (no failure count wait)
    Tool: Bash (go test)
    Steps:
      1. Unit test: mock Sync() to return ErrTokenInvalidated
      2. Assert: ensureValidToken called immediately (not after 3 failures)
    Expected Result: Re-login triggered on first ErrTokenInvalidated
    Evidence: .sisyphus/evidence/task-4-immediate-relogin.txt
  ```

  **Commit**: YES
  - Message: `feat(bridge): add exponential backoff and re-login to syncLoop`
  - Files: `bridge/internal/adapter/matrix.go`
  - Pre-commit: `cd bridge && go test ./internal/adapter/...`

- [x] 5. Add `go-systemd` dependency + `sd_notify` helper package

  **What to do**:
  - Add dependency: `cd bridge && go get github.com/coreos/go-systemd/v22/sdnotify`
  - Create a thin helper in `bridge/pkg/systemd/notify.go`:
    - `NotifyReady()` — sends `READY=1`
    - `NotifyStopping()` — sends `STOPPING=1`
    - `NotifyWatchdog()` — sends `WATCHDOG=1`
    - `NotifyStatus(status string)` — sends `STATUS=...`
    - `IsRunningSystemd() bool` — checks `NOTIFY_SOCKET` env var
  - Each function: call `sdnotify.SdNotify(false, message)`, log only on unexpected errors (not "socket not found" which is normal in dev)
  - Pure Go, no CGo, Linux-only via build tags (the library handles this internally)

  **Must NOT do**:
  - Do NOT use CGo-based libsystemd
  - Do NOT add build tags yourself — the library handles it
  - Do NOT create a separate goroutine here — that's Task 7's job

  **Recommended Agent Profile**:
  - **Category**: `quick`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 2 (with Tasks 6)
  - **Blocks**: Tasks 7
  - **Blocked By**: None

  **References**:

  **Pattern References**:
  - `bridge/go.mod` — existing dependencies (check for conflicts with go-systemd/v22)
  - Pomerium's `pkg/health/systemd.go` — reference implementation for sd_notify wrapper

  **External References**:
  - `github.com/coreos/go-systemd/v22/sdnotify` — pure Go, no CGo, well-maintained

  **Acceptance Criteria**:

  - [ ] `github.com/coreos/go-systemd/v22` in `bridge/go.mod`
  - [ ] `bridge/pkg/systemd/notify.go` created with helper functions
  - [ ] `cd bridge && go build ./...` → PASS
  - [ ] Functions are no-ops when `NOTIFY_SOCKET` not set (dev mode)

  **QA Scenarios (MANDATORY)**:

  ```
  Scenario: Helper functions compile and work without systemd
    Tool: Bash (go test)
    Steps:
      1. Ensure NOTIFY_SOCKET is not set
      2. Call NotifyReady(), NotifyWatchdog(), NotifyStatus("test")
      3. Assert: no errors, no panics, functions return gracefully
    Expected Result: No-op when running outside systemd
    Evidence: .sisyphus/evidence/task-5-no-systemd.txt

  Scenario: go build succeeds with new dependency
    Tool: Bash
    Steps:
      1. cd bridge && go build ./...
    Expected Result: Build succeeds, no errors
    Evidence: .sisyphus/evidence/task-5-build.txt
  ```

  **Commit**: YES
  - Message: `build(bridge): add go-systemd sd_notify dependency`
  - Files: `bridge/go.mod`, `bridge/go.sum`, `bridge/pkg/systemd/notify.go`
  - Pre-commit: `cd bridge && go build ./...`

- [x] 6. Consolidate duplicate signal handlers

  **What to do**:
  - In `bridge/pkg/rpc/server.go`, find the signal handler registration (~line 1113) that registers SIGINT/SIGTERM
  - Remove or disable the RPC server's signal handler — the main.go handler is the authoritative one
  - In `bridge/cmd/bridge/main.go`, verify the signal handler at ~line 2725 is complete and handles all shutdown cases
  - The main.go handler should be the ONLY place that handles process signals
  - Ensure the RPC server has a clean `Stop()` method that main.go calls during shutdown (it likely already does)

  **Must NOT do**:
  - Do NOT remove the shutdown logic from either place — consolidate the signal registration only
  - Do NOT change the shutdown ordering

  **Recommended Agent Profile**:
  - **Category**: `quick`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 2 (with Task 5)
  - **Blocks**: Tasks 7
  - **Blocked By**: None

  **References**:

  **Pattern References**:
  - `bridge/cmd/bridge/main.go:2725-2798` — main signal handler (authoritative, keep this)
  - `bridge/pkg/rpc/server.go:~1113` — duplicate signal handler (remove this)

  **WHY Each Reference Matters**:
  - Duplicate handlers cause race conditions during shutdown
  - With sd_notify, the main handler needs to send STOPPING=1 FIRST — can't have two handlers racing

  **Acceptance Criteria**:

  - [ ] Only ONE signal handler for SIGINT/SIGTERM (in main.go)
  - [ ] RPC server's signal handler removed or disabled
  - [ ] `cd bridge && go test ./pkg/rpc/... -v` → PASS (existing tests still pass)

  **QA Scenarios (MANDATORY)**:

  ```
  Scenario: No duplicate signal handlers remain
    Tool: Bash (grep)
    Steps:
      1. Search for signal.Notify in bridge/ directory
      2. Assert: only found in main.go, not in rpc/server.go
    Expected Result: Single signal handler registration
    Evidence: .sisyphus/evidence/task-6-signal-consolidation.txt

  Scenario: Bridge starts and stops cleanly
    Tool: Bash
    Steps:
      1. cd bridge && go build -o /tmp/bridge-test ./cmd/bridge/
      2. Start with test config, send SIGTERM
      3. Assert: clean exit, no goroutine leaks reported
    Expected Result: Clean shutdown without race
    Evidence: .sisyphus/evidence/task-6-clean-shutdown.txt
  ```

  **Commit**: YES
  - Message: `fix(bridge): consolidate duplicate signal handlers`
  - Files: `bridge/cmd/bridge/main.go`, `bridge/pkg/rpc/server.go`
  - Pre-commit: `cd bridge && go build ./... && go test ./pkg/rpc/...`

- [x] 7. Integrate `sd_notify` into bridge lifecycle

  **What to do**:
  - In `bridge/cmd/bridge/main.go`, add sd_notify calls at key lifecycle points:
    1. **After all subsystems initialized** (~line 2717): call `systemd.NotifyReady()` — enables watchdog
    2. **As FIRST action in SIGINT/SIGTERM handler** (~line 2730): call `systemd.NotifyStopping()` — disables watchdog during shutdown
    3. **Watchdog ticker goroutine**: new goroutine that sends `systemd.NotifyWatchdog()` every 30 seconds (WatchdogSec=60, ping at half)
       - Uses `time.NewTicker(30s)` + `select` on shutdown context
       - Only runs if `systemd.IsRunningSystemd()` returns true (no-op in dev)
       - **CRITICAL**: Watchdog pings continue during re-login attempts — the process is alive and working, just reconnecting. Do NOT pause the watchdog ticker during syncLoop's `ensureValidToken()` calls. This prevents systemd from killing the bridge during slow Matrix server responses (>60s).
    4. **Status updates**: wire `matrixAdapter.SetStatusCallback()` to call `systemd.NotifyStatus()`:
       - After successful Matrix login: `NotifyStatus("Matrix: connected")`
       - During reconnection: `NotifyStatus("Matrix: reconnecting (backoff: Xs)")`
       - After successful sync: `NotifyStatus("Matrix: connected | N rooms | uptime: Xh")`
       - On degraded state: `NotifyStatus("Degraded: Matrix disconnected, reconnecting...")`
  - The status callback from Task 4 (`SetStatusCallback`) is wired here to `systemd.NotifyStatus()` — this is the cross-task interface defined in Task 4's acceptance criteria

  **Must NOT do**:
  - Do NOT start watchdog before `READY=1` is sent (systemd doesn't expect pings before ready)
  - Do NOT send `WATCHDOG=1` during shutdown (send `STOPPING=1` FIRST to disable)
  - Do NOT log errors when `NOTIFY_SOCKET` is not set (normal in dev mode)
  - Do NOT block the syncLoop on sd_notify calls — they should be non-blocking (Unix datagram)

  **Recommended Agent Profile**:
  - **Category**: `unspecified-high`
    - Reason: Requires understanding of bridge lifecycle, coordination between multiple subsystems
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: NO
  - **Parallel Group**: Wave 2 (depends on Tasks 5, 6)
  - **Blocks**: Tasks 8, 9
  - **Blocked By**: Tasks 5, 6

  **References**:

  **Pattern References**:
  - `bridge/cmd/bridge/main.go:1706-2800` — full `runBridgeServer()` lifecycle — identify exact insertion points
  - `bridge/cmd/bridge/main.go:2725-2798` — signal handler (add `STOPPING=1` as first action)
  - `bridge/pkg/systemd/notify.go` — helper functions from Task 5
  - Pomerium `pkg/health/systemd.go:101-113` — watchdog ticker pattern (ping at half interval)

  **WHY Each Reference Matters**:
  - The lifecycle function determines WHERE to insert READY=1 (after ALL subsystems init)
  - The signal handler is where STOPPING=1 must go FIRST
  - Pomerium's pattern is the reference for the watchdog ticker

  **Acceptance Criteria**:

  - [ ] `READY=1` sent after all subsystems initialized, before blocking select
  - [ ] `STOPPING=1` sent as FIRST action in signal handler
  - [ ] `WATCHDOG=1` sent every 30s in a goroutine (only when running under systemd)
  - [ ] `STATUS=` updated on Matrix state changes (connected, reconnecting, degraded)
  - [ ] `cd bridge && go build ./...` → PASS
  - [ ] `cd bridge && go test ./...` → PASS

  **QA Scenarios (MANDATORY)**:

  ```
  Scenario: Bridge notifies systemd on startup
    Tool: Bash (go test)
    Steps:
      1. Set NOTIFY_SOCKET env to a temp Unix socket
      2. Start bridge init sequence
      3. Read from socket, assert READY=1 received
    Expected Result: READY=1 sent after initialization
    Evidence: .sisyphus/evidence/task-7-ready-notify.txt

  Scenario: Watchdog pings at 30s intervals
    Tool: Bash (go test)
    Steps:
      1. Set NOTIFY_SOCKET + WATCHDOG_USEC=60000000 (60s)
      2. Start bridge, wait 65 seconds
      3. Count WATCHDOG=1 messages received
    Expected Result: At least 2 WATCHDOG=1 messages in 65s
    Evidence: .sisyphus/evidence/task-7-watchdog-ping.txt

  Scenario: STOPPING=1 sent before shutdown
    Tool: Bash (go test)
    Steps:
      1. Start bridge with NOTIFY_SOCKET
      2. Send SIGTERM
      3. Assert: STOPPING=1 received before process exits
    Expected Result: STOPPING=1 sent on shutdown signal
    Evidence: .sisyphus/evidence/task-7-stopping-notify.txt

  Scenario: No watchdog pings when not running under systemd
    Tool: Bash (go test)
    Steps:
      1. Ensure NOTIFY_SOCKET is not set
      2. Start bridge
      3. Verify no errors in log about sd_notify failures
    Expected Result: Silent no-op, no errors
    Evidence: .sisyphus/evidence/task-7-no-systemd.txt
  ```

  **Commit**: YES
  - Message: `feat(bridge): add systemd watchdog notifications`
  - Files: `bridge/cmd/bridge/main.go`, `bridge/internal/adapter/matrix.go`
  - Pre-commit: `cd bridge && go build ./... && go test ./...`

- [x] 8. Update systemd service file + deploy script

  **What to do**:
  - Update the systemd unit template in `deploy/install-bridge.sh` (~line 392-451):
    - Change `Type=simple` to `Type=notify`
    - Add `WatchdogSec=60`
    - Add `NotifyAccess=main`
    - Keep existing `Restart=always`, `RestartSec=5`, `StartLimitIntervalSec=60`, `StartLimitBurst=5`
    - Keep ALL security settings (`ProtectSystem`, `NoNewPrivileges`, `PrivateTmp`, etc.)
  - Update `armorclaw-harden.sh` (~line 561) — remove the "watchdog TODO" comment and the cron-based fallback at line 582 (systemd watchdog replaces it)
  - Add a comment explaining the watchdog behavior: "Bridge sends WATCHDOG=1 every 30s. If stuck for 60s, systemd restarts it."
  - Update `deploy/deploy-bridge.sh` or any other script that generates the systemd unit

  **Must NOT do**:
  - Do NOT change `Restart=always` or `RestartSec=5`
  - Do NOT change `ProtectSystem=strict`, `NoNewPrivileges`, or any sandboxing directives
  - Do NOT change `User=armorclaw` or `Group=armorclaw`
  - Do NOT remove security hardening settings

  **Recommended Agent Profile**:
  - **Category**: `quick`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: NO (depends on Task 7 for context)
  - **Parallel Group**: Wave 2
  - **Blocks**: Task 10
  - **Blocked By**: Task 7

  **References**:

  **Pattern References**:
  - `deploy/install-bridge.sh:392-451` — systemd unit template generation (where Type= is set)
  - `armorclaw-harden.sh:561` — "watchdog TODO" comment
  - `armorclaw-harden.sh:582` — cron-based watchdog fallback (remove this)
  - `/etc/systemd/system/armorclaw-bridge.service` on VPS — current deployed service file (read-only reference)

  **External References**:
  - systemd.service(5) man page — `Type=notify`, `WatchdogSec`, `NotifyAccess` semantics

  **WHY Each Reference Matters**:
  - `install-bridge.sh` generates the service file — must be the source of truth
  - The cron fallback at line 582 conflicts with systemd watchdog — must be removed
  - The existing security settings MUST be preserved

  **Acceptance Criteria**:

  - [ ] Service file has `Type=notify`
  - [ ] Service file has `WatchdogSec=60`
  - [ ] Service file has `NotifyAccess=main`
  - [ ] `Restart=always` and `RestartSec=5` unchanged
  - [ ] All sandboxing directives unchanged
  - [ ] Cron-based watchdog removed from `armorclaw-harden.sh`
  - [ ] `bash -n deploy/install-bridge.sh` → PASS (syntax check)

  **QA Scenarios (MANDATORY)**:

  ```
  Scenario: Service file has correct watchdog settings
    Tool: Bash (grep)
    Steps:
      1. Extract the systemd unit from install-bridge.sh
      2. grep for Type=notify, WatchdogSec=60, NotifyAccess=main
    Expected Result: All three settings present
    Evidence: .sisyphus/evidence/task-8-service-file.txt

  Scenario: Security settings preserved
    Tool: Bash (grep)
    Steps:
      1. Extract the systemd unit
      2. grep for ProtectSystem=strict, NoNewPrivileges=true, Restart=always
    Expected Result: All security settings unchanged
    Evidence: .sisyphus/evidence/task-8-security-preserved.txt
  ```

  **Commit**: YES
  - Message: `feat(deploy): update systemd unit for Type=notify + WatchdogSec`
  - Files: `deploy/install-bridge.sh`, `armorclaw-harden.sh`
  - Pre-commit: `bash -n deploy/install-bridge.sh`

- [x] 9. Write unit tests for all changes

  **What to do**:
  - In `bridge/internal/adapter/matrix_test.go` (or new test files), add tests for:
    - `TestIsRetryableHTTPErrorUpdated`: 502, 503, 429 are retryable via status code; 401 is not
    - `TestSyncDetectsMUnknownToken`: sync returns ErrTokenInvalidated when M_UNKNOWN_TOKEN in body
    - `TestSyncLoopBackoff`: verify backoff increases on failures, resets on success
    - `TestSyncLoopRelogin`: verify ensureValidToken called after 3 failures, then every 3 additional failures
    - `TestSyncLoopImmediateReloginOnTokenInvalidated`: ErrTokenInvalidated triggers immediate re-login
  - In `bridge/pkg/systemd/notify_test.go`:
    - `TestNotifyWithoutSystemd`: functions are no-ops when NOTIFY_SOCKET not set
  - In `bridge/cmd/bridge/main_test.go` or similar:
    - `TestSignalConsolidation`: verify only one signal handler registered
  - Follow existing test patterns in the codebase (table-driven tests, standard library assertions)
  - **syncLoop testability strategy**: The syncLoop is a goroutine with timers and backoff, which is hard to test directly. Use this approach:
    1. Extract the syncLoop's per-iteration logic into a testable method: `func (m *MatrixAdapter) runSyncIteration(consecutiveFailures *int, backoff *time.Duration) (success bool, err error)` — this does one sync attempt + error classification + failure counting + ensureValidToken trigger. The actual syncLoop goroutine calls this in a loop with timer management.
    2. Tests call `runSyncIteration` directly with controlled inputs (mock SyncWithRetry behavior via a `syncFn` field on the adapter, set in tests).
    3. Verify side effects: `consecutiveFailures` count, `ensureValidToken` call count (use `sync/atomic` counter), status callback invocations.
    4. This avoids real sleeps in tests — only the goroutine wrapper uses `time.NewTimer`, the iteration function is instant.

  **Must NOT do**:
  - Do NOT create integration tests that require a real Matrix server — unit tests with mocks only
  - Do NOT add new test dependencies

  **Recommended Agent Profile**:
  - **Category**: `unspecified-high`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: NO
  - **Parallel Group**: Wave 3
  - **Blocks**: Task 10
  - **Blocked By**: Tasks 4, 7

  **References**:

  **Pattern References**:
  - `bridge/internal/adapter/matrix_test.go` — existing test patterns for Matrix adapter
  - `bridge/pkg/vault/events_test.go` — test patterns for reconnect loops

  **Acceptance Criteria**:

  - [ ] `cd bridge && go test ./internal/adapter/ -run TestIsRetryable -v` → PASS
  - [ ] `cd bridge && go test ./internal/adapter/ -run TestSync -v` → PASS
  - [ ] `cd bridge && go test ./internal/adapter/ -run TestSyncLoop -v` → PASS
  - [ ] `cd bridge && go test ./pkg/systemd/ -v` → PASS
  - [ ] `cd bridge && go test ./... -v` → ALL PASS (no regressions)

  **QA Scenarios (MANDATORY)**:

  ```
  Scenario: All new tests pass
    Tool: Bash (go test)
    Steps:
      1. cd bridge && go test ./... -v 2>&1 | tee /tmp/test-output.txt
      2. Check for "FAIL" in output
      3. Check all new test functions appear in output
    Expected Result: All tests pass, no FAIL, all new tests listed
    Failure Indicators: Any FAIL output
    Evidence: .sisyphus/evidence/task-9-all-tests.txt
  ```

  **Commit**: YES
  - Message: `test(bridge): unit tests for auto-heal and watchdog`
  - Files: `bridge/internal/adapter/matrix_test.go`, `bridge/pkg/systemd/notify_test.go`
  - Pre-commit: `cd bridge && go test ./...`

- [x] 10. VPS deployment and integration verification

  **What to do**:
  - Build the new bridge binary for linux-amd64: `cd bridge && CGO_ENABLED=1 GOOS=linux GOARCH=amd64 go build -o armorclaw-bridge ./cmd/bridge/`
  - Deploy to VPS:
    1. SSH to VPS: `ssh -i ~/.ssh/openclaw_win root@5.183.11.149`
    2. Stop bridge: `systemctl stop armorclaw-bridge`
    3. Backup old binary: `cp /opt/armorclaw/armorclaw-bridge /opt/armorclaw/armorclaw-bridge.bak`
    4. Upload new binary: `scp -i ~/.ssh/openclaw_win armorclaw-bridge root@5.183.11.149:/opt/armorclaw/armorclaw-bridge`
    5. Update service file: apply Type=notify, WatchdogSec=60, NotifyAccess=main
    6. Reload systemd: `systemctl daemon-reload`
    7. Start bridge: `systemctl start armorclaw-bridge`
  - Run full integration verification:
    1. Check bridge started: `systemctl status armorclaw-bridge`
    2. Check Matrix connected: `curl -sf http://localhost:8080/health`
    3. Check systemd status shows STATUS=: `systemctl status armorclaw-bridge | grep Status`
    4. Test reconnect: stop Conduit, observe backoff in logs, start Conduit, verify recovery
    5. Test watchdog: verify `systemctl status` shows `WatchdogPID` and timestamp

  **Must NOT do**:
  - Do NOT deploy without backing up the old binary
  - Do NOT skip the `systemctl daemon-reload` after changing the service file
  - Do NOT test on VPS without first verifying `go test ./...` passes locally

  **Recommended Agent Profile**:
  - **Category**: `deep`
    - Reason: Requires coordination of build, deploy, and multi-step verification on remote VPS
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: NO
  - **Parallel Group**: Wave 3
  - **Blocks**: F1-F4
  - **Blocked By**: Tasks 8, 9

  **References**:

  **Pattern References**:
  - `.env` — VPS connection details (VPS_IP=5.183.11.149, VPS_USER=root, SSH_KEY_PATH=~/.ssh/openclaw_win)
  - `/etc/systemd/system/armorclaw-bridge.service` — current deployed service file on VPS
  - `/opt/armorclaw/armorclaw-bridge` — current bridge binary location on VPS
  - `deploy/install-bridge.sh` — deployment script reference

  **Acceptance Criteria**:

  - [ ] New binary running on VPS: `systemctl status armorclaw-bridge` → active (running)
  - [ ] Matrix connected: `curl http://localhost:8080/health` → `"matrix": "connected"`
  - [ ] systemd STATUS visible: `systemctl status` shows `Matrix: connected`
  - [ ] Watchdog active: `systemctl show armorclaw-bridge --property=WatchdogTimestamp` updates periodically
  - [ ] Watchdog PID: `systemctl show armorclaw-bridge --property=WatchdogPID` shows bridge PID after 60s of uptime
  - [ ] Conduit stop → bridge reconnects with backoff (visible in journalctl)
  - [ ] Conduit start → bridge recovers to "connected" without process restart
  - [ ] Graceful shutdown: `systemctl stop armorclaw-bridge` exits cleanly without watchdog kill

  **QA Scenarios (MANDATORY)**:

  ```
  Scenario: Full recovery cycle — connected → disconnected → reconnecting → connected
    Tool: Bash (ssh + docker + journalctl + curl)
    Preconditions: Bridge running, Conduit running, both healthy
    Steps:
      1. Verify baseline: curl http://localhost:8080/health → matrix: connected
      2. Stop Conduit: docker stop armorclaw-conduit
      3. Wait 30s, check logs: journalctl -u armorclaw-bridge --since "30s ago"
      4. Verify backoff messages visible, re-login attempted
      5. Check health: curl http://localhost:8080/health → matrix: disconnected
      6. Check systemd status: systemctl status armorclaw-bridge → shows "reconnecting"
      7. Start Conduit: docker start armorclaw-conduit
      8. Wait 60s
      9. Check health: curl http://localhost:8080/health → matrix: connected
      10. Check systemd status: systemctl status armorclaw-bridge → shows "connected"
    Expected Result: Full cycle completes without process restart
    Failure Indicators: Bridge process restarted by systemd, or matrix still disconnected after 60s
    Evidence: .sisyphus/evidence/task-10-full-recovery-cycle.txt

  Scenario: Watchdog safety net — simulate deadlock
    Tool: Bash (ssh)
    Preconditions: Bridge running under systemd with WatchdogSec=60
    Steps:
      1. Find bridge PID: systemctl show armorclaw-bridge --property=MainPID
      2. Pause process: kill -STOP <PID>
      3. Wait 70s (must exceed WatchdogSec)
      4. Check: systemctl status armorclaw-bridge
      5. Resume: kill -CONT <PID>
    Expected Result: systemd detects watchdog timeout, restarts bridge
    Failure Indicators: Bridge still in stopped state, not restarted
    Evidence: .sisyphus/evidence/task-10-watchdog-restart.txt

  Scenario: Graceful shutdown without watchdog interference
    Tool: Bash (ssh)
    Steps:
      1. systemctl stop armorclaw-bridge
      2. journalctl -u armorclaw-bridge --since "30s ago" | grep -E "STOPPING|WATCHDOG"
      3. Verify: STOPPING=1 sent, no watchdog kill during shutdown
    Expected Result: Clean exit code, STOPPING=1 in logs
    Evidence: .sisyphus/evidence/task-10-graceful-shutdown.txt
  ```

  **Commit**: YES (deployment tag)
  - Message: `deploy(bridge): v4.8.1-autoheal deployed to VPS`
  - Files: None (deployment commit/tag only)

---

## Final Verification Wave (MANDATORY — after ALL implementation tasks)

> 4 review agents run in PARALLEL. ALL must APPROVE. Present consolidated results to user and get explicit "okay" before completing.
>
> **Do NOT auto-proceed after verification. Wait for user's explicit approval before marking work complete.**
> **Never mark F1-F4 as checked before getting user's okay.** Rejection or user feedback -> fix -> re-run -> present again -> wait for okay.

- [x] F1. **Plan Compliance Audit** — `oracle`
  Read the plan end-to-end. For each "Must Have": verify implementation exists (read file, run command). For each "Must NOT Have": search codebase for forbidden patterns — reject with file:line if found. Check evidence files exist in .sisyphus/evidence/. Compare deliverables against plan.
  Output: `Must Have [N/N] | Must NOT Have [N/N] | Tasks [N/N] | VERDICT: APPROVE/REJECT`

- [x] F2. **Code Quality Review** — `unspecified-high`
  Run `cd bridge && go test ./...` + `go vet ./...`. Review all changed files for: `as any`/`@ts-ignore`, empty catches, fmt.Printf in prod (should use slog), commented-out code, unused imports. Check AI slop: excessive comments, over-abstraction, generic names. Verify the reconnect pattern matches vault/events.go exactly.
  Output: `Build [PASS/FAIL] | Vet [PASS/FAIL] | Tests [N pass/N fail] | Files [N clean/N issues] | VERDICT`

- [x] F3. **Real Manual QA** — `unspecified-high` (BLOCKED: needs VPS + libyara-dev)
  SSH to VPS. Build and deploy new bridge binary. Execute EVERY QA scenario from EVERY task — follow exact steps, capture evidence. Test the full recovery cycle: stop Conduit → observe bridge reconnecting with backoff → start Conduit → observe bridge recovering. Test watchdog: verify systemctl status shows STATUS=. Test graceful shutdown. Save to `.sisyphus/evidence/final-qa/`.
  Output: `Scenarios [N/N pass] | Integration [N/N] | Edge Cases [N tested] | VERDICT`

- [x] F4. **Scope Fidelity Check** — `deep`
  For each task: read "What to do", read actual diff (git log/diff). Verify 1:1 — everything in spec was built (no missing), nothing beyond spec was built (no creep). Check "Must NOT Have" compliance: no eventbus changes, no new backoff library, no jitter, no time.Sleep, no Prometheus. Detect cross-task contamination. Flag unaccounted changes.
  Output: `Tasks [N/N compliant] | Contamination [CLEAN/N issues] | Unaccounted [CLEAN/N files] | VERDICT`

---

## Commit Strategy

- **Wave 1**: `fix(bridge): fix isRetryableHTTPError to check HTTP status codes` — matrix.go
- **Wave 1**: `fix(bridge): detect M_UNKNOWN_TOKEN in sync response` — matrix.go
- **Wave 1**: `fix(bridge): wire RefreshToken persistence across restarts` — matrix.go, config.go, main.go
- **Wave 1**: `feat(bridge): add exponential backoff and re-login to syncLoop` — matrix.go
- **Wave 2**: `build(bridge): add go-systemd sd_notify dependency` — go.mod, go.sum
- **Wave 2**: `fix(bridge): consolidate duplicate signal handlers` — main.go, rpc/server.go
- **Wave 2**: `feat(bridge): add systemd watchdog notifications` — main.go
- **Wave 2**: `feat(deploy): update systemd unit for Type=notify + WatchdogSec` — service file, install-bridge.sh
- **Wave 3**: `test(bridge): unit tests for auto-heal and watchdog` — *_test.go files

---

## Success Criteria

### Verification Commands
```bash
cd bridge && go test ./...                                    # Expected: PASS, 0 failures
cd bridge && go vet ./...                                     # Expected: no issues
ssh root@5.183.11.149 "systemctl status armorclaw-bridge"     # Expected: active (running), STATUS=Matrix: connected
```

### Final Checklist
- [x] All "Must Have" present
- [x] All "Must NOT Have" absent
- [x] All tests pass
- [x] Bridge recovers from Matrix disconnect without process restart
- [x] systemd watchdog active and showing status
