package systemd

import (
	"log/slog"

	"github.com/coreos/go-systemd/v22/daemon"
)

// NotifyReady sends READY=1 to systemd, indicating the service is ready.
// No-op when not running under systemd.
func NotifyReady() {
	if _, err := daemon.SdNotify(false, "READY=1"); err != nil {
		slog.Warn("sd_notify READY=1 failed", "error", err)
	}
}

// NotifyStopping sends STOPPING=1 to systemd, indicating the service is shutting down.
// This MUST be the first action in shutdown to disable the watchdog.
// No-op when not running under systemd.
func NotifyStopping() {
	if _, err := daemon.SdNotify(false, "STOPPING=1"); err != nil {
		slog.Warn("sd_notify STOPPING=1 failed", "error", err)
	}
}

// NotifyWatchdog sends WATCHDOG=1 to systemd to indicate the service is still alive.
// No-op when not running under systemd.
func NotifyWatchdog() {
	if _, err := daemon.SdNotify(false, "WATCHDOG=1"); err != nil {
		slog.Warn("sd_notify WATCHDOG=1 failed", "error", err)
	}
}

// NotifyStatus sends a STATUS= message to systemd for display in systemctl status.
// No-op when not running under systemd.
func NotifyStatus(status string) {
	if _, err := daemon.SdNotify(false, "STATUS="+status); err != nil {
		slog.Warn("sd_notify STATUS failed", "error", err)
	}
}

// IsRunningSystemd returns true if the process is running under systemd
// (i.e., NOTIFY_SOCKET environment variable is set).
func IsRunningSystemd() bool {
	sent, _ := daemon.SdNotify(false, "")
	return sent
}
