# Rust Vault v6.0 Enterprise Sovereign - Security Fixes Implementation

## TL;DR

> **Quick Summary**: Implement Rust Vault cryptographic enclave from scratch, incorporating 7 critical security fixes from the initial architecture. Follow TDD methodology with> 
> **Deliverables**:
> - Rust Vault component with CDP BlindFill, - gRPC-over-UDS with mTLS authentication
> - SQLCipher encrypted vault.db and matrix_state.db
> - Zeroized secret memory handling
> - Rate-limited gRPC middleware
> 
> **Estimated Effort**: 20-28 hours
> **Parallel Execution**: YES - 5 waves + **Critical Path**: Config → DB Pool → Vault DB → gRPC Server → CDP Interceptor → Integration

---

## Context

### Original Request
Review and implement Rust Vault v6.0 Enterprise Sovereign plan with 7 security fixes incorporated from the start.

### Interview Summary
**Key Discussions**:
- **Test Strategy**: TDD confirmed - all components follow RED → GREEN → REFACTOR
- **Priority Order**: CDP bottleneck (CRITICAL) → gRPC auth (CRITICAL) → string leak (CRITICAL) → socket perms (HIGH) → key derivation (HIGH) → token bucket (MEDIUM) → SQLCipher tuning (MEDIUM)
- **Implementation Approach**: Greenfield - rust-vault directory does not exist yet

**Research Findings**:
- **Oracle Analysis**: Identified 7 total security issues (4 original + 3 additional)
- **Metis Review**: Identified 4 critical blocker questions that must be answered before implementation

### Metis Review
**Critical Blockers Identified**:
1. **Database Access Strategy**: Shared vault.db vs gRPC keystore proxy?
2. **Placeholder Format**: Concrete syntax and examples needed
3. **mTLS Certificate Strategy**: Source, rotation, scope?
4. **Key Derivation Parameters**: Algorithm, iterations, salt strategy?

**Additional Guardrails**:
- MUST NOT implement advanced placeholder features (conditionals, loops, nesting)
- MUST NOT intercept WebSocket messages or document.write()
- MUST NOT cache secrets beyond request lifecycle
- MUST NOT log secret values

---

## Work Objectives

### Core Objective
Build Rust Vault v6.0 Enterprise Sovereign from scratch with 7 security fixes integrated into the architecture from day one.

### Concrete Deliverables
- `rust-vault/Cargo.toml` - Project configuration with- `rust-vault/src/config.rs` - Configuration system
- `rust-vault/src/error.rs` - Error types
- `rust-vault/src/db/pool.rs` - SQLCipher connection pool
- `rust-vault/src/db/vault.rs` - Vault database operations
- `rust-vault/src/db/matrix_state.rs` - Matrix state operations
- `rust-vault/src/blindfill/cdp_interceptor.rs` - CDP interceptor
- `rust-vault/src/blindfill/placeholder.rs` - Placeholder parser
- `rust-vault/src/grpc/server.rs` - gRPC server
- `rust-vault/src/grpc/middleware/rate_limit.rs` - Rate limiting
- `rust-vault/src/grpc/middleware/concurrency.rs` - Concurrency limits

### Definition of Done
- [ ] All 7 security fixes implemented and verified
- [ ] All tests pass: `cargo test --all`
- [ ] No clippy warnings: `cargo clippy -- -D warnings`
- [ ] Integration tests verify BlindFill flow end-to-end

### Must Have
- CDP interception filtered by resourceType (XHR, Fetch only)
- gRPC mTLS authentication on Unix socket
- All secret strings zeroized with `zeroize` crate
- Unix socket permissions enforced to 0600
- matrix_state.db key derivation specified
- Token bucket using atomic operations
- SQLCipher pragmas: cipher_plaintext_header_size=32, synchronous=NORMAL

