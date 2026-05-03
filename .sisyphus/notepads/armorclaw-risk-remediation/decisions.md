# Decisions - ArmorClaw Risk Remediation

## 2026-05-03 Session Start
- Server-only scope (no ArmorChat Android client changes)
- E2EE kill switch default false (opt-in)
- Dual-mode messaging (plaintext + encrypted rooms)
- Reuse existing SQLCipher keystore (no separate DB)
- Use goolm (pure Go) by default, libolm as fallback build tag
- State inference uses ForceTransition (bypasses validation by design)
