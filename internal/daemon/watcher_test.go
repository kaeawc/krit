package daemon

import (
	"context"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/kaeawc/krit/internal/pipeline"
)

// countingState tracks watcher-driven invalidation calls so unit
// tests can assert routing without spinning up the real workspace.
type countingState struct {
	mu             sync.Mutex
	invalidate     int
	invalidateAll  int
	codeIndex      int
	dependents     int
	library        int
	resolver       int
	oracle         int
	touch          int
	lastTouched    string
	lastInvalidate string
}

func (c *countingState) Invalidate(p string) {
	c.mu.Lock()
	c.invalidate++
	c.lastInvalidate = p
	c.mu.Unlock()
}
func (c *countingState) InvalidateAll() { c.mu.Lock(); c.invalidateAll++; c.mu.Unlock() }
func (c *countingState) InvalidateCodeIndex() {
	c.mu.Lock()
	c.codeIndex++
	c.mu.Unlock()
}
func (c *countingState) InvalidateDependents() {
	c.mu.Lock()
	c.dependents++
	c.mu.Unlock()
}
func (c *countingState) InvalidateLibraryFacts() {
	c.mu.Lock()
	c.library++
	c.mu.Unlock()
}
func (c *countingState) InvalidateResolver() {
	c.mu.Lock()
	c.resolver++
	c.mu.Unlock()
}
func (c *countingState) InvalidateOracleFilter() {
	c.mu.Lock()
	c.oracle++
	c.mu.Unlock()
}
func (c *countingState) Touch(p string) {
	c.mu.Lock()
	c.touch++
	c.lastTouched = p
	c.mu.Unlock()
}

type stateSnapshot struct {
	invalidate     int
	invalidateAll  int
	codeIndex      int
	dependents     int
	library        int
	resolver       int
	oracle         int
	touch          int
	lastTouched    string
	lastInvalidate string
}

func (c *countingState) snapshot() stateSnapshot {
	c.mu.Lock()
	defer c.mu.Unlock()
	return stateSnapshot{
		invalidate:     c.invalidate,
		invalidateAll:  c.invalidateAll,
		codeIndex:      c.codeIndex,
		dependents:     c.dependents,
		library:        c.library,
		resolver:       c.resolver,
		oracle:         c.oracle,
		touch:          c.touch,
		lastTouched:    c.lastTouched,
		lastInvalidate: c.lastInvalidate,
	}
}

func waitFor(t *testing.T, fn func() bool) bool {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if fn() {
			return true
		}
		time.Sleep(5 * time.Millisecond)
	}
	return fn()
}

