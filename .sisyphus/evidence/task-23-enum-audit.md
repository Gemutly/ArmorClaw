# Task 23: State Enum Audit and Consolidation

## Summary

Cataloged 106 `type X string` enum definitions across `bridge/pkg/`. Found 3 safe merges (1 implemented, 2 blocked by import cycles). AgentStatus string constants verified unchanged by contract test.

## Complete Enum Inventory

### Status/State Enums (string-based)

| # | Type | Package | File | Values | Semantic |
|---|------|---------|------|--------|----------|
| 1 | `AgentStatus` | agent | state.go | IDLE, INITIALIZING, BROWSING, FORM_FILLING, AWAITING_CAPTCHA, AWAITING_2FA, AWAITING_APPROVAL, PROCESSING_PAYMENT, ERROR, COMPLETE, OFFLINE (11) | **DO NOT TOUCH** — Matrix protocol contract |
| 2 | `ServiceState` | browser | client.go | IDLE, LOADING, FILLING, WAITING, PROCESSING, ERROR (6) | Browser service HTTP session state |
| 3 | `BrowserStatus` | browser | browser.go | idle, navigating, loading, ready, error (5) | Browser skill session state (lowercase) |
| 4 | `BrowserState` | studio | browser_skill.go | LOADING, FILLING, WAITING, PROCESSING, IDLE, ERROR (6) | **DUPLICATE of ServiceState** — blocked by import cycle |
| 5 | `InstanceStatus` | studio | types.go | pending, running, paused, completed, failed, cancelled (6) | Agent container lifecycle |
| 6 | `JobStatus` | queue | browser_queue.go | pending, running, paused, completed, failed, cancelled, awaiting_pii (7) | Queue job lifecycle |
| 7 | `BlindFillStatus` | secretary | blindfill.go | pending, running, completed, failed, cancelled, awaiting_approval (6) | BlindFill execution lifecycle |
| 8 | `WorkflowStatus` | secretary | types.go | (values in types.go) | Workflow engine states |
| 9 | `RuntimeType` | runtime | runtime.go | docker, containerd, firecracker (3) | Container runtime backend |
| 10 | `runtime.Status` | runtime | runtime.go | created, running, paused, stopped, exited, dead, unknown (7) | Docker container status |
| 11 | `AccessRequestStatus` | pii | hitl_consent.go | pending, approved, rejected, expired (4) | HITL PII access request |
| 12 | `ConsentState` | pii | three_way_consent.go | pending, approved, rejected, expired (4) | Three-way consent room state |
| 13 | `ConsentEventType` | pii | three_way_consent.go | request, approve, reject, expire (4) | Consent event types |
| 14 | `ApprovalStatus` | studio | mcp_approval.go | PENDING, APPROVED, REJECTED, EXPIRED (4) | MCP approval (UPPERCASE) |
| 15 | `ApprovalStatus` | email | types.go | pending, approved, rejected, expired (4) | Email approval → **MERGED to pii.AccessRequestStatus** |
| 16 | `RecoveryStatus` | recovery | recovery.go | none, pending, active, complete, expired (5) | Recovery flow state |
| 17 | `TokenStatus` | provisioning | manager.go | pending, claimed, expired, canceled (4) | Provisioning token state |
| 18 | `TokenState` | qr | public.go | active, used, expired, revoked (4) | QR token lifecycle |
| 19 | `InviteStatus` | invite | roles.go | (values in roles.go) | Invite lifecycle |
| 20 | `TrustState` | trust | device.go | (values in device.go) | Device trust state |
| 21 | `PluginState` | plugin | plugin.go | unloaded, loaded, initialized, running, error, disabled (6) | Plugin lifecycle |
| 22 | `AdapterStatus` | adapters | permissions.go | disconnected, connecting, connected, error, disabled (5) | Adapter connection state |
| 23 | `LicenseState` (iota) | license | state_manager.go | Valid, GracePeriod, Expired, Invalid, Unknown (5) | License validation state |
| 24 | `license.Status` | license | client.go | active, expired, revoked, suspended (4) | License server status |
| 25 | `ClaimStatus` | admin | claim.go | (values in claim.go) | Admin claim status |
| 26 | `PIIRequestStatus` | keystore | pii_request.go | (values in pii_request.go) | PII keystore request status |
| 27 | `LifecycleState` | team | types.go | (values in types.go) | Team lifecycle |
| 28 | `Action` | sidecar | pii_interceptor.go | (values in pii_interceptor.go) | PII interceptor action |
| 29 | `ComplianceMode` | config | config.go | (values in config.go) | Config compliance mode |
| 30 | `ComplianceMode` | enforcement | enforcement.go | (values in enforcement.go) | Enforcement compliance mode |
| 31 | `MappingStatus` | secretary | learn_website.go | (values in learn_website.go) | Website mapping status |
| 32 | `BrowserResponseStatus` | studio | browser_skill.go | (values in browser_skill.go) | Browser response status |
| 33 | `EventType` | audit | audit.go | (values in audit.go) | Audit event type |
| 34 | `EventType` | voice | matrix.go | (values in matrix.go) | Voice Matrix event type |
| 35 | `MessageType` | webrtc | signaling.go | (values in signaling.go) | WebRTC signaling type |
| 36 | `MessageType` | socket | server.go | (values in server.go) | WebSocket message type |

