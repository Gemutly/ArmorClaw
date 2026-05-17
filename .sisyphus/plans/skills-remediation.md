# Skills Remediation — Fix All CRITICAL + HIGH Findings

## TL;DR

> **Quick Summary**: Fix 14 CRITICAL+HIGH findings from SKILLS_REVIEW.md with minimal surgical patches across 7 files. Each finding gets one atomic commit. Tests deferred to separate phase.
> 
> **Deliverables**:
> - 14 atomic commits fixing all CRITICAL+HIGH findings
> - All fixes verified with `go build ./...` + `go vet ./...` (Go) and `bash -n` (YAML)
> - Bridge Go security posture: deny-by-default, SSRF-hardened, parser-fixed
> - Deployment YAMLs: version-parameterized, no runtime errors
> 
> **Estimated Effort**: Medium (14 surgical patches, ~2-3 hours execution)
> **Parallel Execution**: YES - 3 waves
> **Critical Path**: S3 → S1 → S2 (CRITICAL wave) → S6+S7 → S8 → S9+S10 → F1 → F3 (HIGH Bridge) → S4 → S5 → T2 → F2 (HIGH misc)

---

## Context

### Original Request
Fix all CRITICAL+HIGH findings from the ArmorClaw Skills Review (SKILLS_REVIEW.md, 2026-04-18) with minimal surgical patches.

