// Package studio provides the Agent Studio module for no-code agent creation.
// Business users can create specialized agents via Matrix commands or Dashboard UI
// without writing code or Docker commands.
package studio

import (
	"encoding/json"
	"time"

	"github.com/armorclaw/bridge/pkg/pii"
)

//=============================================================================
// Agent Definition
//=============================================================================

// AgentDefinition represents a no-code agent configuration.
// It defines what skills an agent has and what PII it can access.
type AgentDefinition struct {
	// ID is the unique identifier for this agent definition
	ID string `json:"id"`

	// Name is the human-readable name (e.g., "Contracts Agent")
	Name string `json:"name"`

	// Description explains what this agent does
	Description string `json:"description,omitempty"`

	// Skills is the list of skill IDs this agent can use
	Skills []string `json:"skills"`

	// PIIAccess is the list of PII field IDs this agent can access
	PIIAccess []string `json:"pii_access"`

	// ResourceTier defines resource limits: "low", "medium", "high"
	ResourceTier string `json:"resource_tier"`

	// CreatedBy is the Matrix user ID who created this agent
	CreatedBy string `json:"created_by"`

	// CreatedAt is when this definition was created
	CreatedAt time.Time `json:"created_at"`

	// UpdatedAt is when this definition was last modified
	UpdatedAt time.Time `json:"updated_at"`

	// IsActive indicates if this agent can be spawned
	IsActive bool `json:"is_active"`
}

// MarshalJSON custom marshals AgentDefinition for API responses
func (d *AgentDefinition) MarshalJSON() ([]byte, error) {
	type Alias AgentDefinition
	return json.Marshal(&struct {
		*Alias
		CreatedAt int64 `json:"created_at"`
		UpdatedAt int64 `json:"updated_at"`
	}{
		Alias:     (*Alias)(d),
		CreatedAt: d.CreatedAt.UnixMilli(),
		UpdatedAt: d.UpdatedAt.UnixMilli(),
	})
}

//=============================================================================
// Skill Registry
//=============================================================================

// Skill represents a callable capability that agents can use.
type Skill struct {
	// ID is the unique identifier (e.g., "pdf_generator")
	ID string `json:"id"`

	// Name is the human-readable name (e.g., "PDF Generator")
	Name string `json:"name"`

	// Description explains what this skill does
	Description string `json:"description"`

	// Category groups related skills (e.g., "document", "communication", "research")
	Category string `json:"category"`

	// ContainerImage is an optional custom Docker image for this skill
	ContainerImage string `json:"container_image,omitempty"`

	// RequiredEnvVars lists environment variables this skill needs
	RequiredEnvVars []string `json:"required_env_vars,omitempty"`

	// CreatedAt is when this skill was registered
	CreatedAt time.Time `json:"created_at"`
}

// SkillCategory represents a grouping of skills
type SkillCategory struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Icon        string `json:"icon,omitempty"`
}

// Default skill categories
var DefaultSkillCategories = []SkillCategory{
	{ID: "document", Name: "Documents", Description: "PDF, Word, and document processing", Icon: "📄"},
	{ID: "communication", Name: "Communication", Description: "Email, messaging, and notifications", Icon: "📧"},
	{ID: "research", Name: "Research", Description: "Web scraping and information gathering", Icon: "🔍"},
	{ID: "automation", Name: "Automation", Description: "Form filling and task automation", Icon: "🤖"},
	{ID: "productivity", Name: "Productivity", Description: "Calendar, tasks, and scheduling", Icon: "📅"},
	{ID: "data", Name: "Data", Description: "Database and data processing", Icon: "💾"},
}

//=============================================================================
// PII Registry
//=============================================================================

