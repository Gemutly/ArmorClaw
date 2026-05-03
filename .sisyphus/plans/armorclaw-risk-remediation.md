# ArmorClaw Server-Side Risk Remediation Sprint

## TL;DR

> **Quick Summary**: Full Matrix E2EE implementation on the Go Bridge, Agent State Visibility wiring, TLS metadata bug fixes, Azure Blob rustls migration, and state enum consolidation — server components only.
> 
> **Deliverables**:
> - Matrix E2EE (Olm/Megolm encrypt/decrypt, SAS verification, cross-signing) on the Bridge
> - Agent state inference wired end-to-end (Jetski CDP → Bridge InferAgentState → Matrix events)
> - 3 TLS metadata bugs fixed (hardcoded mode, incomplete HMAC, missing well-known fields)
> - Azure Blob connector re-enabled with rustls
> - State enum audit and safe consolidation
> 
> **Estimated Effort**: XL (6-10 weeks)
> **Parallel Execution**: YES — 5 waves
> **Critical Path**: W1 Validation → W2 ToDevice + Crypto Store → W2 OlmMachine → W3 Encrypt/Decrypt → W4 SAS + Cross-Signing → W5 Integration Tests → FINAL

---

## Context

### Original Request
User provided a detailed GSD plan (6 phases, 23 tasks) for "ArmorChat Risk Remediation Sprint." Upon codebase verification, the plan was grounded in a different architecture (Kotlin Multiplatform client-side). Re-scoped to server components only: Bridge (Go), Jetski (Go), Conduit, sidecars.

### Interview Summary
**Key Discussions**:
- Original plan referenced non-existent KMP shared module — re-scoped to Go Bridge server-side
- Confirmed Bridge has ZERO Matrix E2EE — documented in `HANDOFF_E2EE.md` (4-7 weeks estimate)
- Agent state visibility infrastructure exists but unwired — needs Jetski CDP → Bridge connection
- TLS QR payload has 3 bugs: hardcoded mode, incomplete HMAC signature, missing well-known fields
- User confirmed server-only scope (no ArmorChat Android client changes)

**Research Findings**:
- Crypto store infrastructure exists (`pkg/crypto/store.go` + `keystore_store.go` SQLCipher-backed) — can be extended
- `key_ingestion.go` has skeleton handlers for crypto events but stubs actual crypto operations
- SyncResponse struct in BOTH `matrix.go` AND `client.go` lacks `ToDevice` field — E2EE prerequisite
- Sync filter explicitly excludes encrypted events (`m.room.encrypted`)
- `mautrix-go` v0.26.4 is in go.mod but no code imports `maunium.net/go/mautrix/crypto`
- Agent state: `InferAgentState()` + `ApplyInferredState()` ready, `BroadcastStatus()` already publishes events
- TLS: `deriveTLSMode()` is correct but `updateQRTLSInfo()` ignores it (hardcodes "private")
- Azure Blob: 128-line disabled connector, clean rustls migration path

### Metis Review
**Identified Gaps** (addressed):
- Conduit E2EE support unvalidated — added prerequisite validation task (Task 1)
- Jetski CDP event streaming unvalidated — added prerequisite validation task (Task 2)
- mautrix-go compatibility with Conduit unconfirmed — validation covers this
- State enum values are Matrix protocol contract — guardrail: MUST NOT change string constants
- E2EE needs kill switch for rollback — added `matrix.e2ee.enabled` config gate
- `signConfig()` v2 HMAC omits `TLSTrustHint` and `CertExpiresAt` — confirmed as security bug

---

## Work Objectives

### Core Objective
Implement full Matrix E2EE on the Go Bridge, wire agent state inference end-to-end, fix TLS metadata bugs, re-enable Azure Blob connector, and audit state enums — all server-side only.

### Concrete Deliverables
- `ToDevice` field added to both SyncResponse types
- mautrix-go/crypto OlmMachine integrated into Bridge
- Encrypt/decrypt wired into message pipeline with dual-mode support
- SAS verification and cross-signing bootstrap working
- Jetski CDP events flowing to Bridge InferAgentState
- Workflow side-channel signals wired
- TLS mode derived correctly (not hardcoded)
- v2 QR signature covers all TLS fields
- Well-known endpoint enriched with TLS metadata
- Azure Blob connector builds and passes tests with rustls
- State enum audit document with safe consolidation plan

### Definition of Done
- [ ] Bridge can encrypt/decrypt Matrix messages in E2EE rooms
- [ ] Bridge can participate in SAS emoji verification
- [ ] Bridge can bootstrap cross-signing
- [ ] Agent state transitions visible via `com.armorclaw.agent.status` Matrix events
- [ ] TLS QR v2 payload has correct mode and complete HMAC signature
- [ ] Azure Blob connector compiles with rustls and passes CRUD tests
- [ ] All existing tests still pass: `go test ./...` in bridge/, `cargo test --lib` in sidecar/

### Must Have
- E2EE kill switch (`matrix.e2ee.enabled` config, default false)
- Dual-mode messaging (plaintext for unencrypted rooms, encrypted for encrypted rooms)
- Existing crypto store infrastructure reused (SQLCipher keystore)
- Agent state wiring preserves existing HITL approval flow
- v1 QR clients unaffected by TLS fixes

### Must NOT Have (Guardrails)
- MUST NOT change AgentStatus string constants (Matrix protocol contract with Android)
- MUST NOT replace Bridge's custom sync loop with mautrix-go's built-in sync
- MUST NOT create a separate database for crypto — extend existing SQLCipher keystore schema
- MUST NOT break plaintext messaging in unencrypted rooms
- MUST NOT modify StateMachine.Transition() — inference uses ForceTransition by design
- MUST NOT break existing HITL approval flow (AWAITING_APPROVAL is protected)
- MUST NOT change ConfigPayload struct layout (consumed by Android)
- MUST NOT touch S3 connector (`aws_s3.rs`) during Azure Blob work
- MUST NOT use native-tls or OpenSSL for Azure Blob — rustls only
- MUST NOT implement key gossip, multi-bridge sync, or key backup/restore (scope creep)

---

## Verification Strategy

> **ZERO HUMAN INTERVENTION** — ALL verification is agent-executed. No exceptions.
> Acceptance criteria requiring "user manually tests/confirms" are FORBIDDEN.

### Test Decision
- **Infrastructure exists**: YES (Go tests in bridge/, Rust tests in sidecar/, Bash harness in tests/)
- **Automated tests**: YES (Tests-after — infrastructure work, not TDD)
- **Framework**: Go testing + Rust cargo test + Bash integration test harness
- **E2EE tests**: Integration tests against Conduit test server

### QA Policy
Every task MUST include agent-executed QA scenarios.
Evidence saved to `.sisyphus/evidence/task-{N}-{scenario-slug}.{ext}`.

- **Go Bridge**: Use Bash (`go test`, `curl`) — Run tests, check API responses, verify database state
- **Jetski/Bridge integration**: Use Bash — Simulate events, verify state transitions
- **Rust sidecar**: Use Bash (`cargo test`) — Build and test
- **TLS**: Use Bash (`curl`, `openssl`) — Verify endpoints and payloads

---

## Execution Strategy

### Parallel Execution Waves

```
Wave 1 (Start Immediately — validation + quick wins + sync prep):
├── Task 1: Validate Conduit E2EE support [quick] ← GATE: if FAIL, stop E2EE workstream
├── Task 2: Validate Jetski CDP event access [quick] ← GATE: if FAIL, need T2.5
├── Task 3: Fix TLS hardcoded mode bug [quick]
├── Task 4: Fix TLS signConfig HMAC security bug [quick]
├── Task 5: Fix TLS well-known endpoint enrichment [quick] (sequential after T3 — same file)
├── Task 6: Azure Blob rustls migration [unspecified-high]
├── Task 7: Add ToDevice + m.room.encrypted filter [quick]
│   NOTE: T7 does NOT depend on T1 — struct/filter changes proceed regardless of Conduit validation
└── Task 23: State enum audit and consolidation [unspecified-high]
    NOTE: Moved to Wave 1 — runs before other tasks touch enum files, avoids merge conflicts

Wave 1.5 (Conditional — only if T2 finds no CDP event stream):
└── Task 2.5: Add CDP event streaming endpoint to Jetski RPC API [deep]
    ONLY if T2 output has `cdp_event_stream_exists: false`. Skipped if true.

Wave 2 (After Wave 1 — E2EE foundation + agent state wiring):
├── Task 7.5: Device ID persistence + Conduit test environment [quick]
├── Task 8: Add E2EE config kill switch + runtime toggle (admin-auth + audit-log) [quick]
├── Task 9: Expand crypto.Store for mautrix-go integration (core methods) [deep]
│   NOTE: Cross-signing/verification store methods deferred to Wave 3b alongside T16-T17
│   NOTE: Interface designed for additive extension (T9.5 won't change existing method signatures)
├── Task 10: Add Jetski CDP event subscriber to Bridge [unspecified-high] (depends: T2 or T2.5)
├── Task 11: Wire CDP events → InferAgentState → StateMachine [unspecified-high]
└── Task 12: Wire workflow side-channel signals [unspecified-high]

Wave 3a (After Wave 2 — E2EE core, SEQUENTIAL — shared file matrix.go):
├── Task 13: Wire mautrix-go OlmMachine + SyncResponseAdapter [deep]
│   Includes: SyncResponseAdapter (custom SyncResponse → mautrix.RespSync), OTK count tracking
│   BLOCKING: goolm interop test MUST pass before proceeding to T14. If fails, pivot to -tags libolm.
├── Task 14: Add E2EE encrypt/decrypt + RoomEncryptionCache to message pipeline [deep]
│   Includes: room encryption status caching, Megolm rotation, placeholder on decrypt failure + retry queue
│   Performance gate: encryption <20ms/msg
└── Task 15: Device key upload/query API handlers [deep]

Wave 3b (After Wave 3a — verification features + agent polish):
├── Task 9.5: Expand crypto.Store (cross-signing + verification methods) [deep]
├── Task 16: SAS verification implementation [deep]
├── Task 17: Cross-signing bootstrap (with UIAA solution) [deep]
│   UIAA strategy selected based on Task 1 validation results
├── Task 18: Handle Jetski disconnection gracefully [quick]
└── Task 19: Add state transition logging [quick]

Wave 4 (After Wave 3 — integration + regression):
├── Task 20: Update isBridgeVerified trust model [quick]
├── Task 21: E2EE integration tests (includes deployment migration scenario) [unspecified-high]
├── Task 22: Agent state E2E tests [unspecified-high]
└── Task 24: Full regression pass [unspecified-high]

Wave FINAL (After ALL tasks — 4 parallel reviews):
├── Task F1: Plan compliance audit (oracle)
├── Task F2: Code quality review (unspecified-high)
├── Task F3: Agent-executed integration QA (unspecified-high)
└── Task F4: Scope fidelity check (deep)
→ Present results → Get explicit user okay

Critical Path: T1 → T9 → T13 → T14 → T15 → T16/T17 → T21 → T24 → FINAL
Agent State Path: T2 → (T2.5?) → T10 → T11 → T12 → T22 (parallel with E2EE)
TLS Quick Wins: T3 → T5 → T4 (all independent, ship immediately)
Parallel Speedup: ~50% faster than sequential
Max Concurrent: 8 (Wave 1)
```

### Dependency Matrix

