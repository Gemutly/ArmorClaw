# ToolSidecar MCP Phase 1 - Protocol Translation Layer

## TL;DR

> **Quick Summary**: Implement air-gapped ToolSidecar architecture with protocol translation (OpenClaw Native RPC → MCP), strict egress proxy (LLM APIs only), and network isolation. Front-load Rust Vault binary fixes to verify compilation within 48 hours.
>
> **Deliverables**:
> - Go Bridge HTTP Egress Proxy (DENY ALL by default, allowlist LLM APIs)
> - OpenClaw Network Isolation (Docker `armorclaw-isolated` network)
> - Protocol Translator (Native JSON-RPC → Anthropic MCP)
> - MCP Router with Rust Vault SkillGate integration
> - Fixed Rust Vault binary (resolve 74 compilation errors)
> - ToolSidecar provisioning system
> - Session lifecycle management (orphan cleanup on crash)
>
> **Estimated Effort**: Medium (1-2 weeks)
> **Parallel Execution**: YES - 6 tasks in 4 waves
> **Critical Path**: Task 5 (Rust Binary) → Task 6 (Protocol Translator) → Task 8 (MCP Router) → Integration

---

## Context

### Original Request

Implement ToolSidecar MCP Architecture v6.0 to prevent Confused Deputy attacks by air-gapping OpenClaw and forcing all tool calls through a secure protocol translation layer.

### Executive Decision

**Option 3 Modified**: Protocol Translation + Strict Egress Proxy

```
┌────────────────────┐    Native RPC     ┌────────────────────────┐
│  OPENCLAW          │ ────────────────▶ │  GO BRIDGE             │
│  Net: Isolated     │                   │  1. Egress Proxy (LLM) │
│  (LLM APIs ONLY)   │ ◀──────────────── │  2. Protocol Translator│
└────────────────────┘                   │  3. MCP Router         │
                                         └──────────┬─────────────┘
                                                    │ gRPC (UDS)
                                                    ▼
                                         ┌────────────────────────┐
                                         │  RUST VAULT            │
                                         │  (The Governor)        │
                                         │  - ShadowMap PII       │
                                         └──────────┬─────────────┘
                                                    │ MCP (stdio)
                                                    ▼
                                         ┌────────────────────────┐
                                         │  TOOLSIDECAR           │
                                         │  (The Hands)           │
                                         │  Net: Egress           │
                                         └────────────────────────┘
```

### CTO-Level Constraints

1. **Egress Proxy Strictness (Fail Closed)**
   - Default policy: DENY ALL outbound connections
   - Allowlist: Exact SNI/host matches for `api.openopenai.com`, `api.anthropic.com` only
   - All dropped requests MUST be logged to audit.db (exfiltration attempt indicators)

2. **Rust Vault Timebox**
   - Task 5 (Binary Fixes) FRONT-LOADED to first 48 hours
   - Must verify Docker build environment fixes (libsqlite3-dev, libsqlcipher-dev, protobuf-compiler, bundled-sqlcipher) resolve all 74 compilation errors
   - If not resolved within 48h, escalate to CTO immediately

3. **Session Lifecycle Management**
   - Protocol Translator MUST gracefully tear down orphaned ToolSidecars
   - On OpenClaw crash/restart: cleanup pending MCP executions
   - Prevent resource leaks from crashed sessions

### Research Findings

**OpenClaw MCP Support**: ❌ NOT SUPPORTED
- OpenClaw explicitly disables MCP capabilities (`http: false, sse: false`)
- Ignores MCP server configuration
- Must use native JSON-RPC with protocol translation

**Rust Vault Binary Status**: ⚠️ 74 Compilation Errors
- Root causes: Missing protoc, SQLCipher C-bindings
- Solution: Install protobuf-compiler, use bundled-sqlcipher feature

**MCP Protocol**: Anthropic Model Context Protocol
- Specification: https://modelcontextprotocol.io/specification/2025-11-25/
- Official Go SDK: `github.com/modelcontextprotocol/go-sdk` v1.5.0
- Transport: stdio over Unix domain sockets

---

## Work Objectives

### Core Objective

Implement secure protocol translation layer that:
1. Air-gaps OpenClaw (isolated Docker network)
2. Strictly controls egress (LLM APIs only via Go Bridge proxy)
3. Translates OpenClaw's native JSON-RPC to Anthropic MCP
4. Routes all tool calls through Rust Vault SkillGate
5. Executes tools in isolated ToolSidecar containers

### Concrete Deliverables

- `bridge/pkg/proxy/egress.go` - HTTP egress proxy with DENY ALL default
- `bridge/pkg/proxy/allowlist.go` - Exact SNI matching for LLM APIs
- `docker/networks/armorclaw-isolated.network` - Isolated Docker network config
- `bridge/pkg/translator/rpc_to_mcp.go` - Native RPC → MCP translator
- `bridge/pkg/translator/session_manager.go` - Session lifecycle with orphan cleanup
- `bridge/pkg/mcp/router.go` - MCP request router with SkillGate integration
- `sidecar/Cargo.toml` - Fixed dependencies (bundled-sqlcipher)
- `sidecar/Dockerfile` - Updated build environment
- `bridge/pkg/toolsidecar/provisioner.go` - Container provisioning system

### Definition of Done

- [ ] OpenClaw container on `armorclaw-isolated` network with no external gateway
- [ ] Go Bridge proxy drops all non-LLM traffic (verified with test exfiltration attempt)
- [ ] Protocol translator converts `browser.navigate` to MCP `tools/call`
- [ ] All tool calls pass through Rust Vault SkillGate (verified in audit logs)
- [ ] Rust Vault binary compiles with 0 errors
- [ ] ToolSidecar containers spawn on demand and terminate on session end
- [ ] Orphaned ToolSidecars cleaned up within 60 seconds of OpenClaw crash

### Must Have

- **DENY ALL egress policy** - No exceptions
- **Audit logging** for all dropped requests (exfiltration indicators)
- **Rust Vault binary** compiling successfully
- **Protocol translation** for all 47 existing JSON-RPC methods
- **Session cleanup** on crash/restart

### Must NOT Have (Guardrails)

- **NO** bypass of egress proxy (OpenClaw must use HTTP_PROXY)
- **NO** allowlist wildcards (*.openai.com forbidden - exact matches only)
- **NO** orphaned ToolSidecar containers after crash
- **NO** compilation errors in Rust Vault binary
- **NO** prompt injection detection in Phase 1 (deferred to Phase 2)

---

## Verification Strategy (MANDATORY)

> **ZERO HUMAN INTERVENTION** — ALL verification is agent-executed. No exceptions.

### Test Decision

- **Infrastructure exists**: YES (Go test framework, Rust cargo test)
- **Automated tests**: YES (TDD approach for new code)
- **Framework**: Go: `testing` package, Rust: `cargo test`
- **TDD**: Each new package starts with failing tests, then implementation

### QA Policy

Every task MUST include agent-executed QA scenarios.
Evidence saved to `.sisyphus/evidence/task-{N}-{scenario-slug}.{ext}`.

- **Backend/Proxy**: Use Bash (curl) — Send requests, assert status codes, verify allowlist/deny behavior
- **Docker/Network**: Use Bash (docker) — Inspect networks, verify isolation, test connectivity
- **Protocol Translation**: Use Bash (custom test client) — Send JSON-RPC, verify MCP output
- **Integration**: Use Bash (end-to-end) — Full flow from OpenClaw → ToolSidecar

---

## Execution Strategy

### Parallel Execution Waves

> Maximize throughput by grouping independent tasks into parallel waves.
> Critical path: Task 5 (Rust Binary) → Task 6 (Translator) → Task 8 (Router) → Integration

