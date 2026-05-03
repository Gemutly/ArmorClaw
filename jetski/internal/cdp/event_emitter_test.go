package cdp

import (
	"encoding/json"
	"testing"
	"time"
)

func TestEventEmitterSubscribe(t *testing.T) {
	em := NewEventEmitter(true)
	ch, err := em.Subscribe("device-1")
	if err != nil {
		t.Fatalf("Subscribe should succeed: %v", err)
	}
	if ch == nil {
		t.Fatal("channel should not be nil")
	}
	if em.SubscriberCount() != 1 {
		t.Errorf("expected 1 subscriber, got %d", em.SubscriberCount())
	}
}

func TestEventEmitterReceivesEvents(t *testing.T) {
	em := NewEventEmitter(true)
	ch, _ := em.Subscribe("device-1")

	params := json.RawMessage(`{"url":"https://example.com"}`)
	em.Emit("Page.frameNavigated", params)

	select {
	case evt := <-ch:
		if evt.Method != "Page.frameNavigated" {
			t.Errorf("expected Page.frameNavigated, got %s", evt.Method)
		}
	case <-time.After(1 * time.Second):
		t.Fatal("timed out waiting for event")
	}
}

func TestEventEmitterIrrelevantEventsDropped(t *testing.T) {
	em := NewEventEmitter(true)
	ch, _ := em.Subscribe("device-1")

	em.Emit("Network.requestWillBeSent", json.RawMessage(`{}`))

	select {
	case <-ch:
		t.Fatal("should not receive irrelevant events")
	case <-time.After(100 * time.Millisecond):
	}
}

func TestEventEmitterPIIRedactionURLs(t *testing.T) {
	em := NewEventEmitter(true)
	ch, _ := em.Subscribe("device-1")

	params := json.RawMessage(`{"url":"https://example.com/path?token=secret123&session=abc"}`)
	em.Emit("Page.frameNavigated", params)

	select {
	case evt := <-ch:
		urlVal, _ := evt.Params["url"].(string)
		if urlVal != "https://example.com/path" {
			t.Errorf("query params should be stripped, got: %s", urlVal)
		}
	case <-time.After(1 * time.Second):
		t.Fatal("timed out waiting for event")
	}
}

func TestEventEmitterPIIRedactionSSN(t *testing.T) {
	em := NewEventEmitter(true)
	ch, _ := em.Subscribe("device-1")

	params := json.RawMessage(`{"value":"SSN: 123-45-6789"}`)
	em.Emit("DOM.focus", params)

	select {
	case evt := <-ch:
		val, _ := evt.Params["value"].(string)
		if val != "SSN: [REDACTED_SSN]" {
			t.Errorf("SSN should be redacted, got: %s", val)
		}
	case <-time.After(1 * time.Second):
		t.Fatal("timed out waiting for event")
	}
}

func TestEventEmitterPIIRedactionEmail(t *testing.T) {
	em := NewEventEmitter(true)
	ch, _ := em.Subscribe("device-1")

	params := json.RawMessage(`{"value":"Contact user@example.com for help"}`)
	em.Emit("DOM.focus", params)

	select {
	case evt := <-ch:
		val, _ := evt.Params["value"].(string)
		if val != "Contact [REDACTED_EMAIL] for help" {
			t.Errorf("email should be redacted, got: %s", val)
		}
	case <-time.After(1 * time.Second):
		t.Fatal("timed out waiting for event")
	}
}

func TestEventEmitterPIIRedactionCreditCard(t *testing.T) {
	em := NewEventEmitter(true)
	ch, _ := em.Subscribe("device-1")

	params := json.RawMessage(`{"value":"Card: 4111-1111-1111-1111"}`)
	em.Emit("DOM.focus", params)

	select {
	case evt := <-ch:
		val, _ := evt.Params["value"].(string)
		if val != "Card: [REDACTED_CC]" {
			t.Errorf("credit card should be redacted, got: %s", val)
		}
	case <-time.After(1 * time.Second):
		t.Fatal("timed out waiting for event")
	}
}

func TestEventEmitterDOMTruncation(t *testing.T) {
	em := NewEventEmitter(true)
	ch, _ := em.Subscribe("device-1")

	longContent := make([]byte, 300)
	for i := range longContent {
		longContent[i] = 'a'
	}
	params := json.RawMessage(`{"value":"` + string(longContent) + `"}`)
	em.Emit("DOM.focus", params)

	select {
	case evt := <-ch:
		val, _ := evt.Params["value"].(string)
		if len(val) > 220 {
			t.Errorf("DOM content should be truncated to ~200 chars, got len=%d: %s", len(val), val)
		}
	case <-time.After(1 * time.Second):
		t.Fatal("timed out waiting for event")
	}
}

