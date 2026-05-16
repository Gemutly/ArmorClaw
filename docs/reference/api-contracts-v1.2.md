# ArmorClaw v1.2 Mobile API Contracts

> **Version**: v1.2 (Stabilization)
> **Last Updated**: 2026-05-16
> **Audience**: ArmorChat Android client developers

---

## Transport

All RPC calls use JSON-RPC 2.0 over HTTPS (port 8443) or Unix socket (`/run/armorclaw/bridge.sock`).

```
POST /api  (HTTPS)
Body: {"jsonrpc":"2.0","id":1,"method":"<method>","params":{...}}
Auth:  Authorization: Bearer <admin_token>
```

---

## 1. Workflow Management

### 1.1 secretary.create_workflow

Create a new workflow from a template.

```json
// Request
{
  "jsonrpc": "2.0",
  "id": 1,
  "method": "secretary.create_workflow",
  "params": {
    "template_id": "tmpl-abc123",
    "name": "Pay Utility Bill",
    "description": "Monthly utility payment",
    "variables": {"amount": "150.00", "vendor": "Acme Power"},
    "created_by": "@alice:matrix.org"
  }
}

// Response
{
  "jsonrpc": "2.0",
  "id": 1,
  "result": {
    "id": "wf-xyz789",
    "template_id": "tmpl-abc123",
    "name": "Pay Utility Bill",
    "status": "pending",
    "current_step": 0,
    "created_by": "@alice:matrix.org",
    "created_at": "2026-05-16T12:00:00Z"
  }
}
```

### 1.2 secretary.start_workflow

Start executing a created workflow.

```json
// Request
{
  "jsonrpc": "2.0",
  "id": 2,
  "method": "secretary.start_workflow",
  "params": {
    "workflow_id": "wf-xyz789"
  }
}

// Response
{
  "jsonrpc": "2.0",
  "id": 2,
  "result": {
    "id": "wf-xyz789",
    "status": "running",
    "current_step": 0,
    "started_at": "2026-05-16T12:01:00Z"
  }
}
```

### 1.3 secretary.get_workflow

Poll workflow status.

```json
// Request
{
  "jsonrpc": "2.0",
  "id": 3,
  "method": "secretary.get_workflow",
  "params": {
    "workflow_id": "wf-xyz789"
  }
}

// Response
{
  "jsonrpc": "2.0",
  "id": 3,
  "result": {
    "id": "wf-xyz789",
    "template_id": "tmpl-abc123",
    "name": "Pay Utility Bill",
    "status": "running",
    "current_step": 2,
    "steps": [
      {"step_id": "s1", "name": "Navigate", "type": "action", "status": "completed"},
      {"step_id": "s2", "name": "Fill Form", "type": "action", "status": "running"},
      {"step_id": "s3", "name": "Submit", "type": "action", "status": "pending"}
    ],
    "agent_ids": ["agent-container-1"],
    "started_at": "2026-05-16T12:01:00Z",
    "created_by": "@alice:matrix.org",
    "room_id": "!room:matrix.org"
  }
}
```

### 1.4 secretary.cancel_workflow

```json
// Request
{
  "jsonrpc": "2.0",
  "id": 4,
  "method": "secretary.cancel_workflow",
  "params": {
    "workflow_id": "wf-xyz789",
    "reason": "User requested cancellation"
  }
}
```

### 1.5 secretary.advance_workflow

Advance a blocked workflow past a blocker step.

```json
// Request
{
  "jsonrpc": "2.0",
  "id": 5,
  "method": "secretary.advance_workflow",
  "params": {
    "workflow_id": "wf-xyz789",
    "step_id": "s2",
    "action": "skip"
  }
}
```

### 1.6 secretary.is_running

```json
// Response: {"running": true, "active_count": 3}
```

### 1.7 secretary.get_active_count

```json
// Response: {"count": 3}
```

### 1.8 secretary.shutdown

Graceful shutdown of the secretary subsystem.

---

## 2. Template Management

### 2.1 secretary.create_template

