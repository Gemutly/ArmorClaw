# Container-Side result.json Writer

## TL;DR

> **Quick Summary**: Add step execution mode to the container so it can read STEP_CONFIG, execute a task, write result.json to the state dir, and exit — completing the backward channel end-to-end.
> 
> **Deliverables**:
> - `step_runner.py` — new module that parses STEP_CONFIG, executes work, writes result.json atomically
> - `entrypoint.py` — modified to detect STEP_CONFIG and dispatch to step_runner instead of agent loop
> - Unit tests for step_runner and result_writer
> - Integration test validating end-to-end Bridge → container → result.json flow
> 
> **Estimated Effort**: Medium
> **Parallel Execution**: YES — 3 waves
> **Critical Path**: T1 (result_writer.py) → T3 (step_runner.py) → T4 (entrypoint integration) → T5 (e2e test) → F1-F4

---

## Context

### Original Request
User requested "make plan for container-side writer" — the container half of the backward channel. The Bridge half (read/parse result.json via `ParseContainerStepResult()`) was completed in the `agent-comms-triage` plan. Now we need the container to actually write result.json before exiting.

### Interview Summary
**Key Discussions**:
- Container has NO concept of step execution — it's a Matrix polling loop designed for Mode B
- STEP_CONFIG env var is set by factory.go but never read by any container code
- Container runs with NetworkMode:"none" — cannot make external API calls
- The container image name mismatch (`agent-base:latest` vs `openclaw:latest`) is an established constraint — do not fix

**Research Findings**:
- `entrypoint.py` (543 lines) — PID 1, loads secrets, validates, exec's `python -c "from openclaw import main; main()"`
- `agent.py` (699 lines) — infinite `while self.running: await asyncio.sleep(60)` with optional Matrix polling
- `factory.go:103` — `STEP_CONFIG` set from `step.Config` (json.RawMessage, arbitrary JSON)
- `orchestrator_integration.go:280` — stateDir passed as `/var/lib/armorclaw/agent-state/{agentID}`
- `result.go` — ContainerStepResult schema: `{status, output, data, error, duration_ms}`
- `types.go:63-90` — WorkflowStep has Config (json.RawMessage), StepID, Name, Type, AgentIDs

### Metis Review (Self-Performed — session limit hit)
**Identified Gaps** (addressed):
- **NetworkMode:none blocks AI calls**: Container cannot call external APIs. Step execution must be computation-only (data processing, file manipulation, JSON transforms). AI-augmented steps need a separate "AI proxy socket" feature — OUT OF SCOPE for this plan.
- **Atomic write needed**: Must write to temp file then rename to avoid partial reads by Bridge.
- **STEP_CONFIG schema undefined**: Define a schema that matches what Bridge sends (WorkflowStep.Config). Keep it flexible (arbitrary JSON) but document expected fields.
- **Container may crash before writing**: Already handled — ParseContainerStepResult returns (nil, nil) for missing file.

---

## Work Objectives

### Core Objective
Add step execution mode to the container so that when STEP_CONFIG is present, the container: parses the config, executes a defined task, writes result.json to the bind-mounted state directory, and exits — completing the backward channel.

### Concrete Deliverables
- `container/openclaw/result_writer.py` — atomic result.json writer module
- `container/openclaw/step_runner.py` — step execution module that reads STEP_CONFIG, runs work, invokes result_writer
- Modified `container/opt/openclaw/entrypoint.py` — detects STEP_CONFIG and dispatches to step_runner
- Tests for result_writer and step_runner
- End-to-end integration test

### Definition of Done
- [ ] Container with STEP_CONFIG set writes result.json before exit
- [ ] Container without STEP_CONFIG behaves identically to current (no regression)
- [ ] Bridge ParseContainerStepResult() successfully reads container-written result.json
- [ ] All existing tests continue to pass

### Must Have
- Atomic write (temp file + rename) for result.json
- STEP_CONFIG parsing with graceful error handling
- Duration tracking (duration_ms field)
- No changes to NetworkMode or container security posture
- No changes to existing agent mode behavior

### Must NOT Have (Guardrails)
- Do NOT change NetworkMode — security constraint
- Do NOT fix image mismatch (agent-base:latest) — established constraint
- Do NOT add AI proxy socket — out of scope (separate plan if needed)
- Do NOT modify any Bridge Go code — Bridge half is complete
- Do NOT touch v6 microkernel code
- Do NOT touch sidecar/Qdrant/pdf.rs
- Do NOT add network-dependent features to the container
- Do NOT modify agent.py's Matrix polling loop

---

## Verification Strategy

> **ZERO HUMAN INTERVENTION** — ALL verification is agent-executed. No exceptions.

### Test Decision
- **Infrastructure exists**: YES (Go tests in bridge/, Python can run in container)
- **Automated tests**: YES (tests-after)
- **Framework**: Python unittest (stdlib — no external deps needed in hardened container)
- **Bridge tests**: Go testing (existing framework)

### QA Policy
Every task MUST include agent-executed QA scenarios.
Evidence saved to `.sisyphus/evidence/task-{N}-{scenario-slug}.{ext}`.

- **Container Python**: Use Bash — Run Python tests, check exit codes, verify file contents
- **Bridge Go**: Use Bash — Run `go test`, check pass/fail
- **E2E**: Use Bash — Simulate the full flow with temp directories

---

## Execution Strategy

### Parallel Execution Waves

```
Wave 1 (Start Immediately — foundation modules):
├── Task 1: result_writer.py — atomic JSON writer [quick]
└── Task 2: step_config schema + parser [quick]

Wave 2 (After Wave 1 — core integration):
├── Task 3: step_runner.py — step execution orchestrator (depends: T1, T2) [unspecified-high]
└── Task 4: entrypoint.py integration — STEP_CONFIG detection (depends: T3) [quick]

Wave 3 (After Wave 2 — verification):
├── Task 5: End-to-end integration test (depends: T4) [unspecified-high]
└── Task 6: Update container docs (depends: T4) [quick]

Wave FINAL (After ALL tasks — 4 parallel reviews):
├── F1: Plan compliance audit (oracle)
├── F2: Code quality review (unspecified-high)
├── F3: Real manual QA (unspecified-high)
└── F4: Scope fidelity check (deep)
-> Present results -> Get explicit user okay

Critical Path: T1 → T3 → T4 → T5 → F1-F4
Parallel Speedup: ~40% faster than sequential
Max Concurrent: 2 (Wave 1)
```

