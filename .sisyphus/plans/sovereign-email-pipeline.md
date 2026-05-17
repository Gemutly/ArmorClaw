# Sovereign Email Pipeline

## TL;DR

> **Quick Summary**: Implement a 5-phase Zero-Trust email pipeline for ArmorClaw — inbound ingestion via Postfix, YARA scanning, PII masking, event-driven workflow dispatch, agent-based analysis, Matrix HITL approval, and outbound Gmail/Outlook sending with BlindFill PII resolution.
> 
> **Deliverables**:
> - Postfix pipe handler binary (`cmd/mta-recv`)
> - Email ingestion server with YARA + MIME parsing (`bridge/pkg/email/`)
> - Email proto definitions for gRPC
> - PII Masker (detect → BlindFill placeholders)
> - Email dispatcher (event → workflow template)
> - Bridge-local outbound executor (validate → policy → HITL → send)
> - Gmail API client + SMTP fallback
> - OAuth token storage in SQLCipher
> - Email workflow template + HITL integration
> - Android integration spec (informational)
> - Postfix installation/configuration
> - Comprehensive test suite
> 
> **Estimated Effort**: XL (20+ tasks across 4 waves)
> **Parallel Execution**: YES - 4 waves
> **Critical Path**: Proto → IngestServer → Dispatcher → Outbound Executor → E2E test

---

## Context

### Original Request
CTO memo titled "MASTER IMPLEMENTATION GUIDE: Sovereign Email Pipeline (Final Executable Baseline)" dated April 16, 2026. Specifies 5 phases: Inbound Ingestion & Airlock, Event Dispatch & Agent Wake-Up, Outbound Sequence, Security & Data Modules, Mobile Control Plane.

### Interview Summary
**Key Discussions**:
- Scope: ALL 5 phases in ONE plan
- Gmail protocol: Gmail API (primary) + SMTP fallback for non-Gmail providers
- Postfix setup: INCLUDE Postfix installation + configuration
- Provider scope: Gmail + Outlook MVP with multi-provider abstraction
- Test strategy: Tests after implementation
- Storage: Local FS (`/var/lib/armorclaw/email-files/`) for MVP
- Phase 5 Android: INFORMATIONAL ONLY — document integration points for Android team

**Research Findings**:
- YARA scanner exists and is ready (`bridge/pkg/yara/scanner.go`)
- PII Scrubber detects but does NOT generate BlindFill placeholders — new Masker needed
- BlindFill Engine resolves placeholders from encrypted profiles — can reuse
- EventBus has Subscribe with EventFilter — perfect for email events
- Secretary system has templates, workflows, orchestrator, approvals, scheduler, dispatch — all reusable
- Email Send Skill has validation but mock SMTP — extend for real sending
- SQLCipher Keystore has XChaCha20-Poly1305 encryption — extend for OAuth tokens
- Android PiiApprovalCard.kt provides HITL pattern to document for Android team

### Metis Review (Self-Performed — subagent limit reached)
**Identified Gaps** (addressed):
- Email volume: Assume ≤100/day (VPS secretary, not enterprise)
- Attachment size: Reject >25MB (Postfix default)
- Rejected HITL: Store in audit log, notify agent workflow failed
- Scope creep locked down: No threading, no calendar, no search, no forwarding, no rich HTML
- Edge cases: No-body emails, encrypted attachments, simultaneous emails, OAuth expiry, Postfix retry, malformed agent output

---

## Work Objectives

### Core Objective
Build a complete Zero-Trust email pipeline where: emails arrive via Postfix → Bridge scans/masks/processes → agent analyzes → partner approves via ArmorChat → response sent via Gmail/Outlook. All PII is masked from agents. All sends require HITL approval.

### Concrete Deliverables
- `cmd/mta-recv/main.go` — Postfix pipe handler binary
- `bridge/pkg/email/ingest_server.go` — gRPC ingestion server
- `bridge/pkg/email/events.go` — EmailReceivedEvent
- `bridge/pkg/email/dispatcher.go` — Email dispatcher
- `bridge/pkg/email/proto/` — Email proto definitions
- `bridge/pkg/pii/masker.go` — PII Masker (detect → BlindFill placeholders)
- `bridge/pkg/keystore/oauth.go` — OAuth token storage
- `bridge/pkg/email/gmail_client.go` — Gmail API client
- `bridge/pkg/email/smtp_client.go` — SMTP fallback client
- `bridge/pkg/email/outbound_executor.go` — Bridge-local outbound step executor
- `bridge/pkg/email/email_storage.go` — Local FS email file storage
- `deploy/postfix/` — Postfix configuration files
- `doc/email-pipeline.md` — Pipeline documentation + Android integration spec
- Test files for all modules

### Definition of Done
- [ ] Email arrives at Postfix → appears as `email.received` event in EventBus
- [ ] Agent workflow triggered by email → produces draft response
- [ ] Draft response validated → HITL approval requested via Matrix
- [ ] Partner approves on ArmorChat → email sent via Gmail API
- [ ] YARA-matched attachment produces rejection + audit log
- [ ] PII in email body replaced with `{{VAULT:...}}` placeholders before agent sees it
- [ ] All tests pass: `go test ./pkg/email/... ./pkg/pii/... ./cmd/mta-recv/...`

### Must Have
- YARA scanning before any attachment processing
- PII masking before agent receives email body
- HITL approval for ALL outbound emails (no auto-send)
- OAuth tokens encrypted at rest in SQLCipher
- All email operations audit-logged
- Postfix pipe handler exits with correct codes (0=OK, 75=retry, 67=reject)
- Gmail API as primary sender, SMTP as fallback
- Multi-provider abstraction (Gmail + Outlook from start)

### Must NOT Have (Guardrails)
- Do NOT store unmasked email body — always store masked version
- Do NOT pass OAuth refresh tokens to agent containers
- Do NOT allow Postfix pipe handler to talk to anything except Bridge gRPC socket
- Do NOT skip YARA scanning for any attachment
- Do NOT auto-send emails without HITL approval (except when policy auto-approves with explicit user configuration)
- Do NOT implement email threading/conversation tracking (MVP)
- Do NOT implement calendar invite extraction (MVP)
- Do NOT implement email search/indexing (MVP)
- Do NOT implement rich HTML rendering (text-only for agent analysis)
- Do NOT modify existing proto files (`sidecar.proto`, `keystore.proto`)
- Do NOT modify existing Rust sidecar code
- Do NOT remove or weaken existing PII scrubber — Masker extends it
- Do NOT bypass Matrix as control plane for HITL
- Do NOT implement email forwarding rules (MVP)
- Do NOT support multiple mailboxes per provider (MVP)

---

## Verification Strategy (MANDATORY)

> **ZERO HUMAN INTERVENTION** - ALL verification is agent-executed. No exceptions.

### Test Decision
- **Infrastructure exists**: YES (Go testing + Android instrumented tests)
- **Automated tests**: Tests after implementation
- **Framework**: Go standard testing + testify
- **Android tests**: NONE (informational only for this plan)

### QA Policy
Every task MUST include agent-executed QA scenarios.
Evidence saved to `.sisyphus/evidence/task-{N}-{scenario-slug}.{ext}`.

- **Go unit/integration**: Use Bash (`go test`) — Run tests, assert pass/fail
- **gRPC services**: Use Bash — Start server, send request, assert response
- **MIME parsing**: Use Bash — Feed raw email, assert parsed fields
- **PII masking**: Use Bash — Feed text with PII, assert placeholders generated

---

## Execution Strategy

### Parallel Execution Waves

```
Wave 1 (Foundation — proto, types, infrastructure):
├── T1: Email proto definitions [quick]
├── T2: Email types + event definitions [quick]
├── T3: Email storage interface + local FS impl [quick]
├── T3.5: Secretary schema migrations + DispatchNow + trigger lookup [quick]
├── T4: Postfix installation + pipe config scripts [quick]
├── T5: MIME parser utility [unspecified-high]
├── T6: PII Masker (detect → BlindFill placeholders) [deep]
└── T11: OAuth token storage in SQLCipher [deep]

Wave 2 (Core servers + clients):
├── T7: Postfix pipe handler (cmd/mta-recv) (depends: T1, T5) [unspecified-high]
├── T8: Email IngestServer (depends: T1, T2, T3, T6) [deep]
├── T9: Gmail API client (depends: T1) [unspecified-high]
├── T10: SMTP fallback client (depends: T1) [unspecified-high]
├── T12: Email dispatcher (depends: T2, T3.5, T8) [unspecified-high]
└── T13: Email workflow template setup (depends: T2, T3.5) [quick]

Wave 3 (Integration + outbound):
├── T13.5: Bridge-local execution mode (depends: T13) [deep]
├── T14: Bridge-local outbound executor (depends: T8, T9, T10, T11, T12, T13.5) [deep]
├── T15: HITL email approval integration (depends: T2, T14) [unspecified-high]
├── T16: Outlook provider adapter (depends: T9, T10, T11) [unspecified-high]
├── T17: Email audit logging (depends: T8, T14) [quick]
└── T18: Android integration spec document (depends: T15) [writing]

Wave 4 (Testing + documentation):
├── T19: End-to-end pipeline test (depends: T7-T17) [deep]
├── T20: Edge case + security test suite (depends: T19) [unspecified-high]
├── T21: Pipeline documentation (depends: T18, T19) [writing]
└── T22: Postfix + Bridge integration verification (depends: T4, T19) [quick]

Wave FINAL (After ALL tasks — 4 parallel reviews, then user okay):
├── F1: Plan compliance audit (oracle)
├── F2: Code quality review (unspecified-high)
├── F3: Real manual QA (unspecified-high)
└── F4: Scope fidelity check (deep)
-> Present results -> Get explicit user okay

Critical Path: T1 → T8 → T12 → T14 → T19 → FINAL
Also required for T14: T13.5 chain (T3.5 → T13 → T13.5)
Parallel Speedup: ~65% faster than sequential
Max Concurrent: 8 (Wave 1), 6 (Wave 2), 6 (Wave 3)
```

### Dependency Matrix

| Task | Depends On | Blocks |
|------|-----------|--------|
| T1 | — | T7, T8, T9, T10 |
| T2 | — | T8, T12, T13, T15 |
| T3 | — | T8 |
| T3.5 | — | T12, T13 |
| T4 | — | T22 |
| T5 | — | T7 |
| T6 | — | T8 |
| T7 | T1, T5 | T19 |
| T8 | T1, T2, T3, T6 | T12, T14, T17, T19 |
| T9 | T1 | T14, T16 |
| T10 | T1 | T14, T16 |
| T11 | — | T14, T16 |
| T12 | T2, T3.5, T8 | T14, T19 |
| T13 | T2, T3.5 | T13.5, T19 |
| T13.5 | T13 | T14 |
| T14 | T8, T9, T10, T11, T12, T13.5 | T15, T17, T19 |
| T15 | T2, T14 | T18, T19 |
| T16 | T9, T10, T11 | T19 |
| T17 | T8, T14 | T19 |
| T18 | T15 | T21 |
| T19 | T7-T17 | T20, T21, T22 |
| T20 | T19 | — |
| T21 | T18, T19 | — |
| T22 | T4, T19 | — |

