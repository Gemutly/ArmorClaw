package secretary

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/armorclaw/bridge/pkg/studio"
)

//=============================================================================
// Matrix Adapter Wrapper
//=============================================================================

type MatrixSender interface {
	SendMessageWithRetry(roomID, message, msgType string) (string, error)
	SendFormattedMessage(ctx context.Context, roomID, plainBody, formattedBody string) error
	ReplyToEvent(ctx context.Context, roomID, eventID, message string) error
}

type matrixAdapterWrapper struct {
	inner MatrixSender
}

func (w *matrixAdapterWrapper) SendMessage(ctx context.Context, roomID, message string) error {
	_, err := w.inner.SendMessageWithRetry(roomID, message, "m.notice")
	return err
}

func (w *matrixAdapterWrapper) SendFormattedMessage(ctx context.Context, roomID, plainBody, formattedBody string) error {
	return w.inner.SendFormattedMessage(ctx, roomID, plainBody, formattedBody)
}

func (w *matrixAdapterWrapper) ReplyToEvent(ctx context.Context, roomID, eventID, message string) error {
	return w.inner.ReplyToEvent(ctx, roomID, eventID, message)
}

func WrapMatrixAdapter(inner MatrixSender) studio.MatrixAdapter {
	return &matrixAdapterWrapper{inner: inner}
}

//=============================================================================
// Interfaces for Testability
//=============================================================================

type LearnWebsiteServiceInterface interface {
	Learn(ctx context.Context, req *LearnWebsiteRequest) (*LearnWebsiteResult, error)
	ConfirmMapping(ctx context.Context, mapping *ConfirmedMapping) error
}

type BlindFillExecutorInterface interface {
	Execute(ctx context.Context, req *BlindFillRequest) (*BlindFillResult, error)
	Cancel(executionID string) error
	GetExecutionStatus(executionID string) (*BlindFillResult, bool)
	ListActiveExecutions() []string
	GetActiveExecutionCount() int
}

type TrustedWorkflowEngineInterface interface {
	Evaluate(ctx context.Context, req *TrustEvaluationRequest) (*TrustEvaluationResult, error)
	CreatePolicy(ctx context.Context, policy *TrustedWorkflowPolicy) error
	GetPolicy(ctx context.Context, id string) (*TrustedWorkflowPolicy, error)
	ListPolicies(ctx context.Context, activeOnly bool) ([]*TrustedWorkflowPolicy, error)
	DeletePolicy(ctx context.Context, id string) error
	RevokePolicy(ctx context.Context, id string, revokedBy string, reason string) error
}

type RolodexServiceInterface interface {
	CreateContact(ctx context.Context, req *CreateRequest) (*Contact, error)
	GetContact(ctx context.Context, id string) (*Contact, error)
	ListContacts(ctx context.Context, filter ContactFilter) ([]Contact, error)
	UpdateContact(ctx context.Context, req *UpdateRequest) (*Contact, error)
	DeleteContact(ctx context.Context, id string) error
	SearchContacts(ctx context.Context, query string) ([]Contact, error)
}

type WebDAVServiceInterface interface {
	ExecuteWebDAV(ctx context.Context, params map[string]interface{}) (interface{}, error)
}

type CalendarServiceInterface interface {
	ExecuteCalendar(ctx context.Context, params map[string]interface{}) (interface{}, error)
}

//=============================================================================
// Secretary Command Handler
//=============================================================================

type SecretaryCommandHandler struct {
	store        SecretaryStore
	orchestrator *WorkflowOrchestratorImpl
	integration  *OrchestratorIntegration
	studio       *StudioIntegration
	matrix       studio.MatrixAdapter
	prefix       string

	learnWebsite   LearnWebsiteServiceInterface
	blindFill      BlindFillExecutorInterface
	trustEngine    TrustedWorkflowEngineInterface
	approvalEngine *ApprovalEngineImpl
	rolodex        RolodexServiceInterface
	webdav         WebDAVServiceInterface
	calendar       CalendarServiceInterface
}

type SecretaryCommandHandlerConfig struct {
	Store        SecretaryStore
	Orchestrator *WorkflowOrchestratorImpl
	Integration  *OrchestratorIntegration
	Studio       *StudioIntegration
	Matrix       studio.MatrixAdapter
	Prefix       string

	LearnWebsite   LearnWebsiteServiceInterface
	BlindFill      BlindFillExecutorInterface
	TrustEngine    TrustedWorkflowEngineInterface
	ApprovalEngine *ApprovalEngineImpl
	Rolodex        RolodexServiceInterface
	WebDAV         WebDAVServiceInterface
	Calendar       CalendarServiceInterface
}

func NewSecretaryCommandHandler(cfg SecretaryCommandHandlerConfig) *SecretaryCommandHandler {
	prefix := cfg.Prefix
	if prefix == "" {
		prefix = "!"
	}

	return &SecretaryCommandHandler{
		store:          cfg.Store,
		orchestrator:   cfg.Orchestrator,
		integration:    cfg.Integration,
		studio:         cfg.Studio,
		matrix:         cfg.Matrix,
		prefix:         prefix,
		learnWebsite:   cfg.LearnWebsite,
		blindFill:      cfg.BlindFill,
		trustEngine:    cfg.TrustEngine,
		approvalEngine: cfg.ApprovalEngine,
		rolodex:        cfg.Rolodex,
		webdav:         cfg.WebDAV,
		calendar:       cfg.Calendar,
	}
}

// sendMessage is a nil-safe helper for sending Matrix messages
func (h *SecretaryCommandHandler) sendMessage(ctx context.Context, roomID, message string) error {
	if h.matrix == nil {
		fmt.Printf("[secretary] no matrix adapter: room=%s msg=%s\n", roomID, message)
		return nil
	}
	err := h.matrix.SendMessage(ctx, roomID, message)
	if err != nil {
		fmt.Printf("[secretary] SendMessage failed: room=%s err=%v\n", roomID, err)
	}
	return err
}

func (h *SecretaryCommandHandler) HandleMatrixMessage(ctx context.Context, roomID, userID, eventID, text string) bool {
	handled, _ := h.HandleMessage(ctx, roomID, userID, eventID, text)
	return handled
}

