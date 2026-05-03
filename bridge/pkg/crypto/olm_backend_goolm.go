//go:build !libolm

package crypto

import (
	"maunium.net/go/mautrix/crypto/goolm"
)

func initOlmBackend() error {
	goolm.Register()
	return nil
}
