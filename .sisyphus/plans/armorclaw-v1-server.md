# ArmorClaw Server v1.0.0 — Complete Implementation Plan

## TL;DR

> **Quick Summary**: Refactor and extend the existing ArmorClaw Bridge to add 14 new RPC methods (95→109) gated by runtime feature flags across 7 phases: Zero-Trust Keystore, Passphrase Hardening, Voice Cloud, Browser Polish, E2EE Backup, Hardening, and Validation. This is a **brownfield** effort — most infrastructure already exists and must be adapted, not created from scratch.
> 
> **Deliverables**:
> - 7 new keystore.* RPC handlers with Argon2id password-gated unseal + auto-seal timer
> - 3 new voice.* RPC handlers with OpenAI cloud STT/TTS/VAD
> - 1 new browser.replay_diagnostics handler with multi-tab support
> - 3 new e2ee.* backup handlers with BIP-39 recovery phrases
> - Runtime feature flag system (env-var + config toggles)
> - Memory zeroization across all sensitive paths
> - Comprehensive TDD test coverage for all new code
> - Full validation harness across 6 flag configurations
> 
> **Estimated Effort**: XL (31 days across 7 phases)
> **Parallel Execution**: YES — 7 waves across phases
> **Critical Path**: T1(baseline) → T2(types) → T6(keystore refactor) → T9(keystore handlers) → T12(voice audit) → T15(E2EE) → T19(audit logging) → T21(audit denylist) → T23(method count) → F1-F4(final verification)

---

## Context

### Original Request
Implement ArmorClaw Server v1.0.0 — close 4 documented gaps (zero-trust unseal, voice providers, multi-tab replay, E2EE backup) via feature-flagged RPC additions, validated through the existing A0–A4 contract discovery pipeline.

### Interview Summary
**Key Discussions**:
- Plan v7 described greenfield CREATE actions, but codebase already has substantial implementations
- User confirmed: **adapt to existing code** (brownfield), not greenfield
- User confirmed: **keep challenge-response as alternative policy** alongside new Argon2id password unseal
- Existing AEAD is XChaCha20-Poly1305 (via `key_derivation.go`), not AES-256-GCM — plan adapted accordingly
- Feature flags are license-tier-gated via `enforcement/` — need new runtime toggle mechanism

**Research Findings**:
- `sealed_keystore.go` (681 lines) already has agent-scoped sessions, challenge-response, RWMutex
- `key_derivation.go` (399 lines) already has Argon2id + XChaCha20-Poly1305 WrapKey/UnwrapKey
- `voice/manager.go` (505 lines) exists but commented out in main.go
- `config/flags.go` does NOT exist — feature flags via `enforcement/enforcement.go`
- `pii/engine.go` does NOT exist — PII entry point is `pii/scrubber.go`
- CI only runs `./pkg/rpc/...` Go tests, not the full 274-file test suite
- 5 keystore test files use `//go:build cgo` tags — release builds use CGO_ENABLED=0

### Metis Review
**Identified Gaps** (addressed):
- AEAD mismatch: Plan said AES-256-GCM, codebase uses XChaCha20-Poly1305 → **Resolved**: use existing
- Feature flag wiring: No runtime toggles exist → **Resolved**: extend config.go
- Voice uncomment completeness: May need more than uncommenting → **Resolved**: audit main.go first
- Concurrent policy conflict: Password unseal vs challenge unseal racing → **Resolved**: Mutex on SealedStore
- Argon2id DoS: 64MB per attempt could exhaust memory → **Resolved**: rate limiter + memory budget cap
- CI coverage gap: Go unit tests not run in CI → **Resolved**: note in plan, local testing required
- Fuzz testing absence for crypto paths → **Deferred**: post-v1.0.0

---

## Work Objectives

### Core Objective
Close the 4 documented architectural gaps by adding feature-flagged RPC methods to the existing Bridge, adapting existing infrastructure rather than building from scratch.

### Concrete Deliverables
- `bridge/pkg/keystore/securemem.go` — ZeroBytes helper
- `bridge/pkg/keystore/ratelimit.go` — Per-identity rate limiter
- `bridge/pkg/rpc/identity.go` — Client identity resolution (RPC layer, not keystore)
- Refactored `bridge/pkg/keystore/sealed_keystore.go` — Password policy + auto-seal + Mutex
- 7 new RPC handlers in `bridge/pkg/rpc/server.go` (keystore.*)
- 3 new RPC handlers (voice.*)
- 1 new RPC handler (browser.replay_diagnostics)
- 3 new RPC handlers (`e2ee.create_backup`, `e2ee.delete_backup`, `e2ee.backup_exists`) — extending the existing 2-method `e2ee.*` toggle group to 5 total
- Runtime feature flag fields in `bridge/pkg/config/config.go`
- Updated canonical RPC method inventory (currently centered around `scripts/lib/contract.sh` and A0 contract helpers) for 14 new methods
- Updated `scripts/a4_harness.sh` SUITE_MAP with new test suites
- Updated `doc/architecture.md`, `doc/bridge-reference.md`, `doc/testing.md`

### Definition of Done
- [x] `a0_discover.sh` reports 95 methods with all flags off, 109 with all flags on
- [x] All new RPC methods return `-32601` when their feature flag is disabled
- [x] All Go tests in v1.0.0-touched packages pass: `cd bridge && CGO_ENABLED=1 go test -v ./pkg/keystore/... ./pkg/voice/... ./pkg/crypto/... ./pkg/browser/... ./pkg/rpc/... ./pkg/config/... ./pkg/provisioning/... ./cmd/bootstrap-admin/...` → 0 failures
- [x] All bash harness tests pass or gracefully skip (harness is designed to skip when subsystems are not deployed/configured; `test-voice-stack.sh` skips when voice is unconfigured)
- [x] Zero regression: existing 95-method base unchanged when all flags off
- [x] Evidence captured in `.sisyphus/evidence/` for all QA scenarios

### Must Have
- Argon2id password-gated unseal as DEFAULT policy for zero-trust keystore
- Challenge-response (Ed25519) preserved as ALTERNATIVE policy
- 4-hour auto-seal timer with activity-based reset
- Rate limiting (5 attempts/min) on unseal
- Memory zeroization on all sensitive byte paths
- XChaCha20-Poly1305 AEAD (existing, not AES-256-GCM)
- Feature flags as runtime toggles (env vars / config)
- `e2ee.restore_backup` MUST NOT exist in codebase (the 3 new backup handlers are `e2ee.create_backup`, `e2ee.delete_backup`, `e2ee.backup_exists`)
- Exact names of existing 2 `e2ee.*` methods in the 95-method base must be verified via `a0_discover.sh` before hardcoding (plan does NOT assume their names)
- All flag-off paths = zero regression against 95-method baseline

### Must NOT Have (Guardrails)
- Do NOT replace existing `challenge.go` — keep as alternative policy
- Do NOT create `bridge/pkg/config/flags.go` — extend existing `config.go`
- Do NOT create `bridge/pkg/pii/engine.go` — modify existing `pii/scrubber.go`
- Do NOT create `bridge/pkg/voice/stt.go` — extend existing `voice/stt_service.go`
- Do NOT switch from XChaCha20-Poly1305 to AES-256-GCM
- Do NOT add `sealed` boolean column to keystore.db schema
- Do NOT implement `e2ee.restore_backup`
- Do NOT add fuzz tests (deferred to post-v1.0.0)
- Do NOT run full Go test suite in CI (not in scope)
- Do NOT touch Android client code
- Do NOT modify deployment infrastructure
- Do NOT introduce new Go module dependencies without justification
- AI slop patterns to avoid: excessive comments, over-abstraction, generic names, premature generalization

---

## Verification Strategy (MANDATORY)

> **ZERO HUMAN INTERVENTION** - ALL verification is agent-executed. No exceptions.
> Acceptance criteria requiring "user manually tests/confirms" are FORBIDDEN.

### Test Decision
- **Infrastructure exists**: YES (100+ Go test files, 68 bash scripts, 5 CI workflows)
- **Automated tests**: TDD (Red-Green-Refactor)
- **Framework**: Go `testing` package + bash harness
- **Each task**: Write failing test FIRST, then implement to pass, then refactor

### Test Execution Notes
- Keystore tests require `CGO_ENABLED=1` (SQLCipher dependency)
- 5 existing keystore test files use `//go:build cgo` tags
- Release builds use `CGO_ENABLED=0` — keystore tests CANNOT validate release binaries
- CI only runs `go test ./pkg/rpc/...` — full suite must be run locally
- Benchmark baseline for keystore operations should be captured before Phase 1 changes

### QA Policy
Every task MUST include agent-executed QA scenarios.
Evidence saved to `.sisyphus/evidence/task-{N}-{scenario-slug}.{ext}`.