### Agent Dispatch Summary

- **Wave 1** (8 tasks): T1 `quick`, T2 `quick`, T3 `quick`, T3.5 `quick`, T4 `quick`, T5 `unspecified-high`, T6 `deep`, T11 `deep`
- **Wave 2** (6 tasks): T7 `unspecified-high`, T8 `deep`, T9 `unspecified-high`, T10 `unspecified-high`, T12 `unspecified-high`, T13 `quick`
- **Wave 3** (6 tasks): T13.5 `deep`, T14 `deep`, T15 `unspecified-high`, T16 `unspecified-high`, T17 `quick`, T18 `writing`
- **Wave 4** (4 tasks): T19 `deep`, T20 `unspecified-high`, T21 `writing`, T22 `quick`
- **FINAL** (4 tasks): F1 `oracle`, F2 `unspecified-high`, F3 `unspecified-high`, F4 `deep`

---

## TODOs

- [x] 1. Email Proto Definitions

  **What to do**:
  - Create `bridge/pkg/email/proto/email.proto` with `EmailIngestService` gRPC service
  - Define messages: `IngestEmailRequest` (From, To, Subject, BodyText, Attachments), `IngestEmailResponse` (Accepted bool, FileIDs)
  - Define `Attachment` message (Filename, Content bytes, ContentType, ContentID)
  - Define `EmailSendRequest` and `EmailSendResponse` for outbound
  - Generate Go code from proto (`go generate`)
  - Register service in Bridge gRPC server

  **Must NOT do**:
  - Do NOT modify existing proto files (`sidecar.proto`, `keystore.proto`)
  - Do NOT add fields unrelated to email ingestion/sending

  **Recommended Agent Profile**:
  - **Category**: `quick`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 1 (with T2-T6)
  - **Blocks**: T7, T8, T9, T10
  - **Blocked By**: None (can start immediately)

  **References**:
  - `bridge/pkg/sidecar/sidecar.proto` — Follow this proto file structure and naming conventions (package, go_package, service definition pattern)
  - `bridge/pkg/keystore/keystore.proto` — Another reference for proto style in this codebase
  - `bridge/cmd/bridge/main.go` — Where gRPC services are registered; add EmailIngestService registration here

  **Acceptance Criteria**:
  - [ ] Proto file compiles: `protoc --go_out=. bridge/pkg/email/proto/email.proto`
  - [ ] Generated Go code exists in `bridge/pkg/email/proto/`
  - [ ] `go build ./bridge/pkg/email/proto/...` succeeds

  **QA Scenarios**:
  ```
  Scenario: Proto compilation succeeds
    Tool: Bash
    Steps:
      1. Run: protoc --go_out=. --go-grpc_out=. bridge/pkg/email/proto/email.proto
      2. Assert: exit code 0
      3. Assert: file exists bridge/pkg/email/proto/email.pb.go
      4. Assert: file exists bridge/pkg/email/proto/email_grpc.pb.go
    Expected Result: Proto compiles without errors, Go files generated
    Evidence: .sisyphus/evidence/task-1-proto-compile.txt

  Scenario: Go build succeeds with generated proto
    Tool: Bash
    Steps:
      1. Run: cd bridge && go build ./pkg/email/proto/...
      2. Assert: exit code 0
    Expected Result: No compilation errors
    Evidence: .sisyphus/evidence/task-1-go-build.txt
  ```

  **Commit**: YES (groups with Wave 1)
  - Message: `feat(email): add proto definitions, types, storage, MIME parser, PII masker`
  - Files: `bridge/pkg/email/proto/email.proto`, generated files

- [x] 2. Email Types + Event Definitions

  **What to do**:
  - Create `bridge/pkg/email/events.go` with `EmailReceivedEvent` struct
  - Implement `BridgeEvent` interface: `EventType() → "email.received"`, `Timestamp()`, `ToJSON()`
  - Define event fields: From, To, Subject, BodyMasked, FileIDs, PIIFields, Timestamp
  - Add `EventTypeEmailReceived = "email.received"` constant to eventbus constants
  - Create `bridge/pkg/email/types.go` with shared email types (EmailAddress, EmailAttachment, ProcessedFile, etc.)

  **Must NOT do**:
  - Do NOT store unmasked body in EmailReceivedEvent — BodyMasked only
  - Do NOT add event types for non-email features

  **Recommended Agent Profile**:
  - **Category**: `quick`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 1 (with T1, T3-T6)
  - **Blocks**: T8, T12, T13, T15
  - **Blocked By**: None

  **References**:
  - `bridge/pkg/eventbus/events.go:59-63` — BridgeEvent interface that EmailReceivedEvent must implement
  - `bridge/pkg/eventbus/events.go:12-56` — Existing event type constants; add `EventTypeEmailReceived` here
  - `bridge/pkg/eventbus/events.go:66-84` — BaseEvent struct pattern to embed in EmailReceivedEvent

  **Acceptance Criteria**:
  - [ ] `EmailReceivedEvent` implements `BridgeEvent` interface (compile check)
  - [ ] `EventTypeEmailReceived` constant added to eventbus
  - [ ] `go build ./bridge/pkg/email/...` succeeds

  **QA Scenarios**:
  ```
  Scenario: EmailReceivedEvent implements BridgeEvent interface
    Tool: Bash
    Steps:
      1. Create test that assigns EmailReceivedEvent to BridgeEvent variable
      2. Run: cd bridge && go test -run TestEmailReceivedEventInterface ./pkg/email/...
      3. Assert: PASS
    Expected Result: Interface compliance verified at compile time
    Evidence: .sisyphus/evidence/task-2-event-interface.txt

  Scenario: Event serializes to valid JSON with masked body only
    Tool: Bash
    Steps:
      1. Create EmailReceivedEvent with test data
      2. Call ToJSON()
      3. Assert: JSON contains "email.received" type
      4. Assert: JSON contains body_masked field (NOT body)
    Expected Result: Valid JSON with masked body only
    Evidence: .sisyphus/evidence/task-2-event-json.txt
  ```

  **Commit**: YES (groups with Wave 1)

- [x] 3. Email Storage Interface + Local FS Implementation

  **What to do**:
  - Create `bridge/pkg/email/email_storage.go` with `EmailStorage` interface
  - Interface methods: `StoreProcessedFile(ctx, filename, text) → (fileID, error)`, `RetrieveProcessedFile(ctx, fileID) → (text, error)`, `DeleteProcessedFile(ctx, fileID) → error`, `ListFiles(ctx) → ([]ProcessedFileMeta, error)`
  - Implement `LocalFSEmailStorage` that stores to `/var/lib/armorclaw/email-files/`
  - File IDs are SHA256 hashes of content + timestamp
  - Files stored as `{fileID}.txt` with metadata sidecar `{fileID}.meta.json`
  - Directory created with 0700 permissions

  **Must NOT do**:
  - Do NOT use SQLCipher for email file storage (CTO: documents not in SQLCipher)
  - Do NOT implement S3 storage (MVP is local FS only)

  **Recommended Agent Profile**:
  - **Category**: `quick`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 1 (with T1-T2, T4-T6)
  - **Blocks**: T8, T11
  - **Blocked By**: None

  **References**:
  - `bridge/pkg/secretary/types.go` — Pattern for interface + implementation in this codebase
  - `bridge/pkg/keystore/keystore.go:39-50` — Security constants pattern (file permissions)

  **Acceptance Criteria**:
  - [ ] `EmailStorage` interface defined with 4 methods
  - [ ] `LocalFSEmailStorage` implements interface
  - [ ] Files stored with 0600 permissions
  - [ ] `go test ./bridge/pkg/email/... -run TestEmailStorage` passes

  **QA Scenarios**:
  ```
  Scenario: Store and retrieve a processed file
    Tool: Bash
    Steps:
      1. Create LocalFSEmailStorage with temp dir
      2. Call StoreProcessedFile with "test.txt" and "email content"
      3. Assert: fileID returned (non-empty string)
      4. Call RetrieveProcessedFile with fileID
      5. Assert: content matches "email content"
    Expected Result: Round-trip store/retrieve succeeds
    Evidence: .sisyphus/evidence/task-3-storage-roundtrip.txt

  Scenario: File permissions are 0600
    Tool: Bash
    Steps:
      1. Store a file
      2. os.Stat the file
      3. Assert: FileMode.Perm() == 0600
    Expected Result: Secure file permissions enforced
    Evidence: .sisyphus/evidence/task-3-storage-perms.txt
  ```

  **Commit**: YES (groups with Wave 1)

- [x] 3.5 Secretary Schema Migrations for Email Workflow

  **What to do**:
  - Add SQL migrations to `bridge/pkg/secretary/schema.sql`:
    ```sql
    ALTER TABLE task_templates ADD COLUMN trigger TEXT;
    ALTER TABLE task_templates ADD COLUMN default_definition_id TEXT;
    ALTER TABLE scheduled_tasks ADD COLUMN one_shot INTEGER DEFAULT 0;
    ```
  - Update `task_templates` to support trigger-based lookup: `"email:secretary@example.com"` maps to an email workflow template
  - Update `scheduled_tasks` to support `one_shot` tasks that auto-deactivate after execution
  - Update `TaskScheduler.tick()` to check `one_shot` flag and deactivate task after successful dispatch
  - Add `GetTemplateByTrigger(ctx, triggerKey) → (*TaskTemplate, error)` method to Secretary Store
  - Add `DispatchNow(ctx, task) → error` method to TaskScheduler — immediately evaluates and dispatches a task outside the cron tick loop

  **Must NOT do**:
  - Do NOT modify existing scheduler cron behavior — DispatchNow is additive
  - Do NOT break existing template/workflow queries

  **Recommended Agent Profile**:
  - **Category**: `quick`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 1 (with T1-T6)
  - **Blocks**: T12, T13
  - **Blocked By**: None

  **References**:
  - `bridge/pkg/secretary/schema.sql:10-21` — `task_templates` table to ALTER
  - `bridge/pkg/secretary/schema.sql:85-99` — `scheduled_tasks` table to ALTER
  - `bridge/pkg/secretary/store.go` — Store implementation where `GetTemplateByTrigger()` will be added
  - `bridge/pkg/secretary/task_scheduler.go` — Scheduler where `DispatchNow()` and `one_shot` handling will be added
  - `bridge/pkg/secretary/task_dispatch.go` — Existing dispatch logic to reuse in `DispatchNow()`

  **Acceptance Criteria**:
  - [ ] `task_templates` has `trigger` and `default_definition_id` columns
  - [ ] `scheduled_tasks` has `one_shot` column
  - [ ] `GetTemplateByTrigger("email:test@example.com")` returns matching template
  - [ ] `DispatchNow()` executes task immediately without waiting for cron tick
  - [ ] One-shot tasks auto-deactivate after dispatch
  - [ ] `go test ./bridge/pkg/secretary/...` passes (no regressions)

  **QA Scenarios**:
  ```
  Scenario: Template trigger lookup works
    Tool: Bash
    Steps:
      1. Insert template with trigger="email:bot@example.com"
      2. Call GetTemplateByTrigger("email:bot@example.com")
      3. Assert: template returned with correct ID
      4. Call GetTemplateByTrigger("email:unknown@example.com")
      5. Assert: error (not found)
    Expected Result: Trigger-based template lookup
    Evidence: .sisyphus/evidence/task-3.5-trigger-lookup.txt

  Scenario: DispatchNow executes immediately
    Tool: Bash
    Steps:
      1. Create one-shot task
      2. Call DispatchNow(ctx, task)
      3. Assert: task executed synchronously
      4. Assert: task deactivated (IsActive == false)
    Expected Result: Immediate dispatch with auto-deactivation
    Evidence: .sisyphus/evidence/task-3.5-dispatch-now.txt

  Scenario: Existing scheduler tests still pass
    Tool: Bash
    Steps:
      1. Run: cd bridge && go test ./pkg/secretary/...
      2. Assert: all PASS, no regressions
    Expected Result: Schema changes are backward-compatible
    Evidence: .sisyphus/evidence/task-3.5-regression.txt
  ```

  **Commit**: YES (groups with Wave 1)

