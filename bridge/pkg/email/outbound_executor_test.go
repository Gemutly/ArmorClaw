package email

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/armorclaw/bridge/pkg/logger"
)

type mockOutboundSender struct {
	sendErr   error
	messageID string
	lastTo    string
	lastSubj  string
	lastBody  string
}

func (m *mockOutboundSender) Send(ctx context.Context, to, subject, bodyText, bodyHTML string, attachments ...*EmailAttachment) (string, error) {
	m.lastTo = to
	m.lastSubj = subject
	m.lastBody = bodyText
	return m.messageID, m.sendErr
}

func (m *mockOutboundSender) Provider() string { return "mock" }

func newTestExecutor(senders map[string]EmailSender) *OutboundExecutor {
	log, _ := logger.New(logger.Config{Output: "stdout"})
	return NewOutboundExecutor(OutboundExecutorConfig{
		Senders: senders,
		Log:     log,
	})
}

func TestExecute_InvalidRecipient(t *testing.T) {
	e := newTestExecutor(nil)
	req := &OutboundRequest{To: "not-an-email", From: "from@test.com", Subject: "Test", BodyText: "body"}
	result, err := e.Execute(context.Background(), req)
	if err == nil {
		t.Fatal("expected error for invalid recipient")
	}
	if result.Status != ApprovalRejected {
		t.Errorf("Status = %q, want rejected", result.Status)
	}
}

func TestExecute_InvalidSender(t *testing.T) {
	e := newTestExecutor(nil)
	req := &OutboundRequest{To: "to@test.com", From: "not-an-email", Subject: "Test", BodyText: "body"}
	result, err := e.Execute(context.Background(), req)
	if err == nil {
		t.Fatal("expected error for invalid sender")
	}
	if result.Status != ApprovalRejected {
		t.Errorf("Status = %q, want rejected", result.Status)
	}
}

func TestExecute_Success(t *testing.T) {
	sender := &mockOutboundSender{messageID: "msg-123"}
	e := newTestExecutor(map[string]EmailSender{"gmail": sender})

	req := &OutboundRequest{
		To:       "to@test.com",
		From:     "from@test.com",
		Subject:  "Test",
		BodyText: "Hello",
		Provider: "gmail",
	}

	result, err := e.Execute(context.Background(), req)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if !result.Success {
		t.Error("expected success")
	}
	if result.MessageID != "msg-123" {
		t.Errorf("MessageID = %q, want msg-123", result.MessageID)
	}
	if result.ProviderUsed != "gmail" {
		t.Errorf("ProviderUsed = %q, want gmail", result.ProviderUsed)
	}
	if result.Status != ApprovalApproved {
		t.Errorf("Status = %q, want approved", result.Status)
	}
	if sender.lastTo != "to@test.com" {
		t.Errorf("sender got To = %q", sender.lastTo)
	}
}

func TestExecute_DefaultProvider(t *testing.T) {
	sender := &mockOutboundSender{messageID: "msg-456"}
	e := newTestExecutor(map[string]EmailSender{"gmail": sender})

	req := &OutboundRequest{
		To:       "to@test.com",
		From:     "from@test.com",
		Subject:  "Test",
		BodyText: "Body",
	}

	result, _ := e.Execute(context.Background(), req)
	if result.ProviderUsed != "gmail" {
		t.Errorf("ProviderUsed = %q, want gmail (default)", result.ProviderUsed)
	}
}

func TestExecute_FallbackToSMTP(t *testing.T) {
	smtpSender := &mockOutboundSender{messageID: "msg-smtp"}
	e := newTestExecutor(map[string]EmailSender{"smtp": smtpSender})

	req := &OutboundRequest{
		To:       "to@test.com",
		From:     "from@test.com",
		Subject:  "Test",
		BodyText: "Body",
		Provider: "nonexistent",
	}

	result, err := e.Execute(context.Background(), req)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if result.ProviderUsed != "smtp" {
		t.Errorf("ProviderUsed = %q, want smtp fallback", result.ProviderUsed)
	}
}

func TestExecute_NoSenderAvailable(t *testing.T) {
	e := newTestExecutor(map[string]EmailSender{})

	req := &OutboundRequest{
		To:       "to@test.com",
		From:     "from@test.com",
		Subject:  "Test",
		BodyText: "Body",
		Provider: "nonexistent",
	}

	result, err := e.Execute(context.Background(), req)
	if err == nil {
		t.Fatal("expected error when no sender available")
	}
	if result.Status != ApprovalRejected {
		t.Errorf("Status = %q, want rejected", result.Status)
	}
	if !strings.Contains(result.Error, "no sender") {
		t.Errorf("Error = %q", result.Error)
	}
}

func TestExecute_SenderError(t *testing.T) {
	sender := &mockOutboundSender{sendErr: errors.New("network failure")}
	e := newTestExecutor(map[string]EmailSender{"gmail": sender})

	req := &OutboundRequest{
		To:       "to@test.com",
		From:     "from@test.com",
		Subject:  "Test",
		BodyText: "Body",
	}

	result, err := e.Execute(context.Background(), req)
	if err == nil {
		t.Fatal("expected sender error")
	}
	if result.Status != ApprovalRejected {
		t.Errorf("Status = %q, want rejected", result.Status)
	}
	if !strings.Contains(result.Error, "network failure") {
		t.Errorf("Error = %q", result.Error)
	}
}

func TestExecute_PIIResolution(t *testing.T) {
	sender := &mockOutboundSender{messageID: "msg-pii"}
	e := newTestExecutor(map[string]EmailSender{"gmail": sender})

	req := &OutboundRequest{
		To:             "to@test.com",
		From:           "from@test.com",
		Subject:        "Subject [PII_0]",
		BodyText:       "Body with [PII_0] and [PII_1]",
		Provider:       "gmail",
		PIIResolutions: map[string]string{"[PII_0]": "123-45-6789", "[PII_1]": "4111111111111111"},
	}

	result, _ := e.Execute(context.Background(), req)
	if !result.Success {
		t.Error("expected success")
	}
	if sender.lastBody != "Body with 123-45-6789 and 4111111111111111" {
		t.Errorf("resolved body = %q", sender.lastBody)
	}
	if sender.lastSubj != "Subject 123-45-6789" {
		t.Errorf("resolved subject = %q", sender.lastSubj)
	}
}

func TestAvailableProviders(t *testing.T) {
	s1 := &mockOutboundSender{}
	s2 := &mockOutboundSender{}
	e := newTestExecutor(map[string]EmailSender{"gmail": s1, "smtp": s2})

	providers := e.AvailableProviders()
	if len(providers) != 2 {
		t.Fatalf("expected 2 providers, got %d", len(providers))
	}

	set := make(map[string]bool)
	for _, p := range providers {
		set[p] = true
	}
	if !set["gmail"] || !set["smtp"] {
		t.Errorf("providers = %v", providers)
	}
}

func TestAvailableProviders_Empty(t *testing.T) {
	e := newTestExecutor(nil)
	providers := e.AvailableProviders()
	if len(providers) != 0 {
		t.Errorf("expected empty providers, got %v", providers)
	}
}
