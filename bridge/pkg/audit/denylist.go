package audit

// AuditDenylist defines field names that must never appear in audit log entries.
// Any map keys matching these patterns should be redacted before persistence.
// Challenge-response secrets are kept (alternative policy). HMAC proof references
// from the rejected challenge-ONLY design are intentionally absent.
var AuditDenylist = map[string]struct{}{
	"password":        {},
	"passphrase":      {},
	"password_hash":   {},
	"passphrase_hash": {},
	"verifier":        {},
	"verify_salt":     {},
	"vault_key":       {},
	"vault_key_enc":   {},
	"recovery_phrase": {},
	"recovery_seed":   {},
	"wrapped_key":     {},
	"master_key":      {},
	"challenge_nonce":     {},
	"challenge_token":     {},
	"ed25519_private_key": {},
	"signing_key":         {},
	"session_token":       {},
}
