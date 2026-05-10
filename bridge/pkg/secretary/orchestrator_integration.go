package secretary

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/url"
	"os"
	"regexp"
	"sync"
	"time"

	"github.com/armorclaw/bridge/internal/events"
	"github.com/armorclaw/bridge/pkg/agent"
	"github.com/armorclaw/bridge/pkg/browser"
	"github.com/armorclaw/bridge/pkg/capability"
	"github.com/armorclaw/bridge/pkg/interfaces"
	"github.com/armorclaw/bridge/pkg/studio"
)

// stepDataRefRegex matches {{steps.<step_id>.data.<key>}} template references.
var stepDataRefRegex = regexp.MustCompile(`\{\{steps\.([^.}]+)\.data\.([^.}]+)\}\}`)

// resolveStepInput replaces {{steps.step_id.data.key}} references in the
// step's Input map using accumulated results from prior steps.
// Returns an error if a reference points to a step that hasn't produced data.
func resolveStepInput(input map[string]any, accumulatedData map[string]map[string]any) (map[string]any, error) {
	if len(input) == 0 {
		return nil, nil
	}

	resolved := make(map[string]any, len(input))
	for k, v := range input {
		switch val := v.(type) {
		case string:
			resolved[k] = resolveTemplateString(val, accumulatedData)
		default:
			resolved[k] = v
		}
	}
	return resolved, nil
}

// resolveTemplateString replaces all {{steps.X.data.Y}} references in a string.
func resolveTemplateString(s string, accumulatedData map[string]map[string]any) string {
	return stepDataRefRegex.ReplaceAllStringFunc(s, func(match string) string {
		sub := stepDataRefRegex.FindStringSubmatch(match)
		if len(sub) < 3 {
			return match
		}
		stepID, key := sub[1], sub[2]
		stepData, ok := accumulatedData[stepID]
		if !ok {
			return match
		}
		val, ok := stepData[key]
		if !ok {
			return match
		}
		return fmt.Sprintf("%v", val)
	})
}

// injectPrevStepData merges resolved input into the step's Config JSON as
// _prev_step_data. If step.Config is nil, a new JSON object is created.
func injectPrevStepData(config json.RawMessage, input map[string]any) json.RawMessage {
	if len(input) == 0 {
		return config
	}

	var configMap map[string]interface{}
	if config != nil {
		if err := json.Unmarshal(config, &configMap); err != nil {
			configMap = make(map[string]interface{})
		}
	} else {
		configMap = make(map[string]interface{})
	}

	configMap["_prev_step_data"] = input

	updated, err := json.Marshal(configMap)
	if err != nil {
		return config
	}
	return updated
}

// LearnedSkillInfo is a minimal interface for injecting learned skill suggestions
// into step config. Avoids importing pkg/skills directly (would create import cycle).
type LearnedSkillInfo struct {
	Name         string
	Confidence   float64
	PatternType  string
	SourceTaskID string
}

// SkillFinder finds relevant learned skills for a task description.
type SkillFinder interface {
	FindForTask(taskDesc string, limit int) ([]LearnedSkillInfo, error)
}

// ChartFinder finds relevant navigation charts for a browser domain.
// Mirrors SkillFinder but targets browser-execute steps.
type ChartFinder interface {
	FindForDomain(ctx context.Context, domain string, limit int) ([]browser.ChartRecord, error)
}

//=============================================================================
// Step Execution Errors
//=============================================================================

var (
	ErrNoAgentForStep        = errors.New("no agent available for step")
	ErrStepExecutionFailed   = errors.New("step execution failed")
	ErrAgentSpawnFailed      = errors.New("failed to spawn agent")
	ErrAgentTimeout          = errors.New("agent execution timeout")
	ErrWorkflowNotRunning    = errors.New("workflow not running")
	ErrInvalidExecutionOrder = errors.New("invalid execution order")
	ErrAllAgentsFailed       = errors.New("all agents failed for step")
)

type StepExecutionError struct {
	StepID      string
	AgentID     string
	Err         error
	Recoverable bool
}

func (e *StepExecutionError) Error() string {
	if e.AgentID != "" {
		return fmt.Sprintf("%v: step %s (agent %s)", e.Err, e.StepID, e.AgentID)
	}
	return fmt.Sprintf("%v: step %s", e.Err, e.StepID)
}

func (e *StepExecutionError) Unwrap() error {
	return e.Err
}

//=============================================================================
// Step Executor
//=============================================================================

type ApprovalChecker interface {
	EvaluateStep(ctx context.Context, workflow *Workflow, template *TaskTemplate, step *WorkflowStep, piiFields []string, initiator string) (*ApprovalResult, error)
}

type StepExecutorConfig struct {
	Factory          *studio.AgentFactory
	Validator        *DependencyValidator
	ApprovalEngine   ApprovalChecker
	ApprovalManager  *PendingApprovalManager
	EventBus         *events.MatrixEventBus
	Broker           interfaces.CapabilityBroker // inserted between step start and agent dispatch; nil = all steps execute (backward compatible)
	BridgeLocal      *BridgeLocalRegistry        // if non-nil, bridge_local steps dispatch here instead of container spawn
	DefaultTimeout   time.Duration
	StepRetryCount   int
	StepRetryDelay   time.Duration
	StateDirBase     string
	SkillFinder      SkillFinder
	ChartFinder      ChartFinder
	FailoverPolicy   FailoverPolicy
	ParallelConfig   ParallelConfig

	OnSkillExtraction func(result *ExtendedStepResult, taskDesc, taskID, templateID string)
	OnSkillOutcome    func(skillID string, success bool) error
	OnChartOutcome    func(chartID string, success bool) error
}

type FailoverAttempt struct {
	AgentID string
	Err     error
}

type FailoverAggregatedError struct {
	StepID    string
	Attempts  []FailoverAttempt
	LastAgent string
}

func (e *FailoverAggregatedError) Error() string {
	var parts []string
	for _, a := range e.Attempts {
		parts = append(parts, fmt.Sprintf("agent %s: %v", a.AgentID, a.Err))
	}
	return fmt.Sprintf("%s: step %s [%s]", ErrAllAgentsFailed, e.StepID, joinStrings(parts, "; "))
}

