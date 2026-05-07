# ArmorClaw System Architecture

**Version**: v1.0.0 | **Status**: Production Ready | **Updated**: 2026-05-07

This document is the authoritative architecture reference for ArmorClaw. It describes every component, how they connect, how data flows, and the key decisions that shape the system.

---

## System Overview

ArmorClaw is a VPS-based agent platform. AI agents run 24/7 on your server, browsing websites, filling forms, and managing tasks. You control them from your phone via Matrix, approving sensitive operations before they execute.

```
 USER (Phone)                    THE VPS                           ISOLATED CONTAINERS
 ──────────────                  ───────────                       ─────────────────────

 ┌──────────┐  Matrix E2EE   ┌──────────────┐   Docker API    ┌──────────────────────┐
 │ArmorChat │───────────────▶│    Matrix     │                  │  Agent Container     │
 │(Android) │                │   Conduit     │                  │  (NetworkMode: none) │
 └──────────┘                └──────┬───────┘                  │                      │
                                     │ AppService               │  ┌─────┐  CDP  ┌───┐ │
 ┌──────────┐  JSON-RPC             ▼                          │  │Open │──────▶│Jet │ │
 │  Admin   │───────────────▶┌──────────────┐                  │  │Claw │      │ski │ │
 │  Panel   │                │   Go Bridge  │◀──gRPC(Unix)────▶│  └─────┘      └───┘ │
 │ (React)  │                │ (Control Pl.)│                  └──────────────────────┘
 └──────────┘                │              │     ┌─────────┐
                             │ EventBus     │     │browser- │ (Legacy fallback)
 ┌──────────┐  JSON-RPC      │ Secretary    │     │service  │
 │  Setup   │───────────────▶│ Trust (PII)  │     └─────────┘
 │  Wizard  │                │ BrowserBroker│
 │ (React)  │                └──┬───┬───┬───┘
 └──────────┘                   │   │   │     gRPC over Unix sockets
                                ▼   ▼   ▼
                             ┌────┐┌────┐┌────────┐
                             │Rust││Py  ││Rust    │
                             │Side││Side││Vault   │
                             │car ││car ││(SQLCiph│
                             └────┘└────┘└────────┘
                                              ┌──────────────┐
                                              │License Server│
                                              │(PostgreSQL)  │
                                              └──────────────┘
```

---

## Components

### 1. Go Bridge (Control Plane)

| Field | Value |
|-------|-------|
| Language | Go 1.24+ |
| Location | `bridge/` |
| Status | Production Ready |
| Packages | 68 (`bridge/pkg/`) |
| RPC Methods | 95–108 (feature-flag dependent) |
| Binaries | 5 (`cmd/bridge`, `cmd/bootstrap-admin`, `cmd/gen-models`, `cmd/migrate-templates`, `cmd/mta-recv`) |

The Bridge is the central orchestrator. Everything passes through it.

**Core responsibilities:**

- Matrix Conduit connection management and appservice registration
- User authentication via Matrix login
- JSON-RPC server (Unix socket or TCP) for client communication
- Container lifecycle (create, monitor, terminate agent containers)
- AI provider routing (OpenAI, Anthropic, OpenRouter, xAI, DeepSeek, and 7+ more)
- Audit logging for all operations
- EventBus for real-time event streaming to connected clients
- PII trust layer (BlindFill, Governor-Shield, ShadowMap)
- Browser automation coordination via BrowserBroker
- Workflow orchestration via Secretary engine
- E2EE crypto via CryptoEngine (Olm/Megolm, SAS verification, cross-signing)

**Key packages:**

