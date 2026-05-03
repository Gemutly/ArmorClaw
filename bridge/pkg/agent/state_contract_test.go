package agent

import "testing"

// TestAgentStatusStringConstantsUnchanged verifies that the AgentStatus string
// constants remain exactly as defined. These values are the Matrix protocol
// contract with ArmorChat Android — changing any string breaks the client.
func TestAgentStatusStringConstantsUnchanged(t *testing.T) {
	expected := map[AgentStatus]string{
		StatusIdle:             "IDLE",
		StatusInitializing:    "INITIALIZING",
		StatusBrowsing:        "BROWSING",
		StatusFormFilling:     "FORM_FILLING",
		StatusAwaitingCaptcha: "AWAITING_CAPTCHA",
		StatusAwaiting2FA:     "AWAITING_2FA",
		StatusAwaitingApproval: "AWAITING_APPROVAL",
		StatusProcessingPayment: "PROCESSING_PAYMENT",
		StatusError:           "ERROR",
		StatusComplete:        "COMPLETE",
		StatusOffline:         "OFFLINE",
	}

	for status, want := range expected {
		got := string(status)
		if got != want {
			t.Errorf("AgentStatus constant changed: got %q, want %q", got, want)
		}
	}

	if len(expected) != 11 {
		t.Errorf("expected 11 AgentStatus values, got %d — enum was added or removed", len(expected))
	}
}
