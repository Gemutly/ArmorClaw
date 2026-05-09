# ArmorClaw Huh? Wizard Implementation - Updated Plan

> **Status:** IN PROGRESS - Partially Complete
> **Last Updated:** 2026-02-26
> **Version:** 0.3.3

---

## Executive Summary

This document updates the original plan with current implementation status. Many phases are **already complete**, but gaps remain in Go-native error handling and automated testing.

---

## Implementation Status

### Phase 1: Huh? Wizard (Go) - ✅ COMPLETE

**Location:** `bridge/internal/wizard/` (not `bridge/cmd/wizard/` as originally planned)

| Component | Status | File |
|-----------|--------|------|
| Profile Selection | ✅ Done | `profile.go` |
| Quick Start Forms | ✅ Done | `quick.go` |
| Wizard Runner | ✅ Done | `wizard.go` |
| Theme/Styling | ✅ Done | `theme.go` |
| Input Validation | ✅ Done | `validation.go` |
| Terminal Detection | ✅ Done | `wizard.go:checkTerminal()` |
| Non-Interactive Mode | ✅ Done | `wizard.go:tryNonInteractive()` |
| Server Name Auto-Detection | ✅ Done | `wizard.go:detectServerName()` |

**Dependencies Added:**
```go
github.com/charmbracelet/huh v0.8.0
github.com/charmbracelet/lipgloss v1.1.0
github.com/charmbracelet/bubbles v1.0.0 // indirect
```

### Phase 2: Error Handling - 🔄 PARTIAL

| Component | Status | Location |
|-----------|--------|----------|
| Error Codes (INS-XXX) | ✅ Done | `docs/guides/error-catalog.md` |
| Bash Error Handling | ✅ Done | `container-setup.sh` |
| Crash Handler | ✅ Done | `quickstart.sh` in Dockerfile |
| Preflight Checks | ✅ Done | `container-setup.sh:preflight_checks()` |
| Go Error Types | ❌ Missing | `bridge/pkg/setup/errors.go` (not created) |
| Go Error Messages | ❌ Missing | Actionable messages not in Go code |

**What's Missing:**
- `bridge/pkg/setup/errors.go` - Typed errors with actionable messages
- `bridge/pkg/setup/docker.go` - Docker validation in Go
- `bridge/pkg/setup/ssl.go` - SSL generation in Go
- Error messages with root cause + fix instructions

### Phase 3: Dockerfile Integration - ✅ COMPLETE

| Component | Status | Details |
|-----------|--------|---------|
| Wizard Build Stage | ✅ Done | Multi-stage build in Dockerfile.quickstart |
| Binary Location | ✅ Done | `/opt/armorclaw/armorclaw-bridge` |
| Entrypoint Update | ✅ Done | `quickstart.sh` calls Go wizard |
| Fallback Chain | ✅ Done | Go wizard → Bash wizard |

### Phase 4: Wizard Implementation Details - ✅ COMPLETE

| Feature | Status | Implementation |
|---------|--------|----------------|
| Welcome Banner | ✅ Done | `wizard.go:printBanner()` |
| Docker Socket Check | ✅ Done | `quickstart.sh` before wizard |
| Profile Selection | ✅ Done | `profile.go:runProfileForm()` |
| API Key Input (masked) | ✅ Done | `quick.go` with `EchoModePassword` |
| Password Input | ✅ Done | `quick.go` with auto-generate option |
| Server Name Detection | ✅ Done | `wizard.go:detectServerName()` |
| Progress Indication | ✅ Done | `container-setup.sh` progress bar |
| Success Screen | ✅ Done | `container-setup.sh:final_summary()` |

### Phase 5: Integration Testing - ❌ NOT DONE

| Test Scenario | Status |
|---------------|--------|
| First-time setup (empty volumes) | ❌ No automated test |
| Setup with existing config | ❌ No automated test |
| Docker permission error handling | ❌ No automated test |
| Invalid input validation | ❌ No automated test |
| API key injection after wizard | ❌ No automated test |
| Health check timeout | ❌ No automated test |

---

## Remaining Work

### Priority 1: Go Error Types Package

Create `bridge/pkg/setup/errors.go` with typed errors:

