package voice

import (
	"errors"
	"fmt"
	"os"
)

const (
	ErrVoiceNotConfiguredCode = -32007

	// ErrVoiceRateLimitCode is returned when the voice provider rate-limits or
	// rejects requests due to quota exhaustion.
	ErrVoiceRateLimitCode = -32008
)

var ErrVoiceNotConfigured = &VoiceError{
	Code:    ErrVoiceNotConfiguredCode,
	Message: "voice pipeline not configured: feature flag is off or API key is missing",
}

type VoiceError struct {
	Code    int
	Message string
	Cause   error
}

func (e *VoiceError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("voice error [%d]: %s: %v", e.Code, e.Message, e.Cause)
	}
	return fmt.Sprintf("voice error [%d]: %s", e.Code, e.Message)
}

func (e *VoiceError) Unwrap() error {
	return e.Cause
}

func NewVoiceError(code int, message string, cause error) *VoiceError {
	return &VoiceError{Code: code, Message: message, Cause: cause}
}

func IsVoiceNotConfigured(err error) bool {
	var ve *VoiceError
	if errors.As(err, &ve) {
		return ve.Code == ErrVoiceNotConfiguredCode
	}
	return false
}

// IsVoiceRateLimit returns true if the error is a rate-limit or quota exceeded error.
func IsVoiceRateLimit(err error) bool {
	var ve *VoiceError
	if errors.As(err, &ve) {
		return ve.Code == ErrVoiceRateLimitCode
	}
	return false
}

// VoicePrereqReason enumerates distinct reasons a voice prereq check can fail.
type VoicePrereqReason string

const (
	PrereqTurnSecretMissing VoicePrereqReason = "VOICE_PREREQ_TURN_SECRET_MISSING"
	PrereqOpenAIKeyMissing  VoicePrereqReason = "VOICE_PREREQ_OPENAI_KEY_MISSING"
	PrereqMatrixUnavailable VoicePrereqReason = "VOICE_PREREQ_MATRIX_UNAVAILABLE"
	PrereqMatrixUnwired     VoicePrereqReason = "VOICE_PREREQ_MATRIX_UNWIRED"
)

// VoicePrereqFailure describes a single failed prerequisite.
type VoicePrereqFailure struct {
	Reason  VoicePrereqReason `json:"reason"`
	Message string            `json:"message"`
}

// CheckVoicePrereqs validates that all voice pipeline prerequisites are met.
// It returns nil when everything is OK, or a slice of failures describing
// each missing prerequisite. matrixWired should be true when the Matrix
// adapter has been successfully wired (logged in / connected).
func CheckVoicePrereqs(matrixWired bool) []VoicePrereqFailure {
	var failures []VoicePrereqFailure

	if os.Getenv("TURN_SECRET") == "" {
		failures = append(failures, VoicePrereqFailure{
			Reason:  PrereqTurnSecretMissing,
			Message: "TURN_SECRET environment variable is not set",
		})
	}
	if os.Getenv("OPENAI_API_KEY") == "" {
		failures = append(failures, VoicePrereqFailure{
			Reason:  PrereqOpenAIKeyMissing,
			Message: "OPENAI_API_KEY environment variable is not set",
		})
	}
	if !matrixWired {
		failures = append(failures, VoicePrereqFailure{
			Reason:  PrereqMatrixUnwired,
			Message: "Matrix adapter is not wired (not logged in or not connected)",
		})
	}

	return failures
}
