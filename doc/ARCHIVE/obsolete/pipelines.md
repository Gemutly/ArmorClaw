# ArmorClaw Data Pipelines
<!-- Source: email-pipeline.md, email-android-integration.md, sidecar-pipeline.md, voice-stack.md -->

## Table of Contents

- [Email Pipeline](#email-pipeline)
  - [Overview](#overview)
  - [Architecture](#architecture)
  - [Package Structure](#package-structure)
  - [Configuration](#configuration)
    - [Environment Variables](#environment-variables)
    - [Postfix Config](#postfix-config)
  - [Testing](#testing)
  - [File Paths](#file-paths)
  - [Security Model](#security-model)
  - [Email HITL Approval Manager](#email-hitl-approval-manager)
    - [Core Components](#core-components)
    - [Approval Flow](#approval-flow)
    - [Concurrency & Safety](#concurrency--safety)
    - [Matrix Integration](#matrix-integration)
  - [EmailReceivedEvent](#emailreceivedevent)
    - [Event Structure](#event-structure)
    - [Event Bus Integration](#event-bus-integration)
    - [Lifecycle](#lifecycle)
  - [RPC Handlers](#rpc-handlers)
    - [Email Approval RPCs](#email-approval-rpcs)
    - [Implementation](#implementation)
    - [Registration](#registration)
    - [Flow](#flow)
  - [OAuth Token Storage](#oauth-token-storage)
    - [Storage Details](#storage-details)
    - [Token Lifecycle](#token-lifecycle)
    - [Security Properties](#security-properties)
    - [Token Structure](#token-structure)
  - [Bridge-Local Registry](#bridge-local-registry)
    - [Registry Integration](#registry-integration)
    - [Email Handlers Registered](#email-handlers-registered)
    - [Benefits](#benefits)
    - [Workflow Integration](#workflow-integration)
- [Email Android Integration](#email-android-integration)
  - [Overview](#overview-1)
  - [Matrix Event Types](#matrix-event-types)
    - [Email Approval Request](#email-approval-request)
    - [Email Approval Response](#email-approval-response)
    - [Email Received](#email-received)
  - [UI Requirements](#ui-requirements)
    - [PiiApprovalCard Component](#piiapprovalcard-component)
    - [Biometric Model](#biometric-model)
    - [Notification](#notification)
  - [Event Flow](#event-flow)
  - [Configuration](#configuration-1)
  - [Compatibility Notes](#compatibility-notes)
  - [Implementation Status](#implementation-status)
- [Document Sidecar Pipeline](#document-sidecar-pipeline)
  - [Overview](#overview-2)
  - [Architecture](#architecture-2)
    - [Data Flow](#data-flow)
  - [Key Packages](#key-packages)
    - [Rust Sidecar](#rust-sidecar-sidecar)
      - [Connectors](#connectors-sidecarsrcconnectors)
      - [Document Processing](#document-processing-sidecarsrcdocument)
      - [ShadowMap PII Redaction (XLSX)](#shadowmap-pii-redaction-xlsx)
      - [ProcessDocument Convert](#processdocument-convert)
      - [Encryption](#encryption-sidecarsrcencryption)
      - [Provenance](#provenance-sidecarsrcprovenance)
      - [Split-Storage Manager](#split-storage-manager-sidecarsrcsplit_storage)
      - [gRPC Service](#grpc-service-sidecarsrcgrpc)
    - [Go Client](#go-client-bridgepkgsidecar)
      - [client.go](#clientgo)
      - [audit_client.go](#audit_clientgo)
      - [pii_interceptor.go](#pii_interceptorgo)
      - [queue.go](#queuego)
      - [token.go](#tokengo)
      - [version.go](#versiongo)
      - [sidecar.proto](#sidecarproto)
    - [YARA Scanner](#yara-scanner-bridgepkgyara)
      - [scanner.go](#scannergo)
  - [Configuration](#configuration-3)
    - [Rust Sidecar Environment Variables](#rust-sidecar-environment-variables)
    - [Go Client Configuration](#go-client-configuration)
    - [SidecarConfig Struct (Rust)](#sidecarconfig-struct-rust)
  - [Integration Points](#integration-points)
    - [Bridge to Sidecar](#bridge-to-sidecar)
    - [YARA Integration](#yara-integration)
    - [Split-Storage RAG Pipeline](#split-storage-rag-pipeline)
    - [Matrix / ArmorChat](#matrix--armorchat)
    - [Jetski Browser Sidecar](#jetski-browser-sidecar)
    - [Python MarkItDown Sidecar](#python-markitdown-sidecar-sidecar-python)
      - [Architecture](#architecture-3)
      - [Routing Logic (3-Layer)](#routing-logic-3-layer)
      - [Key Design Decisions](#key-design-decisions)
      - [Python Server (sidecar-python/worker.py)](#python-server-sidecar-pythonworkerpy)
      - [Token Interceptor (sidecar-python/interceptor.py)](#token-interceptor-sidecar-pythoninterceptorpy)
      - [Supported Formats](#supported-formats)
      - [PPTX Migration to Rust (v0.6.0)](#pptx-migration-to-rust-v060)
    - [Java Apache POI Sidecar](#java-apache-poi-sidecar-sidecar-java)
      - [Architecture](#architecture-4)
      - [Routing Logic](#routing-logic)
      - [Supported Formats](#supported-formats-1)
      - [Key Design Decisions](#key-design-decisions-1)
      - [Test Coverage](#test-coverage)
      - [Running Tests](#running-tests)
      - [Docker Deployment (sidecar-java)](#docker-deployment-sidecar-java)
      - [Docker Deployment (sidecar-py)](#docker-deployment-sidecar-py)
      - [Test Coverage (Combined)](#test-coverage-combined)
      - [Running Tests (Combined)](#running-tests-combined)
      - [Go Client Routing](#go-client-routing-bridgepkgsidecaroffice_clientgo)
  - [References](#references)
- [Voice Stack](#voice-stack)
  - [Current State](#current-state)
    - [What Exists](#what-exists)
    - [What Is Missing](#what-is-missing)
    - [Runtime Reality](#runtime-reality)
    - [Interface Discrepancy](#interface-discrepancy)
    - [E2E Test Expectations](#e2e-test-expectations)
  - [Overview](#overview-3)
  - [Architecture](#architecture-5)
  - [Key Packages](#key-packages-1)
    - [bridge/pkg/audio/](#bridgepkgaudio)
    - [bridge/pkg/voice/](#bridgepkgvoice)
    - [bridge/pkg/webrtc/](#bridgepkgwebrtc)
    - [bridge/pkg/turn/](#bridgepkgturn)
    - [Speech Services (bridge/pkg/voice/)](#speech-services-bridgepkgvoice)
      - [STTService (stt_service.go)](#sttservice-stt_servicego)
      - [TTSService (tts_service.go)](#ttsservice-tts_servicego)
      - [VADService (vad_service.go)](#vadservice-vad_servicego)
      - [Design Pattern](#design-pattern)
  - [Configuration](#configuration-4)
    - [Environment Variables](#environment-variables-1)
    - [Budget Configuration](#budget-configuration)
    - [Audio Configuration](#audio-configuration)
  - [Integration Points](#integration-points-1)
    - [Matrix Rooms](#matrix-rooms)
    - [Budget System](#budget-system)
    - [Agent Runtime](#agent-runtime)
    - [TURN Infrastructure](#turn-infrastructure)

---

## Email Pipeline

### Overview

The Sovereign Email Pipeline is a zero-trust email processing system for ArmorClaw Bridge. It handles:
- **Inbound**: Postfix → pipe handler → MIME parse → YARA scan → PII mask → event dispatch
- **Outbound**: Agent analysis → HITL approval → PII resolve → Gmail/Outlook/SMTP send

### Architecture

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

### Package Structure

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

### Configuration

#### Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `ARMORCLAW_INGEST_SOCKET` | `/run/armorclaw/email-ingest.sock` | Unix socket for mta-recv → IngestServer |
| `ARMORCLAW_EMAIL_STORAGE` | `/var/lib/armorclaw/email-files/` | Base directory for email file storage |

#### Postfix Config

See `deploy/postfix/main.cf` *(planned — not yet implemented)* for full configuration. Key settings:
- `inet_interfaces = all` — accept external connections
- `smtpd_tls_security_level = may` — STARTTLS enabled
- `transport_maps = hash:/etc/postfix/transport` — route to armorclaw pipe
- `maximal_queue_lifetime = 1d` — retry for up to 1 day

### Testing

```bash
# Run email pipeline tests (requires Go)
cd bridge && go test ./pkg/email/... -v

# Run PII masker tests
cd bridge && go test ./pkg/pii/... -v

# Verify Postfix setup (requires Postfix installed)
bash deploy/postfix/verify-setup.sh  # planned — not yet implemented
```

### File Paths

| Path | Purpose |
|------|---------|
| `/var/lib/armorclaw/email-files/emails/{id}/raw.eml` | Raw email storage |
| `/var/lib/armorclaw/email-files/attachments/{id}/` | Email attachments |
| `/var/log/armorclaw/email/YYYY-MM-DD.audit.log` | Audit log |
| `/run/armorclaw/email-ingest.sock` | Unix domain socket (mta-recv ↔ IngestServer) |
| `/usr/local/bin/armorclaw-mta-recv` | Pipe handler binary |
| `/etc/armorclaw/ssl/email.crt` | STARTTLS certificate |

### Security Model

1. **Zero Trust**: All emails scanned by YARA before processing
2. **PII Masking**: SSN, credit cards, phone numbers replaced with `{{VAULT:...}}` placeholders
3. **HITL Approval**: Outbound emails with PII require Matrix approval (300s timeout)
4. **Encrypted Storage**: OAuth tokens encrypted at rest with XChaCha20-Poly1305
5. **Audit Trail**: All pipeline events logged with hashed addresses (no raw PII)
6. **STARTTLS**: Mandatory TLS for Postfix inbound connections

### Email HITL Approval Manager

The `EmailApprovalManager` provides human-in-the-loop approval for outbound emails containing PII. It blocks outbound email processing until the user approves or denies via ArmorChat.

#### Core Components

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

#### Approval Flow

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

#### Concurrency & Safety

- Thread-safe with `sync.RWMutex` protecting the `pendingRequests` map
- Nil-guard on logger in timeout path prevents panics
- Default timeout: 300 seconds (configurable via `EmailApprovalConfig`)

#### Matrix Integration

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

### EmailReceivedEvent

The `EmailReceivedEvent` represents a processed inbound email after YARA scanning, MIME parsing, and PII masking. It implements the `eventbus.BridgeEvent` interface for consumption by the dispatcher and secretary workflow.

#### Event Structure

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

#### Event Bus Integration

- **Event Type**: `eventbus.EventTypeEmailReceived` (= `"email.received"`)
- **Interface**: Implements `eventbus.BridgeEvent`
- **Publisher**: IngestServer after processing complete
- **Consumers**: `EmailDispatcher` for template lookup and routing

#### Lifecycle

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

### RPC Handlers

Bridge exposes RPC methods for ArmorChat to deliver approval decisions for outbound emails.

#### Email Approval RPCs

**RPC Methods** (`bridge/pkg/rpc/email_approval.go`):

| Method | Parameters | Description |
|--------|------------|-------------|
| `approve_email` | `email_id: string`, `approver_id: string` | Approves a pending outbound email approval request |
| `deny_email` | `email_id: string`, `approver_id: string`, `reason: string` | Denies a pending outbound email approval request |

#### Implementation

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

#### Registration

Handlers are registered in the Bridge RPC server initialization:

```go
rpcServer.RegisterMethod("approve_email", s.approve_email)
rpcServer.RegisterMethod("deny_email", s.deny_email)
```

#### Flow

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

### OAuth Token Storage

OAuth2 tokens for Gmail and Outlook are stored securely in the SQLCipher keystore at rest with XChaCha20-Poly1305 encryption.

#### Storage Details

**Location**: `bridge/pkg/keystore/oauth.go`

**Encryption**: XChaCha20-Poly1305 (authenticated encryption)

**Providers Supported**:
- Gmail (Google OAuth2)
- Outlook (Microsoft OAuth2)

#### Token Lifecycle

1. **Token Storage**: Tokens encrypted and stored in SQLCipher keystore after OAuth flow
2. **Token Retrieval**: Bridge decrypts tokens on-demand for outbound email sending
3. **Token Refresh**: Bridge automatically refreshes expired tokens using refresh tokens
4. **Token Invalidation**: Tokens are removed when user revokes access via OAuth provider

#### Security Properties

- Tokens encrypted at rest with XChaCha20-Poly1305
- No raw token values exposed to agent containers
- Token refresh handled entirely by Bridge (no agent access to refresh tokens)
- Keystore locked with user's passphrase (SQLCipher database key)
- Access logged to audit database

#### Token Structure

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

### Bridge-Local Registry

The bridge-local execution registry enables email pipeline steps (send, approval) to run as native Bridge operations without spawning agent containers.

#### Registry Integration

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

#### Email Handlers Registered

| Step Type | Handler | Description |
|-----------|---------|-------------|
| `email_send` | `OutboundExecutor` | Validates, resolves PII, sends via Gmail/Outlook/SMTP |
| `email_approval` | `EmailApprovalManager` | Blocks until user approves via ArmorChat |

#### Benefits

- **Performance**: No container spawn overhead for native Bridge operations
- **Security**: Sensitive operations (PII resolution, token access) stay in Bridge
- **Simplicity**: Email pipeline steps run as native Go code, not containers
- **Audit**: Native Bridge operations are fully audited

#### Workflow Integration

When the secretary workflow executes an email step:

```
1. Secretary Workflow Engine reads step
2. Step type: "email_send" or "email_approval"
3. Check BridgeLocalRegistry for handler
4. If found: execute handler directly in Bridge (no container)
5. If not found: spawn agent container for step execution
```

---

## Email Android Integration

### Overview

This document specifies the Matrix event types, payload schemas, and UI requirements for ArmorChat Android to integrate with the Sovereign Email Pipeline. **No Kotlin code changes are included** — this is an informational specification for the Android team.

---

### Matrix Event Types

#### Email Approval Request

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

#### Email Approval Response

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

#### Email Received

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

### UI Requirements

#### PiiApprovalCard Component

Reuse the existing `PiiApprovalCard.kt` pattern with email-specific extensions:

1. **Card Layout**: Same as existing PII approval card (batched fields, sensitivity badges)
2. **Email Context**: Show recipient (masked), subject (masked), body preview (first 150 chars)
3. **maxLines**: Body preview limited to 5 lines
4. **Buttons**: Approve / Reject (same as existing PII approval flow)

#### Biometric Model

- **Device-level KeyGuard**: Use Android KeyGuard confirmation (device PIN/fingerprint)
- **NOT per-click prompt**: Single biometric unlock per session for email approvals
- **Fallback**: PIN entry if biometric unavailable

#### Notification

- Push notification via existing Matrix notification channel
- Title: "Email Approval Required"
- Body: "Response to [recipient] needs your approval"
- Action: Deep link to approval card in conversation (`armorclaw://email/approve/<approval_id>`, handled by `DeepLinkHandler` → `Route.EmailApproval`)

> **v0.7.0**: Deep link routing for email approvals is now implemented. `DeepLinkHandler.kt` resolves `armorclaw://email/approve/{id}` to `Route.EmailApproval(approvalId)`. Cold-start and warm-resume are handled via `MainActivity.onNewIntent()` + `LaunchedEffect`.

---

### Event Flow

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

### Configuration

| Parameter | Default | Description |
|-----------|---------|-------------|
| `approval_timeout_seconds` | 300 | Time before approval request expires |
| `max_body_preview_chars` | 150 | Characters shown in approval card |
| `pii_masking_enabled` | true | Whether PII is masked in previews |
| `biometric_required` | false | Whether biometric is required for approval |

---

### Compatibility Notes

- These event types extend the existing `app.armorclaw.*` namespace
- Events are transient (not state) — they do not persist in room state
- The `sensitivity_badges` field reuses the existing `SensitivityBadge` model from `PiiApprovalCard.kt`
- No changes to existing Matrix SDK or event handling — new event types are handled via the existing message pipeline

---

### Implementation Status

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

---

## Document Sidecar Pipeline

### Overview

The document processing pipeline handles file ingestion, text extraction, encryption, and split-storage for RAG across multiple codebases: a Rust sidecar (data plane), a Java gRPC sidecar (Apache POI — legacy DOC/PPT extraction), a Python MarkItDown sidecar (MSG/XLS legacy Office formats), a Go gRPC client with 3-layer routing (control plane bridge), and a YARA content scanner. Together they form the secure document path from cloud storage to chunked, encrypted storage with provenance tracking.

**Not to be confused with `rust-vault/`.** The vault handles secrets and credential storage. The sidecar handles documents: extracting text, encrypting chunks, scanning for malware, and maintaining a provenance chain. They share no code.

The Rust sidecar is a high-performance data plane component. It does the heavy lifting: cloud storage I/O, document parsing, AEAD encryption, and chunking. The Go Bridge is the control plane that owns security decisions, audit logging, PII interception, and request queuing. They communicate over a Unix domain socket via gRPC.

For the full sidecar API reference, compilation status, test coverage, and deployment instructions, see [sidecar/README.md](../sidecar/README.md). This document covers the pipeline as a whole, including the Go and YARA components that the sidecar README does not address.

### Architecture

```
                        Go Bridge (Control Plane)
                       ┌────────────────────────────────────────┐
                       │                                        │
                       │  bridge/pkg/sidecar/                   │
                       │  ┌──────────┐  ┌──────────────────┐   │
Agent request ────────▶│  │ Client   │  │ PIIInterceptor   │   │
                       │  │ (gRPC)   │  │ (redact/reject)  │   │
                       │  └────┬─────┘  └────────┬─────────┘   │
                       │       │                  │              │
                       │  ┌────▼─────┐  ┌────────▼─────────┐   │
                       │  │ Queue    │  │ AuditClient      │   │
                       │  │ Manager  │  │ (audit.db log)   │   │
                       │  └────┬─────┘  └──────────────────┘   │
                       │       │                                │
                       │  ┌────▼─────┐                          │
                       │  │ Token    │ HMAC-SHA256, 30 min TTL  │
                       │  │ Generator│                          │
                       │  └────┬─────┘                          │
                       └───────┼────────────────────────────────┘
                               │ gRPC over Unix Socket
                               │ (0600 permissions)
                       ┌───────▼────────────────────────────────┐
                       │  Rust Sidecar (Data Plane)              │
                       │  sidecar/                               │
                       │                                        │
                       │  ┌────────────┐  ┌───────────────┐     │
                       │  │ Connectors │  │ Document      │     │
                        │  │ S3, SP     │  │ PDF, DOCX,    │     │
                        │  │            │  │ XLSX, PPTX,   │     │
                        │  │            │  │ OCR           │     │
                       │  └──────┬─────┘  └───────┬───────┘     │
                       │         │                │             │
                       │  ┌──────▼────────────────▼───────┐     │
                       │  │ Split-Storage Manager          │     │
                       │  │ Encrypt chunks (XChaCha20)     │     │
                       │  │ Provenance signing (HMAC-SHA256)│     │
                       │  └────────────────────────────────┘     │
                       └────────────────────────────────────────┘

                       ┌────────────────────────────────────────┐
                       │  YARA Scanner (bridge/pkg/yara/)       │
                       │  Content disarm and reconstruction     │
                       │  Scans files before sidecar processing │
                       └────────────────────────────────────────┘
```

#### Data Flow

1. Agent sends a document request to the Go Bridge.
2. The Bridge runs PII detection on the request payload. If PII is found and the interceptor is set to `redact`, the payload is scrubbed before forwarding. If set to `reject`, the request is denied.
3. The Bridge generates an ephemeral HMAC-SHA256 token (30 minute TTL) and attaches it as request metadata.
4. The YARA scanner (`bridge/pkg/yara/`) checks the file for known malware signatures before the sidecar touches it.
5. The request is forwarded to the Rust sidecar over a Unix domain socket via gRPC.
6. The sidecar extracts text, chunks it, encrypts each chunk with XChaCha20-Poly1305, and signs the result with a provenance signature.
7. Results flow back through the Bridge, which logs the full operation to its audit database.

If the sidecar is down, the Bridge's queue manager buffers requests and retries with exponential backoff.

### Key Packages

#### Rust Sidecar (`sidecar/`)

The Rust sidecar is organized into the following modules. The library code is production-quality but requires `protoc` (Protocol Buffers compiler) to compile due to the gRPC service definition. The binary target has outstanding compilation errors and is not needed for library use.

##### Connectors (`sidecar/src/connectors/`)

Cloud storage adapters. Each connector implements upload, download, list, and delete operations with streaming support for large files (up to 5 GB).

| Connector | File | Status |
|-----------|------|--------|
| AWS S3 | `aws_s3.rs` | Functional |
| SharePoint | `sharepoint.rs` | Functional (Microsoft Graph API) |
| Azure Blob | `azure_blob.rs.disabled` | Disabled, needs rustls migration |

The `CloudConnector` trait in `connector.rs` defines the shared interface. The `SharePointConnector` is the reference implementation.

##### Document Processing (`sidecar/src/document/`)

Extracts text from common document formats. All extractors return structured results with page counts and metadata maps.

| Format | File | Notes |
|--------|------|-------|
| PDF | `pdf.rs` | Text extraction, split, merge |
| DOCX | `docx.rs` | Text extraction, paragraph insert/delete, find/replace |
| XLSX | `xlsx.rs` | Functional — calamine-based extraction with ShadowMap redaction |
| PPTX | `pptx.rs` | ZIP-based extraction using `zip` + `quick-xml` crates (v0.6.0) |
| OCR | `ocr.rs` | Functional — Tesseract subprocess + ONNX fallback, multi-language |

OCR extraction tries Tesseract first. If Tesseract fails or is unavailable, the ONNX runtime model runs as a fallback, ensuring extraction succeeds even without a Tesseract installation.
| Diff | `diff.rs` | Myers algorithm for text diff |
| HTML Diff | `html_diff.rs` | HTML-aware diff generation |
| DOCX Diff | `docx_diff.rs` | Stub, redline document generation |

### ShadowMap PII Redaction (XLSX)

The XLSX extractor in `sidecar/src/document/xlsx.rs` integrates ShadowMap-based PII redaction during cell extraction. As cells are read from the Excel file via the calamine library, each cell value is checked against PII patterns (SSN, credit card numbers, phone numbers, email addresses). Matches are replaced with `[REDACTED:hash]` placeholders using SHA256 hash-based references, matching the Governor-Shield placeholder format. The redaction happens at the cell level before text assembly, ensuring PII never enters the extracted text output.

Additional document modules:

| Module | File | Purpose |
|--------|------|---------|
| RAG Chunking | `rag.rs` | `TextChunker` with pluggable chunking strategies |
| Embeddings | `embeddings.rs` | `EmbeddingGenerator` trait, `OpenAIEmbedder` implementation |
| Qdrant | `qdrant.rs` | Implemented — create/upsert/search (needs qdrant-client-rs v1.7 builder migration) |

The `MAX_FILE_SIZE` constant (5 GB) caps all document operations.

##### ProcessDocument Convert

The `ProcessDocument` RPC's `convert` operation supports DOCX-to-PDF and XLSX-to-CSV conversion:
- **DOCX→PDF**: Extracts text from DOCX via `extract_text_from_docx()`, paginates to A4 pages (210x297mm), renders with printpdf using Helvetica built-in font.
- **XLSX→CSV**: Extracts structured data from XLSX via `extract_data_from_xlsx()` (calamine), formats rows as RFC 4180 CSV (fields with commas/quotes/newlines are quoted).
- PPTX→PDF returns a clear "not yet supported" error (not silent passthrough).

##### Encryption (`sidecar/src/encryption/`)

`aead.rs` implements `AeadCipher`, which wraps XChaCha20-Poly1305 with deterministic nonce derivation. Nonces are derived via HMAC-SHA256 of `key_id || blob_id` plus a fixed message, ensuring the same blob always encrypts to the same ciphertext (idempotent encryption). The cipher key is zeroized on drop. Decryption returns plaintext wrapped in `Zeroizing<Vec<u8>>` to limit plaintext exposure in memory.

Wire format: `[version: 1 byte][nonce: 24 bytes][ciphertext + Poly1305 tag]`

##### Provenance (`sidecar/src/provenance/`)

`signer.rs` implements `ProvenanceSigner`, which produces truncated 8-byte HMAC-SHA256 signatures for lightweight provenance tracking. Verification uses constant-time comparison to prevent timing attacks. The formatted output looks like:

```
[Provenance: AC-v6-Sig:a1b2c3d4e5f6a1b2 | Sess:sess-123]
```

##### Split-Storage Manager (`sidecar/src/split_storage/`)

`manager.rs` ties encryption and chunking together. `SplitStorageManager` takes text chunks, encrypts them with the AEAD cipher, and wraps the result in an `EncryptedPayload` struct (base64-encoded ciphertext, version byte, clearance level). It supports decryption and clearance-based filtering, ensuring that retrieval only returns chunks the caller is authorized to see.

##### gRPC Service (`sidecar/src/grpc/`)

The server in `server.rs` implements the `SidecarService` trait defined in the proto. It routes gRPC calls to the appropriate connector or document module. Key RPCs:

| RPC | Purpose |
|-----|---------|
| `HealthCheck` | Returns status, uptime, version, active_requests, memory_used_bytes |
| `UploadBlob` | Upload to S3 via `destination_uri` (s3://bucket/key) |
| `DownloadBlob` | Server-streaming download, 1 MB chunks |
| `ListBlobs` | List objects with prefix filter |
| `DeleteBlob` | Delete an object |
| `ExtractText` | Extract text from PDF, DOCX, XLSX, or images (OCR) |
| `ProcessDocument` | General document processing: extract_text, convert (DOCX→PDF, XLSX→CSV) |
| `QueryDocuments` | Query encrypted chunks from split-storage by clearance level |

`interceptor.rs` implements `SecurityInterceptor`, which validates ephemeral tokens on every incoming request. The server binds to a Unix domain socket with `0600` permissions and handles SIGTERM/SIGINT for graceful shutdown.

The proto definition lives in `sidecar/src/grpc/proto/sidecar.proto` and is synced with `bridge/pkg/sidecar/sidecar.proto`. Both define 8 RPCs: HealthCheck, UploadBlob, DownloadBlob, ListBlobs, DeleteBlob, ExtractText, ProcessDocument, and QueryDocuments.

#### Go Client (`bridge/pkg/sidecar/`)

The Go client is the Bridge's interface to the sidecar. It provides a layered architecture: raw client, audit wrapper, and queuing system.

##### `client.go`

The core `Client` type manages a gRPC connection over a Unix domain socket. Key design decisions:

- **Retry with exponential backoff.** Every operation runs through `withRetry()`, which reconnects and retries up to 5 times with capped backoff (max 5 seconds).
- **PII interception.** When enabled, the `PIIInterceptor` scans request payloads before forwarding them to the sidecar.
- **Streaming downloads.** `DownloadBlob` collects chunks from the server stream and reassembles them into a single byte slice.
- **Configurable message sizes.** Default max is 256 MB for both send and receive.
- **Version negotiation.** gRPC interceptors attach `x-sidecar-version` metadata to every request.

##### `audit_client.go`

`AuditClient` wraps `Client` and logs every operation to the Bridge's audit database (`audit.db`). It records:

- Operation name and duration
- Success/failure status
- File sizes
- Request/user/agent/session IDs extracted from gRPC metadata
- Custom event types: `EventSidecarHealthCheck`, `EventSidecarUploadBlob`, `EventSidecarDownloadBlob`, `EventSidecarExtractText`, `EventSidecarProcessDocument`, `EventSidecarListBlobs`, `EventSidecarDeleteBlob`

It also provides `LogQueueEvent` and `LogRetryEvent` for when the sidecar is unavailable.

##### `pii_interceptor.go`

`PIIInterceptor` scans outgoing requests for personally identifiable information before they reach the sidecar. It supports two modes:

| Action | Behavior |
|--------|----------|
| `redact` | Scrubs PII from the request, forwards the cleaned version |
| `reject` | Returns an error, does not forward the request |

A `LogOnly` mode is available for monitoring without modifying requests. The interceptor uses `bridge/pkg/pii.Scrubber` for detection and handles `UploadBlobRequest`, `ExtractTextRequest`, and `ProcessDocumentRequest`. It skips binary content using a heuristic (90% printable ASCII threshold).

##### `queue.go`

`QueueManager` buffers requests when the sidecar is down. It runs a background goroutine that periodically health-checks the sidecar and drains the queue when it comes back up. Configuration:

| Parameter | Default |
|-----------|---------|
| Max queue size | 1000 |
| Max retry attempts | 5 |
| Initial backoff | 1s |
| Max backoff | 30s |
| Backoff multiplier | 2.0 |
| Health check interval | 10s |

`QueuedClient` wraps `Client` with automatic queuing on transient errors (unavailable, deadline exceeded, resource exhausted).

##### `token.go`

`TokenGenerator` creates and validates ephemeral tokens for sidecar authentication. Token format:

```
{request_id}:{timestamp}:{operation}:{hmac_sha256_signature}
```

Constants:

| Constant | Value |
|----------|-------|
| Token TTL | 30 minutes |
| Max timestamp age | 5 minutes |

The generator uses constant-time HMAC comparison to prevent timing attacks. Request IDs are generated with `crypto/rand` (16 bytes, hex-encoded).

##### `version.go`

Client version `1.0.0`, supported server range `1.0.0` through `1.5.0`. gRPC interceptors attach version metadata to every request for compatibility negotiation.

##### `sidecar.proto`

The Protocol Buffers service definition. Defines the `SidecarService` with 8 RPCs (HealthCheck, UploadBlob, DownloadBlob, ListBlobs, DeleteBlob, ExtractText, ProcessDocument, QueryDocuments), request/response messages, and `RequestMetadata` for authentication. The same proto file is compiled into both Rust (tonic) and Go (protoc-gen-go) stubs.

#### YARA Scanner (`bridge/pkg/yara/`)

##### `scanner.go`

The YARA scanner provides content disarm and reconstruction (CDR) for files entering the pipeline. It compiles YARA rules from a file at startup (`InitYARA`) and scans files against those rules (`ScanFileForMalware`). If any rule matches, the scan returns `false` (not clean) and logs the matching rule name and file path at `SECURITY` priority.

The scanner runs in the Go Bridge, before any request reaches the Rust sidecar. This keeps malicious content out of the data plane entirely.

Test data lives in `bridge/pkg/yara/testdata/`.

### Configuration

#### Rust Sidecar Environment Variables

| Variable | Default | Purpose |
|----------|---------|---------|
| `SIDECAR_SOCKET_PATH` | `/tmp/armorclaw-sidecar.sock` | Unix socket path |
| `SIDECAR_MAX_CONCURRENT_REQUESTS` | `1000` | Concurrency limit |
| `AWS_ACCESS_KEY_ID` | | S3 credential |
| `AWS_SECRET_ACCESS_KEY` | | S3 credential |
| `AWS_REGION` | `us-east-1` | S3 region |
| `SHAREPOINT_TENANT_ID` | | SharePoint Graph API |
| `SHAREPOINT_CLIENT_ID` | | SharePoint Graph API |
| `SHAREPOINT_CLIENT_SECRET` | | SharePoint Graph API |
| `SHAREPOINT_SITE_URL` | | SharePoint Graph API |
| `SHARED_SECRET` | | HMAC key for token validation |

#### Go Client Configuration

| Field | Default | Purpose |
|-------|---------|---------|
| `SocketPath` | `/run/armorclaw/sidecar.sock` | Unix socket path |
| `Timeout` | 30s | Default operation timeout |
| `MaxRetries` | 5 | Retry attempts |
| `DialTimeout` | 10s | Connection timeout |
| `IdleTimeout` | 5m | Connection idle timeout |
| `MaxMsgSize` | 256 MB | gRPC message size limit |

#### SidecarConfig Struct (Rust)

```rust
pub struct SidecarConfig {
    pub socket_path: PathBuf,
    pub max_concurrent_requests: usize,
    pub rate_limit_requests_per_second: usize,
    pub rate_limit_burst_capacity: usize,
    pub circuit_breaker_failure_threshold: usize,
    pub circuit_breaker_timeout_seconds: u64,
}
```

The `SidecarConfig` struct also carries the `shared_secret` field used by the security interceptor.

### Integration Points

#### Bridge to Sidecar

The Go Bridge connects to the Rust sidecar via gRPC over a Unix domain socket. The socket is created with `0600` permissions, restricting access to the Bridge process. Every request carries `RequestMetadata` containing an ephemeral token, timestamp, and operation signature. The sidecar's `SecurityInterceptor` validates these before processing.

#### YARA Integration

The YARA scanner runs inside the Bridge process, before sidecar calls. Files are scanned on disk. If a YARA rule matches, the file is flagged and the sidecar request is not made. Rules are loaded from a compiled YARA rules file at Bridge startup.

#### Split-Storage RAG Pipeline

Documents flow through this pipeline:

1. Agent requests document processing via Matrix.
2. Bridge downloads the document (or receives it from the agent).
3. YARA scans the file for malware.
4. Bridge sends the document to the sidecar for text extraction.
5. Sidecar extracts text, chunks it via `TextChunker`.
6. Each chunk is encrypted with `AeadCipher` (XChaCha20-Poly1305).
7. `ProvenanceSigner` attaches a signature to the chunk metadata.
8. `SplitStorageManager` wraps chunks into `EncryptedPayload` structs with clearance levels.
9. Encrypted chunks are stored separately from their embeddings (split-storage pattern).
10. At query time, chunks are decrypted and filtered by clearance before being returned to the agent.

#### Matrix / ArmorChat

Agents initiate document operations through Matrix rooms. The Bridge translates these into sidecar gRPC calls. The `AuditClient` logs every operation to `audit.db`, enabling compliance review through the ArmorChat admin interface.

#### Jetski Browser Sidecar

Jetski (`jetski/`) is a separate component that handles browser automation via CDP. The document sidecar does not interact with Jetski directly. They share the same Bridge control plane but operate independently. Jetski handles web pages; the document sidecar handles files.

#### Python MarkItDown Sidecar (`sidecar-python/`)

The Python sidecar extends the document pipeline with Microsoft Office legacy format support via the MarkItDown library. It handles formats that the Rust and Java sidecars do not support natively: `.msg` (Outlook email) and `.xls` (legacy Excel). DOC and PPT were migrated to the Java Apache POI sidecar in v0.8.0. PPTX was migrated to the Rust sidecar in v0.6.0. XLSX was migrated to the Rust sidecar in v0.8.0 (calamine-based extraction with ShadowMap PII redaction).

##### Architecture

```
                        Go Bridge (Control Plane)
                        ┌────────────────────────────────────────┐
                        │  bridge/pkg/sidecar/                   │
                        │  ┌──────────────────────────────────┐  │
                        │  │ RouteExtractText()               │  │
                        │  │ Layer 0: native text bypass      │  │
                        │  │ Layer 1: compound magic+format   │  │
                        │  │ Layer 2: strict drop on mismatch │  │
                        │  └──────────┬───────────────────────┘  │
                        │             │                           │
                         │     ┌──────┴──────────┐                │
                         │     ▼        ▼        ▼                │
                         │  ┌──────┐ ┌──────┐ ┌──────────┐       │
                          │  │ Rust │ │ Java │ │ Python   │       │
                          │  │ Side │ │ POI  │ │ MarkIt-  │       │
                          │  │ car  │ │ Side │ │ Down     │       │
                          │  │      │ │ car  │ │ Sidecar  │       │
                          │  │ PDF, │ │ DOC, │ │ (MSG,    │       │
                          │  │ DOCX,│ │ PPT  │ │  XLS)    │       │
                          │  │ XLSX,│ │      │ │          │       │
                          │  │ PPTX │ │      │ │          │       │
                          │  └──────┘ └──────┘ └──────────┘       │
                         └────────────────────────────────────────┘
```

##### Routing Logic (3-Layer)

| Layer | Condition | Action |
|-------|-----------|--------|
| **Layer 0** | `text/plain`, `text/csv`, `application/json`, `text/markdown` | Decode natively in Go — no sidecar call |
| **Layer 1** | ZIP magic + xlsx/docx/pptx/pdf → Rust; OLE magic + doc/ppt → Java (fallback Python); OLE magic + xls/msg → Python | Route to appropriate sidecar based on compound magic byte + MIME type validation |
| **Layer 2** | Magic bytes don't match declared format (e.g., ZIP magic + msg format) | **Strict drop** — return `InvalidArgument` immediately |

##### Key Design Decisions

- **Compound validation**: The Go Bridge validates both the file's magic bytes (ZIP: `PK\x03\x04` or OLE: `\xd0\xcf\x11\xe0\xa1\xb1\x1a\xe1`) AND the declared MIME type before routing. Mismatches are rejected at the gateway.
- **No HTTP/FastAPI**: The Python sidecar uses `grpc.server()` exclusively — no HTTP endpoints exposed.
- **Threshold streaming**: Files under 10 MB are converted in-memory via `BytesIO`. Files over 10 MB are written to a temp file for conversion, then cleaned up.
- **TTL recycling**: The server exits gracefully after `MAX_REQUESTS` (default: 50) to enable container restart cycling.
- **Network isolation**: Container runs with `NetworkMode: none`, `cap_drop: ALL`, read-only root filesystem, and tmpfs for `/tmp/office_worker`.

##### Python Server (`sidecar-python/worker.py`)

| Feature | Implementation |
|---------|---------------|
| **gRPC Server** | Sync `grpc.server()` with `ThreadPoolExecutor` |
| **Format Mapping** | `FORMAT_MAP` — 6 MIME types → extensions |
| **Conversion** | MarkItDown library with `StreamInfo` for in-memory path |
| **Threshold** | `_THRESHOLD_BYTES = 10 * 1024 * 1024` (10 MB) |
| **TTL** | `MAX_REQUESTS = 50` before graceful shutdown |
| **Version** | `SERVER_VERSION = "1.0.0"` in `HealthCheck` response |
| **Socket** | `SIDECAR_SOCKET` env var (default: `/run/armorclaw/office-sidecar/sidecar-office.sock`) |

##### Token Interceptor (`sidecar-python/interceptor.py`)

HMAC-SHA256 token validation using a sync `grpc.ServerInterceptor`. Tokens carry `{request_id}:{timestamp}:{hmac_signature}` format with configurable TTL. The interceptor was originally implemented as `grpc_aio.ServerInterceptor` (async), which was incompatible with the sync `grpc.server()` in `worker.py`. This has been fixed: `interceptor.py` now uses a sync interceptor that works correctly with the sync gRPC server.

##### Supported Formats

| Format | MIME Type | Magic Bytes | Extension | Converter |
|--------|-----------|-------------|-----------|-----------|
| Excel (modern) | `application/vnd.openxmlformats-officedocument.spreadsheetml.sheet` | ZIP (PK) | `.xlsx` | Rust calamine extractor (xlsx.rs) — migrated from Python in v0.8.0 |
| PowerPoint (modern) | `application/vnd.openxmlformats-officedocument.presentationml.presentation` | ZIP (PK) | `.pptx` | Rust PPTX Extractor (pptx.rs) |
| Word (legacy) | `application/msword` | OLE (D0CF) | `.doc` | Java Apache POI sidecar (`HWPFDocument`) — migrated from Python in v0.8.0 |
| PowerPoint (legacy) | `application/vnd.ms-powerpoint` | OLE (D0CF) | `.ppt` | Java Apache POI sidecar (`HSLFSlideShow`) — migrated from Python in v0.8.0 |
| Outlook Email | `application/vnd.ms-outlook` | OLE (D0CF) | `.msg` | Python MarkItDown `OutlookMsgConverter` |
| Excel (legacy) | `application/vnd.ms-excel` | OLE (D0CF) | `.xls` | Python MarkItDown `XlsConverter` |

> **DOC/PPT resolved**: Legacy `.doc` and `.ppt` files previously produced conversion errors because MarkItDown's `XlsConverter` claimed OLE files before the Word/PowerPoint converters. This was resolved in v0.8.0 by routing DOC/PPT to the Java Apache POI sidecar (`sidecar-java/`), which uses `HWPFDocument` and `HSLFSlideShow` respectively for reliable OLE2 extraction.

##### PPTX Migration to Rust (v0.6.0)

PPTX text extraction has been migrated from Python MarkItDown to the Rust sidecar:

- **Extractor**: `sidecar/src/document/pptx.rs` — ZIP-based extraction using `zip` + `quick-xml` crates
- **Routing**: `bridge/pkg/sidecar/office_client.go` — `.pptx` files route to Rust sidecar (Layer 1)
- **Format support**: Multi-slide presentations, speaker notes, embedded media metadata
- **Security**: Malformed archive protection, XML bomb mitigation, size limits
- The 3-layer routing architecture is preserved — only the PPTX destination changed from Python to Rust

#### Java Apache POI Sidecar (`sidecar-java/`)

The Java sidecar handles legacy `.doc` and `.ppt` extraction using Apache POI, formats that previously produced errors in the Python MarkItDown sidecar. It was introduced in v0.8.0.

##### Architecture

```
                        Go Bridge (Control Plane)
                        ┌────────────────────────────────────────┐
                        │  bridge/pkg/sidecar/                   │
                        │  ┌──────────────────────────────────┐  │
                        │  │ RouteExtractText()               │  │
                        │  │ OLE + doc/ppt → javaClient       │  │
                        │  │ Fallback → officeClient (Python) │  │
                        │  └──────────┬───────────────────────┘  │
                        └────────────┼───────────────────────────┘
                                     │ gRPC over Unix Socket
                                     │ (0600 permissions)
                        ┌────────────▼───────────────────────────┐
                        │  Java Sidecar (sidecar-java/)          │
                        │                                        │
                        │  ┌──────────────────────────────────┐  │
                        │  │ ExtractorServiceImpl              │  │
                        │  │  - DOC: HWPFDocument extract      │  │
                        │  │  - PPT: HSLFSlideShow extract     │  │
                        │  │  - Unsupported → INVALID_ARGUMENT │  │
                        │  └──────────────────────────────────┘  │
                        │                                        │
                        │  ┌──────────────────────────────────┐  │
                        │  │ ServerMain                        │  │
                        │  │  - gRPC ServerBuilder             │  │
                        │  │  - Unix socket from SOCKET_PATH   │  │
                        │  │  - TokenInterceptor               │  │
                        │  │  - VersionInterceptor             │  │
                        │  └──────────────────────────────────┘  │
                        └────────────────────────────────────────┘
```

##### Routing Logic

The Go Bridge routes DOC/PPT to the Java sidecar via `RouteExtractText()`:

1. **Primary path**: OLE magic bytes + `application/msword` or `application/vnd.ms-powerpoint` MIME type → `javaClient.ExtractText()` (4th parameter)
2. **Fallback path**: If `javaClient` is nil (Java sidecar not deployed) → falls back to `officeClient` (Python MarkItDown sidecar)
3. **XLS exclusion**: `.xls` always routes to Python regardless of Java sidecar availability

##### Supported Formats

| Format | MIME Type | Magic Bytes | Extension | POI Component |
|--------|-----------|-------------|-----------|---------------|
| Word (legacy) | `application/msword` | OLE (D0CF) | `.doc` | `org.apache.poi.hwpf.HWPFDocument` |
| PowerPoint (legacy) | `application/vnd.ms-powerpoint` | OLE (D0CF) | `.ppt` | `org.apache.poi.hslf.usermodel.HSLFSlideShow` |

##### Key Design Decisions

- **Apache POI**: Chosen over MarkItDown for DOC/PPT because POI natively understands OLE2 compound document format, whereas MarkItDown's `XlsConverter` incorrectly claims all OLE files before Word/PowerPoint converters can process them
- **Fallback to Python**: If Java sidecar is unavailable, DOC/PPT fall back to the Python sidecar (which will produce the XlsConverter error, but maintains pipeline availability)
- **gRPC over Unix socket**: Same communication pattern as Rust and Python sidecars — `SOCKET_PATH` env var, 0600 permissions
- **Token + Version interceptors**: gRPC server interceptors for HMAC-SHA256 token validation and version reporting, matching Python sidecar security model
- **No network access**: Container runs with same hardening as Python sidecar (`NetworkMode: none`, `cap_drop: ALL`, read-only root)
- **Java 21 runtime**: Requires JDK 21+ (tested with Eclipse Temurin 21.0.11)

##### Test Coverage

| Test File | Tests | Description |
|-----------|-------|-------------|
| `sidecar-java/src/test/java/.../ExtractorServiceTest.java` | 8 | DOC/PPT extraction, empty input, unsupported format, null body |
| `bridge/pkg/sidecar/office_client_test.go` | 22 | Go routing: DOC/PPT → Java, fallback to Python, XLS stays Python |
| `bridge/pkg/sidecar/java_sidecar_e2e_test.go` | 4 | Full E2E: health, DOC extraction, PPT extraction, unsupported (skip without Java 21) |
| `tests/test-sidecar-docs.sh` | 3 | Bash harness: D2.5 Java health, D5.5 DOC, D5.6 PPT |

##### Running Tests

```bash
# Java unit tests (requires Java 21)
cd sidecar-java && JAVA_HOME="$(asdf where java temurin-21.0.11+10.0.LTS)" mvn test

# Go routing tests (22 tests including Java paths)
cd bridge && go test -v -run "TestRouteExtractText" ./pkg/sidecar/...

# Go E2E tests (skip gracefully without Java 21/JAR)
cd bridge && go test -v -run "TestJavaSidecarE2E" ./pkg/sidecar/...

# Bash harness
bash tests/test-sidecar-docs.sh
```

##### Docker Deployment (`deploy/docker-compose.sidecar-java.yml`)

```yaml
# Container hardening (matches Python sidecar)
network_mode: none
cap_drop: [ALL]
read_only: true
security_opt: [no-new-privileges:true]
mem_limit: 512MB
environment:
  - SOCKET_PATH=/run/armorclaw/sidecar-java.sock
  - TOKEN_SECRET=${SIDECAR_TOKEN_SECRET}
```

##### Docker Deployment (`deploy/docker-compose.sidecar-py.yml`)

```yaml
# Container hardening
network_mode: none
cap_drop: [ALL]
read_only: true
security_opt: [no-new-privileges:true]
mem_limit: 512MB
tmpfs:
  - /tmp/office_worker:size=100M
```

##### Test Coverage (Combined)

| Test File | Tests | Status |
|-----------|-------|--------|
| `sidecar-python/test_worker.py` | 23 | All pass |
| `sidecar-python/test_edge_cases.py` | 16 | All pass |
| `sidecar-python/test_interceptor.py` | 12 | All pass |
| `sidecar-python/test_docker_integration.py` | 10 | Skip when no Docker |
| `sidecar-java/src/test/java/.../ExtractorServiceTest.java` | 8 | All pass |
| `bridge/pkg/sidecar/office_client_test.go` | 22 | All pass |
| `bridge/pkg/sidecar/office_client_e2e_test.go` | 7 | All pass |
| `bridge/pkg/sidecar/java_sidecar_e2e_test.go` | 4 | Skip without Java 21 |
| **Total** | **102** | **0 regressions** |

##### Running Tests (Combined)

```bash
# Python unit + integration tests
cd sidecar-python && python -m pytest test_worker.py test_edge_cases.py test_interceptor.py -v

# Java unit tests (requires Java 21)
cd sidecar-java && JAVA_HOME="$(asdf where java temurin-21.0.11+10.0.LTS)" mvn test

# Go routing + E2E tests (includes Java routing paths)
cd bridge && go test -v -run "TestRouteExtractText|TestE2E|TestJavaSidecarE2E" ./pkg/sidecar/

# Full regression (Python + Java + Go)
cd sidecar-python && python -m pytest -v
cd sidecar-java && JAVA_HOME="$(asdf where java temurin-21.0.11+10.0.LTS)" mvn test
cd bridge && go test ./pkg/sidecar/...
```

##### Go Client Routing (`bridge/pkg/sidecar/office_client.go`)

The `RouteExtractText()` function implements the 3-layer routing:

1. **Native text bypass**: Detects `text/*` MIME types and returns decoded content immediately without any gRPC call.
2. **Compound validation**: Reads first 8 bytes for magic bytes, cross-references with `document_format` MIME type. Routes ZIP-based xlsx/docx/pptx to Rust sidecar. Routes OLE-based doc/ppt to Java sidecar (with Python fallback). Routes OLE-based xls/msg to Python sidecar.
3. **Strict drop**: If magic bytes contradict the declared format (e.g., OLE magic with xlsx MIME), returns `codes.InvalidArgument` without calling any sidecar.

### References

- [sidecar/README.md](../sidecar/README.md) - Full Rust sidecar documentation (API, testing, deployment, security audit)
- armorclaw.md - ArmorClaw system documentation index
- `.sisyphus/audits/SECURITY_AUDIT_TASK_49.md` - Security audit results
- `.sisyphus/plans/rust-office-sidecar.md` - Rust sidecar implementation plan
- `.sisyphus/plans/markitdown-sidecar.md` - Python MarkItDown sidecar implementation plan
- `.sisyphus/plans/markitdown-sidecar-testing.md` - Python sidecar testing plan
- `.sisyphus/plans/java-sidecar-legacy-office.md` - Java Apache POI sidecar implementation plan (DOC/PPT)

---

## Voice Stack

### Current State

The voice stack has a complete infrastructure layer (WebRTC, budget enforcement, security policies, TURN traversal) but **zero concrete speech providers**. STT, TTS, and VAD services define interfaces only. No AI provider backends exist. The voice manager initialization is commented out in `bridge/cmd/bridge/main.go`.

#### What Exists

| Component | Status | File |
|-----------|--------|------|
| WebRTC Engine | Implemented | `bridge/pkg/webrtc/engine.go` |
| WebRTC Sessions | Implemented | `bridge/pkg/webrtc/session.go` |
| TURN Manager | Implemented | `bridge/pkg/turn/turn.go` |
| Budget Tracker | Implemented | `bridge/pkg/voice/budget.go` |
| Security Enforcer | Implemented | `bridge/pkg/voice/security.go` |
| TTL Manager | Implemented | `bridge/pkg/voice/security.go` |
| Security Audit | Implemented | `bridge/pkg/voice/security.go` |
| Matrix Call Signaling | Implemented (unwired) | `bridge/pkg/voice/matrix.go` |
| Voice Manager | Implemented (commented out) | `bridge/pkg/voice/manager.go` |

#### What Is Missing

| Component | Status | Gap |
|-----------|--------|-----|
| STT Provider | Interface only | `voice.Transcriber` has no implementation |
| TTS Provider | Interface only | `voice.Synthesizer` has no implementation |
| VAD Provider | Interface only | `voice.SpeechDetector` has no implementation |
| Audio Pipeline | Not implemented | No PCM routing between WebRTC and agent |
| Voice Manager Wiring | Commented out | `main.go` lines 1988-2103 |

#### Runtime Reality

The voice import and all initialization code in `main.go` is wrapped in a block comment:

```go
// TODO: Voice package needs refactoring - uncomment when fixed
// "github.com/armorclaw/bridge/pkg/voice"

/*
    voiceConfig := voice.DefaultConfig()
    ...
    voiceMgr := voice.NewManager(...)
    if err := voiceMgr.Start(); err != nil { ... }
*/
```

Even if uncommented, the voice manager sets its internal `voiceMgr` field to `nil`, so Matrix call signaling would not function.

#### Interface Discrepancy

Two packages define overlapping voice interfaces with different method signatures:

**`bridge/pkg/interfaces/voice.go`** (canonical result types):
- `VoiceManager.HandleMatrixCallEvent(roomID, eventID, senderID string, event interface{}) error`
- `Transcriber.Transcribe(ctx, audioData []byte) (*TranscriptionResult, error)`
- `Synthesizer.Synthesize(ctx, text string) (*SynthesisResult, error)`
- `SpeechDetector.DetectSpeech(ctx, audioData []byte) (*VADResult, error)`

**`bridge/pkg/voice/` package** (service wrappers):
- `Manager.HandleMatrixCallEvent(roomID, eventID, senderID string, event *CallEvent) error`
- `Transcriber.Transcribe(ctx, audioData []byte) (*interfaces.TranscriptionResult, error)`
- `Synthesizer.Synthesize(ctx, text string) (*interfaces.SynthesisResult, error)`
- `SpeechDetector.DetectSpeech(ctx, audioData []byte) (*interfaces.VADResult, error)`

The `VoiceManager` signatures differ: `interface{}` vs `*CallEvent`. The `Manager` struct does not satisfy the `interfaces.VoiceManager` interface. The `Transcriber`, `Synthesizer`, and `SpeechDetector` interfaces are duplicated between packages, though they share the same method signatures and return types from `interfaces`.

#### E2E Test Expectations

`bridge/pkg/voice/e2e_test.go` expects HTTP sidecar services that do not exist:
- VAD at `http://localhost:8001/health`
- STT at `http://localhost:8002/health`
- TTS at `http://localhost:8003/health`

These tests run only when `ARMORCLAW_E2E=1` is set. They will fail until concrete providers are deployed.

### Overview

The voice stack is designed to let ArmorClaw agents make and receive real-time phone calls through the mobile app. It handles everything from audio encoding to NAT traversal entirely inside the Bridge. Audio never touches the agent container directly; the Bridge encodes, decodes, and forwards it between the WebRTC peer and the agent's stdin/stdout or gRPC stream.

The stack is built on four packages: `audio` for PCM and Opus processing, `voice` for call budget enforcement and speech services, `webrtc` for peer connection management and session lifecycle, and `turn` for NAT traversal with ephemeral credentials.

### Architecture

Audio flows through a fixed path from the caller's phone to the agent and back. The Bridge sits in the middle, handling codec work, budget checks, and signaling. TURN relays handle NAT punching when direct connections are not possible.

```
                          ArmorClaw Voice Call Flow

  ┌──────────┐       ┌───────────┐       ┌─────────────────────────────────────┐
  │  Phone   │       │   TURN    │       │            Bridge (VPS)             │
  │ ArmorChat│       │  Relay    │       │                                     │
  │          │       │           │       │  ┌─────────┐  ┌───────┐  ┌───────┐ │
  │ Mic ─────┼──SDP──┼───────────┼──RTP──┼─▶│ webrtc  │─▶│ audio │─▶│ voice │ │
  │          │       │  (NAT     │       │  │ engine  │  │ pcm   │  │ budget│ │
  │ Speaker ◀┼──SDP──┼──traversal│◀─RTP──┼──│ session │◀─│ opus  │◀─│ check │ │
  │          │       │  only)    │       │  └────┬────┘  └───┬───┘  └───┬───┘ │
  └──────────┘       └───────────┘       │       │            │           │     │
                                          │       │            │           │     │
                                          │       ▼            ▼           │     │
                                          │  ┌──────────────────────┐     │     │
                                          │  │    Agent Container   │◀────┘     │
                                          │  │    (AI runtime)      │           │
                                          │  └──────────────────────┘           │
                                          └─────────────────────────────────────┘

  Signaling path (SDP offer/answer, ICE candidates):
    Phone ◀── Matrix E2EE room ──▶ Bridge

  Media path (audio RTP):
    Phone ◀── TURN relay (or direct) ──▶ Bridge

  Budget path:
    Bridge tracks tokens + duration per session, enforces hard stop
```

The signaling layer uses Matrix rooms for SDP exchange and ICE candidate trickling. The media layer runs over RTP through TURN or direct UDP. Budget enforcement runs as a background goroutine that checks every 30 seconds.

### Key Packages

#### `bridge/pkg/audio/`

PCM processing and Opus codec support. All audio I/O lives here, not in agent containers.

| File | Purpose |
|------|---------|
| `pcm.go` | `AudioConfig` defaults (48 kHz, mono, 16-bit, 20 ms frames), `AudioStream` bidirectional frame channels, `AudioPipeline` per-session stream pairs, `PCMMixer` for combining multiple streams, `PCMEncoder` with sample rate conversion, `AudioBuffer` circular ring buffer, `WebRTCTrackReader`/`Writer` for Pion track I/O |
| `opus.go` | `OpusEncoder`/`OpusDecoder` for PCM-to-Opus conversion, `OpusConfig` with bitrate/complexity/FEC/DTX tuning, `RTPOpusPacketizer`/`RTPDepacketizer` for RTP framing, `AudioStats` frame/packet/jitter tracking, `AudioLevelMeter` dBFS measurement, `OpusPayloader`/`Depayloader` for Pion integration |

Default audio config: 48 kHz sample rate, mono, 16-bit depth, 960 samples per frame (20 ms), 10-frame buffer (200 ms).

#### `bridge/pkg/voice/`

Call budget tracking, security enforcement, and speech service wrappers. Prevents runaway token costs, enforces time limits, and defines abstraction over AI provider speech APIs.

| File | Purpose |
|------|---------|
| `budget.go` | `BudgetTracker` manages per-session limits, `VoiceSessionTracker` tracks token usage (input + output) and duration, `TokenUsage` counters, `Config` with default/duration limits and warning thresholds, background `EnforceLimits` loop (30 s interval), security logging for budget events |
| `stt_service.go` | `STTService` wraps `Transcriber` interface for speech-to-text. **Interface only, no provider.** |
| `tts_service.go` | `TTSService` wraps `Synthesizer` interface for text-to-speech synthesis. **Interface only, no provider.** |
| `vad_service.go` | `VADService` wraps `SpeechDetector` interface for voice activity detection. **Interface only, no provider.** |
| `manager.go` | `Manager` orchestrates sessions, budget, security, and WebRTC. Implemented but commented out in `main.go`. `MatrixManager` field is `nil`. |
| `matrix.go` | `MatrixManager` for Matrix call signaling (invite, answer, hangup, reject, ICE candidates). Implemented but never wired into the top-level `Manager`. |
| `security.go` | `SecurityEnforcer` (concurrent call limits, blocklists, rate limiting), `SecurityAudit` (call auditing, violation tracking, reports), `TTLManager` (session expiry enforcement). All fully implemented. |
| `e2e_test.go` | Health check tests for STT/TTS/VAD HTTP sidecars. Expects services at ports 8001/8002/8003 that do not exist. Skipped unless `ARMORCLAW_E2E=1`. |

Key defaults:
- Token limit: 100,000 per call
- Duration limit: 30 minutes per call
- Warning threshold: 80% of limit
- Hard stop: enabled by default

The tracker emits `voice_budget_warning` security events when usage crosses the warning threshold and `voice_budget_enforced` when hard-stopping a call.

#### `bridge/pkg/webrtc/`

WebRTC peer connection management and session lifecycle. This is where Matrix rooms, agent containers, TURN allocations, and budget sessions are bound together.

| File | Purpose |
|------|---------|
| `engine.go` | `Engine` creates and manages `PeerConnectionWrapper` instances, registers Opus codec, handles SDP offer/answer exchange, writes audio to local tracks, reads RTP from remote tracks, integrates with `turn.Manager` for ephemeral credentials |
| `session.go` | `SessionManager` handles the full lifecycle of `Session` objects (pending, active, ended, failed, expired), TTL enforcement with 1-minute cleanup interval, binds session to container ID, Matrix room ID, TURN credentials, and budget session |
| `signaling.go` | WebRTC signaling |
| `token.go` | Token management |

Session states: `pending` (created, not connected) to `active` (media flowing) to `ended` (normal close), `failed` (error), or `expired` (TTL hit).

Default TTL: 10 minutes. Max TTL: 1 hour. Session IDs use `sess_` prefix with 16 hex chars from `crypto/rand`.

#### `bridge/pkg/turn/`

NAT traversal with ephemeral per-session TURN credentials. No static passwords.

| File | Purpose |
|------|---------|
| `turn.go` | `Manager` generates time-limited TURN credentials using HMAC-SHA1, `TURNCredentials` with `<expiry>:<session_id>` username format, `ICEGatherer` for host candidate gathering (reflexive/relay gathering return empty, delegated to WebRTC stack), `ICECandidate` parsing and serialization, `STUNMessage` builder/parser for STUN binding requests, `CreateICEServers` helper for Pion integration |

Credential format: username is `<unix_expiry>:<session_id>`, password is `base64(HMAC-SHA1(secret, username))`. Credentials are scoped to a single session and auto-expire. A cleanup goroutine runs every minute to purge stale entries.

#### Speech Services (`bridge/pkg/voice/`)

Three service wrappers define interfaces for AI provider speech APIs. **No concrete providers exist.** Each source file carries an `INTERFACE-ONLY` comment.

##### STTService (stt_service.go)
- Wraps `Transcriber` interface for speech-to-text
- `NewSTTService(client Transcriber)` creates service
- `Transcribe(ctx, audioData []byte)` returns `*TranscriptionResult, error`
- Uses `slog.Logger` for structured logging

##### TTSService (tts_service.go)
- Wraps `Synthesizer` interface for text-to-speech synthesis
- `NewTTSService(client Synthesizer)` creates service
- `Synthesize(ctx, text string)` returns `*SynthesisResult, error`

##### VADService (vad_service.go)
- Wraps `SpeechDetector` interface for voice activity detection
- `NewVADService(client SpeechDetector)` creates service
- `DetectSpeech(ctx, audioData []byte)` returns `*VADResult, error`

##### Design Pattern
All three services follow the same interface+wrapper pattern:
1. Define a provider interface (`Transcriber`, `Synthesizer`, `SpeechDetector`)
2. Service struct holds the provider client and a logger
3. Constructor takes the provider, returns the service
4. Methods delegate to provider with error passthrough

This allows swapping AI providers (OpenAI Whisper, Google STT, etc.) without changing callers. But no providers are plugged in yet.

### Configuration

#### Environment Variables

| Variable | Description | Default |
|----------|-------------|---------|
| `TURN_SECRET` | **Required.** Shared secret for HMAC-SHA1 credential generation. Bridge refuses to start if empty. | _(none, must be set)_ |
| `TURN_HOST` | TURN relay hostname or IP | `matrix.armorclaw.com` |
| `TURN_PORT` | TURN relay port | `3478` |
| `TURN_PROTOCOL` | Transport protocol: `udp`, `tcp`, or `tls` | `udp` |
| `TURN_REALM` | Authentication realm | `armorclaw` |
| `TURN_DEFAULT_TTL` | Credential lifetime | `10m` |
| `TURN_MAX_TTL` | Maximum credential lifetime | `1h` |

#### Budget Configuration

| Setting | Default | Description |
|---------|---------|-------------|
| `DefaultTokenLimit` | 100,000 | Max tokens per call |
| `DefaultDurationLimit` | 30 min | Max call duration |
| `WarningThreshold` | 0.8 (80%) | Emit warning at this usage fraction |
| `HardStop` | true | Terminate call when limit is hit |
| `DefaultLifetime` | 10 min | Default session TTL |
| `MaxLifetime` | 1 hour | Maximum session TTL |

#### Audio Configuration

| Setting | Default | Description |
|---------|---------|-------------|
| `SampleRate` | 48,000 Hz | Opus standard sample rate |
| `Channels` | 1 (mono) | Voice calls use mono |
| `BitDepth` | 16 bit | PCM16 format |
| `FrameSize` | 960 samples | 20 ms frames at 48 kHz |
| `BufferSize` | 10 frames | 200 ms jitter buffer |
| `Bitrate` | 64,000 bps | Opus target bitrate |
| `Complexity` | 5 (0-10) | Encoder complexity |
| `FEC` | enabled | Forward error correction |
| `DTX` | disabled | Discontinuous transmission |

### Integration Points

#### Matrix Rooms

Signaling uses the existing Matrix E2EE infrastructure. The Bridge sends SDP offers and answers as Matrix events, and ICE candidates are trickled through the same encrypted channel. Each voice session is bound to a Matrix room ID, and the `RequireMembership` config flag (on by default) ensures only room members can initiate calls.

#### Budget System

The `voice.BudgetTracker` integrates with the Bridge's security logger. Every session start, end, budget warning, and enforcement action is logged as a security event. Token usage from the AI model's input (speech-to-text) and output (text-to-speech) is tracked per call and checked against limits every 30 seconds.

#### Agent Runtime

Audio frames flow between the Bridge and agent containers through the `AudioPipeline`. The pipeline creates a `StreamPair` (inbound + outbound) for each session. The agent container receives decoded PCM audio and produces PCM audio back, without needing to know about WebRTC, Opus, or TURN. The Bridge handles all codec and protocol work.

#### TURN Infrastructure

The `turn.Manager` generates ephemeral credentials scoped to individual sessions. The WebRTC engine calls `SetTURNServersWithManager` before each peer connection is created, getting fresh TURN URLs and credentials that expire with the session. This avoids static credentials and limits the blast radius of any leak.
