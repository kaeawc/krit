package pipeline

import (
	"sync"
	"sync/atomic"
	"testing"
)

// TestWorkspaceState_BackgroundSave_RunsAndFlushes pins the core
// contract: enqueued closures run on the background worker, and
// FlushBackgroundSaves blocks until every pending save has completed.
// This is the daemon-shutdown guarantee — a save deferred off the warm
// critical path must still land before the process exits.
func TestWorkspaceState_BackgroundSave_RunsAndFlushes(t *testing.T) {
	w := NewWorkspaceState("")

	var ran atomic.Int64
	const n = 5
	for i := 0; i < n; i++ {
		w.EnqueueBackgroundSave(func() { ran.Add(1) })
	}

	w.FlushBackgroundSaves()

	if got := ran.Load(); got != n {
		t.Fatalf("expected %d background saves to complete after flush, got %d", n, got)
	}
}

// TestWorkspaceState_BackgroundSave_NilSafe verifies the nil-receiver
// and nil-closure guards: a nil receiver runs a non-nil closure inline
// (so callers needn't branch), and FlushBackgroundSaves on a workspace
// that never enqueued anything is a no-op.
func TestWorkspaceState_BackgroundSave_NilSafe(t *testing.T) {
	var ran bool
	var w *WorkspaceState
	w.EnqueueBackgroundSave(func() { ran = true })
	if !ran {
		t.Fatal("nil receiver should run the closure inline")
	}

	fresh := NewWorkspaceState("")
	fresh.FlushBackgroundSaves() // worker never started — must not block or panic
}

// TestWorkspaceState_BackgroundSave_BackPressureRunsInline drives more
// saves than the buffer holds while the worker is blocked, proving the
// producer falls back to running inline rather than dropping a write.
// Every enqueued save must still complete.
func TestWorkspaceState_BackgroundSave_BackPressureRunsInline(t *testing.T) {
	w := NewWorkspaceState("")

	// Block the worker on the first job until we release it, so the
	// buffer fills and later enqueues take the inline back-pressure path.
	release := make(chan struct{})
	var started sync.WaitGroup
	started.Add(1)
	var once sync.Once
	var ran atomic.Int64

	total := backgroundSaveQueueDepth * 3
	for i := 0; i < total; i++ {
		first := i == 0
		w.EnqueueBackgroundSave(func() {
			if first {
				once.Do(started.Done)
				<-release
			}
			ran.Add(1)
		})
	}
	// Ensure the worker has picked up the blocking job before releasing.
	started.Wait()
	close(release)
	w.FlushBackgroundSaves()

	if got := ran.Load(); int(got) != total {
		t.Fatalf("expected all %d saves to run under back-pressure, got %d", total, got)
	}
}
