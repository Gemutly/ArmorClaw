# Communication Infrastructure

> Part of the [ArmorClaw System Documentation](armorclaw.md)

## Overview

**These are independent subsystems.** They do not share a common architecture. Each solves a distinct communication problem within ArmorClaw. They were initially grouped under "operational governance" but code analysis confirmed zero cross-dependencies between them. Treat each one as a standalone module with its own lifecycle, configuration, and error handling.

The six subsystems covered here:

| Subsystem | Package | Purpose |
|-----------|---------|---------|
| Push Notifications | `pkg/push/` | Mobile push via FCM, APNS, Web Push, and Matrix Sygnal |
| Single Sign-On | `pkg/sso/` | Enterprise SAML 2.0 and OIDC authentication |
| WebSocket Server | `pkg/websocket/` | Real-time event streaming to clients via EventBus adapter |
| Event Bus Internals | `pkg/eventbus/` | In-process pub/sub with optional durable log |
| Platform Adapters | `internal/adapter/` | Matrix and Slack message routing |
| Additional Platform Adapters | `internal/sdtw/` | Discord, Telegram, Teams, WhatsApp adapters |

## Subsystems

### Push Notifications (`pkg/push/`)

The push subsystem delivers mobile and browser notifications to ArmorClaw users. It centers on the `Gateway` type, which manages device registrations and routes notifications to platform-specific providers.

**Supported platforms:**

- **FCM** (Firebase Cloud Messaging) for Android and iOS via Google's infrastructure
- **APNS** (Apple Push Notification Service) for iOS devices
- **Web Push** (VAPID) for browser notifications using ECDSA P-256 key pairs
- **Unified Push** as an abstract platform identifier (constant defined, no provider implementation)

**How it works:**

1. Devices register with the `Gateway` via `RegisterDevice`, associating a user ID, platform, and device token.
2. The gateway maintains an in-memory map of user to device list, protected by a read-write mutex.
3. To send a notification, call `SendToUser` with a user ID and a `Notification` struct. The gateway fans out to all registered devices for that user, selecting the correct provider per device.
4. Each send attempt includes configurable retries (default 3) with a configurable delay (default 5 seconds).
5. The gateway keeps the last 1,000 `PushResult` entries for stats.

**Matrix Sygnal integration:**

The `CreateMatrixPushNotification` method converts a Matrix room event (room ID, event ID, sender, content) into a set of per-device `Notification` structs. This is the bridge between Matrix message events and native mobile push. The `SygnalURL` and `SygnalAPIKey` fields in `Config` allow the gateway to delegate delivery to a Sygnal push gateway when configured.

**Provider interface:**

Each provider implements `PushProvider` with three methods: `Send`, `ValidateToken`, and `Platform`. Token validation is platform-specific (FCM tokens are long strings, APNS tokens are exactly 64 hex characters, Web Push subscription endpoints are long URLs). A `MockProvider` exists for testing.

**Key types:** `Gateway`, `Notification`, `DeviceRegistration`, `PushResult`, `PushProvider` interface.

### Single Sign-On (`pkg/sso/`)

The SSO subsystem handles enterprise authentication via SAML 2.0 and OpenID Connect. It's managed by `SSOManager`, which wraps an `SSOProvider` implementation and a `StateStore` for OAuth state parameters.

**Authentication flow:**

1. `BeginAuth(redirect)` generates a cryptographic state parameter, stores it (with a 10-minute TTL), and returns the IdP authorization URL.
2. After the user authenticates at the IdP, the callback handler calls `HandleCallback(ctx, code, state)`.
3. The state is validated (one-time use, then deleted). The provider-specific handler processes the code or SAML response.
4. User attributes are extracted and mapped to ArmorClaw roles via `RoleMapping` config. Unmapped users get the default "user" role.
5. The resulting `SSOSession` is stored in memory and can be validated or refreshed.

**SAML 2.0 provider:**

Loads IdP metadata from a URL or file, builds `AuthnRequest` XML, and redirects to the IdP's SSO URL. Callbacks decode and validate the base64-encoded `SAMLResponse`, extract the `NameID` and assertion attributes. SAML sessions cannot be natively refreshed; users must re-authenticate. Supports Single Logout (SLO) when the IdP provides a SLO URL.