### Must NOT Have (Guardrails)
- Advanced placeholder features (conditionals, loops, nested placeholders)
- WebSocket message interception
- document.write() or innerHTML interception
- Comprehensive observability (Prometheus, tracing) - basic logging only
- Circuit breakers or advanced retry logic
- Secret caching beyond request lifecycle
- Secret values in logs

---

## Verification Strategy (MANDATORY)

> **ZERO HUMAN INTERVENTION** — ALL verification is agent-executed. No exceptions.

### Test Decision
- **Infrastructure exists**: NO (greenfield project)
- **Automated tests**: YES (TDD)
- **Framework**: `cargo test` with `tokio::test` for async tests
- **TDD Approach**: Each task follows RED (failing test) → GREEN (minimal impl) → REFACTOR

### QA Policy
Every task MUST include agent-executed QA scenarios.
Evidence saved to `.sisyphus/evidence/task-{N}-{scenario-slug}.{ext}`.

- **Library/Module**: Use Bash (cargo test) — Run tests, verify pass
- **gRPC**: Use Bash (grpcurl) — Test endpoints, verify auth
- **Database**: Use Bash (sqlite3) — Verify pragmas, data integrity
- **CDP**: Use Playwright — Test browser integration, verify placeholder resolution

---

## Execution Strategy

### Parallel Execution Waves

```
Wave 0 (BLOCKER - Must complete before any implementation):
├── Task 0.1: Database access strategy [collaborative]
├── Task 0.2: Placeholder format specification [collaborative]
├── Task 0.3: mTLS certificate strategy [collaborative]
└── Task 0.4: Key derivation parameters [collaborative]

Wave 1 (After Wave 0 — foundation):
├── Task 1.1: Project scaffolding [quick]
├── Task 1.2: Config and error types [quick]
└── Task 1.3: SQLCipher DB pool [deep]

Wave 2 (After Wave 1 — core databases):
├── Task 2.1: Vault DB with zeroizing [deep]
└── Task 2.2: Matrix State DB with key derivation [deep]

Wave 3 (After Wave 2 — placeholder parser):
└── Task 3.1: Placeholder parser [quick]

Wave 4 (After Wave 3 — gRPC server and middleware):
├── Task 4.1: gRPC server with Unix socket [deep]
├── Task 4.2: gRPC mTLS authentication [deep]
├── Task 4.3: Token bucket rate limiting [unspecified-low]
└── Task 4.4: Concurrency limits [quick]

Wave 5 (After Wave 4 — CDP interceptor):
├── Task 5.1: CDP interceptor with resourceType filtering [deep]
└── Task 5.2: Placeholder resolution integration [deep]

Wave FINAL (After ALL tasks — verification):
├── Task F1: Plan compliance audit (oracle)
├── Task F2: Code quality review (unspecified-high)
├── Task F3: Integration tests (unspecified-high)
└── Task F4: Security verification (deep)
-> Present results -> Get explicit user okay

Critical Path: Wave 0 → Task 1.1 → Task 1.3 → Task 2.1 → Task 4.1 → Task 5.1 → Wave FINAL
Parallel Speedup: ~60% faster than sequential
Max Concurrent: 4 (Waves 1, 4, 5, FINAL)
```

### Dependency Matrix

| Task | Depends On | Blocks |
|------|------------|--------|
| 0.1-0.4 | — | All Wave 1+ tasks |
| 1.1 | 0.1, 0.4 | 1.2, 1.3 |
| 1.2 | 1.1 | 1.3 |
| 1.3 | 1.2, 0.4 | 2.1, 2.2 |
| 2.1 | 1.3, 0.1 | 3.1, 5.2 |
| 2.2 | 1.3, 0.4 | 4.1 |
| 3.1 | 2.1, 0.2 | 5.2 |
| 4.1 | 2.2 | 4.2, 4.3, 4.4 |
| 4.2 | 4.1, 0.3 | 5.1 |
| 4.3 | 4.2 | 5.1 |
| 4.4 | 4.2 | 5.1 |
| 5.1 | 4.4, 0.2 | 5.2 |
| 5.2 | 5.1, 3.1 | Wave FINAL |
| F1-F4 | All tasks | — |

