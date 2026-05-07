# ArmorChat Android — Email Pipeline Integration Specification

## Overview

This document specifies the Matrix event types, payload schemas, and UI requirements for ArmorChat Android to integrate with the Sovereign Email Pipeline. **No Kotlin code changes are included** — this is an informational specification for the Android team.

---

## Matrix Event Types

### 1. Email Approval Request

Sent by the Bridge when an outbound email requires HITL (Human-in-the-Loop) approval.

**Event Type**: `app.armorclaw.email_approval_request`
**Classification**: Transient message event (NOT state event)

```json
{
  "type": "app.armorclaw.email_approval_request",
  "content": {
    "approval_id": "approval_1713312000000",
    "email_id": "a1b2c3d4e5f6",
    "step_id": "step_2_send",
    "to": "recipient@example.com",
    "subject": "[masked]",
    "body_preview": "Hello, I wanted to follow up on...",
    "pii_fields": ["ssn_0", "phone_1"],
    "pii_field_types": ["ssn", "phone"],
    "sensitivity_badges": [
      {"type": "ssn", "level": "high", "label": "SSN"},
      {"type": "phone", "level": "medium", "label": "Phone"}
    ],
    "timeout_seconds": 300,
    "requested_at": 1713312000
  }
}
```

### 2. Email Approval Response

Sent by ArmorChat when the user approves or rejects.

**Event Type**: `app.armorclaw.email_approval_response`
**Classification**: Transient message event (NOT state event)

> **Transport Note**: While this event type is defined for Matrix, the Bridge actually processes approval responses via JSON-RPC methods (`approve_email`, `deny_email`). The Matrix event type serves as the conceptual schema for the ArmorChat UI layer.

```json
{
  "type": "app.armorclaw.email_approval_response",
  "content": {
    "approval_id": "approval_1713312000000",
    "email_id": "a1b2c3d4e5f6",
    "step_id": "step_2_send",
    "approved": true,
    "approved_by": "@user:matrix.example.com",
    "approved_fields": ["ssn_0"],
    "denied_fields": [],
    "responded_at": 1713312015
  }
}
```

### 3. Email Received

Sent by the Bridge when an inbound email is processed and ready for display.

**Event Type**: `app.armorclaw.email.received`
**Classification**: Transient message event (NOT state event)

```json
{
  "type": "app.armorclaw.email.received",
  "content": {
    "from": "sender@example.com",
    "to": "user@armorclaw.com",
    "subject": "Meeting Tomorrow",
    "body_masked": "Hi, just wanted to confirm... {{VAULT:ssn_0}}...",
    "email_id": "a1b2c3d4e5f6",
    "file_ids": ["file_001", "file_002"],
    "pii_fields": ["ssn_0", "phone_1"],
    "attachments": [
      {
        "filename": "report.pdf",
        "content_type": "application/pdf",
        "size": 1024
      }
    ]
  }
}
```

**Fields:**
- `body_masked` — PII replaced with `{{VAULT:...}}` placeholders
- `pii_fields` — list of detected PII field IDs for approval tracking
- `file_ids` — references to stored raw email and attachment files
- `attachments` — list of attachment metadata (filename, content_type, size)

**Published after:** YARA scan, MIME parse, and PII masking complete

---

## UI Requirements

### PiiApprovalCard Component

Reuse the existing `PiiApprovalCard.kt` pattern with email-specific extensions:

1. **Card Layout**: Same as existing PII approval card (batched fields, sensitivity badges)
2. **Email Context**: Show recipient (masked), subject (masked), body preview (first 150 chars)
3. **maxLines**: Body preview limited to 5 lines
4. **Buttons**: Approve / Reject (same as existing PII approval flow)

### Biometric Model

- **Device-level KeyGuard**: Use Android KeyGuard confirmation (device PIN/fingerprint)
- **NOT per-click prompt**: Single biometric unlock per session for email approvals
- **Fallback**: PIN entry if biometric unavailable

### Notification

- Push notification via existing Matrix notification channel
- Title: "Email Approval Required"
- Body: "Response to [recipient] needs your approval"
- Action: Deep link to approval card in conversation (`armorclaw://email/approve/<approval_id>`, handled by `DeepLinkHandler` → `Route.EmailApproval`)

> **v0.7.0**: Deep link routing for email approvals is now implemented. `DeepLinkHandler.kt` resolves `armorclaw://email/approve/{id}` to `Route.EmailApproval(approvalId)`. Cold-start and warm-resume are handled via `MainActivity.onNewIntent()` + `LaunchedEffect`.

---

## Event Flow

```
Bridge                          Matrix                          ArmorChat
  |                               |                               |
  |=== INBOUND PATH ===           |                               |
  |                               |                               |
  |-- email.received ----------->|                               |
  |                               |---- push notification ------>|
  |                               |                               |
  |                               |<--- read/dismiss ------------|
  |                               |                               |
  |=== OUTBOUND PATH ===          |                               |
  |                               |                               |
  |-- email_approval_request --->|                               |
  |                               |---- push notification ------>|
  |                               |                               |
  |                               |<--- approval_response --------|
  |<-- approval_response ---------|                               |
  |                               |                               |
  |-- email sent (audit logged) --|                               |
```

---

## Configuration

| Parameter | Default | Description |
|-----------|---------|-------------|
| `approval_timeout_seconds` | 300 | Time before approval request expires |
| `max_body_preview_chars` | 150 | Characters shown in approval card |
| `pii_masking_enabled` | true | Whether PII is masked in previews |
| `biometric_required` | false | Whether biometric is required for approval |

---

## Compatibility Notes

- These event types extend the existing `app.armorclaw.*` namespace
- Events are transient (not state) — they do not persist in room state
- The `sensitivity_badges` field reuses the existing `SensitivityBadge` model from `PiiApprovalCard.kt`
- No changes to existing Matrix SDK or event handling — new event types are handled via the existing message pipeline

---

## Implementation Status

| Component | Status | Notes |
|-----------|--------|-------|
| EmailApprovalCard composable | Implemented | `applications/ArmorChat/.../EmailApprovalCard.kt` |
| approve_email RPC | Implemented | `bridge/pkg/rpc/email_approval.go` |
| deny_email RPC | Implemented | `bridge/pkg/rpc/email_approval.go` |
| EmailApprovalManager | Implemented | `bridge/pkg/email/hitl_approval.go` |
| EmailReceivedEvent | Implemented | `bridge/pkg/email/events.go` |
| Matrix event routing | Implemented | `bridge/internal/adapter/matrix.go` handles `app.armorclaw.email_*` events |
| OAuth token storage | Implemented | `bridge/pkg/keystore/oauth.go` (XChaCha20-Poly1305 encrypted) |
| Bridge-local registry | Implemented | `bridge/pkg/secretary/bridge_local_registry.go` |
| Deep link routing (v0.7.0) | Implemented | `DeepLinkHandler.kt` → `Route.EmailApproval`, cold-start + warm-resume |
