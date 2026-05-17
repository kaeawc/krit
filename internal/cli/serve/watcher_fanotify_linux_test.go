//go:build linux && fanotify_integration

package serve

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// fanotify_integration is a build tag set only by the Docker test
// harness (see Dockerfile.fanotify). The test needs CAP_SYS_ADMIN
// plus a recent kernel — neither is available in the normal `go
// test ./...` path, so the tag keeps `make test` quiet on developer
// laptops while letting CI / docker-run pick it up explicitly.
//
// The test exercises the real fanotify backend end-to-end: mark the
// filesystem, fsync a new .kt file, assert the backend reports its
// path. Path resolution via open_by_handle_at is exercised
// implicitly — without it the backendEvent.Path would be empty.

func TestFanotifyBackend_CreateAndModify(t *testing.T) {
	root := t.TempDir()
	b, err := newFanotifyBackend(root)
	if err != nil {
		t.Skipf("fanotify backend unavailable: %v", err)
	}
	defer b.Close()

	path := filepath.Join(root, "Foo.kt")
	// Briefly let the mark settle. The kernel marks the filesystem
	// synchronously inside FanotifyMark, but the read goroutine
	// hasn't necessarily entered poll() yet; 10 ms is enough.
	time.Sleep(10 * time.Millisecond)
	if err := os.WriteFile(path, []byte("package x\n"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	got := drainPath(t, b.Events(), path, 2*time.Second)
	if got.Path != path {
		t.Fatalf("Path: got %q, want %q", got.Path, path)
	}
	if got.Op&(opCreate|opWrite) == 0 {
		t.Fatalf("Op: got %d, want opCreate or opWrite bit", got.Op)
	}
}

func TestFanotifyBackend_FiltersOutsideRoot(t *testing.T) {
	root := t.TempDir()
	other := t.TempDir()
	b, err := newFanotifyBackend(root)
	if err != nil {
		t.Skipf("fanotify backend unavailable: %v", err)
	}
	defer b.Close()

	time.Sleep(10 * time.Millisecond)
	// Editing a file outside the watched root must not produce an
	// event. fanotify's filesystem-wide mark sees it; the backend's
	// underRoot filter drops it.
	if err := os.WriteFile(filepath.Join(other, "noise.kt"), []byte("noise"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	// Then a file inside root we *do* expect.
	want := filepath.Join(root, "Signal.kt")
	if err := os.WriteFile(want, []byte("signal"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	got := drainPath(t, b.Events(), want, 2*time.Second)
	if got.Path != want {
		t.Fatalf("Path: got %q, want %q", got.Path, want)
	}
}

// TestFanotifyBackend_AnyEventDelivery is a smoke test that gates
// on the kernel's ability to deliver fanotify events at all,
// regardless of path resolution. If this fails the more specific
// tests above are guaranteed to fail too — debug this first.
//
// Reaches into the backend's raw byte stream by replacing the
// resolved channel with one that admits the unfiltered backendEvent
// emitted from handleEvent. If the test times out, the issue is
// either (a) the kernel isn't delivering events on the mount, or
// (b) open_by_handle_at is failing — see errors channel.
func TestFanotifyBackend_AnyEventDelivery(t *testing.T) {
	root := t.TempDir()
	b, err := newFanotifyBackend(root)
	if err != nil {
		t.Skipf("fanotify backend unavailable: %v", err)
	}
	fb := b.(*fanotifyBackend)
	defer fb.Close()

	time.Sleep(20 * time.Millisecond)
	path := filepath.Join(root, "Probe.kt")
	if err := os.WriteFile(path, []byte("x"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	deadline := time.NewTimer(2 * time.Second)
	defer deadline.Stop()
	for {
		select {
		case ev := <-fb.Events():
			t.Logf("event: path=%q op=%d", ev.Path, ev.Op)
			return
		case err := <-fb.Errors():
			t.Logf("error: %v", err)
		case <-deadline.C:
			t.Fatalf("no events at all after 2s")
		}
	}
}

// drainPath consumes events from ch until one matches target's path
// or the deadline elapses. Lets the test ignore unrelated events
// the kernel might emit on the same mount (cache invalidations,
// directory mtime bumps, etc.) without flaking.
func drainPath(t *testing.T, ch <-chan backendEvent, target string, deadline time.Duration) backendEvent {
	t.Helper()
	timer := time.NewTimer(deadline)
	defer timer.Stop()
	for {
		select {
		case ev := <-ch:
			if ev.Path == target {
				return ev
			}
		case <-timer.C:
			t.Fatalf("timeout waiting for event on %s", target)
			return backendEvent{}
		}
	}
}
