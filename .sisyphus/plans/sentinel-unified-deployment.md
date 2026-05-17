# Sentinel Unified Automatic Deployment Plan

> **Objective**: Implement automatic "Sentinel" mode detection and configuration in the standard ArmorClaw installer, enabling zero-manual-config VPS deployment with one command.
>
> **Design Principle**: Sentinel is NOT a separate product—it is an automatic configuration profile triggered when the installer detects a VPS with a domain.

---

## TL;DR

> **Quick Summary**: Extend ArmorClaw installer to automatically detect VPS vs local deployment and configure TCP+TLS+Discovery for remote access (Sentinel mode) vs Unix sockets for local access (Native mode).
>
> **Deliverables**:
> - Bridge runtime TCP transport support
> - Unified docker-compose.yml with profiles
> - Automatic installer mode detection
> - Caddyfile auto-generation
> - Updated discovery endpoints
>
> **Estimated Effort**: Medium
> **Parallel Execution**: YES - 4 waves
> **Critical Path**: Phase 1 Config → Phase 2 Installer → Phase 4 Testing

---

## Gap Analysis

### Phase 1: Bridge Runtime Support

**Current State** (`bridge/pkg/rpc/server.go`):
```go
func (s *Server) Run(socketPath string) error {
    if runtime.GOOS == "windows" {
        listener, err = net.Listen("tcp", "127.0.0.1:6168")  // Windows fallback only
    } else {
        listener, err = net.Listen("unix", socketPath)       // Unix socket only
    }
}
```

**Current Config** (`bridge/pkg/config/config.go`):
- `ServerConfig.SocketPath` - Unix socket path only
- `ServerConfig.Auth` - Authentication mode
- NO `Mode`, `RPCTransport`, `ListenAddr`, `PublicBaseURL` fields

| Gap | Severity | Description |
|-----|----------|-------------|
| **G1.1** | **CRITICAL** | No TCP transport option for non-Windows systems |
| **G1.2** | **CRITICAL** | No mode switching (native vs sentinel) |
| **G1.3** | HIGH | No `PublicBaseURL` for discovery endpoint |
| **G1.4** | MEDIUM | No `AdminToken` field in config (installer generates but nowhere to store) |
| **G1.5** | MEDIUM | Hardcoded Windows fallback address (should be configurable) |

### Phase 2: Installer

**Current State** (`deploy/install.sh`):
- Bootstrap loader that downloads `installer-v5.sh`
- GPG signature verification
- No interactive mode detection
- No domain/email prompts

**Current State** (`deploy/installer-v5.sh`):
- Need to review actual implementation (not yet analyzed)

| Gap | Severity | Description |
|-----|----------|-------------|
| **G2.1** | **CRITICAL** | No domain detection/prompt logic |
| **G2.2** | **CRITICAL** | No mode determination (native vs sentinel) |
| **G2.3** | **CRITICAL** | No `.env` file generation |
| **G2.4** | **CRITICAL** | No Caddyfile auto-generation |
| **G2.5** | HIGH | No secret generation (ADMIN_TOKEN, KEYSTORE_SECRET, etc.) |
| **G2.6** | HIGH | No public IP detection |
| **G2.7** | MEDIUM | No Docker Compose profile selection |
| **G2.8** | MEDIUM | No user instructions generation |

### Phase 3: Docker Compose

**Current State** (`docker-compose.yml`):
- Includes sub-stacks via `include:`
- `armorclaw-sentinel` service with TCP binding (hardcoded)
- Environment variables not unified

| Gap | Severity | Description |
|-----|----------|-------------|
| **G3.1** | **CRITICAL** | No profile-based activation for sentinel mode |
| **G3.2** | HIGH | No unified `.env` variable usage |
| **G3.3** | HIGH | Bridge runs in container (installer plan says native binary on host) |
| **G3.4** | MEDIUM | No conditional port exposure based on mode |
| **G3.5** | MEDIUM | No Caddy service in compose file |

### Phase 4: Discovery

**Current State** (`bridge/pkg/discovery/http.go`):
- `/api/discovery` endpoint exists
- Returns `api_url`, `ws_url`, `matrix_homeserver`, `push_gateway`
- Uses `info.Host` for URL generation

