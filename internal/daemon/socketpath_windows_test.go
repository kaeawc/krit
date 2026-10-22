//go:build windows

package daemon

import "testing"

func TestOwnedByCurrentUserFalseOnWindows(t *testing.T) {
	if ownedByCurrentUser(nil) {
		t.Fatal("ownedByCurrentUser = true; ownership cannot be verified on Windows")
	}
}