### Agent Dispatch Summary

- **Wave 0**: **4** — T0.1-T0.4 → `collaborative`
- **Wave 1**: **3** — T1.1 → `quick`, T1.2 → `quick`, T1.3 → `deep`
- **Wave 2**: **2** — T2.1 → `deep`, T2.2 → `deep`
- **Wave 3**: **1** — T3.1 → `quick`
- **Wave 4**: **4** — T4.1 → `deep`, T4.2 → `deep`, T4.3 → `unspecified-low`, T4.4 → `quick`
- **Wave 5**: **2** — T5.1 → `deep`, T5.2 → `deep`
- **FINAL**: **4** — F1 → `oracle`, F2 → `unspecified-high`, F3 → `unspecified-high`, F4 → `deep`

---

## TODOs

### Wave 0: Critical Questions (BLOCKER - Must Complete First)

- [x] 0.1. Database Access Strategy

  **What to do**:
  - Determine whether Rust Vault shares vault.db with Go Bridge or uses gRPC keystore proxy
  - Document decision in config.rs
  - If gRPC proxy: define proto file for keystore service

  **Must NOT do**:
  - Assume shared DB access without confirmation
  - Start implementation before decision is made

  **Recommended Agent Profile**:
  - **Category**: `unspecified-low`
    - Reason: Collaborative question-gathering task, no code changes
  - **Skills**: []
  - **Skills Evaluated but Omitted**: All implementation skills - not applicable

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 0 (with Tasks 0.2, 0.3, 0.4)
  - **Blocks**: All Wave 1+ tasks
  - **Blocked By**: None

  **References**:
  - `AGENTS.md` - Project architecture constraints
  - Go Bridge keystore location (if exists): `bridge/pkg/store/`

  **Acceptance Criteria**:
  - [ ] Decision documented: shared DB OR gRPC proxy
  - [ ] If gRPC: proto file defined

  **QA Scenarios**:
  ```
  Scenario: Database access strategy decision
    Tool: Read
    Preconditions: User has answered database access question
    Steps:
      1. Read config.rs or planning notes
      2. Verify decision is documented
      3. If gRPC: verify proto file exists
    Expected Result: Clear decision documented
    Evidence: .sisyphus/evidence/task-0.1-db-strategy.txt
  ```

  **Commit**: NO (planning phase)

- [x] 0.2. Placeholder Format Specification

  **What to do**:
  - Get concrete placeholder syntax and examples from user
  - Document format in blindfill/placeholder.rs as comments
  - Collect 5-10 example placeholders

  **Must NOT do**:
  - Assume format without examples
  - Design complex nested/conditional syntax

  **Recommended Agent Profile**:
  - **Category**: `unspecified-low`
    - Reason: Requirements gathering, no implementation
  - **Skills**: []
  - **Skills Evaluated but Omitted**: All implementation skills

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 0 (with Tasks 0.1, 0.3, 0.4)
  - **Blocks**: Task 3.1 (placeholder parser)
  - **Blocked By**: None

  **References**:
  - Oracle analysis recommendations
  - BlindFill™ documentation (if exists)

  **Acceptance Criteria**:
  - [ ] Placeholder syntax documented (e.g., `{{secret:name}}`, `[[secret:name]]`)
  - [ ] 5-10 examples provided
  - [ ] Scope confirmed: flat lookups only (no nesting/conditionals)

  **QA Scenarios**:
  ```
  Scenario: Placeholder format specification
    Tool: Read
    Preconditions: User has provided placeholder examples
    Steps:
      1. Read placeholder.rs or planning notes
      2. Verify syntax is documented
      3. Verify examples are listed
    Expected Result: Clear format spec with examples
    Evidence: .sisyphus/evidence/task-0.2-placeholder-format.txt
  ```

  **Commit**: NO (planning phase)

