# Plan: Make ArmorClaw Default to Private HTTPS with Optional Public CA HTTPS

**Version:** 1.1  
**Date:** April 28, 2026  
**Scope:** ArmorClaw deployment, provisioning, discovery, client trust, and verification flows  
**Purpose:** Ensure every new ArmorClaw deployment uses HTTPS by default, even when the operator has no domain, while preserving an optional public CA path for domain-backed installs. QR-first provisioning remains the primary bootstrap path for private/self-hosted installs, and well-known discovery is treated as primary only when a real domain exists.

---

## 1. Goal

Default ArmorClaw to encrypted transport on every deployment by introducing two TLS modes:

- **Private HTTPS (default)** for VPS installs with no domain
- **Public CA HTTPS (optional)** for installs with a domain and public-trust certificates

The default path must work for self-hosted users with only a VPS IP, without requiring DNS, Cloudflare, or public certificate issuance. The public CA path remains available for operators who want standard browser trust and well-known discovery.

This operationalizes the documented self-hosted split: **Let’s Encrypt or self-signed**, with QR-first provisioning for self-hosted setups.

---

## 2. Decision Model

ArmorClaw setup will choose TLS mode as follows:

- If `DOMAIN` is **unset**, setup defaults to **private HTTPS**
- If `DOMAIN` is **set**, setup offers:
  - `tls_mode=private`
  - `tls_mode=public`
- If the operator does not choose explicitly and a domain exists, default to **public CA HTTPS**
- If public certificate issuance fails, setup may fall back to **private HTTPS** only if the operator opts in or `ALLOW_TLS_FALLBACK=1` is set

This keeps the no-domain path frictionless while preserving the current domain-oriented deployment model.

**TLS scope**: TLS mode selection applies to **all externally consumed control-plane endpoints** used during provisioning, including the effective Matrix homeserver URL exposed to clients, unless a deployment-specific exception is documented.

**Matrix URL consistency**: The published `matrix_homeserver` URL in QR, manual config, and provisioning outputs **must always match** the selected TLS mode and the externally reachable topology. In private mode this means `https://<ip-or-host>:<port>`; in public mode it must use the domain. This prevents Bridge being HTTPS-clean while the homeserver URL remains inconsistent.

---

## 3. Why This Model

ArmorClaw already assumes:

- External Bridge access should be **HTTPS-first**
- Self-hosted deployments may rely on **self-signed certificates**
- **QR-first provisioning** is a first-class path
- **Well-known discovery** fits best when a real domain exists

The gap is not architectural. The gap is **default behavior and enforcement**. This plan makes the documented self-hosted posture the product default instead of leaving it implicit.

---

## 3.1 Upgrade Compatibility Rule (NEW)

**Existing deployments retain their current TLS mode unless the operator explicitly migrates.**

This plan applies to **new deployments** by default. Upgrades of existing domain-backed (public CA) installs must not automatically switch to private HTTPS or regenerate certificates/trust material. Migration is an explicit operator action only.

## 4. Non-Negotiable Architecture Rules

- External Bridge access must be **HTTPS-first**
- Localhost probes over SSH may continue using **HTTP**
- **QR/deep-link provisioning** remains the primary bootstrap path
- `/.well-known/matrix/client` remains preferred **only when a real domain exists**
- No-domain installs must **not depend on DNS** to become usable
- Existing deploy/provision infrastructure must be **extended, not duplicated**
- Evidence containing session or token data must remain **gitignored and redacted** before sharing

---

## 5. Target Modes

### Mode A — Private HTTPS (Default)

Used when no domain is available.

**Properties**
- **TLS termination model**: Bridge runs internal TLS (ListenAndServeTLS with TLS 1.2+). For private mode, the reverse proxy (Caddy) terminates external TLS and proxies to Bridge over localhost HTTP/HTTPS. This is the current self-hosted pattern.
- **Default**: single self-signed server certificate (simple, no private CA root)
- Private CA (ArmorClaw-generated root) is an **optional later enhancement** for advanced deployments that need multiple services or client cert auth
- Bridge is exposed externally as `https://<ip-or-host>:<port>`
- Provisioning uses signed QR/deep-link payloads as the source of truth
- Manual entry and mDNS remain valid fallbacks
- `/.well-known` is **optional** and not required for first pairing

