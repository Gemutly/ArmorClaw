# Matrix Command-Path Audit & Route Inventory

> **Branch:** stabilization-v4  
> **Audited:** processEvents() in `matrix.go`  
> **Date:** 2026-05-09

## Executive Summary

`processEvents()` has a single routing point for command dispatch: `m.studioCmdHandler.HandleMatrixMessage()`. However, **no handler is ever registered** — `SetStudioCommandHandler()` and `SetCommandHandler()` are defined but never called from production code. The result: **all commands are unhandled at the routing layer** and fall through to the generic `eventQueue`.

Four separate command handler implementations exist across the codebase, but none are wired into the live path.

---

## Architecture: How processEvents() Routes Commands

```
processEvents()                          [matrix.go:737]
  │
  ├─ m.room.message events (line 753)
  │   ├─ trust/PII validation (lines 755-863)
  │   ├─ studioCmdHandler.HandleMatrixMessage() (line 875) ← ONLY routing point
  │   │   └─ if nil → "WARNING: studioHandler is nil" (line 881)
  │   │   └─ if handled → continue (skip queue)
  │   └─ else → eventQueue (line 885)
  │
  ├─ m.room.encrypted events (line 907)
  │   ├─ decrypt → same studioHandler check (line 978)
  │   └─ else → eventQueue (line 985)
  │
  └─ custom event types → publishCustomEvent (lines 1004-1036)
```

**Critical finding:** `studioCmdHandler` is always `nil` because `SetStudioCommandHandler()` is never called from any production wiring code.

---

## Handler Inventory

### Handler 1: `adapter.CommandHandler` — commands_integration.go

| Aspect | Detail |
|--------|--------|
| **Type** | `*CommandHandler` (struct in adapter package) |
| **Storage** | `m.commandHandler` field on MatrixAdapter |
| **Setter** | `SetCommandHandler(h *CommandHandler)` — **never called** |
| **Live path** | `ProcessMessageWithCommands()` — **never called from processEvents()** |
| **Response type** | `m.notice` via `SendMessageWithRetry` |

**Commands it would handle:**

| Command | Handler Method | Routed? |
|---------|---------------|---------|
| `/claim_admin` | `handleClaimAdmin()` | No |
| `/status` | `handleStatus()` | No |
| `/verify <code>` | `handleVerify()` | No |
| `/approve <id>` | `handleApprove()` | No |
| `/reject <id> [reason]` | `handleReject()` | No |
| `/help` | `handleHelp()` | No |
| `!agent skills <id>` | `handleAgentSkills()` | No |
| `!agent forget-skill <id> <skill>` | `handleAgentForgetSkill()` | No |

### Handler 2: `studio.CommandHandler` — pkg/studio/commands.go

| Aspect | Detail |
|--------|--------|
| **Type** | `*CommandHandler` (struct in studio package) |
| **Storage** | Would be set via `SetStudioCommandHandler()` — **never called** |
| **Interface** | Implements `StudioCommandHandler` interface |
| **Response type** | `m.text` via `SendMessage()` |

**Commands it would handle:**

| Command | Handler Method | Routed? |
|---------|---------------|---------|
| `!agent help` | `handleHelp()` | No |
| `!agent list-skills` / `!agent skills` | `handleListSkills()` | No |
| `!agent list-pii` / `!agent pii` | `handleListPII()` | No |
| `!agent list-profiles` / `!agent profiles` | `handleListProfiles()` | No |
| `!agent create name="X"` | `handleCreateStart()` → wizard | No |
| `!agent list` / `!agent ls` | `handleListAgents()` | No |
| `!agent show <id>` / `!agent get <id>` | `handleShowAgent()` | No |
| `!agent spawn <id>` / `!agent run <id>` | `handleSpawnAgent()` | No |
| `!agent stop <id>` | `handleStopInstance()` | No |
| `!agent delete <id>` / `!agent rm <id>` | `handleDeleteAgent()` | No |
| `!agent stats` | `handleStats()` | No |

### Handler 3: `secretary.SecretaryCommandHandler` — pkg/secretary/secretary_commands.go