- [x] 0.3 mTLS Certificate Strategy

  **What to do**:
  - Determine certificate source (Go Bridge generates? External PKI?)
  - Define rotation strategy
  - Confirm scope (per-client vs shared certificate)

  **Must NOT do**:
  - Assume self-signed certificates without confirmation
  - Implement before strategy is defined

  **Recommended Agent Profile**:
  - **Category**: `unspecified-low`
    - Reason: Requirements gathering, no implementation
  - **Skills**: []
  - **Skills Evaluated but Omitted**: All implementation skills

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 0 (with Tasks 0.1, 0.2, 0.4)
  - **Blocks**: Task 4.2 (gRPC mTLS)
  - **Blocked By**: None

  **References**:
  - Go Bridge gRPC setup: `bridge/pkg/rpc/`
  - tonic mTLS documentation

  **Acceptance Criteria**:
  - [ ] Certificate source documented
  - [ ] Rotation strategy defined
  - [ ] Scope confirmed (per-client vs shared)

  **QA Scenarios**:
  ```
  Scenario: mTLS certificate strategy
    Tool: Read
    Preconditions: User has answered certificate questions
    Steps:
      1. Read grpc/middleware/auth.rs or planning notes
      2. Verify strategy is documented
      3. Verify rotation approach is defined
    Expected Result: Clear mTLS strategy documented
    Evidence: .sisyphus/evidence/task-0.3-mtls-strategy.txt
  ```

  **Commit**: NO (planning phase)

- [x] 0.4 Key Derivation Parameters

  **What to do**:
  - Get exact algorithm (PBKDF2-HMAC-SHA256? Argon2? HKDF?)
  - Confirm iteration count and parameters
  - Verify compatibility with Go Bridge implementation

  **Must NOT do**:
  - Assume algorithm without confirmation
  - Use parameters that don't match Go Bridge

  **Recommended Agent Profile**:
  - **Category**: `unspecified-low`
    - Reason: Requirements gathering, no implementation
  - **Skills**: []
  - **Skills Evaluated but Omitted**: All implementation skills

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 0 (with Tasks 0.1, 0.2, 0.3)
  - **Blocks**: Task 1.3 (DB pool), Task 2.2 (Matrix State DB)
  - **Blocked By**: None

  **References**:
  - Go Bridge SQLCipher config: `bridge/pkg/store/sqlcipher.go`
  - SQLCipher key derivation docs

  **Acceptance Criteria**:
  - [ ] Algorithm specified (e.g., PBKDF2-HMAC-SHA256)
  - [ ] Iteration count defined
  - [ ] Salt strategy documented
  - [ ] Compatibility with Go Bridge verified

  **QA Scenarios**:
  ```
  Scenario: Key derivation parameters
    Tool: Read
    Preconditions: User has provided key derivation specs
    Steps:
      1. Read db/matrix_state.rs or planning notes
      2. Verify algorithm is documented
      3. Verify parameters match Go Bridge
    Expected Result: Clear key derivation spec
    Evidence: .sisyphus/evidence/task-0.4-key-derivation.txt
  ```

  **Commit**: NO (planning phase)

### Wave 1: Foundation (After Wave 0 Completes)