// HandleMessage processes Matrix messages for Secretary commands
func (h *SecretaryCommandHandler) HandleMessage(ctx context.Context, roomID, userID, eventID, text string) (bool, error) {
	text = strings.TrimSpace(text)

	if !strings.HasPrefix(text, h.prefix+"secretary ") {
		return false, nil
	}

	text = strings.TrimPrefix(text, h.prefix+"secretary ")
	parts := strings.Fields(text)
	if len(parts) == 0 {
		return true, h.sendHelp(ctx, roomID)
	}

	subCmd := parts[0]
	args := parts[1:]

	switch subCmd {
	case "help":
		return true, h.sendHelp(ctx, roomID)
	case "create":
		if len(args) >= 2 && args[0] == "workflow" {
			return true, h.handleCreateWorkflow(ctx, roomID, userID, args[1:])
		}
	case "start":
		if len(args) >= 2 && args[0] == "workflow" {
			return true, h.handleStartWorkflow(ctx, roomID, args[1])
		}
	case "list":
		if len(args) >= 1 {
			switch args[0] {
			case "workflows":
				return true, h.handleListWorkflows(ctx, roomID)
			case "agents":
				return true, h.handleListAgents(ctx, roomID)
			case "templates":
				return true, h.handleListTemplates(ctx, roomID)
			case "trust", "trust-policies", "trusted":
				return true, h.handleListTrustPolicies(ctx, roomID)
			}
		}
	case "workflow":
		if len(args) >= 1 {
			switch args[0] {
			case "status":
				if len(args) >= 2 {
					return true, h.handleWorkflowStatus(ctx, roomID, args[1])
				}
			case "cancel":
				if len(args) >= 2 {
					return true, h.handleCancelWorkflow(ctx, roomID, args[1])
				}
			}
		}
	case "delete":
		if len(args) >= 2 && args[0] == "agent" {
			return true, h.handleDeleteAgent(ctx, roomID, args[1])
		}
	case "learn":
		if len(args) >= 2 && args[0] == "website" {
			return true, h.handleLearnWebsite(ctx, roomID, userID, args[1:])
		}
	case "review":
		if len(args) >= 2 && args[0] == "mapping" {
			return true, h.handleReviewMapping(ctx, roomID, args[1:])
		}
	case "confirm":
		if len(args) >= 2 && args[0] == "mapping" {
			return true, h.handleConfirmMapping(ctx, roomID, userID, args[1:])
		}
	case "run":
		if len(args) >= 2 && args[0] == "blindfill" {
			return true, h.handleRunBlindFill(ctx, roomID, userID, args[1:])
		}
	case "trust":
		if len(args) >= 1 {
			switch args[0] {
			case "list":
				return true, h.handleListTrustPolicies(ctx, roomID)
			case "create":
				return true, h.handleCreateTrustPolicy(ctx, roomID, userID, args[1:])
			case "revoke":
				if len(args) >= 2 {
					return true, h.handleRevokeTrustPolicy(ctx, roomID, userID, args[1:])
				}
			}
		}
	case "contact":
		if len(args) >= 1 {
			switch args[0] {
			case "create":
				return true, h.handleCreateContact(ctx, roomID, userID, args[1:])
			case "get":
				if len(args) >= 2 {
					return true, h.handleGetContact(ctx, roomID, args[1])
				}
			case "list":
				return true, h.handleListContacts(ctx, roomID, args[1:])
			case "update":
				if len(args) >= 2 {
					return true, h.handleUpdateContact(ctx, roomID, userID, args[1:])
				}
			case "delete":
				if len(args) >= 2 {
					return true, h.handleDeleteContact(ctx, roomID, args[1])
				}
			case "search":
				if len(args) >= 2 {
					return true, h.handleSearchContacts(ctx, roomID, strings.Join(args[1:], " "))
				}
			}
		}
	case "webdav":
		if len(args) >= 1 {
			switch args[0] {
			case "list":
				return true, h.handleWebDAVList(ctx, roomID, args[1:])
			case "get":
				if len(args) >= 2 {
					return true, h.handleWebDAVGet(ctx, roomID, args[1:])
				}
			case "put":
				if len(args) >= 2 {
					return true, h.handleWebDAVPut(ctx, roomID, args[1:])
				}
			case "delete":
				if len(args) >= 2 {
					return true, h.handleWebDAVDelete(ctx, roomID, args[1:])
				}
			}
		}
	case "calendar":
		if len(args) >= 1 {
			switch args[0] {
			case "list":
				return true, h.handleCalendarList(ctx, roomID, args[1:])
			case "create":
				if len(args) >= 2 {
					return true, h.handleCalendarCreate(ctx, roomID, args[1:])
				}
			case "get_events":
				if len(args) >= 2 {
					return true, h.handleCalendarGetEvents(ctx, roomID, args[1:])
				}
			case "get_event":
				if len(args) >= 2 {
					return true, h.handleCalendarGetEvent(ctx, roomID, args[1:])
				}
			case "update":
				if len(args) >= 2 {
					return true, h.handleCalendarUpdate(ctx, roomID, args[1:])
				}
			case "delete":
				if len(args) >= 2 {
					return true, h.handleCalendarDelete(ctx, roomID, args[1:])
				}
			}
		}
	default:
		return true, h.sendHelp(ctx, roomID)
	}

	return false, nil
}

func (h *SecretaryCommandHandler) sendHelp(ctx context.Context, roomID string) error {
	help := `📋 Secretary Commands

**Workflow Management:**
` + h.prefix + `secretary help                    - Show this help
` + h.prefix + `secretary create workflow <id>    - Create workflow from template
` + h.prefix + `secretary start workflow <id>     - Start a pending workflow
` + h.prefix + `secretary list workflows          - List all workflows
` + h.prefix + `secretary workflow status <id>    - Show workflow status
` + h.prefix + `secretary workflow cancel <id>    - Cancel running workflow
` + h.prefix + `secretary list agents             - List secretary agents
` + h.prefix + `secretary delete agent <id>       - Delete secretary agent
` + h.prefix + `secretary list templates          - List available templates

**Learn Website (Form Discovery):**
` + h.prefix + `secretary learn website <url>     - Discover form fields on a website
` + h.prefix + `secretary review mapping <id>     - Review a mapping draft
` + h.prefix + `secretary confirm mapping <id> [fields...] - Confirm field mappings

**BlindFill Execution:**
` + h.prefix + `secretary run blindfill <template> [options] - Execute BlindFill

**Trusted Workflows:**
` + h.prefix + `secretary trust list              - List trust policies
` + h.prefix + `secretary trust create <name>     - Create a trust policy
` + h.prefix + `secretary trust revoke <id>       - Revoke a trust policy

**Examples:**
  ` + h.prefix + `secretary learn website https://example.com/form
  ` + h.prefix + `secretary review mapping draft_abc123
  ` + h.prefix + `secretary confirm mapping draft_abc123
  ` + h.prefix + `secretary run blindfill template_xyz
  ` + h.prefix + `secretary trust list

**Contacts:**
` + h.prefix + `secretary contact create <name> [company=...] [relationship=...] [phone=...] [email=...] [address=...] [notes=...] - Create a contact
` + h.prefix + `secretary contact get <id> - Get a contact by ID
` + h.prefix + `secretary contact list [name=...] [company=...] [relationship=...] - List contacts with optional filters
` + h.prefix + `secretary contact update <id> [name=...] [company=...] [relationship=...] [phone=...] [email=...] [address=...] [notes=...] - Update a contact
` + h.prefix + `secretary contact delete <id> - Delete a contact
` + h.prefix + `secretary contact search <query> - Search contacts by name or company

**Examples:**
  ` + h.prefix + `secretary contact create "John Doe" company="Acme Inc" relationship="client" phone="555-1234" email="john@example.com"
  ` + h.prefix + `secretary contact list company="Acme"
  ` + h.prefix + `secretary contact search "John"

**WebDAV:**
` + h.prefix + `secretary webdav list <url> [username=...] [password=...] - List directory contents
` + h.prefix + `secretary webdav get <url> [username=...] [password=...] - Get file content
` + h.prefix + `secretary webdav put <url> <content> [username=...] [password=...] [content_type=...] - Upload file
` + h.prefix + `secretary webdav delete <url> [username=...] [password=...] - Delete file

**Examples:**
  ` + h.prefix + `secretary webdav list http://localhost:8080/
  ` + h.prefix + `secretary webdav get http://localhost:8080/test.txt username=user password=pass
  ` + h.prefix + `secretary webdav put http://localhost:8080/test.txt "Hello World" content_type=text/plain

**Calendar:**
` + h.prefix + `secretary calendar list <calendar_url> [username=...] [password=...] - List calendars
` + h.prefix + `secretary calendar create <title> start="YYYY-MM-DDTHH:MM:SSZ" end="YYYY-MM-DDTHH:MM:SSZ" [calendar_url=...] [description=...] [location=...] [attendees=...] - Create event
` + h.prefix + `secretary calendar get_events <calendar_url> [username=...] [password=...] - Get all events
` + h.prefix + `secretary calendar get_event <calendar_url> uid=<event_uid> [username=...] [password=...] - Get specific event
` + h.prefix + `secretary calendar update <calendar_url> uid=<event_uid> [start="..."] [end="..."] [title=...] [description=...] [location=...] - Update event
` + h.prefix + `secretary calendar delete <calendar_url> uid=<event_uid> [username=...] [password=...] - Delete event

**Examples:**
  ` + h.prefix + `secretary calendar list http://localhost:8080/calendars/default/
  ` + h.prefix + `secretary calendar create "Team Meeting" start="2026-03-20T10:00:00Z" end="2026-03-20T11:00:00Z" location="Room 101"
  ` + h.prefix + `secretary calendar update http://localhost:8080/calendars/default/ uid="cal-12345" start="2026-03-20T12:00:00Z"
`
	return h.sendMessage(ctx, roomID, help)
}

