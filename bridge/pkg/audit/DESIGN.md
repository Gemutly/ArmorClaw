# AuditLog Interface & Backing Implementation Design

**Status**: BLOCKING GATE — T3 implementation cannot proceed until reviewed
**Date**: 2026-05-09
**Branch**: stabilization-v4

---

## 1. rpc.Config.AuditLog Field Definition

In `bridge/pkg/rpc/server.go:227`:

```go
type Config struct {
    // ...
    AuditLog         *audit.AuditLog    // line 227
    // ...
}
```

**The field is a concrete pointer `*audit.AuditLog`, not an interface.**

The `Server` struct stores it identically at line 189:

```go
auditLog          *audit.AuditLog
```

### Current Wiring Gap

At `bridge/cmd/bridge/main.go:2630`:

```go
rpcCfg.AuditLog = nil // TODO: wire audit.AuditLog when constructed
```

The RPC server receives this nil value and stores it but never calls any methods on it. No RPC handlers currently reference `s.auditLog`.

---

## 2. audit.AuditLog — Method Signatures

Defined in `bridge/pkg/audit/audit.go`. `AuditLog` is a **concrete struct** (not an interface).

### Constructor

```go
func NewAuditLog(cfg Config) (*AuditLog, error)
```

### Configuration

```go
type Config struct {
    Path   string  // Default: "/var/lib/armorclaw/audit.db"
    MaxLen int     // Default: 10000
}

func DefaultConfig() Config
```

### Methods

| Method | Signature | Description |
|--------|-----------|-------------|
| `Log` | `(al *AuditLog) Log(entry Entry) error` | Append an entry. Auto-timestamps if zero. Trims to maxLen. Persists to file. |
| `LogEvent` | `(al *AuditLog) LogEvent(eventType EventType, sessionID string, roomID string, userID string, details interface{}) error` | Convenience: builds Entry and calls Log. |
| `Query` | `(al *AuditLog) Query(params QueryParams) ([]Entry, error)` | Reverse-chronological query with filters. Limit default 100, max 1000. |
| `Count` | `(al *AuditLog) Count() int` | Returns total entry count. |
| `Clear` | `(al *AuditLog) Clear() error` | Wipes all entries and saves. |
| `ExportJSON` | `(al *AuditLog) ExportJSON() ([]byte, error)` | Marshals all entries to JSON. |
| `ImportJSON` | `(al *AuditLog) ImportJSON(data []byte) error` | Replaces entries from JSON and saves. |

### Core Types

```go
type EventType string   // 35+ event constants defined (call_created, pii_access_*, sidecar_*, keystore_*, etc.)

type Entry struct {
    Timestamp time.Time   `json:"timestamp"`
    EventType EventType   `json:"event_type"`
    SessionID string      `json:"session_id"`
    RoomID    string      `json:"room_id"`
    UserID    string      `json:"user_id"`
    Details   interface{} `json:"details,omitempty"`
}

type QueryParams struct {
    Limit     int
    EventType EventType
    SessionID string
    RoomID    string
    Since     time.Time
}
```

---

## 3. Existing Package Inventory — `bridge/pkg/audit/`

### Files

| File | Purpose |
|------|---------|
| `audit.go` | Core `AuditLog` struct — JSON-lines file-backed event store |
| `compliance.go` | `ComplianceAuditLog` — enterprise-grade with HMAC hash chain, GDPR redaction, CSV/JSON export, retention purge with tombstones |
| `tamper_evident.go` | `TamperEvidentLog` — hash-chain verified, structured entries with Actor/Resource/ComplianceFlags |
| `audit_helper.go` | `CriticalOperationLogger` — high-level convenience wrapper around `TamperEvidentLog`. 30+ domain-specific methods. Also contains global singleton (`SetGlobalAuditLogger`/`GetGlobalAuditLogger`) and `SensitiveFieldDenylist`. |
| `lineage.go` | `LineageTracker` — artifact lineage (source → transformation → output). File-backed JSON. |
| `denylist.go` | `AuditDenylist` — field names that must never appear in audit log entries |
| `compliance_test.go` | 14 tests for ComplianceAuditLog |
| `lineage_test.go` | Tests for LineageTracker |
| `tamper_evident_test.go` | Tests for TamperEvidentLog |

### Three Distinct Audit Types