| Package | Purpose |
|---------|---------|
| `pkg/rpc/` | JSON-RPC server, 95–108 method handlers (feature-flag dependent) |
| `pkg/matrix/` | Matrix client, appservice, E2EE crypto |
| `pkg/eventbus/` | Pub/sub event streaming to WebSocket clients |
| `pkg/secretary/` | Workflow engine, event reader, step executor |
| `pkg/browser/` | BrowserBroker interface, JetskiBroker, NavChart types |
| `pkg/sidecar/` | 3-layer document routing to Rust/Python sidecars |
| `pkg/trust/` | PII detection, approval flow, risk classification |
| `pkg/skills/` | Learned skill extraction and persistence (SQLite) |
| `pkg/crypto/` | CryptoEngine wrapping mautrix-go Olm/Megolm |
| `pkg/docker/` | Container creation with seccomp/AppArmor hardening |
| `pkg/keystore/` | SQLCipher encrypted key-value storage |
| `pkg/email/` | Email HITL approval pipeline |
| `pkg/governor/` | Governance events and policy enforcement |
| `pkg/providers/` | AI provider registry with alias resolution |
| `pkg/webrtc/` | Voice infrastructure (partial, no STT/TTS/VAD providers yet) |

**Feature Flags (v1.0.0):**

The Bridge exposes RPC methods conditionally based on feature flags in `/etc/armorclaw/config.toml` (or equivalent env vars `ARMORCLAW_FEATURE_*`). Flags are parsed by `bridge/pkg/config/config.go` into a `FeatureFlags` struct, then wired to the RPC server's `Config` in `main.go`. With all flags off, the baseline is 95 methods. Each flag adds methods to the discovery surface.

| Configuration | Methods | Flags Enabled |
|---|---|---|
| All off (baseline) | 95 | None |
| Zero-Trust Keystore | 102 | ZeroTrustKeystore |
| + Voice Pipeline | 105 | ZeroTrustKeystore + VoicePipeline |
| + E2EE Backup | 108 | ZeroTrustKeystore + VoicePipeline + E2EEBackup |
| + Multi-Tab Replay | 108 | All flags on (gates `browser.replay_diagnostics` at handler level, does not add new method) |

Flag-dependent methods return error code `-32601` (method not found) when their flag is disabled. The `a0_discover.sh` contract discovery pipeline counts responding methods to detect which flags are active.

**E2EEBackup wiring:** When `feature_e2ee_backup = true`, main.go (lines 2592-2603) creates a BackupStore at `{keystore_dir}/backups` and wires a BackupManager to the RPC server. If the store init fails (e.g. directory not writable), the feature is gracefully disabled and the backup methods return `-32601`.

**Zero-Trust Keystore (v1.0.0):**

The keystore (`pkg/keystore/`) now supports password-gated unseal alongside the existing challenge-response mechanism. When the `ZeroTrustKeystore` flag is enabled, 7 additional RPC methods are registered.

- **Argon2id password verification** for keystore unseal. Passwords are hashed with Argon2id (memory: 64MB, iterations: 3, parallelism: 4) and never stored in plaintext.
- **Auto-seal timer** seals the keystore after a configurable idle period (default: 5 minutes). Any keystore RPC call resets the timer.
- **Rate limiting** on unseal attempts (5 failures within 60 seconds triggers a 30-second lockout). Tracked per-connection.
- **Memory zeroization** of password material immediately after Argon2id verification completes.
- **Audit logging** for all seal/unseal events, session extensions, and key deletions.

**Known gap:** `rpcCfg.SealedKS` is not initialized in main.go. The `ZeroTrustKeystore` flag enables the RPC surface (7 methods), but they all return "sealed keystore not configured" because no `NewSealedKeystore()` call exists yet.

**RPC method groups:**

