package daemon

// Socket location. NOTE: the daemon package is Unix-only today (lock.go
// uses syscall.Flock without build tags), so Unix-only syscalls are used
// freely here.

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"syscall"

	"github.com/kaeawc/krit/internal/fsutil"
)

// maxSocketPath is the longest unix socket path used before falling back to
// a short temp-dir path. sun_path holds 108 bytes on Linux and 104 on macOS
// (including the NUL), and bind(2) fails with EINVAL beyond that.
const maxSocketPath = 100

// ExitAlreadyRunning is the exit status of a daemon that finds another one
// already serving the same repository (the repo lock is held).
const ExitAlreadyRunning = 2

const (
	pointerFileName = "daemon.sock.path"
	pointerMaxSize  = 4096
)

// systemTmp is the last-resort fallback base; a var so tests can redirect it.
var systemTmp = "/tmp"

var fallbackSockName = regexp.MustCompile(`^[0-9a-f]{16}\.sock$`)

// NormalizeRoot makes root absolute and resolves symlinks (falling back to
// the absolute form), so a relative root and its absolute form map to the
// same socket.
func NormalizeRoot(root string) string {
	abs, err := filepath.Abs(root)
	if err != nil {
		return filepath.Clean(root)
	}
	if resolved, err := filepath.EvalSymlinks(abs); err == nil {
		return resolved
	}
	return abs
}

// ConventionalSocketPath is <root>/.krit/daemon.sock for the normalized root.
func ConventionalSocketPath(root string) string {
	return filepath.Join(NormalizeRoot(root), ".krit", "daemon.sock")
}

// FallbackSocketName is the per-root socket file name used on the fallback
// path: 16 hex characters of sha256(normalized root).
func FallbackSocketName(root string) string {
	sum := sha256.Sum256([]byte(NormalizeRoot(root)))
	return hex.EncodeToString(sum[:8]) + ".sock"
}

// IsFallbackSocket reports whether path has the shape of a socket created by
// the long-root fallback (<private dir>/<16 hex>.sock under a dir named
// "krit" or "krit-<uid>").
func IsFallbackSocket(path string) bool {
	if !fallbackSockName.MatchString(filepath.Base(path)) {
		return false
	}
	d := filepath.Base(filepath.Dir(path))
	return d == "krit" || strings.HasPrefix(d, "krit-")
}

// fallbackSockets lists, in preference order, the fallback socket paths for
// root whose full length fits a unix socket: $XDG_RUNTIME_DIR/krit,
// $TMPDIR/krit-<uid>, /tmp/krit-<uid>.
func fallbackSockets(root string) (paths []string, tooLong []error) {
	name := FallbackSocketName(root)
	uid := os.Getuid()
	var dirs []string
	if x := os.Getenv("XDG_RUNTIME_DIR"); x != "" {
		dirs = append(dirs, filepath.Join(x, "krit"))
	}
	dirs = append(dirs,
		filepath.Join(os.TempDir(), fmt.Sprintf("krit-%d", uid)),
		filepath.Join(systemTmp, fmt.Sprintf("krit-%d", uid)),
	)
	seen := map[string]bool{}
	for _, d := range dirs {
		// A relative TMPDIR/XDG_RUNTIME_DIR would make the socket path
		// depend on the process's working directory (which the daemon
		// changes per request), and pointer files only hold absolute paths.
		if !filepath.IsAbs(d) {
			tooLong = append(tooLong, fmt.Errorf("%s: not an absolute path", d))
			continue
		}
		sock := filepath.Join(d, name)
		if seen[sock] {
			continue
		}
		seen[sock] = true
		if len(sock) > maxSocketPath {
			tooLong = append(tooLong, fmt.Errorf("%s: path too long for a unix socket", sock))
			continue
		}
		paths = append(paths, sock)
	}
	return paths, tooLong
}

// DefaultSocketPath is the socket path a daemon for root is expected to use,
// derived without touching the filesystem beyond read-only checks: the
// conventional <root>/.krit/daemon.sock, or, when that is longer than a unix
// socket allows (deeply nested checkouts, long CI workspace names), the
// first fallback socket whose directory is usable (see ResolveSocketPath).
// Safe for lookups (status, Discover): it never creates directories.
// Callers about to bind or spawn should use ResolveSocketPath to get a clear
// error when no fallback directory is usable.
func DefaultSocketPath(root string) string {
	conv := ConventionalSocketPath(root)
	if len(conv) <= maxSocketPath {
		return conv
	}
	paths, _ := fallbackSockets(root)
	for _, sock := range paths {
		if fsutil.CheckPrivateDir(filepath.Dir(sock), true) == nil {
			return sock
		}
	}
	return conv
}