func TestEventEmitterRequiresRegistration(t *testing.T) {
	em := NewEventEmitter(true)

	if em.SubscriberCount() != 0 {
		t.Error("should start with 0 subscribers")
	}

	params := json.RawMessage(`{"url":"https://example.com"}`)
	em.Emit("Page.frameNavigated", params)

	if em.SubscriberCount() != 0 {
		t.Error("emit should not create subscribers")
	}
}

func TestEventEmitterConfigDisabled(t *testing.T) {
	em := NewEventEmitter(false)

	_, err := em.Subscribe("device-1")
	if err != ErrEmitterDisabled {
		t.Errorf("expected ErrEmitterDisabled, got: %v", err)
	}
	if em.Enabled() {
		t.Error("disabled emitter should report Enabled()=false")
	}
}

func TestEventEmitterDisabledNoSubscribe(t *testing.T) {
	em := NewEventEmitter(false)

	params := json.RawMessage(`{"url":"https://example.com"}`)
	em.Emit("Page.frameNavigated", params)
}

func TestEventEmitterAlreadySubscribed(t *testing.T) {
	em := NewEventEmitter(true)
	_, err := em.Subscribe("device-1")
	if err != nil {
		t.Fatalf("first subscribe should succeed: %v", err)
	}

	_, err = em.Subscribe("device-1")
	if err != ErrAlreadySubscribed {
		t.Errorf("expected ErrAlreadySubscribed, got: %v", err)
	}
}

func TestEventEmitterMissingDeviceID(t *testing.T) {
	em := NewEventEmitter(true)
	_, err := em.Subscribe("")
	if err != ErrMissingDeviceID {
		t.Errorf("expected ErrMissingDeviceID, got: %v", err)
	}
}

func TestEventEmitterUnsubscribe(t *testing.T) {
	em := NewEventEmitter(true)
	ch, _ := em.Subscribe("device-1")

	em.Unsubscribe("device-1")

	if em.SubscriberCount() != 0 {
		t.Error("subscriber count should be 0 after unsubscribe")
	}

	_, ok := <-ch
	if ok {
		t.Error("channel should be closed after unsubscribe")
	}
}

func TestEventEmitterFanOut(t *testing.T) {
	em := NewEventEmitter(true)
	ch1, _ := em.Subscribe("device-1")
	ch2, _ := em.Subscribe("device-2")

	params := json.RawMessage(`{"url":"https://example.com"}`)
	em.Emit("Page.frameNavigated", params)

	select {
	case <-ch1:
	case <-time.After(1 * time.Second):
		t.Fatal("device-1 should receive event")
	}

	select {
	case <-ch2:
	case <-time.After(1 * time.Second):
		t.Fatal("device-2 should receive event")
	}
}

func TestEventEmitterFrameURLRedaction(t *testing.T) {
	em := NewEventEmitter(true)
	ch, _ := em.Subscribe("device-1")

	params := json.RawMessage(`{"frame":{"url":"https://bank.com/account?session=xyz123"}}`)
	em.Emit("Page.frameNavigated", params)

	select {
	case evt := <-ch:
		frame, _ := evt.Params["frame"].(map[string]interface{})
		if frame == nil {
			t.Fatal("frame should be present")
		}
		urlVal, _ := frame["url"].(string)
		if urlVal != "https://bank.com/account" {
			t.Errorf("frame URL query params should be stripped, got: %s", urlVal)
		}
	case <-time.After(1 * time.Second):
		t.Fatal("timed out waiting for event")
	}
}

func TestIsRelevantEvent(t *testing.T) {
	tests := []struct {
		method   string
		expected bool
	}{
		{"Page.frameNavigated", true},
		{"DOM.focus", true},
		{"Runtime.executionContextCreated", true},
		{"Page.javascriptDialogOpening", true},
		{"Page.loadEventFired", true},
		{"Network.requestWillBeSent", false},
		{"Runtime.evaluate", false},
		{"Input.insertText", false},
	}

	for _, tt := range tests {
		t.Run(tt.method, func(t *testing.T) {
			if IsRelevantEvent(tt.method) != tt.expected {
				t.Errorf("IsRelevantEvent(%q) = %v, want %v", tt.method, !tt.expected, tt.expected)
			}
		})
	}
}

func TestRedactURLInvalidURL(t *testing.T) {
	result := redactURL("://not-a-url")
	if result != "[INVALID_URL]" {
		t.Errorf("invalid URL should be redacted, got: %s", result)
	}
}