- [x] 4. Postfix Installation + Pipe Configuration Scripts

  **What to do**:
  - Create `deploy/postfix/install-postfix.sh` — installs Postfix, configures for external email reception
  - Create `deploy/postfix/master.cf.entry` — pipe(8) transport entry pointing to `/usr/local/bin/armorclaw-mta-recv`
  - Create `deploy/postfix/main.cf.template` — Postfix config for receiving external email and routing to pipe:
    - `inet_interfaces = all` (must accept external SMTP connections on Port 25)
    - `smtpd_tls_security_level = may` (opportunistic STARTTLS — enforced where possible, plaintext fallback)
    - `smtpd_tls_cert_file` and `smtpd_tls_key_file` pointing to Let's Encrypt or self-signed cert
    - `maximal_queue_lifetime = 1d` (bounce undeliverable after 1 day)
    - `mailbox_command = /usr/local/bin/armorclaw-mta-recv` or transport_maps for domain-based routing
  - Create `deploy/postfix/dns-template.txt` — DNS records needed:
    - MX record pointing to VPS IP
    - SPF TXT record: `v=spf1 mx -all`
  - Create `deploy/postfix/rspamd-integration.md` — document Rspamd milter integration for SPF/DKIM verification
  - Configure `armorclaw` transport type in master.cf (routes `*@agent.example.com` to pipe handler)
  - Ensure Postfix runs as isolated user, no root

  **Must NOT do**:
  - Do NOT configure Postfix as open relay — only accept mail for configured domains
  - Do NOT skip STARTTLS configuration
  - Do NOT require sudo for testing (scripts only needed in deployment)

  **Recommended Agent Profile**:
  - **Category**: `quick`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 1 (with T1-T3, T5-T6)
  - **Blocks**: T22
  - **Blocked By**: None

  **References**:
  - `deploy/` — Existing deployment scripts for conventions
  - `deploy/install.sh` — Installer script pattern to follow
  - CTO memo Phase 1 — External MTA connects on Port 25 with enforced STARTTLS
  - CTO memo Operational Security section — STARTTLS cert configuration
  - CTO memo DNS section — MX record, SPF record requirements

  **Acceptance Criteria**:
  - [ ] `deploy/postfix/install-postfix.sh` exists and is valid bash
  - [ ] `master.cf.entry` defines `armorclaw` transport
  - [ ] `main.cf.template` sets `inet_interfaces = all` (NOT loopback-only)
  - [ ] `main.cf.template` sets `smtpd_tls_security_level = may`
  - [ ] `main.cf.template` sets `maximal_queue_lifetime = 1d`
  - [ ] `dns-template.txt` includes MX and SPF records
  - [ ] `rspamd-integration.md` documents milter setup
  - [ ] `bash -n deploy/postfix/install-postfix.sh` succeeds

  **QA Scenarios**:
  ```
  Scenario: Scripts are syntactically valid
    Tool: Bash
    Steps:
      1. Run: bash -n deploy/postfix/install-postfix.sh
      2. Assert: exit code 0
    Expected Result: All scripts parse without errors
    Evidence: .sisyphus/evidence/task-4-script-syntax.txt

  Scenario: Config accepts external email with STARTTLS
    Tool: Bash
    Steps:
      1. Grep main.cf.template for "inet_interfaces = all"
      2. Assert: found
      3. Grep for "smtpd_tls_security_level = may"
      4. Assert: found
      5. Grep for "maximal_queue_lifetime = 1d"
      6. Assert: found
      7. Grep for "relay_domains" or open relay patterns
      8. Assert: NOT found (no open relay)
    Expected Result: External email accepted with STARTTLS, no open relay
    Evidence: .sisyphus/evidence/task-4-security-config.txt

  Scenario: DNS template has MX and SPF
    Tool: Bash
    Steps:
      1. Grep dns-template.txt for "MX"
      2. Assert: found
      3. Grep for "v=spf1"
      4. Assert: found
    Expected Result: Required DNS records documented
    Evidence: .sisyphus/evidence/task-4-dns-template.txt
  ```

  **Commit**: YES (groups with Wave 1)

- [x] 5. MIME Parser Utility

  **What to do**:
  - Create `bridge/pkg/email/mime_parser.go` with `ParseMIME(rawMessage []byte) (*ParsedEmail, error)`
  - Use Go stdlib `mime` and `mime/multipart` packages (no external deps)
  - Extract: From, To, CC, Subject, Date, BodyText, BodyHTML
  - Extract attachments with content type detection
  - Handle: base64, quoted-printable encodings
  - Handle multipart/alternative (prefer text/plain over HTML)
  - Handle multipart/mixed (body + attachments)
  - Handle multipart/related (embedded images)
  - Return `ParsedEmail` struct with all extracted data

  **Must NOT do**:
  - Do NOT add external MIME parsing dependencies — use stdlib only
  - Do NOT render HTML — extract text content only
  - Do NOT execute any embedded scripts or load external resources

  **Recommended Agent Profile**:
  - **Category**: `unspecified-high`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 1 (with T1-T4, T6)
  - **Blocks**: T7
  - **Blocked By**: None

  **References**:
  - `bridge/internal/skills/email_send.go:347-379` — `buildEmailContent()` shows email header construction pattern (reverse of parsing)
  - Go stdlib `net/mail` — For parsing email addresses
  - Go stdlib `mime/multipart` — For parsing multipart messages
  - Go stdlib `encoding/base64` + `mime/quotedprintable` — For decoding

  **Acceptance Criteria**:
  - [ ] `ParseMIME()` extracts From, To, Subject, BodyText from plain text email
  - [ ] `ParseMIME()` extracts attachments from multipart/mixed email
  - [ ] `ParseMIME()` handles base64-encoded attachments
  - [ ] `ParseMIME()` handles quoted-printable body text
  - [ ] `go test ./bridge/pkg/email/... -run TestMIMEParser` passes

  **QA Scenarios**:
  ```
  Scenario: Parse plain text email
    Tool: Bash
    Steps:
      1. Create raw email with From/To/Subject/Body
      2. Call ParseMIME(rawBytes)
      3. Assert: From == "sender@example.com"
      4. Assert: Subject == "Test Subject"
      5. Assert: BodyText == "Hello World"
    Expected Result: All fields extracted correctly
    Evidence: .sisyphus/evidence/task-5-mime-plain.txt

  Scenario: Parse multipart email with attachment
    Tool: Bash
    Steps:
      1. Create raw multipart/mixed email with text body + PDF attachment
      2. Call ParseMIME(rawBytes)
      3. Assert: BodyText == "See attached"
      4. Assert: len(Attachments) == 1
      5. Assert: Attachments[0].Filename == "report.pdf"
      6. Assert: Attachments[0].ContentType == "application/pdf"
    Expected Result: Body and attachment both extracted
    Evidence: .sisyphus/evidence/task-5-mime-multipart.txt

  Scenario: Parse base64 encoded attachment
    Tool: Bash
    Steps:
      1. Create email with base64-encoded binary attachment
      2. Call ParseMIME(rawBytes)
      3. Assert: attachment content decoded correctly (compare bytes)
    Expected Result: Binary attachment decoded from base64
    Evidence: .sisyphus/evidence/task-5-mime-base64.txt
  ```

  **Commit**: YES (groups with Wave 1)

- [x] 6. PII Masker (Detect → BlindFill Placeholders)

  **What to do**:
  - Create `bridge/pkg/pii/masker.go` with `Masker` struct
  - `Mask(ctx, text) → (*MaskResult, error)` — detects PII spans using existing `Scrubber.Detect()`, replaces each with `{{VAULT:category:hash}}` placeholder
  - `MaskResult` contains: MaskedText, Mapping (placeholder → original value), DetectedFields (category list)
  - `ResolvePlaceholders(ctx, maskedText, mapping) → (resolvedText, error)` — replaces placeholders with original values for outbound
  - Categories: email, phone, ssn, credit_card, name, address, date_of_birth
  - Generate stable hashes for same values (deterministic within one mask call)

  **Must NOT do**:
  - Do NOT modify existing `Scrubber` — Masker wraps it
  - Do NOT log original PII values — only log category + placeholder
  - Do NOT store mapping in database — pass in-memory through workflow context

  **Recommended Agent Profile**:
  - **Category**: `deep`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 1 (with T1-T5)
  - **Blocks**: T8
  - **Blocked By**: None

  **References**:
  - `bridge/pkg/pii/scrubber.go:1-50` — Existing Scrubber to wrap; `Detect()` method returns PII spans
  - `bridge/pkg/pii/resolver.go` — BlindFill engine resolves from profiles; Masker generates the placeholders Resolver consumes
  - `bridge/pkg/pii/scrubber.go:27-34` — `Redaction` struct has Type, Start, End, Original, Replacement — reuse for span detection

  **Acceptance Criteria**:
  - [ ] `Mask()` detects email addresses and replaces with `{{VAULT:email:...}}`
  - [ ] `Mask()` detects SSNs and replaces with `{{VAULT:ssn:...}}`
  - [ ] `Mask()` detects credit cards and replaces with `{{VAULT:credit_card:...}}`
  - [ ] `ResolvePlaceholders()` restores original values
  - [ ] `go test ./bridge/pkg/pii/... -run TestMasker` passes

  **QA Scenarios**:
  ```
  Scenario: Mask email body with PII
    Tool: Bash
    Steps:
      1. Create Masker
      2. Call Mask(ctx, "Contact john@example.com or call 555-123-4567. SSN: 123-45-6789")
      3. Assert: MaskedText contains "{{VAULT:email:" but NOT "john@example.com"
      4. Assert: MaskedText contains "{{VAULT:phone:" but NOT "555-123-4567"
      5. Assert: MaskedText contains "{{VAULT:ssn:" but NOT "123-45-6789"
      6. Assert: len(Mapping) == 3
      7. Assert: DetectedFields contains "email", "phone", "ssn"
    Expected Result: All PII replaced with vault placeholders
    Evidence: .sisyphus/evidence/task-6-masker-pii.txt

  Scenario: Resolve placeholders back to originals
    Tool: Bash
    Steps:
      1. Mask text from scenario above
      2. Call ResolvePlaceholders(ctx, result.MaskedText, result.Mapping)
      3. Assert: resolved text == original text
    Expected Result: Perfect round-trip restoration
    Evidence: .sisyphus/evidence/task-6-masker-resolve.txt

  Scenario: No PII in text returns unchanged
    Tool: Bash
    Steps:
      1. Call Mask(ctx, "Hello, how are you today?")
      2. Assert: MaskedText == original text
      3. Assert: len(Mapping) == 0
    Expected Result: No masking applied when no PII detected
    Evidence: .sisyphus/evidence/task-6-masker-noop.txt
  ```

  **Commit**: YES (groups with Wave 1)