func (e *FailoverAggregatedError) Unwrap() error {
	return ErrAllAgentsFailed
}

func joinStrings(ss []string, sep string) string {
	if len(ss) == 0 {
		return ""
	}
	result := ss[0]
	for i := 1; i < len(ss); i++ {
		result += sep + ss[i]
	}
	return result
}

type StepExecutor struct {
	factory          *studio.AgentFactory
	validator        *DependencyValidator
	approvalEngine   ApprovalChecker
	approvalManager  *PendingApprovalManager
	eventBus         *events.MatrixEventBus
	broker           interfaces.CapabilityBroker
	bridgeLocal      *BridgeLocalRegistry
	defaultTimeout   time.Duration
	retryCount       int
	retryDelay       time.Duration
	stateDirBase     string
	skillFinder      SkillFinder
	chartFinder      ChartFinder
	failoverPolicy   FailoverPolicy
	parallelExec     *ParallelExecutor

	onSkillExtraction func(result *ExtendedStepResult, taskDesc, taskID, templateID string)
	onSkillOutcome    func(skillID string, success bool) error
	onChartOutcome    func(chartID string, success bool) error

	mu           sync.RWMutex
	runningSteps map[string]*runningStep
}

type runningStep struct {
	workflowID string
	stepID     string
	instanceID string
	startedAt  time.Time
	cancelFunc context.CancelFunc
}

func NewStepExecutor(cfg StepExecutorConfig) *StepExecutor {
	if cfg.DefaultTimeout == 0 {
		cfg.DefaultTimeout = 5 * time.Minute
	}
	if cfg.StepRetryCount == 0 {
		cfg.StepRetryCount = 1
	}
	if cfg.StepRetryDelay == 0 {
		cfg.StepRetryDelay = 1 * time.Second
	}
	if cfg.StateDirBase == "" {
		cfg.StateDirBase = "/var/lib/armorclaw/agent-state"
	}
	if cfg.FailoverPolicy == "" {
		cfg.FailoverPolicy = FailoverRetry
	}

	return &StepExecutor{
		factory:           cfg.Factory,
		validator:         cfg.Validator,
		approvalEngine:    cfg.ApprovalEngine,
		approvalManager:   cfg.ApprovalManager,
		eventBus:          cfg.EventBus,
		broker:            cfg.Broker,
		bridgeLocal:       cfg.BridgeLocal,
		defaultTimeout:    cfg.DefaultTimeout,
		retryCount:        cfg.StepRetryCount,
		retryDelay:        cfg.StepRetryDelay,
		stateDirBase:      cfg.StateDirBase,
		skillFinder:       cfg.SkillFinder,
		chartFinder:       cfg.ChartFinder,
		failoverPolicy:    cfg.FailoverPolicy,
		onSkillExtraction: cfg.OnSkillExtraction,
		onSkillOutcome:    cfg.OnSkillOutcome,
		onChartOutcome:    cfg.OnChartOutcome,
		runningSteps:      make(map[string]*runningStep),
	}
}

func (e *StepExecutor) initParallelExecutor() {
	if e.parallelExec == nil {
		e.parallelExec = NewParallelExecutor(e, DefaultParallelConfig())
	}
}

func (e *StepExecutor) mergeParallelGroupData(workflow *Workflow, splitStepID string, parResult *ParallelResult) {
	merged := make(map[string]any)
	for stepID, sr := range parResult.BranchResults {
		if sr != nil && sr.ContainerResult != nil && sr.ContainerResult.Data != nil {
			merged[stepID] = sr.ContainerResult.Data
		}
	}
	if len(merged) > 0 {
		if workflow.StepsData == nil {
			workflow.StepsData = make(map[string]any)
		}
		workflow.StepsData[splitStepID] = merged
	}
}

func (e *StepExecutor) mergeStandaloneParallelData(workflow *Workflow, stepID string, parResult *ParallelResult) {
	merged := make(map[string]any)
	for branchID, sr := range parResult.BranchResults {
		if sr != nil && sr.ContainerResult != nil && sr.ContainerResult.Data != nil {
			merged[branchID] = sr.ContainerResult.Data
		}
	}
	if len(merged) > 0 {
		if workflow.StepsData == nil {
			workflow.StepsData = make(map[string]any)
		}
		workflow.StepsData[stepID] = merged
	}
}

func (e *StepExecutor) agentStateDir(agentID string) string {
	return fmt.Sprintf("%s/%s", e.stateDirBase, agentID)
}

func (e *StepExecutor) ExecuteSteps(
	ctx context.Context,
	orchestrator *WorkflowOrchestratorImpl,
	workflow *Workflow,
	template *TaskTemplate,
) error {
	if template == nil || len(template.Steps) == 0 {
		return nil
	}

	validation := e.validator.ValidateTemplate(template)
	if !validation.Valid {
		if len(validation.Errors) > 0 {
			return fmt.Errorf("%w: %v", ErrInvalidExecutionOrder, validation.Errors[0])
		}
		return ErrInvalidExecutionOrder
	}

	stepMap := make(map[string]WorkflowStep)
	for _, step := range template.Steps {
		stepMap[step.StepID] = step
	}

	if HasParallelSteps(template.Steps) {
		return e.executeWithParallel(ctx, orchestrator, workflow, template, stepMap, validation.ExecutionOrder)
	}

	return e.executeSequential(ctx, orchestrator, workflow, template, stepMap, validation.ExecutionOrder)
}