| Gap | Severity | Description |
|-----|----------|-------------|
| **G4.1** | HIGH | `info.Host` not populated from `PublicBaseURL` |
| **G4.2** | MEDIUM | No `provisioning_available` field |
| **G4.3** | MEDIUM | No `server_name` field |

### Phase 5: ArmorChat Compatibility

| Gap | Severity | Description |
|-----|----------|-------------|
| **G5.1** | LOW | No changes needed - app already supports domain entry |
| **G5.2** | MEDIUM | First-run admin token prompt UX needs verification |

### Cross-Cutting Gaps

| Gap | Severity | Description |
|-----|----------|-------------|
| **GX.1** | **CRITICAL** | Migration path: existing native → sentinel upgrade? |
| **GX.2** | HIGH | Security: Admin token stored in `.env` file (world-readable?) |
| **GX.3** | HIGH | TLS certificate provisioning: Let's Encrypt rate limits |
| **GX.4** | MEDIUM | Idempotency: re-running installer should not break config |
| **GX.5** | MEDIUM | Rollback: How to undo sentinel mode? |
| **GX.6** | MEDIUM | Offline install: Can't download Caddy if air-gapped |
| **GX.7** | LOW | DNS propagation: Wait for DNS before cert issuance |

---

## Work Objectives

### Core Objective

Implement automatic Sentinel mode in ArmorClaw installer with:
1. Bridge TCP transport support
2. Installer mode detection and configuration
3. Unified Docker Compose with profiles
4. Updated discovery endpoints

### Concrete Deliverables

- `bridge/pkg/config/config.go` - Extended ServerConfig with mode fields
- `bridge/pkg/rpc/server.go` - TCP transport support
- `bridge/cmd/bridge/main.go` - Mode-aware listener selection
- `deploy/installer-v6.sh` - Unified installer with mode detection
- `docker-compose.yml` - Profile-based deployment
- `configs/Caddyfile.template` - Caddyfile template
- `bridge/pkg/discovery/http.go` - PublicBaseURL support

### Definition of Done

- [ ] `curl -fsSL https://.../install.sh | bash` detects VPS and configures Sentinel
- [ ] Same command on local machine configures Native mode
- [ ] ArmorChat can connect via `https://domain.com` after installer completes
- [ ] TLS certificates automatically provisioned via Let's Encrypt
- [ ] Admin token displayed once and stored securely

### Must Have

- TCP transport for RPC server
- Mode field in ServerConfig
- Installer domain detection
- `.env` file generation
- Caddyfile auto-generation
- Profile-based Docker Compose

### Must NOT Have (Guardrails)

- NO breaking changes to existing Native mode
- NO requirement for manual config file editing
- NO exposed admin tokens in logs
- NO separate "sentinel" binary or package

---

## Verification Strategy

### Test Decision

- **Infrastructure exists**: YES (E2E tests in `/tests/e2e/`)
- **Automated tests**: YES - TDD with integration tests
- **Framework**: `bun test` for installer, Go tests for bridge
- **Agent-Executed QA**: YES

### QA Policy

Every task includes agent-executed QA scenarios:
- **CLI/Installer**: Bash execution with output validation
- **Bridge**: Go tests + RPC verification
- **Docker**: Container startup and health checks

---

## Execution Strategy

### Parallel Execution Waves

```
Wave 1 (Foundation - Core Config Changes):
├── Task 1: Extend ServerConfig with Sentinel fields [quick]
├── Task 2: Add TCP transport to RPC server [quick]
└── Task 3: Update main.go listener selection [quick]

Wave 2 (Discovery & Integration):
├── Task 4: Update discovery endpoint for PublicBaseURL [quick]
├── Task 5: Create config loading from env vars [quick]
└── Task 6: Add validation for sentinel mode [quick]

Wave 3 (Installer - The Core Change):
├── Task 7: Implement domain detection logic [unspecified-high]
├── Task 8: Add secret generation [quick]
├── Task 9: Implement .env file generation [quick]
├── Task 10: Create Caddyfile template [quick]
└── Task 11: Implement Docker Compose profile selection [quick]

Wave 4 (Docker Compose & Templates):
├── Task 12: Unify docker-compose.yml with profiles [quick]
├── Task 13: Create Caddyfile.template [quick]
└── Task 14: Add .env.example file [quick]

Wave FINAL (Verification):
├── Task F1: End-to-end Sentinel mode test [deep]
├── Task F2: End-to-end Native mode test [quick]
├── Task F3: Migration/upgrade test [unspecified-high]
└── Task F4: Security review [deep]
-> Present results -> Get explicit user okay
```