func (h *SecretaryCommandHandler) handleCreateWorkflow(ctx context.Context, roomID, userID string, args []string) error {
	if len(args) == 0 {
		return h.sendMessage(ctx, roomID, "❌ Usage: !secretary create workflow <template_id>")
	}

	templateID := args[0]

	if h.studio == nil {
		return h.sendMessage(ctx, roomID, "❌ Studio integration not configured")
	}

	workflow, err := h.studio.CreateWorkflowFromTemplate(ctx, templateID, nil, userID)
	if err != nil {
		return h.sendMessage(ctx, roomID, fmt.Sprintf("❌ Failed to create workflow: %v", err))
	}

	msg := fmt.Sprintf("✅ Workflow created\n\nID: `%s`\nStatus: %s", workflow.ID, workflow.Status)
	return h.sendMessage(ctx, roomID, msg)
}

func (h *SecretaryCommandHandler) handleStartWorkflow(ctx context.Context, roomID, workflowID string) error {
	if h.orchestrator == nil {
		return h.sendMessage(ctx, roomID, "❌ Orchestrator not configured")
	}

	if err := h.orchestrator.StartWorkflow(workflowID); err != nil {
		return h.sendMessage(ctx, roomID, fmt.Sprintf("❌ Failed to start workflow: %v", err))
	}

	// Persist the room ID from the triggering context
	workflow, _ := h.orchestrator.GetWorkflow(workflowID)
	if workflow != nil {
		workflow.RoomID = roomID
		_ = h.store.UpdateWorkflow(ctx, workflow)
	}

	// Start executing workflow steps
	if h.integration != nil {
		if err := h.integration.StartWorkflowExecution(workflowID); err != nil {
			return h.sendMessage(ctx, roomID, fmt.Sprintf("⚠️ Workflow started but execution failed: %v", err))
		}
	}

	if workflow != nil {
		msg := fmt.Sprintf("✅ Workflow started and executing\n\nID: `%s`\nStatus: %s", workflowID, workflow.Status)
		return h.sendMessage(ctx, roomID, msg)
	}
	return nil
}

func (h *SecretaryCommandHandler) handleListWorkflows(ctx context.Context, roomID string) error {
	if h.store == nil {
		return h.sendMessage(ctx, roomID, "❌ Store not configured")
	}

	workflows, err := h.store.ListWorkflows(ctx, WorkflowFilter{})
	if err != nil {
		return h.sendMessage(ctx, roomID, fmt.Sprintf("❌ Failed to list workflows: %v", err))
	}

	if len(workflows) == 0 {
		return h.sendMessage(ctx, roomID, "No workflows found.")
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("📋 Workflows (%d)\n\n", len(workflows)))
	for _, wf := range workflows {
		sb.WriteString(fmt.Sprintf("**%s** `%s` - %s\n\n", wf.Name, wf.ID, wf.Status))
	}

	return h.sendMessage(ctx, roomID, sb.String())
}

func (h *SecretaryCommandHandler) handleWorkflowStatus(ctx context.Context, roomID, workflowID string) error {
	if h.store == nil {
		return h.sendMessage(ctx, roomID, "❌ Store not configured")
	}

	workflow, err := h.store.GetWorkflow(ctx, workflowID)
	if err != nil {
		return h.sendMessage(ctx, roomID, fmt.Sprintf("❌ Workflow not found: %s", workflowID))
	}

	msg := fmt.Sprintf(`📋 Workflow: %s

ID: %s
Status: %s
Started: %s
`, workflow.Name, workflow.ID, workflow.Status, workflow.StartedAt.Format("2006-01-02 15:04"))

	return h.sendMessage(ctx, roomID, msg)
}

func (h *SecretaryCommandHandler) handleCancelWorkflow(ctx context.Context, roomID, workflowID string) error {
	if h.orchestrator != nil {
		if err := h.orchestrator.CancelWorkflow(workflowID, "cancelled via command"); err != nil {
			return h.sendMessage(ctx, roomID, fmt.Sprintf("❌ Failed to cancel workflow: %v", err))
		}
		return h.sendMessage(ctx, roomID, fmt.Sprintf("✅ Workflow `%s` cancelled", workflowID))
	}

	if h.store == nil {
		return h.sendMessage(ctx, roomID, "❌ Store not configured")
	}

	workflow, err := h.store.GetWorkflow(ctx, workflowID)
	if err != nil {
		return h.sendMessage(ctx, roomID, fmt.Sprintf("❌ Workflow not found: %s", workflowID))
	}

	if workflow.Status != StatusRunning {
		return h.sendMessage(ctx, roomID, fmt.Sprintf("❌ Workflow is not running: %s (status: %s)", workflowID, workflow.Status))
	}

	workflow.Status = StatusCancelled
	if err := h.store.UpdateWorkflow(ctx, workflow); err != nil {
		return h.sendMessage(ctx, roomID, fmt.Sprintf("❌ Failed to cancel workflow: %v", err))
	}

	return h.sendMessage(ctx, roomID, fmt.Sprintf("✅ Workflow `%s` cancelled", workflowID))
}

func (h *SecretaryCommandHandler) handleListAgents(ctx context.Context, roomID string) error {
	if h.studio == nil {
		return h.sendMessage(ctx, roomID, "❌ Studio integration not configured")
	}

	agents, err := h.studio.ListSecretaryAgents(ctx)
	if err != nil {
		return h.sendMessage(ctx, roomID, fmt.Sprintf("❌ Failed to list agents: %v", err))
	}

	if len(agents) == 0 {
		return h.sendMessage(ctx, roomID, "No secretary agents running.")
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("🤖 Secretary Agents (%d)\n\n", len(agents)))
	for _, agent := range agents {
		sb.WriteString(fmt.Sprintf("**%s** `%s` - %s\n\n", agent.DefinitionID, agent.ID, agent.Status))
	}

	return h.sendMessage(ctx, roomID, sb.String())
}

func (h *SecretaryCommandHandler) handleDeleteAgent(ctx context.Context, roomID, agentID string) error {
	if h.studio == nil {
		return h.sendMessage(ctx, roomID, "❌ Studio integration not configured")
	}

	if err := h.studio.DeleteSecretaryAgent(ctx, agentID); err != nil {
		return h.sendMessage(ctx, roomID, fmt.Sprintf("❌ Failed to delete agent: %v", err))
	}

	return h.sendMessage(ctx, roomID, fmt.Sprintf("✅ Agent `%s` deleted", agentID))
}

func (h *SecretaryCommandHandler) handleListTemplates(ctx context.Context, roomID string) error {
	if h.store == nil {
		return h.sendMessage(ctx, roomID, "❌ Store not configured")
	}

	templates, err := h.store.ListTemplates(ctx, TemplateFilter{})
	if err != nil {
		return h.sendMessage(ctx, roomID, fmt.Sprintf("❌ Failed to list templates: %v", err))
	}

	if len(templates) == 0 {
		return h.sendMessage(ctx, roomID, "No templates found.")
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("📋 Templates (%d)\n\n", len(templates)))
	for _, t := range templates {
		status := "inactive"
		if t.IsActive {
			status = "active"
		}
		sb.WriteString(fmt.Sprintf("**%s** `%s` - %s\n", t.Name, t.ID, status))
		if t.Description != "" {
			sb.WriteString(fmt.Sprintf("  %s\n", t.Description))
		}
		sb.WriteString("\n")
	}

	return h.sendMessage(ctx, roomID, sb.String())
}

