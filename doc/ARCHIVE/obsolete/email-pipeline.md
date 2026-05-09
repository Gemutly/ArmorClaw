# Sovereign Email Pipeline — Architecture & API Reference

## Overview

The Sovereign Email Pipeline is a zero-trust email processing system for ArmorClaw Bridge. It handles:
- **Inbound**: Postfix → pipe handler → MIME parse → YARA scan → PII mask → event dispatch
- **Outbound**: Agent analysis → HITL approval → PII resolve → Gmail/Outlook/SMTP send

## Architecture

```
External Email → Postfix (Port 25/STARTTLS)
                    ↓
              pipe(8) transport
                    ↓
         cmd/mta-recv (stdin → Unix socket)
                    ↓
         IngestServer (YARA → MIME → PII → Storage → Event)
                    ↓
         EventBus (email.received)
                    ↓
         EmailDispatcher (template lookup → DispatchNow)
                    ↓
         Secretary Workflow (step_1_analyze → step_2_send)
                    ↓
         OutboundExecutor (validate → approval → resolve → send)
                    ↓
         GmailClient / OutlookClient / SMTPClient
```

## Package Structure

| Package | Purpose |
|---------|---------|
| `bridge/pkg/email/` | Core email pipeline (ingest, dispatch, outbound, audit) |
| `bridge/pkg/email/proto/` | Message types (protobuf definitions + Go structs) |
| `bridge/pkg/email/hitl_approval.go` | HITL approval manager for outbound emails |
| `bridge/pkg/email/events.go` | EmailReceivedEvent and event bus definitions |
| `bridge/pkg/pii/masker.go` | PII detection → `{{VAULT:...}}` placeholder masking |
| `bridge/pkg/keystore/oauth.go` | OAuth2 token storage in SQLCipher |
| `bridge/pkg/secretary/bridge_local_registry.go` | Bridge-local execution handler registry |
| `bridge/pkg/rpc/email_approval.go` | RPC handlers for approve_email and deny_email |
| `bridge/cmd/mta-recv/` | Postfix pipe handler binary |
| `deploy/postfix/` | Postfix config, install script, verify script *(planned — not yet implemented)* |

## Configuration

### Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `ARMORCLAW_INGEST_SOCKET` | `/run/armorclaw/email-ingest.sock` | Unix socket for mta-recv → IngestServer |
| `ARMORCLAW_EMAIL_STORAGE` | `/var/lib/armorclaw/email-files/` | Base directory for email file storage |

### Postfix Config

See `deploy/postfix/main.cf` *(planned — not yet implemented)* for full configuration. Key settings:
- `inet_interfaces = all` — accept external connections
- `smtpd_tls_security_level = may` — STARTTLS enabled
- `transport_maps = hash:/etc/postfix/transport` — route to armorclaw pipe
- `maximal_queue_lifetime = 1d` — retry for up to 1 day

## Testing

```bash
# Run email pipeline tests (requires Go)
cd bridge && go test ./pkg/email/... -v

# Run PII masker tests
cd bridge && go test ./pkg/pii/... -v

# Verify Postfix setup (requires Postfix installed)
bash deploy/postfix/verify-setup.sh  # planned — not yet implemented
```

## File Paths

| Path | Purpose |
|------|---------|
| `/var/lib/armorclaw/email-files/emails/{id}/raw.eml` | Raw email storage |
| `/var/lib/armorclaw/email-files/attachments/{id}/` | Email attachments |
| `/var/log/armorclaw/email/YYYY-MM-DD.audit.log` | Audit log |
| `/run/armorclaw/email-ingest.sock` | Unix domain socket (mta-recv ↔ IngestServer) |
| `/usr/local/bin/armorclaw-mta-recv` | Pipe handler binary |
| `/etc/armorclaw/ssl/email.crt` | STARTTLS certificate |

## Security Model

