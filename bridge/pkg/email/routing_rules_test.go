package email

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func tempDB(t *testing.T) *RoutingRuleStore {
	t.Helper()
	path := filepath.Join(t.TempDir(), "routing_test.db")
	store, err := NewRoutingRuleStore(RoutingRuleStoreConfig{Path: path})
	require.NoError(t, err)
	t.Cleanup(func() { store.Close() })
	return store
}

func TestRoutingRuleStore_CreateAndGet(t *testing.T) {
	store := tempDB(t)

	rule := &RoutingRule{
		ID:        "rule_1",
		Pattern:   "sales@company.com",
		TeamID:    "team_sales",
		Priority:  10,
		IsActive:  true,
		MatchField: "to",
	}

	err := store.CreateRule(context.Background(), rule)
	require.NoError(t, err)

	got, err := store.GetRule(context.Background(), "rule_1")
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, "sales@company.com", got.Pattern)
	assert.Equal(t, "team_sales", got.TeamID)
	assert.True(t, got.IsActive)
}

func TestRoutingRuleStore_ListRules(t *testing.T) {
	store := tempDB(t)

	for i, pattern := range []string{"sales@co.com", "support@co.com", "billing@co.com"} {
		err := store.CreateRule(context.Background(), &RoutingRule{
			ID:        string(rune('A' + i)),
			Pattern:   pattern,
			TeamID:    "team_" + string(rune('A'+i)),
			Priority:  i,
			IsActive:  true,
			MatchField: "to",
		})
		require.NoError(t, err)
	}

	rules, err := store.ListRules(context.Background())
	require.NoError(t, err)
	assert.Len(t, rules, 3)
}

func TestRoutingRuleStore_DeleteRule(t *testing.T) {
	store := tempDB(t)

	err := store.CreateRule(context.Background(), &RoutingRule{
		ID:        "rule_del",
		Pattern:   "ops@company.com",
		TeamID:    "team_ops",
		Priority:  5,
		IsActive:  true,
		MatchField: "to",
	})
	require.NoError(t, err)

	err = store.DeleteRule(context.Background(), "rule_del")
	require.NoError(t, err)

	got, err := store.GetRule(context.Background(), "rule_del")
	require.NoError(t, err)
	assert.Nil(t, got)
}

func TestRoutingRuleStore_MatchExactEmail(t *testing.T) {
	store := tempDB(t)

	err := store.CreateRule(context.Background(), &RoutingRule{
		ID: "r1", Pattern: "sales@company.com", TeamID: "team_sales", Priority: 10, IsActive: true, MatchField: "to",
	})
	require.NoError(t, err)

	rule, found := store.Match(context.Background(), "sales@company.com", "Hello")
	assert.True(t, found)
	assert.Equal(t, "team_sales", rule.TeamID)

	_, found = store.Match(context.Background(), "random@other.com", "Hello")
	assert.False(t, found)
}

func TestRoutingRuleStore_MatchDomainWildcard(t *testing.T) {
	store := tempDB(t)

	err := store.CreateRule(context.Background(), &RoutingRule{
		ID: "r2", Pattern: "*@company.com", TeamID: "team_company", Priority: 5, IsActive: true, MatchField: "to",
	})
	require.NoError(t, err)

	rule, found := store.Match(context.Background(), "anyone@company.com", "Test")
	assert.True(t, found)
	assert.Equal(t, "team_company", rule.TeamID)

	_, found = store.Match(context.Background(), "anyone@other.com", "Test")
	assert.False(t, found)
}

func TestRoutingRuleStore_MatchSubject(t *testing.T) {
	store := tempDB(t)

	err := store.CreateRule(context.Background(), &RoutingRule{
		ID: "r3", Pattern: "urgent", TeamID: "team_ops", Priority: 20, IsActive: true, MatchField: "subject",
	})
	require.NoError(t, err)

	rule, found := store.Match(context.Background(), "someone@any.com", "URGENT: server down")
	assert.True(t, found)
	assert.Equal(t, "team_ops", rule.TeamID)

	_, found = store.Match(context.Background(), "someone@any.com", "Weekly update")
	assert.False(t, found)
}

func TestRoutingRuleStore_InactiveRuleSkipped(t *testing.T) {
	store := tempDB(t)

	err := store.CreateRule(context.Background(), &RoutingRule{
		ID: "r4", Pattern: "disabled@co.com", TeamID: "team_disabled", Priority: 10, IsActive: false, MatchField: "to",
	})
	require.NoError(t, err)

	_, found := store.Match(context.Background(), "disabled@co.com", "Test")
	assert.False(t, found)
}

func TestRoutingRuleStore_PriorityOrder(t *testing.T) {
	store := tempDB(t)

	err := store.CreateRule(context.Background(), &RoutingRule{
		ID: "low", Pattern: "*@company.com", TeamID: "team_low", Priority: 1, IsActive: true, MatchField: "to",
	})
	require.NoError(t, err)

	err = store.CreateRule(context.Background(), &RoutingRule{
		ID: "high", Pattern: "vip@company.com", TeamID: "team_high", Priority: 100, IsActive: true, MatchField: "to",
	})
	require.NoError(t, err)

	rule, found := store.Match(context.Background(), "vip@company.com", "Hello")
	assert.True(t, found)
	assert.Equal(t, "team_high", rule.TeamID)
}

func TestRoutingRuleStore_Persistence(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "persist_test.db")

	store1, err := NewRoutingRuleStore(RoutingRuleStoreConfig{Path: dbPath})
	require.NoError(t, err)

	err = store1.CreateRule(context.Background(), &RoutingRule{
		ID: "persist_1", Pattern: "persist@co.com", TeamID: "team_persist", Priority: 5, IsActive: true, MatchField: "to",
	})
	require.NoError(t, err)
	store1.Close()

	store2, err := NewRoutingRuleStore(RoutingRuleStoreConfig{Path: dbPath})
	require.NoError(t, err)
	defer store2.Close()

	got, err := store2.GetRule(context.Background(), "persist_1")
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, "team_persist", got.TeamID)
}

func TestRoutingRuleStore_WildcardSubject(t *testing.T) {
	store := tempDB(t)

	err := store.CreateRule(context.Background(), &RoutingRule{
		ID: "r5", Pattern: "invoice*", TeamID: "team_billing", Priority: 15, IsActive: true, MatchField: "subject",
	})
	require.NoError(t, err)

	rule, found := store.Match(context.Background(), "any@any.com", "Invoice #12345 payment")
	assert.True(t, found)
	assert.Equal(t, "team_billing", rule.TeamID)
}

func TestRoutingRuleStore_GetNonExistent(t *testing.T) {
	store := tempDB(t)

	got, err := store.GetRule(context.Background(), "nonexistent")
	require.NoError(t, err)
	assert.Nil(t, got)
}

func TestRoutingRuleStore_DomainSuffix(t *testing.T) {
	store := tempDB(t)

	err := store.CreateRule(context.Background(), &RoutingRule{
		ID: "r6", Pattern: "@globex.com", TeamID: "team_globex", Priority: 5, IsActive: true, MatchField: "to",
	})
	require.NoError(t, err)

	rule, found := store.Match(context.Background(), "ceo@globex.com", "Hello")
	assert.True(t, found)
	assert.Equal(t, "team_globex", rule.TeamID)
}