### Mode B — Public CA HTTPS (Optional)

Used when the operator has a real domain.

**Properties**
- TLS terminates at the reverse proxy with a publicly trusted cert
- Well-known discovery is **enabled**
- Matrix and Bridge are exposed on domain-based URLs
- QR remains supported, but well-known/manual entry can be first-class

These two modes match the documented discovery split between QR/self-hosted flows and domain-based well-known flows.

---

## 6. Operator Inputs

```bash
TLS_MODE=auto                # auto | private | public
DOMAIN=                      # optional
PUBLIC_IP=                   # required if DOMAIN is empty
BRIDGE_PORT=8443             # external TLS port
MATRIX_PORT=6167             # external Matrix port
TLS_CERT_PATH=               # optional override
TLS_KEY_PATH=                # optional override
ALLOW_TLS_FALLBACK=0         # 0 | 1
```

**Note**: The port numbers (8443, 6167) and example URLs throughout this plan are **illustrative only**. Implementations and tests **must not hardcode** these sample values. All published URLs and ports must be derived from actual deployment configuration and the externally reachable topology.

**Behavior**

- `TLS_MODE=auto` + no `DOMAIN` → **private HTTPS**
- `TLS_MODE=auto` + `DOMAIN` set → **public CA HTTPS**
- `TLS_MODE=private` → force private HTTPS even with a domain
- `TLS_MODE=public` → require domain and fail clearly if certificate issuance fails

---

## 7. Provisioning and Discovery Policy

### Private HTTPS bootstrap order

1. **Signed QR / deep link**
2. **mDNS** (if supported and available)
3. **Manual entry**
4. **Fallback servers** (only if explicitly configured)

### Public CA bootstrap order

1. **Well-known discovery**
2. **Signed QR / deep link**
3. **Manual entry**
4. **mDNS** (as optional local-network assist)

This is consistent with the existing ArmorChat discovery model.

---

## 8. Certificate Trust Model

### Private HTTPS

ArmorClaw will use a **fingerprint-based trust flow** for first pairing:

- Generate certificate fingerprint during deploy
- Embed fingerprint in the signed provisioning payload
- Show fingerprint confirmation during first client connection
- Persist accepted trust record
- Warn on mismatch after certificate rotation
- Require explicit user acknowledgment before trusting a new fingerprint

This is **new productized behavior** built on top of the current self-hosted/self-signed posture. It should not be assumed complete already.

### Public CA HTTPS

- Use standard CA validation
- No self-signed trust prompt in the normal path
- Optional pinning remains available if product policy wants it

---

## 9. Client Trust Ownership

This plan makes **client ownership explicit**.

**ArmorChat ownership**  
Client trust UX changes belong in the **ArmorChat repository**, specifically the pairing/provisioning and secure storage flow. Initial implementation should be mapped against existing pairing and provisioning classes before coding.

**Deliverables owned by ArmorChat**

- Trust prompt for private HTTPS certificates
- Fingerprint display during pairing
- Persisted accepted fingerprint or trust record
- Mismatch / rotation warning flow
- Secure storage of trust material
- Manual-entry trust acceptance flow for `https://<ip>:<port>`

### Server-Side Ownership Map

