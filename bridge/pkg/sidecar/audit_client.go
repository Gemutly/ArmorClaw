package sidecar

import (
	"context"
	"log"
	"time"

	"github.com/armorclaw/bridge/pkg/audit"
	"google.golang.org/grpc/metadata"
)

// AuditClient wraps a sidecar client with audit logging
type AuditClient struct {
	client    *Client
	auditLog  *audit.AuditLog
	extractFn func(metadata.MD) (requestID, userID, agentID, sessionID string)
}

// NewAuditClient creates a new audit-wrapped sidecar client
func NewAuditClient(client *Client, auditLog *audit.AuditLog) *AuditClient {
	return &AuditClient{
		client:    client,
		auditLog:  auditLog,
		extractFn: extractMetadata,
	}
}

// SetMetadataExtractor sets a custom function to extract metadata from gRPC metadata
func (ac *AuditClient) SetMetadataExtractor(fn func(metadata.MD) (string, string, string, string)) {
	ac.extractFn = fn
}

// auditLogEntry captures details for an audit log entry
type auditLogEntry struct {
	Operation string
	Success   bool
	Error     string
	Duration  time.Duration
	RequestID string
	UserID    string
	AgentID   string
	SessionID string
	FileSize  int64
	Metadata  map[string]interface{}
}

// logAudit logs an audit entry for a sidecar operation
func (ac *AuditClient) logAudit(ctx context.Context, eventType audit.EventType, entry *auditLogEntry) error {
	// Extract metadata from context
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		md = metadata.MD{}
	}

	requestID, userID, agentID, sessionID := ac.extractFn(md)

	// Use values from entry if provided
	if entry.RequestID != "" {
		requestID = entry.RequestID
	}
	if entry.UserID != "" {
		userID = entry.UserID
	}
	if entry.AgentID != "" {
		agentID = entry.AgentID
	}
	if entry.SessionID != "" {
		sessionID = entry.SessionID
	}

	details := map[string]interface{}{
		"operation":   entry.Operation,
		"success":     entry.Success,
		"duration_ms": entry.Duration.Milliseconds(),
		"request_id":  requestID,
		"user_id":     userID,
		"agent_id":    agentID,
		"session_id":  sessionID,
	}

	if entry.Error != "" {
		details["error"] = entry.Error
	}
	if entry.FileSize > 0 {
		details["file_size"] = entry.FileSize
	}

	// Add any additional metadata
	for k, v := range entry.Metadata {
		details[k] = v
	}

	return ac.auditLog.LogEvent(eventType, sessionID, "", userID, details)
}

// extractMetadata extracts request ID, user ID, agent ID, and session ID from gRPC metadata
func extractMetadata(md metadata.MD) (requestID, userID, agentID, sessionID string) {
	if vals := md.Get("x-request-id"); len(vals) > 0 {
		requestID = vals[0]
	}
	if vals := md.Get("x-user-id"); len(vals) > 0 {
		userID = vals[0]
	}
	if vals := md.Get("x-agent-id"); len(vals) > 0 {
		agentID = vals[0]
	}
	if vals := md.Get("x-session-id"); len(vals) > 0 {
		sessionID = vals[0]
	}
	return
}

// HealthCheck performs a health check with audit logging
func (ac *AuditClient) HealthCheck(ctx context.Context) (*HealthCheckResponse, error) {
	start := time.Now()
	resp, err := ac.client.HealthCheck(ctx)
	duration := time.Since(start)

	auditErr := ac.logAudit(ctx, audit.EventSidecarHealthCheck, &auditLogEntry{
		Operation: "HealthCheck",
		Success:   err == nil,
		Duration:  duration,
	})

	if auditErr != nil {
		log.Printf("audit log warning: %v\n", auditErr)
	}

	if err != nil {
		return nil, err
	}

	return resp, nil
}

// UploadBlob uploads a blob with audit logging
func (ac *AuditClient) UploadBlob(ctx context.Context, req *UploadBlobRequest) (*UploadBlobResponse, error) {
	start := time.Now()
	resp, err := ac.client.UploadBlob(ctx, req)
	duration := time.Since(start)

	var fileSize int64
	if req != nil {
		fileSize = int64(len(req.Content))
	}

	auditErr := ac.logAudit(ctx, audit.EventSidecarUploadBlob, &auditLogEntry{
		Operation: "UploadBlob",
		Success:   err == nil,
		Duration:  duration,
		FileSize:  fileSize,
		Metadata: map[string]interface{}{
			"provider":     req.GetProvider(),
			"destination":  req.GetDestinationUri(),
			"content_type": req.GetContentType(),
			"request_id":   req.GetMetadata().GetRequestId(),
		},
	})

	if auditErr != nil {
		log.Printf("audit log warning: %v\n", auditErr)
	}

	if err != nil {
		return nil, err
	}

	return resp, nil
}

// DownloadBlob downloads a blob with audit logging
func (ac *AuditClient) DownloadBlob(ctx context.Context, req *DownloadBlobRequest) ([]byte, error) {
	start := time.Now()
	resp, err := ac.client.DownloadBlob(ctx, req)
	duration := time.Since(start)

	fileSize := int64(len(resp))

	auditErr := ac.logAudit(ctx, audit.EventSidecarDownloadBlob, &auditLogEntry{
		Operation: "DownloadBlob",
		Success:   err == nil,
		Duration:  duration,
		FileSize:  fileSize,
		Metadata: map[string]interface{}{
			"provider":   req.GetProvider(),
			"source_uri": req.GetSourceUri(),
			"request_id": req.GetMetadata().GetRequestId(),
		},
	})

	if auditErr != nil {
		log.Printf("audit log warning: %v\n", auditErr)
	}

	if err != nil {
		return nil, err
	}

	return resp, nil
}

