package provisioning

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"unicode"
)

// RPCHandler provides JSON-RPC handlers for provisioning
type RPCHandler struct {
	manager *Manager
}

// NewRPCHandler creates a new RPC handler
func NewRPCHandler(manager *Manager) *RPCHandler {
	return &RPCHandler{manager: manager}
}

// Method names
const (
	MethodStart          = "provisioning.start"
	MethodStatus         = "provisioning.status"
	MethodCancel         = "provisioning.cancel"
	MethodClaim          = "provisioning.claim"
	MethodRotate         = "provisioning.rotate"
	MethodList           = "provisioning.list"
	MethodGetQR          = "provisioning.get_qr"
)

// RPCRequest represents a generic RPC request
type RPCRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params"`
	ID      interface{}     `json:"id"`
}

// RPCResponse represents a generic RPC response
type RPCResponse struct {
	JSONRPC string      `json:"jsonrpc"`
	Result  interface{} `json:"result,omitempty"`
	Error   *RPCError   `json:"error,omitempty"`
	ID      interface{} `json:"id"`
}

// RPCError represents an RPC error
type RPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// validatePassphraseStrength checks that a password meets minimum strength requirements.
// Requirements: ≥12 characters, at least 1 uppercase, 1 lowercase, 1 digit.
func validatePassphraseStrength(password string) error {
	if len(password) < 12 {
		return fmt.Errorf("passphrase must be at least 12 characters (got %d)", len(password))
	}

	var hasUpper, hasLower, hasDigit bool
	for _, r := range password {
		switch {
		case unicode.IsUpper(r):
			hasUpper = true
		case unicode.IsLower(r):
			hasLower = true
		case unicode.IsDigit(r):
			hasDigit = true
		}
	}

	if !hasUpper {
		return fmt.Errorf("passphrase must contain at least 1 uppercase letter")
	}
	if !hasLower {
		return fmt.Errorf("passphrase must contain at least 1 lowercase letter")
	}
	if !hasDigit {
		return fmt.Errorf("passphrase must contain at least 1 digit")
	}

	return nil
}

// StartRequest is the request for provisioning.start
type StartRequest struct {
	ExpirySeconds int    `json:"expires_in,omitempty"`
	DeviceName    string `json:"device_name,omitempty"`
}

// ServerConfigResponse is the nested server config in provisioning responses
type ServerConfigResponse struct {
	MatrixHomeserver string `json:"matrix_homeserver"`
	RPCURL           string `json:"rpc_url"`
	WSURL            string `json:"ws_url"`
	PushGateway      string `json:"push_gateway,omitempty"`
	ServerName       string `json:"server_name"`
}

// StartResponse is the response for provisioning.start (ArmorChat contract)
type StartResponse struct {
	ProvisioningID string                `json:"provisioning_id"`
	QRData         string                `json:"qr_data"`
	SetupToken     string                `json:"setup_token"`
	ExpiresAt      int64                 `json:"expires_at"`
	ServerConfig   *ServerConfigResponse `json:"server_config"`
}

// StatusResponse is the response for provisioning.status (ArmorChat contract)
type StatusResponse struct {
	ProvisioningID string       `json:"provisioning_id"`
	Status         string       `json:"status"`
	ExpiresAt      int64        `json:"expires_at"`
	ClaimedBy      *DeviceInfo  `json:"claimed_by"`
	ClaimedAt      *int64       `json:"claimed_at"`
}

// ClaimRequest is the request for provisioning.claim (ArmorChat contract)
type ClaimRequest struct {
	SetupToken    string `json:"setup_token"`
	DeviceID      string `json:"device_id"`
	DeviceName    string `json:"device_name"`
	DeviceType    string `json:"device_type,omitempty"`
	UserAgent     string `json:"user_agent,omitempty"`
	CorrelationID string `json:"correlation_id,omitempty"`
}

// ClaimResponse is the response for provisioning.claim (ArmorChat contract)
type ClaimResponse struct {
	Success          bool   `json:"success"`
	Role             string `json:"role,omitempty"`
	AdminToken       string `json:"admin_token,omitempty"`
	UserID           string `json:"user_id,omitempty"`
	DeviceID         string `json:"device_id,omitempty"`
	Message          string `json:"message"`
	MatrixHomeserver string `json:"matrix_homeserver,omitempty"`
	CorrelationID    string `json:"correlation_id,omitempty"`
}