### Dependency Matrix

| Task | Depends On | Blocks |
|------|------------|--------|
| 1 | - | 2, 3, 5, 6 |
| 2 | 1 | 3 |
| 3 | 1, 2 | F1, F2 |
| 4 | 1 | F1 |
| 5 | 1 | 7 |
| 6 | 1 | F1 |
| 7 | 5 | 9, 10 |
| 8 | - | 9 |
| 9 | 7, 8 | 12 |
| 10 | 7 | 13 |
| 11 | - | 12 |
| 12 | 9, 11 | F1, F2 |
| 13 | 10 | F1 |
| 14 | - | 7 |

### Agent Dispatch Summary

- **Wave 1**: 3 tasks → `quick`
- **Wave 2**: 3 tasks → `quick`
- **Wave 3**: 5 tasks → `quick` (2), `unspecified-high` (1)
- **Wave 4**: 3 tasks → `quick`
- **FINAL**: 4 tasks → `deep` (2), `quick` (1), `unspecified-high` (1)

---

## TODOs

- [x] 1. Extend ServerConfig with Sentinel Fields

  **What to do**:
  - Add `Mode` field (string: "native" | "sentinel")
  - Add `RPCTransport` field (string: "unix" | "tcp")
  - Add `ListenAddr` field (string: TCP address like "0.0.0.0:8080")
  - Add `PublicBaseURL` field (string: "https://domain.com")
  - Add `AdminToken` field (string: generated token)
  - Add env var tags: `env:"ARMORCLAW_SERVER_MODE"`, etc.
  - Update `DefaultConfig()` to use native mode by default

  **Must NOT do**:
  - Do NOT change existing field names (backward compatibility)
  - Do NOT remove Unix socket support

  **Recommended Agent Profile**:
  - **Category**: `quick`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: NO (blocks tasks 2, 3)
  - **Parallel Group**: Wave 1
  - **Blocks**: Tasks 2, 3, 5, 6
  - **Blocked By**: None

  **References**:
  - `bridge/pkg/config/config.go:121-136` - Current ServerConfig
  - `bridge/pkg/config/config.go:772-779` - DefaultConfig

  **Acceptance Criteria**:
  - [ ] ServerConfig has Mode, RPCTransport, ListenAddr, PublicBaseURL, AdminToken fields
  - [ ] All fields have env var tags
  - [ ] DefaultConfig returns native mode with unix transport
  - [ ] `go build ./bridge/...` succeeds

  **QA Scenarios**:
  ```
  Scenario: Config loads from environment variables
    Tool: Bash
    Steps:
      1. ARMORCLAW_SERVER_MODE=sentinel ARMORCLAW_RPC_TRANSPORT=tcp go test -v ./pkg/config/... -run TestSentinelConfig
    Expected Result: Test passes with sentinel mode loaded
    Evidence: .sisyphus/evidence/task-1-config-env-test.log

  Scenario: Default config is native mode
    Tool: Bash
    Steps:
      1. go test -v ./pkg/config/... -run TestDefaultConfig
    Expected Result: Mode="native", RPCTransport="unix"
    Evidence: .sisyphus/evidence/task-1-default-config-test.log
  ```

  **Commit**: YES
  - Message: `feat(config): add sentinel mode configuration fields`
  - Files: `bridge/pkg/config/config.go`

---

