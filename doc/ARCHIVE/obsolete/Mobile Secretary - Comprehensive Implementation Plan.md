# Mobile Secretary: Comprehensive Implementation Plan

> **Version:** 2.0.0
> **Last Updated:** 2026-02-26
> **Status:** Ready for Implementation
> **Timeline:** 12-15 Days (reduced from 17 due to existing infrastructure)

---

## Executive Summary

The Mobile Secretary feature enables ArmorClaw to act as an autonomous personal assistant that can:
- Browse websites and complete forms on behalf of the user
- Request user consent before accessing sensitive PII (BlindFill)
- Provide real-time status updates during tasks
- Operate with a zero-trust keystore that requires user authorization

**Key Finding:** The codebase already has 80% of the required infrastructure:
- ✅ Browser automation (Playwright)
- ✅ PII/BlindFill system with consent flows
- ✅ Encrypted keystore (SQLCipher)
- ✅ Matrix event infrastructure
- ✅ RPC framework with 50+ methods
- ✅ Agent framework basics

**What's Actually Missing:**
- Agent state machine (status transitions)
- Status event types for Matrix
- Mobile UI components (status banners, BlindFill cards)
- Unseal flow (user-held keys)
- Integration glue between components

---

## Part 1: Current Infrastructure Audit

### Already Implemented (DO NOT REBUILD)

| Component | Location | Status |
|-----------|----------|--------|
| **Browser Automation** | `container/openclaw-src/browser/` | ✅ Full Playwright integration |
| **Browser Skills** | `container/openclaw-src/skills/` | ✅ 50+ skills, route system |
| **BlindFill Engine** | `bridge/pkg/pii/` (Go-side; Rust vault blindfill removed v0.9.0) | ✅ Complete with sensitivity levels |
| **PII Request System** | `bridge/pkg/rpc/` | ✅ `pii.request_access`, `approve`, `reject` |
| **Encrypted Keystore** | `bridge/pkg/keystore/` | ✅ SQLCipher, hardware-derived keys |
| **Agent Framework** | `container/openclaw/agent.py` | ✅ Basic agent with Matrix |
| **RPC Server** | `bridge/pkg/rpc/` | ✅ 50+ methods implemented |
| **Matrix Events** | `shared/.../matrix/` | ✅ Event bus, sync, encryption |
| **HITL Approval** | `shared/.../hitl/` | ✅ Approval/rejection flows |
| **Biometric Auth** | `androidApp/.../biometric/` | ✅ AndroidX Biometric integration |

### Missing Components (NEED TO BUILD)

| Component | Gap | Effort | Phase |
|-----------|-----|--------|-------|
| Agent Status Enum | No state machine | 0.5 day | Phase 1 |
| Status Matrix Events | Missing event types | 0.5 day | Phase 1 |
| Mobile Status UI | No banners/indicators | 1 day | Phase 1 |
| Unseal Protocol | Server-side keys only | 2 days | Phase 2 |
| Unseal Mobile UI | No challenge flow | 1.5 days | Phase 2 |
| Session Manager | No auto-seal | 1 day | Phase 2 |
| Integration Tests | No E2E coverage | 1.5 days | Phase 3 |

---

## Part 2: Detailed Implementation Plan

### Phase 1: Agent Status & Mobile Visibility (Days 1-3)

**Objective:** Users can see what their secretary is doing in real-time.

#### Day 1: Agent State Machine

**Files to CREATE:**

```
bridge/pkg/agent/
├── state.go           # State enum and transitions
├── state_machine.go   # State machine logic
└── state_machine_test.go
```

**Implementation - `state.go`:**

