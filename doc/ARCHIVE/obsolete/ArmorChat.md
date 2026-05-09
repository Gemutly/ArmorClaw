# ArmorChat Project Review

> **Last Updated:** 2026-04-28
> **Version:** 4.0.0-alpha01 (Governor Version)
> **Build Status:** ✅ ALL MODULES COMPILE
> **Deployment:** ✅ SUCCESS (Samsung Galaxy Note 20 Ultra - Android 13)
> **Architecture:** Android (Kotlin) + Jetpack Compose
> **Matrix Migration:** ✅ COMPLETE (See MATRIX_MIGRATION.md)
> **Discovery System:** ✅ ENHANCED (See Section 15)
> **Unified Theme:** ✅ ArmorColors Design System (See Section 3.3)
> **Governor Strategy:** ✅ COMPLETE (All 4 Phases - See Section 17)
> **New Features:** ✅ Cold Vault, Governor UI, Audit & Revocation, Provisioning System, Commercial Polish, SAS Verification State Machine, Trust Badges, Per-Message Encryption Indicators, Email Approval Screen, ToDevice Processing, ConfigRepository
> **Bug Fixes:** ✅ 13 Critical Bugs Fixed (8 on 2026-02-21, 5 on 2026-02-24)
> **Bucket 2:** ✅ COMPLETE (T1-T22 + F1-F4 verified: Matrix Trust, SAS State Machine, Color Migration (~240 hex→ArmorColors), Font Loading (Inter+JetBrains Mono), ConfigRepository, ToDeviceProcessor, Trust Badge, Per-Message Indicators, Email Approval, Crab Mascot, Hardening Test Plan)
> **Phases 2-5:** ✅ COMPLETE (Workflow UI Maturity, Security Hardening, Communication Robustness, UI Polish — see below)
> **Crypto Completion:** ✅ COMPLETE (Real E2EE via matrix-rust-sdk 26.2.16, Cross-Signing Bootstrap, SAS Verification, Sliding Sync MSC3575 Detection, SessionRefresh Stress Test, Accessibility CI Gate)

---

## Table of Contents

