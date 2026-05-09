# License System

> Part of the [ArmorClaw System Documentation](armorclaw.md)

## Overview

The ArmorClaw license system controls access to premium features like platform bridging, compliance modes, and advanced security. It has three parts: a standalone microservice that manages license records, a client library inside the Bridge that caches validation results and handles offline scenarios, and an enforcement layer that gates features by tier.

The system is designed to be resilient. If the license server becomes unreachable, the Bridge keeps operating using cached results for a configurable grace period. This means temporary network blips don't take down production deployments.

## Architecture

```
┌──────────────────────┐
│   license-server     │  Standalone Go microservice (Docker)
│   (PostgreSQL)       │  Owns the license database, handles
│                      │  validation, activation, admin ops
└──────────┬───────────┘
           │ HTTP (JSON API)
           ▼
┌──────────────────────┐
│  bridge/pkg/license  │  Client library inside the Bridge
│  Client              │  Caches validations, offline-first
│  StateManager        │  Runs periodic polls, tracks state
└──────────┬───────────┘
           │
           ▼
┌──────────────────────┐
│ bridge/pkg/enforcement│ Feature gating by tier
│ Manager              │  Platform limits, compliance modes
│ BridgeEnforcer       │  Hooks into bridge lifecycle
│ BridgeHook           │
└──────────────────────┘
```

The license server runs as its own Docker container. The Bridge communicates with it over HTTP. On startup, the Bridge validates its license, caches the result, and then rechecks periodically in the background.

### License Tiers

| Tier | Key | Default Instances | Bridging | Compliance |
|------|-----|--------------------|----------|------------|
| Free | `free` | 1 | Slack only (3 channels, 10 users) | Basic |
| Pro | `pro` | 3 | Slack, Discord, Teams (50 channels, 200 users) | Standard with PHI scrubbing |
| Enterprise | `ent` | 10 | All platforms, unlimited channels/users | Strict with HIPAA, tamper evidence |

## Key Packages

### License Server (`license-server/`)

The license server is a standalone Go HTTP service backed by PostgreSQL. It auto-creates its schema on startup.

**Endpoints:**

| Method | Path | Auth | Purpose |
|--------|------|------|---------|
| POST | `/v1/licenses/validate` | License key | Validate a license and optional feature |
| POST | `/v1/licenses/activate` | None | Register a bridge instance |
| GET | `/v1/licenses/status` | Admin token | Full license details with usage stats |
| POST | `/admin/v1/licenses` | Admin token | Create a new license |
| DELETE | `/admin/v1/licenses/{key}` | Admin token | Revoke a license |
| GET | `/health` | None | Health check (pings database) |

**Validation flow:**

1. The request includes a license key, instance ID, optional feature name, and version.
2. The server checks key format, rate limits the request, then queries the database.
3. It verifies the license exists, is active, and hasn't expired.
4. If a specific feature was requested, it checks whether that feature is included in the license.
5. It registers or updates the instance heartbeat.
6. Every validation is logged to the `validations` table for audit purposes.

**Activation flow:**

Activation uses a database transaction with `SELECT FOR UPDATE` row locking to prevent race conditions when multiple instances try to activate simultaneously. If the license has a `max_instances` limit and that limit is reached, the request is rejected with `INSTANCE_LIMIT_EXCEEDED`.

**Rate limits** are per-license-key, hourly, and vary by tier: Free gets 100 requests/hour, Pro gets 1,000, and Enterprise gets 10,000.

### License Client (`bridge/pkg/license/`)

The client library runs inside the Bridge process. It has two main types: `Client` handles validation requests and caching, while `StateManager` handles runtime state tracking and polling.

#### Client

`Client` wraps HTTP calls to the license server with an offline-first caching strategy:

- On first call for a feature, it contacts the server, validates, and caches the result.
- Subsequent calls for the same feature return the cached result without a network round trip.
- If the server is unreachable, it falls back to the cache.
- Cache entries include a grace period (default 3 days) calculated from the server's reported expiration. During grace, the Bridge keeps running even though the license has technically expired.
- `OfflineMode` can be set to never contact the server, relying entirely on cached data.

The cache is keyed by feature name, so checking different features produces separate cache entries. A special `"license-info"` feature key fetches general license metadata without checking a specific feature.

#### StateManager

`StateManager` tracks the overall license state across the Bridge's lifetime. It defines these states:

| State | Behavior | Meaning |
|-------|----------|---------|
| `Valid` | Normal | License active, all features available |
| `GracePeriod` | Degraded or ReadOnly | License expired but within grace window |
| `Expired` | Blocked or ReadOnly | Grace period over, service paused |
| `Invalid` | Blocked | License revoked or malformed |
| `Unknown` | Degraded | Could not reach server on startup |

