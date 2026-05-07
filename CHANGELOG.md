# ArmorClaw Changelog

> **Last Updated:** 2026-05-07
> **Current Version:** 1.0.0

All notable changes to ArmorClaw are documented here with commit references.

---

## [1.0.0] - 2026-05-07 - Feature Flags, Zero-Trust Keystore, Voice Pipeline, E2EE Backup

### Added

- **14 new RPC methods** (7 keystore + 3 voice + 3 e2ee + 1 browser):
  - `keystore.unseal`, `keystore.sealed`, `keystore.seal`, `keystore.extend_session`, `keystore.session_status`, `keystore.list_keys`, `keystore.delete_key`
  - `voice.start_session`, `voice.stop_session`, `voice.status`
  - `e2ee.create_backup`, `e2ee.delete_backup`, `e2ee.backup_exists`
  - `browser.replay_diagnostics`
- **Feature flags** for conditional RPC method registration. Baseline: 95 methods with all flags off. Maximum: 109 with all flags on. Flags: `ZeroTrustKeystore`, `VoicePipeline`, `E2EEBackup`, `MultiTabReplay`.
- **Zero-trust keystore** with Argon2id password-gated unseal, auto-seal timer (default 5 min), rate limiting (5 failures per 60s triggers 30s lockout), and memory zeroization.
- **Voice pipeline** RPC methods for session management. Requires `VoicePipeline` flag. Routes through OpenAI cloud (local WebRTC stack remains partial).
- **E2EE backup** methods for key backup creation and deletion. Requires `E2EEBackup` flag. Note: `e2ee.restore_backup` does NOT exist.
- **Multi-tab replay diagnostics** via `browser.replay_diagnostics`. Requires `MultiTabReplay` flag.
- **Memory zeroization** of password material after Argon2id verification.
- **Audit logging** for keystore seal/unseal events, session extensions, and key deletions.

### Changed

- **SealedKeystore** now supports password-gated unseal (Argon2id) alongside the existing challenge-response mechanism. Both unlock paths coexist.
- RPC method surface is now feature-flag dependent (95–109 methods) instead of fixed at 95.

### Security

- **Argon2id password verification** with hardened parameters (memory: 64MB, iterations: 3, parallelism: 4). Passwords never stored in plaintext.
- **Rate limiting** on keystore unseal attempts (5 failures per 60s triggers 30s lockout).
- **Memory zeroization** of sensitive password material immediately after verification.
- **Audit trail** for all keystore state transitions and key lifecycle operations.

---

## [0.7.0] - 2026-04-18 - Inter-Step Data Passing, Dead Code Purge, Android & Admin Wiring

### Phase 1 — Workflow Engine

- **WorkflowStep.Input field** (`bridge/pkg/secretary/types.go`) — Optional `Input map[string]any` field on `WorkflowStep` with `json:"input,omitempty"` tag. Supports template variable references (`{{steps.step_1.data.order_id}}`) for inter-step data flow. Backward compatible: existing templates without `input` round-trip unchanged.
- **Migration guide** (`doc/migration/workflow-step-input.md`) — Documents the new field, template variable syntax, resolution rules, and backward compatibility guarantees.
- **Template migration tool** (`bridge/cmd/migrate-templates/`) — CLI tool that adds `"input": {}` to WorkflowStep objects in existing template JSON files. Validates templates against the WorkflowStep schema (step types, required fields). Idempotent and non-destructive: only adds missing `input` fields, rewrites only if changes were made.

### Phase 2 — Dead Code Purge

- **Warm dispatch removed** — `warmDispatch()`, `GetRunningInstance()` from `FactoryInterface`, `EventTypeTaskDispatch`, `BuildTaskDispatchPayload()`, and `task_dispatch.go` deleted from `bridge/pkg/secretary/`. All dispatch is now cold-only (ephemeral container spawn). This was architecturally required: `NetworkMode: none` makes warm dispatch impossible since containers cannot receive inbound Matrix events.
- **Dead code audit** — Swept `bridge/pkg/secretary/` and `bridge/internal/adapter/` for warm dispatch references. All callers updated to use cold dispatch path only.

