# Cloudflare Tunnel Integration Plan

> **Objective**: Add optional Cloudflare Tunnel setup to `setup-quick.sh` interactive flow, enabling permanent HTTPS access with custom domains for production VPS deployments.
>
> **Design Principle**: Cloudflare Tunnel is one of multiple access modes, not a replacement for existing options (native, IP-only, quick tunnel, ngrok).

---

## TL;DR

> **Quick Summary**: Extend setup-quick.sh with interactive access mode selection, implementing Cloudflare Tunnel as the recommended production option with automatic DNS configuration and systemd service management.
>
> **Deliverables**:
> - Interactive access mode prompt (6 options)
> - Cloudflare Tunnel setup function
> - Manual domain setup function
> - Standalone setup-cloudflare-tunnel.sh script
> - Shared library for tunnel functions
> - Dry-run testing mode
>
> **Estimated Effort**: Medium
> **Parallel Execution**: YES - 3 waves
> **Critical Path**: Functions → Integration → Testing

---

## Gap Analysis

### Current State (`deploy/setup-quick.sh`)

**Existing Access Modes:**
- Native mode (Unix socket, local only)
- IP-only mode (HTTP, no encryption)
- Quick Tunnel (temporary Cloudflare URL)
- ngrok (temporary URL, requires account)

**Current Flow:**
```
Pre-flight checks → Mode detection → Matrix setup → Bridge setup → Summary
```

**Missing:**
- No Cloudflare Tunnel (permanent custom domain) option
- No manual DNS configuration option
- No interactive prompt for access mode selection

### Required Changes

| Gap | Severity | Description |
|-----|----------|-------------|
| **G1** | CRITICAL | No interactive access mode prompt |
| **G2** | CRITICAL | No Cloudflare Tunnel setup function |
| **G3** | HIGH | No manual domain setup option |
| **G4** | MEDIUM | No standalone tunnel setup script |
| **G5** | MEDIUM | No shared function library |
| **G6** | MEDIUM | No dry-run testing mode |
| **G7** | LOW | No edge case handling (NS check, existing tunnel, etc.) |

---

## Work Objectives

### Core Objective

Implement Cloudflare Tunnel as a first-class access mode option in the quick setup flow, enabling:
1. Interactive access mode selection
2. Automated Cloudflare Tunnel setup with DNS
3. Manual domain configuration fallback
4. Production-ready HTTPS deployment

### Concrete Deliverables

- `deploy/setup-quick.sh` - Updated with interactive prompt
- `deploy/setup-cloudflare-tunnel.sh` - Standalone tunnel setup
- `deploy/lib/cloudflare-functions.sh` - Shared tunnel functions
- `tests/test-cloudflare-tunnel.sh` - Dry-run and integration tests

### Definition of Done

- [ ] User can select from 6 access modes in interactive prompt
- [ ] Cloudflare Tunnel mode installs cloudflared and creates tunnel
- [ ] DNS records automatically configured for domain and matrix subdomain
- [ ] Systemd service created and started
- [ ] Manual domain mode provides clear DNS instructions
- [ ] Existing modes (native, IP, quick tunnel, ngrok) still work
- [ ] Dry-run mode works for CI testing
- [ ] Edge cases handled (NS check, existing tunnel, auth timeout)

### Must Have

- Interactive access mode prompt
- Cloudflare Tunnel setup function
- cloudflared binary installation
- Tunnel creation and DNS routing
- Systemd service installation
- Manual domain fallback

### Must NOT Have (Guardrails)

- NO breaking changes to existing setup modes
- NO requirement for Cloudflare account (optional)
- NO removal of Quick Tunnel option
- NO automatic domain registration (user must own domain)

---

## Verification Strategy

### Test Decision

- **Infrastructure exists**: YES (existing test scripts)
- **Automated tests**: YES - Dry-run mode + integration tests
- **Framework**: bash test scripts
- **Agent-Executed QA**: YES

### QA Policy

Every task includes agent-executed QA scenarios:
- **CLI/Installer**: Bash execution with output validation
- **Tunnel setup**: Mock cloudflared for CI, real testing with test domain

---

## Execution Strategy

### Parallel Execution Waves