// TestRouting_TableDriven feeds synthetic paths through route() and
// asserts the correct invalidate buckets fire. fsnotify is bypassed so
// the assertions are deterministic.
func TestRouting_TableDriven(t *testing.T) {
	cases := []struct {
		name        string
		path        string
		assertState func(t *testing.T, s stateSnapshot)
		assertEvent WatcherEvent // 0 = none
	}{
		{
			name: "kotlin source",
			path: "/repo/src/main/kotlin/Foo.kt",
			assertState: func(t *testing.T, s stateSnapshot) {
				if s.invalidate != 1 || s.codeIndex != 1 || s.dependents != 1 || s.resolver != 1 || s.oracle != 1 {
					t.Fatalf("kotlin: %+v", s)
				}
			},
		},
		{
			name: "kts script",
			path: "/repo/buildSrc/foo.kts",
			assertState: func(t *testing.T, s stateSnapshot) {
				if s.invalidate != 1 || s.codeIndex != 1 {
					t.Fatalf("kts: %+v", s)
				}
			},
		},
		{
			name: "java source",
			path: "/repo/src/Bar.java",
			assertState: func(t *testing.T, s stateSnapshot) {
				if s.invalidate != 1 || s.codeIndex != 1 {
					t.Fatalf("java: %+v", s)
				}
			},
		},
		{
			name: "gradle build script",
			path: "/repo/app/build.gradle.kts",
			assertState: func(t *testing.T, s stateSnapshot) {
				if s.library != 1 || s.codeIndex != 1 || s.dependents != 1 {
					t.Fatalf("gradle: %+v", s)
				}
			},
		},
		{
			name: "version catalog",
			path: "/repo/gradle/libs.versions.toml",
			assertState: func(t *testing.T, s stateSnapshot) {
				if s.library != 1 {
					t.Fatalf("toml: %+v", s)
				}
			},
		},
		{
			name: "krit.yml",
			path: "/repo/krit.yml",
			assertState: func(t *testing.T, s stateSnapshot) {
				if s.invalidateAll != 1 {
					t.Fatalf("config: %+v", s)
				}
			},
			assertEvent: EventConfigReload,
		},
		{
			name: "AndroidManifest",
			path: "/repo/app/src/main/AndroidManifest.xml",
			assertState: func(t *testing.T, s stateSnapshot) {
				if s.touch != 1 {
					t.Fatalf("manifest: %+v", s)
				}
			},
		},
		{
			name: "res xml",
			path: "/repo/app/src/main/res/layout/foo.xml",
			assertState: func(t *testing.T, s stateSnapshot) {
				if s.touch != 1 {
					t.Fatalf("res: %+v", s)
				}
			},
		},
		{
			name: "unrelated file",
			path: "/repo/README.md",
			assertState: func(t *testing.T, s stateSnapshot) {
				if s.invalidate != 0 || s.codeIndex != 0 || s.touch != 0 {
					t.Fatalf("readme: %+v", s)
				}
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			state := &countingState{}
			var androidCleared, resourceCleared atomic.Int32
			w, err := newWatcherWithState(state, t.TempDir(), nil,
				WithDebounceWindow(time.Millisecond),
				WithSweepInterval(0),
				WithAndroidProjectCallback(func() { androidCleared.Add(1) }),
				WithResourceIndexCallback(func() { resourceCleared.Add(1) }),
			)
			if err != nil {
				t.Fatalf("new: %v", err)
			}
			defer w.Stop()
			w.route(tc.path)
			// Wait for debounce timer to fire for source-path cases.
			waitFor(t, func() bool {
				s := state.snapshot()
				return s.invalidate > 0 || s.codeIndex > 0 || s.touch > 0 || s.invalidateAll > 0 || s.library > 0
			})
			tc.assertState(t, state.snapshot())
			if tc.assertEvent != 0 {
				select {
				case got := <-w.Events:
					if got != tc.assertEvent {
						t.Fatalf("event: got %v want %v", got, tc.assertEvent)
					}
				case <-time.After(time.Second):
					t.Fatal("no event")
				}
			}
		})
	}
}