- **Go unit/integration tests**: Use Bash (`go test -v -run TestName ./pkg/...`)
- **RPC endpoints**: Use Bash (curl over Unix socket / TCP)
- **Discovery verification**: Use Bash (`a0_discover.sh` + jq)
- **Cross-subsystem**: Use bash harness scripts

> **Note on verification scope**: All task-level QA is agent-executed. The final "user okay" after the verification wave is a human gate on whether to ship, not a QA step. The two are distinct.

---

## Execution Strategy

### Parallel Execution Waves

```
Wave 1 (Start Immediately — foundation, no dependencies):
├── Task 1: Capture keystore benchmark baseline [quick]
├── Task 2: Runtime feature flag system [quick]
├── Task 3: securemem.go — ZeroBytes helper [quick]
├── Task 4: ratelimit.go — Per-identity rate limiter [quick]
├── Task 5: identity.go — Client identity resolution [quick]

Wave 2 (After Wave 1 — core keystore, depends on T2-T4):
├── Task 6: Refactor sealed_keystore.go — add password policy [deep]
├── Task 7: Auto-seal timer + activity tracking [deep]
├── Task 8: keystore CRUD (list_keys, delete_key) [quick]

Wave 3 (After Wave 2 — keystore integration, depends on T6-T8):
├── Task 9: Keystore RPC handlers (7 methods) + discovery [unspecified-high]
├── Task 10: Integrate sealed checks into existing Get/Set/PII/provider flows [unspecified-high]
├── Task 11: Passphrase validation + bootstrap password generation [quick]

Wave 4 (After Wave 3 — voice, browser, E2EE; can run in parallel):
├── Task 12: Voice stack — uncomment + extend with OpenAI providers [unspecified-high]
├── Task 13: Voice RPC handlers (3 methods) + discovery [unspecified-high]
├── Task 14: Browser multi-tab replay + diagnostics [unspecified-high]
├── Task 15: E2EE key backup storage (BIP-39 + create/delete/exists) [deep]

Wave 5 (After Wave 4 — remaining RPC + discovery):
├── Task 16: E2EE backup RPC handlers (3 methods) + discovery [quick]
├── Task 17: Browser replay_diagnostics RPC handler + discovery [quick]

Wave 6 (After Wave 5 — hardening):
├── Task 18: Memory zeroization audit + fixes [deep]
├── Task 19: Audit logging for all new event types [unspecified-high]
├── Task 20: Legacy browser deprecation notices [quick]
├── Task 21: Audit denylist cleanup (rejected-design artifacts only) [quick]

Wave 7 (After ALL tasks — validation):
├── Task 22: Full harness — 6 flag configurations [deep]
├── Task 23: Contract spot checks + method count verification [quick]
├── Task 24: Documentation updates (architecture, bridge-reference, testing, CHANGELOG) [writing]

Wave FINAL (After ALL implementation — 4 parallel reviews):
├── Task F1: Plan compliance audit (oracle)
├── Task F2: Code quality review (unspecified-high)
├── Task F3: Agent-Executed E2E QA Sweep (unspecified-high)
└── Task F4: Scope fidelity check (deep)
→ Present results → Get explicit user okay

Critical Path: T1 → T2 → T6 → T9 → T12/T14/T15 → T16/T17 → T19(audit logging) → T21(denylist) → T23(method count) → F1-F4 → user okay
Parallel Speedup: ~60% faster than sequential
Max Concurrent: 5 (Wave 1)
```

### Dependency Matrix

| Task | Depends On | Blocks | Wave |
|------|-----------|--------|------|
| T1 | — | — | 1 |
| T2 | — | T6, T9, T12, T13, T14, T15, T16, T17 | 1 |
| T3 | — | T6, T18 | 1 |
| T4 | — | T6 | 1 |
| T5 | — | T9 | 1 |
| T6 | T2, T3, T4 | T7, T8, T9, T10 | 2 |
| T7 | T6 | T9 | 2 |
| T8 | T6 | T9 | 2 |
| T9 | T6, T7, T8 | T19 | 3 |
| T10 | T6 | T18 | 3 |
| T11 | — | — | 3 |
| T12 | T2 | T13 | 4 |
| T13 | T12 | — | 4 |
| T14 | T2 | T17 | 4 |
| T15 | T2 | T16 | 4 |
| T16 | T15 | — | 5 |
| T17 | T14 | — | 5 |
| T18 | T3, T10 | — | 6 |
| T19 | T9, T13, T16 | — | 6 |
| T20 | — | — | 6 |
| T21 | — | — | 6 |
| T22 | T9, T13, T17, T16 | — | 7 |
| T23 | T22 | — | 7 |
| T24 | T22, T23 | — | 7 |

### Agent Dispatch Summary

- **Wave 1**: 5 — T1 → `quick`, T2 → `quick`, T3 → `quick`, T4 → `quick`, T5 → `quick`
- **Wave 2**: 3 — T6 → `deep`, T7 → `deep`, T8 → `quick`
- **Wave 3**: 3 — T9 → `unspecified-high`, T10 → `unspecified-high`, T11 → `quick`
- **Wave 4**: 4 — T12 → `unspecified-high`, T13 → `unspecified-high`, T14 → `unspecified-high`, T15 → `deep`
- **Wave 5**: 2 — T16 → `quick`, T17 → `quick`
- **Wave 6**: 4 — T18 → `deep`, T19 → `unspecified-high`, T20 → `quick`, T21 → `quick`
- **Wave 7**: 3 — T22 → `deep`, T23 → `quick`, T24 → `writing`
- **FINAL**: 4 — F1 → `oracle`, F2 → `unspecified-high`, F3 → `unspecified-high`, F4 → `deep`

---

## TODOs

- [x] 1. Capture Keystore Benchmark Baseline

  **What to do**:
  - Run `cd bridge && CGO_ENABLED=1 go test -bench=Benchmark -benchmem ./pkg/keystore/...` and capture output
  - Save output to `.sisyphus/evidence/task-01-keystore-benchmark-baseline.txt`
  - Document current ms/op for key operations: DeriveKey, WrapKey, UnwrapKey, SealedKeystore operations
  - This baseline will be used to detect performance regressions after Phase 1 changes

  **Must NOT do**:
  - Do NOT modify any existing code
  - Do NOT add new benchmarks — only capture existing ones

  **Recommended Agent Profile**:
  - **Category**: `quick`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 1 (with T2, T3, T4, T5)
  - **Blocks**: None (baseline capture is informational only, not a code dependency)
  - **Blocked By**: None

  **References**:

  **Pattern References**:
  - `bridge/pkg/keystore/key_derivation_test.go` — Existing Argon2id tests and benchmarks
  - `bridge/pkg/keystore/sealed_keystore_test.go` — Existing sealed keystore tests
  - `bridge/pkg/keystore/challenge_test.go` — Challenge tests
  - `bridge/pkg/keystore/keystore_test.go` — Base keystore tests

  **Acceptance Criteria**:

  **QA Scenarios (MANDATORY):**

  ```
  Scenario: Benchmark baseline captured
    Tool: Bash
    Preconditions: Bridge code compiles with CGO_ENABLED=1
    Steps:
      1. cd ${REPO_ROOT}/bridge
      2. CGO_ENABLED=1 go test -bench=Benchmark -benchmem -count=3 ./pkg/keystore/... 2>&1 | tee ${REPO_ROOT}/.sisyphus/evidence/task-01-keystore-benchmark-baseline.txt
      3. Verify file contains benchmark output lines (grep "Benchmark" evidence file)
      4. Verify file contains at least 10 benchmark entries
    Expected Result: File exists with benchmark data for all keystore operations
    Failure Indicators: No Benchmark lines, compilation error, 0 entries
    Evidence: .sisyphus/evidence/task-01-keystore-benchmark-baseline.txt

  Scenario: Existing tests still pass
    Tool: Bash
    Preconditions: CGO_ENABLED=1
    Steps:
      1. cd ${REPO_ROOT}/bridge
      2. CGO_ENABLED=1 go test -v ./pkg/keystore/... 2>&1 | tail -20
      3. Check for "FAIL" in output
    Expected Result: All existing tests pass, 0 failures
    Failure Indicators: Any "FAIL" or panic in output
    Evidence: .sisyphus/evidence/task-01-existing-tests-pass.txt
  ```

  **Commit**: YES
  - Message: `test(keystore): capture benchmark baseline before v1.0 changes`
  - Files: `.sisyphus/evidence/task-01-keystore-benchmark-baseline.txt`
  - Pre-commit: none

