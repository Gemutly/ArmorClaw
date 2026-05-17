# Production Sovereign Launch - Bridge/Go Backend

## TL;DR

> **Quick Summary**: Security hardening and infrastructure improvements for the ArmorClaw Bridge, focusing on keystore persistence, PII shadowing, and vault management.
>
> **Deliverables**:
> - Keystore persistence fix with migration helper
> - PII Shadow Interceptor (BlindFill Engine)
> - Automated vault reseal on inactivity
> - OpenClaw 2026.3 compatibility
>
> **Estimated Effort**: Medium (3-5 days)
> **Parallel Execution**: Partial - Wave 1 can parallelize, Wave 2 sequential
> **Critical Path**: Keystore Fix → PII Shadow → Vault Reseal

---

## Context

### Original Request
Engineering backlog for Production Sovereign Launch, focusing on closing the "Cold Vault" and fixing critical persistence blockers.

### Current State
- **Keystore**: Has deriveMasterKey() with 4-tier priority system (env var > file > container > hardware)
- **Issue**: Keys can be lost on container restart if keystore.key file not persisted
- **PII**: No shadow interception yet - sensitive data passes through LLM
- **Vault**: No automatic reseal on inactivity

### Metis Review
**Identified Gaps** (to be addressed):
- Migration path for existing users with hardware-derived keys
- Volume mount documentation for Docker deployments
- Error message clarity when key mismatch occurs

---

## Work Objectives

### Core Objective
Fix critical keystore persistence issue and implement PII shadowing to protect sensitive data from LLM exposure.

### Concrete Deliverables
- Keystore migration helper with backward compatibility
- Container detection with automatic key persistence
- PII Shadow Interceptor middleware
- Vault reseal on mobile heartbeat loss
- Token auth enforcement

### Definition of Done
- [ ] Keys survive `docker restart` and container recreation
- [ ] Existing users can migrate without data loss
- [ ] PII patterns (`{{VAULT:hash}}`) intercepted before LLM
- [ ] Vault reseals after 15 minutes of mobile inactivity

### Must Have
- Keystore persistence across container restarts
- Migration helper for existing hardware-derived keys
- Container detection (isRunningInContainer)
- PII shadow middleware

### Must NOT Have (Guardrails)
- DO NOT break existing keystore for native deployments
- DO NOT force key rotation on existing users
- DO NOT log sensitive key material
- DO NOT expose PII in error messages

---

## Verification Strategy

### Test Decision
- **Infrastructure exists**: YES (go test)
- **Automated tests**: TDD for new logic, integration tests for persistence
- **Framework**: go test with t.TempDir()

### QA Policy
Every task includes agent-executed QA scenarios:
- **Backend**: Use Bash (go test, curl for APIs)
- **Evidence**: Saved to `.sisyphus/evidence/task-{N}-{scenario-slug}.txt`

---

## Execution Strategy

### Parallel Execution Waves

```
Wave 1 (Start Immediately — independent foundation):
├── Task 1: Keystore migration helper [quick]
├── Task 2: Container detection improvement [quick]
└── Task 3: Error message clarity [quick]

Wave 2 (After Wave 1 — depends on keystore fix):
├── Task 4: PII Shadow Interceptor middleware [unspecified-high]
├── Task 5: Unix Socket injection path [unspecified-high]
└── Task 6: Integration tests for PII shadow [unspecified-high]

Wave 3 (After Wave 2 — vault management):
├── Task 7: Mobile heartbeat tracking [unspecified-high]
├── Task 8: Vault reseal implementation [unspecified-high]
└── Task 9: Token auth enforcement [quick]

Wave FINAL (After ALL tasks — verification):
├── Task F1: Plan compliance audit [oracle]
├── Task F2: Code quality review [unspecified-high]
├── Task F3: Docker restart persistence test [unspecified-high]
└── Task F4: Scope fidelity check [deep]
```

### Dependency Matrix

- **1-3**: No dependencies (parallel)
- **4**: Depends on 1 (needs stable keystore)
- **5**: Depends on 4 (needs middleware)
- **6**: Depends on 4, 5
- **7-9**: No dependencies on 4-6 (can parallel with Wave 2 if bandwidth)
- **F1-F4**: Depends on ALL implementation tasks

---