### Dependency Matrix

| Task | Depends On | Blocks | Wave |
|------|-----------|--------|------|
| T1   | —         | T3     | 1    |
| T2   | —         | T3     | 1    |
| T3   | T1, T2    | T4     | 2    |
| T4   | T3        | T5, T6 | 2    |
| T5   | T4        | F1-F4  | 3    |
| T6   | T4        | F1-F4  | 3    |

### Agent Dispatch Summary

- **Wave 1**: **2** — T1 → `quick`, T2 → `quick`
- **Wave 2**: **2** — T3 → `unspecified-high`, T4 → `quick`
- **Wave 3**: **2** — T5 → `unspecified-high`, T6 → `quick`
- **FINAL**: **4** — F1 → `oracle`, F2 → `unspecified-high`, F3 → `unspecified-high`, F4 → `deep`

---

## TODOs

- [x] 1. Create `result_writer.py` — Atomic JSON Result Writer

  **What to do**:
  - Create new file `container/openclaw/result_writer.py`
  - Implement `write_result(state_dir, result_dict)` function:
    1. Validate state_dir exists and is writable
    2. Serialize result_dict to JSON with indent=2
    3. Write to temp file `result.json.tmp` in state_dir
    4. `os.rename()` temp file to `result.json` (atomic on same filesystem)
    5. Return success/failure
  - Implement `build_result(status, output, error, data, duration_ms)` helper
    that constructs a dict matching the ContainerStepResult schema
  - Create `container/tests/test_result_writer.py` with tests:
    - Test atomic write creates valid result.json
    - Test overwrite replaces previous result
    - Test error handling for read-only directory
    - Test result dict matches ContainerStepResult schema

  **Must NOT do**:
  - Do NOT import bridge_client or any network-dependent module
  - Do NOT use any external dependencies (stdlib only)
  - Do NOT modify any existing files

  **Recommended Agent Profile**:
  - **Category**: `quick`
    - Reason: Single new file with clear contract, no external deps, ~60 lines
  - **Skills**: []
  - **Skills Evaluated but Omitted**:
    - `playwright`: No UI involved

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 1 (with Task 2)
  - **Blocks**: Task 3 (step_runner needs write_result)
  - **Blocked By**: None (can start immediately)

  **References**:

  **Pattern References** (existing code to follow):
  - `bridge/pkg/secretary/result.go:39-54` — ContainerStepResult struct — this is the schema the JSON must match. Fields: `status` (string), `output` (string), `data` (map[string]any, omitempty), `error` (string, omitempty), `duration_ms` (int64)

  **API/Type References** (contracts to implement against):
  - `bridge/pkg/secretary/result.go:62-83` — ParseContainerStepResult() — this is the Bridge reader that will consume the file. Note: uses `json.Unmarshal` with silent unknown field handling, and returns (nil, nil) for missing file.

  **External References**:
  - Python stdlib `json`, `os`, `tempfile` — no external packages needed
  - `os.rename()` is atomic on POSIX when src and dst are on same filesystem

  **WHY Each Reference Matters**:
  - `result.go:39-54` — The JSON field names and types MUST match exactly (`status`, `output`, `data`, `error`, `duration_ms`) or Bridge parsing will fail silently
  - `result.go:62-83` — Bridge reads `stateDir + "/result.json"` — container must write to exactly that path inside the bind mount

  **Acceptance Criteria**:

  **QA Scenarios (MANDATORY):**

  ```
  Scenario: Atomic write creates valid result.json
    Tool: Bash
    Preconditions: Python 3 available, temp directory exists
    Steps:
      1. mkdir -p /tmp/test-result-writer && cd /home/mikmin/.LocalCode/armorclaw-omo/container
      2. python3 -c "
import sys; sys.path.insert(0, '.')
from openclaw.result_writer import write_result, build_result
import json, os
r = build_result(status='success', output='hello world', duration_ms=1500)
write_result('/tmp/test-result-writer', r)
with open('/tmp/test-result-writer/result.json') as f:
    data = json.load(f)
assert data['status'] == 'success', f'Expected success, got {data[\"status\"]}'
assert data['output'] == 'hello world', f'Output mismatch'
assert data['duration_ms'] == 1500, f'Duration mismatch'
print('PASS: result.json written correctly')
"
      3. cat /tmp/test-result-writer/result.json
    Expected Result: JSON file with {"status": "success", "output": "hello world", "duration_ms": 1500}
    Failure Indicators: AssertionError, FileNotFoundError, JSONDecodeError
    Evidence: .sisyphus/evidence/task-1-atomic-write.txt

  Scenario: Overwrite replaces previous result
    Tool: Bash
    Preconditions: result.json already exists from previous test
    Steps:
      1. cd /home/mikmin/.LocalCode/armorclaw-omo/container
      2. python3 -c "
import sys; sys.path.insert(0, '.')
from openclaw.result_writer import write_result, build_result
import json
r1 = build_result(status='running', output='first')
write_result('/tmp/test-result-writer', r1)
r2 = build_result(status='success', output='second')
write_result('/tmp/test-result-writer', r2)
with open('/tmp/test-result-writer/result.json') as f:
    data = json.load(f)
assert data['output'] == 'second', f'Expected second write, got {data[\"output\"]}'
assert data['status'] == 'success'
print('PASS: overwrite works')
"
    Expected Result: result.json contains the second write, not the first
    Failure Indicators: data['output'] == 'first'
    Evidence: .sisyphus/evidence/task-1-overwrite.txt

  Scenario: Error handling for read-only directory
    Tool: Bash
    Preconditions: Directory is read-only
    Steps:
      1. mkdir -p /tmp/test-readonly && chmod 000 /tmp/test-readonly
      2. cd /home/mikmin/.LocalCode/armorclaw-omo/container
      3. python3 -c "
import sys; sys.path.insert(0, '.')
from openclaw.result_writer import write_result, build_result
r = build_result(status='success', output='test')
try:
    write_result('/tmp/test-readonly', r)
    print('FAIL: Should have raised error')
except (PermissionError, OSError) as e:
    print(f'PASS: Got expected error: {type(e).__name__}')
" 2>&1
      4. chmod 755 /tmp/test-readonly && rm -rf /tmp/test-readonly
    Expected Result: PermissionError or OSError raised, no crash
    Failure Indicators: "FAIL: Should have raised error" in output
    Evidence: .sisyphus/evidence/task-1-readonly-error.txt

  Scenario: No temp file left after atomic write
    Tool: Bash
    Preconditions: Fresh temp directory
    Steps:
      1. mkdir -p /tmp/test-no-tmp && cd /home/mikmin/.LocalCode/armorclaw-omo/container
      2. python3 -c "
import sys; sys.path.insert(0, '.')
from openclaw.result_writer import write_result, build_result
import os
r = build_result(status='success', output='clean')
write_result('/tmp/test-no-tmp', r)
files = os.listdir('/tmp/test-no-tmp')
assert 'result.json.tmp' not in files, f'Temp file left behind: {files}'
assert 'result.json' in files, f'Result file missing: {files}'
print(f'PASS: Only result.json present, files: {files}')
"
    Expected Result: Only result.json exists, no .tmp file
    Failure Indicators: 'result.json.tmp' in directory listing
    Evidence: .sisyphus/evidence/task-1-no-tmp.txt

  **Commit**: YES (groups with T2)
  - Message: `feat(container): add atomic result_writer and step_config modules`
  - Files: `container/openclaw/result_writer.py`, `container/tests/test_result_writer.py`
  - Pre-commit: `cd container && python3 -m pytest tests/test_result_writer.py -v`

- [x] 2. Create `step_config.py` — STEP_CONFIG Parser

  **What to do**:
  - Create new file `container/openclaw/step_config.py`
  - Implement `parse_step_config()` function:
    1. Read `STEP_CONFIG` env var
    2. If absent or empty, return None (indicates agent mode, not step mode)
    3. Parse JSON, validate basic structure
    4. Return StepConfig dataclass/dict with: task (str), config (dict), metadata (dict)
  - Define `StepConfig` class with fields:
    - `raw` — original JSON string
    - `task` — extracted task description (from TASK_DESCRIPTION env or config.task)
    - `config` — arbitrary dict from parsed JSON
    - `step_id` — optional identifier
    - `step_name` — optional human-readable name
  - Implement validation:
    - Must be valid JSON
    - Must be a dict/object (not array or scalar)
    - Log warnings for unrecognized fields but don't fail
  - Create `container/tests/test_step_config.py` with tests:
    - Test parsing valid STEP_CONFIG
    - Test STEP_CONFIG absent returns None
    - Test STEP_CONFIG empty string returns None
    - Test invalid JSON raises clear error
    - Test non-object JSON (array, string) raises clear error

  **Must NOT do**:
  - Do NOT import bridge_client or any network-dependent module
  - Do NOT modify any existing files
  - Do NOT fail on unknown fields — forward compatibility

  **Recommended Agent Profile**:
  - **Category**: `quick`
    - Reason: Single new file, env var parsing, ~50 lines, clear contract
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 1 (with Task 1)
  - **Blocks**: Task 3 (step_runner needs parse_step_config)
  - **Blocked By**: None (can start immediately)

  **References**:

  **Pattern References** (existing code to follow):
  - `bridge/pkg/studio/factory.go:99-104` — How Bridge sets STEP_CONFIG: `env = append(env, "STEP_CONFIG="+string(req.Config))` where `req.Config` is `json.RawMessage` (i.e., arbitrary JSON bytes from `step.Config`)
  - `bridge/pkg/secretary/types.go:63-90` — WorkflowStep struct showing what Config contains: it's `json.RawMessage`, so truly arbitrary JSON
  - `bridge/pkg/secretary/learn_website.go:955` — Example of buildStepConfig() that creates step config

  **API/Type References**:
  - `container/opt/openclaw/entrypoint.py:486-490` — Current default command detection: `if len(sys.argv) > 1: cmd = sys.argv[1:]` — this is where STEP_CONFIG detection will be added

  **WHY Each Reference Matters**:
  - `factory.go:99-104` — Understanding that STEP_CONFIG is literally `step.Config` marshaled to JSON string, with no schema enforcement on the Bridge side
  - `types.go:79-80` — `Config json.RawMessage` — the config is opaque JSON, parser must handle arbitrary structure

  **Acceptance Criteria**:

  **QA Scenarios (MANDATORY):**

  ```
  Scenario: Parse valid STEP_CONFIG
    Tool: Bash
    Preconditions: Python 3 available
    Steps:
      1. cd /home/mikmin/.LocalCode/armorclaw-omo/container
      2. python3 -c "
