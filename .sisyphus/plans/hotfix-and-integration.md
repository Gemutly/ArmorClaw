# Hotfix and Integration - Phase 1 Completion

## TL;DR

> **Quick Summary**: Fix 3 syntax errors in Task 8 (router.go), verify build passes, then execute integration tests (Tasks 10-11) and final verification (F1-F4) to complete Phase 1.
>
> **Deliverables**:
> - Fixed `bridge/pkg/mcp/router.go` (3 return statements corrected)
> - MCP package builds with 0 errors
> - MCP package tests pass
> - E2E integration test evidence (Task 10)
> - Crash recovery test evidence (Task 11)
> - Final verification reports (F1-F4)
>
> **Estimated Effort**: Quick (30-60 minutes)
> **Parallel Execution**: NO - Sequential (hotfix must complete first)
> **Critical Path**: Hotfix → Build Gate → Task 10 → Task 11 → F1-F4

---

## Context

### Current State

**Completed (8/11 tasks)**:
- ✅ Task 5: Rust Vault Binary - Compiles with 0 errors
- ✅ Task 1: Docker Network Isolation
- ✅ Task 2: Egress Proxy Core (DENY ALL)
- ✅ Task 3: Egress Allowlist (LLM APIs)
- ✅ Task 4: Audit Logging
- ✅ Task 6: Protocol Translator
- ✅ Task 7: Session Lifecycle Manager
- ✅ Task 9: ToolSidecar Provisioner

**Blocking Issue**:
- Task 8 (MCP Router) has 3 syntax errors in `bridge/pkg/mcp/router.go`

### The Problem

Go function signatures expect `(*MCPResponse, error)` tuple returns, but 3 `errorResponse()` calls return only `*MCPResponse`:

```go
// Current (WRONG):
return r.errorResponse(req.ID, -32603, "SkillGate validation failed", err.Error())

// Required (CORRECT):
return r.errorResponse(req.ID, -32603, "SkillGate validation failed", err.Error()), nil
```

**Why `, nil` is correct**: MCP uses JSON-RPC. The error must be wrapped inside `*MCPResponse` payload so the client receives the JSON error object. A non-nil Go error would drop the connection instead of returning the error payload.

---

## Work Objectives

### Core Objective

Fix Task 8 syntax errors, verify build passes, then complete integration testing to close out Phase 1.

### Concrete Deliverables

1. `bridge/pkg/mcp/router.go` - Lines 182, 281, 338 fixed
2. Build output showing 0 errors
3. Test output showing all tests pass
4. `.sisyphus/evidence/task-10-e2e-integration.log`
5. `.sisyphus/evidence/task-11-crash-recovery.log`
6. `.sisyphus/evidence/final-verification/` directory with F1-F4 reports

### Definition of Done

- [ ] `go build ./pkg/mcp/...` returns 0 errors
- [ ] `go test ./pkg/mcp/...` passes all tests
- [ ] E2E integration test completes with round-trip latency logged
- [ ] Crash recovery test shows ToolSidecar cleanup within 60 seconds
- [ ] All 4 final verification agents report APPROVE

### Must Have

- **Exact line edits** at lines 182, 281, 338
- **Zero compilation errors** before proceeding
- **Evidence files** for all tests

### Must NOT Have

- NO proceeding past Build Gate with errors
- NO skipped tests
- NO missing evidence files

---

## Verification Strategy

### Test Decision

- **Infrastructure exists**: YES (Go test framework)
- **Automated tests**: YES (go test)
- **Framework**: Go `testing` package
- **TDD**: Fix first, then verify with tests

### QA Policy

Every task MUST include agent-executed QA scenarios.
Evidence saved to `.sisyphus/evidence/task-{N}-{scenario-slug}.{ext}`.

---

## Execution Strategy

### Sequential Execution (NO PARALLELISM)