```json
// Request
{
  "jsonrpc": "2.0",
  "id": 10,
  "method": "secretary.create_template",
  "params": {
    "name": "Bill Payment",
    "description": "Pay a bill via browser automation",
    "steps": [
      {"step_id": "s1", "order": 0, "type": "action", "name": "Navigate", "config": {"url": "{{vendor_url}}"}},
      {"step_id": "s2", "order": 1, "type": "action", "name": "Fill", "config": {"fields": [{"selector": "#amount", "value_ref": "amount"}]}}
    ],
    "variables": {"amount": {"type": "string"}, "vendor_url": {"type": "string"}},
    "pii_refs": ["payment.card_number"],
    "created_by": "@alice:matrix.org"
  }
}
```

### 2.2 secretary.list_templates
### 2.3 secretary.get_template
### 2.4 secretary.update_template
### 2.5 secretary.delete_template

All follow standard CRUD patterns with `template_id` parameter.

---

## 3. Task Management (Simplified)

### 3.1 task.create
### 3.2 task.list
### 3.3 task.cancel
### 3.4 task.get

Quick-create tasks without templates:

```json
// task.create
{
  "jsonrpc": "2.0",
  "id": 20,
  "method": "task.create",
  "params": {
    "description": "Research best pizza near Times Square",
    "skills": ["web_browsing"]
  }
}
```

---

## 4. Progress Events (Matrix Stream)

Workflow progress is delivered via Matrix `m.notice` messages in the workflow's dedicated room. ArmorChat consumes these via the Matrix `/sync` endpoint.

### 4.1 Event Types

| Event | Type | Content Shape |
|-------|------|---------------|
| Workflow started | `workflow.started` | `{workflow_id, template_id, name}` |
| Step progress | `workflow.step_progress` | `{workflow_id, step_id, step_name, progress: 0.0-1.0}` |
| Step error | `workflow.step_error` | `{workflow_id, step_id, error, recoverable}` |
| Workflow completed | `workflow.completed` | `{workflow_id, result}` |
| Workflow failed | `workflow.failed` | `{workflow_id, step_id, error, recoverable}` |
| Workflow cancelled | `workflow.cancelled` | `{workflow_id, reason}` |
| Workflow blocked | `workflow.blocked` | `{workflow_id, reason, message}` |
| Agent status | `com.armorclaw.agent.status` | `{agent_id, state, message}` |

### 4.2 WorkflowEvent Shape

```json
{
  "workflow_id": "wf-xyz789",
  "template_id": "tmpl-abc123",
  "status": "running",
  "step_id": "s2",
  "step_name": "Fill Form",
  "progress": 0.65,
  "timestamp": "2026-05-16T12:02:30Z"
}
```

---

## 5. HITL Approval Contracts

### 5.1 Approval Request Flow

When a step requires PII or a sensitive action, the approval engine evaluates policies:

1. **Policy evaluation**: Check auto-approve conditions, PII field lists, conditions
2. **If auto-approve**: Proceed without user interaction
3. **If HITL required**: Send approval request to ArmorChat via Matrix
4. **User decides**: Approve or deny via `resolve_blocker` RPC

### 5.2 ApprovalRequest Shape

```json
{
  "id": "apr-123",
  "workflow_id": "wf-xyz789",
  "step_id": "s2",
  "fields": ["payment.card_number", "payment.cvv"],
  "status": "pending",
  "created_at": "2026-05-16T12:02:00Z",
  "expires_at": "2026-05-16T12:12:00Z"
}
```

### 5.3 resolve_blocker RPC

```json
// Request
{
  "jsonrpc": "2.0",
  "id": 30,
  "method": "resolve_blocker",
  "params": {
    "workflow_id": "wf-xyz789",
    "step_id": "s2",
    "decision": "approved",
    "reason": "User approved payment"
  }
}

// Response
{
  "jsonrpc": "2.0",
  "id": 30,
  "result": {
    "delivered": true
  }
}
```

### 5.4 Approval Decision Types

- `"approved"` — proceed with the step
- `"denied"` — abort the step, mark workflow as blocked/failed

---

