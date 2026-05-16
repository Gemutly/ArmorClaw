package security

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func tempConfigStore(t *testing.T) *ConfigStore {
	t.Helper()
	path := filepath.Join(t.TempDir(), "security_config_test.db")
	store, err := NewConfigStore(ConfigStoreConfig{Path: path})
	require.NoError(t, err)
	t.Cleanup(func() { store.Close() })
	return store
}

func TestConfigStore_SetAndGetCategoryConfig(t *testing.T) {
	store := tempConfigStore(t)

	cfg := &CategoryConfig{
		Permission:       PermissionAllow,
		AllowedWebsites:  []string{"trusted.com"},
		RequiresApproval: true,
		AuditLevel:       AuditStandard,
	}

	err := store.SetCategoryConfig(context.Background(), CategoryBanking, cfg, "@admin:test.com")
	require.NoError(t, err)

	got, err := store.GetCategoryConfig(context.Background(), CategoryBanking)
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, PermissionAllow, got.Permission)
	assert.True(t, got.RequiresApproval)
	assert.Equal(t, "@admin:test.com", got.ConfiguredBy)
}

func TestConfigStore_GetNonExistentCategory(t *testing.T) {
	store := tempConfigStore(t)

	got, err := store.GetCategoryConfig(context.Background(), CategoryMedical)
	require.NoError(t, err)
	assert.Nil(t, got)
}

func TestConfigStore_UpdateExistingConfig(t *testing.T) {
	store := tempConfigStore(t)

	cfg1 := &CategoryConfig{
		Permission:   PermissionDeny,
		AuditLevel:   AuditNone,
	}
	err := store.SetCategoryConfig(context.Background(), CategoryPII, cfg1, "@admin:test.com")
	require.NoError(t, err)

	cfg2 := &CategoryConfig{
		Permission:       PermissionAllow,
		AllowedWebsites:  []string{"safe.com"},
		RequiresApproval: true,
		AuditLevel:       AuditVerbose,
	}
	err = store.SetCategoryConfig(context.Background(), CategoryPII, cfg2, "@admin:test.com")
	require.NoError(t, err)

	got, err := store.GetCategoryConfig(context.Background(), CategoryPII)
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, PermissionAllow, got.Permission)
	assert.Equal(t, AuditVerbose, got.AuditLevel)
}

func TestConfigStore_AddAndListCustomPatterns(t *testing.T) {
	store := tempConfigStore(t)

	err := store.AddCustomPattern(context.Background(), &CustomPattern{
		ID:       "pat_1",
		Category: "banking",
		Pattern:  `\d{4}-\d{4}-\d{4}-\d{4}`,
		IsActive: true,
	})
	require.NoError(t, err)

	err = store.AddCustomPattern(context.Background(), &CustomPattern{
		ID:       "pat_2",
		Category: "pii",
		Pattern:  `\d{3}-\d{2}-\d{4}`,
		IsActive: true,
	})
	require.NoError(t, err)

	patterns, err := store.ListCustomPatterns(context.Background(), "")
	require.NoError(t, err)
	assert.Len(t, patterns, 2)

	piiOnly, err := store.ListCustomPatterns(context.Background(), "pii")
	require.NoError(t, err)
	assert.Len(t, piiOnly, 1)
	assert.Equal(t, "pat_2", piiOnly[0].ID)
}

func TestConfigStore_AddInvalidRegex(t *testing.T) {
	store := tempConfigStore(t)

	err := store.AddCustomPattern(context.Background(), &CustomPattern{
		ID:       "pat_bad",
		Category: "banking",
		Pattern:  `[invalid(`,
		IsActive: true,
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid regex")
}

func TestConfigStore_DeleteCustomPattern(t *testing.T) {
	store := tempConfigStore(t)

	err := store.AddCustomPattern(context.Background(), &CustomPattern{
		ID:       "pat_del",
		Category: "pii",
		Pattern:  `\d+`,
		IsActive: true,
	})
	require.NoError(t, err)

	err = store.DeleteCustomPattern(context.Background(), "pat_del")
	require.NoError(t, err)

	patterns, err := store.ListCustomPatterns(context.Background(), "")
	require.NoError(t, err)
	assert.Len(t, patterns, 0)
}

func TestConfigStore_MatchCustomPatterns(t *testing.T) {
	store := tempConfigStore(t)

	err := store.AddCustomPattern(context.Background(), &CustomPattern{
		ID:       "pat_cc",
		Category: "banking",
		Pattern:  `\d{4}-\d{4}-\d{4}-\d{4}`,
		IsActive: true,
	})
	require.NoError(t, err)

	err = store.AddCustomPattern(context.Background(), &CustomPattern{
		ID:       "pat_ssn",
		Category: "pii",
		Pattern:  `\d{3}-\d{2}-\d{4}`,
		IsActive: true,
	})
	require.NoError(t, err)

	matches := store.MatchCustomPatterns("My card is 1234-5678-9012-3456", "banking")
	assert.Len(t, matches, 1)
	assert.Equal(t, "pat_cc", matches[0].ID)

	matches = store.MatchCustomPatterns("SSN: 123-45-6789", "pii")
	assert.Len(t, matches, 1)
	assert.Equal(t, "pat_ssn", matches[0].ID)

	matches = store.MatchCustomPatterns("Hello world", "banking")
	assert.Len(t, matches, 0)
}

func TestConfigStore_EmptyPatternFails(t *testing.T) {
	store := tempConfigStore(t)

	err := store.AddCustomPattern(context.Background(), &CustomPattern{
		ID:       "pat_empty",
		Category: "banking",
		Pattern:  "",
		IsActive: true,
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "pattern is required")
}

func TestConfigStore_Persistence(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "persist.db")

	store1, err := NewConfigStore(ConfigStoreConfig{Path: dbPath})
	require.NoError(t, err)

	cfg := &CategoryConfig{
		Permission:  PermissionAllow,
		AuditLevel:  AuditStandard,
	}
	err = store1.SetCategoryConfig(context.Background(), CategoryBanking, cfg, "@admin:test.com")
	require.NoError(t, err)

	err = store1.AddCustomPattern(context.Background(), &CustomPattern{
		ID:       "pat_persist",
		Category: "banking",
		Pattern:  `\d{16}`,
		IsActive: true,
	})
	require.NoError(t, err)
	store1.Close()

	store2, err := NewConfigStore(ConfigStoreConfig{Path: dbPath})
	require.NoError(t, err)
	defer store2.Close()

	got, err := store2.GetCategoryConfig(context.Background(), CategoryBanking)
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, PermissionAllow, got.Permission)

	patterns, err := store2.ListCustomPatterns(context.Background(), "banking")
	require.NoError(t, err)
	assert.Len(t, patterns, 1)
	assert.Equal(t, "pat_persist", patterns[0].ID)
}

func TestConfigStore_MatchAllCategories(t *testing.T) {
	store := tempConfigStore(t)

	err := store.AddCustomPattern(context.Background(), &CustomPattern{
		ID:       "pat_1",
		Category: "banking",
		Pattern:  `\d{4}`,
		IsActive: true,
	})
	require.NoError(t, err)

	matches := store.MatchCustomPatterns("Code 1234", "")
	assert.Len(t, matches, 1)

	matches = store.MatchCustomPatterns("Code 1234", "pii")
	assert.Len(t, matches, 0)
}