func (h *SecretaryCommandHandler) handleLearnWebsite(ctx context.Context, roomID, userID string, args []string) error {
	if h.learnWebsite == nil {
		return h.sendMessage(ctx, roomID, "❌ Learn Website service not configured")
	}

	if len(args) == 0 {
		return h.sendMessage(ctx, roomID, "❌ Usage: "+h.prefix+"secretary learn website <url> [form_selector]")
	}

	targetURL := args[0]
	var formSelector string
	if len(args) > 1 {
		formSelector = args[1]
	}

	h.sendMessage(ctx, roomID, fmt.Sprintf("🔍 Learning website: %s", targetURL))

	req := &LearnWebsiteRequest{
		TargetURL:    targetURL,
		Initiator:    userID,
		FormSelector: formSelector,
		Timeout:      30000,
		WaitUntil:    "networkidle",
	}

	result, err := h.learnWebsite.Learn(ctx, req)
	if err != nil {
		return h.sendMessage(ctx, roomID, fmt.Sprintf("❌ Failed to learn website: %v", err))
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("✅ Website learned successfully\n\n"))
	sb.WriteString(fmt.Sprintf("**Request ID:** `%s`\n", result.RequestID))
	sb.WriteString(fmt.Sprintf("**URL:** %s\n", result.TargetURL))
	if result.PageTitle != "" {
		sb.WriteString(fmt.Sprintf("**Page Title:** %s\n", result.PageTitle))
	}
	sb.WriteString(fmt.Sprintf("**Fields Discovered:** %d\n\n", len(result.Fields)))

	if result.MappingDraft != nil {
		sb.WriteString(fmt.Sprintf("**Mapping Draft ID:** `%s`\n", result.MappingDraft.ID))
		sb.WriteString(fmt.Sprintf("**Template Name:** %s\n\n", result.MappingDraft.TemplateName))
	}

	sb.WriteString("**Discovered Fields:**\n")
	for i, field := range result.Fields {
		if i >= 10 {
			sb.WriteString(fmt.Sprintf("  ... and %d more fields\n", len(result.Fields)-10))
			break
		}
		sb.WriteString(fmt.Sprintf("  • %s (`%s`) - %s\n", field.LabelText, field.Selector, field.TagName))
	}

	if len(result.Warnings) > 0 {
		sb.WriteString(fmt.Sprintf("\n⚠️ **Warnings:** %d\n", len(result.Warnings)))
	}

	sb.WriteString(fmt.Sprintf("\nTo review and confirm mapping: `%ssecretary review mapping %s`", h.prefix, result.MappingDraft.ID))

	return h.sendMessage(ctx, roomID, sb.String())
}

func (h *SecretaryCommandHandler) handleReviewMapping(ctx context.Context, roomID string, args []string) error {
	if len(args) == 0 {
		return h.sendMessage(ctx, roomID, "❌ Usage: "+h.prefix+"secretary review mapping <draft_id>")
	}

	draftID := args[0]

	if h.store == nil {
		return h.sendMessage(ctx, roomID, "❌ Store not configured")
	}

	templates, err := h.store.ListTemplates(ctx, TemplateFilter{})
	if err != nil {
		return h.sendMessage(ctx, roomID, fmt.Sprintf("❌ Failed to search templates: %v", err))
	}

	var foundTemplate *TaskTemplate
	for i := range templates {
		if templates[i].ID == draftID || strings.Contains(templates[i].Name, draftID) {
			foundTemplate = &templates[i]
			break
		}
	}

	if foundTemplate == nil {
		return h.sendMessage(ctx, roomID, fmt.Sprintf("❌ Mapping draft not found: %s", draftID))
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("📋 **Mapping Draft Review**\n\n"))
	sb.WriteString(fmt.Sprintf("**ID:** `%s`\n", foundTemplate.ID))
	sb.WriteString(fmt.Sprintf("**Name:** %s\n", foundTemplate.Name))
	if foundTemplate.Description != "" {
		sb.WriteString(fmt.Sprintf("**Description:** %s\n", foundTemplate.Description))
	}
	sb.WriteString(fmt.Sprintf("**Created:** %s\n", foundTemplate.CreatedAt.Format("2006-01-02 15:04")))
	sb.WriteString(fmt.Sprintf("**Steps:** %d\n\n", len(foundTemplate.Steps)))

	sb.WriteString("**Field Mappings:**\n")
	for _, step := range foundTemplate.Steps {
		sb.WriteString(fmt.Sprintf("  %d. **%s** (`%s`)\n", step.Order+1, step.Name, step.StepID))
	}

	if len(foundTemplate.PIIRefs) > 0 {
		sb.WriteString(fmt.Sprintf("\n**PII Fields Required:** %d\n", len(foundTemplate.PIIRefs)))
		for _, ref := range foundTemplate.PIIRefs {
			sb.WriteString(fmt.Sprintf("  • %s\n", ref))
		}
	}

	sb.WriteString(fmt.Sprintf("\nTo confirm: `%ssecretary confirm mapping %s`", h.prefix, draftID))

	return h.sendMessage(ctx, roomID, sb.String())
}

func (h *SecretaryCommandHandler) handleConfirmMapping(ctx context.Context, roomID, userID string, args []string) error {
	if h.learnWebsite == nil {
		return h.sendMessage(ctx, roomID, "❌ Learn Website service not configured")
	}

	if len(args) == 0 {
		return h.sendMessage(ctx, roomID, "❌ Usage: "+h.prefix+"secretary confirm mapping <draft_id> [field1=value1 field2=value2 ...]")
	}

	draftID := args[0]

	fieldOverrides := make(map[string]string)
	for _, arg := range args[1:] {
		parts := strings.SplitN(arg, "=", 2)
		if len(parts) == 2 {
			fieldOverrides[parts[0]] = parts[1]
		}
	}

	h.sendMessage(ctx, roomID, fmt.Sprintf("⏳ Confirming mapping `%s`...", draftID))

	mapping := &ConfirmedMapping{
		DraftID:       draftID,
		TemplateName:  draftID,
		ConfirmedBy:   userID,
		ConfirmedAt:   timeNow(),
		FieldMappings: []ConfirmedFieldMapping{},
	}

	if err := h.learnWebsite.ConfirmMapping(ctx, mapping); err != nil {
		return h.sendMessage(ctx, roomID, fmt.Sprintf("❌ Failed to confirm mapping: %v", err))
	}

	msg := fmt.Sprintf("✅ Mapping confirmed and template created\n\n**Template ID:** `%s`\n**Created by:** %s",
		draftID, userID)

	return h.sendMessage(ctx, roomID, msg)
}

func (h *SecretaryCommandHandler) handleRunBlindFill(ctx context.Context, roomID, userID string, args []string) error {
	if h.blindFill == nil {
		return h.sendMessage(ctx, roomID, "❌ BlindFill executor not configured")
	}

	if len(args) == 0 {
		return h.sendMessage(ctx, roomID, "❌ Usage: "+h.prefix+"secretary run blindfill <template_id|workflow_id> [url] [dryrun]")
	}

	templateID := args[0]
	var targetURL string
	dryRun := false

	for _, arg := range args[1:] {
		if arg == "dryrun" || arg == "--dry-run" {
			dryRun = true
		} else if strings.HasPrefix(arg, "http://") || strings.HasPrefix(arg, "https://") {
			targetURL = arg
		}
	}

	h.sendMessage(ctx, roomID, fmt.Sprintf("⏳ Starting BlindFill execution for template `%s`...", templateID))

	req := &BlindFillRequest{
		TemplateID:      templateID,
		TargetURL:       targetURL,
		Initiator:       userID,
		DryRun:          dryRun,
		Timeout:         120000,
		ContinueOnError: false,
	}

	result, err := h.blindFill.Execute(ctx, req)
	if err != nil {
		return h.sendMessage(ctx, roomID, fmt.Sprintf("❌ BlindFill execution failed: %v", err))
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("📋 **BlindFill Result**\n\n"))
	sb.WriteString(fmt.Sprintf("**Execution ID:** `%s`\n", result.ExecutionID))
	sb.WriteString(fmt.Sprintf("**Status:** %s\n", result.Status))
	sb.WriteString(fmt.Sprintf("**Duration:** %dms\n\n", result.Duration))

	if len(result.StepResults) > 0 {
		sb.WriteString(fmt.Sprintf("**Steps Executed:** %d\n", len(result.StepResults)))
		for _, step := range result.StepResults {
			statusIcon := "✅"
			if step.Status == "error" {
				statusIcon = "❌"
			} else if step.Status == "skipped" {
				statusIcon = "⏭️"
			}
			sb.WriteString(fmt.Sprintf("  %s %s (%dms)\n", statusIcon, step.StepName, step.ExecutionTime))
		}
	}

	if len(result.FilledFields) > 0 {
		sb.WriteString(fmt.Sprintf("\n**Fields Filled:** %d\n", len(result.FilledFields)))
	}

	if result.Error != "" {
		sb.WriteString(fmt.Sprintf("\n❌ **Error:** %s\n", result.Error))
	}

	if result.ApprovalRequired {
		sb.WriteString(fmt.Sprintf("\n⚠️ **Approval Required**\n"))
		sb.WriteString(fmt.Sprintf("**Request ID:** `%s`\n", result.ApprovalRequestID))
		if len(result.RequiredPIIFields) > 0 {
			sb.WriteString("**PII Fields:**\n")
			for _, field := range result.RequiredPIIFields {
				sb.WriteString(fmt.Sprintf("  • %s\n", field))
			}
		}
	}

	if len(result.Warnings) > 0 {
		sb.WriteString(fmt.Sprintf("\n⚠️ **Warnings:**\n"))
		for _, w := range result.Warnings {
			sb.WriteString(fmt.Sprintf("  • %s\n", w))
		}
	}

	return h.sendMessage(ctx, roomID, sb.String())
}