### Phase 3 — WebSocket EventBus Wiring

- **WebSocket EventBus wiring** (`bridge/pkg/eventbus/eventbus.go`) — EventBus now initializes a WebSocket connection to the Bridge's own HTTP server for real-time event delivery to clients. Uses crash-only design: `log.Fatalf` on WebSocket init failure (intentional, not a bug).
- **WebSocket server** (`bridge/pkg/websocket/websocket.go`) — Replaced 74-line stub with functional WebSocket server. Accepts connections on the Bridge's HTTP listener, upgrades to WebSocket, and streams `MatrixEventBus` events to connected clients.
- **Event streaming to clients** — Events from the ring buffer (`MatrixEventBus`) are now pushed to connected WebSocket clients in real time, enabling live status updates in ArmorChat and the admin panel.

### Phase 4 — Android ArmorChat Wiring

- **DeepLinkHandler** (`applications/ArmorChat/.../DeepLinkHandler.kt`) — Notification taps now navigate to the correct screen. Parses `armorclaw://` deep links and routes to the appropriate destination (workflow detail, approval card, agent status).
- **SecurityConfigViewModel wired** — `SecurityConfigViewModel` connected to `SecurityConfigScreen`. Permission changes now persist across app restarts via the Bridge repository.
- **BridgeRepository persistence** — `BridgeRepository` credentials now survive app restarts. Uses encrypted SharedPreferences backed by the Android Keystore for server URL, admin token, and Matrix credentials.

### Phase 5 — Admin Panel Wiring

- **Real Bridge API integration** — Admin panel now calls actual Bridge RPC methods instead of using mock data. Replaced mock fetch calls with live RPC client.
- **Error handling** — Admin panel displays real connection errors, loading states, and empty states from the Bridge.

### Phase 6 — Integration Test Cleanup

- **ArmorChat integration tests** — Replaced all 12 `assertTrue(true)` placeholder tests with meaningful assertions covering deep link routing, credential persistence, SecurityConfig wiring, and notification navigation.

### Added

- **WorkflowStep.Input field** (`bridge/pkg/secretary/types.go`) — Optional `Input map[string]any` field on `WorkflowStep` with `json:"input,omitempty"` tag. Supports template variable references (`{{steps.step_1.data.order_id}}`) for inter-step data flow. Backward compatible: existing templates without `input` round-trip unchanged.
- **Migration guide** (`doc/migration/workflow-step-input.md`) — Documents the new field, template variable syntax, resolution rules, and backward compatibility guarantees.
- **Template migration tool** (`bridge/cmd/migrate-templates/`) — CLI tool that adds `"input": {}` to WorkflowStep objects in existing template JSON files. Validates templates against the WorkflowStep schema (step types, required fields). Idempotent and non-destructive: only adds missing `input` fields, rewrites only if changes were made.
- **DeepLinkHandler.kt** — Android deep link routing for notification taps.
- **WebSocket event streaming** — Real-time Bridge event delivery to connected clients.

### Changed

- **BREAKING SCHEMA CHANGE (additive):** `WorkflowStep` JSON now accepts an optional `input` object. Existing JSON without `input` continues to deserialize correctly. No database migration required.

### Removed

- **Warm dispatch dead code** — `warmDispatch()`, `GetRunningInstance()`, `EventTypeTaskDispatch`, `BuildTaskDispatchPayload()`, and `task_dispatch.go` removed from `bridge/pkg/secretary/`. All dispatch is cold-only.
- **Vestigial Rust `blindfill/` module removed** — Superseded by Jetski CDP proxy, zero production callers.

## [0.6.0] - 2026-04-17 - Operational Transparency, Governance Hardening, Advanced Automation, Mobile Polish

### Phase 0 — Warm Dispatch Deprecation

