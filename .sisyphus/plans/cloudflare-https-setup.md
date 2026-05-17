# Cloudflare HTTPS Setup Plan (Expanded)

> **Objective**: Add Cloudflare HTTPS setup with automatic mode detection, supporting both Tunnel (no ports) and Proxy (full features) approaches.
>
> **Design Principle**: Automatic detection with smart recommendation, user override always available.

---

## TL;DR

> **Quick Summary**: Extend setup-quick.sh with intelligent Cloudflare HTTPS setup that auto-detects network environment (NAT, open ports) and recommends the best approach—Tunnel for restricted networks, Proxy for VPS with open ports.
>
> **Deliverables**:
> - Automatic network detection (NAT, port availability)
> - Interactive prompt with smart recommendation
> - Cloudflare Tunnel setup (Approach A)
> - Cloudflare Proxy setup with origin certificate (Approach B)
> - Standalone setup-cloudflare.sh script
> - Dry-run testing mode
>
> **Estimated Effort**: Large
> **Parallel Execution**: YES - 4 waves
> **Critical Path**: Functions → Detection → Integration → Testing

---

## Context

### Original Request
Add optional Cloudflare Tunnel setup to `setup-quick.sh` interactive flow as Option 6.

### Clarification Session
**Key Discussions**:
- **Approach**: Both Tunnel and Proxy needed (not just Tunnel)
- **Use Case**: Mixed - 80% VPS, 20% Home/NAT
- **Auto-Detection**: YES - detect NAT and port availability, recommend best option
- **User Override**: Always allow user to choose different option

**Research Findings**:
- **Tunnel**: Simpler, no ports, works behind NAT, slightly slower
- **Proxy**: Faster, full Cloudflare features, requires origin certificate
- **Detection**: Check public vs local IP, test ports 80/443

### Metis Review
**Identified Gaps** (addressed):
- Gap: Only Tunnel planned, but Proxy also needed → Added Phase 2
- Gap: No auto-detection → Added detect_network_environment()
- Gap: User might not know which to choose → Added smart recommendation

---

## Work Objectives

### Core Objective

Implement dual-approach Cloudflare HTTPS setup with intelligent mode selection:
1. Automatic network environment detection
2. Cloudflare Tunnel setup (no ports, behind NAT)
3. Cloudflare Proxy setup (faster, full features)
4. Smart recommendation with user override

### Concrete Deliverables

- `deploy/lib/cloudflare-functions.sh` - Shared library (Tunnel + Proxy)
- `deploy/setup-quick.sh` - Updated with Cloudflare option
- `deploy/setup-cloudflare.sh` - Standalone script
- `tests/test-cloudflare-setup.sh` - Integration tests

### Definition of Done

- [ ] Network detection works (NAT, ports)
- [ ] Recommendation displays based on detection
- [ ] User can choose either approach
- [ ] Tunnel mode: cloudflared installed, tunnel created, DNS routed
- [ ] Proxy mode: A record created, origin cert installed, Caddy configured
- [ ] Both result in working HTTPS
- [ ] Dry-run mode works for CI
- [ ] Edge cases handled

### Must Have

- Automatic network detection
- Both Tunnel and Proxy approaches
- Smart recommendation with reasoning
- User override option
- Cloudflare Tunnel: cloudflared install, auth, tunnel create, DNS route
- Cloudflare Proxy: A record, origin certificate, Caddy config

### Must NOT Have (Guardrails)

- NO breaking changes to existing setup modes
- NO requirement for Cloudflare account (optional)
- NO removal of Quick Tunnel option
- NO automatic domain registration (user must own domain)
- NO Cloudflare Access setup (out of scope)
- NO multiple domain support (single domain per setup)

---

## Verification Strategy (MANDATORY)

### Test Decision

- **Infrastructure exists**: YES (existing test scripts)
- **Automated tests**: YES - Dry-run mode + integration tests
- **Framework**: bash test scripts
- **Agent-Executed QA**: YES

### QA Policy

Every task includes agent-executed QA scenarios:
- **CLI/Installer**: Bash execution with output validation
- **Network detection**: Mock different environments
- **Tunnel/Proxy setup**: Mock cloudflared and Cloudflare API for CI, real testing with test domain

---

## Execution Strategy

### Parallel Execution Waves

```
Wave 1 (Core Functions - Tunnel):
├── Task 1: Create shared function library [quick]
├── Task 2: Implement prompt_cloudflare_mode() [quick]
├── Task 3: Implement detect_network_environment() [quick]
└── Task 4: Implement setup_cloudflare_tunnel() [unspecified-high]

Wave 2 (Proxy Mode):
├── Task 5: Implement setup_cloudflare_proxy() [unspecified-high]
├── Task 6: Implement create_cloudflare_origin_cert() [quick]
├── Task 7: Implement configure_caddy_origin() [quick]
├── Task 8: Implement create_dns_a_record() [quick]
└── Task 9: Implement setup_manual_domain() [quick]

Wave 3 (Integration):
├── Task 10: Update setup-quick.sh main() [quick]
├── Task 11: Create standalone setup-cloudflare.sh [quick]
└── Task 12: Add edge case handlers [unspecified-high]

Wave 4 (Testing):
├── Task 13: Create dry-run mode [quick]
├── Task 14: Write integration tests [quick]
└── Task 15: Update documentation [quick]

Wave FINAL (Verification):
├── Task F1: Tunnel E2E test [deep]
├── Task F2: Proxy E2E test [deep]
└── Task F3: Detection + recommendation test [unspecified-high]
```

### Dependency Matrix