```
Wave 0 (FRONT-LOADED - Start Immediately - CRITICAL PATH):
├── Task 5: Rust Vault Binary Fixes [deep]
    - MUST complete within 48 hours
    - Blocker for all downstream work
    - Verify 0 compilation errors

Wave 1 (After Wave 0 - Foundation):
├── Task 1: Docker Network Isolation [quick]
├── Task 2: Egress Proxy Core (DENY ALL) [quick]
├── Task 3: Egress Allowlist (LLM APIs) [quick]
└── Task 4: Audit Logging for Dropped Requests [quick]

Wave 2 (After Wave 1 - Core Translation):
├── Task 6: Protocol Translator (RPC → MCP) [deep]
├── Task 7: Session Lifecycle Manager [deep]
└── Task 9: ToolSidecar Provisioner [unspecified-high]

Wave 3 (After Wave 2 - Integration):
├── Task 8: MCP Router with SkillGate [deep]
├── Task 10: End-to-End Integration Test [deep]
└── Task 11: OpenClaw Crash Recovery Test [deep]

Wave FINAL (After ALL tasks - 4 parallel reviews):
├── Task F1: Plan Compliance Audit (oracle)
├── Task F2: Code Quality Review (unspecified-high)
├── Task F3: Security Audit (unspecified-high)
└── Task F4: Real Manual QA (unspecified-high)
-> Present results -> Get explicit user okay

Critical Path: Task 5 → Task 6 → Task 8 → Task 10 → F1-F4 → user okay
Parallel Speedup: ~60% faster than sequential
Max Concurrent: 4 (Waves 1 & 2)
```

### Dependency Matrix

- **5**: — — 6, 8, 2
- **1**: — — 6, 10, 1
- **2**: — — 3, 4, 1
- **3**: 2 — 10, 1
- **4**: 2 — 8, 10, 1
- **6**: 5, 1 — 8, 10, 1
- **7**: 6 — 8, 11, 1
- **8**: 5, 6, 4 — 10, 1
- **9**: — — 8, 10, 1
- **10**: 1, 3, 6, 8, 9 — 11, 1
- **11**: 7, 10 — F1-F4, 1

### Agent Dispatch Summary

- **Wave 0**: **1** — T5 → `deep`
- **Wave 1**: **4** — T1 → `quick`, T2 → `quick`, T3 → `quick`, T4 → `quick`
- **Wave 2**: **3** — T6 → `deep`, T7 → `deep`, T9 → `unspecified-high`
- **Wave 3**: **3** — T8 → `deep`, T10 → `deep`, T11 → `deep`
- **FINAL**: **4** — F1 → `oracle`, F2 → `unspecified-high`, F3 → `unspecified-high`, F4 → `unspecified-high`

---

## TODOs

---

### Wave 0: FRONT-LOADED (Critical Path)

- [x] **5. Rust Vault Binary Fixes** [deep] - **MUST COMPLETE IN FIRST 48 HOURS**

  **What to do**:
  - Fix Docker build environment to resolve 74 compilation errors
  - Install missing build dependencies: `protobuf-compiler`, `libsqlite3-dev`, `libsqlcipher-dev`
  - Update `sidecar/Cargo.toml` to use `bundled-sqlcipher` feature for sqlx
  - Verify `protoc` is accessible during build
  - Run `cargo build --release` and confirm 0 errors

  **Must NOT do**:
  - Do NOT skip protobuf installation (causes cascading type errors)
  - Do NOT use system SQLCipher without bundled feature (FFI issues)
  - Do NOT proceed to Wave 1 until binary compiles successfully

  **Recommended Agent Profile**:
  - **Category**: `deep`
    - Reason: Complex build environment debugging, dependency resolution, Rust FFI compilation issues
  - **Skills**: []
    - No specific skills needed - standard Rust/Docker build work

  **Parallelization**:
  - **Can Run In Parallel**: NO - Critical path blocker
  - **Parallel Group**: Sequential (Wave 0 only)
  - **Blocks**: ALL downstream tasks (6, 8, 10)
  - **Blocked By**: None (can start immediately)

  **References**:

  > Build environment fixes needed to unblock compilation

  **Build Dependencies**:
  - `sidecar/Dockerfile:15-25` - Add `apt-get install -y protobuf-compiler libsqlite3-dev libsqlcipher-dev`
  - `sidecar/Cargo.toml:25-30` - Update sqlx dependency to use `bundled-sqlcipher` feature

  **Compilation Error Sources**:
  - Missing `protoc` → tonic-build fails → cascading gRPC type errors
  - Missing SQLCipher C-bindings → sqlx compilation failures
  - OpenSSL vs rustls conflict → Azure Blob connector disabled

  **Why This Matters**:
  - Without protoc, gRPC code generation fails, causing 50+ type errors
  - Without bundled-sqlcipher, FFI linking fails on some platforms
  - These are infrastructure issues, not architectural problems

  **Acceptance Criteria**:

  **If TDD (tests enabled):**
  - [ ] Test file exists: `sidecar/tests/build_verification_test.rs`
  - [ ] `cargo test --test build_verification_test` → PASS

  **QA Scenarios (MANDATORY):**

  ```
  Scenario: Rust Vault binary compiles successfully
    Tool: Bash (cargo)
    Preconditions: Fresh Docker build environment
    Steps:
      1. cd sidecar && cargo build --release 2>&1 | tee build.log
      2. grep -c "error\[" build.log || echo "0 errors"
      3. test -f target/release/sidecar && echo "Binary exists"
    Expected Result: 0 compilation errors, binary exists at target/release/sidecar
    Failure Indicators: "error[E" in build.log, missing binary file
    Evidence: .sisyphus/evidence/task-5-compilation-success.log

  Scenario: Rust tests pass after fixes
    Tool: Bash (cargo test)
    Preconditions: Binary compiles successfully
    Steps:
      1. cd sidecar && cargo test --all 2>&1 | tee test.log
      2. grep "test result:" test.log
    Expected Result: 159/160 tests pass (99.4%)
    Failure Indicators: "test result: FAILED", less than 150 tests pass
    Evidence: .sisyphus/evidence/task-5-tests-pass.log

  Scenario: Protoc available in build environment
    Tool: Bash (which)
    Preconditions: Docker build environment set up
    Steps:
      1. which protoc && protoc --version
    Expected Result: protoc found, version 3.x or higher
    Failure Indicators: "protoc not found"
    Evidence: .sisyphus/evidence/task-5-protoc-installed.log
  ```

  **Evidence to Capture:**
  - [ ] Build log showing 0 errors: `task-5-compilation-success.log`
  - [ ] Test output showing 159/160 pass: `task-5-tests-pass.log`
  - [ ] Protoc version check: `task-5-protoc-installed.log`

  **Commit**: YES
  - Message: `fix(sidecar): resolve 74 compilation errors with bundled-sqlcipher and protoc`
  - Files: `sidecar/Dockerfile`, `sidecar/Cargo.toml`
  - Pre-commit: `cargo build --release && cargo test --lib`

---

### Wave 1: Foundation (After Task 5 Completes)