// Handle handles an RPC request
func (h *RPCHandler) Handle(ctx context.Context, req *RPCRequest) *RPCResponse {
	var result interface{}
	var err error

	switch req.Method {
	case MethodStart:
		result, err = h.handleStart(ctx, req.Params)
	case MethodStatus:
		result, err = h.handleStatus(ctx, req.Params)
	case MethodCancel:
		result, err = h.handleCancel(ctx, req.Params)
	case MethodClaim:
		result, err = h.handleClaim(ctx, req.Params)
	case MethodRotate:
		result, err = h.handleRotate(ctx, req.Params)
	case MethodList:
		result, err = h.handleList(ctx, req.Params)
	case MethodGetQR:
		result, err = h.handleGetQR(ctx, req.Params)
	default:
		return &RPCResponse{
			JSONRPC: "2.0",
			Error:   &RPCError{Code: -32601, Message: "method not found"},
			ID:      req.ID,
		}
	}

	if err != nil {
		return &RPCResponse{
			JSONRPC: "2.0",
			Error:   &RPCError{Code: -32000, Message: err.Error()},
			ID:      req.ID,
		}
	}

	return &RPCResponse{
		JSONRPC: "2.0",
		Result:  result,
		ID:      req.ID,
	}
}

func (h *RPCHandler) handleStart(ctx context.Context, params json.RawMessage) (*StartResponse, error) {
	var req StartRequest
	if len(params) > 0 {
		if err := json.Unmarshal(params, &req); err != nil {
			return nil, fmt.Errorf("invalid params: %w", err)
		}
	}

	opts := &StartTokenOptions{
		ExpirySeconds: req.ExpirySeconds,
		DeviceName:    req.DeviceName,
	}

	token, err := h.manager.StartToken(ctx, opts)
	if err != nil {
		return nil, err
	}

	qrData, err := h.manager.GetQRData(token)
	if err != nil {
		return nil, err
	}

	return &StartResponse{
		ProvisioningID: token.ID,
		QRData:         qrData,
		SetupToken:     token.Config.SetupToken,
		ExpiresAt:      token.ExpiresAt.Unix(),
		ServerConfig: &ServerConfigResponse{
			MatrixHomeserver: token.Config.MatrixHomeserver,
			RPCURL:           token.Config.RPCURL,
			WSURL:            token.Config.WSURL,
			PushGateway:      token.Config.PushGateway,
			ServerName:       token.Config.ServerName,
		},
	}, nil
}

func (h *RPCHandler) handleStatus(ctx context.Context, params json.RawMessage) (*StatusResponse, error) {
	var req struct {
		ProvisioningID string `json:"provisioning_id"`
	}
	if err := json.Unmarshal(params, &req); err != nil {
		return nil, fmt.Errorf("invalid params: %w", err)
	}

	token, err := h.manager.GetToken(req.ProvisioningID)
	if err != nil {
		return nil, err
	}

	var claimedAt *int64
	if token.ClaimedAt != nil {
		unix := token.ClaimedAt.Unix()
		claimedAt = &unix
	}

	return &StatusResponse{
		ProvisioningID: token.ID,
		Status:         string(token.Status),
		ExpiresAt:      token.ExpiresAt.Unix(),
		ClaimedBy:      token.ClaimedBy,
		ClaimedAt:      claimedAt,
	}, nil
}

func (h *RPCHandler) handleCancel(ctx context.Context, params json.RawMessage) (map[string]interface{}, error) {
	var req struct {
		ProvisioningID string `json:"provisioning_id"`
	}
	if err := json.Unmarshal(params, &req); err != nil {
		return nil, fmt.Errorf("invalid params: %w", err)
	}

	if err := h.manager.CancelToken(req.ProvisioningID); err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"success": true,
		"message": "Provisioning session cancelled",
	}, nil
}