```
Wave 1 (Core Functions):
├── Task 1: Create shared function library [quick]
├── Task 2: Implement prompt_domain_config() [quick]
├── Task 3: Implement setup_cloudflare_tunnel() [unspecified-high]
└── Task 4: Implement setup_manual_domain() [quick]

Wave 2 (Integration):
├── Task 5: Update setup-quick.sh main() [quick]
├── Task 6: Create standalone setup script [quick]
└── Task 7: Add edge case handlers [unspecified-high]

Wave 3 (Testing):
├── Task 8: Create dry-run mode [quick]
├── Task 9: Write integration tests [quick]
└── Task 10: Update documentation [quick]

Wave FINAL (Verification):
├── Task F1: End-to-end fresh install test [deep]
├── Task F2: Existing tunnel reuse test [quick]
└── Task F3: Edge case validation [unspecified-high]
```

### Dependency Matrix

| Task | Depends On | Blocks |
|------|------------|--------|
| 1 | - | 2, 3, 4, 6 |
| 2 | 1 | 5 |
| 3 | 1 | 5, 7 |
| 4 | 1 | 5 |
| 5 | 2, 3, 4 | F1 |
| 6 | 1, 3 | F1 |
| 7 | 3 | F3 |
| 8 | 3, 6 | 9 |
| 9 | 8 | F1, F2, F3 |
| 10 | 5, 6 | - |

---

## TODOs