- [x] 7. Postfix Pipe Handler (`cmd/mta-recv`)

  **What to do**:
  - Create `cmd/mta-recv/main.go` — the binary Postfix pipes raw MIME to
  - Read raw MIME from `os.Stdin` (`io.ReadAll(os.Stdin)`)
  - Parse MIME headers using T5 MIME parser
  - Forward parsed email to Bridge IngestServer via gRPC (`unix:///run/armorclaw/email-ingest.sock`)
  - Exit codes: 0 (EX_OK), 75 (EX_TEMPFAIL — Postfix retries), 67 (EX_NOINPUT — permanent reject)
  - Time limit: 300s (Postfix default), configurable via env var
  - Minimal binary: no logging to disk, only to stderr (Postfix captures)

  **Must NOT do**:
  - Do NOT talk to anything except Bridge gRPC socket
  - Do NOT write to filesystem (temp files handled by IngestServer)
  - Do NOT hold Postfix delivery longer than 300s

  **Recommended Agent Profile**:
  - **Category**: `unspecified-high`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 2 (with T8-T13)
  - **Blocks**: T19
  - **Blocked By**: T1 (proto), T5 (MIME parser)

  **References**:
  - `cmd/bridge/main.go` — Existing command structure pattern
  - `bridge/pkg/email/proto/` (T1 output) — gRPC client stubs to import
  - `bridge/pkg/email/mime_parser.go` (T5 output) — ParseMIME function to call
  - CTO memo Phase 1 section — Exact exit code semantics and pipe(8) behavior

  **Acceptance Criteria**:
  - [ ] Binary builds: `go build -o armorclaw-mta-recv ./cmd/mta-recv/`
  - [ ] Reads from stdin, sends gRPC to Bridge
  - [ ] Exits 0 on success, 75 on Bridge unreachable, 67 on malformed MIME
  - [ ] `go test ./cmd/mta-recv/...` passes

  **QA Scenarios**:
  ```
  Scenario: Pipe handler processes valid email
    Tool: Bash
    Steps:
      1. Start mock IngestServer on test socket
      2. Pipe raw email: echo "From: a@b.com\nTo: c@d.com\nSubject: Test\n\nBody" | ./armorclaw-mta-recv
      3. Assert: exit code 0
      4. Assert: mock server received IngestEmail RPC with correct From/To/Subject
    Expected Result: Email forwarded to Bridge, exit 0
    Evidence: .sisyphus/evidence/task-7-pipe-success.txt

  Scenario: Bridge unreachable triggers retry
    Tool: Bash
    Steps:
      1. Do NOT start mock server
      2. Pipe raw email: echo "From: a@b.com\n..." | ./armorclaw-mta-recv
      3. Assert: exit code 75
    Expected Result: Temp fail so Postfix retries
    Evidence: .sisyphus/evidence/task-7-pipe-retry.txt

  Scenario: Malformed MIME triggers permanent reject
    Tool: Bash
    Steps:
      1. Pipe garbage: echo "NOT VALID MIME {{{" | ./armorclaw-mta-recv
      2. Assert: exit code 67
    Expected Result: Permanent reject, no retry
    Evidence: .sisyphus/evidence/task-7-pipe-reject.txt
  ```

  **Commit**: YES
  - Message: `feat(email): add Postfix pipe handler for inbound email ingestion`
  - Files: `cmd/mta-recv/main.go`, `cmd/mta-recv/main_test.go`

- [x] 8. Email IngestServer (YARA + PII Masking + Storage)

  **What to do**:
  - Create `bridge/pkg/email/ingest_server.go` implementing `EmailIngestServiceServer` from proto
  - `IngestEmail()` method:
    1. For each attachment: write to temp file → YARA scan → clean up temp → if clean, extract via sidecar → store to local FS
    2. Mask email body PII using Masker from T6
    3. Publish `EmailReceivedEvent` to EventBus
    4. Return `{Accepted: true, FileIDs: [...]}`
  - Reject entire email if any attachment fails YARA (fail-closed)
  - Audit log: attachment rejected, email accepted, file IDs stored
  - Listen on Unix socket: `/run/armorclaw/email-ingest.sock`

  **Must NOT do**:
  - Do NOT skip YARA scanning for any attachment
  - Do NOT store unmasked body in any event or log
  - Do NOT pass raw attachment bytes to sidecar before YARA clears them
  - Do NOT modify existing sidecar proto

  **Recommended Agent Profile**:
  - **Category**: `deep`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 2 (with T7, T9-T13)
  - **Blocks**: T12, T14, T17, T19
  - **Blocked By**: T1 (proto), T2 (events), T3 (storage), T6 (masker)

  **References**:
  - `bridge/pkg/yara/scanner.go:45-69` — `ScanFileForMalware(filePath string) (bool, error)` to call
  - `bridge/pkg/sidecar/office_client.go` — Sidecar client for text extraction of attachments
  - `bridge/pkg/pii/masker.go` (T6 output) — `Mask(ctx, text)` to mask email body
  - `bridge/pkg/eventbus/eventbus.go` — EventBus.Publish() to emit EmailReceivedEvent
  - `bridge/pkg/email/email_storage.go` (T3 output) — StoreProcessedFile()
  - CTO memo Phase 1 section — Exact flow: temp file → YARA → cleanup → sidecar → store

  **Acceptance Criteria**:
  - [ ] `IngestEmail()` processes attachments through YARA → sidecar → storage
  - [ ] Email body is PII-masked before event publication
  - [ ] YARA match rejects entire email with audit log
  - [ ] `go test ./bridge/pkg/email/... -run TestIngestServer` passes

  **QA Scenarios**:
  ```
  Scenario: Ingest valid email with attachment
    Tool: Bash
    Steps:
      1. Create IngestServer with mock YARA (returns clean), mock sidecar, mock storage
      2. Send IngestEmail RPC with From/To/Subject/Body + 1 attachment
      3. Assert: response.Accepted == true
      4. Assert: len(response.FileIDs) == 1
      5. Assert: EventBus received EmailReceivedEvent with BodyMasked (NOT raw body)
    Expected Result: Email ingested, attachment stored, event published
    Evidence: .sisyphus/evidence/task-8-ingest-valid.txt

  Scenario: YARA match rejects email
    Tool: Bash
    Steps:
      1. Create IngestServer with mock YARA (returns match)
      2. Send IngestEmail RPC with 1 attachment
      3. Assert: response is error containing "YARA match"
      4. Assert: audit log entry created for rejection
    Expected Result: Email rejected, no files stored
    Evidence: .sisyphus/evidence/task-8-ingest-yara.txt

  Scenario: PII in body is masked in event
    Tool: Bash
    Steps:
      1. Send email with body "Contact john@example.com for details"
      2. Assert: EmailReceivedEvent.BodyMasked contains "{{VAULT:email:" but NOT "john@example.com"
    Expected Result: PII never reaches event
    Evidence: .sisyphus/evidence/task-8-ingest-mask.txt
  ```

  **Commit**: YES
  - Message: `feat(email): add email ingestion server with YARA scanning`
  - Files: `bridge/pkg/email/ingest_server.go`, `bridge/pkg/email/ingest_server_test.go`

- [x] 9. Gmail API Client

  **What to do**:
  - Create `bridge/pkg/email/gmail_client.go` with `GmailClient` struct
  - Use `google.golang.org/api/gmail/v1` + `google.golang.org/api/option` official libraries
  - `Send(ctx, refreshToken, to, subject, body) → (messageID, error)`:
    1. Build `gmail.Message` with RFC2822 content
    2. Use refresh token to get access token via OAuth2 config
    3. Call `gmail.Service.Users.Messages.Send("me", msg).Do()`
  - Support both text/plain and text/html bodies
  - Handle: OAuth token refresh, rate limiting, quota errors
  - Provider interface: implement `EmailSender` interface (defined in T2 types)

  **Must NOT do**:
  - Do NOT store Gmail credentials — receive refresh token at call time
  - Do NOT add Gmail-specific logic outside this file
  - Do NOT bypass OAuth2 flow

  **Recommended Agent Profile**:
  - **Category**: `unspecified-high`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 2 (with T7-T8, T10-T13)
  - **Blocks**: T14, T16
  - **Blocked By**: T1 (proto/types)

  **References**:
  - `bridge/internal/skills/email_send.go:347-379` — `buildEmailContent()` for RFC2822 construction pattern
  - `bridge/pkg/keystore/keystore.go:33-37` — XChaCha20-Poly1305 + PBKDF2 encryption pattern to follow for token handling
  - Gmail API docs: `google.golang.org/api/gmail/v1` — official Go client

  **Acceptance Criteria**:
  - [ ] `GmailClient` implements `EmailSender` interface
  - [ ] `Send()` builds valid RFC2822 message
  - [ ] OAuth2 refresh token flow works
  - [ ] `go test ./bridge/pkg/email/... -run TestGmailClient` passes

  **QA Scenarios**:
  ```
  Scenario: Gmail client builds valid message
    Tool: Bash
    Steps:
      1. Create GmailClient with mock HTTP server (returns 200)
      2. Call Send(ctx, refreshToken, "to@example.com", "Subject", "Body text")
      3. Assert: HTTP request made to gmail/v1/users/me/messages/send
      4. Assert: message ID returned
    Expected Result: Valid Gmail API call
    Evidence: .sisyphus/evidence/task-9-gmail-send.txt

  Scenario: OAuth token refresh handled
    Tool: Bash
    Steps:
      1. Mock OAuth2 token endpoint to return access token
      2. Call Send with refresh token
      3. Assert: access token requested before API call
    Expected Result: Transparent token refresh
    Evidence: .sisyphus/evidence/task-9-gmail-oauth.txt
  ```

  **Commit**: YES (groups with T10)
  - Message: `feat(email): add Gmail API and SMTP email clients`

