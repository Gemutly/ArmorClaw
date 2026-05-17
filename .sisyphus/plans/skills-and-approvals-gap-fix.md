# Skills and approvals Gap Fix Plan

## TL;DR

> **Quick Summary**: Fix the 20% Skills System gap by implementing WebDAV, Calendar, Rolodex, and Three-Way Consent integration. Add missing Approvals for planned features (Rolodex Database, Three-Way Consent).
>
> **Deliverables**:
> - WebDAV skill (bridge/internal/skills/webdav.go)
> - Calendar skill with CalDAV (bridge/internal/skills/calendar.go)
> - Rolodex database schema and tables (bridge/pkg/secretary/schema.sql)
> - Three-Way Consent Matrix integration (bridge/pkg/pii/three_way_consent.go)
>
> **Estimated Effort**: Medium (3-4 days)
> **Parallel Execution**: YES - 4 waves
> **Critical Path**: WebDAV → Calendar → Rolodex → Three-Way Consent

---

## Context

### Original Request
Implement missing skills and approvals components:
1. Skills System (20% ⚠️):
   - WebDAV: ❌ Not implemented
   - Calendar: ❌ Not implemented
   
2. Planned Features:
   - Rolodex Database: ❌ Designed, not built
   - Three-Way Consent: ❌ Designed, not built

### Interview Summary
**Key Discussions**:
- WebDAV: Use for document storage, integrate with BlindFill
- Calendar: CalDAV first (self-hosted), Google Calendar later
- Rolodex: Contact storage with encryption for sensitive fields
- Three-Way Consent: Matrix room with user + agent + bridge participants

**Research Findings**:
- **Existing Skills**: Learn Website (✅), BlindFill (✅), Email (✅), Web Search (✅)
- **Missing Skills**: WebDAV (❌), Calendar (❌)
- **Database Patterns**: SQLCipher, TEXT primary keys, INTEGER timestamps, JSON for structured data
- **Matrix Patterns**: Room creation via CreateRoom, invitations via InviteUser, events via SendMessage
- **HITL Consent**: Existing AccessRequest flow with pending/approved/rejected states

### Metis Review
**Identified Gaps** (addressed):
- WebDAV: Need WebDAV client library and no external dependencies
- Calendar: Need CalDAV client (github.com/emersion CaldAV)
- Rolodex: Need new tables in secretary schema
- Three-Way Consent: Need Matrix room creation + consent state machine

---

## Work Objectives

### Core Objective
Implement the missing 20% of the Skills System (WebDAV, Calendar) and the high-priority Planned Features (Rolodex Database, Three-Way Consent).

### Concrete Deliverables
- `bridge/internal/skills/webdav.go` - WebDAV skill implementation
- `bridge/internal/skills/calendar.go` - Calendar skill with CalDAV
- `bridge/pkg/secretary/schema.sql` - Add rolodex tables
- `bridge/pkg/secretary/rolodex.go` - Rolodex service
- `bridge/pkg/pii/three_way_consent.go` - Three-Way Consent integration

### Definition of Done
- [ ] `!secretary webdav <url>` returns document contents
- [ ] `!secretary calendar add appointment` creates calendar event
- [ ] `!secretary contact add` stores contact in rolodex
- [ ] Three-Way Consent creates Matrix room for approval

### Must Have
- WebDAV skill for file operations (upload, download, list)
- Calendar skill with CalDAV support
- Rolodex database with encrypted contact storage
- Three-Way Consent with Matrix room creation

### Must NOT Have (Guardrails)
- NO external WebDAV services (self-hosted only)
- NO Google Calendar API in first phase (CalDAV only)
- NO favorite locations in rolodex (out of scope)
- NO automatic approvals in Three-Way Consent (manual only)

---

## Verification Strategy

### Test Decision
- **Infrastructure exists**: NO unit tests for infrastructure
- **Automated tests**: NONE (infrastructure changes)
- **Verification**: Agent-executed QA scenarios via Bash commands

### QA Policy
Every task includes agent-executed QA scenarios using Bash commands to verify Docker healthcheck status, IP format, build output, and rollback functionality.

Evidence saved to `.sisyphus/evidence/task-{N}-{scenario-slug}.{ext}`.

---

## Execution Strategy

### Parallel Execution Waves