1. **Zero Trust**: All emails scanned by YARA before processing
2. **PII Masking**: SSN, credit cards, phone numbers replaced with `{{VAULT:...}}` placeholders
3. **HITL Approval**: Outbound emails with PII require Matrix approval (300s timeout)
4. **Encrypted Storage**: OAuth tokens encrypted at rest with XChaCha20-Poly1305
5. **Audit Trail**: All pipeline events logged with hashed addresses (no raw PII)
6. **STARTTLS**: Mandatory TLS for Postfix inbound connections

## Email HITL Approval Manager

The `EmailApprovalManager` provides human-in-the-loop approval for outbound emails containing PII. It blocks outbound email processing until the user approves or denies via ArmorChat.

### Core Components

**EmailApprovalManager** (`bridge/pkg/email/hitl_approval.go`):

```go
type EmailApprovalManager struct {
    mu              sync.RWMutex
    pendingRequests map[string]chan ApprovalDecision
    config          EmailApprovalConfig
}

type EmailApprovalConfig struct {
    ApprovalTimeout time.Duration
    Logger          *zap.Logger
    MessageSender   MatrixMessageSender
}

type ApprovalDecision struct {
    Approved     bool
    ApproverID   string
    Timestamp    time.Time
    DenialReason string
}
```

### Approval Flow

1. `RequestApproval(emailID string, email *EmailContent) error`
   - Registers a pending request in `pendingRequests` map
   - Sends `app.armorclaw.email_approval_request` Matrix event to ArmorChat
   - Blocks on a buffered channel until response or timeout

2. `HandleApprovalResponse(emailID string, approved bool, approverID string, reason string)`
   - Delivers user response from ArmorChat via RPC
   - Sends decision through the pending request channel
   - Unblocks the outbound executor to proceed or abort

3. `PendingCount() int`
   - Returns count of currently pending approval requests
   - Used for monitoring and health checks

### Concurrency & Safety

- Thread-safe with `sync.RWMutex` protecting the `pendingRequests` map
- Nil-guard on logger in timeout path prevents panics
- Default timeout: 300 seconds (configurable via `EmailApprovalConfig`)

### Matrix Integration

Approval requests are sent as Matrix events:

```
Event Type: app.armorclaw.email_approval_request
Room: Agent room (private room with user)
Payload: {
  "email_id": "...",
  "subject": "...",
  "pii_count": 3,
  "masked_body": "...{{VAULT:...}}..."
}
```

User responses are delivered via RPC calls to Bridge (`approve_email`, `deny_email`).

## EmailReceivedEvent

The `EmailReceivedEvent` represents a processed inbound email after YARA scanning, MIME parsing, and PII masking. It implements the `eventbus.BridgeEvent` interface for consumption by the dispatcher and secretary workflow.

### Event Structure

**EmailReceivedEvent** (`bridge/pkg/email/events.go`):

```go
type EmailReceivedEvent struct {
    From         string
    To           []string
    Subject      string
    BodyMasked   string
    FileIDs      []string
    PIIFields    []string
    EmailID      string
    Attachments []AttachmentMetadata
    Timestamp    time.Time
}

type AttachmentMetadata struct {
    FileID      string
    Filename    string
    SizeBytes   int64
    ContentType string
}
```

### Event Bus Integration

- **Event Type**: `eventbus.EventTypeEmailReceived` (= `"email.received"`)
- **Interface**: Implements `eventbus.BridgeEvent`
- **Publisher**: IngestServer after processing complete
- **Consumers**: `EmailDispatcher` for template lookup and routing

### Lifecycle

```
1. Postfix → IngestServer (raw email)
2. YARA scan (malware detection)
3. MIME parse (extract headers, attachments)
4. PII mask ({{VAULT:...}} placeholders)
5. Storage (files to /var/lib/armorclaw/email-files/)
6. EmailReceivedEvent published to EventBus
7. EmailDispatcher receives event
8. Template lookup → Secretary workflow dispatch
```

## RPC Handlers

Bridge exposes RPC methods for ArmorChat to deliver approval decisions for outbound emails.

### Email Approval RPCs

**RPC Methods** (`bridge/pkg/rpc/email_approval.go`):

| Method | Parameters | Description |
|--------|------------|-------------|
| `approve_email` | `email_id: string`, `approver_id: string` | Approves a pending outbound email approval request |
| `deny_email` | `email_id: string`, `approver_id: string`, `reason: string` | Denies a pending outbound email approval request |