```go
package setup

import "fmt"

// SetupError represents a setup failure with actionable guidance.
type SetupError struct {
    Code        string // e.g., "INS-001"
    Title       string // Short title
    Cause       string // Root cause explanation
    Fix         []string // Step-by-step remediation
    DocLink     string // Documentation URL
}

func (e *SetupError) Error() string {
    return fmt.Sprintf("[%s] %s", e.Code, e.Title)
}

// Predefined errors
var (
    ErrDockerSocket = &SetupError{
        Code:    "INS-001",
        Title:   "Docker socket not accessible",
        Cause:   "The Docker socket at /var/run/docker.sock is not accessible.",
        Fix: []string{
            "Run with --user root or add user to docker group",
            "Ensure Docker socket is mounted: -v /var/run/docker.sock:/var/run/docker.sock",
        },
        DocLink: "https://docs.armorclaw.com/errors/ins-001",
    }

    ErrTerminalNotTTY = &SetupError{
        Code:    "INS-002",
        Title:   "Interactive terminal required",
        Cause:   "The TUI wizard requires an interactive terminal (TTY).",
        Fix: []string{
            "Run with -it flags: docker run -it ...",
            "Or use environment variables for non-interactive setup",
        },
        DocLink: "https://docs.armorclaw.com/errors/ins-002",
    }

    // ... more errors
)
```

### Priority 2: Docker Validation in Go

Create `bridge/pkg/setup/docker.go`:

```go
package setup

import (
    "os"
    "syscall"
)

// ValidateDockerSocket checks if Docker socket is accessible.
func ValidateDockerSocket() error {
    socketPath := "/var/run/docker.sock"

    // Check exists
    if _, err := os.Stat(socketPath); os.IsNotExist(err) {
        return ErrDockerSocket
    }

    // Check accessible
    if err := syscall.Access(socketPath, syscall.R_OK|syscall.W_OK); err != nil {
        return ErrDockerPermission
    }

    return nil
}
```

### Priority 3: Integration Tests

Create `bridge/internal/wizard/wizard_test.go`:

```go
package wizard

import (
    "os"
    "testing"
)

func TestNonInteractiveWithAPIKey(t *testing.T) {
    os.Setenv("ARMORCLAW_API_KEY", "sk-test-key-1234567890")
    defer os.Unsetenv("ARMORCLAW_API_KEY")

    result := tryNonInteractive()
    if result == nil {
        t.Fatal("expected non-interactive result with API key set")
    }
    if result.Secrets.APIKey != "sk-test-key-1234567890" {
        t.Errorf("expected API key to be set, got %s", result.Secrets.APIKey)
    }
}

func TestNonInteractiveWithoutAPIKey(t *testing.T) {
    os.Unsetenv("ARMORCLAW_API_KEY")

    result := tryNonInteractive()
    if result != nil {
        t.Fatal("expected nil result without API key")
    }
}

func TestDetectServerName(t *testing.T) {
    name := detectServerName()
    if name == "" {
        t.Error("expected non-empty server name")
    }
}
```

### Priority 4: Move Infrastructure to Go (Optional)

Currently the wizard only collects config, then `container-setup.sh` does:
- Docker Compose stack startup
- SSL certificate generation
- Matrix user registration
- Bridge room creation

**Future work:** Port these to Go for better error handling and progress feedback.

---

## Files to Create

| File | Purpose | Priority |
|------|---------|----------|
| `bridge/pkg/setup/errors.go` | Typed errors with actionable messages | P1 |
| `bridge/pkg/setup/docker.go` | Docker validation in Go | P1 |
| `bridge/pkg/setup/ssl.go` | SSL certificate generation in Go | P2 |
| `bridge/pkg/setup/config.go` | Config file generation in Go | P2 |
| `bridge/internal/wizard/wizard_test.go` | Integration tests | P1 |

---

## Success Criteria (Updated)

- [x] Wizard binary builds successfully
- [x] Profile selection works (Quick/Enterprise)
- [x] API key input with masking
- [x] Password auto-generation
- [x] Server name auto-detection
- [x] Progress indication during setup
- [x] Success screen with connection info
- [x] Error codes documented (INS-XXX)
- [x] Crash handler with log capture
- [x] Preflight checks before setup
- [ ] Go-native error types with actionable messages
- [ ] Automated integration tests
- [ ] Docker validation in Go (not bash)
- [ ] SSL generation in Go (not bash)

---

## Next Steps

1. **Create `bridge/pkg/setup/errors.go`** - Typed errors with fix instructions
2. **Add wizard tests** - `bridge/internal/wizard/wizard_test.go`
3. **Validate Docker in Go** - Move socket check from bash to Go
4. **Improve error messages** - Use typed errors in wizard.Run()

---

**Document Version:** 2.0.0
**Previous Version:** 1.0.0 (original plan)
**Last Updated:** 2026-02-26