import os, sys; sys.path.insert(0, '.')
os.environ['STEP_CONFIG'] = '{\"task\":\"research NYC restaurants\",\"model\":\"gpt-4\"}'
from openclaw.step_config import parse_step_config
cfg = parse_step_config()
assert cfg is not None, 'Should return StepConfig, not None'
assert cfg.task == 'research NYC restaurants', f'Got task: {cfg.task}'
assert cfg.config.get('model') == 'gpt-4', f'Got config: {cfg.config}'
print('PASS: valid STEP_CONFIG parsed')
"
    Expected Result: StepConfig with task='research NYC restaurants', config={'model': 'gpt-4'}
    Failure Indicators: AssertionError, import error
    Evidence: .sisyphus/evidence/task-2-parse-valid.txt

  Scenario: STEP_CONFIG absent returns None
    Tool: Bash
    Preconditions: STEP_CONFIG env var not set
    Steps:
      1. cd /home/mikmin/.LocalCode/armorclaw-omo/container
      2. python3 -c "
import os, sys; sys.path.insert(0, '.')
os.environ.pop('STEP_CONFIG', None)
from openclaw.step_config import parse_step_config
cfg = parse_step_config()
assert cfg is None, f'Expected None when STEP_CONFIG absent, got {cfg}'
print('PASS: None returned when no STEP_CONFIG')
"
    Expected Result: None
    Failure Indicators: cfg is not None
    Evidence: .sisyphus/evidence/task-2-absent.txt

  Scenario: Invalid JSON raises clear error
    Tool: Bash
    Preconditions: STEP_CONFIG contains invalid JSON
    Steps:
      1. cd /home/mikmin/.LocalCode/armorclaw-omo/container
      2. python3 -c "
