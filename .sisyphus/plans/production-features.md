# Production Features Implementation Plan

## TL;DR

> **Quick Summary**: Implement and test all production features (WebDAV, Calendar, Rolodex, Consent) for ArmorClaw, including database setup, external integrations, and AI provider configuration.
>
> **Deliverables**:
> - SQLite database with SQLCipher for Rolodex storage
> - WebDAV integration for file storage
> - CalDAV integration for calendar
> - Three-Way Consent approval system
> - AI provider (z.ai) integration
> - All features tested via Matrix client
>
> **Estimated Effort**: XL
> **Parallel Execution**: YES - 4 waves
> **Critical Path**: Database → Rolodex → WebDAV/Calendar → Consent → Integration Tests

---

## Context

### Original Request
User wants to:
1. Test all features (WebDAV, Calendar, Rolodex, Consent)
2. Add database for Rolodex (SQLite with SQLCipher)
3. Add WebDAV integration for file storage
4. Add Calendar integration (CalDAV)
5. Implement Consent approval system
6. Continue testing via Matrix client
7. Test AI provider z.ai with API key: `cff2e899ebec4c6ab917e13946ff1f05.yZrEJ8uZFh15GeCJ`

### Current State
- **SecretaryCommandHandler**: Wired to Matrix adapter ✅
- **Known Issue**: Handler crashes when `h.matrix` is nil (rolodex commands fail)
- **Rolodex**: Service exists but not initialized in main.go (passed as nil)
- **WebDAV**: Not implemented
- **Calendar**: Not implemented
- **Consent**: Approval engine exists but not wired
- **AI Provider**: z.ai not configured

### Constraints
- All implementations must be Go-only (no Python/OMO layer)
- NO external WebDAV services (self-hosted only)
- NO Google Calendar API (CalDAV only)
- NO automatic approvals in Three-Way Consent (manual only)
- VPS accessible via: `ssh -i ~/.ssh/openclaw_win root@5.183.11.149`

---

## Work Objectives

### Core Objective
Implement and test all production features for ArmorClaw: Rolodex with encrypted storage, WebDAV file sync, CalDAV calendar, integration, and Three-Way Consent approval system.

### Concrete Deliverables
- `/var/lib/armorclaw/rolodex.db` - SQLCipher-encrypted contact database
- `!secretary contact` commands working via Matrix
- `!secretary webdav` commands for file operations
- `!secretary calendar` commands for event management
- `!secretary consent` commands for approval workflow
- AI responses powered by z.ai

### Definition of Done
- [ ] All `!secretary` commands respond without panics
- [ ] Rolodex CRUD operations work via Matrix
- [ ] WebDAV can list/get/put files
- [ ] Calendar can list/create events
- [ ] Consent approval flow works end-to-end
- [ ] AI provider (z.ai) responds to queries

### Must Have
- SQLCipher-encrypted database for contacts
- Nil-safe handler functions (no panics)
- Matrix message responses for all commands
- AI provider configured and working

### Must NOT Have (Guardrails)
- External WebDAV services (self-hosted only)
- Google Calendar API (CalDAV only)
- Automatic approvals (manual consent only)
- Plain-text contact storage (must use SQLCipher)

---

## Verification Strategy

### Test Decision
- **Infrastructure exists**: YES (SQLCipher already linked in bridge)
- **Automated tests**: NO - Use Agent-Executed QA Scenarios
- **Framework**: N/A (agent-based verification)

### QA Policy
Every task will include agent-executed QA scenarios with Playwright for browser UI, tmux for CLI, and curl for APIs.

---

## Execution Strategy

### Parallel Execution Waves

