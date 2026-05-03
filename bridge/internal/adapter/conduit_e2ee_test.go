// Package adapter provides Matrix client functionality for ArmorClaw bridge.
package adapter

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
)

// conduitE2EETestConfig holds configuration for E2EE validation tests.
type conduitE2EETestConfig struct {
	HomeserverURL string
	AccessToken   string
	UserID        string
	DeviceID      string
}

// skipIfNoConduit skips the test if no Conduit server is running at the default URL.
// Tests that hit this skip are expected — they validate compilation, not runtime behavior.
func skipIfNoConduit(t *testing.T) conduitE2EETestConfig {
	t.Helper()

	hsURL := os.Getenv("CONDUIT_TEST_URL")
	if hsURL == "" {
		hsURL = "http://localhost:6167"
	}

	token := os.Getenv("CONDUIT_TEST_TOKEN")
	if token == "" {
		t.Skip("No CONDUIT_TEST_TOKEN set — skipping live Conduit E2EE validation")
	}

	cfg := conduitE2EETestConfig{
		HomeserverURL: hsURL,
		AccessToken:   token,
		UserID:        os.Getenv("CONDUIT_TEST_USER"),
		DeviceID:      os.Getenv("CONDUIT_TEST_DEVICE"),
	}

	if cfg.UserID == "" {
		cfg.UserID = "@e2ee-test:localhost"
	}
	if cfg.DeviceID == "" {
		cfg.DeviceID = "E2EE_TEST_DEVICE"
	}

	// Verify the server is reachable
	resp, err := http.Get(hsURL + "/_matrix/client/versions")
	if err != nil {
		t.Skipf("Conduit not reachable at %s — skipping live E2EE validation: %v", hsURL, err)
	}
	resp.Body.Close()

	return cfg
}

// matrixRequest is a helper for making authenticated Matrix Client-Server API requests.
func matrixRequest(t *testing.T, cfg conduitE2EETestConfig, method, path string, body interface{}) *http.Response {
	t.Helper()

	var reqBody io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("failed to marshal request body: %v", err)
		}
		reqBody = bytes.NewReader(data)
	}

	url := cfg.HomeserverURL + path
	req, err := http.NewRequest(method, url, reqBody)
	if err != nil {
		t.Fatalf("failed to create request: %v", err)
	}

	req.Header.Set("Authorization", "Bearer "+cfg.AccessToken)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request to %s failed: %v", url, err)
	}

	return resp
}