func (e *StepExecutor) executeSequential(
	ctx context.Context,
	orchestrator *WorkflowOrchestratorImpl,
	workflow *Workflow,
	template *TaskTemplate,
	stepMap map[string]WorkflowStep,
	executionOrder []string,
) error {
	accumulatedData := make(map[string]map[string]any)

	for i, stepID := range executionOrder {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		step, exists := stepMap[stepID]
		if !exists {
			return fmt.Errorf("%w: step %s not found", ErrInvalidExecutionOrder, stepID)
		}

		if len(step.Input) > 0 {
			resolved, err := resolveStepInput(step.Input, accumulatedData)
			if err != nil {
				return fmt.Errorf("step %s: %w", stepID, err)
			}
			step.Config = injectPrevStepData(step.Config, resolved)
		}

		progress := float64(i) / float64(len(executionOrder))
		orchestrator.UpdateProgress(workflow.ID, stepID, progress)

		if err := e.checkApproval(ctx, workflow, template, &step, stepID); err != nil {
			return err
		}

		result := e.executeStepWithRetry(ctx, workflow, step)

		if result.ContainerResult != nil && len(result.ContainerResult.Data) > 0 {
			accumulatedData[stepID] = result.ContainerResult.Data
		}

		if result.Err != nil {
			if !result.Recoverable {
				return &StepExecutionError{
					StepID:      stepID,
					AgentID:     result.AgentID,
					Err:         result.Err,
					Recoverable: result.Recoverable,
				}
			}
		}

		if err := orchestrator.AdvanceWorkflow(workflow.ID, stepID); err != nil {
			if errors.Is(err, fmt.Errorf("all steps completed")) ||
				(err != nil && err.Error() == "all steps completed") {
				return nil
			}
			return fmt.Errorf("failed to advance workflow: %w", err)
		}
	}

	return nil
}

func (e *StepExecutor) executeWithParallel(
	ctx context.Context,
	orchestrator *WorkflowOrchestratorImpl,
	workflow *Workflow,
	template *TaskTemplate,
	stepMap map[string]WorkflowStep,
	executionOrder []string,
) error {
	e.initParallelExecutor()

	groups := IdentifyParallelGroups(template.Steps)
	groupIdx := BuildParallelGroupIndex(groups)
	stepToGroup := BuildStepToGroupMap(groups)

	skippedSteps := make(map[string]bool)

	for i, stepID := range executionOrder {
		if skippedSteps[stepID] {
			continue
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		step, exists := stepMap[stepID]
		if !exists {
			return fmt.Errorf("%w: step %s not found", ErrInvalidExecutionOrder, stepID)
		}

		switch step.Type {
		case StepParallel:
			if err := e.handleStandaloneParallel(ctx, orchestrator, workflow, template, stepMap, executionOrder, i, step); err != nil {
				return err
			}

		case StepParallelSplit:
			group, hasGroup := groupIdx[stepID]
			if !hasGroup {
				return fmt.Errorf("parallel_split step %s has no matching parallel_merge", stepID)
			}
			if err := e.handleParallelGroup(ctx, orchestrator, workflow, template, stepMap, executionOrder, i, group, skippedSteps); err != nil {
				return err
			}

		case StepParallelMerge:
			// Merge steps are executed as synchronization barriers after the split handler.
			// If we encounter a merge here, it means the split handler already processed it.
			// Skip it to avoid double execution.
			continue

		default:
			if err := e.executeSequentialStep(ctx, orchestrator, workflow, template, stepMap, executionOrder, i, step, stepToGroup); err != nil {
				return err
			}
		}
	}

	return nil
}

func (e *StepExecutor) checkApproval(
	ctx context.Context,
	workflow *Workflow,
	template *TaskTemplate,
	step *WorkflowStep,
	stepID string,
) error {
	if e.approvalEngine == nil || len(template.PIIRefs) == 0 {
		return nil
	}

	approvalResult, err := e.approvalEngine.EvaluateStep(ctx, workflow, template, step, template.PIIRefs, workflow.CreatedBy)
	if err != nil {
		return fmt.Errorf("approval evaluation failed for step %s: %w", stepID, err)
	}

	if approvalResult.Required && !approvalResult.Approved {
		if approvalResult.NeedsApproval {
			if e.eventBus == nil {
				return fmt.Errorf("step %s requires PII approval but no event bus configured", stepID)
			}
			mgr := e.approvalManager
			if mgr == nil {
				mgr = defaultManager
			}
			approvedFields, err := mgr.RequestApproval(ctx, e.eventBus, workflow.RoomID, stepID, approvalResult.DeniedFields)
			if err != nil {
				return fmt.Errorf("PII approval failed for step %s: %w", stepID, err)
			}
			_ = approvedFields
		} else if len(approvalResult.DeniedFields) > 0 {
			return fmt.Errorf("step %s denied: fields %v blocked by policy", stepID, approvalResult.DeniedFields)
		}
	}

	return nil
}

func (e *StepExecutor) executeSequentialStep(
	ctx context.Context,
	orchestrator *WorkflowOrchestratorImpl,
	workflow *Workflow,
	template *TaskTemplate,
	stepMap map[string]WorkflowStep,
	executionOrder []string,
	index int,
	step WorkflowStep,
	stepToGroup map[string]*ParallelGroup,
) error {
	stepID := step.StepID
	progress := float64(index) / float64(len(executionOrder))
	orchestrator.UpdateProgress(workflow.ID, stepID, progress)

	if err := e.checkApproval(ctx, workflow, template, &step, stepID); err != nil {
		return err
	}

	result := e.executeStepWithRetry(ctx, workflow, step)

	if result.Err != nil {
		if !result.Recoverable {
			return &StepExecutionError{
				StepID:      stepID,
				AgentID:     result.AgentID,
				Err:         result.Err,
				Recoverable: result.Recoverable,
			}
		}
	}

	if err := orchestrator.AdvanceWorkflow(workflow.ID, stepID); err != nil {
		if errors.Is(err, fmt.Errorf("all steps completed")) ||
			(err != nil && err.Error() == "all steps completed") {
			return nil
		}
		return fmt.Errorf("failed to advance workflow: %w", err)
	}

	return nil
}

func (e *StepExecutor) handleStandaloneParallel(
	ctx context.Context,
	orchestrator *WorkflowOrchestratorImpl,
	workflow *Workflow,
	template *TaskTemplate,
	stepMap map[string]WorkflowStep,
	executionOrder []string,
	index int,
	step WorkflowStep,
) error {
	stepID := step.StepID
	progress := float64(index) / float64(len(executionOrder))
	orchestrator.UpdateProgress(workflow.ID, stepID, progress)

	if err := e.checkApproval(ctx, workflow, template, &step, stepID); err != nil {
		return err
	}

	parResult, err := e.parallelExec.ExecuteStandaloneParallel(ctx, workflow, step)
	if err != nil {
		return &StepExecutionError{
			StepID:      stepID,
			Err:         err,
			Recoverable: false,
		}
	}

	e.mergeStandaloneParallelData(workflow, stepID, parResult)

	if advanceErr := orchestrator.AdvanceWorkflow(workflow.ID, stepID); advanceErr != nil {
		if errors.Is(advanceErr, fmt.Errorf("all steps completed")) ||
			(advanceErr != nil && advanceErr.Error() == "all steps completed") {
			return nil
		}
		return fmt.Errorf("failed to advance workflow: %w", advanceErr)
	}

	return nil
}

func (e *StepExecutor) handleParallelGroup(
	ctx context.Context,
	orchestrator *WorkflowOrchestratorImpl,
	workflow *Workflow,
	template *TaskTemplate,
	stepMap map[string]WorkflowStep,
	executionOrder []string,
	splitIndex int,
	group *ParallelGroup,
	skippedSteps map[string]bool,
) error {
	totalSteps := len(executionOrder)
	splitProgress := float64(splitIndex) / float64(totalSteps)

	// Mark branch steps and merge step as skipped in the main loop.
	for _, branchID := range group.BranchStepIDs {
		skippedSteps[branchID] = true
	}
	skippedSteps[group.MergeStepID] = true

	progressSpan := float64(len(group.BranchStepIDs)+1) / float64(totalSteps)

	parResult, err := e.parallelExec.ExecuteParallelGroup(
		ctx, workflow, group, stepMap, splitProgress, progressSpan,
	)
	if err != nil {
		// Find first branch error for StepExecutionError wrapping.
		if len(parResult.Errors) > 0 {
			first := parResult.Errors[0]
			return &StepExecutionError{
				StepID:      first.StepID,
				AgentID:     first.AgentID,
				Err:         err,
				Recoverable: false,
			}
		}
		return err
	}

	e.mergeParallelGroupData(workflow, group.SplitStepID, parResult)

	// Advance past split step.
	if advanceErr := orchestrator.AdvanceWorkflow(workflow.ID, group.SplitStepID); advanceErr != nil {
		if errors.Is(advanceErr, fmt.Errorf("all steps completed")) ||
			(advanceErr != nil && advanceErr.Error() == "all steps completed") {
			return nil
		}
		return fmt.Errorf("failed to advance workflow past split: %w", advanceErr)
	}

	// Advance past each branch step.
	for _, branchID := range group.BranchStepIDs {
		if advanceErr := orchestrator.AdvanceWorkflow(workflow.ID, branchID); advanceErr != nil {
			if errors.Is(advanceErr, fmt.Errorf("all steps completed")) ||
				(advanceErr != nil && advanceErr.Error() == "all steps completed") {
				return nil
			}
			return fmt.Errorf("failed to advance workflow past branch %s: %w", branchID, advanceErr)
		}
	}

	// Merge step — synchronization barrier. Execute as a no-op advance.
	if advanceErr := orchestrator.AdvanceWorkflow(workflow.ID, group.MergeStepID); advanceErr != nil {
		if errors.Is(advanceErr, fmt.Errorf("all steps completed")) ||
			(advanceErr != nil && advanceErr.Error() == "all steps completed") {
			return nil
		}
		return fmt.Errorf("failed to advance workflow past merge: %w", advanceErr)
	}

	return nil
}

type CompletionResult struct {
	ExitCode        int
	ContainerResult *ContainerStepResult
	ExtendedResult  *ExtendedStepResult
}

type StepResult struct {
	StepID          string
	AgentID         string
	InstanceID      string
	Err             error
	Recoverable     bool
	ContainerResult *ContainerStepResult
}

func (e *StepExecutor) executeStepWithRetry(ctx context.Context, workflow *Workflow, step WorkflowStep) *StepResult {
	var lastErr error
	var lastAgentID string

	for attempt := 0; attempt <= e.retryCount; attempt++ {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return &StepResult{
					StepID:      step.StepID,
					Err:         ctx.Err(),
					Recoverable: false,
				}
			case <-time.After(e.retryDelay):
			}
		}

		result := e.executeStep(ctx, workflow, step)
		lastErr = result.Err
		lastAgentID = result.AgentID

		if result.Err == nil {
			return result
		}

		if !result.Recoverable {
			return result
		}
	}

	return &StepResult{
		StepID:      step.StepID,
		AgentID:     lastAgentID,
		Err:         lastErr,
		Recoverable: false,
	}
}

