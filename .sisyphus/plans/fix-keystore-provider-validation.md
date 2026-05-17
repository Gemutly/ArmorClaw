# Fix: Keystore Provider Validation

## Issue
The `isValidProvider()` function in `keystore.go` only accepts hardcoded canonical IDs. This causes validation failures when storing API keys for providers that exist in the registry but aren't in the hardcoded list.

## Root Cause
1. `isValidProvider` in `keystore.go` only checks: `openai`, `anthropic`, `openrouter`, `google`, `xai`, `zhipu`
2. Provider registry has additional providers: `deepseek`, `moonshot`, `nvidia`, `groq`, `cloudflare`, `ollama`
3. When bridge tries to store API key with provider ID from registry, validation fails

## Error Message
```
Warning: Failed to store z.ai API key: invalid provider
```

## Solution
Update `isValidProvider` function in `bridge/pkg/keystore/keystore.go` to include all providers from the registry.

### Current Code (line 869-877)
```go
func isValidProvider(p Provider) bool {
	switch p {
	case ProviderOpenAI, ProviderAnthropic, ProviderOpenRouter, ProviderGoogle, ProviderXAI, ProviderZhipu:
		return true
	default:
		return false
	}
}
```

### Required Changes

1. **Add new provider constants** (lines 62-69):
```go
const (
	ProviderOpenAI     Provider = "openai"
	ProviderAnthropic  Provider = "anthropic"
	ProviderOpenRouter Provider = "openrouter"
	ProviderGoogle     Provider = "google"
	ProviderXAI        Provider = "xai"
	ProviderZhipu      Provider = "zhipu"
	ProviderDeepSeek   Provider = "deepseek"
	ProviderMoonshot   Provider = "moonshot"
	ProviderNVIDIA     Provider = "nvidia"
	ProviderGroq       Provider = "groq"
	ProviderCloudflare Provider = "cloudflare"
	ProviderOllama     Provider = "ollama"
)
```

2. **Update isValidProvider function**:
```go
func isValidProvider(p Provider) bool {
	switch p {
	case ProviderOpenAI, ProviderAnthropic, ProviderOpenRouter, ProviderGoogle, ProviderXAI, ProviderZhipu,
		ProviderDeepSeek, ProviderMoonshot, ProviderNVIDIA, ProviderGroq, ProviderCloudflare, ProviderOllama:
		return true
	default:
		return false
	}
}
```

## Files to Modify
- `bridge/pkg/keystore/keystore.go`

## Testing
1. Build bridge binary
2. Deploy to VPS
3. Restart bridge service
4. Verify API key storage works
5. Test AI chat with zhipu provider

## Status
- [x] Issue identified
- [x] Root cause found
- [x] Solution documented
- [x] Code changes applied
- [x] Tested on VPS
- [x] Committed to repo
