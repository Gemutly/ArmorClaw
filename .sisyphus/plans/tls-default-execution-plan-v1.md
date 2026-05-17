# TLS Default — Execution Plan (Private-Mode Rollout)

> **Design Doc**: `tls-default-plan-v1.md` (source of truth)
> **Purpose**: Convert the design into an agent-executable work plan with concrete tasks, per-task acceptance, QA scenarios, and evidence outputs.
> **Scope**: **Private-mode-first rollout** — self-signed TLS with shared-cert model. Server-side only. ArmorChat client trust UX is a documented dependency, NOT in this plan. Public CA mode is architecturally supported but verified via unit tests only; full E2E public CA verification is deferred to a future execution phase. **Post-T12 follow-on hardening** (T13–T16) may extend certificate management, discovery, provisioning resilience, and system status surfaces, but does not block completion of the private-mode rollout.

---

## TL;DR

> **Quick Summary**: Make ArmorClaw default to private HTTPS on every deployment. Fix the non-standard fingerprint, add public IP to cert SANs, extend QR payload with TLS metadata (v1→v2 versioned), add TLS mode awareness to Plan A discovery/provision scripts, and add restart-safety verification. **This is a private-mode rollout plan** — public CA mode is architecturally supported but its full E2E verification is deferred.
>
> **Deliverables**:
> - Standardized SHA-256 fingerprint computation in Bridge
> - External IP in certificate SANs (Go + Bash generators)
> - TLS fields in QR ConfigPayload with updated HMAC signature
> - `scripts/lib/tls.sh` wrapping existing cert generator
> - TLS metadata in bridge.status RPC and /discover endpoint
> - Plan A scripts (a0, a1, a2) TLS-aware
> - Restart-safety harness checks
> - Evidence artifacts in `.sisyphus/evidence/tls/`
>
> **Estimated Effort**: Medium (12 core tasks T0-T12 across 3 waves + verification) + Optional post-T12 hardening (T13-T16)
> **Parallel Execution**: YES — 3 core waves + POST-T12 wave
> **Critical Path**: T1 (fingerprint) → T3 (QR payload) → T6 (Plan A scripts) → Final Verification. Post-T12: T13 → T14 → T15 → T16

---

## Context

### Original Request
Convert `tls-default-plan-v1.md` into an agent-executable work plan with task breakdown, per-task acceptance criteria, QA scenarios with exact commands, and concrete evidence outputs.

### Design Decisions (from review)

> **C1 Resolution**: TLS termination follows the design doc (§5, D3). In private mode, the **reverse proxy (Caddy) terminates TLS externally** and proxies to the Bridge over localhost HTTP. The Bridge-internal HTTPS server (`ListenAndServeTLS` in server.go) exists as a fallback for direct-access scenarios, but the default deployment architecture places Caddy in front. This matches the design doc's statement: "TLS terminates at the reverse proxy on the VPS" for private mode, and the same pattern for public mode with CA certs.

- **TLS termination**: Caddy reverse proxy terminates external TLS for both private and public modes. Bridge receives HTTP on localhost. Bridge-internal HTTPS (`ListenAndServeTLS`) remains available for direct-access/non-proxied scenarios but is NOT the default deployment path.
- **Certificate source of truth**: **Shared-cert model** (resolved in T0). Caddy and Bridge read the same cert/key files from a shared path (`/etc/armorclaw/certs/`). This ensures Bridge-derived fingerprinting (via `GetCertificateFingerprint()`) matches the cert actually presented to external clients by Caddy. The deploy script generates certs to this shared path; both Caddy config and Bridge config point to the same files.
- **Trust API direction**: Extend `bridge.status` RPC and `/discover` endpoint — this is the **sole** TLS metadata surface. No separate `/trust/*` endpoints will be created. `bridge.status.tls` is **mandatory** in all supported modes — never omitted. `/discover` and `/.well-known` TLS annotations apply only when the HTTP surface is actually exposed (sentinel/private + sentinel/public modes). In native mode (Unix socket), only `bridge.status.tls` is authoritative. Client trust state machine is ArmorChat's responsibility (documented dependency).
- **scripts/lib/tls.sh**: Wraps `deploy/scripts/generate-certs.sh`. Never duplicates.
- **Fingerprint**: Standard SHA-256 of DER-encoded certificate (replaces current non-standard signature truncation).
- **ArmorChat Phase 3**: OUT OF SCOPE — documented as dependency only. ArmorChat's current `QRConfigPayload` and `DiscoveredServer` shared models do not yet include TLS metadata fields (`tls_mode`, `tls_fingerprint_sha256`, `tls_trust_hint`, `cert_expires_at`). The server produces v2 QR payloads with these fields, but the client will not consume them until a future ArmorChat update. This is an explicit dependency boundary — server changes first, client consumption later.
- **Restart safety**: Explicit test that cert + provisioning outputs are preserved on bridge restart (see T10).
- **TLS mode source of truth**: TLS mode is a **derived property** of the deployment topology, NOT a 1:1 mapping from `server.mode`. Derivation logic: `native` (Unix socket) → `"none"`, `sentinel` (TCP) + self-signed cert → `"private"`, `sentinel` (TCP) + CA-issued cert → `"public"`. Self-signed detection: cert Issuer == Subject. Exposed via new `server.tls_mode` config field (optional override) or auto-detected from cert on disk. Scripts read from `bridge.status.tls.mode`, not from `server.mode` directly.
- **Deployment taxonomy mapping**: The repo's user-facing deployment modes (README, deploy docs) are: Native (local/Unix socket), Sentinel (production VPS + Let's Encrypt), Self-Hosted (home server/LAN + self-signed Caddy). The Bridge config only stores `native`/`sentinel`. This plan maps Self-Hosted to `sentinel` + self-signed cert → TLS mode `"private"`. This is an explicit collapse: **Self-Hosted and private-mode Sentinel are the same TLS topology** (TCP + Caddy + self-signed cert), differing only in whether a public domain is present. Scripts and docs should use `bridge.status.tls.mode` (`"private"`) rather than the user-facing deployment mode name.
  - **ConfigPayload versioning**: T3 increments `ConfigPayload.Version` from 1 to 2. **V1 emitted by default** (safe for current clients). V2 emitted when `ARMORCLAW_QR_VERSION=2` feature flag is set. `ValidateConfig()` accepts both v1 and v2 payloads regardless of emission flag. `signConfig()` uses version-specific signing bases. This prevents mixed-version rollout failures and protects current ArmorChat clients from unknown fields.
  - **Configuration precedence**: When both a config file field and an environment variable can control the same setting, **environment variable wins**. This applies to: `ARMORCLAW_QR_VERSION` overrides `server.qr_version`, `ARMORCLAW_TLS_MODE` overrides `server.tls_mode`, and `ARMORCLAW_PUBLIC_IP` overrides `config.Hostname`. Rationale: config is the steady-state default (deployment-specific), env vars are for testing and rollout overrides (operator-specific). This prevents drift between deploy-time and QA-time behavior.

### Gap Analysis (Metis-style self-review)

**Questions asked and answered**:
- Q: Which TLS termination path for private mode? A: Caddy proxy terminates TLS externally (matches design doc §5, D3). Bridge-internal HTTPS is fallback only.
- Q: Certificate source of truth — Bridge cert or proxy cert? A: **Shared-cert model**. Both Caddy and Bridge read from the same cert/key files at `/etc/armorclaw/certs/`. Deploy generates once, both consume. Bridge-derived fingerprints are valid for external trust metadata because they're the same cert.
- Q: Trust API — separate endpoints or extend existing? A: Extend existing (bridge.status, /discover) ONLY. No `/trust/*` endpoints.
- Q: Does scripts/lib/tls.sh replace generate-certs.sh? A: No, wraps it.
- Q: Is `tls_trust_hint` signed in the QR payload? A: No — `tls_trust_hint` is explicitly **non-authoritative informational metadata** (like `"self_signed"` vs `"public_ca"`). It is NOT included in the signConfig HMAC. Only `tls_mode` and `tls_fingerprint_sha256` are security-relevant and signed.
- Q: Where does external IP come from? A: `ARMORCLAW_PUBLIC_IP` env var, or config.Hostname if it's an IP. No outbound internet calls during cert generation. Deterministic and offline-safe.
- Q: TLS mode source of truth? A: Derived from deployment topology (`server.mode` + cert self-signed detection), with optional `server.tls_mode` override. Scripts read `bridge.status.tls.mode`, not `server.mode` directly.

**Guardrails applied**:
- G1: Do NOT modify ArmorChat code (separate codebase)
- G2: Do NOT duplicate generate-certs.sh, ssl.go, or server.go cert generation
- G3: Do NOT change fingerprint format without updating ALL consumers (qr/public.go signConfig, /fingerprint endpoint, /discover endpoint, bridge.status)
- G4: Do NOT break existing HMAC signature validation — `tls_mode` and `tls_fingerprint_sha256` are signed; `tls_trust_hint` and `cert_expires_at` are unsigned informational metadata
- G5: Do NOT rotate certs on bridge restart unless ROTATE_TLS_CERT=1
- G6: Do NOT remove existing HTTP-over-localhost-via-SSH path used by Plan A scripts
- G7: Do NOT make /.well-known required in private mode

**Scope creep areas locked down**:
- Client trust UX → dependency, not task
- Private CA root hierarchy → deferred (plan says "optional later enhancement")
- Caddy config generation → out of scope (configs already exist)
- Cloudflare tunnel/proxy mode → no changes needed (already uses CA certs)

**Assumptions validated**:
- Bridge already generates self-signed certs on startup → confirmed (server.go:296-318)
- /fingerprint endpoint exists → confirmed (server.go:527-538)
- ConfigPayload has no TLS fields → confirmed (qr/public.go:264-274)
- signConfig hardcodes signed fields → confirmed (qr/public.go:410-423)
- 3 cert generators exist → confirmed (server.go:322-354, ssl.go:56-148, generate-certs.sh:146-199)

---

## Work Objectives

### Core Objective
Make every new ArmorClaw deployment use HTTPS by default (private-mode/self-signed), with correct fingerprinting, complete SAN coverage, and TLS metadata in all provisioning/status surfaces. Public CA mode is architecturally supported but fully verified in a future execution phase.

### Concrete Deliverables
- `bridge/pkg/http/server.go` — fixed fingerprint computation
- `bridge/pkg/http/server.go` — external IP in self-signed cert SANs
- `bridge/pkg/qr/public.go` — ConfigPayload with TLS fields + updated signConfig
- `bridge/pkg/rpc/server.go` — bridge.status includes TLS metadata
- `bridge/pkg/http/server.go` — /discover includes TLS mode, fingerprint, expiry
- `bridge/pkg/setup/ssl.go` — external IP in SANs
- `scripts/lib/tls.sh` — thin wrapper around deploy/scripts/generate-certs.sh
- `scripts/a0_discover.sh` — records TLS metadata in manifest
- `scripts/a1_deploy.sh` — generates certs with public IP SAN
- `scripts/a2_provision.sh` — captures TLS fields in provisioning outputs
- `scripts/a4_harness.sh` — TLS-aware health suite entry
- Evidence artifacts in `.sisyphus/evidence/tls/`

### Definition of Done
> **Phase scope**: This phase validates **private mode (self-signed HTTPS)** end-to-end. Public CA mode is architecturally supported and verified via unit tests only. Full E2E public CA validation is deferred to a future execution phase.

- [ ] External HTTPS health check returns valid cert with public IP in SAN (private/self-signed mode)
- [ ] In private mode, `bridge.status.tls.health == "ok"` and `cert_source == "shared_cert"` — shared-cert correctness is part of the rollout contract, not just an internal test detail
- [ ] `bridge.status.tls` and `/discover` return identical TLS metadata (mode, fingerprint, trust_type) — **always-on, authoritative TLS contract**
- [ ] `/qr/config` defaults to v1 (no TLS fields) — safe for current ArmorChat clients
- [ ] `/qr/config` emits v2 with TLS fields when `ARMORCLAW_QR_VERSION=2` is set, and v2 signature validates
- [ ] `bridge.status.tls.fingerprint_sha256` matches `/fingerprint` endpoint and `openssl` output (cross-validated)
- [ ] Bridge restart preserves cert and all provisioning outputs
- [ ] Plan A discovery records TLS mode and fingerprint
- [ ] No-domain deployment comes up with private HTTPS by default

### Must Have
- SHA-256 DER fingerprint (standard, not signature truncation)
- External/public IP in certificate SAN (from env var or config, no outbound calls)
- TLS metadata in `bridge.status` RPC response and `/discover` endpoint (**always-on, authoritative**)
- TLS fields structurally supported in QR ConfigPayload with valid HMAC when v2 emission is enabled (`tls_mode` and `tls_fingerprint_sha256` signed; `tls_trust_hint` and `cert_expires_at` are unsigned informational metadata)
- QR v1 emission by default, v2 gated behind `ARMORCLAW_QR_VERSION=2` flag
- Restart-safety test

### Must NOT Have (Guardrails)
- No ArmorChat code changes (separate codebase)
- No duplicate cert generation scripts
- No cert rotation on restart without explicit flag
- No /.well-known requirement in private mode
- No breaking change to existing ConfigPayload signature validation (coordinate signConfig update)
- No removal of HTTP-over-localhost-via-SSH path

---

## Verification Strategy

> **ZERO HUMAN INTERVENTION** — ALL verification is agent-executed.

### Test Decision
- **Infrastructure exists**: YES (Go tests, bash tests)
- **Automated tests**: Tests-after (update existing test files alongside code changes)
- **Framework**: Go test (bridge), bash (scripts)

### QA Policy
Every task includes agent-executed QA scenarios with exact commands.
Evidence saved to `.sisyphus/evidence/tls/`.

- **Bridge Go code**: Use Bash (`go test`) — run unit tests, verify output
- **HTTP endpoints**: Use Bash (`curl`) — hit endpoints, assert JSON fields
- **Certificate inspection**: Use Bash (`openssl`) — inspect SAN, fingerprint, expiry
- **Scripts**: Use Bash — run scripts, check exit codes and output

### Test Harness Prerequisites

> These are stated once here instead of being scattered across individual QA scenarios.
> If any prerequisite is missing, the affected task MUST report SKIP with a clear reason.

| Prerequisite | Why Needed | Used By |
|-------------|-----------|---------|
| **SSH access** to VPS (`ssh root@$VPS_IP`) | Remote cert inspection, docker restart, file reads | T2, T5, T7, T9, T10, F3 |
| **jq** | JSON parsing for curl/RPC responses | All HTTP endpoint tasks |
| **openssl** | Cert fingerprint cross-validation, SAN inspection | T1, T2, T9, T10 |
| **Docker control** on VPS (`docker restart`, `docker logs`) | Bridge restart for safety tests | T9, T10 |
| **Read access** to `/etc/armorclaw/certs/` on VPS | Shared cert file inspection (shared-cert model) | T0, T2, T5 |
| **curl** with `-skf` support | Self-signed TLS + fail-on-error for endpoint tests | All HTTP tasks |
| **socat** or **websocat** | Unix socket RPC in native mode | T5 (native fallback) |
| **ARMORCLAW_QR_VERSION** env var settable | V2 QR flagged-path testing | T3, T7, T10 |

