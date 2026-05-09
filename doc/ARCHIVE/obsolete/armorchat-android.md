# ArmorChat Android Client

> **Purpose**: LLM-readable documentation for the ArmorChat Android mobile application — actual architecture, working features, and current status.
>
> **Version**: 1.0.0 (build.gradle.kts versionName)
> **Last Updated**: 2026-05-04
> **Location**: `applications/ArmorChat/`

---

## 1. Executive Summary

ArmorChat is the Android mobile client for ArmorClaw. It provides a control interface for managing agents, approving sensitive operations (HITL), running secretary workflows, and managing security hardening. The app communicates with the Go Bridge via JSON-RPC 2.0 over HTTPS.

**Current state**: The app is a functional **Bridge remote control** — it can discover, connect to, and manage an ArmorClaw Bridge. It does NOT yet implement Matrix messaging (the Room screen is a placeholder and Matrix sync is a stub).

### Key Characteristics

| Aspect | Actual Implementation |
|--------|----------------------|
| **Language** | Kotlin (JVM) |
| **UI Framework** | Jetpack Compose (BOM 2024.01.00), Material 3 |
| **DI Framework** | Hilt / Dagger 2.50 |
| **Database** | DataStore Preferences 1.0.0 (no SQL) |
| **Network** | OkHttp 4.12.0 + Retrofit 2.9.0 |
| **Async** | Kotlin Coroutines, Flow, StateFlow |
| **Messaging Protocol** | Matrix Java SDK 1.6.10 (`org.matrix.android:matrix-sdk-android`) |
| **Bridge Communication** | JSON-RPC 2.0 over HTTPS |
| **Build Config** | compileSdk=34, targetSdk=34, Java 17, Compose Compiler 1.5.8 |

### Feature Status

| Feature | Status | Notes |
|---------|--------|-------|
| Bridge Discovery (mDNS + manual) | ✅ Working | `_armorclaw._tcp` NSD discovery + manual validation |
| Provisioning (QR + deep link) | ✅ Working | HMAC-SHA256 signed config, TOFU trust store |
| Security Hardening Wizard | ✅ Working | 5-step: password rotation, device verification, key backup, biometrics |
| HITL Approval (PII/MCP/Email) | ✅ Working | 3-tab approval screen with real Bridge RPC calls |
| Agent Management | ✅ Working | CRUD via `studio.deploy` RPC, instance listing |
| Secretary Workflows | ✅ Working | Template listing, start/cancel, blocker resolution, timeline |
| Email Approval | ✅ Working | Pending list, approve/deny via RPC |
| Push Notifications | ✅ Working | FCM + native Matrix HTTP Pusher |
| Account Deletion | ✅ Working | Password confirmation, `account.delete` RPC |
| Device Bonding | ✅ Working | Admin claiming with challenge-response |
| Security Config | ✅ Working | Per-category allow/deny permissions |
| Bridge Verification | ✅ Working | Emoji-based SAS verification |
| Matrix Messaging | ❌ Placeholder | Room screen is `PlaceholderScreen`, sync is `Thread.sleep(500)` |
| E2EE | ⏳ Stub | JNI bindings exist, all Olm/Megolm methods return "Not implemented" |
| Secrets Management | ⏳ UI Only | Screen renders but add/delete are no-ops |

---

## 2. Architecture Overview

ArmorChat uses a **flat package structure** under `app.armorclaw/`. There is no multi-layer clean architecture — ViewModels call the network layer directly, and data is persisted via EncryptedSharedPreferences.

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
User Action → ViewModel (Hilt) → BridgeApi (Retrofit/OkHttp) → Go Bridge (JSON-RPC 2.0)
                                          ↕
                              ResilientWebSocket (EventBus events)
                                          ↕
                              BridgeDiscovery (mDNS/NSD)