## TODOs

- [ ] 1. Keystore Migration Helper

  **What to do**:
  - Add migration function to convert hardware-derived keys to file-persisted keys
  - Create `migrateToPersistedKey()` in keystore.go
  - On first run with existing DB: read hardware key, verify DB opens, save to keystore.key file
  - Add `--migrate-keystore` CLI flag for explicit migration
  - Write tests for migration scenarios

  **Must NOT do**:
  - DO NOT auto-migrate without user consent
  - DO NOT delete old key until new one verified

  **Recommended Agent Profile**:
  - **Category**: `quick`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 1 (with Tasks 2, 3)
  - **Blocks**: Task 4
  - **Blocked By**: None

  **References**:
  - `bridge/pkg/keystore/keystore.go:192-232` — deriveMasterKey hierarchy
  - `bridge/pkg/keystore/keystore.go:246-287` — verifyKeyWorks pattern

  **Acceptance Criteria**:
  - [ ] Migration function exists and works
  - [ ] Existing DB with hardware key can be migrated
  - [ ] keystore.key file created after migration
  - [ ] Tests pass: `go test -run TestKeystoreMigration ./pkg/keystore/...`

  **QA Scenarios**:
  ```
  Scenario: Hardware key migration
    Tool: Bash (go test)
    Steps:
      1. Create keystore with hardware-derived key
      2. Run migration function
      3. Verify keystore.key file exists
      4. Verify DB still opens with persisted key
    Expected Result: Migration successful, key persisted
    Evidence: .sisyphus/evidence/task-1-migration.txt
  ```

  **Commit**: YES
  - Message: `fix(keystore): add migration helper for hardware-derived keys`

---

- [ ] 2. Container Detection Improvement

  **What to do**:
  - Enhance `isContainerized()` with more detection methods
  - Add systemd-nspawn detection
  - Add Podman detection
  - Add ECS/Fargate detection via environment
  - Add logging when container mode detected
  - Write tests for each detection path

  **Must NOT do**:
  - DO NOT change key derivation logic (just detection)
  - DO NOT add external dependencies

  **Recommended Agent Profile**:
  - **Category**: `quick`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 1 (with Tasks 1, 3)
  - **Blocks**: None
  - **Blocked By**: None

  **References**:
  - `bridge/pkg/keystore/keystore.go:234-244` — isContainerized function

  **Acceptance Criteria**:
  - [ ] systemd-nspawn detected
  - [ ] Podman detected
  - [ ] ECS environment detected
  - [ ] Log message when container mode active
  - [ ] Tests pass: `go test -run TestContainerDetection ./pkg/keystore/...`

  **Commit**: YES
  - Message: `feat(keystore): improve container detection for more environments`

---

- [ ] 3. Error Message Clarity

  **What to do**:
  - Improve KEY MISMATCH error message with actionable steps
  - Add suggestion to check volume mounts
  - Add suggestion to verify ARMORCLAW_KEYSTORE_SECRET
  - Include container-specific guidance
  - Add link to troubleshooting docs

  **Must NOT do**:
  - DO NOT expose key material in errors
  - DO NOT change error structure (just messages)

  **Recommended Agent Profile**:
  - **Category**: `quick`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 1 (with Tasks 1, 2)
  - **Blocks**: None
  - **Blocked By**: None

  **References**:
  - `bridge/pkg/keystore/keystore.go:270-283` — current error message

  **Acceptance Criteria**:
  - [ ] Error message includes volume mount check
  - [ ] Error message includes env var check
  - [ ] Container-specific guidance present
  - [ ] Tests verify message content

  **Commit**: YES
  - Message: `fix(keystore): improve KEY MISMATCH error with actionable guidance`

---

