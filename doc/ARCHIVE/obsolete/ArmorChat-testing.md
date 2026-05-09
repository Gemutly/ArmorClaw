# Testing Reference

Complete inventory of every test class, test method, framework, and infrastructure utility in the ArmorChat codebase.

## At a Glance

| Metric | Value |
|--------|-------|
| Total test files | 86 |
| Total `@Test` methods | ~630 |
| Test source sets | 4 |
| Test frameworks | JUnit 4, MockK, Robolectric, WireMock, Turbine, Compose Test, Espresso |

| Source Set | Files | Tests | Type |
|------------|-------|-------|------|
| `shared/src/commonTest/` | 30 | ~238 | Pure JVM unit tests |
| `shared/src/androidUnitTest/` | 12 | ~91 | Robolectric + integration (WireMock) |
| `androidApp/src/test/` | 32 | ~258 | Robolectric + MockK unit tests |
| `androidApp/src/androidTest/` | 12 | ~43 | On-device instrumented tests |

## Running Tests

```bash
# Full unit test suite (both modules)
./gradlew test

# Per-module unit tests
./gradlew :shared:testDebugUnitTest
./gradlew :androidApp:testDebugUnitTest

# Specific test class
./gradlew :shared:testDebugUnitTest --tests "integration.GovernanceStubTest"
./gradlew :androidApp:testDebugUnitTest --tests "com.armorclaw.app.viewmodels.DeviceApprovalViewModelTest"

# Instrumented tests (requires device/emulator)
./gradlew connectedDebugAndroidTest

# Build instrumented test APK without running
./gradlew :androidApp:assembleDebugAndroidTest

# Static analysis
./gradlew detekt

# Coverage report
./gradlew jacocoTestReport
```

## Test Frameworks & Dependencies

| Framework | Version | Purpose | Used In |
|-----------|---------|---------|---------|
| JUnit 4 | 4.13.2 | Test runner and assertions | All source sets |
| kotlin-test | 1.9.20 | Kotlin-native assertions | All source sets |
| MockK | 1.13.8 | Mocking (mockk, coEvery, coVerify) | androidApp/test, shared/androidUnitTest |
| Robolectric | 4.11.1 | Android runtime on JVM | shared/androidUnitTest, androidApp/test |
| Turbine | 1.0.0 | Flow testing (.test {}) | androidApp/test |
| WireMock | 3.3.1 | HTTP stub server | shared/androidUnitTest |
| Compose UI Test | 1.5.0 | Compose assertions | shared/androidUnitTest, androidApp/test, androidApp/androidTest |
| Espresso | 3.5.1 | Android UI assertions | androidApp/androidTest |
| Coroutines Test | 1.7.3 | runTest, TestDispatcher | All source sets |

## Build Configuration

### shared/build.gradle.kts

```kotlin
android {
    testOptions {
        unitTests {
            isReturnDefaultValues = true
            isIncludeAndroidResources = true  // Required for Robolectric Compose tests
        }
    }
}

// Jetty excluded from AndroidTest to prevent dexing errors on API <26
configurations.configureEach {
    if (name.contains("AndroidTest", ignoreCase = true)) {
        exclude(group = "org.eclipse.jetty")
    }
}
```

### androidApp/build.gradle.kts

```kotlin
android {
    testOptions {
        unitTests {
            isIncludeAndroidResources = true
            isReturnDefaultValues = true
        }
        unitTests.all {
            it.jvmArgs("-Xmx2048m", "-XX:+HeapDumpOnOutOfMemoryError")
        }
    }
}
```

---

## Test Inventory by Source Set

---

### 1. shared/src/commonTest/ (Pure JVM Unit Tests)

30 test files, ~238 test methods. Run on JVM without Android dependencies. Uses `kotlin-test` assertions and `kotlinx-coroutines-test`.

#### Domain Layer -- Models