| Task | Depends On | Blocks |
|------|------------|--------|
| 1 | - | 2, 3, 4, 5, 6, 7, 8, 11 |
| 2 | 1, 3 | 10 |
| 3 | 1 | 2, 10, 11 |
| 4 | 1 | 10, 11, F1 |
| 5 | 1 | 10, 11, F2 |
| 6 | 1 | 5 |
| 7 | 1 | 5 |
| 8 | 1 | 5 |
| 9 | 1 | 10 |
| 10 | 2, 4, 5, 9 | F1, F2, F3 |
| 11 | 1, 3, 4, 5 | F1, F2, F3 |
| 12 | 4, 5 | F3 |
| 13 | 4, 5, 11 | 14 |
| 14 | 13 | F1, F2, F3 |
| 15 | 10, 11 | - |

---

## TODOs

- [ ] 1. Create Shared Function Library

  **What to do**:
  - Create `deploy/lib/cloudflare-functions.sh`
  - Implement `check_cloudflare_prerequisites()`
  - Implement `install_cloudflared()`
  - Implement `check_cloudflare_nameservers()`
  - Add error handling utilities
  - Add logging functions

  **Must NOT do**:
  - Do NOT modify existing setup scripts yet
  - Do NOT add mode-specific logic yet

  **Recommended Agent Profile**:
  - **Category**: `quick`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: NO (blocks all other tasks)
  - **Parallel Group**: Wave 1
  - **Blocks**: Tasks 2-9, 11
  - **Blocked By**: None

  **References**:
  - User spec: Network detection logic
  - Existing: `deploy/installer-v6.sh` for pattern reference

  **Acceptance Criteria**:
  - [ ] File created: `deploy/lib/cloudflare-functions.sh`
  - [ ] Functions are testable independently
  - [ ] Bash syntax valid: `bash -n deploy/lib/cloudflare-functions.sh`

  **QA Scenarios**:
  ```
  Scenario: Library functions are loadable
    Tool: Bash
    Steps:
      1. source deploy/lib/cloudflare-functions.sh
      2. type check_cloudflare_prerequisites
      3. type install_cloudflared
    Expected Result: Both functions defined
    Evidence: .sisyphus/evidence/task-1-library-load.log
  ```

  **Commit**: YES
  - Message: `feat(deploy): add Cloudflare shared function library`
  - Files: `deploy/lib/cloudflare-functions.sh`

---

- [ ] 2. Implement Interactive Mode Prompt

  **What to do**:
  - Create `prompt_cloudflare_mode()` function
  - Display detected environment summary
  - Display recommendation with reasoning
  - Show both options (Tunnel vs Proxy) with descriptions
  - Validate user input (1 or 2)
  - Export `CF_MODE` variable (`tunnel` or `proxy`)

  **Must NOT do**:
  - Do NOT integrate into main() yet (Task 10)
  - Do NOT call setup functions yet

  **Recommended Agent Profile**:
  - **Category**: `quick`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: NO (depends on tasks 1, 3)
  - **Parallel Group**: Wave 1
  - **Blocks**: Task 10
  - **Blocked By**: Tasks 1, 3

  **References**:
  - User clarification: Prompt design with auto-detection display

  **Acceptance Criteria**:
  - [ ] Function displays detected environment
  - [ ] Function shows recommendation with reason
  - [ ] Both options displayed with clear descriptions
  - [ ] Input validation accepts 1 or 2 only
  - [ ] CF_MODE exported correctly
  - [ ] Bash syntax valid

  **QA Scenarios**:
  ```
  Scenario: Prompt displays recommendation
    Tool: Bash
    Steps:
      1. Mock RECOMMEND="proxy"
      2. source deploy/lib/cloudflare-functions.sh && prompt_cloudflare_mode
      3. Check output contains "Recommended"
    Expected Result: Recommendation shown
    Evidence: .sisyphus/evidence/task-2-prompt-recommend.log

  Scenario: Prompt accepts valid input
    Tool: Bash
    Steps:
      1. echo "2" | prompt_cloudflare_mode
      2. echo $CF_MODE
    Expected Result: CF_MODE=tunnel
    Evidence: .sisyphus/evidence/task-2-prompt-valid.log
  ```

  **Commit**: YES
  - Message: `feat(deploy): add interactive Cloudflare mode prompt`
  - Files: `deploy/lib/cloudflare-functions.sh`

---

- [ ] 3. Implement Network Environment Detection

  **What to do**:
  - Create `detect_network_environment()` function
  - Get public IP via api.ipify.org
  - Get local IP via `ip` command
  - Detect NAT (public IP != local IP)
  - Test TCP connectivity to ports 80 and 443
  - Set `RECOMMEND` variable based on detection
  - Set `REASON` variable with explanation

  **Must NOT do**:
  - Do NOT fail on detection errors (use fallback)
  - Do NOT require specific network configuration

  **Recommended Agent Profile**:
  - **Category**: `quick`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: NO (depends on task 1)
  - **Parallel Group**: Wave 1
  - **Blocks**: Tasks 2, 10, 11
  - **Blocked By**: Task 1

  **References**:
  - User clarification: Detection logic with NAT and port checks

  **Acceptance Criteria**:
  - [ ] Public IP detected successfully
  - [ ] Local IP detected successfully
  - [ ] NAT detection works correctly
  - [ ] Port 80/443 availability checked
  - [ ] Recommendation set based on environment
  - [ ] Reason provided for recommendation
  - [ ] Bash syntax valid

  **QA Scenarios**:
  ```
  Scenario: Detection identifies NAT environment
    Tool: Bash
    Steps:
      1. Mock different public/local IPs
      2. Call detect_network_environment
      3. Check RECOMMEND="tunnel"
    Expected Result: Tunnel recommended for NAT
    Evidence: .sisyphus/evidence/task-3-detect-nat.log

  Scenario: Detection identifies VPS with open ports
    Tool: Bash
    Steps:
      1. Mock same public/local IP
      2. Mock open ports
      3. Call detect_network_environment
      4. Check RECOMMEND="proxy"
    Expected Result: Proxy recommended for VPS
    Evidence: .sisyphus/evidence/task-3-detect-vps.log
  ```

  **Commit**: YES
  - Message: `feat(deploy): add network environment detection`
  - Files: `deploy/lib/cloudflare-functions.sh`