- [x] 2. Runtime Feature Flag System

  **What to do**:
  - Add feature flag fields to the existing config struct in `bridge/pkg/config/config.go`:
    ```go
    type FeatureFlags struct {
        ZeroTrustKeystore bool   `toml:"feature_zero_trust_keystore" env:"ARMORCLAW_FEATURE_ZERO_TRUST_KEYSTORE"`
        VoicePipeline     string `toml:"feature_voice_pipeline" env:"ARMORCLAW_FEATURE_VOICE_PIPELINE"` // "cloud" or "off"
        MultiTabReplay    bool   `toml:"feature_multi_tab_replay" env:"ARMORCLAW_FEATURE_MULTI_TAB_REPLAY"`
        E2EEBackup        bool   `toml:"feature_e2ee_backup" env:"ARMORCLAW_FEATURE_E2EE_BACKUP"`
    }
    ```
  - Embed `FeatureFlags` in the main `Config` struct
  - Add env var loading in `loader.go` (follow existing pattern for env var parsing)
  - Create a helper function `IsFeatureEnabled(flagName string) bool` accessible from RPC handlers
  - Write failing tests FIRST for: flag off returns false, flag on returns true, env var overrides config, default values (all off)

  **Must NOT do**:
  - Do NOT create a new `config/flags.go` file
  - Do NOT modify the existing enforcement/license-tier system
  - Do NOT add feature flags to the license enforcement middleware

  **Recommended Agent Profile**:
  - **Category**: `quick`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 1 (with T1, T3, T4, T5)
  - **Blocks**: T6, T9, T12, T13, T14, T15, T16, T17
  - **Blocked By**: None

  **References**:

  **Pattern References**:
  - `bridge/pkg/config/config.go:1-1313` — Main config struct, TOML+env loading. Follow existing field pattern.
  - `bridge/pkg/config/loader.go` — Config loading logic, env var parsing
  - `bridge/pkg/config/config_test.go` — Existing test patterns for config

  **API/Type References**:
  - `bridge/pkg/enforcement/enforcement.go` — Existing feature flag mechanism (license-tier). New runtime flags are SEPARATE from this.

  **Acceptance Criteria**:

  **If TDD:**
  - [ ] Test file created: `bridge/pkg/config/feature_flags_test.go`
  - [ ] `CGO_ENABLED=1 go test -v ./pkg/config/...` → PASS

  **QA Scenarios (MANDATORY):**

  ```
  Scenario: Feature flags default to off
    Tool: Bash
    Preconditions: No ARMORCLAW_FEATURE_* env vars set
    Steps:
      1. cd ${REPO_ROOT}/bridge
      2. unset ARMORCLAW_FEATURE_ZERO_TRUST_KEYSTORE ARMORCLAW_FEATURE_VOICE_PIPELINE ARMORCLAW_FEATURE_MULTI_TAB_REPLAY ARMORCLAW_FEATURE_E2EE_BACKUP
      3. CGO_ENABLED=1 go test -v -run TestFeatureFlagsDefault ./pkg/config/... 2>&1
    Expected Result: All flags default to false/"off"
    Evidence: .sisyphus/evidence/task-02-flags-default-off.txt

  Scenario: Env var enables feature flag
    Tool: Bash
    Preconditions: ARMORCLAW_FEATURE_ZERO_TRUST_KEYSTORE=true
    Steps:
      1. ARMORCLAW_FEATURE_ZERO_TRUST_KEYSTORE=true CGO_ENABLED=1 go test -v -run TestFeatureFlagsEnvOverride ./pkg/config/... 2>&1
    Expected Result: ZeroTrustKeystore flag reads as true
    Evidence: .sisyphus/evidence/task-02-flags-env-override.txt

  Scenario: Voice pipeline only accepts cloud/off
    Tool: Bash
    Steps:
      1. CGO_ENABLED=1 go test -v -run TestVoicePipelineValidation ./pkg/config/... 2>&1
    Expected Result: Invalid values rejected, "cloud" and "off" accepted
    Evidence: .sisyphus/evidence/task-02-voice-validation.txt
  ```

  **Commit**: YES
  - Message: `feat(config): add runtime feature flags for v1.0 phases`
  - Files: `bridge/pkg/config/config.go`, `bridge/pkg/config/loader.go`, `bridge/pkg/config/feature_flags_test.go`
  - Pre-commit: `cd bridge && CGO_ENABLED=1 go test ./pkg/config/...`

- [x] 3. Secure Memory Zeroization Helper

  **What to do**:
  - Create `bridge/pkg/keystore/securemem.go` with `ZeroBytes(b []byte)` function
  - Implementation: iterate and set each byte to 0, then `runtime.KeepAlive(b)`
  - Empty slice guard: if `len(b) == 0`, return immediately (no panic)
  - Write failing tests FIRST for: non-empty slice zeroized, empty slice no panic, nil slice no panic

  **Must NOT do**:
  - Do NOT attempt to zero Go strings (immutable)
  - Do NOT use `unsafe` package
  - Do NOT claim zeroization of immutable data

  **Recommended Agent Profile**:
  - **Category**: `quick`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 1 (with T1, T2, T4, T5)
  - **Blocks**: T6, T18
  - **Blocked By**: None

  **References**:

  **Pattern References**:
  - `bridge/pkg/keystore/key_derivation.go:181-183` — Existing ad-hoc zeroization pattern (for-range loop zeroing `derived.Key`). Extract this to a shared helper.
  - `bridge/pkg/keystore/key_derivation.go:229-231` — Same pattern for unwrap cleanup

  **Acceptance Criteria**:

  **If TDD:**
  - [ ] Test file created: `bridge/pkg/keystore/securemem_test.go`
  - [ ] `CGO_ENABLED=1 go test -v -run TestZeroBytes ./pkg/keystore/...` → PASS

  **QA Scenarios (MANDATORY):**

  ```
  Scenario: Non-empty slice zeroized
    Tool: Bash
    Steps:
      1. cd ${REPO_ROOT}/bridge && CGO_ENABLED=1 go test -v -run TestZeroBytes ./pkg/keystore/... 2>&1
    Expected Result: All bytes are 0 after ZeroBytes call
    Evidence: .sisyphus/evidence/task-03-zerobytes-nonempty.txt

  Scenario: Empty slice no panic
    Tool: Bash
    Steps:
      1. cd ${REPO_ROOT}/bridge && CGO_ENABLED=1 go test -v -run TestZeroBytesEmpty ./pkg/keystore/... 2>&1
    Expected Result: No panic, test passes
    Evidence: .sisyphus/evidence/task-03-zerobytes-empty.txt

  Scenario: Nil slice no panic
    Tool: Bash
    Steps:
      1. cd ${REPO_ROOT}/bridge && CGO_ENABLED=1 go test -v -run TestZeroBytesNil ./pkg/keystore/... 2>&1
    Expected Result: No panic, test passes
    Evidence: .sisyphus/evidence/task-03-zerobytes-nil.txt
  ```

  **Commit**: YES
  - Message: `feat(keystore): add secure memory zeroization helper`
  - Files: `bridge/pkg/keystore/securemem.go`, `bridge/pkg/keystore/securemem_test.go`
  - Pre-commit: `cd bridge && CGO_ENABLED=1 go test -run TestZeroBytes ./pkg/keystore/...`

- [x] 4. Per-Identity Rate Limiter

  **What to do**:
  - Create `bridge/pkg/keystore/ratelimit.go` with `RateLimiter` struct
  - Per-identity fixed 60-second window: track attempts keyed by identity + window start time; reject after 5 attempts within the same window
  - `Exceeded(identity string) bool` — returns true if >5 attempts in current window
  - Reset: new window starts on first attempt after previous window expires
  - Identity is resolved EXTERNALLY (by caller), not inside RateLimiter
  - Write failing tests FIRST: under limit passes, at limit returns true, window expiry resets, concurrent access safe

  **Must NOT do**:
  - Do NOT call RateLimiter from inside Unseal — handler calls it before Unseal
  - Do NOT use external rate limiting libraries

  **Recommended Agent Profile**:
  - **Category**: `quick`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 1 (with T1, T2, T3, T5)
  - **Blocks**: T6
  - **Blocked By**: None

  **References**:

  **Pattern References**:
  - `bridge/internal/cache/ratelimit.go` — Existing cache-level rate limiter pattern in the codebase
  - `bridge/pkg/keystore/sealed_keystore.go` — Existing concurrency patterns with mutex

  **Acceptance Criteria**:

  **If TDD:**
  - [ ] Test file created: `bridge/pkg/keystore/ratelimit_test.go`
  - [ ] `CGO_ENABLED=1 go test -v -run TestRateLimiter ./pkg/keystore/...` → PASS

  **QA Scenarios (MANDATORY):**

  ```
  Scenario: Rate limit triggers at 6th attempt
    Tool: Bash
    Steps:
      1. cd ${REPO_ROOT}/bridge && CGO_ENABLED=1 go test -v -run TestRateLimiterExceeded ./pkg/keystore/... 2>&1
    Expected Result: Exceeded returns true on 6th call for same identity within 60s
    Evidence: .sisyphus/evidence/task-04-ratelimit-exceeded.txt

  Scenario: Different identities tracked separately
    Tool: Bash
    Steps:
      1. cd ${REPO_ROOT}/bridge && CGO_ENABLED=1 go test -v -run TestRateLimiterPerIdentity ./pkg/keystore/... 2>&1
    Expected Result: 5 attempts from identity A don't affect identity B
    Evidence: .sisyphus/evidence/task-04-ratelimit-per-identity.txt

  Scenario: Window expiry resets counter
    Tool: Bash
    Steps:
      1. cd ${REPO_ROOT}/bridge && CGO_ENABLED=1 go test -v -run TestRateLimiterWindowExpiry ./pkg/keystore/... 2>&1
    Expected Result: After window expires, counter resets to 1
    Evidence: .sisyphus/evidence/task-04-ratelimit-window-expiry.txt
  ```

  **Commit**: YES
  - Message: `feat(keystore): add per-identity rate limiter`
  - Files: `bridge/pkg/keystore/ratelimit.go`, `bridge/pkg/keystore/ratelimit_test.go`
  - Pre-commit: `cd bridge && CGO_ENABLED=1 go test -run TestRateLimiter ./pkg/keystore/...`

