# Bridge Server-Side Device & Invite Governance

## TL;DR

> **Quick Summary**: Implement the missing server-side RPC handlers for device and invite governance in the Bridge, wire auth middleware using provisioning admin tokens, add SQLCipher persistence, and emit governance events via Matrix.
>
> **Deliverables**:
> - 8 RPC handlers matching admin panel TypeScript contract exactly
> - SQLCipher-backed device store and invite store (shared keystore DB)
> - Admin token auth middleware wired to `/api` endpoint
> - Governance events (app.armorclaw.device.*, app.armorclaw.invite.*)
> - Updated admin panel RPC client to send Authorization header
> - docs/reference/rpc-api.md documenting the complete RPC surface
>
> **Estimated Effort**: Large
> **Parallel Execution**: YES - 5 waves
> **Critical Path**: Task 1 (auth) → Task 3-4 (stores) → Task 5-6 (RPC handlers) → Task 8-9 (events) → Task 10-11 (docs + client)

---

## Context

### Original Request
Verify the existing Bridge RPC surface, then add only the missing server-side governance pieces needed for ArmorChat/mobile governance. The docs suggest the admin panel already uses real Bridge RPC, but verification was needed first.

### Interview Summary
**Key Discussions**:
- Phase 0 verification revealed the admin panel has typed RPC client code but ZERO server handlers — every device/invite call returns Method Not Found
- `review.md` claim about "real RPC" is misleading — client code exists, server handlers don't
- Admin panel sends NO auth header and `/api` has NO authentication middleware
- User chose: Full governance scope (device RPC + invite RPC + auth + events + docs)
- User chose: Admin panel TypeScript is source of truth — server must match exactly
- User chose: SQLCipher persistence for device and invite stores
- User chose: Provisioning admin token (aat_ prefix) for auth, not Matrix tokens

**Research Findings**:
- 82 RPC methods exist in the Bridge, none are device/invite related
- `bridge/pkg/trust/device.go` has Device struct with TrustState enum (foundation exists)
- `bridge/pkg/invite/roles.go` has InviteManager with code generation, validation, expiration (domain logic exists)
- `bridge/pkg/provisioning/manager.go` has `generateAdminToken()` (auth foundation exists)
- `bridge/pkg/auth/matrix_auth.go` has `RPCAuthMiddleware` with extensible method lists (auth infrastructure exists but unwired)
- `bridge/pkg/audit/` has AuditLog with file persistence (audit exists)
- `bridge/pkg/push/` has Gateway with FCM/APNS/WebPush + Sygnal (push exists)
- Email approval pattern: Matrix event for notification + RPC call for decision (event template exists)
- `docs/reference/rpc-api.md` does NOT exist (needs creation)

### Metis Review
**Identified Gaps** (addressed):
- Auth middleware already exists (`bridge/pkg/auth/matrix_auth.go`) but is unwired — must wire, not rebuild
- `device.revoke` does NOT exist in the TS client — removed from scope
- Admin panel sends no auth header — must update TS client `rpc()` method
- Invite package has existing domain logic — add persistence underneath, don't replace
- `expiration` field sent as duration string ("7d", "1h", "never") — server must parse
- JSON field names must match TS exactly: `trust_state` (snake_case), `last_seen` (ISO 8601), `is_current` (boolean)
- `invite.validate` may leak info if public — must be admin-gated

---

## Work Objectives

### Core Objective
Connect the admin panel's existing device and invite RPC client code to real Bridge server handlers, with proper authentication, persistent storage, audit logging, and real-time governance events.

### Concrete Deliverables
- `bridge/pkg/rpc/device_handlers.go` — 4 device RPC handlers
- `bridge/pkg/rpc/invite_handlers.go` — 4 invite RPC handlers
- `bridge/pkg/trust/device_store.go` — SQLCipher device persistence
- `bridge/pkg/invite/store.go` — SQLCipher invite persistence
- Auth middleware wired to `/api` with provisioning admin token validation
- Governance event emission in `app.armorclaw.device.*` and `app.armorclaw.invite.*` namespaces
- `docs/reference/rpc-api.md` — Complete RPC API documentation
- Updated `applications/admin-panel/src/services/bridgeApi.ts` to send Authorization header

### Definition of Done
- [ ] `curl -X POST /api -d '{"jsonrpc":"2.0","id":1,"method":"device.list"}'` returns `Device[]` (empty array)
- [ ] `curl -X POST /api -d '{"jsonrpc":"2.0","id":1,"method":"device.list"}'` without auth returns auth error
- [ ] `device.approve` transitions a pending device to verified and emits audit log entry
- [ ] `invite.create` returns Invite with code, persists to SQLCipher, emits event
- [ ] `invite.revoke` transitions invite to revoked status, subsequent `invite.validate` fails
- [ ] `go test ./pkg/rpc/... ./pkg/trust/... ./pkg/invite/...` — all pass
- [ ] Admin panel can list devices and invites without errors
- [ ] `docs/reference/rpc-api.md` exists and documents all methods

### Must Have
- Exact shape match with admin panel TypeScript types (field names, types, formats)
- SQLCipher persistence for device and invite stores (shared keystore DB)
- Admin token auth on all governance RPC methods
- Audit log entries for every mutation (approve, reject, revoke, create, revoke invite)
- Governance events emitted after state-changing mutations
- Closed-registration posture maintained (no public registration toggle)

### Must NOT Have (Guardrails)
- NO `device.revoke` method — doesn't exist in TS client
- NO lockdown, security, adapter, QR, admin claim, or secrets methods (20+ out of scope)
- NO separate database files — share the keystore DB
- NO replacement of existing InviteManager or DeviceManager — add persistence underneath
- NO changes to CORS middleware (already allows Authorization header)
- NO public registration toggle added
- NO NetworkMode changes or container security model changes
- NO excessive comments, over-abstraction, or generic names (AI slop patterns)

---

## Verification Strategy (MANDATORY)

> **ZERO HUMAN INTERVENTION** - ALL verification is agent-executed. No exceptions.

### Test Decision
- **Infrastructure exists**: YES (Go testing, no external test framework)
- **Automated tests**: YES (tests-after — write tests alongside implementation)
- **Framework**: Go standard `testing` package + `testify/assert` + `testify/require`
- **Pattern**: Each RPC handler gets unit tests with mock dependencies

### QA Policy
Every task MUST include agent-executed QA scenarios.
Evidence saved to `.sisyphus/evidence/task-{N}-{scenario-slug}.{ext}`.

- **RPC Handlers**: Use Bash (curl) - Send JSON-RPC requests, assert status + response fields
- **Store Layer**: Use Bash (go test) - Run Go test suite, assert PASS
- **Auth Middleware**: Use Bash (curl) - Send requests with/without tokens, assert error codes
- **Events**: Use Bash (go test) - Mock MatrixAdapter, assert SendEvent call count and payload

---

## Execution Strategy

### Parallel Execution Waves

```
Wave 1 (Start Immediately — foundation + stores):
├── Task 1: Wire auth middleware to /api [deep]
├── Task 2: Device store SQLCipher persistence [deep]
├── Task 3: Invite store SQLCipher persistence [deep]
└── Task 4: Shared request/response types [quick]

Wave 2 (After Wave 1 — RPC handlers):
├── Task 5: Device RPC handlers (depends: 1, 2, 4) [unspecified-high]
├── Task 6: Invite RPC handlers (depends: 1, 3, 4) [unspecified-high]
└── Task 7: Audit integration for mutations (depends: 2, 3) [unspecified-high]

Wave 3 (After Wave 2 — events + client):
├── Task 8: Governance event emission (depends: 5, 6) [unspecified-high]
├── Task 9: Update admin panel RPC client auth (depends: 1) [quick]
└── Task 10: Update review.md accuracy (depends: nothing) [quick]

Wave 4 (After Wave 3 — docs):
├── Task 11: Create docs/reference/rpc-api.md (depends: 5, 6, 8) [writing]

Wave FINAL (After ALL tasks — 4 parallel reviews):
├── Task F1: Plan compliance audit (oracle)
├── Task F2: Code quality review (unspecified-high)
├── Task F3: Real manual QA (unspecified-high)
└── Task F4: Scope fidelity check (deep)
-> Present results -> Get explicit user okay

Critical Path: Task 1 → Task 5 → Task 8 → Task 11 → F1-F4
Parallel Speedup: ~60% faster than sequential
Max Concurrent: 4 (Wave 1)
```

### Dependency Matrix

