//go:build libolm

package crypto

import (
	"maunium.net/go/mautrix/crypto/libolm"
)

func initOlmBackend() error {
	libolm.Register()
	return nil
}
