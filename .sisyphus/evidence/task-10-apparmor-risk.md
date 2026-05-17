# AppArmor Risk Acceptance — BEATO v1.1

## Risk Level
**MEDIUM** — AppArmor profile `armorclaw-office-worker` does not exist. Current Docker hardening provides significant containment.

## Current State
- AppArmor profile `armorclaw-office-worker` does NOT exist in the repository
- AppArmor is commented out in `deploy/docker-compose.sidecar-py.yml` (lines 60-64)
- The Python sidecar runs WITHOUT an AppArmor confinement profile

## Compensating Controls (Active)

The following Docker security controls are in place and verified:

1. **`network_mode: none`** — No network access. The sidecar cannot make outbound or inbound network connections.
2. **`cap_drop: ALL`** — All Linux capabilities dropped. No privileged operations possible.
3. **`read_only: true`** — Read-only root filesystem. Prevents filesystem modification.
4. **`security_opt: no-new-privileges:true`** — Prevents privilege escalation via setuid binaries.
5. **HMAC-SHA256 token validation** — Sidecar only processes authenticated requests from the Go Bridge.
6. **Unix domain socket only** — Communication via `/run/armorclaw/sidecar-office.sock` (0600 perms). No TCP exposure.

## What AppArmor Would Add

An AppArmor profile would provide:
- File path restrictions (only allow reads from /tmp, /data, /usr)
- Prevent execve of non-whitelisted binaries (no shell escapes)
- Limit mount, ptrace, and syscall access
- Fine-grained network policy (even within network_mode: none)
- Audit logging of denied operations

## Risk Assessment

| Factor | Rating | Justification |
|--------|--------|---------------|
| Likelihood of exploitation | LOW | Container has no network, no capabilities, read-only filesystem |
| Impact if exploited | MEDIUM | Could access documents in mounted volume |
| Compensating control strength | HIGH | 6 independent controls provide defense-in-depth |
| Overall risk | **MEDIUM** | Acceptable with current controls, improvement planned |

## Follow-Up Task
1. Create `deploy/apparmor/armorclaw-office-worker` AppArmor profile
2. Profile should allow: read /usr, read/write /tmp/sidecar-*, read /run/armorclaw/*.sock
3. Profile should deny: network, mount, ptrace, execve of non-whitelisted binaries
4. Integrate into `deploy/docker-compose.sidecar-py.yml` uncommenting security_opt
5. Test in CI with AppArmor enabled
6. Target: next hardening sprint

## Decision
**ACCEPTED** — Risk is MEDIUM with strong compensating controls. AppArmor profile creation scheduled for next hardening sprint.

## Verified By
- Evidence: compensating controls verified in `deploy/docker-compose.sidecar-py.yml`
- Evidence: healthcheck verification confirms no auth deadlock
- Date: BEATO v1.1 sprint
