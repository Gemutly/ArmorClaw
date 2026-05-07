package keystore

import "runtime"

// ZeroBytes securely overwrites the contents of b with zeros.
// It is safe to call with nil or empty slices.
func ZeroBytes(b []byte) {
	if len(b) == 0 {
		return
	}
	for i := range b {
		b[i] = 0
	}
	runtime.KeepAlive(b)
}
