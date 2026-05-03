package rpc

import (
	"context"
	"log/slog"
	"time"

	"github.com/armorclaw/bridge/pkg/audit"
)

type e2eeToggleResponse struct {
	Enabled  bool   `json:"enabled"`
	Previous bool   `json:"previous"`
	Caller   string `json:"caller"`
	At       string `json:"at"`
}

func (s *Server) handleE2EEEnable(ctx context.Context, req *Request) (interface{}, *ErrorObj) {
	caller := s.requireAdmin(ctx, req)
	if caller == "" {
		return nil, &ErrorObj{Code: -32603, Message: "Not authenticated"}
	}

	previous := s.e2eeEnabled.Swap(true)
	slog.Info("e2ee_enabled", "caller", caller, "previous", previous)

	s.auditGovernanceMutation(audit.EventType("e2ee_enabled"), caller, map[string]interface{}{
		"action":   "enable",
		"previous": previous,
		"current":  true,
	})

	return e2eeToggleResponse{
		Enabled:  true,
		Previous: previous,
		Caller:   caller,
		At:       time.Now().UTC().Format(time.RFC3339),
	}, nil
}

func (s *Server) handleE2EEDisable(ctx context.Context, req *Request) (interface{}, *ErrorObj) {
	caller := s.requireAdmin(ctx, req)
	if caller == "" {
		return nil, &ErrorObj{Code: -32603, Message: "Not authenticated"}
	}

	previous := s.e2eeEnabled.Swap(false)
	slog.Info("e2ee_disabled", "caller", caller, "previous", previous)

	s.auditGovernanceMutation(audit.EventType("e2ee_disabled"), caller, map[string]interface{}{
		"action":   "disable",
		"previous": previous,
		"current":  false,
	})

	return e2eeToggleResponse{
		Enabled:  false,
		Previous: previous,
		Caller:   caller,
		At:       time.Now().UTC().Format(time.RFC3339),
	}, nil
}

func (s *Server) IsE2EERuntimeEnabled() bool {
	return s.e2eeEnabled.Load()
}

func (s *Server) requireAdmin(ctx context.Context, req *Request) string {
	matrixAdapter, ok := s.matrix.(interface{ GetUserID() string })
	if !ok || matrixAdapter == nil {
		return ""
	}
	userID := matrixAdapter.GetUserID()
	if userID == "" {
		return ""
	}
	return userID
}