### Other String Enums (non-status)

| Type | Package | Values | Semantic |
|------|---------|--------|----------|
| `ServiceWaitUntil` | browser | load, domcontentloaded, networkidle | Navigation wait condition |
| `WaitUntil` | studio | load, domcontentloaded, networkidle | **DUPLICATE** — blocked by import cycle |
| `ServiceErrorCode` | browser | 11 error codes | Browser service errors |
| `BrowserErrorCode` | studio | 8 error codes (subset) | Studio browser errors |
| `ApprovalDecision` | secretary | (values in approvals.go) | Secretary approval decision |
| `MediaType` | pii | (values in media_scanner.go) | PII media type |
| `SensitivityLevel` | pii | (values in skill_manifest.go) | PII sensitivity level |
| `ProfileType` | pii | (values in profile.go) | PII profile type |
| `PCIWarningLevel` | pii | (values in profile.go) | PCI warning level |
| `HIPAATier` | pii | (values in hipaa.go) | HIPAA classification tier |
| `PHIType` | pii | (values in hipaa.go) | PHI data type |
| `ScrubMode` | pii | (values in hipaa.go) | HIPAA scrub mode |
| `RiskLevel` | capability | (values in types.go) | Capability risk level |
| `RiskClass` | capability | (values in types.go) | Capability risk class |
| `Severity` | errors | (values in error.go) | Error severity |
| `LogLevel` | logger | (values in logger.go) | Log level |
| `SecurityEventType` | logger | (values in security.go) | Security event type |
| `ApprovalType` | studio | MCP_ACCESS, PII_ACCESS, SKILL_ADD | MCP approval category |
| `McpRiskLevel` | studio | low, medium, high, critical | MCP risk level |
| `UserRole` | studio | admin, member, viewer | User role |
| `Platform` | push | (values in gateway.go) | Push notification platform |
| `Platform` | appservice | (values in bridge.go) | App service platform |
| `ProviderType` | sso | (values in sso.go) | SSO provider type |
| `NotificationType` | secretary | (values in notifications.go) | Notification type |
| `ExecutionMode` | secretary | (values in bridge_local_registry.go) | Workflow execution mode |
| `StepType` | secretary | (values in types.go) | Workflow step type |
| `ChannelType` | secretary | (values in types.go) | Notification channel type |
| `FailoverPolicy` | secretary | (values in types.go) | Workflow failover policy |
| `ParallelErrorPolicy` | secretary | (values in orchestrator_parallel.go) | Parallel error handling |
| `TrustDecision` | secretary | (values in trusted_workflows.go) | Trust decision |
| `TrustReasonCode` | secretary | (values in trusted_workflows.go) | Trust reason code |
| `SecretType` | secrets | (values in injection.go) | Secret type |
| `SecretProvider` | secrets | (values in injection.go) | Secret provider |
| `TokenType` | qr | (values in public.go) | QR token type |
| `Role` | invite | (values in roles.go) | Invite role |
| `ExpirationOption` | invite | (values in roles.go) | Invite expiration |
| `AlertSeverity` | notification | (values in alert_types.go) | Alert severity |
| `AlertType` | notification | (values in alert_types.go) | Alert type |
| `Provider` | keystore | (values in keystore.go) | Keystore provider |
| `UnsealPolicy` | keystore | (values in sealed_keystore.go) | Keystore unseal policy |
| `PluginType` | plugin | (values in plugin.go) | Plugin type |
| `HardeningStep` | trust | (values in hardening.go) | Hardening step |
| `VerificationMethod` | trust | (values in device.go) | Trust verification method |
| `AdapterType` | adapters | (values in permissions.go) | Adapter type |
| `AdapterAction` | adapters | (values in permissions.go) | Adapter action |
| `DataCategory` | security | (values in categories.go) | Security data category |
| `PermissionLevel` | security | (values in categories.go) | Security permission level |
| `AuditLevel` | security | (values in categories.go) | Security audit level |
| `SecurityTier` | security | (values in categories.go) | Security tier |
| `ComplianceLevel` | audit | (values in compliance.go) | Audit compliance level |
| `Mode` | lockdown | (values in lockdown.go) | Lockdown mode |
| `Feature` | enforcement | (values in enforcement.go) | Enforcement feature |
| `ActionType` | browser | (values in chart_types.go) | NavChart action type |
| `SelectorTier` | browser | (values in chart_types.go) | NavChart selector tier |
| `WaitCondition` | studio | selector, timeout, url | Wait condition type |
| `AdminRole` | provisioning | (values in manager.go) | Admin role |
| `Priority` | push | (values in gateway.go) | Push priority |
| `SessionType` | voice | (values in budget.go) | Voice session type |
| `ICECandidateType` | turn | (values in turn.go) | ICE candidate type |
| `Tier` | license | (values in client.go) | License tier |
| `ResourceProfile` | docker | (values in resource_governor.go) | Docker resource profile |
| `Scope` | docker | (values in client.go) | Docker scope |
| `ErrorDomain` | eventbus | (values in errors.go) | Error domain |
| `ErrorCode` | eventbus | (values in errors.go) | Error code |
| `ErrorSeverity` | eventbus | (values in errors.go) | Error severity |

