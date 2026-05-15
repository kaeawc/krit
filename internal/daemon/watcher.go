package daemon

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/fsnotify/fsnotify"

	"github.com/kaeawc/krit/internal/config"
	"github.com/kaeawc/krit/internal/fileignore"
	"github.com/kaeawc/krit/internal/pipeline"
)

// Logger is the minimal logging surface the watcher needs. *diag.Reporter
// satisfies it via Warnf; tests can substitute a counting fake. A nil
// Logger silences output.
type Logger interface {
	Warnf(format string, args ...any)
}

// watcherState is the subset of pipeline.WorkspaceState the watcher
// drives. The interface keeps tests fast (no real workspace) and lets
// the daemon plug in a thin facade if it wraps WorkspaceState in
// additional bookkeeping later.
type watcherState interface {
	Invalidate(path string)
	InvalidateAll()
	InvalidateCodeIndex()
	InvalidateDependents()
	InvalidateLibraryFacts()
	InvalidateResolver()
	InvalidateOracleFilter()
	Touch(path string)
}

// WatcherEvent is a notification emitted on Watcher.Events for changes
// the daemon must react to beyond cache invalidation: a config reload
// or a self-binary swap requiring a restart.
type WatcherEvent int

const (
	// EventConfigReload signals that krit.yml / .krit.yml changed. The
	// daemon should drop its cached *config.Config and re-read on the
	// next analyze.
	EventConfigReload WatcherEvent = iota + 1
	// EventShutdownRequest signals that the krit binary the daemon
	// loaded has been overwritten on disk. The CLI restarts the
	// daemon after observing the shutdown.
	EventShutdownRequest
)

const (
	defaultDebounceWindow = 50 * time.Millisecond
	defaultSweepInterval  = 30 * time.Second
)

// Watcher pushes filesystem-change events into a WorkspaceState's
// invalidation API and surfaces config / self-binary events on the
// Events channel. It watches the repo recursively (one fsnotify watch
// per directory, populated asynchronously after Start returns).
//
// The watcher is best-effort: a missed fsnotify event causes at worst
// a stale parse, which the periodic mtime sweep catches within
// SweepInterval.
type Watcher struct {
	ws      watcherState
	repoDir string
	log     Logger

	binaryPath string

	w *fsnotify.Watcher

	debounceWindow time.Duration
	sweepInterval  time.Duration

	// onAndroidProjectChange fires when build.gradle / settings.gradle /
	// version-catalog edits land. The daemon clears
	// Session.AndroidProject in response. Nil is fine (no callback).
	onAndroidProjectChange func()
	// onResourceIndexChange fires when AndroidManifest.xml or res/**/*.xml
	// edits land. The daemon clears its android.ResourceIndexCache.
	onResourceIndexChange func()

	// Events is the non-buffered notification channel. ConfigReload
	// and ShutdownRequest are delivered here; consumers select on it
	// with the daemon's main loop. A nil channel is fine (no consumer)
	// — sends are non-blocking.
	Events chan WatcherEvent

	debounceMu sync.Mutex
	debounce   map[string]*time.Timer

	// tracked is the union of paths the watcher believes are clean —
	// successfully parsed by the daemon and not yet observed dirty.
	// The mtime sweep re-stats each entry every sweepInterval; any
	// path whose mtime advanced past its tracked value is treated as
	// a missed fsnotify event.
	trackedMu sync.Mutex
	tracked   map[string]int64

	// counters for tests / observability.
	codeIndexInvalidations atomic.Int64
	sweepCatches           atomic.Int64

	stopOnce sync.Once
	started  atomic.Bool
	stopped  chan struct{}
	stopErr  error
}

// WatcherOption tunes a Watcher.
type WatcherOption func(*Watcher)

// WithDebounceWindow overrides the coalescing window. Default 50 ms.
func WithDebounceWindow(d time.Duration) WatcherOption {
	return func(w *Watcher) { w.debounceWindow = d }
}