```
Wave 1 (Start Immediately - independent skills):
├── Task 1: WebDAV Skill Implementation [quick]
└── Task 2: Calendar Skill (CalDAV) Implementation [quick]

Wave 2 (After Wave 1 - database foundation):
├── Task 3: Rolodex Database Schema [quick]
└── Task 4: Rolodex Service Implementation [unspecified-high]

Wave 3 (After Wave 2 - integration):
└── Task 5: Three-Way Consent Matrix Integration [unspecified-high]

Wave FINAL (After ALL tasks - verification):
├── Task F1: Plan compliance audit (oracle)
├── Task F2: Skills integration review (unspecified-high)
└── Task F3: Scope fidelity check (deep)

Critical Path: Task 1-2 (parallel) → Task 3-4 (parallel) → Task 5 → Final
Parallel Speedup: ~60% faster than sequential
Max Concurrent: 2 (Wave 1)
```

### Dependency Matrix
- **1-2**: — — 3-4
- **3-4**: 1-2 — 5
- **5**: 3-4 — F1-F3
- **F1-F3**: 1-5 — —

### Agent Dispatch Summary
- **Wave 1**: **2** agents → `quick`
- **Wave 2**: **2** agents → `quick`, `unspecified-high`
- **Wave 3**: **1** agent → `unspecified-high`
- **FINAL**: **3** agents → `oracle`, `unspecified-high`, `deep`

---

## TODOs

- [ ] 1. WebDAV Skill Implementation

  **What to do**:
  - Create `bridge/internal/skills/webdav.go`
  - Implement WebDAV client using Go standard library (net/http)
  - Support operations: upload, download, list, delete
  - Use existing SSRF protection patterns from `ssrf.go`
  - Add skill to registry in `registry.go`

  **Must NOT do**:
  - Do NOT add external WebDAV library dependencies
  - Do NOT create WebDAV server (client only)

  **Recommended Agent Profile**:
  - **Category**: `quick`
    - Reason: Single file, well-defined WebDAV protocol
  - **Skills**: []
  - **Skills Evaluated but Omitted**:
    - `git-master`: Simple file, atomic commit afterward

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 1 (with Task 2)
  - **Blocks**: Task 3 (rolodex needs file operations)
  - **Blocked By**: None (can start immediately)

  **References**:
  **Pattern References**:
  - `bridge/internal/skills/file_read.go` - File operation patterns
  - `bridge/internal/skills/ssrf.go` - SSRF validation for URLs
  - `bridge/internal/skills/registry.go` - Skill registration pattern

  **API/Type References**:
  - WebDAV protocol: RFC 4918 (PROPFIND, GET, PUT, DELETE methods)
  - Go net/http client for HTTP requests

  **WHY Each Reference Matters**:
  - file_read.go shows how to implement file operations as a skill
  - ssrf.go provides URL validation for WebDAV endpoints
  - registry.go shows how to register skills with the executor

  **Acceptance Criteria**:
  - [ ] webdav.go created in bridge/internal/skills/
  - [ ] Skill registered in registry.go
  - [ ] SSRF validation applied to WebDAV URLs

  **QA Scenarios**:

  ```
  Scenario: WebDAV skill uploads file
    Tool: Bash
    Preconditions: WebDAV server running (docker compose)
    Steps:
      1. curl -X PROPFIND http://localhost:8080/webdav/
      2. Verify PROPFIND response
    Expected Result: Response contains WebDAV XML
    Failure Indicators: Connection refused or non-WebDAV response
    Evidence: .sisyphus/evidence/task-01-webdav-upload.txt

  Scenario: WebDAV skill lists files
    Tool: Bash
    Preconditions: Files exist in WebDAV server
    Steps:
      1. curl -X PROPFIND -H "Depth: 1" http://localhost:8080/webdav/
      2. grep -c "href" response.xml
    Expected Result: Count > 0
    Failure Indicators: Empty response or no href elements
    Evidence: .sisyphus/evidence/task-01-webdav-list.txt
  ```

  **Evidence to Capture**:
  - [ ] curl PROPFIND response
  - [ ] File upload/download verification

  **Commit**: YES
  - Message: `feat(skills): add WebDAV skill for file operations`
  - Files: `bridge/internal/skills/webdav.go`
  - Pre-commit: `go build ./bridge/...`

---

