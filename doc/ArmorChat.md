# ArmorChat Project Review

> **Last Updated:** 2026-05-07
> **Version:** 1.0.0
> **Build Status:** Compiles
> **Architecture:** Android (Kotlin) + Jetpack Compose
> **Location:** `applications/ArmorChat/`

---

## Table of Contents

1. [Executive Summary](#1-executive-summary)
2. [Architecture Overview](#2-architecture-overview)
3. [Package Reference](#3-package-reference)
4. [Navigation System](#4-navigation-system)
5. [Bridge Communication](#5-bridge-communication)
6. [Security Architecture](#6-security-architecture)
7. [Push Notifications](#7-push-notifications)
8. [Feature Status](#8-feature-status)
9. [Build and Configuration](#9-build-and-configuration)
10. [Testing](#10-testing)
11. [Known Gaps and Tech Debt](#11-known-gaps-and-tech-debt)
12. [Quick Reference](#12-quick-reference)

---

## 1. Executive Summary

ArmorChat is the Android mobile client for ArmorClaw. It acts as a remote control for managing agents, approving sensitive operations (HITL), running secretary workflows, and managing security hardening. The app talks to the Go Bridge via JSON-RPC 2.0 over HTTPS.

**Current state:** The app is a functional Bridge remote control. It can discover, connect to, and manage an ArmorClaw Bridge. It does not yet implement Matrix messaging. The Room screen is a placeholder and Matrix sync is a stub.

### Key Characteristics

| Aspect | Implementation |
|--------|----------------|
| **Language** | Kotlin (JVM) |
| **UI Framework** | Jetpack Compose (BOM 2024.01.00), Material 3 |
| **DI Framework** | Hilt / Dagger 2.50 |
| **Persistence** | DataStore Preferences 1.0.0 (no SQL) |
| **Network** | OkHttp 4.12.0 + Retrofit 2.9.0 |
| **Async** | Kotlin Coroutines, Flow, StateFlow |
| **Messaging Protocol** | Matrix Java SDK 1.6.10 (`org.matrix.android:matrix-sdk-android`) |
| **Bridge Communication** | JSON-RPC 2.0 over HTTPS |
| **Build Config** | compileSdk=34, targetSdk=34, Java 17, Compose Compiler 1.5.8 |

---

## 2. Architecture Overview

ArmorChat uses a **flat package structure** under `app.armorclaw/`. There is no multi-layer clean architecture. ViewModels call the network layer directly, and data is persisted via EncryptedSharedPreferences.

```
app.armorclaw/
├── MainActivity.kt              # Single Activity, Compose entry point
├── config/       (3 files)      # Encrypted config, TOFU trust, signed config parser
├── crypto/       (3 files)      # AndroidKeyStore AES-256-GCM, Vodozemac JNI stubs
├── data/         (6 files)      # Repositories, models, entities (in-memory + DataStore)
├── navigation/   (3 files)      # 14 routes, NavHost, deep link handler
├── network/      (4 files)      # Bridge API (JSON-RPC), mDNS discovery, WebSocket
├── push/         (4 files)      # FCM, Matrix HTTP Pusher, notification builder
├── repository/   (1 file)       # Setup/bonding operations
├── ui/           (~25 files)    # Screens + 13 shared components
├── utils/        (1 file)       # Error handler with retry
├── validation/   (1 file)       # CI/ADB validation state export
└── viewmodel/    (6 files)      # ViewModels (Hilt-injected)
```

**Total: ~61 source files across 14 packages.**

### Data Flow

```
User Action
    │
    ▼
ViewModel (Hilt @HiltViewModel)
    │
    ├──▶ BridgeApi (Retrofit/OkHttp) ──▶ Go Bridge (JSON-RPC 2.0)
    │
    ├──▶ ResilientWebSocket ──▶ Bridge EventBus
    │
    └──▶ BridgeDiscovery (mDNS/NSD)
```

### Key Architectural Decisions

- **No local database.** All data comes from Bridge RPC calls. No Room DAO, no SQLCipher on the Android side.
- **Hilt DI.** All ViewModels use `@HiltViewModel` with `@Inject` constructor injection. No Koin modules, no manual service registration.
- **Encrypted preferences.** `ConfigManager` uses `EncryptedSharedPreferences` (AES256_SIV + AES256_GCM) for server config.
- **TOFU trust model.** `BridgeTrustStore` uses Trust-On-First-Use for bridge public keys.
- **Resilient networking.** `ResilientWebSocket` uses exponential backoff (1s to 30s) with jitter, max 10 attempts.

---

## 3. Package Reference

### `config/` (3 files)

| File | Purpose |
|------|---------|
| `ConfigManager.kt` | Encrypted `SharedPreferences` for `ServerConfig` (homeserver, RPC URL, WS URL, push gateway, bridge public key, expiration) |
| `BridgeTrustStore.kt` | TOFU trust store for bridge public keys via `SharedPreferences` |
| `SignedConfigParser.kt` | HMAC-SHA256 signed config parser for QR/deep-link provisioning |

### `crypto/` (3 files)

| File | Purpose | Status |
|------|---------|--------|
| `CryptoService.kt` | AndroidKeyStore AES-256-GCM encryption, master key (10-year validity) | Working |
| `MatrixOlmService.kt` | Interface + stub `VodozemacOlmService` | Stub |
| `VodozemacNative.kt` | JNI bindings for vodozemac Rust library (curve25519, ed25519, Olm, Megolm) | All methods return failure |

### `data/` (6 files)

| File | Purpose |
|------|---------|
| `repository/BridgeRepository.kt` | Singleton: Matrix pusher registration, quick sync stub (`Thread.sleep(500)`) |
| `repository/UserRepository.kt` | In-memory `MutableStateFlow<UserEntity>` with namespace-tagged autocomplete |
| `repository/BridgeCapabilities.kt` | Feature/limitation inference per bridge protocol (9 protocols modeled) |
| `local/entity/Entities.kt` | `UserEntity` Room entity annotation |
| `model/EmailApprovalEvent.kt` | Email approval event model with `SystemAlertContent` conversion |
| `model/SystemAlert.kt` | Alert types/severities/factory: budget, license, security, PII, email, compliance |

### `network/` (4 files)

| File | Purpose | Lines |
|------|---------|-------|
| `BridgeApi.kt` | Concrete Retrofit service, JSON-RPC 2.0 client with 40+ methods across 10+ domains | ~896 |
| `BridgeDiscovery.kt` | mDNS/NSD discovery on `_armorclaw._tcp` + manual connection validation | ~200 |
| `ResilientWebSocket.kt` | WebSocket with exponential backoff (1s to 30s), jitter, message queueing | ~200 |
| `NetworkResilience.kt` | Network change detection, retry coordination, connection state monitoring | ~150 |

### `viewmodel/` (6 files)

| ViewModel | Purpose | Screens |
|-----------|---------|---------|
| `BondingViewModel` | Admin claiming: lockdown check, challenge-response | BondingScreen |
| `HardeningWizardViewModel` | 5-step wizard: password, device verify, key backup, biometrics | PasswordRotation, BridgeVerification, KeyBackup, BiometricEnable |
| `SecurityConfigViewModel` | Data category permissions (allow/deny per category) | SecurityConfigScreen |
| `HitlViewModel` | 3-tab HITL: PII access, MCP agent deploy, email approval | ApprovalScreen |
| `AgentManagementViewModel` | Agent CRUD, instance listing, skill-based creation | AgentScreen |
| `WorkflowViewModel` | Secretary workflow templates, start/cancel, blocker resolution, timeline | WorkflowScreen |

`DashboardViewModel` is defined inline in `HomeScreen.kt` (agent count, pending approvals, running workflows).

### `ui/` (Screens and Components)

**15 composable screens:**

| Screen | Route | Purpose |
|--------|-------|---------|
| BondingScreen | `bonding` | Admin ownership claiming |
| PasswordRotationScreen | `hardening_password` | Rotate bootstrap password (wizard step 1) |
| BridgeVerificationScreen | `hardening_device` | Emoji-based device verification (wizard step 3) |
| KeyBackupScreen | `key_backup` | Recovery key backup (wizard step 4) |
| BiometricEnableScreen | `hardening_biometrics` | Biometric enrollment (wizard step 5) |
| SecurityConfigScreen | `security_config` | Data category permission grid |
| HomeScreen | `home` | Dashboard: agent count, approvals, workflows |
| AgentScreen | `agent_management` | Agent Studio: list, detail, create, instances |
| WorkflowScreen | `workflow` | Templates, start, timeline, blocker resolution |
| ApprovalScreen | `approvals` | 3-tab HITL: PII / MCP / Email |
| EmailApprovalScreen | `email/approve/{id}` | Single email approval detail |
| SecretsScreen | `secrets` | API key/credential management (UI only) |
| KeyRecoveryScreen | `key_recovery` | Recovery from backup |
| MigrationScreen | `migration` | v2.5 to secure architecture migration |
| AccountDeletionScreen | `account_deletion` | Account deactivation |

**13 shared components** in `ui/components/`: WorkflowTimeline, BlockerResponseDialog, PiiApprovalCard, EmailApprovalCard, BlindFillCard, GovernanceBanner, SystemAlertMessage, BridgeSecurityWarning, ErrorComponents, MessageActions, CallButtonController, ContextTransferDialog, AutocompleteComponents.

---

## 4. Navigation System

All routes are defined as a **sealed class** in `Route.kt` with **14 routes total**. The NavHost lives in `ArmorClawNavHost.kt`.

### Navigation Routes

**Onboarding flow** (conditional start destination):

```
bonding → hardening_password → hardening_device → key_backup → hardening_biometrics → security_config → home
```

**Main navigation:**

```
home → agent_management
home → approvals
home → workflow
home → account_deletion
```

**Standalone routes:**

```
key_recovery, migration, room/{roomId} (PLACEHOLDER), email/approve/{approvalId}, secrets
```

### Deep Links

| URI Pattern | Target |
|-------------|--------|
| `armorclaw://room/{roomId}` | Room (placeholder) |
| `armorclaw://email/approve/{id}` | EmailApprovalScreen |
| `armorclaw://config?d=...` | SignedConfigParser (not a nav route) |

Deep link handling is in `navigation/DeepLinkHandler.kt`.

---

## 5. Bridge Communication

### Primary Channel: JSON-RPC 2.0 over HTTPS

The app communicates with the Go Bridge via `BridgeApi.kt`, a concrete Retrofit service. This is not an interface with pluggable implementations. It is a single class that handles all RPC calls.

```
ArmorChat → OkHttp/Retrofit → HTTPS → Go Bridge (port 8080 or Unix socket)
                                       ↕
                                  JSON-RPC 2.0
```

### WebSocket (Real-time Events)

Real-time events (workflow progress, approval requests) flow through WebSocket:

```
Go Bridge EventBus → WebSocket (/ws) → ResilientWebSocket → ViewModels
```

Connection requires a registration handshake:
```json
{"type":"register","payload":{"device_id":"..."}}
```

### mDNS Discovery

Local network discovery via NSD (Network Service Discovery):
- Service type: `_armorclaw._tcp`
- TXT records: Matrix URL, push gateway, TLS config, public key
- Fallback: manual IP/port entry with validation

### BridgeApi RPC Method Coverage

| Domain | Methods |
|--------|---------|
| Lockdown | `lockdown.status`, `lockdown.get_challenge`, `lockdown.claim_ownership`, `lockdown.transition` |
| Security | `security.get_categories`, `security.set_category`, `security.get_tiers`, `security.set_tier` |
| Device | `device.list`, `device.approve`, `device.reject`, `device.start_verification`, `device.confirm_verification`, `device.cancel_verification` |
| Invite | `invite.create`, `invite.list`, `invite.revoke` |
| PII | `pii.approve_access`, `pii.reject_access` |
| Email | `approve_email`, `deny_email`, `email.list_pending` |
| Hardening | `hardening.status`, `hardening.ack`, `hardening.rotate_password` |
| Secretary | `secretary.start/get/cancel/advance_workflow`, `secretary.list/create/get/delete/update_template` |
| Task | `task.create`, `task.list`, `task.cancel`, `task.get` |
| Studio | `studio.deploy` (with actions: list/get/create/delete agents, list instances), `studio.stats` |
| Provisioning | `provisioning.claim` |
| QR | `qr.generate_setup` |
| Account | `account.delete` |
| Blocker | `resolve_blocker` |
| Recovery | `recovery.create_backup` |
| Push | `push.register_token`, `push.unregister_token` |

---

## 6. Security Architecture

### What is Actually Implemented

| Layer | Component | Status |
|-------|-----------|--------|
| **Config Storage** | `ConfigManager` with `EncryptedSharedPreferences` (AES256_SIV + AES256_GCM) | Working |
| **Bridge Trust** | `BridgeTrustStore` with TOFU model, constant-time comparison | Working |
| **Provisioning** | `SignedConfigParser` with HMAC-SHA256 config verification | Working |
| **Biometric Auth** | `BiometricEnableScreen` with AndroidX Biometric 1.2.0-alpha05 | Working |
| **Device Verification** | `BridgeVerificationScreen` with emoji-based SAS verification via Bridge RPC | Working |
| **Hardening** | `HardeningWizardViewModel` with password rotation, bootstrap wipe, device verify, key backup, biometrics | Working |
| **Local Encryption** | `CryptoService` with AndroidKeyStore AES-256-GCM, master key 10-year validity | Working |
| **E2EE (Matrix)** | `VodozemacNative` with JNI bindings for vodozemac Rust | Stub, all methods return failure |
| **Passphrase Hashing** | `SetupRepository` with SHA-256 (comment says "argon2id in production") | Weak |

### Security Libraries

| Dependency | Version | Purpose |
|------------|---------|---------|
| `androidx.security:security-crypto` | 1.1.0-alpha06 | EncryptedSharedPreferences |
| `androidx.biometric:biometric` | 1.2.0-alpha05 | BiometricPrompt integration |
| `com.google.dagger:hilt-android` | 2.50 | DI framework |

### What is NOT Implemented

- **No SQLCipher on Android.** Data is persisted via `EncryptedSharedPreferences` and plain `SharedPreferences`. No database encryption on the client side. (SQLCipher is used on the Go Bridge side, not in the Android app.)
- **No Matrix E2EE.** `VodozemacOlmService` is a stub. All Olm/Megolm methods return "Not implemented".
- **No local message database.** No Room DAO definitions, no message store.
- **No secure clipboard.** No `SecureClipboard` class exists.
- **No memory zeroization.** No `MemoryWiper` class exists.
- **No PII interception layer on the client.** PII approval is handled by Bridge-side HITL, not client-side interception.

---

## 7. Push Notifications

```
FCM → ArmorClawMessagingService → NotificationHelper → System Notification
                                    ↕
                           PushTokenManager (FCM lifecycle)
                                    ↕
                           MatrixPusherManager (native Matrix HTTP Pusher)
```

### Push Notification Files

| File | Purpose |
|------|---------|
| `ArmorClawMessagingService.kt` | FCM service: 5 notification types (message, mention, invite, sync, email approval) |
| `PushTokenManager.kt` | FCM token lifecycle, migrated from legacy Bridge API to native Matrix HTTP Pusher |
| `MatrixPusherManager.kt` | Native Matrix HTTP Pusher registration via `/_matrix/client/v3/pushers/set` |
| `NotificationHelper.kt` | Notification builder: messaging style, mentions, summary bundling, encrypted indicator |

Push registration was migrated from legacy Bridge API to native Matrix `/_matrix/client/v3/pushers/set`.

---

## 8. Feature Status

### Working Features

| Feature | Notes |
|---------|-------|
| Bridge Discovery (mDNS + manual) | `_armorclaw._tcp` NSD discovery + manual validation |
| Provisioning (QR + deep link) | HMAC-SHA256 signed config, TOFU trust store |
| Security Hardening Wizard | 5-step: password rotation, device verification, key backup, biometrics |
| HITL Approval (PII/MCP/Email) | 3-tab approval screen with real Bridge RPC calls |
| Agent Management | CRUD via `studio.deploy` RPC, instance listing |
| Secretary Workflows | Template listing, start/cancel, blocker resolution, timeline |
| Email Approval | Pending list, approve/deny via RPC |
| Push Notifications | FCM + native Matrix HTTP Pusher |
| Account Deletion | Password confirmation, `account.delete` RPC |
| Device Bonding | Admin claiming with challenge-response |
| Security Config | Per-category allow/deny permissions |
| Bridge Verification | Emoji-based SAS verification |

### Not Yet Implemented

| Feature | Status | Notes |
|---------|--------|-------|
| Matrix Messaging | Placeholder | Room screen is `PlaceholderScreen`, sync is `Thread.sleep(500)` |
| E2EE | Stub | JNI bindings exist, all Olm/Megolm methods return "Not implemented" |
| Secrets Management | UI Only | Screen renders but add/delete are no-ops |

---

## 9. Build and Configuration

### Dependencies

| Category | Dependency | Version |
|----------|-----------|---------|
| **DI** | `com.google.dagger:hilt-android` | 2.50 |
| **Compose BOM** | `androidx.compose:compose-bom` | 2024.01.00 |
| **Compose Compiler** | (extension) | 1.5.8 |
| **Matrix SDK** | `org.matrix.android:matrix-sdk-android` | 1.6.10 |
| **Network** | `com.squareup.okhttp3:okhttp` | 4.12.0 |
| **Network** | `com.squareup.retrofit2:retrofit` | 2.9.0 |
| **Security** | `androidx.security:security-crypto` | 1.1.0-alpha06 |
| **Security** | `androidx.biometric:biometric` | 1.2.0-alpha05 |
| **Data** | `androidx.datastore:datastore-preferences` | 1.0.0 |
| **Navigation** | `androidx.navigation:navigation-compose` | 2.7.6 |
| **WebSockets** | `com.squareup.okhttp3:okhttp` (built-in) | 4.12.0 |

### Build Config Fields

| Field | Purpose |
|-------|---------|
| `BRIDGE_API_URL` | Bridge RPC endpoint |
| `BRIDGE_WS_URL` | Bridge WebSocket endpoint |
| `MATRIX_PUSH_GATEWAY` | Sygnal push gateway URL |

### Build Types

| Type | Configuration |
|------|---------------|
| `debug` | Debuggable, default config |
| `release` | Minified, ProGuard |

No product flavors. No multi-module setup.

### Build Commands

```bash
# Build Debug APK
./gradlew assembleDebug

# Build Release APK
./gradlew assembleRelease

# Install Debug
./gradlew installDebug

# Unit Tests
./gradlew test

# Instrumented Tests
./gradlew connectedAndroidTest
```

---

## 10. Testing

### Test Files

| Location | Count | Type |
|----------|-------|------|
| `app/src/test/` | 12 | Unit tests (JVM) |
| `app/src/androidTest/` | 1 | Instrumented test |
| **Total** | **13** | |

### Test Framework

| Dependency | Version |
|------------|---------|
| `junit` | 4.13.2 |
| `androidx.test.ext:junit` | 1.1.5 |
| `androidx.compose.ui:ui-test-junit4` | (from BOM) |

---

## 11. Known Gaps and Tech Debt

| Gap | Impact | Priority |
|-----|--------|----------|
| No Matrix messaging implementation | App is a remote control, not a chat client | High |
| E2EE is stub only | No client-side encryption | High |
| Secrets management is UI-only (no-ops) | Cannot add/delete secrets from mobile | Medium |
| 6 files duplicated with ArmorTerminal | No shared module, maintenance burden | Medium |
| Passphrase hashing uses SHA-256 | Weak compared to argon2id | Low |
| No local message store | No offline capability | Low |
| `BridgeRepository.performQuickSync()` is `Thread.sleep(500)` | Fake sync placeholder | Low |

### ArmorTerminal Duplication

ArmorTerminal (`applications/ArmorTerminal/android-app/`) is a separate minimal Android app for pairing/configuration only. It shares 6 near-identical files with ArmorChat (different package names): `BridgeDiscovery`, `BridgeApi`, `ResilientWebSocket`, `NetworkResilience`, `BridgeTrustStore`, `SignedConfigParser`. No shared module exists between the two apps.

---

## 12. Quick Reference

| "I need to..." | File to read/edit |
|----------------|-------------------|
| Add a new screen | Create in `ui/`, add route in `navigation/Route.kt`, register in `navigation/ArmorClawNavHost.kt` |
| Add a new RPC method call | Add to `network/BridgeApi.kt` |
| Change Bridge discovery | `network/BridgeDiscovery.kt` |
| Modify HITL approval flow | `viewmodel/HitlViewModel.kt` + `ui/approval/ApprovalScreen.kt` |
| Change security settings | `viewmodel/SecurityConfigViewModel.kt` + `ui/security/SecurityConfigScreen.kt` |
| Add push notification type | `push/ArmorClawMessagingService.kt` + `push/NotificationHelper.kt` |
| Change config persistence | `config/ConfigManager.kt` |
| Modify hardening wizard | `viewmodel/HardeningWizardViewModel.kt` + `ui/security/` screens |
| Add a new ViewModel | Create in `viewmodel/`, annotate with `@HiltViewModel` |
| Change WebSocket handling | `network/ResilientWebSocket.kt` |
| Modify deep link routing | `navigation/DeepLinkHandler.kt` + `navigation/ArmorClawNavHost.kt` |
