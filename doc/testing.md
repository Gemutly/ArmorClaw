# ArmorClaw E2E Testing Framework

> **Purpose**: LLM-readable documentation for ArmorClaw's end-to-end testing infrastructure — how the contract discovery pipeline, test harness, and verification system work together.
>
> **Version**: 1.1.0
>
> **Last Updated**: 2026-04-29

---

## Context Routing Rules

> **RULE**: Before modifying any test infrastructure, read the source files listed below.

| Task | Required Reading |
|------|-----------------|
| Add or modify test helpers | `tests/lib/` and `scripts/lib/contract.sh` |
| Add new test scripts | `tests/test-system-health-baseline.sh` (Tier A pattern), `tests/test-voice-stack.sh` (Tier B pattern) |
| Change E2E discovery pipeline | `scripts/a0_discover.sh`, `scripts/lib/contract.sh` |
| Change deployment testing | `scripts/a1_deploy.sh`, `scripts/deploy-infrastructure.sh` |
| Modify RPC probing | `scripts/lib/contract.sh` (`_contract_bridge_rpc`), `bridge/pkg/rpc/server.go` |
| Update evidence format | `scripts/lib/contract.sh` (`_contract_save`), `.gitignore` |
| Add test suites to harness | `scripts/a4_harness.sh` (SUITE_MAP) |
| Change master runner | `scripts/a_run_all.sh` |

---

## Table of Contents