func (h *SecretaryCommandHandler) handleListTrustPolicies(ctx context.Context, roomID string) error {
	if h.trustEngine == nil {
		return h.sendMessage(ctx, roomID, "❌ Trust engine not configured")
	}

	policies, err := h.trustEngine.ListPolicies(ctx, true)
	if err != nil {
		return h.sendMessage(ctx, roomID, fmt.Sprintf("❌ Failed to list trust policies: %v", err))
	}

	if len(policies) == 0 {
		return h.sendMessage(ctx, roomID, "No trust policies found.")
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("🔐 **Trust Policies** (%d active)\n\n", len(policies)))

	for _, p := range policies {
		status := "✅ active"
		if p.RevokedAt != nil {
			status = "❌ revoked"
		} else if p.ExpiresAt != nil && timeNow().After(*p.ExpiresAt) {
			status = "⏰ expired"
		}

		sb.WriteString(fmt.Sprintf("**%s** `%s` - %s\n", p.Name, p.ID, status))
		if p.Description != "" {
			sb.WriteString(fmt.Sprintf("  %s\n", p.Description))
		}

		sb.WriteString("  Scope: ")
		if len(p.Scope.TemplateIDs) > 0 {
			sb.WriteString(fmt.Sprintf("%d templates, ", len(p.Scope.TemplateIDs)))
		}
		if len(p.Scope.Initiators) > 0 {
			sb.WriteString(fmt.Sprintf("%d initiators, ", len(p.Scope.Initiators)))
		}
		if len(p.AllowedPIIRefs) > 0 {
			sb.WriteString(fmt.Sprintf("%d PII refs allowed", len(p.AllowedPIIRefs)))
		}
		sb.WriteString("\n")

		if p.MaxExecutions > 0 {
			sb.WriteString(fmt.Sprintf("  Executions: %d/%d\n", p.ExecutionCount, p.MaxExecutions))
		}
		if p.ExpiresAt != nil {
			sb.WriteString(fmt.Sprintf("  Expires: %s\n", p.ExpiresAt.Format("2006-01-02 15:04")))
		}
		sb.WriteString("\n")
	}

	return h.sendMessage(ctx, roomID, sb.String())
}

func (h *SecretaryCommandHandler) handleCreateTrustPolicy(ctx context.Context, roomID, userID string, args []string) error {
	if h.trustEngine == nil {
		return h.sendMessage(ctx, roomID, "❌ Trust engine not configured")
	}

	if len(args) == 0 {
		return h.sendMessage(ctx, roomID, fmt.Sprintf("❌ Usage: %ssecretary trust create <name> [template=...] [initiator=...] [expires=...] [max_executions=...]", h.prefix))
	}

	name := args[0]

	policy := &TrustedWorkflowPolicy{
		Name:      name,
		CreatedBy: userID,
		Scope:     TrustScope{},
	}

	for _, arg := range args[1:] {
		parts := strings.SplitN(arg, "=", 2)
		if len(parts) != 2 {
			continue
		}
		key, value := parts[0], parts[1]

		switch key {
		case "template":
			policy.Scope.TemplateIDs = []string{value}
		case "initiator":
			policy.Scope.Initiators = []string{value}
		case "subject":
			policy.Scope.Subjects = []string{value}
		case "pii_ref", "pii":
			policy.AllowedPIIRefs = []string{value}
		case "pii_class":
			policy.AllowedPIIClasses = []string{value}
		case "domain":
			policy.Scope.AllowedDomains = []string{value}
		}
	}

	if err := h.trustEngine.CreatePolicy(ctx, policy); err != nil {
		return h.sendMessage(ctx, roomID, fmt.Sprintf("❌ Failed to create trust policy: %v", err))
	}

	msg := fmt.Sprintf("✅ Trust policy created\n\n**ID:** `%s`\n**Name:** %s\n**Created by:** %s",
		policy.ID, policy.Name, userID)

	return h.sendMessage(ctx, roomID, msg)
}

func (h *SecretaryCommandHandler) handleRevokeTrustPolicy(ctx context.Context, roomID, userID string, args []string) error {
	if h.trustEngine == nil {
		return h.sendMessage(ctx, roomID, "❌ Trust engine not configured")
	}

	if len(args) == 0 {
		return h.sendMessage(ctx, roomID, "❌ Usage: "+h.prefix+"secretary trust revoke <policy_id> [reason]")
	}

	policyID := args[0]
	reason := "Revoked via command"
	if len(args) > 1 {
		reason = strings.Join(args[1:], " ")
	}

	if err := h.trustEngine.RevokePolicy(ctx, policyID, userID, reason); err != nil {
		return h.sendMessage(ctx, roomID, fmt.Sprintf("❌ Failed to revoke trust policy: %v", err))
	}

	msg := fmt.Sprintf("✅ Trust policy revoked\n\n**ID:** `%s`\n**Revoked by:** %s\n**Reason:** %s",
		policyID, userID, reason)

	return h.sendMessage(ctx, roomID, msg)
}

func (h *SecretaryCommandHandler) handleCreateContact(ctx context.Context, roomID, userID string, args []string) error {
	if h.rolodex == nil {
		return h.sendMessage(ctx, roomID, "❌ Rolodex service not configured")
	}

	if len(args) == 0 {
		return h.sendMessage(ctx, roomID, "❌ Usage: "+h.prefix+"secretary contact create <name> [company=...] [relationship=...] [phone=...] [email=...] [address=...] [notes=...]")
	}

	req := &CreateRequest{
		Name:      args[0],
		CreatedBy: userID,
	}

	for _, arg := range args[1:] {
		parts := strings.SplitN(arg, "=", 2)
		if len(parts) != 2 {
			continue
		}
		key, value := parts[0], parts[1]

		switch key {
		case "company":
			req.Company = value
		case "relationship":
			req.Relationship = value
		case "phone":
			req.Phone = value
		case "email":
			req.Email = value
		case "address":
			req.Address = value
		case "notes":
			req.Notes = value
		}
	}

	contact, err := h.rolodex.CreateContact(ctx, req)
	if err != nil {
		return h.sendMessage(ctx, roomID, fmt.Sprintf("❌ Failed to create contact: %v", err))
	}

	msg := fmt.Sprintf("✅ Contact created\n\n**ID:** `%s`\n**Name:** %s\n**Company:** %s\n**Relationship:** %s",
		contact.ID, contact.Name, contact.Company, contact.Relationship)
	if contact.Phone != "" {
		msg += fmt.Sprintf("\n**Phone:** %s", contact.Phone)
	}
	if contact.Email != "" {
		msg += fmt.Sprintf("\n**Email:** %s", contact.Email)
	}

	return h.sendMessage(ctx, roomID, msg)
}

func (h *SecretaryCommandHandler) handleGetContact(ctx context.Context, roomID, contactID string) error {
	if h.rolodex == nil {
		return h.sendMessage(ctx, roomID, "❌ Rolodex service not configured")
	}

	contact, err := h.rolodex.GetContact(ctx, contactID)
	if err != nil {
		return h.sendMessage(ctx, roomID, fmt.Sprintf("❌ Failed to get contact: %v", err))
	}

	msg := fmt.Sprintf(`📋 **Contact**

**ID:** %s
**Name:** %s
**Company:** %s
**Relationship:** %s
**Created:** %s
`,
		contact.ID, contact.Name, contact.Company, contact.Relationship,
		contact.CreatedAt.Format("2006-01-02 15:04"))

	if contact.Phone != "" {
		msg += fmt.Sprintf("\n**Phone:** %s", contact.Phone)
	}
	if contact.Email != "" {
		msg += fmt.Sprintf("\n**Email:** %s", contact.Email)
	}
	if contact.Address != "" {
		msg += fmt.Sprintf("\n**Address:** %s", contact.Address)
	}
	if contact.Notes != "" {
		msg += fmt.Sprintf("\n**Notes:** %s", contact.Notes)
	}

	return h.sendMessage(ctx, roomID, msg)
}

