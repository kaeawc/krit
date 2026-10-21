package serve

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"slices"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/kaeawc/krit/internal/arch"
	"github.com/kaeawc/krit/internal/cache"
	"github.com/kaeawc/krit/internal/cli/clishared"
	"github.com/kaeawc/krit/internal/config"
	"github.com/kaeawc/krit/internal/daemon"
	"github.com/kaeawc/krit/internal/gradlemodel"
	"github.com/kaeawc/krit/internal/module"
	"github.com/kaeawc/krit/internal/oracle"
	"github.com/kaeawc/krit/internal/pipeline"
	"github.com/kaeawc/krit/internal/scanner"
)

// Version is the krit binary version reported by `status`. cmd/krit
// sets it from its own version variable so daemon clients can detect
// upgrades. Defaults to "dev" so out-of-band callers (tests) get a
// useful value without explicit wiring.
var Version = "dev"

// BinaryHashOverride, when non-empty, is returned by daemonBinaryHash()
// in place of the hash of the running executable. cmd/krit-daemon sets
// this to the CLI's hash advertised via --client-binary-hash (which
// internal/cli/daemoncmd/start.go passes from computeCurrentBinaryHash),
// falling back to a sibling krit-binary scan when the flag is empty
// (e.g. third-party respawn). Both branches keep the CLI-vs-daemon
// handshake comparing apples to apples — the CLI hashes its own
// executable; daemonbinary != CLI binary in the krit-daemon-as-shim
// topology. Empty disables the override.
var BinaryHashOverride string

// daemonBinaryHash returns the SHA-256 hex digest of the krit binary
// this daemon is paired with — see BinaryHashOverride for the
// shim-vs-direct-serve split. Cached after the first call so /status
// responses are cheap. Returns "" if the executable can't be located
// or read — clients treat the empty string as "no opinion" and skip
// the version handshake.
func daemonBinaryHash() string {
	if cached := binaryHashCache.Load(); cached != nil {
		return *cached
	}
	hash := BinaryHashOverride
	if hash == "" {
		hash = computeBinaryHash()
	}
	binaryHashCache.Store(&hash)
	return hash
}

func computeBinaryHash() string {
	exe, err := os.Executable()
	if err != nil {
		return ""
	}
	f, err := os.Open(exe)
	if err != nil {
		return ""
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return ""
	}
	return hex.EncodeToString(h.Sum(nil))
}

func kritVersion() string { return Version }

// binaryHashCache is read+stored atomically since /status may run on
// any goroutine. The first writer wins; subsequent ones see the same
// value (computeBinaryHash is deterministic for an unchanged
// binary).
var binaryHashCache atomic.Pointer[string]

// runServeSubcommand implements `krit serve`. Long-lived process that
// warms the module graph and exposes build-integration verbs over a Unix
// socket. `krit serve --stop` sends a shutdown request to an existing
// daemon at --socket.
func Run(args []string) int {
	fs := flag.NewFlagSet("serve", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	rootFlag := fs.String("root", ".", "Project root")
	socketFlag := fs.String("socket", "", "Unix socket path (default <root>/.krit/daemon.sock, or a short per-user temp path when that exceeds the unix socket length limit)")
	stopFlag := fs.Bool("stop", false, "Stop a running daemon at --socket")
	idleFlag := fs.Duration("idle-timeout", 30*time.Minute, "Auto-shutdown after no request for this duration; 0 disables")
	maxParseBytesFlag := fs.Int64("max-parse-bytes", 0, "Cap on resident parsed-file bytes; over the cap, LRU entries are evicted. 0 disables")
	noWatcherFlag := fs.Bool("no-watcher", false, "Disable filesystem watching for cache invalidation")
	// auto probes fanotify (Linux 5.17+, needs CAP_SYS_ADMIN +
	// CAP_DAC_READ_SEARCH) and falls back to fsnotify silently
	// elsewhere. Users on platforms without fanotify, or without the
	// caps, see no behaviour change; users who run `setcap
	// cap_sys_admin,cap_dac_read_search+ep /path/to/krit` once
	// transparently get the faster backend on the next daemon start.
	// See docs/perf.md for the trade-offs.
	watchBackendFlag := fs.String("watch-backend", "auto", "Filesystem-watch backend: auto (default; fanotify if available, else fsnotify), fsnotify, or fanotify")
	// --strict-verify reruns every analyze in-process from cold caches
	// and fails the response on row-level divergence vs the daemon's
	// resident path. Doubles per-request CPU; intended for alpha and
	// targeted divergence hunts, not steady-state production. See
	// issue #202.
	strictVerifyFlag := fs.Bool("strict-verify", false,
		"compare every analyze response against a fresh in-process baseline; fail on divergence (off by default, see #202)")
	if err := fs.Parse(args); err != nil {
		return 1
	}

	root, err := filepath.Abs(*rootFlag)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 1
	}
	if *stopFlag {
		if err := daemon.Call(stopTarget(root, *socketFlag), daemon.VerbShutdown, nil, nil); err != nil {
			fmt.Fprintf(os.Stderr, "krit serve --stop: %v\n", err)
			return 1
		}
		return 0
	}
	socketPath := *socketFlag
	if socketPath == "" {
		// Creates/validates the private fallback dir when the root is too
		// deep for a unix socket, and fails here with a clear error when no
		// usable dir exists instead of a bare bind EINVAL later.
		var rerr error
		if socketPath, rerr = daemon.ResolveSocketPath(root); rerr != nil {
			fmt.Fprintf(os.Stderr, "krit serve: %v\n", rerr)
			return 1
		}
	}

	// Single-instance enforcement: one daemon per repository, whatever
	// socket path each would have derived from its environment. The kernel
	// drops the flock if we die, so no manual cleanup is needed.
	lock, err := daemon.AcquireRepoLock(daemon.NormalizeRoot(root))
	if err != nil {
		if errors.Is(err, daemon.ErrAlreadyHeld) {
			if pid := daemon.ReadPIDFile(daemon.NormalizeRoot(root)); pid > 0 {
				fmt.Fprintf(os.Stderr, "krit serve: another daemon is running for this repo (already running, PID %d)\n", pid)
			} else {
				fmt.Fprintln(os.Stderr, "krit serve: another daemon is running for this repo (already running, PID unknown)")
			}
			return daemon.ExitAlreadyRunning
		}
		fmt.Fprintf(os.Stderr, "krit serve: acquire repo lock: %v\n", err)
		return 1
	}
	defer lock.Close() //nolint:errcheck // best effort on shutdown

	state := newDaemonState(root)
	// Shut down every oracle JVM child this serve invocation owns
	// before returning. Shared (PID-file-reused) daemons just drop the
	// TCP connection; owned daemons get a graceful Shutdown→Kill. The
	// defer covers normal exit, srv.Wait() return, and signalContext
	// cancellation — SIGKILL still orphans the children, but that's
	// the expected ceiling for unrecoverable parent death.
	defer state.closeOracleDaemons()
	// Drain any pending background bundle/manifest disk writes before the
	// process exits so a save deferred off the warm critical path still
	// lands on disk. Runs (LIFO) before the oracle shutdown above.
	defer state.workspace.FlushBackgroundSaves()
	state.strictVerify = *strictVerifyFlag
	state.workspace.SetMaxParsedBytes(*maxParseBytesFlag)
	warmStart := time.Now()
	if err := state.warm(); err != nil {
		fmt.Fprintf(os.Stderr, "krit serve: warm: %v\n", err)
		return 1
	}
	state.warmDuration = time.Since(warmStart)

	srv := daemon.NewServer(socketPath)
	switch {
	case daemon.IsFallbackSocket(socketPath):
		// Long project path: record where the socket really is, so clients
		// with a different TMPDIR/XDG_RUNTIME_DIR still find it.
		srv.PointerPath = daemon.PointerPath(root)
	case socketPath == daemon.ConventionalSocketPath(root):
		// A stale pointer must not steer clients away from this daemon.
		if err := daemon.ClearSocketPointer(root); err != nil {
			fmt.Fprintf(os.Stderr, "krit serve: clear stale socket pointer: %v\n", err)
		}
	}
	srv.IdleTimeout = *idleFlag
	registerVerbs(srv, state)

	ctx, cancel := signalContext()
	defer cancel()
	if err := srv.Start(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "krit serve: %v\n", err)
		return 1
	}

	resolvedBackend := ""
	if !*noWatcherFlag {
		backendKind, ok := parseWatcherBackendKind(*watchBackendFlag)
		if !ok {
			fmt.Fprintf(os.Stderr, "krit serve: unknown --watch-backend %q; using auto\n", *watchBackendFlag)
		}
		watcherOpts := []watcherOption{
			withConfigChangeCallback(state.InvalidateConfig),
			withBackendKind(backendKind),
		}
		if w, err := startFileWatcher(ctx, root, state.workspace, srv.Reporter, watcherOpts...); err != nil {
			fmt.Fprintf(os.Stderr, "krit serve: filesystem watcher disabled: %v\n", err)
		} else {
			defer w.Stop()
			resolvedBackend = w.BackendKind().String()
		}
	}
	watcherLine := ""
	if resolvedBackend != "" {
		watcherLine = fmt.Sprintf(", watcher=%s", resolvedBackend)
	} else if *noWatcherFlag {
		watcherLine = ", watcher=off"
	}
	fmt.Printf("krit daemon: ready (%d files, %d modules, warm in %.1fs%s)\n",
		state.fileCount(), state.moduleCount(), state.warmDuration.Seconds(), watcherLine)
	srv.Wait()
	return 0
}

