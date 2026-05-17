# ArmorClaw Core Roadmap — Deployment Completeness

## TL;DR

> **Quick Summary**: Close 3 operational gaps in ArmorClaw Core so the platform is deployable end-to-end: (1) Postfix email deployment infrastructure, (2) Sidecar binary release build + CI, (3) Self-hosted OpenClaw appliance experience.
>
> **Deliverables**:
> - IngestServer + EmailDispatcher wired into bridge startup (critical — both are dead code)
> - `bridge/cmd/bridge/setup_email.go` — email subsystem initialization
> - Email→Secretary workflow integration (EmailReceivedEvent → TaskScheduler → Workflow)
> - `deploy/postfix/` with main.cf, master.cf, transport_maps, install.sh, verify-setup.sh
> - Sidecar release build succeeding (cmake + clang build env)
> - Sidecar CI workflow (build + test + README cleanup, with caching)
> - `docker-compose.selfhosted.yml` + `configs/Caddyfile.selfhosted`
> - `deploy/deploy-selfhosted.sh` appliance setup script
> - Self-signed cert automation with rotation support
> - Documentation updates (root README + deploy docs)
>
> **Estimated Effort**: Large
> **Parallel Execution**: YES - 4 waves
> **Critical Path**: T1 (IngestServer + EmailDispatcher wiring) → T1b (Email→Secretary) → T4 (Postfix config) → T5 (install.sh) → T6 (verify-setup.sh) → T12 (docs) → F1-F4

---

## Context

### Original Request
Close operational gaps in ArmorClaw Core (bridge/, rust-vault/, jetski/, sidecar*/, deploy/) so the platform is deployable end-to-end. Three priorities: Email Deployment, Sidecar Binary, Self-Hosted Hardening.

### Interview Summary
**Key Discussions**:
- Postfix runs bare-metal on host VPS (not Docker)
- Self-hosted target is single VPS / home server
- No unit tests — agent-executed QA scenarios only
- Sidecar release fix: fix build environment (cmake + clang), not Cargo.toml change
- IngestServer discovered as dead code (Metis catch) — must be wired into bridge startup

**Research Findings**:
- Email Go pipeline is nearly complete (38 files, 21 test files) — gap is ONLY deploy/postfix/ + IngestServer wiring
- Sidecar binary compiles in dev (0 errors), release fails on aws-lc-sys gcc bug (from qdrant-client chain, NOT aws-sdk-s3)
- mDNS has two mature implementations (Go bridge + OpenClaw gateway) — gap is unified self-hosted appliance path
- 18 Docker Compose files exist — none for self-hosted appliance
- Caddy dev + production templates exist — no internal-only/self-signed template

### Metis Review
**Identified Gaps** (addressed):
- IngestServer never started (P1 blocker): Added as Task 1 (highest priority)
- Ring migration target wrong (aws-lc-sys from qdrant-client, not aws-sdk-s3): Corrected approach to build env fix
- Feature unification prevents simple Cargo.toml fix: Confirmed — using cmake + clang instead

---

## Work Objectives

### Core Objective
Make ArmorClaw Core fully deployable end-to-end: email flows from Postfix through the pipeline, sidecar binary builds cleanly for release, and self-hosted single-VPS setup works out of the box.

### Concrete Deliverables
- IngestServer started by bridge at runtime (Unix socket listener active)
- `deploy/postfix/` directory with complete Postfix configuration
- `deploy/postfix/install.sh` that installs and configures Postfix + mta-recv
- `deploy/postfix/verify-setup.sh` that validates end-to-end email flow
- Sidecar `cargo build --release` succeeds (cmake + clang build env)
- `.github/workflows/sidecar.yml` CI workflow (build + test + lint)
- `sidecar/README.md` updated with correct test counts and build instructions
- `docker-compose.selfhosted.yml` for single-VPS appliance
- `configs/Caddyfile.selfhosted` for internal-only TLS
- `deploy/deploy-selfhosted.sh` appliance setup script
- Self-signed cert generation in deploy scripts

### Definition of Done
- [ ] IngestServer listens on `/run/armorclaw/email-ingest.sock` after bridge startup
- [ ] YARA initialized — email attachments trigger malware scanning (verified by scanning EICAR test file)
- [ ] EmailReceivedEvent triggers EmailDispatcher → TaskScheduler → Workflow creation
- [ ] `deploy/postfix/verify-setup.sh` passes on fresh VPS
- [ ] `cargo build --release` succeeds in sidecar/ with cmake + clang installed
- [ ] `.github/workflows/sidecar.yml` passes in CI
- [ ] `deploy/deploy-selfhosted.sh` starts full stack on single VPS
- [ ] mDNS discovery works from Android client on same LAN
- [ ] Root README documents all deployment modes including self-hosted

