package keystore

import "testing"

func TestZeroBytes(t *testing.T) {
	b := []byte{0xDE, 0xAD, 0xBE, 0xEF}
	ZeroBytes(b)
	for i, v := range b {
		if v != 0 {
			t.Errorf("b[%d] = 0x%02X, want 0x00", i, v)
		}
	}
}

func TestZeroBytesEmpty(t *testing.T) {
	b := []byte{}
	ZeroBytes(b) // must not panic
}

func TestZeroBytesNil(t *testing.T) {
	var b []byte
	ZeroBytes(b) // must not panic
}