| Prefix | Count | Purpose |
|--------|-------|---------|
| `browser.*` | 12 | Browser automation (navigate, fill, click, wait, complete, replay_diagnostics) |
| `pii.*` | 9 | PII request/approve/deny/fulfill pipeline |
| `skills.*` | 14 | Skill execution, allowlist, web search, email, Slack |
| `secretary.*` | 13 | Workflow CRUD, templates, active count, shutdown |
| `matrix.*` | 5 | Matrix status, login, send, receive, join |
| `bridge.*` | 10 | Bridge lifecycle, channel management, E2EE toggle |
| `container.*` | 2 | Container terminate and list |
| `device.*` | 4 | Device governance (list, approve, reject) |
| `invite.*` | 4 | Invite token lifecycle |
| `hardening.*` | 3 | Security hardening status and password rotation |
| `task.*` | 4 | Task create/list/cancel/get (delegates to secretary) |
| `events.*` | 2 | Event replay and streaming |
| `email.*` | 3 | Email approval status and pending list |
| `studio.*` | 2 | Agent studio deploy and stats |
| `e2ee.*` | 2 (baseline) | E2EE enable/disable toggle (`e2ee_enable`, `e2ee_disable`). When `E2EEBackup=true`, adds 3 more: `create_backup`, `delete_backup`, `backup_exists` (total 5) |
| `provisioning.*` | 2 | QR provisioning start and claim |
| `keystore.*` | 7 | Zero-trust keystore: unseal, sealed, seal, extend_session, session_status, list_keys, delete_key |
| `voice.*` | 3 | Voice session management: start_session, stop_session, status |
| other | 5 | Health, heartbeat, key storage, AI chat, blocker, account |

---

### 2. ArmorChat (Android Client)

| Field | Value |
|-------|-------|
| Language | Kotlin |
| Location | `applications/ArmorChat/` |
| Status | Active Development (v0.7.0) |
| Kotlin Files | 61 |
| Packages | 11 + MainActivity |
| ViewModels | 6 |

The mobile control app. Users monitor agents, approve PII, manage workflows, and handle security settings from their phone.

**Package structure (flat, 11 packages + root):**

| Package | Contents |
|---------|----------|
| `config/` | ConfigManager, BridgeTrustStore, SignedConfigParser |
| `crypto/` | CryptoService, MatrixOlmService, VodozemacNative |
| `data/` | Models (EmailApprovalEvent, SystemAlert), repositories (BridgeRepository, UserRepository, BridgeCapabilities), local entities |
| `navigation/` | ArmorClawNavHost, Route (14 routes), DeepLinkHandler |
| `network/` | BridgeApi, BridgeDiscovery, ResilientWebSocket, NetworkResilience |
| `push/` | ArmorClawMessagingService, PushTokenManager, MatrixPusherManager, NotificationHelper |
| `repository/` | SetupRepository |
| `ui/` | Screens: agent, approval, email, home, workflow, security, verification, migration, components (WorkflowTimeline, BlindFillCard, PiiApprovalCard, BlockerResponseDialog) |
| `utils/` | ErrorHandler |
| `validation/` | ValidationReceiver |
| `viewmodel/` | AgentManagementViewModel, BondingViewModel, HardeningWizardViewModel, HitlViewModel, SecurityConfigViewModel, WorkflowViewModel |
| root | MainActivity.kt |

**Key features:**

- Biometric-protected keystore access
- Real-time workflow timeline with live progress indicators
- PII approval cards with risk classification display
- BlindFill approval flow
- Email approval cards for HITL
- Deep link handling for QR provisioning
- Bridge capability negotiation
- E2EE support via Vodozemac (Olm/Megolm)
- Push notifications via Sygnal + FCM
- Network resilience with exponential backoff WebSocket

---

### 3. Jetski (CDP Browser Sidecar)

| Field | Value |
|-------|-------|
| Language | Go + Zig |
| Location | `jetski/` |
| Status | Production Ready |
| RPC Port | 9223 |
| Engine Port | 9333 (Lightpanda) |

Jetski sits between agent containers and the browser engine. It proxies CDP (Chrome DevTools Protocol) connections with active PII scrubbing.

**What it does:**

