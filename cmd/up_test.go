package cmd

import "testing"

func Test_remoteEnabled(t *testing.T) {
	up := UpContext{}

	if up.remoteModeEnabled() {
		t.Errorf("default should be remote disabled")
	}

	up.remotePort = 22000
	if !up.remoteModeEnabled() {
		t.Errorf("remote should be enabled after adding a port")
	}
}