```
Step 1: HOTFIX - Fix 3 return statements in router.go
    └── MUST complete before any other step

Step 2: BUILD GATE - Verify compilation
    └── BLOCKS all downstream work if failed

Step 3: Task 10 - E2E Integration Test
    └── Depends on Step 2 success

Step 4: Task 11 - Crash Recovery Test
    └── Depends on Task 10 success

Step 5: F1-F4 - Final Verification Wave
    └── Depends on Tasks 10-11 success
    └── 4 parallel reviews, then user okay required
```

Critical Path: Hotfix → Build Gate → Task 10 → Task 11 → F1-F4 → user okay

---

## TODOs

---

### Step 1: HOTFIX - Fix router.go Return Statements

- [ ] **1. Fix Line 182**

  **What to do**:
  - Open `bridge/pkg/mcp/router.go`
  - Navigate to line 182
  - Find: `return r.errorResponse(req.ID, -32603, "SkillGate validation failed", err.Error())`
  - Replace with: `return r.errorResponse(req.ID, -32603, "SkillGate validation failed", err.Error()), nil`

  **Must NOT do**:
  - Do NOT modify any other code
  - Do NOT change the error message

  **References**:
  - File: `bridge/pkg/mcp/router.go`
  - Line: 182
  - Context: Inside SkillGate validation failure block

  **Acceptance Criteria**:
  - [ ] Line 182 ends with `, nil)`

  **QA Scenarios**:
  ```
  Scenario: Line 182 is correctly fixed
    Tool: Bash (grep)
    Steps:
      1. grep -n "SkillGate validation failed" bridge/pkg/mcp/router.go
      2. Verify the line ends with ", nil)"
    Expected Result: Line shows return statement with tuple
    Evidence: .sisyphus/evidence/hotfix-line-182.txt
  ```

- [ ] **2. Fix Line 281**

  **What to do**:
  - Open `bridge/pkg/mcp/router.go`
  - Navigate to line 281
  - Find: `return r.errorResponse(req.ID, -32603, "Consent required but failed", err.Error())`
  - Replace with: `return r.errorResponse(req.ID, -32603, "Consent required but failed", err.Error()), nil`

  **Must NOT do**:
  - Do NOT modify any other code

  **References**:
  - File: `bridge/pkg/mcp/router.go`
  - Line: 281
  - Context: Inside HITL consent failure block

  **Acceptance Criteria**:
  - [ ] Line 281 ends with `, nil)`

  **QA Scenarios**:
  ```
  Scenario: Line 281 is correctly fixed
    Tool: Bash (grep)
    Steps:
      1. grep -n "Consent required but failed" bridge/pkg/mcp/router.go
      2. Verify the line ends with ", nil)"
    Expected Result: Line shows return statement with tuple
    Evidence: .sisyphus/evidence/hotfix-line-281.txt
  ```

- [ ] **3. Fix Line 338**

  **What to do**:
  - Open `bridge/pkg/mcp/router.go`
  - Navigate to line 338
  - Find: `return r.errorResponse(req.ID, -32603, "Failed to spawn tool container: %s", err.Error())`
  - Replace with: `return r.errorResponse(req.ID, -32603, "Failed to spawn tool container: %s", err.Error()), nil`

  **Must NOT do**:
  - Do NOT modify any other code

  **References**:
  - File: `bridge/pkg/mcp/router.go`
  - Line: 338
  - Context: Inside ToolSidecar spawn failure block

  **Acceptance Criteria**:
  - [ ] Line 338 ends with `, nil)`

  **QA Scenarios**:
  ```
  Scenario: Line 338 is correctly fixed
    Tool: Bash (grep)
    Steps:
      1. grep -n "Failed to spawn tool container" bridge/pkg/mcp/router.go
      2. Verify the line ends with ", nil)"
    Expected Result: Line shows return statement with tuple
    Evidence: .sisyphus/evidence/hotfix-line-338.txt
  ```