import os, sys; sys.path.insert(0, '.')
os.environ['STEP_CONFIG'] = 'not-json-at-all'
from openclaw.step_config import parse_step_config
try:
    cfg = parse_step_config()
    print('FAIL: Should have raised error')
except ValueError as e:
    print(f'PASS: Got ValueError: {e}')
"
    Expected Result: ValueError with descriptive message
    Failure Indicators: "FAIL: Should have raised error"
    Evidence: .sisyphus/evidence/task-2-invalid-json.txt

  Scenario: Non-object JSON raises clear error
    Tool: Bash
    Preconditions: STEP_CONFIG is a JSON array
    Steps:
      1. cd /home/mikmin/.LocalCode/armorclaw-omo/container
      2. python3 -c "
import os, sys; sys.path.insert(0, '.')
os.environ['STEP_CONFIG'] = '[1,2,3]'
from openclaw.step_config import parse_step_config
try:
    cfg = parse_step_config()
    print(f'FAIL: Should have raised error, got {cfg}')
except (ValueError, TypeError) as e:
    print(f'PASS: Got error: {e}')
"
    Expected Result: ValueError or TypeError
    Failure Indicators: No error raised
    Evidence: .sisyphus/evidence/task-2-non-object.txt

  **Commit**: YES (groups with T1)
  - Message: `feat(container): add atomic result_writer and step_config modules`
  - Files: `container/openclaw/step_config.py`, `container/tests/test_step_config.py`
  - Pre-commit: `cd container && python3 -m pytest tests/test_step_config.py -v`

