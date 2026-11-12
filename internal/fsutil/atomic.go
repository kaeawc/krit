package fsutil

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"math/rand/v2"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// WriteFileAtomic writes data to path with perm, using tempfile + fsync + rename
// so a concurrent reader never sees a truncated file and a crash mid-write never
// leaves a torn file on disk.
//
// Parent directories must already exist — callers that want MkdirAll should call
// it themselves, to keep this helper one syscall wide.
//
// The tempfile is created in the same directory as path (same filesystem =
// rename is atomic on POSIX) with prefix derived from filepath.Base(path).
// On any error after tempfile creation, the tempfile is removed best-effort.
//
// Like os.WriteFile, perm is subject to the process umask, and a symlink at
// path is written through: its final target is replaced, the link is kept.
func WriteFileAtomic(path string, data []byte, perm os.FileMode) error {
	return writeAtomic(path, perm, false, func(f *os.File) error {
		_, err := f.Write(data)
		return err
	})
}

// WriteFileAtomicKeepMode is WriteFileAtomic for rewriting a file in place. An existing file
// keeps its exact permission bits; a new file gets defaultPerm less the umask.
func WriteFileAtomicKeepMode(path string, data []byte, defaultPerm os.FileMode) error {
	write := func(f *os.File) error {
		_, err := f.Write(data)
		return err
	}
	if info, err := os.Stat(path); err == nil {
		return writeAtomic(path, info.Mode().Perm(), true, write)
	}
	return writeAtomic(path, defaultPerm, false, write)
}

// WriteFileAtomicStream is like WriteFileAtomic but streams into a caller-provided
// callback. Used for gob/json encoders that want to write directly into the
// tempfile without materialising the full payload.
//
//	err := WriteFileAtomicStream(path, perm, func(w io.Writer) error {
//	    return gob.NewEncoder(w).Encode(v)
//	})
//
// The writer is buffered; flush is handled by the helper. fsync runs after
// flush and before rename.
func WriteFileAtomicStream(path string, perm os.FileMode, write func(io.Writer) error) error {
	return writeAtomic(path, perm, false, func(f *os.File) error {
		bw := bufio.NewWriter(f)
		if err := write(bw); err != nil {
			return err
		}
		return bw.Flush()
	})
}

// writeAtomic is the shared tempfile + fsync + rename core. payload decides
// how bytes reach the file handle (direct write vs. buffered stream). exact
// sets perm regardless of the umask, for preserving an existing file's mode.
func writeAtomic(path string, perm os.FileMode, exact bool, payload func(*os.File) error) error {
	path, err := symlinkTarget(path)
	if err != nil {
		return fmt.Errorf("resolve symlink: %w", err)
	}
	dir := filepath.Dir(path)
	base := filepath.Base(path)

	tmp, err := CreateTempMode(dir, "."+base+".tmp-*", perm)
	if err != nil {
		return fmt.Errorf("write tempfile: %w", err)
	}
	tmpName := tmp.Name()

	cleanup := func() {
		tmp.Close()
		os.Remove(tmpName)
	}

	if exact {
		if err := tmp.Chmod(perm); err != nil {
			cleanup()
			return fmt.Errorf("write tempfile: %w", err)
		}
	}

	if err := payload(tmp); err != nil {
		cleanup()
		return fmt.Errorf("write tempfile: %w", err)
	}

	if err := tmp.Sync(); err != nil {
		cleanup()
		return fmt.Errorf("sync tempfile: %w", err)
	}

	if err := tmp.Close(); err != nil {
		os.Remove(tmpName)
		return fmt.Errorf("close tempfile: %w", err)
	}

	if err := os.Rename(tmpName, path); err != nil {
		os.Remove(tmpName)
		return fmt.Errorf("rename tempfile: %w", err)
	}

	// fsync the parent directory so the rename's dirent is durable. Without
	// this, a crash between rename and the kernel's dirent flush can lose the
	// rename on POSIX filesystems, leaving the file missing or pointing at the
	// pre-rename state. Best-effort: directory fsync is a no-op or rejected
	// (EINVAL/ENOTSUP) on some platforms (notably Windows), and the rename
	// itself already succeeded, so we don't propagate errors from this step.
	syncDir(dir)

	return nil
}

// maxSymlinkHops bounds symlink resolution, matching the common ELOOP limit.
const maxSymlinkHops = 40

// symlinkTarget follows a symlink at path to the file it finally names, so a
// rename replaces that file rather than the link. The target need not exist,
// but its directory must.
func symlinkTarget(path string) (string, error) {
	for range maxSymlinkHops {
		info, err := os.Lstat(path)
		if err != nil {
			// A missing path is created by the rename; other errors surface there.
			return path, nil //nolint:nilerr // the write itself reports the error
		}
		if info.Mode()&os.ModeSymlink == 0 {
			return path, nil
		}
		link, err := os.Readlink(path)
		if err != nil {
			return "", err
		}
		if !filepath.IsAbs(link) {
			// No filepath.Join: cleaning would collapse "alias/.." lexically,
			// while the kernel resolves alias first.
			link = filepath.Dir(path) + string(filepath.Separator) + link
		}
		if path, err = resolveParent(link); err != nil {
			return "", err
		}
	}
	return "", fmt.Errorf("%s: too many levels of symbolic links", path)
}

// resolveParent resolves every symlink in the directory part of path the way
// the filesystem does, leaving only the final element unresolved.
func resolveParent(path string) (string, error) {
	i := len(path) - 1
	for i >= 0 && !os.IsPathSeparator(path[i]) {
		i--
	}
	dir, base := path[:i+1], path[i+1:]
	if base == "" || base == "." || base == ".." {
		return "", fmt.Errorf("%s: symlink target is not a file name", path)
	}
	if dir == "" {
		dir = "."
	}
	resolved, err := filepath.EvalSymlinks(dir)
	if err != nil {
		return "", fmt.Errorf("symlink target directory: %w", err)
	}
	return filepath.Join(resolved, base), nil
}

// CreateTempMode is os.CreateTemp with a caller-chosen mode instead of 0600.
// The mode is subject to the umask, as with os.OpenFile.
func CreateTempMode(dir, pattern string, perm os.FileMode) (*os.File, error) {
	prefix, suffix := pattern, ""
	if i := strings.LastIndex(pattern, "*"); i >= 0 {
		prefix, suffix = pattern[:i], pattern[i+1:]
	}
	for range 10000 {
		name := filepath.Join(dir, prefix+strconv.FormatUint(uint64(rand.Uint32()), 10)+suffix)
		f, err := os.OpenFile(name, os.O_RDWR|os.O_CREATE|os.O_EXCL, perm)
		if errors.Is(err, os.ErrExist) {
			continue
		}
		return f, err
	}
	return nil, fmt.Errorf("create temp file in %s: too many collisions", dir)
}