// stopTarget is the socket `serve --stop` talks to: the explicit --socket,
// else the daemon's pointer file (so a stop issued from an environment with
// a different TMPDIR/XDG_RUNTIME_DIR still finds a fallback-path daemon),
// else the derived default.
func stopTarget(root, socketFlag string) string {
	if socketFlag != "" {
		return socketFlag
	}
	return daemon.LocateSocketPath(root)
}

// signalContext returns a context cancelled on SIGINT or SIGTERM.
func signalContext() (context.Context, context.CancelFunc) {
	ctx, cancel := context.WithCancel(context.Background())
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, os.Interrupt, syscall.SIGTERM)
	go func() {
		select {
		case <-ch:
			cancel()
		case <-ctx.Done():
		}
	}()
	return ctx, cancel
}

// daemonState holds the warm in-memory project state shared across verbs.
type daemonState struct {
	root         string
	warmDuration time.Duration

	// repoDir is the resolved repository root for repo-local krit
	// state. Filled once at construction (oracle.FindRepoDir is a
	// filesystem walk; the value can't change for the duration of a
	// daemon process). Falls back to root when no VCS marker is
	// found.
	repoDir string

	mu    sync.RWMutex
	graph *module.Graph
	files int

	// configMu guards cachedConfig + configDirty. Use ensureConfig().
	configMu     sync.Mutex
	cachedConfig *config.Config
	// configDirty is set by InvalidateConfig (called from the file
	// watcher on krit.yml / .krit.yml edits). The next ensureConfig
	// reloads from disk and clears the flag.
	configDirty atomic.Bool

	// workspace memoizes parsed buffers across analyze-buffer calls so
	// the daemon stays cheap across short-lived client requests.
	workspace *pipeline.WorkspaceState
	// analyzer is the shared per-file rule dispatcher. Built lazily on
	// the first analyze-buffer call so daemons that only do abi-hash
	// or status don't pay the rule-registration cost.
	analyzerOnce sync.Once
	analyzer     *pipeline.SingleFileAnalyzer

	// parseCacheOnce gates lazy construction of the resident parse
	// cache. The first analyze-project verb call builds one via
	// scanner.NewParseCacheWithCap; subsequent calls reuse the same
	// instance so the 7s zstd-decode cost the CLI pays per run is
	// amortized to once per daemon lifetime.
	parseCacheOnce sync.Once
	parseCache     *scanner.ParseCache
	parseCacheErr  error

	// androidCacheWriterOnce gates lazy construction of the resident
	// Android findings cache writer + cache-dir path. The CLI runner
	// pairs these so AndroidPhase can persist resource/manifest/icon
	// rule results between runs; the daemon previously left both
	// fields zero, which made the entire androidFindingsCacheable
	// gate evaluate to false and forced every analyze to rerun all
	// resource rules (~8 s on the Signal-Android corpus). One
	// resident writer is shared across analyze calls — the async
	// writer queues per Save, so concurrent analyze serialization is
	// not required.
	androidCacheWriterOnce sync.Once
	androidCacheWriter     *scanner.AndroidCacheWriter
	androidCacheDir        string

	// analysisCacheMu guards lazy construction of the resident
	// *cache.Cache + its file path, scoped per scan-path set. Pipeline
	// dispatch merges per-file findings into the cache and saves it on
	// each analyze-project call. The CLI's existing on-disk format is
	// preserved so subsequent CLI runs read the daemon-populated cache.
	analysisCacheMu    sync.Mutex
	analysisCacheByKey map[string]*analysisCacheEntry

	// oracleDaemonMu guards lazy construction + recovery of the
	// resident krit-types JVM subprocess. Lifecycle: ensureOracleDaemon
	// constructs once per scan-path set; pingOracleDaemon checks
	// liveness and Close+Rebuilds on a failed ping. Stays nil when
	// the krit-types JAR cannot be located — the caller treats nil
	// as "oracle disabled" and the verb proceeds without type
	// resolution. See issue #125 PR breakdown.
	oracleDaemonMu      sync.Mutex
	oracleDaemonByKey   map[string]*oracleDaemonEntry
	oracleDaemonStarter oracleDaemonStarter

	// analyzeMu serializes whole-project analysis. The pipeline mutates
	// resolver / oracle state and the analysis-cache write-back path
	// is not safe under concurrent runs; queueing is acceptable
	// because the daemon's typical client (LSP / build tool / MCP)
	// invokes analyze-project at human-perceptible cadences, not
	// in burst-parallel.
	analyzeMu sync.Mutex

	// coldDone reports whether at least one analyze-project call has
	// completed. Used by RequireWarm clients (tests, CI gates) and by
	// the response Stats.Cold flag.
	coldDone atomic.Bool

	// strictVerify, when true, makes every analyze-project call rerun
	// the same scan in-process from cold caches and fail the response
	// if the two finding sets diverge. Set by Run from
	// --strict-verify; see internal/cli/serve/strict_verify.go for
	// the implementation. Default false because the rerun roughly
	// doubles per-request CPU.
	strictVerify bool

	// manifestCache holds parsed FindingsBundleManifest entries by
	// on-disk path. The manifest is the 10 MB JSON that drives the
	// bundle-cache short-circuit; reading + JSON-decoding it on
	// every analyze costs 30-40s on kotlin-corpus scale when the OS
	// page cache is cold (observed after each manifest rewrite).
	// Holding parsed copies resident turns subsequent analyze-project
	// calls back into sub-second responses. Wired into
	// pipeline.RunProject via the host's
	// FindingsBundleManifestLoader/Saver hooks. See issue #247
	// follow-up benchmark notes.
	manifestMu    sync.Mutex
	manifestCache map[string]scanner.FindingsBundleManifest

	// analysisCwd identifies the working directory the last
	// path-resolving verb ran in; see enterCallerCwd. Guarded by
	// analyzeMu.
	analysisCwd os.FileInfo
}

