# Task 13: Studio Methods Investigation Report

**Date:** 2026-05-14  
**Scope:** Classification of all `studio.*` RPC methods — StudioMethods list vs handler registration  
**Status:** Investigation complete, no code modified

---

## Executive Summary

| Metric | Count |
|--------|-------|
| Methods in `StudioMethods` list (integration.go:259-286) | 22 |
| Methods in `RPCHandler.Handle()` switch (rpc.go:126-196) | 23 |
| Methods registered in handler map (server.go:1272-1273) | 2 |
| Methods requiring delegation gate (delegation_gate.go:11-22) | 5 |
| **Net unregistered methods** | **20** |

**Key findings:**
- `studio.deploy` is registered in the handler map but NOT in `StudioMethods` — it routes through `handleStudio()` which dispatches to `HandleRPCMethod()` using the actual method name from the request. This is a catch-all pattern.
- `studio.stats` is registered in the handler map AND routed via `handleStudioStats()` directly (server.go:918-936), which calls `s.studio.HandleRPCMethod("studio.stats", ...)`.
- `studio.get_stats` is in StudioMethods but the RPC switch handles `studio.stats` — **naming mismatch**.
- `studio.get_instance` is in the RPC switch but NOT in StudioMethods — **missing from list**.
- ALL 23 methods in the RPC switch have real, substantive implementations (no stubs/placeholders).

---

## Classification Table

### Legend
- **REAL** — Full implementation, should be registered
- **STALE** — Dead/placeholder, no real implementation
- **ANDROID** — Client expects it but bridge has spec mismatch
- **FUTURE** — Deliberately excluded from current surface