- **Deprecated warm dispatch** — architecturally illegal under NetworkMode: none. All dispatch is cold-only (ephemeral container spawn). Dead code removed in v0.7.0. Removed `warmDispatch()`, `GetRunningInstance()` from FactoryInterface, `EventTypeTaskDispatch`, `BuildTaskDispatchPayload()`, and `task_dispatch.go`.

### Phase 1 — Operational Transparency

- **Bridge-side agent state inference** — 11 agent states derived from CDP and workflow signals (IDLE, BROWSING, FORM_FILLING, AWAITING_APPROVAL, PROCESSING_PAYMENT, ERROR, COMPLETE, etc.) with `ForceTransition` for manual override
- **BroadcastStatus via Matrix events** — Agent state changes published as `com.armorclaw.agent.status` Matrix events, enabling real-time status in ArmorChat
- **Blocker metadata pipeline fix** — Fixed 7 bugs in the container-to-Bridge-to-Matrix blocker flow: added `case "blocker"` event routing, wired `EmitBlockerWarning()`, captured blocker name as human-readable message, propagated all 5 metadata fields (blocker_type, suggestion, field, message, step_id) end to end

### Phase 2 — Governance Hardening

- **Rust Vault binary entrypoint** — `armorclaw-vault` service with Docker deployment (network_mode none, UID 10001, cap_drop ALL, read_only, no-new-privileges)
- **PPTX text extraction in Rust sidecar** — ZIP-based extraction with `quick-xml` parsing, zero Python dependency for PowerPoint files
- **Route XLSX/PPTX to Rust sidecar** — Go Bridge routes PPTX to Rust instead of Python, reducing Python sidecar load
- **v6 microkernel audit mode** — Structured logging without interception for governance operations, replacing stub audit hooks

### Phase 3 — Advanced Automation

- **Bridge-side session transcript compaction** — Pre-dispatch compaction with token estimation, reducing context window usage before sending to agents
- **StepParallel execution engine** — `errgroup` goroutine pool (`MaxParallelContainers = 2`) with `StepParallelSplit`/`StepParallelMerge` dependency edges for parallel step execution
- **Step failover with multi-agent fallback** — Per-step configurable retry policy with automatic fallback to alternate agents on failure

### Phase 4 — Mobile Client Polish

- **WorkflowTimeline duration accuracy** — `parseDurationFromText()` and `fromTimelineEventJson()` for correct `(1234ms)` display in Android timeline
- **Dynamic PII masking in BlockerResponseDialog** — 8 sensitive keyword patterns (password, secret, token, api_key, credit_card, ssn, email, phone) with `PasswordVisualTransformation`
- **Email approval card** — Android approval UI for email actions plus Bridge RPC handlers (`approve_email`/`deny_email`) with consent verification

---

## [0.4.0] - 2026-04-17 - Agent Studio: Observable Containers + Learned Skills

### Added
- **Container Event Emission** (`container/openclaw/events.py`) — EventEmitter with 4 primary event types (STEP, PROGRESS, CHECKPOINT, ERROR) and PIPE_BUF 4096-byte enforcement
- **Bridge EventReader** (`bridge/pkg/secretary/event_reader.go`) — Incremental `_events.jsonl` tailing with soft 10MB cap (stops tailing with warning, no SIGKILL)
- **Progress Streaming** (`bridge/pkg/secretary/orchestrator_integration.go`) — Bridge polls `_events.jsonl` at 500ms intervals during execution, routes events to MatrixEventBus, forwards to Matrix rooms as `m.notice` messages
- **Learned Skills Store** (`bridge/pkg/skills/learned_store.go`) — SQLite-backed skill persistence with confidence ≥ 0.4 threshold, suggestions only (never auto-executed)
- **Skill Extraction** (`bridge/pkg/skills/extractor.go`) — Automatic pattern extraction from successful tasks: step_sequence (3+ distinct step names → confidence 0.5), checkpoint_sequence (any checkpoints → confidence 0.4), plus existing command_sequence and file_transform for Mode B
- **Matrix Event Forwarding** (`bridge/pkg/secretary/orchestrator_events.go`) — MatrixEventForwarder subscribes to workflow.* events, publishes as `m.room.message` with `msgtype: m.notice`
- **Timeline Formatter** (`bridge/pkg/secretary/notifications.go`) — FormatTimelineMessage and GetTimelineEvents for structured UI data
- **ArmorChat WorkflowTimeline** (`WorkflowTimeline.kt`) — Scrollable timeline composable with event icons, progress bar, live/complete indicators
- **ArmorChat GovernanceBanner** (`GovernanceBanner.kt`) — Running state with pulsing indicator and step count
- **Matrix Commands** (`bridge/internal/adapter/commands_integration.go`) — `!agent skills <agent_id>` and `!agent forget-skill <agent_id> <skill_id>`
- **State Directory Cleanup** (`bridge/pkg/secretary/cleanup.go`) — cleanupStateDir with purge ordering: parse → RemoveAll → notify

