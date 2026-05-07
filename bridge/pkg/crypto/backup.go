package crypto

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"golang.org/x/crypto/argon2"
)

var (
	ErrBackupDisabled     = errors.New("E2EE backup feature is disabled")
	ErrInvalidUserID      = errors.New("user ID is required")
	ErrInvalidBackupID    = errors.New("backup ID is required")
	ErrInvalidRecoveryLen = errors.New("recovery phrase must be 24 words")
	ErrNoEncryptedKey     = errors.New("encrypted key is required")
)

const (
	argon2Memory      = 64 * 1024
	argon2Iterations  = 3
	argon2Parallelism = 4
	argon2KeyLen      = 32
	argon2SaltLen     = 16
)

type BackupRecord struct {
	BackupID     string `json:"backup_id"`
	UserID       string `json:"user_id"`
	EncryptedKey []byte `json:"encrypted_key"`
	SaltHex      string `json:"salt"`
}

type BackupManager struct {
	store   *BackupStore
	enabled bool
}

func NewBackupManager(store *BackupStore, e2eeBackupEnabled bool) *BackupManager {
	return &BackupManager{
		store:   store,
		enabled: e2eeBackupEnabled,
	}
}

func (bm *BackupManager) checkEnabled() error {
	if !bm.enabled {
		return ErrBackupDisabled
	}
	return nil
}

func (bm *BackupManager) CreateBackup(userID string, recoveryPhrase []string, encryptedKey []byte) error {
	if err := bm.checkEnabled(); err != nil {
		return err
	}

	if userID == "" {
		return ErrInvalidUserID
	}
	if len(recoveryPhrase) != 24 {
		return ErrInvalidRecoveryLen
	}
	if len(encryptedKey) == 0 {
		return ErrNoEncryptedKey
	}

	phraseBytes := []byte(strings.Join(recoveryPhrase, " "))

	salt := make([]byte, argon2SaltLen)
	if _, err := rand.Read(salt); err != nil {
		return fmt.Errorf("failed to generate salt: %w", err)
	}

	derivedKey := argon2.IDKey(phraseBytes, salt, argon2Iterations, argon2Memory, argon2Parallelism, argon2KeyLen)

	backupID := fmt.Sprintf("%s-%s", userID, hex.EncodeToString(derivedKey[:8]))

	record := BackupRecord{
		BackupID:     backupID,
		UserID:       userID,
		EncryptedKey: encryptedKey,
		SaltHex:      hex.EncodeToString(salt),
	}

	data, err := json.Marshal(record)
	if err != nil {
		return fmt.Errorf("failed to marshal backup record: %w", err)
	}

	if err := bm.store.Store(backupID, data); err != nil {
		return fmt.Errorf("failed to store backup: %w", err)
	}

	return nil
}

func (bm *BackupManager) DeleteBackup(userID string, backupID string) error {
	if err := bm.checkEnabled(); err != nil {
		return err
	}

	if userID == "" {
		return ErrInvalidUserID
	}
	if backupID == "" {
		return ErrInvalidBackupID
	}

	data, err := bm.store.Load(backupID)
	if err != nil {
		return fmt.Errorf("backup not found: %w", err)
	}

	var record BackupRecord
	if err := json.Unmarshal(data, &record); err != nil {
		return fmt.Errorf("corrupt backup record: %w", err)
	}

	if record.UserID != userID {
		return fmt.Errorf("backup does not belong to user %s", userID)
	}

	return bm.store.Delete(backupID)
}

func (bm *BackupManager) BackupExists(userID string, backupID string) (bool, error) {
	if err := bm.checkEnabled(); err != nil {
		return false, err
	}

	if userID == "" {
		return false, ErrInvalidUserID
	}
	if backupID == "" {
		return false, ErrInvalidBackupID
	}

	exists, err := bm.store.Exists(backupID)
	if err != nil {
		return false, err
	}
	if !exists {
		return false, nil
	}

	data, err := bm.store.Load(backupID)
	if err != nil {
		return false, err
	}

	var record BackupRecord
	if err := json.Unmarshal(data, &record); err != nil {
		return false, nil
	}

	return record.UserID == userID, nil
}