| Task | Depends On | Blocks | Wave |
|------|-----------|--------|------|
| 1 | - | 5, 6, 9 | 1 |
| 2 | - | 5, 7 | 1 |
| 3 | - | 6, 7 | 1 |
| 4 | - | 5, 6 | 1 |
| 5 | 1, 2, 4 | 8 | 2 |
| 6 | 1, 3, 4 | 8 | 2 |
| 7 | 2, 3 | - | 2 |
| 8 | 5, 6 | 11 | 3 |
| 9 | 1 | - | 3 |
| 10 | - | - | 3 |
| 11 | 5, 6, 8 | - | 4 |

### Agent Dispatch Summary

- **Wave 1**: 4 tasks — T1 → `deep`, T2 → `deep`, T3 → `deep`, T4 → `quick`
- **Wave 2**: 3 tasks — T5 → `unspecified-high`, T6 → `unspecified-high`, T7 → `unspecified-high`
- **Wave 3**: 3 tasks — T8 → `unspecified-high`, T9 → `quick`, T10 → `quick`
- **Wave 4**: 1 task — T11 → `writing`
- **FINAL**: 4 tasks — F1 → `oracle`, F2 → `unspecified-high`, F3 → `unspecified-high`, F4 → `deep`

---

## TODOs

- [x] 1. Wire Auth Middleware to /api with Admin Token Validation

  **What to do**:
  - Read `bridge/pkg/auth/matrix_auth.go` — the `RPCAuthMiddleware` already exists with `Authenticate()`, `DefaultPublicMethods`, `DefaultAdminMethods`
  - Add a second auth path to `RPCAuthMiddleware.Authenticate()` that validates `aat_` prefix tokens (provisioning admin tokens) by calling into `bridge/pkg/provisioning/manager.go`'s token validation
  - Add device and invite method names to `DefaultAdminMethods` list: `device.list`, `device.get`, `device.approve`, `device.reject`, `invite.create`, `invite.list`, `invite.revoke`, `invite.validate`
  - Wire `RPCAuthMiddleware` into `handleRPC()` at `bridge/pkg/http/server.go:370-393` — extract Bearer token from `Authorization` header, call `Authenticate()` before dispatching to handler
  - Extract Bearer token using existing `ExtractBearerToken()` helper in the auth package
  - If auth fails, return JSON-RPC error with code `-32001` and message "unauthorized"
  - Do NOT modify CORS middleware (already allows Authorization header at line 659)
  - Do NOT create a new middleware — extend the existing one

  **Must NOT do**:
  - Create a second middleware (extend existing)
  - Modify CORS settings
  - Use Matrix token auth path for admin tokens (add parallel path)

  **Recommended Agent Profile**:
  - **Category**: `deep`
    - Reason: Auth middleware wiring touches security-critical code paths, needs careful understanding of existing auth infrastructure
  - **Skills**: [`git-master`]
    - `git-master`: For atomic commits after wiring changes
  - **Skills Evaluated but Omitted**:
    - `playwright`: No UI involved

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 1 (with Tasks 2, 3, 4)
  - **Blocks**: Tasks 5, 6, 9
  - **Blocked By**: None (can start immediately)

  **References**:

  **Pattern References** (existing code to follow):
  - `bridge/pkg/auth/matrix_auth.go:244-353` — `RPCAuthMiddleware` with `Authenticate()`, `DefaultPublicMethods`, `DefaultAdminMethods` — extend this, do NOT duplicate
  - `bridge/pkg/auth/matrix_auth.go:376-389` — `DefaultAdminMethods` list — add device/invite method names here
  - `bridge/pkg/provisioning/manager.go:569-576` — `generateAdminToken()` — understand the `aat_` token format for validation
  - `bridge/pkg/provisioning/manager.go` — `ValidateToken()` or equivalent — the validation function to call for admin tokens

  **API/Type References**:
  - `bridge/pkg/http/server.go:370-393` — `handleRPC()` — wire auth check here, before `rpcServer.Handle()` call
  - `bridge/pkg/http/server.go:659` — CORS middleware already allows `Authorization` header — do NOT modify

  **WHY Each Reference Matters**:
  - `matrix_auth.go`: This is the auth infrastructure. The `Authenticate()` method needs a second path for `aat_` tokens alongside the existing Matrix token validation.
  - `server.go:370-393`: This is the wire point. The auth check must happen between request parsing and handler dispatch.
  - `provisioning/manager.go`: The admin token format and validation logic. Must call into this for token verification.

  **Acceptance Criteria**:

  **QA Scenarios (MANDATORY):**

  ```
  Scenario: Unauthenticated request to device.list returns auth error
    Tool: Bash (curl)
    Preconditions: Bridge running on localhost:8080
    Steps:
      1. curl -s -X POST http://localhost:8080/api -H "Content-Type: application/json" -d '{"jsonrpc":"2.0","id":1,"method":"device.list"}'
      2. Parse JSON response, check .error.code
    Expected Result: Response contains {"error":{"code":-32001,"message":"unauthorized"}}
    Failure Indicators: Response contains .result field (auth not enforced), HTTP 200 with no error
    Evidence: .sisyphus/evidence/task-1-auth-no-token.txt

  Scenario: Authenticated request with valid admin token succeeds
    Tool: Bash (curl)
    Preconditions: Bridge running, valid admin token from provisioning.claim
    Steps:
      1. curl -s -X POST http://localhost:8080/api -H "Content-Type: application/json" -H "Authorization: Bearer aat_VALID_TOKEN" -d '{"jsonrpc":"2.0","id":1,"method":"device.list"}'
      2. Parse JSON response, check .result is array (empty is OK)
    Expected Result: Response contains {"result":[]} — auth passed, handler executed
    Failure Indicators: Response contains .error with auth error code, or "Method not found" (handler not registered yet is OK for this task)
    Evidence: .sisyphus/evidence/task-1-auth-valid-token.txt

  Scenario: Invalid token returns auth error
    Tool: Bash (curl)
    Preconditions: Bridge running
    Steps:
      1. curl -s -X POST http://localhost:8080/api -H "Content-Type: application/json" -H "Authorization: Bearer invalid_token" -d '{"jsonrpc":"2.0","id":1,"method":"device.list"}'
      2. Parse JSON response, check .error
    Expected Result: Response contains auth error (code -32001)
    Failure Indicators: Response contains .result field (invalid token accepted)
    Evidence: .sisyphus/evidence/task-1-auth-invalid-token.txt
  ```

  **Commit**: YES
  - Message: `feat(auth): wire RPCAuthMiddleware to /api with admin token validation`
  - Files: `bridge/pkg/auth/matrix_auth.go, bridge/pkg/http/server.go`
  - Pre-commit: `cd bridge && go build ./... && go vet ./...`