| Aspect | Detail |
|--------|--------|
| **Type** | `*SecretaryCommandHandler` (struct in secretary package) |
| **Storage** | Would be set via `SetStudioCommandHandler()` — **never called** |
| **Interface** | Implements `StudioCommandHandler` interface |
| **Response type** | Via `matrixAdapterWrapper.SendMessage()` → `m.text` |

**Commands it would handle:**

| Command | Handler Method | Routed? |
|---------|---------------|---------|
| `!secretary help` | `sendHelp()` | No |
| `!secretary create workflow <id>` | `handleCreateWorkflow()` | No |
| `!secretary start workflow <id>` | `handleStartWorkflow()` | No |
| `!secretary list workflows` | `handleListWorkflows()` | No |
| `!secretary list agents` | `handleListAgents()` | No |
| `!secretary list templates` | `handleListTemplates()` | No |
| `!secretary list trust` | `handleListTrustPolicies()` | No |
| `!secretary workflow status <id>` | `handleWorkflowStatus()` | No |
| `!secretary workflow cancel <id>` | `handleCancelWorkflow()` | No |
| `!secretary delete agent <id>` | `handleDeleteAgent()` | No |
| `!secretary learn website <url>` | `handleLearnWebsite()` | No |
| `!secretary review mapping <id>` | `handleReviewMapping()` | No |
| `!secretary confirm mapping <id>` | `handleConfirmMapping()` | No |
| `!secretary run blindfill <tpl>` | `handleRunBlindFill()` | No |
| `!secretary trust list` | `handleListTrustPolicies()` | No |
| `!secretary trust create <name>` | `handleCreateTrustPolicy()` | No |
| `!secretary trust revoke <id>` | `handleRevokeTrustPolicy()` | No |
| `!secretary contact create <name>` | `handleCreateContact()` | No |
| `!secretary contact get <id>` | `handleGetContact()` | No |
| `!secretary contact list` | `handleListContacts()` | No |
| `!secretary contact update <id>` | `handleUpdateContact()` | No |
| `!secretary contact delete <id>` | `handleDeleteContact()` | No |
| `!secretary contact search <q>` | `handleSearchContacts()` | No |
| `!secretary webdav list <url>` | `handleWebDAVList()` | No |
| `!secretary webdav get <url>` | `handleWebDAVGet()` | No |
| `!secretary webdav put <url> <c>` | `handleWebDAVPut()` | No |
| `!secretary webdav delete <url>` | `handleWebDAVDelete()` | No |
| `!secretary calendar list <url>` | `handleCalendarList()` | No |
| `!secretary calendar create <t>` | `handleCalendarCreate()` | No |
| `!secretary calendar get_events <u>` | `handleCalendarGetEvents()` | No |
| `!secretary calendar get_event <u>` | `handleCalendarGetEvent()` | No |
| `!secretary calendar update <u>` | `handleCalendarUpdate()` | No |
| `!secretary calendar delete <u>` | `handleCalendarDelete()` | No |

### Handler 4: `matrixcmd.CommandHandler` — pkg/matrixcmd/handler.go

| Aspect | Detail |
|--------|--------|
| **Type** | `*CommandHandler` (struct in matrixcmd package) |
| **Storage** | Not stored on MatrixAdapter at all |
| **Wiring** | Completely disconnected — no setter exists |
| **Response type** | Via injected `sendMessage` callback |

**Commands it would handle:**

| Command | Handler Method | Routed? |
|---------|---------------|---------|
| `/claim_admin [name]` | `handleClaimAdmin()` | No |
| `/status` | `handleStatus()` | No |
| `/verify <code>` | `handleVerify()` | No |
| `/approve <id>` | `handleApprove()` | No |
| `/reject <id> [reason]` | `handleReject()` | No |
| `/help` | `handleHelp()` | No |
| `/ai` | `handleAI()` | No |
| `/ai providers` | `handleAI()` → providers | No |
| `/ai models <provider>` | `handleAI()` → models | No |
| `/ai switch <p> <m>` | `handleAI()` → switch | No |
| `/ai status` | `handleAI()` → status | No |