| Component                              | Responsibilities                                                                 | Concrete Surfaces                                      |
|----------------------------------------|----------------------------------------------------------------------------------|--------------------------------------------------------|
| **Bridge / deploy scripts**            | Generate certs, detect proxy, emit fingerprint/expiry                            | `scripts/lib/tls.sh`, `deploy-infrastructure.sh`       |
| **Provisioning API / `/qr/config`**    | Publish TLS metadata (mode, fingerprint, expiry)                                 | `/qr/config` endpoint, provisioning outputs            |
| **ArmorChat**                          | Trust prompt, persist trust, mismatch/rotation flow                              | Pairing flow, secure storage, `DeepLinkHandler`        |
| **Admin/status surface**               | Display mode, fingerprint type, expiry, rotation state                           | `bridge.status`, admin UI, status RPC, deployment evidence |

### Trust State Machine

| State                  | Meaning                              | Next Action                  |
|------------------------|--------------------------------------|------------------------------|
| `UNTRUSTED`            | Server cert unknown                  | Prompt user                  |
| `PENDING_CONFIRMATION` | Fingerprint displayed                | Accept or reject             |
| `TRUSTED`              | Fingerprint accepted and stored      | Normal connection            |
| `MISMATCH_DETECTED`    | Presented cert differs from stored   | Block and require re-approval|
| `ROTATED_CONFIRMED`    | User accepted expected cert rotation | Replace stored trust         |

---

## 10. Certificate Lifecycle

This was missing in v1.0 and is now required.

### Private mode lifecycle

- Auto-generate certificate with defined validity window
- Surface certificate expiry date in admin/status UI
- Warn when cert is nearing expiry
- Support explicit certificate rotation
- **Rotation rule**: Certificate rotation is **manual by default**. Redeploy or upgrade **must not** rotate certs unless `ROTATE_TLS_CERT=1` or an explicit migration/rotation action is invoked. This prevents accidental trust breakage on existing clients.

**Certificate generation requirements** (private mode):
- Self-signed cert **must include** the externally reachable IP and/or host in the Subject Alternative Name (SAN)
- Fingerprint format **must be standardized as SHA-256 of the DER-encoded certificate** (RFC 5280 style, hex) and used consistently across `/qr/config`, `bridge.status`, provisioning outputs, and client trust UX. Current code uses raw signature truncation — this must be updated to the standard form.

### Public mode lifecycle

- Configure automated renewal for the public CA path
- Verify renewal job or timer is active
- Expose renewal state and expiry in admin/status UI

### Both modes

- Show **“certificate expires in X days”** in deployment status
- Include expiry metadata in provisioning/admin status outputs
- Fail closed on mismatched cert unless operator/user explicitly re-approves

---

## 11. Deployment Changes

### D1 — Extend existing deployment scripts

Do **not** create a parallel deploy system. Extend the existing deployment path. The testing and Plan A docs already emphasize reuse of current infrastructure.

### D2 — Add TLS bootstrap helper

Add a helper such as:

```text
scripts/lib/tls.sh
```

**Responsibilities**
- Decide TLS mode from env
- **Wrap** `deploy/scripts/generate-certs.sh` (do not duplicate logic)
- Pass external IP (`PUBLIC_IP`) to cert generation for SAN inclusion
- Validate provided cert/key overrides
- Issue public cert in public mode
- Emit metadata:
  - `mode`
  - `subject` / `SAN`
  - `fingerprint`
  - `expires_at`
  - `trust_type`

### D3 — Reverse proxy standardization

The reverse proxy **must always front the Bridge over TLS externally**.

**Hard requirement**: Proxy detection **must occur before** any template selection. Private and public modes **must not fork** deployment into parallel proxy systems. Detect the existing implementation (Caddy, Nginx, Traefik, etc.) from `deploy-infrastructure.sh` / compose assets first, then extend only that implementation with mode-specific configuration.

**Private mode**
- Listen on external TLS port
- Proxy to localhost Bridge backend
- Expose `/health` and `/api` via HTTPS
- Proxy Matrix as needed by current topology

**Public mode**
- Same pattern, but with domain-based routing and public cert issuance

### D4 — HTTPS-first health and status

All external checks must use HTTPS:

```bash
curl -skf https://$VPS_IP:$BRIDGE_PORT/health
curl -skf https://$VPS_IP:$BRIDGE_PORT/api ...
```

