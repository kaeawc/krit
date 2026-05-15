package daemonclient

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/kaeawc/krit/internal/daemon"
)

// TestTryConnect_FindsRunningDaemon checks the happy path: a daemon
// is listening at the conventional socket and TryConnect returns a
// usable client.
func TestTryConnect_FindsRunningDaemon(t *testing.T) {
	socketDir, _ := startEchoDaemon(t)
	root := newTempRoot(t)
	expected := daemon.DefaultSocketPath(root)
	if err := os.MkdirAll(filepath.Dir(expected), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.Symlink(filepath.Join(socketDir, "d.sock"), expected); err != nil {
		t.Fatalf("symlink: %v", err)
	}

	c, ok := TryConnect(root, "")
	if !ok {
		t.Fatalf("TryConnect returned no client")
	}
	if c.SocketPath() != expected {
		t.Errorf("socket path mismatch: got %q want %q", c.SocketPath(), expected)
	}
}

// TestTryConnect_NoSocketFallsBack covers the auto-detect "nothing
// running" branch — no socket at all, no error, ok=false. The CLI's
// silent-fallback contract depends on this.
func TestTryConnect_NoSocketFallsBack(t *testing.T) {
	root := newTempRoot(t)
	if c, ok := TryConnect(root, ""); ok {
		t.Fatalf("expected ok=false when no daemon is running, got client=%q", c.SocketPath())
	}
}

// TestTryConnect_StaleSocketFallsBack mirrors the issue's stale-socket
// requirement: a socket file exists but no listener is bound. Dialing
// fails fast; TryConnect treats this as "fall back silently".
func TestTryConnect_StaleSocketFallsBack(t *testing.T) {
	root := newTempRoot(t)
	socket := daemon.DefaultSocketPath(root)
	if err := os.MkdirAll(filepath.Dir(socket), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(socket, []byte{}, 0o600); err != nil {
		t.Fatalf("create stale socket: %v", err)
	}
	if c, ok := TryConnect(root, ""); ok {
		t.Fatalf("expected ok=false on stale socket, got client=%q", c.SocketPath())
	}
}

// TestTryConnect_HonorsSocketOverride confirms --daemon-socket
// semantics: an explicit path beats the default discovery rule.
func TestTryConnect_HonorsSocketOverride(t *testing.T) {
	socketDir, _ := startEchoDaemon(t)
	override := filepath.Join(socketDir, "d.sock")
	root := newTempRoot(t)
	// Default discovery would fail (no symlink); override drives the
	// dial directly.
	c, ok := TryConnect(root, override)
	if !ok {
		t.Fatalf("expected ok=true with override; got false")
	}
	if c.SocketPath() != override {
		t.Errorf("socket path mismatch: got %q want %q", c.SocketPath(), override)
	}
}

// TestIsBinaryHashMismatch_MatchesDaemonError checks the wire
// detection path: an error coming back from daemon.Call carrying the
// ErrBinaryHashMismatchPrefix is classified as a mismatch.
func TestIsBinaryHashMismatch_MatchesDaemonError(t *testing.T) {
	err := errors.New("binary hash mismatch (daemon=aaaa client=bbbb)")
	if !IsBinaryHashMismatch(err) {
		t.Fatal("expected true")
	}
	if IsBinaryHashMismatch(nil) {
		t.Fatal("nil should be false")
	}
	if IsBinaryHashMismatch(errors.New("unrelated")) {
		t.Fatal("unrelated error should be false")
	}
}

// TestAnalyzeProject_MismatchSurfacesError exercises the end-to-end
// handshake: a daemon configured to enforce a different hash refuses
// the AnalyzeProject call, and IsBinaryHashMismatch classifies the
// error.
func TestAnalyzeProject_MismatchSurfacesError(t *testing.T) {
	socket := startAnalyzeProjectDaemon(t, "aaaa-daemon-hash")
	c := &Client{socketPath: socket}
	_, err := c.AnalyzeProject(daemon.AnalyzeProjectArgs{ClientBinaryHash: "bbbb-client-hash"})
	if err == nil {
		t.Fatal("expected mismatch error, got nil")
	}
	if !IsBinaryHashMismatch(err) {
		t.Errorf("expected IsBinaryHashMismatch=true; got err=%v", err)
	}
}

// TestAnalyzeProject_HashMatchReturnsResult covers the happy
// handshake: matching hashes pass through and the daemon's payload is
// returned unchanged.
func TestAnalyzeProject_HashMatchReturnsResult(t *testing.T) {
	socket := startAnalyzeProjectDaemon(t, "matching-hash")
	c := &Client{socketPath: socket}
	res, err := c.AnalyzeProject(daemon.AnalyzeProjectArgs{ClientBinaryHash: "matching-hash"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(res.Findings) != `{"rules":[],"line":[]}` {
		t.Errorf("findings mismatch: %s", string(res.Findings))
	}
}

// startAnalyzeProjectDaemon spins up a minimal server that registers
// only the analyze-project verb with the same hash-rejection rule the
// real server uses. Returns the socket path; the server is torn down
// via t.Cleanup.
func startAnalyzeProjectDaemon(t *testing.T, daemonHash string) string {
	t.Helper()
	socketDir, err := os.MkdirTemp("/tmp", "krit-cli-mismatch-")
	if err != nil {
		t.Fatalf("MkdirTemp: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(socketDir) })
	socket := filepath.Join(socketDir, "d.sock")

	srv := daemon.NewServer(socket)
	srv.Register(daemon.VerbAnalyzeProject, func(_ context.Context, raw json.RawMessage) (any, error) {
		var args daemon.AnalyzeProjectArgs
		if len(raw) > 0 {
			if err := json.Unmarshal(raw, &args); err != nil {
				return nil, err
			}
		}
		if args.ClientBinaryHash != "" && daemonHash != "" && args.ClientBinaryHash != daemonHash {
			return nil, fmt.Errorf("%s (daemon=%s client=%s)", daemon.ErrBinaryHashMismatchPrefix, daemonHash, args.ClientBinaryHash)
		}
		return daemon.AnalyzeProjectResult{Findings: json.RawMessage(`{"rules":[],"line":[]}`)}, nil
	})
	if err := srv.Start(context.Background()); err != nil {
		t.Fatalf("start: %v", err)
	}
	t.Cleanup(srv.Stop)
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		if daemon.Available(socket) {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
	return socket
}

// newTempRoot is a thin wrapper that builds a short-path temp
// directory under /tmp so macOS's 104-byte socket path limit doesn't
// bite long test names.
func newTempRoot(t *testing.T) string {
	t.Helper()
	root, err := os.MkdirTemp("/tmp", "krit-tryconn-")
	if err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(root) })
	return root
}
