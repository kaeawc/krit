package serve

import (
	"errors"
	"sync"
	"testing"

	"github.com/kaeawc/krit/internal/scanner"
)

// TestDaemonStateReusesParseCache pins the determinism contract for
// daemon-resident state: two calls into parseCacheFor return the same
// *scanner.ParseCache pointer, so a downstream pipeline.RunProject
// invocation sees the same in-memory parse table built by the
// previous call. This is the property that turns the 7s zstd-decode
// cost into a one-time daemon-startup cost.
func TestDaemonStateReusesParseCache(t *testing.T) {
	state := newDaemonState(t.TempDir())
	t.Cleanup(state.closeParseCache)

	pc1, err := state.parseCacheFor(state.root, 0)
	if err != nil {
		t.Fatalf("first parseCacheFor: %v", err)
	}
	if pc1 == nil {
		t.Fatal("first parseCacheFor returned nil")
	}
	pc2, err := state.parseCacheFor(state.root, 0)
	if err != nil {
		t.Fatalf("second parseCacheFor: %v", err)
	}
	if pc1 != pc2 {
		t.Fatalf("expected identical pointers; got pc1=%p pc2=%p", pc1, pc2)
	}
}

// TestDaemonStateParseCacheConcurrentInit verifies that N goroutines
// hitting parseCacheFor simultaneously all observe the same instance
// — sync.Once guards the construction. A regression that swapped to
// a non-locking init would race-construct multiple caches.
func TestDaemonStateParseCacheConcurrentInit(t *testing.T) {
	state := newDaemonState(t.TempDir())
	t.Cleanup(state.closeParseCache)

	const N = 16
	var (
		wg      sync.WaitGroup
		mu      sync.Mutex
		results [N]*scanner.ParseCache
		errs    [N]error
	)
	wg.Add(N)
	for i := 0; i < N; i++ {
		i := i
		go func() {
			defer wg.Done()
			pc, err := state.parseCacheFor(state.root, 0)
			mu.Lock()
			results[i] = pc
			errs[i] = err
			mu.Unlock()
		}()
	}
	wg.Wait()

	for i, e := range errs {
		if e != nil {
			t.Errorf("goroutine %d: %v", i, e)
		}
	}
	first := results[0]
	if first == nil {
		t.Fatal("first goroutine did not get a parse cache")
	}
	for i, p := range results {
		if p != first {
			t.Errorf("goroutine %d saw pointer %p, want %p", i, p, first)
		}
	}
}

// TestDaemonStateParseCacheRequiresRepoDir asserts an empty repoDir
// surfaces a clear error rather than panicking deeper in
// scanner.NewParseCacheWithCap.
func TestDaemonStateParseCacheRequiresRepoDir(t *testing.T) {
	state := newDaemonState("")
	t.Cleanup(state.closeParseCache)

	_, err := state.parseCacheFor("", 0)
	if err == nil {
		t.Fatal("expected error for empty repoDir")
	}

	// Subsequent calls return the same error (sync.Once latched).
	_, err2 := state.parseCacheFor("", 0)
	if !errors.Is(err2, err) {
		t.Errorf("expected latched error; first=%v second=%v", err, err2)
	}
}

// TestDaemonStateColdDoneFlag confirms the atomic flag round-trips
// across goroutines (used by the analyze-project verb to gate
// RequireWarm).
func TestDaemonStateColdDoneFlag(t *testing.T) {
	state := newDaemonState(t.TempDir())
	if state.coldDone.Load() {
		t.Fatal("expected fresh daemon state to report cold")
	}
	state.coldDone.Store(true)
	if !state.coldDone.Load() {
		t.Fatal("expected coldDone to be true after Store")
	}
}

// TestDaemonStateCloseParseCacheIdempotent — the Server's shutdown
// path may close the parse cache multiple times. Both calls must be
// safe.
func TestDaemonStateCloseParseCacheIdempotent(t *testing.T) {
	state := newDaemonState(t.TempDir())
	if _, err := state.parseCacheFor(state.root, 0); err != nil {
		t.Fatalf("parseCacheFor: %v", err)
	}
	state.closeParseCache()
	state.closeParseCache() // must not panic
	if state.parseCache != nil {
		t.Errorf("expected parseCache to be nil after close, got %p", state.parseCache)
	}
}

// TestDaemonStateAndroidCacheWriterLifecycle pins the lazy-init +
// idempotent-close contract for the resident Android findings
// cache writer. Mirrors the parse cache pattern: first call builds,
// repeated calls return the same instance; closing twice must not
// panic.
func TestDaemonStateAndroidCacheWriterLifecycle(t *testing.T) {
	state := newDaemonState(t.TempDir())
	w1, dir1 := state.androidCacheWriterFor(state.root)
	if w1 == nil || dir1 == "" {
		t.Fatalf("first androidCacheWriterFor returned (%p, %q), want non-empty", w1, dir1)
	}
	w2, dir2 := state.androidCacheWriterFor(state.root)
	if w2 != w1 || dir2 != dir1 {
		t.Errorf("second call did not reuse the resident writer: got (%p,%q) want (%p,%q)", w2, dir2, w1, dir1)
	}
	state.closeAndroidCacheWriter()
	state.closeAndroidCacheWriter() // must not panic
	if state.androidCacheWriter != nil {
		t.Errorf("expected androidCacheWriter to be nil after close, got %p", state.androidCacheWriter)
	}
}

// TestDaemonStateAndroidCacheWriterEmptyRepo: with no repoDir the
// helper must return (nil, "") and stay that way — the AndroidPhase
// treats a nil writer as "caching disabled" and degrades gracefully.
func TestDaemonStateAndroidCacheWriterEmptyRepo(t *testing.T) {
	state := newDaemonState("")
	w, dir := state.androidCacheWriterFor("")
	if w != nil || dir != "" {
		t.Errorf("empty-repo path: want (nil, \"\"), got (%p, %q)", w, dir)
	}
}
