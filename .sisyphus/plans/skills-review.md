# ArmorClaw Skills Review: Thorough Capability & Test Plan

## TL;DR

> **Quick Summary**: Comprehensive static review of all 4 skill layers in ArmorClaw (Deployment YAML, Bridge Go, OpenClaw TypeScript, Container Python), producing a SKILLS_REVIEW.md document and a test execution plan. No code changes, no live VPS — static analysis only.
> 
> **Deliverables**:
> - SKILLS_REVIEW.md — Comprehensive review document (≤800 lines) with findings, severity ratings, and file:line references
> - Test execution plan — Concrete tasks for implementing tests and fixes in a future phase
> - Previous review delta — Cross-reference with DEPLOYMENT_SKILLS_REVIEW.md (11/12 items still open)
> 
> **Estimated Effort**: Large
> **Parallel Execution**: YES — 3 waves
> **Critical Path**: T1 (harvest findings) → T2-T4 (layer analysis, parallel) → T5-T6 (synthesis) → T7-T8 (review document) → T9-T11 (test plan)

---

## Context

### Original Request
User asked to "plan a review of all the skills for testing VPS deployment — make a plan for a thorough review of the capabilities and expected results of the skills."

### Interview Summary
**Key Discussions**:
- Scope: All 4 skill layers (Deployment YAML, Bridge Go, OpenClaw TS, Container Python) — user confirmed
- Output: Review document + test execution plan — user confirmed
- Environment: Static code review only, no live VPS — user confirmed
- Previous review: DEPLOYMENT_SKILLS_REVIEW.md (April 5) exists with 11/12 items unimplemented — supersede it

**Research Findings**:
- 4 deployment skills: deploy (7 steps, 280 lines), status (9 steps, 210 lines), cloudflare (4 steps, 339 lines), provision (3 steps, 87 lines)
- Bridge has 15 Go skill implementations (6,872 lines) with ZERO test files in `internal/skills/`
- Bridge `pkg/skills/` has 2 files (481 lines) with full test coverage (692 lines)
- OpenClaw has 58 bundled SKILL.md files + 31 test files (5,114 lines)
- Container has Python runtime skills (SSL tunnels) with partial test coverage
- 19 security findings identified by Metis (2 CRITICAL, 3 HIGH, 8 MEDIUM, 6 LOW)
- 4 skills claim impossible Windows platform support
- `test-deployment-skills.sh` validates structure only, not function — gives false confidence

### Metis Review
**Identified Gaps** (addressed):
- Missing audience specification → Default: developer implementing fixes
- TS infrastructure vs SKILL.md distinction → Include both, TS infrastructure as code review
- Dual deployment paths (deploy.sh vs .skills/deploy.yaml) → Document only, don't reconcile
- SKILL.md format spec (hand-rolled parser in registry.go) → Include format spec assessment
- Mock vs real Go skill implementations → Flag which skills are production vs mock
- Review document lifecycle → Include maintenance cadence recommendation
- `containsDangerousChars` blocks legitimate inputs → Include as functional bug finding
- `test-deployment-skills.sh` validates false Windows claims → Include as meta-finding

---

## Work Objectives

### Core Objective
Produce a comprehensive SKILLS_REVIEW.md documenting the capabilities, expected results, and gaps of all 4 skill layers in ArmorClaw, plus a test execution plan for implementing fixes.

### Concrete Deliverables
- `SKILLS_REVIEW.md` — Review document at project root (supersedes DEPLOYMENT_SKILLS_REVIEW.md)
- Detailed test execution plan embedded as TODOs in this plan file

### Definition of Done
- [ ] Every file in `bridge/internal/skills/` individually assessed (15 files)
- [ ] Every `.skills/*.yaml` has a platform-claims-vs-reality table
- [ ] Every security finding has: file, line, severity, description, remediation
- [ ] Summary table of all 4 layers with test coverage percentages
- [ ] Cross-reference with old DEPLOYMENT_SKILLS_REVIEW action items (carry-forward status)
- [ ] Test execution plan with concrete tasks specifying language, file, test name, cases, expected results

### Must Have
- All findings backed by file:line references
- Severity classification (CRITICAL/HIGH/MEDIUM/LOW)
- Previous review delta section
- Action item table with priority ordering