**OIDC provider:**

Discovers endpoints automatically from the issuer's `/.well-known/openid-configuration`. Supports PKCE (S256 challenge) for public clients. Token exchange, userinfo retrieval, and refresh token flows are all implemented. Sessions can be refreshed using the stored refresh token. Logout redirects to the IdP's end-session endpoint.

**Security measures:**

- State parameters are one-time use with automatic cleanup.
- Redirect URLs are validated against dangerous schemes (`javascript:`, `data:`, `vbscript:`) and control characters.
- Issuer URLs must use HTTPS except for localhost development.
- Client IDs are length-limited and checked for control characters.

**Key types:** `SSOManager`, `SSOSession`, `SSOProvider` interface, `SAMLProvider`, `OIDCProvider`, `StateStore`.

### WebSocket Server (`pkg/websocket/`)

> **Updated in v0.7.0**: The WebSocket server is no longer a stub. It now implements the `EventBroadcaster` interface, acting as an adapter between the EventBus and the HTTP server's existing `/ws` endpoint. See [EventBus–WebSocket Wiring](#eventbus--websocket-wiring-v070) below.

The `Server` type accepts a `Config` with address, path, allowed origins, max connections, inactivity timeout, and handler callbacks. The `Broadcast` method sends serialized events to all connected WebSocket clients.

The event bus (`pkg/eventbus`) consumes this package. When `WebSocketEnabled` is true in the EventBus config, published bridge events are forwarded to the WebSocket server for real-time streaming to connected clients.

**Key types:** `Server`, `Config`, `EventBroadcaster` interface, `MessageHandler`, `ConnectHandler`, `DisconnectHandler`.

#### EventBus–WebSocket Wiring (v0.7.0)

The EventBus publishes events through the HTTP server's existing `/ws` endpoint using an adapter pattern:

```
EventBus.PublishBridgeEvent()
  → websocketServer.Broadcast(data)        // EventBroadcaster interface
    → http.Server.BroadcastEvent()          // implements EventBroadcaster
      → gorilla/websocket WriteJSON         // to each connected client
```

- `websocket.EventBroadcaster` is an interface in `pkg/websocket/` that avoids circular imports between `eventbus` and `http` packages
- `http.Server.BroadcastEvent()` implements this interface, sending JSON-framed events to all clients in the `clients` map
- `EventBus.sendToSubscriber()` was previously discarding event data (`_ = data`). It now forwards data to the broadcaster when wired
- The crash-only `log.Fatalf` in `eventbus.Start()` is preserved — if the broadcaster fails to initialize when WebSocket is enabled, the Bridge halts
- Wire-up happens in `bridge/cmd/bridge/main.go` via `eventBus.SetBroadcaster(httpsServer)` before `eventBus.Start()`

### Event Bus Internals (`pkg/eventbus/`)

The event bus is the in-process pub/sub backbone for real-time Matrix event distribution. It enables containers and internal components to receive events without polling.

> The main doc's Event Bus Patterns section covers event types (matrix.message, agent.started, workflow.progress, hitl.pending, budget.alert, etc.). This section covers the internal mechanism.

**Core mechanism:**

1. Subscribers call `Subscribe(filter)` and receive a `Subscriber` with a buffered channel (capacity 100).
2. Publishers call `Publish(event)` with a `MatrixEvent`. The bus walks all subscribers, checks each filter (room ID, sender ID, event type), and sends matching events to subscriber channels.
3. If a subscriber's channel is full (slow consumer), the event is dropped and logged. The bus does not block publishers on slow subscribers.
4. A background goroutine cleans up subscribers inactive for more than 30 minutes.
5. Optionally, a durable log (`eventlog.Log`) appends every published event for replay or auditing.
6. Optionally, an embedded WebSocket server streams events to external clients via the `EventBroadcaster` adapter wired to the HTTP server's `/ws` endpoint (implemented in v0.7.0).

**Filter model:**

`EventFilter` has three optional fields: `RoomID`, `SenderID`, and `EventType` (slice). Empty fields match everything. All filter conditions must match (AND logic).

**Bridge events:**