- [x] 10. SMTP Fallback Client

  **What to do**:
  - Create `bridge/pkg/email/smtp_client.go` with `SMTPClient` struct
  - Use Go stdlib `net/smtp` — no external deps
  - `Send(ctx, smtpConfig, to, subject, body) → (messageID, error)`
  - Support STARTTLS, PLAIN auth
  - Support XOAUTH2 for Gmail SMTP fallback (optional)
  - Implement `EmailSender` interface from T2
  - Connection pooling for high-frequency sends

  **Must NOT do**:
  - Do NOT add external SMTP libraries
  - Do NOT store SMTP credentials in this client

  **Recommended Agent Profile**:
  - **Category**: `unspecified-high`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 2 (with T7-T9, T11-T13)
  - **Blocks**: T14, T16
  - **Blocked By**: T1 (types)

  **References**:
  - `bridge/internal/skills/email_send.go:245-344` — Existing SMTP mock implementation; the real SMTP logic in comments (lines 287-340) is the exact pattern to follow
  - Go stdlib `net/smtp` — standard SMTP client

  **Acceptance Criteria**:
  - [ ] `SMTPClient` implements `EmailSender` interface
  - [ ] Supports STARTTLS
  - [ ] `go test ./bridge/pkg/email/... -run TestSMTPClient` passes

  **QA Scenarios**:
  ```
  Scenario: SMTP client sends via mock server
    Tool: Bash
    Steps:
      1. Start mock SMTP server on localhost
      2. Call Send(ctx, config, "to@example.com", "Subject", "Body")
      3. Assert: mock server received MAIL FROM, RCPT TO, DATA
    Expected Result: Email delivered via SMTP
    Evidence: .sisyphus/evidence/task-10-smtp-send.txt

  Scenario: SMTP connection failure returns error
    Tool: Bash
    Steps:
      1. Do NOT start SMTP server
      2. Call Send with config pointing to localhost:9999
      3. Assert: error returned (connection refused)
    Expected Result: Graceful error handling
    Evidence: .sisyphus/evidence/task-10-smtp-fail.txt
  ```

  **Commit**: YES (groups with T9)

- [x] 11. OAuth Token Storage in SQLCipher

  **What to do**:
  - Create `bridge/pkg/keystore/oauth.go` extending the existing keystore
  - SQL migration: `CREATE TABLE oauth_tokens` (id TEXT PK, provider TEXT, account_email TEXT, refresh_token_encrypted BLOB, refresh_token_nonce BLOB, scopes TEXT, created_at INTEGER, last_refreshed_at INTEGER, status TEXT DEFAULT 'active')
  - `StoreOAuthToken(ctx, provider, email, refreshToken) → (id, error)` — encrypt with XChaCha20-Poly1305 using keystore master key
  - `GetOAuthRefreshToken(ctx, provider) → (*OAuthTokenRecord, error)` — decrypt and return
  - `RevokeOAuthToken(ctx, id) → error` — set status='revoked'
  - `ListOAuthTokens(ctx) → ([]OAuthTokenInfo, error)` — metadata only, no decrypted values
  - Add migration to `bridge/pkg/keystore/migrations/`

  **Must NOT do**:
  - Do NOT store refresh tokens in plaintext
  - Do NOT return decrypted tokens in list operations
  - Do NOT modify existing keystore tables or encryption logic
  - Do NOT create a separate database — use the existing SQLCipher DB

  **Recommended Agent Profile**:
  - **Category**: `deep`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 1 (with T1-T6)
  - **Blocks**: T14, T16
  - **Blocked By**: None (no compilation dependency on T3 — uses same SQLCipher DB but standalone table)

  **References**:
  - `bridge/pkg/keystore/keystore.go:33-37` — XChaCha20-Poly1305 encryption + PBKDF2 key derivation to reuse
  - `bridge/pkg/keystore/keystore.go:1590` — End of file; add OAuth methods here
  - `bridge/pkg/keystore/keystore.proto:216-225` — Credential message pattern to follow for OAuthTokenRecord
  - CTO memo Phase 4 section — Exact SQL schema and encryption requirements

  **Acceptance Criteria**:
  - [ ] `oauth_tokens` table created in SQLCipher
  - [ ] Tokens encrypted with XChaCha20-Poly1305
  - [ ] `GetOAuthRefreshToken()` decrypts and returns valid token
  - [ ] `go test ./bridge/pkg/keystore/... -run TestOAuth` passes

  **QA Scenarios**:
  ```
  Scenario: Store and retrieve OAuth token
    Tool: Bash
    Steps:
      1. StoreOAuthToken(ctx, "gmail", "user@gmail.com", "refresh-token-abc")
      2. Assert: ID returned
      3. GetOAuthRefreshToken(ctx, "gmail")
      4. Assert: RefreshToken == "refresh-token-abc"
      5. Assert: AccountEmail == "user@gmail.com"
    Expected Result: Round-trip store/decrypt/retrieve
    Evidence: .sisyphus/evidence/task-11-oauth-roundtrip.txt

  Scenario: Token is encrypted at rest
    Tool: Bash
    Steps:
      1. Store a token
      2. Query SQLCipher directly for refresh_token_encrypted
      3. Assert: raw bytes != "refresh-token-abc"
    Expected Result: Token not readable without decryption
    Evidence: .sisyphus/evidence/task-11-oauth-encrypted.txt

  Scenario: List returns metadata without decrypted values
    Tool: Bash
    Steps:
      1. Store 2 tokens
      2. ListOAuthTokens(ctx)
      3. Assert: len == 2
      4. Assert: no RefreshToken field populated (metadata only)
    Expected Result: Safe metadata listing
    Evidence: .sisyphus/evidence/task-11-oauth-list.txt
  ```

  **Commit**: YES
  - Message: `feat(keystore): add OAuth token storage with XChaCha20-Poly1305`
  - Files: `bridge/pkg/keystore/oauth.go`, `bridge/pkg/keystore/oauth_test.go`

- [x] 12. Email Dispatcher (Event → Workflow)

  **What to do**:
  - Create `bridge/pkg/email/dispatcher.go` with `EmailDispatcher`
  - `Start(ctx, bus)` — subscribes to `email.received` events via EventBus
  - `dispatchToWorkflow(ctx, event)` — looks up workflow template by trigger key `"email:" + event.To`, creates `ScheduledTask` with `OneShot: true`, calls `scheduler.DispatchNow()`
  - Template variables: `email_from`, `email_subject`, `email_body` (masked), `email_files`
  - Handle unmapped addresses gracefully (log + drop)
  - Concurrency: one goroutine per dispatch (non-blocking subscriber)

  **Must NOT do**:
  - Do NOT dispatch to workflows for unmapped email addresses
  - Do NOT block EventBus on dispatch failures
  - Do NOT modify existing scheduler/secretary code

  **Recommended Agent Profile**:
  - **Category**: `unspecified-high`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 2 (with T7-T11, T13)
  - **Blocks**: T14, T19
  - **Blocked By**: T2 (events), T8 (ingest server publishes events)

  **References**:
  - `bridge/pkg/eventbus/eventbus.go:150-200` — `Subscribe(filter)` returns `Subscriber` with `Ch()` channel — exact pattern to use
  - `bridge/pkg/secretary/task_dispatch.go` — `DispatchNow()` and `ScheduledTask` types
  - `bridge/pkg/secretary/types.go:168-195` — `ScheduledTask` struct with TemplateID, DefinitionID, Variables
  - CTO memo Phase 2 section — Exact dispatch flow

  **Acceptance Criteria**:
  - [ ] Dispatcher subscribes to `email.received` events
  - [ ] Looks up template by `"email:" + recipientAddress`
  - [ ] Creates ScheduledTask with email variables and dispatches immediately
  - [ ] Unmapped addresses logged and dropped
  - [ ] `go test ./bridge/pkg/email/... -run TestEmailDispatcher` passes

  **QA Scenarios**:
  ```
  Scenario: Dispatch creates workflow for mapped email
    Tool: Bash
    Steps:
      1. Create dispatcher with mock template store (returns template for "email:secretary@example.com")
      2. Publish EmailReceivedEvent with To: "secretary@example.com"
      3. Assert: DispatchNow called with correct template ID
      4. Assert: Variables contain email_from, email_subject, email_body
    Expected Result: Workflow dispatched
    Evidence: .sisyphus/evidence/task-12-dispatch-mapped.txt

  Scenario: Unmapped email is dropped
    Tool: Bash
    Steps:
      1. Create dispatcher with empty template store
      2. Publish EmailReceivedEvent with To: "unknown@example.com"
      3. Assert: no workflow created
      4. Assert: log contains "unmapped" or "no template"
    Expected Result: Graceful drop without error
    Evidence: .sisyphus/evidence/task-12-dispatch-unmapped.txt
  ```

  **Commit**: YES (groups with T13)
  - Message: `feat(email): add email dispatcher and workflow template`

- [x] 13. Email Workflow Template Setup

  **What to do**:
  - Create `bridge/pkg/email/email_template.go` with `CreateEmailWorkflowTemplate() → (*TaskTemplate, error)`
  - Define 2-step workflow template:
    - Step 1 (`step_1_analyze`): `type: "action"`, agent analyzes email, writes draft to `result.json` with `draft_to`, `draft_subject`, `draft_body`
    - Step 2 (`step_2_send`): `type: "action"`, bridge-local execution — validates agent output, runs policy, HITL, sends email
  - PII refs: `["email.from", "email.body", "email.subject"]`
  - Template variables schema: `email_from`, `email_subject`, `email_body` (masked), `email_files` (file IDs)
  - Register template on Bridge startup

  **Must NOT do**:
  - Do NOT hardcode agent ID — make it configurable via template variables
  - Do NOT modify existing template system

  **Recommended Agent Profile**:
  - **Category**: `quick`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 2 (with T7-T12)
  - **Blocks**: T13.5, T19
  - **Blocked By**: T2 (types), T3.5 (schema migrations)

  **References**:
  - `bridge/pkg/secretary/types.go:30-60` — `TaskTemplate` struct with Steps, Variables, PIIRefs
  - `bridge/pkg/secretary/types.go:63-101` — `WorkflowStep` struct with StepID, Order, Type, Config
  - `bridge/pkg/secretary/schema.sql:10-21` — `task_templates` table schema

  **Acceptance Criteria**:
  - [ ] Template has 2 steps (analyze + send)
  - [ ] Step 2 config marks it as bridge-local execution
  - [ ] Template can be stored and retrieved from secretary store

  **QA Scenarios**:
  ```
  Scenario: Template creates and validates
    Tool: Bash
    Steps:
      1. Call CreateEmailWorkflowTemplate()
      2. Assert: len(Steps) == 2
      3. Assert: Steps[0].StepID == "step_1_analyze"
      4. Assert: Steps[1].StepID == "step_2_send"
      5. Assert: PIIRefs contains "email.body"
    Expected Result: Valid 2-step email template
    Evidence: .sisyphus/evidence/task-13-template.txt
  ```

  **Commit**: YES (groups with T12)