func newDaemonState(root string) *daemonState {
	repoDir := oracle.FindRepoDir([]string{root})
	if repoDir == "" {
		repoDir = root
	}
	return &daemonState{
		root:                root,
		repoDir:             repoDir,
		workspace:           pipeline.NewWorkspaceState(root),
		analysisCacheByKey:  make(map[string]*analysisCacheEntry),
		oracleDaemonByKey:   make(map[string]*oracleDaemonEntry),
		oracleDaemonStarter: defaultOracleDaemonStarter{},
		manifestCache:       make(map[string]scanner.FindingsBundleManifest),
	}
}

// loadManifest is the daemon-side FindingsBundleManifestLoader. It
// consults the in-memory cache first and falls back to a disk read
// (and JSON parse) when the entry isn't resident. The cache is
// populated on disk hits and on saveManifest calls so the
// kotlin-corpus 30-40s cold-OS-page-cache cost is paid at most once
// per daemon lifetime.
func (s *daemonState) loadManifest(path string) (scanner.FindingsBundleManifest, bool) {
	if path == "" {
		return scanner.FindingsBundleManifest{}, false
	}
	s.manifestMu.Lock()
	if m, ok := s.manifestCache[path]; ok {
		s.manifestMu.Unlock()
		return m, true
	}
	s.manifestMu.Unlock()

	// Disk read happens outside the lock so concurrent loaders don't
	// serialise on each other; if two callers miss simultaneously the
	// second's store just overwrites with identical bytes.
	m, ok := scanner.LoadFindingsBundleManifestFromPath(path)
	if !ok {
		return scanner.FindingsBundleManifest{}, false
	}
	s.manifestMu.Lock()
	s.manifestCache[path] = m
	s.manifestMu.Unlock()
	return m, true
}

// saveManifest is the daemon-side FindingsBundleManifestSaver. Called
// after pipeline.SaveFindingsBundleManifest persists a fresh manifest
// to disk; the cache entry is replaced so the next analyze sees the
// new contents without re-reading from disk.
func (s *daemonState) saveManifest(path string, m scanner.FindingsBundleManifest) {
	if path == "" {
		return
	}
	s.manifestMu.Lock()
	s.manifestCache[path] = m
	s.manifestMu.Unlock()
}

// priorManifest returns the resident manifest for the request's scan
// paths or (zero, false) when no entry is cached. Daemon callers use
// this both to short-circuit filesystem walks (see prepopulatedSourcePaths)
// and to short-circuit per-file hash/structural-fingerprint recompute
// inside RunProjectAnalysis (see PriorContentHashes / PriorStructuralFPs
// on pipeline.ProjectHostState).
func (s *daemonState) priorManifest(repoDir string, paths []string) (scanner.FindingsBundleManifest, bool) {
	key := scanner.FindingsBundleManifestKey(repoDir, paths)
	if key == "" {
		return scanner.FindingsBundleManifest{}, false
	}
	manifestPath := scanner.FindingsBundleManifestPath(repoDir, key)
	if manifestPath == "" {
		return scanner.FindingsBundleManifest{}, false
	}
	return s.loadManifest(manifestPath)
}

// priorSourceState returns the prior manifest for the request's scan
// paths along with the source-path lists prepopulated from it. The
// manifest is an on-disk cache, so --no-cache skips it the same way
// resolveDiskCacheWiring drops the manifest loader and saver.
func (s *daemonState) priorSourceState(noCache bool, repoDir string, paths []string) (kotlinPaths, javaPaths []string, prior scanner.FindingsBundleManifest) {
	if noCache {
		return nil, nil, scanner.FindingsBundleManifest{}
	}
	kotlinPaths, javaPaths = s.prepopulatedSourcePaths(repoDir, paths)
	prior, _ = s.priorManifest(repoDir, paths)
	return kotlinPaths, javaPaths, prior
}