// WithSweepInterval overrides the mtime sweep interval. Default 30 s.
// Set to zero to disable the sweep (tests only — production must keep
// it on as the missed-event safety net).
func WithSweepInterval(d time.Duration) WatcherOption {
	return func(w *Watcher) { w.sweepInterval = d }
}

// WithBinaryPath sets the krit binary path to watch. A write/rename/
// remove on this path emits EventShutdownRequest. Empty disables the
// self-binary watch.
func WithBinaryPath(p string) WatcherOption {
	return func(w *Watcher) { w.binaryPath = p }
}

// WithAndroidProjectCallback wires the build.gradle / version-catalog
// edit hook so the daemon can clear Session.AndroidProject.
func WithAndroidProjectCallback(fn func()) WatcherOption {
	return func(w *Watcher) { w.onAndroidProjectChange = fn }
}

// WithResourceIndexCallback wires the AndroidManifest / res/**/*.xml
// edit hook so the daemon can clear its android.ResourceIndexCache.
func WithResourceIndexCallback(fn func()) WatcherOption {
	return func(w *Watcher) { w.onResourceIndexChange = fn }
}

// NewWatcher constructs a Watcher bound to ws and repoDir. The
// underlying fsnotify handle is created here; call Start to begin the
// event loop and recursive directory walk. log may be nil.
func NewWatcher(ws *pipeline.WorkspaceState, repoDir string, log Logger, opts ...WatcherOption) (*Watcher, error) {
	return newWatcherWithState(ws, repoDir, log, opts...)
}

func newWatcherWithState(state watcherState, repoDir string, log Logger, opts ...WatcherOption) (*Watcher, error) {
	if repoDir == "" {
		return nil, errors.New("watcher: empty repoDir")
	}
	fw, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}
	w := &Watcher{
		ws:             state,
		repoDir:        repoDir,
		log:            log,
		w:              fw,
		debounceWindow: defaultDebounceWindow,
		sweepInterval:  defaultSweepInterval,
		Events:         make(chan WatcherEvent, 8),
		debounce:       make(map[string]*time.Timer),
		tracked:        make(map[string]int64),
		stopped:        make(chan struct{}),
	}
	for _, opt := range opts {
		opt(w)
	}
	return w, nil
}

// Start begins the event loop. Returns after the root directory is
// registered; the rest of the tree is added asynchronously so startup
// latency stays bounded. Stops when ctx is cancelled or Stop is
// called.
func (w *Watcher) Start(ctx context.Context) error {
	if err := w.w.Add(w.repoDir); err != nil {
		_ = w.w.Close()
		return err
	}
	if w.binaryPath != "" {
		if err := w.w.Add(filepath.Dir(w.binaryPath)); err != nil {
			w.warn("watch binary dir %s: %v", filepath.Dir(w.binaryPath), err)
		}
	}
	w.started.Store(true)
	go w.populate()
	go w.run(ctx)
	if w.sweepInterval > 0 {
		go w.sweepLoop(ctx)
	}
	return nil
}

// Stop closes the underlying fsnotify watcher and waits for the event
// loop to exit. Safe to call multiple times and safe on a Watcher that
// was never Started.
func (w *Watcher) Stop() error {
	w.stopOnce.Do(func() {
		w.stopErr = w.w.Close()
		// Cancel any debounce timers so they don't fire on a torn-down
		// state after Stop returns.
		w.debounceMu.Lock()
		for k, t := range w.debounce {
			t.Stop()
			delete(w.debounce, k)
		}
		w.debounceMu.Unlock()
		if w.started.Load() {
			<-w.stopped
		} else {
			close(w.stopped)
		}
	})
	return w.stopErr
}

// CodeIndexInvalidations returns the number of times the watcher has
// called InvalidateCodeIndex. Used by the stress test to verify
// debounce coalescing.
func (w *Watcher) CodeIndexInvalidations() int64 { return w.codeIndexInvalidations.Load() }

