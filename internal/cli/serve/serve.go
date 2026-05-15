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
// this to the hash of the sibling krit binary so the CLI-vs-daemon
// handshake compares apples to apples (the CLI hashes its own
// executable; daemonbinary != CLI binary in the krit-daemon-as-shim
// topology). Empty disables the override.
//
// TODO(#247-followup): wire this from a --client-binary-hash flag on
// krit-daemon so the daemon advertises the exact CLI hash it was
// started against, not just a sibling lookup that races CLI rebuilds.
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
	socketFlag := fs.String("socket", "", "Unix socket path (default <root>/.krit/daemon.sock)")
	stopFlag := fs.Bool("stop", false, "Stop a running daemon at --socket")
	idleFlag := fs.Duration("idle-timeout", 30*time.Minute, "Auto-shutdown after no request for this duration; 0 disables")
	noWatcherFlag := fs.Bool("no-watcher", false, "Disable filesystem watching for cache invalidation")
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
	socketPath := *socketFlag
	if socketPath == "" {
		socketPath = daemon.DefaultSocketPath(root)
	}

	if *stopFlag {
		if err := daemon.Call(socketPath, daemon.VerbShutdown, nil, nil); err != nil {
			fmt.Fprintf(os.Stderr, "krit serve --stop: %v\n", err)
			return 1
		}
		return 0
	}

	state := newDaemonState(root)
	state.strictVerify = *strictVerifyFlag
	warmStart := time.Now()
	if err := state.warm(); err != nil {
		fmt.Fprintf(os.Stderr, "krit serve: warm: %v\n", err)
		return 1
	}
	state.warmDuration = time.Since(warmStart)

	srv := daemon.NewServer(socketPath)
	srv.IdleTimeout = *idleFlag
	registerVerbs(srv, state)

	ctx, cancel := signalContext()
	defer cancel()
	if err := srv.Start(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "krit serve: %v\n", err)
		return 1
	}

	if !*noWatcherFlag {
		if w, err := startFileWatcher(ctx, root, state.workspace, srv.Reporter, withConfigChangeCallback(state.InvalidateConfig)); err != nil {
			fmt.Fprintf(os.Stderr, "krit serve: filesystem watcher disabled: %v\n", err)
		} else {
			defer w.Stop()
		}
	}
	fmt.Printf("krit daemon: ready (%d files, %d modules, warm in %.1fs)\n",
		state.fileCount(), state.moduleCount(), state.warmDuration.Seconds())
	srv.Wait()
	return 0
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
	}
}

// oracleDaemonStarter abstracts the JVM-subprocess construction so
// tests can substitute a fake without spinning up a real daemon. The
// production implementation is defaultOracleDaemonStarter.
type oracleDaemonStarter interface {
	Start(jarPath string, sourceDirs []string, classpath []string, verbose bool) (*oracle.Daemon, error)
}

type defaultOracleDaemonStarter struct{}

func (defaultOracleDaemonStarter) Start(jarPath string, sourceDirs, classpath []string, verbose bool) (*oracle.Daemon, error) {
	return oracle.ConnectOrStartDaemon(jarPath, sourceDirs, classpath, verbose)
}

// oracleDaemonEntry caches a started Daemon plus the inputs it was
// constructed under so a future change to scan paths or jar location
// can detect divergence and rebuild.
type oracleDaemonEntry struct {
	daemon     *oracle.Daemon
	jarPath    string
	sourceDirs []string
}

// ensureOracleDaemon lazy-starts (or reuses) a krit-types JVM daemon
// for the given scan paths. Returns (nil, nil) when the krit-types JAR
// cannot be located — callers treat that as "oracle disabled" and the
// verb proceeds without type resolution. Subsequent calls with the
// same scan-path key reuse the cached *oracle.Daemon.
//
// Wired into the analyze-project verb path: buildProjectInput threads
// the returned handle into ProjectHostState.OracleDaemon and flips
// ProjectArgs.OracleEnabled when the daemon is non-nil, so type-aware
// rules in the daemon get oracle precision without paying JVM startup
// on every call.
func (s *daemonState) ensureOracleDaemon(scanPaths []string) (*oracle.Daemon, error) {
	jarPath := oracle.FindJar(scanPaths)
	if jarPath == "" {
		// Graceful disable: no jar means oracle isn't installed in
		// this environment. Cache the negative result so we don't
		// re-walk the filesystem on every call.
		return nil, nil
	}
	sourceDirs := oracle.FindSourceDirs(scanPaths)
	key := jarPath + "\x00" + strings.Join(sourceDirs, "\x00")

	s.oracleDaemonMu.Lock()
	defer s.oracleDaemonMu.Unlock()
	if entry, ok := s.oracleDaemonByKey[key]; ok {
		return entry.daemon, nil
	}
	d, err := s.oracleDaemonStarter.Start(jarPath, sourceDirs, nil, false)
	if err != nil {
		return nil, fmt.Errorf("start oracle daemon: %w", err)
	}
	s.oracleDaemonByKey[key] = &oracleDaemonEntry{daemon: d, jarPath: jarPath, sourceDirs: sourceDirs}
	return d, nil
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
		if err := entry.daemon.Ping(); err == nil {
			continue
		}
		// Failed ping → close and rebuild. Closing a dead daemon is
		// best-effort; errors are intentionally ignored.
		_ = entry.daemon.Close()
		d, err := s.oracleDaemonStarter.Start(entry.jarPath, entry.sourceDirs, nil, false)
		if err != nil {
			// Drop the entry so the next ensureOracleDaemon retries.
			delete(s.oracleDaemonByKey, key)
			continue
		}
		entry.daemon = d
	}
}

// closeOracleDaemons shuts down every cached daemon. Called from the
// serve shutdown hook so JVM children don't survive their parent.
func (s *daemonState) closeOracleDaemons() {
	if s == nil {
		return
	}
	s.oracleDaemonMu.Lock()
	defer s.oracleDaemonMu.Unlock()
	for _, entry := range s.oracleDaemonByKey {
		if entry == nil || entry.daemon == nil {
			continue
		}
		_ = entry.daemon.Close()
	}
	s.oracleDaemonByKey = map[string]*oracleDaemonEntry{}
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

func (s *daemonState) warm() error {
	graph, err := module.DiscoverModules(s.root)
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
		files, _ := scanner.ScanFiles(paths, runtime.NumCPU())
		return files, nil
	}
	if !strings.HasSuffix(path, ".kt") {
		return nil, fmt.Errorf("expected a .kt file or directory, got %s", target)
	}
	f, err := scanner.ParseFile(path)
	if err != nil {
		return nil, fmt.Errorf("parsing %s: %w", target, err)
	}
	return []*scanner.File{f}, nil
}