`PublishBridgeEvent` handles non-Matrix events (agent, workflow, HITL, budget, platform, email). These events implement the `BridgeEvent` interface (`EventType()`, `Timestamp()`, `ToJSON()`). They are wrapped, serialized, and broadcast to WebSocket clients via the `EventBroadcaster` adapter when the server is enabled. Broadcast failures are logged but don't fail the publish operation.

**In-process handlers**: `RegisterBridgeHandler(eventType, handler)` registers a callback for a specific event type. Handlers are dispatched in goroutines with panic recovery (panics are logged via securityLog). This is the mechanism used by EmailDispatcher to receive `email.received` events.

**Error handling:**

The `errors.go` file defines a structured error system. Every error has a domain, code, severity, message, source location, stack trace, and optional context hints. Error codes are organized by category:

| Range | Category | Examples |
|-------|----------|---------|
| E001-E099 | Publisher | nil event, wrap failed, serialize failed, broadcast failed |
| E101-E199 | Subscriber | not found, inactive, channel full, closed |
| E201-E299 | WebSocket | not enabled, connect failed, message failed |
| E301-E399 | Filter | invalid filter |

Each error code is registered in `ErrorRegistry` with a description and resolution hint. The `IsErrorCode` and `AsError` helpers integrate with Go's `errors.Is`/`errors.As` patterns.

**Key types:** `EventBus`, `Subscriber`, `EventFilter`, `MatrixEventWrapper`, `EventError`, `ErrorCode`, `ErrorDomain`.

#### Dual-Bus Architecture

The Bridge runs two separate event buses with different delivery semantics. They do not share code, state, or configuration. Each one exists because its consumers need fundamentally different guarantees.

**Push Bus** (`pkg/eventbus`): fire-and-forget delivery. Events are pushed to subscriber channels and WebSocket clients as they arrive. If a subscriber is slow, the event is dropped. There are no sequence numbers, no cursors, and no replay. This works well for vault events and email events, where missing one update is acceptable and consumers care about the latest state, not a gap-free history. `RegisterBridgeHandler` is the main registration point for in-process consumers.

**Stream Bus** (`internal/events`): ring buffer with cursor-based polling. Every published event gets a monotonically increasing sequence number. Consumers call `WaitForEvents(ctx, cursor)` to tail the buffer from their last seen position, reading batches of up to 128 events. Slow consumers can replay missed events by re-requesting from an earlier cursor. This is the bus that Matrix sync events, workflow progress, agent status updates, and RPC long-poll (ArmorChat streaming) flow through. The ordering guarantee matters because these consumers need a consistent view of what happened and when.

| Aspect | Push Bus (`pkg/eventbus`) | Stream Bus (`internal/events`) |
|--------|---------------------------|-------------------------------|
| Delivery model | Fire-and-forget to channels | Cursor-based polling from ring buffer |
| Consumers | WebSocket clients, in-process handlers | Long-poll RPC (ArmorChat), Matrix sync, workflow, agent status |
| Ordering | None | Monotonic sequence numbers |
| Replay | No | Yes, via cursor re-read |
| Backpressure | Drop on slow consumer, log a warning | Skip slow channel subscribers, buffer remains readable |
| Primary registration | `RegisterBridgeHandler(eventType, handler)` | `Subscribe()` returns channel, `GetEventsAfter(cursor)` for batched reads |
| Use cases | Vault events, email events | Matrix sync, workflow progress, agent status, RPC streaming |

> See package-level doc comments in `bridge/pkg/eventbus/eventbus.go` and `bridge/internal/events/matrix_event_bus.go` for the authoritative source-level descriptions.

### Workflow Events (`pkg/secretary/`)

The orchestrator events system (`orchestrator_events.go`) defines workflow lifecycle events that flow through the `MatrixEventBus`. These events are emitted by `WorkflowEventEmitter` during container execution and consumed by the Matrix adapter's `processEvents()` method, which routes them to Matrix rooms.

**Core workflow events** (existed before v3):