- [x] 2. Device Store SQLCipher Persistence

  **What to do**:
  - Read `bridge/pkg/trust/device.go` — existing `Device` struct with `TrustState` enum, `VerificationMethod`, in-memory `devices map[string]*Device`
  - Read `bridge/pkg/trust/hardening.go:84-87` — pattern for sharing keystore `*sql.DB` connection
  - Create `bridge/pkg/trust/device_store.go` with a `DeviceStore` struct that accepts a shared `*sql.DB` connection
  - Implement `initSchema()` with `CREATE TABLE IF NOT EXISTS devices` containing columns matching the admin panel's Device type: id, name, type, platform, trust_state, last_seen, first_seen, ip_address, user_agent, is_current, verified_at, created_at, updated_at
  - Implement CRUD methods: `GetDevice(id)`, `ListDevices()`, `CreateDevice()`, `UpdateDevice()`, `UpdateTrustState()`
  - The Device struct JSON tags must use snake_case: `json:"trust_state"`, `json:"last_seen"`, `json:"is_current"` etc. to match admin panel TypeScript
  - `last_seen` and `first_seen` must be stored as timestamps but serialized as ISO 8601 strings in JSON
  - TrustState values must be lowercase strings in JSON: `"verified"`, `"unverified"`, `"pending_approval"`, `"rejected"` — match TS exactly
  - Do NOT replace existing `DeviceManager` — add persistence layer that the RPC handlers will call
  - Do NOT create a separate database file — share the keystore DB

  **Must NOT do**:
  - Replace existing DeviceManager (add layer underneath)
  - Create separate database file (share keystore DB)
  - Use PascalCase JSON tags (must be snake_case)

  **Recommended Agent Profile**:
  - **Category**: `deep`
    - Reason: SQLCipher schema design, careful JSON tag alignment with TS types
  - **Skills**: [`git-master`]
    - `git-master`: For atomic commits

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 1 (with Tasks 1, 3, 4)
  - **Blocks**: Tasks 5, 7
  - **Blocked By**: None (can start immediately)

  **References**:

  **Pattern References**:
  - `bridge/pkg/trust/device.go` — Device struct, TrustState enum, VerificationMethod — extend with JSON tags
  - `bridge/pkg/trust/hardening.go:84-87` — `NewKeystoreHardeningStore(db *sql.DB)` — follow this shared-DB pattern
  - `bridge/pkg/trust/hardening.go` — `initSchema()` with `CREATE TABLE IF NOT EXISTS` — follow this DDL pattern

  **API/Type References**:
  - `applications/admin-panel/src/services/bridgeApi.ts:Device` interface — `id, name, type, platform, trust_state, last_seen, first_seen, ip_address, user_agent, is_current` — match these EXACTLY
  - `bridge/pkg/trust/device.go:TrustState` — StateUnverified, StatePendingApproval, StateAwaitingSecondFactor, StateVerified, StateRejected, StateExpired — map to lowercase strings in JSON

  **WHY Each Reference Matters**:
  - `device.go`: The Go struct that will be persisted. Must add correct JSON tags.
  - `hardening.go`: The exact pattern for SQLCipher store initialization. Copy the `initSchema()` + shared DB approach.
  - `bridgeApi.ts:Device`: The source of truth for field names and types. Every JSON tag must match.

  **Acceptance Criteria**:

  **QA Scenarios (MANDATORY):**

  ```
  Scenario: Device store CRUD operations work
    Tool: Bash (go test)
    Preconditions: Go build environment, SQLCipher dev libs installed
    Steps:
      1. cd bridge && go test -v -run TestDeviceStore ./pkg/trust/...
      2. Tests should cover: CreateDevice, GetDevice, ListDevices, UpdateTrustState, UpdateDevice
    Expected Result: All tests PASS, 0 failures
    Failure Indicators: Any test FAIL, compilation error
    Evidence: .sisyphus/evidence/task-2-device-store-test.txt

  Scenario: Device JSON serialization matches TS contract
    Tool: Bash (go test)
    Preconditions: Go build environment
    Steps:
      1. cd bridge && go test -v -run TestDeviceJSON ./pkg/trust/...
      2. Test marshals Device to JSON and asserts field names: trust_state (not trustState), last_seen (not lastSeen), is_current (not IsCurrent)
      3. Assert trust_state values are lowercase: "pending_approval" (not "PendingApproval")
      4. Assert timestamps are ISO 8601 strings
    Expected Result: JSON output matches TS Device interface exactly
    Failure Indicators: PascalCase field names, uppercase enum values, non-ISO timestamps
    Evidence: .sisyphus/evidence/task-2-device-json-test.txt

  Scenario: Device persists across DB close/reopen
    Tool: Bash (go test)
    Preconditions: Go build environment
    Steps:
      1. Open DB, create device, close DB
      2. Reopen DB, query device by ID
      3. Assert device data matches what was stored
    Expected Result: Device survives DB restart
    Failure Indicators: Device not found after reopen, data mismatch
    Evidence: .sisyphus/evidence/task-2-device-persistence.txt
  ```

  **Commit**: YES
  - Message: `feat(trust): add SQLCipher device store persistence`
  - Files: `bridge/pkg/trust/device_store.go`
  - Pre-commit: `cd bridge && go build ./pkg/trust/... && go test ./pkg/trust/...`

- [x] 3. Invite Store SQLCipher Persistence

  **What to do**:
  - Read `bridge/pkg/invite/roles.go` — existing `InviteManager` with code generation, validation, expiration, usage tracking, all in-memory
  - Read `bridge/pkg/trust/hardening.go:84-87` — shared DB pattern
  - Create `bridge/pkg/invite/store.go` with an `InviteStore` struct accepting shared `*sql.DB`
  - Implement `initSchema()` with `CREATE TABLE IF NOT EXISTS invites` containing columns matching admin panel Invite type: id, code, role, created_by, created_at, expires_at, max_uses, use_count, status, welcome_message
  - Implement CRUD: `GetInvite(id)`, `GetInviteByCode(code)`, `ListInvites()`, `CreateInvite()`, `RevokeInvite()`, `IncrementUseCount()`
  - Status values in JSON must match TS exactly: `"active"`, `"used"`, `"expired"`, `"revoked"`, `"exhausted"`
  - `expires_at` stored as timestamp, `null` for "never" expiration; serialized as ISO 8601 or `null` in JSON
  - Parse `expiration` duration strings in `CreateInvite`: `"1h"` → 1 hour, `"7d"` → 7 days, `"30d"` → 30 days, `"never"` → NULL expires_at
  - The invite code must be cryptographically random (use `crypto/rand` with base62 encoding, minimum 16 chars)
  - Do NOT replace existing `InviteManager` — add persistence layer
  - Do NOT create a separate database file

  **Must NOT do**:
  - Replace existing InviteManager
  - Create separate database file
  - Use weak random for invite codes (must use crypto/rand)

  **Recommended Agent Profile**:
  - **Category**: `deep`
    - Reason: SQLCipher schema, cryptographic code generation, duration parsing
  - **Skills**: [`git-master`]

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 1 (with Tasks 1, 2, 4)
  - **Blocks**: Tasks 6, 7
  - **Blocked By**: None (can start immediately)

  **References**:

  **Pattern References**:
  - `bridge/pkg/invite/roles.go` — Existing `InviteManager` with code generation, role system, validation — this is the domain logic layer, add persistence underneath
  - `bridge/pkg/trust/hardening.go:84-87` — Shared keystore DB pattern

  **API/Type References**:
  - `applications/admin-panel/src/services/bridgeApi.ts:Invite` interface — `id, code, role, created_by, created_at, expires_at, max_uses, use_count, status` — match EXACTLY
  - `applications/admin-panel/src/pages/InvitationsPage.tsx` — expiration dropdown values: `'1h'`, `'6h'`, `'1d'`, `'7d'`, `'30d'`, `'never'` — server must parse these

  **WHY Each Reference Matters**:
  - `roles.go`: The existing domain logic. The store layer plugs underneath this.
  - `bridgeApi.ts:Invite`: Source of truth for JSON field names and status values.
  - `InvitationsPage.tsx`: The actual expiration strings the client sends — must parse these server-side.

  **Acceptance Criteria**:

  **QA Scenarios (MANDATORY):**

  ```
  Scenario: Invite store CRUD operations work
    Tool: Bash (go test)
    Preconditions: Go build environment, SQLCipher dev libs
    Steps:
      1. cd bridge && go test -v -run TestInviteStore ./pkg/invite/...
      2. Cover: CreateInvite, GetInvite, GetInviteByCode, ListInvites, RevokeInvite, IncrementUseCount
    Expected Result: All tests PASS
    Failure Indicators: Any test FAIL
    Evidence: .sisyphus/evidence/task-3-invite-store-test.txt

  Scenario: Invite JSON matches TS contract
    Tool: Bash (go test)
    Preconditions: Go build environment
    Steps:
      1. cd bridge && go test -v -run TestInviteJSON ./pkg/invite/...
      2. Assert snake_case field names: use_count (not UseCount), expires_at (not ExpiresAt)
      3. Assert status values: "active", "used", "expired", "revoked", "exhausted"
      4. Assert null expires_at serialized as JSON null (not "0001-01-01")
    Expected Result: JSON matches TS Invite interface
    Failure Indicators: PascalCase, wrong status values, zero-time instead of null
    Evidence: .sisyphus/evidence/task-3-invite-json-test.txt

  Scenario: Expiration string parsing
    Tool: Bash (go test)
    Preconditions: Go build environment
    Steps:
      1. cd bridge && go test -v -run TestParseExpiration ./pkg/invite/...
      2. Test "1h" → 1 hour from now, "7d" → 7 days, "30d" → 30 days, "never" → nil
    Expected Result: Correct timestamp deltas
    Failure Indicators: Parsing error, wrong delta
    Evidence: .sisyphus/evidence/task-3-expiration-parse.txt

  Scenario: Invite code is cryptographically random
    Tool: Bash (go test)
    Preconditions: Go build environment
    Steps:
      1. Generate 100 invite codes
      2. Assert all unique
      3. Assert length >= 16 chars
      4. Assert only base62 characters [a-zA-Z0-9]
    Expected Result: All codes unique, >= 16 chars, base62 only
    Failure Indicators: Duplicate codes, short codes, non-base62 chars
    Evidence: .sisyphus/evidence/task-3-code-random.txt
  ```

  **Commit**: YES
  - Message: `feat(invite): add SQLCipher invite store persistence`
  - Files: `bridge/pkg/invite/store.go`
  - Pre-commit: `cd bridge && go build ./pkg/invite/... && go test ./pkg/invite/...`

