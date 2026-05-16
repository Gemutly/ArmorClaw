package rpc

import "testing"

func TestEmailMethodRegistration(t *testing.T) {
	server := &Server{}
	server.registerHandlers()

	emailMethods := []string{
		"email.queue_status",
		"email.get",
		"email.retry",
		"email.list",
	}

	for _, method := range emailMethods {
		handler, ok := server.handlers[method]
		if !ok {
			t.Errorf("method %q not registered in handler map", method)
			continue
		}
		if handler == nil {
			t.Errorf("method %q has nil handler", method)
		}
	}
}
