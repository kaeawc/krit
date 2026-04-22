package oracle

// Cache-aware oracle invocation.
//
// InvokeCached wraps Invoke with an on-disk incremental cache keyed by
// (content hash, closure fingerprint). On a cold run it delegates to a
// full krit-types launch and writes per-file cache entries from the
// accompanying --cache-deps-out JSON. On a warm run it partitions source
// files into hits (served from cache, no JVM) and misses (re-analyzed via
// krit-types with --files LISTFILE), then assembles a merged OracleData
// and writes it to outputPath so existing downstream consumers
// (oracle.Load, -output-types) keep working unchanged.
//
// The existing Invoke() signature is NOT touched — see invoke.go. Callers
// that want caching explicitly choose it via InvokeCached.

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/kaeawc/krit/internal/fsutil"
	"github.com/kaeawc/krit/internal/perf"
	"github.com/kaeawc/krit/internal/store"
)

// readFilterListFile parses a rule-classification filter list (one
// absolute path per line) into a set for fast membership tests. Used by
// InvokeCached to intersect the cache-lookup universe with the filter.
func readFilterListFile(path string) (map[string]bool, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	set := map[string]bool{}
	for _, line := range strings.Split(string(data), "\n") {
		t := strings.TrimSpace(line)
		if t != "" {
			set[t] = true
		}
	}
	return set, nil
}

// defaultExcludeGlobs mirrors the DEFAULT_EXCLUDE_GLOBS constant on the
// krit-types Kotlin side. Files whose absolute path contains any of these
// substrings are skipped by the JVM-side analyze loop; we apply the same
// filter Go-side before classify so excluded files don't leak into the
// miss list. If this drifts from the Kotlin default, krit-types wins
// (the jar's filter is authoritative); Go just avoids extra work.
var defaultExcludeSubstrings = []string{
	"/testData/",
	"/test-resources/",
}

// excludedByDefault returns true if path matches any default exclude
// pattern. Uses substring matching rather than glob matching to avoid a
// dependency; the krit-types default patterns (**/testData/** and
// **/test-resources/**) are semantically equivalent to "path contains
// /testData/ or /test-resources/ as a directory segment".
func excludedByDefault(path string) bool {
	for _, s := range defaultExcludeSubstrings {
		if strings.Contains(path, s) {
			return true
		}
	}
	return false
}

// CollectKtFiles walks the given source directories and returns absolute
// paths of all .kt files. Mirrors the directory pruning FindSourceDirs
// does (build/.gradle/.git/node_modules) so the Go-side enumeration
// matches what the JVM side will actually see. Also applies the krit-types
// default exclude patterns so excluded files don't leak into the cache
// miss list — without this, repos like kotlin/kotlin push 40k+ excluded
// testData files through classify every warm run.
func CollectKtFiles(sourceDirs []string) ([]string, error) {
	seen := map[string]bool{}
	var out []string
	for _, root := range sourceDirs {
		err := filepath.Walk(root, func(p string, info os.FileInfo, err error) error {
			if err != nil {
				return nil
			}
			if info.IsDir() {
				base := filepath.Base(p)
				// Prune dirs that never contain user-written sources we
				// want in the oracle. `build` used to be on this list but
				// was removed after an audit found it excluded 1709 real
				// checked-in .kt files in kotlin/kotlin (core/builtins/build/
				// holds generated kotlin-reflect stubs that downstream code
				// imports). If a project genuinely has a noisy build dir,
				// the krit-types --exclude glob is the correct knob.
				if base == ".gradle" || base == ".git" || base == "node_modules" {
					return filepath.SkipDir
				}
				// Prune excluded dirs (testData, test-resources) at the
				// walker level so we don't recurse into them at all.
				if base == "testData" || base == "test-resources" {
					return filepath.SkipDir
				}
				return nil
			}
			// Match krit-types JVM side: KtFile includes both .kt and
			// .kts (Kotlin script — used by build-logic gradle files).
			// Limiting Go-side collection to .kt only would cause the
			// cache path to silently drop .kts files that the plain
			// Invoke path would have analyzed.
			name := info.Name()
			if !strings.HasSuffix(name, ".kt") && !strings.HasSuffix(name, ".kts") {
				return nil
			}
			// File-level exclude check as a backstop — the dir-level
			// prune above should catch everything but a file matching
			// a non-directory substring would slip through that. Cheap.
			if excludedByDefault(p) {
				return nil
			}
			if seen[p] {
				return nil
			}
			seen[p] = true
			out = append(out, p)
			return nil
		})
		if err != nil {
			return nil, err
		}
	}
	return out, nil
}

