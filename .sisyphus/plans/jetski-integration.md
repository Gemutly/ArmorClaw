# Jetski Browser Sidecar Integration into ArmorClaw

## TL;DR

> **Quick Summary**: Copy the jetski browser sidecar (Go CDP proxy + Lightpanda engine) into ArmorClaw, fix 4 wiring gaps, implement full Tethered Mode security (SQLCipher sessions, Matrix HITL approval, active PII scrubbing), and integrate all 3 sub-projects (Core, Lighthouse, Chartmaker) into the monorepo with TDD.
> 
> **Deliverables**:
> - `jetski/` — Standalone Go module with working CDP proxy, Translator, PII Scanner, RPC API
> - `jetski/lighthouse/` — Nav-Chart REST API sub-project
> - `jetski/jetski-chartmaker/` — TypeScript CLI for recording browser interactions
> - `docker-compose.jetski.yml` — Docker integration with ArmorClaw network topology
> - SQLCipher session encryption replacing `age`
> - Matrix HITL approval client for browser operations
> - Active PII scrubbing in CDP proxy flow
> - `go.work` for multi-module Go workspace
> 
> **Estimated Effort**: Large
> **Parallel Execution**: YES - 6 waves
> **Critical Path**: T1 (copy core) → T5 (rename module) → T9 (verify build) → T11 (wire Translator) → T15 (SQLCipher sessions) → T20 (e2e test) → F1-F4

---

## Context

### Original Request
"copy the jetski into current working directory and adapt this browser sidecar for armorclaw"

### Interview Summary
**Key Discussions**:
- **Module structure**: Standalone `jetski/` with own `go.mod` — NOT merged into bridge module
- **Sub-projects**: ALL THREE — Core (Observer Shield), Lighthouse (Nav-Chart API), Chartmaker (TypeScript CLI)
- **Wiring gaps**: Fix ALL — Translator unwired from router, PII Scanner orphaned, port 8080 RPC empty, Sonar telemetry dead code
- **Tethered Mode**: FULL implementation — Replace `age` with SQLCipher sessions, HITL approval via Matrix, active PII scrubbing
- **Test strategy**: TDD — failing tests before implementation for every module

**Research Findings**:
- Jetski is a Go project (18 core source files) acting as CDP WebSocket proxy between AI agents (port 9222) and Lightpanda engine (port 9223)
- Core uses NO CGO (pure Go + gorilla/websocket) — clean compilation
- Lighthouse uses CGO (`mattn/go-sqlite3`) — separate module, separate build
- Session encryption uses `filippo.io/age` — must be replaced with SQLCipher for Tethered Mode
- 4 wiring gaps discovered: Translator no-op stubs in router, PII scanner discarded in main.go, no RPC handlers on port 8080, Sonar telemetry never imported
- Lightpanda Dockerfile SHA256 is a placeholder — must resolve before any build

### Metis Review
**Critical Findings** (all addressed in plan):
- **Port 8080 conflict**: Bridge already owns 8080 in Sentinel mode → Jetski RPC remapped to port 9223
- **No "browser" network**: docker-compose has no browser network → New `browser-net` (172.23) created in docker-compose.jetski.yml
- **Go version fragmentation**: Core 1.24, Bridge 1.25, Lighthouse 1.26 → Standardize on Go 1.25
- **browser-service/ coexistence**: ArmorClaw already has `browser-service/` (TypeScript/Playwright) → Jetski COMPLEMENTS it (CDP-level control vs Playwright automation)
- **Lighthouse SQLite driver**: Uses `mattn/go-sqlite3` (CGO) while bridge uses `go-sqlcipher/v4` → Keep separate for now; both already require CGO

**Guardrails Applied**:
- Copy-as-is FIRST (commit), THEN modify (separate commits) — never interleave
- Use `go.work` for multi-module workspace
- Pin real Lightpanda SHA256 before any Dockerfile modification
- PII scanner patterns locked to existing 4 (SSN, credit card, email, password)
- No new Matrix event types — reuse existing bridge approval patterns
- No changes to existing `browser-service/`, `bridge/`, `sidecar/`, or proto files

---

## Work Objectives

### Core Objective
Integrate the jetski browser sidecar as a standalone Go module within ArmorClaw, fixing all wiring gaps and implementing full Tethered Mode security (SQLCipher sessions, HITL Matrix approval, active PII scrubbing), with TDD coverage.

### Concrete Deliverables
- `jetski/` directory with working CDP proxy and all sub-projects
- `go.work` file linking bridge + jetski + lighthouse modules
- `docker-compose.jetski.yml` with browser-net network, port mappings, resource limits
- SQLCipher session store replacing `age` encryption
- Matrix HITL approval client for browser session operations
- Active PII scrubbing in CDP proxy message flow
- Working Translator, PII Scanner, RPC API, Sonar telemetry

### Definition of Done
- [ ] `go build ./jetski/...` compiles cleanly for all modules
- [ ] `go test ./jetski/...` passes all tests (Core + Lighthouse)
- [ ] `docker compose up jetski` starts and responds to health check
- [ ] CDP proxy forwards messages through Translator with PII scrubbing
- [ ] Sessions encrypted with SQLCipher, not `age`
- [ ] HITL approval requested via Matrix for sensitive browser operations
- [ ] Lighthouse Nav-Chart API responds to HTTP requests
- [ ] Chartmaker CLI builds and runs

### Must Have
- All 3 sub-projects copied and compiling
- All 4 wiring gaps fixed (Translator, PII Scanner, RPC API, Sonar)
- SQLCipher session encryption (replacing `age`)
- Matrix HITL approval client
- Active PII scrubbing in CDP proxy
- Docker integration with correct network topology
- TDD: failing test → implementation → passing test for every wiring/security fix
- `go.work` multi-module workspace

### Must NOT Have (Guardrails)
- NO changes to existing `browser-service/` directory
- NO changes to `bridge/` go.mod, proto files, or existing RPCs
- NO changes to `sidecar/` Rust code
- NO modification to existing SQLCipher keystore
- NO new Matrix event types — reuse existing bridge approval patterns
- NO expansion of PII scanner beyond 4 existing patterns (SSN, CC, email, password)
- NO bypass of Matrix as control plane for approval flows
- NO removal of existing `age` dependency until SQLCipher replacement is complete and tested
- NO copying of `node_modules/`, `.git/`, binary artifacts, or `go.sum` (regenerate)
- NO interleaving of copy and adaptation commits — copy first, then modify
- NO CGO introduction into Core module (Core stays pure Go)

---

## Verification Strategy (MANDATORY)

> **ZERO HUMAN INTERVENTION** — ALL verification is agent-executed. No exceptions.
> Acceptance criteria requiring "user manually tests/confirms" are FORBIDDEN.

### Test Decision
- **Infrastructure exists**: YES (Go `testing` package, jetski has existing test files)
- **Automated tests**: TDD — failing tests before implementation
- **Framework**: Go `testing` package + `stretchr/testify` (if already in jetski deps)

### QA Policy
Every task MUST include agent-executed QA scenarios.
Evidence saved to `.sisyphus/evidence/task-{N}-{scenario-slug}.{ext}`.

- **Go modules**: Use Bash — `go build`, `go test`, `go vet`
- **Docker containers**: Use Bash — `docker compose up`, `curl` health checks
- **CDP proxy**: Use Bash — WebSocket connection via `websocat` or Go test client
- **PII scrubbing**: Use Bash (Go test) — verify scrubbed output for known PII inputs

---

## Execution Strategy

### Parallel Execution Waves

```
Wave 1 (Copy & Foundation — 4 parallel tasks):
├── T1: Copy jetski core source tree [quick]
├── T2: Copy lighthouse sub-project [quick]
├── T3: Copy chartmaker sub-project [quick]
└── T4: Create go.work multi-module workspace [quick]

Wave 2 (Module Setup — 4 tasks, Wave 1 complete):
├── T5: Rename core module path + all imports [unspecified-high]
├── T6: Fix Dockerfile — real Lightpanda SHA256 + port remap [quick]
├── T7: Create docker-compose.jetski.yml [unspecified-high]
└── T8: Integrate jetski compose into main docker-compose.yml [quick]

Wave 3 (Verify Foundation — 2 tasks, Wave 2 complete):
├── T9: Verify clean compilation all 3 Go modules [quick]
└── T10: Verify jetski container starts + health check [unspecified-high]

Wave 4 (Wiring Gaps — 4 parallel tasks, Wave 3 complete, TDD):
├── T11: Wire Translator into router handlers [deep]
├── T12: Wire PII Scanner into CDP proxy flow [deep]
├── T13: Add RPC API handlers on port 9223 [unspecified-high]
└── T14: Wire Sonar telemetry into main flow [quick]

Wave 5 (Tethered Mode — 3 parallel tasks, Wave 4 complete, TDD):
├── T15: Replace age with SQLCipher session store [deep]
├── T16: Active PII scrubbing in CDP proxy [deep]
└── T17: Matrix HITL approval client [deep]

Wave 6 (Sub-project Integration + E2E — 3 tasks, Wave 5 complete):
├── T18: Lighthouse Docker build + API verification [unspecified-high]
├── T19: Chartmaker npm build + CLI verification [quick]
└── T20: End-to-end Tethered mode integration test [deep]

Wave FINAL (4 parallel reviews):
├── F1: Plan compliance audit [oracle]
├── F2: Code quality review [unspecified-high]
├── F3: Real manual QA [unspecified-high]
└── F4: Scope fidelity check [deep]

Critical Path: T1 → T5 → T9 → T11 → T15 → T20 → F1-F4
Parallel Speedup: ~65% faster than sequential
Max Concurrent: 4 (Waves 1, 4, 5)
```

### Dependency Matrix

| Task | Depends On | Blocks | Wave |
|------|-----------|--------|------|
| T1 | — | T5, T9 | 1 |
| T2 | — | T5, T9 | 1 |
| T3 | — | T19 | 1 |
| T4 | T1, T2 | T5, T9 | 1 |
| T5 | T1, T2, T4 | T9 | 2 |
| T6 | T1 | T10 | 2 |
| T7 | T1 | T8, T10 | 2 |
| T8 | T7 | T10 | 2 |
| T9 | T5 | T11-T14 | 3 |
| T10 | T6, T8, T9 | T11-T14 | 3 |
| T11 | T9 | T15, T16 | 4 |
| T12 | T9 | T16 | 4 |
| T13 | T9 | T17 | 4 |
| T14 | T9 | T20 | 4 |
| T15 | T11 | T20 | 5 |
| T16 | T11, T12 | T20 | 5 |
| T17 | T13 | T20 | 5 |
| T18 | T9 | T20 | 6 |
| T19 | T3 | T20 | 6 |
| T20 | T15-T19 | F1-F4 | 6 |
| F1-F4 | T20 | — | FINAL |

### Agent Dispatch Summary

- **Wave 1**: 4 tasks — T1-T3 → `quick`, T4 → `quick`
- **Wave 2**: 4 tasks — T5 → `unspecified-high`, T6 → `quick`, T7 → `unspecified-high`, T8 → `quick`
- **Wave 3**: 2 tasks — T9 → `quick`, T10 → `unspecified-high`
- **Wave 4**: 4 tasks — T11 → `deep`, T12 → `deep`, T13 → `unspecified-high`, T14 → `quick`
- **Wave 5**: 3 tasks — T15 → `deep`, T16 → `deep`, T17 → `deep`
- **Wave 6**: 3 tasks — T18 → `unspecified-high`, T19 → `quick`, T20 → `deep`
- **FINAL**: 4 tasks — F1 → `oracle`, F2 → `unspecified-high`, F3 → `unspecified-high`, F4 → `deep`

