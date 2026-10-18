package daemon

import (
	"bufio"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"syscall"
	"time"

	"github.com/kaeawc/krit/internal/fsutil"
)

// maxSocketPath is the longest unix socket path DefaultSocketPath will
// return before falling back to a short temp-dir path. sun_path holds 108
// bytes on Linux and 104 on macOS (including the NUL), and bind(2) fails
// with EINVAL beyond that.
const maxSocketPath = 100

// NOTE: the daemon package is Unix-only today (lock.go uses syscall.Flock
// without build tags), so the helpers here use Unix-only syscalls freely.

// DefaultSocketPath returns the conventional socket path for a project
// rooted at root (.krit/daemon.sock under the project root). When that
// path would exceed the unix socket path limit (deeply nested checkouts,
// long CI workspace names), it returns a short, stable per-root path in a
// private (0700, current-user-owned) directory instead, so the daemon can
// still bind. Server and clients both derive the path here from the
// absolute, symlink-resolved root, so they always agree.
//
// If no safe fallback directory exists it returns the conventional path
// unchanged; ResolveSocketPath reports why.
func DefaultSocketPath(root string) string {
	p, err := ResolveSocketPath(root)
	if err != nil {
		return filepath.Join(NormalizeRoot(root), ".krit", "daemon.sock")
	}
	return p
}

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

const (
	fallbackSockSuffix = ".sock"
	pointerFileName    = "daemon.sock.path"
)

var fallbackSockName = regexp.MustCompile(`^[0-9a-f]{16}\.sock$`)

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

// RemoveSocketPointer removes the pointer file if it still names sock (so a
// successor daemon's pointer is left alone). A missing file is not an error.
func RemoveSocketPointer(pointer, sock string) error {
	if got, err := os.ReadFile(pointer); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	} else if strings.TrimSpace(string(got)) != sock {
		return nil
	}
	if err := os.Remove(pointer); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}

// readSocketPointer returns the socket path recorded for root, when the
// pointer file is a regular file owned by us holding an absolute path that
// is either the conventional socket or a fallback-shaped one.
func readSocketPointer(root string) (string, bool) {
	ptr := PointerPath(root)
	fi, err := os.Lstat(ptr)
	if err != nil || !fi.Mode().IsRegular() || fi.Size() > 4096 {
		return "", false
	}
	if st, ok := fi.Sys().(*syscall.Stat_t); !ok || int(st.Uid) != os.Getuid() {
		return "", false
	}
	data, err := os.ReadFile(ptr)
	if err != nil {
		return "", false
	}
	p := strings.TrimSpace(string(data))
	conventional := filepath.Join(NormalizeRoot(root), ".krit", "daemon.sock")
	if !filepath.IsAbs(p) || len(p) > maxSocketPath || (p != conventional && !IsFallbackSocket(p)) {
		return "", false
	}
	return p, true
}

// LocateSocketPath is what clients use to find a daemon: the pointer file
// written by the daemon (robust to a different TMPDIR/XDG_RUNTIME_DIR in the
// daemon's environment) when present, else the derived ResolveSocketPath.
// The server never uses it; it always derives (or is told via --socket).
func LocateSocketPath(root string) (string, error) {
	if p, ok := readSocketPointer(root); ok {
		return p, nil
	}
	return ResolveSocketPath(root)
}