| Test Class | Tests | What It Validates |
|------------|-------|-------------------|
| `TrustModelTest` | 18 | TrustLevel isTrusted/isVerified, VerificationState states, EmojiInfo (64 emojis), DeviceInfo, CrossSigningKeys, UserSession serialization |
| `PiiAccessRequestTest` | 8 | PII request expiration, critical field detection, sensitivity grouping, biometric requirements |
| `TeamEventsTest` | 5 | TeamCreated/MemberAdded/Dissolved deserialization, round-trip, event type constants |
| `AgentStatusEventTest` | 5 | AgentTaskStatus intervention detection, active status, display strings |
| `MessageTest` | 3 | Message serialization/deserialization, attachments |
| `ThreadInfoTest` | 10 | ThreadInfo serialization, isThreadRoot/isInThread, getSummary, Reaction model, message-with-thread |
| `RoomTest` | 4 | Room serialization, direct rooms, room with members |
| `KeystoreStatusTest` | 11 | Sealed/Unsealed/Error states, isAccessible, isExpired, remainingTimeMs, remainingTimeString, KEYSTORE_SESSION_DURATION_MS (4h) |
| `ArmorClawErrorCodeTest` | 11 | Error code lookup (fromCode/fromCategory), recoverable flags, user messages, code uniqueness |
| `CallModelTest` | 31 | CallSession, CallState, CallParticipant, ConnectionQuality, HangupReason, CallStatistics, CallPermissions, SDP, ICE, BandwidthLimit |
| `UnifiedMessageTest` | 23 | Regular/Agent/System/Command messages, MessageSender types, isFromCurrentUser/canReact/canReply/canEdit, AgentType/AgentStatus |

#### Domain Layer -- Security

| Test Class | Tests | What It Validates |
|------------|-------|-------------------|
| `MemoryWiperTest` | 10 | CharArray/ByteArray wiping, wipeAndNull, double-wipe safety |

#### Data Layer -- Stores

| Test Class | Tests | What It Validates |
|------------|-------|-------------------|
| `RealTimeEventStoreTest` | 41 | Typed flow emissions: messages, typing, presence, receipts, room memberships, call signaling, room state changes, errors, browser/agent/PII events, room filtering, delegation |
| `ControlPlaneStoreTest` | 27 | Workflow lifecycle, agent tasks, thinking agents, budget warnings, bridge status, PII request management, keystore status, agent intervention RPC integration |
| `InviteRepositoryImplTest` | 9 | RPC-to-domain mapping for create/list/revoke invites |
| `DeviceGovernanceRepositoryImplTest` | 8 | RPC-to-domain mapping for device governance operations |

#### Platform Layer -- Bridge RPC

