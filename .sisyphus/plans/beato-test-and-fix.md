# BEATO Test & Fix Plan — v1.1 Refined

## TL;DR

> **Quick Summary**: Deploy and verify the 5 BEATO capability pillars (Browser, Email, Audio, Text, Office) on the ArmorClaw VPS. Text is production-ready; Browser (Jetski) and Office (Python sidecar) are code-complete but undeployed; Email needs a minimal outbound queue; Audio is audit-only.
>
> **Deliverables**:
> - Jetski browser sidecar deployed via Docker Compose overlay with hardened security
> - Python Office sidecar deployed and YARA `$mz` warning fixed
> - Minimal outbound email queue with SQLite backend
> - Audio truth audit report
> - RPC safety middleware (auth, timeout, rate limiting) on all new RPCs
> - Full BEATO verification report
>
> **Estimated Effort**: Large (6 waves, 26 tasks)
> **Parallel Execution**: YES — 6 waves + final verification
> **Critical Path**: W0 → W1 (Jetski) → W2 (Office+RPC) → W3 (Email+RPC) → W5 (Verify)

---

## Context

### Original Request
Deploy and activate the BEATO capability pillars on the production VPS, addressing the gaps identified in the v1.2 completion report (74% BEATO coverage). User provided CTO-level refinements with a 9.3/10 dispatch rating.

### Interview Summary
**Key Discussions**:
- Jetski deployment method: Docker Compose overlay, NOT ad-hoc `docker run` — user's explicit requirement
- VPS resource constraints: ~2GB RAM, ~800MB used — resource gate required before each wave
- Rollback: must be reversible without touching SQLCipher, Matrix data, or Secretary state
- YARA `$mz` fix: use `$_mz` convention for unreferenced informational strings
- Email queue: scoped to outbound-only, SQLite-backed, 8 statuses, no inbound queue
- RPC safety: auth + timeout + rate limiting + audit events + fail-closed on all new RPCs
- Evidence: standardized naming `beato-<wave>-<task>-{pre,change,test,rollback}.txt`

**Research Findings**:
- `JetskiBroker` is a complete 1536-line Go implementation (`bridge/pkg/browser/jetski_broker.go`)
- `docker-compose.jetski.yml` exists but exposes public ports (9222, 9223) — must be overridden
- Browser RPC handlers exist (`bridge/pkg/rpc/browser.go`, 1032 lines, `BrowserJobManager`)
- Python sidecar compose exists (`deploy/docker-compose.sidecar-py.yml`) — production-ready
- YARA `pe_header_in_non_pe` rule has `$mz` and `$pe` as unreferenced strings (line 82-83)
- Email approval RPC already exists (`bridge/pkg/rpc/email_approval.go`)
- Bridge config TOML has `[browser]` section expected by user's refinement
- Session at max 50 descendants — executor cannot spawn sub-agents

### Metis Review (Self-Analysis)
**Identified Gaps** (addressed):
- Jetski image prerequisite: `armorclaw/jetski:beato` must be pre-built/pushed — added as W0 task
- Docker network conflicts: existing `armorclaw-bridge` vs new `armorclaw-internal` — plan uses separate `armorclaw-internal` network
- Bridge `[browser]` config: must map to JetskiBroker constructor params (`cdpURL`, `rpcURL`)
- Audio wave scope creep risk: locked to audit/truth-check only, no implementation
- Disk space for Docker pulls: added to resource gate checks