```go
package agent

// AgentStatus represents the operational state of a secretary agent.
type AgentStatus string

const (
    StatusIdle              AgentStatus = "IDLE"
    StatusInitializing      AgentStatus = "INITIALIZING"
    StatusBrowsing          AgentStatus = "BROWSING"
    StatusFormFilling       AgentStatus = "FORM_FILLING"
    StatusAwaitingCaptcha   AgentStatus = "AWAITING_CAPTCHA"
    StatusAwaiting2FA       AgentStatus = "AWAITING_2FA"
    StatusAwaitingApproval  AgentStatus = "AWAITING_APPROVAL"
    StatusProcessingPayment AgentStatus = "PROCESSING_PAYMENT"
    StatusError             AgentStatus = "ERROR"
    StatusComplete          AgentStatus = "COMPLETE"
)

// ValidTransitions defines allowed state transitions.
var ValidTransitions = map[AgentStatus][]AgentStatus{
    StatusIdle: {StatusInitializing, StatusError},
    StatusInitializing: {StatusBrowsing, StatusError},
    StatusBrowsing: {StatusFormFilling, StatusAwaitingCaptcha, StatusAwaiting2FA, StatusError, StatusComplete},
    StatusFormFilling: {StatusAwaitingApproval, StatusProcessingPayment, StatusError, StatusComplete},
    StatusAwaitingCaptcha: {StatusBrowsing, StatusFormFilling, StatusError},
    StatusAwaiting2FA: {StatusBrowsing, StatusFormFilling, StatusError},
    StatusAwaitingApproval: {StatusFormFilling, StatusProcessingPayment, StatusError},
    StatusProcessingPayment: {StatusComplete, StatusError},
    StatusError: {StatusIdle},
    StatusComplete: {StatusIdle},
}

// StatusEvent represents a status change event for Matrix.
type StatusEvent struct {
    AgentID    string      `json:"agent_id"`
    Status     AgentStatus `json:"status"`
    Previous   AgentStatus `json:"previous,omitempty"`
    Metadata   StatusMetadata `json:"metadata,omitempty"`
    Timestamp  int64       `json:"timestamp"`
}

type StatusMetadata struct {
    URL      string `json:"url,omitempty"`
    Step     string `json:"step,omitempty"`
    Progress int    `json:"progress,omitempty"`
    Error    string `json:"error,omitempty"`
}
```

**Implementation - `state_machine.go`:**

```go
package agent

import (
    "errors"
    "sync"
    "time"
)

type StateMachine struct {
    mu          sync.RWMutex
    current     AgentStatus
    agentID     string
    eventChan   chan StatusEvent
    metadata    StatusMetadata
}

func NewStateMachine(agentID string) *StateMachine {
    return &StateMachine{
        current:   StatusIdle,
        agentID:   agentID,
        eventChan: make(chan StatusEvent, 100),
    }
}

// Transition attempts to change state. Returns error if invalid.
func (sm *StateMachine) Transition(newStatus AgentStatus, metadata ...StatusMetadata) error {
    sm.mu.Lock()
    defer sm.mu.Unlock()

    // Validate transition
    allowed, exists := ValidTransitions[sm.current]
    if !exists {
        return errors.New("invalid current state")
    }

    valid := false
    for _, s := range allowed {
        if s == newStatus {
            valid = true
            break
        }
    }
    if !valid {
        return fmt.Errorf("invalid transition: %s -> %s", sm.current, newStatus)
    }

    // Record transition
    prev := sm.current
    sm.current = newStatus
    if len(metadata) > 0 {
        sm.metadata = metadata[0]
    }

    // Emit event
    event := StatusEvent{
        AgentID:   sm.agentID,
        Status:    newStatus,
        Previous:  prev,
        Metadata:  sm.metadata,
        Timestamp: time.Now().UnixMilli(),
    }

    select {
    case sm.eventChan <- event:
    default:
        // Channel full, drop event (non-blocking)
    }

    return nil
}

// Events returns the event channel for Matrix integration.
func (sm *StateMachine) Events() <-chan StatusEvent {
    return sm.eventChan
}

// Current returns the current state.
func (sm *StateMachine) Current() AgentStatus {
    sm.mu.RLock()
    defer sm.mu.RUnlock()
    return sm.current
}
```

#### Day 2: Matrix Status Events & RPC

**Files to MODIFY:**

```
bridge/pkg/rpc/server.go       # Add status RPC methods
shared/.../matrix/events.go    # Add status event type
```