func (e *StepExecutor) executeStep(ctx context.Context, workflow *Workflow, step WorkflowStep) *StepResult {
	if e.broker != nil {
		req := capability.ActionRequest{
			TeamID: workflow.ID,
			Action: step.Name,
			Params: step.Input,
		}
		if len(step.AgentIDs) > 0 {
			req.AgentID = step.AgentIDs[0]
		}
		resp, err := e.broker.Authorize(ctx, req)
		if err != nil || !resp.Allowed {
			reason := "broker denied step"
			if err != nil {
				reason = fmt.Sprintf("broker error: %v", err)
			} else if resp.Reason != "" {
				reason = fmt.Sprintf("capability denied: %s", resp.Reason)
			}
			return &StepResult{
				StepID:      step.StepID,
				Err:         fmt.Errorf("%s: %s", step.StepID, reason),
				Recoverable: false,
			}
		}
	}

	if e.bridgeLocal != nil {
		stepCfg, err := ParseStepConfig(step.Config)
		if err != nil {
			return &StepResult{
				StepID:      step.StepID,
				Err:         fmt.Errorf("parse step config: %w", err),
				Recoverable: false,
			}
		}
		if stepCfg.ExecutionMode == ExecutionModeBridgeLocal {
			result := ExecuteBridgeLocal(ctx, e.bridgeLocal, step.Config)
			if result.StepID == "" {
				result.StepID = step.StepID
			}
			return result
		}
	}

	if len(step.AgentIDs) == 0 {
		return &StepResult{
			StepID:      step.StepID,
			Err:         ErrNoAgentForStep,
			Recoverable: false,
		}
	}

	if e.failoverPolicy == FailoverImmediateFail || len(step.AgentIDs) == 1 {
		return e.executeStepWithAgent(ctx, workflow, step, step.AgentIDs[0])
	}

	var attempts []FailoverAttempt
	for i, agentID := range step.AgentIDs {
		select {
		case <-ctx.Done():
			return &StepResult{
				StepID:      step.StepID,
				Err:         ctx.Err(),
				Recoverable: false,
			}
		default:
		}

		result := e.executeStepWithAgent(ctx, workflow, step, agentID)

		if result.Err == nil {
			if i > 0 {
				log.Printf("failover: step %s succeeded on fallback agent %s after %d failed attempt(s)",
					step.StepID, agentID, i)
			}
			return result
		}

		attempts = append(attempts, FailoverAttempt{
			AgentID: agentID,
			Err:     result.Err,
		})

		log.Printf("failover: step %s agent %s failed: %v (attempt %d/%d)",
			step.StepID, agentID, result.Err, i+1, len(step.AgentIDs))
	}

	last := attempts[len(attempts)-1]
	return &StepResult{
		StepID:  step.StepID,
		AgentID: last.AgentID,
		Err: &FailoverAggregatedError{
			StepID:    step.StepID,
			Attempts:  attempts,
			LastAgent: last.AgentID,
		},
		Recoverable: false,
	}
}