### Must Have
- IngestServer wired into bridge startup (NOT optional — email is dead without it)
- EmailDispatcher wired to EventBus (NOT optional — emails won't trigger workflows)
- YARA initialization added to bridge startup (IngestServer depends on it)
- Postfix bare-metal installation (not Docker)
- Sidecar release build succeeding
- Self-signed cert automation for self-hosted mode
- mDNS working across LAN (Android can discover bridge via BOTH `_armorclaw._tcp` and `_openclaw-gw._tcp`)
- Documentation for new deployment modes

### Must NOT Have (Guardrails)
- Do NOT modify ArmorChat Android code (separate codebase)
- Do NOT change mDNS service types (both `_armorclaw._tcp` and `_openclaw-gw._tcp` remain)
- Do NOT remove qdrant-client dependency (fix build env instead)
- Do NOT add Let's Encrypt to self-hosted mode (self-signed only)
- Do NOT modify existing docker-compose files (create new selfhosted overlay)
- Do NOT touch Matrix Conduit internals
- Do NOT weaken SQLCipher or approval flows

---

## Verification Strategy

> **ZERO HUMAN INTERVENTION** - ALL verification is agent-executed. No exceptions.

### Test Decision
- **Infrastructure exists**: YES (Go tests, Rust tests exist)
- **Automated tests**: None for new deploy configs
- **Framework**: Agent-executed QA scenarios only (bash, curl, cargo)
- **No TDD**: Deployment configs tested via verify-setup.sh and health checks

### QA Policy
Every task MUST include agent-executed QA scenarios.
Evidence saved to `.sisyphus/evidence/task-{N}-{scenario-slug}.{ext}`.

- **Deploy scripts**: Use Bash — run script, check exit code, verify files exist, curl health endpoints
- **Sidecar builds**: Use Bash — cargo build, cargo test, cargo clippy
- **Compose configs**: Use Bash — docker compose config validation, container startup
- **Cert generation**: Use Bash — openssl verify, check cert fields

---

## Execution Strategy

### Parallel Execution Waves

```
Wave 1 (Start Immediately — critical blocker + foundation):
├── T1: Wire IngestServer + YARA into bridge startup [deep]
├── T2: Sidecar build env fix (cmake + clang) [quick]
└── T3: Sidecar README cleanup + warning fix [quick]

Wave 1.5 (After Wave 1 — email pipeline completion):
└── T1b: Wire EmailDispatcher → Secretary workflow [deep]

Wave 2 (After Wave 1.5 — Postfix deployment + CI):
├── T4: Create deploy/postfix/ config files [unspecified-high]
├── T5: Create deploy/postfix/install.sh [unspecified-high]
├── T6: Create deploy/postfix/verify-setup.sh [unspecified-high]
└── T7: Add sidecar CI workflow [quick]

Wave 3 (After Wave 2 — self-hosted appliance):
├── T8: Create docker-compose.selfhosted.yml [unspecified-high]
├── T9: Create configs/Caddyfile.selfhosted [quick]
├── T10: Create deploy/deploy-selfhosted.sh [unspecified-high]
├── T11: Self-signed cert automation + rotation [unspecified-high]
└── T12: Documentation updates [writing]

Wave FINAL (After ALL tasks — 4 parallel reviews):
├── F1: Plan compliance audit (oracle)
├── F2: Code quality review (unspecified-high)
├── F3: Real manual QA (unspecified-high)
└── F4: Scope fidelity check (deep)
-> Present results -> Get explicit user okay

Critical Path: T1 → T1b → T4 → T5 → T6 → T10 + T12 → F1-F4 → user okay
Max Concurrent: 4 (Waves 2 & 3)
```

### Dependency Matrix

| Task | Depends On | Blocks | Wave |
|------|-----------|--------|------|
| T1 | — | T1b, T4, T5, T6 | 1 |
| T2 | — | T7 | 1 |
| T3 | — | T7 | 1 |
| T1b | T1 | T4, T12 | 1.5 |
| T4 | T1, T1b | T5, T6 | 2 |
| T5 | T4 | T6 | 2 |
| T6 | T5 | F1-F4 | 2 |
| T7 | T2, T3 | F1-F4 | 2 |
| T8 | — | T10 | 3 |
| T9 | — | T10 | 3 |
| T10 | T8, T9, T11 | F1-F4 | 3 |
| T11 | — | T10 | 3 |
| T12 | T1, T1b, T4-T11 | F1-F4 | 3 |

### Agent Dispatch Summary

- **Wave 1**: 3 — T1 → `deep`, T2 → `quick`, T3 → `quick`
- **Wave 1.5**: 1 — T1b → `deep`
- **Wave 2**: 4 — T4 → `unspecified-high`, T5 → `unspecified-high`, T6 → `unspecified-high`, T7 → `quick`
- **Wave 3**: 5 — T8 → `unspecified-high`, T9 → `quick`, T10 → `unspecified-high`, T11 → `unspecified-high`, T12 → `writing`
- **FINAL**: 4 — F1 → `oracle`, F2 → `unspecified-high`, F3 → `unspecified-high`, F4 → `deep`

---

## TODOs

- [x] 1. Wire IngestServer + YARA into Bridge Startup

  **What to do**:

  This is the single most critical task. Two subsystems need wiring: IngestServer (Unix socket listener) and YARA scanner (package-level singleton).

  **Step 1: Add YARA initialization** (must happen BEFORE IngestServer):
  - In `bridge/cmd/bridge/main.go`, inside `runBridgeServer()`, after EventBus creation (~line 2203):
  - Add import: `"github.com/armorclaw/bridge/pkg/yara"` and `"path/filepath"` to import block
  ```go
  // Initialize YARA scanner (required by email ingest for attachment scanning)
  yaraRulesPath := filepath.Join("configs", "yara_rules.yar")
  if err := yara.InitYARA(yaraRulesPath); err != nil {
      log.Printf("Warning: YARA initialization failed (email attachments will bypass malware scan): %v", err)
      // Non-fatal: email attachments will pass through with warning
      // Note: using log.Printf (standard library) consistent with main.go error handling pattern
  }
  ```

  **Step 2: Create `bridge/cmd/bridge/setup_email.go`** — new file for email subsystem initialization:
  ```go
  package main

  import (
      "log"
      "path/filepath"
      "github.com/armorclaw/bridge/pkg/email"
      "github.com/armorclaw/bridge/pkg/eventbus"
      "github.com/armorclaw/bridge/pkg/logger"
      "github.com/armorclaw/bridge/pkg/sidecar"
  )

  func setupEmailIngest(eventBus *eventbus.EventBus, storageBaseDir string) *email.IngestServer {
      if eventBus == nil {
          return nil // Matrix disabled, no email
      }
      emailStorageDir := filepath.Join(storageBaseDir, "email-files")
      storage := email.NewLocalFSEmailStorage(emailStorageDir)
      
      // NOTE: PII masker is created internally by NewIngestServer via pii.NewMasker()
      // NOTE: YARA scan function is set internally — calls yara.ScanFileForMalware()
      //       Requires yara.InitYARA() to have been called first (Step 1 above)
      server := email.NewIngestServer(email.IngestServerConfig{
          Storage:             storage,              // EmailStorage interface — LocalFS
          Bus:                 eventBus,              // *eventbus.EventBus — for EmailReceivedEvent
          Socket:              "/run/armorclaw/email-ingest.sock",  // Unix domain socket path
          Log:                 logger.Get(),          // *logger.Logger
          SidecarOfficeClient: nil,                   // *sidecar.Client — nil safe (checked at line 165)
          SidecarRustClient:   nil,                   // *sidecar.Client — nil safe
          SidecarJavaClient:   nil,                   // *sidecar.Client — nil safe
      })
      if err := server.Start(); err != nil {
          log.Printf("Warning: Failed to start email ingest server: %v", err)
          return nil
      }
      log.Println("Email ingest server listening on", server.Socket())
      return server
  }
  ```

  **Internal initialization (no wiring needed)**:
  - `pii.NewMasker()` — called internally by `NewIngestServer` (ingest_server.go:47). Zero-arg, no dependencies.
  - `yara.ScanFileForMalware()` — set internally as default scan function (ingest_server.go:54). Uses package-level singleton.

  **Step 3: Call from `runBridgeServer()`** — insert after SDTW adapters (~line 2370), before RPC server creation (~line 2507):
  ```go
  // --- EMAIL INGEST SERVER ---
  var ingestServer *email.IngestServer
  if cfg.Matrix.Enabled {
      ingestServer = setupEmailIngest(eventBus, filepath.Dir(cfg.Keystore.DBPath))
  }
  ```

  **Step 4: Add graceful shutdown** — inside signal handler goroutine (~line 2728):
  ```go
  if ingestServer != nil {
      log.Println("Stopping email ingest server...")
      ingestServer.Stop()
  }
  ```

  **Error handling strategy**: Warning pattern (`log.Printf` from standard library) + continue, NOT `log.Fatalf`. Email ingest is an optional subsystem — the bridge must start even if email socket creation fails. Matches the pattern used for health monitor (~line 2163), signaling server (~line 2230), and studio (~line 2458). Note: `main.go` uses standard `log` package for warnings, not the project's `logger` — follow the same convention.

  **Must NOT do**:
  - Do NOT modify IngestServer's existing API or behavior
  - Do NOT create a separate binary for IngestServer — it runs inside the bridge process
  - Do NOT add email-related RPC methods (they already exist in email_approval.go)
  - Do NOT make IngestServer startup fatal — bridge must work without email

  **Recommended Agent Profile**:
  - **Category**: `deep`
    - Reason: Requires understanding bridge startup lifecycle, dependency injection, and graceful shutdown patterns
  - **Skills**: []
  - **Skills Evaluated but Omitted**:
    - `git-master`: Not needed — single focused change

  **Parallelization**:
  - **Can Run In Parallel**: YES (with T2, T3)
  - **Parallel Group**: Wave 1
  - **Blocks**: T4, T5, T6 (Postfix deployment needs IngestServer running)
  - **Blocked By**: None (can start immediately)

  **References**:

  **Pattern References** (existing code to follow — *line numbers are approximate and may shift after edits*):
  - `bridge/cmd/bridge/main.go` EventBus instantiation: `var eventBus *eventbus.EventBus` (~line 2170), `eventbus.NewEventBus(eventBusConfig)` (~line 2203). Conditional on `cfg.Matrix.Enabled`.
  - `bridge/cmd/bridge/main.go:2332-2370` — SDTW adapters section. INSERT POINT is after this block, before RPC server creation.
  - `bridge/cmd/bridge/main.go:2507-2540` — RPC server creation and startup in goroutine. IngestServer must start BEFORE this.
  - `bridge/cmd/bridge/main.go:2695-2761` — Signal handler for graceful shutdown. Add `ingestServer.Stop()` at ~line 2728, alongside existing stops.
  - `bridge/cmd/bridge/main.go:2163` — Health monitor error handling: `log.Printf("Warning: ...")` + continue. FOLLOW THIS PATTERN.
  - `bridge/cmd/bridge/main.go:2458` — Studio setup: `log.Printf("Warning: ...")` + continue. SAME PATTERN.

  **API/Type References**:
  - `bridge/pkg/email/ingest_server.go:33-41` — IngestServerConfig struct with 7 fields: `Storage EmailStorage`, `Bus *eventbus.EventBus`, `Socket string`, `Log *logger.Logger`, `SidecarOfficeClient *sidecar.Client`, `SidecarRustClient *sidecar.Client`, `SidecarJavaClient *sidecar.Client`
  - `bridge/pkg/email/ingest_server.go:43` — `NewIngestServer(cfg IngestServerConfig) *IngestServer` constructor
  - `bridge/pkg/email/ingest_server.go:47` — PII masker created internally via `pii.NewMasker()` (zero-arg, no wiring needed)
  - `bridge/pkg/email/ingest_server.go:54` — YARA scan function set internally, calls `yara.ScanFileForMalware()`
  - `bridge/pkg/email/ingest_server.go:56` — Default socket: `"/run/armorclaw/email-ingest.sock"`
  - `bridge/pkg/email/email_storage.go:28` — `NewLocalFSEmailStorage(dir string)` constructor
  - `bridge/pkg/yara/scanner.go:17` — `InitYARA(ruleFile string) error` — MUST be called before IngestServer, otherwise `ScanFileForMalware()` returns "YARA not initialized" error
  - `bridge/pkg/yara/scanner.go:13` — Package-level `compiledRules` singleton
  - `bridge/pkg/pii/masker.go:26` — `NewMasker()` zero-arg constructor (already called internally)

  **WHY Each Reference Matters**:
  - `main.go:2170-2203`: IngestServer requires EventBus — only created when Matrix is enabled
  - `main.go:2332-2370`: Exact insertion point for email setup
  - `main.go:2695-2761`: Graceful shutdown must drain pending emails
  - `main.go:2163,2458`: Error handling pattern — Warning, not Fatal
  - `ingest_server.go:33-41`: The EXACT config struct with all 7 fields — no guessing
  - `yara/scanner.go:17`: CRITICAL — YARA must be initialized or email attachments bypass malware scanning

  **Acceptance Criteria**:

  **QA Scenarios (MANDATORY):**

  ```
  Scenario: IngestServer starts with bridge and listens on Unix socket
    Tool: Bash
    Preconditions: Bridge binary compiled, /run/armorclaw/ directory exists
    Steps:
      1. Start bridge in background: `./bridge &`
      2. Wait 3 seconds for startup: `sleep 3`
      3. Check socket exists: `test -S /run/armorclaw/email-ingest.sock && echo "SOCKET_EXISTS"`
      4. Check process is running: `pgrep -f "bridge" | head -1`
      5. Stop bridge: `kill $(pgrep -f "bridge")`
    Expected Result: "SOCKET_EXISTS" printed, bridge process found
    Failure Indicators: Socket file missing, bridge crashed during startup
    Evidence: .sisyphus/evidence/task-1-ingest-socket-start.txt

  Scenario: IngestServer stops gracefully on bridge shutdown
    Tool: Bash
    Preconditions: Bridge running with IngestServer active
    Steps:
      1. Start bridge: `./bridge &`
      2. Wait for startup: `sleep 3`
      3. Send SIGTERM: `kill -TERM $(pgrep -f "bridge")`
      4. Wait for shutdown: `sleep 2`
      5. Check socket removed: `test -S /run/armorclaw/email-ingest.sock && echo "STILL_EXISTS" || echo "CLEANED_UP"`
      6. Check process stopped: `pgrep -f "bridge" || echo "STOPPED"`
    Expected Result: "CLEANED_UP" and "STOPPED"
    Failure Indicators: Socket still exists, process still running after 5s
    Evidence: .sisyphus/evidence/task-1-ingest-graceful-stop.txt

  Scenario: mta-recv can connect to IngestServer socket
    Tool: Bash
    Preconditions: Bridge running with IngestServer, mta-recv binary compiled
    Steps:
      1. Start bridge: `./bridge &`
      2. Wait for startup: `sleep 3`
      3. Send test email via mta-recv: `echo "From: test@example.com\nTo: user@armorclaw.local\nSubject: Test\n\nHello" | ./mta-recv --sender test@example.com --recipient user@armorclaw.local --queue-id TEST001`
      4. Check exit code: `echo $?`
      5. Stop bridge: `kill $(pgrep -f "bridge")`
    Expected Result: mta-recv exits with code 0 (successfully delivered to socket)
    Failure Indicators: mta-recv exits non-zero, "connection refused" error
    Evidence: .sisyphus/evidence/task-1-mta-recv-connect.txt

  Scenario: End-to-end email processing — YARA scan + PII mask + event published
    Tool: Bash
    Preconditions: Bridge running with IngestServer + YARA initialized, EventBus subscriber running
    Steps:
      1. Start bridge: `./bridge &`
      2. Wait for startup: `sleep 3`
      3. Send email with PII: `echo "From: test@example.com\nTo: user@armorclaw.local\nSubject: SSN Test\n\nMy SSN is 123-45-6789" | ./mta-recv --sender test@example.com --recipient user@armorclaw.local --queue-id TEST002`
      4. Check email stored: `ls /var/lib/armorclaw/email-files/emails/`
      5. Check PII masked in stored text: `grep -c "VAULT" /var/lib/armorclaw/email-files/emails/*/extracted-text.txt`
      6. Stop bridge: `kill $(pgrep -f "bridge")`
    Expected Result: Email stored, PII replaced with {{VAULT:ssn_N}} placeholders
    Failure Indicators: Raw PII in stored text, email not stored, YARA error
    Evidence: .sisyphus/evidence/task-1-e2e-email-processing.txt
  ```

  **Commit**: YES
  - Message: `feat(bridge): wire IngestServer into bridge startup`
  - Files: `bridge/cmd/bridge/main.go`
  - Pre-commit: `go build ./bridge/cmd/bridge/ && go test ./bridge/pkg/email/... -count=1`

- [x] 1b. Wire EmailReceivedEvent → Secretary Workflow

  **What to do**:

  The EmailDispatcher is fully implemented but NEVER instantiated. It must be created and wired to the EventBus so that inbound emails trigger secretary workflows.

  **Step 1: Add EmailDispatcher creation to `bridge/cmd/bridge/setup_email.go`** (created in T1):
  - Add `"context"` to imports
  ```go
  func setupEmailDispatcher(
      eventBus *eventbus.EventBus,
      ingestServer *email.IngestServer,
      scheduler *secretary.TaskScheduler,
      rolodexStore secretary.Store,
      log *logger.Logger,
  ) *email.EmailDispatcher {
      if eventBus == nil || ingestServer == nil || scheduler == nil {
          return nil
      }
      dispatcher := email.NewEmailDispatcher(email.DispatcherConfig{
          Store:     rolodexStore,     // secretary.Store from setup_secretary.go
          Scheduler: scheduler,         // *TaskScheduler from setup_secretary.go
          TeamMatcher: func(ctx context.Context, recipient string) (string, error) {
              // Resolve email recipient → teamID via rolodex/team store
              // If no team match, return "" (dispatcher falls through to template routing)
              return "", nil
          },
          TeamAgentLookup: func(ctx context.Context, teamID string) ([]string, error) {
              // Find agents with "email_clerk" role in the team
              // Uses rolodexStore or agent registry
              return nil, nil
          },
          Log: log,
      })
      // Subscribe dispatcher to EventBus for EmailReceivedEvent
      eventBus.Subscribe(eventbus.EventTypeEmailReceived, func(evt eventbus.BridgeEvent) {
          if emailEvt, ok := evt.(*email.EmailReceivedEvent); ok {
              dispatcher.OnEmailReceived(context.Background(), emailEvt)
          }
      })
      return dispatcher
  }
  ```

  **Note on TeamMatcher/TeamAgentLookup**: These are callback functions that the DispatcherConfig expects. If no team-based routing is needed initially, return empty values — the dispatcher will fall through to template-based routing (`dispatchViaTemplate`). Team routing can be implemented later by wiring to the rolodex/team store.

  **Step 2: Call from `runBridgeServer()`** — after IngestServer setup (T1), using existing secretary infrastructure:
  - The `secretary.TaskScheduler` is already created in `bridge/cmd/bridge/setup_secretary.go`
  - The `secretary.Store` (rolodexStore) is already created in `setup_secretary.go`
  - Wire them together: pass scheduler + store to `setupEmailDispatcher()`

  **Step 3: Seed the default email workflow template** — at the END of `setupEmailDispatcher()`, after the EventBus subscription succeeds. This ensures the "Email Analysis and Response" template exists in the secretary store for the dispatcher to look up:
  ```go
  // Seed default email workflow template if not already present
  if err := email.CreateEmailWorkflowTemplate(rolodexStore); err != nil {
      log.Printf("Warning: Failed to seed email workflow template: %v", err)
  }
  ```

  **Step 4: Verify the complete chain works**:
  - IngestServer publishes `EmailReceivedEvent` to EventBus
  - EmailDispatcher subscribes to `EventTypeEmailReceived`
  - Dispatcher calls `TaskScheduler.DispatchNow()` with a `ScheduledTask`
  - TaskScheduler creates a `Workflow` via `orchestrator.StartWorkflow()`
  - Workflow executes steps (Analyze + Send)

  **Must NOT do**:
  - Do NOT modify the EventBus publish mechanism (it already supports in-process + WebSocket)
  - Do NOT create new RPC methods
  - Do NOT modify the secretary package internals

  **Recommended Agent Profile**:
  - **Category**: `deep`
    - Reason: Cross-cutting integration between email and secretary subsystems, requires understanding both
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: NO (depends on T1)
  - **Parallel Group**: Sequential after Wave 1
  - **Blocks**: T4 (Postfix config — email pipeline should be end-to-end first)
  - **Blocked By**: T1 (IngestServer must be wired first)

  **References**:

  **Pattern References**:
  - `bridge/cmd/bridge/setup_secretary.go` — Secretary initialization. Creates `TaskScheduler`, `WorkflowOrchestrator`, `rolodexStore`. These are the EXACT dependencies EmailDispatcher needs.
  - `bridge/pkg/email/dispatcher.go` — EmailDispatcher with two paths: `tryTeamRouting()` (lines 72-114) and `dispatchViaTemplate()` (lines 116-151). The template path calls `d.scheduler.DispatchNow(ctx, task)`.
  - `bridge/pkg/email/dispatcher.go:NewEmailDispatcher()` — Constructor. Check exact parameter names/types.
  - `bridge/pkg/email/email_template.go` — `CreateEmailWorkflowTemplate(store)` factory function for seeding the default email template.

  **API/Type References**:
  - `bridge/pkg/eventbus/eventbus.go` — `Subscribe(eventType string, handler func(event BridgeEvent))` — how to register an in-process handler
  - `bridge/pkg/secretary/task_scheduler.go:288-296` — `DispatchNow(ctx, task)` — the exact method dispatcher calls
  - `bridge/pkg/secretary/types.go` — `TaskTemplate`, `ScheduledTask`, `Workflow` types

  **WHY Each Reference Matters**:
  - `setup_secretary.go`: The secretary infrastructure already exists — just need to pass its outputs to EmailDispatcher
  - `dispatcher.go`: The EXACT integration point — `dispatchViaTemplate()` creates `ScheduledTask` and calls `DispatchNow()`
  - `eventbus.go`: Must subscribe to `EventTypeEmailReceived` in-process (currently only WebSocket subscribers work)

  **Acceptance Criteria**:

  **QA Scenarios (MANDATORY):**

  ```
  Scenario: Inbound email creates a secretary workflow
    Tool: Bash
    Preconditions: Bridge running with IngestServer + EmailDispatcher, secretary initialized
    Steps:
      1. Start bridge: `./bridge &`
      2. Wait for startup: `sleep 3`
      3. Send email: `echo "From: boss@example.com\nTo: assistant@armorclaw.local\nSubject: Meeting\n\nSchedule a meeting" | ./mta-recv --sender boss@example.com --recipient assistant@armorclaw.local --queue-id TEST003`
      4. Check workflow created: Call `task.list` RPC method, grep for workflow with template "Email Analysis and Response"
      5. Check workflow state: Should be "pending" or "running" (not "error")
      6. Stop bridge: `kill $(pgrep -f "bridge")`
    Expected Result: Workflow created with email template, state is active
    Failure Indicators: No workflow created, workflow in "error" state, dispatcher not subscribed
    Evidence: .sisyphus/evidence/task-1b-email-workflow-created.txt

  Scenario: Email with no matching template dispatches without error
    Tool: Bash
    Preconditions: Bridge running, no template for recipient domain
    Steps:
      1. Start bridge: `./bridge &`
      2. Send email to unknown recipient: `echo "From: test@test.com\nTo: unknown@nowhere.xyz\nSubject: Test\n\nBody" | ./mta-recv --sender test@test.com --recipient unknown@nowhere.xyz --queue-id TEST004`
      3. Check bridge logs for errors: `grep -i "error\|panic" /tmp/bridge.log`
      4. Check no workflow created for unknown recipient
    Expected Result: No errors, no workflow created (graceful no-op)
    Failure Indicators: Panic, error in logs, spurious workflow
    Evidence: .sisyphus/evidence/task-1b-no-template-graceful.txt
  ```

  **Commit**: YES
  - Message: `feat(bridge): wire EmailDispatcher to EventBus for email-triggered workflows`
  - Files: `bridge/cmd/bridge/setup_email.go`
  - Pre-commit: `go build ./bridge/cmd/bridge/ && go test ./bridge/pkg/email/... -count=1`

- [x] 2. Fix Sidecar Release Build Environment

  **What to do**:
  - Create `sidecar/.cargo/config.toml` with build environment configuration:
    ```toml
    [env]
    CC = "clang"
    CXX = "clang++"
    ```
  - This overrides the C compiler for `aws-lc-sys` (from qdrant-client chain) to use clang instead of gcc, avoiding the gcc memcmp bug
  - Create `sidecar/build-release.sh`:
    ```bash
    #!/bin/bash
    set -euo pipefail
    # Check prerequisites
    for cmd in cmake clang cargo; do
        command -v "$cmd" >/dev/null || { echo "ERROR: $cmd not found. Install: apt-get install -y cmake clang"; exit 1; }
    done
    cd "$(dirname "$0")"
    echo "Building armorclaw-sidecar (release)..."
    cargo build --release --bin armorclaw-sidecar
    echo "Build complete: target/release/armorclaw-sidecar"
    ```
  - Add `sidecar/.github/env-setup.sh` documenting the build prerequisites (cmake, clang, nasm)

  **Must NOT do**:
  - Do NOT modify Cargo.toml dependencies or feature flags
  - Do NOT remove qdrant-client dependency
  - Do NOT change aws-lc-sys version or add patches

  **Recommended Agent Profile**:
  - **Category**: `quick`
    - Reason: Configuration file changes only, no complex logic
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES (with T1, T3)
  - **Parallel Group**: Wave 1
  - **Blocks**: T7 (CI workflow needs working release build)
  - **Blocked By**: None

  **References**:

  **Pattern References**:
  - `sidecar/build.log` — Previous release build failure output showing the exact gcc memcmp error
  - `sidecar/Cargo.toml` — Current dependency configuration showing qdrant-client v1.7 as the source of aws-lc-sys

  **API/Type References**:
  - `sidecar/.cargo/` — Check if directory already exists for config.toml placement

  **External References**:
  - Cargo config reference: https://doc.rust-lang.org/cargo/reference/config.html
  - aws-lc-sys build requirements: cmake + C compiler (gcc or clang)

  **WHY Each Reference Matters**:
  - `build.log`: Shows the exact error to fix — "COMPILER BUG DETECTED" from aws-lc-sys v0.39.1
  - `Cargo.toml`: Confirms qdrant-client is the dependency bringing in aws-lc-sys (not aws-sdk-s3)

  **Acceptance Criteria**:

  **QA Scenarios (MANDATORY):**

  ```
  Scenario: Sidecar release build succeeds with clang
    Tool: Bash
    Preconditions: cmake and clang installed (`apt-get install -y cmake clang`)
    Steps:
      1. cd sidecar
      2. cargo clean
      3. cargo build --release --bin armorclaw-sidecar 2>&1 | tail -20
      4. Check binary exists: `test -f target/release/armorclaw-sidecar && echo "BINARY_EXISTS"`
    Expected Result: Build completes with 0 errors, "BINARY_EXISTS" printed
    Failure Indicators: Build fails with aws-lc-sys error, compilation error, missing cmake
    Evidence: .sisyphus/evidence/task-2-release-build.txt

  Scenario: Dev build still works (no regression)
    Tool: Bash
    Preconditions: sidecar/ directory clean
    Steps:
      1. cd sidecar
      2. cargo build --bin armorclaw-sidecar 2>&1 | tail -5
      3. cargo test --lib 2>&1 | grep "test result"
    Expected Result: 0 errors, "252 passed; 0 failed; 8 ignored"
    Failure Indicators: Any test failures, compilation errors
    Evidence: .sisyphus/evidence/task-2-dev-build.txt
  ```

  **Commit**: YES
  - Message: `fix(sidecar): configure clang for release builds to fix aws-lc-sys gcc bug`
  - Files: `sidecar/.cargo/config.toml`, `sidecar/build-release.sh`
  - Pre-commit: `cd sidecar && cargo build --bin armorclaw-sidecar`

- [x] 3. Sidecar README Cleanup + Warning Fix

  **What to do**:
  - Update `sidecar/README.md`:
    - Fix "74 binary errors remaining" → "Binary compiles cleanly in dev and release (with cmake + clang)"
    - Fix "31/33 tests (2 failing)" → "252 tests passing, 8 ignored, 0 failing (260 total)"
    - Add correct build instructions: `cargo build --release --bin armorclaw-sidecar` with cmake + clang prerequisite
    - Update the test command: `cargo test --lib`
  - Run `cargo fix --lib -p armorclaw-sidecar --allow-dirty` to auto-fix 21 warnings
  - Manually review remaining 10 warnings (unused SharePoint code) — add `#[allow(dead_code)]` with TODO comment or gate behind feature flag

  **Must NOT do**:
  - Do NOT remove SharePoint module entirely (test files may depend on it)
  - Do NOT change any test assertions or test logic

  **Recommended Agent Profile**:
  - **Category**: `quick`
    - Reason: README update + auto-fix warnings, straightforward
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES (with T1, T2)
  - **Parallel Group**: Wave 1
  - **Blocks**: T7 (CI workflow needs clean README)
  - **Blocked By**: None

  **References**:

  **Pattern References**:
  - `sidecar/README.md` — Current stale content with wrong test counts and error counts
  - `sidecar/src/connectors/sharepoint.rs` — Dead code module (all methods "never used")

  **API/Type References**:
  - `sidecar/src/main.rs` — Binary entry point (23 lines, minimal)
  - `sidecar/src/lib.rs` — Library root with 12 public modules

  **WHY Each Reference Matters**:
  - `README.md`: The exact file to update with correct information
  - `sharepoint.rs`: Source of dead code warnings — needs `#[allow(dead_code)]` or feature gate

  **Acceptance Criteria**:

  **QA Scenarios (MANDATORY):**

  ```
  Scenario: README contains correct test counts
    Tool: Bash
    Preconditions: README.md updated
    Steps:
      1. grep -c "252 tests passing" sidecar/README.md
      2. grep -c "74 binary errors" sidecar/README.md
      3. grep -c "31/33 tests" sidecar/README.md
    Expected Result: grep for "252 tests" returns >= 1, grep for stale counts returns 0
    Failure Indicators: Stale "74 errors" or "31/33 tests" still present
    Evidence: .sisyphus/evidence/task-3-readme-cleanup.txt

  Scenario: Warning count reduced after cargo fix
    Tool: Bash
    Preconditions: cargo fix applied
    Steps:
      1. cd sidecar && cargo build --bin armorclaw-sidecar 2>&1 | grep -c "warning"
    Expected Result: Warning count <= 10 (down from 31)
    Failure Indicators: Warning count still at 31 (auto-fix didn't apply)
    Evidence: .sisyphus/evidence/task-3-warning-count.txt
  ```

  **Commit**: YES
  - Message: `docs(sidecar): update README with correct build status and test counts`
  - Files: `sidecar/README.md`, `sidecar/src/connectors/sharepoint.rs` (if adding #[allow])
  - Pre-commit: `cd sidecar && cargo build --bin armorclaw-sidecar`

- [x] 4. Create Postfix Configuration Files

  **What to do**:
  - Create `deploy/postfix/` directory with:
    - `main.cf` — Postfix main configuration:
      - `myhostname`, `mydomain`, `myorigin` from environment or auto-detected
      - `inet_interfaces = all` (listen on all interfaces)
      - `mailbox_size_limit = 26214400` (25MB, matching mta-recv max)
      - `message_size_limit = 26214400`
      - Transport maps pointing to the armorclaw pipe handler
      - Basic anti-spam: `smtpd_helo_required = yes`, `disable_vrfy_command = yes`
      - TLS: `smtpd_tls_security_level = may`, cert paths
    - `master.cf` — Append/override section defining the armorclaw pipe service:
      - Service name: `armorclaw`
      - Type: `unix` or `pipe`
      - Command: path to mta-recv binary with sender/recipient/queue-id args
      - User: `armorclaw` (dedicated system user)
      - Max processes: 10
    - `transport_maps` — Map email domains to the armorclaw pipe transport:
      - Example line: `*    armorclaw:` (catch-all — routes ALL local email to armorclaw pipe)
      - Or domain-specific: `armorclaw.local    armorclaw:`
      - Generated with `postmap /etc/postfix/transport_maps` to produce `.db` file
  - All config files should use environment variable substitution where possible

  **Must NOT do**:
  - Do NOT configure Postfix as an open relay
  - Do NOT hardcode domain names (use variables)
  - Do NOT configure Postfix to run in Docker

  **Recommended Agent Profile**:
  - **Category**: `unspecified-high`
    - Reason: Requires Postfix configuration knowledge, security considerations
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES (with T5 in same wave, but logically T4 creates configs that T5 installs)
  - **Parallel Group**: Wave 2
  - **Blocks**: T5 (install.sh needs these config files)
  - **Blocked By**: T1 (IngestServer must be wired first — Postfix needs a listening socket)

  **References**:

  **Pattern References**:
  - `bridge/cmd/mta-recv/main.go` — The pipe handler binary. Reads envelope args: `--sender`, `--recipient`, `--queue-id`. Reads raw email from stdin. Connects to `/run/armorclaw/email-ingest.sock`. This defines EXACTLY what the Postfix pipe transport must provide.
  - `deploy/container-setup.sh` — Existing deploy script pattern (2528 lines). Shows the code style, error handling patterns, and how other deploy scripts are structured.
  - `deploy/deploy-infra.sh` — Infrastructure deployment patterns

  **API/Type References**:
  - `bridge/pkg/email/ingest_server.go:NewIngestServer()` — Socket path config: `/run/armorclaw/email-ingest.sock`
  - `bridge/pkg/email/mime_parser.go` — Max email size: 26MB (26214400 bytes)

  **External References**:
  - Postfix pipe(8) documentation: http://www.postfix.org/pipe.8.html
  - Postfix transport(5) documentation: http://www.postfix.org/transport.5.html

  **WHY Each Reference Matters**:
  - `mta-recv/main.go`: Defines the EXACT CLI interface the Postfix pipe must invoke — flags, args, stdin behavior
  - `ingest_server.go`: Defines the socket path and protocol that mta-recv expects
  - `mime_parser.go`: Defines max email size — Postfix must match

  **Acceptance Criteria**:

  **QA Scenarios (MANDATORY):**

  ```
  Scenario: Postfix config files are syntactically valid
    Tool: Bash
    Preconditions: Postfix installed on system
    Steps:
      1. postconf -c deploy/postfix/ 2>&1 | head -5
      2. grep "armorclaw" deploy/postfix/master.cf
      3. grep "transport_maps" deploy/postfix/main.cf
    Expected Result: postconf parses without error, armorclaw pipe defined in master.cf, transport_maps referenced in main.cf
    Failure Indicators: postconf reports syntax errors, missing armorclaw service
    Evidence: .sisyphus/evidence/task-4-postfix-config.txt

  Scenario: Transport map correctly routes to armorclaw pipe
    Tool: Bash
    Preconditions: transport_maps file exists
    Steps:
      1. postmap -q "user@armorclaw.local" deploy/postfix/transport_maps
    Expected Result: Returns "armorclaw:" (routes to armorclaw pipe transport)
    Failure Indicators: Empty result or wrong transport name
    Evidence: .sisyphus/evidence/task-4-transport-map.txt
  ```

  **Commit**: YES
  - Message: `feat(deploy): add Postfix configuration files for email ingestion`
  - Files: `deploy/postfix/main.cf`, `deploy/postfix/master.cf`, `deploy/postfix/transport_maps`
  - Pre-commit: `grep -r "armorclaw" deploy/postfix/`

- [x] 5. Create Postfix Install Script

  **What to do**:
  - Create `deploy/postfix/install.sh` that:
    1. Checks prerequisites: Postfix installed, mta-recv binary exists, bridge running
    2. Creates system user `armorclaw` if not exists (for pipe handler)
    3. Creates directories: `/run/armorclaw/`, `/var/lib/armorclaw/email-files/`, `/var/log/armorclaw/email/`
    4. Sets permissions: socket dir owned by armorclaw user, email dirs readable by bridge
    5. **Socket permission setup for host Postfix + container bridge**:
       - Create group `armorclaw-mail` if not exists
       - Add `armorclaw` user to this group
       - Add `postfix` user to this group if it exists (non-fatal if missing — Postfix may not be installed yet)
       - Set `/run/armorclaw/` permissions: `0775` owned by `armorclaw:armorclaw-mail`
       - This ensures host Postfix (running as `postfix`) can write to the socket that bridge (running as `armorclaw`) creates
    6. Copies main.cf and master.cf to `/etc/postfix/`
    7. Generates transport_maps.db: `postmap /etc/postfix/transport_maps`
    8. Builds mta-recv binary: `go build -o /usr/local/bin/mta-recv ./bridge/cmd/mta-recv/`
    9. Creates systemd service `mta-recv.service` (if IngestServer runs in bridge, this may not be needed — verify after T1)
    10. Reloads Postfix: `postfix reload`
    11. Prints success with next steps
  - Script must be idempotent (safe to re-run)
  - Follow existing deploy script patterns from `deploy/container-setup.sh`

  **Must NOT do**:
  - Do NOT modify existing Postfix config without backing up
  - Do NOT start Postfix if it's not already configured for the domain
  - Do NOT hardcode paths — use variables

  **Recommended Agent Profile**:
  - **Category**: `unspecified-high`
    - Reason: Complex bash script with system administration, security considerations
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: NO (needs T4 config files)
  - **Parallel Group**: Wave 2
  - **Blocks**: T6 (verify-setup.sh needs installed system)
  - **Blocked By**: T4 (needs main.cf, master.cf, transport_maps)

  **References**:

  **Pattern References**:
  - `deploy/container-setup.sh` — 2528-line setup wizard. Follow its structure: banner, prerequisites check, idempotent operations, error handling, colored output.
  - `deploy/install.sh` — Quick installer pattern for reference
  - `deploy/armorclaw-harden.sh` — Hardening patterns (user creation, permissions, systemd)

  **API/Type References**:
  - `bridge/cmd/mta-recv/main.go` — The binary to install as `/usr/local/bin/mta-recv`
  - `bridge/pkg/email/ingest_server.go` — Socket path: `/run/armorclaw/email-ingest.sock`
  - `bridge/pkg/email/email_storage.go` — Storage path: `/var/lib/armorclaw/email-files/`
  - `bridge/pkg/email/audit.go` — Audit log path: `/var/log/armorclaw/email/`

  **WHY Each Reference Matters**:
  - `container-setup.sh`: The canonical deploy script pattern — follow its error handling and output style
  - `mta-recv/main.go`: Binary source path and compilation target
  - `ingest_server.go`, `email_storage.go`, `audit.go`: Exact filesystem paths that must be created

  **Acceptance Criteria**:

  **QA Scenarios (MANDATORY):**

  ```
  Scenario: install.sh completes without errors on fresh system
    Tool: Bash
    Preconditions: Go compiler installed, bridge source available, Postfix installed
    Steps:
      1. bash deploy/postfix/install.sh 2>&1
      2. Check exit code: echo $?
      3. Verify mta-recv binary: test -f /usr/local/bin/mta-recv && echo "BINARY_INSTALLED"
      4. Verify directories: test -d /var/lib/armorclaw/email-files && echo "DIRS_CREATED"
    Expected Result: Exit code 0, binary and directories created
    Failure Indicators: Non-zero exit, missing binary, missing directories
    Evidence: .sisyphus/evidence/task-5-install-script.txt

  Scenario: install.sh is idempotent (safe to re-run)
    Tool: Bash
    Preconditions: install.sh already ran once
    Steps:
      1. bash deploy/postfix/install.sh 2>&1
      2. echo $?
    Expected Result: Exit code 0, no errors about existing users/directories
    Failure Indicators: "user already exists" error treated as fatal, directory creation fails
    Evidence: .sisyphus/evidence/task-5-install-idempotent.txt
  ```

  **Commit**: YES
  - Message: `feat(deploy): add Postfix install script with mta-recv setup`
  - Files: `deploy/postfix/install.sh`
  - Pre-commit: `bash -n deploy/postfix/install.sh` (syntax check)

- [x] 6. Create Postfix Verification Script

  **What to do**:
  - Create `deploy/postfix/verify-setup.sh` that performs comprehensive health checks:
    1. **Binary checks**: mta-recv exists and is executable
    2. **Directory checks**: `/run/armorclaw/`, `/var/lib/armorclaw/email-files/`, `/var/log/armorclaw/email/` exist with correct permissions
    3. **Socket check**: `/run/armorclaw/email-ingest.sock` exists and is a Unix socket (bridge must be running)
    4. **Postfix check**: `postfix status` returns running
    5. **Transport check**: `postmap -q test@localhost transport_maps` returns `armorclaw:`
    6. **YARA check**: `test -f bridge/configs/yara_rules.yar` — rules file exists (this is the path used by T1's `yara.InitYARA()`)
    7. **End-to-end email test**: Send a test email via `sendmail` or `swaks`, verify it reaches mta-recv
    8. **Log check**: Email audit log writable
  - Output: Clear pass/fail for each check with green ✓ / red ✗ indicators
  - Exit code: 0 if all pass, 1 if any fail
  - Follow pattern from `deploy/verify-bridge.sh` if it exists

  **Must NOT do**:
  - Do NOT require human interaction (fully automated)
  - Do NOT send real emails to external addresses (test only)
  - Do NOT modify any system state

  **Recommended Agent Profile**:
  - **Category**: `unspecified-high`
    - Reason: Comprehensive verification script with multiple system checks
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES (with T7)
  - **Parallel Group**: Wave 2
  - **Blocks**: F1-F4 (final verification)
  - **Blocked By**: T5 (needs installed system to verify)

  **References**:

  **Pattern References**:
  - `deploy/verify-bridge.sh` — Existing verification script pattern
  - `deploy/verify-security.sh` — Security verification patterns
  - `deploy/verify-checksum.sh` — Checksum verification patterns

  **API/Type References**:
  - `bridge/pkg/email/ingest_server.go` — Socket path for verification
  - `bridge/configs/yara_rules.yar` — YARA rules file that must exist

  **WHY Each Reference Matters**:
  - `verify-bridge.sh`: Follow the same output format and check structure
  - `yara_rules.yar`: Must verify this file exists (IngestServer needs it)

  **Acceptance Criteria**:

  **QA Scenarios (MANDATORY):**

  ```
  Scenario: verify-setup.sh detects missing components
    Tool: Bash
    Preconditions: Postfix installed but bridge NOT running (socket missing)
    Steps:
      1. bash deploy/postfix/verify-setup.sh 2>&1
      2. echo $?
    Expected Result: Exit code 1, reports socket check as FAILED, other checks may pass
    Failure Indicators: Exit code 0 when socket is missing (false positive)
    Evidence: .sisyphus/evidence/task-6-verify-missing-socket.txt

  Scenario: verify-setup.sh passes on fully configured system
    Tool: Bash
    Preconditions: install.sh ran, bridge running, Postfix configured
    Steps:
      1. bash deploy/postfix/verify-setup.sh 2>&1
      2. echo $?
    Expected Result: Exit code 0, all checks show ✓
    Failure Indicators: Any check shows ✗ when system is fully configured
    Evidence: .sisyphus/evidence/task-6-verify-all-pass.txt
  ```

  **Commit**: YES
  - Message: `feat(deploy): add Postfix verification script with health checks`
  - Files: `deploy/postfix/verify-setup.sh`
  - Pre-commit: `bash -n deploy/postfix/verify-setup.sh`

- [x] 7. Add Sidecar CI Workflow

  **What to do**:
  - Create `.github/workflows/sidecar.yml`:
    ```yaml
    name: Sidecar CI
    on: [push, pull_request]
    paths: ['sidecar/**']
    jobs:
      build:
        runs-on: ubuntu-latest
        steps:
          - uses: actions/checkout@v4
          - uses: dtolnay/rust-toolchain@stable
            with:
              toolchain: 1.82.0
          - name: Cache cargo
            uses: actions/cache@v4
            with:
              path: |
                ~/.cargo/registry
                ~/.cargo/git
                sidecar/target
              key: ${{ runner.os }}-cargo-${{ hashFiles('sidecar/Cargo.lock') }}
              restore-keys: ${{ runner.os }}-cargo-
          - name: Install build deps
            run: sudo apt-get install -y cmake clang nasm
          - name: Build (dev)
            run: cd sidecar && cargo build --bin armorclaw-sidecar
          - name: Build (release)
            run: cd sidecar && CC=clang CXX=clang++ cargo build --release --bin armorclaw-sidecar
          - name: Test
            run: cd sidecar && cargo test --lib
          - name: Clippy
            run: cd sidecar && cargo clippy -- -D warnings
          - name: Upload release binary
            if: startsWith(github.ref, 'refs/tags/')
            uses: actions/upload-artifact@v4
            with:
              name: armorclaw-sidecar-${{ runner.os }}
              path: sidecar/target/release/armorclaw-sidecar
    ```
  - Follow existing workflow patterns from `.github/workflows/test.yml`

  **Must NOT do**:
  - Do NOT modify existing workflows
  - Do NOT add Docker build steps (binary-only for now)
  - Do NOT run tests that require external services (S3, Qdrant)

  **Recommended Agent Profile**:
  - **Category**: `quick`
    - Reason: Standard GitHub Actions workflow, follows existing patterns
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES (with T6)
  - **Parallel Group**: Wave 2
  - **Blocks**: F1-F4
  - **Blocked By**: T2 (build env fix), T3 (README cleanup)

  **References**:

  **Pattern References**:
  - `.github/workflows/test.yml` — Existing CI workflow pattern (Docker container tests)
  - `.github/workflows/build-release.yml` — Release build pattern (Go only, extend to Rust)

  **WHY Each Reference Matters**:
  - `test.yml`: Follow the same trigger patterns, checkout steps, and output format

  **Acceptance Criteria**:

  **QA Scenarios (MANDATORY):**

  ```
  Scenario: Workflow YAML is valid
    Tool: Bash
    Preconditions: sidecar.yml created
    Steps:
      1. python3 -c "import yaml; yaml.safe_load(open('.github/workflows/sidecar.yml'))"
      2. echo $?
    Expected Result: Exit code 0, valid YAML
    Failure Indicators: YAML parse error
    Evidence: .sisyphus/evidence/task-7-ci-yaml-valid.txt

  Scenario: Workflow triggers on sidecar path changes
    Tool: Bash
    Preconditions: sidecar.yml created
    Steps:
      1. grep "paths:" .github/workflows/sidecar.yml
      2. grep "sidecar" .github/workflows/sidecar.yml
    Expected Result: Path filter includes 'sidecar/**'
    Failure Indicators: No path filter, or wrong path
    Evidence: .sisyphus/evidence/task-7-ci-path-filter.txt
  ```

  **Commit**: YES
  - Message: `ci(sidecar): add GitHub Actions workflow for sidecar build + test`
  - Files: `.github/workflows/sidecar.yml`
  - Pre-commit: `python3 -c "import yaml; yaml.safe_load(open('.github/workflows/sidecar.yml'))"`

- [x] 8. Create Self-Hosted Docker Compose

  **What to do**:
  - Create `docker-compose.selfhosted.yml` — single-VPS appliance overlay:
    - Services: bridge, matrix (conduit), caddy, openclaw (default, not profile)
    - Caddy uses `configs/Caddyfile.selfhosted` (from T9)
    - OpenClaw container started as default service (not behind a profile)
    - Bridge uses `network_mode: host` for mDNS to work (or mount avahi socket)
    - **Avahi socket mount** for containers that need mDNS: `- /var/run/avahi-daemon/socket:/var/run/avahi-daemon/socket:ro`
    - **ArmorClaw socket dir** for host Postfix → container bridge: `- /run/armorclaw:/run/armorclaw`
    - **Bridge healthcheck** verifying email socket:
      ```yaml
      healthcheck:
        test: ["CMD", "test", "-S", "/run/armorclaw/email-ingest.sock"]
        interval: 30s
        timeout: 5s
        retries: 3
      ```
    - Both mDNS service types advertised: `_armorclaw._tcp` (Go bridge) AND `_openclaw-gw._tcp` (OpenClaw gateway)
    - Document in compose comments that both service types are intentional and must remain:
      ```yaml
      # IMPORTANT: Both mDNS service types must remain:
      #   _armorclaw._tcp     (Go Bridge — bridge/pkg/discovery/mdns.go)
      #   _openclaw-gw._tcp   (OpenClaw Gateway — container/openclaw-src/src/infra/bonjour.ts)
      ```
    - Named volumes for persistence: `matrix-data`, `bridge-data`, `email-data`
    - Health checks for all services
    - Resource limits appropriate for single VPS (2-4GB RAM total)
  - Include `.env.selfhosted` template with variables: `ARMORCLAW_HOSTNAME`, `ADMIN_EMAIL`, `LAN_DOMAIN`
  - Add README section or `deploy/selfhosted/README.md` explaining the appliance mode

  **Must NOT do**:
  - Do NOT modify existing docker-compose files
  - Do NOT include Cloudflare or Let's Encrypt (self-signed only)
  - Do NOT expose ports to public internet (LAN only)
  - Do NOT require a public domain name

  **Recommended Agent Profile**:
  - **Category**: `unspecified-high`
    - Reason: Docker Compose networking, mDNS requirements, service orchestration
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES (with T9, T11)
  - **Parallel Group**: Wave 3
  - **Blocks**: T10 (deploy script needs compose file)
  - **Blocked By**: None (independent of email/sidecar work)

  **References**:

  **Pattern References**:
  - `docker-compose.yml` — Meta-composition with bridge + vault + qdrant + caddy sentinel. Shows the canonical service structure, network isolation, volume naming.
  - `docker-compose-full.yml` — Complete stack with matrix + sygnal + caddy. Shows production patterns: health checks, resource limits, named volumes.
  - `docker-compose.bridge.yml` — Shows openclaw as a profile, sygnal, mautrix bridges. Self-hosted compose should include openclaw as DEFAULT (not profile).

  **API/Type References**:
  - `container/Dockerfile.openclaw` — Python agent container. Image name, ports (none — Unix socket), security hardening.
  - `container/Dockerfile.openclaw-standalone` — Full Node.js gateway container. Alternative image.
  - `bridge/pkg/discovery/mdns.go` — Go bridge mDNS advertiser. Requires `network_mode: host` or avahi socket mount for multicast to work.

  **External References**:
  - Docker Compose networking: https://docs.docker.com/compose/networking/
  - Avahi socket passthrough for mDNS in containers: mount `/var/run/avahi-daemon/socket`

  **WHY Each Reference Matters**:
  - `docker-compose.yml` and `docker-compose-full.yml`: Patterns for service structure, health checks, volumes — follow these conventions.
  - `mdns.go`: mDNS requires either host networking or avahi socket — this determines the network configuration.
  - `Dockerfile.openclaw`: The container image to include as a default service.

  **Acceptance Criteria**:

  **QA Scenarios (MANDATORY):**

  ```
  Scenario: Docker Compose config validates
    Tool: Bash
    Preconditions: docker-compose.selfhosted.yml created
    Steps:
      1. docker compose -f docker-compose.selfhosted.yml config 2>&1
      2. echo $?
    Expected Result: Exit code 0, valid configuration
    Failure Indicators: Validation errors, missing service definitions
    Evidence: .sisyphus/evidence/task-8-compose-valid.txt

  Scenario: OpenClaw is a default service (not behind profile)
    Tool: Bash
    Preconditions: docker-compose.selfhosted.yml created
    Steps:
      1. grep -A5 "openclaw:" docker-compose.selfhosted.yml | grep -c "profiles"
      2. grep "openclaw:" docker-compose.selfhosted.yml
    Expected Result: No "profiles" line for openclaw service (it starts by default)
    Failure Indicators: openclaw behind a profile (wouldn't start by default)
    Evidence: .sisyphus/evidence/task-8-openclaw-default.txt

  Scenario: Both mDNS service types documented in compose
    Tool: Bash
    Preconditions: docker-compose.selfhosted.yml created
    Steps:
      1. grep -c "_armorclaw._tcp\|_openclaw-gw._tcp\|armorclaw._tcp\|openclaw-gw._tcp" docker-compose.selfhosted.yml
    Expected Result: At least 1 match (both service types mentioned in comments or config)
    Failure Indicators: Zero matches (mDNS service types not documented)
    Evidence: .sisyphus/evidence/task-8-mdns-service-types.txt

  Scenario: Avahi socket mounted for mDNS in containers
    Tool: Bash
    Preconditions: docker-compose.selfhosted.yml created
    Steps:
      1. grep -c "avahi" docker-compose.selfhosted.yml
    Expected Result: At least 1 match (avahi socket path referenced)
    Failure Indicators: Zero matches (no avahi mount, mDNS won't work in containers)
    Evidence: .sisyphus/evidence/task-8-avahi-mount.txt
  ```

  **Commit**: YES
  - Message: `feat(deploy): add self-hosted Docker Compose for single-VPS appliance`
  - Files: `docker-compose.selfhosted.yml`, `.env.selfhosted`
  - Pre-commit: `docker compose -f docker-compose.selfhosted.yml config`

- [x] 9. Create Self-Hosted Caddyfile

  **What to do**:
  - Create `configs/Caddyfile.selfhosted`:
    - Listen on `:443` with self-signed TLS (internal CA or auto-generated)
    - Alternatively listen on `:80` (HTTP only for LAN) — support both modes
    - Reverse proxy routes:
      - `/api*` → bridge:8080 (JSON-RPC)
      - `/_matrix/*` → matrix:6167 (Matrix Conduit)
      - `/.well-known/*` → matrix:6167
      - `/discover` → bridge:8080 (mDNS discovery info)
    - No Let's Encrypt, no ACME, no public domain required
    - Use internal hostname (e.g., `armorclaw.local` or LAN IP)
  - Include comments explaining self-signed cert setup (references T11)

  **Must NOT do**:
  - Do NOT use Let's Encrypt or ACME
  - Do NOT require a public domain name
  - Do NOT modify existing Caddyfile or Caddyfile.template

  **Recommended Agent Profile**:
  - **Category**: `quick`
    - Reason: Caddy configuration file, straightforward reverse proxy
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES (with T8, T11)
  - **Parallel Group**: Wave 3
  - **Blocks**: T10 (deploy script needs Caddyfile)
  - **Blocked By**: None

  **References**:

  **Pattern References**:
  - `Caddyfile` — Dev config: `{$MATRIX_DOMAIN:matrix.localhost}`, routes `/api*`, `/_matrix/*`, well-known. Follow this exact routing pattern.
  - `configs/Caddyfile.template` — Production template: `${DOMAIN_NAME}`, `${ADMIN_EMAIL}`, Let's Encrypt. Shows the full route set.

  **WHY Each Reference Matters**:
  - `Caddyfile`: The dev config has the exact routes needed — copy the routing, change TLS to self-signed.

  **Acceptance Criteria**:

  **QA Scenarios (MANDATORY):**

  ```
  Scenario: Caddyfile.selfhosted has all required routes
    Tool: Bash
    Preconditions: Caddyfile.selfhosted created
    Steps:
      1. grep -c "api" configs/Caddyfile.selfhosted
      2. grep -c "_matrix" configs/Caddyfile.selfhosted
      3. grep -c "well-known" configs/Caddyfile.selfhosted
      4. grep -c "acme\|letsencrypt\|cloudflare" configs/Caddyfile.selfhosted || echo "NO_PUBLIC_TLS"
    Expected Result: api >= 1, _matrix >= 1, well-known >= 1, NO_PUBLIC_TLS
    Failure Indicators: Missing routes, or contains Let's Encrypt config
    Evidence: .sisyphus/evidence/task-9-caddyfile-routes.txt
  ```

  **Commit**: YES
  - Message: `feat(config): add Caddyfile for self-hosted internal-only TLS`
  - Files: `configs/Caddyfile.selfhosted`
  - Pre-commit: `grep -c "reverse_proxy" configs/Caddyfile.selfhosted`

- [x] 10. Create Self-Hosted Deployment Script

  **What to do**:
  - Create `deploy/deploy-selfhosted.sh` — appliance mode setup:
    1. Detect LAN-only mode (no public IP, no domain)
    2. Install prerequisites: Docker, Docker Compose, Avahi (for mDNS)
    3. Generate self-signed certs (call T11 script)
    4. Create `.env.selfhosted` with auto-detected hostname
    5. Build/pull Docker images
    6. Start stack: `docker compose -f docker-compose.selfhosted.yml up -d`
    7. Wait for health checks to pass
    8. Print discovery info: mDNS hostname, TLS fingerprint, QR code hint
    9. Print next steps: install ArmorChat, scan QR, enter hostname
  - Follow patterns from `deploy/container-setup.sh` (2528 lines)
  - Must be idempotent

  **Must NOT do**:
  - Do NOT require interactive input (support `--auto` mode)
  - Do NOT modify existing deploy scripts
  - Do NOT install Cloudflare tools or require a public domain

  **Recommended Agent Profile**:
  - **Category**: `unspecified-high`
    - Reason: Complex bash script with system admin, Docker orchestration, mDNS setup
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: NO (needs T8 compose + T9 Caddyfile + T11 certs)
  - **Parallel Group**: Wave 3
  - **Blocks**: F1-F4
  - **Blocked By**: T8 (compose file), T9 (Caddyfile), T11 (cert generation)

  **References**:

  **Pattern References**:
  - `deploy/container-setup.sh` — 2528-line setup wizard. Follow its structure: banner, prerequisites check, idempotent operations, error handling, colored output, `--auto` mode support.
  - `deploy/deploy-infra.sh` — Infrastructure deployment patterns: Docker install, service start, health check waiting.

  **API/Type References**:
  - `docker-compose.selfhosted.yml` — The compose file to start (from T8)
  - `configs/Caddyfile.selfhosted` — The Caddy config to mount (from T9)
  - `deploy/scripts/generate-certs.sh` — Cert generation script to call (from T11)

  **WHY Each Reference Matters**:
  - `container-setup.sh`: The canonical deploy script — follow its structure exactly for consistency.
  - The T8/T9/T11 deliverables: This script orchestrates them all together.

  **Acceptance Criteria**:

  **QA Scenarios (MANDATORY):**

  ```
  Scenario: deploy-selfhosted.sh dry-run validates prerequisites
    Tool: Bash
    Preconditions: Script created, Docker installed
    Steps:
      1. bash deploy/deploy-selfhosted.sh --dry-run 2>&1
      2. echo $?
    Expected Result: Exit code 0, lists prerequisites and planned actions
    Failure Indicators: Missing --dry-run support, fails on prerequisite check
    Evidence: .sisyphus/evidence/task-10-deploy-dry-run.txt

  Scenario: Script detects LAN-only mode correctly
    Tool: Bash
    Preconditions: No public IP configured
    Steps:
      1. bash deploy/deploy-selfhosted.sh --auto --dry-run 2>&1 | grep -i "LAN\|local\|private"
    Expected Result: Output mentions LAN/local/private mode detection
    Failure Indicators: Script assumes public domain is needed
    Evidence: .sisyphus/evidence/task-10-lan-detection.txt
  ```

  **Commit**: YES
  - Message: `feat(deploy): add self-hosted appliance deployment script`
  - Files: `deploy/deploy-selfhosted.sh`
  - Pre-commit: `bash -n deploy/deploy-selfhosted.sh`

- [x] 11. Self-Signed Certificate Automation

  **What to do**:
  - Create `deploy/scripts/generate-certs.sh`:
    1. Generate CA key + cert (ECDSA P-256, 10-year validity)
    2. Generate server key + cert signed by CA (ECDSA P-256, 1-year validity)
    3. Add SANs: hostname, `armorclaw.local`, LAN IP (auto-detected), `localhost`
    4. Output to `/etc/armorclaw/certs/`: `ca.crt`, `server.crt`, `server.key`
    5. Set permissions: keys readable by armorclaw user only
    6. Print CA fingerprint for Android provisioning
    7. Print CA cert in PEM format (for QR code generation)
  - Follow the patterns from `bridge/pkg/setup/ssl.go` (Go self-signed cert generation):
    - Same ECDSA P-256 curve
    - Same SAN format
    - Same validity periods
  - Add cert rotation reminder (log expiry date)
  - Make callable from `deploy-selfhosted.sh` (T10)
  - Add cert rotation support:
    - Log expiry date prominently at generation time
    - Support `--rotate` flag that regenerates server cert (keeps CA)
    - Print renewal command: `deploy/scripts/generate-certs.sh --rotate --output /etc/armorclaw/certs/`
    - When run with `--rotate`: loads existing CA key, generates new server cert only, prints diff of expiry dates
    - Add comment in Caddyfile.selfhosted referencing rotation procedure

  **Must NOT do**:
  - Do NOT generate RSA keys (use ECDSA P-256 like ssl.go)
  - Do NOT install certs system-wide (only for armorclaw services)
  - Do NOT require interactive input

  **Recommended Agent Profile**:
  - **Category**: `unspecified-high`
    - Reason: Certificate generation with correct SANs, permissions, security
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES (with T8, T9)
  - **Parallel Group**: Wave 3
  - **Blocks**: T10 (deploy script calls this)
  - **Blocked By**: None

  **References**:

  **Pattern References**:
  - `bridge/pkg/setup/ssl.go` — Go self-signed cert generation: ECDSA P-256, SAN format, validity period. Replicate this EXACTLY in bash (openssl commands).
  - `bridge/pkg/setup/ssl_test.go` — Tests showing expected cert properties

  **API/Type References**:
  - `container/openclaw-src/src/config/types.gateway.ts` — `GatewayTlsConfig` with `autoGenerate`, `certPath`, `keyPath`, `caPath`. The cert paths must match these config options.

  **External References**:
  - OpenSSL ECDSA cert generation: `openssl ecparam -genkey -name prime256v1 | openssl req -new -x509 ...`

  **WHY Each Reference Matters**:
  - `ssl.go`: The authoritative cert generation pattern — replicate its key type, SANs, and validity in bash.
  - `types.gateway.ts`: The OpenClaw gateway expects certs at specific paths — generate to those paths.

  **Acceptance Criteria**:

  **QA Scenarios (MANDATORY):**

  ```
  Scenario: Cert generation produces valid CA + server certs
    Tool: Bash
    Preconditions: openssl installed
    Steps:
      1. bash deploy/scripts/generate-certs.sh --output /tmp/test-certs/
      2. openssl x509 -in /tmp/test-certs/ca.crt -text -noout | grep -c "CA:TRUE"
      3. openssl x509 -in /tmp/test-certs/server.crt -text -noout | grep -c "CA:FALSE"
      4. openssl verify -CAfile /tmp/test-certs/ca.crt /tmp/test-certs/server.crt
    Expected Result: CA cert has CA:TRUE, server cert has CA:FALSE, verify returns OK
    Failure Indicators: Verification fails, wrong key usage, missing CA flag
    Evidence: .sisyphus/evidence/task-11-cert-generation.txt

  Scenario: Server cert has correct SANs for LAN
    Tool: Bash
    Preconditions: Certs generated
    Steps:
      1. openssl x509 -in /tmp/test-certs/server.crt -text -noout | grep -A5 "Subject Alternative Name"
    Expected Result: SANs include: armorclaw.local, localhost, and LAN IP
    Failure Indicators: Missing SANs, only has CN without SANs
    Evidence: .sisyphus/evidence/task-11-cert-sans.txt

  Scenario: Key permissions are restrictive
    Tool: Bash
    Preconditions: Certs generated
    Steps:
      1. stat -c "%a" /tmp/test-certs/server.key
    Expected Result: Permissions are 600 or 640 (not world-readable)
    Failure Indicators: Permissions 644 or 666 (keys readable by anyone)
    Evidence: .sisyphus/evidence/task-11-cert-permissions.txt
  ```

  **Commit**: YES
  - Message: `feat(deploy): add self-signed certificate generation for self-hosted mode`
  - Files: `deploy/scripts/generate-certs.sh`
  - Pre-commit: `bash -n deploy/scripts/generate-certs.sh && bash deploy/scripts/generate-certs.sh --output /tmp/test-certs/`

- [x] 12. Documentation Updates

  **What to do**:
  - Update root `README.md`:
    - Add **"Self-Hosted"** as a new deployment mode in the Deployment Modes table (alongside Native, Sentinel, Cloudflare)
    - Add quick-start command: `deploy/deploy-selfhosted.sh --auto`
    - Document the email pipeline: Postfix → mta-recv → IngestServer → YARA → PII → Secretary workflow
  - Create or update `deploy/README.md`:
    - Document all deployment modes: Native / Sentinel / Cloudflare / Self-Hosted
    - Explain when to use each mode
    - Link to specific compose files and deploy scripts
  - Update `doc/armorclaw.md`:
    - Add email deployment section (currently marked "planned")
    - Update self-hosted section (currently minimal)
    - Add deploy/postfix/ file references

  **Must NOT do**:
  - Do NOT modify ArmorChat docs (separate codebase)
  - Do NOT add features that don't exist yet (document only what's implemented)

  **Recommended Agent Profile**:
  - **Category**: `writing`
    - Reason: Documentation task, needs clear technical writing
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES (with any Wave 3 task)
  - **Parallel Group**: Wave 3
  - **Blocks**: F1-F4
  - **Blocked By**: T1, T1b, T4-T11 (document what's actually built)

  **References**:

  **Pattern References**:
  - `README.md` — Current Deployment Modes section. Add Self-Hosted as 4th mode.
  - `doc/armorclaw.md` — Master architecture doc. Has "planned" markers for email.

  **Acceptance Criteria**:

  **QA Scenarios (MANDATORY):**

  ```
  Scenario: README lists Self-Hosted as deployment mode
    Tool: Bash
    Preconditions: README.md updated
    Steps:
      1. grep -c "Self-Hosted\|selfhosted\|self-hosted" README.md
    Expected Result: >= 3 matches (table + section + command)
    Failure Indicators: Zero matches (self-hosted mode not documented)
    Evidence: .sisyphus/evidence/task-12-readme-selfhosted.txt

  Scenario: deploy/README.md documents all modes
    Tool: Bash
    Preconditions: deploy/README.md created
    Steps:
      1. grep -c "Native\|Sentinel\|Cloudflare\|Self-Hosted" deploy/README.md
    Expected Result: >= 4 matches (all four modes mentioned)
    Failure Indicators: Missing modes
    Evidence: .sisyphus/evidence/task-12-deploy-readme.txt
  ```

  **Commit**: YES
  - Message: `docs: add self-hosted mode and email pipeline documentation`
  - Files: `README.md`, `deploy/README.md`, `doc/armorclaw.md`

---

## Final Verification Wave (MANDATORY — after ALL implementation tasks)

> 4 review agents run in PARALLEL. ALL must APPROVE. Present consolidated results to user and get explicit "okay" before completing.

- [x] F1. **Plan Compliance Audit** — `oracle`
  Read the plan end-to-end. For each "Must Have": verify implementation exists (read file, curl endpoint, run command). For each "Must NOT Have": search codebase for forbidden patterns — reject with file:line if found. Check evidence files exist in .sisyphus/evidence/. Compare deliverables against plan.
  Output: `Must Have [N/N] | Must NOT Have [N/N] | Tasks [N/N] | VERDICT: APPROVE/REJECT`

- [x] F2. **Code Quality Review** — `unspecified-high`
  Run `go build ./...` + `go vet ./...` in bridge/. Run `cargo build --release` + `cargo test --lib` + `cargo clippy` in sidecar/. Review all changed files for: empty error handling, hardcoded secrets, commented-out code, unused imports. Check bash scripts with shellcheck.
  Output: `Go Build [PASS/FAIL] | Rust Build [PASS/FAIL] | Rust Tests [N pass/N fail] | Shellcheck [N issues] | VERDICT`

- [x] F3. **Real Manual QA** — `unspecified-high`
  Start from clean state. Execute EVERY QA scenario from EVERY task — follow exact steps, capture evidence. Test cross-task integration: email flows through pipeline to secretary workflow, self-hosted stack starts. Test edge cases: missing config, invalid email, expired cert. Save to `.sisyphus/evidence/final-qa/`.
  
  **VPS-HARDWARE-REQUIRED scenarios** (deferred — mark as "VPS-REQUIRED" with manual test checklist):
  - T5 full `install.sh` on bare metal with real Postfix
  - T6 end-to-end email test with real Postfix delivery
  - T10 `deploy-selfhosted.sh --auto` full stack on VPS
  - T1 last QA scenario (end-to-end with YARA + PII masking) — requires bridge binary + mta-recv on host, but does NOT require Postfix (can run standalone)
  
  **MUST be executed by agent** (all others): T1 socket start/stop tests, T1b, T2, T3, T4 (config validation only), T7, T8, T9, T11, T12.
  
  Output: `Scenarios [N/N pass] | Integration [N/N] | Edge Cases [N tested] | VPS-Required [N deferred] | VERDICT`

- [x] F4. **Scope Fidelity Check** — `deep`
  For each task: read "What to do", read actual diff (git log/diff). Verify 1:1 — everything in spec was built (no missing), nothing beyond spec was built (no creep). Check "Must NOT do" compliance. Detect cross-task contamination. Flag unaccounted changes.
  Output: `Tasks [N/N compliant] | Contamination [CLEAN/N issues] | Unaccounted [CLEAN/N files] | VERDICT`

---

## Commit Strategy

- **T1**: `feat(bridge): wire IngestServer and YARA into bridge startup` — bridge/cmd/bridge/main.go, bridge/cmd/bridge/setup_email.go
- **T1b**: `feat(bridge): wire EmailDispatcher to EventBus for email-triggered workflows` — bridge/cmd/bridge/setup_email.go
- **T2**: `fix(sidecar): add cmake + clang build env for release builds` — sidecar/.cargo/config.toml or build script
- **T3**: `docs(sidecar): update README with correct test counts and build instructions` — sidecar/README.md
- **T4**: `feat(deploy): add Postfix configuration files for email ingestion` — deploy/postfix/main.cf, master.cf, transport_maps
- **T5**: `feat(deploy): add Postfix install script with mta-recv setup` — deploy/postfix/install.sh
- **T6**: `feat(deploy): add Postfix verification script with health checks` — deploy/postfix/verify-setup.sh
- **T7**: `ci(sidecar): add GitHub Actions workflow for sidecar build + test` — .github/workflows/sidecar.yml
- **T8**: `feat(deploy): add self-hosted Docker Compose for single-VPS appliance` — docker-compose.selfhosted.yml
- **T9**: `feat(config): add Caddyfile for self-hosted internal-only TLS` — configs/Caddyfile.selfhosted
- **T10**: `feat(deploy): add self-hosted appliance deployment script` — deploy/deploy-selfhosted.sh
- **T11**: `feat(deploy): add self-signed cert generation with rotation for self-hosted mode` — deploy/scripts/generate-certs.sh
- **T12**: `docs: add self-hosted mode and email pipeline documentation` — README.md, deploy/README.md, doc/armorclaw.md

---

## Success Criteria

### Verification Commands
```bash
# Email pipeline (end-to-end)
deploy/postfix/verify-setup.sh                                    # Expected: all checks pass
go test ./bridge/pkg/email/... -count=1                           # Expected: all pass

# Email → Secretary workflow
echo '{"jsonrpc":"2.0","id":1,"method":"task.list"}' | socat - UNIX-CONNECT:/run/armorclaw/bridge.sock  # Expected: tasks after email (adjust socket path if different in config)

# Sidecar
cd sidecar && CC=clang CXX=clang++ cargo build --release         # Expected: success, 0 errors
cd sidecar && cargo test --lib                                    # Expected: 252 passed, 8 ignored
cd sidecar && cargo clippy -- -D warnings                         # Expected: 0 errors

# Self-hosted
docker compose -f docker-compose.selfhosted.yml config            # Expected: valid config
deploy/deploy-selfhosted.sh --dry-run                             # Expected: dry-run passes
openssl x509 -in /etc/armorclaw/certs/server.crt -text | head    # Expected: valid cert with SANs

# Bridge with email
go build ./bridge/cmd/bridge/                                     # Expected: success
go test ./bridge/... -count=1 -run Ingest                         # Expected: ingest tests pass

# Documentation
grep -c "Self-Hosted" README.md                                   # Expected: >= 3
```

### Final Checklist
- [ ] All "Must Have" present
- [ ] All "Must NOT Have" absent
- [ ] IngestServer starts on bridge launch
- [ ] YARA initialized (attachments scanned)
- [ ] EmailDispatcher subscribes to EventBus
- [ ] Email flows Postfix → mta-recv → IngestServer → EmailDispatcher → Secretary workflow
- [ ] Sidecar release build succeeds with cmake + clang
- [ ] Self-hosted stack starts on single VPS
- [ ] Both mDNS service types advertised
- [ ] Documentation updated for all deployment modes