- [ ] **4. Fix Test File Issues (if present)**

  **What to do**:
  - Check for duplicate declarations in `bridge/pkg/mcp/router_test.go` and `bridge/pkg/mcp/simple_test.go`
  - Remove `simple_test.go` if it duplicates `router_test.go`
  - Fix undefined type references

  **Must NOT do**:
  - Do NOT delete valid test cases

  **References**:
  - Files: `bridge/pkg/mcp/router_test.go`, `bridge/pkg/mcp/simple_test.go`

  **Acceptance Criteria**:
  - [ ] No redeclaration errors
  - [ ] No undefined type errors

---

### Step 2: BUILD GATE - Verify Compilation

- [ ] **5. Run Go Build**

  **What to do**:
  - Navigate to `bridge/` directory
  - Run: `go build ./pkg/mcp/...`
  - Verify 0 errors

  **Must NOT do**:
  - Do NOT proceed to Step 3 if errors exist

  **Parallelization**:
  - **Can Run In Parallel**: NO
  - **Blocked By**: Steps 1-4 (hotfix)

  **Acceptance Criteria**:
  - [ ] `go build ./pkg/mcp/...` exits with code 0
  - [ ] No error output

  **QA Scenarios**:
  ```
  Scenario: MCP package builds successfully
    Tool: Bash (go build)
    Steps:
      1. cd /home/mink/src/armorclaw-omo/bridge
      2. go build ./pkg/mcp/... 2>&1 | tee .sisyphus/evidence/build-gate.log
      3. echo "Exit code: $?"
    Expected Result: Exit code 0, no error messages
    Failure Indicators: "error[" in output, non-zero exit code
    Evidence: .sisyphus/evidence/build-gate.log
  ```

- [ ] **6. Run Go Tests**

  **What to do**:
  - Run: `go test ./pkg/mcp/... -v`
  - Verify all tests pass

  **Must NOT do**:
  - Do NOT proceed to Step 3 if tests fail

  **Parallelization**:
  - **Can Run In Parallel**: NO
  - **Blocked By**: Step 5 (build)

  **Acceptance Criteria**:
  - [ ] `go test ./pkg/mcp/...` exits with code 0
  - [ ] All tests show PASS

  **QA Scenarios**:
  ```
  Scenario: MCP package tests pass
    Tool: Bash (go test)
    Steps:
      1. cd /home/mink/src/armorclaw-omo/bridge
      2. go test ./pkg/mcp/... -v 2>&1 | tee .sisyphus/evidence/test-gate.log
      3. echo "Exit code: $?"
    Expected Result: Exit code 0, all tests PASS
    Failure Indicators: "FAIL" in output, non-zero exit code
    Evidence: .sisyphus/evidence/test-gate.log
  ```

---

### Step 3: Task 10 - E2E Integration Test