| # | Method | In StudioMethods | In RPC Switch | Registered | Delegation Gate | Classification | Evidence | Recommendation |
|---|--------|:-:|:-:|:-:|:-:|:-:|----------|----------------|
| 1 | `studio.list_skills` | Yes | Yes (rpc.go:128) | **No** | No | **REAL** | Full impl: calls `skillRegistry.List()`, returns skills+count (rpc.go:207-224) | Register via `handleStudio` |
| 2 | `studio.get_skill` | Yes | Yes (rpc.go:130) | **No** | No | **REAL** | Full impl: calls `skillRegistry.Get()`, validates ID (rpc.go:231-247) | Register via `handleStudio` |
| 3 | `studio.register_skill` | Yes | Yes (rpc.go:132) | **No** | No | **REAL** | Full impl: creates Skill struct, calls `skillRegistry.Register()`, audit log (rpc.go:259-282) | Register via `handleStudio` |
| 4 | `studio.list_pii` | Yes | Yes (rpc.go:136) | **No** | No | **REAL** | Full impl: calls `piiRegistry.List()`, returns fields+count (rpc.go:293-310) | Register via `handleStudio` |
| 5 | `studio.get_pii` | Yes | Yes (rpc.go:138) | **No** | No | **REAL** | Full impl: calls `piiRegistry.Get()`, validates ID (rpc.go:317-333) | Register via `handleStudio` |
| 6 | `studio.register_pii` | Yes | Yes (rpc.go:140) | **No** | No | **REAL** | Full impl: creates PIIField struct, calls `piiRegistry.Register()`, auto-sets requires_approval for high/critical (rpc.go:345-368) | Register via `handleStudio` |
| 7 | `studio.list_profiles` | Yes | Yes (rpc.go:170) | **No** | No | **REAL** | Full impl: calls `profileManager.List()`, returns profiles map (rpc.go:845-850) | Register via `handleStudio` |
| 8 | `studio.create_agent` | Yes | Yes (rpc.go:144) | **No** | **Yes** | **REAL** | Full impl: validates skills, PII, resource tier, checks duplicates, stores definition (rpc.go:383-452). Delegation gate required (delegation_gate.go:13). | Register via `handleStudio` |
| 9 | `studio.update_agent` | Yes | Yes (rpc.go:150) | **No** | **Yes** | **REAL** | Full impl: gets existing def, validates updates, stores (rpc.go:512-575). Delegation gate required (delegation_gate.go:14). | Register via `handleStudio` |
| 10 | `studio.delete_agent` | Yes | Yes (rpc.go:152) | **No** | **Yes** | **REAL** | Full impl: checks running instances, deletes def (rpc.go:582-611). Delegation gate required (delegation_gate.go:15). | Register via `handleStudio` |
| 11 | `studio.list_agents` | Yes | Yes (rpc.go:148) | **No** | No | **REAL** | Full impl: calls `store.ListDefinitions()`, returns agents+count (rpc.go:482-499) | Register via `handleStudio` |
| 12 | `studio.get_agent` | Yes | Yes (rpc.go:146) | **No** | No | **REAL** | Full impl: calls `store.GetDefinition()`, validates ID (rpc.go:459-475) | Register via `handleStudio` |
| 13 | `studio.spawn_agent` | Yes | Yes (rpc.go:156) | **No** | **Yes** | **REAL** | Full impl: gets def, checks PII approval, creates instance, calls `factory.Spawn()` if available (rpc.go:623-724). Delegation gate required (delegation_gate.go:16). | Register via `handleStudio` |
| 14 | `studio.list_instances` | Yes | Yes (rpc.go:160) | **No** | No | **REAL** | Full impl: calls `store.ListInstances()`, returns instances+count (rpc.go:761-778) | Register via `handleStudio` |
| 15 | `studio.stop_instance` | Yes | Yes (rpc.go:162) | **No** | **Yes** | **REAL** | Full impl: gets instance, calls `factory.Stop()`, updates status (rpc.go:785-826). Delegation gate required (delegation_gate.go:17). | Register via `handleStudio` |
| 16 | `studio.get_stats` | Yes | **No** (switch uses `studio.stats`) | **No** | No | **ANDROID** | StudioMethods lists `studio.get_stats` but RPC switch (rpc.go:166) handles `studio.stats`. Name mismatch. Client calling `studio.get_stats` gets "Unknown method" error from the switch default. | Fix: either rename in StudioMethods to `studio.stats`, or add `studio.get_stats` case to switch |
| 17 | `studio.list_mcps` | Yes | Yes (rpc.go:174) | **No** | No | **REAL** | Full impl: calls `mcpRegistry.ListMcps()` with filter, returns mcps+count (rpc.go:863-886) | Register via `handleStudio` |
| 18 | `studio.get_mcp` | Yes | Yes (rpc.go:176) | **No** | No | **REAL** | Full impl: calls `mcpRegistry.GetMcp()`, validates ID (rpc.go:893-909) | Register via `handleStudio` |
| 19 | `studio.get_mcp_warning` | Yes | Yes (rpc.go:178) | **No** | No | **REAL** | Full impl: calls `approvalManager.GetMcpRiskAssessment()`, returns MCP + risk + audit (rpc.go:916-944) | Register via `handleStudio` |
| 20 | `studio.request_mcp_approval` | Yes | Yes (rpc.go:182) | **No** | No | **REAL** | Full impl: validates MCP exists, creates approval request via `approvalManager.CreateApprovalRequest()`, notifies admins (rpc.go:957-999) | Register via `handleStudio` |
| 21 | `studio.list_pending_approvals` | Yes | Yes (rpc.go:184) | **No** | No | **REAL** | Full impl: calls `approvalManager.ListPendingApprovals()` (rpc.go:1004-1014) | Register via `handleStudio` |
| 22 | `studio.list_my_approvals` | Yes | Yes (rpc.go:186) | **No** | No | **REAL** | Full impl: validates UserID, calls `approvalManager.ListUserApprovals()` (rpc.go:1019-1033) | Register via `handleStudio` |
| 23 | `studio.approve_mcp_request` | Yes | Yes (rpc.go:188) | **No** | No | **REAL** | Full impl: validates IDs, calls `approvalManager.ApproveRequest()`, audit log (rpc.go:1041-1067) | Register via `handleStudio` |
| 24 | `studio.reject_mcp_request` | Yes | Yes (rpc.go:190) | **No** | No | **REAL** | Full impl: validates IDs, calls `approvalManager.RejectRequest()`, audit log (rpc.go:1075-1101) | Register via `handleStudio` |
| 25 | `studio.deploy` | **No** | **No** (not in RPC switch) | **Yes** | No | **REAL** (registered) | Registered in handler map (server.go:1272) as `s.handleStudio`. The `handleStudio` function (studio.go:25) is a generic dispatcher that delegates ANY `studio.*` method to `s.studio.HandleRPCMethod()`. Since `studio.deploy` is not in the RPC switch (rpc.go:126-196), calling it returns "Unknown method: studio.deploy". **This is a ghost registration — it's registered but leads to a method-not-found error.** | Remove from handler map OR add to RPC switch + StudioMethods |
| 26 | `studio.stats` | **No** | Yes (rpc.go:166) | **Yes** | No | **REAL** (registered) | Registered via `handleStudioStats` (server.go:1273, impl at 917-936). Implementation calls `s.studio.HandleRPCMethod("studio.stats", req.Params)` then falls back to basic stats. **Note:** This bypasses the delegation gate since it doesn't go through `handleStudio()`. | Keep registered; add to StudioMethods list |
| 27 | `studio.get_instance` | **No** | Yes (rpc.go:158) | **No** | No | **REAL** | Full impl: calls `store.GetInstance()`, returns instance+definition (rpc.go:731-753). **Missing from StudioMethods list** and not registered in handler map. | Add to StudioMethods; register via `handleStudio` |

