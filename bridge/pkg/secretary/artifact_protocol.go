package secretary

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
)

//=============================================================================
// Artifact Status
//=============================================================================

// ArtifactStatus represents the lifecycle state of an artifact envelope.
type ArtifactStatus string

const (
	ArtifactPending    ArtifactStatus = "pending"
	ArtifactProcessing ArtifactStatus = "processing"
	ArtifactCompleted  ArtifactStatus = "completed"
	ArtifactFailed     ArtifactStatus = "failed"
	ArtifactExpired    ArtifactStatus = "expired"
)

// ValidArtifactStatuses is the set of allowed status values.
var ValidArtifactStatuses = map[ArtifactStatus]bool{
	ArtifactPending:    true,
	ArtifactProcessing: true,
	ArtifactCompleted:  true,
	ArtifactFailed:     true,
	ArtifactExpired:    true,
}

//=============================================================================
// Current Protocol Version
//=============================================================================

const (
	// ArtifactProtocolVersion is the current version of the Artifact Envelope Protocol.
	ArtifactProtocolVersion = "1.0"
)

// SupportedVersions is the set of protocol versions this bridge understands.
var SupportedVersions = map[string]bool{
	"1.0": true,
}

//=============================================================================
// Artifact Metadata
//=============================================================================

// ArtifactMetadata contains sanitized metadata about an artifact file.
type ArtifactMetadata struct {
	// MIMEType is the sanitized MIME type (e.g., "application/pdf").
	MIMEType string `json:"mime_type"`

	// Filename is the sanitized base filename (no path separators).
	Filename string `json:"filename"`

	// Tags are sanitized classification tags.
	Tags []string `json:"tags,omitempty"`

	// Source identifies the origin (e.g., "email", "browser", "sidecar").
	Source string `json:"source,omitempty"`

	// Destination identifies the target consumer.
	Destination string `json:"destination,omitempty"`

	// SizeBytes is the artifact size in bytes.
	SizeBytes int64 `json:"size_bytes"`
}

// Validation regexps
var (
	// mimeTypeRe validates standard MIME type format: type/subtype
	mimeTypeRe = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9!#$&\-^_.+]*\/[a-zA-Z0-9][a-zA-Z0-9!#$&\-^_.+]*$`)

	// sha256HexRe validates 64 lowercase hex characters.
	sha256HexRe = regexp.MustCompile(`^[a-f0-9]{64}$`)
)

//=============================================================================
// Artifact Envelope
//=============================================================================

// ArtifactEnvelope is the versioned container for an artifact in the v1.2
// Artifact Envelope Protocol. Every artifact exchanged between the bridge
// and ArmorChat must be wrapped in an envelope.
//
// Security rules (fail-closed):
//   - Owner and WorkflowID are required — reject without binding.
//   - Metadata fields are sanitized (MIME, filename, tags, source, destination).
//   - Checksum is SHA-256 for integrity only — NOT authentication.
type ArtifactEnvelope struct {
	// ID is a unique identifier for this artifact envelope.
	ID string `json:"id"`

	// Version is the protocol version (currently "1.0").
	Version string `json:"version"`

	// Status is the current lifecycle state.
	Status ArtifactStatus `json:"status"`

	// Metadata contains sanitized artifact metadata.
	Metadata ArtifactMetadata `json:"metadata"`

	// Owner is the Matrix user ID that owns this artifact. Required.
	Owner string `json:"owner"`

	// WorkflowID is the workflow this artifact belongs to. Required.
	WorkflowID string `json:"workflow_id"`

	// StepID optionally links to the specific workflow step.
	StepID string `json:"step_id,omitempty"`

	// Checksum is the SHA-256 hex digest of the artifact content.
	Checksum string `json:"checksum,omitempty"`

	// CreatedAt is when the envelope was created.
	CreatedAt time.Time `json:"created_at"`

	// UpdatedAt is when the envelope was last modified.
	UpdatedAt time.Time `json:"updated_at"`
}

// NewArtifactEnvelope creates a new artifact envelope with a generated UUID,
// current protocol version, and Pending status.
func NewArtifactEnvelope(owner, workflowID string, meta ArtifactMetadata) (*ArtifactEnvelope, error) {
	if owner == "" {
		return nil, fmt.Errorf("artifact: owner is required")
	}
	if workflowID == "" {
		return nil, fmt.Errorf("artifact: workflow_id is required")
	}

	now := time.Now().UTC()
	return &ArtifactEnvelope{
		ID:         uuid.New().String(),
		Version:    ArtifactProtocolVersion,
		Status:     ArtifactPending,
		Metadata:   meta,
		Owner:      owner,
		WorkflowID: workflowID,
		CreatedAt:  now,
		UpdatedAt:  now,
	}, nil
}

