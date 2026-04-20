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
	"strings"
	"time"

	"github.com/kaeawc/krit/internal/fsutil"
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
	if repoDir == "" {
		// Fall back to the filter-only path — we need a repo root to anchor
		// the cache dir, and without it the caching layer can't do its
		// job safely. Still honor the filter so we don't re-analyze files
		// no rule cares about.
		return InvokeWithFiles(jarPath, sourceDirs, outputPath, filterListPath, verbose)
	}
	cacheDir, err := CacheDir(repoDir)
	if err != nil {
		if verbose {
			fmt.Fprintf(os.Stderr, "verbose: cache dir init failed (%v), falling back to full run\n", err)
		}
		return InvokeWithFiles(jarPath, sourceDirs, outputPath, filterListPath, verbose)
	}

	ktFiles, err := CollectKtFiles(sourceDirs)
	if err != nil || len(ktFiles) == 0 {
		if verbose {
			fmt.Fprintf(os.Stderr, "verbose: no .kt files discovered for cache; running full oracle\n")
		}
		return InvokeWithFiles(jarPath, sourceDirs, outputPath, filterListPath, verbose)
	}

	// Apply the rule-classification filter (if any) before cache lookup:
	// files not in the filter set are dropped from both the hit-lookup and
	// the miss-analysis stages because no enabled rule has declared a need
	// for them. This makes the filter and the cache stack multiplicatively.
	if filterListPath != "" {
		wanted, ferr := readFilterListFile(filterListPath)
		if ferr != nil {
			if verbose {
				fmt.Fprintf(os.Stderr, "verbose: read filter list %s: %v (ignoring filter)\n", filterListPath, ferr)
			}
		} else {
			before := len(ktFiles)
			filtered := ktFiles[:0]
			for _, p := range ktFiles {
				if wanted[p] {
					filtered = append(filtered, p)
				}
			}
			ktFiles = filtered
			if verbose {
				fmt.Fprintf(os.Stderr, "verbose: cache filter intersection: %d/%d files after oracle-filter\n", len(ktFiles), before)
			}
		}
	}

	startClassify := time.Now()
	hits, misses := ClassifyFilesWithStore(s, cacheDir, ktFiles)
	classifyElapsed := time.Since(startClassify)
	if verbose {
		fmt.Fprintf(os.Stderr, "verbose: cache classify: %d hits, %d misses (%s, %d files)\n",
			len(hits), len(misses), classifyElapsed, len(ktFiles))
	}

	// Fast path: all hits. Assemble, write, return — no JVM launched.
	if len(misses) == 0 {
		merged := AssembleOracle(hits, nil)
		if err := writeOracleJSON(outputPath, merged); err != nil {
			return "", err
		}
		if verbose {
			count, bytes, _ := CacheStats(cacheDir)
			fmt.Fprintf(os.Stderr, "verbose: oracle served entirely from cache (%d entries, %d bytes)\n", count, bytes)
		}
		return outputPath, nil
	}

	// Slow path: there are misses. Prefer the persistent daemon — it
	// amortizes the Analysis API session build (~20-28 s on kotlin)
	// across invocations — and fall back to the one-shot jar on any
	// daemon failure. Tempfiles are prepared unconditionally because
	// the fallback path needs them.
	missListPath, missFreshPath, missDepsPath, err := prepareMissTemps(misses)
	if err != nil {
		return "", err
	}
	defer func() {
		_ = os.Remove(missListPath)
		_ = os.Remove(missFreshPath)
		_ = os.Remove(missDepsPath)
	}()

	freshData, depsFile, usedDaemon, err := runMissAnalysis(
		jarPath, sourceDirs, misses,
		missListPath, missFreshPath, missDepsPath, verbose,
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
		written, _ := WriteFreshEntriesToStore(s, cacheDir, freshData, depsFile)
		if verbose {
			fmt.Fprintf(os.Stderr, "verbose: wrote %d new cache entries\n", written)
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
	for _, p := range misses {
		if analyzed[p] {
			continue
		}
		hash, herr := ContentHash(p)
		if herr != nil {
			continue
		}
		entry := &CacheEntry{
			V:           CacheVersion,
			ContentHash: hash,
			FilePath:    p,
			Crashed:     true,
			CrashError:  "jar-skipped: file not in Analysis API KtFile set (typically oversized source)",
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
	if skipped > 0 && verbose {
		fmt.Fprintf(os.Stderr, "verbose: wrote %d jar-skipped poison entries\n", skipped)
	}

	merged := AssembleOracle(hits, freshData)
	if err := writeOracleJSON(outputPath, merged); err != nil {
		return "", err
	}
	if verbose {
		count, bytes, _ := CacheStats(cacheDir)
		fmt.Fprintf(os.Stderr, "verbose: cache now has %d entries, %d bytes total\n", count, bytes)
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
) error {
	javaPath, err := exec.LookPath("java")
	if err != nil {
		return fmt.Errorf("java not found in PATH: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(freshOutPath), 0o755); err != nil {
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
	if verbose {
		fmt.Fprintf(os.Stderr, "verbose: Running krit-types (cached): %s %s\n", javaPath, strings.Join(args, " "))
	}

	timeout := invokeTimeout()
	grace := invokeGraceExit()
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	_, err = runOracleProcess(ctx, javaPath, args, freshOutPath, timeout, grace, verbose)
	return err
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
) (*OracleData, *CacheDepsFile, bool, error) {
	fallback := func(reason string) (*OracleData, *CacheDepsFile, bool, error) {
		if verbose {
			fmt.Fprintf(os.Stderr, "verbose: daemon cache path falling back to one-shot: %s\n", reason)
		}
		if err := runKritTypesCached(jarPath, sourceDirs, missListPath, missFreshPath, missDepsPath, verbose); err != nil {
			return nil, nil, false, err
		}
		fresh, err := readOracleJSON(missFreshPath)
		if err != nil {
			return nil, nil, false, fmt.Errorf("read fresh oracle: %w", err)
		}
		// Same swallowed-error policy as the pre-daemon code — missing
		// or malformed cache-deps is non-fatal, we just skip cache writes.
		deps, _ := LoadCacheDeps(missDepsPath)
		return fresh, deps, false, nil
	}

	// Opt-out knob: KRIT_DAEMON_CACHE=off forces the one-shot path
	// for diagnostics and baseline reproduction. Any other value
	// (including unset) takes the daemon path.
	if strings.EqualFold(os.Getenv("KRIT_DAEMON_CACHE"), "off") {
		return fallback("KRIT_DAEMON_CACHE=off")
	}

	d, err := ConnectOrStartDaemon(jarPath, sourceDirs, nil, verbose)
	if err != nil {
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
		return fallback("daemon sourceDirs mismatch")
	}

	fresh, deps, err := d.AnalyzeWithDeps(misses)
	if err != nil {
		return fallback(fmt.Sprintf("AnalyzeWithDeps: %v", err))
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