---

- [ ] 4. Implement Cloudflare Tunnel Setup

  **What to do**:
  - Create `setup_cloudflare_tunnel()` function
  - Check if cloudflared installed, install if needed
  - Prompt for domain input
  - Extract subdomain and base domain
  - Check existing authentication
  - Trigger `cloudflared tunnel login` if needed
  - Create or reuse tunnel named "armorclaw-<domain>"
  - Generate `~/.cloudflared/config.yml` with ingress rules
  - Create DNS records via `cloudflared tunnel route dns`
  - Install as systemd service `cloudflared-tunnel`
  - Export `PUBLIC_URL`, `MATRIX_URL`, `DOMAIN`

  **Must NOT do**:
  - Do NOT integrate into main() yet
  - Do NOT remove existing tunnel options

  **Recommended Agent Profile**:
  - **Category**: `unspecified-high`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: NO (depends on task 1)
  - **Parallel Group**: Wave 1
  - **Blocks**: Tasks 10, 11, F1
  - **Blocked By**: Task 1

  **References**:
  - User spec: Tunnel setup flow
  - Cloudflare docs: cloudflared tunnel commands

  **Acceptance Criteria**:
  - [ ] Function handles fresh cloudflared install
  - [ ] Function handles existing cloudflared
  - [ ] Authentication flow works
  - [ ] Tunnel created or reused
  - [ ] Config generated correctly
  - [ ] DNS records created
  - [ ] Systemd service installed
  - [ ] Variables exported for downstream use
  - [ ] Bash syntax valid

  **QA Scenarios**:
  ```
  Scenario: Fresh cloudflared install
    Tool: Bash
    Steps:
      1. Mock cloudflared not installed
      2. Call setup_cloudflare_tunnel with test domain
      3. Verify binary downloaded and installed
    Expected Result: cloudflared in /usr/local/bin
    Evidence: .sisyphus/evidence/task-4-fresh-install.log

  Scenario: Existing tunnel reuse
    Tool: Bash
    Steps:
      1. Mock existing tunnel "armorclaw-test"
      2. Call setup_cloudflare_tunnel
      3. Verify tunnel reused, not recreated
    Expected Result: Same tunnel ID
    Evidence: .sisyphus/evidence/task-4-existing-tunnel.log

  Scenario: Config file generated correctly
    Tool: Bash
    Steps:
      1. Call setup_cloudflare_tunnel with "test.example.com"
      2. Read ~/.cloudflared/config.yml
      3. Verify ingress rules for both domains
    Expected Result: Config contains test.example.com and matrix.test.example.com
    Evidence: .sisyphus/evidence/task-4-config-gen.log
  ```

  **Commit**: YES
  - Message: `feat(deploy): implement Cloudflare Tunnel setup function`
  - Files: `deploy/lib/cloudflare-functions.sh`

---

- [ ] 5. Implement Cloudflare Proxy Setup

  **What to do**:
  - Create `setup_cloudflare_proxy()` function
  - Verify ports 80/443 are open
  - Call `create_dns_a_record()` to proxy enabled
  - Call `create_cloudflare_origin_cert()` to get cert
  - Call `configure_caddy_origin()` to configure Caddy
  - Display SSL/TLS instructions (set to Full Strict)
  - Export `PUBLIC_URL`, `MATRIX_URL`, `DOMAIN`

  **Must NOT do**:
  - Do NOT modify Caddy if already configured for Let's Encrypt
  - Do NOT proceed if ports are blocked

  **Recommended Agent Profile**:
  - **Category**: `unspecified-high`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: NO (depends on tasks 1, 6-8)
  - **Parallel Group**: Wave 2
  - **Blocks**: Tasks 10, 11, F2
  - **Blocked By**: Tasks 1, 6, 7, 8

  **References**:
  - User clarification: Proxy mode with origin certificate
  - Cloudflare docs: Origin certificates, SSL/TLS modes

  **Acceptance Criteria**:
  - [ ] Function verifies ports are accessible
  - [ ] DNS A record created with proxy enabled
  - [ ] Origin certificate generated
  - [ ] Caddy configured with origin cert
  - [ ] SSL/TLS mode instructions displayed
  - [ ] Variables exported for downstream use
  - [ ] Bash syntax valid

  **QA Scenarios**:
  ```
  Scenario: Proxy setup with open ports
    Tool: Bash
    Steps:
      1. Mock open ports 80/443
      2. Call setup_cloudflare_proxy with test domain
      3. Verify A record creation attempted
      4. Verify origin cert generated
      5. Verify Caddy config updated
    Expected Result: All steps complete successfully
    Evidence: .sisyphus/evidence/task-5-proxy-setup.log

  Scenario: Proxy setup blocked by closed ports
    Tool: Bash
    Steps:
      1. Mock closed ports 80/443
      2. Call setup_cloudflare_proxy
      3. Verify error message shown
    Expected Result: Error about blocked ports
    Evidence: .sisyphus/evidence/task-5-ports-blocked.log
  ```

  **Commit**: YES
  - Message: `feat(deploy): implement Cloudflare Proxy setup function`
  - Files: `deploy/lib/cloudflare-functions.sh`

---

