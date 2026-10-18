//go:build !windows

package fsutil

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"syscall"
)

// EnsurePrivateDir creates dir with mode 0700 or validates an existing one,
// so a predictable path in a shared directory (for example
// $TMPDIR/krit-<uid>) cannot be pre-created by another local user.
//
// An existing dir must be a real directory (opened with O_NOFOLLOW, so a
// symlink is refused) owned by the current user; a looser mode on a dir we
// own is tightened to 0700 through the open descriptor, not the path. With
// checkParent, the parent directory must also be owned by us or root and
// must not be writable by group or others unless it has the sticky bit (as
// /tmp does), because otherwise another user could rename our dir away and
// plant their own.
func EnsurePrivateDir(dir string, checkParent bool) error {
	if checkParent {
		if err := checkParentDir(filepath.Dir(dir)); err != nil {
			return err
		}
	}
	if err := os.Mkdir(dir, 0o700); err != nil && !errors.Is(err, fs.ErrExist) {
		if !errors.Is(err, fs.ErrNotExist) {
			return fmt.Errorf("create private dir %s: %w", dir, err)
		}
		if perr := os.MkdirAll(filepath.Dir(dir), 0o700); perr != nil {
			return fmt.Errorf("create private dir %s: %w", dir, perr)
		}
		if err := os.Mkdir(dir, 0o700); err != nil && !errors.Is(err, fs.ErrExist) {
			return fmt.Errorf("create private dir %s: %w", dir, err)
		}
	}
	f, err := os.OpenFile(dir, os.O_RDONLY|syscall.O_NOFOLLOW|syscall.O_DIRECTORY, 0)
	if err != nil {
		return fmt.Errorf("%s is not a real directory (symlink or file?); refusing to use it: %w", dir, err)
	}
	defer f.Close()
	fi, err := f.Stat()
	if err != nil {
		return fmt.Errorf("stat private dir %s: %w", dir, err)
	}
	if err := checkOwner(dir, fi); err != nil {
		return err
	}
	if fi.Mode().Perm() != 0o700 {
		if err := f.Chmod(0o700); err != nil {
			return fmt.Errorf("%s has mode %o and could not be tightened to 0700: %w", dir, fi.Mode().Perm(), err)
		}
	}
	return nil
}

// CheckPrivateDir is the read-only counterpart of EnsurePrivateDir: it
// reports whether EnsurePrivateDir could use dir, without creating or
// changing anything. A missing dir passes (it would be created by us).
func CheckPrivateDir(dir string, checkParent bool) error {
	if checkParent {
		if err := checkParentDir(filepath.Dir(dir)); err != nil {
			return err
		}
	}
	fi, err := os.Lstat(dir)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil
		}
		return fmt.Errorf("stat %s: %w", dir, err)
	}
	if !fi.IsDir() {
		return fmt.Errorf("%s is not a real directory (symlink or file); refusing to use it", dir)
	}
	return checkOwner(dir, fi)
}

func checkOwner(dir string, fi os.FileInfo) error {
	st, ok := fi.Sys().(*syscall.Stat_t)
	if !ok || int(st.Uid) != os.Getuid() {
		return fmt.Errorf("%s is not owned by the current user; refusing to use it", dir)
	}
	return nil
}

func checkParentDir(parent string) error {
	fi, err := os.Stat(parent) // follow symlinks: /tmp may be a link
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil // will be created by us, 0700
		}
		return fmt.Errorf("stat %s: %w", parent, err)
	}
	if st, ok := fi.Sys().(*syscall.Stat_t); ok && int(st.Uid) != os.Getuid() && st.Uid != 0 {
		return fmt.Errorf("%s is owned by uid %d, neither us nor root; refusing to place a private dir in it", parent, st.Uid)
	}
	if fi.Mode().Perm()&0o022 != 0 && fi.Mode()&os.ModeSticky == 0 {
		return fmt.Errorf("%s is writable by other users without the sticky bit; refusing to place a private dir in it", parent)
	}
	return nil
}
