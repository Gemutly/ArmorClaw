// Package sidecar provides office HMAC secret provisioning utilities.
//
// The actual provisioning is performed by the `office-secret-init` one-shot
// Docker Compose service (see deploy/docker-compose.sidecar-py.yml) which:
//   - Runs as root (UID 0) to create and chown the secret file
//   - Generates a 32-byte random hex secret if none exists
//   - Sets ownership to 10001:10001 (sidecar UID)
//   - Sets file mode to 0440
//
// This file provides Bridge-side helpers for reading the provisioned secret
// and generating new secrets when needed (e.g., for test environments).
package sidecar

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
)

const (
	// DefaultHMACSecretPath is the default host-side path for the office HMAC secret.
	DefaultHMACSecretPath = "/run/armorclaw/secrets/office-hmac"
)

// GenerateHMACSecret creates a new 32-byte random hex secret.
// This is used by the init-service during provisioning.
func GenerateHMACSecret() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generate HMAC secret: %w", err)
	}
	return hex.EncodeToString(b), nil
}

// ReadHMACSecret reads the provisioned HMAC secret from the given path.
// Returns the secret as a string, or an error if the file cannot be read.
func ReadHMACSecret(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("read HMAC secret from %s: %w", path, err)
	}
	return string(data), nil
}
