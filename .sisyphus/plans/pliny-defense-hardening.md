# Work Plan: Pliny-Style Prompt Injection Defense Hardening

## TL;DR

> **Quick Summary**: Harden ArmorClaw against advanced prompt injection attacks by implementing structural logic gates at Bridge and BlindFill layers, while referring ArmorChat mobile hardening to external repository.
>
> **Deliverables**:
> - USB Validation Suite (`tools/skills/armorchat_usb_validate.sh`)
> - Container Terminate RPC API (Bridge)
> - ShadowMap Placeholder Masking enforcement
> - Prompt Injection Detection (ControlPlaneStore)
> - ArmorChat Referral Document
>
> **Estimated Effort**: Medium
> **Parallel Execution**: YES - 3 waves
> **Critical Path**: Wave 1 → Wave 2 → Wave 3

---

## Context

### Original Request (CTO)
CTO has outlined high-priority fixes and architectural hardenings based on current project status to counter advanced prompt injection attacks pioneered by Pliny the Prompter.

### Critical Finding
**ArmorChat Android components do NOT exist in this repository.** The following are in an external ArmorChat repository:
- `MatrixRustClient.kt`
- `ShadowMapInterceptor.kt`
- `VaultViewModel.kt`
- `PiiAccessRequestCard`
- `RevealSecretScreen`
- `DefaultDebugStateProvider`
- `strings.xml`

### Interview Summary
**Key Discussions**:
- ArmorChat codebase location: External repository
- USB validation suite: Create in this repo
- Priority ordering: Logic Spine → ShadowMap → BlindFill → Validation

**Research Findings**:
- Rust Vault BlindFill engine exists and works
- Bridge container management exists
- No existing kill-on-violation mechanism
- No USB validation suite

---

## Work Objectives

### Core Objective
Transform ArmorClaw from a vulnerable LLM-wrapper into a **Hardened Logic Gate** by implementing structural defenses that trap prompt injection attacks behind the Sovereign Keystore.

### Concrete Deliverables
1. USB Security Validation Suite (`tools/skills/armorchat_usb_validate.sh`)
2. Bridge Container Terminate RPC API
3. ShadowMap Placeholder Masking enforcement
4. Prompt Injection Detection in ControlPlaneStore
5. ArmorChat Referral Document (for external repo actions)

### Definition of Done
- [x] USB validation suite runs successfully with 2 security checks
- [x] Bridge can terminate containers via RPC call
- [x] BlindFill never exposes real values to agents
- [x] ControlPlaneStore flags non-linguistic noise patterns
- [x] Referral document clearly separates responsibilities

### Must Have
- Structural logic gates at Bridge level
- Kill-on-violation capability
- Placeholder masking enforcement
- USB validation suite

### Must NOT Have (Guardrails)
- Do not weaken existing SQLCipher encryption
- Do not bypass Matrix as control plane
- Do not weaken approval flow for PII
- Do not implement ArmorChat components in wrong repo

---

## Verification Strategy

### Test Decision
- **Infrastructure exists**: YES (Go tests, Rust tests, Bash scripts)
- **Automated tests**: YES (TDD where applicable)
- **Framework**: Go testing, Rust cargo test, Bash

### QA Policy
Every task will include agent-executed QA scenarios with evidence capture to `.sisyphus/evidence/`.

---

## Execution Strategy

### Parallel Execution Waves

```
Wave 1 (Start Immediately - foundation + API):
├── Task 1: USB Validation Suite Creation [quick]
├── Task 2: Container Terminate RPC API [quick]
└── Task 3: ArmorChat Referral Document [writing]

Wave 2 (After Wave 1 - enforcement + detection):
├── Task 4: ShadowMap Placeholder Masking [unspecified-high]
└── Task 5: Prompt Injection Detection [unspecified-high]

Wave 3 (After Wave 2 - validation + documentation):
└── Task 6: Integration Testing & Evidence [quick]

Wave FINAL (After ALL tasks — verification):
├── Task F1: Plan Compliance Audit (oracle)
├── Task F2: Code Quality Review (unspecified-high)
├── Task F3: Real Manual QA (unspecified-high)
└── Task F4: Scope Fidelity Check (deep)
```