- CDP WebSocket proxy with PII scrubbing at the `net.Conn` level
- SQLCipher session encryption (PBKDF2-HMAC-SHA512, 256k iterations)
- Matrix HITL approval for sensitive browser operations (60s timeout)
- Active PII scrubbing: SSN, credit card, email, password patterns
- Sonar telemetry for session monitoring
- NavChart pipeline: 6-stage CDP-to-chart normalization (filter, group, detect PII, replace, extract selectors, attach metadata)
- Chartmaker sub-project (TypeScript CLI) for recording browser interactions
- Lighthouse sub-project for NavChart REST API
- Lightpanda lightweight browser engine as an alternative to Playwright

**Security model:**

Tethered Mode means the CDP proxy actively inspects and modifies traffic between the agent and the browser. Secrets never reach the agent's view. Session data is encrypted at rest with SQLCipher.

---

### 4. Rust Sidecar (Data Plane)

| Field | Value |
|-------|-------|
| Language | Rust |
| Location | `sidecar/` |
| Status | Production Ready |
| Communication | gRPC over Unix Domain Socket |
| Socket | `/run/armorclaw/sidecar.sock` (0600) |

Handles heavy I/O operations that the Go Bridge shouldn't do directly.

**Capabilities:**

- S3, SharePoint, Azure Blob upload/download/list/delete with streaming
- PDF text extraction, split, merge
- DOCX text extraction and editing
- OCR integration
- Split-Storage RAG (document chunks stored separately from embeddings)
- YARA Content Disarm and Reconstruction (CDR)

**Performance characteristics:**

- Zero-copy streaming, no buffering
- Single-pass SHA256 hashing
- 1MB chunk size for downloads
- Memory bounded to ~2MB for download streams
- Handles files up to 5GB
- Circuit breaker: 5 failures triggers open state, 30s recovery
- Rate limiting: 100 req/s
- Prometheus metrics endpoint

**Security:**

- No persistent credential storage
- No credential caching beyond request lifecycle
- Ephemeral tokens with 30 min TTL
- PII interception happens in the Go Bridge before sidecar calls
- All operations logged in Bridge `audit.db`

---

### 5. Python Sidecar (MarkItDown)

| Field | Value |
|-------|-------|
| Language | Python |
| Location | `sidecar-python/` |
| Status | Production Ready |
| Communication | gRPC over Unix Domain Socket |
| Socket | `/run/armorclaw/sidecar-office.sock` |
| Test Coverage | 65 tests (27 worker + 16 edge cases + 12 interceptor + 10 Docker) |

Extends document processing to legacy Microsoft Office formats.

**Supported formats:** XLSX, PPTX, MSG (Outlook email), XLS, DOC, PPT

**3-Layer Routing** (Go Bridge `RouteExtractText()`):

| Layer | Condition | Action |
|-------|-----------|--------|
| 0 | Plain text (txt, csv, json, md) | Decode natively in Go, no sidecar call |
| 1 | Valid magic bytes + MIME match | Route to Python (XLSX/PPTX/MSG) or Rust (PDF/DOCX) |
| 2 | Magic bytes mismatch declared format | Strict drop, reject immediately |

**Design details:**

- Threshold streaming: in-memory for files under 10MB, temp file for larger
- TTL recycling: server exits after 50 requests for container restart cycling
- HMAC-SHA256 token validation for all requests
- `NetworkMode: none`, `cap_drop: ALL`, read-only root filesystem

---

### 6. OpenClaw (Agent Runtime)

| Field | Value |
|-------|-------|
| Language | TypeScript + Python |
| Location | `container/openclaw/` (runtime), `container/openclaw-src/` (source) |
| Status | Production Ready |
| Platform Extensions | 37 |
| Skills | 52 |

The agent runtime that executes inside isolated containers. Ships with a rich skill ecosystem and platform integrations.

**Runtime components:**

| Component | File | Purpose |
|-----------|------|---------|
| EventEmitter | `openclaw/events.py` | Writes `_events.jsonl` with 11 event types |
| StepRunner | `openclaw/step_runner.py` | Executes task steps |
| BridgeClient | `openclaw/bridge_client.py` | Communicates with the Bridge |
| Security Hook | `openclaw/security_hook.c` | Container-level security instrumentation |
| Entry Point | `openclaw/entrypoint.ts` | Container startup orchestration |