- [x] 2. Add TCP Transport to RPC Server

  **What to do**:
  - Modify `Run(socketPath string)` to accept config or detect transport type
  - Add TCP listener path: `net.Listen("tcp", addr)`
  - Log transport type: "RPC transport: tcp" or "RPC transport: unix socket"
  - Support both via ServerConfig

  **Must NOT do**:
  - Do NOT remove Windows TCP fallback
  - Do NOT break existing Unix socket behavior

  **Recommended Agent Profile**:
  - **Category**: `quick`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: NO (depends on task 1)
  - **Parallel Group**: Wave 1
  - **Blocks**: Task 3
  - **Blocked By**: Task 1

  **References**:
  - `bridge/pkg/rpc/server.go:927-963` - Current Run() implementation

  **Acceptance Criteria**:
  - [ ] Server supports TCP transport when configured
  - [ ] Unix socket still works in native mode
  - [ ] Windows fallback preserved
  - [ ] Unit test for both transports

  **QA Scenarios**:
  ```
  Scenario: TCP transport works
    Tool: Bash
    Steps:
      1. Start bridge with ARMORCLAW_RPC_TRANSPORT=tcp ARMORCLAW_LISTEN_ADDR=127.0.0.1:18080
      2. curl http://127.0.0.1:18080/health
    Expected Result: Health check returns 200
    Evidence: .sisyphus/evidence/task-2-tcp-test.log

  Scenario: Unix transport still works
    Tool: Bash
    Steps:
      1. Start bridge with default config
      2. echo '{"jsonrpc":"2.0","id":1,"method":"health.check"}' | socat - UNIX-CONNECT:/run/armorclaw/bridge.sock
    Expected Result: JSON-RPC response received
    Evidence: .sisyphus/evidence/task-2-unix-test.log
  ```

  **Commit**: YES
  - Message: `feat(rpc): add TCP transport support for sentinel mode`
  - Files: `bridge/pkg/rpc/server.go`

---

- [x] 3. Update main.go Listener Selection

  **What to do**:
  - Read ServerConfig.Mode or RPCTransport
  - Pass correct parameters to `rpcServer.Run()`
  - For TCP: pass `cfg.Server.ListenAddr`
  - For Unix: pass `cfg.Server.SocketPath`
  - Log mode on startup

  **Must NOT do**:
  - Do NOT change CLI flag parsing structure

  **Recommended Agent Profile**:
  - **Category**: `quick`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: NO (depends on tasks 1, 2)
  - **Parallel Group**: Wave 1
  - **Blocks**: F1, F2
  - **Blocked By**: Task 1, Task 2

  **References**:
  - `bridge/cmd/bridge/main.go` - Main entry point

  **Acceptance Criteria**:
  - [ ] Bridge starts in TCP mode when configured
  - [ ] Bridge starts in Unix mode by default
  - [ ] Mode logged on startup
  - [ ] Integration test for both modes

  **QA Scenarios**:
  ```
  Scenario: Bridge starts in sentinel mode
    Tool: Bash
    Steps:
      1. ARMORCLAW_SERVER_MODE=sentinel ./bridge --config test-config.toml
      2. Check logs for "RPC transport: tcp"
    Expected Result: TCP listener started
    Evidence: .sisyphus/evidence/task-3-sentinel-start.log

  Scenario: Bridge starts in native mode
    Tool: Bash
    Steps:
      1. ./bridge --config test-config.toml
      2. Check logs for "RPC transport: unix socket"
    Expected Result: Unix socket created
    Evidence: .sisyphus/evidence/task-3-native-start.log
  ```

  **Commit**: YES
  - Message: `feat(bridge): mode-aware listener selection`
  - Files: `bridge/cmd/bridge/main.go`

---

- [x] 4. Update Discovery Endpoint for PublicBaseURL

  **What to do**:
  - Add `Host` field population from `PublicBaseURL` in HTTPServer
  - Parse PublicBaseURL to extract hostname
  - Add `provisioning_available` field to response
  - Add `server_name` field to response

  **Must NOT do**:
  - Do NOT break existing discovery response format

  **Recommended Agent Profile**:
  - **Category**: `quick`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: NO (depends on task 1)
  - **Parallel Group**: Wave 2
  - **Blocks**: F1
  - **Blocked By**: Task 1

  **References**:
  - `bridge/pkg/discovery/http.go:164-197` - handleDiscovery

  **Acceptance Criteria**:
  - [ ] Discovery uses PublicBaseURL when set
  - [ ] Falls back to mDNS hostname when PublicBaseURL empty
  - [ ] Response includes provisioning_available
  - [ ] Response includes server_name

  **QA Scenarios**:
  ```
  Scenario: Discovery returns public URL in sentinel mode
    Tool: Bash
    Steps:
      1. Start bridge with ARMORCLAW_PUBLIC_BASE_URL=https://bridge.example.com
      2. curl http://localhost:8080/api/discovery
    Expected Result: api_url contains "https://bridge.example.com"
    Evidence: .sisyphus/evidence/task-4-discovery-public.log

  Scenario: Discovery returns local URL in native mode
    Tool: Bash
    Steps:
      1. Start bridge without ARMORCLAW_PUBLIC_BASE_URL
      2. curl http://localhost:8080/api/discovery
    Expected Result: api_url contains hostname
    Evidence: .sisyphus/evidence/task-4-discovery-local.log
  ```

  **Commit**: YES
  - Message: `feat(discovery): support PublicBaseURL for sentinel mode`
  - Files: `bridge/pkg/discovery/http.go`