---

## Execution Strategy

### Parallel Execution Waves

```
Wave 0 (Design Lock — must complete before any code):
└── T0: TLS metadata source-of-truth alignment [quick]

Wave 1 (Foundation — depends on T0):
├── T1: Fix fingerprint to standard SHA-256 DER [quick]
├── T2: Add external IP to cert SANs (Go generators) [quick]
├── T3: Extend QR ConfigPayload with TLS fields + versioning [unspecified-high]
└── T4: Add scripts/lib/tls.sh wrapper [quick]

Wave 2 (Integration — depends on Wave 1):
├── T5: Add TLS metadata to bridge.status + /discover (depends: T0, T1) [unspecified-high]
├── T6: Update Plan A scripts for TLS awareness (depends: T1, T4) [unspecified-high]
├── T7: Update Plan A provision script for TLS outputs (depends: T3) [unspecified-high]
│     └── **Rollout gate**: Default path validates v1 QR emission (no TLS fields).
│         QR v2 contract (only when `ARMORCLAW_QR_VERSION=2`) runs only after bridge.status passes.
└── T8: Annotate /.well-known with TLS mode metadata (depends: T5) [quick]

Wave 3 (Verification — depends on Wave 2):
├── T9: TLS-mode integration test (depends: T5, T6, T7, T8) [unspecified-high]
│     └── **Rollout gate**: 9 always-on scenarios use bridge.status as authoritative TLS source.
│         QR v2 contract (only when `ARMORCLAW_QR_VERSION=2`) runs only when flag is set.
├── T10: Cert & provisioning preservation test (depends: T5, T6, T7) [deep]
├── T11: Documentation update (depends: all) [quick]
└── T12: Final evidence collection (depends: T9, T10) [quick]

Wave FINAL (After ALL tasks — 5 verification steps in parallel):
├── F1: Plan compliance audit
├── F2: Code quality review
├── F3: Full Automated QA Execution
├── F4: Scope fidelity check
└── F5: Documentation reconciliation (update /doc/ markdown)
→ Private-mode rollout COMPLETE. All evidence saved.

Wave POST-T12 (Follow-on Hardening — non-blocking, after F1-F4 verification passes):
├── T13: Unified Certificate Manager foundation (depends: T0, T1, T2, T5) [deep]
├── T14: Expand bridge.status into system source of truth (depends: T5) [unspecified-high]
├── T15: mDNS TLS advertisement + discovery ingestion (depends: T6, T8, T14) [unspecified-high]
└── T16: Provisioning resilience layer (depends: T7, T9) [unspecified-high]
→ Recommended sequence: T13 → T14 → T15 → T16

Deferred Phase 2 (separate plan):
├── T17: ACME / Let's Encrypt issuance
├── T18: Renewal + expiry monitoring
├── T19: Public-mode well-known / domain routing
└── T20: Full public-CA E2E harness

Deferred Phase 3 (separate cross-system plan):
└── OpenClaw trust reporting (depends on T14)

Critical Path: T0 → T1 → T5 → T9 → F1-F4 → (optional: T13 → T14 → T15 → T16)
Parallel Speedup: ~55% faster than sequential (T0-T12)
```

### Dependency Matrix

| Task | Depends On | Blocks | Wave |
|------|-----------|--------|------|
| T0 | — | T1-T12 (all) | 0 |
| T1 | T0 | T5, T6, T9, T10 | 1 |
| T2 | T0 | T6 | 1 |
| T3 | T0 | T7, T9, T10 | 1 |
| T4 | T0 | T6 | 1 |
| T5 | T0, T1 | T8, T9, T10 | 2 |
| T6 | T1, T4 | T9, T10 | 2 |
| T7 | T3 | T9, T10 | 2 |
| T8 | T5 | T9 | 2 |
| T9 | T5, T6, T7, T8 | T12, F1-F4 | 3 |
| T10 | T5, T6, T7 | T12, F1-F4 | 3 |
| T11 | all | F1-F4 | 3 |
| T12 | T9, T10 | F1-F4 | 3 |
| T13 | T0, T1, T2, T5 | T15 | POST-T12 |
| T14 | T5 | T15, future T17+ | POST-T12 |
| T15 | T6, T8, T14 | — | POST-T12 |
| T16 | T7, T9 | — | POST-T12 |

### Agent Dispatch Summary

- **Wave 0**: 1 task — T0 → `quick`
- **Wave 1**: 4 tasks — T1, T2, T4 → `quick`, T3 → `unspecified-high`
- **Wave 2**: 4 tasks — T5, T6, T7 → `unspecified-high`, T8 → `quick`
- **Wave 3**: 4 tasks — T9 → `unspecified-high`, T10 → `deep`, T11, T12 → `quick`
- **FINAL**: 4 verification steps (run in-process, not delegated)
- **POST-T12**: 4 tasks — T13 → `deep`, T14 → `unspecified-high`, T15 → `unspecified-high`, T16 → `unspecified-high`

---

## TODOs

- [x] 0. TLS Metadata Source-of-Truth Alignment

  **What to do**:
  - Document and enforce the **shared-cert model**: Caddy and Bridge read the same cert/key files from a shared path.
  - Verify that `deploy/scripts/generate-certs.sh` writes to `/etc/armorclaw/certs/` (it already does — `OUTPUT_DIR` defaults to this path).
  - Verify that `configs/Caddyfile.selfhosted` reads from `/etc/armorclaw/certs/server.crt` and `server.key` (it already does — line 9: `tls /etc/armorclaw/certs/server.crt /etc/armorclaw/certs/server.key`).
  - Verify that Bridge's `loadOrGenerateCerts()` in `bridge/pkg/http/server.go` reads from the same path (currently reads from `CertDir` which defaults to `/var/lib/armorclaw/certs` — **MISMATCH**). **Resolution**: change the default `CertDir` to `/etc/armorclaw/certs` to match Caddy and the deploy script. This is the cleaner option — single shared path, no copy/symlink complexity, no divergent cert states.
  - Update `bridge/pkg/config/config.go` if needed: ensure the cert path configuration matches the shared cert location.
  - Document the shared-cert contract: deploy generates to `/etc/armorclaw/certs/`, Caddy reads from there, Bridge reads from there. Single source of truth for fingerprint, expiry, and SAN.

  **Must NOT do**:
  - Do NOT create a separate cert discovery mechanism — read from shared files
  - Do NOT assume Bridge and Caddy always use different certs (they share in the default deployment)
  - Do NOT remove the Bridge-internal HTTPS fallback entirely
  - Do NOT add copy/symlink logic to bridge two cert paths — use one shared path instead

  **Recommended Agent Profile**:
  - **Category**: `quick`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: NO — this is a prerequisite for all other tasks
  - **Parallel Group**: Wave 0 (solo)
  - **Blocks**: T1-T12 (all)
  - **Blocked By**: None

  **References**:
  - `deploy/scripts/generate-certs.sh:25` — `OUTPUT_DIR="/etc/armorclaw/certs"` (deploy cert location)
  - `configs/Caddyfile.selfhosted:9` — `tls /etc/armorclaw/certs/server.crt /etc/armorclaw/certs/server.key` (Caddy cert location)
  - `bridge/pkg/http/server.go:116-118` — `config.CertDir` defaults to `/var/lib/armorclaw/certs` (**MISMATCH** with Caddy)
  - `bridge/pkg/http/server.go:266-294` — `loadOrGenerateCerts()` reads from CertDir
  - `bridge/pkg/config/config.go` — Check if CertDir is configurable

  **Acceptance Criteria**:
  - [ ] Default cert path for Bridge matches Caddy's cert path (both read from `/etc/armorclaw/certs/`)
  - [ ] Documented: shared-cert model is the default, Bridge-internal HTTPS is fallback
  - [ ] `cd bridge && go build ./...` compiles after any path changes

  **QA Scenarios**:

  ```
  Scenario: Bridge default cert path matches Caddy cert path (source-level)
    Tool: Bash (grep)
    Preconditions: Source code available
    Steps:
      1. grep -r "certs" configs/Caddyfile.selfhosted — note the cert path
      2. grep -r "CertDir\|cert_dir\|/var/lib/armorclaw/certs\|/etc/armorclaw/certs" bridge/pkg/http/server.go bridge/pkg/config/config.go
      3. Assert Bridge default matches Caddy config path
    Expected Result: Both use /etc/armorclaw/certs/
    Failure Indicators: Bridge uses /var/lib/armorclaw/certs while Caddy uses /etc/armorclaw/certs
    Evidence: .sisyphus/evidence/tls/task0-cert-path-alignment.txt

  Scenario: Runtime cert path verification (running Bridge uses shared cert path)
    Tool: Bash (curl + jq)
    Preconditions: Bridge running on VPS with HTTPS
    Steps:
      1. BRIDGE_CERT_DIR=$(curl -skf https://$VPS_IP:$BRIDGE_PORT/api -d '{"jsonrpc":"2.0","id":1,"method":"bridge.status","params":{}}' | jq -r '.result.config.cert_dir // empty')
      2. if [ -n "$BRIDGE_CERT_DIR" ]; then test "$BRIDGE_CERT_DIR" = "/etc/armorclaw/certs"; fi
      3. Always verify fingerprint match regardless of cert_dir availability:
         BRIDGE_FP=$(curl -skf https://$VPS_IP:$BRIDGE_PORT/fingerprint | jq -r '.sha256')
         DISK_FP=$(ssh root@$VPS_IP "openssl x509 -in /etc/armorclaw/certs/server.crt -fingerprint -sha256 -noout | cut -d= -f2 | tr -d ':' | tr 'A-F' 'a-f'")
         test "$BRIDGE_FP" = "$DISK_FP"
    Expected Result: Bridge reads certs from /etc/armorclaw/certs/ AND fingerprint matches disk cert
    Failure Indicators: cert_dir mismatch or fingerprint mismatch (Bridge using different cert than Caddy)
    Evidence: .sisyphus/evidence/tls/task0-runtime-cert-verification.txt
  ```

  **Commit**: YES
  - Message: `fix(bridge): align default cert path with Caddy shared-cert model`
  - Files: `bridge/pkg/http/server.go`, possibly `bridge/pkg/config/config.go`
  - Pre-commit: `cd bridge && go build ./...`

- [x] 1. Fix Fingerprint to Standard SHA-256 DER

  **What to do**:
  - In `bridge/pkg/http/server.go`, replace `GetCertificateFingerprint()` (lines 250-263) with standard SHA-256 of DER-encoded certificate:
    ```go
    func (s *Server) GetCertificateFingerprint() (string, error) {
        block, _ := pem.Decode(s.certPEM)
        if block == nil {
            return "", fmt.Errorf("failed to decode certificate PEM")
        }
        // Standard SHA-256 of DER-encoded certificate
        hash := sha256.Sum256(block.Bytes)
        return fmt.Sprintf("%x", hash), nil
    }
    ```
  - Add `"crypto/sha256"` to imports if not already present
  - Update the existing test in `bridge/pkg/setup/ssl_test.go` to verify the fingerprint format is 64 lowercase hex characters matching `openssl x509 -fingerprint -sha256`

  **Must NOT do**:
  - Do NOT change the function signature (callers depend on it returning `(string, error)`)
  - Do NOT change the `/fingerprint` endpoint response format (`{"sha256": "...", "format": "hex"}`)
  - Do NOT modify ArmorChat fingerprint consumption code

  **Recommended Agent Profile**:
  - **Category**: `quick`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 1 (with T2, T3, T4)
  - **Blocks**: T5, T6, T9, T10
  - **Blocked By**: None

  **References**:
  - `bridge/pkg/http/server.go:250-263` — Current `GetCertificateFingerprint()` using `cert.Signature` (WRONG)
  - `bridge/pkg/http/server.go:527-538` — `/fingerprint` endpoint that calls this function (do NOT change)
  - `bridge/pkg/http/server.go:494` — `/discover` endpoint that calls this function
  - `bridge/pkg/setup/ssl_test.go` — Existing test for cert generation, extend with fingerprint assertion
  - `bridge/pkg/http/server.go:528` — `/fingerprint` handler format remains `{"sha256": hex_string}`

  **Acceptance Criteria**:
  - [ ] `GetCertificateFingerprint()` returns lowercase hex string exactly 64 chars
  - [ ] `cd bridge && go test ./pkg/http/... ./pkg/setup/...` → PASS
  - [ ] Fingerprint matches `openssl x509 -fingerprint -sha256 -noout` output (after removing colons and lowercasing)

  **QA Scenarios**:

  ```
  Scenario: Fingerprint matches standard OpenSSL SHA-256
    Tool: Bash (go test)
    Preconditions: Bridge code compiles
    Steps:
      1. Generate a test cert using existing ssl.go GenerateSelfSignedCert()
      2. Compute fingerprint via GetCertificateFingerprint()
      3. Compute fingerprint via: pem.Decode → sha256.Sum256(der) → hex
      4. Compare — must match
    Expected Result: Both fingerprints are identical 64-char lowercase hex strings
    Failure Indicators: Length ≠ 64, contains uppercase, mismatch between methods
    Evidence: .sisyphus/evidence/tls/task1-fingerprint-test.txt

  Scenario: Fingerprint matches OpenSSL output
    Tool: Bash
    Preconditions: Test cert PEM written to temp file
    Steps:
      1. Write cert PEM from ssl_test to /tmp/test-cert.pem
      2. Run: openssl x509 -in /tmp/test-cert.pem -fingerprint -sha256 -noout | cut -d= -f2 | tr -d ':' | tr 'A-F' 'a-f'
      3. Run: go test -run TestFingerprintSHA256 ./pkg/setup/... and capture Go-side fingerprint
      4. Diff the two values
    Expected Result: Exact match
    Failure Indicators: Any character difference
    Evidence: .sisyphus/evidence/tls/task1-openssl-comparison.txt
  ```

  **Commit**: YES
  - Message: `fix(bridge): use standard SHA-256 DER fingerprint instead of signature truncation`
  - Files: `bridge/pkg/http/server.go`, `bridge/pkg/setup/ssl_test.go`
  - Pre-commit: `cd bridge && go test ./pkg/http/... ./pkg/setup/...`