- [ ] **7. E2E Integration Test**

  **What to do**:
  1. Ensure `armorclaw-isolated` Docker network exists
  2. Start Rust Vault binary: `./target/release/armorclaw-sidecar`
  3. Start Go Bridge with MCP router enabled
  4. Send simulated OpenClaw native RPC call:
     ```json
     {"jsonrpc":"2.0","id":"test-001","method":"browser.navigate","params":{"url":"https://example.com"}}
     ```
  5. Verify the flow:
     - RPC → Protocol Translator → MCP format
     - MCP → SkillGate validation
     - SkillGate → ToolSidecar provisioning
     - ToolSidecar executes browser.navigate
     - Response returns through the chain
  6. Log round-trip latency (start timestamp to response timestamp)

  **Must NOT do**:
  - Do NOT skip any verification step
  - Do NOT proceed if any step fails

  **Parallelization**:
  - **Can Run In Parallel**: NO
  - **Blocked By**: Steps 5-6 (Build Gate)

  **References**:
  - Network: `armorclaw-isolated` (created in Task 1)
  - Rust Vault: `sidecar/target/release/armorclaw-sidecar`
  - Protocol Translator: `bridge/pkg/translator/rpc_to_mcp.go`
  - MCP Router: `bridge/pkg/mcp/router.go`
  - ToolSidecar Provisioner: `bridge/pkg/toolsidecar/provisioner.go`

  **Acceptance Criteria**:
  - [ ] RPC call successfully translated to MCP
  - [ ] SkillGate validation passes (or fails gracefully with proper error)
  - [ ] ToolSidecar container spawned (if validation passes)
  - [ ] Response received within 30 seconds
  - [ ] Round-trip latency logged

  **QA Scenarios**:
  ```
  Scenario: E2E integration test completes successfully
    Tool: Bash (integration test script)
    Preconditions:
      - armorclaw-isolated network exists
      - Rust Vault binary compiled
      - Go Bridge built
    Steps:
      1. docker network inspect armorclaw-isolated
      2. cd /home/mink/src/armorclaw-omo/sidecar && ./target/release/armorclaw-sidecar &
      3. cd /home/mink/src/armorclaw-omo/bridge && ./armorclaw-bridge &
      4. sleep 5  # Wait for services to start
      5. START=$(date +%s.%N)
      6. echo '{"jsonrpc":"2.0","id":"test-001","method":"browser.navigate","params":{"url":"https://example.com"}}' | nc -U /run/armorclaw/bridge.sock
      7. END=$(date +%s.%N)
      8. echo "Round-trip latency: $(echo "$END - $START" | bc) seconds"
      9. docker ps --filter name=toolsidecar  # Verify ToolSidecar spawned
    Expected Result: Response received, latency logged, ToolSidecar spawned
    Failure Indicators: No response, timeout, no ToolSidecar container
    Evidence: .sisyphus/evidence/task-10-e2e-integration.log
  ```

  **Commit**: YES
  - Message: `fix(mcp): correct errorResponse return tuples (lines 182, 281, 338)`
  - Files: `bridge/pkg/mcp/router.go`

---

### Step 4: Task 11 - Crash Recovery Test

- [ ] **8. Crash Recovery Test**

  **What to do**:
  1. Start a fresh E2E session (repeat Task 10 setup)
  2. Record the ToolSidecar container ID
  3. Simulate OpenClaw crash: `docker kill <openclaw-container>`
  4. Monitor Session Lifecycle Manager for orphan detection
  5. Verify ToolSidecar container is removed within 60 seconds
  6. Check audit logs for cleanup event

  **Must NOT do**:
  - Do NOT proceed if cleanup takes >60 seconds
  - Do NOT manually remove the ToolSidecar (must be automatic)

  **Parallelization**:
  - **Can Run In Parallel**: NO
  - **Blocked By**: Task 10

  **References**:
  - Session Lifecycle Manager: `bridge/pkg/translator/session_manager.go`
  - Cleanup interval: 60 seconds (configured in Task 7)
  - Audit log: `/var/log/armorclaw/audit.db`

  **Acceptance Criteria**:
  - [ ] OpenClaw container killed
  - [ ] Orphan detected by Session Lifecycle Manager
  - [ ] ToolSidecar container removed within 60 seconds
  - [ ] Cleanup event logged in audit.db

  **QA Scenarios**:
  ```
  Scenario: Crash recovery cleans up orphaned ToolSidecar
    Tool: Bash (crash simulation)
    Preconditions:
      - E2E session active with ToolSidecar running
      - Session Lifecycle Manager running
    Steps:
      1. TOOLSIDECAR_ID=$(docker ps --filter name=toolsidecar -q)
      2. echo "ToolSidecar ID: $TOOLSIDECAR_ID"
      3. OPENCLAW_ID=$(docker ps --filter name=openclaw -q)
      4. docker kill $OPENCLAW_ID
      5. echo "OpenClaw killed at $(date)"
      6. START=$(date +%s)
      7. for i in {1..60}; do
           if ! docker ps --filter name=toolsidecar | grep -q $TOOLSIDECAR_ID; then
             END=$(date +%s)
             echo "ToolSidecar cleaned up in $((END - START)) seconds"
             exit 0
           fi
           sleep 1
         done
      8. echo "FAILED: ToolSidecar not cleaned up within 60 seconds"
      9. exit 1
    Expected Result: ToolSidecar removed within 60 seconds
    Failure Indicators: ToolSidecar still running after 60 seconds
    Evidence: .sisyphus/evidence/task-11-crash-recovery.log
  ```

