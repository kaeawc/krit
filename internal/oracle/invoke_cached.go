package oracle

// Cache-aware oracle invocation.
//
// InvokeCached wraps Invoke with an on-disk incremental cache keyed by
// (content hash, closure fingerprint). On a cold run it delegates to a
// full krit-fir launch and writes per-file cache entries from the
// accompanying --cache-deps-out JSON. On a warm run it partitions source
// files into hits (served from cache, no JVM) and misses (re-analyzed via
// krit-fir with --files LISTFILE), then assembles a merged Data
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
	"strings"
	"time"

	"github.com/kaeawc/krit/internal/fileignore"
	"github.com/kaeawc/krit/internal/fsutil"
	"github.com/kaeawc/krit/internal/gradlemodel"
	"github.com/kaeawc/krit/internal/hashutil"
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
// krit-fir Kotlin side. Files whose absolute path contains any of these
// substrings are skipped by the JVM-side analyze loop; we apply the same
// filter Go-side before classify so excluded files don't leak into the
// miss list. If this drifts from the Kotlin default, krit-fir wins
// (the jar's filter is authoritative); Go just avoids extra work.
var defaultExcludeSubstrings = []string{
	"/testData/",
	"/test-resources/",
}

// excludedByDefault returns true if path matches any default exclude
// pattern. Uses substring matching rather than glob matching to avoid a
// dependency; the krit-fir default patterns (**/testData/** and
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

// CollectKtFiles walks the given source directories and returns
// absolute paths of all .kt files, matching what the JVM-side
// enumeration sees (DefaultPrunedDir + gitignore + krit-fir
// default exclude patterns, plus testData/test-resources).
func CollectKtFiles(sourceDirs []string) ([]string, error) {
	return collectKotlinSourceFiles(sourceDirs)
}