**Startup:** `Initialize()` calls the validator once. If the server is unreachable, it sets state to `Unknown` with `BehaviorDegraded`, so the Bridge can still run in a limited capacity.

**Polling:** `StartPolling()` launches a background goroutine that revalidates at a configurable interval (default 24 hours). When a state transition occurs (Valid to GracePeriod, GracePeriod to Expired), it fires an alert through the `AlertSender` interface.

**Alert thresholds** are configurable. The defaults fire warnings at 30, 14, 7, and 1 day before expiry.

**Operation gating:** `CanPerformOperation()` checks whether a given operation type (read, write, container create, admin access, etc.) is allowed under the current behavior. In `BehaviorDegraded`, admin and config change operations are blocked. In `BehaviorBlocked`, everything is blocked.

### Enforcement (`bridge/pkg/enforcement/`)

The enforcement layer sits between the Bridge's business logic and the license client. It answers questions like "can I bridge to Discord?" and "should I scrub PHI on this message?"

#### Manager

`Manager` holds a registry of all known features, each tagged with a minimum tier, category, and compliance flag. On creation, it registers these features:

- **Bridging:** Slack (Free), Discord (Pro), Teams (Pro), WhatsApp (Enterprise)
- **Compliance:** PHI scrubbing (Pro), HIPAA mode (Enterprise), audit export (Pro), tamper evidence (Enterprise)
- **Security:** SSO (Pro), SAML (Enterprise), MFA enforcement (Pro), hardware keys (Enterprise)
- **Voice:** Calls (Free), recording (Enterprise), transcription (Enterprise)
- **Management:** Dashboard (Pro), REST API (Pro), webhooks (Pro), priority support (Enterprise)
- **Limits:** Unlimited bridges (Pro), unlimited users (Enterprise)

`CheckFeature()` returns whether a feature is allowed. If no license is loaded, it falls back to Free tier defaults. If the license is expired but grace mode is enabled (not strict), Free tier features stay available.

`GetComplianceMode()` derives the compliance level from the license tier and its feature flags:

| Compliance Mode | Trigger |
|-----------------|---------|
| `none` | No license |
| `basic` | Free tier or expired Pro |
| `standard` | Pro tier with PHI scrubbing feature |
| `full` | Enterprise tier (without HIPAA feature) |
| `strict` | Enterprise tier with HIPAA feature |

`GetPlatformLimit()` returns per-platform limits (channels, users, messages/day) adjusted for the current tier. Enterprise gets unlimited everything; Pro gets increased quotas; Free gets the base limits.

#### BridgeEnforcer and BridgeHook

`BridgeEnforcer` wraps the Manager with bridge-specific checks. `BridgeHook` provides lifecycle hooks that the Bridge's AppService integration calls at key moments:

- `BeforeBridgeStart()` checks that at least the Slack bridge feature is available (Free tier minimum).
- `BeforeAdapterStart(platform)` checks that bridging to a specific platform is allowed.
- `BeforeChannelBridge(platform, currentCount)` checks platform access and enforces channel count limits.
- `ShouldScrubPHI()` returns whether PHI scrubbing is active and at what compliance level.
- `ShouldAuditLog()` returns whether audit logging is required.
- `GetComplianceConfig()` bundles all compliance settings into a single config struct.

#### LicenseStatusHandler

`LicenseStatusHandler` exposes license state over RPC. It provides `GetStatus()` (full license info plus platform availability), `RefreshLicense()`, `CheckFeatureAccess()`, and `GetComplianceMode()`. This is how the RPC layer and dashboard query the current license state.

## Configuration

### License Server

| Variable | Default | Required | Description |
|----------|---------|----------|-------------|
| `PORT` | `8080` | No | HTTP listen port |
| `DATABASE_URL` | | Yes | PostgreSQL connection string |
| `ADMIN_TOKEN` | | Yes | Bearer token for admin endpoints |
| `GRACE_PERIOD_DAYS` | `3` | No | Days after expiry before hard block |

### Bridge License Client

| Setting | Default | Description |
|---------|---------|-------------|
| `ServerURL` | `https://api.armorclaw.com/v1` | License server base URL |
| `LicenseKey` | | License key for this instance |
| `InstanceID` | Auto-generated | Unique instance identifier |
| `GracePeriodDays` | `3` | Local grace period if server unreachable |
| `OfflineMode` | `false` | Never contact server |
| `Timeout` | `10s` | HTTP request timeout |

### State Manager