- [x] 4. Shared Request/Response Types for Governance RPC

  **What to do**:
  - Create `bridge/pkg/rpc/governance_types.go` with typed request and response structs for all 8 methods
  - Device request types:
    - `DeviceListRequest` — no params (empty struct)
    - `DeviceGetRequest` — `DeviceID string \`json:"device_id"\``
    - `DeviceApproveRequest` — `DeviceID string \`json:"device_id"\``, `ApprovedBy string \`json:"approved_by"\``
    - `DeviceRejectRequest` — `DeviceID string \`json:"device_id"\``, `RejectedBy string \`json:"rejected_by"\``, `Reason string \`json:"reason"\``
  - Invite request types:
    - `InviteCreateRequest` — `Role string \`json:"role"\``, `Expiration string \`json:"expiration"\``, `MaxUses int \`json:"max_uses"\``, `WelcomeMessage string \`json:"welcome_message"\``, `CreatedBy string \`json:"created_by"\``
    - `InviteListRequest` — no params (empty struct)
    - `InviteRevokeRequest` — `InviteID string \`json:"invite_id"\``, `RevokedBy string \`json:"revoked_by"\``
    - `InviteValidateRequest` — `Code string \`json:"code"\``
  - All JSON tags must use snake_case to match the admin panel's `bridgeApi.ts` params
  - Response types: Device, Invite (re-use from trust and invite packages with correct JSON tags)
  - Add `SuccessResponse` struct: `Success bool \`json:"success"\``

  **Must NOT do**:
  - Use PascalCase JSON tags
  - Over-abstract into generic types (keep simple per-method structs like existing handlers)

  **Recommended Agent Profile**:
  - **Category**: `quick`
    - Reason: Simple struct definitions, no complex logic
  - **Skills**: [`git-master`]

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 1 (with Tasks 1, 2, 3)
  - **Blocks**: Tasks 5, 6
  - **Blocked By**: None (can start immediately)

  **References**:

  **API/Type References**:
  - `applications/admin-panel/src/services/bridgeApi.ts` — lines with method definitions and param objects — match field names exactly
  - `bridge/pkg/rpc/browser.go` — example of typed request structs with json tags — follow this pattern
  - `bridge/pkg/provisioning/rpc.go` — example of typed request/response pairs — follow this pattern

  **WHY Each Reference Matters**:
  - `bridgeApi.ts`: Source of truth for param field names
  - `browser.go` and `provisioning/rpc.go`: Existing patterns for typed RPC structs in this codebase

  **Acceptance Criteria**:

  **QA Scenarios (MANDATORY):**

  ```
  Scenario: Types compile and JSON tags match TS contract
    Tool: Bash (go build)
    Preconditions: Go build environment
    Steps:
      1. cd bridge && go build ./pkg/rpc/...
      2. Verify governance_types.go compiles without errors
      3. Run go vet on the file
    Expected Result: Build succeeds, vet passes
    Failure Indicators: Compilation error, vet warning
    Evidence: .sisyphus/evidence/task-4-types-compile.txt

  Scenario: JSON unmarshaling matches admin panel params
    Tool: Bash (go test)
    Preconditions: Go build environment
    Steps:
      1. cd bridge && go test -v -run TestGovernanceTypes ./pkg/rpc/...
      2. Test: unmarshal `{"device_id":"dev_123","approved_by":"admin"}` into DeviceApproveRequest
      3. Test: unmarshal `{"role":"user","expiration":"7d","max_uses":5,"created_by":"admin"}` into InviteCreateRequest
      4. Assert fields populated correctly
    Expected Result: All JSON unmarshaling tests pass
    Failure Indicators: Zero values after unmarshal, wrong field mapping
    Evidence: .sisyphus/evidence/task-4-types-unmarshal.txt
  ```

  **Commit**: YES
  - Message: `feat(rpc): add device and invite request/response types`
  - Files: `bridge/pkg/rpc/governance_types.go`
  - Pre-commit: `cd bridge && go build ./pkg/rpc/...`

- [x] 5. Device RPC Handlers

  **What to do**:
  - Create `bridge/pkg/rpc/device_handlers.go` with 4 handler functions
  - `handleDeviceList(ctx, req)` → call DeviceStore.ListDevices(), return `[]Device` as JSON array
  - `handleDeviceApprove(ctx, req)` → parse DeviceApproveRequest, call DeviceStore.UpdateTrustState(id, "verified"), return `{success: true}`
  - `handleDeviceReject(ctx, req)` → parse DeviceRejectRequest, call DeviceStore.UpdateTrustState(id, "rejected"), return `{success: true}`
  - `handleDeviceGet(ctx, req)` → parse DeviceGetRequest, call DeviceStore.GetDevice(id), return Device or error if not found
  - Register all 4 methods in `bridge/pkg/rpc/server.go:registerHandlers()` (add to the map at lines 833-918)
  - Follow existing handler pattern: `func(ctx context.Context, req *Request) (interface{}, *ErrorObj)` per server.go:132-134
  - Parameter validation: return JSON-RPC error `-32602` with descriptive message for missing required fields
  - `approved_by` and `rejected_by` fields in request are currently hardcoded to `'admin'` by the TS client — accept whatever is sent
  - Error for non-existent device: return JSON-RPC error `-32000` with message "device not found"
  - Idempotency: approving an already-approved device should succeed (return success: true), not error

  **Must NOT do**:
  - Add `device.revoke` (not in TS client)
  - Add authentication logic inside handlers (handled by middleware)
  - Over-validate beyond required fields

  **Recommended Agent Profile**:
  - **Category**: `unspecified-high`
    - Reason: Multiple handler implementations with careful JSON shape alignment, error handling
  - **Skills**: [`git-master`]

  **Parallelization**:
  - **Can Run In Parallel**: YES (with Task 6, but both depend on Wave 1)
  - **Parallel Group**: Wave 2 (with Task 6, 7)
  - **Blocks**: Task 8
  - **Blocked By**: Tasks 1 (auth), 2 (device store), 4 (types)

  **References**:

  **Pattern References**:
  - `bridge/pkg/rpc/server.go:833-918` — `registerHandlers()` — add `"device.list": s.handleDeviceList`, etc. to this map
  - `bridge/pkg/rpc/server.go:132-134` — handler signature `func(ctx context.Context, req *Request) (interface{}, *ErrorObj)`
  - `bridge/pkg/rpc/email_approval.go:40-96` — existing mutation handler pattern (approve_email, deny_email) — follow this

  **API/Type References**:
  - `bridge/pkg/rpc/governance_types.go` (from Task 4) — request/response types
  - `bridge/pkg/trust/device_store.go` (from Task 2) — DeviceStore CRUD methods
  - `applications/admin-panel/src/services/bridgeApi.ts` — Device type shape, method params

  **WHY Each Reference Matters**:
  - `server.go:833-918`: Where handlers are registered. Must add to this map.
  - `email_approval.go`: The most similar existing handlers — approval/rejection with success response.
  - `device_store.go`: The persistence layer these handlers call into.

  **Acceptance Criteria**:

  **QA Scenarios (MANDATORY):**

  ```
  Scenario: device.list returns empty array when no devices
    Tool: Bash (curl)
    Preconditions: Bridge running, valid admin token, empty device store
    Steps:
      1. curl -s -X POST http://localhost:8080/api -H "Authorization: Bearer aat_xxx" -H "Content-Type: application/json" -d '{"jsonrpc":"2.0","id":1,"method":"device.list"}'
      2. Assert .result is [] (empty array)
      3. Assert .error is null/absent
    Expected Result: {"jsonrpc":"2.0","id":1,"result":[]}
    Failure Indicators: .error present, .result is null (not []), Method not found
    Evidence: .sisyphus/evidence/task-5-device-list-empty.txt

  Scenario: device.approve transitions pending device
    Tool: Bash (curl)
    Preconditions: Bridge running, device with trust_state "pending_approval" exists in store
    Steps:
      1. curl -s -X POST http://localhost:8080/api -H "Authorization: Bearer aat_xxx" -H "Content-Type: application/json" -d '{"jsonrpc":"2.0","id":1,"method":"device.approve","params":{"device_id":"dev_test123","approved_by":"admin"}}'
      2. Assert .result.success is true
      3. Query device.list and find device with trust_state "verified"
    Expected Result: Device trust_state changes to "verified"
    Failure Indicators: .result.success is false, device state unchanged, Method not found
    Evidence: .sisyphus/evidence/task-5-device-approve.txt

  Scenario: device.reject with missing device_id returns error
    Tool: Bash (curl)
    Preconditions: Bridge running, valid admin token
    Steps:
      1. curl -s -X POST http://localhost:8080/api -H "Authorization: Bearer aat_xxx" -H "Content-Type: application/json" -d '{"jsonrpc":"2.0","id":1,"method":"device.reject","params":{"rejected_by":"admin","reason":"test"}}'
      2. Assert .error.code is -32602 (invalid params)
    Expected Result: Error response indicating missing device_id
    Failure Indicators: Success response, or silent accept without device_id
    Evidence: .sisyphus/evidence/task-5-device-reject-missing-id.txt

  Scenario: device.approve nonexistent device returns error
    Tool: Bash (curl)
    Preconditions: Bridge running, valid admin token, no device with given ID
    Steps:
      1. curl -s -X POST http://localhost:8080/api -H "Authorization: Bearer aat_xxx" -H "Content-Type: application/json" -d '{"jsonrpc":"2.0","id":1,"method":"device.approve","params":{"device_id":"nonexistent","approved_by":"admin"}}'
      2. Assert .error.code is -32000 with message containing "not found"
    Expected Result: Device not found error
    Failure Indicators: .result.success is true (created ghost device?)
    Evidence: .sisyphus/evidence/task-5-device-approve-notfound.txt

  Scenario: Approving already-approved device is idempotent
    Tool: Bash (curl)
    Preconditions: Bridge running, device already in "verified" state
    Steps:
      1. curl -s -X POST http://localhost:8080/api -H "Authorization: Bearer aat_xxx" -H "Content-Type: application/json" -d '{"jsonrpc":"2.0","id":1,"method":"device.approve","params":{"device_id":"dev_already","approved_by":"admin"}}'
      2. Assert .result.success is true (no error)
    Expected Result: Idempotent success
    Failure Indicators: Error response (e.g. "already approved")
    Evidence: .sisyphus/evidence/task-5-device-approve-idempotent.txt
  ```

  **Commit**: YES
  - Message: `feat(rpc): implement device governance RPC handlers`
  - Files: `bridge/pkg/rpc/device_handlers.go, bridge/pkg/rpc/server.go`
  - Pre-commit: `cd bridge && go build ./pkg/rpc/... && go test ./pkg/rpc/...`

