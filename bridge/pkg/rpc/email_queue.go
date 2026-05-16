package rpc

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/armorclaw/bridge/pkg/email"
)

// OutboxStoreReader defines the read interface for the email outbox store.
type OutboxStoreReader interface {
	GetQueueStats(ctx context.Context) (map[string]interface{}, error)
	GetByID(ctx context.Context, id string) (*email.OutboxEntry, error)
	ListByStatus(ctx context.Context, status string, limit, offset int) ([]*email.OutboxEntry, error)
	Retry(ctx context.Context, id string) error
}

// ---------------------------------------------------------------------------
// email.queue_status
// ---------------------------------------------------------------------------

func (s *Server) handleEmailQueueStatus(ctx context.Context, req *Request) (interface{}, *ErrorObj) {
	var params struct {
		Token string `json:"token"`
	}

	if err := json.Unmarshal(req.Params, &params); err != nil {
		return nil, &ErrorObj{
			Code:    InvalidParams,
			Message: "invalid parameters: " + err.Error(),
		}
	}

	store := s.getEmailOutboxStore()
	if store == nil {
		return nil, &ErrorObj{
			Code:    InternalError,
			Message: "email outbox store not configured",
		}
	}

	stats, err := store.GetQueueStats(ctx)
	if err != nil {
		return nil, &ErrorObj{
			Code:    InternalError,
			Message: "failed to get queue stats: " + err.Error(),
		}
	}

	return stats, nil
}

// ---------------------------------------------------------------------------
// email.get
// ---------------------------------------------------------------------------

func (s *Server) handleEmailGet(ctx context.Context, req *Request) (interface{}, *ErrorObj) {
	var params struct {
		Token   string `json:"token"`
		EmailID string `json:"email_id"`
	}

	if err := json.Unmarshal(req.Params, &params); err != nil {
		return nil, &ErrorObj{
			Code:    InvalidParams,
			Message: "invalid parameters: " + err.Error(),
		}
	}

	if params.EmailID == "" {
		return nil, &ErrorObj{
			Code:    InvalidParams,
			Message: "email_id is required",
		}
	}

	store := s.getEmailOutboxStore()
	if store == nil {
		return nil, &ErrorObj{
			Code:    InternalError,
			Message: "email outbox store not configured",
		}
	}

	entry, err := store.GetByID(ctx, params.EmailID)
	if err != nil {
		return nil, &ErrorObj{
			Code:    InternalError,
			Message: "failed to get email: " + err.Error(),
		}
	}

	if entry == nil {
		return nil, &ErrorObj{
			Code:    InvalidParams,
			Message: fmt.Sprintf("email %q not found", params.EmailID),
		}
	}

	return entry, nil
}

// ---------------------------------------------------------------------------
// email.retry
// ---------------------------------------------------------------------------

func (s *Server) handleEmailRetry(ctx context.Context, req *Request) (interface{}, *ErrorObj) {
	var params struct {
		Token   string `json:"token"`
		EmailID string `json:"email_id"`
	}

	if err := json.Unmarshal(req.Params, &params); err != nil {
		return nil, &ErrorObj{
			Code:    InvalidParams,
			Message: "invalid parameters: " + err.Error(),
		}
	}

	if params.EmailID == "" {
		return nil, &ErrorObj{
			Code:    InvalidParams,
			Message: "email_id is required",
		}
	}

	store := s.getEmailOutboxStore()
	if store == nil {
		return nil, &ErrorObj{
			Code:    InternalError,
			Message: "email outbox store not configured",
		}
	}

	if err := store.Retry(ctx, params.EmailID); err != nil {
		return nil, &ErrorObj{
			Code:    InternalError,
			Message: "failed to retry email: " + err.Error(),
		}
	}

	return map[string]interface{}{
		"email_id":   params.EmailID,
		"status":     "queued",
		"retried_at": time.Now().Format(time.RFC3339),
	}, nil
}

// ---------------------------------------------------------------------------
// email.list
// ---------------------------------------------------------------------------

func (s *Server) handleEmailList(ctx context.Context, req *Request) (interface{}, *ErrorObj) {
	var params struct {
		Token   string `json:"token"`
		Status  string `json:"status,omitempty"`
		Limit   int    `json:"limit,omitempty"`
		Offset  int    `json:"offset,omitempty"`
	}

	if err := json.Unmarshal(req.Params, &params); err != nil {
		return nil, &ErrorObj{
			Code:    InvalidParams,
			Message: "invalid parameters: " + err.Error(),
		}
	}

	if params.Limit <= 0 {
		params.Limit = 50
	}

	store := s.getEmailOutboxStore()
	if store == nil {
		return nil, &ErrorObj{
			Code:    InternalError,
			Message: "email outbox store not configured",
		}
	}

	entries, err := store.ListByStatus(ctx, params.Status, params.Limit, params.Offset)
	if err != nil {
		return nil, &ErrorObj{
			Code:    InternalError,
			Message: "failed to list emails: " + err.Error(),
		}
	}

	return map[string]interface{}{
		"emails": entries,
	}, nil
}

// getEmailOutboxStore returns the outbox store from the Server, or nil if not configured.
func (s *Server) getEmailOutboxStore() OutboxStoreReader {
	if s.outboxStore == nil {
		return nil
	}
	return s.outboxStore
}