---

## TODOs

- [x] 1. Copy jetski core source tree

  **What to do**:
  - Copy `/home/mink/src/jetski/` contents into `/home/mink/src/armorclaw-omo/jetski/` — Core only (exclude `lighthouse/`, `jetski-chartmaker/`, `node_modules/`, `.git/`, binary artifacts)
  - Include: `cmd/`, `internal/`, `pkg/`, `configs/`, `Dockerfile`, `docker-compose.yml`, `go.mod`, `go.sum`, `README.md`, `LICENSE`, `.gitignore`
  - Exclude: `lighthouse/`, `jetski-chartmaker/`, `node_modules/`, `.git/`, `*.bin`, `*.exe`, `lightpanda` binary
  - Verify: `ls jetski/cmd/observer/main.go` exists after copy

  **Must NOT do**:
  - Do NOT modify any file contents during copy
  - Do NOT copy lighthouse or chartmaker (separate tasks)
  - Do NOT copy node_modules or .git

  **Recommended Agent Profile**:
  - **Category**: `quick`
    - Reason: Pure file copy operation, no code changes
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 1 (with T2, T3, T4)
  - **Blocks**: T5 (rename module), T6 (fix Dockerfile), T9 (verify build)
  - **Blocked By**: None

  **References**:
  **Pattern References**:
  - `/home/mink/src/jetski/` — Source directory to copy from. Contains: `cmd/`, `internal/`, `pkg/`, `configs/`, `Dockerfile`, `go.mod`, `go.sum`, `README.md`
  - `/home/mink/src/jetski/.gitignore` — Should be copied to preserve build ignore patterns

  **Acceptance Criteria**:
  - [ ] `jetski/cmd/observer/main.go` exists
  - [ ] `jetski/internal/cdp/proxy.go` exists
  - [ ] `jetski/internal/security/pii_scanner.go` exists
  - [ ] `jetski/pkg/config/config.go` exists
  - [ ] `jetski/go.mod` exists with module name `jetski-browser`
  - [ ] `jetski/Dockerfile` exists
  - [ ] `jetski/lighthouse/` does NOT exist (separate task)
  - [ ] `jetski/jetski-chartmaker/` does NOT exist (separate task)
  - [ ] No `.git/` directory inside `jetski/`

  **QA Scenarios (MANDATORY)**:
  ```
  Scenario: Verify core source tree structure
    Tool: Bash
    Preconditions: Copy operation completed
    Steps:
      1. Run: find jetski/ -name "*.go" -not -path "*/lighthouse/*" -not -path "*/jetski-chartmaker/*" | wc -l
      2. Assert: count is 18 (the 18 Go source files in Core)
      3. Run: ls jetski/cmd/observer/main.go
      4. Assert: file exists (exit code 0)
      5. Run: test ! -d jetski/lighthouse
      6. Assert: lighthouse directory does NOT exist (exit code 0)
    Expected Result: 18 Go files present, no lighthouse/chartmaker directories
    Evidence: .sisyphus/evidence/task-1-copy-verification.txt

  Scenario: Verify no binary artifacts or .git copied
    Tool: Bash
    Preconditions: Copy operation completed
    Steps:
      1. Run: test ! -d jetski/.git && echo "clean" || echo "dirty"
      2. Assert: output is "clean"
      3. Run: find jetski/ -name "node_modules" -type d
      4. Assert: no output (no node_modules)
      5. Run: find jetski/ -name "*.bin" -o -name "*.exe" -o -name "lightpanda"
      6. Assert: no output (no binary artifacts)
    Expected Result: No .git, node_modules, or binary artifacts
    Evidence: .sisyphus/evidence/task-1-no-artifacts.txt
  ```

  **Commit**: YES
  - Message: `chore: copy jetski core source tree`
  - Files: `jetski/` (core only)
  - Pre-commit: none (raw copy)

- [x] 2. Copy lighthouse sub-project

  **What to do**:
  - Copy `/home/mink/src/jetski/lighthouse/` into `/home/mink/src/armorclaw-omo/jetski/lighthouse/`
  - Include: all `.go` files, `go.mod`, `go.sum`, any config files, SQL migration files
  - Exclude: `.git/`, any SQLite database files (`*.db`, `*.sqlite`), binary artifacts
  - Lighthouse already has module path `github.com/armorclaw/lighthouse` (pre-renamed)
  - Verify: `ls jetski/lighthouse/go.mod` exists and contains `module github.com/armorclaw/lighthouse`

  **Must NOT do**:
  - Do NOT modify any file contents during copy
  - Do NOT change the module path (already correct)
  - Do NOT copy SQLite database files

  **Recommended Agent Profile**:
  - **Category**: `quick`
    - Reason: Pure file copy operation
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 1 (with T1, T3, T4)
  - **Blocks**: T9 (verify build), T18 (lighthouse integration)
  - **Blocked By**: None

  **References**:
  **Pattern References**:
  - `/home/mink/src/jetski/lighthouse/` — Source directory. Contains: `cmd/`, `internal/`, `go.mod` (module `github.com/armorclaw/lighthouse`), `go.sum`

  **Acceptance Criteria**:
  - [ ] `jetski/lighthouse/go.mod` exists
  - [ ] `jetski/lighthouse/go.mod` contains `module github.com/armorclaw/lighthouse`
  - [ ] `jetski/lighthouse/cmd/server/main.go` exists (or equivalent entry point)
  - [ ] No `.db` or `.sqlite` files in `jetski/lighthouse/`

  **QA Scenarios (MANDATORY)**:
  ```
  Scenario: Verify lighthouse source tree
    Tool: Bash
    Preconditions: Copy completed
    Steps:
      1. Run: cat jetski/lighthouse/go.mod | head -1
      2. Assert: output is "module github.com/armorclaw/lighthouse"
      3. Run: find jetski/lighthouse/ -name "*.go" | wc -l
      4. Assert: count > 0 (Go files exist)
      5. Run: find jetski/lighthouse/ -name "*.db" -o -name "*.sqlite"
      6. Assert: no output (no database files)
    Expected Result: Lighthouse module with correct path, Go files present, no databases
    Evidence: .sisyphus/evidence/task-2-lighthouse-verification.txt
  ```

  **Commit**: YES
  - Message: `chore: copy lighthouse sub-project`
  - Files: `jetski/lighthouse/`
  - Pre-commit: none

- [x] 3. Copy chartmaker sub-project

  **What to do**:
  - Copy `/home/mink/src/jetski/jetski-chartmaker/` into `/home/mink/src/armorclaw-omo/jetski/jetski-chartmaker/`
  - Include: `package.json`, `tsconfig.json`, `src/`, any config files
  - Exclude: `node_modules/`, `.git/`, `dist/`, `package-lock.json` (will regenerate)
  - Chartmaker already uses `@armorclaw/jetski-chartmaker` in package.json
  - Verify: `ls jetski/jetski-chartmaker/package.json` exists

  **Must NOT do**:
  - Do NOT copy `node_modules/` or `dist/`
  - Do NOT run `npm install` (separate task)
  - Do NOT modify package.json contents

  **Recommended Agent Profile**:
  - **Category**: `quick`
    - Reason: Pure file copy operation
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 1 (with T1, T2, T4)
  - **Blocks**: T19 (chartmaker build)
  - **Blocked By**: None

  **References**:
  **Pattern References**:
  - `/home/mink/src/jetski/jetski-chartmaker/` — Source directory. TypeScript CLI with `package.json`, `tsconfig.json`, `src/`

  **Acceptance Criteria**:
  - [ ] `jetski/jetski-chartmaker/package.json` exists
  - [ ] `jetski/jetski-chartmaker/package.json` contains `@armorclaw/jetski-chartmaker`
  - [ ] `jetski/jetski-chartmaker/src/` exists with `.ts` files
  - [ ] `jetski/jetski-chartmaker/node_modules/` does NOT exist
  - [ ] `jetski/jetski-chartmaker/dist/` does NOT exist

  **QA Scenarios (MANDATORY)**:
  ```
  Scenario: Verify chartmaker source tree
    Tool: Bash
    Preconditions: Copy completed
    Steps:
      1. Run: cat jetski/jetski-chartmaker/package.json | grep '"name"'
      2. Assert: output contains "@armorclaw/jetski-chartmaker"
      3. Run: test ! -d jetski/jetski-chartmaker/node_modules && echo "clean" || echo "dirty"
      4. Assert: output is "clean"
      5. Run: find jetski/jetski-chartmaker/src/ -name "*.ts" | wc -l
      6. Assert: count > 0
    Expected Result: Chartmaker package with correct name, no node_modules, TypeScript sources present
    Evidence: .sisyphus/evidence/task-3-chartmaker-verification.txt
  ```

  **Commit**: YES
  - Message: `chore: copy chartmaker sub-project`
  - Files: `jetski/jetski-chartmaker/`
  - Pre-commit: none

- [x] 4. Create go.work for multi-module workspace

  **What to do**:
  - Create `go.work` at repo root linking all Go modules
  - Include: `bridge/`, `jetski/`, `jetski/lighthouse/`
  - Follow Go workspace convention: `go work init ./bridge ./jetski ./jetski/lighthouse`
  - Regenerate `go.work.sum` if needed
  - Verify: `go list ./jetski/...` works from repo root

  **Must NOT do**:
  - Do NOT modify any existing `go.mod` files
  - Do NOT add `replace` directives in individual go.mod files (go.work handles this)
  - Do NOT include `jetski/jetski-chartmaker/` (TypeScript, not Go)

  **Recommended Agent Profile**:
  - **Category**: `quick`
    - Reason: Single file creation following Go workspace pattern
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES (but needs T1 and T2 completed for verification)
  - **Parallel Group**: Wave 1 (starts with T1-T3, verifies after they complete)
  - **Blocks**: T5 (module rename), T9 (verify build)
  - **Blocked By**: T1 (core must exist), T2 (lighthouse must exist)

  **References**:
  **Pattern References**:
  - `bridge/go.mod` — Existing Go module at `github.com/armorclaw/bridge` (Go 1.25.0). Reference for Go version and module path format.
  - `/home/mink/src/jetski/go.mod` — Module `jetski-browser` (Go 1.24). Will be renamed in T5.
  - `/home/mink/src/jetski/lighthouse/go.mod` — Module `github.com/armorclaw/lighthouse` (Go 1.26). Already correctly named.

  **Acceptance Criteria**:
  - [ ] `go.work` exists at repo root
  - [ ] `go.work` contains entries for `bridge/`, `jetski/`, `jetski/lighthouse/`
  - [ ] `go list ./jetski/...` succeeds from repo root (may have errors until T5 rename, but go.work is syntactically valid)

  **QA Scenarios (MANDATORY)**:
  ```
  Scenario: Verify go.work structure
    Tool: Bash
    Preconditions: T1 and T2 completed (core and lighthouse copied)
    Steps:
      1. Run: cat go.work
      2. Assert: contains "use (" block
      3. Assert: contains "./bridge/"
      4. Assert: contains "./jetski/"
      5. Assert: contains "./jetski/lighthouse/"
      6. Run: go work sync
      7. Assert: exit code 0
    Expected Result: go.work with all 3 module entries, valid syntax
    Evidence: .sisyphus/evidence/task-4-gowork-verification.txt

  Scenario: Verify go.work does not include TypeScript project
    Tool: Bash
    Steps:
      1. Run: cat go.work | grep chartmaker
      2. Assert: no output (chartmaker not in go.work)
    Expected Result: No TypeScript project referenced in go.work
    Evidence: .sisyphus/evidence/task-4-gowork-no-ts.txt
  ```

  **Commit**: YES (groups with T1-T3)
  - Message: `chore: add go.work for multi-module workspace`
  - Files: `go.work`, `go.work.sum`
  - Pre-commit: `go work sync`