- [x] 13.5 Bridge-Local Execution Mode for Step Executor

  **What to do**:
  - Add `ExecutionModeBridgeLocal` constant to `bridge/pkg/secretary/orchestrator_integration.go`
  - Modify `executeStep()` in the orchestrator to branch on execution mode:
    - `ExecutionModeContainer` (existing): spawn container, poll, read result.json
    - `ExecutionModeBridgeLocal` (new): route to a handler registry instead of `factory.Spawn()`
  - Create `bridge/pkg/secretary/bridge_local_registry.go` with `BridgeLocalHandler` interface and registry:
    - `RegisterHandler(stepType string, handler BridgeLocalHandler)`
    - `Execute(ctx, step, workflowCtx) → (*StepResult, error)`
  - Register email send handler (`step_2_send`) in the registry during Bridge startup
  - Step config includes `execution_mode: "bridge_local"` field

  **Must NOT do**:
  - Do NOT modify existing container execution path
  - Do NOT add HTTP/FastAPI — handler registry is internal Go only
  - Do NOT break existing workflow execution

  **Recommended Agent Profile**:
  - **Category**: `deep`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 3 (with T14-T18)
  - **Blocks**: T14
  - **Blocked By**: T13 (template defines step_2_send with bridge_local config)

  **References**:
  - `bridge/pkg/secretary/orchestrator_integration.go` — Step execution logic where branching is added
  - `bridge/pkg/secretary/orchestrator_integration_test.go` — Existing tests to preserve
  - `bridge/pkg/secretary/types.go:93-101` — StepType constants where ExecutionModeBridgeLocal is added
  - `bridge/pkg/secretary/types.go:63-90` — WorkflowStep struct where execution_mode config goes

  **Acceptance Criteria**:
  - [ ] `ExecutionModeBridgeLocal` constant defined
  - [ ] `executeStep()` branches on execution mode
  - [ ] `BridgeLocalHandler` interface and registry implemented
  - [ ] Container execution path unchanged (existing tests pass)
  - [ ] `go test ./bridge/pkg/secretary/...` passes

  **QA Scenarios**:
  ```
  Scenario: Bridge-local handler invoked instead of container spawn
    Tool: Bash
    Steps:
      1. Create orchestrator with mock bridge-local handler registered for "test_action"
      2. Create workflow step with execution_mode: "bridge_local", type: "test_action"
      3. Execute step
      4. Assert: mock handler called (NOT factory.Spawn)
      5. Assert: StepResult from handler returned
    Expected Result: Bridge-local routing works
    Evidence: .sisyphus/evidence/task-13.5-bridge-local.txt

  Scenario: Container execution path unchanged
    Tool: Bash
    Steps:
      1. Create orchestrator WITHOUT bridge-local handler
      2. Create workflow step with NO execution_mode (default container)
      3. Execute step
      4. Assert: container spawn attempted (existing behavior)
    Expected Result: No regression in container execution
    Evidence: .sisyphus/evidence/task-13.5-container-unchanged.txt

  Scenario: Secretary regression suite passes
    Tool: Bash
    Steps:
      1. Run: cd bridge && go test ./pkg/secretary/...
      2. Assert: all PASS
    Expected Result: No regressions
    Evidence: .sisyphus/evidence/task-13.5-regression.txt
  ```

  **Commit**: YES
  - Message: `feat(secretary): add bridge-local execution mode for step executor`

- [x] 14. Bridge-Local Outbound Executor

  **What to do**:
  - Create `bridge/pkg/email/outbound_executor.go` with `OutboundExecutor`
  - `ExecuteEmailSend(ctx, step, workflowCtx) → (*StepResult, error)`:
    1. Extract previous step result from `step_1_analyze`: `prevResult.Data["draft_to"]`, `draft_subject`, `draft_body`
    2. Strict validation: all 3 fields must be non-empty strings, `draft_to` must be valid email
    3. Policy evaluation via ApprovalEngine: `EvaluateStep()` with email PII fields (wraps existing engine with email-specific conditions like recipient domain check)
    4. If `NeedsApproval`: send Matrix HITL event, block until response or configurable timeout (default 300s, matching legal document review time)
    5. If approved (or no approval needed): resolve PII placeholders via Masker.ResolvePlaceholders()
    6. Get OAuth refresh token from keystore
    7. Send via GmailClient (primary) or SMTPClient (fallback)
    8. Return success/failure StepResult
  - Register as step executor for `step_1_send` in the orchestrator

  **Must NOT do**:
  - Do NOT auto-send if policy requires approval and no response received
  - Do NOT pass refresh token to agent — resolved server-side only
  - Do NOT skip validation even if policy auto-approves
  - Do NOT allow agent output with empty draft_to/draft_body through

  **Recommended Agent Profile**:
  - **Category**: `deep`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 3 (with T15-T18)
  - **Blocks**: T15, T17, T19
  - **Blocked By**: T8 (ingest), T9 (Gmail), T10 (SMTP), T11 (OAuth), T12 (dispatcher)

  **References**:
  - `bridge/pkg/secretary/approvals.go:162-174` — `EvaluateStep()` to call for policy check
  - `bridge/pkg/secretary/approvals.go:289-360` — Condition evaluation engine already supports workflow/template/step context
  - `bridge/pkg/pii/masker.go` (T6 output) — `ResolvePlaceholders()` for BlindFill resolution
  - `bridge/pkg/keystore/oauth.go` (T11 output) — `GetOAuthRefreshToken()` for OAuth
  - `bridge/internal/skills/email_send.go:56-77` — Existing email validation pattern to reuse
  - CTO memo Phase 3 section — Exact 5-step outbound flow

  **Acceptance Criteria**:
  - [ ] Extracts draft_to, draft_subject, draft_body from previous step
  - [ ] Validates all fields non-empty, draft_to valid email
  - [ ] Calls ApprovalEngine.EvaluateStep()
  - [ ] Blocks for Matrix HITL approval when policy requires it
  - [ ] Resolves PII placeholders before sending
  - [ ] Sends via GmailClient with OAuth token
  - [ ] `go test ./bridge/pkg/email/... -run TestOutboundExecutor` passes

  **QA Scenarios**:
  ```
  Scenario: Valid agent output triggers send
    Tool: Bash
    Steps:
      1. Create executor with mock approval (auto-approve), mock Gmail, mock keystore
      2. Call ExecuteEmailSend with step result containing draft_to="partner@example.com", draft_subject="Re: Legal", draft_body="We agree to terms"
      3. Assert: GmailClient.Send() called with correct to/subject/body
      4. Assert: StepResult.Status == "success"
    Expected Result: Email sent via Gmail
    Evidence: .sisyphus/evidence/task-14-outbound-valid.txt

  Scenario: Missing draft_to rejects
    Tool: Bash
    Steps:
      1. Call ExecuteEmailSend with step result missing draft_to
      2. Assert: error returned containing "draft_to"
    Expected Result: Validation failure, no send
    Evidence: .sisyphus/evidence/task-14-outbound-missing.txt

  Scenario: HITL approval required and granted
    Tool: Bash
    Steps:
      1. Create executor with policy requiring approval
      2. Start ExecuteEmailSend in goroutine
      3. Simulate Matrix approval response after 1s
      4. Assert: GmailClient.Send() called after approval
    Expected Result: Send blocked until approved, then proceeds
    Evidence: .sisyphus/evidence/task-14-outbound-hitl-approve.txt

  Scenario: HITL approval timeout auto-rejects
    Tool: Bash
    Steps:
      1. Create executor with policy requiring approval, 5s timeout
      2. Call ExecuteEmailSend — do NOT send approval
      3. Wait 6s
      4. Assert: StepResult.Status == "failed", Error contains "timeout" or "rejected"
    Expected Result: Auto-reject on timeout
    Evidence: .sisyphus/evidence/task-14-outbound-hitl-timeout.txt
  ```

  **Commit**: YES (groups with T15)
  - Message: `feat(email): add outbound executor with HITL approval`

- [x] 15. HITL Email Approval Matrix Integration

  **What to do**:
  - Create `bridge/pkg/email/hitl_approval.go` with `EmailApprovalManager`
  - `RequestApproval(ctx, eventBus, roomID, stepID, emailDraft) → (<-chan ApprovalResponse, error)`:
    1. Build Matrix event content: `app.armorclaw.email_approval_request` with step_id, to, subject, body_preview (truncated to 8 lines), timestamp, expires_at
    2. Publish as transient event (NOT state event — CTO: prevents race conditions)
    3. Subscribe to `app.armorclaw.email_approval_response` events for this step_id
    4. Return channel that receives approval or times out (default 300s, configurable via `ARMORCLAW_EMAIL_HITL_TIMEOUT` env var)
  - `PendingEmailApproval()` — blocking wrapper used by outbound executor
  - Approval response parsing: `approved: bool`, `step_id: string`
  - Cleanup: unsubscribe on approval/rejection/timeout

  **Must NOT do**:
  - Do NOT use `sendStateEvent` — transient events only per CTO
  - Do NOT store approval status in persistent room state
  - Do NOT expose full email body in approval request — preview only (8 lines max)
  - Do NOT bypass approval flow for any reason

  **Recommended Agent Profile**:
  - **Category**: `unspecified-high`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 3 (with T14, T16-T18)
  - **Blocks**: T18, T19
  - **Blocked By**: T2 (events), T14 (outbound executor)

  **References**:
  - `bridge/pkg/eventbus/eventbus.go` — EventBus for publishing/subscribing to Matrix events
  - `bridge/pkg/eventbus/events.go:36-40` — Existing HITL event types (pending, approved, rejected, expired) to follow
  - `bridge/pkg/secretary/approvals.go:367-398` — Approval request creation pattern
  - `applications/ArmorChat/app/src/main/java/app/armorclaw/ui/components/PiiApprovalCard.kt` — Android approval UI pattern (for understanding what the Android team will build)
  - CTO memo Phase 5 section — Transient event format and biometric flow

  **Acceptance Criteria**:
  - [ ] Publishes `app.armorclaw.email_approval_request` transient Matrix event
  - [ ] Subscribes to matching response events by step_id
  - [ ] Configurable timeout (default 300s) auto-rejects
  - [ ] `go test ./bridge/pkg/email/... -run TestEmailHITL` passes

  **QA Scenarios**:
  ```
  Scenario: Approval request published and response received
    Tool: Bash
    Steps:
      1. Create manager with mock EventBus
      2. Call RequestApproval with test draft
      3. Assert: event published with type "app.armorclaw.email_approval_request"
      4. Assert: body_preview truncated to 8 lines
      5. Simulate approval response event
      6. Assert: channel receives approved=true
    Expected Result: End-to-end approval flow
    Evidence: .sisyphus/evidence/task-15-hitl-flow.txt

  Scenario: Timeout auto-rejects
    Tool: Bash
    Steps:
      1. Call RequestApproval with 2s timeout
      2. Do NOT send response
      3. Wait 3s
      4. Assert: channel receives approved=false
    Expected Result: Auto-reject after timeout
    Evidence: .sisyphus/evidence/task-15-hitl-timeout.txt
  ```

  **Commit**: YES (groups with T14)