// InvokeCached is the cache-aware variant of Invoke. It walks sourceDirs,
// classifies .kt files into hits and misses via the on-disk cache, runs
// krit-types only on misses (with --files + --cache-deps-out), writes new
// cache entries, and assembles the final oracle JSON at outputPath.
//
// filterListPath (optional, "" = no filter) is a path to a newline-separated
// list of absolute .kt paths produced by the rule-classification pre-scan.
// When present, it narrows the universe of files the cache classifies —
// files not in the filter are neither looked up nor analyzed, since no
// enabled rule cares about them. Rule filtering and per-file caching thus
// compose: filter narrows first, cache dedupes what remains.
//
// Returns the output path on success. If no files were discovered or the
// cache can't be created, the function falls back to a plain Invoke so
// the caller still gets a complete oracle.
// InvokeCached is the cache-aware variant of Invoke.
// s is the optional unified store; when non-nil, oracle cache entries are
// read from and written to s instead of the legacy cacheDir file layout.
func InvokeCached(
	jarPath string,
	sourceDirs []string,
	repoDir string,
	outputPath string,
	filterListPath string,
	verbose bool,
	s *store.FileStore,
) (string, error) {
	return InvokeCachedWithOptions(jarPath, sourceDirs, repoDir, outputPath, filterListPath, verbose, s, InvocationOptions{})
}