---

- [x] 5. Create Config Loading from Environment Variables

  **What to do**:
  - Implement env var override for all new fields
  - Use existing env tag pattern from config.go
  - Ensure TOML file can be minimal in sentinel mode
  - Add `LoadFromEnv()` helper if needed

  **Must NOT do**:
  - Do NOT break existing TOML loading

  **Recommended Agent Profile**:
  - **Category**: `quick`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: NO (depends on task 1)
  - **Parallel Group**: Wave 2
  - **Blocks**: Task 7
  - **Blocked By**: Task 1

  **References**:
  - `bridge/pkg/config/config.go` - Existing env tag pattern

  **Acceptance Criteria**:
  - [ ] All sentinel fields loadable from env vars
  - [ ] Env vars override TOML values
  - [ ] Minimal TOML works with env vars

  **QA Scenarios**:
  ```
  Scenario: Full config from environment
    Tool: Bash
    Steps:
      1. export ARMORCLAW_SERVER_MODE=sentinel
      2. export ARMORCLAW_RPC_TRANSPORT=tcp
      3. export ARMORCLAW_LISTEN_ADDR=0.0.0.0:8080
      4. export ARMORCLAW_PUBLIC_BASE_URL=https://test.example.com
      5. go test -v ./pkg/config/... -run TestEnvLoading
    Expected Result: Config loaded from env
    Evidence: .sisyphus/evidence/task-5-env-loading.log
  ```

  **Commit**: YES
  - Message: `feat(config): environment variable loading for sentinel`
  - Files: `bridge/pkg/config/config.go`

---

- [x] 6. Add Validation for Sentinel Mode

  **What to do**:
  - Add validation: if mode=sentinel, require ListenAddr and PublicBaseURL
  - Add validation: if mode=native, require SocketPath
  - Clear error messages for missing fields
  - Update `Validate()` method

  **Must NOT do**:
  - Do NOT break existing validation

  **Recommended Agent Profile**:
  - **Category**: `quick`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: NO (depends on task 1)
  - **Parallel Group**: Wave 2
  - **Blocks**: F1
  - **Blocked By**: Task 1

  **References**:
  - `bridge/pkg/config/config.go:972-1078` - Validate()

  **Acceptance Criteria**:
  - [ ] Sentinel mode requires ListenAddr, PublicBaseURL
  - [ ] Native mode requires SocketPath
  - [ ] Clear error messages
  - [ ] Unit tests for validation

  **QA Scenarios**:
  ```
  Scenario: Sentinel mode validation catches missing fields
    Tool: Bash
    Steps:
      1. ARMORCLAW_SERVER_MODE=sentinel go test -v ./pkg/config/... -run TestSentinelValidation
    Expected Result: Validation error for missing ListenAddr
    Evidence: .sisyphus/evidence/task-6-sentinel-validation.log
  ```

  **Commit**: YES
  - Message: `feat(config): sentinel mode validation`
  - Files: `bridge/pkg/config/config.go`

---