func (e *StepExecutor) executeStepWithAgent(ctx context.Context, workflow *Workflow, step WorkflowStep, agentID string) *StepResult {
	taskDesc := fmt.Sprintf("Workflow %s - Step: %s", workflow.ID, step.Name)

	spawnReq := &studio.SpawnRequest{
		DefinitionID:    agentID,
		TaskDescription: taskDesc,
		UserID:          workflow.CreatedBy,
		RoomID:          workflow.RoomID,
		Config:          step.Config,
	}

	// Inject learned skills and/or browser charts into step config before spawning the agent.
	if e.skillFinder != nil || e.chartFinder != nil {
		spawnReq.Config = e.injectLearnedSkills(ctx, spawnReq.Config, taskDesc)
	}

	spawnCtx, cancel := context.WithTimeout(ctx, e.defaultTimeout)
	defer cancel()

	result, err := e.factory.Spawn(spawnCtx, spawnReq)
	if err != nil {
		return &StepResult{
			StepID:      step.StepID,
			AgentID:     agentID,
			Err:         fmt.Errorf("%w: %v", ErrAgentSpawnFailed, err),
			Recoverable: true,
		}
	}

	instanceID := result.Instance.ID

	stateDir := e.agentStateDir(agentID)

	e.mu.Lock()
	stepCtx, stepCancel := context.WithCancel(ctx)
	e.runningSteps[instanceID] = &runningStep{
		workflowID: workflow.ID,
		stepID:     step.StepID,
		instanceID: instanceID,
		startedAt:  time.Now(),
		cancelFunc: stepCancel,
	}
	e.mu.Unlock()

	defer func() {
		e.mu.Lock()
		delete(e.runningSteps, instanceID)
		e.mu.Unlock()
	}()

	// Start agent file tailer for backward communication (agent_events.jsonl,
	// agent_status.json). Emits to MatrixEventBus so events reach Matrix rooms
	// as m.notice messages via MatrixEventForwarder.
	tailer := agent.NewAgentFileTailer(stateDir,
		func(_ agent.AgentFileEvent) {},
		func(_ agent.AgentFileStatus) {},
	)
	if e.eventBus != nil && workflow.CreatedBy != "" {
		tailer.WithEventBus(e.eventBus, workflow.CreatedBy, agentID)
	}
	tailer.Start(ctx)
	defer tailer.Stop()

	completionResult, waitErr := e.waitForCompletion(stepCtx, instanceID, stateDir, workflow.CreatedBy)

	stepResult := &StepResult{
		StepID:      step.StepID,
		AgentID:     agentID,
		InstanceID:  instanceID,
		Recoverable: waitErr != nil && !errors.Is(waitErr, context.Canceled),
	}

	if completionResult != nil {
		stepResult.ContainerResult = completionResult.ContainerResult
	}

	if waitErr != nil {
		stepResult.Err = waitErr
	} else if completionResult != nil && completionResult.ExitCode != 0 {
		stepResult.Err = fmt.Errorf("%w: agent exited with failure", ErrStepExecutionFailed)
	}

	stepSuccess := stepResult.Err == nil

	e.recordSkillOutcomes(step.Config, stepSuccess)

	e.recordChartOutcomes(step.Config, stepSuccess)

	if stepSuccess && e.onSkillExtraction != nil && completionResult != nil && completionResult.ExtendedResult != nil {
		e.onSkillExtraction(completionResult.ExtendedResult, taskDesc, workflow.ID, workflow.TemplateID)
	}

	return stepResult
}