// SweepCatches returns the number of missed-fsnotify events the mtime
// sweep has recovered.
func (w *Watcher) SweepCatches() int64 { return w.sweepCatches.Load() }

// Track records that the daemon believes path is clean — its mtime
// will be snapshot for the sweep loop to compare against. Called by
// the daemon whenever a file is parsed/analysed; the next sweep
// surfaces a stale entry if fsnotify missed an event.
func (w *Watcher) Track(path string) {
	info, err := os.Stat(path)
	if err != nil {
		return
	}
	key := filepath.Clean(path)
	w.trackedMu.Lock()
	w.tracked[key] = info.ModTime().UnixNano()
	w.trackedMu.Unlock()
}

// Untrack drops path from the tracked set. Called when the file
// disappears or the daemon stops caring about it.
func (w *Watcher) Untrack(path string) {
	key := filepath.Clean(path)
	w.trackedMu.Lock()
	delete(w.tracked, key)
	w.trackedMu.Unlock()
}

func (w *Watcher) populate() {
	if err := w.addRecursiveSkip(w.repoDir, true); err != nil {
		w.warn("watch populate: %v", err)
	}
}

func (w *Watcher) run(ctx context.Context) {
	defer close(w.stopped)
	for {
		select {
		case <-ctx.Done():
			return
		case ev, ok := <-w.w.Events:
			if !ok {
				return
			}
			w.handle(ev)
		case err, ok := <-w.w.Errors:
			if !ok {
				return
			}
			w.warn("watcher error: %v", err)
		}
	}
}

// handle dispatches a single fsnotify event.
func (w *Watcher) handle(ev fsnotify.Event) {
	if ev.Op&fsnotify.Create != 0 {
		if info, err := os.Stat(ev.Name); err == nil && info.IsDir() {
			if err := w.addRecursive(ev.Name); err != nil {
				w.warn("watch new dir %s: %v", ev.Name, err)
			}
			return
		}
	}
	if ev.Op&(fsnotify.Write|fsnotify.Create|fsnotify.Remove|fsnotify.Rename|fsnotify.Chmod) == 0 {
		return
	}
	w.route(ev.Name)
}

// route dispatches the path to the right invalidation bucket. Tests
// drive this directly to bypass fsnotify timing.
func (w *Watcher) route(path string) {
	switch {
	case w.binaryPath != "" && filepath.Clean(path) == filepath.Clean(w.binaryPath):
		w.emit(EventShutdownRequest)
	case isKritConfigPath(path):
		w.ws.InvalidateAll()
		w.ws.Touch(path)
		w.emit(EventConfigReload)
	case isLibraryConfigPath(path):
		w.ws.InvalidateLibraryFacts()
		w.ws.InvalidateCodeIndex()
		w.ws.InvalidateDependents()
		w.ws.Touch(path)
		if w.onAndroidProjectChange != nil {
			w.onAndroidProjectChange()
		}
	case isAndroidResourcePath(path):
		w.ws.Touch(path)
		if w.onResourceIndexChange != nil {
			w.onResourceIndexChange()
		}
	case isSourcePath(path):
		w.scheduleSourceInvalidate(path)
	}
}

func (w *Watcher) emit(e WatcherEvent) {
	if w.Events == nil {
		return
	}
	select {
	case w.Events <- e:
	default:
		// A blocked consumer must not silence shutdown signals; the
		// daemon's correctness depends on observing them.
		w.warn("watcher: dropped event %d (consumer not draining)", int(e))
	}
}

// scheduleSourceInvalidate coalesces rapid Write+Write+Chmod
// sequences (editors emit several events per save) into a single
// invalidation pass per path within debounceWindow.
func (w *Watcher) scheduleSourceInvalidate(path string) {
	w.debounceMu.Lock()
	if t, ok := w.debounce[path]; ok {
		t.Stop()
	}
	w.debounce[path] = time.AfterFunc(w.debounceWindow, func() {
		w.debounceMu.Lock()
		delete(w.debounce, path)
		w.debounceMu.Unlock()
		w.fireSourceInvalidate(path)
	})
	w.debounceMu.Unlock()
}