- [ ] 4. PII Shadow Interceptor Middleware

  **What to do**:
  - Create `bridge/pkg/agent/pii_interceptor.go`
  - Implement `{{VAULT:hash}}` pattern detection in prompt content
  - When detected, replace with placeholder token
  - Store mapping of placeholder → actual value
  - Provide interface for value injection later
  - Write comprehensive tests

  **Must NOT do**:
  - DO NOT modify existing agent code paths (new middleware only)
  - DO NOT log PII values
  - DO NOT send PII to LLM

  **Recommended Agent Profile**:
  - **Category**: `unspecified-high`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: NO
  - **Parallel Group**: Wave 2
  - **Blocks**: Task 5, 6
  - **Blocked By**: Task 1 (stable keystore)

  **References**:
  - `bridge/pkg/agent/` — agent package structure
  - `bridge/pkg/pii/` — existing PII handling

  **Acceptance Criteria**:
  - [ ] Pattern `{{VAULT:[a-f0-9]+}}` detected
  - [ ] PII replaced with `[REDACTED:hash]`
  - [ ] Mapping stored for later injection
  - [ ] Tests pass: `go test -run TestPIIInterceptor ./pkg/agent/...`

  **Commit**: YES
  - Message: `feat(agent): add PII shadow interceptor middleware`

---

- [ ] 5. Unix Socket Injection Path

  **What to do**:
  - Create Unix socket listener in `bridge/pkg/agent/injection.go`
  - Accept connections from browser-service
  - On request: lookup PII value by hash, return via socket
  - Never expose value to LLM process
  - Add authentication via shared secret
  - Write integration tests

  **Must NOT do**:
  - DO NOT expose socket to network (Unix socket only)
  - DO NOT log injected values
  - DO NOT allow unauthenticated connections

  **Recommended Agent Profile**:
  - **Category**: `unspecified-high`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: NO
  - **Parallel Group**: Wave 2
  - **Blocks**: Task 6
  - **Blocked By**: Task 4

  **References**:
  - `bridge/pkg/secrets/socket.go` — existing socket patterns
  - `bridge/pkg/browser/` — browser service integration

  **Acceptance Criteria**:
  - [ ] Unix socket created at `/run/armorclaw/pii.sock`
  - [ ] Authenticated connections only
  - [ ] Values returned without LLM exposure
  - [ ] Tests pass: `go test -run TestPIIInjection ./pkg/agent/...`

  **Commit**: YES
  - Message: `feat(agent): add Unix socket injection path for PII values`

---

- [ ] 6. PII Shadow Integration Tests

  **What to do**:
  - Create end-to-end test for full PII shadow flow
  - Test: prompt with `{{VAULT:hash}}` → interceptor → injection → browser
  - Verify LLM never sees actual value
  - Test error cases: invalid hash, missing value, auth failure
  - Add to CI test suite

  **Must NOT do**:
  - DO NOT use real PII in tests (synthetic data only)
  - DO NOT skip auth in tests

  **Recommended Agent Profile**:
  - **Category**: `unspecified-high`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: NO
  - **Parallel Group**: Wave 2
  - **Blocks**: None
  - **Blocked By**: Task 4, 5

  **References**:
  - `bridge/pkg/agent/pii_interceptor.go` — middleware from Task 4
  - `bridge/pkg/agent/injection.go` — socket from Task 5

  **Acceptance Criteria**:
  - [ ] Full flow test passes
  - [ ] LLM exposure verified as none
  - [ ] Error cases handled
  - [ ] Tests pass: `go test -run TestPIIShadowE2E ./pkg/agent/...`

  **Commit**: YES
  - Message: `test(agent): add PII shadow end-to-end integration tests`

---

- [ ] 7. Mobile Heartbeat Tracking

  **What to do**:
  - Add heartbeat endpoint to RPC handlers
  - Track last heartbeat timestamp per user
  - Store in memory (or keystore for persistence)
  - Expose `getLastHeartbeat(userID)` function
  - Write tests

  **Must NOT do**:
  - DO NOT require heartbeat (optional feature)
  - DO NOT block operations on heartbeat

  **Recommended Agent Profile**:
  - **Category**: `unspecified-high`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES (with Task 4-6 if bandwidth)
  - **Parallel Group**: Wave 3
  - **Blocks**: Task 8
  - **Blocked By**: None

  **References**:
  - `bridge/pkg/rpc/server.go` — RPC handler patterns

  **Acceptance Criteria**:
  - [ ] `mobile.heartbeat` RPC endpoint added
  - [ ] Timestamp tracked per user
  - [ ] `getLastHeartbeat()` function available
  - [ ] Tests pass

  **Commit**: YES
  - Message: `feat(rpc): add mobile heartbeat tracking endpoint`

---