// TestIntegration_RealFsnotify writes a real .kt file and asserts the
// daemon-resident WorkspaceState.parsed map drops the entry.
func TestIntegration_RealFsnotify(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "Foo.kt")
	if err := os.WriteFile(path, []byte("fun a() {}\n"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	ws := pipeline.NewWorkspaceState(root)
	if _, err := ws.ParseFile(context.Background(), path, []byte("fun a() {}\n")); err != nil {
		t.Fatalf("seed: %v", err)
	}
	if got := ws.Stats().ParsedEntries; got != 1 {
		t.Fatalf("setup: %d", got)
	}

	w, err := NewWatcher(ws, root, nil, WithDebounceWindow(5*time.Millisecond), WithSweepInterval(0))
	if err != nil {
		t.Fatalf("new: %v", err)
	}
	if err := w.Start(context.Background()); err != nil {
		t.Fatalf("start: %v", err)
	}
	defer w.Stop()

	if err := os.WriteFile(path, []byte("fun a() { 42 }\n"), 0o644); err != nil {
		t.Fatalf("rewrite: %v", err)
	}
	if !waitFor(t, func() bool { return ws.Stats().ParsedEntries == 0 }) {
		t.Fatalf("entry survived: %+v", ws.Stats())
	}
}

// TestStress_DebounceCoalesces fires many writes in rapid succession
// and asserts the watcher coalesces them into one InvalidateCodeIndex
// call per file.
func TestStress_DebounceCoalesces(t *testing.T) {
	state := &countingState{}
	w, err := newWatcherWithState(state, t.TempDir(), nil,
		WithDebounceWindow(20*time.Millisecond),
		WithSweepInterval(0),
	)
	if err != nil {
		t.Fatalf("new: %v", err)
	}
	defer w.Stop()

	// 50 logical files, 4 events each (Write+Write+Chmod+Write).
	const files = 50
	const eventsPerFile = 4
	for i := 0; i < files; i++ {
		p := filepath.Join("/repo", "F"+strconv.Itoa(i)+".kt")
		for j := 0; j < eventsPerFile; j++ {
			w.route(p)
		}
	}
	// Wait for debouncer to drain.
	waitFor(t, func() bool { return w.CodeIndexInvalidations() == files })
	got := w.CodeIndexInvalidations()
	if got != files {
		t.Fatalf("expected %d invalidations after coalescing, got %d (state=%+v)", files, got, state.snapshot())
	}
}

// TestMtimeSweep_CatchesMissedEvent simulates a missed fsnotify event
// by mutating mtime on a tracked file without triggering the
// underlying watcher, and asserts the sweep recovers the change.
func TestMtimeSweep_CatchesMissedEvent(t *testing.T) {
	state := &countingState{}
	root := t.TempDir()
	w, err := newWatcherWithState(state, root, nil,
		WithDebounceWindow(time.Millisecond),
		WithSweepInterval(20*time.Millisecond),
	)
	if err != nil {
		t.Fatalf("new: %v", err)
	}
	// Don't Start() — we drive the sweep directly. Stop() is safe on
	// an un-Started watcher.
	defer w.Stop()

	path := filepath.Join(root, "Stale.kt")
	if err := os.WriteFile(path, []byte("a"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	// Track the file with its current mtime.
	w.Track(path)
	// Mutate the file's mtime artificially (no fsnotify delivery).
	future := time.Now().Add(time.Minute)
	if err := os.Chtimes(path, future, future); err != nil {
		t.Fatalf("chtimes: %v", err)
	}
	w.sweepOnce()
	if w.SweepCatches() == 0 {
		t.Fatalf("sweep did not catch missed event: state=%+v", state.snapshot())
	}
	if !waitFor(t, func() bool { return state.snapshot().codeIndex > 0 }) {
		t.Fatalf("expected InvalidateCodeIndex via sweep route, got %+v", state.snapshot())
	}
}

// TestBinaryPath_EmitsShutdown asserts a write to the watched krit
// binary path emits EventShutdownRequest.
func TestBinaryPath_EmitsShutdown(t *testing.T) {
	state := &countingState{}
	bin := "/tmp/krit"
	w, err := newWatcherWithState(state, t.TempDir(), nil,
		WithBinaryPath(bin),
		WithDebounceWindow(time.Millisecond),
		WithSweepInterval(0),
	)
	if err != nil {
		t.Fatalf("new: %v", err)
	}
	defer w.Stop()
	w.route(bin)
	select {
	case got := <-w.Events:
		if got != EventShutdownRequest {
			t.Fatalf("want ShutdownRequest, got %v", got)
		}
	case <-time.After(time.Second):
		t.Fatal("no shutdown event")
	}
}

// TestStop_Idempotent verifies Stop is safe to call multiple times.
func TestStop_Idempotent(t *testing.T) {
	state := &countingState{}
	w, err := newWatcherWithState(state, t.TempDir(), nil, WithSweepInterval(0))
	if err != nil {
		t.Fatalf("new: %v", err)
	}
	if err := w.Start(context.Background()); err != nil {
		t.Fatalf("start: %v", err)
	}
	if err := w.Stop(); err != nil {
		t.Fatalf("stop1: %v", err)
	}
	if err := w.Stop(); err != nil {
		t.Fatalf("stop2: %v", err)
	}
}

// TestRouting_AndroidResource asserts the resource-index callback fires
// for both AndroidManifest.xml and res/**/*.xml.
func TestRouting_AndroidResource(t *testing.T) {
	for _, p := range []string{
		"/repo/app/src/main/AndroidManifest.xml",
		"/repo/app/src/main/res/values/strings.xml",
		"/repo/app/src/main/res/layout/main.xml",
	} {
		state := &countingState{}
		var calls atomic.Int32
		w, err := newWatcherWithState(state, t.TempDir(), nil,
			WithDebounceWindow(time.Millisecond),
			WithSweepInterval(0),
			WithResourceIndexCallback(func() { calls.Add(1) }),
		)
		if err != nil {
			t.Fatalf("new: %v", err)
		}
		w.route(p)
		_ = w.Stop()
		if calls.Load() != 1 {
			t.Fatalf("%s: callbacks=%d", p, calls.Load())
		}
	}
}

// TestPathClassification spot-checks the helpers against negative
// lookalikes — a file named `res.xml` outside a res/ dir must not
// route as an Android resource.
func TestPathClassification(t *testing.T) {
	cases := []struct {
		path string
		want bool
	}{
		{"/a/res/x.xml", true},
		{"/a/src/main/res/layout/foo.xml", true},
		{"/a/AndroidManifest.xml", true},
		{"/a/notres/x.xml", false},
		{"/a/build.gradle", false},
		{"/a/Foo.kt", false},
	}
	for _, tc := range cases {
		if got := isAndroidResourcePath(tc.path); got != tc.want {
			t.Errorf("isAndroidResourcePath(%q) = %v want %v", tc.path, got, tc.want)
		}
	}
}