- [x] 16. Outlook Provider Adapter

  **What to do**:
  - Create `bridge/pkg/email/outlook_client.go` with `OutlookClient`
  - Use Microsoft Graph API (`graph.microsoft.com/v1.0/me/sendMail`)
  - OAuth2 flow: same `GetOAuthRefreshToken(ctx, "outlook")` from keystore
  - Implement `EmailSender` interface
  - Send mail: `POST /users/{userId}/sendMail` with `message` body
  - Support text/plain and text/html
  - Handle: rate limiting (429), token refresh, Graph API error format

  **Must NOT do**:
  - Do NOT add Microsoft-specific SDK — use plain HTTP client with OAuth2
  - Do NOT duplicate validation logic from Gmail client — share through EmailSender interface

  **Recommended Agent Profile**:
  - **Category**: `unspecified-high`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 3 (with T14-T15, T17-T18)
  - **Blocks**: T19
  - **Blocked By**: T9 (Gmail client pattern), T10 (SMTP pattern), T11 (OAuth keystore)

  **References**:
  - `bridge/pkg/email/gmail_client.go` (T9 output) — Same EmailSender interface to implement
  - `bridge/pkg/keystore/oauth.go` (T11 output) — `GetOAuthRefreshToken(ctx, "outlook")`
  - Microsoft Graph API docs: `POST /v1.0/me/sendMail`

  **Acceptance Criteria**:
  - [ ] `OutlookClient` implements `EmailSender` interface
  - [ ] Sends via Microsoft Graph API
  - [ ] OAuth2 refresh token from keystore
  - [ ] `go test ./bridge/pkg/email/... -run TestOutlookClient` passes

  **QA Scenarios**:
  ```
  Scenario: Outlook client sends via Graph API
    Tool: Bash
    Steps:
      1. Create OutlookClient with mock HTTP server
      2. Call Send(ctx, refreshToken, "to@example.com", "Subject", "Body")
      3. Assert: POST to graph.microsoft.com/v1.0/me/sendMail
      4. Assert: Authorization header present
    Expected Result: Valid Graph API call
    Evidence: .sisyphus/evidence/task-16-outlook-send.txt

  Scenario: Rate limit (429) handled gracefully
    Tool: Bash
    Steps:
      1. Mock Graph API returns 429 with Retry-After header
      2. Call Send
      3. Assert: error indicates rate limited, includes retry info
    Expected Result: Graceful rate limit handling
    Evidence: .sisyphus/evidence/task-16-outlook-ratelimit.txt
  ```

  **Commit**: YES
  - Message: `feat(email): add Outlook provider adapter`

- [x] 17. Email Audit Logging

  **What to do**:
  - Create `bridge/pkg/email/audit.go` with email-specific audit functions
  - `LogEmailAccepted(ctx, from, to, subject, fileIDs)` — email ingested successfully
  - `LogEmailRejected(ctx, from, to, reason)` — YARA match, malformed, etc.
  - `LogEmailSent(ctx, to, subject, provider, messageID)` — outbound sent
  - `LogEmailSendFailed(ctx, to, subject, error)` — outbound failed
  - `LogHITLApprovalRequested(ctx, stepID, to, subject)` — approval requested
  - `LogHITLApprovalResult(ctx, stepID, approved, by)` — approval result
  - Use existing `bridge/pkg/audit/` infrastructure
  - Never log PII values — only masked/placeholder versions

  **Must NOT do**:
  - Do NOT log unmasked email body or PII values
  - Do NOT log OAuth tokens
  - Do NOT create a separate audit database

  **Recommended Agent Profile**:
  - **Category**: `quick`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 3 (with T14-T16, T18)
  - **Blocks**: T19
  - **Blocked By**: T8 (ingest events), T14 (outbound events)

  **References**:
  - `bridge/pkg/audit/` — Existing audit infrastructure
  - `bridge/pkg/logger/security_test.go` — Security logging patterns

  **Acceptance Criteria**:
  - [ ] All 6 audit functions log structured entries
  - [ ] No PII values in any log entry
  - [ ] `go test ./bridge/pkg/email/... -run TestEmailAudit` passes

  **QA Scenarios**:
  ```
  Scenario: Audit log contains no PII
    Tool: Bash
    Steps:
      1. Call LogEmailAccepted with subject containing PII
      2. Read audit log
      3. Assert: no raw email addresses in log (only masked)
    Expected Result: PII-safe audit trail
    Evidence: .sisyphus/evidence/task-17-audit-pii-safe.txt
  ```

  **Commit**: YES
  - Message: `feat(email): add audit logging for all email operations`

- [x] 18. Android Integration Specification

  **What to do**:
  - Create `doc/email-android-integration.md` — comprehensive spec for Android team
  - Document the Matrix event format for `app.armorclaw.email_approval_request`:
    ```json
    {
      "type": "app.armorclaw.email_approval_request",
      "content": {
        "step_id": "step_1_send_abc123",
        "to": "partner@lawfirm.com",
        "subject": "Re: Contract Review",
        "body_preview": "Dear Partner,\n\nWe have reviewed the contract...",
        "timestamp": 1713280000,
        "expires_at": 1713280060
      }
    }
    ```
  - Document the expected response event `app.armorclaw.email_approval_response`:
    ```json
    {
      "type": "app.armorclaw.email_approval_response",
      "content": {
        "step_id": "step_1_send_abc123",
        "approved": true
      }
    }
    ```
  - Document: MUST use `sendEvent` (transient), NOT `sendStateEvent`
  - Document: biometric already unlocked at app level (per CTO), no per-button biometric
  - Document: EmailApprovalCard UI spec (from CTO memo Phase 5)
  - Document: Integration with existing `HitlViewModel` pattern
  - Reference existing `PiiApprovalCard.kt` as similar pattern

  **Must NOT do**:
  - Do NOT write Kotlin code — specification only
  - Do NOT modify existing Android code

  **Recommended Agent Profile**:
  - **Category**: `writing`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 3 (with T14-T17)
  - **Blocks**: T21
  - **Blocked By**: T15 (HITL approval — need exact event format)

  **References**:
  - `applications/ArmorChat/app/src/main/java/app/armorclaw/ui/components/PiiApprovalCard.kt` — Existing HITL UI pattern to reference
  - CTO memo Phase 5 — Exact EmailApprovalCard spec and Matrix event format
  - `bridge/pkg/email/hitl_approval.go` (T15 output) — The Go-side event format that Android must match

  **Acceptance Criteria**:
  - [ ] Document specifies both request and response Matrix event schemas
  - [ ] Document specifies transient events (not state events)
  - [ ] Document includes EmailApprovalCard UI spec from CTO memo
  - [ ] Document references existing PiiApprovalCard as similar pattern

  **QA Scenarios**:
  ```
  Scenario: Spec contains all required sections
    Tool: Bash
    Steps:
      1. Read doc/email-android-integration.md
      2. Assert: contains "app.armorclaw.email_approval_request"
      3. Assert: contains "app.armorclaw.email_approval_response"
      4. Assert: contains "sendEvent" and "transient"
      5. Assert: contains "EmailApprovalCard"
      6. Assert: contains "biometric"
    Expected Result: Complete integration spec
    Evidence: .sisyphus/evidence/task-18-android-spec.txt
  ```

  **Commit**: YES
  - Message: `docs(email): add Android integration specification`

- [x] 19. End-to-End Pipeline Test

  **What to do**:
  - Create `bridge/pkg/email/e2e_test.go` with full pipeline integration test
  - Test `TestEmailPipelineE2E`: 
    1. Start IngestServer with mock YARA (clean), mock sidecar, real EventBus, real Storage
    2. Send IngestEmail RPC with a valid email + attachment
    3. Assert: EmailReceivedEvent published to EventBus
    4. Dispatcher picks up event → creates workflow task
    5. Simulate agent Step 1 result: draft_to, draft_subject, draft_body
    6. Outbound executor validates → policy auto-approves → resolves PII → sends via mock Gmail
    7. Assert: email sent with resolved PII (not placeholders)
  - Test `TestEmailPipelineYARAReject`: send email with YARA match → assert rejection
  - Test `TestEmailPipelineHITLReject`: HITL timeout → assert auto-reject
  - Test `TestEmailPipelineOutlookFallback`: Gmail fails → SMTP fallback succeeds

  **Must NOT do**:
  - Do NOT require real Gmail credentials or network access
  - Do NOT require Postfix running
  - Do NOT test agent container runtime — mock step results

  **Recommended Agent Profile**:
  - **Category**: `deep`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: NO
  - **Parallel Group**: Sequential (depends on all prior tasks)
  - **Blocks**: T20, T21, T22
  - **Blocked By**: T7-T17 (all implementation tasks)

  **References**:
  - `bridge/pkg/sidecar/office_client_e2e_test.go` — E2E test pattern from sidecar testing
  - `bridge/pkg/secretary/orchestrator_integration_test.go` — Secretary integration test pattern
  - All T7-T17 outputs — the actual modules under test

  **Acceptance Criteria**:
  - [ ] `TestEmailPipelineE2E` passes: ingest → dispatch → outbound → send
  - [ ] `TestEmailPipelineYARAReject` passes: YARA match → rejection
  - [ ] `TestEmailPipelineHITLReject` passes: timeout → auto-reject
  - [ ] All tests run without network access or external services

  **QA Scenarios**:
  ```
  Scenario: Full pipeline happy path
    Tool: Bash
    Steps:
      1. Run: cd bridge && go test -v -run TestEmailPipelineE2E ./pkg/email/...
      2. Assert: PASS
      3. Assert: output shows "ingest→dispatch→validate→approve→send" flow
    Expected Result: End-to-end pipeline verified
    Evidence: .sisyphus/evidence/task-19-e2e-happy.txt

  Scenario: YARA rejection path
    Tool: Bash
    Steps:
      1. Run: cd bridge && go test -v -run TestEmailPipelineYARAReject ./pkg/email/...
      2. Assert: PASS
    Expected Result: YARA rejection verified
    Evidence: .sisyphus/evidence/task-19-e2e-yara.txt
  ```

  **Commit**: YES (groups with T20)
  - Message: `test(email): add E2E pipeline and edge case tests`