- [x] 5. Rename core module path + all imports

  **What to do**:
  - Change `jetski/go.mod` module name from `jetski-browser` to `github.com/armorclaw/jetski`
  - Update Go version from 1.24 to 1.25 (match bridge)
  - Find and replace ALL import paths across all `.go` files in `jetski/` (core only, NOT lighthouse which is already correct)
  - Use `ast_grep_replace` or `sed` for bulk import path renaming: `jetski-browser/` → `github.com/armorclaw/jetski/`
  - Verify with `go build ./jetski/...` after rename
  - Use `lsp_find_references` to verify no dangling imports

  **Must NOT do**:
  - Do NOT modify `jetski/lighthouse/go.mod` (already correct: `github.com/armorclaw/lighthouse`)
  - Do NOT modify `bridge/go.mod`
  - Do NOT change any functionality — this is a pure rename
  - Do NOT upgrade dependencies during rename

  **Recommended Agent Profile**:
  - **Category**: `unspecified-high`
    - Reason: Requires careful multi-file renaming with verification
  - **Skills**: []
  - **Skills Evaluated but Omitted**:
    - `git-master`: Useful but rename is Go-specific, not git-specific

  **Parallelization**:
  - **Can Run In Parallel**: NO (depends on T1, T2, T4)
  - **Parallel Group**: Sequential (Wave 2 lead task)
  - **Blocks**: T9 (verify build), T11-T14 (wiring tasks need correct imports)
  - **Blocked By**: T1 (core source), T2 (lighthouse for go.work), T4 (go.work)

  **References**:
  **Pattern References**:
  - `jetski/go.mod:1` — Current: `module jetski-browser`. Target: `module github.com/armorclaw/jetski`
  - `bridge/go.mod:1` — Reference for ArmorClaw module path convention: `module github.com/armorclaw/bridge`
  - `/home/mink/src/jetski/internal/cdp/proxy.go` — Contains imports like `jetski-browser/pkg/config`, `jetski-browser/internal/security` that need updating

  **API/Type References**:
  - Every `.go` file in `jetski/internal/` and `jetski/pkg/` — All import paths need updating

  **Acceptance Criteria**:
  - [ ] `jetski/go.mod` line 1 reads `module github.com/armorclaw/jetski`
  - [ ] `jetski/go.mod` Go version is `1.25`
  - [ ] `grep -r "jetski-browser" jetski/ --include="*.go"` returns zero results
  - [ ] `grep -r "jetski-browser" jetski/go.mod` returns zero results
  - [ ] `go build ./jetski/...` succeeds (compilation clean)

  **QA Scenarios (MANDATORY)**:
  ```
  Scenario: Verify module path rename complete
    Tool: Bash
    Preconditions: Rename completed
    Steps:
      1. Run: head -1 jetski/go.mod
      2. Assert: "module github.com/armorclaw/jetski"
      3. Run: grep -r "jetski-browser" jetski/ --include="*.go" | wc -l
      4. Assert: 0 (no old module path remaining)
      5. Run: grep -r "jetski-browser" jetski/go.mod | wc -l
      6. Assert: 0
      7. Run: grep "go 1.25" jetski/go.mod
      8. Assert: output contains "go 1.25"
    Expected Result: All references to jetski-browser eliminated, Go 1.25 set
    Evidence: .sisyphus/evidence/task-5-module-rename.txt

  Scenario: Verify compilation after rename
    Tool: Bash
    Preconditions: Rename completed
    Steps:
      1. Run: go build ./jetski/... 2>&1
      2. Assert: exit code 0, no error output
    Expected Result: Clean compilation with new module path
    Failure Indicators: "cannot find module", "package jetski-browser not found"
    Evidence: .sisyphus/evidence/task-5-build-after-rename.txt
  ```

  **Commit**: YES
  - Message: `refactor: rename jetski-browser module to github.com/armorclaw/jetski`
  - Files: `jetski/go.mod`, all `jetski/**/*.go` with import changes
  - Pre-commit: `go build ./jetski/...`

