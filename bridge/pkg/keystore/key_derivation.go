// Package keystore provides Argon2id-based key derivation for zero-trust keystore.
// This implements memory-hard key derivation to resist GPU/ASIC attacks.
package keystore

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"time"

	"golang.org/x/crypto/argon2"
	"golang.org/x/crypto/chacha20poly1305"
)

// Key derivation errors
var (
	ErrInvalidKeyLength  = errors.New("invalid key length")
	ErrInvalidSaltLength = errors.New("invalid salt length")
	ErrInvalidWrappedKey = errors.New("invalid wrapped key")
	ErrDecryptionFailed  = errors.New("decryption failed - wrong password or corrupted data")
	ErrInvalidKeyParams  = errors.New("invalid key derivation parameters")
)

// DefaultKeyDerivationParams provides secure defaults for Argon2id
var DefaultKeyDerivationParams = KeyDerivationParams{
	Memory:      64 * 1024, // 64 MB
	Iterations:  3,
	Parallelism: 4,
	KeyLength:   32, // 256 bits
	SaltLength:  16,
}

// KeyDerivationParams holds Argon2id parameters
type KeyDerivationParams struct {
	// Memory is the memory cost in KiB (default: 64 MB = 65536 KiB)
	Memory uint32 `json:"memory"`

	// Iterations is the time cost (default: 3)
	Iterations uint32 `json:"iterations"`

	// Parallelism is the number of threads (default: 4)
	Parallelism uint8 `json:"parallelism"`

	// KeyLength is the output key length in bytes (default: 32)
	KeyLength uint32 `json:"key_length"`

	// SaltLength is the salt length in bytes (default: 16)
	SaltLength uint32 `json:"salt_length"`
}

// Validate validates the key derivation parameters
func (p *KeyDerivationParams) Validate() error {
	if p.Memory < 8*1024 { // Minimum 8 MB
		return fmt.Errorf("%w: memory must be at least 8192 KiB", ErrInvalidKeyParams)
	}
	if p.Iterations < 1 {
		return fmt.Errorf("%w: iterations must be at least 1", ErrInvalidKeyParams)
	}
	if p.Parallelism < 1 {
		return fmt.Errorf("%w: parallelism must be at least 1", ErrInvalidKeyParams)
	}
	if p.KeyLength < 16 {
		return fmt.Errorf("%w: key length must be at least 16 bytes", ErrInvalidKeyParams)
	}
	if p.SaltLength < 8 {
		return fmt.Errorf("%w: salt length must be at least 8 bytes", ErrInvalidKeyParams)
	}
	return nil
}

// DerivedKey represents a derived key with its parameters
type DerivedKey struct {
	// Key is the derived key material
	Key []byte `json:"-"`

	// Salt is the salt used for derivation
	Salt []byte `json:"salt"`

	// Params are the derivation parameters
	Params KeyDerivationParams `json:"params"`
}

// WrappedKey represents an encrypted key with metadata
type WrappedKey struct {
	// Ciphertext is the encrypted key (includes auth tag)
	Ciphertext []byte `json:"ciphertext"`

	// Nonce is the encryption nonce
	Nonce []byte `json:"nonce"`

	// Salt is the key derivation salt
	Salt []byte `json:"salt"`

	// Params are the derivation parameters used
	Params KeyDerivationParams `json:"params"`

	// Version is the wrapping algorithm version
	Version int `json:"version"`
}

// KeyDerivation provides Argon2id-based key derivation
type KeyDerivation struct {
	params KeyDerivationParams
}

// NewKeyDerivation creates a new key derivation instance
func NewKeyDerivation(params KeyDerivationParams) (*KeyDerivation, error) {
	if err := params.Validate(); err != nil {
		return nil, err
	}

	return &KeyDerivation{
		params: params,
	}, nil
}

// DeriveKey derives a key from a password using Argon2id
func (kd *KeyDerivation) DeriveKey(password []byte, salt []byte) (*DerivedKey, error) {
	if len(salt) == 0 {
		return nil, ErrInvalidSaltLength
	}

	key := argon2.IDKey(
		password,
		salt,
		kd.params.Iterations,
		kd.params.Memory,
		kd.params.Parallelism,
		kd.params.KeyLength,
	)

	return &DerivedKey{
		Key:    key,
		Salt:   salt,
		Params: kd.params,
	}, nil
}

