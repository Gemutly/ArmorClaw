package secretary

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/armorclaw/bridge/pkg/logger"
)

type DelegationConfig struct {
	DelegateTo     string `json:"delegate_to"`
	PolicyID       string `json:"policy_id"`
	TimeoutMinutes int    `json:"timeout_minutes,omitempty"`
	EscalateTo     string `json:"escalate_to,omitempty"`
}

type DelegationInfo struct {
	PolicyID       string `json:"policy_id"`
	DelegateTo     string `json:"delegate_to"`
	TimeoutMinutes int    `json:"timeout_minutes,omitempty"`
	EscalateTo     string `json:"escalate_to,omitempty"`
	IsActive       bool   `json:"is_active"`
}

type DelegationService struct {
	store Store
	log   *logger.Logger
}

func NewDelegationService(store Store) *DelegationService {
	if store == nil {
		return nil
	}
	return &DelegationService{
		store: store,
		log:   logger.Global().WithComponent("delegation"),
	}
}

func (s *DelegationService) SetDelegation(ctx context.Context, cfg DelegationConfig) (*DelegationInfo, error) {
	if cfg.PolicyID == "" {
		return nil, fmt.Errorf("policy_id is required")
	}
	if cfg.DelegateTo == "" {
		return nil, fmt.Errorf("delegate_to is required")
	}

	policy, err := s.store.GetPolicy(ctx, cfg.PolicyID)
	if err != nil {
		return nil, fmt.Errorf("get policy: %w", err)
	}
	if policy == nil {
		return nil, fmt.Errorf("policy %s not found", cfg.PolicyID)
	}

	policy.DelegateTo = cfg.DelegateTo
	if cfg.TimeoutMinutes > 0 {
		b, _ := json.Marshal(map[string]interface{}{
			"delegation_timeout_minutes": cfg.TimeoutMinutes,
			"escalate_to":               cfg.EscalateTo,
		})
		policy.Conditions = b
	}

	if err := s.store.UpdatePolicy(ctx, policy); err != nil {
		return nil, fmt.Errorf("update policy: %w", err)
	}

	return &DelegationInfo{
		PolicyID:       cfg.PolicyID,
		DelegateTo:     cfg.DelegateTo,
		TimeoutMinutes: cfg.TimeoutMinutes,
		EscalateTo:     cfg.EscalateTo,
		IsActive:       true,
	}, nil
}

func (s *DelegationService) GetDelegation(ctx context.Context, policyID string) (*DelegationInfo, error) {
	if policyID == "" {
		return nil, fmt.Errorf("policy_id is required")
	}

	policy, err := s.store.GetPolicy(ctx, policyID)
	if err != nil {
		return nil, fmt.Errorf("get policy: %w", err)
	}
	if policy == nil {
		return nil, fmt.Errorf("policy %s not found", policyID)
	}

	timeout, escalateTo := parseDelegationConditions(policy.Conditions)

	return &DelegationInfo{
		PolicyID:       policyID,
		DelegateTo:     policy.DelegateTo,
		TimeoutMinutes: timeout,
		EscalateTo:     escalateTo,
		IsActive:       policy.IsActive && policy.DelegateTo != "",
	}, nil
}

func (s *DelegationService) RevokeDelegation(ctx context.Context, policyID string) error {
	if policyID == "" {
		return fmt.Errorf("policy_id is required")
	}

	policy, err := s.store.GetPolicy(ctx, policyID)
	if err != nil {
		return fmt.Errorf("get policy: %w", err)
	}
	if policy == nil {
		return fmt.Errorf("policy %s not found", policyID)
	}

	policy.DelegateTo = ""
	policy.Conditions = json.RawMessage(`{}`)

	if err := s.store.UpdatePolicy(ctx, policy); err != nil {
		return fmt.Errorf("update policy: %w", err)
	}

	return nil
}

func (s *DelegationService) CheckEscalation(policyID string, delegatedAt time.Time) (string, error) {
	ctx := context.Background()
	info, err := s.GetDelegation(ctx, policyID)
	if err != nil {
		return "", err
	}

	if info.TimeoutMinutes <= 0 {
		return "", nil
	}

	deadline := delegatedAt.Add(time.Duration(info.TimeoutMinutes) * time.Minute)
	if time.Now().After(deadline) {
		if info.EscalateTo != "" {
			return info.EscalateTo, nil
		}
		return "original_approver", nil
	}

	return "", nil
}

func parseDelegationConditions(conditions json.RawMessage) (timeoutMinutes int, escalateTo string) {
	if len(conditions) == 0 {
		return 0, ""
	}

	var m map[string]interface{}
	if err := json.Unmarshal(conditions, &m); err != nil {
		return 0, ""
	}

	if v, ok := m["delegation_timeout_minutes"].(float64); ok {
		timeoutMinutes = int(v)
	}
	if v, ok := m["escalate_to"].(string); ok {
		escalateTo = v
	}

	return timeoutMinutes, escalateTo
}
