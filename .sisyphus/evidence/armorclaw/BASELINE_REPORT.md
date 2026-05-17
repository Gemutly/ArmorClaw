# ArmorClaw VPS Baseline Report — 2026-04-30

## Status: BASELINE ESTABLISHED

### Deployment
- **VPS**: root@5.183.11.149
- **Bridge Binary**: `/opt/armorclaw/bridge/build/armorclaw-bridge` (md5: 4c7e8740796099f0397ebe095ec62a5e)
- **Service**: `armorclaw-bridge.service` — active
- **Bridge Port**: 8080 (HTTPS)
- **Matrix Port**: 6167 (HTTP/Conduit)
- **Config**: `/etc/armorclaw/config.toml`

### Code Changes This Session
| File | Change |
|------|--------|
| `bridge/pkg/rpc/secretary_handlers.go` | Adapter + NewSecretaryHandler + user_id extraction |
| `bridge/pkg/rpc/server.go` | 3 missing method registrations |
| `bridge/cmd/bridge/main.go` | Secretary handler wiring with nil guard |
| `bridge/pkg/secretary/store.go` | NULL-safe scan for variables/steps/pii_refs (3 occurrences) |
| `scripts/a0_discover.sh` | 5 HTTP→HTTPS fixes |
| `tests/*.sh` (14 files) | zsh `${2:-{}}` → `${2:-{\}}` fix (26 occurrences) |
| `bridge/pkg/rpc/server.go` | PII singleton (previous session) |
| `bridge/pkg/rpc/pii.go` | PII persistence fix (previous session) |

### Test Results

| Suite | PASS | FAIL | SKIP | Status |
|-------|------|------|------|--------|
| A0 Discovery | 16 | 0 | 0 | ✅ Clean |
| A2 Provision | 7 | 0 | 0 | ✅ Operational |
| A3 Events | 1 | 0 | 2 | ✅ Core works; websocat skips |
| A4 Health | 7 | 0 | 1 | ✅ Bridge + Matrix healthy |
| A4 Trust | 33 | 0 | 2 | ✅ PII lifecycle operational |
| A4 Workflow-Core | 29 | 4 | 0 | ⚠️ Template CRUD ✅; start_workflow fails |
| **TOTAL** | **109** | **4** | **5** | |

### Operational Subsystems
- [x] Bridge HTTPS (port 8080)
- [x] Matrix Conduit (port 6167)
- [x] RPC API (89 methods registered, 13 secretary/task live)
- [x] Provisioning (user, room, token)
- [x] PII request/approve/deny lifecycle
- [x] Secretary template CRUD
- [x] Secretary health checks (is_running, get_active_count)

### Known Gaps
1. **secretary start_workflow** — 4 FAIL: workflow not found after creation (persistence/retrieval bug)
2. **EventBus WebSocket** — 5 SKIP: websocat not installed on VPS (low priority)

### Evidence Locations
```
.sisyphus/evidence/armorclaw/
├── contract_manifest.json          # A0: Full API contract
├── a0_summary.json                 # A0: Discovery summary
├── a0_rpc_schemas.json             # A0: RPC parameter schemas
├── a0_matrix_status.json           # A0: Matrix homeserver status
├── a0_tls_status.json              # A0: TLS metadata
├── a4_workflow-core_output.txt     # A4: Full workflow-core output
├── a4_summary.json                 # A4: Harness summary
├── a2_*.json                       # A2: Provisioning artifacts
├── a3_*.json                       # A3: Event test artifacts
└── BASELINE_REPORT.md              # This file
```

### Secretary RPC Verification (Live)
```
secretary.is_running       → {"running":true}           ✅
secretary.get_active_count → {"active_count":0}         ✅
secretary.list_templates   → 1 template seeded          ✅
secretary.create_template  → Creates + persists          ✅
secretary.get_template     → Retrieves by ID             ✅
secretary.delete_template  → Removes                     ✅
secretary.update_template  → Updates fields              ✅
task.list                  → Returns task list            ✅
secretary.start_workflow   → "workflow not found" after   ❌
                              creation (persistence bug)
```

### Next Investigation
**Topic**: secretary start_workflow persistence/retrieval bug
**Scope**: Trace StartWorkflow end-to-end; verify creation, persistence, read-after-write, retrieval by ID
**Out of Scope**: TLS, trust, provisioning, harness scripts