**New RPC Methods:**

```go
// In bridge/pkg/rpc/server.go

// agent_status returns current agent status
func (s *Server) AgentStatus(params json.RawMessage) interface{} {
    // Returns current status, metadata, last update time
}

// agent_status_history returns recent status changes
func (s *Server) AgentStatusHistory(params json.RawMessage) interface{} {
    // Returns last N status events for reconnection
}

// agent_wait waits for status change (long-poll)
func (s *Server) AgentWait(params json.RawMessage) interface{} {
    // Long-poll for status changes, useful for slow connections
}
```

**Matrix Event Type:**

```json
{
    "type": "com.armorclaw.agent.status",
    "content": {
        "agent_id": "secretary-001",
        "status": "FORM_FILLING",
        "previous": "BROWSING",
        "metadata": {
            "url": "https://united.com/booking",
            "step": "2/5",
            "progress": 40
        },
        "timestamp": 1739980800000
    }
}
```

#### Day 3: Mobile Status UI

**Files to CREATE:**

```
androidApp/.../ui/components/
├── AgentStatusBanner.kt      # Top-of-chat status bar
├── AgentStatusIndicator.kt   # Animated status dot
└── AgentProgressOverlay.kt   # Full-screen progress

shared/.../data/store/
└── AgentStatusStore.kt       # StateFlow for status
```

**Implementation - `AgentStatusBanner.kt`:**

```kotlin
@Composable
fun AgentStatusBanner(
    status: AgentStatusEvent,
    onDismiss: () -> Unit,
    modifier: Modifier = Modifier
) {
    val backgroundColor = when (status.status) {
        AgentStatus.IDLE -> MaterialTheme.colorScheme.surfaceVariant
        AgentStatus.BROWSING, AgentStatus.FORM_FILLING ->
            MaterialTheme.colorScheme.primaryContainer
        AgentStatus.AWAITING_APPROVAL ->
            MaterialTheme.colorScheme.tertiaryContainer
        AgentStatus.AWAITING_CAPTCHA, AgentStatus.AWAITING_2FA ->
            MaterialTheme.colorScheme.secondaryContainer
        AgentStatus.ERROR -> MaterialTheme.colorScheme.errorContainer
        AgentStatus.COMPLETE -> MaterialTheme.colorScheme.primary
        else -> MaterialTheme.colorScheme.surfaceVariant
    }

    Surface(
        modifier = modifier.fillMaxWidth(),
        color = backgroundColor,
        tonalElevation = 2.dp
    ) {
        Row(
            modifier = Modifier
                .padding(horizontal = 16.dp, vertical = 8.dp)
                .fillMaxWidth(),
            verticalAlignment = Alignment.CenterVertically
        ) {
            // Animated indicator
            AgentStatusIndicator(status.status)

            Spacer(modifier = Modifier.width(12.dp))

            // Status text
            Column(modifier = Modifier.weight(1f)) {
                Text(
                    text = status.status.toDisplayString(),
                    style = MaterialTheme.typography.bodyMedium,
                    fontWeight = FontWeight.Medium
                )
                if (status.metadata.url != null) {
                    Text(
                        text = status.metadata.url.let {
                            "Viewing: ${it.take(40)}${if (it.length > 40) "..." else ""}"
                        },
                        style = MaterialTheme.typography.bodySmall,
                        maxLines = 1,
                        overflow = TextOverflow.Ellipsis
                    )
                }
            }

            // Progress indicator
            if (status.metadata.progress > 0) {
                CircularProgressIndicator(
                    progress = status.metadata.progress / 100f,
                    modifier = Modifier.size(24.dp),
                    strokeWidth = 2.dp
                )
            }
        }
    }
}

@Composable
fun AgentStatusIndicator(status: AgentStatus) {
    val infiniteTransition = rememberInfiniteTransition()
    val alpha by infiniteTransition.animateFloat(
        initialValue = 0.3f,
        targetValue = 1f,
        animationSpec = infiniteRepeatable(
            animation = tween(1000),
            repeatMode = RepeatMode.Reverse
        )
    )

    Box(
        modifier = Modifier
            .size(12.dp)
            .alpha(if (status in listOf(AgentStatus.IDLE, AgentStatus.COMPLETE)) 1f else alpha)
            .background(
                color = when (status) {
                    AgentStatus.IDLE -> Color.Gray
                    AgentStatus.COMPLETE -> Color(0xFF4CAF50)
                    AgentStatus.ERROR -> Color(0xFFF44336)
                    AgentStatus.AWAITING_APPROVAL -> Color(0xFFFF9800)
                    else -> Color(0xFF2196F3)
                },
                shape = CircleShape
            )
    )
}
```

