# F3: Integration QA

**Date**: 2026-04-18
**Auditor**: Atlas (Orchestrator)

## Scenarios Tested

### Broker + Teams Integration
- TestBrokerWithTeamRegistry: Broker uses TeamCapabilityRegistry for role-based capability lookup ✅
- TestTeamCapabilityRegistry_GetCapabilities_KnownRole: Role→caps mapping works ✅
- TestTeamCapabilityRegistry_GetCapabilities_UnknownRole: Unknown role → empty capabilities ✅

### Broker Fail-Closed
- TestBrokerAuthorize_FailClosed_NilClassifier: Nil classifier → DENY ✅
- TestBrokerAuthorize_FailClosed_Panic: Panic → DENY ✅
- TestBrokerAuthorize_FailClosed_NilRegistry: Nil registry → DENY ✅
- TestBrokerAuthorize_QueueFull: Queue full → DENY ✅
- TestBrokerAuthorize_CircularDependency: Circular deps → DENY ✅
- TestBrokerAuthorize_ContextCancelled: Context cancel → DENY ✅
- TestBrokerAuthorize_InvalidInput: Invalid input → DENY ✅

### Broker Risk Classification
- TestBrokerAuthorize_Allow: ALLOW flow ✅
- TestBrokerAuthorize_DenyCapability: DENY capability ✅
- TestBrokerAuthorize_DenyRisk: DENY risk ✅
- TestBrokerAuthorize_DeferConsentGranted: DEFER → consent granted → ALLOW ✅
- TestBrokerAuthorize_DeferConsentDenied: DEFER → consent denied → DENY ✅
- TestBrokerAuthorize_DeferTimeout: DEFER → timeout → DENY ✅

### Team Lifecycle
- TestService_CreateTeam / GetTeam / ListTeams / DissolveTeam ✅
- TestService_AddMember / RemoveMember / AssignRole ✅
- TestService_RemoveMember_AutoDissolve: Last member removal → auto-dissolve ✅
- TestService_CreateTeam_WithCollectionCreator: Qdrant collection created ✅

### Secret Request Flow
- TestRequestSecret_Approved / Denied / Timeout ✅
- TestNewSecretHandler_Approval / Denial / Missing fields ✅

### Team Composition
- TestCompose_ValidGoal / InvalidRole / CircularDependency / EmptyGoal ✅
- TestExecute_SimplePlan / ParallelSubtasks / StepFailure_Retry / AllRetriesFail ✅
- TestExecute_TopologicalSort_Cycle / Valid ✅

### Edge Cases
- Zero capabilities → all actions DENY (TestBrokerAuthorize_DenyCapability) ✅
- Team dissolution → store prevents new members (TestAddMember_DissolvedTeam) ✅
- Broker crash → DENY (TestBrokerAuthorize_FailClosed_Panic) ✅
- Role escalation → DENY (TestBrowserSpecialist_CannotSendEmail) ✅
- Circular deps → DENY (TestCompose_CircularDependency, TestBrokerAuthorize_CircularDependency) ✅
- Concurrent team creation (TestConcurrentCreateTeams) ✅

## Rust Vault
- cargo check --lib: Compiles ✅ (1 pre-existing warning)

## License Server
- 8/8 tests PASS: Team limits enforced per tier (Free/Pro/Enterprise) ✅

## VERDICT: ✅ APPROVE
