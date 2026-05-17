# Full System Test Harness — Complete Rewrite

## TL;DR

> **Quick Summary**: Build a comprehensive ArmorChat-independent test harness covering 13 major ArmorClaw subsystems, organized in dependency order with shared fixtures, two execution tiers (VPS-deployed vs code-only), and the same rigorous verification philosophy as harness-hardening.
>
> **Deliverables**:
> - `tests/lib/` shared fixture library (6 files) extending existing `tests/e2e/common.sh`
> - 13 subsystem test scripts in `tests/`
> - 4 cross-subsystem integration scenarios
> - Tier A (VPS-real): 5 scripts with live integration testing
> - Tier B (code-only): 8 scripts with API contract tests + graceful skip on VPS
>
> **Estimated Effort**: XL (13 test scripts + 4 scenarios + shared fixtures)
> **Parallel Execution**: YES — 5 waves + cross-subsystem + final verification
> **Critical Path**: T0 (fixtures) → T1/T2/T2.5 (foundations) → T3a/T4 (orchestration) → T3b/T5-T8 (high-risk) → T9-T11 (medium) → X1-X4 → F1-F4

---

## Context

### Original Request
Build a comprehensive test program expanding beyond the governance + Matrix transport harness (already complete) to validate all major ArmorClaw subsystems in dependency order. Keep the two-plane philosophy (Bridge admin + Execution/communication). Fix the biggest weakness: foundational subsystems must be tested before orchestration-heavy subsystems that depend on them.

### Interview Summary
**Key Discussions**:
- VPS deployment status: Only bridge + Matrix running — voice, jetski, sidecars, license server NOT deployed
- Undeployed subsystems: Skip gracefully (same pattern as harness-hardening socat skip)
- WebSocket client: websocat for T1 event streaming
- T0 polish: Include README.md + JSON health output
- Two-tier model: Tier A (VPS-deployed, real tests) and Tier B (code-only, graceful skip on VPS)

**Research Findings**:
- 90 registered JSON-RPC methods + 11 HTTP endpoints + 6 License REST + 7 Sidecar gRPC = 114 total API endpoints
- EventBus: 29 event types, WebSocket at /ws, subscriber filters, broadcaster interface
- Trust: RiskClassifier with OWASP taxonomy, PII Scrubber (16 patterns), Broker (fail-closed), AuditLog (65+ event types)
- Secretary: 11 RPC methods, state machine (6 states: pending/running/blocked/completed/failed/cancelled), blocker model (3 retries)
- Email: 4 approval RPCs, Unix socket ingest, outbound PII gate (300s timeout)
- Sidecar: 3-tier routing (Go/Rust/Python), gRPC health checks
- Voice: BudgetTracker, STT/TTS/VAD — NOT on VPS
- Jetski: CDP proxy + RPC on 9223 — NOT on VPS
- License: REST API + bridge client + state machine (5 states) — NOT on VPS
- Platform adapters: Matrix (primary) + Slack, static registration
- Existing: `tests/e2e/common.sh` has 281 lines of battle-tested utilities — must extend, not duplicate

### Metis Review
**Identified Gaps** (addressed):
- RPC count corrected: 90 JSON-RPC (not 124) — incorporated
- `tests/e2e/common.sh` must be sourced, not reimplemented — incorporated into T0
- Tier A/B partitioning for VPS vs code-only — incorporated
- Bridge restart contention: must serialize restart tests via flock — incorporated
- websocat availability check with graceful skip — incorporated
- Secretary tests must create/cleanup own templates — incorporated
- EventBus WebSocket tests must NOT trigger crash path (log.Fatalf) — guardrail added
- Cross-subsystem scenarios need explicit execution environment — incorporated
- License server is separate binary — T8 tests bridge client RPCs only

---

## Work Objectives

### Core Objective
Prove that the remaining major ArmorClaw subsystems work on real infrastructure, with shared fixtures enabling consistent testing patterns across all scripts.

### Concrete Deliverables
- `tests/lib/load_env.sh` — Environment loading (extends common.sh)
- `tests/lib/assert_json.sh` — JSON assertion helpers
- `tests/lib/restart_bridge.sh` — Bridge restart with flock serialization
- `tests/lib/event_subscriber_helper.sh` — WebSocket event subscription via websocat
- `tests/lib/common_output.sh` — Standardized PASS/FAIL/SKIP output
- `tests/lib/README.md` — Usage documentation for test helpers
- `tests/test-eventbus-streaming.sh` — T1: EventBus + WebSocket live events
- `tests/test-trust-layer.sh` — T2: Security/trust layer
- `tests/test-system-health-baseline.sh` — T2.5: Full startup health
- `tests/test-secretary-workflow-core.sh` — T3a: Core workflow execution
- `tests/test-secretary-workflow-deep.sh` — T3b: Deep workflow validation
- `tests/test-email-pipeline.sh` — T4: Email approval pipeline
- `tests/test-sidecar-docs.sh` — T5: Document sidecar pipeline
- `tests/test-voice-stack.sh` — T6: Voice stack (Tier B)
- `tests/test-jetski-sidecar.sh` — T7: Browser sidecar (Tier B)
- `tests/test-license-enforcement.sh` — T8: License enforcement (Tier B)
- `tests/test-platform-adapters.sh` — T9: Platform adapters (Tier B)
- `tests/test-agent-runtime.sh` — T10: Agent runtime invariants (Tier B)
- `tests/test-deployment-usb.sh` — T11: Deployment/USB (Tier B)

### Definition of Done
- [ ] All shared fixtures in tests/lib/ pass `bash -n` and are sourced by ≥ 3 test scripts
- [ ] Tier A scripts (T1, T2, T2.5, T3a, T4) pass on VPS with live bridge
- [ ] Tier B scripts (T5-T11) either pass live or skip gracefully with clear reason
- [ ] Cross-subsystem scenarios (X1-X4) pass or skip with documented reason
- [ ] All scripts source `.env`, use strict mode, emit PASS/FAIL/SKIP counts
- [ ] Full harness runs in ≤ 10 minutes wall-clock
- [ ] F1-F4 all pass with fresh evidence

### Tier Classification

**Tier A — VPS-Deployed (live integration tests)**:
- T1: EventBus (bridge has WebSocket wiring, needs `websocket_enabled=true`)
- T2: Trust Layer (Broker/PII/Scrubber are bridge-internal, testable via RPC)
- T2.5: System Health (bridge health endpoints already proven)
- T3a: Secretary Workflow Core (secretary RPCs on bridge, creates own templates)
- T4: Email Pipeline (email approval RPCs on bridge)

**Tier B — Code-Only (API contract tests, skip on VPS if undeployed)**:
- T3b: Secretary Workflow Deep (may need container runtime)
- T5: Sidecar/Docs (needs Rust+Python sidecars running)
- T6: Voice Stack (NOT deployed on VPS)
- T7: Jetski Browser Sidecar (NOT deployed on VPS)
- T8: License Enforcement (license server NOT deployed; tests bridge client RPCs)
- T9: Platform Adapters (Slack adapter NOT configured)
- T10: Agent Runtime (needs Docker + agent container)
- T11: Deployment/USB (needs physical USB device)

