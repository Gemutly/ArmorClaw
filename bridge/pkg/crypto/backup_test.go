package crypto

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGenerateRecoveryPhrase(t *testing.T) {
	words, err := GenerateRecoveryPhrase()
	if err != nil {
		t.Fatalf("GenerateRecoveryPhrase() error = %v", err)
	}

	if len(words) != 24 {
		t.Fatalf("expected 24 words, got %d", len(words))
	}

	ensureWordlistLoaded()
	seen := make(map[string]bool)
	for i, word := range words {
		idx, ok := bip39WordMap[word]
		if !ok {
			t.Errorf("word %d %q not in BIP-39 wordlist", i, word)
		}
		if idx < 0 || idx >= 2048 {
			t.Errorf("word %d index %d out of range [0, 2048)", i, idx)
		}
		seen[word] = true
	}

	t.Logf("generated %d unique words out of 24", len(seen))
}

func TestValidateRecoveryPhrase(t *testing.T) {
	words, err := GenerateRecoveryPhrase()
	if err != nil {
		t.Fatalf("GenerateRecoveryPhrase() error = %v", err)
	}

	if err := ValidateRecoveryPhrase(words); err != nil {
		t.Fatalf("ValidateRecoveryPhrase() valid phrase failed: %v", err)
	}

	invalidPhrase := make([]string, 24)
	copy(invalidPhrase, words)
	invalidPhrase[0] = "zzzznotaword"
	if err := ValidateRecoveryPhrase(invalidPhrase); err == nil {
		t.Fatal("expected error for invalid word, got nil")
	}

	shortPhrase := words[:12]
	if err := ValidateRecoveryPhrase(shortPhrase); err == nil {
		t.Fatal("expected error for short phrase, got nil")
	}

	swapped := make([]string, 24)
	copy(swapped, words)
	swapped[0], swapped[23] = swapped[23], swapped[0]
	if err := ValidateRecoveryPhrase(swapped); err == nil {
		t.Fatal("expected error for swapped words, got nil")
	}
}

func TestCreateBackup(t *testing.T) {
	vaultPath := t.TempDir()
	store, err := NewBackupStore(vaultPath)
	if err != nil {
		t.Fatalf("NewBackupStore() error = %v", err)
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

	err = bm.CreateBackup("user-123", phrase, encryptedKey)
	if err != nil {
		t.Fatalf("CreateBackup() error = %v", err)
	}

	disabled := NewBackupManager(store, false)
	err = disabled.CreateBackup("user-123", phrase, encryptedKey)
	if err != ErrBackupDisabled {
		t.Fatalf("expected ErrBackupDisabled, got %v", err)
	}
}

func TestDeleteBackup(t *testing.T) {
	vaultPath := t.TempDir()
	store, err := NewBackupStore(vaultPath)
	if err != nil {
		t.Fatalf("NewBackupStore() error = %v", err)
	}

	bm := NewBackupManager(store, true)

	phrase, _ := GenerateRecoveryPhrase()
	encryptedKey := make([]byte, 32)

	if err := bm.CreateBackup("user-del", phrase, encryptedKey); err != nil {
		t.Fatalf("CreateBackup() error = %v", err)
	}

	files, _ := filepath.Glob(filepath.Join(vaultPath, "*.json"))
	if len(files) == 0 {
		t.Fatal("expected at least one backup file after create")
	}
	backupID := strings.TrimSuffix(filepath.Base(files[0]), ".json")

	if err := bm.DeleteBackup("user-del", backupID); err != nil {
		t.Fatalf("DeleteBackup() error = %v", err)
	}

	exists, err := bm.BackupExists("user-del", backupID)
	if err != nil {
		t.Fatalf("BackupExists() error = %v", err)
	}
	if exists {
		t.Fatal("expected backup to not exist after delete")
	}
}

func TestBackupExists(t *testing.T) {
	vaultPath := t.TempDir()
	store, err := NewBackupStore(vaultPath)
	if err != nil {
		t.Fatalf("NewBackupStore() error = %v", err)
	}

	bm := NewBackupManager(store, true)

	exists, err := bm.BackupExists("user-ex", "nonexistent-id")
	if err != nil {
		t.Fatalf("BackupExists() on nonexistent error = %v", err)
	}
	if exists {
		t.Fatal("expected false for nonexistent backup")
	}

	phrase, _ := GenerateRecoveryPhrase()
	encryptedKey := make([]byte, 32)
	if err := bm.CreateBackup("user-ex", phrase, encryptedKey); err != nil {
		t.Fatalf("CreateBackup() error = %v", err)
	}

	files, _ := filepath.Glob(filepath.Join(vaultPath, "*.json"))
	if len(files) == 0 {
		t.Fatal("expected at least one backup file")
	}
	backupID := strings.TrimSuffix(filepath.Base(files[0]), ".json")

	exists, err = bm.BackupExists("user-ex", backupID)
	if err != nil {
		t.Fatalf("BackupExists() error = %v", err)
	}
	if !exists {
		t.Fatal("expected backup to exist after create")
	}
}

func TestNoRestoreBackup(t *testing.T) {
	sourceFiles := []string{"backup.go", "backup_store.go", "recovery_phrase.go"}
	for _, f := range sourceFiles {
		data, err := os.ReadFile(f)
		if err != nil {
			t.Fatalf("cannot read %s: %v", f, err)
		}
		content := string(data)
		if idx := strings.Index(content, "restore_backup"); idx != -1 {
			line := strings.Count(content[:idx], "\n") + 1
			t.Fatalf("restore_backup found in %s at line %d", f, line)
		}
	}
}