- [x] 5. Client Identity Resolution (RPC Layer)

  **What to do**:
  - Create `bridge/pkg/rpc/identity.go` (NOT in keystore package — identity resolution is an RPC-layer concern)
  - Function: `resolveClientIdentity(r *http.Request) string`
  - Logic: Unix socket → peer credentials UID (extracted from `r.Context()` where the RPC server's Unix listener injects peer creds), TCP with trusted proxy → X-Forwarded-For, fallback → RemoteAddr
  - **Plumbing requirement**: The RPC server's Unix domain socket listener must inject peer credentials into each request's `context.Context` before the handler is called. On Linux this uses `unix.GetsockoptUcred()` on the raw fd obtained from `(*net.UnixConn).File()` (NOT `syscall.Getpeername`, which yields socket addresses, not peer credentials). The task must implement a custom `net.Listener` wrapper whose `Accept()` calls `unix.GetsockoptUcred()` on each accepted connection, stores the `Ucred` struct (UID, GID, PID) as a context value, and returns an `*http.Request` with that context. Alternatively, verify whether the existing listener already provides this.
  - Read bridge mode from config (native/sentinel/cloudflare) to determine socket vs TCP
  - Used by `handleKeystoreUnseal` to call rate limiter before invoking UnsealWithPassword
  - Write failing tests FIRST: Unix socket extracts UID, trusted proxy reads header, fallback uses IP

  **Must NOT do**:
  - Do NOT put HTTP request handling into the keystore package — identity is an RPC concern
  - Do NOT modify the existing auth middleware
  - Do NOT add new HTTP headers

  **Recommended Agent Profile**:
  - **Category**: `quick`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 1 (with T1, T2, T3, T4)
  - **Blocks**: T6
  - **Blocked By**: None

  **References**:

  - **Pattern References**:
  - `bridge/pkg/rpc/server.go` — RPC handler receives `*http.Request`, understand how requests arrive
  - `bridge/pkg/config/config.go` — Server mode (native/sentinel) determines transport type
  - `bridge/pkg/socket/` — Unix socket handling patterns

  **Acceptance Criteria**:

  **If TDD:**
  - [ ] Test file created: `bridge/pkg/rpc/identity_test.go`
  - [ ] `CGO_ENABLED=1 go test -v -run TestResolveIdentity ./pkg/rpc/...` → PASS

  **QA Scenarios (MANDATORY):**

  ```
  Scenario: RemoteAddr fallback works
    Tool: Bash
    Steps:
      1. cd ${REPO_ROOT}/bridge && CGO_ENABLED=1 go test -v -run TestResolveIdentity ./pkg/rpc/... 2>&1
    Expected Result: Returns IP:port from RemoteAddr when no special headers
    Evidence: .sisyphus/evidence/task-05-identity-resolution.txt

  Scenario: Trusted proxy header extraction
    Tool: Bash
    Steps:
      1. Test with mock request having X-Forwarded-For header
    Expected Result: Returns X-Forwarded-For value when trusted proxy configured
    Evidence: .sisyphus/evidence/task-05-identity-proxy.txt
  ```

  **Commit**: YES
  - Message: `feat(rpc): add client identity resolution for rate limiting`
  - Files: `bridge/pkg/rpc/identity.go`, `bridge/pkg/rpc/identity_test.go`
  - Pre-commit: `cd bridge && CGO_ENABLED=1 go test -run TestResolveIdentity ./pkg/rpc/...`

- [x] 6. Refactor SealedKeystore — Add Password-Gated Unseal Policy

  **What to do**:
  - This is the LARGEST single task. Refactor existing `bridge/pkg/keystore/sealed_keystore.go` to add `PolicyPassword` as a new unseal policy alongside existing policies.
  - Add new fields to `SealedKeystore`: `vaultKey []byte`, `passwordKD *KeyDerivation`
  - Note: Rate limiter is owned by the RPC handler layer (Task 4/5), NOT stored in SealedKeystore. The keystore package has no knowledge of rate limiting.
  - Add `SealedStoreConfig` for password-policy initialization: `PasswordVerifier`, `WrappedVaultKey` (using existing `WrappedKey` type from `key_derivation.go`)
  - Implement `UnsealWithPassword(password string) error`:
    1. Check `isUnsealed` → `ErrAlreadyUnsealed` (-32003)
    2. Derive verifier candidate via existing `KeyDerivation.DeriveKey(password, storedVerifySalt)`
    3. `subtle.ConstantTimeCompare(candidate, storedVerifier)` → `ErrInvalidPassword` (-32001)
    4. Derive wrap key → decrypt vault key via existing `KeyDerivation.UnwrapKey()`
    5. Set `vaultKey` in memory, mark unsealed, set timestamps
    6. Zeroize all temporary byte slices via `ZeroBytes()`
  - NOTE: Rate limiting is handled at the RPC handler layer (before calling UnsealWithPassword), NOT inside this method
  - Implement `Seal()` for password policy: zero `vaultKey`, mark sealed, stop timer
  - Add `sync.Mutex` alongside existing `sync.RWMutex` for password-policy state (vaultKey, timer). Use a separate lock to avoid upgrading the existing RWMutex.
  - Write failing tests FIRST: correct password unseals, wrong password fails, rate limiting works, concurrent Get+Seal no race, vaultKey zeroed on Seal

  **Must NOT do**:
  - Do NOT remove existing challenge-response (`challenge.go`) — it stays as `PolicyChallenge`
  - Do NOT remove existing mobile approval (`PolicyMobileApproval`) — it stays
  - Do NOT change existing `sync.RWMutex` usage for agent-scoped session operations
  - Do NOT add a `sealed` boolean column to keystore.db
  - Do NOT use AES-256-GCM — use existing XChaCha20-Poly1305 from `key_derivation.go`

  **Recommended Agent Profile**:
  - **Category**: `deep`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: NO — depends on Wave 1
  - **Parallel Group**: Wave 2 (with T7, T8 after T6 foundation)
  - **Blocks**: T7, T8, T9, T10
  - **Blocked By**: T2, T3, T4 (T5 identity is RPC-layer only, not needed for keystore refactor)

  **References**:

  **Pattern References**:
  - `bridge/pkg/keystore/sealed_keystore.go:1-681` — EXISTING sealed keystore. MUST be modified, not replaced. Keep all existing methods and types.
  - `bridge/pkg/keystore/key_derivation.go:1-399` — USE `KeyDerivation.WrapKey/UnwrapKey` for vault key encryption. DO NOT create new AEAD helpers.
  - `bridge/pkg/keystore/key_derivation.go:29-35` — `DefaultKeyDerivationParams` (Memory: 64*1024 KiB, Iterations: 3, Parallelism: 4, KeyLength: 32)
  - `bridge/pkg/keystore/challenge.go:1-453` — Existing challenge-response. MUST remain unchanged.
  - `bridge/pkg/keystore/key_derivation.go:348-349` — `ConstantTimeCompare` helper already exists

  **API/Type References**:
  - `bridge/pkg/keystore/key_derivation.go:88-103` — `WrappedKey` struct (Ciphertext, Nonce, Salt, Params, Version)
  - `bridge/pkg/keystore/key_derivation.go:154-192` — `WrapKey` method signature
  - `bridge/pkg/keystore/key_derivation.go:195-234` — `UnwrapKey` method signature
  - `bridge/pkg/keystore/key_derivation.go:76-85` — `DerivedKey` struct

  **Test References**:
  - `bridge/pkg/keystore/sealed_keystore_test.go` — Existing tests to extend, NOT replace
  - `bridge/pkg/keystore/key_derivation_test.go` — Existing key derivation tests
  - `bridge/pkg/keystore/challenge_unseal_test.go` — Challenge-specific unseal tests (must remain passing)

  **WHY Each Reference Matters**:
  - `sealed_keystore.go` is the primary file being modified — the executor must understand the full 681-line structure
  - `key_derivation.go` provides `WrapKey/UnwrapKey` which replace the plan's proposed AES-256-GCM `EncryptAEAD/DecryptAEAD`
  - `challenge.go` must NOT be touched — existing tests depend on it
  - `WrappedKey` type replaces the plan's 60-byte encrypted_vault_key format

  **Acceptance Criteria**:

  **If TDD:**
  - [ ] Test file extended: `bridge/pkg/keystore/sealed_keystore_test.go`
  - [ ] `CGO_ENABLED=1 go test -v -run "TestPasswordUnseal|TestPasswordUnsealWrong|TestPasswordUnsealAlreadyUnsealed" ./pkg/keystore/...` → PASS (rate limit tested at RPC layer in T9)
  - [ ] `CGO_ENABLED=1 go test -v ./pkg/keystore/...` → ALL pass (existing + new)

  **QA Scenarios (MANDATORY):**

  ```
  Scenario: Correct password unseals vault
    Tool: Bash
    Preconditions: SealedKeystore initialized with password policy, vault setup with known password
    Steps:
      1. cd ${REPO_ROOT}/bridge && CGO_ENABLED=1 go test -v -run TestPasswordUnseal ./pkg/keystore/... 2>&1
      2. Verify test asserts: IsSealed returns false after UnsealWithPassword
    Expected Result: Vault unsealed, vaultKey set in memory
    Evidence: .sisyphus/evidence/task-06-password-unseal-success.txt

  Scenario: Wrong password rejected with -32001
    Tool: Bash
    Steps:
      1. CGO_ENABLED=1 go test -v -run TestPasswordUnsealWrong ./pkg/keystore/... 2>&1
    Expected Result: ErrInvalidPassword returned, vault stays sealed
    Evidence: .sisyphus/evidence/task-06-password-unseal-wrong.txt

  Scenario: Already unsealed returns -32003
    Tool: Bash
    Steps:
      1. CGO_ENABLED=1 go test -v -run TestPasswordUnsealAlreadyUnsealed ./pkg/keystore/... 2>&1
    Expected Result: ErrAlreadyUnsealed, no state change
    Evidence: .sisyphus/evidence/task-06-password-already-unsealed.txt

  Scenario: Rate limited after 5 rapid attempts
    Tool: Bash
    Steps:
      1. CGO_ENABLED=1 go test -v -run TestKeystoreUnsealRateLimited ./pkg/rpc/... 2>&1
    Expected Result: 6th unseal attempt via RPC handler returns rate limit error (-32006)
    Evidence: .sisyphus/evidence/task-06-rpc-ratelimited.txt

  Scenario: Existing challenge-response tests still pass
    Tool: Bash
    Steps:
      1. CGO_ENABLED=1 go test -v -run "TestChallenge|TestUnseal" ./pkg/keystore/... 2>&1
    Expected Result: All existing challenge tests pass unchanged
    Failure Indicators: Any FAIL in challenge-related tests
    Evidence: .sisyphus/evidence/task-06-challenge-unchanged.txt

  Scenario: Concurrent Get + Seal no race
    Tool: Bash
    Steps:
      1. CGO_ENABLED=1 go test -v -race -run TestPasswordConcurrent ./pkg/keystore/... 2>&1
    Expected Result: No race condition detected
    Evidence: .sisyphus/evidence/task-06-concurrent-race.txt
  ```

  **Commit**: YES
  - Message: `refactor(keystore): add Argon2id password-gated unseal policy`
  - Files: `bridge/pkg/keystore/sealed_keystore.go`, `bridge/pkg/keystore/sealed_keystore_test.go`
  - Pre-commit: `cd bridge && CGO_ENABLED=1 go test ./pkg/keystore/...`

- [x] 7. Auto-Seal Timer + Activity Tracking

  **What to do**:
  - Add to `SealedKeystore` (in `sealed_keystore.go`): `autoSealTimer *time.Timer`, `sessionExpiresAt time.Time`, `lastActivityAt time.Time`
  - Implement `resetTimerLocked()` called under Mutex after activity operations
  - Timer fires after 4 hours → calls `Seal()` to zero vaultKey and mark sealed
  - Activity operations (reset timer): unseal, extend_session, Get (credential read), pii.fulfill, provider key lookup
  - Non-activity operations (no reset): list_keys, delete_key, sealed, session_status
  - Implement `SessionStatus()` returning: sealed, remaining_seconds, last_activity_at
  - Implement `ExtendSession()` for explicit timer extension
  - Write failing tests FIRST: 4h timer fires, Get resets timer, list_keys does NOT reset, SessionStatus correct values

  **Must NOT do**:
  - Do NOT use `time.AfterFunc` with captures of sensitive data in closures
  - Do NOT create a separate goroutine that accesses vaultKey without locking

  **Recommended Agent Profile**:
  - **Category**: `deep`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES (after T6 foundation laid)
  - **Parallel Group**: Wave 2 (with T6, T8)
  - **Blocks**: T9
  - **Blocked By**: T6

  **References**:

  **Pattern References**:
  - `bridge/pkg/keystore/sealed_keystore.go:400-430` — Existing `ExtendSession` method (modifies ExpiresAt)
  - `bridge/pkg/keystore/sealed_keystore.go:514-524` — Existing `recordOperation` (updates LastAccess, Operations count)

  **Acceptance Criteria**:

  **If TDD:**
  - [ ] Tests in: `bridge/pkg/keystore/sealed_keystore_test.go`
  - [ ] `CGO_ENABLED=1 go test -v -run "TestAutoSeal|TestTimerReset|TestSessionStatus" ./pkg/keystore/...` → PASS

  **QA Scenarios (MANDATORY):**

  ```
  Scenario: Auto-seal fires after 4 hours (use short timeout in test)
    Tool: Bash
    Steps:
      1. CGO_ENABLED=1 go test -v -run TestAutoSealTimer ./pkg/keystore/... 2>&1
    Expected Result: Vault sealed after timer fires, vaultKey zeroed
    Evidence: .sisyphus/evidence/task-07-auto-seal-timer.txt

  Scenario: Get() resets timer, list_keys does not
    Tool: Bash
    Steps:
      1. CGO_ENABLED=1 go test -v -run TestTimerActivityTracking ./pkg/keystore/... 2>&1
    Expected Result: Get() updates lastActivityAt and expiresAt, list_keys does not
    Evidence: .sisyphus/evidence/task-07-timer-activity.txt

  Scenario: SessionStatus returns correct remaining_seconds
    Tool: Bash
    Steps:
      1. CGO_ENABLED=1 go test -v -run TestSessionStatus ./pkg/keystore/... 2>&1
    Expected Result: remaining_seconds decrements correctly, sealed=true when sealed
    Evidence: .sisyphus/evidence/task-07-session-status.txt
  ```

  **Commit**: YES (groups with T6 if sequential)
  - Message: `feat(keystore): add auto-seal timer with activity tracking`
  - Files: `bridge/pkg/keystore/sealed_keystore.go`
  - Pre-commit: `cd bridge && CGO_ENABLED=1 go test ./pkg/keystore/...`

- [x] 8. Keystore CRUD — list_keys, delete_key

  **What to do**:
  - Add `ListKeys() ([]string, error)` to `SealedKeystore` — returns key names only (not values), requires unsealed check
  - Add `DeleteKey(name string) error` — removes key, requires unsealed check
  - Both return `ErrKeystoreSealed` (-32005) when sealed
  - `ListKeys` delegates to base keystore's existing key listing
  - Write failing tests FIRST: list returns names, delete removes, both fail when sealed

  **Must NOT do**:
  - Do NOT expose key values through ListKeys
  - Do NOT reset the auto-seal timer on list/delete operations

  **Recommended Agent Profile**:
  - **Category**: `quick`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES (after T6)
  - **Parallel Group**: Wave 2 (with T6, T7)
  - **Blocks**: T9
  - **Blocked By**: T6

  **References**:

  **Pattern References**:
  - `bridge/pkg/keystore/sealed_keystore.go:466-511` — Existing `Retrieve`, `RetrieveProfile` patterns (check sealed, delegate to base)
  - `bridge/pkg/keystore/keystore.go` — Base keystore methods for key listing and deletion

  **Acceptance Criteria**:

  **If TDD:**
  - [ ] Tests in: `bridge/pkg/keystore/sealed_keystore_test.go` or new `crud_test.go`
  - [ ] `CGO_ENABLED=1 go test -v -run "TestListKeys|TestDeleteKey" ./pkg/keystore/...` → PASS

  **QA Scenarios (MANDATORY):**

  ```
  Scenario: ListKeys returns names only
    Tool: Bash
    Steps:
      1. CGO_ENABLED=1 go test -v -run TestListKeys ./pkg/keystore/... 2>&1
    Expected Result: Returns slice of key name strings, no values
    Evidence: .sisyphus/evidence/task-08-list-keys.txt

  Scenario: DeleteKey removes key
    Tool: Bash
    Steps:
      1. CGO_ENABLED=1 go test -v -run TestDeleteKey ./pkg/keystore/... 2>&1
    Expected Result: Key no longer appears in subsequent ListKeys
    Evidence: .sisyphus/evidence/task-08-delete-key.txt

  Scenario: Both fail when sealed
    Tool: Bash
    Steps:
      1. CGO_ENABLED=1 go test -v -run "TestListKeysSealed|TestDeleteKeySealed" ./pkg/keystore/... 2>&1
    Expected Result: Both return ErrKeystoreSealed (-32005)
    Evidence: .sisyphus/evidence/task-08-crud-sealed.txt
  ```

  **Commit**: YES (groups with T6-T7)
  - Message: `feat(keystore): add list_keys and delete_key CRUD operations`
  - Files: `bridge/pkg/keystore/sealed_keystore.go`, `bridge/pkg/keystore/crud_test.go`
  - Pre-commit: `cd bridge && CGO_ENABLED=1 go test ./pkg/keystore/...`

- [x] 9. Keystore RPC Handlers (7 methods) + Discovery Integration

  **What to do**:
  - Add 7 handler functions to `registerHandlers()` in `bridge/pkg/rpc/server.go` following existing pattern:
    - `handleKeystoreUnseal` → `keystore.unseal` — resolves client identity via `rpc/identity.go`, checks rate limiter, then calls `UnsealWithPassword`
    - `handleKeystoreSealed` → `keystore.sealed` — returns `sealed` boolean (true = sealed, false = unsealed)
    - `handleKeystoreSeal` → `keystore.seal` — calls `Seal()`, zeroes vaultKey
    - `handleKeystoreExtendSession` → `keystore.extend_session` — extends timer
    - `handleKeystoreSessionStatus` → `keystore.session_status` — returns status struct
    - `handleKeystoreListKeys` → `keystore.list_keys` — calls `ListKeys()`
    - `handleKeystoreDeleteKey` → `keystore.delete_key` — calls `DeleteKey(name)`
  - ALL 7 check `config.FeatureFlags.ZeroTrustKeystore` → return `-32601` when disabled
  - Update the canonical RPC method inventory that `a0_discover.sh` uses (currently centered around `scripts/lib/contract.sh` and the A0 contract helpers) with the 7 new method names. Verify structure by reading the discovery scripts first, then update whatever inventory mechanism A0 actually uses. Then verify manifest counts.
  - Verify: `bash scripts/a0_discover.sh` → 95 with flag off, 102 with flag on
  - Add `keystore-agent` suite to `scripts/a4_harness.sh` SUITE_MAP
  - Write failing tests FIRST

  **Must NOT do**:
  - Do NOT register handlers only when flag is on — handlers must always exist but check flag at call time

  **Recommended Agent Profile**: `unspecified-high`
  **Parallelization**: Wave 3 | Blocked By: T6, T7, T8 | Blocks: T19

  **References**:
  - `bridge/pkg/rpc/server.go:875-975` — `registerHandlers()` flat map literal pattern
  - `bridge/pkg/rpc/server.go:729-790` — Existing `store_key` handler pattern
  - `scripts/lib/contract.sh` — `_contract_bridge_rpc()` canonical method inventory (verify structure before modifying)
  - `scripts/a4_harness.sh` — `SUITE_MAP` associative array

  **QA Scenarios:**
  ```
  Scenario: Flag off returns -32601 for all keystore methods
    Tool: Bash
    Steps:
      1. For each method: echo RPC call | socat - UNIX-CONNECT:${BRIDGE_SOCK} (or use configured RPC transport — TCP if in Sentinel mode)
      2. Verify each response contains error code -32601
    Expected Result: All 7 methods return -32601
    Evidence: .sisyphus/evidence/task-09-flag-off-32601.txt

  Scenario: Discovery shows 102 with flag on
    Tool: Bash
    Steps:
      1. ARMORCLAW_FEATURE_ZERO_TRUST_KEYSTORE=true bash scripts/a0_discover.sh
      2. jq '.live_discovered.rpc_methods | length' manifest.json
    Expected Result: 102
    Evidence: .sisyphus/evidence/task-09-discovery-102.txt
  ```

  **Commit**: YES
  - Message: `feat(rpc): add 7 keystore.* RPC handlers + discovery`
  - Files: `bridge/pkg/rpc/server.go`, `scripts/lib/contract.sh`, `scripts/a4_harness.sh`

- [x] 10. Integrate Sealed Checks into Existing Flows

  **What to do**:
  - Modify `bridge/pkg/keystore/keystore.go`: Wrap existing `Get`/`Set` to check sealed state when flag on → `-32005`
  - Modify `bridge/pkg/pii/scrubber.go`: Check sealed before PII fulfill, reset timer on success
  - Modify `bridge/pkg/providers/registry.go`: Check sealed on key lookup, reset timer on success
  - Flag off → existing behavior unchanged

  **Must NOT do**:
  - Do NOT modify `pii/engine.go` (doesn't exist) — modify `scrubber.go`
  - Do NOT break existing flows when flag is off

  **Recommended Agent Profile**: `unspecified-high`
  **Parallelization**: Wave 3 | Blocked By: T6 | Blocks: T18

  **References**:
  - `bridge/pkg/keystore/keystore.go` — Base keystore Get/Set (1590 lines)
  - `bridge/pkg/pii/scrubber.go` — PII scrubber (559 lines)
  - `bridge/pkg/providers/registry.go:1-120` — Provider registry

  **QA Scenarios:**
  ```
  Scenario: Get blocked when sealed, allowed when unsealed
    Tool: Bash
    Steps: CGO_ENABLED=1 go test -v -run TestSealedGet ./pkg/keystore/...
    Expected Result: ErrKeystoreSealed when sealed, success when unsealed
    Evidence: .sisyphus/evidence/task-10-sealed-get.txt
  ```

  **Commit**: YES
  - Message: `refactor(keystore): integrate sealed checks into Get/Set/PII/provider`
  - Files: `bridge/pkg/keystore/keystore.go`, `bridge/pkg/pii/scrubber.go`, `bridge/pkg/providers/registry.go`

- [x] 11. Passphrase Validation + Bootstrap Password Generation

  **What to do**:
  - Add passphrase validation (≥12 chars, uppercase, lowercase, digit) to `bridge/pkg/provisioning/rpc.go`
  - Generate 24-char password via `crypto/rand` in `bridge/cmd/bootstrap-admin/main.go`
  - Verify no SHA-256 on password paths

  **Recommended Agent Profile**: `quick`
  **Parallelization**: Wave 3 | Blocked By: None | Blocks: None

  **References**: `bridge/pkg/provisioning/rpc.go`, `bridge/cmd/bootstrap-admin/main.go`

  **QA Scenarios:**
  ```
  Scenario: No SHA-256 on password derivation paths
    Tool: Bash
    Steps:
      1. Check all files in the password/unseal derivation path for SHA-256 usage: grep -n "sha256\|SHA-256\|crypto/sha256" bridge/cmd/bootstrap-admin/main.go bridge/pkg/provisioning/rpc.go bridge/pkg/keystore/sealed_keystore.go bridge/pkg/keystore/securemem.go bridge/pkg/keystore/key_derivation.go bridge/pkg/keystore/keystore.go 2>&1
    Expected Result: Empty output (no SHA-256 in these specific password-path files)
    Failure Indicators: Any matches in these files indicate SHA-256 on the password path
    Evidence: .sisyphus/evidence/task-11-no-sha256.txt
  ```

  **Commit**: YES
  - Message: `feat(provisioning): add passphrase strength validation`
  - Files: `bridge/pkg/provisioning/rpc.go`, `bridge/cmd/bootstrap-admin/main.go`

- [x] 12. Voice Stack — Uncomment + Extend with OpenAI Providers

  **What to do**:
  - **Step 1 — Audit**: Read `bridge/cmd/bridge/main.go` to find where voice manager is commented out. Understand ALL dependencies needed for uncommenting. Check if any additional setup beyond uncommenting is required (config structs, provider registration, wiring).
  - **Step 2 — Uncomment**: Only after audit confirms it's safe, uncomment voice manager initialization
  - **Step 3 — Extend**: Once uncommented, extend providers with OpenAI

  **Must NOT do**:
  - Do NOT create new stt.go/tts.go/vad.go — extend existing *_service.go files
  - Do NOT create pipeline.go — manager.go IS the pipeline
  - Do NOT download ONNX/native binaries
  - Do NOT modify `bridge/pkg/webrtc/` unless the Step 1 audit proves it is required for voice wiring

  **Recommended Agent Profile**: `unspecified-high`
  **Parallelization**: Wave 4 | Blocked By: T2 | Blocks: T13

  **References**:
  - `bridge/pkg/voice/manager.go:1-505` — Full voice manager, extend providers
  - `bridge/pkg/voice/stt_service.go`, `tts_service.go`, `vad_service.go` — Extend these
  - `bridge/cmd/bridge/main.go` — Find and uncomment voice init

  **QA Scenarios:**
  ```
  Scenario: Missing API key returns -32007
    Tool: Bash
    Steps: CGO_ENABLED=1 go test -v -run TestVoiceNotConfigured ./pkg/voice/...
    Expected Result: ErrVoiceNotConfigured
    Evidence: .sisyphus/evidence/task-12-voice-nokey.txt

  Scenario: No native binaries
    Tool: Bash
    Steps: find bridge/ -name "*.onnx" -o -name "*whisper*" -o -name "*piper*"
    Expected Result: Empty output
    Evidence: .sisyphus/evidence/task-12-no-binaries.txt
  ```

  **Commit**: YES
  - Message: `feat(voice): uncomment + extend voice manager with OpenAI providers`
  - Files: `bridge/cmd/bridge/main.go`, `bridge/pkg/voice/stt_service.go`, `bridge/pkg/voice/tts_service.go`, `bridge/pkg/voice/errors.go`

- [x] 13. Voice RPC Handlers (3 methods) + Discovery

  **What to do**:
  - Add 3 handlers: `voice.start_session`, `voice.stop_session`, `voice.status`
  - Check `VoicePipeline != "off"` → `-32601`
  - Update canonical method inventory that A0 uses → verify 105 with keystore+voice on

  **Recommended Agent Profile**: `unspecified-high`
  **Parallelization**: Wave 4 | Blocked By: T12 | Blocks: None

  **QA Scenarios:**
  ```
  Scenario: Discovery 105 with keystore+voice
    Tool: Bash
    Expected Result: 105 methods
    Evidence: .sisyphus/evidence/task-13-discovery-105.txt
  ```

  **Commit**: YES
  - Message: `feat(rpc): add 3 voice.* RPC handlers + discovery`
  - Files: `bridge/pkg/rpc/server.go`, `scripts/lib/contract.sh`

- [x] 14. Browser Multi-Tab Replay + Diagnostics

  **What to do**:
  - Create `jetski/navchart/multi_tab.go` — Tab-scoped NavChart storage
  - Modify `jetski/navchart/replay.go` — Optional `tab_id`, flag on: require it
  - Create `jetski/navchart/diagnostics.go` — Diff engine
  - Modify `bridge/pkg/browser/chart_types.go` — Add `TabID` to `NavChart`

  **Recommended Agent Profile**: `unspecified-high`
  **Parallelization**: Wave 4 | Blocked By: T2 | Blocks: T17

  **References**: `bridge/pkg/browser/chart_types.go:1-67` — NavChart types

  **Commit**: YES
  - Message: `feat(browser): add multi-tab NavChart replay + diagnostics`
  - Files: `jetski/navchart/multi_tab.go`, `jetski/navchart/diagnostics.go`, `jetski/navchart/replay.go`, `bridge/pkg/browser/chart_types.go`

- [x] 15. E2EE Key Backup Storage (BIP-39 + create/delete/exists)

  **What to do**:
  - **Before writing code**: verify the exact names of the existing 2 `e2ee.*` methods in the 95-method base by running `a0_discover.sh` and checking the manifest. Plan v7 assumed `e2ee_enable`/`e2ee_disable` but this must be confirmed against the live Bridge.
  - Create `bridge/pkg/crypto/recovery_phrase.go` — 24-word BIP-39 generation
  - Create `bridge/pkg/crypto/backup.go` — CreateBackup, DeleteBackup, BackupExists
  - Create `bridge/pkg/crypto/backup_store.go` — Store in Rust Vault
  - Check `FEATURE_E2EE_BACKUP` flag

  **Must NOT do**:
  - Do NOT implement `e2ee.restore_backup`
  - Do NOT modify existing crypto/engine.go

  **Recommended Agent Profile**: `deep`
  **Parallelization**: Wave 4 | Blocked By: T2 | Blocks: T16

  **References**: `bridge/pkg/crypto/engine.go:1-232`, `bridge/pkg/vault/`

  **QA Scenarios:**
  ```
  Scenario: restore_backup does NOT exist
    Tool: Bash
    Steps: grep -rn "restore_backup" bridge/
    Expected Result: 0 matches
    Evidence: .sisyphus/evidence/task-15-no-restore.txt
  ```

  **Commit**: YES
  - Message: `feat(crypto): add E2EE key backup with BIP-39 recovery`
  - Files: `bridge/pkg/crypto/recovery_phrase.go`, `bridge/pkg/crypto/backup.go`, `bridge/pkg/crypto/backup_store.go`

- [x] 16. E2EE Backup RPC Handlers (3 methods) + Discovery

  **What to do**: Add 3 handlers (`e2ee.create_backup`, `e2ee.delete_backup`, `e2ee.backup_exists`), check `FEATURE_E2EE_BACKUP` flag, update canonical method inventory, add `e2ee-backup` suite to harness. Verify `e2ee.restore_backup` is NOT in `registerHandlers()`.

  **Recommended Agent Profile**: `quick`
  **Parallelization**: Wave 5 | Blocked By: T15 | Blocks: None

  **Commit**: YES
  - Message: `feat(rpc): add e2ee backup handlers (create_backup, delete_backup, backup_exists) + discovery`

- [x] 17. Browser replay_diagnostics RPC Handler + Discovery

  **What to do**: Add handler, check `MultiTabReplay` flag, update probes, verify all flags on → 109

  **Recommended Agent Profile**: `quick`
  **Parallelization**: Wave 5 | Blocked By: T14 | Blocks: None

  **QA Scenarios:**
  ```
  Scenario: All flags on = 109 methods
    Tool: Bash
    Expected Result: 109
    Evidence: .sisyphus/evidence/task-17-discovery-109.txt
  ```

  **Commit**: YES
  - Message: `feat(rpc): add browser.replay_diagnostics handler + discovery`

- [x] 18. Memory Zeroization Audit + Fixes

  **What to do**:
  - Audit all sensitive byte paths: vaultKey on seal, Argon2id outputs in key_derivation.go, PII after BlindFill, email body/attachments
  - Use `ZeroBytes` from T3 to zero all sensitive temps on success AND failure paths
  - Replace existing ad-hoc zeroization loops (`for i := range derived.Key`) with `ZeroBytes()` calls
  - Ensure vaultKey is zeroed on both manual Seal() and auto-seal timer fire

  **Must NOT do**:
  - Do NOT claim zeroization of immutable Go strings
  - Do NOT use unsafe package

  **Recommended Agent Profile**: `deep`
  **Parallelization**: Wave 6 | Blocked By: T3, T10 | Blocks: None

  **References**: `bridge/pkg/keystore/key_derivation.go:181-183,229-231` — Existing ad-hoc zeroization to replace

  **Commit**: YES
  - Message: `security(keystore): audit + fix memory zeroization across all paths`

- [x] 19. Audit Logging for New Event Types

  **What to do**:
  - Extend `bridge/pkg/audit/audit.go` to log: keystore seal/unseal, backup create/delete, voice start/stop
  - Never log: passwords, verifiers, vault keys, recovery phrases, Ed25519 challenge nonces, wrapped keys
  - Audit denylist must NOT reference artifacts from the rejected challenge-only design, but MUST keep entries protecting still-active challenge-response secrets

  **Recommended Agent Profile**: `unspecified-high`
  **Parallelization**: Wave 6 | Blocked By: T9, T13, T16 | Blocks: None

  **Commit**: YES
  - Message: `feat(audit): add logging for keystore/voice/backup events`

- [x] 20. Legacy Browser Deprecation Notices

  **What to do**:
  - Add deprecation notice to `browser-service/README.md`
  - Log WARN in `bridge/pkg/browser/broker.go` when `ARMORCLAW_BROWSER_BACKEND=legacy`
  - Keep `legacy` as a documented option in config (it's still a supported fallback), but mark it deprecated in comments and docs

  **Recommended Agent Profile**: `quick`
  **Parallelization**: Wave 6 | Blocked By: None | Blocks: None

  **Commit**: YES
  - Message: `chore(browser): add legacy browser deprecation notices`

- [x] 21. Audit Denylist Cleanup

  **What to do**:
  - Remove HMAC proof references from audit denylist that originated from the rejected challenge-only design (the design where challenge was the ONLY unseal method, not the current alternative)
  - Do NOT remove denylist entries that protect still-active challenge-response secrets or tokens — those remain valid since challenge is kept as an alternative policy
  - Verify denylist covers: passwords, verifiers, vault_keys, recovery_phrases, wrapped_keys, challenge nonces

  **Recommended Agent Profile**: `quick`
  **Parallelization**: Wave 6 | Blocked By: None | Blocks: None

  **Commit**: YES
  - Message: `fix(audit): remove rejected-design-only artifact references from denylist`

- [x] 22. Full Harness — 5 Flag Configurations

  **What to do**:
  - Run full harness across 6 configurations:
    - A: all off → 95
    - B: keystore → 102
    - C: keystore + voice → 105
    - D: keystore + multi-tab → 106
    - E: keystore + e2ee backup → 105 (isolated E2EE validation: verifies E2EE flag works independently before all-on)
    - F: all on → 109
  - For each: run `a0_discover.sh`, verify method count, run relevant test suites
  - Capture all evidence

  **Recommended Agent Profile**: `deep`
  **Parallelization**: Wave 7 | Blocked By: T9, T13, T17, T16 | Blocks: T23

  **Commit**: YES
  - Message: `test(harness): full validation across 6 flag configurations`

- [x] 23. Contract Spot Checks + Method Count Verification

  **What to do**:
  - For each configuration, run jq spot checks on manifest:
    - Total method count (95/102/105/106/109 per configuration)
    - `keystore.*` count (7 when flag on, 0 when off)
    - `voice.*` count (3 when flag on, 0 when off)
    - `e2ee.restore_backup` count (always 0 — must never appear)
    - `e2ee.create_backup` + `e2ee.delete_backup` + `e2ee.backup_exists` count (0 when flag off, 3 when flag on)
    - `browser.replay_diagnostics` count (0 when FEATURE_MULTI_TAB_REPLAY off, 1 when on)
  - Verify no regression on existing methods

  **Recommended Agent Profile**: `quick`
  **Parallelization**: Wave 7 | Blocked By: T22 | Blocks: T24

  **Commit**: YES
  - Message: `test(discovery): contract spot checks + method count verification`

- [x] 24. Documentation Updates

  **What to do**:
  - Update `doc/architecture.md`: Update Bridge/server sections only — Feature Flags section with 95→109 table, method count update, zero-trust keystore section. Do NOT modify ArmorChat sections, client-side gap sections, or claim the system is globally gap-free. The architecture doc still describes ArmorChat as active development and voice as partial in `pkg/webrtc`; only update what this server plan actually delivers.
  - Update `doc/bridge-reference.md`: Add 14 methods with handler names, header "95–109"
  - Update `doc/testing.md`: Flag-dependent discovery, updated baseline
  - Create `CHANGELOG.md`: v1.0.0 entry

  **Recommended Agent Profile**: `writing`
  **Parallelization**: Wave 7 | Blocked By: T22, T23 | Blocks: None

  **Commit**: YES
  - Message: `docs: update architecture, bridge-reference, testing, CHANGELOG for v1.0.0`

---

## Final Verification Wave (MANDATORY — after ALL implementation tasks)

> 4 review agents run in PARALLEL. ALL must APPROVE. Present consolidated results to user and get explicit "okay" before completing.
>
> **Local vs CI scope**: CI only runs `go test ./pkg/rpc/...` (no CGO, no harness). The final verification commands below run **locally** with `CGO_ENABLED=1`, harness scripts, and full package coverage. "Green CI" alone does NOT mean the plan is complete — only a passing local full-validation run does.

- [x] F1. **Plan Compliance Audit** — `oracle`
  Read the plan end-to-end. For each "Must Have": verify implementation exists (read file, run test, check RPC response). For each "Must NOT Have": search codebase for forbidden patterns (`e2ee.restore_backup`, `config/flags.go`, `pii/engine.go`, AES-256-GCM imports). Check evidence files exist. Compare deliverables against plan.
  Output: `Must Have [N/N] | Must NOT Have [N/N] | Tasks [N/N] | VERDICT: APPROVE/REJECT`

- [x] F2. **Code Quality Review** — `unspecified-high`
  Run `cd bridge && go vet ./...` + `golangci-lint run` + `CGO_ENABLED=1 go test -v ./pkg/keystore/... ./pkg/voice/... ./pkg/crypto/... ./pkg/browser/... ./pkg/rpc/... ./pkg/config/... ./pkg/provisioning/... ./cmd/bootstrap-admin/...`. Review all changed files for: `interface{}` abuse where concrete types are known, empty `if err != nil {}` blocks, `fmt.Println`/`log.Println` in production code (use structured logger), commented-out code, unused imports, naked goroutines without recovery. Check AI slop: excessive comments, over-abstraction, generic variable names (`data`, `result`, `item`, `temp`), premature interface extraction.
  Output: `Build [PASS/FAIL] | Lint [PASS/FAIL] | Tests [N pass/N fail] | Files [N clean/N issues] | VERDICT`

- [x] F3. **Agent-Executed End-to-End QA Sweep** — `unspecified-high`
  Start from clean state. Execute EVERY QA scenario from EVERY task — follow exact steps, capture evidence. Test cross-task integration (keystore + PII, voice + agent, E2EE + crypto). Test edge cases: sealed state, wrong password, rate limit exceeded, flag toggles at runtime. Save to `.sisyphus/evidence/final-qa/`.
  Output: `Scenarios [N/N pass] | Integration [N/N] | Edge Cases [N tested] | VERDICT`

- [x] F4. **Scope Fidelity Check** — `deep`
  For each task: read "What to do", read actual diff (`git log --oneline`, `git diff`). Verify 1:1 — everything in spec was built, nothing beyond spec. Check "Must NOT do" compliance. Detect cross-task contamination. Flag unaccounted changes.
  Output: `Tasks [N/N compliant] | Contamination [CLEAN/N issues] | Unaccounted [CLEAN/N files] | VERDICT`

---

## Commit Strategy

- **T1**: `test(keystore): capture benchmark baseline before v1.0 changes`
- **T2**: `feat(config): add runtime feature flags for v1.0 phases`
- **T3**: `feat(keystore): add secure memory zeroization helper`
- **T4**: `feat(keystore): add per-identity rate limiter`
- **T5**: `feat(rpc): add client identity resolution for rate limiting`
- **T6**: `refactor(keystore): add Argon2id password-gated unseal policy`
- **T7**: `feat(keystore): add auto-seal timer with activity tracking`
- **T8**: `feat(keystore): add list_keys and delete_key CRUD operations`
- **T9**: `feat(rpc): add 7 keystore.* RPC handlers + discovery`
- **T10**: `refactor(keystore): integrate sealed checks into Get/Set/PII/provider`
- **T11**: `feat(provisioning): add passphrase strength validation`
- **T12**: `feat(voice): uncomment + extend voice manager with OpenAI providers`
- **T13**: `feat(rpc): add 3 voice.* RPC handlers + discovery`
- **T14**: `feat(browser): add multi-tab NavChart replay + diagnostics`
- **T15**: `feat(crypto): add E2EE key backup with BIP-39 recovery`
- **T16**: `feat(rpc): add 3 e2ee backup RPC handlers (create/delete/exists) + discovery`
- **T17**: `feat(rpc): add browser.replay_diagnostics handler + discovery`
- **T18**: `security(keystore): audit + fix memory zeroization across all paths`
- **T19**: `feat(audit): add logging for keystore/voice/backup events`
- **T20**: `chore(browser): add legacy browser deprecation notices`
- **T21**: `fix(audit): remove rejected-design-only artifact references from denylist`
- **T22**: `test(harness): full validation across 6 flag configurations`
- **T23**: `test(discovery): contract spot checks + method count verification`
- **T24**: `docs: update architecture, bridge-reference, testing, CHANGELOG for v1.0.0`

---

## Success Criteria

### Verification Commands
```bash
# Method count (all flags off)
cd ${REPO_ROOT} && bash scripts/a0_discover.sh
# Expected: 95 methods

# Method count (all flags on — requires flag environment variables set)
# Expected: 109 methods

# Go tests (requires CGO_ENABLED=1 for SQLCipher)
cd bridge && CGO_ENABLED=1 go test -v ./pkg/keystore/... ./pkg/voice/... ./pkg/crypto/... ./pkg/browser/... ./pkg/rpc/...
# Expected: all pass, 0 failures

# Bash harness
bash scripts/a4_harness.sh keystore-agent,voice,e2ee-backup
# Expected: all pass

# Verify e2ee.restore_backup does NOT exist
grep -rn "restore_backup" bridge/
# Expected: 0 matches

# Verify no config/flags.go exists
test ! -f bridge/pkg/config/flags.go && echo "OK: flags.go does not exist"

# Feature flag verification — each new method returns -32601 when disabled
echo '{"jsonrpc":"2.0","id":1,"method":"keystore.unseal","params":{}}' | socat - UNIX-CONNECT:${BRIDGE_SOCK}
# Expected: {"error":{"code":-32601,...}}
```

### Final Checklist
- [x] All "Must Have" present
- [x] All "Must NOT Have" absent
- [x] All Go tests pass (CGO_ENABLED=1)
- [x] Method count: 95 (flags off) / 109 (flags on)
- [x] Zero regression on existing 95-method base
- [x] Evidence captured for all QA scenarios
- [x] Documentation updated