// prepopulatedSourcePaths returns the resident manifest's kotlin/java
// path lists when available so the daemon can hand them to
// pipeline.ProjectArgs without forcing runProjectParsePhase to walk
// the filesystem again. Returns (nil, nil) when no resident manifest
// matches the request's scan paths — the pipeline falls back to its
// existing collect-from-disk path in that case.
//
// The manifest's ContentHashes map is the file set the last successful
// analyze indexed, so it cannot name a file added since. Nothing downstream
// notices: the bundle fingerprint is computed over the lists it is handed.
// dropStalePrepopulatedPaths therefore discards these lists on a cold start
// and whenever the watcher reports a path the manifest does not know.
func (s *daemonState) prepopulatedSourcePaths(repoDir string, paths []string) (kotlinPaths, javaPaths []string) {
	m, ok := s.priorManifest(repoDir, paths)
	if !ok || len(m.ContentHashes) == 0 {
		return nil, nil
	}
	kotlinPaths = make([]string, 0, len(m.ContentHashes))
	javaPaths = make([]string, 0)
	for p := range m.ContentHashes {
		switch {
		case strings.HasSuffix(p, ".kt") || strings.HasSuffix(p, ".kts"):
			kotlinPaths = append(kotlinPaths, p)
		case strings.HasSuffix(p, ".java"):
			javaPaths = append(javaPaths, p)
		}
	}
	// Sort so the pipeline's downstream deterministic-fingerprint
	// helpers see a stable order — matters for the resolver/codeindex
	// fingerprint caches that key on the sorted path list.
	sort.Strings(kotlinPaths)
	sort.Strings(javaPaths)
	return kotlinPaths, javaPaths
}

// dropStalePrepopulatedPaths hands the pipeline no source-path lists when the
// prior manifest's lists cannot be trusted to be complete, so it walks the
// source roots and sees an added, renamed-to or moved-in file. They cannot be
// trusted on a daemon's first call (the manifest on disk may predate files
// added while no daemon ran) or when the watcher reports a dirty path the
// manifest does not know.
func dropStalePrepopulatedPaths(in *pipeline.ProjectInput, cold bool, dirty []string) {
	if cold || sourceSetMayHaveChanged(dirty, in.Host.PriorContentHashes, in.Args.Paths) {
		in.Args.KotlinPaths, in.Args.JavaPaths = nil, nil
	}
}

// sourceSetMayHaveChanged reports whether the set of source files may differ
// from the prior manifest's: a dirty Kotlin or Java path the manifest does not
// know (a file added, renamed to or moved in), or a dirty path that no longer
// exists and has known sources beneath it (a directory moved or deleted out).
// The watcher also touches XML, krit.yml, version catalogs and Gradle scripts;
// none of them can change the source set, so editing one must not cost a walk
// of the source roots.
func sourceSetMayHaveChanged(dirty []string, manifest map[string]string, scanRoots []string) bool {
	var sources []string
	for _, p := range dirty {
		switch {
		case isKotlinPath(p) || isJavaPath(p):
			sources = append(sources, p)
		case directoryWithKnownSourcesIsGone(p, manifest, scanRoots):
			return true
		}
	}
	return len(sources) > 0 && !pipeline.DirtyPathsAllInManifest(sources, manifest, scanRoots)
}

// directoryWithKnownSourcesIsGone reports whether p no longer exists and the
// manifest lists files beneath it, under any of the spellings the manifest
// may use (as written, absolute, or relative to a scan root).
func directoryWithKnownSourcesIsGone(p string, manifest map[string]string, scanRoots []string) bool {
	if len(manifest) == 0 || filepath.Ext(p) != "" {
		return false
	}
	if _, err := os.Lstat(p); err == nil {
		return false
	}
	prefixes := []string{p + string(filepath.Separator)}
	// A scan root such as ../lib spells manifest keys relative to the working
	// directory (../lib/gone/B.kt), which neither p nor a root-relative path
	// reproduces.
	if cwd, err := os.Getwd(); err == nil {
		if rel, err := filepath.Rel(cwd, p); err == nil {
			prefixes = append(prefixes, rel+string(filepath.Separator))
		}
	}
	for _, root := range scanRoots {
		abs, err := filepath.Abs(root)
		if err != nil {
			continue
		}
		if rel, err := filepath.Rel(abs, p); err == nil && !strings.HasPrefix(rel, "..") {
			prefixes = append(prefixes, rel+string(filepath.Separator))
		}
	}
	for key := range manifest {
		for _, prefix := range prefixes {
			if strings.HasPrefix(key, prefix) {
				return true
			}
		}
	}
	return false
}

// oracleDaemonStarter abstracts the JVM-subprocess construction so
// tests can substitute a fake without spinning up a real daemon. The
// production implementation is defaultOracleDaemonStarter.
type oracleDaemonStarter interface {
	Start(jarPath string, sourceDirs []string, classpath []string, verbose bool, opts ...oracle.DaemonOption) (*oracle.Daemon, error)
}

type defaultOracleDaemonStarter struct{}

func (defaultOracleDaemonStarter) Start(jarPath string, sourceDirs, classpath []string, verbose bool, opts ...oracle.DaemonOption) (*oracle.Daemon, error) {
	return oracle.ConnectOrStartDaemon(jarPath, sourceDirs, classpath, verbose, opts...)
}

// oracleDaemonEntry caches a started Daemon plus the inputs it was
// constructed under so a future change to scan paths or jar location
// can detect divergence and rebuild.
type oracleDaemonEntry struct {
	daemon      *oracle.Daemon
	jarPath     string
	jarIdentity string
	sourceDirs  []string
	// roots are the walked scan roots before any context is applied; daemons
	// for the same jar and roots under another context retire each other.
	roots []string
	// classpath is the classpath the daemon was started with.
	classpath []string
	// classpathFP fingerprints the classpath entries' size and mtime (walking
	// class directories) when the daemon started. The key holds only the path strings, so a jar rebuilt in
	// place must retire the daemon, whose session still holds the old classes.
	classpathFP string
	// kotlin is the compiler settings the daemon was launched with; a daemon
	// for other settings would answer with other facts, so it is part of the key.
	kotlin gradlemodel.KotlinSettings
	// backend is the oracle backend the daemon serves; a rebuilt daemon
	// must report it too, or its analyses carry no resident backend marker.
	backend oracle.Backend
	// closeFn, when non-nil, replaces entry.daemon.Close() in
	// closeOracleDaemons. Used by tests to inject a controllable
	// close — production code leaves it nil.
	closeFn func() error
}