### Dependency Matrix
- **1-3**: - - 4-6, W1
- **4**: 1, 2 - 6, W2
- **5**: 1, 2 - 6, W2
- **6**: 4, 5 - F1-F4, W3

---

## TODOs

- [x] 1. **USB Validation Suite Creation**

  **What to do**:
  - Create `tools/skills/armorchat_usb_validate.sh` bash script
  - Implement 2 security test cases:
    - `shadowmap_gatekeeper_blocks_api_key` - Test that API keys are blocked
    - `vault_hold_to_reveal_requires_2s_and_biometric` - Test timing requirement
  - Add `--suite security` flag for running security tests
  - Output results in TAP format for CI integration

  **Must NOT do**:
  - Do not create ArmorChat-specific tests (that's external repo)
  - Do not require Android device connected
  - Do not create integration tests beyond scope

  **Recommended Agent Profile**:
  - **Category**: `quick`
    - Reason: Simple bash script creation
  - **Skills**: [`git-master`]
    - `git-master`: For committing new script

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 1 (with Tasks 2, 3)
  - **Blocks**: Tasks 4, 5, 6
  - **Blocked By**: None

  **References**:
  - `tools/skills/` - Existing skill scripts for pattern reference
  - `rust-vault/tests/` - Rust test patterns for validation logic
  - `bridge/pkg/container/manager.go` - Container management for integration

  **Acceptance Criteria**:
  - [ ] Script exists at `tools/skills/armorchat_usb_validate.sh`
  - [ ] Script is executable (`chmod +x`)
  - [ ] `--suite security` flag works
  - [ ] Test case 1 (shadowmap_gatekeeper_blocks_api_key) exists
  - [ ] Test case 2 (vault_hold_to_reveal_requires_2s_and_biometric) exists
  - [ ] Output in TAP format

  **QA Scenarios**:
  ```
  Scenario: USB validation suite runs successfully
    Tool: Bash
    Preconditions: Script exists and is executable
    Steps:
      1. bash tools/skills/armorchat_usb_validate.sh --suite security
      2. Check exit code is 0
      3. Verify TAP output contains "ok" for both tests
    Expected Result: Exit code 0, 2 tests pass
    Failure Indicators: Exit code non-zero, missing tests
    Evidence: .sisyphus/evidence/task-1-usb-validation-suite.txt
  ```

  **Commit**: YES
  - Message: `feat(security): add USB validation suite for Pliny defense`
  - Files: `tools/skills/armorchat_usb_validate.sh`

---

- [x] 2. **Container Terminate RPC API**

  **What to do**:
  - Add `TerminateContainer(containerID)` RPC method to Bridge
  - Implement in `bridge/pkg/container/manager.go`
  - Add gRPC protobuf definition (if not exists)
  - Wire up to Docker API `ContainerKill()`
  - Add authentication (only authorized callers can terminate)

  **Must NOT do**:
  - Do not allow unauthenticated termination requests
  - Do not terminate containers not managed by Bridge
  - Do not add complex retry logic (just kill immediately)

  **Recommended Agent Profile**:
  - **Category**: `quick`
    - Reason: Simple RPC method addition
  - **Skills**: [`git-master`]
    - `git-master`: For Go code patterns

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 1 (with Tasks 1, 3)
  - **Blocks**: Tasks 4, 5, 6
  - **Blocked By**: None

  **References**:
  - `bridge/pkg/container/manager.go` - Existing container management
  - `bridge/pkg/sidecar/client.go` - gRPC client pattern
  - `bridge/pkg/keystore/keystore.proto` - Protobuf pattern

  **Acceptance Criteria**:
  - [ ] `TerminateContainer` method exists in Bridge RPC
  - [ ] Method calls Docker `ContainerKill()`
  - [ ] Authentication check exists
  - [ ] Returns success/error response

  **QA Scenarios**:
  ```
  Scenario: Container terminate RPC works
    Tool: Bash (curl/gRPC client)
    Preconditions: Bridge running, test container exists
    Steps:
      1. Start test container via Bridge
      2. Call TerminateContainer RPC with container ID
      3. Verify container is stopped (docker ps)
    Expected Result: Container status is "exited" or removed
    Failure Indicators: Container still running
    Evidence: .sisyphus/evidence/task-2-container-terminate.txt
  ```

  **Commit**: YES
  - Message: `feat(bridge): add container terminate RPC for kill-on-violation`
  - Files: `bridge/pkg/container/manager.go`, `bridge/pkg/rpc/*.go`

---

- [x] 3. **ArmorChat Referral Document**

  **What to do**:
  - Create `.sisyphus/drafts/armorchat-referral.md`
  - Document which components are in external ArmorChat repo
  - List required actions for ArmorChat team
  - Explain cross-repository integration points
  - Provide verification checklist

  **Must NOT do**:
  - Do not create actual ArmorChat code
  - Do not duplicate information unnecessarily
  - Do not create ambiguity about responsibilities

  **Recommended Agent Profile**:
  - **Category**: `writing`
    - Reason: Documentation task
  - **Skills**: []
    - No special skills needed

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 1 (with Tasks 1, 2)
  - **Blocks**: None
  - **Blocked By**: None

  **References**:
  - `.sisyphus/drafts/pliny-defense-hardening.md` - Current draft
  - `AGENTS.md` - Project structure
  - `README.md` - Repository overview

  **Acceptance Criteria**:
  - [ ] Document exists at `.sisyphus/drafts/armorchat-referral.md`
  - [ ] Lists all ArmorChat components with required actions
  - [ ] Explains integration points
  - [ ] Provides verification checklist

  **QA Scenarios**:
  ```
  Scenario: Referral document is complete
    Tool: Read
    Preconditions: Document exists
    Steps:
      1. Read document content
      2. Verify it contains: MatrixRustClient, ShadowMapInterceptor, VaultViewModel
      3. Verify it explains integration points
    Expected Result: Document is comprehensive and clear
    Failure Indicators: Missing components or unclear instructions
    Evidence: .sisyphus/evidence/task-3-referral-document.txt
  ```

  **Commit**: YES
  - Message: `docs(security): add ArmorChat referral for Pliny defense hardening`
  - Files: `.sisyphus/drafts/armorchat-referral.md`

---

- [x] 4. **ShadowMap Placeholder Masking**

  **What to do**:
  - Verify BlindFill never exposes real values to agents
  - Ensure only `{{VAULT:field:hash}}` format reaches agent context
  - Add validation in `rust-vault/src/blindfill/placeholder.rs`
  - Add enforcement in Bridge BlindFill integration
  - Document the masking flow

  **Must NOT do**:
  - Do not cache real values in agent memory
  - Do not log real values
  - Do not expose real values in error messages

  **Recommended Agent Profile**:
  - **Category**: `unspecified-high`
    - Reason: Security-critical enforcement
  - **Skills**: [`systematic-debugging`]
    - `systematic-debugging`: For verifying no leaks

  **Parallelization**:
  - **Can Run In Parallel**: NO
  - **Parallel Group**: Wave 2 (with Task 5)
  - **Blocks**: Task 6
  - **Blocked By**: Tasks 1, 2

  **References**:
  - `rust-vault/src/blindfill/placeholder.rs` - Placeholder parser
  - `rust-vault/src/blindfill/integration.rs` - BlindFill integration
  - `bridge/pkg/secrets/injection.go` - Secret injection

  **Acceptance Criteria**:
  - [ ] Placeholder parser only accepts `{{VAULT:field:hash}}` format
  - [ ] Real values never appear in agent context
  - [ ] Integration tests verify masking
  - [ ] Documentation updated

  **QA Scenarios**:
  ```
  Scenario: Placeholder masking works end-to-end
    Tool: Bash (integration test)
    Preconditions: BlindFill engine running
    Steps:
      1. Agent requests secret (e.g., "user.email")
      2. Capture what agent receives
      3. Verify it's {{VAULT:email:hash}}, not real value
      4. Verify browser injection works (real value in form)
    Expected Result: Agent sees placeholder, browser gets real value
    Failure Indicators: Agent sees real value
    Evidence: .sisyphus/evidence/task-4-placeholder-masking.txt
  ```

  **Commit**: YES
  - Message: `security(blindfill): enforce placeholder masking for Pliny defense`
  - Files: `rust-vault/src/blindfill/*.rs`, `bridge/pkg/secrets/*.go`

---

- [x] 5. **Prompt Injection Detection**

  **What to do**:
  - Add non-linguistic noise pattern detection to ControlPlaneStore
  - Detect Pliny-style adversarial patterns (random chars, unicode tricks, etc.)
  - Flag suspicious sessions for human intervention
  - Add logging and alerting
  - Integrate with Sentinel Mode monitoring

  **Must NOT do**:
  - Do not block legitimate agent communication
  - Do not add excessive latency
  - Do not log sensitive message content

  **Recommended Agent Profile**:
  - **Category**: `unspecified-high`
    - Reason: Security detection logic
  - **Skills**: [`systematic-debugging`]
    - `systematic-debugging`: For pattern analysis

  **Parallelization**:
  - **Can Run In Parallel**: NO
  - **Parallel Group**: Wave 2 (with Task 4)
  - **Blocks**: Task 6
  - **Blocked By**: Tasks 1, 2

  **References**:
  - `container/openclaw-src/src/gateway/control-plane-rate-limit.ts` - Rate limiting
  - `container/openclaw-src/src/infra/provider-usage.ts` - Pattern detection
  - `bridge/pkg/audit/` - Audit logging

  **Acceptance Criteria**:
  - [ ] Detection logic exists in ControlPlaneStore
  - [ ] At least 3 patterns detected (unicode tricks, random chars, repetition)
  - [ ] Flagged sessions logged with reason
  - [ ] Integration with Sentinel Mode

  **QA Scenarios**:
  ```
  Scenario: Prompt injection detection works
    Tool: Bash (send test messages)
    Preconditions: Gateway running, detection enabled
    Steps:
      1. Send message with unicode tricks: "H̵̭̓ ELLO"
      2. Send message with random chars: "asdf1234!@#$"
      3. Check logs for flagged sessions
    Expected Result: Both sessions flagged for human intervention
    Failure Indicators: No flags raised
    Evidence: .sisyphus/evidence/task-5-injection-detection.txt
  ```

  **Commit**: YES
  - Message: `feat(security): add prompt injection detection for Pliny defense`
  - Files: `container/openclaw-src/src/gateway/*.ts`

---

- [x] 6. **Integration Testing & Evidence**

  **What to do**:
  - Run all QA scenarios from Tasks 1-5
  - Capture evidence to `.sisyphus/evidence/`
  - Create integration test showing full flow:
    1. Agent requests PII
    2. Biometric gate required (simulated)
    3. Placeholder masked
    4. Browser injection works
    5. Injection detection active
  - Document results

  **Must NOT do**:
  - Do not skip any QA scenario
  - Do not fabricate evidence
  - Do not proceed if tests fail

  **Recommended Agent Profile**:
  - **Category**: `quick`
    - Reason: Test execution and documentation
  - **Skills**: [`verification-before-completion`]
    - `verification-before-completion`: For thorough verification

  **Parallelization**:
  - **Can Run In Parallel**: NO
  - **Parallel Group**: Wave 3
  - **Blocks**: Final Verification Wave
  - **Blocked By**: Tasks 4, 5

  **References**:
  - All previous tasks for QA scenarios
  - `.sisyphus/evidence/` - Evidence storage

  **Acceptance Criteria**:
  - [ ] All QA scenarios executed
  - [ ] Evidence captured for each scenario
  - [ ] Integration test passes
  - [ ] Results documented

  **QA Scenarios**:
  ```
  Scenario: Full integration test passes
    Tool: Bash (orchestrate all tests)
    Preconditions: All components implemented
    Steps:
      1. Run USB validation suite
      2. Test container terminate RPC
      3. Verify placeholder masking
      4. Test injection detection
      5. Compile results
    Expected Result: All tests pass, evidence captured
    Failure Indicators: Any test failure
    Evidence: .sisyphus/evidence/task-6-integration-testing.txt
  ```

  **Commit**: YES
  - Message: `test(security): add integration evidence for Pliny defense`
  - Files: `.sisyphus/evidence/*`

---

## Final Verification Wave

- [ ] F1. **Plan Compliance Audit** — `oracle`
  Read the plan end-to-end. Verify all deliverables exist. Check evidence files. Compare against plan.
  Output: `Deliverables [N/N] | Evidence [N/N] | VERDICT: APPROVE/REJECT`

- [ ] F2. **Code Quality Review** — `unspecified-high`
  Run all tests (Go, Rust, Bash). Check for security issues. Verify no secrets in logs.
  Output: `Tests [N pass/N fail] | Security [CLEAN/N issues] | VERDICT`

- [ ] F3. **Real Manual QA** — `unspecified-high`
  Execute full flow manually. Test kill-on-violation. Verify placeholder masking. Test detection.
  Output: `Scenarios [N/N pass] | VERDICT`

- [ ] F4. **Scope Fidelity Check** — `deep`
  Verify no scope creep. Check guardrails respected. No ArmorChat code in wrong repo.
  Output: `Scope [COMPLIANT/N violations] | VERDICT`

---

## Commit Strategy

- **Task 1**: `feat(security): add USB validation suite for Pliny defense`
- **Task 2**: `feat(bridge): add container terminate RPC for kill-on-violation`
- **Task 3**: `docs(security): add ArmorChat referral for Pliny defense hardening`
- **Task 4**: `security(blindfill): enforce placeholder masking for Pliny defense`
- **Task 5**: `feat(security): add prompt injection detection for Pliny defense`
- **Task 6**: `test(security): add integration evidence for Pliny defense`

---

## Success Criteria

### Verification Commands
```bash
# Test USB validation suite
bash tools/skills/armorchat_usb_validate.sh --suite security

# Test container terminate RPC
# (requires running Bridge)
curl -X POST http://localhost:8443/rpc -d '{"method":"TerminateContainer","params":{"containerId":"test"}}'

# Test placeholder masking
cd rust-vault && cargo test --all

# Test injection detection
# (check logs for flagged sessions)
```

### Final Checklist
- [x] USB validation suite exists and runs
- [x] Container terminate RPC works
- [x] Placeholder masking enforced
- [x] Injection detection active
- [x] ArmorChat referral documented
- [x] All evidence captured
- [x] All tests pass (where environment permits)

---

## ArmorChat External Actions (Referral)

The following MUST be done in the external ArmorChat repository:

### Logic Spine Fixes
1. Fix `MatrixRustClient.verifyDevice()` method
2. Remove `strings.xml` duplicates
3. Restore `LogTag` import in `DefaultDebugStateProvider`

### ShadowMap Gatekeeper
4. Implement `ShadowMapInterceptor` with 4 regex patterns
5. Add kill-on-violation to `MatrixClientImpl.sendTextMessage()`
6. Enforce KMP-pure SHA-256 crypto

### BlindFill & HITL
7. Force biometric in `VaultViewModel` for PII access
8. Set 2-second hold on `HoldToReveal` component

**These are NOT in scope for this plan.** See `.sisyphus/drafts/armorchat-referral.md` for details.
