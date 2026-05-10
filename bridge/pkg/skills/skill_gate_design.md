# SkillGate Interface and Execution Points

> **Investigation Date:** 2026-05-09
> **Branch:** stabilization-v4
> **Status:** BLOCKING GATE — T7 implementation cannot proceed until reviewed

---

## 1. SkillGate Interface Definition

**Location:** `bridge/pkg/interfaces/skillgate.go`

`SkillGate` is an **interface** (not a concrete type). It is defined in the `interfaces` package and consumed by multiple subsystems.

### Method Signatures

```go
type SkillGate interface {
    // InterceptToolCall intercepts and scrubs PII from tool call arguments before
    // execution. Returns modified ToolCall with redacted arguments.
    InterceptToolCall(ctx context.Context, call *ToolCall) (*ToolCall, error)

    // InterceptPrompt scans and redacts PII from user prompts before they reach
    // the AI model. Returns redacted prompt and PIIMapping for restoration.
    InterceptPrompt(ctx context.Context, prompt string) (string, *PIIMapping, error)

    // RestoreOutput restores redacted PII placeholders in AI output with original
    // values from the PIIMapping. Used when returning results to user's enclave.
    RestoreOutput(ctx context.Context, output string, mapping *PIIMapping) (string, error)

    // ValidateArgs validates tool call arguments for PII violations without modifying
    // the call. Returns list of violations found.
    ValidateArgs(ctx context.Context, toolName string, args map[string]interface{}) ([]PIIViolation, error)
}
```

### Supporting Types (same file)

```go
type ToolCall struct {
    ID        string                 `json:"id"`
    ToolName  string                 `json:"tool_name"`
    Arguments map[string]interface{} `json:"arguments"`
    Priority  int                    `json:"priority,omitempty"`
}

type PIIMapping struct {
    OriginalArgs map[string]interface{} `json:"original_args"`
    RedactedArgs map[string]interface{} `json:"redacted_args"`
    Placeholders map[string]string      `json:"placeholders"`
}

type PIIViolation struct {
    Field       string `json:"field"`
    PatternType string `json:"pattern_type"`
    Message     string `json:"message"`
    Severity    string `json:"severity"` // "low", "medium", "high", "critical"
}

type PIIPattern struct {
    Name        string         `json:"name"`
    Pattern     *regexp.Regexp `json:"-"`
    Replacement string         `json:"replacement"`
    Description string         `json:"description"`
}

type SkillGateConfig struct {
    Enabled        bool     `json:"enabled"`
    StrictMode     bool     `json:"strict_mode"`
    LogViolations  bool     `json:"log_violations"`
    AllowedTools   []string `json:"allowed_tools"`
    BlockedTools   []string `json:"blocked_tools"`
    RedactionStyle string   `json:"redaction_style"` // "placeholder", "hash", "mask"
}
```

**Note:** `SkillGateConfig` exists in the interfaces package but the actual `Governor` uses its own `Config` struct (see Section 2). The `SkillGateConfig` appears to be a planned but not-yet-wired configuration type.

---

## 2. Default Implementation: Governor

**Location:** `bridge/pkg/governor/`

### Governor Struct

```go
type Governor struct {
    scrubber *pii.Scrubber
    logger   *logger.Logger
    config   *Config
    mapping  *interfaces.PIIMapping
    mu       sync.RWMutex
}
```

### Governor Config (separate from SkillGateConfig)

```go
type Config struct {
    LogViolations     bool
    LogMaskedPII      bool
    StrictMode        bool
    AllowPatterns     []string
    BlockPatterns     []string
    UseShadowMapping  bool
    PlaceholderPrefix string
    MaxConcurrentCalls int
    CacheMappings     bool
}
```

### Constructor

```go
func NewGovernor(cfg *Config, log *logger.Logger) *Governor
```

When `cfg == nil`, creates a default config with:
- `LogViolations: true`
- `LogMaskedPII: true`
- `StrictMode: false`
- `UseShadowMapping: true`
- `PlaceholderPrefix: "[REDACTED:"`
- `MaxConcurrentCalls: 100`
- `CacheMappings: true`

### Key Behavior