- [ ] 6. Implement Cloudflare Origin Certificate Generation

  **What to do**:
  - Create `create_cloudflare_origin_cert()` function
  - Use Cloudflare API to generate origin certificate
  - Save certificate and key to files
  - Return paths to cert files
  - Handle API errors gracefully

  **Must NOT do**:
  - Do NOT expose API token in logs
  - Do NOT overwrite existing certs without backup

  **Recommended Agent Profile**:
  - **Category**: `quick`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: NO (depends on task 1)
  - **Parallel Group**: Wave 2
  - **Blocks**: Task 5
  - **Blocked By**: Task 1

  **References**:
  - Cloudflare API docs: Origin certificates
  - User clarification: 15-year validity origin certs

  **Acceptance Criteria**:
  - [ ] Function generates origin certificate
  - [ ] Certificate saved to correct path
  - [ ] Key saved to correct path
  - [ ] Paths returned for use
  - [ ] Bash syntax valid

  **QA Scenarios**:
  ```
  Scenario: Origin certificate generated
    Tool: Bash
    Steps:
      1. Mock Cloudflare API
      2. Call create_cloudflare_origin_cert
      3. Verify cert and key files created
    Expected Result: Both files exist
    Evidence: .sisyphus/evidence/task-6-origin-cert.log
  ```

  **Commit**: YES
  - Message: `feat(deploy): add Cloudflare origin certificate generation`
  - Files: `deploy/lib/cloudflare-functions.sh`

---

- [ ] 7. Implement Caddy Origin Certificate Configuration

  **What to do**:
  - Create `configure_caddy_origin()` function
  - Update Caddyfile with origin certificate paths
  - Configure TLS settings for domain
  - Test Caddy config validity

  **Must NOT do**:
  - Do NOT remove existing Caddy config
  - Do NOT break Let's Encrypt config if present

  **Recommended Agent Profile**:
  - **Category**: `quick`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: NO (depends on task 1)
  - **Parallel Group**: Wave 2
  - **Blocks**: Task 5
  - **Blocked By**: Task 1

  **References**:
  - Existing: `configs/Caddyfile.template`
  - Caddy docs: TLS configuration

  **Acceptance Criteria**:
  - [ ] Caddyfile updated with origin cert paths
  - [ ] TLS block configured correctly
  - [ ] Config validates: `caddy validate --config`
  - [ ] Bash syntax valid

  **QA Scenarios**:
  ```
  Scenario: Caddy configured with origin cert
    Tool: Bash
    Steps:
      1. Call configure_caddy_origin with test paths
      2. Read generated Caddyfile
      3. Verify tls block present
    Expected Result: Caddyfile contains origin cert config
    Evidence: .sisyphus/evidence/task-7-caddy-origin.log
  ```

  **Commit**: YES
  - Message: `feat(deploy): add Caddy origin certificate configuration`
  - Files: `deploy/lib/cloudflare-functions.sh`

---

- [ ] 8. Implement DNS A Record Creation

  **What to do**:
  - Create `create_dns_a_record()` function
  - Use Cloudflare API to create A record
  - Enable proxy (orange cloud)
  - Create both main domain and matrix subdomain records
  - Handle existing record conflicts

  **Must NOT do**:
  - Do NOT disable proxy mode
  - Do NOT delete existing records without confirmation

  **Recommended Agent Profile**:
  - **Category**: `quick`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: NO (depends on task 1)
  - **Parallel Group**: Wave 2
  - **Blocks**: Task 5
  - **Blocked By**: Task 1

  **References**:
  - Cloudflare API docs: DNS records
  - User clarification: A record with proxy enabled

  **Acceptance Criteria**:
  - [ ] Function creates A record via API
  - [ ] Proxy mode enabled (orange cloud)
  - [ ] Both domain and matrix subdomain records created
  - [ ] Bash syntax valid

  **QA Scenarios**:
  ```
  Scenario: DNS A records created with proxy
    Tool: Bash
    Steps:
      1. Mock Cloudflare API
      2. Call create_dns_a_record with test domain and IP
      3. Verify API call includes "proxied": true
    Expected Result: A record created with proxy enabled
    Evidence: .sisyphus/evidence/task-8-dns-a-record.log
  ```

  **Commit**: YES
  - Message: `feat(deploy): add DNS A record creation with proxy`
  - Files: `deploy/lib/cloudflare-functions.sh`

---

- [ ] 9. Implement Manual Domain Setup

  **What to do**:
  - Create `setup_manual_domain()` function
  - Prompt for domain input
  - Detect public IP via api.ipify.org
  - Display DNS configuration instructions
  - Wait for user confirmation
  - Export `PUBLIC_URL`, `MATRIX_URL`, `DOMAIN`

  **Must NOT do**:
  - Do NOT create DNS records automatically
  - Do NOT verify DNS propagation

  **Recommended Agent Profile**:
  - **Category**: `quick`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: NO (depends on task 1)
  - **Parallel Group**: Wave 2
  - **Blocks**: Task 10
  - **Blocked By**: Task 1

  **References**:
  - User spec section 3: Manual domain setup

  **Acceptance Criteria**:
  - [ ] Function prompts for domain
  - [ ] Public IP detected and displayed
  - [ ] DNS instructions clear and complete
  - [ ] Variables exported for downstream use
  - [ ] Bash syntax valid

  **QA Scenarios**:
  ```
  Scenario: Manual domain displays instructions
    Tool: Bash
    Steps:
      1. echo "test.example.com" | setup_manual_domain
      2. Check output contains "A record"
      3. Check output contains public IP
    Expected Result: Clear DNS instructions displayed
    Evidence: .sisyphus/evidence/task-9-manual-domain.log
  ```

  **Commit**: YES
  - Message: `feat(deploy): add manual domain setup option`
  - Files: `deploy/lib/cloudflare-functions.sh`

---

