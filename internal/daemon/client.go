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
	"time"
)

// maxSocketPath is the longest unix socket path DefaultSocketPath will
// return before falling back to a short temp-dir path. sun_path holds 108
// bytes on Linux and 104 on macOS (including the NUL), and bind(2) fails
// with EINVAL beyond that.
const maxSocketPath = 100

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
		return filepath.Join(normalizeRoot(root), ".krit", "daemon.sock")
	}
	return p
}

// normalizeRoot makes root absolute and resolves symlinks (falling back to
// the absolute form), so a relative root and its absolute form map to the
// same socket.
func normalizeRoot(root string) string {
	abs, err := filepath.Abs(root)
	if err != nil {
		return filepath.Clean(root)
	}
	if resolved, err := filepath.EvalSymlinks(abs); err == nil {
		return resolved
	}
	return abs
}

// ResolveSocketPath is DefaultSocketPath with the failure reason: it errors
// when the conventional path is too long and no private fallback directory
// is usable.
func ResolveSocketPath(root string) (string, error) {
	root = normalizeRoot(root)
	p := filepath.Join(root, ".krit", "daemon.sock")
	if len(p) <= maxSocketPath {
		return p, nil
	}
	sum := sha256.Sum256([]byte(root))
	name := hex.EncodeToString(sum[:8]) + ".sock"
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
		if err := ensurePrivateDir(dir); err != nil {
			errs = append(errs, err)
			continue
		}
		return sock, nil
	}
	return "", fmt.Errorf("daemon: no usable socket directory for %s: %w", root, errors.Join(errs...))
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
	conn, err := (&net.Dialer{Timeout: 200 * time.Millisecond}).DialContext(context.Background(), "unix", socketPath)
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
	conn, err := (&net.Dialer{Timeout: 2 * time.Second}).DialContext(context.Background(), "unix", socketPath)
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