---

## Detailed Analysis

### 1. Naming Mismatch: `studio.get_stats` vs `studio.stats`

**The Bug:** StudioMethods (integration.go:275) lists `"studio.get_stats"`. The RPC switch (rpc.go:166) handles `"studio.stats"`. The handler map (server.go:1273) registers `"studio.stats"`.

**Impact:** A client calling `studio.get_stats` over RPC gets routed to `handleStudio` → `HandleRPCMethod` → switch default → "Unknown method: studio.get_stats". The method is unreachable.

**Fix:** Either:
- (A) Change StudioMethods entry from `studio.get_stats` to `studio.stats` (align with switch + registration)
- (B) Add `studio.get_stats` as an alias case in the RPC switch pointing to `handleStats`

Recommendation: **Option A** — the switch and registration both use `studio.stats`, fix the list.

### 2. Ghost Registration: `studio.deploy`

**The Bug:** `studio.deploy` is registered in the handler map (server.go:1272) pointing to `handleStudio`. When invoked, `handleStudio` passes `req.Method` (which is `"studio.deploy"`) to `s.studio.HandleRPCMethod()`. The RPC switch (rpc.go:126-196) has no case for `"studio.deploy"`, so it hits the default: `"Unknown method: studio.deploy"`.

**Impact:** Calling `studio.deploy` returns a method-not-found error despite being registered. This is a dead registration.

**Possible original intent:** May have been intended as a convenience alias for `studio.spawn_agent` or as a separate deployment step. No implementation exists.

**Fix:** Either:
- (A) Remove `"studio.deploy"` from handler map (clean dead registration)
- (B) Add `studio.deploy` to the RPC switch with a real implementation
- (C) Add `studio.deploy` as an alias for `studio.spawn_agent` in the RPC switch

### 3. Missing from StudioMethods: `studio.get_instance`

The RPC switch (rpc.go:158-159) handles `studio.get_instance` with a full implementation. However, it's not in the `StudioMethods` list and not registered in the handler map. This method is completely unreachable from external callers.

### 4. Missing from StudioMethods: `studio.stats`

Registered in the handler map and implemented in the RPC switch, but absent from the `StudioMethods` list. This is the canonical stats method.

### 5. Registration Architecture

The current registration pattern for studio methods is inconsistent:

- **`studio.deploy`** → registered directly → routes through `handleStudio` → **dead end** (not in switch)
- **`studio.stats`** → registered directly → routes through `handleStudioStats` → calls `HandleRPCMethod("studio.stats")` directly → **works** but bypasses delegation gate
- **All other 22 methods** → **NOT registered** → completely unreachable from RPC clients