| Task | Depends On | Blocks | Wave |
|------|-----------|--------|------|
| 1 | — | 9, 13 | 1 |
| 2 | — | 10, 2.5 | 1 |
| 2.5 | 2 (conditional) | 10 | 1.5 |
| 3 | — | 5 | 1 |
| 4 | — | — | 1 |
| 5 | 3 | — | 1 |
| 6 | — | — | 1 |
| 7 | — | 8, 9, 13 | 1 |
| 8 | 7 | 13 | 2 |
| 9 | 7 | 13 | 2 |
| 9.5 | 9, 15 | 16, 17 | 3b |
| 10 | 2 or 2.5 | 11 | 2 |
| 11 | 10 | 12, 18, 19 | 2 |
| 12 | 11 | 22 | 2 |
| 13 | 1, 8, 9 | 14 | 3a |
| 14 | 13 | 15 | 3a |
| 15 | 14 | 9.5, 16, 17 | 3a |
| 16 | 15, 9.5 | 20, 21 | 3b |
| 17 | 15, 9.5 | 20, 21 | 3b |
| 18 | 11 | — | 3b |
| 19 | 11 | — | 3b |
| 20 | 16, 17 | — | 4 |
| 21 | 14, 16, 17 | 24 | 4 |
| 22 | 11, 12 | 24 | 4 |
| 23 | — | 24 | 4 |
| 24 | 21, 22, 23 | FINAL | 4 |

### Agent Dispatch Summary

- **Wave 1**: **8** — T1-T2 → `quick`, T3-T5 → `quick` (T5 sequential after T3), T6 → `unspecified-high`, T7 → `quick`, T23 → `unspecified-high`
- **Wave 1.5**: **0-1** — T2.5 → `deep` (ONLY if T2 output has `cdp_event_stream_exists: false`, conditional)
- **Wave 2**: **6** — T7.5 → `quick`, T8 → `quick`, T9 → `deep`, T10-T12 → `unspecified-high`
- **Wave 3a**: **3** — T13-T15 → `deep` (SEQUENTIAL — shared matrix.go, T13 has BLOCKING goolm interop gate)
- **Wave 3b**: **5** — T9.5 → `deep`, T16-T17 → `deep`, T18-T19 → `quick`
- **Wave 4**: **4** — T20 → `quick`, T21-T22 → `unspecified-high`, T24 → `unspecified-high`
- **FINAL**: **4** — F1 → `oracle`, F2-F3 → `unspecified-high`, F4 → `deep`

---

## TODOs