- [x] 6. Fix Dockerfile — real Lightpanda SHA256 + port remap

  **What to do**:
  - Replace placeholder SHA256 hash in `jetski/Dockerfile` with real Lightpanda v0.2.6 binary hash
  - Remap RPC port from 8080 to 9223 (bridge already owns 8080 in Sentinel mode)
  - Update `jetski/configs/config.yaml` default RPC port from 8080 to 9223
  - Update port references in `jetski/cmd/observer/main.go` if hardcoded
  - Verify Dockerfile builds without SHA256 verification failure

  **Must NOT do**:
  - Do NOT change the Lightpanda version (keep v0.2.6)
  - Do NOT change the Alpine base image
  - Do NOT modify the memory limit (keep 150MB)
  - Do NOT remove the SHA256 verification step (security requirement)

  **Recommended Agent Profile**:
  - **Category**: `quick`
    - Reason: Targeted file edits (Dockerfile + config)
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES (after T1)
  - **Parallel Group**: Wave 2 (with T5, T7, T8)
  - **Blocks**: T10 (container verification)
  - **Blocked By**: T1 (core must be copied)

  **References**:
  **Pattern References**:
  - `jetski/Dockerfile:21` — Contains placeholder: `echo "EXPECTED_SHA256_HASH  /lightpanda" | sha256sum -c -`. Replace `EXPECTED_SHA256_HASH` with real hash from Lightpanda GitHub releases.
  - `jetski/configs/config.yaml` — Default config with port settings. Find the RPC port setting and change from 8080 to 9223.
  - `jetski/cmd/observer/main.go:78-106` — HTTP mux setup. Check for hardcoded port references.
  - `docker-compose.bridge.yml` — Reference for port mapping patterns in ArmorClaw.

  **External References**:
  - Lightpanda releases: `https://github.com/nicholasgasior/lightpanda/releases` — Source for v0.2.6 binary SHA256

  **Acceptance Criteria**:
  - [ ] `jetski/Dockerfile` contains a 64-character hex SHA256 hash (not "EXPECTED_SHA256_HASH")
  - [ ] `grep "8080" jetski/Dockerfile` returns no results (RPC port remapped)
  - [ ] `grep "9223" jetski/Dockerfile` returns results (new RPC port)
  - [ ] `jetski/configs/config.yaml` reflects port 9223 for RPC

  **QA Scenarios (MANDATORY)**:
  ```
  Scenario: Verify SHA256 is real hash
    Tool: Bash
    Preconditions: Dockerfile updated
    Steps:
      1. Run: grep -o '[0-9a-f]\{64\}' jetski/Dockerfile | head -1
      2. Assert: output is a 64-character hex string
      3. Run: grep "EXPECTED_SHA256_HASH" jetski/Dockerfile
      4. Assert: exit code 1 (no placeholder found)
    Expected Result: Real SHA256 hash, no placeholder
    Evidence: .sisyphus/evidence/task-6-sha256-verification.txt

  Scenario: Verify port remap
    Tool: Bash
    Steps:
      1. Run: grep -n "8080" jetski/Dockerfile jetski/configs/config.yaml
      2. Assert: exit code 1 (no 8080 references)
      3. Run: grep -n "9223" jetski/configs/config.yaml
      4. Assert: exit code 0 (9223 present)
    Expected Result: All 8080 references replaced with 9223
    Evidence: .sisyphus/evidence/task-6-port-remap.txt
  ```

  **Commit**: YES
  - Message: `fix: pin Lightpanda SHA256 + remap RPC port to 9223`
  - Files: `jetski/Dockerfile`, `jetski/configs/config.yaml`, `jetski/cmd/observer/main.go` (if changed)
  - Pre-commit: none (Dockerfile can't be pre-commit verified without Docker build)

- [x] 7. Create docker-compose.jetski.yml

  **What to do**:
  - Create `docker-compose.jetski.yml` in repo root following ArmorClaw's meta-composition pattern
  - Define `jetski` service with:
    - Image: built from `jetski/Dockerfile`
    - Networks: new `browser-net` (172.23.0.0/16) + `bridge-net` (for Matrix communication)
    - Ports: 9222:9222 (CDP), 9223:9223 (RPC)
    - Environment: config overrides for Lightpanda URL, Matrix homeserver, etc.
    - Resource limits: 150MB memory (per jetski README)
    - Health check: `curl http://localhost:9222/health`
  - Define `browser-net` network with subnet 172.23.0.0/16
  - Reference existing `bridge-net` as external
  - Optionally define `lighthouse` service (if it runs separately from core)

  **Must NOT do**:
  - Do NOT modify existing `docker-compose.yml` (that's T8)
  - Do NOT use port 8080 (bridge owns it)
  - Do NOT create networks that conflict with existing subnets (172.20, 172.21, 172.28, 172.29)
  - Do NOT remove existing service definitions from any compose file

  **Recommended Agent Profile**:
  - **Category**: `unspecified-high`
    - Reason: Docker networking requires understanding of existing ArmorClaw topology
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES (after T1)
  - **Parallel Group**: Wave 2 (with T5, T6)
  - **Blocks**: T8 (integrate into main compose), T10 (container verification)
  - **Blocked By**: T1 (core must exist for Dockerfile reference)

  **References**:
  **Pattern References**:
  - `docker-compose.yml:1-25` — Meta-composition pattern using `include:` directives. Follow this exact pattern for adding jetski.
  - `docker-compose.bridge.yml:227-286` — Bridge service definition pattern: security hardening (read_only, no-new-privileges, drop capabilities), resource limits, health checks. Mirror this for jetski.
  - `docker-compose.yml` — Existing networks: `matrix-net` (172.20), `bridge-net` (172.21), `armorclaw-isolated` (172.28), `armorclaw-vault` (172.29). Jetski's `browser-net` should be 172.23.
  - `/home/mink/src/jetski/docker-compose.yml` — Jetski's original compose file with service definition, memory limits, port mappings. Use as reference but adapt for ArmorClaw topology.

  **Acceptance Criteria**:
  - [ ] `docker-compose.jetski.yml` exists
  - [ ] Contains `jetski` service with `build: ./jetski`
  - [ ] Defines `browser-net` with subnet 172.23.0.0/16
  - [ ] References `bridge-net` as external network
  - [ ] Ports mapped: 9222 (CDP), 9223 (RPC) — NOT 8080
  - [ ] Memory limit: 150MB
  - [ ] Health check defined
  - [ ] `docker compose -f docker-compose.jetski.yml config` validates

  **QA Scenarios (MANDATORY)**:
  ```
  Scenario: Verify compose file validates
    Tool: Bash
    Preconditions: File created
    Steps:
      1. Run: docker compose -f docker-compose.jetski.yml config 2>&1
      2. Assert: exit code 0, valid YAML output
      3. Run: docker compose -f docker-compose.jetski.yml config | grep -A5 "browser-net"
      4. Assert: network defined with correct subnet
    Expected Result: Valid compose config with browser-net
    Evidence: .sisyphus/evidence/task-7-compose-validation.txt

  Scenario: Verify no port conflicts
    Tool: Bash
    Steps:
      1. Run: grep "8080" docker-compose.jetski.yml
      2. Assert: exit code 1 (no 8080 usage)
      3. Run: grep "9222" docker-compose.jetski.yml && grep "9223" docker-compose.jetski.yml
      4. Assert: both ports present
    Expected Result: No 8080, 9222 and 9223 present
    Evidence: .sisyphus/evidence/task-7-port-check.txt
  ```

  **Commit**: YES
  - Message: `feat: add docker-compose.jetski.yml with browser-net`
  - Files: `docker-compose.jetski.yml`
  - Pre-commit: `docker compose -f docker-compose.jetski.yml config`

- [x] 8. Integrate jetski compose into main docker-compose.yml

  **What to do**:
  - Add `docker-compose.jetski.yml` to the main `docker-compose.yml` `include:` list
  - Follow existing pattern for meta-composition (how bridge and matrix compose files are included)
  - Verify: `docker compose config` parses correctly with all services visible

  **Must NOT do**:
  - Do NOT modify existing service definitions in any compose file
  - Do NOT change existing network definitions
  - Do NOT reorder existing include entries (append jetski at end)

  **Recommended Agent Profile**:
  - **Category**: `quick`
    - Reason: Single line addition to existing include list
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: NO (depends on T7)
  - **Parallel Group**: Sequential after T7
  - **Blocks**: T10 (container verification)
  - **Blocked By**: T7 (jetski compose must exist first)

  **References**:
  **Pattern References**:
  - `docker-compose.yml:1-25` — The `include:` block showing how `docker-compose.bridge.yml` and `docker-compose.matrix.yml` are included. Add `docker-compose.jetski.yml` in the same pattern.

  **Acceptance Criteria**:
  - [ ] `docker-compose.yml` includes `docker-compose.jetski.yml` in its `include:` list
  - [ ] `docker compose config` validates without errors
  - [ ] `docker compose config --services` lists `jetski` service

  **QA Scenarios (MANDATORY)**:
  ```
  Scenario: Verify jetski service visible in combined config
    Tool: Bash
    Preconditions: docker-compose.yml updated
    Steps:
      1. Run: docker compose config --services 2>&1
      2. Assert: output includes "jetski"
      3. Run: docker compose config 2>&1 | head -5
      4. Assert: exit code 0, valid config
    Expected Result: Jetski service visible in combined compose config
    Evidence: .sisyphus/evidence/task-8-integration.txt

  Scenario: Verify existing services unchanged
    Tool: Bash
    Steps:
      1. Run: docker compose config --services 2>&1 | sort
      2. Assert: all pre-existing services still listed (bridge, matrix, etc.)
    Expected Result: All existing services still present
    Evidence: .sisyphus/evidence/task-8-no-regression.txt
  ```

  **Commit**: YES (groups with T7)
  - Message: `feat: integrate jetski into main docker-compose`
  - Files: `docker-compose.yml`
  - Pre-commit: `docker compose config`

- [x] 9. Verify clean compilation for all 3 Go modules

  **What to do**:
  - Run `go build ./jetski/...` — Core module must compile
  - Run `go build ./jetski/lighthouse/...` — Lighthouse module must compile
  - Run `go vet ./jetski/...` — No warnings
  - Run `go test ./jetski/...` — Existing tests pass (may need Go version alignment)
  - Fix any compilation errors from module rename or Go version bump
  - Run `go mod tidy` in both modules to clean up go.sum files
  - Document any test failures that are pre-existing (not introduced by integration)

  **Must NOT do**:
  - Do NOT write new tests (that's for wiring tasks)
  - Do NOT fix pre-existing bugs — only fix compilation issues from integration
  - Do NOT upgrade any dependencies (pin to current versions)

  **Recommended Agent Profile**:
  - **Category**: `quick`
    - Reason: Build verification and minor fixup
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: NO (depends on T5)
  - **Parallel Group**: Sequential (gate between setup and wiring)
  - **Blocks**: T11-T14 (all wiring tasks need clean build)
  - **Blocked By**: T5 (module rename), T4 (go.work)

  **References**:
  **Pattern References**:
  - `jetski/go.mod` — After T5, should read `module github.com/armorclaw/jetski` with `go 1.25`
  - `jetski/lighthouse/go.mod` — Should read `module github.com/armorclaw/lighthouse`

  **Acceptance Criteria**:
  - [ ] `go build ./jetski/...` exits 0
  - [ ] `go build ./jetski/lighthouse/...` exits 0
  - [ ] `go vet ./jetski/...` exits 0
  - [ ] `go test ./jetski/...` exits 0 (or documents pre-existing failures)

  **QA Scenarios (MANDATORY)**:
  ```
  Scenario: Verify all modules compile
    Tool: Bash
    Preconditions: All Wave 1-2 tasks complete
    Steps:
      1. Run: go build ./jetski/... 2>&1
      2. Assert: exit code 0
      3. Run: go build ./jetski/lighthouse/... 2>&1
      4. Assert: exit code 0
      5. Run: go vet ./jetski/... 2>&1
      6. Assert: exit code 0
    Expected Result: All 3 Go modules compile cleanly
    Failure Indicators: "undefined:", "cannot find package", "syntax error"
    Evidence: .sisyphus/evidence/task-9-compilation.txt

  Scenario: Verify existing tests pass
    Tool: Bash
    Steps:
      1. Run: go test ./jetski/... -v -count=1 2>&1
      2. Assert: exit code 0 (PASS)
      3. Run: go test ./jetski/lighthouse/... -v -count=1 2>&1
      4. Assert: exit code 0 (PASS)
    Expected Result: All existing tests pass after module rename
    Failure Indicators: FAIL, panic, "no such file or directory"
    Evidence: .sisyphus/evidence/task-9-tests.txt
  ```

  **Commit**: YES
  - Message: `chore: verify jetski compilation and fix integration issues`
  - Files: Any files fixed for compilation (go.mod, go.sum, import paths)
  - Pre-commit: `go build ./jetski/... && go test ./jetski/...`

- [ ] 10. Verify jetski container starts and responds to health check

  **What to do**:
  - Build and start jetski container: `docker compose up jetski --build -d`
  - Wait for health check to pass: `docker compose ps jetski` shows "healthy"
  - Test health endpoint: `curl http://localhost:9222/health` returns `{"status":"ok"}`
  - Test RPC endpoint: `curl http://localhost:9223/rpc/status` (may return 404 if RPC not yet implemented — acceptable)
  - Check logs: `docker compose logs jetski` — no panic, no fatal errors
  - Stop container: `docker compose down jetski`

  **Must NOT do**:
  - Do NOT leave the container running after verification
  - Do NOT modify application code to fix container issues (only config/Dockerfile fixes)
  - Do NOT run full E2E test (that's T20)

  **Recommended Agent Profile**:
  - **Category**: `unspecified-high`
    - Reason: Docker build + runtime verification may require troubleshooting
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: NO (depends on T6, T7, T8, T9)
  - **Parallel Group**: Sequential (gate between setup and wiring)
  - **Blocks**: T11-T14 (need running container for wiring verification)
  - **Blocked By**: T6 (Dockerfile), T7 (compose file), T8 (integration), T9 (clean build)

  **References**:
  **Pattern References**:
  - `jetski/Dockerfile` — After T6, should have real SHA256 and port 9223
  - `docker-compose.jetski.yml` — After T7, should have correct network and ports
  - `jetski/cmd/observer/main.go:100-106` — Health check handler: `mux.HandleFunc("/health", healthHandler)`. Returns `{"status":"ok"}`.

  **Acceptance Criteria**:
  - [ ] `docker compose up jetski --build` succeeds (image builds)
  - [ ] `docker compose ps jetski` shows container running
  - [ ] `curl http://localhost:9222/health` returns HTTP 200 with `{"status":"ok"}`
  - [ ] `docker compose logs jetski` shows no panics or fatal errors
  - [ ] Container stopped after verification

  **QA Scenarios (MANDATORY)**:
  ```
  Scenario: Container build and health check
    Tool: Bash
    Preconditions: Docker daemon running, all Wave 1-2 tasks complete
    Steps:
      1. Run: docker compose up jetski --build -d 2>&1
      2. Assert: exit code 0, image built
      3. Run: sleep 5 && docker compose ps jetski
      4. Assert: status shows "running" or "healthy"
      5. Run: curl -s http://localhost:9222/health
      6. Assert: HTTP 200, body contains "ok"
      7. Run: docker compose logs jetski --tail=20 2>&1
      8. Assert: no "panic" or "FATAL" in logs
    Expected Result: Container builds, starts, responds to health check
    Failure Indicators: "build failed", "port already allocated", panic in logs
    Evidence: .sisyphus/evidence/task-10-container-health.txt

  Scenario: Clean shutdown
    Tool: Bash
    Steps:
      1. Run: docker compose down jetski 2>&1
      2. Assert: exit code 0
      3. Run: docker compose ps jetski 2>&1
      4. Assert: no running container
    Expected Result: Container stops cleanly
    Evidence: .sisyphus/evidence/task-10-shutdown.txt
  ```

  **Commit**: YES (if config changes needed)
  - Message: `fix: resolve jetski container startup issues`
  - Files: Any Dockerfile or config changes needed
  - Pre-commit: none (Docker runtime verification)

- [x] 11. Wire Translator into router handlers (TDD)

  **What to do**:
  - Write failing test FIRST: Test that `MethodRouter` delegates mouse/keyboard/text events to `Translator.Translate()` instead of no-op stubs
  - The current no-op handlers (`handleMouseClick`, `handleKeyInput`, `handleTextInsert`) must call `Translator.Translate()` and return the translated CDP command
  - The `Translator` struct converts high-level intents to `Runtime.evaluate` CDP commands
  - Wire `Translator` as a dependency of `MethodRouter` (constructor injection)
  - Verify: Mouse click → Runtime.evaluate with correct coordinates
  - Verify: Key input → Runtime.evaluate with correct key dispatch
  - Verify: Text insert → Runtime.evaluate with correct DOM manipulation
  - Verify: Unknown events → pass through unchanged (fallback)

  **Must NOT do**:
  - Do NOT modify the `Translator` implementation itself (it works, it's just not called)
  - Do NOT remove the fallback mechanism
  - Do NOT add new CDP event types beyond what Translator already handles
  - Do NOT change the WebSocket proxy architecture

  **Recommended Agent Profile**:
  - **Category**: `deep`
    - Reason: TDD workflow with CDP protocol understanding required
  - **Skills**: [`test-driven-development`]
    - `test-driven-development`: Required — this is a TDD task (RED-GREEN-REFACTOR)
  - **Skills Evaluated but Omitted**:
    - `systematic-debugging`: Not debugging, this is new wiring

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 4 (with T12, T13, T14)
  - **Blocks**: T15 (SQLCipher), T16 (PII scrubbing)
  - **Blocked By**: T9 (clean build required)

  **References**:
  **Pattern References**:
  - `jetski/internal/cdp/router.go:125-135` — Current no-op handlers: `handleMouseClick`, `handleKeyInput`, `handleTextInsert` all just `return msg, nil`. These need to delegate to Translator instead.
  - `jetski/internal/cdp/router.go:148-156` — `forwardToEngine()` calls `route.Handler(&msg)` — this is the dispatch point that invokes the handlers.
  - `jetski/internal/cdp/translator.go` — The `Translator` struct with `Translate()` method. This converts high-level browser intents into CDP `Runtime.evaluate` commands. Read this to understand the translation contract.

  **Test References**:
  - `jetski/internal/cdp/router_test.go` — Existing router tests. Follow this pattern for new test structure.

  **Acceptance Criteria**:
  - [ ] Test file created/extended: `jetski/internal/cdp/router_test.go`
  - [ ] Failing test written BEFORE implementation
  - [ ] `go test ./jetski/internal/cdp/... -run TestTranslator` → PASS
  - [ ] Router delegates to Translator for mouse, keyboard, text events
  - [ ] Unknown events pass through unchanged
  - [ ] No changes to Translator.go implementation

  **QA Scenarios (MANDATORY)**:
  ```
  Scenario: Translator receives mouse click events
    Tool: Bash (Go test)
    Preconditions: Clean build, TDD cycle complete
    Steps:
      1. Run: go test ./jetski/internal/cdp/... -run TestTranslatorMouseClick -v
      2. Assert: PASS, test output shows Runtime.evaluate dispatched with click coordinates
    Expected Result: Mouse click events trigger Translator.Translate() and produce valid CDP command
    Evidence: .sisyphus/evidence/task-11-translator-mouse.txt

  Scenario: Unknown events pass through unchanged
    Tool: Bash (Go test)
    Steps:
      1. Run: go test ./jetski/internal/cdp/... -run TestTranslatorPassthrough -v
      2. Assert: PASS, unknown event type returned unchanged
    Expected Result: Unrecognized CDP events bypass translation
    Evidence: .sisyphus/evidence/task-11-translator-passthrough.txt

  Scenario: TDD RED phase verification
    Tool: Bash (Go test)
    Steps:
      1. Run: git log --oneline -5
      2. Assert: failing test commit exists BEFORE implementation commit
    Expected Result: Test commit predates implementation commit
    Evidence: .sisyphus/evidence/task-11-tdd-red.txt
  ```

  **Commit**: YES
  - Message: `feat(jetski): wire Translator into router handlers`
  - Files: `jetski/internal/cdp/router.go`, `jetski/internal/cdp/router_test.go`
  - Pre-commit: `go test ./jetski/internal/cdp/...`

- [x] 12. Wire PII Scanner into CDP proxy flow (TDD)

  **What to do**:
  - Write failing test FIRST: Test that CDP messages flowing through `forwardToEngine()` are scanned for PII
  - Currently `main.go:67` calls `setupPIIScanner()` but discards the result (`_ = setupPIIScanner()`)
  - Wire the PII Scanner as middleware in the CDP proxy's `forwardToEngine()` path
  - When PII detected: log warning + emit Sonar telemetry event (passive mode) or scrub + redact (Tethered mode)
  - For Free-Ride mode: passive warning only (current behavior, but actually connected)
  - For Tethered mode: active scrubbing (replace PII with `[REDACTED]`) — this is the foundation for T16
  - Verify: SSN pattern in CDP message → detected and logged/warned
  - Verify: Credit card pattern → detected and logged/warned
  - Verify: Email pattern → detected and logged/warned
  - Verify: Clean messages → pass through without modification

  **Must NOT do**:
  - Do NOT add new PII patterns beyond the existing 4 (SSN, credit card, email, password)
  - Do NOT block CDP message flow on PII detection (non-blocking scan)
  - Do NOT modify the PII scanner regex patterns themselves
  - Do NOT add network calls in the scanning path

  **Recommended Agent Profile**:
  - **Category**: `deep`
    - Reason: TDD workflow + understanding CDP message flow + PII detection integration
  - **Skills**: [`test-driven-development`]
    - `test-driven-development`: Required — TDD task

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 4 (with T11, T13, T14)
  - **Blocks**: T16 (active PII scrubbing extends this wiring)
  - **Blocked By**: T9 (clean build)

  **References**:
  **Pattern References**:
  - `jetski/cmd/observer/main.go:67` — Current: `_ = setupPIIScanner()` — scanner created but discarded. Fix: assign to variable and pass to CDP proxy constructor.
  - `jetski/internal/security/pii_scanner.go` — The `PIIScanner` struct with `Scan(content string) []PIIMatch` method. Has 4 patterns: SSN (`\d{3}-\d{2}-\d{4}`), credit card (`\d{4}[\s-]?\d{4}[\s-]?\d{4}[\s-]?\d{4}`), email (`[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}`), password fields.
  - `jetski/internal/cdp/proxy.go` — The `forwardToEngine()` method that forwards CDP messages to the browser engine. This is where PII scanning must be inserted.

  **Test References**:
  - `jetski/internal/security/pii_scanner_test.go` — Existing PII scanner tests. Follow this pattern.

  **Acceptance Criteria**:
  - [ ] Test file created/extended: `jetski/internal/cdp/proxy_test.go` or `jetski/internal/security/pii_scanner_test.go`
  - [ ] Failing test written BEFORE implementation
  - [ ] `main.go` assigns PII scanner result and passes it to proxy constructor
  - [ ] `forwardToEngine()` calls `scanner.Scan()` on outbound CDP messages
  - [ ] PII detection logged as warning
  - [ ] `go test ./jetski/internal/cdp/... -run TestPII` → PASS

  **QA Scenarios (MANDATORY)**:
  ```
  Scenario: SSN detected in CDP message
    Tool: Bash (Go test)
    Steps:
      1. Run: go test ./jetski/internal/cdp/... -run TestPIIDetection -v
      2. Assert: PASS, test shows SSN "123-45-6789" detected and logged
    Expected Result: SSN pattern detected in CDP traffic, warning emitted
    Evidence: .sisyphus/evidence/task-12-pii-ssn.txt

  Scenario: Clean message passes through
    Tool: Bash (Go test)
    Steps:
      1. Run: go test ./jetski/internal/cdp/... -run TestPIIClean -v
      2. Assert: PASS, message without PII passes unmodified
    Expected Result: Clean CDP messages unaffected
    Evidence: .sisyphus/evidence/task-12-pii-clean.txt

  Scenario: All 4 PII types detected
    Tool: Bash (Go test)
    Steps:
      1. Run: go test ./jetski/internal/security/... -run TestPIIScanner -v
      2. Assert: PASS, SSN/CC/email/password all detected
    Expected Result: All 4 pattern types trigger detection
    Evidence: .sisyphus/evidence/task-12-pii-all-types.txt
  ```

  **Commit**: YES
  - Message: `feat(jetski): wire PII Scanner into CDP proxy flow`
  - Files: `jetski/cmd/observer/main.go`, `jetski/internal/cdp/proxy.go`, test files
  - Pre-commit: `go test ./jetski/internal/cdp/... ./jetski/internal/security/...`

- [x] 13. Add RPC API handlers on port 9223 (TDD)

  **What to do**:
  - Write failing test FIRST: Test that port 9223 RPC endpoints respond to requests
  - Add RPC API handlers to the HTTP mux in `main.go` alongside existing WebSocket and health handlers
  - Endpoints to implement:
    - `GET /rpc/status` — Returns session status (active sessions, engine health)
    - `POST /rpc/session/create` — Creates a new browser session
    - `POST /rpc/session/close` — Closes an existing session
    - `GET /rpc/health` — Detailed health (engine status, memory usage, uptime)
  - RPC mux should be on a SEPARATE HTTP server (port 9223), NOT on the CDP WebSocket port (9222)
  - Follow existing patterns from bridge for JSON-RPC response format
  - Implement session lifecycle tracking (create/close with ID)

  **Must NOT do**:
  - Do NOT use port 8080 (bridge owns it)
  - Do NOT add RPC endpoints on the CDP WebSocket port (9222)
  - Do NOT import bridge proto definitions (separate module)
  - Do NOT implement authentication on RPC yet (T17 adds HITL approval)

  **Recommended Agent Profile**:
  - **Category**: `unspecified-high`
    - Reason: New API implementation with TDD, multiple endpoints
  - **Skills**: [`test-driven-development`]
    - `test-driven-development`: Required — TDD task

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 4 (with T11, T12, T14)
  - **Blocks**: T17 (HITL approval client needs RPC endpoints)
  - **Blocked By**: T9 (clean build)

  **References**:
  **Pattern References**:
  - `jetski/cmd/observer/main.go:78-106` — Current HTTP mux setup with only `/` (WebSocket upgrade) and `/health`. This is where the RPC handlers need to be added, but on a SEPARATE `http.Server` listening on 9223.
  - `bridge/internal/rpc/` — Bridge's RPC handler patterns for JSON-RPC response format. Reference the response structure (`{"jsonrpc":"2.0","id":1,"result":{}}`) but do NOT import bridge code.

  **Acceptance Criteria**:
  - [ ] Test file created: `jetski/cmd/observer/rpc_test.go` or equivalent
  - [ ] Failing test written BEFORE implementation
  - [ ] Separate HTTP server on port 9223 with RPC mux
  - [ ] `GET /rpc/status` returns JSON with session info
  - [ ] `POST /rpc/session/create` creates session and returns ID
  - [ ] `POST /rpc/session/close` closes session
  - [ ] `go test ./jetski/cmd/observer/... -run TestRPC` → PASS

  **QA Scenarios (MANDATORY)**:
  ```
  Scenario: RPC status endpoint responds
    Tool: Bash (Go test)
    Steps:
      1. Run: go test ./jetski/cmd/observer/... -run TestRPCStatus -v
      2. Assert: PASS, /rpc/status returns JSON with sessions array
    Expected Result: Status endpoint returns valid session status
    Evidence: .sisyphus/evidence/task-13-rpc-status.txt

  Scenario: Session create and close lifecycle
    Tool: Bash (Go test)
    Steps:
      1. Run: go test ./jetski/cmd/observer/... -run TestRPCSessionLifecycle -v
      2. Assert: PASS, session created with ID, then closed, status updated
    Expected Result: Full session lifecycle works
    Evidence: .sisyphus/evidence/task-13-rpc-lifecycle.txt

  Scenario: RPC on correct port (not 8080)
    Tool: Bash
    Steps:
      1. Run: grep -n "8080" jetski/cmd/observer/main.go
      2. Assert: exit code 1 (no 8080 references)
      3. Run: grep -n "9223" jetski/cmd/observer/main.go
      4. Assert: output shows RPC server on 9223
    Expected Result: RPC on 9223, no 8080
    Evidence: .sisyphus/evidence/task-13-rpc-port.txt
  ```

  **Commit**: YES
  - Message: `feat(jetski): add RPC API handlers on port 9223`
  - Files: `jetski/cmd/observer/main.go`, `jetski/internal/rpc/` (new), test files
  - Pre-commit: `go test ./jetski/...`

- [x] 14. Wire Sonar telemetry into main flow (TDD)

  **What to do**:
  - Write failing test FIRST: Test that CDP events are recorded in Sonar circular buffer
  - Currently `internal/sonar/` has buffer, reporter, and telemetry but is never imported in `main.go`
  - Wire `Sonar.Buffer` into the CDP proxy to record browser interaction events
  - Wire `Sonar.Reporter` to periodically flush telemetry to configured endpoint (or log)
  - Add Sonar initialization in `main.go` startup sequence
  - Verify: CDP events → buffer.Record() → buffer fills → reporter flushes

  **Must NOT do**:
  - Do NOT modify the circular buffer implementation
  - Do NOT add external telemetry dependencies (Prometheus, etc.)
  - Do NOT block CDP proxy on telemetry recording (async/non-blocking)
  - Do NOT send telemetry outside the container network

  **Recommended Agent Profile**:
  - **Category**: `quick`
    - Reason: Straightforward wiring of existing telemetry module
  - **Skills**: [`test-driven-development`]
    - `test-driven-development`: Required — TDD task

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 4 (with T11, T12, T13)
  - **Blocks**: T20 (E2E test needs telemetry)
  - **Blocked By**: T9 (clean build)

  **References**:
  **Pattern References**:
  - `jetski/internal/sonar/buffer.go` — Circular buffer for CDP events. Has `Record(event)` and `Flush()` methods.
  - `jetski/internal/sonar/reporter.go` — Periodic reporter that flushes buffer to configured output.
  - `jetski/internal/sonar/telemetry.go` — Telemetry event types.
  - `jetski/cmd/observer/main.go` — Current startup sequence. Add Sonar initialization between subprocess manager and CDP proxy setup.

  **Acceptance Criteria**:
  - [ ] Test file created: `jetski/internal/sonar/sonar_test.go`
  - [ ] Failing test written BEFORE implementation
  - [ ] `main.go` imports and initializes Sonar components
  - [ ] CDP events recorded in buffer during proxy flow
  - [ ] `go test ./jetski/internal/sonar/...` → PASS

  **QA Scenarios (MANDATORY)**:
  ```
  Scenario: CDP events recorded in buffer
    Tool: Bash (Go test)
    Steps:
      1. Run: go test ./jetski/internal/sonar/... -run TestSonarRecording -v
      2. Assert: PASS, events recorded and retrievable from buffer
    Expected Result: CDP events captured in circular buffer
    Evidence: .sisyphus/evidence/task-14-sonar-recording.txt

  Scenario: Reporter flushes buffer
    Tool: Bash (Go test)
    Steps:
      1. Run: go test ./jetski/internal/sonar/... -run TestSonarReporter -v
      2. Assert: PASS, reporter flushes buffer contents
    Expected Result: Buffer contents flushed via reporter
    Evidence: .sisyphus/evidence/task-14-sonar-reporter.txt
  ```

  **Commit**: YES
  - Message: `feat(jetski): wire Sonar telemetry into main flow`
  - Files: `jetski/cmd/observer/main.go`, `jetski/internal/cdp/proxy.go`, `jetski/internal/sonar/sonar_test.go`
  - Pre-commit: `go test ./jetski/internal/sonar/...`

- [x] 15. Replace age with SQLCipher session store (TDD)

  **What to do**:
  - Write failing test FIRST: Test that sessions are encrypted with SQLCipher, not `age`
  - Create new `jetski/internal/security/sqlcipher_session.go` implementing the `SessionStore` interface
  - Follow bridge keystore pattern: PBKDF2-HMAC-SHA512 key derivation, hardware-derived key, PRAGMA-based SQLCipher configuration
  - Replace `age` encrypt/decrypt calls in `session.go` with SQLCipher store
  - Session data (ID, UserAgent, Cookies, ExpiresAt) stored in encrypted SQLite database
  - Remove `filippo.io/age` dependency from `go.mod`
  - Add `github.com/mutecomm/go-sqlcipher/v4` dependency
  - Key derivation: Use same pattern as bridge keystore (device-specific, PBKDF2 with high iteration count)
  - Database path: configurable, default `/var/lib/jetski/sessions.db`
  - Implement `CreateSession()`, `GetSession()`, `CloseSession()`, `RotateKey()` methods
  - Verify: Session data is unreadable without correct key (attempt raw SQLite read → fails)

  **Must NOT do**:
  - Do NOT modify bridge's SQLCipher keystore (separate module, separate database)
  - Do NOT share encryption keys with bridge keystore (independent key derivation)
  - Do NOT weaken encryption parameters (match bridge: PBKDF2 with 256k iterations minimum)
  - Do NOT store keys in plaintext files or environment variables
  - Do NOT remove the `Session` struct interface — only change the encryption backend
  - Do NOT skip key zeroization after use (security requirement from Phase 2 F3 review)

  **Recommended Agent Profile**:
  - **Category**: `deep`
    - Reason: Cryptographic implementation with SQLCipher, key derivation, TDD — requires careful security work
  - **Skills**: [`test-driven-development`]
    - `test-driven-development`: Required — TDD task for security-critical code

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 5 (with T16, T17)
  - **Blocks**: T20 (E2E test needs SQLCipher sessions)
  - **Blocked By**: T11 (proxy wiring must be stable), T9 (clean build)

  **References**:
  **Pattern References**:
  - `bridge/pkg/keystore/keystore.go:1-50` — Bridge's SQLCipher keystore pattern. Follow EXACTLY: PBKDF2-HMAC-SHA512, `PRAGMA key`, `PRAGMA cipher_page_size`, hardware-derived key. This is the canonical ArmorClaw encryption pattern.
  - `bridge/pkg/keystore/keystore.go:51-100` — Key derivation and PRAGMA setup. Mirror this for session encryption.
  - `jetski/internal/security/session.go` — Current session encryption using `age`. The `Encrypt()`/`Decrypt()` methods need to be replaced with SQLCipher store operations. Session struct: `{ID string, UserAgent string, Cookies []http.Cookie, ExpiresAt time.Time}`.
  - `jetski/internal/security/session.go:1-20` — Session struct definition. Preserve this interface.

  **Test References**:
  - `bridge/pkg/keystore/keystore_test.go` — Test patterns for SQLCipher: create store, write data, verify encryption, close and reopen, verify persistence.

  **External References**:
  - `go-sqlcipher` docs: `https://github.com/mutecomm/go-sqlcipher` — SQLCipher Go driver API

  **Acceptance Criteria**:
  - [ ] Test file created: `jetski/internal/security/sqlcipher_session_test.go`
  - [ ] Failing test written BEFORE implementation
  - [ ] `filippo.io/age` removed from `jetski/go.mod`
  - [ ] `github.com/mutecomm/go-sqlcipher/v4` added to `jetski/go.mod`
  - [ ] `go test ./jetski/internal/security/... -run TestSQLCipherSession` → PASS
  - [ ] Raw SQLite read of sessions.db fails (proves encryption)
  - [ ] Key zeroization verified (no key material in memory after operations)

  **QA Scenarios (MANDATORY)**:
  ```
  Scenario: Session encrypted with SQLCipher
    Tool: Bash (Go test)
    Steps:
      1. Run: go test ./jetski/internal/security/... -run TestSQLCipherSession -v
      2. Assert: PASS, session created, retrieved, and verified encrypted
    Expected Result: Session data encrypted at rest with SQLCipher
    Evidence: .sisyphus/evidence/task-15-sqlcipher-session.txt

  Scenario: Raw database read fails without key
    Tool: Bash (Go test)
    Steps:
      1. Run: go test ./jetski/internal/security/... -run TestSQLCipherRawRead -v
      2. Assert: PASS, attempting to open sessions.db without key returns error
    Expected Result: Database unreadable without correct key
    Evidence: .sisyphus/evidence/task-15-encryption-verification.txt

  Scenario: Key zeroization after operation
    Tool: Bash (Go test)
    Steps:
      1. Run: go test ./jetski/internal/security/... -run TestKeyZeroization -v
      2. Assert: PASS, key bytes zeroed after use
    Expected Result: No key material remains in memory
    Evidence: .sisyphus/evidence/task-15-key-zeroization.txt

  Scenario: age dependency removed
    Tool: Bash
    Steps:
      1. Run: grep "filippo.io/age" jetski/go.mod
      2. Assert: exit code 1 (no age dependency)
      3. Run: grep -r "filippo.io/age" jetski/ --include="*.go"
      4. Assert: exit code 1 (no age imports)
    Expected Result: age completely removed
    Evidence: .sisyphus/evidence/task-15-age-removed.txt
  ```

  **Commit**: YES
  - Message: `feat(jetski): replace age with SQLCipher session store`
  - Files: `jetski/internal/security/sqlcipher_session.go`, `jetski/internal/security/session.go` (modified), `jetski/go.mod`, `jetski/go.sum`
  - Pre-commit: `go test ./jetski/internal/security/...`

- [x] 16. Active PII scrubbing in CDP proxy (TDD)

  **What to do**:
  - Write failing test FIRST: Test that PII is actively REDACTED (not just detected) in Tethered mode
  - Extend the PII scanning wiring from T12 to add active scrubbing
  - In Tethered mode: when PII detected in `forwardToEngine()`, replace with `[REDACTED_SSN]`, `[REDACTED_CC]`, `[REDACTED_EMAIL]`, `[REDACTED_PASSWORD]`
  - In Free-Ride mode: keep passive warning only (no modification)
  - Add mode flag to CDP proxy constructor: `tetheredMode bool`
  - Verify: SSN "123-45-6789" in CDP message → replaced with `[REDACTED_SSN]`
  - Verify: Multiple PII types in single message → all redacted
  - Verify: False positive rate acceptable (test with edge cases like phone numbers)
  - Log every redaction event to Sonar telemetry

  **Must NOT do**:
  - Do NOT add new PII patterns (locked to 4 existing)
  - Do NOT block CDP flow during scrubbing (must be synchronous but fast)
  - Do NOT modify the PII scanner regex patterns
  - Do NOT add network calls during scrubbing
  - Do NOT break CDP command-response ordering (scrubbing must not introduce async delays)

  **Recommended Agent Profile**:
  - **Category**: `deep`
    - Reason: TDD + security-sensitive data scrubbing + CDP protocol integrity
  - **Skills**: [`test-driven-development`]
    - `test-driven-development`: Required — TDD task

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 5 (with T15, T17)
  - **Blocks**: T20 (E2E test needs PII scrubbing)
  - **Blocked By**: T11 (Translator wired), T12 (PII scanner wired)

  **References**:
  **Pattern References**:
  - `jetski/internal/cdp/proxy.go` — The `forwardToEngine()` method. After T12, PII scanner is called here. Extend to add active scrubbing when `tetheredMode` is true.
  - `jetski/internal/security/pii_scanner.go` — PII scanner with `Scan()` returning `[]PIIMatch`. Each match has `Type`, `Value`, `Start`, `End` positions — use for targeted replacement.
  - `jetski/internal/cdp/translator.go` — After T11, Translator is wired in. PII scrubbing should happen BEFORE translation (raw CDP message → scrub → translate → send to engine).

  **Acceptance Criteria**:
  - [ ] Test file: `jetski/internal/cdp/pii_scrub_test.go`
  - [ ] Failing test written BEFORE implementation
  - [ ] Tethered mode: SSN replaced with `[REDACTED_SSN]`
  - [ ] Tethered mode: CC replaced with `[REDACTED_CC]`
  - [ ] Tethered mode: Email replaced with `[REDACTED_EMAIL]`
  - [ ] Tethered mode: Password replaced with `[REDACTED_PASSWORD]`
  - [ ] Free-Ride mode: no modification, only logging
  - [ ] `go test ./jetski/internal/cdp/... -run TestPIIScrub` → PASS

  **QA Scenarios (MANDATORY)**:
  ```
  Scenario: Active PII redaction in Tethered mode
    Tool: Bash (Go test)
    Steps:
      1. Run: go test ./jetski/internal/cdp/... -run TestPIIScrubTethered -v
      2. Assert: PASS, all 4 PII types replaced with [REDACTED_*] tokens
    Expected Result: PII actively redacted in outbound CDP messages
    Evidence: .sisyphus/evidence/task-16-scrub-tethered.txt

  Scenario: Free-Ride mode preserves messages
    Tool: Bash (Go test)
    Steps:
      1. Run: go test ./jetski/internal/cdp/... -run TestPIIScrubFreeRide -v
      2. Assert: PASS, messages unmodified, only warning logged
    Expected Result: Free-Ride mode does NOT modify messages
    Evidence: .sisyphus/evidence/task-16-scrub-freeride.txt

  Scenario: Multiple PII types in single message
    Tool: Bash (Go test)
    Steps:
      1. Run: go test ./jetski/internal/cdp/... -run TestPIIScrubMultiple -v
      2. Assert: PASS, all PII instances replaced, non-PII text preserved
    Expected Result: Multiple PII patterns in one message all redacted
    Evidence: .sisyphus/evidence/task-16-scrub-multiple.txt

  Scenario: CDP ordering preserved
    Tool: Bash (Go test)
    Steps:
      1. Run: go test ./jetski/internal/cdp/... -run TestPIIScrubOrdering -v
      2. Assert: PASS, message ordering unchanged after scrubbing
    Expected Result: Scrubbing is synchronous, no reordering
    Evidence: .sisyphus/evidence/task-16-scrub-ordering.txt
  ```

  **Commit**: YES
  - Message: `feat(jetski): add active PII scrubbing in Tethered mode`
  - Files: `jetski/internal/cdp/proxy.go`, `jetski/internal/cdp/pii_scrub_test.go`
  - Pre-commit: `go test ./jetski/internal/cdp/...`

- [x] 17. Matrix HITL approval client (TDD)

  **What to do**:
  - Write failing test FIRST: Test that sensitive browser operations require Matrix approval before executing
  - Create `jetski/internal/approval/matrix_client.go` — Matrix client for HITL approval
  - Use existing bridge Matrix patterns (do NOT invent new event types)
  - Approval flow:
    1. Agent requests sensitive CDP operation via RPC
    2. Jetski posts approval request to user's Matrix room via bridge
    3. User approves/denies via existing Matrix approval UI
    4. Bridge sends approval event back
    5. Jetski receives approval → executes or denies operation
  - Operations requiring approval: session creation (in Tethered mode), navigation to new domains, file downloads
  - Timeout: configurable, default 60 seconds. If no response → deny
  - Use the RPC API from T13 for the approval request/response endpoints
  - Matrix client config: homeserver URL, access token, room ID from `configs/config.yaml`

  **Must NOT do**:
  - Do NOT create new Matrix event types — reuse existing bridge approval patterns
  - Do NOT bypass Matrix (approval MUST go through Matrix control plane)
  - Do NOT store approval state in plaintext
  - Do NOT make approval blocking for non-sensitive operations
  - Do NOT hardcode room IDs or access tokens

  **Recommended Agent Profile**:
  - **Category**: `deep`
    - Reason: Matrix integration, approval flow design, async communication, TDD
  - **Skills**: [`test-driven-development`]
    - `test-driven-development`: Required — TDD task

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 5 (with T15, T16)
  - **Blocks**: T20 (E2E test needs HITL)
  - **Blocked By**: T13 (RPC endpoints needed for approval API)

  **References**:
  **Pattern References**:
  - `bridge/internal/matrix/` — Bridge's Matrix client implementation. Reference for how bridge sends/receives events. Follow the same patterns for approval requests.
  - `bridge/internal/approval/` (if exists) — Bridge approval flow patterns. Look for how the bridge handles approval for payments/PII.
  - `jetski/configs/config.yaml` — Configuration file. Add Matrix section: `matrix: { homeserver, access_token, room_id, approval_timeout }`.
  - `jetski/cmd/observer/main.go` — RPC server setup from T13. Add approval endpoints to the RPC mux.

  **Acceptance Criteria**:
  - [ ] Test file created: `jetski/internal/approval/matrix_client_test.go`
  - [ ] Failing test written BEFORE implementation
  - [ ] `jetski/internal/approval/matrix_client.go` implements approval request/response
  - [ ] Sensitive operations blocked until approval received
  - [ ] Timeout handled (default 60s → auto-deny)
  - [ ] Approval events use existing bridge Matrix patterns
  - [ ] `go test ./jetski/internal/approval/...` → PASS

  **QA Scenarios (MANDATORY)**:
  ```
  Scenario: Approval request sent for sensitive operation
    Tool: Bash (Go test)
    Steps:
      1. Run: go test ./jetski/internal/approval/... -run TestApprovalRequest -v
      2. Assert: PASS, approval request queued, response awaited
    Expected Result: Sensitive operation triggers approval request
    Evidence: .sisyphus/evidence/task-17-approval-request.txt

  Scenario: Approved operation executes
    Tool: Bash (Go test)
    Steps:
      1. Run: go test ./jetski/internal/approval/... -run TestApprovalGranted -v
      2. Assert: PASS, approved operation returns success
    Expected Result: Approved operation proceeds normally
    Evidence: .sisyphus/evidence/task-17-approval-granted.txt

  Scenario: Denied operation blocked
    Tool: Bash (Go test)
    Steps:
      1. Run: go test ./jetski/internal/approval/... -run TestApprovalDenied -v
      2. Assert: PASS, denied operation returns error
    Expected Result: Denied operation returns error, no execution
    Evidence: .sisyphus/evidence/task-17-approval-denied.txt

  Scenario: Timeout auto-deny
    Tool: Bash (Go test)
    Steps:
      1. Run: go test ./jetski/internal/approval/... -run TestApprovalTimeout -v
      2. Assert: PASS, operation denied after timeout
    Expected Result: No response within 60s → auto-deny
    Evidence: .sisyphus/evidence/task-17-approval-timeout.txt
  ```

  **Commit**: YES
  - Message: `feat(jetski): add Matrix HITL approval client`
  - Files: `jetski/internal/approval/` (new), `jetski/configs/config.yaml`, `jetski/go.mod`
  - Pre-commit: `go test ./jetski/internal/approval/...`

- [ ] 18. Lighthouse Docker build + Nav-Chart API verification

  **What to do**:
  - Add `lighthouse` service to `docker-compose.jetski.yml`
  - Lighthouse has its own `Dockerfile` (check `/home/mink/src/jetski/lighthouse/` for existing or create new)
  - Service configuration:
    - Build from `jetski/lighthouse/Dockerfile` (create if missing, following core Dockerfile pattern)
    - Port: 8081 (or as configured — must NOT conflict with bridge 8080)
    - Network: `browser-net` (needs to talk to jetski core) + `bridge-net` (optional, for external access)
    - Environment: database path, signing key config
    - Resource limits: 100MB memory (Lighthouse is lighter than core)
    - Health check: `curl http://localhost:8081/health`
  - Verify: `docker compose up lighthouse --build` succeeds
  - Verify: Nav-Chart API responds: `curl http://localhost:8081/nav-charts`
  - If Lighthouse has no Dockerfile: create one following Alpine + CGO pattern (mattn/go-sqlite3 requires CGO)

  **Must NOT do**:
  - Do NOT modify Lighthouse Go source code
  - Do NOT change the SQLite driver (keep `mattn/go-sqlite3`)
  - Do NOT use port 8080 (bridge owns it)
  - Do NOT share database files with core jetski

  **Recommended Agent Profile**:
  - **Category**: `unspecified-high`
    - Reason: Docker build with CGO, may require Dockerfile creation
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 6 (with T19, T20)
  - **Blocks**: T20 (E2E test)
  - **Blocked By**: T9 (Lighthouse compiles), T7 (compose file exists)

  **References**:
  **Pattern References**:
  - `jetski/lighthouse/` — Lighthouse sub-project. Has its own `go.mod` (`github.com/armorclaw/lighthouse`). Uses chi router + mattn/go-sqlite3 + HMAC-SHA256 signing.
  - `jetski/Dockerfile` — Core Dockerfile pattern (after T6). Follow this pattern for Lighthouse Dockerfile: Alpine base, CGO_ENABLED=1, multi-stage build.
  - `docker-compose.jetski.yml` — After T7, has core jetski service. Add lighthouse service alongside.

  **Acceptance Criteria**:
  - [ ] Lighthouse service defined in `docker-compose.jetski.yml`
  - [ ] `docker compose up lighthouse --build` succeeds
  - [ ] `curl http://localhost:8081/nav-charts` returns response (even if empty list)
  - [ ] Health check passing

  **QA Scenarios (MANDATORY)**:
  ```
  Scenario: Lighthouse container builds and starts
    Tool: Bash
    Preconditions: Docker daemon running, T9 complete
    Steps:
      1. Run: docker compose up lighthouse --build -d 2>&1
      2. Assert: exit code 0, image built
      3. Run: sleep 5 && docker compose ps lighthouse
      4. Assert: container running or healthy
    Expected Result: Lighthouse container starts successfully
    Evidence: .sisyphus/evidence/task-18-lighthouse-start.txt

  Scenario: Nav-Chart API responds
    Tool: Bash
    Steps:
      1. Run: curl -s http://localhost:8081/nav-charts
      2. Assert: HTTP 200 with JSON response (may be empty array)
    Expected Result: Nav-Chart API accessible and responding
    Evidence: .sisyphus/evidence/task-18-nav-charts.txt

  Scenario: Clean shutdown
    Tool: Bash
    Steps:
      1. Run: docker compose down lighthouse 2>&1
      2. Assert: exit code 0
    Expected Result: Container stops cleanly
    Evidence: .sisyphus/evidence/task-18-lighthouse-stop.txt
  ```

  **Commit**: YES
  - Message: `feat(jetski): integrate lighthouse into docker-compose`
  - Files: `docker-compose.jetski.yml`, `jetski/lighthouse/Dockerfile` (if created)
  - Pre-commit: `docker compose -f docker-compose.jetski.yml config`

- [x] 19. Chartmaker npm build + CLI verification

  **What to do**:
  - Run `npm install` in `jetski/jetski-chartmaker/`
  - Run `npm run build` to compile TypeScript
  - Verify CLI runs: `npx jetski-chartmaker --help` or equivalent
  - If there's a global binary: verify `npm link` or `npx` execution
  - Verify TypeScript compilation produces `dist/` output
  - Add `jetski-chartmaker` as optional service in docker-compose (or document as CLI-only tool)
  - Ensure `node_modules/` and `dist/` are in `.gitignore`

  **Must NOT do**:
  - Do NOT modify chartmaker source code
  - Do NOT add chartmaker to `go.work` (it's TypeScript, not Go)
  - Do NOT commit `node_modules/` or `dist/`
  - Do NOT add chartmaker as a Docker service unless it has a server mode

  **Recommended Agent Profile**:
  - **Category**: `quick`
    - Reason: npm build verification, minimal changes
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 6 (with T18, T20)
  - **Blocks**: T20 (E2E test may use chartmaker)
  - **Blocked By**: T3 (chartmaker source copied)

  **References**:
  **Pattern References**:
  - `jetski/jetski-chartmaker/package.json` — NPM package config. Check for `build` and `start` scripts.
  - `jetski/jetski-chartmaker/tsconfig.json` — TypeScript config. Reference for build output directory.
  - `browser-service/package.json` — Existing TypeScript project in ArmorClaw. Reference for Node.js build patterns.

  **Acceptance Criteria**:
  - [ ] `npm install` completes without errors
  - [ ] `npm run build` produces `dist/` output
  - [ ] CLI tool runs (help output or version check)
  - [ ] `node_modules/` and `dist/` in `.gitignore`
  - [ ] No modifications to chartmaker source code

  **QA Scenarios (MANDATORY)**:
  ```
  Scenario: npm build succeeds
    Tool: Bash
    Preconditions: T3 complete (source copied)
    Steps:
      1. Run: cd jetski/jetski-chartmaker && npm install 2>&1
      2. Assert: exit code 0, no critical vulnerabilities
      3. Run: npm run build 2>&1
      4. Assert: exit code 0, dist/ directory created
    Expected Result: TypeScript compiled, dist/ output present
    Evidence: .sisyphus/evidence/task-19-npm-build.txt

  Scenario: CLI tool runs
    Tool: Bash
    Steps:
      1. Run: cd jetski/jetski-chartmaker && npx . --help 2>&1 || npx . --version 2>&1
      2. Assert: output contains help text or version number
    Expected Result: CLI tool executable
    Evidence: .sisyphus/evidence/task-19-cli.txt

  Scenario: No artifacts committed
    Tool: Bash
    Steps:
      1. Run: grep "node_modules" jetski/.gitignore || grep "node_modules" .gitignore
      2. Assert: node_modules is gitignored
    Expected Result: Build artifacts won't be committed
    Evidence: .sisyphus/evidence/task-19-gitignore.txt
  ```

  **Commit**: YES
  - Message: `feat(jetski): verify chartmaker npm build`
  - Files: `jetski/.gitignore` (updated), possibly `jetski/jetski-chartmaker/package-lock.json`
  - Pre-commit: `cd jetski/jetski-chartmaker && npm run build`

- [x] 20. End-to-end Tethered mode integration test

  **What to do**:
  - Create comprehensive integration test in `jetski/tests/e2e_tethered_test.go`
  - Test the COMPLETE Tethered mode flow:
    1. Start jetski container with Tethered mode enabled
    2. Connect CDP client to port 9222
    3. Request session via RPC port 9223 → HITL approval requested
    4. Simulate approval via Matrix mock → session created
    5. Send CDP message with PII (SSN, CC, email) → verify scrubbed in outbound
    6. Verify session encrypted in SQLCipher database
    7. Verify Sonar telemetry recorded for all events
    8. Close session via RPC → verify cleanup
    9. Verify Translator converts high-level events correctly
  - Test Free-Ride mode in parallel:
    1. Start jetski with Free-Ride mode
    2. Connect CDP client
    3. Send PII data → verify NOT scrubbed, only warned
    4. Verify session in cleartext (age fallback or plaintext)
  - This is the GATE test — all Waves 1-5 must be complete and working

  **Must NOT do**:
  - Do NOT test against real Matrix server (use mock)
  - Do NOT test against real Lightpanda engine (use mock CDP target)
  - Do NOT skip any Tethered mode feature (PII, HITL, SQLCipher all must work)
  - Do NOT modify any implementation code to make tests pass — fix the implementation

  **Recommended Agent Profile**:
  - **Category**: `deep`
    - Reason: Complex integration test spanning all modules, requires mocking strategy
  - **Skills**: [`test-driven-development`]
    - `test-driven-development`: Required — integration TDD

  **Parallelization**:
  - **Can Run In Parallel**: NO (depends on ALL prior tasks)
  - **Parallel Group**: Wave 6 (sequential gate, after T18 and T19)
  - **Blocks**: F1-F4 (verification wave)
  - **Blocked By**: T15 (SQLCipher), T16 (PII scrubbing), T17 (HITL), T18 (Lighthouse), T19 (Chartmaker)

  **References**:
  **Pattern References**:
  - `jetski/internal/cdp/proxy.go` — CDP proxy with PII scrubbing (after T16)
  - `jetski/internal/security/sqlcipher_session.go` — SQLCipher sessions (after T15)
  - `jetski/internal/approval/matrix_client.go` — HITL approval (after T17)
  - `jetski/internal/sonar/` — Telemetry recording (after T14)
  - `tests/test_transport_guard.sh` — Phase 2 integration test pattern. Follow this for E2E test structure.
  - `tests/test_yara_heap_profile.sh` — Another Phase 2 test pattern for shell-based integration testing.

  **Acceptance Criteria**:
  - [ ] `jetski/tests/e2e_tethered_test.go` created
  - [ ] Tethered mode flow tested: session → approval → PII scrub → encrypt → close
  - [ ] Free-Ride mode flow tested: session → PII warn (no scrub) → close
  - [ ] `go test ./jetski/tests/... -v -timeout 120s` → PASS
  - [ ] All 4 security features verified in single test run

  **QA Scenarios (MANDATORY)**:
  ```
  Scenario: Full Tethered mode E2E
    Tool: Bash (Go test)
    Preconditions: ALL Waves 1-5 complete
    Steps:
      1. Run: go test ./jetski/tests/... -run TestE2ETethered -v -timeout 120s
      2. Assert: PASS, all steps complete:
         - Session created with HITL approval
         - PII scrubbed in CDP traffic
         - Session encrypted in SQLCipher
         - Sonar telemetry recorded
         - Session closed cleanly
    Expected Result: Full Tethered mode pipeline works end-to-end
    Failure Indicators: "approval timeout", "PII not scrubbed", "session not encrypted"
    Evidence: .sisyphus/evidence/task-20-e2e-tethered.txt

  Scenario: Free-Ride mode E2E
    Tool: Bash (Go test)
    Steps:
      1. Run: go test ./jetski/tests/... -run TestE2EFreeRide -v -timeout 60s
      2. Assert: PASS, PII detected but NOT scrubbed, session in cleartext
    Expected Result: Free-Ride mode preserves data, warns only
    Evidence: .sisyphus/evidence/task-20-e2e-freeride.txt

  Scenario: Edge case — concurrent sessions
    Tool: Bash (Go test)
    Steps:
      1. Run: go test ./jetski/tests/... -run TestE2EConcurrentSessions -v -timeout 60s
      2. Assert: PASS, multiple sessions handled independently
    Expected Result: Session isolation maintained under concurrent access
    Evidence: .sisyphus/evidence/task-20-e2e-concurrent.txt

  Scenario: Edge case — Lightpanda crash recovery
    Tool: Bash (Go test)
    Steps:
      1. Run: go test ./jetski/tests/... -run TestE2ECrashRecovery -v -timeout 60s
      2. Assert: PASS, engine restart detected, sessions recover or clean up
    Expected Result: Graceful handling of engine crashes
    Evidence: .sisyphus/evidence/task-20-e2e-crash.txt
  ```

  **Commit**: YES
  - Message: `test(jetski): end-to-end Tethered mode integration test`
  - Files: `jetski/tests/e2e_tethered_test.go`, `jetski/tests/helpers/` (mocks)
  - Pre-commit: `go test ./jetski/tests/... -timeout 120s`

---

## Final Verification Wave (MANDATORY — after ALL implementation tasks)

> 4 review agents run in PARALLEL. ALL must APPROVE. Present consolidated results to user and get explicit "okay" before completing.
>
> **Do NOT auto-proceed after verification. Wait for user's explicit approval before marking work complete.**
> **Never mark F1-F4 as checked before getting user's okay.** Rejection or user feedback -> fix -> re-run -> present again -> wait for okay.

- [x] F1. **Plan Compliance Audit** — `oracle`
  Read the plan end-to-end. For each "Must Have": verify implementation exists (read file, go build, go test). For each "Must NOT Have": search codebase for forbidden patterns — reject with file:line if found. Check evidence files exist in .sisyphus/evidence/. Compare deliverables against plan. Verify no changes to browser-service/, bridge/ go.mod, sidecar/, or proto files.
  Output: `Must Have [N/N] | Must NOT Have [N/N] | Tasks [N/N] | VERDICT: APPROVE/REJECT`

- [x] F2. **Code Quality Review** — `unspecified-high`
  Run `go vet ./jetski/...` + `go build ./jetski/...` + `go test ./jetski/...`. Review all changed files for: `interface{}` without purpose, empty catches, `fmt.Println` in prod, commented-out code, unused imports. Check AI slop: excessive comments, over-abstraction, generic names. Verify no CGO in Core module.
  Output: `Build [PASS/FAIL] | Vet [PASS/FAIL] | Tests [N pass/N fail] | Files [N clean/N issues] | VERDICT`

- [ ] F3. **Real Manual QA** — `unspecified-high`
  Start from clean state (`docker compose down && docker compose up --build jetski`). Execute EVERY QA scenario from EVERY task — follow exact steps, capture evidence. Test cross-task integration: CDP proxy with PII scrubbing, HITL approval flow, SQLCipher session persistence. Test edge cases: concurrent sessions, Lightpanda crash recovery, PII false positives. Save to `.sisyphus/evidence/final-qa/`.
  Output: `Scenarios [N/N pass] | Integration [N/N] | Edge Cases [N tested] | VERDICT`

- [x] F4. **Scope Fidelity Check** — `deep`
  For each task: read "What to do", read actual diff (git log/diff). Verify 1:1 — everything in spec was built (no missing), nothing beyond spec was built (no creep). Check "Must NOT do" compliance. Verify no changes to browser-service/, bridge/, sidecar/, proto files. Detect cross-task contamination. Flag unaccounted changes.
  Output: `Tasks [N/N compliant] | Contamination [CLEAN/N issues] | Unaccounted [CLEAN/N files] | VERDICT`

---

| # | Message | Files | Pre-commit |
|---|---------|-------|------------|
| 1 | `chore: copy jetski core source tree` | `jetski/` (core only) | — |
| 2 | `chore: copy lighthouse sub-project` | `jetski/lighthouse/` | — |
| 3 | `chore: copy chartmaker sub-project` | `jetski/jetski-chartmaker/` | — |
| 4 | `chore: add go.work for multi-module workspace` | `go.work` | `go build ./jetski/...` |
| 5 | `refactor: rename jetski-browser module to github.com/armorclaw/jetski` | `jetski/go.mod`, all imports | `go build ./jetski/...` |
| 6 | `fix: pin Lightpanda SHA256 + remap RPC port` | `jetski/Dockerfile` | — |
| 7 | `feat: add docker-compose.jetski.yml with browser-net` | `docker-compose.jetski.yml` | — |
| 8 | `feat: integrate jetski into main docker-compose` | `docker-compose.yml` | `docker compose config` |
| 9 | `test: verify all Go modules compile` | — | `go build ./jetski/...` |
| 10 | `feat(jetski): wire Translator into router handlers` | `jetski/internal/cdp/router.go` + test | `go test ./jetski/internal/cdp/...` |
| 11 | `feat(jetski): wire PII Scanner into CDP proxy` | `jetski/internal/security/` + test | `go test ./jetski/internal/security/...` |
| 12 | `feat(jetski): add RPC API handlers on port 9223` | `jetski/cmd/observer/main.go` + test | `go test ./jetski/...` |
| 13 | `feat(jetski): wire Sonar telemetry into main flow` | `jetski/internal/sonar/` + test | `go test ./jetski/internal/sonar/...` |
| 14 | `feat(jetski): replace age with SQLCipher session store` | `jetski/internal/security/session.go` | `go test ./jetski/internal/security/...` |
| 15 | `feat(jetski): add active PII scrubbing to CDP proxy` | `jetski/internal/cdp/proxy.go` | `go test ./jetski/internal/cdp/...` |
| 16 | `feat(jetski): add Matrix HITL approval client` | `jetski/internal/approval/` | `go test ./jetski/internal/approval/...` |
| 17 | `feat(jetski): integrate lighthouse into docker-compose` | `docker-compose.jetski.yml` | `docker compose config` |
| 18 | `feat(jetski): integrate chartmaker npm build` | `jetski/jetski-chartmaker/` | `npm run build` |
| 19 | `test(jetski): end-to-end Tethered mode integration` | `jetski/tests/` | `go test ./jetski/tests/...` |

---

## Success Criteria

### Verification Commands
```bash
go build ./jetski/...                          # Expected: clean compilation, no errors
go test ./jetski/...                           # Expected: all tests pass
go vet ./jetski/...                            # Expected: no warnings
docker compose up jetski --build               # Expected: container starts
curl http://localhost:9222/health              # Expected: {"status":"ok"}
curl http://localhost:9223/rpc/status          # Expected: RPC API responds
docker compose up lighthouse --build           # Expected: container starts
curl http://localhost:8081/nav-charts          # Expected: Nav-Chart API responds
cd jetski/jetski-chartmaker && npm run build   # Expected: build succeeds
```

### Final Checklist
- [ ] All "Must Have" present
- [ ] All "Must NOT Have" absent
- [ ] All tests pass
- [ ] No changes to browser-service/, bridge/, sidecar/, or proto files
- [ ] Jetski container starts and responds to health check
- [ ] CDP proxy forwards messages with PII scrubbing
- [ ] Sessions encrypted with SQLCipher (no `age` references in production code)
- [ ] HITL approval requested via Matrix for sensitive operations
