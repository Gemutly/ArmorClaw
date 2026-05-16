# RPC Method Registration Audit

**Generated**: 2026-05-16 (BEATO Wave 0, T0.6)
**Purpose**: Pre-deploy checkpoint before registering new BEATO RPC methods

## Baseline Count

| Metric | Value |
|--------|-------|
| **Registered methods** | 129 |
| **Expected after BEATO** | 136 (+3 `document.*` + 4 `email.*`) |
| **Handler test references** | 12 unique handler references |

## Method Groups

| Group | Count | Methods |
|-------|-------|---------|
| browser.* | 11 | navigate, fill, click, status, wait_for_element, wait_for_captcha, complete, fail, list, cancel, replay_diagnostics |
| bridge.* | 7 | start, stop, status, channel, unchannel, list, ghost_list, appservice_status |
| pii.* | 9 | request, approve, deny, status, list_pending, stats, cancel, fulfill, wait_for_approval |
| skills.* | 11 | execute, list, get_schema, allow, block, allowlist_add, allowlist_remove, allowlist_list, web_search, web_extract, email_send, slack_message, file_read, data_analyze |
| studio.* | 21 | list_skills, get_skill, register_skill, list_pii, get_pii, register_pii, list_profiles, create_agent, update_agent, delete_agent, list_agents, get_agent, spawn_agent, list_instances, get_instance, stop_instance, stats, list_mcps, get_mcp, get_mcp_warning, request_mcp_approval, list_pending_approvals, list_my_approvals, approve_mcp_request, reject_mcp_request |
| secretary.* | 14 | start_workflow, get_workflow, cancel_workflow, create_workflow, advance_workflow, list_templates, create_template, get_template, delete_template, update_template, is_running, get_active_count, shutdown, set_approval_delegation, get_approval_delegation, revoke_approval_delegation |
| artifact.* | 4 | artifact_upload, artifact_download, artifact_list, artifact_update_status |
| task.* | 4 | create, list, cancel, get |
| keystore.* | 7 | unseal, sealed, seal, extend_session, session_status, list_keys, delete_key |
| matrix.* | 5 | status, login, send, receive, join_room |
| device.* | 4 | list, get, approve, reject |
| invite.* | 4 | list, create, revoke, validate |
| voice.* | 3 | start_session, stop_session, status |
| container.* | 2 | terminate, list |
| events.* | 2 | replay, stream |
| hardening.* | 3 | status, ack, rotate_password |
| email.* | 1 | list_pending (BEATO adds 4 more: queue_status, get, retry, list) |
| provisioning.* | 2 | start, claim |
| sidecar.* | 1 | extraction_mode |
| health.* | 1 | check |
| mobile.* | 1 | heartbeat |
| account.* | 1 | delete |
| ai.* | 1 | chat |
| **document.*** | **0** | **(BEATO adds 3: extract_text, status, list_jobs)** |

## Test Files That Reference Method Names

| File | References | Update After BEATO? |
|------|-----------|---------------------|
| `server_test.go:12-19` | 6 critical methods (hardcoded list) | NO — list won't change |
| `server_test.go:34-45` | 10 expected methods (hardcoded list) | MAYBE — add `document.extract_text` if desired |
| `email_approval_test.go:25-37` | approve_email, deny_email, email_approval_status, email.list_pending | YES — add new email.* method tests after T3.0 |
| `replay_diagnostics_test.go:15` | browser.replay_diagnostics | NO |
| `replay_gating_test.go:34-125` | browser.replay_diagnostics | NO |
| `secretary_handlers_test.go:31` | secretary.get_workflow | NO |
| `account_test.go:15` | account.delete | NO |
| `artifact_handlers_test.go:28-43` | secretary.artifact_upload | NO |
| `test-rpc-methods.sh:119` | health.check only | NO |
| `test-secretary-lifecycle-proof.sh:34-137` | 17 secretary/task methods | NO |
| `.github/workflows/test.yml:52-59` | `go test -short ./pkg/rpc/...` | NO — runs all tests, no count |

## Numeric Count Assertions

**No hardcoded numeric count assertions found** in any CI workflow or test file.
The test at `server_test.go` checks method NAME presence (not count).
CI workflow `test.yml` runs `go test -short ./pkg/rpc/...` with no count expectation.

## Action Items After New Methods Registered

1. After T2.2b (document RPC): Add `document.extract_text` to `server_test.go:TestMethodRegistrationCompleteness` expected methods list
2. After T3.0 (email RPC): Add new email.* method tests to `email_approval_test.go` or new test file
3. Re-run `grep -oP '"[a-z_]+\.[a-z_]+"' bridge/pkg/rpc/server.go | sort -u | wc -l` → expect 136
4. Run `cd bridge && go test ./pkg/rpc/... -count=1` → ALL pass
