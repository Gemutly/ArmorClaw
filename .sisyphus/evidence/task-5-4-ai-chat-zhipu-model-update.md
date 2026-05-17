# Task 5.4: Test AI chat with zhipu model (UPDATED)

**What to do**: Test AI chat with glm-5 and glm-4.7-flash models on VPS

**Commands executed**:
1. Updated /etc/armorclaw/config.toml with new models
2. Restarted armorclaw-bridge service
3. Test via Unix socket (socat)
4. Test via HTTP API (curl)

**Results**:

### Test 1: Unix Socket with glm-4
```bash
echo '{"jsonrpc":"2.0","id":1,"method":"ai.chat","params":{"model":"glm-4","messages":[{"role":"user","content":"Say hello"}]}}' | socat - UNIX-CONNECT:/run/armorclaw/bridge.sock
```

**Result**:
```
{"jsonrpc":"2.0","id":1,"error":{"code":-10004,"message":"failed to retrieve API key: key not found"}}
```

### Test 2: Unix Socket with glm-4.7-flash
```bash
echo '{"jsonrpc":"2.0","id":2,"method":"ai.chat","params":{"model":"glm-4.7-flash","messages":[{"role":"user","content":"Say hello"}]}}' | socat - UNIX-CONNECT:/run/armorclaw/bridge.sock
```

**Result**:
```
{"jsonrpc":"2.0","id":2,"error":{"code":-10004,"message":"failed to retrieve API key: key not found"}}
```

### Test 3: Unix Socket with glm-5
```bash
echo '{"jsonrpc":"2.0","id":3,"method":"ai.chat","params":{"model":"glm-5","messages":[{"role":"user","content":"Say hello"}]}}' | socat - UNIX-CONNECT:/run/armorclaw/bridge.sock
```

**Result**:
```
{"jsonrpc":"2.0","id":3,"error":{"code":-10004,"message":"failed to retrieve API key: key not found"}}
```

### Test 4: HTTP API with glm-4
```bash
curl -sf -X POST -H "Content-Type: application/json" -d '{"jsonrpc":"2.0","id":4,"method":"ai.chat","params":{"model":"glm-4","messages":[{"role":"user","content":"Say hello"}]}}' http://localhost:8080/api
```

**Result**:
```
{"jsonrpc":"2.0","id":4,"error":{"code":-10004,"message":"failed to retrieve API key: key not found"}}
```

### Test 5: HTTP API with glm-4.7-flash
```bash
curl -sf -X POST -H "Content-Type: application/json" -d '{"jsonrpc":"2.0","id":5,"method":"ai.chat","params":{"model":"glm-4.7-flash","messages":[{"role":"user","content":"Say hello"}]}}' http://localhost:8080/api
```

**Result**:
```
{"jsonrpc":"2.0","id":5,"error":{"code":-10004,"message":"failed to retrieve API key: key not found"}}
```

### Test 6: HTTP API with glm-5
```bash
curl -sf -X POST -H "Content-Type: application/json" -d '{"jsonrpc":"2.0","id":6,"method":"ai.chat","params":{"model":"glm-5","messages":[{"role":"user","content":"Say hello"}]}}' http://localhost:8080/api
```

**Result**:
```
{"jsonrpc":"2.0","id":6,"error":{"code":-10004,"message":"failed to retrieve API key: key not found"}}
```

**Issue**: API key is not being stored in the keystore.

**Root Cause**: The keystore `store` command failed with "invalid provider" error. The config was updated but the models were added, but the bridge still cannot find the key.

**Solution**: Need to store the API key in the keystore using the correct provider ID.

**Status**: Partial success - Matrix and Bridge work, AI chat blocked by keystore issue

**Recommendation**: 
1. Store API key in keystore using: `armorclaw-bridge add-key --provider zhipu --id zhipu-main --token <your-token> --base-url https://api.z.ai/api/paas/v4 --display-name "Zhipu AI"
2. Or use environment variable `ZAI_API_KEY` and systemd service override
3. Update bash completion to include `zhipu` provider
4. Consider using `openrouter` provider as fallback
