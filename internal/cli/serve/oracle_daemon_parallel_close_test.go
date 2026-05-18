package serve

import (
	"sync/atomic"
	"testing"
	"time"
)

// fakeBlockingEntry produces an oracleDaemonEntry whose close blocks
// for `closeDelay`, recording invocation count via gotClose. Used by
// the parallel-shutdown assertions below.
func fakeBlockingEntry(closeDelay time.Duration, gotClose *atomic.Int32) *oracleDaemonEntry {
	return &oracleDaemonEntry{
		jarPath:    "fake.jar",
		sourceDirs: []string{},
		closeFn: func() error {
			gotClose.Add(1)
			time.Sleep(closeDelay)
			return nil
		},
	}
}

// TestCloseOracleDaemons_ShutsDownInParallel proves fix D: N owned
// daemons close concurrently rather than serially under the registry
// mutex. With the old implementation N×closeDelay was the lower bound;
// the fix collapses to ~closeDelay + scheduler slack.
func TestCloseOracleDaemons_ShutsDownInParallel(t *testing.T) {
	state := newDaemonState(t.TempDir())

	const N = 4
	const closeDelay = 200 * time.Millisecond
	var calls atomic.Int32
	for i := 0; i < N; i++ {
		state.oracleDaemonByKey["k"+string(rune('a'+i))] = fakeBlockingEntry(closeDelay, &calls)
	}

	start := time.Now()
	state.closeOracleDaemons()
	elapsed := time.Since(start)

	if got := calls.Load(); got != int32(N) {
		t.Fatalf("close invoked %d times; want %d", got, N)
	}

	// Parallel: ~closeDelay + slack. Serial would be N×closeDelay = 800ms.
	// Use a 1.5× single-delay ceiling: gives plenty of room for slow CI
	// without admitting the serial path.
	maxParallel := closeDelay * 3 / 2
	if elapsed > maxParallel {
		t.Errorf("closeOracleDaemons elapsed %s; expected <%s (parallel close)", elapsed, maxParallel)
	}
	// Also assert we did at least one delay's worth — guards against a
	// future refactor that accidentally skips Close entirely.
	if elapsed < closeDelay/2 {
		t.Errorf("closeOracleDaemons elapsed %s; expected ≥%s (proves we waited)", elapsed, closeDelay/2)
	}

	if got := len(state.oracleDaemonByKey); got != 0 {
		t.Errorf("after close, cache has %d entries; want 0", got)
	}
}

// TestCloseOracleDaemons_DoesNotHoldRegistryMutexAcrossClose is a
// complementary check: while closeOracleDaemons is mid-flight (i.e.
// blocked inside one of the parallel Close goroutines), the registry
// mutex must already be available to a concurrent reader. With the
// old implementation, the reader would block for N×closeDelay.
func TestCloseOracleDaemons_DoesNotHoldRegistryMutexAcrossClose(t *testing.T) {
	state := newDaemonState(t.TempDir())

	const N = 4
	const closeDelay = 250 * time.Millisecond
	var calls atomic.Int32

	// Block one of the closes so the goroutines are mid-flight when
	// we probe the registry mutex.
	probeStart := make(chan struct{})
	probeDone := make(chan struct{})
	for i := 0; i < N; i++ {
		state.oracleDaemonByKey["k"+string(rune('a'+i))] = &oracleDaemonEntry{
			jarPath: "fake.jar",
			closeFn: func() error {
				calls.Add(1)
				time.Sleep(closeDelay)
				return nil
			},
		}
	}

	go func() {
		<-probeStart
		// Should be able to acquire the registry mutex immediately —
		// the parallel-close fix releases it once entries are
		// snapshotted, well before any Close completes.
		state.oracleDaemonMu.Lock()
		_ = state.oracleDaemonByKey
		state.oracleDaemonMu.Unlock()
		close(probeDone)
	}()

	go func() {
		// Let the probe race against closeOracleDaemons; small sleep
		// to make sure closeOracleDaemons has had a chance to acquire
		// & release the mutex before the probe asks for it.
		time.Sleep(20 * time.Millisecond)
		close(probeStart)
	}()

	state.closeOracleDaemons()

	// Probe must finish promptly (the registry mutex was released
	// quickly inside closeOracleDaemons even though Close goroutines
	// were still blocked).
	select {
	case <-probeDone:
	case <-time.After(closeDelay):
		t.Fatalf("registry mutex was held across the Close path; probe blocked >%s", closeDelay)
	}
}