// close routes to closeFn when set (tests) and to daemon.Close()
// otherwise (production). nil-receiver and nil-daemon are no-ops so
// closeOracleDaemons can safely call this on every cached entry.
func (e *oracleDaemonEntry) close() error {
	if e == nil {
		return nil
	}
	if e.closeFn != nil {
		return e.closeFn()
	}
	if e.daemon == nil {
		return nil
	}
	return e.daemon.Close()
}

// oracleJarForBackend picks the JVM jar the resident oracle daemon
// runs: krit-fir for BackendFIR (the default backend) and krit-types for
// every other value, including "" and unknown backends, so the caller does not
// reproduce the routing logic. Returns "" when neither jar is
// installed; callers degrade to "oracle disabled".
func oracleJarForBackend(scanPaths []string, backend oracle.Backend) string {
	switch backend {
	case oracle.BackendFIR:
		// FIR is the default backend, so tagged releases download a
		// missing krit-fir jar rather than silently running without it.
		jarPath, _ := oracle.EnsureBackendJar(context.Background(), oracle.BackendFIR, scanPaths, false)
		return jarPath
	default:
		return oracle.FindJar(scanPaths)
	}
}

// ensureOracleDaemon lazy-starts (or reuses) the JVM daemon backing
// the requested backend for the given scan paths. BackendKAA spawns
// krit-types via oracle.FindJar; BackendFIR spawns krit-fir via
// oracle.EnsureBackendJar. Returns (nil, nil) when the matching JAR
// cannot be located — callers treat that as "oracle disabled" and
// the verb proceeds without type resolution.
//
// Each (jarPath, sourceDirs) pair gets its own cache entry, so a
// project that flips between FIR and KAA reaches each backend
// without trampling the other's resident state. The on-the-wire
// protocol is the same for both jars (see #559), so the
// defaultOracleDaemonStarter implementation is reused verbatim.
//
// Wired into the analyze-project verb path: buildProjectInput threads
// the returned handle into ProjectHostState.OracleDaemon and flips
// ProjectArgs.OracleEnabled when the daemon is non-nil, so type-aware
// rules in the daemon get oracle precision without paying JVM startup
// on every call.
func (s *daemonState) ensureOracleDaemon(scanPaths []string, backend oracle.Backend, kotlin ...gradlemodel.KotlinSettings) (*oracle.Daemon, error) {
	var ctx oracleDaemonContext
	if len(kotlin) > 0 {
		ctx.Kotlin = kotlin[0]
	}
	return s.ensureOracleDaemonFor(scanPaths, backend, ctx)
}

// oracleDaemonContext is the compile context a request's oracle runs under,
// beyond the scan paths: the Kotlin settings, and, for krit-fir, the classpath
// and extra source roots the in-process scan and the FIR checkers compile with.
// A resident daemon compiles only with what it was started with (requests carry
// no classpath), so these are part of its identity.
type oracleDaemonContext struct {
	Kotlin gradlemodel.KotlinSettings
	// Classpath is the resolved oracle classpath (Gradle model plus config).
	// Both backends start and key on it.
	Classpath []string
	// ExtraSourceDirs are the Gradle-model source roots added to the walked
	// ones (krit-fir only).
	ExtraSourceDirs []string
}

// oracleContextFor is the context the request's FIR checker compiles with, so
// the resident oracle and the checker can share one compile.
func oracleContextFor(args daemon.AnalyzeProjectArgs) oracleDaemonContext {
	return oracleDaemonContext{Kotlin: args.OracleKotlin, Classpath: args.OracleClasspath, ExtraSourceDirs: args.OracleSourceDirs}
}

// ensureOracleDaemonFor is ensureOracleDaemon under an explicit context. For
// krit-fir the daemon is started and keyed with the classpath and the
// filtered, unioned source roots the checker uses, exactly as an in-process
// scan's oracle compiles; a different context gets its own daemon.
func (s *daemonState) ensureOracleDaemonFor(scanPaths []string, backend oracle.Backend, octx oracleDaemonContext) (*oracle.Daemon, error) {
	kt := octx.Kotlin
	jarPath := oracleJarForBackend(scanPaths, backend)
	if jarPath == "" {
		// Graceful disable: no jar means oracle isn't installed in
		// this environment. Cache the negative result so we don't
		// re-walk the filesystem on every call.
		return nil, nil
	}
	// oracleJarForBackend picks krit-fir for FIR and krit-types otherwise.
	served := oracle.BackendKAA
	if backend == oracle.BackendFIR {
		served = oracle.BackendFIR
	}
	roots := oracle.FindSourceDirs(scanPaths)
	// Both backends start and key on the request's classpath, so a daemon
	// started without one is never reused for a request that has one.
	sourceDirs, classpath := roots, octx.Classpath
	if served == oracle.BackendFIR {
		// The checker's context: FilterFIRSourceDirs(Union(Filter(roots), extra)).
		sourceDirs = oracle.FilterFIRSourceDirs(oracle.UnionSourceDirs(oracle.FilterFIRSourceDirs(roots), octx.ExtraSourceDirs))
	}
	key := jarPath + "\x00" + strings.Join(sourceDirs, "\x00") + "\x00" + kt.Identity() + "\x00" + strings.Join(classpath, "\x00")
	classpathFP := oracleClasspathFingerprint(classpath)

	s.oracleDaemonMu.Lock()
	defer s.oracleDaemonMu.Unlock()
	if entry, ok := s.oracleDaemonByKey[key]; ok {
		if entry != nil && entry.jarIdentity == oracle.JarIdentity(jarPath) && entry.classpathFP == classpathFP {
			return entry.daemon, nil
		}
		_ = entry.close()
		delete(s.oracleDaemonByKey, key)
	}
	d, err := s.oracleDaemonStarter.Start(jarPath, sourceDirs, classpath, false, oracle.WithKotlinSettings(kt), oracle.WithBackend(served))
	if err != nil {
		return nil, fmt.Errorf("start oracle daemon: %w", err)
	}
	s.oracleDaemonByKey[key] = &oracleDaemonEntry{daemon: d, jarPath: jarPath, jarIdentity: oracle.JarIdentity(jarPath), sourceDirs: sourceDirs, roots: roots, classpath: classpath, classpathFP: classpathFP, kotlin: kt, backend: served}
	// A resident krit-types JVM costs 1-2 GB. A daemon for the same jar and scan
	// roots under another context (Kotlin settings, classpath, source roots) can no
	// longer be reached once the project's context changed, so retire it instead of
	// keeping one JVM per identity ever seen.
	for otherKey, other := range s.oracleDaemonByKey {
		if otherKey == key || other == nil || other.jarPath != jarPath || !slices.Equal(other.roots, roots) {
			continue
		}
		_ = other.close()
		delete(s.oracleDaemonByKey, otherKey)
	}
	return d, nil
}

