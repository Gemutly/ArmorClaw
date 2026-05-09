# Client Applications

> Part of the [ArmorClaw System Documentation](armorclaw.md)

## Overview

ArmorClaw ships with several client applications, each targeting a different phase of the product lifecycle: initial setup, ongoing administration, and agent interaction. All web-based clients communicate with the Go Bridge over JSON-RPC. Android clients connect through Matrix and the Bridge RPC API.

This document covers four applications. ArmorChat, the primary mobile client, is documented separately.

## ArmorChat (Android)

> See [doc/ArmorChat.md](ArmorChat.md) for full documentation.

The consumer-facing mobile app. End-to-end encrypted messaging with agents via Matrix, biometric keystore, HITL approval flows, and QR provisioning.

### Workflow UI Components (v3)

Three Jetpack Compose components added by the Agent Studio Improvement Plan v3 give users real-time visibility into agent workflows and a human-in-the-loop mechanism for resolving blockers.

#### WorkflowTimeline

Renders workflow events as a scrollable vertical timeline. Each row shows an emoji icon (mapped from Go bridge `stepIcon()` values), step name, optional detail line, and duration badge. A progress bar and live/paused/complete status indicator sit above the event list. The component consumes `WorkflowTimelineState` (event list, progress float, running flag) from a ViewModel/StateFlow pattern fed by Matrix `/sync` events. It does not manage network connections.

**Key types:** `WorkflowEvent` (seq, type, name, tsMs, detail, durationMs), `WorkflowTimelineState` (events, progress, isRunning, workflowName)

**Icon mapping:** step, file_read, file_write, file_delete, command_run, observation, blocker, error, artifact, checkpoint

#### BlockerResponseDialog

Modal dialog for resolving workflow blockers that need user input. Displays the blocker message, field label, and optional suggestion. Sensitive fields (password, card, key, token, secret, cvv, pin, ssn) get password masking automatically. Includes a collapsible note field and a state machine with four states: INPUT, LOADING, ERROR, DISMISSED. On submit, calls the parent's `onResolve` callback with (workflowId, stepId, input, note), which the ViewModel routes to `BridgeApi.resolveBlocker()`.

**Key types:** `BlockerInfo` (blockerType, message, suggestion, field, workflowId, stepId), `BlockerDialogState` enum

#### GovernanceBanner

Persistent compact banner showing the current workflow status. Maps to Go `WorkflowStatus` values: IDLE (hidden), RUNNING (blue, pulsing dot, step counter), BLOCKED (amber, clickable, triggers `BlockerResponseDialog`), COMPLETED (green), FAILED (red), CANCELLED (grey). The BLOCKED variant accepts an `onBlockedTap` callback to open the blocker resolution flow.

**Key type:** `WorkflowStatus` enum with `fromGo(String)` parser for Go-side lowercase values

### Blocker Resolution RPC

`BridgeApi.resolveBlocker()` sends a JSON-RPC call to the Bridge with the user's response:

```
method: "resolve_blocker"
params: { workflow_id, step_id, input, note? }
returns: Result<Map<String, String>>
```

The input field is never logged or cached (PII safety). Sensitive values are masked in the UI and travel E2EE through the Bridge to the Orchestrator, which unblocks the workflow and lets the container retry.

## Admin Panel

| | |
|---|---|
| **Location** | `applications/admin-panel/` |
| **Tech stack** | React 18, TypeScript, Vite, Tailwind CSS, TanStack React Query, React Router |
| **Status** | In development (v0.7.0: API wiring complete) |

### Purpose

The admin panel is a browser-based dashboard for managing an ArmorClaw instance after initial setup. It surfaces operational controls that don't belong in the mobile app: device trust management, adapter configuration, invitation lifecycle, audit log review, and security policy tuning.

### Key Pages

- **AdminDashboard** - System overview and status (v0.7.0: real data via `useDashboardStats` hook)
- **DevicesPage** - List, approve, and reject connected devices (v0.7.0: real data via Bridge RPC)
- **AdaptersPage** - Enable, disable, and configure adapters (v0.7.0: real data via Bridge RPC)
- **APIKeysPage** - Manage provider API keys (v0.7.0: real data via Bridge RPC)
- **InvitationsPage** - Create and revoke invite codes (v0.7.0: real data via Bridge RPC)
- **AuditLogPage** - Browse security and system audit events (v0.7.0: real data via Bridge RPC)
- **SecurityConfig** - Configure data categories and security tiers
- **LoginPage** - Session authentication

### Bridge Connection

