package secretary

import (
	"context"
	"encoding/json"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type delegationTestStore struct {
	sync.RWMutex
	policies map[string]*ApprovalPolicy
}

func newDelegationTestStore() *delegationTestStore {
	return &delegationTestStore{
		policies: make(map[string]*ApprovalPolicy),
	}
}

func (s *delegationTestStore) CreatePolicy(_ context.Context, policy *ApprovalPolicy) error {
	s.Lock()
	defer s.Unlock()
	s.policies[policy.ID] = policy
	return nil
}

func (s *delegationTestStore) GetPolicy(_ context.Context, id string) (*ApprovalPolicy, error) {
	s.RLock()
	defer s.RUnlock()
	p, ok := s.policies[id]
	if !ok {
		return nil, nil
	}
	cp := *p
	return &cp, nil
}

func (s *delegationTestStore) ListPolicies(_ context.Context) ([]ApprovalPolicy, error) {
	s.RLock()
	defer s.RUnlock()
	var result []ApprovalPolicy
	for _, p := range s.policies {
		result = append(result, *p)
	}
	return result, nil
}

func (s *delegationTestStore) UpdatePolicy(_ context.Context, policy *ApprovalPolicy) error {
	s.Lock()
	defer s.Unlock()
	s.policies[policy.ID] = policy
	return nil
}

func (s *delegationTestStore) DeletePolicy(_ context.Context, id string) error {
	s.Lock()
	defer s.Unlock()
	delete(s.policies, id)
	return nil
}

func (s *delegationTestStore) CreateScheduledTask(_ context.Context, _ *ScheduledTask) error   { return nil }
func (s *delegationTestStore) GetScheduledTask(_ context.Context, _ string) (*ScheduledTask, error) {
	return nil, nil
}
func (s *delegationTestStore) ListScheduledTasks(_ context.Context) ([]ScheduledTask, error)    { return nil, nil }
func (s *delegationTestStore) UpdateScheduledTask(_ context.Context, _ *ScheduledTask) error    { return nil }
func (s *delegationTestStore) DeleteScheduledTask(_ context.Context, _ string) error            { return nil }
func (s *delegationTestStore) ListPendingScheduledTasks(_ context.Context) ([]ScheduledTask, error) {
	return nil, nil
}
func (s *delegationTestStore) ListDueTasks(_ context.Context) ([]ScheduledTask, error)          { return nil, nil }
func (s *delegationTestStore) MarkDispatched(_ context.Context, _ string, _ time.Time) error    { return nil }
func (s *delegationTestStore) GetTemplateByTrigger(_ context.Context, _ string) (*TaskTemplate, error) {
	return nil, nil
}
func (s *delegationTestStore) CreateTemplate(_ context.Context, _ *TaskTemplate) error          { return nil }
func (s *delegationTestStore) GetTemplate(_ context.Context, _ string) (*TaskTemplate, error)   { return nil, nil }
func (s *delegationTestStore) ListTemplates(_ context.Context, _ TemplateFilter) ([]TaskTemplate, error) {
	return nil, nil
}
func (s *delegationTestStore) UpdateTemplate(_ context.Context, _ *TaskTemplate) error          { return nil }
func (s *delegationTestStore) DeleteTemplate(_ context.Context, _ string) error                 { return nil }
func (s *delegationTestStore) CreateWorkflow(_ context.Context, _ *Workflow) error              { return nil }
func (s *delegationTestStore) GetWorkflow(_ context.Context, _ string) (*Workflow, error)       { return nil, nil }
func (s *delegationTestStore) ListWorkflows(_ context.Context, _ WorkflowFilter) ([]Workflow, error) {
	return nil, nil
}
func (s *delegationTestStore) UpdateWorkflow(_ context.Context, _ *Workflow) error              { return nil }
func (s *delegationTestStore) DeleteWorkflow(_ context.Context, _ string) error                 { return nil }
func (s *delegationTestStore) CreateNotificationChannel(_ context.Context, _ *NotificationChannel) error {
	return nil
}
func (s *delegationTestStore) GetNotificationChannel(_ context.Context, _ string) (*NotificationChannel, error) {
	return nil, nil
}
func (s *delegationTestStore) ListNotificationChannels(_ context.Context, _ string) ([]NotificationChannel, error) {
	return nil, nil
}
func (s *delegationTestStore) UpdateNotificationChannel(_ context.Context, _ *NotificationChannel) error {
	return nil
}
func (s *delegationTestStore) DeleteNotificationChannel(_ context.Context, _ string) error { return nil }
func (s *delegationTestStore) CreateContact(_ context.Context, _ *Contact) error           { return nil }
func (s *delegationTestStore) GetContact(_ context.Context, _ string) (*Contact, error)    { return nil, nil }
func (s *delegationTestStore) ListContacts(_ context.Context, _ ContactFilter) ([]Contact, error) {
	return nil, nil
}
func (s *delegationTestStore) UpdateContact(_ context.Context, _ *Contact) error  { return nil }
func (s *delegationTestStore) DeleteContact(_ context.Context, _ string) error    { return nil }
func (s *delegationTestStore) Close() error                                       { return nil }

func seedPolicy(t *testing.T, store *delegationTestStore, policyID string) {
	t.Helper()
	err := store.CreatePolicy(context.Background(), &ApprovalPolicy{
		ID:        policyID,
		Name:      "Test Policy",
		CreatedBy: "@alice:example.com",
		IsActive:  true,
	})
	require.NoError(t, err)
}

func TestDelegationService_SetAndGet(t *testing.T) {
	store := newDelegationTestStore()
	svc := NewDelegationService(store)
	require.NotNil(t, svc)

	seedPolicy(t, store, "pol_001")

	info, err := svc.SetDelegation(context.Background(), DelegationConfig{
		PolicyID:   "pol_001",
		DelegateTo: "@bob:example.com",
	})
	require.NoError(t, err)
	assert.Equal(t, "@bob:example.com", info.DelegateTo)
	assert.True(t, info.IsActive)

	got, err := svc.GetDelegation(context.Background(), "pol_001")
	require.NoError(t, err)
	assert.Equal(t, "@bob:example.com", got.DelegateTo)
	assert.True(t, got.IsActive)
}

func TestDelegationService_SetWithTimeout(t *testing.T) {
	store := newDelegationTestStore()
	svc := NewDelegationService(store)

	seedPolicy(t, store, "pol_002")

	info, err := svc.SetDelegation(context.Background(), DelegationConfig{
		PolicyID:       "pol_002",
		DelegateTo:     "@carol:example.com",
		TimeoutMinutes: 30,
		EscalateTo:     "@alice:example.com",
	})
	require.NoError(t, err)
	assert.Equal(t, 30, info.TimeoutMinutes)
	assert.Equal(t, "@alice:example.com", info.EscalateTo)

	got, err := svc.GetDelegation(context.Background(), "pol_002")
	require.NoError(t, err)
	assert.Equal(t, 30, got.TimeoutMinutes)
	assert.Equal(t, "@alice:example.com", got.EscalateTo)
}

func TestDelegationService_Revoke(t *testing.T) {
	store := newDelegationTestStore()
	svc := NewDelegationService(store)

	seedPolicy(t, store, "pol_003")

	_, err := svc.SetDelegation(context.Background(), DelegationConfig{
		PolicyID:   "pol_003",
		DelegateTo: "@bob:example.com",
	})
	require.NoError(t, err)

	err = svc.RevokeDelegation(context.Background(), "pol_003")
	require.NoError(t, err)

	got, err := svc.GetDelegation(context.Background(), "pol_003")
	require.NoError(t, err)
	assert.Equal(t, "", got.DelegateTo)
	assert.False(t, got.IsActive)
}

func TestDelegationService_Escalation(t *testing.T) {
	store := newDelegationTestStore()
	svc := NewDelegationService(store)

	seedPolicy(t, store, "pol_004")

	_, err := svc.SetDelegation(context.Background(), DelegationConfig{
		PolicyID:       "pol_004",
		DelegateTo:     "@bob:example.com",
		TimeoutMinutes: 5,
		EscalateTo:     "@alice:example.com",
	})
	require.NoError(t, err)

	escalateTo, err := svc.CheckEscalation("pol_004", time.Now().Add(-10*time.Minute))
	require.NoError(t, err)
	assert.Equal(t, "@alice:example.com", escalateTo)

	escalateTo, err = svc.CheckEscalation("pol_004", time.Now())
	require.NoError(t, err)
	assert.Equal(t, "", escalateTo)
}

func TestDelegationService_NilStore(t *testing.T) {
	svc := NewDelegationService(nil)
	assert.Nil(t, svc)
}

func TestDelegationService_MissingPolicy(t *testing.T) {
	store := newDelegationTestStore()
	svc := NewDelegationService(store)

	_, err := svc.SetDelegation(context.Background(), DelegationConfig{
		PolicyID:   "nonexistent",
		DelegateTo: "@bob:example.com",
	})
	assert.Error(t, err)
}

func TestDelegationRPC_SetDelegation(t *testing.T) {
	store := newDelegationTestStore()
	delegationSvc := NewDelegationService(store)

	seedPolicy(t, store, "pol_rpc_001")

	handler := NewRPCHandler(RPCHandlerConfig{
		Store:             store,
		DelegationService: delegationSvc,
	})

	params, _ := json.Marshal(map[string]interface{}{
		"policy_id":   "pol_rpc_001",
		"delegate_to": "@bob:example.com",
	})

	resp := handler.Handle(&RPCRequest{
		Method: "secretary.set_approval_delegation",
		Params: params,
	})

	require.NotNil(t, resp)
	assert.Nil(t, resp.Error)
	result, ok := resp.Result.(*DelegationInfo)
	require.True(t, ok)
	assert.Equal(t, "@bob:example.com", result.DelegateTo)
}

func TestDelegationRPC_GetDelegation(t *testing.T) {
	store := newDelegationTestStore()
	delegationSvc := NewDelegationService(store)

	seedPolicy(t, store, "pol_rpc_002")

	_, _ = delegationSvc.SetDelegation(context.Background(), DelegationConfig{
		PolicyID:   "pol_rpc_002",
		DelegateTo: "@carol:example.com",
	})

	handler := NewRPCHandler(RPCHandlerConfig{
		Store:             store,
		DelegationService: delegationSvc,
	})

	params, _ := json.Marshal(map[string]interface{}{
		"policy_id": "pol_rpc_002",
	})

	resp := handler.Handle(&RPCRequest{
		Method: "secretary.get_approval_delegation",
		Params: params,
	})

	require.NotNil(t, resp)
	assert.Nil(t, resp.Error)
	result, ok := resp.Result.(*DelegationInfo)
	require.True(t, ok)
	assert.Equal(t, "@carol:example.com", result.DelegateTo)
}

func TestDelegationRPC_RevokeDelegation(t *testing.T) {
	store := newDelegationTestStore()
	delegationSvc := NewDelegationService(store)

	seedPolicy(t, store, "pol_rpc_003")

	_, _ = delegationSvc.SetDelegation(context.Background(), DelegationConfig{
		PolicyID:   "pol_rpc_003",
		DelegateTo: "@bob:example.com",
	})

	handler := NewRPCHandler(RPCHandlerConfig{
		Store:             store,
		DelegationService: delegationSvc,
	})

	params, _ := json.Marshal(map[string]interface{}{
		"policy_id": "pol_rpc_003",
	})

	resp := handler.Handle(&RPCRequest{
		Method: "secretary.revoke_approval_delegation",
		Params: params,
	})

	require.NotNil(t, resp)
	assert.Nil(t, resp.Error)
}

func TestDelegationRPC_NoService(t *testing.T) {
	store := newDelegationTestStore()
	handler := NewRPCHandler(RPCHandlerConfig{
		Store:             store,
		DelegationService: nil,
	})

	params, _ := json.Marshal(map[string]interface{}{
		"policy_id": "pol_001",
	})

	resp := handler.Handle(&RPCRequest{
		Method: "secretary.set_approval_delegation",
		Params: params,
	})

	require.NotNil(t, resp)
	assert.NotNil(t, resp.Error)
	assert.Equal(t, ErrInternal, resp.Error.Code)
}