| Method | Behavior |
|--------|----------|
| `InterceptToolCall` | Iterates string args, runs `pii.Scrub()`, uses Shadow Mapping (SHA256 hash placeholders) or standard redaction. Mutates `call.Arguments` in-place and stores mapping in `g.mapping`. |
| `InterceptPrompt` | Scrubs prompt, creates hash-based placeholders per violation (reverse-order replacement to maintain positions). |
| `RestoreOutput` | Replaces all placeholders in output with originals from `mapping.Placeholders`. |
| `ValidateArgs` | Non-mutating PII detection. Returns `[]PIIViolation` with severity classification. |

### PII Patterns

7 core patterns defined in `interfaces.DefaultPIIPatterns()`:
1. Email addresses
2. US phone numbers
3. US Social Security Numbers
4. Credit card numbers (Visa, MC, Amex, Discover)
5. API keys (sk_, pk_, ai_ prefixes)
6. JWT bearer tokens
7. IPv4 addresses

Additional patterns exist in `pii/scrubber.go` (aws_secret, aws_key_id, github_token, bearer_token, token, secret, password).

### Severity Classification

| Pattern | Severity |
|---------|----------|
| credit_card, aws_secret, aws_key_id, api_key_sk, api_key_pk, api_key_ai | critical |
| ssn, github_token | high |
| email, phone, ip_address, bearer_token, token, secret, password | medium |
| (default) | low |

---

## 3. Where SkillGate is Wired

### 3a. RPC Server (`bridge/pkg/rpc/server.go`)

```go
type Server struct {
    // ...
    skillGate  interfaces.SkillGate  // line 172
    // ...
}

type Config struct {
    // ...
    SkillGate  interfaces.SkillGate  // line 219
    // ...
}
```

In `New()` constructor (line 261): `skillGate: cfg.SkillGate`

**Current wiring status:** In `bridge/cmd/bridge/main.go` line 2633:
```go
rpcCfg.SkillGate = nil     // TODO: wire interfaces.SkillGate when constructed
```
The RPC server receives `SkillGate` as nil. **It is NOT currently wired in the RPC server path.** The RPC server stores it but never uses it directly — execution goes through `SkillManager` (legacy) or `MCPRouter` (v6).

### 3b. MCP Router (`bridge/pkg/mcp/router.go`)

```go
type MCPRouter struct {
    skillGate  interfaces.SkillGate  // line 48
    // ...
}
```

**This is the PRIMARY gating point.** `HandleToolsCall()` at line 198:
```go
sanitizedCall, err := r.skillGate.InterceptToolCall(ctx, toolCall)
```

The MCP router:
1. Receives `tools/call` request
2. Runs CapabilityBroker authorize check (if configured)
3. Calls `skillGate.InterceptToolCall()` — PII scrubbing
4. Spawns ToolSidecar
5. Executes tool with sanitized arguments
6. Audit logs

**Required in constructor:** `cfg.SkillGate == nil` → returns error (line 85-87).

### 3c. SkillExecutor (`bridge/internal/skills/executor.go`)

```go
type SkillExecutor struct {
    skillGate  interfaces.SkillGate  // line 40
    // ...
}
```

In `ExecuteSkill()` at line 100-101:
```go
call := &interfaces.ToolCall{ToolName: skillName, Arguments: params}
_, err := se.skillGate.InterceptToolCall(ctx, call)
```

**Fallback:** If `cfg.SkillGate == nil`, defaults to `governor.NewGovernor(nil, nil)` (line 55-56).

### 3d. PETG Gateway (`bridge/internal/petg/gateway.go`)

```go
type Gateway struct {
    skillGate  interfaces.SkillGate  // line 149
    // ...
}
```

In `ValidateToolCall()` at line 175-177:
```go
call := &interfaces.ToolCall{ToolName: toolName, Arguments: args}
_, err := g.skillGate.InterceptToolCall(ctx, call)
```

**Fallback:** Same as SkillExecutor — defaults to `governor.NewGovernor(nil, nil)`.

### 3e. CapabilityBroker (`bridge/pkg/capability/`)

The broker receives SkillGate via an adapter (`bridge/cmd/bridge/setup_broker.go`):

```go
type skillGateAdapter struct {
    inner interfaces.SkillGate
}

func (a *skillGateAdapter) InterceptToolCall(ctx context.Context, call *capability.ToolCall) (*capability.ToolCall, error)
```

This adapts `interfaces.ToolCall` ↔ `capability.ToolCall` to avoid import cycles. The broker calls `InterceptToolCall` during its `Authorize()` flow.

### 3f. MCP Setup (`bridge/cmd/bridge/setup_mcp.go`)