func (e *StepExecutor) waitForCompletion(ctx context.Context, instanceID string, stateDir string, roomIDOpt ...string) (*CompletionResult, error) {
	roomID := ""
	if len(roomIDOpt) > 0 {
		roomID = roomIDOpt[0]
	}
	reader := NewEventReader(stateDir)

	tailerCtx, tailerCancel := context.WithCancel(ctx)
	defer tailerCancel()

	var tailer *agent.AgentFileTailer
	if e.eventBus != nil && roomID != "" {
		emitter := NewWorkflowEventEmitter(e.eventBus)
		tailer = agent.NewAgentFileTailer(stateDir,
			func(_ agent.AgentFileEvent) {},
			func(s agent.AgentFileStatus) {
				emitter.EmitAgentStatus(roomID, s.AgentID, s.State, s.Message)
			},
		)
		tailer.Start(tailerCtx)
	}

	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			_ = e.factory.Stop(context.Background(), instanceID, 10*time.Second)
			_ = cleanupStateDir(stateDir)
			return nil, ctx.Err()
		case <-ticker.C:
			instance, err := e.factory.GetStatus(ctx, instanceID)
			if err != nil {
				return nil, fmt.Errorf("failed to get agent status: %w", err)
			}

			// Tail _events.jsonl while container is running.
			var events []StepEvent
			var readErr error
			if reader != nil {
				events, _, readErr = reader.ReadNew()
				if readErr != nil {
					if errors.Is(readErr, ErrEventLogExceeded) {
						log.Printf("event log exceeded 10MB soft cap, stopping event tail (stateDir=%s)", stateDir)
						reader = nil
					}
				}
			}
			// Route events by type to Matrix room.
			if reader != nil && e.eventBus != nil && roomID != "" {
				emitter := NewWorkflowEventEmitter(e.eventBus)
				for _, evt := range events {
					switch evt.Type {
					case "step", "checkpoint", "progress":
						emitter.EmitStepProgress(roomID, evt)
					case "error":
						emitter.EmitStepError(roomID, evt)
					case "blocker":
						emitter.EmitBlockerWarning(roomID, evt, "", "")
					}
				}
			}

			switch instance.Status {
			case studio.StatusCompleted:
				// Debug: check if result.json exists
				resultPath := stateDir
				if len(resultPath) > 0 && resultPath[len(resultPath)-1] != '/' {
					resultPath += "/"
				}
				resultPath += "result.json"
				ext, _ := ParseExtendedStepResult(stateDir)
				var base *ContainerStepResult
				if ext != nil {
					base = ext.ContainerStepResult
				}
				_ = cleanupStateDir(stateDir)
				return &CompletionResult{ExitCode: 0, ContainerResult: base, ExtendedResult: ext}, nil
			case studio.StatusFailed:
				ext, _ := ParseExtendedStepResult(stateDir)
				var base *ContainerStepResult
				if ext != nil {
					base = ext.ContainerStepResult
				}
				_ = cleanupStateDir(stateDir)
				return &CompletionResult{ExitCode: 1, ContainerResult: base, ExtendedResult: ext}, nil
			case studio.StatusRunning:
				continue
			default:
				continue
			}
		}
	}
}

func (e *StepExecutor) CancelStep(instanceID string) error {
	e.mu.Lock()
	running, exists := e.runningSteps[instanceID]
	e.mu.Unlock()

	if !exists {
		return nil
	}

	running.cancelFunc()

	return e.factory.Stop(context.Background(), instanceID, 10*time.Second)
}

func (e *StepExecutor) CancelAllForWorkflow(workflowID string) error {
	e.mu.RLock()
	var toCancel []string
	for instanceID, running := range e.runningSteps {
		if running.workflowID == workflowID {
			toCancel = append(toCancel, instanceID)
		}
	}
	e.mu.RUnlock()

	var lastErr error
	for _, instanceID := range toCancel {
		if err := e.CancelStep(instanceID); err != nil {
			lastErr = err
		}
	}

	return lastErr
}

func (e *StepExecutor) GetRunningSteps(workflowID string) []string {
	e.mu.RLock()
	defer e.mu.RUnlock()

	var steps []string
	for _, running := range e.runningSteps {
		if running.workflowID == workflowID {
			steps = append(steps, running.stepID)
		}
	}
	return steps
}

//=============================================================================
// Blocker Handling
//=============================================================================

const (
	MaxBlockerRetries = 3
	BlockerTimeout    = 10 * time.Minute
)

// BlockerResponse holds the user's response to a blocker prompt.
// PII SAFETY: The Input field may contain sensitive data and must NEVER be logged.
type BlockerResponse struct {
	Input      string `json:"input"`
	Note       string `json:"note,omitempty"`
	UserID     string `json:"user_id"`
	ProvidedAt int64  `json:"provided_at"`
}

// pendingBlockers stores channels for in-flight blocker waits.
// Key format: "blocker:{workflowID}:{stepID}" → value: chan BlockerResponse
var pendingBlockers sync.Map

// executeStepWithBlockerHandling runs the spawn→wait→blocker loop for a single step.
// If the container reports blockers, the workflow is blocked and the method waits
// for a response before re-spawning with the updated config (up to MaxBlockerRetries).
func (e *StepExecutor) executeStepWithBlockerHandling(
	ctx context.Context,
	workflow *Workflow,
	step WorkflowStep,
	agentID string,
	orchestrator *WorkflowOrchestratorImpl,
) (*StepResult, error) {
	if len(step.AgentIDs) == 0 {
		return nil, ErrNoAgentForStep
	}

	config := step.Config

	for attempt := 1; attempt <= MaxBlockerRetries; attempt++ {
		// 1. Spawn container
		taskDesc := fmt.Sprintf("Workflow %s - Step: %s (attempt %d)", workflow.ID, step.Name, attempt)
		spawnReq := &studio.SpawnRequest{
			DefinitionID:    agentID,
			TaskDescription: taskDesc,
			UserID:          workflow.CreatedBy,
			RoomID:          workflow.RoomID,
			Config:          config,
		}

		spawnCtx, spawnCancel := context.WithTimeout(ctx, e.defaultTimeout)
		spawnResult, err := e.factory.Spawn(spawnCtx, spawnReq)
		spawnCancel()
		if err != nil {
			return nil, fmt.Errorf("%w: %v", ErrAgentSpawnFailed, err)
		}

		instanceID := spawnResult.Instance.ID
		stateDir := e.agentStateDir(agentID)

		// Track running step
		e.mu.Lock()
		stepCtx, stepCancel := context.WithCancel(ctx)
		e.runningSteps[instanceID] = &runningStep{
			workflowID: workflow.ID,
			stepID:     step.StepID,
			instanceID: instanceID,
			startedAt:  time.Now(),
			cancelFunc: stepCancel,
		}
		e.mu.Unlock()

		// 2. Wait for completion
		completionResult, waitErr := e.waitForCompletion(stepCtx, instanceID, stateDir, workflow.CreatedBy)

		// Clean up running step tracking
		e.mu.Lock()
		delete(e.runningSteps, instanceID)
		e.mu.Unlock()

		// 3. Check for ErrEventLogExceeded — return immediately
		if waitErr != nil {
			if errors.Is(waitErr, ErrEventLogExceeded) {
				return nil, waitErr
			}
			return nil, waitErr
		}

		// 4. Check exit code
		if completionResult != nil && completionResult.ExitCode != 0 {
			return nil, fmt.Errorf("%w: agent exited with failure", ErrStepExecutionFailed)
		}

		// 5. Check for blockers
		hasBlockers := completionResult != nil && completionResult.ExtendedResult != nil && len(completionResult.ExtendedResult.Blockers) > 0
		if hasBlockers {
			blocker := completionResult.ExtendedResult.Blockers[0]

			blockerMeta := map[string]interface{}{
				"blocker_type": blocker.BlockerType,
				"message":      blocker.Message,
				"suggestion":   blocker.Suggestion,
				"field":        blocker.Field,
			}

			// Block the workflow
			if blockErr := orchestrator.BlockWorkflow(workflow.ID, "blocker", blocker.Message, blockerMeta); blockErr != nil {
				return nil, fmt.Errorf("failed to block workflow: %w", blockErr)
			}

			// Wait for blocker response
			response, respErr := e.waitForBlockerResponse(ctx, workflow.ID, step.StepID)
			if respErr != nil {
				return nil, fmt.Errorf("blocker timeout: %w", respErr)
			}

			// Append response to config (PII safe — memory only, never logged)
			config = appendBlockerResponse(config, response)

			if unblockErr := orchestrator.UnblockWorkflow(workflow.ID); unblockErr != nil {
				return nil, fmt.Errorf("failed to unblock workflow: %w", unblockErr)
			}

			continue
		}

		// 6. No blockers — build success result and return
		stepResult := &StepResult{
			StepID:      step.StepID,
			AgentID:     agentID,
			InstanceID:  instanceID,
			Recoverable: false,
		}
		if completionResult != nil {
			stepResult.ContainerResult = completionResult.ContainerResult
		}
		return stepResult, nil
	}

	// Max retries exceeded
	return nil, fmt.Errorf("max blocker retries (%d) exceeded", MaxBlockerRetries)
}

