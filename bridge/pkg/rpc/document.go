package rpc

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/armorclaw/bridge/pkg/sidecar"
	"github.com/armorclaw/bridge/pkg/yara"
)

// ---------------------------------------------------------------------------
// Sidecar client injection (follows SetBrowserBroker pattern)
// ---------------------------------------------------------------------------

// SetSidecarClients injects sidecar clients for document RPC routing.
// When set, document.extract_text routes through the 3-layer extraction
// pipeline (native text → sidecar → strict drop). Pass nil clients to
// disable sidecar routing (only native text bypass will work).
func (s *Server) SetSidecarClients(office, rust, java *sidecar.Client) {
	s.sidecarOffice = office
	s.sidecarRust = rust
	s.sidecarJava = java
}

// ---------------------------------------------------------------------------
// document.extract_text
// ---------------------------------------------------------------------------

// handleDocumentExtractText extracts text from a document using the 3-layer
// routing pipeline: native text bypass → sidecar (Python/Rust/Java) → strict drop.
// All requests require auth (enforced by RPC safety middleware) and YARA scanning.
func (s *Server) handleDocumentExtractText(ctx context.Context, req *Request) (interface{}, *ErrorObj) {
	var params struct {
		Token           string `json:"token"`
		DocumentFormat  string `json:"document_format"`
		DocumentContent []byte `json:"document_content"`
		DocumentURI     string `json:"document_uri,omitempty"`
		AgentID         string `json:"agent_id,omitempty"`
		WorkflowID      string `json:"workflow_id,omitempty"`
	}

	if err := json.Unmarshal(req.Params, &params); err != nil {
		return nil, &ErrorObj{
			Code:    InvalidParams,
			Message: "invalid parameters: " + err.Error(),
		}
	}

	if params.DocumentFormat == "" {
		return nil, &ErrorObj{
			Code:    InvalidParams,
			Message: "document_format is required",
		}
	}

	if len(params.DocumentContent) == 0 && params.DocumentURI == "" {
		return nil, &ErrorObj{
			Code:    InvalidParams,
			Message: "document_content or document_uri is required",
		}
	}

	// YARA Content Disarm & Reconstruction (CDR) — scan content before extraction.
	// Write to temp file for YARA scanning (YARA requires file paths).
	if len(params.DocumentContent) > 0 {
		tmpFile, err := os.CreateTemp("", "armorclaw-yara-scan-*.bin")
		if err != nil {
			return nil, &ErrorObj{
				Code:    InternalError,
				Message: "failed to create temp file for YARA scan: " + err.Error(),
			}
		}
		tmpPath := tmpFile.Name()
		defer os.Remove(tmpPath)

		if _, err := tmpFile.Write(params.DocumentContent); err != nil {
			tmpFile.Close()
			return nil, &ErrorObj{
				Code:    InternalError,
				Message: "failed to write temp file for YARA scan: " + err.Error(),
			}
		}
		tmpFile.Close()

		clean, err := yara.ScanFileForMalware(tmpPath)
		if err != nil {
			slog.Warn("document YARA scan error (allowing extraction to proceed)",
				"error", err,
				"format", params.DocumentFormat,
			)
		} else if !clean {
			return nil, &ErrorObj{
				Code:    RPCFailClosed,
				Message: "document blocked by YARA content disarm: malicious content detected",
			}
		}
	}

	// Build sidecar request
	sidecarReq := &sidecar.ExtractTextRequest{
		DocumentFormat:  params.DocumentFormat,
		DocumentContent: params.DocumentContent,
		DocumentUri:     params.DocumentURI,
	}

	// Route through 3-layer extraction pipeline
	resp, err := sidecar.RouteExtractText(ctx, sidecarReq, s.sidecarOffice, s.sidecarRust, s.sidecarJava)
	if err != nil {
		return nil, &ErrorObj{
			Code:    InternalError,
			Message: "document extraction failed: " + err.Error(),
		}
	}

	// Return extraction result — no raw document text in logs
	slog.Info("document extraction complete",
		"format", params.DocumentFormat,
		"page_count", resp.PageCount,
		"agent_id", params.AgentID,
		"workflow_id", params.WorkflowID,
	)

	result := map[string]interface{}{
		"text":       resp.Text,
		"page_count": resp.PageCount,
		"metadata":   resp.Metadata,
	}

	return result, nil
}

// ---------------------------------------------------------------------------
// document.status
// ---------------------------------------------------------------------------

