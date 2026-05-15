package daemon

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// TestSocketWatchdog_StopsOnSocketUnlink pins the phantom-socket
// failure mode: with the daemon process alive but its socket dirent
// gone from the filesystem, new clients can't reach it (connect(2)
// returns ENOENT on the missing path). Without the watchdog the
// daemon sits there forever serving zero requests and CLI callers
// silently fall back to in-process on every invocation.
//
// The watchdog polls os.Stat on its own socket on a tight interval
// (production: 5s; here: 25ms) and calls Stop the first time the
// stat fails so the next CLI invocation hits "no daemon, spawn
// fresh" rather than "daemon socket missing, give up".
//
// See issue #247-followup.
func TestSocketWatchdog_StopsOnSocketUnlink(t *testing.T) {
	socket := filepath.Join(shortTempDir(t), "d.sock")
	srv := NewServer(socket)
	// Speed up detection so the test doesn't pay the production 5s.
	srv.SocketWatchdogInterval = 25 * time.Millisecond
	srv.Register("echo", func(_ context.Context, raw json.RawMessage) (any, error) {
		var v any
		_ = json.Unmarshal(raw, &v)
		return v, nil
	})
	if err := srv.Start(context.Background()); err != nil {
		t.Fatalf("start: %v", err)
	}

	// Wait for the socket to become reachable before unlinking it —
	// otherwise the watchdog may trigger on the brief window between
	// MkdirAll and the kernel exposing the bound socket.
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		if Available(socket) {
			break
		}
		time.Sleep(2 * time.Millisecond)
	}
	if !Available(socket) {
		t.Fatal("server never became Available; cannot exercise watchdog")
	}

	// Sanity: socket is bound, server serves requests.
	var got string
	if err := Call(socket, "echo", "hi", &got); err != nil {
		t.Fatalf("pre-unlink Call: %v", err)
	}

	// Unlink the dirent. The kernel keeps the bound socket alive on
	// its open fd, but no new connect(2) can resolve the path.
	if err := os.Remove(socket); err != nil {
		t.Fatalf("os.Remove(socket): %v", err)
	}

	// The watchdog must notice within roughly one tick and Stop the
	// server. Allow a generous slack (10× the tick) to ride out
	// scheduler hiccups under -race / CI load.
	stopDeadline := time.Now().Add(500 * time.Millisecond)
	select {
	case <-stoppedSignal(srv):
		// Good — server stopped.
	case <-time.After(time.Until(stopDeadline)):
		t.Fatalf("socket watchdog did not stop the server within %s of unlink", 500*time.Millisecond)
	}

	// Subsequent Call must fail (server is down). The exact error
	// shape varies by OS; the contract is "not nil" so the CLI knows
	// to spawn fresh.
	if err := Call(socket, "echo", "post", nil); err == nil {
		t.Fatal("expected Call to fail after watchdog stop, got nil error")
	} else if !errors.Is(err, os.ErrNotExist) && err.Error() == "" {
		t.Fatalf("Call error after watchdog stop should be a real dial error; got %v", err)
	}
}

// TestSocketWatchdog_Disabled exercises the opt-out: with
// SocketWatchdogDisabled=true the goroutine returns immediately and
// the server keeps running even when the socket dirent is unlinked.
// Drives the bypass path tests outside this file rely on.
func TestSocketWatchdog_Disabled(t *testing.T) {
	socket := filepath.Join(shortTempDir(t), "d.sock")
	srv := NewServer(socket)
	srv.SocketWatchdogDisabled = true
	srv.SocketWatchdogInterval = 25 * time.Millisecond // would-be fast tick
	if err := srv.Start(context.Background()); err != nil {
		t.Fatalf("start: %v", err)
	}
	t.Cleanup(srv.Stop)

	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		if Available(socket) {
			break
		}
		time.Sleep(2 * time.Millisecond)
	}
	if err := os.Remove(socket); err != nil {
		t.Fatalf("os.Remove(socket): %v", err)
	}

	// Sleep for ten tick intervals; the watchdog would have fired by
	// now if not disabled. The stopped channel must remain open.
	time.Sleep(10 * 25 * time.Millisecond)
	select {
	case <-stoppedSignal(srv):
		t.Fatal("server stopped despite SocketWatchdogDisabled=true")
	default:
		// Good — server is still up. Cleanup runs Stop explicitly.
	}
}

// stoppedSignal exposes the server's internal stopped channel without
// surfacing it as a public API. The closure pattern keeps the export
// scoped to this test file.
func stoppedSignal(s *Server) <-chan struct{} { return s.stopped }

// shortTempDir returns a temp directory rooted under /tmp so the
// resulting "<dir>/d.sock" path fits within sun_path's ~104-byte
// limit on macOS. t.TempDir() roots under /var/folders/... which
// blows the limit and makes bind() fail with EINVAL.
func shortTempDir(t *testing.T) string {
	t.Helper()
	dir, err := os.MkdirTemp("/tmp", "krit-d-")
	if err != nil {
		t.Fatalf("mkdir temp: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	return dir
}