- [ ] 1. Create Shared Function Library

  **What to do**:
  - Create `deploy/lib/cloudflare-functions.sh`
  - Implement `check_cloudflare_prerequisites()`
  - Implement `install_cloudflared()`
  - Implement `check_cloudflare_nameservers()`
  - Add error handling utilities

  **Must NOT do**:
  - Do NOT modify existing setup scripts yet
  - Do NOT add Cloudflare-specific logic to main flow

  **Recommended Agent Profile**:
  - **Category**: `quick`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: NO (blocks tasks 2-4, 6)
  - **Parallel Group**: Wave 1
  - **Blocks**: Tasks 2, 3, 4, 6
  - **Blocked By**: None

  **References**:
  - User spec section 6: `check_cloudflare_prerequisites()`
  - User spec section 4: `install_cloudflared()` logic

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
    Expected Result: Function is defined
    Evidence: .sisyphus/evidence/task-1-library-load.log

  Scenario: cloudflared installation function works
    Tool: Bash
    Steps:
      1. Mock /tmp/cloudflared binary
      2. Call install_cloudflared
      3. Verify /usr/local/bin/cloudflared exists
    Expected Result: Binary installed
    Evidence: .sisyphus/evidence/task-1-install.log
  ```

  **Commit**: YES
  - Message: `feat(deploy): add Cloudflare shared function library`
  - Files: `deploy/lib/cloudflare-functions.sh`

---

- [ ] 2. Implement Interactive Access Mode Prompt

  **What to do**:
  - Create `prompt_domain_config()` function
  - Display 6 access mode options with descriptions
  - Validate user input (1-6)
  - Export `ACCESS_MODE` variable
  - Add to cloudflare-functions.sh

  **Must NOT do**:
  - Do NOT integrate into main() yet (Task 5)
  - Do NOT call tunnel setup functions yet

  **Recommended Agent Profile**:
  - **Category**: `quick`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: NO (depends on task 1)
  - **Parallel Group**: Wave 1
  - **Blocks**: Task 5
  - **Blocked By**: Task 1

  **References**:
  - User spec section 1: `prompt_domain_config()` implementation

  **Acceptance Criteria**:
  - [ ] Function displays all 6 options
  - [ ] Input validation accepts 1-6 only
  - [ ] ACCESS_MODE exported correctly
  - [ ] Bash syntax valid

  **QA Scenarios**:
  ```
  Scenario: Prompt accepts valid input
    Tool: Bash
    Steps:
      1. echo "6" | source deploy/lib/cloudflare-functions.sh && prompt_domain_config
      2. echo $ACCESS_MODE
    Expected Result: ACCESS_MODE=cloudflare_tunnel
    Evidence: .sisyphus/evidence/task-2-prompt-valid.log

  Scenario: Prompt rejects invalid input
    Tool: Bash
    Steps:
      1. echo -e "7\n6" | prompt_domain_config
    Expected Result: Re-prompts on invalid input
    Evidence: .sisyphus/evidence/task-2-prompt-invalid.log
  ```

  **Commit**: YES
  - Message: `feat(deploy): add interactive access mode prompt`
  - Files: `deploy/lib/cloudflare-functions.sh`

---

- [ ] 3. Implement Cloudflare Tunnel Setup Function

  **What to do**:
  - Create `setup_cloudflare_tunnel()` function
  - Check if cloudflared installed, install if needed
  - Prompt for domain input
  - Extract subdomain and base domain
  - Check existing authentication
  - Trigger `cloudflared tunnel login` if needed
  - Create or reuse tunnel
  - Generate config.yml with ingress rules
  - Create DNS records via tunnel route dns
  - Install as systemd service
  - Export PUBLIC_URL, MATRIX_URL, DOMAIN

  **Must NOT do**:
  - Do NOT integrate into main() yet
  - Do NOT remove existing tunnel options

  **Recommended Agent Profile**:
  - **Category**: `unspecified-high`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: NO (depends on task 1)
  - **Parallel Group**: Wave 1
  - **Blocks**: Tasks 5, 7
  - **Blocked By**: Task 1

  **References**:
  - User spec section 2: `setup_cloudflare_tunnel()` implementation
  - User spec section 5: Caddyfile template considerations

  **Acceptance Criteria**:
  - [ ] Function handles fresh cloudflared install
  - [ ] Function handles existing cloudflared
  - [ ] Authentication flow works
  - [ ] Tunnel created or reused
  - [ ] Config generated correctly
  - [ ] DNS records created
  - [ ] Systemd service installed
  - [ ] Variables exported for downstream use

  **QA Scenarios**:
  ```
  Scenario: Fresh cloudflared install
    Tool: Bash
    Steps:
      1. Mock cloudflared not installed
      2. Call setup_cloudflare_tunnel with test domain
      3. Verify binary downloaded and installed
    Expected Result: cloudflared in /usr/local/bin
    Evidence: .sisyphus/evidence/task-3-fresh-install.log

  Scenario: Existing tunnel reuse
    Tool: Bash
    Steps:
      1. Mock existing tunnel "armorclaw-test"
      2. Call setup_cloudflare_tunnel
      3. Verify tunnel reused, not recreated
    Expected Result: Same tunnel ID
    Evidence: .sisyphus/evidence/task-3-existing-tunnel.log

  Scenario: Config file generated correctly
    Tool: Bash
    Steps:
      1. Call setup_cloudflare_tunnel with "test.example.com"
      2. Read ~/.cloudflared/config.yml
      3. Verify ingress rules for both domains
    Expected Result: Config contains test.example.com and matrix.test.example.com
    Evidence: .sisyphus/evidence/task-3-config-gen.log
  ```

  **Commit**: YES
  - Message: `feat(deploy): implement Cloudflare Tunnel setup function`
  - Files: `deploy/lib/cloudflare-functions.sh`

---

- [ ] 4. Implement Manual Domain Setup Function

  **What to do**:
  - Create `setup_manual_domain()` function
  - Prompt for domain input
  - Detect public IP via api.ipify.org
  - Display DNS configuration instructions
  - Wait for user confirmation
  - Export PUBLIC_URL, MATRIX_URL, DOMAIN

  **Must NOT do**:
  - Do NOT create DNS records automatically
  - Do NOT verify DNS propagation (manual verification)

  **Recommended Agent Profile**:
  - **Category**: `quick`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: NO (depends on task 1)
  - **Parallel Group**: Wave 1
  - **Blocks**: Task 5
  - **Blocked By**: Task 1

  **References**:
  - User spec section 3: `setup_manual_domain()` implementation

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
    Evidence: .sisyphus/evidence/task-4-manual-domain.log
  ```

  **Commit**: YES
  - Message: `feat(deploy): add manual domain setup option`
  - Files: `deploy/lib/cloudflare-functions.sh`

---

