package secretary

import (
	"context"
	"encoding/json"
	"fmt"
)

//=============================================================================
// Artifact RPC Handlers
//=============================================================================

// ArtifactRPCHandler handles secretary.artifact_* RPC methods.
// It operates on the ArtifactStore and enforces fail-closed security.
type ArtifactRPCHandler struct {
	store *ArtifactStore
}

// NewArtifactRPCHandler creates a handler backed by the given store.
func NewArtifactRPCHandler(store *ArtifactStore) *ArtifactRPCHandler {
	return &ArtifactRPCHandler{store: store}
}

// Handle dispatches an artifact RPC method.
func (h *ArtifactRPCHandler) Handle(req *RPCRequest) *RPCResponse {
	if req.UserID == "" {
		return ErrorResponse(-32001, "authentication required")
	}

	switch req.Method {
	case "secretary.artifact_upload":
		return h.handleUpload(req)
	case "secretary.artifact_download":
		return h.handleDownload(req)
	case "secretary.artifact_list":
		return h.handleList(req)
	case "secretary.artifact_update_status":
		return h.handleUpdateStatus(req)
	default:
		return ErrorResponse(ErrNotFound, fmt.Sprintf("Unknown artifact method: %s", req.Method))
	}
}

// --- Upload ---

type ArtifactUploadParams struct {
	Owner      string            `json:"owner"`
	WorkflowID string            `json:"workflow_id"`
	StepID     string            `json:"step_id,omitempty"`
	Metadata   ArtifactMetadata  `json:"metadata"`
	Checksum   string            `json:"checksum,omitempty"`
}

func (h *ArtifactRPCHandler) handleUpload(req *RPCRequest) *RPCResponse {
	var params ArtifactUploadParams
	if err := json.Unmarshal(req.Params, &params); err != nil {
		return ErrorResponse(ErrInvalidParams, "invalid upload params: "+err.Error())
	}

	meta := SanitizeMetadata(params.Metadata)

	env, err := NewArtifactEnvelope(params.Owner, params.WorkflowID, meta)
	if err != nil {
		return ErrorResponse(ErrValidation, err.Error())
	}
	env.StepID = params.StepID
	if params.Checksum != "" {
		env.Checksum = params.Checksum
	}

	if err := env.Validate(); err != nil {
		return ErrorResponse(ErrValidation, err.Error())
	}

	ctx := context.Background()
	if err := h.store.StoreEnvelope(ctx, env); err != nil {
		return ErrorResponse(ErrInternal, "store failed: "+err.Error())
	}

	return SuccessResponse(map[string]interface{}{
		"id":      env.ID,
		"status":  env.Status,
		"version": env.Version,
	})
}

// --- Download ---

type ArtifactDownloadParams struct {
	ArtifactID string `json:"artifact_id"`
}

func (h *ArtifactRPCHandler) handleDownload(req *RPCRequest) *RPCResponse {
	var params ArtifactDownloadParams
	if err := json.Unmarshal(req.Params, &params); err != nil {
		return ErrorResponse(ErrInvalidParams, "invalid download params: "+err.Error())
	}
	if params.ArtifactID == "" {
		return ErrorResponse(ErrInvalidParams, "artifact_id is required")
	}

	ctx := context.Background()
	env, err := h.store.GetEnvelope(ctx, params.ArtifactID)
	if err != nil {
		return ErrorResponse(ErrInternal, "query failed: "+err.Error())
	}
	if env == nil {
		return ErrorResponse(ErrNotFound, "artifact not found")
	}

	// Authorization check: user must be the owner
	if env.Owner != req.UserID {
		return ErrorResponse(-32001, "not authorized to access this artifact")
	}

	return SuccessResponse(env)
}

// --- List ---

type ArtifactListParams struct {
	Owner      string `json:"owner,omitempty"`
	WorkflowID string `json:"workflow_id,omitempty"`
	Status     string `json:"status,omitempty"`
}

func (h *ArtifactRPCHandler) handleList(req *RPCRequest) *RPCResponse {
	var params ArtifactListParams
	if err := json.Unmarshal(req.Params, &params); err != nil {
		return ErrorResponse(ErrInvalidParams, "invalid list params: "+err.Error())
	}

	filter := ArtifactFilter{
		Owner:      params.Owner,
		WorkflowID: params.WorkflowID,
	}
	if params.Status != "" {
		s := ArtifactStatus(params.Status)
		filter.Status = &s
	}

	ctx := context.Background()
	envelopes, err := h.store.ListEnvelopes(ctx, filter)
	if err != nil {
		return ErrorResponse(ErrInternal, "query failed: "+err.Error())
	}
	if envelopes == nil {
		envelopes = []ArtifactEnvelope{}
	}

	return SuccessResponse(map[string]interface{}{
		"artifacts": envelopes,
		"count":     len(envelopes),
	})
}

// --- Update Status ---

type ArtifactUpdateStatusParams struct {
	ArtifactID string `json:"artifact_id"`
	Status     string `json:"status"`
}

func (h *ArtifactRPCHandler) handleUpdateStatus(req *RPCRequest) *RPCResponse {
	var params ArtifactUpdateStatusParams
	if err := json.Unmarshal(req.Params, &params); err != nil {
		return ErrorResponse(ErrInvalidParams, "invalid update_status params: "+err.Error())
	}
	if params.ArtifactID == "" {
		return ErrorResponse(ErrInvalidParams, "artifact_id is required")
	}
	if params.Status == "" {
		return ErrorResponse(ErrInvalidParams, "status is required")
	}

	newStatus := ArtifactStatus(params.Status)
	if !ValidArtifactStatuses[newStatus] {
		return ErrorResponse(ErrValidation, fmt.Sprintf("invalid status: %q", params.Status))
	}

	ctx := context.Background()
	env, err := h.store.GetEnvelope(ctx, params.ArtifactID)
	if err != nil {
		return ErrorResponse(ErrInternal, "query failed: "+err.Error())
	}
	if env == nil {
		return ErrorResponse(ErrNotFound, "artifact not found")
	}

	// Authorization check
	if env.Owner != req.UserID {
		return ErrorResponse(-32001, "not authorized to modify this artifact")
	}

	if err := env.TransitionStatus(newStatus); err != nil {
		return ErrorResponse(ErrValidation, err.Error())
	}

	if err := h.store.UpdateStatus(ctx, params.ArtifactID, newStatus); err != nil {
		return ErrorResponse(ErrInternal, "update failed: "+err.Error())
	}

	return SuccessResponse(map[string]interface{}{
		"id":     params.ArtifactID,
		"status": newStatus,
	})
}