// parseMatrixError extracts the Matrix error code and message from a response body.
func parseMatrixError(t *testing.T, body []byte) (errcode, errmsg string) {
	t.Helper()
	var result struct {
		ErrCode string `json:"errcode"`
		Err     string `json:"error"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return "UNKNOWN", string(body)
	}
	return result.ErrCode, result.Err
}

// --- E2EE Capability Tests ---

// TestConduitE2EE is the top-level E2EE capability validation test.
// It probes the Conduit homeserver for E2EE-related endpoints and documents
// which are supported. This is a SPIKE test — failures are informational.
func TestConduitE2EE(t *testing.T) {
	cfg := skipIfNoConduit(t)

	t.Run("ToDevice", func(t *testing.T) {
		testToDeviceEndpoint(t, cfg)
	})
	t.Run("KeysUpload", func(t *testing.T) {
		testKeysUploadEndpoint(t, cfg)
	})
	t.Run("KeysQuery", func(t *testing.T) {
		testKeysQueryEndpoint(t, cfg)
	})
	t.Run("KeysClaim", func(t *testing.T) {
		testKeysClaimEndpoint(t, cfg)
	})
	t.Run("CrossSigning", func(t *testing.T) {
		testCrossSigningEndpoints(t, cfg)
	})
	t.Run("UIAAForCrossSigning", func(t *testing.T) {
		testUIAAForCrossSigning(t, cfg)
	})
}

// testToDeviceEndpoint validates that the Conduit /_matrix/client/v3/sendToDevice endpoint exists.
func testToDeviceEndpoint(t *testing.T, cfg conduitE2EETestConfig) {
	// Send a minimal ToDevice message (m.dummy) to validate the endpoint
	toDeviceBody := map[string]interface{}{
		"messages": map[string]interface{}{
			"*": map[string]interface{}{},
		},
	}

	resp := matrixRequest(t, cfg, http.MethodPut,
		"/_matrix/client/v3/sendToDevice/m.dummy/"+cfg.DeviceID, toDeviceBody)
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusCreated {
		t.Logf("ToDevice: SUPPORTED (status %d)", resp.StatusCode)
	} else {
		errcode, errmsg := parseMatrixError(t, body)
		// M_UNKNOWN is acceptable — it means the endpoint exists but the payload may need adjustment
		if resp.StatusCode == http.StatusBadRequest && errcode == "M_BAD_JSON" {
			t.Logf("ToDevice: SUPPORTED (endpoint exists, payload format issue: %s)", errmsg)
		} else {
			t.Logf("ToDevice: NOT SUPPORTED or error (status %d, %s: %s)", resp.StatusCode, errcode, errmsg)
		}
	}
}

// testKeysUploadEndpoint validates that /_matrix/client/v3/keys/upload exists.
func testKeysUploadEndpoint(t *testing.T, cfg conduitE2EETestConfig) {
	// Upload empty device keys (valid Matrix API call, even with no keys)
	uploadBody := map[string]interface{}{
		"device_keys": map[string]interface{}{
			"algorithm": "dummy",
			"keys":      map[string]interface{}{},
		},
	}

	resp := matrixRequest(t, cfg, http.MethodPost,
		"/_matrix/client/v3/keys/upload", uploadBody)
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode == http.StatusOK {
		t.Logf("KeysUpload: SUPPORTED (status 200)")
	} else {
		errcode, errmsg := parseMatrixError(t, body)
		t.Logf("KeysUpload: status %d, %s: %s", resp.StatusCode, errcode, errmsg)
		// M_UNRECOGNIZED algorithm means endpoint exists but our test payload is invalid
		if resp.StatusCode == http.StatusBadRequest && (errcode == "M_UNRECOGNIZED" || errcode == "M_BAD_JSON") {
			t.Log("KeysUpload: SUPPORTED (endpoint exists, algorithm not recognized — expected for dummy)")
		}
	}
}

// testKeysQueryEndpoint validates that /_matrix/client/v3/keys/query exists.
func testKeysQueryEndpoint(t *testing.T, cfg conduitE2EETestConfig) {
	queryBody := map[string]interface{}{
		"device_keys": map[string]interface{}{
			cfg.UserID: map[string]interface{}{},
		},
	}

	resp := matrixRequest(t, cfg, http.MethodPost,
		"/_matrix/client/v3/keys/query", queryBody)
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode == http.StatusOK {
		t.Logf("KeysQuery: SUPPORTED (status 200)")
	} else {
		errcode, errmsg := parseMatrixError(t, body)
		t.Logf("KeysQuery: status %d, %s: %s", resp.StatusCode, errcode, errmsg)
	}
}

// testKeysClaimEndpoint validates that /_matrix/client/v3/keys/claim exists.
func testKeysClaimEndpoint(t *testing.T, cfg conduitE2EETestConfig) {
	claimBody := map[string]interface{}{
		"one_time_keys": map[string]interface{}{
			cfg.UserID: map[string]interface{}{
				cfg.DeviceID: "dummy_key",
			},
		},
	}

	resp := matrixRequest(t, cfg, http.MethodPost,
		"/_matrix/client/v3/keys/claim", claimBody)
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode == http.StatusOK {
		t.Logf("KeysClaim: SUPPORTED (status 200)")
	} else {
		errcode, errmsg := parseMatrixError(t, body)
		t.Logf("KeysClaim: status %d, %s: %s", resp.StatusCode, errcode, errmsg)
	}
}

// testCrossSigningEndpoints validates cross-signing related endpoints.
func testCrossSigningEndpoints(t *testing.T, cfg conduitE2EETestConfig) {
	// Try to upload cross-signing keys (will fail without UIAA but proves endpoint exists)
	crossSignBody := map[string]interface{}{
		"master_key":    map[string]interface{}{},
		"self_signing_key": map[string]interface{}{},
	}

	resp := matrixRequest(t, cfg, http.MethodPost,
		"/_matrix/client/v3/keys/device_signing/upload", crossSignBody)
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode == http.StatusOK {
		t.Logf("CrossSigning: SUPPORTED (status 200)")
	} else if resp.StatusCode == http.StatusUnauthorized {
		t.Logf("CrossSigning: PARTIALLY SUPPORTED (endpoint exists, UIAA required)")
	} else {
		errcode, errmsg := parseMatrixError(t, body)
		t.Logf("CrossSigning: status %d, %s: %s", resp.StatusCode, errcode, errmsg)
	}
}

// testUIAAForCrossSigning validates that cross-signing endpoints properly require
// User-Interactive Authentication (UIAA).
func testUIAAForCrossSigning(t *testing.T, cfg conduitE2EETestConfig) {
	// Upload cross-signing keys without auth — should get 401 with UIAA flows
	crossSignBody := map[string]interface{}{
		"master_key": map[string]interface{}{
			"user_id":   cfg.UserID,
			"usage":     []string{"master"},
			"keys":      map[string]interface{}{},
		},
	}

	resp := matrixRequest(t, cfg, http.MethodPost,
		"/_matrix/client/v3/keys/device_signing/upload", crossSignBody)
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode == http.StatusUnauthorized {
		var uiaaResp struct {
			Flows []struct {
				Stages []string `json:"stages"`
			} `json:"flows"`
			Session string `json:"session"`
			ErrCode string `json:"errcode"`
		}
		if err := json.Unmarshal(body, &uiaaResp); err == nil && len(uiaaResp.Flows) > 0 {
			t.Logf("UIAA for CrossSigning: REQUIRED (session=%s, flows=%d)", uiaaResp.Session, len(uiaaResp.Flows))
			for i, flow := range uiaaResp.Flows {
				t.Logf("  Flow %d: stages=%v", i, flow.Stages)
			}
		} else {
			t.Logf("UIAA for CrossSigning: status 401 but no flows parsed")
		}
	} else if resp.StatusCode == http.StatusOK {
		t.Logf("UIAA for CrossSigning: NOT REQUIRED (cross-signing upload succeeded without auth)")
	} else {
		errcode, errmsg := parseMatrixError(t, body)
		t.Logf("UIAA for CrossSigning: status %d, %s: %s", resp.StatusCode, errcode, errmsg)
	}
}

// --- Compilation Tests (no server required) ---

// TestConduitE2EECompilation validates that the E2EE types and helpers compile correctly.
// This test does NOT require a running Conduit server.
func TestConduitE2EECompilation(t *testing.T) {
	// Verify the SyncResponse struct is correctly defined (it currently LACKS ToDevice)
	sr := SyncResponse{}
	sr.NextBatch = "batch_123"

	if sr.NextBatch != "batch_123" {
		t.Error("SyncResponse struct should hold NextBatch")
	}

	// Document: SyncResponse currently has NO ToDevice field.
	// E2EE requires adding: ToDevice *mautrix.SyncToDeviceSync `json:"to_device"`
	// See bridge/internal/adapter/matrix.go:127-136

	// Verify the sync filter excludes m.room.encrypted
	filterData, _ := json.Marshal(bridgeSyncFilter)
	filterStr := string(filterData)

	if contains := bytes.Contains([]byte(filterStr), []byte("m.room.encrypted")); contains {
		t.Log("WARNING: bridgeSyncFilter includes m.room.encrypted — E2EE events WILL be received")
	} else {
		t.Log("CONFIRMED: bridgeSyncFilter excludes m.room.encrypted — E2EE events are filtered out")
	}

	// Validate the expected mautrix-go crypto imports we'll need
	// These are documented here for reference — they are NOT imported yet.
	requiredImports := []string{
		"maunium.net/go/mautrix",
		"maunium.net/go/mautrix/crypto/cryptohelper",
		"maunium.net/go/mautrix/id",
	}
	t.Logf("Required mautrix-go crypto imports (NOT yet in go.mod):")
	for _, imp := range requiredImports {
		t.Logf("  - %s", imp)
	}
}

// --- Mock Server Tests ---

// TestConduitE2EEMockServer validates E2EE endpoint handling using a mock server.
// This test always runs (no Conduit required) and validates our test infrastructure.
func TestConduitE2EEMockServer(t *testing.T) {
	// Create a mock Conduit server
	mock := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/_matrix/client/versions":
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]interface{}{
				"versions": []string{"v1.11", "v1.12"},
				"unstable_features": map[string]interface{}{
					"org.matrix.e2e_cross_signing": true,
				},
			})

		case r.URL.Path == "/_matrix/client/v3/keys/upload" && r.Method == http.MethodPost:
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]interface{}{
				"one_time_key_counts": map[string]interface{}{
					"curve25519": 0,
					"signed_curve25519": 0,
				},
			})

		case r.URL.Path == "/_matrix/client/v3/keys/query" && r.Method == http.MethodPost:
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]interface{}{
				"device_keys": map[string]interface{}{},
				"failures":    map[string]interface{}{},
			})

		case r.URL.Path == "/_matrix/client/v3/keys/claim" && r.Method == http.MethodPost:
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]interface{}{
				"one_time_keys": map[string]interface{}{},
				"failures":      map[string]interface{}{},
			})

		case r.URL.Path == "/_matrix/client/v3/sendToDevice/" && r.Method == http.MethodPut:
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]interface{}{})

		case r.URL.Path == "/_matrix/client/v3/keys/device_signing/upload" && r.Method == http.MethodPost:
			// Cross-signing requires UIAA
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"errcode": "M_UNAUTHORIZED",
				"error":   "User-interactive authentication required",
				"flows": []map[string]interface{}{
					{"stages": []string{"m.login.password"}},
				},
				"session": "mock_session_123",
			})

		default:
			w.WriteHeader(http.StatusNotFound)
			fmt.Fprintf(w, `{"errcode":"M_UNRECOGNIZED","error":"unknown endpoint"}`)
		}
	}))
	defer mock.Close()

	cfg := conduitE2EETestConfig{
		HomeserverURL: mock.URL,
		AccessToken:   "mock_token",
		UserID:        "@test:localhost",
		DeviceID:      "TEST_DEVICE",
	}

	// Test keys upload
	t.Run("MockKeysUpload", func(t *testing.T) {
		resp := matrixRequest(t, cfg, http.MethodPost, "/_matrix/client/v3/keys/upload", map[string]interface{}{})
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Errorf("expected 200, got %d", resp.StatusCode)
		}
	})

	// Test keys query
	t.Run("MockKeysQuery", func(t *testing.T) {
		resp := matrixRequest(t, cfg, http.MethodPost, "/_matrix/client/v3/keys/query", map[string]interface{}{})
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Errorf("expected 200, got %d", resp.StatusCode)
		}
	})

	// Test keys claim
	t.Run("MockKeysClaim", func(t *testing.T) {
		resp := matrixRequest(t, cfg, http.MethodPost, "/_matrix/client/v3/keys/claim", map[string]interface{}{})
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Errorf("expected 200, got %d", resp.StatusCode)
		}
	})

	// Test cross-signing UIAA
	t.Run("MockCrossSigningUIAA", func(t *testing.T) {
		resp := matrixRequest(t, cfg, http.MethodPost, "/_matrix/client/v3/keys/device_signing/upload", map[string]interface{}{})
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusUnauthorized {
			t.Errorf("expected 401 for cross-signing without UIAA, got %d", resp.StatusCode)
		}
		body, _ := io.ReadAll(resp.Body)
		var uiaa struct {
			Flows []struct {
				Stages []string `json:"stages"`
			} `json:"flows"`
			Session string `json:"session"`
		}
		if err := json.Unmarshal(body, &uiaa); err != nil {
			t.Fatalf("failed to parse UIAA response: %v", err)
		}
		if len(uiaa.Flows) == 0 {
			t.Error("expected UIAA flows in response")
		}
		if uiaa.Session != "mock_session_123" {
			t.Errorf("expected session=mock_session_123, got %s", uiaa.Session)
		}
	})
}

// TestConduitE2EESyncFilterIncludesEncrypted verifies the sync filter includes m.room.encrypted.
func TestConduitE2EESyncFilterIncludesEncrypted(t *testing.T) {
	requiredE2EETypes := []string{
		"m.room.encrypted",
	}

	filterData, _ := json.Marshal(bridgeSyncFilter)
	filterStr := string(filterData)

	for _, eventType := range requiredE2EETypes {
		if !bytes.Contains([]byte(filterStr), []byte(eventType)) {
			t.Errorf("event type %s NOT in sync filter — required for E2EE", eventType)
		}
	}

	t.Log("Sync filter includes m.room.encrypted for E2EE support")
}

// TestConduitE2EESyncResponseHasE2EEFields verifies SyncResponse includes E2EE fields.
func TestConduitE2EESyncResponseHasE2EEFields(t *testing.T) {
	// SyncResponse now includes E2EE-required fields:
	//   - ToDevice *ToDevice `json:"to_device,omitempty"`
	//   - DeviceLists *DeviceLists `json:"device_lists,omitempty"`
	//   - DeviceOneTimeKeysCount map[string]int `json:"device_one_time_keys_count,omitempty"`

	sr := SyncResponse{
		NextBatch: "batch_123",
		ToDevice: &ToDevice{
			Events: []json.RawMessage{json.RawMessage(`{"type":"m.dummy"}`)},
		},
		DeviceLists: &DeviceLists{
			Changed: []string{"@user:example.com"},
			Left:    []string{"@old:example.com"},
		},
		DeviceOneTimeKeysCount: map[string]int{
			"curve25519":     50,
			"signed_curve25519": 10,
		},
	}

	// Verify ToDevice field
	if sr.ToDevice == nil {
		t.Fatal("Expected ToDevice to be non-nil")
	}
	if len(sr.ToDevice.Events) != 1 {
		t.Fatalf("Expected 1 to_device event, got %d", len(sr.ToDevice.Events))
	}

	// Verify DeviceLists field
	if sr.DeviceLists == nil {
		t.Fatal("Expected DeviceLists to be non-nil")
	}
	if len(sr.DeviceLists.Changed) != 1 {
		t.Fatalf("Expected 1 changed device, got %d", len(sr.DeviceLists.Changed))
	}

	// Verify DeviceOneTimeKeysCount field
	if sr.DeviceOneTimeKeysCount == nil {
		t.Fatal("Expected DeviceOneTimeKeysCount to be non-nil")
	}
	if sr.DeviceOneTimeKeysCount["curve25519"] != 50 {
		t.Fatalf("Expected curve25519 count 50, got %d", sr.DeviceOneTimeKeysCount["curve25519"])
	}

	t.Log("SyncResponse includes all E2EE fields: ToDevice, DeviceLists, DeviceOneTimeKeysCount")
}
