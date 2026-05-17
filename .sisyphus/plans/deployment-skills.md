# Deployment Skills for AI CLI Tools

## TL;DR

> **Quick Summary**: Add cross-platform deployment skills (`.skills/*.yaml` + `SKILL.md`) to help users deploy and manage ArmorClaw using their AI CLI tools (Claude Code, OpenCode, etc.).
>
> **Deliverables**:
> - `.skills/deploy.yaml` + `deploy/SKILL.md` - VPS deployment skill
> - `.skills/status.yaml` + `status/SKILL.md` - Health/status checking skill
> - `.skills/cloudflare.yaml` + `cloudflare/SKILL.md` - Cloudflare HTTPS setup skill
> - `.skills/provision.yaml` + `provision/SKILL.md` - Mobile provisioning skill
> - Updated `ARMORCLAW.md` with skill discovery instructions
>
> **Estimated Effort**: Medium (4 skills × 2 files each + docs updates)
> **Parallel Execution**: YES - 3 waves (structure → skills → docs)
> **Critical Path**: Task 1 (directory) → Tasks 2-5 (skills) → Task 6 (docs) → Task 7 (verify)

---

## Context

### Original Request
Review and revise the plan to add skills/YAML documents to let new users use their AI CLI tools to launch and manage ArmorClaw deployments to VPS, with cross-platform support (Linux, macOS, Windows).

### Interview Summary
**Key Discussions**:
- **Format**: Both YAML (machine-readable) + SKILL.md (AI-friendly documentation)
- **Location**: Project-level `.skills/` directory
- **Automation**: Hybrid model - `auto` (execute), `confirm` (ask first), `guide` (user does it)
- **Commands**: `/deploy`, `/status`, `/cloudflare`, `/provision`
- **Platforms**: Linux, macOS, Windows (PowerShell, Git Bash, WSL)

**Research Findings**:
- Agent skills in `container/openclaw-src/skills/` use YAML frontmatter + markdown
- Deploy scripts exist in `deploy/` (install.sh, setup-cloudflare.sh, etc.)
- No existing `.skills/` directory for AI CLI tools
- ARMORCLAW.md has comprehensive deployment documentation
- Deployment modes: Native, Sentinel, Cloudflare Tunnel, Cloudflare Proxy

### Metis Review
**Identified Gaps** (addressed):
- Directory structure decision: `.skills/` at project root (validated)
- Format rationale: YAML for structure, SKILL.md for AI guidance (dual purpose)
- Windows compatibility: Git Bash/WSL preferred over PowerShell (documented)
- Security model: Environment variables only, no secrets in files (enforced)
- Error recovery: Graceful degradation with clear messages (included)
- Test strategy: Agent-executed QA scenarios (defined)

---

## Work Objectives

### Core Objective
Create deployment skills that help users leverage their AI CLI tools to deploy and manage ArmorClaw on VPS, with seamless cross-platform support.

### Concrete Deliverables
- `.skills/deploy.yaml` - Structured deployment steps with parameters
- `.skills/deploy/SKILL.md` - AI-friendly deployment instructions
- `.skills/status.yaml` - Status checking configuration
- `.skills/status/SKILL.md` - Status verification guide
- `.skills/cloudflare.yaml` - Cloudflare setup configuration
- `.skills/cloudflare/SKILL.md` - Cloudflare integration guide
- `.skills/provision.yaml` - Mobile provisioning configuration
- `.skills/provision/SKILL.md` - Device provisioning guide
- Updated `ARMORCLAW.md` - Skills discovery section

### Definition of Done
- [ ] All 4 skills created with YAML + SKILL.md
- [ ] Cross-platform support verified (Linux, macOS, Windows)
- [ ] Automation flags implemented (auto/confirm/guide)
- [ ] ARMORCLAW.md updated with skill discovery
- [ ] Skills reference existing deploy/ scripts correctly
- [ ] No hardcoded secrets or API keys
- [ ] Error handling with actionable messages