```go
SkillGate: gov,  // line 46
```

The Governor is passed to the MCP router during setup.

---

## 4. Execution Flow Summary

### Path A: v6 Microkernel (MCPRouter)

```
RPC request (skills.execute)
  → Server.handleSkillsExecute() [methods_skills.go:14]
    → if mcpRouter != nil:
      → MCPRouter.HandleToolsCall() [router.go:159]
        → CapabilityBroker.Authorize() [optional]
        → skillGate.InterceptToolCall() [router.go:198] ← GATING POINT
        → Provisioner.SpawnToolSidecar()
        → Execute in sidecar
    → else (legacy):
      → SkillManager.ExecuteSkill() → SkillExecutor.ExecuteSkill()
        → skillGate.InterceptToolCall() [executor.go:101] ← GATING POINT
```

### Path B: Secretary StepExecutor → Agent Spawn

```
StepExecutor.executeStepWithAgent() [orchestrator_integration.go:825]
  → injectLearnedSkills() [orchestrator_integration.go:1231]
    → LearnedStore.FindForTask() → inject into config
  → factory.Spawn(spawnCtx, spawnReq)
    → Agent runs in container
    → Events tailed → skills extracted → stored
```

**NOTE:** The `injectLearnedSkills()` path does NOT go through SkillGate. It injects learned skill metadata (names, patterns, confidence) into agent spawn config. PII interception happens at the agent's tool call level, not at skill injection time.

### Path C: PETG Gateway

```
Gateway.ValidateToolCall() [gateway.go:173]
  → skillGate.InterceptToolCall() [gateway.go:177] ← GATING POINT
  → CircuitBreaker → Sanitizer → SSRF Checker
```

---

## 5. Interaction with skills.allow / skills.block RPC Methods

### RPC Handlers (`bridge/pkg/rpc/methods_skills.go`)

| RPC Method | Handler | Action |
|------------|---------|--------|
| `skills.allow` | `handleSkillsAllow` | Calls `s.skillMgr.AllowSkill(name)` |
| `skills.block` | `handleSkillsBlock` | Calls `s.skillMgr.BlockSkill(name)` |
| `skills.allowlist_add` | `handleSkillsAllowlistAdd` | Calls `s.skillMgr.AllowIP()` or `s.skillMgr.AllowCIDR()` |
| `skills.allowlist_remove` | `handleSkillsAllowlistRemove` | Stub (no-op, acknowledged) |
| `skills.allowlist_list` | `handleSkillsAllowlistList` | Returns `s.skillMgr.GetAllowlist()` |

### SkillManager Interface (`bridge/pkg/rpc/server.go:87`)

```go
type SkillManager interface {
    ExecuteSkill(ctx context.Context, skillName string, params map[string]interface{}) (*skills.SkillResult, error)
    ListEnabled() []*skills.Skill
    GetSkill(skillName string) (*skills.Skill, bool)
    AllowSkill(skillName string) error
    BlockSkill(skillName string) error
    AllowIP(ip string) error
    AllowCIDR(cidr string) error
    GetAllowlist() ([]string, []string)
    GenerateSchema(skill *skills.Skill) interface{}
}
```

### How Allow/Block Relates to SkillGate

The allow/block lists operate at **two separate layers**:

1. **PolicyEnforcer (SkillExecutor layer):** `PolicyEnforcer.IsAllowed()` checks `allowedSkills` / `blockedSkills` maps. Default is **deny-by-default** — only skills with explicit policy entries pass. `AllowSkill()` adds to allowed map, `BlockSkill()` adds to blocked map.

2. **SkillGate (Governor layer):** `SkillGateConfig` has `AllowedTools` and `BlockedTools` fields, but **these are NOT wired** to the Governor's actual `Config`. The Governor uses `AllowPatterns` / `BlockPatterns` for PII pattern types (not skill names).

**Key finding:** There is a **gap** between the RPC-level allow/block lists (which control whether a skill can execute at all) and the SkillGate (which scrubs PII from arguments). The `SkillGateConfig.AllowedTools`/`BlockedTools` fields exist in the interface definition but are never consumed by the Governor implementation.

---

## 6. Existing Skill System Components

### `bridge/pkg/skills/` Directory

