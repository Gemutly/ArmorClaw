# First-Login Hardening Wizard

## TL;DR

> **Quick Summary**: Implement a mandatory security ceremony that forces ArmorClaw users to harden their setup (rotate bootstrap password, verify device, backup recovery key) before delegation or agent operations are allowed. Spans Go Bridge (state model, RPC handlers, delegation gate) and Kotlin ArmorChat (navigation infrastructure, wizard screens, biometric integration).
> 
> **Deliverables**:
> - `bridge/pkg/trust/hardening.go` — Hardening state model + store
> - `bridge/pkg/keystore/keystore.go` — New hardening_state table
> - `bridge/internal/adapter/matrix.go` — ChangePassword method
> - `bridge/pkg/rpc/server.go` — Hardening RPC handlers registered
> - Delegation gate wired into existing RPC handlers
> - `MainActivity.kt` + NavHost + route definitions (new navigation infrastructure)
> - `HardeningWizardViewModel.kt` + wizard step screens
> - BiometricPrompt integration (optional step)
> - Onboarding flow merged: Bonding → Rotate Password → Verify Device → KeyBackup → Biometrics → SecurityConfig
>
> **Estimated Effort**: Large (25-35 hours)
> **Parallel Execution**: YES - 4 waves
> **Critical Path**: T1 (types) → T7 (store) → T9 (gate) → T14 (wire flow) → FINAL

---

## Context

### Original Request
Implement a "First-Login Hardening Wizard" that forces the transition from insecure Bootstrap Trust to Sovereign Trust before allowing delegation or high-risk agent operations. 5-step ceremony: rotate password, wipe bootstrap file, verify device, backup recovery key, enable biometrics (optional). Must gate Team Builder and all delegation endpoints.

### Interview Summary
**Key Discussions**:
- **Trust package**: Extend existing `bridge/pkg/trust/` (already has zero_trust.go, device.go) — add `hardening.go`
- **Navigation**: Include ArmorChat navigation infrastructure (no MainActivity/NavHost exists)
- **Password rotation**: Add `ChangePassword` to MatrixAdapter (follows existing HTTP pattern)
- **Biometrics**: Optional/skippable step (dependency present but zero implementation)
- **Test strategy**: TDD (write failing test first, then implement)
- **Onboarding integration**: Merge into existing flow (absorb KeyBackup, add new steps around it)
- **State persistence**: New table in keystore DB (SQLCipher encrypted)
- **Flow order**: Bonding → Rotate Password → Wipe Bootstrap (auto) → Verify Device → KeyBackup (existing) → Biometrics (optional) → SecurityConfig → Delegation unlocked

**Research Findings**:
- `bridge/pkg/trust/` already exists with ZeroTrustManager (device fingerprinting, risk scoring) and Manager (device registration, approval)
- `bridge/internal/wizard/` exists (14K+ lines, setup-time wizard — hardening is separate post-login flow)
- ArmorChat has NO navigation system (no MainActivity, no NavHost, no routes — dependency present but unused)
- Biometrics dependency present (`biometric:1.2.0-alpha05`) but zero implementation
- MatrixAdapter has no ChangePassword method — uses direct HTTP, not SDK
- BridgeApi uses HTTP JSON-RPC 2.0 over OkHttp (not socket-based RPC)
- `/var/lib/armorclaw/.admin_password` exists per review.md:2559, should be deleted after first login
- Keystore uses `CREATE TABLE IF NOT EXISTS` pattern — never ALTER TABLE

### Metis Review
**Identified Gaps** (addressed):
- **Existing user migration**: Default to grandfathered (existing users skip hardening, new users only) — resolved via DEFAULT values in keystore table
- **Password rotation atomicity**: Sequential with fail-fast; if Matrix change fails, nothing else happens; if keystore update fails, user retries with error message
- **Mid-process failure**: Resume at failed step (state persists in keystore, wizard reads on launch)
- **Biometrics skip semantics**: "Enable later via settings" — skipping doesn't block completion
- **Device verification**: Self-verification using existing emoji verification flow (first device, no other device to verify against)
- **Delegation gate enforcement**: Backend enforcement in RPC handlers (not UI-only)
- **State ownership**: Bridge owns authoritative state (keystore DB); Android reads via RPC

**Guardrails Applied**:
- MUST NOT modify existing zero_trust.go logic — call it, don't rewrite
- MUST NOT modify existing KeyBackup logic — integrate into flow, don't replace
- MUST NOT break existing SecurityConfig screen — preserve backward compatibility
- MUST NOT implement biometrics fallback UI — simple skip only
- MUST NOT add hardening for existing users — DEFAULT values mark them as complete
- MUST follow `CREATE TABLE IF NOT EXISTS` pattern for keystore table
- MUST follow provisioning/rpc.go handler pattern for RPC registration
- MUST follow ArmorChat ViewModel StateFlow + sealed class pattern

---

## Work Objectives

### Core Objective
Force all new ArmorClaw users through a mandatory security hardening ceremony before allowing delegation, team management, or high-risk agent operations.

### Concrete Deliverables
- Hardening state model persisted in SQLCipher keystore
- RPC handlers: `hardening.status`, `hardening.ack`, `hardening.rotate_password`
- Delegation gate: backend enforcement in all delegation/agent management RPC handlers
- ArmorChat navigation infrastructure (MainActivity, NavHost, routes)
- Hardening wizard screens: PasswordRotation, DeviceVerification, BiometricEnable
- Onboarding flow: Bonding → Rotate Password → Verify Device → KeyBackup → Biometrics → SecurityConfig
- ChangePassword method in MatrixAdapter

### Definition of Done
- [ ] `go test ./pkg/trust/...` passes (hardening model tests)
- [ ] `go test ./pkg/keystore/...` passes (hardening_state table tests)
- [ ] `go test ./pkg/rpc/...` passes (hardening RPC handler tests)
- [ ] `go build ./cmd/bridge` succeeds
- [ ] ArmorChat builds and launches with MainActivity
- [ ] New user completes hardening wizard on first login
- [ ] Delegation RPC returns error before hardening completes
- [ ] Delegation RPC succeeds after hardening completes
- [ ] Existing users are NOT forced through hardening (grandfathered)

### Must Have
- Backend enforcement of delegation gate (not UI-only)
- Hardening state persisted in SQLCipher keystore
- Password rotation with Matrix password change + bootstrap file deletion
- Device verification step (using existing emoji verification)
- Recovery key backup step (reusing existing KeyBackupSetupScreen)
- Biometrics as optional step (can be skipped)
- Navigation infrastructure for ArmorChat (prerequisite for wizard flow)
- Existing users grandfathered (no forced hardening)

### Must NOT Have (Guardrails)
- **DO NOT** modify existing `zero_trust.go` or `device.go` logic
- **DO NOT** modify existing `KeyBackupSetupScreen` logic (only integrate into flow)
- **DO NOT** break existing `SecurityConfigScreen` backward compatibility
- **DO NOT** implement biometrics fallback UI (simple skip button only)
- **DO NOT** force hardening on existing users (DEFAULT values in keystore)
- **DO NOT** use ALTER TABLE on existing keystore schema
- **DO NOT** implement audit logging for hardening steps
- **DO NOT** add hardening state history/analytics
- **DO NOT** implement admin bypass mechanisms
- **DO NOT** implement multi-device hardening coordination
- **DO NOT** modify existing BondingScreen beyond adding hardening entry point

---

## Verification Strategy

> **ZERO HUMAN INTERVENTION** — ALL verification is agent-executed. No exceptions.

### Test Decision
- **Infrastructure exists**: YES (Go: testify, 85 test files; Kotlin: JUnit 4 + Mockk, 10 test files)
- **Automated tests**: YES (TDD — write failing test first, then implement)
- **Framework**: Go: `go test` + testify; Kotlin: JUnit 4 + Mockk + Compose UI testing
- **If TDD**: Each task follows RED (failing test) → GREEN (minimal impl) → REFACTOR

### QA Policy
Every task MUST include agent-executed QA scenarios. Evidence saved to `.sisyphus/evidence/task-{N}-{scenario-slug}.{ext}`.