---

## Combined Command Status Table

| # | Command | Handler Implementation | Routed in processEvents? | m.notice | Status | T0 Priority |
|---|---------|----------------------|--------------------------|----------|--------|-------------|
| 1 | `/claim_admin` | adapter.CommandHandler, matrixcmd.CommandHandler | No | Yes (adapter) / No (matrixcmd) | **missing** | HIGH |
| 2 | `/status` | adapter.CommandHandler, matrixcmd.CommandHandler | No | Yes (adapter) / No (matrixcmd) | **missing** | HIGH |
| 3 | `/verify <code>` | adapter.CommandHandler, matrixcmd.CommandHandler | No | Yes (adapter) / No (matrixcmd) | **missing** | HIGH |
| 4 | `/approve <id>` | adapter.CommandHandler, matrixcmd.CommandHandler | No | Yes (adapter) / No (matrixcmd) | **missing** | HIGH |
| 5 | `/reject <id> [reason]` | adapter.CommandHandler, matrixcmd.CommandHandler | No | Yes (adapter) / No (matrixcmd) | **missing** | HIGH |
| 6 | `/help` | adapter.CommandHandler, matrixcmd.CommandHandler | No | Yes (adapter) / No (matrixcmd) | **missing** | HIGH |
| 7 | `/ai` | matrixcmd.CommandHandler | No | No | **missing** | HIGH |
| 8 | `/ai providers` | matrixcmd.CommandHandler | No | No | **missing** | HIGH |
| 9 | `/ai models <p>` | matrixcmd.CommandHandler | No | No | **missing** | HIGH |
| 10 | `/ai switch <p> <m>` | matrixcmd.CommandHandler | No | No | **missing** | HIGH |
| 11 | `/ai status` | matrixcmd.CommandHandler | No | No | **missing** | HIGH |
| 12 | `!agent help` | studio.CommandHandler | No | No (m.text) | **missing** | HIGH |
| 13 | `!agent skills` | adapter.CommandHandler, studio.CommandHandler | No | adapter: yes / studio: no | **missing** | HIGH |
| 14 | `!agent forget-skill` | adapter.CommandHandler | No | Yes | **missing** | MEDIUM |
| 15 | `!agent list-skills` | studio.CommandHandler | No | No | **missing** | MEDIUM |
| 16 | `!agent list-pii` | studio.CommandHandler | No | No | **missing** | LOW |
| 17 | `!agent list-profiles` | studio.CommandHandler | No | No | **missing** | LOW |
| 18 | `!agent create` | studio.CommandHandler | No | No | **missing** | HIGH |
| 19 | `!agent list` | studio.CommandHandler | No | No | **missing** | HIGH |
| 20 | `!agent show` | studio.CommandHandler | No | No | **missing** | MEDIUM |
| 21 | `!agent spawn` | studio.CommandHandler | No | No | **missing** | HIGH |
| 22 | `!agent stop` | studio.CommandHandler | No | No | **missing** | HIGH |
| 23 | `!agent delete` | studio.CommandHandler | No | No | **missing** | MEDIUM |
| 24 | `!agent stats` | studio.CommandHandler | No | No | **missing** | LOW |
| 25 | `!secretary help` | secretary.SecretaryCommandHandler | No | No | **missing** | HIGH |
| 26 | `!secretary list workflows` | secretary.SecretaryCommandHandler | No | No | **missing** | HIGH |
| 27 | `!secretary create workflow` | secretary.SecretaryCommandHandler | No | No | **missing** | HIGH |
| 28 | `!secretary start workflow` | secretary.SecretaryCommandHandler | No | No | **missing** | HIGH |
| 29 | `!secretary workflow status` | secretary.SecretaryCommandHandler | No | No | **missing** | MEDIUM |
| 30 | `!secretary workflow cancel` | secretary.SecretaryCommandHandler | No | No | **missing** | MEDIUM |
| 31 | `!secretary list agents` | secretary.SecretaryCommandHandler | No | No | **missing** | MEDIUM |
| 32 | `!secretary delete agent` | secretary.SecretaryCommandHandler | No | No | **missing** | LOW |
| 33 | `!secretary list templates` | secretary.SecretaryCommandHandler | No | No | **missing** | MEDIUM |
| 34 | `!secretary learn website` | secretary.SecretaryCommandHandler | No | No | **missing** | MEDIUM |
| 35 | `!secretary review mapping` | secretary.SecretaryCommandHandler | No | No | **missing** | LOW |
| 36 | `!secretary confirm mapping` | secretary.SecretaryCommandHandler | No | No | **missing** | LOW |
| 37 | `!secretary run blindfill` | secretary.SecretaryCommandHandler | No | No | **missing** | MEDIUM |
| 38 | `!secretary trust list` | secretary.SecretaryCommandHandler | No | No | **missing** | MEDIUM |
| 39 | `!secretary trust create` | secretary.SecretaryCommandHandler | No | No | **missing** | LOW |
| 40 | `!secretary trust revoke` | secretary.SecretaryCommandHandler | No | No | **missing** | LOW |
| 41 | `!secretary contact *` | secretary.SecretaryCommandHandler | No | No | **missing** | LOW |
| 42 | `!secretary webdav *` | secretary.SecretaryCommandHandler | No | No | **missing** | LOW |
| 43 | `!secretary calendar *` | secretary.SecretaryCommandHandler | No | No | **missing** | LOW |