- [ ] 5. Integrate Access Mode Selection into setup-quick.sh

  **What to do**:
  - Update `main()` in setup-quick.sh
  - Call `prompt_domain_config()` after pre-flight checks
  - Add case statement for ACCESS_MODE
  - Route to appropriate setup function
  - Ensure existing modes still work

  **Must NOT do**:
  - Do NOT break existing setup flow
  - Do NOT remove existing access modes

  **Recommended Agent Profile**:
  - **Category**: `quick`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: NO (depends on tasks 2, 3, 4)
  - **Parallel Group**: Wave 2
  - **Blocks**: F1
  - **Blocked By**: Tasks 2, 3, 4

  **References**:
  - User spec section 4: Updated main() flow
  - Existing setup-quick.sh structure

  **Acceptance Criteria**:
  - [ ] prompt_domain_config() called in main()
  - [ ] All 6 modes route correctly
  - [ ] Existing modes (1-4) unchanged
  - [ ] New modes (5-6) work
  - [ ] Bash syntax valid

  **QA Scenarios**:
  ```
  Scenario: Native mode still works
    Tool: Bash
    Steps:
      1. echo "1" | bash deploy/setup-quick.sh --dry-run
      2. Check ACCESS_MODE=native
    Expected Result: Native mode configured
    Evidence: .sisyphus/evidence/task-5-native-mode.log

  Scenario: Cloudflare Tunnel mode routes correctly
    Tool: Bash
    Steps:
      1. echo "6" | bash deploy/setup-quick.sh --dry-run
      2. Check setup_cloudflare_tunnel called
    Expected Result: Tunnel setup function invoked
    Evidence: .sisyphus/evidence/task-5-tunnel-route.log
  ```

  **Commit**: YES
  - Message: `feat(deploy): integrate access mode selection into quick setup`
  - Files: `deploy/setup-quick.sh`

---

- [ ] 6. Create Standalone Cloudflare Tunnel Setup Script

  **What to do**:
  - Create `deploy/setup-cloudflare-tunnel.sh`
  - Source cloudflare-functions.sh
  - Implement standalone CLI interface
  - Support --domain flag for non-interactive use
  - Support --dry-run flag for testing
  - Add usage/help text

  **Must NOT do**:
  - Do NOT duplicate code from cloudflare-functions.sh
  - Do NOT require setup-quick.sh to run

  **Recommended Agent Profile**:
  - **Category**: `quick`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: NO (depends on tasks 1, 3)
  - **Parallel Group**: Wave 2
  - **Blocks**: F1
  - **Blocked By**: Tasks 1, 3

  **References**:
  - User spec section 2: Core tunnel setup logic
  - User answer 1: Both embedded and standalone scripts needed

  **Acceptance Criteria**:
  - [ ] Script can run independently
  - [ ] Sources cloudflare-functions.sh
  - [ ] --domain flag works
  - [ ] --dry-run flag works
  - [ ] Help text clear
  - [ ] Bash syntax valid

  **QA Scenarios**:
  ```
  Scenario: Standalone script runs
    Tool: Bash
    Steps:
      1. bash deploy/setup-cloudflare-tunnel.sh --help
      2. Check usage text displayed
    Expected Result: Help text shown
    Evidence: .sisyphus/evidence/task-6-standalone-help.log

  Scenario: --domain flag works
    Tool: Bash
    Steps:
      1. bash deploy/setup-cloudflare-tunnel.sh --domain test.example.com --dry-run
      2. Check domain used in config
    Expected Result: Domain set correctly
    Evidence: .sisyphus/evidence/task-6-domain-flag.log
  ```

  **Commit**: YES
  - Message: `feat(deploy): add standalone Cloudflare Tunnel setup script`
  - Files: `deploy/setup-cloudflare-tunnel.sh`

---

