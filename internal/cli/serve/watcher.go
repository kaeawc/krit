package serve

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"

	"github.com/kaeawc/krit/internal/config"
	"github.com/kaeawc/krit/internal/diag"
	"github.com/kaeawc/krit/internal/pipeline"
)

// watcherState is the subset of pipeline.WorkspaceState the file watcher
// calls. The interface lets tests substitute a counting fake.
type watcherState interface {
	Invalidate(path string)
	InvalidateCodeIndex()
	InvalidateDependents()
	InvalidateLibraryFacts()
	InvalidateResolver()
	InvalidateOracleFilter()
	Touch(path string)
}

const defaultDebounceWindow = 50 * time.Millisecond

// fileWatcher pushes filesystem-change events into a daemon's
// WorkspaceState invalidation API. It watches the project root
// recursively (one watch per directory, populated on creation) and
// drops cache entries for any .kt / .kts file the OS reports
// changed, removed, or renamed.
//
// The watcher is best-effort: a missed event causes a stale parse
// at worst, which the next request's content-hash compare in
// WorkspaceState catches. Errors are logged via a Reporter and never
// crash the daemon.
type fileWatcher struct {
	w        *fsnotify.Watcher
	root     string
	state    watcherState
	reporter *diag.Reporter
	// onConfigChange fires when the watcher observes an edit to a
	// krit.yml / .krit.yml file. Daemon callers wire this to
	// daemonState.InvalidateConfig so the next analyze-project verb
	// reloads. Nil is allowed (no daemon, no callback).
	onConfigChange func()

	closeOnce sync.Once
	done      chan struct{}
	// ready closes once the initial recursive walk has finished. The
	// walk runs asynchronously after startFileWatcher returns so the
	// daemon's startup latency is bounded by NewWatcher() + a single
	// root Add() rather than by a full project tree walk. Callers who
	// need a fully primed watcher (tests on sub-dirs, e.g.) can wait
	// on Ready() — production callers don't need to: events arriving
	// before the walk completes for a still-unwatched subtree are
	// covered by the watcher's documented best-effort contract.
	ready chan struct{}

	// debounce coalesces rapid Write+Write+Chmod sequences that editors
	// emit on a single logical save. Each path gets its own sliding timer;
	// events within debounceWindow of each other are collapsed into one
	// Invalidate+Touch call.
	debounceMu     sync.Mutex
	debounce       map[string]*time.Timer
	debounceWindow time.Duration
}

// startFileWatcher returns a started watcher rooted at root. Callers
// must call Stop to release the underlying fsnotify resources.
// Returns nil + error when the watcher couldn't be created (e.g. on
// platforms without inotify/kqueue or when the root doesn't exist).
func startFileWatcher(ctx context.Context, root string, state *pipeline.WorkspaceState, reporter *diag.Reporter, opts ...watcherOption) (*fileWatcher, error) {
	return startFileWatcherWithState(ctx, root, state, reporter, opts...)
}

func startFileWatcherWithState(ctx context.Context, root string, state watcherState, reporter *diag.Reporter, opts ...watcherOption) (*fileWatcher, error) {
	w, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}
	fw := &fileWatcher{
		w:              w,
		root:           root,
		state:          state,
		reporter:       reporter,
		done:           make(chan struct{}),
		ready:          make(chan struct{}),
		debounce:       make(map[string]*time.Timer),
		debounceWindow: defaultDebounceWindow,
	}
	for _, opt := range opts {
		opt(fw)
	}
	// Add the root synchronously so files written directly under it
	// are caught from t=0. The recursive descent runs in a goroutine —
	// on a 60k-file repo a sync walk costs multiple seconds, which is
	// the daemon's first-call latency budget.
	if err := w.Add(root); err != nil {
		_ = w.Close()
		return nil, err
	}
	go fw.populate()
	go fw.run(ctx)
	return fw, nil
}