---

### Step 5: Final Verification Wave

> **4 review agents run in PARALLEL. ALL must APPROVE.**
> **Present consolidated results to user and get explicit "okay" before completing.**

- [ ] **F1. Plan Compliance Audit** — `oracle`

  **What to do**:
  - Read `.sisyphus/plans/toolsidecar-mcp-phase1.md` end-to-end
  - For each "Must Have": verify implementation exists (read file, curl endpoint, run command)
  - For each "Must NOT Have": search codebase for forbidden patterns
  - Check evidence files exist in `.sisyphus/evidence/`
  - Compare deliverables against plan

  **Output Format**:
  ```
  Must Have [N/N]:
    - [ ] Item 1: [VERIFIED/MISSING]
    - [ ] Item 2: [VERIFIED/MISSING]
  
  Must NOT Have [N/N]:
    - [ ] Forbidden 1: [ABSENT/FOUND at file:line]
    - [ ] Forbidden 2: [ABSENT/FOUND at file:line]
  
  Tasks [N/N]:
    - [ ] Task X: [COMPLETE/INCOMPLETE]
  
  Evidence Files: [N found]
  
  VERDICT: APPROVE / REJECT
  ```

  **QA Scenarios**:
  ```
  Scenario: Plan compliance audit passes
    Tool: oracle agent
    Steps:
      1. Read plan file
      2. Verify each Must Have item
      3. Verify each Must NOT Have item
      4. Count evidence files
      5. Generate report
    Expected Result: VERDICT: APPROVE
    Evidence: .sisyphus/evidence/final-verification/F1-plan-compliance.txt
  ```

- [ ] **F2. Code Quality Review** — `unspecified-high`

  **What to do**:
  - Run `go fmt ./pkg/mcp/...` and verify no changes
  - Run `go vet ./pkg/mcp/...`
  - Review all changed files for:
    - `as any` / `@ts-ignore` (Go equivalent: `interface{}` abuse)
    - Empty catches
    - `fmt.Println` in production code
    - Commented-out code
    - Unused imports
  - Check AI slop: excessive comments, over-abstraction, generic names

  **Output Format**:
  ```
  Build: [PASS/FAIL]
  Fmt: [PASS/FAIL - N files changed]
  Vet: [PASS/FAIL]
  
  Files Reviewed: N
  Issues Found: N
    - [file:line]: [issue description]
  
  VERDICT: APPROVE / REJECT
  ```

  **QA Scenarios**:
  ```
  Scenario: Code quality review passes
    Tool: unspecified-high agent
    Steps:
      1. go fmt ./pkg/mcp/...
      2. go vet ./pkg/mcp/...
      3. Review changed files
      4. Generate report
    Expected Result: VERDICT: APPROVE
    Evidence: .sisyphus/evidence/final-verification/F2-code-quality.txt
  ```

- [ ] **F3. Security Audit** — `unspecified-high`

  **What to do**:
  - Verify egress proxy DENY ALL policy is enforced
  - Verify no allowlist wildcards exist
  - Verify ToolSidecar containers run with restricted privileges
  - Verify audit logging captures all dropped requests
  - Check for SQL injection, command injection in MCP router
  - Verify SkillGate validation cannot be bypassed

  **Output Format**:
  ```
  Egress Policy: [DENY ALL / BYPASS FOUND]
  Allowlist: [EXACT MATCHES ONLY / WILDCARDS FOUND]
  Container Security: [RESTRICTED / PRIVILEGED]
  Audit Logging: [ENABLED / DISABLED]
  Injection Vulnerabilities: [NONE FOUND / N FOUND]
  
  VERDICT: APPROVE / REJECT
  ```

  **QA Scenarios**:
  ```
  Scenario: Security audit passes
    Tool: unspecified-high agent
    Steps:
      1. Test egress proxy with blocked domain
      2. Inspect allowlist configuration
      3. Inspect ToolSidecar container security options
      4. Query audit.db for dropped requests
      5. Review MCP router for injection vectors
    Expected Result: VERDICT: APPROVE
    Evidence: .sisyphus/evidence/final-verification/F3-security-audit.txt
  ```