- [ ] 10. Integrate Cloudflare HTTPS into setup-quick.sh

  **What to do**:
  - Update `main()` in setup-quick.sh
  - Call `detect_network_environment()` first
  - Call `prompt_cloudflare_mode()` with detection results
  - Add case statement for CF_MODE
  - Route to appropriate setup function (tunnel/proxy/manual)
  - Ensure existing modes still work

  **Must NOT do**:
  - Do NOT break existing setup flow
  - Do NOT remove existing access modes

  **Recommended Agent Profile**:
  - **Category**: `quick`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: NO (depends on tasks 2, 4, 5, 9)
  - **Parallel Group**: Wave 3
  - **Blocks**: F1, F2, F3
  - **Blocked By**: Tasks 2, 4, 5, 9

  **References**:
  - Existing: `deploy/setup-quick.sh`
  - User clarification: Updated main() flow

  **Acceptance Criteria**:
  - [ ] Network detection called first
  - [ ] Prompt shows recommendation with reasoning
  - [ ] All modes route correctly
  - [ ] Existing modes unchanged
  - [ ] New Cloudflare modes work
  - [ ] Bash syntax valid

  **QA Scenarios**:
  ```
  Scenario: Native mode still works
    Tool: Bash
    Steps:
      1. echo "1" | bash deploy/setup-quick.sh --dry-run
      2. Check ACCESS_MODE=native
    Expected Result: Native mode configured
    Evidence: .sisyphus/evidence/task-10-native-mode.log

  Scenario: Cloudflare Tunnel mode routes correctly
    Tool: Bash
    Steps:
      1. Mock NAT environment
      2. Run setup-quick.sh
      3. Verify Tunnel recommended
      4. Select Tunnel mode
      5. Check setup_cloudflare_tunnel called
    Expected Result: Tunnel setup function invoked
    Evidence: .sisyphus/evidence/task-10-tunnel-route.log

  Scenario: Cloudflare Proxy mode routes correctly
    Tool: Bash
    Steps:
      1. Mock VPS with open ports
      2. Run setup-quick.sh
      3. Verify Proxy recommended
      4. Select Proxy mode
      5. Check setup_cloudflare_proxy called
    Expected Result: Proxy setup function invoked
    Evidence: .sisyphus/evidence/task-10-proxy-route.log
  ```

  **Commit**: YES
  - Message: `feat(deploy): integrate Cloudflare HTTPS into quick setup`
  - Files: `deploy/setup-quick.sh`

---

- [ ] 11. Create Standalone Cloudflare Setup Script

  **What to do**:
  - Create `deploy/setup-cloudflare.sh`
  - Source cloudflare-functions.sh
  - Implement standalone CLI interface
  - Support `--domain` flag for non-interactive use
  - Support `--mode tunnel|proxy` flag
  - Support `--dry-run` flag for testing
  - Add usage/help text

  **Must NOT do**:
  - Do NOT duplicate code from cloudflare-functions.sh
  - Do NOT require setup-quick.sh to run

  **Recommended Agent Profile**:
  - **Category**: `quick`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: NO (depends on tasks 1, 3, 4, 5)
  - **Parallel Group**: Wave 3
  - **Blocks**: F1, F2, F3
  - **Blocked By**: Tasks 1, 3, 4, 5

  **References**:
  - User answer 1: Both embedded and standalone scripts needed

  **Acceptance Criteria**:
  - [ ] Script can run independently
  - [ ] Sources cloudflare-functions.sh
  - [ ] `--domain` flag works
  - [ ] `--mode` flag works
  - [ ] `--dry-run` flag works
  - [ ] Help text clear
  - [ ] Bash syntax valid

  **QA Scenarios**:
  ```
  Scenario: Standalone script runs
    Tool: Bash
    Steps:
      1. bash deploy/setup-cloudflare.sh --help
      2. Check usage text displayed
    Expected Result: Help text shown
    Evidence: .sisyphus/evidence/task-11-standalone-help.log

  Scenario: --domain and --mode flags work
    Tool: Bash
    Steps:
      1. bash deploy/setup-cloudflare.sh --domain test.example.com --mode tunnel --dry-run
      2. Check domain used in config
    Expected Result: Domain set correctly
    Evidence: .sisyphus/evidence/task-11-domain-mode-flags.log
  ```

  **Commit**: YES
  - Message: `feat(deploy): add standalone Cloudflare HTTPS setup script`
  - Files: `deploy/setup-cloudflare.sh`

---

- [ ] 12. Add Edge Case Handlers

  **What to do**:
  - Implement NS check warning (non-blocking)
  - Add existing tunnel detection and reuse logic
  - Add authentication timeout handling with manual URL fallback
  - Add DNS propagation wait loop with timeout
  - Add tunnel/proxy service failure recovery
  - Add partial state cleanup on cancel
  - Handle cloudflared already installed
  - Handle existing Caddy/Nginx conflict

  **Must NOT do**:
  - Do NOT block setup on non-critical failures
  - Do NOT remove user's existing tunnels without confirmation

  **Recommended Agent Profile**:
  - **Category**: `unspecified-high`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: NO (depends on tasks 4, 5)
  - **Parallel Group**: Wave 3
  - **Blocks**: F3
  - **Blocked By**: Tasks 4, 5

  **References**:
  - User answer 3: Edge cases to handle

  **Acceptance Criteria**:
  - [ ] NS check warns but doesn't block
  - [ ] Existing tunnel reused correctly
  - [ ] Auth timeout provides manual URL
  - [ ] DNS propagation wait with timeout
  - [ ] Service failure shows logs
  - [ ] Cancel doesn't leave broken state
  - [ ] Existing web servers handled
  - [ ] Bash syntax valid

  **QA Scenarios**:
  ```
  Scenario: NS check warns for non-Cloudflare domain
    Tool: Bash
    Steps:
      1. Mock domain with non-CF nameservers
      2. Call check_cloudflare_nameservers
      3. Check warning displayed
    Expected Result: Warning shown, setup continues
    Evidence: .sisyphus/evidence/task-12-ns-check.log

  Scenario: Existing tunnel reused
    Tool: Bash
    Steps:
      1. Mock existing tunnel in cloudflared list
      2. Call setup_cloudflare_tunnel
      3. Verify no new tunnel created
    Expected Result: Existing tunnel ID used
    Evidence: .sisyphus/evidence/task-12-existing-tunnel.log

  Scenario: Auth timeout provides manual URL
    Tool: Bash
    Steps:
      1. Mock auth timeout
      2. Run setup script
      3. Verify manual URL provided
    Expected Result: Manual auth option shown
    Evidence: .sisyphus/evidence/task-12-auth-timeout.log
  ```

  **Commit**: YES
  - Message: `feat(deploy): add Cloudflare edge case handlers`
  - Files: `deploy/lib/cloudflare-functions.sh`

