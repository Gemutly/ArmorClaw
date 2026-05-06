package rpc

import (
	"context"
	"testing"
)

type mockMatrixAdapter struct {
	loggedIn   bool
	userID     string
	homeserver string
	loginError error
}

func (m *mockMatrixAdapter) SendMessage(roomID, message, msgType string) (string, error) {
	return "", nil
}

func (m *mockMatrixAdapter) SendEvent(roomID, eventType string, content []byte) error {
	return nil
}

func (m *mockMatrixAdapter) Login(username, password string) error {
	return m.loginError
}

func (m *mockMatrixAdapter) JoinRoom(ctx context.Context, roomIDOrAlias string, viaServers []string, reason string) (string, error) {
	return roomIDOrAlias, nil
}

func (m *mockMatrixAdapter) GetUserID() string {
	return m.userID
}

func (m *mockMatrixAdapter) IsLoggedIn() bool {
	return m.loggedIn
}

func (m *mockMatrixAdapter) GetHomeserver() string {
	return m.homeserver
}

func (m *mockMatrixAdapter) GetStatus() string {
	return "connected"
}

func TestHandleMatrixStatus(t *testing.T) {
	tests := []struct {
		name           string
		adapter        *mockMatrixAdapter
		wantEnabled    bool
		wantLoggedIn   bool
		wantHomeserver string
	}{
		{
			name: "logged in and connected",
			adapter: &mockMatrixAdapter{
				loggedIn:   true,
				userID:     "@admin:example.com",
				homeserver: "https://matrix.example.com",
			},
			wantEnabled:    true,
			wantLoggedIn:   true,
			wantHomeserver: "https://matrix.example.com",
		},
		{
			name: "connected but not logged in",
			adapter: &mockMatrixAdapter{
				loggedIn:   false,
				homeserver: "http://127.0.0.1:6167",
			},
			wantEnabled:    true,
			wantLoggedIn:   false,
			wantHomeserver: "http://127.0.0.1:6167",
		},
		{
			name: "not configured",
			adapter: &mockMatrixAdapter{
				loggedIn: false,
			},
			wantEnabled:    true,
			wantLoggedIn:   false,
			wantHomeserver: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := &Server{matrix: tt.adapter}
			server.registerHandlers()

			result, err := server.handleMatrixStatus(context.Background(), &Request{
				JSONRPC: "2.0",
				ID:      "test-1",
				Method:  "matrix.status",
				Params:  nil,
			})

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			status, ok := result.(MatrixHealthResult)
			if !ok {
				t.Fatalf("expected MatrixHealthResult, got %T", result)
			}

			if status.Enabled != tt.wantEnabled {
				t.Errorf("Enabled = %v, want %v", status.Enabled, tt.wantEnabled)
			}
			if status.LoggedIn != tt.wantLoggedIn {
				t.Errorf("LoggedIn = %v, want %v", status.LoggedIn, tt.wantLoggedIn)
			}
			if status.Homeserver != tt.wantHomeserver {
				t.Errorf("Homeserver = %v, want %v", status.Homeserver, tt.wantHomeserver)
			}
		})
	}
}

func TestHandleMatrixStatusResponse(t *testing.T) {
	server := &Server{
		matrix: &mockMatrixAdapter{
			loggedIn:   true,
			userID:     "@test:server",
			homeserver: "https://test.server",
		},
	}
	server.registerHandlers()

	result, err := server.handleMatrixStatus(context.Background(), &Request{
		JSONRPC: "2.0",
		ID:      "1",
		Method:  "matrix.status",
		Params:  nil,
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	status := result.(MatrixHealthResult)

	if status.UserID != "@test:server" {
		t.Errorf("UserID = %v, want @test:server", status.UserID)
	}
}