// oracleClasspathFingerprint is the size/mtime fingerprint of a resident
// daemon's classpath entries. Jars are stat'd. Directory entries (Gradle
// exports dependent modules' class-output directories) are walked, because
// recompiling a module overwrites .class files without touching the
// directory's own size or mtime.
func oracleClasspathFingerprint(classpath []string) string {
	if len(classpath) == 0 {
		return ""
	}
	h := sha256.New()
	for _, entry := range oracle.AbsolutePaths(classpath) {
		fmt.Fprintf(h, "%s\x00", entry)
		info, err := os.Stat(entry)
		if err != nil {
			_, _ = io.WriteString(h, "missing\n")
			continue
		}
		if !info.IsDir() {
			fmt.Fprintf(h, "%d\x00%d\n", info.Size(), info.ModTime().UnixNano())
			continue
		}
		// WalkDir does not descend into a root that is a symlink.
		walkEntry := entry
		if resolved, resolveErr := filepath.EvalSymlinks(entry); resolveErr == nil {
			walkEntry = resolved
		}
		_ = filepath.WalkDir(walkEntry, func(path string, d os.DirEntry, walkErr error) error {
			if walkErr == nil && !d.IsDir() {
				if fi, infoErr := d.Info(); infoErr == nil {
					fmt.Fprintf(h, "%s\x00%d\x00%d\n", path, fi.Size(), fi.ModTime().UnixNano())
				}
			}
			return nil
		})
		_, _ = io.WriteString(h, "\n")
	}
	return hex.EncodeToString(h.Sum(nil))
}

// pingOracleDaemon checks the liveness of every cached daemon and
// rebuilds any that fail to respond. Called at the start of each
// analyze-project verb so a JVM that died (OOM, host kill) gets
// replaced before the verb attempts to use it. nil-receiver and
// nil-daemon entries are no-ops.
func (s *daemonState) pingOracleDaemon() {
	if s == nil {
		return
	}
	s.oracleDaemonMu.Lock()
	defer s.oracleDaemonMu.Unlock()
	for key, entry := range s.oracleDaemonByKey {
		if entry == nil || entry.daemon == nil {
			continue
		}
		currentClasspathFP := oracleClasspathFingerprint(entry.classpath)
		if entry.jarIdentity == oracle.JarIdentity(entry.jarPath) && entry.classpathFP == currentClasspathFP && entry.daemon.Ping() == nil {
			continue
		}
		// Failed ping → close and rebuild. Closing a dead daemon is
		// best-effort; errors are intentionally ignored.
		_ = entry.daemon.Close()
		d, err := s.oracleDaemonStarter.Start(entry.jarPath, entry.sourceDirs, entry.classpath, false, oracle.WithKotlinSettings(entry.kotlin), oracle.WithBackend(entry.backend))
		if err != nil {
			// Drop the entry so the next ensureOracleDaemon retries.
			delete(s.oracleDaemonByKey, key)
			continue
		}
		entry.daemon = d
		entry.jarIdentity = oracle.JarIdentity(entry.jarPath)
		entry.classpathFP = currentClasspathFP
	}
}

// closeOracleDaemons shuts down every cached daemon. Deferred from
// Run() so the JVM children this serve invocation owns don't survive
// their parent on normal exit (signalContext cancel, srv.Wait return,
// or any return path). SIGKILL on serve still orphans the children —
// they self-terminate on the 30-minute idle timeout in that case.
//
// Daemons connected to via PID-file reuse (shared=true) have their
// TCP connections closed but the underlying process is left alive
// for the next krit invocation to find. Daemons started by serve
// (shared=false) get a graceful Shutdown→Kill via Daemon.Close.
//
// The registry mutex is held only long enough to snapshot the entry
// list and clear the map; Daemon.Close (which can block for the
// 10s shutdown grace period per daemon) runs after the mutex is
// released. Closes run concurrently so N daemons shut down in
// ~one grace period rather than N×grace period, and the registry
// mutex never blocks an insertion path during exit.
func (s *daemonState) closeOracleDaemons() {
	if s == nil {
		return
	}
	s.oracleDaemonMu.Lock()
	entries := make([]*oracleDaemonEntry, 0, len(s.oracleDaemonByKey))
	for _, entry := range s.oracleDaemonByKey {
		if entry == nil {
			continue
		}
		if entry.daemon == nil && entry.closeFn == nil {
			continue
		}
		entries = append(entries, entry)
	}
	s.oracleDaemonByKey = map[string]*oracleDaemonEntry{}
	s.oracleDaemonMu.Unlock()

	var wg sync.WaitGroup
	wg.Add(len(entries))
	for _, entry := range entries {
		entry := entry
		go func() {
			defer wg.Done()
			_ = entry.close()
		}()
	}
	wg.Wait()
}

// analysisCacheEntry holds a lazily-loaded *cache.Cache and the file
// path it persists to. Keyed by the cache file path inside daemonState
// so distinct scan-path sets don't share an entry.
type analysisCacheEntry struct {
	cache *cache.Cache
	path  string
}

// analysisCacheFor returns the resident *cache.Cache and its file path
// for the given scan paths, lazily loading from disk on first request.
// Returns (nil, "") when the cache directory cannot be derived (no
// repo root found). Subsequent calls with the same scan-path set
// return the same pointer so DispatchPhase write-back is amortized
// across daemon calls.
func (s *daemonState) analysisCacheFor(scanPaths []string) (*cache.Cache, string) {
	cacheDir, filePath := cache.ResolveCacheDir("", scanPaths)
	if cacheDir == "" || filePath == "" {
		return nil, ""
	}
	s.analysisCacheMu.Lock()
	defer s.analysisCacheMu.Unlock()
	if entry, ok := s.analysisCacheByKey[filePath]; ok {
		return entry.cache, entry.path
	}
	loaded := cache.Load(filePath)
	s.analysisCacheByKey[filePath] = &analysisCacheEntry{cache: loaded, path: filePath}
	return loaded, filePath
}