func (h *RPCHandler) handleClaim(ctx context.Context, params json.RawMessage) (*ClaimResponse, error) {
	var req ClaimRequest
	if err := json.Unmarshal(params, &req); err != nil {
		// Return success:false instead of RPC error — ArmorChat checks result.success
		return &ClaimResponse{
			Success: false,
			Message: "invalid request params",
		}, nil
	}

	if req.SetupToken == "" {
		return &ClaimResponse{
			Success: false,
			Message: "setup_token is required",
		}, nil
	}

	// Generate device_id if not provided (ArmorChat sends device_name + device_type only)
	deviceID := req.DeviceID
	if deviceID == "" {
		deviceID = generateDeviceID(req.DeviceName, req.DeviceType)
	}

	// Resolve setup_token → token_id by scanning active tokens
	tokenID := h.manager.ResolveSetupToken(req.SetupToken)
	if tokenID == "" {
		return &ClaimResponse{
			Success:       false,
			Message:       "invalid or expired setup_token",
			CorrelationID: req.CorrelationID,
		}, nil
	}

	device := &DeviceInfo{
		DeviceID:   deviceID,
		DeviceName: req.DeviceName,
		UserAgent:  req.UserAgent,
	}

	token, role, adminToken, err := h.manager.ClaimTokenWithRole(ctx, tokenID, device, RoleNone)
	if err != nil {
		// Return success:false instead of RPC error — ArmorChat checks result.success
		return &ClaimResponse{
			Success:       false,
			Message:       err.Error(),
			CorrelationID: req.CorrelationID,
		}, nil
	}

	// Build Matrix-style user_id: @<device-hash>:<server_name>
	serverName := ""
	if token.Config != nil {
		serverName = token.Config.ServerName
	}
	userID := generateMatrixUserID(deviceID, serverName)

	message := "Admin role claimed successfully"
	if role != RoleOwner {
		message = "Device registered successfully"
	}

	return &ClaimResponse{
		Success:          true,
		Role:             string(role),
		AdminToken:       adminToken,
		UserID:           userID,
		DeviceID:         deviceID,
		Message:          message,
		MatrixHomeserver: token.Config.MatrixHomeserver,
		CorrelationID:    req.CorrelationID,
	}, nil
}

func (h *RPCHandler) handleRotate(ctx context.Context, params json.RawMessage) (map[string]interface{}, error) {
	var req struct {
		NewSecret     string `json:"new_secret"`
		CorrelationID string `json:"correlation_id,omitempty"`
	}
	if len(params) > 0 {
		if err := json.Unmarshal(params, &req); err != nil {
			return nil, fmt.Errorf("invalid params: %w", err)
		}
	}

	// If no new secret provided, auto-generate one
	if req.NewSecret == "" {
		b := make([]byte, 32)
		if _, err := rand.Read(b); err != nil {
			return nil, fmt.Errorf("failed to generate new secret: %w", err)
		}
		req.NewSecret = hex.EncodeToString(b)
	} else {
		if err := validatePassphraseStrength(req.NewSecret); err != nil {
			return nil, fmt.Errorf("weak passphrase: %w", err)
		}
	}

	if err := h.manager.RotateSecret(req.NewSecret); err != nil {
		return nil, err
	}

	// Generate a new provisioning token with the rotated secret
	token, err := h.manager.StartToken(context.Background(), nil)
	if err != nil {
		return map[string]interface{}{
			"success": true,
			"message": "Provisioning secret rotated (failed to generate new token)",
		}, nil
	}

	qrData, _ := h.manager.GetQRData(token)

	return map[string]interface{}{
		"success":         true,
		"new_setup_token": token.Config.SetupToken,
		"new_qr_data":     qrData,
		"expires_at":      token.ExpiresAt.Unix(),
		"message":         "Provisioning secret rotated",
	}, nil
}

func (h *RPCHandler) handleList(ctx context.Context, params json.RawMessage) ([]*StatusResponse, error) {
	tokens := h.manager.ListTokens()

	// Always return an array, never null
	result := make([]*StatusResponse, 0, len(tokens))
	for _, token := range tokens {
		result = append(result, &StatusResponse{
			ProvisioningID: token.ID,
			Status:         string(token.Status),
			ExpiresAt:      token.ExpiresAt.Unix(),
		})
	}

	return result, nil
}

func (h *RPCHandler) handleGetQR(ctx context.Context, params json.RawMessage) (map[string]interface{}, error) {
	var req struct {
		TokenID string `json:"token_id"`
	}
	if err := json.Unmarshal(params, &req); err != nil {
		return nil, fmt.Errorf("invalid params: %w", err)
	}

	token, err := h.manager.GetToken(req.TokenID)
	if err != nil {
		return nil, err
	}

	qrData, err := h.manager.GetQRData(token)
	if err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"token_id":   token.ID,
		"qr_data":    qrData,
		"deep_link":  qrData,
		"expires_at": token.ExpiresAt.Unix(),
		"status":     token.Status,
	}, nil
}