// InvokeCachedWithOptions is InvokeCached plus optional deep perf
// instrumentation for the cache/filter/JVM path.
func InvokeCachedWithOptions(
	jarPath string,
	sourceDirs []string,
	repoDir string,
	outputPath string,
	filterListPath string,
	verbose bool,
	s *store.FileStore,
	opts InvocationOptions,
) (string, error) {
	tracker := opts.tracker()
	if repoDir == "" {
		// Fall back to the filter-only path — we need a repo root to anchor
		// the cache dir, and without it the caching layer can't do its
		// job safely. Still honor the filter so we don't re-analyze files
		// no rule cares about.
		addOracleInstant(tracker, "cacheBypass", nil, map[string]string{"reason": "missingRepoDir"})
		return InvokeWithFilesWithOptions(jarPath, sourceDirs, outputPath, filterListPath, verbose, opts)
	}
	var cacheDir string
	if err := trackOracle(tracker, "cacheDirInit", func() error {
		var err error
		cacheDir, err = CacheDir(repoDir)
		return err
	}); err != nil {
		if verbose {
			fmt.Fprintf(os.Stderr, "verbose: cache dir init failed (%v), falling back to full run\n", err)
		}
		addOracleInstant(tracker, "cacheBypass", nil, map[string]string{"reason": "cacheDirInit"})
		return InvokeWithFilesWithOptions(jarPath, sourceDirs, outputPath, filterListPath, verbose, opts)
	}

	var ktFiles []string
	if err := trackOracle(tracker, "collectKtFiles", func() error {
		var err error
		ktFiles, err = CollectKtFiles(sourceDirs)
		return err
	}); err != nil || len(ktFiles) == 0 {
		if verbose {
			fmt.Fprintf(os.Stderr, "verbose: no .kt files discovered for cache; running full oracle\n")
		}
		if err != nil {
			addOracleInstant(tracker, "cacheBypass", nil, map[string]string{"reason": "collectKtFiles", "error": err.Error()})
		} else {
			addOracleInstant(tracker, "cacheBypass", map[string]int64{"files": 0}, map[string]string{"reason": "noKtFiles"})
		}
		return InvokeWithFilesWithOptions(jarPath, sourceDirs, outputPath, filterListPath, verbose, opts)
	}
	addOracleInstant(tracker, "ktFilesDiscovered", map[string]int64{"files": int64(len(ktFiles)), "sourceDirs": int64(len(sourceDirs))}, nil)
	callFilterScope := callFilterFingerprint(opts)

	// Apply the rule-classification filter (if any) before cache lookup:
	// files not in the filter set are dropped from both the hit-lookup and
	// the miss-analysis stages because no enabled rule has declared a need
	// for them. This makes the filter and the cache stack multiplicatively.
	if filterListPath != "" {
		var wanted map[string]bool
		var ferr error
		start := time.Now()
		wanted, ferr = readFilterListFile(filterListPath)
		if ferr != nil {
			if verbose {
				fmt.Fprintf(os.Stderr, "verbose: read filter list %s: %v (ignoring filter)\n", filterListPath, ferr)
			}
			addOracleEntry(tracker, "readOracleFilterList", start, nil, map[string]string{"error": ferr.Error()})
		} else {
			before := len(ktFiles)
			filtered := ktFiles[:0]
			for _, p := range ktFiles {
				if wanted[p] {
					filtered = append(filtered, p)
				}
			}
			ktFiles = filtered
			addOracleEntry(tracker, "applyOracleFilterList", start, map[string]int64{
				"before": int64(before),
				"after":  int64(len(ktFiles)),
				"wanted": int64(len(wanted)),
			}, nil)
			if verbose {
				fmt.Fprintf(os.Stderr, "verbose: cache filter intersection: %d/%d files after oracle-filter\n", len(ktFiles), before)
			}
		}
	}

	startClassify := time.Now()
	hits, misses := ClassifyFilesWithStoreScoped(s, cacheDir, ktFiles, callFilterScope)
	classifyElapsed := time.Since(startClassify)
	perf.AddEntryDetails(tracker, "cacheClassify", classifyElapsed, map[string]int64{
		"files":  int64(len(ktFiles)),
		"hits":   int64(len(hits)),
		"misses": int64(len(misses)),
	}, nil)
	if verbose {
		fmt.Fprintf(os.Stderr, "verbose: cache classify: %d hits, %d misses (%s, %d files)\n",
			len(hits), len(misses), classifyElapsed, len(ktFiles))
	}

	// Fast path: all hits. Assemble, write, return — no JVM launched.
	if len(misses) == 0 {
		var merged *OracleData
		trackOracle(tracker, "assembleOracleFromCache", func() error {
			merged = AssembleOracle(hits, nil)
			return nil
		})
		if err := trackOracle(tracker, "writeOracleJSON", func() error {
			return writeOracleJSON(outputPath, merged)
		}); err != nil {
			return "", err
		}
		if verbose {
			var count int
			var bytes int64
			trackOracle(tracker, "cacheStats", func() error {
				count, bytes, _ = CacheStats(cacheDir)
				return nil
			})
			fmt.Fprintf(os.Stderr, "verbose: oracle served entirely from cache (%d entries, %d bytes)\n", count, bytes)
			addOracleInstant(tracker, "cacheStatsValues", map[string]int64{"entries": int64(count), "bytes": bytes}, nil)
		}
		return outputPath, nil
	}

	// Slow path: there are misses. Prefer the persistent daemon — it
	// amortizes the Analysis API session build (~20-28 s on kotlin)
	// across invocations — and fall back to the one-shot jar on any
	// daemon failure. Tempfiles are prepared unconditionally because
	// the fallback path needs them.
	var missListPath, missFreshPath, missDepsPath string
	if err := trackOracle(tracker, "prepareMissTemps", func() error {
		var err error
		missListPath, missFreshPath, missDepsPath, err = prepareMissTemps(misses)
		return err
	}); err != nil {
		return "", err
	}
	defer func() {
		_ = os.Remove(missListPath)
		_ = os.Remove(missFreshPath)
		_ = os.Remove(missDepsPath)
	}()

	freshData, depsFile, usedDaemon, err := runMissAnalysis(
		jarPath, sourceDirs, misses,
		missListPath, missFreshPath, missDepsPath, verbose, tracker, opts,
	)
	if err != nil {
		return "", err
	}
	if verbose {
		source := "one-shot"
		if usedDaemon {
			source = "daemon"
		}
		fmt.Fprintf(os.Stderr, "verbose: miss analysis via %s\n", source)
	}

	if depsFile == nil {
		if verbose {
			fmt.Fprintf(os.Stderr, "verbose: no cache deps returned; cache not updated\n")
		}
	} else {
		if opts.CacheWriter != nil {
			start := time.Now()
			queued, _ := opts.CacheWriter.QueueFreshEntriesToStoreScoped(s, cacheDir, freshData, depsFile, callFilterScope)
			perf.AddEntryDetails(tracker, "queueFreshCacheEntries", time.Since(start), map[string]int64{"queued": int64(queued)}, nil)
			addOracleInstant(tracker, "freshCacheEntriesQueued", map[string]int64{"entries": int64(queued)}, nil)
			if verbose {
				fmt.Fprintf(os.Stderr, "verbose: queued %d new cache entries\n", queued)
			}
		} else {
			var written int
			writeTracker := tracker.Serial("writeFreshCacheEntries")
			written, _ = WriteFreshEntriesToStoreWithTrackerScoped(s, cacheDir, freshData, depsFile, writeTracker, callFilterScope)
			writeTracker.End()
			addOracleInstant(tracker, "freshCacheEntriesWritten", map[string]int64{"entries": int64(written)}, nil)
			if verbose {
				fmt.Fprintf(os.Stderr, "verbose: wrote %d new cache entries\n", written)
			}
		}
	}

	// Silently-dropped files: Go requested a miss analysis for a file
	// the jar then skipped without producing a FileResult or a crash
	// marker. Observed cause on kotlin/kotlin: a 3.9 MB data file
	// (GraphSolverBenchmark.kt with hardcoded graph literals) that the
	// jar's PSI enumeration excludes silently. Without a cache entry
	// these files re-enter the miss list on every subsequent warm run
	// and trigger a fresh JVM launch just to be skipped again. Write a
	// "jar-skipped" poison-style entry so classify treats them as hits.
	analyzed := map[string]bool{}
	if freshData != nil {
		for path := range freshData.Files {
			analyzed[path] = true
		}
	}
	if depsFile != nil {
		for path := range depsFile.Crashed {
			analyzed[path] = true
		}
	}
	skipped := 0
	trackOracle(tracker, "writeSkippedPoisonEntries", func() error {
		for _, p := range misses {
			if analyzed[p] {
				continue
			}
			hash, herr := ContentHash(p)
			if herr != nil {
				continue
			}
			entry := &CacheEntry{
				V:                     CacheVersion,
				ContentHash:           hash,
				FilePath:              p,
				Crashed:               true,
				CrashError:            "jar-skipped: file not in Analysis API KtFile set (typically oversized source)",
				CallFilterFingerprint: callFilterScope,
			}
			writeErr := func() error {
				if s != nil {
					return WriteEntryToStore(s, entry)
				}
				return WriteEntry(cacheDir, entry)
			}()
			if writeErr == nil {
				skipped++
			}
		}
		return nil
	})
	addOracleInstant(tracker, "skippedPoisonEntriesWritten", map[string]int64{"entries": int64(skipped)}, nil)
	if skipped > 0 && verbose {
		fmt.Fprintf(os.Stderr, "verbose: wrote %d jar-skipped poison entries\n", skipped)
	}

	var merged *OracleData
	trackOracle(tracker, "assembleOracle", func() error {
		merged = AssembleOracle(hits, freshData)
		return nil
	})
	if err := trackOracle(tracker, "writeOracleJSON", func() error {
		return writeOracleJSON(outputPath, merged)
	}); err != nil {
		return "", err
	}
	if verbose {
		var count int
		var bytes int64
		trackOracle(tracker, "cacheStats", func() error {
			count, bytes, _ = CacheStats(cacheDir)
			return nil
		})
		fmt.Fprintf(os.Stderr, "verbose: cache now has %d entries, %d bytes total\n", count, bytes)
		addOracleInstant(tracker, "cacheStatsValues", map[string]int64{"entries": int64(count), "bytes": bytes}, nil)
	}
	return outputPath, nil
}