// ensureConfig returns the daemon's cached *config.Config, loading
// from disk on the first call and on any call after InvalidateConfig
// has been signalled. Concurrent callers serialise on configMu but
// only the loader pays the I/O.
func (s *daemonState) ensureConfig() (*config.Config, error) {
	if cfg := s.cachedConfig; cfg != nil && !s.configDirty.Load() {
		return cfg, nil
	}
	s.configMu.Lock()
	defer s.configMu.Unlock()
	if s.cachedConfig != nil && !s.configDirty.Load() {
		return s.cachedConfig, nil
	}
	cfg, err := loadDaemonConfig(s.root)
	if err != nil {
		return nil, err
	}
	s.cachedConfig = cfg
	s.configDirty.Store(false)
	return cfg, nil
}

// InvalidateConfig flags the cached config stale; the next
// ensureConfig call reloads krit.yml from disk. Called by the file
// watcher on krit.yml / .krit.yml edits.
func (s *daemonState) InvalidateConfig() {
	s.configDirty.Store(true)
}

// singleFileAnalyzer returns the lazily-built shared analyzer.
func (s *daemonState) singleFileAnalyzer() *pipeline.SingleFileAnalyzer {
	s.analyzerOnce.Do(func() {
		s.analyzer = pipeline.NewSingleFileAnalyzer(nil, nil)
	})
	return s.analyzer
}

// parseCacheFor returns the daemon-resident *scanner.ParseCache,
// constructing one on the first call against the given repoDir +
// capBytes. Subsequent calls return the same pointer so per-call
// invocations of pipeline.RunProject share the same in-memory parse
// table and skip zstd-decode for files whose content hash matches a
// previous run.
//
// repoDir / capBytes are sampled only on the first call; later calls
// ignore them. Callers that need to swap caches across roots should
// stop the current daemon and start a new one (one root per daemon
// is the documented constraint).
//
// A nil return is valid and means the parse cache is disabled (e.g.
// the on-disk pack store could not be opened). RunProject tolerates
// a nil ParseCache by falling back to direct tree-sitter parses.
//
// capBytes is sampled only on the first call; later calls ignore it.
// Tests pass 0 (default cap); the verb passes
// cacheutil.DefaultParseCacheCapBytes.
func (s *daemonState) parseCacheFor(repoDir string, capBytes int64) (*scanner.ParseCache, error) {
	s.parseCacheOnce.Do(func() {
		if repoDir == "" {
			s.parseCacheErr = errors.New("parseCacheFor: repoDir is empty")
			return
		}
		pc, err := scanner.NewParseCacheWithCap(repoDir, capBytes)
		if err != nil {
			s.parseCacheErr = err
			return
		}
		s.parseCache = pc
	})
	return s.parseCache, s.parseCacheErr
}

// closeParseCache releases the resident parse cache, if any. Called
// from Server shutdown; safe to call when no cache exists.
func (s *daemonState) closeParseCache() {
	if s.parseCache == nil {
		return
	}
	_ = s.parseCache.Close()
	s.parseCache = nil
}

// androidCacheWriterFor returns the daemon's resident
// AndroidCacheWriter and its on-disk cache directory, constructing
// both lazily on the first call. Returns (nil, "") when repoDir is
// empty — the AndroidPhase treats a nil writer as "caching disabled"
// and just skips the persistence step, so the rest of the analyze
// still runs correctly without it. workers is the worker count
// passed to NewAndroidCacheWriter; the daemon picks a fixed small
// value because the writer caps it to 4 internally.
func (s *daemonState) androidCacheWriterFor(repoDir string) (*scanner.AndroidCacheWriter, string) {
	if s == nil {
		return nil, ""
	}
	s.androidCacheWriterOnce.Do(func() {
		if repoDir == "" {
			return
		}
		s.androidCacheDir = scanner.AndroidFindingsCacheDir(repoDir)
		s.androidCacheWriter = scanner.NewAndroidCacheWriter(2)
	})
	return s.androidCacheWriter, s.androidCacheDir
}

// closeAndroidCacheWriter flushes queued Android cache writes and
// releases the writer. Called from Server shutdown; safe to call
// when no writer exists.
func (s *daemonState) closeAndroidCacheWriter() {
	if s == nil || s.androidCacheWriter == nil {
		return
	}
	_ = s.androidCacheWriter.Close()
	s.androidCacheWriter = nil
}

func (s *daemonState) warm() error {
	graph, err := module.DiscoverModules(context.Background(), s.root)
	if err != nil {
		return fmt.Errorf("discovering modules: %w", err)
	}
	s.mu.Lock()
	s.graph = graph
	s.files = countModuleFiles(graph)
	s.mu.Unlock()
	return nil
}

func (s *daemonState) moduleGraph() *module.Graph {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.graph
}

func (s *daemonState) moduleCount() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.graph == nil {
		return 0
	}
	return len(s.graph.Modules)
}

func (s *daemonState) fileCount() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.files
}

func countModuleFiles(graph *module.Graph) int {
	if graph == nil {
		return 0
	}
	total := 0
	for _, m := range graph.Modules {
		roots := m.SourceRoots
		if len(roots) == 0 {
			roots = []string{filepath.Join(m.Dir, "src", "main", "kotlin")}
		}
		for _, r := range roots {
			_ = filepath.Walk(r, func(_ string, info os.FileInfo, err error) error {
				if err == nil && info != nil && !info.IsDir() && strings.HasSuffix(info.Name(), ".kt") {
					total++
				}
				return nil
			})
		}
	}
	return total
}