// DeriveKeyWithNewSalt derives a key with a newly generated salt
func (kd *KeyDerivation) DeriveKeyWithNewSalt(password []byte) (*DerivedKey, error) {
	salt := make([]byte, kd.params.SaltLength)
	if _, err := io.ReadFull(rand.Reader, salt); err != nil {
		return nil, fmt.Errorf("failed to generate salt: %w", err)
	}

	return kd.DeriveKey(password, salt)
}

// WrapKey encrypts a key using a password
func (kd *KeyDerivation) WrapKey(plaintextKey []byte, password []byte) (*WrappedKey, error) {
	if len(plaintextKey) != 32 {
		return nil, ErrInvalidKeyLength
	}

	// Derive KEK from password
	derived, err := kd.DeriveKeyWithNewSalt(password)
	if err != nil {
		return nil, err
	}
	defer ZeroBytes(derived.Key)

	// Create AEAD cipher
	aead, err := chacha20poly1305.NewX(derived.Key)
	if err != nil {
		return nil, fmt.Errorf("failed to create cipher: %w", err)
	}

	// Generate nonce
	nonce := make([]byte, aead.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, fmt.Errorf("failed to generate nonce: %w", err)
	}

	// Encrypt
	ciphertext := aead.Seal(nil, nonce, plaintextKey, nil)

	return &WrappedKey{
		Ciphertext: ciphertext,
		Nonce:      nonce,
		Salt:       derived.Salt,
		Params:     kd.params,
		Version:    1,
	}, nil
}

// UnwrapKey decrypts a key using a password
func (kd *KeyDerivation) UnwrapKey(wrapped *WrappedKey, password []byte) ([]byte, error) {
	if wrapped == nil {
		return nil, ErrInvalidWrappedKey
	}

	// Verify version
	if wrapped.Version != 1 {
		return nil, fmt.Errorf("%w: unsupported version %d", ErrInvalidWrappedKey, wrapped.Version)
	}

	// Derive KEK from password using stored parameters
	derived, err := kd.deriveWithParams(password, wrapped.Salt, wrapped.Params)
	if err != nil {
		return nil, err
	}
	defer ZeroBytes(derived.Key)

	// Create AEAD cipher
	aead, err := chacha20poly1305.NewX(derived.Key)
	if err != nil {
		return nil, fmt.Errorf("failed to create cipher: %w", err)
	}

	// Verify nonce size
	if len(wrapped.Nonce) != aead.NonceSize() {
		return nil, fmt.Errorf("%w: invalid nonce size", ErrInvalidWrappedKey)
	}

	// Decrypt
	plaintext, err := aead.Open(nil, wrapped.Nonce, wrapped.Ciphertext, nil)
	if err != nil {
		return nil, ErrDecryptionFailed
	}

	return plaintext, nil
}

// deriveWithParams derives a key with specific parameters
func (kd *KeyDerivation) deriveWithParams(password, salt []byte, params KeyDerivationParams) (*DerivedKey, error) {
	key := argon2.IDKey(
		password,
		salt,
		params.Iterations,
		params.Memory,
		params.Parallelism,
		params.KeyLength,
	)

	return &DerivedKey{
		Key:    key,
		Salt:   salt,
		Params: params,
	}, nil
}

// VerifyPassword checks if a password can decrypt a wrapped key
func (kd *KeyDerivation) VerifyPassword(wrapped *WrappedKey, password []byte) bool {
	_, err := kd.UnwrapKey(wrapped, password)
	return err == nil
}

// Rekey re-encrypts a key with a new password
func (kd *KeyDerivation) Rekey(wrapped *WrappedKey, oldPassword, newPassword []byte) (*WrappedKey, error) {
	// Decrypt with old password
	plaintext, err := kd.UnwrapKey(wrapped, oldPassword)
	if err != nil {
		return nil, err
	}
	defer ZeroBytes(plaintext)

	// Re-encrypt with new password
	return kd.WrapKey(plaintext, newPassword)
}

