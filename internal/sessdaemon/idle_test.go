package sessdaemon

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// TestIdleAutoShutdownStopsServer asserts that a daemon configured with
// IdleTimeout self-stops after no requests for the configured window.
// Validation criterion: Wait() returns within idleTimeout + slack when
// no traffic arrives.
func TestIdleAutoShutdownStopsServer(t *testing.T) {
	repo := t.TempDir()
	sockDir, err := os.MkdirTemp("", "kritd")
	if err != nil {
		t.Fatalf("mkdtemp: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(sockDir) })
	socket := filepath.Join(sockDir, "d.sock")

	idle := 300 * time.Millisecond
	srv, err := NewServer(context.Background(), Options{
		RepoDir:     repo,
		SocketPath:  socket,
		IdleTimeout: idle,
	})
	if err != nil {
		t.Fatalf("new server: %v", err)
	}
	if err := srv.Start(context.Background()); err != nil {
		t.Fatalf("start: %v", err)
	}

	done := make(chan struct{})
	go func() {
		srv.Wait()
		close(done)
	}()

	select {
	case <-done:
		// stopped — good
	case <-time.After(idle + 2*time.Second):
		srv.Stop()
		t.Fatalf("daemon did not self-stop within %s of idle timeout %s", 2*time.Second, idle)
	}
}

// TestIdleAutoShutdownResetsOnRequest asserts that incoming requests
// reset the idle clock: a daemon with a short idle window stays alive
// while a client sends health requests faster than the window.
func TestIdleAutoShutdownResetsOnRequest(t *testing.T) {
	repo := t.TempDir()
	sockDir, err := os.MkdirTemp("", "kritd")
	if err != nil {
		t.Fatalf("mkdtemp: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(sockDir) })
	socket := filepath.Join(sockDir, "d.sock")

	idle := 500 * time.Millisecond
	srv, err := NewServer(context.Background(), Options{
		RepoDir:     repo,
		SocketPath:  socket,
		IdleTimeout: idle,
	})
	if err != nil {
		t.Fatalf("new server: %v", err)
	}
	if err := srv.Start(context.Background()); err != nil {
		t.Fatalf("start: %v", err)
	}
	t.Cleanup(func() {
		srv.Stop()
		srv.Wait()
	})

	// Drive health requests for ~1.2s at ~100ms cadence — longer than
	// the idle window. The server must still be reachable at the end.
	stop := time.After(1200 * time.Millisecond)
	tick := time.NewTicker(100 * time.Millisecond)
	defer tick.Stop()
	for {
		select {
		case <-stop:
			if _, err := Health(socket); err != nil {
				t.Fatalf("daemon went away despite request traffic: %v", err)
			}
			return
		case <-tick.C:
			if _, err := Health(socket); err != nil {
				t.Fatalf("health during traffic: %v", err)
			}
		}
	}
}

// TestIdleAutoShutdownDisabledByDefault asserts the zero-value
// IdleTimeout leaves the daemon running indefinitely. We can't wait
// forever, so we wait past a duration that would have tripped any
// reasonable watchdog and confirm the server still answers Health.
func TestIdleAutoShutdownDisabledByDefault(t *testing.T) {
	repo := t.TempDir()
	socket := startTestServer(t, repo)

	time.Sleep(400 * time.Millisecond)
	if _, err := Health(socket); err != nil {
		t.Fatalf("daemon should still be running with idle disabled: %v", err)
	}
}
