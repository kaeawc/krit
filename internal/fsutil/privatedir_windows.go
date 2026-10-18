//go:build windows

package fsutil

import (
	"fmt"
	"os"
)

// EnsurePrivateDir creates dir; Windows ACLs are not inspected here.
func EnsurePrivateDir(dir string, _ bool) error {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("create private dir %s: %w", dir, err)
	}
	return nil
}