- [ ] **1. Docker Network Isolation** [quick]

  **What to do**:
  - Create Docker network `armorclaw-isolated` with no external gateway
  - Configure OpenClaw container to use isolated network
  - Set `HTTP_PROXY=http://go-bridge:3128` environment variable
  - Verify OpenClaw cannot reach external internet directly

  **Must NOT do**:
  - Do NOT add external gateway to armorclaw-isolated network
  - Do NOT allow OpenClaw to bypass HTTP_PROXY
  - Do NOT use bridge network mode for OpenClaw

  **Recommended Agent Profile**:
  - **Category**: `quick`
    - Reason: Simple Docker network configuration, well-documented patterns
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 1 (with Tasks 2, 3, 4)
  - **Blocks**: Task 6 (Protocol Translator needs isolated network)
  - **Blocked By**: None (can start after Task 5)

  **References**:

  **Docker Network Configuration**:
  - `deploy/docker-compose.yml:35-45` - OpenClaw service definition
  - Add network configuration: `networks: [armorclaw-isolated]`

  **Network Definition**:
  - Create new network section in docker-compose.yml:
    ```yaml
    networks:
      armorclaw-isolated:
        driver: bridge
        internal: true  # No external gateway
        ipam:
          config:
            - subnet: 172.28.0.0/16
    ```

  **Environment Variables**:
  - Add to OpenClaw service: `HTTP_PROXY=http://go-bridge:3128`
  - Add: `NO_PROXY=localhost,127.0.0.1,go-bridge`

  **Why This Matters**:
  - `internal: true` prevents any external internet access
  - OpenClaw MUST use Go Bridge proxy for LLM API calls
  - Prevents direct exfiltration attempts

  **Acceptance Criteria**:

  **QA Scenarios (MANDATORY):**

  ```
  Scenario: OpenClaw isolated network has no external gateway
    Tool: Bash (docker)
    Preconditions: Docker Compose up with new network
    Steps:
      1. docker network inspect armorclaw-isolated | jq '.[0].Internal'
      2. docker network inspect armorclaw-isolated | jq '.[0].IPAM.Config'
    Expected Result: Internal=true, no gateway in IPAM config
    Failure Indicators: Internal=false, gateway present
    Evidence: .sisyphus/evidence/task-1-network-isolated.log

  Scenario: OpenClaw cannot reach external internet directly
    Tool: Bash (docker exec)
    Preconditions: OpenClaw container running on isolated network
    Steps:
      1. docker exec openclaw curl -s --max-time 5 https://google.com 2>&1 || echo "FAILED"
    Expected Result: Connection timeout or failure (no external access)
    Failure Indicators: HTTP 200 response from google.com
    Evidence: .sisyphus/evidence/task-1-no-direct-egress.log

  Scenario: OpenClaw uses HTTP_PROXY environment variable
    Tool: Bash (docker exec)
    Preconditions: OpenClaw container running
    Steps:
      1. docker exec openclaw env | grep HTTP_PROXY
    Expected Result: HTTP_PROXY=http://go-bridge:3128
    Failure Indicators: HTTP_PROXY not set or different value
    Evidence: .sisyphus/evidence/task-1-proxy-env-set.log
  ```

  **Evidence to Capture:**
  - [ ] Network inspection showing internal=true: `task-1-network-isolated.log`
  - [ ] Failed external curl from OpenClaw: `task-1-no-direct-egress.log`
  - [ ] HTTP_PROXY environment variable set: `task-1-proxy-env-set.log`

  **Commit**: YES
  - Message: `feat(network): add armorclaw-isolated Docker network for OpenClaw`
  - Files: `deploy/docker-compose.yml`
  - Pre-commit: `docker-compose config --quiet`

---

- [ ] **2. Egress Proxy Core (DENY ALL)** [quick]

  **What to do**:
  - Create Go package `bridge/pkg/proxy/egress.go`
  - Implement HTTP proxy server on port 3128
  - Default policy: DENY ALL outbound connections
  - Log all denied requests to audit system
  - Return 403 Forbidden for non-allowlisted destinations

  **Must NOT do**:
  - Do NOT allow any outbound connections by default
  - Do NOT use wildcards in allowlist (*.domain forbidden)
  - Do NOT skip audit logging for denied requests

  **Recommended Agent Profile**:
  - **Category**: `quick`
    - Reason: Standard HTTP proxy implementation, clear requirements
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 1 (with Tasks 1, 3, 4)
  - **Blocks**: Task 3 (Allowlist depends on core proxy)
  - **Blocked By**: None (can start after Task 5)

  **References**:

  **Proxy Implementation Pattern**:
  - `bridge/pkg/rpc/server.go:100-150` - HTTP server pattern for Go Bridge
  - Use standard Go `net/http/httputil` for proxy functionality

  **Audit Logging Integration**:
  - `bridge/pkg/audit/audit.go:45-80` - Audit event logging
  - Create new event type: `EventEgressDenied`

  **HTTP Proxy Structure**:
  ```go
  type EgressProxy struct {
      allowlist *Allowlist
      auditor   *audit.Logger
      logger    *logger.Logger
  }

  func (p *EgressProxy) ServeHTTP(w http.ResponseWriter, r *http.Request) {
      // Extract destination host
      host := r.URL.Hostname()
      
      // Check allowlist
      if !p.allowlist.IsAllowed(host) {
          // Audit log the denial
          p.auditor.Log(audit.Event{
              Type:   audit.EventEgressDenied,
              Detail: fmt.Sprintf("host=%s path=%s", host, r.URL.Path),
          })
          http.Error(w, "403 Forbidden - Egress Denied", http.StatusForbidden)
          return
      }
      
      // Forward to allowed destination
      p.forwardRequest(w, r)
  }
  ```

  **Why This Matters**:
  - DENY ALL prevents exfiltration by default
  - Audit logs provide evidence of attempted breaches
  - Strict policy enforces Sovereign Enclave security model

  **Acceptance Criteria**:

  **If TDD:**
  - [ ] Test file created: `bridge/pkg/proxy/egress_test.go`
  - [ ] `go test ./pkg/proxy/...` → PASS (5+ tests)

  **QA Scenarios (MANDATORY):**

  ```
  Scenario: Proxy denies all requests by default
    Tool: Bash (curl)
    Preconditions: Egress proxy running on localhost:3128
    Steps:
      1. curl --proxy http://localhost:3128 https://evil.com/test 2>&1
      2. curl --proxy http://localhost:3128 https://random-site.org 2>&1
    Expected Result: Both return 403 Forbidden
    Failure Indicators: HTTP 200 or any successful response
    Evidence: .sisyphus/evidence/task-2-deny-all-default.log

  Scenario: Denied requests logged to audit system
    Tool: Bash (sqlite3)
    Preconditions: Egress proxy denied at least one request
    Steps:
      1. sqlite3 /var/lib/armorclaw/audit.db "SELECT * FROM events WHERE type='egress_denied' LIMIT 1"
    Expected Result: At least one audit entry with type='egress_denied'
    Failure Indicators: No rows returned
    Evidence: .sisyphus/evidence/task-2-audit-logging.log

  Scenario: Proxy returns 403 with clear error message
    Tool: Bash (curl)
    Preconditions: Egress proxy running
    Steps:
      1. curl -i --proxy http://localhost:3128 https://evil.com 2>&1 | head -5
    Expected Result: HTTP/1.1 403 Forbidden, body contains "Egress Denied"
    Failure Indicators: Different status code or missing error message
    Evidence: .sisyphus/evidence/task-2-403-response.log
  ```

  **Evidence to Capture:**
  - [ ] Denied request output: `task-2-deny-all-default.log`
  - [ ] Audit log entry: `task-2-audit-logging.log`
  - [ ] 403 response headers: `task-2-403-response.log`

  **Commit**: YES (groups with Tasks 3-4)
  - Message: `feat(proxy): add strict egress proxy with DENY ALL policy and LLM allowlist`
  - Files: `bridge/pkg/proxy/egress.go`, `bridge/pkg/proxy/egress_test.go`
  - Pre-commit: `go test ./pkg/proxy/...`

---