### Fixed
- **Warm dispatch silent failure** (`bridge/pkg/secretary/task_scheduler.go`) — Skip warm dispatch with WARN log for NetworkMode "none" containers (no Matrix connectivity)
- **Python sidecar interceptor crash** (`sidecar-python/interceptor.py`) — Rewrote TokenInterceptor to use sync `grpc.ServerInterceptor` instead of async `grpc_aio.ServerInterceptor` (AttributeError at runtime with sync worker.py)
- **Event routing placeholder** (`bridge/pkg/secretary/orchestrator_integration.go`) — Wired `_ = evt` placeholder to actual EmitStepProgress/EmitStepError calls
- **Soft cap on event log overflow** — Container finishes normally when `_events.jsonl` exceeds 10MB (sets capExceeded flag, logs warning, continues Docker polling — no Kill() call)

---

## [0.3.1] - 2026-02-24 - Docker Quickstart Admin & Hangup Fixes (Pass 6)

### Fixed
- **Bridge user never created on Conduit** — Setup wizard collected bridge credentials but never registered the user; bridge `matrix.Login()` fatally crashed the container on every first run
- **No admin user for Element X / ArmorChat** — `create-matrix-admin.sh` was copied but never called; chicken-and-egg problem with admin token requirement made it unusable
- **No Conduit health check after `docker compose up`** — Replaced `sleep 5` with a 120-second polling loop on `/_matrix/client/versions`, with progress indicators
- **`conduit.toml` server_name hardcoded** — Template always said `matrix.armorclaw.com`; now dynamically updated via `sed` to match user's domain/IP before copying to host
- **`set -e` in both scripts** — Removed from `container-setup.sh` and `quickstart.sh`; transient failures (curl timeouts, docker commands) no longer kill the container
- **`docker compose up -d` errors silently swallowed** — Replaced `2>/dev/null || true` with captured output that shows errors on failure
- **`create-matrix-admin.sh` broken API path** — Rewrote (v2.0.0) to use Conduit's Synapse-compatible shared-secret registration API instead of admin token/CLI dependency
- **Redundant setup flag** — Removed duplicate `touch $SETUP_FLAG` from `container-setup.sh`; `quickstart.sh` already handles it

### Added
- **Shared-secret user registration** — `register_matrix_user()` function uses Conduit's `/_synapse/admin/v1/register` endpoint with HMAC-SHA1 nonce authentication
- **Admin user creation in setup flow** — `configure_admin_user()` prompts for admin credentials; `register_users()` creates both bridge bot and admin user on Conduit
- **Conduit health check loop** — `wait_for_conduit()` polls for up to 120s with progress dots, handles first-run image pulls
- **Admin credentials in final summary** — Setup complete screen now shows Element X / ArmorChat login credentials with homeserver URL
- **Security: shared secret auto-cleanup** — Registration shared secret is injected into conduit.toml for user creation, then removed and Conduit restarted

### Changed
- **`create-matrix-admin.sh`** — Rewritten from v1.0.0 to v2.0.0; now uses shared-secret registration instead of admin token/CLI
- **`final_summary()`** — Now shows admin credentials, homeserver URL, and explicit Element X / ArmorChat login steps
- **`configs/conduit.toml`** — Added documented `registration_shared_secret` placeholder with usage instructions