// prepareMissTemps creates three tempfiles for the miss-run round trip:
//
//	missListPath — newline-separated absolute paths for --files
//	missFreshPath — krit-types --output target (we'll read back)
//	missDepsPath  — krit-types --cache-deps-out target (we'll read back)
//
// The caller is responsible for removing all three.
func prepareMissTemps(misses []string) (string, string, string, error) {
	f, err := os.CreateTemp("", "krit-miss-list-*.txt")
	if err != nil {
		return "", "", "", fmt.Errorf("tempfile (miss list): %w", err)
	}
	for _, p := range misses {
		fmt.Fprintln(f, p)
	}
	_ = f.Close()

	fresh, err := os.CreateTemp("", "krit-miss-fresh-*.json")
	if err != nil {
		_ = os.Remove(f.Name())
		return "", "", "", fmt.Errorf("tempfile (fresh): %w", err)
	}
	_ = fresh.Close()

	deps, err := os.CreateTemp("", "krit-miss-deps-*.json")
	if err != nil {
		_ = os.Remove(f.Name())
		_ = os.Remove(fresh.Name())
		return "", "", "", fmt.Errorf("tempfile (deps): %w", err)
	}
	_ = deps.Close()

	return f.Name(), fresh.Name(), deps.Name(), nil
}