| Type | Backing | Hash Chain | Entry Model | Used By |
|------|---------|------------|-------------|---------|
| `AuditLog` | JSON-lines file (`audit.db`) | No | `Entry` (flat: event_type, session_id, room_id, user_id, details) | MCP router, sidecar audit client |
| `ComplianceAuditLog` | JSON-lines file (`compliance-audit.db`) | HMAC-SHA256 | `ComplianceEntry` (extended: source, IP, user_agent, action, resource, status, reason) | Standalone, no current consumers |
| `TamperEvidentLog` | In-memory only (no file persistence) | SHA256 | `TamperEvidentEntry` (structured: Actor, Resource, ComplianceFlags, sequence numbers) | CriticalOperationLogger → 15+ packages |

### CriticalOperationLogger Method Catalog

`CriticalOperationLogger` wraps `*TamperEvidentLog` and provides these domain-specific methods:

| Method | Consumers |
|--------|-----------|
| `LogContainerStart(ctx, containerID, image, keyID, sessionID)` | Docker client |
| `LogContainerStop(ctx, containerID, reason, exitCode)` | Docker client |
| `LogContainerError(ctx, containerID, errorMsg, errorCode)` | Docker client |
| `LogKeyAccess(ctx, keyID, userID, operation, success)` | Keystore |
| `LogKeyCreated(ctx, keyID, provider, userID)` | Keystore |
| `LogKeyDeleted(ctx, keyID, userID)` | Keystore |
| `LogSecretInjection(ctx, containerID, keyID, success)` | Secrets |
| `LogSecretCleanup(ctx, containerID, success)` | Secrets |
| `LogConfigurationChange(ctx, userID, section, key, oldValue, newValue)` | Config |
| `LogAuthenticationEvent(ctx, userID, method, success, ipAddress)` | Auth |
| `LogSecurityEvent(ctx, eventType, severity, details)` | Trust, enforcement |
| `LogBudgetEvent(ctx, sessionID, tokensUsed, tokensLimit, exceeded)` | Budget |
| `LogPHIAccess(ctx, userID, resourceType, resourceID, action)` | PII |
| `LogProfileStored(ctx, profileID, profileType)` | PII profiles |
| `LogProfileAccess(ctx, profileID, operation, success)` | PII profiles |
| `LogProfileDeleted(ctx, profileID)` | PII profiles |
| `LogPIIAccessRequest(ctx, requestID, skillID, profileID, requestedFields)` | PII/HITL |
| `LogPIIAccessGranted(ctx, requestID, skillID, userID, approvedFields)` | PII/HITL |
| `LogPIIAccessRejected(ctx, requestID, skillID, userID, reason)` | PII/HITL |
| `LogPIIAccessExpired(ctx, requestID, skillID)` | PII/HITL |
| `LogPIIInjected(ctx, containerID, skillID, fieldsInjected, method)` | PII/BlindFill |
| `LogKeystoreUnseal(ctx, identity, success)` | Keystore |
| `LogKeystoreSeal(ctx, identity, reason)` | Keystore |
| `LogKeystoreExtendSession(ctx, identity)` | Keystore |
| `LogBackupCreate(ctx, userID, backupID)` | E2EE backup |
| `LogBackupDelete(ctx, userID, backupID)` | E2EE backup |
| `LogVoiceStartSession(ctx, sessionID, provider)` | Voice |
| `LogVoiceStopSession(ctx, sessionID)` | Voice |

---

## 4. audit.db Storage Model

**Despite the `.db` extension, there is no SQLite database.** Both `AuditLog` and `ComplianceAuditLog` persist as JSON-lines files:

- `AuditLog`: Loads entire file via `os.ReadFile`, unmarshals `[]Entry` from JSON array, writes back via `json.MarshalIndent` (entire array rewrite on every `Log()` call). Default path: `/var/lib/armorclaw/audit.db`.
- `ComplianceAuditLog`: Streams entries via `json.NewEncoder`/`json.NewDecoder` (one JSON object per line). Default path: `/var/lib/armorclaw/compliance-audit.db`.
- `TamperEvidentLog`: **In-memory only** — no file persistence at all. Data lost on restart.
- `LineageTracker`: Separate JSON file, full array rewrite.

The architecture doc (`doc/architecture.md:309`) states "All operations logged in Bridge `audit.db`" referencing the Rust sidecar, which confirms the intended use of the `AuditLog` type for sidecar operation auditing.

---

## 5. Backing Implementation Decision

### Decision: **Option B — Existing `AuditLog` satisfies `rpc.Config.AuditLog` directly**