---

## [0.3.0] - 2026-02-24 - Docker Deployment Hardening (5-Pass Review)

### Fixed
- **LD_PRELOAD security hook** — `security_hook.c` now checks `addr->sa_family` and allows `AF_UNIX` through, preventing bridge socket communication from being silently blocked
- **CGO_ENABLED=0 panic** — Bridge Dockerfile stages now build with `CGO_ENABLED=1` and include `gcc`, `libc-dev`, `libsqlite3-dev` for go-sqlcipher compatibility
- **Dockerfile.openclaw rewrite** — Removed invalid multi-stage references (`COPY --from=openclaw-src`), consolidated to working image
- **Coturn shell expansion** — Replaced YAML command array with shell form entrypoint for runtime expression evaluation
- **OpenClaw healthcheck** — Switched from `.sh` file with Python shebang to inline `CMD ["python3", "-c", "..."]`
- **RPC method mismatch** — Setup scripts used `keystore.add_provider` but bridge only handles `store_key`; fixed method name and added missing `id` param
- **Docker Compose V2 plugin path** — Installed to `/usr/lib/docker/cli-plugins/` instead of invalid symlink with space in name
- **ARMORCLAW_CONFIG env var** — Implemented in `parseFlags()` so documented env var actually works
- **Dockerfile COPY of non-existent file** — Removed `COPY deploy/setup-wizard.sh` that referenced a file that doesn't exist at that path
- **Docker bind mount host path resolution** — Parameterized compose paths with `${ARMORCLAW_CONFIGS:-./configs}` and added host-copy helper
- **External curl calls with no timeout** — All `curl` calls in setup scripts now include `--connect-timeout 5`
- **Socket path string slicing** — Replaced hardcoded length arithmetic with `filepath.Dir()`
- **Missing libsqlite3-0** — Added to runtime stage so bridge binary can dynamically link against SQLite
- **Provider detection lost** — `API_PROVIDER` now persisted to temp file and used in dynamic RPC calls instead of hardcoded `"openai"`
- **MATRIX_SERVER_NAME not exported** — Now exported before `docker compose up` so Conduit gets the user's actual server name
- **TURN_SECRET not generated** — Now generated via `openssl rand -hex 32` instead of using the placeholder default
- **API key shell expansion** — Switched from unquoted heredoc to `printf '%s'` to prevent `$` in keys from being interpreted
- **Interactive prompts exit on invalid input** — All required prompts (Matrix server, provider selection, API key, custom URL) now use retry loops with `print_warning` instead of `exit 1`

### Changed
- **nginx** — Added `profiles: ["frontend"]` so it only starts when explicitly requested
- **Setup wizard UX** — Invalid input re-prompts instead of killing the container; no single typo forces full restart

### Documentation
- Created `doc/LESSONS_LEARNED.md` — 18 lessons from all 5 review passes
- Moved `DOCKER-HUB-HOSTINGER-DEPLOYMENT.md` and `GITHUB-ACTIONS-DOCKERHUB-SETUP.md` to `doc/`

---

## [0.2.0] - 2026-02-19 - Phase 5: Zero-Trust Hardening

### Added
- **Zero-Trust System** (`bridge/pkg/trust/`)
  - `zero_trust.go` - Core trust verification engine with device fingerprinting
  - `device.go` - Device identification and tracking
  - `middleware.go` - Operation-level enforcement with configurable policies
  - `zero_trust_test.go` - 15 tests for trust verification

- **Audit System** (`bridge/pkg/audit/`)
  - `tamper_evident.go` - Hash-chain audit logging with integrity verification
  - `compliance.go` - 90-day retention, JSON/CSV export, compliance reports
  - `audit_helper.go` - CriticalOperationLogger for centralized logging
  - Tests: 28 tests for audit functionality

- **Trust Integration**
  - `bridge/internal/adapter/trust_integration.go` - Matrix adapter integration
  - Event verification in Matrix adapter (`matrix.go`)
  - Trust middleware integration in RPC server (`server.go`)