- [ ] 2. Calendar Skill (CalDAV) Implementation

  **What to do**:
  - Create `bridge/internal/skills/calendar.go`
  - Use github.com/emersion/go-webdav/caldav library
  - Support operations: list_calendars, create_event, get_events, delete_event
  - Add skill to registry in `registry.go`
  - Include conflict detection for scheduling

  **Must NOT do**:
  - Do NOT implement Google Calendar API in this phase
  - Do NOT add calendar UI components

  **Recommended Agent Profile**:
  - **Category**: `quick`
    - Reason: Single file, well-defined CalDAV protocol
  - **Skills**: []
  - **Skills Evaluated but Omitted**:
    - `git-master`: Simple file, atomic commit afterward

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 1 (with Task 1)
  - **Blocks**: None (independent skill)
  - **Blocked By**: None (can start immediately)

  **References**:
  **Pattern References**:
  - `bridge/internal/skills/email_send.go` - External service integration pattern
  - `bridge/internal/skills/registry.go` - Skill registration pattern

  **API/Type References**:
  - github.com/emersion/go-webdav/caldav - CalDAV client library
  - CalDAV protocol: RFC 4791

  **WHY Each Reference Matters**:
  - email_send.go shows how to integrate external services (SMTP) as skills
  - registry.go shows how to register skills with the executor

  **Acceptance Criteria**:
  - [ ] calendar.go created in bridge/internal/skills/
  - [ ] Skill registered in registry.go
  - [ ] CalDAV library added to go.mod

  **QA Scenarios**:

  ```
  Scenario: Calendar skill lists calendars
    Tool: Bash
    Preconditions: CalDAV server running (e.g., Radicale)
    Steps:
      1. curl -X PROPFIND -u user:pass http://localhost:5232/user/calendars/
      2. Verify CalDAV response
    Expected Result: Response contains calendar XML
    Failure Indicators: Connection refused or non-CalDAV response
    Evidence: .sisyphus/evidence/task-02-calendar-list.txt

  Scenario: Calendar skill creates event
    Tool: Bash
    Preconditions: CalDAV server running
    Steps:
      1. curl -X PUT -H "Content-Type: text/calendar" http://localhost:5232/user/calendars/test/test.ics -d "BEGIN:VCALENDAR..."
      2. curl -X GET http://localhost:5232/user/calendars/test/test.ics
    Expected Result: Event exists in calendar
    Failure Indicators: Event not found after creation
    Evidence: .sisyphus/evidence/task-02-calendar-create.txt
  ```

  **Evidence to Capture**:
  - [ ] curl CalDAV response
  - [ ] Event creation verification

  **Commit**: YES
  - Message: `feat(skills): add Calendar skill with CalDAV support`
  - Files: `bridge/internal/skills/calendar.go`, `go.mod`
  - Pre-commit: `go build ./bridge/...`

---

- [ ] 3. Rolodex Database Schema

  **What to do**:
  - Add rolodex tables to `bridge/pkg/secretary/schema.sql`
  - Tables: user_contacts, contact_details (encrypted), contact_relationships
  - Follow existing patterns: TEXT primary keys, INTEGER timestamps, JSON for structured data
  - Add indexes for user_id, relationship_type, last_contacted_at

  **Must NOT do**:
  - Do NOT store favorite locations (restaurants, venues) - out of scope
  - Do NOT store social media profiles - out of scope

  **Recommended Agent Profile**:
  - **Category**: `quick`
    - Reason: Single file edit, well-defined schema patterns
  - **Skills**: []
  - **Skills Evaluated but Omitted**:
    - `git-master`: Simple edit, atomic commit afterward

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 2 (with Task 4)
  - **Blocks**: Task 4 (rolodex service needs schema)
  - **Blocked By**: None (can start immediately)

  **References**:
  **Pattern References**:
  - `bridge/pkg/secretary/schema.sql:1-100` - Existing schema patterns
  - `bridge/pkg/studio/schema.sql:32-42` - skill_registry pattern
  - `bridge/pkg/keystore/keystore.go` - Encryption patterns

  **API/Type References**:
  - SQLCipher for encrypted contact_details table

  **WHY Each Reference Matters**:
  - secretary schema shows the existing table structure patterns
  - studio schema shows how to store structured data as JSON
  - keystore shows how to implement encrypted storage

  **Acceptance Criteria**:
  - [ ] user_contacts table added to schema.sql
  - [ ] contact_details table with encrypted BLOB
  - [ ] contact_relationships table for user-agent relationships
  - [ ] Indexes created for user_id, relationship_type

  **QA Scenarios**:

  ```
  Scenario: Rolodex schema validates
    Tool: Bash
    Preconditions: schema.sql exists
    Steps:
      1. sqlite3 :memory: < bridge/pkg/secretary/schema.sql
      2. .tables
    Expected Result: Tables include user_contacts, contact_details
    Failure Indicators: SQL syntax error or missing tables
    Evidence: .sisyphus/evidence/task-03-schema-validate.txt

  Scenario: Contact can be inserted
    Tool: Bash
    Preconditions: Schema loaded
    Steps:
      1. sqlite3 :memory: < bridge/pkg/secretary/schema.sql
      2. INSERT INTO user_contacts (id, user_id, name, created_at, updated_at) VALUES ('test', 'user1', 'John Doe', 0, 0);
      3. SELECT * FROM user_contacts;
    Expected Result: Row returned with John Doe
    Failure Indicators: Insert fails or no rows
    Evidence: .sisyphus/evidence/task-03-contact-insert.txt
  ```

  **Evidence to Capture**:
  - [ ] sqlite3 schema validation output
  - [ ] Insert/select verification

  **Commit**: YES
  - Message: `feat(secretary): add rolodex database schema`
  - Files: `bridge/pkg/secretary/schema.sql`
  - Pre-commit: `sqlite3 :memory: < bridge/pkg/secretary/schema.sql`