```

### Key Architectural Decisions

- **No local database**: All data comes from Bridge RPC calls. No Room DAO, no SQLCipher on the Android side.
- **Hilt DI**: All ViewModels use `@HiltViewModel` with `@Inject` constructor injection.
- **Encrypted preferences**: `ConfigManager` uses `EncryptedSharedPreferences` (AES256_SIV + AES256_GCM) for server config.
- **TOFU trust model**: `BridgeTrustStore` uses Trust-On-First-Use for bridge public keys.
- **Resilient networking**: `ResilientWebSocket` uses exponential backoff (1s-30s) with jitter, max 10 attempts.

---

## 3. Package Reference

### `config/` — Configuration & Trust (3 files)

| File | Purpose |
|------|---------|
| `ConfigManager.kt` | Encrypted `SharedPreferences` for `ServerConfig` (homeserver, RPC URL, WS URL, push gateway, bridge public key, expiration) |
| `BridgeTrustStore.kt` | TOFU trust store for bridge public keys via `SharedPreferences` |
| `SignedConfigParser.kt` | HMAC-SHA256 signed config parser for QR/deep-link provisioning |

### `crypto/` — Encryption (3 files)

| File | Purpose | Status |
|------|---------|--------|
| `CryptoService.kt` | AndroidKeyStore AES-256-GCM encryption, master key (10-year validity) | ✅ Working |
| `MatrixOlmService.kt` | Interface + stub `VodozemacOlmService` | ⏳ Stub |
| `VodozemacNative.kt` | JNI bindings for vodozemac Rust library (curve25519, ed25519, Olm, Megolm) | ⏳ All methods return failure |

### `data/` — Repositories & Models (6 files)

| File | Purpose |
|------|---------|
| `repository/BridgeRepository.kt` | Singleton: Matrix pusher registration, quick sync stub (`Thread.sleep(500)`) |
| `repository/UserRepository.kt` | In-memory `MutableStateFlow<UserEntity>` with namespace-tagged autocomplete |
| `repository/BridgeCapabilities.kt` | Feature/limitation inference per bridge protocol (9 protocols modeled) |
| `local/entity/Entities.kt` | `UserEntity` Room entity annotation |
| `model/EmailApprovalEvent.kt` | Email approval event model with `SystemAlertContent` conversion |
| `model/SystemAlert.kt` | Alert types/severities/factory: budget, license, security, PII, email, compliance |

### `navigation/` — Routing (3 files)

| File | Purpose |
|------|---------|
| `Route.kt` | 14 routes defined as sealed class |
| `ArmorClawNavHost.kt` | Nav graph with conditional start destination (config → bonding → hardening → home) |
| `DeepLinkHandler.kt` | `armorclaw://` URI parser for room and email approval deep links |

#### Navigation Routes (16 total)

**Onboarding flow** (conditional start):
```
bonding → hardening_password → hardening_device → key_backup → hardening_biometrics → security_config → home
```

**Main navigation**:
```
home → agent_management
home → approvals
home → workflow
home → account_deletion
```

**Standalone routes**:
```
key_recovery, migration, room/{roomId} (PLACEHOLDER), email/approve/{approvalId}, secrets
```

**Deep links**:
```
armorclaw://room/{roomId}           → Room (placeholder)
armorclaw://email/approve/{id}      → EmailApprovalScreen
armorclaw://config?d=...            → SignedConfigParser (not nav route)
```

### `network/` — Bridge Communication (4 files)

| File | Purpose | Lines |
|------|---------|-------|
| `BridgeApi.kt` | JSON-RPC 2.0 client with 40+ methods across 10+ domains | ~896 |
| `BridgeDiscovery.kt` | mDNS/NSD discovery on `_armorclaw._tcp` + manual connection validation | ~200 |
| `ResilientWebSocket.kt` | WebSocket with exponential backoff (1s-30s), jitter, message queueing | ~200 |
| `NetworkResilience.kt` | Network change detection, retry coordination, connection state monitoring | ~150 |

#### BridgeApi RPC Method Coverage

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

### `push/` — Notifications (4 files)

| File | Purpose |
|------|---------|
| `ArmorClawMessagingService.kt` | FCM service: 5 notification types (message, mention, invite, sync, email approval) |
| `PushTokenManager.kt` | FCM token lifecycle, migrated from legacy Bridge API to native Matrix HTTP Pusher |
| `MatrixPusherManager.kt` | Native Matrix HTTP Pusher registration via `/_matrix/client/v3/pushers/set` |
| `NotificationHelper.kt` | Notification builder: messaging style, mentions, summary bundling, encrypted indicator |

### `viewmodel/` — State Management (6 files)

| ViewModel | Purpose | Screens |
|-----------|---------|---------|
| `BondingViewModel` | Admin claiming: lockdown check, challenge-response | BondingScreen |
| `HardeningWizardViewModel` | 5-step wizard: password → device verify → key backup → biometrics | PasswordRotation, BridgeVerification, KeyBackup, BiometricEnable |
| `SecurityConfigViewModel` | Data category permissions (allow/deny per category) | SecurityConfigScreen |
| `HitlViewModel` | 3-tab HITL: PII access, MCP agent deploy, email approval | ApprovalScreen |
| `AgentManagementViewModel` | Agent CRUD, instance listing, skill-based creation | AgentScreen |
| `WorkflowViewModel` | Secretary workflow templates, start/cancel, blocker resolution, timeline | WorkflowScreen |

*Plus `DashboardViewModel` defined inline in `HomeScreen.kt` (agent count, pending approvals, running workflows).*

### `ui/` — Screens & Components

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

**13 shared components** in `ui/components/`:

| Component | Purpose |
|-----------|---------|
| WorkflowTimeline | Event timeline with progress bar, live/complete indicators |
| BlockerResponseDialog | Workflow blocker input dialog |
| PiiApprovalCard | PII access request card |
| EmailApprovalCard | Email approval card |
| BlindFillCard | BlindFill secret injection card |
| GovernanceBanner | Governance/compliance banner |
| SystemAlertMessage | System alert rendering |
| BridgeSecurityWarning | Bridge security downgrade warning |
| ErrorComponents | Reusable error states |
| MessageActions | Message action buttons |
| CallButtonController | Voice call state controller |
| ContextTransferDialog | Context transfer between agents |
| AutocompleteComponents | User mention autocomplete |

