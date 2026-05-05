package systemd

import (
	"log/slog"

	"github.com/coreos/go-systemd/v22/daemon"
)

// NotifyReady sends READY=1 to systemd, indicating the service is ready.
func NotifyReady() {
	if _, err := daemon.SdNotify(false, "READY=1"); err != nil {
		slog.Warn("sd_notify READY=1 failed", "error", err)
	}
}

// NotifyStopping sends STOPPING=1 to systemd, disabling the watchdog during shutdown.
func NotifyStopping() {
	if _, err := daemon.SdNotify(false, "STOPPING=1"); err != nil {
		slog.Warn("sd_notify STOPPING=1 failed", "error", err)
	}
}

// NotifyWatchdog sends WATCHDOG=1 to systemd, keeping the watchdog alive.
func NotifyWatchdog() {
	if _, err := daemon.SdNotify(false, "WATCHDOG=1"); err != nil {
		slog.Warn("sd_notify WATCHDOG=1 failed", "error", err)
	}
}

// NotifyStatus sends a STATUS= message to systemd.
func NotifyStatus(status string) {
	if _, err := daemon.SdNotify(false, "STATUS="+status); err != nil {
		slog.Warn("sd_notify STATUS failed", "error", err, "status", status)
	}
}

// IsRunningSystemd checks if the process is running under systemd
// by checking for the NOTIFY_SOCKET environment variable.
func IsRunningSystemd() bool {
	sent, _ := daemon.SdNotify(false, "")
	return sent
}
