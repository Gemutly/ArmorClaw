# Remaining Work: Fix Router + Integration Tests

## Overview

This plan completes Phase 1 of ToolSidecar MCP Architecture by:
1. Fixing 4 syntax errors in `bridge/pkg/mcp/router.go`
2. Running integration tests (Tasks 10-11 from original plan)

## Prerequisites
- ✅ All 8 core tasks implemented (Tasks 1, 2, 3, 4, 5, 6, 7, 9)
- ✅ 67 tests passing
- ❌ Task 8 has syntax errors (blocking integration tests)

## Work Items

### Part 1: Fix Router Syntax Errors [quick]

**Goal**: Fix 4 syntax errors in `bridge/pkg/mcp/router.go`

**Problem**: Struct tags are using backticks (`\`) instead of double quotes (")

**Lines to fix** (lines 83, 85, 90, 95, 97, 102):
```go
// WRONG (current):
JSONRPC string      `json:"jsonrpc"`

// CORRECT (target):
JSONRPC string      `json:"jsonrpc"`
```

**Files to modify**:
- `bridge/pkg/mcp/router.go`

**Verification**:
- `go build ./pkg/mcp/...` → PASS (0 errors)

**Evidence**:
- `.sisyphus/evidence/router-fix-build.log`

---

### Part 2: End-to-End Integration Test [deep]

**Goal**: Test complete flow: OpenClaw → Go Bridge → Rust Vault → ToolSidecar

**Depends on**: Part 1 complete

**What to do**:
- Create `tests/integration/e2e_test.go`
- Test browser.navigate tool end-to-end
- Verify PII redaction at each layer
- Verify audit trail completeness
- Test with real components (not mocks)

**QA Scenarios**:
1. Full flow OpenClaw to ToolSidecar works
2. PII redacted at every layer
3. Error handling works end-to-end
4. Performance within latency target (<100ms)

**Evidence**:
- `.sisyphus/evidence/task-10-e2e-flow.log`
- `.sisyphus/evidence/task-10-pii-redaction-e2e.log`
- `.sisyphus/evidence/task-10-error-handling.log`
- `.sisyphus/evidence/task-10-performance.log`

---

### Part 3: Crash Recovery Test [deep]

**Goal**: Test orphan cleanup when OpenClaw crashes

**Depends on**: Part 1 + Part 2 complete

**What to do**:
- Create `tests/integration/crash_test.go`
- Test ToolSidecar cleanup within 60 seconds of crash
- Test session state recovery after Go Bridge restart
- Test concurrent crash scenarios

**QA Scenarios**:
1. ToolSidecar cleaned up after OpenClaw crash
2. Session state recovered after Go Bridge restart
3. Multiple concurrent crashes handled correctly
4. Active execution protected during crash

**Evidence**:
- `.sisyphus/evidence/task-11-crash-cleanup.log`
- `.sisyphus/evidence/task-11-session-recovery.log`
- `.sisyphus/evidence/task-11-concurrent-crashes.log`
- `.sisyphus/evidence/task-11-active-protection.log`

---

## Final Verification Wave (After Parts 1-3)

Run these 4 verification agents in parallel:
- F1: Plan Compliance Audit (oracle)
- F2: Code Quality Review (run all tests)
- F3: Security Audit (verify controls)
- F4: Manual QA Scenarios

## Commit Strategy

After Part 1: `fix(mcp): correct struct tag syntax in router.go`
After Parts 2-3: `test(integration): add end-to-end and crash recovery tests`

## Success Criteria

- [ ] Part 1: Router builds without errors
- [ ] Part 2: E2E tests pass (4 scenarios)
- [ ] Part 3: Crash recovery tests pass (4 scenarios)
- [ ] Final Verification: All 4 agents approve
- [ ] Total tests: 67 + new integration tests

## Estimated Time

- Part 1: 5 minutes (simple find/replace)
- Part 2: 20-30 minutes (integration test setup + execution)
- Part 3: 15-20 minutes (crash test setup + execution)
- Final Verification: 10 minutes (parallel agent execution)

**Total**: ~1 hour

---

## Instructions for Sisyphus

1. **Fix router.go first** - This is blocking everything else
2. **Run tests to verify fix** - `go build ./pkg/mcp/...`
3. **Launch integration tests** - Parts 2 and 3 can run in parallel after Part 1
4. **Run final verification** - All 4 agents in parallel
5. **Present results to user** - Get explicit approval
6. **Commit changes** - Group related work together
