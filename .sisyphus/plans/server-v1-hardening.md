# ArmorClaw Server v1.0.0 Hardening Plan

## TL;DR

> **Quick Summary**: Wire the Zero-Trust keystore runtime, complete voice providers, validate E2EE backup and replay gating, then harden with contract tests and wipe verification.
>
> **Deliverables**:
> - SealedKS + KeystoreLimiter wired in main.go
> - Voice STT/TTS/VAD providers operational
> - E2EE backup and replay gating validated
> - Contract tests for all flag combinations (95/102/105/108)
> - Flow-level wipe verification for all secret buffers
> - Updated documentation
>
> **Estimated Effort**: Large
> **Parallel Execution**: YES — 4 waves
> **Critical Path**: T1 (SealedKS wiring) → T5 (passphrase hardening) → T10-T12 (contract tests)

---

## Context

### Original Request
Server-only execution of ArmorClaw v5.5 plan Part A — 7 phases (S1-S7) to harden the Bridge for v1.0.0 release.

### Codebase-Verified Findings
- **SealedKS gap CONFIRMED**: `rpcCfg.SealedKS` is never assigned in `main.go:2565-2612` — all 7 keystore handlers receive nil
- **KeystoreLimiter gap**: Also never wired — `server.go:937` checks `s.keystoreLimiter.Exceeded()` but it's always nil
- **SealedKeystore implementation EXISTS**: 1032 lines in `sealed_keystore.go` with full Argon2id + XChaCha20-Poly1305
- **E2EE BackupStore ALREADY WIRED**: Commit `4f64caf` — S5 is validation only
- **Voice RPC methods registered**: `voice.start_session`, `voice.stop_session`, `voice.status` in `server.go:1197-1199`
- **Interface mismatch**: `pkg/interfaces/voice.go` vs `pkg/voice` needs resolution
- **Method count**: 95 baseline → 102 keystore → 105 voice → 108 max

### Metis Review
- Guardrails applied: no scope widening, no new config files, no `e2ee.restore_backup`
- Key concern: ChallengeManager wiring deferred to v1.1 (password-only unseal for v1.0)

---

## Work Objectives

### Core Objective
Wire all v1.0.0 feature-flagged subsystems in main.go, complete voice providers, validate existing E2EE/replay wiring, and harden with comprehensive contract tests.

### Concrete Deliverables
- `main.go`: SealedKS + KeystoreLimiter construction gated by `feature_zero_trust_keystore`
- `pkg/voice/`: STT/TTS OpenAI backends + energy-threshold VAD + PCM routing
- Contract tests for all 4 flag combinations (95/102/105/108 methods)
- Flow-level wipe tests for password buffers, wrap_key, vault_key, nonces
- Updated architecture.md and bridge-reference.md

### Definition of Done
- [ ] `feature_zero_trust_keystore=true` → all 7 keystore methods return real data (not "not configured")
- [ ] `feature_voice_pipeline=cloud` → voice.start_session/stop_session/status work end-to-end
- [ ] `feature_e2ee_backup=true` → 3 backup methods work, init failure disables cleanly
- [ ] `feature_multi_tab_replay=true/false` → replay gating changes behavior, count stays 108
- [ ] All flag-off states return `-32601` for gated methods
- [ ] `go test ./bridge/...` passes with zero failures
- [ ] No secret material in logs or audit payloads

### Must Have
- SealedKS wired and functional
- Voice providers operational with rate-limit error handling (-32008)
- Contract tests covering all 4 flag combinations
- Wipe verification for all secret buffers
- Restart always begins sealed

### Must NOT Have (Guardrails)
- Do NOT implement `e2ee.restore_backup`
- Do NOT create `bridge/pkg/config/flags.go` — extend existing `config.go`
- Do NOT modify existing RPC handler signatures in `server.go`
- Do NOT remove existing challenge-response code (`challenge.go`) — it stays for v1.1
- Do NOT wire ChallengeManager in v1.0 — password-only unseal
- Do NOT add sealed boolean column to keystore.db schema
- Do NOT use AES-256-GCM — use existing XChaCha20-Poly1305 from `key_derivation.go`
- AI slop patterns to avoid: excessive comments, over-abstraction, generic names, premature generalization

---

## Verification Strategy

> **ZERO HUMAN INTERVENTION** — ALL verification is agent-executed. No exceptions.

### Test Decision
- **Infrastructure exists**: YES (Go test framework)
- **Automated tests**: YES (Tests-after — existing tests in place, new tests added)
- **Framework**: `go test`
- **Coverage target**: All newly wired methods have handler tests

### QA Policy
Every task MUST include agent-executed QA scenarios.
Evidence saved to `.sisyphus/evidence/task-{N}-{scenario-slug}.{ext}`.

- **Go tests**: Use Bash (`go test -v -run TestName ./bridge/...`)
- **RPC validation**: Use Bash (`echo '{"jsonrpc":"2.0","method":"...","id":1}' | socat - UNIX-CONNECT:/run/armorclaw/bridge.sock`)
- **Binary checks**: Use Bash (`go build`, `go vet`)

---

## Execution Strategy

### Parallel Execution Waves

```
Wave 1 (Foundation — start immediately):
├── T1: Wire SealedKS + KeystoreLimiter in main.go [deep]
├── T2: Validate E2EE BackupStore wiring + init-failure path [quick]
├── T3: Validate replay gating behavior (flag on/off) [quick]
└── T4: Resolve voice interface mismatch [quick]

Wave 2 (After Wave 1 — core implementation):
├── T5: Verify Argon2id params + passphrase policy + lockout (depends: T1) [deep]
├── T6: Implement STT backend — OpenAI Whisper (depends: T4) [unspecified-high]
├── T7: Implement TTS backend — OpenAI TTS (depends: T4) [unspecified-high]
└── T8: Implement energy-threshold VAD + PCM routing (depends: T4) [unspecified-high]

Wave 3 (After Wave 2 — hardening + tests):
├── T9: Feature-flag discovery validation — 95/102/105/108 (depends: T1-T8) [quick]
├── T10: Keystore handler contract tests (depends: T1, T5) [unspecified-high]
├── T11: Flow-level wipe verification tests (depends: T5) [deep]
├── T12: Voice end-to-end + rate-limit tests (depends: T6-T8) [unspecified-high]
└── T13: Invalid-state + edge-case contract tests (depends: T1-T8) [unspecified-high]

Wave 4 (After Wave 3 — docs):
└── T14: Update architecture.md + bridge-reference.md (depends: T9-T13) [writing]

Wave FINAL (After ALL tasks — 4 parallel reviews):
├── F1: Plan compliance audit (oracle)
├── F2: Code quality review (unspecified-high)
├── F3: Real manual QA (unspecified-high)
└── F4: Scope fidelity check (deep)
→ Present results → Get explicit user okay

Critical Path: T1 → T5 → T10,T11 → T13 → T14 → F1-F4
Parallel Speedup: ~60% faster than sequential
Max Concurrent: 4 (Waves 1 & 2)
```

### Dependency Matrix