func (w *Watcher) fireSourceInvalidate(path string) {
	w.ws.Invalidate(path)
	w.ws.Touch(path)
	w.ws.InvalidateCodeIndex()
	w.ws.InvalidateResolver()
	w.ws.InvalidateDependents()
	w.ws.InvalidateOracleFilter()
	w.codeIndexInvalidations.Add(1)
	w.Untrack(path)
}

func (w *Watcher) addRecursive(dir string) error {
	return w.addRecursiveSkip(dir, false)
}

func (w *Watcher) addRecursiveSkip(dir string, skipRoot bool) error {
	return filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			w.warn("walk %s: %v", path, err)
			return nil
		}
		if !info.IsDir() {
			return nil
		}
		base := filepath.Base(path)
		if fileignore.DefaultPrunedDir(base) && path != dir {
			return filepath.SkipDir
		}
		if base == ".krit" && path != dir {
			return filepath.SkipDir
		}
		if skipRoot && path == dir {
			return nil
		}
		if err := w.w.Add(path); err != nil {
			w.warn("watch %s: %v", path, err)
		}
		return nil
	})
}

// sweepLoop re-stats every tracked path every sweepInterval. Paths
// whose mtime advanced past the tracked value are treated as missed
// fsnotify events and routed through the standard invalidation path.
func (w *Watcher) sweepLoop(ctx context.Context) {
	t := time.NewTicker(w.sweepInterval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-w.stopped:
			return
		case <-t.C:
			w.sweepOnce()
		}
	}
}

func (w *Watcher) sweepOnce() {
	w.trackedMu.Lock()
	snapshot := make(map[string]int64, len(w.tracked))
	for k, v := range w.tracked {
		snapshot[k] = v
	}
	w.trackedMu.Unlock()

	for path, tracked := range snapshot {
		info, err := os.Stat(path)
		if err != nil {
			if os.IsNotExist(err) {
				w.sweepCatches.Add(1)
				w.route(path)
				w.Untrack(path)
			}
			continue
		}
		mtime := info.ModTime().UnixNano()
		if mtime != tracked {
			w.sweepCatches.Add(1)
			w.route(path)
		}
	}
}

func (w *Watcher) warn(format string, args ...any) {
	if w.log == nil {
		return
	}
	w.log.Warnf(format, args...)
}

// --- path classification ---

func isSourcePath(p string) bool {
	return strings.HasSuffix(p, ".kt") || strings.HasSuffix(p, ".kts") || strings.HasSuffix(p, ".java")
}

func isKritConfigPath(p string) bool {
	base := filepath.Base(p)
	for _, name := range config.Filenames {
		if base == name {
			return true
		}
	}
	return false
}

func isLibraryConfigPath(p string) bool {
	base := filepath.Base(p)
	switch base {
	case "build.gradle", "build.gradle.kts",
		"settings.gradle", "settings.gradle.kts":
		return true
	}
	return strings.HasSuffix(base, ".versions.toml")
}

// isAndroidResourcePath reports whether p is an AndroidManifest.xml or
// a resource XML under a res/ directory.
func isAndroidResourcePath(p string) bool {
	base := filepath.Base(p)
	if base == "AndroidManifest.xml" {
		return true
	}
	if !strings.HasSuffix(base, ".xml") {
		return false
	}
	// res/**/*.xml — require a "res" component anywhere above the file.
	clean := filepath.ToSlash(filepath.Clean(p))
	for _, seg := range strings.Split(filepath.ToSlash(filepath.Dir(clean)), "/") {
		if seg == "res" {
			return true
		}
	}
	return false
}