| Test Class | Tests | What It Validates |
|------------|-------|-------------------|
| `ProvisioningRegressionTest` | 8 | Wire name regression tests (Bug #12: provisioning.rotate vs rotate_secret) |
| `BridgeLifecycleTest` | 20 | start/status/stop/health lifecycle, state transitions, wire names, OperationContext |
| `AgentInterventionTest` | 16 | ReportIntervention/ResolveBlocker serialization, edge cases, Result contracts |
| `AgentStatusMapperTest` | 2 | String-to-AgentTaskStatus mapping |
| `BridgeRpcClientTest` | 48 | RpcResult, BridgeConfig, JSON-RPC serialization, response models, MockBridgeRpcClient, OperationContext, IceServer, WebRTC models |
| `BridgeServiceTest` (6 inner classes) | 19 | InviteService (generate/parse/expire/revoke), SetupService state, SecurityWarning types, InviteExpiration, AdminLevel, SetupState |
| `BridgeWebSocketClientTest` | 29 | WebSocketState, BridgeEvent serialization (13 types), MockBridgeWebSocketClient, exponential backoff |

#### Platform Layer -- Matrix

| Test Class | Tests | What It Validates |
|------------|-------|-------------------|
| `MatrixSyncManagerTest` | ~30 | Event routing: timeline, state, ephemeral, account data, to-device, presence, invite, left room events |

#### UI Layer -- Components

| Test Class | Tests | What It Validates |
|------------|-------|-------------------|
| `AgentThinkingIndicatorTest` | ~4 | Thinking indicator rendering |
| `WorkflowCardTest` | ~4 | Workflow card rendering |
| `WorkflowProgressBannerTest` | ~4 | Progress banner rendering |

#### ViewModel Layer

| Test Class | Tests | What It Validates |
|------------|-------|-------------------|
| `ChatViewModelUnifiedTest` | ~8 | ChatViewModel unified message handling |

---

### 2. shared/src/androidUnitTest/ (Robolectric + Integration Tests)

12 test files, ~91 test methods. Requires Android runtime via Robolectric. Integration tests use WireMock for HTTP stubbing.

#### Integration Tests

| Test Class | Tests | What It Validates |
|------------|-------|-------------------|
| `GovernanceStubTest` | 10 | All 8 governance RPC stubs (device.list_pending, approve, reject, list, revoke, invite.create/list/revoke), auth rejection (-32001), unknown method (-32601) |
| `WireMockSetupTest` | 8 | WireMock infrastructure: Matrix login/sync/profile stubs, Bridge health/RPC stubs, TestConfig loading, QR config fixture, stub mapping files |

#### Platform Layer -- Matrix

| Test Class | Tests | What It Validates |
|------------|-------|-------------------|
| `SessionRefreshManagerStressTest` | 5 | 50 concurrent refresh requests (mutex serialization), token expiry, timeout/deadlock, failure recovery |
| `JoinRoomEncryptionTest` | 3 | Room join encryption status preservation (Robolectric + WireMock + Ktor) |

#### Domain Layer -- Use Cases

| Test Class | Tests | What It Validates |
|------------|-------|-------------------|
| `LoginUseCaseTest` | 8 | Input validation (URL format, blank fields), successful login, auth failure |
| `LogoutUseCaseTest` | 8 | Logout flow: stopSync + logout + clearLocalAuth, clearAllData, failure resilience |

#### Data Layer -- Repository

| Test Class | Tests | What It Validates |
|------------|-------|-------------------|
| `VerificationRepositoryImplTest` | 19 | SAS verification state machine: happy path, all cancel transitions, invalid transitions, timeout, concurrent independence, Flow observation |

#### Platform Layer -- Logging

| Test Class | Tests | What It Validates |
|------------|-------|-------------------|
| `ErrorAnalyticsTest` | 23 | Error tracking, rate calculation, category/source/code aggregation, threshold alerts, StateFlow exposure, healthy state detection |

#### UI Layer -- Compose Components (Robolectric)

| Test Class | Tests | What It Validates |
|------------|-------|-------------------|
| `EmptyStatePanelTest` | 3 | Title, message, action rendering |
| `SettingsRowTest` | 4 | Title, subtitle, trailing content, disabled state |
| `ArmorTopBarTest` | 3 | Title, subtitle, background color |
| `SecurityBadgeTest` | 5 | SecurityLevel labels (None/Basic/Standard/Maximum), showLabel flag |

#### Integration Test Infrastructure

| File | Purpose |
|------|---------|
| `BridgeRpcStubs.kt` | WireMock stubs for 16 Bridge RPC methods: health, bridge.status, provisioning.claim, push.register_token, device.* (5 methods), invite.* (3 methods), auth check, fallback |
| `WireMockStubs.kt` | WireMock stubs for Matrix Client-Server API: login, sync, sendMessage, profile, logout, whoami, error responses |
| `TestConfig.kt` | Loads test-config.properties with override priority: system property > env variable > properties file |

---

### 3. androidApp/src/test/ (Unit + Robolectric Tests)

32 test files, ~258 test methods.

#### ViewModels (12 files, ~113 tests)

| Test Class | Tests | Frameworks | What It Validates |
|------------|-------|------------|-------------------|
| `DeviceListViewModelRealTest` | 8 | MockK, Coroutines | Device loading, removal, rollback, refresh, currentDevice flag, unverified/trusted device flows |
| `DeviceApprovalViewModelTest` | 10 | MockK, Turbine, Coroutines | Init loading, approval/rejection with rollback, biometric gating, refresh |
| `RoomInviteViewModelTest` | 7 | MockK, Turbine, Coroutines | Invite filtering, accept/reject with failure rollback, combined operations |
| `EmailApprovalViewModelTest` | 8 | MockK, Turbine, Coroutines | Optimistic approve/reject with rollback, processing tracking, request type filtering, room scoping |
| `EmailApprovalViewModelStandaloneTest` | 20 | MockK, Coroutines | State machine: InvalidId/NotFound/Expired/AlreadyHandled/Ready/Loading/Timeout/Error/Processing/Approved/Rejected |
| `ChatViewModelEncryptionTest` | 3 | MockK, Turbine, Coroutines | Encryption status mapping: VERIFIED/UNVERIFIED/UNENCRYPTED |
| `UserProfileViewModelTest` | 5 | MockK, Coroutines | Profile loading, unauthenticated state, API errors, shared rooms |
| `SyncStatusViewModelTest` | 15 | Turbine, Coroutines | State transitions, flow emissions, online/offline, retry with backoff |
| `AccountDeletionViewModelTest` | 8 | MockK, Coroutines | Delete success/failure, null handling, duplicate prevention |
| `ChangePhoneViewModelTest` | 14 | MockK, Coroutines | Phone validation, verification code flow, error mapping, state resets |
| `ChangePasswordViewModelTest` | 13 | MockK, Coroutines | Password validation, API error mapping, duplicate prevention |
| `EditBioViewModelTest` | 10 | MockK, Coroutines | Bio loading, 150-char limit, whitespace trimming, error handling |

#### Security (3 files, ~34 tests)

| Test Class | Tests | What It Validates |
|------------|-------|-------------------|
| `VaultRepositoryTest` | 10 | Mutex serialization, timeout handling, retry on lock, fail-closed on all operations |
| `VaultCryptoManagerTest` | 25 | Encryption round-trips, UTF-8 encoding, key derivation (32 bytes), salt generation, error handling |
| `VaultStressTest` | 5 | Concurrent access (10 coroutines, 200 ops), lock contention, timeout bounding, fail-closed verification, mutex enforcement |

#### Screens (8 files, ~38 tests)

| Test Class | Tests | What It Validates |
|------------|-------|-------------------|
| `CoreV2ScreenTest` | 6 | Home/Chat/Profile V2 screen rendering and content |
| `SecondaryV2ScreenTest` | 6 | Settings/Search/RoomDetails V2 screen rendering |
| `AuthV2ScreenTest` | 6 | Login/Registration/ForgotPassword V2 screen rendering |
| `EncryptionStatusMapperTest` | 5 | RoomEncryptionStatus -> UI EncryptionStatus mapping |
| `PermissionsScreenTest` | 5 | Permission count, required vs optional, granting, progress |
| `SecurityExplanationScreenTest` | 2 | 4 security steps in correct order |
| `ConnectServerScreenTest` | 4 | Idle/Connecting/Success/Error state transitions |
| `ChatScreenTest` | 4 | Sample messages, incoming/outgoing mix, list mutation |

#### Navigation (3 files, ~126 tests)

| Test Class | Tests | What It Validates |
|------------|-------|-------------------|
| `NavigationTransitionTest` | 64 | Route constants, route builders, onboarding/auth flows, 15 feature transitions, 12 user journeys |
| `NavHostRoutingTest` | 59 | Route builder output formats, special characters, empty args, template consistency, deep links, all 57 route constants non-empty and unique |
| `NavHostRuntimeTest` | 33 | 59 routes: count, uniqueness, parameterized format, builder round-trip consistency |

#### Components (1 file, 8 tests)

| Test Class | Tests | What It Validates |
|------------|-------|-------------------|
| `EmailApprovalCardTest` | 8 | Card rendering, body truncation, approve/reject buttons, callback firing, processing state |

#### Accessibility (1 file, 7 tests)

| Test Class | Tests | What It Validates |
|------------|-------|-------------------|
| `VerificationAccessibilityTest` | 7 | TrustBadge descriptions, emoji accessibility, TrustLevel coverage, COMPROMISED urgent wording |

#### Data (1 file, 6 tests)

| Test Class | Tests | What It Validates |
|------------|-------|-------------------|
| `EmailApprovalDataTest` | 6 | extractEmailData() context parsing, missing keys, null context, wrong request type |

#### HITL (1 file, 16 tests)

| Test Class | Tests | What It Validates |
|------------|-------|-------------------|
| `HITLStateMachineTest` | 16 | PII request expiry, sensitivity checks, field grouping, KeystoreStatus accessibility, SensitivityLevel ordering |

#### UI / Smoke (2 files, 2 tests)

| Test Class | Tests | What It Validates |
|------------|-------|-------------------|
| `ComposeTestSmokeTest` | 1 | Compose test infrastructure available |
| `ExampleUnitTest` | 1 | Default scaffold (2+2=4) |

---

### 4. androidApp/src/androidTest/ (Instrumented Tests)

12 test files, ~43 test methods. Run on physical device or emulator via `connectedDebugAndroidTest`.

#### On-Device E2E (3 files, 13 tests)

| Test Class | Tests | Rule Type | What It Validates |
|------------|-------|-----------|-------------------|
| `OnboardingComposeTest` | 2 | `createAndroidComposeRule<MainActivity>` | Welcome screen buttons, Login navigation |
| `LoginScreenComposeTest` | 2 | `createAndroidComposeRule<MainActivity>` | Login field display, credential input |
| `ChatE2EComposeTest` | 4 | `createAndroidComposeRule<MainActivity>` | Full E2E onboarding + chat send, send button disabled, multiple messages, back navigation |

#### Isolated Compose UI (2 files, 15 tests)

| Test Class | Tests | Rule Type | What It Validates |
|------------|-------|-----------|-------------------|
| `ChatScreenComposeTest` | 7 | `createComposeRule` | Message input, send button, enable/disable, callback, message list rendering |
| `ChatMessageDeliveryComposeTest` | 8 | `createComposeRule` | Sender name, content, timestamp, encryption indicator, empty state, long messages, multiple messages, incoming vs outgoing |

#### Validation / Logic (5 files, 19 tests)

| Test Class | Tests | What It Validates |
|------------|-------|-------------------|
| `ConfigExpiryTest` | 7 | Config TTL, expiry banner, sensitive operation blocking, DebugStateProvider consistency |
| `MatrixSyncTest` | 6 | Sync event parsing (messages, typing, presence), DebugStateProvider sync state |
| `ShadowMapGatekeeperTest` | 7 | PII screening: API key/Anthropic/Google AI/Bearer blocking, false positive prevention ("skateboarding"), interception logging |
| `AgentManagementTest` | 4 | Running agent filtering, agent stop, empty state, capability badges |
| `WorkflowTimelineTest` | 4 | Chronological step ordering, status icons, 25-step scroll performance, empty state |
| `MediaUploadTest` | 3 | Bitmap thumbnail generation, JPEG compression, preview sizing |

#### Scaffold

| Test Class | Tests | What It Validates |
|------------|-------|-------------------|
| `ExampleInstrumentedTest` | 1 | App context package name |

---

## Test Infrastructure Utilities

### TestTags (androidApp/androidTest)

Centralized Compose `testTag` constants used across instrumented tests and production UI code. 100+ constants organized by feature:

| Category | Tags | Examples |
|----------|------|----------|
| Welcome/Auth | 6 | `WELCOME_GET_STARTED`, `LOGIN_BUTTON`, `LOGIN_BIOMETRIC` |
| Registration | 5 | `REGISTRATION_USERNAME`, `REGISTRATION_BUTTON` |
| Onboarding | 14 | `CONNECT_SERVER`, `PERMISSIONS_GRANT_ALL`, `COMPLETION_START_CHATTING`, `TUTORIAL_NEXT` |
| Home | 5 | `HOME_CHATS_TAB`, `HOME_FAVORITES_TAB`, `HOME_ROOM_FAB` |
| Chat | 11 | `CHAT_MESSAGE_INPUT`, `CHAT_SEND`, `CHAT_ENCRYPTION_INDICATOR`, `CHAT_REPLY_PREVIEW` |
| Calls | 6 | `CALL_MUTE`, `CALL_END`, `INCOMING_CALL_ANSWER` |
| Search | 2 | `SEARCH_INPUT`, `SEARCH_CLEAR` |
| Profile | 5 | `PROFILE_CHANGE_PASSWORD`, `PROFILE_DELETE_ACCOUNT` |
| Settings | 11 | `SETTINGS_BIOMETRIC`, `SETTINGS_THEME`, `SETTINGS_DEVICES` |
| Vault | 17 | `VAULT_SECRETS_LIST`, `ADD_SECRET_SAVE`, `HOLD_TO_REVEAL` |
| PII/Approval | 8 | `EMAIL_APPROVAL_CARD`, `PII_APPROVE_BUTTON`, `BODY_PREVIEW` |

### ComposeTestExtensions (androidApp/androidTest)

Extension functions on `ComposeUiTest` for concise test assertions:

| Function | Description |
|----------|-------------|
| `waitForTag(tag, timeout=5000)` | Poll until node with tag exists |
| `assertTagIsDisplayed(tag)` | Assert node is displayed |
| `assertTagIsEnabled(tag)` | Assert node is enabled |
| `clickTag(tag)` | Click node by tag |
| `inputTextAtTag(tag, text)` | Input text at node by tag |

### TestConfig (shared/commonTest)

Loads test configuration from `test-config.properties` with override priority:

1. System property (`-Dtest.homeserver.url=...`)
2. Environment variable (`TEST_HOMESERVER_URL=...`)
3. Properties file value

Properties: homeserverUrl, homeserverDomain, bridgeUrl, bridgeHealthUrl, username, password, deviceId, displayName, roomId, roomName, connectionTimeoutMs, syncTimeoutMs.

### TestUtils (shared/commonTest)

Factory for test data: `createTestMessage()` with configurable id, roomId, senderId, content.

### BridgeRpcStubs (shared/androidUnitTest)

WireMock stub registry for Bridge RPC endpoints. 16 stub methods covering health, bridge.status, provisioning, push, device governance (5 methods), invite governance (3 methods), auth check, and error fallbacks.

### WireMockStubs (shared/androidUnitTest)

WireMock stub registry for Matrix Client-Server API. 9 stub methods: login, sync, sendMessage, profile, logout, whoami, and error responses (403, 401).

### Test Resources

| Resource | Location | Purpose |
|----------|----------|---------|
| `test-config.properties` | commonTest/resources | Test server URLs, credentials, timeouts |
| `fixtures/test-qr-config.txt` | commonTest/resources | QR deep link fixture (Base64-encoded JSON) |
| `wiremock/mappings/matrix-api-stubs.json` | commonTest/resources | Static Matrix stub mappings |
| `wiremock/mappings/bridge-rpc-stubs.json` | commonTest/resources | Static Bridge RPC stub mappings |

All resources are also available in `shared/src/androidUnitTest/resources/` for Robolectric classloader compatibility.

---

## Coverage by Feature Area

| Feature | Unit Tests | Integration | Instrumented | Total |
|---------|-----------|-------------|-------------|-------|
| **Bridge RPC** | RpcClient, Service, Lifecycle, WebSocket, Intervention, Provisioning | GovernanceStub, WireMockSetup | - | ~150 |
| **Matrix Sync** | SyncManager, EventRouter, TeamEventDispatcher | JoinRoomEncryption | MatrixSyncTest | ~42 |
| **Device Governance** | RepositoryImpl, ApprovalVM, ListVM | - | - | ~28 |
| **Encryption/Security** | MemoryWiper, VaultCrypto, VaultRepository | - | ShadowMapGatekeeper | ~40 |
| **HITL / Email Approval** | EmailApprovalVM, StandaloneVM, DataTest, HITLStateMachine | - | - | ~50 |
| **Navigation** | Transitions, Routing, Runtime | - | - | ~126 |
| **Chat UI** | ChatScreen, ChatVM Encryption | - | ChatScreen, ChatMessageDelivery, ChatE2E | ~25 |
| **Vault** | VaultCrypto, VaultRepository, VaultStressTest | - | - | ~40 |
| **Auth / Profile** | LoginUseCase, LogoutUseCase, AccountDeletion, ChangePhone, ChangePassword, EditBio | - | LoginScreen, Onboarding | ~55 |
| **Verification** | VerificationRepositoryImpl, TrustModel, VerificationAccessibility | - | - | ~48 |
| **Call Models** | CallModelTest | - | - | 31 |
| **Error Handling** | ErrorCode, ErrorAnalytics | - | ConfigExpiry | ~35 |
| **Agent Management** | AgentStatus, AgentIntervention, ControlPlaneStore | - | AgentManagementTest | ~30 |
| **Invite** | InviteRepositoryImpl, InviteService | GovernanceStub | - | ~28 |
| **UI Components** | SecurityBadge, ArmorTopBar, EmptyStatePanel, SettingsRow, WorkflowCard/Progress, AgentThinking | - | WorkflowTimeline, MediaUpload | ~30 |
| **Onboarding** | Permissions, SecurityExplanation, ConnectServer | - | OnboardingComposeTest | ~13 |