func collectKotlinSourceFiles(sourceDirs []string) ([]string, error) {
	var out []string
	seen := map[string]bool{}
	ignoreMatchers := make(map[string]*fileignore.Matcher)
	for _, root := range sourceDirs {
		rootInfo, statErr := os.Stat(root)
		if statErr != nil {
			return nil, statErr
		}
		matcher := fileignore.MatcherForPath(root, rootInfo, ignoreMatchers)
		walkErr := filepath.Walk(root, func(p string, info os.FileInfo, err error) error {
			if err != nil {
				return nil //nolint:nilerr // Walk callback skip-and-continue: per-entry error means skip this entry
			}
			if info.IsDir() {
				base := filepath.Base(p)
				if fileignore.DefaultPrunedDir(base) || matcher.Ignored(p, true) {
					return filepath.SkipDir
				}
				if base == "testData" || base == "test-resources" {
					return filepath.SkipDir
				}
				return nil
			}
			// Match the krit-fir JVM side: KtFile includes both .kt and
			// .kts (Kotlin script — used by build-logic gradle files).
			// Limiting Go-side collection to .kt only would cause the
			// cache path to silently drop .kts files that the plain
			// Invoke path would have analyzed.
			name := info.Name()
			if !strings.HasSuffix(name, ".kt") && !strings.HasSuffix(name, ".kts") {
				return nil
			}
			if matcher.Ignored(p, false) {
				return nil
			}
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
		if walkErr != nil {
			return nil, walkErr
		}
	}
	return out, nil
}

// InvokeCached is the cache-aware variant of Invoke. It walks sourceDirs,
// classifies .kt files into hits and misses via the on-disk cache, runs
// krit-fir only on misses (with --files + --cache-deps-out), writes new
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

// applyOracleFilter intersects ktFiles with the filter list at filterListPath.
// Returns the filtered slice; on any read error returns ktFiles unchanged.
func applyOracleFilter(ktFiles []string, filterListPath string, tracker perf.Tracker, verbose bool) []string {
	start := time.Now()
	wanted, ferr := readFilterListFile(filterListPath)
	if ferr != nil {
		if verbose {
			reporter().Verbosef("verbose: read filter list %s: %v (ignoring filter)\n", filterListPath, ferr)
		}
		addOracleEntry(tracker, "readOracleFilterList", start, nil, map[string]string{"error": ferr.Error()})
		return ktFiles
	}
	// The filter list holds absolute paths while ktFiles is spelled like the
	// scan root (relative for `krit .`), so compare in absolute form. The
	// result keeps the caller's spelling of each file.
	wantedAbs := make(map[string]bool, len(wanted))
	for p := range wanted {
		wantedAbs[filepath.Clean(AbsolutePath(p))] = true
	}
	before := len(ktFiles)
	filtered := ktFiles[:0]
	for _, p := range ktFiles {
		if wantedAbs[filepath.Clean(AbsolutePath(p))] {
			filtered = append(filtered, p)
		}
	}
	addOracleEntry(tracker, "applyOracleFilterList", start, map[string]int64{
		"before": int64(before),
		"after":  int64(len(filtered)),
		"wanted": int64(len(wanted)),
	}, nil)
	if verbose {
		reporter().Verbosef("verbose: cache filter intersection: %d/%d files after oracle-filter\n", len(filtered), before)
	}
	return filtered
}

// writeFreshCacheEntries persists freshData/depsFile to the cache (either via
// the async CacheWriter or synchronously) and logs the result.
func writeFreshCacheEntries(
	s *store.FileStore,
	cacheDir string,
	freshData *Data,
	depsFile *CacheDepsFile,
	callFilterScope, declarationProfileScope string,
	tracker perf.Tracker,
	opts InvocationOptions,
	verbose bool,
) {
	if opts.CacheWriter != nil {
		start := time.Now()
		queued, _ := opts.CacheWriter.QueueFreshEntriesToStoreScopedV2(s, cacheDir, freshData, depsFile, callFilterScope, declarationProfileScope)
		perf.AddEntryDetails(tracker, "queueFreshCacheEntries", time.Since(start), map[string]int64{"queued": int64(queued)}, nil)
		addOracleInstant(tracker, "freshCacheEntriesQueued", map[string]int64{"entries": int64(queued)}, nil)
		if verbose {
			reporter().Verbosef("verbose: queued %d new cache entries\n", queued)
		}
		return
	}
	writeTracker := tracker.Serial("writeFreshCacheEntries")
	written, _ := WriteFreshEntriesToStoreWithTrackerScopedV2(s, cacheDir, freshData, depsFile, writeTracker, callFilterScope, declarationProfileScope)
	writeTracker.End()
	addOracleInstant(tracker, "freshCacheEntriesWritten", map[string]int64{"entries": int64(written)}, nil)
	if verbose {
		reporter().Verbosef("verbose: wrote %d new cache entries\n", written)
	}
}

// minUnexplainedForMismatch is how many requested files the JVM must leave
// entirely unanswered, with no expected reason, before that is treated as a
// request mismatch rather than as skipped sources. One is enough: krit-fir
// answers for every in-root file at or under the size limit, even an empty
// one, a package-only file, a script or an expect declaration, so an
// unanswered one is never a normal skip and a marker for it would hide the
// file from later runs.
const minUnexplainedForMismatch = 1

// maxJVMSourceBytes is IntelliJ's idea.max.intellisense.filesize default
// (FileUtilRt.getUserFileSizeLimit: 2500 KB, so 2,560,000 bytes). An
// unanswered file above it is treated as an expected skip rather than a
// request mismatch; krit-fir answers oversized files, so in practice the
// guard only explains files a stale or custom jar dropped.
const maxJVMSourceBytes = 2560000

// pathUnderRoot reports whether the clean absolute path abs is root or lies
// below it. A sibling sharing a name prefix (/a/rootX for /a/root) is not under
// it, and a root that already ends in a separator (a filesystem root such as
// / or C:\) is not given a second one.
func pathUnderRoot(abs, root string) bool {
	if abs == root {
		return true
	}
	prefix := root
	if !strings.HasSuffix(prefix, string(filepath.Separator)) {
		prefix += string(filepath.Separator)
	}
	return strings.HasPrefix(abs, prefix)
}

// expectedlySkipped reports whether the JVM is expected not to answer for p:
// it is over the Analysis API size limit, or it lies outside every source
// root (the JVM only builds KtFiles for files under them). Anything else it
// leaves unanswered is unexplained.
func expectedlySkipped(p string, absRoots []string) bool {
	abs := filepath.Clean(AbsolutePath(p))
	underRoot := false
	for _, root := range absRoots {
		if pathUnderRoot(abs, root) {
			underRoot = true
			break
		}
	}
	if !underRoot {
		return true
	}
	info, err := os.Stat(abs)
	return err != nil || info.Size() > maxJVMSourceBytes
}

// unexplainedMisses returns the misses to withhold skipped markers from: none
// unless the JVM answered for none of them and at least
// minUnexplainedForMismatch have no expected reason to be skipped.
func unexplainedMisses(sourceDirs, misses []string, analyzed map[string]bool) map[string]bool {
	for _, p := range misses {
		if analyzed[p] {
			return nil
		}
	}
	absRoots := make([]string, len(sourceDirs))
	for i, d := range sourceDirs {
		absRoots[i] = filepath.Clean(AbsolutePath(d))
	}
	unexplained := map[string]bool{}
	for _, p := range misses {
		if !expectedlySkipped(p, absRoots) {
			unexplained[p] = true
		}
	}
	if len(unexplained) < minUnexplainedForMismatch {
		return nil
	}
	return unexplained
}

// writeSkippedPoisonEntries writes jar-skipped poison cache entries for misses
// that freshData/depsFile did not cover, preventing them from re-entering miss
// lists on future warm runs.
//
// When the JVM answered for none of the misses and at least
// minUnexplainedForMismatch of them have no expected reason to be skipped
// (see expectedlySkipped), that is a request/response mismatch rather than
// skipped sources, and marking those files skipped would poison the cache for
// later runs: they get no marker, with a warning. Files with an expected
// reason, such as oversized sources, always get theirs.
func writeSkippedPoisonEntries(
	s *store.FileStore,
	cacheDir string,
	sourceDirs []string,
	misses []string,
	freshData *Data,
	depsFile *CacheDepsFile,
	callFilterScope, declarationProfileScope string,
	tracker perf.Tracker,
	verbose bool,
) {
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
	unexplained := unexplainedMisses(sourceDirs, misses, analyzed)
	if len(unexplained) > 0 {
		reporter().Warnf("warning: oracle analyzed 0 of %d requested files; not caching skipped markers for %d unexplained files\n", len(misses), len(unexplained))
		addOracleInstant(tracker, "skippedPoisonEntriesSuppressed", map[string]int64{"requested": int64(len(misses)), "unexplained": int64(len(unexplained))}, nil)
	}
	skipped := 0
	_ = trackOracle(tracker, "writeSkippedPoisonEntries", func() error {
		for _, p := range misses {
			if analyzed[p] || unexplained[p] {
				continue
			}
			hash, herr := ContentHash(p)
			if herr != nil {
				continue
			}
			entry := &CacheEntry{
				V:           CacheVersion,
				ContentHash: hash,
				// Absolute, so a relative FilePath on a skipped marker only
				// ever means a marker written by the relative-root bug.
				FilePath:                      AbsolutePath(p),
				Crashed:                       true,
				CrashError:                    skipPoisonMarker + " file not in Analysis API KtFile set (typically oversized source)",
				Approximation:                 depsFile.approximation(),
				CompilationFingerprint:        depsFile.compilationFingerprint(),
				CallFilterFingerprint:         callFilterScope,
				DeclarationProfileFingerprint: declarationProfileScope,
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
		reporter().Verbosef("verbose: wrote %d jar-skipped poison entries\n", skipped)
	}
}

// assembleAndWriteOracle merges hits+freshData and writes the combined oracle
// JSON to outputPath. Also emits verbose cache-stats when requested.
func assembleAndWriteOracle(
	hits []*CacheEntry,
	freshData *Data,
	outputPath string,
	cacheDir string,
	tracker perf.Tracker,
	verbose bool,
) (string, error) {
	var merged *Data
	_ = trackOracle(tracker, "assembleOracle", func() error {
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
		_ = trackOracle(tracker, "cacheStats", func() error {
			count, bytes, _ = CacheStats(cacheDir)
			return nil
		})
		reporter().Verbosef("verbose: cache now has %d entries, %d bytes total\n", count, bytes)
		addOracleInstant(tracker, "cacheStatsValues", map[string]int64{"entries": int64(count), "bytes": bytes}, nil)
	}
	return outputPath, nil
}

// splitForcedMisses partitions ktFiles into the subset that should
// be passed through the cache classifier and the subset that the
// caller has already pre-decided are stale. Paths in forcedMisses
// that are NOT in ktFiles are silently dropped — the caller's hint
// may include paths that fell out of the current source set (e.g.
// after a path filter) and we must not force-analyze something the
// JVM wasn't asked to look at.
//
// Returns (classifyInput, forcedInScan): classifyInput is the
// subset whose membership in the cache will be checked by
// ClassifyFilesWithStoreScopedV2; forcedInScan is the subset that
// will be appended to the resulting miss list verbatim. When
// forcedMisses is empty both slices reuse ktFiles' backing array
// so the common path allocates nothing.
func splitForcedMisses(ktFiles []string, forcedMisses []string) ([]string, []string) {
	if len(forcedMisses) == 0 {
		return ktFiles, nil
	}
	// Compare in absolute form: the hint and the scan may spell one file
	// differently (`./x` vs `x`). The result keeps ktFiles' spelling.
	forcedSet := make(map[string]bool, len(forcedMisses))
	for _, p := range forcedMisses {
		forcedSet[filepath.Clean(AbsolutePath(p))] = true
	}
	classifyInput := make([]string, 0, len(ktFiles))
	forcedInScan := make([]string, 0, len(forcedSet))
	for _, p := range ktFiles {
		if forcedSet[filepath.Clean(AbsolutePath(p))] {
			forcedInScan = append(forcedInScan, p)
			continue
		}
		classifyInput = append(classifyInput, p)
	}
	return classifyInput, forcedInScan
}

// recordForcedMissCount emits a perf event when the caller supplied a
// forced-miss hint. Kept as a helper to bound the cyclomatic complexity
// of InvokeCachedWithOptions.
func recordForcedMissCount(tracker perf.Tracker, declared, inScan int) {
	if declared <= 0 {
		return
	}
	addOracleInstant(tracker, "forcedMisses", map[string]int64{
		"declared": int64(declared),
		"inScan":   int64(inScan),
	}, nil)
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
			reporter().Verbosef("verbose: cache dir init failed (%v), falling back to full run\n", err)
		}
		addOracleInstant(tracker, "cacheBypass", nil, map[string]string{"reason": "cacheDirInit"})
		return InvokeWithFilesWithOptions(jarPath, sourceDirs, outputPath, filterListPath, verbose, opts)
	}
	cacheDir = legacyEntryDirForRun(s, cacheDir, opts)

	var ktFiles []string
	if err := trackOracle(tracker, "collectKtFiles", func() error {
		var err error
		ktFiles, err = collectKotlinSourceFiles(sourceDirs)
		return err
	}); err != nil || len(ktFiles) == 0 {
		if verbose {
			reporter().Verbosef("verbose: no .kt files discovered for cache; running full oracle\n")
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
	declarationProfileScope := factProfileScope(opts)

	compilation := wholeCompilationFingerprint(opts, sourceDirs, jarPath, tracker)

	if filterListPath != "" {
		ktFiles = applyOracleFilter(ktFiles, filterListPath, tracker, verbose)
	}

	classifyInput, forcedInScan := splitForcedMisses(ktFiles, opts.ForcedMisses)
	recordForcedMissCount(tracker, len(opts.ForcedMisses), len(forcedInScan))

	startClassify := time.Now()
	hits, misses := ClassifyFilesWithStoreScopedV3(s, cacheDir, classifyInput, callFilterScope, declarationProfileScope, opts.Backend.CacheApproximation())
	if len(forcedInScan) > 0 {
		misses = append(misses, forcedInScan...)
	}
	misses = requestCompilationRefresh(opts, hits, misses, compilation, tracker, verbose)
	classifyElapsed := time.Since(startClassify)
	perf.AddEntryDetails(tracker, "cacheClassify", classifyElapsed, map[string]int64{
		"files":  int64(len(ktFiles)),
		"hits":   int64(len(hits)),
		"misses": int64(len(misses)),
	}, nil)
	if verbose {
		reporter().Verbosef("verbose: cache classify: %d hits, %d misses (%s, %d files)\n",
			len(hits), len(misses), classifyElapsed, len(ktFiles))
	}

	if len(misses) == 0 {
		var merged *Data
		_ = trackOracle(tracker, "assembleOracleFromCache", func() error {
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
			_ = trackOracle(tracker, "cacheStats", func() error {
				count, bytes, _ = CacheStats(cacheDir)
				return nil
			})
			reporter().Verbosef("verbose: oracle served entirely from cache (%d entries, %d bytes)\n", count, bytes)
			addOracleInstant(tracker, "cacheStatsValues", map[string]int64{"entries": int64(count), "bytes": bytes}, nil)
		}
		return outputPath, nil
	}

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

	freshData, depsFile, usedDaemon, err := missAnalyzer(
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
		reporter().Verbosef("verbose: miss analysis via %s\n", source)
	}

	stampCacheDeps(depsFile, opts, compilation)
	if depsFile == nil {
		if verbose {
			reporter().Verbosef("verbose: no cache deps returned; cache not updated\n")
		}
	} else {
		writeFreshCacheEntries(s, cacheDir, freshData, depsFile, callFilterScope, declarationProfileScope, tracker, opts, verbose)
	}

	writeSkippedPoisonEntries(s, cacheDir, sourceDirs, misses, freshData, depsFile, callFilterScope, declarationProfileScope, tracker, verbose)

	return assembleAndWriteOracle(hits, freshData, outputPath, cacheDir, tracker, verbose)
}

// wholeCompilationFingerprint returns the current compilation's fingerprint
// for a whole-compilation backend, or "" for other backends. The backend's
// facts depend on every file it compiles, including generated sources and
// files the oracle filter drops, so the fingerprint covers its own source
// enumeration rather than the classified files.
func wholeCompilationFingerprint(opts InvocationOptions, sourceDirs []string, jarPath string, tracker perf.Tracker) string {
	if !opts.Backend.ReturnsWholeCompilation() {
		return ""
	}
	var compilation string
	_ = trackOracle(tracker, "compilationFingerprint", func() error {
		compilation = CompilationFingerprint(CompilationSources(sourceDirs), opts.Classpath, jarPath)
		return nil
	})
	return compilation
}

// requestCompilationRefresh returns the misses to analyze. Every run of a
// whole-compilation backend refreshes every file's facts, so any miss already
// refreshes stale hits too. With no misses, a hit computed against a
// different compilation (a dependency outside the classified files changed, a
// file was deleted, or the classpath moved) still needs one run: it is
// requested for that file alone, and the backend answers with facts for the
// whole compilation.
func requestCompilationRefresh(opts InvocationOptions, hits []*CacheEntry, misses []string, compilation string, tracker perf.Tracker, verbose bool) []string {
	if !opts.Backend.ReturnsWholeCompilation() || len(misses) > 0 {
		return misses
	}
	stale := staleCompilationHit(hits, compilation)
	if stale == "" {
		return misses
	}
	addOracleInstant(tracker, "compilationChanged", nil, map[string]string{"refresh": stale})
	if verbose {
		reporter().Verbosef("verbose: cache classify: compilation changed; refreshing through %s\n", stale)
	}
	return []string{stale}
}

// stampCacheDeps records which backend and compilation produced depsFile, so
// the entries written from it say what Go ran rather than what the jar
// reported about itself.
func stampCacheDeps(depsFile *CacheDepsFile, opts InvocationOptions, compilation string) {
	if depsFile == nil || opts.Backend == "" {
		return
	}
	depsFile.Approximation = opts.Backend.CacheApproximation()
	depsFile.CompilationFingerprint = compilation
}

// readOneShotMissResult reads a one-shot JVM run's facts and cache deps and
// re-spells their absolute paths to the caller's spelling of requested and
// sourceDirs. A cache-deps read failure is returned separately (depsErr) so
// callers can choose to ignore it; err is set only when the facts are unreadable.
func readOneShotMissResult(tracker perf.Tracker, freshPath, depsPath string, sourceDirs, requested []string) (fresh *Data, deps *CacheDepsFile, depsErr, err error) {
	if err = trackOracle(tracker, "readFreshOracleJSON", func() error {
		var readErr error
		fresh, readErr = readOracleJSON(freshPath)
		return readErr
	}); err != nil {
		return nil, nil, nil, err
	}
	_ = trackOracle(tracker, "readCacheDepsJSON", func() error {
		deps, depsErr = LoadCacheDeps(depsPath)
		return nil
	})
	spelling := oneShotSpelling(sourceDirs, requested)
	spelling.CallerData(fresh)
	spelling.CallerCacheDeps(deps)
	return fresh, deps, depsErr, nil
}

// oneShotSpelling maps the absolute paths a one-shot JVM run reports back to
// the caller's spelling of the requested files and source roots, so a
// relative scan root (`krit .`) keeps the cache and merge keys it had before
// the request was absolutized.
func oneShotSpelling(sourceDirs, requested []string) PathSpelling {
	_, spelling := AbsoluteRequestPaths(requested)
	return spelling.WithDirs(sourceDirs)
}

// missListLines returns the --files entries for misses: absolute, and with
// forward slashes, which is how krit-fir's virtual file system spells paths
// on every platform (Main.kt compares entries by exact equality).
func missListLines(misses []string) []string {
	abs := AbsolutePaths(misses)
	out := make([]string, len(abs))
	for i, p := range abs {
		out[i] = filepath.ToSlash(p)
	}
	return out
}

// prepareMissTemps creates three tempfiles for the miss-run round trip:
//
//	missListPath — newline-separated absolute paths for --files
//	missFreshPath — krit-fir --output target (we'll read back)
//	missDepsPath  — krit-fir --cache-deps-out target (we'll read back)
//
// The caller is responsible for removing all three.
func prepareMissTemps(misses []string) (string, string, string, error) {
	f, err := os.CreateTemp("", "krit-miss-list-*.txt")
	if err != nil {
		return "", "", "", fmt.Errorf("tempfile (miss list): %w", err)
	}
	// krit-fir matches --files entries by exact equality with the absolute
	// virtual file path, so a relative scan root must be absolutized here.
	for _, p := range missListLines(misses) {
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

	// Match the daemon launch's arg shape so the one-shot JVM gets
	// the same Leyden AOT / AppCDS startup-cache acceleration. On
	// the first call this writes the per-jar `.aotconf` / `.jsa`
	// under ~/.krit/cache/; subsequent one-shot calls reuse them
	// and cut JVM startup cost roughly in half.
	args := buildJVMBaseArgs()
	args = appendStartupCacheArgs(args, javaPath, jarPath, verbose)
	args = append(args,
		"-jar", jarPath,
		"--sources", strings.Join(AbsolutePaths(sourceDirs), ","),
		"--output", freshOutPath,
		"--files", missListPath,
		"--cache-deps-out", depsOutPath,
	)
	args = append(args, compileContextArgs(jarPath, opts)...)
	rider, riderArgs, err := startCheckRider(opts)
	if err != nil {
		return err
	}
	defer rider.cleanup()
	args = append(args, riderArgs...)
	extraJVMArgs := configuredExtraJVMArgs(opts)
	args = appendExtraJVMArgsBeforeJar(args, extraJVMArgs)
	recordKritTypesJVMArgs(tracker, extraJVMArgs)
	rider.noteLaunch(args)
	if parallelArg := experimentalParallelFilesArg(); len(parallelArg) > 0 {
		args = append(args, parallelArg...)
	}
	callFilterPath, cleanupCallFilter, err := writeCallFilterArg(opts, tracker)
	if err != nil {
		return fmt.Errorf("call filter: %w", err)
	}
	defer cleanupCallFilter()
	if callFilterPath != "" {
		args = append(args, "--call-filter", callFilterPath)
	}
	if profileArg := declarationProfileCLIValue(opts); profileArg != "" {
		args = append(args, "--declaration-profile", profileArg)
	}
	if opts.DisableDiagnostics {
		args = append(args, "--no-diagnostics")
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
		// Whatever jar the caller passed in — krit-fir.
		reporter().Verbosef("verbose: Running oracle JVM (cached) jar=%s: %s %s\n", filepath.Base(jarPath), javaPath, strings.Join(args, " "))
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
	rider.finish(tracker, trackErr)
	if trackErr != nil {
		return trackErr
	}
	addKotlinTimingsFromFile(tracker, timingsPath)
	return runErr
}

func configuredDaemonCacheMode() string {
	switch strings.ToLower(strings.TrimSpace(os.Getenv("KRIT_DAEMON_CACHE"))) {
	case "off", "false", "0", "one-shot", "oneshot":
		return "off"
	case "on", "true", "1", "daemon":
		return "on"
	default:
		return ""
	}
}

// daemonPreferredMissThreshold is the upper bound on miss count below
// which the persistent single-daemon path beats parallel one-shot by
// a wide margin. Reasoning: parallel one-shot's win comes from running
// `defaultKritTypesParallelFiles` analyses concurrently inside a fresh
// JVM, paying JVM cold-start (~30-60s on first run, ~JIT-warm time
// subsequently) once per invocation. For a small miss set the
// parallelism can't accelerate the work — a 1-file or 2-file analysis
// finishes in milliseconds — so the cold-start cost dominates.
// A reused daemon serves those same misses without paying startup at
// all on warm reruns. Threshold of 8 leaves enough headroom that the
// occasional handful of ABI-driven changes still hit the fast path
// while keeping large cache-miss waves on the parallel one-shot path.
//
// KRIT_DAEMON_CACHE=on / =off still wins.
const daemonPreferredMissThreshold = 8

func shouldUseOneShotMissAnalysis(misses int) (bool, string) {
	switch configuredDaemonCacheMode() {
	case "off":
		return true, "KRIT_DAEMON_CACHE=off"
	case "on":
		return false, ""
	}
	// Small miss sets: the persistent daemon amortizes JVM cold-start
	// across reruns, which dominates the per-file analysis cost for
	// ABI-sized changes (typically 1-few files).
	if misses > 0 && misses <= daemonPreferredMissThreshold {
		return false, ""
	}
	if workers := configuredKritTypesParallelFiles(); workers > 1 {
		return true, fmt.Sprintf("default parallel one-shot (%d workers)", workers)
	}
	return false, ""
}

// missAnalyzer analyzes the misses. It is runMissAnalysis except in tests,
// which substitute recorded facts for the JVM.
var missAnalyzer = runMissAnalysis

// runMissAnalysis runs the miss-list analysis. The default cold-miss path is
// one-shot krit-fir with in-JVM file parallelism; explicit daemon settings
// can still route misses through persistent daemon workers. Returns
// (freshData, depsFile, usedDaemon, err).
//
// The daemon path remains useful for explicit warm-miss experiments because it
// amortizes the Analysis API session build across invocations. The one-shot
// path preserves the exact same output shape:
// runKritTypesCached writes tempfiles which are then loaded via readOracleJSON
// + LoadCacheDeps.
//
// Set KRIT_DAEMON_CACHE=on to force the persistent daemon path. Set
// KRIT_DAEMON_CACHE=off to force one-shot even when parallel file analysis is
// disabled. ConnectOrStartDaemon handles the "no daemon running" case by
// starting one when the daemon path is selected.
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
) (*Data, *CacheDepsFile, bool, error) {
	// nolint:unparam // (bool) is the daemonPathUsed flag that the outer
	// function returns; this lambda is the false branch by definition,
	// but returning the full tuple lets call sites stay symmetric with
	// the daemon-success path (`return fresh, deps, true, nil`).
	fallback := func(reason string) (*Data, *CacheDepsFile, bool, error) {
		if verbose {
			reporter().Verbosef("verbose: daemon cache path falling back to one-shot: %s\n", reason)
		}
		addOracleInstant(tracker, "missAnalysisFallback", nil, map[string]string{"reason": reason})
		if err := runKritTypesCached(jarPath, sourceDirs, missListPath, missFreshPath, missDepsPath, verbose, tracker, opts); err != nil {
			return nil, nil, false, err
		}
		// Same swallowed-error policy as the pre-daemon code: missing or
		// malformed cache-deps is non-fatal, we just skip cache writes.
		fresh, deps, _, err := readOneShotMissResult(tracker, missFreshPath, missDepsPath, sourceDirs, misses)
		if err != nil {
			return nil, nil, false, fmt.Errorf("read fresh oracle: %w", err)
		}
		return fresh, deps, false, nil
	}

	if useOneShot, reason := oneShotMissAnalysisReason(len(misses), opts); useOneShot {
		return fallback(reason)
	}

	var d *Daemon
	if err := trackOracle(tracker, "daemonConnectOrStart", func() error {
		var err error
		d, err = ConnectOrStartDaemon(jarPath, sourceDirs, opts.Classpath, verbose, WithKotlinSettings(opts.Kotlin))
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
	defer func() { _ = d.Release() }()

	var matchErr error
	d, matchErr = reconnectMismatchedDaemon(d, jarPath, sourceDirs, opts.Classpath, opts.Kotlin, verbose, len(misses), tracker)
	if matchErr != nil {
		return fallback(matchErr.Error())
	}

	var fresh *Data
	var deps *CacheDepsFile
	var kotlinTimings []perf.TimingEntry
	if err := trackOracle(tracker, "daemonAnalyzeWithDeps", func() error {
		var err error
		fresh, deps, kotlinTimings, err = d.AnalyzeWithDepsWithTimings(misses, tracker != nil && tracker.IsEnabled(), opts.CallFilter, opts.DeclarationProfile)
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

func oneShotMissAnalysisReason(misses int, opts InvocationOptions) (bool, string) {
	// The generic oracle daemon registry does not key JVM target. An
	// explicit FIR target uses the one-shot runner and its scoped cache.
	if opts.JvmTarget != "" {
		return true, "FIR JVM target requires target-scoped compilation"
	}
	return shouldUseOneShotMissAnalysis(misses)
}

func reconnectMismatchedDaemon(d *Daemon, jarPath string, sourceDirs, classpath []string, kt gradlemodel.KotlinSettings, verbose bool, misses int, tracker perf.Tracker) (*Daemon, error) {
	if d.MatchesRepoSettings(jarPath, sourceDirs, kt, classpath...) {
		return d, nil
	}
	addOracleInstant(tracker, "daemonRepoMismatch", map[string]int64{"misses": int64(misses)}, nil)
	_ = d.Release()
	replacement, err := ConnectOrStartDaemon(jarPath, sourceDirs, classpath, verbose, WithKotlinSettings(kt))
	if err != nil {
		return d, fmt.Errorf("reconnect daemon: %w", err)
	}
	if !replacement.MatchesRepoSettings(jarPath, sourceDirs, kt, classpath...) {
		_ = replacement.Release()
		return replacement, fmt.Errorf("daemon (jar, sourceDirs) mismatch after reconnect")
	}
	return replacement, nil
}

// readOracleJSON parses a krit-fir output file into Data. Shares
// format with oracle.Load but returns the raw struct rather than a fully
// indexed Oracle — the caller (InvokeCached) wants to merge with cache
// hits before indexing.
func readOracleJSON(path string) (*Data, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var od Data
	if err := json.Unmarshal(data, &od); err != nil {
		return nil, err
	}
	return &od, nil
}

// writeOracleJSON writes a merged Data to disk as JSON. Pretty
// printing is avoided to keep the file compact — the existing consumers
// (oracle.Load) parse via encoding/json which is indent-insensitive.
func writeOracleJSON(path string, data *Data) error {
	if data.Files == nil {
		data.Files = map[string]*File{}
	}
	if data.Dependencies == nil {
		data.Dependencies = map[string]*Class{}
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

// legacyScopesDir is the directory under the versioned cache root holding the
// per-file entries of runs with Kotlin compiler settings, one subdirectory per
// settings identity.
const legacyScopesDir = "scopes"

// maxLegacyScopes bounds how many settings- and classpath-scoped entry
// directories are kept; the least recently used beyond it are pruned. Classpath
// scopes share the budget, and a classpath that flips between IDE and shell
// runs (or across a few dependency updates) would thrash a smaller cap, so it
// is 8 rather than 4.
const maxLegacyScopes = 8

// legacyScopeGrace protects a scope directory touched recently: a long
// concurrent run under other settings may still be reading and writing it.
const legacyScopeGrace = 10 * time.Minute

// LegacyEntryDir is where the per-file entries of the legacy (no .krit/store)
// layout live for the given Kotlin settings. Without a store the entries carry
// no scope of their own, so a settings-only change plus one edit would leave
// unchanged files hitting facts computed under the old settings. Zero settings
// use cacheDir itself, byte-identical to before settings existed, so projects
// without settings never re-analyze. Every reader of these entries (InvokeCached
// and the freshness gate's reverse-dependency expansion) must use this function.
//
// The classpath (jar path plus size and mtime, as krit-fir fingerprints it) is
// part of the scope too: a changed dependency jar changes the facts of files
// whose own content did not change (issue #827). An empty classpath adds
// nothing, so classpath-less projects keep their existing directories.
// Directory entries are only stat'd at the top level and relative entries
// resolve against the working directory; both can only cause a miss, never a
// stale hit.
func LegacyEntryDir(cacheDir string, kt gradlemodel.KotlinSettings, classpath []string) string {
	hash := kt.IdentityHash()
	if len(classpath) > 0 {
		hash = hashutil.HashHex([]byte(kt.Identity() + "|classpath:" + gradlemodel.ClasspathFingerprint(classpath)))[:16]
	}
	if hash == "" {
		return cacheDir
	}
	return filepath.Join(cacheDir, legacyScopesDir, hash)
}

// legacyEntryDirForRun is the per-file entry directory for one invocation. With
// a store the scope lives in the store keys, so the directory is not created:
// an empty scope would only take an LRU slot and could prune a real one.
func legacyEntryDirForRun(s *store.FileStore, cacheDir string, opts InvocationOptions) string {
	if s != nil {
		return LegacyEntryDir(cacheDir, opts.Kotlin, opts.Classpath)
	}
	return prepareLegacyEntryDir(cacheDir, opts.Kotlin, opts.Classpath)
}

// prepareLegacyEntryDir returns LegacyEntryDir, marks it most recently used and
// prunes the least recently used scopes beyond maxLegacyScopes.
func prepareLegacyEntryDir(cacheDir string, kt gradlemodel.KotlinSettings, classpath []string) string {
	dir := LegacyEntryDir(cacheDir, kt, classpath)
	if dir == cacheDir {
		return dir
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return dir
	}
	now := time.Now()
	_ = os.Chtimes(dir, now, now)
	pruneLegacyScopes(filepath.Join(cacheDir, legacyScopesDir), filepath.Base(dir), maxLegacyScopes)
	return dir
}

func pruneLegacyScopes(root, keepName string, limit int) {
	entries, err := os.ReadDir(root)
	if err != nil {
		return
	}
	type scope struct {
		name  string
		mtime time.Time
	}
	var scopes []scope
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		scopes = append(scopes, scope{e.Name(), info.ModTime()})
	}
	if len(scopes) <= limit {
		return
	}
	sort.Slice(scopes, func(i, j int) bool {
		if scopes[i].name == keepName || scopes[j].name == keepName {
			return scopes[i].name == keepName
		}
		return scopes[i].mtime.After(scopes[j].mtime)
	})
	for _, old := range scopes[limit:] {
		if time.Since(old.mtime) < legacyScopeGrace {
			continue
		}
		_ = os.RemoveAll(filepath.Join(root, old.name))
	}
}