| Task | Depends On | Blocks | Wave |
|------|-----------|--------|------|
| T1 | — | T5, T9, T10, T13 | 1 |
| T2 | — | T9, T13 | 1 |
| T3 | — | T9, T13 | 1 |
| T4 | — | T6, T7, T8 | 1 |
| T5 | T1 | T10, T11 | 2 |
| T6 | T4 | T12 | 2 |
| T7 | T4 | T12 | 2 |
| T8 | T4 | T12 | 2 |
| T9 | T1-T8 | T14 | 3 |
| T10 | T1, T5 | T14 | 3 |
| T11 | T5 | T14 | 3 |
| T12 | T6-T8 | T14 | 3 |
| T13 | T1-T8 | T14 | 3 |
| T14 | T9-T13 | F1-F4 | 4 |

### Agent Dispatch Summary

- **Wave 1**: 4 tasks — T1 → `deep`, T2-T4 → `quick`
- **Wave 2**: 4 tasks — T5 → `deep`, T6-T7 → `unspecified-high`, T8 → `unspecified-high`
- **Wave 3**: 5 tasks — T9 → `quick`, T10 → `unspecified-high`, T11 → `deep`, T12 → `unspecified-high`, T13 → `unspecified-high`
- **Wave 4**: 1 task — T14 → `writing`
- **FINAL**: 4 tasks — F1 → `oracle`, F2-F3 → `unspecified-high`, F4 → `deep`

---

## TODOs

- [x] 1. Wire SealedKS + KeystoreLimiter in main.go (Phase S1)

  **What to do**:
  - In `bridge/cmd/bridge/main.go`, inside `runBridgeServer()` around line 2587 where `cfg.Features.ZeroTrustKeystore` is checked:
    1. Construct `keystore.NewSealedKeystore()` with appropriate config (backing store path, session timeout, unseal policy = `PolicyPassword`)
    2. Assign to `rpcCfg.SealedKS`
    3. Construct `keystore.NewRateLimiter()` with sensible defaults (5 attempts per 60 seconds)
    4. Assign to `rpcCfg.KeystoreLimiter`
    5. Handle construction errors gracefully — log and disable the feature (do not fatal)
  - Config sourcing for SealedKeystore:
    - Backing store path: derive from existing keystore path in config (same base dir as `keystore.db`)
    - Session timeout: 5 minutes (default, from config if available — see `config.go` for existing timeout fields)
    - Unseal policy: `PolicyPassword` (constant — challenge-based deferred to v1.1)
    - All other `SealedKeystoreConfig` fields: use zero-value defaults unless `config.go` has overrides
  - Config sourcing for KeystoreLimiter:
    - Max attempts: 5 per identity per 60 seconds (sensible defaults)
    - Identity key: use user/device identifier from session context
  - The 7 RPC handlers are already registered (`server.go:1254-1260`) — no registration changes needed
  - The `Config.SealedKS` field already exists at `server.go:231` — no struct changes needed

  **Must NOT do**:
  - Do NOT wire ChallengeManager (deferred to v1.1)
  - Do NOT modify handler signatures in server.go
  - Do NOT create new config files
  - Do NOT add sealed column to keystore.db schema

  **Recommended Agent Profile**:
  - **Category**: `deep`
    - Reason: Security-critical wiring with multiple config dependencies, needs understanding of keystore lifecycle
  - **Skills**: []
  - **Skills Evaluated but Omitted**:
    - `git-master`: Not needed — single-file change with targeted commits

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 1 (with T2, T3, T4)
  - **Blocks**: T5, T9, T10, T13
  - **Blocked By**: None (can start immediately)

  **References**:

  **Pattern References**:
  - `bridge/cmd/bridge/main.go:2565-2612` — Current `rpcCfg` construction; line 2587 where `cfg.Features.ZeroTrustKeystore` flag is checked. Pattern to follow: same style as E2EEBackup block at lines ~2598-2603
  - `bridge/cmd/bridge/main.go:2598-2603` — E2EEBackup block showing feature-gated construction + error handling pattern (create store → check err → assign to rpcCfg)

  **API/Type References**:
  - `bridge/pkg/keystore/sealed_keystore.go` — `SealedKeystore` struct, `NewSealedKeystore()` constructor, config struct. Line 48: `PolicyPassword` unseal policy (password-only). Line 30-40: `SealedKeystoreConfig` fields
  - `bridge/pkg/rpc/server.go:231` — `Config.SealedKS *keystore.SealedKeystore` field (already defined)
  - `bridge/pkg/rpc/server.go:232` — `Config.KeystoreLimiter *keystore.RateLimiter` field (already defined)
  - `bridge/pkg/rpc/server.go:937` — `s.keystoreLimiter.Exceeded(identity)` usage in unseal handler
  - `bridge/pkg/rpc/server.go:1254-1260` — All 7 keystore handlers already registered
  - `bridge/pkg/keystore/sealed_keystore.go:101` — `challengeMgr` field — leave nil for v1.0 password-only

  **Test References**:
  - `bridge/pkg/rpc/keystore_handlers_test.go` — Existing handler tests showing mock setup pattern
  - `bridge/pkg/rpc/matrix_handler_test.go` — Pattern for testing feature-flagged handlers

  **WHY Each Reference Matters**:
  - `main.go:2598-2603`: The E2EEBackup block is the exact pattern to copy for SealedKS wiring
  - `sealed_keystore.go`: Constructor signature and config options — the implementer needs to know what `SealedKeystoreConfig` requires
  - `server.go:231-232`: These fields already exist — just need non-nil values assigned
  - `server.go:937`: Shows how KeystoreLimiter is used — confirms wiring is needed

  **Acceptance Criteria**:

  **QA Scenarios (MANDATORY):**

  ```
  Scenario: SealedKS wired — keystore.sealed returns true after restart
    Tool: Bash
    Preconditions: Bridge binary built with feature_zero_trust_keystore=true
    Steps:
      1. Build bridge: cd bridge && go build -o /tmp/armorclaw-bridge ./cmd/bridge
      2. Run unit tests: go test -v -run TestHandleKeystoreSealed ./pkg/rpc/...
      3. Verify test passes with "sealed" state
    Expected Result: Tests pass, keystore reports sealed=true on fresh start
    Failure Indicators: Test fails, or handler returns "sealed keystore not configured"
    Evidence: .sisyphus/evidence/task-1-sealed-after-restart.txt

  Scenario: KeystoreLimiter wired — rate limiting active
    Tool: Bash
    Preconditions: Bridge built with feature_zero_trust_keystore=true
    Steps:
      1. Run: go test -v -run TestKeystoreRateLimit ./pkg/rpc/... - or ./pkg/keystore/...
      2. Verify rate limiter records attempts and eventually blocks
    Expected Result: After N failed unseal attempts, further attempts are rejected
    Failure Indicators: Unlimited attempts allowed (limiter not wired)
    Evidence: .sisyphus/evidence/task-1-rate-limiter.txt

  Scenario: Feature flag off — all keystore methods return -32601
    Tool: Bash
    Preconditions: Bridge built with feature_zero_trust_keystore=false
    Steps:
      1. Run: go test -v -run TestKeystoreFeatureDisabled ./pkg/rpc/...
      2. Verify all 7 methods return MethodNotFound (-32601)
    Expected Result: All 7 return error code -32601
    Failure Indicators: Any method returns a different error or succeeds
    Evidence: .sisyphus/evidence/task-1-flag-off.txt

  Scenario: Construction error — feature gracefully disabled
    Tool: Bash
    Preconditions: Bridge built with invalid keystore path
    Steps:
      1. Run: go test -v -run TestSealedKSInitFailure ./pkg/rpc/...
      2. Verify bridge does not crash, methods return "not configured"
    Expected Result: No panic, graceful degradation
    Failure Indicators: Bridge crashes on startup
    Evidence: .sisyphus/evidence/task-1-init-failure.txt
  ```

  **Commit**: YES
  - Message: `feat(keystore): wire SealedKS and KeystoreLimiter in main.go`
  - Files: `bridge/cmd/bridge/main.go`
  - Pre-commit: `cd bridge && go test ./pkg/rpc/... ./pkg/keystore/...`