**11 event types:** STEP, FILE_READ, FILE_WRITE, FILE_DELETE, COMMAND_RUN, OBSERVATION, BLOCKER, ERROR, ARTIFACT, PROGRESS, CHECKPOINT

**Platform extensions (37):** Matrix, Slack, Discord, Telegram, WhatsApp, Signal, iMessage, Google Chat, MS Teams, Feishu, Line, IRC, Nostr, Mattermost, Nextcloud Talk, Twitch, Tlon, Lobster, BlueBubbles, Zalo, device-pair, thread-ownership, voice-call, talk-voice, phone-control, memory-core, memory-lancedb, llm-task, diagnostics-otel, copilot-proxy, open-prose, and auth modules for Qwen, Minimax, Google Gemini CLI, Google Antigravity.

**Skills (52):** web browsing, coding agent, GitHub issues, Discord/Slack messaging, Notion, Obsidian, Trello, weather, Spotify, image generation (OpenAI, nano-banana-pro), PDF processing, video frames, voice call, Whisper transcription, TTS, health checks, tmux, canvas, and more.

---

### 7. Matrix Conduit (Homeserver)

| Field | Value |
|-------|-------|
| Software | Conduit (matrix-conduit) |
| Status | Production Ready |
| Port | 6167 |
| Protocol | Matrix (Federation) |

The Matrix homeserver is the control plane transport. All communication between ArmorChat and the Bridge flows through Matrix.

**Role in the system:**

- E2EE message transport between phone and server
- Room management for agent conversations
- AppService registration for Bridge bot accounts
- Push notification delivery via Sygnal
- SAS verification and cross-signing for device trust
- Kill switch for E2EE: `matrix.e2ee.enabled` (default: false)

The Bridge registers as a Matrix AppService. When users create agents, the Bridge creates dedicated Matrix rooms and invites the user. All agent interactions happen through these rooms.

---

### 8. Admin Panel (React)

| Field | Value |
|-------|-------|
| Language | TypeScript (React + Vite + Tailwind) |
| Location | `applications/admin-panel/` |
| Status | Active Development (v0.7.0) |

Web dashboard for server administration.

**Features:**

- System health monitoring
- Device governance (list, approve, reject)
- Invite token management (create, revoke, validate)
- Bridge status overview
- Typed RPC client calls to the Bridge API

---

### 9. ArmorTerminal (Android Terminal)

| Field | Value |
|-------|-------|
| Language | Kotlin (Android) |
| Location | `applications/ArmorTerminal/` |
| Status | Production Ready |

A minimal Android pairing client. Used for initial device registration and basic terminal access to the Bridge RPC. Simpler than ArmorChat, focused on setup and diagnostics.

---

### 10. Setup Wizard (React)

| Field | Value |
|-------|-------|
| Language | TypeScript (React + Vite + Tailwind) |
| Location | `applications/setup-wizard/` |
| Status | Production Ready |

The initial configuration interface. Walks new users through:

- AI provider selection and API key entry
- Admin account creation
- Deployment mode selection (Native, Sentinel, Cloudflare)
- QR code generation for ArmorChat provisioning

---

### 11. Rust Vault

| Field | Value |
|-------|-------|
| Language | Rust |
| Location | `rust-vault/` |
| Status | Production Ready |
| Storage | SQLCipher |

Encrypted key-value storage backed by SQLCipher. Used for persisting governance data, audit records, and other sensitive state that needs encryption at rest.

---

### 12. License Server

| Field | Value |
|-------|-------|
| Language | Go |
| Location | `license-server/` |
| Status | Production Ready |
| Database | PostgreSQL |

Manages license tiers and enforcement. The Bridge checks with the License Server for tier validation and grace period handling.

---

### 13. browser-service (TypeScript/Playwright)