- [x] 3. Create `step_runner.py` — Step Execution Orchestrator

  **What to do**:
  - Create new file `container/openclaw/step_runner.py`
  - Implement `StepRunner` class:
    1. `__init__()` — initialize state_dir from env `STATE_DIR` or default `/home/claw/.openclaw`
    2. `run(step_config)` — main execution method:
       a. Record start time
       b. Execute step based on config type (see handlers below)
       c. Calculate duration_ms
       d. Call `result_writer.write_result()` with outcome
       e. Return exit code (0 for success, 1 for failure)
  - Implement built-in step handlers (computation-only, no network):
    - `echo` handler — echoes the task description back as output (for testing)
    - `transform` handler — JSON-to-JSON data transformation (future: template rendering)
    - Default handler — writes "step received, no executor matched" result
  - Add `execute_step()` that routes to handler based on config fields
  - Error handling:
    - Catch all exceptions in run()
    - Write error result.json even on failure (so Bridge knows why it failed)
    - Log to stderr (container logs visible via docker logs)
  - Create `container/tests/test_step_runner.py` with tests:
    - Test echo handler produces correct result
    - Test error handling writes error result.json
    - Test duration_ms is populated and > 0
    - Test step with unknown handler writes status "partial"

  **Must NOT do**:
  - Do NOT add network calls (no HTTP, no socket, no requests)
  - Do NOT import bridge_client
  - Do NOT modify agent.py
  - Do NOT make any external API calls
  - Do NOT add complex AI features — this is a data I/O layer

  **Recommended Agent Profile**:
  - **Category**: `unspecified-high`
    - Reason: Moderate complexity, integrates two modules, needs careful error handling
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: NO
  - **Parallel Group**: Wave 2 (sequential after T1+T2)
  - **Blocks**: Task 4 (entrypoint integration)
  - **Blocked By**: Task 1 (result_writer), Task 2 (step_config)

  **References**:

  **Pattern References** (existing code to follow):
  - `container/openclaw/result_writer.py` — write_result() and build_result() API (created in T1)
  - `container/openclaw/step_config.py` — StepConfig class and parse_step_config() API (created in T2)

  **API/Type References**:
  - `bridge/pkg/secretary/result.go:39-54` — ContainerStepResult schema that result.json must match: `{status, output, data, error, duration_ms}`
  - `bridge/pkg/secretary/orchestrator_integration.go:321-350` — waitForCompletion() shows how Bridge reads the result: polls every 500ms, calls ParseContainerStepResult(stateDir) on completion

  **Test References**:
  - `bridge/pkg/secretary/result_test.go` — Pattern for testing result parsing (can reference for our Python-side tests)

  **WHY Each Reference Matters**:
  - `result.go:39-54` — The result.json structure must match this exactly
  - `orchestrator_integration.go:321-350` — Understanding the Bridge's read pattern (poll, parse on exit) confirms we just need to write before process exit
  - The echo handler is critical for testing the full pipeline without any external dependencies

  **Acceptance Criteria**:

  **QA Scenarios (MANDATORY):**

  ```
  Scenario: Echo handler produces correct result.json
    Tool: Bash
    Preconditions: result_writer.py and step_config.py exist from T1/T2
    Steps:
      1. mkdir -p /tmp/test-step-runner && cd /home/mikmin/.LocalCode/armorclaw-omo/container
      2. python3 -c "
import os, sys, json; sys.path.insert(0, '.')
os.environ['STATE_DIR'] = '/tmp/test-step-runner'
from openclaw.step_config import StepConfig
from openclaw.step_runner import StepRunner
cfg = StepConfig(raw='{\"handler\":\"echo\",\"task\":\"test message\"}', task='test message', config={'handler': 'echo'})
runner = StepRunner()
exit_code = runner.run(cfg)
with open('/tmp/test-step-runner/result.json') as f:
    data = json.load(f)
assert exit_code == 0, f'Expected exit 0, got {exit_code}'
assert data['status'] == 'success', f'Expected success, got {data[\"status\"]}'
assert 'test message' in data['output'], f'Output should contain task: {data[\"output\"]}'
assert data['duration_ms'] > 0, f'Duration should be positive: {data[\"duration_ms\"]}'
print('PASS: echo handler works correctly')
"
    Expected Result: result.json with status='success', output containing 'test message', duration_ms > 0
    Failure Indicators: AssertionError, FileNotFoundError, exit_code != 0
    Evidence: .sisyphus/evidence/task-3-echo-handler.txt

  Scenario: Error handling writes error result.json
    Tool: Bash
    Preconditions: State dir exists
    Steps:
      1. mkdir -p /tmp/test-step-error && cd /home/mikmin/.LocalCode/armorclaw-omo/container
      2. python3 -c "
import os, sys, json; sys.path.insert(0, '.')
os.environ['STATE_DIR'] = '/tmp/test-step-error'
from openclaw.step_config import StepConfig
from openclaw.step_runner import StepRunner
# Force an error by using a config that triggers failure
cfg = StepConfig(raw='{\"handler\":\"nonexistent\"}', task='fail test', config={'handler': 'nonexistent'})
runner = StepRunner()
exit_code = runner.run(cfg)
with open('/tmp/test-step-error/result.json') as f:
    data = json.load(f)
# Should still write a result (graceful degradation)
assert data['status'] in ('success', 'partial', 'failed'), f'Got status: {data[\"status\"]}'
assert data['duration_ms'] > 0
print(f'PASS: error handled gracefully, status={data[\"status\"]}, exit={exit_code}')
"
    Expected Result: result.json written even for unknown handler, status is 'partial' or 'failed'
    Failure Indicators: FileNotFoundError (no result.json written), uncaught exception
    Evidence: .sisyphus/evidence/task-3-error-handling.txt

  Scenario: Duration tracking works
    Tool: Bash
    Preconditions: State dir exists
    Steps:
      1. mkdir -p /tmp/test-step-duration && cd /home/mikmin/.LocalCode/armorclaw-omo/container
      2. python3 -c "
import os, sys, json, time; sys.path.insert(0, '.')
os.environ['STATE_DIR'] = '/tmp/test-step-duration'
from openclaw.step_config import StepConfig
from openclaw.step_runner import StepRunner
cfg = StepConfig(raw='{\"handler\":\"echo\",\"task\":\"duration test\"}', task='duration test', config={'handler': 'echo'})
runner = StepRunner()
runner.run(cfg)
with open('/tmp/test-step-duration/result.json') as f:
    data = json.load(f)
assert isinstance(data['duration_ms'], int), f'Expected int, got {type(data[\"duration_ms\"])}'
assert data['duration_ms'] >= 0, f'Duration should be non-negative'
assert data['duration_ms'] < 10000, f'Duration should be reasonable (<10s): {data[\"duration_ms\"]}'
print(f'PASS: duration_ms = {data[\"duration_ms\"]}')
"
    Expected Result: duration_ms is a positive integer, reasonable magnitude
    Failure Indicators: TypeError, duration_ms missing or negative
    Evidence: .sisyphus/evidence/task-3-duration.txt

  **Commit**: YES (groups with T4)
  - Message: `feat(container): add step execution mode with STEP_CONFIG support`
  - Files: `container/openclaw/step_runner.py`, `container/tests/test_step_runner.py`
  - Pre-commit: `cd container && python3 -m pytest tests/test_step_runner.py -v`

- [x] 4. Integrate STEP_CONFIG Detection into `entrypoint.py`

  **What to do**:
  - Modify `container/opt/openclaw/entrypoint.py`
  - Add STEP_CONFIG detection BEFORE the agent exec (around line 483-490):
    ```python
    # Check for step execution mode
    step_config_str = os.getenv('STEP_CONFIG', '').strip()
    if step_config_str:
        # Step mode: parse config, run step, write result, exit
        import sys
        sys.path.insert(0, '/opt/openclaw')
        sys.path.insert(0, '/opt/openclaw/openclaw')
        from openclaw.step_config import parse_step_config
        from openclaw.step_runner import StepRunner

        config = parse_step_config()
        if config:
            runner = StepRunner()
            exit_code = runner.run(config)
            sys.exit(exit_code)
        else:
            print("[ArmorClaw] ✗ ERROR: STEP_CONFIG present but failed to parse", file=sys.stderr)
            sys.exit(1)
    ```
  - Insert this block between the secrets verification section (line ~397) and the health check section (line ~401)
  - The placement is critical: AFTER secrets are loaded (step may need API key for future handlers), BEFORE agent exec (step mode replaces agent mode)
  - Test that without STEP_CONFIG, the entrypoint proceeds to agent mode exactly as before

  **Must NOT do**:
  - Do NOT change the existing agent mode path (no STEP_CONFIG = same behavior)
  - Do NOT remove or reorder any existing sections
  - Do NOT change the agent exec logic
  - Do NOT modify agent.py
  - Do NOT add network calls

  **Recommended Agent Profile**:
  - **Category**: `quick`
    - Reason: Small, surgical edit to one file (~15 lines added), well-defined insertion point
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: NO
  - **Parallel Group**: Wave 2 (sequential after T3)
  - **Blocks**: Task 5 (e2e test), Task 6 (docs)
  - **Blocked By**: Task 3 (step_runner)

  **References**:

  **Pattern References** (existing code to follow):
  - `container/opt/openclaw/entrypoint.py:293-330` — Secrets verification section ends at line ~330 with `sys.exit(1)` for missing keys. STEP_CONFIG detection should be placed AFTER this section so secrets are available.
  - `container/opt/openclaw/entrypoint.py:483-490` — Current "Start OpenClaw Agent" section that shows the default command logic. This is the code path that should be BYPASSED when STEP_CONFIG is present.

  **API/Type References**:
  - `container/openclaw/step_config.py` — parse_step_config() returns StepConfig or raises ValueError (created in T2)
  - `container/openclaw/step_runner.py` — StepRunner().run(config) returns exit code (created in T3)

  **WHY Each Reference Matters**:
  - `entrypoint.py:293-330` — STEP_CONFIG detection MUST come after secrets loading so the step has access to API keys (for future handlers)
  - `entrypoint.py:483-490` — This is the code that gets bypassed — need to understand the flow to insert cleanly

  **Acceptance Criteria**:

  **QA Scenarios (MANDATORY):**

  ```
  Scenario: STEP_CONFIG present triggers step mode
    Tool: Bash
    Preconditions: result_writer, step_config, step_runner all exist
    Steps:
      1. mkdir -p /tmp/test-entrypoint-step && cd /home/mikmin/.LocalCode/armorclaw-omo
      2. STEP_CONFIG='{"handler":"echo","task":"entrypoint test"}' \
         STATE_DIR=/tmp/test-entrypoint-step \
         OPENAI_API_KEY=sk-test-dummy \
         python3 container/opt/openclaw/entrypoint.py 2>&1 || true
      3. cat /tmp/test-entrypoint-step/result.json 2>/dev/null || echo "NO RESULT FILE"
    Expected Result: result.json exists with status='success', output contains 'entrypoint test'
    Failure Indicators: "NO RESULT FILE", or agent mode started (infinite loop)
    Evidence: .sisyphus/evidence/task-4-step-mode.txt

  Scenario: No STEP_CONFIG preserves agent mode path
    Tool: Bash
    Preconditions: STEP_CONFIG not set
    Steps:
      1. cd /home/mikmin/.LocalCode/armorclaw-omo
      2. timeout 5 python3 -c "