- [x] 1.1.Create Rust Vault Project Structure

  **What to do**:
  - Initialize Cargo project: `cargo init rust-vault`
  - Add dependencies to Cargo.toml: tokio, tonic, prost, zeroize, sqlcipher (rusqlite with sqlcipher feature)
  - Create directory structure: src/, src/db/, src/blindfill/, src/grpc/, src/grpc/middleware/
  - Set up basic .gitignore for Rust

  **TDD - Test First**:
  ```rust
  // tests/integration_test.rs
  #[tokio::test]
  async fn test_project_compiles() {
      let output = Command::new("cargo")
          .args(&["check"])
          .current_dir(Path::new("rust-vault"))
          .output()
          .expect("Failed to run cargo check");
      assert!(output.status.success());
  }
  ```

  **Must NOT do**:
  - Add unnecessary dependencies
  - Set up complex build scripts

  **Recommended Agent Profile**:
  - **Category**: `quick`
    - Reason: Straightforward project initialization, standard Cargo setup
  - **Skills**: []
  - **Skills Evaluated but Omitted**: All domain-specific skills

  **Parallelization**:
  - **Can Run In Parallel**: NO
  - **Parallel Group**: Sequential (must complete first in Wave 1)
  - **Blocks**: Tasks 1.2, 1.3
  - **Blocked By**: Wave 0 (all tasks)

  **References**:
  - Cargo book: https://doc.rust-lang.org/cargo/
  - tokio docs: https://docs.rs/tokio/
  - tonic docs: https://docs.rs/tonic/
  - zeroize docs: https://docs.rs/zeroize/

  **Acceptance Criteria**:
  - [ ] rust-vault/Cargo.toml exists with all dependencies
  - [ ] `cargo check` succeeds
  - [ ] Directory structure created
  - [ ] .gitignore exists

  **QA Scenarios**:
  ```
  Scenario: Project compiles successfully
    Tool: Bash
    Preconditions: Cargo.toml created with dependencies
    Steps:
      1. cd rust-vault
      2. cargo check
      3. Verify exit code 0
    Expected Result: cargo check exits with code 0
    Failure Indicators: Compilation errors, missing dependencies
    Evidence: .sisyphus/evidence/task-1.1-cargo-check.log

  Scenario: All required dependencies present
    Tool: Bash
    Preconditions: Cargo.toml exists
    Steps:
      1. grep -E "tokio|tonic|prost|zeroize|rusqlite" rust-vault/Cargo.toml
      2. Verify all dependencies listed
    Expected Result: All 5+ dependencies found in Cargo.toml
    Failure Indicators: Missing dependency
    Evidence: .sisyphus/evidence/task-1.1-dependencies.txt
  ```

  **Commit**: YES
  - Message: `feat(vault): initialize Rust Vault project structure`
  - Files: Cargo.toml, .gitignore, src/lib.rs
  - Pre-commit: `cargo check`

- [x] 1.2. Implement Config and Error Types

  **What to do**:
  - Create src/config.rs with configuration struct (database paths, gRPC socket path, rate limits)
  - Create src/error.rs with error types (DatabaseError, GrpcError, CdpError, etc.)
  - Implement From traits for error conversions
  - Add secure config loading (no secrets in config)

  **TDD - Test First**:
  ```rust
  // tests/config_test.rs
  #[test]
  fn test_config_defaults() {
      let config = Config::default();
      assert_eq!(config.grpc_socket_path, "/tmp/rust-vault.sock");
      assert_eq!(config.vault_db_path, "vault.db");
  }

  #[test]
  fn test_error_display() {
      let err = VaultError::Database("connection failed".to_string());
      assert!(format!("{}", err).contains("connection failed"));
  }
  ```

  **Must NOT do**:
  - Store secrets in config file
  - Use unwrap() in error paths
  - Log sensitive configuration values

  **Recommended Agent Profile**:
  - **Category**: `quick`
    - Reason: Standard Rust struct/enum definitions, straightforward implementation
  - **Skills**: []
  - **Skills Evaluated but Omitted**: All domain-specific skills

  **Parallelization**:
  - **Can Run In Parallel**: NO
  - **Parallel Group**: Sequential (after Task 1.1)
  - **Blocks**: Task 1.3
  - **Blocked By**: Task 1.1

  **References**:
  - Rust error handling: https://doc.rust-lang.org/book/ch09-00-error-handling.html
  - thiserror crate: https://docs.rs/thiserror/

  **Acceptance Criteria**:
  - [ ] src/config.rs exists with Config struct
  - [ ] src/error.rs exists with VaultError enum
  - [ ] `cargo test --lib config error` passes
  - [ ] No secrets in config

  **QA Scenarios**:
  ```
  Scenario: Config loads successfully
    Tool: Bash
    Preconditions: config.rs implemented
    Steps:
      1. cd rust-vault
      2. cargo test --lib config
      3. Verify all config tests pass
    Expected Result: All config tests pass
    Failure Indicators: Test failures
    Evidence: .sisyphus/evidence/task-1.2-config-tests.log

  Scenario: Error types implement Display
    Tool: Bash
    Preconditions: error.rs implemented
    Steps:
      1. cd rust-vault
      2. cargo test --lib error
      3. Verify error display tests pass
    Expected Result: All error tests pass
    Failure Indicators: Test failures
    Evidence: .sisyphus/evidence/task-1.2-error-tests.log
  ```

  **Commit**: YES
  - Message: `feat(vault): add config and error types`
  - Files: src/config.rs, src/error.rs
  - Pre-commit: `cargo test --lib`