// populate walks the project tree and registers each directory with
// the underlying watcher. Closes fw.ready when done. Errors are
// logged via the reporter — a partial registration just means the
// usual best-effort fallback (next request rehashes contents).
func (fw *fileWatcher) populate() {
	defer close(fw.ready)
	if err := fw.addRecursiveSkip(fw.root, true); err != nil {
		fw.warn("watch populate: %v\n", err)
	}
}

// Ready returns a channel that closes once the initial recursive
// walk has finished. Tests that exercise pre-existing subtrees can
// wait on it; production callers don't need to.
func (fw *fileWatcher) Ready() <-chan struct{} { return fw.ready }

// watcherOption tunes startFileWatcher. Variadic so callers without a
// daemonState (e.g. unit tests of the watcher itself) don't need to
// pass a no-op callback.
type watcherOption func(*fileWatcher)

// withConfigChangeCallback wires fw.onConfigChange. Used by serve.Run
// to flag the daemon's cached config stale on krit.yml edits.
func withConfigChangeCallback(fn func()) watcherOption {
	return func(fw *fileWatcher) { fw.onConfigChange = fn }
}

// withDebounceWindow overrides the Kotlin-file debounce window. Used by
// tests that need a shorter window to avoid slow test runs.
func withDebounceWindow(d time.Duration) watcherOption {
	return func(fw *fileWatcher) { fw.debounceWindow = d }
}

// Stop releases the watcher. Safe to call multiple times.
func (fw *fileWatcher) Stop() {
	fw.closeOnce.Do(func() {
		_ = fw.w.Close()
		<-fw.done
	})
}

// run is the event loop. It exits when the watcher closes or ctx is
// cancelled.
func (fw *fileWatcher) run(ctx context.Context) {
	defer close(fw.done)
	for {
		select {
		case <-ctx.Done():
			return
		case event, ok := <-fw.w.Events:
			if !ok {
				return
			}
			fw.handle(event)
		case err, ok := <-fw.w.Errors:
			if !ok {
				return
			}
			fw.warn("watcher error: %v\n", err)
		}
	}
}

// handle dispatches a single fsnotify event. Newly created
// directories get a fresh watch so additions in subtrees don't slip
// past; .kt/.kts file events invalidate the per-file parse cache and
// the cross-file CodeIndex; Gradle / version-catalog edits also
// invalidate the cached LibraryFacts since they describe the
// project's dependency closure.
func (fw *fileWatcher) handle(ev fsnotify.Event) {
	if ev.Op&fsnotify.Create != 0 {
		if info, err := os.Stat(ev.Name); err == nil && info.IsDir() {
			if err := fw.addRecursive(ev.Name); err != nil {
				fw.warn("watch new dir %s: %v\n", ev.Name, err)
			}
			return
		}
	}
	if ev.Op&(fsnotify.Write|fsnotify.Create|fsnotify.Remove|fsnotify.Rename|fsnotify.Chmod) == 0 {
		return
	}
	// Library-config paths come first — build.gradle.kts also matches
	// isKotlinPath because of its extension, but it drives library
	// facts, not the per-file parse cache.
	switch {
	case isKritConfigPath(ev.Name):
		// krit.yml / .krit.yml drives rule selection. The cached
		// config on the daemon is now stale; the next analyze-project
		// call rebuilds.
		if fw.onConfigChange != nil {
			fw.onConfigChange()
		}
		fw.state.Touch(ev.Name)
	case isLibraryConfigPath(ev.Name):
		fw.state.InvalidateLibraryFacts()
		fw.state.InvalidateCodeIndex()
		fw.state.InvalidateDependents()
		// Touch the path so daemon verbs reporting "files changed
		// since last analyze" see Gradle/version-catalog edits.
		fw.state.Touch(ev.Name)
	case isKotlinPath(ev.Name):
		// Editors emit Write+Write+Chmod on a single logical save.
		// Coalesce within debounceWindow so only one Invalidate+Touch
		// fires per burst — the dirty-set's map semantics already dedup
		// Touch, but Invalidate and CodeIndex drops are not free.
		fw.scheduleKotlinInvalidate(ev.Name)
	}
}