The admin panel talks to the Bridge through an HTTP proxy that forwards requests to the Bridge's Unix domain socket. The client (`src/services/bridgeApi.ts`) sends JSON-RPC 2.0 calls to `/api/*`, which a reverse proxy (Caddy or Nginx) routes to `bridge.sock`. It supports the full lockdown lifecycle: claiming ownership, transitioning between modes (lockdown, bonding, configuring, hardening, operational), and generating QR codes for mobile provisioning.

> **v0.7.0**: Mock data has been removed from all admin panel pages. `bridgeApi.ts` now provides typed React Query hooks (`useDashboardStats`, `useDevices`, `useAdapters`, `useInvitations`, `useAuditLogs`, `useApiKeys`) that call real Bridge RPC endpoints. Mutations use `onSuccess` to invalidate query cache for automatic refetch.

## ArmorTerminal

| | |
|---|---|
| **Location** | `applications/ArmorTerminal/` |
| **Tech stack** | Android (Kotlin), traditional Android SDK, OkHttp, Retrofit |
| **Min SDK** | 26 (Android 8.0) |
| **Target SDK** | 34 |
| **Status** | In development |

### Purpose

ArmorTerminal is an Android application for provisioning new ArmorClaw installations from a phone or tablet. It handles the physical pairing workflow: discovering bridges on the local network, scanning QR codes, verifying certificate fingerprints, registering the device, and waiting for admin approval. Think of it as the field technician's tool for bringing a fresh VPS online.

### Key Components

- **BridgeDiscovery** - mDNS/Bonjour service discovery for finding ArmorClaw bridges on the local network, with manual IP fallback
- **PairingViewModel** - Orchestrates the full pairing flow: discovery, QR parsing, certificate verification, device registration, and approval polling
- **ResilientWebSocket** - Auto-reconnecting WebSocket with exponential backoff for real-time status updates during pairing
- **BridgeTrustStore** - Certificate pinning and trust-on-first-use for discovered bridges
- **ConfigManager** / **SignedConfigParser** - Parses bridge configuration from QR codes and signed payloads
- **NetworkResilience** - Retry policies and connectivity monitoring for unreliable network conditions

### Bridge Connection

ArmorTerminal discovers bridges via mDNS (`NsdManager`) or manual IP entry. Once connected, it calls the Bridge's JSON-RPC API over HTTP(S) for device registration and uses WebSocket for real-time approval status. The Matrix SDK handles E2EE messaging setup after provisioning completes. Debug builds point to the Android emulator host (`10.0.2.2`), while release builds use configurable endpoints via Gradle properties.

## Setup Wizard

| | |
|---|---|
| **Location** | `applications/setup-wizard/` |
| **Tech stack** | React 18, TypeScript, Vite, Tailwind CSS |
| **Status** | In development |

### Purpose

The setup wizard is a streamlined web application that runs during first-time ArmorClaw installation. It guides a new administrator through bridge discovery, ownership claiming, security configuration, and API key injection. Unlike the admin panel, which assumes an established admin session, the setup wizard operates during the lockdown-to-operational transition before any user exists.

### Key Components

- **BridgeDiscovery** (`src/components/BridgeDiscovery.tsx`) - UI for mDNS-based bridge discovery with manual IP fallback
- **bridgeApi** (`src/services/bridgeApi.ts`) - JSON-RPC client focused on lockdown and security methods
- **bridgeDiscovery** (`src/services/bridgeDiscovery.ts`) - Discovery client that probes the local network, tests connections, and scans common IP ranges

### Bridge Connection

Same pattern as the admin panel: JSON-RPC 2.0 over HTTP, proxied to the Bridge's Unix socket via `/api/*`. The wizard includes a dedicated discovery service that attempts mDNS first, then falls back to scanning common local IP ranges (192.168.1.x, 192.168.0.x, 10.0.0.x). It supports passphrase hashing (SHA-256 via Web Crypto API) and device fingerprint generation for the ownership claim flow.

## OpenClaw UI

| | |
|---|---|
| **Location** | `container/openclaw-src/ui/` |
| **Tech stack** | Canvas-host HTTP server (serves a local web interface), TypeScript, Vite, Vitest, Playwright (browser tests) |
| **Status** | In development |

### Purpose

OpenClaw UI is the control interface for the agent runtime. It provides a chat-based interaction surface for talking to agents, managing agent configurations, monitoring tool execution, reviewing channel integrations (Telegram, Discord, WhatsApp, Signal, Slack, iMessage, Nostr, Google Chat), and inspecting usage metrics. Unlike the admin panel, which manages the Bridge, OpenClaw UI manages the agents themselves.