---

### Phase 2: Zero-Trust Keystore (Days 4-7)

**Objective:** VPS cannot access credentials without user authorization.

#### Day 4: Sealed Keystore Protocol

**Files to CREATE:**

```
bridge/pkg/keystore/
├── sealed_store.go       # Sealed/unsealed states
├── key_derivation.go     # Argon2id KEK derivation
├── challenge.go          # Challenge-response protocol
└── session.go            # Session timeout management
```

**Architecture:**

```
┌─────────────────────────────────────────────────────────────────────┐
│                    ZERO-TRUST KEYSTORE                              │
├─────────────────────────────────────────────────────────────────────┤
│                                                                      │
│   ┌─────────────┐    ┌─────────────┐    ┌─────────────────────┐    │
│   │   MOBILE    │    │   BRIDGE    │    │   KEYSTORE DB       │    │
│   │             │    │             │    │   (SQLCipher)       │    │
│   │  ┌───────┐  │    │  ┌───────┐  │    │                     │    │
│   │  │  KEK  │  │    │  │  DEK  │  │    │  ┌───────────────┐  │    │
│   │  │(user  │──┼────┼─►│(RAM   │──┼────┼─►│   Encrypted   │  │    │
│   │  │pass)  │  │    │  │only)  │  │    │  │   Credentials │  │    │
│   │  └───────┘  │    │  └───────┘  │    │  └───────────────┘  │    │
│   │             │    │             │    │                     │    │
│   └─────────────┘    └─────────────┘    └─────────────────────┘    │
│                                                                      │
│   FLOW:                                                              │
│   1. Mobile: deriveKEK(password) → wrappedKEK                        │
│   2. Bridge: challenge() → {nonce, serverPubKey}                    │
│   3. Mobile: sign(nonce) + encrypt(wrappedKEK) → response           │
│   4. Bridge: verify + decrypt → store DEK in RAM                    │
│   5. Bridge: auto-seal after 4 hours of inactivity                  │
│                                                                      │
└─────────────────────────────────────────────────────────────────────┘
```

**Implementation - `sealed_store.go`:**