// ExtractionJob tracks an in-progress document extraction job.
type ExtractionJob struct {
	ID           string    `json:"id"`
	Status       string    `json:"status"`
	DocumentFormat string  `json:"document_format"`
	CreatedAt    time.Time `json:"created_at"`
	CompletedAt  *time.Time `json:"completed_at,omitempty"`
	Error        string    `json:"error,omitempty"`
	AgentID      string    `json:"agent_id,omitempty"`
	WorkflowID   string    `json:"workflow_id,omitempty"`
}

// handleDocumentStatus returns the status of a document extraction job.
func (s *Server) handleDocumentStatus(ctx context.Context, req *Request) (interface{}, *ErrorObj) {
	var params struct {
		Token string `json:"token"`
		JobID string `json:"job_id"`
	}

	if err := json.Unmarshal(req.Params, &params); err != nil {
		return nil, &ErrorObj{
			Code:    InvalidParams,
			Message: "invalid parameters: " + err.Error(),
		}
	}

	if params.JobID == "" {
		return nil, &ErrorObj{
			Code:    InvalidParams,
			Message: "job_id is required",
		}
	}

	// Look up job in the extraction job store
	job, ok := s.getExtractionJob(params.JobID)
	if !ok {
		return nil, &ErrorObj{
			Code:    InvalidParams,
			Message: fmt.Sprintf("extraction job %q not found", params.JobID),
		}
	}

	return map[string]interface{}{
		"job_id":        job.ID,
		"status":        job.Status,
		"document_format": job.DocumentFormat,
		"created_at":    job.CreatedAt.Format(time.RFC3339),
		"completed_at":  formatOptionalTime(job.CompletedAt),
		"error":         job.Error,
		"agent_id":      job.AgentID,
		"workflow_id":   job.WorkflowID,
	}, nil
}

// ---------------------------------------------------------------------------
// document.list_jobs
// ---------------------------------------------------------------------------

// handleDocumentListJobs lists document extraction jobs for a workflow.
func (s *Server) handleDocumentListJobs(ctx context.Context, req *Request) (interface{}, *ErrorObj) {
	var params struct {
		Token      string `json:"token"`
		WorkflowID string `json:"workflow_id"`
		AgentID    string `json:"agent_id,omitempty"`
		Limit      int    `json:"limit,omitempty"`
	}

	if err := json.Unmarshal(req.Params, &params); err != nil {
		return nil, &ErrorObj{
			Code:    InvalidParams,
			Message: "invalid parameters: " + err.Error(),
		}
	}

	if params.WorkflowID == "" {
		return nil, &ErrorObj{
			Code:    InvalidParams,
			Message: "workflow_id is required",
		}
	}

	if params.Limit <= 0 {
		params.Limit = 50
	}

	jobs := s.listExtractionJobs(params.WorkflowID, params.AgentID, params.Limit)

	result := make([]map[string]interface{}, 0, len(jobs))
	for _, job := range jobs {
		result = append(result, map[string]interface{}{
			"job_id":        job.ID,
			"status":        job.Status,
			"document_format": job.DocumentFormat,
			"created_at":    job.CreatedAt.Format(time.RFC3339),
			"completed_at":  formatOptionalTime(job.CompletedAt),
			"error":         job.Error,
		})
	}

	return map[string]interface{}{
		"jobs":   result,
		"count":  len(result),
	}, nil
}

// ---------------------------------------------------------------------------
// In-memory extraction job store (for MVP; will migrate to SQLite later)
// ---------------------------------------------------------------------------

func (s *Server) getExtractionJob(jobID string) (*ExtractionJob, bool) {
	if s.extractionJobs == nil {
		return nil, false
	}
	job, ok := s.extractionJobs[jobID]
	if !ok {
		return nil, false
	}
	return job, true
}

func (s *Server) listExtractionJobs(workflowID, agentID string, limit int) []*ExtractionJob {
	if s.extractionJobs == nil {
		return nil
	}

	var results []*ExtractionJob
	for _, job := range s.extractionJobs {
		if job.WorkflowID == workflowID {
			if agentID == "" || job.AgentID == agentID {
				results = append(results, job)
				if len(results) >= limit {
					break
				}
			}
		}
	}
	return results
}

// StoreExtractionJob stores a new extraction job in the in-memory store.
// Exported for use by other handlers (e.g., secretary workflow steps).
func (s *Server) StoreExtractionJob(job *ExtractionJob) {
	if s.extractionJobs == nil {
		s.extractionJobs = make(map[string]*ExtractionJob)
	}
	s.extractionJobs[job.ID] = job
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func formatOptionalTime(t *time.Time) string {
	if t == nil {
		return ""
	}
	return t.Format(time.RFC3339)
}