| File | Purpose |
|------|---------|
| `learned_store.go` | `LearnedStore` — SQLite-backed persistence for learned skills. `Save()`, `FindForTask()`, `RecordOutcome()`, `Delete()`, `ListForAgent()`. NOT SQLCipher (no secrets). |
| `learned_store_test.go` | Tests for LearnedStore |
| `extractor.go` | `ExtractFromResult()` — Extracts learned skills from `ExtendedStepResult` using 5 strategies: self-reported candidates, command sequences, file operations, step sequences, checkpoint sequences. |
| `extractor_test.go` | Tests for Extractor |

### `bridge/internal/skills/` Directory (different package!)

| File | Purpose |
|------|---------|
| `executor.go` | `SkillExecutor` — Full PETG pipeline with registry, router, SSRF validator, allowlist, policy enforcer, SkillGate. |
| `executor_*_test.go` | Tests for executor (timeout, authorizer, etc.) |

**Two separate packages:** `bridge/pkg/skills` (public API, learned skills) vs `bridge/internal/skills` (internal executor). The RPC server uses `internal/skills.SkillExecutor` indirectly through the `SkillManager` interface.

---

## 7. Wiring Gaps and Current TODOs

1. **RPC Server SkillGate is nil** (`bridge/cmd/bridge/main.go:2633`):
   ```go
   rpcCfg.SkillGate = nil     // TODO: wire interfaces.SkillGate when constructed
   ```
   The RPC server stores SkillGate but never uses it directly in the legacy path. The v6 path uses MCPRouter's SkillGate.

2. **SkillGateConfig.AllowedTools/BlockedTools not connected** to Governor's actual filtering. The `SkillGateConfig` struct has these fields but Governor uses its own `Config` with `AllowPatterns`/`BlockPatterns` (which are PII pattern names, not skill names).

3. **No Check(skillName) method** on the SkillGate interface. The interface is purely about PII interception/redaction. Skill-level allow/block is handled by `PolicyEnforcer` in `SkillExecutor`, which is a separate mechanism.

4. **PolicyEnforcer is deny-by-default** — `IsAllowed()` returns false for any skill not in the allowed map. Skills are auto-allowed from `Policy` map entries, and can be added via `AllowSkill()` RPC call.

---

## 8. Summary Table: Gating Points

| Location | Component | Method | When | Purpose |
|----------|-----------|--------|------|---------|
| `mcp/router.go:198` | MCPRouter | `skillGate.InterceptToolCall()` | Before tool sidecar spawn | PII scrubbing (v6 path) |
| `internal/skills/executor.go:101` | SkillExecutor | `skillGate.InterceptToolCall()` | Before skill execution | PII scrubbing (legacy path) |
| `internal/petg/gateway.go:177` | PETG Gateway | `skillGate.InterceptToolCall()` | Before circuit breaker | PII scrubbing (gateway path) |
| `capability/broker.go` (via adapter) | CapabilityBroker | `InterceptToolCall()` via adapter | During Authorize() | PII + capability check |
| `internal/skills/executor.go:111` | PolicyEnforcer | `IsAllowed(skillName)` | After PII check | Allow/block by skill name |
| `bridge/cmd/bridge/setup_mcp.go:46` | Setup wiring | `SkillGate: gov` | Initialization | Governor → MCPRouter |
| `bridge/cmd/bridge/main.go:2633` | Main wiring | `rpcCfg.SkillGate = nil` | Initialization | **NOT WIRED** |

---

## 9. Key Findings for T7 Implementation

1. **SkillGate is an interface** (`interfaces.SkillGate`) with 4 methods: `InterceptToolCall`, `InterceptPrompt`, `RestoreOutput`, `ValidateArgs`.

2. **Default implementation is `Governor`** (`bridge/pkg/governor/`) — PII interception engine using Shadow Mapping 2.0.

3. **There is NO `Check(skillName) (allowed bool, reason string)` method** on the current SkillGate interface. The interface is PII-focused, not access-control-focused.

4. **Allow/block by skill name is handled by `PolicyEnforcer`** in `SkillExecutor`, which is separate from SkillGate. The RPC methods `skills.allow`/`skills.block` modify `PolicyEnforcer` maps, not SkillGate.

5. **Skill injection (`injectLearnedSkills`)** happens at `StepExecutor.executeStepWithAgent()` — it adds learned skill metadata to agent spawn config but does NOT gate execution through SkillGate.

6. **The RPC server's `SkillGate` field is currently nil** and unused. All gating goes through MCPRouter (v6) or SkillExecutor (legacy).