- [ ] **3. Egress Allowlist (LLM APIs)** [quick]

  **What to do**:
  - Create Go package `bridge/pkg/proxy/allowlist.go`
  - Implement exact SNI/host matching (no wildcards)
  - Add approved LLM API endpoints: `api.openai.com`, `api.anthropic.com`
  - Load allowlist from configuration file
  - Verify exact match logic (subdomain.api.openai.com denied)

  **Must NOT do**:
  - Do NOT use wildcard matching (*.openai.com forbidden)
  - Do NOT allow subdomains without explicit allowlist entry
  - Do NOT allow HTTP (HTTPS only for LLM APIs)

  **Recommended Agent Profile**:
  - **Category**: `quick`
    - Reason: Straightforward string matching logic, clear security requirements
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 1 (with Tasks 1, 2, 4)
  - **Blocks**: Task 10 (Integration tests need working allowlist)
  - **Blocked By**: Task 2 (Needs core proxy to integrate with)

  **References**:

  **Allowlist Configuration**:
  - Create `bridge/config/egress-allowlist.toml`:
    ```toml
    [allowlist]
    hosts = [
      "api.openai.com",
      "api.anthropic.com",
    ]
    require_https = true
    ```

  **Exact Match Implementation**:
  ```go
  type Allowlist struct {
      hosts       map[string]bool
      requireHTTPS bool
  }

  func (a *Allowlist) IsAllowed(host string) bool {
      // Exact match only - no wildcards
      return a.hosts[host]
  }

  func (a *Allowlist) IsHTTPSRequired() bool {
      return a.requireHTTPS
  }
  ```

  **Configuration Loading**:
  - `bridge/config/config.go:80-120` - TOML config loading pattern
  - Load on startup, fail if allowlist file missing

  **Why This Matters**:
  - Exact matching prevents DNS rebinding attacks
  - HTTPS requirement prevents MITM on LLM API traffic
  - Configurable allowlist allows easy updates without code changes

  **Acceptance Criteria**:

  **If TDD:**
  - [ ] Test file created: `bridge/pkg/proxy/allowlist_test.go`
  - [ ] `go test ./pkg/proxy/... -run TestAllowlist` → PASS (6+ tests)

  **QA Scenarios (MANDATORY):**

  ```
  Scenario: Allowlist permits exact match for api.openai.com
    Tool: Bash (curl)
    Preconditions: Egress proxy with allowlist configured
    Steps:
      1. curl --proxy http://localhost:3128 https://api.openai.com/v1/models 2>&1 | head -1
    Expected Result: HTTP 200 or 401 (auth required), NOT 403
    Failure Indicators: 403 Forbidden
    Evidence: .sisyphus/evidence/task-3-allow-openai.log

  Scenario: Allowlist denies subdomain not in allowlist
    Tool: Bash (curl)
    Preconditions: Egress proxy with allowlist configured
    Steps:
      1. curl --proxy http://localhost:3128 https://subdomain.api.openai.com/test 2>&1
    Expected Result: 403 Forbidden (exact match only)
    Failure Indicators: Any response other than 403
    Evidence: .sisyphus/evidence/task-3-deny-subdomain.log

  Scenario: Allowlist denies non-allowlisted LLM provider
    Tool: Bash (curl)
    Preconditions: Egress proxy with allowlist configured
    Steps:
      1. curl --proxy http://localhost:3128 https://api.replicate.com/v1/models 2>&1
    Expected Result: 403 Forbidden (not in allowlist)
    Failure Indicators: Any response other than 403
    Evidence: .sisyphus/evidence/task-3-deny-other-provider.log

  Scenario: HTTPS requirement enforced
    Tool: Bash (curl)
    Preconditions: Egress proxy with require_https=true
    Steps:
      1. curl --proxy http://localhost:3128 http://api.openai.com/test 2>&1
    Expected Result: 403 Forbidden (HTTP not allowed)
    Failure Indicators: Connection attempt to HTTP endpoint
    Evidence: .sisyphus/evidence/task-3-https-required.log
  ```

  **Evidence to Capture:**
  - [ ] Allowed OpenAI request: `task-3-allow-openai.log`
  - [ ] Denied subdomain: `task-3-deny-subdomain.log`
  - [ ] Denied other provider: `task-3-deny-other-provider.log`
  - [ ] HTTPS enforcement: `task-3-https-required.log`

  **Commit**: YES (groups with Tasks 2, 4)
  - Message: `feat(proxy): add strict egress proxy with DENY ALL policy and LLM allowlist`
  - Files: `bridge/pkg/proxy/allowlist.go`, `bridge/pkg/proxy/allowlist_test.go`, `bridge/config/egress-allowlist.toml`
  - Pre-commit: `go test ./pkg/proxy/...`

---

- [ ] **4. Audit Logging for Dropped Requests** [quick]

  **What to do**:
  - Create audit event type `EventEgressDenied` in `bridge/pkg/audit/`
  - Log ALL dropped egress requests with full context (host, path, timestamp, user_id)
  - Add query interface for security team to review exfiltration attempts
  - Include source container (OpenClaw) in audit trail

  **Must NOT do**:
  - Do NOT skip logging for any denied request
  - Do NOT log request body (PII risk)
  - Do NOT log query parameters (may contain secrets)

  **Recommended Agent Profile**:
  - **Category**: `quick`
    - Reason: Extending existing audit system with new event type
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 1 (with Tasks 1, 2, 3)
  - **Blocks**: Task 10 (Integration tests verify audit logs)
  - **Blocked By**: Task 2 (Needs core proxy to generate events)

  **References**:

  **Audit Event Types**:
  - `bridge/pkg/audit/audit.go:30-50` - Existing event type definitions
  - Add new event type:
    ```go
    const (
        EventEgressDenied EventType = "egress_denied"
    )
    ```

  **Audit Entry Structure**:
  ```go
  type EgressDeniedEvent struct {
      Timestamp   time.Time `json:"timestamp"`
      EventType   string    `json:"event_type"`
      UserID      string    `json:"user_id"`
      ContainerID string    `json:"container_id"`
      Host        string    `json:"host"`
      Path        string    `json:"path"`
      Method      string    `json:"method"`
      Reason      string    `json:"reason"`
  }
  ```

  **Query Interface**:
  - Add method to audit client: `GetEgressDeniedEvents(since time.Time) ([]EgressDeniedEvent, error)`
  - Used by security team to detect exfiltration attempts

  **Why This Matters**:
  - Dropped requests are PRIMARY INDICATORS of prompt injection attempts
  - LLM trying to exfiltrate data will trigger many denied egress events
  - Security team needs visibility into these attempts

  **Acceptance Criteria**:

  **If TDD:**
  - [ ] Test file created: `bridge/pkg/audit/egress_test.go`
  - [ ] `go test ./pkg/audit/... -run TestEgressDenied` → PASS (3+ tests)

  **QA Scenarios (MANDATORY):**

  ```
  Scenario: Denied egress logged to audit database
    Tool: Bash (sqlite3)
    Preconditions: At least one egress request denied
    Steps:
      1. sqlite3 /var/lib/armorclaw/audit.db "SELECT COUNT(*) FROM events WHERE type='egress_denied'"
    Expected Result: Count >= 1
    Failure Indicators: Count = 0
    Evidence: .sisyphus/evidence/task-4-audit-entry-exists.log

  Scenario: Audit entry contains required fields
    Tool: Bash (sqlite3)
    Preconditions: At least one egress denied event
    Steps:
      1. sqlite3 /var/lib/armorclaw/audit.db "SELECT timestamp, event_type, user_id, container_id, host FROM events WHERE type='egress_denied' LIMIT 1"
    Expected Result: All fields populated, valid timestamp
    Failure Indicators: NULL values in required fields
    Evidence: .sisyphus/evidence/task-4-audit-fields.log

  Scenario: Query interface returns denied events
    Tool: Bash (Go test)
    Preconditions: Multiple egress denied events in database
    Steps:
      1. go test ./pkg/audit/... -run TestGetEgressDeniedEvents -v
    Expected Result: Test passes, returns list of events
    Failure Indicators: Test fails or returns empty list
    Evidence: .sisyphus/evidence/task-4-query-interface.log
  ```

  **Evidence to Capture:**
  - [ ] Audit entry exists: `task-4-audit-entry-exists.log`
  - [ ] Required fields populated: `task-4-audit-fields.log`
  - [ ] Query interface works: `task-4-query-interface.log`

  **Commit**: YES (groups with Tasks 2, 3)
  - Message: `feat(proxy): add strict egress proxy with DENY ALL policy and LLM allowlist`
  - Files: `bridge/pkg/audit/egress.go`, `bridge/pkg/audit/egress_test.go`
  - Pre-commit: `go test ./pkg/audit/...`

---

### Wave 2: Core Translation (After Wave 1)