// runKritTypesCached is the miss-run exec path: same JVM invocation as
// Invoke, with three extra flags (--files / --cache-deps-out / keep
// --sources so the session still sees the full source module). The full
// source roots are passed because Analysis API still needs the complete
// module to resolve cross-file references — only the analyze loop gets
// restricted to the miss list.
func runKritTypesCached(
	jarPath string,
	sourceDirs []string,
	missListPath, freshOutPath, depsOutPath string,
	verbose bool,
	tracker perf.Tracker,
	opts InvocationOptions,
) error {
	var javaPath string
	if err := trackOracle(tracker, "javaLookup", func() error {
		var err error
		javaPath, err = exec.LookPath("java")
		if err != nil {
			return fmt.Errorf("java not found in PATH: %w", err)
		}
		return nil
	}); err != nil {
		return err
	}
	if err := trackOracle(tracker, "freshOutputDirCreate", func() error {
		return os.MkdirAll(filepath.Dir(freshOutPath), 0o755)
	}); err != nil {
		return fmt.Errorf("create output dir: %w", err)
	}

	args := []string{
		"-XX:+UseG1GC",
		"-XX:+UseStringDeduplication",
		"-Xms1g",
		"-jar", jarPath,
		"--sources", strings.Join(sourceDirs, ","),
		"--output", freshOutPath,
		"--files", missListPath,
		"--cache-deps-out", depsOutPath,
	}
	callFilterPath, cleanupCallFilter, err := writeCallFilterArg(opts, tracker)
	if err != nil {
		return fmt.Errorf("call filter: %w", err)
	}
	defer cleanupCallFilter()
	if callFilterPath != "" {
		args = append(args, "--call-filter", callFilterPath)
	}
	var timingsPath string
	if tracker != nil && tracker.IsEnabled() {
		path, cleanup, err := tempTimingsPath()
		if err != nil {
			return err
		}
		timingsPath = path
		defer cleanup()
		args = append(args, "--timings-out", timingsPath)
	}
	if verbose {
		fmt.Fprintf(os.Stderr, "verbose: Running krit-types (cached): %s %s\n", javaPath, strings.Join(args, " "))
	}

	timeout := invokeTimeout()
	grace := invokeGraceExit()
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	var runErr error
	var proc oracleProcessResult
	trackErr := trackOracle(tracker, "kritTypesProcess", func() error {
		proc, runErr = runOracleProcessMeasured(ctx, javaPath, args, freshOutPath, timeout, grace, verbose)
		return runErr
	})
	addOracleProcessResources(tracker, "kritTypesProcessResources", proc.PeakRSSMB)
	if trackErr != nil {
		return trackErr
	}
	addKotlinTimingsFromFile(tracker, timingsPath)
	return runErr
}

type kritTypesCachedRunner func(
	jarPath string,
	sourceDirs []string,
	missListPath, freshOutPath, depsOutPath string,
	verbose bool,
	tracker perf.Tracker,
) error

type shardResult struct {
	Fresh      *OracleData
	Deps       *CacheDepsFile
	DepsErr    error
	Err        error
	Files      int
	Cost       int64
	Bytes      int64
	CallTokens int64
}

func configuredKritTypesShards(misses int) int {
	if misses <= 1 {
		return 1
	}
	raw := strings.TrimSpace(os.Getenv("KRIT_TYPES_SHARDS"))
	if raw == "" {
		return 1
	}
	n, err := strconv.Atoi(raw)
	if err != nil || n <= 1 {
		return 1
	}
	if n > misses {
		return misses
	}
	return n
}

type kaaMissCost struct {
	Path       string
	Cost       int64
	Bytes      int64
	CallTokens int64
}

type kaaMissGroup struct {
	Paths      []string
	Cost       int64
	Bytes      int64
	CallTokens int64
}

const kaaCallTokenCostBytes int64 = 512

func splitMissesForKAA(paths []string, shards int) [][]string {
	groups := splitMissesForKAAWithStats(paths, shards)
	out := make([][]string, len(groups))
	for i, group := range groups {
		out[i] = group.Paths
	}
	return out
}

func splitMissesForKAAWithStats(paths []string, shards int) []kaaMissGroup {
	if len(paths) == 0 {
		return nil
	}
	if shards < 1 {
		shards = 1
	}
	if shards > len(paths) {
		shards = len(paths)
	}
	costs := make([]kaaMissCost, 0, len(paths))
	for _, p := range paths {
		costs = append(costs, estimateKAAMissCost(p))
	}
	sort.SliceStable(costs, func(i, j int) bool {
		if costs[i].Cost != costs[j].Cost {
			return costs[i].Cost > costs[j].Cost
		}
		return costs[i].Path < costs[j].Path
	})

	groups := make([]kaaMissGroup, shards)
	loads := make([]int64, shards)
	for _, c := range costs {
		k := indexOfMinInt64(loads)
		groups[k].Paths = append(groups[k].Paths, c.Path)
		groups[k].Cost += c.Cost
		groups[k].Bytes += c.Bytes
		groups[k].CallTokens += c.CallTokens
		loads[k] += c.Cost
	}
	return groups
}

func estimateKAAMissCost(path string) kaaMissCost {
	c := kaaMissCost{Path: path, Cost: 1}
	if st, err := os.Stat(path); err == nil && st.Size() > 0 {
		c.Bytes = st.Size()
	}
	if data, err := os.ReadFile(path); err == nil {
		c.Bytes = int64(len(data))
		c.CallTokens = roughKAACallTokens(data)
	}
	c.Cost = c.Bytes + c.CallTokens*kaaCallTokenCostBytes
	if c.Cost <= 0 {
		c.Cost = 1
	}
	return c
}

func roughKAACallTokens(data []byte) int64 {
	var n int64
	for _, b := range data {
		if b == '(' {
			n++
		}
	}
	return n
}