```go
package keystore

import (
    "crypto/rand"
    "sync"
    "time"
)

type SealState int

const (
    Sealed SealState = iota
    Unsealed
)

type SealedKeystore struct {
    mu           sync.RWMutex
    state        SealState
    dek          []byte // Only in RAM when unsealed
    sessionStart time.Time
    sessionTTL   time.Duration
    challenge    []byte
    onSeal       []func()
}

func NewSealedKeystore(ttl time.Duration) *SealedKeystore {
    return &SealedKeystore{
        state:      Sealed,
        sessionTTL: ttl,
    }
}

// Challenge generates a new challenge for the unseal protocol.
func (s *SealedKeystore) Challenge() ([]byte, error) {
    s.mu.Lock()
    defer s.mu.Unlock()

    challenge := make([]byte, 32)
    if _, err := rand.Read(challenge); err != nil {
        return nil, err
    }
    s.challenge = challenge
    return challenge, nil
}

// Unseal decrypts the DEK and stores it in memory.
func (s *SealedKeystore) Unseal(wrappedKEK []byte, signature []byte) error {
    s.mu.Lock()
    defer s.mu.Unlock()

    // Verify signature against challenge
    // Decrypt wrapped KEK to get DEK
    dek, err := s.unwrapKEK(wrappedKEK, signature)
    if err != nil {
        return err
    }

    s.dek = dek
    s.state = Unsealed
    s.sessionStart = time.Now()
    return nil
}

// Seal wipes the DEK from memory.
func (s *SealedKeystore) Seal(reason string) {
    s.mu.Lock()
    defer s.mu.Unlock()

    if s.state == Sealed {
        return
    }

    // Wipe DEK from memory
    for i := range s.dek {
        s.dek[i] = 0
    }
    s.dek = nil
    s.state = Sealed

    // Notify listeners
    for _, fn := range s.onSeal {
        go fn()
    }
}

// Get retrieves a value (fails if sealed).
func (s *SealedKeystore) Get(key string) (string, error) {
    s.mu.RLock()
    defer s.mu.RUnlock()

    if s.state == Sealed {
        return "", ErrKeystoreSealed
    }

    // Use DEK to decrypt value from SQLCipher DB
    return s.getValue(key, s.dek)
}

// IsSealed returns current seal state.
func (s *SealedKeystore) IsSealed() bool {
    s.mu.RLock()
    defer s.mu.RUnlock()
    return s.state == Sealed
}

// SessionRemaining returns time until auto-seal.
func (s *SealedKeystore) SessionRemaining() time.Duration {
    s.mu.RLock()
    defer s.mu.RUnlock()

    if s.state == Sealed {
        return 0
    }

    elapsed := time.Since(s.sessionStart)
    remaining := s.sessionTTL - elapsed
    if remaining < 0 {
        return 0
    }
    return remaining
}
```

#### Day 5: RPC Methods for Unseal

**Files to MODIFY:**

```
bridge/pkg/rpc/server.go   # Add unseal RPC methods
```

**New RPC Methods:**

```go
// keystore.challenge - Get challenge for unseal
// Request: {}
// Response: { "nonce": "base64", "serverPublicKey": "base64" }

// keystore.unseal - Unseal with wrapped KEK
// Request: { "wrappedKEK": "base64", "signature": "base64" }
// Response: { "success": true, "expiresAt": timestamp }

// keystore.sealed - Check seal status
// Request: {}
// Response: { "sealed": true/false, "remainingSeconds": 3600 }

// keystore.extend_session - Extend unsealed session
// Request: {}
// Response: { "success": true, "newExpiry": timestamp }

// keystore.seal - Manually seal the keystore
// Request: {}
// Response: { "success": true }
```

#### Day 6-7: Mobile Unseal UI

**Files to CREATE:**

```
androidApp/.../screens/keystore/
├── UnsealScreen.kt           # Main unseal flow
├── UnsealViewModel.kt        # Business logic
└── SealedIndicator.kt        # Status indicator

androidApp/.../platform/crypto/
└── KeyDerivationImpl.kt      # Argon2id implementation
```

**Implementation - `UnsealScreen.kt`:**

