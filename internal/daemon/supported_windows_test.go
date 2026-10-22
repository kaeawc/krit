//go:build windows

package daemon

import "testing"

func TestSupportedFalseOnWindows(t *testing.T) {
	if Supported() {
		t.Fatal("Supported() = true on Windows")
	}
}
