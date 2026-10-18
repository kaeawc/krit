//go:build windows

package daemon

import (
	"fmt"
	"os"
)

func ensurePrivateDir(dir string) error {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("daemon: create socket dir %s: %w", dir, err)
	}
	return nil
}