- [ ] **F4. Real Manual QA** — `unspecified-high`

  **What to do**:
  - Start from clean state (all containers stopped)
  - Execute EVERY QA scenario from Tasks 10-11
  - Follow exact steps, capture evidence
  - Test cross-task integration (features working together)
  - Test edge cases: empty state, invalid input, rapid actions
  - Save all evidence to `.sisyphus/evidence/final-qa/`

  **Output Format**:
  ```
  Scenarios Executed: N
  Scenarios Passed: N
  Scenarios Failed: N
  
  Integration Tests: N/N passed
  
  Edge Cases Tested: N
    - [edge case]: [PASS/FAIL]
  
  VERDICT: APPROVE / REJECT
  ```

  **QA Scenarios**:
  ```
  Scenario: Manual QA passes
    Tool: unspecified-high agent
    Steps:
      1. Stop all containers
      2. Re-run all QA scenarios from scratch
      3. Test integration between components
      4. Test edge cases
      5. Generate report
    Expected Result: VERDICT: APPROVE
    Evidence: .sisyphus/evidence/final-verification/F4-manual-qa.txt
  ```

---

## Commit Strategy

- **After Step 6 (Build Gate passes)**:
  ```
  git add bridge/pkg/mcp/router.go
  git commit -m "fix(mcp): correct errorResponse return tuples (lines 182, 281, 338)

  - Line 182: SkillGate validation failure
  - Line 281: HITL consent failure
  - Line 338: ToolSidecar spawn failure

  All errorResponse() calls now return (*MCPResponse, error) tuple
  with nil Go error to ensure JSON-RPC error payload is delivered
  to client instead of dropping connection."
  ```

- **After Task 10**:
  ```
  git add .sisyphus/evidence/task-10-*
  git commit -m "test(integration): add E2E integration test evidence (Task 10)"
  ```

- **After Task 11**:
  ```
  git add .sisyphus/evidence/task-11-*
  git commit -m "test(crash-recovery): add crash recovery test evidence (Task 11)"
  ```

- **After F1-F4**:
  ```
  git add .sisyphus/evidence/final-verification/
  git commit -m "docs(verification): add Phase 1 final verification reports (F1-F4)"
  ```

---

## Success Criteria

### Verification Commands

```bash
# Build verification
cd /home/mink/src/armorclaw-omo/bridge
go build ./pkg/mcp/...
# Expected: No output (success)

# Test verification
go test ./pkg/mcp/... -v
# Expected: PASS for all tests

# Integration verification
cat .sisyphus/evidence/task-10-e2e-integration.log
# Expected: Round-trip latency logged, ToolSidecar spawned

# Crash recovery verification
cat .sisyphus/evidence/task-11-crash-recovery.log
# Expected: ToolSidecar cleaned up within 60 seconds

# Final verification
cat .sisyphus/evidence/final-verification/F*.txt
# Expected: All reports show VERDICT: APPROVE
```

### Final Checklist

- [ ] Lines 182, 281, 338 in router.go end with `, nil)`
- [ ] `go build ./pkg/mcp/...` returns 0 errors
- [ ] `go test ./pkg/mcp/...` passes all tests
- [ ] E2E integration test completes with latency logged
- [ ] Crash recovery test shows cleanup within 60 seconds
- [ ] F1-F4 all report APPROVE
- [ ] User provides explicit "okay" after reviewing F1-F4 results

---

## Handoff

**After this plan is saved:**

1. Plan location: `.sisyphus/plans/hotfix-and-integration.md`
2. Execute with: `/start-work hotfix-and-integration`
3. Sisyphus will execute all steps sequentially
4. Final verification results will be presented for user approval
5. Phase 1 complete after user says "okay"

**Estimated total time**: 30-60 minutes