### Must Have
- Shared fixtures sourced from all test scripts (no reimplemented utilities)
- Tier A/B classification per script
- Bridge restart serialization via flock (prevent concurrent restarts)
- websocat availability check with graceful skip for T1
- Secretary tests create and cleanup own templates
- Every test: success path + blocked/refused path + restart path where applicable
- Evidence saved to `.sisyphus/evidence/full-system-{task-name}/`
- Max wall-clock: 30s for non-restart Tier A, 60s for restart, 10s for Tier B skip
- All scripts source tests/e2e/common.sh (extend, don't duplicate)
- JSON health output from T2.5 for F3 automation

### Must NOT Have (Guardrails)
- NO modifications to bridge source code, config, Go/Rust/Python/TypeScript files
- NO new Go/Rust/Python test files — those belong in v080-comprehensive-tests.md
- NO reimplemented utilities from tests/e2e/common.sh — extend only
- NO EventBus WebSocket tests that trigger crash path (log.Fatalf)
- NO parallel bridge restarts — serialize with flock
- NO assuming pre-existing secretary templates or email data
- NO interactive prompts in any script
- NO hardcoded credentials — all from .env
- NO test timeout exceeding 60s per individual script
- NO touching production bridge config permanently
- NO overlap with v080-comprehensive-tests.md scope (Go/Rust unit tests)

---

## Verification Strategy (MANDATORY)

> **ZERO HUMAN INTERVENTION** — ALL verification is agent-executed. No exceptions.

### Test Decision
- **Infrastructure exists**: YES — tests/e2e/common.sh (281 lines), existing harness scripts
- **Automated tests**: The scripts themselves are the tests
- **Framework**: Bash + curl + jq + socat + websocat (optional) + flock
- **Evidence path**: `.sisyphus/evidence/full-system-{task-name}/`

### QA Policy
Every task includes agent-executed QA scenarios.
Evidence saved to `.sisyphus/evidence/full-system-{task-name}/`.

- **Tier A scripts**: Run on VPS via SSH, assert exit code + output content
- **Tier B scripts**: Run structure check (`bash -n`, grep for patterns), graceful skip on VPS
- **Cross-subsystem**: Run after all subsystem tests pass
- **Bridge restart**: Serialized via flock, verified with `systemctl is-active`

---

## Execution Strategy

### Parallel Execution Waves

```
Wave 0 (Start Immediately — shared foundation):
└── T0: Shared fixtures + helper library [unspecified-high]

Wave A (After T0 — foundational subsystems, MAX PARALLEL):
├── T1: EventBus + WebSocket live event harness [unspecified-high]
├── T2: Security / Trust Layer harness [unspecified-high]
└── T2.5: System health after full startup [quick]

Wave B (After Wave A — core orchestration, PARALLEL):
├── T3a: Secretary Workflow core harness [unspecified-high]
└── T4: Email Pipeline harness [unspecified-high]

Wave C (After Wave B — high-risk subsystems, MAX PARALLEL):
├── T3b: Secretary Workflow deep validation [deep]
├── T5: Sidecar / Document pipeline [unspecified-high]
├── T6: Voice Stack [unspecified-high]
├── T7: Jetski Browser Sidecar [unspecified-high]
└── T8: License Enforcement [unspecified-high]

Wave D (After Wave C — medium priority, MAX PARALLEL):
├── T9: Platform Adapters [unspecified-high]
├── T10: Agent Runtime invariants [deep]
└── T11: Deployment/USB [quick]

Wave E (After Wave D — cross-subsystem scenarios, SEQUENTIAL):
├── X1: Workflow -> Email Approval [unspecified-high]
├── X2: Workflow -> Document Sidecar [unspecified-high]
├── X3: Browser Action -> Trust Block [unspecified-high]
└── X4: Event Stream Truth [unspecified-high]

Wave FINAL (After ALL tasks — 4 parallel reviews):
├── F1: Plan compliance audit [oracle]
├── F2: Code quality review [unspecified-high]
├── F3: Full automated execution audit [unspecified-high]
└── F4: Scope fidelity check [deep]
→ Present results → Get explicit user okay

Critical Path: T0 → T1/T2/T2.5 → T3a/T4 → T3b → X1-X4 → F1-F4
Parallel Speedup: ~65% faster than sequential
Max Concurrent: 5 (Wave C)
```

### Dependency Matrix

| Task | Depends On | Blocks | Wave | Tier |
|------|-----------|--------|------|------|
| T0 | — | ALL | 0 | — |
| T1 | T0 | X4 | A | A |
| T2 | T0 | X3 | A | A |
| T2.5 | T0 | F3 | A | A |
| T3a | T0, T1, T2 | T3b, X1 | B | A |
| T4 | T0, T2 | X1 | B | A |
| T3b | T3a | X1, X2 | C | B |
| T5 | T0 | X2 | C | B |
| T6 | T0 | — | C | B |
| T7 | T0 | X3 | C | B |
| T8 | T0 | — | C | B |
| T9 | T0 | — | D | B |
| T10 | T0 | — | D | B |
| T11 | T0 | — | D | B |
| X1 | T3a, T4 | F1-F4 | E | — |
| X2 | T3b, T5 | F1-F4 | E | — |
| X3 | T2, T7 | F1-F4 | E | — |
| X4 | T1 | F1-F4 | E | — |
| F1-F4 | ALL | user okay | FINAL | — |

### Agent Dispatch Summary

- **Wave 0**: 1 — T0 → `unspecified-high`
- **Wave A**: 3 — T1 → `unspecified-high`, T2 → `unspecified-high`, T2.5 → `quick`
- **Wave B**: 2 — T3a → `unspecified-high`, T4 → `unspecified-high`
- **Wave C**: 5 — T3b → `deep`, T5 → `unspecified-high`, T6 → `unspecified-high`, T7 → `unspecified-high`, T8 → `unspecified-high`
- **Wave D**: 3 — T9 → `unspecified-high`, T10 → `deep`, T11 → `quick`
- **Wave E**: 4 — X1-X4 → `unspecified-high`
- **FINAL**: 4 — F1 → `oracle`, F2-F3 → `unspecified-high`, F4 → `deep`

---

## TODOs

- [x] T0. Shared Fixtures + Helper Library

  **What to do**:
  - Create `tests/lib/` directory with 6 files extending (not duplicating) `tests/e2e/common.sh`
  - `load_env.sh`: Source `.env`, export VPS_IP/VPS_USER/ADMIN_TOKEN/BRIDGE_PORT/MATRIX_PORT/SSH_KEY_PATH. Source `tests/e2e/common.sh` for existing `rpc_call()` etc.
  - `assert_json.sh`: Helpers like `assert_json_has_key()`, `assert_json_equals()`, `assert_json_contains()`, `assert_json_not_contains()`, `assert_rpc_success()`, `assert_rpc_error()`
  - `restart_bridge.sh`: Bridge restart with flock serialization (`flock /tmp/armorclaw-test-restart.lock`), poll for readiness (max 30s), verify both socket and HTTP up
  - `event_subscriber_helper.sh`: Check websocat availability, subscribe to WebSocket events, capture N events with timeout, parse event types via jq
  - `common_output.sh`: Standardized PASS/FAIL/SKIP counters, `[INFO]`/`[PASS]`/`[FAIL]`/`[SKIP]` output functions, summary function
  - `README.md`: One-paragraph usage guide per helper + sourcing example
  - Create `tests/fixtures/` directory (empty, for future shared test data)

  **Must NOT do**:
  - Do NOT reimplement functions from tests/e2e/common.sh (rpc_call, log_result, test_summary) — source them
  - Do NOT hardcode credentials — all from .env
  - Do NOT add interactive prompts

  **Recommended Agent Profile**:
  - **Category**: `unspecified-high`
  - **Skills**: []

  **Parallelization**: Wave 0 (all others depend on this)
  - **Blocks**: T1-T11, X1-X4, F1-F4
  - **Blocked By**: None (start immediately)

  **References**:
  - `tests/e2e/common.sh` — Existing 281-line utility library with rpc_call(), start_bridge(), wait_for_bridge(), log_result(), test_summary(). MUST source this, not reimplement.
  - `tests/test-vps-smoke.sh:15-17` — .env sourcing pattern (set -a; source .env 2>/dev/null || true; set +a)
  - `tests/test-persistence.sh:201-220` — Bridge readiness polling pattern (15×2s intervals)
  - `tests/adversarial/test_confused_deputy.sh` — pass()/skip()/fail() helper pattern

  **Acceptance Criteria**:
  - [ ] All 6 files pass `bash -n` syntax check
  - [ ] `tests/lib/README.md` exists with ≥ 1 paragraph per helper
  - [ ] `tests/fixtures/` directory exists
  - [ ] `grep -c 'source.*common.sh\|source.*e2e/common' tests/lib/load_env.sh` ≥ 1 (sources existing utilities)
  - [ ] `grep -c 'flock' tests/lib/restart_bridge.sh` ≥ 1 (restart serialization)
  - [ ] `grep -c 'websocat' tests/lib/event_subscriber_helper.sh` ≥ 1 (WebSocket client)

  **QA Scenarios**:
  ```
  Scenario: All fixture files are syntactically valid
    Tool: Bash (bash -n)
    Steps:
      1. for f in tests/lib/*.sh; do bash -n "$f" || exit 1; done
    Expected Result: All files pass syntax check
    Evidence: .sisyphus/evidence/full-system-t0/syntax-check.txt

  Scenario: Fixtures source common.sh (no duplication)
    Tool: Bash (grep)
    Steps:
      1. grep 'source.*common.sh\|source.*e2e/common' tests/lib/load_env.sh
      2. grep -c 'rpc_call\|log_result\|test_summary' tests/lib/*.sh
    Expected Result: load_env.sh sources common.sh; no reimplemented functions
    Evidence: .sisyphus/evidence/full-system-t0/no-duplication.txt

  Scenario: Restart helper uses flock
    Tool: Bash (grep)
    Steps:
      1. grep 'flock' tests/lib/restart_bridge.sh
    Expected Result: flock present for serialization
    Evidence: .sisyphus/evidence/full-system-t0/flock-check.txt
  ```

  **Commit**: YES
  - Message: `feat(tests): add shared test fixtures and helper library`
  - Files: `tests/lib/*`, `tests/fixtures/.gitkeep`

- [x] T1. EventBus + WebSocket Live Event Harness

  **What to do**:
  - Create `tests/test-eventbus-streaming.sh` that validates EventBus publish/subscribe over WebSocket
  - **Tier A**: Runs on VPS if `websocket_enabled=true` in config (currently false — test will skip gracefully until enabled)
  - Test sequence:
    1. **E0: Prerequisites** — Source fixtures, check bridge running, check websocat available (skip if not), check WebSocket enabled (skip if not)
    2. **E1: WebSocket connection** — Connect via websocat to `wss://$VPS_IP:$BRIDGE_PORT/ws`, verify heartbeat received within 10s
    3. **E2: Event subscription** — Send subscribe message, verify confirmation
    4. **E3: Event fanout** — Connect 2 subscribers, trigger event via RPC (e.g., `device.list`), verify both receive event
    5. **E4: Event filtering** — Subscribe with event_type filter, verify only matching events received
    6. **E5: No-subscriber behavior** — Unsubscribe all, emit event, verify no errors
    7. **E6: Reconnect** — Disconnect subscriber, reconnect, verify resume (if applicable)
  - Print summary with event counts per type, subscriber counts, pass/fail/skip
  - Save evidence to `.sisyphus/evidence/full-system-t1/`

  **Must NOT do**:
  - Do NOT trigger crash path — do NOT test WebSocket initialization failure (log.Fatalf per review.md:42)
  - Do NOT modify bridge config — test reads current config, skips if WS not enabled
  - Do NOT test with > 10 concurrent subscribers (MaxSubscribers=100, keep test reasonable)

  **Recommended Agent Profile**:
  - **Category**: `unspecified-high`
  - **Skills**: []

  **Parallelization**: Wave A (with T2, T2.5)
  - **Blocks**: X4
  - **Blocked By**: T0

  **References**:
  - `bridge/pkg/eventbus/eventbus.go` — EventBus: Publish, Subscribe, Unsubscribe, subscriber cleanup
  - `bridge/pkg/eventbus/events.go` — 29 event type constants, event structs
  - `bridge/pkg/http/server.go:697-1135` — WebSocket handler: /ws endpoint, message format, broadcast methods
  - `bridge/pkg/websocket/websocket.go` — EventBroadcaster interface
  - `bridge/pkg/http/server.go:1124` — BroadcastEvent() implementation
  - Message format: Inbound `{"type":"ping"}` or `{"type":"register","payload":{"device_id":"..."}}`, Outbound `{"type":"event","event":{...},"received":"...","sequence":123}` or `{"type":"heartbeat","timestamp":"..."}`
  - Config: `websocket_enabled=true` required (currently false on VPS), `WebSocketAddr: "0.0.0.0:8444"`, `MaxSubscribers: 100`

  **Acceptance Criteria**:
  - [ ] Script passes `bash -n`
  - [ ] Script sources tests/lib/ fixtures
  - [ ] Graceful skip when websocat unavailable or websocket_enabled=false
  - [ ] Clear PASS/FAIL/SKIP summary with event counts

  **QA Scenarios**:
  ```
  Scenario: Script structure valid
    Tool: Bash (bash -n + grep)
    Steps:
      1. bash -n tests/test-eventbus-streaming.sh
      2. grep 'source.*tests/lib' tests/test-eventbus-streaming.sh
      3. grep 'websocat' tests/test-eventbus-streaming.sh
      4. grep 'websocket_enabled\|SKIP' tests/test-eventbus-streaming.sh
    Expected Result: Syntax OK, sources fixtures, checks websocat, has skip logic
    Evidence: .sisyphus/evidence/full-system-t1/structure-check.txt

  Scenario: Graceful skip when WS disabled
    Tool: Bash (script execution on VPS)
    Steps:
      1. bash tests/test-eventbus-streaming.sh
      2. Assert exit code 0 and output contains "[SKIP]"
    Expected Result: Script exits 0 with skip message (WS currently disabled on VPS)
    Evidence: .sisyphus/evidence/full-system-t1/skip-output.txt
  ```

  **Commit**: YES
  - Message: `test(eventbus): add EventBus + WebSocket live event streaming harness`
  - Files: `tests/test-eventbus-streaming.sh`

- [x] T2. Security / Trust Layer Harness

  **What to do**:
  - Create `tests/test-trust-layer.sh` that validates trust gates actually block, sanitize, and signal correctly
  - **Tier A**: Tests broker behavior via RPC calls to bridge on VPS
  - Test sequence:
    1. **S0: Prerequisites** — Source fixtures, check bridge running
    2. **S1: PII detection** — Send `pii.request` with sensitive fields (credit_card, ssn), verify scrubbing in response
    3. **S2: PII approval flow** — `pii.request` → verify status "pending" → `pii.approve` → verify "approved" → `pii.fulfill` → verify placeholder resolution
    4. **S3: PII denial** — `pii.request` → `pii.deny` → verify "denied" + deny_reason
    5. **S4: Risk classification** — Test actions that should be ALLOW (browser.navigate), DEFER (email.send, secret.access), DENY (unknown action)
    6. **S5: Secret approval policy** — Verify payment secrets → DENY, generic API keys → ALLOW
    7. **S6: False-positive control** — Send non-PII data, verify NOT flagged
    8. **S7: Audit trail** — Verify audit events generated for PII access (via `events.replay` or `events.stream`)
  - Print summary: blocked vs allowed counts, sanitized payload evidence

  **Must NOT do**:
  - Do NOT modify security policy or PII patterns — test existing behavior only
  - Do NOT test with real PII — use obviously fake test data (test SSN: 000-00-0000)

  **Recommended Agent Profile**:
  - **Category**: `unspecified-high`
  - **Skills**: []

  **Parallelization**: Wave A (with T1, T2.5)
  - **Blocks**: X3, T3a, T4
  - **Blocked By**: T0

  **References**:
  - `bridge/pkg/capability/broker.go` — Broker.Authorize pipeline: validate → registry → classify → consent → scrub
  - `bridge/pkg/capability/types.go` — ActionRequest, ActionResponse, RiskLevel (ALLOW/DENY/DEFER), RiskClass (6 classes)
  - `bridge/pkg/capability/risk_classifier.go` — RiskClassifierImpl with OWASP taxonomy: browser.browse→ALLOW, email.send→DEFER, unknown→DENY
  - `bridge/pkg/capability/secret_approval.go` — SecretApprovalPolicy: payment→DENY, generic→ALLOW
  - `bridge/pkg/pii/scrubber.go` — 16 PII patterns: credit_card, phone, ssn, bearer_token, github_token, aws_key_id, api_key_*, email, ip_address, password, token, secret
  - `bridge/pkg/pii/masker.go` — MaskPII: {{VAULT:type_N}} placeholder format
  - `bridge/pkg/rpc/pii.go` — RPC handlers: pii.request, pii.approve, pii.deny, pii.status, pii.list_pending, pii.stats, pii.cancel, pii.fulfill, pii.wait_for_approval
  - `bridge/pkg/audit/audit.go` — AuditLog: 65+ event types, persistent at /var/lib/armorclaw/audit.db

  **Acceptance Criteria**:
  - [ ] Script passes `bash -n`
  - [ ] Sources tests/lib/ fixtures
  - [ ] Tests PII request/approve/deny/fulfill lifecycle
  - [ ] Tests risk classification for ALLOW/DEFER/DENY actions
  - [ ] Clear PASS/FAIL summary with blocked/allowed counts

  **QA Scenarios**:
  ```
  Scenario: Trust layer script structure
    Tool: Bash (grep)
    Steps:
      1. bash -n tests/test-trust-layer.sh
      2. grep 'pii.request\|pii.approve\|pii.deny' tests/test-trust-layer.sh
      3. grep 'ALLOW\|DENY\|DEFER' tests/test-trust-layer.sh
    Expected Result: PII lifecycle methods + risk levels present
    Evidence: .sisyphus/evidence/full-system-t2/structure-check.txt

  Scenario: PII request/approve lifecycle works on VPS
    Tool: Bash (RPC via SSH)
    Steps:
      1. bash tests/test-trust-layer.sh 2>&1 | tee evidence
      2. Assert exit code 0 and ≥ 3 PASS markers
    Expected Result: PII detection, approval, denial all pass
    Evidence: .sisyphus/evidence/full-system-t2/pii-lifecycle.txt
  ```

  **Commit**: YES
  - Message: `test(trust): add security/trust layer harness`
  - Files: `tests/test-trust-layer.sh`

- [x] T2.5. System Health After Full Startup

  **What to do**:
  - Create `tests/test-system-health-baseline.sh` that verifies all configured components report healthy after startup
  - **Tier A**: Runs on VPS, tests live health endpoints
  - Test sequence:
    1. **H0: Prerequisites** — Source fixtures, check bridge running
    2. **H1: Bridge health** — `GET /health` → assert `status: "ok"` (or "healthy")
    3. **H2: Bridge status** — `GET /api/status` → assert `status: "running"`
    4. **H3: Discovery** — `GET /api/discovery` → assert version and endpoints present
    5. **H4: Matrix health** — `health.check` RPC → assert matrix component healthy (or connected)
    6. **H5: Keystore** — `store_key` RPC with test key → assert success
    7. **H6: Component table** — Build and output a machine-readable JSON summary: `{"components":{"bridge":"ok","matrix":"connected","keystore":"ok","eventbus":"ready"},"timestamp":"...","overall":"healthy"}`
  - Save JSON health summary to `.sisyphus/evidence/full-system-t2.5/health-summary.json`

  **Must NOT do**:
  - Do NOT restart the bridge (it's a read-only health check)
  - Do NOT modify any state

  **Recommended Agent Profile**:
  - **Category**: `quick`
  - **Skills**: []

  **Parallelization**: Wave A (with T1, T2)
  - **Blocks**: F3 (baseline for full harness audit)
  - **Blocked By**: T0

  **References**:
  - `bridge/pkg/http/server.go` — `/health`, `/api/status`, `/api/discovery` endpoints
  - `bridge/pkg/rpc/server.go` — `health.check` RPC method → HealthCheckResponse
  - `bridge/pkg/rpc/public_handlers.go` — `system.health` (public, no auth)
  - `tests/test-vps-smoke.sh` — Existing health check patterns (Category A)

  **Acceptance Criteria**:
  - [ ] Script passes `bash -n`
  - [ ] Outputs JSON health summary to evidence
  - [ ] Tests ≥ 4 different health endpoints/methods
  - [ ] Clear PASS/FAIL component table

  **QA Scenarios**:
  ```
  Scenario: Health baseline passes on VPS
    Tool: Bash (script execution)
    Steps:
      1. bash tests/test-system-health-baseline.sh
      2. Assert exit code 0
      3. cat .sisyphus/evidence/full-system-t2.5/health-summary.json | jq '.overall'
    Expected Result: All components healthy, JSON summary with overall="healthy"
    Evidence: .sisyphus/evidence/full-system-t2.5/health-summary.json
  ```

  **Commit**: YES
  - Message: `test(health): add system health baseline harness`
  - Files: `tests/test-system-health-baseline.sh`

- [x] T3a. Secretary Workflow Core Harness

  **What to do**:
  - Create `tests/test-secretary-workflow-core.sh` that validates real workflow execution, blockers, and restart survival
  - **Tier A**: Tests secretary RPCs on live VPS bridge — creates and cleans up own templates
  - Test sequence:
    1. **W0: Prerequisites** — Source fixtures, check bridge, verify secretary RPCs available (`secretary.is_running`)
    2. **W1: Template lifecycle** — `secretary.create_template` → `secretary.get_template` → `secretary.update_template` → `secretary.list_templates` → `secretary.delete_template` (cleanup)
    3. **W2: Single-step workflow** — Create template with 1 step → `secretary.start_workflow` → `secretary.get_workflow` → verify status transitions → `secretary.cancel_workflow`
    4. **W3: Multi-step workflow** — Create template with 3 steps → start → advance → verify progress
    5. **W4: Blocker creation/resolution** — Create workflow → simulate blocker → `resolve_blocker` → verify unblock
    6. **W5: Restart survival** — Create template + start workflow → restart bridge → `secretary.get_workflow` → verify state persisted
    7. **W6: Negative paths** — Timeout without resolution, duplicate resolution, malformed input
    8. **Cleanup** — Delete all test templates
  - Use unique IDs per run: `TEST_TEMPLATE="harness-t3a-$(date +%s)-$$"`

  **Must NOT do**:
  - Do NOT assume pre-existing templates — create and cleanup own
  - Do NOT leave orphan workflows/templates — cleanup in trap
  - Do NOT test container spawning (that's T3b/T10 scope) — test RPC state machine only

  **Recommended Agent Profile**:
  - **Category**: `unspecified-high`
  - **Skills**: []

  **Parallelization**: Wave B (with T4)
  - **Blocks**: T3b, X1
  - **Blocked By**: T0, T1, T2

  **References**:
  - `bridge/pkg/secretary/rpc.go` — 11 RPC methods: secretary.start/get/cancel/advance_workflow, create/get/delete/update/list_template, get_active_count, is_running, task.create/list/cancel/get
  - `bridge/pkg/secretary/types.go` — WorkflowStatus (pending/running/blocked/completed/failed/cancelled), StepType (action/condition/parallel), Workflow, TaskTemplate
  - `bridge/pkg/secretary/orchestrator.go` — validateTransition state machine, all workflow operations
  - `bridge/pkg/secretary/pending_approval.go` — PendingApprovalManager: 120s default, 900s max
  - `bridge/pkg/rpc/server.go` — `resolve_blocker` RPC at line ~930: params {workflow_id, step_id, input, note?}

  **Acceptance Criteria**:
  - [ ] Script passes `bash -n`
  - [ ] Sources tests/lib/ fixtures
  - [ ] Creates and cleans up own templates (grep for cleanup/trap)
  - [ ] Tests ≥ 4 workflow states
  - [ ] Tests blocker creation + resolution
  - [ ] Tests restart survival
  - [ ] Clear PASS/FAIL with state transition evidence

  **QA Scenarios**:
  ```
  Scenario: Workflow state machine coverage
    Tool: Bash (grep)
    Steps:
      1. grep -c 'pending\|running\|blocked\|completed\|failed\|cancelled' tests/test-secretary-workflow-core.sh
    Expected Result: ≥ 4 distinct workflow states tested
    Evidence: .sisyphus/evidence/full-system-t3a/state-coverage.txt

  Scenario: Template lifecycle on VPS
    Tool: Bash (script execution)
    Steps:
      1. bash tests/test-secretary-workflow-core.sh
      2. Assert exit code 0 and ≥ 5 PASS markers
    Expected Result: All lifecycle stages pass, templates cleaned up
    Evidence: .sisyphus/evidence/full-system-t3a/workflow-lifecycle.txt
  ```

  **Commit**: YES
  - Message: `test(secretary): add core workflow execution harness`
  - Files: `tests/test-secretary-workflow-core.sh`

- [x] T4. Sovereign Email Pipeline Harness

  **What to do**:
  - Create `tests/test-email-pipeline.sh` that validates the email approval boundary
  - **Tier A**: Tests email approval RPCs on live VPS bridge
  - Test sequence:
    1. **M0: Prerequisites** — Source fixtures, check bridge, verify email RPCs available (`email_approval_status`)
    2. **M1: Approval status** — `email_approval_status` → verify pending_count field exists
    3. **M2: List pending** — `email.list_pending` → verify returns list (may be empty)
    4. **M3: Deny email** — Simulate deny with test approval_id → verify error or "denied" status
    5. **M4: Approve email** — Simulate approve with test approval_id → verify response shape
    6. **M5: Restart with pending** — Verify approval state survives bridge restart (if approvals can be created)
    7. **M6: Negative paths** — Invalid approval_id, already-decided approval, missing fields
  - Print summary: inbound/outbound counts, approval state transitions, blocked vs sent evidence

  **Must NOT do**:
  - Do NOT send real emails — test the approval RPC boundary only
  - Do NOT require IMAP/SMTP configuration — those are infrastructure tests
  - Do NOT modify email routing or dispatch config

  **Recommended Agent Profile**:
  - **Category**: `unspecified-high`
  - **Skills**: []

  **Parallelization**: Wave B (with T3a)
  - **Blocks**: X1
  - **Blocked By**: T0, T2

  **References**:
  - `bridge/pkg/rpc/email_approval.go` — RPC handlers: approve_email, deny_email, email_approval_status, email.list_pending
  - `bridge/pkg/email/hitl_approval.go` — EmailApprovalManager: 300s timeout, pending channel
  - `bridge/pkg/email/outbound_executor.go` — PII approval gate, provider resolution
  - Params: `approve_email {approval_id, user_id?}` → `{approval_id, status:"approved", approved_by, approved_at, message}`
  - Params: `deny_email {approval_id, user_id?, reason?}` → `{approval_id, status:"denied", denied_by, deny_reason, denied_at, message}`

  **Acceptance Criteria**:
  - [ ] Script passes `bash -n`
  - [ ] Sources tests/lib/ fixtures
  - [ ] Tests all 4 email approval RPCs
  - [ ] Tests negative paths (invalid ID, already decided)
  - [ ] Clear PASS/FAIL with approval state evidence

  **QA Scenarios**:
  ```
  Scenario: Email approval RPCs tested
    Tool: Bash (grep)
    Steps:
      1. grep -c 'approve_email\|deny_email\|email_approval_status\|email.list_pending' tests/test-email-pipeline.sh
    Expected Result: ≥ 4 (all email RPC methods covered)
    Evidence: .sisyphus/evidence/full-system-t4/rpc-coverage.txt

  Scenario: Email pipeline runs on VPS
    Tool: Bash (script execution)
    Steps:
      1. bash tests/test-email-pipeline.sh
      2. Assert exit code 0
    Expected Result: All email approval RPCs respond correctly
    Evidence: .sisyphus/evidence/full-system-t4/email-pipeline.txt
  ```

  **Commit**: YES
  - Message: `test(email): add email approval pipeline harness`
  - Files: `tests/test-email-pipeline.sh`

- [x] T3b. Secretary Workflow Deep Validation Harness

  **What to do**:
  - Create `tests/test-secretary-workflow-deep.sh` extending T3a with deeper validation
  - **Tier B**: Requires container runtime + agent spawning — skip gracefully on VPS if unavailable
  - Test sequence:
    1. **WD0: Prerequisites** — Source fixtures, check secretary available, check Docker available (skip if not)
    2. **WD1: PII-gated workflow halt** — Create workflow with PII-requiring step → verify workflow blocks at PII gate
    3. **WD2: Learned skill injection** — Verify skill injection mechanics (if learned skills present)
    4. **WD3: Parallel step execution** — Create template with parallel_split → verify concurrent execution
    5. **WD4: _events.jsonl validation** — Verify event file structure if container runs
    6. **WD5: Workflow artifact integrity** — Verify result.json structure after completion
    7. **WD6: Failover behavior** — Test FailoverRetry across multiple AgentIDs (if configured)
  - Cleanup all test data

  **Must NOT do**:
  - Do NOT require specific AI provider keys — skip if unavailable
  - Do NOT leave running containers — cleanup in trap

  **Recommended Agent Profile**:
  - **Category**: `deep`
  - **Skills**: []

  **Parallelization**: Wave C (with T5-T8)
  - **Blocks**: X1, X2
  - **Blocked By**: T3a

  **References**:
  - `bridge/pkg/secretary/orchestrator_integration.go` — StepExecutor, failover, blocker handling, learned skill injection
  - `bridge/pkg/secretary/orchestrator_parallel.go` — ParallelExecutor for parallel_split/parallel_merge
  - `bridge/pkg/secretary/event_reader.go` — EventReader: _events.jsonl tailing, 10MB cap
  - `bridge/pkg/secretary/result.go` — ContainerStepResult, StepEvent, Blocker, ExtendedStepResult
  - `bridge/pkg/skills/learned_store.go` — LearnedSkill persistence, confidence ≥ 0.4

  **Acceptance Criteria**:
  - [ ] Script passes `bash -n`
  - [ ] Graceful skip when Docker unavailable
  - [ ] Tests ≥ 2 deep validation scenarios (PII gate + events/artifacts)

  **Commit**: YES
  - Message: `test(secretary): add deep workflow validation harness`
  - Files: `tests/test-secretary-workflow-deep.sh`

- [x] T5. Sidecar / Document Pipeline End-to-End Harness

  **What to do**:
  - Create `tests/test-sidecar-docs.sh` that validates routing and extraction
  - **Tier B**: Needs Rust+Python sidecars running — skip gracefully if sockets absent
  - Test sequence:
    1. **D0: Prerequisites** — Source fixtures, check sidecar sockets exist (`/run/armorclaw/sidecar.sock`, `/run/armorclaw/sidecar-office.sock`)
    2. **D1: Rust health** — gRPC HealthCheck on sidecar.sock → verify status
    3. **D2: Python health** — gRPC HealthCheck on sidecar-office.sock → verify status
    4. **D3: Plain text (Layer 0)** — Call doc_query with .txt → verify native Go extraction
    5. **D4: PDF → Rust (Layer 1)** — Call with PDF content → verify extraction via Rust
    6. **D5: Office → Python (Layer 1)** — Call with XLSX content → verify extraction via Python
    7. **D6: Format mismatch (Layer 2)** — Call with ZIP masquerading as PDF → verify strict drop
    8. **D7: PII interception** — Call with PII in document → verify redaction/rejection
  - Skip entire script if neither sidecar socket exists

  **Recommended Agent Profile**:
  - **Category**: `unspecified-high`
  - **Skills**: []

  **Parallelization**: Wave C (with T3b, T6, T7, T8)

  **References**:
  - `bridge/pkg/sidecar/office_client.go` — RouteExtractText 3-layer logic
  - `bridge/pkg/sidecar/client.go` — Rust sidecar gRPC client
  - `bridge/pkg/sidecar/pii_interceptor.go` — PII modes: redact/reject/log_only
  - `bridge/pkg/yara/scanner.go` — YARA malware scanning
  - Sockets: `/run/armorclaw/sidecar.sock` (Rust), `/run/armorclaw/sidecar-office.sock` (Python)

  **Acceptance Criteria**:
  - [ ] Script passes `bash -n`
  - [ ] Graceful skip when sidecar sockets absent
  - [ ] Tests 3-layer routing if sockets present

  **Commit**: YES
  - Message: `test(sidecar): add document pipeline end-to-end harness`
  - Files: `tests/test-sidecar-docs.sh`

- [x] T6. Voice Stack Harness

  **What to do**:
  - Create `tests/test-voice-stack.sh` that validates voice subsystem behavior
  - **Tier B**: Voice NOT deployed on VPS — script is API contract test that skips on VPS
  - Test sequence (all skip if voice unavailable):
    1. **V0: Prerequisites** — Check voice RPCs available (skip if not)
    2. **V1: Budget enforcement** — Test token/duration limits
    3. **V2: STT smoke** — Test Transcriber interface
    4. **V3: TTS smoke** — Test Synthesizer interface
    5. **V4: VAD gating** — Test SpeechDetector interface
    6. **V5: WebRTC session** — Test session establishment
  - Entire script expected to skip on VPS

  **Recommended Agent Profile**:
  - **Category**: `unspecified-high`
  - **Skills**: []

  **Parallelization**: Wave C (with T3b, T5, T7, T8)

  **References**:
  - `bridge/pkg/voice/manager.go` — VoiceManager: CreateCall, AnswerCall, RejectCall, EndCall
  - `bridge/pkg/voice/budget.go` — BudgetTracker: 100K tokens/call, 30min duration, 80% warning
  - `bridge/pkg/voice/stt_service.go` — Transcriber interface
  - `bridge/pkg/voice/tts_service.go` — Synthesizer interface
  - `bridge/pkg/voice/vad_service.go` — SpeechDetector interface

  **Acceptance Criteria**:
  - [ ] Script passes `bash -n`
  - [ ] Exits 0 with [SKIP] on VPS (voice not deployed)
  - [ ] Has test structure for all 5 sub-tests (grep-able)

  **Commit**: YES
  - Message: `test(voice): add voice stack harness`
  - Files: `tests/test-voice-stack.sh`

- [x] T7. Jetski Browser Sidecar Harness

  **What to do**:
  - Create `tests/test-jetski-sidecar.sh` that validates browser sidecar core operation
  - **Tier B**: Jetski NOT deployed on VPS — skips gracefully
  - Test sequence (all skip if Jetski unavailable):
    1. **J0: Prerequisites** — Check port 9223 reachable (skip if not)
    2. **J1: Health check** — `GET /rpc/health` → verify status
    3. **J2: Session lifecycle** — `POST /rpc/session/create` → `POST /rpc/session/close`
    4. **J3: Status** — `GET /rpc/status` → verify active_sessions field
    5. **J4: CDP proxy** — Connect to ws://localhost:9222 → verify proxy responds
    6. **J5: PII scanner** — Send Input.insertText with SSN → verify interception
  - Entire script expected to skip on VPS

  **Recommended Agent Profile**:
  - **Category**: `unspecified-high`
  - **Skills**: []

  **Parallelization**: Wave C (with T3b, T5, T6, T8)

  **References**:
  - `jetski/internal/rpc/rpc.go` — RPC API: /rpc/status, /rpc/session/create, /rpc/session/close, /rpc/health, /rpc/approval/request
  - `jetski/internal/cdp/proxy.go` — CDP WebSocket proxy with PII scanning
  - `jetski/internal/security/pii_scanner.go` — PII patterns: SSN, CC, email, password
  - Ports: 9222 (CDP WS), 9223 (RPC API), 9333 (Lightpanda engine)
  - Docker healthcheck: `wget --spider http://localhost:9222/health`

  **Acceptance Criteria**:
  - [ ] Script passes `bash -n`
  - [ ] Exits 0 with [SKIP] on VPS
  - [ ] Tests session lifecycle structure if deployed

  **Commit**: YES
  - Message: `test(jetski): add browser sidecar harness`
  - Files: `tests/test-jetski-sidecar.sh`

- [x] T8. License Enforcement Harness

  **What to do**:
  - Create `tests/test-license-enforcement.sh` that validates enforcement behavior
  - **Tier B**: License server NOT deployed — tests bridge client RPCs only (license.status, license.features, compliance.status, platform.check)
  - Test sequence:
    1. **L0: Prerequisites** — Check license RPCs available
    2. **L1: License status** — `license.status` → verify response shape (tier, valid, compliance_mode, expires_at)
    3. **L2: Features list** — `license.features` → verify available features array
    4. **L3: Feature check** — `license.check_feature` with specific feature → verify allowed/denied
    5. **L4: Compliance status** — `compliance.status` → verify mode and fields
    6. **L5: Platform limits** — `platform.limits` → verify tier and platform limits
    7. **L6: Grace period** — Verify bridge still responds when license server unreachable (offline cache)
  - May pass on VPS (bridge client talks to cached license) or skip gracefully

  **Recommended Agent Profile**:
  - **Category**: `unspecified-high`
  - **Skills**: []

  **Parallelization**: Wave C (with T3b, T5, T6, T7)

  **References**:
  - `bridge/pkg/enforcement/rpc_handlers.go` — RPC: license.status, license.features, license.check_feature, compliance.status, platform.limits, platform.check
  - `bridge/pkg/license/client.go` — Bridge license client: offline-first caching, grace period
  - `bridge/pkg/license/state_manager.go` — States: Valid, GracePeriod, Expired, Invalid, Unknown
  - `bridge/pkg/enforcement/enforcement.go` — Compliance modes: none/basic/standard/full/strict, 26 features

  **Acceptance Criteria**:
  - [ ] Script passes `bash -n`
  - [ ] Tests license.* RPCs (shape validation at minimum)
  - [ ] Graceful handling when RPCs unavailable

  **Commit**: YES
  - Message: `test(license): add license enforcement harness`
  - Files: `tests/test-license-enforcement.sh`

- [x] T9. Platform Adapter Harness

  **What to do**:
  - Create `tests/test-platform-adapters.sh` that validates adapter registration and behavior
  - **Tier B**: Only Matrix adapter configured on VPS — Slack/others skip gracefully
  - Test sequence:
    1. **P0: Prerequisites** — Source fixtures, check bridge running
    2. **P1: Matrix adapter** — `matrix.status` → verify connected, logged_in, user_id present
    3. **P2: Matrix send/receive** — `matrix.send` → `matrix.receive` → verify round-trip (reuse harness-hardening patterns)
    4. **P3: Matrix join room** — `matrix.join_room` → verify success
    5. **P4: Bridge channel list** — `bridge.list` → verify channels array
    6. **P5: AppService status** — `bridge.appservice_status` → verify enabled, status
    7. **P6: Slack adapter** — Check if Slack configured → skip if not, test if present
  - Matrix adapter tests should pass on VPS; others skip

  **Recommended Agent Profile**:
  - **Category**: `unspecified-high`
  - **Skills**: []

  **Parallelization**: Wave D (with T10, T11)

  **References**:
  - `bridge/pkg/rpc/server.go` — bridge.start/stop/status/channel/unchannel/list/ghost_list/appservice_status, matrix.status/login/send/receive/join_room
  - `bridge/internal/adapter/matrix.go` — MatrixAdapter: HTTP Matrix v3 API, EventBus integration
  - `bridge/internal/adapter/slack.go` — SlackAdapter: Bot token auth, Socket Mode

  **Acceptance Criteria**:
  - [ ] Script passes `bash -n`
  - [ ] Tests Matrix adapter RPCs (P1-P5)
  - [ ] Graceful skip for Slack/other adapters

  **Commit**: YES
  - Message: `test(adapters): add platform adapter harness`
  - Files: `tests/test-platform-adapters.sh`

- [x] T10. Agent Runtime Invariants Harness

  **What to do**:
  - Create `tests/test-agent-runtime.sh` that validates runtime correctness properties
  - **Tier B**: Requires Docker + agent containers — skip gracefully on VPS if Docker unavailable
  - Test sequence (skip if Docker unavailable):
    1. **R0: Prerequisites** — Check Docker available, check container.* RPCs
    2. **R1: Container list** — `container.list` → verify response shape
    3. **R2: Container terminate** — `container.terminate` with invalid ID → verify error
    4. **R3: Studio agents** — `studio.list_agents` → verify response shape
    5. **R4: Studio instances** — `studio.list_instances` → verify response shape
    6. **R5: Runtime health** — `studio.get_stats` → verify stats present
    7. **R6: No cross-agent leakage** — Verify container isolation (if containers running)
  - Tests RPC response shapes even without running containers

  **Recommended Agent Profile**:
  - **Category**: `deep`
  - **Skills**: []

  **Parallelization**: Wave D (with T9, T11)

  **References**:
  - `bridge/pkg/rpc/server.go` — container.terminate, container.list
  - `bridge/pkg/runtime/runtime.go` — Runtime interface, ContainerSpec, DefaultContainerSpec
  - `bridge/pkg/studio/integration.go` — 27 studio.* methods including list_agents, list_instances, get_stats
  - Security: user 10001, cap-drop ALL, read-only rootfs, 512MB memory, pids=100

  **Acceptance Criteria**:
  - [ ] Script passes `bash -n`
  - [ ] Tests container.* and studio.* RPC shapes
  - [ ] Graceful skip when Docker unavailable

  **Commit**: YES
  - Message: `test(runtime): add agent runtime invariants harness`
  - Files: `tests/test-agent-runtime.sh`

- [x] T11. Deployment/USB Validation Harness

  **What to do**:
  - Create `tests/test-deployment-usb.sh` that validates hardware/deployment edge paths
  - **Tier B**: Requires physical USB device — always skips on VPS
  - Test sequence (all skip):
    1. **U0: Prerequisites** — Check USB device detection capability → skip if not physical hardware
    2. **U1: Device detection** — Verify USB enumeration
    3. **U2: Permission gating** — Verify unprivileged access denied
    4. **U3: Unsafe device refusal** — Verify unknown devices rejected
    5. **U4: Metadata extraction** — Verify safe device metadata
    6. **U5: No-device behavior** — Verify clean behavior when no device present
  - Always skips on VPS — provides test structure for future hardware testing

  **Recommended Agent Profile**:
  - **Category**: `quick`
  - **Skills**: []

  **Parallelization**: Wave D (with T9, T10)

  **Acceptance Criteria**:
  - [ ] Script passes `bash -n`
  - [ ] Exits 0 with [SKIP] on VPS
  - [ ] Has test structure for all 5 sub-tests

  **Commit**: YES
  - Message: `test(deploy): add deployment/USB validation harness`
  - Files: `tests/test-deployment-usb.sh`

- [x] X1. Workflow → Email Approval Scenario

  **What to do**:
  - Create cross-subsystem test combining T3a (workflow) + T4 (email approval)
  - **Tier A**: Uses secretary + email RPCs on VPS
  - Scenario: Create workflow template with email-sending step → start workflow → verify email approval triggered → approve/deny → verify workflow state reflects decision
  - Skip if either subsystem unavailable

  **Recommended Agent Profile**:
  - **Category**: `unspecified-high`
  - **Skills**: []

  **Parallelization**: Wave E (sequential with X2, X3, X4)

  **References**:
  - T3a and T4 references combined
  - `bridge/pkg/secretary/approvals.go` — ApprovalEngine for workflow PII approval
  - `bridge/pkg/email/hitl_approval.go` — EmailApprovalManager

  **Commit**: YES (with X2-X4 or individually)

- [x] X2. Workflow → Document Sidecar Scenario

  **What to do**:
  - Create cross-subsystem test combining T3b (workflow deep) + T5 (sidecar)
  - **Tier B**: Requires container runtime + sidecars
  - Scenario: Workflow sends document task to sidecar → sidecar returns structured output → workflow consumes result
  - Skip if either subsystem unavailable

  **Recommended Agent Profile**:
  - **Category**: `unspecified-high`
  - **Skills**: []

  **Parallelization**: Wave E (sequential)

  **References**:
  - T3b and T5 references combined

  **Commit**: YES (grouped with cross-subsystem)

- [x] X3. Browser Action → Trust Block Scenario

  **What to do**:
  - Create cross-subsystem test combining T2 (trust) + T7 (Jetski)
  - **Tier B**: Requires Jetski + trust layer
  - Scenario: Jetski CDP action hits trust boundary → event emitted → action blocked or approval required
  - Skip if either subsystem unavailable

  **Recommended Agent Profile**:
  - **Category**: `unspecified-high`
  - **Skills**: []

  **Parallelization**: Wave E (sequential)

  **Commit**: YES (grouped with cross-subsystem)

- [x] X4. Event Stream Truth Scenario

  **What to do**:
  - Create cross-subsystem test using T1 (EventBus)
  - **Tier A/B**: Uses WebSocket event streaming + triggers events from multiple subsystems
  - Scenario: Emit workflow + approval + sidecar events → verify live stream reflects all state transitions in order
  - Skip if WebSocket not enabled

  **Recommended Agent Profile**:
  - **Category**: `unspecified-high`
  - **Skills**: []

  **Parallelization**: Wave E (sequential)

  **Commit**: YES (grouped with cross-subsystem)
  - Message: `test(cross): add cross-subsystem integration scenarios`
  - Files: `tests/test-cross-workflow-email.sh`, `tests/test-cross-workflow-docs.sh`, `tests/test-cross-browser-trust.sh`, `tests/test-cross-event-truth.sh`

---

## Final Verification Wave (MANDATORY — after ALL implementation tasks)

> 4 review agents run in PARALLEL. ALL must APPROVE. Present consolidated results to user and get explicit "okay" before completing.

- [x] F1. **Plan Compliance Audit** — `oracle`
  Read the plan end-to-end. For each "Must Have": verify implementation exists. For each "Must NOT Have": search codebase for forbidden patterns. Check evidence files exist in `.sisyphus/evidence/full-system-*/`. Verify no overlap with v080-comprehensive-tests.md scope. Compare deliverables against plan.
  Output: `Must Have [N/N] | Must NOT Have [N/N] | Tasks [N/N] | VERDICT: APPROVE/REJECT`

- [x] F2. **Code Quality Review** — `unspecified-high`
  Run `bash -n` on all test scripts. Review all files for: excessive comments, over-abstraction, generic names, unused variables. Verify all scripts source common.sh (not reimplement). Check no source code outside tests/ was modified. Verify Tier A/B classification is accurate.
  Output: `Build [PASS/FAIL] | Lint [PASS/FAIL] | Files [N clean/N issues] | VERDICT`

- [x] F3. **Full Automated Execution Audit** — `unspecified-high`
  Run ALL Tier A scripts against VPS, capture outputs. Run ALL Tier B scripts locally for structure check. Verify all scripts source .env. Verify evidence files created. Verify total harness execution ≤ 10 minutes. Verify bridge restart serialization (no concurrent restarts).
  Output: `Tier A [N/N pass] | Tier B [N/N structure-ok] | Evidence [N files] | VERDICT`

- [x] F4. **Scope Fidelity Check** — `deep`
  For each task: read "What to do", read actual git diff. Verify 1:1. Check "Must NOT do" compliance. Verify only tests/ and tests/lib/ modified. Flag unaccounted changes. Verify no overlap with Go/Rust unit tests in v080-comprehensive-tests.md.
  Output: `Tasks [N/N compliant] | Contamination [CLEAN/N issues] | Unaccounted [CLEAN/N files] | VERDICT`

---

## Commit Strategy

- **Commit 0**: `feat(tests): add shared test fixtures and helper library` — tests/lib/*, tests/fixtures/*
- **Commit 1**: `test(eventbus): add EventBus + WebSocket live event streaming harness` — tests/test-eventbus-streaming.sh
- **Commit 2**: `test(trust): add security/trust layer harness` — tests/test-trust-layer.sh
- **Commit 3**: `test(health): add system health baseline harness` — tests/test-system-health-baseline.sh
- **Commit 4**: `test(secretary): add core workflow execution harness` — tests/test-secretary-workflow-core.sh
- **Commit 5**: `test(email): add email approval pipeline harness` — tests/test-email-pipeline.sh
- **Commit 6**: `test(secretary): add deep workflow validation harness` — tests/test-secretary-workflow-deep.sh
- **Commit 7**: `test(sidecar): add document pipeline end-to-end harness` — tests/test-sidecar-docs.sh
- **Commit 8**: `test(voice): add voice stack harness` — tests/test-voice-stack.sh
- **Commit 9**: `test(jetski): add browser sidecar harness` — tests/test-jetski-sidecar.sh
- **Commit 10**: `test(license): add license enforcement harness` — tests/test-license-enforcement.sh
- **Commit 11**: `test(adapters): add platform adapter harness` — tests/test-platform-adapters.sh
- **Commit 12**: `test(runtime): add agent runtime invariants harness` — tests/test-agent-runtime.sh
- **Commit 13**: `test(deploy): add deployment/USB validation harness` — tests/test-deployment-usb.sh
- **Commit 14**: `test(cross): add cross-subsystem integration scenarios` — tests/test-cross-*.sh or scenarios in existing scripts

---

## Success Criteria

### Verification Commands
```bash
# Shared fixtures exist
ls tests/lib/load_env.sh tests/lib/assert_json.sh tests/lib/restart_bridge.sh tests/lib/event_subscriber_helper.sh tests/lib/common_output.sh tests/lib/README.md

# All test scripts pass syntax check
for f in tests/test-*.sh; do bash -n "$f" && echo "OK: $f"; done

# Tier A scripts pass on VPS
bash tests/test-system-health-baseline.sh  # exit 0
bash tests/test-trust-layer.sh             # exit 0

# Tier B scripts have graceful skip
bash tests/test-voice-stack.sh             # exit 0 (with [SKIP] output)
bash tests/test-jetski-sidecar.sh          # exit 0 (with [SKIP] output)

# No source code modified outside tests/
git diff --name-only HEAD~14 -- ':!tests/' ':!.sisyphus/'  # should be empty
```

### Final Checklist
- [ ] All "Must Have" present
- [ ] All "Must NOT Have" absent
- [ ] All Tier A scripts pass live on VPS
- [ ] All Tier B scripts pass structure check or skip gracefully
- [ ] Full harness ≤ 10 minutes
- [ ] No files modified outside tests/ and .sisyphus/
- [ ] No overlap with v080-comprehensive-tests.md
