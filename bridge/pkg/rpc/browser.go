// Package rpc provides browser automation RPC methods.
// These methods expose the BrowserSkill to external callers via JSON-RPC.
package rpc

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/armorclaw/bridge/pkg/browser"
)

// BrowserJob represents an active browser automation job
type BrowserJob struct {
	ID          string
	AgentID     string
	URL         string
	Status      string // "running", "completed", "failed"
	CreatedAt   time.Time
	CompletedAt *time.Time
	Error       string
	Session     *browser.BrowserSession
	skill       *browser.BrowserSkill
	cancelFunc  context.CancelFunc
}

// BrowserJobManager manages active browser jobs
type BrowserJobManager struct {
	mu      sync.RWMutex
	jobs    map[string]*BrowserJob
	emitter browser.StatusEmitter
}

// NewBrowserJobManager creates a new job manager
func NewBrowserJobManager() *BrowserJobManager {
	return &BrowserJobManager{
		jobs: make(map[string]*BrowserJob),
	}
}

// SetEmitter sets the status emitter for new jobs
func (m *BrowserJobManager) SetEmitter(emitter browser.StatusEmitter) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.emitter = emitter
}

// CreateJob creates a new browser job
func (m *BrowserJobManager) CreateJob(id, agentID string) *BrowserJob {
	m.mu.Lock()
	defer m.mu.Unlock()

	job := &BrowserJob{
		ID:        id,
		AgentID:   agentID,
		Status:    "idle",
		CreatedAt: time.Now(),
		skill:     browser.NewBrowserSkill(agentID, m.emitter),
	}
	m.jobs[id] = job
	return job
}

// GetJob retrieves a job by ID
func (m *BrowserJobManager) GetJob(id string) (*BrowserJob, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	job, exists := m.jobs[id]
	return job, exists
}

// RemoveJob removes a job
func (m *BrowserJobManager) RemoveJob(id string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if job, exists := m.jobs[id]; exists {
		if job.cancelFunc != nil {
			job.cancelFunc()
		}
	}
	delete(m.jobs, id)
}

// ListJobs returns all active jobs
func (m *BrowserJobManager) ListJobs() []*BrowserJob {
	m.mu.RLock()
	defer m.mu.RUnlock()

	jobs := make([]*BrowserJob, 0, len(m.jobs))
	for _, job := range m.jobs {
		jobs = append(jobs, job)
	}
	return jobs
}

// CleanupOldJobs removes jobs older than the specified duration
func (m *BrowserJobManager) CleanupOldJobs(maxAge time.Duration) int {
	m.mu.Lock()
	defer m.mu.Unlock()

	cutoff := time.Now().Add(-maxAge)
	removed := 0

	for id, job := range m.jobs {
		if job.CreatedAt.Before(cutoff) && (job.Status == "completed" || job.Status == "failed") {
			delete(m.jobs, id)
			removed++
		}
	}
	return removed
}

// generateID creates a simple unique ID
func generateID() string {
	return fmt.Sprintf("%d", time.Now().UnixNano())
}