- [x] 1.3. Set Up SQLCipher DB Pool

  **What to do**:
  - Create src/db/pool.rs with connection pool manager
  - Implement SQLCipher connection with security pragmas:
    - `cipher_plaintext_header_size = 32`
    - `synchronous = NORMAL`
    - `wal_autocheckpoint = 1000`
  - Add connection pooling with tokio::sync
  - Use key derivation from Wave 0 decision

  **TDD - Test First**:
  ```rust
  // tests/db_pool_test.rs
  #[tokio::test]
  async fn test_sqlcipher_pragmas_applied() {
      let pool = DbPool::new("test.db", "test_key").await.unwrap();
      let conn = pool.get().await.unwrap();
      
      let header_size: i64 = conn.query_row(
          "PRAGMA cipher_plaintext_header_size", 
          [], 
          |row| row.get(0)
      ).unwrap();
      assert_eq!(header_size, 32);
      
      let synchronous: String = conn.query_row(
          "PRAGMA synchronous", 
          [], 
          |row| row.get(0)
      ).unwrap();
      assert_eq!(synchronous, "NORMAL");
  }
  ```

  **Must NOT do**:
  - Use default SQLCipher settings
  - Skip WAL mode
  - Hardcode keys in code

  **Recommended Agent Profile**:
  - **Category**: `deep`
    - Reason: SQLCipher integration with security pragmas requires careful configuration and testing
  - **Skills**: []
  - **Skills Evaluated but Omitted**: All non-database skills

  **Parallelization**:
  - **Can Run In Parallel**: NO
  - **Parallel Group**: Sequential (after Task 1.2)
  - **Blocks**: Tasks 2.1, 2.2
  - **Blocked By**: Task 1.2, Wave 0 Task 0.4 (key derivation params)

  **References**:
  - SQLCipher docs: https://www.zetetic.net/sqlcipher/sqlcipher-api/
  - rusqlite: https://docs.rs/rusqlite/
  - Pragma reference: `PRAGMA cipher_plaintext_header_size`, `PRAGMA synchronous`

  **Acceptance Criteria**:
  - [ ] src/db/pool.rs exists
  - [ ] cipher_plaintext_header_size = 32 verified
  - [ ] synchronous = NORMAL verified
  - [ ] wal_autocheckpoint = 1000 set
  - [ ] Connection pool works
  - [ ] `cargo test --test db_pool` passes

  **QA Scenarios**:
  ```
  Scenario: SQLCipher pragmas correctly applied
    Tool: Bash
    Preconditions: db/pool.rs implemented with pragmas
    Steps:
      1. cd rust-vault
      2. cargo test --test db_pool -- --nocapture
      3. Verify pragma test passes
    Expected Result: Pragma tests pass, values match spec
    Failure Indicators: Wrong pragma values, missing pragmas
    Evidence: .sisyphus/evidence/task-1.3-db-pool-tests.log

  Scenario: Connection pool functional
    Tool: Bash
    Preconditions: Pool implemented
    Steps:
      1. cd rust-vault
      2. cargo test --test db_pool test_pool_operations
      3. Verify pool get/release works
    Expected Result: Pool operations succeed
    Failure Indicators: Pool exhaustion, connection failures
    Evidence: .sisyphus/evidence/task-1.3-pool-ops.log
  ```

  **Commit**: YES
  - Message: `feat(vault): implement SQLCipher DB pool with security pragmas`
  - Files: src/db/pool.rs, src/db/mod.rs
  - Pre-commit: `cargo test --test db_pool`

