# Go Bridge Reference

The Go Bridge is the control plane for ArmorClaw. It orchestrates agent containers, manages encrypted credentials, routes AI provider requests, runs the BlindFill pipeline, and bridges Matrix Conduit to the outside world.

This document covers the public API surface, internal architecture, and key subsystems. For ArmorChat Android details, see `armorchat-android.md`. For Rust/Python sidecar internals, see `architecture.md`.

---

## Table of Contents

1. [Binaries](#binaries)
2. [Package Map (68 packages)](#package-map)
3. [RPC API (95–109 methods)](#rpc-api)
4. [Agent State Machine](#agent-state-machine)
5. [Event Flow](#event-flow)
6. [Internal Packages (17 packages)](#internal-packages)
7. [Matrix Conduit Relationship](#matrix-conduit-relationship)
8. [Quick Reference](#quick-reference)

---

## Binaries

Five binaries live under `bridge/cmd/`. Each compiles to a standalone executable.

### `bridge`

```
bridge/cmd/bridge/main.go
```

The main Bridge server. Subcommands:

| Command | Purpose |
|---------|---------|
| `init` | Generate example config at `~/.armorclaw/config.toml` |
| `validate` | Validate config file |
| `setup` | Interactive Huh? TUI wizard for local config |
| `container-setup` | TUI wizard + delegates to `container-setup.sh` for Docker Compose |
| `daemon start/stop/restart/status/logs` | Background daemon management via PID file |
| `add-key` | Store an API key in the encrypted keystore |
| `list-keys` | List stored credentials |
| `start` | Start an agent container (legacy, delegates to RPC) |
| `generate-qr` | Produce a deep link + QR code for ArmorChat provisioning |
| `start-agent` | Start an AI agent (OpenClaw, assistant, custom) |
| `completion` | Generate bash/zsh shell completions |
| `readmin` | Admin password reset mode |
| `version` | Print version info |

Default behavior (no subcommand): starts the RPC server on a Unix domain socket at `/run/armorclaw/bridge.sock`.

### `bootstrap-admin`

```
bridge/cmd/bootstrap-admin/main.go
```

Runs inside the quickstart container at first boot. Connects to the local Conduit homeserver on port 6167, creates an admin user via the shared-secret registration API with a randomized username (`armor-admin-XXXXXXXX`). Writes the password to stdout once and creates `/var/lib/armorclaw/.bootstrapped` as a guard flag.

### `mta-recv`

```
bridge/cmd/mta-recv/main.go
```

Postfix pipe transport daemon. Receives raw email on stdin (sender, recipient, queue_id from args), forwards it over a Unix domain socket to `/run/armorclaw/email-ingest.sock` for Bridge processing. Exit codes follow Postfix conventions: 0 (delivered), 75 (temp fail, retry), 65 (permanent fail). Max email size: 26 MB.

### `migrate-templates`

```
bridge/cmd/migrate-templates/main.go
```

One-shot migration tool for workflow template JSON files. Adds `"input": {}` to `WorkflowStep` objects that are missing it. Idempotent. Supports `--dry-run` and `--validate` flags. Processes files or directories recursively.

### `gen-models`

```
bridge/cmd/gen-models/main.go
```

Generates `docs/reference/models.md` from the provider registry. Optional Catwalk URL for enrichment. Output path defaults to `<repo-root>/docs/reference/models.md`.

---

## Package Map

68 packages under `bridge/pkg/`. Organized by functional category.

### Core Orchestration

| Package | Description |
|---------|-------------|
| `agent` | Agent state machine (11 states), CDP inference, status events for Matrix |
| `secretary` | Workflow engine: templates, step execution, BlindFill integration, event reader, parallel orchestration |
| `studio` | Agent Studio: observable containers, learned skills, container lifecycle |
| `runtime` | Agent runtime configuration and execution environment |

### Cryptography and Security

| Package | Description |
|---------|-------------|
| `crypto` | Cryptographic primitives for key derivation, encryption, signing |
| `pii` | PII detection, risk classification, BlindFill request/approval pipeline |
| `trust` | Trust layer: risk scoring, approval gates for sensitive operations |
| `keystore` | SQLCipher-backed encrypted credential storage |
| `audit` | Audit logging for all operations (compliance trail) |
| `yara` | YARA-based content disarm and reconstruction (CDR) |
| `governor` | Rate limiting and request throttling |
| `enforcement` | Policy enforcement engine for security rules |
| `lockdown` | Container lockdown and security hardening |
| `security` | Security utilities, TLS configuration, cipher suites |

### Communication

| Package | Description |
|---------|-------------|
| `rpc` | JSON-RPC 2.0 server over Unix domain socket or TCP |
| `websocket` | WebSocket server for real-time event streaming to ArmorChat |
| `http` | HTTP server for health checks, metrics, API endpoints |
| `eventbus` | Fire-and-forget event delivery (Push Bus) to WebSocket clients |
| `eventlog` | Durable event log with sequence numbers for replay |
| `socket` | Unix domain socket management |
| `matrix` | Matrix client: login, send/receive, room management |
| `matrixcmd` | Matrix command parser for admin commands (!agent, !ai, etc.) |
| `push` | Push notification gateway (Sygnal integration) |
| `appservice` | Matrix Application Service protocol implementation |

### Browser and Sidecars

| Package | Description |
|---------|-------------|
| `browser` | Browser automation: navigate, fill, click, status, NavChart types |
| `sidecar` | Sidecar routing: 3-layer document pipeline (native/compound/strict) |
| `toolsidecar` | Tool sidecar lifecycle management |

### Email

| Package | Description |
|---------|-------------|
| `email` | Email pipeline: approval workflow, ingestion, MTA integration |

### Voice and WebRTC

| Package | Description |
|---------|-------------|
| `voice` | Voice processing: session management, start/stop/status RPC handlers |
| `webrtc` | WebRTC signaling and peer connection management (partial, voice routes via OpenAI cloud) |
| `audio` | Audio processing utilities |
| `turn` | TURN/STUN server configuration for NAT traversal |

### AI and Providers

| Package | Description |
|---------|-------------|
| `providers` | Provider registry: OpenAI, Anthropic, OpenRouter, xAI, DeepSeek, etc. |
| `interfaces` | Provider interface definitions (chat completion, streaming) |
| `mcp` | Model Context Protocol integration |
| `translator` | Provider-specific request/response translation |

### Provisioning and Setup

| Package | Description |
|---------|-------------|
| `provisioning` | QR-based device provisioning, setup tokens, claim flow |
| `qr` | QR code generation for ArmorChat deep links |
| `setup` | Setup wizard, Docker preflight checks, error handling |
| `discovery` | mDNS service discovery and network detection |
| `config` | TOML configuration loading, validation, defaults |

### Agent Management

| Package | Description |
|---------|-------------|
| `skills` | Skill registry: execution, allow/block lists, learned skill extraction |
| `docker` | Docker client wrapper for container lifecycle |
| `secrets` | Secret management for containers (env injection) |

### Teams and Governance

| Package | Description |
|---------|-------------|
| `team` | Team management and membership |
| `capability` | Capability-based access control |
| `invite` | Invite code generation, validation, revocation |
| `permissions` | Permission matrix and role-based access |
| `budget` | Budget tracking and alerts for AI provider spending |
| `license` | License enforcement and validation |

### Infrastructure

| Package | Description |
|---------|-------------|
| `health` | Health check aggregation (Bridge, Matrix, Docker) |
| `logger` | Structured logging, security audit logger |
| `errors` | Error system with component tracker, error codes, user-facing messages |
| `ttl` | TTL-based token management for sidecar communication |
| `cache` | In-memory caching utilities |
| `notification` | Notification dispatch (Matrix, push, email) |
| `dashboard` | Dashboard data aggregation for web UI |
| `admin` | Admin user management and operations |
| `ghost` | Ghost user management for Matrix AppService |
| `plugin` | Plugin system for extensibility |
| `recovery` | Crash recovery and state restoration |
| `systemd` | Systemd service management helpers |
| `vault` | Vault integration for secret storage |
| `adapters` | Platform adapters (Matrix, Slack, Discord) |
| `sso` | Single sign-on integration |
| `securerandom` | Cryptographically secure random generation |
| `queue` | In-memory request queue for graceful degradation |
| `ffi` | Foreign function interface for sidecar communication |
| `auth` | Authentication middleware and token validation |

---

## RPC API

95–109 RPC Methods (v1.0.0), registered in `bridge/pkg/rpc/server.go` `registerHandlers()`. Protocol: JSON-RPC 2.0 over Unix domain socket (Native mode) or TCP (Sentinel/Cloudflare mode).

The method count varies by feature flags. Baseline: 95 methods with all flags off. Maximum: 109 with all flags enabled. Flag-dependent methods return error code `-32601` when their flag is disabled.

| Flag | Additional Methods |
|------|-------------------|
| `ZeroTrustKeystore` | +7 (`keystore.unseal`, `keystore.sealed`, `keystore.seal`, `keystore.extend_session`, `keystore.session_status`, `keystore.list_keys`, `keystore.delete_key`) |
| `VoicePipeline` | +3 (`voice.start_session`, `voice.stop_session`, `voice.status`) |
| `E2EEBackup` | +3 (`e2ee.create_backup`, `e2ee.delete_backup`, `e2ee.backup_exists`) |
| `MultiTabReplay` | +1 (`browser.replay_diagnostics`) |

### AI Chat (1 method)

| Method | Handler | Description |
|--------|---------|-------------|
| `ai.chat` | `handleAIChat` | Send a message to the AI provider and get a response |

### Browser (11–12 methods)

| Method | Handler | Description |
|--------|---------|-------------|
| `browser.navigate` | `handleBrowserNavigate` | Navigate to a URL |
| `browser.fill` | `handleBrowserFill` | Fill a form field (BlindFill injection point) |
| `browser.click` | `handleBrowserClick` | Click an element |
| `browser.status` | `handleBrowserStatus` | Get current browser state |
| `browser.wait_for_element` | `handleBrowserWaitForElement` | Block until element appears |
| `browser.wait_for_captcha` | `handleBrowserWaitForCaptcha` | Block until CAPTCHA is resolved |
| `browser.wait_for_2fa` | `handleBrowserWaitFor2FA` | Block until 2FA code is provided |
| `browser.complete` | `handleBrowserComplete` | Mark browser task as complete |
| `browser.fail` | `handleBrowserFail` | Mark browser task as failed |
| `browser.list` | `handleBrowserList` | List active browser sessions |
| `browser.cancel` | `handleBrowserCancel` | Cancel an in-progress browser operation |
| `browser.replay_diagnostics` | `handleBrowserReplayDiagnostics` | Get multi-tab replay diagnostic data (requires `MultiTabReplay` flag) |

### Bridge Control (10 methods)

| Method | Handler | Description |
|--------|---------|-------------|
| `bridge.start` | `handleBridgeStart` | Start a platform bridge |
| `bridge.stop` | `handleBridgeStop` | Stop a platform bridge |
| `bridge.status` | `handleBridgeStatus` | Get bridge status |
| `bridge.channel` | `handleBridgeChannel` | Bridge a Matrix room to a platform channel |
| `bridge.unchannel` | `handleUnbridgeChannel` | Remove a channel bridge |
| `bridge.list` | `handleListBridgedChannels` | List all bridged channels |
| `bridge.ghost_list` | `handleGhostUserList` | List ghost users |
| `bridge.appservice_status` | `handleAppServiceStatus` | Check AppService connection status |
| `bridge.e2ee_enable` | `handleE2EEEnable` | Enable end-to-end encryption |
| `bridge.e2ee_disable` | `handleE2EEDisable` | Disable end-to-end encryption |

### PII / BlindFill (9 methods)

| Method | Handler | Description |
|--------|---------|-------------|
| `pii.request` | `handlePIIRequest` | Request access to a PII field |
| `pii.approve` | `handlePIIApprove` | Approve a PII access request |
| `pii.deny` | `handlePIIDeny` | Deny a PII access request |
| `pii.status` | `handlePIIStatus` | Check PII request status |
| `pii.list_pending` | `handlePIIListPending` | List pending PII requests |
| `pii.stats` | `handlePIIStats` | Get PII usage statistics |
| `pii.cancel` | `handlePIICancel` | Cancel a PII request |
| `pii.fulfill` | `handlePIIFulfill` | Fulfill a PII request (inject secret) |
| `pii.wait_for_approval` | `handlePIIWaitForApproval` | Long-poll until PII request is resolved |

### Skills (14 methods)

| Method | Handler | Description |
|--------|---------|-------------|
| `skills.execute` | `handleSkillsExecute` | Execute a skill by name |
| `skills.list` | `handleSkillsList` | List enabled skills |
| `skills.get_schema` | `handleSkillsGetSchema` | Get a skill's parameter schema |
| `skills.allow` | `handleSkillsAllow` | Enable a skill |
| `skills.block` | `handleSkillsBlock` | Disable a skill |
| `skills.allowlist_add` | `handleSkillsAllowlistAdd` | Add IP/CIDR to skill allowlist |
| `skills.allowlist_remove` | `handleSkillsAllowlistRemove` | Remove IP/CIDR from allowlist |
| `skills.allowlist_list` | `handleSkillsAllowlistList` | List current allowlist entries |
| `skills.web_search` | `handleSkillsWebSearch` | Web search skill |
| `skills.web_extract` | `handleSkillsWebExtract` | Web content extraction skill |
| `skills.email_send` | `handleSkillsEmailSend` | Send email skill |
| `skills.slack_message` | `handleSkillsSlackMessage` | Slack messaging skill |
| `skills.file_read` | `handleSkillsFileRead` | File reading skill |
| `skills.data_analyze` | `handleSkillsDataAnalyze` | Data analysis skill |

### Matrix (5 methods)

| Method | Handler | Description |
|--------|---------|-------------|
| `matrix.status` | `handleMatrixStatus` | Matrix connection status |
| `matrix.login` | `handleMatrixLogin` | Login to Matrix homeserver |
| `matrix.send` | `handleMatrixSend` | Send a Matrix message |
| `matrix.receive` | `handleMatrixReceive` | Receive pending Matrix messages |
| `matrix.join_room` | `handleMatrixJoinRoom` | Join a Matrix room |

### Events (2 methods)

| Method | Handler | Description |
|--------|---------|-------------|
| `events.replay` | `handleEventsReplay` | Replay events from a cursor position |
| `events.stream` | `handleEventsStream` | Long-poll for new events |

### Studio (2 methods)

| Method | Handler | Description |
|--------|---------|-------------|
| `studio.deploy` | `handleStudio` | Deploy an agent to the studio |
| `studio.stats` | `handleStudioStats` | Get studio agent statistics |

### Keystore (7 methods, requires `ZeroTrustKeystore` flag)

| Method | Handler | Description |
|--------|---------|-------------|
| `keystore.unseal` | `handleKeystoreUnseal` | Unseal the keystore with a password (Argon2id verification) |
| `keystore.sealed` | `handleKeystoreSealed` | Check whether the keystore is currently sealed |
| `keystore.seal` | `handleKeystoreSeal` | Manually seal the keystore |
| `keystore.extend_session` | `handleKeystoreExtendSession` | Extend the auto-seal timer |
| `keystore.session_status` | `handleKeystoreSessionStatus` | Get current session status (time remaining, unseal count) |
| `keystore.list_keys` | `handleKeystoreListKeys` | List stored key identifiers |
| `keystore.delete_key` | `handleKeystoreDeleteKey` | Delete a stored key by identifier |

**Error codes:**

| Code | Name | When |
|------|------|------|
| `-32001` | `invalid_password` | Password verification failed |
| `-32003` | `already_unsealed` | Attempted to unseal an already-unsealed keystore |
| `-32005` | `keystore_sealed` | Operation requires unsealed keystore |
| `-32006` | `rate_limited` | Too many failed unseal attempts |

### Voice (3 methods, requires `VoicePipeline` flag)

| Method | Handler | Description |
|--------|---------|-------------|
| `voice.start_session` | `handleVoiceStartSession` | Start a voice session |
| `voice.stop_session` | `handleVoiceStopSession` | Stop an active voice session |
| `voice.status` | `handleVoiceStatus` | Get voice session status |

**Error codes:**

| Code | Name | When |
|------|------|------|
| `-32007` | `voice_not_configured` | Voice pipeline is not configured |

### E2EE Backup (3 methods, requires `E2EEBackup` flag)

| Method | Handler | Description |
|--------|---------|-------------|
| `e2ee.create_backup` | `handleE2EECreateBackup` | Create an E2EE key backup |
| `e2ee.delete_backup` | `handleE2EEDeleteBackup` | Delete an existing E2EE key backup |
| `e2ee.backup_exists` | `handleE2EEBackupExists` | Check whether an E2EE key backup exists |

Note: `e2ee.restore_backup` does NOT exist. Key restoration is out of scope for v1.0.0.

### Keystore (1 method)

| Method | Handler | Description |
|--------|---------|-------------|
| `store_key` | `handleStoreKey` | Store a credential in the encrypted keystore |

### Provisioning (2 methods)

| Method | Handler | Description |
|--------|---------|-------------|
| `provisioning.start` | `handleProvisioningStart` | Generate a QR provisioning token |
| `provisioning.claim` | `handleProvisioningClaim` | Claim a provisioning token |

### Hardening (3 methods)

| Method | Handler | Description |
|--------|---------|-------------|
| `hardening.status` | `handleHardeningStatus` | Check security hardening status |
| `hardening.ack` | `handleHardeningAck` | Acknowledge a hardening advisory |
| `hardening.rotate_password` | `handleHardeningRotatePassword` | Rotate admin password |

### Health (1 method)

| Method | Handler | Description |
|--------|---------|-------------|
| `health.check` | `handleHealthCheck` | Aggregate health check (Bridge, Matrix, Docker) |

### Mobile (1 method)

| Method | Handler | Description |
|--------|---------|-------------|
| `mobile.heartbeat` | `handleMobileHeartbeat` | Record mobile app heartbeat |

### Container (2 methods)

| Method | Handler | Description |
|--------|---------|-------------|
| `container.terminate` | `handleTerminateContainer` | Terminate a running container |
| `container.list` | `handleListContainers` | List running containers |

### Blocker (1 method)

| Method | Handler | Description |
|--------|---------|-------------|
| `resolve_blocker` | `handleResolveBlocker` | Resolve a workflow blocker with user input |

### Email Approval (4 methods)

| Method | Handler | Description |
|--------|---------|-------------|
| `approve_email` | `handleApproveEmail` | Approve an email action |
| `deny_email` | `handleDenyEmail` | Deny an email action |
| `email_approval_status` | `handleEmailApprovalStatus` | Check email approval status |
| `email.list_pending` | `handleEmailListPending` | List pending email approvals |

### Account (1 method)

| Method | Handler | Description |
|--------|---------|-------------|
| `account.delete` | `handleAccountDelete` | Delete a user account |

### Secretary / Workflow (13 methods)

All route through `handleSecretaryMethod` which dispatches to the workflow engine.

| Method | Description |
|--------|-------------|
| `secretary.start_workflow` | Start a workflow from a template |
| `secretary.get_workflow` | Get workflow instance status |
| `secretary.cancel_workflow` | Cancel a running workflow |
| `secretary.create_workflow` | Create a new workflow instance |
| `secretary.advance_workflow` | Advance workflow to next step |
| `secretary.list_templates` | List available workflow templates |
| `secretary.create_template` | Create a new workflow template |
| `secretary.get_template` | Get a template definition |
| `secretary.delete_template` | Delete a template |
| `secretary.update_template` | Update a template |
| `secretary.is_running` | Check if any workflow is active |
| `secretary.get_active_count` | Count active workflows |
| `secretary.shutdown` | Graceful shutdown of the workflow engine |

### Tasks (4 methods)

All route through `handleSecretaryMethod` to the task scheduler.

| Method | Description |
|--------|-------------|
| `task.create` | Create a scheduled task |
| `task.list` | List tasks |
| `task.cancel` | Cancel a task |
| `task.get` | Get task details |

### Device (4 methods)

| Method | Handler | Description |
|--------|---------|-------------|
| `device.list` | `handleDeviceList` | List registered devices |
| `device.get` | `handleDeviceGet` | Get device details |
| `device.approve` | `handleDeviceApprove` | Approve a new device |
| `device.reject` | `handleDeviceReject` | Reject a device registration |

### Invite (4 methods)

| Method | Handler | Description |
|--------|---------|-------------|
| `invite.list` | `handleInviteList` | List invite codes |
| `invite.create` | `handleInviteCreate` | Generate an invite code |
| `invite.revoke` | `handleInviteRevoke` | Revoke an invite code |
| `invite.validate` | `handleInviteValidate` | Validate an invite code |

---

## Agent State Machine

Defined in `bridge/pkg/agent/state.go` and `state_machine.go`. The state machine tracks agent operational status on the Bridge side. It is a Bridge-side library: states advance based on container lifecycle events and inference, not agent-reported phase transitions.

### The 11 States

```
IDLE              Not performing any task
INITIALIZING      Starting up, loading config
BROWSING          Actively navigating to a URL
FORM_FILLING      Filling form fields
AWAITING_CAPTCHA  Needs human CAPTCHA solving
AWAITING_2FA      Needs a 2FA code
AWAITING_APPROVAL Waiting for BlindFill approval
PROCESSING_PAYMENT Submitting a payment
ERROR             Recoverable error
COMPLETE          Task finished successfully
OFFLINE           Agent not reachable
```

### 4 Terminal States

Terminal states require external action to leave:

- `AWAITING_CAPTCHA` (user must solve)
- `AWAITING_2FA` (user must provide code)
- `AWAITING_APPROVAL` (user must approve/deny PII)
- `OFFLINE` (system must restart agent)

### Transition Graph

```
IDLE           -> INITIALIZING, ERROR
INITIALIZING   -> BROWSING, FORM_FILLING, ERROR, IDLE
BROWSING       -> FORM_FILLING, AWAITING_CAPTCHA, AWAITING_2FA,
                  AWAITING_APPROVAL, ERROR, COMPLETE, IDLE
FORM_FILLING   -> AWAITING_APPROVAL, PROCESSING_PAYMENT,
                  AWAITING_CAPTCHA, AWAITING_2FA, ERROR,
                  COMPLETE, BROWSING, IDLE
AWAITING_CAPTCHA -> BROWSING, FORM_FILLING, ERROR, IDLE
AWAITING_2FA   -> BROWSING, FORM_FILLING, PROCESSING_PAYMENT, ERROR, IDLE
AWAITING_APPROVAL -> FORM_FILLING, PROCESSING_PAYMENT, ERROR, IDLE
PROCESSING_PAYMENT -> COMPLETE, ERROR, AWAITING_2FA, IDLE
ERROR          -> IDLE, INITIALIZING
COMPLETE       -> IDLE
OFFLINE        -> INITIALIZING
```

Validation: `ValidateTransition()` returns an error for disallowed transitions. `ForceTransition()` bypasses validation for recovery scenarios.

### State Inference Engine

File: `bridge/pkg/agent/state_inference.go`

Three inference sources, applied in priority order:

| Priority | Source | Mechanism | States Inferred |
|----------|--------|-----------|-----------------|
| 1 | Workflow engine | Side-channel status (`captcha`, `twofa`, `payment`, `offline`) | AWAITING_CAPTCHA, AWAITING_2FA, PROCESSING_PAYMENT, OFFLINE |
| 2 | Exit codes | Workflow exit semantics | COMPLETE (exit 0), ERROR (exit nonzero) |
| 3 | CDP events | Chrome DevTools Protocol events from Jetski | BROWSING (Page.frameNavigated), FORM_FILLING (DOM.focus on input), INITIALIZING (Runtime.executionContextCreated) |

When `AWAITING_APPROVAL`, the inference engine never transitions away based on CDP events. Approval state is managed exclusively by the PII RPC methods.

### Observability

- Transition log: ring buffer of last 100 transitions with `from`, `to`, `timestamp`, `inferred_from`
- History: last N events (configurable, default 100) for reconnection support
- Subscribers: fan-out to multiple channels, non-blocking sends
- StatusEvent emits to Matrix as `com.armorclaw.agent.status`

---

## Event Flow

The Bridge has two event buses with different semantics.

### Push Bus (`pkg/eventbus`)

Fire-and-forget delivery to WebSocket clients. No cursor/sequence semantics, at-most-once delivery.

```
Source (vault, email, etc.)
  -> EventBus.Publish()
  -> WebSocket clients (ArmorChat)
  -> In-process handlers (RegisterBridgeHandler)
```

Event types: `agent.*`, `workflow.*`, `hitl.*`, `budget.*`, `platform.*`, `email.*`, `bridge.*`, `matrix.*`.

### Stream Bus (`internal/events`)

Durable ring buffer with sequence numbers and cursor-based replay. Used for Matrix sync, workflow events, and RPC long-poll streaming.

```
Matrix sync
  -> MatrixEventBus.Publish()
  -> Ring buffer (1024 events default)
  -> WaitForEvents(cursor) -> subscribers
  -> RPC events.stream / events.replay
```

### Container Event Pipeline

Agent containers write structured events to `_events.jsonl` inside their state directory. The Bridge tails this file during execution.

```
Container (_events.jsonl)
  -> EventReader (500ms poll, incremental byte offset)
  -> Soft 10MB cap (stops tailing with warning)
  -> Secretary orchestrator (event routing)
  -> MatrixEventBus (progress events)
  -> Matrix room (m.notice messages)
  -> ArmorChat (/sync)
```

11 container event types: STEP, FILE_READ, FILE_WRITE, FILE_DELETE, COMMAND_RUN, OBSERVATION, BLOCKER, ERROR, ARTIFACT, PROGRESS, CHECKPOINT. Lines enforced to PIPE_BUF (4096 bytes).

After task completion: `_events.jsonl` and state directory are purged (parse, cleanup, notify ordering).

---

## Internal Packages

17 packages under `bridge/internal/`. These contain implementation details not exported for external use.

| Package | Description |
|---------|-------------|
| `adapter` | Matrix adapter: translates between Bridge RPC and Matrix client protocol |
| `agent` | Internal agent management (container spawning, health monitoring) |
| `ai` | AI provider routing: model selection, streaming, fallback logic |
| `cache` | Internal caching layer for provider responses, template caching |
| `capability` | Capability resolution and enforcement engine |
| `events` | Stream Bus: ring buffer with cursor-based polling (MatrixEventBus) |
| `executor` | Step executor for workflow engine |
| `memory` | Conversation memory management for AI context windows |
| `metrics` | Prometheus metrics collection and export |
| `petg` | PETG (Privacy-Enhanced Token Gateway) implementation |
| `queue` | Internal work queue for background task processing |
| `router` | Request routing: method dispatch, middleware chain |
| `sdtw` | SDTW (Secure Direct Tunnel WebSocket) platform bridge protocol |
| `skills` | Skill definitions, execution sandbox, registry |
| `speculative` | Speculative execution for parallel workflow steps |
| `trace` | Distributed tracing instrumentation |
| `wizard` | Huh? TUI wizard for setup and container-setup commands |

---

## Matrix Conduit Relationship

The Bridge is **not embedded** inside Matrix Conduit. It operates as a **Matrix Application Service** (AppService).

### Architecture

```
ArmorChat (mobile)
  |
  | E2EE Matrix protocol
  v
Matrix Conduit homeserver (port 6167)
  |
  | AppService HTTP push (transactions)
  v
Bridge AppService listener (port configurable)
  |
  | Internal processing
  v
Agent containers, browser, AI providers
```

### How It Works

1. **Conduit** is the Matrix homeserver. It handles user auth, E2EE key exchange, room state, and message routing.

2. **Bridge** registers as an AppService with Conduit at startup. The registration includes:
   - AppService ID and token
   - Sender localpart (e.g., `_bridge`)
   - URL where Conduit should push events
   - Namespace patterns for ghost users

3. **Conduit pushes events** to the Bridge via HTTP transactions. The AppService handler (`pkg/appservice`) receives batches of Matrix events and routes them to the appropriate subsystem.

4. **Ghost users**: The Bridge can create virtual Matrix users (ghosts) for platform bridging (SDTW). Ghosts exist in Conduit but are controlled by the Bridge.

5. **Clients connect directly to Conduit** (Zero-Trust). The Bridge never handles user crypto or E2EE keys.

### Key AppService Files

| File | Purpose |
|------|---------|
| `pkg/appservice/appservice.go` | AppService protocol implementation, HTTP listener, event handler |
| `pkg/appservice/bridge.go` | Bridge manager for channel bridging |
| `pkg/appservice/client.go` | Matrix client with AppService authentication |
| `internal/adapter/` | Matrix adapter translating between Bridge and Matrix protocol |
| `internal/sdtw/` | SDTW platform bridge protocol for Slack, Discord, etc. |

---

## Quick Reference

"I need to work on X, where do I go?"

| I need to... | Go to package | Key file(s) |
|-------------|---------------|-------------|
| Add a new RPC method | `pkg/rpc` | `server.go` (registerHandlers map) |
| Change how agents track state | `pkg/agent` | `state.go`, `state_machine.go`, `state_inference.go` |
| Modify the BlindFill pipeline | `pkg/pii` | `pkg/secretary/blindfill.go`, `blindfill_integration.go` |
| Add a new AI provider | `pkg/providers` | `internal/ai` for routing |
| Change workflow execution | `pkg/secretary` | `orchestrator.go`, `orchestrator_integration.go` |
| Modify browser automation | `pkg/browser` | `browser.go`, `chart_types.go` |
| Change email approval flow | `pkg/email` | `pkg/rpc/server.go` (approve_email, deny_email handlers) |
| Add a Matrix command | `pkg/matrixcmd` | Command parser, then `internal/adapter` for execution |
| Change event streaming | `pkg/eventbus` (push) | `internal/events` (stream) |
| Modify the AppService | `pkg/appservice` | `appservice.go`, `bridge.go` |
| Work on QR provisioning | `pkg/provisioning` | `pkg/qr` for code generation |
| Change container security | `pkg/docker` | Container creation flags, `pkg/lockdown` |
| Add a new skill | `internal/skills` | `pkg/skills` for public API |
| Modify container event pipeline | `pkg/secretary` | `event_reader.go`, `orchestrator_integration.go` |
| Work on voice/WebRTC | `pkg/voice`, `pkg/webrtc` | `pkg/turn` for TURN config |
| Change the setup wizard | `internal/wizard` | `cmd/bridge/main.go` (setup command) |
| Debug a workflow issue | `pkg/secretary` | `orchestrator.go`, `types.go` |
| Change budget tracking | `pkg/budget` | Alert events in `pkg/eventbus/events.go` |
| Work on E2EE | `pkg/crypto` | `bridge.e2ee_enable/disable` in RPC |
| Add device management | `pkg/provisioning` | `device.*` RPC handlers in `server.go` |
| Work on invite system | `pkg/invite` | `invite.*` RPC handlers |
| Change YARA scanning | `pkg/yara` | Content disarm and reconstruction |
| Debug sidecar routing | `pkg/sidecar` | 3-layer routing: native, compound, strict |
| Modify the daemon | `cmd/bridge/main.go` | `daemonStart`, `daemonStop`, `daemonStatus` |
| Change the MTA pipeline | `cmd/mta-recv` | `pkg/email` for Bridge-side processing |
| Add a new workflow step type | `pkg/secretary` | `types.go`, `orchestrator.go` |
| Modify HITL approval | `pkg/trust` | `pkg/pii` for PII-specific approval |
| Work on the dashboard | `pkg/dashboard` | `pkg/http` for API endpoints |
| Change logging | `pkg/logger` | `pkg/errors` for structured errors |