func (h *SecretaryCommandHandler) handleListContacts(ctx context.Context, roomID string, args []string) error {
	if h.rolodex == nil {
		return h.sendMessage(ctx, roomID, "❌ Rolodex service not configured")
	}

	filter := ContactFilter{}

	for _, arg := range args {
		parts := strings.SplitN(arg, "=", 2)
		if len(parts) != 2 {
			continue
		}
		key, value := parts[0], parts[1]

		switch key {
		case "name":
			filter.Name = value
		case "company":
			filter.Company = value
		case "relationship":
			filter.Relationship = value
		}
	}

	contacts, err := h.rolodex.ListContacts(ctx, filter)
	if err != nil {
		return h.sendMessage(ctx, roomID, fmt.Sprintf("❌ Failed to list contacts: %v", err))
	}

	if len(contacts) == 0 {
		return h.sendMessage(ctx, roomID, "No contacts found.")
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("📋 **Contacts** (%d)\n\n", len(contacts)))

	for _, contact := range contacts {
		sb.WriteString(fmt.Sprintf("**%s** `%s` - %s", contact.Name, contact.ID, contact.Company))
		if contact.Relationship != "" {
			sb.WriteString(fmt.Sprintf(" (%s)", contact.Relationship))
		}
		sb.WriteString("\n")
	}

	return h.sendMessage(ctx, roomID, sb.String())
}

func (h *SecretaryCommandHandler) handleUpdateContact(ctx context.Context, roomID, userID string, args []string) error {
	if h.rolodex == nil {
		return h.sendMessage(ctx, roomID, "❌ Rolodex service not configured")
	}

	if len(args) == 0 {
		return h.sendMessage(ctx, roomID, "❌ Usage: "+h.prefix+"secretary contact update <id> [name=...] [company=...] [relationship=...] [phone=...] [email=...] [address=...] [notes=...]")
	}

	contactID := args[0]
	req := &UpdateRequest{
		ID:        contactID,
		UpdatedBy: userID,
	}

	for _, arg := range args[1:] {
		parts := strings.SplitN(arg, "=", 2)
		if len(parts) != 2 {
			continue
		}
		key, value := parts[0], parts[1]

		switch key {
		case "name":
			req.Name = value
		case "company":
			req.Company = value
		case "relationship":
			req.Relationship = value
		case "phone":
			req.Phone = value
		case "email":
			req.Email = value
		case "address":
			req.Address = value
		case "notes":
			req.Notes = value
		}
	}

	contact, err := h.rolodex.UpdateContact(ctx, req)
	if err != nil {
		return h.sendMessage(ctx, roomID, fmt.Sprintf("❌ Failed to update contact: %v", err))
	}

	msg := fmt.Sprintf("✅ Contact updated\n\n**ID:** `%s`\n**Name:** %s\n**Company:** %s",
		contact.ID, contact.Name, contact.Company)

	return h.sendMessage(ctx, roomID, msg)
}

func (h *SecretaryCommandHandler) handleDeleteContact(ctx context.Context, roomID, contactID string) error {
	if h.rolodex == nil {
		return h.sendMessage(ctx, roomID, "❌ Rolodex service not configured")
	}

	if err := h.rolodex.DeleteContact(ctx, contactID); err != nil {
		return h.sendMessage(ctx, roomID, fmt.Sprintf("❌ Failed to delete contact: %v", err))
	}

	return h.sendMessage(ctx, roomID, fmt.Sprintf("✅ Contact deleted\n\n**ID:** `%s`", contactID))
}

func (h *SecretaryCommandHandler) handleSearchContacts(ctx context.Context, roomID, query string) error {
	if h.rolodex == nil {
		return h.sendMessage(ctx, roomID, "❌ Rolodex service not configured")
	}

	contacts, err := h.rolodex.SearchContacts(ctx, query)
	if err != nil {
		return h.sendMessage(ctx, roomID, fmt.Sprintf("❌ Failed to search contacts: %v", err))
	}

	if len(contacts) == 0 {
		return h.sendMessage(ctx, roomID, fmt.Sprintf("No contacts found matching: %s", query))
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("🔍 **Search Results** (%d)\n\n", len(contacts)))

	for _, contact := range contacts {
		sb.WriteString(fmt.Sprintf("**%s** `%s` - %s", contact.Name, contact.ID, contact.Company))
		if contact.Relationship != "" {
			sb.WriteString(fmt.Sprintf(" (%s)", contact.Relationship))
		}
		sb.WriteString("\n")
		if contact.Phone != "" {
			sb.WriteString(fmt.Sprintf("  Phone: %s\n", contact.Phone))
		}
		if contact.Email != "" {
			sb.WriteString(fmt.Sprintf("  Email: %s\n", contact.Email))
		}
	}

	return h.sendMessage(ctx, roomID, sb.String())
}

func (h *SecretaryCommandHandler) handleWebDAVList(ctx context.Context, roomID string, args []string) error {
	if h.webdav == nil {
		return h.sendMessage(ctx, roomID, "❌ WebDAV service not configured. Check config.toml.")
	}

	if len(args) == 0 {
		return h.sendMessage(ctx, roomID, "❌ Usage: "+h.prefix+"secretary webdav list <url> [username=...] [password=...]")
	}

	url := args[0]
	params := map[string]interface{}{
		"operation": "list",
		"url":       url,
	}

	for _, arg := range args[1:] {
		parts := strings.SplitN(arg, "=", 2)
		if len(parts) == 2 {
			key := strings.TrimSpace(parts[0])
			value := strings.TrimSpace(parts[1])
			switch key {
			case "username":
				params["username"] = value
			case "password":
				params["password"] = value
			}
		}
	}

	result, err := h.webdav.ExecuteWebDAV(ctx, params)
	if err != nil {
		return h.sendMessage(ctx, roomID, fmt.Sprintf("❌ Failed to list WebDAV directory: %v", err))
	}

	var sb strings.Builder
	sb.WriteString("✅ WebDAV Directory Listing\n\n")
	sb.WriteString(fmt.Sprintf("**URL:** %s\n\n", url))

	if resultMap, ok := result.(map[string]interface{}); ok {
		if entries, ok := resultMap["entries"].([]interface{}); ok && len(entries) > 0 {
			sb.WriteString(fmt.Sprintf("**%d items:**\n\n", len(entries)))
			for _, entry := range entries {
				if entryMap, ok := entry.(map[string]interface{}); ok {
					name, _ := entryMap["name"].(string)
					isDir, _ := entryMap["is_directory"].(bool)
					icon := "📄"
					if isDir {
						icon = "📁"
					}
					sb.WriteString(fmt.Sprintf("  %s %s\n", icon, name))
				}
			}
		} else {
			sb.WriteString("**Empty directory**\n")
		}
	}

	return h.sendMessage(ctx, roomID, sb.String())
}

func (h *SecretaryCommandHandler) handleWebDAVGet(ctx context.Context, roomID string, args []string) error {
	if h.webdav == nil {
		return h.sendMessage(ctx, roomID, "❌ WebDAV service not configured. Check config.toml.")
	}

	if len(args) == 0 {
		return h.sendMessage(ctx, roomID, "❌ Usage: "+h.prefix+"secretary webdav get <url> [username=...] [password=...]")
	}

	url := args[0]
	params := map[string]interface{}{
		"operation": "get",
		"url":       url,
	}

	for _, arg := range args[1:] {
		parts := strings.SplitN(arg, "=", 2)
		if len(parts) == 2 {
			key := strings.TrimSpace(parts[0])
			value := strings.TrimSpace(parts[1])
			switch key {
			case "username":
				params["username"] = value
			case "password":
				params["password"] = value
			}
		}
	}

	result, err := h.webdav.ExecuteWebDAV(ctx, params)
	if err != nil {
		return h.sendMessage(ctx, roomID, fmt.Sprintf("❌ Failed to get file from WebDAV: %v", err))
	}

	var sb strings.Builder
	sb.WriteString("✅ File Retrieved\n\n")
	sb.WriteString(fmt.Sprintf("**URL:** %s\n\n", url))

	if resultMap, ok := result.(map[string]interface{}); ok {
		if content, ok := resultMap["content"].([]byte); ok {
			sb.WriteString("**Content:**\n```\n")
			sb.WriteString(string(content))
			sb.WriteString("\n```")
		} else {
			sb.WriteString("**No content**\n")
		}
	}

	return h.sendMessage(ctx, roomID, sb.String())
}

