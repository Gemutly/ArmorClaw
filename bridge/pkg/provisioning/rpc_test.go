package provisioning

import (
	"testing"
)

func TestValidatePassphraseStrength(t *testing.T) {
	tests := []struct {
		name    string
		pass    string
		wantErr bool
	}{
		{"valid strong passphrase", "SecurePass1phrase", false},
		{"valid with special chars", "MyP@ssw0rd!2024", false},
		{"exactly 12 chars valid", "Abcdefgh1234", false},
		{"too short", "Short1A", true},
		{"11 chars one short", "Abcdefghij1", true},
		{"no uppercase", "lowercasepass1", true},
		{"no lowercase", "UPPERCASEPASS1", true},
		{"no digit", "NoDigitsHereA", true},
		{"empty string", "", true},
		{"only lowercase", "abcdefghijklmnop", true},
		{"only digits", "123456789012", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validatePassphraseStrength(tt.pass)
			if (err != nil) != tt.wantErr {
				t.Errorf("validatePassphraseStrength(%q) error = %v, wantErr %v", tt.pass, err, tt.wantErr)
			}
		})
	}
}