- [x] 2. Validate E2EE BackupStore Wiring + Init-Failure Path (Phase S5)

  **What to do**:
  - Read `bridge/cmd/bridge/main.go` around lines 2598-2603 to verify BackupStore construction
  - Verify the 3 E2EE handlers are registered: `e2ee.create_backup`, `e2ee.delete_backup`, `e2ee.backup_exists`
  - Write validation tests:
    1. Feature on + valid path → backup methods work
    2. Feature on + invalid path → init failure disables methods cleanly (no crash, methods return error)
    3. Feature off → methods return `-32601`
  - Verify `e2ee.restore_backup` is NOT registered (search for it, confirm absent)

  **Must NOT do**:
  - Do NOT implement `e2ee.restore_backup`
  - Do NOT modify the existing BackupStore wiring

  **Recommended Agent Profile**:
  - **Category**: `quick`
    - Reason: Validation of existing wiring, write tests only
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 1 (with T1, T3, T4)
  - **Blocks**: T9, T13
  - **Blocked By**: None

  **References**:

  **Pattern References**:
  - `bridge/cmd/bridge/main.go:2598-2603` — E2EEBackup block showing existing wiring pattern

  **API/Type References**:
  - `bridge/pkg/crypto/backup.go` — BackupManager with argon2id key derivation
  - `bridge/pkg/crypto/backup_store.go` — BackupStore with file-system persistence
  - `bridge/pkg/rpc/e2ee_backup_handlers.go` — 3 handlers (create_backup, delete_backup, backup_exists)
  - `bridge/pkg/rpc/server.go:1197-1199` — E2EE handler registrations

  **WHY Each Reference Matters**:
  - `main.go:2598-2603`: Confirms the wiring is already done — just needs test validation
  - `e2ee_backup_handlers.go`: Shows handler implementation — needed for understanding test targets

  **Acceptance Criteria**:

  **QA Scenarios:**

  ```
  Scenario: E2EE backup methods work with flag on
    Tool: Bash
    Preconditions: Bridge with feature_e2ee_backup=true
    Steps:
      1. go test -v -run TestE2EEBackup ./pkg/rpc/...
      2. Verify create_backup, delete_backup, backup_exists handlers respond
    Expected Result: All 3 methods return success responses
    Failure Indicators: Any method returns error or "not configured"
    Evidence: .sisyphus/evidence/task-2-backup-flag-on.txt

  Scenario: E2EE backup init failure disables cleanly
    Tool: Bash
    Preconditions: Bridge with invalid backup store path
    Steps:
      1. go test -v -run TestE2EEBackupInitFailure ./pkg/rpc/...
      2. Verify methods return graceful error, no crash
    Expected Result: "backup not available" error, no panic
    Failure Indicators: Bridge panics or methods return raw nil pointer error
    Evidence: .sisyphus/evidence/task-2-init-failure.txt

  Scenario: e2ee.restore_backup does NOT exist
    Tool: Bash
    Steps:
      1. grep -r "restore_backup" bridge/pkg/rpc/ — confirm zero hits
      2. grep -r "restore_backup" bridge/cmd/ — confirm zero hits
    Expected Result: Zero occurrences found
    Failure Indicators: Any file contains "restore_backup"
    Evidence: .sisyphus/evidence/task-2-no-restore.txt
  ```

  **Commit**: YES
  - Message: `test(e2ee): validate backup store wiring and init-failure path`
  - Files: `bridge/pkg/rpc/e2ee_backup_validation_test.go`
  - Pre-commit: `cd bridge && go test -v -run TestE2EE ./pkg/rpc/...`

- [x] 3. Validate Replay Gating Behavior (Phase S4)

  **What to do**:
  - Read the `browser.replay_diagnostics` handler in `server.go` or the browser handlers file
  - Verify the handler checks `s.flags.MultiTabReplay` internally (handler-gated, not flag-registered)
  - Write validation tests:
    1. Flag on → handler returns replay diagnostics data
    2. Flag off → handler returns feature-disabled error (NOT -32601, since method is always registered)
    3. Discovery still reports 108 methods regardless of flag state
  - Verify `tab_id` parameter handling

  **Must NOT do**:
  - Do NOT change the method registration pattern
  - Do NOT make replay count-expanding

  **Recommended Agent Profile**:
  - **Category**: `quick`
    - Reason: Read and validate existing code, write targeted tests
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 1 (with T1, T2, T4)
  - **Blocks**: T9, T13
  - **Blocked By**: None

  **References**:

  **Pattern References**:
  - `bridge/pkg/rpc/server.go` — Search for `replay_diagnostics` handler registration and implementation

  **API/Type References**:
  - `bridge/pkg/rpc/server.go:1202` area — Handler registration map
  - `bridge/pkg/config/config.go:63` — `MultiTabReplay bool` feature flag

  **WHY Each Reference Matters**:
  - Need to find the actual handler to verify it's handler-gated not flag-gated

  **Acceptance Criteria**:

  **QA Scenarios:**

  ```
  Scenario: Replay diagnostics changes behavior with flag
    Tool: Bash
    Steps:
      1. go test -v -run TestReplayGating ./pkg/rpc/...
      2. Verify flag-on returns data, flag-off returns feature-disabled error
    Expected Result: Different behavior with flag state, but method always discoverable
    Failure Indicators: Method returns -32601 when flag off (should be feature error)
    Evidence: .sisyphus/evidence/task-3-replay-gating.txt

  Scenario: Discovery count remains 108 regardless of replay flag
    Tool: Bash
    Steps:
      1. go test -v -run TestDiscoveryCount ./pkg/rpc/...
      2. Verify registered method count is 108 with flag on AND off
    Expected Result: 108 in both cases
    Failure Indicators: Count changes to 107 or 109
    Evidence: .sisyphus/evidence/task-3-discovery-count.txt
  ```

  **Commit**: YES
  - Message: `test(replay): validate multi-tab replay gating behavior`
  - Files: `bridge/pkg/rpc/replay_gating_test.go`
  - Pre-commit: `cd bridge && go test -v -run TestReplay ./pkg/rpc/...`