func (h *SecretaryCommandHandler) handleWebDAVPut(ctx context.Context, roomID string, args []string) error {
	if h.webdav == nil {
		return h.sendMessage(ctx, roomID, "❌ WebDAV service not configured. Check config.toml.")
	}

	if len(args) < 2 {
		return h.sendMessage(ctx, roomID, "❌ Usage: "+h.prefix+"secretary webdav put <url> <content> [username=...] [password=...] [content_type=...]")
	}

	url := args[0]
	content := args[1]
	params := map[string]interface{}{
		"operation":      "put",
		"url":            url,
		"content":        []byte(content),
		"content_length": int64(len(content)),
	}

	for _, arg := range args[2:] {
		parts := strings.SplitN(arg, "=", 2)
		if len(parts) == 2 {
			key := strings.TrimSpace(parts[0])
			value := strings.TrimSpace(parts[1])
			switch key {
			case "username":
				params["username"] = value
			case "password":
				params["password"] = value
			case "content_type":
				params["content_type"] = value
			}
		}
	}

	result, err := h.webdav.ExecuteWebDAV(ctx, params)
	if err != nil {
		return h.sendMessage(ctx, roomID, fmt.Sprintf("❌ Failed to upload file to WebDAV: %v", err))
	}

	var sb strings.Builder
	sb.WriteString("✅ File Uploaded\n\n")
	sb.WriteString(fmt.Sprintf("**URL:** %s\n", url))
	sb.WriteString(fmt.Sprintf("**Size:** %d bytes\n\n", len(content)))

	if resultMap, ok := result.(map[string]interface{}); ok {
		if newURL, ok := resultMap["new_url"].(string); ok {
			sb.WriteString(fmt.Sprintf("**New URL:** %s\n", newURL))
		}
	}

	return h.sendMessage(ctx, roomID, sb.String())
}

func (h *SecretaryCommandHandler) handleWebDAVDelete(ctx context.Context, roomID string, args []string) error {
	if h.webdav == nil {
		return h.sendMessage(ctx, roomID, "❌ WebDAV service not configured. Check config.toml.")
	}

	if len(args) == 0 {
		return h.sendMessage(ctx, roomID, "❌ Usage: "+h.prefix+"secretary webdav delete <url> [username=...] [password=...]")
	}

	url := args[0]
	params := map[string]interface{}{
		"operation": "delete",
		"url":       url,
	}

	for _, arg := range args[1:] {
		parts := strings.SplitN(arg, "=", 2)
		if len(parts) == 2 {
			key := strings.TrimSpace(parts[0])
			value := strings.TrimSpace(parts[1])
			switch key {
			case "username":
				params["username"] = value
			case "password":
				params["password"] = value
			}
		}
	}

	_, err := h.webdav.ExecuteWebDAV(ctx, params)
	if err != nil {
		return h.sendMessage(ctx, roomID, fmt.Sprintf("❌ Failed to delete file from WebDAV: %v", err))
	}

	return h.sendMessage(ctx, roomID, fmt.Sprintf("✅ File deleted\n\n**URL:** %s", url))
}

func (h *SecretaryCommandHandler) handleCalendarList(ctx context.Context, roomID string, args []string) error {
	if h.calendar == nil {
		return h.sendMessage(ctx, roomID, "❌ Calendar service not configured. Check config.toml.")
	}

	if len(args) == 0 {
		return h.sendMessage(ctx, roomID, "❌ Usage: "+h.prefix+"secretary calendar list <calendar_url> [username=...] [password=...]")
	}

	calendarURL := args[0]
	params := map[string]interface{}{
		"operation":    "list_calendars",
		"calendar_url": calendarURL,
	}

	for _, arg := range args[1:] {
		parts := strings.SplitN(arg, "=", 2)
		if len(parts) == 2 {
			key := strings.TrimSpace(parts[0])
			value := strings.TrimSpace(parts[1])
			switch key {
			case "username":
				params["username"] = value
			case "password":
				params["password"] = value
			}
		}
	}

	result, err := h.calendar.ExecuteCalendar(ctx, params)
	if err != nil {
		return h.sendMessage(ctx, roomID, fmt.Sprintf("❌ Failed to list calendars: %v", err))
	}

	var sb strings.Builder
	sb.WriteString("✅ Calendars Listed\n\n")
	sb.WriteString(fmt.Sprintf("**Calendar URL:** %s\n\n", calendarURL))

	if resultMap, ok := result.(map[string]interface{}); ok {
		if events, ok := resultMap["events"].([]interface{}); ok && len(events) > 0 {
			sb.WriteString(fmt.Sprintf("**%d events found:**\n\n", len(events)))
			for _, event := range events {
				if eventMap, ok := event.(map[string]interface{}); ok {
					id, _ := eventMap["id"].(string)
					title, _ := eventMap["title"].(string)
					sb.WriteString(fmt.Sprintf("  • **%s** (%s)\n", title, id))
				}
			}
		} else {
			sb.WriteString("**No events found**\n")
		}
	}

	return h.sendMessage(ctx, roomID, sb.String())
}

func (h *SecretaryCommandHandler) handleCalendarCreate(ctx context.Context, roomID string, args []string) error {
	if h.calendar == nil {
		return h.sendMessage(ctx, roomID, "❌ Calendar service not configured. Check config.toml.")
	}

	if len(args) < 4 {
		return h.sendMessage(ctx, roomID, "❌ Usage: "+h.prefix+"secretary calendar create <title> start=\"<time>\" end=\"<time>\" [calendar_url=...] [username=...] [password=...] [description=...] [location=...] [attendees=...]")
	}

	title := args[0]
	params := map[string]interface{}{
		"operation":    "create_event",
		"title":        title,
		"calendar_url": "http://localhost:8080/calendars/default/",
	}

	for _, arg := range args[1:] {
		parts := strings.SplitN(arg, "=", 2)
		if len(parts) == 2 {
			key := strings.TrimSpace(parts[0])
			value := strings.TrimSpace(parts[1])
			switch key {
			case "start", "start_time":
				params["start_time"] = value
			case "end", "end_time":
				params["end_time"] = value
			case "calendar_url":
				params["calendar_url"] = value
			case "username":
				params["username"] = value
			case "password":
				params["password"] = value
			case "description":
				params["description"] = value
			case "location":
				params["location"] = value
			case "attendees":
				attendees := strings.Split(value, ",")
				params["attendees"] = attendees
			}
		}
	}

	result, err := h.calendar.ExecuteCalendar(ctx, params)
	if err != nil {
		return h.sendMessage(ctx, roomID, fmt.Sprintf("❌ Failed to create calendar event: %v", err))
	}

	var sb strings.Builder
	sb.WriteString("✅ Event Created\n\n")
	sb.WriteString(fmt.Sprintf("**Title:** %s\n", title))

	if resultMap, ok := result.(map[string]interface{}); ok {
		if eventID, ok := resultMap["event_id"].(string); ok {
			sb.WriteString(fmt.Sprintf("**Event ID:** %s\n", eventID))
		}
		if conflicts, ok := resultMap["conflicts_detected"].(bool); ok && conflicts {
			sb.WriteString("\n⚠️ **Conflict detected:** There are overlapping events.\n")
		}
	}

	return h.sendMessage(ctx, roomID, sb.String())
}