### Interview Summary
**Key Discussions**:
- Scope: 14 fixes (3 CRITICAL + 11 HIGH minus T1 tests). T1 (add tests) deferred to separate phase.
- Approach: Minimal surgical patches — smallest diff per finding, no refactoring
- S2 migration: Auto-allow skills that have Policy map entries in NewPolicyEnforcer()
- S1 char list: Block shell constructs only (`|`, `;`, `` ` ``, `$(`, `${`), not structured-data chars
- S4 tag strategy: Add version parameter defaulting to main, pass VERSION=$version in SSH
- F3 scope: Excise dead ValidateParameters call, don't implement body parsing

**Research Findings**:
- yaml.v3 already in go.mod as indirect — just promote to direct for S3
- install.sh already hardened (SHA256+GPG verification) — S4 is just parameterization
- governor.Governor exists and works — F1 is 1-line wiring
- Only git tag: v0.1.0
- registry.go:143 type assertion will break with yaml.v3 — must change `map[interface{}]interface{}` → `map[string]interface{}`

### Metis Review
**Critical Gaps Identified (addressed)**:
- S3 type assertion landmine at registry.go:143 — included in S3 fix scope
- S2 breaks all skills — resolved with auto-allow from Policy map
- S1 `$` needed in URLs but dangerous in shell — resolved: block `$(` and `${` only
- S4 tag strategy — resolved: add version param, default main
- F3 scope — resolved: excise dead call
- S10 redirect bypass — added scheme check on redirect targets

---

## Work Objectives

### Core Objective
Apply 14 minimal surgical patches to fix all CRITICAL+HIGH findings, verified by `go build` + `go vet` after each commit.

### Concrete Deliverables
- 14 commits, one per finding, format: `fix(security): [ID] description`
- All Bridge Go code compiles cleanly after each commit
- Deployment YAMLs pass `bash -n` syntax check

### Definition of Done
- [ ] `cd bridge && go build ./...` → exit 0
- [ ] `cd bridge && go vet ./...` → exit 0
- [ ] 14 commits in git log matching finding IDs
- [ ] No source files modified beyond the 7 target files

### Must Have
- S3: yaml.v3 parser replaces hand-rolled, handles nested objects/lists
- S1: Shell constructs only blocked, structured data chars pass
- S2: Deny-by-default, auto-allow from Policy map
- S6: IPv6 private ranges added
- S7: extractHost uses net/url.Parse, strips userinfo
- S9: WebDAV client has 30s timeout
- S10: WebDAV requires HTTPS when Basic Auth present
- S4: deploy.yaml has version parameter, passes VERSION in SSH
- S5: status.yaml uses exit 0 not return 0

### Must NOT Have (Guardrails)
- G1: Do NOT merge PolicyEnforcer with Policy map — just read entries
- G2: Do NOT create a new SkillGate impl — wire existing Governor
- G3: Do NOT refactor the executor pipeline — touch specific lines only
- G4: Do NOT add policy entries beyond email.send
- G5: yaml.v3 import ONLY in registry.go
- G6: Do NOT change the SKILL.md file format
- G7: Each finding = exactly one commit
- G8: `go build ./...` must pass after EVERY fix
- G9: Do NOT touch test files — separate phase
- G10: Do NOT implement body parsing for F3 — excise only

---

## Verification Strategy

> **ZERO HUMAN INTERVENTION** — ALL verification is agent-executed.

### Test Decision
- **Automated tests**: None (deferred to separate phase)
- **Build verification**: `go build ./...` + `go vet ./...` after every Go commit
- **Syntax verification**: `bash -n` after YAML changes

### QA Policy
Every task includes agent-executed QA via grep + build verification.
Evidence saved to `.sisyphus/evidence/remediation/task-{N}-{slug}.txt`.

---

## Execution Strategy

### Parallel Execution Waves

```
Wave 1 (CRITICAL — 3 parallel tasks):
├── Task 1: S3 — Replace hand-rolled YAML parser with yaml.v3 [deep]
├── Task 2: S1 — Narrow containsDangerousChars to shell constructs [quick]
└── Task 3: S2 — Flip to deny-by-default, auto-allow from Policy map [quick]

Wave 2 (HIGH Bridge — 5 parallel tasks, after Wave 1 builds pass):
├── Task 4: S6+S7 — Fix SSRF: add IPv6 ranges, use url.Parse [unspecified-high]
├── Task 5: S8 — Add email.send policy entry [quick]
├── Task 6: S9+S10 — WebDAV: add timeout + HTTPS enforcement [unspecified-high]
├── Task 7: F1 — Wire default Governor as SkillGate [quick]
└── Task 8: F3 — Excise dead ValidateParameters call [quick]

Wave 3 (HIGH misc — 4 parallel tasks):
├── Task 9: S4 — Add version parameter to deploy.yaml [quick]
├── Task 10: S5 — Fix return 0 → exit 0 in status.yaml [quick]
├── Task 11: T2 — Wire allowlist remove: interface + handler + impl [deep]
└── Task 12: F2 — Remove midnight normalization from hasOverlap [quick]

Wave FINAL (After ALL fixes — 3 parallel reviews):
├── Task F1: Build + vet all Bridge code [quick]
├── Task F2: Grep-verify all 14 fixes applied [quick]
└── Task F3: Scope fidelity — no refactoring beyond spec [deep]
```

### Dependency Matrix

| Task | Blocks | Blocked By |
|------|--------|-----------|
| 1 (S3) | FINAL | - |
| 2 (S1) | FINAL | - |
| 3 (S2) | FINAL | - |
| 4 (S6+S7) | FINAL | - |
| 5 (S8) | FINAL | - |
| 6 (S9+S10) | FINAL | - |
| 7 (F1) | FINAL | - |
| 8 (F3) | FINAL | - |
| 9 (S4) | FINAL | - |
| 10 (S5) | FINAL | - |
| 11 (T2) | FINAL | - |
| 12 (F2) | FINAL | - |
| FINAL | - | ALL |

### Agent Dispatch Summary

- **Wave 1**: 3 tasks — T1 → `deep`, T2 → `quick`, T3 → `quick`
- **Wave 2**: 5 tasks — T4 → `unspecified-high`, T5 → `quick`, T6 → `unspecified-high`, T7 → `quick`, T8 → `quick`
- **Wave 3**: 4 tasks — T9 → `quick`, T10 → `quick`, T11 → `deep`, T12 → `quick`
- **FINAL**: 3 tasks — F1 → `quick`, F2 → `quick`, F3 → `deep`

---

## TODOs

- [x] 1. S3 — Replace hand-rolled YAML parser with yaml.v3

  **What to do**:
  - In `bridge/internal/skills/registry.go`: Replace `parseYAMLFrontmatter()` function body (lines 350-396) with `yaml.Unmarshal()` from `gopkg.in/yaml.v3`
  - Promote yaml.v3 from indirect to direct dependency in `bridge/go.mod`
  - Fix type assertion at line 143: change `.(map[interface{}]interface{})` to `.(map[string]interface{})` — yaml.v3 produces `map[string]interface{}`, not `map[interface{}]interface{}`
  - Fix `convertInterfaceMap()` at line 340 if it only handles `map[interface{}]interface{}` — add `map[string]interface{}` branch
  - Add import `"gopkg.in/yaml.v3"` to registry.go
  - Verify with a SKILL.md containing nested `metadata.openclaw` object — must parse correctly

  **Must NOT do**: Don't change the Skill struct. Don't change SKILL.md files. Don't touch other files.

  **Recommended Agent Profile**:
  - **Category**: `deep`
    - Reason: Parser replacement requires understanding yaml.v3 semantics, type assertion migration, and edge case handling
  - **Skills**: `[]`

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 1 (with Tasks 2, 3)
  - **Blocks**: FINAL
  - **Blocked By**: None

  **References**:
  - `bridge/internal/skills/registry.go:350-396` — Current parseYAMLFrontmatter function (REPLACE)
  - `bridge/internal/skills/registry.go:143` — Type assertion that will break: `.(map[interface{}]interface{})`
  - `bridge/internal/skills/registry.go:340` — convertInterfaceMap may need map[string]interface{} branch
  - `bridge/go.mod:117` — yaml.v3 already present as indirect

  **Acceptance Criteria**:
  - [ ] `cd bridge && go build ./...` exits 0
  - [ ] `grep -c 'parseYAMLFrontmatter' bridge/internal/skills/registry.go` — function either gone or redirects to yaml.Unmarshal
  - [ ] `grep 'map\[interface{}\]interface{}' bridge/internal/skills/registry.go` — returns 0 matches (all changed to map[string]interface{})
  - [ ] `grep 'gopkg.in/yaml.v3' bridge/go.mod` — shows yaml.v3 as direct dependency

  **QA Scenarios**:
  ```
  Scenario: SKILL.md with nested metadata.openclaw parses correctly
    Tool: Bash
    Steps:
      1. Create temp SKILL.md with: metadata: {openclaw: {requires: {bins: ["curl"]}}}
      2. Run: cd bridge && go run -exec "echo test" ./internal/skills/ (or write minimal test main)
    Expected Result: Nested metadata parsed as map, not raw string
    Evidence: .sisyphus/evidence/remediation/task-1-yaml-v3-parser.txt
  ```

  **Commit**: YES
  - Message: `fix(skills): S3 replace hand-rolled YAML parser with yaml.v3`
  - Files: `bridge/internal/skills/registry.go`, `bridge/go.mod`, `bridge/go.sum`

---

- [x] 2. S1 — Narrow containsDangerousChars to shell constructs

  **What to do**:
  - In `bridge/internal/skills/executor.go`: Replace the `dangerous` slice at line 267
  - Current: `[]string{"|", "&", ";", "`", "$", "(", ")", "{", "}", "<", ">"}`
  - New: `[]string{"|", ";", "`", "$(", "${"}`
  - This blocks actual shell injection vectors (pipe, semicolon, backtick, command substitution, variable expansion) while allowing structured data chars needed in URLs (`&`), JSON/math (`()`), HTML (`<>`), and JSON objects (`{}`)

  **Must NOT do**: Don't build a type-aware validation system. Don't add URL detection. Just narrow the list.

  **Recommended Agent Profile**:
  - **Category**: `quick`
    - Reason: Single line change in a slice literal
  - **Skills**: `[]`

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 1 (with Tasks 1, 3)
  - **Blocks**: FINAL
  - **Blocked By**: None

  **References**:
  - `bridge/internal/skills/executor.go:265-274` — containsDangerousChars function
  - `bridge/internal/skills/executor.go:257` — Call site in validateParameters

  **Acceptance Criteria**:
  - [ ] `cd bridge && go build ./...` exits 0
  - [ ] URL with `&` passes: `containsDangerousChars("https://example.com?a=1&b=2")` returns false
  - [ ] Pipe injection blocked: `containsDangerousChars("cat file | rm -rf")` returns true
  - [ ] `$(cmd)` blocked: `containsDangerousChars("$(whoami)")` returns true

  **QA Scenarios**:
  ```
  Scenario: Structured data chars pass validation
    Tool: Bash
    Steps:
      1. cd bridge && go build ./...
      2. Verify "https://api.example.com/search?q=test&page=1" passes (contains &)
      3. Verify '{"key": "value"}' passes (contains {})
    Expected Result: All return false from containsDangerousChars
    Evidence: .sisyphus/evidence/remediation/task-2-dangerous-chars.txt
  ```

  **Commit**: YES
  - Message: `fix(security): S1 narrow containsDangerousChars to shell constructs`
  - Files: `bridge/internal/skills/executor.go`

---

- [x] 3. S2 — Flip IsAllowed to deny-by-default with auto-allow

  **What to do**:
  - In `bridge/internal/skills/executor.go`:
    1. Change line 363 from `return !pe.blockedSkills[skillName]` to `return false`
    2. Modify `NewPolicyEnforcer()` to auto-populate `allowedSkills` from the `Policy` map in policy.go
    3. Import policy.go's Policy map (it's package-level, may need accessor or direct reference)
    4. For each key in Policy map: `pe.allowedSkills[key] = true`
  - This ensures skills with defined policies (weather, github, web.fetch, email.send after S8) are auto-allowed while unknown skills are denied

  **Must NOT do**: Don't merge PolicyEnforcer with Policy map. Don't add AllowSkill() calls to Register functions. Just read Policy entries in constructor.

  **Recommended Agent Profile**:
  - **Category**: `quick`
    - Reason: 2-line logic change + ~5 lines to iterate Policy map
  - **Skills**: `[]`

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 1 (with Tasks 1, 2)
  - **Blocks**: FINAL
  - **Blocked By**: None (note: S8 adds email.send policy — if S3 runs before S8, email.send won't be auto-allowed until S8 adds it)

  **References**:
  - `bridge/internal/skills/executor.go:350-364` — IsAllowed function
  - `bridge/internal/skills/executor.go:338-348` — PolicyEnforcer struct and constructor
  - `bridge/internal/skills/policy.go:20-84` — Policy map with 5 entries

  **Acceptance Criteria**:
  - [ ] `cd bridge && go build ./...` exits 0
  - [ ] `IsAllowed("weather")` returns true (has policy entry)
  - [ ] `IsAllowed("unknown_skill")` returns false
  - [ ] `IsAllowed("weather")` returns true WITHOUT calling AllowSkill

  **Commit**: YES
  - Message: `fix(security): S2 flip IsAllowed to deny-by-default with auto-allow`
  - Files: `bridge/internal/skills/executor.go`

---

- [x] 4. S6+S7 — Fix SSRF: add IPv6 ranges, use url.Parse

  **What to do**:
  - In `bridge/internal/skills/ssrf.go`:
    1. Add IPv6 CIDRs to `cidrs` slice (lines 21-29): `::1/128`, `fc00::/7`, `fe80::/10`, `::ffff:0:0/96`, `::/128`, `64:ff9b::/96`
    2. Replace `extractHost()` function (lines 112-129) with `net/url.Parse()` based implementation:
       ```go
       func (v *SSRFValidator) extractHost(urlStr string) string {
           u, err := url.Parse(urlStr)
           if err != nil { return "" }
           host := u.Hostname() // strips userinfo and port
           return host
       }
       ```
    3. Add import `"net/url"` to ssrf.go

  **Must NOT do**: Don't add DNS rebinding protection (separate concern). Don't change ValidateURL.

  **Recommended Agent Profile**:
  - **Category**: `unspecified-high`
    - Reason: Security-sensitive change needs careful edge case handling
  - **Skills**: `[]`

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 2 (with Tasks 5-8)
  - **Blocks**: FINAL
  - **Blocked By**: None

  **References**:
  - `bridge/internal/skills/ssrf.go:20-35` — current privateNetworks init
  - `bridge/internal/skills/ssrf.go:112-129` — current extractHost

  **Acceptance Criteria**:
  - [ ] `cd bridge && go build ./...` exits 0
  - [ ] `grep '::1/128' bridge/internal/skills/ssrf.go` returns 1 match
  - [ ] `extractHost("https://attacker@169.254.169.254/")` returns `"169.254.169.254"`
  - [ ] `extractHost("https://example.com:8080/path")` returns `"example.com"`

  **Commit**: YES
  - Message: `fix(security): S6+S7 add IPv6 ranges and use url.Parse in SSRF validator`
  - Files: `bridge/internal/skills/ssrf.go`

---

- [x] 5. S8 — Add email.send policy entry

  **What to do**:
  - In `bridge/internal/skills/policy.go`: Add `"email.send"` entry to Policy map (after line 83)
  - Entry should follow existing pattern:
    ```go
    "email.send": {
        Risk:           "high",
        AutoExecute:    false,
        Timeout:        30 * time.Second,
        MaxOutput:      1024,
        AllowedSchemes: []string{"https"},
    },
    ```

  **Must NOT do**: Don't add policies for other missing skills (calendar, slack, etc.)

  **Recommended Agent Profile**:
  - **Category**: `quick`
  - **Skills**: `[]`

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 2 (with Tasks 4, 6-8)
  - **Blocks**: FINAL
  - **Blocked By**: None

  **References**:
  - `bridge/internal/skills/policy.go:20-84` — existing Policy map
  - `bridge/internal/skills/policy.go:6-17` — ToolPolicy struct

  **Acceptance Criteria**:
  - [ ] `cd bridge && go build ./...` exits 0
  - [ ] `grep '"email.send"' bridge/internal/skills/policy.go` returns 1 match

  **Commit**: YES
  - Message: `fix(skills): S8 add email.send policy entry`
  - Files: `bridge/internal/skills/policy.go`

---

- [x] 6. S9+S10 — WebDAV: add timeout + HTTPS enforcement

  **What to do**:
  - In `bridge/internal/skills/webdav.go`:
    1. Create helper function:
       ```go
       func newWebDAVClient() *http.Client {
           return &http.Client{
               Timeout: 30 * time.Second,
               CheckRedirect: func(req *http.Request, via []*http.Request) error {
                   if len(via) >= 10 { return fmt.Errorf("too many redirects") }
                   return nil
               },
           }
       }
       ```
    2. Replace all 4 instances of `client := &http.Client{}` (lines 165, 254, 311, 389) with `client := newWebDAVClient()`
    3. Add HTTPS check before Basic Auth on all 4 functions: if username/password provided AND URL is http:// (not https://), return error "Basic Auth requires HTTPS"
    4. Add import `"crypto/tls"` and `"time"` if not already present

  **Must NOT do**: Don't add connection pooling, retry logic, or custom transport.

  **Recommended Agent Profile**:
  - **Category**: `unspecified-high`
  - **Skills**: `[]`

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 2 (with Tasks 4, 5, 7, 8)
  - **Blocks**: FINAL
  - **Blocked By**: None

  **References**:
  - `bridge/internal/skills/webdav.go:165, 254, 311, 389` — 4 instances of `&http.Client{}`

  **Acceptance Criteria**:
  - [ ] `cd bridge && go build ./...` exits 0
  - [ ] `grep -c 'newWebDAVClient' bridge/internal/skills/webdav.go` returns 5 (1 def + 4 calls)
  - [ ] `grep 'Timeout:' bridge/internal/skills/webdav.go` returns 1 match (in helper)

  **Commit**: YES
  - Message: `fix(security): S9+S10 add WebDAV timeout and HTTPS enforcement`
  - Files: `bridge/internal/skills/webdav.go`

---

- [x] 7. F1 — Wire default Governor as SkillGate

  **What to do**:
  - In `bridge/internal/skills/executor.go`:
    1. Find where Governor is imported/available (check `gateway.go` or similar for pattern)
    2. In `NewSkillExecutor()`, change from `SkillExecutorConfig{}` to `SkillExecutorConfig{SkillGate: governor.NewGovernor(nil, nil)}`
    3. Or add nil-check fallback in `NewSkillExecutorWithConfig`: if cfg.SkillGate is nil, create default Governor

  **Must NOT do**: Don't create a new SkillGate implementation. Don't add Governor as a hard dependency if it isn't already.

  **Recommended Agent Profile**:
  - **Category**: `quick`
  - **Skills**: `[]`

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 2 (with Tasks 4-6, 8)
  - **Blocks**: FINAL
  - **Blocked By**: None

  **References**:
  - `bridge/internal/skills/executor.go:42-58` — NewSkillExecutor / NewSkillExecutorWithConfig
  - `bridge/internal/skills/executor.go:80-91` — skillGate nil check

  **Acceptance Criteria**:
  - [ ] `cd bridge && go build ./...` exits 0
  - [ ] `NewSkillExecutor()` creates executor with non-nil skillGate

  **Commit**: YES
  - Message: `fix(skills): F1 wire default Governor as SkillGate`
  - Files: `bridge/internal/skills/executor.go`

---

- [x] 8. F3 — Excise dead ValidateParameters call

  **What to do**:
  - In `bridge/internal/skills/executor.go`: Remove the `ValidateParameters()` call from `ExecuteSkill()` (around line 103)
  - In `bridge/internal/skills/registry.go`: Remove or comment the empty `extractParametersFromBody` function (lines 225-233)
  - Remove or comment the empty `ValidateParameters` function if it only calls extractParametersFromBody

  **Must NOT do**: Don't implement body parsing. Don't add parameter validation.

  **Recommended Agent Profile**:
  - **Category**: `quick`
  - **Skills**: `[]`

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 2 (with Tasks 4-7)
  - **Blocks**: FINAL
  - **Blocked By**: None

  **References**:
  - `bridge/internal/skills/registry.go:225-233` — extractParametersFromBody (always empty)
  - `bridge/internal/skills/executor.go:~103` — ValidateParameters call

  **Acceptance Criteria**:
  - [ ] `cd bridge && go build ./...` exits 0
  - [ ] `grep 'ValidateParameters' bridge/internal/skills/executor.go` returns 0 (call removed)

  **Commit**: YES
  - Message: `fix(skills): F3 excise dead ValidateParameters call`
  - Files: `bridge/internal/skills/executor.go`, `bridge/internal/skills/registry.go`

---

- [ ] 9. S4 — Add version parameter to deploy.yaml

  **What to do**:
  - In `.skills/deploy.yaml`:
    1. Add `version` parameter to parameters section (default: `"main"`)
    2. On lines 163 and 166: Change the curl URL from hardcoded `/main/` to `/$version/` (using the parameter variable)
    3. The URL pattern should become: `https://raw.githubusercontent.com/Gemutly/ArmorClaw/$version/deploy/install.sh`

  **Must NOT do**: Don't hardcode to v0.1.0. Don't change install.sh (it's already parameterized).

  **Recommended Agent Profile**:
  - **Category**: `quick`
  - **Skills**: `[]`

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 3 (with Tasks 10-12)
  - **Blocks**: FINAL
  - **Blocked By**: None

  **References**:
  - `.skills/deploy.yaml:132` — deploy_installer step
  - `.skills/deploy.yaml:161-167` — curl commands with hardcoded /main/
  - `deploy/install.sh:12` — VERSION="${VERSION:-main}" (already parameterized)

  **Acceptance Criteria**:
  - [ ] `python3 -c "import yaml; yaml.safe_load(open('.skills/deploy.yaml'))"` exits 0
  - [ ] `grep '/main/' .skills/deploy.yaml` returns 0 matches (no hardcoded main)
  - [ ] `grep 'version' .skills/deploy.yaml` returns ≥ 1 match (new parameter)

  **Commit**: YES
  - Message: `fix(deploy): S4 add version parameter to deploy.yaml`
  - Files: `.skills/deploy.yaml`

---

- [ ] 10. S5 — Fix return 0 to exit 0 in status.yaml

  **What to do**:
  - In `.skills/status.yaml`: Change line 150 from `return 0` to `exit 0`

  **Must NOT do**: Don't change any other lines.

  **Recommended Agent Profile**:
  - **Category**: `quick`
  - **Skills**: `[]`

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 3 (with Tasks 9, 11, 12)
  - **Blocks**: FINAL
  - **Blocked By**: None

  **References**:
  - `.skills/status.yaml:150` — `return 0` in check_ssl step

  **Acceptance Criteria**:
  - [ ] `grep 'return 0' .skills/status.yaml` returns 0 matches
  - [ ] `grep 'exit 0' .skills/status.yaml` returns ≥ 1 match

  **Commit**: YES
  - Message: `fix(deploy): S5 fix return 0 to exit 0 in status.yaml`
  - Files: `.skills/status.yaml`

---

- [ ] 11. T2 — Wire allowlist remove to AllowlistManager

  **What to do**:
  - This is a 3-file fix:
    1. `bridge/pkg/rpc/server.go` (line 74-84): Add `RemoveAllowedIP(ip string) error` and `RemoveAllowedCIDR(cidr string) error` to `SkillManager` interface
    2. `bridge/pkg/rpc/methods_skills.go` (lines 350-356): Wire the switch cases to call `s.skillMgr.RemoveAllowedIP(params.Value)` and `s.skillMgr.RemoveAllowedCIDR(params.Value)`
    3. Find the concrete type implementing SkillManager (likely a wrapper struct) and add the two methods, delegating to `AllowlistManager.RemoveAllowedIP` / `RemoveAllowedCIDR`
  - Use `lsp_find_references` on SkillManager to find all implementations

  **Must NOT do**: Don't add error handling for not-found items. Don't add validation. Just wire the existing methods.

  **Recommended Agent Profile**:
  - **Category**: `deep`
    - Reason: Interface change requires finding all implementations and updating each
  - **Skills**: `[]`

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 3 (with Tasks 9, 10, 12)
  - **Blocks**: FINAL
  - **Blocked By**: None

  **References**:
  - `bridge/pkg/rpc/server.go:74-84` — SkillManager interface
  - `bridge/pkg/rpc/methods_skills.go:319-370` — handleSkillsAllowlistRemove stub
  - `bridge/internal/skills/allowlist.go:80-104` — RemoveAllowedIP / RemoveAllowedCIDR methods

  **Acceptance Criteria**:
  - [ ] `cd bridge && go build ./...` exits 0
  - [ ] `grep 'RemoveAllowedIP' bridge/pkg/rpc/server.go` returns 1 match (interface)
  - [ ] `grep 'RemoveAllowedIP' bridge/pkg/rpc/methods_skills.go` returns 1 match (wired)

  **Commit**: YES
  - Message: `fix(skills): T2 wire allowlist remove to AllowlistManager`
  - Files: `bridge/pkg/rpc/server.go`, `bridge/pkg/rpc/methods_skills.go`, + concrete impl file

---

- [ ] 12. F2 — Remove midnight normalization from hasOverlap

  **What to do**:
  - In `bridge/internal/skills/calendar.go`: Remove the 4 Date() normalization lines (387-390)
  - Change from:
    ```go
    start1 = time.Date(start1.Year(), start1.Month(), start1.Day(), 0, 0, 0, 0, start1.Location())
    end1 = time.Date(end1.Year(), end1.Month(), end1.Day(), 0, 0, 0, 0, end1.Location())
    start2 = time.Date(start2.Year(), start2.Month(), start2.Day(), 0, 0, 0, 0, start2.Location())
    end2 = time.Date(end2.Year(), end2.Month(), end2.Day(), 0, 0, 0, 0, end2.Location())
    ```
  - To: (delete those 4 lines, keep the return statement)
  - The overlap check `start1.Before(end2) && end1.After(start2)` works correctly with raw times

  **Must NOT do**: Don't add time-of-day validation. Don't change the function signature.

  **Recommended Agent Profile**:
  - **Category**: `quick`
  - **Skills**: `[]`

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 3 (with Tasks 9-11)
  - **Blocks**: FINAL
  - **Blocked By**: None

  **References**:
  - `bridge/internal/skills/calendar.go:386-395` — hasOverlap function

  **Acceptance Criteria**:
  - [ ] `cd bridge && go build ./...` exits 0
  - [ ] `grep 'time.Date' bridge/internal/skills/calendar.go` — hasOverlap no longer normalizes
  - [ ] Same-day events with different hours can overlap correctly

  **Commit**: YES
  - Message: `fix(skills): F2 remove midnight normalization from hasOverlap`
  - Files: `bridge/internal/skills/calendar.go`

---

## Final Verification Wave

- [ ] F1. **Build + Vet Verification** — `quick`
  Run `cd bridge && go build ./...` and `cd bridge && go vet ./...`. Verify zero errors, zero warnings. Run `bash -n .skills/deploy.yaml .skills/status.yaml` (if YAML commands are extractable).
  Output: `Build [PASS/FAIL] | Vet [PASS/FAIL] | VERDICT`

- [ ] F2. **Fix Verification Audit** — `quick`
  For each of the 14 fixes: grep for the old pattern (should NOT be found) and the new pattern (SHOULD be found). Verify commit messages match finding IDs.
  Output: `Fixes [14/14 verified] | Commits [14 matching IDs] | VERDICT`

- [ ] F3. **Scope Fidelity Check** — `deep`
  Verify no refactoring occurred beyond the minimal patches. Check that no files were modified beyond the 7 target files. Verify each commit touches exactly the files specified in the task.
  Output: `Files [7 target / 7 modified] | Guardrails [CLEAN/N violations] | VERDICT`

---

## Commit Strategy

Each task creates one atomic commit:
- T1: `fix(skills): S3 replace hand-rolled YAML parser with yaml.v3`
- T2: `fix(security): S1 narrow containsDangerousChars to shell constructs`
- T3: `fix(security): S2 flip IsAllowed to deny-by-default with auto-allow`
- T4: `fix(security): S6+S7 add IPv6 ranges and use url.Parse in SSRF validator`
- T5: `fix(skills): S8 add email.send policy entry`
- T6: `fix(security): S9+S10 add WebDAV timeout and HTTPS enforcement`
- T7: `fix(skills): F1 wire default Governor as SkillGate`
- T8: `fix(skills): F3 excise dead ValidateParameters call`
- T9: `fix(deploy): S4 add version parameter to deploy.yaml`
- T10: `fix(deploy): S5 fix return 0 to exit 0 in status.yaml`
- T11: `fix(skills): T2 wire allowlist remove to AllowlistManager`
- T12: `fix(skills): F2 remove midnight normalization from hasOverlap`

---

## Success Criteria

### Verification Commands
```bash
cd bridge && go build ./...  # Expected: exit 0
cd bridge && go vet ./...    # Expected: exit 0
git log --oneline -14        # Expected: 14 commits with finding IDs
```

### Final Checklist
- [ ] All 14 findings fixed with atomic commits
- [ ] `go build ./...` passes
- [ ] `go vet ./...` passes
- [ ] No refactoring beyond minimal patches
- [ ] No test files touched
- [ ] S3: yaml.v3 parses nested metadata.openclaw objects
- [ ] S2: IsAllowed("unknown") returns false
- [ ] S1: URLs with & pass, pipes blocked