### Security Features
- Trust score levels (Untrusted → Verified, 0-4)
- Default enforcement policies for 8 operation types
- Anomaly detection (IP changes, impossible travel, sensitive access)
- Session lockout after failed verification attempts
- Audit logging for container, key, secret, config, auth operations

### Commits
```
da6b415 docs: Update review.md to 0.2.0 with Phase 5 completion
41a0479 feat(security): Add core trust and audit packages
2b81bad feat(security): Complete Phase 5 audit and zero-trust integration
```

---

## [0.1.5] - 2026-02-18 - Step 4: Push Notification Gateway

### Added
- **Push Gateway** (`bridge/pkg/push/`)
  - `gateway.go` - Multi-platform notification gateway
  - `providers.go` - FCM, APNS, WebPush implementations
  - `sygnal.go` - Matrix Sygnal client
  - `push_test.go` - 15 tests

- **Client Applications** (`applications/`)
  - ArmorChat - Android Matrix client with E2EE
  - ArmorTerminal - Terminal pairing application
  - Admin Panel - React/TypeScript web dashboard
  - Setup Wizard - React/TypeScript setup flow

### Commits
```
6c15fd6 feat: Add client applications and update documentation
```

---

## [0.1.4] - 2026-02-18 - Step 3: Enterprise Enforcement Layer

### Added
- **Enforcement System** (`bridge/pkg/enforcement/`)
  - `enforcement.go` - License-based feature access control
  - `middleware.go` - Request interception and validation
  - `rpc_handlers.go` - RPC methods for enforcement
  - `bridge_integration.go` - AppService hooks
  - Tests: 10 tests

### Commits
```
ca6699c feat: Add adapters, configs, tests, and deployment scripts
```

---

## [0.1.3] - 2026-02-18 - Step 2: Bridge AppService Implementation

### Added
- **AppService** (`bridge/pkg/appservice/`)
  - `appservice.go` - HTTP server for Matrix transactions
  - `bridge.go` - BridgeManager for SDTW adapter coordination
  - `client.go` - Matrix client wrapper
  - Ghost user namespaces (@slack_*, @discord_*, @teams_*, @whatsapp_*)
  - Tests: 16 tests

### Commits
```
ca6699c feat: Add adapters, configs, tests, and deployment scripts
```

---

## [0.1.2] - 2026-02-18 - Step 1: Matrix Infrastructure

### Added
- **Deployment Configs** (`configs/`)
  - `synapse/homeserver.yaml` - Synapse configuration
  - `coturn/turnserver.conf` - TURN/STUN for WebRTC
  - `nginx/matrix.conf` - Reverse proxy with TLS
  - `postgres/postgresql.conf` - Database optimization
  - `appservices/bridge-registration.yaml` - AppService registration

- **Deployment Scripts** (`deploy/matrix/`)
  - `deploy-matrix.sh` - Automated Matrix deployment
  - `docker-compose.matrix.yml` - Production compose

### Commits
```
ca6699c feat: Add adapters, configs, tests, and deployment scripts
```

---

## [0.1.1] - 2026-02-18 - Critical Bug Fixes

### Fixed
- **LLM Response PHI Scrubbing** - Tier-dependent compliance for outbound responses
  - `bridge/pkg/pii/llm_compliance.go` - LLM response compliance handler
  - `bridge/pkg/pii/errors.go` - Structured compliance error types

- **License Activation Race Condition** - Transaction-based activation
  - `license-server/main.go` - SELECT FOR UPDATE pattern

- **Budget Tracker Persistence** - WAL-based durability
  - `bridge/pkg/budget/persistence.go` - Write-ahead logging
  - `bridge/pkg/budget/tracker.go` - Fsync on critical writes

- **Quarantine Notifications** - Callback support
  - `bridge/pkg/pii/hipaa.go` - QuarantineNotifier callback

- **Code Quality** - Atomic operations, structured errors
  - Replaced sync.RWMutex with atomic operations
  - Added component context to all logs

