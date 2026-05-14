package email

import (
	"context"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/armorclaw/bridge/pkg/logger"
	"github.com/armorclaw/bridge/pkg/secretary"
)

func TestIntegration_TeamRouting_WithEmailClerk(t *testing.T) {
	store := newDispatcherTestStore()
	log, _ := logger.New(logger.Config{Output: "stdout"})

	matcher := func(ctx context.Context, address string) (string, bool, error) {
		if address == "sales@company.com" {
			return "team-sales", true, nil
		}
		return "", false, nil
	}

	lookup := func(ctx context.Context, teamID, role string) ([]string, error) {
		if teamID == "team-sales" && role == "email_clerk" {
			return []string{"clerk-001"}, nil
		}
		return nil, nil
	}

	d := NewEmailDispatcher(EmailDispatcherConfig{
		Store:           store,
		Log:             log,
		TeamMatcher:     matcher,
		TeamAgentLookup: lookup,
	})

	routed := false
	d.RegisterHandler(func(evt *EmailReceivedEvent) {
		routed = true
	})

	evt := NewEmailReceivedEvent("client@external.com", "sales@company.com", "Quote Request", "body", "e-001", nil, nil)
	d.OnEmailReceived(evt)

	assert.True(t, routed, "email to sales@company.com should be team-routed")
}

func TestIntegration_TeamRouting_NoMatch_FallsBackToTemplate(t *testing.T) {
	store := newDispatcherTestStore()
	store.addTemplate("info@company.com")
	log, _ := logger.New(logger.Config{Output: "stdout"})

	matcher := func(ctx context.Context, address string) (string, bool, error) {
		return "", false, nil
	}

	lookup := func(ctx context.Context, teamID, role string) ([]string, error) {
		return nil, nil
	}

	d := NewEmailDispatcher(EmailDispatcherConfig{
		Store:           store,
		Log:             log,
		TeamMatcher:     matcher,
		TeamAgentLookup: lookup,
	})

	templateRouted := false
	d.RegisterHandler(func(evt *EmailReceivedEvent) {
		templateRouted = true
	})

	evt := NewEmailReceivedEvent("client@external.com", "info@company.com", "General", "body", "e-002", nil, nil)
	d.OnEmailReceived(evt)

	assert.True(t, templateRouted, "unmatched email should fall back to template routing")
}

func TestIntegration_ThreadTracker_MultiMessageThread(t *testing.T) {
	tracker := NewThreadTracker()

	tracker.AddMessage(ThreadMessage{
		MessageID: "msg-001", Subject: "Thread Start",
		From: "a@company.com", To: []string{"b@company.com"}, EmailID: "e-001",
	})

	tracker.AddMessage(ThreadMessage{
		MessageID: "msg-002", InReplyTo: "msg-001", Subject: "Re: Thread Start",
		From: "b@company.com", To: []string{"a@company.com"}, EmailID: "e-002",
	})

	tracker.AddMessage(ThreadMessage{
		MessageID: "msg-003", InReplyTo: "msg-002", Subject: "Re: Thread Start",
		From: "a@company.com", To: []string{"b@company.com"}, EmailID: "e-003",
	})

	thread, err := tracker.GetThreadContext("msg-001")
	require.NoError(t, err)
	require.Len(t, thread, 3, "all 3 messages should be in the thread")
	assert.Equal(t, "msg-001", thread[0].MessageID)
	assert.Equal(t, "msg-002", thread[1].MessageID)
	assert.Equal(t, "msg-003", thread[2].MessageID)
}

func TestIntegration_DraftManager_CreateSend(t *testing.T) {
	log, _ := logger.New(logger.Config{Output: "stdout"})

	var sentTo string
	sender := &mockTeamSender{sendFn: func(ctx context.Context, to, subject, bodyText, bodyHTML string, attachments ...*EmailAttachment) (string, error) {
		sentTo = to
		return "sent-001", nil
	}}

	dm := NewDraftManager(DraftManagerConfig{Sender: sender, Log: log})

	draftID, err := dm.SaveDraft(context.Background(), EmailDraft{
		TeamID:   "team-1",
		To:       "recipient@test.com",
		CC:       "",
		Subject:  "Subject",
		BodyText: "Body text",
		BodyHTML: "",
	})
	require.NoError(t, err)
	assert.NotEmpty(t, draftID)

	drafts, err := dm.ListDrafts(context.Background(), "team-1")
	require.NoError(t, err)
	draft := drafts[0]
	assert.Equal(t, "team-1", draft.TeamID)
	assert.Equal(t, "recipient@test.com", draft.To)

	msgID, err := dm.SendDraft(context.Background(), draftID)
	require.NoError(t, err)
	assert.Equal(t, "sent-001", msgID)
	assert.Equal(t, "recipient@test.com", sentTo)
}