Localhost-over-SSH may remain HTTP. The testing docs already require this distinction.

---

## 12. Provisioning Changes

### P1 — Extend QR payload

The signed QR/deep-link payload must include TLS trust metadata in private mode. The `ws_url` field is **optional** (ArmorChat is Matrix-first; WebSocket is not required for all setups).

**Example payload** (valid JSON — `ws_url` may be omitted):

```json
{
  "matrix_homeserver": "https://203.0.113.10:6167",
  "rpc_url": "https://203.0.113.10:8443/api",
  "tls_mode": "private",
  "tls_fingerprint_sha256": "a1b2c3d4e5f67890abcdef1234567890abcdef1234567890abcdef1234567890",
  "tls_trust_hint": "self_signed",
  "cert_expires_at": 1714410000,
  "expires_at": 1714410000
}
```

**Implementation note**: Adding `tls_mode`, `tls_fingerprint_sha256`, `tls_trust_hint`, and `cert_expires_at` requires updating the `ConfigPayload` struct and the HMAC signature computation (`signConfig`). The external IP must be passed to cert generation (currently missing in both `bridge/pkg/http/server.go` and `deploy/scripts/generate-certs.sh`).

QR remains the source of truth in no-domain mode.

### P2 — `/.well-known` policy

- **Public mode**: generate and verify `/.well-known/matrix/client`
- **Private mode**: best-effort only, **must not block** provisioning

### P3 — Manual entry

For private mode manual entry:

- Accept `https://<ip>:<port>`
- Show trust prompt
- Allow proceed **only after explicit confirmation**
- Persist accepted trust record

---

## 13. Testing and Harness Changes

Because script names may drift, the plan targets the relevant Plan A and harness scripts **by role**, not by assuming every exact filename is fixed.

### T1 — Discovery scripts

Update the relevant Plan A discovery scripts (`scripts/a0_*.sh` / discovery helpers) so that:

- External probes are **HTTPS-first**
- Discovery records:
  - `tls_mode`
  - `external_scheme`
  - `cert_trust`
  - `cert_expires_at`
  - `well_known_required`

### T2 — Deployment scripts

Update the relevant deployment scripts (`scripts/a1_*.sh` / deploy helpers) so that:

- Private HTTPS with **no domain** is supported directly
- Fingerprint and expiry are recorded in evidence
- External `/health` and `/api` checks are **HTTPS**
- DNS absence does **not** fail private mode

### T3 — Provisioning scripts

Update the relevant provisioning scripts (`scripts/a2_*.sh`) so that:

- `/qr/config` is **required** in private mode
- `/.well-known` is required **only** in public mode
- Provisioning outputs record TLS mode, fingerprint, and expiry
- Blocked Matrix registration remains a controlled SKIP

### T4 — Event validation

No architecture change. Event validation continues using the existing Matrix-based model and degraded behavior when session availability is limited. It should simply consume HTTPS URLs from provisioning outputs.

### T5 — Health suite

Update `tests/test-system-health-baseline.sh` or the equivalent baseline health suite to verify:

- External HTTPS works
- Private-mode QR payload contains trust metadata
- Public-mode well-known exists when public mode is selected
- Cert expiry is present in status/evidence

---

## 14. Implementation Phases

### Phase 0 — Design lock

- Approve TLS mode decision tree
- Approve QR payload extension
- Approve client trust UX and ownership
- Approve certificate lifecycle behavior

### Phase 1 — Deployment plumbing

- Add `scripts/lib/tls.sh`
- Extend existing deploy infrastructure
- Add or extend proxy templates/config
- Add TLS metadata to deploy evidence

### Phase 2 — Provisioning integration

- Extend `/qr/config`
- Add TLS fields to provisioning outputs
- Make well-known conditional by mode
- Add TLS summary to admin/status output

### Phase 3 — Client trust flow (ArmorChat dependency — out of scope for this repo)

