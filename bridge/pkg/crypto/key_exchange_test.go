package crypto

import (
	"context"
	"log/slog"
	"testing"
)

func TestKeyExchangeService_NilSafe(t *testing.T) {
	var s *KeyExchangeService

	if err := s.UploadKeys(context.Background()); err != nil {
		t.Errorf("nil UploadKeys should return nil, got: %v", err)
	}

	if err := s.QueryKeys(context.Background(), []string{"@user:test"}); err != nil {
		t.Errorf("nil QueryKeys should return nil, got: %v", err)
	}

	if err := s.ClaimKeys(context.Background(), map[string][]string{
		"@user:test": {"DEVICE1"},
	}); err != nil {
		t.Errorf("nil ClaimKeys should return nil, got: %v", err)
	}

	if err := s.ProcessDeviceListChanges(context.Background(),
		[]string{"@user:test"}, []string{"@other:test"}); err != nil {
		t.Errorf("nil ProcessDeviceListChanges should return nil, got: %v", err)
	}
}

func TestKeyExchangeService_ProcessDeviceListChanges_Empty(t *testing.T) {
	svc := &KeyExchangeService{
		engine: &CryptoEngine{},
		logger: slog.Default(),
	}

	if err := svc.ProcessDeviceListChanges(context.Background(), nil, nil); err != nil {
		t.Errorf("empty lists should return nil, got: %v", err)
	}

	if err := svc.ProcessDeviceListChanges(context.Background(), []string{}, []string{}); err != nil {
		t.Errorf("empty slices should return nil, got: %v", err)
	}
}

func TestNewKeyExchangeService_NilEngine(t *testing.T) {
	svc := NewKeyExchangeService(nil, nil)
	if svc != nil {
		t.Error("NewKeyExchangeService with nil engine should return nil")
	}
}

func TestKeyExchangeService_QueryKeys_Empty(t *testing.T) {
	svc := &KeyExchangeService{
		engine: &CryptoEngine{},
		logger: slog.Default(),
	}

	if err := svc.QueryKeys(context.Background(), nil); err != nil {
		t.Errorf("nil user list should return nil, got: %v", err)
	}

	if err := svc.QueryKeys(context.Background(), []string{}); err != nil {
		t.Errorf("empty user slice should return nil, got: %v", err)
	}
}

func TestKeyExchangeService_ClaimKeys_Empty(t *testing.T) {
	svc := &KeyExchangeService{
		engine: &CryptoEngine{},
		logger: slog.Default(),
	}

	if err := svc.ClaimKeys(context.Background(), nil); err != nil {
		t.Errorf("nil targets should return nil, got: %v", err)
	}

	if err := svc.ClaimKeys(context.Background(), map[string][]string{}); err != nil {
		t.Errorf("empty targets should return nil, got: %v", err)
	}
}