// scheduleKotlinInvalidate coalesces rapid fsnotify events for path into a
// single Invalidate+Touch call fired after fw.debounceWindow of silence.
// Each new event within the window restarts the timer (sliding debounce).
func (fw *fileWatcher) scheduleKotlinInvalidate(path string) {
	fw.debounceMu.Lock()
	if t, ok := fw.debounce[path]; ok {
		t.Stop()
	}
	fw.debounce[path] = time.AfterFunc(fw.debounceWindow, func() {
		fw.debounceMu.Lock()
		delete(fw.debounce, path)
		fw.debounceMu.Unlock()
		fw.state.Invalidate(path)
		fw.state.InvalidateCodeIndex()
		fw.state.InvalidateDependents()
		fw.state.InvalidateResolver()
		fw.state.InvalidateOracleFilter()
		fw.state.Touch(path)
	})
	fw.debounceMu.Unlock()
}

// addRecursive walks dir and adds every directory to the watcher.
// Pruned dirs (.git, build, .gradle) are skipped: the daemon never
// analyses files under them, so receiving events for them just
// burns descriptors and CPU.
func (fw *fileWatcher) addRecursive(dir string) error {
	return fw.addRecursiveSkip(dir, false)
}

// addRecursiveSkip is addRecursive with a hook to skip the root Add —
// startFileWatcher adds the root synchronously and only the async
// descendant walk should skip re-adding it.
func (fw *fileWatcher) addRecursiveSkip(dir string, skipRoot bool) error {
	return filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			// Skip unreadable entries — the daemon shouldn't crash
			// because one subtree has bad permissions. Log so the
			// operator can fix it.
			fw.warn("walk %s: %v\n", path, err)
			return nil
		}
		if !info.IsDir() {
			return nil
		}
		base := filepath.Base(path)
		if isPrunedDir(base) && path != dir {
			return filepath.SkipDir
		}
		if skipRoot && path == dir {
			return nil
		}
		if err := fw.w.Add(path); err != nil {
			fw.warn("watch %s: %v\n", path, err)
		}
		return nil
	})
}

func (fw *fileWatcher) warn(format string, args ...any) {
	if fw.reporter == nil {
		return
	}
	fw.reporter.Warnf(format, args...)
}

// isKotlinPath reports whether the path's basename ends in .kt or
// .kts. Mirrors the precommit/cli filter.
func isKotlinPath(p string) bool {
	return strings.HasSuffix(p, ".kt") || strings.HasSuffix(p, ".kts")
}

// isKritConfigPath reports whether the path is the krit.yml /
// .krit.yml the daemon honours for rule selection. Editing one
// invalidates the daemon's cached *config.Config.
func isKritConfigPath(p string) bool {
	base := filepath.Base(p)
	for _, name := range config.Filenames {
		if base == name {
			return true
		}
	}
	return false
}

// isLibraryConfigPath reports whether the path is a Gradle build
// script or a version catalog whose contents drive
// librarymodel.Facts. Editing one of these flips the entire library
// fingerprint, so the cached LibraryFacts must drop.
func isLibraryConfigPath(p string) bool {
	base := filepath.Base(p)
	switch base {
	case "build.gradle", "build.gradle.kts",
		"settings.gradle", "settings.gradle.kts":
		return true
	}
	return strings.HasSuffix(base, ".versions.toml")
}

// isPrunedDir lists directory basenames the watcher refuses to
// recurse into. Keep aligned with internal/fileignore.DefaultPrunedDir.
func isPrunedDir(name string) bool {
	switch name {
	case ".git", "build", ".gradle", "node_modules", ".idea", ".krit":
		return true
	}
	return false
}