---

## Structural Issues

1. **No handler registration** — `SetStudioCommandHandler()` and `SetCommandHandler()` are defined but never called from production wiring code. Both `studioCmdHandler` and `commandHandler` are always `nil`.

2. **Duplicate implementations** — Admin commands (`/claim_admin`, `/status`, `/verify`, `/approve`, `/reject`, `/help`) are implemented identically in both `commands_integration.go` and `matrixcmd/handler.go`. Neither is wired.

3. **`!agent skills` collision** — Handled by both `adapter.CommandHandler` (for learned skills) and `studio.CommandHandler` (for studio skills). Different semantics, same prefix.

4. **Single-slot interface** — `StudioCommandHandler` is a single interface field. Only one handler can occupy it. Studio (`!agent`) and Secretary (`!secretary`) both implement this interface but cannot coexist without a multiplexer.

5. **`/ai` commands orphaned** — `matrixcmd.CommandHandler` handles `/ai` but has no setter on MatrixAdapter and is not wired to any routing point.

---

## T0 Scope: Commands to Wire

**Phase T0 must:**

1. **Register handlers** — Wire at minimum one handler to `studioCmdHandler` via `SetStudioCommandHandler()` call in bridge initialization code
2. **Multiplex routing** — ProcessEvents must dispatch to the correct handler based on prefix (`!agent` → studio, `!secretary` → secretary, `/` → admin)
3. **Wire admin commands** — Connect `/claim_admin`, `/status`, `/verify`, `/approve`, `/reject`, `/help` to processEvents routing
4. **Wire `/ai` commands** — Connect `/ai providers|models|switch|status` to processEvents routing
5. **Ensure m.notice responses** — All command responses should use `m.notice` (currently studio uses `m.text`)

### Minimum Viable T0 Command Set (HIGH priority):

- `/claim_admin`, `/status`, `/verify`, `/approve`, `/reject`, `/help`
- `/ai` (and subcommands)
- `!agent create`, `!agent list`, `!agent spawn`, `!agent stop`, `!agent skills`
- `!secretary help`, `!secretary list workflows`, `!secretary create workflow`, `!secretary start workflow`

---

## Source Files

| File | Role |
|------|------|
| `bridge/internal/adapter/matrix.go` | processEvents() routing, MatrixAdapter struct |
| `bridge/internal/adapter/commands_integration.go` | adapter.CommandHandler (admin + !agent skills) |
| `bridge/pkg/studio/commands.go` | studio.CommandHandler (!agent CRUD) |
| `bridge/pkg/secretary/secretary_commands.go` | secretary.SecretaryCommandHandler (!secretary) |
| `bridge/pkg/matrixcmd/handler.go` | matrixcmd.CommandHandler (admin + /ai) |
