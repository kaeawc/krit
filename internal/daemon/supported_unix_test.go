//go:build unix

package daemon

import "testing"

func TestSupportedOnUnix(t *testing.T) {
	if !Supported() {
		t.Fatal("Supported() = false on a unix platform")
	}
}