| Field | Value |
|-------|-------|
| Language | TypeScript |
| Location | `browser-service/` |
| Status | Production Ready (Legacy Fallback) |

The original browser automation service. Still available as a fallback via `ARMORCLAW_BROWSER_BACKEND=legacy`. All new browser operations route through BrowserBroker via JetskiBroker instead.

---

### 14. Container Security

Not a separate component, but a cross-cutting concern applied to all agent containers.

**Hardening applied to every container:**

```
--cap-drop=ALL
--security-opt=no-new-privileges
--read-only
--pids-limit=100
--memory=512M
--security-opt seccomp=seccomp-profile.json
--security-opt apparmor=armorclaw-profile
```

**Security profiles:**

- `container/seccomp-profile.json` restricts syscalls
- `container/apparmor-profile` limits file and network access
- `container/apparmor-profile-office` for Python sidecar containers
- No network tools installed inside containers
- Read-only root filesystem with tmpfs for writes

---

## Architectural Decisions

### NetworkMode: none (Absolute)

All agent containers run with `NetworkMode: none`. No container networking, no exceptions. This is non-negotiable.

Consequences:
- Containers cannot make outbound HTTP requests
- Containers cannot receive inbound connections
- No IPC channels, bind-mounted event files, or any mechanism requiring network
- All external access must go through the Bridge via Unix sockets
- Warm dispatch was architecturally impossible and removed in v0.7.0

### Crash-only WebSocket

The EventBus wiring uses `log.Fatalf` when WebSocket initialization fails (`bridge/pkg/eventbus/eventbus.go:146`). The Bridge crashes rather than running in a degraded state where events are silently lost.

There is no graceful fallback or retry logic on this path. This is intentional and requires CTO approval to change.

### Cold-only Dispatch

Warm dispatch (`warmDispatch()` in TaskScheduler) was architecturally illegal under `NetworkMode: none`. Containers cannot receive inbound connections, so warm dispatch could never work. Dead code was removed in v0.7.0.

### Zero-trust PII

Three mechanisms protect sensitive data:

**BlindFill** injects secrets directly into browser form fields via memory-only Unix sockets. The agent requests a reference like `payment.card_number`. The Bridge checks user approval, retrieves the actual value from encrypted storage, and injects it into the form field. The agent never sees the raw value.

**Governor-Shield** scrubs tool call arguments before they reach agents. PII patterns are detected and masked in transit.

**ShadowMap** maintains PII detection patterns for active masking during data flow between components.

### Matrix as Control Plane

Matrix is not just a messaging layer. It is the control plane transport for the entire system. All communication between ArmorChat and the Bridge flows through Matrix rooms. This provides:

- E2EE for all user-facing communication
- Room-based access control per agent
- Federation support for multi-server setups
- Push notification delivery via Sygnal
- Device management via Matrix device identity

### gRPC over Unix Sockets for Sidecars

Both the Rust and Python sidecars communicate with the Bridge via gRPC over Unix Domain Sockets. This avoids TCP overhead, keeps traffic local, and enforces filesystem permissions (0600) as an additional access control layer.

### JSON-RPC over Unix/TCP for Clients

ArmorChat and the Admin Panel communicate with the Bridge via JSON-RPC 2.0. In Native mode this goes over Unix sockets (`/run/armorclaw/bridge.sock`). In Sentinel or Cloudflare modes it goes over TCP (`0.0.0.0:8080`) with TLS.

---

## Deployment Modes

| Mode | Transport | TLS | Access | Use Case |
|------|-----------|-----|--------|----------|
| Native | Unix socket | None | Local | Development, testing |
| Sentinel | TCP + Caddy | Let's Encrypt | Public | Production VPS |
| Cloudflare Tunnel | cloudflared | Cloudflare | Public | NAT/firewall, no public IP |
| Cloudflare Proxy | HTTP(S) | Cloudflare | Public | Existing Cloudflare setup |
| Self-Hosted | TCP + Caddy | Self-signed | LAN | Home server, mDNS discovery |