**Server-side API contract only** (this repo):
- Define `/trust/accept` and `/trust/status` endpoints (or extend `bridge.status`)
- Return current TLS mode, fingerprint, expiry, and rotation state
- Accept client-reported trust acceptance for audit logging

**ArmorChat work** (separate repo):
- Implement trust prompt, fingerprint display, mismatch/rotation flow, and secure persistence in the Android/KMP client

This phase is a dependency, not part of the current execution scope.

### Phase 4 — Verification

- Update discovery/deploy/provision scripts
- Update baseline health suite
- Add private-mode deployment scenario
- Add public-mode deployment scenario
- Verify both modes on real VPS targets

---

## 15. Recommended Deliverables

```text
scripts/lib/tls.sh
scripts/deploy-infrastructure.sh                # extended
scripts/provision-matrix.sh                     # extended if needed
scripts/a0_*.sh or equivalent discovery scripts # HTTPS-first and TLS-aware
scripts/a1_*.sh or equivalent deploy scripts    # TLS mode aware
scripts/a2_*.sh or equivalent provision scripts # TLS metadata aware
tests/test-system-health-baseline.sh            # TLS checks added
deploy/templates/proxy-private.*                # proxy-specific extension
deploy/templates/proxy-public.*                 # proxy-specific extension
doc/tls-modes.md
doc/provisioning-private-https.md
```

---

## 16. Acceptance Criteria

The plan passes when **all** of the following are true:

- A no-domain VPS install comes up with **private HTTPS by default**
- External Bridge health and RPC checks succeed over **HTTPS**
- QR provisioning works for private mode **without relying on `/.well-known`**
- Public CA mode works when a domain is present
- `/.well-known/matrix/client` is required **only in public CA mode**
- ArmorChat/manual pairing can connect to private-mode deployments with **explicit trust confirmation**
- Private-mode certificate has a **clear expiry date** and admin-visible warning
- Public-mode renewal is configured and verified
- Both modes surface TLS mode, fingerprint/trust type, and expiry in status or evidence
- Plan A and the harness can validate both modes **without hardcoded domain assumptions**
- **Existing public-CA/domain deployments remain unchanged after upgrade** unless the operator explicitly requests migration
- **Upgrade safety test**: The harness must verify that an existing public-CA deployment remains unchanged (TLS mode, certs, trust material, **and all published provisioning outputs** including `/qr/config`, `bridge.status`, and provisioning manifests) after upgrade when no migration flag is set.
- Private-mode self-signed cert **includes the externally reachable IP/host in SAN**
- Provisioning succeeds when `ws_url` is omitted from the QR payload
- **Fingerprint standardization test**: `/qr/config`, `bridge.status`, and client trust UX all return the same SHA-256-of-DER value
- **External IP in SAN test**: Generated private-mode cert contains the `PUBLIC_IP` in Subject Alternative Name

---

## 17. Must-Not-Do Rules

- Do **not** make no-domain installs depend on DNS
- Do **not** keep external health/RPC probes on plain HTTP
- Do **not** require `/.well-known` in private mode
- Do **not** duplicate deployment infrastructure when the existing deploy path can be extended
- Do **not** assume public certificate issuance is always available
- Do **not** hardcode reverse proxy implementation; **detect and extend** what already exists
- Do **not** commit unredacted session, token, or QR-secret evidence

---

## 18. Recommended Status Line

> ArmorClaw should default to **private HTTPS** for self-hosted VPS deployments without a domain, using **QR-first provisioning** and **explicit certificate trust establishment**. Public CA HTTPS remains an optional domain-backed mode with well-known discovery, automated renewal, and standard browser trust.

---

## 19. CTO Assessment

**This is the correct product move.**

The current system already assumes HTTPS externally, already supports QR-first provisioning, and already acknowledges self-signed certificates in self-hosted deployments. The missing piece was not architecture. It was **default behavior, ownership, lifecycle, and verification**. This v1.1 plan closes those gaps cleanly.

---

**End of Plan v1.1**