// PIIField represents an accessible data field with sensitivity classification.
type PIIField struct {
	// ID is the unique identifier (e.g., "client_ssn")
	ID string `json:"id"`

	// Name is the human-readable name (e.g., "Client SSN")
	Name string `json:"name"`

	// Description explains what this field contains
	Description string `json:"description"`

	// Sensitivity classification: "low", "medium", "high", "critical"
	Sensitivity string `json:"sensitivity"`

	// KeystoreKey is the path in the keystore to retrieve the value
	KeystoreKey string `json:"keystore_key,omitempty"`

	// RequiresApproval indicates if explicit approval is needed for access
	RequiresApproval bool `json:"requires_approval"`

	// CreatedAt is when this field was registered
	CreatedAt time.Time `json:"created_at"`
}

// SensitivityLevel is an alias for pii.SensitivityLevel.
// Use pii.SensitivityLevel directly for new code.
//
// Deprecated: Use pii.SensitivityLevel directly.
type SensitivityLevel = pii.SensitivityLevel

const (
	SensitivityLow      = pii.SensitivityLow
	SensitivityMedium   = pii.SensitivityMedium
	SensitivityHigh     = pii.SensitivityHigh
	SensitivityCritical = pii.SensitivityCritical
)

// RequiresApprovalForSensitivity returns true if the given sensitivity requires approval
func RequiresApprovalForSensitivity(level string) bool {
	switch level {
	case "high", "critical":
		return true
	default:
		return false
	}
}

//=============================================================================
// Resource Profiles
//=============================================================================

// ResourceProfile defines container resource limits for an agent tier.
type ResourceProfile struct {
	// Tier is the profile identifier: "low", "medium", "high"
	Tier string `json:"tier"`

	// MemoryMB is the memory limit in megabytes
	MemoryMB int `json:"memory_mb"`

	// CPUShares is the CPU weight (relative to other containers)
	CPUShares int `json:"cpu_shares"`

	// TimeoutSeconds is the maximum execution time
	TimeoutSeconds int `json:"timeout_seconds"`

	// MaxConcurrency is the maximum number of concurrent instances
	MaxConcurrency int `json:"max_concurrency"`

	// Description explains when to use this profile
	Description string `json:"description"`
}

// DefaultResourceProfiles provides standard resource configurations
var DefaultResourceProfiles = map[string]ResourceProfile{
	"low": {
		Tier:           "low",
		MemoryMB:       256,
		CPUShares:      512,
		TimeoutSeconds: 300,
		MaxConcurrency: 5,
		Description:    "Lightweight tasks, quick operations",
	},
	"medium": {
		Tier:           "medium",
		MemoryMB:       512,
		CPUShares:      1024,
		TimeoutSeconds: 600,
		MaxConcurrency: 3,
		Description:    "Standard tasks, moderate processing",
	},
	"high": {
		Tier:           "high",
		MemoryMB:       2048,
		CPUShares:      2048,
		TimeoutSeconds: 1800,
		MaxConcurrency: 1,
		Description:    "Heavy processing, complex operations",
	},
}

// GetProfile returns the resource profile for a tier, defaulting to medium
func GetProfile(tier string) ResourceProfile {
	if profile, ok := DefaultResourceProfiles[tier]; ok {
		return profile
	}
	return DefaultResourceProfiles["medium"]
}

//=============================================================================
// Agent Instance
//=============================================================================

// InstanceStatus represents the state of a running agent
type InstanceStatus string

const (
	StatusPending   InstanceStatus = "pending"
	StatusRunning   InstanceStatus = "running"
	StatusPaused    InstanceStatus = "paused"
	StatusCompleted InstanceStatus = "completed"
	StatusFailed    InstanceStatus = "failed"
	StatusCancelled InstanceStatus = "cancelled"
)

