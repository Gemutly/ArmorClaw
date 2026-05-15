# Matrix Command Coverage Matrix

> Generated: 2026-05-16 | Bridge: v4.6.0 | VPS: 5.183.11.149

## Overview

This document maps the Matrix-related RPC commands available in ArmorClaw v4.6.0,
their expected behavior, observed responses, and test coverage status.

## Core Matrix Commands

| Command | Subsystem | Expected Effect | Expected Event | Probed Status | Tested |
|---------|-----------|-----------------|----------------|---------------|--------|
| `matrix.status` | Matrix | Returns connection state | None | ✅ Works (object) | ✅ T7 contract |
| `matrix.login` | Matrix | Authenticates bridge user | None | ⚠️ -32602 (needs params) | ⬜ T10 |
| `matrix.send` | Matrix | Sends message to room | message event | ⚠️ -32602 (needs params) | ⬜ T10 |
| `matrix.receive` | Matrix | Receives/polls messages | None | ⚠️ -32602 (needs params) | ⬜ T10 |
| `matrix.join_room` | Matrix | Joins a Matrix room | room join event | ⚠️ -32602 (needs params) | ⬜ T10 |

## Event Bus Commands

| Command | Subsystem | Expected Effect | Expected Event | Probed Status | Tested |
|---------|-----------|-----------------|----------------|---------------|--------|
| `events.replay` | EventBus | Replays past events | Event stream | ⚠️ -32603 (internal error) | ⬜ T8 |
| `events.stream` | EventBus | Streams live events via WebSocket | Live events | ⚠️ -32603 (internal error) | ⬜ T8 |

## Studio Commands (Agent Lifecycle)

| Command | Subsystem | Expected Effect | Expected Event | Probed Status | Tested |
|---------|-----------|-----------------|----------------|---------------|--------|
| `studio.list_agents` | Studio | Lists all agents | None | ✅ Works (object) | ✅ Baseline |
| `studio.create_agent` | Studio | Creates new agent | agent_created | ✅ Works (object) | ⬜ T12 |
| `studio.delete_agent` | Studio | Deletes agent | agent_deleted | ✅ Works (object) | ⬜ T12 |
| `studio.get_agent` | Studio | Gets agent details | None | ✅ Registered | ⬜ T12 |
| `studio.spawn_agent` | Studio | Spawns agent instance | instance_started | ✅ Registered | ⬜ T12 |
| `studio.list_instances` | Studio | Lists running instances | None | ✅ Registered | ⬜ T12 |
| `studio.stop_instance` | Studio | Stops instance | instance_stopped | ✅ Registered | ⬜ T12 |
| `studio.stats` | Studio | Studio statistics | None | ✅ Registered | ⬜ T12 |

## Secretary Commands (Workflow Engine)

| Command | Subsystem | Expected Effect | Expected Event | Probed Status | Tested |
|---------|-----------|-----------------|----------------|---------------|--------|
| `secretary.list_templates` | Secretary | Lists workflow templates | None | ✅ Works (object) | ⬜ T13 |
| `secretary.start_workflow` | Secretary | Starts workflow from template | workflow_started | ⚠️ -32602 (needs params) | ⬜ T13 |
| `secretary.get_workflow` | Secretary | Gets workflow status | None | ✅ Registered | ⬜ T13 |
| `secretary.cancel_workflow` | Secretary | Cancels running workflow | workflow_cancelled | ✅ Registered | ⬜ T13 |
| `secretary.create_template` | Secretary | Creates workflow template | None | ✅ Registered | ⬜ T13 |
| `secretary.get_template` | Secretary | Gets template details | None | ✅ Registered | ⬜ T13 |
| `secretary.delete_template` | Secretary | Deletes template | None | ✅ Registered | ⬜ T13 |
| `secretary.is_running` | Secretary | Check if engine running | None | ✅ Registered | ⬜ T13 |
| `task.create` | Task | Creates a task | task_created | ⚠️ -32602 (needs params) | ⬜ T13 |
| `task.list` | Task | Lists tasks | None | ⚠️ -32602 (needs params) | ⬜ T13 |
| `task.cancel` | Task | Cancels a task | task_cancelled | ✅ Registered | ⬜ T13 |
| `task.get` | Task | Gets task details | None | ✅ Registered | ⬜ T13 |

## Infrastructure Commands

| Command | Subsystem | Expected Effect | Expected Event | Probed Status | Tested |
|---------|-----------|-----------------|----------------|---------------|--------|
| `bridge.status` | Bridge | Bridge health/state | None | ✅ Works (object) | ✅ Baseline |
| `health.check` | Health | Full health check | None | ✅ Works (object) | ✅ Baseline |

## Status Legend

- ✅ Works — Returns result (object/array)
- ⚠️ -32602 — Invalid params (expected — needs required parameters)
- ⚠️ -32603 — Internal error (handler exists but failed without params)
- ✅ Registered — Handler registered (not probed without params)
- ⬜ — Not yet tested (will be covered by referenced task)

## Coverage Summary

| Category | Total | Probed Working | Needs Params | Not Probed | Tested |
|----------|-------|---------------|--------------|------------|--------|
| Matrix Core | 5 | 1 | 4 | 0 | 1 |
| Event Bus | 2 | 0 | 0 | 2 | 0 |
| Studio | 8 | 3 | 0 | 5 | 1 |
| Secretary | 12 | 1 | 3 | 8 | 0 |
| Infrastructure | 2 | 2 | 0 | 0 | 2 |
| **Total** | **29** | **7** | **7** | **15** | **4** |

## Error Code Reference

| Code | Meaning | Typical Cause |
|------|---------|---------------|
| -32600 | Invalid Request | Malformed JSON |
| -32601 | Method not found | Unknown RPC method |
| -32602 | Invalid params | Missing required parameters |
| -32603 | Internal error | Handler threw exception |
| -32001 | Unauthorized | Auth required but missing/invalid |

## Source References

- Handler registration: `bridge/pkg/rpc/server.go:1219-1350`
- Matrix handlers: `bridge/pkg/rpc/server.go:480-520`
- Studio handler: `bridge/pkg/rpc/server.go:1273-1297` (routes through `handleStudio`)
- Secretary handler: `bridge/pkg/rpc/server.go:1314-1330` (routes through `handleSecretaryMethod`)
