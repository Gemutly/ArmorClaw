package crypto

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestE2EE_FeatureOnValidPath(t *testing.T) {
	vaultPath := t.TempDir()
	store, err := NewBackupStore(vaultPath)
	if err != nil {
		t.Fatalf("NewBackupStore(valid path) error = %v", err)
	}

	bm := NewBackupManager(store, true)

	phrase, err := GenerateRecoveryPhrase()
	if err != nil {
		t.Fatalf("GenerateRecoveryPhrase() error = %v", err)
	}
	encryptedKey := make([]byte, 32)
	for i := range encryptedKey {
		encryptedKey[i] = byte(i)
	}

	// --- create_backup ---
	backupID, err := bm.CreateBackup("user-wiring", phrase, encryptedKey)
	if err != nil {
		t.Fatalf("CreateBackup() error = %v", err)
	}
	if backupID == "" {
		t.Fatal("CreateBackup() returned empty backupID")
	}
	t.Logf("created backup: %s", backupID)

	files, _ := filepath.Glob(filepath.Join(vaultPath, "*.json"))
	if len(files) == 0 {
		t.Fatal("expected backup file on disk after create")
	}

	// --- backup_exists ---
	exists, err := bm.BackupExists("user-wiring", backupID)
	if err != nil {
		t.Fatalf("BackupExists() error = %v", err)
	}
	if !exists {
		t.Fatal("expected backup to exist after create")
	}

	exists, err = bm.BackupExists("other-user", backupID)
	if err != nil {
		t.Fatalf("BackupExists(wrong user) error = %v", err)
	}
	if exists {
		t.Fatal("backup should not exist for different user")
	}

	// --- delete_backup ---
	err = bm.DeleteBackup("user-wiring", backupID)
	if err != nil {
		t.Fatalf("DeleteBackup() error = %v", err)
	}

	exists, err = bm.BackupExists("user-wiring", backupID)
	if err != nil {
		t.Fatalf("BackupExists(after delete) error = %v", err)
	}
	if exists {
		t.Fatal("expected backup to not exist after delete")
	}
}

func TestE2EE_InitFailureDisablesGracefully(t *testing.T) {
	invalidPath := "/proc/nonexistent/path/that/does/not/exist/backup.db"

	store, err := NewBackupStore(invalidPath)
	if err == nil {
		os.RemoveAll(invalidPath)
		t.Fatal("expected NewBackupStore to fail for invalid path, got nil error")
	}
	t.Logf("init failed gracefully: %v", err)

	if store != nil {
		t.Fatal("expected nil store on init failure")
	}

	bm := NewBackupManager(nil, false)

	phrase, _ := GenerateRecoveryPhrase()
	encryptedKey := make([]byte, 32)

	_, err = bm.CreateBackup("user-initfail", phrase, encryptedKey)
	if err != ErrBackupDisabled {
		t.Fatalf("expected ErrBackupDisabled, got %v", err)
	}

	err = bm.DeleteBackup("user-initfail", "some-id")
	if err != ErrBackupDisabled {
		t.Fatalf("expected ErrBackupDisabled, got %v", err)
	}

	_, err = bm.BackupExists("user-initfail", "some-id")
	if err != ErrBackupDisabled {
		t.Fatalf("expected ErrBackupDisabled, got %v", err)
	}
}

func TestE2EE_FeatureOffReturnsError(t *testing.T) {
	vaultPath := t.TempDir()
	store, err := NewBackupStore(vaultPath)
	if err != nil {
		t.Fatalf("NewBackupStore() error = %v", err)
	}

	bm := NewBackupManager(store, false)

	phrase, _ := GenerateRecoveryPhrase()
	encryptedKey := make([]byte, 32)

	tests := []struct {
		name    string
		err     error
		wantErr error
	}{
		{
			name:    "create_backup",
			err:     func() error { _, e := bm.CreateBackup("user-off", phrase, encryptedKey); return e }(),
			wantErr: ErrBackupDisabled,
		},
		{
			name:    "delete_backup",
			err:     bm.DeleteBackup("user-off", "any-id"),
			wantErr: ErrBackupDisabled,
		},
		{
			name:    "backup_exists",
			err:     func() error { _, e := bm.BackupExists("user-off", "any-id"); return e }(),
			wantErr: ErrBackupDisabled,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if tc.err != tc.wantErr {
				t.Errorf("expected %v, got %v", tc.wantErr, tc.err)
			}
		})
	}

	files, _ := filepath.Glob(filepath.Join(vaultPath, "*.json"))
	if len(files) != 0 {
		t.Fatalf("expected no backup files when feature off, found %d", len(files))
	}
}

// SECURITY: restore_backup is intentionally omitted until a secure approval flow is designed.
func TestE2EE_NoRestoreBackupAnywhere(t *testing.T) {
	err := filepath.Walk("..", func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if info.IsDir() || !strings.HasSuffix(path, ".go") {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return nil
		}
		if strings.Contains(string(data), "restore_backup") && !strings.HasSuffix(path, "backup_store_test.go") && !strings.HasSuffix(path, "backup_test.go") {
			t.Errorf("restore_backup found in %s — MUST NOT exist in codebase", path)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("filepath.Walk error: %v", err)
	}
}