- **Go Backend**: Bash (`go test`) — Run unit tests, assert pass/fail
- **Android UI**: Bash (`adb`) — Install APK, launch activity, verify UI elements
- **API/RPC**: Bash (`socat` or `curl`) — Send RPC requests, assert responses
- **Integration**: Bash — Start bridge, run Android, verify end-to-end flow

---

## Execution Strategy

### Parallel Execution Waves

```
Wave 1 (Foundation — 6 tasks, T1-T4 and T6 independent, T5 depends on T1+T3):
├── Task 1: Hardening state types (bridge/pkg/trust/hardening.go) [quick]
├── Task 2: Keystore hardening_state table (bridge/pkg/keystore/keystore.go) [quick]
├── Task 3: MatrixAdapter.ChangePassword (bridge/internal/adapter/matrix.go) [quick]
├── Task 4: ArmorChat navigation infrastructure (new files) [unspecified-high]
├── Task 5: Hardening RPC handlers (bridge/pkg/rpc/server.go + hardening_handlers.go) [unspecified-high] — starts after T1+T3
└── Task 6: BridgeApi hardening methods (ArmorChat network/BridgeApi.kt) [quick]

Wave 2 (Core Implementation — 5 tasks, depends on Wave 1):
├── Task 7: Hardening Store implementation (bridge/pkg/trust/hardening.go) [unspecified-high]
├── Task 8: Delegation gate (bridge/pkg/rpc/) [deep]
├── Task 9: HardeningWizardViewModel (ArmorChat viewmodel/) [unspecified-high]
├── Task 10: PasswordRotationScreen (ArmorChat ui/security/) [visual-engineering]
└── Task 11: BiometricEnableScreen (ArmorChat ui/security/) [visual-engineering]

Wave 3 (Integration — 4 tasks, depends on Wave 2):
├── Task 12: Onboarding flow wiring (ArmorChat navigation + existing screens) [deep]
├── Task 13: DeviceVerification step integration (reuse BridgeVerificationScreen) [unspecified-high]
├── Task 14: Delegation gate wiring into RPC handlers [deep]
└── Task 15: BridgeApi hardening methods + tests (ArmorChat) [unspecified-high]

Wave FINAL (4 parallel reviews):
├── Task F1: Plan compliance audit (oracle)
├── Task F2: Code quality review (unspecified-high)
├── Task F3: Integration test (unspecified-high)
└── Task F4: Scope fidelity check (deep)

Critical Path: T1 → T2 → T7 → T8 → T14 → FINAL
Parallel Speedup: ~65% faster than sequential
Max Concurrent: 6 (Wave 1)
```

### Dependency Matrix

| Task | Depends On | Blocks |
|------|------------|--------|
| 1 | — | 5, 7 |
| 2 | — | 7 |
| 3 | — | 5 |
| 4 | — | 9, 12, 13 |
| 5 | 1, 3 | 8, 14, 15 |
| 6 | — | 9, 15 |
| 7 | 1, 2 | 8, 9 |
| 8 | 5, 7 | 14 |
| 9 | 4, 6, 7 | 12, 13 |
| 10 | 4, 9 | 12 |
| 11 | 4, 9 | 12 |
| 12 | 4, 9, 10, 11, 13 | FINAL |
| 13 | 4, 9 | 12 |
| 14 | 5, 8 | FINAL |
| 15 | 5, 6 | FINAL |

### Agent Dispatch Summary

- **Wave 1**: 6 agents — T1→quick, T2→quick, T3→quick, T4→unspecified-high, T5→unspecified-high, T6→quick
- **Wave 2**: 5 agents — T7→unspecified-high, T8→deep, T9→unspecified-high, T10→visual-engineering, T11→visual-engineering
- **Wave 3**: 4 agents — T12→deep, T13→unspecified-high, T14→deep, T15→unspecified-high
- **FINAL**: 4 agents — F1→oracle, F2→unspecified-high, F3→unspecified-high, F4→deep

---

## TODOs

- [ ] 1. Define Hardening State Types

  **What to do**:
  - Create `bridge/pkg/trust/hardening.go` with hardening state types
  - Define `HardeningStep` enum: PASSWORD_ROTATED, BOOTSTRAP_WIPED, DEVICE_VERIFIED, RECOVERY_BACKED_UP, BIOMETRICS_ENABLED
  - Define `UserHardeningState` struct with user_id, completed_steps map, delegation_ready bool, created_at, updated_at
  - Define `Store` interface: Get(userID), Put(state), IsDelegationReady(userID)
  - Write tests FIRST: `bridge/pkg/trust/hardening_test.go`
  - TDD: Test that UserHardeningState.Recompute() correctly sets delegation_ready based on completed steps
  - TDD: Test that delegation_ready requires password_rotated AND bootstrap_wiped AND device_verified AND recovery_backed_up (NOT biometrics)
  - Do NOT implement Store — that's Task 7

  **Must NOT do**:
  - DO NOT modify existing zero_trust.go or device.go
  - DO NOT create a separate package — this lives in bridge/pkg/trust/
  - DO NOT implement the Store interface (Task 7)

  **Recommended Agent Profile**:
  - **Category**: `quick`
    - Reason: Single file, well-defined types, follows existing patterns
  - **Skills**: []
  - **Skills Evaluated but Omitted**:
    - `brainstorming`: Types are clearly defined, no creative exploration needed

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 1 (with Tasks 2, 3, 4, 5, 6)
  - **Blocks**: Task 5, Task 7
  - **Blocked By**: None

  **References**:
  - `bridge/pkg/trust/zero_trust.go:17-26` — TrustScore enum pattern to follow for HardeningStep
  - `bridge/pkg/trust/device.go:14-44` — TrustState enum pattern to follow
  - `bridge/pkg/trust/device.go:82-113` — TrustConfig struct pattern to follow for HardeningConfig

  **WHY Each Reference Matters**:
  - zero_trust.go TrustScore: Shows the idiomatic way this package defines enums and string methods
  - device.go TrustState: Shows the state machine pattern used in this package
  - device.go TrustConfig: Shows config struct pattern with defaults

  **Acceptance Criteria**:
  - [ ] Test file created: `bridge/pkg/trust/hardening_test.go`
  - [ ] `cd bridge && go test -v -run TestHardening ./pkg/trust/...` → PASS
  - [ ] UserHardeningState has all 5 step fields + delegation_ready computed field
  - [ ] delegation_ready is true ONLY when 4 mandatory steps complete (biometrics optional)
  - [ ] Store interface defined with Get, Put, IsDelegationReady methods

  **QA Scenarios (MANDATORY):**

  ```
  Scenario: Hardening state computes delegation_ready correctly
    Tool: Bash (go test)
    Steps:
      1. Run: cd bridge && go test -v -run TestDelegationReady ./pkg/trust/...
      2. Verify test creates state with password_rotated=true, bootstrap_wiped=true, device_verified=true, recovery_backed_up=true, biometrics_enabled=false
      3. Assert delegation_ready == true
    Expected Result: PASS — delegation_ready true with 4 mandatory steps, biometrics=false
    Evidence: .sisyphus/evidence/task-1-delegation-ready.txt

  Scenario: Hardening state requires all mandatory steps
    Tool: Bash (go test)
    Steps:
      1. Run: cd bridge && go test -v -run TestIncompleteHardening ./pkg/trust/...
      2. Verify test creates state with only 3 of 4 mandatory steps
      3. Assert delegation_ready == false
    Expected Result: PASS — delegation_ready false when any mandatory step missing
    Evidence: .sisyphus/evidence/task-1-incomplete.txt
  ```

  **Commit**: YES
  - Message: `feat(hardening): define hardening state types and Store interface`
  - Files: `bridge/pkg/trust/hardening.go`, `bridge/pkg/trust/hardening_test.go`