// Validate performs fail-closed validation on the envelope.
// Returns an error describing the first validation failure.
func (e *ArtifactEnvelope) Validate() error {
	// Owner binding required
	if e.Owner == "" {
		return fmt.Errorf("artifact: owner is required")
	}

	// Workflow binding required
	if e.WorkflowID == "" {
		return fmt.Errorf("artifact: workflow_id is required")
	}

	// ID required
	if e.ID == "" {
		return fmt.Errorf("artifact: id is required")
	}

	// Version must be supported
	if !SupportedVersions[e.Version] {
		return fmt.Errorf("artifact: unsupported protocol version %q", e.Version)
	}

	// Status must be valid
	if !ValidArtifactStatuses[e.Status] {
		return fmt.Errorf("artifact: invalid status %q", e.Status)
	}

	// Validate metadata
	if err := e.Metadata.Validate(); err != nil {
		return err
	}

	// Validate checksum format if present
	if e.Checksum != "" {
		if !sha256HexRe.MatchString(e.Checksum) {
			return fmt.Errorf("artifact: checksum must be 64 lowercase hex characters (SHA-256)")
		}
	}

	return nil
}

// SupportsVersion checks if a given protocol version is supported.
func SupportsVersion(v string) bool {
	return SupportedVersions[v]
}

// TransitionStatus moves the artifact to a new status.
// Returns error if the transition is invalid.
func (e *ArtifactEnvelope) TransitionStatus(newStatus ArtifactStatus) error {
	if !ValidArtifactStatuses[newStatus] {
		return fmt.Errorf("artifact: invalid target status %q", newStatus)
	}

	// Define allowed transitions
	allowed := allowedTransitions[e.Status]
	if !allowed[newStatus] {
		return fmt.Errorf("artifact: cannot transition from %q to %q", e.Status, newStatus)
	}

	e.Status = newStatus
	e.UpdatedAt = time.Now().UTC()
	return nil
}

// allowedTransitions defines the valid status state machine.
var allowedTransitions = map[ArtifactStatus]map[ArtifactStatus]bool{
	ArtifactPending: {
		ArtifactProcessing: true,
		ArtifactFailed:     true,
		ArtifactExpired:    true,
	},
	ArtifactProcessing: {
		ArtifactCompleted: true,
		ArtifactFailed:    true,
	},
	ArtifactCompleted: {
		ArtifactExpired: true,
	},
	ArtifactFailed: {
		ArtifactPending: true, // retry
	},
	ArtifactExpired: {},
}

// SetChecksum computes and sets the SHA-256 checksum from raw content bytes.
func (e *ArtifactEnvelope) SetChecksum(content []byte) {
	h := sha256.Sum256(content)
	e.Checksum = hex.EncodeToString(h[:])
	e.UpdatedAt = time.Now().UTC()
}

//=============================================================================
// Metadata Validation
//=============================================================================

// Validate sanitizes and validates all metadata fields.
func (m *ArtifactMetadata) Validate() error {
	// MIMEType validation (if provided)
	if m.MIMEType != "" {
		if !mimeTypeRe.MatchString(m.MIMEType) {
			return fmt.Errorf("artifact: invalid MIME type %q", m.MIMEType)
		}
	}

	// Filename validation (if provided)
	if m.Filename != "" {
		if strings.Contains(m.Filename, "..") {
			return fmt.Errorf("artifact: filename must not contain path traversal")
		}
		if strings.ContainsAny(m.Filename, "/\\") {
			return fmt.Errorf("artifact: filename must not contain path separators")
		}
	}

	// Tags sanitization
	for _, tag := range m.Tags {
		trimmed := strings.TrimSpace(tag)
		if trimmed == "" {
			return fmt.Errorf("artifact: tags must not be empty after trimming")
		}
	}

	return nil
}

// SanitizeMetadata returns a copy of the metadata with sanitized fields.
func SanitizeMetadata(m ArtifactMetadata) ArtifactMetadata {
	// Trim whitespace from all string fields
	m.MIMEType = strings.TrimSpace(m.MIMEType)
	m.Filename = strings.TrimSpace(m.Filename)
	m.Source = strings.TrimSpace(m.Source)
	m.Destination = strings.TrimSpace(m.Destination)

	// Sanitize tags: trim and remove empties
	cleanTags := make([]string, 0, len(m.Tags))
	for _, tag := range m.Tags {
		trimmed := strings.TrimSpace(tag)
		if trimmed != "" {
			cleanTags = append(cleanTags, trimmed)
		}
	}
	m.Tags = cleanTags

	return m
}