func (h *SecretaryCommandHandler) handleCalendarGetEvents(ctx context.Context, roomID string, args []string) error {
	if h.calendar == nil {
		return h.sendMessage(ctx, roomID, "❌ Calendar service not configured. Check config.toml.")
	}

	if len(args) == 0 {
		return h.sendMessage(ctx, roomID, "❌ Usage: "+h.prefix+"secretary calendar get_events <calendar_url> [username=...] [password=...]")
	}

	calendarURL := args[0]
	params := map[string]interface{}{
		"operation":    "get_events",
		"calendar_url": calendarURL,
	}

	for _, arg := range args[1:] {
		parts := strings.SplitN(arg, "=", 2)
		if len(parts) == 2 {
			key := strings.TrimSpace(parts[0])
			value := strings.TrimSpace(parts[1])
			switch key {
			case "username":
				params["username"] = value
			case "password":
				params["password"] = value
			}
		}
	}

	result, err := h.calendar.ExecuteCalendar(ctx, params)
	if err != nil {
		return h.sendMessage(ctx, roomID, fmt.Sprintf("❌ Failed to get calendar events: %v", err))
	}

	var sb strings.Builder
	sb.WriteString("✅ Events Retrieved\n\n")
	sb.WriteString(fmt.Sprintf("**Calendar URL:** %s\n\n", calendarURL))

	if resultMap, ok := result.(map[string]interface{}); ok {
		if events, ok := resultMap["events"].([]interface{}); ok && len(events) > 0 {
			sb.WriteString(fmt.Sprintf("**%d events:**\n\n", len(events)))
			for _, event := range events {
				if eventMap, ok := event.(map[string]interface{}); ok {
					id, _ := eventMap["id"].(string)
					title, _ := eventMap["title"].(string)
					startTime, _ := eventMap["start_time"].(string)
					endTime, _ := eventMap["end_time"].(string)
					location, _ := eventMap["location"].(string)
					sb.WriteString(fmt.Sprintf("  • **%s**\n", title))
					sb.WriteString(fmt.Sprintf("    ID: %s\n", id))
					if startTime != "" {
						sb.WriteString(fmt.Sprintf("    Start: %s\n", startTime))
					}
					if endTime != "" {
						sb.WriteString(fmt.Sprintf("    End: %s\n", endTime))
					}
					if location != "" {
						sb.WriteString(fmt.Sprintf("    Location: %s\n", location))
					}
					sb.WriteString("\n")
				}
			}
		} else {
			sb.WriteString("**No events found**\n")
		}
	}

	return h.sendMessage(ctx, roomID, sb.String())
}

func (h *SecretaryCommandHandler) handleCalendarGetEvent(ctx context.Context, roomID string, args []string) error {
	if h.calendar == nil {
		return h.sendMessage(ctx, roomID, "❌ Calendar service not configured. Check config.toml.")
	}

	if len(args) == 0 {
		return h.sendMessage(ctx, roomID, "❌ Usage: "+h.prefix+"secretary calendar get_event <calendar_url> uid=<event_uid> [username=...] [password=...]")
	}

	calendarURL := args[0]
	params := map[string]interface{}{
		"operation":    "get_event",
		"calendar_url": calendarURL,
	}

	for _, arg := range args[1:] {
		parts := strings.SplitN(arg, "=", 2)
		if len(parts) == 2 {
			key := strings.TrimSpace(parts[0])
			value := strings.TrimSpace(parts[1])
			switch key {
			case "uid":
				params["event_data"] = map[string]interface{}{"uid": value}
			case "username":
				params["username"] = value
			case "password":
				params["password"] = value
			}
		}
	}

	result, err := h.calendar.ExecuteCalendar(ctx, params)
	if err != nil {
		return h.sendMessage(ctx, roomID, fmt.Sprintf("❌ Failed to get event: %v", err))
	}

	var sb strings.Builder
	sb.WriteString("✅ Event Retrieved\n\n")

	if resultMap, ok := result.(map[string]interface{}); ok {
		if events, ok := resultMap["events"].([]interface{}); ok && len(events) > 0 {
			if eventMap, ok := events[0].(map[string]interface{}); ok {
				title, _ := eventMap["title"].(string)
				startTime, _ := eventMap["start_time"].(string)
				endTime, _ := eventMap["end_time"].(string)
				location, _ := eventMap["location"].(string)
				description, _ := eventMap["description"].(string)

				sb.WriteString(fmt.Sprintf("**Title:** %s\n\n", title))
				if startTime != "" {
					sb.WriteString(fmt.Sprintf("**Start:** %s\n", startTime))
				}
				if endTime != "" {
					sb.WriteString(fmt.Sprintf("**End:** %s\n", endTime))
				}
				if location != "" {
					sb.WriteString(fmt.Sprintf("**Location:** %s\n", location))
				}
				if description != "" {
					sb.WriteString(fmt.Sprintf("\n**Description:**\n%s\n", description))
				}
			}
		} else {
			sb.WriteString("**Event not found**\n")
		}
	}

	return h.sendMessage(ctx, roomID, sb.String())
}

func (h *SecretaryCommandHandler) handleCalendarUpdate(ctx context.Context, roomID string, args []string) error {
	if h.calendar == nil {
		return h.sendMessage(ctx, roomID, "❌ Calendar service not configured. Check config.toml.")
	}

	if len(args) < 2 {
		return h.sendMessage(ctx, roomID, "❌ Usage: "+h.prefix+"secretary calendar update <calendar_url> uid=<event_uid> [start=\"...\"] [end=\"...\"] [title=...] [description=...] [location=...] [username=...] [password=...]")
	}

	calendarURL := args[0]
	params := map[string]interface{}{
		"operation":    "update_event",
		"calendar_url": calendarURL,
	}

	for _, arg := range args[1:] {
		parts := strings.SplitN(arg, "=", 2)
		if len(parts) == 2 {
			key := strings.TrimSpace(parts[0])
			value := strings.TrimSpace(parts[1])
			switch key {
			case "uid":
				params["event_data"] = map[string]interface{}{"uid": value}
			case "start", "start_time":
				params["start_time"] = value
			case "end", "end_time":
				params["end_time"] = value
			case "title":
				params["title"] = value
			case "description":
				params["description"] = value
			case "location":
				params["location"] = value
			case "username":
				params["username"] = value
			case "password":
				params["password"] = value
			}
		}
	}

	result, err := h.calendar.ExecuteCalendar(ctx, params)
	if err != nil {
		return h.sendMessage(ctx, roomID, fmt.Sprintf("❌ Failed to update event: %v", err))
	}

	var sb strings.Builder
	sb.WriteString("✅ Event Updated\n\n")

	if resultMap, ok := result.(map[string]interface{}); ok {
		if eventID, ok := resultMap["event_id"].(string); ok {
			sb.WriteString(fmt.Sprintf("**Event ID:** %s\n", eventID))
		}
	}

	return h.sendMessage(ctx, roomID, sb.String())
}

func (h *SecretaryCommandHandler) handleCalendarDelete(ctx context.Context, roomID string, args []string) error {
	if h.calendar == nil {
		return h.sendMessage(ctx, roomID, "❌ Calendar service not configured. Check config.toml.")
	}

	if len(args) < 2 {
		return h.sendMessage(ctx, roomID, "❌ Usage: "+h.prefix+"secretary calendar delete <calendar_url> uid=<event_uid> [username=...] [password=...]")
	}

	calendarURL := args[0]
	params := map[string]interface{}{
		"operation":    "delete_event",
		"calendar_url": calendarURL,
	}

	for _, arg := range args[1:] {
		parts := strings.SplitN(arg, "=", 2)
		if len(parts) == 2 {
			key := strings.TrimSpace(parts[0])
			value := strings.TrimSpace(parts[1])
			switch key {
			case "uid":
				params["event_data"] = map[string]interface{}{"uid": value}
			case "username":
				params["username"] = value
			case "password":
				params["password"] = value
			}
		}
	}

	result, err := h.calendar.ExecuteCalendar(ctx, params)
	if err != nil {
		return h.sendMessage(ctx, roomID, fmt.Sprintf("❌ Failed to delete event: %v", err))
	}

	var sb strings.Builder
	sb.WriteString("✅ Event Deleted\n\n")

	if resultMap, ok := result.(map[string]interface{}); ok {
		if eventID, ok := resultMap["event_id"].(string); ok {
			sb.WriteString(fmt.Sprintf("**Event ID:** %s\n", eventID))
		}
	}

	return h.sendMessage(ctx, roomID, sb.String())
}

func timeNow() time.Time {
	return time.Now()
}
