# V6Microkernel Wiring Gaps — Broker Prerequisite Assessment

> **Scope**: Audit 5 known wiring gaps in the V6Microkernel codebase.
> **Purpose**: Determine whether each gap blocks broker (Task 9) implementation.
> **Date**: 2026-04-18

---

## Summary

| Gap | Verdict | Blocks Broker? |
|-----|---------|----------------|
| 1. ApprovalEngine not wired to StepExecutor | NOT_RELEVANT | No |
| 2. SealedKeystore never instantiated | RESOLVED | No |
| 3. SecurityConfig never loaded | NOT_RELEVANT | No |
| 4. TrustedWorkflowEngine not wired into step execution | NOT_RELEVANT | No |
| 5. Vault client not passed to MCPRouter | RESOLVED | No |

**Conclusion**: None of the 5 gaps block broker work. The broker uses its own interfaces (`ConsentProvider`, `RiskClassifier`) and does not depend on the existing approval/workflow/security systems.

---

## Detailed Assessment

### Gap 1: ApprovalEngine created but StepExecutor gets nil

**File**: `bridge/cmd/bridge/setup_secretary.go:115-120`
**Issue**: `setupApprovalAndTrust()` (line 51) creates an `ApprovalEngineImpl`, but `setupWorkflowEngine()` hardcodes `ApprovalEngine: nil` when constructing `StepExecutorConfig` (line 118). The `IntegrationConfig` also passes `ApprovalEngine: nil` (line 132).

```go
// setup_secretary.go:115-120
stepExecutor = secretary.NewStepExecutor(secretary.StepExecutorConfig{
    Factory:        fac,
    Validator:      dependencyValidator,
    ApprovalEngine: nil,  // ← always nil, even when approvalEngine is available
    EventBus:       matrixBus,
})
```

The `ApprovalEngine` _is_ wired into `SecretaryCommandHandlerConfig` (line 165) for Matrix chat commands, so it's reachable via `!approval`-style commands — just not during automated step execution.

**Verdict**: `NOT_RELEVANT`
**Rationale**: The broker has its own `ConsentProvider` interface for pre-check consent. It does not execute workflow steps, so the StepExecutor's missing ApprovalEngine is irrelevant. The approval engine is only needed for the secretary's automated workflow execution path, which is separate from the broker.
**Action**: Wire `approvalEngine` into `StepExecutorConfig` and `IntegrationConfig` when both are non-nil. Low priority — only affects secretary workflow step execution, not broker.

---

### Gap 2: SealedKeystore (681 lines) — RESOLVED

**File**: `bridge/pkg/keystore/sealed_keystore.go` (681 lines)
**Issue**: `SealedKeystore` existed with full implementation (challenge-unseal flow, PBKDF2 key derivation, SQLCipher integration) but was never instantiated in any production wiring path. Only referenced in test files.

**Status**: `RESOLVED` — `NewSealedKeystore()` is now called in `main.go:2655` with a `SealedKeystoreConfig`.
**Action**: Complete.

---

### Gap 3: SecurityConfig (675 lines, 5 tiers) never loaded in production

**File**: `bridge/pkg/security/categories.go` (675 lines)
**Issue**: `SecurityConfig` defines a 5-tier security categorization system (TIER_PUBLIC through TIER_RESTRICTED) with policy enforcement. Referenced in `config.go`, `website_guard.go`, and `lockdown.go` but never loaded or activated in production wiring paths.

**Verdict**: `NOT_RELEVANT`
**Rationale**: v0.9.0 scope. The broker's risk classification uses its own `RiskClassifier` interface with a simpler model. The 5-tier system is for a future fine-grained data classification feature, not for broker pre-checks.
**Action**: Schedule for v0.9.0. No action needed for broker.

---

### Gap 4: TrustedWorkflowEngine created but not wired into step execution

**File**: `bridge/cmd/bridge/setup_secretary.go:67-82`
**Issue**: `setupApprovalAndTrust()` creates a `TrustedWorkflowEngine` (line 70), but it's only wired into `SecretaryCommandHandlerConfig` (line 166) for chat commands. It is never passed to the `StepExecutor` or `OrchestratorIntegration`, meaning automated workflow execution has no trusted-workflow enforcement.

The engine itself exists in `bridge/pkg/secretary/trusted_workflows.go` and is tested in `security_test.go`, confirming the logic is sound — just not wired into the execution path.

**Verdict**: `NOT_RELEVANT`
**Rationale**: The broker is a pre-check gate, not a workflow engine. It doesn't execute workflows, so trusted-workflow enforcement at the step-execution level is irrelevant. The broker's authorization is handled by `ConsentProvider` + `RiskClassifier`.
**Action**: Wire `trustEngine` into step execution path if secretary automated workflows need trusted-workflow enforcement. Low priority for broker.

---

### Gap 5: Vault client not passed to MCPRouter — RESOLVED

**File**: `bridge/cmd/bridge/setup_mcp.go:44-50` and `bridge/cmd/bridge/main.go:2261,2469`
**Issue**: `setupVaultClient()` creates a `*vault.VaultGovernanceClient` (main.go:2261), and `setupMCPRouter()` creates an `*mcp.MCPRouter` (main.go:2469). However, `setupMCPRouter()` never receives the vault client and never sets `VaultClient` in `mcp.Config`. The `mcp.Config` struct has a `VaultClient` field (router.go:73) that supports `IssueBlindFillToken()` and `ZeroizeToolSecrets()`, but it's always nil.

The MCPRouter already handles nil VaultClient gracefully (router.go:387,409 — checks `r.vaultClient != nil` before calling vault methods). Tests confirm graceful degradation (`TestExecuteTool_NilVaultClientSkipsGracefully`).

```go
// setup_mcp.go:44-50 — BEFORE FIX: VaultClient field was never set
// AFTER FIX: setupMCPRouter now takes *vault.VaultGovernanceClient parameter
// and sets VaultClient on mcp.Config:
mcpRouter, err = mcp.New(mcp.Config{
    SkillGate:      gov,
    Provisioner:    prov,
    ConsentManager: consentMgr,
    Auditor:        auditor,
    V6Microkernel:  true,
    VaultClient:    vaultClient,  // ← NOW PASSED
})
```

**Verdict**: `RESOLVED`
**Rationale**: The vaultClient is now passed from `main.go` through `setupMCPRouter()` and set on `mcp.Config.VaultClient`. The MCPRouter can now issue BlindFill tokens and zeroize tool secrets via vault governance when V6Microkernel is enabled. Tool execution is real (Docker exec-based). Feature remains gated (V6Microkernel=false default).
**Action**: Complete. Additionally, tool execution in MCPRouter was upgraded from mock stub to real Docker exec-based execution.

---

## Appendix: Wiring Flow Reference

```
main.go
  ├─ setupVaultClient()           → vaultClient (used for shutdown only)
  ├─ setupMCPRouter(vaultClient)  → mcpRouter  (vaultClient NOW PASSED IN) ✓
  ├─ setupSecretaryServices()     → rolodexStore
  ├─ setupApprovalAndTrust()      → approvalEngine, trustEngine
  │                                    ↓ NOT passed to StepExecutor
  ├─ setupWorkflowEngine()        → orchestrator, integration
  │                                    ↓ ApprovalEngine: nil (hardcoded)
  └─ setupSecretaryCommandHandler() → approvalEngine ✓, trustEngine ✓ (wired here)
```
