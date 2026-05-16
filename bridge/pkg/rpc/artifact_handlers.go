package rpc

import (
	"context"
	"encoding/json"

	"github.com/armorclaw/bridge/pkg/secretary"
)

// artifactRPCHandlerAdapter wraps ArtifactRPCHandler to bridge the signature
// mismatch between the RPC server's (method, params) style and the secretary
// package's RPCRequest/RPCResponse style.
type artifactRPCHandlerAdapter struct {
	handler *secretary.ArtifactRPCHandler
}

func NewArtifactRPCHandlerAdapter(h *secretary.ArtifactRPCHandler) *artifactRPCHandlerAdapter {
	if h == nil {
		return nil
	}
	return &artifactRPCHandlerAdapter{handler: h}
}

func (a *artifactRPCHandlerAdapter) Handle(method string, params json.RawMessage) (interface{}, error) {
	var userID string
	if len(params) > 0 {
		var p map[string]json.RawMessage
		if json.Unmarshal(params, &p) == nil {
			if uid, ok := p["user_id"]; ok {
				_ = json.Unmarshal(uid, &userID)
			}
		}
	}
	if userID == "" {
		userID = "rpc"
	}

	req := &secretary.RPCRequest{
		Method: method,
		Params: params,
		UserID: userID,
	}

	resp := a.handler.Handle(req)
	if resp == nil {
		return nil, nil
	}

	if resp.Error != nil {
		return nil, &secretaryHandlerError{
			code:    resp.Error.Code,
			message: resp.Error.Message,
		}
	}

	return resp.Result, nil
}

// handleArtifactMethod dispatches secretary.artifact_* RPC methods.
func (s *Server) handleArtifactMethod(ctx context.Context, req *Request) (interface{}, *ErrorObj) {
	if s.artifactHandler == nil {
		return nil, &ErrorObj{
			Code:    InternalError,
			Message: "artifact service not initialized",
		}
	}

	result, err := s.artifactHandler.Handle(req.Method, req.Params)
	if err != nil {
		if handlerErr, ok := err.(*secretaryHandlerError); ok {
			return nil, &ErrorObj{
				Code:    handlerErr.code,
				Message: handlerErr.message,
			}
		}
		return nil, &ErrorObj{
			Code:    InternalError,
			Message: err.Error(),
		}
	}

	return result, nil
}