### CTO Review (P0 Patches Applied)
**Condition**: CONDITIONAL REJECT → Fixed
**Patches Applied**:
1. ✅ Removed hardcoded ADMIN_TOKEN — replaced with `$ADMIN_TOKEN` env variable reference
2. ✅ Added T2.2b: Document RPC method registration (`document.extract_text`, `document.status`, `document.list_jobs`)
3. ✅ Added T3.0: Email queue RPC method registration (`email.queue_status`, `email.get`, `email.retry`, `email.list`)
4. ✅ Fixed Jetski config mismatch: explicit `jetski_cdp_url` (ws://jetski:9222) and `jetski_rpc_url` (http://jetski:9223), removed port 7331
5. ✅ Fixed Bridge/Jetski networking: compose overlay attaches both bridge and jetski to `armorclaw-internal`
6. ✅ Narrowed RPC safety: reusable helpers first, applied ONLY to new BEATO RPCs (not all 139 existing)
7. ✅ Browser smoke test: deterministic local test first (data: URL), external HTTPS as non-blocking extended smoke
8. ✅ BEATO coverage scoring rubric: 100-point system with per-pillar point breakdown
9. ✅ Added Office sidecar volume/socket mount verification to T2.1

**Note on Browser RPC**: Browser methods ARE already registered (12 methods in server.go lines 1225-1236). `browser.navigate` internally calls `broker.StartJob` + `broker.Navigate` when a broker is configured. No separate `browser.start_job` method exists or is needed.

---

## Work Objectives

### Core Objective
Deploy Jetski browser sidecar, Python Office sidecar, and minimal email queue to the production VPS. Fix YARA warning. Add RPC safety middleware. Verify all BEATO pillars.

### Concrete Deliverables
- `deploy/docker-compose.beato.yml` — Jetski compose overlay
- `deploy/env/beato.env.example` — Environment template
- `bridge/configs/yara_rules.yar` — Fixed `$mz` warning
- `bridge/pkg/email/outbox.go` — Outbound email queue
- `bridge/pkg/email/outbox_test.go` — Queue tests
- `bridge/pkg/rpc/rpc_safety.go` — RPC middleware (auth, timeout, rate limit)
- `bridge/pkg/rpc/rpc_safety_test.go` — Safety tests
- BEATO verification report at `tests/reports/beato-verification-report.md`

### Definition of Done
- [x] Jetski container running on VPS, no public ports, healthcheck passing
- [x] Python sidecar container running on VPS, socket active
- [x] YARA rules compile without warnings
- [x] Email outbox table created, CRUD tested
- [x] RPC safety middleware applied to browser/email/document RPCs
- [x] BEATO verification report shows ≥90% coverage
- [x] No OOM kills in VPS dmesg

### Must Have
- Docker Compose overlay for Jetski (NOT ad-hoc docker run)
- Resource gate before each deployment wave
- Rollback snapshot before any VPS change
- YARA `$mz` fix using `$_mz` convention
- Email outbox: outbound-only, SQLite-backed
- RPC auth + timeout + rate limiting on all new RPCs
- Evidence files for every task (pre/change/test/rollback)
- Bridge + Jetski + Python sidecar running without OOM on 2GB VPS

### Must NOT Have (Guardrails)
- Do NOT expose Jetski on a public host port
- Do NOT let agent containers join the Jetski network
- Do NOT bypass Bridge BrowserBroker policy
- Do NOT add privileged container flags or SYS_ADMIN/NET_ADMIN capabilities
- Do NOT modify `container-setup.sh` or `deploy/container-setup.sh`
- Do NOT restructure `risk_classifier.go`
- Do NOT add structured logging library
- Do NOT use `mattn/go-sqlite3` (CGO conflict with go-sqlcipher)
- Do NOT implement inbound email queue (v1.4)
- Do NOT implement Audio processing (audit only)
- Do NOT add queue UI or multi-worker scheduler
- Do NOT store raw email body in queue table
- Do NOT touch SQLCipher, Matrix data, or Secretary state during rollback

---

## Verification Strategy

> **ZERO HUMAN INTERVENTION** — ALL verification is agent-executed. No exceptions.

### Test Decision
- **Infrastructure exists**: YES (Go test framework, bash test harness)
- **Automated tests**: YES (Tests-after — new code gets tests, deployment gets smoke tests)
- **Framework**: `go test` for Go code, bash for VPS smoke tests

### QA Policy
Every task MUST include agent-executed QA scenarios.
Evidence saved to `.sisyphus/evidence/beato-<wave>-<task>-{pre,change,test,rollback}.txt`.

- **Deployment**: Bash (SSH + Docker commands) — deploy, inspect, verify health
- **Go code**: Bash (`go test`) — compile, test, assert pass
- **VPS integration**: Bash (curl over unix socket) — RPC smoke tests
- **Resource monitoring**: Bash (`free`, `docker stats`) — check RAM/disk/load

---

## Execution Strategy

### Parallel Execution Waves

```
Wave 0 (Baseline + Prerequisites — start immediately):
├── T0.1: Build & push Jetski Docker image [quick]
├── T0.2: Resource safety baseline snapshot [quick]
├── T0.3: Rollback snapshot [quick]
├── T0.4: RPC safety middleware (auth, timeout, rate limit) [unspecified-high]
├── T0.5: RPC safety tests [quick]
└── T0.6: Pre-deploy RPC method registration audit [quick]

Wave 1 (Browser / Jetski — after W0):
├── T1.1: Create BEATO Docker Compose overlay [quick]
├── T1.2: Deploy Jetski on VPS [unspecified-high]
├── T1.3: Wire bridge config to Jetski [quick]
├── T1.4: Browser smoke test (session create/navigate/close) [unspecified-high]
└── T1.5: Browser security verification (no public ports, cap_drop, read_only) [quick]

Wave 2 (Office / Python sidecar — after W1):
├── T2.1: Deploy Python Office sidecar on VPS [unspecified-high]
├── T2.2: Fix YARA unreferenced $mz warning [quick]
├── T2.2b: Register document RPC methods [unspecified-high]
├── T2.3: Office sidecar smoke test [quick]
└── T2.4: Resource check after W2 (Bridge + Jetski + Python) [quick]

Wave 3 (Email outbound queue — after W2):
├── T3.0: Register email queue RPC methods [quick]
├── T3.1: Create email outbox schema + Store [unspecified-high]
├── T3.2: Outbox tests (CRUD + status transitions) [quick]
├── T3.3: Wire outbox into email approval flow [quick]
└── T3.4: Email queue smoke test on VPS [quick]

Wave 4 (Audio truth audit — after W3):
├── T4.1: Audio capability audit report [writing]

Wave 5 (Final BEATO verification — after W4):
├── T5.1: Full BEATO pillar verification [unspecified-high]
├── T5.2: Resource under-load verification [quick]
├── T5.3: Rollback drill [quick]
└── T5.4: BEATO verification report [writing]

Critical Path: T0.1 → T1.1 → T1.2 → T1.3 → T1.4 → T2.1 → T3.1 → T4.1 → T5.1
Parallel Speedup: ~50% (W0 has 6 parallel tasks, W1/W2 partially parallel)
Max Concurrent: 6 (Wave 0)
```

### Dependency Matrix

| Task | Depends On | Blocks |
|------|-----------|--------|
| T0.1 | — | T1.1, T1.2 |
| T0.2 | — | T1.2 (gate) |
| T0.3 | — | T1.2 (rollback) |
| T0.4 | — | T0.5, T1.4, T3.3 |
| T0.5 | T0.4 | T1.4 |
| T0.6 | — | T2.2b, T3.0 (audit reference) |
| T1.1 | T0.1 | T1.2 |
| T1.2 | T0.2, T0.3, T1.1 | T1.3 |
| T1.3 | T1.2 | T1.4 |
| T1.4 | T0.4, T0.5, T1.3 | T1.5 |
| T1.5 | T1.4 | T2.1 |
| T2.1 | T1.5 | T2.3 |
| T2.2 | — | T2.2b, T2.3 |
| T2.2b | T0.4, T0.6 | T2.3 |
| T2.3 | T2.1, T2.2, T2.2b | T2.4 |
| T2.4 | T2.3 | T3.1 |
| T3.0 | T0.4, T0.6 | T3.2 |
| T3.1 | T2.4 | T3.2 |
| T3.2 | T3.0, T3.1 | T3.3 |
| T3.3 | T0.4, T3.2 | T3.4 |
| T3.4 | T3.3 | T4.1 |
| T4.1 | T3.4 | T5.1 |
| T5.1 | T4.1 | T5.2 |
| T5.2 | T5.1 | T5.3 |
| T5.3 | T5.2 | T5.4 |
| T5.4 | T5.3 | — |

### Agent Dispatch Summary

- **W0**: **5** — T0.1→`quick`, T0.2→`quick`, T0.3→`quick`, T0.4→`unspecified-high`, T0.5→`quick`
- **W1**: **5** — T1.1→`quick`, T1.2→`unspecified-high`, T1.3→`quick`, T1.4→`unspecified-high`, T1.5→`quick`
- **W2**: **4** — T2.1→`unspecified-high`, T2.2→`quick`, T2.3→`quick`, T2.4→`quick`
- **W3**: **4** — T3.1→`unspecified-high`, T3.2→`quick`, T3.3→`quick`, T3.4→`quick`
- **W4**: **1** — T4.1→`writing`
- **W5**: **4** — T5.1→`unspecified-high`, T5.2→`quick`, T5.3→`quick`, T5.4→`writing`

---

## TODOs

- [x] T0.1. Build & Push Jetski Docker Image

  **What to do**:
  - Build the Jetski Docker image from `jetski/` directory
  - Tag as `armorclaw/jetski:beato`
  - Push to Docker Hub (or registry accessible from VPS)
  - Verify the image contains: Jetski binary, Lightpanda engine, healthcheck endpoint

  **Must NOT do**:
  - Do NOT modify Jetski source code
  - Do NOT change Jetski configuration

  **Recommended Agent Profile**:
  - **Category**: `quick`
    - Reason: Single Docker build+push command
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 0 (with T0.2, T0.3, T0.4)
  - **Blocks**: T1.1, T1.2
  - **Blocked By**: None

  **References**:

  **Pattern References**:
  - `jetski/Dockerfile` — Jetski Docker build definition
  - `docker-compose.jetski.yml:31` — Current build config (`build: ./jetski`)

  **API/Type References**:
  - `bridge/pkg/browser/jetski_broker.go:24-42` — JetskiBroker constructor expects `cdpURL` (ws://host:9222) and `rpcURL` (http://host:9223)

  **WHY Each Reference Matters**:
  - Dockerfile defines what goes into the image
  - Compose file shows the current build context and expected ports

  **Acceptance Criteria**:

  **QA Scenarios (MANDATORY):**

  ```
  Scenario: Jetski image builds and pushes successfully
    Tool: Bash
    Preconditions: Docker CLI logged into registry
    Steps:
      1. cd jetski && docker build -t armorclaw/jetski:beato .
      2. docker push armorclaw/jetski:beato
      3. docker manifest inspect armorclaw/jetski:beato
    Expected Result: Manifest inspect returns valid JSON with layer info
    Failure Indicators: Build fails, push rejected, manifest not found
    Evidence: .sisyphus/evidence/beato-wave0-t01-pre.txt (docker images before)
              .sisyphus/evidence/beato-wave0-t01-test.txt (manifest inspect output)
  ```

  **Commit**: NO (infrastructure prep, no code changes)

- [x] T0.2. Resource Safety Baseline Snapshot

  **What to do**:
  - SSH to VPS and capture current resource state
  - Record: RAM (free -m), disk (df -h), load average, Docker stats, running containers
  - Verify minimum resources: RAM ≥ 900 MB available, disk ≥ 5 GB free, load < 2.0
  - If below minimums, STOP and report — do not proceed to Wave 1

  **Must NOT do**:
  - Do NOT modify any VPS services
  - Do NOT stop any running containers

  **Recommended Agent Profile**:
  - **Category**: `quick`
    - Reason: Single SSH command, read-only
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 0 (with T0.1, T0.3, T0.4)
  - **Blocks**: T1.2 (gate condition)
  - **Blocked By**: None

  **References**:

  **External References**:
  - VPS: `5.183.11.149`, user `root`, SSH key `~/.ssh/openclaw_win`

  **WHY Each Reference Matters**:
  - Must know VPS access details to run remote checks

  **Acceptance Criteria**:

  **QA Scenarios (MANDATORY):**

  ```
  Scenario: VPS has sufficient resources for Jetski deployment
    Tool: Bash (SSH)
    Preconditions: VPS accessible via SSH
    Steps:
      1. ssh -i ~/.ssh/openclaw_win root@5.183.11.149 'free -m | grep Mem'
      2. ssh -i ~/.ssh/openclaw_win root@5.183.11.149 'df -h / | tail -1'
      3. ssh -i ~/.ssh/openclaw_win root@5.183.11.149 'uptime'
      4. ssh -i ~/.ssh/openclaw_win root@5.183.11.149 'docker stats --no-stream'
    Expected Result: Available RAM ≥ 900MB, Disk free ≥ 5GB, Load < 2.0
    Failure Indicators: RAM < 900MB → STOP WAVE 1
    Evidence: .sisyphus/evidence/beato-wave0-t02-pre.txt (full resource snapshot)
  ```

  **Commit**: NO (read-only snapshot)

- [x] T0.3. Rollback Snapshot

  **What to do**:
  - SSH to VPS and capture rollback state
  - Save: `docker ps` output, bridge container inspect, bridge image inspect, compose config, `/etc/armorclaw` config
  - Store in `/root/armorclaw-rollback/<timestamp>/` on VPS
  - Verify bridge health before snapshot

  **Must NOT do**:
  - Do NOT stop or restart any containers
  - Do NOT modify any configuration

  **Recommended Agent Profile**:
  - **Category**: `quick`
    - Reason: Single SSH command sequence
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 0 (with T0.1, T0.2, T0.4)
  - **Blocks**: T1.2 (rollback prerequisite)
  - **Blocked By**: None

  **References**:

  **External References**:
  - VPS: `5.183.11.149`, user `root`, SSH key `~/.ssh/openclaw_win`
  - ADMIN_TOKEN: Load from `$ADMIN_TOKEN` environment variable or VPS secret file. **NEVER print into evidence files. NEVER commit. Rotate if previously exposed.**

  **WHY Each Reference Matters**:
  - Need VPS access and admin token for bridge health check

  **Acceptance Criteria**:

  **QA Scenarios (MANDATORY):**

  ```
  Scenario: Rollback snapshot captures current VPS state
    Tool: Bash (SSH)
    Preconditions: VPS accessible, bridge running
    Steps:
      1. ssh -i ~/.ssh/openclaw_win root@5.183.11.149 'mkdir -p /root/armorclaw-rollback/$(date +%Y%m%d-%H%M%S)'
      2. ssh -i ~/.ssh/openclaw_win root@5.183.11.149 'ROLLBACK_DIR=$(ls -td /root/armorclaw-rollback/* | head -1) && docker ps --format "{{.Names}} {{.Image}}" > "$ROLLBACK_DIR/docker-ps.txt"'
      3. ssh -i ~/.ssh/openclaw_win root@5.183.11.149 'ROLLBACK_DIR=$(ls -td /root/armorclaw-rollback/* | head -1) && docker inspect armorclaw > "$ROLLBACK_DIR/armorclaw-container.json" 2>/dev/null || true'
      4. ssh -i ~/.ssh/openclaw_win root@5.183.11.149 'ROLLBACK_DIR=$(ls -td /root/armorclaw-rollback/* | head -1) && docker image inspect mikegemut/armorclaw:latest > "$ROLLBACK_DIR/bridge-image.json" 2>/dev/null || true'
      5. ssh -i ~/.ssh/openclaw_win root@5.183.11.149 'ROLLBACK_DIR=$(ls -td /root/armorclaw-rollback/* | head -1) && cp -a /etc/armorclaw "$ROLLBACK_DIR/etc-armorclaw" 2>/dev/null || true'
      6. Verify bridge health: curl --unix-socket /run/armorclaw/bridge.sock http://localhost/health
    Expected Result: Rollback directory populated, bridge health returns OK
    Failure Indicators: Bridge health fails → abort plan
    Evidence: .sisyphus/evidence/beato-wave0-t03-pre.txt (rollback dir listing)
  ```

  **Commit**: NO (VPS state snapshot)

- [x] T0.4. RPC Safety Middleware

  **What to do**:
  - Create `bridge/pkg/rpc/rpc_safety.go` with reusable helper functions:
    - Authn helper: extract and validate credentials from request (support admin token AND user/workflow scoped auth)
    - Authorization helper: method-specific role checking (admin-only for config/security mutation, user/workflow scoped for browser/document/email operations)
    - Timeout helper: wrap handler with context deadline per RPC group (browser=30s, jetski=10s, document=60s, email=30s)
    - Rate limiting helper: per-user rate counter (browser=20/min, jetski=30/min, document=10/min, email=20/min) — in-memory token bucket
    - Audit event helper: log allow/deny/error with method, user, timestamp (no raw PII in logs)
    - Fail-closed helper: if dependency unavailable, return error (not hang)
    - Max request body size enforcement (1MB default)
  - Apply ONLY to new BEATO RPCs (browser.*, document.*, email.queue_*) — do NOT retrofit existing 139 RPC methods
  - Add regression test proving existing v1.2.1 RPC methods still pass unchanged

  **Must NOT do**:
  - Do NOT change existing RPC method signatures
  - Do NOT break existing test assertions
  - Do NOT add external rate limiting library — use in-memory token bucket

  **Recommended Agent Profile**:
  - **Category**: `unspecified-high`
    - Reason: Multi-file changes touching security-critical middleware
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 0 (with T0.1, T0.2, T0.3)
  - **Blocks**: T0.5, T1.4, T3.3
  - **Blocked By**: None

  **References**:

  **Pattern References**:
  - `bridge/pkg/rpc/server.go` — Server struct and RPC dispatch pattern (how handlers are registered)
  - `bridge/pkg/rpc/browser.go:30-35` — BrowserJobManager struct pattern
  - `bridge/pkg/rpc/email_approval.go:38-80` — Existing approval handler pattern (Request validation, ErrorObj returns)

  **API/Type References**:
  - `bridge/pkg/rpc/server.go` — `ErrorObj` struct, `InvalidParams`/`InternalError` error codes
  - `bridge/pkg/rpc/browser.go` — All browser RPC handlers that need middleware wrapping

  **WHY Each Reference Matters**:
  - Must follow the existing RPC dispatch pattern for consistency
  - ErrorObj/InvalidParams are the standard error format
  - Browser handlers show the functions to wrap with safety middleware

  **Acceptance Criteria**:

  **QA Scenarios (MANDATORY):**

  ```
  Scenario: RPC safety middleware blocks unauthenticated requests
    Tool: Bash (go test)
    Preconditions: Code compiles
    Steps:
      1. cd bridge && go test ./pkg/rpc/... -run TestRPCSafetyAuth -count=1 -v
    Expected Result: Test passes — requests without token are rejected with InvalidParams error
    Failure Indicators: Test fails or panics
    Evidence: .sisyphus/evidence/beato-wave0-t04-test.txt (test output)

  Scenario: RPC safety middleware enforces timeouts
    Tool: Bash (go test)
    Preconditions: Code compiles
    Steps:
      1. cd bridge && go test ./pkg/rpc/... -run TestRPCTimeouts -count=1 -v
    Expected Result: Browser RPC times out after 30s, Jetski after 10s, Email after 30s, Document after 60s
    Failure Indicators: Timeout not enforced
    Evidence: .sisyphus/evidence/beato-wave0-t04-test.txt

  Scenario: RPC safety middleware rate limits requests
    Tool: Bash (go test)
    Preconditions: Code compiles
    Steps:
      1. cd bridge && go test ./pkg/rpc/... -run TestRPCRateLimit -count=1 -v
    Expected Result: Requests exceeding rate limit return 429-style error
    Failure Indicators: No rate limiting applied
    Evidence: .sisyphus/evidence/beato-wave0-t04-test.txt
  ```

  **Commit**: YES
  - Message: `feat(beato): add RPC safety middleware — auth, timeout, rate limit`
  - Files: `bridge/pkg/rpc/rpc_safety.go`
  - Pre-commit: `cd bridge && go test ./pkg/rpc/... -count=1`

- [x] T0.5. RPC Safety Tests

  **What to do**:
  - Create `bridge/pkg/rpc/rpc_safety_test.go`
  - Test auth: missing token, invalid token, valid token
  - Test timeout: browser=30s, jetski=10s, email=30s, document=60s
  - Test rate limit: burst within limit, burst exceeding limit, limit reset
  - Test fail-closed: dependency unavailable returns error
  - Test audit logging: allow/deny/error events written
  - Test PII sanitization: no raw values in log output

  **Must NOT do**:
  - Do NOT modify existing test assertions

  **Recommended Agent Profile**:
  - **Category**: `quick`
    - Reason: Test file following established patterns
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: NO
  - **Parallel Group**: Wave 0 (sequential after T0.4)
  - **Blocks**: T1.4
  - **Blocked By**: T0.4

  **References**:

  **Test References**:
  - `bridge/pkg/rpc/email_approval_test.go` — Test structure and mocking patterns
  - `bridge/pkg/browser/browser_test.go` — Browser test patterns

  **WHY Each Reference Matters**:
  - Follow existing test patterns for consistency

  **Acceptance Criteria**:

  **QA Scenarios (MANDATORY):**

  ```
  Scenario: All RPC safety tests pass
    Tool: Bash
    Preconditions: T0.4 complete
    Steps:
      1. cd bridge && go test ./pkg/rpc/... -run "TestRPC" -count=1 -v
    Expected Result: All tests PASS, 0 failures
    Failure Indicators: Any test FAIL
    Evidence: .sisyphus/evidence/beato-wave0-t05-test.txt
  ```

  **Commit**: YES (groups with T0.4)
  - Message: `feat(beato): add RPC safety middleware — auth, timeout, rate limit`
  - Files: `bridge/pkg/rpc/rpc_safety.go`, `bridge/pkg/rpc/rpc_safety_test.go`
  - Pre-commit: `cd bridge && go test ./pkg/rpc/... -count=1`

- [x] T0.6. Pre-Deploy RPC Method Registration Audit

  **What to do**:
  - Before any BEATO code changes, audit ALL places where RPC method names or counts are referenced and update them AFTER T2.2b and T3.0 register new methods
  - Collect the baseline count: run `grep -oP '"[a-z_]+\.[a-z_]+"' bridge/pkg/rpc/server.go | sort -u | wc -l` → currently **129 methods**
  - Find all hardcoded method name lists in test files:
    1. `bridge/pkg/rpc/server_test.go:12-19` — `TestMethodRegistration`: 6 critical methods (hardcoded list)
    2. `bridge/pkg/rpc/server_test.go:34-45` — `TestMethodRegistrationCompleteness`: 10 expected methods (hardcoded list)
    3. `bridge/pkg/rpc/email_approval_test.go:25-37` — email method names in `handlers["approve_email"]`, `handlers["deny_email"]`, `handlers["email_approval_status"]`, `handlers["email.list_pending"]`
    4. `bridge/pkg/rpc/replay_diagnostics_test.go` — `handlers["browser.replay_diagnostics"]`
    5. `tests/test-rpc-methods.sh:119` — tests `health.check`
    6. `tests/test-secretary-lifecycle-proof.sh` — probes 17 secretary/task methods by name
    7. `.github/workflows/test.yml:52-59` — `go test -short ./pkg/rpc/...` (runs all RPC tests, no count check)
  - Create a checklist file `bridge/pkg/rpc/METHOD_AUDIT.md` listing:
    - Current method count (129)
    - Expected count after BEATO (129 + 3 document.* + 4 email.* = 136)
    - Every test file that references method names and what it checks
    - Which tests need updating after new methods are registered
  - This task is a **pre-deploy checkpoint**: the actual updates happen in T2.2b and T3.0, but this task ensures we don't miss any test files

  **Must NOT do**:
  - Do NOT modify any test files yet (T2.2b and T3.0 will do that)
  - Do NOT change server.go registrations
  - Do NOT skip this audit — it prevents CI breakage

  **Recommended Agent Profile**:
  - **Category**: `quick`
    - Reason: Grep-and-document task, no code changes
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 0 (with T0.1-T0.5)
  - **Blocks**: T2.2b, T3.0 (they reference this audit)
  - **Blocked By**: None

  **References**:

  **Pattern References**:
  - `bridge/pkg/rpc/server.go:1224-1360` — Full method registration map (129 unique methods)
  - `bridge/pkg/rpc/server_test.go:12-45` — Two test functions with hardcoded method name lists
  - `bridge/pkg/rpc/email_approval_test.go:25-37` — Email handler test references
  - `bridge/pkg/rpc/replay_diagnostics_test.go:15` — Browser replay handler reference
  - `tests/test-rpc-methods.sh:119` — Critical method smoke test
  - `tests/test-secretary-lifecycle-proof.sh:34-137` — 17 method probes
  - `.github/workflows/test.yml:52-59` — CI test runner (`go test -short ./pkg/rpc/...`)

  **WHY Each Reference Matters**:
  - server.go: source of truth for registered methods — count it
  - server_test.go: most likely to break if methods change — lists must be updated after T2.2b/T3.0
  - email_approval_test.go: will need new email.* method tests after T3.0
  - CI test.yml: runs all RPC tests — will catch any breakage but no hardcoded count to update

  **Acceptance Criteria**:

  **QA Scenarios (MANDATORY):**

  ```
  Scenario: Method audit captures all references
    Tool: Bash
    Preconditions: None
    Steps:
      1. grep -roP 'handlers\["[a-z_.]+"' bridge/pkg/rpc/ | sort -u | wc -l (count handler references in tests)
      2. grep -oP '"[a-z_]+\.[a-z_]+"' bridge/pkg/rpc/server.go | sort -u | wc -l (count registered methods)
      3. Create bridge/pkg/rpc/METHOD_AUDIT.md with both counts
    Expected Result: Audit file created with 129 registered methods, expected 136 after BEATO, list of 7+ test files to update
    Failure Indicators: Audit file missing or incomplete
    Evidence: .sisyphus/evidence/beato-wave0-t06-audit.txt

  Scenario: No numeric count assertions found in CI
    Tool: Bash
    Preconditions: None
    Steps:
      1. grep -rn '== 139\|== 129\|== 136\|len(methods)' .github/ bridge/pkg/rpc/
      2. Expect: no results (no hardcoded numeric count assertions)
    Expected Result: No hardcoded numeric count checks found — only method name lists
    Evidence: .sisyphus/evidence/beato-wave0-t06-no-count-checks.txt
  ```

  **Commit**: YES
  - Message: `chore(beato): add pre-deploy RPC method registration audit`
  - Files: `bridge/pkg/rpc/METHOD_AUDIT.md`
  - Pre-commit: none (documentation only)

- [x] T1.1. Create BEATO Docker Compose Overlay

  **What to do**:
  - Create `deploy/docker-compose.beato.yml` with Jetski service definition
  - Service name: `jetski`
  - Image: `armorclaw/jetski:beato`
  - Container name: `armorclaw-jetski`
  - Security: `read_only: true`, `no-new-privileges`, `cap_drop: ALL`, `mem_limit: 512m`, `pids_limit: 256`
  - Networks: `armorclaw-internal` (internal only, no host access) — both jetski AND bridge must be on this network for Docker DNS resolution
  - Expose ports 9222 (CDP WebSocket) and 9223 (RPC) — EXPOSE only, NO `ports:` mapping to host
  - Volume: `jetski-sessions:/var/lib/armorclaw/jetski`
  - tmpfs: `/tmp:rw,noexec,nosuid,size=128m`
  - Environment: `JETSKI_CDP_BIND=0.0.0.0:9222`, `JETSKI_RPC_BIND=0.0.0.0:9223`, `JETSKI_SESSION_DIR=/var/lib/armorclaw/jetski`
  - Create `deploy/env/beato.env.example` with environment variable template

  **Must NOT do**:
  - Do NOT expose Jetski on a public host port (NO `ports:` section, only `expose:`)
  - Do NOT let agent containers join the Jetski network
  - Do NOT add privileged flags, SYS_ADMIN, NET_ADMIN capabilities

  **Recommended Agent Profile**:
  - **Category**: `quick`
    - Reason: Creating two config files from user-provided spec
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: NO
  - **Parallel Group**: Wave 1 (first task)
  - **Blocks**: T1.2
  - **Blocked By**: T0.1 (image must exist)

  **References**:

  **Pattern References**:
  - `docker-compose.jetski.yml` — Existing Jetski compose (has public ports — DO NOT copy this pattern)
  - `deploy/docker-compose.sidecar-py.yml` — Python sidecar compose (correct security pattern: network_mode none, cap_drop ALL, read_only)
  - `docker-compose.yml:95-128` — Vault service (example of hardened container: read_only, cap_drop ALL, no-new-privileges, resource limits)

  **WHY Each Reference Matters**:
  - Jetski compose shows port structure but has WRONG public port mapping — must override
  - Python sidecar shows the target security posture
  - Vault service shows the hardened container pattern to follow

  **Acceptance Criteria**:

  **QA Scenarios (MANDATORY):**

  ```
  Scenario: BEATO compose overlay validates
    Tool: Bash
    Preconditions: docker-compose.beato.yml created
    Steps:
      1. docker compose -f docker-compose.yml -f deploy/docker-compose.beato.yml config
    Expected Result: Valid compose config output with jetski service defined
    Failure Indicators: Validation error, missing service
    Evidence: .sisyphus/evidence/beato-wave1-t11-change.txt (compose config output)

  Scenario: Jetski has no public port bindings
    Tool: Bash
    Preconditions: Compose config valid
    Steps:
      1. grep -n "ports:" deploy/docker-compose.beato.yml || echo "NO_PUBLIC_PORTS"
    Expected Result: "NO_PUBLIC_PORTS" — no host port mappings
    Failure Indicators: Any "ports:" section found
    Evidence: .sisyphus/evidence/beato-wave1-t11-test.txt
  ```

  **Commit**: YES
  - Message: `feat(beato): add Docker Compose overlay for Jetski deployment`
  - Files: `deploy/docker-compose.beato.yml`, `deploy/env/beato.env.example`

- [x] T1.2. Deploy Jetski on VPS

  **What to do**:
  - Copy `deploy/docker-compose.beato.yml` to VPS
  - Pull `armorclaw/jetski:beato` image on VPS
  - Deploy: `docker compose -f docker-compose.yml -f deploy/docker-compose.beato.yml up -d jetski`
  - Wait for healthcheck: `docker ps --filter name=armorclaw-jetski` shows "healthy"
  - Verify container running, no public ports, security options applied

  **Must NOT do**:
  - Do NOT modify bridge container
  - Do NOT change existing Docker networks
  - Do NOT deploy if resource gate (T0.2) shows < 900 MB available

  **Recommended Agent Profile**:
  - **Category**: `unspecified-high`
    - Reason: Multi-step VPS deployment with verification
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: NO
  - **Parallel Group**: Wave 1 (sequential after T1.1)
  - **Blocks**: T1.3
  - **Blocked By**: T0.2 (resource gate), T0.3 (rollback snapshot), T1.1 (compose file)

  **References**:

  **External References**:
  - VPS: `5.183.11.149`, user `root`, SSH key `~/.ssh/openclaw_win`

  **WHY Each Reference Matters**:
  - VPS access required for deployment

  **Acceptance Criteria**:

  **QA Scenarios (MANDATORY):**

  ```
  Scenario: Jetski container deployed and healthy
    Tool: Bash (SSH)
    Preconditions: T0.2 gate passed, T0.3 snapshot taken, T1.1 compose created
    Steps:
      1. scp -i ~/.ssh/openclaw_win deploy/docker-compose.beato.yml root@5.183.11.149:/opt/armorclaw/
      2. ssh -i ~/.ssh/openclaw_win root@5.183.11.149 'docker pull armorclaw/jetski:beato'
      3. ssh -i ~/.ssh/openclaw_win root@5.183.11.149 'cd /opt/armorclaw && docker compose -f docker-compose.yml -f deploy/docker-compose.beato.yml up -d jetski'
      4. ssh -i ~/.ssh/openclaw_win root@5.183.11.149 'docker ps --filter name=armorclaw-jetski'
      5. ssh -i ~/.ssh/openclaw_win root@5.183.11.149 'docker inspect armorclaw-jetski --format "{{json .HostConfig.SecurityOpt}}"'
      6. ssh -i ~/.ssh/openclaw_win root@5.183.11.149 'docker inspect armorclaw-jetski --format "{{json .HostConfig.CapDrop}}"'
      7. ssh -i ~/.ssh/openclaw_win root@5.183.11.149 'docker inspect armorclaw-jetski --format "{{json .HostConfig.Memory}}"'
    Expected Result: Container running, SecurityOpt contains "no-new-privileges:true", CapDrop contains "ALL", Memory limit set
    Failure Indicators: Container not running, missing security options, no memory limit
    Evidence: .sisyphus/evidence/beato-wave1-t12-change.txt (docker ps + inspect output)

  Scenario: Jetski not exposed on public ports
    Tool: Bash (SSH)
    Preconditions: Jetski deployed
    Steps:
      1. ssh -i ~/.ssh/openclaw_win root@5.183.11.149 'docker inspect armorclaw-jetski --format "{{json .HostConfig.PortBindings}}"'
      2. ssh -i ~/.ssh/openclaw_win root@5.183.11.149 'docker port armorclaw-jetski || echo "NO_PUBLIC_PORTS"'
    Expected Result: PortBindings null/empty, "NO_PUBLIC_PORTS" from docker port
    Failure Indicators: Any host port binding found
    Evidence: .sisyphus/evidence/beato-wave1-t12-test.txt
  ```

  **Commit**: NO (VPS deployment, no code changes)

- [x] T1.3. Wire Bridge Config to Jetski

  **What to do**:
  - Update bridge config on VPS (`/etc/armorclaw/config.toml` via volume mount)
  - Add `[browser]` section with `backend = "jetski"`, `jetski_cdp_url = "ws://jetski:9222"`, `jetski_rpc_url = "http://jetski:9223"` (explicit CDP and RPC URLs matching JetskiBroker constructor)
  - The compose overlay must attach BOTH bridge and jetski to `armorclaw-internal` network so Docker DNS resolves `jetski` hostname from the bridge container
  - Restart bridge container to pick up config change
  - Verify bridge can reach Jetski via Docker network DNS

  **Must NOT do**:
  - Do NOT modify bridge source code (config change only)
  - Do NOT change the Bridge container's Dockerfile
  - Do NOT remove existing bridge network attachments (only ADD armorclaw-internal via overlay)

  **Recommended Agent Profile**:
  - **Category**: `quick`
    - Reason: Single config edit + container restart
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: NO
  - **Parallel Group**: Wave 1 (sequential after T1.2)
  - **Blocks**: T1.4
  - **Blocked By**: T1.2

  **References**:

  **Pattern References**:
  - `bridge/pkg/browser/jetski_broker.go:54-60` — `NewJetskiBroker(cdpURL, rpcURL, logger)` — config must provide these URLs

  **External References**:
  - VPS: `5.183.11.149`

  **WHY Each Reference Matters**:
  - JetskiBroker constructor defines the URL format the config must provide

  **Acceptance Criteria**:

  **QA Scenarios (MANDATORY):**

  ```
  Scenario: Bridge config updated with Jetski backend
    Tool: Bash (SSH)
    Preconditions: Jetski container running (T1.2)
    Steps:
      1. ssh -i ~/.ssh/openclaw_win root@5.183.11.149 'docker exec armorclaw cat /data/config.toml | grep -A5 "\[browser\]"'
      2. ssh -i ~/.ssh/openclaw_win root@5.183.11.149 'docker restart armorclaw'
      3. Wait 10s for bridge startup
      4. curl --unix-socket /run/armorclaw/bridge.sock http://localhost/health
    Expected Result: Config shows [browser] section with jetski backend, bridge health OK after restart
    Failure Indicators: Config missing, bridge fails to start
    Evidence: .sisyphus/evidence/beato-wave1-t13-change.txt
  ```

  **Commit**: NO (VPS config change)

- [x] T1.4. Browser Smoke Test

  **What to do**:
  - Create `tests/test-browser-smoke-beato.sh` test script
  - Note: browser RPC methods ARE already registered (12 methods including browser.navigate, browser.fill, browser.click, browser.status, etc.)
  - The `browser.navigate` method internally calls `broker.StartJob` + `broker.Navigate` when a broker is configured
  - Test 1: Navigate to a static data URL (`data:text/html,<h1>BEATO Test</h1>`) — deterministic, no DNS/TLS dependency
  - Test 2: Navigate to `https://example.com` — external smoke (non-blocking)
  - Test 3: Get browser status
  - Test 4: Complete/close session
  - Run against VPS bridge via unix socket

  **Must NOT do**:
  - Do NOT modify browser source code

  **Recommended Agent Profile**:
  - **Category**: `unspecified-high`
    - Reason: Multi-step integration test against live VPS
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: NO
  - **Parallel Group**: Wave 1 (sequential after T1.3)
  - **Blocks**: T1.5
  - **Blocked By**: T0.4 (RPC safety), T0.5 (safety tests), T1.3 (bridge config)

  **References**:

  **Pattern References**:
  - `bridge/pkg/rpc/browser.go:51-65` — BrowserJobManager.CreateJob pattern
  - `bridge/pkg/browser/jetski_broker.go:83-145` — StartJob RPC flow

  **Test References**:
  - `tests/test-jetski-sidecar.sh` — Existing Jetski test patterns

  **External References**:
  - ADMIN_TOKEN: Load from `$ADMIN_TOKEN` environment variable. **NEVER print into evidence. NEVER commit.**

  **WHY Each Reference Matters**:
  - BrowserJobManager shows the RPC method names to call
  - JetskiBroker shows expected request/response format
  - Existing test script shows the bash test pattern

  **Acceptance Criteria**:

  **QA Scenarios (MANDATORY):**

  ```
  Scenario: Browser session lifecycle works end-to-end (deterministic)
    Tool: Bash (SSH + curl)
    Preconditions: Bridge running with Jetski config (T1.3)
    Steps:
      1. Navigate to static data URL via browser.navigate: ssh -i ~/.ssh/openclaw_win root@5.183.11.149 "curl --unix-socket /run/armorclaw/bridge.sock -X POST http://localhost/ -H 'Content-Type: application/json' -d '{\"jsonrpc\":\"2.0\",\"id\":1,\"method\":\"browser.navigate\",\"params\":{\"agent_id\":\"beato-test\",\"url\":\"data:text/html,<h1>BEATO+Test</h1>\"},\"auth\":\"$ADMIN_TOKEN\"}'"
      2. Parse response for job_id and success=true
      3. Poll browser.status with job_id until status=running
      4. Complete: method browser.complete with job_id
    Expected Result: Navigate returns job_id, status shows running, complete succeeds — all deterministic, no external DNS/TLS
    Failure Indicators: Any step returns error or timeout
    Evidence: .sisyphus/evidence/beato-wave1-t14-test.txt

  Scenario: Browser navigates to external HTTPS (non-blocking extended smoke)
    Tool: Bash (SSH + curl)
    Preconditions: Deterministic test passed
    Steps:
      1. Navigate to https://example.com via browser.navigate
      2. Expect success (may fail if VPS has no outbound HTTPS — that's OK for first pass)
    Expected Result: Navigation succeeds OR fails with clear network error (not crash)
    Evidence: .sisyphus/evidence/beato-wave1-t14-test-external.txt

  Scenario: Browser session fails gracefully with invalid URL
    Tool: Bash (SSH + curl)
    Preconditions: Bridge running
    Steps:
      1. Navigate to "not-a-url" via browser.navigate
      2. Expect error response with clear message
    Expected Result: Error returned, no crash, bridge still healthy
    Evidence: .sisyphus/evidence/beato-wave1-t14-test-error.txt
  ```

  **Commit**: YES
  - Message: `test(beato): add browser smoke test for Jetski VPS deployment`
  - Files: `tests/test-browser-smoke-beato.sh`

- [x] T1.5. Browser Security Verification

  **What to do**:
  - Verify Jetski container security posture on VPS
  - Check: SecurityOpt, CapDrop, Memory limit, ReadOnlyRootFs, Network isolation
  - Verify: No public port exposure, agent containers cannot reach Jetski network
  - Document findings as evidence

  **Must NOT do**:
  - Do NOT modify container configuration

  **Recommended Agent Profile**:
  - **Category**: `quick`
    - Reason: Read-only verification commands
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: NO
  - **Parallel Group**: Wave 1 (sequential after T1.4)
  - **Blocks**: T2.1
  - **Blocked By**: T1.4

  **References**:

  **Pattern References**:
  - `deploy/docker-compose.sidecar-py.yml` — Target security posture (cap_drop ALL, read_only, no-new-privileges)

  **WHY Each Reference Matters**:
  - Python sidecar is the security benchmark to compare against

  **Acceptance Criteria**:

  **QA Scenarios (MANDATORY):**

  ```
  Scenario: Jetski container meets all security requirements
    Tool: Bash (SSH)
    Preconditions: T1.2 complete
    Steps:
      1. ssh -i ~/.ssh/openclaw_win root@5.183.11.149 'docker inspect armorclaw-jetski --format "{{json .HostConfig.SecurityOpt}}"'
      2. ssh -i ~/.ssh/openclaw_win root@5.183.11.149 'docker inspect armorclaw-jetski --format "{{json .HostConfig.CapDrop}}"'
      3. ssh -i ~/.ssh/openclaw_win root@5.183.11.149 'docker inspect armorclaw-jetski --format "{{json .HostConfig.Memory}}"'
      4. ssh -i ~/.ssh/openclaw_win root@5.183.11.149 'docker inspect armorclaw-jetski --format "{{json .HostConfig.ReadonlyRootfs}}"'
      5. ssh -i ~/.ssh/openclaw_win root@5.183.11.149 'docker inspect armorclaw-jetski --format "{{json .NetworkSettings.Networks}}"'
    Expected Result: no-new-privileges:true, CapDrop=[ALL], Memory=536870912 (512MB), ReadonlyRootfs=true, Networks does NOT include bridge-net or host
    Failure Indicators: Missing any security control
    Evidence: .sisyphus/evidence/beato-wave1-t15-test.txt
  ```

  **Commit**: NO (verification only)

- [x] T2.1. Deploy Python Office Sidecar on VPS

  **What to do**:
  - Deploy using existing `deploy/docker-compose.sidecar-py.yml`
  - Pull `armorclaw/sidecar-office:latest` image on VPS
  - Start sidecar: `docker compose -f deploy/docker-compose.sidecar-py.yml up -d`
  - Verify container running, socket active at `/run/armorclaw/sidecar-office.sock`
  - Verify network_mode: none, cap_drop: ALL, read_only: true

  **Must NOT do**:
  - Do NOT modify the Python sidecar source code
  - Do NOT change the sidecar Dockerfile

  **Recommended Agent Profile**:
  - **Category**: `unspecified-high`
    - Reason: VPS deployment + multi-step verification
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: NO (sequential after W1)
  - **Parallel Group**: Wave 2 (first task)
  - **Blocks**: T2.3
  - **Blocked By**: T1.5

  **References**:

  **Pattern References**:
  - `deploy/docker-compose.sidecar-py.yml` — Complete compose definition (copy to VPS and deploy as-is)

  **WHY Each Reference Matters**:
  - This is the exact compose file to deploy — no modifications needed

  **Acceptance Criteria**:

  **QA Scenarios (MANDATORY):**

  ```
  Scenario: Python Office sidecar deployed and socket active
    Tool: Bash (SSH)
    Preconditions: T1.5 complete, resource gate passed
    Steps:
      1. ssh -i ~/.ssh/openclaw_win root@5.183.11.149 'docker pull armorclaw/sidecar-office:latest'
      2. scp -i ~/.ssh/openclaw_win deploy/docker-compose.sidecar-py.yml root@5.183.11.149:/opt/armorclaw/
      3. ssh -i ~/.ssh/openclaw_win root@5.183.11.149 'cd /opt/armorclaw && docker compose -f deploy/docker-compose.sidecar-py.yml up -d'
      4. ssh -i ~/.ssh/openclaw_win root@5.183.11.149 'docker ps --filter name=armorclaw-sidecar-office'
      5. ssh -i ~/.ssh/openclaw_win root@5.183.11.149 'test -S /run/armorclaw/sidecar-office.sock && echo "SOCKET_ACTIVE" || echo "SOCKET_MISSING"'
      6. ssh -i ~/.ssh/openclaw_win root@5.183.11.149 'docker inspect armorclaw-sidecar-office --format "{{json .HostConfig.NetworkMode}}"'
    Expected Result: Container running, SOCKET_ACTIVE, NetworkMode="none"
    Failure Indicators: Container not running, socket missing, network mode not none
    Evidence: .sisyphus/evidence/beato-wave2-t21-change.txt
  ```

  **Commit**: NO (VPS deployment)

- [x] T2.2. Fix YARA Unreferenced `$mz` Warning

  **What to do**:
  - Open `bridge/configs/yara_rules.yar`
  - In rule `pe_header_in_non_pe` (line 77-87):
    - `$mz` on line 82 is unreferenced in condition (condition uses `$this_program`)
    - `$pe` on line 83 is also unreferenced in condition
  - Fix: rename `$mz` → `$_mz` and `$pe` → `$_pe` (YARA ignores unreferenced strings prefixed with `_`)
  - This preserves the informational strings without warnings
  - Run `go test ./pkg/yara/... -count=1` to verify no warnings

  **Must NOT do**:
  - Do NOT delete the MZ/PE strings — they are informational
  - Do NOT weaken malware/CDR detection
  - Do NOT change the condition (`$this_program`)

  **Recommended Agent Profile**:
  - **Category**: `quick`
    - Reason: Two-character rename in a config file
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES (independent of T2.1)
  - **Parallel Group**: Wave 2 (with T2.1)
  - **Blocks**: T2.2b, T2.3
  - **Blocked By**: None

  **References**:

  **Pattern References**:
  - `bridge/configs/yara_rules.yar:77-87` — The `pe_header_in_non_pe` rule containing `$mz` (line 82) and `$pe` (line 83) that are unreferenced in condition (only `$this_program` is used on line 86)

  **WHY Each Reference Matters**:
  - Exact location of the fix — rename $mz to $_mz and $pe to $_pe

  **Acceptance Criteria**:

  **QA Scenarios (MANDATORY):**

  ```
  Scenario: YARA rules compile without warnings after fix
    Tool: Bash
    Preconditions: Fix applied
    Steps:
      1. cd bridge && go test ./pkg/yara/... -count=1 -v
      2. grep -n '\$mz\|\$pe' configs/yara_rules.yar (should only show $_mz and $_pe)
    Expected Result: All tests pass, no YARA compilation warnings, grep shows $_mz and $_pe (not bare $mz or $pe)
    Failure Indicators: Any test failure or warning
    Evidence: .sisyphus/evidence/beato-wave2-t22-change.txt (grep output)
              .sisyphus/evidence/beato-wave2-t22-test.txt (test output)
  ```

  **Commit**: YES
  - Message: `fix(yara): resolve unreferenced $mz and $pe warnings in pe_header_in_non_pe rule`
  - Files: `bridge/configs/yara_rules.yar`
  - Pre-commit: `cd bridge && go test ./pkg/yara/... -count=1`

- [x] T2.2b. Register Document RPC Methods

  **What to do**:
  - Register `document.*` RPC methods in `bridge/pkg/rpc/server.go`
  - Methods to register:
    - `document.extract_text` — route through `sidecar.RouteExtractText()` with YARA scan, MIME/magic validation, and strict drop on mismatch
    - `document.status` — check extraction job status
    - `document.list_jobs` — list extraction jobs for a workflow
  - Auth required on all document RPCs (use RPC safety helpers from T0.4)
  - Apply 3-layer routing: native text bypass → valid magic+MIME → strict drop on mismatch
  - No raw document text in logs
  - Results stored as artifacts via Secretary artifact API

  **Must NOT do**:
  - Do NOT modify existing test assertions
  - Do NOT store raw document content in audit logs

  **Recommended Agent Profile**:
  - **Category**: `unspecified-high`
    - Reason: New RPC method registration with security routing logic
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES (after T2.2)
  - **Parallel Group**: Wave 2 (with T2.3)
  - **Blocks**: T2.3
  - **Blocked By**: T0.4 (RPC safety helpers), T2.2 (YARA fix)

  **References**:

  **Pattern References**:
  - `bridge/pkg/rpc/server.go:1225-1236` — Browser RPC registration pattern (map of method name → handler)
  - `bridge/pkg/sidecar/office_client.go:36-118` — `RouteExtractText()` 3-layer routing implementation (native text → Python/Rust/Java sidecar → strict drop)

  **API/Type References**:
  - `bridge/pkg/sidecar/office_client.go:40-46` — `RouteExtractText(ctx, req, officeClient, rustClient, javaClient)` signature
  - `bridge/pkg/rpc/server.go:309-311` — `SetBrowserBroker()` pattern for injecting dependencies

  **WHY Each Reference Matters**:
  - Server.go shows the exact registration pattern to follow
  - RouteExtractText is the core routing function to call from the RPC handler
  - BrowserBroker pattern shows how to inject sidecar clients into the server

  **Acceptance Criteria**:

  **QA Scenarios (MANDATORY):**

  ```
  Scenario: Document RPC methods registered and callable
    Tool: Bash (go test)
    Preconditions: Code compiles
    Steps:
      1. cd bridge && go test ./pkg/rpc/... -run TestDocumentMethodRegistration -count=1 -v
    Expected Result: document.extract_text, document.status, document.list_jobs all registered
    Failure Indicators: Method not found in registration map
    Evidence: .sisyphus/evidence/beato-wave2-t22b-test.txt

  Scenario: Existing v1.2.1 RPC methods still pass unchanged
    Tool: Bash (go test)
    Steps:
      1. cd bridge && go test ./pkg/rpc/... -count=1
    Expected Result: All existing tests pass (regression)
    Evidence: .sisyphus/evidence/beato-wave2-t22b-regression.txt
  ```

  **Commit**: YES
  - Message: `feat(beato): register document RPC methods with 3-layer routing`
  - Files: `bridge/pkg/rpc/server.go`, `bridge/pkg/rpc/document.go` (new)
  - Pre-commit: `cd bridge && go test ./pkg/rpc/... ./pkg/sidecar/... -count=1`

- [x] T2.3. Office Sidecar Smoke Test

  **What to do**:
  - Test document processing through the deployed sidecar on VPS
  - Send a small test document (e.g., .xlsx) through the bridge RPC
  - Verify text extraction returns content
  - Verify error handling for invalid input

  **Must NOT do**:
  - Do NOT modify sidecar or bridge code

  **Recommended Agent Profile**:
  - **Category**: `quick`
    - Reason: Single integration test via curl
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: NO
  - **Parallel Group**: Wave 2 (after T2.1 and T2.2)
  - **Blocks**: T2.4
  - **Blocked By**: T2.1, T2.2

  **References**:

  **Pattern References**:
  - `bridge/pkg/rpc/server.go` — Document RPC methods registered in server

  **WHY Each Reference Matters**:
  - Need to know the exact RPC method name for document extraction

  **Acceptance Criteria**:

  **QA Scenarios (MANDATORY):**

  ```
  Scenario: Office document extraction works via sidecar
    Tool: Bash (SSH + curl)
    Preconditions: T2.1 sidecar running, T2.2 YARA fixed
    Steps:
      1. Create a small test xlsx file on VPS
      2. Submit via bridge RPC for text extraction
      3. Verify response contains extracted text
    Expected Result: Text extraction returns document content
    Failure Indicators: RPC error, empty response, timeout
    Evidence: .sisyphus/evidence/beato-wave2-t23-test.txt

  Scenario: Invalid document handled gracefully
    Tool: Bash (SSH + curl)
    Preconditions: Sidecar running
    Steps:
      1. Submit corrupt file bytes via RPC
      2. Verify error response (not crash)
    Expected Result: Error returned, sidecar and bridge still healthy
    Evidence: .sisyphus/evidence/beato-wave2-t23-test-error.txt
  ```

  **Commit**: NO (VPS test only)

- [x] T2.4. Resource Check After Wave 2

  **What to do**:
  - Record VPS resource state with Bridge + Jetski + Python sidecar all running
  - Verify: available RAM ≥ 250 MB, no OOM kills, disk free ≥ 3 GB
  - If below thresholds, stop and report

  **Must NOT do**:
  - Do NOT modify any containers

  **Recommended Agent Profile**:
  - **Category**: `quick`
    - Reason: Single SSH read command
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: NO
  - **Parallel Group**: Wave 2 (last task)
  - **Blocks**: T3.1
  - **Blocked By**: T2.3

  **References**: (none needed — VPS resource check)

  **Acceptance Criteria**:

  **QA Scenarios (MANDATORY):**

  ```
  Scenario: VPS stable with all W2 services running
    Tool: Bash (SSH)
    Preconditions: T2.3 complete
    Steps:
      1. ssh -i ~/.ssh/openclaw_win root@5.183.11.149 'free -m | grep Mem'
      2. ssh -i ~/.ssh/openclaw_win root@5.183.11.149 'df -h / | tail -1'
      3. ssh -i ~/.ssh/openclaw_win root@5.183.11.149 'docker stats --no-stream'
      4. ssh -i ~/.ssh/openclaw_win root@5.183.11.149 'dmesg | grep -i "oom-kill" | tail -5'
    Expected Result: Available RAM ≥ 250MB, no OOM kills, disk ≥ 3GB
    Failure Indicators: RAM < 250MB → STOP W3, OOM kills found
    Evidence: .sisyphus/evidence/beato-wave2-t24-test.txt
  ```

  **Commit**: NO (verification only)

- [x] T3.0. Register Email Queue RPC Methods

  **What to do**:
  - Register `email.*` queue RPC methods in `bridge/pkg/rpc/server.go`
  - Existing methods: `email_approval_status`, `email.list_pending` (already registered)
  - New methods to register:
    - `email.queue_status` — get queue statistics (pending, retrying, dead_letter counts)
    - `email.get` — get outbox entry by ID
    - `email.retry` — retry a dead_letter or failed email
    - `email.list` — list outbox entries with status filter
  - Auth required (use RPC safety helpers from T0.4)

  **Must NOT do**:
  - Do NOT modify existing email approval handler logic
  - Do NOT change existing test assertions

  **Recommended Agent Profile**:
  - **Category**: `quick`
    - Reason: Simple RPC method registration following established pattern
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES (with T3.1)
  - **Parallel Group**: Wave 3 (first task, parallel with T3.1)
  - **Blocks**: T3.2
  - **Blocked By**: T0.4 (RPC safety helpers)

  **References**:

  **Pattern References**:
  - `bridge/pkg/rpc/server.go:1314-1315` — Existing email RPC registrations (`email_approval_status`, `email.list_pending`)
  - `bridge/pkg/rpc/email_approval.go` — Email handler pattern

  **WHY Each Reference Matters**:
  - Must follow existing email registration pattern exactly

  **Acceptance Criteria**:

  **QA Scenarios (MANDATORY):**

  ```
  Scenario: Email queue RPC methods registered
    Tool: Bash (go test)
    Preconditions: Code compiles
    Steps:
      1. cd bridge && go test ./pkg/rpc/... -run TestEmailMethodRegistration -count=1 -v
    Expected Result: email.queue_status, email.get, email.retry, email.list all registered
    Failure Indicators: Method not found
    Evidence: .sisyphus/evidence/beato-wave3-t30-test.txt
  ```

  **Commit**: YES (groups with T3.1)
  - Message: `feat(beato): register email queue RPC methods`
  - Files: `bridge/pkg/rpc/server.go`
  - Pre-commit: `cd bridge && go test ./pkg/rpc/... -count=1`

- [x] T3.1. Create Email Outbox Schema + Store

  **What to do**:
  - Create `bridge/pkg/email/outbox.go` with:
    - `OutboxStore` struct backed by SQLite (go-sqlcipher)
    - Schema: `email_outbox` table with columns per user's spec (id, workflow_id, message_id, status, attempt_count, next_attempt_at, last_error_code, recipient_hash, subject_hash, created_at, updated_at)
    - Statuses: queued, awaiting_approval, approved, sending, sent, retry_wait, failed, dead_letter
    - Methods: `Enqueue`, `GetByID`, `UpdateStatus`, `ListByStatus`, `IncrementAttempt`, `MarkDeadLetter`
  - Status transitions must be validated (e.g., can't go from `sent` back to `queued`)

  **Must NOT do**:
  - Do NOT use `mattn/go-sqlite3` — use `mutecomm/go-sqlcipher/v4` ONLY
  - Do NOT implement inbound queue
  - Do NOT store raw email body in queue table
  - Do NOT add queue UI or multi-worker scheduler

  **Recommended Agent Profile**:
  - **Category**: `unspecified-high`
    - Reason: New Go module with SQLite schema and business logic
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: NO
  - **Parallel Group**: Wave 3 (first task)
  - **Blocks**: T3.2
  - **Blocked By**: T2.4

  **References**:

  **Pattern References**:
  - `bridge/pkg/email/email_storage.go` — Existing email storage pattern (follow for SQLite access)
  - `bridge/pkg/security/config_store.go` — ConfigStore pattern for SQLite-backed store with go-sqlcipher

  **API/Type References**:
  - User's specified schema:
    ```sql
    CREATE TABLE IF NOT EXISTS email_outbox (
      id TEXT PRIMARY KEY,
      workflow_id TEXT NOT NULL,
      message_id TEXT,
      status TEXT NOT NULL,
      attempt_count INTEGER NOT NULL DEFAULT 0,
      next_attempt_at INTEGER,
      last_error_code TEXT,
      recipient_hash TEXT NOT NULL,
      subject_hash TEXT,
      created_at INTEGER NOT NULL,
      updated_at INTEGER NOT NULL
    );
    ```

  **WHY Each Reference Matters**:
  - email_storage.go shows the existing pattern for email-related DB access
  - config_store.go shows the go-sqlcipher pattern to follow
  - User's schema is the exact table definition to implement

  **Acceptance Criteria**:

  **QA Scenarios (MANDATORY):**

  ```
  Scenario: Outbox store CRUD operations work
    Tool: Bash (go test)
    Preconditions: Code compiles
    Steps:
      1. cd bridge && go test ./pkg/email/... -run TestOutbox -count=1 -v
    Expected Result: All CRUD tests pass — Enqueue, GetByID, UpdateStatus, ListByStatus, IncrementAttempt, MarkDeadLetter
    Failure Indicators: Any test failure
    Evidence: .sisyphus/evidence/beato-wave3-t31-test.txt
  ```

  **Commit**: YES
  - Message: `feat(beato): add outbound email queue with SQLite backing`
  - Files: `bridge/pkg/email/outbox.go`
  - Pre-commit: `cd bridge && go test ./pkg/email/... -count=1`

- [x] T3.2. Outbox Tests

  **What to do**:
  - Create `bridge/pkg/email/outbox_test.go`
  - Test all CRUD operations
  - Test status transition validation (invalid transitions rejected)
  - Test concurrent access safety
  - Test attempt counter increment and dead_letter marking
  - Test ListByStatus filtering

  **Must NOT do**:
  - Do NOT modify existing test assertions

  **Recommended Agent Profile**:
  - **Category**: `quick`
    - Reason: Test file for existing code
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: NO
  - **Parallel Group**: Wave 3 (after T3.1)
  - **Blocks**: T3.3
  - **Blocked By**: T3.1

  **References**:

  **Test References**:
  - `bridge/pkg/email/email_storage_test.go` — Existing email test patterns

  **Acceptance Criteria**:

  **QA Scenarios (MANDATORY):**

  ```
  Scenario: All outbox tests pass
    Tool: Bash
    Preconditions: T3.1 complete
    Steps:
      1. cd bridge && go test ./pkg/email/... -run TestOutbox -count=1 -v
    Expected Result: All tests pass
    Failure Indicators: Any failure
    Evidence: .sisyphus/evidence/beato-wave3-t32-test.txt
  ```

  **Commit**: YES (groups with T3.1)
  - Message: `feat(beato): add outbound email queue with SQLite backing`
  - Files: `bridge/pkg/email/outbox.go`, `bridge/pkg/email/outbox_test.go`

- [x] T3.3. Wire Outbox into Email Approval Flow

  **What to do**:
  - Modify email approval handlers to persist outbound emails to outbox
  - When `handleRequestEmailApproval` is called, enqueue to outbox with status `awaiting_approval`
  - When `handleApproveEmail` is called, update outbox status to `approved`
  - When email is sent successfully, update status to `sent`
  - On failure, update to `retry_wait` and increment attempt count
  - Apply RPC safety middleware from T0.4 to email RPCs

  **Must NOT do**:
  - Do NOT modify existing `EmailDispatcher` core dispatch logic
  - Do NOT change existing test assertions

  **Recommended Agent Profile**:
  - **Category**: `quick`
    - Reason: Small wiring changes to existing handlers
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: NO
  - **Parallel Group**: Wave 3 (after T3.2)
  - **Blocks**: T3.4
  - **Blocked By**: T0.4 (RPC safety), T3.2 (outbox ready)

  **References**:

  **Pattern References**:
  - `bridge/pkg/rpc/email_approval.go:38-80` — `handleApproveEmail` handler to wire into
  - `bridge/pkg/email/email_storage.go` — Existing email storage integration pattern

  **WHY Each Reference Matters**:
  - Must wire into the exact handler functions that process email approvals

  **Acceptance Criteria**:

  **QA Scenarios (MANDATORY):**

  ```
  Scenario: Email approval flow persists to outbox
    Tool: Bash (go test)
    Preconditions: T3.2 complete, T0.4 complete
    Steps:
      1. cd bridge && go test ./pkg/rpc/... -run TestEmailApprovalOutbox -count=1 -v
    Expected Result: Approval flow creates outbox entry, approval updates status
    Failure Indicators: Outbox not updated on approval
    Evidence: .sisyphus/evidence/beato-wave3-t33-test.txt
  ```

  **Commit**: YES
  - Message: `feat(beato): wire email outbox into approval flow`
  - Files: `bridge/pkg/rpc/email_approval.go`

- [x] T3.4. Email Queue Smoke Test on VPS

  **What to do**:
  - Deploy updated bridge binary to VPS (Docker rebuild + push + pull)
  - Test email queue RPC: request approval → verify outbox entry → approve → verify status update
  - Verify queue persists across bridge restart

  **Must NOT do**:
  - Do NOT modify SQLCipher keystore or Matrix data

  **Recommended Agent Profile**:
  - **Category**: `quick`
    - Reason: VPS deployment + smoke test
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: NO
  - **Parallel Group**: Wave 3 (last task)
  - **Blocks**: T4.1
  - **Blocked By**: T3.3

  **References**: (VPS deployment pattern from T1.2)

  **Acceptance Criteria**:

  **QA Scenarios (MANDATORY):**

  ```
  Scenario: Email queue works on VPS after bridge redeploy
    Tool: Bash (SSH + curl)
    Preconditions: New bridge image pushed, T3.3 complete
    Steps:
      1. Push new bridge Docker image
      2. Pull on VPS, recreate bridge container
      3. Submit email approval request via RPC
      4. Verify outbox entry exists
      5. Approve the email
      6. Verify status updated to "approved" in outbox
    Expected Result: Full email queue lifecycle works on VPS
    Failure Indicators: Any step fails
    Evidence: .sisyphus/evidence/beato-wave3-t34-test.txt
  ```

  **Commit**: NO (VPS deployment)

- [x] T4.1. Audio Capability Audit Report

  **What to do**:
  - Write `tests/reports/audio-capability-audit.md`
  - Audit current audio/voice stack:
    - `bridge/pkg/` — any voice/audio related packages
    - `tests/test-voice-stack.sh` — existing voice test script
    - `tests/docker-compose.voice.yml` — voice Docker config
  - Document what exists, what's stubbed, what's missing
  - Assess STT/TTS/VAD capability gaps
  - Provide prioritized activation recommendations for v1.4

  **Must NOT do**:
  - Do NOT implement any Audio changes (strictly audit only)
  - Do NOT modify any code

  **Recommended Agent Profile**:
  - **Category**: `writing`
    - Reason: Documentation/audit task
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: NO
  - **Parallel Group**: Wave 4 (single task)
  - **Blocks**: T5.1
  - **Blocked By**: T3.4

  **References**:

  **Pattern References**:
  - `tests/test-voice-stack.sh` — Existing voice test script
  - `tests/docker-compose.voice.yml` — Voice Docker compose

  **Acceptance Criteria**:

  **QA Scenarios (MANDATORY):**

  ```
  Scenario: Audio audit report exists and is comprehensive
    Tool: Bash
    Preconditions: T3.4 complete
    Steps:
      1. test -f tests/reports/audio-capability-audit.md && echo "EXISTS" || echo "MISSING"
      2. grep -c "STT\|TTS\|VAD\|activation" tests/reports/audio-capability-audit.md
    Expected Result: Report exists, contains STT/TTS/VAD sections with activation recommendations
    Failure Indicators: File missing or incomplete
    Evidence: .sisyphus/evidence/beato-wave4-t41-test.txt
  ```

  **Commit**: YES
  - Message: `docs(beato): add audio capability audit report`
  - Files: `tests/reports/audio-capability-audit.md`

---

## Final Verification Wave (after ALL implementation tasks)

> These tasks run AFTER all waves complete. ALL must pass.

- [x] T5.1. Full BEATO Pillar Verification

  **What to do**:
  - Test every BEATO pillar on VPS:
    - **Browser**: Create session, navigate, screenshot, close — full lifecycle
    - **Email**: Request approval, verify queue, approve, verify sent status
    - **Audio**: Confirm audit report exists, no active audio processing expected
    - **Text**: Verify existing 14/14 RPC methods still pass (regression check)
    - **Office**: Submit .xlsx, verify extraction, submit corrupt file, verify error handling
  - Record per-pillar PASS/FAIL status
  - Calculate overall BEATO coverage percentage using scoring rubric:

  **BEATO Coverage Scoring Rubric (100 points total):**
  - **Browser (25 pts)**: Jetski deployed (5), no public ports (5), session lifecycle passes (10), external HTTPS works (5)
  - **Office (25 pts)**: Python sidecar deployed (5), document RPC registered (5), extraction works (10), YARA clean (5)
  - **Email (20 pts)**: Outbox store created (5), RPC methods registered (5), approval flow wired (5), VPS smoke passes (5)
  - **Text (20 pts)**: 14/14 RPC regression passes (20)
  - **Audio (10 pts)**: Audit report exists (5), audit reconciles voice-stack docs vs BEATO report (5)
  - **Target: ≥90 points (A-grade)**

  **Must NOT do**:
  - Do NOT modify any code or configuration

  **Recommended Agent Profile**:
  - **Category**: `unspecified-high`
    - Reason: Multi-step integration verification across all pillars
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: NO
  - **Parallel Group**: Wave 5 (first task)
  - **Blocks**: T5.2
  - **Blocked By**: T4.1

  **References**:

  **Pattern References**:
  - `tests/reports/beato-progress-report.md` — Previous BEATO coverage report (baseline: 74%)

  **Acceptance Criteria**:

  **QA Scenarios (MANDATORY):**

  ```
  Scenario: All 5 BEATO pillars verified on VPS
    Tool: Bash (SSH + curl)
    Preconditions: All waves complete
    Steps:
      1. Browser: Create session → navigate → screenshot → close
      2. Email: Request approval → verify queue → approve → verify status
      3. Audio: Check audit report exists
      4. Text: Run 14 RPC regression tests
      5. Office: Submit .xlsx → verify extraction
    Expected Result: Browser=PASS, Email=PASS, Audio=AUDIT_ONLY, Text=PASS, Office=PASS → Overall ≥90%
    Failure Indicators: Any pillar FAIL
    Evidence: .sisyphus/evidence/beato-wave5-t51-test.txt
  ```

  **Commit**: NO (verification only)

- [x] T5.2. Resource Under-Load Verification

  **What to do**:
  - Run a combined load scenario: browser session + email approval + document extraction simultaneously
  - Record VPS resources during active load
  - Verify: no OOM kills, available RAM ≥ 250 MB during load, load average < 3.0
  - Record Docker stats for each container during load

  **Must NOT do**:
  - Do NOT stress test beyond normal operation
  - Do NOT modify any configuration

  **Recommended Agent Profile**:
  - **Category**: `quick`
    - Reason: Single resource monitoring command during load
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: NO
  - **Parallel Group**: Wave 5 (after T5.1)
  - **Blocks**: T5.3
  - **Blocked By**: T5.1

  **References**: (VPS: `5.183.11.149`)

  **Acceptance Criteria**:

  **QA Scenarios (MANDATORY):**

  ```
  Scenario: VPS stable under combined BEATO load
    Tool: Bash (SSH)
    Preconditions: T5.1 complete
    Steps:
      1. Start a browser session (navigate to example.com)
      2. Submit an email approval request
      3. Submit a document for extraction
      4. While all active: ssh root@5.183.11.149 'free -m; docker stats --no-stream; uptime'
      5. Check dmesg for OOM kills
    Expected Result: Available RAM ≥ 250MB, no OOM kills, load < 3.0
    Failure Indicators: OOM kill detected, RAM < 250MB
    Evidence: .sisyphus/evidence/beato-wave5-t52-test.txt
  ```

  **Commit**: NO (verification only)

- [x] T5.3. Rollback Drill

  **What to do**:
  - Execute the rollback procedure:
    1. Stop Jetski: `docker stop armorclaw-jetski && docker rm armorclaw-jetski`
    2. Stop Python sidecar: `docker stop armorclaw-sidecar-office && docker rm armorclaw-sidecar-office`
    3. Remove BEATO compose overlay from deploy command
    4. Verify bridge health: `curl --unix-socket /run/armorclaw/bridge.sock http://localhost/health`
    5. Verify 14/14 RPC methods still work without sidecars
  - After drill: redeploy everything (re-run T1.2, T2.1)

  **Must NOT do**:
  - Do NOT touch SQLCipher, Matrix data, or Secretary state during rollback
  - Do NOT modify bridge container

  **Recommended Agent Profile**:
  - **Category**: `quick`
    - Reason: Sequential rollback commands
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: NO
  - **Parallel Group**: Wave 5 (after T5.2)
  - **Blocks**: T5.4
  - **Blocked By**: T5.2

  **References**:

  **Pattern References**:
  - T0.3 rollback snapshot — compare against pre-deployment state

  **Acceptance Criteria**:

  **QA Scenarios (MANDATORY):**

  ```
  Scenario: Rollback restores bridge to pre-BEATO state
    Tool: Bash (SSH)
    Preconditions: T5.2 complete
    Steps:
      1. ssh -i ~/.ssh/openclaw_win root@5.183.11.149 'docker stop armorclaw-jetski armorclaw-sidecar-office || true'
      2. ssh -i ~/.ssh/openclaw_win root@5.183.11.149 'docker rm armorclaw-jetski armorclaw-sidecar-office || true'
      3. curl --unix-socket /run/armorclaw/bridge.sock http://localhost/health
      4. Run 14 RPC regression tests
    Expected Result: Bridge healthy, all 14 RPC methods pass without sidecars
    Failure Indicators: Bridge health fails, any RPC regression
    Evidence: .sisyphus/evidence/beato-wave5-t53-test.txt
              .sisyphus/evidence/beato-wave5-t53-rollback.txt
  ```

  **Commit**: NO (drill + redeploy)

- [x] T5.4. BEATO Verification Report

  **What to do**:
  - Write `tests/reports/beato-verification-report.md`
  - Include: per-pillar status, coverage percentage, resource utilization, rollback drill results
  - Compare against baseline (74% from beato-progress-report.md)
  - Document: what changed, what's deployed, what's deferred
  - Include: VPS resource summary, container inventory, network topology

  **Must NOT do**:
  - Do NOT falsify any test results

  **Recommended Agent Profile**:
  - **Category**: `writing`
    - Reason: Documentation task
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: NO
  - **Parallel Group**: Wave 5 (last task)
  - **Blocks**: None
  - **Blocked By**: T5.3

  **References**:

  **Pattern References**:
  - `tests/reports/beato-progress-report.md` — Previous report (baseline 74%)
  - `tests/reports/pre-beato-complete-report.md` — Pre-BEATO calibration (84% readiness)

  **Acceptance Criteria**:

  **QA Scenarios (MANDATORY):**

  ```
  Scenario: Final BEATO verification report exists with ≥90% coverage
    Tool: Bash
    Preconditions: T5.3 complete
    Steps:
      1. test -f tests/reports/beato-verification-report.md && echo "EXISTS" || echo "MISSING"
      2. grep -o '[0-9]*%' tests/reports/beato-verification-report.md | head -1
    Expected Result: Report exists, overall coverage ≥ 90%
    Failure Indicators: Report missing or coverage < 90%
    Evidence: .sisyphus/evidence/beato-wave5-t54-test.txt
  ```

  **Commit**: YES
  - Message: `docs(beato): final BEATO verification report`
  - Files: `tests/reports/beato-verification-report.md`

---

## Commit Strategy

- **W0**: `feat(beato): add RPC safety middleware` — rpc_safety.go, rpc_safety_test.go
- **W1**: `feat(beato): deploy Jetski via Docker Compose overlay` — docker-compose.beato.yml, beato.env.example
- **W2**: `fix(beato): resolve YARA unreferenced $mz warning + deploy Python sidecar` — yara_rules.yar
- **W3**: `feat(beato): add outbound email queue` — outbox.go, outbox_test.go
- **W4**: `docs(beato): audio capability audit report`
- **W5**: `docs(beato): final BEATO verification report`

---

## Success Criteria

### Verification Commands
```bash
# Jetski running, no public ports
ssh -i ~/.ssh/openclaw_win root@5.183.11.149 'docker ps --filter name=armorclaw-jetski --format "{{.Names}} {{.Status}}"'
# Expected: armorclaw-jetski Up ...

# No public port mapping for Jetski
ssh -i ~/.ssh/openclaw_win root@5.183.11.149 'docker inspect armorclaw-jetski --format "{{json .HostConfig.PortBindings}}"'
# Expected: null or empty (expose only, no ports)

# Python sidecar running
ssh -i ~/.ssh/openclaw_win root@5.183.11.149 'docker ps --filter name=armorclaw-sidecar-office --format "{{.Names}} {{.Status}}"'

# YARA rules compile clean
cd bridge && go test ./pkg/yara/... -count=1
# Expected: PASS, 0 failures

# Email outbox tests pass
cd bridge && go test ./pkg/email/... -run TestOutbox -count=1
# Expected: PASS

# RPC safety tests pass
cd bridge && go test ./pkg/rpc/... -run TestRPCSafety -count=1
# Expected: PASS

# VPS has sufficient free memory
ssh -i ~/.ssh/openclaw_win root@5.183.11.149 'free -m | grep Mem'
# Expected: available ≥ 250 MB

# No OOM kills
ssh -i ~/.ssh/openclaw_win root@5.183.11.149 'dmesg | grep -i "oom-kill" | tail -5'
# Expected: empty or no recent kills
```

### Final Checklist
- [x] All "Must Have" present
- [x] All "Must NOT Have" absent
- [x] All go tests pass
- [x] Jetski container healthy, no public ports
- [x] Python sidecar container healthy
- [x] YARA rules compile without warnings
- [x] Email outbox CRUD works
- [x] RPC safety middleware applied
- [x] VPS stable under load (no OOM)
- [x] BEATO coverage ≥90%
- [x] Rollback drill successful