### Must NOT Have (Guardrails)
- No code modifications — review and plan only
- No live VPS testing, SSH, Docker, or network calls
- No CI/CD pipeline implementation (recommend only)
- No `deploy.sh` vs `.skills/deploy.yaml` reconciliation (document only)
- No ArmorChat, ArmorTerminal, Rust Vault, Matrix subsystem analysis
- No actual Go test code (plan the tests, don't implement)
- No per-file manual analysis of 58 OpenClaw SKILL.md files (automated checks only)
- Review document ≤ 800 lines (use tables over prose)

---

## Verification Strategy (MANDATORY)

> **ZERO HUMAN INTERVENTION** — ALL verification is agent-executed. No exceptions.

### Test Decision
- **Infrastructure exists**: YES (existing test files in multiple languages)
- **Automated tests**: Tests-after (new tests planned for future phase)
- **Framework**: bash (shell), Go (bridge), TypeScript (OpenClaw)
- **This plan creates**: Review document + test plan (no test implementation)

### QA Policy
Every task includes agent-executed QA scenarios using static tools only:
- `bash -n <file>` — Bash syntax validation
- `python3 -c "import yaml; yaml.safe_load(open('file'))"` — YAML validation
- `grep -c "pattern" file` — Content pattern verification
- `go vet ./...` — Go static analysis
- `pandoc file.md --to plain > /dev/null` — Markdown validity

Evidence saved to `.sisyphus/evidence/task-{N}-{scenario-slug}.{ext}`.

---

## Execution Strategy

### Parallel Execution Waves

```
Wave 1 (Foundation — harvest raw findings):
├── Task 1: Harvest deployment skill findings (YAML + SKILL.md + scripts) [deep]
├── Task 2: Harvest Bridge Go skill findings (internal/skills/ + pkg/skills/) [deep]
└── Task 3: Harvest OpenClaw + Container findings (TS infra + Python + SKILL.md format) [deep]

Wave 2 (Synthesis — produce deliverables):
├── Task 4: Synthesize all findings into structured severity table [ultrabrain]
├── Task 5: Write SKILLS_REVIEW.md — Layer 1 (Deployment) + Layer 2 (Bridge) sections [writing]
├── Task 6: Write SKILLS_REVIEW.md — Layer 3 (OpenClaw) + Layer 4 (Container) + Cross-cutting sections [writing]

Wave 3 (Test plan + final assembly):
├── Task 7: Write test execution plan — Bridge Go tests (zero-test gap) [deep]
├── Task 8: Write test execution plan — Deployment + OpenClaw + Container tests [unspecified-high]
├── Task 9: Write SKILLS_REVIEW.md — Executive summary + previous review delta + action items [writing]

Wave FINAL (Verification — after ALL implementation tasks):
├── Task F1: Plan compliance audit (oracle)
├── Task F2: Finding accuracy verification (unspecified-high)
├── Task F3: Static QA — all references verified (unspecified-high)
└── Task F4: Scope fidelity check (deep)
→ Present results → Get explicit user okay

Critical Path: T1-T3 (parallel harvest) → T4 (synthesize) → T5-T6 (document, parallel) → T7-T8 (test plan, parallel) → T9 (finalize) → F1-F4 → user okay
Parallel Speedup: ~50% faster than sequential
Max Concurrent: 3 (Waves 1 and 3)
```

### Dependency Matrix

| Task | Depends On | Blocks | Wave |
|------|-----------|--------|------|
| 1 | - | 4, 5 | 1 |
| 2 | - | 4, 5 | 1 |
| 3 | - | 4, 6 | 1 |
| 4 | 1, 2, 3 | 5, 6 | 2 |
| 5 | 4 | 9 | 2 |
| 6 | 4 | 9 | 2 |
| 7 | 4 | 9 | 3 |
| 8 | 4 | 9 | 3 |
| 9 | 5, 6, 7, 8 | F1-F4 | 3 |
| F1 | 9 | - | FINAL |
| F2 | 9 | - | FINAL |
| F3 | 9 | - | FINAL |
| F4 | 9 | - | FINAL |

### Agent Dispatch Summary

- **Wave 1**: **3** — T1 → `deep`, T2 → `deep`, T3 → `deep`
- **Wave 2**: **3** — T4 → `ultrabrain`, T5 → `writing`, T6 → `writing`
- **Wave 3**: **3** — T7 → `deep`, T8 → `unspecified-high`, T9 → `writing`
- **FINAL**: **4** — F1 → `oracle`, F2 → `unspecified-high`, F3 → `unspecified-high`, F4 → `deep`

---

## TODOs

> EVERY task MUST have: Recommended Agent Profile + Parallelization info + QA Scenarios.
> **A task WITHOUT QA Scenarios is INCOMPLETE. No exceptions.**

- [x] 1. Harvest Deployment Skill Findings (YAML + SKILL.md + Scripts)

  **What to do**:
  - Run `bash -n` on every deploy script referenced by `.skills/*.yaml` (install.sh, setup-cloudflare.sh, armorclaw-provision.sh, health-check.sh)
  - Validate all YAML files with `python3 -c "import yaml; yaml.safe_load(...)"`
  - For each of the 4 deployment skills, extract: parameters, steps, automation levels, platform claims
  - For each step, verify the embedded bash command syntax with `bash -n` (extract to temp file first)
  - Cross-reference platform claims against actual commands used (bash-isms, `stat -c`, `sudo`, `nc`)
  - Check all referenced scripts exist and are executable
  - Catalog security patterns: `sudo` usage, `curl | bash`, hardcoded IPs, unquoted variables
  - Verify SKILL.md frontmatter matches YAML definitions (name, version, description, parameters)

  **Must NOT do**:
  - No modifications to any file
  - No SSH connections or network calls
  - No per-SKILL.md content quality review beyond frontmatter consistency

  **Recommended Agent Profile**:
  - **Category**: `deep`
    - Reason: Multi-file analysis with cross-referencing, bash syntax validation, and security pattern detection
  - **Skills**: `[]`
  - **Skills Evaluated but Omitted**:
    - `playwright`: Not UI testing
    - `git-master`: No git operations needed

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 1 (with Tasks 2, 3)
  - **Blocks**: Tasks 4, 5
  - **Blocked By**: None (can start immediately)

  **References**:

  **Pattern References**:
  - `.skills/deploy.yaml` — Deploy skill definition (280 lines, 7 steps)
  - `.skills/status.yaml` — Status skill definition (210 lines, 9 steps)
  - `.skills/cloudflare.yaml` — Cloudflare skill definition (339 lines, 4 steps)
  - `.skills/provision.yaml` — Provision skill definition (87 lines, 3 steps)
  - `.skills/TEMPLATE.yaml` — Schema template for skill definitions
  - `.skills/deploy/SKILL.md` — Deploy skill documentation (260 lines)
  - `.skills/status/SKILL.md` — Status skill documentation
  - `.skills/cloudflare/SKILL.md` — Cloudflare skill documentation
  - `.skills/provision/SKILL.md` — Provision skill documentation

  **API/Type References**:
  - `deploy/install.sh` — Main installer script referenced by deploy.yaml step 4
  - `deploy/setup-cloudflare.sh` — Cloudflare setup referenced by cloudflare.yaml step 3
  - `deploy/armorclaw-provision.sh` — Provisioning script referenced by provision.yaml step 1
  - `deploy/health-check.sh` — Health check referenced by status.yaml step 9

  **Test References**:
  - `tests/test-deployment-skills.sh` — Existing structural test (10 tests, structure-only)
  - `DEPLOYMENT_SKILLS_REVIEW.md` — Previous review with 12 action items (1/12 done)

  **WHY Each Reference Matters**:
  - The YAML files define the skill contract — every step command must be valid bash
  - The deploy scripts are what actually runs on VPS — must be syntactically correct
  - The existing test validates YAML structure but NOT function — document this gap
  - The previous review's action items must be cross-referenced for carry-forward

  **Acceptance Criteria**:

  **QA Scenarios (MANDATORY):**

  ```
  Scenario: YAML syntax validation — all deployment skills parse correctly
    Tool: Bash
    Preconditions: Working directory is project root
    Steps:
      1. Run: python3 -c "import yaml; yaml.safe_load(open('.skills/deploy.yaml'))" && echo "OK"
      2. Run: python3 -c "import yaml; yaml.safe_load(open('.skills/status.yaml'))" && echo "OK"
      3. Run: python3 -c "import yaml; yaml.safe_load(open('.skills/cloudflare.yaml'))" && echo "OK"
      4. Run: python3 -c "import yaml; yaml.safe_load(open('.skills/provision.yaml'))" && echo "OK"
    Expected Result: All 4 commands exit 0 with "OK" output
    Failure Indicators: Any exit code non-zero, yaml.scanner.ScannerError, yaml.parser.ParserError
    Evidence: .sisyphus/evidence/task-1-yaml-validation.txt

  Scenario: Referenced deploy scripts exist and are valid bash
    Tool: Bash
    Preconditions: Working directory is project root
    Steps:
      1. Run: test -f deploy/install.sh && echo "EXISTS" || echo "MISSING"
      2. Run: test -f deploy/setup-cloudflare.sh && echo "EXISTS" || echo "MISSING"
      3. Run: test -f deploy/armorclaw-provision.sh && echo "EXISTS" || echo "MISSING"
      4. Run: test -f deploy/health-check.sh && echo "EXISTS" || echo "MISSING"
      5. Run: bash -n deploy/install.sh && echo "SYNTAX_OK" || echo "SYNTAX_ERROR"
      6. Run: bash -n deploy/setup-cloudflare.sh && echo "SYNTAX_OK" || echo "SYNTAX_ERROR"
      7. Run: bash -n deploy/armorclaw-provision.sh && echo "SYNTAX_OK" || echo "SYNTAX_ERROR"
      8. Run: bash -n deploy/health-check.sh && echo "SYNTAX_OK" || echo "SYNTAX_ERROR"
    Expected Result: All 4 files EXIST, all 4 pass bash -n syntax check
    Failure Indicators: Any MISSING output, any SYNTAX_ERROR output
    Evidence: .sisyphus/evidence/task-1-script-validation.txt

  Scenario: Security pattern detection — sudo and curl|bash usage
    Tool: Bash
    Preconditions: Working directory is project root
    Steps:
      1. Run: grep -rn "sudo" .skills/*.yaml | head -20
      2. Run: grep -rn "curl.*|.*bash" .skills/*..yaml | head -10
      3. Run: grep -rn "5\.183\.11\.149" .skills/*..yaml | head -10
    Expected Result: All occurrences catalogued with file:line:content
    Failure Indicators: Unexpected patterns not found (should find at least 2 sudo, 2 curl|bash, 4 hardcoded IPs)
    Evidence: .sisyphus/evidence/task-1-security-patterns.txt
  ```

  **Commit**: NO (read-only analysis task)

---

- [x] 2. Harvest Bridge Go Skill Findings (internal/skills/ + pkg/skills/)

  **What to do**:
  - Catalog all 15 files in `bridge/internal/skills/` — lines of code, purpose, mock vs real status
  - Run `go vet ./internal/skills/...` from bridge/ directory
  - For each Go skill file, extract: skill name, domain, risk level, timeout, SSRF protection, dangerous-char checks
  - Analyze `executor.go` 8-step pipeline — document each step, identify gaps
  - Analyze `policy.go` — identify which tools have policies vs allow-by-default
  - Analyze `ssrf.go` — identify bypass vectors (IPv6, userinfo, DNS rebinding)
  - Analyze `registry.go` — assess hand-rolled YAML parser limitations (lines 352-396)
  - Catalog `bridge/pkg/skills/` (2 files) — verify existing test coverage
  - Check `methods_skills.go` for v6 MCP router path vs legacy path differences
  - Flag commented-out real implementations in email_send.go, slack_message.go, calendar.go
  - Count lines of dangerous patterns: `log.Printf` with sensitive data, `fmt.Sprintf` for URLs, unchecked errors

  **Must NOT do**:
  - No modifications to any file
  - No `go test` execution (may require build dependencies)
  - No network calls or Docker

  **Recommended Agent Profile**:
  - **Category**: `deep`
    - Reason: Go source analysis, security pattern detection, architecture assessment
  - **Skills**: `[]`

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 1 (with Tasks 1, 3)
  - **Blocks**: Tasks 4, 5
  - **Blocked By**: None (can start immediately)

  **References**:

  **Pattern References**:
  - `bridge/internal/skills/executor.go` — 8-step PETG execution pipeline
  - `bridge/internal/skills/policy.go` — Tool policy definitions
  - `bridge/internal/skills/ssrf.go` — SSRF protection (203 lines)
  - `bridge/internal/skills/registry.go` — Skill registry with hand-rolled YAML parser
  - `bridge/internal/skills/schema.go` — OpenAI-compatible schema generator
  - `bridge/internal/skills/router.go` — Keyword-to-domain routing

  **API/Type References**:
  - `bridge/internal/skills/web_search.go` — Web search implementation (332 lines)
  - `bridge/internal/skills/web_extract.go` — Web extraction (507 lines)
  - `bridge/internal/skills/email_send.go` — Email sending (582 lines)
  - `bridge/internal/skills/slack_message.go` — Slack messaging (582 lines)
  - `bridge/internal/skills/calendar.go` — CalDAV calendar (589 lines)
  - `bridge/internal/skills/webdav.go` — WebDAV operations
  - `bridge/internal/skills/file_read.go` — File reading (421 lines)
  - `bridge/internal/skills/data_analyze.go` — Data analysis (959 lines)
  - `bridge/internal/skills/allowlist.go` — IP/CIDR allowlist

  **Test References**:
  - `bridge/pkg/skills/learned_store_test.go` — Tests for learned skill persistence
  - `bridge/pkg/skills/extractor_test.go` — Tests for skill extraction

  **WHY Each Reference Matters**:
  - `executor.go` is the security gate for ALL skill execution — any gap here affects everything
  - `policy.go` determines which tools are restricted — missing policies = unrestricted execution
  - `registry.go` hand-rolled parser means malformed SKILL.md silently produces bad results
  - Mock vs real distinction in email/slack/calendar affects what actually works on VPS

  **Acceptance Criteria**:

  **QA Scenarios (MANDATORY):**

  ```
  Scenario: Go vet passes on skills package
    Tool: Bash
    Preconditions: Working directory is bridge/
    Steps:
      1. Run: go vet ./internal/skills/... 2>&1 | head -30
      2. Run: go vet ./pkg/skills/... 2>&1 | head -30
    Expected Result: No errors (or catalogued errors as findings)
    Failure Indicators: Unexpected compilation errors not related to missing dependencies
    Evidence: .sisyphus/evidence/task-2-go-vet.txt

  Scenario: Zero-test gap confirmed for internal/skills/
    Tool: Bash
    Preconditions: Working directory is project root
    Steps:
      1. Run: find bridge/internal/skills/ -name "*_test.go" | wc -l
      2. Run: find bridge/pkg/skills/ -name "*_test.go" | wc -l
    Expected Result: internal/skills/ returns 0, pkg/skills/ returns 2
    Failure Indicators: Any _test.go found in internal/skills/ (gap already closed)
    Evidence: .sisyphus/evidence/task-2-test-gap.txt

  Scenario: Dangerous-char function blocks legitimate inputs
    Tool: Bash (grep)
    Preconditions: Working directory is project root
    Steps:
      1. Run: grep -n "containsDangerousChars" bridge/internal/skills/executor.go
      2. Extract the function body and catalog all characters it blocks
    Expected Result: Function found with list of blocked chars including |, &, ;, `, $, (, ), {, }, <, >
    Failure Indicators: Function not found or different name
    Evidence: .sisyphus/evidence/task-2-dangerous-chars.txt
  ```

  **Commit**: NO (read-only analysis task)

---

- [x] 3. Harvest OpenClaw + Container Findings (TS Infra + Python + SKILL.md Format)

  **What to do**:
  - Catalog OpenClaw TS skill infrastructure: `container/openclaw-src/src/agents/skills/` (~40 files, ~5,700 lines)
  - Count SKILL.md files in `container/openclaw-src/skills/` — verify 58 count
  - Assess `skill-scanner.ts` (426 lines) for security scanning completeness
  - Check `container/openclaw/skills/` Python files — verify what's runtime vs tooling
  - Verify `registry.go` YAML parser can handle actual SKILL.md frontmatter from 5+ random files
  - Test YAML frontmatter parsing: extract frontmatter from 10 SKILL.md files, validate with Python yaml
  - Catalog test coverage: count test files in `container/openclaw-src/src/agents/` that test skills
  - Check `container/openclaw-src/src/config/types.skills.ts` Zod schema completeness
  - Assess `container/openclaw-src/skills/skill-creator/` — is this the intended tool for creating skills?
  - Document the skill discovery priority order (workspace > .agents > ~ > bundled > extra)

  **Must NOT do**:
  - No modifications to any file
  - No per-file content quality review of 58 SKILL.md files (automated checks only)
  - No npm/bun install or TypeScript compilation

  **Recommended Agent Profile**:
  - **Category**: `deep`
    - Reason: Multi-language analysis (TS + Python + YAML), format spec assessment, test coverage mapping
  - **Skills**: `[]`

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 1 (with Tasks 1, 2)
  - **Blocks**: Tasks 4, 6
  - **Blocked By**: None (can start immediately)

  **References**:

  **Pattern References**:
  - `container/openclaw-src/src/agents/skills.ts` — Barrel exports for skills system
  - `container/openclaw-src/src/agents/skills/workspace.ts` — Skill snapshot building, discovery priority
  - `container/openclaw-src/src/agents/skills/filter.ts` — Skill eligibility filtering
  - `container/openclaw-src/src/agents/skills/frontmatter.ts` — SKILL.md YAML parser
  - `container/openclaw-src/src/security/skill-scanner.ts` — Security scanner (426 lines)
  - `container/openclaw-src/src/config/types.skills.ts` — Zod config schema

  **API/Type References**:
  - `container/openclaw/skills/ssl_skill_handler.py` — Runtime SSL skills
  - `container/openclaw/skills/ssl_tunnel_setup.py` — Tunnel implementations
  - `bridge/internal/skills/registry.go` — Bridge-side SKILL.md parser (lines 352-396)

  **Test References**:
  - `container/openclaw-src/src/agents/skills.e2e.test.ts` — E2E skills test
  - `container/openclaw-src/src/security/skill-scanner.test.ts` — Scanner test
  - `container/openclaw-src/src/agents/skills-install.e2e.test.ts` — Install test
  - 28+ additional test files in container/openclaw-src/src/

  **WHY Each Reference Matters**:
  - `workspace.ts` determines skill discovery order — wrong priority = wrong skills loaded
  - `frontmatter.ts` is the TS-side parser — compare with Go-side parser in registry.go
  - `skill-scanner.ts` is the security gate for skill content — gaps = malicious skill injection
  - Python runtime skills are what actually execute in containers on VPS

  **Acceptance Criteria**:

  **QA Scenarios (MANDATORY):**

  ```
  Scenario: SKILL.md count matches expected inventory
    Tool: Bash
    Preconditions: Working directory is project root
    Steps:
      1. Run: find container/openclaw-src/skills/ -name "SKILL.md" | wc -l
      2. Run: find container/openclaw-src/extensions/ -name "SKILL.md" | wc -l
      3. Run: find bridge/internal/skills/ -name "*.md" | wc -l
    Expected Result: ~58 bundled + ~5 extension + ~22 bridge browser = ~85 total
    Failure Indicators: Significant deviation from expected counts
    Evidence: .sisyphus/evidence/task-3-skill-count.txt

  Scenario: YAML frontmatter validation on sample SKILL.md files
    Tool: Bash
    Preconditions: Working directory is project root
    Steps:
      1. Extract frontmatter from 5 random SKILL.md files
      2. Validate each with: python3 -c "import yaml; yaml.safe_load(frontmatter)"
      3. Check for required fields: name, description
    Expected Result: All sampled frontmatter parses correctly and has required fields
    Failure Indicators: YAML parse errors or missing required fields
    Evidence: .sisyphus/evidence/task-3-frontmatter-validation.txt

  Scenario: Test coverage count for OpenClaw skills infrastructure
    Tool: Bash
    Preconditions: Working directory is project root
    Steps:
      1. Run: find container/openclaw-src/src/ -name "*skill*test*" -o -name "*skill*.test.ts" | wc -l
      2. Run: find container/openclaw-src/src/ -name "*skill*test*" -o -name "*skill*.test.ts" -exec wc -l {} + | tail -1
    Expected Result: ~31 test files, ~5,114 lines
    Failure Indicators: Significant deviation from expected counts
    Evidence: .sisyphus/evidence/task-3-test-coverage.txt
  ```

  **Commit**: NO (read-only analysis task)

---

- [x] 4. Synthesize All Findings Into Structured Severity Table

  **What to do**:
  - Collect raw findings from Tasks 1-3
  - Classify each finding by severity: CRITICAL, HIGH, MEDIUM, LOW
  - Assign each finding a unique ID (S1, S2, ... for security; P1, P2, ... for platform; T1, T2, ... for testing)
  - Create structured table: ID | Severity | Layer | File | Line | Description | Remediation
  - Cross-reference with previous review's 12 action items (11 still open)
  - Identify which previous items are superseded by new findings vs still valid
  - Sort by severity (CRITICAL first), then by layer, then by file
  - Produce a JSON or Markdown table consumable by Tasks 5-9

  **Must NOT do**:
  - No modifications to source files
  - No inventing findings not backed by evidence from Tasks 1-3
  - No suppressing or downgrading severity without justification

  **Recommended Agent Profile**:
  - **Category**: `ultrabrain`
    - Reason: Requires analytical rigor — classify, cross-reference, and organize 30+ findings without inventing or suppressing
  - **Skills**: `[]`

  **Parallelization**:
  - **Can Run In Parallel**: NO
  - **Parallel Group**: Wave 2 (sequential, depends on T1-T3)
  - **Blocks**: Tasks 5, 6, 7, 8
  - **Blocked By**: Tasks 1, 2, 3

  **References**:

  **Pattern References**:
  - Output from Task 1 — Deployment skill findings
  - Output from Task 2 — Bridge Go skill findings
  - Output from Task 3 — OpenClaw + Container findings

  **Test References**:
  - `DEPLOYMENT_SKILLS_REVIEW.md` — Previous review with 12 action items

  **WHY Each Reference Matters**:
  - Task outputs are the raw data — must be faithfully represented
  - Previous review action items must be tracked — don't lose institutional knowledge

  **Acceptance Criteria**:

  **QA Scenarios (MANDATORY):**

  ```
  Scenario: Findings table has minimum expected entries
    Tool: Bash
    Preconditions: Synthesized findings file exists
    Steps:
      1. Count security findings (S-prefixed IDs)
      2. Count platform findings (P-prefixed IDs)
      3. Count test gap findings (T-prefixed IDs)
      4. Verify total >= 20 findings
    Expected Result: ≥10 security, ≥5 platform, ≥5 test gap findings
    Failure Indicators: Significantly fewer findings than Metis pre-analysis identified (19 security alone)
    Evidence: .sisyphus/evidence/task-4-findings-count.txt

  Scenario: Every finding has file:line reference
    Tool: Bash
    Preconditions: Synthesized findings file exists
    Steps:
      1. Extract all file references from findings
      2. For each file reference, verify file exists: test -f <file>
    Expected Result: 100% of referenced files exist in the codebase
    Failure Indicators: Any referenced file not found
    Evidence: .sisyphus/evidence/task-4-reference-validity.txt
  ```

  **Commit**: NO (intermediate synthesis output)

---

- [x] 5. Write SKILLS_REVIEW.md — Layer 1 (Deployment) + Layer 2 (Bridge) Sections

  **What to do**:
  - Using the synthesized findings from Task 4, write Layer 1 (Deployment Skills) section of SKILLS_REVIEW.md
  - Using the synthesized findings from Task 4, write Layer 2 (Bridge Go Skills) section of SKILLS_REVIEW.md
  - Each section includes: capabilities table, expected results per skill/step, findings table, test coverage assessment
  - Format: severity-tagged findings with file:line references
  - Target: ~300 lines for both sections combined
  - Use Markdown tables for findings (ID | Severity | File | Line | Description | Remediation)

  **Must NOT do**:
  - No prose longer than 3 sentences without a table or list
  - No findings without file:line references
  - No implementing fixes

  **Recommended Agent Profile**:
  - **Category**: `writing`
    - Reason: Document production with structured formatting
  - **Skills**: `[]`

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 2 (with Tasks 6, via Task 4 dependency)
  - **Blocks**: Task 9
  - **Blocked By**: Task 4

  **References**:
  - Task 4 synthesized findings output
  - Task 1 raw findings (deployment skills)
  - Task 2 raw findings (Bridge Go skills)
  - `DEPLOYMENT_SKILLS_REVIEW.md` — Previous review for format reference

  **Acceptance Criteria**:

  **QA Scenarios (MANDATORY):**

  ```
  Scenario: Layer 1 + Layer 2 sections present in SKILLS_REVIEW.md
    Tool: Bash
    Preconditions: SKILLS_REVIEW.md exists
    Steps:
      1. Run: grep -c "## Layer 1" SKILLS_REVIEW.md
      2. Run: grep -c "## Layer 2" SKILLS_REVIEW.md
    Expected Result: Both return ≥ 1
    Failure Indicators: Either section missing
    Evidence: .sisyphus/evidence/task-5-sections-exist.txt

  Scenario: All findings have file:line references
    Tool: Bash
    Preconditions: SKILLS_REVIEW.md exists
    Steps:
      1. Extract all severity-tagged findings (CRITICAL/HIGH/MEDIUM/LOW)
      2. Count findings with file path references (pattern: `path/to/file:line`)
      3. Calculate ratio
    Expected Result: 100% of findings have file:line references
    Failure Indicators: Any finding without a verifiable file reference
    Evidence: .sisyphus/evidence/task-5-references-check.txt
  ```

  **Commit**: NO (partial deliverable, commits with Task 9)

---

- [x] 6. Write SKILLS_REVIEW.md — Layer 3 (OpenClaw) + Layer 4 (Container) + Cross-cutting Sections

  **What to do**:
  - Using the synthesized findings from Task 4, write Layer 3 (OpenClaw Skills) section
  - Write Layer 4 (Container Python Skills) section
  - Write Cross-cutting Concerns section (security patterns across all layers, testing gaps, platform claims)
  - Each section includes: capabilities table, findings, test coverage assessment
  - Target: ~250 lines for all three sections combined

  **Must NOT do**:
  - No prose longer than 3 sentences without a table or list
  - No findings without file:line references
  - No implementing fixes

  **Recommended Agent Profile**:
  - **Category**: `writing`
    - Reason: Document production with structured formatting
  - **Skills**: `[]`

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 2 (with Task 5, via Task 4 dependency)
  - **Blocks**: Task 9
  - **Blocked By**: Task 4

  **References**:
  - Task 4 synthesized findings output
  - Task 3 raw findings (OpenClaw + Container)

  **Acceptance Criteria**:

  **QA Scenarios (MANDATORY):**

  ```
  Scenario: Layer 3 + Layer 4 + Cross-cutting sections present
    Tool: Bash
    Preconditions: SKILLS_REVIEW.md exists
    Steps:
      1. Run: grep -c "## Layer 3" SKILLS_REVIEW.md
      2. Run: grep -c "## Layer 4" SKILLS_REVIEW.md
      3. Run: grep -c "## Cross-cutting" SKILLS_REVIEW.md
    Expected Result: All return ≥ 1
    Failure Indicators: Any section missing
    Evidence: .sisyphus/evidence/task-6-sections-exist.txt
  ```

  **Commit**: NO (partial deliverable, commits with Task 9)

---

- [x] 7. Write Test Execution Plan — Bridge Go Tests (Zero-Test Gap)

  **What to do**:
  - Design test plan for `bridge/internal/skills/` — currently has ZERO test files
  - For each of the 15 Go files, specify: test file name, test function names, test cases, expected results
  - Prioritize: executor.go (8-step pipeline), ssrf.go (security), policy.go (access control)
  - Estimate test complexity (lines of test code per file)
  - Specify mock strategy: how to test without external dependencies (no real email, no real Slack)
  - Include test for `containsDangerousChars` false positives (legitimate URLs/JSON blocked)
  - Include test for registry.go hand-rolled YAML parser edge cases
  - Add to SKILLS_REVIEW.md as "Test Execution Plan — Bridge Go" section

  **Must NOT do**:
  - No actual Go test code (plan only)
  - No modifications to existing files

  **Recommended Agent Profile**:
  - **Category**: `deep`
    - Reason: Requires Go testing expertise, understanding of mocking patterns, security test design
  - **Skills**: `[]`

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 3 (with Task 8)
  - **Blocks**: Task 9
  - **Blocked By**: Task 4

  **References**:

  **Pattern References**:
  - `bridge/internal/skills/executor.go` — PETG pipeline (primary test target)
  - `bridge/internal/skills/ssrf.go` — SSRF protection (security-critical)
  - `bridge/internal/skills/policy.go` — Tool policies
  - `bridge/internal/skills/registry.go` — Skill registry with custom YAML parser

  **Test References**:
  - `bridge/pkg/skills/learned_store_test.go` — Example of existing Go test style in pkg/skills/
  - `bridge/pkg/skills/extractor_test.go` — Example of table-driven test patterns

  **WHY Each Reference Matters**:
  - The executor is the security gate for all skill execution — highest priority for tests
  - SSRF protection has known bypass vectors — tests must cover IPv6, userinfo, DNS rebinding
  - Existing tests in pkg/skills/ show the project's Go test conventions

  **Acceptance Criteria**:

  **QA Scenarios (MANDATORY):**

  ```
  Scenario: Test plan covers all 15 Go files in internal/skills/
    Tool: Bash
    Preconditions: Test plan written in SKILLS_REVIEW.md
    Steps:
      1. List all .go files (excluding _test.go) in bridge/internal/skills/
      2. For each file, verify test plan mentions it
    Expected Result: All 15 files have corresponding test specifications
    Failure Indicators: Any file not mentioned in test plan
    Evidence: .sisyphus/evidence/task-7-coverage-check.txt
  ```

  **Commit**: NO (partial deliverable, commits with Task 9)

---

- [x] 8. Write Test Execution Plan — Deployment + OpenClaw + Container Tests

  **What to do**:
  - Design test plan for `.skills/` deployment YAML files (functional testing beyond structure)
  - For each deployment skill, specify: test cases per step, expected outputs, error scenarios
  - Design test for platform-claims validation (automated check that Windows claims match reality)
  - Design test for referenced script existence + bash syntax validity
  - Design test for OpenClaw SKILL.md frontmatter conformance (automated, not manual)
  - Design test for Container Python runtime skills (ssl_skill_handler.py)
  - Add to SKILLS_REVIEW.md as "Test Execution Plan — Deployment + OpenClaw + Container" section

  **Must NOT do**:
  - No actual test code (plan only)
  - No modifications to existing files

  **Recommended Agent Profile**:
  - **Category**: `unspecified-high`
    - Reason: Multi-format test planning (bash, TypeScript, Python), cross-platform validation design
  - **Skills**: `[]`

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 3 (with Task 7)
  - **Blocks**: Task 9
  - **Blocked By**: Task 4

  **References**:

  **Pattern References**:
  - `tests/test-deployment-skills.sh` — Existing structural test (build on this)
  - `.skills/deploy.yaml` — Primary deployment skill
  - `.skills/status.yaml` — Health check skill
  - `container/openclaw/skills/ssl_skill_handler.py` — Python runtime skill

  **Test References**:
  - `container/openclaw-src/src/agents/skills.e2e.test.ts` — Example OpenClaw test
  - `tests/integration/test-installer-hardening.sh` — Example bash integration test

  **WHY Each Reference Matters**:
  - Existing structural test should be extended, not replaced
  - Platform validation must be automated to prevent false Windows claims
  - Script reference checking prevents silent skill breakage

  **Acceptance Criteria**:

  **QA Scenarios (MANDATORY):**

  ```
  Scenario: Test plan covers all 4 deployment skills
    Tool: Bash
    Preconditions: Test plan written in SKILLS_REVIEW.md
    Steps:
      1. Verify test plan mentions deploy.yaml
      2. Verify test plan mentions status.yaml
      3. Verify test plan mentions cloudflare.yaml
      4. Verify test plan mentions provision.yaml
    Expected Result: All 4 skills have test specifications
    Failure Indicators: Any skill not mentioned in test plan
    Evidence: .sisyphus/evidence/task-8-deployment-coverage.txt
  ```

  **Commit**: NO (partial deliverable, commits with Task 9)

---

- [x] 9. Write SKILLS_REVIEW.md — Executive Summary + Previous Review Delta + Action Items + Final Assembly

  **What to do**:
  - Write Executive Summary section (≤50 lines): overall assessment, key metrics, top 3 risks
  - Write "Methodology" section: what was checked, what tools were used
  - Write "Previous Review Delta" section: cross-reference with DEPLOYMENT_SKILLS_REVIEW.md action items (11/12 still open), mark which are carried forward, superseded, or resolved
  - Write "Action Items" table: ID, Priority, Effort, Layer, Description — sorted by priority
  - Write "Recommendations" section: maintenance cadence, CI integration suggestions
  - Assemble all sections from Tasks 5-8 into final SKILLS_REVIEW.md
  - Add frontmatter header noting this supersedes DEPLOYMENT_SKILLS_REVIEW.md
  - Verify total document ≤ 800 lines
  - Verify all findings have file:line references

  **Must NOT do**:
  - No implementing fixes
  - No deleting DEPLOYMENT_SKILLS_REVIEW.md (mark as superseded, keep for history)
  - No prose bloat — use tables

  **Recommended Agent Profile**:
  - **Category**: `writing`
    - Reason: Document assembly, executive communication, action item prioritization
  - **Skills**: `[]`

  **Parallelization**:
  - **Can Run In Parallel**: NO
  - **Parallel Group**: Wave 3 (sequential, depends on Tasks 5-8)
  - **Blocks**: F1-F4
  - **Blocked By**: Tasks 5, 6, 7, 8

  **References**:

  **Pattern References**:
  - Task 5 output — Layer 1 + Layer 2 sections
  - Task 6 output — Layer 3 + Layer 4 + Cross-cutting sections
  - Task 7 output — Bridge Go test plan
  - Task 8 output — Deployment + OpenClaw + Container test plan
  - Task 4 output — Synthesized findings table

  **Test References**:
  - `DEPLOYMENT_SKILLS_REVIEW.md` — Previous review for delta analysis

  **WHY Each Reference Matters**:
  - The executive summary is what stakeholders read first — must be concise and accurate
  - Previous review delta prevents institutional knowledge loss
  - Action items must be concrete enough for immediate implementation

  **Acceptance Criteria**:

  **QA Scenarios (MANDATORY):**

  ```
  Scenario: SKILLS_REVIEW.md is valid Markdown
    Tool: Bash
    Preconditions: SKILLS_REVIEW.md fully assembled
    Steps:
      1. Run: pandoc SKILLS_REVIEW.md --to plain > /dev/null 2>&1; echo $?
    Expected Result: Exit code 0 (valid Markdown)
    Failure Indicators: Exit code non-zero (parsing error)
    Evidence: .sisyphus/evidence/task-9-markdown-valid.txt

  Scenario: Document within 800-line limit
    Tool: Bash
    Preconditions: SKILLS_REVIEW.md fully assembled
    Steps:
      1. Run: wc -l SKILLS_REVIEW.md
    Expected Result: ≤ 800 lines
    Failure Indicators: > 800 lines
    Evidence: .sisyphus/evidence/task-9-line-count.txt

  Scenario: All required sections present
    Tool: Bash
    Preconditions: SKILLS_REVIEW.md fully assembled
    Steps:
      1. Run: grep -c "Executive Summary" SKILLS_REVIEW.md
      2. Run: grep -c "Methodology" SKILLS_REVIEW.md
      3. Run: grep -c "Previous Review Delta" SKILLS_REVIEW.md
      4. Run: grep -c "Action Items" SKILLS_REVIEW.md
      5. Run: grep -c "Layer 1" SKILLS_REVIEW.md
      6. Run: grep -c "Layer 2" SKILLS_REVIEW.md
      7. Run: grep -c "Layer 3" SKILLS_REVIEW.md
      8. Run: grep -c "Layer 4" SKILLS_REVIEW.md
      9. Run: grep -c "Test Execution Plan" SKILLS_REVIEW.md
    Expected Result: All return ≥ 1
    Failure Indicators: Any section missing
    Evidence: .sisyphus/evidence/task-9-sections-complete.txt

  Scenario: Previous review action items cross-referenced
    Tool: Bash
    Preconditions: SKILLS_REVIEW.md fully assembled
    Steps:
      1. Run: grep -c "DEPLOYMENT_SKILLS_REVIEW" SKILLS_REVIEW.md
      2. Run: grep -c "carried forward\\|superseded\\|resolved" SKILLS_REVIEW.md
    Expected Result: ≥ 1 mention of previous review, ≥ 3 status labels
    Failure Indicators: No reference to previous review
    Evidence: .sisyphus/evidence/task-9-delta-check.txt
  ```

  **Commit**: YES
  - Message: `docs(skills): add comprehensive skills review and test execution plan`
  - Files: `SKILLS_REVIEW.md` (new)
  - Pre-commit: `pandoc SKILLS_REVIEW.md --to plain > /dev/null`

---

## Final Verification Wave (MANDATORY — after ALL implementation tasks)

> 4 review agents run in PARALLEL. ALL must APPROVE.

- [x] F1. **Plan Compliance Audit** — `oracle`
  Read the plan end-to-end. For each "Must Have": verify deliverable exists (read file, check content). For each "Must NOT Have": search output for forbidden patterns — reject if found. Check evidence files exist in .sisyphus/evidence/. Compare deliverables against plan.
  Output: `Must Have [N/N] | Must NOT Have [N/N] | Tasks [N/N] | VERDICT: APPROVE/REJECT`

- [x] F2. **Finding Accuracy Verification** — `unspecified-high`
  For every finding with a file:line reference: run `sed -n '<line>p' <file>` and verify the cited content matches the finding description. Spot-check at least 20% of findings. Flag any finding where the line content doesn't support the claim.
  Output: `Findings [N verified / N total] | Accuracy [N%] | Inaccurate [N] | VERDICT: APPROVE/REJECT`

- [x] F3. **Static QA — All References Verified** — `unspecified-high`
  Run all QA scenarios from all tasks. Verify each evidence file exists and contains expected output. Validate SKILLS_REVIEW.md renders as valid Markdown: `pandoc SKILLS_REVIEW.md --to plain > /dev/null`. Check review document ≤ 800 lines: `wc -l SKILLS_REVIEW.md`.
  Output: `Evidence [N/N] | Markdown [PASS/FAIL] | Lines [N] | VERDICT: APPROVE/REJECT`

- [x] F4. **Scope Fidelity Check** — `deep`
  For each task: read "What to do", check actual output (SKILLS_REVIEW.md, evidence files). Verify 1:1 — everything in spec was done (no missing), nothing beyond spec was done (no creep). Check "Must NOT Have" compliance — no code modifications, no live testing, no CI implementation.
  Output: `Tasks [N/N compliant] | Guardrails [CLEAN/N violations] | Unaccounted [CLEAN/N items] | VERDICT: APPROVE/REJECT`

---

## Commit Strategy

- **Single commit** after ALL tasks complete + verification passes:
  - `docs(skills): add comprehensive skills review and test execution plan`
  - Files: `SKILLS_REVIEW.md` (new)
  - Note: `DEPLOYMENT_SKILLS_REVIEW.md` is NOT deleted (historical reference) but is marked as superseded

---

## Success Criteria

### Verification Commands
```bash
# Review document exists and is valid Markdown
test -f SKILLS_REVIEW.md && pandoc SKILLS_REVIEW.md --to plain > /dev/null
echo $?  # Expected: 0

# Review document within size limit
wc -l SKILLS_REVIEW.md  # Expected: ≤ 800 lines

# All evidence files present
ls .sisyphus/evidence/task-*-*.txt | wc -l  # Expected: ≥ 9

# All referenced files exist (spot check)
grep -oP '(?<=\`).+?(?=\`)' SKILLS_REVIEW.md | head -20 | while read f; do test -f "$f"; done
```

### Final Checklist
- [ ] All "Must Have" present in SKILLS_REVIEW.md
- [ ] All "Must NOT Have" absent from deliverables
- [ ] Every finding has file:line reference
- [ ] Previous review action items cross-referenced
- [ ] Test execution plan has concrete tasks with language, file, test names
- [ ] Review document ≤ 800 lines
- [ ] All evidence files present in .sisyphus/evidence/
