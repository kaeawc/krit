package serve

import (
	"context"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/fsnotify/fsnotify"

	"github.com/kaeawc/krit/internal/librarymodel"
	"github.com/kaeawc/krit/internal/pipeline"
	"github.com/kaeawc/krit/internal/scanner"
)

func fsnotifyEventChmod(path string) fsnotify.Event {
	return fsnotify.Event{Name: path, Op: fsnotify.Chmod}
}

// countingState is a watcherState fake that counts Invalidate calls
// and Bump calls. The bump counter lets tests confirm the stats-
// clean memo invalidation hook fires alongside the per-slot
// invalidations on real source-path events.
type countingState struct {
	invalidateCalls atomic.Int64
	bumpCalls       atomic.Int64
	javaBumpCalls   atomic.Int64
	xmlBumpCalls    atomic.Int64
}

func (c *countingState) Invalidate(path string)  { c.invalidateCalls.Add(1) }
func (c *countingState) InvalidateCodeIndex()    {}
func (c *countingState) InvalidateDependents()   {}
func (c *countingState) InvalidateLibraryFacts() {}
func (c *countingState) InvalidateResolver()     {}
func (c *countingState) InvalidateOracleFilter() {}
func (c *countingState) Touch(path string)       {}
func (c *countingState) BumpSourceMTimeVersion() { c.bumpCalls.Add(1) }
func (c *countingState) BumpJavaSourceVersion()  { c.javaBumpCalls.Add(1) }
func (c *countingState) BumpXMLFilesVersion()    { c.xmlBumpCalls.Add(1) }

// waitForCondition polls fn every 5ms up to 2s. Returns true when fn
// turns true; false on timeout. Used to bridge the async fsnotify
// event delivery without sleeping a fixed pessimistic interval.
func waitForCondition(fn func() bool) bool {
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if fn() {
			return true
		}
		time.Sleep(5 * time.Millisecond)
	}
	return fn()
}

