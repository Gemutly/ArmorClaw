package crypto

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

type backupEntry struct {
	EncryptedKey []byte `json:"encrypted_key"`
	UserID       string `json:"user_id"`
}

type BackupStore struct {
	mu       sync.RWMutex
	basePath string
	data     map[string][]byte
}

func NewBackupStore(vaultPath string) (*BackupStore, error) {
	if vaultPath == "" {
		return nil, fmt.Errorf("vault path is required")
	}

	if err := os.MkdirAll(vaultPath, 0700); err != nil {
		return nil, fmt.Errorf("failed to create vault directory: %w", err)
	}

	bs := &BackupStore{
		basePath: vaultPath,
		data:     make(map[string][]byte),
	}

	if err := bs.loadFromDisk(); err != nil {
		return nil, fmt.Errorf("failed to load backup store: %w", err)
	}

	return bs, nil
}

func (bs *BackupStore) loadFromDisk() error {
	entries, err := os.ReadDir(bs.basePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		if filepath.Ext(entry.Name()) != ".json" {
			continue
		}
		backupID := entry.Name()[:len(entry.Name())-5]
		data, err := os.ReadFile(filepath.Join(bs.basePath, entry.Name()))
		if err != nil {
			continue
		}
		bs.data[backupID] = data
	}

	return nil
}

func (bs *BackupStore) Store(backupID string, data []byte) error {
	if backupID == "" {
		return fmt.Errorf("backup ID is required")
	}
	if len(data) == 0 {
		return fmt.Errorf("backup data is required")
	}

	bs.mu.Lock()
	defer bs.mu.Unlock()

	filePath := filepath.Join(bs.basePath, backupID+".json")
	if err := os.WriteFile(filePath, data, 0600); err != nil {
		return fmt.Errorf("failed to write backup: %w", err)
	}

	bs.data[backupID] = make([]byte, len(data))
	copy(bs.data[backupID], data)

	return nil
}

func (bs *BackupStore) Load(backupID string) ([]byte, error) {
	bs.mu.RLock()
	defer bs.mu.RUnlock()

	data, ok := bs.data[backupID]
	if !ok {
		return nil, fmt.Errorf("backup not found: %s", backupID)
	}

	result := make([]byte, len(data))
	copy(result, data)
	return result, nil
}

func (bs *BackupStore) Delete(backupID string) error {
	bs.mu.Lock()
	defer bs.mu.Unlock()

	if _, ok := bs.data[backupID]; !ok {
		return fmt.Errorf("backup not found: %s", backupID)
	}

	filePath := filepath.Join(bs.basePath, backupID+".json")
	if err := os.Remove(filePath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to delete backup file: %w", err)
	}

	delete(bs.data, backupID)
	return nil
}

func (bs *BackupStore) Exists(backupID string) (bool, error) {
	bs.mu.RLock()
	defer bs.mu.RUnlock()

	_, ok := bs.data[backupID]
	return ok, nil
}