### Rationale

1. **Type match is exact.** `rpc.Config.AuditLog` is `*audit.AuditLog` — a concrete pointer. No adapter needed. The `AuditLog` struct from `audit.go` is the direct, intended type.

2. **Wiring path is straightforward.** One call at main.go:2630:
   ```go
   auditor, auditErr := audit.NewAuditLog(audit.DefaultConfig())
   if auditErr != nil {
       log.Printf("Warning: audit log init failed (%v)", auditErr)
   } else {
       rpcCfg.AuditLog = auditor
   }
   ```
   This pattern already exists in `setup_mcp.go:40` where the MCP router creates its own `AuditLog` instance. The RPC server just needs its own instance (or the same one).

3. **The "real" audit system runs in parallel.** `CriticalOperationLogger` → `TamperEvidentLog` is already wired across 15+ packages (keystore, docker, PII, secrets, trust, auth, enforcement, voice, crypto). This operates independently from `rpc.Config.AuditLog`. The RPC server's `AuditLog` field serves a different purpose: logging RPC-level events (sidecar operations, capability requests, governance mutations) into the queryable `audit.db` file.

4. **Why not switch to TamperEvidentLog?** That would require changing the `rpc.Config` field type from `*audit.AuditLog` to `*audit.TamperEvidentLog`, which is a breaking API change across the RPC package. The brownfield rule (AGENTS.md) says: preserve existing architecture unless explicitly asked.

5. **Why not ComplianceAuditLog?** Same reason — wrong type, no current consumers, and it's a separate system for enterprise compliance reporting.

6. **Why not Option C (composite adapter)?** There's no need. The two audit paths serve distinct purposes:
   - `rpc.Config.AuditLog` → queryable event log for sidecar/RPC-level operations (read by MCP router, sidecar audit client, and potentially the Admin Panel audit page)
   - `CriticalOperationLogger` → tamper-evident critical operations log (security-focused, in-memory, used by internal packages)

### Config Additions Needed

No new config fields required. The existing `audit.DefaultConfig()` provides sensible defaults:
- Path: `/var/lib/armorclaw/audit.db`
- MaxLen: 10000

If configuration is desired, a `config.toml` section could be added:
```toml
[audit]
path = "/var/lib/armorclaw/audit.db"
max_entries = 10000
```

But this is optional — the defaults are production-ready.

### Implementation Sketch (for T3 reference)

```
main.go wiring:
  1. Create AuditLog via NewAuditLog(DefaultConfig())
  2. Assign to rpcCfg.AuditLog
  3. Optionally create a shared instance for MCP router

RPC handler usage:
  - s.auditLog.LogEvent(audit.EventSidecarHealthCheck, sessionID, roomID, userID, details)
  - s.auditLog.Query(params) for admin panel audit page
```

---

## 6. Relationship Between Audit Types

```
rpc.Config.AuditLog (*audit.AuditLog)
  ├── Used by: RPC server (future), MCP router, sidecar audit client
  ├── Storage: JSON-lines file (audit.db)
  └── Purpose: Queryable event log for operations

audit.CriticalOperationLogger (wraps *TamperEvidentLog)
  ├── Used by: keystore, docker, PII, secrets, trust, auth, enforcement, voice, crypto
  ├── Storage: In-memory only (data lost on restart)
  └── Purpose: Tamper-evident logging for security-critical operations

audit.ComplianceAuditLog
  ├── Used by: None currently (enterprise feature)
  ├── Storage: JSON-lines file (compliance-audit.db)
  └── Purpose: Enterprise compliance reporting with GDPR redaction

audit.LineageTracker
  ├── Used by: Artifact governance
  ├── Storage: JSON file
  └── Purpose: Artifact lineage tracking (source → transform → output)
```

---

## 7. Open Questions for Reviewer

1. **Should `TamperEvidentLog` gain file persistence?** Currently in-memory only, which means critical operation audit data is lost on Bridge restart. This is a significant gap but out of scope for this design doc.

2. **Should `rpc.Config.AuditLog` and `CriticalOperationLogger` share the same backing store?** Currently they're independent. Unification would require either (a) making `AuditLog` wrap `TamperEvidentLog` or (b) adding a composite adapter. This would be a larger refactor.

3. **Should the `.db` extension be renamed to `.jsonl`?** The current naming is misleading — these are JSON-lines files, not SQLite databases. However, renaming would break existing deployments and is out of scope.