- [ ] 8. Vault Reseal Implementation

  **What to do**:
  - Create reaper goroutine in keystore
  - Check heartbeat every 1 minute
  - If no heartbeat for 15 minutes: purge decrypted keys from memory
  - Use `memguard` for secure key storage when available
  - Log reseal events (without key material)
  - Write tests

  **Must NOT do**:
  - DO NOT delete persisted keys (just memory purge)
  - DO NOT reseal if heartbeat never received (feature not enabled)

  **Recommended Agent Profile**:
  - **Category**: `unspecified-high`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: NO
  - **Parallel Group**: Wave 3
  - **Blocks**: None
  - **Blocked By**: Task 7

  **References**:
  - `bridge/pkg/keystore/keystore.go` — keystore structure
  - `github.com/awnumar/memguard` — secure memory

  **Acceptance Criteria**:
  - [ ] Reaper goroutine started
  - [ ] Keys purged after 15 min inactivity
  - [ ] memguard used for secure storage
  - [ ] Reseal events logged
  - [ ] Tests pass

  **Commit**: YES
  - Message: `feat(keystore): implement automatic vault reseal on inactivity`

---

- [ ] 9. Token Auth Enforcement

  **What to do**:
  - Remove `auth: none` support from config
  - Require token auth for all RPC calls
  - Add deprecation warning if `auth = "none"` in config
  - Update docs/examples
  - Write tests

  **Must NOT do**:
  - DO NOT break existing token auth
  - DO NOT change token format

  **Recommended Agent Profile**:
  - **Category**: `quick`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES (with Tasks 7-8)
  - **Parallel Group**: Wave 3
  - **Blocks**: None
  - **Blocked By**: None

  **References**:
  - `bridge/pkg/config/config.go` — auth configuration
  - `bridge/pkg/auth/` — auth implementation

  **Acceptance Criteria**:
  - [ ] `auth: none` rejected
  - [ ] Deprecation warning logged
  - [ ] Token auth required
  - [ ] Tests pass

  **Commit**: YES
  - Message: `feat(auth): enforce token auth, remove 'none' support`

---

## Final Verification Wave

- [ ] F1. **Plan Compliance Audit** — `oracle`
  Read plan end-to-end. Verify each "Must Have" implemented. Check "Must NOT Have" compliance.
  Output: `Must Have [N/N] | Must NOT Have [N/N] | Tasks [N/N] | VERDICT`

- [ ] F2. **Code Quality Review** — `unspecified-high`
  Run `go build ./cmd/bridge` + `go test ./...`. Check for anti-patterns.
  Output: `Build [PASS/FAIL] | Tests [N pass/N fail] | Files [N clean/N issues] | VERDICT`

- [ ] F3. **Docker Restart Persistence Test** — `unspecified-high`
  Build Docker image, start container, create keys, restart container, verify keys survive.
  Output: `Keys Persist [YES/NO] | Migration [WORKS/FAILS] | VERDICT`

- [ ] F4. **Scope Fidelity Check** — `deep`
  For each task: verify 1:1 implementation vs spec. Check for scope creep.
  Output: `Tasks [N/N compliant] | Contamination [CLEAN/N issues] | VERDICT`

---

## Commit Strategy

- **Wave 1**: `fix(keystore): add migration helper, improve container detection, clarify errors`
- **Wave 2**: `feat(agent): implement PII shadow interceptor with Unix socket injection`
- **Wave 3**: `feat(keystore): add vault reseal, enforce token auth`

---

## Success Criteria

### Verification Commands
```bash
# Build
cd bridge && go build ./cmd/bridge

# All tests
cd bridge && go test ./pkg/keystore/... ./pkg/agent/... ./pkg/auth/...

# Docker persistence test
docker build -t armorclaw-bridge-test -f Dockerfile .
docker run -d --name test-bridge armorclaw-bridge-test
# Create keys via RPC
docker restart test-bridge
# Verify keys still accessible

# PII shadow test
echo '{"jsonrpc":"2.0","id":1,"method":"test.pii_shadow"}' | nc -U /run/armorclaw/bridge.sock
```

### Final Checklist
- [ ] Keys survive Docker restart
- [ ] Migration helper works for existing users
- [ ] PII never exposed to LLM
- [ ] Vault reseals on inactivity
- [ ] Token auth enforced