| Event Type | Constant | When Emitted |
|------------|----------|-------------|
| `workflow.started` | `WorkflowEventStarted` | Workflow begins execution |
| `workflow.progress` | `WorkflowEventProgress` | Step progress update |
| `workflow.blocked` | `WorkflowEventBlocked` | Workflow hits a human-in-the-loop blocker |
| `workflow.completed` | `WorkflowEventCompleted` | Workflow finishes successfully |
| `workflow.failed` | `WorkflowEventFailed` | Workflow fails (may be recoverable) |
| `workflow.cancelled` | `WorkflowEventCancelled` | Workflow is cancelled by user or system |

**Step-level events** (new in v3):

These events are derived from container `StepEvent` objects (read from `_events.jsonl` files) and published by the `WorkflowEventEmitter` through the `MatrixEventBus`.

| Event Type | Constant | When Emitted | Payload |
|------------|----------|-------------|---------|
| `workflow.step_progress` | `WorkflowEventStepProgress` | Container emits a step progress update | `progress` (float from `detail.percent`), `step_name`, `timestamp`, `status: running`, `metadata.progress_detail` with `event_seq`, `event_type`, `step_name`, `elapsed_ms`, `detail` |
| `workflow.step_error` | `WorkflowEventStepError` | Container step fails | `error` (step name), `timestamp`, `status: failed`, `metadata` with `event_seq`, `event_type`, `detail` |
| `workflow.blocker_warning` | `WorkflowEventBlockerWarning` | Container encounters a blocker condition | `status: blocked`, `timestamp`, `metadata` with `blocker_type`, `message`, `event_seq`, `event_type` |

**Event structure:**

All workflow events share the `WorkflowEvent` struct:

```
WorkflowID  string                 // The running workflow
TemplateID  string                 // Template that spawned it (optional)
Status      WorkflowStatus         // running, completed, failed, blocked, cancelled
StepID      string                 // Current step (optional)
StepName    string                 // Human-readable step name (optional)
Progress    float64                // 0.0 to 1.0 (optional)
Timestamp   int64                  // Unix milliseconds
Error       string                 // Error message (optional)
Recoverable bool                   // Whether a failure is recoverable
Reason      string                 // Blocker or cancellation reason (optional)
Result      string                 // Completion result (optional)
Duration    int64                  // Total duration in milliseconds (optional)
Metadata    map[string]interface{} // Extra context
```

**Emission pattern:**

The `WorkflowEventEmitter` wraps the `events.MatrixEventBus` (a separate high-throughput ring buffer in `internal/events/`). Each emit method builds a `WorkflowEvent`, wraps it in a `MatrixEvent` with a generated ID (`{eventType}-{workflowID}-{nanotime}`), and calls `bus.Publish()`. The bus assigns a monotonically increasing sequence number and broadcasts to live subscribers.

Standalone helper functions (`EmitStepProgressEvent`, `EmitStepErrorEvent`, `EmitBlockerWarningEvent`) create a temporary emitter and publish in one call, convenient for callers that don't hold a long-lived emitter.

**Step icon rendering:**

The `stepIcon()` function in `notifications.go` maps event types to emoji for timeline display in Matrix messages:

| Event Type | Icon |
|-----------|------|
| `step` | `🔹` |
| `file_read` | `📄` |
| `file_write` | `✏️` |
| `file_delete` | `🗑️` |
| `command_run` | `⌨️` |
| `observation` | `💭` |
| `blocker` | `🚧` |
| `error` | `❌` |
| `artifact` | `📦` |
| `checkpoint` | `🏁` |
| (default) | `•` |

`FormatTimelineMessage()` builds a human-readable timeline from `ExtendedStepResult.Events`, rendering each event with its icon, name, duration, and context-specific details (line counts, exit codes, blocker messages). `FormatBlockerMessage()` formats blocker lists into structured Matrix notifications with suggestions, field names, and expiration timers.

### Platform Adapters (`internal/adapter/`)

This package contains the primary platform adapters that route messages between external services and the ArmorClaw Bridge.

**Matrix adapter (`matrix.go`):**

The central adapter. `MatrixAdapter` is a full Matrix client that syncs with the homeserver, processes events, and publishes them to the event bus. It handles:

- Login and access token management with token refresh
- Long polling via `/sync` with server-side sync filters for performance
- Event queue for inbound Matrix events
- PII scrubbing via the `pii.Scrubber`
- Zero-trust verification via `TrustVerifier` (sender trust scoring, device fingerprinting)
- Audit logging via `audit.TamperEvidentLog`
- Integration with the high-throughput `events.MatrixEventBus` for agent streaming
- Sync performance metrics (`SyncMetrics`)

