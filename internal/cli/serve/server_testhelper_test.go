package serve

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/kaeawc/krit/internal/daemon"
)

// startServerWith spins up a serve.Server rooted at root with the
// standard verbs registered, returns the socket path and the daemon
// state, and registers teardown on tb. warm() is intentionally not
// invoked — analyze-buffer and the corpus benches drive their own
// warm/cold cadence.
//
// macOS limits Unix-socket paths to 104 bytes; with long test names
// even t.TempDir() overruns. Place the socket under a short MkdirTemp
// rooted at /tmp so the path stays well under the cap.
//
// readyTimeout caps the daemon.Available polling. Use 0 for a tight
// spin (benchmarks tolerate startup latency); pass a deadline for
// unit tests where a stuck server should fail fast.
func startServerWith(tb testing.TB, root string, readyTimeout time.Duration) (string, *daemonState) {
	tb.Helper()
	socketDir, err := os.MkdirTemp("/tmp", "krit-srv-")
	if err != nil {
		tb.Fatalf("MkdirTemp: %v", err)
	}
	tb.Cleanup(func() { _ = os.RemoveAll(socketDir) })
	socket := filepath.Join(socketDir, "d.sock")

	state := newDaemonState(root)
	srv := daemon.NewServer(socket)
	registerVerbs(srv, state)
	if err := srv.Start(context.Background()); err != nil {
		tb.Fatalf("start: %v", err)
	}
	tb.Cleanup(srv.Stop)

	if readyTimeout > 0 {
		deadline := time.Now().Add(readyTimeout)
		for time.Now().Before(deadline) {
			if daemon.Available(socket) {
				break
			}
			time.Sleep(5 * time.Millisecond)
		}
	} else {
		for !daemon.Available(socket) {
			// Spin briefly; benchmarks tolerate this since they're not
			// measuring startup cost.
		}
	}
	return socket, state
}
