package rpc

import "testing"

func TestDocumentMethodRegistration(t *testing.T) {
	server := &Server{}
	server.registerHandlers()

	documentMethods := []string{
		"document.extract_text",
		"document.status",
		"document.list_jobs",
	}

	for _, method := range documentMethods {
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
