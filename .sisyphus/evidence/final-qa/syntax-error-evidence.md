# QA Evidence: Critical Syntax Error Found

## Test Item 4: Edge Cases and Syntax Errors

### CRITICAL SYNTAX ERROR FOUND

**Location:** `/home/mink/src/armorclaw-omo/bridge/pkg/mcp/router_test.go`
**Line:** 39
**Error Type:** Missing return value (type mismatch)

### Problematic Code (Lines 34-48)
```go
func (m *mockProvisioner) SpawnToolSidecar(ctx context.Context, skillName, sessionID string) (*toolsidecar.ToolSidecar, error) {
	if m.shouldFail {
        return nil
    }
}
	m.spawned = true
	m.toolName = skillName
	m.sessionID = sessionID
	return &toolsidecar.ToolSidecar{
		ID:        "mock_container_id_123456789012",
		SkillName: skillName,
		SessionID: sessionID,
		CreatedAt: time.Now(),
		Status:    "running",
	}, nil
}
```

### Issues Identified

1. **Line 36: Incorrect return statement**
   ```go
   return nil
   ```
   - **Problem:** Function signature requires `(*toolsidecar.ToolSidecar, error)` but only returning `nil`
   - **Expected:** `return nil, fmt.Errorf("mock failure")` or similar error
   - **Impact:** Compilation error

2. **Line 38: Incorrect brace placement**
   ```go
   }
   ```
   - **Problem:** Closing brace appears after the early return, but before remaining code
   - **Impact:** Lines 39-48 are outside the function scope, causing "expected declaration" error

### Compilation Error Message
```
# github.com/armorclaw/bridge/pkg/mcp
pkg/mcp/router_test.go:39:2: expected declaration, found m
FAIL	github.com/armorclaw/bridge/pkg/mcp [setup failed]
FAIL
```

### LSP Diagnostic
```
error[syntax] at 39:1: expected declaration, found m
```

### Corrected Code Should Be
```go
func (m *mockProvisioner) SpawnToolSidecar(ctx context.Context, skillName, sessionID string) (*toolsidecar.ToolSidecar, error) {
	if m.shouldFail {
		return nil, fmt.Errorf("mock provisioner failure")
	}
	m.spawned = true
	m.toolName = skillName
	m.sessionID = sessionID
	return &toolsidecar.ToolSidecar{
		ID:        "mock_container_id_123456789012",
		SkillName: skillName,
		SessionID: sessionID,
		CreatedAt: time.Now(),
		Status:    "running",
	}, nil
}
```

### Impact Analysis

**Severity:** 🔴 CRITICAL - Blocks test compilation

**Scope:** 
- Affects only test code (router_test.go)
- Does NOT affect production code (router.go)
- Blocks all tests in pkg/mcp from running

**Hotfix Status:**
- The hotfix changes to router.go are CORRECT
- The test infrastructure has a PRE-EXISTING syntax error
- This error must be fixed before tests can verify the hotfix

### Additional LSP Hints (Non-Critical)

File: router.go
- Multiple hints suggesting `interface{}` → `any` (cosmetic, Go 1.18+)
- Suggestion to use tagged switch on sensitivity (code quality, not error)
- Suggestion to use `fmt.Appendf` (performance, not error)

**All hints are informational, not blocking issues.**

## Summary
- Production hotfix code: ✅ CORRECT (3/3 return format checks passed)
- Test helper signature: ✅ CORRECT
- Test infrastructure usage: ✅ CORRECT (5/5 instances)
- **Test compilation status:** ❌ CRITICAL SYNTAX ERROR blocks testing