// waitForBlockerResponse waits for a user response to a blocker prompt.
// It registers a channel in the pendingBlockers sync.Map and waits for
// an external caller (T18 Matrix handler) to deliver the response.
func (e *StepExecutor) waitForBlockerResponse(ctx context.Context, workflowID, stepID string) (BlockerResponse, error) {
	key := fmt.Sprintf("blocker:%s:%s", workflowID, stepID)
	ch := make(chan BlockerResponse, 1)
	pendingBlockers.Store(key, ch)
	defer pendingBlockers.Delete(key)

	timeoutCtx, timeoutCancel := context.WithTimeout(ctx, BlockerTimeout)
	defer timeoutCancel()

	select {
	case resp := <-ch:
		return resp, nil
	case <-timeoutCtx.Done():
		return BlockerResponse{}, fmt.Errorf("blocker response timeout after %v", BlockerTimeout)
	case <-ctx.Done():
		return BlockerResponse{}, ctx.Err()
	}
}

// DeliverBlockerResponse delivers a blocker response to a waiting step executor.
// This is the public API for T18's Matrix handler to call.
func DeliverBlockerResponse(workflowID, stepID string, response BlockerResponse) bool {
	key := fmt.Sprintf("blocker:%s:%s", workflowID, stepID)
	val, ok := pendingBlockers.Load(key)
	if !ok {
		return false
	}
	ch, ok := val.(chan BlockerResponse)
	if !ok {
		return false
	}
	select {
	case ch <- response:
		return true
	default:
		return false
	}
}

func (e *StepExecutor) injectLearnedSkills(ctx context.Context, config json.RawMessage, taskDesc string) json.RawMessage {
	if e.skillFinder == nil && e.chartFinder == nil {
		return config
	}

	var configMap map[string]interface{}
	if config != nil {
		if err := json.Unmarshal(config, &configMap); err != nil {
			configMap = make(map[string]interface{})
		}
	} else {
		configMap = make(map[string]interface{})
	}

	modified := false
	skillsEnabled := os.Getenv("ARMORCLAW_LEARNED_SKILLS_INJECTION") != "false"

	if skillsEnabled && e.skillFinder != nil {
		matched, err := e.skillFinder.FindForTask(taskDesc, 3)
		if err == nil && len(matched) > 0 {
			var skillContexts []map[string]interface{}
			for _, skill := range matched {
				skillContexts = append(skillContexts, map[string]interface{}{
					"name":       skill.Name,
					"confidence": skill.Confidence,
					"pattern":    skill.PatternType,
					"source":     skill.SourceTaskID,
				})
			}
			configMap["relevant_skills"] = skillContexts
			modified = true
		}
	}

	if e.chartFinder != nil {
		if rawURL, ok := configMap["url"].(string); ok && rawURL != "" {
			if domain := extractDomainFromURL(rawURL); domain != "" {
				charts, err := e.chartFinder.FindForDomain(ctx, domain, 3)
				if err == nil && len(charts) > 0 {
					var chartContexts []map[string]interface{}
					for _, chart := range charts {
						chartContexts = append(chartContexts, map[string]interface{}{
							"chart_id":          chart.ChartID,
							"domain":            chart.Domain,
							"title":             chart.Title,
							"confidence":        chart.Confidence,
							"steps":             chart.Steps,
							"requires_approval": chart.RequiresApproval,
						})
					}
					configMap["relevant_charts"] = chartContexts
					modified = true
				}
			}
		}
	}

	if !modified {
		return config
	}

	updated, err := json.Marshal(configMap)
	if err != nil {
		return config
	}
	return updated
}

func extractDomainFromURL(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil || u.Host == "" {
		return ""
	}
	return u.Hostname()
}

// appendBlockerResponse adds the blocker response metadata to the step config.
// PII SAFETY: The response.Input field is NEVER serialized into the config map.
// Only a presence indicator (has_input) is stored. The raw Input value remains
// available in-memory via the blocker response channel for the current step execution.
func appendBlockerResponse(config json.RawMessage, response BlockerResponse) json.RawMessage {
	var configMap map[string]interface{}
	if config != nil {
		if err := json.Unmarshal(config, &configMap); err != nil {
			configMap = make(map[string]interface{})
		}
	} else {
		configMap = make(map[string]interface{})
	}

	// SECURITY: Never serialize raw PII into the config map.
	// Store only a presence indicator; raw Input is available in-memory
	// via the blocker response channel for the current step execution.
	configMap["_blocker_response"] = map[string]interface{}{
		"has_input":   response.Input != "",
		"note":        response.Note,
		"user_id":     response.UserID,
		"provided_at": response.ProvidedAt,
	}

	updated, err := json.Marshal(configMap)
	if err != nil {
		return config // fallback to original on marshal error
	}
	return updated
}