- [x] 7. Implement Domain Detection Logic in Installer

  **What to do**:
  - Add interactive prompt for domain name
  - Detect public IP via `curl -s https://api.ipify.org`
  - Determine mode: domain provided → sentinel, else → native
  - Prompt for email if sentinel mode (for Let's Encrypt)
  - Store detected values for later use

  **Must NOT do**:
  - Do NOT require domain for local installs
  - Do NOT fail if IP detection fails (use fallback)

  **Recommended Agent Profile**:
  - **Category**: `unspecified-high`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: NO (depends on task 5)
  - **Parallel Group**: Wave 3
  - **Blocks**: Tasks 9, 10
  - **Blocked By**: Task 5

  **References**:
  - `deploy/install.sh` - Bootstrap loader
  - `deploy/installer-v5.sh` - Current installer

  **Acceptance Criteria**:
  - [ ] Domain prompt appears
  - [ ] Mode correctly determined
  - [ ] Email prompt for sentinel mode
  - [ ] Public IP detected

  **QA Scenarios**:
  ```
  Scenario: Domain provided triggers sentinel mode
    Tool: Bash
    Steps:
      1. echo "bridge.example.com" | ./deploy/installer-v6.sh --non-interactive
      2. Check MODE=sentinel in output
    Expected Result: Sentinel mode activated
    Evidence: .sisyphus/evidence/task-7-domain-detection.log

  Scenario: No domain triggers native mode
    Tool: Bash
    Steps:
      1. echo "" | ./deploy/installer-v6.sh --non-interactive
      2. Check MODE=native in output
    Expected Result: Native mode activated
    Evidence: .sisyphus/evidence/task-7-native-mode.log
  ```

  **Commit**: YES
  - Message: `feat(installer): domain detection and mode selection`
  - Files: `deploy/installer-v6.sh`

---

- [x] 8. Add Secret Generation

  **What to do**:
  - Generate `ADMIN_TOKEN` (32 bytes, base64)
  - Generate `KEYSTORE_SECRET` (32 bytes, base64)
  - Generate `MATRIX_SECRET` (for Conduit registration)
  - Use `openssl rand -base64 32`
  - Store in variables for .env generation

  **Must NOT do**:
  - Do NOT log secrets to console (except admin token at end)

  **Recommended Agent Profile**:
  - **Category**: `quick`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 3 (with Task 11)
  - **Blocks**: Task 9
  - **Blocked By**: None

  **References**:
  - Existing install scripts

  **Acceptance Criteria**:
  - [ ] All secrets generated with proper entropy
  - [ ] Secrets stored in variables
  - [ ] Only admin token shown to user

  **QA Scenarios**:
  ```
  Scenario: Secrets are unique and properly formatted
    Tool: Bash
    Steps:
      1. Run secret generation function twice
      2. Compare outputs
    Expected Result: Different secrets each time, base64 format
    Evidence: .sisyphus/evidence/task-8-secret-gen.log
  ```

  **Commit**: YES
  - Message: `feat(installer): secure secret generation`
  - Files: `deploy/installer-v6.sh`

---

- [x] 9. Implement .env File Generation

  **What to do**:
  - Create `/etc/armorclaw/.env` file
  - Include all generated secrets
  - Include mode-specific variables
  - Include Matrix configuration
  - Set proper permissions (0600)

  **Must NOT do**:
  - Do NOT overwrite existing .env without backup
  - Do NOT make .env world-readable

  **Recommended Agent Profile**:
  - **Category**: `quick`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: NO (depends on tasks 7, 8)
  - **Parallel Group**: Wave 3
  - **Blocks**: Task 12
  - **Blocked By**: Task 7, Task 8

  **References**:
  - Plan's .env example

  **Acceptance Criteria**:
  - [ ] .env file created with all variables
  - [ ] Permissions set to 0600
  - [ ] Existing .env backed up

  **QA Scenarios**:
  ```
  Scenario: .env file created correctly
    Tool: Bash
    Steps:
      1. Run installer in sentinel mode
      2. Check /etc/armorclaw/.env exists
      3. Verify permissions are 0600
      4. Verify all required variables present
    Expected Result: .env file with correct format and permissions
    Evidence: .sisyphus/evidence/task-9-env-file.log
  ```

  **Commit**: YES
  - Message: `feat(installer): .env file generation`
  - Files: `deploy/installer-v6.sh`

---

- [x] 10. Create Caddyfile Template

  **What to do**:
  - Create `configs/Caddyfile.template`
  - Use variables: `${DOMAIN_NAME}`, `${ADMIN_EMAIL}`
  - Include routes: /api, /health, /discover, /_matrix/*
  - Enable automatic HTTPS

  **Must NOT do**:
  - Do NOT hardcode domain

  **Recommended Agent Profile**:
  - **Category**: `quick`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: NO (depends on task 7)
  - **Parallel Group**: Wave 3
  - **Blocks**: Task 13
  - **Blocked By**: Task 7

  **References**:
  - Plan's Caddyfile example

  **Acceptance Criteria**:
  - [ ] Template created with variable placeholders
  - [ ] All routes configured
  - [ ] TLS enabled

  **QA Scenarios**:
  ```
  Scenario: Caddyfile renders correctly
    Tool: Bash
    Steps:
      1. sed -e 's/${DOMAIN_NAME}/test.example.com/g' -e 's/${ADMIN_EMAIL}/admin@test.example.com/g' configs/Caddyfile.template
      2. Validate Caddyfile syntax with caddy validate
    Expected Result: Valid Caddyfile output
    Evidence: .sisyphus/evidence/task-10-caddyfile.log
  ```

  **Commit**: YES
  - Message: `feat(configs): Caddyfile template for sentinel mode`
  - Files: `configs/Caddyfile.template`

---

- [x] 11. Implement Docker Compose Profile Selection

  **What to do**:
  - Add `--profile sentinel` flag when sentinel mode detected
  - Default to no profile (native mode)
  - Update installer to call correct docker-compose command

  **Must NOT do**:
  - Do NOT break existing compose commands

  **Recommended Agent Profile**:
  - **Category**: `quick`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 3 (with Task 8)
  - **Blocks**: Task 12
  - **Blocked By**: None

  **References**:
  - `docker-compose.yml`

  **Acceptance Criteria**:
  - [ ] Sentinel mode activates proxy profile
  - [ ] Native mode runs without profiles
  - [ ] Installer outputs correct command

  **QA Scenarios**:
  ```
  Scenario: Correct compose command generated
    Tool: Bash
    Steps:
      1. Run installer in sentinel mode
      2. Check output contains "--profile sentinel"
    Expected Result: Profile flag present
    Evidence: .sisyphus/evidence/task-11-profile.log
  ```

  **Commit**: YES
  - Message: `feat(installer): docker compose profile selection`
  - Files: `deploy/installer-v6.sh`

---

- [x] 12. Unify docker-compose.yml with Profiles

  **What to do**:
  - Add `profiles: [sentinel]` to proxy service
  - Use `${VARIABLE:-default}` for all config
  - Remove hardcoded values
  - Ensure bridge service works in both modes

  **Must NOT do**:
  - Do NOT break existing deployments

  **Recommended Agent Profile**:
  - **Category**: `quick`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: NO (depends on tasks 9, 11)
  - **Parallel Group**: Wave 4
  - **Blocks**: F1, F2
  - **Blocked By**: Task 9, Task 11

  **References**:
  - `docker-compose.yml`
  - `docker-compose.bridge.yml`

  **Acceptance Criteria**:
  - [ ] Single compose file for both modes
  - [ ] Profile-based proxy activation
  - [ ] All variables from .env

  **QA Scenarios**:
  ```
  Scenario: Native mode starts without proxy
    Tool: Bash
    Steps:
      1. docker-compose up -d
      2. docker ps | grep -c caddy
    Expected Result: No caddy container (0)
    Evidence: .sisyphus/evidence/task-12-native-compose.log

  Scenario: Sentinel mode starts with proxy
    Tool: Bash
    Steps:
      1. docker-compose --profile sentinel up -d
      2. docker ps | grep -c caddy
    Expected Result: Caddy container running (1)
    Evidence: .sisyphus/evidence/task-12-sentinel-compose.log
  ```

  **Commit**: YES
  - Message: `feat(docker): unified compose with sentinel profile`
  - Files: `docker-compose.yml`

---

- [ ] 13. Finalize Caddyfile Configuration

  **What to do**:
  - Installer generates Caddyfile from template
  - Place at `/etc/armorclaw/Caddyfile`
  - Mount in docker-compose for proxy service

  **Must NOT do**:
  - Do NOT commit generated Caddyfile

  **Recommended Agent Profile**:
  - **Category**: `quick`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: NO (depends on task 10)
  - **Parallel Group**: Wave 4
  - **Blocks**: F1
  - **Blocked By**: Task 10

  **References**:
  - `configs/Caddyfile.template`

  **Acceptance Criteria**:
  - [ ] Caddyfile generated at install time
  - [ ] Correct mount in compose
  - [ ] Syntax validated

  **QA Scenarios**:
  ```
  Scenario: Caddyfile generated and valid
    Tool: Bash
    Steps:
      1. Run installer in sentinel mode
      2. cat /etc/armorclaw/Caddyfile
      3. caddy validate --config /etc/armorclaw/Caddyfile
    Expected Result: Valid Caddyfile
    Evidence: .sisyphus/evidence/task-13-caddyfile-gen.log
  ```

  **Commit**: YES
  - Message: `feat(installer): Caddyfile generation from template`
  - Files: `deploy/installer-v6.sh`

---

- [x] 14. Add .env.example File

  **What to do**:
  - Create `configs/.env.example` with all variables documented
  - Include comments explaining each variable
  - Include default values where applicable

  **Must NOT do**:
  - Do NOT include actual secrets in example

  **Recommended Agent Profile**:
  - **Category**: `quick`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 4 (independent)
  - **Blocks**: None
  - **Blocked By**: None

  **References**:
  - Plan's .env example

  **Acceptance Criteria**:
  - [ ] All variables documented
  - [ ] No secrets included
  - [ ] Comments explain each variable

  **QA Scenarios**:
  ```
  Scenario: .env.example is complete
    Tool: Bash
    Steps:
      1. grep -c '^ARMORCLAW_' configs/.env.example
    Expected Result: Count >= 10 (all major variables)
    Evidence: .sisyphus/evidence/task-14-env-example.log
  ```

  **Commit**: YES
  - Message: `docs(configs): add .env.example with documentation`
  - Files: `configs/.env.example`

---

## Final Verification Wave

- [x] F1. End-to-End Sentinel Mode Test

  **What to do**: Run full installer in sentinel mode, verify ArmorChat can connect

  **Recommended Agent Profile**: `deep`

  **QA Scenarios**:
  ```
  Scenario: Full sentinel deployment works
    Tool: Bash
    Steps:
      1. Run install.sh with domain "test.example.com"
      2. Wait for services to start
      3. curl https://test.example.com/api/discovery
      4. Verify TLS certificate issued
    Expected Result: Discovery returns correct URLs, TLS valid
    Evidence: .sisyphus/evidence/f1-sentinel-e2e.log
  ```

- [x] F2. End-to-End Native Mode Test

  **What to do**: Run installer without domain, verify Unix socket access

  **Recommended Agent Profile**: `quick`

  **QA Scenarios**:
  ```
  Scenario: Native mode unchanged
    Tool: Bash
    Steps:
      1. Run install.sh without domain
      2. Test Unix socket connection
      3. Verify no TCP listener on 8080
    Expected Result: Unix socket works, no TCP exposed
    Evidence: .sisyphus/evidence/f2-native-e2e.log
  ```

- [x] F3. Migration/Upgrade Test

  **What to do**: Test upgrading existing native install to sentinel

  **Recommended Agent Profile**: `unspecified-high`

  **QA Scenarios**:
  ```
  Scenario: Native to sentinel upgrade
    Tool: Bash
    Steps:
      1. Install in native mode
      2. Re-run installer with domain
      3. Verify config migrated correctly
    Expected Result: Sentinel mode activated, data preserved
    Evidence: .sisyphus/evidence/f3-migration.log
  ```

- [x] F4. Security Review

  **What to do**: Review secret handling, file permissions, TLS configuration

  **Recommended Agent Profile**: `deep`

  **QA Scenarios**:
  ```
  Scenario: Secrets are secure
    Tool: Bash
    Steps:
      1. Check .env permissions (should be 0600)
      2. Check logs don't contain secrets
      3. Verify TLS configuration
    Expected Result: No secrets leaked, proper permissions
    Evidence: .sisyphus/evidence/f4-security.log
  ```

---

## Commit Strategy

| Wave | Commit Pattern |
|------|----------------|
| 1 | `feat(config|rpc|bridge): sentinel mode foundation` |
| 2 | `feat(discovery|config): sentinel integration` |
| 3 | `feat(installer): automatic mode detection and config` |
| 4 | `feat(docker): unified compose with profiles` |
| FINAL | `test(e2e): sentinel deployment verification` |

---

## Success Criteria

### Verification Commands

```bash
# Sentinel mode
curl -fsSL https://.../install.sh | bash
# Enter domain when prompted
# Verify: curl https://domain.com/api/discovery

# Native mode
curl -fsSL https://.../install.sh | bash
# Leave domain blank
# Verify: socat - UNIX-CONNECT:/run/armorclaw/bridge.sock
```

### Final Checklist

- [x] All "Must Have" present
- [x] All "Must NOT Have" absent
- [x] E2E tests pass for both modes
- [x] Security review complete
- [ ] Documentation updated