### Implementation

Both methods call `EmailApprovalManager.HandleApprovalResponse()`:

```go
func (s *BridgeRPCServer) approve_email(params json.RawMessage) (interface{}, error) {
    // Parse email_id, approver_id
    // Call approvalManager.HandleApprovalResponse(emailID, true, approverID, "")
    // Return success response
}

func (s *BridgeRPCServer) deny_email(params json.RawMessage) (interface{}, error) {
    // Parse email_id, approver_id, reason
    // Call approvalManager.HandleApprovalResponse(emailID, false, approverID, reason)
    // Return success response
}
```

### Registration

Handlers are registered in the Bridge RPC server initialization:

```go
rpcServer.RegisterMethod("approve_email", s.approve_email)
rpcServer.RegisterMethod("deny_email", s.deny_email)
```

### Flow

```
ArmorChat (user action)
    ↓
Matrix RPC (encrypted)
    ↓
Bridge RPC Server (approve_email / deny_email)
    ↓
EmailApprovalManager.HandleApprovalResponse()
    ↓
Pending request channel (unblocks)
    ↓
OutboundExecutor (proceeds or aborts)
```

## OAuth Token Storage

OAuth2 tokens for Gmail and Outlook are stored securely in the SQLCipher keystore at rest with XChaCha20-Poly1305 encryption.

### Storage Details

**Location**: `bridge/pkg/keystore/oauth.go`

**Encryption**: XChaCha20-Poly1305 (authenticated encryption)

**Providers Supported**:
- Gmail (Google OAuth2)
- Outlook (Microsoft OAuth2)

### Token Lifecycle

1. **Token Storage**: Tokens encrypted and stored in SQLCipher keystore after OAuth flow
2. **Token Retrieval**: Bridge decrypts tokens on-demand for outbound email sending
3. **Token Refresh**: Bridge automatically refreshes expired tokens using refresh tokens
4. **Token Invalidation**: Tokens are removed when user revokes access via OAuth provider

### Security Properties

- Tokens encrypted at rest with XChaCha20-Poly1305
- No raw token values exposed to agent containers
- Token refresh handled entirely by Bridge (no agent access to refresh tokens)
- Keystore locked with user's passphrase (SQLCipher database key)
- Access logged to audit database

### Token Structure

```go
type OAuthToken struct {
    Provider      string  // "gmail" or "outlook"
    AccessToken   string
    RefreshToken  string
    Expiry        time.Time
    EmailAddress  string
    EncryptedAt   time.Time
}
```

## Bridge-Local Registry

The bridge-local execution registry enables email pipeline steps (send, approval) to run as native Bridge operations without spawning agent containers.

### Registry Integration

**Location**: `bridge/pkg/secretary/bridge_local_registry.go`

The registry maps secretary workflow step types to native Bridge handlers:

```go
type BridgeLocalRegistry struct {
    handlers map[string]BridgeLocalHandler
}

func (r *BridgeLocalRegistry) RegisterHandler(stepType string, handler BridgeLocalHandler) {
    r.handlers[stepType] = handler
}
```

### Email Handlers Registered

| Step Type | Handler | Description |
|-----------|---------|-------------|
| `email_send` | `OutboundExecutor` | Validates, resolves PII, sends via Gmail/Outlook/SMTP |
| `email_approval` | `EmailApprovalManager` | Blocks until user approves via ArmorChat |

### Benefits

- **Performance**: No container spawn overhead for native Bridge operations
- **Security**: Sensitive operations (PII resolution, token access) stay in Bridge
- **Simplicity**: Email pipeline steps run as native Go code, not containers
- **Audit**: Native Bridge operations are fully audited

### Workflow Integration

When the secretary workflow executes an email step:

```
1. Secretary Workflow Engine reads step
2. Step type: "email_send" or "email_approval"
3. Check BridgeLocalRegistry for handler
4. If found: execute handler directly in Bridge (no container)
5. If not found: spawn agent container for step execution
```