- [x] 1. Validate Conduit E2EE Support (Prerequisite Spike)

  **What to do**:
  - Send a `/sync` request to Conduit with a `ToDevice` filter and verify the response includes a `to_device` section
  - Verify Conduit supports `/keys/upload`, `/keys/query`, `/keys/claim` endpoints
  - Create a test Olm account using mautrix-go/crypto and attempt key exchange with Conduit
  - Document Conduit's E2EE capability matrix: ToDevice ✅/❌, device keys ✅/❌, cross-signing ✅/❌
  - If Conduit LACKS support, document the gap and alternative approaches (e.g., contribute to Conduit, use Synapse)

  **Must NOT do**:
  - Do NOT start any E2EE implementation until validation passes
  - Do NOT assume Conduit supports all E2EE features without testing

  **Recommended Agent Profile**:
  - **Category**: `quick`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 1 (with Tasks 2-6)
  - **Blocks**: Tasks 7, 9 (E2EE foundation)
  - **Blocked By**: None

  **References**:
  - `bridge/internal/adapter/matrix.go:76-107` — Current sync filter (no ToDevice section)
  - `bridge/internal/adapter/matrix.go:127-136` — SyncResponse struct (no ToDevice field)
  - `bridge/doc/HANDOFF_E2EE.md` — E2EE implementation handoff document, lists phases
  - `bridge/doc/LESSONS_LEARNED.md:18-21` — "E2EE is not implemented at all"
  - `go.mod` — mautrix-go v0.26.4 is listed but no crypto imports

  **Acceptance Criteria**:
  - [ ] Document written to `.sisyphus/evidence/task-1-conduit-e2ee-validation.md` with capability matrix
  - [ ] Go test file `bridge/internal/adapter/conduit_e2ee_test.go` exists with validation tests
  - [ ] `go test -run TestConduitE2EE ./internal/adapter/...` → PASS or documented failure

  **QA Scenarios**:
  ```
  Scenario: Conduit ToDevice support validation
    Tool: Bash
    Preconditions: Conduit running at http://localhost:6167
    Steps:
      1. Create test user on Conduit: `curl -X POST http://localhost:6167/_matrix/client/v3/register -H "Content-Type: application/json" -d '{"username":"e2ee-test","password":"test123","auth":{"type":"m.login.dummy"}}'`
      2. Login and get access token
      3. Send `/sync` with filter: `{"to_device":{"limit":10}}`
      4. Assert response body contains `"to_device"` key
    Expected Result: Response includes `to_device` section (even if empty array)
    Failure Indicators: Response lacks `to_device` key, HTTP 400/500 on ToDevice filter
    Evidence: .sisyphus/evidence/task-1-conduit-todevice.txt

  Scenario: Conduit key upload/query support
    Tool: Bash
    Preconditions: Conduit running, test user logged in
    Steps:
      1. `curl -X POST http://localhost:6167/_matrix/client/v3/keys/upload -H "Authorization: Bearer $TOKEN" -d '{"device_keys":{}}'`
      2. Assert response contains `"one_time_key_counts"` key
      3. `curl -X POST http://localhost:6167/_matrix/client/v3/keys/query -d '{"@e2ee-test:localhost":{}}'`
      4. Assert response contains device key data structure
    Expected Result: Both endpoints return valid Matrix responses
    Failure Indicators: HTTP 404 (endpoint not supported), malformed response
    Evidence: .sisyphus/evidence/task-1-conduit-keys.txt

  Scenario: Cross-signing UIAA requirement validation
    Tool: Bash
    Preconditions: Conduit running, test user logged in with fresh access token
    Steps:
      1. After login, immediately call `POST /_matrix/client/v3/keys/device_signing/upload` with empty cross-signing keys
      2. If HTTP 401: document which auth flows are required (UIAA required — Strategy A or B)
      3. If HTTP 200: document that no UIAA is required (Strategy C applies to Task 17)
      4. Test Strategy A: within 60s of login, retry upload — does it succeed without re-auth?
      5. Document applicable UIAA strategy in evidence file
    Expected Result: UIAA requirement documented with applicable strategy for Task 17
    Failure Indicators: Unexpected HTTP 403, server error, or undocumented auth flow
    Evidence: .sisyphus/evidence/task-1-crosssign-uiaa.txt
  ```

  **Commit**: YES (groups with 2)
  - Message: `chore(e2ee): add Conduit E2EE capability validation tests`
  - Files: `bridge/internal/adapter/conduit_e2ee_test.go`, `.sisyphus/evidence/task-1-*.md`

- [x] 2. Validate Jetski CDP Event Access (Prerequisite Spike)

  **What to do**:
  - Read Jetski RPC API source (`jetski/internal/rpc/rpc.go`) and determine if it exposes CDP events to external consumers
  - Check if Jetski streams CDP events via WebSocket, gRPC, or polling
  - Verify the RPC endpoint at port 9223 is accessible from the Bridge
  - Document available CDP event types and their data structures
  - If Jetski DOES NOT expose CDP events, document what changes are needed and estimate effort

  **Must NOT do**:
  - Do NOT modify Jetski source code — only investigate and document
  - Do NOT assume CDP events are available without verification

  **Recommended Agent Profile**:
  - **Category**: `quick`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 1 (with Tasks 1, 3-6)
  - **Blocks**: Task 10 (Jetski subscriber)
  - **Blocked By**: None

  **References**:
  - `jetski/internal/rpc/rpc.go` — RPC server (117 lines), check for event subscription endpoints
  - `jetski/internal/cdp/proxy.go` — CDP proxy (476 lines), check for event interception points
  - `bridge/pkg/agent/state_inference.go` — InferAgentState takes `[]CDPEvent` as input
  - `bridge/pkg/agent/state_inference.go:30-45` — Expected CDP event types (Page.frameNavigated, DOM.focus, etc.)

  **Acceptance Criteria**:
  - [ ] Document written to `.sisyphus/evidence/task-2-jetski-cdp-validation.md`
  - [ ] Document includes: available RPC methods, CDP event access mechanism, data format
  - [ ] Document includes boolean field `cdp_event_stream_exists: true/false` (triggers Task 2.5 if false)
  - [ ] If changes needed: document specific files/functions to modify in Jetski

  **QA Scenarios**:
  ```
  Scenario: Jetski RPC API capability check
    Tool: Bash
    Preconditions: Read jetski/internal/rpc/rpc.go source code
    Steps:
      1. List all RPC method registrations in rpc.go
      2. For each method, document: method name, parameters, return type
      3. Check if any method returns CDP events or event stream
      4. Check proxy.go for event interception/emission hooks
    Expected Result: Complete inventory of Jetski RPC API surface
    Failure Indicators: No CDP event access mechanism found
    Evidence: .sisyphus/evidence/task-2-jetski-cdp-validation.md

  Scenario: Bridge-to-Jetski connectivity test
    Tool: Bash
    Preconditions: Jetski running on port 9223
    Steps:
      1. `curl http://localhost:9223/rpc -d '{"jsonrpc":"2.0","method":"status","id":1}'`
      2. Assert response contains Jetski status data
      3. Test any discovered event subscription endpoint
    Expected Result: Bridge can reach Jetski RPC API
    Failure Indicators: Connection refused, no response, empty response
    Evidence: .sisyphus/evidence/task-2-jetski-connectivity.txt
  ```

  **Commit**: YES (groups with 1)
  - Message: `chore(agent): document Jetski CDP event access validation`
  - Files: `.sisyphus/evidence/task-2-*.md`

- [ ] 2.5. Add CDP Event Streaming Endpoint to Jetski RPC API (CONDITIONAL)

  > **⚠️ CONDITIONAL TASK**: Execute ONLY if Task 2 validation finds no existing CDP event streaming mechanism.
  > Skip entirely if Jetski already exposes CDP events to external consumers.
  >
  > **Decision gate**: Task 2's output document MUST include a boolean field `cdp_event_stream_exists: true/false`.
  > If `false`, Task 2.5 is triggered. If `true`, Task 2.5 is skipped and Task 10 proceeds directly.

  **What to do**:
  - Add `events.subscribe` RPC method to Jetski's RPC server (`jetski/internal/rpc/rpc.go`)
  - Create a WebSocket or SSE streaming endpoint that pushes intercepted CDP events to subscribers
  - Intercept CDP events in `jetski/internal/cdp/proxy.go` during proxy pass-through and emit them
  - Redact PII from event params (URLs may contain tokens, DOM content may contain PII)
  - Include registration handshake: subscriber must send `{"type":"register","payload":{"device_id":"..."}}`
  - Add configuration flag: `emit_state_events: true/false`
  - Map relevant CDP event types: `Page.frameNavigated`, `DOM.focus`, `Runtime.executionContextCreated`, `Page.javascriptDialogOpening`

  **Must NOT do**:
  - Do NOT log raw CDP event content (PII risk)
  - Do NOT modify existing CDP proxy behavior for browser sessions
  - Do NOT add network dependencies to Jetski (already runs with NetworkMode constraints)

  **Recommended Agent Profile**:
  - **Category**: `deep`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: NO (conditional, sequential after Task 2)
  - **Parallel Group**: Wave 1.5 (only if needed)
  - **Blocks**: Task 10
  - **Blocked By**: Task 2 (must confirm no existing stream)

  **References**:
  - `jetski/internal/rpc/rpc.go` — RPC server (117 lines), add streaming endpoint here
  - `jetski/internal/cdp/proxy.go` — CDP proxy (476 lines), intercept events here
  - `bridge/pkg/agent/state_inference.go:30-45` — Expected CDPEvent types and structure

  **Acceptance Criteria**:
  - [ ] `events.subscribe` RPC method registered and documented
  - [ ] CDP events intercepted and emitted with PII redaction
  - [ ] Registration handshake enforced before event delivery
  - [ ] `emit_state_events` config flag works (default false)
  - [ ] Jetski-side tests pass

  **QA Scenarios**:
  ```
  Scenario: CDP events streamed to subscriber
    Tool: Bash
    Preconditions: Jetski running with emit_state_events=true, browser session active
    Steps:
      1. Connect to Jetski RPC events.subscribe endpoint
      2. Send registration handshake with device_id
      3. Navigate browser to https://example.com
      4. Verify CDP event received: {"method":"Page.frameNavigated","params":{...}}
    Expected Result: CDP events received with redacted PII
    Failure Indicators: No events received, events contain raw URLs/DOM content
    Evidence: .sisyphus/evidence/task-2.5-cdp-stream.txt
  ```

  **Commit**: YES
  - Message: `feat(jetski): add CDP event streaming endpoint for state inference`
  - Files: `jetski/internal/rpc/rpc.go`, `jetski/internal/cdp/proxy.go`, `jetski/internal/cdp/event_emitter.go`

- [x] 3. Fix TLS Hardcoded Mode Bug

  **What to do**:
  - Fix `updateQRTLSInfo()` in `bridge/pkg/http/server.go:445` to call `s.deriveTLSMode()` instead of hardcoding `"private"`
  - Verify the fix works for all deployment modes: native (→ "none"), sentinel+self-signed (→ "private"), sentinel+CA (→ "public"), cloudflare
  - Ensure native mode explicitly calls `SetTLSInfo("none", "", "", 0)` with zero-values

  **Must NOT do**:
  - Do NOT change the `ConfigPayload` struct layout
  - Do NOT break v1 QR clients (v2 fields are already gated by `ARMORCLAW_QR_VERSION`)

  **Recommended Agent Profile**:
  - **Category**: `quick`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 1 (with Tasks 1-2, 4-6)
  - **Blocks**: None
  - **Blocked By**: None

  **References**:
  - `bridge/pkg/http/server.go:424-447` — `updateQRTLSInfo()` with hardcoded `"private"` at line 445
  - `bridge/pkg/http/server.go:344-366` — `deriveTLSMode()` has correct logic (none/private/public)
  - `bridge/pkg/qr/public.go:285-299` — ConfigPayload struct with TLS fields (lines 293-296)
  - `bridge/pkg/qr/public.go:322-328` — v2 gating by `ARMORCLAW_QR_VERSION=2`

  **Acceptance Criteria**:
  - [ ] `updateQRTLSInfo()` calls `s.deriveTLSMode()` instead of hardcoding
  - [ ] Test: native mode → TLS mode = `"none"`, fingerprint = `""`, expires = `0`
  - [ ] Test: sentinel+self-signed → TLS mode = `"private"`, fingerprint populated
  - [ ] Test: sentinel+CA → TLS mode = `"public"`, fingerprint populated
  - [ ] `go test ./pkg/http/... -run TestUpdateQRTLSInfo` → PASS

  **QA Scenarios**:
  ```
  Scenario: Native mode produces zero-value TLS fields
    Tool: Bash
    Preconditions: Bridge started with ARMORCLAW_SERVER_MODE=native (no cert)
    Steps:
      1. Start bridge in native mode
      2. Set ARMORCLAW_QR_VERSION=2
      3. `curl http://localhost:8443/qr/config`
      4. Parse JSON response, check tls_mode field
    Expected Result: tls_mode="" or tls_mode="none", tls_fingerprint_sha256="", cert_expires_at=0
    Failure Indicators: tls_mode="private" when no cert is loaded
    Evidence: .sisyphus/evidence/task-3-tls-native-mode.txt

  Scenario: Sentinel+CA mode produces "public" TLS mode
    Tool: Bash
    Preconditions: Bridge started with ARMORCLAW_SERVER_MODE=sentinel, CA-issued cert loaded
    Steps:
      1. Start bridge in sentinel mode with CA cert
      2. Set ARMORCLAW_QR_VERSION=2
      3. `curl http://localhost:8443/qr/config`
      4. Check tls_mode field
    Expected Result: tls_mode="public", non-empty fingerprint, non-zero cert_expires_at
    Failure Indicators: tls_mode="private" for CA-issued cert
    Evidence: .sisyphus/evidence/task-3-tls-sentinel-ca.txt
  ```

  **Commit**: YES
  - Message: `fix(tls): derive TLS mode from server config instead of hardcoding`
  - Files: `bridge/pkg/http/server.go`
  - Pre-commit: `go test ./pkg/http/... ./pkg/qr/...`

- [x] 4. Fix TLS signConfig HMAC Security Bug

  **What to do**:
  - Fix `signConfig()` in `bridge/pkg/qr/public.go:471-482` to include `TLSTrustHint` and `CertExpiresAt` in the v2 HMAC signature
  - Update `ValidateConfig()` to verify the complete v2 signature including the new fields
  - Add test: tamper with `TLSTrustHint` in a signed v2 config → verify `ValidateConfig()` rejects it

  **Must NOT do**:
  - Do NOT change the v1 signature format (v1 has no TLS fields)
  - Do NOT break existing v2 signed configs (this is a security fix, existing configs will need re-signing)

  **Recommended Agent Profile**:
  - **Category**: `quick`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 1 (with Tasks 1-3, 5-6)
  - **Blocks**: None
  - **Blocked By**: None

  **References**:
  - `bridge/pkg/qr/public.go:471-482` — `signConfig()` for v2, omits TLSTrustHint and CertExpiresAt
  - `bridge/pkg/qr/public.go:385-410` — `ValidateConfig()` must also be updated
  - `bridge/pkg/qr/public.go:293-296` — ConfigPayload TLS field definitions

  **Acceptance Criteria**:
  - [ ] `signConfig()` v2 includes `TLSTrustHint` and `CertExpiresAt` in HMAC input
  - [ ] `ValidateConfig()` verifies complete v2 signature
  - [ ] Test: tampered TLSTrustHint → ValidateConfig returns error
  - [ ] Test: tampered CertExpiresAt → ValidateConfig returns error
  - [ ] Test: valid v2 config → ValidateConfig returns nil
  - [ ] `go test ./pkg/qr/... -run TestSignConfig` → PASS

  **QA Scenarios**:
  ```
  Scenario: HMAC tamper detection for TLSTrustHint
    Tool: Bash
    Preconditions: v2 QR config generated with valid signature
    Steps:
      1. Generate v2 config: `curl http://localhost:8443/qr/config` with ARMORCLAW_QR_VERSION=2
      2. Base64-decode payload, modify tls_trust_hint value
      3. Base64-encode modified payload
      4. Call ValidateConfig with modified payload
    Expected Result: ValidateConfig returns signature mismatch error
    Failure Indicators: ValidateConfig accepts tampered payload
    Evidence: .sisyphus/evidence/task-4-hmac-tamper.txt

  Scenario: v1 signature format unchanged
    Tool: Bash
    Preconditions: v1 QR config (ARMORCLAW_QR_VERSION unset or "1")
    Steps:
      1. Generate v1 config
      2. Verify signature format is `Version:MatrixHomeserver:RpcURL:...` (no TLS fields)
      3. ValidateConfig accepts unmodified v1 config
    Expected Result: v1 configs validate correctly with unchanged format
    Failure Indicators: v1 config validation fails
    Evidence: .sisyphus/evidence/task-4-v1-unchanged.txt
  ```

  **Commit**: YES
  - Message: `fix(tls): include all v2 fields in QR config HMAC signature`
  - Files: `bridge/pkg/qr/public.go`
  - Pre-commit: `go test ./pkg/qr/...`

- [ ] 5. Fix TLS Well-Known Endpoint Enrichment

  **What to do**:
  - Update `handleWellKnown()` in `bridge/pkg/http/server.go:689-719` to include TLS metadata fields beyond just `tls_mode`
  - Add `tls_fingerprint_sha256`, `tls_trust_hint`, `cert_expires_at` to the `com.armorclaw` section
  - Match the field naming convention used in ConfigPayload (not TLSInfo struct)
  - Add explicit native-mode handling: return zero-value TLS fields when mode is "none"

  **Must NOT do**:
  - Do NOT change the `m.homeserver` or `m.identity_server` sections
  - Do NOT change the TLSInfo struct field names (naming alignment is a separate concern)

  **Recommended Agent Profile**:
  - **Category**: `quick`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 1 (with Tasks 1-4, 6)
  - **Blocks**: None
  - **Blocked By**: None

  **References**:
  - `bridge/pkg/http/server.go:689-719` — `handleWellKnown()`, only includes `tls_mode`
  - `bridge/pkg/http/server.go:294-302` — TLSInfo struct with full fields
  - `bridge/pkg/http/server.go:622-657` — `/discover` endpoint includes full TLSInfo (reference for fields)
  - `bridge/pkg/qr/public.go:293-296` — ConfigPayload TLS field names (tls_trust_hint vs trust_type)

  **Acceptance Criteria**:
  - [ ] `/.well-known/matrix/client` response includes `tls_mode`, `tls_fingerprint_sha256`, `tls_trust_hint`, `cert_expires_at`
  - [ ] Native mode returns zero-value TLS fields (not omitted)
  - [ ] `go test ./pkg/http/... -run TestWellKnown` → PASS

  **QA Scenarios**:
  ```
  Scenario: Well-known includes full TLS metadata in sentinel mode
    Tool: Bash
    Preconditions: Bridge running in sentinel mode with TLS cert
    Steps:
      1. `curl http://localhost:8443/.well-known/matrix/client`
      2. Parse JSON, check com.armorclaw section
    Expected Result: Response contains tls_mode, tls_fingerprint_sha256, tls_trust_hint, cert_expires_at
    Failure Indicators: Only tls_mode present, missing other fields
    Evidence: .sisyphus/evidence/task-5-wellknown-sentinel.txt

  Scenario: Well-known includes zero-value TLS in native mode
    Tool: Bash
    Preconditions: Bridge running in native mode (no TLS)
    Steps:
      1. `curl http://localhost:8443/.well-known/matrix/client`
      2. Parse JSON, check com.armorclaw section
    Expected Result: tls_mode="none" or "", fingerprint="", trust_hint="", expires_at=0
    Failure Indicators: TLS fields completely absent (should be present with zero values)
    Evidence: .sisyphus/evidence/task-5-wellknown-native.txt
  ```

  **Commit**: YES
  - Message: `fix(tls): enrich well-known endpoint with full TLS metadata`
  - Files: `bridge/pkg/http/server.go`
  - Pre-commit: `go test ./pkg/http/...`

- [x] 6. Azure Blob rustls Migration

  **What to do**:
  - Rename `sidecar/src/connectors/azure_blob.rs.disabled` → `azure_blob.rs`
  - Identify which Azure SDK crate is used and its TLS feature flags
  - Add `rustls-tls` feature to `reqwest` dependency (NOT just adding `rustls` crate — reqwest controls TLS backend)
  - Disable default features on the Azure SDK crate that pull in `native-tls`: `default-features = false`
  - Update `AzureBlobConnector::new()` to use rustls TLS configuration
  - Ensure the connector builds without OpenSSL system dependency
  - Add to module registration in `connectors/mod.rs` (remove cfg gate or add rustls feature gate)

  **Must NOT do**:
  - Do NOT touch `aws_s3.rs` or SharePoint connector
  - Do NOT change the `CloudConnector` trait interface
  - Do NOT use `native-tls` or OpenSSL — rustls only

  **Recommended Agent Profile**:
  - **Category**: `unspecified-high`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 1 (with Tasks 1-5)
  - **Blocks**: None
  - **Blocked By**: None

  **References**:
  - `sidecar/src/connectors/azure_blob.rs.disabled` — 128-line Azure Blob connector using azure_storage crate
  - `sidecar/src/connectors/mod.rs` — Module registration, check for cfg gate
  - `sidecar/Cargo.toml` — Dependencies, add rustls + webpki-roots
  - `sidecar/README.md` — Documents the disabled state and rustls migration path
  - `sidecar/src/connectors/aws_s3.rs` — DO NOT TOUCH — S3 connector reference for pattern matching

  **Acceptance Criteria**:
  - [ ] `azure_blob.rs.disabled` renamed to `azure_blob.rs`
  - [ ] `cargo build` succeeds without OpenSSL system dependency
  - [ ] `cargo test --lib azure_blob` → PASS
  - [ ] `cargo test --lib` → ALL tests pass (252+ tests, including S3)
  - [ ] No `native-tls` or `openssl` in Cargo.lock for sidecar (verify with `cargo tree`)
  - [ ] `cargo tree -i native-tls` returns no results

  **QA Scenarios**:
  ```
  Scenario: Azure Blob connector builds with rustls
    Tool: Bash
    Preconditions: sidecar/ directory, Rust toolchain installed
    Steps:
      1. `cd sidecar && cargo build 2>&1 | tee /tmp/azure-build.log`
      2. Check for OpenSSL errors: `grep -i openssl /tmp/azure-build.log`
      3. Check for native-tls: `grep -i native.tls /tmp/azure-build.log`
    Expected Result: Build succeeds, no OpenSSL or native-tls references
    Failure Indicators: Build fails with OpenSSL linker errors, native-tls in dependency tree
    Evidence: .sisyphus/evidence/task-6-azure-build.txt

  Scenario: S3 connector unaffected by Azure Blob changes
    Tool: Bash
    Preconditions: Azure Blob connector enabled
    Steps:
      1. `cd sidecar && cargo test --lib aws_s3 2>&1`
      2. Count passing tests
    Expected Result: All S3 tests pass unchanged
    Failure Indicators: Any S3 test failure
    Evidence: .sisyphus/evidence/task-6-s3-unaffected.txt
  ```

  **Commit**: YES
  - Message: `feat(sidecar): migrate Azure Blob connector from native-tls to rustls`
  - Files: `sidecar/src/connectors/azure_blob.rs`, `sidecar/Cargo.toml`, `sidecar/src/connectors/mod.rs`
  - Pre-commit: `cargo test --lib`

- [x] 7. Add ToDevice to SyncResponse Types + m.room.encrypted Filter

  **What to do**:
  - Add `ToDevice` struct and field to `SyncResponse` in BOTH files:
    - `bridge/internal/adapter/matrix.go:127-136`
    - `bridge/pkg/matrix/client.go:288-291`
  - Add `ToDevice` section to the sync filter in `bridge/internal/adapter/matrix.go:76-107`
  - Add `DeviceLists` and `DeviceOneTimeKeysCount` fields to SyncResponse
  - **Add `m.room.encrypted` to the sync filter timeline types** (currently excluded — E2EE messages silently dropped without this)
  - Process `to_device` events in the sync loop — route crypto events to key_ingestion handlers

  **Must NOT do**:
  - Do NOT replace the custom sync loop with mautrix-go's built-in sync
  - Do NOT modify existing event processing (m.room.message, m.room.member, etc.)
  - Do NOT depend on Task 1 (Conduit validation) — struct/filter changes proceed regardless
  - Do NOT handle device ID persistence (Task 7.5) or Conduit test env (Task 7.5)

  **Recommended Agent Profile**:
  - **Category**: `quick`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 1 (with Tasks 1-6)
  - **Blocks**: Tasks 8, 9, 13
  - **Blocked By**: None (removed dependency on Task 1 — struct changes don't need Conduit validation)

  **References**:
  - `bridge/internal/adapter/matrix.go:76-107` — Sync filter, add `{"to_device":{"limit":100}}` AND add `m.room.encrypted` to timeline types
  - `bridge/internal/adapter/matrix.go:80-86` — Current timeline filter explicitly excludes `m.room.encrypted` — this MUST be fixed here
  - `bridge/internal/adapter/matrix.go:127-136` — SyncResponse struct, add `ToDevice` field
  - `bridge/pkg/matrix/client.go:288-291` — Secondary SyncResponse, must also be updated
  - `bridge/internal/adapter/key_ingestion.go:167-178` — Skeleton for crypto event handling (wire into this)
  - `bridge/internal/adapter/matrix.go` — Check login flow for device_id parameter handling

  **Acceptance Criteria**:
  - [ ] Both SyncResponse types have `ToDevice`, `DeviceLists`, `DeviceOneTimeKeysCount` fields
  - [ ] Sync filter includes `to_device` section with limit
  - [ ] Sync filter includes `m.room.encrypted` in timeline types
  - [ ] `to_device` events are routed to `KeyIngestionManager` for processing
  - [ ] Existing sync tests pass: `go test ./internal/adapter/... -run TestSync`

  **QA Scenarios**:
  ```
  Scenario: ToDevice events received from Conduit sync
    Tool: Bash
    Preconditions: Bridge running with updated SyncResponse, Conduit running
    Steps:
      1. Bridge logs in to Conduit and starts sync loop
      2. Send a to_device event to the bridge user from another client
      3. Check bridge logs for "received to_device event" or similar
    Expected Result: Bridge processes to_device events without error
    Failure Indicators: JSON unmarshal error, to_device field ignored
    Evidence: .sisyphus/evidence/task-7-todevice-sync.txt

  Scenario: Existing sync behavior unchanged
    Tool: Bash
    Preconditions: Bridge running with updated SyncResponse
    Steps:
      1. Send m.room.message to a room the bridge is in
      2. Verify bridge receives and processes the message
      3. `go test ./internal/adapter/... -run TestSync`
    Expected Result: All existing sync tests pass, messages received normally
    Failure Indicators: Any existing test failure, messages not received
    Evidence: .sisyphus/evidence/task-7-sync-regression.txt
  ```

  **Commit**: YES (groups with 8)
  - Message: `feat(e2ee): add ToDevice sync support to SyncResponse`
  - Files: `bridge/internal/adapter/matrix.go`, `bridge/pkg/matrix/client.go`

- [ ] 8. Add E2EE Config Kill Switch + Runtime Toggle

  **What to do**:
  - Add `matrix.e2ee.enabled` config field to `bridge/pkg/config/config.go` (default: `false`)
  - Add `ARMORCLAW_E2EE_ENABLED` env var binding
  - Gate all E2EE operations behind this flag: encrypt, decrypt, key upload, SAS verification
  - When disabled, Bridge operates in plaintext-only mode (current behavior)
  - When enabled and crypto init fails, log warning and fall back to plaintext
  - **Add runtime toggle**: `bridge.e2ee_disable` emergency RPC method to disable E2EE without restart
  - **Add runtime toggle**: `bridge.e2ee_enable` RPC method to re-enable after fix
  - **Security**: Both toggle methods require admin-level authorization (check caller is bridge admin user)
  - **Audit**: Every toggle event is logged to audit.db with timestamp, caller, action, previous state

  **Must NOT do**:
  - Do NOT default to `true` — must be opt-in
  - Do NOT prevent Bridge from starting if E2EE init fails — graceful degradation

  **Recommended Agent Profile**:
  - **Category**: `quick`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 2 (with Tasks 7, 9-12)
  - **Blocks**: Task 13 (OlmMachine needs config check)
  - **Blocked By**: Task 7 (needs ToDevice in place)

  **References**:
  - `bridge/pkg/config/config.go:147-168` — Config struct with Mode field (follow same pattern)
  - `bridge/pkg/config/loader.go:92` — Env var override (add ARMORCLAW_E2EE_ENABLED)
  - `bridge/internal/adapter/matrix.go` — SendMessage/ProcessEvent will check config before E2EE ops

  **Acceptance Criteria**:
  - [ ] `config.Matrix.E2EE.Enabled` field exists with `toml:"enabled" env:"ARMORCLAW_E2EE_ENABLED"`
  - [ ] Default is `false`
  - [ ] When `false`: no crypto operations attempted, all messages plaintext
  - [ ] When `true` + crypto init fails: log warning, continue in plaintext mode
  - [ ] Runtime toggle methods require admin-level authorization
  - [ ] Every toggle event is audit-logged (timestamp, caller, action, previous state)
  - [ ] `go test ./pkg/config/... -run TestE2EEConfig` → PASS

  **QA Scenarios**:
  ```
  Scenario: E2EE disabled - Bridge starts normally without crypto
    Tool: Bash
    Preconditions: ARMORCLAW_E2EE_ENABLED unset or "false"
    Steps:
      1. Start bridge without E2EE config
      2. Send and receive plaintext messages
      3. Check logs: no crypto-related errors
    Expected Result: Bridge operates identically to pre-E2EE version
    Failure Indicators: Crypto initialization attempted, errors about missing keys
    Evidence: .sisyphus/evidence/task-8-e2ee-disabled.txt

  Scenario: E2EE enabled with graceful degradation
    Tool: Bash
    Preconditions: ARMORCLAW_E2EE_ENABLED=true, no crypto state initialized
    Steps:
      1. Start bridge with E2EE enabled but no prior crypto state
      2. Verify log contains "E2EE initialization failed, falling back to plaintext"
      3. Send/receive plaintext messages
    Expected Result: Bridge starts and works despite E2EE init failure
    Failure Indicators: Bridge exits, panic, messages not sent
    Evidence: .sisyphus/evidence/task-8-e2ee-degradation.txt
  ```

  **Commit**: YES (groups with 7)
  - Message: `feat(e2ee): add config kill switch for E2EE operations`
  - Files: `bridge/pkg/config/config.go`, `bridge/pkg/config/loader.go`

- [ ] 7.5. Device ID Persistence + Conduit Test Environment

  **What to do**:
  - **Verify Bridge persists device ID across restarts**: check login flow for device_id parameter
  - If device ID is not persisted: add device ID to SQLCipher keystore, include in login request
  - Create `bridge/test/conduit/` with docker-compose for repeatable Conduit E2EE test environment
    - Include E2EE-enabled Conduit config (`conduit.toml`)
    - Test user creation script
    - Health check endpoint

  **Must NOT do**:
  - Do NOT modify the sync loop or SyncResponse (Task 7's job)
  - Do NOT create a separate database for device IDs — use existing SQLCipher keystore

  **Recommended Agent Profile**:
  - **Category**: `quick`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES (with Task 8)
  - **Parallel Group**: Wave 2
  - **Blocks**: Task 13 (OlmMachine needs stable device ID)
  - **Blocked By**: Task 7 (needs ToDevice infrastructure)

  **References**:
  - `bridge/internal/adapter/matrix.go` — Check login flow for device_id parameter handling
  - `bridge/pkg/crypto/keystore_store.go` — SQLCipher keystore (store device ID here)

  **Acceptance Criteria**:
  - [ ] Bridge uses consistent device ID across restarts (verified by checking keystore + login params)
  - [ ] `bridge/test/conduit/docker-compose.yml` exists with E2EE-enabled Conduit config
  - [ ] `docker compose -f bridge/test/conduit/docker-compose.yml up -d` → Conduit healthy

  **QA Scenarios**:
  ```
  Scenario: Device ID persistence across restarts
    Tool: Bash
    Steps:
      1. Start bridge, note device_id from Conduit device list
      2. Stop bridge, restart bridge
      3. Check Conduit device list again
    Expected Result: Same device_id after restart (not a new device)
    Failure Indicators: New device_id generated, old device orphaned
    Evidence: .sisyphus/evidence/task-7.5-device-id-persistence.txt

  Scenario: Conduit test environment starts
    Tool: Bash
    Steps:
      1. `docker compose -f bridge/test/conduit/docker-compose.yml up -d`
      2. `curl http://localhost:6167/_matrix/client/versions`
    Expected Result: Conduit responds with valid Matrix client versions
    Failure Indicators: Container not starting, health check fails
    Evidence: .sisyphus/evidence/task-7.5-conduit-env.txt
  ```

  **Commit**: YES
  - Message: `feat(e2ee): add device ID persistence and Conduit test environment`
  - Files: `bridge/internal/adapter/matrix.go`, `bridge/test/conduit/docker-compose.yml`

- [ ] 9. Expand crypto.Store for mautrix-go Integration (Core Methods)

  **What to do**:
  - Extend `bridge/pkg/crypto/store.go` interface with core mautrix-go `crypto.Store` methods:
    - Olm account management: `PutOlmAccount`, `GetOlmAccount` (2-3 methods)
    - Inbound group sessions: `UpdateInboundGroupSession`, `GetGroupSessionsForRoom` (2-3 methods — existing Add/Get remain)
    - Outbound group sessions: `PutOutboundGroupSession`, `GetOutboundGroupSession` (2 methods)
    - Device key storage: `PutDeviceKeys`, `GetDeviceKeys`, `PutCrossSigningKey` (3-4 methods)
    - Session management: `PutSession`, `GetSession` (2 methods)
    - **Total: ~15 methods in this phase** (cross-signing/verification methods deferred to Task 9.5)
  - Update `bridge/pkg/crypto/keystore_store.go` SQLCipher implementation with new tables:
    - `olm_accounts` (device_id, account_pickle, shared)
    - `outbound_group_sessions` (room_id, session_id, session_pickle)
    - `device_keys` (user_id, device_id, key_data)
  - **Add schema versioning**: `schema_version` table tracking migration level
  - **Write down migrations** (and rollback/down migrations for each table)
  - Run schema migration on existing SQLCipher keystore
  - Write comprehensive store tests

  **Must NOT do**:
  - Do NOT create a separate database — extend existing SQLCipher keystore
  - Do NOT change the existing `AddInboundGroupSession`/`GetInboundGroupSession` methods
  - Do NOT implement cross-signing key or verification methods yet (Task 9.5)

  **Recommended Agent Profile**:
  - **Category**: `deep`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES (with Tasks 8, 10-12)
  - **Parallel Group**: Wave 2
  - **Blocks**: Task 13 (OlmMachine needs core store)
  - **Blocked By**: Task 7 (needs ToDevice in SyncResponse)

  **References**:
  - `bridge/pkg/crypto/store.go` — Current 3-method interface (Add/Get/Has InboundGroupSession)
  - `bridge/pkg/crypto/keystore_store.go` — SQLCipher implementation, add tables here
  - `bridge/pkg/keystore/` — Existing SQLCipher + XChaCha20-Poly1305 encrypted credential storage (reference pattern)
  - mautrix-go `crypto.Store` interface — target interface to match (approx 20 methods)

  **Acceptance Criteria**:
  - [ ] `crypto.Store` interface has all methods required by mautrix-go `crypto.Store`
  - [ ] `KeystoreBackedStore` implements all new methods with SQLCipher persistence
  - [ ] Schema migration runs on existing keystore without data loss
  - [ ] `go test ./pkg/crypto/... -run TestStore` → PASS (all CRUD operations)
  - [ ] `go test ./pkg/crypto/... -run TestMigration` → PASS (existing data preserved)
  - [ ] **Interface designed for extension**: all new methods in Task 9.5 will be additive (no existing method signatures changed). Document this constraint as a code comment on the Store interface.

  **QA Scenarios**:
  ```
  Scenario: Crypto store CRUD for Olm account
    Tool: Bash
    Steps:
      1. `go test ./pkg/crypto/... -run TestOlmAccountCRUD -v`
      2. Verify: PutOlmAccount → GetOlmAccount round-trip
    Expected Result: Account pickle stored and retrieved without corruption
    Failure Indicators: Data corruption, SQL error, pickle decode failure
    Evidence: .sisyphus/evidence/task-9-store-olm.txt

  Scenario: Schema migration preserves existing inbound sessions
    Tool: Bash
    Steps:
      1. Create keystore with existing inbound_group_sessions data
      2. Run migration
      3. Query inbound_group_sessions → verify data intact
      4. Query new tables → verify they exist with correct schema
    Expected Result: All existing data preserved, new tables created
    Failure Indicators: Data loss, migration error, table not found
    Evidence: .sisyphus/evidence/task-9-store-migration.txt
  ```

  **Commit**: YES
  - Message: `feat(e2ee): expand crypto store for mautrix-go integration`
  - Files: `bridge/pkg/crypto/store.go`, `bridge/pkg/crypto/keystore_store.go`
  - Pre-commit: `go test ./pkg/crypto/...`

- [ ] 10. Add Jetski CDP Event Subscriber to Bridge

  **What to do**:
  - Create `JetskiStateEventSubscriber` in `bridge/pkg/agent/` (or `bridge/pkg/browser/`)
  - Subscribe to Jetski RPC event stream on Bridge startup
  - Parse incoming CDP events, map to `CDPEvent` struct expected by `InferAgentState()`
  - **Send registration handshake** on connection: `{"type":"register","payload":{"device_id":"bridge"}}` — Jetski requires this before event delivery
  - Match events to active agent by correlating Jetski session ID with agent ID
  - Call `InferAgentState()` with parsed CDP events for each active agent

  **Must NOT do**:
  - Do NOT modify `InferAgentState()` or `ApplyInferredState()` — they are complete
  - Do NOT modify `StateMachine.Transition()` — use `ForceTransition` via `ApplyInferredState`
  - Do NOT create new event types — use existing `com.armorclaw.agent.status`

  **Recommended Agent Profile**:
  - **Category**: `unspecified-high`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES (with Tasks 7-9, 11-12)
  - **Parallel Group**: Wave 2
  - **Blocks**: Tasks 11, 12, 18, 19
  - **Blocked By**: Task 2 (Jetski CDP event access validation)

  **References**:
  - `bridge/pkg/agent/state_inference.go:30-45` — Expected CDPEvent types and structure
  - `bridge/pkg/agent/state_inference.go:138-146` — `ApplyInferredState()` wires to ForceTransition
  - `bridge/pkg/agent/state_machine.go:195` — `ForceTransition()` for bypassing validation
  - `bridge/pkg/agent/integration.go:279` — AgentCoordinator manages multiple agents
  - `jetski/internal/rpc/rpc.go` — Jetski RPC API (subscribe to events here)
  - Task 2 output: `.sisyphus/evidence/task-2-jetski-cdp-validation.md` — How to access CDP events

  **Acceptance Criteria**:
  - [ ] `JetskiStateEventSubscriber` struct created in `bridge/pkg/agent/`
  - [ ] Subscribes to Jetski RPC event stream on Bridge startup
  - [ ] Parses CDP events into `CDPEvent` structs
  - [ ] Calls `InferAgentState()` per agent with incoming events
  - [ ] `go test ./pkg/agent/... -run TestJetskiSubscriber` → PASS

  **QA Scenarios**:
  ```
  Scenario: CDP Page.frameNavigated triggers BROWSING state inference
    Tool: Bash
    Steps:
      1. Start bridge with Jetski subscriber
      2. Simulate CDP event: {"method":"Page.frameNavigated","params":{"url":"https://example.com"}}
      3. Check agent state via StateMachine.Current()
    Expected Result: Agent status transitions to BROWSING
    Failure Indicators: State remains IDLE, event not processed
    Evidence: .sisyphus/evidence/task-10-cdp-browsing.txt

  Scenario: CDP DOM.focus on INPUT triggers FORM_FILLING state
    Tool: Bash
    Steps:
      1. Simulate CDP event: {"method":"DOM.focus","params":{"nodeId":42,"nodeName":"INPUT"}}
      2. Check agent state
    Expected Result: Agent status transitions to FORM_FILLING
    Failure Indicators: State unchanged, method not recognized
    Evidence: .sisyphus/evidence/task-10-cdp-form.txt
  ```

  **Commit**: YES (groups with 11)
  - Message: `feat(agent): add Jetski CDP event subscriber for state inference`
  - Files: `bridge/pkg/agent/jetski_subscriber.go`, `bridge/pkg/agent/jetski_subscriber_test.go`

- [ ] 11. Wire CDP Events → InferAgentState → StateMachine

  **What to do**:
  - Connect `JetskiStateEventSubscriber` (Task 10) to the existing `AgentCoordinator`
  - On each CDP event batch, call `InferAgentState()` to compute new status
  - Call `ApplyInferredState()` on the matched agent's StateMachine
  - Verify `BroadcastStatus()` fires `com.armorclaw.agent.status` Matrix events after each transition
  - Handle priority rules: workflow side-channel > exit > approval-lock > CDP

  **Must NOT do**:
  - Do NOT modify existing HITL approval flow (AWAITING_APPROVAL is protected by priority rules)
  - Do NOT create new event types
  - Do NOT modify `BroadcastStatus()` — it already works

  **Recommended Agent Profile**:
  - **Category**: `unspecified-high`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES (with Tasks 7-9, 12)
  - **Parallel Group**: Wave 2
  - **Blocks**: Tasks 12, 18, 19, 22
  - **Blocked By**: Task 10 (Jetski subscriber)

  **References**:
  - `bridge/pkg/agent/state_inference.go` — Full inference engine with priority rules
  - `bridge/pkg/agent/integration.go:359` — `BroadcastStatus()` already publishes events
  - `bridge/pkg/agent/integration.go:279` — AgentCoordinator manages agent StateMachines
  - `bridge/pkg/agent/state.go:175` — `StatusEvent.EventType()` returns "com.armorclaw.agent.status"

  **Acceptance Criteria**:
  - [ ] CDP events flow through: subscriber → InferAgentState → ApplyInferredState → StateMachine
  - [ ] `com.armorclaw.agent.status` event appears in Matrix room after state transition
  - [ ] AWAITING_APPROVAL state is NOT overridden by CDP events (priority rule)
  - [ ] `go test ./pkg/agent/... -run TestCDPInference` → PASS

  **QA Scenarios**:
  ```
  Scenario: State transition broadcasts to Matrix
    Tool: Bash
    Preconditions: Agent active with IDLE state, EventBus connected
    Steps:
      1. Simulate CDP Page.frameNavigated event
      2. Wait 2 seconds
      3. Check Matrix room for com.armorclaw.agent.status event with status="BROWSING"
    Expected Result: Matrix room contains event with matching status
    Failure Indicators: No event in Matrix room, status mismatch
    Evidence: .sisyphus/evidence/task-11-matrix-broadcast.txt

  Scenario: AWAITING_APPROVAL protected from CDP override
    Tool: Bash
    Preconditions: Agent in AWAITING_APPROVAL state (HITL approval pending)
    Steps:
      1. Simulate CDP Page.frameNavigated event
      2. Check agent state
    Expected Result: Agent remains AWAITING_APPROVAL (priority rule)
    Failure Indicators: State changes to BROWSING
    Evidence: .sisyphus/evidence/task-11-hitl-protected.txt
  ```

  **Commit**: YES (groups with 10)
  - Message: `feat(agent): wire CDP events to state inference engine with Matrix broadcasting`
  - Files: `bridge/pkg/agent/jetski_subscriber.go`, `bridge/pkg/agent/integration.go`

- [ ] 12. Wire Workflow Side-Channel Signals

  **What to do**:
  - Add `EmitSideChannelSignal(agentID, signal)` to the orchestrator integration layer
  - Wire captcha detection in browser job processor → `captcha` signal
  - Wire 2FA detection (after `WaitFor2FA` browser call) → `twofa` signal
  - Wire payment detection (after payment form submission) → `payment` signal
  - Wire container loss detection → `offline` signal
  - Route signals to `InferAgentState()` as priority-1 (override CDP events)

  **Must NOT do**:
  - Do NOT create new Matrix event types for these signals
  - Do NOT modify the state machine transition graph

  **Recommended Agent Profile**:
  - **Category**: `unspecified-high`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES (with Tasks 7-9)
  - **Parallel Group**: Wave 2
  - **Blocks**: Task 22
  - **Blocked By**: Task 11 (needs inference wiring)

  **References**:
  - `bridge/pkg/agent/state_inference.go:47-65` — Priority-1 workflow side-channel handling
  - `bridge/pkg/agent/state_inference.go:75-95` — WorkflowStatus struct with Captcha/Twofa/Payment/Offline signals
  - `bridge/pkg/secretary/orchestrator_integration.go` — Step execution, wire signals here
  - `bridge/pkg/browser/processor.go` — Browser lifecycle, container loss detection (confirmed location)

  **Acceptance Criteria**:
  - [ ] `EmitSideChannelSignal()` function exists and routes to `InferAgentState()`
  - [ ] Captcha signal → agent transitions to AWAITING_CAPTCHA
  - [ ] 2FA signal → agent transitions to AWAITING_2FA
  - [ ] Payment signal → agent transitions to PROCESSING_PAYMENT
  - [ ] Offline signal → agent transitions to OFFLINE
  - [ ] Workflow signals override CDP inference (priority-1 > priority-4)
  - [ ] `go test ./pkg/agent/... -run TestSideChannel` → PASS

  **QA Scenarios**:
  ```
  Scenario: Captcha signal overrides CDP BROWSING state
    Tool: Bash
    Preconditions: Agent in BROWSING state (from CDP inference)
    Steps:
      1. Emit captcha side-channel signal for agent
      2. Check agent state
    Expected Result: Agent transitions to AWAITING_CAPTCHA (priority-1 overrides priority-4)
    Failure Indicators: State remains BROWSING
    Evidence: .sisyphus/evidence/task-12-captcha-override.txt

  Scenario: Container loss triggers OFFLINE signal
    Tool: Bash
    Preconditions: Agent in BROWSING state
    Steps:
      1. Simulate container exit (loss detection)
      2. Check agent state
    Expected Result: Agent transitions to OFFLINE
    Failure Indicators: State unchanged, no broadcast
    Evidence: .sisyphus/evidence/task-12-offline-signal.txt
  ```

  **Commit**: YES
  - Message: `feat(agent): wire workflow side-channel signals to state inference`
  - Files: `bridge/pkg/agent/jetski_subscriber.go`, `bridge/pkg/secretary/orchestrator_integration.go`

- [ ] 13. Wire mautrix-go OlmMachine + SyncResponseAdapter into Bridge

  **What to do**:
  - Create `CryptoEngine` wrapper in `bridge/pkg/crypto/` that initializes mautrix-go `OlmMachine`
  - Wire `OlmMachine` to use the expanded `crypto.Store` (Task 9)
  - **Create `SyncResponseAdapter`** that converts Bridge's custom `SyncResponse` → `mautrix.RespSync`:
    - Map `ToDevice` field to mautrix format
    - Map `DeviceLists` changes and `DeviceOneTimeKeysCount`
    - Handle OTK count tracking and proactive key generation
    - Manage OlmMachine's internal state transitions during sync processing
  - Initialize Olm account on first startup (or load from store)
  - Upload device keys and one-time keys to Conduit via `/keys/upload`
  - Handle `/keys/query` responses to populate known device list
  - Handle `/keys/claim` for establishing 1:1 Olm sessions
  - Gate everything behind `matrix.e2ee.enabled` (Task 8)
  - **NOTE**: goolm (pure Go Olm) is less battle-tested than libolm. Add build tag choice:
    - Default: `-tags goolm` (pure Go, no CGO beyond SQLCipher)
    - Alternative: `-tags libolm` (CGO, production-proven)
    - Add crypto compatibility test: verify goolm output interoperates with libolm clients

  **Must NOT do**:
  - Do NOT replace Bridge's custom sync loop — feed to_device events from our sync to OlmMachine via SyncResponseAdapter
  - Do NOT start encrypting messages yet (Task 14)
  - Do NOT use CGO libolm by default — use pure Go goolm backend (but provide build tag choice)

  **Recommended Agent Profile**:
  - **Category**: `deep`
  - **Skills**: []
  - **Note**: This task is heavier than typical `deep` due to the SyncResponseAdapter mapping layer. Allocate extra time.

  **Parallelization**:
  - **Can Run In Parallel**: NO (sequential, first task in Wave 3a — others depend on it)
  - **Parallel Group**: Wave 3a
  - **Blocks**: Tasks 14, 15
  - **Blocked By**: Tasks 1 (Conduit validation), 8 (kill switch), 9 (core store)

  **References**:
  - `bridge/pkg/crypto/store.go` — Expanded store interface (from Task 9)
  - `bridge/pkg/crypto/keystore_store.go` — SQLCipher implementation
  - `bridge/doc/HANDOFF_E2EE.md` — E2EE handoff document
  - `go.mod` — mautrix-go v0.26.4 already listed

  **Acceptance Criteria**:
  - [ ] `CryptoEngine` struct created with `OlmMachine` initialization
  - [ ] Olm account created and device keys uploaded to Conduit
  - [ ] to_device events from sync fed to OlmMachine for session creation
  - [ ] `go test ./pkg/crypto/... -run TestCryptoEngine` → PASS
  - [ ] When disabled: CryptoEngine is nil, no operations attempted
  - [ ] **goolm interop verified**: encrypt with goolm via Bridge, decrypt with reference libolm client (or verify against Matrix spec test vectors). Document results. If interop fails, pivot to `-tags libolm` before proceeding to Task 14.

  **QA Scenarios**:
  ```
  Scenario: OlmMachine initialization and key upload
    Tool: Bash
    Preconditions: Conduit running, ARMORCLAW_E2EE_ENABLED=true
    Steps:
      1. Start bridge with E2EE enabled
      2. Query device keys via Conduit API
    Expected Result: Bridge device keys present in Conduit
    Failure Indicators: No keys uploaded, crypto init error
    Evidence: .sisyphus/evidence/task-13-olm-init.txt

  Scenario: CryptoEngine nil when E2EE disabled
    Tool: Bash
    Preconditions: ARMORCLAW_E2EE_ENABLED=false
    Steps:
      1. Start bridge without E2EE
      2. Send/receive plaintext messages
    Expected Result: No crypto operations, messages work normally
    Evidence: .sisyphus/evidence/task-13-e2ee-disabled.txt

  Scenario: goolm/libolm cryptographic interop
    Tool: Bash
    Preconditions: Bridge built with `-tags goolm`, two Olm accounts initialized
    Steps:
      1. `go test ./pkg/crypto/... -run TestGoolmInterop -v`
      2. Test encrypts with goolm account, decrypts with a second Olm account initialized via libolm-compatible API
      3. If libolm not available: verify against known Megolm test vectors from Matrix spec (https://spec.matrix.org/v1.11/rooms/v11/#end-to-end-encryption)
      4. Document results in evidence file
    Expected Result: Message encrypted with goolm decryptable by libolm client. Test vectors pass.
    Failure Indicators: Decryption fails, garbled output, session mismatch
    Evidence: .sisyphus/evidence/task-13-goolm-interop.txt
    BLOCKING: If this test fails, rebuild with `-tags libolm` and re-test before proceeding to Task 14.
  ```

  **Commit**: YES
  - Message: `feat(e2ee): wire mautrix-go OlmMachine into Bridge with config gate`
  - Files: `bridge/pkg/crypto/engine.go`, `bridge/pkg/crypto/engine_test.go`
  - Pre-commit: `go test ./pkg/crypto/...`

- [ ] 14. Add E2EE Encrypt/Decrypt + RoomEncryptionCache to Message Pipeline

  **What to do**:
  - **Create `RoomEncryptionCache`** in `bridge/pkg/crypto/` that tracks which rooms are encrypted:
    - Check `m.room.encryption` state event on room join
    - Update from sync when `m.room.encryption` state events arrive
    - Persist across restarts (in-memory with sync refresh)
  - Wrap `SendMessage()` to check `RoomEncryptionCache` for room status
  - Encrypted rooms: encrypt via `CryptoEngine.Encrypt()` → send as `m.room.encrypted`
  - Unencrypted rooms: send as `m.room.message` (plaintext, unchanged)
  - In `processEvents()`: handle `m.room.encrypted` → decrypt → process plaintext
  - **Decryption failure handling (NOT "log and skip")**:
    - Emit placeholder message to room: `🔒 Encrypted message — decryption failed`
    - Log failure with `event_id` (NOT content) at WARN level
    - Queue failed event_id for retry when new key material arrives via `to_device`
    - Track retry count — after 3 retries, mark as permanently failed and stop retrying
  - **Handle Megolm session rotation**: when session expires or reaches message limit, create new session and share

  **Must NOT do**:
  - Do NOT store decrypted message content — decrypt in-flight only
  - Do NOT modify the agent message processing pipeline
  - Do NOT break plaintext messaging

  **Recommended Agent Profile**:
  - **Category**: `deep`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: NO (sequential after T13 — both modify matrix.go)
  - **Parallel Group**: Wave 3
  - **Blocks**: Task 21
  - **Blocked By**: Task 13 (needs CryptoEngine)

  **References**:
  - `bridge/internal/adapter/matrix.go:357` — SendMessage sends plaintext
  - `bridge/internal/adapter/matrix.go:572+` — processEvents, add m.room.encrypted
  - `bridge/internal/adapter/matrix.go:80-86` — Sync filter, add m.room.encrypted

  **Acceptance Criteria**:
  - [ ] Encrypted room: `SendMessage()` produces `m.room.encrypted`
  - [ ] Unencrypted room: `SendMessage()` produces `m.room.message` (unchanged)
  - [ ] Incoming `m.room.encrypted` decrypted before processing
  - [ ] Decryption failure: placeholder message `🔒 Encrypted message — decryption failed` emitted to room, event logged with event_id, queued for retry (max 3 attempts)
  - [ ] Megolm session rotation handled correctly (new session created and shared when limit reached)
  - [ ] Performance: encryption adds <20ms latency per message (measured with 100-message batch)

  **QA Scenarios**:
  ```
  Scenario: Dual-mode messaging
    Tool: Bash
    Steps:
      1. Send to unencrypted room → verify m.room.message
      2. Send to encrypted room → verify m.room.encrypted
      3. Receive m.room.encrypted → verify decrypted and processed
    Expected Result: Correct event types per room encryption status
    Evidence: .sisyphus/evidence/task-14-dual-mode.txt

  Scenario: Decryption failure with placeholder message and retry
    Tool: Bash
    Preconditions: Bridge in encrypted room, missing Megolm session key for a message
    Steps:
      1. Send encrypted message from another client without sharing session key
      2. Verify Bridge emits placeholder: "🔒 Encrypted message — decryption failed"
      3. Verify Bridge logs event_id at WARN level
      4. Share correct key material via to_device
      5. Verify Bridge retries and successfully decrypts on next attempt
    Expected Result: Placeholder shown, retry succeeds when key material arrives
    Failure Indicators: Silent message loss, no placeholder, crash on decryption failure
    Evidence: .sisyphus/evidence/task-14-decryption-failure.txt

  Scenario: Encryption performance benchmark
    Tool: Bash
    Steps:
      1. `cd bridge && go test -bench=BenchmarkEncrypt -benchmem ./pkg/crypto/...`
      2. Send 100 messages to encrypted room, measure average latency
    Expected Result: <20ms average encryption overhead per message
    Evidence: .sisyphus/evidence/task-14-perf-benchmark.txt
  ```

  **Commit**: YES
  - Message: `feat(e2ee): add encrypt/decrypt to message pipeline with dual-mode support`
  - Files: `bridge/internal/adapter/matrix.go`
  - Pre-commit: `go test ./internal/adapter/...`

- [ ] 15. Device Key Upload/Query API Handlers

  **What to do**:
  - Add `/keys/upload`, `/keys/query`, `/keys/claim` handlers wired to CryptoEngine
  - Handle device list change notifications from sync (`device_lists.changed`)

  **Must NOT do**:
  - Do NOT implement key gossip or key backup

  **Recommended Agent Profile**:
  - **Category**: `deep`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: NO (sequential after T14 — shared matrix.go)
  - **Parallel Group**: Wave 3a
  - **Blocks**: Tasks 9.5, 16, 17
  - **Blocked By**: Task 14 (needs encrypt/decrypt wiring)

  **References**:
  - `bridge/internal/adapter/matrix.go` — Add key management endpoints
  - `bridge/pkg/crypto/engine.go` — CryptoEngine

  **Acceptance Criteria**:
  - [ ] Bridge uploads device keys on startup
  - [ ] Bridge queries device keys for encrypted room users
  - [ ] Bridge claims one-time keys for Olm sessions
  - [ ] `go test ./pkg/crypto/... -run TestKeyManagement` → PASS

  **QA Scenarios**:
  ```
  Scenario: Device key upload on startup
    Tool: Bash
    Steps:
      1. Start bridge with E2EE enabled
      2. Query Conduit for bridge device keys
    Expected Result: Device keys present
    Evidence: .sisyphus/evidence/task-15-key-upload.txt
  ```

  **Commit**: YES
  - Message: `feat(e2ee): add device key upload/query/claim handlers`
  - Files: `bridge/pkg/crypto/engine.go`, `bridge/internal/adapter/matrix.go`

- [ ] 9.5. Expand crypto.Store (Cross-Signing + Verification Methods)

  **What to do**:
  - Add remaining mautrix-go `crypto.Store` methods needed for verification:
    - Cross-signing key storage: `GetCrossSigningKey`, `PutCrossSigningKey`, `GetCrossSigningKeysForUser` (3-4 methods)
    - Key verification tracking: `PutKeyVerification`, `GetKeyVerification`, `DeleteKeyVerification` (3 methods)
    - Message verification cache: `PutVerifiedMessage`, `IsMessageVerified` (2 methods)
  - Add new SQLCipher tables:
    - `cross_signing_keys` (user_id, key_type, key_data, usage)
    - `key_verifications` (verification_id, state, timestamp)
    - `verified_messages` (event_id, verified_at)
  - Write migration with rollback/down scripts
  - Write store tests for all new methods

  **Must NOT do**:
  - Do NOT modify methods added in Task 9
  - Do NOT create a separate database

  **Recommended Agent Profile**:
  - **Category**: `deep`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: NO (sequential — depends on T9 core, blocks T16/T17)
  - **Parallel Group**: Wave 3b
  - **Blocks**: Tasks 16, 17
  - **Blocked By**: Tasks 9 (core store), 15 (device keys exist)

  **References**:
  - `bridge/pkg/crypto/store.go` — Expanded from Task 9
  - `bridge/pkg/crypto/keystore_store.go` — SQLCipher implementation
  - mautrix-go `crypto.Store` interface — remaining methods

  **Acceptance Criteria**:
  - [ ] All verification-related store methods implemented
  - [ ] New tables created with schema version tracking
  - [ ] Down migrations written for rollback
  - [ ] `go test ./pkg/crypto/... -run TestCrossSignStore` → PASS

  **QA Scenarios**:
  ```
  Scenario: Cross-signing key store CRUD
    Tool: Bash
    Steps:
      1. `go test ./pkg/crypto/... -run TestCrossSignKeyCRUD -v`
    Expected Result: Keys stored, retrieved, and deleted correctly
    Evidence: .sisyphus/evidence/task-9.5-crosssign-store.txt
  ```

  **Commit**: YES
  - Message: `feat(e2ee): add cross-signing and verification store methods`
  - Files: `bridge/pkg/crypto/store.go`, `bridge/pkg/crypto/keystore_store.go`

- [ ] 16. SAS Verification Implementation

  **What to do**:
  - Implement Matrix SAS verification: start → accept → key exchange → emoji → MAC → done
  - Wire into existing RPC handlers (`device.start_verification`, `device.confirm_verification`, `device.cancel_verification`)
  - Handle cancellation and timeout

  **Must NOT do**:
  - Do NOT create a Matrix verification bot
  - Do NOT store verification state in database

  **Recommended Agent Profile**:
  - **Category**: `deep`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES (with Tasks 14, 17-19)
  - **Parallel Group**: Wave 3
  - **Blocks**: Tasks 20, 21
  - **Blocked By**: Task 15 (needs device key exchange)

  **References**:
  - `bridge/pkg/rpc/bridge_handlers.go` — RPC handlers
  - `mautrix-go/crypto/verify` — Built-in SAS support

  **Acceptance Criteria**:
  - [ ] Full SAS flow working end-to-end
  - [ ] RPC handlers trigger verification flow
  - [ ] Cancellation handled cleanly
  - [ ] `go test ./pkg/crypto/... -run TestSAS` → PASS

  **QA Scenarios**:
  ```
  Scenario: Full SAS verification happy path
    Tool: Bash
    Steps:
      1. Initiate verification via RPC
      2. Verify m.key.verification.start event in Matrix
      3. Simulate accept + key exchange
      4. Confirm via RPC
      5. Verify m.key.verification.done event
    Expected Result: Verification completes, device marked verified
    Evidence: .sisyphus/evidence/task-16-sas-happy.txt
  ```

  **Commit**: YES
  - Message: `feat(e2ee): implement SAS verification flow`
  - Files: `bridge/pkg/crypto/verification.go`, `bridge/pkg/rpc/bridge_handlers.go`

- [ ] 17. Cross-Signing Bootstrap (with UIAA Solution)

  **What to do**:
  - Generate MSK, SSK, USK keys
  - Sign device key with MSK
  - **Handle UIAA (User-Interactive Authentication)**:
    - `POST /_matrix/client/v3/keys/device_signing/upload` returns 401 requiring re-auth
    - **Strategy A**: Perform cross-signing setup during initial Bridge login when access token is fresh (some servers allow setup without re-auth immediately after login)
    - **Strategy B**: Store admin password in SQLCipher keystore and use it for UIAA `m.login.password` auth
    - **Strategy C**: If Conduit doesn't require UIAA for this endpoint, proceed without it
    - During Task 1 validation, test which strategy applies to our Conduit version
  - Upload cross-signing keys to Conduit
  - Add RPC handler for bootstrap trigger
  - Sign other devices during verification using USK

  **Must NOT do**:
  - Do NOT implement key backup/restore
  - Do NOT implement cross-signing reset
  - Do NOT store admin password in plaintext — SQLCipher keystore only

  **Recommended Agent Profile**:
  - **Category**: `deep`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES (with Tasks 16, 18-19)
  - **Parallel Group**: Wave 3b
  - **Blocks**: Tasks 20, 21
  - **Blocked By**: Tasks 15 (device key infrastructure), 9.5 (cross-signing store methods)

  **References**:
  - `bridge/pkg/crypto/engine.go` — CryptoEngine
  - `mautrix-go/crypto/cross-sign` — Cross-signing support

  **Acceptance Criteria**:
  - [ ] Cross-signing keys generated and uploaded
  - [ ] Bridge device key signed by MSK
  - [ ] `go test ./pkg/crypto/... -run TestCrossSigning` → PASS

  **QA Scenarios**:
  ```
  Scenario: Cross-signing bootstrap
    Tool: Bash
    Steps:
      1. Trigger bootstrap via RPC
      2. Query cross-signing keys from Conduit
    Expected Result: MSK, SSK, USK all present
    Evidence: .sisyphus/evidence/task-17-crosssign.txt
  ```

  **Commit**: YES
  - Message: `feat(e2ee): implement cross-signing bootstrap`
  - Files: `bridge/pkg/crypto/crosssign.go`

- [ ] 18. Handle Jetski Disconnection Gracefully

  **What to do**:
  - Add disconnection handler to JetskiStateEventSubscriber
  - On disconnect: transition agents to OFFLINE, broadcast via Matrix
  - Implement reconnection with exponential backoff

  **Must NOT do**:
  - Do NOT crash the Bridge if Jetski is unavailable
  - Do NOT block Bridge startup waiting for Jetski

  **Recommended Agent Profile**:
  - **Category**: `quick`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 3
  - **Blocks**: None
  - **Blocked By**: Task 11

  **References**:
  - `bridge/pkg/agent/jetski_subscriber.go` — Subscriber from Task 10
  - `bridge/pkg/agent/state.go` — OFFLINE status constant

  **Acceptance Criteria**:
  - [ ] Jetski disconnect detected and logged
  - [ ] Affected agents transition to OFFLINE
  - [ ] Reconnection succeeds after backoff

  **QA Scenarios**:
  ```
  Scenario: Jetski disconnect → agents OFFLINE
    Tool: Bash
    Steps:
      1. Bridge with active agents, kill Jetski
      2. Check agent states within 10s
    Expected Result: All agents OFFLINE
    Evidence: .sisyphus/evidence/task-18-disconnect.txt
  ```

  **Commit**: YES (groups with 19)
  - Message: `feat(agent): handle Jetski disconnection gracefully`
  - Files: `bridge/pkg/agent/jetski_subscriber.go`

- [ ] 19. Add State Transition Logging

  **What to do**:
  - Add structured logging: agent_id, previous state, new state, inference source
  - Redact PII from logged CDP event params

  **Must NOT do**:
  - Do NOT log PII or sensitive data

  **Recommended Agent Profile**:
  - **Category**: `quick`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 3
  - **Blocks**: None
  - **Blocked By**: Task 11

  **References**:
  - `bridge/pkg/agent/state_inference.go` — Add logging at inference points
  - `bridge/pkg/agent/state_machine.go:195` — Add logging in ForceTransition

  **Acceptance Criteria**:
  - [ ] Every state transition logged with required fields
  - [ ] PII redacted from log output

  **QA Scenarios**:
  ```
  Scenario: Structured log entry on state transition
    Tool: Bash
    Steps:
      1. Trigger state transition
      2. Check bridge logs
    Expected Result: Structured entry with agent_id, previous, current, source
    Evidence: .sisyphus/evidence/task-19-logging.txt
  ```

  **Commit**: YES (groups with 18)
  - Message: `feat(agent): add structured logging for state transitions`
  - Files: `bridge/pkg/agent/state_inference.go`

- [ ] 20. Update isBridgeVerified Trust Model

  **What to do**:
  - Replace temporary trust assessment with real cross-signing-based verification
  - Check if bridge device has valid cross-signing signature
  - Return proper trust level based on cross-signing status

  **Must NOT do**:
  - Do NOT remove existing device trust/approval workflow

  **Recommended Agent Profile**:
  - **Category**: `quick`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 4
  - **Blocks**: None
  - **Blocked By**: Tasks 16, 17 (needs SAS + cross-signing)

  **References**:
  - `bridge/pkg/trust/device.go` — Device trust management
  - `bridge/pkg/crypto/crosssign.go` — Cross-signing from Task 17

  **Acceptance Criteria**:
  - [ ] Trust assessment uses cross-signing verification
  - [ ] Remove TEMPORARY markers from current implementation

  **QA Scenarios**:
  ```
  Scenario: Bridge trust reflects cross-signing status
    Tool: Bash
    Steps:
      1. Bootstrap cross-signing
      2. Query bridge trust level
    Expected Result: Trust level reflects cross-signed status
    Evidence: .sisyphus/evidence/task-20-trust.txt
  ```

  **Commit**: YES
  - Message: `feat(e2ee): update bridge trust model to use cross-signing verification`
  - Files: `bridge/pkg/trust/device.go`

- [ ] 21. E2EE Integration Tests

  **What to do**:
  - Write comprehensive integration tests for E2EE:
    - Encrypt/decrypt round-trip in encrypted room
    - Dual-mode (encrypted + unencrypted rooms simultaneously)
    - Session persistence across Bridge restart
    - Kill switch verification (disable mid-session → plaintext fallback)
    - SAS verification full flow
    - Cross-signing bootstrap + device verification
  - Use Conduit test server instance

  **Must NOT do**:
  - Do NOT use mocks for crypto — test against real Olm/Megolm operations
  - Do NOT require external Matrix server (use local Conduit)

  **Recommended Agent Profile**:
  - **Category**: `unspecified-high`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES (with Tasks 22-23)
  - **Parallel Group**: Wave 4
  - **Blocks**: Task 24
  - **Blocked By**: Tasks 14, 16, 17

  **References**:
  - `bridge/pkg/crypto/` — All crypto modules
  - `bridge/internal/adapter/matrix.go` — Message pipeline
  - `tests/e2ee/` — Check for existing E2EE test infrastructure

  **Acceptance Criteria**:
  - [ ] `go test -v -run TestE2EE ./pkg/crypto/... ./internal/adapter/...` → PASS
  - [ ] Tests cover: encrypt, decrypt, dual-mode, session persistence, kill switch, SAS, cross-signing
  - [ ] Deployment migration scenario: Bridge with existing plaintext rooms enables E2EE — existing rooms remain plaintext, new encrypted rooms work

  **QA Scenarios**:
  ```
  Scenario: Full E2EE integration suite
    Tool: Bash
    Steps:
      1. `cd bridge && go test -v -run TestE2EE ./... 2>&1 | tee /tmp/e2ee-test.log`
      2. Verify all tests pass
    Expected Result: All E2EE integration tests pass
    Evidence: .sisyphus/evidence/task-21-e2ee-tests.txt

  Scenario: Deployment migration — existing plaintext rooms when E2EE enabled
    Tool: Bash
    Preconditions: Bridge running with existing rooms (plaintext), ARMORCLAW_E2EE_ENABLED=true
    Steps:
      1. Verify existing rooms still send/receive plaintext messages (m.room.message)
      2. Create a new room with encryption enabled (m.room.encryption state event)
      3. Send to new encrypted room → verify m.room.encrypted
      4. Send to existing plaintext room → verify m.room.message (unchanged)
    Expected Result: Existing rooms remain plaintext, new encrypted rooms work correctly
    Failure Indicators: Existing rooms start encrypting, messages lost in transition
    Evidence: .sisyphus/evidence/task-21-deployment-migration.txt
  ```

  **Commit**: YES
  - Message: `test(e2ee): add comprehensive E2EE integration tests`
  - Files: `bridge/pkg/crypto/e2ee_integration_test.go`

- [ ] 22. Agent State E2E Tests

  **What to do**:
  - Write end-to-end tests for agent state visibility:
    - CDP event → state inference → state transition → Matrix broadcast
    - Workflow side-channel signals override CDP inference
    - Jetski disconnect → agents go OFFLINE
    - HITL approval protected from CDP override
    - Multiple agents with independent states
  - Use mock Jetski RPC server for CDP events

  **Recommended Agent Profile**:
  - **Category**: `unspecified-high`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES (with Tasks 21, 23)
  - **Parallel Group**: Wave 4
  - **Blocks**: Task 24
  - **Blocked By**: Tasks 11, 12

  **References**:
  - `bridge/pkg/agent/` — All agent state modules
  - `tests/test-agent-runtime.sh` — Existing agent runtime test
  - `tests/test-cross-event-truth.sh` — Existing cross-event test

  **Acceptance Criteria**:
  - [ ] `go test -v -run TestAgentStateE2E ./pkg/agent/...` → PASS
  - [ ] Tests cover all inference paths and priority rules

  **QA Scenarios**:
  ```
  Scenario: Full agent state E2E suite
    Tool: Bash
    Steps:
      1. `cd bridge && go test -v -run TestAgentStateE2E ./... 2>&1`
    Expected Result: All agent state tests pass
    Evidence: .sisyphus/evidence/task-22-agent-tests.txt
  ```

  **Commit**: YES
  - Message: `test(agent): add end-to-end agent state visibility tests`
  - Files: `bridge/pkg/agent/state_e2e_test.go`

- [x] 23. State Enum Audit and Consolidation

  **What to do**:
  - Catalog ALL state/status enums across the Go codebase (AgentStatus, TaskStatus, BrowserStatus, ServiceState, etc.)
  - Identify true duplicates (identical semantics, different names)
  - Identify safe merges (same values, same lifecycle)
  - Add type aliases for backward compatibility
  - Add deprecation annotations to merged types
  - Verify AgentStatus string constants are UNCHANGED

  **Must NOT do**:
  - Do NOT change AgentStatus string constants (Matrix protocol contract)
  - Do NOT change ValidTransitions graph without updating tests
  - Do NOT persist new enum values that break existing data

  **Recommended Agent Profile**:
  - **Category**: `unspecified-high`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES (independent of all other tasks)
  - **Parallel Group**: Wave 1 (moved from Wave 4 — runs before other tasks touch enum files, avoids merge conflicts)
  - **Blocks**: Task 24
  - **Blocked By**: None

  **References**:
  - `bridge/pkg/agent/state.go` — AgentStatus (11 values)
  - `ast_grep_search` — Use to find all status enum patterns across Go code

  **Acceptance Criteria**:
  - [ ] Audit document written to `.sisyphus/evidence/task-23-enum-audit.md`
  - [ ] Safe merges implemented with type aliases
  - [ ] `go test ./pkg/agent/... ./pkg/browser/...` → PASS
  - [ ] AgentStatus constants UNCHANGED

  **QA Scenarios**:
  ```
  Scenario: State enum consolidation passes all tests
    Tool: Bash
    Steps:
      1. `cd bridge && go test ./pkg/agent/... ./pkg/browser/... -v`
    Expected Result: All tests pass after consolidation
    Evidence: .sisyphus/evidence/task-23-enum-tests.txt
  ```

  **Commit**: YES
  - Message: `refactor(agent): consolidate overlapping state enums with type aliases`
  - Files: affected enum files in bridge/

- [ ] 24. Full Regression Pass

  **What to do**:
  - Run complete test suites across all modified components:
    - `cd bridge && go test ./...` — all Go tests
    - `cd sidecar && cargo test --lib` — all Rust tests
    - Bash integration test harness in `tests/`
  - Verify no regressions from E2EE, agent state, TLS, or Azure Blob changes
  - Run existing integration tests: `test-agent-runtime.sh`, `test-cross-event-truth.sh`, `test-system-health-baseline.sh`

  **Recommended Agent Profile**:
  - **Category**: `unspecified-high`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: NO (must wait for all other tasks)
  - **Parallel Group**: Wave 4 (sequential after Tasks 21-23)
  - **Blocks**: FINAL
  - **Blocked By**: Tasks 21, 22, 23

  **References**:
  - `tests/` — Full bash test harness
  - `bridge/` — Go test suite
  - `sidecar/` — Rust test suite

  **Acceptance Criteria**:
  - [ ] `go test ./...` in bridge/ → 0 failures
  - [ ] `cargo test --lib` in sidecar/ → 0 failures
  - [ ] All Tier A integration tests pass
  - [ ] All Tier B integration tests pass or skip gracefully

  **QA Scenarios**:
  ```
  Scenario: Full regression pass
    Tool: Bash
    Steps:
      1. `cd bridge && go test ./... 2>&1 | tee /tmp/regression-go.log`
      2. `cd sidecar && cargo test --lib 2>&1 | tee /tmp/regression-rust.log`
      3. Run bash harness: `for f in tests/test-*.sh; do bash "$f"; done`
    Expected Result: Zero failures across all test suites
    Evidence: .sisyphus/evidence/task-24-regression.txt
  ```

  **Commit**: YES
  - Message: `chore: full regression pass after risk remediation sprint`
  - Files: None (verification only, commit test results)

---

## Final Verification Wave

- [ ] F1. **Plan Compliance Audit** — `oracle`
  Read the plan end-to-end. For each "Must Have": verify implementation exists (read file, run command). For each "Must NOT Have": search codebase for forbidden patterns — reject with file:line if found. Check evidence files exist in .sisyphus/evidence/. Compare deliverables against plan.
  Output: `Must Have [N/N] | Must NOT Have [N/N] | Tasks [N/N] | VERDICT: APPROVE/REJECT`

- [ ] F2. **Code Quality Review** — `unspecified-high`
  Run `go test ./...` in bridge/, `cargo test --lib` in sidecar/, linters. Review all changed files for: `panic("todo")`, empty error catches, `fmt.Println` in prod, commented-out code, unused imports. Check AI slop: excessive comments, over-abstraction, generic names.
  Output: `Build [PASS/FAIL] | Lint [PASS/FAIL] | Tests [N pass/N fail] | Files [N clean/N issues] | VERDICT`

- [ ] F3. **Agent-Executed Integration QA** — `unspecified-high`
  Start from clean state. Execute EVERY QA scenario from EVERY task using automated tools (Playwright for browser, curl for API, go test for unit/integration). Test cross-task integration (E2EE + agent state working together). Test edge cases: dual-mode messaging, Jetski reconnect, TLS with different modes. Save to `.sisyphus/evidence/final-qa/`.
  Output: `Scenarios [N/N pass] | Integration [N/N] | Edge Cases [N tested] | VERDICT`

- [ ] F4. **Scope Fidelity Check** — `deep`
  For each task: read "What to do", read actual diff (git log/diff). Verify 1:1 — everything in spec was built (no missing), nothing beyond spec was built (no creep). Check "Must NOT do" compliance. Detect cross-task contamination. Flag unaccounted changes.
  Output: `Tasks [N/N compliant] | Contamination [CLEAN/N issues] | Unaccounted [CLEAN/N files] | VERDICT`

---

## Commit Strategy

- **Wave 1 Tasks (3-5)**: `fix(tls): derive TLS mode from server config instead of hardcoding` — bridge/pkg/http/server.go
- **Wave 1 Task 4**: `fix(tls): include all v2 fields in QR config HMAC signature` — bridge/pkg/qr/public.go
- **Wave 1 Task 5**: `fix(tls): enrich well-known endpoint with TLS metadata` — bridge/pkg/http/server.go
- **Wave 1 Task 6**: `feat(sidecar): migrate Azure Blob connector from native-tls to rustls` — sidecar/src/connectors/azure_blob.rs
- **Wave 2 Tasks (7-8)**: `feat(e2ee): add ToDevice sync support and config kill switch` — bridge/internal/adapter/matrix.go, bridge/pkg/matrix/client.go, bridge/pkg/config/config.go
- **Wave 2 Task 9**: `feat(e2ee): expand crypto store for mautrix-go integration` — bridge/pkg/crypto/store.go, bridge/pkg/crypto/keystore_store.go
- **Wave 2 Tasks (10-12)**: `feat(agent): wire Jetski CDP events to state inference engine` — bridge/pkg/agent/, jetski/
- **Wave 3 Tasks (13-14)**: `feat(e2ee): wire OlmMachine encrypt/decrypt into message pipeline` — bridge/internal/adapter/matrix.go
- **Wave 3 Task 15**: `feat(e2ee): add device key upload/query API handlers` — bridge/internal/adapter/matrix.go
- **Wave 3 Task 16**: `feat(e2ee): implement SAS verification flow` — bridge/internal/adapter/
- **Wave 3 Task 17**: `feat(e2ee): implement cross-signing bootstrap` — bridge/internal/adapter/
- **Wave 3 Tasks (18-19)**: `feat(agent): add disconnection handling and transition logging` — bridge/pkg/agent/
- **Wave 4 Task 20**: `feat(e2ee): update bridge verification trust model` — bridge/pkg/trust/
- **Wave 4 Tasks (21-24)**: Individual commits per test suite

---

## Success Criteria

### Verification Commands
```bash
# E2EE
cd bridge && go test -v -run TestE2EE ./pkg/crypto/... ./internal/adapter/...

# Agent State
cd bridge && go test -v -run TestAgentState ./pkg/agent/...

# TLS
cd bridge && go test -v -run TestTLS ./pkg/qr/... ./pkg/http/...

# Azure Blob
cd sidecar && cargo test --lib azure_blob

# Full regression
cd bridge && go test ./...
cd sidecar && cargo test --lib
```

### Final Checklist
- [ ] All "Must Have" present
- [ ] All "Must NOT Have" absent
- [ ] All Go tests pass: `go test ./...` in bridge/
- [ ] All Rust tests pass: `cargo test --lib` in sidecar/
- [ ] E2EE dual-mode works (encrypted rooms encrypt, unencrypted rooms plaintext)
- [ ] Agent state transitions broadcast to Matrix
- [ ] TLS QR v2 has correct mode and complete HMAC
- [ ] Azure Blob CRUD works with rustls
- [ ] No AgentStatus string constants changed
- [ ] No S3 connector touched during Azure Blob work