## 6. Risk Classification Contract

### 6.1 Action Taxonomy

The risk classifier maps 16 actions + 6 wildcards to risk levels:

| Risk Level | Actions |
|------------|---------|
| **Critical** | `payment.submit`, `pii.submit`, `credentials.submit` |
| **High** | `browser.navigate`, `browser.fill`, `file.upload`, `file.download` |
| **Medium** | `browser.click`, `browser.extract`, `email.send` |
| **Low** | `browser.wait`, `browser.screenshot`, `file.read`, `clipboard.copy` |

### 6.2 Classification Result

```json
{
  "action": "payment.submit",
  "risk_class": "financial",
  "risk_level": "critical"
}
```

---

## 7. Trust Policy Contract

### 7.1 TrustedWorkflowPolicy Shape

```json
{
  "id": "tp-abc123",
  "name": "Acme Bill Payment",
  "description": "Trusted for monthly utility payments",
  "scope": {
    "domains": ["acme-power.com"],
    "actions": ["browser.navigate", "browser.fill", "payment.submit"],
    "fields": ["payment.card_number"]
  },
  "time_restrictions": {
    "allowed_hours": [8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20],
    "allowed_days": [1, 2, 3, 4, 5]
  },
  "max_amount": 500.00,
  "auto_approve": false,
  "status": "active",
  "created_by": "@alice:matrix.org",
  "created_at": "2026-05-16T10:00:00Z",
  "expires_at": "2027-05-16T10:00:00Z",
  "execution_count": 5,
  "last_executed_at": "2026-05-15T08:30:00Z"
}
```

### 7.2 Trust Evaluation Result

```json
{
  "decision": "allowed",
  "confidence": 0.92,
  "reason": "Policy 'Acme Bill Payment' matches scope and time restrictions",
  "reason_code": "policy_match",
  "matching_policy_id": "tp-abc123",
  "scope_match": {
    "domain_match": true,
    "action_match": true,
    "field_match": true
  }
}
```

### 7.3 Trust Decision Types

- `"allowed"` — proceed without approval
- `"conditional"` — proceed with conditions
- `"denied"` — block the action
- `"requires_approval"` — escalate to HITL

---

## 8. Browser Integration Contract

### 8.1 Browser Commands

| Command | Parameters | Response |
|---------|-----------|----------|
| `navigate` | `{url, wait_until}` | `{status, title, url}` |
| `fill` | `{selector, value, value_ref}` | `{success: true}` |
| `click` | `{selector}` | `{success: true}` |
| `extract` | `{selector, attribute}` | `{value}` |
| `wait` | `{selector, timeout_ms}` | `{found: true}` |

### 8.2 Browser Step Execution

Workflow steps with `type: "action"` and browser config are executed via `BrowserIntegration.ExecuteStep()`, which returns:

```json
{
  "success": true,
  "data": {"url": "https://acme-power.com/pay", "title": "Payment Portal"},
  "screenshots": ["step-2-before.png", "step-2-after.png"]
}
```

---

## 9. Email Pipeline Contract

### 9.1 Email Storage

| Operation | Method | Parameters |
|-----------|--------|-----------|
| Store email | `StoreEmail` | `emailID, rawEmail` |
| Store attachment | `StoreAttachment` | `emailID, filename, content` |
| Get attachment | `GetAttachment` | `fileID` |
| Store attachment text | `StoreAttachmentText` | `emailID, filename, text` |
| Delete email | `DeleteEmail` | `emailID` |

### 9.2 Email Approval RPC

Email approval requests follow the same HITL pattern as workflow approvals, dispatched through `resolve_blocker`.

---

## 10. Sidecar Contract

### 10.1 Tool Sidecar Lifecycle

| Operation | Method | Parameters |
|-----------|--------|-----------|
| Spawn | `SpawnToolSidecar` | `skillName, sessionID` |
| Execute | `ExecuteInSidecar` | `containerID, toolName, arguments` |
| Stop | `StopToolSidecar` | `containerID` |

### 10.2 Sidecar Response

