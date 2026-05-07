# ArmorClaw Implementation Plans

> **Consolidated documentation file for ArmorClaw implementation plans and their execution status.**
> **Last Updated:** 2026-05-04

---

## Source Files Consolidated Here

| Source File | Original Coverage | Lines |
|-------------|-------------------|-------|
| `doc/ACTIVE/plan/ArmorClaw Huh_ Wizard Implementation & Error Handling - UPDATED.md` | Setup wizard phases 1-5, error handling gaps, remaining Go work | ~269 |
| `doc/ACTIVE/plan/Mobile Secretary - Comprehensive Implementation Plan.md` | Mobile secretary feature: agent status, zero-trust keystore, integration testing | ~892 |

---

## Plan A: Huh? Wizard Implementation & Error Handling

> **Status:** COMPLETE — All Phases Implemented
> **Last Updated:** 2026-05-04
> **Version:** 0.3.6

### Executive Summary

Setup wizard with Charm Huh TUI forms. Most phases complete; Go-native error handling and automated testing remain.

### Implementation Status

#### Phase 1: Huh? Wizard (Go) — ✅ COMPLETE

**Location:** `bridge/internal/wizard/`

| Component | Status | File |
|-----------|--------|------|
| Profile Selection | ✅ Done | `profile.go` |
| Quick Start Forms | ✅ Done | `quick.go` |
| Wizard Runner | ✅ Done | `wizard.go` |
| Theme/Styling | ✅ Done | `theme.go` |
| Input Validation | ✅ Done | `validation.go` |
| Terminal Detection | ✅ Done | `wizard.go:checkTerminal()` |
| Non-Interactive Mode | ✅ Done | `wizard.go:tryNonInteractive()` |
| Server Name Auto-Detection | ✅ Done | `wizard.go:detectServerName()` |

**Dependencies:** `charmbracelet/huh v0.8.0`, `charmbracelet/lipgloss v1.1.0`

#### Phase 2: Error Handling — ✅ COMPLETE

| Component | Status | Location |
|-----------|--------|----------|
| Error Codes (INS-XXX) | ✅ Done | `docs/guides/error-catalog.md` |
| Bash Error Handling | ✅ Done | `container-setup.sh` |
| Crash Handler | ✅ Done | `quickstart.sh` in Dockerfile |
| Preflight Checks | ✅ Done | `container-setup.sh:preflight_checks()` |
| Go Error Types | ✅ Done | `bridge/pkg/setup/errors.go` (254 lines) |
| Go Error Messages | ✅ Done | Actionable messages in Go code |

#### Phase 3: Dockerfile Integration — ✅ COMPLETE

| Component | Status |
|-----------|--------|
| Wizard Build Stage | ✅ Done (multi-stage in Dockerfile.quickstart) |
| Binary Location | ✅ `/opt/armorclaw/armorclaw-bridge` |
| Entrypoint Update | ✅ `quickstart.sh` calls Go wizard |
| Fallback Chain | ✅ Go wizard → Bash wizard |

#### Phase 4: Wizard Implementation Details — ✅ COMPLETE

| Feature | Status |
|---------|--------|
| Welcome Banner | ✅ Done |
| Docker Socket Check | ✅ Done |
| Profile Selection | ✅ Done |
| API Key Input (masked) | ✅ Done |
| Password Input | ✅ Done with auto-generate |
| Server Name Detection | ✅ Done |
| Progress Indication | ✅ Done |
| Success Screen | ✅ Done |

#### Phase 5: Integration Testing — ✅ COMPLETE

| Test Scenario | Status |
|---------------|--------|
| First-time setup (empty volumes) | ✅ |
| Setup with existing config | ✅ |
| Docker permission error handling | ✅ |
| Invalid input validation | ✅ |
| API key injection after wizard | ✅ |
| Health check timeout | ✅ |

### Completed Work (All Done)

| Priority | File | Purpose | Status |
|----------|------|---------|--------|
| P1 | `bridge/pkg/setup/errors.go` | Typed errors with actionable messages | ✅ Done (254 lines) |
| P1 | `bridge/pkg/setup/docker.go` | Docker validation in Go | ✅ Done (215 lines) |
| P1 | `bridge/internal/wizard/wizard_test.go` | Integration tests | ✅ Done |
| P2 | `bridge/pkg/setup/ssl.go` | SSL generation in Go | ✅ Done (296 lines) |
| P2 | `bridge/pkg/setup/config.go` | Config file generation in Go | ✅ Done (316 lines) |

---

## Plan B: Mobile Secretary — Comprehensive Implementation

> **Status:** Partially Complete — Agent State & Integration Tests Done, Android UI Pending
> **Version:** 2.0.0
> **Last Updated:** 2026-05-04

### Executive Summary

Enables ArmorClaw as autonomous personal assistant with real-time status, BlindFill consent, and zero-trust keystore. Codebase already has ~80% of required infrastructure.

### Existing Infrastructure (DO NOT REBUILD)

