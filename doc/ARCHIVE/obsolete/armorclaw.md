# ArmorClaw System Documentation

> **Purpose**: LLM-readable comprehensive documentation for ArmorClaw architecture, components, APIs, and security.
>
> **Version**: 0.8.0
>
> **Last Updated**: 2026-04-27

> ⚠️ **Architecture Note (v0.4.1)**: Agent containers always run with `NetworkMode: "none"` (no network access). Structured results are passed via `result.json` in the bind-mounted state dir (backward channel). Browser automation runs through the Jetski sidecar, a separate container with its own network stack; agent containers never perform browser operations directly.
>
> **v0.8.0 Changes**: Java Apache POI sidecar for legacy DOC/PPT extraction, Java routing in Go Bridge (`RouteExtractText` 4-param signature), 106 sidecar tests (22 Go routing + 8 Java JUnit 5 + 7 Go Python E2E + 4 Go Java E2E + 65 Python).
>
> **v0.7.0 Changes**: WorkflowStep.Input field for inter-step data passing, warm dispatch dead code purged, WebSocket EventBus wiring, Android DeepLinkHandler, SecurityConfigViewModel wired, BridgeRepository persistence, admin panel real API, ArmorChat integration test cleanup.
>
> **v0.6.0 Changes**: Bridge-side state inference, Rust Vault deployment, parallel execution, session compaction, step failover, email approval, PPTX Rust migration, v6 audit mode.

---

## Context Routing Rules

> **RULE**: Before modifying any subsystem, you MUST read the source files listed below. Do not plan changes from this document alone.

| Task | Required Reading |
|------|-----------------|
| Modify PII injection / BlindFill | `bridge/pkg/pii/` and Jetski CDP proxy (`jetski/`) |
| Add or change RPC methods | `bridge/pkg/rpc/server.go` and `docs/reference/rpc-api.md` |
| Change agent state transitions | `bridge/pkg/agent/state.go` and `bridge/pkg/agent/state_machine.go` |
| Modify Matrix event handling | `bridge/internal/adapter/` and `bridge/pkg/matrix/` |
| Update JetSki CDP proxy | `jetski/internal/cdp/proxy.go` and `jetski/internal/security/pii_scanner.go` |
| Change keystore encryption | `bridge/pkg/keystore/keystore.go` (Go) and `rust-vault/src/db/` (Rust) |
| Add browser skills | `container/openclaw-src/skills/` and `browser-service/src/` |
| Modify container hardening | `container/Dockerfile.openclaw`, `container/seccomp-profile.json`, `container/apparmor-profile` |
| Update Android client | `applications/ArmorChat/app/` |
| Change deployment scripts | `deploy/` and `.skills/` |
| Modify MCP Router / tool routing | `bridge/pkg/mcp/router.go` and `bridge/pkg/interfaces/skillgate.go` |
| Change vault governance integration | `bridge/pkg/vault/proto/` and `rust-vault/src/governance/` |
| Add tool sidecar isolation | `bridge/pkg/toolsidecar/toolsidecar.go` (implemented, v6-gated) |
| Modify Governor-Shield PII interception | `bridge/pkg/governor/skillgate.go`, `bridge/pkg/governor/types.go`, and `bridge/pkg/pii/` |
| Add or modify voice services (STT/TTS/VAD) | `bridge/pkg/voice/stt_service.go`, `bridge/pkg/voice/tts_service.go`, `bridge/pkg/voice/vad_service.go` |
| Modify Python office sidecar | `sidecar-python/worker.py`, `sidecar-python/interceptor.py`, `bridge/pkg/sidecar/office_client.go` |
| Add or manage scheduled tasks | `bridge/pkg/secretary/store.go` and `bridge/pkg/secretary/task_scheduler.go` |
| Execute or modify workflow steps | `bridge/pkg/secretary/orchestrator_integration.go` |
| Modify sovereign email pipeline | `bridge/pkg/email/` and `bridge/cmd/mta-recv/` and `doc/email-pipeline.md` |
| Change PII masking rules | `bridge/pkg/pii/masker.go` |
| Modify OAuth token storage | `bridge/pkg/keystore/oauth.go` |
| Change bridge-local step execution | `bridge/pkg/secretary/bridge_local_registry.go` |
| Modify browser broker or chart pipeline | `bridge/pkg/browser/broker.go`, `bridge/pkg/browser/jetski_broker.go`, `bridge/pkg/browser/chart_types.go` |
| Add or modify NavChart validation | `bridge/pkg/browser/chart_validator.go` and `bridge/pkg/browser/normalizer.go` |
| Change chart persistence | `bridge/pkg/browser/chart_store.go` and `bridge/pkg/secretary/store.go` |

---

## Deprecation Notices

- **Warm dispatch** (removed v0.7.0): `warmDispatch()`, `GetRunningInstance()`, `EventTypeTaskDispatch`, `BuildTaskDispatchPayload()`, and `task_dispatch.go` were deleted from `bridge/pkg/secretary/`. All dispatch is cold-only. This was required because `NetworkMode: none` prevents containers from receiving inbound Matrix events, making warm dispatch architecturally impossible.

---

## Table of Contents

