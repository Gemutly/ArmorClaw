package rpc

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/armorclaw/bridge/pkg/trust"
)

func TestAccountDeleteMethodRegistration(t *testing.T) {
	server := &Server{}
	server.registerHandlers()

	if _, exists := server.handlers["account.delete"]; !exists {
		t.Error("account.delete method not registered")
	}
}

func TestAccountDeleteMissingPassword(t *testing.T) {
	server := &Server{}
	server.registerHandlers()

	handler := server.handlers["account.delete"]

	tests := []struct {
		name   string
		params string
	}{
		{"empty params", `{}`},
		{"empty password", `{"password": ""}`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := &Request{
				Params: json.RawMessage(tt.params),
			}
			_, errObj := handler(context.Background(), req)
			if errObj == nil {
				t.Fatal("expected error for missing password")
			}
			if errObj.Code != InvalidParams {
				t.Errorf("expected InvalidParams, got %d", errObj.Code)
			}
			if errObj.Message != "password is required" {
				t.Errorf("expected 'password is required', got %s", errObj.Message)
			}
		})
	}
}

func TestAccountDeleteInvalidParams(t *testing.T) {
	server := &Server{}
	server.registerHandlers()

	handler := server.handlers["account.delete"]

	req := &Request{
		Params: json.RawMessage(`invalid json`),
	}

	_, errObj := handler(context.Background(), req)
	if errObj == nil {
		t.Fatal("expected error for invalid JSON")
	}
	if errObj.Code != InvalidParams {
		t.Errorf("expected InvalidParams, got %d", errObj.Code)
	}
}

func TestAccountDeleteNoMatrixAdapter(t *testing.T) {
	server := &Server{}
	server.registerHandlers()

	handler := server.handlers["account.delete"]

	req := &Request{
		Params: json.RawMessage(`{"password": "testpassword123"}`),
	}

	_, errObj := handler(context.Background(), req)
	if errObj == nil {
		t.Fatal("expected error when matrix adapter not configured")
	}
	if errObj.Code != InternalError {
		t.Errorf("expected InternalError, got %d", errObj.Code)
	}
	if errObj.Message != "Matrix adapter not configured" {
		t.Errorf("unexpected message: %s", errObj.Message)
	}
}

func TestAccountDeleteHardeningNotComplete(t *testing.T) {
	mockStore := &mockStore{
		state: &trust.UserHardeningState{
			UserID:           "@test:server",
			DelegationReady:  false,
		},
	}

	server := &Server{
		matrix: &mockDeactivatableAdapter{
			userID:     "@test:server",
			loggedIn:   true,
			deactivate: func(ctx context.Context, password string, erase bool) error { return nil },
		},
		hardeningStore: mockStore,
	}
	server.registerHandlers()

	handler := server.handlers["account.delete"]

	req := &Request{
		Params: json.RawMessage(`{"password": "testpassword123"}`),
	}

	_, errObj := handler(context.Background(), req)
	if errObj == nil {
		t.Fatal("expected error when hardening not complete")
	}
	if errObj.Code != InternalError {
		t.Errorf("expected InternalError, got %d", errObj.Code)
	}
	if errObj.Message != "Account hardening must be complete before deletion" {
		t.Errorf("unexpected message: %s", errObj.Message)
	}
}

func TestAccountDeleteSuccess(t *testing.T) {
	deactivated := false
	mockStore := &mockStore{
		state: &trust.UserHardeningState{
			UserID:           "@test:server",
			DelegationReady:  true,
			PasswordRotated:  true,
			BootstrapWiped:   true,
			DeviceVerified:   true,
			RecoveryBackedUp: true,
		},
	}

	server := &Server{
		matrix: &mockDeactivatableAdapter{
			userID:   "@test:server",
			loggedIn: true,
			deactivate: func(ctx context.Context, password string, erase bool) error {
				deactivated = true
				if password != "correctpass" {
					t.Errorf("unexpected password: %s", password)
				}
				return nil
			},
		},
		hardeningStore: mockStore,
	}
	server.registerHandlers()

	handler := server.handlers["account.delete"]

	req := &Request{
		Params: json.RawMessage(`{"password": "correctpass", "erase": true}`),
	}

	result, errObj := handler(context.Background(), req)
	if errObj != nil {
		t.Fatalf("unexpected error: %v", errObj)
	}

	if !deactivated {
		t.Error("expected DeactivateAccount to be called")
	}

	resultMap, ok := result.(map[string]interface{})
	if !ok {
		t.Fatal("result is not a map")
	}
	if resultMap["status"] != "deactivated" {
		t.Errorf("expected status=deactivated, got %v", resultMap["status"])
	}
	if resultMap["user_id"] != "@test:server" {
		t.Errorf("expected user_id=@test:server, got %v", resultMap["user_id"])
	}
	if resultMap["erase"] != true {
		t.Errorf("expected erase=true, got %v", resultMap["erase"])
	}
}