// ResolveSocketPath is DefaultSocketPath with the failure reason: it errors
// when the conventional path is too long and no private fallback directory
// is usable.
func ResolveSocketPath(root string) (string, error) {
	root = NormalizeRoot(root)
	p := filepath.Join(root, ".krit", "daemon.sock")
	if len(p) <= maxSocketPath {
		return p, nil
	}
	sum := sha256.Sum256([]byte(root))
	name := hex.EncodeToString(sum[:8]) + fallbackSockSuffix
	uid := os.Getuid()
	var bases []string
	if x := os.Getenv("XDG_RUNTIME_DIR"); x != "" {
		bases = append(bases, filepath.Join(x, "krit"))
	}
	bases = append(bases,
		filepath.Join(os.TempDir(), fmt.Sprintf("krit-%d", uid)),
		filepath.Join("/tmp", fmt.Sprintf("krit-%d", uid)),
	)
	var errs []error
	for _, dir := range bases {
		sock := filepath.Join(dir, name)
		if len(sock) > maxSocketPath {
			errs = append(errs, fmt.Errorf("%s: path too long for a unix socket", sock))
			continue
		}
		if err := fsutil.EnsurePrivateDir(dir, true); err != nil {
			errs = append(errs, err)
			continue
		}
		return sock, nil
	}
	return "", fmt.Errorf("daemon: no usable socket directory for %s: %w", root, errors.Join(errs...))
}

// dialVerified dials the unix socket and, before anything is sent over it,
// confirms the listener is a process of the current user. The socket is
// 0600 and fallback dirs are 0700, so this is defence in depth against a
// socket planted by another local user.
func dialVerified(socketPath string, timeout time.Duration) (net.Conn, error) {
	conn, err := (&net.Dialer{Timeout: timeout}).DialContext(context.Background(), "unix", socketPath)
	if err != nil {
		return nil, err
	}
	if err := verifyPeer(conn, socketPath); err != nil {
		_ = conn.Close()
		return nil, err
	}
	return conn, nil
}

// Available reports whether a daemon appears reachable at socketPath. It
// performs a short-timeout dial; absence is not an error from the caller's
// perspective, just "fall back to in-process".
func Available(socketPath string) bool {
	if socketPath == "" {
		return false
	}
	if _, err := os.Stat(socketPath); err != nil {
		return false
	}
	conn, err := dialVerified(socketPath, 200*time.Millisecond)
	if err != nil {
		return false
	}
	_ = conn.Close()
	return true
}

// Call sends a single Request to the daemon at socketPath and decodes the
// Data field of the Response into out (which may be nil). It returns the
// daemon's error string as a Go error when OK=false.
func Call(socketPath, verb string, args any, out any) error {
	conn, err := dialVerified(socketPath, 2*time.Second)
	if err != nil {
		return fmt.Errorf("daemon: dial %s: %w", socketPath, err)
	}
	defer conn.Close()

	var raw json.RawMessage
	if args != nil {
		buf, err := json.Marshal(args)
		if err != nil {
			return fmt.Errorf("daemon: marshal args: %w", err)
		}
		raw = buf
	}
	req := Request{Verb: verb, Args: raw}
	body, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("daemon: marshal request: %w", err)
	}
	body = append(body, '\n')
	if _, err := conn.Write(body); err != nil {
		return fmt.Errorf("daemon: write: %w", err)
	}
	reader := bufio.NewReader(conn)
	line, err := reader.ReadBytes('\n')
	if err != nil {
		return fmt.Errorf("daemon: read: %w", err)
	}
	// Fast path for analyze-project's giant 30 MB envelope: a hand-
	// rolled byte scanner extracts Findings (json.RawMessage) and
	// Stats (small struct) without paying the ~150 ms two-pass
	// json.Unmarshal that the generic path costs on warm baselines.
	// Falls back transparently to json.Unmarshal when the envelope
	// shape doesn't match (different field order, whitespace, etc.).
	if apr, ok := out.(*AnalyzeProjectResult); ok {
		handled, daemonErr := ScanAnalyzeProjectResponse(line, apr)
		if handled {
			return daemonErr
		}
	}
	var resp Response
	if err := json.Unmarshal(line, &resp); err != nil {
		return fmt.Errorf("daemon: decode response: %w", err)
	}
	if !resp.OK {
		return errors.New(resp.Error)
	}
	if out == nil {
		return nil
	}
	if len(resp.Data) == 0 {
		return nil
	}
	return json.Unmarshal(resp.Data, out)
}