1. [Architecture Overview](#architecture-overview)
2. [Shared Libraries](#shared-libraries)
3. [Test Tiers](#test-tiers)
4. [E2E Contract Discovery Pipeline (Plan A)](#e2e-contract-discovery-pipeline-plan-a)
5. [Evidence System](#evidence-system)
6. [How to Run](#how-to-run)
7. [Contract Manifest Schema](#contract-manifest-schema)
8. [RPC Method Discovery](#rpc-method-discovery)
9. [Known Limitations](#known-limitations)

---

## Architecture Overview

ArmorClaw testing has three layers that stack on each other:

```
┌─────────────────────────────────────────────────────────┐
│  Plan A: E2E Pipeline (scripts/a0-a4 + a_run_all.sh)   │
│  Discovers contract, deploys, provisions, validates     │
├─────────────────────────────────────────────────────────┤
│  Test Harness (tests/test-*.sh)                         │
│  13 subsystem tests + 4 cross-subsystem scenarios       │
│  Organized in Tier A (VPS) and Tier B (code-only)      │
├─────────────────────────────────────────────────────────┤
│  Shared Libraries (tests/lib/ + scripts/lib/)           │
│  load_env.sh, contract.sh, assert_json.sh, etc.         │
└─────────────────────────────────────────────────────────┘
```

**Key principle**: Tests probe the **live** system. No mocks, no stubs. Every test makes real SSH, curl, or RPC calls to the running ArmorClaw instance on the VPS. Tests that cannot run (missing dependencies, subsystem not deployed) **SKIP gracefully** rather than fail.

---

## Shared Libraries

### `tests/lib/` — Core Test Infrastructure

Every test script sources at least `load_env.sh` and `common_output.sh`. These are the foundation.

| File | Provides | Used By |
|------|----------|---------|
| `load_env.sh` | `ssh_vps()`, `check_bridge_running()`, env vars (VPS_IP, BRIDGE_PORT, etc.) | All test scripts, `contract.sh` |
| `common_output.sh` | `log_pass()`, `log_fail()`, `log_skip()`, `harness_summary()`, PASS/FAIL/SKIP counters | All test scripts |
| `assert_json.sh` | `assert_json_has_key()`, `assert_json_equals()`, `assert_rpc_success()` | Tests that validate JSON responses |
| `restart_bridge.sh` | `restart_bridge()` with `flock` serialization | Tests that need bridge restart |
| `event_subscriber_helper.sh` | `subscribe_events()`, `capture_events()` via `websocat` | Event streaming tests (note: bridge requires registration handshake before sending events) |

**Sourcing pattern** (followed by all test scripts):
```bash
#!/usr/bin/env bash
set -euo pipefail
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
source "$SCRIPT_DIR/lib/load_env.sh"
source "$SCRIPT_DIR/lib/common_output.sh"
source "$SCRIPT_DIR/lib/assert_json.sh"
```

### `scripts/lib/contract.sh` — E2E Pipeline Library

Extends `tests/lib/` with contract discovery helpers used by the Phase A scripts (a0-a4). Sources `load_env.sh` and `common_output.sh` automatically.

| Function | Purpose |
|----------|---------|
| `_contract_bridge_rpc(method, params, retries)` | Call Bridge JSON-RPC via `ssh_vps` with exponential backoff retry |
| `_contract_wait_http(port, path, timeout)` | Poll HTTP endpoint on VPS until 200 or timeout |
| `_contract_save(filename, content)` | Write evidence to `.sisyphus/evidence/armorclaw/` |
| `_contract_save_raw(filename)` | Write stdin to evidence (for piped content) |
| `_contract_load_manifest()` | Load `contract_manifest.json`; creates minimal if missing |
| `_contract_update_manifest(filter, value)` | Atomic jq update to manifest |
| `_contract_update_manifest_merge(expression)` | Apply arbitrary jq expression to manifest |
| `_contract_ssh_test()` | Verify SSH connectivity to VPS |
| `_contract_check_bridge_port(port)` | Check if port is listening on VPS |

**Sourcing pattern** (followed by all Phase A scripts):
```bash
#!/usr/bin/env bash
set -uo pipefail
_SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
source "${_SCRIPT_DIR}/lib/contract.sh"
```

---

## Test Tiers

### Tier A — VPS-Deployed (Live Integration)

These tests require the ArmorClaw Bridge running on a VPS. They make real SSH + curl + RPC calls. Each test saves evidence to `.sisyphus/evidence/`.

| Script | Subsystem | Scenarios | What It Tests |
|--------|-----------|-----------|---------------|
| `test-system-health-baseline.sh` | Health | 7 (H0-H6) | Bridge /health, Matrix /versions, keystore, EventBus |
| `test-trust-layer.sh` | Security | 8 (S0-S7) | PII detection, approval/denial flow, risk classification, audit trail |
| `test-eventbus-streaming.sh` | Events | 7 (E0-E6) | WebSocket events, subscription, EventBus publishing |
| `test-secretary-workflow-core.sh` | Workflows | 7 (W0-W6) | Workflow state machine, blocker resolution, restart survival |
| `test-email-pipeline.sh` | Email | 7 (M0-M6) | Email approval RPC boundary, pending list, status check |
| `test-tls-restart-safety.sh` | TLS | 3 | Fingerprint, TLS mode, QR state preservation across bridge restart |
| `test-tls-mode-integration.sh` | TLS | 10 | Full TLS surface: fingerprint consistency, mode detection, HTTPS enforcement, QR v1/v2 |

**Tier A pattern** — each script follows this structure:
```
H0/S0/W0: Prerequisites (jq, curl, SSH, bridge running)
H1-S1: Core functionality tests
...
Hn-Sn: Summary + evidence save
harness_summary → exit 0/1
```

### Tier B — Code-Only (Graceful Skip on VPS)

These tests validate code-level properties. They SKIP gracefully when subsystems aren't deployed (e.g., voice not configured, sidecar not running).

| Script | Subsystem | Skip Condition |
|--------|-----------|---------------|
| `test-secretary-workflow-deep.sh` | Workflow deep | PII-gated halt, parallel steps |
| `test-sidecar-docs.sh` | Document pipeline | Sidecar container not running |
| `test-voice-stack.sh` | Voice | Voice services not configured |
| `test-jetski-sidecar.sh` | Browser sidecar | Jetski not deployed |
| `test-license-enforcement.sh` | License | License RPC not configured |
| `test-platform-adapters.sh` | Matrix/Slack | Slack adapter not present |
| `test-agent-runtime.sh` | Agent containers | Docker not available |
| `test-deployment-usb.sh` | USB detection | No USB device present |

### Cross-Subsystem Scenarios

These tests validate interactions between subsystems:

| Script | Path | Tests |
|--------|------|-------|
| `test-cross-workflow-email.sh` | Secretary → Email | Workflow triggers email approval |
| `test-cross-workflow-docs.sh` | Secretary → Sidecar | Workflow uses document pipeline |
| `test-cross-browser-trust.sh` | Jetski → Trust | Browser automation respects PII gates |
| `test-cross-event-truth.sh` | EventBus → Multi | Event stream consistency across subsystems |

---

## E2E Contract Discovery Pipeline (Plan A)

Plan A is the end-to-end pipeline that discovers the live Bridge contract, provisions admin access, validates events, and runs the test harness. It consists of 5 phases executed in order.

### Execution Flow

```
A0 (Discovery) ──→ A1 (Deploy) ──→ A0 (Re-discover) ──→ A2 (Provision) ──→ A3 (Events) ──→ A4 (Harness)
     │                  │                                      │                  │                │
     │                  │                                      │                  │                │
     ▼                  ▼                                      ▼                  ▼                ▼
 contract_         docker containers                    a2_matrix_          a3_summary     a4_summary
 manifest.json     started (if needed)                  session.json        .json          .json
```

### Phase A0 — Contract Discovery (`scripts/a0_discover.sh`)

Probes the live Bridge to discover what it actually supports at runtime. No hardcoded assumptions.

| Step | What It Does |
|------|-------------|
| A0.1 | Verify VPS SSH connectivity |
| A0.2 | Check if Bridge is running (systemd / Docker / port probe). If not running: set `deployment_required=true`, exit cleanly |
| A0.3 | Probe HTTP endpoints: `/health`, `/api`, `/.well-known/matrix/client`, `/qr/config`, `/metrics`, `/version`, `/status` |
| A0.4 | Probe all 89 RPC methods with empty params — record `responds` / `error` / `timeout` status for each |
| A0.5 | For responding methods, capture error messages as parameter schema hints |
| A0.6 | Check Matrix homeserver (`/_matrix/client/versions`, `/.well-known/matrix/client`) |
| A0.7 | Document env var names from codebase |
| A0.8 | Document deep link formats |
| A0.9 | Generate `contract_manifest.json` with all discoveries |

**Key output**: `contract_manifest.json` — the authoritative contract artifact used by all downstream phases.

**Bridge already running**: If A0.2 finds the bridge healthy, it sets `deployment_required=false` and continues discovery. The master runner (`a_run_all.sh`) only invokes A1 when this flag is true.

**Bridge not running**: A0 records `deployment_required=true` and exits. The master runner then runs A1 (deploy), then re-runs A0 to populate the manifest with live data.

### Phase A1 — Topology-Aware Deployment (`scripts/a1_deploy.sh`)

Deploys ArmorClaw only if not already running. Inspects the Docker image to determine topology before composing.

| Step | What It Does |
|------|-------------|
| A1.1 | Verify SSH |
| A1.2 | Ensure Docker on VPS |
| A1.3 | Resource preflight — check RAM ≥1500MB, disk ≥5GB, CPU count. Fails early with diagnostic if insufficient |
| A1.4 | Pull image, inspect exposed ports and entrypoint |
| A1.5 | Generate secrets (TURN_SECRET, KEYSTORE_SECRET) |
| A1.6 | Resolve API key from env (tries OPENROUTER_API_KEY, OPEN_AI_KEY, ZAI_API_KEY in order) |
| A1.7 | Determine topology (single image vs multi-service) based on whether port 6167 is in the image |
| A1.8 | Create `docker-compose.plan-a.yml` on VPS at `/opt/armorclaw/` |
| A1.9 | Start containers |
| A1.10 | Wait for Bridge `/health` (3 min timeout) |
| A1.11 | Wait for Matrix `/_matrix/client/versions` (3 min timeout) |
| A1.12 | Verify `/.well-known/matrix/client` |
| A1.13 | Collect deployment evidence (container list, compose file, resource stats) |

**Idempotency**: If bridge is already healthy at `BRIDGE_PORT`, A1 skips deployment unless `FORCE_REDEPLOY=1` is set.

**Topology detection**: Inspects `docker inspect` output. If image exposes port 6167, it's a single-image deployment (Bridge + Matrix in one container). Otherwise, creates separate Bridge and Matrix containers.

### Phase A2 — Admin Bootstrap (`scripts/a2_provision.sh`)

Provisions admin access using RPC methods discovered in A0. Handles blocked Matrix registration gracefully.

| Step | What It Does |
|------|-------------|
| A2.1 | Find provisioning RPC method from manifest (falls back to `provisioning.claim`) |
| A2.2 | Attempt provisioning claim with empty params |
| A2.3 | Retrieve bridge configuration via `bridge.status` |
| A2.4 | Verify bridge health via RPC or HTTP |
| A2.5 | Retrieve `/qr/config` for mobile provisioning |
| A2.6 | Verify `/.well-known/matrix/client` for Matrix discovery |
| A2.7 | Create test Matrix user (tries `m.login.dummy`, then `m.login.registration_token`) |
| A2.8 | Create test Matrix room (requires session from A2.7) |
| A2.9 | Write provisioning outputs — always writes `a2_provisioning_outputs.json`; writes `a2_matrix_session.json` only if registration succeeded |

**Matrix session SKIP path**: If registration is disabled (common in production), A2.7 records `matrix_session: "SKIPPED"` with the reason in `a2_provisioning_outputs.json`. Downstream phases (A3) degrade gracefully.

### Phase A3 — Event Validation (`scripts/a3_events.sh`)

Validates that the Bridge emits events through the Matrix control plane. Operates in full or degraded mode depending on Matrix session availability.

| Step | Session Required | What It Does |
|------|-----------------|-------------|
| A3.0 | No | Check if `a2_matrix_session.json` exists. If missing, set degraded mode |
| A3.1 | Yes | Send `m.room.message` to test room, verify in `/sync` |
| A3.2 | Yes | Start workflow via `secretary.start_workflow` RPC, observe workflow events in `/sync` |
| A3.3 | Yes | Check for agent status events via `studio.stats` |
| A3.4 | No | Scan bridge logs for event publication evidence |
| A3.5 | Best-effort | Full event type scan — multiple `/sync` calls, collect all types, cross-reference |

**Degraded mode**: When no Matrix session is available, A3.1-A3.3 are marked SKIP with documented reasons. A3.4 (log scan) and A3.5 (event type collection) still run on a best-effort basis.

**Output**: `a3.5_discovered_event_types.txt` — one event type per line, for cross-referencing with expected types.

### Phase A4 — Harness Execution (`scripts/a4_prepare.sh` + `scripts/a4_harness.sh`)

Copies the test harness to the VPS and runs selected suites.

**a4_prepare.sh**: Copies `tests/test-*.sh`, `tests/lib/`, and `tests/e2e/` to `/opt/armorclaw/tests/` on the VPS via scp.

**a4_harness.sh**: Maps suite names to test files and runs them on the VPS:

| Suite Name | Test File |
|-----------|-----------|
| `health` | `test-system-health-baseline.sh` |
| `eventbus` | `test-eventbus-streaming.sh` |
| `trust` | `test-trust-layer.sh` |
| `workflow-core` | `test-secretary-workflow-core.sh` |
| `email` | `test-email-pipeline.sh` |
| `workflow-deep` | `test-secretary-workflow-deep.sh` |
| `sidecar-docs` | `test-sidecar-docs.sh` |
| `voice` | `test-voice-stack.sh` |
| `jetski` | `test-jetski-sidecar.sh` |
| `license` | `test-license-enforcement.sh` |
| `platform` | `test-platform-adapters.sh` |
| `agent-runtime` | `test-agent-runtime.sh` |
| `deployment-usb` | `test-deployment-usb.sh` |
| `cross-workflow-email` | `test-cross-workflow-email.sh` |
| `cross-workflow-docs` | `test-cross-workflow-docs.sh` |
| `cross-browser-trust` | `test-cross-browser-trust.sh` |
| `cross-event-truth` | `test-cross-event-truth.sh` |

### Master Runner (`scripts/a_run_all.sh`)

Orchestrates all phases. Supports positional arguments:

```bash
bash scripts/a_run_all.sh all              # Full pipeline: A0→A1(conditional)→A0→A2→A3→A4
bash scripts/a_run_all.sh A0                # Contract discovery only
bash scripts/a_run_all.sh A1                # Deploy only
bash scripts/a_run_all.sh A2                # Provision only
bash scripts/a_run_all.sh A3                # Event validation only
bash scripts/a_run_all.sh A4                # Harness execution only
```

**Failure handling**: A0 and A1 failures are fatal (stop the pipeline). If A0 discovers `deployment_required=true`, the `all` path automatically runs A1, then re-runs A0.

---

## Evidence System

### Directory Structure

All evidence files are written to `.sisyphus/evidence/armorclaw/`. This directory is in `.gitignore` and must never be committed with raw tokens or credentials.

### Evidence Boundary

Two evidence roots exist for different purposes:

| Root | Purpose | Written By |
|------|---------|-----------|
| `.sisyphus/evidence/armorclaw/` | Plan A pipeline outputs (A0–A4) | `scripts/lib/contract.sh` (`_contract_save()`), called by `a0_discover.sh`, `a1_deploy.sh`, `a2_provision.sh`, etc. |
| `.sisyphus/evidence/tls/` | TLS verification artifacts | `tests/test-tls-*.sh` and TLS plan execution |

Agents and scripts must write to the correct root. Plan A scripts always use `.sisyphus/evidence/armorclaw/`. TLS-specific verification uses `.sisyphus/evidence/tls/`.

```
.sisyphus/evidence/armorclaw/
├── contract_manifest.json         # THE key output — discovered contract
├── a0_summary.json                # Discovery phase results
├── a0_well_known.json             # /.well-known/matrix/client response
├── a0_matrix_versions.json        # Matrix /versions response
├── a0_rpc_schemas.json            # Parameter hints from error messages
├── a2_bridge_status.json          # bridge.status RPC result
├── a2_provisioning_outputs.json   # Connection details, session status
├── a2_matrix_session.json         # Matrix session (only if registration succeeded)
├── a2_qr_config.json              # /qr/config response
├── a2_summary.json                 # Provisioning phase results
├── a3_summary.json                 # Event validation results
├── a3.5_discovered_event_types.txt # All discovered event types
├── a3_log_events.txt              # Bridge log event evidence
├── a4_summary.json                 # Harness execution results
├── a4_{suite}_output.txt          # Per-suite test output
├── a1_deploy_status.json          # Deployment results
├── a1_vps_resources.json          # VPS resource stats
└── final_summary.json             # Master runner final status
```

### Security Rules

1. **`.gitignore` covers**: `.sisyphus/evidence/armorclaw/*session*.json` and `.sisyphus/evidence/armorclaw/*token*`
2. **Never commit**: Raw Matrix access tokens, cookies, QR payload secrets
3. **Runtime-only artifacts**: Session JSON files are for test execution only
4. **Redact before sharing**: Strip `access_token` fields from any evidence shared externally

---

## How to Run

### Prerequisites

```bash
# Local machine requirements
jq --version       # JSON processor
curl --version     # HTTP client
ssh -V             # SSH client
scp                # File copy

# VPS requirements (for Tier A tests)
docker --version   # Docker Engine
docker compose     # Docker Compose (v2)
```

### Environment Setup

```bash
# .env at repo root
VPS_IP=5.183.11.149
VPS_USER=root
BRIDGE_PORT=8080
MATRIX_PORT=6167
SSH_KEY_PATH=~/.ssh/openclaw_win
OPENROUTER_API_KEY=sk-or-v1-xxx  # At least one AI provider key
```

### Running the Full Pipeline

```bash
# From repo root, with VPS access:
bash scripts/a_run_all.sh all
```

### Running Individual Phases

```bash
# Just discover the contract (no deploy needed if bridge already running)
bash scripts/a0_discover.sh

# Deploy if bridge not running
bash scripts/a1_deploy.sh

# Re-discover after deployment
bash scripts/a0_discover.sh

# Provision admin access
bash scripts/a2_provision.sh

# Validate events
bash scripts/a3_events.sh

# Run specific test suites
bash scripts/a4_harness.sh health
bash scripts/a4_harness.sh health,trust,workflow-core
```

### Running Individual Tests

```bash
# Tier A (requires VPS)
bash tests/test-system-health-baseline.sh
bash tests/test-trust-layer.sh

# Tier B (graceful skip if subsystem not deployed)
bash tests/test-voice-stack.sh
bash tests/test-sidecar-docs.sh
```

### Syntax Verification (No VPS Required)

```bash
# Validate all scripts are syntactically correct
for f in scripts/a*.sh scripts/lib/*.sh; do bash -n "$f" && echo "OK: $f"; done
for f in tests/test-*.sh tests/lib/*.sh; do bash -n "$f" && echo "OK: $f"; done
```

---

## Contract Manifest Schema

The `contract_manifest.json` is the central output of the discovery pipeline. All downstream phases read from it.

```json
{
  "live_discovered": {
    "rpc_methods": [
      {
        "name": "bridge.status",
        "status": "responds",
        "empty_params_result": "",
        "notes": "responds with result"
      },
      {
        "name": "container.terminate",
        "status": "error",
        "empty_params_result": "user_id is required for authentication",
        "notes": "responds with error: user_id is required for authentication"
      }
    ],
    "event_types": [
      {
        "type": "m.room.message",
        "source": "sync",
        "verified": true
      }
    ],
    "endpoints": [
      {
        "path": "/health",
        "status_code": 200,
        "description": "Bridge health (HTTPS)",
        "response_keys": ["bridge_ready", "status", "version", "timestamp"]
      }
    ]
  },
  "documented_reference": {
    "env_vars": ["VPS_IP", "BRIDGE_PORT", "..."],
    "deep_links": ["armorclaw://config?d={base64_config}", "..."]
  },
  "runtime_flags": {
    "deployment_required": false,
    "bridge_protocol": "https",
    "bridge_reachable": true,
    "ssh_reachable": true
  },
  "provisioning": {
    "homeserver_url": "http://5.183.11.149:6167",
    "matrix_session": "active",
    "provisioning_claim_status": "success"
  },
  "metadata": {
    "generated_at": "2026-04-29T00:44:42Z",
    "bridge_version": "4.6.0",
    "vps_ip": "5.183.11.149",
    "bridge_port": 8080,
    "methods_discovered": 89,
    "methods_responding": 16
  }
}
```

**Field semantics**:

| Field | Values | Meaning |
|-------|--------|---------|
| `rpc_methods[].status` | `responds` | Method returned a result (may be empty) |
| | `error` | Method returned a JSON-RPC error (params wrong, auth required, etc.) |
| | `timeout` | No response within timeout |
| | `unknown` | Unexpected response format |
| `runtime_flags.deployment_required` | `true` | Bridge not running, needs deploy before further probing |
| | `false` | Bridge is running and accessible |
| `provisioning.matrix_session` | `"active"` | Matrix user registered and session saved |
| | `"SKIPPED"` | Registration blocked, downstream phases degrade gracefully |

---

## RPC Method Discovery

### Method List

The bridge registers 95–109 RPC methods (feature-flag dependent) in `bridge/pkg/rpc/server.go` → `registerHandlers()`. Discovery probes each one with empty params `{}`. Baseline is 95 methods with all flags off; maximum is 109 with all flags enabled.

### Method Categories

| Category | Methods | Typical Empty-Params Response |
|----------|---------|-------------------------------|
| **Bridge** | `bridge.start`, `bridge.stop`, `bridge.status`, `bridge.channel`, `bridge.unchannel`, `bridge.list`, `bridge.ghost_list`, `bridge.appservice_status` | `bridge.status` returns `{enabled, status}`, others require params |
| **Health** | `health.check` | Returns `{status: "healthy", components: {bridge, keystore, matrix}}` |
| **Provisioning** | `provisioning.start`, `provisioning.claim` | `provisioning.start` generates setup token and QR data |
| **PII/Trust** | `pii.request`, `pii.approve`, `pii.deny`, `pii.status`, `pii.list_pending`, `pii.stats`, `pii.cancel`, `pii.fulfill`, `pii.wait_for_approval` | `pii.list_pending` and `pii.stats` return data, others require params |
| **Browser** | `browser.navigate`, `browser.fill`, `browser.click`, `browser.status`, `browser.wait_for_element`, `browser.wait_for_captcha`, `browser.wait_for_2fa`, `browser.complete`, `browser.fail`, `browser.list`, `browser.cancel` | All require active browser session params |
| **Skills** | `skills.execute`, `skills.list`, `skills.get_schema`, `skills.allow`, `skills.block`, `skills.allowlist_add/remove/list`, `skills.web_search`, `skills.web_extract`, `skills.email_send`, `skills.slack_message`, `skills.file_read`, `skills.data_analyze` | Most require params |
| **Matrix** | `matrix.status`, `matrix.login`, `matrix.send`, `matrix.receive`, `matrix.join_room` | `matrix.status` returns connection state |
| **Secretary** | `secretary.start/get/cancel/advance_workflow`, `secretary.list/create/get/delete/update_template` | Require workflow/template params |
| **Tasks** | `task.create`, `task.list`, `task.cancel`, `task.get` | Require task params |
| **Containers** | `container.terminate`, `container.list` | Require auth params |
| **Events** | `events.replay`, `events.stream` | Require durable log to be enabled |
| **Studio** | `studio.deploy`, `studio.stats` | `studio.stats` returns definition/instance counts |
| **Email** | `approve_email`, `deny_email`, `email_approval_status`, `email.list_pending` | `email.list_pending` returns pending approvals |
| **Devices** | `device.list`, `device.get`, `device.approve`, `device.reject` | Require auth |
| **Invites** | `invite.list`, `invite.create`, `invite.revoke`, `invite.validate` | Require auth |
| **Hardening** | `hardening.status`, `hardening.ack`, `hardening.rotate_password` | Require hardening store |
| **Other** | `store_key`, `resolve_blocker`, `mobile.heartbeat`, `account.delete`, `ai.chat` | Various params required |

### Calling Conventions

The bridge exposes a JSON-RPC 2.0 API at `/api`:

```bash
# Via HTTPS (external)
curl -skf https://$VPS_IP:$BRIDGE_PORT/api \
  -H 'Content-Type: application/json' \
  -d '{"jsonrpc":"2.0","id":1,"method":"health.check","params":{}}'

# Via SSH + localhost (from scripts using ssh_vps)
ssh_vps "curl -sf http://localhost:$BRIDGE_PORT/api \
  -H 'Content-Type: application/json' \
  -d '{\"jsonrpc\":\"2.0\",\"id\":1,\"method\":\"health.check\",\"params\":{}}'"

# Via Unix socket (inside VPS)
echo '{"jsonrpc":"2.0","id":1,"method":"status"}' | \
  socat - UNIX-CONNECT:/run/armorclaw/bridge.sock
```

**Note**: The bridge may use HTTPS even on localhost depending on deployment mode. Scripts use `ssh_vps "curl -sf http://localhost:..."` because localhost connections within the VPS don't need TLS. External access requires `https://`.

---

## Flag-Dependent Discovery

### Updated Baseline (v1.0.0)

The Bridge RPC method count is no longer fixed. With v1.0.0, feature flags control which methods are registered at startup. The contract discovery pipeline (`a0_discover.sh`) must account for this variability.

| Configuration | Methods | Flags Enabled |
|---|---|---|
| All off (baseline) | 95 | None |
| Zero-Trust Keystore | 102 | ZeroTrustKeystore |
| + Voice Pipeline | 105 | ZeroTrustKeystore + VoicePipeline |
| + E2EE Backup | 108 | ZeroTrustKeystore + VoicePipeline + E2EEBackup |
| + Multi-Tab Replay | 109 | All flags on |

### How Discovery Adapts

The `a0_discover.sh` script (Phase A0, step A0.4) probes all registered RPC methods with empty params. When a flag is disabled, its methods return JSON-RPC error `-32601` (method not found). The discovery pipeline treats `-32601` the same as any other error response: the method is recorded as `status: "error"` in the manifest with the error message.

To determine which flags are active, count responding methods:

```bash
# After a0_discover.sh completes
TOTAL=$(jq '.live_discovered.rpc_methods | length' .sisyphus/evidence/armorclaw/contract_manifest.json)
RESPONDING=$(jq '[.[] | select(.status == "responds")] | length' .sisyphus/evidence/armorclaw/contract_manifest.json)

echo "Total methods discovered: $TOTAL"
echo "Responding methods: $RESPONDING"
```

If the count is 95, no flags are enabled. If 102, only ZeroTrustKeystore is on. If 109, all flags are active.

### Flag-Dependent Test Behavior

Tests that exercise flag-dependent methods should check for `-32601` responses and SKIP gracefully rather than FAIL. This follows the existing Tier B pattern (graceful skip when subsystems are not deployed).

| Test Area | Skip Condition | Flag |
|-----------|---------------|------|
| `keystore.unseal` / seal / session | Method returns `-32601` | `ZeroTrustKeystore` |
| `voice.*` | Method returns `-32601` | `VoicePipeline` |
| `e2ee.create_backup` / delete / exists | Method returns `-32601` | `E2EEBackup` |
| `browser.replay_diagnostics` | Method returns `-32601` | `MultiTabReplay` |

### Reference

- Method counting: `scripts/a0_discover.sh` (A0.4)
- Feature flag configuration: `/etc/armorclaw/config.toml`
- Error code reference: `doc/bridge-reference.md`

---

## Known Limitations

| Limitation | Impact | Workaround |
|-----------|--------|-----------|
| SSH auth can be slow (60s+ timeout) | E2E pipeline stalls on SSH-dependent steps | Use direct HTTPS API when SSH is unavailable |
| Matrix registration often disabled | A3 event validation degrades to all-SKIP | This is by design — the SKIP path is documented |
| Hardening store may be closed | `hardening.status` returns database error | Run hardening setup first |
| 50-descendant limit per session | Cannot delegate to subagents in long sessions | Implement directly or start new session |

### Resolved Limitations (2026-04-30)

| Limitation | Resolution |
|-----------|-----------|
| Durable event log not enabled by default | Enable `enable_durable_log = true` under `[eventbus]` in `/etc/armorclaw/config.toml` for `events.replay` support |
| Secretary service missing `create_workflow` RPC | Added `secretary.create_workflow` handler in bridge — tests now create workflow from template before starting |
| WebSocket requires registration handshake | Bridge sends no events until client sends `{"type":"register","payload":{"device_id":"..."}}` — all test captures updated |
| zsh brace expansion breaks `${2:-{}}` default params | Changed to `${2:-{\}}` across all test files for POSIX compatibility |

---

## Baseline Test Results (2026-04-30)

Results from full-system test run with `websocket_enabled=true` and `enable_durable_log=true`:

| Suite | PASS | FAIL | SKIP | Status |
|-------|------|------|------|--------|
| `test-secretary-workflow-core.sh` (T3a) | 43 | 0 | 0 | ✅ All pass |
| `test-secretary-workflow-deep.sh` (T3b) | 32 | 0 | 1 | ✅ WD2 skill injection SKIP (expected) |
| `test-eventbus-streaming.sh` (T1) | 15 | 0 | 0 | ✅ All pass (WebSocket enabled) |
| `test-cross-workflow-email.sh` (X1) | 19 | 0 | 0 | ✅ All pass |
| `test-cross-workflow-docs.sh` (X2) | 4 | 0 | 4 | ⏭ Sidecar sockets not deployed |
| `test-cross-browser-trust.sh` (X3) | 3 | 0 | 4 | ⏭ Jetski not deployed (port 9223) |
| `test-cross-event-truth.sh` (X4) | 20 | 0 | 0 | ✅ All pass (WebSocket + durable log) |

**Total: 136 PASS / 0 FAIL / 9 SKIP** — zero product defects, all SKIPs are expected deployment gaps.

### VPS Configuration Required for Full Coverage

```toml
[eventbus]
websocket_enabled = true
enable_durable_log = true
```

---

## Final Verification Checklist

After running the full pipeline, verify these 5 conditions:

| ID | Check | Command |
|----|-------|---------|
| F1 | Contract manifest exists with ≥1 RPC method | `jq '.live_discovered.rpc_methods \| length' .sisyphus/evidence/armorclaw/contract_manifest.json` |
| F2 | Deployment healthy | `curl -skf https://$VPS_IP:$BRIDGE_PORT/health` → `status: "ok"` |
| F3 | Provisioning outputs + admin RPC | `jq '.homeserver_url' .sisyphus/evidence/armorclaw/a2_provisioning_outputs.json` non-empty + ≥1 admin-class RPC responding |
| F4 | Event validation (pass OR documented skip) | `jq '.pass' .sisyphus/evidence/armorclaw/a3_summary.json` ≥1 OR all-SKIP with documented reasons |
| F5 | Harness results (health suite pass) | `jq '.pass' .sisyphus/evidence/armorclaw/a4_summary.json` ≥1 |
| F6 | TLS fingerprint identical across `/fingerprint`, `bridge.status.tls.fingerprint_sha256`, and `openssl x509 -fingerprint -sha256` | `bash tests/test-tls-mode-integration.sh` → PASS |

## TLS Mode Derivation

ArmorClaw derives TLS mode from deployment topology (not a 1:1 mapping from `server.mode`):

| Topology | TLS Mode | Trust Type | Fingerprint |
|----------|----------|------------|-------------|
| `server.mode=native` (Unix socket) | `none` | `""` | `""` |
| `server.mode=sentinel` + self-signed cert | `private` | `self_signed` | SHA-256 of DER-encoded cert |
| `server.mode=sentinel` + CA-issued cert | `public` | `public_ca` | SHA-256 of DER-encoded cert |

**Native mode zero-value semantics**: `bridge.status.tls` is always present. In native mode: `mode="none"`, `fingerprint_sha256=""`, `trust_type=""`, `expires_at=0`, `san_includes_public_ip=false`. These are zero-value (not null) so scripts can use simple string/numeric checks without null handling.

### TLS health values

| Value | Meaning |
|-------|---------|
| `ok` | Bridge has direct access to the active shared certificate material |
| `degraded` | TLS topology is unchanged, but Bridge cannot read the active cert directly (for example `cert_source="proxy_only"`) |

Mode never changes due to health — scripts that only check topology (`private`/`public`/`none`) are stable.

**Shared-cert model**: Bridge and Caddy read from `/etc/armorclaw/certs/server.crt` and `server.key`. `cert_source="shared_cert"` when the cert file is present; `cert_source="proxy_only"` when missing (degraded state).

**Native mode zero-value semantics**: `bridge.status.tls` is always present. In native mode:
`mode="none"`, `fingerprint_sha256=""`, `trust_type=""`, `expires_at=0`,
`san_includes_public_ip=false`.

Example:
```bash
if [[ "$(jq -r '.tls.mode' status.json)" == "none" ]]; then
    echo "Native mode — skip trust flow"
fi
```

**QR payload versioning**: Default is v1 (no TLS fields). V2 (with `tls_mode`, `tls_fingerprint_sha256`, `tls_trust_hint`, `cert_expires_at`) only emitted when `ARMORCLAW_QR_VERSION=2` env var is set.

**Client-side dependency**: ArmorChat's `QRConfigPayload` and `DiscoveredServer` models do not yet include TLS metadata fields. Server-side changes produce v2 QR payloads and extended well-known responses, but ArmorChat (separate codebase) needs updating to consume these fields.
