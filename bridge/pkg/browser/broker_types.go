package browser

import (
	"time"

	"github.com/armorclaw/bridge/pkg/queue"
)

type JobID string

type StartJobRequest struct {
	AgentID string   `json:"agent_id"`
	URL     string   `json:"url"`
	Steps   []string `json:"steps,omitempty"`
	Timeout int      `json:"timeout,omitempty"` // ms
}

// FillRequest describes a single fill operation with optional PII gating.
//
// When Sensitive is true the broker must route the field through the PII
// approval path: the value is resolved via ValueRef (a keystore reference)
// and the human-in-the-loop gate must grant access before the value is
// injected into the browser. This maps directly to ServiceFillField.ValueRef.
type FillRequest struct {
	Selector  string `json:"selector"`
	Value     string `json:"value,omitempty"`
	ValueRef  string `json:"value_ref,omitempty"` // keystore PII reference
	Sensitive bool   `json:"sensitive"`           // true → PII approval path
}

type ExtractSpec struct {
	Fields []ExtractField `json:"fields"`
}

type ExtractField struct {
	Name      string `json:"name"`
	Selector  string `json:"selector"`
	Attribute string `json:"attribute,omitempty"` // default: "textContent"
}

type ExtractResult struct {
	Fields     map[string]string `json:"fields,omitempty"`
	Screenshot string            `json:"screenshot,omitempty"` // base64
}

type BrokerResult struct {
	JobID         JobID        `json:"job_id,omitempty"`
	Success       bool         `json:"success"`
	URL           string       `json:"url,omitempty"`
	Title         string       `json:"title,omitempty"`
	ExtractedData []string     `json:"extracted_data,omitempty"`
	Screenshots   []string     `json:"screenshots,omitempty"`
	Duration      int          `json:"duration"` // ms
	Error         *BrokerError `json:"error,omitempty"`
}

type BrokerError struct {
	Code       string `json:"code"`
	Message    string `json:"message"`
	Selector   string `json:"selector,omitempty"`
	Screenshot string `json:"screenshot,omitempty"` // base64
}

// JobStatus is an alias for queue.JobStatus.
// All browser job status tracking uses the canonical type from the queue package.
//
// Deprecated: Use queue.JobStatus directly.
type JobStatus = queue.JobStatus

const (
	JobStatusPending     = queue.JobStatusPending
	JobStatusRunning     = queue.JobStatusRunning
	JobStatusPaused      = queue.JobStatusPaused
	JobStatusCompleted   = queue.JobStatusCompleted
	JobStatusFailed      = queue.JobStatusFailed
	JobStatusCancelled   = queue.JobStatusCancelled
	JobStatusAwaitingPII = queue.JobStatusAwaitingPII
)

type JobSummary struct {
	ID          JobID      `json:"id"`
	AgentID     string     `json:"agent_id"`
	Status      JobStatus  `json:"status"`
	URL         string     `json:"url,omitempty"`
	Error       string     `json:"error,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
	StartedAt   *time.Time `json:"started_at,omitempty"`
	CompletedAt *time.Time `json:"completed_at,omitempty"`
}

func (r *BrokerResult) ToBrowserResult() *BrowserResultCompat {
	return &BrowserResultCompat{
		URL:           r.URL,
		Title:         r.Title,
		ExtractedData: r.ExtractedData,
		Screenshots:   r.Screenshots,
	}
}

type BrowserResultCompat struct {
	URL           string   `json:"url"`
	Title         string   `json:"title,omitempty"`
	ExtractedData []string `json:"extracted_data,omitempty"`
	Screenshots   []string `json:"screenshots,omitempty"`
}