| Setting | Default | Description |
|---------|---------|-------------|
| `GracePeriodDuration` | `3 days` | Time after expiry before hard block |
| `PollInterval` | `24 hours` | How often to revalidate |
| `AlertThresholds` | `[30, 14, 7, 1]` | Days before expiry to send alerts |
| `BlockOnExpired` | `true` | Block all ops when fully expired |
| `ReadOnlyOnGrace` | `false` | Restrict to reads during grace |

### Enforcement

| Setting | Default | Description |
|---------|---------|-------------|
| `DefaultComplianceMode` | `basic` | Compliance level when license missing |
| `EnableGracePeriod` | `true` | Allow degraded mode on expired license |
| `GracePeriodDays` | `3` | Grace window |
| `StrictMode` | `false` | Block all features on invalid license |

### Team-Aware Enforcement

The license system integrates with the team subsystem (`bridge/pkg/team/`) to enforce per-team governance and audit compliance:

**Team Governance** (`bridge/pkg/team/governance.go`):
- `GovernanceConfig` defines team size limits (`MaxMembersPerTeam`, `MaxTeamsPerInstance`) and allowed roles
- `GovernanceEnforcer` validates team creation, member additions, and role assignments against governance limits
- Per-team policy overrides allow individual teams to deviate from default risk-class handling (`overrides map[teamID][riskClass] → ALLOW or DEFER`)

**Team Audit Events** (`bridge/pkg/team/audit.go`):
- 7 event types: `team_created`, `team_dissolved`, `member_added`, `member_removed`, `role_assigned`, `delegation_sent`, `handoff_complete`
- Each governance mutation emits a `TeamAuditEntry` with event ID, team ID, agent ID, role, and timestamp

**Team Metrics** (`bridge/pkg/team/metrics.go`):
- Per-team tracking: token usage, cost, latency, handoff success rate, secret access count, approval rates
- `TeamMetricsSnapshot` provides read-only metric views per team

**Team Roles** (`bridge/pkg/team/roles.go`):
- Built-in role registry with capability sets (browser, form filling, document processing, etc.)
- Roles gated by license tier — Pro and Enterprise tiers unlock additional capabilities

## Integration Points

### Bridge Startup Sequence

1. Bridge creates a `license.Client` with the configured license key.
2. Bridge creates a `StateManager` with the client as its validator.
3. `StateManager.Initialize()` contacts the license server. If the server is reachable and the key is valid, state becomes `Valid`. If unreachable, state becomes `Unknown` with `BehaviorDegraded`.
4. Bridge creates an `enforcement.Manager` with the license client.
5. Bridge calls `Manager.RefreshLicense()` to cache the license.
6. Bridge creates a `BridgeEnforcer` and `BridgeHook` from the Manager.
7. `BridgeHook.BeforeBridgeStart()` confirms the minimum bridge feature is available.
8. `StateManager.StartPolling()` begins background revalidation.
9. `Manager.StartPeriodicRefresh()` begins background license refresh.

### During Operation

- The StateManager polls the server at the configured interval (default 24 hours).
- The enforcement Manager runs its own periodic refresh goroutine.
- Each platform adapter checks `BridgeHook.BeforeAdapterStart()` before connecting.
- Each new channel bridge checks `BridgeHook.BeforeChannelBridge()` to enforce limits.
- Messages passing through may be PHI-scrubbed or audit-logged depending on `GetComplianceConfig()`.
- RPC handlers use `LicenseStatusHandler.GetStatus()` to report license state to callers.

### When a License Expires

1. The server returns `LICENSE_EXPIRED` on the next validation.
2. The StateManager transitions to `StateGracePeriod` (if within grace) or `StateExpired`.
3. During grace: `BehaviorDegraded` allows most operations but blocks admin/config changes. The dashboard shows a warning.
4. After grace: `BehaviorBlocked` pauses all operations. The dashboard shows an error page.
5. Alerts fire on state transitions through the `AlertSender` interface.

### Docker Deployment

The license server runs as a separate container in the Docker Compose stack. It needs a PostgreSQL database (configured via `DATABASE_URL`). The Bridge container connects to it over the Docker network using the configured `ServerURL`.

Typical `docker-compose.yml` addition:

```yaml
license-server:
  build:
    context: ./license-server
  environment:
    DATABASE_URL: postgres://user:pass@license-db:5432/licenses
    ADMIN_TOKEN: ${LICENSE_ADMIN_TOKEN}
    GRACE_PERIOD_DAYS: "3"
  depends_on:
    - license-db
  ports:
    - "8080:8080"

license-db:
  image: postgres:16-alpine
  environment:
    POSTGRES_DB: licenses
    POSTGRES_USER: user
    POSTGRES_PASSWORD: ${LICENSE_DB_PASSWORD}
  volumes:
    - license-db-data:/var/lib/postgresql/data
```
