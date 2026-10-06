package api

import (
	"mirai/internal/nodeapi"
	"mirai/internal/panel/updates"
	"mirai/internal/release"
	"testing"
)

func TestNodeUpdateAvailableOnlyForOlderNodes(t *testing.T) {
	state := updates.State{Current: "0.5.2", Latest: &release.Manifest{Version: "0.5.2", Native: map[string]release.Asset{"x86_64": {}}}}
	for _, tc := range []struct {
		version   string
		supported bool
		want      bool
	}{{"0.5.1", true, true}, {"0.5.2", true, false}, {"0.5.3", true, false}, {"0.5.1", false, false}, {"", true, false}} {
		if got := nodeUpdateAvailable(state, nodeapi.UpdateStatus{Version: tc.version, Supported: tc.supported}); got != tc.want {
			t.Fatalf("%s: %v", tc.version, got)
		}
	}
	state.Found.Unreachable = true
	if nodeUpdateAvailable(state, nodeapi.UpdateStatus{Version: "0.5.1", Supported: true}) {
		t.Fatal("unreachable release offered")
	}
	state.Latest = nil
	if nodeUpdateAvailable(state, nodeapi.UpdateStatus{Version: "0.5.1", Supported: true}) {
		t.Fatal("unknown release offered")
	}
}