**processEvents() and event routing:**

The `processEvents()` method is called after each successful `/sync`. It iterates through joined room timelines and routes events by type:

1. **`m.room.message` events** go through the full trust/PII pipeline. Before queuing, the adapter checks for studio commands via `StudioCommandHandler.HandleMatrixMessage()`. If a studio handler consumes the event, it skips the queue. Otherwise, the event is pushed to `eventQueue` and published to the `MatrixEventBus`.

2. **Custom ArmorClaw event types** are routed by prefix:
   - `workflow.*` (progress, step_progress, step_error, blocker_warning, blocked, timeline) are forwarded to `publishCustomEvent()`
   - `agent.*` (comment, etc.) are forwarded the same way
   - `blocker.*` (required, etc.) are forwarded the same way
   - Other dotted types not starting with `m.` are logged at debug level

`publishCustomEvent()` publishes to the `MatrixEventBus` (high-throughput ring buffer). The publisher call runs in a goroutine to avoid blocking sync.

**MatrixEventBus integration:**

The `events.MatrixEventBus` (defined in `internal/events/matrix_event_bus.go`) is a ring buffer separate from the general `EventBus` in `pkg/eventbus/`. It is designed for high-throughput agent streaming:

- Ring buffer with configurable size (default 1024 events)
- Monotonically increasing sequence numbers assigned at publish time
- Non-blocking publish: slow subscribers are skipped rather than blocking
- `Publish()` returns the assigned sequence number
- `GetEventsAfter(cursor)` and `WaitForEvents(ctx, cursor)` allow consumers to tail the buffer by sequence position, with batch reads up to 128 events
- `Subscribe()` returns a buffered channel (capacity 100) that receives live events
- Condition variable broadcast wakes polling consumers when new events arrive

The `MatrixAdapter` stores the bus in its `eventBus` field, set via `SetEventBus()`. Both message events and custom events (workflow, agent, blocker) are published to this bus. The workflow event emitter (`WorkflowEventEmitter`) also publishes directly to the same bus, creating a unified stream that Matrix consumers can subscribe to.

**Slack adapter (`slack.go`):**

Routes messages between Slack workspaces and Matrix rooms. `SlackAdapter` supports:

- Bot token authentication (`xoxb-` tokens)
- Socket Mode for real-time events (optional)
- Channel monitoring and forwarding to a configured Matrix room
- User and channel caching
- Rate limiting
- Message queue integration via `queue.MessageQueue`

**Command handling (`commands_integration.go`):**

`CommandHandler` processes Matrix messages that start with `/` or `!` as commands. It integrates with `admin.ClaimManager`, `lockdown.Manager`, and `skills.LearnedStore` for administrative and agent operations.

Admin commands use the `/` prefix and cover setup and operations: `/claim_admin`, `/status`, `/verify`, `/approve`, `/reject`, `/help`.

Agent commands use the `!agent` prefix and interact with the `LearnedStore` for persistent skill management:

| Command | Syntax | Purpose |
|---------|--------|---------|
| `!agent skills` | `!agent skills <agent_id>` | Lists all learned skills for an agent. Each skill shows its name, confidence score (0.00 to 1.00), and successful invocation count. Returns an empty message if the agent has no learned skills. |
| `!agent forget-skill` | `!agent forget-skill <agent_id> <skill_id>` | Deletes a learned skill by ID. Used to remove skills that are no longer useful or were learned incorrectly. Returns an error if the skill ID is not found. |

Both commands require a non-nil `LearnedStore`. If the store is unavailable (nil), they return an error. The `!agent` prefix routes to `handleAgentSubcommand`, which dispatches to `handleAgentSkills` or `handleAgentForgetSkill` based on the first argument. Unknown subcommands get an error with the list of available subcommands.

Responses are sent back to the Matrix room as `m.notice` messages via `SendMessageWithRetry`. Errors are prefixed with a red X emoji and formatted in bold.

**Trust integration (`trust_integration.go`):**