**Mode detection:** The installer auto-detects mode based on whether a domain is provided. Native mode is the default when no domain is entered.

**Environment variables that control mode:**

| Variable | Native | Sentinel | Cloudflare |
|----------|--------|----------|------------|
| `ARMORCLAW_SERVER_MODE` | `native` | `sentinel` | `cloudflare` |
| `ARMORCLAW_RPC_TRANSPORT` | `unix` | `tcp` | `tcp` |
| `ARMORCLAW_LISTEN_ADDR` | (empty) | `0.0.0.0:8080` | `0.0.0.0:8080` |
| `ARMORCLAW_PUBLIC_BASE_URL` | (empty) | `https://domain` | `https://domain` |

---

## Event Flow

The primary event flow from container execution to the user's phone:

```
┌──────────────────────────────────────────────────────────────────────┐
│ Container                                                            │
│                                                                      │
│  Agent executes task step                                            │
│       │                                                              │
│       ▼                                                              │
│  EventEmitter writes _events.jsonl                                   │
│  (PIPE_BUF 4096B line enforcement, 11 event types)                   │
│       │                                                              │
│       │ (filesystem, bind-mounted)                                   │
│       ▼                                                              │
│  Soft 10MB cap: stops tailing with warning, container finishes       │
└───────────────────────────────┬──────────────────────────────────────┘
                                │
                                ▼
┌──────────────────────────────────────────────────────────────────────┐
│ Bridge                                                               │
│                                                                      │
│  EventReader (500ms polling)                                         │
│       │                                                              │
│       ▼                                                              │
│  MatrixEventBus (pub/sub)                                            │
│       │                                                              │
│       ├─────► Matrix Room (m.notice messages)                        │
│       │                                                              │
│       ├─────► WebSocket subscribers (ArmorChat live stream)          │
│       │                                                              │
│       ├─────► Learned Skill Extractor (SQLite persistence)           │
│       │         confidence >= 0.4, never auto-executed               │
│       │                                                              │
│       └─────► NavChart Extractor (browser pattern persistence)       │
│                                                                      │
│  State Cleanup: _events.jsonl purged after task completion           │
│  (parse -> cleanup -> notify ordering)                               │
└───────────────────────────────┬──────────────────────────────────────┘
                                │
                                │ Matrix /sync
                                ▼
┌──────────────────────────────────────────────────────────────────────┐
│ ArmorChat                                                            │
│                                                                      │
│  WorkflowTimeline composable                                         │
│  (event icons, progress bar, live/complete indicators)               │
│                                                                      │
│  NotificationHelper (push via Sygnal + FCM)                          │
└──────────────────────────────────────────────────────────────────────┘
```

---

## Security Model

### Transport Security

| Layer | Mechanism |
|-------|-----------|
| Phone to Matrix | Matrix E2EE (Olm/Megolm), optional (kill switch default-off) |
| Matrix to Bridge | AppService token + local network |
| Bridge to Sidecars | gRPC over Unix sockets (0600 permissions) |
| Bridge to Containers | Docker API socket + bind mounts |
| Containers to Browser | CDP via Jetski (Tethered Mode, PII scrubbing) |
| Admin/Setup to Bridge | JSON-RPC over Unix socket or TLS |

### Data at Rest

| Store | Encryption |
|-------|-----------|
| Bridge keystore | SQLCipher |
| Jetski sessions | SQLCipher (PBKDF2-HMAC-SHA512, 256k iterations) |
| Rust Vault | SQLCipher |
| Learned skills | SQLite (inside Bridge keystore context) |
| NavCharts | SQLite (PII-free, validated before storage) |
| API keys | Environment variables only, never persisted to disk |

### PII Protection

