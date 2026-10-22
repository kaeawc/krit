//go:build windows

package daemon

import "testing"

func TestVerifyPeerUnsupportedOnWindows(t *testing.T) {
	err := verifyPeer(nil, "")
	if err == nil || err.Error() != "daemon mode is not supported on Windows" {
		t.Fatalf("verifyPeer error = %v, want unsupported-on-Windows error", err)
	}
}