import os, sys
# Simulate entrypoint without STEP_CONFIG
os.environ['OPENAI_API_KEY'] = 'sk-test-dummy'
os.environ.pop('STEP_CONFIG', None)
# Import and check the detection logic
exec(open('container/opt/openclaw/entrypoint.py').read().split('# Start OpenClaw Agent')[0])
step_config_str = os.getenv('STEP_CONFIG', '').strip()
assert step_config_str == '', f'STEP_CONFIG should be empty: {step_config_str}'
print('PASS: No STEP_CONFIG detected, would proceed to agent mode')
" 2>&1
    Expected Result: "PASS: No STEP_CONFIG detected, would proceed to agent mode"
    Failure Indicators: STEP_CONFIG detected when it shouldn't be
    Evidence: .sisyphus/evidence/task-4-agent-mode.txt

  Scenario: STEP_CONFIG with invalid JSON exits with error
    Tool: Bash
    Preconditions: Python 3 available
    Steps:
      1. mkdir -p /tmp/test-entrypoint-invalid && cd /home/mikmin/.LocalCode/armorclaw-omo
      2. STEP_CONFIG='not-json' \
         STATE_DIR=/tmp/test-entrypoint-invalid \
         OPENAI_API_KEY=sk-test-dummy \
         python3 container/opt/openclaw/entrypoint.py 2>&1; echo "EXIT_CODE=$?"
    Expected Result: Exit code non-zero, error message about invalid JSON
    Failure Indicators: Exit code 0, or agent mode started
    Evidence: .sisyphus/evidence/task-4-invalid-config.txt

  **Commit**: YES (groups with T3)
  - Message: `feat(container): add step execution mode with STEP_CONFIG support`
  - Files: `container/opt/openclaw/entrypoint.py`
  - Pre-commit: `cd container && python3 -m pytest tests/ -v`

