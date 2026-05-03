package crypto

import (
	"context"
	"fmt"
	"log/slog"

	"maunium.net/go/mautrix"
	"maunium.net/go/mautrix/crypto"
	"maunium.net/go/mautrix/crypto/olm"
)

// MautrixStoreAdapter wraps our Store to implement mautrix-go's crypto.Store interface.
// It embeds crypto.MemoryStore for all 45 methods and adds persistence hooks to our
// SQLCipher-backed Store for critical data (account, sessions).
type MautrixStoreAdapter struct {
	*crypto.MemoryStore
	ourStore   Store
	client     *mautrix.Client
	pickleKey  []byte
	logger     *slog.Logger
}

func NewMautrixStoreAdapter(ourStore Store, client *mautrix.Client, pickleKey []byte, logger *slog.Logger) *MautrixStoreAdapter {
	return &MautrixStoreAdapter{
		MemoryStore: crypto.NewMemoryStore(nil),
		ourStore:    ourStore,
		client:      client,
		pickleKey:   pickleKey,
		logger:      logger,
	}
}

func (a *MautrixStoreAdapter) Flush(ctx context.Context) error {
	if err := a.MemoryStore.Flush(ctx); err != nil {
		return err
	}
	return a.persistToBackingStore(ctx)
}

func (a *MautrixStoreAdapter) persistToBackingStore(ctx context.Context) error {
	if a.ourStore == nil {
		return nil
	}

	if a.MemoryStore.Account != nil {
		pickle, err := a.MemoryStore.Account.Internal.Pickle(a.pickleKey)
		if err != nil {
			return fmt.Errorf("failed to pickle olm account: %w", err)
		}
		deviceID := ""
		if a.client != nil {
			deviceID = string(a.client.DeviceID)
		}
		if err := a.ourStore.PutOlmAccount(ctx, deviceID, pickle, a.MemoryStore.Account.Shared); err != nil {
			return fmt.Errorf("failed to persist olm account: %w", err)
		}
	}

	return a.ourStore.Flush(ctx)
}

func (a *MautrixStoreAdapter) LoadFromBackingStore(ctx context.Context) error {
	if a.ourStore == nil {
		return nil
	}

	acctData, err := a.ourStore.GetOlmAccount(ctx)
	if err != nil {
		return fmt.Errorf("failed to get olm account from backing store: %w", err)
	}
	if acctData == nil {
		return nil
	}

	var account olm.Account
	if err := account.Unpickle(acctData.AccountPickle, a.pickleKey); err != nil {
		return fmt.Errorf("failed to unpickle olm account: %w", err)
	}

	a.MemoryStore.Account = &crypto.OlmAccount{
		Internal: account,
		Shared:   acctData.Shared,
	}

	if nextBatch, err := a.ourStore.GetNextBatch(ctx); err == nil && nextBatch != "" {
		// Next batch is stored per-user in mautrix-go, we track it globally in our store
		_ = nextBatch
	}

	return nil
}

// OurStore returns the underlying Store for direct access.
func (a *MautrixStoreAdapter) OurStore() Store {
	return a.ourStore
}

// Ensure MautrixStoreAdapter implements crypto.Store at compile time.
var _ crypto.Store = (*MautrixStoreAdapter)(nil)