- [ ] **6. Protocol Translator (RPC → MCP)** [deep]

  **What to do**:
  - Create Go package `bridge/pkg/translator/rpc_to_mcp.go`
  - Translate OpenClaw's native JSON-RPC to Anthropic MCP protocol
  - Map 47 existing JSON-RPC methods to MCP `tools/call` format
  - Preserve request context (session_id, user_id, tool_name)
  - Handle streaming responses for long-running tools

  **Must NOT do**:
  - Do NOT modify OpenClaw source code
  - Do NOT break existing JSON-RPC API (backward compatibility required)
  - Do NOT skip protocol version validation

  **Recommended Agent Profile**:
  - **Category**: `deep`
    - Reason: Complex protocol translation, needs deep understanding of both RPC and MCP formats
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 2 (with Tasks 7, 9)
  - **Blocks**: Task 8 (MCP Router needs translator)
  - **Blocked By**: Task 5 (Rust Vault), Task 1 (Network isolation)

  **References**:

  **OpenClaw Native JSON-RPC Format**:
  - `bridge/pkg/rpc/server.go:817-880` - Existing 47 JSON-RPC methods
  - Example request:
    ```json
    {
      "jsonrpc": "2.0",
      "method": "browser.navigate",
      "params": {"url": "https://example.com", "agent_id": "agent-123"},
      "id": 1
    }
    ```

  **MCP Protocol Format**:
  - Official spec: https://modelcontextprotocol.io/specification/2025-11-25/
  - MCP `tools/call` request:
    ```json
    {
      "jsonrpc": "2.0",
      "method": "tools/call",
      "params": {
        "name": "browser_navigate",
        "arguments": {"url": "https://example.com", "agent_id": "agent-123"}
      },
      "id": 1
    }
    ```

  **Translation Logic**:
  ```go
  type RPCToMCPTranslator struct {
      methodMap map[string]string // JSON-RPC method -> MCP tool name
  }

  func (t *RPCToMCPTranslator) Translate(rpcReq *JSONRPCRequest) (*MCPRequest, error) {
      // Map method name (browser.navigate -> browser_navigate)
      toolName := t.methodMap[rpcReq.Method]
      
      return &MCPRequest{
          JSONRPC: "2.0",
          Method:  "tools/call",
          Params: MCPParams{
              Name:      toolName,
              Arguments: rpcReq.Params,
          },
          ID: rpcReq.ID,
      }, nil
  }
  ```

  **Method Mapping Table** (47 methods):
  ```go
  var methodMap = map[string]string{
      "browser.navigate":        "browser_navigate",
      "browser.fill":            "browser_fill",
      "browser.click":           "browser_click",
      "pii.request":             "pii_request",
      "skills.execute":          "skills_execute",
      // ... 42 more mappings
  }
  ```

  **Why This Matters**:
  - Enables OpenClaw to use ToolSidecar without MCP client support
  - Maintains backward compatibility with existing JSON-RPC API
  - Central translation point for all tool calls

  **Acceptance Criteria**:

  **If TDD:**
  - [ ] Test file created: `bridge/pkg/translator/rpc_to_mcp_test.go`
  - [ ] `go test ./pkg/translator/... -run TestTranslation` → PASS (10+ tests)

  **QA Scenarios (MANDATORY):**

  ```
  Scenario: Translate browser.navigate to MCP tools/call
    Tool: Bash (Go test)
    Preconditions: Translator package built
    Steps:
      1. go test ./pkg/translator/... -run TestBrowserNavigate -v
      2. Verify output contains MCP format with method="tools/call"
    Expected Result: Test passes, correct MCP structure
    Failure Indicators: Test fails or wrong method name
    Evidence: .sisyphus/evidence/task-6-translate-browser.log

  Scenario: Preserve request context across translation
    Tool: Bash (Go test)
    Preconditions: Translator with session context
    Steps:
      1. go test ./pkg/translator/... -run TestContextPreservation -v
      2. Verify session_id, user_id preserved in MCP params
    Expected Result: All context fields present in MCP request
    Failure Indicators: Missing or corrupted context fields
    Evidence: .sisyphus/evidence/task-6-context-preservation.log

  Scenario: Translate all 47 JSON-RPC methods
    Tool: Bash (Go test)
    Preconditions: Full method map implemented
    Steps:
      1. go test ./pkg/translator/... -run TestAllMethods -v
      2. Verify 47/47 methods translate successfully
    Expected Result: 47 translations, 0 errors
    Failure Indicators: Any method missing or translation error
    Evidence: .sisyphus/evidence/task-6-all-methods.log

  Scenario: Handle unknown method gracefully
    Tool: Bash (Go test)
    Preconditions: Translator with method validation
    Steps:
      1. go test ./pkg/translator/... -run TestUnknownMethod -v
      2. Send request with method="unknown.method"
    Expected Result: Error returned, no crash
    Failure Indicators: Panic or silent failure
    Evidence: .sisyphus/evidence/task-6-unknown-method.log
  ```

  **Evidence to Capture:**
  - [ ] Browser navigate translation: `task-6-translate-browser.log`
  - [ ] Context preservation: `task-6-context-preservation.log`
  - [ ] All 47 methods: `task-6-all-methods.log`
  - [ ] Unknown method handling: `task-6-unknown-method.log`

  **Commit**: YES (groups with Task 7)
  - Message: `feat(translator): implement RPC to MCP protocol translation with session lifecycle`
  - Files: `bridge/pkg/translator/rpc_to_mcp.go`, `bridge/pkg/translator/rpc_to_mcp_test.go`, `bridge/pkg/translator/method_map.go`
  - Pre-commit: `go test ./pkg/translator/...`

---

- [ ] **7. Session Lifecycle Manager** [deep]

  **What to do**:
  - Create Go package `bridge/pkg/translator/session_manager.go`
  - Track active OpenClaw sessions and associated ToolSidecar containers
  - Implement orphan detection (OpenClaw process died, ToolSidecar still running)
  - Graceful teardown: kill orphaned ToolSidecars within 60 seconds
  - Persist session state to survive Go Bridge restarts

  **Must NOT do**:
  - Do NOT allow orphaned ToolSidecar containers to persist >60 seconds
  - Do NOT lose session state on Go Bridge restart
  - Do NOT kill ToolSidecar during active execution

  **Recommended Agent Profile**:
  - **Category**: `deep`
    - Reason: Complex lifecycle management, race conditions, state persistence
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 2 (with Tasks 6, 9)
  - **Blocks**: Task 8 (MCP Router needs session context), Task 11 (Crash recovery test)
  - **Blocked By**: Task 6 (Needs translator to know session structure)

  **References**:

  **Session Tracking Structure**:
  ```go
  type Session struct {
      ID              string
      OpenClawPID     int
      ToolSidecarIDs  []string
      CreatedAt       time.Time
      LastActivityAt  time.Time
      Status          SessionStatus // active, orphaned, terminated
  }

  type SessionManager struct {
      sessions    map[string]*Session
      orphanCheck *time.Ticker
      db          *sql.DB
  }
  ```

  **Orphan Detection Logic**:
  ```go
  func (sm *SessionManager) checkOrphans() {
      for _, session := range sm.sessions {
          // Check if OpenClaw process still exists
          if !sm.processExists(session.OpenClawPID) {
              session.Status = Orphaned
              sm.cleanupOrphan(session)
          }
      }
  }

  func (sm *SessionManager) cleanupOrphan(session *Session) {
      // Kill all associated ToolSidecar containers
      for _, containerID := range session.ToolSidecarIDs {
          docker.KillContainer(containerID)
      }
      delete(sm.sessions, session.ID)
  }
  ```

  **State Persistence**:
  - Store sessions in SQLite: `bridge/data/sessions.db`
  - Load on startup to recover from crashes
  - Use WAL mode for concurrent access

  **Why This Matters**:
  - OpenClaw crashes shouldn't leave zombie ToolSidecar containers
  - Prevents resource leaks from crashed sessions
  - Critical for production reliability

  **Acceptance Criteria**:

  **If TDD:**
  - [ ] Test file created: `bridge/pkg/translator/session_manager_test.go`
  - [ ] `go test ./pkg/translator/... -run TestSession` → PASS (8+ tests)

  **QA Scenarios (MANDATORY):**

  ```
  Scenario: Detect orphaned session within 60 seconds
    Tool: Bash (docker + sleep)
    Preconditions: Active session with ToolSidecar running
    Steps:
      1. docker kill openclaw (simulate crash)
      2. sleep 65
      3. docker ps | grep toolsidecar
    Expected Result: No toolsidecar containers running
    Failure Indicators: ToolSidecar container still exists
    Evidence: .sisyphus/evidence/task-7-orphan-detection.log

  Scenario: Session state persists across Go Bridge restart
    Tool: Bash (docker restart)
    Preconditions: Active session in database
    Steps:
      1. docker restart go-bridge
      2. sleep 5
      3. curl http://localhost:8080/sessions | jq '.sessions | length'
    Expected Result: Session count matches pre-restart
    Failure Indicators: Session count = 0
    Evidence: .sisyphus/evidence/task-7-state-persistence.log

  Scenario: Do not kill ToolSidecar during active execution
    Tool: Bash (Go test)
    Preconditions: ToolSidecar executing long-running tool
    Steps:
      1. Start tool with 30s timeout
      2. Trigger orphan check during execution
      3. Verify ToolSidecar not killed
    Expected Result: ToolSidecar completes execution
    Failure Indicators: ToolSidecar killed mid-execution
    Evidence: .sisyphus/evidence/task-7-active-execution.log

  Scenario: Handle concurrent session cleanup without race
    Tool: Bash (Go test -race)
    Preconditions: Multiple orphaned sessions
    Steps:
      1. go test -race ./pkg/translator/... -run TestConcurrentCleanup
    Expected Result: No race conditions detected
    Failure Indicators: DATA RACE warnings
    Evidence: .sisyphus/evidence/task-7-race-detection.log
  ```

  **Evidence to Capture:**
  - [ ] Orphan detection log: `task-7-orphan-detection.log`
  - [ ] State persistence: `task-7-state-persistence.log`
  - [ ] Active execution protection: `task-7-active-execution.log`
  - [ ] Race detection: `task-7-race-detection.log`

  **Commit**: YES (groups with Task 6)
  - Message: `feat(translator): implement RPC to MCP protocol translation with session lifecycle`
  - Files: `bridge/pkg/translator/session_manager.go`, `bridge/pkg/translator/session_manager_test.go`, `bridge/data/sessions.db`
  - Pre-commit: `go test -race ./pkg/translator/...`