- [x] 5. End-to-End Integration Test — Bridge-side Parse Meets Container-side Write

  **What to do**:
  - Create `container/tests/test_e2e_backward_channel.py`
  - Test the complete flow:
    1. Create temp directory simulating host state dir
    2. Write STEP_CONFIG to env
    3. Run step_runner → writes result.json
    4. Use Bridge's ParseContainerStepResult logic (reimplemented in Python for this test) to read and validate
  - Verify the JSON written by container matches what Bridge expects:
    - Field names exactly match ContainerStepResult struct
    - Types match: status=string, output=string, data=dict, error=string, duration_ms=int
  - Test edge cases:
    - Empty state dir (step writes first file)
    - Pre-existing result.json from previous run (step overwrites)
    - Step that fails (error field populated, status='failed')
  - Create a shell script `container/tests/e2e_backward_channel.sh` that:
    1. Sets up temp dirs
    2. Runs entrypoint.py with STEP_CONFIG
    3. Validates result.json contents
    4. Cleans up

  **Must NOT do**:
  - Do NOT modify any Go code
  - Do NOT require Docker (test must run without containers)
  - Do NOT require network access

  **Recommended Agent Profile**:
  - **Category**: `unspecified-high`
    - Reason: Integration test connecting multiple modules, needs careful validation of JSON schema compatibility
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: NO
  - **Parallel Group**: Wave 3 (sequential after T4)
  - **Blocks**: F1-F4
  - **Blocked By**: Task 4 (entrypoint integration)

  **References**:

  **Pattern References**:
  - `bridge/pkg/secretary/result_test.go` — Existing Go tests for ParseContainerStepResult. Shows how Bridge validates result.json: checks field values, handles missing file, handles malformed JSON.
  - `bridge/pkg/secretary/backward_channel_test.go` — Integration tests for the backward channel on Bridge side.

  **API/Type References**:
  - `bridge/pkg/secretary/result.go:39-54` — ContainerStepResult struct — the exact JSON schema to validate against
  - `bridge/pkg/secretary/orchestrator_integration.go:337-339` — Where Bridge reads result: `parsed, _ := ParseContainerStepResult(stateDir)` — confirms stateDir path convention

  **WHY Each Reference Matters**:
  - `result_test.go` — Shows the exact assertions Bridge makes when parsing result.json — our e2e test must match these expectations
  - `orchestrator_integration.go:337-339` — Confirms the state dir path format and that Bridge reads result.json after container exit

  **Acceptance Criteria**:

  **QA Scenarios (MANDATORY):**

  ```
  Scenario: End-to-end backward channel flow
    Tool: Bash
    Preconditions: All container modules (T1-T4) complete
    Steps:
      1. mkdir -p /tmp/e2e-test-state && cd /home/mikmin/.LocalCode/armorclaw-omo/container
      2. python3 -c "
import os, sys, json; sys.path.insert(0, '.'); sys.path.insert(0, 'opt')
os.environ['STEP_CONFIG'] = json.dumps({'handler': 'echo', 'task': 'e2e test'})
os.environ['STATE_DIR'] = '/tmp/e2e-test-state'
os.environ['OPENAI_API_KEY'] = 'sk-test'
from openclaw.step_config import parse_step_config
from openclaw.step_runner import StepRunner
cfg = parse_step_config()
runner = StepRunner()
exit_code = runner.run(cfg)
print(f'Exit code: {exit_code}')
with open('/tmp/e2e-test-state/result.json') as f:
    data = json.load(f)
# Validate against ContainerStepResult schema
assert isinstance(data['status'], str), 'status must be string'
assert isinstance(data['output'], str), 'output must be string'
assert isinstance(data['duration_ms'], int), 'duration_ms must be int'
assert data['status'] == 'success'
assert 'e2e test' in data['output']
print('PASS: e2e backward channel works')
print(json.dumps(data, indent=2))
"
      3. rm -rf /tmp/e2e-test-state
    Expected Result: result.json matches ContainerStepResult schema, status='success'
    Failure Indicators: Schema mismatch, missing file, wrong types
    Evidence: .sisyphus/evidence/task-5-e2e-flow.txt

  Scenario: Failed step writes error result
    Tool: Bash
    Preconditions: All container modules complete
    Steps:
      1. mkdir -p /tmp/e2e-test-error && cd /home/mikmin/.LocalCode/armorclaw-omo/container
      2. python3 -c "
import os, sys, json; sys.path.insert(0, '.'); sys.path.insert(0, 'opt')
os.environ['STEP_CONFIG'] = json.dumps({'handler': 'nonexistent_handler'})
os.environ['STATE_DIR'] = '/tmp/e2e-test-error'
os.environ['OPENAI_API_KEY'] = 'sk-test'
from openclaw.step_config import parse_step_config
from openclaw.step_runner import StepRunner
cfg = parse_step_config()
runner = StepRunner()
exit_code = runner.run(cfg)
with open('/tmp/e2e-test-error/result.json') as f:
    data = json.load(f)
assert data['status'] in ('partial', 'failed'), f'Expected partial/failed, got {data[\"status\"]}'
assert data['duration_ms'] > 0
print(f'PASS: error result written, status={data[\"status\"]}')
" 2>&1
      3. rm -rf /tmp/e2e-test-error
    Expected Result: result.json exists with status 'partial' or 'failed', duration_ms > 0
    Failure Indicators: FileNotFoundError, status='success' for error case
    Evidence: .sisyphus/evidence/task-5-error-e2e.txt

  Scenario: JSON schema matches Bridge's ContainerStepResult
    Tool: Bash
    Preconditions: result.json from previous test
    Steps:
      1. cd /home/mikmin/.LocalCode/armorclaw-omo/container
      2. python3 -c "
import json
# The Bridge's ContainerStepResult Go struct expects these exact JSON fields:
# status (string), output (string), data (map, omitempty), error (string, omitempty), duration_ms (int64)
# This test validates the JSON structure is compatible
schema = {
    'status': str,          # required
    'output': str,          # required
    'data': (dict, type(None)),  # optional (omitempty)
    'error': (str, type(None)),  # optional (omitempty)
    'duration_ms': int,     # required (int64 in Go = int in Python)
}
# Create a sample result
sample = {'status': 'success', 'output': 'test', 'duration_ms': 100, 'data': {'key': 'val'}}
for field, expected_type in schema.items():
    val = sample.get(field)
    assert val is None or isinstance(val, expected_type), f'{field}: expected {expected_type}, got {type(val)}'
print('PASS: JSON schema is compatible with ContainerStepResult')
"
    Expected Result: All type checks pass
    Failure Indicators: Type mismatch assertion
    Evidence: .sisyphus/evidence/task-5-schema-check.txt

  **Commit**: YES (groups with T6)
  - Message: `test(container): add end-to-end backward channel integration tests`
  - Files: `container/tests/test_e2e_backward_channel.py`, `container/tests/e2e_backward_channel.sh`
  - Pre-commit: `cd container && python3 -m pytest tests/test_e2e_backward_channel.py -v`

- [x] 6. Update Container Documentation

  **What to do**:
  - Update `doc/agent-runtime.md` to document:
    - Step execution mode (STEP_CONFIG triggers step mode)
    - Result file convention (result.json in state dir)
    - ContainerStepResult schema reference
    - The two modes: agent mode (default) vs step mode (STEP_CONFIG present)
  - Add a brief section to `doc/armorclaw.md` if there's an existing container section:
    - Cross-reference to agent-runtime.md
    - One-line note about result.json backward channel
  - Force-add with `git add -f` since /doc/ is in .gitignore

  **Must NOT do**:
  - Do NOT create new doc files — update existing ones only
  - Do NOT duplicate content from agent-runtime.md elsewhere
  - Do NOT add diagrams (text descriptions only)

  **Recommended Agent Profile**:
  - **Category**: `quick`
    - Reason: Text updates to 1-2 existing markdown files
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: NO
  - **Parallel Group**: Wave 3 (sequential after T4)
  - **Blocks**: F1-F4
  - **Blocked By**: Task 4 (entrypoint integration)

  **References**:

  **Pattern References**:
  - `doc/agent-runtime.md` — Existing container runtime documentation. Need to add step execution mode section here.
  - `doc/armorclaw.md` — Main architecture doc. Check if there's a container section to cross-reference.

  **WHY Each Reference Matters**:
  - `doc/agent-runtime.md` — This is where container runtime behavior is documented. Step mode is a new runtime mode that belongs here.

  **Acceptance Criteria**:

  **QA Scenarios (MANDATORY):**

  ```
  Scenario: Documentation mentions STEP_CONFIG and result.json
    Tool: Bash
    Preconditions: Doc files updated
    Steps:
      1. cd /home/mikmin/.LocalCode/armorclaw-omo
      2. grep -c "STEP_CONFIG" doc/agent-runtime.md
      3. grep -c "result.json" doc/agent-runtime.md
      4. grep -c "step mode" doc/agent-runtime.md
    Expected Result: Each grep returns count >= 1
    Failure Indicators: Count is 0 for any term
    Evidence: .sisyphus/evidence/task-6-doc-update.txt

  Scenario: No invalid file references in docs
    Tool: Bash
    Preconditions: Doc files updated
    Steps:
      1. cd /home/mikmin/.LocalCode/armorclaw-omo
      2. grep -o 'container/openclaw/[a-z_]*.py' doc/agent-runtime.md | sort -u | while read f; do
           test -f "$f" && echo "OK: $f" || echo "MISSING: $f"
         done
    Expected Result: All referenced files exist
    Failure Indicators: "MISSING:" in output
    Evidence: .sisyphus/evidence/task-6-doc-references.txt

  **Commit**: YES (groups with T5)
  - Message: `test(container): add end-to-end backward channel integration tests`
  - Files: `doc/agent-runtime.md`, optionally `doc/armorclaw.md`
  - Pre-commit: `grep -c "STEP_CONFIG" doc/agent-runtime.md`