- [x] 20. Edge Case + Security Test Suite

  **What to do**:
  - Create `bridge/pkg/email/edge_cases_test.go`
  - Edge cases to test:
    1. Email with no body, only attachments → extract attachments, empty body
    2. Email with encrypted attachment (password-protected ZIP) → sidecar returns error → graceful handling
    3. Simultaneous emails to same workflow → both dispatched (no serialization)
    4. OAuth token expired mid-send → error propagated, retry possible
    5. Agent produces empty draft_body → validation rejects
    6. Agent produces invalid email in draft_to → validation rejects
    7. Email with 25MB+ attachment → reject (Postfix limit)
    8. Email with no attachments → body-only processing
    9. PII in email subject → subject also masked in event
    10. Concurrent IngestEmail RPCs → no race conditions
  - Security tests:
    1. Verify no unmasked PII in any stored file
    2. Verify no unmasked PII in EventBus events
    3. Verify no unmasked PII in audit logs
    4. Verify OAuth tokens encrypted at rest in SQLCipher
    5. Verify Postfix handler cannot reach non-Bridge sockets

  **Must NOT do**:
  - Do NOT require external services (mock everything)
  - Do NOT test agent container internals

  **Recommended Agent Profile**:
  - **Category**: `unspecified-high`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: NO
  - **Parallel Group**: Sequential (after T19)
  - **Blocks**: None
  - **Blocked By**: T19 (E2E test)

  **References**:
  - `bridge/pkg/sidecar/office_client_e2e_test.go` — E2E test pattern
  - `sidecar-python/test_edge_cases.py` — Edge case test pattern from sidecar testing

  **Acceptance Criteria**:
  - [ ] 10 edge case tests pass
  - [ ] 5 security tests pass
  - [ ] All tests run without network access

  **QA Scenarios**:
  ```
  Scenario: Edge case suite passes
    Tool: Bash
    Steps:
      1. Run: cd bridge && go test -v -run TestEmailEdge ./pkg/email/...
      2. Assert: 10/10 PASS
    Expected Result: All edge cases handled
    Evidence: .sisyphus/evidence/task-20-edge-cases.txt

  Scenario: Security test suite passes
    Tool: Bash
    Steps:
      1. Run: cd bridge && go test -v -run TestEmailSecurity ./pkg/email/...
      2. Assert: 5/5 PASS
    Expected Result: No PII leaks
    Evidence: .sisyphus/evidence/task-20-security.txt
  ```

  **Commit**: YES (groups with T19)

- [x] 21. Pipeline Documentation

  **What to do**:
  - Create `doc/email-pipeline.md` — comprehensive pipeline documentation
  - Sections:
    1. Architecture overview diagram (ASCII)
    2. Phase 1: Inbound flow (Postfix → pipe → MIME → YARA → sidecar → storage → event)
    3. Phase 2: Dispatch flow (event → template lookup → workflow creation)
    4. Phase 3: Outbound flow (agent draft → validation → policy → HITL → BlindFill → send)
    5. Phase 4: Security modules (PII Masker, OAuth storage, provider abstraction)
    6. Configuration reference (Postfix, environment variables, socket paths)
    7. API reference (gRPC proto methods)
    8. Testing guide (how to run tests, mock setup)
  - Update `README.md` with email pipeline section
  - Cross-reference Android integration spec (T18)

  **Must NOT do**:
  - Do NOT duplicate Android integration spec (reference T18 doc)
  - Do NOT include sensitive configuration values

  **Recommended Agent Profile**:
  - **Category**: `writing`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 4 (with T19-T20, T22)
  - **Blocks**: None
  - **Blocked By**: T18 (Android spec reference), T19 (E2E test for accuracy)

  **References**:
  - `doc/sidecar-pipeline.md` — Existing pipeline documentation for style reference
  - `README.md` — Main documentation to update
  - All T1-T17 outputs — source of truth for what was built

  **Acceptance Criteria**:
  - [ ] `doc/email-pipeline.md` exists with 8 sections
  - [ ] README.md updated with email pipeline section
  - [ ] All file paths and command examples are accurate

  **QA Scenarios**:
  ```
  Scenario: Documentation is complete
    Tool: Bash
    Steps:
      1. Assert: doc/email-pipeline.md exists
      2. Assert: contains "Postfix" and "YARA" and "HITL" and "Gmail"
      3. Assert: README.md contains "Email Pipeline" section
    Expected Result: Complete documentation
    Evidence: .sisyphus/evidence/task-21-docs.txt
  ```

  **Commit**: YES
  - Message: `docs(email): add pipeline documentation`

- [x] 22. Postfix + Bridge Integration Verification

  **What to do**:
  - Verify Postfix configuration from T4 works with the actual pipe handler from T7
  - Create `deploy/postfix/verify-setup.sh` — checks:
    1. Postfix installed and running
    2. `master.cf` has armorclaw transport
    3. `main.cf` has `inet_interfaces = all` (Port 25 external with STARTTLS)
    4. STARTTLS certificate configured
    5. Pipe handler binary exists at `/usr/local/bin/armorclaw-mta-recv`
    6. Bridge IngestServer socket exists at `/run/armorclaw/email-ingest.sock`
    7. Permissions correct (0600 on socket, 0755 on binary)
  - Create integration test: simulate SMTP connection on Port 25 → STARTTLS negotiation → pipe(8) delivery → verify IngestServer receives it → verify event published
  - Verify full external-to-internal path: SMTP:25 → STARTTLS → Postfix → pipe → mta-recv → gRPC → IngestServer → EventBus
  - Document any deployment gotchas

  **Must NOT do**:
  - Do NOT require running Postfix for unit tests (verification script only)
  - Do NOT modify system Postfix config during testing

  **Recommended Agent Profile**:
  - **Category**: `quick`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 4 (with T19-T21)
  - **Blocks**: None
  - **Blocked By**: T4 (Postfix config), T19 (E2E verified)

  **References**:
  - `deploy/postfix/` (T4 output) — Configuration files to verify
  - `cmd/mta-recv/main.go` (T7 output) — Binary to verify
  - `bridge/pkg/email/ingest_server.go` (T8 output) — Socket to verify

  **Acceptance Criteria**:
  - [ ] `verify-setup.sh` runs and reports all checks
  - [ ] All checks pass when Postfix + Bridge are running
  - [ ] `bash -n deploy/postfix/verify-setup.sh` succeeds

  **QA Scenarios**:
  ```
  Scenario: Verification script syntax-valid and checks Port 25 + STARTTLS
    Tool: Bash
    Steps:
      1. Run: bash -n deploy/postfix/verify-setup.sh
      2. Assert: exit code 0 (syntax OK)
      3. Grep for "inet_interfaces" and "STARTTLS" in verify-setup.sh
      4. Assert: both present
    Expected Result: Script validates external Port 25 + STARTTLS config
    Evidence: .sisyphus/evidence/task-22-verify-script.txt

  Scenario: Full external-to-internal path (simulated)
    Tool: Bash
    Preconditions: Postfix running, Bridge running, mta-recv installed
    Steps:
      1. Send test email via: openssl s_client -connect localhost:25 -starttls smtp
      2. Issue: EHLO test, MAIL FROM:<test@example.com>, RCPT TO:<agent@example.com>, DATA, .
      3. Check Bridge logs for IngestServer receiving email
      4. Check EventBus for email.received event
    Expected Result: SMTP:25 → STARTTLS → Postfix → pipe → gRPC → event published
    Evidence: .sisyphus/evidence/task-22-smtp-path.txt
  ```

  **Commit**: YES
  - Message: `feat(deploy): add Postfix configuration and integration verification`

---

## Final Verification Wave (MANDATORY — after ALL implementation tasks)

> 4 review agents run in PARALLEL. ALL must APPROVE. Present consolidated results to user and get explicit "okay" before completing.

- [x] F1. **Plan Compliance Audit** — `oracle`
  Read the plan end-to-end. For each "Must Have": verify implementation exists (read file, run command). For each "Must NOT Have": search codebase for forbidden patterns — reject with file:line if found. Check evidence files exist in .sisyphus/evidence/. Compare deliverables against plan.
  Output: `Must Have [N/N] | Must NOT Have [N/N] | Tasks [N/N] | VERDICT: APPROVE/REJECT`

- [x] F2. **Code Quality Review** — `unspecified-high`
  Run `go vet` + `go test` + linter. Review all changed files for: `interface{}` used instead of `any`, empty catches, `fmt.Println` in prod, commented-out code, unused imports. Check AI slop: excessive comments, over-abstraction, generic names.
  Output: `Build [PASS/FAIL] | Lint [PASS/FAIL] | Tests [N pass/N fail] | Files [N clean/N issues] | VERDICT`

- [x] F3. **Real Manual QA** — `unspecified-high`
  Start from clean state. Execute EVERY QA scenario from EVERY task — follow exact steps, capture evidence. Test cross-task integration (ingestion → dispatch → outbound). Test edge cases: empty email, YARA match, OAuth expiry, malformed agent output. Save to `.sisyphus/evidence/final-qa/`.
  Output: `Scenarios [N/N pass] | Integration [N/N] | Edge Cases [N tested] | VERDICT`

- [x] F4. **Scope Fidelity Check** — `deep`
  For each task: read "What to do", read actual diff (git log/diff). Verify 1:1 — everything in spec was built (no missing), nothing beyond spec was built (no creep). Check "Must NOT do" compliance. Detect cross-task contamination. Flag unaccounted changes.
  Output: `Tasks [N/N compliant] | Contamination [CLEAN/N issues] | Unaccounted [CLEAN/N files] | VERDICT`

---

## Commit Strategy

- **Wave 1**: `feat(email): add proto definitions, types, storage, MIME parser, PII masker` - multiple files
- **T7**: `feat(email): add Postfix pipe handler for inbound email ingestion` - cmd/mta-recv/
- **T8**: `feat(email): add email ingestion server with YARA scanning` - bridge/pkg/email/
- **T9-T10**: `feat(email): add Gmail API and SMTP email clients` - bridge/pkg/email/
- **T11**: `feat(keystore): add OAuth token storage with XChaCha20-Poly1305` - bridge/pkg/keystore/
- **T12-T13**: `feat(email): add email dispatcher and workflow template` - bridge/pkg/email/
- **T14-T15**: `feat(email): add outbound executor with HITL approval` - bridge/pkg/email/
- **T16**: `feat(email): add Outlook provider adapter` - bridge/pkg/email/
- **T17**: `feat(email): add audit logging for all email operations` - bridge/pkg/email/
- **T18**: `docs(email): add Android integration specification` - doc/
- **T19-T20**: `test(email): add E2E pipeline and edge case tests` - tests/
- **T21**: `docs(email): add pipeline documentation` - doc/
- **T22**: `feat(deploy): add Postfix configuration and integration` - deploy/

---

## Success Criteria

### Verification Commands
```bash
# All email package tests pass
cd bridge && go test -v ./pkg/email/... ./cmd/mta-recv/...
# Expected: PASS, 0 failures

# PII masker tests pass
cd bridge && go test -v ./pkg/pii/... -run TestMasker
# Expected: PASS

# OAuth keystore tests pass
cd bridge && go test -v ./pkg/keystore/... -run TestOAuth
# Expected: PASS

# E2E pipeline test
cd bridge && go test -v -run TestEmailPipelineE2E ./pkg/email/...
# Expected: PASS

# Full regression
cd bridge && go test ./...
# Expected: All PASS
```

### Final Checklist
- [ ] All "Must Have" present
- [ ] All "Must NOT Have" absent
- [ ] All tests pass
- [ ] Android integration spec written
- [ ] Postfix config verified
- [ ] E2E pipeline test passes