### Commits
```
2205375 chore: Update core packages with Phase 4/5 changes
```

---

## [3.0.0] - 2026-02-17 - Phase 4: Enterprise Features

### Added
- **License Server** (`license-server/`)
  - PostgreSQL-backed license validation
  - Rate limiting, grace periods, machine binding
  - Tests: 15 tests

- **HIPAA Compliance** (`bridge/pkg/pii/`)
  - `hipaa.go` - PHI detection (SSN, CC, MRN patterns)
  - Severity-based action routing
  - Tests: 15 tests

- **SSO Integration** (`bridge/pkg/sso/`)
  - `sso.go` - SAML 2.0 and OIDC authentication
  - Role mapping from attributes
  - Tests: 13 tests

- **Web Dashboard** (`bridge/pkg/dashboard/`)
  - `dashboard.go` - Embedded HTTP server
  - Static HTML templates
  - Tests: 19 tests

### Commits
```
c6e7efa feat(enterprise): Add Phase 4 enterprise packages
```

---

## [2.0.0] - 2026-02-15 - Sprint 2: All Gaps Resolved

### Added
- **Error Handling System** (`bridge/pkg/errors/`)
  - Structured error codes (CTX-XXX, MAT-XXX, RPC-XXX, SYS-XXX, BGT-XXX, VOX-XXX)
  - Component-scoped event tracking with ring buffers
  - SQLite persistence
  - 3-tier admin resolution chain
  - RPC methods: `get_errors`, `resolve_error`