// AgentInstance tracks a running agent spawned from a definition.
type AgentInstance struct {
	// ID is the unique instance identifier
	ID string `json:"id"`

	// DefinitionID references the agent definition
	DefinitionID string `json:"definition_id"`

	// ContainerID is the Docker container ID
	ContainerID string `json:"container_id"`

	// Status is the current instance state
	Status InstanceStatus `json:"status"`

	// TaskDescription is what this instance was spawned to do
	TaskDescription string `json:"task_description,omitempty"`

	// SpawnedBy is the Matrix user ID who spawned this instance
	SpawnedBy string `json:"spawned_by"`

	// StartedAt is when the instance was created
	StartedAt *time.Time `json:"started_at,omitempty"`

	// CompletedAt is when the instance finished
	CompletedAt *time.Time `json:"completed_at,omitempty"`

	// ExitCode is the container exit code (if completed)
	ExitCode *int `json:"exit_code,omitempty"`

	// ErrorMessage describes any failure
	ErrorMessage string `json:"error_message,omitempty"`

	// RoomID is the Matrix room ID associated with this instance
	RoomID string `json:"room_id,omitempty"`
}

// MarshalJSON custom marshals AgentInstance for API responses
func (i *AgentInstance) MarshalJSON() ([]byte, error) {
	type Alias AgentInstance
	return json.Marshal(&struct {
		*Alias
		StartedAt   *int64 `json:"started_at,omitempty"`
		CompletedAt *int64 `json:"completed_at,omitempty"`
	}{
		Alias:       (*Alias)(i),
		StartedAt:   timeToMillis(i.StartedAt),
		CompletedAt: timeToMillis(i.CompletedAt),
	})
}

//=============================================================================
// Validation Results
//=============================================================================

// ValidationResult contains the outcome of a validation check
type ValidationResult struct {
	Valid   bool     `json:"valid"`
	Invalid []string `json:"invalid,omitempty"`
	Reason  string   `json:"reason,omitempty"`
}

// SkillValidationResult is the result of validating skill IDs
type SkillValidationResult struct {
	Valid      bool     `json:"valid"`
	InvalidIDs []string `json:"invalid_ids,omitempty"`
	Message    string   `json:"message,omitempty"`
}

// PIIValidationResult is the result of validating PII field IDs
type PIIValidationResult struct {
	Valid            bool     `json:"valid"`
	InvalidIDs       []string `json:"invalid_ids,omitempty"`
	RequiresApproval []string `json:"requires_approval,omitempty"`
	Message          string   `json:"message,omitempty"`
}

//=============================================================================
// Wizard State (for interactive creation)
//=============================================================================

// WizardStep represents the current step in agent creation wizard
type WizardStep int

const (
	WizardStepSkills WizardStep = iota + 1
	WizardStepPII
	WizardStepResources
	WizardStepConfirm
)

// WizardState tracks an in-progress agent creation wizard
type WizardState struct {
	UserID    string     `json:"user_id"`
	RoomID    string     `json:"room_id"`
	Step      WizardStep `json:"step"`
	Name      string     `json:"name"`
	Skills    []string   `json:"skills,omitempty"`
	PIIAccess []string   `json:"pii_access,omitempty"`
	Tier      string     `json:"tier"`
	ExpiresAt time.Time  `json:"expires_at"`
}

// IsExpired checks if the wizard session has expired
func (w *WizardState) IsExpired() bool {
	return time.Now().After(w.ExpiresAt)
}

//=============================================================================
// Statistics
//=============================================================================

// StudioStats provides overview statistics for the studio
type StudioStats struct {
	TotalDefinitions   int            `json:"total_definitions"`
	ActiveDefinitions  int            `json:"active_definitions"`
	TotalInstances     int            `json:"total_instances"`
	RunningInstances   int            `json:"running_instances"`
	SkillsAvailable    int            `json:"skills_available"`
	PIIFieldsAvailable int            `json:"pii_fields_available"`
	ByTier             map[string]int `json:"by_tier"`
}

//=============================================================================
// Helper Functions
//=============================================================================

func timeToMillis(t *time.Time) *int64 {
	if t == nil {
		return nil
	}
	ms := t.UnixMilli()
	return &ms
}