| Mechanism | Where | What |
|-----------|-------|------|
| BlindFill | Bridge to Browser | Secrets injected into form fields, agent sees nothing |
| Governor-Shield | Bridge | Tool call arguments scrubbed before reaching agents |
| ShadowMap | Bridge transit | Active PII pattern masking during data flow |
| Jetski scrubbing | CDP proxy level | SSN, credit card, email, password patterns filtered |
| 3-layer routing | Document pipeline | Magic bytes validation, strict drop on mismatch |
| TTL Proxy Guard | Sidecar tokens | Ephemeral tokens with 30 min TTL |

### Container Isolation

Every agent container gets:

- `NetworkMode: none` (no networking whatsoever)
- `cap-drop=ALL` (no Linux capabilities)
- `no-new-privileges` (no SUID escalation)
- Read-only root filesystem
- `pids-limit=100`
- `memory=512M`
- Custom seccomp profile
- Custom AppArmor profile
- No network tools in the container image

### E2EE (v0.9.0)

Matrix E2EE is implemented in the Bridge (`bridge/pkg/crypto/`) using mautrix-go's Olm/Megolm. Features:

- Full encrypt/decrypt for Matrix messages
- SAS verification for device trust
- Cross-signing bootstrap
- Kill switch: `matrix.e2ee.enabled` defaults to false

Not yet implemented: `e2ee.restore_backup` (intentionally omitted for security; restore must go through manual device verification).

---

## Subsystem Status Summary

| Subsystem | Status | Location |
|-----------|--------|----------|
| Bridge (Go) | Production Ready | `bridge/` |
| Matrix Conduit | Production Ready | Conduit homeserver |
| ArmorChat (Android) | Active Development (v0.7.0) | `applications/ArmorChat/` |
| Admin Panel (React) | Active Development (v0.7.0) | `applications/admin-panel/` |
| Jetski (CDP Proxy) | Production Ready | `jetski/` |
| Rust Sidecar | Production Ready | `sidecar/` |
| Python Sidecar | Production Ready | `sidecar-python/` |
| browser-service (TS) | Production Ready (Legacy) | `browser-service/` |
| Secretary / Workflow | Production Ready | `bridge/pkg/secretary/` |
| Email HITL | Production Ready | `bridge/pkg/email/` |
| Voice (WebRTC) | Partial | `bridge/pkg/webrtc/`, `bridge/pkg/voice/` |
| ArmorTerminal | Production Ready | `applications/ArmorTerminal/` |
| OpenClaw (Agent Runtime) | Production Ready | `container/openclaw/` |
| BrowserBroker (Go) | Production Ready | `bridge/pkg/browser/` |
| Matrix E2EE (Bridge) | Production Ready (v0.9.0) | `bridge/pkg/crypto/` |
| Rust Vault | Production Ready | `rust-vault/` |
| License Server | Production Ready | `license-server/` |
| Setup Wizard | Production Ready | `applications/setup-wizard/` |

---

## Known Gaps (v1.0.0 Scope)

- **BrowserBroker**: All browser ops route through BrowserBroker (15 methods) via JetskiBroker. Legacy browser-service available as temporary fallback via `ARMORCLAW_BROWSER_BACKEND=legacy`. NavChart pipeline supports single-tab replay. Multi-tab replay's `browser.replay_diagnostics` is implemented and gated by the `MultiTabReplay` feature flag.
- **Matrix E2EE**: Key backup is implemented and wired in main.go (lines 2592-2603) when `feature_e2ee_backup = true`, using BackupStore + BackupManager. The `e2ee.restore_backup` method is intentionally not implemented (security constraint: restore must go through manual device verification, not automated RPC).
- **Voice**: Voice manager initializes when `VoicePipeline` flag is enabled (`feature_voice_pipeline = "cloud"` in config). RPC methods (`voice.start_session`, `voice.stop_session`, `voice.status`) are flag-gated and return `-32601` when off, `-32603` when manager fails to start. STT/TTS/VAD providers remain interface-only — no AI provider backends exist yet. Voice sessions route through OpenAI cloud, not through the local WebRTC stack.
- **Azure Blob**: Re-enabled with rustls in v0.9.0, no native-tls/openssl dependency.