- **Documentation** (11 GAPs resolved)
  - Getting Started guide (GAP #1)
  - Platform deployment guides (GAP #2)
  - API key validation (GAP #3)
  - QR scanning flow (GAP #4)
  - Multi-device UX (GAP #5)
  - Account recovery (GAP #6)
  - Error escalation (GAP #7)
  - Platform onboarding (GAP #8)
  - Slack adapter (GAP #9)
  - Alert integration (GAP #10)
  - Security tier upgrade (GAP #11)

### Commits
```
3c0493e docs: Update progress.md with Sprint 2 Complete milestone
b7781d8 docs: Update review.md to v2.0.0 - All Gaps Resolved
519d8fd docs: Add security tier upgrade guide (GAP #11 resolved)
fd2fb5c docs: Add QR scanning flow guide (GAP #4 resolved)
a9b3780 docs: Add API key validation guide (GAP #3 resolved)
ba9bbbd docs: Add multi-device UX guide (GAP #5 resolved)
62aaca8 docs: Add alert integration guide (GAP #10 resolved)
e0fc171 docs: Add Getting Started guide (GAP #1 resolved)
5b07d54 docs: Update user journey gap analysis (v2.0.0)
c8cee3c docs: Add error handling system documentation
dde4b7f feat(errors): integrate error system with Matrix adapter
2d1cbb4 feat(errors): integrate error system with docker client
a0c7023 feat: Add get_errors and resolve_error RPC methods
a00b577 feat: Integrate error handling system with bridge main
a183a63 feat(errors): Add package initialization and documentation
d9a1a35 feat(errors): Add notification sender
7727492 feat(errors): Add error persistence store
3051240 feat(errors): Add admin resolution chain
ec1d940 feat(errors): Add smart sampling registry
203afd0 feat(errors): Add component-scoped event tracking
aac1d4e feat(errors): Add core error types and builder
d2f3542 docs: Add error handling system design
```

---

## [1.5.0] - 2026-02-12 - P0 Critical Fixes

### Fixed
- **P0-CRIT-3**: Memory-only Unix socket injection
  - `bridge/pkg/secrets/socket.go` - SecretInjector with Unix domain sockets
  - Secrets never written to disk, transmitted via socket only
  - RPC method: `send_secret`

- **P0-CRIT-1**: Egress proxy for SDTW adapters
  - Outbound traffic through proxy only
  - No direct container internet access

### Commits
```
9f2b13d feat: Add Agent.SendSecret RPC method (P0-CRIT-3)
458e80c feat: Implement memory-only Unix socket injection (P0-CRIT-3)
db86c8e feat: Implement egress proxy for SDTW adapters (P0-CRIT-1)
```

---

## [1.4.0] - 2026-02-11 - SDTW Message Queue

### Added
- **Message Queue** (`bridge/internal/queue/`)
  - SQLite-based persistent queue with WAL
  - Retry logic with exponential backoff
  - Dead letter queue (DLQ) support
  - Circuit breaker pattern

### Commits
```
c2132e0 Add production-ready features to SDTW message queue
3dfc6cc Implement full SQLite message queue with retry logic and DLQ support
441f177 Fix SDTW queue package compilation errors
9fe233c Add SDTW message queue stub implementation
dcad27d Add SDTW Message Queue specification and tasks
```

---

## [1.3.0] - 2026-02-08 - WebRTC Voice Implementation

### Added
- **WebRTC Engine** (`bridge/pkg/webrtc/`)
  - `engine.go` - Pion-based WebRTC implementation
  - `session.go` - Session management
  - `token.go` - Token-based authorization

- **TURN Server** (`bridge/pkg/turn/`)
  - `turn.go` - Coturn integration
  - NAT traversal support

### Commits
```
ec813c4 fix(voice): Resolve structural issues in voice package
```

---

## [1.2.0] - 2026-02-07 - Security Enhancements

### Added
- **Zero-Trust Middleware** - Trusted senders/rooms validation
- **Budget Guardrails** - Token-aware budget tracking
- **Container TTL** - Auto-cleanup with heartbeat

### Commits
```
[Historical commits for security enhancements]
```

---

## [1.1.0] - 2026-02-06 - Setup Wizard

### Added
- **Interactive Setup** (`deploy/setup-wizard.sh`)
  - 10-step guided installation
  - System requirements validation
  - Docker installation/verification
  - Keystore initialization

---

## [1.0.0] - 2026-02-05 - Phase 1 Complete

### Core Features
- **Encrypted Keystore** - SQLCipher + XChaCha20-Poly1305
- **Docker Client** - Scoped operations (create, exec, remove)
- **Matrix Adapter** - E2EE support with Conduit/Synapse
- **JSON-RPC Server** - 11 core methods
- **Configuration System** - TOML + environment variables
- **Container Entrypoint** - Secrets validation, fail-fast

### Build Artifacts
- Bridge binary: 31 MB
- Container image: 393 MB (98.2 MB compressed)

---

## Historical Plans (Superseded)

The following planning documents have been superseded by implementation:

| Document | Status | Superseded By |
|----------|--------|---------------|
| `2026-02-05-minimal-bridge-spec.md` | Implemented | v1.0.0 |
| `2026-02-05-robust-bridge-spec.md` | Implemented | v1.0.0 |
| `2026-02-05-secureclaw-v1-design.md` | Renamed | ArmorClaw |
| `2026-02-05-local-bridge-matrix-gateway.md` | Implemented | v1.2.0 |
| `2026-02-05-license-server-api.md` | Implemented | v3.0.0 |
| `2026-02-07-swarmclaw-design.md` | Deferred | Future |
| `2026-02-08-armorclaw-renaming-plan.md` | Complete | N/A |
| `2026-02-08-webrtc-voice-implementation.md` | Implemented | v1.3.0 |
| `2026-02-10-armorclaw-fix-plan.md` | Complete | v2.0.0 |
| `2026-02-11-missing-rpc-methods-implementation.md` | Complete | v2.0.0 |
| `2026-02-15-error-handling-system-design.md` | Implemented | v2.0.0 |
| `2026-02-15-error-handling-implementation-tasks.md` | Complete | v2.0.0 |
| `2026-02-16-first-boot-security-configuration.md` | Implemented | v3.2.0 |
| `SDTW_Adapter_Implementation_Plan_v2.0.md` | In Progress | - |
| `SDTW_MessageQueue_Specification.md` | Implemented | v1.4.0 |

---

**Changelog Last Updated:** 2026-02-19