---

- [ ] **9. ToolSidecar Provisioner** [unspecified-high]

  **What to do**:
  - Create Go package `bridge/pkg/toolsidecar/provisioner.go`
  - Implement on-demand container spawning for MCP tool execution
  - Use Docker SDK for Go to manage container lifecycle
  - Mount tmpfs for secrets (no persistent storage)
  - Set resource limits: 256MB RAM, 30s timeout, read-only filesystem
  - Connect to isolated network, expose only to Go Bridge via UDS

  **Must NOT do**:
  - Do NOT give ToolSidecar persistent storage
  - Do NOT allow ToolSidecar network egress (isolated network)
  - Do NOT spawn containers without resource limits

  **Recommended Agent Profile**:
  - **Category**: `unspecified-high`
    - Reason: Container orchestration, security-critical isolation, complex Docker API usage
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 2 (with Tasks 6, 7)
  - **Blocks**: Task 8 (MCP Router needs provisioner), Task 10 (Integration tests)
  - **Blocked By**: None (can start after Task 5)

  **References**:

  **ToolSidecar Container Spec**:
  ```yaml
  # Generated dynamically by provisioner
  image: armorclaw/toolsidecar:latest
  network_mode: "none"  # Isolated
  read_only: true
  tmpfs:
    - /tmp/secrets:size=1M,mode=0600
  cap_drop: [ALL]
  security_opt: [no-new-privileges:true]
  mem_limit: 256m
  pids_limit: 100
  timeout: 30s
  ```

  **Docker SDK Usage**:
  ```go
  import "github.com/docker/docker/client"

  type Provisioner struct {
      dockerClient *client.Client
      skillPackages map[string]string // skill_name -> package_path
  }

  func (p *Provisioner) SpawnToolSidecar(skillName string, sessionID string) (*ToolSidecar, error) {
      ctx := context.Background()
      
      // Create container with security constraints
      resp, err := p.dockerClient.ContainerCreate(ctx, &container.Config{
          Image: "armorclaw/toolsidecar:latest",
          Env: []string{
              fmt.Sprintf("SKILL_NAME=%s", skillName),
              fmt.Sprintf("SESSION_ID=%s", sessionID),
          },
      }, &container.HostConfig{
          ReadOnlyRootfs: true,
          Tmpfs: map[string]string{"/tmp/secrets": "size=1M,mode=0600"},
          CapDrop: []string{"ALL"},
          SecurityOpt: []string{"no-new-privileges:true"},
          Resources: container.Resources{
              Memory: 256 * 1024 * 1024, // 256MB
              PidsLimit: 100,
          },
      }, nil, nil, fmt.Sprintf("toolsidecar-%s", sessionID))
      
      return &ToolSidecar{ID: resp.ID}, nil
  }
  ```

  **Skill Package Loading**:
  - Skills stored in `bridge/skills/{skill_name}/`
  - Mounted as read-only volume into ToolSidecar
  - Validated before mounting (checksum verification)

  **Why This Matters**:
  - ToolSidecar isolation prevents compromised tools from affecting host
  - Resource limits prevent DoS from runaway tools
  - No persistent storage prevents data exfiltration via filesystem

  **Acceptance Criteria**:

  **If TDD:**
  - [ ] Test file created: `bridge/pkg/toolsidecar/provisioner_test.go`
  - [ ] `go test ./pkg/toolsidecar/...` → PASS (6+ tests)

  **QA Scenarios (MANDATORY):**

  ```
  Scenario: Spawn ToolSidecar container on demand
    Tool: Bash (docker)
    Preconditions: Provisioner running, skill package available
    Steps:
      1. curl -X POST http://localhost:8080/toolsidecar/spawn -d '{"skill":"agentmail"}'
      2. docker ps | grep toolsidecar
    Expected Result: ToolSidecar container running
    Failure Indicators: No container spawned
    Evidence: .sisyphus/evidence/task-9-spawn-container.log

  Scenario: ToolSidecar has no persistent storage
    Tool: Bash (docker inspect)
    Preconditions: ToolSidecar container running
    Steps:
      1. docker inspect toolsidecar-xxx | jq '.[0].HostConfig.ReadOnlyRootfs'
      2. docker inspect toolsidecar-xxx | jq '.[0].HostConfig.Binds'
    Expected Result: ReadOnlyRootfs=true, Binds=[]
    Failure Indicators: Writable volumes mounted
    Evidence: .sisyphus/evidence/task-9-no-persistent-storage.log

  Scenario: ToolSidecar respects resource limits
    Tool: Bash (docker stats)
    Preconditions: ToolSidecar running with load
    Steps:
      1. docker stats --no-stream toolsidecar-xxx
    Expected Result: Memory usage < 256MB
    Failure Indicators: Memory exceeds limit or OOM killed
    Evidence: .sisyphus/evidence/task-9-resource-limits.log

  Scenario: ToolSidecar terminates after timeout
    Tool: Bash (docker + sleep)
    Preconditions: ToolSidecar spawned with 30s timeout
    Steps:
      1. sleep 35
      2. docker ps -a | grep toolsidecar-xxx
    Expected Result: Container status = "exited"
    Failure Indicators: Container still running
    Evidence: .sisyphus/evidence/task-9-timeout-termination.log
  ```

  **Evidence to Capture:**
  - [ ] Container spawned: `task-9-spawn-container.log`
  - [ ] No persistent storage: `task-9-no-persistent-storage.log`
  - [ ] Resource limits: `task-9-resource-limits.log`
  - [ ] Timeout termination: `task-9-timeout-termination.log`

  **Commit**: YES (groups with Tasks 8)
  - Message: `feat(mcp): add MCP router with SkillGate integration and ToolSidecar provisioning`
  - Files: `bridge/pkg/toolsidecar/provisioner.go`, `bridge/pkg/toolsidecar/provisioner_test.go`, `bridge/skills/`
  - Pre-commit: `go test ./pkg/toolsidecar/...`

---

### Wave 3: Integration (After Waves 1 & 2)