`TrustVerifier` applies zero-trust principles to every Matrix event. It checks sender trust scores, device fingerprints, and IP reputation. Events from untrusted sources are rejected based on a configurable minimum trust level.

**Additional files:**

- `key_ingestion.go` handles cryptographic key ingestion for the Matrix adapter
- `pii_consent.go` manages PII consent tracking per user

### Additional Platform Adapters (`internal/sdtw/`)

The `sdtw` package (named after the platforms it supports: Slack, Discord, Teams, WhatsApp) provides a uniform adapter interface for external messaging platforms. Each adapter implements `SDTWAdapter` and embeds `BaseAdapter` for shared functionality.

**Adapter interface (`adapter.go`):**

`SDTWAdapter` defines the contract every platform adapter must follow:

- **Lifecycle:** `Initialize`, `Start`, `Shutdown`
- **Core:** `SendMessage`, `ReceiveEvent`
- **Reactions:** `SendReaction`, `RemoveReaction`, `GetReactions`
- **Mutation:** `EditMessage`, `DeleteMessage`, `GetMessageHistory`
- **Health:** `HealthCheck`, `Metrics`

Each adapter declares a `CapabilitySet` (read, write, media, reactions, threads, edit, delete, typing, read receipts) so callers can check what operations are supported before attempting them.

Messages carry HMAC-SHA256 signatures for integrity verification. `SignMessage` and `VerifySignature` handle signing and validation.

**Discord adapter (`discord.go`):**

Connects to Discord via bot token. Uses the Discord Gateway protocol with opcodes for heartbeat, identify, resume, and dispatch. Supports rich embeds, message references (threaded replies), and guild-scoped operation.

**WhatsApp adapter (`whatsapp.go`):**

Uses the WhatsApp Business Cloud API. Authenticates with an access token and phone number ID. Supports text, template, image, document, audio, and video message types. Includes a token-bucket rate limiter to respect WhatsApp API limits.

**Microsoft Teams adapter (`teams.go`):**

Authenticates via Azure AD (client ID, client secret, tenant ID). Processes Teams messages via webhook. Supports OAuth token refresh. Includes HMAC-SHA256 webhook verification for inbound request validation.

**Slack adapter (`slack.go`):**

A second Slack adapter in the `sdtw` package, separate from `internal/adapter/slack.go`. This one follows the `SDTWAdapter` interface contract, supports webhook-based message delivery, and handles Slack's URL verification challenge for event subscriptions.

## Integration Points

These subsystems connect to the Bridge core at specific, well-defined points. They don't connect to each other.

```
Bridge Core
  |
  +-- Matrix Adapter (internal/adapter)
  |     |-- Receives Matrix events via /sync
  |     |-- processEvents() routes by type:
  |     |     |-- m.room.message -> trust/PII pipeline -> eventQueue
  |     |     |-- workflow.* / agent.* / blocker.* -> publishCustomEvent()
  |     |-- Publishes to EventBus (pkg/eventbus) and MatrixEventBus
  |     |-- Applies TrustVerifier and PII scrubbing
  |
  +-- Observable Container Event Flow (v3)
  |     |-- Container writes events.jsonl (StepEvents)
  |     |-- EventReader (Bridge) tails events.jsonl
  |     |-- OrchestratorIntegration processes StepEvents
  |     |-- WorkflowEventEmitter publishes to MatrixEventBus:
  |     |     workflow.started, workflow.step_progress,
  |     |     workflow.step_error, workflow.blocker_warning,
  |     |     workflow.completed
  |     |-- MatrixEventBus -> processEvents() -> Matrix room
  |     |-- Matrix /sync -> ArmorChat receives timeline
  |
  +-- EventBus (pkg/eventbus)
  |     |-- Receives events from Matrix adapter and internal components
  |     |-- Distributes to in-process subscribers
  |     |-- Optionally broadcasts via WebSocket (pkg/websocket) — wired since v0.7.0 (EventBroadcaster adapter)
  |     |-- Optionally appends to durable log
  |
  +-- MatrixEventBus (internal/events)
  |     |-- High-throughput ring buffer for agent streaming
  |     |-- Receives from processEvents() and WorkflowEventEmitter
  |     |-- Sequence-numbered cursor-based reads
  |     |-- Non-blocking: slow subscribers are skipped
  |
  +-- Push Gateway (pkg/push)
  |     |-- Called when Matrix messages need mobile notification
  |     |-- Delegates to FCM, APNS, or Web Push providers
  |     |-- Integrates with Sygnal for Matrix push
  |
  +-- SSO Manager (pkg/sso)
  |     |-- Called during authentication flow
  |     |-- Independent of other subsystems
  |
  +-- Platform Adapters (internal/adapter, internal/sdtw)
         |-- Route messages between external platforms and Matrix
         |-- Each adapter is independently configured and started
```

