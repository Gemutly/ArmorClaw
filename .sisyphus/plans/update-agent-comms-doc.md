# Plan: Update Agent Communication Architecture Docs

**Created**: 2026-04-14
**Status**: Active
**Scope**: Documentation only — single file edit

## Context

Investigation of the Agent → Bridge communication channel revealed that the existing `### Bridge ↔ OpenClaw Agents` section in `doc/armorclaw.md` (lines 1500-1511) is inaccurate. It references `AgentFactory` under `orchestrator.go` and describes a "Factory interface with container lifecycle management" without explaining the actual data flow limitation: containers run with `NetworkMode: "none"`, have zero network access, and communicate only via exit code. No structured results flow back.

## TODOs

- [x] T1: Replace `### Bridge ↔ OpenClaw Agents` section with accurate architecture documentation

### Acceptance Criteria

- Section accurately describes env-var-only injection, `NetworkMode: "none"`, exit-code polling via `waitForCompletion`
- ASCII diagram shows the actual Bridge → Container data flow
- State directory bind-mount is documented
- Data flow limitation (no structured results) is clearly stated
- Task Scheduler paragraph preserved (unchanged)

## Final Verification Wave

- [x] F1: Read updated section, verify accuracy against source code