1. [Executive Summary](#executive-summary)
2. [Deployment Skills for AI CLI Tools](#deployment-skills-for-ai-cli-tools)
3. [System Architecture](#system-architecture)
4. [Go Bridge Component](#go-bridge-component)
5. [SQLCipher Keystore](#sqlcipher-keystore)
6. [Rust Vault Sidecar](#rust-vault-sidecar)
7. [Phase 2: Secure Document Pipeline](#phase-2-secure-document-pipeline)
8. [Sovereign Email Pipeline](#sovereign-email-pipeline)
 9. [Matrix Conduit Control Plane](#matrix-conduit-control-plane)
 10. [Security Architecture](#security-architecture)
 11. [Governor-Shield PII Interception](#governor-shield-pii-interception)
 12. [Component Integration Patterns](#component-integration-patterns)
 13. [Agent Communication Model](#agent-communication-model)
 14. [Agent Studio](#agent-studio)
 15. [Browser Automation (BrowserBroker)](#browser-automation-browserbroker)
 16. [Jetski Browser Sidecar](#jetski-browser-sidecar)
 17. [v6 Microkernel Governance](#v6-microkernel-governance-feature-flagged)
 18. [Rust Office Sidecar](#rust-office-sidecar)
 19. [ArmorChat Android Client](#armorchat-android-client)
 20. [OpenClaw Agent Runtime](#openclaw-agent-runtime)
 21. [RPC API Reference](#rpc-api-reference)
 22. [Event Types Reference](#event-types-reference)
 23. [Configuration Reference](#configuration-reference)
 24. [Deployment Modes](#deployment-modes)
 25. [Testing & Verification](#testing--verification)
 26. [Local Development Guide](#local-development-guide)
 27. [Document Index](#document-index)
 28. [Agent State Machine (Go Bridge)](#agent-state-machine-go-bridge)
 29. [Review Documentation](#review-documentation)

---

<component id="executive-summary">
## Executive Summary
</component>

### What is ArmorClaw?

ArmorClaw is a **VPS-based AI secretary platform** that runs AI agents 24/7 on your server, controlled from your phone. It enables automated web browsing, form filling, and task management with **human-in-the-loop approval** for sensitive operations.

### Core Value Proposition

**Problem**: Traditional AI agents see your passwords and credit cards when you give them access to perform tasks.

**Solution**: ArmorClaw's **BlindFill™** technology injects secrets directly into the browser. The agent requests "credit card" but never sees the actual number—it goes straight from encrypted storage to the form field.

### Key Features

| Feature | Description |
|---------|-------------|
| **BlindFill™** | Memory-only secret injection, agents never see raw values |
| **Placeholder Masking** | Strict `{{VAULT:field:hash}}` format prevents secret exposure |
| **Prompt Injection Detection** | 3 pattern detectors (unicode tricks, random chars, repetition) |
| **Kill-on-Violation** | Terminate compromised containers via RPC *(post-hoc: detected via exit code, not reactive)* |
| **USB Security Validation** | 2 security tests for ShadowMap gatekeeper and vault hold-to-reveal |
| **E2EE Messaging** | All communication via Matrix protocol with Megolm encryption |
| **Container Isolation** | Each agent runs in hardened Docker container |
| **Human-in-the-Loop** | Mobile approval for sensitive operations (payments, PII) |
| **SQLCipher Keystore** | Hardware-bound encrypted credential storage |
| **No-Code Agent Studio** | Define agents via chat commands or dashboard |
| **52 Browser Skills** | Chrome DevTools MCP integration for web automation via Jetski sidecar |
| **Sentinel Mode** | Automatic VPS deployment with Let's Encrypt TLS |
| **Split-Storage RAG** | Document chunks stored separately from vector embeddings |
| **YARA Content Disarm** | Malicious content detected and neutralized before processing |
| **TTL Proxy Guard** | Ephemeral tokens (30 min TTL) for sidecar communication |
| **Jetski CDP Proxy** | Tethered Mode browser proxy with PII scrubbing and encrypted sessions |

<component id="component-overview">
### Component Overview

| Component | Language | Purpose | Entry Point |
|-----------|----------|---------|-------------|
| **Go Bridge** | Go | Central orchestrator, RPC server, container manager | `bridge/cmd/bridge/main.go` |
| **SQLCipher Keystore** | Go | Encrypted credential storage with hardware binding | `bridge/pkg/keystore/keystore.go` |
| **Matrix Conduit** | Rust | Matrix homeserver for E2EE messaging | Conduit binary |
| **BrowserBroker** | Go | Browser automation abstraction layer over Jetski CDP proxy | `bridge/pkg/browser/broker.go` |
| **OpenClaw Runtime** | TypeScript/Node | AI agent runtime in containers | `container/openclaw-src/openclaw.mjs` |
| **License Server** | Go | Enterprise license validation | `license-server/main.go` |
| **ArmorChat** | Kotlin | Android mobile client | `applications/ArmorChat/` |
| **Jetski Sidecar** | Go | CDP proxy with Tethered Mode security | `jetski/cmd/observer/main.go` |
| **Rust Vault** | Rust | Security enclave, governance gRPC, BlindFill service | `rust-vault/src/main.rs` |
| **Python MarkItDown Sidecar** | Python | Legacy Office format conversion (MSG, XLS). DOC/PPT migrated to Java. XLSX/PPTX migrated to Rust. | `sidecar-python/worker.py` |
| **Java Apache POI Sidecar** | Java | Legacy DOC/PPT text extraction via Apache POI (HWPFDocument, HSLFSlideShow) | `sidecar-java/src/main/java/com/armorclaw/sidecar/` |
| **Email Approval** | Go (Bridge) | Email-based HITL approval for sensitive operations | `bridge/pkg/rpc/email_approval.go`, `bridge/pkg/email/` |
</component>

---

## Deployment Skills for AI CLI Tools

### Overview

ArmorClaw includes **built-in deployment skills** that let coding agents like Claude Code, OpenCode, Cursor, and Crush deploy and manage your VPS secretary platform.

All skills use **shell variable interpolation** (`${variable}`) for consistency across platforms.

### Available Skills

| Skill | Purpose | Command | Key Parameters |
|-------|---------|---------|----------------|
| **Deploy** | Deploy ArmorClaw to VPS | `/deploy vps_ip=...` | `vps_ip`, `ssh_key`, `domain`, `mode` |
| **Status** | Check deployment health | `/status vps_ip=...` | `vps_ip`, `ssh_key`, `domain`, `verbose` |
| **Cloudflare** | Configure HTTPS | `/cloudflare domain=...` | `domain`, `mode`, `cf_api_token` |
| **Provision** | Connect mobile device | `/provision vps_ip=...` | `vps_ip`, `expiry`, `show_url` |

### Automation Levels

| Level | Behavior | Examples |
|-------|----------|----------|
| `auto` | Execute immediately | Health checks, status, OS detection |
| `confirm` | Ask user before executing | SSH connection, running installer |
| `guide` | Provide instructions | Account creation, DNS setup |

### Skills Directory

```
.skills/
├── deploy.yaml          # Deployment skill definition
├── deploy/SKILL.md      # AI-friendly instructions
├── status.yaml          # Status check skill
├── status/SKILL.md      # Status documentation
├── cloudflare.yaml      # Cloudflare setup skill
├── cloudflare/SKILL.md  # Cloudflare guide
├── provision.yaml       # Mobile provisioning skill
├── provision/SKILL.md   # Provisioning guide
├── PLATFORM.md          # Cross-platform patterns
├── TEMPLATE.yaml        # Schema for new skills
└── README.md            # Skills index
```

---

## System Architecture

### High-Level Architecture Diagram

```
┌───────────────────────────────────────────────────────────────────────┐
│                         THE VPS (Office)                              │
│                                                                       │
│  Agent containers: env vars → container (NetworkMode: "none") → exit code + result.json  │
│  Browser automation: Agent → Bridge RPC → Jetski CDP proxy → Lightpanda browser       │
│                                                                       │
│  ┌─────────────┐  env vars   ┌─────────────┐       ┌─────────────┐   │
│  │ ArmorClaw   │────────────▶│  OpenClaw    │       │   Jetski    │   │
│  │ Bridge      │◀──exit code─│  (Agent)     │ CDP   │ CDP Proxy   │   │
│  │ (Orchestr.) │             │ NetworkMode: │ :9222 │ (Tethered)  │   │
│  └──────┬──────┘             │  "none"      │       └──────┬──────┘   │
│         │                    └──────┬──────┘              │           │
│         │                           │                     │           │
│         │   BlindFill Engine        │                     │           │
│         │   (Memory-Only)           │                     │           │
│         │                           │                     │           │
│         │    ┌──────────────────────┘                     │           │
│         │    │ Rust Vault Sidecar                         │           │
│         │    │ (gRPC/Unix Socket)                         │           │
│         │    │ - Zeroization                              │           │
│         │    │ - Network BlindFill                        │           │
│         │    │ - Circuit Breaking                         │           │
│         │    └────────────────────────────────────────────┘           │
│         │                                                             │
│         │    ┌───────────────────┐                                    │
│         │    │ Phase 2 Pipeline  │                                    │
│         │    │ - Split-Storage   │                                    │
│         │    │ - YARA CDR        │                                    │
│         │    │ - TTL Proxy Guard │                                    │
│         │    └───────────────────┘                                    │
│         │                                                             │
└─────────┼─────────────────────────────────────────────────────────────┘
          │
          │ Secure Matrix Tunnel (E2EE)
          │
┌─────────▼─────────────────────────────────────────────────────────────┐
│                         USER (Mobile)                                 │
│   ArmorChat App                                                      │
│   "Book a flight to NYC"  [Approve Credit Card] 🔐                   │
└───────────────────────────────────────────────────────────────────────┘
```

### Directory Structure

```
armorclaw-omo/
├── bridge/                    # Go Bridge orchestrator (67 packages)
│   ├── cmd/bridge/main.go    # Primary entry point (3,538 lines)
│   ├── pkg/                  # Public packages
│   │   ├── rpc/              # JSON-RPC 2.0 server (93 registered methods)
│   │   ├── keystore/         # SQLCipher encrypted storage
│   │   ├── pii/              # BlindFill engine
│   │   ├── studio/           # Agent container management
│   │   ├── eventbus/         # Event broadcasting
│   │   ├── matrix/           # Matrix client
│   │   ├── browser/          # Browser automation
│   │   │   ├── broker.go       # BrowserBroker interface (15 methods)
│   │   │   ├── broker_types.go # JobID, StartJobRequest, FillRequest, etc.
│   │   │   ├── jetski_broker.go # JetskiBroker implementation + ReplayChart
│   │   │   ├── broker_handler.go # BrokerHandler + FallbackHandler adapters
│   │   │   ├── chart_types.go  # NavChart Go types matching chartmaker schema
│   │   │   ├── chart_store.go  # ChartStore persistence (SQLite)
│   │   │   ├── chart_validator.go # PII/policy validation
│   │   │   ├── chart_audit.go  # Lifecycle audit trail
│   │   │   ├── normalizer.go   # CDP-to-NavChart normalization pipeline
│   │   │   ├── pii_scanner.go  # Chart PII diagnostic scanner
│   │   │   ├── processor.go    # Browser job processor (queue integration, agent state machine)
│   │   │   ├── context_manager.go # Agent-to-browser-context mapping
│   │   │   ├── handler.go      # Bridge-local browser_execute handler (BrowserIntent artifact)
│   │   │   ├── client.go       # HTTP client for legacy browser-service communication
│   │   │   └── browser.go      # Browser status tracking and event emission
│   │   ├── provisioning/     # Mobile provisioning
│   │   ├── trust/            # Zero-trust verification
│   │   ├── audit/            # Audit logging
│   │   ├── mcp/              # MCP Router with SkillGate (v6 microkernel)
│   │   ├── vault/proto/      # Governance gRPC client (v6 microkernel)
│   │   ├── toolsidecar/      # Tool sidecar provisioning (v6-gated)
│   │   └── ... (50 more)
│   └── internal/             # Internal implementation (17 packages)
│       ├── adapter/          # Matrix/Slack adapters
│       ├── ai/               # AI service
│       ├── skills/           # Built-in skills
│       └── agent/            # Agent runtime
│
├── rust-vault/               # Rust security enclave (library crate, not a deployed service)
│   ├── src/
│   │   ├── db/               # SQLCipher vault + matrix state databases
│   │   ├── governance/       # gRPC governance service (ephemeral tokens)
│   │   └── grpc/             # gRPC server with mTLS auth
│   ├── proto/governance.proto
│   └── tests/                # 33 tests (config, vault, mTLS — blindfill tests removed v0.9.0)
│
├── jetski/                    # Go CDP proxy with Tethered Mode security
│   ├── cmd/observer/main.go  # Primary entry point
│   ├── internal/cdp/          # CDP proxy, router, translator, PII scanner
│   ├── internal/rpc/          # RPC API (port 9223)
│   ├── internal/security/     # SQLCipher sessions, PII scanning, session management
│   ├── internal/approval/     # Matrix HITL approval client
│   ├── internal/sonar/        # Telemetry recorder, reporter, buffer
│   ├── internal/subprocess/   # Process manager, watchdog, health checker, restarter
│   ├── internal/network/      # Proxy manager, circuit breaker
│   ├── pkg/config/            # Jetski configuration package
│   ├── pkg/logger/            # Structured logging package
│   ├── lighthouse/            # Nav-Chart REST API (Go sub-project)
│   ├── jetski-chartmaker/     # Browser interaction recorder (TypeScript CLI)
│   ├── configs/config.yaml    # Configuration
│   ├── Dockerfile             # Container build
│   └── go.mod                 # Standalone module (github.com/armorclaw/jetski)
│
├── go.work                    # Multi-module Go workspace
│
├── container/openclaw-src/   # OpenClaw agent runtime
│   ├── extensions/           # 37 platform adapters
│   └── skills/               # Browser skills
│
├── applications/             # Client applications
│   ├── ArmorChat/           # Android Kotlin client
│   ├── ArmorTerminal/       # Play Store publishing assets and checklist
│   └── admin-panel/         # Web dashboard
│
├── deploy/                   # Deployment scripts (32 scripts)
│   └── install.sh           # One-command installer
│
├── sidecar-python/           # Python MarkItDown sidecar (legacy Office formats)
│   ├── worker.py            # gRPC server with ExtractText, threshold streaming, TTL recycling
│   ├── interceptor.py       # HMAC-SHA256 token validation interceptor
│   ├── conftest.py          # Test fixtures for 6 document formats
│   ├── test_worker.py       # 27 server unit tests
│   ├── test_edge_cases.py   # 16 edge case tests
│   ├── test_interceptor.py  # 12 interceptor tests
│   └── proto/               # Generated gRPC stubs from sidecar.proto
│
├── sidecar-java/             # Java Apache POI sidecar (legacy DOC/PPT extraction)
│   ├── pom.xml              # Maven build with POI 5.3.0, gRPC-Java, JUnit 5, Mockito
│   ├── src/main/java/com/armorclaw/sidecar/  # ServerMain, ExtractorServiceImpl, interceptors
│   ├── src/test/java/com/armorclaw/sidecar/  # 8 JUnit 5 tests + TestFixtures (base64 DOC/PPT)
│   └── src/main/proto/         # sidecar.proto (Apache POI + gRPC stubs)
│
├── .skills/                  # AI CLI deployment skills
│   ├── deploy.yaml
│   ├── status.yaml
│   ├── cloudflare.yaml
│   ├── provision.yaml
│   ├── ops.yaml
│   └── TEMPLATE.yaml
│
└── tests/                    # Test suites
    ├── ssh/                 # VPS testing suite (10 categories)
    ├── e2e/                 # E2E test utilities (common.sh)
    ├── unit/                # Unit tests
    ├── integration/         # Installer integration tests
    ├── adversarial/         # Security adversarial tests
    ├── voice/               # Voice soak/load testing
    ├── voice-services/      # Voice service configs
    ├── config/              # Test configuration files
    ├── e2ee/                # E2EE cross-client testing
    ├── matrix-test-server/  # Matrix test server
    ├── lib/                 # Shared test fixtures (load_env, assert_json, restart_bridge, etc.)
    ├── fixtures/            # Shared test data
    ├── test-eventbus-streaming.sh      # T1: EventBus + WebSocket (Tier A)
    ├── test-trust-layer.sh             # T2: Security / Trust Layer (Tier A)
    ├── test-system-health-baseline.sh  # T2.5: Health baseline (Tier A)
    ├── test-secretary-workflow-core.sh # T3a: Workflow core (Tier A)
    ├── test-email-pipeline.sh          # T4: Email approval (Tier A)
    ├── test-secretary-workflow-deep.sh # T3b: Workflow deep (Tier B)
    ├── test-sidecar-docs.sh            # T5: Document pipeline (Tier B)
    ├── test-voice-stack.sh             # T6: Voice stack (Tier B)
    ├── test-jetski-sidecar.sh          # T7: Browser sidecar (Tier B)
    ├── test-license-enforcement.sh     # T8: License enforcement (Tier B)
    ├── test-platform-adapters.sh       # T9: Platform adapters (Tier B)
    ├── test-agent-runtime.sh           # T10: Agent runtime (Tier B)
    ├── test-deployment-usb.sh          # T11: Deployment / USB (Tier B)
    └── test-cross-*.sh                # X1-X4: Cross-subsystem integration
```

### Communication Patterns

| Pattern | Protocol | Purpose | Port/Path |
|---------|----------|---------|-----------|
| **Matrix Protocol** | Matrix Client-Server API v3 | E2EE messaging, control plane | 6167 |
| **JSON-RPC 2.0 (Native)** | Unix domain socket | Internal component communication | `/run/armorclaw/bridge.sock` |
| **JSON-RPC 2.0 (Sentinel)** | TCP | Public API access (via Caddy proxy) | `0.0.0.0:8080` |
| **Docker Socket** | Docker Engine API | Container lifecycle management | `/var/run/docker.sock` |
| **gRPC (v6 Vault)** | gRPC over Unix socket | Vault governance: ephemeral tokens, zeroization (v6 only) | `/run/armorclaw/rust-vault.sock` |
| **HTTP/WebSocket** | REST + WebSocket | Health checks, metrics, real-time event streaming (v0.7.0: EventBus wired) | 8080 |
| **WebRTC** | ICE/STUN/TURN | Voice/video calls | Dynamic |
| **CDP WebSocket** | Chrome DevTools Protocol | Browser automation (agent → Bridge → Jetski CDP proxy → Lightpanda) | 9222 |
| **Jetski RPC** | JSON-RPC 2.0 (HTTP) | Jetski sidecar status, sessions, health | 9223 |
| **Lightpanda Engine** | CDP over WebSocket | Headless browser engine (Jetski internal) | 9333 |

---

## Go Bridge Component

### Purpose

The Go Bridge is the **central orchestrator** that coordinates between the host system and isolated AI agent containers. It provides:
- Secure credential management via SQLCipher
- JSON-RPC 2.0 API (93 registered methods across 18 domains)
- Matrix integration for encrypted messaging
- Browser automation job queue
- Skill execution with allowlist control
- PII approval workflow

### Main Structure

**File**: `bridge/pkg/rpc/server.go`

```go
type Server struct {
    // Integration dependencies
    keystore          Keystore
    matrix            MatrixAdapter
    aiService         *ai.AIService
    aiSemaphore       chan struct{}
    aiMaxConcurrent   int
    bridgeMgr         BridgeManager
    browserJobs       *BrowserJobManager
    browserBroker     browser.BrowserBroker
    studio            StudioService
    appService        AppService
    provisioningMgr   ProvisioningManager
    skillMgr          SkillManager
    skillGate         interfaces.SkillGate
    mcpRouter         *mcp.MCPRouter
    translator        *translator.RPCToMCPTranslator
    eventBus          *eventbus.EventBus
    hardeningStore    trust.Store
    deviceStore       *trust.DeviceStore
    inviteStore       *invite.InviteStore
    secretaryHandler  secretaryRPCHandler
    heartbeats        sync.Map
    metrics           *Metrics
    
    // Network / lifecycle
    handlers          map[string]HandlerFunc  // 93 registered methods
    listener          net.Listener
    shutdownCh        chan struct{}
    rpcTransport      string
    listenAddr        string
    
    // Security / governance
    dockerClient      *docker.Client
    guard             *trust.TrustedProxyGuard
    auditLog          *audit.AuditLog
    governanceRoomID  string
    tlsInfoProvider   TLSInfoProvider
    
    // PII
    piiRequestManager *keystore.PIIRequestManager
}
```

### Package Index

#### Control Plane
| Package | Purpose |
|---------|---------|
| `pkg/rpc/` | JSON-RPC 2.0 server with all method handlers |
| `pkg/eventbus/` | Event broadcasting to WebSocket clients (v0.7.0: wired to WebSocket for live push) |
| `pkg/config/` | TOML configuration management |
| `pkg/logger/` | Structured logging |
| `pkg/secretary/` | Workflow engine with 29 source files: RPC dispatch (13 secretary.* + 4 task.* methods), orchestrator (sequential + parallel + dependencies), approval engine, PII approval blocking, event reader (_events.jsonl tailer), task scheduler (15s tick), template engine, BlindFill integration, browser integration, calendar service, WebDAV service, rolodex, trusted workflows, bridge-local registry, doc query registration, studio integration, audit, cleanup, notifications, store (SQLite), result types, secretary commands (v0.7.0: WorkflowStep.Input for inter-step data passing) |
| `pkg/health/` | Health check and readiness monitoring |
| `pkg/runtime/` | Bridge runtime configuration and lifecycle |

#### AI & Skills
| Package | Purpose |
|---------|---------|
| `internal/ai/` | AI provider clients (OpenAI, Anthropic, OpenRouter, etc.) |
| `internal/skills/` | Built-in skills (web_search, calendar, email, data_analyze) |
| `pkg/skills/` | Skill registry and management |
| `pkg/interfaces/skillgate.go` | PII interception interface |

#### Security & Trust
| Package | Purpose |
|---------|---------|
| `pkg/pii/` | BlindFill engine for secure PII injection |
| `pkg/keystore/` | SQLCipher encrypted credential storage |
| `pkg/trust/` | Zero-trust device verification |
| `pkg/security/` | Website guard and security policies |
| `pkg/enforcement/` | License validation and enforcement |
| `pkg/lockdown/` | Admin reset mode |
| `pkg/yara/` | YARA-based content disarm and reconstruction scanner |
| `pkg/securerandom/` | Cryptographically secure random number generation |
| `pkg/crypto/` | Encryption and key management utilities |

#### Communication
| Package | Purpose |
|---------|---------|
| `internal/adapter/` | Matrix, Slack adapters (messaging platforms) |
| `internal/sdtw/` | SDTW adapters — Discord, Teams, WhatsApp (uniform interface, HMAC signatures) |
| `internal/queue/` | Persistent message queue (SQLite WAL) for SDTW adapters |
| `pkg/matrix/` | Matrix client library |
| `pkg/appservice/` | Matrix AppService bridges |
| `pkg/provisioning/` | Mobile device provisioning via QR |
| `pkg/ghost/` | Ghost user management |
| `pkg/push/` | Mobile push notifications via Matrix Sygnal |
| `pkg/sso/` | Single sign-on authentication |
| `pkg/websocket/` | WebSocket server for real-time event streaming (v0.7.0: wired to EventBus) |
| `pkg/translator/` | Message format translation between platforms |
| `pkg/matrixcmd/` | Matrix command parser and handler |
| `pkg/notification/` | Cross-channel notification dispatch |

#### Container & Runtime
| Package | Purpose |
|---------|---------|
| `pkg/studio/` | Agent container lifecycle (Docker) |
| `pkg/browser/` | BrowserBroker interface, JetskiBroker implementation, NavChart pipeline (types, normalizer, store, validator, audit, scanner), job processor, context manager, bridge-local handler, legacy HTTP client, status tracking |
| `pkg/queue/` | Job queue for browser tasks |
| `pkg/docker/` | Docker client wrapper with resource governance |
| `internal/agent/` | Agent runtime state machine |
| `internal/executor/` | Task execution engine |
| `pkg/sidecar/` | Go gRPC client for Rust document pipeline sidecar |
| `pkg/setup/` | Initial configuration and Docker setup |

#### Observability & Governance
| Package | Purpose |
|---------|---------|
| `pkg/audit/` | Critical operation audit logging |
| `pkg/budget/` | AI spend budget tracking |
| `pkg/governor/` | Rate limiting and throttling |
| `pkg/metrics/` | Metrics collection |
| `pkg/eventlog/` | Structured event logging and segment management |

#### Real-Time Communication
| Package | Purpose |
|---------|---------|
| `pkg/audio/` | Opus and PCM audio encoding/decoding |
| `pkg/voice/` | Voice call budget tracking and management |
| `pkg/webrtc/` | WebRTC session engine and management |
| `pkg/turn/` | TURN/STUN relay configuration |

> See [doc/voice-stack.md](voice-stack.md) for full documentation.

### Voice STT/TTS/VAD Services

**Package**: `bridge/pkg/voice/`

**Purpose**: Service wrappers for speech-to-text (STT), text-to-speech (TTS), and voice activity detection (VAD) with structured logging.

#### STT Service (Speech-to-Text)

**File**: `bridge/pkg/voice/stt_service.go`

| Component | Description |
|-----------|-------------|
| **STTService** | Wrapper service around Transcriber interface |
| **Transcriber** | Interface for speech-to-text conversion |

**Interface**:
```go
type Transcriber interface {
    Transcribe(ctx context.Context, audioData []byte) (*TranscriptionResult, error)
}
```

**Service Methods**:
- `NewSTTService(client Transcriber)` - Create new STT service with client
- `Transcribe(ctx context.Context, audioData []byte)` - Convert audio to text

**Return Type** (`bridge/pkg/interfaces/voice.go`):
```go
type TranscriptionResult struct {
    Text       string
    Confidence float64
    Duration   time.Duration
    WordCount  int
    Timestamp  time.Time
    Latency    time.Duration
}
```

#### TTS Service (Text-to-Speech)

**File**: `bridge/pkg/voice/tts_service.go`

| Component | Description |
|-----------|-------------|
| **TTSService** | Wrapper service around Synthesizer interface |
| **Synthesizer** | Interface for text-to-speech conversion |

**Interface**:
```go
type Synthesizer interface {
    Synthesize(ctx context.Context, text string) (*SynthesisResult, error)
}
```

**Service Methods**:
- `NewTTSService(client Synthesizer)` - Create new TTS service with client
- `Synthesize(ctx context.Context, text string)` - Convert text to audio

**Return Type** (`bridge/pkg/interfaces/voice.go`):
```go
type SynthesisResult struct {
    AudioData  []byte
    TextLength int
    Duration   time.Duration
    Timestamp  time.Time
    Latency    time.Duration
}
```

#### VAD Service (Voice Activity Detection)

**File**: `bridge/pkg/voice/vad_service.go`

| Component | Description |
|-----------|-------------|
| **VADService** | Wrapper service around SpeechDetector interface |
| **SpeechDetector** | Interface for voice activity detection |

**Interface**:
```go
type SpeechDetector interface {
    DetectSpeech(ctx context.Context, audioData []byte) (*VADResult, error)
}
```

**Service Methods**:
- `NewVADService(client SpeechDetector)` - Create new VAD service with client
- `DetectSpeech(ctx context.Context, audioData []byte)` - Detect speech in audio

**Return Type** (`bridge/pkg/interfaces/voice.go`):
```go
type VADResult struct {
    SpeechDetected bool
    Confidence     float64
    Timestamp      time.Time
    Latency        time.Duration
}
```

#### Common Pattern

All three voice services follow the same architectural pattern:
1. Define an interface for the core functionality (`Transcriber`, `Synthesizer`, `SpeechDetector`)
2. Create a service wrapper with `slog` logging
3. Implement a simple constructor (`NewXxxService`)
4. Delegate to the underlying client with context propagation

**Example Usage**:
```go
// Create STT service
sttClient := &MyTranscriber{} // Implements Transcriber interface
sttService := voice.NewSTTService(sttClient)

// Transcribe audio
result, err := sttService.Transcribe(ctx, audioData)
if err != nil {
    return err
}
fmt.Printf("Transcribed: %s (confidence: %.2f)\n", result.Text, result.Confidence)

// Create TTS service
ttsClient := &MySynthesizer{} // Implements Synthesizer interface
ttsService := voice.NewTTSService(ttsClient)

// Synthesize text
audioResult, err := ttsService.Synthesize(ctx, "Hello, world")
if err != nil {
    return err
}
fmt.Printf("Audio data length: %d bytes\n", len(audioResult.AudioData))

// Create VAD service
vadClient := &MySpeechDetector{} // Implements SpeechDetector interface
vadService := voice.NewVADService(vadClient)

// Detect speech activity
vadResult, err := vadService.DetectSpeech(ctx, audioData)
if err != nil {
    return err
}
if vadResult.SpeechDetected {
    fmt.Printf("Speech detected (confidence: %.2f)\n", vadResult.Confidence)
}
```

#### Integration

| Component | Integration Point |
|-----------|------------------|
| **Voice Manager** | `bridge/pkg/interfaces/voice.go` - `VoiceManager` interface for Matrix call events |
| **WebRTC** | `bridge/pkg/webrtc/` - Session engine for voice calls |
| **Audio Encoding** | `bridge/pkg/audio/` - Opus and PCM encoding/decoding |

#### Identity & Access
| Package | Purpose |
|---------|---------|
| `pkg/license/` | License client and state management |
| `pkg/permissions/` | Permission prediction and access control |
| `pkg/invite/` | Room invitation and role management |
| `pkg/admin/` | Admin claim and privilege management |

> See [doc/license-system.md](license-system.md) for full documentation.

#### v6 Microkernel (feature-flagged, off by default)
| Package | Purpose |
|---------|---------|
| `pkg/mcp/` | MCP Router — routes tool calls through SkillGate, consent, and vault governance |
| `pkg/vault/proto/` | Generated gRPC client for Rust Vault governance (ephemeral tokens, zeroization) |
| `pkg/toolsidecar/` | Tool sidecar provisioning — isolated tool execution containers (v6-gated) |

### Initialization Flow

**CLI Commands:**
```
init              → Generate example config
validate          → Validate configuration
setup             → Interactive setup wizard
daemon            → Daemon management (start/stop/restart/status)
add-key           → Add API key to keystore
generate-qr       → Generate QR for mobile app
(no command)      → Start bridge server
```

**Server Initialization Sequence:**
1. Parse CLI flags and load configuration
2. Setup logging from config
3. Pre-flight Docker availability check
4. Create runtime directory (`/run/armorclaw/`)
5. Initialize encrypted keystore (with recovery for corruption)
6. Initialize audit logger
7. Create Matrix adapter (if enabled)
8. Initialize AI service with keystore
9. Initialize event bus
10. Initialize browser job manager
11. Initialize studio (agent factory)
12. Initialize provisioning manager
13. Initialize skill manager
14. Register RPC handlers
15. Start RPC server (Unix socket or TCP)
16. Start event bus broadcaster
17. Start Matrix sync loop
18. Wait for shutdown signal
19. Graceful shutdown

### Key Interfaces

```go
// Bridge management
type BridgeManager interface {
    Start() error
    Stop() error
    RegisterAdapter(platform, adapter) error
    BridgeChannel(roomID, platform, channelID) error
    UnbridgeChannel(roomID string) error
    GetBridgedChannels() []*BridgedChannel
    GetStats() *BridgeStats
}

// PII interception
type SkillGate interface {
    InterceptToolCall(ctx, call) (*ToolCall, error)
    InterceptPrompt(ctx, prompt) (string, *PIIMapping, error)
    RestoreOutput(ctx, output, mapping) (string, error)
    ValidateArgs(ctx, toolName, args) ([]PIIViolation, error)
}

// Matrix communication
type MatrixAdapter interface {
    SendMessage(roomID, message, msgType) (string, error)
    SendEvent(roomID, eventType, content) error
    Login(username, password) error
    JoinRoom(ctx, roomIDOrAlias, viaServers, reason) (string, error)
    GetUserID() string
    IsLoggedIn() bool
}
```

---

## SQLCipher Keystore

### Purpose

The keystore provides **zero-knowledge encrypted credential storage** using SQLCipher with hardware-bound master keys. It enables:
- Secure API key storage (never persisted to disk as plaintext)
- BlindFill™ secret injection (agents never see raw values)
- Hardware binding (database useless if stolen)
- Zero-touch reboot (no password required)

### Database Schema

**Database Path**: `/var/lib/armorclaw/keystore.db` (encrypted)
**Encryption**: SQLCipher with XChaCha20-Poly1305 AEAD

```sql
-- API Credentials
CREATE TABLE credentials (
    id TEXT PRIMARY KEY,
    provider TEXT NOT NULL,                    -- openai, anthropic, cloudflare, etc.
    token_encrypted BLOB NOT NULL,             -- XChaCha20-Poly1305 encrypted
    nonce BLOB NOT NULL,                       -- AEAD nonce
    base_url TEXT,                             -- Custom endpoint
    display_name TEXT NOT NULL,
    created_at INTEGER NOT NULL,
    expires_at INTEGER,                        -- Token expiry (optional)
    tags TEXT                                  -- JSON array
);

CREATE INDEX idx_provider ON credentials(provider);
CREATE INDEX idx_expires_at ON credentials(expires_at);

-- User Profiles (BlindFill PII)
CREATE TABLE user_profiles (
    id TEXT PRIMARY KEY,
    profile_name TEXT NOT NULL,
    profile_type TEXT NOT NULL DEFAULT 'personal',
    data_encrypted BLOB NOT NULL,              -- JSON-serialized PII (encrypted)
    data_nonce BLOB NOT NULL,
    field_schema TEXT NOT NULL,                -- JSON schema of fields
    created_at INTEGER NOT NULL,
    updated_at INTEGER NOT NULL,
    last_accessed INTEGER,
    is_default INTEGER DEFAULT 0
);

CREATE INDEX idx_profile_type ON user_profiles(profile_type);
CREATE INDEX idx_profile_default ON user_profiles(is_default);

-- Matrix Refresh Tokens
CREATE TABLE matrix_refresh_tokens (
    id TEXT PRIMARY KEY,
    token_encrypted BLOB NOT NULL,
    nonce BLOB NOT NULL,
    homeserver_url TEXT NOT NULL,
    user_id TEXT NOT NULL,
    created_at INTEGER NOT NULL
);

-- Hardening State
CREATE TABLE hardening_state (
    user_id TEXT PRIMARY KEY,
    password_rotated INTEGER DEFAULT 0,
    bootstrap_wiped INTEGER DEFAULT 0,
    device_verified INTEGER DEFAULT 0,
    recovery_backed_up INTEGER DEFAULT 0,
    biometrics_enabled INTEGER DEFAULT 0,
    delegation_ready INTEGER DEFAULT 0,
    created_at INTEGER NOT NULL,
    updated_at INTEGER NOT NULL
);

-- Hardware Binding
CREATE TABLE hardware_binding (
    signature_hash TEXT PRIMARY KEY,
    bound_at INTEGER NOT NULL,
    entropy_sources TEXT NOT NULL             -- JSON of sources used
);

-- Metadata
CREATE TABLE metadata (
    key TEXT PRIMARY KEY,
    value TEXT NOT NULL
);
```

### Key Derivation Hierarchy

**Priority order for master key source:**
1. `ARMORCLAW_KEYSTORE_SECRET` environment variable (base64-encoded 32 bytes)
2. `keystore.db.key` file (base64-encoded)
3. Container-persisted random key
4. Hardware-derived key (fallback)

**Hardware Entropy Sources:**
```go
// CollectEntropy() gathers from:
1. /etc/machine-id, /var/lib/dbus/machine-id
2. /sys/class/dmi/id/product_uuid (SMBIOS)
3. Primary MAC address (first non-loopback)
4. Hostname
5. OS/Architecture (runtime.GOOS, runtime.GOARCH)
6. /proc/cpuinfo (model name, vendor_id)
```

### Encryption Configuration

```go
const (
    saltLength       = 32
    pbkdf2Iterations = 256000  // SQLCipher default
    keyLength        = 32
    cipherPageSize   = 4096
    cipherKdfIter    = 256000
    cipherHmacAlg    = "HMAC_SHA512"
    cipherKdfAlgorithm = "PBKDF2_HMAC_SHA512"
)
```

**Connection String:**
```
file:keystore.db?_pragma_key=x'hex_master_key'&_pragma_cipher_page_size=4096&_pragma_kdf_iter=256000&_pragma_cipher_hmac_algorithm=HMAC_SHA512&_pragma_cipher_kdf_algorithm=PBKDF2_HMAC_SHA512&_foreign_keys=ON
```

### Supported Providers

```go
const (
    ProviderOpenAI     Provider = "openai"
    ProviderAnthropic  Provider = "anthropic"
    ProviderCloudflare Provider = "cloudflare"
    ProviderDeepSeek   Provider = "deepseek"
    ProviderGoogle     Provider = "google"
    ProviderGroq       Provider = "groq"
    ProviderMoonshot   Provider = "moonshot"
    ProviderNvidia     Provider = "nvidia"
    ProviderOllama     Provider = "ollama"
    ProviderOpenRouter Provider = "openrouter"
    ProviderXAI        Provider = "xai"
    ProviderZhipu      Provider = "zhipu"
)
```

### Environment Fallback

`Retrieve()` checks environment variables first:
- `OPENROUTER_API_KEY`
- `ZAI_API_KEY`
- `OPEN_AI_KEY`

---

## Rust Vault Sidecar

### Purpose

The Rust Vault is a **security-hardened cryptographic enclave** that provides heavy I/O operations for ArmorClaw with enhanced security features. It implements:

- **State Bifurcation** - Separate persistent secrets (vault.db) from ephemeral crypto state (matrix_state.db)
- **Network-Layer BlindFill** - Inject secrets at network layer via Chrome DevTools Protocol
- **gRPC Governance** - Ephemeral token lifecycle management with zeroization
- **Zeroization** - All secrets zeroized in memory after use
- **mTLS Authentication** - gRPC over Unix domain sockets with certificate validation

### Runtime Model: Deployed Service (v0.6.0)

> **Updated in v0.6.0**: The Rust Vault is now a **deployed Docker service** with its own binary entrypoint, hardened container, and docker-compose configuration. It communicates with the Go Bridge via Unix domain socket IPC over a shared volume.

**Binary entrypoint**: `rust-vault/src/main.rs` (28 lines) registers the gRPC governance service and starts the Tokio runtime.

**Cargo.toml** `[[bin]]` section: `name = "armorclaw-vault"`

**Docker build** (multi-stage hardened):
- `network_mode: none` at build time for dependency fetch
- Runtime user UID 10001 (non-root)
- `cap_drop: ALL`, `read_only: true`, `no-new-privileges: true`

**docker-compose** service: `armorclaw-vault` shares `/run/armorclaw/` volume with the bridge for Unix socket IPC (`rust-vault.sock`).

This means:
- The Rust Vault **runs as a standalone process** alongside the Bridge in production
- There is **no runtime port conflict** with Jetski (see below) — communication is Unix socket only
- The `blindfill` module was **removed in v0.9.0** (commit 1563260) — superseded by Jetski CDP proxy
- The governance gRPC service (`rust-vault/src/governance/`) is activated when the v6 microkernel flag is enabled

### Relationship to Jetski Browser Sidecar

The Rust Vault's blindfill module (Phase 1 CDP interception) was removed in v0.9.0. Jetski is now the sole CDP security layer.

| Aspect | Rust Vault (historical) | Jetski CDP Proxy |
|--------|--------------------------|-----------------|
| **Type** | Library (removed v0.9.0) | Standalone Go binary (Docker container) |
| **What it did** | Generated `Fetch.enable` params, resolved `{{VAULT:field:hash}}` placeholders | Full WebSocket proxy between agent and Lightpanda engine |
| **Port usage** | None (library, no listener) | Listens on 9222 (CDP), 9223 (RPC) |
| **PII handling** | Placeholder format validation | Active CDP message-level PII scrubbing |
| **Runtime state** | Removed in commit 1563260 | Deployed via `docker-compose.jetski.yml` |

> **Note**: The Go-side BlindFill engine (`bridge/pkg/pii/`) remains active and handles placeholder resolution at the Bridge level. The `{{VAULT:field:hash}}` format is still used for PII injection workflows.

### Architecture

```
┌───────────────────────────────────────────────────────────────────────┐
│                         THE VPS (Office)                              │
│                                                                       │
│  ┌─────────────┐      ┌─────────────┐      ┌─────────────┐           │
│  │ ArmorClaw   │◀────▶│  Rust Vault   │◀────▶│  Playwright │           │
│  │ Bridge      │ gRPC │  (Sidecar)    │ CDP  │  (Browser)  │           │
│  │ (Orchestr.) │ Unix │             │      │             │           │
│  └──────┬──────┘      └──────┬──────┘      └──────┬──────┘           │
│         │                    │                     │                   │
│         │                    │                     │                   │
│         │                    │   BlindFill Engine │                   │
│         │                    │   (Memory-Only)    │                   │
│         │                    │                     │                   │
└─────────┼────────────────────┼─────────────────────┼───────────────────┘
          │                    │                     │
          │                    │                     │
          │ Secure Matrix Tunnel (E2EE)             │
          │                    │                     │
┌─────────▼────────────────────▼─────────────────────▼───────────────────┐
│                         USER (Mobile)                                 │
│   ArmorChat App                                                      │
│   "Book a flight to NYC"  [Approve Credit Card] 🔐                   │
└───────────────────────────────────────────────────────────────────────┘
```

### Integration with ArmorClaw

**Go Bridge → Rust Vault:**
- gRPC over Unix Domain Socket (`/run/armorclaw/rust-vault.sock`)
- mTLS authentication with certificate validation
- Keystore proxy API for secret retrieval
- Rate limiting (100 requests/second) with atomic operations
- Concurrency limiting (10 concurrent requests)

**Rust Vault → Playwright/Browser:**
- Chrome DevTools Protocol (CDP) interception
- Filters XHR and Fetch requests only (not wildcard)
- Placeholder resolution: `{{secret:payment.card_number}}`
- Network-layer injection (secrets never reach agent)

**Security Features:**

1. **State Bifurcation**
   - `vault.db` - Persistent secrets (SQLCipher encrypted)
   - `matrix_state.db` - Ephemeral crypto state (SQLCipher encrypted)
   - Separate databases prevent cross-contamination

2. **Network-Layer BlindFill**
   - CDP interceptor filters by resourceType (XHR, Fetch only)
   - Placeholder format: `{{secret:name}}` (flat lookups only)
   - Secrets injected at network layer, never accessible to agent
   - Zeroized immediately after request completes

3. **gRPC Security**
   - Unix domain socket with 0600 permissions
   - mTLS authentication (certificate validation)
   - Rate limiting: 100 req/s with atomic operations (no mutex)
   - Concurrency limiting: 10 concurrent requests with semaphore

4. **Memory Safety**
   - All secrets use `Zeroizing<String>` from zeroize crate
   - Secrets zeroized on drop
   - No secret caching beyond request lifecycle
   - No secret values in logs

5. **Key Derivation**
   - PBKDF2-HMAC-SHA512 with 256,000 iterations
   - 32-byte salt for each database
   - Compatible with Go Bridge implementation

6. **SQLCipher Configuration**
   - `cipher_plaintext_header_size=32` for performance
   - `synchronous=NORMAL` for durability
   - Separate encryption keys for vault.db and matrix_state.db

7. **Logging**
   - Basic logging only (no comprehensive observability)
   - No secret values in logs
   - No circuit breakers or advanced retry logic

### Configuration

**Environment Variables:**

```bash
# Rust Vault Configuration
RUST_VAULT_ENABLED=true
RUST_VAULT_SOCKET_PATH=/run/armorclaw/rust-vault.sock
RUST_VAULT_TLS_ENABLED=true
RUST_VAULT_TLS_CERT_PATH=/etc/armorclaw/rust-vault.crt
RUST_VAULT_TLS_KEY_PATH=/etc/armorclaw/rust-vault.key
RUST_VAULT_TLS_CA_PATH=/etc/armorclaw/ca.crt

# Rate Limiting
RUST_VAULT_RATE_LIMIT=100              # Requests per second
RUST_VAULT_BURST_SIZE=20               # Burst capacity

# Concurrency
RUST_VAULT_MAX_CONCURRENT=10           # Max concurrent requests

# BlindFill
RUST_VAULT_CDP_ENABLED=true            # Enable CDP interception
```

**Default Configuration:**

```rust
pub struct VaultConfig {
    // Socket Configuration
    pub keystore_socket_path: PathBuf,
    pub use_tls: bool,
    pub tls: Option<TlsConfig>,
    
    // Rate Limiting
    pub rate_limit: u32,           // Default: 100
    pub burst_size: u32,           // Default: 20
    
    // Concurrency
    pub max_concurrent: usize,     // Default: 10
}
```

### API Reference

**gRPC Methods (via Unix Socket):**

```protobuf
service Keystore {
    // Secret Management
    rpc StoreSecret(StoreSecretRequest) returns (StoreSecretResponse);
    rpc RetrieveSecret(RetrieveSecretRequest) returns (RetrieveSecretResponse);
    rpc DeleteSecret(DeleteSecretRequest) returns (DeleteSecretResponse);
    rpc ListSecrets(ListSecretsRequest) returns (ListSecretsResponse);
    
    // Matrix State
    rpc StoreMatrixState(StoreMatrixStateRequest) returns (StoreMatrixStateResponse);
    rpc RetrieveMatrixState(RetrieveMatrixStateRequest) returns (RetrieveMatrixStateResponse);
}
```

**CDP Interception:**

```json
{
  "method": "Fetch.enable",
  "params": {
    "patterns": [
      {
        "urlPattern": "*",
        "resourceType": "XHR",
        "requestStage": "Request"
      },
      {
        "urlPattern": "*",
        "resourceType": "Fetch",
        "requestStage": "Request"
      }
    ]
  }
}
```

### Testing

**Test Coverage: 33 tests (cargo test --lib)**

**Run Tests:**

```bash
cd rust-vault
cargo test --lib
cargo clippy -- -D warnings
```

### Security Considerations

**Guardrails Respected:**

- ✅ No wildcard URL patterns (resourceType filtering instead)
- ✅ No WebSocket interception
- ✅ No document.write() or innerHTML interception
- ✅ No comprehensive observability (basic logging only)
- ✅ No circuit breakers or advanced retry logic
- ✅ No secret caching beyond request lifecycle
- ✅ No secret values in logs
- ✅ No advanced placeholder features (conditionals, loops, nesting)

**Production Checklist:**

- [ ] Generate TLS certificates for mTLS
- [ ] Set Unix socket permissions to 0600
- [ ] Configure SQLCipher encryption keys
- [ ] Enable rate limiting and concurrency limits
- [ ] Test CDP interception with real browser
- [ ] Verify zeroization in memory dumps
- [ ] Audit logs for secret exposure

### BlindFill Placeholder System

The Rust Vault enforces **strict placeholder masking** to prevent agents from ever seeing real secret values. This is a critical security feature for defending against prompt injection attacks.

#### Placeholder Format

**Strict Format**: `{{VAULT:field:hash}}`

- **VAULT:** - Required prefix (case-sensitive)
- **field** - Secret identifier (e.g., `payment.card_number`, `user.email`)
- **hash** - Lowercase hexadecimal hash (e.g., `a1b2c3d4e5f6`)

**Examples**:
```
{{VAULT:payment.card_number:a1b2c3d4e5f6}}
{{VAULT:user.email:f7e8d9c0b1a2}}
{{VAULT:api.stripe_key:3d4e5f6a7b8c}}
```

#### Security Guarantees

1. **Real Values Never Exposed**:
   - Agents only see placeholders, never actual secrets
   - Real values injected at network layer via CDP
   - Secrets zeroized immediately after injection

2. **Strict Validation**:
   - Case-sensitive `VAULT:` prefix required
   - Lowercase hexadecimal hash only
   - No whitespace, nested placeholders, or conditionals
   - Old formats (e.g., `{{secret:...}}`) explicitly rejected

3. **Prompt Injection Defense**:
   - Placeholder format prevents adversarial prompts from accessing secrets
   - No support for conditionals (`if/else/endif`)
   - No support for loops (`for/endfor`)
   - No support for nested placeholders

4. **Placeholder Resolution Flow**:
   ```
   Agent Request → Placeholder → CDP Interceptor → Real Value → Browser Form
                  (agent sees)    (network layer)  (injected)    (filled)
   ```

#### Implementation Details

**Go BlindFill Engine** (`bridge/pkg/pii/`):
- Validates strict `{{VAULT:field:hash}}` format
- Rejects malformed placeholders with clear error messages
- Prevents injection attacks via field/hash manipulation
- Resolves placeholders to real values from SQLCipher keystore
- Values injected at browser form level via Jetski CDP proxy

#### Use Cases

1. **Payment Processing**:
   - Agent requests: `{{VAULT:payment.card_number:abc123}}`
   - Browser receives: `4242 4242 4242 4242`
   - Agent never sees: Real card number

2. **Form Filling**:
   - Agent requests: `{{VAULT:user.email:def456}}`
   - Browser receives: `user@example.com`
   - Agent never sees: Real email address

3. **API Authentication**:
   - Agent requests: `{{VAULT:api.stripe_key:ghi789}}`
   - Browser receives: `sk_live_...`
   - Agent never sees: Real API key

#### Error Handling

**Invalid Placeholder Examples**:
```
{{secret:payment.card}}          ❌ Wrong prefix (must be VAULT:)
{{VAULT:payment.card:ABC123}}    ❌ Uppercase hash (must be lowercase)
{{VAULT:payment.card:abc}}       ❌ Invalid hash length
{{ VAULT:payment.card:abc123 }}  ❌ Whitespace not allowed
{{VAULT:{{nested}}:abc123}}      ❌ Nested placeholders not allowed
{{VAULT:payment.card:abc123}}    ✅ Valid
```

### Performance Characteristics

- **Memory**: ~2MB bounded for download streams
- **Rate Limiting**: 100 req/s with atomic operations
- **Concurrency**: 10 concurrent requests with semaphore
- **Key Derivation**: 256,000 iterations (compatible with Go Bridge)
- **Zeroization**: Immediate on drop, no caching
- **Socket**: Unix domain socket (0600 permissions)

### Troubleshooting

**Common Issues:**

1. **Socket Permission Denied**
   ```bash
   ls -la /run/armorclaw/rust-vault.sock
   # Should show: srw------- 1 root root 0 ... rust-vault.sock
   chmod 0600 /run/armorclaw/rust-vault.sock
   ```

2. **mTLS Authentication Failed**
   ```bash
   # Verify certificates exist
   ls -la /etc/armorclaw/rust-vault.{crt,key} /etc/armorclaw/ca.crt
   
   # Check certificate expiry
   openssl x509 -in /etc/armorclaw/rust-vault.crt -text -noout | grep "Not After"
   ```

3. **SQLCipher Key Derivation Mismatch**
   ```bash
   # Ensure PBKDF2-HMAC-SHA512 with 256,000 iterations
   # Check Go Bridge compatibility
   grep -r "PBKDF2" bridge/pkg/keystore/
   ```

4. **CDP Interception Not Working**
   ```bash
   # Verify CDP is enabled
   curl http://localhost:9222/json/list
   
    # Check resourceType filtering
    # Should only intercept XHR and Fetch requests
    ```

---

## Phase 2: Secure Document Pipeline

### Purpose

Phase 2 added a **secure document processing pipeline** to ArmorClaw, providing enterprise-grade document handling with security controls at every stage. It implements:

- **Split-Storage RAG** - Documents are split into chunks; embeddings stored separately from content in Qdrant
- **YARA Content Disarm & Reconstruct (CDR)** - Malicious content detected and neutralized before processing
- **TTL Proxy Guard** - Ephemeral authentication tokens (30-minute TTL) for sidecar communication

### Architecture

```
┌─────────────┐     ┌──────────────┐     ┌─────────────┐
│  Document   │────▶│  YARA CDR   │────▶│   Split     │
│  Ingestion  │     │  Scanner     │     │   Storage   │
└─────────────┘     └──────────────┘     └──────┬──────┘
                                               │
                                    ┌──────────┴──────────┐
                                    ▼                     ▼
                             ┌─────────────┐      ┌─────────────┐
                             │  Qdrant     │      │  Content    │
                             │  Embeddings │      │  Store      │
                             │  (vectors)  │      │  (chunks)   │
                             └─────────────┘      └─────────────┘
```

### Components

#### Split-Storage RAG

| Feature | Description |
|---------|-------------|
| **Chunking** | Documents split into semantically meaningful chunks |
| **Embedding Separation** | Vector embeddings stored in Qdrant, content stored separately |
| **Retrieval** | Embedding similarity search retrieves relevant chunks |
| **Provider** | Uses OpenAI embeddings (no ONNX migration) |

#### YARA Content Disarm & Reconstruct

| Feature | Description |
|---------|-------------|
| **Rule Matching** | YARA rules scan incoming documents for malicious patterns |
| **Disarm** | Detected threats neutralized before processing |
| **Reconstruct** | Safe content reconstructed for downstream use |
| **Integration** | Requires CGO + libyara-dev |

#### TTL Proxy Guard

| Feature | Description |
|---------|-------------|
| **Token TTL** | 30-minute ephemeral tokens |
| **Validation** | HMAC-SHA256 signatures |
| **Replay Prevention** | Timestamp validation (5-minute max age) |
| **Scope** | Sidecar communication only |

### Security Guarantees

- ✅ No persistent credentials in sidecar memory
- ✅ No credential caching beyond request lifecycle
- ✅ All document processing logged in Go Bridge audit trail
- ✅ PII interception before sidecar calls
- ✅ YARA rules validated before deployment
- ✅ TTL tokens cannot be reused after expiry

### Backlog Items

| Item | Priority | Description |
|------|----------|-------------|
| PH2.1 | Medium | qdrant-client-rs v1.7 builder pattern migration |
| PH2.2 | High | pdf.rs `.unwrap()` panic fix |
| PH2.3 | Low | lopdf text extraction gap |

---

## Sovereign Email Pipeline

### Purpose

The sovereign email pipeline lets ArmorClaw receive and process inbound email without relying on third-party services. Postfix handles SMTP delivery, and a Go binary (`mta-recv`) pipes messages into the Bridge.

### Data Flow

```
Postfix (port 25) -> pipe transport -> mta-recv -> email-ingest.sock -> IngestServer -> YARA -> PII scan -> EmailDispatcher -> Secretary workflow
```

### Components

| Component | Language | File | Purpose |
|-----------|----------|------|---------|
| **Postfix** | Config | `deploy/postfix/main.cf`, `deploy/postfix/master.cf` | SMTP listener, pipe transport to armorclaw |
| **mta-recv** | Go | `bridge/cmd/mta-recv/` | Reads stdin from Postfix pipe, writes to Unix socket |
| **IngestServer** | Go | `bridge/pkg/email/` | Reads from `email-ingest.sock`, runs YARA + PII scan |
| **EmailDispatcher** | Go | `bridge/pkg/email/` | Routes sanitized email to Secretary workflow |
| **Bridge wiring** | Go | `bridge/cmd/bridge/main.go` | Starts IngestServer + EmailDispatcher at startup |

### Setup

```bash
# Install Postfix with ArmorClaw pipe transport
sudo bash deploy/postfix/install.sh

# Verify configuration
sudo bash deploy/postfix/verify-setup.sh
```

The installer is idempotent and safe to re-run. It configures Postfix `master.cf` with the `armorclaw` pipe transport and sets up `transport_maps` for email routing.

### RPC Methods

Email approval is exposed through 4 RPC methods (see [Email Approval](#email-approval-4-methods) in the RPC API Reference):

| Method | Purpose |
|--------|---------|
| `approve_email` | Approve a pending email |
| `deny_email` | Deny a pending email |
| `email_approval_status` | Check approval status |
| `email.list_pending` | List pending approvals |

### Testing

The email pipeline is covered by `tests/test-email-pipeline.sh` (Tier A, 7 scenarios: M0-M6) and `tests/test-cross-workflow-email.sh` (Secretary to Email Approval cross-subsystem).

---

## Matrix Conduit Control Plane

### Purpose

Matrix serves as the **primary control plane** for ArmorClaw, providing:
- End-to-end encrypted messaging
- Real-time event streaming
- Admin command processing
- Agent control commands
- Voice call signaling

### Event Types

**Standard Matrix Events:**
- `m.room.message` - Text messages and commands
- `m.room.member` - Membership changes
- `m.room.power_levels` - RBAC (admin=50)
- `m.typing` - Typing notifications
- `m.receipt` - Read receipts

**Voice Call Events:**
- `m.call.invite` - Call initiation
- `m.call.answer` - Call acceptance
- `m.call.hangup` - Call termination
- `m.call.candidates` - ICE candidates
- `m.call.negotiate` - SDP renegotiation

**Custom ArmorClaw Events:**
- `app.armorclaw.alert` - System alerts
- `app.armorclaw.pii_request` - PII access request
- `app.armorclaw.pii_response` - PII access response
- `app.armorclaw.consent.request` - Three-way consent request
- `app.armorclaw.consent.response` - Three-way consent response
- ~~`app.armorclaw.task_dispatch`~~ - **Removed v0.7.0** (warm dispatch deleted; see deprecation notes)
- `app.armorclaw.workflow_step_progress` - Step execution progress during workflow run
- `app.armorclaw.workflow_completed` - Workflow completed successfully (all steps done)
- `app.armorclaw.workflow_failed` - Workflow failed with error (includes step ID and recoverability)

### Control Plane Commands

#### Admin Commands (`/` prefix)

| Command | Description |
|---------|-------------|
| `/claim_admin [device_name]` | Claim admin rights (lockdown mode only) |
| `/status` | Show system status |
| `/verify <code>` | Verify challenge code |
| `/approve <claim_id>` | Approve admin claim |
| `/reject <claim_id> [reason]` | Reject claim |
| `/help` | Show available commands |

#### AI Management Commands (`/ai` prefix)

| Command | Description |
|---------|-------------|
| `/ai providers` | List available AI providers |
| `/ai models <provider>` | List models for provider |
| `/ai switch <provider> <model>` | Switch runtime provider/model |
| `/ai status` | Show current AI configuration |

#### Agent Studio Commands (`!agent` prefix)

| Command | Description |
|---------|-------------|
| `!agent help` | Show help |
| `!agent list-skills` | List available skills |
| `!agent create name=... skills=...` | Create new agent |
| `!agent list` | List all agents |
| `!agent start <agent_id>` | Start agent |
| `!agent stop <agent_id>` | Stop agent |
| `!agent remove <agent_id>` | Remove agent |

---

## Security Architecture

### BlindFill™ Secret Injection

**Core Principle**: Agents request PII by reference name, never see actual values. Secrets are injected directly into browser/containers via memory-only methods.

**Flow Architecture:**
```
┌─────────────┐     ┌──────────────┐     ┌─────────────┐
│   Agent     │────▶│ Approval     │────▶│   PII       │
│ Requests    │     │ Engine       │     │ Injector    │
│ "card_num"  │     │ Evaluate     │     │ (Socket)    │
└─────────────┘     │ Policy       │     └─────────────┘
                   │ Returns:     │            │
                   │ ["card_num"] │            │
                   └──────────────┘            │
                                                 ▼
                                          ┌─────────────┐
                                          │   Browser/  │
                                          │ Container   │
                                          │ Receives    │
                                          │ 4242...     │
                                          │ (not agent) │
                                          └─────────────┘
```

**Injection Methods:**

1. **Unix Domain Socket** (Primary, memory-only):
   - Path: `/run/armorclaw/pii/{container}.pii.sock`
   - Permissions: 0600 (owner only)
   - TTL: 5 seconds
   - Socket deleted after delivery

2. **Environment Variables** (Fallback):
   - Prefix: `PII_`
   - Format: `PII_{field_name}={value}`
   - Warning: May be visible in process listings

### PII Approval Workflow

**States:**
- `pending` — Awaiting user approval (default: 5 min TTL)
- `approved` — User approved specific fields
- `denied` — User denied request
- `expired` — Request timed out
- `cancelled` — Agent cancelled request
- `fulfilled` — Approved data delivered

**PII Request Structure:**
```go
type PIIRequest struct {
    ID              string
    AgentID         string
    SkillID         string
    ProfileID       string
    RequestedFields []PIIFieldRequest
    Context         string              // Reason shown to user
    RoomID          string              // Matrix room for events
    Status          PIIRequestStatus
    CreatedAt       time.Time
    ExpiresAt       time.Time           // Default: +5 min
    ApprovedFields  []string
    ApprovedBy      string
    DeniedReason    string
}

type PIIFieldRequest struct {
    Key         string
    DisplayName string
    Required    bool
    Sensitive   bool
}
```

**Approval Engine Decision Types:**
- `DecisionAllow` — Auto-approve
- `DecisionDeny` — Block
- `DecisionRequireApproval` — Ask user

### Hardening State Management

**Mandatory Steps** (all must be true for `delegation_ready`):
```go
type UserHardeningState struct {
    UserID           string
    PasswordRotated  bool   // Changed initial password
    BootstrapWiped   bool   // Cleaned temp files
    DeviceVerified   bool   // Device is trusted
    RecoveryBackedUp bool   // Recovery keys backed up
    BiometricsEnabled bool   // Optional
    DelegationReady  bool   // Computed: all mandatory steps complete
}
```

### Audit Logging

**Three-Tier Audit System:**

#### Tier 1: Basic Audit
```go
type Entry struct {
    Timestamp   time.Time
    EventType   EventType
    SessionID   string
    RoomID      string
    UserID      string
    Details     interface{}
}
```

#### Tier 2: Compliance Audit
```go
type ComplianceEntry struct {
    ID           string
    Timestamp    time.Time
    EventType    EventType
    UserID       string
    Source       string          // Component
    IPAddress    string
    UserAgent    string
    Action       string          // create, read, update, delete
    Resource     string
    Status       string          // success, failure, denied
    PreviousHash string          // Hash chain
    EntryHash    string
}
```

**Compliance Levels:**
- `standard` — 30-day retention
- `extended` — 90-day retention
- `full` — 1-year retention
- `hipaa` — 6-year retention

#### Tier 3: Tamper-Evident Audit
```go
type TamperEvidentEntry struct {
    Sequence     int64
    Timestamp    time.Time
    EventType    string
    Actor        Actor
    Action       string
    Resource     Resource
    Hash         string
    PreviousHash string
    Signature    string          // Optional: high-security mode
    Compliance   ComplianceFlags
}
```

### Zero-Trust Device Verification

**Trust Score Calculation:**
- Base score from verification count, device status, IP history
- Anomalies add: +30 (new device), +20 (unverified), +15 (unknown IP), +25 (>3 failures)

**Device States:**
```go
const (
    StateUnverified        = "unverified"
    StatePendingApproval   = "pending_approval"
    StateAwaitingSecondFactor = "awaiting_second_factor"
    StateVerified          = "verified"
    StateRejected          = "rejected"
    StateExpired           = "expired"
)
```

**Verification Methods:**
- `admin_approval` — Admin must manually approve
- `second_factor` — Existing device confirms
- `wait_period` — Auto-approve after delay
- `automatic` — Not recommended

### Prompt Injection Detection

ArmorClaw includes **real-time prompt injection detection** to defend against adversarial attacks like those pioneered by "Pliny the Prompter". The system detects non-linguistic noise patterns and flags suspicious sessions for human intervention.

#### Detection Patterns

| Pattern | Detection Method | Examples |
|---------|-----------------|----------|
| **Unicode Tricks** | Zero-width chars, combining diacritics, homoglyphs | `H̵̭̓ ELLO`, `\u200B`, Cyrillic lookalikes |
| **Random Characters** | Shannon entropy >3.4 bits + >50% non-alphanumeric | `asdf1234!@#$`, `xk29!@#mz84` |
| **Repetition** | 8+ consecutive chars, repeated sequences | `aaaaaaaa`, `testtesttesttest` |

#### Implementation

**Location**: `container/openclaw-src/src/gateway/injection-detection.ts`

```typescript
interface DetectionResult {
  isSuspicious: boolean;
  reasons: DetectionReason[];  // "unicode_tricks" | "random_chars" | "repetition"
}

function detectPromptInjection(text: string): DetectionResult;
```

#### Integration Points

- **Rate Limiting**: Integrated with `control-plane-rate-limit.ts`
- **Security Logging**: Flagged sessions logged with reason codes
- **Sentinel Mode**: Hook point available for human intervention alerts

#### Performance

- **Latency**: <1ms per detection
- **Complexity**: O(n) where n = message length
- **False Positives**: Tested against 5 legitimate message patterns

### USB Security Validation Suite

ArmorClaw includes a **security validation suite** for testing critical security controls via TAP-formatted output for CI/CD integration.

**Location**: `tools/skills/armorchat_usb_validate.sh`

#### Test Cases

| Test | Purpose | Validates |
|------|---------|-----------|
| `shadowmap_gatekeeper_blocks_api_key` | API keys blocked by gatekeeper | ShadowMap regex patterns |
| `vault_hold_to_reveal_requires_2s_and_biometric` | Timing and biometric enforcement | Vault security requirements |

#### Usage

```bash
# Run security validation suite
bash tools/skills/armorchat_usb_validate.sh --suite security

# Expected output (TAP format)
TAP version 13
1..2
ok 1 - shadowmap_gatekeeper_blocks_api_key - API keys are blocked by gatekeeper
ok 2 - vault_hold_to_reveal_requires_2s_and_biometric - Timing and biometric requirements enforced
```

#### CI Integration

- Exit code 0 = all tests pass
- TAP format compatible with most CI systems
- Can be extended with additional security tests

### Container Terminate RPC (Kill-on-Violation)

ArmorClaw provides a **kill-on-violation capability** via the `TerminateContainer` RPC method, allowing immediate termination of compromised or misbehaving agent containers.

**Location**: `bridge/pkg/rpc/container_handlers.go`

#### Method Signature

```go
// TerminateContainer immediately stops a running container
func (h *Handlers) handleTerminateContainer(req jsonrpc.Request) jsonrpc.Response {
    // Parameters:
    // - container_id: string (required) - Docker container ID
    // - user_id: string (required) - Requesting user for authorization
    //
    // Returns:
    // - success: bool - Whether termination succeeded
    // - error: string - Error message if failed
}
```

#### Security Checks

1. **Authentication**: Requires valid `user_id` parameter
2. **Container Ownership**: Verifies container has ArmorClaw labels
3. **Docker API**: Calls `ContainerKill()` with SIGKILL for immediate termination

#### Usage

```bash
# Via JSON-RPC
curl -X POST http://localhost:8443/rpc -d '{
  "jsonrpc": "2.0",
  "id": 1,
  "method": "terminateContainer",
  "params": {
    "container_id": "abc123...",
    "user_id": "user@matrix.org"
  }
}'
```

#### Integration with Sentinel Mode

- Can be triggered automatically on security violation detection
- Integrates with prompt injection detection for automated response
- Audit logged via `EventSecurityViolation` event type

### Governor-Shield PII Interception

**Package**: `bridge/pkg/governor/`

**Purpose**: DefaultSkillGate layer for PII interception in AI tool calls and prompts, preventing sensitive data from reaching external AI models.

**Core Components**:

| Component | Description |
|-----------|-------------|
| **Governor** | Main PII interceptor with config, scrubber, and mapping management |
| **Config** | Configuration for logging, scrubbing behavior, Shadow Mapping, and performance |
| **PIIMapping** | Placeholder to original value mapping for restoration |

#### Governor Configuration

| Field | Type | Default | Description |
|-------|------|----------|-------------|
| `LogViolations` | bool | true | Log PII violations to audit trail |
| `LogMaskedPII` | bool | true | Include masked snippets in logs (safe for audit) |
| `StrictMode` | bool | false | Block all tool calls with any PII detected |
| `UseShadowMapping` | bool | true | Use Shadow Mapping (SHA256 hash-based placeholders) |
| `PlaceholderPrefix` | string | "[REDACTED:" | Prefix for placeholders |
| `MaxConcurrentCalls` | int | 100 | Maximum concurrent tool calls |
| `CacheMappings` | bool | true | Cache PII mappings for performance |

#### Core Methods

| Method | Purpose |
|--------|---------|
| `InterceptToolCall()` | Scrubs PII from tool call arguments using Shadow Mapping |
| `InterceptPrompt()` | Scans and redacts PII from user prompts before AI model |
| `RestoreOutput()` | Restores redacted PII placeholders in AI output using PIIMapping |
| `ValidateArgs()` | Validates tool call arguments for PII violations without modifying |

#### Shadow Mapping Implementation

**Process**:
1. Detect PII patterns using `bridge/pkg/pii` scrubber
2. Compute SHA256 hash of detected PII value
3. Replace with placeholder: `[REDACTED:{8-char-hash}]`
4. Store mapping: placeholder → original value
5. Restore in output before returning to user

**Benefits**:
- AI never sees raw PII values
- Reversible for legitimate use cases
- Audit trail with masked snippets
- Pattern-aware severity classification

#### Severity Classification

| Severity | Patterns | Examples |
|----------|-----------|-----------|
| **critical** | credit_card, aws_secret, aws_key_id, api_key (sk/pk/ai) | Payment cards, AWS credentials, API keys |
| **high** | ssn, github_token | Social Security Numbers, GitHub tokens |
| **medium** | email, phone, ip_address, bearer_token, token, secret, password | Contact info, auth tokens |
| **low** | All other patterns | Default classification |

#### Integration Points

| Component | Integration Point |
|-----------|------------------|
| **MCP Router** | `bridge/pkg/mcp/router.go` - Tool call PII gate |
| **PII Scrubber** | `bridge/pkg/pii/` - Detection patterns and redaction logic |

#### Usage Example

```go
// Initialize Governor with config
governor := NewGovernor(&Config{
    LogViolations:      true,
    UseShadowMapping:   true,
    PlaceholderPrefix:  "[REDACTED:",
    MaxConcurrentCalls: 100,
    CacheMappings:      true,
}, logger)

// Intercept tool call
scrubbedCall, err := governor.InterceptToolCall(ctx, &ToolCall{
    ToolName: "search_web",
    Arguments: map[string]interface{}{
        "query": "Call 555-123-4567 for John Doe",
    },
})

// scrubbedCall.Arguments["query"] = "Call [REDACTED:a1b2c3d4] for [REDACTED:e5f6g7h8]"
```

#### Audit Logging

Governor logs all PII violations with:
- Tool name and argument key
- Masked snippet (first 2 + *** + last 2 chars)
- Pattern types detected
- Severity classification

**Example Log Entry**:
```
WARN PII violation detected in tool_call tool=search_web key=query violations=2 masked_snippet=55********67 pattern_types=[phone, name]
```

---

## Component Integration Patterns

### Bridge ↔ Matrix Conduit

**Communication Pattern**: HTTP-based Matrix Client API with long-poll sync

**Key Components:**
- **Matrix Client** (`bridge/pkg/matrix/client.go`): Login, incremental sync, message sending
- **Authentication** (`bridge/pkg/auth/matrix_auth.go`): Token-based auth with power level RBAC
- **Command Handlers** (`bridge/pkg/matrixcmd/handler.go`): Regex-based command parsing

**Data Flow:**
```
Bridge → Conduit: POST /_matrix/client/v3/login, PUT /rooms/{id}/send
Bridge ← Conduit: GET /_matrix/client/v3/sync?filter={}&since={token}
```

### Bridge ↔ Browser Service

**Communication Pattern**: Event-based job queue with status emissions

**Key Components:**
- **Browser Queue** (`bridge/pkg/queue/browser_queue.go`): Priority queue with workers
- **Browser Skill** (`bridge/pkg/browser/browser.go`): Status tracking interface
- **Studio Protocol** (`bridge/pkg/studio/browser_skill.go`): Event namespace `com.armorclaw.browser.*`

**Job States**: PENDING → RUNNING → COMPLETED/FAILED/CANCELLED/AWAITING_PII

### Bridge ↔ OpenClaw Agents

**Communication Pattern**: Env-var injection + exit-code polling + `result.json` backward channel

The Bridge communicates with agent containers via **environment variables only**. There is no Matrix connection inside the container, no HTTP callback, no Unix socket, and no stdout capture. The container runs with `NetworkMode: "none"` (factory.go:121) — it has zero network access.

```
Bridge (Go)                           Container (Docker)
┌─────────────────────┐              ┌──────────────────────────┐
│ AgentFactory        │              │ armorclaw/agent-base     │
│   │                 │  Spawn()     │                          │
│   ├─factory.Spawn()──────────────▶│  Reads env vars:         │
│   │                 │ env vars     │   TASK_DESCRIPTION       │
│   │                 │ NetworkMode: │   STEP_CONFIG (JSON)     │
│   │                 │   "none"     │   PII_xxx values         │
│   │                 │              │   ENABLED_SKILLS         │
│   │                 │              │                          │
│   │  waitForComp()  │              │  STEP_CONFIG present?    │
│   │  polls every    │              │    → Step mode: execute, │
│   │  500ms via      │              │      write result.json,  │
│   │  ContainerInspect              │      exit                │
│   │  checks exit ◀─│──────────────│    → Agent mode: Matrix  │
│   │   code          │              │      polling loop        │
│   │                 │              │                          │
│   │  ParseContainer │  result.json │  Exit code:              │
│   │  StepResult() ◀─│◀────────────│    0 = completed         │
│   │  (state dir)    │              │    !0 = failed           │
│   └─StepResult      │              │                          │
│     + ContainerResult│              │  result.json (step mode) │
└─────────────────────┘              └──────────────────────────┘
```

**Key Components:**
- **AgentFactory** (`bridge/pkg/studio/factory.go`): `Spawn()` creates container with env vars, returns immediately. `GetStatus()` polls Docker `ContainerInspect` to check if container exited.
- **StepExecutor** (`bridge/pkg/secretary/orchestrator_integration.go`): `executeStep()` calls `factory.Spawn()` then `waitForCompletion()` which polls `GetStatus()` every 500ms until `StatusCompleted` or `StatusFailed`.
- **Agent Integration** (`bridge/pkg/agent/integration.go`): StateMachine + HITLConsentManager (used by the agent runtime for browser-based flows, not workflow steps).

**Agent States**: OFFLINE, IDLE, BROWSING, FORM_FILLING, AWAITING_APPROVAL, AWAITING_CAPTCHA, AWAITING_2FA, PROCESSING_PAYMENT, COMPLETE, ERROR

**Data flow**: In step mode, the container writes structured results to `result.json` in the state dir before exit. The Bridge reads this via `ParseContainerStepResult()`. In agent mode (no STEP_CONFIG), the agent output is not captured — only the exit code is observed.

> ⚠️ **Mode A Communication**: Agent containers spawned by the studio factory run with `NetworkMode: "none"`. They receive task configuration via environment variables (`STEP_CONFIG`) and report results via exit code + `result.json` (step mode) or exit code only (agent mode). See [Agent Communication Model](#agent-communication-model) below.

**State directory**: Each agent gets a bind-mounted directory at `/var/lib/armorclaw/agent-state/{id}` mapped to `/home/claw/.openclaw` inside the container. In step mode, the container writes `result.json` here before exit. The Bridge reads it via `ParseContainerStepResult()` after container exit.

**Task Scheduler**: The secretary includes a persistent task scheduler with a 15-second tick interval. It is a stateless dispatcher that reads due tasks from `rolodex.db`, dispatches them, and updates `next_run`. All dispatch is cold-only (ephemeral container spawn). Warm dispatch was removed in v0.7.0 because containers cannot receive inbound Matrix events under `NetworkMode: none`. Uses `robfig/cron/v3` for cron expression parsing.

### Workflow Execution Lifecycle

The secretary workflow engine turns task templates into multi-step workflows, executing each step as an isolated agent container.

**Source**: `bridge/pkg/secretary/orchestrator_integration.go`

**Lifecycle flow**:

```
Template → Workflow Creation → StartWorkflow (status=Running)
  → StartWorkflowExecution (goroutine)
    → StepExecutor.ExecuteSteps()
      → For each step: DependencyValidator → ApprovalChecker (if PII) → factory.Spawn(agent)
      → waitForCompletion (500ms polling)
      → On complete: AdvanceWorkflow
      → On fail: FailWorkflow
    → On all steps complete: CompleteWorkflow
```

**Two dispatch paths for scheduled tasks** (routing in `task_scheduler.go:dispatchTask`):

| Condition | Path | What happens |
|-----------|------|--------------|
| `TemplateID != ""` | Workflow engine | Creates a `Workflow` from the `TaskTemplate`, calls `StartWorkflow` then `StartWorkflowExecution`. Steps execute sequentially through `StepExecutor`. Inter-step data passing via `WorkflowStep.Input` template variables (v0.7.0). |
| `TemplateID == ""` | Cold dispatch only | Spawns a new container via `factory.Spawn()`. No warm dispatch (removed v0.7.0). |

**Room ID semantics**:

- Workflows store the Matrix room ID of the triggering context in the `room_id` column.
- Scheduler-triggered workflows (from `templateDispatch`) set `room_id=""` because there is no user in the loop. These are fire-and-forget.
- User-triggered workflows (from `!secretary start workflow`) persist the room ID from the triggering Matrix room. Workflow-spawned agents use this stored `room_id` for Matrix communication.
- When `StepExecutor.executeStep` spawns an agent, it passes `workflow.RoomID` as the `RoomID` in the `SpawnRequest`, so the agent can respond in the original room.

**Key components**:

| Component | File | Purpose |
|-----------|------|---------|
| `StepExecutor` | `orchestrator_integration.go` | Executes workflow steps sequentially, spawns agents, waits for completion |
| `OrchestratorIntegration` | `orchestrator_integration.go` | Manages goroutine lifecycle per workflow, cancellation, status tracking |
| `DependencyValidator` | `orchestrator.go` | Validates step ordering and resolves execution order |
| `NotificationService` | `notifications.go` | Emits `workflow.started`, `workflow.progress`, `workflow.completed`, `workflow.failed`, `workflow.cancelled` notifications |
| `ApprovalChecker` | `orchestrator_integration.go` | Evaluates PII approval requirements per step before execution |

**Error handling**:

- Recoverable errors (agent spawn timeout, transient failure) retry up to `StepRetryCount` times (default: 1) with `StepRetryDelay` (default: 1s).
- Unrecoverable errors (no agent for step, invalid execution order) fail the workflow immediately.
- Context cancellation triggers `CancelWorkflow` and stops all running steps via `CancelAllForWorkflow`.

### Multi-Agent Execution (v0.6.0)

Parallel execution and step failover are now **implemented** via `orchestrator_parallel.go`:

- **Parallel execution**: Uses `errgroup` goroutine pool with configurable `MaxParallelContainers` (default: 2). Steps with multiple `AgentIDs` are dispatched in parallel when the workflow declares them as parallel-safe.
- **Step failover**: When an agent fails mid-step, the executor falls back to the next agent in `AgentIDs[1:]` and retries from the last checkpoint.
- **Single-container scope**: All agents run on the same host. No distributed execution across nodes yet.

> See [doc/secretary-workflow.md](secretary-workflow.md) for the full workflow engine deep dive: two dispatch paths, step execution lifecycle, PII approval flow, and event system.

### Bridge ↔ ArmorChat Mobile

**Communication Pattern**: QR code deep link + Matrix messaging + RPC with bearer tokens

**Key Components:**
- **Provisioning Manager** (`bridge/pkg/provisioning/manager.go`): QR token lifecycle with HMAC-SHA256 signatures
- **Token Structure**: `armorclaw://config?d={base64(json)}` with signature
- **Role Persistence**: Roles saved to `provisioning_roles.json`

**Admin Levels**: NONE, MODERATOR, ADMIN, OWNER

### Event Bus Patterns

**Communication Pattern**: Pub/sub with WebSocket push (v0.7.0)

The EventBus (`bridge/pkg/eventbus/eventbus.go`) initializes a WebSocket connection during startup. All events published to the `MatrixEventBus` ring buffer are pushed to connected WebSocket clients in real time. This enables live status updates in ArmorChat and the admin panel without polling.

**Crash-only design**: EventBus wiring uses `log.Fatalf` when WebSocket initialization fails. The Bridge must crash rather than run in a degraded state where events are silently lost. Do not add graceful fallback or retry logic to this path without CTO approval.

**Handler registration**: Bridge-side event handlers are registered via `RegisterBridgeHandler()` (see `doc/communication-infra.md` for full details). This mechanism provides type-safe handler registration with panic recovery — if a handler panics, the EventBus captures a snapshot of registered handlers and continues, preventing cascading failures.

**Event Types:**
- **Matrix**: `matrix.message`, `matrix.receipt`, `matrix.typing`, `matrix.presence`
- **Agent**: `agent.started`, `agent.stopped`, `agent.status_changed`, `agent.command`, `agent.error`
- **Workflow**: `workflow.started`, `workflow.progress`, `workflow.completed`, `workflow.failed`, `workflow.cancelled`
- **HITL**: `hitl.pending`, `hitl.approved`, `hitl.rejected`, `hitl.expired`, `hitl.escalated`
- **Budget**: `budget.alert`, `budget.limit`, `budget.updated`
- **Platform**: `platform.connected`, `platform.disconnected`, `platform.message`, `platform.error`

### Bridge ↔ Rust Vault

**Communication Pattern**: gRPC over Unix domain socket (when v6 microkernel enabled)

**Key Components:**
- **Vault Governance Client** (`bridge/pkg/vault/proto/governance_grpc.pb.go`): Generated gRPC stubs
- **Governance Service** (`rust-vault/src/governance/`): Ephemeral token lifecycle, event streaming
- **MCP Router** (`bridge/pkg/mcp/router.go`): Consumes vault client for token issuance/zeroization

**Data Flow (v6 microkernel only):**
```
Bridge → Rust Vault: IssueEphemeralToken (grant scoped secret access)
Bridge → Rust Vault: ConsumeEphemeralToken (one-time use, then invalidated)
Bridge → Rust Vault: ZeroizeToolSecrets (secure memory erasure)
Rust Vault → Bridge: SubscribeEvents (gRPC stream for governance events)
```

**Important**: This integration is **inactive by default** (`v6_microkernel = false`). When disabled, the MCP Router skips all vault governance calls and falls back to direct SkillGate behavior.

### Bridge ↔ Jetski

**Communication Pattern**: CDP WebSocket + JSON-RPC HTTP

**Key Components:**
- **Jetski Container** (`docker-compose.jetski.yml`): Standalone Docker service
- **CDP Proxy** (`jetski/internal/cdp/proxy.go`): Agent-facing on port 9222
- **RPC API** (`jetski/internal/rpc/rpc.go`): Management on port 9223
- **Bridge Browser Queue** (`bridge/pkg/queue/browser_queue.go`): Dispatches jobs to agents

**Data Flow:**
```
OpenClaw Agent → Jetski:9222 (CDP WebSocket connect)
Jetski → Lightpanda:9333 (proxied CDP)
Bridge → Jetski:9223 (RPC: status, sessions, health)
Jetski → Matrix (HITL approval requests)
```

**Network**: Jetski runs on `browser-net` (172.23.0.0/16) and `bridge-net` (armorclaw-bridge), enabling both agent-to-Jetski CDP and Bridge-to-Jetski RPC communication.

---

## Agent Communication Model

ArmorClaw supports two agent execution modes with different communication capabilities:

### Mode A: Agent Studio (Workflow Containers)

- **Network**: `NetworkMode: "none"` (zero network access)
- **Inbound**: Environment variables (`STEP_CONFIG`, `PII_*` fallback)
- **Outbound**: Exit code + `result.json` (step mode) or exit code only (agent mode)
- **Bind-mount**: `/var/lib/armorclaw/agent-state/{id}` mapped to `/home/claw/.openclaw` (read-write)
- **Step mode**: When `STEP_CONFIG` is present, container parses config, executes task, writes `result.json` to state dir, exits. Bridge reads via `ParseContainerStepResult()`. See `doc/agent-runtime.md` for details.
- **Agent mode**: When `STEP_CONFIG` is absent, container runs the OpenClaw agent Matrix polling loop (no backward channel in this mode)
- **Limitations**: No network access, no progress reporting, no browser automation, no CDP connectivity
- **Used by**: Secretary workflow engine (`StepExecutor`), task scheduler (`coldDispatch`)

### Browser Automation via Jetski Sidecar

Browser automation does not run inside agent containers. Instead, the Jetski sidecar handles all browser operations as a separate container with its own network stack (`network: "bridge"`):

- **Agent container**: `NetworkMode: "none"` (no network, as above)
- **Jetski sidecar**: Separate container with network access, runs the CDP proxy and Lightpanda browser engine
- **Communication path**: Agent → Bridge (Unix socket RPC) → Jetski (`:9222` CDP proxy) → Lightpanda (`:9333`)
- **Outbound proxy rotation**: Jetski's `ProxyManager` rotates outbound proxies for anti-WAF purposes (not for giving agent containers network access)
- **Security**: PII scrubbing at the CDP message level, SQLCipher session encryption, Matrix HITL approval for sensitive operations

> ⚠️ **CRITICAL**: Agent containers cannot browse the web directly. All browser automation goes through the Jetski sidecar. The Bridge brokers communication between the isolated agent container and the networked Jetski sidecar. Structured results are available in step mode via `result.json`.

---

## Agent Studio

### Purpose

Agent Studio provides **no-code agent creation and management**. Users can define, deploy, and manage AI agents through Matrix chat commands or the web dashboard.

### Agent Definition

```yaml
name: "Travel Booker"
skills:
  - web_browsing
  - form_filling
  - email
provider: openrouter
model: anthropic/claude-3.5-sonnet
system_prompt: |
  You are a travel booking assistant...
constraints:
  - require_approval_for: [payment, pii]
  - max_budget: 10.00  # USD per day
```

### Agent Lifecycle

```
Create → Deploy → Start → [Running] → Stop → Remove
         │                   │
         └─── Containers ────┘
              (Docker)
```

### Studio Service Interface

```go
type StudioService interface {
    HandleRPCMethod(method string, params json.RawMessage) *RPCResponse
}

// Methods: studio.deploy, studio.stats
```

---

## Browser Automation (BrowserBroker)

### Purpose

The BrowserBroker interface provides a **unified abstraction layer** for all browser automation operations in ArmorClaw. All browser ops, whether from the legacy browser-service or the Jetski sidecar, route through this interface.

### BrowserBroker Interface

**File**: `bridge/pkg/browser/broker.go`

The `BrowserBroker` interface defines 15 methods:

| Method | Description |
|--------|-------------|
| `Navigate` | Navigate browser to a URL |
| `Fill` | Inject values into form fields (sensitive fields route through PII approval) |
| `Click` | Click an element by selector |
| `WaitForElement` | Wait for an element to appear (with timeout) |
| `WaitForCaptcha` | Wait for CAPTCHA resolution (with timeout) |
| `WaitFor2FA` | Wait for 2FA code entry (with timeout) |
| `Extract` | Extract data from page via `ExtractSpec` |
| `Screenshot` | Capture page screenshot (full-page option) |
| `StartJob` | Start a new browser automation job |
| `Status` | Get current job status |
| `Complete` | Mark job as completed |
| `Fail` | Mark job as failed with reason |
| `List` | List all jobs for an agent |
| `Cancel` | Cancel a running job |
| `ReplayChart` | Replay all actions from a NavChart (PII actions route through approval) |

### Implementations

**JetskiBroker** (`bridge/pkg/browser/jetski_broker.go`): Primary implementation that routes all operations through the Jetski CDP proxy. Implements all 15 methods including `ReplayChart`.

**FallbackHandler** (`bridge/pkg/browser/broker_handler.go`): Temporary adapter that falls back to the legacy browser-service when Jetski is unavailable. Planned removal in Phase 4.

### Backend Feature Flag

The browser backend is controlled by the `ARMORCLAW_BROWSER_BACKEND` environment variable:

| Value | Backend | Description |
|-------|---------|-------------|
| `jetski` (default) | JetskiBroker | All ops route through Jetski CDP proxy |
| `legacy` | FallbackHandler | Ops route through legacy browser-service |

### Architecture

```
┌─────────────┐     ┌──────────────────────┐     ┌─────────────┐
│   Bridge    │────▶│   BrowserBroker      │────▶│  Jetski     │
│   RPC       │     │   Interface           │     │  CDP Proxy  │
└─────────────┘     │                      │     └─────────────┘
                    │  JetskiBroker ────────│───▶  :9222
                    │  FallbackHandler ─────│───▶  browser-service
                    └──────────────────────┘     (legacy, Phase 4 removal)
```

### Browser Skills

> All browser skills route through the BrowserBroker interface, regardless of backend. Agent containers (always `NetworkMode: "none"`) never perform browser operations directly.

| Skill | Description |
|-------|-------------|
| `navigate` | Navigate to URL |
| `fill` | Fill form fields |
| `click` | Click element |
| `wait_for_element` | Wait for element |
| `wait_for_captcha` | Wait for CAPTCHA |
| `wait_for_2fa` | Wait for 2FA |
| `extract` | Extract data |
| `screenshot` | Take screenshot |

### Job States

```
PENDING → RUNNING → COMPLETED
                 → FAILED
                 → CANCELLED
                  → AWAITING_PII
```

---

## Jetski Browser Sidecar

### Purpose

Jetski is a **Go-based CDP (Chrome DevTools Protocol) proxy** that provides secure browser automation for ArmorClaw agents. It sits between AI agents and the browser engine, implementing **Tethered Mode** security with PII scrubbing, encrypted sessions, and human-in-the-loop approval.

> **Architectural role**: Jetski is the **active CDP security layer** in the deployed system. It supersedes the Rust Vault's Phase 1 CDP interception design (removed v0.9.0) by operating as a full WebSocket proxy with richer security (PII scrubbing at the CDP message level, SQLCipher-encrypted sessions, and Matrix HITL approval). The Go-side BlindFill engine (`bridge/pkg/pii/`) handles placeholder resolution at the Bridge level.

### Key Features

| Feature | Description |
|---------|-------------|
| **CDP Proxy** | WebSocket proxy between agents (port 9222) and Lightpanda engine (port 9333) |
| **Translator** | CDP method translation and routing |
| **PII Scanner** | Active scrubbing of SSN, credit card, email, password patterns |
| **SQLCipher Sessions** | Encrypted session storage (PBKDF2-HMAC-SHA512, 256k iterations) |
| **Matrix HITL Approval** | Human-in-the-loop approval for sensitive browser operations (60s timeout) |
| **Sonar Telemetry** | Session monitoring and event recording |
| **RPC API** | Status, sessions, health, and approval endpoints on port 9223 |

### Architecture

```
┌─────────────┐     CDP :9222    ┌─────────────┐     CDP     ┌─────────────┐
│   Agent     │◀────────────────▶│   Jetski    │◀──────────▶│ Lightpanda  │
│  (OpenClaw) │   WebSocket      │ CDP Proxy   │  :9333     │  Engine      │
└─────────────┘                  └──────┬──────┘             └─────────────┘
                                        │
                         ┌──────────────┼──────────────┐
                         ▼              ▼              ▼
                  ┌────────────┐ ┌───────────┐ ┌────────────┐
                  │ PII Scanner│ │ SQLCipher │ │  Matrix    │
                  │ (scrub)    │ │ Sessions  │ │  HITL      │
                  └────────────┘ └───────────┘ └────────────┘
                         │              │              │
                         ▼              ▼              ▼
                  ┌────────────┐ ┌───────────┐ ┌────────────┐
                  │ Clean CDP  │ │ Encrypted │ │ User       │
                  │ Traffic    │ │ Storage   │ │ Approval   │
                  └────────────┘ └───────────┘ └────────────┘
```

### Component Structure

```
jetski/
├── cmd/observer/main.go       # Primary entry point - wires all components
├── internal/
│   ├── cdp/
│   │   ├── proxy.go           # CDP WebSocket proxy with PII scrubbing
│   │   ├── router.go          # Method router with Translator injection
│   │   ├── translator.go      # CDP method translation
│   │   └── fallback.go        # FallbackHandler for unsupported CDP methods
│   ├── rpc/
│   │   └── rpc.go             # JSON-RPC 2.0 server (port 9223)
│   ├── security/
│   │   ├── sqlcipher_session.go  # SQLCipher session store (PBKDF2, key zeroization)
│   │   ├── session.go            # Session management (rewritten from age to SQLCipher)
│   │   ├── pii_scanner.go        # 4-pattern PII detection (SSN, CC, email, password)
│   │   ├── store_cgo.go          # SQLCipher store (cgo build tag)
│   │   └── store_nocgo.go        # SQLCipher store (nocgo stub)
│   ├── approval/
│   │   └── matrix_client.go   # Matrix HITL approval client (channel-based, 60s timeout)
│   ├── sonar/
│   │   ├── recorder.go        # Telemetry event recorder
│   │   ├── reporter.go        # WreckageReport generation
│   │   ├── telemetry.go       # Telemetry types
│   │   └── buffer.go          # CDPFrame buffer for sonar capture
│   ├── subprocess/
│   │   ├── manager.go         # ProcessManager for engine lifecycle
│   │   ├── watchdog.go        # Watchdog for health monitoring
│   │   ├── health.go          # CDPHealthChecker interface
│   │   └── restart.go         # Restarter for engine recovery
│   └── network/
│       ├── proxy.go           # ProxyManager for outbound proxy rotation
│       └── circuit_breaker.go # CircuitBreaker for proxy failover
├── pkg/
│   ├── config/config.go       # Jetski configuration package
│   └── logger/logger.go       # Structured logging package
├── lighthouse/                # Nav-Chart REST API (Go sub-project)
├── jetski-chartmaker/         # Browser interaction recorder (TypeScript CLI)
├── configs/config.yaml        # Configuration file
├── Dockerfile                 # Container build (real SHA256)
└── go.mod                     # Standalone module (github.com/armorclaw/jetski)
```

### RPC API (Port 9223)

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/rpc` | `status` | Get observer status |
| `/rpc` | `sessions` | List active sessions |
| `/rpc` | `health` | Health check |
| `/rpc` | `approval.request` | Request HITL approval |
| `/rpc` | `approval.respond` | Respond to approval request |

### Tethered Mode

Tethered Mode is the security enforcement layer in Jetski. When enabled:

1. **PII Scrubbing**: All CDP traffic is scanned for PII patterns (SSN, credit card, email, password). Detected values are replaced with `[REDACTED_TYPE]` tokens.
2. **SQLCipher Sessions**: Browser sessions are encrypted using SQLCipher with PBKDF2-HMAC-SHA512 key derivation (256,000 iterations). Keys are zeroized from memory after use. `encryptSession=true` by default in Tethered Mode config.
3. **Matrix HITL Approval**: Sensitive browser operations (form submissions with PII, navigation to financial sites) require human approval via Matrix. Pending requests timeout after 60 seconds. Approval gating is active for PII input (`Input.insertText` with PII patterns) and navigation (`Page.navigate`).
4. **Free-Ride Mode**: When Tethered Mode is disabled, CDP traffic passes through without scrubbing (for development/testing).

### Configuration

**File**: `jetski/configs/config.yaml`

```yaml
server:
  host: "0.0.0.0"
  port: "9222"
  readTimeout: 30s
  writeTimeout: 30s

browser:
  enginePath: "/usr/local/bin/lightpanda"
  enginePort: "9333"
  healthCheck: true
  checkInterval: 5s
  watchdog:
    enabled: true
    checkInterval: 5s
    maxFailures: 3
    autoRestart: true
    restartDelay: 5s

security:
  passphrase: ""
  sessionDir: "./sessions"
  piiScanning: true
  encryptSession: true

network:
  proxyList: []
  proxyEnabled: false
  proxyHealthCheckURL: "http://www.google.com"
  proxyHealthInterval: 60s
  circuitBreaker:
    failureThreshold: 3
    resetTimeout: 30s
    halfOpenThreshold: 1

logging:
  level: "INFO"
  format: "text"
  output: "stdout"
  structured: false

approval:
  enabled: true
  bridgeURL: "http://127.0.0.1:8080"
  roomID: ""
  timeout: 60s
  sensitiveOperations:
    - "session_create"
    - "navigation"
    - "file_download"
```

**Environment Variable Overrides**:

| Variable | Field | Default |
|----------|-------|---------|
| `JETSKI_PORT` | `server.port` | `9222` |
| `JETSKI_HOST` | `server.host` | `0.0.0.0` |
| `JETSKI_ENGINE_PATH` | `browser.enginePath` | `/usr/local/bin/lightpanda` |
| `JETSKI_ENGINE_PORT` | `browser.enginePort` | `9333` |
| `JETSKI_PASSPHRASE` | `security.passphrase` | (empty) |
| `JETSKI_SESSION_DIR` | `security.sessionDir` | `./sessions` |
| `JETSKI_PII_SCANNING` | `security.piiScanning` | `true` |
| `JETSKI_ENCRYPT_SESSION` | `security.encryptSession` | `true` |
| `JETSKI_PROXY_LIST` | `network.proxyList` | (comma-separated) |
| `JETSKI_LOG_LEVEL` | `logging.level` | `INFO` |
| `JETSKI_LOG_FORMAT` | `logging.format` | `text` |
| `JETSKI_LOG_OUTPUT` | `logging.output` | `stdout` |
| `JETSKI_APPROVAL_ENABLED` | `approval.enabled` | `false` |
| `JETSKI_BRIDGE_URL` | `approval.bridgeURL` | `http://127.0.0.1:8080` |
| `JETSKI_ROOM_ID` | `approval.roomID` | (empty) |
| `JETSKI_APPROVAL_TIMEOUT` | `approval.timeout` | `60` (seconds) |

### Ports

| Port | Service | Protocol |
|------|---------|----------|
| 9222 | CDP WebSocket (agent-facing) | WebSocket |
| 9223 | RPC API (management) | HTTP/JSON-RPC |
| 9333 | Lightpanda engine | CDP over WebSocket |

### Sub-Projects

#### Lighthouse (Nav-Chart API)

- **Language**: Go (`github.com/armorclaw/lighthouse`)
- **Purpose**: REST API for navigation chart data
- **Files**: 17 Go source files
- **Port**: 8081 (planned)

#### Chartmaker (TypeScript CLI)

- **Language**: TypeScript (`@armorclaw/jetski-chartmaker`)
- **Purpose**: Record and replay browser interactions
- **Files**: 13 TypeScript source files
- **Build**: `npm install && npm run build`

### Docker Integration

**Compose File**: `docker-compose.jetski.yml`
**Network**: `browser-net` (172.23.0.0/16)
**Memory Limit**: 150MB
**Integration**: Included via `docker-compose.yml` with `include` directive

### Testing

| Test Suite | File | Tests | Status |
|------------|------|-------|--------|
| Router/Translator | `internal/cdp/router_test.go` | 4 | ✅ Pass |
| Proxy/PII Scanner | `internal/cdp/proxy_test.go` | 6 | ✅ Pass |
| PII Scrubbing | `internal/cdp/pii_scrub_test.go` | 8 | ✅ Pass |
| RPC API | `internal/rpc/rpc_test.go` | 8 | ✅ Pass |
| Sonar Telemetry | `internal/sonar/sonar_test.go` | 4 | ✅ Pass |
| SQLCipher Sessions | `internal/security/sqlcipher_session_test.go` | 10 | ✅ Pass |
| Session Round-trip | `internal/security/session_test.go` | 3 | ✅ Pass |
| Matrix HITL Approval | `internal/approval/matrix_client_test.go` | 12 | ✅ Pass |
| E2E Tethered Mode | `tests/e2e_tethered_test.go` | 5 | ✅ Pass |

**Total**: 183 Go tests, all passing

### NavChart Pipeline

The NavChart pipeline converts raw CDP interactions into reusable, validated, and audited navigation charts that can be replayed for future browser tasks on the same domain.

#### NavChart Go Types

**File**: `bridge/pkg/browser/chart_types.go`

Go types matching the chartmaker TypeScript schema:

| Type | Purpose |
|------|---------|
| `NavChart` | Top-level chart with version, target domain, metadata, and action map |
| `ChartAction` | Individual action (click, input, navigate, wait, assert) with selector and value |
| `ChartSelector` | 3-tier selector: primary CSS, secondary XPath, fallback JS |
| `ChartMetadata` | Generation source, timestamp, session ID |
| `FrameRouting` | iframe targeting (selector, name, origin) |
| `WaitCondition` | Post-action wait with type, selector, timeout |
| `Assertion` | Post-action assertion with type and expected value |
| `ActionType` | Enum: click, input, navigate, wait, assert |
| `SelectorTier` | Enum: primary, secondary, fallback, failed |

#### CDP-to-NavChart Normalization Pipeline

**File**: `bridge/pkg/browser/normalizer.go`

Raw CDP frames pass through a 6-stage pipeline:

1. **Filter** — Remove non-essential CDP events (network, console, DOM mutations)
2. **Group** — Correlate related CDP events into logical action boundaries
3. **Detect PII** — Scan values for SSN, credit card, email, and password patterns
4. **Replace** — Substitute detected PII with `{{VAULT:field:hash}}` placeholders
5. **Extract Selectors** - Derive CSS, XPath, and JS selectors from CDP target info
6. **Attach Metadata** — Add frame routing, wait conditions, and assertions

#### Chart Persistence

**File**: `bridge/pkg/browser/chart_store.go`

Charts are stored in the `learned_charts` table in the secretary SQLite database (`rolodex.db`).

**ChartStore interface**:

| Method | Purpose |
|--------|---------|
| `Save` | Store a new NavChart (creates new version) |
| `FindForDomain` | Find best-matching chart for a domain |
| `RecordOutcome` | Record replay success/failure, adjust confidence |
| `Get` | Retrieve a specific chart by ID |
| `Delete` | Remove a chart and all versions |
| `ListVersions` | List all versions of a chart |
| `RevertToVersion` | Roll back to a previous chart version |

#### Chart Validator

**File**: `bridge/pkg/browser/chart_validator.go`

Two validation modes:

**`ValidateForStorage`**: Checks charts before saving. Rejects charts containing PII values (must use placeholders). Enforces minimum selector requirements (primary CSS required).

**`ValidateForReplay`**: Checks charts before replay. Ensures all PII placeholders have resolved values. Validates domain policy (chart domain must match target domain).

#### Replay-Through-Approval

**`ReplayChart`** method on BrowserBroker replays all actions from a NavChart. Sensitive input actions (those with PII placeholders) are routed through the Bridge PII approval path. There are no shortcuts. If the user denies approval for any PII field, the entire replay aborts.

This guarantees that replay follows the same approval gates as manual browser operations.

#### Audit Trail

**File**: `bridge/pkg/browser/chart_audit.go`

The `chart_audit` table tracks 5 chart lifecycle events:

| Event | When |
|-------|------|
| `created` | New chart saved |
| `updated` | Chart version updated |
| `replayed` | Chart replayed (success or failure) |
| `rejected` | Validation rejected a chart |
| `deleted` | Chart removed |

Audit details never contain PII values. Only placeholder references, domain, and outcome are logged.

#### Selector Fallback

When replaying a chart action, selectors are tried in a 3-tier fallback order:

| Tier | Type | Confidence Adjustment |
|------|------|-----------------------|
| Primary | CSS selector | +0.05 on success |
| Secondary | XPath | +0.02 on success |
| Fallback | JavaScript expression | +0.01 on success |
| Failed | All tiers exhausted | -0.1 on failure |

Confidence scores are persisted with the chart and used by `FindForDomain` to rank candidates.

#### Chart Versioning

Charts use linear versioning. Each `Save` creates a new version. `ListVersions` returns all versions for a chart ID. `RevertToVersion` restores a previous version as the current active version.

#### PII Scanner

**File**: `bridge/pkg/browser/pii_scanner.go`

The `ScanChartsForPII` diagnostic tool scans all stored charts for PII values that should have been replaced with placeholders during normalization. Used for auditing and validation of the normalization pipeline.

#### Chart Injection

**File**: `bridge/pkg/secretary/orchestrator_integration.go`

The `ChartFinder` interface integrates with the workflow executor. Before spawning a browser step, relevant charts for the target domain are injected into the step configuration alongside learned skills. This lets the agent reuse proven interaction patterns.

#### NavChart Pipeline Test Coverage

**Go Unit Tests** (`bridge/pkg/browser/`):

| Test Suite | File | Tests | Status |
|------------|------|-------|--------|
| NavChart Types | `chart_types_test.go` | 4 | ✅ Pass |
| Normalizer Pipeline | `normalizer_test.go` | 13 | ✅ Pass |
| Chart Store | `chart_store_test.go` | 19 | ✅ Pass |
| Chart Validator | `chart_validator_test.go` | 37 | ✅ Pass |
| Chart Audit | `chart_audit_test.go` | 7 | ✅ Pass |
| PII Scanner | `pii_scanner_test.go` | 14 | ✅ Pass |

**Bash Harness Tests** (`tests/`):

| Test Suite | File | Scenarios | Status |
|------------|------|-----------|--------|
| Browser Broker Harness | `test-browser-broker.sh` | 14 (BB0-BB13) | ✅ Pass |
| NavChart Security | `test-navchart-security.sh` | 6 (NS0-NS5) | ✅ Pass |
| NavChart Pipeline | `test-navchart-pipeline.sh` | 6 (NP0-NP5) | ✅ Pass |

### Relationship to browser-service

Jetski **complements** (does not replace) the existing `browser-service/`:

| Aspect | browser-service | jetski |
|--------|----------------|--------|
| **Level** | Playwright automation API | CDP protocol proxy |
| **Purpose** | High-level browser tasks | Low-level browser control |
| **Security** | Job queue + PII approval | CDP message-level PII scrubbing |
| **Language** | TypeScript | Go |
| **Engine** | Playwright (Chromium) | Lightpanda |

---

## v6 Microkernel Governance (Feature-Flagged, Audit Mode in v0.6.0)

### Purpose

The v6 microkernel is a **governance layer** that adds ephemeral token lifecycle management, vault governance, and tool isolation to ArmorClaw. It is fully implemented in code but **disabled by default**. In v0.6.0, it operates in audit-only mode when enabled.

### Activation

```bash
# Enable v6 microkernel (requires Rust Vault governance service running)
export ARMORCLAW_V6_MICROKERNEL=true

# Or via TOML configuration
[vault]
v6_microkernel = true
socket_path = "/run/armorclaw/rust-vault.sock"
```

**Default**: `v6_microkernel = false` (see `bridge/pkg/config/config.go:990`)

### Architecture (v6 mode)

```
┌───────────────────────────────────────────────────────────────────────┐
│                       v6 Microkernel Architecture                     │
│                                                                       │
│  ┌─────────────┐     ┌──────────────┐     ┌──────────────────────┐   │
│  │ MCP Router  │────▶│ SkillGate    │────▶│ ToolSidecar          │   │
│  │ (router.go) │     │ (PII check)  │     │ (isolated container) │   │
│  └──────┬──────┘     └──────────────┘     └──────────────────────┘   │
│         │                                  ▲ v6Microkernel=true     │
│         │                                  │                         │
│         ▼                                  │                         │
│  ┌──────────────┐   gRPC/Unix   ┌─────────┴──────────┐              │
│  │ Vault Client │──────────────▶│ Rust Vault          │              │
│  │ (proto/)     │              │ Governance Service   │              │
│  └──────────────┘              │ - IssueEphemeralTok │              │
│                                │ - ConsumeEphemeral  │              │
│                                │ - ZeroizeToolSecrets│              │
│                                │ - SubscribeEvents   │              │
│                                └────────────────────┘              │
└───────────────────────────────────────────────────────────────────────┘
```

### Components

### Audit Mode (v0.6.0)

When v6 is enabled, it operates in **audit-only mode** by default. This logs what *would* happen without actually intercepting tool calls:

- Requires **both** `V6Microkernel=true` **and** `V6AuditMode=true` in VaultConfig
- Logs PII violations detected by SkillGate
- Logs governance checks that would block or redirect tool calls
- Logs would-be ToolSidecar spawns (no containers are actually created)
- **ToolSidecar communication protocol** is a hard prerequisite for enforcement mode. Until that protocol ships, audit mode is the safe default.
- Source: `bridge/pkg/mcp/router.go:handleAuditMode()`, `bridge/pkg/config/config.go`

#### MCP Router (`bridge/pkg/mcp/router.go`)

Routes all MCP `tools/call` requests through a security pipeline:

1. **SkillGate validation** — PII interception and redaction
2. **HITL consent workflow** — Human approval for PII operations
3. **ToolSidecar provisioning** — Isolated execution via Docker exec API (when v6 enabled)
4. **Vault governance** — Ephemeral token issuance + zeroization (when v6 enabled)
5. **Audit logging** — Compliance trail

> **vaultClient wiring**: `vaultClient` is passed directly to `setupMCPRouter()` in `bridge/cmd/bridge/setup_mcp.go`, which forwards it to `mcp.New()` config. The previous gap where vaultClient was nil is now closed.

```go
type MCPRouter struct {
    skillGate     interfaces.SkillGate
    consentMgr    *pii.HITLConsentManager
    auditor       *audit.AuditLog
    translator    *translator.RPCToMCPTranslator
    vaultClient   VaultClient    // wired via setupMCPRouter, nil when v6_microkernel=false
    v6Microkernel bool           // false by default
}
```

#### Vault Governance Client (`bridge/pkg/vault/proto/`)

Generated gRPC client stubs from `governance.proto`. Provides four methods:

| Method | Purpose |
|--------|---------|
| `IssueEphemeralToken` | Create short-lived token granting scoped secret access |
| `ConsumeEphemeralToken` | One-time use — token invalidated after consumption |
| `ZeroizeToolSecrets` | Securely erase all in-memory secrets for a tool/session |
| `SubscribeEvents` | gRPC server stream for governance events |

**Token lifecycle**: Issue → Consume (one-time) → Expire (TTL) → Zeroize

#### ToolSidecar (`bridge/pkg/toolsidecar/`)

> **Status: Implemented** — `Provisioner.SpawnToolSidecar()` creates hardened containers (NetworkMode: none, readonly, cap-drop ALL, 512MB memory). Tool execution uses Docker exec API for real command execution inside the container. `StopToolSidecar()` tears them down. Gated behind v6 microkernel flag (default: `V6Microkernel=false`).

```go
type ToolSidecar struct {
    ID        string
    SkillName string
    SessionID string
    CreatedAt time.Time
    Status    string
}
```

### Behavior: v6 On vs Off

| Aspect | v6 Microkernel OFF (default) | v6 Microkernel ON |
|--------|----------------------------|-------------------|
| **Vault governance** | Skipped entirely | Active — ephemeral tokens, zeroization |
| **Tool isolation** | Skills execute in-process | ToolSidecar containers (SpawnToolSidecar) |
| **Secret access** | Direct keystore retrieval | Vault-issued ephemeral tokens |
| **Event streaming** | No governance events | gRPC stream from Rust Vault |
| **Backward compat** | Full v4.x behavior | Enhanced security model |

### Test Coverage

4 dedicated tests in `bridge/pkg/mcp/router_test.go`:
- `TestExecuteTool_V6MicrokernelIssuesAndZeroizes` — verifies token issuance + zeroization
- `TestExecuteTool_V6MicrokernelOffSkipsVault` — verifies vault bypass when disabled
- Edge case tests for token lifecycle and consent integration

### Relationship to v4.x Documentation

This section documents code that exists in the repository but is **not active** in the current v4.10.0 release. The rest of this document describes the active v4.x architecture. When `v6_microkernel` is enabled, the MCP Router adds the governance layer described here on top of the existing v4.x components.

---

## Rust Office Sidecar

### Purpose

The Rust Office Sidecar is a **high-performance data plane component** for heavy I/O operations, separate from the Rust Vault security enclave. It handles:

- **Cloud Storage Access** - S3, SharePoint, Azure Blob operations
- **Document Processing** - PDF text extraction, DOCX parsing, OCR
- **Data Transformation** - Heavy computational work
- **Reliability Features** - Circuit breakers, rate limiting, retry logic

> **Routing split**: PDF and DOCX documents route to this Rust sidecar. XLSX and PPTX extraction also handled in Rust (calamine-based XLSX, PPTX parsing). DOC and PPT formats route to the Java Apache POI sidecar (`sidecar-java/`) with Python fallback. MSG and XLS formats route to the Python MarkItDown sidecar (`sidecar-python/`). See [doc/sidecar-pipeline.md](sidecar-pipeline.md) for the full 3-layer routing architecture.

### Architecture

```
┌─────────────────┐
│   Go Bridge     │ (Control Plane - Security Sovereignty)
│   Unix Socket   │
└────────┬────────┘
         │
         │ gRPC over Unix Socket
         │
┌────────▼────────┐
│  Rust Sidecar   │ (Data Plane - Heavy I/O)
│  ┌────────────┐ │
│  │ Connectors │ │ S3, SharePoint, Azure Blob
│  └────────────┘ │
│  ┌────────────┐ │
│  │ Documents  │ │ PDF, DOCX, XLSX, OCR
│  └────────────┘ │
│  ┌────────────┐ │
│  │  Security  │ │ Token Validation, HMAC
│  └────────────┘ │
│  ┌────────────┐ │
│  │ Reliability│ │ Circuit Breakers, Rate Limiting
│  └────────────┘ │
└─────────────────┘
```

### Compilation Status

**Binary + Library: ✅ Compiles Clean**
- 0 compilation errors (warnings only: unused imports, dead code)
- 252 lib tests pass, 0 failures, 8 ignored (260 total)
- All 8 gRPC RPCs functional

```bash
# Build library + binary
cd sidecar
cargo build --release

# Run tests
cargo test --lib
```

### Features

#### Cloud Connectors

| Connector | Status | Operations |
|-----------|--------|------------|
| **S3** | ✅ Working | Upload, download, list, delete, streaming |
| **SharePoint** | ✅ Working | Microsoft Graph API integration |
| **Azure Blob** | ⚠️ Disabled | OpenSSL dependency (needs rustls migration) |

#### Document Processing

| Format | Status | Features |
|--------|--------|----------|
| **PDF** | ✅ Working | Text extraction, metadata, merging |
| **DOCX** | ✅ Working | Text extraction; convert to PDF via printpdf |
| **XLSX** | ✅ Working | Calamine-based extraction (migrated from Python in v0.8.0); convert to CSV |
| **PPTX** | ✅ Working | Extraction supported; conversion not yet supported |
| **OCR** | ✅ Working | Tesseract primary + ONNX fallback chain, 16 languages |
| **Diff** | ✅ Working | Myers algorithm, HTML diff |

#### Security Features

| Feature | Description |
|---------|-------------|
| **Token Validation** | HMAC-SHA256 signatures |
| **Token TTL** | 30 minutes (ephemeral) |
| **Timestamp Validation** | 5-minute max age (replay prevention) |
| **Rate Limiting** | Token bucket algorithm |
| **Circuit Breakers** | Fault tolerance |

#### Reliability Features

| Feature | Description |
|---------|-------------|
| **Circuit Breakers** | Prevent cascade failures |
| **Rate Limiting** | Configurable token bucket |
| **Retry Logic** | Exponential backoff |
| **Metrics** | Prometheus integration |

### API Usage

#### Library Import

```rust
use armorclaw_sidecar::{
    connectors::{S3Connector, SharePointConnector},
    document::{extract_text_from_pdf, extract_text_from_docx},
    security::validate_token,
    error::{SidecarError, Result},
};
```

#### S3 Operations

```rust
// Initialize connector
let s3 = S3Connector::new(aws_config).await?;

// Upload file
let upload_result = s3.upload(S3UploadRequest {
    bucket: "my-bucket".to_string(),
    key: "document.pdf".to_string(),
    content: Some(pdf_bytes),
    file_path: None,
    content_type: Some("application/pdf".to_string()),
}).await?;

// Download file
let downloaded = s3.download(S3DownloadRequest {
    bucket: "my-bucket".to_string(),
    key: "document.pdf".to_string(),
    offset_bytes: None,  // Optional range request
    max_bytes: None,
}).await?;

// List objects
let objects = s3.list(S3ListRequest {
    bucket: "my-bucket".to_string(),
    prefix: Some("documents/".to_string()),
    max_keys: Some(100),
}).await?;

// Delete object
s3.delete(S3DeleteRequest {
    bucket: "my-bucket".to_string(),
    key: "old-file.pdf".to_string(),
}).await?;
```

#### Document Processing

```rust
// PDF text extraction
let pdf_bytes = std::fs::read("document.pdf")?;
let pdf_text = extract_text_from_pdf(&pdf_bytes)?;
println!("PDF content: {}", pdf_text);

// DOCX text extraction
let docx_bytes = std::fs::read("document.docx")?;
let docx_text = extract_text_from_docx(&docx_bytes)?;
println!("DOCX content: {}", docx_text);
```

#### Security

```rust
// Token validation
let token_info = validate_token(&token, &shared_secret)?;
if is_token_expired(&token_info) {
    return Err(SidecarError::AuthenticationFailed("Token expired".to_string()));
}

// Token structure
pub struct TokenInfo {
    pub user_id: String,
    pub issued_at: i64,
    pub expires_at: i64,
    pub signature: Vec<u8>,
}
```

### Configuration

#### Environment Variables

```bash
# gRPC Server
SIDECAR_SOCKET_PATH=/tmp/armorclaw-sidecar.sock
SIDECAR_MAX_CONCURRENT_REQUESTS=1000

# AWS S3
AWS_ACCESS_KEY_ID=AKIAIOSFODNN7EXAMPLE
AWS_SECRET_ACCESS_KEY=wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY
AWS_REGION=us-east-1

# SharePoint
SHAREPOINT_TENANT_ID=00000000-0000-0000-0000-000000000000
SHAREPOINT_CLIENT_ID=00000000-0000-0000-0000-000000000000
SHAREPOINT_CLIENT_SECRET=your-client-secret
SHAREPOINT_SITE_URL=your-site.sharepoint.com

# Security
SHARED_SECRET=your-256-bit-secret-here
```

#### Configuration Struct

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

### Performance Characteristics

| Metric | Value |
|--------|-------|
| **Throughput** | 1000+ concurrent requests (target) |
| **Latency** | <10ms for token validation |
| **Max File Size** | Up to 5GB supported |
| **Memory** | Efficient streaming (no full file loads) |
| **Runtime** | Tokio async/await |
| **I/O** | Zero-copy where possible |

### Testing

```bash
cd sidecar

# Run library tests (94% pass rate)
cargo test --lib

# Integration tests (require credentials)
cargo test --test aws_s3_integration_test
cargo test --test security_interceptor_integration_test
cargo test --test document_integration_test
```

**Test Coverage:**
- Security: 11 tests (token validation, signatures, expiration)
- Reliability: 5 tests (circuit breakers, concurrent operations)
- Rate Limiting: 15 tests (token bucket, replenishment, burst)
- Document Processing: 220+ tests (PDF, DOCX, XLSX, OCR, convert)
- gRPC Server: integration coverage via `sidecar/tests/e2e_integration_test.rs` (22 tests)
- Total: 252 lib tests pass, 0 failures, 8 ignored (260 total)

### Security Constraints

All security constraints from the plan are met:

- ✅ **NO** persistent credential storage in Rust sidecar
- ✅ **NO** credential caching beyond request lifecycle
- ✅ **NO** direct cloud API calls without Go Bridge
- ✅ **NO** audit logging in sidecar (Go Bridge handles)
- ✅ Token TTL: 30 minutes (ephemeral tokens)

### Known Limitations

| Limitation | Status | Workaround |
|------------|--------|------------|
| Azure Blob | Disabled (OpenSSL) | Use S3 or SharePoint |
| PPTX conversion | Not yet supported | Extraction works; PDF conversion pending |
| Binary warnings | 31 warnings (unused imports, dead code) | Non-blocking, cosmetic |

### Integration with Go Bridge

**Current State:**
- ✅ gRPC over Unix domain socket, 8 RPCs functional
- ✅ Proto files synced between Rust and Go (8 identical RPCs, including QueryDocuments)
- ✅ HealthCheck returns real telemetry: uptime_seconds, active_requests, memory_used_bytes, version
- ✅ ProcessDocument convert operations: DOCX→PDF (printpdf), XLSX→CSV (calamine)
- ✅ Token-based authentication via SecurityInterceptor
- ✅ Rate limiting and circuit breaking
- ✅ Separate from Rust Vault (different purpose)
- ✅ E2E integration tests: `sidecar/tests/e2e_integration_test.rs` (22 tests)

### Troubleshooting

#### "Library not found"
```bash
cd sidecar
cargo build --lib
```

#### "Token validation failed"
- Check shared secret matches
- Verify token hasn't expired (TTL: 30 minutes)
- Check timestamp is within 5 minutes
- Ensure HMAC signature is correct

#### "S3 upload failed"
- Verify AWS credentials
- Check bucket exists and is accessible
- Ensure region is correct
- Check IAM permissions

### Documentation References

| Document | Location |
|----------|----------|
| **Implementation State** | `sidecar/IMPLEMENTATION_STATE.md` |
| **README** | `sidecar/README.md` |
| **Security Audit** | `.sisyphus/audits/SECURITY_AUDIT_TASK_49.md` |
| **Source Code** | `sidecar/src/` |

### Summary

The Rust Office Sidecar **compiles clean and is production-ready** for:
- ✅ S3 and SharePoint cloud storage operations
- ✅ PDF and DOCX document processing
- ✅ XLSX extraction (calamine) and PPTX extraction
- ✅ DOCX→PDF and XLSX→CSV conversion
- ✅ OCR with Tesseract + ONNX fallback
- ✅ 8 gRPC RPCs functional (HealthCheck, UploadBlob, DownloadBlob, ListBlobs, DeleteBlob, ExtractText, ProcessDocument, QueryDocuments)
- ✅ Secure token validation
- ✅ Rate limiting and circuit breaking
- ✅ 252 lib tests passing, 8 ignored (260 total)

> See [doc/sidecar-pipeline.md](sidecar-pipeline.md) for the Go gRPC client, YARA scanner, and document pipeline architecture.

---

## ArmorChat Android Client

### Purpose

ArmorChat is the **Android mobile client** that provides:
- End-to-end encrypted messaging with agents
- Human-in-the-loop approval for sensitive operations
- QR code provisioning
- Push notifications

### Key Features

| Feature | Description |
|---------|-------------|
| **E2EE Messaging** | Megolm encryption via Matrix |
| **QR Provisioning** | Scan to connect to VPS |
| **PII Approval** | Approve/deny sensitive data access |
| **Push Notifications** | Real-time alerts via Sygnal |
| **Biometric Auth** | Secure keystore access |
| **Email Approval Card** (v0.6.0) | Email-based HITL approval flow with `EmailApprovalCard` |
| **Dynamic PII Masking** (v0.6.0) | `BlockerResponseDialog` masks 8 sensitive keywords using `PasswordVisualTransformation` |
| **Workflow Blocker Events** (v0.6.0) | Parses `workflow.blocker_warning` Matrix events for live blocker status |
| **Deep Link Navigation** (v0.7.0) | `DeepLinkHandler.kt` routes notification taps to correct screen |
| **Persistent Credentials** (v0.7.0) | `BridgeRepository` persists credentials via encrypted SharedPreferences |
| **Security Config Persistence** (v0.7.0) | `SecurityConfigViewModel` wired to `SecurityConfigScreen`, permissions survive app restarts |

### Provisioning Flow

```
Bridge: generate QR with setup_token
   ↓
ArmorChat: scan QR
   ↓
ArmorChat: POST /provisioning.claim
   ↓
Bridge: return admin_token
   ↓
ArmorChat: store credentials, connect to Matrix
```

> See [doc/client-applications.md](client-applications.md) for other client applications: Admin Panel, ArmorTerminal, Setup Wizard, and OpenClaw UI.

---

## OpenClaw Agent Runtime

### Purpose

OpenClaw is the **agent runtime** that executes inside isolated Docker containers. It provides:
- AI model integration
- Skill execution
- Browser automation
- Secure PII handling

### Container Security

```yaml
security_opt:
  - no-new-privileges:true
  - seccomp:seccomp-profile.json
  - apparmor:armorclaw-agent
cap_drop:
  - ALL
read_only: true
pids_limit: 100
memory: 512M
```

### Skills

OpenClaw includes **52 browser skills** for web automation:
- Navigation, form filling, clicking
- Data extraction, screenshots
- CAPTCHA/2FA handling
- File operations

### Context Management Architecture

OpenClaw manages LLM context windows through a **reactive overflow → compaction** pipeline. There is currently **no proactive (pre-overflow) compression**.

#### Context Window Resolution

The context window for each model is resolved through a priority chain:

1. **Per-provider config overrides** (`modelsConfig.providers.<provider>.models[].contextWindow`)
2. **Model's own `contextWindow`** field from the model registry
3. **`agents.defaults.contextTokens`** config cap
4. **`DEFAULT_CONTEXT_TOKENS = 200,000`** fallback

**Files**:
- `container/openclaw-src/src/agents/context-window-guard.ts` — `resolveContextWindowInfo()`, guards: `HARD_MIN=16,000`, `WARN_BELOW=32,000`
- `container/openclaw-src/src/agents/context.ts` — `MODEL_CACHE` (Map), `lookupContextTokens()`
- `container/openclaw-src/src/agents/defaults.ts` — `DEFAULT_CONTEXT_TOKENS = 200_000`

#### In-Memory Chat History

The conversation history lives in `activeSession.messages` (type `AgentMessage[]`):

- **Created**: `pi-embedded-runner/run/attempt.ts:575` — `createAgentSession()`
- **Replaced after trimming**: `attempt.ts:691` — `activeSession.agent.replaceMessages(limited)`
- **Persisted to disk**: Via `SessionManager.open(params.sessionFile)` — JSONL session file
- **Pre-compaction snapshot**: `pi-embedded-runner/compact.ts:582` — `const preCompactionMessages = [...session.messages]`

#### Compaction Pipeline (Post-Overflow, Reactive)

When a prompt exceeds the model's context window, the runtime detects the overflow error and triggers auto-compaction:

1. **Overflow detection** — `pi-embedded-runner/run.ts:585-601`:
   - Checks `promptError` and `assistantErrorText` for overflow patterns via `isLikelyContextOverflowError()`
2. **Auto-compaction trigger** — `run.ts:603-681`:
   - Up to `MAX_OVERFLOW_COMPACTION_ATTEMPTS = 3` retries
   - Calls `compactEmbeddedPiSessionDirect()` which opens the session file, runs `session.compact()`, and uses `estimateTokens()` + `generateSummary()` from `@mariozechner/pi-coding-agent`
3. **Fallback: tool-result truncation** — `run.ts:685-731`:
   - If compaction fails, truncates oversized tool results
4. **Final fallback** — `run.ts:744-765`:
   - Returns "Context overflow" error to user with `/reset` suggestion

#### Bridge-Side Pre-Dispatch Compaction (v0.6.0)

In addition to the reactive container-side compaction above, the Bridge now performs **pre-dispatch pruning** before sending prompts to the LLM:

- **Source**: `bridge/internal/ai/compaction.go`
- **Threshold**: `CompactionThresholdTokens` (default: 100,000) in VaultConfig, separate from `MaxTokens`
- **Behavior**: When estimated token count exceeds the threshold, the Bridge requests a summary from the LLM and replaces older messages with the summary before dispatch
- **Fallback**: Falls back to windowed truncation (keep most recent N messages) on LLM failure
- **Relationship**: This runs **before** the container-side reactive pipeline. Both layers cooperate: Bridge prunes proactively, container compacts reactively on overflow errors

#### Compaction Engine

**File**: `container/openclaw-src/src/agents/compaction.ts`

| Function | Purpose |
|----------|---------|
| `estimateMessagesTokens()` | Sum `estimateTokens(msg)` for all messages (strips `toolResult.details`) |
| `summarizeInStages()` | Split into N parts → summarize each → merge summaries |
| `pruneHistoryForContextShare()` | Drop oldest chunks until budget fits (50% of context by default) |
| `chunkMessagesByMaxTokens()` | Group messages into token-budgeted chunks |
| `computeAdaptiveChunkRatio()` | Adjust chunk size based on average message size |
| `summarizeWithFallback()` | Progressive: full → partial (skip oversized) → notes-only |

#### Proactive Compression: Available Hooks

> **Update**: The OpenClaw runtime **does** have internal task-completion signals, independent of the Bridge state machine. The capability gap is smaller than initially assessed — it is not "no proactive compression" but "no plugin leveraging the existing `agent_end` hook for proactive compression."

**Three-Trigger Architecture for Layer 0:**

| Tier | Hook | When It Fires | Purpose |
|------|------|---------------|---------|
| **Primary** | `agent_end` (success === true) | After task completes | Compaction at natural task boundaries |
| **Safety net** | `before_prompt_build` | Before every LLM call | Catches long single-task sessions |
| **Future** | External Bridge signal | On state machine → IDLE/COMPLETE | Reserved; requires Bridge→Container plumbing |

**`agent_end` plugin hook** — Fires at `attempt.ts:1151` after every LLM run completes. Receives `{messages, success, error, durationMs}`. Plugins register via `api.on("agent_end", handler)`.

**Why `agent_end` is the correct primary trigger:**

| Aspect | `before_prompt_build` | `agent_end` |
|--------|-----------------------|-------------|
| When it fires | Before every LLM call | After task completes |
| Risk of mid-task compaction | **Yes** — fires during multi-step tasks | **No** — `success === true` means task is done |
| Token cost of compaction itself | Charged against current task's context | Charged in a clean window after task is done |
| Message snapshot freshness | Messages about to be sent anyway | Completed task's final state — ideal for summarization |

**`success === true` gate with context-overflow exclusion:**
`agent_end` fires on *every* run completion, including failures and aborts. The handler must gate on `success === true` to avoid compacting corrupted or incomplete history. **Critical exclusion**: if the run failed *because* of context overflow (the existing `isLikelyContextOverflowError` path at `run.ts:585`), the handler must skip — the reactive compaction retry loop at `run.ts:603-681` already handles this case.

**`session.state` diagnostic event** — Fires via `clearActiveEmbeddedRun()` at `runs.ts:143` with `{state: "idle", reason: "run_completed"}`. Observable through `emitDiagnosticEvent()`.

**Full plugin lifecycle** — The system supports 20 hooks: `before_prompt_build`, `llm_input`, `llm_output`, `agent_end`, `before_compaction`, `after_compaction`, `session_start`, `session_end`, `before_model_resolve`, `before_agent_start`, `before_reset`, `message_received`, `message_sending`, `message_sent`, `before_tool_call`, `after_tool_call`, `tool_result_persist`, `before_message_write`, `gateway_start`, `gateway_stop`. All defined in `plugins/types.ts:298-318`.

**Recommended approach for Layer 0**: Register an `agent_end` plugin that gates on `success === true`, checks `estimateMessagesTokens(messages)` against the context window (~75% threshold), and calls `compactEmbeddedPiSessionDirect()`. Add a `before_prompt_build` safety net at `attempt.ts:838` for long single-task sessions. No Bridge changes needed.

> See [doc/agent-runtime.md](agent-runtime.md) for agent runtime internals: memory store, LRU cache, tool executor, and speculative execution.

---

## RPC API Reference

### Health & System (2 active + 5 planned)

| Method | Parameters | Returns | Description |
|--------|------------|---------|-------------|
| `health.check` | - | `{status, components}` | Health check |
| `mobile.heartbeat` | `{user_id}` | `{acknowledged}` | Mobile heartbeat |
| `system.health` | - | `{status, timestamp, uptime, checks}` | System health |
| `system.config` | - | `{version, features, endpoints, limits}` | System configuration |
| `system.info` | - | `{server, protocol, capabilities}` | System info |
| `system.time` | - | `{server_time, server_time_utc}` | System time |
| `device.validate` | `{device_id}` | `{valid, trust_level}` | Device validation |

> † `system.health`, `system.config`, `system.info`, `system.time`, `device.validate` are defined in `public_handlers.go` but not yet wired into the RPC handler map.

### AI (1 method)

| Method | Parameters | Returns | Description |
|--------|------------|---------|-------------|
| `ai.chat` | `{messages[], model, temperature, max_tokens, key_id}` | `{id, choices[], model, usage}` | Chat completion (rate-limited) |

### Matrix (5 methods)

| Method | Parameters | Returns | Description |
|--------|------------|---------|-------------|
| `matrix.status` | - | `{enabled, connected, logged_in, homeserver, user_id}` | Connection status |
| `matrix.login` | `{username, password}` | `{success, user_id}` | Login to homeserver |
| `matrix.send` | `{room_id, message, msgtype}` | `{event_id, room_id}` | Send message |
| `matrix.receive` | `{cursor, timeout_ms}` | `{events[], cursor, count}` | Receive events (long-poll) |
| `matrix.join_room` | `{room_id, via_servers, reason}` | `{room_id}` | Join room |

### Browser Automation (11 methods)

| Method | Parameters | Returns | Description |
|--------|------------|---------|-------------|
| `browser.navigate` | `{url, agent_id, job_id}` | `{job_id, status, url}` | Navigate to URL |
| `browser.fill` | `{job_id, selector, value}` | `{job_id, status, selector, success}` | Fill form field |
| `browser.click` | `{job_id, selector}` | `{job_id, status, selector, success}` | Click element |
| `browser.status` | `{job_id}` | `{job_id, status, url, session}` | Get job status |
| `browser.wait_for_element` | `{job_id, selector, timeout}` | `{job_id, status, success}` | Wait for element |
| `browser.wait_for_captcha` | `{job_id}` | `{job_id, status}` | Wait for CAPTCHA |
| `browser.wait_for_2fa` | `{job_id}` | `{job_id, status}` | Wait for 2FA |
| `browser.complete` | `{job_id}` | `{job_id, status, completed_at}` | Mark complete |
| `browser.fail` | `{job_id, reason}` | `{job_id, status, error}` | Mark failed |
| `browser.list` | - | `{jobs[], count}` | List all jobs |
| `browser.cancel` | `{job_id}` | `{job_id, status, cancelled_at}` | Cancel job |

### PII Management (9 methods)

| Method | Parameters | Returns | Description |
|--------|------------|---------|-------------|
| `pii.request` | `{agent_id, skill_id, profile_id, variables[], ttl}` | `{request_id, status, expires_at}` | Request PII access |
| `pii.approve` | `{request_id, user_id, approved_fields[]}` | `{request_id, status, approved_at}` | Approve access |
| `pii.deny` | `{request_id, user_id, reason}` | `{request_id, status, denied_at}` | Deny access |
| `pii.status` | `{request_id}` | `{request_id, status, fields}` | Get request status |
| `pii.list_pending` | - | `{requests[], count}` | List pending requests |
| `pii.stats` | - | `{stats}` | PII statistics |
| `pii.cancel` | `{request_id}` | `{request_id, status}` | Cancel request |
| `pii.fulfill` | `{request_id, resolved_vars}` | `{request_id, status}` | Mark fulfilled |
| `pii.wait_for_approval` | `{request_id, timeout}` | `{request_id, status}` | Wait for approval |

### Skills (14 methods)

| Method | Parameters | Returns | Description |
|--------|------------|---------|-------------|
| `skills.execute` | `{skill_name, params}` | `SkillResult` | Execute skill |
| `skills.list` | - | `{skills[], count}` | List enabled skills |
| `skills.get_schema` | `{skill_name}` | `{skill_name, schema}` | Get skill schema |
| `skills.allow` | `{skill_name}` | `{skill_name, status}` | Allow skill |
| `skills.block` | `{skill_name}` | `{skill_name, status}` | Block skill |
| `skills.allowlist_add` | `{type, value}` | `{type, value, status}` | Add to allowlist |
| `skills.allowlist_remove` | `{type, value}` | `{type, value, status}` | Remove from allowlist |
| `skills.allowlist_list` | - | `{ips[], cidrs[]}` | List allowlist |
| `skills.web_search` | `{params}` | `SkillResult` | Web search |
| `skills.web_extract` | `{params}` | `SkillResult` | Web extraction |
| `skills.email_send` | `{params}` | `SkillResult` | Send email |
| `skills.slack_message` | `{params}` | `SkillResult` | Slack message |
| `skills.file_read` | `{params}` | `SkillResult` | Read file |
| `skills.data_analyze` | `{params}` | `SkillResult` | Data analysis |

### Bridge Management (9 methods)

| Method | Parameters | Returns | Description |
|--------|------------|---------|-------------|
| `bridge.start` | - | `{status, message}` | Start bridge |
| `bridge.stop` | - | `{status, message}` | Stop bridge |
| `bridge.status` | `{user_id}` | `{enabled, status, stats}` | Get bridge status |
| `bridge.channel` | `{matrix_room_id, platform, channel_id}` | `{status}` | Bridge channel |
| `bridge.unchannel` | `{platform, channel_id}` | `{status}` | Unbridge channel |
| `bridge.list` | - | `{channels[], count}` | List bridges |
| `bridge.ghost_list` | - | `{ghosts[], count}` | List ghost users |
| `bridge.appservice_status` | - | `{status}` | AppService status |
| `store_key` | `{id, provider, token, display_name, base_url}` | `{success, id}` | Store API key |

### Events (2 methods)

| Method | Parameters | Returns | Description |
|--------|------------|---------|-------------|
| `events.replay` | `{offset, limit}` | `EventLogRecords[]` | Replay events |
| `events.stream` | `{offset, timeout_ms}` | `EventLogRecords[]` | Stream events (long-poll) |

### Studio (2 methods)

| Method | Parameters | Returns | Description |
|--------|------------|---------|-------------|
| `studio.deploy` | `{method_name, params}` | Varies | Deploy agent |
| `studio.stats` | - | `{agents, instances, skills}` | Studio statistics |

### Provisioning (2 methods)

| Method | Parameters | Returns | Description |
|--------|------------|---------|-------------|
| `provisioning.start` | - | `{setup_token, qr_data, expires_in}` | Start provisioning |
| `provisioning.claim` | `{setup_token, device_id, device_name}` | `{success, role, device_id}` | Claim device |

### Hardening (3 methods)

| Method | Parameters | Returns | Description |
|--------|------------|---------|-------------|
| `hardening.status` | - | `HardeningState` | Hardening status |
| `hardening.ack` | `{step}` | `HardeningState` | Acknowledge step |
| `hardening.rotate_password` | `{new_password}` | `{success}` | Rotate password |

### Secretary / Workflow (13 methods)

| Method | Parameters | Returns | Description |
|--------|------------|---------|-------------|
| `secretary.create_workflow` | `{template_id, variables, user_id?}` | `{workflow_id, status, steps[]}` | Create a new workflow instance from a template |
| `secretary.start_workflow` | `{template_id, variables}` | `{workflow_id, status, steps[]}` | Start a new workflow from template |
| `secretary.get_workflow` | `{workflow_id}` | `{workflow_id, status, state, steps[], current_step}` | Get workflow status and state |
| `secretary.cancel_workflow` | `{workflow_id}` | `{workflow_id, status, cancelled_at}` | Cancel a running workflow |
| `secretary.advance_workflow` | `{workflow_id, step_id, input}` | `{workflow_id, status, advanced_to}` | Manually advance a workflow step |
| `secretary.list_templates` | - | `{templates[], count}` | List available workflow templates |
| `secretary.create_template` | `{name, description, steps[]}` | `{template_id, status}` | Create a new workflow template |
| `secretary.get_template` | `{template_id}` | `{template_id, name, steps[]}` | Get template definition |
| `secretary.delete_template` | `{template_id}` | `{template_id, status}` | Delete a workflow template |
| `secretary.update_template` | `{template_id, name?, description?, steps[]?}` | `{template_id, status}` | Update template definition |
| `secretary.is_running` | `{workflow_id}` | `{running: bool}` | Check if a workflow is currently running |
| `secretary.get_active_count` | - | `{count: int}` | Get number of actively running workflows |
| `secretary.shutdown` | `{workflow_id}` | `{status}` | Gracefully shutdown a running workflow |

### Task Management (4 methods)

| Method | Parameters | Returns | Description |
|--------|------------|---------|-------------|
| `task.create` | `{title, description, priority, due_date}` | `{task_id, status, created_at}` | Create a new task |
| `task.list` | `{status?, limit?, offset?}` | `{tasks[], count, total}` | List tasks |
| `task.get` | `{task_id}` | `{task_id, title, status, steps[], created_at}` | Get task details |
| `task.cancel` | `{task_id}` | `{task_id, status, cancelled_at}` | Cancel a task |

### Device Management (4 methods)

| Method | Parameters | Returns | Description |
|--------|------------|---------|-------------|
| `device.list` | - | `{devices[], count}` | List registered devices |
| `device.get` | `{device_id}` | `{device_id, name, status, trust_level, last_seen}` | Get device details |
| `device.approve` | `{device_id}` | `{device_id, status, approved_at}` | Approve a pending device |
| `device.reject` | `{device_id, reason?}` | `{device_id, status, rejected_at}` | Reject a pending device |

### Invite Management (4 methods)

| Method | Parameters | Returns | Description |
|--------|------------|---------|-------------|
| `invite.create` | `{max_uses?, expires_in?}` | `{code, expires_at, max_uses}` | Create an invite code |
| `invite.list` | - | `{invites[], count}` | List active invites |
| `invite.revoke` | `{code}` | `{code, status, revoked_at}` | Revoke an invite |
| `invite.validate` | `{code}` | `{valid, expires_at, uses_remaining}` | Validate an invite code |

### Email Approval (4 methods)

| Method | Parameters | Returns | Description |
|--------|------------|---------|-------------|
| `approve_email` | `{approval_id, user_id}` | `{approval_id, status, approved_at}` | Approve a pending email |
| `deny_email` | `{approval_id, user_id, reason?}` | `{approval_id, status, denied_at}` | Deny a pending email |
| `email_approval_status` | `{approval_id}` | `{approval_id, status, email, created_at}` | Check email approval status |
| `email.list_pending` | - | `{approvals[], count}` | List pending email approvals |

### Container Management (2 methods)

| Method | Parameters | Returns | Description |
|--------|------------|---------|-------------|
| `container.terminate` | `{container_id}` | `{container_id, status, terminated_at}` | Terminate a running container (SIGKILL) |
| `container.list` | - | `{containers[], count}` | List running containers |

### Account (1 method)

| Method | Parameters | Returns | Description |
|--------|------------|---------|-------------|
| `account.delete` | `{user_id, confirmation}` | `{status, deleted_at}` | Delete user account |

### Standalone Methods (1 method)

| Method | Parameters | Returns | Description |
|--------|------------|---------|-------------|
| `resolve_blocker` | `{workflow_id, blocker_id, resolution}` | `{workflow_id, blocker_id, status, resolved_at}` | Resolve a workflow blocker |

---

## Event Types Reference

### Matrix Events
| Event | Description |
|-------|-------------|
| `matrix.message` | New message received |
| `matrix.receipt` | Read receipt |
| `matrix.typing` | Typing notification |
| `matrix.presence` | Presence update |

### Agent Events
| Event | Description |
|-------|-------------|
| `agent.started` | Agent started |
| `agent.stopped` | Agent stopped |
| `agent.status_changed` | Status transition |
| `agent.command` | Command received |
| `agent.error` | Error occurred |

### Workflow Events
| Event | Description |
|-------|-------------|
| `workflow.started` | Workflow started |
| `workflow.progress` | Progress update |
| `workflow.completed` | Workflow completed |
| `workflow.failed` | Workflow failed |
| `workflow.cancelled` | Workflow cancelled |
| `workflow.paused` | Workflow paused |
| `workflow.resumed` | Workflow resumed |

### HITL Events
| Event | Description |
|-------|-------------|
| `hitl.pending` | Approval pending |
| `hitl.approved` | Approval granted |
| `hitl.rejected` | Approval rejected |
| `hitl.expired` | Approval expired |
| `hitl.escalated` | Approval escalated |

### Budget Events
| Event | Description |
|-------|-------------|
| `budget.alert` | Budget alert |
| `budget.limit` | Budget limit reached |
| `budget.updated` | Budget updated |

### Platform Events
| Event | Description |
|-------|-------------|
| `platform.connected` | Platform connected |
| `platform.disconnected` | Platform disconnected |
| `platform.message` | Platform message |
| `platform.error` | Platform error |

> See [doc/communication-infra.md](communication-infra.md) for communication infrastructure internals: push notifications, SSO, WebSocket server, event bus, and platform adapters.

---

## Configuration Reference

### Environment Variables

| Variable | Description | Default |
|----------|-------------|---------|
| `ARMORCLAW_KEYSTORE_SECRET` | Base64-encoded keystore master key | - |
| `OPENROUTER_API_KEY` | OpenRouter API key | - |
| `ZAI_API_KEY` | xAI API key | - |
| `OPEN_AI_KEY` | OpenAI API key | - |
| `ARMORCLAW_SERVER_MODE` | Deployment mode (native/sentinel) | `native` |
| `ARMORCLAW_RPC_TRANSPORT` | RPC transport (unix/tcp) | `unix` |
| `ARMORCLAW_SOCKET_PATH` | Unix socket path | `/run/armorclaw/bridge.sock` |
| `ARMORCLAW_LISTEN_ADDR` | TCP listen address | `0.0.0.0:8080` |
| `ARMORCLAW_PUBLIC_BASE_URL` | Public base URL | - |
| `ARMORCLAW_EMAIL` | Admin email (Let's Encrypt) | - |
| `CF_API_TOKEN` | Cloudflare API token | - |
| `CF_TUNNEL_DOMAIN` | Cloudflare tunnel domain | - |

### TOML Configuration

**File**: `/etc/armorclaw/config.toml`

```toml
[server]
mode = "native"                    # native | sentinel
socket_path = "/run/armorclaw/bridge.sock"
listen_addr = "0.0.0.0:8080"
public_base_url = "https://your-domain.com"

[matrix]
enabled = true
homeserver_url = "http://localhost:6167"
username = "bridge"
password = ""

[ai]
default_provider = "openrouter"
default_model = "anthropic/claude-3.5-sonnet"
max_concurrent = 4

[keystore]
path = "/var/lib/armorclaw/keystore.db"

[logging]
level = "info"
format = "json"

[audit]
enabled = true
retention_days = 30

[browser]
enabled = true                           # Enable browser automation
service_url = "http://localhost:3002"     # Legacy browser-service HTTP API URL
cdp_url = "ws://localhost:9222"          # CDP WebSocket URL (Jetski)
rpc_url = "http://localhost:9223"        # JSON-RPC management URL (Jetski)
legacy_url = "http://localhost:3002"     # Legacy browser-service fallback URL
backend = "jetski"                       # Browser backend: "jetski" or "legacy"
fallback = false                         # Auto-fallback from jetski to legacy on failure
timeout = 30                             # Default timeout for browser operations (seconds)
max_retries = 3                          # Max retries for failed operations
retry_delay = 1                          # Delay between retries (seconds)

[browser.stealth]
enabled = false                          # Enable stealth anti-detection mode
fingerprint_seed = ""                    # Seed for deterministic fingerprint generation
human_like_typing = false                # Enable human-like typing delays
human_like_mouse = false                 # Enable human-like mouse movements
webgl_noise = false                      # Enable WebGL fingerprint noise
canvas_noise = false                     # Enable canvas fingerprint noise

[browser.queue]
max_workers = 4                          # Max concurrent browser workers
max_queue_size = 100                     # Max pending jobs in queue
job_timeout = 300                        # Max time a job can run (seconds)
priority_levels = 3                      # Number of priority levels (1-10)
```

**Browser Environment Variables**:

| Variable | Field | Default |
|----------|-------|---------|
| `ARMORCLAW_BROWSER_ENABLED` | `browser.enabled` | `false` |
| `ARMORCLAW_BROWSER_SERVICE_URL` | `browser.service_url` | (empty) |
| `ARMORCLAW_BROWSER_CDP_URL` | `browser.cdp_url` | (empty) |
| `ARMORCLAW_BROWSER_RPC_URL` | `browser.rpc_url` | (empty) |
| `ARMORCLAW_BROWSER_LEGACY_URL` | `browser.legacy_url` | (empty) |
| `ARMORCLAW_BROWSER_BACKEND` | `browser.backend` | `jetski` |
| `ARMORCLAW_BROWSER_FALLBACK` | `browser.fallback` | `false` |
| `ARMORCLAW_BROWSER_TIMEOUT` | `browser.timeout` | `30` |
| `ARMORCLAW_BROWSER_MAX_RETRIES` | `browser.max_retries` | `3` |
| `ARMORCLAW_BROWSER_RETRY_DELAY` | `browser.retry_delay` | `1` |
| `ARMORCLAW_BROWSER_STEALTH_ENABLED` | `browser.stealth.enabled` | `false` |
| `ARMORCLAW_BROWSER_FINGERPRINT_SEED` | `browser.stealth.fingerprint_seed` | (empty) |
| `ARMORCLAW_BROWSER_HUMAN_TYPING` | `browser.stealth.human_like_typing` | `false` |
| `ARMORCLAW_BROWSER_HUMAN_MOUSE` | `browser.stealth.human_like_mouse` | `false` |
| `ARMORCLAW_BROWSER_WEBGL_NOISE` | `browser.stealth.webgl_noise` | `false` |
| `ARMORCLAW_BROWSER_CANVAS_NOISE` | `browser.stealth.canvas_noise` | `false` |
| `ARMORCLAW_BROWSER_MAX_WORKERS` | `browser.queue.max_workers` | `4` |
| `ARMORCLAW_BROWSER_MAX_QUEUE_SIZE` | `browser.queue.max_queue_size` | `100` |
| `ARMORCLAW_BROWSER_JOB_TIMEOUT` | `browser.queue.job_timeout` | `300` |
| `ARMORCLAW_BROWSER_PRIORITY_LEVELS` | `browser.queue.priority_levels` | `3` |

---

## Deployment Modes

### Mode Comparison

| Feature | Native | Sentinel | Cloudflare Tunnel | Cloudflare Proxy | Self-Hosted |
|---------|--------|----------|-------------------|------------------|-------------|
| **Use Case** | Development | Production VPS | NAT/firewall | Existing CF | Home server, LAN |
| **Communication** | Unix socket | TCP + TLS | cloudflared tunnel | HTTP(S) proxy | TCP + HTTPS (Caddy) |
| **Access** | Local-only | Public | Public | Public | mDNS (LAN only) |
| **TLS** | None | Let's Encrypt | Cloudflare SSL | Cloudflare SSL | Self-signed |
| **Public IP Required** | No | Yes | No | Yes | No |
| **Setup Time** | ~2 min | ~5 min | ~3 min | ~5 min | ~5 min |

### Native Mode

**Configuration:**
```bash
ARMORCLAW_SERVER_MODE=native
ARMORCLAW_RPC_TRANSPORT=unix
ARMORCLAW_SOCKET_PATH=/run/armorclaw/bridge.sock
```

### Sentinel Mode

**Configuration:**
```bash
ARMORCLAW_SERVER_MODE=sentinel
ARMORCLAW_RPC_TRANSPORT=tcp
ARMORCLAW_LISTEN_ADDR=0.0.0.0:8080
ARMORCLAW_PUBLIC_BASE_URL=https://your-domain.com
ARMORCLAW_EMAIL=admin@your-domain.com
```

### Cloudflare Tunnel Mode

**Configuration:**
```bash
CF_API_TOKEN=your-token
CF_TUNNEL_DOMAIN=armorclaw.example.com
```

### Self-Hosted Mode (LAN Appliance)

**Configuration:**
```bash
# Single-command setup
sudo bash deploy/deploy-selfhosted.sh --auto

# Custom hostname
ARMORCLAW_HOSTNAME=myserver.local sudo bash deploy/deploy-selfhosted.sh --auto
```

**Key files:**
- `docker-compose.selfhosted.yml` -- Docker Compose stack
- `configs/Caddyfile.selfhosted` -- Reverse proxy with self-signed TLS
- `deploy/scripts/generate-certs.sh` -- Self-signed cert generation and rotation
- `.env.selfhosted` -- Environment template

**mDNS service types:** `_armorclaw._tcp` and `_openclaw-gw._tcp` (both advertised for discovery).

### TLS Mode Detection

ArmorClaw derives TLS mode from deployment topology:

| Deployment | TLS Mode | Trust Type | bridge.status.tls.health |
|------------|----------|------------|--------------------------|
| Native (Unix socket) | `none` | `""` | `ok` |
| Sentinel + self-signed cert | `private` | `self_signed` | `ok` / `degraded` |
| Sentinel + CA-issued cert | `public` | `public_ca` | `ok` / `degraded` |

**Shared-cert model**: Caddy reverse proxy and Bridge read the same certificate files from `/etc/armorclaw/certs/server.crt` and `server.key`. When both files are present, `bridge.status.tls.cert_source == "shared_cert"` and `health == "ok"`. If the cert file is missing, `cert_source == "proxy_only"` and `health == "degraded"` — the mode stays stable (topology doesn't change due to health).

**Fingerprint format**: Standard SHA-256 of DER-encoded certificate bytes (64 lowercase hex characters). Operators upgrading from pre-TLS versions should expect different fingerprint values than the old signature truncation method.

**Native mode zero-value semantics**: `bridge.status.tls` is always present in the RPC response. In native mode: `mode="none"`, `fingerprint_sha256=""`, `trust_type=""`, `expires_at=0`, `san_includes_public_ip=false`. Scripts can use simple string/numeric checks without null handling.

**QR payload versioning**: `/qr/config` emits v1 by default (no TLS fields). Set `ARMORCLAW_QR_VERSION=2` to enable v2 emission with `tls_mode`, `tls_fingerprint_sha256` (signed), `tls_trust_hint`, and `cert_expires_at` (unsigned informational metadata).

**/.well-known**: `/.well-known/matrix/client` includes `com.armorclaw.tls_mode` field reflecting the current TLS mode. In self-hosted/private topology, both `/.well-known` and API endpoints (`/api`, `/discover`, `/qr/config`) resolve to the same VPS host with Caddy routing by path.

**TLS environment variables**:
- `ARMORCLAW_TLS_MODE` — Override TLS mode (precedence over auto-detection)
- `ARMORCLAW_PUBLIC_IP` — External IP to include in certificate SANs
- `ARMORCLAW_QR_VERSION` — Set to `2` for QR v2 payload with TLS fields

**Client-side note**: ArmorChat's `QRConfigPayload` and `DiscoveredServer` models (separate codebase) do not yet consume TLS metadata fields. Server-side changes produce v2 QR payloads and extended well-known responses; ArmorChat needs updating to consume these.

---

## Testing & Verification

### Full System Test Harness

A comprehensive bash-based test harness covering **13 major ArmorClaw subsystems** with **4 cross-subsystem integration scenarios**, organized in two execution tiers.

**Shared Fixtures** (`tests/lib/`):

| File | Purpose |
|------|---------|
| `load_env.sh` | Environment loader (`.env` + `tests/e2e/common.sh` sourcing, `ssh_vps()`, `check_bridge_running()`) |
| `assert_json.sh` | 6 JSON assertion helpers (`assert_json_has_key`, `assert_json_equals`, `assert_rpc_success`, etc.) |
| `restart_bridge.sh` | Bridge restart with `flock` serialization and readiness polling |
| `event_subscriber_helper.sh` | WebSocket event subscription via `websocat` (graceful skip if unavailable) |
| `common_output.sh` | Standardized PASS/FAIL/SKIP counters + `harness_summary()` |
| `README.md` | Usage documentation per helper |

**Tier A — VPS-Deployed (live integration tests)**:

| Script | Subsystem | Scenarios |
|--------|-----------|-----------|
| `test-eventbus-streaming.sh` | EventBus + WebSocket | E0–E6: connection, subscription, fanout, filtering, reconnect |
| `test-trust-layer.sh` | Security / Trust Layer | S0–S7: PII detection, approval flow, denial, risk classification, audit trail |
| `test-system-health-baseline.sh` | System Health | H0–H6: bridge health, status, discovery, Matrix health, keystore, JSON summary |
| `test-secretary-workflow-core.sh` | Secretary Workflow Core | W0–W6: template lifecycle, state transitions, blocker resolution, restart survival |
| `test-email-pipeline.sh` | Email Approval Pipeline | M0–M6: approval status, list pending, deny, approve, restart persistence |

**Tier B — Code-Only (API contract tests, graceful skip on VPS)**:

| Script | Subsystem | Scenarios |
|--------|-----------|-----------|
| `test-secretary-workflow-deep.sh` | Secretary Workflow Deep | WD0–WD6: PII-gated halt, parallel steps, event validation, failover |
| `test-sidecar-docs.sh` | Document Sidecar Pipeline | D0–D7: Rust health, Python health, 3-layer routing, PII interception |
| `test-voice-stack.sh` | Voice Stack | V0–V5: budget enforcement, STT/TTS/VAD smoke, WebRTC sessions |
| `test-jetski-sidecar.sh` | Jetski Browser Sidecar | J0–J5: health check, session lifecycle, CDP proxy, PII scanner |
| `test-license-enforcement.sh` | License Enforcement | L0–L6: license status, features, compliance, platform limits, grace period |
| `test-platform-adapters.sh` | Platform Adapters | P0–P6: Matrix adapter, send/receive, join room, AppService, Slack (skip) |
| `test-agent-runtime.sh` | Agent Runtime | R0–R6: container list, studio agents, runtime health, isolation check |
| `test-deployment-usb.sh` | Deployment / USB | U0–U5: device detection, permission gating, unsafe device refusal |

**Cross-Subsystem Integration Scenarios**:

| Script | Cross-Subsystem | Scenarios |
|--------|----------------|-----------|
| `test-cross-workflow-email.sh` | Secretary → Email Approval | Workflow with email-sending step triggers approval |
| `test-cross-workflow-docs.sh` | Secretary → Document Sidecar | Workflow sends document task to sidecar pipeline |
| `test-cross-browser-trust.sh` | Jetski → Trust Layer | Browser CDP action hits trust boundary |
| `test-cross-event-truth.sh` | EventBus → Multi-subsystem | Live stream reflects state transitions in order |

**Running the Full Harness**:

```bash
# Run all Tier A scripts (requires VPS with bridge running) — includes TLS verification
for f in tests/test-eventbus-streaming.sh tests/test-trust-layer.sh \
         tests/test-system-health-baseline.sh tests/test-secretary-workflow-core.sh \
         tests/test-email-pipeline.sh \
         tests/test-tls-restart-safety.sh tests/test-tls-mode-integration.sh; do
  bash "$f"
done

# Run all Tier B scripts (structure check; graceful skip if subsystems undeployed)
for f in tests/test-secretary-workflow-deep.sh tests/test-sidecar-docs.sh \
         tests/test-voice-stack.sh tests/test-jetski-sidecar.sh \
         tests/test-license-enforcement.sh tests/test-platform-adapters.sh \
         tests/test-agent-runtime.sh tests/test-deployment-usb.sh; do
  bash "$f"
done

# Run cross-subsystem scenarios
for f in tests/test-cross-*.sh; do bash "$f"; done

# Syntax check all scripts
for f in tests/test-*.sh tests/lib/*.sh; do bash -n "$f" && echo "OK: $f"; done
```

**TLS suite integration**: TLS tests are also available through the Plan A harness via:
```bash
bash scripts/a4_harness.sh tls-mode
bash scripts/a4_harness.sh tls-restart
```

**Evidence and Results**:

- **Evidence path**: `.sisyphus/evidence/armorclaw/` (Plan A pipeline); `.sisyphus/evidence/tls/` (TLS verification)
- **Plan file**: `.sisyphus/plans/full-system-test-harness.md`
- **Full harness wall-clock target**: ≤ 10 minutes

### VPS SSH Test Suite

The testing suite also includes **11 comprehensive SSH-based test categories** with **226+ individual tests**:

| Category | Description | Test Count |
|----------|-------------|------------|
| **SSH Connectivity** | Key validation, connection, retry logic | 12 |
| **Command Execution** | Remote commands, output capture, exit codes | 8 |
| **Container Health** | Container status, logs, resource usage | 6 |
| **API Endpoints** | Bridge RPC, Matrix client, health checks | 8 |
| **Integration** | Cross-component communication | 8 |
| **Security** | Firewall, SSH hardening, container isolation | 35 |
| **Deployment Modes** | Native, Sentinel, Cloudflare detection | 6 |
| **SSL/TLS** | Certificate presence, expiry, chain | 6 |
| **Performance** | SSH speed, API times, container resources | 6 |
| **Output Formatting** | JSON console output, error handling | 1 |
| **Python Sidecar** | Worker unit tests, edge cases, token interceptor, Docker integration | 65 |
| **Java Sidecar** | JUnit 5 extraction tests, Go routing, Go E2E, bash harness | 33 |

```bash
# Run all SSH tests
bash tests/ssh/run_all_tests.sh --all

# Run specific category
bash tests/ssh/run_all_tests.sh --security

# Run with verbose output
bash tests/ssh/run_all_tests.sh --all --verbose
```

### Test Results Location

- **Full System Harness Evidence**: `.sisyphus/evidence/armorclaw/` (Plan A pipeline); `.sisyphus/evidence/tls/` (TLS verification)
- **SSH Evidence Directory**: `.sisyphus/evidence/`
- **Summary File**: `.sisyphus/evidence/IMPLEMENTATION_SUMMARY.md`
- **JSON Output**: `task-N-results.json`
- **Console Output**: `task-N-success.txt`

### Supplementary Test Scripts

Additional test scripts beyond the Tier A/B harness, organized by category.

**Test Directories**:

| Directory | Purpose |
|-----------|---------|
| `tests/unit/` | Unit tests |
| `tests/voice/` | Voice soak/load testing |
| `tests/voice-services/` | Voice service configs |
| `tests/config/` | Test configuration files |
| `tests/e2ee/` | E2EE cross-client testing |
| `tests/matrix-test-server/` | Matrix test server |

**Browser and NavChart Harness Tests**:

| Script | Scenarios | Description |
|--------|-----------|-------------|
| `test-browser-broker.sh` | BB0-BB13 | BrowserBroker interface tests (also in Tier B table) |
| `test-navchart-security.sh` | NS0-NS5 | NavChart PII validation and security checks |
| `test-navchart-pipeline.sh` | NP0-NP5 | NavChart normalize-store-replay pipeline |

**Integration and Smoke Tests**:

| Script | Description |
|--------|-------------|
| `test-vps-smoke.sh` | VPS integration smoke test |
| `test-persistence.sh` | Invite state survival across restarts |
| `test-matrix-plane.sh` | Matrix messaging via Conduit API |
| `test-rpc-methods.sh` | RPC method coverage |
| `test-e2e.sh` | End-to-end test script |

**Security and Exploit Tests**:

| Script | Description |
|--------|-------------|
| `test-exploits.sh` | Exploit prevention tests |
| `test-secrets.sh` | Secret passing mechanism |
| `test-governance-rpc.sh` | Governance RPC tests |
| `test-secret-passing.sh` | Keystore to bridge to container secret flow |
| `test-p0crit3-socket-injection.sh` | Socket-based secret injection |
| `test-yara-heap-profile.sh` | YARA heap profiling |
| `test-xchacha-nonce-length.sh` | XChaCha nonce validation |

**Transport and Voice Tests**:

| Script | Description |
|--------|-------------|
| `test-transport-guard.sh` | Transport guard tests |
| `test-eventbus-filtering.sh` | EventBus filtering |
| `test-webrtc-voice-integration.sh` | WebRTC voice and Matrix integration |

**Deployment and Config Tests**:

| Script | Description |
|--------|-------------|
| `test-discovery.sh` | mDNS discovery |
| `test-element-x-flow.sh` | Element X config flow E2E |
| `test-container-setup.sh` | Container setup |
| `test-attach-config.sh` | Config attachment |
| `test-deployment-skills.sh` | Deployment skills |
| `test-cloudflare-setup.sh` | Cloudflare setup |
| `test-quickstart-entrypoint.sh` | Quickstart entrypoint |
| `test-matrix-integration.sh` | Matrix integration |

---

## Document Index

### Primary Documentation
- **README.md** - System overview and quick start
- **ARMORCLAW.md** - AI-powered deployment skills introduction
- **AGENTS.md** - Agent OS orchestration guidance
- **CLAUDE.md** - Claude Code development standards

### Sidecar Documentation
- **doc/sidecar-pipeline.md** - Document processing pipeline (Rust + Python + Java sidecars, Go routing, YARA)
- **sidecar/README.md** - Rust sidecar internals (connectors, encryption, provenance)
- **sidecar-python/worker.py** - Python MarkItDown sidecar (MSG, XLS conversion; DOC/PPT migrated to Java)
- **sidecar-java/** - Java Apache POI sidecar (DOC, PPT extraction via HWPFDocument/HSLFSlideShow)
- **bridge/pkg/sidecar/office_client.go** - 3-layer routing logic with Java DOC/PPT path (native bypass, compound validation, strict drop)

### Review Documentation
- `applications/ArmorChat-review.md` - Android client review
- `applications/ArmorTerminal-review.md` - Terminal client review
- `DEPLOYMENT_SKILLS_REVIEW.md` - Deployment skills audit

### Jetski Documentation
- **Jetski Sidecar**: `jetski/README.md` - Browser sidecar documentation
- **Jetski Integration Plan**: `.sisyphus/plans/jetski-integration.md` - Integration plan and status

---

## Local Development Guide

ArmorClaw provides a complete local development environment using Docker Desktop. This enables developers to run, test, and modify the system without provisioning a VPS.

### Quick Start

```bash
# 1. Install Docker Desktop (https://www.docker.com/products/docker-desktop)
# 2. Clone the repository
git clone https://github.com/Gemutly/ArmorClaw.git
cd armorclaw

# 3. Start all services
docker-compose up -d

# 4. Verify deployment
curl http://localhost:8080/health
# Should return health status

# 5. View logs
docker-compose logs -f bridge
```

### Development Workflow

| Action | Command |
|--------|---------|
| Start all services | `docker-compose up -d` |
| Stop all services | `docker-compose down` |
| View logs (all services) | `docker-compose logs -f` |
| View logs (specific service) | `docker-compose logs -f bridge` |
| Restart service | `docker-compose restart bridge` |
| Rebuild service | `docker-compose up -d --build bridge` |
| Execute command in container | `docker exec -it armorclaw-bridge bash` |
| Clean up | `docker system prune -a --volumes -f` |

### Hot Reload Development

For Bridge development, you can enable hot-reloading:

```bash
# Install inotify-tools (Linux)
sudo apt-get install inotify-tools

# Watch for changes and rebuild
while inotifywait -e modify -r bridge/; do
  docker-compose up -d --build bridge
done
```

### Running Tests Locally

```bash
# Run all test suites
make test-all

# Run specific test suites
make test-hardening
make test-secrets
make test-exploits
make test-e2e

# Quick smoke test (hardening only)
make smoke

# Run Go unit tests
cd bridge
go test ./...

# Run Rust Vault tests
cd rust-vault
cargo test --all

# Run JetSki tests
cd jetski
go test ./...

# Run Python MarkItDown sidecar tests
cd sidecar-python
python -m pytest test_worker.py test_edge_cases.py test_interceptor.py -v

# Run Java Apache POI sidecar tests (requires Java 21)
cd sidecar-java
JAVA_HOME="$(asdf where java temurin-21.0.11+10.0.LTS)" mvn test

# Run Go routing + E2E tests (includes Java DOC/PPT routing paths)
cd bridge
go test -v -run "TestRouteExtractText|TestE2E|TestJavaSidecarE2E" ./pkg/sidecar/...
```

### Environment Variables for Development

Create a `.env` file in the project root:

```bash
ARMORCLAW_ENV=development
LOG_LEVEL=debug
# Add any API keys needed for testing:
# OPENROUTER_API_KEY=sk-or-...
# OPENAI_API_KEY=sk-...
```

### Ports Used in Local Development

| Port | Service | Protocol |
|------|---------|----------|
| 8080 | Bridge API (TCP) | HTTP |
| 6167 | Matrix Conduit | HTTP |
| 80 | Caddy HTTP Proxy | HTTP |
| 443 | Caddy HTTPS Proxy | HTTPS |
| 9222 | JetSki CDP Proxy (agent-facing) | WebSocket |
| 9223 | JetSki RPC API | HTTP/JSON-RPC |
| 9333 | Lightpanda Engine | CDP over WebSocket |
| Unix socket | Python MarkItDown Sidecar | gRPC (`/run/armorclaw/sidecar-office.sock`) |
| Unix socket | Java Apache POI Sidecar | gRPC (`/run/armorclaw/sidecar-java.sock`) |

---

## Agent State Machine (Go Bridge)

The Go Bridge manages agent lifecycle through a **state machine** (`bridge/pkg/agent/state.go`) with **11 states** and validated transitions.

**Agent State Persistence**: Agent state directory is bind-mounted at `/var/lib/armorclaw/agent-state/{definitionID}/` to `/home/claw/.openclaw` inside containers. This overrides `ReadonlyRootfs` for the state path specifically. JSONL sessions, agent configuration, and logs persist across container lifecycle events (stop, remove, re-spawn).

### States

| State | Description | Category |
|-------|-------------|----------|
| `IDLE` | Agent not performing any task | Rest |
| `INITIALIZING` | Agent starting up | Active |
| `BROWSING` | Navigating to a URL | Active |
| `FORM_FILLING` | Filling form fields | Active |
| `AWAITING_CAPTCHA` | Needs human CAPTCHA solving | Terminal (needs user) |
| `AWAITING_2FA` | Needs a 2FA code | Terminal (needs user) |
| `AWAITING_APPROVAL` | Waiting for BlindFill PII approval | Terminal (needs user) |
| `PROCESSING_PAYMENT` | Submitting a payment | Active |
| `ERROR` | Recoverable error | Recovery |
| `COMPLETE` | Task finished successfully | Terminal (final) |
| `OFFLINE` | Agent not reachable | Terminal (final) |

### Transitions Leading to IDLE or COMPLETE

Every path that ends a task cycle:

| From | To | Trigger | Code Path |
|------|----|---------|-----------|
| `INITIALIZING` | `IDLE` | Agent startup finished | `state_machine.go:231` `SetReady()` |
| `BROWSING` | `COMPLETE` | Browser task finished successfully | `browser.go:291` `integration.CompleteTask()` |
| `BROWSING` | `IDLE` | Browser navigation ended without task | `state.go:58` (valid transition) |
| `FORM_FILLING` | `COMPLETE` | Form submission succeeded | `browser.go:291` |
| `FORM_FILLING` | `IDLE` | Form filling cancelled/ended | `state.go:68` |
| `AWAITING_CAPTCHA` | `IDLE` | Captcha resolution abandoned | `state.go:74` |
| `AWAITING_2FA` | `IDLE` | 2FA resolution abandoned | `state.go:81` |
| `AWAITING_APPROVAL` | `IDLE` | Approval abandoned | `state.go:87` |
| `PROCESSING_PAYMENT` | `COMPLETE` | Payment succeeded | `state.go:90`, `processor.go:165` |
| `PROCESSING_PAYMENT` | `IDLE` | Payment cancelled | `state.go:93` |
| `ERROR` | `IDLE` | Error recovered | `state_machine.go:293` `Reset()` |
| `COMPLETE` | `IDLE` | Post-completion reset (terminal drain) | `state.go:100` |

### Event Flow: Bridge → Matrix → OpenClaw

When a state transition occurs:

1. **`StateMachine.Transition()`** (`state_machine.go:59`) validates the transition, emits a `StatusEvent` to:
   - `sm.eventChan` (buffered channel, 100 capacity)
   - All subscribers via `sm.subscribers`
2. **`Integration.processEvents()`** (`integration.go:232`) reads from the event channel and calls `onStatusChange` callback (if set)
3. **`StatusEvent`** has `EventType()` → `"com.armorclaw.agent.status"` (Matrix event type)

### State Signal Propagation (v0.6.0 Partial Close)

The `onStatusChange` callback is now wired through state inference in v0.6.0. The Bridge infers agent state from container lifecycle events and workflow progress:

- `OnStatusChange()` (`integration.go:66`) accepts a callback. In v0.6.0, the workflow executor sets this callback when orchestrating multi-step workflows.
- `AgentCoordinator.BroadcastStatus()` (`integration.go:339`) was previously a stub. In v0.6.0, state inference is implemented: the Bridge infers agent state from container lifecycle events and workflow progress rather than relying on network push. The eventbus now publishes agent state machine events.
- The eventbus `EventTypeAgentStatusChanged` (`eventbus/events.go:22`) now receives published agent state events
- OpenClaw TypeScript code has **zero references** to `com.armorclaw.agent.status`, `state_machine`, or `StatusEvent` (container-side awareness unchanged)

### Implication for Layer 0 (Context Window Persistence)

The Bridge state machine's `→ IDLE` / `→ COMPLETE` transitions do not reach the container. However, the OpenClaw runtime has its own `agent_end` plugin hook (see [Context Management Architecture](#context-management-architecture)) that fires after every LLM run with `{messages, success, error, durationMs}`. This is the natural compression trigger — no Bridge changes needed.

**Three-trigger approach for Layer 0:**

1. **Register an `agent_end` plugin** (primary trigger — recommended): The plugin gates on `success === true`, checks `estimateMessagesTokens(messages)` against the context window (~75% threshold), and calls `compactEmbeddedPiSessionDirect()`. Compaction runs at natural task boundaries where summaries are most coherent. **Critical exclusion**: skip when the error is a context overflow — the existing reactive compaction retry loop at `run.ts:585-681` already handles this. No cross-process plumbing needed — the hook already fires in-process with the message snapshot.

2. **Add a `before_prompt_build` token check** (safety net): At `attempt.ts:838`, check `estimateMessagesTokens(activeSession.messages)` vs `ctxInfo.tokens * 0.75` before each LLM call. This catches long single-task sessions that never cross a task boundary. Note: this fires mid-task, so compaction here is more disruptive — use only as fallback.

3. **External Bridge signal** (future): A Bridge→Container RPC or Matrix event triggered on state machine `→ IDLE` / `→ COMPLETE`. Reserved; requires new cross-process plumbing.

Tiers 1 and 2 coexist today — `agent_end` gives compaction at task boundaries (cheaper, more coherent), while `before_prompt_build` is a safety net. Tier 3 is a future extension point if cross-boundary events become necessary.

---

## Review Documentation

See the [Document Index](#document-index) for links to all review documents including:
- `applications/ArmorChat-review.md` - Android client architecture review
- `applications/ArmorTerminal-review.md` - Terminal client review
- `DEPLOYMENT_SKILLS_REVIEW.md` - Deployment skills audit

---

**End of Documentation**