The key takeaway: each subsystem is wired into the Bridge core independently. There is no shared subsystem-to-subsystem wiring. The event bus is the closest thing to a shared dependency, but only the Matrix adapter publishes to it directly. Other subsystems don't depend on it.

The observable container flow (v3) adds a second publishing path: `WorkflowEventEmitter` publishes directly to the `MatrixEventBus`, bypassing the general `EventBus`. This keeps high-frequency step events on a dedicated ring buffer that won't interfere with the general pub/sub backbone. The `MatrixEventBus` feeds into `processEvents()` and then out to Matrix rooms, where ArmorChat picks them up via `/sync`.

### BroadcastStatus Events

When agent state transitions occur, the Bridge-side state inference engine detects the change and publishes a `com.armorclaw.agent.status` Matrix event. This gives downstream consumers (ArmorChat, monitoring dashboards) a real-time view of agent lifecycle without requiring polling.

**Event type:** `com.armorclaw.agent.status`

**Event content:**

```json
{
  "status": "<new_status>",
  "agent_id": "<agent_identifier>",
  "previous": "<prior_status>",
  "metadata": {
    "workflow_id": "<optional_workflow>",
    "step": "<optional_current_step>",
    "inferred_from": "<signal_source>"
  },
  "timestamp": 1710000000000
}
```

The `inferred_from` field indicates what signal triggered the status change:

| Value | Meaning |
|-------|---------|
| `cdp` | Chrome DevTools Protocol activity (page load, navigation, console events) |
| `workflow` | Workflow engine side-channel (step transitions, completion, failure) |
| `command` | Explicit RPC call (user-initiated status change) |

**Routing:** `bridge/internal/adapter/matrix.go` handles all `com.armorclaw.` prefixed events via the `publishCustomEvent()` method. This is the same code path used by `workflow.*`, `agent.*`, and `blocker.*` events described in the Matrix adapter section above.

**Source:** `bridge/pkg/agent/integration.go:359-384`

### Email Approval Events

When an agent requests to send outbound email containing PII, the Bridge emits an `app.armorclaw.email_approval_request` Matrix event. This triggers a human-in-the-loop approval flow on the mobile client before the email is sent.

**Event type:** `app.armorclaw.email_approval_request`

**Event content:**

```json
{
  "approval_id": "<unique_approval_identifier>",
  "email_id": "<email_reference>",
  "to": "<recipient_address>",
  "subject": "[masked]",
  "pii_fields": ["body", "attachment_names"],
  "timeout_s": 60,
  "event_type": "EMAIL_APPROVAL_REQUEST"
}
```

The `subject` field is masked in transit. The actual subject is only revealed after the user approves. `pii_fields` lists which parts of the email contain personally identifiable information.

**Approval flow:**

1. Agent requests email send via RPC.
2. Bridge detects PII in the email payload and emits the approval request event to the user's Matrix room.
3. ArmorChat receives the event via Matrix push, renders the `EmailApprovalCard` composable.
4. User taps **Approve** or **Deny**.
5. The mobile client calls `approve_email` or `deny_email` RPC methods on the Bridge.
6. If approved, the Bridge proceeds with the send. If denied (or timed out), the request is discarded.

**Alert type:** `EMAIL_APPROVAL_REQUEST` in the `SystemAlert` enum.

**Key files:**

| Component | File | Purpose |
|-----------|------|---------|
| RPC handlers | `bridge/pkg/rpc/email_approval.go` | `approve_email` and `deny_email` method handlers |
| Android UI | `applications/ArmorChat/.../EmailApprovalCard.kt` | Approval card composable with approve/deny actions |