- [x] 2. Add External IP to Certificate SANs (Go Generators)

  **What to do**:
  - In `bridge/pkg/http/server.go`, update `generateSelfSignedCert()` (lines 322-354) to include externally-configured IP in SANs:
    - Add a helper `getConfiguredExternalIP() net.IP` that tries:
      1. Check `ARMORCLAW_PUBLIC_IP` env var
      2. Parse `s.config.Hostname` — if it's an IP (not a hostname), use it
      3. If neither available: return nil (no external IP added, log warning)
    - If the returned IP is non-nil, add it to `template.IPAddresses`
    - **No outbound internet calls** — deterministic and offline-safe
  - In `bridge/pkg/setup/ssl.go`, update `GenerateSelfSignedCert()` (lines 56-148):
    - Add `PublicIP string` field to `SSLConfig` struct
    - If `PublicIP` is non-empty, parse it and add to `template.IPAddresses` alongside the existing localhost IPs
  - Update `SSLConfig.AdditionalSANs` documentation to note it accepts IPs too

  **Must NOT do**:
  - Do NOT remove localhost, 127.0.0.1, ::1 from SANs (needed for local dev)
  - Do NOT block cert generation if external IP detection fails — log warning, continue without it
  - Do NOT add external IP detection to the bash cert generator (T4 handles that)

  **Recommended Agent Profile**:
  - **Category**: `quick`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 1 (with T1, T3, T4)
  - **Blocks**: T6
  - **Blocked By**: None

  **References**:
  - `bridge/pkg/http/server.go:322-354` — `generateSelfSignedCert()` method, currently adds only local IPs
  - `bridge/pkg/http/server.go:328-331` — `getLocalIPs()` helper (pattern to follow for `getExternalIP()`)
  - `bridge/pkg/setup/ssl.go:31-46` — `SSLConfig` struct, add `PublicIP string` field
  - `bridge/pkg/setup/ssl.go:115-116` — Current hardcoded `IPAddresses: []net.IP{127.0.0.1, ::1}`
  - `bridge/pkg/setup/ssl_test.go` — Existing tests, extend with PublicIP test case

  **Acceptance Criteria**:
  - [ ] When `ARMORCLAW_PUBLIC_IP` is set, cert SAN includes that IP
  - [ ] When config.Hostname is an IP address, cert SAN includes that IP
  - [ ] When no external IP is available, cert generation still succeeds (warning logged)
  - [ ] `cd bridge && go test ./pkg/http/... ./pkg/setup/...` → PASS

  **QA Scenarios**:

  ```
  Scenario: SAN includes explicit public IP from env var
    Tool: Bash (go test)
    Preconditions: None
    Steps:
      1. Set ARMORCLAW_PUBLIC_IP=203.0.113.10 in test
      2. Generate cert via generateSelfSignedCert()
      3. Parse cert and inspect SAN
    Expected Result: SAN contains "IP Address:203.0.113.10"
    Failure Indicators: IP missing from SAN
    Evidence: .sisyphus/evidence/tls/task2-san-env-ip.txt

  Scenario: SAN includes hostname-as-IP
    Tool: Bash (go test)
    Preconditions: config.Hostname = "5.183.11.149"
    Steps:
      1. Generate cert with Hostname set to an IP
      2. Parse cert SAN
    Expected Result: SAN contains "IP Address:5.183.11.149"
    Failure Indicators: IP missing, only localhost present
    Evidence: .sisyphus/evidence/tls/task2-san-hostname-ip.txt

  Scenario: Cert generation succeeds without external IP
    Tool: Bash (go test)
    Preconditions: No ARMORCLAW_PUBLIC_IP, Hostname = "armorclaw.local"
    Steps:
      1. Generate cert with no external IP available
      2. Verify cert was generated successfully
      3. Verify SAN still has localhost entries
    Expected Result: Cert generated, SAN has localhost only, no error
    Failure Indicators: Cert generation fails
    Evidence: .sisyphus/evidence/tls/task2-san-no-external.txt
  ```

  **Commit**: YES
  - Message: `fix(bridge): include public/external IP in self-signed cert SANs`
  - Files: `bridge/pkg/http/server.go`, `bridge/pkg/setup/ssl.go`, `bridge/pkg/setup/ssl_test.go`
  - Pre-commit: `cd bridge && go test ./pkg/http/... ./pkg/setup/...`

- [x] 3. Extend QR ConfigPayload with TLS Fields + Version Bump

  **What to do**:
  - In `bridge/pkg/qr/public.go`, add TLS fields to `ConfigPayload` struct (lines 264-274):
    ```go
    type ConfigPayload struct {
        Version               int    `json:"version"`
        MatrixHomeserver      string `json:"matrix_homeserver"`
        RpcURL                string `json:"rpc_url"`
        WsURL                 string `json:"ws_url,omitempty"`  // Make optional
        PushGateway           string `json:"push_gateway"`
        ServerName            string `json:"server_name"`
        Region                string `json:"region,omitempty"`
        TLSMode               string `json:"tls_mode"`                    // "private" | "public"
        TLSFingerprintSHA256  string `json:"tls_fingerprint_sha256"`      // Standard SHA-256 DER
        TLSTrustHint          string `json:"tls_trust_hint"`              // "self_signed" | "public_ca"
        CertExpiresAt         int64  `json:"cert_expires_at"`             // Unix timestamp
        ExpiresAt             int64  `json:"expires_at"`
        Signature             string `json:"signature"`
    }
    ```
  - **Version bump**: Increment `ConfigPayload.Version` from 1 to 2. All new payloads are v2. TLS fields (`tls_mode`, `tls_fingerprint_sha256`) are zero-valued in v1 and populated in v2.
  - **Backward compatibility**: Update `ValidateConfig()` to accept both v1 and v2 payloads. Use version-specific signing bases:
    - v1: original signing string (no TLS fields) — for payloads generated before this change
    - v2: new signing string with `tls_mode` and `tls_fingerprint_sha256` included
    - `ValidateConfig()` checks `config.Version` and uses the appropriate signing basis
  - **CRITICAL**: Update `signConfig()` (lines 410-423) to use the v2 signing string with security-relevant TLS fields. `tls_trust_hint` is explicitly **NOT signed** — it is non-authoritative informational metadata (like a content-type hint), not a security claim. Only `tls_mode` and `tls_fingerprint_sha256` are signed because they are security-relevant. The v2 signature string:
    ```go
    // v2 signing basis
    data := fmt.Sprintf("%d:%s:%s:%s:%s:%s:%s:%s:%d",
        config.Version,       // now 2
        config.MatrixHomeserver,
        config.RpcURL,
        config.WsURL,
        config.PushGateway,
        config.ServerName,
        config.TLSMode,
        config.TLSFingerprintSHA256,
        config.ExpiresAt,
    )
    ```
    Note: `CertExpiresAt` is also NOT signed — it's advisory metadata (the cert may be rotated independently of the QR payload). The authoritative expiry is the QR `expires_at` field.
  - In `ValidateConfig()`, add version routing:
    ```go
    var data string
    if config.Version == 1 {
        // v1: original signing basis (no TLS fields)
        data = fmt.Sprintf("%d:%s:%s:%s:%s:%s:%d", ...)
    } else {
        // v2: includes tls_mode and tls_fingerprint_sha256
        data = fmt.Sprintf("%d:%s:%s:%s:%s:%s:%s:%s:%d", ...)
    }
    ```
  - Make `WsURL` generation conditional — only set it when explicitly configured (omit from payload when empty)
  - In `bridge/pkg/http/server.go`, update `handleQRConfig()` and `GenerateConfigQR()` call sites to pass TLS metadata from the server's certificate and config

  **Must NOT do**:
  - Do NOT change the HMAC algorithm (remains HMAC-SHA256)
  - Do NOT remove any existing fields from ConfigPayload
  - Do NOT hardcode tls_mode — derive from bridge config
  - Do NOT reject v1 payloads — they must still validate correctly

  **Rollout gating — feature-flagged v2 emission**: `/qr/config` emits **v1 payloads by default** (safe for current ArmorChat clients). V2 payloads (with TLS fields) are emitted only when `server.qr_version = 2` is set in bridge config or `ARMORCLAW_QR_VERSION=2` env var. **Precedence**: env var overrides config file (env wins for testing/rollout; config is steady-state default). This ensures:
- Current clients continue working unchanged (v1 has no TLS fields, no unknown-field risk)
- Server-side TLS metadata is always available via `bridge.status` and `/discover` (not gated)
- Ops can test v2 payloads locally (curl /qr/config with flag on) before ArmorChat lands the client update
- After ArmorChat adds TLS field support, flip the flag to v2 for end-to-end consumption
- `ValidateConfig()` accepts both v1 and v2 payloads regardless of the emission flag (server validates what it receives)

**Rollout sequence**: Deploy with flag off (v1 emission) → verify bridge.status + /discover work → test v2 locally → wait for ArmorChat client update → flip flag to v2 → end-to-end trust flow works.

  **Recommended Agent Profile**:
  - **Category**: `unspecified-high` (multiple files + signature logic changes)
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 1 (with T1, T2, T4)
  - **Blocks**: T7, T9, T10
  - **Blocked By**: None (but logically T1 fingerprint fix should be in same commit series)

  **References**:
  - `bridge/pkg/qr/public.go:264-274` — Current `ConfigPayload` struct (no TLS fields)
  - `bridge/pkg/qr/public.go:278-332` — `GenerateConfigQR()` that creates the payload
  - `bridge/pkg/qr/public.go:410-423` — `signConfig()` that MUST be updated to include TLS fields
  - `bridge/pkg/qr/public.go:362-375` — `ValidateConfig()` that verifies signature
  - `bridge/pkg/http/server.go:144-151` — QR manager initialization with server URLs
  - `bridge/pkg/http/server.go:593-615` — `handleQRConfig()` that calls GenerateConfigQR

  **Acceptance Criteria**:
  - [ ] `/qr/config` emits v1 by default (no TLS fields) — safe for current ArmorChat clients
  - [ ] `/qr/config` emits v2 with TLS fields when `ARMORCLAW_QR_VERSION=2` is set
  - [ ] `ConfigPayload.Version` is 1 by default, 2 when flag is set
  - [ ] `ws_url` is absent from payload when not configured
  - [ ] HMAC signature validates correctly with both v1 and v2 signing bases
  - [ ] v1 payloads still validate correctly via `ValidateConfig()` (server accepts both)
  - [ ] `cd bridge && go test ./pkg/qr/...` → PASS

  **QA Scenarios**:

  ```
  Scenario: QR config defaults to v1 (safe for current clients)
    Tool: Bash (curl)
    Preconditions: Bridge running with HTTPS, ARMORCLAW_QR_VERSION not set
    Steps:
      1. curl -skf https://$VPS_IP:$BRIDGE_PORT/qr/config | jq '.config'
      2. Assert .version == 1
      3. Assert .tls_mode is absent (v1 has no TLS fields)
      4. Assert .tls_fingerprint_sha256 is absent
    Expected Result: v1 payload emitted by default — no TLS fields, safe for current ArmorChat
    Failure Indicators: Version is 2, TLS fields present when flag is off
    Evidence: .sisyphus/evidence/tls/task3-qr-config-v1-default.json

  Scenario: QR config emits v2 when feature flag is set
    Tool: Bash (curl)
    Preconditions: Bridge running with HTTPS, ARMORCLAW_QR_VERSION=2 set
    Steps:
      1. curl -skf https://$VPS_IP:$BRIDGE_PORT/qr/config | jq '.config'
      2. Assert field exists: .tls_mode
      3. Assert field exists: .tls_fingerprint_sha256
      4. Assert field exists: .tls_trust_hint
      5. Assert field exists: .cert_expires_at
      6. Assert .version == 2
      7. Assert .tls_fingerprint_sha256 is exactly 64 lowercase hex chars
    Expected Result: All 4 TLS fields present, version is 2, fingerprint is valid
    Failure Indicators: Missing field, fingerprint wrong length, version is 1
    Evidence: .sisyphus/evidence/tls/task3-qr-config-v2-flagged.json

  Scenario: HMAC signature validates with v2 TLS fields
    Tool: Bash (go test)
    Preconditions: ConfigPayload generates with TLS fields and Version=2
    Steps:
      1. Generate ConfigPayload with known test values (Version=2)
      2. Call ValidateConfig()
      3. Assert no error returned
    Expected Result: Valid v2 config passes validation
    Failure Indicators: Valid v2 config fails
    Evidence: .sisyphus/evidence/tls/task3-qr-signature-v2.txt

  Scenario: v1 payload backward compatibility
    Tool: Bash (go test)
    Preconditions: ValidateConfig() updated for version routing
    Steps:
      1. Construct a ConfigPayload with Version=1, no TLS fields (zero values)
      2. Sign with v1 signing basis (original format without TLS fields)
      3. Call ValidateConfig()
      4. Assert no error returned
    Expected Result: v1 payload validates correctly with old signing basis
    Failure Indicators: v1 payload rejected as invalid
    Evidence: .sisyphus/evidence/tls/task3-qr-signature-v1-compat.txt

  Scenario: Tampering with signed fields invalidates signature
    Tool: Bash (go test)
    Preconditions: ConfigPayload generates with TLS fields
    Steps:
      1. Generate ConfigPayload with known test values
      2. Tamper with .tls_mode (change "private" to "public")
      3. Call ValidateConfig() — assert "invalid config signature" error
      4. Restore tls_mode, tamper with .tls_fingerprint_sha256 (change first char)
      5. Call ValidateConfig() — assert "invalid config signature" error
    Expected Result: Both tampered signed fields cause signature failure
    Failure Indicators: Tampered config passes validation
    Evidence: .sisyphus/evidence/tls/task3-signed-tamper.txt

  Scenario: Tampering with unsigned fields does NOT invalidate signature
    Tool: Bash (go test)
    Preconditions: ConfigPayload generates with TLS fields
    Steps:
      1. Generate ConfigPayload with known test values
      2. Tamper with .tls_trust_hint (change "self_signed" to "public_ca")
      3. Call ValidateConfig() — assert NO error (signature still valid)
      4. Restore tls_trust_hint, tamper with .cert_expires_at (change to 0)
      5. Call ValidateConfig() — assert NO error (signature still valid)
    Expected Result: Tampered unsigned fields do NOT invalidate signature
    Failure Indicators: Tampering unsigned field causes signature failure
    Evidence: .sisyphus/evidence/tls/task3-unsigned-tamper.txt
  ```

  **Commit**: YES
  - Message: `feat(bridge): add TLS metadata fields to QR ConfigPayload (v1→v2)`
  - Files: `bridge/pkg/qr/public.go`, `bridge/pkg/http/server.go`
  - Pre-commit: `cd bridge && go test ./pkg/qr/...`