- [x] 4. Resolve Voice Interface Mismatch (Phase S3 foundation)

  **What to do**:
  - Read `bridge/pkg/interfaces/voice.go` and `bridge/pkg/voice/` to understand the mismatch
  - Determine which package owns the canonical voice interfaces
  - Resolve the mismatch — either:
    a. Make `pkg/voice/` implement the interfaces from `pkg/interfaces/voice.go`
    b. Or consolidate interfaces into `pkg/voice/` and update references
  - Ensure no compilation errors after resolution
  - Run `go vet ./bridge/...` to verify

  **Must NOT do**:
  - Do NOT break existing voice handler code
  - Do NOT create new packages

  **Recommended Agent Profile**:
  - **Category**: `quick`
    - Reason: Interface consolidation, targeted refactoring
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 1 (with T1, T2, T3)
  - **Blocks**: T6, T7, T8
  - **Blocked By**: None

  **References**:

  **Pattern References**:
  - `bridge/pkg/rpc/server.go:197-202` — `VoiceProvider` interface used by RPC server
  - `bridge/pkg/voice/` — Existing voice implementations

  **API/Type References**:
  - `bridge/pkg/rpc/server.go:197-202` — `VoiceProvider` interface used by RPC handlers (THIS is the runtime interface that matters)
  - `bridge/pkg/interfaces/voice.go` — Canonical voice interfaces (if exists — may be redundant with VoiceProvider)
  - `bridge/pkg/voice/stt_service.go` — Existing STT service
  - `bridge/pkg/voice/tts_service.go` — Existing TTS service

  **WHY Each Reference Matters**:
  - `server.go:197-202`: The VoiceProvider interface in RPC server is the primary contract — T4 must make `pkg/voice/` satisfy BOTH `interfaces.VoiceManager` AND `server.go:VoiceProvider`
  - `interfaces/voice.go`: May define additional interfaces that pkg/voice must also satisfy — need to reconcile both

  **Acceptance Criteria**:

  **QA Scenarios:**

  ```
  Scenario: Voice interfaces resolve — code compiles
    Tool: Bash
    Steps:
      1. cd bridge && go build ./...
      2. go vet ./...
    Expected Result: Zero compilation errors, zero vet warnings
    Failure Indicators: Type mismatch errors, undefined method errors
    Evidence: .sisyphus/evidence/task-4-voice-interfaces.txt

  Scenario: Existing voice tests still pass
    Tool: Bash
    Steps:
      1. cd bridge && go test -v ./pkg/voice/...
    Expected Result: All existing tests pass
    Failure Indicators: Any test failure
    Evidence: .sisyphus/evidence/task-4-voice-tests.txt
  ```

  **Commit**: YES
  - Message: `refactor(voice): resolve interface mismatch between pkg/interfaces and pkg/voice`
  - Files: `bridge/pkg/voice/`, `bridge/pkg/interfaces/voice.go` (if modified)
  - Pre-commit: `cd bridge && go build ./... && go test ./pkg/voice/...`

- [x] 5. Verify Argon2id Params + Passphrase Policy + Lockout (Phase S2)

  **What to do**:
  - Read `bridge/pkg/keystore/key_derivation.go` — verify Argon2id parameters:
    - Time cost ≥ 3
    - Memory ≥ 64MB (65536 KB)
    - Threads ≥ 4
    - Output key length = 32 bytes (256 bits for XChaCha20-Poly1305)
  - Verify salt generation: crypto/rand, unique per vault, minimum 16 bytes
  - Read `bridge/pkg/keystore/sealed_keystore.go` — verify:
    - Passphrase policy: minimum length, complexity requirements
    - Failed-unseal lockout: attempt counting, lockout duration, persistent across sessions (or not)
    - Idle auto-seal: timer starts on last activity, seals after configured timeout
    - Session extension: extends timer on activity
  - Read `bridge/pkg/keystore/` for any rate limiter implementation
  - Fix any gaps found (e.g., weak passphrase policy, missing lockout)
  - Write verification tests for all paths

  **Must NOT do**:
  - Do NOT change the encryption algorithm (keep XChaCha20-Poly1305)
  - Do NOT add sealed column to database schema
  - Do NOT persist lockout state to disk (in-memory is fine for v1.0)

  **Recommended Agent Profile**:
  - **Category**: `deep`
    - Reason: Security-sensitive cryptographic parameter verification, needs careful analysis
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES (with T6-T8)
  - **Parallel Group**: Wave 2
  - **Blocks**: T10, T11
  - **Blocked By**: T1 (needs SealedKS wired to test runtime behavior)

  **References**:

  **Pattern References**:
  - `bridge/pkg/keystore/sealed_keystore.go` — Full sealed keystore implementation (1032 lines)
  - `bridge/pkg/keystore/key_derivation.go` — Argon2id + XChaCha20-Poly1305

  **API/Type References**:
  - `bridge/pkg/keystore/sealed_keystore.go:30-40` — `SealedKeystoreConfig` struct (session timeout, policy settings)
  - `bridge/pkg/keystore/sealed_keystore.go:48` — `PolicyPassword` constant
  - `bridge/pkg/keystore/sealed_keystore.go:101` — `challengeMgr` field (leave nil for v1.0)
  - `bridge/pkg/rpc/server.go:937` — `keystoreLimiter.Exceeded(identity)` rate limiting check
  - `bridge/pkg/config/config.go:1090` — Default config values

  **WHY Each Reference Matters**:
  - `key_derivation.go`: The Argon2id parameters live here — need to verify they're OWASP-compliant
  - `sealed_keystore.go:30-40`: Config struct determines session timeout and policy
  - `server.go:937`: Rate limiting check — needs KeystoreLimiter wired (T1) to work

  **Acceptance Criteria**:

  **QA Scenarios:**

  ```
  Scenario: Argon2id parameters meet security requirements
    Tool: Bash
    Steps:
      1. go test -v -run TestArgon2idParams ./pkg/keystore/...
      2. Verify time≥3, memory≥64MB, threads≥4, keyLen=32
    Expected Result: All parameters meet or exceed minimums
    Failure Indicators: Any parameter below threshold
    Evidence: .sisyphus/evidence/task-5-argon2id-params.txt

  Scenario: Lockout blocks after N failed attempts
    Tool: Bash
    Steps:
      1. go test -v -run TestUnsealLockout ./pkg/keystore/...
      2. Attempt unseal with wrong password 6 times
      3. Verify 6th+ attempt is rejected with lockout error
    Expected Result: Lockout activates, further attempts blocked
    Failure Indicators: Unlimited attempts allowed
    Evidence: .sisyphus/evidence/task-5-lockout.txt

  Scenario: Idle auto-seal works
    Tool: Bash
    Steps:
      1. go test -v -run TestAutoSeal ./pkg/keystore/...
      2. Unseal vault, wait for session timeout, verify auto-sealed
    Expected Result: Vault seals after idle timeout
    Failure Indicators: Vault remains unsealed indefinitely
    Evidence: .sisyphus/evidence/task-5-auto-seal.txt

  Scenario: Session extension resets timer
    Tool: Bash
    Steps:
      1. go test -v -run TestSessionExtension ./pkg/keystore/...
      2. Unseal, wait nearly timeout, extend session, verify not sealed
    Expected Result: Session timer resets on extension
    Failure Indicators: Extension doesn't reset timer
    Evidence: .sisyphus/evidence/task-5-session-extend.txt
  ```

  **Commit**: YES
  - Message: `security(keystore): verify Argon2id params, passphrase policy, lockout behavior`
  - Files: `bridge/pkg/keystore/`
  - Pre-commit: `cd bridge && go test -v ./pkg/keystore/...`