---

- [ ] 13. Create Dry-Run Testing Mode

  **What to do**:
  - Add `--dry-run` flag to setup-cloudflare-tunnel.sh
  - Add `--mode tunnel|proxy` flag to setup-cloudflare-proxy.sh
  - Support `--mode detection` flag for testing detection
  - Mock cloudflared responses
  - Mock Cloudflare API responses
  - Generate config files without execution
  - Validate all paths without side effects

  **Must NOT do**:
  - Do NOT require real Cloudflare account for dry-run
  - Do NOT create real tunnels

  **Recommended Agent Profile**:
  - **Category**: `quick`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: NO (depends on tasks 4, 5, 11)
  - **Parallel Group**: Wave 4
  - **Blocks**: Task 14
            - **Blocked By**: Tasks 4, 5, 11

  **References**:
  - User answer 5: Dry-run mode for for CI

  - Existing: `tests/` directory for test patterns

  - User clarification: Detection logic

          - **Category**: `quick`
          - **Skills**: []
            - `dry-run` is match TDD pattern
          - Reason: Simple validation, fast feedback

          - Reason: Testing in CI without external dependencies
          - `unspecified-high`: Complex logic, multiple edge cases

  **Acceptance Criteria**:
  - [ ] `--dry-run` flag recognized
  - [ ] Mock cloudflared binary created
    - [ ] Mock cloudflared responses generated
    - [ ] Config files generated without execution
    - [ ] Service files generated without execution
    - [ ] Bash syntax valid

  **QA Scenarios**:
  ```
  Scenario: Dry-run generates config without execution
    Tool: Bash
    Steps:
      1. bash deploy/setup-cloudflare-tunnel.sh --domain test.example.com --dry-run --      2. Read generated config
      3. Verify no real tunnel created
    Expected Result: Config exists, no tunnel
    Evidence: .sisyphus/evidence/task-13-dry-run.log

  Scenario: Proxy dry-run validates Caddy config
    Tool: Bash
    Steps:
      1. bash deploy/setup-cloudflare-tunnel.sh --domain test.example.com --dry-run --mode proxy
      2. Read generated Caddyfile
      3. Verify origin cert paths present
    Expected Result: Origin cert referenced, no Let's Encrypt
    Evidence: .sisyphus/evidence/task-13-proxy-dry-run.log
  ```

  **Commit**: YES
  - Message: `feat(deploy): add dry-run mode for Cloudflare setup`
  - Files: `deploy/setup-cloudflare-tunnel.sh`, `deploy/lib/cloudflare-functions.sh`

---

- [ ] 14. Write Integration tests

  **What to do**:
  - Create `tests/test-cloudflare-setup.sh`
  - Test library function loading
  - Test network detection in mocked environment
  - Test prompt with recommendation
  - Test Tunnel mode in mocked environment
  - Test Proxy mode in mocked environment
  - Test mode routing in main()
  - Test integration of modes
  - Add to CI pipeline

  - Use bash test runner for automation

  - Test with `--tap` flag for interactive mode selection
  - Mock cloudflared via expect + jq wrapper
  - Run: `bash tests/test-cloudflare-setup.sh`

  **Must NOT do**:
  - Do NOT require real Cloudflare account
  - Do NOT create real tunnels
  - Do NOT require external API calls (mock all)
  - Do not block setup if detection fails

  **Recommended Agent Profile**:
  - **Category**: `quick`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: NO (depends on task 13)
  - **Parallel Group**: Wave 4
  - **Blocks**: Task 14
            - **Blocked by**: Task 13

  **References**:
  - User answer 5: Test matrix with detection,
  - Existing: `tests/` for test patterns
  - User clarification: Detection logic for testing

          - **category**: `quick`
          - **Skills**: []
            - `detect_network_environment()`: Quick to validate
            - `prompt_cloudflare_mode()`: More complex, involves user interaction
            - `detect_network_environment()`: Simple validation of environment
            - `setup_cloudflare_tunnel()`: cloudflared binary + auth + tunnel + DNS
            - `setup_cloudflare_proxy()`: Cloudflare API for A record + origin cert + Caddy config
            - Mock `cloudflared` for tunnel testing
            - Use `curl` to mock Cloudflare API responses
            - Validate generated config files
          - **Category**: `quick`
          - **Skills**: []
            - Testing is layered - unit tests for functions, then integration
            - Detection logic for environment testing
            - Recommendation prompt for smart suggestions
          - **Category**: `quick`
          - **Skills**: []
            - Test prompt displays recommendation with reasoning
            - Test prompt rejects invalid input
            - Test tunnel mode with mocked cloudflared
            - Test proxy mode with mocked cloudflare API
            - Test mode routing (tunnel vs proxy vs manual)
          - **category**: `quick`
          - **Skills**: []
            - Test with `--tap` to flag for interactive testing
          - **Category**: `quick`
          - **Skills**: []
            - Test config file generation
            - Test service file generation
            - Test Caddy config generation
            - Test DNS record creation
            - Test edge case handling
          - **Category**: `quick`
          - **Skills**: []
            - `source deploy/lib/cloudflare-functions.sh`
            - Run with `--dry-run` to skip actual execution
            - Run with `--tap` for interactive mode selection
            - Verify mode routing works

            - Test detection logic in mocked NAT environment
            - Test detection logic in mocked VPS environment
            - Verify recommendation is correct
          - **Category**: `quick`
          - **Skills**: []
            - Test that prompt shows recommendation based on detected environment
            - Test that prompt accepts valid input
            - Test that mode routing works correctly

  **Acceptance Criteria**:
  - [ ] All functions tested
  - [ ] Mock cloudflared responses generated
  - [ ] Config files generated without execution
  - [ ] Service files generated without execution
  - [ ] Bash syntax valid

  **QA Scenarios**:
  ```
  Scenario: Test suite runs successfully
    Tool: Bash
    Steps:
      1. bash tests/test-cloudflare-setup.sh
      2. Check all tests pass
    Expected Result: All tests pass
    Evidence: .sisyphus/evidence/task-14-tests-pass.log
  ```

  **Commit**: YES
  - Message: `test(deploy): add Cloudflare setup integration tests`
  - Files: `tests/test-cloudflare-setup.sh`