func (e *StepExecutor) recordSkillOutcomes(config json.RawMessage, success bool) {
	if e.onSkillOutcome == nil {
		return
	}

	var configMap map[string]interface{}
	if err := json.Unmarshal(config, &configMap); err != nil {
		return
	}

	skills, ok := configMap["relevant_skills"].([]interface{})
	if !ok {
		return
	}

	for _, s := range skills {
		skillMap, ok := s.(map[string]interface{})
		if !ok {
			continue
		}
		source, ok := skillMap["source"].(string)
		if !ok {
			continue
		}
		_ = e.onSkillOutcome(source, success)
	}
}

func (e *StepExecutor) recordChartOutcomes(config json.RawMessage, success bool) {
	if e.onChartOutcome == nil {
		return
	}

	var configMap map[string]interface{}
	if err := json.Unmarshal(config, &configMap); err != nil {
		return
	}

	charts, ok := configMap["relevant_charts"].([]interface{})
	if !ok {
		return
	}

	for _, c := range charts {
		chartMap, ok := c.(map[string]interface{})
		if !ok {
			continue
		}
		chartID, ok := chartMap["chart_id"].(string)
		if !ok {
			continue
		}
		_ = e.onChartOutcome(chartID, success)
	}
}

//=============================================================================
// Orchestrator Integration
//=============================================================================

type OrchestratorIntegration struct {
	orchestrator        *WorkflowOrchestratorImpl
	executor            *StepExecutor
	store               Store
	approvalEngine      *ApprovalEngineImpl
	notificationService *NotificationService

	mu               sync.RWMutex
	executionCancels map[string]context.CancelFunc
}

type IntegrationConfig struct {
	Orchestrator        *WorkflowOrchestratorImpl
	Executor            *StepExecutor
	Store               Store
	ApprovalEngine      *ApprovalEngineImpl
	NotificationService *NotificationService
}

func NewOrchestratorIntegration(cfg IntegrationConfig) *OrchestratorIntegration {
	return &OrchestratorIntegration{
		orchestrator:        cfg.Orchestrator,
		executor:            cfg.Executor,
		store:               cfg.Store,
		approvalEngine:      cfg.ApprovalEngine,
		notificationService: cfg.NotificationService,
		executionCancels:    make(map[string]context.CancelFunc),
	}
}

func (i *OrchestratorIntegration) StartWorkflowExecution(workflowID string) error {
	i.mu.Lock()
	defer i.mu.Unlock()

	if _, exists := i.executionCancels[workflowID]; exists {
		return fmt.Errorf("workflow %s is already executing", workflowID)
	}

	workflow, err := i.orchestrator.GetWorkflow(workflowID)
	if err != nil {
		return fmt.Errorf("failed to get workflow: %w", err)
	}

	if workflow.Status != StatusRunning {
		return fmt.Errorf("%w: current status %s", ErrWorkflowNotRunning, workflow.Status)
	}

	var template *TaskTemplate
	if workflow.TemplateID != "" {
		template, err = i.store.GetTemplate(context.Background(), workflow.TemplateID)
		if err != nil {
			return fmt.Errorf("failed to get template: %w", err)
		}
	}

	ctx, cancel := context.WithCancel(context.Background())
	i.executionCancels[workflowID] = cancel

	go i.runWorkflow(ctx, workflowID, workflow, template)

	return nil
}

func (i *OrchestratorIntegration) runWorkflow(
	ctx context.Context,
	workflowID string,
	workflow *Workflow,
	template *TaskTemplate,
) {
	defer func() {
		i.mu.Lock()
		delete(i.executionCancels, workflowID)
		i.mu.Unlock()
	}()

	if i.notificationService != nil && workflow != nil {
		i.notificationService.NotifyWorkflowStarted(workflow, template)
	}

	err := i.executor.ExecuteSteps(ctx, i.orchestrator, workflow, template)

	if err != nil {
		var stepErr *StepExecutionError
		recoverable := false
		stepID := ""

		if errors.As(err, &stepErr) {
			recoverable = stepErr.Recoverable
			stepID = stepErr.StepID
		}

		if errors.Is(err, context.Canceled) {
			i.orchestrator.CancelWorkflow(workflowID, "execution cancelled")
			if i.notificationService != nil {
				updatedWf, _ := i.orchestrator.GetWorkflow(workflowID)
				i.notificationService.NotifyWorkflowCancelled(updatedWf, "execution cancelled", template)
			}
			return
		}

		i.orchestrator.FailWorkflow(workflowID, stepID, err, recoverable)
		if i.notificationService != nil {
			updatedWf, _ := i.orchestrator.GetWorkflow(workflowID)
			i.notificationService.NotifyWorkflowFailed(updatedWf, stepID, err, recoverable, template)
		}
		return
	}

	i.orchestrator.CompleteWorkflow(workflowID, "all steps completed successfully")
	if i.notificationService != nil {
		updatedWf, _ := i.orchestrator.GetWorkflow(workflowID)
		i.notificationService.NotifyWorkflowCompleted(updatedWf, "all steps completed successfully", template)
	}
}

func (i *OrchestratorIntegration) CancelWorkflowExecution(workflowID string) error {
	i.mu.Lock()
	cancel, exists := i.executionCancels[workflowID]
	i.mu.Unlock()

	if !exists {
		return nil
	}

	cancel()

	return i.executor.CancelAllForWorkflow(workflowID)
}

func (i *OrchestratorIntegration) GetExecutionStatus(workflowID string) *ExecutionStatus {
	var runningSteps []string
	if i.executor != nil {
		runningSteps = i.executor.GetRunningSteps(workflowID)
	}

	i.mu.RLock()
	_, isExecuting := i.executionCancels[workflowID]
	i.mu.RUnlock()

	return &ExecutionStatus{
		WorkflowID:   workflowID,
		IsExecuting:  isExecuting,
		RunningSteps: runningSteps,
	}
}

type ExecutionStatus struct {
	WorkflowID   string
	IsExecuting  bool
	RunningSteps []string
}