// ListBlobs lists blobs with audit logging
func (ac *AuditClient) ListBlobs(ctx context.Context, req *ListBlobsRequest) (*ListBlobsResponse, error) {
	start := time.Now()
	resp, err := ac.client.ListBlobs(ctx, req)
	duration := time.Since(start)

	blobCount := 0
	if resp != nil {
		blobCount = len(resp.Blobs)
	}

	auditErr := ac.logAudit(ctx, audit.EventSidecarListBlobs, &auditLogEntry{
		Operation: "ListBlobs",
		Success:   err == nil,
		Duration:  duration,
		Metadata: map[string]interface{}{
			"provider":   req.GetProvider(),
			"prefix":     req.GetPrefix(),
			"blob_count": blobCount,
			"request_id": req.GetMetadata().GetRequestId(),
		},
	})

	if auditErr != nil {
		log.Printf("audit log warning: %v\n", auditErr)
	}

	if err != nil {
		return nil, err
	}

	return resp, nil
}

// DeleteBlob deletes a blob with audit logging
func (ac *AuditClient) DeleteBlob(ctx context.Context, req *DeleteBlobRequest) (*DeleteBlobResponse, error) {
	start := time.Now()
	resp, err := ac.client.DeleteBlob(ctx, req)
	duration := time.Since(start)

	auditErr := ac.logAudit(ctx, audit.EventSidecarDeleteBlob, &auditLogEntry{
		Operation: "DeleteBlob",
		Success:   err == nil,
		Duration:  duration,
		Metadata: map[string]interface{}{
			"provider":   req.GetProvider(),
			"uri":        req.GetUri(),
			"request_id": req.GetMetadata().GetRequestId(),
		},
	})

	if auditErr != nil {
		log.Printf("audit log warning: %v\n", auditErr)
	}

	if err != nil {
		return nil, err
	}

	return resp, nil
}

// ExtractText extracts text from a document with audit logging
func (ac *AuditClient) ExtractText(ctx context.Context, req *ExtractTextRequest) (*ExtractTextResponse, error) {
	start := time.Now()
	resp, err := ac.client.ExtractText(ctx, req)
	duration := time.Since(start)

	docSize := int64(len(req.GetDocumentContent()))

	auditErr := ac.logAudit(ctx, audit.EventSidecarExtractText, &auditLogEntry{
		Operation: "ExtractText",
		Success:   err == nil,
		Duration:  duration,
		FileSize:  docSize,
		Metadata: map[string]interface{}{
			"format":     req.GetDocumentFormat(),
			"request_id": req.GetMetadata().GetRequestId(),
		},
	})

	if auditErr != nil {
		log.Printf("audit log warning: %v\n", auditErr)
	}

	if err != nil {
		return nil, err
	}

	return resp, nil
}

// ProcessDocument processes a document with audit logging
func (ac *AuditClient) ProcessDocument(ctx context.Context, req *ProcessDocumentRequest) (*ProcessDocumentResponse, error) {
	start := time.Now()
	resp, err := ac.client.ProcessDocument(ctx, req)
	duration := time.Since(start)

	docSize := int64(len(req.GetInputContent()))

	auditErr := ac.logAudit(ctx, audit.EventSidecarProcessDocument, &auditLogEntry{
		Operation: "ProcessDocument",
		Success:   err == nil,
		Duration:  duration,
		FileSize:  docSize,
		Metadata: map[string]interface{}{
			"operation":     req.GetOperation(),
			"input_format":  req.GetInputFormat(),
			"output_format": req.GetOutputFormat(),
			"request_id":    req.GetMetadata().GetRequestId(),
		},
	})

	if auditErr != nil {
		log.Printf("audit log warning: %v\n", auditErr)
	}

	if err != nil {
		return nil, err
	}

	return resp, nil
}

// LogQueueEvent logs when a request is queued due to sidecar being unavailable
func (ac *AuditClient) LogQueueEvent(ctx context.Context, operation string, queueSize int) error {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		md = metadata.MD{}
	}

	requestID, userID, agentID, sessionID := ac.extractFn(md)

	return ac.auditLog.LogEvent(audit.EventSidecarQueued, sessionID, "", userID, map[string]interface{}{
		"operation":  operation,
		"request_id": requestID,
		"user_id":    userID,
		"agent_id":   agentID,
		"queue_size": queueSize,
	})
}

// LogRetryEvent logs when a queued request is retried
func (ac *AuditClient) LogRetryEvent(ctx context.Context, operation string, attempt int) error {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		md = metadata.MD{}
	}

	requestID, userID, agentID, sessionID := ac.extractFn(md)

	return ac.auditLog.LogEvent(audit.EventSidecarRetry, sessionID, "", userID, map[string]interface{}{
		"operation":  operation,
		"request_id": requestID,
		"user_id":    userID,
		"agent_id":   agentID,
		"attempt":    attempt,
	})
}

// GetClient returns the underlying sidecar client
func (ac *AuditClient) GetClient() *Client {
	return ac.client
}
