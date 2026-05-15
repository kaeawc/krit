package scan

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/kaeawc/krit/internal/daemon"
)

// TestTryDaemonDelegate_NoDaemonShortCircuits guards the
// `--no-daemon` contract: even if a socket is reachable, the CLI must
// not attempt the dial when the user has opted out.
func TestTryDaemonDelegate_NoDaemonShortCircuits(t *testing.T) {
	socketDir := startMockDaemon(t, mockBehavior{})
	root := newRoot(t)
	linkSock(t, root, filepath.Join(socketDir, "d.sock"))

	f := freshScanFlags(t)
	*f.NoDaemon = true

	handled, code := tryDaemonDelegate(f, []string{root}, root)
	if handled {
		t.Fatalf("expected fall-through; got handled=true code=%d", code)
	}
}

// TestTryDaemonDelegate_NoSocketFallsBack is the auto-detect path:
// no daemon listening, no fallback flag, the CLI should simply hand
// off to in-process by returning handled=false.
func TestTryDaemonDelegate_NoSocketFallsBack(t *testing.T) {
	root := newRoot(t)
	f := freshScanFlags(t)

	handled, _ := tryDaemonDelegate(f, []string{root}, root)
	if handled {
		t.Fatalf("expected fall-through when no daemon is reachable")
	}
}

// TestTryDaemonDelegate_StaleSocketFallsBack pins the kill -9 case:
// a leftover socket inode with no listener must not propagate as an
// error — the CLI should silently drop into in-process mode.
func TestTryDaemonDelegate_StaleSocketFallsBack(t *testing.T) {
	root := newRoot(t)
	socket := daemon.DefaultSocketPath(root)
	if err := os.MkdirAll(filepath.Dir(socket), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(socket, []byte{}, 0o600); err != nil {
		t.Fatalf("create stale: %v", err)
	}
	f := freshScanFlags(t)

	handled, _ := tryDaemonDelegate(f, []string{root}, root)
	if handled {
		t.Fatalf("expected fall-through on stale socket; got handled=true")
	}
}

// TestTryDaemonDelegate_HashMismatchFallsBack confirms the daemon's
// rejection lands as a fallback rather than an error: the CLI prints
// a warning (not asserted here) and continues in-process.
func TestTryDaemonDelegate_HashMismatchFallsBack(t *testing.T) {
	socketDir := startMockDaemon(t, mockBehavior{rejectHash: true})
	root := newRoot(t)
	linkSock(t, root, filepath.Join(socketDir, "d.sock"))

	f := freshScanFlags(t)
	handled, _ := tryDaemonDelegate(f, []string{root}, root)
	if handled {
		t.Fatalf("expected fall-through after mismatch; got handled=true")
	}
}

// TestTryDaemonDelegate_FlagBlocksDelegation exercises the
// compatibility filter: `--fix` (and friends) must not be silently
// dispatched through the daemon since the daemon doesn't perform
// file rewrites.
func TestTryDaemonDelegate_FlagBlocksDelegation(t *testing.T) {
	socketDir := startMockDaemon(t, mockBehavior{})
	root := newRoot(t)
	linkSock(t, root, filepath.Join(socketDir, "d.sock"))

	f := freshScanFlags(t)
	*f.Fix = true

	handled, _ := tryDaemonDelegate(f, []string{root}, root)
	if handled {
		t.Fatalf("expected fall-through with --fix; got handled=true")
	}
}

// TestTryDaemonDelegate_HappyPathReturnsExitFromFindings closes the
// loop: on a clean delegation, the CLI surfaces FindingsCount via
// the standard exit-code rule (0 = clean, 1 = any finding).
func TestTryDaemonDelegate_HappyPathReturnsExitFromFindings(t *testing.T) {
	socketDir := startMockDaemon(t, mockBehavior{
		findings:      `{"rules":["X"],"line":[1]}`,
		findingsCount: 1,
	})
	root := newRoot(t)
	linkSock(t, root, filepath.Join(socketDir, "d.sock"))

	out := redirectStdout(t)
	f := freshScanFlags(t)
	handled, code := tryDaemonDelegate(f, []string{root}, root)
	if !handled {
		t.Fatalf("expected handled=true on clean delegation")
	}
	if code != 1 {
		t.Errorf("expected exit 1 with findings; got %d", code)
	}
	if want := `{"rules":["X"],"line":[1]}`; out() != want {
		t.Errorf("findings byte mismatch:\n got %q\nwant %q", out(), want)
	}
}

// freshScanFlags builds a default-valued scanFlags via a fresh
// FlagSet so each test gets independent storage.
func freshScanFlags(t *testing.T) *scanFlags {
	t.Helper()
	fs := flag.NewFlagSet("scan-test", flag.ContinueOnError)
	return registerScanFlags(fs)
}

// linkSock places root/.krit/daemon.sock pointing at socket so the
// default-discovery path inside TryConnect succeeds.
func linkSock(t *testing.T, root, socket string) {
	t.Helper()
	target := daemon.DefaultSocketPath(root)
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.Symlink(socket, target); err != nil {
		t.Fatalf("symlink: %v", err)
	}
}

// newRoot returns a short-path tempdir under /tmp so the
// .krit/daemon.sock path stays under macOS's 104-byte cap.
func newRoot(t *testing.T) string {
	t.Helper()
	r, err := os.MkdirTemp("/tmp", "krit-deleg-")
	if err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(r) })
	return r
}