func indexOfMinInt64(values []int64) int {
	if len(values) == 0 {
		return 0
	}
	best := 0
	for i := 1; i < len(values); i++ {
		if values[i] < values[best] {
			best = i
		}
	}
	return best
}

func runKritTypesCachedSharded(
	jarPath string,
	sourceDirs []string,
	misses []string,
	shards int,
	verbose bool,
	tracker perf.Tracker,
) (*OracleData, *CacheDepsFile, error) {
	runner := func(jarPath string, sourceDirs []string, missListPath, freshOutPath, depsOutPath string, verbose bool, tracker perf.Tracker) error {
		return runKritTypesCached(jarPath, sourceDirs, missListPath, freshOutPath, depsOutPath, verbose, tracker, InvocationOptions{})
	}
	return runKritTypesCachedShardedWithRunner(jarPath, sourceDirs, misses, shards, verbose, tracker, runner)
}

func runKritTypesCachedShardedWithRunner(
	jarPath string,
	sourceDirs []string,
	misses []string,
	shards int,
	verbose bool,
	tracker perf.Tracker,
	runner kritTypesCachedRunner,
) (*OracleData, *CacheDepsFile, error) {
	if tracker == nil {
		tracker = perf.New(false)
	}
	groups := splitMissesForKAAWithStats(misses, shards)
	if len(groups) == 0 {
		return mergeOracleData(), nil, nil
	}
	var totalCost, totalBytes, totalCallTokens int64
	for _, group := range groups {
		totalCost += group.Cost
		totalBytes += group.Bytes
		totalCallTokens += group.CallTokens
	}
	addOracleInstant(tracker, "shardedMissAnalysisSummary", map[string]int64{
		"shards":     int64(len(groups)),
		"files":      int64(len(misses)),
		"cost":       totalCost,
		"bytes":      totalBytes,
		"callTokens": totalCallTokens,
	}, nil)

	results := make([]shardResult, len(groups))
	var wg sync.WaitGroup
	for i, group := range groups {
		i, group := i, group
		wg.Add(1)
		go func() {
			defer wg.Done()
			child := tracker.Serial(fmt.Sprintf("kritTypesShard/%d", i))
			defer child.End()
			results[i].Files = len(group.Paths)
			results[i].Cost = group.Cost
			results[i].Bytes = group.Bytes
			results[i].CallTokens = group.CallTokens
			addOracleInstant(child, "shardInputSummary", map[string]int64{
				"files":      int64(len(group.Paths)),
				"cost":       group.Cost,
				"bytes":      group.Bytes,
				"callTokens": group.CallTokens,
			}, nil)

			listPath, freshPath, depsPath, err := prepareMissTemps(group.Paths)
			if err != nil {
				results[i].Err = err
				return
			}
			defer func() {
				_ = os.Remove(listPath)
				_ = os.Remove(freshPath)
				_ = os.Remove(depsPath)
			}()

			if err := runner(jarPath, sourceDirs, listPath, freshPath, depsPath, verbose, child); err != nil {
				results[i].Err = err
				return
			}

			var fresh *OracleData
			if err := trackOracle(child, "readFreshOracleJSON", func() error {
				var readErr error
				fresh, readErr = readOracleJSON(freshPath)
				return readErr
			}); err != nil {
				results[i].Err = fmt.Errorf("shard %d read fresh oracle: %w", i, err)
				return
			}

			var deps *CacheDepsFile
			var depsErr error
			trackOracle(child, "readCacheDepsJSON", func() error {
				deps, depsErr = LoadCacheDeps(depsPath)
				return nil
			})
			results[i] = shardResult{
				Fresh:      fresh,
				Deps:       deps,
				DepsErr:    depsErr,
				Files:      len(group.Paths),
				Cost:       group.Cost,
				Bytes:      group.Bytes,
				CallTokens: group.CallTokens,
			}
		}()
	}
	wg.Wait()

	for i, result := range results {
		if result.Err != nil {
			return nil, nil, fmt.Errorf("krit-types shard %d failed: %w", i, result.Err)
		}
	}

	var fresh *OracleData
	if err := trackOracle(tracker, "mergeShardOracleJSON", func() error {
		parts := make([]*OracleData, 0, len(results))
		for _, result := range results {
			parts = append(parts, result.Fresh)
		}
		fresh = mergeOracleData(parts...)
		return nil
	}); err != nil {
		return nil, nil, err
	}

	var deps *CacheDepsFile
	if err := trackOracle(tracker, "mergeShardCacheDeps", func() error {
		parts := make([]*CacheDepsFile, 0, len(results))
		for i, result := range results {
			if result.DepsErr != nil {
				addOracleInstant(tracker, "shardedCacheDepsReadError", map[string]int64{"shard": int64(i)}, map[string]string{"error": result.DepsErr.Error()})
				deps = nil
				return nil
			}
			parts = append(parts, result.Deps)
		}
		deps = mergeCacheDeps(parts...)
		return nil
	}); err != nil {
		return nil, nil, err
	}

	return fresh, deps, nil
}