- [x] 6. Invite RPC Handlers

  **What to do**:
  - Create `bridge/pkg/rpc/invite_handlers.go` with 4 handler functions
  - `handleInviteList(ctx, req)` → call InviteStore.ListInvites(), return `[]Invite`
  - `handleInviteCreate(ctx, req)` → parse InviteCreateRequest, generate code via crypto/rand, set status "active", parse expiration string, call InviteStore.CreateInvite(), return full Invite object
  - `handleInviteRevoke(ctx, req)` → parse InviteRevokeRequest, verify invite exists and is active, set status "revoked", return `{success: true}`
  - `handleInviteValidate(ctx, req)` → parse InviteValidateRequest, look up invite by code, check status is "active", check not expired, check use_count < max_uses, return Invite. If invalid: return error with reason (expired/revoked/exhausted/not found)
  - Register all 4 methods in `bridge/pkg/rpc/server.go:registerHandlers()`
  - Follow existing handler signature and error patterns
  - `invite.validate` must be admin-gated (in DefaultAdminMethods) — it returns full Invite including created_by
  - Error codes: `-32602` for invalid params, `-32000` for invite not found/expired/revoked
  - Idempotent revoke: revoking already-revoked invite returns success

  **Must NOT do**:
  - Make invite.validate a public endpoint (must require auth)
  - Generate invite codes with math/rand (use crypto/rand)

  **Recommended Agent Profile**:
  - **Category**: `unspecified-high`
    - Reason: Multiple handlers with business logic (expiration, code gen, status transitions)
  - **Skills**: [`git-master`]

  **Parallelization**:
  - **Can Run In Parallel**: YES (with Task 5, 7)
  - **Parallel Group**: Wave 2 (with Tasks 5, 7)
  - **Blocks**: Task 8
  - **Blocked By**: Tasks 1 (auth), 3 (invite store), 4 (types)

  **References**:

  **Pattern References**:
  - `bridge/pkg/rpc/server.go:833-918` — registerHandlers() map
  - `bridge/pkg/rpc/device_handlers.go` (from Task 5) — sibling handler file, follow same pattern
  - `bridge/pkg/rpc/email_approval.go` — mutation handler pattern

  **API/Type References**:
  - `bridge/pkg/rpc/governance_types.go` (from Task 4) — InviteCreateRequest etc.
  - `bridge/pkg/invite/store.go` (from Task 3) — InviteStore CRUD methods
  - `applications/admin-panel/src/services/bridgeApi.ts` — Invite type, method params

  **WHY Each Reference Matters**:
  - `server.go`: Registration point.
  - `device_handlers.go`: Same pattern, different domain — consistency.
  - `invite/store.go`: The persistence layer these handlers call.

  **Acceptance Criteria**:

  **QA Scenarios (MANDATORY):**

  ```
  Scenario: invite.create returns valid invite with code
    Tool: Bash (curl)
    Preconditions: Bridge running, valid admin token
    Steps:
      1. curl -s -X POST http://localhost:8080/api -H "Authorization: Bearer aat_xxx" -H "Content-Type: application/json" -d '{"jsonrpc":"2.0","id":1,"method":"invite.create","params":{"role":"user","expiration":"7d","max_uses":5,"created_by":"admin"}}'
      2. Assert .result.code is non-empty string (>= 16 chars)
      3. Assert .result.status is "active"
      4. Assert .result.role is "user"
      5. Assert .result.max_uses is 5
      6. Assert .result.use_count is 0
    Expected Result: Full Invite object with generated code
    Failure Indicators: Missing code, wrong status, wrong types
    Evidence: .sisyphus/evidence/task-6-invite-create.txt

  Scenario: invite.list returns created invites
    Tool: Bash (curl)
    Preconditions: Bridge running, at least one invite created
    Steps:
      1. Create an invite (as above)
      2. curl -s -X POST http://localhost:8080/api -H "Authorization: Bearer aat_xxx" -H "Content-Type: application/json" -d '{"jsonrpc":"2.0","id":1,"method":"invite.list"}'
      3. Assert .result is array with length >= 1
      4. Assert first element has .code matching created invite
    Expected Result: Array containing created invite
    Failure Indicators: Empty array after creation, Method not found
    Evidence: .sisyphus/evidence/task-6-invite-list.txt

  Scenario: invite.revoke changes status to revoked
    Tool: Bash (curl)
    Preconditions: Bridge running, active invite exists
    Steps:
      1. Create invite, capture invite_id from .result.id
      2. curl -s -X POST http://localhost:8080/api -H "Authorization: Bearer aat_xxx" -H "Content-Type: application/json" -d '{"jsonrpc":"2.0","id":1,"method":"invite.revoke","params":{"invite_id":"INVITE_ID","revoked_by":"admin"}}'
      3. Assert .result.success is true
      4. List invites, find the invite, assert .status is "revoked"
    Expected Result: Invite status changes to "revoked"
    Failure Indicators: Status unchanged, error response
    Evidence: .sisyphus/evidence/task-6-invite-revoke.txt

  Scenario: invite.validate returns invite for valid code
    Tool: Bash (curl)
    Preconditions: Bridge running, active invite with known code
    Steps:
      1. Create invite, capture .result.code
      2. curl -s -X POST http://localhost:8080/api -H "Authorization: Bearer aat_xxx" -H "Content-Type: application/json" -d '{"jsonrpc":"2.0","id":1,"method":"invite.validate","params":{"code":"THE_CODE"}}'
      3. Assert .result.code matches
      4. Assert .result.status is "active"
    Expected Result: Full Invite object for valid code
    Failure Indicators: Not found error, wrong invite returned
    Evidence: .sisyphus/evidence/task-6-invite-validate.txt

  Scenario: invite.validate fails for revoked invite
    Tool: Bash (curl)
    Preconditions: Bridge running, revoked invite with known code
    Steps:
      1. Create invite, revoke it, capture code
      2. curl -s -X POST http://localhost:8080/api -H "Authorization: Bearer aat_xxx" -H "Content-Type: application/json" -d '{"jsonrpc":"2.0","id":1,"method":"invite.validate","params":{"code":"REVOKED_CODE"}}'
      3. Assert .error.message contains "revoked"
    Expected Result: Error indicating invite is revoked
    Failure Indicators: Returns .result (full invite) for revoked code
    Evidence: .sisyphus/evidence/task-6-invite-validate-revoked.txt

  Scenario: invite.create with missing required field returns error
    Tool: Bash (curl)
    Preconditions: Bridge running, valid admin token
    Steps:
      1. curl -s -X POST http://localhost:8080/api -H "Authorization: Bearer aat_xxx" -H "Content-Type: application/json" -d '{"jsonrpc":"2.0","id":1,"method":"invite.create","params":{"role":"user"}}'
      2. Assert .error.code is -32602
    Expected Result: Invalid params error (missing expiration, max_uses)
    Failure Indicators: Success with defaults, or panic
    Evidence: .sisyphus/evidence/task-6-invite-create-missing.txt
  ```

  **Commit**: YES
  - Message: `feat(rpc): implement invite governance RPC handlers`
  - Files: `bridge/pkg/rpc/invite_handlers.go, bridge/pkg/rpc/server.go`
  - Pre-commit: `cd bridge && go build ./pkg/rpc/... && go test ./pkg/rpc/...`

