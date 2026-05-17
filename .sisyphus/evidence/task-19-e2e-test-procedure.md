# Task 19: E2E Test with Real Containers

## Status: CONFIGURATION READY (Requires docker-compose)

## Prerequisites

1. Install docker-compose:
   ```bash
   # Ubuntu/Debian
   sudo apt-get install docker-compose-plugin
   # Or standalone
   sudo curl -L "https://github.com/docker/compose/releases/latest/download/docker-compose-$(uname -s)-$(uname -m)" -o /usr/local/bin/docker-compose
   sudo chmod +x /usr/local/bin/docker-compose
   ```

2. Verify Docker is running:
   ```bash
   docker info
   ```

## E2E Test Procedure

### Step 1: Start Voice AI Sidecars

```bash
cd /home/mink/src/armorclaw-omo
docker compose -f deploy/ai/docker-compose.voice.yml up -d
```

### Step 2: Wait for Services to be Healthy

```bash
# Check service health
docker compose -f deploy/ai/docker-compose.voice.yml ps

# Wait for all services to report "healthy"
# Expected: 3 services running (whisper, piper, silero-vad)
```

### Step 3: Verify Service Endpoints

```bash
# Test Whisper ASR (STT) - Port 9001
curl -s http://localhost:9001/health || echo "Whisper not ready"

# Test Piper TTS - Port 9002
curl -s http://localhost:9002/health || echo "Piper not ready"

# Test Silero VAD - Port 9003
curl -s http://localhost:9003/health || echo "Silero VAD not ready"
```

### Step 4: Run Bridge with Voice Enabled

```bash
cd /home/mink/src/armorclaw-omo/bridge
export ARMORCLAW_VOICE_STT_URL=http://localhost:9001
export ARMORCLAW_VOICE_TTS_URL=http://localhost:9002
export ARMORCLAW_VOICE_VAD_URL=http://localhost:9003
export ARMORCLAW_VOICE_HITL_TIMEOUT=30s

go run ./cmd/bridge/...
```

### Step 5: Test Voice Pipeline

1. Initiate a Matrix call to the bridge
2. Speak test phrase: "Hello, this is a test"
3. Verify:
   - VAD detects speech
   - STT transcribes audio
   - TTS generates response
   - Audio plays back

### Step 6: Test HITL Interlock

1. During call, attempt a sensitive action (e.g., PII access)
2. Verify:
   - Pipeline pauses
   - Matrix notification sent with ✅/❌ reactions
   - Timeout auto-rejects after 30s

### Step 7: Test Skill Gate

1. During call, attempt to invoke MCP skill
2. Verify:
   - Error returned: "Skill mcp is disabled during voice calls"
   - Non-sensitive skills allowed

### Step 8: Cleanup

```bash
docker compose -f deploy/ai/docker-compose.voice.yml down
```

## Expected Results

| Test | Expected | Status |
|------|----------|--------|
| Whisper health check | 200 OK | Pending E2E |
| Piper health check | 200 OK | Pending E2E |
| Silero VAD health check | 200 OK | Pending E2E |
| STT transcription | "Hello, this is a test" | Pending E2E |
| TTS synthesis | Audio output | Pending E2E |
| VAD detection | Speech detected | Pending E2E |
| HITL approval | Pause → Approve → Resume | Pending E2E |
| Skill gate blocking | MCP blocked | Pending E2E |

## Notes

- E2E testing requires docker-compose which is not available in current environment
- All unit tests and integration tests with mock servers pass
- Configuration validated: docker-compose.voice.yml is syntactically correct
- Service ports: Whisper (9001), Piper (9002), Silero VAD (9003)

## Alternative: Manual Container Start

If docker-compose is unavailable, containers can be started individually:

```bash
# Whisper ASR
docker run -d --name armorclaw-whisper -p 9001:9000 \
  -e ASR_MODEL=base ahmetoner/whisper-asr-webservice:latest

# Piper TTS
docker run -d --name armorclaw-piper -p 9002:5000 \
  -e PIPER_VOICE=en_US-lessac-medium rhasspy/piper:latest

# Silero VAD
docker run -d --name armorclaw-silero-vad -p 9003:5001 \
  pbz1vke/silero-vad-server:latest
```