### Must Have
- YAML files with structured `parameters`, `steps`, `platforms`
- SKILL.md files following superpowers conventions
- Cross-platform compatibility table in each skill
- Automation flags (`auto`/`confirm`/`guide`) in each step
- References to existing deploy/ scripts and docs

### Must NOT Have (Guardrails)
- NO duplication of existing deployment documentation
- NO hardcoded API keys, passwords, or secrets
- NO PowerShell-only examples (include Git Bash/WSL alternatives)
- NO complex rollback procedures (keep simple)
- NO modification of existing deploy/ scripts
- NO external integrations beyond ArmorClaw + Cloudflare

---

## Verification Strategy (MANDATORY)

> **ZERO HUMAN INTERVENTION** — ALL verification is agent-executed.

### Test Decision
- **Infrastructure exists**: YES (existing deploy scripts)
- **Automated tests**: Tests-after (validate after creation)
- **Framework**: Bash validation (syntax, structure checks)
- **Agent-Executed QA**: ALWAYS (mandatory for all tasks)

### QA Policy
Every task includes agent-executed QA scenarios.
Evidence saved to `.sisyphus/evidence/task-{N}-{scenario-slug}.{ext}`.

- **File validation**: Use Bash (cat, grep, yamllint) — Verify structure, syntax
- **Cross-platform**: Use Bash (test commands) — Verify platform-specific instructions
- **Integration**: Use Bash (curl) — Test skill references work correctly

---

## Execution Strategy

### Parallel Execution Waves

```
Wave 1 (Start Immediately — scaffolding):
├── Task 1: Create .skills/ directory structure [quick]
├── Task 2: Design skill schema template [quick]
└── Task 3: Create cross-platform utility functions [quick]

Wave 2 (After Wave 1 — core skills, MAX PARALLEL):
├── Task 4: Create deploy skill (YAML + SKILL.md) [unspecified-high]
├── Task 5: Create status skill (YAML + SKILL.md) [unspecified-high]
├── Task 6: Create cloudflare skill (YAML + SKILL.md) [unspecified-high]
└── Task 7: Create provision skill (YAML + SKILL.md) [unspecified-high]

Wave 3 (After Wave 2 — documentation):
├── Task 8: Update ARMORCLAW.md with skills section [quick]
└── Task 9: Create skills README index [quick]

Wave FINAL (After ALL tasks — verification):
├── Task F1: File structure audit [quick]
├── Task F2: Cross-platform compatibility check [unspecified-high]
├── Task F3: Reference validation [quick]
└── Task F4: Integration smoke test [unspecified-high]
```

### Dependency Matrix

- **1-3**: — — 4-7, 1
- **4-7**: 1, 2, 3 — 8, 9, 2
- **8-9**: 4, 5, 6, 7 — F1-F4, 3
- **F1-F4**: 1-9 — User approval, 4

### Agent Dispatch Summary

- **Wave 1**: **3** — T1-T2 → `quick`, T3 → `quick`
- **Wave 2**: **4** — T4-T7 → `unspecified-high`
- **Wave 3**: **2** — T8-T9 → `quick`
- **FINAL**: **4** — F1-F4 → `quick`, `unspecified-high`

---

## TODOs

