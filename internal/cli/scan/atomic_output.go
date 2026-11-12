package scan

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/kaeawc/krit/internal/fsutil"
)

// atomicOutput stages a -o report in a temp file next to the target. Commit
// renames it into place, so a crash or failed run never leaves a truncated
// report where a previous good one stood.
type atomicOutput struct {
	file *os.File
	path string
}

// openAtomicOutput returns nil when path is not a plain file target (a device,
// FIFO or symlink), which the caller then opens directly. An existing report
// keeps its mode; a new one gets 0644 less the umask, as os.Create would.
func openAtomicOutput(path string) *atomicOutput {
	var existing os.FileInfo
	if info, err := os.Lstat(path); err == nil {
		if !info.Mode().IsRegular() {
			return nil
		}
		existing = info
	} else if !os.IsNotExist(err) {
		return nil
	}
	tmp, err := fsutil.CreateTempMode(filepath.Dir(path), "."+filepath.Base(path)+".tmp-*", 0o644)
	if err != nil {
		return nil // unwritable directory: the direct open reports the error
	}
	if existing != nil {
		if err := tmp.Chmod(existing.Mode().Perm()); err != nil {
			discard(tmp)
			return nil
		}
	}
	return &atomicOutput{file: tmp, path: path}
}

func discard(f *os.File) {
	_ = f.Close()
	_ = os.Remove(f.Name())
}

// Finish publishes the staged report when the run produced a complete one
// (exitCode != 2) and discards it otherwise.
func (a *atomicOutput) Finish(exitCode int) error {
	if exitCode == 2 {
		discard(a.file)
		return nil
	}
	if err := a.file.Close(); err != nil {
		_ = os.Remove(a.file.Name())
		return err
	}
	if err := os.Rename(a.file.Name(), a.path); err != nil {
		_ = os.Remove(a.file.Name())
		return fmt.Errorf("rename report: %w", err)
	}
	return nil
}
