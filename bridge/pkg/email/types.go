package email

import (
	"context"
	"time"

	"github.com/armorclaw/bridge/pkg/pii"
)

type EmailAddress struct {
	Address string `json:"address"`
	Name    string `json:"name,omitempty"`
}

type EmailAttachment struct {
	Filename    string `json:"filename"`
	ContentType string `json:"content_type"`
	Size        int64  `json:"size"`
	ContentID   string `json:"content_id,omitempty"`
	FileID      string `json:"file_id"`
}

type ProcessedFile struct {
	FileID   string `json:"file_id"`
	FilePath string `json:"file_path"`
	Filename string `json:"filename"`
	Size     int64  `json:"size"`
	IsClean  bool   `json:"is_clean"`
	YARARule string `json:"yara_rule,omitempty"`
	SHA256   string `json:"sha256"`
}

type EmailSender interface {
	Send(ctx context.Context, to, subject, bodyText, bodyHTML string, attachments ...*EmailAttachment) (messageID string, err error)
	Provider() string
}

// ApprovalStatus is an alias for pii.AccessRequestStatus for backward compatibility.
// Deprecated: Use pii.AccessRequestStatus directly.
type ApprovalStatus = pii.AccessRequestStatus

const (
	ApprovalPending  = pii.StatusPending
	ApprovalApproved = pii.StatusApproved
	ApprovalRejected = pii.StatusRejected
	ApprovalExpired  = pii.StatusExpired
)

type OutboundResult struct {
	Success      bool           `json:"success"`
	MessageID    string         `json:"message_id,omitempty"`
	ProviderUsed string         `json:"provider_used,omitempty"`
	Error        string         `json:"error,omitempty"`
	ApprovedAt   *time.Time     `json:"approved_at,omitempty"`
	Status       ApprovalStatus `json:"status"`
}
