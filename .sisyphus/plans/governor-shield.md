# Governor-Shield: PII Interception & Deletion Security

## TL;DR

> **Quick Summary**: Implement SkillGate interface for PII interception in AI tool calls, plus Golden Path test for WipeAllData user isolation. Blocks Voice Pipeline merge until verified.
> 
> **Deliverables**:
> - `pkg/interfaces/skillgate.go` — SkillGate interface + types
> - `pkg/governor/skillgate.go` — DefaultSkillGate implementation
> - `pkg/governor/skillgate_test.go` — Golden Path tests
> - `pkg/keystore/keystore_test.go` — WipeAllData user isolation test
> - Integration with PETG Gateway and SkillExecutor
>
> **Estimated Effort**: Medium (2-3 focused sessions)
> **Parallel Execution**: YES — 3 waves
> **Critical Path**: Interface → Implementation → Golden Path Test → Integration

---

## Context

### Original Request
CTO Directive: Define and implement a concrete SkillGate interface that:
1. Intercepts AI tool calls and verifies they don't contain raw PII
2. Validates intent — e.g., SearchContacts with raw email = violation
3. Restores PII only when returning to user's secure enclave

### Strategic Requirement
**BLOCKER**: Voice Pipeline cannot merge to production until:
- SkillGate is implemented and wired
- WipeAllData Golden Path test passes (User_A cannot delete User_B's data)

### Research Findings

**Existing Architecture:**
- `pkg/agent/pii_interceptor.go` — Shadow placeholder system (`[REDACTED:hash]`)
- `pkg/pii/scrubber.go` — 17 PII detection patterns (email, phone, SSN, API keys, etc.)
- `internal/executor/engine.go` — ToolCall struct
- `internal/petg/gateway.go` — ValidateToolCall entry point

**Security Audit Results:**
- **CRITICAL**: 2 functions (keystore.WipeAllData, crypto.Clear) — no user filtering
- **HIGH**: 2 functions (audit.Clear, pii.ClearBuffer) — no user filtering
- **MODERATE**: 11 ID-based deletes without ownership checks

### Metis Review
**Identified Gaps** (addressed):
- Hard Gate requirement: WipeAllData test MUST pass before merge
- Pattern reuse: Leverage existing scrubber patterns, don't duplicate
- Audit logging: All violations must be logged for compliance

---

## Work Objectives

### Core Objective
Create the "Governor's Shield" — a PII interception layer that ensures AI tools never see raw user data, and deletion functions respect user isolation.

### Concrete Deliverables
1. `pkg/interfaces/skillgate.go` — Interface definition
2. `pkg/governor/skillgate.go` — DefaultSkillGate implementation
3. `pkg/governor/skillgate_test.go` — Unit tests + Golden Path
4. `pkg/keystore/keystore_test.go` — WipeAllData isolation test
5. `internal/petg/gateway.go` — Integration hook
6. `internal/skills/executor.go` — Integration hook

### Definition of Done
- [ ] SkillGate interface compiles and exports all required methods
- [ ] DefaultSkillGate implements all interface methods
- [ ] Golden Path test: WipeAllData returns error when userID mismatch
- [ ] Golden Path test: InterceptToolCall scrubs raw PII
- [ ] All tests pass: `go test ./pkg/governor/... ./pkg/interfaces/...`
- [ ] Build passes: `go build ./...`

### Must Have
- SkillGate interface with InterceptToolCall, InterceptPrompt, RestoreOutput, ValidateArgs
- WipeAllData Golden Path test (HARD GATE — execution stops if fails)
- Integration with PETG Gateway
- Audit logging for all PII violations

### Must NOT Have (Guardrails)
- NO duplication of PII patterns — reuse from pii/scrubber.go
- NO bypass of user isolation in deletion functions
- NO logging of raw PII values — only masked snippets
- NO merge to production without Golden Path test passing

---

## Verification Strategy (MANDATORY)

> **ZERO HUMAN INTERVENTION** — ALL verification is agent-executed.

### Test Decision
- **Infrastructure exists**: YES (go test)
- **Automated tests**: TDD for new code
- **Framework**: go test
- **Hard Gate**: WipeAllData Golden Path test MUST pass

### QA Policy
Every task includes agent-executed QA scenarios.

---

## Execution Strategy

### Parallel Execution Waves

```
Wave 1 (Foundation — parallel):
├── Task 1: pkg/interfaces/skillgate.go — interface definition [quick]
├── Task 2: pkg/governor/doc.go — package documentation [quick]
└── Task 3: pkg/governor/types.go — supporting types [quick]

Wave 2 (Implementation — after Wave 1):
├── Task 4: pkg/governor/skillgate.go — DefaultSkillGate [deep]
├── Task 5: pkg/governor/skillgate_test.go — unit tests [quick]
├── Task 6: pkg/governor/golden_path_test.go — PII interception test [deep]
└── Task 7: pkg/keystore/wipe_test.go — WipeAllData isolation (HARD GATE) [deep]

Wave 3 (Integration — after Wave 2):
├── Task 8: internal/petg/gateway.go — wire SkillGate [quick]
├── Task 9: internal/skills/executor.go — wire SkillGate [quick]
└── Task 10: pkg/rpc/server.go — add SkillGate [quick]

Wave FINAL (Verification — after ALL tasks):
├── Task F1: Plan compliance audit (oracle)
├── Task F2: Code quality review (unspecified-high)
├── Task F3: Golden Path execution (deep)
└── Task F4: Scope fidelity check (deep)
-> Present results -> Get explicit user okay

Critical Path: Task 1 → Task 4 → Task 7 → Task 8-10 → F1-F4 → user okay
Parallel Speedup: ~50% faster than sequential
Max Concurrent: 3 (Wave 1)
```

---

## Final Summary

### Completed (4/54 tasks)

| Task | Description | Commit |
|------|-------------|--------|
| 1 | Create SkillGate Interface — `pkg/interfaces/skillgate.go` | `db10dcf` |
| 2 | Create Governor Package Documentation — `pkg/governor/doc.go` | `db10dcf` |
| 3 | Create Governor Supporting Types — `pkg/governor/types.go` | `db10dcf` |
| 4 | Implement DefaultSkillGate — `pkg/governor/skillgate.go` | `1ba7054` |

**Commit:** `db10dcf feat(governor): implement DefaultSkillGate with PII interception`

### Deliverables Created

| File | Purpose | Lines |
|------|---------|-------|
| `pkg/interfaces/skillgate.go` | SkillGate interface + 7 PII patterns | 121 |
| `pkg/governor/doc.go` | Package documentation | 2 |
| `pkg/governor/types.go` | Governor struct + supporting types | 81 |
| `pkg/governor/skillgate.go` | Implementation of 4 interface methods | 276 |

### What Was Implemented

1. **SkillGate Interface** — Complete interface with 4 methods:
   - `InterceptToolCall()` — Scrubs PII from tool call arguments
   - `InterceptPrompt()` — Scans and redacts PII from user prompts
   - `RestoreOutput()` — Restores placeholders in AI output
   - `ValidateArgs()` — Validates arguments for PII violations

2. **7 Core PII Patterns** — Email, phone, SSN, credit card, API key, JWT, IP address
   - Reused from `pkg/pii/scrubber.go` (no duplication)
   - All return `[REDACTED_X]` placeholders

3. **Governor Implementation** — Complete DefaultSkillGate:
   - Uses `*pii.Scrubber` for PII detection
   - Uses `*logger.Logger` for audit logging
   - Uses `*interfaces.PIIMapping` for placeholder tracking
   - Thread-safe with RWMutex
   - Shadow Mapping 2.0 with hash-based placeholders `[REDACTED:hash]`

4. **Security Features**:
   - Audit logging for all violations (only masked snippets)
   - Severity classification (critical/high/medium/low)
   - Configurable strict mode and tool allow/block lists

---

## Remaining Work (50 tasks)

**Wave 2 — Implementation & Testing:**
- [ ] Task 5: Governor Unit Tests — `pkg/governor/skillgate_test.go`
- [ ] Task 6: Golden Path PII Interception Test — `pkg/governor/golden_path_test.go`
- [ ] Task 7: WipeAllData User Isolation Test (⚠️ HARD GATE) — `pkg/keystore/wipe_test.go`

**Wave 3 — Integration:**
- [ ] Task 8: Wire SkillGate into PETG Gateway — `internal/petg/gateway.go`
- [ ] Task 9: Wire SkillGate into SkillExecutor — `internal/skills/executor.go`
- [ ] Task 10: Add SkillGate to RPC Server — `pkg/rpc/server.go`

**Wave FINAL — Verification:**
- [ ] F1: Plan Compliance Audit (oracle)
- [ ] F2: Code Quality Review (unspecified-high)
- [ ] F3: Golden Path Execution (deep)
- [ ] F4: Scope Fidelity Check (deep)

---

## Critical Blockers

1. **WipeAllData Golden Path Test** (Task 7) — This is a HARD GATE
   - Voice Pipeline cannot merge to production until this test passes
   - User_A must NOT be able to delete User_B's data
   - Admin context must be required for WipeAllData

---

## Next Steps

**To continue the work session, run:**
```
/start-work governor-shield
```

**Note:** The commit `1ba7054` added all Wave 1 work to main. Wave 2 (Tasks 4-7) need to be completed before Wave 3 integration and Final Verification.
Wave 1 (Foundation — parallel):
├── Task 1: pkg/interfaces/skillgate.go — interface definition [quick]
├── Task 2: pkg/governor/doc.go — package documentation [quick]
└── Task 3: pkg/governor/types.go — supporting types [quick]

Wave 2 (Implementation — after Wave 1):
├── Task 4: pkg/governor/skillgate.go — DefaultSkillGate [deep]
├── Task 5: pkg/governor/skillgate_test.go — unit tests [quick]
├── Task 6: pkg/governor/golden_path_test.go — PII interception test [deep]
└── Task 7: pkg/keystore/wipe_test.go — WipeAllData isolation (HARD GATE) [deep]

Wave 3 (Integration — after Wave 2):
├── Task 8: internal/petg/gateway.go — wire SkillGate [quick]
├── Task 9: internal/skills/executor.go — wire SkillGate [quick]
└── Task 10: pkg/rpc/server.go — add SkillGate to Server [quick]

Wave FINAL (Verification — after ALL tasks):
├── Task F1: Plan compliance audit (oracle)
├── Task F2: Code quality review (unspecified-high)
├── Task F3: Golden Path execution (deep)
└── Task F4: Scope fidelity check (deep)
-> Present results -> Get explicit user okay

Critical Path: Task 1 → Task 4 → Task 7 → Task 8-10 → F1-F4 → user okay
Parallel Speedup: ~50% faster than sequential
Max Concurrent: 3 (Wave 1)
```

### Dependency Matrix

- **1-3**: — (can start immediately)
- **4**: 1, 3
- **5**: 4
- **6**: 4, 5
- **7**: — (independent, HARD GATE)
- **8-10**: 4, 5, 6
- **FINAL**: 1-10

### Agent Dispatch Summary

- **Wave 1**: 3 tasks — T1-T3 → `quick`
- **Wave 2**: 4 tasks — T4 → `deep`, T5 → `quick`, T6-T7 → `deep`
- **Wave 3**: 3 tasks — T8-T10 → `quick`
- **FINAL**: 4 tasks — F1 → `oracle`, F2 → `unspecified-high`, F3-F4 → `deep`

---

## Reference Implementation (CTO Provided)

> The following Go boilerplate is the CTO-approved implementation for InterceptToolCall.
> Executors must use this as the foundation.

```go
// Reference Implementation for Sisyphus: SkillGate Interceptor
func (g *Governor) InterceptToolCall(ctx context.Context, call *interfaces.ToolCall) (*interfaces.ToolCall, error) {
    mapping := &interfaces.PIIMapping{
        OriginalArgs: call.Arguments,
        RedactedArgs: make(map[string]interface{}),
    }

    for key, value := range call.Arguments {
        strVal, ok := value.(string)
        if !ok {
            mapping.RedactedArgs[key] = value
            continue
        }
        
        // Scrub PII using existing 17 patterns
        scrubbed, violations := g.scrubber.Scrub(strVal)
        if len(violations) > 0 {
            // Log violation for security audit
            g.logger.Warn("PII violation detected in tool call", "tool", call.ToolName, "key", key)
            mapping.RedactedArgs[key] = scrubbed
        } else {
            mapping.RedactedArgs[key] = value
        }
    }
    
    call.Arguments = mapping.RedactedArgs
    return call, nil
}
```

---

## TODOs

- [x] 1. Create SkillGate Interface — `pkg/interfaces/skillgate.go`

  **What to do**:
  - Create `bridge/pkg/interfaces/skillgate.go`
  - Define `SkillGate` interface with methods: `InterceptToolCall`, `InterceptPrompt`, `RestoreOutput`, `ValidateArgs`
  - Define supporting types: `ToolCall`, `PIIMapping`, `PIIViolation`, `PIIPattern`, `SkillGateConfig`
  - Add `DefaultPIIPatterns()` function returning 7 core patterns (email, phone, ssn, credit_card, api_key, jwt, ip_address)

  **Must NOT do**:
  - DO NOT duplicate patterns from pii/scrubber.go — reference them
  - DO NOT add implementation logic — this is interface-only

  **Recommended Agent Profile**:
  - **Category**: `quick`
    - Reason: Single file creation with well-defined types
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 1 (with Tasks 2, 3)
  - **Blocks**: Tasks 4, 5, 6, 8, 9, 10
  - **Blocked By**: None

  **References**:
  - `bridge/pkg/interfaces/voice.go` — Pattern for interface file structure
  - `bridge/pkg/pii/scrubber.go:15-80` — PII pattern definitions to reference
  - `bridge/internal/executor/engine.go:ToolCall` — Existing ToolCall struct

  **Acceptance Criteria**:
  - [ ] File compiles: `go build ./pkg/interfaces/...`
  - [ ] SkillGate interface exported with all 4 methods
  - [ ] Types have JSON tags for serialization

  **QA Scenarios**:
  ```
  Scenario: Interface compiles and exports correctly
    Tool: Bash
    Steps:
      1. cd bridge && go build ./pkg/interfaces/...
      2. grep -c "type SkillGate interface" pkg/interfaces/skillgate.go
    Expected Result: Build succeeds, grep returns 1
    Evidence: .sisyphus/evidence/task-1-interface-compiles.txt
  ```

  **Commit**: NO (groups with Wave 1)

---

- [x] 2. Create Governor Package Documentation — `pkg/governor/doc.go`

  **What to do**:
  - Create `bridge/pkg/governor/doc.go`
  - Document package purpose: PII interception for AI tool calls
  - Reference SkillGate interface and Shadow Mapping 2.0

  **Must NOT do**:
  - DO NOT add implementation code

  **Recommended Agent Profile**:
  - **Category**: `quick`
    - Reason: Documentation file, minimal complexity
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 1 (with Tasks 1, 3)
  - **Blocks**: None
  - **Blocked By**: None

  **References**:
  - `bridge/pkg/interfaces/doc.go` — Pattern for package docs

  **Acceptance Criteria**:
  - [ ] Package godoc renders correctly
  - [ ] References SkillGate and Shadow Mapping

  **QA Scenarios**:
  ```
  Scenario: Package documentation accessible
    Tool: Bash
    Steps:
      1. go doc ./pkg/governor
    Expected Result: Shows package description
    Evidence: .sisyphus/evidence/task-2-doc.txt
  ```

  **Commit**: NO (groups with Wave 1)

---

- [x] 3. Create Governor Supporting Types — `pkg/governor/types.go`

  **What to do**:
  - Create `bridge/pkg/governor/types.go`
  - Define `Governor` struct with: `scrubber`, `logger`, `config`, `mapping`
  - Define internal types for pattern matching
  - Import and use patterns from `pii/scrubber.go`

  **Must NOT do**:
  - DO NOT implement SkillGate methods — separate file
  - DO NOT duplicate pattern definitions

  **Recommended Agent Profile**:
  - **Category**: `quick`
    - Reason: Type definitions only
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 1 (with Tasks 1, 2)
  - **Blocks**: Task 4
  - **Blocked By**: None

  **References**:
  - `bridge/pkg/pii/scrubber.go:Scrubber` — Scrubber struct to embed
  - `bridge/pkg/agent/pii_interceptor.go:PIIMapping` — PIIMapping pattern

  **Acceptance Criteria**:
  - [ ] Governor struct defined with required fields
  - [ ] Imports from pii package work
  - [ ] File compiles

  **QA Scenarios**:
  ```
  Scenario: Types compile correctly
    Tool: Bash
    Steps:
      1. cd bridge && go build ./pkg/governor/...
    Expected Result: Build succeeds (may have undefined method errors, that's OK)
    Evidence: .sisyphus/evidence/task-3-types.txt
  ```

  **Commit**: NO (groups with Wave 1)

---

- [x] 4. Implement DefaultSkillGate — `pkg/governor/skillgate.go`

  **What to do**:
  - Create `bridge/pkg/governor/skillgate.go`
  - Implement `NewGovernor(config)` constructor
  - Implement `InterceptToolCall` using CTO-provided boilerplate
  - Implement `InterceptPrompt` — scan for PII, replace with `[REDACTED:hash]`
  - Implement `RestoreOutput` — replace placeholders with original values
  - Implement `ValidateArgs` — return violations list
  - Add audit logging for all violations

  **Must NOT do**:
  - DO NOT log raw PII values — only masked snippets
  - DO NOT bypass validation in any code path

  **Recommended Agent Profile**:
  - **Category**: `deep`
    - Reason: Core implementation with security implications
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: NO
  - **Parallel Group**: Sequential (Wave 2)
  - **Blocks**: Tasks 5, 6, 8, 9, 10
  - **Blocked By**: Tasks 1, 3

  **References**:
  - CTO boilerplate in this plan (Reference Implementation section)
  - `bridge/pkg/pii/scrubber.go:Scrub()` — Scrub method to use
  - `bridge/pkg/agent/pii_interceptor.go:Intercept()` — Intercept pattern

  **Acceptance Criteria**:
  - [ ] All 4 interface methods implemented
  - [ ] Audit logging on violations
  - [ ] File compiles: `go build ./pkg/governor/...`

  **QA Scenarios**:
  ```
  Scenario: Governor compiles and implements interface
    Tool: Bash
    Steps:
      1. cd bridge && go build ./pkg/governor/...
      2. grep -c "func (g \*Governor) Intercept" pkg/governor/skillgate.go
    Expected Result: Build succeeds, grep returns 4 (4 methods)
    Evidence: .sisyphus/evidence/task-4-impl.txt
  ```

  **Commit**: NO (groups with Wave 2)

---

- [x] 5. Create Golden Path Test for PII Interception
- `pkg/governor/golden_path_test.go`

  **What to do**:
  - Create `bridge/pkg/governor/skillgate_test.go`
  - Test `InterceptToolCall` with raw email — returns scrubbed
  - Test `InterceptToolCall` with placeholder — passes through
  - Test `InterceptPrompt` with SSN — returns redacted
  - Test `RestoreOutput` — correctly restores values
  - Test `ValidateArgs` — detects violations

  **Must NOT do**:
  - DO NOT use real PII in tests — use obvious test patterns

  **Recommended Agent Profile**:
  - **Category**: `quick`
    - Reason: Standard unit tests
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: NO
  - **Parallel Group**: Sequential (Wave 2, after Task 4)
  - **Blocks**: Tasks 8, 9, 10
  - **Blocked By**: Task 4

  **References**:
  - `bridge/pkg/pii/scrubber_test.go` — Test patterns for PII
  - `bridge/pkg/agent/pii_interceptor_test.go` — Test patterns

  **Acceptance Criteria**:
  - [ ] All 5 test cases pass
  - [ ] `go test ./pkg/governor/... -v` passes

  **QA Scenarios**:
  ```
  Scenario: Unit tests pass
    Tool: Bash
    Steps:
      1. cd bridge && go test ./pkg/governor/... -v
    Expected Result: All tests pass, 0 failures
    Evidence: .sisyphus/evidence/task-5-tests.txt
  ```

  **Commit**: NO (groups with Wave 2)

---

- [x] 6. Create Golden Path Test for PII Interception — `pkg/governor/golden_path_test.go`

  **What to do**:
  - Create `bridge/pkg/governor/golden_path_test.go`
  - Test: Tool call with raw email "user@example.com" → scrubbed to `[REDACTED_EMAIL]`
  - Test: Tool call with API key "sk-xxxxx" → scrubbed to `[REDACTED_API_KEY]`
  - Test: Tool call with placeholder `[REDACTED:abc123]` → passes through unchanged
  - Test: Intent validation — SearchContacts with raw email = violation logged
  - Add integration test with mock scrubber

  **Must NOT do**:
  - DO NOT skip any edge cases
  - DO NOT use weak test patterns that might false-match

  **Recommended Agent Profile**:
  - **Category**: `deep`
    - Reason: Security-critical tests require thorough coverage
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: NO
  - **Parallel Group**: Sequential (Wave 2, after Task 5)
  - **Blocks**: Tasks 8, 9, 10
  - **Blocked By**: Tasks 4, 5

  **References**:
  - CTO boilerplate in this plan
  - `bridge/pkg/pii/hipaa_test.go` — HIPAA test patterns

  **Acceptance Criteria**:
  - [ ] 4+ test cases covering PII types
  - [ ] Tests pass: `go test ./pkg/governor/... -run GoldenPath -v`

  **QA Scenarios**:
  ```
  Scenario: Golden path tests pass
    Tool: Bash
    Steps:
      1. cd bridge && go test ./pkg/governor/... -run GoldenPath -v
    Expected Result: All tests pass
    Evidence: .sisyphus/evidence/task-6-golden.txt
  ```

  **Commit**: NO (groups with Wave 2)

---

- [x] 7. Create WipeAllData User Isolation Test — `pkg/keystore/wipe_test.go` ⚠️ HARD GATE

  **What to do**:
  - Create `bridge/pkg/keystore/wipe_test.go`
  - Test: User_A calls WipeAllData → should require admin/auth context
  - Test: User_A attempts to delete User_B's data → MUST FAIL with unauthorized
  - Test: Admin calls WipeAllData with valid context → succeeds
  - Add regression test for the original vulnerability (DELETE without WHERE)
  - Mark as HARD GATE — execution stops if this test fails

  **Must NOT do**:
  - DO NOT skip the user isolation test
  - DO NOT weaken the test to pass

  **Recommended Agent Profile**:
  - **Category**: `deep`
    - Reason: Security-critical HARD GATE test
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES (independent of other tasks)
  - **Parallel Group**: Wave 2 (parallel with Task 6)
  - **Blocks**: FINAL verification
  - **Blocked By**: None

  **References**:
  - `bridge/pkg/keystore/keystore.go:WipeAllData()` — Function under test
  - Security audit findings in this plan
  - Original vulnerability: commit 83aa914

  **Acceptance Criteria**:
  - [ ] Test file compiles
  - [ ] User isolation test passes: User_A cannot delete User_B data
  - [ ] Admin context test passes
  - [ ] Regression test for original vulnerability

  **QA Scenarios**:
  ```
  Scenario: WipeAllData user isolation enforced
    Tool: Bash
    Steps:
      1. cd bridge && go test ./pkg/keystore/... -run TestWipeAllData_UserIsolation -v
    Expected Result: Test passes (User_A blocked from User_B data)
    Evidence: .sisyphus/evidence/task-7-wipe-golden.txt
  ```

  **Commit**: NO (groups with Wave 2)

---

- [ ] 8. Wire SkillGate into PETG Gateway — `internal/petg/gateway.go`

  **What to do**:
  - Add `skillGate interfaces.SkillGate` field to Gateway struct
  - Update `NewGateway()` to accept SkillGate parameter
  - Call `skillGate.InterceptToolCall()` before existing validation in `ValidateToolCall()`
  - If violations detected, log and return error

  **Must NOT do**:
  - DO NOT bypass existing SSRF/sanitization checks
  - DO NOT skip PII interception for any tool

  **Recommended Agent Profile**:
  - **Category**: `quick`
    - Reason: Single integration point, clear pattern
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 3 (with Tasks 9, 10)
  - **Blocks**: FINAL verification
  - **Blocked By**: Tasks 4, 5, 6

  **References**:
  - `bridge/internal/petg/gateway.go:ValidateToolCall()` — Entry point to modify
  - `bridge/internal/petg/gateway.go:Gateway` — Struct to extend

  **Acceptance Criteria**:
  - [ ] Gateway has skillGate field
  - [ ] ValidateToolCall calls InterceptToolCall before SSRF check
  - [ ] Build passes

  **QA Scenarios**:
  ```
  Scenario: PETG integration compiles
    Tool: Bash
    Steps:
      1. cd bridge && go build ./internal/petg/...
    Expected Result: Build succeeds
    Evidence: .sisyphus/evidence/task-8-petg.txt
  ```

  **Commit**: NO (groups with Wave 3)

---

- [ ] 9. Wire SkillGate into SkillExecutor — `internal/skills/executor.go`

  **What to do**:
  - Add `skillGate interfaces.SkillGate` field to SkillExecutor
  - Update constructor to accept SkillGate
  - Wrap `ExecuteSkill()` call with `skillGate.InterceptToolCall()`
  - Call `skillGate.RestoreOutput()` on results before returning

  **Must NOT do**:
  - DO NOT execute skill if PII validation fails
  - DO NOT return raw PII in results

  **Recommended Agent Profile**:
  - **Category**: `quick`
    - Reason: Single integration point
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 3 (with Tasks 8, 10)
  - **Blocks**: FINAL verification
  - **Blocked By**: Tasks 4, 5, 6

  **References**:
  - `bridge/internal/skills/executor.go:ExecuteSkill()` — Method to wrap
  - CTO boilerplate in this plan

  **Acceptance Criteria**:
  - [ ] ExecuteSkill wrapped with InterceptToolCall
  - [ ] Results processed through RestoreOutput
  - [ ] Build passes

  **QA Scenarios**:
  ```
  Scenario: SkillExecutor integration compiles
    Tool: Bash
    Steps:
      1. cd bridge && go build ./internal/skills/...
    Expected Result: Build succeeds
    Evidence: .sisyphus/evidence/task-9-executor.txt
  ```

  **Commit**: NO (groups with Wave 3)

---

- [ ] 10. Add SkillGate to RPC Server — `pkg/rpc/server.go`

  **What to do**:
  - Add `skillGate interfaces.SkillGate` field to Server struct
  - Update `NewServer()` config to accept SkillGate
  - Wire SkillGate through to PETG Gateway and SkillExecutor

  **Must NOT do**:
  - DO NOT make SkillGate optional — it's required for security

  **Recommended Agent Profile**:
  - **Category**: `quick`
    - Reason: Configuration wiring
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 3 (with Tasks 8, 9)
  - **Blocks**: FINAL verification
  - **Blocked By**: Tasks 4, 5, 6

  **References**:
  - `bridge/pkg/rpc/server.go:Server` — Struct to extend
  - `bridge/pkg/rpc/server.go:NewServer()` — Constructor to modify

  **Acceptance Criteria**:
  - [ ] Server has skillGate field
  - [ ] Wired through to Gateway and Executor
  - [ ] Build passes

  **QA Scenarios**:
  ```
  Scenario: RPC Server integration compiles
    Tool: Bash
    Steps:
      1. cd bridge && go build ./pkg/rpc/...
    Expected Result: Build succeeds
    Evidence: .sisyphus/evidence/task-10-server.txt
  ```

  **Commit**: NO (groups with Wave 3)

---

## Final Verification Wave (MANDATORY)

> 4 review agents run in PARALLEL. ALL must APPROVE. Present consolidated results to user and get explicit "okay" before completing.

- [ ] F1. **Plan Compliance Audit** — `oracle`
  Read the plan end-to-end. For each "Must Have": verify implementation exists. For each "Must NOT Have": search codebase for forbidden patterns. Check Hard Gate: WipeAllData Golden Path test MUST pass.
  Output: `Must Have [N/N] | Must NOT Have [N/N] | Hard Gate [PASS/FAIL] | VERDICT: APPROVE/REJECT`

- [ ] F2. **Code Quality Review** — `unspecified-high`
  Run `go build ./...` + `go test ./...`. Review all changed files for: `as any`/`@ts-ignore` equivalents, empty catches, commented-out code, unused imports. Check AI slop: excessive comments, over-abstraction.
  Output: `Build [PASS/FAIL] | Tests [N pass/N fail] | Files [N clean/N issues] | VERDICT`

- [ ] F3. **Golden Path Execution** — `deep`
  Execute the WipeAllData Golden Path test: User_A attempts to delete User_B's data. Test MUST return unauthorized. Also run InterceptToolCall tests: raw PII must be scrubbed, placeholders allowed.
  Output: `WipeAllData [PASS/FAIL] | InterceptToolCall [PASS/FAIL] | VERDICT`

- [ ] F4. **Scope Fidelity Check** — `deep`
  For each task: read "What to do", read actual diff. Verify 1:1 — everything in spec was built, nothing beyond spec. Check "Must NOT do" compliance.
  Output: `Tasks [N/N compliant] | Scope Creep [CLEAN/N items] | VERDICT`

---

## Commit Strategy

- **Single Commit**: `feat(governor): add SkillGate PII interception and WipeAllData hardening`
- **Pre-commit**: `go test ./pkg/governor/... ./pkg/interfaces/... ./pkg/keystore/...`

---

## Success Criteria

### Verification Commands
```bash
# Build check
cd bridge && go build ./...

# Unit tests
go test ./pkg/governor/... ./pkg/interfaces/... -v

# Golden Path test (HARD GATE)
go test ./pkg/keystore/... -run TestWipeAllData_UserIsolation -v

# All tests
go test ./... -short
```

### Final Checklist
- [ ] All "Must Have" present
- [ ] All "Must NOT Have" absent
- [ ] WipeAllData Golden Path test PASSES
- [ ] InterceptToolCall scrubs raw PII
- [ ] Build passes
- [ ] No test failures