1. [Executive Summary](#1-executive-summary)
2. [Architecture Overview](#2-architecture-overview)
3. [Module Structure](#3-module-structure)
4. [Domain Layer](#4-domain-layer)
5. [Platform Layer](#5-platform-layer)
6. [Data Layer](#6-data-layer)
7. [Presentation Layer](#7-presentation-layer)
8. [Navigation System](#8-navigation-system)
9. [Security Architecture](#9-security-architecture)
10. [Communication Architecture](#10-communication-architecture)
11. [Control Plane & Agent System](#11-control-plane--agent-system)
12. [Feature Reference](#12-feature-reference)
13. [Build & Configuration](#13-build--configuration)
14. [Development Guidelines](#14-development-guidelines)
15. [ArmorChat ↔ ArmorClaw Communication](#15-armorchat--armorclaw-communication)
16. [Quick Reference](#16-quick-reference)
17. [Governor Strategy](#17-governor-strategy)
18. [Testing Strategy](#18-testing-strategy)

---

## 1. Executive Summary

ArmorChat is a **secure end-to-end encrypted chat application** built on the Matrix protocol. The project uses **Kotlin** with **Jetpack Compose** for the UI, following standard Android architecture.

### Key Characteristics

| Aspect | Description |
|--------|-------------|
| **Language** | Kotlin 1.9.20 |
| **Architecture** | Clean Architecture (Domain → Data → Presentation) |
| **UI Framework** | Jetpack Compose 1.5.0, Material 3 |
| **DI Framework** | Koin 3.5.0 |
| **Database** | SQLDelight 2.0.0 with SQLCipher |
| **Network** | Ktor 2.3.5 (WebSocket, HTTP) |
| **Async** | Kotlin Coroutines, Flow |
| **Messaging Protocol** | Matrix (via Matrix Rust SDK FFI) |
| **Admin Backend** | ArmorClaw Bridge (Go) via JSON-RPC 2.0 |

### Project Status

| Component | Status | Notes |
|-----------|--------|-------|
| App Module (main) | ✅ Compiles | All compilation errors fixed |
| App Module (tests) | ✅ Compiles | Tests updated for Matrix SDK |
| App Module (UI) | ✅ Compiles | All navigation routes working |
| ArmorColors Theme | ✅ Active | Unified theme system (teal/navy branding) |
| Bridge Server | ✅ Builds | Go backend operational |
| Matrix Migration | ✅ Complete | See `doc/MATRIX_MIGRATION.md` |
| Unified Branding | ✅ Complete (~240 colors migrated to ArmorColors tokens) | See `doc/styling.md` |
| Documentation | ✅ Complete | Comprehensive docs in `doc/` |
| **Device Deployment** | ✅ **Success** | Deployed to SM-N986U (Android 13) |

### Phases 2-5 Completion Status (2026-04-28)

| Component | Status | Notes |
|-----------|--------|-------|
| TestToolRegistry | ✅ Architecture Spec | 6 tools defined (`.sisyphus/plans/test-tool-registry-spec.md`) |
| Phase 1 Crypto | ✅ Architecture Doc | Ready for SDK (`doc/CRYPTO_ARCHITECTURE.md`) |
| ArmorColors Lint Rule | ✅ Integrated | `HardcodedColorLiteral` Detekt rule in `buildSrc/` |
| Debug TestHooks | ✅ Implemented | 5 ADB broadcast receivers + UiStateContentProvider |
| Workflow Timeline | ✅ Hardened | 200+ event handling, truncation, debounce, AnimatedContent transitions |
| Blocker Response Dialog | ✅ Hardened | Retry with exponential backoff (1s/2s/4s, max 3), field validation |
| Governance Banner | ✅ Enhanced | WorkflowStatus mapping, pulse animation on BLOCKED, recovery flash |
| Email Approval | ✅ Hardened | Expired/Timeout/AlreadyHandled/InvalidId states, 5s cold-start timeout |
| Security Audit | ✅ Complete | 7 data paths audited (`doc/SECURITY_AUDIT.md`) |
| Memory Zeroization | ✅ Audited + Gaps Fixed | BlockerResponseDialog uses MemoryWiper API; TODO comments on 4 ViewModels |
| Vault Resilience | ✅ Stress Tested | Concurrent access, DB lock contention, deadlock-free, fail-closed |
| Deep Link Validation | ✅ Hardened | DANGEROUS_CHARS + DANGEROUS_PATTERNS + null byte + fragment validation |
| Matrix Sync Recovery | ✅ Enhanced | Exponential backoff (1s→30s max), M_UNKNOWN_TOKEN→full resync, HTTP 4xx/5xx triggers |
| Offline Message Queue | ✅ New | SQLDelight-backed FIFO, drop-oldest-when-full, max 1000 entries, max 3 retries |
| Session Refresh | ✅ New | 5-min monitoring interval, 5-min expiry threshold, mutex-guarded refresh |
| ChatScreen Performance | ✅ Optimized | Memoized timestamp formatting, LazyColumn key optimization |
| Accessibility | ✅ WCAG 2.1 AA | Top 10 screens audited (`doc/ACCESSIBILITY_AUDIT.md`) |
| ArmorColors CI Gate | ✅ Active | Detekt runs on app module, zero violations |

### Latest Bug Fixes (2026-02-21)

| Bug # | Description | Status | Impact |
|-------|-------------|--------|--------|
| #1 | Hardcoded Production URLs | ✅ Fixed | Can now connect to any VPS |
| #2 | Well-Known Discovery Missing | ✅ Fixed | Auto-configuration works |
| #3 | java.net.URLDecoder in Common | ✅ Fixed | iOS compatibility restored |
| #4 | Session Never Expires | ✅ Fixed | Proper session management |
| #5 | MatrixSyncManager Not Injected | ✅ Fixed | Real-time events work |
| #6 | Encryption Undocumented | ✅ Fixed | Trust model documented |
| #7 | deriveBridgeUrl Insufficient | ✅ Fixed | Better URL handling |

### Bug Fixes (2026-02-24) — ArmorClaw Alignment Review

| Bug # | Description | Status | Impact |
|-------|-------------|--------|--------|
| #8 | Push RPC method names wrong (`push.register`/`push.unregister` → `push.register_token`/`push.unregister_token`) | ✅ Fixed | Push notifications would fail with -32601 |
| #9 | Matrix RPC method names wrong (`matrix.invite` → `matrix.invite_user`, `matrix.typing` → `matrix.send_typing`, `matrix.read_receipt` → `matrix.send_read_receipt`) | ✅ Fixed | Deprecated Matrix RPC calls would fail |
| #10 | WebSocket failure blocks setup (`SetupService.connectWithCredentials()` threw fatal error on WS failure) | ✅ Fixed | Setup now succeeds even without WS |
| #11 | `BridgeStatusResponse.userRole` comment said lowercase values but `AdminLevel` enum expects uppercase | ✅ Fixed | Developer confusion during integration |
| #12 | `doc/ArmorClaw.md` listed `provisioning.rotate_secret` instead of `provisioning.rotate` | ✅ Fixed | Documentation mismatch |

### Self-Hosted VPS Support

The app now supports connecting to any Matrix homeserver:

```kotlin
// Runtime configuration
BridgeConfig.setRuntimeConfig(
    BridgeConfig.createCustom(
        bridgeUrl = "https://your-vps.com:8080",
        homeserverUrl = "https://your-vps.com:8008",
        serverName = "My VPS"
    )
)
```

Or via Well-Known Discovery:
```json
// https://your-vps.com/.well-known/matrix/client
{
  "m.homeserver": { "base_url": "https://matrix.your-vps.com" },
  "com.armorclaw": {
    "bridge_url": "https://bridge.your-vps.com",
    "server_name": "My VPS"
  }
}
```

### Migration Status (Matrix Protocol)

ArmorChat has completed the migration from custom JSON-RPC to **proper Matrix Protocol**:

| Aspect | Status | Implementation |
|--------|--------|----------------|
| Messaging | ✅ Matrix SDK | `MatrixClient` interface (40+ methods) |
| Real-time Events | ✅ Matrix /sync | `MatrixSyncManager` long-poll |
| E2E Encryption | ✅ Client Keys | libolm/libvodozemac via SDK |
| Admin Operations | ✅ Bridge RPC | `BridgeAdminClient` (22 methods) |
| Control Plane | ✅ Matrix Events | `ControlPlaneStore` processor |
| Session Storage | ✅ Encrypted | AES256-GCM + Android Keystore |
| Session Expiration | ✅ Proper | `MatrixSession.expiresAt` handling |

---

## 2. Architecture Overview

ArmorChat follows **Clean Architecture** with clear separation of concerns across multiple layers:

```
┌─────────────────────────────────────────────────────────────────────────┐
│                     PRESENTATION LAYER (Compose UI)                      │
│                                                                          │
│   Screens │ Components │ ViewModels │ Navigation (58 routes)            │
│   Location: app/src/main/java/app/armorclaw/                        │
└─────────────────────────────────────────────────────────────────────────┘
                                    │
                                    ▼
┌─────────────────────────────────────────────────────────────────────────┐
│                           DOMAIN LAYER                                   │
│                                                                          │
│   Models │ Repository Interfaces │ Use Cases                            │
│   Location: app/src/main/java/app/armorclaw/domain/                  │
└─────────────────────────────────────────────────────────────────────────┘
                                    │
                                    ▼
┌─────────────────────────────────────────────────────────────────────────┐
│                            DATA LAYER                                    │
│                                                                          │
│   Repository Implementations │ DAOs │ Stores │ Offline Queue            │
│   Location: app/src/main/java/app/armorclaw/data/                    │
│            app/src/main/java/app/armorclaw/network/                  │
└─────────────────────────────────────────────────────────────────────────┘
                                    │
                                    ▼
┌─────────────────────────────────────────────────────────────────────────┐
│                       PLATFORM/SDK LAYER                                 │
│                                                                          │
│   ┌─────────────────┐    ┌─────────────────┐    ┌─────────────────┐     │
│   │  MatrixClient   │    │ BridgeRpcClient │    │  Platform SVCS  │     │
│   │  (Messaging)    │    │ (Admin Ops)     │    │  (Biometric,    │     │
│   │                 │    │                 │    │   Clipboard,    │     │
│   │  Matrix Rust    │    │  JSON-RPC 2.0   │    │   Network)      │     │
│   │  SDK FFI        │    │  WebSocket      │    │  Android SVCS   │     │
│   └────────┬────────┘    └────────┬────────┘    └─────────────────┘     │
│            │                      │                                      │
│   Location: app/src/main/java/app/armorclaw/platform/                │
└─────────────────────────────────────────────────────────────────────────┘
                                    │
                                    ▼
┌─────────────────────────────────────────────────────────────────────────┐
│                         EXTERNAL SERVICES                                │
│                                                                          │
│   ┌─────────────────┐    ┌─────────────────┐    ┌─────────────────┐     │
│   │ Matrix Server   │    │ ArmorClaw Bridge│    │ Firebase (FCM)  │     │
│   │ (Conduit)       │    │ (Go Backend)    │    │ Push Notifs     │     │
│   │ :8008 (HTTPS)   │    │ :8080 (HTTP/WS) │    │                 │     │
│   └─────────────────┘    └─────────────────┘    └─────────────────┘     │
└─────────────────────────────────────────────────────────────────────────┘
```

### Architecture Patterns

| Pattern | Implementation | Location |
|---------|---------------|----------|
| **Clean Architecture** | Domain → Data → Presentation layers | All modules |
| **MVVM** | ViewModels expose StateFlow, UI via collectAsState() | `viewmodels/` |
| **Repository Pattern** | Interfaces in domain/repository, impl in data/repository | `repository/` |
| **Dependency Injection** | Koin modules for service registration | `di/` |
| **Atomic Design** | UI components: atom → molecule → organism | `ui/components/` |
| **Use Case Pattern** | Single responsibility use case classes | `domain/usecase/` |

---

## 3. Module Structure

### Root Directory

```
ArmorChat/
├── app/                        # Android application module
│   └── src/
│       ├── main/java/app/armorclaw/
│       ├── androidTest/
│       └── test/
├── vodozemac/                  # Rust Matrix E2EE library (native FFI)
├── doc/                        # Documentation
├── gradle/                     # Gradle wrapper
├── build.gradle.kts            # Root build configuration
├── settings.gradle.kts         # Project settings
└── gradle.properties           # Gradle properties
```

### App Module (`app/`)

The app module contains all application code (domain, data, platform, and UI layers):

```
app/src/
├── main/java/app/armorclaw/
│   ├── config/                # App Configuration
│   │   ├── BridgeTrustStore.kt       # TLS trust store
│   │   ├── ConfigManager.kt          # Runtime config management
│   │   └── SignedConfigParser.kt     # Signed QR/deep-link config parsing
│   │
│   ├── crypto/                # Cryptography
│   │   ├── CryptoService.kt          # Crypto service interface
│   │   ├── MatrixOlmService.kt       # Olm/Megolm operations
│   │   └── VodozemacNative.kt        # Native vodozemac FFI bridge
│   │
│   ├── data/
│   │   ├── local/entity/             # Room database entities
│   │   │   └── Entities.kt           # Database entity definitions
│   │   ├── model/                    # Data models
│   │   │   ├── EmailApprovalEvent.kt # Email approval event types
│   │   │   └── SystemAlert.kt        # System alert types
│   │   └── repository/              # Repository implementations
│   │       ├── BridgeCapabilities.kt # Bridge capability detection
│   │       ├── BridgeRepository.kt   # Bridge RPC facade
│   │       └── UserRepository.kt     # User data operations
│   │
│   ├── domain/                # Domain Layer
│   │   ├── model/            # Domain models (12 files)
│   │   │   ├── AppResult.kt          # Result wrapper
│   │   │   ├── ArmorClawErrorCode.kt # Error codes (100+ codes)
│   │   │   ├── Call.kt              # Call state models
│   │   │   ├── Message.kt           # Basic message model
│   │   │   ├── Notification.kt      # Notification model
│   │   │   ├── OperationContext.kt  # Operation context
│   │   │   ├── Room.kt              # Room model
│   │   │   ├── SyncState.kt         # Sync state enum
│   │   │   ├── SystemAlert.kt       # System alert types
│   │   │   ├── Trust.kt             # Trust level models
│   │   │   ├── UnifiedMessage.kt    # Unified message model
│   │   │   └── User.kt              # User model
│   │   ├── repository/       # Repository interfaces (11 files)
│   │   │   ├── AgentRepository.kt
│   │   │   ├── AuthRepository.kt
│   │   │   ├── CallRepository.kt
│   │   │   ├── MessageRepository.kt
│   │   │   ├── NotificationRepository.kt
│   │   │   ├── RoomRepository.kt
│   │   │   ├── SyncRepository.kt
│   │   │   ├── ThreadRepository.kt
│   │   │   ├── UserRepository.kt
│   │   │   ├── VerificationRepository.kt
│   │   │   └── WorkflowRepository.kt
│   │   └── usecase/          # Use case interfaces
│   │
│   ├── navigation/            # Navigation
│   │   ├── ArmorClawNavHost.kt       # NavHost with all routes
│   │   ├── DeepLinkHandler.kt        # Deep link parsing and routing
│   │   └── Route.kt                  # Route definitions
│   │
│   ├── network/               # Network Layer
│   │   ├── BridgeApi.kt              # Bridge API interface
│   │   ├── BridgeDiscovery.kt        # Server discovery
│   │   ├── NetworkResilience.kt      # Retry, backoff, circuit breaker
│   │   └── ResilientWebSocket.kt     # WebSocket with auto-reconnect
│   │
│   ├── platform/             # Platform Layer
│   │   ├── bridge/           # Bridge client (12 files)
│   │   │   ├── BridgeAdminClient.kt      # Admin operations (22 methods)
│   │   │   ├── BridgeAdminClientImpl.kt
│   │   │   ├── BridgeClientFactory.kt
│   │   │   ├── BridgeEvent.kt            # Event types (16 events)
│   │   │   ├── BridgeRepository.kt       # Repository facade
│   │   │   ├── BridgeRpcClient.kt        # RPC interface (50+ methods)
│   │   │   ├── BridgeRpcClientImpl.kt    # RPC implementation
│   │   │   ├── BridgeWebSocketClient.kt  # WebSocket interface
│   │   │   ├── BridgeWebSocketClientImpl.kt
│   │   │   ├── InviteService.kt          # Room invite handling
│   │   │   ├── RpcModels.kt              # RPC data models
│   │   │   └── SetupService.kt           # Setup flow service
│   │   ├── biometric/        # Biometric auth
│   │   ├── clipboard/        # Secure clipboard
│   │   ├── encryption/       # Encryption services
│   │   ├── error/            # Error handling
│   │   ├── logging/          # Logging (AppLogger, LoggerDelegate)
│   │   ├── matrix/           # Matrix SDK integration
│   │   │   ├── event/        # Matrix event types
│   │   │   ├── MatrixClient.kt          # Matrix client interface (40+ methods)
│   │   │   ├── MatrixOlmService.kt      # Olm/Megolm operations
│   │   │   ├── MatrixSyncManager.kt     # Sync with exponential backoff recovery
│   │   │   ├── MatrixSessionStorage.kt  # Session persistence
│   │   │   └── SessionRefreshManager.kt # Proactive token refresh (Phases 2-5)
│   │   ├── network/          # Network monitoring
│   │   ├── notification/     # Notification interface
│   │   └── voice/            # Voice input/recording
│   │
│   ├── push/                  # Push Notifications
│   │   ├── ArmorClawMessagingService.kt  # FCM messaging service
│   │   ├── MatrixPusherManager.kt        # Matrix push registration
│   │   ├── NotificationHelper.kt         # Notification builder
│   │   └── PushTokenManager.kt           # Token lifecycle
│   │
│   ├── repository/            # Additional repositories
│   │   └── SetupRepository.kt        # Setup flow state
│   │
│   ├── ui/                   # UI Layer (Jetpack Compose)
│   │   ├── account/          # AccountDeletionScreen.kt
│   │   ├── agent/            # AgentScreen.kt
│   │   ├── approval/         # ApprovalScreen.kt
│   │   ├── components/       # Shared UI components (14 files)
│   │   │   ├── BlindFillCard.kt
│   │   │   ├── BlockerResponseDialog.kt
│   │   │   ├── EmailApprovalCard.kt
│   │   │   ├── GovernanceBanner.kt
│   │   │   ├── WorkflowTimeline.kt
│   │   │   └── ... (more)
│   │   ├── email/            # EmailApprovalScreen.kt
│   │   ├── home/             # HomeScreen.kt
│   │   ├── migration/        # MigrationScreen.kt
│   │   ├── security/         # Security screens (7 files)
│   │   │   ├── BiometricEnableScreen.kt
│   │   │   ├── BondingScreen.kt
│   │   │   ├── KeyBackupScreen.kt
│   │   │   ├── KeyRecoveryScreen.kt
│   │   │   ├── PasswordRotationScreen.kt
│   │   │   ├── SecretsScreen.kt
│   │   │   └── SecurityConfigScreen.kt
│   │   ├── verification/     # BridgeVerificationScreen.kt
│   │   ├── workflow/         # WorkflowScreen.kt
│   │   └── theme/            # ArmorColors design system
│   │       ├── ArmorColors.kt        # Color tokens (teal/navy)
│   │       ├── ArmorTypography.kt    # Inter + JetBrains Mono
│   │       ├── ArmorShapes.kt        # Shape definitions
│   │       └── ArmorTheme.kt         # Theme composable
│   │
│   ├── utils/                 # Utilities
│   │   └── ErrorHandler.kt
│   │
│   ├── validation/            # Validation
│   │   └── ValidationReceiver.kt
│   │
│   ├── viewmodel/             # ViewModels (6 files)
│   │   ├── AgentManagementVM.kt
│   │   ├── BondingVM.kt
│   │   ├── HardeningWizardVM.kt
│   │   ├── HitlVM.kt
│   │   ├── SecurityConfigVM.kt
│   │   └── WorkflowVM.kt
│   │
│   └── MainActivity.kt        # Main activity
│
├── androidTest/               # Instrumented tests
│   └── FFIBoundaryTest.kt
│
└── test/                      # Unit tests (8 files)
```

### Vodozemac (`vodozemac/`)

The `vodozemac/` directory contains the Rust Matrix E2EE library compiled as a native library for Android FFI:

```
vodozemac/
├── Cargo.toml                # Rust crate definition
├── src/                      # Rust source (libolm-compatible API)
└── target/                   # Build artifacts (`.so` files for ARM64/x86)
```

This is consumed by `app/src/main/java/app/armorclaw/crypto/VodozemacNative.kt` via JNI.

### ArmorColors Design System

The ArmorColors theme is defined within the app module (not a separate module). It provides a **unified theme** for ArmorClaw applications:

```
app/src/main/java/app/armorclaw/ui/theme/
├── ArmorColors.kt        # Color palette, ArmorColors object, Bridge platform brand colors
├── ArmorTypography.kt    # Typography (Inter, JetBrains Mono)
├── ArmorShapes.kt        # Shape definitions
├── ArmorTheme.kt         # Theme wrapper composable
└── GlowModifiers.kt      # Teal glow effect modifiers (if present)
```

**Brand Colors:**
| Color | Hex | Usage |
|-------|-----|-------|
| Teal (Primary) | `#14F0C8` | Primary actions, accents |
| Teal Glow | `#67F5D8` | Hover states, glows |
| Navy (Background) | `#0A1428` | Dark background |
| Precision Blue | `#0EA5E9` | Secondary actions |
| Success Green | `#22C55E` | Success states |
| Warning Amber | `#F59E0B` | Warning states |
| Error Red | | Error, compromised trust states |
| Info Blue | | Verified-in-person trust states |

**Design Principles:**
- Dark mode only (default experience)
- High contrast (≥4.5:1 ratio)
- Sparse mascot usage (splash, empty states, about screen)
- Unified typography across platforms
- 100% ArmorColors token migration (~240 hardcoded hex values replaced across vault, governor, audit, app modules)
- Fonts loaded via Android resources: Inter (body text) + JetBrains Mono (code, terminal)

---

## 4. Domain Layer

The domain layer contains business logic that is platform-agnostic.

### Domain Models

#### UnifiedMessage

The `UnifiedMessage` is the primary message model that supports both regular Matrix messages and agent/system messages:

```kotlin
sealed class UnifiedMessage {
    abstract val id: String
    abstract val roomId: String
    abstract val timestamp: Instant
    abstract val sender: MessageSender

    data class Regular(...) : UnifiedMessage()   // User messages
    data class Agent(...) : UnifiedMessage()     // AI assistant messages
    data class System(...) : UnifiedMessage()    // System notifications
    data class Command(...) : UnifiedMessage()   // User commands to agent
}
```

Message senders are also typed:

```kotlin
sealed class MessageSender {
    data class UserSender(...) : MessageSender()    // Regular users
    data class AgentSender(...) : MessageSender()   // AI agents
    data class SystemSender(...) : MessageSender()  // System messages
}
```

#### Other Domain Models

| Model | File | Purpose |
|-------|------|---------|
| `User` | User.kt | User profile information |
| `Room` | Room.kt | Chat room with metadata |
| `Message` | Message.kt | Basic message structure |
| `Call` | Call.kt | Call session and state |
| `Trust` | Trust.kt | Trust level for users/devices |
| `SyncState` | SyncState.kt | Synchronization state |
| `SystemAlert` | SystemAlert.kt | System alert types |
| `AppResult` | AppResult.kt | Result wrapper for operations |
| `ArmorClawErrorCode` | ArmorClawErrorCode.kt | 100+ error codes |

### Repository Interfaces

All repository interfaces are defined in `domain/repository/`:

| Repository | Purpose | Key Methods |
|------------|---------|-------------|
| `AuthRepository` | Authentication | `login()`, `logout()`, `isLoggedIn()` |
| `MessageRepository` | Messages | `getMessages()`, `sendMessage()`, `sendReaction()` |
| `RoomRepository` | Rooms | `getRooms()`, `createRoom()`, `joinRoom()` |
| `UserRepository` | Users | `getUser()`, `updateProfile()` |
| `CallRepository` | Calls | `startCall()`, `endCall()`, `toggleMute()` |
| `ThreadRepository` | Threads | `getThread()`, `sendReply()` |
| `VerificationRepository` | Verification | `requestVerification()`, `confirmVerification()` |
| `WorkflowRepository` | Workflows | `startWorkflow()`, `updateStep()` |
| `AgentRepository` | Agents | `taskStarted()`, `taskCompleted()` |
| `SyncRepository` | Sync | `sync()`, `getSyncState()` |
| `NotificationRepository` | Notifications | `registerToken()`, `updateSettings()` |
| `ConfigRepository` | Server Config | `getServerConfig()`, `saveServerConfig()`, `getProvisioningStatus()`, `isServerSigned()`, `getConnectionProfile()` |

---

## 5. Platform Layer

The platform layer provides abstractions for platform-specific functionality using standard Android services.

### MatrixClient Interface

The `MatrixClient` is the primary interface for all Matrix operations:

```kotlin
interface MatrixClient {
    // Connection State
    val syncState: StateFlow<SyncState>
    val isLoggedIn: StateFlow<Boolean>
    val currentUser: StateFlow<User?>
    val connectionState: StateFlow<ConnectionState>

    // Authentication
    suspend fun login(homeserver: String, username: String, password: String, deviceId: String?): Result<MatrixSession>
    suspend fun logout(): Result<Unit>
    suspend fun restoreSession(session: MatrixSession): Result<Unit>

    // Sync
    fun startSync()
    fun stopSync()
    suspend fun syncOnce(): Result<Unit>

    // Rooms
    val rooms: StateFlow<List<Room>>
    suspend fun createRoom(...): Result<Room>
    suspend fun joinRoom(roomIdOrAlias: String): Result<Room>
    suspend fun leaveRoom(roomId: String): Result<Unit>

    // Messages
    suspend fun getMessages(roomId: String, limit: Int, fromToken: String?): Result<MessageBatch>
    suspend fun sendTextMessage(roomId: String, text: String): Result<String>
    suspend fun sendReaction(roomId: String, eventId: String, key: String): Result<String>

    // Events
    fun observeEvents(): Flow<MatrixEvent>
    fun observeRoomEvents(roomId: String): Flow<MatrixEvent>
    fun observeArmorClawEvents(roomId: String?): Flow<MatrixEvent>

    // Presence, Typing, Read Receipts, Encryption, etc.
    // ... (40+ methods total)
}
```

### Matrix Rust SDK Integration (Crypto Completion)

The project integrates `org.matrix.rustcomponents:sdk-android:26.2.16` and `crypto-android:26.1.28` from Maven Central for real E2EE operations:

| Component | File | Purpose |
|-----------|------|---------|
| `RustMatrixClientFactory` | `app/src/main/java/app/armorclaw/matrix/RustMatrixClientFactory.kt` | Creates Rust SDK `Client` via `ClientBuilder` |
| `MatrixClientImpl` | `app/src/main/java/app/armorclaw/matrix/MatrixClientImpl.kt` | `sendTextMessage()` uses Rust `Timeline.send()` for encrypted rooms; `getRoomEncryptionStatus()` uses `Room.encryptionState()` |
| `ToDeviceProcessorImpl` | `app/src/main/java/app/armorclaw/matrix/ToDeviceProcessorImpl.kt` | Routes `to_device` events to Rust SDK (replaces stub) |
| `VerificationRepositoryImpl` | `data/repository/VerificationRepositoryImpl.kt` | Cross-signing bootstrap, SAS verification, key backup status via Rust `Encryption` API |
| `SlidingSyncCapability` | `app/src/main/java/app/armorclaw/matrix/SlidingSyncCapability.kt` | MSC3575 detection via `.well-known` and endpoint probing |
| `SessionRefreshManager` | `app/src/main/java/app/armorclaw/matrix/SessionRefreshManager.kt` | Token refresh with Mutex serialization (stress-tested: 50 concurrent) |

### BridgeRpcClient Interface

The `BridgeRpcClient` handles admin operations via JSON-RPC 2.0:

```kotlin
interface BridgeRpcClient {
    // Bridge Lifecycle
    suspend fun bridgeStart(...): Result<BridgeStartResponse>
    suspend fun bridgeStop(): Result<Unit>
    suspend fun bridgeStatus(): Result<BridgeStatusResponse>
    suspend fun healthCheck(): Result<HealthCheckResponse>

    // Platform Integration
    suspend fun platformConnect(...): Result<PlatformConnectResponse>
    suspend fun platformDisconnect(...): Result<Unit>
    suspend fun platformList(): Result<List<PlatformInfo>>

    // Push Notifications
    suspend fun pushRegister(...): Result<Unit>
    suspend fun pushUnregister(...): Result<Unit>

    // License & Compliance
    suspend fun licenseStatus(): Result<LicenseStatusResponse>
    suspend fun complianceStatus(): Result<ComplianceStatusResponse>

    // Recovery
    suspend fun recoveryGeneratePhrase(): Result<RecoveryPhraseResponse>
    suspend fun recoveryVerify(...): Result<VerificationResponse>

    // Error Management
    suspend fun getErrors(): Result<List<ErrorInfo>>
    suspend fun resolveError(...): Result<Unit>

    // Agent Management (NEW in v3.3.0)
    suspend fun agentList(): Result<AgentListResponse>
    suspend fun agentStatus(agentId: String): Result<AgentStatusResponse>
    suspend fun agentStop(agentId: String): Result<Unit>

    // Workflow (NEW in v3.3.0)
    suspend fun workflowTemplates(): Result<WorkflowTemplatesResponse>
    suspend fun workflowStart(...): Result<WorkflowStartResponse>
    suspend fun workflowStatus(workflowId: String): Result<WorkflowStatusResponse>

    // HITL (NEW in v3.3.0)
    suspend fun hitlPending(): Result<HitlPendingResponse>
    suspend fun hitlApprove(gateId: String): Result<Unit>
    suspend fun hitlReject(gateId: String, reason: String?): Result<Unit>

    // Budget (NEW in v3.3.0)
    suspend fun budgetStatus(): Result<BudgetStatusResponse>

    // Provisioning (NEW - ArmorClaw Bridge Admin)
    suspend fun provisioningClaim(claimToken: String): Result<ProvisioningClaimResponse>
    suspend fun provisioningRotate(secretType: String): Result<ProvisioningRotateResponse>
    suspend fun provisioningRevoke(targetUserId: String): Result<ProvisioningRevokeResponse>
    suspend fun provisioningGetConfig(): Result<ProvisioningConfigResponse>
    suspend fun provisioningSetConfig(config: Map<String, Any>): Result<ProvisioningSetConfigResponse>

    // Agent Intervention (NEW in v4.1.1)
    suspend fun reportAgentIntervention(request: ReportInterventionRequest): Result<ReportInterventionResponse>
    suspend fun resolveBlocker(request: ResolveBlockerRequest): Result<ResolveBlockerResponse>

    // Total: 57+ methods
}
```

### Platform Services (Interfaces & Implementations)

| Service | Interface Location | Implementation Location | Purpose |
|---------|-----------------|-----------------|---------|
| `BiometricAuth` | `app/.../platform/biometric/` | `app/.../platform/` | Fingerprint/FaceID |
| `SecureClipboard` | `app/.../platform/clipboard/` | `app/.../platform/` | Encrypted clipboard |
| `NotificationManager` | `app/.../platform/notification/` | `app/.../platform/` | Push notifications |
| `NetworkMonitor` | `app/.../platform/network/` | `app/.../platform/` | Connectivity |
| `VoiceRecorder` | `app/.../platform/voice/` | `app/.../platform/` | Voice messages |

---

## 6. Data Layer

The data layer handles persistence and data access.

### ControlPlaneStore

The `ControlPlaneStore` processes ArmorClaw-specific Matrix events:

```kotlin
class ControlPlaneStore(
    private val matrixClient: MatrixClient,
    private val workflowRepository: WorkflowRepository,
    private val agentRepository: AgentRepository
) {
    // State Flows
    val activeWorkflows: StateFlow<List<WorkflowState>>
    val agentTasks: StateFlow<List<AgentTaskState>>
    val thinkingAgents: StateFlow<Map<String, AgentThinkingState>>
    val budgetWarnings: StateFlow<List<BudgetWarningState>>
    val bridgeStatus: StateFlow<Map<String, BridgeStatusState>>

    // Event Processing
    fun subscribeToRoom(roomId: String)
    private suspend fun processEvent(event: MatrixEvent)
}
```

### Database

The app uses **SQLDelight** with **SQLCipher** for encrypted storage:

```
app/src/main/sqldelight/
└── app/armorclaw/
    └── ArmorClawDatabase.sq   # Schema definitions
```

Key tables:
- Messages (with sync state, expiration)
- Rooms (with metadata, statistics)
- SyncQueue (pending operations)
- Users (profiles, presence)

### Offline Sync

The offline sync system includes:

| Component | Purpose |
|-----------|---------|
| `OfflineQueue` | Queue pending operations with priorities |
| `SyncEngine` | Execute operations, detect conflicts |
| `ConflictResolver` | Resolve sync conflicts |
| `BackgroundSyncWorker` | WorkManager for periodic sync |
| `OfflineMessageQueue` | SQLDelight-backed FIFO for Matrix write operations (Phases 2-5) |
| `OfflineMessageQueueImpl` | Android implementation with drop-oldest-when-full eviction |

---

## 7. Presentation Layer

The presentation layer uses **Jetpack Compose** with **MVVM** pattern.

### ViewModels

Key ViewModels and their responsibilities:

| ViewModel | Screen | Key State |
|-----------|--------|-----------|
| `ChatViewModel` | Chat | Messages, input, typing, call state |
| `HomeViewModel` | Home | Room list, filters, sync status |
| `ProfileViewModel` | Profile | User data, edit mode |
| `SettingsViewModel` | Settings | Settings state, logout |
| `SetupViewModel` | Connect | Server connection, credentials |
| `EmailApprovalViewModel` | Email Approval | Expired/Timeout/AlreadyHandled/InvalidId states (Phases 2-5) |
| `SplashViewModel` | Splash | Auth state, navigation decision |
| `DeviceListViewModel` | Devices | Device list, verification state |
| `SyncStatusViewModel` | (Various) | Sync progress, errors |
| `VerificationViewModel` | Verification | Device verification state, NotImplementedError handling |

### State Management Pattern

```kotlin
// ViewModel exposes StateFlow
class ChatViewModel(...) : ViewModel() {
    private val _uiState = MutableStateFlow(ChatUiState())
    val uiState: StateFlow<ChatState> = _uiState.asStateFlow()

    private val _events = Channel<ChatEvent>()
    val events: Flow<ChatEvent> = _events.receiveAsFlow()

    fun onAction(action: ChatAction) { ... }
}

// Screen collects state
@Composable
fun ChatScreen(viewModel: ChatViewModel) {
    val state by viewModel.uiState.collectAsState()

    LaunchedEffect(Unit) {
        viewModel.events.collect { event ->
            when (event) { ... }
        }
    }

    // UI based on state
}
```

### UI Component Hierarchy

Components follow **Atomic Design**:

```
Screen
└── Organism Components (e.g., MessageList, RoomList)
    └── Molecule Components (e.g., MessageBubble, RoomItem)
        └── Atom Components (e.g., Button, InputField, Avatar)
```

---

## 8. Navigation System

### Route Definitions

All 47 routes are defined in `AppNavigation.kt`:

```kotlin
object AppNavigation {
    // Core
    const val SPLASH = "splash"
    const val HOME = "home"

    // Onboarding
    const val WELCOME = "welcome"
    const val SECURITY = "security"
    const val CONNECT = "connect"
    const val PERMISSIONS = "permissions"
    const val COMPLETION = "completion"

    // Agent & Workflow (NEW in v3.3.0)
    const val AGENT_MANAGEMENT = "settings/agents"
    const val HITL_APPROVALS = "settings/approvals"
    const val WORKFLOW_MANAGEMENT = "settings/workflows"
    const val BUDGET_STATUS = "settings/budget"
    const val TUTORIAL = "tutorial"

    // Auth
    const val LOGIN = "login"
    const val REGISTRATION = "registration"
    const val FORGOT_PASSWORD = "forgot_password"

    // Main
    const val CHAT = "chat/{roomId}"
    const val PROFILE = "profile"
    const val SETTINGS = "settings"
    const val SEARCH = "search"

    // Room
    const val ROOM_MANAGEMENT = "room_management"
    const val ROOM_DETAILS = "room_details/{roomId}"
    const val ROOM_SETTINGS = "room_settings/{roomId}"

    // Profile Options
    const val CHANGE_PASSWORD = "change_password"
    const val CHANGE_PHONE = "change_phone"
    const val EDIT_BIO = "edit_bio"
    const val DELETE_ACCOUNT = "delete_account"

    // Settings Options
    const val SECURITY_SETTINGS = "security_settings"
    const val NOTIFICATION_SETTINGS = "notification_settings"
    const val APPEARANCE = "appearance"
    const val PRIVACY_POLICY = "privacy_policy"
    const val MY_DATA = "my_data"
    const val DATA_SAFETY = "data_safety"
    const val ABOUT = "about"
    const val REPORT_BUG = "report_bug"
    const val LICENSES = "licenses"
    const val TERMS_OF_SERVICE = "terms"

    // Devices & Verification
    const val DEVICES = "devices"
    const val ADD_DEVICE = "add_device"
    const val EMOJI_VERIFICATION = "verification/{deviceId}"

    // Calls
    const val ACTIVE_CALL = "call/{callId}"
    const val INCOMING_CALL = "incoming_call/{callId}/{callerId}/{callerName}/{callType}"

    // Threads & Media
    const val THREAD = "thread/{roomId}/{rootMessageId}"
    const val IMAGE_VIEWER = "image/{imageId}"
    const val FILE_PREVIEW = "file/{fileId}"

    // User Profile
    const val USER_PROFILE = "user/{userId}"
    const val SHARED_ROOMS = "shared_rooms/{userId}"

    // Helper functions for parameterized routes
    fun createChatRoute(roomId: String): String
    fun createCallRoute(callId: String): String
    fun createUserProfileRoute(userId: String): String
    // ... etc
}
```

### Navigation Flows

#### Onboarding Flow
```
Splash → Welcome → Security/{0-3} → Connect → Permissions → Completion → [Tutorial] → Home
                      │
                      └── (Skip) → Home
```

#### Authentication Flow
```
Splash → Login → [Forgot Password | Registration] → Home
```

#### Chat Flow
```
Home → Chat → [Room Details → Room Settings]
           → [User Profile → DM Chat | Call | Shared Rooms]
           → [Thread]
           → [Call]
           → [Image Viewer | File Preview]
```

#### Settings Flow
```
Home → Settings → [Profile]
                → [Security Settings → Devices → Verification]
                → [Notification Settings]
                → [Appearance]
                → [Privacy Policy | My Data | Data Safety]
                → [About → Licenses | Terms]
                → [Report Bug]
                → [Logout → Login]
```

---

## 9. Security Architecture

### Security Layers

```
┌─────────────────────────────────────────────────────────────────────────┐
│                     SECURITY LAYERS                                      │
├─────────────────────────────────────────────────────────────────────────┤
│  Layer 1: Transport Security (TLS 1.3 + Certificate Pinning)            │
│  Layer 2: Message Encryption (AES-256-GCM via Matrix E2EE)              │
│  Layer 3: Database Encryption (SQLCipher 256-bit)                       │
│  Layer 4: Key Storage (AndroidKeyStore)                                 │
│  Layer 5: Biometric Authentication                                      │
│  Layer 6: Deep Link Validation                                          │
└─────────────────────────────────────────────────────────────────────────┘
```

### Encryption Trust Model

| Model | Trust Assumption | Key Management | Status |
|-------|-----------------|----------------|--------|
| **CLIENT_SIDE** | Trust only self | Client manages keys via Matrix Rust SDK (Megolm v1) | ✅ Active (Complete) |
| **SERVER_SIDE** | Trust Bridge Server | Server manages E2EE keys | ❌ Deprecated (removed v4.1.0-alpha01) |
| **NONE** | No encryption | N/A | ⚠️ Dev only |

#### SERVER_SIDE Mode — Removed

`EncryptionService` was removed in v4.1.0-alpha01. The `EncryptionTypes.kt` file (containing `EncryptionMode` and `DeviceVerificationStatus` enums) was **fully deleted** in v4.1.1-alpha05 — both enums had zero runtime consumers.

#### CLIENT_SIDE Mode — Active (Complete)

Architecture declares CLIENT_SIDE as the active encryption mode (comments: "Active — Matrix Rust SDK"). Room creation correctly configures `m.megolm.v1.aes-sha2` encryption algorithm via Matrix SDK.

**What works:**
- Room creation configures Megolm v1 (AES-SHA2) encryption
- `EncryptionMode` enum deleted in v4.1.1-alpha05 (zero consumers); `RoomEncryptionStatus` sealed class in `MatrixClient.kt` is the active model
- Key storage via AndroidKeyStore
- Architecture wired for client-side encryption path

**What's incomplete:**
- Cross-signing and device verification workflows not yet implemented (pending matrix-rust-sdk)
- `requestVerification()` delegates to VerificationRepository scaffold (emoji state machine works, real crypto pending)
- `isRoomEncrypted()` now correctly detects encryption on joined rooms (Bucket 2 fix)
- `getRoomEncryptionStatus()` returns real status via `RoomEncryptionStatus` → `EncryptionStatus` mapper (Bucket 2)
- `isBridgeVerified()` scaffold added (TEMPORARY — bridge health ≠ crypto trust)

**Trust Badge UI (Bucket 2):**
- `TrustBadge` composable renders trust level with animated colors, sized variants (SM/MD/LG)
- `TrustIndicator` compact dot for list items
- `TrustStatusCard` expandable card with device name, last verified timestamp, action button
- Five trust levels: `UNVERIFIED`, `CROSS_SIGNED`, `VERIFIED_IN_PERSON`, `KNOWN_UNCOMPROMISED`, `COMPROMISED`
- Per-message trust indicator wired to `MessageBubble` via sender trust level
- All colors sourced from `ArmorColors` tokens (WarningAmber, SuccessGreen, InfoBlue, ErrorRed)

**ToDevice Event Processing (Bucket 2 Scaffold):**
- `ToDeviceProcessor` interface in `app/src/main/java/app/armorclaw/matrix/` processes `to_device` events from sync responses
- `ToDeviceProcessorStub` implementation logs event counts by type, warns on unknown types
- Handles seven event types: `m.room.key`, `m.room.forwarded_key`, `m.key.verification.start/accept/key/mac/cancel`
- Security contract: implementations MUST NOT log content field (contains cryptographic key material)
- Full implementation pending matrix-rust-sdk integration

**SAS Verification State Machine (Bucket 2 Scaffold):**
- `VerificationState` drives the `EmojiVerificationScreen` UI
- State machine handles: request, accept, key exchange, MAC verification, cancel
- `EmojiVerificationScreen` shows emoji comparison with confirm/deny/cancel actions
- Cancellation shows loading spinner while awaiting server response
- Full crypto operations throw `NotImplementedError` pending matrix-rust-sdk

**Architectural scaffolding added (v4.1.1-alpha05):**
- `VerificationRepositoryImpl` wired with 30+ methods (crypto methods throw `NotImplementedError` pending matrix-rust-sdk)
- `VerificationViewModel` with state machine driving `EmojiVerificationScreen`
- `MemoryWiper` utility for CharArray/ByteArray zeroization
- `VaultRepository` resilience wrapping (mutex, bounded timeout, DB lock retry, fail-closed)
- `BlockerResponseDialog` CharArray zeroization after callback
- Agent intervention RPC methods (`agent.report_intervention`, `agent.resolve_blocker`)
- `AgentStatusMapper` consolidating status string→enum mapping

### Session Management (NEW)

Sessions now properly handle expiration:

```kotlin
data class MatrixSession(
    val userId: String,
    val deviceId: String,
    val accessToken: String,
    val refreshToken: String? = null,
    val homeserver: String,
    val expiresIn: Long? = null,          // Seconds until token expires
    val expiresAt: Long? = null,          // Absolute expiration timestamp
    val loginAt: Long? = null             // When session was created
) {
    fun isExpired(): Boolean
    fun isExpiringSoon(withinSeconds: Long = 300): Boolean
    fun remainingTimeSeconds(): Long?
}
```

**Session Lifecycle:**
1. On login: Calculate `expiresAt` from `expiresIn` or use 24-hour default
2. On app start: Check `isExpired()` before restoring session
3. On API call: Check `isExpiringSoon()` and proactively refresh
4. On expiration: Clear session, redirect to login

### Security Features

| Feature | Implementation | Status |
|---------|---------------|--------|
| Database Encryption | SQLCipher (256-bit) | ✅ |
| Key Storage | AndroidKeyStore | ✅ |
| TLS | TLS 1.3 minimum | ✅ |
| Certificate Pinning | SHA-256 pins | ✅ |
| Biometric Auth | AndroidX Biometric | ✅ |
| Secure Clipboard | Encrypted, auto-clear | ✅ |
| Deep Link Validation | URI validation, confirmation dialogs | ✅ |
| Push Security | No content in push payload | ✅ |
| Memory Zeroization | MemoryWiper utility (CharArray/ByteArray) | ✅ |
| Vault Resilience | Mutex, 5s timeout, retry on DB lock, fail-closed | ✅ |
| Trust Badge UI | Animated trust level badge with 5 levels | ✅ |
| ToDevice Processing | Scaffold for E2EE key exchange event handling | ✅ (Stub) |
| Deep Link Hardening | DANGEROUS_CHARS + DANGEROUS_PATTERNS + null byte + fragment validation | ✅ |
| Security Audit | 7 data paths audited, MemoryWiper gaps identified | ✅ (`doc/SECURITY_AUDIT.md`) |
| ChatScreen Memory | Memoized timestamp formatting, optimized LazyColumn | ✅ |
| Per-Message Trust | Trust indicator on individual message bubbles | ✅ |
| Email Approval | HITL approval screen for email deep-link actions | ✅ |

### OWASP Mobile Top 10 Compliance

| Risk | Status | Mitigation |
|------|--------|------------|
| M1: Improper Platform Usage | ✅ | Secure intent handling, validated deep links |
| M2: Insecure Data Storage | ✅ | SQLCipher, AndroidKeyStore |
| M3: Insecure Communication | ✅ | TLS 1.3, Certificate Pinning |
| M4: Insecure Authentication | ✅ | Biometric, secure token storage |
| M5: Insufficient Cryptography | ✅ | AES-256-GCM, proper key management |
| M6: Insecure Authorization | ✅ | Server-authoritative roles |
| M7: Client Code Quality | ✅ | Clean architecture, code review |
| M8: Code Tampering | ✅ | ProGuard, signature verification |
| M9: Reverse Engineering | ✅ | Code obfuscation, native crypto |
| M10: Extraneous Functionality | ✅ | Debug logging disabled in release |

---

## 10. Communication Architecture

> **Migration Status:** ✅ COMPLETE
> ArmorChat now uses **Matrix Protocol** as the primary channel for all messaging operations.
> Bridge RPC is reserved for **admin operations only**.

ArmorChat communicates through **three primary channels**:

```
┌─────────────────────────────────────────────────────────────────────────┐
│                    COMMUNICATION ARCHITECTURE                            │
├─────────────────────────────────────────────────────────────────────────┤
│                                                                          │
│   ┌─────────────────────────────────────────────────────────────────┐   │
│   │                        ArmorChat (Android App)                   │   │
│   │                                                                  │   │
│   │  ┌──────────────────┐  ┌──────────────────┐  ┌────────────────┐ │   │
│   │  │ BridgeAdminClient│  │   MatrixClient   │  │  Matrix /sync  │ │   │
│   │  │  (Admin RPC)     │  │  (Messaging)     │  │  (Real-time)   │ │   │
│   │  └────────┬─────────┘  └────────┬─────────┘  └───────┬────────┘ │   │
│   │           │                     │                     │          │   │
│   │           │              ┌──────┴───────┐             │          │   │
│   │           │              │ToDeviceProc. │             │          │   │
│   │           │              │ (E2EE Keys)  │◄────────────┘          │   │
│   │           │              └──────────────┘                        │   │
│   └───────────┼─────────────────────┼─────────────────────┼──────────┘   │
│               │                     │                     │               │
│               ▼                     ▼                     ▼               │
│   ┌─────────────────────┐  ┌───────────────────────────────────────────┐ │
│   │  ArmorClaw Bridge   │  │           Matrix Homeserver               │ │
│   │  (Go Backend)       │  │           (Conduit/Synapse)               │ │
│   │                     │  │                                           │ │
│   │  • JSON-RPC 2.0     │  │  • Client API (Messaging, Rooms)         │ │
│   │  • Admin Operations │  │  • /sync (Real-time Events)              │ │
│   │  • Platform Bridges │  │  • Media Repository (Uploads)            │ │
│   │  • License/Comply   │  │  • E2EE Keys (Megolm)                    │ │
│   │  :8080 (HTTP/RPC)   │  │  :8008 (HTTPS)                           │ │
│   └─────────────────────┘  └───────────────────────────────────────────┘ │
│                                                                          │
│   ┌─────────────────────────────────────────────────────────────────────┐│
│   │  PRIMARY CHANNEL: Matrix Protocol (E2E Encrypted)                   ││
│   │  ADMIN CHANNEL:  Bridge RPC (JSON-RPC 2.0)                          ││
│   │  REAL-TIME:      Matrix /sync (Long-poll, NOT WebSocket)            ││
│   └─────────────────────────────────────────────────────────────────────┘│
│                                                                          │
└─────────────────────────────────────────────────────────────────────────┘
```

### Communication Endpoints

| Endpoint | Protocol | Purpose | Status |
|----------|----------|---------|--------|
| **Matrix Client API** | HTTPS | Messaging, rooms, presence | ✅ Primary |
| **Matrix /sync** | HTTPS (long-poll) | Real-time events | ✅ Primary |
| **Matrix Media** | HTTPS | File uploads/downloads | ✅ Primary |
| Bridge RPC | HTTP/JSON-RPC 2.0 | Admin operations only | ✅ Admin |
| ~~Bridge WebSocket~~ | ~~WebSocket~~ | ~~Real-time events~~ | ❌ Not available |

### What Goes Where (Data Flow Summary)

| Operation | Channel | Endpoint | Notes |
|-----------|---------|----------|-------|
| Login | Matrix SDK | `/_matrix/client/v3/login` | E2EE keys retrieved |
| Send Message | Matrix SDK | `/_matrix/client/v3/rooms/.../send` | E2E encrypted |
| Receive Message | Matrix /sync | `/_matrix/client/v3/sync` | Long-poll (30s) |
| Typing/Presence | Matrix SDK | `/_matrix/client/v3/...` | Direct to Matrix |
| File Upload | Matrix Media | `/_matrix/media/v3/upload` | Direct to Matrix |
| Start Bridge Session | Bridge RPC | `bridge.start` | Admin via RPC |
| Platform Connect | Bridge RPC | `platform.connect` | Admin via RPC |
| Push Register | Bridge RPC | `push.register_token` | Admin via RPC |
| Recovery Flow | Bridge RPC | `recovery.*` | Admin via RPC |
| License Check | Bridge RPC | `license.status` | Admin via RPC |

### Resilience Features (Phases 2-5)

**Matrix Sync Recovery** — Exponential backoff with 4 server-signaled triggers:
- `M_UNKNOWN_TOKEN` → full resync (clear state, restart from scratch)
- HTTP 4xx → full resync (authentication/protocol error)
- HTTP 5xx → capped backoff (max 10s, escalate after 5 consecutive failures)
- `IOException` → standard backoff (1s → 2s → 4s → 8s → 30s max)

**Offline Message Queue** — SQLDelight-backed FIFO for offline resilience:
- Schema: `OfflineQueue.sq` with status tracking (PENDING → PROCESSING → COMPLETED/FAILED)
- Eviction: drop-oldest-when-full (insertion-age), max 1000 entries
- Retry: max 3 retries per message, dead-letter after exhaustion
- Drain: FIFO order, automatic on connectivity restore

**Session Refresh Manager** — Proactive token refresh:
- 5-minute monitoring interval with mutex-guarded refresh
- 5-minute expiry warning threshold before proactive refresh
- `RefreshResult` sealed class (Success/Failed/NetworkError/ServerError)

### Bridge RPC Method Coverage (Admin Only)

| Category | Methods | Count |
|----------|---------|-------|
| Bridge Lifecycle | `bridge.start`, `bridge.stop`, `bridge.status`, `health` | 4 |
| Platform Integration | `platform.connect`, `platform.disconnect`, `platform.list` | 5 |
| Push Notifications | `push.register_token`, `push.unregister_token`, `push.update_settings` | 3 |
|| License & Compliance | `license.status`, `compliance.status` | 5 |
|| Recovery | `recovery.generate_phrase`, `recovery.verify`, etc. | 6 |
|| Provisioning | `provisioning.claim`, `provisioning.rotate`, `provisioning.revoke`, `provisioning.get_config`, `provisioning.set_config` | 5 |
|| Error Management | `get_errors`, `resolve_error` | 2 |
|| **Total** | | **44** |

### Real-Time Events (Matrix /sync)

> **Important:** Real-time events come from Matrix /sync, NOT from Bridge WebSocket.
> The Bridge WebSocket is not available. Use `MatrixSyncManager` to subscribe to events.

Events received via Matrix /sync long-polling:

| Event Type | Matrix Event | Purpose |
|------------|--------------|---------|
| `m.room.message` | Timeline | New message in room |
| `m.room.encrypted` | Timeline | Encrypted message (decrypt client-side) |
| `m.typing` | Ephemeral | Typing indicator |
| `m.receipt` | Ephemeral | Read receipt |
| `m.presence` | Presence | User presence change |
| `m.room.member` | State | Room membership change |
| `m.call.*` | Timeline | VoIP call signaling |
| `com.armorclaw.*` | Timeline | ArmorClaw control plane events |

### ArmorClaw Control Plane Events (via Matrix)

| Event Type | Purpose |
|------------|---------|
| `message.received` | New message in room |
| `message.status` | Message delivery status |
| `room.created` | New room created |
| `room.membership` | User joined/left room |
| `typing` | Typing indicator |
| `receipt.read` | Read receipt |
| `presence` | User presence change |
| `call` | WebRTC call signaling |
| `platform.message` | External platform message |
| `session.expired` | Bridge session ended |
| `bridge.status` | Bridge health update |
| `recovery` | Recovery flow events |
| `app.armorclaw.alert` | System alerts |
| `license` | License state changes |
| `budget` | Budget usage alerts |
| `compliance` | Compliance notifications |

---

## 11. Control Plane & Agent System

### Control Plane Events

ArmorClaw extends Matrix with custom event types for control plane operations:

```kotlin
object ArmorClawEventType {
    // Workflow events
    const val WORKFLOW_STARTED = "com.armorclaw.workflow.started"
    const val WORKFLOW_STEP = "com.armorclaw.workflow.step"
    const val WORKFLOW_COMPLETED = "com.armorclaw.workflow.completed"
    const val WORKFLOW_FAILED = "com.armorclaw.workflow.failed"

    // Agent events
    const val AGENT_TASK_STARTED = "com.armorclaw.agent.task.started"
    const val AGENT_TASK_PROGRESS = "com.armorclaw.agent.task.progress"
    const val AGENT_TASK_COMPLETE = "com.armorclaw.agent.task.complete"
    const val AGENT_THINKING = "com.armorclaw.agent.thinking"

    // System events
    const val BUDGET_WARNING = "com.armorclaw.budget.warning"
    const val BRIDGE_CONNECTED = "com.armorclaw.bridge.connected"
    const val BRIDGE_DISCONNECTED = "com.armorclaw.bridge.disconnected"
}
```

### Agent Management (NEW in v3.3.0)

ArmorChat now includes a dedicated **Agent Management Screen** for viewing and controlling AI agents:

| Feature | Description | Status |
|---------|-------------|--------|
| Agent List | View all running agents with status | ✅ |
| Agent Status | Real-time status (idle/busy/error) | ✅ |
| Stop Agent | Safely stop a running agent | ✅ |
| Room Association | See which room an agent is active in | ✅ |

**New ViewModels:**
| ViewModel | Purpose |
|-----------|---------|
| `AgentManagementViewModel` | Manage AI agents |
| `HitlViewModel` | Handle HITL approvals |
| `WorkflowViewModel` | Workflow management |

**New RPC Methods:**
| Method | Purpose |
|--------|---------|
| `agent.list` | List all running agents |
| `agent.status` | Get agent status details |
| `agent.stop` | Stop a running agent |

### HITL (Human-in-the-Loop) Approvals (NEW in v3.3.0)

The **HITL Approval Screen** allows users to review and approve/reject AI action requests:

| Feature | Description | Status |
|---------|-------------|--------|
| Pending List | View all pending approvals | ✅ |
| Approve/Reject | One-click approval or rejection | ✅ |
| Priority Levels | Critical, High, Medium, Low | ✅ |
| Reason Input | Optional rejection reason | ✅ |

**New RPC Methods:**
| Method | Purpose |
|--------|---------|
| `hitl.pending` | Get pending approvals |
| `hitl.approve` | Approve a request |
| `hitl.reject` | Reject a request |

### Workflow Management (NEW in v3.3.0)

**New RPC Methods:**
| Method | Purpose |
|--------|---------|
| `workflow.templates` | List available workflow templates |
| `workflow.start` | Start a new workflow |
| `workflow.status` | Get workflow status |

### Budget Tracking (NEW in v3.3.0)

**New RPC Method:**
| Method | Purpose |
|--------|---------|
| `budget.status` | Get budget usage and limits |

### Agent Types

```kotlin
enum class AgentType {
    GENERAL,           // General purpose assistant
    ANALYSIS,          // Document/data analysis
    CODE_REVIEW,       // Code review and suggestions
    RESEARCH,          // Research and information gathering
    WRITING,           // Content writing
    TRANSLATION,       // Translation services
    SCHEDULING,        // Calendar and scheduling
    WORKFLOW,          // Workflow orchestration
    PLATFORM_BRIDGE    // External platform integration
}
```

### Agent Status

```kotlin
enum class AgentStatus {
    ONLINE,
    BUSY,
    THINKING,
    OFFLINE,
    ERROR
}
```

---

## 12. Feature Reference

### Core Features (150+)

#### Authentication
- ✅ Login (username/email + password)
- ✅ Biometric authentication
- ✅ Secure session management
- ⚠️ Password reset (placeholder)
- ⚠️ Registration (placeholder)

#### Onboarding
- ✅ Welcome screen
- ✅ Security explanation (4 steps)
- ✅ Server connection
- ✅ Permission requests
- ✅ Completion celebration
- ✅ Tutorial (optional)
- ✅ State persistence

#### Chat
- ✅ Message list with all states
- ✅ Message bubbles with status
- ✅ Reply/Forward/React
- ✅ File/Image attachments
- ✅ Voice messages
- ✅ Typing indicators
- ✅ Read receipts
- ✅ Thread support
- ✅ Encryption indicators
- ✅ Command detection

#### Rooms
- ✅ Room list (active/favorites/archived)
- ✅ Create/Join rooms
- ✅ Room details
- ✅ Room settings
- ✅ Pull-to-refresh

#### Calls
- ✅ Voice calls
- ✅ Video calls
- ✅ Incoming call dialog
- ✅ Call controls (mute, speaker, video, hold)

#### Profile
- ✅ Profile display/edit
- ✅ Change password (wired to Matrix `POST /_matrix/client/v3/account/password`)
- ✅ Change phone (wired to Matrix 3PID endpoints: requestToken + add)
- ✅ Edit bio (wired to MatrixClient.setDisplayName())
- ✅ Delete account (wired to Matrix `POST /_matrix/client/v3/account/deactivate` + UIAA)

#### Settings
- ✅ Settings screen
- ✅ Security settings
- ✅ Notification settings
- ✅ Appearance settings
- ✅ Device management
- ✅ Verification flow
- ✅ About screen
- ✅ Report bug

### Platform Integrations

| Platform | Bridge | License Tier |
|----------|--------|--------------|
| Slack | ✅ | Essential+ |
| Discord | ✅ | Professional+ |
| Teams | ✅ | Professional+ |
| WhatsApp | ✅ | Enterprise only |

---

## 13. Build & Configuration

### Build Commands

```bash
# Clean
.\gradlew.bat clean

# Build Debug APK
.\gradlew.bat assembleDebug

# Build Release APK
.\gradlew.bat assembleRelease

# Install Debug
.\gradlew.bat installDebug

# Unit Tests
.\gradlew.bat test

# Instrumented Tests
.\gradlew.bat connectedAndroidTest

# Static Analysis
.\gradlew.bat detekt
```

### Build Variants

| Type | Flavors | Purpose |
|------|---------|---------|
| Debug | demo, alpha, beta, stable | Development |
| Release | demo, alpha, beta, stable | Production |

### Build Optimizations

- **R8**: Code shrinking and obfuscation
- **ProGuard**: Additional rules in `app/proguard-rules.pro`
- **Resource Shrinking**: `isShrinkResources = true`
- **Native Library Optimization**: Symbol stripping

### Debug TestHooks (Phases 2-5)

Debug builds include a test instrumentation layer (`app/src/debug/`):

| Component | Type | Purpose |
|-----------|------|---------|
| `SET_NETWORK_OFFLINE` | ADB Broadcast | Simulate network disconnection |
| `SET_NETWORK_ONLINE` | ADB Broadcast | Restore network connectivity |
| `SET_AUTH_EXPIRED` | ADB Broadcast | Force auth token expiration |
| `SET_VAULT_LOCKED` | ADB Broadcast | Lock the vault (simulate timeout) |
| `TRIGGER_SYNC_ERROR` | ADB Broadcast | Inject a sync failure |
| `UiStateContentProvider` | Content Provider | Query UI state at `content://com.armorclaw.test.uistate` |

Initialization via `TestHooksInitProvider` (ContentProvider, no Application subclass dependency).

### Dependency Versions

| Dependency | Version |
|------------|---------|
| Kotlin | 1.9.20 |
| Compose | 1.5.0 |
| Material 3 | 1.1.2 |
| Koin | 3.5.0 |
| SQLDelight | 2.0.0 |
| Ktor | 2.3.5 |
| Coroutines | 1.7.3 |
| Sentry | 7.6.0 |
| Coil | 2.4.0 |

---

## 14. Development Guidelines

### Code Style

- Follow Kotlin coding conventions
- Use meaningful variable names
- Prefer immutability (val over var)
- Use data classes for models
- Use sealed classes for state

### Architecture Guidelines

- **Domain layer**: No Android dependencies
- **Data layer**: Repository implementations
- **Presentation layer**: ViewModels + Compose
- **Platform layer**: Android service implementations for platform code

### Dependency Injection

```kotlin
// Define module
val appModule = module {
    viewModel { ChatViewModel(get(), get(), get()) }
    single<MatrixClient> { MatrixClientFactory.create(get()) }
    single<BridgeRpcClient> { BridgeClientFactory.createClient(get()) }
}

// Use in Compose
val viewModel: ChatViewModel = koinViewModel()
val client: MatrixClient = koinInject()
```

### Navigation

```kotlin
// Navigate with parameters
navController.navigate(AppNavigation.createChatRoute(roomId))

// Navigate with pop up
navController.navigate(AppNavigation.HOME) {
    popUpTo(AppNavigation.SPLASH) { inclusive = true }
}
```

### Logging

```kotlin
// Use LoggerDelegate
private val logger = LoggerDelegate(LogTag.Domain.Chat)

logger.logInfo("Message sent", mapOf("roomId" to roomId))
logger.logError("Failed to send", exception)
```

### Error Handling

```kotlin
// Use AppResult
suspend fun sendMessage(...): AppResult<String> {
    return try {
        val eventId = matrixClient.sendTextMessage(roomId, text).getOrThrow()
        AppResult.success(eventId)
    } catch (e: Exception) {
        AppResult.error(ArmorClawErrorCode.MESSAGE_SEND_FAILED, e.message)
    }
}
```

### ArmorColors Lint Rule

A custom Detekt rule (`HardcodedColorLiteral`) enforces use of `ArmorColors` tokens over hardcoded `Color(0x...)` literals:

```bash
# Run the lint gate
./gradlew :app:detekt

# Rule configuration (detekt-config.yml)
armorclaw:
  HardcodedColorLiteral:
    active: true
    allowlist: ["ui/theme/"]  # Only the theme package may define raw colors
```

New colors must be added to `ArmorColors` in `ui/theme/`. The rule catches violations at build time — zero hardcoded colors allowed outside the theme package.

### TestToolRegistry

Future development requiring UI testing should reference the TestToolRegistry spec (`.sisyphus/plans/test-tool-registry-spec.md`). The spec defines 6 test tools for manipulating app state during automated testing. Debug TestHooks (`app/src/debug/`) provide the initial implementation.

---

## 15. ArmorChat ↔ ArmorClaw Communication

This section describes how ArmorChat (the Android app) communicates with ArmorClaw (the Bridge server backend written in Go) and the Matrix homeserver.

> **Migration Status:** ✅ COMPLETE
> ArmorChat now uses **Matrix Protocol** as the primary communication channel with **E2E encryption**.
> Bridge RPC is used **only for admin operations** (lifecycle, recovery, platforms, push).
> See `doc/MATRIX_MIGRATION.md` for full migration details.

### 15.1 Server Discovery Flow

Before ArmorChat can communicate, it must discover server endpoints:

```
┌─────────────────────────────────────────────────────────────────────────────────┐
│                     ARMORCHAT SERVER DISCOVERY FLOW                              │
├─────────────────────────────────────────────────────────────────────────────────┤
│                                                                                  │
│  DISCOVERY PRIORITY (Highest to Lowest Trust):                                   │
│                                                                                  │
│  1. SIGNED QR/DEEP LINK ────────────────────────────────────────────────         │
│     │                                                                            │
│     │  User scans QR or taps deep link                                          │
│     │  Format: armorclaw://config?d=<base64-config>                             │
│     │  Contains: matrix_homeserver, rpc_url, ws_url, push_gateway               │
│     │  Signed by Bridge, expires after 24h                                       │
│     │                                                                            │
│     └────────────────────────────────────────────────────────────────────────▶  │
│                                                                                  │
│  2. WELL-KNOWN DISCOVERY ───────────────────────────────────────────────        │
│     │                                                                            │
│     │  User enters domain: "armorclaw.app"                                      │
│     │  App fetches: https://armorclaw.app/.well-known/matrix/client             │
│     │  Response includes Bridge URLs under "com.armorclaw.bridge"               │
│     │  Standard Matrix discovery mechanism                                       │
│     │                                                                            │
│     └────────────────────────────────────────────────────────────────────────▶  │
│                                                                                  │
│  3. mDNS DISCOVERY (Local Network) ─────────────────────────────────────        │
│     │                                                                            │
│     │  App scans for _armorclaw._tcp.local                                      │
│     │  Bridge advertises itself via mDNS                                        │
│     │  Auto-discovers IP, port, and basic config                                │
│     │  May require QR for Matrix URL if not included                            │
│     │                                                                            │
│     └────────────────────────────────────────────────────────────────────────▶  │
│                                                                                  │
│  4. MANUAL ENTRY ───────────────────────────────────────────────────────        │
│     │                                                                            │
│     │  User enters: https://matrix.armorclaw.app                                │
│     │  App derives: https://bridge.armorclaw.app/api                            │
│     │  Fallback when other methods fail                                         │
│     │                                                                            │
│     └────────────────────────────────────────────────────────────────────────▶  │
│                                                                                  │
│  5. FALLBACK SERVERS ───────────────────────────────────────────────────        │
│     │                                                                            │
│     │  Pre-configured backup servers:                                           │
│     │  • bridge.armorclaw.app                                                   │
│     │  • bridge-backup.armorclaw.app                                            │
│     │  • bridge-eu.armorclaw.app                                                │
│     │                                                                            │
│     └────────────────────────────────────────────────────────────────────────▶  │
│                                                                                  │
│  6. BUILDCONFIG DEFAULTS ───────────────────────────────────────────────        │
│     │                                                                            │
│     │  Production defaults in BuildConfig:                                      │
│     │  • MATRIX_HOMESERVER = "https://matrix.armorclaw.app"                     │
│     │  • ARMORCLAW_RPC_URL = "https://bridge.armorclaw.app/api"                 │
│     │  • ARMORCLAW_WS_URL = "wss://bridge.armorclaw.app/ws"                     │
│     │                                                                            │
│     └────────────────────────────────────────────────────────────────────────▶  │
│                                                                                  │
└─────────────────────────────────────────────────────────────────────────────────┘
```

### 15.2 Configuration Data Flow

```
┌─────────────────────────────────────────────────────────────────────────────────┐
│                     CONFIGURATION STORAGE & PRIORITY                             │
├─────────────────────────────────────────────────────────────────────────────────┤
│                                                                                  │
│  ┌──────────────────────────────────────────────────────────────────────────┐   │
│  │                        SetupService (Shared)                              │   │
│  │                                                                            │   │
│  │  parseSignedConfig(deepLink) → SetupConfig                                │   │
│  │  startSetupWithDiscovery(input) → SetupConfig                             │   │
│  │  useDemoServer() → SetupConfig                                            │   │
│  │                                                                            │   │
│  │  SetupConfig:                                                              │   │
│  │  • homeserver: String        (Matrix server URL)                          │   │
│  │  • bridgeUrl: String?        (Bridge RPC URL: /api)                       │   │
│  │  • wsUrl: String?            (WebSocket URL: /ws)                         │   │
│  │  • pushGateway: String?      (Push gateway URL)                           │   │
│  │  • serverName: String?       (Human-readable name)                        │   │
│  │  • configSource: ConfigSource (How config was obtained)                   │   │
│  │  • expiresAt: Long?          (Expiration timestamp)                       │   │
│  │                                                                            │   │
│  └────────────────────────────────────────────────────────────────────────────┘   │
│                                      │                                          │
│                                      ▼                                          │
│  ┌──────────────────────────────────────────────────────────────────────────┐   │
│  │                    ConfigRepository (Shared Domain)                       │   │
│  │                                                                            │   │
│  │  Interface: domain/repository/ConfigRepository.kt                          │   │
│  │  Implementation: data/repository/ConfigRepositoryImpl                     │   │
│  │                                                                            │   │
│  │  Methods:                                                                  │   │
│  │  • getServerConfig() → ServerConfig                                       │   │
│  │  • saveServerConfig(config) → Unit                                        │   │
│  │  • getProvisioningStatus() → ProvisioningStatus                           │   │
│  │  • isServerSigned() → Boolean                                             │   │
│  │  • getConnectionProfile() → ConnectionProfile                             │   │
│  │                                                                            │   │
│  │  SharedPreferences Persistence:                                            │   │
│  │  • matrix_homeserver                                                       │   │
│  │  • bridge_url                                                              │   │
│  │  • ws_url                                                                  │   │
│  │  • push_gateway                                                            │   │
│  │  • server_name                                                             │   │
│  │  • config_source                                                           │   │
│  │  • expires_at                                                              │   │
│  │                                                                            │   │
│  └────────────────────────────────────────────────────────────────────────────┘   │
│                                                                                  │
│  ConfigSource Priority:                                                          │
│  SIGNED_URL > WELL_KNOWN > MDNS > MANUAL > CACHED > DEFAULT                      │
│                                                                                  │
└─────────────────────────────────────────────────────────────────────────────────┘
```

### 15.3 Communication Architecture Overview

ArmorChat uses **three parallel communication channels**:

```
┌─────────────────────────────────────────────────────────────────────────────────┐
│                     ARMORCHAT COMMUNICATION ARCHITECTURE                         │
├─────────────────────────────────────────────────────────────────────────────────┤
│                                                                                  │
│   ┌─────────────────────────────────────────────────────────────────────────┐   │
│   │                        ArmorChat (Android App)                          │   │
│   │                                                                         │   │
│   │  ┌──────────────────┐  ┌──────────────────┐  ┌──────────────────┐      │   │
│   │  │ BridgeRpcClient  │  │ MatrixSyncManager│  │ MatrixClient     │      │   │
│   │  │ (Admin Ops)      │  │ (Real-time)      │  │ (Direct API)     │      │   │
│   │  └────────┬─────────┘  └────────┬─────────┘  └────────┬─────────┘      │   │
│   │           │                     │                     │                │   │
│   └───────────┼─────────────────────┼─────────────────────┼────────────────┘   │
│               │                     │                     │                     │
│               ▼                     ▼                     ▼                     │
│                                                                                  │
│   ┌─────────────────────────────────────────────────────────────────────────┐   │
│   │                        COMMUNICATION CHANNELS                            │   │
│   │                                                                          │   │
│   │  CHANNEL 1: Bridge RPC (HTTP/JSON-RPC 2.0)                              │   │
│   │  ────────────────────────────────────────                                │   │
│   │  • Bridge lifecycle (bridge.start/stop/status)                          │   │
│   │  • Recovery operations (recovery.generate_phrase, etc.)                 │   │
│   │  • Platform integration (platform.connect/list/status)                  │   │
│   │  • Push notifications (push.register_token/unregister_token)            │   │
│   │  • License management (license.status/features)                         │   │
│   │  • Compliance (compliance.status)                                       │   │
│   │  • QR generation (qr.config/invite)                                     │   │
│   │  • Provisioning (provisioning.claim/rotate/revoke/get_config/set_config)│   │
│   │                                                                          │   │
│   │  Endpoint: https://bridge.armorclaw.app/api                             │   │
│   │  Protocol: JSON-RPC 2.0 over HTTPS POST                                 │   │
│   │  ⚠️  IMPORTANT: Endpoint is /api NOT /rpc                               │   │
│   │                                                                          │   │
│   └──────────────────────────────────────────────────────────────────────────┘   │
│                                                                                  │
│   ┌─────────────────────────────────────────────────────────────────────────┐   │
│   │                                                                          │   │
│   │  CHANNEL 2: Matrix /sync (Direct HTTPS Long-Poll)                       │   │
│   │  ─────────────────────────────────────────────                          │   │
│   │  • Message received events                                              │   │
│   │  • Message status updates (sent/delivered/read)                         │   │
│   │  • Typing indicators                                                    │   │
│   │  • Presence updates                                                     │   │
│   │  • Read receipts                                                        │   │
│   │  • Room membership changes                                              │   │
│   │  • Room state changes (name, topic, avatar, encryption)                 │   │
│   │  • Call signaling (m.call.* events)                                     │   │
│   │  • To-device messages (E2EE key exchange)                               │   │
│   │  • Device list changes (cross-signing)                                  │   │
│   │                                                                          │   │
│   │  Endpoint: https://matrix.armorclaw.app/_matrix/client/v3/sync          │   │
│   │  Protocol: HTTPS GET with long-polling (30s timeout)                    │   │
│   │                                                                          │   │
│   └──────────────────────────────────────────────────────────────────────────┘   │
│                                                                                  │
│   ┌─────────────────────────────────────────────────────────────────────────┐   │
│   │                                                                          │   │
│   │  CHANNEL 3: Matrix Media (Direct HTTPS Upload)                          │   │
│   │  ─────────────────────────────────────────                              │   │
│   │  • File uploads (images, videos, documents)                             │   │
│   │  • Thumbnail generation                                                  │   │
│   │  • Avatar uploads                                                        │   │
│   │                                                                          │   │
│   │  Endpoint: https://matrix.armorclaw.app/_matrix/media/v3/upload         │   │
│   │  Protocol: HTTPS POST (multipart/form-data)                             │   │
│   │                                                                          │   │
│   └──────────────────────────────────────────────────────────────────────────┘   │
│                                                                                  │
│                                   ▼                                              │
│                                                                                  │
│   ┌─────────────────────────────────────────────────────────────────────────┐   │
│   │                          SERVER INFRASTRUCTURE                           │   │
│   │                                                                          │   │
│   │  ┌─────────────────────┐       ┌─────────────────────┐                  │   │
│   │  │  ArmorClaw Bridge   │       │  Matrix Homeserver  │                  │   │
│   │  │  (Go Backend)       │       │  (Conduit/Synapse)  │                  │   │
│   │  │                     │       │                     │                  │   │
│   │  │  • HTTP :8443/api   │◄─────►│  • Client API       │                  │   │
│   │  │  • JSON-RPC 2.0     │       │  • Federation       │                  │   │
│   │  │  • Platform bridges │       │  • Media repo       │                  │   │
│   │  │  • License mgmt     │       │  • E2EE keys        │                  │   │
│   │  │  • QR generation    │       │                     │                  │   │
│   │  └─────────────────────┘       └─────────────────────┘                  │   │
│   │                                                                          │   │
│   └──────────────────────────────────────────────────────────────────────────┘   │
│                                                                                  │
└─────────────────────────────────────────────────────────────────────────────────┘
```

### 15.2 Communication Endpoints

| Endpoint | Protocol | Purpose | Default URL |
|----------|----------|---------|-------------|
| **Bridge RPC** | HTTP/JSON-RPC 2.0 | Admin and control-plane operations only | `https://bridge.armorclaw.app/api` |
| **Matrix Client API** | HTTPS | Direct Matrix operations | `https://matrix.armorclaw.app/_matrix/client/v3/` |
| **Matrix Sync** | HTTPS (long-poll) | Real-time events | `https://matrix.armorclaw.app/_matrix/client/v3/sync` |
| **Matrix Media** | HTTPS | File uploads/downloads | `https://matrix.armorclaw.app/_matrix/media/v3/` |
| ~~Bridge WebSocket~~ | ~~WebSocket~~ | ~~ArmorTerminal only~~ | **NOT AVAILABLE** (use Matrix /sync) |

### 15.3 Configuration

The app supports multiple configuration sources with priority ordering:

| Priority | Source | Description |
|----------|--------|-------------|
| 1 (Highest) | Runtime Config | Set via `BridgeConfig.setRuntimeConfig()` |
| 2 | Well-Known Discovery | Auto-discovered from `/.well-known/matrix/client` |
| 3 | BuildConfig Defaults | Configured at build time |
| 4 (Lowest) | Hardcoded Fallbacks | PRODUCTION defaults |

```kotlin
data class BridgeConfig(
    val baseUrl: String,              // Bridge RPC URL
    val homeserverUrl: String,        // Matrix homeserver URL
    val wsUrl: String? = null,        // WebSocket (null - not available)
    val useDirectMatrixSync: Boolean = true,  // Use Matrix /sync
    val environment: Environment = Environment.PRODUCTION,
    val serverName: String? = null
) {
    enum class Environment {
        DEVELOPMENT,    // Local development
        STAGING,        // Test server
        PRODUCTION,     // Live server
        CUSTOM          // User-configured (self-hosted VPS)
    }

    companion object {
        // Set runtime configuration (highest priority)
        fun setRuntimeConfig(config: BridgeConfig)

        // Create custom configuration for self-hosted servers
        fun createCustom(
            bridgeUrl: String,
            homeserverUrl: String,
            serverName: String? = null
        ): BridgeConfig

        // Derive Bridge URL from Matrix homeserver
        fun deriveBridgeUrl(homeserver: String): String

        // Predefined configurations
        val PRODUCTION = BridgeConfig(
            baseUrl = "https://bridge.armorclaw.app",
            homeserverUrl = "https://matrix.armorclaw.app",
            environment = Environment.PRODUCTION,
            serverName = "ArmorClaw"
        )

        val DEVELOPMENT = BridgeConfig(
            baseUrl = "http://10.0.2.2:8080",      // Android emulator
            homeserverUrl = "http://10.0.2.2:8008", // Local Matrix
            environment = Environment.DEVELOPMENT,
            serverName = "Development Server"
        )

        val STAGING = BridgeConfig(
            baseUrl = "https://bridge-staging.armorclaw.app",
            homeserverUrl = "https://matrix-staging.armorclaw.app",
            environment = Environment.STAGING,
            serverName = "Staging Server"
        )
    }
}
```

### 15.3.1 Well-Known Discovery (NEW)

The app supports automatic server configuration via Matrix well-known:

```
GET https://example.com/.well-known/matrix/client
```

Response format:
```json
{
  "m.homeserver": {
    "base_url": "https://matrix.example.com"
  },
  "com.armorclaw": {
    "bridge_url": "https://bridge.example.com",
    "ws_url": "wss://bridge.example.com/ws",
    "push_gateway": "https://bridge.example.com/_matrix/push/v1/notify",
    "server_name": "Example Server",
    "version": "1.6.2",
    "region": "us-east",
    "supports_e2ee": true,
    "supports_recovery": true
  }
}
```

### 15.3.2 Self-Hosted VPS Configuration (NEW)

For connecting to your own Matrix homeserver:

**Option 1: Runtime Configuration**
```kotlin
BridgeConfig.setRuntimeConfig(
    BridgeConfig.createCustom(
        bridgeUrl = "https://your-vps.com:8080",
        homeserverUrl = "https://your-vps.com:8008",
        serverName = "My VPS"
    )
)
```

**Option 2: Well-Known Auto-Discovery**
Set up `/.well-known/matrix/client` on your domain.

**Option 3: Manual Entry in App**
Enter server address in Connect screen - app will auto-derive endpoints.

### 15.4 Setup Flow (First Connection)

When a user first sets up ArmorChat:

```
┌─────────────────────────────────────────────────────────────────────────────────┐
│                          SETUP FLOW COMMUNICATION                                │
├─────────────────────────────────────────────────────────────────────────────────┤
│                                                                                  │
│  STEP 1: USER ENTERS SERVER INFO                                                 │
│  ────────────────────────────────                                                │
│  User Input:                                                                     │
│  • Homeserver URL (e.g., "https://matrix.example.com")                          │
│  • Bridge URL (optional, auto-derived if not provided)                          │
│                                                                                  │
│  STEP 2: SERVER HEALTH CHECK                                                     │
│  ────────────────────────────                                                    │
│  ArmorChat ──HTTP──▶ POST /rpc (JSON-RPC 2.0)                                   │
│                                                                                  │
│  Request:                                                                        │
│  {                                                                               │
│    "jsonrpc": "2.0",                                                            │
│    "method": "bridge.health",                                                   │
│    "params": {                                                                   │
│      "correlation_id": "corr_abc123"                                            │
│    },                                                                           │
│    "id": "req_001"                                                              │
│  }                                                                              │
│                                                                                  │
│  Response:                                                                       │
│  {                                                                               │
│    "jsonrpc": "2.0",                                                            │
│    "result": {                                                                   │
│      "version": "1.6.2",                                                        │
│      "supports_e2ee": true,                                                     │
│      "supports_recovery": true,                                                 │
│      "region": "us-east"                                                        │
│    },                                                                           │
│    "id": "req_001"                                                              │
│  }                                                                              │
│                                                                                  │
│  STEP 3: USER ENTERS CREDENTIALS                                                 │
│  ──────────────────────────────────                                              │
│  User Input:                                                                     │
│  • Username (e.g., "@alice:example.com")                                        │
│  • Password                                                                     │
│                                                                                  │
│  STEP 4: BRIDGE SESSION START                                                    │
│  ────────────────────────────                                                    │
│  ArmorChat ──RPC───▶ bridge.start(userId, deviceId)                             │
│                                                                                  │
│  Request:                                                                        │
│  {                                                                               │
│    "jsonrpc": "2.0",                                                            │
│    "method": "bridge.start",                                                    │
│    "params": {                                                                   │
│      "user_id": "@alice:example.com",                                           │
│      "device_id": "ANDROID_abc123",                                             │
│      "correlation_id": "corr_def456"                                            │
│    },                                                                           │
│    "id": "req_002"                                                              │
│  }                                                                              │
│                                                                                  │
│  Response:                                                                       │
│  {                                                                               │
│    "jsonrpc": "2.0",                                                            │
│    "result": {                                                                   │
│      "session_id": "sess_xyz789",                                               │
│      "container_id": "container_abc",                                           │
│      "status": "running",                                                       │
│      "ice_servers": [                                                           │
│        {"urls": ["stun:stun.l.google.com:19302"]},                              │
│        {"urls": ["turn:turn.armorclaw.app:3478"], "username": "...", ...}       │
│      ]                                                                          │
│    },                                                                           │
│    "id": "req_002"                                                              │
│  }                                                                              │
│                                                                                  │
│  STEP 5: MATRIX AUTHENTICATION (via Bridge)                                      │
│  ────────────────────────────────────────────                                    │
│  ArmorChat ──RPC───▶ matrix.login(homeserver, username, password, deviceId)     │
│                                                                                  │
│  Request:                                                                        │
│  {                                                                               │
│    "jsonrpc": "2.0",                                                            │
│    "method": "matrix.login",                                                    │
│    "params": {                                                                   │
│      "homeserver": "https://matrix.example.com",                                │
│      "username": "@alice:example.com",                                          │
│      "password": "********",                                                    │
│      "device_id": "ANDROID_abc123",                                             │
│      "correlation_id": "corr_ghi789"                                            │
│    },                                                                           │
│    "id": "req_003"                                                              │
│  }                                                                              │
│                                                                                  │
│  Response:                                                                       │
│  {                                                                               │
│    "jsonrpc": "2.0",                                                            │
│    "result": {                                                                   │
│      "user_id": "@alice:example.com",                                           │
│      "access_token": "syt_abc123...",                                           │
│      "device_id": "ANDROID_abc123",                                             │
│      "refresh_token": "ref_xyz...",                                             │
│      "display_name": "Alice",                                                   │
│      "avatar_url": "mxc://matrix.example.com/avatar123"                         │
│    },                                                                           │
│    "id": "req_003"                                                              │
│  }                                                                              │
│                                                                                  │
│  STEP 6: START MATRIX SYNC (Real-time Events)                                    │
│  ─────────────────────────────────────────────                                   │
│  ArmorChat ──HTTPS──▶ GET /_matrix/client/v3/sync                               │
│                                                                                  │
│  Request:                                                                        │
│  GET https://matrix.example.com/_matrix/client/v3/sync                          │
│    ?access_token=syt_abc123...                                                  │
│    &timeout=30000                                                               │
│    &since=<empty for initial sync>                                              │
│                                                                                  │
│  Response:                                                                       │
│  {                                                                               │
│    "next_batch": "batch_token_abc123",                                          │
│    "rooms": {                                                                    │
│      "join": {                                                                   │
│        "!room1:example.com": {                                                  │
│          "timeline": { "events": [...] },                                       │
│          "state": { "events": [...] },                                          │
│          "ephemeral": { "events": [...] }                                       │
│        }                                                                         │
│      }                                                                           │
│    },                                                                           │
│    "presence": { "events": [...] },                                             │
│    "device_lists": { "changed": [...], "left": [...] }                          │
│  }                                                                              │
│                                                                                  │
│  ⚠️  NOTE: This replaces WebSocket. The sync loop continues indefinitely.       │
│                                                                                  │
│  STEP 7: REGISTER PUSH TOKEN                                                     │
│  ────────────────────────────                                                    │
│  ArmorChat ──RPC───▶ push.register_token(token, platform, deviceId)             │
│                                                                                  │
│  Request:                                                                        │
│  {                                                                               │
│    "jsonrpc": "2.0",                                                            │
│    "method": "push.register_token",                                             │
│    "params": {                                                                   │
│      "push_token": "fcm_xyz789...",                                             │
│      "push_platform": "fcm",                                                    │
│      "device_id": "ANDROID_abc123"                                              │
│    },                                                                           │
│    "id": "req_004"                                                              │
│  }                                                                              │
│                                                                                  │
│  STEP 8: SETUP COMPLETE                                                          │
│  ─────────────────────                                                           │
│  • Session persisted in encrypted storage (SQLCipher)                           │
│  • Push token registered with bridge                                            │
│  • Matrix sync loop running in background                                       │
│  • User navigated to home screen                                                 │
│                                                                                  │
└─────────────────────────────────────────────────────────────────────────────────┘
```

### 15.5 Message Sending Flow

> **⚠️ DEPRECATED:** This section documents the legacy Bridge RPC message flow (`matrix.send`). Messages are now sent **directly via MatrixClient** (Matrix SDK). See [Section 15.18](#1518-complete-data-flow-example) for the current end-to-end flow.

When a user sends a text message:

```
┌─────────────────────────────────────────────────────────────────────────────────┐
│                        MESSAGE SENDING FLOW                                      │
├─────────────────────────────────────────────────────────────────────────────────┤
│                                                                                  │
│  USER SENDS MESSAGE                                                              │
│  ────────────────────                                                            │
│  User types "Hello!" and presses send in room !abc123:example.com              │
│                                                                                  │
│  STEP 1: LOCAL STATE UPDATE (Optimistic UI)                                      │
│  ─────────────────────────────────────────────                                   │
│  ChatViewModel updates local state:                                              │
│  • Add message to list with status SENDING                                       │
│  • Message appears immediately in UI with clock icon                             │
│                                                                                  │
│  STEP 2: RPC CALL TO BRIDGE                                                      │
│  ─────────────────────────                                                       │
│  ArmorChat ──RPC───▶ matrix.send(roomId, eventType, content)                    │
│                                                                                  │
│  Request:                                                                        │
│  {                                                                               │
│    "jsonrpc": "2.0",                                                            │
│    "method": "matrix.send",                                                     │
│    "params": {                                                                   │
│      "room_id": "!abc123:example.com",                                          │
│      "event_type": "m.room.message",                                            │
│      "content": {                                                                │
│        "msgtype": "m.text",                                                     │
│        "body": "Hello!"                                                         │
│      },                                                                          │
│      "txn_id": "txn_1708123456789"                                              │
│    },                                                                           │
│    "id": "req_005"                                                              │
│  }                                                                              │
│                                                                                  │
│  STEP 3: BRIDGE PROCESSING                                                       │
│  ─────────────────────────                                                       │
│  ArmorClaw Bridge:                                                               │
│  1. Receives RPC request                                                         │
│  2. Encrypts message with E2EE (libolm/libvodozemac) if room is encrypted       │
│  3. Sends to Matrix homeserver via Client API                                   │
│  4. Returns event ID to client                                                  │
│                                                                                  │
│  STEP 4: BRIDGE RESPONSE                                                         │
│  ─────────────────────                                                           │
│  Response:                                                                       │
│  {                                                                               │
│    "jsonrpc": "2.0",                                                            │
│    "result": {                                                                   │
│      "event_id": "$event_xyz789",                                               │
│      "txn_id": "txn_1708123456789"                                              │
│    },                                                                           │
│    "id": "req_005"                                                              │
│  }                                                                              │
│                                                                                  │
│  STEP 5: LOCAL STATE UPDATE (Confirmed)                                          │
│  ─────────────────────────────────────────                                       │
│  ChatViewModel updates message:                                                  │
│  • Status: SENDING → SENT                                                        │
│  • Event ID: $event_xyz789                                                       │
│  • Single tick icon (✓)                                                          │
│                                                                                  │
│  STEP 6: DELIVERY CONFIRMATION VIA MATRIX /SYNC                                  │
│  ────────────────────────────────────────────────                                │
│  Matrix sync response includes the sent event (echo):                            │
│                                                                                  │
│  {                                                                               │
│    "next_batch": "batch_token_def456",                                          │
│    "rooms": {                                                                    │
│      "join": {                                                                   │
│        "!abc123:example.com": {                                                 │
│          "timeline": {                                                           │
│            "events": [                                                           │
│              {                                                                   │
│                "event_id": "$event_xyz789",                                     │
│                "type": "m.room.message",                                        │
│                "sender": "@alice:example.com",                                  │
│                "content": {"msgtype": "m.text", "body": "Hello!"},              │
│                "origin_server_ts": 1708123456789                                │
│              }                                                                   │
│            ]                                                                     │
│          }                                                                       │
│        }                                                                         │
│      }                                                                           │
│    }                                                                             │
│  }                                                                              │
│                                                                                  │
│  STEP 7: READ RECEIPT VIA MATRIX /SYNC                                           │
│  ─────────────────────────────────────                                           │
│  When recipient reads the message:                                               │
│                                                                                  │
│  {                                                                               │
│    "rooms": {                                                                    │
│      "join": {                                                                   │
│        "!abc123:example.com": {                                                 │
│          "ephemeral": {                                                          │
│            "events": [                                                           │
│              {                                                                   │
│                "type": "m.receipt",                                             │
│                "content": {                                                      │
│                  "$event_xyz789": {                                             │
│                    "m.read": {                                                   │
│                      "@bob:example.com": {"ts": 1708123457890}                  │
│                    }                                                             │
│                  }                                                               │
│                }                                                                 │
│              }                                                                   │
│            ]                                                                     │
│          }                                                                       │
│        }                                                                         │
│      }                                                                           │
│    }                                                                             │
│  }                                                                              │
│                                                                                  │
│  ChatViewModel updates message:                                                  │
│  • Status: SENT → READ                                                           │
│  • Double tick icon (✓✓)                                                         │
│                                                                                  │
└─────────────────────────────────────────────────────────────────────────────────┘
```

### 15.6 File/Media Upload Flow

When a user sends an image or file:

```
┌─────────────────────────────────────────────────────────────────────────────────┐
│                        FILE/MEDIA UPLOAD FLOW                                    │
├─────────────────────────────────────────────────────────────────────────────────┤
│                                                                                  │
│  USER SELECTS FILE                                                               │
│  ────────────────────                                                            │
│  User selects "vacation.jpg" (2.5 MB) to send                                   │
│                                                                                  │
│  STEP 1: LOCAL PREVIEW & COMPRESSION                                             │
│  ─────────────────────────────────────                                           │
│  • Generate thumbnail for preview                                                │
│  • Compress if needed (configurable quality)                                     │
│  • Display preview in UI                                                         │
│  • Show upload progress indicator                                                │
│  • Message status: UPLOADING                                                     │
│                                                                                  │
│  STEP 2: UPLOAD TO MATRIX MEDIA REPOSITORY (Direct)                              │
│  ────────────────────────────────────────────────────                            │
│  ArmorChat ──HTTPS──▶ POST /_matrix/media/v3/upload                              │
│                                                                                  │
│  Request:                                                                        │
│  POST https://matrix.example.com/_matrix/media/v3/upload                        │
│    ?access_token=syt_abc123...                                                  │
│    &filename=vacation.jpg                                                       │
│  Headers:                                                                        │
│    Content-Type: image/jpeg                                                     │
│    Content-Length: 2621440                                                      │
│  Body:                                                                           │
│    <binary file data>                                                           │
│                                                                                  │
│  Upload Progress:                                                                │
│  • Ktor tracks bytes sent                                                       │
│  • Progress emitted to UI (0% → 100%)                                           │
│  • User can cancel upload                                                       │
│                                                                                  │
│  STEP 3: MATRIX RESPONSE                                                         │
│  ─────────────────────                                                           │
│  Response:                                                                       │
│  {                                                                               │
│    "content_uri": "mxc://matrix.example.com/abc123def456"                       │
│  }                                                                              │
│                                                                                  │
│  STEP 4: GENERATE THUMBNAIL (Client-Side)                                        │
│  ────────────────────────────────────────                                        │
│  • Create thumbnail (320x180 or similar)                                        │
│  • Upload thumbnail to Matrix media repo                                        │
│  • Get thumbnail MXC URL                                                        │
│                                                                                  │
│  STEP 5: SEND MATRIX MESSAGE WITH ATTACHMENT                                     │
│  ─────────────────────────────────────────────                                   │
│  ArmorChat ──RPC───▶ matrix.send(roomId, "m.room.message", content)             │
│                                                                                  │
│  Request:                                                                        │
│  {                                                                               │
│    "jsonrpc": "2.0",                                                            │
│    "method": "matrix.send",                                                     │
│    "params": {                                                                   │
│      "room_id": "!abc123:example.com",                                          │
│      "event_type": "m.room.message",                                            │
│      "content": {                                                                │
│        "msgtype": "m.image",                                                    │
│        "body": "vacation.jpg",                                                  │
│        "url": "mxc://matrix.example.com/abc123def456",                          │
│        "info": {                                                                 │
│          "mimetype": "image/jpeg",                                              │
│          "size": 2621440,                                                       │
│          "w": 1920,                                                             │
│          "h": 1080,                                                             │
│          "thumbnail_url": "mxc://matrix.example.com/thumb_abc123",              │
│          "thumbnail_info": {                                                    │
│            "mimetype": "image/jpeg",                                            │
│            "size": 51200,                                                       │
│            "w": 320,                                                            │
│            "h": 180                                                             │
│          }                                                                       │
│        }                                                                         │
│      }                                                                           │
│    },                                                                           │
│    "id": "req_006"                                                              │
│  }                                                                              │
│                                                                                  │
│  STEP 6: LOCAL STATE UPDATE                                                      │
│  ───────────────────────                                                         │
│  • Message status: UPLOADING → SENT                                              │
│  • Image displayed with MXC URL                                                  │
│  • Thumbnail loaded from thumbnail_url                                           │
│                                                                                  │
│  STEP 7: ENCRYPTED ROOM HANDLING (If Applicable)                                 │
│  ────────────────────────────────────────────                                    │
│  For encrypted rooms:                                                            │
│  • Bridge encrypts file with room key                                           │
│  • Uses Matrix encrypted media format                                           │
│  • URL points to encrypted file                                                 │
│  • Key sent via m.room.encrypted event                                          │
│                                                                                  │
└─────────────────────────────────────────────────────────────────────────────────┘
```

### 15.7 Real-Time Events (Matrix /sync)

Real-time events come from Matrix /sync endpoint, NOT from WebSocket:

```
┌─────────────────────────────────────────────────────────────────────────────────┐
│                      MATRIX /SYNC EVENT TYPES                                    │
├─────────────────────────────────────────────────────────────────────────────────┤
│                                                                                  │
│  MatrixSyncManager polls: GET /_matrix/client/v3/sync?timeout=30000             │
│                                                                                  │
│  ─────────────────────────────────────────────────────────────────────────────  │
│  MESSAGE RECEIVED (Timeline Event)                                               │
│  ───────────────────────────────────                                             │
│  {                                                                               │
│    "rooms": {                                                                    │
│      "join": {                                                                   │
│        "!abc123:example.com": {                                                 │
│          "timeline": {                                                           │
│            "events": [                                                           │
│              {                                                                   │
│                "event_id": "$event_new123",                                     │
│                "type": "m.room.message",                                        │
│                "sender": "@bob:example.com",                                    │
│                "content": {                                                      │
│                  "msgtype": "m.text",                                           │
│                  "body": "Hey Alice!"                                           │
│                },                                                                │
│                "origin_server_ts": 1708123456789                                │
│              }                                                                   │
│            ]                                                                     │
│          }                                                                       │
│        }                                                                         │
│      }                                                                           │
│    }                                                                             │
│  }                                                                              │
│                                                                                  │
│  ─────────────────────────────────────────────────────────────────────────────  │
│  ENCRYPTED MESSAGE RECEIVED                                                      │
│  ──────────────────────────────────                                              │
│  {                                                                               │
│    "event_id": "$encrypted_event",                                              │
│    "type": "m.room.encrypted",                                                  │
│    "sender": "@bob:example.com",                                                │
│    "content": {                                                                  │
│      "algorithm": "m.megolm.v1.aes-sha2",                                       │
│      "ciphertext": "encrypted...",                                               │
│      "sender_key": "device_key...",                                              │
│      "session_id": "session_id..."                                               │
│    }                                                                             │
│  }                                                                              │
│  → Bridge decrypts using room key                                                │
│                                                                                  │
│  ─────────────────────────────────────────────────────────────────────────────  │
│  TYPING INDICATOR (Ephemeral Event)                                              │
│  ───────────────────────────────────                                             │
│  {                                                                               │
│    "rooms": {                                                                    │
│      "join": {                                                                   │
│        "!abc123:example.com": {                                                 │
│          "ephemeral": {                                                          │
│            "events": [                                                           │
│              {                                                                   │
│                "type": "m.typing",                                              │
│                "content": {                                                      │
│                  "user_ids": ["@bob:example.com", "@charlie:example.com"]       │
│                }                                                                 │
│              }                                                                   │
│            ]                                                                     │
│          }                                                                       │
│        }                                                                         │
│      }                                                                           │
│    }                                                                             │
│  }                                                                              │
│                                                                                  │
│  ─────────────────────────────────────────────────────────────────────────────  │
│  PRESENCE UPDATE (Top-Level Presence)                                            │
│  ────────────────────────────────────                                            │
│  {                                                                               │
│    "presence": {                                                                 │
│      "events": [                                                                 │
│        {                                                                         │
│          "type": "m.presence",                                                  │
│          "sender": "@bob:example.com",                                          │
│          "content": {                                                            │
│            "presence": "online",                                                │
│            "status_msg": "Working from home",                                    │
│            "last_active_ago": 60000                                              │
│          }                                                                       │
│        }                                                                         │
│      ]                                                                           │
│    }                                                                             │
│  }                                                                              │
│                                                                                  │
│  ─────────────────────────────────────────────────────────────────────────────  │
│  READ RECEIPT (Ephemeral Event)                                                  │
│  ────────────────────────────────                                                │
│  {                                                                               │
│    "rooms": {                                                                    │
│      "join": {                                                                   │
│        "!abc123:example.com": {                                                 │
│          "ephemeral": {                                                          │
│            "events": [                                                           │
│              {                                                                   │
│                "type": "m.receipt",                                             │
│                "content": {                                                      │
│                  "$event_xyz789": {                                             │
│                    "m.read": {                                                   │
│                      "@bob:example.com": {"ts": 1708123457890}                  │
│                    }                                                             │
│                  }                                                               │
│                }                                                                 │
│              }                                                                   │
│            ]                                                                     │
│          }                                                                       │
│        }                                                                         │
│      }                                                                           │
│    }                                                                             │
│  }                                                                              │
│                                                                                  │
│  ─────────────────────────────────────────────────────────────────────────────  │
│  ROOM MEMBERSHIP CHANGE (State Event)                                            │
│  ────────────────────────────────────                                            │
│  {                                                                               │
│    "rooms": {                                                                    │
│      "join": {                                                                   │
│        "!abc123:example.com": {                                                 │
│          "timeline": {                                                           │
│            "events": [                                                           │
│              {                                                                   │
│                "type": "m.room.member",                                         │
│                "state_key": "@dave:example.com",                                │
│                "sender": "@alice:example.com",                                  │
│                "content": {                                                      │
│                  "membership": "join",                                          │
│                  "displayname": "Dave"                                          │
│                }                                                                 │
│              }                                                                   │
│            ]                                                                     │
│          }                                                                       │
│        }                                                                         │
│      }                                                                           │
│    }                                                                             │
│  }                                                                              │
│                                                                                  │
│  ─────────────────────────────────────────────────────────────────────────────  │
│  CALL SIGNALING (Timeline Event - Vo6)                                           │
│  ─────────────────────────────────────────                                       │
│  {                                                                               │
│    "event_id": "$call_invite_123",                                              │
│    "type": "m.call.invite",                                                     │
│    "sender": "@bob:example.com",                                                │
│    "content": {                                                                  │
│      "call_id": "call_abc123",                                                  │
│      "version": "1",                                                            │
│      "lifetime": 60000,                                                          │
│      "offer": {                                                                  │
│        "type": "offer",                                                          │
│        "sdp": "v=0\r\no=- 4611731400430051336 2 IN IP4 127.0.0.1..."            │
│      }                                                                           │
│    }                                                                             │
│  }                                                                              │
│                                                                                  │
│  Also: m.call.candidates, m.call.answer, m.call.hangup                          │
│                                                                                  │
│  ─────────────────────────────────────────────────────────────────────────────  │
│  ROOM INVITE (Invite Section)                                                    │
│  ────────────────────────────                                                    │
│  {                                                                               │
│    "rooms": {                                                                    │
│      "invite": {                                                                 │
│        "!newroom:example.com": {                                                │
│          "invite_state": {                                                       │
│            "events": [                                                           │
│              {                                                                   │
│                "type": "m.room.member",                                         │
│                "state_key": "@alice:example.com",                               │
│                "sender": "@bob:example.com",                                    │
│                "content": {"membership": "invite"}                              │
│              }                                                                   │
│            ]                                                                     │
│          }                                                                       │
│        }                                                                         │
│      }                                                                           │
│    }                                                                             │
│  }                                                                              │
│                                                                                  │
│  ─────────────────────────────────────────────────────────────────────────────  │
│  DEVICE LIST CHANGES (For E2EE)                                                  │
│  ──────────────────────────────────                                              │
│  {                                                                               │
│    "device_lists": {                                                             │
│      "changed": ["@bob:example.com"],                                           │
│      "left": ["@charlie:example.com"]                                           │
│    },                                                                           │
│    "device_one_time_keys_count": {                                              │
│      "signed_curve25519": 50                                                    │
│    }                                                                            │
│  }                                                                              │
│                                                                                  │
│  ─────────────────────────────────────────────────────────────────────────────  │
│  TO-DEVICE MESSAGES (For E2EE Key Exchange)                                      │
│  ───────────────────────────────────────────                                     │
│  {                                                                               │
│    "to_device": {                                                                │
│      "events": [                                                                 │
│        {                                                                         │
│          "type": "m.room_key",                                                  │
│          "sender": "@bob:example.com",                                          │
│          "content": {                                                            │
│            "algorithm": "m.megolm.v1.aes-sha2",                                 │
│            "room_id": "!abc123:example.com",                                    │
│            "session_key": "..."                                                  │
│          }                                                                       │
│        }                                                                         │
│      ]                                                                           │
│    }                                                                             │
│  }                                                                              │
│                                                                                  │
└─────────────────────────────────────────────────────────────────────────────────┘
```

### 15.8 MatrixSyncManager Events

The MatrixSyncManager converts raw Matrix sync events to typed Kotlin events:

```kotlin
sealed class MatrixSyncEvent {
    // Messages
    data class MessageReceived(
        val roomId: String,
        val event: MatrixEventRaw
    ) : MatrixSyncEvent()

    // Typing
    data class TypingNotification(
        val roomId: String,
        val userIds: List<String>,
        val event: MatrixEventRaw
    ) : MatrixSyncEvent()

    // Presence
    data class PresenceUpdate(
        val event: MatrixEventRaw
    ) : MatrixSyncEvent()

    // Receipts
    data class ReceiptEvent(
        val roomId: String,
        val event: MatrixEventRaw
    ) : MatrixSyncEvent()

    // Room membership
    data class RoomMembership(
        val roomId: String,
        val userId: String,
        val membership: String,
        val event: MatrixEventRaw
    ) : MatrixSyncEvent()

    // Room state changes
    data class RoomNameChanged(val roomId: String, val name: String?, ...)
    data class RoomTopicChanged(val roomId: String, val topic: String?, ...)
    data class RoomAvatarChanged(val roomId: String, val avatarUrl: String?, ...)
    data class RoomEncryptionEnabled(val roomId: String, ...)

    // Calls
    data class CallSignaling(
        val roomId: String,
        val eventType: String,
        val callId: String,
        val event: MatrixEventRaw
    ) : MatrixSyncEvent()

    // Errors
    data class SyncError(val error: Throwable) : MatrixSyncEvent()
}
```

### 15.9 Platform Integration Flow

Connecting external platforms (Slack, Discord, Teams):

```
┌─────────────────────────────────────────────────────────────────────────────────┐
│                    PLATFORM INTEGRATION FLOW                                     │
├─────────────────────────────────────────────────────────────────────────────────┤
│                                                                                  │
│  USER WANTS TO CONNECT SLACK                                                     │
│  ───────────────────────────────────                                             │
│                                                                                  │
│  STEP 1: INITIATE CONNECTION                                                     │
│  ────────────────────────────                                                    │
│  ArmorChat ──RPC───▶ platform.connect("slack", config)                          │
│                                                                                  │
│  Request:                                                                        │
│  {                                                                               │
│    "jsonrpc": "2.0",                                                            │
│    "method": "platform.connect",                                                │
│    "params": {                                                                   │
│      "platform_type": "slack",                                                  │
│      "config": {                                                                 │
│        "workspace": "mycompany",                                                │
│        "channels": ["general", "random"]                                        │
│      }                                                                           │
│    },                                                                           │
│    "id": "req_007"                                                              │
│  }                                                                              │
│                                                                                  │
│  STEP 2: OAUTH FLOW                                                              │
│  ───────────────────                                                             │
│  Response:                                                                       │
│  {                                                                               │
│    "jsonrpc": "2.0",                                                            │
│    "result": {                                                                   │
│      "success": false,                                                          │
│      "auth_url": "https://slack.com/oauth/authorize?client_id=...",             │
│      "platform_id": null,                                                       │
│      "message": "Authorization required"                                        │
│    },                                                                           │
│    "id": "req_007"                                                              │
│  }                                                                              │
│                                                                                  │
│  STEP 3: USER AUTHORIZES IN BROWSER                                              │
│  ──────────────────────────────────────                                          │
│  • ArmorChat opens auth_url in browser                                          │
│  • User logs into Slack and authorizes                                          │
│  • Slack redirects to callback URL on Bridge                                    │
│  • Bridge receives callback with auth code                                      │
│  • Bridge exchanges code for access token                                       │
│                                                                                  │
│  STEP 4: CONNECTION COMPLETE (Polling or Next RPC Call)                          │
│  ────────────────────────────────────────────────────────                        │
│  Check connection status:                                                       │
│  ArmorChat ──RPC───▶ platform.list()                                            │
│                                                                                  │
│  Response:                                                                       │
│  {                                                                               │
│    "jsonrpc": "2.0",                                                            │
│    "result": {                                                                   │
│      "platforms": [                                                              │
│        {                                                                         │
│          "id": "slack_mycompany",                                               │
│          "type": "slack",                                                       │
│          "name": "MyCompany Workspace",                                         │
│          "status": "connected",                                                 │
│          "connected_at": 1708123460000,                                         │
│          "last_sync": 1708123470000                                             │
│        }                                                                         │
│      ]                                                                           │
│    },                                                                           │
│    "id": "req_008"                                                              │
│  }                                                                              │
│                                                                                  │
│  STEP 5: BIDIRECTIONAL MESSAGE FLOW                                              │
│  ──────────────────────────────────────                                          │
│                                                                                  │
│  Slack → Matrix:                                                                 │
│  • User posts in #general on Slack                                              │
│  • Bridge receives via Slack Events API                                         │
│  • Bridge creates Matrix event in !slack_general:example.com                    │
│  • ArmorChat receives via Matrix /sync (NOT WebSocket)                          │
│                                                                                  │
│  Matrix → Slack:                                                                 │
│  • User posts in !slack_general:example.com                                     │
│  • Bridge receives via Matrix appservice                                        │
│  • Bridge sends to Slack #general via Web API                                   │
│  • Message appears in Slack                                                     │
│                                                                                  │
└─────────────────────────────────────────────────────────────────────────────────┘
```

### 15.10 RPC Method Reference

All RPC methods available in BridgeRpcClient. **Note: Methods use snake_case to match Bridge API.**

> **Migration Status:** ✅ COMPLETE
> Matrix methods (login, sync, send, rooms) are **DEPRECATED** - use `MatrixClient` instead.
> Only use Bridge RPC for **admin operations** (lifecycle, recovery, platforms, push, license).

#### ✅ Available Methods (Bridge Admin)

| Kotlin Method | RPC Method | Category | Description |
|---------------|------------|----------|-------------|
| `startBridge()` | `bridge.start` | Lifecycle | Start bridge session |
| `stopBridge()` | `bridge.stop` | Lifecycle | Stop bridge session |
| `getBridgeStatus()` | `bridge.status` | Lifecycle | Get session status |
| `healthCheck()` | `bridge.health` | Lifecycle | Health check |
| `recoveryGeneratePhrase()` | `recovery.generate_phrase` | Recovery | Generate BIP39 phrase |
| `recoveryStorePhrase()` | `recovery.store_phrase` | Recovery | Store encrypted phrase |
| `recoveryVerify()` | `recovery.verify` | Recovery | Verify phrase |
| `recoveryStatus()` | `recovery.status` | Recovery | Get recovery status |
| `recoveryComplete()` | `recovery.complete` | Recovery | Complete recovery |
| `recoveryIsDeviceValid()` | `recovery.is_device_valid` | Recovery | Check device validity |
| `platformConnect()` | `platform.connect` | Platforms | Connect platform |
| `platformDisconnect()` | `platform.disconnect` | Platforms | Disconnect platform |
| `platformList()` | `platform.list` | Platforms | List platforms |
| `platformStatus()` | `platform.status` | Platforms | Get platform status |
| `platformTest()` | `platform.test` | Platforms | Test connection |
| `pushRegister()` | `push.register_token` | Push | Register FCM/APNs |
| `pushUnregister()` | `push.unregister_token` | Push | Unregister push |
| `pushUpdateSettings()` | `push.update_settings` | Push | Update settings |
| `licenseStatus()` | `license.status` | License | Get license status |
| `licenseFeatures()` | `license.features` | License | Get features map |
| `complianceStatus()` | `compliance.status` | Compliance | Get compliance mode |
| `platformLimits()` | `platform.limits` | Compliance | Get platform limits |
| `getErrors()` | `get_errors` | Errors | Get recent errors |
| `resolveError()` | `resolve_error` | Errors | Resolve error |

#### ⚠️ Deprecated Methods (Use MatrixClient)

| Kotlin Method | RPC Method | Replacement |
|---------------|------------|-------------|
| `matrixLogin()` | `matrix.login` | `MatrixClient.login()` |
| `matrixSync()` | `matrix.sync` | `MatrixClient.startSync()` |
| `matrixSend()` | `matrix.send` | `MatrixClient.sendTextMessage()` |
| `matrixRefreshToken()` | `matrix.refresh_token` | SDK handles automatically |
| `matrixCreateRoom()` | `matrix.create_room` | `MatrixClient.createRoom()` |
| `matrixJoinRoom()` | `matrix.join_room` | `MatrixClient.joinRoom()` |
| `matrixLeaveRoom()` | `matrix.leave_room` | `MatrixClient.leaveRoom()` |
| `matrixInviteUser()` | `matrix.invite` | `MatrixClient.inviteUser()` |
| `matrixSendTyping()` | `matrix.typing` | `MatrixClient.sendTyping()` |
| `matrixSendReadReceipt()` | `matrix.read_receipt` | `MatrixClient.sendReadReceipt()` |

### 15.11 Key Implementation Classes

| Class | Package | Purpose |
|-------|---------|---------|
| **`MatrixClient`** | `platform.matrix` | **Primary**: Matrix protocol operations (40+ methods) |
| **`MatrixSyncManager`** | `platform.matrix` | **Primary**: Matrix /sync long-poll for real-time events |
| `MatrixSessionStorage` | `platform.matrix` | Encrypted session persistence |
| `ControlPlaneStore` | `data.store` | ArmorClaw event processing |
| `BridgeAdminClient` | `platform.bridge` | Admin-only RPC interface (22 methods) |
| `BridgeRpcClient` | `platform.bridge` | Full RPC interface (includes deprecated methods) |
| `BridgeRpcClientImpl` | `platform.bridge` | RPC implementation with retry logic |
| `BridgeRepository` | `platform.bridge` | Integration layer (domain ↔ bridge) |
| `RealTimeEventStore` | `data.store` | Event distribution to UI |
| `CheckFeatureUseCase` | `domain.usecase` | Feature availability with caching |
| `SetupService` | `domain.service` | First-time setup flow |
| `UnifiedMessage` | `domain.model` | Message model for all types (Regular, Agent, System, Command) |
| `AppNavigation` | `app` | All navigation routes (40 routes) |
| `MatrixClient` | `platform.matrix` | Direct Matrix API |
| `BridgeConfig` | `platform.bridge` | Configuration |
| `BridgeWebSocketClient` | `platform.bridge` | WebSocket (stub - not used) |

### 15.12 Error Handling & Fallbacks

```
┌─────────────────────────────────────────────────────────────────────────────────┐
│                    ERROR HANDLING & FALLBACKS                                    │
├─────────────────────────────────────────────────────────────────────────────────┤
│                                                                                  │
│  CONNECTION ERRORS                                                               │
│  ───────────────────                                                             │
│                                                                                  │
│  Scenario: Primary bridge server unreachable                                     │
│                                                                                  │
│  1. ArmorChat detects connection failure                                         │
│  2. Shows "Connecting..." indicator                                              │
│  3. Attempts reconnection with exponential backoff:                              │
│     • Attempt 1: Wait 1s                                                         │
│     • Attempt 2: Wait 2s                                                         │
│     • Attempt 3: Wait 4s                                                         │
│     • ... up to 30s max                                                          │
│  4. After 3 failed attempts, try fallback servers:                               │
│     • https://bridge-backup.armorclaw.app                                        │
│     • https://bridge-eu.armorclaw.app                                            │
│  5. If all fail, show "Offline Mode" banner                                      │
│  6. Queue operations in offline queue                                            │
│  7. Sync when connection restored                                                │
│                                                                                  │
│  RPC ERRORS                                                                      │
│  ──────────                                                                      │
│                                                                                  │
│  JSON-RPC Error Codes:                                                           │
│  • -32700: Parse error (invalid JSON)                                            │
│  • -32600: Invalid Request                                                       │
│  • -32601: Method not found                                                      │
│  • -32602: Invalid params                                                        │
│  • -32603: Internal error                                                        │
│  • -32001: Session expired                                                       │
│  • -32002: Authentication failed                                                 │
│  • -32003: Network error                                                         │
│                                                                                  │
│  Handling:                                                                       │
│  • Session expired → Re-authenticate                                             │
│  • Network error → Queue for retry                                               │
│  • Invalid params → Show user error                                              │
│  • Method not found → Log and degrade gracefully                                 │
│                                                                                  │
│  MESSAGE SEND FAILURES                                                           │
│  ─────────────────────────                                                       │
│                                                                                  │
│  1. Message queued locally with status FAILED                                    │
│  2. Show retry button in UI                                                      │
│  3. User can tap to retry                                                        │
│  4. Or auto-retry when connection restored                                       │
│  5. After max retries (3), mark as permanently failed                            │
│  6. User can delete or retry manually                                            │
│                                                                                  │
│  FILE UPLOAD FAILURES                                                            │
│  ────────────────────────────                                                    │
│                                                                                  │
│  1. Upload paused, shows "Retry" button                                          │
│  2. Supports resume for large files                                              │
│  3. Chunk-based upload with progress tracking                                    │
│  4. Failed chunks retried individually                                           │
│  5. After complete failure, show "Upload failed" with retry option               │
│                                                                                  │
└─────────────────────────────────────────────────────────────────────────────────┘
```

### 15.13 Configuration Classes

```kotlin
// Setup Configuration (Shared)
data class SetupConfig(
    val homeserver: String = "",
    val bridgeUrl: String? = null,        // Bridge RPC URL: /api (NOT /rpc!)
    val serverVersion: String? = null,
    val supportsE2EE: Boolean = true,
    val supportsRecovery: Boolean = true,
    val detectedRegion: String? = null,
    val isDemo: Boolean = false,
    val configSource: ConfigSource = ConfigSource.DEFAULT,
    val wsUrl: String? = null,            // WebSocket URL
    val pushGateway: String? = null,      // Push gateway URL
    val serverName: String? = null,
    val expiresAt: Long? = null
) {
    companion object {
        fun createDefault(): SetupConfig = SetupConfig(
            homeserver = "https://matrix.armorclaw.app",
            bridgeUrl = "https://bridge.armorclaw.app/api",  // ⚠️ /api NOT /rpc
            wsUrl = "wss://bridge.armorclaw.app/ws",
            pushGateway = "https://bridge.armorclaw.app/_matrix/push/v1/notify",
            serverName = "ArmorClaw",
            configSource = ConfigSource.DEFAULT
        )

        fun createDebug(): SetupConfig = SetupConfig(
            homeserver = "http://10.0.2.2:8008",
            bridgeUrl = "http://10.0.2.2:8080/api",  // ⚠️ /api NOT /rpc
            wsUrl = "ws://10.0.2.2:8080/ws",
            pushGateway = "http://10.0.2.2:8080/_matrix/push/v1/notify",
            serverName = "Debug Server",
            configSource = ConfigSource.DEFAULT,
            isDemo = true
        )
    }
}

// Bridge Configuration (Legacy - kept for compatibility)
data class BridgeConfig(
    val baseUrl: String,                    // Bridge RPC URL
    val homeserverUrl: String,              // Matrix homeserver URL
    val wsUrl: String? = null,
    val pushGateway: String? = null,
    val timeoutMs: Long = 30000,
    val enableCertificatePinning: Boolean = true,
    val certificatePins: List<String> = emptyList(),
    val retryCount: Int = 3,
    val retryDelayMs: Long = 1000,
    val useDirectMatrixSync: Boolean = true
) {
    companion object {
        val PRODUCTION = BridgeConfig(
            baseUrl = "https://bridge.armorclaw.app/api",  // ⚠️ /api NOT /rpc
            homeserverUrl = "https://matrix.armorclaw.app",
            wsUrl = "wss://bridge.armorclaw.app/ws",
            pushGateway = "https://bridge.armorclaw.app/_matrix/push/v1/notify",
            timeoutMs = 30000,
            enableCertificatePinning = true,
            useDirectMatrixSync = true
        )

        val DEVELOPMENT = BridgeConfig(
            baseUrl = "http://10.0.2.2:8080/api",      // ⚠️ /api NOT /rpc
            homeserverUrl = "http://10.0.2.2:8008",
            wsUrl = "ws://10.0.2.2:8080/ws",
            pushGateway = "http://10.0.2.2:8080/_matrix/push/v1/notify",
            timeoutMs = 60000,
            enableCertificatePinning = false,
            retryCount = 1,
            useDirectMatrixSync = true
        )
    }
}
```

### 15.14 URL Reference Table

| Service | Production URL | Development URL |
|---------|---------------|-----------------|
| Matrix Homeserver | `https://matrix.armorclaw.app` | `http://10.0.2.2:8008` |
| Bridge RPC | `https://bridge.armorclaw.app/api` | `http://10.0.2.2:8080/api` |
| Bridge WebSocket | `wss://bridge.armorclaw.app/ws` | `ws://10.0.2.2:8080/ws` |
| Push Gateway | `https://bridge.armorclaw.app/_matrix/push/v1/notify` | `http://10.0.2.2:8080/_matrix/push/v1/notify` |
| Well-Known | `https://armorclaw.app/.well-known/matrix/client` | N/A |
| QR Config | `https://bridge.armorclaw.app/qr/config` | `http://10.0.2.2:8080/qr/config` |
| Discovery | `https://bridge.armorclaw.app/discover` | `http://10.0.2.2:8080/discover` |

### 15.15 Android Emulator Localhost

When developing with Android emulator, use these addresses:

| Service | Address | Notes |
|---------|---------|-------|
| Bridge RPC (local) | `http://10.0.2.2:8080/api` | Emulator's localhost mapping |
| Bridge WebSocket (local) | `ws://10.0.2.2:8080/ws` | ArmorTerminal only |
| Matrix Server (local) | `http://10.0.2.2:8008` | Matrix API |
| Matrix Media (local) | `http://10.0.2.2:8008/_matrix/media/` | Media uploads |
| Discovery (local) | `http://10.0.2.2:8080/discover` | Server discovery |
| QR Config (local) | `http://10.0.2.2:8080/qr/config` | QR generation |

### 15.16 Security During Communication

| Security Layer | Implementation |
|----------------|----------------|
| Transport | TLS 1.3 with certificate pinning |
| Authentication | Bearer token + session ID |
| Message Encryption | E2EE via Matrix (libolm/libvodozemac) |
| Session Encryption | AES-256 encrypted session storage (SQLCipher) |
| Request Signing | Correlation IDs + trace IDs for tracing |
| Local Storage | SQLCipher with biometric-protected key |
| QR Config Signing | HMAC signature from Bridge |

### 15.17 Communication Summary

```
┌─────────────────────────────────────────────────────────────────────────────────┐
│                    COMMUNICATION QUICK REFERENCE                                 │
├─────────────────────────────────────────────────────────────────────────────────┤
│                                                                                  │
│  ┌─────────────────────────────────────────────────────────────────────────┐    │
│  │  WHAT ARMORCHAT USES FOR EACH OPERATION                                  │    │
│  ├─────────────────────────────────────────────────────────────────────────┤    │
│  │                                                                          │    │
│  │  Operation                    │ Channel        │ Endpoint               │    │
│  │  ─────────────────────────────┼────────────────┼─────────────────────── │    │
│  │  Discovery                    │ Multiple       │ See 15.1               │    │
│  │  Login (Auth)                 │ Matrix SDK     │ /_matrix/client/login  │    │
│  │  Send Message                 │ Matrix SDK     │ /rooms/.../send (E2E)  │    │
│  │  Receive Message              │ Matrix /sync   │ /_matrix/client/sync   │    │
│  │  Create/Join/Leave Room       │ Matrix SDK     │ /_matrix/client/...    │    │
│  │  Typing/Read Receipts         │ Matrix SDK     │ /_matrix/client/...    │    │
│  │  Upload File/Media            │ Matrix Media   │ /_matrix/media/upload  │    │
│  │  Real-time Events             │ Matrix /sync   │ Long-poll (30s)        │    │
│  │  WebRTC Call Signaling        │ Matrix Events  │ m.call.* in timeline   │    │
│  │  ─────────────────────────────┼────────────────┼─────────────────────── │    │
│  │  Start/Stop Bridge            │ RPC            │ bridge.start/stop      │    │
│  │  Platform Connect             │ RPC            │ platform.connect       │    │
│  │  Push Notifications           │ RPC            │ push.register_token    │    │
│  │  Recovery                     │ RPC            │ recovery.*             │    │
│  │  License Check                │ RPC            │ license.status         │    │
│  │  QR Generation                │ RPC            │ qr.config              │    │
│  │                                                                          │    │
│  │  ✅ PRIMARY: Matrix Protocol (E2E Encrypted)                             │    │
│  │  🔧 ADMIN:   Bridge RPC /api (JSON-RPC 2.0)                              │    │
│  │                                                                          │    │
│  └─────────────────────────────────────────────────────────────────────────┘    │
│                                                                                  │
└─────────────────────────────────────────────────────────────────────────────────┘
```

### 15.18 Complete Data Flow Example

```
┌─────────────────────────────────────────────────────────────────────────────────┐
│                    MESSAGE SENDING FLOW (End-to-End)                             │
├─────────────────────────────────────────────────────────────────────────────────┤
│                                                                                  │
│  USER SENDS MESSAGE IN ARMORCHAT                                                 │
│                                                                                  │
│  1. USER TYPES MESSAGE                                                           │
│     ┌──────────────────────────────────────────────────────────────────────┐    │
│     │  ChatScreen.kt                                                       │    │
│     │  onSendMessage(text: String)                                         │    │
│     │      │                                                               │    │
│     │      ▼                                                               │    │
│     │  ChatViewModel.onAction(ChatAction.SendMessage)                      │    │
│     │      │                                                               │    │
│     │      ▼                                                               │    │
│     │  Create local message (status: SENDING)                              │    │
│     └──────────────────────────────────────────────────────────────────────┘    │
│                                                                                  │
│  2. MESSAGE ENCRYPTION (E2EE)                                                    │
│     ┌──────────────────────────────────────────────────────────────────────┐    │
│     │  MatrixClient.sendMessage()                                          │    │
│     │      │                                                               │    │
│     │      ▼                                                               │    │
│     │  Encrypt message with Megolm (room key)                              │    │
│     │      │                                                               │    │
│     │      ▼                                                               │    │
│     │  Create m.room.encrypted event                                       │    │
│     └──────────────────────────────────────────────────────────────────────┘    │
│                                                                                  │
│  3. SEND TO MATRIX SERVER                                                        │
│     ┌──────────────────────────────────────────────────────────────────────┐    │
│     │  POST https://matrix.armorclaw.app/_matrix/client/v3/rooms/.../send  │    │
│     │  Body: { type: "m.room.encrypted", content: { ... } }                │    │
│     │      │                                                               │    │
│     │      ▼                                                               │    │
│     │  Response: { event_id: "$abc123" }                                   │    │
│     └──────────────────────────────────────────────────────────────────────┘    │
│                                                                                  │
│  4. RECIPIENT RECEIVES MESSAGE                                                   │
│     ┌──────────────────────────────────────────────────────────────────────┐    │
│     │  GET https://matrix.armorclaw.app/_matrix/client/v3/sync             │    │
│     │  (Long-poll, recipient's ArmorChat app)                              │    │
│     │      │                                                               │    │
│     │      ▼                                                               │    │
│     │  Response includes m.room.encrypted event                            │    │
│     │      │                                                               │    │
│     │      ▼                                                               │    │
│     │  MatrixSyncManager processes event                                   │    │
│     │      │                                                               │    │
│     │      ▼                                                               │    │
│     │  Decrypt message with room key (Megolm)                              │    │
│     │      │                                                               │    │
│     │      ▼                                                               │    │
│     │  Display decrypted message in ChatScreen                             │    │
│     └──────────────────────────────────────────────────────────────────────┘    │
│                                                                                  │
│  ⚠️  IMPORTANT: Bridge RPC is NOT used for message sending/receiving            │
│     Messages go directly through Matrix protocol (E2E encrypted)                │
│                                                                                  │
└─────────────────────────────────────────────────────────────────────────────────┘
```

---

## 16. Quick Reference

### File Locations

| What | Location |
|------|----------|
| Navigation routes | `app/.../navigation/ArmorClawNavHost.kt` |
| Deep link handler | `app/.../navigation/DeepLinkHandler.kt` |
| ViewModels | `app/.../viewmodel/` |
| Screens | `app/.../ui/` |
| Domain models | `app/.../domain/model/` |
| Repository interfaces | `app/.../domain/repository/` |
| Setup service | `app/.../platform/bridge/SetupService.kt` |
| Setup config | `app/.../platform/bridge/SetupService.kt` |
| Matrix client | `app/.../platform/matrix/MatrixClient.kt` |
| Matrix sync manager | `app/.../platform/matrix/MatrixSyncManager.kt` |
| Bridge client | `app/.../platform/bridge/BridgeRpcClient.kt` |
| Bridge RPC impl | `app/.../platform/bridge/BridgeRpcClientImpl.kt` |
| Bridge repository | `app/.../platform/bridge/BridgeRepository.kt` |
| Real-time event store | `app/.../data/store/RealTimeEventStore.kt` |
| Check feature use case | `app/.../domain/usecase/CheckFeatureUseCase.kt` |
| Control plane | `app/.../data/store/ControlPlaneStore.kt` |
| Unified message | `app/.../domain/model/UnifiedMessage.kt` |
| RPC models | `app/.../platform/bridge/RpcModels.kt` |
| Android Manifest | `app/src/main/AndroidManifest.xml` |

### Key Classes

| Class | Package | Purpose |
|-------|---------|---------|
| **`SetupService`** | `platform.bridge` | **Discovery**: Server discovery, signed config, setup flow |
| **`SetupConfig`** | `platform.bridge` | Server configuration (homeserver, bridgeUrl, etc.) |
| **`ConfigSource`** | `platform.bridge` | How config was obtained (SIGNED_URL, MDNS, etc.) |
| **`MatrixClient`** | `platform.matrix` | **Primary**: Matrix protocol operations (40+ methods) |
| **`MatrixSyncManager`** | `platform.matrix` | **Primary**: Matrix /sync long-poll for real-time events |
| `MatrixSessionStorage` | `platform.matrix` | Encrypted session persistence |
| `ControlPlaneStore` | `data.store` | ArmorClaw event processing |
| `BridgeAdminClient` | `platform.bridge` | Admin-only RPC interface (22 methods) |
| `BridgeRpcClient` | `platform.bridge` | Full RPC interface |
| `BridgeRpcClientImpl` | `platform.bridge` | RPC implementation with retry logic |
| `BridgeRepository` | `platform.bridge` | Integration layer (domain ↔ bridge) |
| `RealTimeEventStore` | `data.store` | Event distribution to UI |
| `CheckFeatureUseCase` | `domain.usecase` | Feature availability with caching |
| `UnifiedMessage` | `domain.model` | Message model (Regular, Agent, System, Command) |
| `AppNavigation` | `app` | All navigation routes (40 routes) |
| `DeepLinkHandler` | `app` | Deep link parsing and routing |
| `ChatViewModel` | `app` | Chat screen state (unified messages) |
| `HomeViewModel` | `app` | Home screen state (rooms + workflows) |

### Related Documentation

| Document | Location | Purpose |
|----------|----------|---------|
| **Matrix Migration** | `doc/MATRIX_MIGRATION.md` | **Primary**: RPC → Matrix migration guide (✅ COMPLETE) |
| Communication Fixes | `doc/COMMUNICATION_FIXES.md` | Details on RPC/Matrix sync implementation |
| Architecture | `doc/ARCHITECTURE.md` | System architecture overview |
| Security | `doc/SECURITY.md` | Security implementation details |
| API Reference | `doc/API.md` | Public API documentation |

### Bridge Server Endpoints

| Endpoint | Method | Purpose |
|----------|--------|---------|
| `/api` | POST | JSON-RPC 2.0 API (all RPC methods) |
| `/ws` | WebSocket | ArmorTerminal only (optional, Matrix sync preferred) |
| `/health` | GET | Health check |
| `/discover` | GET | Server discovery info |
| `/.well-known/matrix/client` | GET | Matrix well-known discovery |
| `/qr/config` | GET | Generate signed config QR |
| `/qr/image` | GET | QR code image |
| `/_matrix/push/v1/notify` | POST | Push gateway |

### Deep Link Formats

| Scheme | Format | Purpose |
|--------|--------|---------|
| Config | `armorclaw://config?d=<base64>` | Signed server configuration |
| Setup | `armorclaw://setup?token=xxx&server=xxx` | Device setup with token |
| Invite | `armorclaw://invite?code=xxx` | Room/server invite |
| Bond | `armorclaw://bond?token=xxx` | Device bonding (admin pairing) |
| Web Config | `https://armorclaw.app/config?d=<base64>` | Web-based config |
| Web Invite | `https://armorclaw.app/invite/<code>` | Web-based invite |

---

*This document provides a comprehensive overview of the ArmorChat project. For implementation details, see the source code and additional documentation in the `doc/` directory.*

**Document Version:** 3.3
**Last Updated:** 2026-02-22
**Matrix Migration:** ✅ COMPLETE
**Discovery System:** ✅ ENHANCED
**Governor Strategy:** ✅ COMPLETE

---

## 17. Governor Strategy

> **Status:** ✅ COMPLETE (All 4 Phases)
> **Version:** 4.0.0-alpha01
> **Completion Date:** 2026-02-22

The Governor Strategy transforms ArmorChat from a messaging client to a **secure, authoritative controller** for ArmorClaw agents. The app becomes the "Governor" - authorizing, monitoring, and revoking agent actions.

### Architecture Overview

```
┌─────────────────────────────────────────────────────────────────┐
│                    ARMORCLAW GOVERNOR STACK                     │
├─────────────────────────────────────────────────────────────────┤
│                                                                  │
│  PHASE 4: COMMERCIAL POLISH                                     │
│  ┌─────────────────────────────────────────────────────────────┐│
│  │ ArmorClawTheme │ StatusIcons │ RiskLevelBadge │ StatusBar  ││
│  └─────────────────────────────────────────────────────────────┘│
│                              │                                   │
│  PHASE 3: AUDIT & TRANSPARENCY                                   │
│  ┌─────────────────────────────────────────────────────────────┐│
│  │ ArmorTerminal │ TaskReceipt │ RevocationPanel │ PiiAccess  ││
│  └─────────────────────────────────────────────────────────────┘│
│                              │                                   │
│  PHASE 2: GOVERNOR UI                                           │
│  ┌─────────────────────────────────────────────────────────────┐│
│  │ CommandBlock │ CapabilityRibbon │ HITLAuthorization         ││
│  └─────────────────────────────────────────────────────────────┘│
│                              │                                   │
│  PHASE 1: COLD VAULT                                            │
│  ┌─────────────────────────────────────────────────────────────┐│
│  │ SQLCipher │ KeystoreManager │ VaultRepository │ ShadowMap  ││
│  └─────────────────────────────────────────────────────────────┘│
│                                                                  │
└─────────────────────────────────────────────────────────────────┘
```

### Phase 1: Cold Vault (Identity & Security) ✅

**Goal:** Encrypt all PII at rest using SQLCipher via SQLDelight 2.0.0

| Component | Status | Location |
|-----------|--------|----------|
| SQLCipher Integration | ✅ Complete | `app/.../security/` |
| KeystoreManager | ✅ Complete | Hardware-backed key management |
| SqlCipherProvider | ✅ Complete | Encrypted database factory |
| VaultRepository | ✅ Complete | PII CRUD operations |
| PiiRegistry | ✅ Complete | PII key registry |
| ShadowMap | ✅ Complete | Placeholder mapping |
| AgentRequestInterceptor | ✅ Complete | Outbound request middleware |

**Shadow Mapping:**
- Placeholder format: `{{VAULT:field_name:hash}}`
- Replaces raw PII before agent transmission
- Hash verification for integrity

### Phase 2: Governor UI ✅

**Goal:** Technical "Command Block" UI and capability visualization

| Component | Status | Location |
|-----------|--------|----------|
| CommandBlock | ✅ Complete | `app/.../governor/` |
| CommandStatusBadge | ✅ Complete | Status indicators |
| CapabilityRibbon | ✅ Complete | Horizontal capability display |
| CapabilityChip | ✅ Complete | Individual capabilities |
| CapabilityIndicator | ✅ Complete | Context-aware indicators |
| HITLAuthorizationCard | ✅ Complete | Human-in-the-loop approvals |
| SimpleApprovalDialog | ✅ Complete | Quick approve/reject |

### Phase 3: Audit & Transparency ✅

**Goal:** Immutable audit trail and one-click revocation

| Component | Status | Location |
|-----------|--------|----------|
| TaskReceipt | ✅ Complete | `app/.../audit/` |
| ActionType | ✅ Complete | Action type enumeration |
| TaskStatus | ✅ Complete | Status tracking |
| CapabilityUsage | ✅ Complete | Usage tracking |
| PiiAccess | ✅ Complete | PII access logging |
| RevocationRecord | ✅ Complete | Revocation history |
| AuditSession | ✅ Complete | Session audit |
| RiskSummary | ✅ Complete | Risk assessment |
| ArmorTerminal | ✅ Complete | Real-time activity log |
| RevocationPanel | ✅ Complete | One-click revocation |

### Phase 4: Commercial Polish ✅

**Goal:** Consistent branding and status indicators

| Component | Status | Location |
|-----------|--------|----------|
| ArmorClawTheme | ✅ Complete | `app/.../theme/` |
| SecurityStatusIcon | ✅ Complete | Security state indicator |
| AgentStatusIcon | ✅ Complete | Agent state indicator |
| CapabilityStatusIcon | ✅ Complete | Capability state |
| NetworkStatusIcon | ✅ Complete | Connection state |
| RiskLevelBadge | ✅ Complete | Risk level display |
| ActivityPulseIndicator | ✅ Complete | Animated activity |
| StatusBar | ✅ Complete | Combined status bar |

### Files Created (20 Total)

```
app/src/main/java/app/armorclaw/
├── components/
│   ├── vault/
│   │   ├── VaultModels.kt           ✅ UI data models
│   │   ├── VaultPulseIndicator.kt   ✅ Pulse animation
│   │   └── VaultKeyPanel.kt         ✅ Sidebar panel
│   ├── governor/
│   │   ├── GovernorModels.kt        ✅ Governor data models
│   │   ├── CommandBlock.kt          ✅ Action card
│   │   ├── CapabilityRibbon.kt      ✅ Capability display
│   │   ├── HITLAuthorization.kt     ✅ Hold-to-approve
│   │   └── GovernorComponents.kt    ✅ Barrel export
│   └── audit/
│       ├── AuditModels.kt           ✅ Audit data models
│       ├── ArmorTerminal.kt         ✅ Activity log
│       └── RevocationControls.kt    ✅ Revocation UI
├── theme/
│   ├── ArmorClawTheme.kt            ✅ Brand theme
│   └── StatusIcons.kt               ✅ Status icons
└── domain/
    └── store/vault/
        └── VaultStore.kt            ✅ State management

app/src/main/java/app/armorclaw/security/
├── KeystoreManager.kt               ✅ Keystore management
├── SqlCipherProvider.kt             ✅ Database encryption
└── VaultRepository.kt               ✅ PII storage

app/src/main/java/app/armorclaw/domain/security/
├── PiiRegistry.kt                   ✅ PII key registry
├── ShadowMap.kt                     ✅ Placeholder mapping
└── AgentRequestInterceptor.kt       ✅ Request middleware
```

### Key Features Delivered

| Feature | Description |
|---------|-------------|
| 🔐 Cold Vault | Hardware-backed encryption (Android Keystore + AES-256) |
| 🔒 SQLCipher | Encrypted database with 256-bit passphrase |
| 👥 Shadow Mapping | PII replaced with `{{VAULT:field:hash}}` placeholders |
| 📋 Command Blocks | Technical UI replacing chat bubbles |
| 🎛️ Capability Ribbon | Quick visibility of active capabilities |
| ✋ HITL Authorization | Hold-to-approve for sensitive actions |
| 📊 ArmorTerminal | Real-time activity log (terminal-style) |
| ⏪ One-Click Revocation | Immediate capability/session undo |
| 🎨 Brand Theme | Consistent Teal #14F0C8 / Navy #0A1428 |

### Security Model

| Layer | Implementation |
|-------|----------------|
| Transport | TLS 1.3 + Certificate Pinning |
| Database | SQLCipher 256-bit encryption |
| Keys | Android Keystore (hardware-backed) |
| PII | Shadow placeholders in transit |
| Audit | Immutable TaskReceipt records |

---

*This document provides a comprehensive overview of the ArmorChat project. For implementation details, see the source code and additional documentation in the `doc/` directory.*

> **Status:** 🟢 READY FOR DEPLOYMENT - All critical issues fixed
> **Last Reviewed:** 2026-02-20
> **Reviewer:** System Review

### ✅ Issues Fixed

| Issue | Severity | Status | File Changed |
|-------|----------|--------|--------------|
| Domain inconsistency | 🔴 Critical | ✅ FIXED | Multiple files |
| WebSocket URL mismatch | 🔴 Critical | ✅ FIXED | `BridgeWebSocketClient.kt` |
| AppModules Matrix URL | 🔴 Critical | ✅ FIXED | `AppModules.kt` |
| **API endpoint mismatch** | 🔴 **CRITICAL** | ✅ FIXED | `BridgeRpcClientImpl.kt` |
| ExternalLinkHandler URLs | 🟡 Medium | ✅ FIXED | `ExternalLinkHandler.kt` |
| ErrorReportingService URL | 🟡 Medium | ✅ FIXED | `ErrorReportingService.kt` |
| ReleaseConfig Matrix URL | 🟡 Medium | ✅ FIXED | `ReleaseConfig.kt` |
| TermsOfService email | 🟡 Medium | ✅ FIXED | `TermsOfServiceScreen.kt` |
| Test assertions | 🟡 Medium | ✅ FIXED | Test files |

### ⚠️ Critical Bug Fixed: API Endpoint Mismatch

**Problem:** ArmorChat client was posting to `/rpc` but Bridge server listens at `/api`

**Fixed:** Changed client to use `/api` endpoint:
- `BridgeRpcClientImpl.kt`: Changed `httpClient.post("/rpc")` → `httpClient.post("/api")`

### 📋 Domain Configuration (All Consistent)

All URLs now use `*.armorclaw.app`:

| Component | URL | File |
|-----------|-----|------|
| Bridge RPC | `https://bridge.armorclaw.app` | `RpcModels.kt` |
| Matrix Homeserver | `https://matrix.armorclaw.app` | `RpcModels.kt` |
| WebSocket | `wss://bridge.armorclaw.app` | `BridgeWebSocketClient.kt` |
| Demo Server | `https://demo.armorclaw.app` | `SetupService.kt` |
| Demo Bridge | `https://bridge-demo.armorclaw.app` | `SetupService.kt` |
| Fallback Bridge 1 | `https://bridge.armorclaw.app` | `SetupService.kt` |
| Fallback Bridge 2 | `https://bridge-backup.armorclaw.app` | `SetupService.kt` |
| Fallback Bridge 3 | `https://bridge-eu.armorclaw.app` | `SetupService.kt` |
| Website | `https://armorclaw.app` | `ExternalLinkHandler.kt` |
| Privacy Policy | `https://armorclaw.app/privacy` | `ExternalLinkHandler.kt` |
| Terms of Service | `https://armorclaw.app/terms` | `ExternalLinkHandler.kt` |
| Error Reporting API | `https://api.armorclaw.app` | `ErrorReportingService.kt` |
| Support Email | `support@armorclaw.app` | `ExternalLinkHandler.kt` |
| Legal Email | `legal@armorclaw.app` | `TermsOfServiceScreen.kt` |

### 🔗 Server Endpoints Required

| Service | Endpoint | Protocol | Purpose |
|---------|----------|----------|---------|
| Matrix Homeserver | `matrix.armorclaw.app` | HTTPS | User accounts, messaging |
| Bridge RPC | `bridge.armorclaw.app/api` | HTTPS | Admin operations |
| Bridge Health | `bridge.armorclaw.app/health` | HTTPS | Health check |
| Bridge Discovery | `bridge.armorclaw.app/discover` | HTTPS | Server discovery |
| Error Reporting | `api.armorclaw.app` | HTTPS | Error telemetry |

### Required Server URLs for First Deployment

| Service | URL | Notes |
|---------|-----|-------|
| Matrix Homeserver | `https://matrix.armorclaw.app` | Client connects directly |
| Bridge RPC | `https://bridge.armorclaw.app` | Admin operations via JSON-RPC |
| Bridge WebSocket | ❌ NOT USED | Use Matrix /sync instead |

### Files Updated for Deployment

| File | Change |
|------|--------|
| `RpcModels.kt` | ✅ Already correct - uses `.armorclaw.app` |
| `BridgeWebSocketClient.kt` | ✅ Fixed - changed `.com` to `.app` |
| `AppModules.kt` | ✅ Fixed - changed `.com` to `.app` |
| `BridgeRpcClientTest.kt` | ✅ Fixed - updated assertions |
| `BridgeWebSocketClientTest.kt` | ✅ Fixed - updated assertions |

### Configuration Checklist

- [x] All URLs use consistent domain (`.armorclaw.app`)
- [ ] Certificate pins configured for production
- [x] Demo server URLs defined
- [x] Fallback server URLs defined
- [ ] Matrix homeserver is accessible at `https://matrix.armorclaw.app`
- [ ] Bridge RPC endpoint is accessible at `https://bridge.armorclaw.app`
- [ ] SSL/TLS certificates valid for both domains

### ✅ Pre-Deployment Validation Steps

1. **DNS Verification:**
   ```bash
   # Verify DNS records exist
   nslookup matrix.armorclaw.app
   nslookup bridge.armorclaw.app
   nslookup api.armorclaw.app
   ```

2. **HTTPS Connectivity:**
   ```bash
   # Test Matrix homeserver
   curl -v https://matrix.armorclaw.app/_matrix/client/versions

   # Test Bridge health endpoint
   curl -v https://bridge.armorclaw.app/health

   # Test Bridge RPC endpoint (NOTE: uses /api, not /rpc!)
   curl -X POST https://bridge.armorclaw.app/api \
     -H "Content-Type: application/json" \
     -d '{"jsonrpc":"2.0","method":"system.health","id":"1"}'
   ```

3. **Certificate Validation:**
   ```bash
   openssl s_client -connect matrix.armorclaw.app:443
   openssl s_client -connect bridge.armorclaw.app:443
   openssl s_client -connect api.armorclaw.app:443
   ```

4. **Certificate Pinning (if enabled):**
   ```bash
   # Get certificate SHA-256 pins
   openssl s_client -connect bridge.armorclaw.app:443 | openssl x509 -pubkey -noout | openssl pkey -pubin -outform der | openssl dgst -sha256 -binary | openssl enc -base64
   ```

### 📱 Onboarding Flow Summary

```
Splash Screen
     │
     ├── hasCompletedOnboarding? ──NO──▶ WelcomeScreen
     │                                       │
     │                                       ▼
     │                                 SecurityExplanationScreen (3 screens)
     │                                       │
     │                                       ▼
     │                                 ConnectServerScreen
     │                                       │
     │                                       ├── User enters homeserver URL
     │                                       ├── App calls SetupService.startSetup()
     │                                       ├── Bridge health check at /health
     │                                       ├── If fails: Try fallback servers
     │                                       │
     │                                       ▼
     │                                 User enters credentials
     │                                       │
     │                                       ▼
     │                                 SetupService.connectWithCredentials()
     │                                       │
     │                                       ├── Start Bridge session
     │                                       ├── Login to Matrix
     │                                       ├── Get user role from server
     │                                       │
     │                                       ▼
     │                                 PermissionsScreen
     │                                       │
     │                                       ▼
     │                                 CompletionScreen
     │                                       │
     │◀──────────────────────────────────────┘
     │
     ├── isLoggedIn? ──NO──▶ LoginScreen
     │
     ▼
HomeScreen
```

### 🔧 Server Discovery Mechanism

1. **User enters homeserver URL** (e.g., `https://matrix.example.com`)
2. **App derives bridge URL**:
   - `https://matrix.example.com` → `https://bridge.example.com`
3. **Health check** on bridge at `/health` endpoint
4. **If primary fails**, try fallback servers:
   - `https://bridge.armorclaw.app`
   - `https://bridge-backup.armorclaw.app`
   - `https://bridge-eu.armorclaw.app`

### 🌐 Communication Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│                        ArmorChat Client                         │
├─────────────────────────────────────────────────────────────────┤
│                                                                 │
│  ┌───────────────┐    ┌──────────────┐    ┌────────────────┐   │
│  │  Matrix SDK   │    │ BridgeRpc    │    │ SetupService   │   │
│  │  (Messages)   │    │ Client       │    │ (Onboarding)   │   │
│  └───────┬───────┘    └──────┬───────┘    └────────┬───────┘   │
│          │                   │                     │           │
└──────────┼───────────────────┼─────────────────────┼───────────┘
           │                   │                     │
           ▼                   ▼                     ▼
    ┌──────────────┐    ┌──────────────┐    ┌──────────────┐
    │   Matrix     │    │   Bridge     │    │   Bridge     │
    │ Homeserver   │    │   /api       │    │   /health    │
    │ /_matrix/... │    │  (JSON-RPC)  │    │              │
    └──────────────┘    └──────────────┘    └──────────────┘
         matrix.            bridge.           bridge.
         armorclaw.app      armorclaw.app     armorclaw.app
```

### 📡 Bridge Server Endpoints

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/api` | POST | JSON-RPC 2.0 API |
| `/health` | GET | Health check (public) |
| `/discover` | GET | Server discovery info |
| `/fingerprint` | GET | Certificate fingerprint |
| `/ws` | WebSocket | Real-time events (not used by ArmorChat) |

**Note:** ArmorChat uses Matrix `/sync` for real-time events, not Bridge WebSocket.

---

## ArmorChat ↔ ArmorClaw Compatibility

### ✅ RPC Methods Compatibility Matrix

| Category | Client Method | Server Handler | Status |
|----------|---------------|----------------|--------|
| **Bridge Lifecycle** | `bridge.start` | ✅ `handleBridgeStart` | Compatible |
| | `bridge.stop` | ✅ `handleBridgeStop` | Compatible |
| | `bridge.status` | ✅ `handleBridgeStatus` | Compatible |
| | `bridge.health` | ✅ `handleBridgeHealth` | Compatible |
| **Matrix Operations** | `matrix.login` | ✅ `handleMatrixLogin` | Compatible |
| | `matrix.sync` | ✅ `handleMatrixSync` | Compatible |
| | `matrix.send` | ✅ `handleMatrixSend` | Compatible |
| | `matrix.create_room` | ✅ `handleMatrixCreateRoom` | Compatible |
| | `matrix.join_room` | ✅ `handleMatrixJoinRoom` | Compatible |
| | `matrix.leave_room` | ✅ `handleMatrixLeaveRoom` | Compatible |
| | `matrix.invite_user` | ✅ `handleMatrixInviteUser` | Compatible |
| | `matrix.send_typing` | ✅ `handleMatrixSendTyping` | Compatible |
| | `matrix.send_read_receipt` | ✅ `handleMatrixSendReadReceipt` | Compatible |
| | `matrix.refresh_token` | ✅ `handleMatrixRefreshToken` | Compatible |
| **Platform** | `platform.connect` | ✅ `handlePlatformConnect` | Compatible |
| | `platform.disconnect` | ✅ `handlePlatformDisconnect` | Compatible |
| | `platform.list` | ✅ `handlePlatformList` | Compatible |
| | `platform.status` | ✅ `handlePlatformStatus` | Compatible |
| | `platform.test` | ✅ `handlePlatformTest` | Compatible |
| | `platform.limits` | ✅ `handlePlatformLimits` | Compatible |
| **Push Notifications** | `push.register_token` | ✅ `handlePushRegisterToken` | Compatible |
| | `push.unregister_token` | ✅ `handlePushUnregisterToken` | Compatible |
| | `push.update_settings` | ✅ `handlePushUpdateSettings` | Compatible |
| **Recovery** | `recovery.*` | ✅ All recovery methods | Compatible |
| **License** | `license.status` | ✅ `handleLicenseStatus` | Compatible |
| | `license.features` | ✅ `handleLicenseFeatures` | Compatible |
| | `license.check_feature` | ✅ `handleLicenseCheckFeature` | Compatible |
| **Compliance** | `compliance.status` | ✅ `handleComplianceStatus` | Compatible |
| **Agent Management** | `agent.list` | ✅ `handleAgentList` | Compatible |
| | `agent.status` | ✅ `handleAgentStatus` | Compatible |
| | `agent.stop` | ✅ `handleAgentStop` | Compatible |
| **Workflow** | `workflow.templates` | ✅ `handleWorkflowTemplates` | Compatible |
| | `workflow.start` | ✅ `handleWorkflowStart` | Compatible |
| | `workflow.status` | ✅ `handleWorkflowStatus` | Compatible |
| **HITL (Human-in-the-Loop)** | `hitl.pending` | ✅ `handleHitlPending` | Compatible |
| | `hitl.approve` | ✅ `handleHitlApprove` | Compatible |
| | `hitl.reject` | ✅ `handleHitlReject` | Compatible |
| **Budget** | `budget.status` | ✅ `handleBudgetStatus` | Compatible |
| **Error Handling** | `get_errors` | ✅ `handleGetErrors` | Compatible |
| | `resolve_error` | ✅ `handleResolveError` | Compatible |
| **System (Public)** | `system.health` | ✅ `HandleSystemHealth` | Compatible |
| | `system.config` | ✅ `HandleSystemConfig` | Compatible |

### 📡 Communication Protocol Stack

| Layer | Protocol | Implementation |
|-------|----------|----------------|
| Application | JSON-RPC 2.0 | HTTP POST to `/api` |
| Transport | HTTPS | TLS 1.3 |
| Real-time Events | Matrix /sync | Long-poll (30s timeout) |
| E2EE | Megolm | Via Matrix Rust SDK |
| Authentication | Bearer Token | Matrix access token |

### ⚠️ Known Limitations

1. **WebSocket Not Used:** Bridge WebSocket exists at `/ws` but ArmorChat uses Matrix `/sync` for real-time events
2. **Matrix SDK Required:** For messaging functionality, use `MatrixClient` interface directly
3. **Admin Operations via RPC:** Bridge RPC is for admin/container operations, not messaging
4. **First-User Admin:** Admin privileges are server-authoritative (no client-side race conditions)

---

## 🚀 Deployment Order

### 1. Infrastructure Setup

```bash
# DNS Records (create these first)
matrix.armorclaw.app     A     <matrix-server-ip>
bridge.armorclaw.app     A     <bridge-server-ip>
api.armorclaw.app        A     <api-server-ip>
```

### 2. Deploy Matrix Homeserver

```bash
# Option A: Conduit (recommended for simplicity)
docker run -d \
  --name conduit \
  -v /opt/conduit:/data \
  -p 6167:6167 \
  girlbossceo/conduit:latest

# Option B: Synapse (full-featured)
docker run -d \
  --name synapse \
  -v /opt/synapse:/data \
  -p 8008:8008 \
  matrixdotorg/synapse:latest
```

### 3. Deploy ArmorClaw Bridge

```bash
# Build bridge
cd ArmorClaw/bridge
go build -o armorclaw-bridge ./cmd/bridge

# Create config
./armorclaw-bridge init

# Edit config.toml with your settings
# - matrix.homeserver_url = "https://matrix.armorclaw.app"
# - matrix.enabled = true
# - server.socket_path = "/run/armorclaw/bridge.sock"

# Start bridge
./armorclaw-bridge
```

### 4. Configure Reverse Proxy (nginx/Caddy)

```nginx
# /etc/nginx/sites-enabled/bridge.armorclaw.app
server {
    listen 443 ssl http2;
    server_name bridge.armorclaw.app;

    ssl_certificate /etc/letsencrypt/live/bridge.armorclaw.app/fullchain.pem;
    ssl_certificate_key /etc/letsencrypt/live/bridge.armorclaw.app/privkey.pem;

    location /api {
        proxy_pass http://127.0.0.1:8443;
        proxy_http_version 1.1;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
    }

    location /health {
        proxy_pass http://127.0.0.1:8443;
    }

    location /discover {
        proxy_pass http://127.0.0.1:8443;
    }
}
```

### 5. SSL Certificates

```bash
# Get Let's Encrypt certificates
certbot certonly --nginx -d matrix.armorclaw.app
certbot certonly --nginx -d bridge.armorclaw.app
certbot certonly --nginx -d api.armorclaw.app
```

### 6. Build ArmorChat APK

```bash
cd ArmorChat

# Update certificate pins in CertificatePinner.kt if using pinning

# Build release APK
./gradlew assembleRelease

# APK will be at:
# app/build/outputs/apk/release/app-release.apk
```

---

## Server Discovery Implementation

### Discovery Methods Available

| Method | Status | Description |
|--------|--------|-------------|
| Manual Entry | ✅ Implemented | User enters homeserver URL |
| URL Derivation | ✅ Implemented | Auto-derive bridge URL from matrix URL |
| Demo Server | ✅ Implemented | Quick option for demo |
| Localhost | ✅ Implemented | Quick option for development |
| Fallback Servers | ✅ Implemented | Auto-retry with backup servers |
| **Matrix Well-Known** | 🚧 NEW | Standard Matrix discovery |
| **QR Code Scanning** | 🚧 NEW | Scan QR from Bridge server |
| **mDNS/Bonjour** | 🚧 NEW | Discover local servers |
| **Deep Links** | ✅ Implemented | `armorclaw://` scheme |

### Discovery Flow

```
┌──────────────────────────────────────────────────────────────────────────┐
│                      ArmorChat Server Discovery                          │
├──────────────────────────────────────────────────────────────────────────┤
│                                                                          │
│  1. QR CODE SCANNING (Preferred for local setup)                         │
│     ┌─────────┐      ┌─────────────┐      ┌──────────────────┐          │
│     │ Bridge  │ ───▶ │ QR Code     │ ───▶ │ ArmorChat        │          │
│     │ Server  │      │ (config)    │      │ scans & connects │          │
│     └─────────┘      └─────────────┘      └──────────────────┘          │
│                                                                          │
│  2. mDNS DISCOVERY (For local network)                                   │
│     ┌─────────┐      ┌─────────────┐      ┌──────────────────┐          │
│     │ Bridge  │ ───▶ │ _armorclaw  │ ───▶ │ ArmorChat        │          │
│     │ Server  │      │ ._tcp.local │      │ discovers        │          │
│     └─────────┘      └─────────────┘      └──────────────────┘          │
│                                                                          │
│  3. WELL-KNOWN DISCOVERY (Standard Matrix)                               │
│     ┌────────────────┐      ┌──────────────────┐      ┌────────────┐    │
│     │ armorclaw.app  │ ───▶ │ /.well-known/    │ ───▶ │ ArmorChat  │    │
│     │ (server name)  │      │ matrix/client    │      │ connects   │    │
│     └────────────────┘      └──────────────────┘      └────────────┘    │
│                                                                          │
│  4. MANUAL ENTRY (Fallback)                                              │
│     ┌────────────┐      ┌──────────────────────────────────────┐        │
│     │ User types │ ───▶ │ https://matrix.armorclaw.app         │        │
│     │ server URL │      │ → derives: https://bridge.armorclaw.app/api │ │
│     └────────────┘      └──────────────────────────────────────┘        │
│                                                                          │
└──────────────────────────────────────────────────────────────────────────┘
```

### ArmorClaw Bridge Server - Required Changes

#### 1. Add Well-Known Endpoint

Create `bridge/pkg/http/wellknown.go`:

```go
package http

import (
    "encoding/json"
    "net/http"
)

// WellKnownResponse is the Matrix well-known discovery response
type WellKnownResponse struct {
    Homeserver     HomeserverInfo     `json:"m.homeserver"`
    IdentityServer *IdentityServerInfo `json:"m.identity_server,omitempty"`
    Bridge         *BridgeInfo         `json:"com.armorclaw.bridge,omitempty"`
}

type HomeserverInfo struct {
    BaseURL string `json:"base_url"`
}

type IdentityServerInfo struct {
    BaseURL string `json:"base_url"`
}

type BridgeInfo struct {
    BaseURL      string `json:"base_url"`
    APIEndpoint  string `json:"api_endpoint"`
    PushGateway  string `json:"push_gateway,omitempty"`
}

// handleWellKnown serves the Matrix well-known discovery document
// This should be served at /.well-known/matrix/client
func (s *Server) handleWellKnown(w http.ResponseWriter, r *http.Request) {
    response := WellKnownResponse{
        Homeserver: HomeserverInfo{
            BaseURL: "https://matrix.armorclaw.app",
        },
        IdentityServer: &IdentityServerInfo{
            BaseURL: "https://matrix.armorclaw.app",
        },
        Bridge: &BridgeInfo{
            BaseURL:     "https://bridge.armorclaw.app",
            APIEndpoint: "https://bridge.armorclaw.app/api",
            PushGateway: "https://bridge.armorclaw.app/_matrix/push/v1/notify",
        },
    }

    w.Header().Set("Content-Type", "application/json")
    w.Header().Set("Access-Control-Allow-Origin", "*")
    json.NewEncoder(w).Encode(response)
}
```

Add to `bridge/pkg/http/server.go`:
```go
// In Start() function, add:
mux.HandleFunc("/.well-known/matrix/client", s.handleWellKnown)
```

#### 2. Enhanced Discovery Endpoint

Update `bridge/pkg/http/server.go` handleDiscover:

```go
func (s *Server) handleDiscover(w http.ResponseWriter, r *http.Request) {
    hostname, _ := os.Hostname()
    ips, _ := getLocalIPs()
    fingerprint, _ := s.GetCertificateFingerprint()

    response := map[string]interface{}{
        "name":        hostname,
        "hostname":    s.config.Hostname,
        "port":        s.config.Port,
        "ips":         ips,
        "version":     "1.0.0",
        "fingerprint": fingerprint,
        "endpoints": map[string]string{
            "matrix": "https://matrix.armorclaw.app",
            "rpc":    "https://bridge.armorclaw.app/api",
            "ws":     "wss://bridge.armorclaw.app/ws",
            "push":   "https://bridge.armorclaw.app/_matrix/push/v1/notify",
        },
        "features": map[string]bool{
            "e2ee":  true,
            "voice": true,
            "push":  true,
        },
    }

    w.Header().Set("Content-Type", "application/json")
    w.Header().Set("Access-Control-Allow-Origin", "*")
    json.NewEncoder(w).Encode(response)
}
```

#### 3. QR Code Endpoints

The Bridge already has QR code support in `bridge/pkg/qr/public.go`. Add HTTP endpoints:

```go
// In bridge/pkg/http/server.go, add:
func (s *Server) handleQRConfig(w http.ResponseWriter, r *http.Request) {
    result, err := s.qrManager.GenerateConfigQR(24 * time.Hour)
    if err != nil {
        http.Error(w, "Failed to generate QR", 500)
        return
    }

    // Return JSON with QR image as base64
    response := map[string]interface{}{
        "url":        result.URL,
        "deep_link":  result.DeepLink,
        "qr_image":   base64.StdEncoding.EncodeToString(result.QRImage),
        "expires_at": result.ExpiresAt,
        "config":     result.Config,
    }

    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(response)
}

func (s *Server) handleQRImage(w http.ResponseWriter, r *http.Request) {
    result, err := s.qrManager.GenerateConfigQR(24 * time.Hour)
    if err != nil {
        http.Error(w, "Failed to generate QR", 500)
        return
    }

    w.Header().Set("Content-Type", "image/png")
    w.Write(result.QRImage)
}
```

Add routes in Start():
```go
mux.HandleFunc("/qr/config", s.handleQRConfig)
mux.HandleFunc("/qr/image", s.handleQRImage)
```

### ArmorChat - Required Changes

#### 1. Create Discovery Service

Create `app/src/main/java/app/armorclaw/platform/discovery/DiscoveryService.kt`:

```kotlin
package app.armorclaw.platform.discovery

import app.armorclaw.domain.model.OperationContext
import app.armorclaw.platform.bridge.DetectServerInfo
import app.armorclaw.platform.logging.repositoryLogger
import io.ktor.client.*
import io.ktor.client.call.*
import io.ktor.client.request.*
import io.ktor.http.*
import kotlinx.coroutines.flow.*
import kotlinx.serialization.SerialName
import kotlinx.serialization.Serializable
import kotlinx.serialization.json.Json

/**
 * Server Discovery Service
 *
 * Provides multiple methods for discovering ArmorClaw servers:
 * 1. Matrix Well-Known discovery (standard)
 * 2. mDNS/Bonjour discovery (local network)
 * 3. QR code scanning (direct configuration)
 * 4. Manual entry with URL derivation
 */
class DiscoveryService(
    private val httpClient: HttpClient = HttpClient(),
    private val mdnsDiscovery: MDNSDiscovery = MDNSDiscoveryStub(),
    private val qrScanner: QRScanner = QRScannerStub()
) {
    private val logger = repositoryLogger("DiscoveryService")
    private val json = Json { ignoreUnknownKeys = true }

    private val _discoveredServers = MutableStateFlow<List<DiscoveredServer>>(emptyList())
    val discoveredServers: StateFlow<List<DiscoveredServer>> = _discoveredServers.asStateFlow()

    private val _isScanning = MutableStateFlow(false)
    val isScanning: StateFlow<Boolean> = _isScanning.asStateFlow()

    /**
     * Discover server using Matrix well-known
     * Standard Matrix discovery: GET https://server/.well-known/matrix/client
     */
    suspend fun discoverWellKnown(
        serverName: String,
        context: OperationContext? = null
    ): DiscoveryResult {
        return try {
            val url = "https://$serverName/.well-known/matrix/client"
            logger.logInfo("Discovering via well-known", mapOf("url" to url))

            val response = httpClient.get(url) {
                header("Accept", "application/json")
            }

            if (!response.status.isSuccess()) {
                return DiscoveryResult.Error("Server returned ${response.status}")
            }

            val wellKnown = json.decodeFromString<WellKnownResponse>(response.body())

            DiscoveryResult.Success(
                DiscoveredServer(
                    homeserver = wellKnown.homeserver.baseUrl,
                    bridgeUrl = wellKnown.bridge?.apiEndpoint
                        ?: deriveBridgeUrl(wellKnown.homeserver.baseUrl),
                    serverName = serverName,
                    discoveryMethod = DiscoveryMethod.WELL_KNOWN,
                    pushGateway = wellKnown.bridge?.pushGateway
                )
            )
        } catch (e: Exception) {
            logger.logError("Well-known discovery failed", e)
            DiscoveryResult.Error(e.message ?: "Discovery failed")
        }
    }

    /**
     * Discover local servers via mDNS
     * Returns immediately with cached results, updates via flow
     */
    suspend fun discoverLocal(): List<DiscoveredServer> {
        _isScanning.value = true
        return try {
            val servers = mdnsDiscovery.discover()
            _discoveredServers.value = servers
            servers
        } catch (e: Exception) {
            logger.logError("mDNS discovery failed", e)
            emptyList()
        } finally {
            _isScanning.value = false
        }
    }

    /**
     * Parse QR code data
     * Supports: armorclaw://config?d=<base64>
     */
    fun parseQRCode(qrData: String): DiscoveryResult {
        return try {
            when {
                qrData.startsWith("armorclaw://config?") -> parseConfigDeepLink(qrData)
                qrData.startsWith("armorclaw://setup?") -> parseSetupDeepLink(qrData)
                qrData.startsWith("armorclaw://invite?") -> parseInviteDeepLink(qrData)
                qrData.startsWith("https://armorclaw.app/") -> parseWebLink(qrData)
                else -> DiscoveryResult.Error("Unknown QR code format")
            }
        } catch (e: Exception) {
            logger.logError("QR parse failed", e)
            DiscoveryResult.Error("Invalid QR code: ${e.message}")
        }
    }

    /**
     * Start QR scanning session
     */
    suspend fun startQRScanning(): QRScanResult {
        return qrScanner.scan()
    }

    /**
     * Derive bridge URL from homeserver URL
     */
    fun deriveBridgeUrl(homeserver: String): String {
        val url = homeserver.removeSuffix("/")
        return when {
            url.contains("://matrix.") -> url.replace("://matrix.", "://bridge.")
            url.contains("://chat.") -> url.replace("://chat.", "://bridge.")
            else -> url.replace("://", "://bridge.")
        }
    }

    private fun parseConfigDeepLink(uri: String): DiscoveryResult {
        // armorclaw://config?d=<base64-encoded-config>
        val params = uri.substringAfter("?").parseQueryParams()
        val data = params["d"] ?: return DiscoveryResult.Error("Missing config data")

        val configJson = java.util.Base64.getUrlDecoder().decode(data).decodeToString()
        val config = json.decodeFromString<QRConfigPayload>(configJson)

        // Validate signature (optional, for security)
        if (config.expiresAt < System.currentTimeMillis() / 1000) {
            return DiscoveryResult.Error("Config expired")
        }

        return DiscoveryResult.Success(
            DiscoveredServer(
                homeserver = config.matrixHomeserver,
                bridgeUrl = config.rpcUrl,
                serverName = config.serverName,
                discoveryMethod = DiscoveryMethod.QR_CODE,
                pushGateway = config.pushGateway
            )
        )
    }

    private fun parseSetupDeepLink(uri: String): DiscoveryResult {
        // armorclaw://setup?token=xxx&server=xxx
        val params = uri.substringAfter("?").parseQueryParams()
        val token = params["token"] ?: return DiscoveryResult.Error("Missing token")
        val server = params["server"] ?: return DiscoveryResult.Error("Missing server")

        return DiscoveryResult.Success(
            DiscoveredServer(
                homeserver = server,
                bridgeUrl = deriveBridgeUrl(server),
                serverName = server,
                discoveryMethod = DiscoveryMethod.QR_CODE,
                setupToken = token
            )
        )
    }

    private fun parseInviteDeepLink(uri: String): DiscoveryResult {
        // armorclaw://invite?code=xxx
        val params = uri.substringAfter("?").parseQueryParams()
        val code = params["code"] ?: return DiscoveryResult.Error("Missing invite code")

        // Invite codes contain server info
        return DiscoveryResult.InviteCode(code)
    }

    private fun parseWebLink(uri: String): DiscoveryResult {
        // https://armorclaw.app/config?d=xxx
        // https://armorclaw.app/invite/xxx
        // https://armorclaw.app/setup?token=xxx

        return when {
            uri.contains("/config?") -> parseConfigDeepLink(
                uri.replace("https://armorclaw.app/", "armorclaw://")
            )
            uri.contains("/invite/") -> {
                val code = uri.substringAfter("/invite/").substringBefore("?")
                DiscoveryResult.InviteCode(code)
            }
            uri.contains("/setup?") -> parseSetupDeepLink(
                uri.replace("https://armorclaw.app/", "armorclaw://")
            )
            else -> DiscoveryResult.Error("Unknown web link format")
        }
    }

    private fun String.parseQueryParams(): Map<String, String> {
        return split("&")
            .mapNotNull { param ->
                val parts = param.split("=", limit = 2)
                if (parts.size == 2) parts[0] to parts[1] else null
            }
            .toMap()
    }
}

// ============================================================================
// Models
// ============================================================================

@Serializable
data class WellKnownResponse(
    @SerialName("m.homeserver")
    val homeserver: HomeserverInfo,
    @SerialName("m.identity_server")
    val identityServer: HomeserverInfo? = null,
    @SerialName("com.armorclaw.bridge")
    val bridge: BridgeWellKnownInfo? = null
)

@Serializable
data class HomeserverInfo(
    @SerialName("base_url")
    val baseUrl: String
)

@Serializable
data class BridgeWellKnownInfo(
    @SerialName("base_url")
    val baseUrl: String,
    @SerialName("api_endpoint")
    val apiEndpoint: String? = null,
    @SerialName("push_gateway")
    val pushGateway: String? = null
)

@Serializable
data class QRConfigPayload(
    val version: Int,
    @SerialName("matrix_homeserver")
    val matrixHomeserver: String,
    @SerialName("rpc_url")
    val rpcUrl: String,
    @SerialName("ws_url")
    val wsUrl: String? = null,
    @SerialName("push_gateway")
    val pushGateway: String? = null,
    @SerialName("server_name")
    val serverName: String,
    val region: String? = null,
    @SerialName("expires_at")
    val expiresAt: Long,
    val signature: String? = null
)

data class DiscoveredServer(
    val homeserver: String,
    val bridgeUrl: String,
    val serverName: String,
    val discoveryMethod: DiscoveryMethod,
    val pushGateway: String? = null,
    val setupToken: String? = null,
    val host: String? = null,        // For mDNS
    val port: Int? = null,           // For mDNS
    val fingerprint: String? = null  // For TLS verification
)

enum class DiscoveryMethod {
    WELL_KNOWN,
    MDNS,
    QR_CODE,
    MANUAL,
    DEEP_LINK
}

sealed class DiscoveryResult {
    data class Success(val server: DiscoveredServer) : DiscoveryResult()
    data class Error(val message: String) : DiscoveryResult()
    data class InviteCode(val code: String) : DiscoveryResult()

    val isSuccess: Boolean get() = this is Success
}

sealed class QRScanResult {
    data class Success(val data: String) : QRScanResult()
    data class Cancelled(val reason: String? = null) : QRScanResult()
    data class Error(val message: String) : QRScanResult()
}

// ============================================================================
// Platform Interfaces (Android service pattern)
// ============================================================================

expect class MDNSDiscovery() {
    suspend fun discover(): List<DiscoveredServer>
    fun startScanning()
    fun stopScanning()
}

expect class QRScanner() {
    suspend fun scan(): QRScanResult
    fun isAvailable(): Boolean
}

// Stub implementations for common code
class MDNSDiscoveryStub : MDNSDiscovery {
    override suspend fun discover(): List<DiscoveredServer> = emptyList()
    override fun startScanning() {}
    override fun stopScanning() {}
}

class QRScannerStub : QRScanner {
    override suspend fun scan(): QRScanResult = QRScanResult.Error("QR scanning not available")
    override fun isAvailable(): Boolean = false
}
```

#### 2. Android mDNS Implementation

Create `app/src/main/java/app/armorclaw/platform/discovery/MDNSDiscovery.android.kt`:

```kotlin
package app.armorclaw.platform.discovery

import android.content.Context
import android.net.nsd.NsdManager
import android.net.nsd.NsdServiceInfo
import kotlinx.coroutines.channels.awaitClose
import kotlinx.coroutines.flow.Flow
import kotlinx.coroutines.flow.callbackFlow
import kotlinx.coroutines.suspendCancellableCoroutine
import kotlin.coroutines.resume
import kotlin.coroutines.resumeWithException

actual class MDNSDiscovery actual constructor(
    private val context: Context? = null
) {
    private val SERVICE_TYPE = "_armorclaw._tcp"

    private var nsdManager: NsdManager? = null
    private var isScanning = false

    init {
        context?.let {
            nsdManager = it.getSystemService(Context.NSD_SERVICE) as NsdManager
        }
    }

    actual suspend fun discover(): List<DiscoveredServer> = suspendCancellableCoroutine { cont ->
        if (nsdManager == null) {
            cont.resume(emptyList())
            return@suspendCancellableCoroutine
        }

        val discoveredServers = mutableListOf<DiscoveredServer>()
        var resolveCount = 0
        var expectedResolves = 0

        val discoveryListener = object : NsdManager.DiscoveryListener {
            override fun onDiscoveryStarted(regType: String) {
                isScanning = true
            }

            override fun onServiceFound(service: NsdServiceInfo) {
                if (service.serviceType == SERVICE_TYPE) {
                    expectedResolves++
                    nsdManager?.resolveService(service, object : NsdManager.ResolveListener {
                        override fun onResolveFailed(serviceInfo: NsdServiceInfo, errorCode: Int) {
                            resolveCount++
                            checkComplete()
                        }

                        override fun onServiceResolved(serviceInfo: NsdServiceInfo) {
                            resolveCount++
                            discoveredServers.add(
                                DiscoveredServer(
                                    homeserver = "https://${serviceInfo.host.hostAddress ?: serviceInfo.serviceName}",
                                    bridgeUrl = "https://${serviceInfo.host.hostAddress ?: serviceInfo.serviceName}:${serviceInfo.port}",
                                    serverName = serviceInfo.serviceName,
                                    discoveryMethod = DiscoveryMethod.MDNS,
                                    host = serviceInfo.host.hostAddress,
                                    port = serviceInfo.port
                                )
                            )
                            checkComplete()
                        }

                        fun checkComplete() {
                            // Wait a bit for all resolves to complete
                            if (resolveCount >= expectedResolves && expectedResolves > 0) {
                                nsdManager?.stopServiceDiscovery(this@discoveryListener)
                            }
                        }
                    })
                }
            }

            override fun onServiceLost(service: NsdServiceInfo) {}
            override fun onDiscoveryStopped(serviceType: String) {
                isScanning = false
                cont.resume(discoveredServers)
            }
            override fun onStartDiscoveryFailed(serviceType: String, errorCode: Int) {
                nsdManager?.stopServiceDiscovery(this)
                cont.resume(emptyList())
            }
            override fun onStopDiscoveryFailed(serviceType: String, errorCode: Int) {
                cont.resume(discoveredServers)
            }
        }

        nsdManager?.discoverServices(SERVICE_TYPE, NsdManager.PROTOCOL_DNS_SD, discoveryListener)

        cont.invokeOnCancellation {
            nsdManager?.stopServiceDiscovery(discoveryListener)
        }
    }

    actual fun startScanning() {
        // Continuous scanning not implemented yet
    }

    actual fun stopScanning() {
        isScanning = false
    }
}
```

#### 3. Android QR Scanner Implementation

Create `app/src/main/java/app/armorclaw/platform/discovery/QRScanner.android.kt`:

```kotlin
package app.armorclaw.platform.discovery

import android.app.Activity
import android.content.Context
import android.content.Intent
import androidx.activity.result.contract.ActivityResultContracts
import com.journeyapps.barcodescanner.ScanContract
import com.journeyapps.barcodescanner.ScanIntentResult
import com.journeyapps.barcodescanner.ScanOptions
import kotlinx.coroutines.suspendCancellableCoroutine
import kotlin.coroutines.resume

actual class QRScanner actual constructor(
    private val context: Context? = null
) {
    actual suspend fun scan(): QRScanResult = suspendCancellableCoroutine { cont ->
        // This needs to be called from an Activity with lifecycle awareness
        // For now, return a result that indicates the caller should use the Activity-based API
        cont.resume(QRScanResult.Error("Use scanWithActivity() for Android"))
    }

    /**
     * Activity-based QR scanning for Android
     * Call this from your Activity/Fragment
     */
    fun scanWithActivity(
        activity: Activity,
        callback: (QRScanResult) -> Unit
    ) {
        val options = ScanOptions()
            .setDesiredBarcodeFormats(ScanOptions.QR_CODE)
            .setPrompt("Scan ArmorClaw QR Code")
            .setCameraId(0)
            .setBeepEnabled(false)
            .setBarcodeImageEnabled(true)

        // This requires integration with ActivityResultLauncher
        // See: https://github.com/journeyapps/zxing-android-embedded
        callback(QRScanResult.Error("Integrate with ActivityResultLauncher"))
    }

    actual fun isAvailable(): Boolean = context != null
}
```

#### 4. Update SetupService

Add to `SetupService.kt`:

```kotlin
class SetupService(
    private val rpcClient: BridgeRpcClient,
    private val wsClient: BridgeWebSocketClient,
    private val discoveryService: DiscoveryService = DiscoveryService()
) {
    // ... existing code ...

    /**
     * Start setup with automatic discovery
     * Tries well-known first, then falls back to URL derivation
     */
    suspend fun startSetupWithDiscovery(
        serverNameOrUrl: String,
        context: OperationContext? = null
    ): SetupResult {
        val ctx = context ?: OperationContext.create()

        _setupState.value = SetupState.DetectingServer

        // Try well-known discovery first
        if (!serverNameOrUrl.startsWith("http")) {
            val wellKnownResult = discoveryService.discoverWellKnown(serverNameOrUrl, ctx)
            if (wellKnownResult is DiscoveryResult.Success) {
                val server = wellKnownResult.server
                return startSetup(server.homeserver, server.bridgeUrl, ctx)
            }
        }

        // Fall back to manual entry
        val homeserver = if (serverNameOrUrl.startsWith("http")) {
            serverNameOrUrl
        } else {
            "https://matrix.$serverNameOrUrl"
        }

        return startSetup(homeserver, null, ctx)
    }

    /**
     * Setup from QR code
     */
    suspend fun setupFromQRCode(
        qrData: String,
        context: OperationContext? = null
    ): SetupResult {
        val ctx = context ?: OperationContext.create()

        val result = discoveryService.parseQRCode(qrData)
        return when (result) {
            is DiscoveryResult.Success -> {
                val server = result.server
                startSetup(server.homeserver, server.bridgeUrl, ctx)
            }
            is DiscoveryResult.Error -> {
                SetupResult.Error(result.message)
            }
            is DiscoveryResult.InviteCode -> {
                // Handle invite code
                parseInviteAndSetup(result.code, ctx)
            }
        }
    }

    /**
     * Discover local servers via mDNS
     */
    suspend fun discoverLocalServers(): List<DiscoveredServer> {
        return discoveryService.discoverLocal()
    }
}
```

### Add Dependencies

Add to `app/build.gradle.kts`:
```kotlin
// In app dependencies:
implementation("com.journeyapps:zxing-android-embedded:4.3.0")
```

### Update ConnectServerScreen

Add discovery options to the UI:

```kotlin
// Add to ConnectServerScreen.kt

// Scan QR button
OutlinedButton(
    onClick = { showQRScanner = true },
    modifier = Modifier.fillMaxWidth()
) {
    Icon(Icons.Default.QrCodeScanner, contentDescription = null)
    Spacer(modifier = Modifier.width(8.dp))
    Text("Scan QR Code")
}

// Discover local servers button
OutlinedButton(
    onClick = {
        scope.launch {
            val servers = viewModel.discoverLocalServers()
            if (servers.isNotEmpty()) {
                discoveredServers = servers
            }
        }
    },
    modifier = Modifier.fillMaxWidth()
) {
    Icon(Icons.Default.Wifi, contentDescription = null)
    Spacer(modifier = Modifier.width(8.dp))
    Text("Find Local Servers")
}

// Show discovered servers
if (discoveredServers.isNotEmpty()) {
    Text("Discovered Servers:", style = MaterialTheme.typography.labelMedium)
    discoveredServers.forEach { server ->
        Card(
            modifier = Modifier
                .fillMaxWidth()
                .padding(vertical = 4.dp)
                .clickable {
                    homeserver = server.homeserver
                    customBridgeUrl = server.bridgeUrl
                }
        ) {
            ListItem(
                headlineContent = { Text(server.serverName) },
                supportingContent = { Text(server.host ?: server.homeserver) },
                leadingContent = {
                    Icon(
                        when (server.discoveryMethod) {
                            DiscoveryMethod.MDNS -> Icons.Default.Wifi
                            else -> Icons.Default.Dns
                        },
                        contentDescription = null
                    )
                }
            )
        }
    }
}
```

---

## Signed Configuration System Review

### Proposal Analysis

The signed configuration system is a **good addition** but has issues:

| Aspect | Status | Issue |
|--------|--------|-------|
| Domain | ❌ | Uses `.com` instead of `.app` |
| API Path | ❌ | Uses `/rpc` instead of `/api` |
| Architecture | ⚠️ | Creates parallel system instead of integrating |
| Security | ✅ | HMAC signature verification is good |
| UX Flow | ✅ | QR/deep link flow is user-friendly |

### Recommended: Unified Discovery Architecture

Instead of creating a separate system, integrate signed config into `SetupService`:

```
┌─────────────────────────────────────────────────────────────────────────┐
│                    SetupService (Single Entry Point)                     │
├─────────────────────────────────────────────────────────────────────────┤
│                                                                         │
│  startSetupWithDiscovery(input: String)                                 │
│       │                                                                 │
│       ├─── QR Code? ────▶ parseSignedConfig() ───▶ startSetup()         │
│       │                                                                 │
│       ├─── Deep Link? ──▶ parseSignedConfig() ───▶ startSetup()         │
│       │                                                                 │
│       ├─── Domain? ─────▶ discoverWellKnown() ───▶ startSetup()         │
│       │                                                                 │
│       ├─── Local IP? ───▶ discoverMDNS() ────────▶ startSetup()         │
│       │                                                                 │
│       └─── Full URL? ───▶ deriveBridgeUrl() ─────▶ startSetup()         │
│                                                                         │
└─────────────────────────────────────────────────────────────────────────┘
```

### Corrected Configuration

```kotlin
// CORRECTED BuildConfig fields
android {
    defaultConfig {
        // Production defaults (release builds) - USE .app DOMAIN
        buildConfigField("String", "MATRIX_HOMESERVER", "\"https://matrix.armorclaw.app\"")
        buildConfigField("String", "ARMORCLAW_RPC_URL", "\"https://bridge.armorclaw.app/api\"")
        buildConfigField("String", "ARMORCLAW_WS_URL", "\"wss://bridge.armorclaw.app/ws\"")
        buildConfigField("String", "PUSH_GATEWAY", "\"https://bridge.armorclaw.app/_matrix/push/v1/notify\"")
    }

    buildTypes {
        debug {
            // Debug defaults (emulator)
            buildConfigField("String", "MATRIX_HOMESERVER", "\"http://10.0.2.2:8008\"")
            buildConfigField("String", "ARMORCLAW_RPC_URL", "\"http://10.0.2.2:8080/api\"")
            buildConfigField("String", "ARMORCLAW_WS_URL", "\"ws://10.0.2.2:8080/ws\"")
            buildConfigField("String", "PUSH_GATEWAY", "\"http://10.0.2.2:8080/_matrix/push/v1/notify\"")
        }
    }
}
```

### Bridge Server: Corrected QR Config

The Bridge's `pkg/qr/public.go` already has the right structure but needs endpoint fix:

```go
// CORRECTED in GenerateConfigQR
config := &ConfigPayload{
    Version:          1,
    MatrixHomeserver: m.serverURL,
    RpcURL:           m.bridgeURL + "/api",    // FIXED: was /rpc
    WsURL:            m.bridgeURL + "/ws",
    PushGateway:      m.bridgeURL + "/_matrix/push/v1/notify",
    ServerName:       m.serverName,
    ExpiresAt:        time.Now().Add(expiration).Unix(),
}
```

### Integration with SetupService

Add to existing `SetupService.kt`:

```kotlin
/**
 * Parse signed configuration from QR/deep link
 * Returns server info if valid, error if invalid/expired
 */
suspend fun parseSignedConfig(
    deepLinkOrQR: String,
    context: OperationContext? = null
): SetupResult {
    val ctx = context ?: OperationContext.create()

    return try {
        // Parse the deep link
        val config = when {
            deepLinkOrQR.startsWith("armorclaw://config?") -> {
                parseConfigDeepLink(deepLinkOrQR)
            }
            deepLinkOrQR.startsWith("https://armorclaw.app/config?") -> {
                parseConfigDeepLink(deepLinkOrQR)
            }
            else -> return SetupResult.Error("Invalid config format")
        }

        // Validate signature (if bridge provides public key)
        // For now, trust the config (could add signature verification)

        // Check expiration
        if (config.expiresAt != null && config.expiresAt < Clock.System.now().epochSeconds) {
            return SetupResult.Error(
                "Configuration has expired. Please scan a new QR code.",
                listOf(FallbackOption.CONTACT_SUPPORT)
            )
        }

        // Apply configuration
        _config.value = SetupConfig(
            homeserver = config.matrixHomeserver,
            bridgeUrl = config.rpcUrl,
            serverVersion = "1.0.0",
            supportsE2EE = true,
            supportsRecovery = true,
            configSource = ConfigSource.SIGNED_QR
        )

        _serverInfo.value = DetectServerInfo(
            homeserver = config.matrixHomeserver,
            bridgeUrl = config.rpcUrl,
            version = "1.0.0",
            supportsE2EE = true,
            supportsRecovery = true,
            region = config.region ?: "us-east"
        )

        _setupState.value = SetupState.ReadyForCredentials

        SetupResult.Success(_serverInfo.value!!)
    } catch (e: Exception) {
        logger.logOperationError("parseSignedConfig", e)
        SetupResult.Error("Invalid configuration: ${e.message}")
    }
}

// Add to ConfigSource enum
enum class ConfigSource {
    DEFAULT,
    MANUAL,
    WELL_KNOWN,
    MDNS,
    SIGNED_QR,
    DEEP_LINK
}
```

### Proposed AndroidManifest.xml (Corrected)

```xml
<activity android:name=".MainActivity">
    <!-- armorclaw:// deep links -->
    <intent-filter>
        <action android:name="android.intent.action.VIEW" />
        <category android:name="android.intent.category.DEFAULT" />
        <category android:name="android.intent.category.BROWSABLE" />
        <data android:scheme="armorclaw" android:host="config" />
    </intent-filter>

    <intent-filter>
        <action android:name="android.intent.action.VIEW" />
        <category android:name="android.intent.category.DEFAULT" />
        <category android:name="android.intent.category.BROWSABLE" />
        <data android:scheme="armorclaw" android:host="setup" />
    </intent-filter>

    <intent-filter>
        <action android:name="android.intent.action.VIEW" />
        <category android:name="android.intent.category.DEFAULT" />
        <category android:name="android.intent.category.BROWSABLE" />
        <data android:scheme="armorclaw" android:host="invite" />
    </intent-filter>

    <!-- https:// web links -->
    <intent-filter android:autoVerify="true">
        <action android:name="android.intent.action.VIEW" />
        <category android:name="android.intent.category.DEFAULT" />
        <category android:name="android.intent.category.BROWSABLE" />
        <data android:scheme="https"
              android:host="armorclaw.app"
              android:pathPrefix="/config" />
    </intent-filter>

    <intent-filter android:autoVerify="true">
        <action android:name="android.intent.action.VIEW" />
        <category android:name="android.intent.category.DEFAULT" />
        <category android:name="android.intent.category.BROWSABLE" />
        <data android:scheme="https"
              android:host="armorclaw.app"
              android:pathPrefix="/invite" />
    </intent-filter>

    <intent-filter android:autoVerify="true">
        <action android:name="android.intent.action.VIEW" />
        <category android:name="android.intent.category.DEFAULT" />
        <category android:name="android.intent.category.BROWSABLE" />
        <data android:scheme="https"
              android:host="armorclaw.app"
              android:pathPrefix="/setup" />
    </intent-filter>
</activity>
```

### Complete Discovery Priority Order

```
1. SIGNED QR/DEEP LINK  ← Highest trust, pre-verified by bridge
       ↓ (if not available)
2. WELL-KNOWN          ← Standard Matrix discovery
       ↓ (if not available)
3. MDNS                ← Local network discovery
       ↓ (if not available)
4. MANUAL ENTRY        ← User provides URL
       ↓
5. FALLBACK SERVERS    ← Pre-configured backup servers
```

### File Changes Summary

| File | Action | Priority |
|------|--------|----------|
| `SetupService.kt` | Add `parseSignedConfig()` | HIGH |
| `RpcModels.kt` | Add `ConfigSource` enum | HIGH |
| `AndroidManifest.xml` | Add intent filters | HIGH |
| `build.gradle.kts` | Fix BuildConfig URLs (.app, /api) | CRITICAL |
| `ConnectServerScreen.kt` | Add QR scan button | MEDIUM |
| `DeepLinkHandler.kt` | NEW - route to SetupService | MEDIUM |
| `bridge/pkg/qr/public.go` | Fix `/rpc` → `/api` | CRITICAL |

### Bridge Server Required Changes

1. **Fix QR endpoint path** in `pkg/qr/public.go`:
   - Change `RpcURL: m.bridgeURL + "/rpc"` to `RpcURL: m.bridgeURL + "/api"`
   - ✅ DONE

2. **Add QR HTTP endpoints** in `pkg/http/server.go`:
   - `GET /qr/config` - Returns JSON with QR data
   - `GET /qr/image` - Returns PNG image (or redirect info)
   - ✅ DONE

3. **Add well-known endpoint** for Matrix standard discovery:
   - `GET /.well-known/matrix/client`
   - ✅ DONE

---

## OpenClaw Suggestions

OpenClaw is a self-hosted, open-source version of ArmorClaw. Here are suggestions for making discovery work:

### Discovery Architecture for OpenClaw

```
┌──────────────────────────────────────────────────────────────────────────┐
│                     OpenClaw Discovery Options                            │
├──────────────────────────────────────────────────────────────────────────┤
│                                                                          │
│  1. MANUAL ENTRY (Simplest)                                              │
│     - User enters: https://matrix.yourdomain.com                         │
│     - App derives: https://bridge.yourdomain.com/api                     │
│     - Works for any self-hosted deployment                               │
│                                                                          │
│  2. WELL-KNOWN DISCOVERY (Recommended)                                   │
│     - Serve at: https://yourdomain.com/.well-known/matrix/client         │
│     - Include Bridge URLs in response                                    │
│     - Standard Matrix discovery                                          │
│                                                                          │
│  3. mDNS DISCOVERY (Local Network)                                       │
│     - OpenClaw Bridge advertises via mDNS                                │
│     - ArmorChat discovers local servers automatically                     │
│     - Great for home/office deployments                                  │
│                                                                          │
│  4. QR CODE (Self-Hosted)                                                │
│     - OpenClaw generates signed config QR                                │
│     - Users scan QR to auto-configure                                    │
│     - Config can be signed with your own key                             │
│                                                                          │
└──────────────────────────────────────────────────────────────────────────┘
```

### OpenClaw Bridge Server Setup

#### 1. Well-Known Endpoint (nginx/Caddy)

```nginx
# /etc/nginx/sites-enabled/openclaw.conf
server {
    listen 443 ssl;
    server_name yourdomain.com;

    # Well-known discovery
    location /.well-known/matrix/client {
        default_type application/json;
        add_header Access-Control-Allow-Origin *;
        return 200 '{
            "m.homeserver": {"base_url": "https://matrix.yourdomain.com"},
            "com.armorclaw.bridge": {
                "base_url": "https://bridge.yourdomain.com",
                "api_endpoint": "https://bridge.yourdomain.com/api"
            }
        }';
    }
}
```

#### 2. OpenClaw Bridge Config

```toml
# /etc/openclaw/config.toml

[server]
hostname = "bridge.yourdomain.com"
port = 8443

[discovery]
# Enable mDNS advertisement
mdns_enabled = true
mdns_name = "OpenClaw Bridge"

# Matrix homeserver URL
matrix_homeserver = "https://matrix.yourdomain.com"

# Human-readable server name
server_name = "My OpenClaw Server"

[qr]
# QR code signing key (generate with: openssl rand -hex 32)
signing_key = "your-32-byte-hex-key"
```

#### 3. QR Code Generation

OpenClaw should expose:
- `GET /qr/config` - Generate config QR for ArmorChat
- `GET /qr/setup` - Generate setup QR for new device pairing

#### 4. Self-Signed Certificates

For local deployments without proper certificates:

```bash
# Generate self-signed certificate
openssl req -x509 -newkey rsa:4096 -keyout key.pem -out cert.pem -days 365 -nodes

# Or use the Bridge's built-in cert generation
openclaw-bridge --generate-certs
```

ArmorChat will show a warning for self-signed certificates but allow connection.

### OpenClaw Docker Compose Example

```yaml
version: '3.8'

services:
  # Matrix homeserver (Conduit - lightweight)
  conduit:
    image: girlbossceo/conduit:latest
    volumes:
      - conduit-data:/data
    environment:
      - CONDUIT_SERVER_NAME=yourdomain.com
    ports:
      - "6167:6167"

  # OpenClaw Bridge
  bridge:
    image: openclaw/bridge:latest
    volumes:
      - bridge-data:/data
      - bridge-certs:/certs
    environment:
      - OPENCLAW_HOSTNAME=bridge.yourdomain.com
      - OPENCLAW_MATRIX_URL=http://conduit:6167
      - OPENCLAW_MDNS_ENABLED=true
    ports:
      - "8443:8443"
    depends_on:
      - conduit

  # Reverse proxy
  caddy:
    image: caddy:latest
    ports:
      - "80:80"
      - "443:443"
    volumes:
      - ./Caddyfile:/etc/caddy/Caddyfile
      - caddy-data:/data
    depends_on:
      - conduit
      - bridge

volumes:
  conduit-data:
  bridge-data:
  bridge-certs:
  caddy-data:
```

### Caddyfile for OpenClaw

```
yourdomain.com {
    # Well-known discovery
    respond /.well-known/matrix/client `{
        "m.homeserver": {"base_url": "https://matrix.yourdomain.com"},
        "com.armorclaw.bridge": {
            "base_url": "https://bridge.yourdomain.com",
            "api_endpoint": "https://bridge.yourdomain.com/api"
        }
    }` 200

    header Access-Control-Allow-Origin *
}

matrix.yourdomain.com {
    reverse_proxy conduit:6167
}

bridge.yourdomain.com {
    reverse_proxy bridge:8443
}
```

### Self-Hosted Discovery Considerations

| Aspect | ArmorClaw (Cloud) | OpenClaw (Self-Hosted) |
|--------|-------------------|------------------------|
| Domain | `*.armorclaw.app` | Your own domain |
| SSL | Let's Encrypt (auto) | Let's Encrypt or self-signed |
| Discovery | Built-in well-known | Configure in reverse proxy |
| QR Signing | ArmorClaw signs | Your own key |
| Fallback | Multiple cloud servers | Your own fallback(s) |
| mDNS | Optional | Recommended for local |

### OpenClaw-Specific Recommendations

1. **Use Well-Known Discovery**
   - Easiest for users
   - Standard Matrix approach
   - One-time setup in reverse proxy

2. **Enable mDNS for Local Networks**
   - Auto-discovery in home/office
   - No DNS configuration needed
   - Works with local IP addresses

3. **Generate Config QR Codes**
   - Create QR for initial setup
   - Include in admin panel
   - Share via email/link

4. **Document Manual Setup**
   - Fallback if discovery fails
   - Clear instructions for users

5. **Certificate Warnings**
   - ArmorChat handles self-signed certs
   - Show warning but allow connection
   - Consider using Let's Encrypt for trusted certs

- [ ] All DNS records created and propagated
- [ ] Matrix homeserver running and accessible
- [ ] Bridge server running and accessible
- [ ] SSL certificates valid for all domains
- [ ] Health check returns 200 OK: `curl https://bridge.armorclaw.app/health`
- [ ] RPC endpoint works: `curl -X POST https://bridge.armorclaw.app/api -d '{"jsonrpc":"2.0","method":"system.health","id":1}'`
- [ ] Matrix login works via app
- [ ] Certificate pins added to `CertificatePinner.kt` (if using pinning)
- [ ] APK signed with release key
- [ ] APK tested on physical device

---

## Onboarding Flow

### User Onboarding Steps

```
Splash Screen
     │
     ├── hasCompletedOnboarding? ─────NO────▶ WelcomeScreen
     │                                            │
     │                                            ▼
     │                                      SecurityExplanationScreen (3 screens)
     │                                            │
     │                                            ▼
     │                                      ConnectServerScreen
     │                                            │
     │                                            ▼
     │                                      PermissionsScreen
     │                                            │
     │                                            ▼
     │                                      CompletionScreen
     │                                            │
     │◀────────────────────────────────────────────┘
     │
     ├── isLoggedIn? ─────NO────▶ LoginScreen
     │                                  │
     │                                  ▼
     │                            (Matrix Login)
     │                                  │
     │◀─────────────────────────────────┘
     │
     ▼
HomeScreen
```

### Server Connection Flow

```
User enters homeserver URL
         │
         ▼
   SetupService.startSetup()
         │
         ├──▶ detectServer() - Health check on bridge
         │         │
         │         ├── SUCCESS: Server info captured
         │         │
         │         └── FAIL: Try fallback servers
         │                   │
         │                   ├── bridge.armorclaw.app
         │                   ├── bridge-backup.armorclaw.app
         │                   └── bridge-eu.armorclaw.app
         │
         ▼
   User enters credentials
         │
         ▼
   SetupService.connectWithCredentials()
         │
         ├──▶ startBridge() - Start bridge session
         ├──▶ matrixLogin() - Login to Matrix (deprecated)
         │         │
         │         NOTE: Should use MatrixClient.login() instead!
         │
         ├──▶ wsClient.connect() - WebSocket connection (NON-FATAL)
         │         │
         │         NOTE: WS failure is caught and logged but does NOT
         │         block setup. ArmorChat uses Matrix /sync, not WS.
         │
         └──▶ getUserPrivilegesFromServer() - Get admin role
         │
         ▼
   Setup Complete
```

### How ArmorChat Finds ArmorClaw

1. **User-Provided URL:** User enters homeserver URL in ConnectServerScreen
2. **Bridge URL Derivation:** App derives bridge URL from homeserver:
   - `https://matrix.example.com` → `https://bridge.example.com`
3. **Health Check:** App calls `bridge.health` RPC to verify server
4. **Fallback:** If primary fails, tries known fallback servers
5. **Manual Override:** User can specify custom bridge URL in Advanced Options

### Quick Options Available

| Option | Homeserver | Bridge URL |
|--------|------------|------------|
| Demo Server | `https://demo.armorclaw.app` | `https://bridge-demo.armorclaw.app` |
| Local Development | `http://10.0.2.2:8008` | `http://10.0.2.2:8080` |

---

## ArmorChat: Signed Config Integration

### ✅ Implementation Status

| Component | File | Status |
|-----------|------|--------|
| SetupConfig (enhanced) | `app/.../bridge/SetupService.kt` | ✅ Done |
| ConfigSource (enhanced) | `app/.../bridge/SetupService.kt` | ✅ Done |
| SetupState (enhanced) | `app/.../bridge/SetupService.kt` | ✅ Done |
| SignedServerConfig | `app/.../bridge/SetupService.kt` | ✅ Done |
| parseSignedConfig() | `app/.../bridge/SetupService.kt` | ✅ Done |
| startSetupWithDiscovery() | `app/.../bridge/SetupService.kt` | ✅ Done |
| DiscoveredServer | `app/.../bridge/SetupService.kt` | ✅ Done |
| Deep Link Actions | `app/.../navigation/DeepLinkHandler.kt` | ✅ Done |
| Intent Filters | `app/.../AndroidManifest.xml` | ✅ Done |

### 📋 Android Components (To Create)

#### 1. ConfigRepository (Persistence)

Create `app/src/main/java/app/armorclaw/platform/config/ConfigRepository.android.kt`:

```kotlin
package app.armorclaw.platform.config

import android.content.Context
import android.content.SharedPreferences
import app.armorclaw.platform.bridge.*
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asStateFlow

class ConfigRepository(context: Context) {

    companion object {
        private const val PREFS_NAME = "armorclaw_config"
        private const val KEY_HOMESERVER = "matrix_homeserver"
        private const val KEY_BRIDGE_URL = "bridge_url"
        private const val KEY_WS_URL = "ws_url"
        private const val KEY_PUSH_GATEWAY = "push_gateway"
        private const val KEY_SERVER_NAME = "server_name"
        private const val KEY_CONFIG_SOURCE = "config_source"
        private const val KEY_EXPIRES_AT = "expires_at"

        @Volatile
        private var instance: ConfigRepository? = null

        fun getInstance(context: Context): ConfigRepository {
            return instance ?: synchronized(this) {
                instance ?: ConfigRepository(context.applicationContext).also {
                    instance = it
                }
            }
        }
    }

    private val prefs: SharedPreferences =
        context.getSharedPreferences(PREFS_NAME, Context.MODE_PRIVATE)

    private val _config = MutableStateFlow(loadConfig())
    val config: StateFlow<SetupConfig> = _config.asStateFlow()

    fun saveConfig(config: SetupConfig) {
        prefs.edit()
            .putString(KEY_HOMESERVER, config.homeserver)
            .putString(KEY_BRIDGE_URL, config.bridgeUrl)
            .putString(KEY_WS_URL, config.wsUrl)
            .putString(KEY_PUSH_GATEWAY, config.pushGateway)
            .putString(KEY_SERVER_NAME, config.serverName)
            .putString(KEY_CONFIG_SOURCE, config.configSource.name)
            .putLong(KEY_EXPIRES_AT, config.expiresAt ?: Long.MAX_VALUE)
            .apply()
        _config.value = config
    }

    fun loadConfig(): SetupConfig {
        if (!prefs.contains(KEY_HOMESERVER)) {
            return SetupConfig.createDefault()
        }
        val sourceName = prefs.getString(KEY_CONFIG_SOURCE, null)
        val source = try {
            sourceName?.let { ConfigSource.valueOf(it) } ?: ConfigSource.CACHED
        } catch (e: IllegalArgumentException) {
            ConfigSource.CACHED
        }
        return SetupConfig(
            homeserver = prefs.getString(KEY_HOMESERVER, "") ?: "",
            bridgeUrl = prefs.getString(KEY_BRIDGE_URL, null),
            wsUrl = prefs.getString(KEY_WS_URL, null),
            pushGateway = prefs.getString(KEY_PUSH_GATEWAY, null),
            serverName = prefs.getString(KEY_SERVER_NAME, null),
            configSource = source,
            expiresAt = prefs.getLong(KEY_EXPIRES_AT, Long.MAX_VALUE)
        )
    }

    fun clearConfig() {
        prefs.edit().clear().apply()
        _config.value = SetupConfig.createDefault()
    }

    fun isConfigured(): Boolean = _config.value.isConfigured()
}
```

#### 2. SetupViewModel (Android)

Create `app/src/main/java/app/armorclaw/ui/setup/SetupViewModel.kt`:

```kotlin
package app.armorclaw.ui.setup

import androidx.lifecycle.ViewModel
import androidx.lifecycle.viewModelScope
import app.armorclaw.platform.bridge.*
import app.armorclaw.platform.config.ConfigRepository
import kotlinx.coroutines.flow.*
import kotlinx.coroutines.launch

class SetupViewModel(
    private val setupService: SetupService,
    private val configRepository: ConfigRepository
) : ViewModel() {

    private val _state = MutableStateFlow<SetupState>(SetupState.Idle)
    val state: StateFlow<SetupState> = _state.asStateFlow()

    val config: StateFlow<SetupConfig> = configRepository.config

    init {
        if (configRepository.isConfigured()) {
            _state.value = SetupState.ReadyForCredentials
        }
    }

    fun startDiscovery(input: String) {
        viewModelScope.launch {
            _state.value = SetupState.Discovering
            val result = setupService.startSetupWithDiscovery(input)
            when (result) {
                is SetupResult.Success -> {
                    configRepository.saveConfig(setupService.config.value)
                    _state.value = SetupState.ReadyForCredentials
                }
                is SetupResult.Error -> {
                    _state.value = SetupState.Error(result.message, result.fallbackOptions)
                }
            }
        }
    }

    fun handleDeepLink(deepLink: String) {
        viewModelScope.launch {
            _state.value = SetupState.Discovering
            val result = setupService.parseSignedConfig(deepLink)
            when (result) {
                is SetupResult.Success -> {
                    configRepository.saveConfig(setupService.config.value)
                    _state.value = SetupState.ReadyForCredentials
                }
                is SetupResult.Error -> {
                    _state.value = SetupState.Error(result.message, result.fallbackOptions)
                }
            }
        }
    }

    fun useDemoServer() {
        viewModelScope.launch {
            val result = setupService.useDemoServer()
            when (result) {
                is SetupResult.Success -> {
                    configRepository.saveConfig(setupService.config.value)
                    _state.value = SetupState.ReadyForCredentials
                }
                is SetupResult.Error -> {
                    _state.value = SetupState.Error(result.message)
                }
            }
        }
    }

    fun reset() {
        setupService.resetSetup()
        configRepository.clearConfig()
        _state.value = SetupState.Idle
    }
}
```

#### 3. Koin DI Module

Create `app/src/main/java/app/armorclaw/di/ConfigModule.kt`:

```kotlin
package app.armorclaw.di

import app.armorclaw.platform.config.ConfigRepository
import app.armorclaw.platform.bridge.SetupService
import app.armorclaw.platform.bridge.BridgeRpcClient
import app.armorclaw.platform.bridge.BridgeWebSocketClient
import org.koin.android.ext.koin.androidContext
import org.koin.dsl.module

val configModule = module {
    single { ConfigRepository.getInstance(androidContext()) }

    factory { BridgeRpcClient(get()) }
    factory { BridgeWebSocketClient() }
    factory { SetupService(get(), get()) }
}
```

### Corrected Default URLs

```kotlin
// In SetupConfig.createDefault()
SetupConfig(
    homeserver = "https://matrix.armorclaw.app",      // NOT .com
    bridgeUrl = "https://bridge.armorclaw.app/api",   // NOT /rpc
    wsUrl = "wss://bridge.armorclaw.app/ws",
    pushGateway = "https://bridge.armorclaw.app/_matrix/push/v1/notify",
    serverName = "ArmorClaw",
    configSource = ConfigSource.DEFAULT
)
```

### Deep Link Flow Integration

```kotlin
// In MainActivity.kt or NavHost
when (action) {
    is DeepLinkAction.ApplySignedConfig -> {
        setupViewModel.handleDeepLink("armorclaw://config?d=${action.encodedData}")
        navController.navigate("onboarding/config")
    }
    is DeepLinkAction.SetupWithToken -> {
        setupViewModel.handleDeepLink(
            "armorclaw://setup?token=${action.token}&server=${action.server}"
        )
        navController.navigate("onboarding/setup")
    }
    is DeepLinkAction.AcceptInvite -> {
        navController.navigate("onboarding/invite?code=${action.code}")
    }
    // ... other actions
}
```

---

## 18. Testing Strategy

> **Status:** Execution-Grade (v3.5)
> **Last Updated:** April 29, 2026
> **Full Plans:** `.sisyphus/plans/ArmorChat_TestPlan_v3.5.md` and `.sisyphus/plans/Phase0_Appium_Prereqs.md`

### 18.1 Overview

ArmorChat uses the **FAME stack** (Finalrun, Appium, Maestro, Espresso) for integration testing, organized into three sequential phases:

| Phase | Focus | Primary Tools | Status |
|-------|-------|---------------|--------|
| **Phase 0** | Appium harness bootstrap + dual-client topology | Appium, CI | Prerequisite |
| **Phase 1** | Baseline: navigation, security, accessibility, admin | Maestro, Espresso/Compose | Ready to execute |
| **Phase 2** | Cross-client E2EE round-trip (after Phase 0) | Appium (dual-client) | Blocked on Phase 0 |

### 18.2 FAME Tool Ownership

| Tool | Use It For | Do NOT Use It For |
|------|-----------|-------------------|
| **Maestro** | Onboarding, deep links, admin traversal, black-box flows | Cross-client coordination, Compose semantics |
| **Espresso/Compose** | Trust semantics, encryption indicators, accessibility | Dual-client orchestration, external app interaction |
| **Appium** | Two-client message exchange, cross-app switching, full-stack E2EE round-trip | Fine-grained Compose semantics (unless accessibility mapping verified) |
| **Finalrun** | Animated visual evidence, flaky visual transitions | Primary pass/fail for core crypto correctness |

One primary FAME tool per test. Finalrun may supplement a passing test but may not convert a failing assertion into a pass.

### 18.3 Test Phases

#### Phase A — Environment Bootstrap & Capability Detection
- Record environment capabilities (`environment_capabilities.json`)
- Resolve package ID deterministically via `qa/scripts/resolve_package_id.sh`
- Generate route inventory (`route_inventory.json`) via `qa/scripts/generate_route_inventory.sh`
- Record environment capabilities via `qa/scripts/record_environment_capabilities.sh`

#### Phase B — Navigation & Route Coverage
- Verify all 55 route constants and 61 composable registrations
- Test top-level routes, parameterized routes, and deep links
- 15 source-confirmed deep links: `armorclaw://config`, `armorclaw://setup`, `armorclaw://invite`, `armorclaw://bond`, `armorclaw://team`, `armorclaw://secret/approve`, `armorclaw://email-approval/{gateId}`, `armorclaw://room/{roomId}`, `armorclaw://user/{userId}`, `armorclaw://chat/{roomId}`, `armorclaw://call/{callId}`, `armorclaw://settings`, `armorclaw://profile`, `https://matrix.to/#/{roomId}`, `https://chat.armorclaw.app/room/{roomId}`

#### Phase C — Security & Encryption
- **C1**: Key backup flow (Espresso/Compose)
- **C2**: Cross-signing bootstrap (Espresso/Compose)
- **C3a**: Selective Bridge message blocking (requires `selective_bridge_blocking_supported=true` in env)
- **C3b**: Broad message content verification (always required)
- **C4**: Per-message encryption indicators (Espresso/Compose regression gate)
- **C5**: SAS verification state machine (Espresso/Compose)
- **C6**: Trust badge rendering across 5 trust levels
- **C7**: Bridge governance workflow (requires frozen trigger path in `environment_capabilities.json`)

#### Phase D — Messaging & Sync (Topology-Aware)
- **D1**: E2EE send (Tier A: dual-UI Appium | Tier B: API verifier fallback)
- **D2**: E2EE receive (Tier A | Tier B)
- **D3**: Encryption indicator accuracy in encrypted room
- **D4**: Offline queue FIFO ordering
- **D5**: Session refresh under simulated expiry

#### Phase E — Diagnostics & Accessibility
- Debug TestHooks validation (6 hooks in `TestHooks.kt`)
- `UiStateContentProvider` content query
- WCAG 2.1 AA accessibility audit (top 10 screens)

#### Phase F — Admin Screen Traversal
- Agent management, HITL approvals, workflow management, budget status
- Bridge RPC admin operations (44 methods)

#### Phase G — Edge-to-Edge & Configuration Resilience
- IME/keyboard insets, configuration change, multi-window

### 18.4 Test Outcome Status Model

| Status | Meaning | Blocks Baseline? |
|--------|---------|-------------------|
| `PASS` | All assertions met | N/A |
| `FAIL` | One or more assertions failed | Yes |
| `SKIPPED_ENV` | Required capability unavailable | No (coverage gap) |
| `SKIPPED_BLOCKED_FIXTURE` | Required fixture does not exist | No (coverage gap) |

### 18.5 Pass Criteria

**Baseline Pass** (Phase 1):
- All Phase A environment checks pass
- Phase B: ≥90% route reachability
- Phase C: C1, C2, C3b, C4, C5, C6 pass (C3a if env supports; C7 records fixture status)
- Phase E: All TestHooks verified, accessibility audit complete
- Phase F: Admin traversal passes
- Phase G: Edge-to-edge passes

**Full Pass Tier A** (requires Phase 0 Appium infrastructure + dual-device):
- Baseline Pass achieved
- D1–D3 Tier A (dual-UI Appium) pass
- D4, D5 pass
- Zero D1–D3 skips

**Full Pass Tier B** (requires Phase 0 Appium infrastructure + API verifier):
- Baseline Pass achieved
- D1–D3 Tier B (API verifier fallback) pass
- D4, D5 pass
- `tier_b_verifier_type` recorded in `environment_capabilities.json`
- Evidence includes verifier crypto-capability statement and API traces
- **Note**: Tier B validates transport and integration behavior but does not prove full two-client user-facing E2EE UX

### 18.6 Phase 0 — Appium Infrastructure Prerequisites

Before Phase D (cross-client E2EE) can execute, the following must be delivered:

| Task | Deliverable |
|------|-------------|
| T0: Harness ADR | Audit existing `appium/` harness, decide extend vs. create |
| T1: Determinism scripts | `resolve_package_id.sh`, `generate_route_inventory.sh`, `record_environment_capabilities.sh` |
| T2: Environment config | `.env.test.example` with topology-conditional validation |
| T3: Topology manifest | `topology_manifest.json` with auth_mode + reset_strategy |
| T4: Desired capabilities | Dual-client Appium capabilities (ArmorChat + Element X) |
| T5: Auth-mode + reset co-validation | Validated auth/reset strategy pairs per execution context |
| T6: Device provisioning | Documentation for physical device setup |
| T7: CI lanes | Config-validation lane + device-backed lane with artifact archival |
| T8: Smoke test | Startup signal verification + Phase 2 prerequisite artifact archival |

**Phase 2 unlock gate** (gate 6): At least one successful device-backed smoke execution has archived all prerequisite artifacts: `topology_manifest.json`, smoke evidence, capability-load output, `route_inventory.json`, package resolution evidence, and `environment_capabilities.json`.

### 18.7 Tier B Verifier Contract

| Verifier Type | Crypto Capability | What It Proves |
|---------------|-------------------|----------------|
| `e2ee_capable` | Full Megolm encrypt/decrypt | Two-client E2EE round-trip correctness |
| `plaintext_api` | Transport-level send/receive only | Message transport and integration behavior |

The verifier type is recorded in `environment_capabilities.json` (`tier_b_verifier_type` field). A `plaintext_api` verifier cannot prove client-side E2EE correctness — evidence must explicitly note this coverage gap.

### 18.8 Source-Verified Test Infrastructure

| Component | Location | Count |
|-----------|----------|-------|
| Route constants | `AppNavigation.kt` | 55 |
| NavHost registrations | `ArmorClawNavHost.kt` | 61 |
| Deep links | `AndroidManifest.xml` + `DeepLinkHandler.kt` | 15 |
| Compose TestTags | `TestTags.kt` | 106 |
| Debug TestHooks | `TestHooks.kt` | 6 (TRIGGER_SYNC, EXPIRE_SESSION, NAVIGATE_TO, SET_NETWORK_STATE, RESET_NETWORK_STATE + UiStateContentProvider) |
| Maestro flows | `maestro/` | 27 |
| Instrumented test files | `app/src/androidTest/` | 14 |
| Existing Appium tests | `appium/tests/` | 10 |
| Unit tests | `app/src/test/` | 19 (pre-existing failures) |

### 18.9 Helper Scripts

All three scripts are created by Phase 0 Task 1 and must be run before test execution begins:

| Script | Output | Purpose |
|--------|--------|---------|
| `qa/scripts/resolve_package_id.sh` | Single `PACKAGE_ID` value | Deterministic package resolution |
| `qa/scripts/generate_route_inventory.sh` | `route_inventory.json` (schema v1) | Route count verification |
| `qa/scripts/record_environment_capabilities.sh` | `environment_capabilities.json` (schema v1) | Gating C3a, C7, and Tier B execution |

### 18.10 Constraints & Guardrails

- **Do NOT** treat Bridge RPC or Bridge WebSocket as the normal message transport path
- **Do NOT** use raw `adb shell input tap` or `uiautomator dump` when a FAME tool is capable
- **Do NOT** assume Compose `testTag` is a valid Appium locator by default
- **Do NOT** require Finalrun as a mandatory execution dependency
- **Do NOT** let C3b substitute for C3a when selective Bridge blocking is available
- **Do NOT** silently skip C7 — record fixture status in evidence
- **Do NOT** branch C7 across multiple trigger paths at runtime — follow the single frozen path
- **Do NOT** run Tier B without recording `tier_b_verifier_type` in evidence
- **Do NOT** claim Tier B with a plaintext verifier proves E2EE correctness
- **Do NOT** test against `ToDeviceProcessorStub` or expect `NotImplementedError` in crypto flows

### 18.11 Artifact Schema Contracts (v1)

Both the main test plan and Phase 0 plan use the same schema versions:

- **`route_inventory.json` schema v1**: `schema_version`, `generated_at`, `repository_snapshot`, `route_constants_count`, `navhost_registration_count`, `top_level_routes`, `hook_assisted_routes`, `excluded_routes`
- **`environment_capabilities.json` schema v1**: `schema_version`, `selective_bridge_blocking_supported`, `c7_fixture_status`, `c7_fixture_method`, `tier_b_verifier_type`

### 18.12 CI Integration

Current CI (`ci.yml`): Build + unit tests + detekt only. No Appium/device farm integration.

Phase 0 adds:
- **Config-validation lane** (7a): Validates `.env.test`, topology manifest, capability scripts — no device required
- **Device-backed lane** (7b): Runs smoke test on real device, archives Phase 2 prerequisite artifacts

---
