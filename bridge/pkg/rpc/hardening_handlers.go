// Package rpc provides JSON-RPC 2.0 server for ArmorClaw bridge communication.
// This file contains handlers for first-login hardening operations.
package rpc

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"

	"github.com/armorclaw/bridge/internal/adapter"
	"github.com/armorclaw/bridge/pkg/trust"
)

// HardeningHandler handles hardening-related RPC methods
type HardeningHandler struct {
	store         trust.Store
	matrixAdapter *adapter.MatrixAdapter
	bootstrapPath string // Path to bootstrap password file
}

func NewHardeningHandler(store trust.Store, matrixAdapter *adapter.MatrixAdapter, bootstrapPath string) *HardeningHandler {
	return &HardeningHandler{
		store:         store,
		matrixAdapter: matrixAdapter,
		bootstrapPath: bootstrapPath,
	}
}

// handleHardeningStatus returns the current hardening state for the authenticated user
func (h *HardeningHandler) handleHardeningStatus(ctx context.Context, req *Request) (interface{}, *ErrorObj) {
	if h.store == nil {
		return nil, &ErrorObj{
			Code:    InternalError,
			Message: "Hardening store not configured",
		}
	}

	if h.matrixAdapter == nil {
		return nil, &ErrorObj{
			Code:    InternalError,
			Message: "Matrix adapter not configured",
		}
	}

	userID := h.matrixAdapter.GetUserID()
	if userID == "" {
		return nil, &ErrorObj{
			Code:    InternalError,
			Message: "Not authenticated",
		}
	}

	state, err := h.store.Get(userID)
	if err != nil {
		return nil, &ErrorObj{
			Code:    InternalError,
			Message: fmt.Sprintf("Failed to retrieve hardening state: %s", err.Error()),
		}
	}

	return state, nil
}

// handleHardeningAck marks a specific hardening step as complete
func (h *HardeningHandler) handleHardeningAck(ctx context.Context, req *Request) (interface{}, *ErrorObj) {
	if h.store == nil {
		return nil, &ErrorObj{
			Code:    InternalError,
			Message: "Hardening store not configured",
		}
	}

	if h.matrixAdapter == nil {
		return nil, &ErrorObj{
			Code:    InternalError,
			Message: "Matrix adapter not configured",
		}
	}

	var params struct {
		Step string `json:"step"`
	}

	if len(req.Params) == 0 {
		return nil, &ErrorObj{
			Code:    InvalidParams,
			Message: "step parameter is required",
		}
	}

	if err := json.Unmarshal(req.Params, &params); err != nil {
		return nil, &ErrorObj{
			Code:    InvalidParams,
			Message: err.Error(),
		}
	}

	if params.Step == "" {
		return nil, &ErrorObj{
			Code:    InvalidParams,
			Message: "step parameter cannot be empty",
		}
	}

	validSteps := map[string]bool{
		string(trust.PasswordRotated):   true,
		string(trust.BootstrapWiped):    true,
		string(trust.DeviceVerified):    true,
		string(trust.RecoveryBackedUp):  true,
		string(trust.BiometricsEnabled): true,
	}

	if !validSteps[params.Step] {
		return nil, &ErrorObj{
			Code:    InvalidParams,
			Message: fmt.Sprintf("Invalid step: %s", params.Step),
		}
	}

	userID := h.matrixAdapter.GetUserID()
	if userID == "" {
		return nil, &ErrorObj{
			Code:    InternalError,
			Message: "Not authenticated",
		}
	}

	if err := h.store.AckStep(userID, trust.HardeningStep(params.Step)); err != nil {
		return nil, &ErrorObj{
			Code:    InternalError,
			Message: fmt.Sprintf("Failed to acknowledge step: %s", err.Error()),
		}
	}

	state, err := h.store.Get(userID)
	if err != nil {
		return nil, &ErrorObj{
			Code:    InternalError,
			Message: fmt.Sprintf("Failed to retrieve updated hardening state: %s", err.Error()),
		}
	}

	return state, nil
}

// handleHardeningRotatePassword rotates the user's Matrix password and cleans up bootstrap files
func (h *HardeningHandler) handleHardeningRotatePassword(ctx context.Context, req *Request) (interface{}, *ErrorObj) {
	if h.matrixAdapter == nil {
		return nil, &ErrorObj{
			Code:    InternalError,
			Message: "Matrix adapter not configured",
		}
	}

	if h.store == nil {
		return nil, &ErrorObj{
			Code:    InternalError,
			Message: "Hardening store not configured",
		}
	}

	var params struct {
		NewPassword string `json:"new_password"`
	}

	if len(req.Params) == 0 {
		return nil, &ErrorObj{
			Code:    InvalidParams,
			Message: "new_password parameter is required",
		}
	}

	if err := json.Unmarshal(req.Params, &params); err != nil {
		return nil, &ErrorObj{
			Code:    InvalidParams,
			Message: err.Error(),
		}
	}

	if params.NewPassword == "" {
		return nil, &ErrorObj{
			Code:    InvalidParams,
			Message: "new_password cannot be empty",
		}
	}

	if len(params.NewPassword) < 8 {
		return nil, &ErrorObj{
			Code:    InvalidParams,
			Message: "new_password must be at least 8 characters",
		}
	}

	userID := h.matrixAdapter.GetUserID()
	if userID == "" {
		return nil, &ErrorObj{
			Code:    InternalError,
			Message: "Not authenticated",
		}
	}

	if err := h.matrixAdapter.ChangePassword(ctx, params.NewPassword, true); err != nil {
		return nil, &ErrorObj{
			Code:    InternalError,
			Message: fmt.Sprintf("Failed to change password: %s", err.Error()),
		}
	}

	if h.bootstrapPath != "" {
		if err := os.Remove(h.bootstrapPath); err != nil && !os.IsNotExist(err) {
			log.Printf("[hardening] Warning: failed to delete bootstrap file %s: %v\n", h.bootstrapPath, err)
		}
	}

	if err := h.store.AckStep(userID, trust.PasswordRotated); err != nil {
		return nil, &ErrorObj{
			Code:    InternalError,
			Message: fmt.Sprintf("Failed to acknowledge password rotation: %s", err.Error()),
		}
	}

	if err := h.store.AckStep(userID, trust.BootstrapWiped); err != nil {
		return nil, &ErrorObj{
			Code:    InternalError,
			Message: fmt.Sprintf("Failed to acknowledge bootstrap wipe: %s", err.Error()),
		}
	}

	state, err := h.store.Get(userID)
	if err != nil {
		return nil, &ErrorObj{
			Code:    InternalError,
			Message: fmt.Sprintf("Failed to retrieve updated hardening state: %s", err.Error()),
		}
	}

	return state, nil
}