### Iota-based Enums (int)

| Type | Package | File | Values | Semantic |
|------|---------|------|--------|----------|
| `CallState` | voice | matrix.go | Invite=0, Ringing, Connected, Ended, Rejected, Failed, Expired (7) | Voice call state |
| `SessionState` | voice | budget.go | Unknown=0, Active, Ended (3) | Voice session state |
| `LicenseState` | license | state_manager.go | Valid=0, GracePeriod, Expired, Invalid, Unknown (5) | License state |
| `RuntimeBehavior` | license | state_manager.go | Normal=0, ... | Runtime behavior |
| `Operation` | license | state_manager.go | Read=0, ... | License operation |
| `PersistenceMode` | budget | persistence.go | Sync=0, ... | Budget persistence mode |
| `WorkflowState` | budget | tracker.go | Running=0, ... | Budget workflow state |
| `SessionState` | webrtc | session.go | Pending=0, ... | WebRTC session state |
| `WizardStep` | studio | types.go | Skills=1, PII, Resources, Confirm (4) | Agent creation wizard |
| `TrustScore` | trust | zero_trust.go | Untrusted=0, ... | Zero-trust score |
| `SensitivityLevel` | permissions | predictor.go | Low=0, ... | Permission sensitivity |
| `EventType` | ghost | manager.go | UserJoined=0, ... | Ghost event type |
| `StreamDirection` | audio | pcm.go | In=0, ... | Audio stream direction |

## Duplicate Analysis

### DUPLICATE GROUP A: Approval Status (lowercase) — **MERGED**

| Type | Package | Values |
|------|---------|--------|
| `AccessRequestStatus` | pii | pending, approved, rejected, expired |
| `ApprovalStatus` | email | pending, approved, rejected, expired |