| Component | Location | Status |
|-----------|----------|--------|
| Browser Automation | `container/openclaw-src/browser/` | ✅ Full Playwright |
| Browser Skills | `container/openclaw-src/skills/` | ✅ 50+ skills |
| BlindFill Engine | `bridge/pkg/pii/` | ✅ Complete |
| PII Request System | `bridge/pkg/rpc/` | ✅ `pii.request_access`, `approve`, `reject` |
| Encrypted Keystore | `bridge/pkg/keystore/` | ✅ SQLCipher |
| Agent Framework | `container/openclaw/agent.py` | ✅ Basic agent |
| RPC Server | `bridge/pkg/rpc/` | ✅ 50+ methods |
| Matrix Events | `shared/.../matrix/` | ✅ Event bus, sync, encryption |
| HITL Approval | `shared/.../hitl/` | ✅ Approval/rejection flows |
| Biometric Auth | `applications/ArmorChat/.../ui/security/BiometricEnableScreen.kt` | ✅ AndroidX Biometric |

### Missing Components (REMAINING)

| Component | Gap | Effort | Phase | Status |
|-----------|-----|--------|-------|--------|
| Agent Status Enum | No state machine | 0.5 day | Phase 1 | ✅ DONE (23 files in `bridge/pkg/agent/`) |
| Status Matrix Events | Missing event types | 0.5 day | Phase 1 | ✅ DONE |
| Mobile Status UI | No banners/indicators | 1 day | Phase 1 | ✅ DONE (HomeScreen DashboardViewModel) |
| Unseal Protocol | Server-side keys only | 2 days | Phase 2 | Pending |
| Unseal Mobile UI | No challenge flow | 1.5 days | Phase 2 | Pending |
| Session Manager | No auto-seal | 1 day | Phase 2 | Pending |
| Integration Tests | No E2E coverage | 1.5 days | Phase 3 | ✅ DONE (11 test files in `bridge/`) |

### Phase Breakdown

#### Phase 1: Agent Status & Mobile Visibility (Days 1-3)

**Objective:** Users see what their secretary is doing in real-time.

Files to CREATE:
- `bridge/pkg/agent/state.go` — State enum and transitions
- `bridge/pkg/agent/state_machine.go` — State machine logic
- `bridge/pkg/agent/state_machine_test.go`
- `applications/ArmorChat/.../ui/components/AgentStatusBanner.kt`
- `applications/ArmorChat/.../ui/components/AgentStatusIndicator.kt`
- `applications/ArmorChat/.../data/store/AgentStatusStore.kt`

Agent states: `IDLE → INITIALIZING → BROWSING → FORM_FILLING → AWAITING_APPROVAL → PROCESSING_PAYMENT → COMPLETE/ERROR`

Files to MODIFY:
- `bridge/pkg/rpc/server.go` — Add `agent_status`, `agent_status_history`, `agent_wait` RPC methods
- `shared/.../matrix/events.go` — Add `com.armorclaw.agent.status` event type

#### Phase 2: Zero-Trust Keystore (Days 4-7)

**Objective:** VPS cannot access credentials without user authorization.

Architecture: Mobile holds KEK (user password) → Bridge holds DEK (RAM only) → SQLCipher stores encrypted credentials. Auto-seal after 4 hours inactivity.

Files to CREATE:
- `bridge/pkg/keystore/sealed_store.go` — Sealed/unsealed states
- `bridge/pkg/keystore/key_derivation.go` — Argon2id KEK derivation
- `bridge/pkg/keystore/challenge.go` — Challenge-response protocol
- `bridge/pkg/keystore/session.go` — Session timeout management
- `applications/ArmorChat/.../screens/keystore/UnsealScreen.kt`
- `applications/ArmorChat/.../platform/crypto/KeyDerivationImpl.kt`

New RPC methods: `keystore.challenge`, `keystore.unseal`, `keystore.sealed`, `keystore.extend_session`, `keystore.seal`

#### Phase 3: Integration & Testing (Days 8-10)

**Objective:** Wire status events into browser automation and BlindFill, validate end-to-end.

Files to MODIFY:
- `bridge/pkg/skills/browser.go` — Emit status on navigate/fill
- `bridge/pkg/pii/engine.go` — Emit status on approval/fill
- `container/openclaw/agent.py` — Integrate state machine

E2E test scenarios:

| Scenario | Expected Result |
|----------|-----------------|
| Basic Browse | Status: BROWSING → IDLE |
| Form Fill with PII (approved) | Status: AWAITING_APPROVAL → FORM_FILLING → COMPLETE |
| PII Denial | Status: AWAITING_APPROVAL → ERROR |
| Auto-Seal (4h idle) | Keystore sealed error |
| Session Extend | Remaining time reset to 4h |

### File Manifest

**18 files to CREATE, 6 files to MODIFY** (see source files for full details).

### Risk Assessment

| Risk | Probability | Impact | Mitigation |
|------|-------------|--------|------------|
| Browser detection (bot blocking) | Medium | High | Playwright stealth, human-like delays |
| CAPTCHA solving | High | Medium | Fallback to user notification |
| Session timeout during long task | Medium | High | Auto-extend on activity, warn at 30 min |
| Keystore corruption | Low | Critical | Backups, recovery phrases |

### Success Criteria

- Agent status visible in mobile within 500ms of state change
- BlindFill requests appear within 1 second
- Unseal flow completes in under 3 seconds
- Auto-seal triggers at exactly 4 hours
- No credentials accessible when sealed

---

*Original files archived at `doc/ARCHIVE/obsolete/`.*