- [ ] 2. Add Hardening State Table to Keystore

  **What to do**:
  - Add `hardening_state` table to `bridge/pkg/keystore/keystore.go` initSchema function
  - Schema: `CREATE TABLE IF NOT EXISTS hardening_state (user_id TEXT PRIMARY KEY, password_rotated INTEGER DEFAULT 0, bootstrap_wiped INTEGER DEFAULT 0, device_verified INTEGER DEFAULT 0, recovery_backed_up INTEGER DEFAULT 0, biometrics_enabled INTEGER DEFAULT 0, delegation_ready INTEGER DEFAULT 0, created_at INTEGER NOT NULL, updated_at INTEGER NOT NULL)`
  - Write tests FIRST: extend `bridge/pkg/keystore/keystore_test.go`
  - TDD: Test that table is created on init
  - TDD: Test that DEFAULT values are 0 (existing users NOT forced through hardening)
  - TDD: Test basic CRUD: insert state, read state, update step
  - Use existing `CreateTableIfNotExists` pattern from keystore.go

  **Must NOT do**:
  - DO NOT use ALTER TABLE on any existing table
  - DO NOT modify existing tables or their schemas
  - DO NOT implement the hardening Store (that's Task 7 — this is just the table)

  **Recommended Agent Profile**:
  - **Category**: `quick`
    - Reason: Single schema addition following established pattern
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 1 (with Tasks 1, 3, 4, 5, 6)
  - **Blocks**: Task 7
  - **Blocked By**: None

  **References**:
  - `bridge/pkg/keystore/keystore.go` — initSchema function with CREATE TABLE IF NOT EXISTS pattern
  - `bridge/pkg/keystore/keystore_test.go` — Test pattern using t.TempDir() and require.NoError

  **WHY Each Reference Matters**:
  - keystore.go initSchema: Shows exact SQL pattern to follow — every table uses CREATE TABLE IF NOT EXISTS
  - keystore_test.go: Shows how to create temp DB for testing, assertion style

  **Acceptance Criteria**:
  - [ ] hardening_state table created in initSchema
  - [ ] `cd bridge && go test -v -run TestHardeningState ./pkg/keystore/...` → PASS
  - [ ] DEFAULT values are 0 for all step columns
  - [ ] Existing tests still pass: `cd bridge && go test ./pkg/keystore/...`

  **QA Scenarios (MANDATORY):**

  ```
  Scenario: Keystore creates hardening_state table on init
    Tool: Bash (go test)
    Steps:
      1. Run: cd bridge && go test -v -run TestHardeningTable ./pkg/keystore/...
      2. Verify test opens keystore, checks table exists via PRAGMA table_info
    Expected Result: PASS — table exists with correct columns
    Evidence: .sisyphus/evidence/task-2-table-created.txt

  Scenario: Existing users have default zero values
    Tool: Bash (go test)
    Steps:
      1. Run: cd bridge && go test -v -run TestDefaultHardening ./pkg/keystore/...
      2. Verify test creates keystore, reads hardening state for new user, all values are 0
    Expected Result: PASS — no user is forced into hardening state
    Evidence: .sisyphus/evidence/task-2-default-values.txt
  ```

  **Commit**: YES
  - Message: `feat(keystore): add hardening_state table with safe defaults`
  - Files: `bridge/pkg/keystore/keystore.go`, `bridge/pkg/keystore/keystore_test.go`
  - Pre-commit: `cd bridge && go test ./pkg/keystore/...`

- [ ] 3. Add ChangePassword to MatrixAdapter

  **What to do**:
  - Add `ChangePassword(ctx context.Context, userID, newPassword string, logoutDevices bool) error` to `bridge/internal/adapter/matrix.go`
  - Call Conduit's Matrix Client-Server API: `POST /_matrix/client/v3/account/password`
  - Request body: `{"new_password": "xxx", "logout_devices": false}`
  - Use existing `m.accessToken` for authentication
  - Write tests FIRST: add test cases to existing adapter test file
  - TDD: Test successful password change with httptest mock server
  - TDD: Test error handling for 401 (invalid token) and 400 (weak password)
  - Default `logoutDevices` to `false` (don't invalidate other sessions)

  **Must NOT do**:
  - DO NOT use any Matrix SDK — this codebase uses direct HTTP calls only
  - DO NOT modify existing adapter methods
  - DO NOT implement re-authentication flow (that's out of scope)

  **Recommended Agent Profile**:
  - **Category**: `quick`
    - Reason: Single method addition following existing HTTP pattern
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 1 (with Tasks 1, 2, 4, 5, 6)
  - **Blocks**: Task 5
  - **Blocked By**: None

  **References**:
  - `bridge/internal/adapter/matrix.go:33-62` — MatrixAdapter struct showing accessToken field
  - `bridge/internal/adapter/matrix.go` — Login method pattern (HTTP POST to Matrix API)
  - `bridge/pkg/appservice/integration_test.go` — httptest mock server pattern

  **WHY Each Reference Matters**:
  - MatrixAdapter struct: Shows where accessToken is stored, needed for auth header
  - Login method: Shows exact pattern for HTTP calls to Matrix API (URL construction, auth header, JSON body)
  - integration_test.go: Shows how to mock Matrix server responses for testing

  **Acceptance Criteria**:
  - [ ] `ChangePassword` method added to MatrixAdapter
  - [ ] `cd bridge && go test -v -run TestChangePassword ./internal/adapter/...` → PASS
  - [ ] Uses POST /_matrix/client/v3/account/password with auth header
  - [ ] logoutDevices defaults to false
  - [ ] Existing adapter tests still pass

  **QA Scenarios (MANDATORY):**

  ```
  Scenario: ChangePassword calls correct Matrix API endpoint
    Tool: Bash (go test)
    Steps:
      1. Run: cd bridge && go test -v -run TestChangePassword ./internal/adapter/...
      2. Verify mock server received POST to /_matrix/client/v3/account/password
      3. Verify request body contains new_password field
    Expected Result: PASS — correct endpoint called with correct payload
    Evidence: .sisyphus/evidence/task-3-password-change.txt

  Scenario: ChangePassword handles 401 unauthorized
    Tool: Bash (go test)
    Steps:
      1. Run: cd bridge && go test -v -run TestChangePasswordUnauthorized ./internal/adapter/...
      2. Verify error returned for 401 response
    Expected Result: PASS — error returned, no panic
    Evidence: .sisyphus/evidence/task-3-unauthorized.txt
  ```

  **Commit**: YES
  - Message: `feat(matrix): add ChangePassword method to MatrixAdapter`
  - Files: `bridge/internal/adapter/matrix.go`
  - Pre-commit: `cd bridge && go test ./internal/adapter/...`

- [ ] 4. Create ArmorChat Navigation Infrastructure

  **What to do**:
  - Create `applications/ArmorChat/app/src/main/java/app/armorclaw/MainActivity.kt`
    - setContent with ArmorClawTheme wrapping NavHost
    - Single Activity, Compose-only
  - Create `applications/ArmorChat/app/src/main/java/app/armorclaw/navigation/ArmorClawNavHost.kt`
    - Define routes: `bonding`, `security_config`, `key_backup`, `key_recovery`, `hardening_password`, `hardening_device`, `hardening_biometrics`, `home`
    - NavHost with NavController, rememberNavController
    - Start destination based on hardening state
  - Create `applications/ArmorChat/app/src/main/java/app/armorclaw/navigation/Route.kt`
    - Sealed class or object defining all routes with arguments
  - Update `AndroidManifest.xml` to point to new MainActivity
  - Write tests FIRST for navigation
  - TDD: Test that NavHost renders start destination
  - TDD: Test route navigation

  **Must NOT do**:
  - DO NOT add animations, transitions, or complex navigation patterns
  - DO NOT create a dependency injection framework (Hilt/Dagger) — use manual DI
  - DO NOT modify existing screen composables (BondingScreen, SecurityConfigScreen, etc.) — just wrap them in navigation
  - DO NOT implement deep links (out of scope)

  **Recommended Agent Profile**:
  - **Category**: `unspecified-high`
    - Reason: Multiple new files, Android-specific, Compose Navigation setup
  - **Skills**: []
  - **Skills Evaluated but Omitted**:
    - `brainstorming`: Navigation patterns are standard Compose Navigation

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 1 (with Tasks 1, 2, 3, 5, 6)
  - **Blocks**: Task 9, Task 12, Task 13
  - **Blocked By**: None

  **References**:
  - `applications/ArmorChat/app/build.gradle.kts:102` — `navigation-compose:2.7.6` dependency (present but unused)
  - `applications/ArmorChat/app/src/main/AndroidManifest.xml` — Current manifest (references missing MainActivity)
  - `applications/ArmorChat/app/src/main/java/app/armorclaw/viewmodel/BondingViewModel.kt` — ViewModel pattern for injection
  - `applications/ArmorChat/app/src/main/java/app/armorclaw/ui/security/BondingScreen.kt` — Screen composable pattern to wrap

  **WHY Each Reference Matters**:
  - build.gradle.kts: Confirms navigation-compose dependency is already available
  - AndroidManifest.xml: Must update activity reference to new MainActivity
  - BondingViewModel.kt: Shows how ViewModels are instantiated (manual DI, no Hilt)
  - BondingScreen.kt: Shows the composable signature pattern (callback-based navigation) to wrap in routes

  **Acceptance Criteria**:
  - [ ] MainActivity.kt created with setContent + ArmorClawTheme
  - [ ] NavHost defined with all routes
  - [ ] Route.kt defines all navigation destinations
  - [ ] AndroidManifest.xml updated to reference new MainActivity
  - [ ] Existing screens render correctly when wrapped in navigation

  **QA Scenarios (MANDATORY):**

  ```
  Scenario: MainActivity launches and shows start destination
    Tool: Bash (go test / gradle)
    Steps:
      1. Build: cd applications/ArmorChat && ./gradlew assembleDebug
      2. Verify APK builds successfully
    Expected Result: BUILD SUCCESSFUL
    Evidence: .sisyphus/evidence/task-4-build.txt

  Scenario: NavHost contains all required routes
    Tool: Bash (grep)
    Steps:
      1. grep -r "composable(" applications/ArmorChat/app/src/main/java/app/armorclaw/navigation/ArmorClawNavHost.kt
      2. Verify routes: bonding, security_config, key_backup, hardening_password, hardening_device, hardening_biometrics, home
    Expected Result: All routes present in NavHost
    Evidence: .sisyphus/evidence/task-4-routes.txt
  ```

  **Commit**: YES
  - Message: `feat(android): add MainActivity and Compose Navigation infrastructure`
  - Files: `MainActivity.kt`, `navigation/ArmorClawNavHost.kt`, `navigation/Route.kt`, `AndroidManifest.xml`

- [ ] 5. Add Hardening RPC Handlers to Bridge

  **What to do**:
  - Create `bridge/pkg/rpc/hardening_handlers.go` with 3 handlers:
    - `handleHardeningStatus` — Returns UserHardeningState for authenticated user
    - `handleHardeningAck` — Records completion of a hardening step
    - `handleHardeningRotatePassword` — Changes Matrix password and deletes bootstrap file
  - Register handlers in `bridge/pkg/rpc/server.go` registerHandlers():
    - `"hardening.status"` → handleHardeningStatus
    - `"hardening.ack"` → handleHardeningAck
    - `"hardening.rotate_password"` → handleHardeningRotatePassword
  - Follow existing handler pattern from `bridge/pkg/rpc/server.go:117` (HandlerFunc signature)
  - Follow provisioning RPC pattern from `bridge/pkg/provisioning/rpc.go` (separate handler struct)
  - Write tests FIRST: `bridge/pkg/rpc/hardening_handlers_test.go`
  - TDD: Test handler registration (method exists in handlers map)
  - TDD: Test hardening.status returns correct structure
  - TDD: Test hardening.ack with valid step name
  - TDD: Test hardening.ack with invalid step name → error
  - TDD: Test hardening.rotate_password calls ChangePassword and deletes bootstrap file
  - For now, handlers use a mock store (real store wired in Task 7)

  **Must NOT do**:
  - DO NOT implement the Store — use an interface that Task 7 will implement
  - DO NOT modify existing RPC handler registration for non-hardening methods
  - DO NOT implement delegation gate logic (that's Task 8)

  **Recommended Agent Profile**:
  - **Category**: `unspecified-high`
    - Reason: Multiple handlers, RPC pattern matching, test coverage
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 1 (with Tasks 1, 2, 3, 4, 6)
  - **Blocks**: Task 8, Task 14, Task 15
  - **Blocked By**: Task 1 (types), Task 3 (ChangePassword)

  **References**:
  - `bridge/pkg/rpc/server.go:117` — HandlerFunc type signature: `func(ctx context.Context, req *Request) (interface{}, *ErrorObj)`
  - `bridge/pkg/rpc/server.go:687-746` — registerHandlers() pattern: map[string]HandlerFunc
  - `bridge/pkg/rpc/bridge_handlers.go:99-117` — Parameter unmarshaling pattern
  - `bridge/pkg/provisioning/rpc.go:22-30` — Method name constants pattern
  - `bridge/pkg/rpc/server_test.go` — Test pattern for method registration

  **WHY Each Reference Matters**:
  - HandlerFunc signature: Every handler MUST match this exact signature
  - registerHandlers: Shows how to add new methods to the handlers map
  - Parameter unmarshaling: Shows the json.Unmarshal pattern for request params
  - Method name constants: Shows naming convention (dot-separated, lowercase)
  - server_test.go: Shows how to test that methods are registered

  **Acceptance Criteria**:
  - [ ] `bridge/pkg/rpc/hardening_handlers.go` created with 3 handlers
  - [ ] 3 methods registered in registerHandlers(): hardening.status, hardening.ack, hardening.rotate_password
  - [ ] `cd bridge && go test -v -run TestHardening ./pkg/rpc/...` → PASS
  - [ ] Existing tests still pass: `cd bridge && go test ./pkg/rpc/...`
  - [ ] `cd bridge && go build ./cmd/bridge` succeeds

  **QA Scenarios (MANDATORY):**

  ```
  Scenario: Hardening RPC handlers registered correctly
    Tool: Bash (go test)
    Steps:
      1. Run: cd bridge && go test -v -run TestHardeningHandlers ./pkg/rpc/...
      2. Verify all 3 methods found in server.handlers map
    Expected Result: PASS — hardening.status, hardening.ack, hardening.rotate_password all registered
    Evidence: .sisyphus/evidence/task-5-handler-registration.txt

  Scenario: hardening.ack rejects invalid step name
    Tool: Bash (go test)
    Steps:
      1. Run: cd bridge && go test -v -run TestHardeningAckInvalid ./pkg/rpc/...
      2. Verify error returned for step="nonexistent_step"
    Expected Result: PASS — error with "unknown step" message
    Evidence: .sisyphus/evidence/task-5-invalid-step.txt
  ```

  **Commit**: YES
  - Message: `feat(rpc): add hardening status, ack, and rotate_password handlers`
  - Files: `bridge/pkg/rpc/hardening_handlers.go`, `bridge/pkg/rpc/hardening_handlers_test.go`, `bridge/pkg/rpc/server.go`
  - Pre-commit: `cd bridge && go test ./pkg/rpc/... && go build ./cmd/bridge`

- [ ] 6. Add Hardening Methods to ArmorChat BridgeApi

  **What to do**:
  - Add 3 methods to `applications/ArmorChat/app/src/main/java/app/armorclaw/network/BridgeApi.kt`:
    - `getHardeningStatus(): Result<HardeningStatus>` — Calls `hardening.status`
    - `acknowledgeHardeningStep(step: String): Result<HardeningStatus>` — Calls `hardening.ack`
    - `rotateBootstrapPassword(newPassword: String): Result<Map<String, Boolean>>` — Calls `hardening.rotate_password`
  - Add response data classes: `HardeningStatus` with fields matching Go UserHardeningState
  - Add to `@Serializable` annotation classes following existing pattern
  - Write tests FIRST: `applications/ArmorChat/app/src/test/java/app/armorclaw/network/HardeningApiTest.kt`
  - TDD: Test request structure matches JSON-RPC 2.0 format
  - TDD: Test response deserialization

  **Must NOT do**:
  - DO NOT modify existing BridgeApi methods
  - DO NOT create a separate BridgeRpcClient class (use existing BridgeApi)

  **Recommended Agent Profile**:
  - **Category**: `quick`
    - Reason: 3 method additions following existing pattern
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 1 (with Tasks 1, 2, 3, 4, 5)
  - **Blocks**: Task 9, Task 15
  - **Blocked By**: None

  **References**:
  - `applications/ArmorChat/app/src/main/java/app/armorclaw/network/BridgeApi.kt:17` — BridgeApi class with rpc() inline function
  - `applications/ArmorChat/app/src/main/java/app/armorclaw/network/BridgeApi.kt` — getLockdownStatus() method pattern
  - `applications/ArmorChat/app/src/test/java/app/armorclaw/network/BridgeApiTest.kt` — Test pattern for RPC structure

  **WHY Each Reference Matters**:
  - BridgeApi class: Shows the rpc<T>() inline function pattern all methods must use
  - getLockdownStatus: Shows exact pattern: fun methodName(): Result<ResponseType> = rpc("method.name")
  - BridgeApiTest.kt: Shows how to test RPC request structure without a real server

  **Acceptance Criteria**:
  - [ ] 3 methods added to BridgeApi
  - [ ] HardeningStatus data class defined with @Serializable
  - [ ] `cd applications/ArmorChat && ./gradlew test --tests "app.armorclaw.network.HardeningApiTest"` → PASS
  - [ ] Request format matches JSON-RPC 2.0: {"jsonrpc":"2.0","id":N,"method":"hardening.status","params":{}}

  **QA Scenarios (MANDATORY):**

  ```
  Scenario: Hardening API methods produce correct RPC requests
    Tool: Bash (gradle test)
    Steps:
      1. Run: cd applications/ArmorChat && ./gradlew test --tests "app.armorclaw.network.HardeningApiTest"
      2. Verify request JSON structure matches JSON-RPC 2.0
    Expected Result: PASS — correct method names and params
    Evidence: .sisyphus/evidence/task-6-api-requests.txt

  Scenario: HardeningStatus deserializes correctly
    Tool: Bash (gradle test)
    Steps:
      1. Verify test creates JSON matching Go UserHardeningState
      2. Verify all fields deserialize correctly (booleans, strings, timestamps)
    Expected Result: PASS — all fields correctly mapped
    Evidence: .sisyphus/evidence/task-6-deserialization.txt
  ```

  **Commit**: YES
  - Message: `feat(android): add hardening RPC methods to BridgeApi`
  - Files: `applications/ArmorChat/app/src/main/java/app/armorclaw/network/BridgeApi.kt`, `applications/ArmorChat/app/src/test/java/app/armorclaw/network/HardeningApiTest.kt`

- [ ] 7. Implement Hardening Store

  **What to do**:
  - Implement the `Store` interface defined in Task 1, in `bridge/pkg/trust/hardening.go`
  - `KeystoreHardeningStore` struct that wraps the keystore DB
  - Implement `Get(userID) (*UserHardeningState, error)` — read from hardening_state table
  - Implement `Put(state *UserHardeningState) error` — upsert to hardening_state table
  - Implement `IsDelegationReady(userID) (bool, error)` — query delegation_ready column
  - Implement `AckStep(userID, step string) error` — update specific step column
  - Wire the store into RPC handlers from Task 5 (replace mock store)
  - Write tests FIRST extending `bridge/pkg/trust/hardening_test.go`
  - TDD: Test Get returns correct state after Put
  - TDD: Test AckStep updates only the specified step
  - TDD: Test IsDelegationReady returns correct boolean
  - TDD: Test that DEFAULT values for existing users result in delegation_ready=false (but no error)

  **Must NOT do**:
  - DO NOT modify the keystore table schema (Task 2 already defined it)
  - DO NOT use ALTER TABLE
  - DO NOT add migration scripts

  **Recommended Agent Profile**:
  - **Category**: `unspecified-high`
    - Reason: SQLCipher CRUD implementation with multiple methods
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: NO
  - **Parallel Group**: Wave 2
  - **Blocks**: Task 8, Task 9
  - **Blocked By**: Task 1 (types), Task 2 (table)

  **References**:
  - `bridge/pkg/trust/hardening.go` — Store interface from Task 1
  - `bridge/pkg/keystore/keystore.go` — DB access patterns (Open, Query, Exec)
  - `bridge/pkg/keystore/keystore_test.go` — Test pattern using t.TempDir()
  - `bridge/pkg/studio/store.go` — SQLite CRUD pattern for Agent Studio store

  **WHY Each Reference Matters**:
  - hardening.go Store interface: This is what we're implementing
  - keystore.go: Shows how to access the DB (sql.Open with SQLCipher driver)
  - keystore_test.go: Shows temp dir pattern for test isolation
  - studio/store.go: Shows mature SQLite CRUD pattern in this codebase

  **Acceptance Criteria**:
  - [ ] KeystoreHardeningStore implements Store interface
  - [ ] `cd bridge && go test -v -run TestHardeningStore ./pkg/trust/...` → PASS
  - [ ] Get/Put/AckStep/IsDelegationReady all work correctly
  - [ ] RPC handlers from Task 5 now use real store instead of mock

  **QA Scenarios (MANDATORY):**

  ```
  Scenario: Store persists hardening state across restarts
    Tool: Bash (go test)
    Steps:
      1. Run: cd bridge && go test -v -run TestHardeningStorePersistence ./pkg/trust/...
      2. Verify test creates store, writes state, closes DB, reopens, reads state
    Expected Result: PASS — state survives DB close/reopen
    Evidence: .sisyphus/evidence/task-7-persistence.txt

  Scenario: AckStep updates only specified step
    Tool: Bash (go test)
    Steps:
      1. Run: cd bridge && go test -v -run TestAckStep ./pkg/trust/...
      2. Verify AckStep("password_rotated") sets only password_rotated=1, others remain 0
    Expected Result: PASS — only targeted column updated
    Evidence: .sisyphus/evidence/task-7-ack-step.txt
  ```

  **Commit**: YES
  - Message: `feat(hardening): implement KeystoreHardeningStore with CRUD operations`
  - Files: `bridge/pkg/trust/hardening.go`, `bridge/pkg/trust/hardening_test.go`, `bridge/pkg/rpc/hardening_handlers.go` (wire store)
  - Pre-commit: `cd bridge && go test ./pkg/trust/... ./pkg/rpc/...`

- [ ] 8. Implement Delegation Gate

  **What to do**:
  - Create `bridge/pkg/rpc/delegation_gate.go` with delegation gate logic
  - Define `ErrHardeningRequired` error: "Complete security hardening before performing this action"
  - Implement `RequireDelegationReady(store trust.Store, userID string) error` function
  - Wire gate into existing RPC handlers that perform delegation/agent management:
    - Agent creation: `studio.create_agent` handler
    - Agent invitation: `studio.invite` handler (if exists)
    - Standing approval creation (if exists)
  - Add gate check at the TOP of each handler — return error immediately if not ready
  - Write tests FIRST: `bridge/pkg/rpc/delegation_gate_test.go`
  - TDD: Test that gate passes when delegation_ready=true
  - TDD: Test that gate blocks when delegation_ready=false with ErrHardeningRequired
  - TDD: Test that gate propagates store errors

  **Must NOT do**:
  - DO NOT modify existing capability/policy.go logic
  - DO NOT change existing handler behavior when delegation IS ready
  - DO NOT add UI-only gates — this is backend enforcement only
  - DO NOT implement partial delegation (all-or-nothing based on delegation_ready)

  **Recommended Agent Profile**:
  - **Category**: `deep`
    - Reason: Security-critical gate logic, must be correct, needs thorough testing
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: NO
  - **Parallel Group**: Wave 2
  - **Blocks**: Task 14
  - **Blocked By**: Task 5 (handlers), Task 7 (store)

  **References**:
  - `bridge/pkg/trust/hardening.go` — Store.IsDelegationReady() method
  - `bridge/pkg/rpc/server.go:117` — HandlerFunc signature (gate must be called within handlers)
  - `bridge/pkg/rpc/bridge_handlers.go` — Example handler to add gate to
  - `bridge/internal/capability/policy.go` — Existing policy pattern (inline checks, not centralized)
  - `bridge/pkg/rpc/server_test.go` — Test pattern for handler behavior

  **WHY Each Reference Matters**:
  - Store.IsDelegationReady: The function the gate calls to check state
  - HandlerFunc signature: Gate must return ErrorObj compatible with handler return type
  - bridge_handlers.go: Shows which handlers need gating (agent management, delegation)
  - capability/policy.go: Shows existing inline check pattern — gate follows similar but centralized approach
  - server_test.go: Shows how to test handler error responses

  **Acceptance Criteria**:
  - [ ] `RequireDelegationReady` function implemented
  - [ ] `ErrHardeningRequired` defined with clear error message
  - [ ] Gate wired into at least agent creation handler
  - [ ] `cd bridge && go test -v -run TestDelegationGate ./pkg/rpc/...` → PASS
  - [ ] Agent creation blocked when delegation_ready=false
  - [ ] Agent creation allowed when delegation_ready=true

  **QA Scenarios (MANDATORY):**

  ```
  Scenario: Delegation gate blocks agent creation before hardening
    Tool: Bash (go test)
    Steps:
      1. Run: cd bridge && go test -v -run TestGateBlocks ./pkg/rpc/...
      2. Verify handler returns ErrHardeningRequired when store returns delegation_ready=false
    Expected Result: PASS — error with "Complete security hardening" message
    Evidence: .sisyphus/evidence/task-8-gate-blocks.txt

  Scenario: Delegation gate allows agent creation after hardening
    Tool: Bash (go test)
    Steps:
      1. Run: cd bridge && go test -v -run TestGateAllows ./pkg/rpc/...
      2. Verify handler proceeds normally when store returns delegation_ready=true
    Expected Result: PASS — handler executes without gate error
    Evidence: .sisyphus/evidence/task-8-gate-allows.txt
  ```

  **Commit**: YES
  - Message: `feat(hardening): add delegation gate blocking agent ops until hardening complete`
  - Files: `bridge/pkg/rpc/delegation_gate.go`, `bridge/pkg/rpc/delegation_gate_test.go`, `bridge/pkg/rpc/bridge_handlers.go` (gate wiring)
  - Pre-commit: `cd bridge && go test ./pkg/rpc/...`

- [ ] 9. Create HardeningWizardViewModel for ArmorChat

  **What to do**:
  - Create `applications/ArmorChat/app/src/main/java/app/armorclaw/viewmodel/HardeningWizardViewModel.kt`
  - Follow existing ViewModel pattern from BondingViewModel.kt:
    - StateFlow with sealed class for UI state (NotStarted, Loading, StepCompleted, Error, AllComplete)
    - viewModelScope.launch for coroutines
    - retryWithBackoff for resilience
  - Methods:
    - `loadState()` — Call BridgeApi.getHardeningStatus(), update UI state
    - `rotatePassword(newPassword: String)` — Call BridgeApi.rotateBootstrapPassword(), then loadState()
    - `acknowledgeStep(step: String)` — Call BridgeApi.acknowledgeHardeningStep(), then loadState()
    - `getCurrentStep(): HardeningStep` — Compute next incomplete step from state
    - `isDelegationReady(): Boolean` — Check if all mandatory steps complete
  - Enum `HardeningStep`: ROTATE_PASSWORD, WIPE_BOOTSTRAP, VERIFY_DEVICE, BACKUP_RECOVERY, ENABLE_BIOMETRICS, COMPLETE
  - Write tests FIRST: `applications/ArmorChat/app/src/test/java/app/armorclaw/viewmodel/HardeningWizardViewModelTest.kt`
  - TDD: Test loadState updates UI state from API response
  - TDD: Test getCurrentStep returns correct next step
  - TDD: Test isDelegationReady returns correct boolean
  - Mock BridgeApi using Mockk

  **Must NOT do**:
  - DO NOT add Hilt/Dagger dependency injection — use manual DI
  - DO NOT implement UI logic in ViewModel (state only)

  **Recommended Agent Profile**:
  - **Category**: `unspecified-high`
    - Reason: Multi-method ViewModel with state management and API integration
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: NO
  - **Parallel Group**: Wave 2
  - **Blocks**: Task 10, Task 11, Task 12, Task 13
  - **Blocked By**: Task 4 (navigation), Task 6 (BridgeApi), Task 7 (store)

  **References**:
  - `applications/ArmorChat/app/src/main/java/app/armorclaw/viewmodel/BondingViewModel.kt` — Full ViewModel pattern to follow
  - `applications/ArmorChat/app/src/main/java/app/armorclaw/viewmodel/SecurityConfigViewModel.kt` — StateFlow + sealed class pattern
  - `applications/ArmorChat/app/src/main/java/app/armorclaw/repository/SetupRepository.kt` — Repository pattern wrapping BridgeApi
  - `applications/ArmorChat/app/src/test/java/app/armorclaw/viewmodel/ViewModelTest.kt` — Test pattern with StandardTestDispatcher

  **WHY Each Reference Matters**:
  - BondingViewModel: Shows exact ViewModel structure, coroutine usage, retryWithBackoff
  - SecurityConfigViewModel: Shows sealed class for UI state pattern
  - SetupRepository: Shows how ViewModels call BridgeApi through a repository wrapper
  - ViewModelTest: Shows test setup with StandardTestDispatcher and viewModelScope

  **Acceptance Criteria**:
  - [ ] HardeningWizardViewModel created with all 5 methods
  - [ ] HardeningStep enum defined
  - [ ] UI state sealed class defined
  - [ ] `cd applications/ArmorChat && ./gradlew test --tests "app.armorclaw.viewmodel.HardeningWizardViewModelTest"` → PASS
  - [ ] getCurrentStep returns ROTATE_PASSWORD when no steps completed
  - [ ] getCurrentStep returns COMPLETE when all mandatory steps done
  - [ ] isDelegationReady returns false until 4 mandatory steps complete

  **QA Scenarios (MANDATORY):**

  ```
  Scenario: ViewModel computes correct current step
    Tool: Bash (gradle test)
    Steps:
      1. Run: cd applications/ArmorChat && ./gradlew test --tests "app.armorclaw.viewmodel.HardeningWizardViewModelTest"
      2. Verify getCurrentStep returns ROTATE_PASSWORD when state is all-false
      3. Verify getCurrentStep returns VERIFY_DEVICE when password_rotated=true, bootstrap_wiped=true
    Expected Result: PASS — correct step progression
    Evidence: .sisyphus/evidence/task-9-step-progression.txt

  Scenario: isDelegationReady requires 4 mandatory steps only
    Tool: Bash (gradle test)
    Steps:
      1. Verify isDelegationReady returns true with 4 mandatory steps, biometrics=false
      2. Verify isDelegationReady returns false with 3 mandatory steps, biometrics=true
    Expected Result: PASS — biometrics optional, 4 mandatory required
    Evidence: .sisyphus/evidence/task-9-delegation-ready.txt
  ```

  **Commit**: YES
  - Message: `feat(android): add HardeningWizardViewModel with state management`
  - Files: `applications/ArmorChat/app/src/main/java/app/armorclaw/viewmodel/HardeningWizardViewModel.kt`, test file

- [ ] 10. Create PasswordRotationScreen

  **What to do**:
  - Create `applications/ArmorChat/app/src/main/java/app/armorclaw/ui/security/PasswordRotationScreen.kt`
  - Follow existing screen pattern from BondingScreen.kt:
    - Scaffold with CenterAlignedTopAppBar title="Security Setup"
    - Column with verticalScroll, 24.dp padding
    - Two OutlinedTextField: new password + confirm password
    - Password visibility toggle (trailing icon)
    - Validation: minimum 12 characters, passwords must match
    - Error text: "Password must be at least 12 characters", "Passwords do not match"
    - Button: "Set Password" — enabled only when valid
    - Warning card explaining WHY (bootstrap password is insecure)
  - Call viewModel.rotatePassword(password) on submit
  - Show loading state during rotation, error state on failure

  **Must NOT do**:
  - DO NOT add password strength meter (out of scope)
  - DO NOT add password generation button
  - DO NOT modify existing BondingScreen

  **Recommended Agent Profile**:
  - **Category**: `visual-engineering`
    - Reason: Compose UI screen with form validation and Material3 patterns
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES (with Task 11)
  - **Parallel Group**: Wave 2
  - **Blocks**: Task 12
  - **Blocked By**: Task 4 (navigation), Task 9 (ViewModel)

  **References**:
  - `applications/ArmorChat/app/src/main/java/app/armorclaw/ui/security/BondingScreen.kt` — Screen pattern to follow exactly
  - `applications/ArmorChat/app/src/main/java/app/armorclaw/ui/security/SecurityConfigScreen.kt` — BottomAppBar pattern, step indicator

  **Acceptance Criteria**:
  - [ ] PasswordRotationScreen renders with two password fields
  - [ ] Button disabled when passwords don't match or < 12 chars
  - [ ] Error text shown for validation failures
  - [ ] Loading indicator shown during password rotation

  **QA Scenarios (MANDATORY):**

  ```
  Scenario: Password rotation screen validates input correctly
    Tool: Bash (gradle build)
    Steps:
      1. cd applications/ArmorChat && ./gradlew assembleDebug
      2. Verify APK builds with new screen composable
    Expected Result: BUILD SUCCESSFUL
    Evidence: .sisyphus/evidence/task-10-build.txt
  ```

  **Commit**: YES
  - Message: `feat(android): add PasswordRotationScreen for hardening step 1`

- [ ] 11. Create BiometricEnableScreen

  **What to do**:
  - Create `applications/ArmorChat/app/src/main/java/app/armorclaw/ui/security/BiometricEnableScreen.kt`
  - Follow existing screen pattern:
    - Scaffold with CenterAlignedTopAppBar title="Biometric Lock"
    - Card explaining biometric benefits
    - Button: "Enable Biometrics" — triggers BiometricPrompt
    - TextButton: "Skip for now" — acknowledges step as skipped, proceeds
  - Implement BiometricPrompt integration:
    - Use `androidx.biometric.BiometricPrompt` (already in dependencies)
    - Create `BiometricAuthHelper` utility class for BiometricPrompt management
    - Handle: SUCCESS → acknowledgeStep("biometrics_enabled"), CAN_AUTHENTICATE → show enable button, NO_HARDWARE → auto-skip with message
  - Simple implementation: no crypto binding, no fallback UI, just enable/skip

  **Must NOT do**:
  - DO NOT implement crypto-bound biometric authentication
  - DO NOT implement periodic re-authentication
  - DO NOT create fallback UI for devices without biometrics (just skip)

  **Recommended Agent Profile**:
  - **Category**: `visual-engineering`
    - Reason: Compose UI with Android BiometricPrompt integration
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES (with Task 10)
  - **Parallel Group**: Wave 2
  - **Blocks**: Task 12
  - **Blocked By**: Task 4 (navigation), Task 9 (ViewModel)

  **References**:
  - `applications/ArmorChat/app/build.gradle.kts:127` — `biometric:1.2.0-alpha05` dependency
  - `applications/ArmorChat/app/src/main/AndroidManifest.xml:30` — `USE_BIOMETRIC` permission

  **Acceptance Criteria**:
  - [ ] BiometricEnableScreen renders with enable and skip buttons
  - [ ] BiometricPrompt shows when "Enable Biometrics" tapped
  - [ ] Skip button proceeds without biometrics
  - [ ] No hardware detection: auto-skip with informational message

  **QA Scenarios (MANDATORY):**

  ```
  Scenario: Biometric screen builds and renders
    Tool: Bash (gradle build)
    Steps:
      1. cd applications/ArmorChat && ./gradlew assembleDebug
      2. Verify APK builds with BiometricPrompt integration
    Expected Result: BUILD SUCCESSFUL
    Evidence: .sisyphus/evidence/task-11-build.txt
  ```

  **Commit**: YES
  - Message: `feat(android): add BiometricEnableScreen with optional skip`

- [ ] 12. Wire Onboarding Flow in Navigation

  **What to do**:
  - Update `applications/ArmorChat/app/src/main/java/app/armorclaw/navigation/ArmorClawNavHost.kt`
  - Implement the merged onboarding flow:
    1. BondingScreen (existing) — admin claiming
    2. PasswordRotationScreen (new from Task 10) — step 1
    3. DeviceVerificationScreen (reuse existing BridgeVerificationScreen from Task 13) — step 2
    4. KeyBackupSetupScreen (existing) — step 3 (recovery key)
    5. BiometricEnableScreen (new from Task 11) — step 4 (optional)
    6. SecurityConfigScreen (existing) — data category permissions
    7. HomeScreen — post-hardening
  - Add logic to determine start destination:
    - If hardening state incomplete → start at first incomplete step
    - If hardening complete → start at home
    - If bonding not done → start at bonding
  - Add back navigation between steps
  - Ensure each step calls viewModel.acknowledgeStep() on completion
  - Do NOT modify existing screen composables — only the navigation wiring

  **Must NOT do**:
  - DO NOT modify BondingScreen, KeyBackupSetupScreen, SecurityConfigScreen composables
  - DO NOT add animations or transitions
  - DO NOT implement deep links

  **Recommended Agent Profile**:
  - **Category**: `deep`
    - Reason: Complex flow orchestration connecting 6+ screens with conditional logic
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: NO
  - **Parallel Group**: Wave 3
  - **Blocks**: FINAL
  - **Blocked By**: Task 4, Task 9, Task 10, Task 11, Task 13

  **References**:
  - `applications/ArmorChat/app/src/main/java/app/armorclaw/navigation/ArmorClawNavHost.kt` — NavHost from Task 4
  - `applications/ArmorChat/app/src/main/java/app/armorclaw/ui/security/BondingScreen.kt` — Existing screen callback signatures
  - `applications/ArmorChat/app/src/main/java/app/armorclaw/ui/security/KeyBackupScreen.kt` — Existing screen callback signatures
  - `applications/ArmorChat/app/src/main/java/app/armorclaw/ui/security/SecurityConfigScreen.kt` — Existing screen with step indicator

  **Acceptance Criteria**:
  - [ ] All 6 screens connected in correct order
  - [ ] Start destination determined by hardening state
  - [ ] Back navigation works between steps
  - [ ] Each step acknowledges completion before navigating forward
  - [ ] APK builds: `cd applications/ArmorChat && ./gradlew assembleDebug`

  **QA Scenarios (MANDATORY):**

  ```
  Scenario: Onboarding flow navigates through all steps
    Tool: Bash (gradle build)
    Steps:
      1. cd applications/ArmorChat && ./gradlew assembleDebug
      2. Verify NavHost contains composable for each step in order
    Expected Result: BUILD SUCCESSFUL, all routes present
    Evidence: .sisyphus/evidence/task-12-flow-wiring.txt

  Scenario: Start destination logic handles incomplete hardening
    Tool: Bash (grep)
    Steps:
      1. grep -n "hardening" applications/ArmorChat/app/src/main/java/app/armorclaw/navigation/ArmorClawNavHost.kt
      2. Verify conditional logic for start destination based on hardening state
    Expected Result: Conditional start destination logic present
    Evidence: .sisyphus/evidence/task-12-start-destination.txt
  ```

  **Commit**: YES
  - Message: `feat(android): wire merged onboarding flow with hardening steps`

- [ ] 13. Integrate Device Verification Step

  **What to do**:
  - Wire existing `BridgeVerificationScreen` into the hardening flow
  - The device verification step uses existing emoji verification (this is the FIRST/PRIMARY device)
  - Add route in NavHost for `hardening_device` → composable wrapping BridgeVerificationScreen
  - On verification success → call viewModel.acknowledgeStep("device_verified")
  - On verification skip/failure → show retry option, don't proceed
  - Write minimal test verifying the route exists and calls acknowledgeStep

  **Must NOT do**:
  - DO NOT modify BridgeVerificationScreen composable
  - DO NOT implement new verification logic (reuse existing)

  **Recommended Agent Profile**:
  - **Category**: `unspecified-high`
    - Reason: Integration task connecting existing screen to new flow
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: NO
  - **Parallel Group**: Wave 3
  - **Blocks**: Task 12
  - **Blocked By**: Task 4 (navigation), Task 9 (ViewModel)

  **References**:
  - `applications/ArmorChat/app/src/main/java/app/armorclaw/ui/verification/BridgeVerificationScreen.kt` — Existing verification screen

  **Acceptance Criteria**:
  - [ ] BridgeVerificationScreen accessible via hardening_device route
  - [ ] acknowledgeStep("device_verified") called on success
  - [ ] APK builds successfully

  **QA Scenarios (MANDATORY):**

  ```
  Scenario: Device verification route exists and renders
    Tool: Bash (grep)
    Steps:
      1. grep "hardening_device\|BridgeVerificationScreen" applications/ArmorChat/app/src/main/java/app/armorclaw/navigation/ArmorClawNavHost.kt
    Expected Result: Route found wrapping BridgeVerificationScreen
    Evidence: .sisyphus/evidence/task-13-verification-route.txt
  ```

  **Commit**: YES
  - Message: `feat(android): integrate device verification into hardening flow`

- [ ] 14. Wire Delegation Gate into RPC Handlers

  **What to do**:
  - Add `RequireDelegationReady()` calls to ALL delegation/agent management RPC handlers
  - Identify all handlers that need gating (search for agent creation, invitation, standing approval)
  - Add gate check as first line of each handler
  - Return `ErrHardeningRequired` ErrorObj if gate fails
  - Write integration test: start bridge, create user with incomplete hardening, attempt agent creation, verify error
  - Write integration test: complete hardening, attempt agent creation, verify success

  **Must NOT do**:
  - DO NOT change handler behavior when gate passes
  - DO NOT add new RPC methods (only modify existing ones)

  **Recommended Agent Profile**:
  - **Category**: `deep`
    - Reason: Security-critical wiring, must identify ALL delegation entry points
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: NO
  - **Parallel Group**: Wave 3
  - **Blocks**: FINAL
  - **Blocked By**: Task 5 (handlers), Task 8 (gate)

  **References**:
  - `bridge/pkg/rpc/delegation_gate.go` — RequireDelegationReady function from Task 8
  - `bridge/pkg/rpc/bridge_handlers.go` — Agent management handlers to gate
  - `bridge/pkg/rpc/server.go:687-746` — registerHandlers to find all delegation-related methods

  **Acceptance Criteria**:
  - [ ] All delegation/agent handlers have gate check as first line
  - [ ] `cd bridge && go test -v ./pkg/rpc/...` → ALL PASS
  - [ ] Integration test proves gate blocks before hardening and allows after

  **QA Scenarios (MANDATORY):**

  ```
  Scenario: All delegation handlers gated
    Tool: Bash (go test)
    Steps:
      1. cd bridge && go test -v -run TestAllDelegationGated ./pkg/rpc/...
      2. Verify every delegation-related handler returns ErrHardeningRequired when not hardened
    Expected Result: PASS — all handlers properly gated
    Evidence: .sisyphus/evidence/task-14-all-gated.txt
  ```

  **Commit**: YES
  - Message: `feat(hardening): wire delegation gate into all agent management handlers`
  - Pre-commit: `cd bridge && go test ./pkg/rpc/... && go build ./cmd/bridge`

- [ ] 15. End-to-End Hardening Flow Integration Test

  **What to do**:
  - Write comprehensive integration test spanning Bridge + ArmorChat:
    - Create new admin user → verify hardening state is pending (all defaults)
    - Attempt agent creation → verify ErrHardeningRequired
    - Complete step 1: rotate password → verify state updated
    - Complete step 2: verify device → verify state updated
    - Complete step 3: backup recovery key → verify state updated
    - Skip step 4: biometrics → verify state shows skipped but delegation_ready=true
    - Attempt agent creation → verify SUCCESS
    - Verify existing user NOT forced through hardening
  - Write shell test script: `tests/test-hardening-flow.sh`
  - Test via RPC calls using socat to bridge socket

  **Must NOT do**:
  - DO NOT modify production code (test-only task)
  - DO NOT test Android UI (test via RPC only)

  **Recommended Agent Profile**:
  - **Category**: `unspecified-high`
    - Reason: Integration test spanning multiple components
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: NO
  - **Parallel Group**: Wave 3
  - **Blocks**: FINAL
  - **Blocked By**: Task 5, Task 6, Task 7, Task 8, Task 14

  **References**:
  - `tests/test-rpc-methods.sh` — Existing shell test pattern for RPC testing
  - `bridge/pkg/rpc/hardening_handlers.go` — Handler methods to test
  - `bridge/pkg/rpc/delegation_gate.go` — Gate function to test

  **Acceptance Criteria**:
  - [ ] `bash tests/test-hardening-flow.sh` → ALL PASS
  - [ ] Full hardening flow completes via RPC
  - [ ] Delegation gate blocks/enables correctly
  - [ ] Existing user not forced through hardening

  **QA Scenarios (MANDATORY):**

  ```
  Scenario: End-to-end hardening flow via RPC
    Tool: Bash (shell test)
    Steps:
      1. bash tests/test-hardening-flow.sh
      2. Verify all test cases pass
    Expected Result: ALL PASS
    Evidence: .sisyphus/evidence/task-15-e2e.txt
  ```

  **Commit**: YES
  - Message: `test(hardening): add end-to-end hardening flow integration test`
  - Files: `tests/test-hardening-flow.sh`

---

## Final Verification Wave

- [ ] F1. **Plan Compliance Audit** — `oracle`
  Read plan end-to-end. For each "Must Have": verify implementation exists. For each "Must NOT Have": search codebase for forbidden patterns. Check evidence files exist. Compare deliverables against plan.
  Output: `Must Have [N/N] | Must NOT Have [N/N] | Tasks [N/N] | VERDICT: APPROVE/REJECT`

- [ ] F2. **Code Quality Review** — `unspecified-high`
  Run `go build ./cmd/bridge` + `go test ./...` in bridge. Check for: `as any`/`@ts-ignore` (Go equivalent), empty catches, console.log in prod, unused imports. Check AI slop: excessive comments, over-abstraction, generic names. Run Kotlin lint on ArmorChat.
  Output: `Build [PASS/FAIL] | Lint [PASS/FAIL] | Tests [N pass/N fail] | Files [N clean/N issues] | VERDICT`

- [ ] F3. **Integration Test** — `unspecified-high`
  Start bridge, create new admin user, verify hardening state is pending. Attempt delegation RPC → expect error. Complete all hardening steps via RPC. Attempt delegation RPC → expect success. Verify existing user NOT forced through hardening.
  Output: `Hardening [N/N steps] | Gate [BLOCKED/ALLOWED] | Existing User [SKIP] | VERDICT`

- [ ] F4. **Scope Fidelity Check** — `deep`
  For each task: read "What to do", read actual diff. Verify 1:1 — everything in spec was built, nothing beyond spec was built. Check "Must NOT do" compliance. Detect cross-task contamination.
  Output: `Tasks [N/N compliant] | Contamination [CLEAN/N issues] | Unaccounted [CLEAN/N files] | VERDICT`

---

## Commit Strategy

- **Wave 1**: `feat(hardening): add state model, keystore table, ChangePassword, navigation infrastructure, RPC stubs`
- **Wave 2**: `feat(hardening): implement store, delegation gate, wizard ViewModel and screens`
- **Wave 3**: `feat(hardening): wire onboarding flow, device verification, gate enforcement`
- **Final**: `test(hardening): add integration and compliance tests`

---

## Success Criteria

### Verification Commands
```bash
# Go build
cd bridge && go build ./cmd/bridge

# Go tests
cd bridge && go test -v ./pkg/trust/... ./pkg/keystore/... ./pkg/rpc/...

# Kotlin build (if gradle available)
cd applications/ArmorChat && ./gradlew assembleDebug

# RPC: Check hardening status
echo '{"jsonrpc":"2.0","id":1,"method":"hardening.status"}' | socat - UNIX-CONNECT:/run/armorclaw/bridge.sock

# RPC: Attempt delegation before hardening (should fail)
echo '{"jsonrpc":"2.0","id":2,"method":"studio.create_agent"}' | socat - UNIX-CONNECT:/run/armorclaw/bridge.sock

# Verify bootstrap password file deleted after rotation
docker exec armorclaw ls /var/lib/armorclaw/.admin_password  # Should fail (file deleted)
```

### Final Checklist
- [ ] All "Must Have" present
- [ ] All "Must NOT Have" absent
- [ ] All Go tests pass
- [ ] ArmorChat builds and launches
- [ ] New user completes hardening wizard
- [ ] Delegation blocked before hardening
- [ ] Delegation allowed after hardening
- [ ] Existing users NOT forced through hardening