- [ ] 7. Add Edge Case Handlers

  **What to do**:
  - Implement `check_cloudflare_nameservers()` function
  - Add existing tunnel detection and reuse logic
  - Add authentication timeout handling
  - Add DNS propagation wait loop
  - Add tunnel service failure recovery
  - Add partial state cleanup on cancel

  **Must NOT do**:
  - Do NOT block setup on non-critical failures
  - Do NOT remove user's existing tunnels

  **Recommended Agent Profile**:
  - **Category**: `unspecified-high`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: NO (depends on task 3)
  - **Parallel Group**: Wave 2
  - **Blocks**: F3
  - **Blocked By**: Task 3

  **References**:
  - User spec section 6: `check_cloudflare_prerequisites()`
  - User answer 3: Edge cases to handle

  **Acceptance Criteria**:
  - [ ] NS check warns but doesn't block
  - [ ] Existing tunnel reused correctly
  - [ ] Auth timeout provides manual URL
  - [ ] DNS propagation wait with timeout
  - [ ] Service failure shows logs
  - [ ] Cancel doesn't leave broken state
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
    Evidence: .sisyphus/evidence/task-7-ns-check.log

  Scenario: Existing tunnel reused
    Tool: Bash
    Steps:
      1. Mock existing tunnel in cloudflared list
      2. Call setup_cloudflare_tunnel
      3. Verify no new tunnel created
    Expected Result: Existing tunnel ID used
    Evidence: .sisyphus/evidence/task-7-existing-tunnel.log
  ```

  **Commit**: YES
  - Message: `feat(deploy): add Cloudflare Tunnel edge case handlers`
  - Files: `deploy/lib/cloudflare-functions.sh`

---

- [ ] 8. Create Dry-Run Testing Mode

  **What to do**:
  - Add `--dry-run` flag to setup-cloudflare-tunnel.sh
  - Mock cloudflared binary responses
  - Skip actual tunnel creation
  - Generate config files without execution
  - Validate all paths without side effects

  **Must NOT do**:
  - Do NOT require real Cloudflare account for dry-run
  - Do NOT create real tunnels in dry-run mode

  **Recommended Agent Profile**:
  - **Category**: `quick`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: NO (depends on tasks 3, 6)
  - **Parallel Group**: Wave 3
  - **Blocks**: Task 9
  - **Blocked By**: Tasks 3, 6

  **References**:
  - User answer 5: Dry-run mode for CI

  **Acceptance Criteria**:
  - [ ] --dry-run flag recognized
  - [ ] No actual cloudflared commands executed
  - [ ] Config files generated
  - [ ] Service files generated
  - [ ] Bash syntax valid

  **QA Scenarios**:
  ```
  Scenario: Dry-run generates config without execution
    Tool: Bash
    Steps:
      1. bash deploy/setup-cloudflare-tunnel.sh --domain test.example.com --dry-run
      2. Check ~/.cloudflared/config.yml exists
      3. Verify no real tunnel created
    Expected Result: Config exists, no tunnel
    Evidence: .sisyphus/evidence/task-8-dry-run.log
  ```

  **Commit**: YES
  - Message: `feat(deploy): add dry-run mode for Cloudflare Tunnel setup`
  - Files: `deploy/setup-cloudflare-tunnel.sh`, `deploy/lib/cloudflare-functions.sh`

---

- [ ] 9. Write Integration Tests

  **What to do**:
  - Create `tests/test-cloudflare-tunnel.sh`
  - Test library function loading
  - Test prompt_domain_config() input validation
  - Test setup_cloudflare_tunnel() with mocks
  - Test setup_manual_domain() output
  - Test edge cases (NS check, existing tunnel)
  - Add to CI pipeline

  **Must NOT do**:
  - Do NOT require real Cloudflare account
  - Do NOT create real tunnels

  **Recommended Agent Profile**:
  - **Category**: `quick`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: NO (depends on task 8)
  - **Parallel Group**: Wave 3
  - **Blocks**: F1, F2, F3
  - **Blocked By**: Task 8

  **References**:
  - User answer 5: Test matrix

  **Acceptance Criteria**:
  - [ ] Test file created
  - [ ] All functions tested
  - [ ] Mock cloudflared responses
  - [ ] CI-friendly (no external dependencies)
  - [ ] Bash syntax valid

  **QA Scenarios**:
  ```
  Scenario: Test suite runs successfully
    Tool: Bash
    Steps:
      1. bash tests/test-cloudflare-tunnel.sh
      2. Check all tests pass
    Expected Result: All tests pass
    Evidence: .sisyphus/evidence/task-9-tests-pass.log
  ```

  **Commit**: YES
  - Message: `test(deploy): add Cloudflare Tunnel integration tests`
  - Files: `tests/test-cloudflare-tunnel.sh`

---

- [ ] 10. Update Documentation

  **What to do**:
  - Update README.md with Cloudflare Tunnel option
  - Add to Deployment Modes section
  - Document new environment variables
  - Add troubleshooting section
  - Update docs/index.md (if tracked)

  **Must NOT do**:
  - Do NOT remove existing documentation
  - Do NOT assume users know Cloudflare

  **Recommended Agent Profile**:
  - **Category**: `quick`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES (with task 8)
  - **Parallel Group**: Wave 3
  - **Blocks**: None
  - **Blocked By**: Tasks 5, 6

  **References**:
  - README.md Deployment Modes section
  - docs/index.md Production Installer section

  **Acceptance Criteria**:
  - [ ] README.md updated with Option 6
  - [ ] Environment variables documented
  - [ ] Troubleshooting guide added
  - [ ] Examples provided

  **QA Scenarios**:
  ```
  Scenario: Documentation mentions Cloudflare Tunnel
    Tool: Bash
    Steps:
      1. grep -i "cloudflare tunnel" README.md
      2. grep -i "option 6" README.md
    Expected Result: Both patterns found
    Evidence: .sisyphus/evidence/task-10-docs.log
  ```

  **Commit**: YES
  - Message: `docs: add Cloudflare Tunnel documentation`
  - Files: `README.md`

---

## Final Verification Wave

- [ ] F1. End-to-End Fresh Install Test

  **What to do**: Run full setup-quick.sh with Cloudflare Tunnel option on fresh VPS

  **Recommended Agent Profile**: `deep`

  **QA Scenarios**:
  ```
  Scenario: Fresh install with Cloudflare Tunnel
    Tool: Bash
    Steps:
      1. Start with fresh VPS (no cloudflared)
      2. Run setup-quick.sh
      3. Select Option 6
      4. Enter test domain
      5. Complete auth
      6. Verify tunnel created
      7. Verify DNS records
      8. Verify service running
      9. Verify HTTPS access
    Expected Result: Full setup completes successfully
    Evidence: .sisyphus/evidence/f1-fresh-install.log
  ```

- [ ] F2. Existing Tunnel Reuse Test

  **What to do**: Run setup with existing tunnel, verify reuse

  **Recommended Agent Profile**: `quick`

  **QA Scenarios**:
  ```
  Scenario: Existing tunnel reused correctly
    Tool: Bash
    Steps:
      1. Create tunnel manually
      2. Run setup-quick.sh
      3. Select Option 6
      4. Enter same domain
      5. Verify tunnel reused
      6. Verify config updated
    Expected Result: Tunnel reused, not duplicated
    Evidence: .sisyphus/evidence/f2-existing-tunnel.log
  ```

- [ ] F3. Edge Case Validation

  **What to do**: Test all edge cases (NS check, auth timeout, DNS delay, service fail)

  **Recommended Agent Profile**: `unspecified-high`

  **QA Scenarios**:
  ```
  Scenario: Non-Cloudflare nameserver handled
    Tool: Bash
    Steps:
      1. Use domain with non-CF nameservers
      2. Run setup-quick.sh
      3. Select Option 6
      4. Verify warning shown
      5. Verify setup continues
    Expected Result: Warning shown, setup completes
    Evidence: .sisyphus/evidence/f3-ns-warning.log

  Scenario: Authentication timeout recovered
    Tool: Bash
    Steps:
      1. Mock auth timeout
      2. Verify manual URL provided
      3. Complete auth manually
      4. Verify setup continues
    Expected Result: Manual auth option works
    Evidence: .sisyphus/evidence/f3-auth-timeout.log
  ```

---

## Commit Strategy

| Wave | Commit Pattern |
|------|----------------|
| 1 | `feat(deploy): add Cloudflare [component]` |
| 2 | `feat(deploy): integrate Cloudflare Tunnel` |
| 3 | `test(deploy): add Cloudflare Tunnel tests` |
| FINAL | `docs: update Cloudflare Tunnel documentation` |

---

## Success Criteria

### Verification Commands

```bash
# Test library loading
source deploy/lib/cloudflare-functions.sh && type setup_cloudflare_tunnel

# Test interactive prompt
echo "6" | bash deploy/setup-quick.sh --dry-run

# Test standalone script
bash deploy/setup-cloudflare-tunnel.sh --help

# Run integration tests
bash tests/test-cloudflare-tunnel.sh
```

### Final Checklist

- [ ] All "Must Have" present
- [ ] All "Must NOT Have" absent
- [ ] E2E tests pass for all 6 access modes
- [ ] Documentation updated
- [ ] Edge cases handled