// runMissAnalysis runs the miss-list analysis via the persistent daemon
// when reachable, or falls back to the one-shot JVM launch on any daemon
// failure. Returns (freshData, depsFile, usedDaemon, err).
//
// The daemon path is preferred because it amortizes the Analysis API
// session build (~20-28 s on kotlin/kotlin) across invocations. The
// fallback preserves the exact same output as the pre-daemon path:
// runKritTypesCached writes tempfiles which are then loaded via
// readOracleJSON + LoadCacheDeps.
//
// Daemon path is default-on. Set KRIT_DAEMON_CACHE=off to force the
// one-shot path for diagnostics. ConnectOrStartDaemon already handles
// the "no daemon running" case by starting one.
//
// On file-not-in-session errors from the daemon (the daemon's
// sourceModule was built before the file existed), this function
// calls daemon.Rebuild() once and retries AnalyzeWithDeps. If the
// second attempt also fails, falls through to one-shot.
func runMissAnalysis(
	jarPath string,
	sourceDirs []string,
	misses []string,
	missListPath, missFreshPath, missDepsPath string,
	verbose bool,
	tracker perf.Tracker,
	opts InvocationOptions,
) (*OracleData, *CacheDepsFile, bool, error) {
	fallback := func(reason string) (*OracleData, *CacheDepsFile, bool, error) {
		if verbose {
			fmt.Fprintf(os.Stderr, "verbose: daemon cache path falling back to one-shot: %s\n", reason)
		}
		addOracleInstant(tracker, "missAnalysisFallback", nil, map[string]string{"reason": reason})
		if err := runKritTypesCached(jarPath, sourceDirs, missListPath, missFreshPath, missDepsPath, verbose, tracker, opts); err != nil {
			return nil, nil, false, err
		}
		var fresh *OracleData
		if err := trackOracle(tracker, "readFreshOracleJSON", func() error {
			var err error
			fresh, err = readOracleJSON(missFreshPath)
			return err
		}); err != nil {
			return nil, nil, false, fmt.Errorf("read fresh oracle: %w", err)
		}
		// Same swallowed-error policy as the pre-daemon code — missing
		// or malformed cache-deps is non-fatal, we just skip cache writes.
		var deps *CacheDepsFile
		trackOracle(tracker, "readCacheDepsJSON", func() error {
			deps, _ = LoadCacheDeps(missDepsPath)
			return nil
		})
		return fresh, deps, false, nil
	}

	shards := configuredKritTypesShards(len(misses))
	if shards > 1 {
		if verbose {
			fmt.Fprintf(os.Stderr, "verbose: sharding miss analysis across %d krit-types JVM workers (%d files)\n", shards, len(misses))
		}
		// KRIT_TYPES_SHARDS is an explicit one-shot JVM experiment: bypass
		// the daemon so the miss list is actually processed by multiple
		// independent Analysis API workers.
		runner := func(jarPath string, sourceDirs []string, missListPath, freshOutPath, depsOutPath string, verbose bool, tracker perf.Tracker) error {
			shardOpts := opts
			shardOpts.Tracker = tracker
			return runKritTypesCached(jarPath, sourceDirs, missListPath, freshOutPath, depsOutPath, verbose, tracker, shardOpts)
		}
		fresh, deps, err := runKritTypesCachedShardedWithRunner(jarPath, sourceDirs, misses, shards, verbose, tracker, runner)
		if err != nil {
			addOracleInstant(tracker, "shardedMissAnalysisFallback", nil, map[string]string{"error": err.Error()})
			return fallback(fmt.Sprintf("sharded miss analysis failed: %v", err))
		}
		return fresh, deps, false, nil
	}

	// Opt-out knob: KRIT_DAEMON_CACHE=off forces the one-shot path
	// for diagnostics and baseline reproduction. Any other value
	// (including unset) takes the daemon path.
	if strings.EqualFold(os.Getenv("KRIT_DAEMON_CACHE"), "off") {
		return fallback("KRIT_DAEMON_CACHE=off")
	}

	poolSize := configuredDaemonPoolSize(len(misses))
	if shouldUseDaemonPool(len(misses), poolSize) {
		var pool *DaemonPool
		if err := trackOracle(tracker, "daemonPoolConnectOrStart", func() error {
			var err error
			pool, err = ConnectOrStartDaemonPool(jarPath, sourceDirs, nil, poolSize, verbose)
			return err
		}); err != nil {
			return fallback(fmt.Sprintf("ConnectOrStartDaemonPool: %v", err))
		}
		defer pool.Release()
		addOracleInstant(tracker, "daemonPoolConnectOrStartSummary", map[string]int64{
			"requested": int64(pool.Requested),
			"connected": int64(pool.Connected),
			"started":   int64(pool.Started),
		}, nil)
		if !pool.MatchesRepo(sourceDirs) {
			addOracleInstant(tracker, "daemonPoolRepoMismatch", map[string]int64{"misses": int64(len(misses))}, nil)
			return fallback("daemon pool sourceDirs mismatch")
		}
		if verbose {
			fmt.Fprintf(os.Stderr, "verbose: sharding daemon miss analysis across %d persistent workers (%d files)\n", len(pool.Members), len(misses))
		}
		fresh, deps, err := pool.AnalyzeWithDepsSharded(misses, tracker != nil && tracker.IsEnabled(), opts.CallFilter, tracker)
		if err != nil {
			addOracleInstant(tracker, "daemonPoolMissAnalysisFallback", nil, map[string]string{"error": err.Error()})
			return fallback(fmt.Sprintf("daemon pool AnalyzeWithDeps: %v", err))
		}
		return fresh, deps, true, nil
	}
	if poolSize > 1 {
		addOracleInstant(tracker, "daemonPoolBypass", map[string]int64{
			"poolSize":  int64(poolSize),
			"misses":    int64(len(misses)),
			"threshold": int64(daemonPoolMinMisses),
		}, map[string]string{"reason": "smallMissSet"})
	}

	var d *Daemon
	if err := trackOracle(tracker, "daemonConnectOrStart", func() error {
		var err error
		d, err = ConnectOrStartDaemon(jarPath, sourceDirs, nil, verbose)
		return err
	}); err != nil {
		return fallback(fmt.Sprintf("ConnectOrStartDaemon: %v", err))
	}
	// Release (not Close): drops the TCP connection but leaves the
	// daemon process alive for the next krit invocation to find via
	// the per-repo PID file. The daemon self-terminates on its
	// 30-minute idle timeout if no new client connects. Using Close
	// here would shut down the daemon and wipe the PID file on every
	// invocation, defeating the whole purpose of the persistent daemon.
	defer d.Release()

	if !d.MatchesRepo(sourceDirs) {
		addOracleInstant(tracker, "daemonRepoMismatch", map[string]int64{"misses": int64(len(misses))}, nil)
		return fallback("daemon sourceDirs mismatch")
	}

	var fresh *OracleData
	var deps *CacheDepsFile
	var kotlinTimings []perf.TimingEntry
	if err := trackOracle(tracker, "daemonAnalyzeWithDeps", func() error {
		var err error
		fresh, deps, kotlinTimings, err = d.AnalyzeWithDepsWithTimings(misses, tracker != nil && tracker.IsEnabled(), opts.CallFilter)
		return err
	}); err != nil {
		return fallback(fmt.Sprintf("AnalyzeWithDeps: %v", err))
	}
	if len(kotlinTimings) > 0 {
		kt := tracker.Serial("kotlinTimings")
		perf.AddEntries(kt, kotlinTimings)
		kt.End()
	}

	// AnalyzeWithDeps no longer returns ErrDaemonFileNotInSession — instead
	// it folds "file not found in source module" errors into
	// deps.Crashed so the caller writes poison markers for them via
	// the existing WriteFreshEntries path. This matches the one-shot
	// jar-skipped-poison behavior and eliminates the rebuild-retry
	// cost that was doubling cold-run wall time on large repos.

	return fresh, deps, true, nil
}

// readOracleJSON parses a krit-types output file into OracleData. Shares
// format with oracle.Load but returns the raw struct rather than a fully
// indexed Oracle — the caller (InvokeCached) wants to merge with cache
// hits before indexing.
func readOracleJSON(path string) (*OracleData, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var od OracleData
	if err := json.Unmarshal(data, &od); err != nil {
		return nil, err
	}
	return &od, nil
}

// writeOracleJSON writes a merged OracleData to disk as JSON. Pretty
// printing is avoided to keep the file compact — the existing consumers
// (oracle.Load) parse via encoding/json which is indent-insensitive.
func writeOracleJSON(path string, data *OracleData) error {
	if data.Files == nil {
		data.Files = map[string]*OracleFile{}
	}
	if data.Dependencies == nil {
		data.Dependencies = map[string]*OracleClass{}
	}
	if data.Version == 0 {
		data.Version = 1
	}
	b, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("marshal oracle: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("mkdir %s: %w", filepath.Dir(path), err)
	}
	if err := fsutil.WriteFileAtomic(path, b, 0o644); err != nil {
		return fmt.Errorf("write oracle json: %w", err)
	}
	return nil
}
