package cdp

import (
	"encoding/json"
	"log"
	"net/url"
	"regexp"
	"strings"
	"sync"
)

// SubscriberEvent is a redacted CDP event delivered to external consumers.
type SubscriberEvent struct {
	Method string                 `json:"method"`
	Params map[string]interface{} `json:"params,omitempty"`
}

// subscriber holds a registered consumer's event channel and device ID.
type subscriber struct {
	deviceID string
	ch       chan SubscriberEvent
}

// EventEmitter fans out intercepted CDP events to registered subscribers
// with PII redaction applied before delivery.
type EventEmitter struct {
	mu          sync.RWMutex
	subscribers map[string]*subscriber // keyed by deviceID
	enabled     bool
}

// Relevant CDP event types for agent state inference.
var relevantEvents = map[string]bool{
	"Page.frameNavigated":            true,
	"DOM.focus":                      true,
	"Runtime.executionContextCreated": true,
	"Page.javascriptDialogOpening":    true,
	"Page.loadEventFired":             true,
}

// PII patterns for redaction in emitted events.
var (
	emailPattern    = regexp.MustCompile(`\b[A-Za-z0-9._%+-]+@[A-Za-z0-9.-]+\.[A-Z|a-z]{2,}\b`)
	ssnPattern      = regexp.MustCompile(`\b\d{3}-\d{2}-\d{4}\b`)
	creditCardPattern = regexp.MustCompile(`\b(?:\d{4}[-\s]?){3}\d{4}\b`)
)

const (
	subscriberChannelSize = 256
	maxDOMContentLen      = 200
)

// NewEventEmitter creates a new EventEmitter. If enabled is false, Subscribe
// returns an error and Emit is a no-op.
func NewEventEmitter(enabled bool) *EventEmitter {
	return &EventEmitter{
		subscribers: make(map[string]*subscriber),
		enabled:     enabled,
	}
}

// Enabled returns whether the emitter is active.
func (e *EventEmitter) Enabled() bool {
	return e.enabled
}

// Subscribe registers a device for event delivery and returns a read-only
// channel. Returns an error if the emitter is disabled or the device is
// already registered.
func (e *EventEmitter) Subscribe(deviceID string) (<-chan SubscriberEvent, error) {
	if !e.enabled {
		return nil, ErrEmitterDisabled
	}
	if deviceID == "" {
		return nil, ErrMissingDeviceID
	}

	e.mu.Lock()
	defer e.mu.Unlock()

	if _, exists := e.subscribers[deviceID]; exists {
		return nil, ErrAlreadySubscribed
	}

	ch := make(chan SubscriberEvent, subscriberChannelSize)
	e.subscribers[deviceID] = &subscriber{
		deviceID: deviceID,
		ch:       ch,
	}

	log.Printf("[JETSKI EMITTER]: subscriber registered device=%s", deviceID)
	return ch, nil
}

// Unsubscribe removes a device and closes its channel.
func (e *EventEmitter) Unsubscribe(deviceID string) {
	e.mu.Lock()
	defer e.mu.Unlock()

	if sub, exists := e.subscribers[deviceID]; exists {
		close(sub.ch)
		delete(e.subscribers, deviceID)
		log.Printf("[JETSKI EMITTER]: subscriber removed device=%s", deviceID)
	}
}

// SubscriberCount returns the number of active subscribers.
func (e *EventEmitter) SubscriberCount() int {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return len(e.subscribers)
}

// Emit fans out a CDP event to all registered subscribers after PII
// redaction. Events not in the relevant set are silently dropped.
func (e *EventEmitter) Emit(method string, params json.RawMessage) {
	if !e.enabled {
		return
	}

	if !relevantEvents[method] {
		return
	}

	redacted := redactEvent(method, params)
	if redacted == nil {
		return
	}

	e.mu.RLock()
	defer e.mu.RUnlock()

	for _, sub := range e.subscribers {
		select {
		case sub.ch <- *redacted:
		default:
			// Drop event if subscriber channel is full rather than blocking.
			log.Printf("[JETSKI EMITTER]: dropping event for device=%s (channel full)", sub.deviceID)
		}
	}
}

// redactEvent parses raw CDP params, applies PII redaction, and returns a
// SubscriberEvent ready for external delivery. Returns nil if params are
// invalid.
func redactEvent(method string, rawParams json.RawMessage) *SubscriberEvent {
	var params map[string]interface{}
	if rawParams != nil {
		if err := json.Unmarshal(rawParams, &params); err != nil {
			// If params can't be parsed as object, wrap as raw.
			params = map[string]interface{}{
				"raw": string(rawParams),
			}
		}
	}

	if params == nil {
		params = make(map[string]interface{})
	}

	redactParams(params)

	return &SubscriberEvent{
		Method: method,
		Params: params,
	}
}

// redactParams mutates the params map in-place to strip PII.
func redactParams(params map[string]interface{}) {
	// Redact URLs: strip query params and fragments (may contain tokens).
	if urlVal, ok := params["url"].(string); ok {
		params["url"] = redactURL(urlVal)
	}
	if frameVal, ok := params["frame"].(map[string]interface{}); ok {
		if urlVal, ok := frameVal["url"].(string); ok {
			frameVal["url"] = redactURL(urlVal)
		}
	}

	// Redact DOM content: truncate to maxDOMContentLen and mask PII.
	for _, key := range []string{"value", "nodeValue", "textContent", "innerHTML", "outerHTML"} {
		if val, ok := params[key].(string); ok {
			if len(val) > maxDOMContentLen {
				params[key] = val[:maxDOMContentLen] + "...[TRUNCATED]"
			}
			params[key] = maskPIIStrings(params[key].(string))
		}
	}

	// Walk all string values and mask PII patterns.
	for k, v := range params {
		if str, ok := v.(string); ok {
			params[k] = maskPIIStrings(str)
		}
	}
}

// redactURL strips query params and fragments from a URL.
func redactURL(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil {
		return "[INVALID_URL]"
	}
	u.RawQuery = ""
	u.Fragment = ""
	return u.String()
}

// maskPIIStrings replaces SSN, credit card, and email patterns with
// redaction markers.
func maskPIIStrings(s string) string {
	s = ssnPattern.ReplaceAllString(s, "[REDACTED_SSN]")
	s = creditCardPattern.ReplaceAllString(s, "[REDACTED_CC]")
	s = emailPattern.ReplaceAllString(s, "[REDACTED_EMAIL]")
	return s
}

// IsRelevantEvent checks if a CDP method is in the relevant events set.
func IsRelevantEvent(method string) bool {
	return relevantEvents[method]
}

// HasPrefix checks if method starts with any of the given CDP domain prefixes.
func HasPrefix(method string, prefixes ...string) bool {
	for _, prefix := range prefixes {
		if strings.HasPrefix(method, prefix+".") {
			return true
		}
	}
	return false
}
