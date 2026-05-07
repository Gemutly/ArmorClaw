package voice

import (
	"errors"
	"fmt"
)

const (
	ErrVoiceNotConfiguredCode = -32007
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