---

- [ ] 15. Update documentation

  **What to do**:
  - Update `README.md` with Cloudflare Tunnel option
  - Add to Deployment Modes section
  - Document new environment variables
  - Add troubleshooting section
  - Update `docs/index.md` (if tracked)

  - Add examples for manual and automated setup

  **Must NOT do**:
  - Do NOT remove existing documentation
  - Do NOT assume users know Cloudflare
  - Do NOT duplicate existing setup documentation

  **Recommended Agent Profile**:
  - **Category**: `quick`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES (with task 13)
  - **Parallel Group**: Wave 4
  - **Blocks**: None
  - **Blocked By**: Tasks 10, 11

  **References**:
  - `README.md` Deployment Modes section
  - `docs/index.md` Production Installer section

  **Acceptance Criteria**:
  - [ ] README.md updated with Cloudflare Tunnel option
  - [ ] Environment variables documented
  - [ ] Troubleshooting guide added
  - [ ] Examples provided
  - [ ] Bash syntax valid

  **QA Scenarios**:
  ```
  Scenario: Documentation mentions Cloudflare Tunnel
    Tool: Bash
    Steps:
      1. grep -i "cloudflare" README.md
      2. grep -i "option 6" README.md || grep -i "cloudflare proxy" README.md
    Expected Result: Both patterns found
    Evidence: .sisyphus/evidence/task-15-docs.log
  ```

  **Commit**: YES
  - Message: `docs: add Cloudflare HTTPS documentation`
  - Files: `README.md`

---