func TestIntegration_Dispatcher_ThreadTracker_Composition(t *testing.T) {
	tracker := NewThreadTracker()

	tracker.AddMessage(ThreadMessage{
		MessageID: "orig-001", Subject: "Inquiry",
		From: "customer@external.com", To: []string{"support@company.com"}, EmailID: "e-orig",
	})

	store := newDispatcherTestStore()
	log, _ := logger.New(logger.Config{Output: "stdout"})

	matcher := func(ctx context.Context, address string) (string, bool, error) {
		if address == "support@company.com" {
			return "team-support", true, nil
		}
		return "", false, nil
	}

	lookup := func(ctx context.Context, teamID, role string) ([]string, error) {
		return []string{"clerk-support"}, nil
	}

	d := NewEmailDispatcher(EmailDispatcherConfig{
		Store:           store,
		Log:             log,
		TeamMatcher:     matcher,
		TeamAgentLookup: lookup,
	})

	var dispatchedEmailID string
	d.RegisterHandler(func(evt *EmailReceivedEvent) {
		dispatchedEmailID = evt.EmailID
	})

	evt := NewEmailReceivedEvent("customer@external.com", "support@company.com", "Re: Inquiry", "follow-up", "e-followup", nil, nil)
	d.OnEmailReceived(evt)

	assert.Equal(t, "e-followup", dispatchedEmailID)

	thread, err := tracker.GetThreadContext("orig-001")
	assert.NoError(t, err)
	assert.Len(t, thread, 1, "original thread should still have 1 message (new email is a separate event)")
}

func TestIntegration_MultipleTeams_IsolatedRouting(t *testing.T) {
	store := newDispatcherTestStore()
	log, _ := logger.New(logger.Config{Output: "stdout"})

	matcher := func(ctx context.Context, address string) (string, bool, error) {
		switch address {
		case "sales@company.com":
			return "team-sales", true, nil
		case "support@company.com":
			return "team-support", true, nil
		default:
			return "", false, nil
		}
	}

	var mu sync.Mutex
	teamsSeen := map[string]string{}
	lookup := func(ctx context.Context, teamID, role string) ([]string, error) {
		mu.Lock()
		defer mu.Unlock()
		teamsSeen[teamID] = role
		return []string{"clerk-" + teamID}, nil
	}

	d := NewEmailDispatcher(EmailDispatcherConfig{
		Store:           store,
		Log:             log,
		TeamMatcher:     matcher,
		TeamAgentLookup: lookup,
	})

	salesRouted := false
	supportRouted := false
	d.RegisterHandler(func(evt *EmailReceivedEvent) {
		if evt.To == "sales@company.com" {
			salesRouted = true
		}
		if evt.To == "support@company.com" {
			supportRouted = true
		}
	})

	d.OnEmailReceived(NewEmailReceivedEvent("c1@ext.com", "sales@company.com", "Quote", "body", "e-sales", nil, nil))
	d.OnEmailReceived(NewEmailReceivedEvent("c2@ext.com", "support@company.com", "Help", "body", "e-support", nil, nil))
	d.OnEmailReceived(NewEmailReceivedEvent("c3@ext.com", "unknown@company.com", "Other", "body", "e-unknown", nil, nil))

	assert.True(t, salesRouted, "sales email should be routed to team-sales")
	assert.True(t, supportRouted, "support email should be routed to team-support")

	mu.Lock()
	assert.Equal(t, "email_clerk", teamsSeen["team-sales"])
	assert.Equal(t, "email_clerk", teamsSeen["team-support"])
	_, hasUnknown := teamsSeen["team-unknown"]
	assert.False(t, hasUnknown, "unknown address should not trigger team lookup")
	mu.Unlock()
}

type mockTeamSender struct {
	sendFn func(ctx context.Context, to, subject, bodyText, bodyHTML string, attachments ...*EmailAttachment) (string, error)
}

func (m *mockTeamSender) Send(ctx context.Context, to, subject, bodyText, bodyHTML string, attachments ...*EmailAttachment) (string, error) {
	return m.sendFn(ctx, to, subject, bodyText, bodyHTML, attachments...)
}

func (m *mockTeamSender) Provider() string { return "mock" }

var _ EmailSender = (*mockSender)(nil)
var _ secretary.Store = (*dispatcherTestStore)(nil)