// handleBrowserNavigate handles browser.navigate RPC method
func (s *Server) handleBrowserNavigate(ctx context.Context, req *Request) (interface{}, *ErrorObj) {
	var params struct {
		URL     string `json:"url"`
		AgentID string `json:"agent_id"`
		JobID   string `json:"job_id,omitempty"`
	}

	if err := json.Unmarshal(req.Params, &params); err != nil {
		return nil, &ErrorObj{
			Code:    InvalidParams,
			Message: "invalid parameters: " + err.Error(),
		}
	}

	if params.URL == "" {
		return nil, &ErrorObj{
			Code:    InvalidParams,
			Message: "url is required",
		}
	}

	if params.AgentID == "" {
		params.AgentID = "default"
	}

	// Broker path: delegate to broker when configured
	if s.browserBroker != nil {
		jobID := params.JobID
		if jobID == "" {
			jobID = "browser_" + generateID()
		}

		startReq := browser.StartJobRequest{
			AgentID: params.AgentID,
			URL:     params.URL,
		}

		id, err := s.browserBroker.StartJob(ctx, startReq)
		if err != nil {
			return nil, &ErrorObj{
				Code:    InternalError,
				Message: "broker start job failed: " + err.Error(),
			}
		}

		result, err := s.browserBroker.Navigate(ctx, id, params.URL)
		if err != nil {
			return nil, &ErrorObj{
				Code:    InternalError,
				Message: "broker navigate failed: " + err.Error(),
			}
		}

		resp := map[string]interface{}{
			"job_id":   string(id),
			"status":   "running",
			"url":      params.URL,
			"agent_id": params.AgentID,
			"message":  "Navigation started. Use browser.status to poll for completion.",
		}
		if result != nil {
			resp["success"] = result.Success
			if result.URL != "" {
				resp["url"] = result.URL
			}
			if result.Title != "" {
				resp["title"] = result.Title
			}
		}
		return resp, nil
	}

	// Fallback: legacy BrowserSkill path
	jobID := params.JobID
	if jobID == "" {
		jobID = "browser_" + generateID()
	}

	job := s.browserJobs.CreateJob(jobID, params.AgentID)
	job.URL = params.URL
	job.Status = "running"

	ctx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	job.cancelFunc = cancel

	go func() {
		defer cancel()

		err := job.skill.Navigate(ctx, params.URL)
		if err != nil {
			job.Status = "failed"
			job.Error = err.Error()
		} else {
			job.Status = "completed"
		}

		now := time.Now()
		job.CompletedAt = &now
		job.Session = job.skill.GetSession()
	}()

	return map[string]interface{}{
		"job_id":     jobID,
		"status":     "running",
		"url":        params.URL,
		"agent_id":   params.AgentID,
		"message":    "Navigation started. Use browser.status to poll for completion.",
		"created_at": job.CreatedAt.Format(time.RFC3339),
	}, nil
}