- [x] 6. Implement STT Backend — OpenAI Whisper (Phase S3)

  **What to do**:
  - Create `bridge/pkg/voice/stt_openai.go`
  - Implement OpenAI Whisper API integration for speech-to-text:
    1. Accept PCM audio input
    2. Convert to format Whisper expects (WAV/MP3)
    3. Call OpenAI `/v1/audio/transcriptions` endpoint
    4. Return transcribed text
  - Handle rate-limit/quota errors with application error code `-32008`
  - Implement the interface resolved in T4
  - Handle configuration: API key from config/env, model selection
  - **Config fields**: Verify `config.go` has voice-specific fields (OpenAI API key, STT model, TTS voice, etc.). If missing, add them following existing provider key patterns (see `pkg/providers/` or AI provider config in `config.go`). This is within the "extend existing config.go" guardrail.

  **Must NOT do**:
  - Do NOT download ONNX models or native binaries
  - Do NOT implement local STT — cloud only for v1.0
  - Do NOT create a new voice package — extend existing `pkg/voice/`

  **Recommended Agent Profile**:
  - **Category**: `unspecified-high`
    - Reason: External API integration with error handling, moderate complexity
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 2 (with T5, T7, T8)
  - **Blocks**: T12
  - **Blocked By**: T4 (interface resolution)

  **References**:

  **Pattern References**:
  - `bridge/pkg/voice/stt_service.go` — Existing STT service to extend
  - `bridge/pkg/ai/provider.go` or similar — Pattern for external API calls with error handling

  **API/Type References**:
  - `bridge/pkg/rpc/server.go:197` — `VoiceProvider` interface
  - `bridge/pkg/config/config.go` — Voice-related config fields, API key storage
  - `bridge/pkg/errors/codes.go` — Error codes including voice-specific ones

  **External References**:
  - OpenAI Whisper API: `https://platform.openai.com/docs/api-reference/audio/createTranscription`

  **WHY Each Reference Matters**:
  - `stt_service.go`: Shows existing STT structure — build on it, don't replace
  - `server.go:197`: Must implement this interface for RPC handler compatibility

  **Acceptance Criteria**:

  **QA Scenarios:**

  ```
  Scenario: STT transcribes audio to text
    Tool: Bash
    Steps:
      1. go test -v -run TestOpenAISTT ./pkg/voice/...
      2. Provide PCM audio input, verify text output
    Expected Result: Transcription returned (may be mocked in test)
    Failure Indicators: Empty output, unhandled error
    Evidence: .sisyphus/evidence/task-6-stt.txt

  Scenario: Rate limit returns -32008
    Tool: Bash
    Steps:
      1. go test -v -run TestOpenAISTTRateLimit ./pkg/voice/...
      2. Simulate 429 response from OpenAI
    Expected Result: Application error with code -32008
    Failure Indicators: Raw HTTP error exposed, or panic
    Evidence: .sisyphus/evidence/task-6-stt-ratelimit.txt
  ```

  **Commit**: YES
  - Message: `feat(voice): implement OpenAI Whisper STT backend`
  - Files: `bridge/pkg/voice/stt_openai.go`
  - Pre-commit: `cd bridge && go build ./... && go test ./pkg/voice/...`

- [x] 7. Implement TTS Backend — OpenAI TTS (Phase S3)

  **What to do**:
  - Create `bridge/pkg/voice/tts_openai.go`
  - Implement OpenAI TTS API integration for text-to-speech:
    1. Accept text input
    2. Call OpenAI `/v1/audio/speech` endpoint
    3. Return PCM audio output
  - Handle rate-limit/quota errors with application error code `-32008`
  - Implement the interface resolved in T4
  - Handle configuration: API key, voice selection, model
  - **Config fields**: Same as T6 — verify/add TTS-specific config fields (TTS voice name, model) in `config.go` if missing.

  **Must NOT do**:
  - Do NOT download ONNX models or native binaries
  - Do NOT implement local TTS — cloud only for v1.0

  **Recommended Agent Profile**:
  - **Category**: `unspecified-high`
    - Reason: External API integration, mirrors T6 pattern
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 2 (with T5, T6, T8)
  - **Blocks**: T12
  - **Blocked By**: T4

  **References**:

  **Pattern References**:
  - `bridge/pkg/voice/tts_service.go` — Existing TTS service to extend

  **API/Type References**:
  - `bridge/pkg/rpc/server.go:197` — `VoiceProvider` interface
  - `bridge/pkg/config/config.go` — Voice config fields

  **External References**:
  - OpenAI TTS API: `https://platform.openai.com/docs/api-reference/audio/createSpeech`

  **WHY Each Reference Matters**:
  - Same pattern as T6 but for output direction

  **Acceptance Criteria**:

  **QA Scenarios:**

  ```
  Scenario: TTS converts text to audio
    Tool: Bash
    Steps:
      1. go test -v -run TestOpenAITTS ./pkg/voice/...
      2. Provide text, verify PCM audio output
    Expected Result: Non-empty audio bytes returned
    Failure Indicators: Empty output, panic
    Evidence: .sisyphus/evidence/task-7-tts.txt

  Scenario: Rate limit returns -32008
    Tool: Bash
    Steps:
      1. go test -v -run TestOpenAITTSRateLimit ./pkg/voice/...
    Expected Result: Application error with code -32008
    Evidence: .sisyphus/evidence/task-7-tts-ratelimit.txt
  ```

  **Commit**: YES
  - Message: `feat(voice): implement OpenAI TTS backend`
  - Files: `bridge/pkg/voice/tts_openai.go`
  - Pre-commit: `cd bridge && go build ./... && go test ./pkg/voice/...`

- [x] 8. Implement Energy-Threshold VAD + PCM Routing (Phase S3)

  **What to do**:
  - Create `bridge/pkg/voice/vad.go` — energy-threshold voice activity detection:
    1. Read PCM audio stream
    2. Calculate RMS energy per frame
    3. Detect speech onset/offset based on configurable threshold
    4. Return VAD events (speech_start, speech_end, silence)
  - Create `bridge/pkg/voice/pcm.go` (or extend existing) — PCM routing at Bridge level:
    1. Route incoming PCM through VAD → STT (text output at Bridge side)
    2. Route agent text responses → TTS (Bridge side) → PCM output
    3. The audio pipeline terminates entirely within the Bridge process
    4. Agent containers receive/send **text only** via existing stdin/stdout mechanism
    5. NO audio enters or leaves agent containers (they run with NetworkMode: none)
    6. Handle sample rate conversion if needed
    7. Buffer management for streaming
  - Wire VAD into the voice session lifecycle
  - **Config fields**: Verify/add VAD-specific config fields (energy threshold, frame duration) in `config.go` if missing.

  **CRITICAL Architecture Note**:
  Agent containers execute with `NetworkMode: none` — zero network access. The audio pipeline MUST be:
  ```
  Input PCM → VAD (Bridge) → STT (Bridge) → text → agent container (stdin/stdout, text only)
  agent container → text (stdout) → TTS (Bridge) → output PCM
  ```
  Audio never reaches the agent container. Text bridges between Bridge audio pipeline and agent I/O.

  **Must NOT do**:
  - Do NOT download ONNX models — energy-threshold only for v1.0
  - Do NOT modify `bridge/pkg/webrtc/` unless audit proves necessary
  - Do NOT route audio to/from agent containers — they have NetworkMode: none

  **Recommended Agent Profile**:
  - **Category**: `unspecified-high`
    - Reason: Audio processing with real-time constraints, non-trivial but well-scoped
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 2 (with T5, T6, T7)
  - **Blocks**: T12
  - **Blocked By**: T4

  **References**:

  **Pattern References**:
  - `bridge/pkg/voice/stt_service.go` — Existing voice service structure

  **API/Type References**:
  - `bridge/pkg/voice/matrix.go` — Voice over Matrix (MatrixCall state)
  - `bridge/pkg/rpc/server.go:197` — VoiceProvider interface
  - `bridge/pkg/websocket/` — WebSocket streaming (if PCM routes through WS)

  **WHY Each Reference Matters**:
  - `matrix.go`: Shows how voice integrates with Matrix — PCM needs to flow through this
  - `server.go:197`: Must be compatible with VoiceProvider interface

  **Acceptance Criteria**:

  **QA Scenarios:**

  ```
  Scenario: VAD detects speech in PCM audio
    Tool: Bash
    Steps:
      1. go test -v -run TestVAD ./pkg/voice/...
      2. Feed PCM with known speech segment, verify speech_start/speech_end events
    Expected Result: VAD events fired at correct boundaries
    Failure Indicators: No events, or events on silence
    Evidence: .sisyphus/evidence/task-8-vad.txt

  Scenario: PCM routing — audio terminates at Bridge, text to agent
    Tool: Bash
    Steps:
      1. go test -v -run TestPCMRouting ./pkg/voice/...
      2. Verify: PCM input → VAD → STT → text output (Bridge side)
      3. Verify: text input → TTS → PCM output (Bridge side)
      4. Verify: NO audio bytes reach agent container interface
    Expected Result: Audio pipeline complete within Bridge process, agent receives text only
    Failure Indicators: Audio bytes leak to agent interface, or text not produced from STT
    Evidence: .sisyphus/evidence/task-8-pcm-routing.txt
  ```

  **Commit**: YES
  - Message: `feat(voice): implement energy-threshold VAD and PCM routing`
  - Files: `bridge/pkg/voice/vad.go`, `bridge/pkg/voice/pcm.go`
  - Pre-commit: `cd bridge && go build ./... && go test ./pkg/voice/...`