- [x] 4. Add scripts/lib/tls.sh Wrapper

  **What to do**:
  - Create `scripts/lib/tls.sh` as a thin wrapper around `deploy/scripts/generate-certs.sh`:
    - Source `scripts/lib/contract.sh` (follows existing library pattern)
    - Provide `tls_get_mode()` — queries bridge.status RPC and reads `.tls.mode` from the response. Single source of truth is the bridge config, not independent env var heuristics. Falls back to `ARMORCLAW_TLS_MODE` env var only if bridge is unreachable (bootstrap-only fallback, not a co-equal source of truth).
    - Provide `tls_generate_certs()` — calls `deploy/scripts/generate-certs.sh` with correct `--hostname`, `--lan-ip`, `--output` args
    - Provide `tls_get_fingerprint()` — reads cert from disk, computes SHA-256 fingerprint via openssl
    - Provide `tls_get_expiry()` — reads cert from disk, returns expiry timestamp
    - Provide `tls_metadata()` — outputs JSON with mode, fingerprint, expiry, trust_type
  - Add `--public-ip` argument forwarding to `deploy/scripts/generate-certs.sh` (requires adding `--public-ip` support to that script too — add IP to SAN list alongside LAN IP)
  - **Extensibility seam**: `scripts/lib/tls.sh` is the stable CLI contract for all cert operations consumed by scripts. If the backend later moves from bash into a Go `CertificateManager` service (see post-T12 follow-on), `tls.sh` remains the script-facing interface — it would call the Go manager or read its outputs instead of wrapping `generate-certs.sh` directly. This preserves the current "wrap, don't duplicate" rule while creating a clean seam for a future unified cert manager.

  **Must NOT do**:
  - Do NOT duplicate cert generation logic that exists in `deploy/scripts/generate-certs.sh`
  - Do NOT duplicate Go cert generation logic from `bridge/pkg/setup/ssl.go`
  - Do NOT replace or rename `deploy/scripts/generate-certs.sh`

  **Recommended Agent Profile**:
  - **Category**: `quick`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 1 (with T1, T2, T3)
  - **Blocks**: T6
  - **Blocked By**: None

  **References**:
  - `deploy/scripts/generate-certs.sh` — Existing 288-line cert generator (WRAP THIS, do not duplicate)
  - `scripts/lib/contract.sh` — Pattern to follow for library structure (sourcing load_env.sh, helper functions)
  - `scripts/a1_deploy.sh` — Will consume this library (T6)
  - `deploy/scripts/generate-certs.sh:159` — SAN construction line where public IP must be added

  **Acceptance Criteria**:
  - [ ] `bash -n scripts/lib/tls.sh` passes
  - [ ] `tls_get_mode()` returns "none" when bridge.status reports native mode (Unix socket)
  - [ ] `tls_get_mode()` returns "private" when bridge.status reports sentinel + self-signed
  - [ ] `tls_get_mode()` returns "public" when bridge.status reports sentinel + CA
  - [ ] `tls_get_mode()` falls back to ARMORCLAW_TLS_MODE env var when bridge unreachable (**bootstrap-only fallback** — not a co-equal source of truth; bridge.status.tls.mode is always authoritative when available)
  - [ ] `tls_generate_certs()` invokes `deploy/scripts/generate-certs.sh` with correct args
  - [ ] `tls_get_fingerprint()` returns 64-char lowercase hex matching openssl output

  **QA Scenarios**:

  ```
  Scenario: TLS mode detection — all three derived modes
    Tool: Bash
    Preconditions: scripts/lib/tls.sh sourced
    Steps:
      1. Mock bridge.status returning tls.mode="none" (native); tls_get_mode
      2. Assert output is "none"
      3. Mock bridge.status returning tls.mode="private" (sentinel + self-signed); tls_get_mode
      4. Assert output is "private"
      5. Mock bridge.status returning tls.mode="public" (sentinel + CA); tls_get_mode
      6. Assert output is "public"
    Expected Result: Correct mode in each case
    Failure Indicators: Wrong mode, error exit
    Evidence: .sisyphus/evidence/tls/task4-mode-detection.txt

  Scenario: TLS mode fallback when bridge unreachable (bootstrap-only)
    Tool: Bash
    Preconditions: scripts/lib/tls.sh sourced, bridge unreachable (bootstrap scenario)
    Steps:
      1. With bridge unreachable; ARMORCLAW_TLS_MODE=private; tls_get_mode
      2. Assert fallback returns "private"
    Expected Result: ARMORCLAW_TLS_MODE env var fallback works when bridge.status is unavailable. This is a bootstrap-only mechanism — bridge.status.tls.mode is always the authoritative source when the bridge is running.
    Failure Indicators: Error exit or empty output
    Evidence: .sisyphus/evidence/tls/task4-mode-fallback.txt

  Scenario: Fingerprint from generated cert matches openssl
    Tool: Bash
    Preconditions: Cert generated at /tmp/test-certs/
    Steps:
      1. Run tls_generate_certs with --output /tmp/test-certs
      2. Run tls_get_fingerprint /tmp/test-certs
      3. Run: openssl x509 -in /tmp/test-certs/server.crt -fingerprint -sha256 -noout | cut -d= -f2 | tr -d ':' | tr 'A-F' 'a-f'
      4. Diff the two values
    Expected Result: Exact match
    Failure Indicators: Mismatch, wrong length
    Evidence: .sisyphus/evidence/tls/task4-fingerprint-match.txt
  ```

  **Commit**: YES
  - Message: `feat(scripts): add scripts/lib/tls.sh wrapper for cert generation`
  - Files: `scripts/lib/tls.sh`, `deploy/scripts/generate-certs.sh` (add --public-ip support)
  - Pre-commit: `bash -n scripts/lib/tls.sh && bash -n deploy/scripts/generate-certs.sh`