// handleBrowserFill handles browser.fill RPC method
func (s *Server) handleBrowserFill(ctx context.Context, req *Request) (interface{}, *ErrorObj) {
	var params struct {
		JobID    string `json:"job_id"`
		Selector string `json:"selector"`
		Value    string `json:"value"`
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

	if params.Selector == "" {
		return nil, &ErrorObj{
			Code:    InvalidParams,
			Message: "selector is required",
		}
	}

	if s.browserBroker != nil {
		fields := []browser.FillRequest{
			{Selector: params.Selector, Value: params.Value},
		}
		result, err := s.browserBroker.Fill(ctx, browser.JobID(params.JobID), fields)
		if err != nil {
			return nil, &ErrorObj{
				Code:    InternalError,
				Message: "broker fill failed: " + err.Error(),
			}
		}

		resp := map[string]interface{}{
			"job_id":   params.JobID,
			"selector": params.Selector,
			"success":  true,
		}
		if result != nil {
			resp["success"] = result.Success
		}
		return resp, nil
	}

	// Fallback: legacy BrowserSkill path
	job, exists := s.browserJobs.GetJob(params.JobID)
	if !exists {
		return nil, &ErrorObj{
			Code:    InvalidParams,
			Message: "job not found: " + params.JobID,
		}
	}

	if job.Status == "failed" {
		return nil, &ErrorObj{
			Code:    InternalError,
			Message: "job has failed: " + job.Error,
		}
	}

	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	err := job.skill.FillForm(ctx, params.Selector, params.Value)
	if err != nil {
		return nil, &ErrorObj{
			Code:    InternalError,
			Message: "failed to fill form: " + err.Error(),
		}
	}

	return map[string]interface{}{
		"job_id":   params.JobID,
		"status":   "filled",
		"selector": params.Selector,
		"success":  true,
	}, nil
}

// handleBrowserClick handles browser.click RPC method
func (s *Server) handleBrowserClick(ctx context.Context, req *Request) (interface{}, *ErrorObj) {
	var params struct {
		JobID    string `json:"job_id"`
		Selector string `json:"selector"`
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

	if params.Selector == "" {
		return nil, &ErrorObj{
			Code:    InvalidParams,
			Message: "selector is required",
		}
	}

	if s.browserBroker != nil {
		result, err := s.browserBroker.Click(ctx, browser.JobID(params.JobID), params.Selector)
		if err != nil {
			return nil, &ErrorObj{
				Code:    InternalError,
				Message: "broker click failed: " + err.Error(),
			}
		}

		resp := map[string]interface{}{
			"job_id":   params.JobID,
			"selector": params.Selector,
			"success":  true,
		}
		if result != nil {
			resp["success"] = result.Success
		}
		return resp, nil
	}

	// Fallback: legacy BrowserSkill path
	job, exists := s.browserJobs.GetJob(params.JobID)
	if !exists {
		return nil, &ErrorObj{
			Code:    InvalidParams,
			Message: "job not found: " + params.JobID,
		}
	}

	if job.Status == "failed" {
		return nil, &ErrorObj{
			Code:    InternalError,
			Message: "job has failed: " + job.Error,
		}
	}

	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	err := job.skill.Click(ctx, params.Selector)
	if err != nil {
		return nil, &ErrorObj{
			Code:    InternalError,
			Message: "failed to click: " + err.Error(),
		}
	}

	return map[string]interface{}{
		"job_id":   params.JobID,
		"status":   "clicked",
		"selector": params.Selector,
		"success":  true,
	}, nil
}

// handleBrowserStatus handles browser.status RPC method
func (s *Server) handleBrowserStatus(ctx context.Context, req *Request) (interface{}, *ErrorObj) {
	var params struct {
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

	if s.browserBroker != nil {
		summary, err := s.browserBroker.Status(ctx, browser.JobID(params.JobID))
		if err != nil {
			return nil, &ErrorObj{
				Code:    InternalError,
				Message: "broker status failed: " + err.Error(),
			}
		}

		result := map[string]interface{}{
			"job_id":   string(summary.ID),
			"agent_id": summary.AgentID,
			"status":   string(summary.Status),
			"url":      summary.URL,
		}
		if !summary.CreatedAt.IsZero() {
			result["created_at"] = summary.CreatedAt.Format(time.RFC3339)
		}
		if summary.CompletedAt != nil {
			result["completed_at"] = summary.CompletedAt.Format(time.RFC3339)
		}
		if summary.Error != "" {
			result["error"] = summary.Error
		}
		result["screenshot_available"] = false
		return result, nil
	}

	// Fallback: legacy BrowserSkill path
	job, exists := s.browserJobs.GetJob(params.JobID)
	if !exists {
		return nil, &ErrorObj{
			Code:    InvalidParams,
			Message: "job not found: " + params.JobID,
		}
	}

	result := map[string]interface{}{
		"job_id":     job.ID,
		"agent_id":   job.AgentID,
		"status":     job.Status,
		"url":        job.URL,
		"created_at": job.CreatedAt.Format(time.RFC3339),
	}

	if job.CompletedAt != nil {
		result["completed_at"] = job.CompletedAt.Format(time.RFC3339)
	}

	if job.Error != "" {
		result["error"] = job.Error
	}

	if job.Session != nil {
		result["session"] = map[string]interface{}{
			"id":            job.Session.ID,
			"url":           job.Session.URL,
			"status":        string(job.Session.Status),
			"title":         job.Session.Title,
			"last_activity": job.Session.LastActivity.Format(time.RFC3339),
		}
	}

	result["screenshot_available"] = false

	return result, nil
}

// handleBrowserWaitForElement handles browser.wait_for_element RPC method
func (s *Server) handleBrowserWaitForElement(ctx context.Context, req *Request) (interface{}, *ErrorObj) {
	var params struct {
		JobID    string `json:"job_id"`
		Selector string `json:"selector"`
		Timeout  int    `json:"timeout"`
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

	if params.Selector == "" {
		return nil, &ErrorObj{
			Code:    InvalidParams,
			Message: "selector is required",
		}
	}

	if params.Timeout == 0 {
		params.Timeout = 30
	}

	if s.browserBroker != nil {
		result, err := s.browserBroker.WaitForElement(ctx, browser.JobID(params.JobID), params.Selector, params.Timeout*1000)
		if err != nil {
			return nil, &ErrorObj{
				Code:    InternalError,
				Message: "element not found: " + err.Error(),
			}
		}

		resp := map[string]interface{}{
			"job_id":   params.JobID,
			"selector": params.Selector,
			"success":  true,
		}
		if result != nil {
			resp["success"] = result.Success
		}
		return resp, nil
	}

	// Fallback: legacy BrowserSkill path
	job, exists := s.browserJobs.GetJob(params.JobID)
	if !exists {
		return nil, &ErrorObj{
			Code:    InvalidParams,
			Message: "job not found: " + params.JobID,
		}
	}

	ctx, cancel := context.WithTimeout(ctx, time.Duration(params.Timeout)*time.Second)
	defer cancel()

	err := job.skill.WaitForElement(ctx, params.Selector, time.Duration(params.Timeout)*time.Second)
	if err != nil {
		return nil, &ErrorObj{
			Code:    InternalError,
			Message: "element not found: " + err.Error(),
		}
	}

	return map[string]interface{}{
		"job_id":   params.JobID,
		"status":   "element_found",
		"selector": params.Selector,
		"success":  true,
	}, nil
}

// handleBrowserWaitForCaptcha handles browser.wait_for_captcha RPC method
func (s *Server) handleBrowserWaitForCaptcha(ctx context.Context, req *Request) (interface{}, *ErrorObj) {
	var params struct {
		JobID   string `json:"job_id"`
		Timeout int    `json:"timeout"`
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

	if s.browserBroker != nil {
		timeoutMs := params.Timeout * 1000
		if timeoutMs == 0 {
			timeoutMs = 60000
		}
		result, err := s.browserBroker.WaitForCaptcha(ctx, browser.JobID(params.JobID), timeoutMs)
		if err != nil {
			return nil, &ErrorObj{
				Code:    InternalError,
				Message: "broker wait_for_captcha failed: " + err.Error(),
			}
		}

		resp := map[string]interface{}{
			"job_id":  params.JobID,
			"status":  "awaiting_captcha",
			"message": "Captcha detection started. Mobile app will be notified.",
		}
		if result != nil {
			resp["success"] = result.Success
		}
		return resp, nil
	}

	// Fallback: legacy BrowserSkill path
	job, exists := s.browserJobs.GetJob(params.JobID)
	if !exists {
		return nil, &ErrorObj{
			Code:    InvalidParams,
			Message: "job not found: " + params.JobID,
		}
	}

	_ = ctx
	job.skill.WaitForCaptcha(ctx)

	return map[string]interface{}{
		"job_id":  params.JobID,
		"status":  "awaiting_captcha",
		"message": "Captcha detection started. Mobile app will be notified.",
	}, nil
}

// handleBrowserWaitFor2FA handles browser.wait_for_2fa RPC method
func (s *Server) handleBrowserWaitFor2FA(ctx context.Context, req *Request) (interface{}, *ErrorObj) {
	var params struct {
		JobID   string `json:"job_id"`
		Timeout int    `json:"timeout"`
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

	if s.browserBroker != nil {
		timeoutMs := params.Timeout * 1000
		if timeoutMs == 0 {
			timeoutMs = 60000
		}
		result, err := s.browserBroker.WaitFor2FA(ctx, browser.JobID(params.JobID), timeoutMs)
		if err != nil {
			return nil, &ErrorObj{
				Code:    InternalError,
				Message: "broker wait_for_2fa failed: " + err.Error(),
			}
		}

		resp := map[string]interface{}{
			"job_id":  params.JobID,
			"status":  "awaiting_2fa",
			"message": "2FA detection started. Mobile app will be notified.",
		}
		if result != nil {
			resp["success"] = result.Success
		}
		return resp, nil
	}

	// Fallback: legacy BrowserSkill path
	job, exists := s.browserJobs.GetJob(params.JobID)
	if !exists {
		return nil, &ErrorObj{
			Code:    InvalidParams,
			Message: "job not found: " + params.JobID,
		}
	}

	_ = ctx
	job.skill.WaitFor2FA(ctx)

	return map[string]interface{}{
		"job_id":  params.JobID,
		"status":  "awaiting_2fa",
		"message": "2FA detection started. Mobile app will be notified.",
	}, nil
}

// handleBrowserComplete handles browser.complete RPC method
func (s *Server) handleBrowserComplete(ctx context.Context, req *Request) (interface{}, *ErrorObj) {
	var params struct {
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

	if s.browserBroker != nil {
		result, err := s.browserBroker.Complete(ctx, browser.JobID(params.JobID))
		if err != nil {
			return nil, &ErrorObj{
				Code:    InternalError,
				Message: "broker complete failed: " + err.Error(),
			}
		}

		resp := map[string]interface{}{
			"job_id":  params.JobID,
			"status":  "completed",
			"success": true,
		}
		if result != nil {
			resp["success"] = result.Success
		}
		return resp, nil
	}

	// Fallback: legacy BrowserSkill path
	job, exists := s.browserJobs.GetJob(params.JobID)
	if !exists {
		return nil, &ErrorObj{
			Code:    InvalidParams,
			Message: "job not found: " + params.JobID,
		}
	}

	_ = ctx
	job.skill.Complete(ctx)
	job.Status = "completed"
	now := time.Now()
	job.CompletedAt = &now
	job.Session = job.skill.GetSession()

	return map[string]interface{}{
		"job_id":       params.JobID,
		"status":       "completed",
		"completed_at": job.CompletedAt.Format(time.RFC3339),
		"success":      true,
	}, nil
}

// handleBrowserFail handles browser.fail RPC method
func (s *Server) handleBrowserFail(ctx context.Context, req *Request) (interface{}, *ErrorObj) {
	var params struct {
		JobID  string `json:"job_id"`
		Reason string `json:"reason"`
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

	if s.browserBroker != nil {
		err := s.browserBroker.Fail(ctx, browser.JobID(params.JobID), params.Reason)
		if err != nil {
			return nil, &ErrorObj{
				Code:    InternalError,
				Message: "broker fail failed: " + err.Error(),
			}
		}

		return map[string]interface{}{
			"job_id":  params.JobID,
			"status":  "failed",
			"error":   params.Reason,
			"success": true,
		}, nil
	}

	// Fallback: legacy BrowserSkill path
	job, exists := s.browserJobs.GetJob(params.JobID)
	if !exists {
		return nil, &ErrorObj{
			Code:    InvalidParams,
			Message: "job not found: " + params.JobID,
		}
	}

	_ = ctx
	job.skill.Fail(ctx, fmt.Errorf("%s", params.Reason))
	job.Status = "failed"
	job.Error = params.Reason
	now := time.Now()
	job.CompletedAt = &now

	return map[string]interface{}{
		"job_id":  params.JobID,
		"status":  "failed",
		"error":   params.Reason,
		"success": true,
	}, nil
}

// handleBrowserList handles browser.list RPC method
func (s *Server) handleBrowserList(ctx context.Context, req *Request) (interface{}, *ErrorObj) {
	if s.browserBroker != nil {
		summaries, err := s.browserBroker.List(ctx, "")
		if err != nil {
			return nil, &ErrorObj{
				Code:    InternalError,
				Message: "broker list failed: " + err.Error(),
			}
		}

		result := make([]map[string]interface{}, 0, len(summaries))
		for _, sum := range summaries {
			jobInfo := map[string]interface{}{
				"job_id":   string(sum.ID),
				"agent_id": sum.AgentID,
				"url":      sum.URL,
				"status":   string(sum.Status),
			}
			if !sum.CreatedAt.IsZero() {
				jobInfo["created_at"] = sum.CreatedAt.Format(time.RFC3339)
			}
			if sum.CompletedAt != nil {
				jobInfo["completed_at"] = sum.CompletedAt.Format(time.RFC3339)
			}
			if sum.Error != "" {
				jobInfo["error"] = sum.Error
			}
			result = append(result, jobInfo)
		}

		return map[string]interface{}{
			"jobs":  result,
			"count": len(result),
		}, nil
	}

	// Fallback: legacy BrowserSkill path
	jobs := s.browserJobs.ListJobs()

	result := make([]map[string]interface{}, 0, len(jobs))
	for _, job := range jobs {
		jobInfo := map[string]interface{}{
			"job_id":     job.ID,
			"agent_id":   job.AgentID,
			"url":        job.URL,
			"status":     job.Status,
			"created_at": job.CreatedAt.Format(time.RFC3339),
		}
		if job.CompletedAt != nil {
			jobInfo["completed_at"] = job.CompletedAt.Format(time.RFC3339)
		}
		if job.Error != "" {
			jobInfo["error"] = job.Error
		}
		result = append(result, jobInfo)
	}

	return map[string]interface{}{
		"jobs":  result,
		"count": len(result),
	}, nil
}

// handleBrowserCancel handles browser.cancel RPC method
func (s *Server) handleBrowserCancel(ctx context.Context, req *Request) (interface{}, *ErrorObj) {
	var params struct {
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

	if s.browserBroker != nil {
		err := s.browserBroker.Cancel(ctx, browser.JobID(params.JobID))
		if err != nil {
			return nil, &ErrorObj{
				Code:    InternalError,
				Message: "broker cancel failed: " + err.Error(),
			}
		}

		return map[string]interface{}{
			"job_id":  params.JobID,
			"status":  "cancelled",
			"success": true,
		}, nil
	}

	// Fallback: legacy BrowserSkill path
	job, exists := s.browserJobs.GetJob(params.JobID)
	if !exists {
		return nil, &ErrorObj{
			Code:    InvalidParams,
			Message: "job not found: " + params.JobID,
		}
	}

	if job.cancelFunc != nil {
		job.cancelFunc()
	}
	job.Status = "cancelled"
	now := time.Now()
	job.CompletedAt = &now

	return map[string]interface{}{
		"job_id":       params.JobID,
		"status":       "cancelled",
		"cancelled_at": job.CompletedAt.Format(time.RFC3339),
		"success":      true,
	}, nil
}

// handleBrowserReplayDiagnostics compares the expected NavChart for a tab
// against a replayed chart and returns a diff with match percentage.
// Requires FeatureFlags.MultiTabReplay to be enabled.
func (s *Server) handleBrowserReplayDiagnostics(ctx context.Context, req *Request) (interface{}, *ErrorObj) {
	if !s.replayFlags.MultiTabReplay {
		return nil, &ErrorObj{
			Code:    MethodNotFound,
			Message: "Feature disabled: multi_tab_replay",
		}
	}

	var params struct {
		TabID     string `json:"tab_id"`
		SessionID string `json:"session_id,omitempty"`
	}

	if err := json.Unmarshal(req.Params, &params); err != nil {
		return nil, &ErrorObj{
			Code:    InvalidParams,
			Message: "invalid parameters: " + err.Error(),
		}
	}

	if params.TabID == "" {
		return nil, &ErrorObj{
			Code:    InvalidParams,
			Message: "tab_id is required",
		}
	}

	if s.navChartStore == nil {
		return nil, &ErrorObj{
			Code:    InternalError,
			Message: "navchart store not configured",
		}
	}

	_ = params.SessionID

	charts, err := s.navChartStore.GetCharts(params.TabID)
	if err != nil {
		return nil, &ErrorObj{
			Code:    InternalError,
			Message: "failed to retrieve charts: " + err.Error(),
		}
	}
	if len(charts) == 0 {
		return nil, &ErrorObj{
			Code:    InvalidParams,
			Message: fmt.Sprintf("no stored charts for tab %q", params.TabID),
		}
	}

	var diffs []browser.ChartDiff
	var matchPercentage float64

	if len(charts) >= 2 {
		expected := charts[len(charts)-2]
		actual := charts[len(charts)-1]
		diffs = browser.DiffCharts(expected, actual)

		matched := 0
		for _, d := range diffs {
			if d.Match {
				matched++
			}
		}
		if len(diffs) > 0 {
			matchPercentage = float64(matched) / float64(len(diffs)) * 100.0
		} else {
			matchPercentage = 100.0
		}
	} else {
		diffs = []browser.ChartDiff{}
		matchPercentage = 100.0
	}

	return map[string]interface{}{
		"tab_id":           params.TabID,
		"diffs":            diffs,
		"match_percentage": matchPercentage,
	}, nil
}

// handleBrowserScreenshot handles browser.screenshot RPC method.
// Returns a placeholder response indicating the feature exists but
// browser-service may not be connected.
func (s *Server) handleBrowserScreenshot(ctx context.Context, req *Request) (interface{}, *ErrorObj) {
	var params struct {
		Format    string `json:"format,omitempty"`
		FullPage  bool   `json:"full_page,omitempty"`
		SessionID string `json:"session_id,omitempty"`
		JobID     string `json:"job_id,omitempty"`
	}

	if err := json.Unmarshal(req.Params, &params); err != nil {
		return nil, &ErrorObj{
			Code:    InvalidParams,
			Message: "invalid parameters: " + err.Error(),
		}
	}

	if params.Format == "" {
		params.Format = "png"
	}

	// Determine the job/session to screenshot
	jobID := params.JobID
	if jobID == "" {
		jobID = params.SessionID
	}

	// Broker path: delegate to broker when configured
	if s.browserBroker != nil {
		if jobID == "" {
			return nil, &ErrorObj{
				Code:    InternalError,
				Message: "no active browser session",
			}
		}

		// Verify the job exists via status check
		_, err := s.browserBroker.Status(ctx, browser.JobID(jobID))
		if err != nil {
			return nil, &ErrorObj{
				Code:    InternalError,
				Message: "no active browser session",
			}
		}

		return map[string]interface{}{
			"status":  "screenshot_pending",
			"message": "screenshot requested",
			"format":  params.Format,
			"job_id":  jobID,
		}, nil
	}

	// Fallback: legacy BrowserSkill path
	if jobID != "" {
		_, exists := s.browserJobs.GetJob(jobID)
		if !exists {
			return nil, &ErrorObj{
				Code:    InternalError,
				Message: "no active browser session",
			}
		}

		return map[string]interface{}{
			"status":  "screenshot_pending",
			"message": "screenshot requested",
			"format":  params.Format,
			"job_id":  jobID,
		}, nil
	}

	return nil, &ErrorObj{
		Code:    InternalError,
		Message: "no active browser session",
	}
}

// handleBrowserClose handles browser.close RPC method.
// Idempotent: returns success even if no active session exists.
func (s *Server) handleBrowserClose(ctx context.Context, req *Request) (interface{}, *ErrorObj) {
	var params struct {
		JobID     string `json:"job_id,omitempty"`
		SessionID string `json:"session_id,omitempty"`
	}

	if err := json.Unmarshal(req.Params, &params); err != nil {
		return nil, &ErrorObj{
			Code:    InvalidParams,
			Message: "invalid parameters: " + err.Error(),
		}
	}

	jobID := params.JobID
	if jobID == "" {
		jobID = params.SessionID
	}

	// No job specified — idempotent success
	if jobID == "" {
		return map[string]interface{}{
			"ok":      true,
			"message": "no active session to close",
		}, nil
	}

	// Broker path
	if s.browserBroker != nil {
		_, err := s.browserBroker.Complete(ctx, browser.JobID(jobID))
		if err != nil {
			// Idempotent: treat "not found" as success
			return map[string]interface{}{
				"ok":      true,
				"message": "no active session to close",
			}, nil
		}

		return map[string]interface{}{
			"ok":        true,
			"session_id": jobID,
			"message":   "session closed",
		}, nil
	}

	// Fallback: legacy BrowserSkill path
	job, exists := s.browserJobs.GetJob(jobID)
	if !exists {
		return map[string]interface{}{
			"ok":      true,
			"message": "no active session to close",
		}, nil
	}

	job.skill.Complete(ctx)
	job.Status = "completed"
	now := time.Now()
	job.CompletedAt = &now
	job.Session = job.skill.GetSession()

	return map[string]interface{}{
		"ok":         true,
		"session_id": jobID,
		"message":    "session closed",
	}, nil
}
