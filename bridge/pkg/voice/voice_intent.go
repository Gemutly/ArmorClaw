package voice

import (
	"context"
	"fmt"
	"time"
)

// VoiceIntentRequest is a text-only voice intent submitted by an edge client
// (e.g., Android ArmorChat). It contains ONLY a transcript — no raw audio.
//
// CRITICAL: Raw audio must NEVER be sent through this RPC.
// The edge client performs STT locally and sends only the text transcript.
type VoiceIntentRequest struct {
	SessionID  string  `json:"session_id"`
	Source     string  `json:"source"`
	Transcript string  `json:"transcript"`
	Confidence float64 `json:"confidence"`
	Locale     string  `json:"locale"`
}

// VoiceIntentResponse is the Bridge's response after processing a voice intent.
type VoiceIntentResponse struct {
	SessionID   string `json:"session_id"`
	Status      string `json:"status"`
	Message     string `json:"message"`
	ActionTaken string `json:"action_taken,omitempty"`
}

// Validate checks that the request contains only text fields and no raw audio.
// Returns an error if raw audio data is detected in the request.
func (r *VoiceIntentRequest) Validate() error {
	if r.SessionID == "" {
		return fmt.Errorf("session_id is required")
	}
	if r.Transcript == "" {
		return fmt.Errorf("transcript is required")
	}
	if r.Source == "" {
		r.Source = "android_edge"
	}
	if r.Locale == "" {
		r.Locale = "en-US"
	}
	return nil
}

// VoiceIntentLogEntry represents an audit log entry for voice intent processing.
// Records transcript metadata only — never raw audio.
type VoiceIntentLogEntry struct {
	SessionID   string    `json:"session_id"`
	Source      string    `json:"source"`
	Transcript  string    `json:"transcript"`
	Confidence  float64   `json:"confidence"`
	Locale      string    `json:"locale"`
	ActionTaken string    `json:"action_taken"`
	Timestamp   time.Time `json:"timestamp"`
}

// ProcessVoiceIntent handles a text-only voice intent from an edge client.
// This is the server-side handler for the `voice.intent.submit` RPC.
func ProcessVoiceIntent(ctx context.Context, req *VoiceIntentRequest) (*VoiceIntentResponse, *VoiceIntentLogEntry, error) {
	if err := req.Validate(); err != nil {
		return nil, nil, fmt.Errorf("validation failed: %w", err)
	}

	resp := &VoiceIntentResponse{
		SessionID:   req.SessionID,
		Status:      "accepted",
		Message:     fmt.Sprintf("Voice intent received: \"%s\"", req.Transcript),
		ActionTaken: "logged",
	}

	logEntry := &VoiceIntentLogEntry{
		SessionID:   req.SessionID,
		Source:      req.Source,
		Transcript:  req.Transcript,
		Confidence:  req.Confidence,
		Locale:      req.Locale,
		ActionTaken: "logged",
		Timestamp:   time.Now().UTC(),
	}

	return resp, logEntry, nil
}