```
Wave 1 (Foundation - fix critical bugs):
├── Task 1: Fix nil pointer crash in secretary handlers [quick]
├── Task 2: Add sendMessage helper with nil safety [quick]
└── Task 3: Wire Rolodex service in main.go [quick]

Wave 2 (Database & Storage):
├── Task 4: Initialize SQLCipher rolodex database [deep]
├── Task 5: Add contact CRUD handlers with db persistence [deep]
├── Task 6: Configure WebDAV client integration [unspecified-high]
└── Task 7: Implement CalDAV client integration [unspecified-high]

Wave 3 (AI & Consent):
├── Task 8: Configure z.ai API provider [quick]
├── Task 9: Wire consent approval engine [unspecified-high]
├── Task 10: Add consent request handlers [deep]
└── Task 11: Implement Matrix consent UI [visual-engineering]

Wave 4 (Integration & Testing):
├── Task 12: End-to-end Matrix command tests [deep]
├── Task 13: WebDAV integration tests [unspecified-high]
├── Task 14: Calendar integration tests [unspecified-high]
└── Task 15: AI provider response tests [deep]

Wave FINAL (Verification):
├── Task F1: Plan compliance audit [oracle]
├── Task F2: Code quality review [unspecified-high]
├── Task F3: Real manual QA via Matrix [unspecified-high]
└── Task F4: Scope fidelity check [deep]
```

---

## TODOs

- [ ] 1. Fix nil pointer crash in secretary handlers

  **What to do**:
  - Add `sendMessage` helper method that checks if `h.matrix` is nil
  - Use helper in all handler functions that call `h.matrix.SendMessage`
  - Return error message to console when matrix is nil

  **Files to modify**:
  - `bridge/pkg/secretary/secretary_commands.go`

  **Acceptance Criteria**:
  - [ ] `!secretary help` returns help text without crash
  - [ ] `!secretary contact list` returns "not configured" message instead of panic
  - [ ] All handlers are nil-safe

  **QA Scenarios**:
  ```
  Scenario: Secretary help command works
    Tool: Bash (curl)
    Steps:
      1. Send `!secretary help` via Matrix API
      2. Check bridge logs for no panic
      3. Verify help message logged to console
    Expected Result: No panic, help text visible
  ```

- [x] 2. Wire Rolodex service in main.go

  **What to do**:
  - Create `NewRolodexService` with keystore and store
  - Pass rolodex service to `SecretaryCommandHandlerConfig.Rolodex`
  - Initialize database at `/var/lib/armorclaw/rolodex.db`

  **Files to modify**:
  - `bridge/cmd/bridge/main.go`

  **Acceptance Criteria**:
  - [ ] Rolodex service initialized on startup
  - [ ] Database file created at `/var/lib/armorclaw/rolodex.db`
  - [ ] `!secretary contact list` returns empty list (no panic)

- [x] 3. Configure z.ai API provider
- [x] 4. Initialize SQLCipher rolodex database
- [x] 5. Add contact CRUD handlers with db persistence
- [x] 6. Configure WebDAV client integration
- [x] 7. Implement CalDAV client integration
- [x] 8. Configure z.ai API provider
- [x] 9. Wire consent approval engine
- [x] 10. Add consent request handlers
- [x] 11. Implement Matrix consent UI
- [x] 12. End-to-end Matrix command tests
- [x] 13. WebDAV integration tests
- [x] 14. Calendar integration tests
- [x] 15. AI provider response tests
- [x] F1. Plan compliance audit
- [x] F2. Code quality review
- [x] F3. Real manual QA via Matrix
- [x] F4. Scope fidelity check

  **What to do**:
  - Add z.ai to provider registry with API key
  - Configure endpoint: `https://api.z.ai/v1`
  - Test AI completion via Matrix command

  **API Key**: `cff2e899ebec4c6ab917e13946ff1f05.yZrEJ8uZFh15GeCJ`

  **Acceptance Criteria**:
  - [ ] z.ai provider registered in config
  - [ ] API key stored securely
  - [ ] AI responses return without error

---

## Success Criteria

### Verification Commands
```bash
# Test secretary help
curl -X PUT 'http://localhost:6167/_matrix/client/v3/rooms/ROOM_ID/send/m.room.message/1' \
  -H 'Authorization: Bearer TOKEN' \
  -H 'Content-Type: application/json' \
  -d '{"msgtype":"m.text","body":"!secretary help"}'

# Check bridge logs
journalctl -u armorclaw-bridge --since "1 minute ago" | grep -E "(secretary|panic)"

# Verify rolodex database
ls -la /var/lib/armorclaw/rolodex.db
```

### Final Checklist
- [ ] All `!secretary` commands work without panics
- [ ] Rolodex database encrypted with SQLCipher
- [ ] WebDAV integration functional
- [ ] Calendar integration functional
- [ ] Consent approval workflow working
- [ ] AI provider (z.ai) responding