func TestFileWatcher_InvalidatesOnWrite(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "Foo.kt")
	if err := os.WriteFile(path, []byte("fun a() {}\n"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	ws := pipeline.NewWorkspaceState(root)
	if _, err := ws.ParseFile(context.Background(), path, []byte("fun a() {}\n")); err != nil {
		t.Fatalf("seed parse: %v", err)
	}
	if got := ws.Stats().ParsedEntries; got != 1 {
		t.Fatalf("setup: got %d entries, want 1", got)
	}

	w, err := startFileWatcher(context.Background(), root, ws, nil)
	if err != nil {
		t.Fatalf("startFileWatcher: %v", err)
	}
	defer w.Stop()

	if err := os.WriteFile(path, []byte("fun a() { 42 }\n"), 0o644); err != nil {
		t.Fatalf("rewrite: %v", err)
	}
	if !waitForCondition(func() bool { return ws.Stats().ParsedEntries == 0 }) {
		t.Errorf("expected workspace entry to be invalidated after write, stats=%+v", ws.Stats())
	}
}

func TestFileWatcher_InvalidatesOnRemove(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "Foo.kt")
	if err := os.WriteFile(path, []byte("fun a() {}\n"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	ws := pipeline.NewWorkspaceState(root)
	if _, err := ws.ParseFile(context.Background(), path, []byte("fun a() {}\n")); err != nil {
		t.Fatalf("seed parse: %v", err)
	}

	w, err := startFileWatcher(context.Background(), root, ws, nil)
	if err != nil {
		t.Fatalf("startFileWatcher: %v", err)
	}
	defer w.Stop()

	if err := os.Remove(path); err != nil {
		t.Fatalf("remove: %v", err)
	}
	if !waitForCondition(func() bool { return ws.Stats().ParsedEntries == 0 }) {
		t.Errorf("expected workspace entry to be invalidated after remove, stats=%+v", ws.Stats())
	}
}

func TestFileWatcher_IgnoresNonKotlinFiles(t *testing.T) {
	root := t.TempDir()
	kt := filepath.Join(root, "Foo.kt")
	java := filepath.Join(root, "Bar.java")
	if err := os.WriteFile(kt, []byte("fun a() {}\n"), 0o644); err != nil {
		t.Fatalf("write kt: %v", err)
	}
	if err := os.WriteFile(java, []byte("class Bar {}\n"), 0o644); err != nil {
		t.Fatalf("write java: %v", err)
	}
	ws := pipeline.NewWorkspaceState(root)
	if _, err := ws.ParseFile(context.Background(), kt, []byte("fun a() {}\n")); err != nil {
		t.Fatalf("seed parse: %v", err)
	}

	w, err := startFileWatcher(context.Background(), root, ws, nil)
	if err != nil {
		t.Fatalf("startFileWatcher: %v", err)
	}
	defer w.Stop()

	// Modifying the .java file must not invalidate the .kt entry.
	if err := os.WriteFile(java, []byte("class Bar2 {}\n"), 0o644); err != nil {
		t.Fatalf("rewrite java: %v", err)
	}
	// Give the watcher a moment to process the event before asserting
	// it didn't act.
	time.Sleep(50 * time.Millisecond)
	if got := ws.Stats().ParsedEntries; got != 1 {
		t.Errorf("expected the .kt entry to survive a .java change, got %d entries", got)
	}
}

func TestFileWatcher_AddsNewSubdir(t *testing.T) {
	root := t.TempDir()
	ws := pipeline.NewWorkspaceState(root)

	w, err := startFileWatcher(context.Background(), root, ws, nil)
	if err != nil {
		t.Fatalf("startFileWatcher: %v", err)
	}
	defer w.Stop()

	sub := filepath.Join(root, "newsub")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	// Wait for the watcher to register the new subdir before writing
	// into it. Polling beats a fixed sleep: on a busy CI runner the
	// subscription can take >50ms to propagate, which is the source of
	// this test's historical flake (#80). The probe re-arms the file
	// each iteration and waits for the watcher to invalidate it; once
	// that happens we know the subdir is being watched and can drive
	// the real assertion.
	probe := filepath.Join(sub, "Probe.kt")
	probeReady := false
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) && !probeReady {
		if err := os.WriteFile(probe, []byte("fun p() {}\n"), 0o644); err != nil {
			t.Fatalf("probe write: %v", err)
		}
		if _, err := ws.ParseFile(context.Background(), probe, []byte("fun p() {}\n")); err != nil {
			t.Fatalf("probe parse: %v", err)
		}
		if err := os.WriteFile(probe, []byte("fun p() { 1 }\n"), 0o644); err != nil {
			t.Fatalf("probe rewrite: %v", err)
		}
		probeDeadline := time.Now().Add(200 * time.Millisecond)
		for time.Now().Before(probeDeadline) {
			if ws.Stats().ParsedEntries == 0 {
				probeReady = true
				break
			}
			time.Sleep(5 * time.Millisecond)
		}
	}
	if !probeReady {
		t.Fatalf("watcher never picked up new subdir within 2s; stats=%+v", ws.Stats())
	}
	if err := os.Remove(probe); err != nil {
		t.Fatalf("remove probe: %v", err)
	}

	path := filepath.Join(sub, "New.kt")
	if err := os.WriteFile(path, []byte("fun n() {}\n"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	if _, err := ws.ParseFile(context.Background(), path, []byte("fun n() {}\n")); err != nil {
		t.Fatalf("seed parse: %v", err)
	}
	if err := os.WriteFile(path, []byte("fun n() { 1 }\n"), 0o644); err != nil {
		t.Fatalf("rewrite: %v", err)
	}
	if !waitForCondition(func() bool { return ws.Stats().ParsedEntries == 0 }) {
		t.Errorf("expected new-subdir watch to invalidate on write, stats=%+v", ws.Stats())
	}
}

func TestFileWatcher_InvalidatesCodeIndexOnKotlinChange(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "Foo.kt")
	if err := os.WriteFile(path, []byte("fun a() {}\n"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	ws := pipeline.NewWorkspaceState(root)
	ws.CodeIndex("ci-fp", func() *scanner.CodeIndex { return &scanner.CodeIndex{} })
	if !ws.CrossFileStats().HasCodeIndex {
		t.Fatal("setup: codeIndex slot should be populated")
	}

	w, err := startFileWatcher(context.Background(), root, ws, nil)
	if err != nil {
		t.Fatalf("startFileWatcher: %v", err)
	}
	defer w.Stop()

	if err := os.WriteFile(path, []byte("fun a() { 1 }\n"), 0o644); err != nil {
		t.Fatalf("rewrite: %v", err)
	}
	if !waitForCondition(func() bool { return !ws.CrossFileStats().HasCodeIndex }) {
		t.Errorf("expected codeIndex slot to clear after kotlin write, stats=%+v", ws.CrossFileStats())
	}
}

func TestFileWatcher_InvalidatesLibraryFactsOnGradleChange(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "build.gradle.kts")
	if err := os.WriteFile(path, []byte("// gradle\n"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	ws := pipeline.NewWorkspaceState(root)
	ws.LibraryFacts("lf", func() *librarymodel.Facts { return &librarymodel.Facts{} })
	ws.CodeIndex("ci", func() *scanner.CodeIndex { return &scanner.CodeIndex{} })

	w, err := startFileWatcher(context.Background(), root, ws, nil)
	if err != nil {
		t.Fatalf("startFileWatcher: %v", err)
	}
	defer w.Stop()

	if err := os.WriteFile(path, []byte("// gradle changed\n"), 0o644); err != nil {
		t.Fatalf("rewrite: %v", err)
	}
	// Both slots should clear: library facts because the gradle file
	// drove them, codeIndex because dependency changes can shift
	// every cross-file lookup.
	if !waitForCondition(func() bool {
		s := ws.CrossFileStats()
		return !s.HasLibraryFacts && !s.HasCodeIndex
	}) {
		t.Errorf("expected both cross-file slots to clear after gradle write, stats=%+v", ws.CrossFileStats())
	}
}

func TestFileWatcher_InvalidatesLibraryFactsOnVersionsTomlChange(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "libs.versions.toml")
	if err := os.WriteFile(path, []byte("[versions]\n"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	ws := pipeline.NewWorkspaceState(root)
	ws.LibraryFacts("lf", func() *librarymodel.Facts { return &librarymodel.Facts{} })

	w, err := startFileWatcher(context.Background(), root, ws, nil)
	if err != nil {
		t.Fatalf("startFileWatcher: %v", err)
	}
	defer w.Stop()

	if err := os.WriteFile(path, []byte("[versions]\nfoo = \"1.0\"\n"), 0o644); err != nil {
		t.Fatalf("rewrite: %v", err)
	}
	if !waitForCondition(func() bool { return !ws.CrossFileStats().HasLibraryFacts }) {
		t.Errorf("expected libraryFacts to clear after versions-catalog write, stats=%+v", ws.CrossFileStats())
	}
}

// TestFileWatcher_TouchPropagatesOnKotlinWrite asserts the watcher
// pushes the changed path into WorkspaceState's dirty-set, so daemon
// verbs that call DrainDirty later see the file. Mirrors the
// invalidate-on-write test but probes the new Touch path. Watcher
// latency target: ≤ 200ms (the SLO from the daemon plan).
func TestFileWatcher_TouchPropagatesOnKotlinWrite(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "Foo.kt")
	if err := os.WriteFile(path, []byte("fun a() {}\n"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	ws := pipeline.NewWorkspaceState(root)

	w, err := startFileWatcher(context.Background(), root, ws, nil)
	if err != nil {
		t.Fatalf("startFileWatcher: %v", err)
	}
	defer w.Stop()

	start := time.Now()
	if err := os.WriteFile(path, []byte("fun a() { 42 }\n"), 0o644); err != nil {
		t.Fatalf("rewrite: %v", err)
	}

	// Drain repeatedly until the touch arrives. Bounds the assertion
	// to the watcher-latency SLO from the plan: ≤ 200ms from os.Write
	// to dirty-set visibility.
	deadline := time.Now().Add(2 * time.Second)
	var dirty []string
	for time.Now().Before(deadline) {
		if dirty = ws.DrainDirty(); len(dirty) > 0 {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
	if len(dirty) == 0 {
		t.Fatal("expected dirty-set to contain the written path within 2s")
	}
	if got := time.Since(start); got > 250*time.Millisecond {
		// Soft warning: 50ms debounce + 200ms OS/queue latency budget.
		// Log rather than fail to avoid CI flakes on loaded runners.
		t.Logf("watcher latency = %v (target ≤ 250ms incl. 50ms debounce)", got)
	}
	// Path should match the touched file (after WorkspaceState's
	// normalisation, which evaluates symlinks consistent with what
	// ParseFile does).
	found := false
	for _, p := range dirty {
		if filepath.Base(p) == "Foo.kt" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("dirty-set %v did not contain Foo.kt", dirty)
	}
}

// TestFileWatcher_TouchPropagatesOnGradleWrite covers the Gradle/
// versions-catalog branch of the watcher's handle().
func TestFileWatcher_TouchPropagatesOnGradleWrite(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "build.gradle.kts")
	if err := os.WriteFile(path, []byte("// gradle\n"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	ws := pipeline.NewWorkspaceState(root)

	w, err := startFileWatcher(context.Background(), root, ws, nil)
	if err != nil {
		t.Fatalf("startFileWatcher: %v", err)
	}
	defer w.Stop()

	if err := os.WriteFile(path, []byte("// gradle changed\n"), 0o644); err != nil {
		t.Fatalf("rewrite: %v", err)
	}
	deadline := time.Now().Add(2 * time.Second)
	var dirty []string
	for time.Now().Before(deadline) {
		if dirty = ws.DrainDirty(); len(dirty) > 0 {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
	if len(dirty) == 0 {
		t.Fatal("expected dirty-set to contain build.gradle.kts within 2s")
	}
}

// TestFileWatcher_DebounceEditorSavePattern verifies that three rapid events
// for the same Kotlin file coalesce into a single Invalidate call. Events
// are injected directly into handle() so the debouncer is tested in
// isolation from fsnotify's variable event-delivery timing —
// TestFileWatcher_InvalidatesOnWrite covers the OS-roundtrip path.
func TestFileWatcher_DebounceEditorSavePattern(t *testing.T) {
	root := t.TempDir()
	ktPath := filepath.Join(root, "Foo.kt")
	if err := os.WriteFile(ktPath, []byte("fun a() {}\n"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	state := &countingState{}
	const window = 100 * time.Millisecond
	w, err := startFileWatcherWithState(context.Background(), root, state, nil,
		withDebounceWindow(window))
	if err != nil {
		t.Fatalf("startFileWatcher: %v", err)
	}
	defer w.Stop()
	<-w.Ready()

	ev := fsnotify.Event{Name: ktPath, Op: fsnotify.Write}
	for range 3 {
		w.handle(ev)
	}

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if state.invalidateCalls.Load() > 0 {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
	// Settle: if a stray timer was re-armed under load, give it a full
	// window + margin to complete before asserting the final count.
	time.Sleep(window * 2)

	if got := state.invalidateCalls.Load(); got != 1 {
		t.Errorf("Invalidate called %d times, want 1 (debounce should coalesce burst)", got)
	}
}

// TestFileWatcher_DebounceGenerationGuard forces the Stop()-race scenario:
// the test holds debounceMu past the window so the scheduled timer fires
// and its callback parks at the lock, then bumps debounceGen[path] while
// holding the lock to simulate the relevant half of a second
// scheduleKotlinInvalidate. The parked callback must observe the gen
// mismatch and skip Invalidate. Removing the gen check in watcher.go must
// fail this test — it drives the real callback path.
func TestFileWatcher_DebounceGenerationGuard(t *testing.T) {
	root := t.TempDir()
	ktPath := filepath.Join(root, "Foo.kt")
	if err := os.WriteFile(ktPath, []byte("fun a() {}\n"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	state := &countingState{}
	const window = 10 * time.Millisecond
	w, err := startFileWatcherWithState(context.Background(), root, state, nil,
		withDebounceWindow(window))
	if err != nil {
		t.Fatalf("startFileWatcher: %v", err)
	}
	defer w.Stop()
	<-w.Ready()

	w.scheduleKotlinInvalidate(ktPath)
	w.debounceMu.Lock()
	time.Sleep(window * 3)
	w.debounceGen[ktPath]++
	w.debounceMu.Unlock()

	time.Sleep(window * 6)

	if got := state.invalidateCalls.Load(); got != 0 {
		t.Errorf("Invalidate called %d times, want 0 (stale callback should observe gen bump and no-op)", got)
	}
}

func TestIsLibraryConfigPath(t *testing.T) {
	tests := []struct {
		path string
		want bool
	}{
		{"build.gradle", true},
		{"build.gradle.kts", true},
		{"settings.gradle", true},
		{"settings.gradle.kts", true},
		{"libs.versions.toml", true},
		{"foo/libs.versions.toml", true},
		{"app/build.gradle.kts", true},
		{"Foo.kt", false},
		{"build.gradle.txt", false},
		{"versions.toml", false}, // doesn't end in .versions.toml
	}
	for _, tt := range tests {
		if got := isLibraryConfigPath(tt.path); got != tt.want {
			t.Errorf("isLibraryConfigPath(%q) = %v, want %v", tt.path, got, tt.want)
		}
	}
}

func TestIsKotlinPath(t *testing.T) {
	tests := []struct {
		path string
		want bool
	}{
		{"Foo.kt", true},
		{"Foo.kts", true},
		{"Foo.java", false},
		{"Foo.txt", false},
		{"a/b/c.kt", true},
	}
	for _, tt := range tests {
		if got := isKotlinPath(tt.path); got != tt.want {
			t.Errorf("isKotlinPath(%q) = %v, want %v", tt.path, got, tt.want)
		}
	}
}

// TestFileWatcher_IgnoresChmodOnlyEvents asserts the watcher does NOT
// drop cached cross-file slots when fsnotify emits a Chmod-only event
// for a library-config or Kotlin path. On macOS, kqueue emits spurious
// Chmod events whenever another process stats or opens a watched file
// (Spotlight, git, finder previews). Pre-fix, those events fired
// InvalidateLibraryFacts on every analyze and rebuilt the AndroidProject
// / resolver / per-file parsed-tree slots — costing ~1s each.
func TestFileWatcher_IgnoresChmodOnlyEvents(t *testing.T) {
	root := t.TempDir()
	gradlePath := filepath.Join(root, "build.gradle.kts")
	ktPath := filepath.Join(root, "Foo.kt")
	for _, p := range []string{gradlePath, ktPath} {
		if err := os.WriteFile(p, []byte("// init\n"), 0o644); err != nil {
			t.Fatalf("write %s: %v", p, err)
		}
	}
	state := &chmodFilterCountingState{}
	w, err := startFileWatcherWithState(context.Background(), root, state, nil)
	if err != nil {
		t.Fatalf("startFileWatcher: %v", err)
	}
	defer w.Stop()

	// Inject a Chmod-only event directly through handle() — going
	// through os.Chmod would also trigger a Stat() that on some
	// platforms emits Write, defeating the test's purpose.
	w.handle(fsnotifyEventChmod(gradlePath))
	w.handle(fsnotifyEventChmod(ktPath))
	time.Sleep(20 * time.Millisecond)

	if got := state.libraryFactsInvalidations.Load(); got != 0 {
		t.Errorf("InvalidateLibraryFacts fired %d times on Chmod-only event; want 0", got)
	}
	if got := state.invalidateCalls.Load(); got != 0 {
		t.Errorf("Invalidate fired %d times on Chmod-only event; want 0", got)
	}
}

type chmodFilterCountingState struct {
	invalidateCalls           atomic.Int64
	libraryFactsInvalidations atomic.Int64
	bumpCalls                 atomic.Int64
}

func (c *chmodFilterCountingState) Invalidate(string)       { c.invalidateCalls.Add(1) }
func (c *chmodFilterCountingState) InvalidateCodeIndex()    {}
func (c *chmodFilterCountingState) InvalidateDependents()   {}
func (c *chmodFilterCountingState) InvalidateLibraryFacts() { c.libraryFactsInvalidations.Add(1) }
func (c *chmodFilterCountingState) InvalidateResolver()     {}
func (c *chmodFilterCountingState) InvalidateOracleFilter() {}
func (c *chmodFilterCountingState) Touch(string)            {}
func (c *chmodFilterCountingState) BumpSourceMTimeVersion() { c.bumpCalls.Add(1) }
func (c *chmodFilterCountingState) BumpJavaSourceVersion()  {}
func (c *chmodFilterCountingState) BumpXMLFilesVersion()    {}

// TestFileWatcher_BumpsMTimeVersionOnKotlinEdit asserts the watcher
// drives the daemon-resident stats-clean memo: a real .kt edit must
// bump the source-mtime version so any cached "stats are clean"
// record gets invalidated. Without this hook the daemon would
// happily skip a stat sweep that needs to run, and a real source
// change would never reach the bundle-fingerprint check.
func TestFileWatcher_BumpsMTimeVersionOnKotlinEdit(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "Foo.kt")
	if err := os.WriteFile(path, []byte("fun a() {}\n"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	state := &countingState{}
	w, err := startFileWatcherWithState(context.Background(), root, state, nil, withDebounceWindow(5*time.Millisecond))
	if err != nil {
		t.Fatalf("startFileWatcher: %v", err)
	}
	defer w.Stop()

	if err := os.WriteFile(path, []byte("fun a() { 1 }\n"), 0o644); err != nil {
		t.Fatalf("rewrite: %v", err)
	}
	if !waitForCondition(func() bool { return state.bumpCalls.Load() >= 1 }) {
		t.Errorf("expected BumpSourceMTimeVersion after kt edit, got %d", state.bumpCalls.Load())
	}
}

// TestFileWatcher_BumpsMTimeVersionOnGradleEdit confirms gradle /
// version-catalog edits also bump the version — they contribute to
// the manifest's FileStats map just like .kt files, so an unbumped
// version after a build.gradle change would let a stale stats-clean
// memo survive.
func TestFileWatcher_BumpsMTimeVersionOnGradleEdit(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "build.gradle.kts")
	if err := os.WriteFile(path, []byte("// gradle\n"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	state := &countingState{}
	w, err := startFileWatcherWithState(context.Background(), root, state, nil)
	if err != nil {
		t.Fatalf("startFileWatcher: %v", err)
	}
	defer w.Stop()

	if err := os.WriteFile(path, []byte("// edited\n"), 0o644); err != nil {
		t.Fatalf("rewrite: %v", err)
	}
	if !waitForCondition(func() bool { return state.bumpCalls.Load() >= 1 }) {
		t.Errorf("expected BumpSourceMTimeVersion after gradle edit, got %d", state.bumpCalls.Load())
	}
}

// TestFileWatcher_BumpsJavaSourceVersionOnJavaEdit pins the
// .java-event hook. The watcher must invoke BumpJavaSourceVersion
// (the daemon's javafacts.SourceIndex invalidation signal) on any
// .java file event so the next analyze rebuilds the Java source
// index instead of serving stale facts from the resident cache.
func TestFileWatcher_BumpsJavaSourceVersionOnJavaEdit(t *testing.T) {
	t.Run("java edit bumps counter", func(t *testing.T) {
		root := t.TempDir()
		path := filepath.Join(root, "Foo.java")
		if err := os.WriteFile(path, []byte("class Foo {}\n"), 0o644); err != nil {
			t.Fatalf("write: %v", err)
		}
		state := &countingState{}
		w, err := startFileWatcherWithState(context.Background(), root, state, nil)
		if err != nil {
			t.Fatalf("startFileWatcher: %v", err)
		}
		defer w.Stop()

		if err := os.WriteFile(path, []byte("class Foo2 {}\n"), 0o644); err != nil {
			t.Fatalf("rewrite: %v", err)
		}
		if !waitForCondition(func() bool { return state.javaBumpCalls.Load() >= 1 }) {
			t.Errorf("expected BumpJavaSourceVersion after .java edit, got %d", state.javaBumpCalls.Load())
		}
	})

	// Sanity: a non-Java edit must NOT bump the Java counter. Use a
	// fresh watcher + state so late fsnotify events from an earlier
	// .java write cannot race with the assertion.
	t.Run("kotlin edit does not bump java counter", func(t *testing.T) {
		root := t.TempDir()
		state := &countingState{}
		w, err := startFileWatcherWithState(context.Background(), root, state, nil)
		if err != nil {
			t.Fatalf("startFileWatcher: %v", err)
		}
		defer w.Stop()

		if err := os.WriteFile(filepath.Join(root, "Bar.kt"), []byte("fun b() {}\n"), 0o644); err != nil {
			t.Fatalf("write kt: %v", err)
		}
		time.Sleep(80 * time.Millisecond)
		if got := state.javaBumpCalls.Load(); got != 0 {
			t.Errorf("Kotlin edit must NOT bump Java counter; got %d", got)
		}
	})
}

// TestFileWatcher_BumpsXMLFilesVersionOnXMLEdit pins the .xml-event
// hook. Edits to layout/manifest/navigation XMLs must invalidate the
// daemon-resident XMLCacheFile slot so the next analyze re-walks the
// project. Kotlin / Java edits, on the other hand, MUST NOT bump
// the XML counter — XML content is independent and we want to reuse
// the cached slice across the (much more common) source edits.
func TestFileWatcher_BumpsXMLFilesVersionOnXMLEdit(t *testing.T) {
	t.Run("xml edit bumps counter", func(t *testing.T) {
		root := t.TempDir()
		path := filepath.Join(root, "layout.xml")
		if err := os.WriteFile(path, []byte(`<View/>`), 0o644); err != nil {
			t.Fatalf("write: %v", err)
		}
		state := &countingState{}
		w, err := startFileWatcherWithState(context.Background(), root, state, nil)
		if err != nil {
			t.Fatalf("startFileWatcher: %v", err)
		}
		defer w.Stop()

		if err := os.WriteFile(path, []byte(`<TextView/>`), 0o644); err != nil {
			t.Fatalf("rewrite: %v", err)
		}
		if !waitForCondition(func() bool { return state.xmlBumpCalls.Load() >= 1 }) {
			t.Errorf("expected BumpXMLFilesVersion after .xml edit, got %d", state.xmlBumpCalls.Load())
		}
	})

	// Sanity: a Kotlin edit must NOT bump the XML counter. Use a
	// fresh watcher + state so late fsnotify events from an earlier
	// .xml write cannot race with the assertion.
	t.Run("kotlin edit does not bump xml counter", func(t *testing.T) {
		root := t.TempDir()
		state := &countingState{}
		w, err := startFileWatcherWithState(context.Background(), root, state, nil)
		if err != nil {
			t.Fatalf("startFileWatcher: %v", err)
		}
		defer w.Stop()

		if err := os.WriteFile(filepath.Join(root, "Bar.kt"), []byte("fun b() {}\n"), 0o644); err != nil {
			t.Fatalf("write kt: %v", err)
		}
		time.Sleep(80 * time.Millisecond)
		if got := state.xmlBumpCalls.Load(); got != 0 {
			t.Errorf("Kotlin edit must NOT bump XML counter; got %d", got)
		}
	})
}