- [x] 9. Feature-Flag Discovery Validation — 95/102/105/108 (Phase S6)

  **What to do**:
  - Write a comprehensive test that validates RPC method discovery counts for ALL flag combinations:
    1. All flags OFF → 95 methods discoverable
    2. `feature_zero_trust_keystore=true` only → 102 methods
    3. `+feature_voice_pipeline=cloud` → 105 methods
    4. `+feature_e2ee_backup=true` and/or `+feature_multi_tab_replay=true` → 108 methods
  - Use the actual `registerHandlers()` method to count registered handlers
  - Verify specific methods appear/disappear with each flag state
  - Document the exact method list for each combination

  **Must NOT do**:
  - Do NOT modify handler registration logic
  - Do NOT change method counts

  **Recommended Agent Profile**:
  - **Category**: `quick`
    - Reason: Focused validation test, well-defined expectations
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 3
  - **Blocks**: T14
  - **Blocked By**: T1-T8 (all implementations must be complete)

  **References**:

  **Pattern References**:
  - `bridge/pkg/rpc/server_test.go` — Existing method count tests (13 and 40 line references)

  **API/Type References**:
  - `bridge/pkg/rpc/server.go:1190-1260` — `registerHandlers()` map with all RPC methods
  - `bridge/pkg/config/config.go:63` — Feature flag definitions

  **WHY Each Reference Matters**:
  - `server_test.go`: Already has method count tests — extend the pattern
  - `registerHandlers()`: The single source of truth for method registration

  **Acceptance Criteria**:

  **QA Scenarios:**

  ```
  Scenario: All 4 flag combinations produce correct method counts
    Tool: Bash
    Steps:
      1. go test -v -run TestFlagDiscoveryMatrix ./pkg/rpc/...
      2. Test all 4 combinations, assert exact counts
    Expected Result: 95, 102, 105, 108 for respective combinations
    Failure Indicators: Any count off by 1+
    Evidence: .sisyphus/evidence/task-9-discovery-matrix.txt
  ```

  **Commit**: YES
  - Message: `test(rpc): feature-flag discovery validation for all combinations`
  - Files: `bridge/pkg/rpc/discovery_test.go`
  - Pre-commit: `cd bridge && go test -v -run TestFlagDiscovery ./pkg/rpc/...`

- [x] 10. Keystore Handler Contract Tests (Phase S6)

  **What to do**:
  - Write contract tests for all 7 keystore RPC handlers:
    1. `keystore.unseal` — correct password → success; wrong → error; when sealed → error
    2. `keystore.sealed` — reports correct state before/after unseal/seal
    3. `keystore.seal` — unsealed → sealed transition
    4. `keystore.extend_session` — extends session, returns new expiry
    5. `keystore.session_status` — returns state, expiry, unsealed status
    6. `keystore.list_keys` — returns key list (empty initially)
    7. `keystore.delete_key` — deletes specified key
  - Test sealed-state behavior: all mutation methods return application errors (not -32601)
  - Test disabled-feature behavior: all return -32601

  **Must NOT do**:
  - Do NOT test challenge-based unseal (deferred to v1.1)

  **Recommended Agent Profile**:
  - **Category**: `unspecified-high`
    - Reason: Comprehensive contract testing across 7 methods with multiple states each
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 3
  - **Blocks**: T14
  - **Blocked By**: T1 (SealedKS wired), T5 (passphrase verified)

  **References**:

  **Pattern References**:
  - `bridge/pkg/rpc/keystore_handlers_test.go` — Existing handler tests (70 lines, shows mock setup)

  **API/Type References**:
  - `bridge/pkg/rpc/server.go:1254-1260` — All 7 handler registrations
  - `bridge/pkg/keystore/sealed_keystore.go` — Full implementation to test against
  - `bridge/pkg/rpc/server.go:937` — Rate limiter integration in handlers

  **WHY Each Reference Matters**:
  - `keystore_handlers_test.go`: Shows the mock pattern for MatrixAdapter — need similar for SealedKeystore
  - `sealed_keystore.go`: Implementation details needed for writing accurate test assertions

  **Acceptance Criteria**:

  **QA Scenarios:**

  ```
  Scenario: All 7 keystore handlers respond correctly when wired
    Tool: Bash
    Steps:
      1. go test -v -run TestKeystoreContract ./pkg/rpc/...
      2. Test each method in sealed/unsealed/disabled states
    Expected Result: 7 methods × 3 states = 21 test cases, all pass
    Failure Indicators: Any method returns unexpected error or nil
    Evidence: .sisyphus/evidence/task-10-keystore-contract.txt
  ```

  **Commit**: YES
  - Message: `test(keystore): handler contract tests for all 7 methods`
  - Files: `bridge/pkg/rpc/keystore_contract_test.go`
  - Pre-commit: `cd bridge && go test -v -run TestKeystoreContract ./pkg/rpc/...`