---

- [ ] 4. Rolodex Service Implementation

  **What to do**:
  - Create `bridge/pkg/secretary/rolodex.go`
  - Implement CRUD operations for contacts
  - Add search by name, company, relationship
  - Implement encryption for sensitive contact details using existing crypto patterns
  - Add Matrix command handler for `!secretary contact` commands

  **Must NOT do**:
  - Do NOT implement contact import from external sources
  - Do NOT add contact deduplication logic

  **Recommended Agent Profile**:
  - **Category**: `unspecified-high`
    - Reason: Multiple concerns (CRUD, encryption, Matrix integration)
  - **Skills**: []
  - **Skills Evaluated but Omitted**:
    - `git-master`: Simple file, atomic commit afterward

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 2 (with Task 3)
  - **Blocks**: Task 5 (Three-Way Consent needs contacts)
  - **Blocked By**: Task 3 (schema must exist)

  **References**:
  **Pattern References**:
  - `bridge/pkg/secretary/blindfill.go` - Service implementation pattern
  - `bridge/pkg/keystore/keystore.go` - Encryption patterns
  - `bridge/pkg/secretary/secretary_commands.go` - Matrix command handler

  **API/Type References**:
  - `bridge/pkg/secretary/store.go` - Database store interface

  **WHY Each Reference Matters**:
  - blindfill.go shows how to implement secretary services with approval flows
  - keystore.go provides encryption patterns for sensitive contact data
  - secretary_commands.go shows how to add Matrix commands

  **Acceptance Criteria**:
  - [ ] rolodex.go created in bridge/pkg/secretary/
  - [ ] CRUD operations work with encrypted contact details
  - [ ] Matrix command handler added for contacts
  - [ ] Search functionality implemented

  **QA Scenarios**:

  ```
  Scenario: Contact can be created via Matrix command
    Tool: Bash
    Preconditions: Bridge running, Matrix connected
    Steps:
      1. Send Matrix message: "!secretary contact add --name 'John Doe' --email 'john@example.com'"
      2. Send Matrix message: "!secretary contact search 'John'"
      3. Verify response contains John Doe
    Expected Result: Contact found in search results
    Failure Indicators: Contact not found or command not recognized
    Evidence: .sisyphus/evidence/task-04-contact-create.txt

  Scenario: Contact details are encrypted
    Tool: Bash
    Preconditions: Contact with sensitive data exists
    Steps:
      1. sqlite3 /var/lib/armorclaw/secretary.db "SELECT contact_data FROM contact_details LIMIT 1"
      2. Verify output is not plaintext
    Expected Result: Output is encrypted BLOB or base64
    Failure Indicators: Plaintext email or phone visible
    Evidence: .sisyphus/evidence/task-04-encryption.txt
  ```

  **Evidence to Capture**:
  - [ ] Matrix command response
  - [ ] Database encryption verification

  **Commit**: YES
  - Message: `feat(secretary): add Rolodex contact management service`
  - Files: `bridge/pkg/secretary/rolodex.go`
  - Pre-commit: `go build ./bridge/...`

---

