package systemd

import (
	"os"
	"testing"
)

func TestNotifyWithoutSystemd(t *testing.T) {
	os.Unsetenv("NOTIFY_SOCKET")

	NotifyReady()
	NotifyStopping()
	NotifyWatchdog()
	NotifyStatus("test status")

	if IsRunningSystemd() {
		t.Error("IsRunningSystemd should return false when NOTIFY_SOCKET not set")
	}
}