- [x] 1. Create .skills/ Directory Structure

  **What to do**:
  - Create `.skills/` directory at project root
  - Create subdirectories for each skill: `deploy/`, `status/`, `cloudflare/`, `provision/`
  - Add `.gitkeep` files to track empty directories
  - Verify structure matches pattern: `.skills/{skill-name}/`

  **Must NOT do**:
  - DO NOT create files in `container/openclaw-src/skills/` (that's for agent skills)
  - DO NOT add README yet (separate task)
  - DO NOT commit until structure verified

  **Recommended Agent Profile**:
  - **Category**: `quick`
    - Reason: Simple directory creation, no complex logic
  - **Skills**: []
    - No specialized skills needed for mkdir operations

  **Parallelization**:
  - **Can Run In Parallel**: NO (foundation for other tasks)
  - **Parallel Group**: Wave 1 (with Tasks 2, 3)
  - **Blocks**: Tasks 4-9
  - **Blocked By**: None (can start immediately)

  **References**:
  - `container/openclaw-src/skills/` - Pattern: subdirectory per skill
  - `.sisyphus/` - Pattern: project-level hidden directory

  **Acceptance Criteria**:
  - [ ] `.skills/` directory exists at project root
  - [ ] Subdirectories exist: `deploy/`, `status/`, `cloudflare/`, `provision/`
  - [ ] `.gitkeep` files created in each subdirectory

  **QA Scenarios**:

  ```
  Scenario: Directory structure created correctly
    Tool: Bash
    Preconditions: None
    Steps:
      1. ls -la .skills/
      2. ls -la .skills/deploy/ .skills/status/ .skills/cloudflare/ .skills/provision/
    Expected Result: All directories exist, .gitkeep files present
    Failure Indicators: "No such file or directory"
    Evidence: .sisyphus/evidence/task-1-directory-structure.txt
  ```

  **Commit**: NO (groups with Wave 1)

---

- [x] 2. Design Skill Schema Template

  **What to do**:
  - Create reusable template for YAML skill structure
  - Define schema: `name`, `version`, `description`, `parameters`, `steps`, `platforms`, `examples`
  - Include automation flag pattern: `auto`/`confirm`/`guide`
  - Document in `.skills/TEMPLATE.yaml`

  **Must NOT do**:
  - DO NOT create skill-specific content (that's Tasks 4-7)
  - DO NOT over-engineer (keep simple, <100 lines)

  **Recommended Agent Profile**:
  - **Category**: `quick`
    - Reason: Documentation/template creation, no code
  - **Skills**: []
    - No specialized skills needed

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 1 (with Tasks 1, 3)
  - **Blocks**: Tasks 4-7 (need template)
  - **Blocked By**: None

  **References**:
  - `container/openclaw-src/skills/github/SKILL.md` - Pattern: YAML frontmatter structure
  - User's proposed YAML format from interview

  **Acceptance Criteria**:
  - [ ] `.skills/TEMPLATE.yaml` created
  - [ ] Template includes all required fields
  - [ ] Template includes automation flag examples

  **QA Scenarios**:

  ```
  Scenario: Template has required schema fields
    Tool: Bash
    Preconditions: Task 1 complete
    Steps:
      1. grep "name:" .skills/TEMPLATE.yaml
      2. grep "parameters:" .skills/TEMPLATE.yaml
      3. grep "steps:" .skills/TEMPLATE.yaml
      4. grep "automation:" .skills/TEMPLATE.yaml
    Expected Result: All fields present in output
    Failure Indicators: Empty grep output
    Evidence: .sisyphus/evidence/task-2-template-schema.txt
  ```

  **Commit**: NO (groups with Wave 1)

---

- [x] 3. Create Cross-Platform Utility Functions

  **What to do**:
  - Document platform detection logic for skills to use
  - Create examples for:
    - SSH connection (Linux/macOS vs Windows PowerShell vs Git Bash/WSL)
    - Path handling (`~/.ssh/` vs `C:\Users\...` vs `/mnt/c/...`)
    - curl vs Invoke-WebRequest
  - Document in `.skills/PLATFORM.md`

  **Must NOT do**:
  - DO NOT create actual scripts (documentation only)
  - DO NOT include PowerShell-only examples (always show Git Bash alternative)

  **Recommended Agent Profile**:
  - **Category**: `quick`
    - Reason: Documentation, no code execution
  - **Skills**: []
    - No specialized skills needed

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 1 (with Tasks 1, 2)
  - **Blocks**: Tasks 4-7 (need platform reference)
  - **Blocked By**: None

  **References**:
  - User's cross-platform analysis table from interview
  - `ARMORCLAW.md` - Existing platform documentation

  **Acceptance Criteria**:
  - [ ] `.skills/PLATFORM.md` created
  - [ ] Platform detection logic documented
  - [ ] SSH, path, curl examples for all 3 platform types

  **QA Scenarios**:

  ```
  Scenario: Platform documentation covers all platforms
    Tool: Bash
    Preconditions: Task 1 complete
    Steps:
      1. grep -c "Linux" .skills/PLATFORM.md
      2. grep -c "macOS" .skills/PLATFORM.md
      3. grep -c "Windows" .skills/PLATFORM.md
      4. grep -c "PowerShell\|Git Bash\|WSL" .skills/PLATFORM.md
    Expected Result: All counts > 0
    Failure Indicators: Zero count for any platform
  **Commit**: NO (groups with Wave 1)

---

- [ ] 4. Create Deploy Skill (YAML + SKILL.md)

  **What to do**:
  - Create `.skills/deploy.yaml` with structured deployment steps
  - Create `.skills/deploy/SKILL.md` with AI-friendly instructions
  - Include parameters: `vps_ip`, `ssh_user`, `ssh_key`, `domain`, `mode`
  - Include steps: `detect_os`, `connect`, `install`, `wait`, `verify`, `get_info`
  - Add automation flags: `auto` for checks, `confirm` for SSH/install, `guide` for account setup
  - Add cross-platform support table

  **Must NOT do**:
  - DO NOT duplicate content from ARMORCLAW.md (reference it instead)
  - DO NOT hardcode API keys or passwords
  - DO NOT create new deployment scripts (use existing `deploy/install.sh`)

  **Recommended Agent Profile**:
  - **Category**: `unspecified-high`
    - Reason: Requires understanding of deployment architecture, cross-platform concerns
  - **Skills**: []
    - Domain knowledge sufficient

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 2 (with Tasks 5, 6, 7)
  - **Blocks**: Tasks 8, 9
  - **Blocked By**: Tasks 1, 2, 3

  **References**:
  - `deploy/install.sh` - Existing installer to invoke
  - `ARMORCLAW.md:Deployment Modes` - Mode definitions
  - `.skills/TEMPLATE.yaml` - Schema template
  - `.skills/PLATFORM.md` - Cross-platform patterns

  **Acceptance Criteria**:
  - [ ] `.skills/deploy.yaml` created with all required fields
  - [ ] `.skills/deploy/SKILL.md` created with YAML frontmatter
  - [ ] All steps have `automation` flags
  - [ ] Cross-platform table present
  - [ ] References `deploy/install.sh`

  **QA Scenarios**:

  ```
  Scenario: Deploy YAML is valid and complete
    Tool: Bash
    Preconditions: Tasks 1-3 complete
    Steps:
      1. python3 -c "import yaml; yaml.safe_load(open('.skills/deploy.yaml'))"
      2. grep "automation:" .skills/deploy.yaml | wc -l
    Expected Result: YAML parses successfully, automation flags > 0
    Failure Indicators: YAML parse error, zero automation flags
    Evidence: .sisyphus/evidence/task-4-deploy-yaml.txt

  Scenario: Deploy SKILL.md has frontmatter and examples
    Tool: Bash
    Preconditions: Tasks 1-3 complete
    Steps:
      1. head -5 .skills/deploy/SKILL.md | grep "^---"
      2. grep -c "## Example" .skills/deploy/SKILL.md
    Expected Result: Frontmatter present, examples > 0
    Failure Indicators: No frontmatter markers
    Evidence: .sisyphus/evidence/task-4-deploy-skill.md
  ```

  **Commit**: NO (groups with Wave 2)

---

- [ ] 5. Create Status Skill (YAML + SKILL.md)

  **What to do**:
  - Create `.skills/status.yaml` with status checking steps
  - Create `.skills/status/SKILL.md` with verification instructions
  - Include parameters: `vps_ip`, `ssh_key`, `ssh_user`
  - Include steps: `check_services`, `check_endpoints`, `check_cloudflare`
  - All steps use `automation: auto` (read-only operations)
  - Include output schema: `bridge_status`, `matrix_status`, `cloudflare_status`

  **Must NOT do**:
  - DO NOT modify any services (status checks only)
  - DO NOT require SSH for endpoint checks (can use curl from local machine)

  **Recommended Agent Profile**:
  - **Category**: `unspecified-high`
    - Reason: Needs understanding of service health checks
  - **Skills**: []
    - No specialized skills needed

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 2 (with Tasks 4, 6, 7)
  - **Blocks**: Tasks 8, 9
  - **Blocked By**: Tasks 1, 2, 3

  **References**:
  - `ARMORCLAW.md:Observability` - Check status commands
  - `deploy/health-check.sh` - Existing health check patterns
  - `.skills/TEMPLATE.yaml` - Schema template

  **Acceptance Criteria**:
  - [ ] `.skills/status.yaml` created
  - [ ] `.skills/status/SKILL.md` created
  - [ ] All steps marked `automation: auto`
  - [ ] Cross-platform curl examples included

  **QA Scenarios**:

  ```
  Scenario: Status skill has auto automation for all steps
    Tool: Bash
    Preconditions: Tasks 1-3 complete
    Steps:
      1. grep "automation:" .skills/status.yaml
      2. grep "automation: confirm\|automation: guide" .skills/status.yaml
    Expected Result: All steps show "auto", no confirm/guide found
    Failure Indicators: Any "confirm" or "guide" flags
    Evidence: .sisyphus/evidence/task-5-status-auto.txt
  ```

  **Commit**: NO (groups with Wave 2)

---

- [ ] 6. Create Cloudflare Skill (YAML + SKILL.md)

  **What to do**:
  - Create `.skills/cloudflare.yaml` with Cloudflare setup steps
  - Create `.skills/cloudflare/SKILL.md` with configuration guide
  - Include parameters: `vps_ip`, `ssh_key`, `domain`, `mode` (tunnel/proxy)
  - Include steps: `detect_network`, `run_setup`, `verify_https`
  - Automation: `confirm` for setup, `auto` for verification
  - Include Cloudflare API token requirements

  **Must NOT do**:
  - DO NOT hardcode Cloudflare API tokens
  - DO NOT create Cloudflare account for user (`automation: guide`)
  - DO NOT modify DNS records outside ArmorClaw scope

  **Recommended Agent Profile**:
  - **Category**: `unspecified-high`
    - Reason: Cloudflare integration requires careful handling
  - **Skills**: []
    - No specialized skills needed

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 2 (with Tasks 4, 5, 7)
  - **Blocks**: Tasks 8, 9
  - **Blocked By**: Tasks 1, 2, 3

  **References**:
  - `deploy/setup-cloudflare.sh` - Existing Cloudflare setup script
  - `ARMORCLAW.md:Cloudflare Setup` - Cloudflare documentation
  - `ARMORCLAW.md:Cloudflare Tunnel Mode` - Tunnel configuration

  **Acceptance Criteria**:
  - [ ] `.skills/cloudflare.yaml` created
  - [ ] `.skills/cloudflare/SKILL.md` created
  - [ ] Tunnel and Proxy modes both documented
  - [ ] API token requirements clearly stated

  **QA Scenarios**:

  ```
  Scenario: Cloudflare skill covers both modes
    Tool: Bash
    Preconditions: Tasks 1-3 complete
    Steps:
      1. grep -c "tunnel" .skills/cloudflare.yaml
      2. grep -c "proxy" .skills/cloudflare.yaml
      3. grep "CF_API_TOKEN" .skills/cloudflare.yaml
    Expected Result: Both modes mentioned, API token parameter present
    Failure Indicators: Missing mode or missing API token parameter
    Evidence: .sisyphus/evidence/task-6-cloudflare-modes.txt
  ```

  **Commit**: NO (groups with Wave 2)

---

- [ ] 7. Create Provision Skill (YAML + SKILL.md)

  **What to do**:
  - Create `.skills/provision.yaml` with mobile provisioning steps
  - Create `.skills/provision/SKILL.md` with device setup guide
  - Include parameters: `vps_ip`, `ssh_key`
  - Include steps: `generate_qr`, `display_deep_link`, `manual_entry`
  - Automation: `auto` for QR generation, `guide` for mobile app setup
  - Document QR code generation with `qrencode`

  **Must NOT do**:
  - DO NOT require QR code display (offer manual entry alternative)
  - DO NOT store credentials (read from VPS, display once)

  **Recommended Agent Profile**:
  - **Category**: `unspecified-high`
    - Reason: Mobile provisioning has security implications
  - **Skills**: []
    - No specialized skills needed

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 2 (with Tasks 4, 5, 6)
  - **Blocks**: Tasks 8, 9
  - **Blocked By**: Tasks 1, 2, 3

  **References**:
  - `deploy/armorclaw-provision.sh` - Existing provisioning script
  - `ARMORCLAW.md:ArmorChat Mobile App Connection` - Provisioning documentation

  **Acceptance Criteria**:
  - [ ] `.skills/provision.yaml` created
  - [ ] `.skills/provision/SKILL.md` created
  - [ ] QR code generation documented
  - [ ] Manual entry alternative provided

  **QA Scenarios**:

  ```
  Scenario: Provision skill offers both QR and manual options
    Tool: Bash
    Preconditions: Tasks 1-3 complete
    Steps:
      1. grep -c "qrencode\|QR" .skills/provision.yaml
      2. grep -c "manual\|Manual entry" .skills/provision.yaml
    Expected Result: Both QR and manual methods documented
    Failure Indicators: Only one method present
    Evidence: .sisyphus/evidence/task-7-provision-methods.txt
  ```

  **Commit**: NO (groups with Wave 2)

---

- [ ] 8. Update ARMORCLAW.md with Skills Section

  **What to do**:
  - Add "## Deployment Skills for AI CLI Tools" section to ARMORCLAW.md
  - Document how to invoke skills: `/deploy`, `/status`, `/cloudflare`, `/provision`
  - Add skill discovery instructions for AI CLI tools
  - Reference `.skills/` directory structure
  - Keep section concise (<50 lines)

  **Must NOT do**:
  - DO NOT duplicate skill content (reference only)
  - DO NOT remove existing deployment documentation

  **Recommended Agent Profile**:
  - **Category**: `quick`
    - Reason: Simple documentation update
  - **Skills**: []
    - No specialized skills needed

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 3 (with Task 9)
  - **Blocks**: None
  - **Blocked By**: Tasks 4-7 (need skills to exist)

  **References**:
  - `ARMORCLAW.md` - Existing documentation to extend
  - `.skills/` directory - Skills to reference

  **Acceptance Criteria**:
  - [ ] ARMORCLAW.md has new skills section
  - [ ] Skill invocation documented
  - [ ] `.skills/` directory mentioned

  **QA Scenarios**:

  ```
  Scenario: ARMORCLAW.md documents skill discovery
    Tool: Bash
    Preconditions: Tasks 4-7 complete
    Steps:
      1. grep "## Deployment Skills" ARMORCLAW.md
      2. grep "/deploy\|/status\|/cloudflare\|/provision" ARMORCLAW.md
      3. grep ".skills/" ARMORCLAW.md
    Expected Result: All skills mentioned, directory referenced
    Failure Indicators: Missing section or commands
    Evidence: .sisyphus/evidence/task-8-armorchaw-update.txt
  ```

  **Commit**: NO (groups with Wave 3)

---

- [ ] 9. Create Skills README Index

  **What to do**:
  - Create `.skills/README.md` with overview of all skills
  - Include quick reference table: skill name, purpose, commands
  - Include platform support matrix
  - Link to each skill's SKILL.md file
  - Keep concise (<100 lines)

  **Must NOT do**:
  - DO NOT duplicate skill content
  - DO NOT create separate README per skill (one index file only)

  **Recommended Agent Profile**:
  - **Category**: `quick`
    - Reason: Documentation index creation
  - **Skills**: []
    - No specialized skills needed

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 3 (with Task 8)
  - **Blocks**: None
  - **Blocked By**: Tasks 4-7 (need skills to index)

  **References**:
  - `.skills/` directory - Skills to index
  - `.skills/PLATFORM.md` - Platform matrix reference

  **Acceptance Criteria**:
  - [ ] `.skills/README.md` created
  - [ ] All 4 skills listed
  - [ ] Platform matrix included
  - [ ] Links to SKILL.md files work

  **QA Scenarios**:

  ```
  Scenario: README indexes all skills correctly
    Tool: Bash
    Preconditions: Tasks 4-7 complete
    Steps:
      1. grep -c "deploy\|status\|cloudflare\|provision" .skills/README.md
      2. grep "\[.*\](.*/SKILL.md)" .skills/README.md | wc -l
    Expected Result: All 4 skills mentioned, links present
    Failure Indicators: Missing skills or broken links
    Evidence: .sisyphus/evidence/task-9-readme-index.txt
  ```

  **Commit**: NO (groups with Wave 3)

---

## Final Verification Wave

- [ ] F1. **File Structure Audit** — `quick`
  Verify all files created correctly:
  ```bash
  # Check directory structure
  ls -la .skills/
  ls -la .skills/*/SKILL.md
  
  # Count files
  find .skills -type f | wc -l  # Expected: 8 files (4 YAML + 4 SKILL.md + README)
  ```
  Output: `Files [8/8] | Structure [VALID] | VERDICT: APPROVE`

- [ ] F2. **Cross-Platform Compatibility Check** — `unspecified-high`
  Verify each skill has platform-specific instructions:
  ```bash
  # Check for platform sections
  grep -r "platforms:" .skills/*.yaml
  grep -r "Linux\|macOS\|Windows" .skills/*/SKILL.md
  
  # Verify automation flags
  grep -r "automation:" .skills/*.yaml
  ```
  Output: `Platforms [4/4] | Flags [PRESENT] | VERDICT: APPROVE`

- [ ] F3. **Reference Validation** — `quick`
  Verify all file references exist:
  ```bash
  # Check deploy script references
  test -f deploy/install.sh && echo "install.sh: OK"
  test -f deploy/setup-cloudflare.sh && echo "setup-cloudflare.sh: OK"
  
  # Check doc references
  test -f ARMORCLAW.md && echo "ARMORCLAW.md: OK"
  ```
  Output: `References [VALID] | Links [WORKING] | VERDICT: APPROVE`

- [ ] F4. **Integration Smoke Test** — `unspecified-high`
  Test skill structure is parseable:
  ```bash
  # Validate YAML syntax
  for f in .skills/*.yaml; do
    python3 -c "import yaml; yaml.safe_load(open('$f'))" && echo "$f: VALID"
  done
  
  # Check SKILL.md frontmatter
  grep -l "^---" .skills/*/SKILL.md | wc -l  # Expected: 4
  ```
  Output: `YAML [VALID] | Frontmatter [4/4] | VERDICT: APPROVE`

---

## Commit Strategy

- **Wave 1**: `feat(skills): add .skills directory structure and templates` — .skills/, 1 file
- **Wave 2**: `feat(skills): add deployment skills (deploy, status, cloudflare, provision)` — .skills/, 8 files
- **Wave 3**: `docs: update ARMORCLAW.md with skill discovery` — ARMORCLAW.md, .skills/README.md
- **Final**: `test(skills): verify skill structure and cross-platform support` — .sisyphus/evidence/

---

## Success Criteria

### Verification Commands
```bash
# Verify all skills created
find .skills -name "*.yaml" | wc -l  # Expected: 4
find .skills -name "SKILL.md" | wc -l  # Expected: 4

# Verify cross-platform support
grep -c "Windows\|PowerShell\|Git Bash" .skills/*/SKILL.md  # Expected: >0 in each

# Verify automation flags
grep -c "automation:" .skills/*.yaml  # Expected: >0 in each

# Verify no hardcoded secrets
grep -r "password\|api_key\|secret" .skills/ | grep -v "example\|placeholder"  # Expected: empty
```

### Final Checklist
- [ ] All 4 skills present (YAML + SKILL.md)
- [ ] Cross-platform support verified
- [ ] Automation flags implemented
- [ ] No hardcoded secrets
- [ ] References to deploy/ scripts correct
- [ ] ARMORCLAW.md updated
- [ ] Evidence captured