- [ ] F1. Tunnel E2E Test

  **What to do**: Test complete Tunnel flow on fresh VPS
  - Run `setup-quick.sh`
  - Select Cloudflare Tunnel mode (Option 6)
  - Enter test domain
  - Complete auth
  - Verify tunnel created
  - Verify DNS records (armorclaw.example.com, matrix.armorclaw.example.com)
  - Verify service running
  - Verify HTTPS access

  - Capture full output log

  - Save evidence to to-tracing files

  **Must NOT do**:
  - Do NOT modify existing test patterns
  - Do NOT skip cleanup on incomplete test

  - Do not proceed if service fails

  - Do not assume manual steps will work

  **Recommended Agent Profile**:
  - **Category**: `deep`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: NO (must on all implementation tasks)
  - **Parallel Group**: Wave FINAL
  - **Blocks**: None (until user approval)
  - **Blocked By**: Tasks 10, 11, 14

  **References**:
  - This plan: Task descriptions
  - Existing: `deploy/setup-quick.sh`
  - User clarification: Tunnel mode flow

  **Acceptance Criteria**:
  - [ ] Full setup completes on
  - [ ] Tunnel created successfully
  - [ ] DNS records correct (main + matrix subdomains)
  - [ ] Service running and accessible
            - [ ] HTTPS works via tunnel URL
  - [ ] Logs captured to no errors
  - [ ] Evidence saved to `.sisyphus/evidence/f1-tunnel-e2e.log`

  **QA Scenarios**:
  ```
  Scenario: Fresh install with Cloudflare Tunnel
    Tool: Bash
    Steps:
      1. Start fresh VPS (no cloudflared)
      2. Run setup-quick.sh
      3. Select Option 6
      4. Enter test domain
      5. Complete auth
      6. Verify tunnel created
      7. Verify DNS records
      8. Verify service running
      9. Verify HTTPS access via tunnel URL
    Expected Result: Full setup completes successfully
    Evidence: .sisyphus/evidence/f1-fresh-install.log
  ```

  **Commit**: YES (  - Message: `test(e2e): Tunnel E2E test complete`
  - Files: `.sisyphus/evidence/f1-tunnel-e2e.log`

---

- [ ] F2. Proxy E2E Test

  **What to do**: Test complete Proxy flow on VPS with open ports
  - Run `setup-quick.sh`
  - Select Cloudflare Proxy mode (Option 7)
  - Enter test domain
  - Verify A record created with proxy
  - Verify origin cert generated
  - Verify Caddy configured
  - Verify HTTPS access via domain
  - Capture full output log
  - Save evidence to to-tracing files

  **Must NOT do**:
  - Do NOT modify existing test patterns
  - Do not skip cleanup on incomplete test
  - Do not proceed if ports are blocked

  - Do not assume manual steps will work

  **Recommended Agent Profile**:
  - **Category**: `deep`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: NO (depends on all implementation tasks)
  - **Parallel Group**: Wave FINAL
  - **Blocks**: User approval
            - **Blocked By**: Tasks 10, 11, 14

  **References**:
  - This plan: Task descriptions
  - User clarification: Proxy mode flow

  **Acceptance Criteria**:
  - [ ] Full setup completes successfully
  - [ ] A record created with proxy enabled
  - [ ] Origin cert generated and installed
  - [ ] Caddy configured with origin cert
            - [ ] Service running and accessible
            - [ ] HTTPS works via domain
  - [ ] Logs captured, no errors
  - [ ] Evidence saved to `.sisyphus/evidence/f2-proxy-e2e.log`

  **QA Scenarios**:
  ```
  Scenario: Fresh install with Cloudflare Proxy
    Tool: Bash
    Steps:
      1. Start fresh VPS (open ports)
      2. Run setup-quick.sh
      3. Select option 7
      4. Enter test domain
      5. Verify A record created
      6. Verify origin cert generated
      7. Verify Caddy configured
      8. Verify service running
      9. Verify HTTPS access via domain
    Expected Result: Full setup completes successfully
    Evidence: .sisyphus/evidence/f2-proxy-e2e.log`
  ```

  **Commit**: YES
  - Message: `test(e2e): Proxy E2E test complete`
  - Files: `.sisyphus/evidence/f2-proxy-e2e.log`

---

- [ ] F3. Detection + Recommendation Test

  **What to do**: Test detection logic in multiple environments
  - Test NAT environment detection (home network)
  - Test VPS environment detection (public IP, open ports)
  - Test mixed environment detection (some ports blocked)
  - Verify recommendation is correct for each environment
  - Test prompt displays correct recommendation with reasoning
  - Test user override works
  - Capture full output log
  - Save evidence to to-tracing files

  **Must NOT do**:
  - Do NOT modify existing test patterns
  - Do not skip cleanup on incomplete test
  - Do not assume manual steps will work
  - Do not proceed if detection fails (should warn and continue)

  **Recommended Agent Profile**:
  - **Category**: `unspecified-high`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: NO (depends on tasks 10, 11, 12)
  - **Parallel Group**: Wave FINAL
  - **Blocks**: User approval
            - **Blocked By**: Tasks 10, 11, 12

  **References**:
  - This plan: Task descriptions
  - User clarification: Detection logic
          - **Category**: `unspecified-high`
          - **Skills**: []
            - Mock `/proc/net/route` and `ip` command for deterministic results
            - Mock `curl` to api.ipify.org for deterministic results
            - Test different scenarios and verify recommendations
          - **category**: `quick`
          - **Skills**: []
            - Test each detection scenario
            - Test recommendation matches environment
            - Verify reasoning is displayed
          - **category**: `quick`
          - **Skills**: []
            - Test prompt with detection results
            - Verify recommendation is displayed with reasoning
            - Test user override works
          - **category**: `quick`
          - **Skills**: []
            - Select different option than recommended
            - Verify selection is respected
          - **category**: `quick`
          - **Skills**: []
            - Select each option
            - Verify correct function is called
            - Verify variables are exported
  **Acceptance Criteria**:
  - [ ] All three environments detected correctly
  - [ ] Recommendations match environment (tunnel for NAT, proxy for VPS)
  - [ ] Reasoning displayed with recommendation
  - [ ] User override works (select different option)
  - [ ] Bash syntax valid

  **QA Scenarios**:
  ```
  Scenario: Detection identifies NAT environment
    Tool: Bash
    Steps:
      1. Mock NAT environment (public IP != local IP)
      2. Call detect_network_environment
      3. Verify RECOMMEND="tunnel"
      4. Verify REASON contains "NAT"
    Expected Result: Tunnel recommended for NAT
    Evidence: .sisyphus/evidence/f3-nat-detection.log

  Scenario: Detection identifies VPS with open ports
    Tool: Bash
    Steps:
      1. Mock VPS environment (public IP = local IP, ports open)
      2. Call detect_network_environment
      3. Verify RECOMMEND="proxy"
      4. Verify REASON contains "public VPS"
    Expected Result: Proxy recommended for VPS
    Evidence: .sisyphus/evidence/f3-vps-detection.log

  Scenario: Prompt shows recommendation with reasoning
    Tool: Bash
    Steps:
      1. Call detect_network_environment
      2. Call prompt_cloudflare_mode
      3. Check output contains "Recommended" and reasoning from detection
    Expected Result: Recommendation displayed with reasoning
    Evidence: .sisyphus/evidence/f3-prompt-recommendation.log
  ```

  **Commit**: YES
  - Message: `test(detection): add detection and recommendation tests`
  - Files: `.sisyphus/evidence/f3-detection-tests.log`

- [ ] F1. **Tunnel E2E Test** — `deep`
  Test complete Tunnel flow on fresh VPS: detection → selection → tunnel creation → DNS → HTTPS verification

- [ ] F2. **Proxy E2E Test** — `deep`
  Test complete Proxy flow on VPS with open ports: detection → selection → A record → origin cert → Caddy → HTTPS verification

- [ ] F3. **Detection + Recommendation Test** — `unspecified-high`
  Test detection logic in multiple environments (NAT, open ports, mixed) and verify correct recommendations

---

## Commit Strategy

| Wave | Commit Pattern |
|------|----------------|
| 1 | `feat(deploy): add Cloudflare [component]` |
| 2 | `feat(deploy): add Cloudflare Proxy mode` |
| 3 | `feat(deploy): integrate Cloudflare HTTPS setup` |
| 4 | `test(deploy): add Cloudflare setup tests` |
| FINAL | `docs: update Cloudflare HTTPS documentation` |

---

## Success Criteria

### Verification Commands

```bash
# Test library loading
source deploy/lib/cloudflare-functions.sh && type setup_cloudflare_tunnel && type setup_cloudflare_proxy

# Test network detection
source deploy/lib/cloudflare-functions.sh && detect_network_environment && echo $RECOMMEND

# Test interactive prompt
echo "1" | bash deploy/setup-quick.sh --dry-run

# Test standalone script
bash deploy/setup-cloudflare.sh --help

# Run integration tests
bash tests/test-cloudflare-setup.sh
```

### Final Checklist

- [ ] All "Must Have" present
- [ ] All "Must NOT Have" absent
- [ ] Network detection works correctly
- [ ] Both Tunnel and Proxy modes work
- [ ] Smart recommendation displays
- [ ] E2E tests pass
- [ ] Documentation updated