```kotlin
@Composable
fun UnsealScreen(
    onUnsealed: () -> Unit,
    viewModel: UnsealViewModel = hiltViewModel()
) {
    val state by viewModel.state.collectAsState()
    var password by remember { mutableStateOf("") }
    var useBiometric by remember { mutableStateOf(false) }

    Scaffold { padding ->
        Column(
            modifier = Modifier
                .fillMaxSize()
                .padding(padding)
                .padding(24.dp),
            horizontalAlignment = Alignment.CenterHorizontally
        ) {
            // Icon
            Icon(
                imageVector = Icons.Default.Lock,
                contentDescription = null,
                modifier = Modifier.size(80.dp),
                tint = MaterialTheme.colorScheme.primary
            )

            Spacer(modifier = Modifier.height(24.dp))

            // Title
            Text(
                text = "VPS Keystore Sealed",
                style = MaterialTheme.typography.headlineMedium,
                textAlign = TextAlign.Center
            )

            Spacer(modifier = Modifier.height(12.dp))

            // Description
            Text(
                text = "The VPS cannot access your credentials until you authorize.",
                style = MaterialTheme.typography.bodyLarge,
                textAlign = TextAlign.Center,
                color = MaterialTheme.colorScheme.onSurfaceVariant
            )

            Spacer(modifier = Modifier.height(32.dp))

            // Password field
            OutlinedTextField(
                value = password,
                onValueChange = { password = it },
                label = { Text("Master Password") },
                visualTransformation = PasswordVisualTransformation(),
                keyboardOptions = KeyboardOptions(
                    keyboardType = KeyboardType.Password,
                    imeAction = ImakeAction.Go
                ),
                keyboardActions = KeyboardActions(
                    onGo = { viewModel.unseal(password) }
                ),
                modifier = Modifier.fillMaxWidth()
            )

            Spacer(modifier = Modifier.height(16.dp))

            // Biometric option
            if (viewModel.hasBiometricCapability()) {
                Row(
                    modifier = Modifier.fillMaxWidth(),
                    verticalAlignment = Alignment.CenterVertically
                ) {
                    Checkbox(
                        checked = useBiometric,
                        onCheckedChange = { useBiometric = it }
                    )
                    Text("Use biometric instead")
                }
            }

            Spacer(modifier = Modifier.height(24.dp))

            // Buttons
            Button(
                onClick = {
                    if (useBiometric) viewModel.unsealWithBiometric()
                    else viewModel.unseal(password)
                },
                modifier = Modifier.fillMaxWidth(),
                enabled = password.isNotEmpty() || useBiometric
            ) {
                if (state.isLoading) {
                    CircularProgressIndicator(
                        modifier = Modifier.size(20.dp),
                        color = MaterialTheme.colorScheme.onPrimary
                    )
                } else {
                    Text("Unseal")
                }
            }

            // Session info
            Spacer(modifier = Modifier.height(16.dp))
            Text(
                text = "⏱️ Session will remain unsealed for 4 hours",
                style = MaterialTheme.typography.bodySmall,
                color = MaterialTheme.colorScheme.onSurfaceVariant
            )

            // Error message
            if (state.error != null) {
                Spacer(modifier = Modifier.height(16.dp))
                Text(
                    text = state.error,
                    style = MaterialTheme.typography.bodyMedium,
                    color = MaterialTheme.colorScheme.error
                )
            }
        }
    }
}
```

---

### Phase 3: Integration & Testing (Days 8-10)

#### Day 8: Browser Skill Integration

**Connect existing browser automation to status events:**

```go
// In bridge/pkg/skills/browser.go

func (s *BrowserSkill) Navigate(url string) (*NavigateResult, error) {
    // Emit status: BROWSING
    s.stateMachine.Transition(agent.StatusBrowsing, agent.StatusMetadata{
        URL: url,
    })

    defer func() {
        // Emit status: IDLE or ERROR
        if err != nil {
            s.stateMachine.Transition(agent.StatusError, agent.StatusMetadata{
                Error: err.Error(),
            })
        } else {
            s.stateMachine.Transition(agent.StatusIdle)
        }
    }()

    // Existing navigation logic
    return s.doNavigate(url)
}
```

#### Day 9: BlindFill Integration

**Connect existing BlindFill to status events:**

```go
// In bridge/pkg/pii/engine.go (Rust vault blindfill removed v0.9.0)

func (e *BlindFillEngine) FillForm(fields []PIIField) error {
    // Emit status: AWAITING_APPROVAL
    e.stateMachine.Transition(agent.StatusAwaitingApproval, agent.StatusMetadata{
        Step:     "Waiting for approval",
        Progress: 50,
    })

    // Request PII access (existing code)
    requestID, err := e.requestPIIAccess(fields)
    if err != nil {
        return err
    }

    // Wait for approval
    approved, err := e.waitForApproval(requestID)
    if !approved {
        return ErrPIIDenied
    }

    // Emit status: FORM_FILLING
    e.stateMachine.Transition(agent.StatusFormFilling, agent.StatusMetadata{
        Step:     "Filling form",
        Progress: 70,
    })

    // Fill form with approved values
    return e.fillWithApprovedValues(requestID)
}
```

#### Day 10: End-to-End Testing