// ResolveSocketPath is DefaultSocketPath for the bind and spawn paths: it
// creates (or validates and tightens) the private fallback directory, and
// errors when the conventional path is too long and no fallback is usable,
// instead of letting bind fail with a bare EINVAL.
func ResolveSocketPath(root string) (string, error) {
	conv := ConventionalSocketPath(root)
	if len(conv) <= maxSocketPath {
		return conv, nil
	}
	paths, errs := fallbackSockets(root)
	for _, sock := range paths {
		if err := fsutil.EnsurePrivateDir(filepath.Dir(sock), true); err != nil {
			errs = append(errs, err)
			continue
		}
		return sock, nil
	}
	return "", fmt.Errorf("daemon: socket path %s is too long and no usable fallback directory exists: %w", conv, errors.Join(errs...))
}

// PointerPath is where a daemon bound to a fallback socket records the real
// socket path: <root>/.krit/daemon.sock.path.
func PointerPath(root string) string {
	return filepath.Join(NormalizeRoot(root), ".krit", pointerFileName)
}

// WriteSocketPointer atomically (temp file + rename) records sock in the
// pointer file at pointer, mode 0600.
func WriteSocketPointer(pointer, sock string) error {
	dir := filepath.Dir(pointer)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, pointerFileName+".tmp-*")
	if err != nil {
		return err
	}
	name := tmp.Name()
	_, werr := tmp.WriteString(sock + "\n")
	cerr := tmp.Close()
	if werr == nil {
		werr = cerr
	}
	if werr == nil {
		werr = os.Chmod(name, 0o600)
	}
	if werr == nil {
		werr = os.Rename(name, pointer)
	}
	if werr != nil {
		_ = os.Remove(name)
	}
	return werr
}

// readPointerFile returns the trimmed content of a pointer file that is a
// regular file (not a symlink), owned by us and at most pointerMaxSize bytes.
func readPointerFile(pointer string) (string, bool) {
	fi, err := os.Lstat(pointer)
	if err != nil || !fi.Mode().IsRegular() || fi.Size() > pointerMaxSize {
		return "", false
	}
	if st, ok := fi.Sys().(*syscall.Stat_t); !ok || int(st.Uid) != os.Getuid() {
		return "", false
	}
	data, err := os.ReadFile(pointer)
	if err != nil {
		return "", false
	}
	return strings.TrimSpace(string(data)), true
}

// RemoveSocketPointer removes the pointer file if it still names sock (so a
// successor daemon's pointer is left alone). A missing, unsafe (symlink,
// foreign-owned, oversize) or differing pointer is left untouched.
func RemoveSocketPointer(pointer, sock string) error {
	got, ok := readPointerFile(pointer)
	if !ok || got != sock {
		return nil
	}
	if err := os.Remove(pointer); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}

// ClearSocketPointer unconditionally removes root's pointer file (a symlink
// is removed itself, never followed). Daemons on the conventional socket call
// it at startup so a stale pointer cannot steer clients elsewhere.
func ClearSocketPointer(root string) error {
	if err := os.Remove(PointerPath(root)); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}

// readSocketPointer returns the socket recorded for root. It is honored only
// when the conventional path is too long to bind (so a short-root daemon on
// .krit/daemon.sock is never hidden), and only for the exact per-root name,
// in a krit-named dir, as an absolute path that fits.
func readSocketPointer(root string) (string, bool) {
	if len(ConventionalSocketPath(root)) <= maxSocketPath {
		return "", false
	}
	p, ok := readPointerFile(PointerPath(root))
	if !ok {
		return "", false
	}
	if !filepath.IsAbs(p) || len(p) > maxSocketPath || !IsFallbackSocket(p) || filepath.Base(p) != FallbackSocketName(root) {
		return "", false
	}
	return p, true
}

// SocketCandidates lists where a daemon for root may be listening, best
// first: the pointer file's target (when valid), then the derived path.
func SocketCandidates(root string) []string {
	derived := DefaultSocketPath(root)
	if p, ok := readSocketPointer(root); ok && p != derived {
		return []string{p, derived}
	}
	return []string{derived}
}

// LocateSocketPath is what clients use to find a daemon: the first of
// SocketCandidates that answers, else the derived path. The pointer makes
// lookups robust to a different TMPDIR/XDG_RUNTIME_DIR in the daemon's
// environment; falling through to the derived path means a stale pointer
// can never hide a live daemon. The server never uses it.
func LocateSocketPath(root string) string {
	cands := SocketCandidates(root)
	for _, c := range cands {
		if Available(c) {
			return c
		}
	}
	return cands[len(cands)-1]
}