func registerVerbs(srv *daemon.Server, state *daemonState) {
	srv.Register(daemon.VerbStatus, func(_ context.Context, _ json.RawMessage) (any, error) {
		xfile := state.workspace.CrossFileStats()
		return daemon.StatusResult{
			Ready:           true,
			Root:            state.root,
			Modules:         state.moduleCount(),
			Files:           state.fileCount(),
			WarmSeconds:     state.warmDuration.Seconds(),
			KritVersion:     kritVersion(),
			BinaryHash:      daemonBinaryHash(),
			HasLibraryFacts: xfile.HasLibraryFacts,
			HasCodeIndex:    xfile.HasCodeIndex,
			DirtyFiles:      state.workspace.DirtyCount(),
		}, nil
	})
	srv.Register(daemon.VerbShutdown, func(_ context.Context, _ json.RawMessage) (any, error) {
		return map[string]bool{"ok": true}, nil
	})
	srv.Register(daemon.VerbAbiHash, func(_ context.Context, raw json.RawMessage) (any, error) {
		var args daemon.AbiHashArgs
		if len(raw) > 0 {
			if err := json.Unmarshal(raw, &args); err != nil {
				return nil, fmt.Errorf("decode args: %w", err)
			}
		}
		if args.Target == "" {
			return nil, errors.New("abi-hash: target is required")
		}
		files, err := resolveAbiHashTargetForDaemon(state, args.Target)
		if err != nil {
			return nil, err
		}
		sigs := arch.ExtractAbiSignatures(files)
		hash := arch.HashAbiSignatures(sigs)
		res := daemon.AbiHashResult{Target: args.Target, Hash: hash, Inputs: len(sigs)}
		if strings.HasPrefix(args.Target, ":") {
			res.Module = args.Target
		} else {
			res.File = args.Target
		}
		return res, nil
	})
	srv.Register(daemon.VerbAnalyzeBuffer, func(ctx context.Context, raw json.RawMessage) (any, error) {
		return handleAnalyzeBuffer(ctx, state, raw)
	})
	srv.Register(daemon.VerbAnalyzeBuffers, func(ctx context.Context, raw json.RawMessage) (any, error) {
		return handleAnalyzeBuffers(ctx, state, raw)
	})
	srv.Register(daemon.VerbAnalyzeProject, func(ctx context.Context, raw json.RawMessage) (any, error) {
		return handleAnalyzeProject(ctx, state, raw)
	})
	srv.Register(daemon.VerbListRules, func(ctx context.Context, raw json.RawMessage) (any, error) {
		return handleListRules(ctx, state, raw)
	})
	srv.Register(daemon.VerbListExperiments, func(ctx context.Context, raw json.RawMessage) (any, error) {
		return handleListExperiments(ctx, state, raw)
	})
	srv.Register(daemon.VerbValidateConfig, func(ctx context.Context, raw json.RawMessage) (any, error) {
		return handleValidateConfig(ctx, state, raw)
	})
	srv.Register(daemon.VerbOracleFilterFingerprint, func(ctx context.Context, raw json.RawMessage) (any, error) {
		return handleOracleFilterFingerprint(ctx, state, raw)
	})
	srv.Register(daemon.VerbDumpTypes, func(ctx context.Context, raw json.RawMessage) (any, error) {
		return handleDumpTypes(ctx, state, raw)
	})
	srv.Register(daemon.VerbClearCache, func(ctx context.Context, raw json.RawMessage) (any, error) {
		return handleClearCache(ctx, state, raw)
	})
	srv.Register(daemon.VerbClearMatrixCache, func(ctx context.Context, raw json.RawMessage) (any, error) {
		return handleClearMatrixCache(ctx, state, raw)
	})
}

// handleAnalyzeBuffer parses (or reuses a cached parse for) the buffer
// and runs per-file rules through the daemon's shared analyzer. The
// returned Findings field is the same JSON shape as `krit -f json`
// findings so clients can decode either form.
func handleAnalyzeBuffer(ctx context.Context, state *daemonState, raw json.RawMessage) (any, error) {
	var args daemon.AnalyzeBufferArgs
	if len(raw) > 0 {
		if err := json.Unmarshal(raw, &args); err != nil {
			return nil, fmt.Errorf("decode args: %w", err)
		}
	}

	file, hit, err := state.workspace.ParseFileWithHit(ctx, args.Path, []byte(args.Content))
	if err != nil {
		return nil, fmt.Errorf("parse: %w", err)
	}
	columns := state.singleFileAnalyzer().AnalyzeFileColumns(file)
	body, err := json.Marshal(columns)
	if err != nil {
		return nil, fmt.Errorf("marshal findings: %w", err)
	}
	return daemon.AnalyzeBufferResult{Findings: body, CacheHit: hit}, nil
}

// handleAnalyzeBuffers runs handleAnalyzeBuffer's logic for every entry
// in args.Buffers and returns a parallel results slice. Per-buffer
// errors are surfaced inline so a single broken file doesn't fail the
// whole batch.
func handleAnalyzeBuffers(ctx context.Context, state *daemonState, raw json.RawMessage) (any, error) {
	var args daemon.AnalyzeBuffersArgs
	if len(raw) > 0 {
		if err := json.Unmarshal(raw, &args); err != nil {
			return nil, fmt.Errorf("decode args: %w", err)
		}
	}
	analyzer := state.singleFileAnalyzer()
	results := make([]daemon.AnalyzeBufferEntry, len(args.Buffers))
	for i, buf := range args.Buffers {
		file, hit, err := state.workspace.ParseFileWithHit(ctx, buf.Path, []byte(buf.Content))
		if err != nil {
			results[i] = daemon.AnalyzeBufferEntry{Error: err.Error()}
			continue
		}
		columns := analyzer.AnalyzeFileColumns(file)
		body, err := json.Marshal(columns)
		if err != nil {
			results[i] = daemon.AnalyzeBufferEntry{Error: "marshal findings: " + err.Error()}
			continue
		}
		results[i] = daemon.AnalyzeBufferEntry{Findings: body, CacheHit: hit}
	}
	return daemon.AnalyzeBuffersResult{Results: results}, nil
}

// resolveAbiHashTargetForDaemon mirrors resolveAbiHashTarget but uses the
// daemon's cached module graph for module-style targets.
func resolveAbiHashTargetForDaemon(state *daemonState, target string) ([]*scanner.File, error) {
	if strings.HasPrefix(target, ":") {
		graph := state.moduleGraph()
		if graph == nil {
			return nil, fmt.Errorf("no module graph (root %s has no settings file)", state.root)
		}
		mod, ok := graph.Modules[target]
		if !ok {
			return nil, fmt.Errorf("module %q not found", target)
		}
		return clishared.ScanModuleKotlinFiles(mod), nil
	}

	path := target
	if !filepath.IsAbs(path) {
		path = filepath.Join(state.root, path)
	}
	info, err := os.Stat(path)
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", target, err)
	}
	if info.IsDir() {
		paths, err := scanner.CollectKotlinFiles([]string{path}, nil)
		if err != nil {
			return nil, fmt.Errorf("collecting %s: %w", target, err)
		}
		files, _ := scanner.ScanFiles(context.Background(), paths, runtime.NumCPU())
		return files, nil
	}
	if !strings.HasSuffix(path, ".kt") {
		return nil, fmt.Errorf("expected a .kt file or directory, got %s", target)
	}
	f, err := scanner.ParseFile(context.Background(), path)
	if err != nil {
		return nil, fmt.Errorf("parsing %s: %w", target, err)
	}
	return []*scanner.File{f}, nil
}