**Test Scenarios:**

| Scenario | Steps | Expected Result |
|----------|-------|-----------------|
| **Basic Browse** | 1. Unseal keystore 2. Navigate to URL | Status shows BROWSING → IDLE |
| **Form Fill with PII** | 1. Start form fill 2. Receive approval request 3. Approve | Status shows AWAITING_APPROVAL → FORM_FILLING → COMPLETE |
| **PII Denial** | 1. Start form fill 2. Deny request | Status shows AWAITING_APPROVAL → ERROR |
| **Auto-Seal** | 1. Unseal 2. Wait 4 hours 3. Try to access PII | Error: Keystore sealed |
| **Session Extend** | 1. Unseal 2. After 3 hours, extend 3. Check remaining | Remaining time reset to 4 hours |
| **Captcha Required** | 1. Browse to site with Captcha | Status shows AWAITING_CAPTCHA |

---

## Part 3: File Manifest

### Files to CREATE (18 files)

```
# Phase 1: Status System
bridge/pkg/agent/state.go
bridge/pkg/agent/state_machine.go
bridge/pkg/agent/state_machine_test.go
androidApp/.../ui/components/AgentStatusBanner.kt
androidApp/.../ui/components/AgentStatusIndicator.kt
androidApp/.../data/store/AgentStatusStore.kt

# Phase 2: Keystore
bridge/pkg/keystore/sealed_store.go
bridge/pkg/keystore/key_derivation.go
bridge/pkg/keystore/challenge.go
bridge/pkg/keystore/session.go
bridge/pkg/keystore/sealed_store_test.go
androidApp/.../screens/keystore/UnsealScreen.kt
androidApp/.../viewmodels/UnsealViewModel.kt
androidApp/.../ui/components/SealedIndicator.kt
androidApp/.../platform/crypto/KeyDerivationImpl.kt

# Phase 3: Integration
bridge/pkg/integration/secretary_integration.go
bridge/pkg/integration/secretary_integration_test.go
```

### Files to MODIFY (6 files)

```
bridge/pkg/rpc/server.go              # Add status + unseal RPC methods
shared/.../matrix/events.go           # Add agent status event type
container/openclaw/agent.py           # Integrate state machine
bridge/pkg/skills/browser.go          # Add status emissions
bridge/pkg/pii/engine.go              # Add status emissions (blindfill path removed v0.9.0)
androidApp/.../screens/chat/ChatScreen.kt  # Add status banner
```

---

## Part 4: Risk Assessment

| Risk | Probability | Impact | Mitigation |
|------|-------------|--------|------------|
| Browser detection (bot blocking) | Medium | High | Playwright stealth plugin, human-like delays |
| CAPTCHA solving | High | Medium | Fallback to user notification |
| 2FA handling | Medium | Medium | Status event for user to enter code |
| Session timeout during long task | Medium | High | Auto-extend on activity, warn at 30 min |
| Mobile network latency | Low | Medium | Long-poll RPC, optimistic UI updates |
| Keystore corruption | Low | Critical | Backups, recovery phrases |

---

## Part 5: Success Criteria

- [ ] Agent status visible in mobile app within 500ms of state change
- [ ] BlindFill requests appear on mobile within 1 second
- [ ] Unseal flow completes in under 3 seconds
- [ ] Auto-seal triggers exactly at 4 hours of inactivity
- [ ] All E2E tests pass
- [ ] No credentials accessible when sealed
- [ ] Session extension works during active tasks

---

## Part 6: Timeline Summary

| Phase | Days | Key Deliverable |
|-------|------|-----------------|
| **Phase 1** | 1-3 | Real-time status visible on mobile |
| **Phase 2** | 4-7 | Zero-trust keystore with unseal flow |
| **Phase 3** | 8-10 | Full integration + E2E tests |
| **Buffer** | 11-12 | Bug fixes, polish |
| **Total** | **12 days** | Mobile Secretary Alpha |

---

**Document Version:** 2.0.0
**Last Updated:** 2026-02-26
**Status:** Ready for Implementation