- [x] 7. Audit Integration for Governance Mutations

  **What to do**:
  - Read `bridge/pkg/audit/audit.go` — existing AuditLog with `LogEvent()` method, `EventType` constants
  - Add governance event types: `"device.approved"`, `"device.rejected"`, `"invite.created"`, `"invite.revoked"`
  - Update `device_handlers.go` (from Task 5) — after successful approve/reject, call `auditLog.LogEvent()` with: event type, user_id (from approved_by/rejected_by), device_id, timestamp
  - Update `invite_handlers.go` (from Task 6) — after successful create/revoke, call `auditLog.LogEvent()` with: event type, user_id (from created_by/revoked_by), invite_id, timestamp
  - The Server struct needs access to AuditLog — check how existing handlers access it (likely `s.auditLog` field)
  - Audit must happen AFTER the mutation succeeds (don't audit failures)
  - Audit entries must include: who (actor), what (action), which (entity ID), when (timestamp)

  **Must NOT do**:
  - Audit failed mutations (only audit successes)
  - Create a new audit system (use existing AuditLog)
  - Add audit to read operations (device.list, invite.list — no mutation)

  **Recommended Agent Profile**:
  - **Category**: `unspecified-high`
    - Reason: Cross-cutting concern, touches both handler files
  - **Skills**: [`git-master`]

  **Parallelization**:
  - **Can Run In Parallel**: YES (with Tasks 5, 6)
  - **Parallel Group**: Wave 2 (with Tasks 5, 6)
  - **Blocks**: None
  - **Blocked By**: Tasks 2 (device store), 3 (invite store)

  **References**:

  **Pattern References**:
  - `bridge/pkg/audit/audit.go` — `AuditLog`, `LogEvent()`, `EventType` — use this directly
  - `bridge/pkg/rpc/email_approval.go` — existing audit pattern for email approval mutations

  **API/Type References**:
  - `bridge/pkg/rpc/device_handlers.go` (from Task 5) — add audit calls after approve/reject
  - `bridge/pkg/rpc/invite_handlers.go` (from Task 6) — add audit calls after create/revoke

  **WHY Each Reference Matters**:
  - `audit.go`: The audit infrastructure. Must use the same event type pattern and LogEvent method.
  - `email_approval.go`: Shows how existing handlers write audit entries.

  **Acceptance Criteria**:

  **QA Scenarios (MANDATORY):**

  ```
  Scenario: Device approve creates audit entry
    Tool: Bash (go test + curl)
    Preconditions: Bridge running with audit log, device in pending state
    Steps:
      1. Call device.approve via curl
      2. Read audit log file
      3. Search for entry with event_type "device.approved" and device_id matching
    Expected Result: Audit entry exists with correct device_id, actor, and timestamp
    Failure Indicators: No audit entry, wrong event type, missing fields
    Evidence: .sisyphus/evidence/task-7-audit-device-approve.txt

  Scenario: Invite create creates audit entry
    Tool: Bash (go test + curl)
    Preconditions: Bridge running with audit log
    Steps:
      1. Call invite.create via curl
      2. Read audit log
      3. Search for "invite.created" entry with invite_id
    Expected Result: Audit entry exists for invite creation
    Failure Indicators: No audit entry for create
    Evidence: .sisyphus/evidence/task-7-audit-invite-create.txt

  Scenario: Failed mutation does not create audit entry
    Tool: Bash (curl)
    Preconditions: Bridge running, device that does not exist
    Steps:
      1. Call device.approve with nonexistent device_id (should return error)
      2. Read audit log
      3. Assert no "device.approved" entry for the nonexistent ID
    Expected Result: No audit entry for failed mutation
    Failure Indicators: Audit entry created despite failure
    Evidence: .sisyphus/evidence/task-7-audit-no-failures.txt
  ```

  **Commit**: YES
  - Message: `feat(audit): integrate audit logging for governance mutations`
  - Files: `bridge/pkg/rpc/device_handlers.go, bridge/pkg/rpc/invite_handlers.go, bridge/pkg/audit/audit.go`
  - Pre-commit: `cd bridge && go build ./... && go test ./pkg/rpc/...`

- [x] 8. Governance Event Emission

  **What to do**:
  - Define event type constants in a new file `bridge/pkg/rpc/governance_events.go`:
    - `EventDeviceApproved = "app.armorclaw.device.approved"`
    - `EventDeviceRejected = "app.armorclaw.device.rejected"`
    - `EventInviteCreated = "app.armorclaw.invite.created"`
    - `EventInviteRevoked = "app.armorclaw.invite.revoked"`
  - Create helper functions: `emitDeviceEvent(matrix MatrixSender, roomID, eventType, deviceID, actor)` and `emitInviteEvent(matrix MatrixSender, roomID, eventType, inviteID, actor, code)`
  - Update device_handlers.go — after successful approve, call `emitDeviceEvent(EventDeviceApproved, ...)`. After reject, call `emitDeviceEvent(EventDeviceRejected, ...)`
  - Update invite_handlers.go — after successful create, call `emitInviteEvent(EventInviteCreated, ...)`. After revoke, call `emitInviteEvent(EventInviteRevoked, ...)`
  - Follow the email approval pattern from `bridge/pkg/rpc/email_approval.go:157-177`: use `MatrixAdapter.SendEvent(roomID, eventType, content)` where content is a JSON map with: `event_type`, `device_id`/`invite_id`, `actor`, `timestamp`
  - The Server struct must have access to the MatrixAdapter (check how existing handlers get it — likely `s.matrix`)
  - Use the admin/owner's room ID for event emission (the room the Bridge shares with the owner)
  - Event emission should be best-effort: log error but don't fail the RPC if event send fails
  - No event emission for read operations (list, get, validate)

  **Must NOT do**:
  - Fail the RPC if event emission fails (best-effort)
  - Emit events for read operations
  - Add event types to the Matrix sync filter (these are outbound-only events)

  **Recommended Agent Profile**:
  - **Category**: `unspecified-high`
    - Reason: Event pattern integration across handler files, MatrixAdapter usage
  - **Skills**: [`git-master`]

  **Parallelization**:
  - **Can Run In Parallel**: YES (with Tasks 9, 10)
  - **Parallel Group**: Wave 3 (with Tasks 9, 10)
  - **Blocks**: Task 11
  - **Blocked By**: Tasks 5, 6 (need handlers to exist)

  **References**:

  **Pattern References**:
  - `bridge/pkg/rpc/email_approval.go:157-177` — `emitEmailApprovalRequestEvent()` — EXACT pattern to follow
  - `bridge/pkg/email/hitl_approval.go:77` — Matrix event emission with `sendMatrixMsg` callback
  - `bridge/internal/adapter/matrix.go` — `SendEvent()` API — the MatrixAdapter method to call

  **API/Type References**:
  - `bridge/pkg/rpc/device_handlers.go` (from Task 5) — add emit calls after mutations
  - `bridge/pkg/rpc/invite_handlers.go` (from Task 6) — add emit calls after mutations
  - `bridge/pkg/notification/alert_types.go` — `app.armorclaw.alert` event type — naming convention reference

  **WHY Each Reference Matters**:
  - `email_approval.go:157-177`: This is the exact template. Copy the pattern: build content map → marshal → call SendEvent.
  - `matrix.go:SendEvent`: The actual API for sending custom Matrix events.
  - `alert_types.go`: Shows the established `app.armorclaw.*` naming convention.

  **Acceptance Criteria**:

  **QA Scenarios (MANDATORY):**

  ```
  Scenario: Device approve emits app.armorclaw.device.approved event
    Tool: Bash (go test)
    Preconditions: Go test environment, mock MatrixAdapter
    Steps:
      1. Run test that calls handleDeviceApprove with valid device
      2. Assert mock MatrixAdapter.SendEvent was called with eventType "app.armorclaw.device.approved"
      3. Assert event content contains device_id and actor
    Expected Result: SendEvent called with correct event type and payload
    Failure Indicators: SendEvent not called, wrong event type, missing fields
    Evidence: .sisyphus/evidence/task-8-event-device-approved.txt

  Scenario: Invite create emits app.armorclaw.invite.created event
    Tool: Bash (go test)
    Preconditions: Go test environment, mock MatrixAdapter
    Steps:
      1. Run test that calls handleInviteCreate with valid params
      2. Assert SendEvent called with "app.armorclaw.invite.created"
      3. Assert content contains invite_id, code, role
    Expected Result: SendEvent called with correct event type
    Failure Indicators: No event emitted for create
    Evidence: .sisyphus/evidence/task-8-event-invite-created.txt

  Scenario: Event emission failure does not fail RPC
    Tool: Bash (go test)
    Preconditions: Go test environment, mock MatrixAdapter that returns error on SendEvent
    Steps:
      1. Configure mock SendEvent to return error
      2. Call handleDeviceApprove
      3. Assert RPC still returns success: true
      4. Assert error was logged (not silently swallowed)
    Expected Result: RPC succeeds despite event failure
    Failure Indicators: RPC returns error because event failed
    Evidence: .sisyphus/evidence/task-8-event-failure-resilient.txt
  ```

  **Commit**: YES
  - Message: `feat(events): emit governance events for device and invite mutations`
  - Files: `bridge/pkg/rpc/governance_events.go, bridge/pkg/rpc/device_handlers.go, bridge/pkg/rpc/invite_handlers.go`
  - Pre-commit: `cd bridge && go build ./... && go test ./pkg/rpc/...`

- [x] 9. Update Admin Panel RPC Client Auth

  **What to do**:
  - Read `applications/admin-panel/src/services/bridgeApi.ts:154-183` — the `rpc()` method that sends JSON-RPC requests
  - Add `Authorization` header to the `fetch()` call: read token from localStorage key `admin_token`
  - If no token exists in localStorage, still send the request (server will reject with auth error, which is correct)
  - The token format from provisioning is `aat_` prefix — the header should be `Authorization: Bearer <token>`
  - Do NOT change the LoginPage.tsx auth flow (still stubbed) — that's a separate task
  - Do NOT add token refresh logic — out of scope
  - Verify the Vite proxy config (`vite.config.ts`) still proxies `/api` to `http://localhost:8080` correctly with the new header

  **Must NOT do**:
  - Change login page behavior (separate task)
  - Add token refresh or expiry handling
  - Change the RPC method names or param shapes

  **Recommended Agent Profile**:
  - **Category**: `quick`
    - Reason: Small targeted change to one function in one file
  - **Skills**: [`git-master`]

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 3 (with Tasks 8, 10)
  - **Blocks**: None
  - **Blocked By**: Task 1 (auth middleware must exist for this to make sense)

  **References**:

  **Pattern References**:
  - `applications/admin-panel/src/services/bridgeApi.ts:154-183` — `rpc()` method — add Authorization header here

  **API/Type References**:
  - `applications/admin-panel/src/App.tsx:58-93` — `AuthProvider` stores token in localStorage `admin_token` — read from this key

  **WHY Each Reference Matters**:
  - `bridgeApi.ts:rpc()`: The exact function to modify. Add one header to the fetch options.
  - `App.tsx:AuthProvider`: Where the token is stored. Must read from the same key.

  **Acceptance Criteria**:

  **QA Scenarios (MANDATORY):**

  ```
  Scenario: RPC client sends Authorization header
    Tool: Playwright (browser)
    Preconditions: Admin panel dev server running, token in localStorage
    Steps:
      1. Open browser to admin panel
      2. Set localStorage admin_token to "aat_test_token_12345"
      3. Navigate to Devices page
      4. Intercept fetch request to /api
      5. Assert request includes header "Authorization: Bearer aat_test_token_12345"
    Expected Result: Authorization header present in request
    Failure Indicators: No Authorization header, wrong format
    Evidence: .sisyphus/evidence/task-9-client-auth-header.png

  Scenario: RPC client works without token
    Tool: Playwright (browser)
    Preconditions: Admin panel dev server running, NO token in localStorage
    Steps:
      1. Clear localStorage
      2. Navigate to Devices page
      3. Intercept fetch request
      4. Assert request still sent (server will reject, but client shouldn't crash)
    Expected Result: Request sent without Authorization header, client handles error gracefully
    Failure Indicators: Client crashes, request not sent
    Evidence: .sisyphus/evidence/task-9-client-no-token.txt
  ```

  **Commit**: YES
  - Message: `fix(admin-panel): send Authorization header in RPC client`
  - Files: `applications/admin-panel/src/services/bridgeApi.ts`
  - Pre-commit: `cd applications/admin-panel && npx tsc --noEmit`

- [x] 10. Update review.md Accuracy

  **What to do**:
  - Read `review.md` line 67: `"Admin panel uses mock data instead of real Bridge API ✅ Replaced with real RPC calls in v0.7.0"`
  - Update this entry to accurately reflect reality: the admin panel has typed RPC client code but the server handlers were not implemented until this work
  - Add a new Known Gap entry: "Admin panel device/invite RPC handlers — implemented in v0.8.0"
  - Update the Active Plan section to reflect v0.8.0 governance work
  - Do NOT change any security constraints or architectural decisions

  **Must NOT do**:
  - Change security constraints or architectural decisions
  - Over-document (keep it concise like existing entries)

  **Recommended Agent Profile**:
  - **Category**: `quick`
    - Reason: Simple documentation accuracy fix
  - **Skills**: [`git-master`]

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 3 (with Tasks 8, 9)
  - **Blocks**: None
  - **Blocked By**: None

  **References**:

  **Pattern References**:
  - `review.md` — follow existing format for Known Gaps entries

  **Acceptance Criteria**:

  **QA Scenarios (MANDATORY):**

  ```
  Scenario: review.md accurately reflects admin panel status
    Tool: Bash (grep)
    Preconditions: File exists
    Steps:
      1. grep -n "admin panel" review.md (case insensitive)
      2. Assert no claim that admin panel uses "real Bridge API" without qualification
      3. Assert device/invite governance mentioned in Known Gaps or completed items
    Expected Result: Accurate documentation
    Failure Indicators: Old misleading claim still present
    Evidence: .sisyphus/evidence/task-10-review-accuracy.txt
  ```

  **Commit**: YES
  - Message: `docs: correct review.md claim about admin panel RPC status`
  - Files: `review.md`
  - Pre-commit: none

- [x] 11. Create RPC API Reference Documentation

  **What to do**:
  - Create `docs/reference/rpc-api.md` — the file referenced in README but currently missing
  - Document ALL existing RPC methods (82 methods) organized by domain:
    - System (health.check, system.*)
    - Bridge Control (bridge.*)
    - Browser Automation (browser.*)
    - PII/HITL (pii.*)
    - Email Approval (approve_email, deny_email, email.*)
    - Skills (skills.*)
    - Matrix (matrix.*)
    - Events (events.*)
    - Studio (studio.*)
    - Containers (container.*)
    - Provisioning (provisioning.*)
    - Hardening (hardening.*)
    - Secretary/Tasks (secretary.*, task.*)
    - Account (account.*)
    - **NEW: Device Governance** (device.list, device.get, device.approve, device.reject)
    - **NEW: Invite Governance** (invite.create, invite.list, invite.revoke, invite.validate)
  - For each method document:
    - Method name
    - Authentication required (public, admin token)
    - Request parameters with types and descriptions
    - Response shape with types
    - Error codes and meanings
    - Example request/response (curl)
  - Focus detailed documentation on the NEW governance methods (device.*, invite.*)
  - For existing methods, provide a concise summary table — full documentation can be added incrementally
  - Follow a clean markdown format consistent with the README style

  **Must NOT do**:
  - Auto-generate from code (write human-readable docs)
  - Document methods that don't exist yet (lockdown, security, adapter, etc.)
  - Over-document — keep it reference-quality, not tutorial-style

  **Recommended Agent Profile**:
  - **Category**: `writing`
    - Reason: Technical documentation task
  - **Skills**: [`git-master`]

  **Parallelization**:
  - **Can Run In Parallel**: NO
  - **Parallel Group**: Wave 4 (solo)
  - **Blocks**: None
  - **Blocked By**: Tasks 5, 6, 8 (need implemented methods to document)

  **References**:

  **Pattern References**:
  - `README.md` — follow the documentation style and formatting conventions
  - `bridge/pkg/rpc/server.go:833-918` — method registration list (source of truth for method names)

  **API/Type References**:
  - `bridge/pkg/rpc/device_handlers.go` (from Task 5) — device method signatures and behaviors
  - `bridge/pkg/rpc/invite_handlers.go` (from Task 6) — invite method signatures and behaviors
  - `bridge/pkg/rpc/governance_types.go` (from Task 4) — request/response type definitions
  - `bridge/pkg/auth/matrix_auth.go:376-389` — DefaultAdminMethods list (which methods require auth)
  - `bridge/pkg/rpc/public_handlers.go` — public methods (no auth required)

  **WHY Each Reference Matters**:
  - `server.go:registerHandlers()`: The definitive list of all RPC methods — must document all of these.
  - Handler files: Provide actual request/response shapes and error codes.
  - `matrix_auth.go`: Documents which methods require authentication.

  **Acceptance Criteria**:

  **QA Scenarios (MANDATORY):**

  ```
  Scenario: RPC API docs file exists and covers governance methods
    Tool: Bash (grep)
    Preconditions: File created
    Steps:
      1. test -f docs/reference/rpc-api.md && echo "EXISTS"
      2. grep -c "device.list" docs/reference/rpc-api.md
      3. grep -c "invite.create" docs/reference/rpc-api.md
      4. grep -c "Authentication" docs/reference/rpc-api.md
    Expected Result: File exists, contains device.list and invite.create, mentions authentication
    Failure Indicators: File missing, governance methods not documented
    Evidence: .sisyphus/evidence/task-11-rpc-docs.txt

  Scenario: Documentation matches actual handler behavior
    Tool: Bash (curl + grep)
    Preconditions: Bridge running, docs file exists
    Steps:
      1. Read the documented request shape for device.approve from docs
      2. Send a curl request matching that shape
      3. Verify response matches documented response shape
    Expected Result: Documentation matches reality
    Failure Indicators: Docs say different field names than actual responses
    Evidence: .sisyphus/evidence/task-11-docs-match.txt
  ```

  **Commit**: YES
  - Message: `docs: create RPC API reference documentation`
  - Files: `docs/reference/rpc-api.md`
  - Pre-commit: none

---

## Final Verification Wave (MANDATORY — after ALL implementation tasks)

- [x] F1. **Plan Compliance Audit** — `oracle`
  Read the plan end-to-end. For each "Must Have": verify implementation exists (read file, curl endpoint, run command). For each "Must NOT Have": search codebase for forbidden patterns — reject with file:line if found. Check evidence files exist in .sisyphus/evidence/. Compare deliverables against plan.
  Output: `Must Have [N/N] | Must NOT Have [N/N] | Tasks [N/N] | VERDICT: APPROVE/REJECT`

- [x] F2. **Code Quality Review** — `unspecified-high`
  Run `go vet ./...` + `go build ./...` + `go test ./pkg/rpc/... ./pkg/trust/... ./pkg/invite/...`. Review all changed files for: `as any`/type assertions without checks, empty catches, fmt.Println in prod, commented-out code, unused imports. Check AI slop: excessive comments, over-abstraction, generic names.
  Output: `Build [PASS/FAIL] | Vet [PASS/FAIL] | Tests [N pass/N fail] | Files [N clean/N issues] | VERDICT`

- [x] F3. **Real Manual QA** — `unspecified-high`
  Start from clean state. Execute EVERY QA scenario from EVERY task — follow exact steps, capture evidence. Test cross-task integration: create device → approve → verify in list. Create invite → revoke → validate fails. Test edge cases: approve already-approved device, revoke used invite, expired invite validation. Save to `.sisyphus/evidence/final-qa/`.
  Output: `Scenarios [N/N pass] | Integration [N/N] | Edge Cases [N tested] | VERDICT`

- [x] F4. **Scope Fidelity Check** — `deep`
  For each task: read "What to do", read actual diff (git log/diff). Verify 1:1 — everything in spec was built (no missing), nothing beyond spec was built (no creep). Check "Must NOT do" compliance. Detect cross-task contamination. Flag unaccounted changes.
  Output: `Tasks [N/N compliant] | Contamination [CLEAN/N issues] | Unaccounted [CLEAN/N files] | VERDICT`

---

## Commit Strategy

- **Task 1**: `feat(auth): wire RPCAuthMiddleware to /api with admin token validation` - bridge/pkg/auth/matrix_auth.go, bridge/pkg/http/server.go
- **Task 2**: `feat(trust): add SQLCipher device store persistence` - bridge/pkg/trust/device_store.go
- **Task 3**: `feat(invite): add SQLCipher invite store persistence` - bridge/pkg/invite/store.go
- **Task 4**: `feat(rpc): add device and invite request/response types` - bridge/pkg/rpc/governance_types.go
- **Task 5**: `feat(rpc): implement device governance RPC handlers` - bridge/pkg/rpc/device_handlers.go, bridge/pkg/rpc/server.go
- **Task 6**: `feat(rpc): implement invite governance RPC handlers` - bridge/pkg/rpc/invite_handlers.go, bridge/pkg/rpc/server.go
- **Task 7**: `feat(audit): integrate audit logging for governance mutations` - bridge/pkg/rpc/device_handlers.go, bridge/pkg/rpc/invite_handlers.go
- **Task 8**: `feat(events): emit governance events for device and invite mutations` - bridge/pkg/rpc/device_handlers.go, bridge/pkg/rpc/invite_handlers.go
- **Task 9**: `fix(admin-panel): send Authorization header in RPC client` - applications/admin-panel/src/services/bridgeApi.ts
- **Task 10**: `docs: correct review.md claim about admin panel RPC status` - review.md
- **Task 11**: `docs: create RPC API reference documentation` - docs/reference/rpc-api.md

---

## Success Criteria

### Verification Commands
```bash
# Build
cd bridge && go build ./...  # Expected: no errors

# Tests
cd bridge && go test -v ./pkg/rpc/... ./pkg/trust/... ./pkg/invite/...  # Expected: PASS

# Auth gate (no token)
curl -s -X POST http://localhost:8080/api -d '{"jsonrpc":"2.0","id":1,"method":"device.list"}' | jq .error.code  # Expected: auth error code

# Auth gate (valid token)
curl -s -X POST http://localhost:8080/api -H "Authorization: Bearer aat_xxx" -d '{"jsonrpc":"2.0","id":1,"method":"device.list"}' | jq .result  # Expected: []

# Device flow
curl -s -X POST http://localhost:8080/api -H "Authorization: Bearer aat_xxx" -d '{"jsonrpc":"2.0","id":1,"method":"device.approve","params":{"device_id":"dev_123","approved_by":"admin"}}' | jq .result.success  # Expected: true

# Invite flow
curl -s -X POST http://localhost:8080/api -H "Authorization: Bearer aat_xxx" -d '{"jsonrpc":"2.0","id":1,"method":"invite.create","params":{"role":"user","expiration":"7d","max_uses":5,"created_by":"admin"}}' | jq .result.code  # Expected: invite code string
```

### Final Checklist
- [ ] All "Must Have" present
- [ ] All "Must NOT Have" absent
- [ ] All tests pass
- [ ] Admin panel TypeScript shapes match Go JSON responses exactly
- [ ] SQLCipher stores share keystore DB connection
- [ ] Auth middleware wired (not duplicated)
- [ ] Governance events emitted for all mutations
- [ ] review.md accurately reflects implementation status
- [ ] docs/reference/rpc-api.md documents all RPC methods