func TestAccountDeleteDeactivationFails(t *testing.T) {
	mockStore := &mockStore{
		state: &trust.UserHardeningState{
			UserID:          "@test:server",
			DelegationReady: true,
		},
	}

	server := &Server{
		matrix: &mockDeactivatableAdapter{
			userID:   "@test:server",
			loggedIn: true,
			deactivate: func(ctx context.Context, password string, erase bool) error {
				return errDeactivationFailed
			},
		},
		hardeningStore: mockStore,
	}
	server.registerHandlers()

	handler := server.handlers["account.delete"]

	req := &Request{
		Params: json.RawMessage(`{"password": "pass123"}`),
	}

	_, errObj := handler(context.Background(), req)
	if errObj == nil {
		t.Fatal("expected error when deactivation fails")
	}
	if errObj.Code != InternalError {
		t.Errorf("expected InternalError, got %d", errObj.Code)
	}
}

func TestAccountDeleteNotAuthenticated(t *testing.T) {
	mockStore := &mockStore{
		state: &trust.UserHardeningState{
			UserID:          "@test:server",
			DelegationReady: true,
		},
	}

	server := &Server{
		matrix: &mockDeactivatableAdapter{
			userID:   "",
			loggedIn: false,
			deactivate: func(ctx context.Context, password string, erase bool) error {
				return nil
			},
		},
		hardeningStore: mockStore,
	}
	server.registerHandlers()

	handler := server.handlers["account.delete"]

	req := &Request{
		Params: json.RawMessage(`{"password": "pass123"}`),
	}

	_, errObj := handler(context.Background(), req)
	if errObj == nil {
		t.Fatal("expected error when not authenticated")
	}
	if errObj.Message != "Not authenticated" {
		t.Errorf("expected 'Not authenticated', got %s", errObj.Message)
	}
}

// mockDeactivatableAdapter implements both MatrixAdapter and the DeactivateAccount interface.
type mockDeactivatableAdapter struct {
	userID      string
	loggedIn    bool
	deactivate  func(ctx context.Context, password string, erase bool) error
}

func (m *mockDeactivatableAdapter) GetUserID() string                                          { return m.userID }
func (m *mockDeactivatableAdapter) IsLoggedIn() bool                                           { return m.loggedIn }
func (m *mockDeactivatableAdapter) GetHomeserver() string                                      { return "https://test.server" }
func (m *mockDeactivatableAdapter) GetStatus() string                                         { return "connected" }
func (m *mockDeactivatableAdapter) SendMessage(_, _, _ string) (string, error)                 { return "", nil }
func (m *mockDeactivatableAdapter) SendEvent(_, _ string, _ []byte) error                      { return nil }
func (m *mockDeactivatableAdapter) Login(_, _ string) error                                    { return nil }
func (m *mockDeactivatableAdapter) JoinRoom(_ context.Context, _ string, _ []string, _ string) (string, error) {
	return "", nil
}
func (m *mockDeactivatableAdapter) DeactivateAccount(ctx context.Context, password string, erase bool) error {
	return m.deactivate(ctx, password, erase)
}

var errDeactivationFailed = newDeactivationError("server rejected deactivation")

type deactivationError struct{ msg string }

func newDeactivationError(msg string) *deactivationError { return &deactivationError{msg: msg} }
func (e *deactivationError) Error() string               { return e.msg }