- [x] **8. MCP Router with SkillGate** [deep] - ⚠️ **HAS issues** - needs fixes

  **What to do**:
  - Create Go package `bridge/pkg/mcp/router.go`
  - Route MCP `tools/call` requests through Rust Vault SkillGate
  - Integrate with existing `bridge/pkg/governor/skillgate.go`
  - Add consent workflow for PII operations
  - Log all tool calls to audit system

  **Must NOT do**:
  - Do NOT bypass SkillGate for any tool call
  - Do NOT skip consent for PII operations
  - Do NOT log PII values in audit trail (log placeholders only)

  **Recommended Agent Profile**:
  - **Category**: `deep`
    - Reason: Security-critical routing, PII handling, consent workflow integration
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 3 (with Tasks 10, 11)
  - **Blocks**: Task 10 (Integration tests need router)
  - **Blocked By**: Task 5 (Rust Vault), Task 6 (Translator), Task 4 (Audit logging)

  **References**:

  **SkillGate Integration**:
  - `bridge/pkg/governor/skillgate.go:44-95` - Existing InterceptToolCall implementation
  - Use existing Governor to validate all MCP tool calls

  **MCP Router Structure**:
  ```go
  type MCPRouter struct {
      skillGate   *governor.Governor
      provisioner *toolsidecar.Provisioner
      auditor     *audit.Logger
  }

  func (r *MCPRouter) HandleToolsCall(ctx context.Context, req *MCPRequest) (*MCPResponse, error) {
      // 1. Validate via SkillGate
      toolCall := &interfaces.ToolCall{
          ToolName:  req.Params.Name,
          Arguments: req.Params.Arguments,
      }
      
      validated, err := r.skillGate.InterceptToolCall(ctx, toolCall)
      if err != nil {
          return nil, err
      }
      
      // 2. Check for consent requirement
      if requiresConsent(validated) {
          return r.initiateConsent(ctx, validated)
      }
      
      // 3. Spawn ToolSidecar
      sidecar, err := r.provisioner.SpawnToolSidecar(validated.ToolName, sessionID)
      if err != nil {
          return nil, err
      }
      
      // 4. Execute tool
      result, err := sidecar.Execute(ctx, validated.Arguments)
      
      // 5. Audit log
      r.auditor.Log(audit.Event{
          Type:   audit.EventToolExecution,
          Detail: fmt.Sprintf("tool=%s session=%s", validated.ToolName, sessionID),
      })
      
      return &MCPResponse{Result: result}, nil
  }
  ```

  **Consent Workflow**:
  - If SkillGate returns `REQUIRE_CONSENT`, send Matrix notification to user
  - Wait for approval (with timeout)
  - If approved, proceed with execution
  - If denied/timeout, return error to OpenClaw

  **Why This Matters**:
  - Central security checkpoint for all tool calls
  - Prevents unauthorized PII access
  - Full audit trail for compliance

  **Acceptance Criteria**:

  **If TDD:**
  - [ ] Test file created: `bridge/pkg/mcp/router_test.go`
  - [ ] `go test ./pkg/mcp/...` → PASS (7+ tests)

  **QA Scenarios (MANDATORY):**

  ```
  Scenario: All tool calls pass through SkillGate
    Tool: Bash (Go test + audit log)
    Preconditions: MCP router running
    Steps:
      1. Execute tool via MCP router
      2. sqlite3 audit.db "SELECT * FROM events WHERE type='tool_execution'"
    Expected Result: Audit entry exists with tool_name
    Failure Indicators: No audit entry (bypass detected)
    Evidence: .sisyphus/evidence/task-8-skillgate-integration.log

  Scenario: Consent required for PII tool
    Tool: Bash (curl + Matrix)
    Preconditions: Tool requires consent (e.g., pii.request)
    Steps:
      1. curl -X POST http://localhost:8080/mcp/tools/call -d '{"name":"pii_request"}'
      2. Check Matrix for consent notification
    Expected Result: Matrix message sent, tool not executed yet
    Failure Indicators: Tool executes without consent
    Evidence: .sisyphus/evidence/task-8-consent-workflow.log

  Scenario: SkillGate blocks malicious tool call
    Tool: Bash (curl)
    Preconditions: Tool call with PII in arguments
    Steps:
      1. curl -X POST http://localhost:8080/mcp/tools/call -d '{"name":"email_send","arguments":{"body":"SSN: 123-45-6789"}}'
    Expected Result: 403 Forbidden, PII detected
    Failure Indicators: Tool executes with raw PII
    Evidence: .sisyphus/evidence/task-8-skillgate-block.log

  Scenario: PII redacted in audit logs
    Tool: Bash (sqlite3)
    Preconditions: Tool call with PII executed
    Steps:
      1. sqlite3 audit.db "SELECT detail FROM events WHERE type='tool_execution'"
    Expected Result: PII replaced with [REDACTED:hash]
    Failure Indicators: Raw PII visible in audit log
    Evidence: .sisyphus/evidence/task-8-pii-redacted.log
  ```

  **Evidence to Capture:**
  - [ ] SkillGate integration: `task-8-skillgate-integration.log`
  - [ ] Consent workflow: `task-8-consent-workflow.log`
  - [ ] SkillGate blocking: `task-8-skillgate-block.log`
  - [ ] PII redaction: `task-8-pii-redacted.log`

  **Commit**: YES (groups with Task 9)
  - Message: `feat(mcp): add MCP router with SkillGate integration and ToolSidecar provisioning`
  - Files: `bridge/pkg/mcp/router.go`, `bridge/pkg/mcp/router_test.go`
  - Pre-commit: `go test ./pkg/mcp/...`

---

- [ ] **10. End-to-End Integration Test** [deep]

  **What to do**:
  - Create comprehensive test: OpenClaw → Go Bridge → Rust Vault → ToolSidecar
  - Test browser.navigate tool end-to-end
  - Verify PII redaction at each layer
  - Verify audit trail completeness
  - Test with real OpenClaw instance (not mock)

  **Must NOT do**:
  - Do NOT use mocks for integration test (must test real components)
  - Do NOT skip audit log verification
  - Do NOT test happy path only (include error scenarios)

  **Recommended Agent Profile**:
  - **Category**: `deep`
    - Reason: Complex multi-component integration, real system testing
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 3 (with Tasks 8, 11)
  - **Blocks**: Final Verification Wave
  - **Blocked By**: All previous tasks (1-9 must be complete)

  **References**:

  **Test Structure**:
  ```go
  func TestEndToEndBrowserNavigate(t *testing.T) {
      // 1. Start OpenClaw on isolated network
      openclaw := startOpenClaw(t)
      defer openclaw.Stop()
      
      // 2. Send browser.navigate RPC
      resp, err := openclaw.Call("browser.navigate", map[string]interface{}{
          "url": "https://example.com",
      })
      require.NoError(t, err)
      
      // 3. Verify Go Bridge received request
      auditEntries := getAuditLog(t, "rpc_received")
      require.Len(t, auditEntries, 1)
      
      // 4. Verify protocol translation (RPC -> MCP)
      mcpEntries := getAuditLog(t, "mcp_call")
      require.Equal(t, "browser_navigate", mcpEntries[0].ToolName)
      
      // 5. Verify SkillGate validation
      skillGateEntries := getAuditLog(t, "skillgate_validation")
      require.Equal(t, "ALLOW", skillGateEntries[0].Decision)
      
      // 6. Verify ToolSidecar spawned
      containers := listToolSidecars(t)
      require.Len(t, containers, 1)
      
      // 7. Verify tool executed
      toolEntries := getAuditLog(t, "tool_execution")
      require.Equal(t, "success", toolEntries[0].Status)
  }
  ```

  **Why This Matters**:
  - Validates entire architecture works together
  - Catches integration issues before production
  - Provides confidence for deployment

  **Acceptance Criteria**:

  **QA Scenarios (MANDATORY):**

  ```
  Scenario: Full flow OpenClaw to ToolSidecar works
    Tool: Bash (integration test)
    Preconditions: All components deployed, clean state
    Steps:
      1. go test ./tests/integration/... -run TestEndToEnd -v
      2. Verify test passes with all assertions
    Expected Result: Test passes, all audit entries present
    Failure Indicators: Any assertion failure
    Evidence: .sisyphus/evidence/task-10-e2e-flow.log

  Scenario: PII redacted at every layer
    Tool: Bash (sqlite3 + grep)
    Preconditions: Tool call with PII executed
    Steps:
      1. sqlite3 audit.db "SELECT * FROM events" | grep "123-45-6789"
    Expected Result: No matches (PII redacted everywhere)
    Failure Indicators: Raw SSN found in any log
    Evidence: .sisyphus/evidence/task-10-pii-redaction-e2e.log

  Scenario: Error handling works end-to-end
    Tool: Bash (curl)
    Preconditions: System running, test malformed request
    Steps:
      1. curl -X POST http://localhost:8080/mcp/tools/call -d '{"name":"invalid_tool"}'
      2. Check error response and audit log
    Expected Result: Error returned, logged in audit
    Failure Indicators: Crash or unlogged error
    Evidence: .sisyphus/evidence/task-10-error-handling.log

  Scenario: Performance within latency target
    Tool: Bash (time)
    Preconditions: System under normal load
    Steps:
      1. time curl -X POST http://localhost:8080/mcp/tools/call -d '{"name":"browser_navigate"}'
    Expected Result: Total time < 100ms
    Failure Indicators: Latency exceeds 100ms
    Evidence: .sisyphus/evidence/task-10-performance.log
  ```

  **Evidence to Capture:**
  - [ ] E2E flow: `task-10-e2e-flow.log`
  - [ ] PII redaction: `task-10-pii-redaction-e2e.log`
  - [ ] Error handling: `task-10-error-handling.log`
  - [ ] Performance: `task-10-performance.log`

  **Commit**: YES (groups with Task 11)
  - Message: `test(integration): add end-to-end and crash recovery tests`
  - Files: `tests/integration/e2e_test.go`, `tests/integration/crash_test.go`
  - Pre-commit: `go test ./tests/integration/...`