### Key Capabilities

- **Chat interface** - Real-time conversation with agents, including markdown rendering and tool stream visualization
- **Agent management** - View and configure agent instances, skills, and tool configurations
- **Channel integrations** - Configure and monitor messaging platform connections (Telegram, Discord, WhatsApp, Signal, Slack, iMessage, Nostr, Google Chat)
- **Usage metrics** - Resource consumption tracking and cost analysis
- **Session management** - Active session monitoring and control
- **Cron scheduling** - Task scheduling interface
- **i18n** - Internationalization support for multiple locales
- **Theme support** - Dark/light theme with smooth transitions
- **Event log** - Real-time event streaming with log filtering

### Agent Runtime Connection

OpenClaw UI connects to the agent runtime via a WebSocket gateway (`GatewayBrowserClient` in `src/ui/gateway.ts`). The gateway protocol supports device identity authentication using Ed25519 key pairs (`@noble/ed25519`), with automatic token management and reconnection. The client operates in "webchat" mode with operator-level scopes. Messages flow as typed frames: request/response pairs for RPC calls, and event frames for real-time updates with sequence gap detection.

## Integration Points

All client applications connect back to the ArmorClaw control plane, but through different channels depending on their role:

```
┌──────────────────┐     JSON-RPC/HTTP      ┌─────────────┐
│   Admin Panel    │ ───────────────────────▶│             │
│   Setup Wizard   │     (via /api/* proxy)  │             │
└──────────────────┘                         │   Go Bridge │
                                              │ (Control    │
┌──────────────────┐     WebSocket           │  Plane)     │
│  OpenClaw UI     │ ───────────────────────▶│             │
│  (Agent Control) │   (Gateway protocol)    │             │
└──────────────────┘                         └──────┬──────┘
                                                     │
┌──────────────────┐     Matrix E2EE               │
│   ArmorChat      │ ──────────────────────────────│
│   ArmorTerminal  │     (via Conduit homeserver)   │
└──────┬───────────┘                                │
       │                                            ▼
       │  Blocker resolution:                 ┌──────────┐
       │  resolve_blocker RPC ──────────────▶ │  Agents  │
       │  (via Bridge HTTP API)               └──────────┘
       │    Orchestrator unblocks workflow
       │    Container retries
```

| Application | Transport | Protocol | Authentication |
|---|---|---|---|
| Admin Panel | HTTP (proxied to Unix socket) | JSON-RPC 2.0 | Session token (ownership claim) |
| Setup Wizard | HTTP (proxied to Unix socket) | JSON-RPC 2.0 | Lockdown challenge/response |
| ArmorTerminal | HTTP(S) + WebSocket | JSON-RPC 2.0 + Matrix | Device registration + certificate pinning |
| OpenClaw UI | WebSocket | Gateway protocol (v3) | Ed25519 device identity + token |
| ArmorChat | Matrix federation + Bridge HTTP RPC | Matrix protocol + JSON-RPC (resolve_blocker) | Matrix E2EE + biometric keystore |

### SDtW Multi-Platform Adapters

The SDtW (Slack, Discord, Teams, WhatsApp) adapters in `bridge/internal/sdtw/` provide a uniform adapter interface for external messaging platforms. Key implementations:

- **WhatsApp** (`whatsapp.go`): Uses WhatsApp Business Cloud API, authenticates with access token + phone number ID, supports text/template/image/document/audio/video messages, includes token-bucket rate limiter
- **Discord** (`discord.go`): Connects via bot token using Discord Gateway protocol, supports rich embeds, message references (threaded replies), guild-scoped operation, edit/delete capabilities
- **Teams** (`teams.go`): Authenticates via Azure AD (client ID, client secret, tenant ID), processes messages via webhook, supports OAuth token refresh, includes HMAC-SHA256 webhook verification
- **Slack** (`slack.go`): Webhook-based message delivery, handles URL verification challenge for event subscriptions

All adapters implement the `SDTWAdapter` interface from `adapter.go` with lifecycle (Initialize, Start, Shutdown), core messaging (SendMessage, ReceiveEvent), reactions, mutation (EditMessage, DeleteMessage), and health check methods. Each declares a `CapabilitySet` for feature discovery. Messages carry HMAC-SHA256 signatures for integrity.

Deployment Skills for AI CLI Tools are available in `.skills/` directory for deploying ArmorClaw via AI assistants (Claude Code, OpenCode, Cursor). Skills include Deploy, Status, Cloudflare, and Provision.