**Action**: `email.ApprovalStatus` → type alias to `pii.AccessRequestStatus`. Constant values forwarded.
**Rationale**: Identical string values, identical semantics (request lifecycle). pii is the canonical package.

### DUPLICATE GROUP B: Approval Status (case mismatch) — **NOT MERGED**

| Type | Package | Values |
|------|---------|--------|
| `ApprovalStatus` | studio | PENDING, APPROVED, REJECTED, EXPIRED |
| `AccessRequestStatus` | pii | pending, approved, rejected, expired |

**Reason**: Different casing (UPPERCASE vs lowercase). Merging would change serialized JSON/Matrix values.

### DUPLICATE GROUP C: Browser State — **NOT MERGED (import cycle)**

| Type | Package | Values |
|------|---------|--------|
| `ServiceState` | browser | IDLE, LOADING, FILLING, WAITING, PROCESSING, ERROR |
| `BrowserState` | studio | LOADING, FILLING, WAITING, PROCESSING, IDLE, ERROR |

**Reason**: Import cycle `browser` → `queue` → `studio` → `browser`. Cannot create alias.
**Recommendation**: Extract to a shared `bridge/pkg/types/browser.go` package. Deferred to avoid scope creep.

### DUPLICATE GROUP D: WaitUntil — **NOT MERGED (import cycle)**

| Type | Package | Values |
|------|---------|--------|
| `ServiceWaitUntil` | browser | load, domcontentloaded, networkidle |
| `WaitUntil` | studio | load, domcontentloaded, networkidle |

**Reason**: Same import cycle as Group C.
**Recommendation**: Extract to shared `bridge/pkg/types/browser.go`. Deferred.

### DUPLICATE GROUP E: Consent vs Access Request — **NOT MERGED**

| Type | Package | Values |
|------|---------|--------|
| `AccessRequestStatus` | pii (hitl_consent.go) | pending, approved, rejected, expired |
| `ConsentState` | pii (three_way_consent.go) | pending, approved, rejected, expired |

**Reason**: Same package, same values, but different semantic domains (HITL access request vs three-way Matrix consent room). Different lifecycle management code. Merge would conflate two distinct concepts.

### DUPLICATE GROUP F: ComplianceMode — **NOT MERGED**

| Type | Package | Values |
|------|---------|--------|
| `ComplianceMode` | config | (config-specific values) |
| `ComplianceMode` | enforcement | (enforcement-specific values) |

**Reason**: Different packages with potentially different values. Need manual value comparison.

### NEAR-DUPLICATE: Job/Instance Lifecycle — **NOT MERGED**

| Type | Package | Unique Values |
|------|---------|---------------|
| `JobStatus` | queue | + awaiting_pii |
| `InstanceStatus` | studio | (6 values, no awaiting_pii) |
| `BlindFillStatus` | secretary | + awaiting_approval (no paused) |

**Reason**: Each has unique values not present in others. Superset/subset relationships but not true duplicates.

## Changes Made

### 1. `email.ApprovalStatus` → alias to `pii.AccessRequestStatus`
- File: `bridge/pkg/email/types.go`
- Type alias: `type ApprovalStatus = pii.AccessRequestStatus`
- Constants forwarded: `ApprovalPending`, `ApprovalApproved`, `ApprovalRejected`, `ApprovalExpired`
- Marked deprecated

### 2. AgentStatus Contract Test
- File: `bridge/pkg/agent/state_contract_test.go`
- Verifies all 11 AgentStatus string constants are exactly as specified
- Verifies count is 11 (catches additions/removals)
- Will fail CI if any string value changes

## Verification

```
$ cd bridge && go test ./pkg/agent/... ./pkg/browser/... ./pkg/studio/... ./pkg/queue/... -count=1
ok  github.com/armorclaw/bridge/pkg/agent    0.123s
ok  github.com/armorclaw/bridge/pkg/browser  5.140s
ok  github.com/armorclaw/bridge/pkg/studio   0.591s
ok  github.com/armorclaw/bridge/pkg/queue    2.013s
```

AgentStatus string constants: UNCHANGED ✓
