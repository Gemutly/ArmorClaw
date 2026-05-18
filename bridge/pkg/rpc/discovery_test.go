package rpc

import (
	"testing"

	"github.com/armorclaw/bridge/pkg/voice"
)

// TestDiscovery_FlagIndependence verifies that ALL RPC methods are always
// registered regardless of feature flag state. Feature flags control handler
// behaviour (return feature-disabled error), not registration — every handler
// is always present in the map so clients get a proper "feature disabled"
// response instead of "method not found".
func TestDiscovery_FlagIndependence(t *testing.T) {
	expectedMethods := TotalRPCMethods()

	// Feature-flagged method groups that MUST always be registered.
	featureMethods := []string{
		// keystore (7 methods) — gated by zeroTrustKS
		"keystore.unseal",
		"keystore.sealed",
		"keystore.seal",
		"keystore.extend_session",
		"keystore.session_status",
		"keystore.list_keys",
		"keystore.delete_key",
		// voice (3 methods) — gated by voiceMgr != nil
		"voice.start_session",
		"voice.stop_session",
		"voice.status",
		// e2ee backup (3 methods) — gated by e2eeBackupEnabled
		"e2ee.create_backup",
		"e2ee.delete_backup",
		"e2ee.backup_exists",
	}

	cases := []struct {
		name        string
		zeroTrustKS bool
		voiceMgr    bool
		e2eeBackup  bool
		replayFlag  bool
	}{
		{"all_flags_off", false, false, false, false},
		{"keystore_flag_only", true, false, false, false},
		{"voice_flag_only", false, true, false, false},
		{"e2ee_flag_only", false, false, true, false},
		{"replay_flag_only", false, false, false, true},
		{"all_flags_on", true, true, true, true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s := &Server{
				zeroTrustKS:       tc.zeroTrustKS,
				e2eeBackupEnabled: tc.e2eeBackup,
			}
			if tc.voiceMgr {
				s.voiceMgr = &voice.Manager{} // non-nil signals voice enabled
			}
			s.replayFlags.MultiTabReplay = tc.replayFlag
			s.registerHandlers()

			if count := len(s.handlers); count != expectedMethods {
				t.Errorf("expected %d registered methods, got %d", expectedMethods, count)
			}

			for _, method := range featureMethods {
				if _, ok := s.handlers[method]; !ok {
					t.Errorf("method %q missing from handler map (flags: %+v)", method, tc)
				}
			}
		})
	}
}