// ChangeParams re-encrypts a key with new derivation parameters
func (kd *KeyDerivation) ChangeParams(wrapped *WrappedKey, password []byte, newParams KeyDerivationParams) (*WrappedKey, error) {
	if err := newParams.Validate(); err != nil {
		return nil, err
	}

	// Decrypt with current params
	plaintext, err := kd.UnwrapKey(wrapped, password)
	if err != nil {
		return nil, err
	}
	defer ZeroBytes(plaintext)

	// Re-encrypt with new params
	newKD := &KeyDerivation{params: newParams}
	return newKD.WrapKey(plaintext, password)
}

// MarshalJSON serializes a wrapped key to JSON
func (wk *WrappedKey) MarshalJSON() ([]byte, error) {
	type Alias struct {
		Ciphertext string              `json:"ciphertext"`
		Nonce      string              `json:"nonce"`
		Salt       string              `json:"salt"`
		Params     KeyDerivationParams `json:"params"`
		Version    int                 `json:"version"`
	}

	return json.Marshal(&Alias{
		Ciphertext: base64.RawURLEncoding.EncodeToString(wk.Ciphertext),
		Nonce:      base64.RawURLEncoding.EncodeToString(wk.Nonce),
		Salt:       base64.RawURLEncoding.EncodeToString(wk.Salt),
		Params:     wk.Params,
		Version:    wk.Version,
	})
}

// UnmarshalJSON deserializes a wrapped key from JSON
func (wk *WrappedKey) UnmarshalJSON(data []byte) error {
	type Alias struct {
		Ciphertext string              `json:"ciphertext"`
		Nonce      string              `json:"nonce"`
		Salt       string              `json:"salt"`
		Params     KeyDerivationParams `json:"params"`
		Version    int                 `json:"version"`
	}

	var alias Alias
	if err := json.Unmarshal(data, &alias); err != nil {
		return err
	}

	ciphertext, err := base64.RawURLEncoding.DecodeString(alias.Ciphertext)
	if err != nil {
		return fmt.Errorf("invalid ciphertext encoding: %w", err)
	}

	nonce, err := base64.RawURLEncoding.DecodeString(alias.Nonce)
	if err != nil {
		return fmt.Errorf("invalid nonce encoding: %w", err)
	}

	salt, err := base64.RawURLEncoding.DecodeString(alias.Salt)
	if err != nil {
		return fmt.Errorf("invalid salt encoding: %w", err)
	}

	wk.Ciphertext = ciphertext
	wk.Nonce = nonce
	wk.Salt = salt
	wk.Params = alias.Params
	wk.Version = alias.Version

	return nil
}

// ConstantTimeCompare compares two byte slices in constant time
func ConstantTimeCompare(a, b []byte) bool {
	return subtle.ConstantTimeCompare(a, b) == 1
}

// GenerateRandomKey generates a cryptographically secure random key
func GenerateRandomKey(length int) ([]byte, error) {
	if length < 16 {
		return nil, ErrInvalidKeyLength
	}

	key := make([]byte, length)
	if _, err := io.ReadFull(rand.Reader, key); err != nil {
		return nil, fmt.Errorf("failed to generate random key: %w", err)
	}

	return key, nil
}

// GetDefaultParams returns the default key derivation parameters
func GetDefaultParams() KeyDerivationParams {
	return DefaultKeyDerivationParams
}

// GetParams returns the current parameters
func (kd *KeyDerivation) GetParams() KeyDerivationParams {
	return kd.params
}

// SetParams updates the key derivation parameters
func (kd *KeyDerivation) SetParams(params KeyDerivationParams) error {
	if err := params.Validate(); err != nil {
		return err
	}
	kd.params = params
	return nil
}

// EstimateDerivationTime estimates how long key derivation will take
// This is useful for UI feedback during key derivation
func EstimateDerivationTime(params KeyDerivationParams) time.Duration {
	// Rough estimate based on typical performance
	// Argon2id at default params (64MB, 3 iterations) takes ~100-500ms on modern CPUs
	baseTime := 200 * time.Millisecond // Base time for default params

	// Scale by memory ratio
	memoryRatio := float64(params.Memory) / float64(DefaultKeyDerivationParams.Memory)

	// Scale by iterations ratio
	iterRatio := float64(params.Iterations) / float64(DefaultKeyDerivationParams.Iterations)

	return time.Duration(float64(baseTime) * memoryRatio * iterRatio)
}