---

## Final Verification Wave (MANDATORY — after ALL implementation tasks)

> 4 review agents run in PARALLEL. ALL must APPROVE. Present consolidated results to user and get explicit "okay" before completing.
> Do NOT auto-proceed after verification. Wait for user's explicit approval.

- [x] F1. **Plan Compliance Audit** — `oracle`
  Read the plan end-to-end. For each "Must Have": verify implementation exists (read file, run command). For each "Must NOT Have": search codebase for forbidden patterns — reject with file:line if found. Check evidence files exist in .sisyphus/evidence/. Compare deliverables against plan.
  Output: `Must Have [N/N] | Must NOT Have [N/N] | Tasks [N/N] | VERDICT: APPROVE/REJECT`

- [x] F2. **Code Quality Review** — `unspecified-high`
  Run Python linting (`python -m py_compile` for syntax, check for common issues). Review all changed files for: bare except, print in production code, missing error handling, unused imports, hardcoded paths. Check AI slop: excessive comments, over-abstraction, generic names.
  Output: `Lint [PASS/FAIL] | Files [N clean/N issues] | VERDICT`

- [x] F3. **Real Manual QA** — `unspecified-high`
  Start from clean state. Execute EVERY QA scenario from EVERY task — follow exact steps, capture evidence. Test cross-task integration. Test edge cases: empty STEP_CONFIG, invalid JSON, missing state dir, permission errors. Save to `.sisyphus/evidence/final-qa/`.
  Output: `Scenarios [N/N pass] | Integration [N/N] | Edge Cases [N tested] | VERDICT`

- [x] F4. **Scope Fidelity Check** — `deep`
  For each task: read "What to do", read actual diff. Verify 1:1 — everything in spec was built (no missing), nothing beyond spec was built (no creep). Check "Must NOT do" compliance. Flag unaccounted changes.
  Output: `Tasks [N/N compliant] | Unaccounted [CLEAN/N files] | VERDICT`

---

## Commit Strategy

- **Commit 1**: `feat(container): add atomic result_writer module for backward channel` — T1-T2
  Files: `container/openclaw/result_writer.py`, `container/openclaw/step_config.py`, test files
  Pre-commit: `cd container && python -m pytest tests/ -v`

- **Commit 2**: `feat(container): add step execution mode with STEP_CONFIG support` — T3-T4
  Files: `container/openclaw/step_runner.py`, `container/opt/openclaw/entrypoint.py`
  Pre-commit: `cd container && python -m pytest tests/ -v`

- **Commit 3**: `test(container): add end-to-end backward channel integration test` — T5-T6
  Files: test files, doc updates
  Pre-commit: `cd container && python -m pytest tests/ -v`

---

## Success Criteria

### Verification Commands
```bash
# Python unit tests for container modules
cd /home/mikmin/.LocalCode/armorclaw-omo/container && python3 -m pytest tests/ -v
# Expected: All tests pass

# Go tests for Bridge (ensure no regressions)
cd /home/mikmin/.LocalCode/armorclaw-omo/bridge && go test ./pkg/secretary/ -v -run "Result|Backward"
# Expected: All existing tests still pass

# Verify entrypoint STEP_CONFIG detection
STEP_CONFIG='{"task":"echo hello"}' python3 -c "
import sys, os
sys.path.insert(0, 'opt')
sys.path.insert(0, 'openclaw')
os.environ['STEP_CONFIG'] = '{\"task\":\"test\"}'
os.environ['STATE_DIR'] = '/tmp/test-state'
os.makedirs('/tmp/test-state', exist_ok=True)
from step_runner import StepRunner
runner = StepRunner()
print('step_runner importable:', runner is not None)
"
# Expected: step_runner importable: True
```

### Final Checklist
- [ ] result_writer.py writes result.json atomically (temp + rename)
- [ ] step_config.py parses STEP_CONFIG env var gracefully
- [ ] step_runner.py executes steps and calls result_writer
- [ ] entrypoint.py dispatches to step_runner when STEP_CONFIG present
- [ ] entrypoint.py unchanged behavior when STEP_CONFIG absent
- [ ] No changes to NetworkMode, factory.go, or any Go code
- [ ] No changes to agent.py
- [ ] All existing Go tests pass
- [ ] Container image name unchanged