---

## 4. Security Architecture

### What's Actually Implemented

| Layer | Component | Status |
|-------|-----------|--------|
| **Config Storage** | `ConfigManager` — `EncryptedSharedPreferences` (AES256_SIV + AES256_GCM) | ✅ Working |
| **Bridge Trust** | `BridgeTrustStore` — TOFU model, constant-time comparison | ✅ Working |
| **Provisioning** | `SignedConfigParser` — HMAC-SHA256 config verification | ✅ Working |
| **Biometric Auth** | `BiometricEnableScreen` — AndroidX Biometric 1.2.0-alpha05 | ✅ Working |
| **Device Verification** | `BridgeVerificationScreen` — emoji-based SAS verification via Bridge RPC | ✅ Working |
| **Hardening** | `HardeningWizardViewModel` — password rotation, bootstrap wipe, device verify, key backup, biometrics | ✅ Working |
| **Local Encryption** | `CryptoService` — AndroidKeyStore AES-256-GCM, master key 10-year validity | ✅ Working |
| **E2EE (Matrix)** | `VodozemacNative` — JNI bindings for vodozemac Rust | ⏳ Stub — all methods return failure |
| **Passphrase Hashing** | `SetupRepository` — SHA-256 (comment says "argon2id in production") | ⚠️ Weak |

### Security Libraries (from build.gradle.kts)

| Dependency | Version | Purpose |
|------------|---------|---------|
| `androidx.security:security-crypto` | 1.1.0-alpha06 | EncryptedSharedPreferences |
| `androidx.biometric:biometric` | 1.2.0-alpha05 | BiometricPrompt integration |
| `com.google.dagger:hilt-android` | 2.50 | DI framework |

### What's NOT Implemented

- **No SQLCipher**: Data is persisted via `EncryptedSharedPreferences` and plain `SharedPreferences`. No database encryption.
- **No Matrix E2EE**: `VodozemacOlmService` is a stub. All Olm/Megolm methods return "Not implemented".
- **No secure message store**: No local message database. No Room DAO definitions.
- **No memory zeroization**: No `MemoryWiper` class exists.
- **No PII interception layer**: PII approval is handled by Bridge-side HITL, not client-side interception.

---

## 5. Communication Architecture

### Bridge Communication (Primary)

The app communicates with the Go Bridge via **JSON-RPC 2.0** over HTTPS:

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

Connection requires registration handshake:
```json
{"type":"register","payload":{"device_id":"..."}}
```

### mDNS Discovery

Local network discovery via NSD (Network Service Discovery):
- Service type: `_armorclaw._tcp`
- TXT records: Matrix URL, push gateway, TLS config, public key
- Fallback: manual IP/port entry with validation

### Push Notifications

```
FCM → ArmorClawMessagingService → NotificationHelper → System Notification
                                    ↕
                           PushTokenManager (FCM lifecycle)
                                    ↕
                           MatrixPusherManager (native Matrix HTTP Pusher)
```

Migrated in v4.5.0 from legacy Bridge API to native Matrix `/_matrix/client/v3/pushers/set`.

---

## 6. Build & Configuration

### Dependencies (from `app/build.gradle.kts`)

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
| `release` | Minified, proguard |

No product flavors. No multi-module setup.

---

## 7. ArmorTerminal Relationship

**ArmorTerminal** (`applications/ArmorTerminal/android-app/`) is a separate minimal Android app for pairing/configuration only.

- Package: `com.armorclaw.armorterminal` (vs `app.armorclaw`)
- Only 8 files in 3 packages: `config/`, `network/`, `viewmodel/`
- **6 files are near-identical copies** of ArmorChat code (different package names):
  - `BridgeDiscovery`, `BridgeApi`, `ResilientWebSocket`, `NetworkResilience`, `BridgeTrustStore`, `SignedConfigParser`
- Unique to ArmorTerminal: `PairingViewModel` (terminal-specific pairing flow)
- No shared module exists between the two apps — code is copy-pasted

---

## 8. Testing

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

## 9. Known Gaps & Tech Debt

| Gap | Impact | Priority |
|-----|--------|----------|
| No Matrix messaging implementation | App is a remote control, not a chat client | High |
| E2EE is stub only | No client-side encryption | High |
| Secrets management is UI-only (no-ops) | Cannot add/delete secrets from mobile | Medium |
| 6 files duplicated with ArmorTerminal | No shared module, maintenance burden | Medium |
| Passphrase hashing uses SHA-256 | Weak compared to argon2id | Low |
| No local message store | No offline capability | Low |
| `BridgeRepository.performQuickSync()` is `Thread.sleep(500)` | Fake sync placeholder | Low |

---

## 10. Quick Reference

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