// mockBehavior controls what startMockDaemon's analyze-project verb
// returns. rejectHash forces the daemon to refuse any non-empty
// client hash with the documented prefix; findings/findingsCount let
// tests assert on byte-for-byte response shape and exit code logic.
type mockBehavior struct {
	rejectHash    bool
	findings      string
	findingsCount int
}

// startMockDaemon spins up a minimal server with status, shutdown,
// and analyze-project verbs registered. The analyze-project handler
// applies the configured behavior so each test can drive a different
// reply shape without re-implementing the wire protocol.
func startMockDaemon(t *testing.T, b mockBehavior) string {
	t.Helper()
	socketDir, err := os.MkdirTemp("/tmp", "krit-deleg-srv-")
	if err != nil {
		t.Fatalf("MkdirTemp: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(socketDir) })
	socket := filepath.Join(socketDir, "d.sock")
	srv := daemon.NewServer(socket)
	srv.Register(daemon.VerbStatus, func(_ context.Context, _ json.RawMessage) (any, error) {
		return daemon.StatusResult{Ready: true}, nil
	})
	srv.Register(daemon.VerbShutdown, func(_ context.Context, _ json.RawMessage) (any, error) {
		return map[string]bool{"ok": true}, nil
	})
	srv.Register(daemon.VerbAnalyzeProject, func(_ context.Context, raw json.RawMessage) (any, error) {
		var args daemon.AnalyzeProjectArgs
		if len(raw) > 0 {
			if err := json.Unmarshal(raw, &args); err != nil {
				return nil, err
			}
		}
		if b.rejectHash {
			return nil, fmt.Errorf("%s (daemon=ZZZZ client=%s)", daemon.ErrBinaryHashMismatchPrefix, args.ClientBinaryHash)
		}
		findings := b.findings
		if findings == "" {
			findings = `{"rules":[],"line":[]}`
		}
		return daemon.AnalyzeProjectResult{
			Findings: json.RawMessage(findings),
			Stats:    daemon.AnalyzeProjectStats{FindingsCount: b.findingsCount},
		}, nil
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
	return socketDir
}

// redirectStdout swaps os.Stdout for a pipe and returns a thunk that
// drains the captured bytes. Used to assert byte-identical findings
// emission from the daemon path without spinning up an external
// process.
func redirectStdout(t *testing.T) func() string {
	t.Helper()
	orig := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	os.Stdout = w
	t.Cleanup(func() { os.Stdout = orig })
	return func() string {
		_ = w.Close()
		buf, _ := io.ReadAll(r)
		return string(buf)
	}
}