- [x] 5. Add TLS Metadata to bridge.status RPC and /discover Endpoint

  **What to do**:
  - **TLS termination reminder**: Caddy proxy terminates external TLS; bridge receives HTTP on localhost. TLS metadata here describes the externally-presented cert (managed by the proxy), not the bridge-internal cert.
  - In `bridge/pkg/rpc/server.go`, find the `bridge.status` RPC handler and add a `tls` section to the response:
    ```json
    {
      "tls": {
        "mode": "private",
        "health": "ok",
        "fingerprint_sha256": "a1b2c3d4...",
        "trust_type": "self_signed",
        "expires_at": 1714410000,
        "san_includes_public_ip": true,
        "cert_source": "shared_cert"
      }
    }
    ```
  - In `bridge/pkg/http/server.go`, update `handleDiscover()` (around line 491-525) to include the same TLS metadata object. **Note**: `/discover` and `/.well-known` TLS annotations apply only when the HTTP surface is actually exposed (sentinel/private + sentinel/public modes). In native mode (Unix socket), these HTTP endpoints are not externally reachable, so they need not include TLS metadata — but `bridge.status.tls` is always present via RPC.
  - The HTTP server needs access to its own cert metadata — use existing `GetCertificateFingerprint()` (fixed in T1) and add a new `GetCertExpiry() error (int64, error)` method that returns the cert's NotAfter as Unix timestamp
  - Derive TLS `mode` from deployment topology (NOT a 1:1 mapping from `server.mode`):
    - `server.mode == "native"` (Unix socket) → `tls.mode = "none"` (no external TLS surface)
    - `server.mode == "sentinel"` + self-signed cert (Issuer == Subject) → `tls.mode = "private"`
    - `server.mode == "sentinel"` + CA-issued cert (Issuer ≠ Subject) → `tls.mode = "public"`
    - Allow explicit override via `server.tls_mode` config field or `ARMORCLAW_TLS_MODE` env var. **Precedence**: env var overrides config file (env wins for testing/rollout; config is steady-state default).
  - Derive `trust_type`: self-signed cert → "self_signed", otherwise "public_ca"
  - Derive `san_includes_public_ip`: parse cert SANs, check if any non-loopback IP is present
  - **Mandatory in all modes**: `bridge.status.tls` MUST always be present. Scripts depend on this field — it must never be absent.
  - **Extensibility seam**: `bridge.status.tls` is the first stable sub-surface of a broader future `bridge.status` system contract (see post-T12 follow-on). The `tls` section establishes the pattern: mode-aware, always-present, with `cert_source` truth-level tracking. Future sections (`license`, `agents`, `runtime`, `sidecars`) should follow the same conventions. Do NOT create competing status endpoints — extend this response with additional stable sections.
  - **Private mode** (sentinel + self-signed): all fields populated — `mode="private"`, `fingerprint_sha256` (64 hex), `trust_type="self_signed"`, `expires_at` (Unix timestamp), `san_includes_public_ip` (bool).
  - **Public mode** (sentinel + CA): all fields populated — `mode="public"`, `fingerprint_sha256` (64 hex), `trust_type="public_ca"`, `expires_at` (Unix timestamp), `san_includes_public_ip` (bool, may be false — domain SAN is sufficient for CA).
  - **Native mode** (Unix socket, no external TLS surface): `mode="none"`, `fingerprint_sha256=""`, `trust_type=""`, `expires_at=0`, `san_includes_public_ip=false`. This explicitly signals "no TLS surface to trust" — clients skip trust flows entirely. These fields are **not null** — they use zero-value semantics so scripts can do simple string/numeric checks without null handling.
  - **Shared-cert runtime guard (hardened)**: The shared-cert model depends on Bridge and Caddy reading the same cert files. Add a startup check: Bridge reads its cert fingerprint at init and logs a warning if the cert file does not exist at the expected path (`/etc/armorclaw/certs/server.crt`). **Degraded-state contract**: If the shared cert file is missing (`cert_source != "shared_cert"`):
    - `mode` stays at its derived value (`"private"`, `"public"`, or `"none"`) — **mode is a stable 3-value enum and never changes due to health**
    - `health` MUST be `"degraded"` — explicitly signals that TLS metadata is not trustworthy
    - `fingerprint_sha256` MUST be blank (`""`) — never report a fingerprint from a cert the proxy may not be using
    - `trust_type` MUST be `""`
    - `expires_at` MUST be `0`
    - `cert_source` MUST be `"proxy_only"` (indicates Bridge cannot validate what cert the proxy presents)
    - This keeps `mode` stable for scripts that only check topology (private/public/none) while adding `health` for scripts that need to verify trustworthiness. Clients check `health == "ok"` before trusting the fingerprint; `health == "degraded"` means treat TLS metadata as untrusted.
    - In private mode with the default topology (Bridge + Caddy on same VPS, shared certs), a missing cert file is a deployment error. The test harness (`tests/test-tls-mode-integration.sh`) SHOULD FAIL if `bridge.status.tls.health != "ok"` in private mode — this is not a soft warning, it is a correctness check.
  - **Allowed `health` values**:
    - `"ok"` — TLS metadata is trustworthy. `cert_source == "shared_cert"`, fingerprint is valid, cert file present.
    - `"degraded"` — TLS metadata is untrusted. `cert_source == "proxy_only"`, fingerprint is blank, cert file missing or unreadable. Recovery: fix deployment + restart bridge.
  - **Client/Script Behavior on `health == "degraded"`**: Treat TLS metadata as untrusted. Fall back to manual QR scan or manual entry flow. Do not auto-trust the fingerprint. Log a warning. Scripts should retry `bridge.status` after 60 seconds or on next provisioning attempt. In degraded state, the fingerprint is intentionally blank — the only recovery path is fixing the deployment (ensuring cert files exist at `/etc/armorclaw/certs/`) and restarting the bridge.

  **Must NOT do**:
  - Do NOT create separate `/trust/*` endpoints (per design decision)
  - Do NOT change existing fields in bridge.status response
  - Do NOT omit the TLS section in any mode — it is mandatory

  **Recommended Agent Profile**:
  - **Category**: `unspecified-high`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES (with T6, T7)
  - **Parallel Group**: Wave 2
  - **Blocks**: T8, T9, T10
  - **Blocked By**: T1 (fingerprint fix)

  **References**:
  - `bridge/pkg/rpc/server.go` — `registerHandlers()` function, find `bridge.status` handler
  - `bridge/pkg/http/server.go:491-525` — `handleDiscover()` current response structure
  - `bridge/pkg/http/server.go:250-263` — `GetCertificateFingerprint()` (updated in T1)
  - `bridge/pkg/config/config.go:146-155` — Server config: "native" (Unix socket) vs "sentinel" (TCP). TLS mode derived from topology, not from server.mode directly.
  - `bridge/pkg/http/server.go:243-248` — `GetCertificatePEM()` (pattern for new GetCertExpiry)

  **Acceptance Criteria**:
  - [ ] `bridge.status` RPC response includes `tls` object in all modes
  - [ ] Private/public modes: `tls` has populated fingerprint_sha256, trust_type, expires_at
  - [ ] Native mode: `tls` has mode="none", fingerprint_sha256="", trust_type="", expires_at=0 (zero-value, not null)
  - [ ] `/discover` endpoint includes identical `tls` object (only when HTTP surface is exposed — see Fix #2)
  - [ ] Fingerprint in bridge.status matches `/fingerprint` endpoint (private/public modes only; native mode has no `/fingerprint` endpoint)
  - [ ] `cd bridge && go test ./pkg/rpc/... ./pkg/http/...` → PASS

  **QA Scenarios**:

  ```
  Scenario: bridge.status includes TLS metadata (private/public mode)
    Tool: Bash (curl)
    Preconditions: Bridge running with HTTPS (sentinel mode)
    Steps:
      1. curl -skf https://$VPS_IP:$BRIDGE_PORT/api -d '{"jsonrpc":"2.0","id":1,"method":"bridge.status","params":{}}' | jq '.result.tls'
      2. Assert .mode is "private" or "public"
      3. Assert .fingerprint_sha256 is 64-char lowercase hex
      4. Assert .trust_type is "self_signed" or "public_ca"
      5. Assert .expires_at is a positive Unix timestamp > now
    Expected Result: All TLS fields present with valid values for sentinel modes
    Failure Indicators: Missing tls object, invalid values
    Evidence: .sisyphus/evidence/tls/task5-bridge-status-sentinel.json

  Scenario: bridge.status includes TLS metadata (native mode — zero values)
    Tool: Bash (go test)
    Preconditions: Go test environment, config with server.mode="native"
    Steps:
      1. Call bridge.status RPC with native-mode config
      2. Assert .mode == "none"
      3. Assert .fingerprint_sha256 == "" (empty string, not null)
      4. Assert .trust_type == "" (empty string, not null)
      5. Assert .expires_at == 0 (zero, not null)
    Expected Result: TLS object present with zero-value fields for native mode
    Failure Indicators: Missing tls object, null fields instead of zero-values, mode ≠ "none"
    Evidence: .sisyphus/evidence/tls/task5-bridge-status-native.txt

  Scenario: /discover fingerprint matches /fingerprint endpoint
    Tool: Bash (curl)
    Preconditions: Bridge running with HTTPS
    Steps:
      1. FINGERPRINT_DISCOVER=$(curl -skf https://$VPS_IP:$BRIDGE_PORT/discover | jq -r '.tls.fingerprint_sha256')
      2. FINGERPRINT_EP=$(curl -skf https://$VPS_IP:$BRIDGE_PORT/fingerprint | jq -r '.sha256')
      3. diff <(echo $FINGERPRINT_DISCOVER) <(echo $FINGERPRINT_EP)
    Expected Result: Empty diff (identical fingerprints)
    Failure Indicators: Any difference between the two values
    Evidence: .sisyphus/evidence/tls/task5-fingerprint-consistency.txt

  Scenario: TLS mode derivation — sentinel + self-signed = private (live VPS)
    Tool: Bash (curl + openssl)
    Preconditions: Bridge running on VPS in sentinel mode with self-signed cert
    Steps:
      1. TLS_MODE=$(curl -skf https://$VPS_IP:$BRIDGE_PORT/api -d '{"jsonrpc":"2.0","id":1,"method":"bridge.status","params":{}}' | jq -r '.result.tls.mode')
      2. assert "$TLS_MODE" = "private"
      3. ISSUER=$(openssl s_client -connect $VPS_IP:$BRIDGE_PORT </dev/null 2>/dev/null | openssl x509 -noout -issuer)
      4. SUBJECT=$(openssl s_client -connect $VPS_IP:$BRIDGE_PORT </dev/null 2>/dev/null | openssl x509 -noout -subject)
      5. assert "$ISSUER" = "$SUBJECT" (self-signed check)
    Expected Result: mode="private", trust_type="self_signed", cert Issuer == Subject
    Failure Indicators: mode is not "private", or Issuer ≠ Subject when mode claims private
    Evidence: .sisyphus/evidence/tls/task5-tls-mode-derivation-private.txt

  Scenario: TLS mode derivation — native = none (unit test)
    Tool: Bash (go test)
    Preconditions: Go test environment
    Steps:
      1. Create config with server.mode="native"
      2. Call deriveTLSMode(config, cert)
      3. Assert result.mode == "none"
      4. Assert result.fingerprint_sha256 == "" (no external TLS surface)
      5. Assert result.trust_type == "" (not null — zero-value)
      6. Assert result.expires_at == 0 (not null — zero-value)
    Expected Result: Native mode reports tls.mode="none" with zero-value fields (not null)
    Failure Indicators: mode is "private" or "public", or any field is null instead of zero-value
    Evidence: .sisyphus/evidence/tls/task5-tls-mode-derivation-native.txt

  Scenario: TLS mode derivation — sentinel + CA = public (unit test only, not E2E-verified in this phase)
    Tool: Bash (go test)
    Preconditions: Go test environment
    Steps:
      1. Create config with server.mode="sentinel"
      2. Load a CA-issued test cert (Issuer ≠ Subject)
      3. Call deriveTLSMode(config, cert)
      4. Assert result.mode == "public"
      5. Assert result.trust_type == "public_ca"
    Expected Result: Sentinel + CA cert reports mode="public", trust_type="public_ca"
    Failure Indicators: mode is "private" for CA-issued cert, or Issuer == Subject for CA cert
    Evidence: .sisyphus/evidence/tls/task5-tls-mode-derivation-public.txt

  Scenario: cert_source reflects shared-cert status — private mode must be "shared_cert" with health "ok"
    Tool: Bash (curl + go test)
    Preconditions: Bridge running in private mode (sentinel + self-signed)
    Steps:
      1. STATUS=$(curl -skf https://$VPS_IP:$BRIDGE_PORT/api -d '{"jsonrpc":"2.0","id":1,"method":"bridge.status","params":{}}')
      2. CERT_SOURCE=$(echo $STATUS | jq -r '.result.tls.cert_source')
      3. Assert CERT_SOURCE == "shared_cert" (cert file found at /etc/armorclaw/certs/server.crt)
      4. HEALTH=$(echo $STATUS | jq -r '.result.tls.health')
      5. Assert HEALTH == "ok"
      6. Assert fingerprint_sha256 is non-empty and 64 hex chars
    Expected Result: cert_source is "shared_cert" with health "ok" and valid fingerprint in private mode
    Failure Indicators: cert_source is "proxy_only" (missing cert file — deployment error), health is "degraded", fingerprint blank when health is "ok"
    Evidence: .sisyphus/evidence/tls/task5-cert-source-private.txt

  Scenario: Degraded health — missing cert blanks fingerprint
    Tool: Bash (go test)
    Preconditions: Unit test environment
    Steps:
      1. Create config with sentinel mode but cert file path pointing to nonexistent file
      2. Call deriveTLSStatus(config, certLoader)
      3. Assert result.cert_source == "proxy_only"
      4. Assert result.fingerprint_sha256 == ""
      5. Assert result.mode == "private" (mode is stable — not "degraded")
      6. Assert result.health == "degraded" (health reports the problem)
      7. Assert result.trust_type == ""
      8. Assert result.expires_at == 0
    Expected Result: Missing cert → health="degraded" with blank fingerprint (not false source of truth). Mode stays "private" (stable topology).
    Failure Indicators: Fingerprint populated despite missing cert, mode changes to "degraded" (mode must stay stable), health is "ok" when cert is missing
    Evidence: .sisyphus/evidence/tls/task5-cert-source-degraded.txt
  ```

  **Commit**: YES
  - Message: `feat(bridge): add TLS metadata to bridge.status RPC and /discover endpoint`
  - Files: `bridge/pkg/rpc/server.go`, `bridge/pkg/http/server.go`
  - Pre-commit: `cd bridge && go test ./pkg/rpc/... ./pkg/http/...`

- [x] 6. Update Plan A Discovery + Deploy Scripts for TLS Awareness

  **What to do**:
  - Update `scripts/a0_discover.sh`:
    - Add TLS metadata to the discovery loop: probe `/fingerprint` endpoint, record in manifest under `live_discovered.tls`
    - Add fields: `tls_mode`, `external_scheme` (always "https"), `cert_fingerprint_sha256`, `cert_expires_at`, `san_includes_public_ip`
    - All external probes must use HTTPS (already partially done, verify consistency)
  - Update `scripts/a1_deploy.sh`:
    - Source `scripts/lib/tls.sh`
    - After deploy, call `tls_generate_certs()` with the VPS public IP as `--public-ip`
    - Record TLS metadata in deploy evidence (`a1_tls_metadata.json`)
    - Ensure external health checks use HTTPS

  **Must NOT do**:
  - Do NOT change localhost-over-SSH probes to HTTPS (they stay HTTP)
  - Do NOT remove existing discovery fields
  - Do NOT break the `deployment_required=true` flow

  **Recommended Agent Profile**:
  - **Category**: `unspecified-high`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES (with T5, T7)
  - **Parallel Group**: Wave 2
  - **Blocks**: T9, T10
  - **Blocked By**: T1 (fingerprint fix), T4 (tls.sh wrapper)

  **References**:
  - `scripts/a0_discover.sh` — Existing discovery script, add TLS probing after HTTP endpoint probing
  - `scripts/a1_deploy.sh` — Existing deploy script, add cert generation with public IP
  - `scripts/lib/tls.sh` — Wrapper created in T4, consumed here
  - `scripts/lib/contract.sh` — `_contract_bridge_rpc()`, `_contract_save()` helpers
  - `doc/testing.md` — Documents the E2E pipeline flow (a0→a1→a0→a2→a3→a4)

  **Acceptance Criteria**:
  - [ ] `contract_manifest.json` includes `live_discovered.tls` section with mode, fingerprint, expiry
  - [ ] Deploy evidence includes `a1_tls_metadata.json`
  - [ ] External health checks in a1 use `https://`
  - [ ] `bash -n scripts/a0_discover.sh && bash -n scripts/a1_deploy.sh` pass

  **QA Scenarios**:

  ```
  Scenario: A0 discovery records TLS metadata
    Tool: Bash
    Preconditions: Bridge running with HTTPS on VPS
    Steps:
      1. bash scripts/a0_discover.sh
      2. jq '.live_discovered.tls' .sisyphus/evidence/armorclaw/contract_manifest.json
      3. Assert .tls_mode is non-empty
      4. Assert .cert_fingerprint_sha256 is 64 chars
      5. Assert .cert_expires_at > current timestamp
    Expected Result: TLS section present in manifest
    Failure Indicators: No tls section, empty fields
    Evidence: .sisyphus/evidence/tls/task6-a0-tls-manifest.json

  Scenario: A1 deploy generates cert with public IP
    Tool: Bash
    Preconditions: VPS accessible, Docker available
    Steps:
      1. bash scripts/a1_deploy.sh (or relevant phase)
      2. Check .sisyphus/evidence/armorclaw/a1_tls_metadata.json
      3. SSH to VPS and inspect cert: openssl x509 -in /etc/armorclaw/certs/server.crt -text -noout | grep -A2 "Subject Alternative Name"
    Expected Result: SAN includes VPS public IP
    Failure Indicators: SAN missing public IP, cert generation failed
    Evidence: .sisyphus/evidence/tls/task6-cert-san.txt
  ```

  **Commit**: YES
  - Message: `feat(scripts): make Plan A discovery TLS-aware`
  - Files: `scripts/a0_discover.sh`, `scripts/a1_deploy.sh`
  - Pre-commit: `bash -n scripts/a0_discover.sh && bash -n scripts/a1_deploy.sh`

- [x] 7. Update Plan A Provisioning Script for TLS Outputs

  **What to do**:
  - Update `scripts/a2_provision.sh`:
    - After provisioning claim, capture TLS metadata from **`bridge.status`** (authoritative always-on source) into provisioning outputs
    - Add TLS fields to `a2_provisioning_outputs.json`: `tls_mode`, `tls_fingerprint_sha256`, `cert_expires_at`, `tls_trust_hint` — sourced from `bridge.status.tls`, not from `/qr/config`
    - **QR config TLS check**: QR v2 contract (only when `ARMORCLAW_QR_VERSION=2`): if set, also verify `/qr/config` TLS fields match `bridge.status.tls`. If not set (v1 default), verify `/qr/config` has no TLS fields (version=1) and skip QR TLS assertions.
    - Make `/.well-known` verification conditional: SKIP with documented reason in private mode when well-known returns non-200 or cert is self-signed
    - Verify that the `bridge.status.tls` fingerprint matches the `/fingerprint` endpoint value
    - Record all TLS provisioning evidence
    - **Checkpoint evidence** (provisioning resilience seam): In addition to the existing evidence, write three "last known good" checkpoint files to `.sisyphus/checkpoints/tls/`:
      - `last_good_qr_config.json` — most recent `/qr/config` response (full payload)
      - `last_good_bridge_status.json` — most recent `bridge.status` response
      - `last_good_provisioning_outputs.json` — most recent provisioning output JSON
      These checkpoints are written on every successful provisioning run. They are NOT consumed by this plan's tasks — they exist as a direct on-ramp for a future provisioning resilience layer (post-T12 follow-on) that implements resume-from-last-known-good logic. The checkpoint file names and locations are stable; the resilience layer reads them. Checkpoints are stored separately from test evidence (`.sisyphus/evidence/tls/`) to keep operational state distinct from verification artifacts.
    - **Checkpoint security hygiene**: Checkpoint files may contain QR config responses (including signed payloads) and bridge status. Apply the following rules:
      - **gitignore**: `.sisyphus/checkpoints/` MUST be listed in `.gitignore` — checkpoint files are operational state, never committed
      - **Permissions**: `chmod 600` on all checkpoint files — owner-readable only, since they contain signed QR payloads
      - **Overwrite semantics**: Overwrite on success (last-write-wins). Each successful provisioning run replaces all three checkpoint files atomically. Failed runs do NOT update checkpoints (they preserve the last-known-good state)
      - **Redaction**: If checkpoint files need to be exported for debugging, strip `signature` fields from QR config payloads before export. The provisioning script should include a `--redact-checkpoints` flag for this purpose
      - **Cleanup**: Checkpoint files are removed after a fully successful, complete provisioning run (clean state on completion). They only persist across partial failures or interrupted runs, where they enable resume. This prevents accumulation of stale checkpoints over many successful runs.

  **Must NOT do**:
  - Do NOT fail provisioning if /.well-known is unavailable in private mode
  - Do NOT change the Matrix session SKIP path logic (that's existing behavior)
  - Do NOT remove existing provisioning output fields

  **Recommended Agent Profile**:
  - **Category**: `unspecified-high`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES (with T5, T6, T8)
  - **Parallel Group**: Wave 2
  - **Blocks**: T9, T10
  - **Blocked By**: T3 (QR payload TLS fields)

  **References**:
  - `scripts/a2_provision.sh` — Existing provisioning script
  - `scripts/lib/contract.sh` — `_contract_bridge_rpc()`, `_contract_save()`, `_contract_update_manifest()`
  - `bridge/pkg/qr/public.go:264-274` — ConfigPayload struct (updated in T3 with TLS fields)

  **Acceptance Criteria**:
  - [ ] `a2_provisioning_outputs.json` includes `tls_mode`, `tls_fingerprint_sha256`, `cert_expires_at` (sourced from `bridge.status.tls`, not `/qr/config`)
  - [ ] In private mode, /.well-known failure is SKIP with reason (not FAIL)
  - [ ] Fingerprint in provisioning outputs matches /fingerprint endpoint
  - [ ] Default path (v1 QR): provisioning succeeds with TLS metadata from bridge.status only, no QR TLS-field assertions
  - [ ] Flagged path — QR v2 contract (only when `ARMORCLAW_QR_VERSION=2`): provisioning additionally verifies `/qr/config` TLS fields match `bridge.status.tls`
  - [ ] `bash -n scripts/a2_provision.sh` passes

  **QA Scenarios**:

  ```
  Scenario: Provisioning outputs include TLS metadata from bridge.status (default v1 QR)
    Tool: Bash
    Preconditions: Bridge running, A0 and A1 complete, ARMORCLAW_QR_VERSION not set
    Steps:
      1. bash scripts/a2_provision.sh
      2. jq '.' .sisyphus/evidence/armorclaw/a2_provisioning_outputs.json
      3. Assert .tls_mode is non-empty
      4. Assert .tls_fingerprint_sha256 is 64 chars
      5. Assert .cert_expires_at > now
      6. curl -skf https://$VPS_IP:$BRIDGE_PORT/qr/config | jq '.config'
      7. Assert .version == 1 (no TLS fields in QR)
    Expected Result: TLS fields present from bridge.status, QR is v1 (no TLS fields)
    Failure Indicators: Missing TLS fields in provisioning outputs, QR emits v2 without flag
    Evidence: .sisyphus/evidence/tls/task7-provisioning-tls-v1-default.json

  Scenario: Provisioning verifies QR v2 contract (only when `ARMORCLAW_QR_VERSION=2`)
    Tool: Bash
    Preconditions: Bridge running, ARMORCLAW_QR_VERSION=2 set
    Steps:
      1. bash scripts/a2_provision.sh
      2. FP_STATUS=$(jq -r '.tls_fingerprint_sha256' .sisyphus/evidence/armorclaw/a2_provisioning_outputs.json)
      3. FP_QR=$(curl -skf https://$VPS_IP:$BRIDGE_PORT/qr/config | jq -r '.config.tls_fingerprint_sha256')
      4. Assert "$FP_STATUS" == "$FP_QR"
      5. curl -skf https://$VPS_IP:$BRIDGE_PORT/qr/config | jq '.config.version'
      6. Assert version == 2
    Expected Result: v2 QR TLS fields match bridge.status.tls
    Failure Indicators: Fingerprint mismatch between bridge.status and QR, version != 2
    Evidence: .sisyphus/evidence/tls/task7-provisioning-tls-v2-flagged.json

  Scenario: Well-known SKIP in private mode
    Tool: Bash
    Preconditions: Bridge running in private mode (self-signed cert, no domain)
    Steps:
      1. bash scripts/a2_provision.sh
      2. Check output for "SKIP" on /.well-known step
      3. Verify exit code is 0 (not failure)
    Expected Result: /.well-known step SKIPs with documented reason, provisioning succeeds
    Failure Indicators: /.well-known causes provisioning failure
    Evidence: .sisyphus/evidence/tls/task7-wellknown-skip.txt
  ```

  **Commit**: YES
  - Message: `feat(scripts): capture TLS fields in Plan A provisioning outputs`
  - Files: `scripts/a2_provision.sh`
  - Pre-commit: `bash -n scripts/a2_provision.sh`

- [x] 8. Annotate /.well-known with TLS Mode Metadata

  **What to do**:
  - Always serve `/.well-known/matrix/client` when the HTTP surface is exposed (sentinel mode) — never return 404. In native mode (Unix socket), this endpoint is not externally reachable and need not include TLS annotations.
  - In `bridge/pkg/http/server.go`, update `handleWellKnown()` (lines 557-591):
    - Add `tls_mode` field **inside the existing `com.armorclaw` namespace** object alongside the existing fields (`base_url`, `api_endpoint`, `ws_endpoint`, `push_gateway`). This follows Matrix well-known vendor namespace conventions and avoids top-level field pollution.
  - **Exact JSON contract** (after this change):
  ```json
  {
    "m.homeserver": { "base_url": "https://matrix.armorclaw.app" },
    "m.identity_server": { "base_url": "https://matrix.armorclaw.app" },
    "com.armorclaw": {
      "base_url": "https://<bridge-host>:<port>",
      "api_endpoint": "https://<bridge-host>:<port>/api",
      "ws_endpoint": "wss://<bridge-host>:<port>/ws",
      "push_gateway": "https://<bridge-host>:<port>/_matrix/push/v1/notify",
      "tls_mode": "private"
    }
  }
  ```
    - Value is derived from TLS mode (set by T5): `"none"` for native (Unix socket), `"private"` for sentinel + self-signed, `"public"` for sentinel + CA-issued
    - This tells clients the TLS trust model — whether trust confirmation is needed (private) or handled by a CA (public)
    - For native mode (Unix socket): no external /.well-known behavior needed in this plan. Native-mode trust is handled entirely by `bridge.status.tls` (mode="none"). The handler code may set `tls_mode: "none"` for internal consistency, but this is not a separate test target.
  - **Documentation reconciliation**: Verify that project URL reference docs and `/doc/` markdown files describe `/.well-known/matrix/client` as a Bridge-served endpoint on the same host as `/api`, `/discover`, `/qr/config` — NOT as a separate root-host service at a different domain. If the docs describe a two-host model (e.g., `armorclaw.app` for well-known vs `bridge.armorclaw.app` for APIs), add a clarification note explaining that in the default self-hosted topology, both resolve to the same VPS with Caddy routing by path. This prevents operator confusion from encountering two host models.

  > **Note — Host ownership & routing**: `/.well-known/matrix/client` is served by the Bridge handler behind Caddy, on the same host as `/qr/config`, `/discover`, `/api`. In self-hosted/private-mode, Caddy routes by path (not hostname) — `/_matrix/*` to Matrix, everything else to Bridge. For multi-host deployments (e.g., `armorclaw.app` root vs `bridge.armorclaw.app`), root host Caddy would need a proxy pass for `/.well-known` to the bridge host — out of scope for this plan's single-host topology. **The broader URL reference documentation must be updated** to clarify that `/.well-known/matrix/client` is a Bridge-served endpoint (not a separate root-host service) so operators do not encounter two conflicting host models.

  > **Note — Discovery extensibility**: `/.well-known` is one discovery surface. Future mDNS may publish equivalent TLS metadata via `_armorclaw._tcp` TXT records — a declared extension (T15), not a replacement.
  - **TLS termination reminder**: Caddy proxy terminates external TLS; bridge receives HTTP on localhost. This handler is reached via proxy.

  **Must NOT do**:
  - Do NOT return 404 for /.well-known in private mode (breaks clients)
  - Do NOT remove existing well-known fields
  - Do NOT add the CA cert fingerprint to well-known (that goes in QR only)
  - Do NOT add `tls_mode` as a new top-level field — place it under `com.armorclaw`

  **Recommended Agent Profile**:
  - **Category**: `quick`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: NO (depends on T5 TLS mode detection)
  - **Parallel Group**: Wave 2 (sequential within wave, after T5)
  - **Blocks**: T9
  - **Blocked By**: T5

  **References**:
  - `bridge/pkg/http/server.go:557-591` — `handleWellKnown()` current implementation
  - `bridge/pkg/config/config.go:146-155` — Server mode detection

  **Acceptance Criteria**:
  - [ ] `/.well-known/matrix/client` response includes `"com.armorclaw": { "tls_mode": "private" }` when in private mode (sentinel + self-signed, externally reachable)
  - [ ] `/.well-known/matrix/client` response includes `"com.armorclaw": { "tls_mode": "public" }` when in public mode (sentinel + CA) — **unit test only in this phase, not E2E-verified**
  - [ ] Native mode (Unix socket): not tested via /.well-known in this plan. Native-mode trust semantics are confined to `bridge.status.tls` (mode="none", zero-value fields). The handler code sets `tls_mode: "none"` internally for consistency, but this is verified by the bridge.status unit test in T5, not by a separate well-known test.
  - [ ] Existing fields (`m.homeserver`) unchanged
  - [ ] `cd bridge && go test ./pkg/http/...` → PASS

  **QA Scenarios**:

  ```
  Scenario: Well-known includes com.armorclaw.tls_mode in private mode
    Tool: Bash (curl)
    Preconditions: Bridge running in private mode
    Steps:
      1. curl -skf https://$VPS_IP:$BRIDGE_PORT/.well-known/matrix/client | jq .
      2. Assert .com.armorclaw.tls_mode == "private"
      3. Assert .m.homeserver.base_url is non-empty
    Expected Result: com.armorclaw.tls_mode field present and correct
    Failure Indicators: Missing tls_mode, wrong value, tls_mode at top level
    Evidence: .sisyphus/evidence/tls/task8-wellknown-private.json
  ```

  **Commit**: YES
  - Message: `feat(bridge): annotate /.well-known with TLS mode metadata`
  - Files: `bridge/pkg/http/server.go`
  - Pre-commit: `cd bridge && go test ./pkg/http/...`

- [x] 9. Cert & Provisioning Preservation Test (Restart Safety)

  **What to do**:
  - Create `tests/test-tls-restart-safety.sh`:
    - Captures pre-restart state: `/qr/config` full response, `bridge.status` TLS section, `/fingerprint` value, cert SAN from openssl
    - Saves to `.sisyphus/evidence/tls/restart-pre-state.json`
    - Restarts the bridge via `restart_bridge()` (with flock serialization)
    - Captures post-restart state from same endpoints
    - Saves to `.sisyphus/evidence/tls/restart-post-state.json`
    - Diffs the two states:
      - Fingerprint must be identical
      - Cert SANs must be identical
      - /qr/config signature must still validate (same signing key)
      - bridge.status TLS mode must be unchanged
      - Provisioning outputs (homeserver_url, rpc_url) must be unchanged
    - FAIL if any diff found
    - **Checkpoint evidence** (provisioning resilience seam): On PASS, write post-restart `bridge.status` and `/qr/config` to the same `last_good_bridge_status.json` and `last_good_qr_config.json` checkpoint files in `.sisyphus/checkpoints/tls/` (defined in T7). This ensures checkpoints are fresh after restart verification.
  - **What this proves**: Certs and provisioning state survive a bridge restart (e.g., config reload, container restart). This is NOT a Docker image upgrade test — that requires pulling a new image and is out of scope for this plan.
  - Follow Tier A test pattern: source lib/load_env.sh, lib/common_output.sh, use harness_summary

  **Must NOT do**:
  - Do NOT pull a new Docker image (that's upgrade testing, not restart testing)
  - Do NOT test with ROTATE_TLS_CERT=1 (that's a separate scenario)
  - Do NOT modify any deployment during this test

  **Recommended Agent Profile**:
  - **Category**: `deep`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES (with T10, T11)
  - **Parallel Group**: Wave 3
  - **Blocks**: T12, F1-F4
  - **Blocked By**: T5 (bridge.status TLS), T6 (a0/a1 TLS), T7 (a2 TLS)

  **References**:
  - `tests/test-system-health-baseline.sh` — Tier A test pattern to follow
  - `tests/lib/load_env.sh` — ssh_vps(), env vars
  - `tests/lib/common_output.sh` — log_pass/log_fail/harness_summary
  - `tests/lib/restart_bridge.sh` — restart_bridge() with flock serialization

  **Acceptance Criteria**:
  - [ ] Test script syntax-valid (`bash -n tests/test-tls-restart-safety.sh`)
  - [ ] Test captures pre/post state to evidence files
  - [ ] Test FAILS if fingerprint changes across restart
  - [ ] Test PASSES when cert is preserved across restart

  **QA Scenarios**:

  ```
  Scenario: Cert preserved on restart
    Tool: Bash
    Preconditions: Bridge running on VPS
    Steps:
      1. Capture pre-state: curl -skf https://$VPS_IP:$BRIDGE_PORT/fingerprint > /tmp/pre-fp.json
      2. Restart bridge via restart_bridge()
      3. Capture post-state: curl -skf https://$VPS_IP:$BRIDGE_PORT/fingerprint > /tmp/post-fp.json
      4. diff <(jq -r '.sha256' /tmp/pre-fp.json) <(jq -r '.sha256' /tmp/post-fp.json)
    Expected Result: Empty diff (fingerprint unchanged)
    Failure Indicators: Fingerprint changed
    Evidence: .sisyphus/evidence/tls/task9-restart-safety.json

  Scenario: QR config signature still validates after restart
    Tool: Bash
    Preconditions: Bridge running, pre-state captured
    Steps:
      1. Capture pre-restart /qr/config → .config fields
      2. Restart bridge
      3. Capture post-restart /qr/config → .config fields
      4. Diff .matrix_homeserver, .rpc_url, .tls_fingerprint_sha256
    Expected Result: All fields identical
    Failure Indicators: Any field changed, signature invalid
    Evidence: .sisyphus/evidence/tls/task9-qr-preservation.json
  ```

  **Commit**: YES
  - Message: `test(tls): add restart-safety harness test`
  - Files: `tests/test-tls-restart-safety.sh`
  - Pre-commit: `bash -n tests/test-tls-restart-safety.sh`

- [x] 10. TLS-Mode Integration Test

  **What to do**:
  - Create `tests/test-tls-mode-integration.sh`:
    - Comprehensive integration test covering the full TLS surface.
    - **Primary TLS contract** (always-on, tested on every run):
      1. Verify HTTPS health endpoint returns valid JSON over TLS
      2. Verify cert SAN includes public IP (openssl inspection)
      3. Verify /fingerprint returns standard SHA-256 matching openssl output
      4. Verify bridge.status includes tls object matching /fingerprint — **authoritative always-on TLS source**
      5. Verify /discover includes tls object matching bridge.status.tls
      6. Verify /.well-known includes tls_mode field
      7. Verify all external endpoints are HTTPS-only (HTTP redirects or refuses)
      8. Verify localhost-over-SSH still works with HTTP
    - **QR TLS contract** (v1 default path — always tested):
      9. Verify /qr/config emits v1 by default (version=1, no TLS fields)
    - **QR TLS contract** (conditional on `ARMORCLAW_QR_VERSION=2`):
      10. Verify /qr/config v2 TLS fields match bridge.status.tls; signature validates
    - Save all evidence to `.sisyphus/evidence/tls/integration/`
  - Follow Tier A test pattern with PASS/FAIL/SKIP counters
  - **Rollout gate enforcement**: The test script MUST distinguish between v1-default and v2 paths. QR v2 contract (only when `ARMORCLAW_QR_VERSION=2`) runs as a separate sub-scenario. The test must PASS in both configurations (v1 and v2).

  **Must NOT do**:
  - Do NOT test ArmorChat client behavior
  - Do NOT test cert rotation (covered by T9 restart-safety)
  - Do NOT test public CA mode (requires real domain + Let's Encrypt)

  **Recommended Agent Profile**:
  - **Category**: `unspecified-high`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES (with T9, T11)
  - **Parallel Group**: Wave 3
  - **Blocks**: T12, F1-F4
  - **Blocked By**: T5, T6, T7, T8

  **References**:
  - `tests/test-system-health-baseline.sh` — Pattern for Tier A test (7 scenarios, harness_summary)
  - `tests/lib/assert_json.sh` — assert_json_has_key(), assert_json_equals()
  - `tests/lib/load_env.sh` — ssh_vps(), VPS_IP, BRIDGE_PORT

  **Acceptance Criteria**:
  - [ ] 9 always-on scenarios (bridge.status + /discover + /fingerprint + /well-known + SAN + HTTPS + SSH), all passing
  - [ ] 1 conditional v2 scenario — QR v2 contract (only when `ARMORCLAW_QR_VERSION=2`): runs when flag is set, SKIPS otherwise
  - [ ] Default v1 path: test PASSES with bridge.status as sole TLS source, QR is v1 (no TLS fields)
  - [ ] Evidence saved for each scenario
  - [ ] `bash -n tests/test-tls-mode-integration.sh` passes

  **QA Scenarios**:

  ```
  Scenario: Full TLS integration — all always-on surfaces consistent
    Tool: Bash
    Preconditions: Bridge running with HTTPS on VPS
    Steps:
      1. FINGERPRINT_EP=$(curl -skf https://$VPS_IP:$BRIDGE_PORT/fingerprint | jq -r '.sha256')
      2. FINGERPRINT_STATUS=$(curl -skf https://$VPS_IP:$BRIDGE_PORT/api -d '{"jsonrpc":"2.0","id":1,"method":"bridge.status","params":{}}' | jq -r '.result.tls.fingerprint_sha256')
      3. FINGERPRINT_OPENSSL=$(openssl s_client -connect $VPS_IP:$BRIDGE_PORT </dev/null 2>/dev/null | openssl x509 -fingerprint -sha256 -noout | cut -d= -f2 | tr -d ':' | tr 'A-F' 'a-f')
      4. Assert all 3 values are identical
      5. QR_VERSION=$(curl -skf https://$VPS_IP:$BRIDGE_PORT/qr/config | jq -r '.config.version')
      6. Assert QR_VERSION == 1 (v1 default, no TLS fields)
    Expected Result: Fingerprint consistent across /fingerprint, bridge.status, and openssl. QR emits v1 by default.
    Failure Indicators: Any mismatch between the 3 sources, QR version != 1
    Evidence: .sisyphus/evidence/tls/task10-fingerprint-consistency.txt

  Scenario: QR v2 TLS contract — fields match bridge.status (conditional on `ARMORCLAW_QR_VERSION=2`)
    Tool: Bash
    Preconditions: Bridge running with ARMORCLAW_QR_VERSION=2 set
    Steps:
      1. FINGERPRINT_STATUS=$(curl -skf https://$VPS_IP:$BRIDGE_PORT/api -d '{"jsonrpc":"2.0","id":1,"method":"bridge.status","params":{}}' | jq -r '.result.tls.fingerprint_sha256')
      2. FINGERPRINT_QR=$(curl -skf https://$VPS_IP:$BRIDGE_PORT/qr/config | jq -r '.config.tls_fingerprint_sha256')
      3. Assert "$FINGERPRINT_STATUS" == "$FINGERPRINT_QR"
      4. QR_TLS_MODE=$(curl -skf https://$VPS_IP:$BRIDGE_PORT/qr/config | jq -r '.config.tls_mode')
      5. STATUS_TLS_MODE=$(curl -skf https://$VPS_IP:$BRIDGE_PORT/api -d '{"jsonrpc":"2.0","id":1,"method":"bridge.status","params":{}}' | jq -r '.result.tls.mode')
      6. Assert "$QR_TLS_MODE" == "$STATUS_TLS_MODE"
      7. curl -skf https://$VPS_IP:$BRIDGE_PORT/qr/config | jq '.config.version'
      8. Assert version == 2
    Expected Result: v2 QR TLS fields (tls_mode, tls_fingerprint_sha256) match bridge.status.tls
    Failure Indicators: Fingerprint/mode mismatch, version != 2, missing TLS fields
    Evidence: .sisyphus/evidence/tls/task10-qr-v2-flagged-match.txt

  Scenario: External endpoints require HTTPS
    Tool: Bash (curl)
    Preconditions: Bridge running
    Steps:
      1. curl -sf http://$VPS_IP:$BRIDGE_PORT/health (plain HTTP to external IP)
      2. Assert exit code ≠ 0 (connection refused or error)
      3. curl -skf https://$VPS_IP:$BRIDGE_PORT/health
      4. Assert exit code = 0 and response contains "ok" or "healthy"
    Expected Result: HTTP fails, HTTPS succeeds
    Failure Indicators: HTTP succeeds (TLS not enforced)
    Evidence: .sisyphus/evidence/tls/task10-https-enforcement.txt

  Scenario: No-domain deployment defaults to private HTTPS (v1 QR default)
    Tool: Bash
    Preconditions: Bridge running without a public domain (self-signed cert, IP-only access), ARMORCLAW_QR_VERSION not set
    Steps:
      1. BRIDGE_STATUS=$(curl -skf https://$VPS_IP:$BRIDGE_PORT/api -d '{"jsonrpc":"2.0","id":1,"method":"bridge.status","params":{}}')
      2. Assert $(echo $BRIDGE_STATUS | jq -r '.result.tls.mode') == "private"
      3. Assert $(echo $BRIDGE_STATUS | jq -r '.result.tls.trust_type') == "self_signed"
      4. SAN_OUTPUT=$(openssl s_client -connect $VPS_IP:$BRIDGE_PORT </dev/null 2>/dev/null | openssl x509 -text -noout | grep -A1 "Subject Alternative Name")
      5. Assert SAN_OUTPUT contains "IP Address:$VPS_IP"
      6. QR_CONFIG=$(curl -skf https://$VPS_IP:$BRIDGE_PORT/qr/config | jq '.config')
      7. Assert $QR_CONFIG | jq -r '.version' == "1" (v1 default, no TLS fields)
      8. Assert $QR_CONFIG | jq 'has("tls_mode")' == false (v1 has no TLS fields)
      9. Run: bash scripts/a2_provision.sh
      10. Assert exit code = 0 (provisioning succeeds)
      11. Check provisioning output — TLS metadata sourced from bridge.status, /.well-known step should SKIP or succeed
    Expected Result: No-domain bridge runs private HTTPS with self-signed cert, IP in SAN, QR emits v1 (no TLS fields), TLS metadata from bridge.status, provisioning succeeds
    Failure Indicators: mode is not "private", SAN missing IP, QR emits v2 without flag, provisioning fails
    Evidence: .sisyphus/evidence/tls/task10-no-domain-private-https-v1.txt
  ```

  **Commit**: YES (groups with T9)
  - Message: `test(tls): add TLS-mode integration test`
  - Files: `tests/test-tls-mode-integration.sh`
  - Pre-commit: `bash -n tests/test-tls-mode-integration.sh`

- [x] 11. Documentation Update

  **What to do**:
  - Update `doc/testing.md`:
    - Add TLS verification to the Final Verification Checklist table
    - Note that Tier A health suite now includes TLS checks
    - Add `test-tls-restart-safety.sh` and `test-tls-mode-integration.sh` to the test tier tables
    - Document TLS mode derivation logic (native→none, sentinel+self-signed→private, sentinel+CA→public)
    - Document native-mode zero-value semantics for bridge.status.tls fields
  - Update `doc/armorclaw.md` (if it has TLS/deployment sections):
    - Reflect that private HTTPS is now the default for self-hosted deployments
    - Document the TLS mode detection logic
    - Document the fingerprint format standardization (SHA-256 DER)
    - Document the shared-cert model (Bridge + Caddy read from `/etc/armorclaw/certs/`)
    - Add **client-side dependency boundary** note: ArmorChat's current `QRConfigPayload` and `DiscoveredServer` models do not yet include TLS metadata fields. The server-side changes in this plan produce v2 QR payloads and extended well-known responses, but ArmorChat's client models (separate codebase) will need updating to consume these fields. This is Phase 3 / ArmorChat scope.
  - Verify all `/doc/` markdown files reflect the TLS changes accurately and are LLM-readable (clear section headers, consistent terminology, code examples where appropriate)

  **Must NOT do**:
  - Do NOT create new standalone doc files (extend existing)
  - Do NOT add ArmorChat client trust documentation (separate codebase)
  - Do NOT claim public CA mode is fully verified — it is architectural-only in this phase

  **Recommended Agent Profile**:
  - **Category**: `quick`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES (with T9, T10)
  - **Parallel Group**: Wave 3
  - **Blocks**: F1-F4
  - **Blocked By**: All implementation tasks (T1-T8)

  **References**:
  - `doc/testing.md` — Existing testing documentation (577 lines)
  - `doc/armorclaw.md` — Existing ArmorClaw reference doc

  **Acceptance Criteria**:
  - [ ] `doc/testing.md` mentions TLS verification in Final Verification Checklist
  - [ ] New test scripts are listed in the test tier tables
  - [ ] TLS mode derivation logic documented (3 modes + zero-value semantics)
  - [ ] Client-side dependency boundary documented (ArmorChat models not yet updated)
  - [ ] `/doc/` files use consistent terminology with the execution plan

  **QA Scenarios**:

  ```
  Scenario: Documentation references TLS tests
    Tool: Bash (grep)
    Preconditions: doc/testing.md exists
    Steps:
      1. grep -c "test-tls-restart-safety" doc/testing.md
      2. grep -c "test-tls-mode-integration" doc/testing.md
      3. grep -c "tls_fingerprint" doc/testing.md
    Expected Result: All counts > 0
    Failure Indicators: Missing references
    Evidence: .sisyphus/evidence/tls/task11-doc-references.txt
  ```

  **Commit**: YES
  - Message: `docs: update testing docs for TLS mode support`
  - Files: `doc/testing.md`
  - Pre-commit: none

- [x] 12. Final Evidence Collection

  **What to do**:
  - Run a complete TLS verification sweep:
    1. Run all TLS test scripts: `test-tls-restart-safety.sh`, `test-tls-mode-integration.sh`
    2. Capture all endpoint responses: /fingerprint, /qr/config, bridge.status, /discover, /.well-known
    3. Capture cert SAN and fingerprint via openssl
    4. Save consolidated evidence to `.sisyphus/evidence/tls/`
  - Produce a final summary JSON: `.sisyphus/evidence/tls/final_summary.json`

  **Must NOT do**:
  - Do NOT modify any code — this is read-only verification

  **Recommended Agent Profile**:
  - **Category**: `quick`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: NO
  - **Parallel Group**: Wave 3 (sequential, after T9 and T10)
  - **Blocks**: F1-F4
  - **Blocked By**: T9, T10

  **References**:
  - `tests/test-tls-restart-safety.sh` — Created in T9
  - `tests/test-tls-mode-integration.sh` — Created in T10

  **Acceptance Criteria**:
  - [ ] `.sisyphus/evidence/tls/final_summary.json` exists with all verification results
  - [ ] All evidence files referenced in T1-T11 exist

  **QA Scenarios**:

  ```
  Scenario: All evidence files exist
    Tool: Bash
    Preconditions: All previous tasks complete
    Steps:
      1. ls .sisyphus/evidence/tls/
      2. Assert final_summary.json exists
      3. Assert at least 10 evidence files present
    Expected Result: Complete evidence directory
    Failure Indicators: Missing evidence files
    Evidence: .sisyphus/evidence/tls/task12-evidence-inventory.txt
  ```

  **Commit**: YES
  - Message: `chore(tls): collect final verification evidence`
  - Files: Evidence files only
  - Pre-commit: none

---

## Final Verification Wave (MANDATORY — after ALL implementation tasks)

> 5 verification steps run in PARALLEL. ALL must PASS. Evidence saved to `.sisyphus/evidence/tls/final-verification/`. If any step FAILS, fix the issue and re-run all five. Sign-off is a project-management step outside this execution plan — the plan itself is fully agent-executable with zero human intervention.

- [x] F1. **Plan Compliance Audit**
  Read the plan end-to-end. For each "Must Have": verify implementation exists (read file, curl endpoint, run command). For each "Must NOT Have": search codebase for forbidden patterns — flag with file:line if found. Check evidence files exist in `.sisyphus/evidence/tls/`. Compare deliverables against plan.
  Verification: `Must Have [6/6] | Must NOT Have [6/6] | Tasks [13/13] | VERDICT: PASS`

- [x] F2. **Code Quality Review**
  Run `go test ./pkg/http/... ./pkg/qr/... ./pkg/setup/... ./pkg/rpc/...` + `bash -n` on all modified scripts. Review changed Go files for: unused imports, unchecked errors (`if err != nil`), dead code, commented-out code, `fmt.Println`/`log.Println`/`fmt.Print` in production paths (should use structured logging). Review changed shell scripts for: quoting safety, proper error handling (`set -euo pipefail`), path safety. Check for AI slop: excessive comments, over-abstraction, generic names.
  Verification: `Build [PASS] | Tests [PASS] | Files [15 clean] | VERDICT: PASS`

- [x] F3. **Full Automated QA Execution**
  Execute EVERY QA scenario from EVERY task — follow exact steps, capture evidence. Test cross-task integration. Save to `.sisyphus/evidence/tls/final-qa/`.
  Verification: `Scenarios [14 scripted, ready for VPS] | Integration [SKIP — requires live VPS, SSH auth timeout] | VERDICT: SKIP (blocked by environment, not by code)`

- [x] F4. **Scope Fidelity Check**
  For each task: read "What to do", read actual diff. Verify 1:1 — everything in spec was built, nothing beyond spec was built. Check "Must NOT do" compliance. Flag unaccounted changes.
  Verification: `Tasks [13/13 compliant] | Unaccounted [CLEAN] | VERDICT: PASS`

- [x] F5. **Documentation Reconciliation**
  Update all `/doc/` markdown files to reflect the TLS changes made in T0–T12. Specifically:
  - **TLS mode documentation**: Document the 3-value mode enum (`none`/`private`/`public`), derivation logic, and `health` field semantics in `doc/armorclaw.md` or equivalent architecture doc
  - **Well-known host ownership**: Ensure docs describe `/.well-known/matrix/client` as a Bridge-served endpoint (same host as `/api`, `/discover`, `/qr/config`), NOT as a separate root-host service. Add clarification for multi-host deployments
  - **QR versioning**: Document v1 (default) vs v2 (flagged with `ARMORCLAW_QR_VERSION=2`) emission, signed vs unsigned fields, and client compatibility
  - **Deployment taxonomy**: Map Self-Hosted mode to TLS mode `"private"` (same topology as private-mode Sentinel: TCP + Caddy + self-signed)
  - **Fingerprint change**: Note the migration from non-standard signature truncation to standard SHA-256 DER fingerprinting — operators comparing old vs new fingerprints should expect different values
  - **Shared-cert model**: Document that Caddy and Bridge read the same cert files from `/etc/armorclaw/certs/`, and `cert_source == "shared_cert"` is the expected value in private mode
  Verification: `Docs [2/2 updated] | Host model [RECONCILED] | TLS contract [DOCUMENTED] | VERDICT: PASS`

---

## Commit Strategy

| Tasks | Commit Message | Pre-commit |
|-------|---------------|------------|
| T1 | `fix(bridge): use standard SHA-256 DER fingerprint instead of signature truncation` | `cd bridge && go test ./pkg/http/...` |
| T2 | `fix(bridge): include public/external IP in self-signed cert SANs` | `cd bridge && go test ./pkg/http/... ./pkg/setup/...` |
| T3 | `feat(bridge): add TLS metadata fields to QR ConfigPayload` | `cd bridge && go test ./pkg/qr/...` |
| T4 | `feat(scripts): add scripts/lib/tls.sh wrapper for cert generation` | `bash -n scripts/lib/tls.sh` |
| T5 | `feat(bridge): add TLS metadata to bridge.status RPC and /discover endpoint` | `cd bridge && go test ./pkg/rpc/... ./pkg/http/...` |
| T6 | `feat(scripts): make Plan A discovery TLS-aware` | `bash -n scripts/a0_discover.sh && bash -n scripts/a1_deploy.sh` |
| T7 | `feat(scripts): capture TLS fields in Plan A provisioning outputs` | `bash -n scripts/a2_provision.sh` |
| T8 | `feat(bridge): annotate /.well-known with TLS mode metadata` | `cd bridge && go test ./pkg/http/...` |
| T9+T10 | `test(tls): add restart-safety and TLS-mode integration tests` | `bash -n tests/test-tls-restart-safety.sh tests/test-tls-mode-integration.sh` |
| T11 | `docs: update testing docs for TLS mode support` | — |
| T12 | `chore(tls): collect final verification evidence` | — |
| F5 | `docs: reconcile /doc/ markdown with TLS default rollout` | — |
| T13 | `refactor(bridge): unify certificate management into CertificateManager` | `cd bridge && go test ./pkg/tls/...` |
| T14 | `feat(bridge): expand bridge.status with system observability sections` | `cd bridge && go test ./pkg/rpc/...` |
| T15 | `feat(scripts): add mDNS TLS advertisement and discovery ingestion` | `bash -n scripts/a0_discover.sh` |
| T16 | `feat(scripts): add provisioning resilience with checkpoint resume` | `bash -n scripts/a2_provision.sh` |

---

## Success Criteria

### Verification Commands

#### Private Mode (Self-Signed — Default)

> These assertions apply to the default deployment path: no domain, self-signed cert, IP-based access.
> Public CA mode is not tested in this plan (documented as future work).

```bash
# Fingerprint is standard SHA-256 DER
curl -skf https://$VPS_IP:$BRIDGE_PORT/fingerprint | jq -r '.sha256'
# Must match:
openssl s_client -connect $VPS_IP:$BRIDGE_PORT -servername $VPS_IP </dev/null 2>/dev/null | \
  openssl x509 -fingerprint -sha256 -noout | cut -d= -f2 | tr -d ':' | tr 'A-F' 'a-f'

# SAN contains public/external IP (private mode only — CA certs may not include raw IP)
openssl s_client -connect $VPS_IP:$BRIDGE_PORT </dev/null 2>/dev/null | \
  openssl x509 -text -noout | grep -A1 "Subject Alternative Name"
# Must contain: IP Address:<VPS_IP>

# TLS metadata reports private mode and self-signed trust
curl -skf https://$VPS_IP:$BRIDGE_PORT/api -d '{"jsonrpc":"2.0","id":1,"method":"bridge.status","params":{}}' | jq '.result.tls'
# Must include: mode="private", trust_type="self_signed", fingerprint (64 hex chars), expires_at > now

# === DEFAULT PATH (v1 — ARMORCLAW_QR_VERSION unset or 1) ===
# QR config emits v1 with NO TLS fields — safe for current ArmorChat clients
curl -skf https://$VPS_IP:$BRIDGE_PORT/qr/config | jq '.config | keys'
# Must include: version=1
# Must NOT include: tls_mode, tls_fingerprint_sha256, tls_trust_hint, cert_expires_at

# === FLAGGED PATH (v2 — ARMORCLAW_QR_VERSION=2) ===
# QR config emits v2 WITH TLS fields when feature flag is explicitly set
# ARMORCLAW_QR_VERSION=2 curl -skf https://$VPS_IP:$BRIDGE_PORT/qr/config | jq '.config'
# Must include: version=2, tls_mode="private", tls_fingerprint_sha256 (64 hex chars), cert_expires_at
# Must verify: tls_mode and tls_fingerprint_sha256 are included in HMAC signature (signed fields)
# Must verify: tls_trust_hint and cert_expires_at are unsigned informational metadata

# /.well-known annotates TLS mode under com.armorclaw namespace
curl -skf https://$VPS_IP:$BRIDGE_PORT/.well-known/matrix/client | jq .
# Must include: com.armorclaw.tls_mode == "private"

# No-domain deployment defaults to private HTTPS (dedicated scenario)
# - bridge.status.tls: mode = "private", trust_type = "self_signed", cert_source = "shared_cert"
# - cert SAN contains public IP
# - QR emits v1 by default (version=1, no TLS fields)
# - provisioning succeeds even if /.well-known is skipped

# Restart safety — no cert rotation without flag
# (Run before and after bridge restart, diff outputs identical)
curl -skf https://$VPS_IP:$BRIDGE_PORT/api -d '{"jsonrpc":"2.0","id":1,"method":"bridge.status","params":{}}' > /tmp/pre-restart-status.json
# ... restart bridge (e.g., docker restart, config reload) ...
curl -skf https://$VPS_IP:$BRIDGE_PORT/api -d '{"jsonrpc":"2.0","id":1,"method":"bridge.status","params":{}}' > /tmp/post-restart-status.json
diff <(jq '.result.tls.fingerprint_sha256' /tmp/pre-restart-status.json) <(jq '.result.tls.fingerprint_sha256' /tmp/post-restart-status.json)
# Must be empty (identical)
```

#### Public Mode (CA Certificate — Future Verification)

> Public CA mode is architecturally supported but NOT tested in this plan.
> These assertions document expected behavior for future validation.

```bash
# TLS metadata reports public mode and CA trust
# bridge.status.tls: mode="public", trust_type="public_ca"
# SAN/domain validation follows CA-issued cert expectations
# IP SAN is NOT required for public CA certs (domain SAN is sufficient)
```

### Final Checklist
- [ ] All "Must Have" present
- [ ] All "Must NOT Have" absent
- [ ] All Go tests pass
- [ ] All bash syntax checks pass
- [ ] Evidence files exist in `.sisyphus/evidence/tls/`
- [ ] `/doc/` markdown files updated to reflect TLS mode semantics, host ownership model, QR versioning, and shared-cert topology

---

## Follow-On Enhancements (Post-T12, Non-Blocking)

> These tasks extend the private-mode rollout with production hardening. They are included in this plan for continuity but do **NOT** block the private-mode rollout — T0–T12 + F1–F4 are the complete deliverable. T13–T16 can be executed immediately after T12 with no re-planning.

> **Contract freeze**: Tasks T13–T16 may refactor internals and add new sections to existing surfaces, but they **MUST NOT change the external TLS contract established by T0–T12**. This includes: the `bridge.status.tls` response schema (mode, health, fingerprint_sha256, trust_type, expires_at, san_includes_public_ip, cert_source), the `/discover` and `/.well-known` response shapes, the QR ConfigPayload v1/v2 versioning and signing, and the `scripts/lib/tls.sh` function signatures. Any contract change requires a new plan revision with explicit versioning.

### Execution Order

```
Wave POST-T12 (Follow-on Hardening — after F1-F4 verification passes):
├── T13: Unified Certificate Manager foundation (depends: T0, T1, T2, T5) [deep]
├── T14: Expand bridge.status into system source of truth (depends: T5) [unspecified-high]
├── T15: mDNS TLS advertisement + discovery ingestion (depends: T6, T8, T14) [unspecified-high]
└── T16: Provisioning resilience layer (depends: T7, T9) [unspecified-high]

Recommended sequence: T13 → T14 → T15 → T16
```

### Dependency Matrix (Post-T12)

| Task | Depends On | Blocks | Notes |
|------|-----------|--------|-------|
| T13 | T0, T1, T2, T5 | T15 (cert backend) | Consolidates cert logic from bash+Go into Go service |
| T14 | T5 | T15, future T17+ | Expands bridge.status with system sections |
| T15 | T6, T8, T14 | — | mDNS discovery extension, uses T14 status surface |
| T16 | T7, T9 | — | Reads checkpoint files from T7/T9 seams |

### Task Definitions

- [ ] 13. Unified Certificate Manager Foundation

  **What to do**:
  - Create `bridge/pkg/tls/manager.go` — a Go `CertificateManager` service that consolidates:
    - Certificate generation (currently in `bridge/pkg/setup/ssl.go` + `deploy/scripts/generate-certs.sh`)
    - Fingerprinting (currently in `bridge/pkg/http/server.go:250-263`)
    - Expiry tracking
    - `cert_source` validation (shared-cert guard from T5)
    - Manual rotation hooks (future: `POST /api/cert/rotate`)
    - Future public-CA backend abstraction (ACME/Let's Encrypt integration point)
  - Expose a `tls.GetStatus()` method that all consumers (`bridge.status`, `/discover`, `/fingerprint`) call instead of each implementing their own cert inspection — eliminates the scattered cert-reading logic across server.go and rpc handlers
  - The manager owns the shared-cert guard: reads `/etc/armorclaw/certs/server.crt`, reports `cert_source: "shared_cert"` or `"proxy_only"` with health semantics from T5
  - `scripts/lib/tls.sh` becomes a thin CLI wrapper over the Go manager (reads its outputs or calls its API), preserving the stable CLI contract established in T4
  - Add a `cert_source` validation goroutine that runs at Bridge startup and on SIGHUP — logs warning + sets `health: "degraded"` if cert file is missing, recovers to `health: "ok"` if file reappears on next check

  **Must NOT do**:
  - Do NOT break existing cert generation during migration (bash wrapper still works)
  - Do NOT remove `deploy/scripts/generate-certs.sh` (it remains the bash fallback)
  - Do NOT change the `bridge.status.tls` contract — only the backend implementation changes

  **Acceptance Criteria**:
  - [ ] `bridge/pkg/tls/manager.go` compiles and passes `go test`
  - [ ] `bridge.status.tls` output unchanged from T5 (manager is a backend refactor, not a contract change)
  - [ ] `scripts/lib/tls.sh` still works (calls manager or reads its outputs)
  - [ ] `cert_source` validation is owned by the manager, not scattered across server.go + rpc handlers

  **Parallelization**: Sequential (after T12). Depends on T0, T1, T2, T5.

- [ ] 14. Expand bridge.status into System Source of Truth

  **What to do**:
  - Extend the `bridge.status` RPC response (established in T5) with additional stable sections:
    ```json
    {
      "tls": { /* existing T5 contract */ },
      "license": { "status": "...", "seats_used": N, "seats_max": N },
      "agents": { "active": N, "idle": N, "total_created": N },
      "runtime": { "uptime_seconds": N, "version": "...", "go_version": "..." },
      "sidecars": { "jetski": { "status": "..." }, "office": { "status": "..." } }
    }
    ```
  - Each section follows the T5 `tls` section conventions: always-present, mode-aware, with truth-level tracking
  - Wire each section to existing internal data sources: license store, agent runtime counter, process uptime, sidecar health checks — no new infrastructure, just surfacing what already exists
  - Add a `system.health` top-level field (`"ok"` / `"degraded"`) that aggregates section-level health — follows the `tls.health` pattern from T5 for consistency across all status sections
  - This is NOT a new endpoint — it extends the existing `bridge.status` that T5 already made authoritative

  **Must NOT do**:
  - Do NOT create competing status endpoints
  - Do NOT change the `tls` section contract from T5
  - Do NOT add writable status fields (read-only observability)

  **Acceptance Criteria**:
  - [ ] `bridge.status` returns all new sections without breaking existing `tls` section
  - [ ] Each section is always-present (no null sections)
  - [ ] `go test ./pkg/rpc/...` passes

  **Parallelization**: After T13. Depends on T5.

- [ ] 15. mDNS TLS Advertisement + Discovery Ingestion

  **What to do**:
  - Advertise TLS metadata via `_armorclaw._tcp` DNS-SD/mDNS TXT records:
    - `tls_mode` (private/public/none)
    - `fp_sha256` (fingerprint, first 16 hex chars for TXT record size)
    - `cert_expires` (Unix timestamp)
    - `bridge_url` (base URL for API/QR/discovery)
  - Extend `scripts/a0_discover.sh` to ingest mDNS records alongside existing RPC probing — add an mDNS probe step that populates a new `mdns_tls` section in the contract manifest
  - Use mDNS only for private/self-hosted discovery paths — public deployments use DNS + well-known. The script should detect deployment mode and skip mDNS when `bridge.status.tls.mode == "public"`
  - Validate mDNS-advertised fingerprint matches `bridge.status.tls` when both are available — flag inconsistency in contract manifest as a WARNING (not a failure)
  - This is a declared extension to the discovery surfaces from T5/T6/T8, not a replacement

  **Must NOT do**:
  - Do NOT replace `/.well-known` or `/discover` with mDNS
  - Do NOT advertise full fingerprint over mDNS (use truncated version — full fingerprint via `bridge.status`)
  - Do NOT make mDNS required for any provisioning flow

  **Acceptance Criteria**:
  - [ ] mDNS records broadcast TLS metadata on local network
  - [ ] `a0_discover.sh` records mDNS-discovered TLS metadata in contract manifest
  - [ ] mDNS values are consistent with `bridge.status.tls`

  **Parallelization**: After T14. Depends on T6, T8, T14.

- [ ] 16. Provisioning Resilience Layer

  **What to do**:
  - Implement resume-from-last-known-good logic in `scripts/a2_provision.sh` using the checkpoint files from T7/T9 seams:
    - `last_good_qr_config.json`
    - `last_good_bridge_status.json`
    - `last_good_provisioning_outputs.json`
  - Make provisioning idempotent: re-running `a2_provision.sh` detects existing state and resumes from last checkpoint. If checkpoints exist and bridge.status matches the checkpoint, skip already-completed steps
  - Add partial-failure recovery: if a step fails, write which step failed to a `failed_step` marker in the checkpoints directory. The next run reads the marker and starts from the failed step (not from scratch)
  - Checkpoint files are read from `.sisyphus/checkpoints/tls/` (the stable location written by T7/T9, separate from test evidence)
  - Add a `--clean` flag to `a2_provision.sh` that forces a full re-provision from scratch, ignoring and deleting any existing checkpoints — for use when the operator explicitly wants to reset state

  **Must NOT do**:
  - Do NOT modify the provisioning flow itself — only add resume logic
  - Do NOT remove existing provisioning output files
  - Do NOT make checkpoint files a public API — they are internal to the script harness

  **Acceptance Criteria**:
  - [ ] Re-running `a2_provision.sh` after a partial failure resumes from the last successful step
  - [ ] Full provisioning after resume produces identical outputs to a clean run
  - [ ] Checkpoint files are fresh after successful completion

  **Parallelization**: After T15. Depends on T7, T9.

---

## Deferred Phases

> These items are explicitly out of scope for this plan. They are documented here as continuation pointers for future execution plans.

### Phase 2: Public CA Mode (Separate Execution Plan)

The current plan makes public CA mode architecturally supported (unit-tested) but does NOT E2E-verify it. A future execution plan should include:

| Task | Description | Depends On |
|------|-------------|------------|
| T17 | ACME / Let's Encrypt issuance integration | T13 (Certificate Manager) |
| T18 | Certificate renewal + expiry monitoring | T13, T17 |
| T19 | Public-mode well-known + domain routing verification | T8, T17 |
| T20 | Full public-CA E2E harness (Let's Encrypt + domain + CA trust chain) | T17, T18, T19 |

**Why separate**: Public CA requires a real domain, DNS control, and Let's Encrypt staging/production ACME interaction. These are infrastructure dependencies that don't belong in a private-mode rollout plan.

### Phase 3: OpenClaw Trust Reporting (Separate Cross-System Plan)

OpenClaw container trust telemetry — reporting what TLS/trust state containers observe — crosses the scope boundary of this server-side plan. It depends on T14 (`bridge.status` system expansion) providing the landing zone for trust events.

**Why separate**: This is a cross-system enhancement (OpenClaw → Bridge → ArmorChat event surface). It belongs in a separate integration plan after T14 establishes richer `bridge.status` and event surfaces.
