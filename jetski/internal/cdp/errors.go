package cdp

import "errors"

var (
	ErrEmitterDisabled   = errors.New("event emitter is disabled: set emit_state_events=true in config")
	ErrMissingDeviceID   = errors.New("device_id is required for event subscription")
	ErrAlreadySubscribed = errors.New("device is already subscribed to events")
)