```json
{
  "container_id": "sidecar-abc123",
  "tool_name": "document_extraction",
  "result": {"text": "...", "pages": 5},
  "duration_ms": 1200
}
```

---

## 11. Artifact Protocol (v1.2 New)

### 11.1 secretary.artifact_upload

```json
// Request
{
  "jsonrpc": "2.0",
  "id": 40,
  "method": "secretary.artifact_upload",
  "params": {
    "owner": "@alice:matrix.org",
    "workflow_id": "wf-xyz789",
    "step_id": "s3",
    "metadata": {
      "mime_type": "application/pdf",
      "filename": "receipt.pdf",
      "tags": ["receipt", "utility"],
      "source": "browser",
      "size_bytes": 24576
    },
    "checksum": "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
  }
}

// Response
{
  "jsonrpc": "2.0",
  "id": 40,
  "result": {
    "id": "art-def456",
    "status": "pending",
    "version": "1.0"
  }
}
```

### 11.2 secretary.artifact_download

```json
// Request
{
  "jsonrpc": "2.0",
  "id": 41,
  "method": "secretary.artifact_download",
  "params": {
    "artifact_id": "art-def456"
  }
}

// Response: full ArtifactEnvelope object (requires owner match)
```

### 11.3 secretary.artifact_list

```json
// Request
{
  "jsonrpc": "2.0",
  "id": 42,
  "method": "secretary.artifact_list",
  "params": {
    "owner": "@alice:matrix.org",
    "workflow_id": "wf-xyz789",
    "status": "completed"
  }
}

// Response
{
  "jsonrpc": "2.0",
  "id": 42,
  "result": {
    "artifacts": [...],
    "count": 3
  }
}
```

### 11.4 secretary.artifact_update_status

```json
// Request
{
  "jsonrpc": "2.0",
  "id": 43,
  "method": "secretary.artifact_update_status",
  "params": {
    "artifact_id": "art-def456",
    "status": "completed"
  }
}
```

### 11.5 Artifact Status State Machine

```
pending → processing → completed → expired
pending → failed → pending (retry)
processing → failed
```

### 11.6 Security Rules

- **Owner/workflow binding required** — reject without both
- **Authorization check** — download/update requires owner match
- **Filename sanitization** — reject path traversal (`..`, `/`, `\`)
- **MIME type validation** — must match standard format
- **SHA-256 integrity** — checksum is integrity-only, NOT authentication
- **TTL expiration** — default 7 days, auto-cleanup

---

## 12. Notification Contract

Notifications are dispatched to Matrix rooms for workflow lifecycle events:

| Notification | Trigger | Recipient |
|-------------|---------|-----------|
| Workflow started | Workflow enters `running` | Room creator |
| Step progress | Step progress updates | Room creator |
| Workflow completed | Workflow reaches `completed` | Room creator |
| Workflow failed | Workflow reaches `failed` | Room creator |
| Workflow cancelled | Workflow reaches `cancelled` | Room creator |
| Approval required | HITL step encountered | Room creator |
| Approval approved | User approves | Requester |
| Approval denied | User denies | Requester |

---

## 13. Registered RPC Methods (Complete List)

### Secretary Methods (13)
1. `secretary.create_workflow`
2. `secretary.start_workflow`
3. `secretary.get_workflow`
4. `secretary.cancel_workflow`
5. `secretary.create_template`
6. `secretary.get_template`
7. `secretary.list_templates`
8. `secretary.update_template`
9. `secretary.delete_template`
10. `secretary.advance_workflow`
11. `secretary.is_running`
12. `secretary.get_active_count`
13. `secretary.shutdown`

### Artifact Methods (4, v1.2 new)
14. `secretary.artifact_upload`
15. `secretary.artifact_download`
16. `secretary.artifact_list`
17. `secretary.artifact_update_status`

### Task Methods (4)
18. `task.create`
19. `task.list`
20. `task.cancel`
21. `task.get`

### Blocker Method (1)
22. `resolve_blocker`

**Total: 22 v1.2 documented methods** (existing 109 bridge methods remain unchanged)