---

- [ ] **11. OpenClaw Crash Recovery Test** [deep]

  **What to do**:
  - Test orphan cleanup when OpenClaw crashes
  - Verify ToolSidecar containers terminated within 60 seconds
  - Test Go Bridge restart (session state recovery)
  - Test concurrent crash scenarios (multiple sessions)

  **Must NOT do**:
  - Do NOT test single crash scenario only (test multiple)
  - Do NOT skip session state recovery test
  - Do NOT ignore race conditions

  **Recommended Agent Profile**:
  - **Category**: `deep`
    - Reason: Complex failure scenarios, timing-sensitive tests, race condition detection
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 3 (with Tasks 8, 10)
  - **Blocks**: Final Verification Wave
  - **Blocked By**: Task 7 (Session manager), Task 10 (E2E test)

  **References**:

  **Crash Simulation**:
  ```go
  func TestOpenClawCrashRecovery(t *testing.T) {
      // 1. Start OpenClaw with active sessions
      openclaw := startOpenClaw(t)
      sessionID := createSession(t, openclaw)
      
      // 2. Spawn ToolSidecar for session
      spawnToolSidecar(t, sessionID)
      
      // 3. Kill OpenClaw (simulate crash)
      openclaw.Kill()
      
      // 4. Wait for orphan detection (60s)
      time.Sleep(65 * time.Second)
      
      // 5. Verify ToolSidecar terminated
      containers := listToolSidecars(t)
      assert.Empty(t, containers, "ToolSidecar should be cleaned up")
  }
  ```

  **Why This Matters**:
  - Production systems crash - must handle gracefully
  - Orphaned containers = resource leaks = production issues
  - Critical for reliability

  **Acceptance Criteria**:

  **QA Scenarios (MANDATORY):**

  ```
  Scenario: ToolSidecar cleaned up after OpenClaw crash
    Tool: Bash (docker + kill)
    Preconditions: Active session with ToolSidecar running
    Steps:
      1. docker kill openclaw
      2. sleep 65
      3. docker ps -a | grep toolsidecar
    Expected Result: No toolsidecar containers
    Failure Indicators: Orphaned containers exist
    Evidence: .sisyphus/evidence/task-11-crash-cleanup.log

  Scenario: Session state recovered after Go Bridge restart
    Tool: Bash (docker restart)
    Preconditions: Active sessions in database
    Steps:
      1. docker restart go-bridge
      2. sleep 5
      3. curl http://localhost:8080/sessions | jq '.sessions | length'
    Expected Result: Session count matches pre-restart
    Failure Indicators: Sessions lost
    Evidence: .sisyphus/evidence/task-11-session-recovery.log

  Scenario: Multiple concurrent crashes handled correctly
    Tool: Bash (Go test -race)
    Preconditions: Multiple sessions active
    Steps:
      1. go test -race ./tests/integration/... -run TestConcurrentCrashes
    Expected Result: No race conditions, all orphans cleaned
    Failure Indicators: Race warnings or orphaned containers
    Evidence: .sisyphus/evidence/task-11-concurrent-crashes.log

  Scenario: Active execution protected during crash
    Tool: Bash (docker + curl)
    Preconditions: ToolSidecar executing long tool
    Steps:
      1. Start tool with 30s timeout
      2. Kill OpenClaw during execution
      3. Verify ToolSidecar not killed until execution complete
    Expected Result: Tool completes, then cleanup
    Failure Indicators: Tool killed mid-execution
    Evidence: .sisyphus/evidence/task-11-active-protection.log
  ```

  **Evidence to Capture:**
  - [ ] Crash cleanup: `task-11-crash-cleanup.log`
  - [ ] Session recovery: `task-11-session-recovery.log`
  - [ ] Concurrent crashes: `task-11-concurrent-crashes.log`
  - [ ] Active protection: `task-11-active-protection.log`

  **Commit**: YES (groups with Task 10)
  - Message: `test(integration): add end-to-end and crash recovery tests`
  - Files: `tests/integration/crash_test.go`
  - Pre-commit: `go test -race ./tests/integration/...`

---

## Final Verification Wave (MANDATORY)

> 4 review agents run in PARALLEL. ALL must APPROVE. Present consolidated results to user and get explicit "okay" before completing.

- [ ] F1. **Plan Compliance Audit** — `oracle`
  Read the plan end-to-end. For each "Must Have": verify implementation exists. For each "Must NOT Have": search codebase for forbidden patterns. Check evidence files exist. Compare deliverables against plan.
  Output: `Must Have [N/N] | Must NOT Have [N/N] | Tasks [N/N] | VERDICT: APPROVE/REJECT`

- [ ] F2. **Code Quality Review** — `unspecified-high`
  Run `go test ./...` + `cargo test --all`. Review all changed files for: `interface{}` abuse, empty catches, commented-out code, unused imports. Check AI slop: excessive comments, over-abstraction.
  Output: `Go Tests [N pass/N fail] | Rust Tests [N pass/N fail] | Files [N clean/N issues] | VERDICT`

- [ ] F3. **Security Audit** — `unspecified-high`
  Verify egress proxy DENY ALL policy (test exfiltration attempt). Check audit logs contain dropped requests. Verify OpenClaw network isolation (no external gateway). Test crash recovery (orphan cleanup).
  Output: `Egress Policy [PASS/FAIL] | Audit Logs [PASS/FAIL] | Network Isolation [PASS/FAIL] | Crash Recovery [PASS/FAIL] | VERDICT`

- [ ] F4. **Real Manual QA** — `unspecified-high`
  Start from clean state. Execute EVERY QA scenario from EVERY task. Test cross-task integration. Test edge cases: OpenClaw restart, exfiltration attempt, malformed RPC. Save evidence to `.sisyphus/evidence/final-qa/`.
  Output: `Scenarios [N/N pass] | Integration [N/N] | Edge Cases [N tested] | VERDICT`

---

## Commit Strategy

- **After Task 5**: `fix(sidecar): resolve 74 compilation errors with bundled-sqlcipher and protoc`
- **After Task 1**: `feat(network): add armorclaw-isolated Docker network for OpenClaw`
- **After Tasks 2-4**: `feat(proxy): add strict egress proxy with DENY ALL policy and LLM allowlist`
- **After Tasks 6-7**: `feat(translator): implement RPC to MCP protocol translation with session lifecycle`
- **After Tasks 8-9**: `feat(mcp): add MCP router with SkillGate integration and ToolSidecar provisioning`
- **After Tasks 10-11**: `test(integration): add end-to-end and crash recovery tests`

---

## Success Criteria

### Verification Commands

```bash
# Verify Rust Vault compiles
cd sidecar && cargo build --release
# Expected: Compiling sidecar v0.1.0 (...) Finished

# Verify network isolation
docker network inspect armorclaw-isolated | grep -A5 "IPAM"
# Expected: No external gateway, internal: true

# Verify egress proxy denies non-LLM traffic
curl --proxy http://localhost:3128 https://evil.com/test
# Expected: 403 Forbidden + audit log entry

# Verify protocol translation
go test ./pkg/translator/... -v
# Expected: PASS - all translation tests

# Verify crash recovery
docker restart openclaw && sleep 5 && docker ps | grep toolsidecar
# Expected: No orphaned toolsidecar containers
```

### Final Checklist

- [ ] All "Must Have" present
- [ ] All "Must NOT Have" absent
- [ ] All tests pass (Go + Rust)
- [ ] Rust Vault compiles with 0 errors
- [ ] Egress proxy drops non-LLM traffic
- [ ] Audit logs contain all dropped requests
- [ ] OpenClaw isolated on armorclaw-isolated network
- [ ] Protocol translation works for all 47 methods
- [ ] Orphaned ToolSidecars cleaned up within 60s
- [ ] End-to-end flow: OpenClaw → Bridge → Vault → ToolSidecar works