- [ ] 5. Three-Way Consent Matrix Integration

  **What to do**:
  - Create `bridge/pkg/pii/three_way_consent.go`
  - Implement Matrix room creation for consent (user + agent + bridge)
  - Add consent request event types
  - Implement approval/rejection handlers
  - Integrate with existing HITL consent flow in `hitl_consent.go`
  - Add token validation for room-based approvals

  **Must NOT do**:
  - Do NOT implement automatic approvals (manual only)
  - Do NOT add voice approval workflows (separate feature)
  - Do NOT modify existing HITL consent for direct approvals

  **Recommended Agent Profile**:
  - **Category**: `unspecified-high`
    - Reason: Multiple integrations (Matrix, HITL, audit logging)
  - **Skills**: []
  - **Skills Evaluated but Omitted**:
    - `git-master`: Complex feature, atomic commit afterward

  **Parallelization**:
  - **Can Run In Parallel**: NO
  - **Parallel Group**: Wave 3
  - **Blocks**: Final verification
  - **Blocked By**: Tasks 3-4 (rolodex must exist for contact-based consent)

  **References**:
  **Pattern References**:
  - `bridge/pkg/pii/hitl_consent.go` - Existing consent flow
  - `bridge/pkg/matrix/client.go` - Matrix room creation
  - `bridge/internal/adapter/pii_consent.go` - Matrix notification callback

  **API/Type References**:
  - Matrix room creation: `CreateRoom`, `InviteUser`
  - Consent states: pending, approved, rejected, expired

  **WHY Each Reference Matters**:
  - hitl_consent.go provides the existing consent architecture to extend
  - matrix client shows how to create rooms and send events
  - pii_consent.go shows the Matrix notification integration pattern

  **Acceptance Criteria**:
  - [ ] three_way_consent.go created in bridge/pkg/pii/
  - [ ] Matrix room creation for consent requests
  - [ ] Integration with HITL consent manager
  - [ ] Approval/rejection via Matrix reactions

  **QA Scenarios**:

  ```
  Scenario: Three-way consent room is created
    Tool: Bash
    Preconditions: Bridge running, Matrix connected, PII request pending
    Steps:
      1. Trigger PII access request
      2. List Matrix rooms: curl http://localhost:6167/_matrix/client/v3/publicRooms
      3. Verify room with "consent" in name exists
    Expected Result: Consent room visible in room list
    Failure Indicators: No consent room created
    Evidence: .sisyphus/evidence/task-05-consent-room.txt

  Scenario: Approval via Matrix reaction
    Tool: Bash
    Preconditions: Consent room exists, user invited
    Steps:
      1. Send approval reaction to consent event
      2. Query HITL consent status
      3. Verify status is "approved"
    Expected Result: Consent status updated to approved
    Failure Indicators: Status remains pending
    Evidence: .sisyphus/evidence/task-05-approval-reaction.txt
  ```

  **Evidence to Capture**:
  - [ ] Matrix room creation verification
  - [ ] Approval reaction verification

  **Commit**: YES
  - Message: `feat(consent): add Three-Way Consent with Matrix room integration`
  - Files: `bridge/pkg/pii/three_way_consent.go`
  - Pre-commit: `go build ./bridge/...`

---

## Final Verification Wave (MANDATORY)

- [ ] F1. **Plan Compliance Audit** — `oracle`
  Read the plan end-to-end. For each "Must Have": verify implementation exists. For each "Must NOT Have": search codebase for forbidden patterns. Check evidence files exist. Compare deliverables against plan.
  Output: `Must Have [N/N] | Must NOT Have [N/N] | Tasks [N/N] | VERDICT: APPROVE/REJECT`

- [ ] F2. **Skills Integration Review** — `unspecified-high`
  Verify WebDAV and Calendar skills are registered. Test skill execution via secretary commands. Verify rolodex integration with BlindFill.
  Output: `Skills [N/N registered] | Rolodex [integrated/not] | VERDICT`

- [ ] F3. **Scope Fidelity Check** — `deep`
  For each task: verify only specified files were modified. Check no external WebDAV services. Check no Google Calendar API. Check no favorite locations in rolodex. Check manual-only consent approvals.
  Output: `Files [N modified] | Scope Creep [CLEAN/N violations] | VERDICT`

---

## Commit Strategy

- **Task 1**: `feat(skills): add WebDAV skill for file operations` — bridge/internal/skills/webdav.go
- **Task 2**: `feat(skills): add Calendar skill with CalDAV support` — bridge/internal/skills/calendar.go, go.mod
- **Task 3**: `feat(secretary): add rolodex database schema` — bridge/pkg/secretary/schema.sql
- **Task 4**: `feat(secretary): add Rolodex contact management service` — bridge/pkg/secretary/rolodex.go
- **Task 5**: `feat(consent): add Three-Way Consent with Matrix room integration` — bridge/pkg/pii/three_way_consent.go

---

## Success Criteria

### Verification Commands
```bash
# WebDAV skill registered
grep -r "webdav" bridge/internal/skills/registry.go

# Calendar skill registered
grep -r "calendar" bridge/internal/skills/registry.go

# Rolodex schema exists
grep -r "user_contacts" bridge/pkg/secretary/schema.sql

# Three-Way Consent exists
grep -r "ThreeWay" bridge/pkg/pii/three_way_consent.go
```

### Final Checklist
- [ ] All "Must Have" present
- [ ] All "Must NOT Have" absent
- [ ] All QA scenarios pass
- [ ] Evidence captured in .sisyphus/evidence/