- [x] 11. Flow-Level Wipe Verification Tests (Phase S6)

  **What to do**:
  - **Prerequisite: Inventory existing wipe infrastructure.** Before writing tests:
    1. Read `bridge/pkg/keystore/key_derivation.go` — search for `wipe`, `zeroize`, `Clear`, `fill(0)` patterns
    2. Read `bridge/pkg/keystore/sealed_keystore.go` — search for same patterns in password handling paths
    3. If wipe helpers exist → write tests that verify they are called at every secret lifecycle end
    4. If wipe helpers are MISSING → write **code-audit tests** that verify `wipe()` calls are present in the source code (grep-based verification), NOT runtime memory-zeroing checks (Go GC makes byte-zeroing unreliable — the GC can copy objects, and string-to-[]byte conversions create copies)
    5. If wipe helpers are missing AND code-audit approach is insufficient → escalate to user before adding wipe helpers (guardrail says "Do NOT add test-only wipe helpers")
  - Verify NO secret material in:
    1. Password buffers after unseal (success and failure paths)
    2. `wrap_key` after vault operations
    3. `vault_key` derivation intermediate values
    4. Nonces used in XChaCha20-Poly1305 operations
    5. Session tokens after seal/expiry
  - Audit all `log.*` and `audit.*` calls to ensure no secret material in output

  **Go Memory Model Acknowledgment**: Go's garbage collector can copy objects at any time, making runtime memory-zeroing unreliable. The verification strategy adapts:
  - **Primary**: Code-audit verification (grep for `wipe()` calls at secret lifecycle endpoints)
  - **Secondary**: Runtime verification where feasible (byte slices that remain in scope)
  - **NOT attempted**: Verification that all memory copies are zeroed (impossible in Go)

  **Must NOT do**:
  - Do NOT add test-only wipe helpers without explicit user approval
  - Do NOT weaken security for testability
  - Do NOT attempt to verify Go GC has zeroed all copies (impossible)

  **Recommended Agent Profile**:
  - **Category**: `deep`
    - Reason: Security-critical verification requiring careful analysis of data flow
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 3
  - **Blocks**: T14
  - **Blocked By**: T5 (passphrase hardening complete)

  **References**:

  **Pattern References**:
  - `bridge/pkg/keystore/key_derivation.go` — Where `wipe()` should be called on intermediate buffers
  - `bridge/pkg/keystore/sealed_keystore.go` — Password handling in unseal/seal paths

  **API/Type References**:
  - `bridge/pkg/keystore/sealed_keystore.go:232` — `UnsealWithPassword()` — password buffer lifecycle
  - `bridge/pkg/keystore/sealed_keystore.go:337` — `VerifyChallengeAndUnseal()` — challenge buffer lifecycle
  - `bridge/pkg/keystore/key_derivation.go` — Key derivation with Argon2id — intermediate key buffers

  **WHY Each Reference Matters**:
  - `key_derivation.go`: The wipe helper lives here — need to verify it's called everywhere
  - `sealed_keystore.go:232`: Password enters here — must verify it's wiped after use

  **Acceptance Criteria**:

  **QA Scenarios:**

  ```
  Scenario: Password buffer zeroed after successful unseal
    Tool: Bash
    Steps:
      1. go test -v -run TestWipePasswordAfterUnseal ./pkg/keystore/...
      2. Unseal with known password, inspect buffer is zeroed
    Expected Result: All password bytes are 0x00
    Failure Indicators: Any non-zero byte in password buffer
    Evidence: .sisyphus/evidence/task-11-wipe-password.txt

  Scenario: No secret material in log output
    Tool: Bash
    Steps:
      1. go test -v -run TestNoSecretsInLogs ./pkg/keystore/...
      2. Perform unseal/seal/list operations, capture log output
      3. Search for password, key material, session tokens in logs
    Expected Result: Zero matches for secret patterns
    Failure Indicators: Any secret value appears in log output
    Evidence: .sisyphus/evidence/task-11-no-secrets-logs.txt
  ```

  **Commit**: YES
  - Message: `security(keystore): flow-level wipe verification tests`
  - Files: `bridge/pkg/keystore/wipe_test.go`
  - Pre-commit: `cd bridge && go test -v -run TestWipe ./pkg/keystore/...`

- [x] 12. Voice End-to-End + Rate-Limit Tests (Phase S6)

  **What to do**:
  - Write end-to-end tests for voice provider pipeline in a NEW file `bridge/pkg/voice/e2e_providers_test.go`
    (DO NOT modify existing `e2e_test.go` which tests HTTP sidecar services under `ARMORCLAW_E2E=1` gate)
  - Tests cover:
    1. `voice.start_session` → returns session ID
    2. `voice.status` → returns session state
    3. `voice.stop_session` → terminates cleanly
  - Test audio path: PCM input → VAD → STT → text → agent (text) → TTS → PCM output (may be mocked)
  - Test error scenarios:
    1. OpenAI API returns 429 → `-32008` error
    2. OpenAI API returns 401 → clear auth error
    3. OpenAI API returns 500 → retry/fallback
    4. Start session when already started → error
    5. Stop session when not started → error
  - Test flag-off behavior: all methods return `-32601`

  **Must NOT do**:
  - Do NOT require real OpenAI API key for tests (mock external calls)
  - Do NOT modify existing `bridge/pkg/voice/e2e_test.go` (ARMORCLAW_E2E=1 sidecar tests)
  - Do NOT overwrite or conflict with existing test infrastructure

  **Recommended Agent Profile**:
  - **Category**: `unspecified-high`
    - Reason: Integration testing across multiple components
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 3
  - **Blocks**: T14
  - **Blocked By**: T6, T7, T8 (all voice implementations complete)

  **References**:

  **Pattern References**:
  - `bridge/pkg/rpc/keystore_handlers_test.go` — Pattern for feature-flagged handler tests

  **API/Type References**:
  - `bridge/pkg/rpc/server.go:1197-1199` — Voice handler registrations
  - `bridge/pkg/voice/` — All voice implementations from T6-T8

  **Acceptance Criteria**:

  **QA Scenarios:**

  ```
  Scenario: Voice session lifecycle works end-to-end
    Tool: Bash
    Steps:
      1. go test -v -run TestVoiceSessionLifecycle ./pkg/rpc/...
      2. start_session → status → stop_session
    Expected Result: Full lifecycle completes without error
    Failure Indicators: Any method returns error in happy path
    Evidence: .sisyphus/evidence/task-12-voice-lifecycle.txt

  Scenario: Rate limit returns -32008
    Tool: Bash
    Steps:
      1. go test -v -run TestVoiceRateLimit ./pkg/rpc/...
      2. Simulate 429 from provider
    Expected Result: Application error code -32008
    Evidence: .sisyphus/evidence/task-12-voice-ratelimit.txt
  ```

  **Commit**: YES
  - Message: `test(voice): end-to-end provider tests and rate-limit handling`
  - Files: `bridge/pkg/voice/e2e_providers_test.go`
  - Pre-commit: `cd bridge && go test -v -run TestVoice ./pkg/voice/... ./pkg/rpc/...`

- [x] 13. Invalid-State + Edge-Case Contract Tests (Phase S6)

  **What to do**:
  - Write edge-case tests for all subsystems:
    1. Keystore: unseal when already unsealed, seal when already sealed, extend expired session
    2. Voice: start when already started, stop when stopped, concurrent sessions
    3. E2EE: create backup when one exists, delete nonexistent backup
    4. Replay: replay with invalid tab_id, replay when no tabs exist
    5. General: nil params, empty params, oversized params, malformed JSON
  - Verify all return appropriate application errors (not panics)

  **Must NOT do**:
  - Do NOT introduce new error types — use existing ones

  **Recommended Agent Profile**:
  - **Category**: `unspecified-high`
    - Reason: Systematic edge-case testing across multiple subsystems
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 3
  - **Blocks**: T14
  - **Blocked By**: T1-T8

  **References**:

  **Pattern References**:
  - `bridge/pkg/rpc/server_test.go` — Existing error handling tests
  - `bridge/pkg/rpc/keystore_handlers_test.go` — Existing handler test patterns

  **API/Type References**:
  - `bridge/pkg/errors/codes.go` — All error codes

  **Acceptance Criteria**:

  **QA Scenarios:**

  ```
  Scenario: All edge cases handled gracefully
    Tool: Bash
    Steps:
      1. go test -v -run TestEdgeCases ./pkg/rpc/...
      2. Run 15+ edge-case scenarios
    Expected Result: All return structured errors, zero panics
    Failure Indicators: Any panic or unstructured error
    Evidence: .sisyphus/evidence/task-13-edge-cases.txt
  ```

  **Commit**: YES
  - Message: `test(rpc): invalid-state and edge-case contract tests`
  - Files: `bridge/pkg/rpc/edge_case_test.go`
  - Pre-commit: `cd bridge && go test -v -run TestEdge ./pkg/rpc/...`

