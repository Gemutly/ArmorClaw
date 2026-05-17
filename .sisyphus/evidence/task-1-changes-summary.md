# Task 1: Docker Network Isolation - Changes Summary

## Changes Made

### 1. docker-compose.yml
Added armorclaw-isolated network definition:
- Driver: bridge
- Internal: true (no external gateway)
- Subnet: 172.28.0.0/16
- Name: armorclaw-isolated

### 2. docker-compose.bridge.yml
Updated openclaw service:
- Changed from network_mode: none to networks: [armorclaw-isolated]
- Added HTTP_PROXY=http://go-bridge:3128 environment variable
- Added NO_PROXY=localhost,127.0.0.1,go-bridge environment variable
- Added armorclaw-isolated as external network

## Expected Verification Results

### Network Inspection
```bash
docker network inspect armorclaw-isolated | jq '.[0].Internal'
# Expected: true

docker network inspect armorclaw-isolated | jq '.[0].IPAM.Config'
# Expected: No gateway, subnet 172.28.0.0/16
```

### OpenClaw Network Isolation
```bash
docker exec armorclaw-openclaw curl -s --max-time 5 https://google.com
# Expected: Connection timeout or failure (no external access)
```

### HTTP_PROXY Environment Variable
```bash
docker exec armorclaw-openclaw env | grep HTTP_PROXY
# Expected: HTTP_PROXY=http://go-bridge:3128
```

## Security Benefits
- OpenClaw cannot reach external internet directly (no gateway)
- OpenClaw MUST use HTTP_PROXY (go-bridge:3128) for LLM API calls
- Prevents direct exfiltration attempts
- All external traffic flows through Go Bridge proxy for auditing

## YAML Validation
- docker-compose.yml: OK
- docker-compose.bridge.yml: OK