The `handleStudio` function (studio.go:25-66) is already designed to be the universal dispatcher for all `studio.*` methods. It:
1. Checks studio initialization
2. Applies delegation gate for 5 sensitive methods
3. Delegates to `s.studio.HandleRPCMethod()`
4. Converts response types

The correct fix (T14) is to register all 22 StudioMethods + `studio.get_instance` via `handleStudio` in the handler map.

### 6. Delegation Gate Coverage

The 5 methods requiring delegation gate (delegation_gate.go:11-22):

| Method | Gate Applied In | Currently Registered | Gap |
|--------|:-:|:-:|:-:|
| `studio.create_agent` | `handleStudio` | No | Unreachable, so gate irrelevant |
| `studio.update_agent` | `handleStudio` | No | Unreachable, so gate irrelevant |
| `studio.delete_agent` | `handleStudio` | No | Unreachable, so gate irrelevant |
| `studio.spawn_agent` | `handleStudio` | No | Unreachable, so gate irrelevant |
| `studio.stop_instance` | `handleStudio` | No | Unreachable, so gate irrelevant |

**Note:** `studio.stats` is registered via `handleStudioStats` which does NOT apply delegation gate. However, `studio.stats` is a read-only operation so this is correct behavior.

---

## Summary Recommendations for T14

### Methods to Register in Handler Map (20 methods)

Register all via `s.handleStudio` (the existing generic dispatcher):

```
studio.list_skills
studio.get_skill
studio.register_skill
studio.list_pii
studio.get_pii
studio.register_pii
studio.list_profiles
studio.create_agent        ← delegation gate applies
studio.update_agent        ← delegation gate applies
studio.delete_agent        ← delegation gate applies
studio.get_agent
studio.list_agents
studio.spawn_agent         ← delegation gate applies
studio.list_instances
studio.stop_instance       ← delegation gate applies
studio.get_instance        ← missing from StudioMethods, add to list
studio.list_mcps
studio.get_mcp
studio.get_mcp_warning
studio.request_mcp_approval
studio.list_pending_approvals
studio.list_my_approvals
studio.approve_mcp_request
studio.reject_mcp_request
```

### Fix Required: StudioMethods List

| Change | Method | Action |
|--------|--------|--------|
| **Rename** | `studio.get_stats` → `studio.stats` | Align with RPC switch + registration |
| **Add** | `studio.get_instance` | Present in switch but missing from list |
| **Add** | `studio.stats` | Already registered, missing from list |
| **Decide** | `studio.deploy` | Either remove from handler map or add implementation |

### Ghost Registration Decision: `studio.deploy`

**Recommendation:** Remove from handler map. It has no implementation in the RPC switch, no entry in StudioMethods, and no clear purpose distinct from `studio.spawn_agent`. If deployment is needed in the future, add it properly with implementation + StudioMethods entry.

---

## File References

| File | Lines | Purpose |
|------|-------|---------|
| `bridge/pkg/studio/integration.go` | 259-286 | StudioMethods list (22 entries) |
| `bridge/pkg/studio/integration.go` | 223-230 | HandleRPCMethod dispatcher |
| `bridge/pkg/studio/rpc.go` | 126-196 | RPCHandler.Handle() switch (23 cases + default) |
| `bridge/pkg/studio/rpc.go` | 207-1101 | All handler implementations (real code) |
| `bridge/pkg/rpc/server.go` | 1272-1273 | Handler map (2 studio entries) |
| `bridge/pkg/rpc/server.go` | 917-936 | handleStudioStats implementation |
| `bridge/pkg/rpc/studio.go` | 25-66 | handleStudio generic dispatcher |
| `bridge/pkg/rpc/studio.go` | 11-22 | requiresDelegationGate (5 methods) |
| `bridge/pkg/rpc/delegation_gate.go` | 16-33 | RequireDelegationReady implementation |
| `bridge/pkg/studio/registry.go` | 1-256 | SkillRegistry, PIIRegistry, ProfileManager |
| `bridge/pkg/studio/mcp_approval.go` | 1-530 | MCP registry, approval manager, audit logging |
