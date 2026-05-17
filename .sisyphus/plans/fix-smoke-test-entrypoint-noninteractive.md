# Fix: Smoke Test Entrypoint Non-Interactive Mode

## TL;DR

> **Quick Summary**: Fix CI smoke test by making entrypoint detect CI environment and run non-interactively.
> 
> **Deliverables**: 
> - Modified `Dockerfile.quickstart` entrypoint wrapper to detect CI and pass `--non-interactive`
> 
> **Estimated Effort**: Quick (5 lines in Dockerfile)
> **Parallel Execution**: NO - single file change
> **Critical Path**: Direct fix

---

## Context

### Current Error
```
Add an API key now? [N/y]: /opt/armorclaw/quickstart.sh: line 104: /dev/tty: No such device or address
...
✗ Bridge not running
Error: Process completed with exit code 1.
```

### Root Cause
The entrypoint wrapper calls `quickstart.sh` without `--non-interactive` flag, causing:
1. Interactive prompts (API key, etc.) fail with "No such device or address"
2. Bridge status check fails

### Current Entrypoint Wrapper
```bash
#!/bin/bash
source /opt/armorclaw/generate-keystore-key.sh
exec /opt/armorclaw/quickstart.sh "$@"
```

### Fix Approach
Detect CI environment and pass `--non-interactive` flag.

---

## TODOs

- [x] 1. Update Dockerfile.quickstart entrypoint wrapper

  **File**: `Dockerfile.quickstart` lines 212-223

  **Current Code**:
  ```dockerfile
  # Entry wrapper: generate keystore key before main entrypoint
  RUN cat > /opt/armorclaw/entrypoint-wrapper.sh << 'EOF'
  #!/bin/bash
  # Source keystore key generation (persists key to keystore.key)
  source /opt/armorclaw/generate-keystore-key.sh
  # Run main entrypoint
  exec /opt/armorclaw/quickstart.sh "$@"
  EOF

  RUN chmod +x /opt/armorclaw/entrypoint-wrapper.sh

  ENTRYPOINT ["/opt/armorclaw/entrypoint-wrapper.sh"]
  ```

  **Fixed Code**:
  ```dockerfile
  # Entry wrapper: generate keystore key before main entrypoint
  RUN cat > /opt/armorclaw/entrypoint-wrapper.sh << 'EOF'
  #!/bin/bash
  # Source keystore key generation (persists key to keystore.key)
  source /opt/armorclaw/generate-keystore-key.sh

  # Detect CI environment and run non-interactively
  if [ "${GITHUB_ACTIONS:-false}" = "true" ] || [ "${CI:-false}" = "true" ] || [ "${ARMORCLAW_SKIP_DOCKER_CHECK:-false}" = "true" ]; then
      exec /opt/armorclaw/quickstart.sh --non-interactive "$@"
  fi

  # Run main entrypoint
  exec /opt/armorclaw/quickstart.sh "$@"
  EOF

  RUN chmod +x /opt/armorclaw/entrypoint-wrapper.sh

  ENTRYPOINT ["/opt/armorclaw/entrypoint-wrapper.sh"]
  ```

  **Commit**: YES
  - Message: `fix(ci): detect CI environment in entrypoint for non-interactive mode`
  - Files: `Dockerfile.quickstart`

---

## Success Criteria

- [x] Dockerfile.quickstart updated
- [x] Commit pushed
- [ ] CI smoke test passes (pending user push)