---

## Final Verification Wave (MANDATORY — after ALL implementation tasks)

> 4 review agents run in PARALLEL. ALL must APPROVE. Present consolidated results to user and get explicit "okay" before completing.

- [ ] F1. **Plan Compliance Audit** — `oracle`
  Read the plan end-to-end. For each "Must Have": verify implementation exists. For each "Must NOT Have": search codebase for forbidden patterns — reject with file:line if found. Check evidence files exist. Compare deliverables against plan.
  Output: `Must Have [N/N] | Must NOT Have [N/N] | Tasks [N/N] | VERDICT: APPROVE/REJECT`

- [ ] F2. **Code Quality Review** — `unspecified-high`
  Run `cargo test --all` + `cargo clippy -- -D warnings`. Review all changed files for: `unsafe` code, empty catches, unwrap() on secrets, unused imports. Check AI slop: excessive comments, over-abstraction, generic names.
  Output: `Build [PASS/FAIL] | Lint [PASS/FAIL] | Tests [N pass/N fail] | Files [N clean/N issues] | VERDICT`

- [ ] F3. **Integration Tests** — `unspecified-high` (+ `playwright` skill if browser testing)
  Start from clean state. Execute EVERY QA scenario from EVERY task — follow exact steps, capture evidence. Test cross-task integration. Test edge cases: empty state, invalid input, rapid actions. Save to `.sisyphus/evidence/final-qa/`.
  Output: `Scenarios [N/N pass] | Integration [N/N] | Edge Cases [N tested] | VERDICT`

- [ ] F4. **Security Verification** — `deep`
  For each security fix: verify CDP filtering (no wildcard), mTLS auth required, zeroization applied (valgrind), socket permissions 0600, key derivation correct, token bucket atomic, SQLCipher pragmas set. Attempt unauthorized access, verify rejection.
  Output: `Security [N/N verified] | Unauthorized Access [N/N blocked] | VERDICT`

---

## Commit Strategy

- **Wave 0**: No commits (planning phase)
- **Wave 1**: `feat(vault): initialize project with config and error types` — Cargo.toml, config.rs, error.rs
- **Wave 2**: 
  - `feat(vault): implement SQLCipher DB pool with security pragmas` — db/pool.rs
  - `feat(vault): add vault DB with zeroized secret handling` — db/vault.rs
  - `feat(vault): add matrix state DB with key derivation` — db/matrix_state.rs
- **Wave 3**: `feat(vault): implement placeholder parser` — blindfill/placeholder.rs
- **Wave 4**:
  - `feat(vault): add gRPC server with Unix socket` — grpc/server.rs
  - `feat(vault): implement mTLS authentication` — grpc/middleware/auth.rs
  - `feat(vault): add token bucket rate limiting` — grpc/middleware/rate_limit.rs
  - `feat(vault): add concurrency limits` — grpc/middleware/concurrency.rs
- **Wave 5**: 
  - `feat(vault): implement CDP interceptor with resourceType filtering` — blindfill/cdp_interceptor.rs
  - `feat(vault): integrate placeholder resolution` — blindfill/integration.rs

---

## Success Criteria

### Verification Commands
```bash
cd rust-vault && cargo test --all           # Expected: All tests pass
cd rust-vault && cargo clippy -- -D warnings # Expected: No warnings
cd rust-vault && cargo build --release     # Expected: Clean build
```

### Final Checklist
- [ ] All "Must Have" present (7 security fixes)
- [ ] All "Must NOT Have" absent (guardrails respected)
- [ ] All tests pass
- [ ] No clippy warnings
- [ ] Integration tests verify BlindFill flow
- [ ] Security verification confirms all 7 fixes