- [x] 14. Update architecture.md + bridge-reference.md (Phase S7)

  **What to do**:
  - Update `doc/architecture.md`:
    1. Document SealedKS wiring pattern (feature-gated construction in main.go)
    2. Document KeystoreLimiter purpose and configuration
    3. Document voice provider architecture (STT/TTS/VAD/PCM flow)
    4. Update feature-flag section with all 4 flags and their combinations
    5. Document that replay is handler-gated (not count-expanding)
    6. Document challenge-response is deferred to v1.1 (password-only for v1.0)
  - Update `doc/bridge-reference.md`:
    1. Add keystore methods documentation (all 7 with params/returns)
    2. Add voice methods documentation (all 3 with params/returns)
    3. Verify E2EE methods documentation is accurate
    4. Update method count to 108 max
    5. Add feature-flag combination matrix

  **Must NOT do**:
  - Do NOT create new doc files — update existing ones
  - Do NOT add AI slop (excessive sections, filler content)

  **Recommended Agent Profile**:
  - **Category**: `writing`
    - Reason: Documentation update with technical accuracy requirements
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: NO
  - **Parallel Group**: Wave 4 (sequential after all tests)
  - **Blocks**: F1-F4
  - **Blocked By**: T9-T13 (all tests must pass before documenting)

  **References**:

  **Pattern References**:
  - `doc/architecture.md` — Current 1067-line doc
  - `doc/bridge-reference.md` — Current 713-line doc

  **Acceptance Criteria**:

  **QA Scenarios:**

  ```
  Scenario: Architecture doc reflects v1.0.0 state
    Tool: Bash
    Steps:
      1. grep "108" doc/architecture.md — verify method count documented
      2. grep "sealed" doc/architecture.md — verify keystore wiring documented
      3. grep "handler-gated" doc/architecture.md — verify replay note
    Expected Result: All 3 patterns found
    Evidence: .sisyphus/evidence/task-14-arch-doc.txt

  Scenario: Bridge reference has all 108 methods
    Tool: Bash
    Steps:
      1. Count method entries in doc/bridge-reference.md
      2. Cross-reference against server.go registerHandlers()
    Expected Result: All 108 methods documented
    Evidence: .sisyphus/evidence/task-14-bridge-ref.txt
  ```

  **Commit**: YES
  - Message: `docs: update architecture.md and bridge-reference.md for v1.0.0`
  - Files: `doc/architecture.md`, `doc/bridge-reference.md`
  - Pre-commit: `grep -c "method" doc/bridge-reference.md`

---

## Final Verification Wave

- [x] F1. **Plan Compliance Audit** — `oracle`
  Read the plan end-to-end. For each "Must Have": verify implementation exists (read file, run command). For each "Must NOT Have": search codebase for forbidden patterns — reject with file:line if found. Check evidence files exist in .sisyphus/evidence/. Compare deliverables against plan.
  Output: `Must Have [N/N] | Must NOT Have [N/N] | Tasks [N/N] | VERDICT: APPROVE/REJECT`

- [x] F2. **Code Quality Review** — `unspecified-high`
  Run `go vet ./bridge/...` + `go test -v ./bridge/...`. Review all changed files for: `as any`/`@ts-ignore` equivalents (interface{} abuse, unchecked type assertions), empty catches, fmt.Println in prod, commented-out code, unused imports. Check AI slop: excessive comments, over-abstraction, generic names.
  Output: `Build [PASS/FAIL] | Vet [PASS/FAIL] | Tests [N pass/N fail] | Files [N clean/N issues] | VERDICT`

- [x] F3. **Comprehensive Scenario QA** — `unspecified-high`
  Build the bridge binary. Start it with a test config (all flags on). Execute EVERY QA scenario from EVERY task — follow exact steps, capture evidence. Test cross-task integration. Save to `.sisyphus/evidence/final-qa/`.
  Output: `Scenarios [N/N pass] | Integration [N/N] | Edge Cases [N tested] | VERDICT`

- [x] F4. **Scope Fidelity Check** — `deep`
  For each task: read "What to do", read actual diff (git log/diff). Verify 1:1 — everything in spec was built (no missing), nothing beyond spec was built (no creep). Check "Must NOT do" compliance. Detect cross-task contamination.
  Output: `Tasks [N/N compliant] | Contamination [CLEAN/N issues] | Unaccounted [CLEAN/N files] | VERDICT`

---

## Commit Strategy

- **T1**: `feat(keystore): wire SealedKS and KeystoreLimiter in main.go` — bridge/cmd/bridge/main.go, `go test ./bridge/pkg/rpc/... ./bridge/pkg/keystore/...`
- **T2**: `test(e2ee): validate backup store wiring and init-failure path` — bridge/pkg/crypto/backup_store_test.go
- **T3**: `test(replay): validate multi-tab replay gating behavior` — bridge/pkg/rpc/replay_gating_test.go
- **T4**: `refactor(voice): resolve interface mismatch between pkg/interfaces and pkg/voice` — bridge/pkg/voice/
- **T5**: `security(keystore): verify Argon2id params, passphrase policy, lockout behavior` — bridge/pkg/keystore/
- **T6**: `feat(voice): implement OpenAI Whisper STT backend` — bridge/pkg/voice/stt_openai.go
- **T7**: `feat(voice): implement OpenAI TTS backend` — bridge/pkg/voice/tts_openai.go
- **T8**: `feat(voice): implement energy-threshold VAD and PCM routing` — bridge/pkg/voice/vad.go, bridge/pkg/voice/pcm.go
- **T9**: `test(rpc): feature-flag discovery validation for all combinations` — bridge/pkg/rpc/discovery_test.go
- **T10**: `test(keystore): handler contract tests for all 7 methods` — bridge/pkg/rpc/keystore_contract_test.go
- **T11**: `security(keystore): flow-level wipe verification tests` — bridge/pkg/keystore/wipe_test.go
- **T12**: `test(voice): end-to-end and rate-limit tests` — bridge/pkg/voice/e2e_test.go
- **T13**: `test(rpc): invalid-state and edge-case contract tests` — bridge/pkg/rpc/edge_case_test.go
- **T14**: `docs: update architecture.md and bridge-reference.md for v1.0.0` — doc/architecture.md, doc/bridge-reference.md

---

## Success Criteria

### Verification Commands
```bash
cd bridge && go test -v ./...                                    # All tests pass
go vet ./...                                                      # No vet issues
go build -o /tmp/armorclaw-bridge ./cmd/bridge                    # Binary builds
echo '{"jsonrpc":"2.0","method":"status","id":1}' | socat - UNIX-CONNECT:/run/armorclaw/bridge.sock  # Bridge responds
```

### Final Checklist
- [ ] All "Must Have" present
- [ ] All "Must NOT Have" absent
- [ ] All tests pass (`go test ./bridge/...`)
- [ ] Method counts correct: 95/102/105/108 for each flag combination
- [ ] No secret material in logs or audit payloads
- [ ] Restart always begins sealed
