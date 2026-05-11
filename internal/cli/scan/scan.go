package scan

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"

	"github.com/kaeawc/krit/internal/cacheutil"
	"github.com/kaeawc/krit/internal/cli/clishared"
	"github.com/kaeawc/krit/internal/config"
	"github.com/kaeawc/krit/internal/experiment"
	"github.com/kaeawc/krit/internal/javafacts"
	"github.com/kaeawc/krit/internal/librarymodel"
	"github.com/kaeawc/krit/internal/oracle"
	"github.com/kaeawc/krit/internal/perf"
	"github.com/kaeawc/krit/internal/pipeline"
	"github.com/kaeawc/krit/internal/rules"
	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
	"github.com/kaeawc/krit/internal/store"
)

// resolveCrossFileCacheDir returns the directory that scanner.BuildIndexCached
// should use, or "" when caching is disabled. Disabling follows the
// --no-cross-file-cache flag or an empty scan-paths list (nothing to key).
func resolveCrossFileCacheDir(paths []string, disabled bool) string {
	if disabled {
		return ""
	}
	repoDir := oracle.FindRepoDir(paths)
	if repoDir == "" {
		return ""
	}
	return scanner.CrossFileCacheDir(repoDir)
}

// resolveCrossFindingsCacheDir returns the directory backing the
// cross-rule findings cache, or "" when caching is disabled (mirrors
// resolveCrossFileCacheDir).
func resolveCrossFindingsCacheDir(paths []string, disabled bool) string {
	if disabled {
		return ""
	}
	repoDir := oracle.FindRepoDir(paths)
	if repoDir == "" {
		return ""
	}
	return scanner.CrossFindingsCacheDir(repoDir)
}

func detectConfigForScanArgs(args []string) string {
	if len(args) == 0 {
		return ""
	}
	root := args[0]
	if root == "" {
		return ""
	}
	info, err := os.Stat(root)
	if err != nil {
		return ""
	}
	dir := root
	if !info.IsDir() {
		dir = filepath.Dir(root)
	}
	return clishared.FindConfigInDir(dir)
}

// resolvedStore returns a *store.FileStore for the given --store-dir flag
// pointer, or nil when no store directory is configured and the default
// .krit/store does not yet exist.
func resolvedStore(storeDirFlag *string) *store.FileStore {
	if storeDirFlag == nil {
		return nil
	}
	dir := *storeDirFlag
	if dir == "" {
		if _, err := os.Stat(".krit/store"); err != nil {
			return nil
		}
		dir = ".krit/store"
	}
	return store.New(dir)
}

func runJavaSemanticFacts(ctx context.Context, scanPaths []string, javaFiles []*scanner.File, facts *librarymodel.Facts, tracker perf.Tracker) (*javafacts.Facts, string, error) {
	if len(javaFiles) == 0 {
		return nil, "", nil
	}
	helperClasspath, cleanup, warning, err := compileJavaFactsHelper(scanPaths)
	if cleanup != nil {
		defer cleanup()
	}
	if warning != "" || err != nil {
		return nil, warning, err
	}
	opts := javafacts.DefaultOptions()
	opts.Classpath = javaSemanticClasspath(facts, javaFiles)
	return javafacts.Invoke(ctx, helperClasspath, javaFilePaths(javaFiles), opts, tracker)
}

func compileJavaFactsHelper(scanPaths []string) (classpath string, cleanup func(), warning string, err error) {
	javac, lookupErr := exec.LookPath("javac")
	if lookupErr != nil {
		// Surfaces the lookup failure as a warning string, not an err
		// — javac being absent is non-fatal: the caller falls back to
		// pure-AST analysis. Errors are reserved for genuine helper-
		// build failures (mkdir/javac compile) below.
		return "", nil, javafacts.UnavailableWarning(fmt.Errorf("javac not found")), nil //nolint:nilerr // see comment: lookupErr is intentionally surfaced as warning, not err
	}
	repoDir := oracle.FindRepoDir(scanPaths)
	if repoDir == "" {
		repoDir = "."
	}
	helper := javaFactsHelperSourcePath(repoDir)
	if helper == "" {
		if cwd, cwdErr := os.Getwd(); cwdErr == nil {
			helper = javaFactsHelperSourcePath(cwd)
		}
	}
	if helper == "" {
		helper = filepath.Join(repoDir, "tools", "krit-java-facts", "src", "main", "java", "dev", "krit", "javafacts", "Main.java")
	}
	if _, statErr := os.Stat(helper); statErr != nil {
		// Same non-fatal warning pattern as the javac lookup above.
		return "", nil, javafacts.UnavailableWarning(fmt.Errorf("helper source not found at %s", helper)), nil //nolint:nilerr // statErr surfaced as warning, not err
	}
	tmp, mkErr := os.MkdirTemp("", "krit-java-facts-helper-*")
	if mkErr != nil {
		return "", nil, "", mkErr
	}
	cleanup = func() { _ = os.RemoveAll(tmp) }
	if output, compileErr := exec.CommandContext(context.Background(), javac, "-d", tmp, helper).CombinedOutput(); compileErr != nil {
		cleanup()
		return "", nil, javafacts.UnavailableWarning(fmt.Errorf("compile helper: %w: %s", compileErr, string(output))), nil
	}
	return tmp, cleanup, "", nil
}

func javaFactsHelperSourcePath(root string) string {
	if root == "" {
		return ""
	}
	helper := filepath.Join(root, "tools", "krit-java-facts", "src", "main", "java", "dev", "krit", "javafacts", "Main.java")
	if _, err := os.Stat(helper); err == nil {
		return helper
	}
	return ""
}

func javaSemanticClasspath(facts *librarymodel.Facts, files []*scanner.File) string {
	seen := map[string]bool{}
	var entries []string
	add := func(path string) {
		if path == "" {
			return
		}
		clean := filepath.Clean(path)
		if seen[clean] {
			return
		}
		if _, err := os.Stat(clean); err != nil {
			return
		}
		seen[clean] = true
		entries = append(entries, clean)
	}
	if facts != nil {
		for _, path := range facts.Profile.Java.SourceRootsForScan(true, true, true) {
			add(path)
		}
		for _, path := range facts.Profile.Java.JavacClasspathCandidates() {
			add(path)
		}
	}
	for _, file := range files {
		if file != nil {
			add(filepath.Dir(file.Path))
		}
	}
	return strings.Join(entries, string(os.PathListSeparator))
}

func javaFilePaths(files []*scanner.File) []string {
	out := make([]string, 0, len(files))
	for _, file := range files {
		if file != nil {
			out = append(out, file.Path)
		}
	}
	return out
}

// resolveParseCacheCap returns the parse cache size cap in bytes.
// Precedence: CLI flag > krit.yml parseCache.maxSizeMB > default
// (cacheutil.DefaultParseCacheCapBytes). A negative flag value disables
// the cap (unlimited cache growth — discouraged but supported for
// benchmarking).
func resolveParseCacheCap(flagMB int, cfg *config.Config) int64 {
	if flagMB < 0 {
		return -1
	}
	if flagMB > 0 {
		return int64(flagMB) * 1024 * 1024
	}
	if cfg != nil {
		if mb := cfg.GetTopLevelInt("parseCache", "maxSizeMB", 0); mb != 0 {
			if mb < 0 {
				return -1
			}
			return int64(mb) * 1024 * 1024
		}
	}
	return cacheutil.DefaultParseCacheCapBytes
}

func newParseCacheAsyncWriter(workers int) *cacheutil.AsyncWriter {
	if workers <= 0 {
		workers = runtime.NumCPU()
	}
	if workers > 4 {
		workers = 4
	}
	if workers < 1 {
		workers = 1
	}
	return cacheutil.NewAsyncWriter(workers, workers*256)
}

// BaselineAuditVerb is set by the verb dispatcher in cmd/krit/main.go
// when the user invokes `krit baseline-audit`. Run treats this exactly
// like the --baseline-audit flag.
var BaselineAuditVerb bool

// Run executes the scan default verb. It reads os.Args directly
// (post-dispatch, so verb tokens have already been stripped). Returns
// the process exit code; some early-exit flag handlers still call
// os.Exit directly.
func Run() int {
	baselineAuditVerb := BaselineAuditVerb

	f := registerScanFlags(flag.CommandLine)

	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: krit [flags] [paths...]\n\nFlags:\n")
		flag.PrintDefaults()
		fmt.Fprintf(os.Stderr, "\nSubcommands:\n")
		fmt.Fprintf(os.Stderr, "  krit baseline-audit [flags] [paths...]\n")
		fmt.Fprintf(os.Stderr, "  krit abi-hash <:module|path/to/File.kt> [--json]\n")
		fmt.Fprintf(os.Stderr, "  krit impact <fqn>... | --from-file PATH | --since GITREF [--json]\n")
		fmt.Fprintf(os.Stderr, "  krit metrics log|query [flags]\n")
		fmt.Fprintf(os.Stderr, "  krit score [--format json|number] [paths...]\n")
		fmt.Fprintf(os.Stderr, "  krit scorecard [--format markdown] [paths...]\n")
		fmt.Fprintf(os.Stderr, "  krit used-symbols <:module|path/to/File.kt> [--json]\n")
		fmt.Fprintf(os.Stderr, "  krit harvest SOURCE:LINE --rule RuleName --out fixture.kt\n")
		fmt.Fprintf(os.Stderr, "  krit rename [flags] <from-fqn> <to-fqn> [paths...]\n")
		fmt.Fprintf(os.Stderr, "  krit dead-code --project [--json] [--root FQN]... [paths...]\n")
		fmt.Fprintf(os.Stderr, "\nSARIF upload example:\n")
		fmt.Fprintf(os.Stderr, "  krit --report=sarif -o results.sarif src/\n")
		fmt.Fprintf(os.Stderr, "  # Then upload to GitHub Code Scanning:\n")
		fmt.Fprintf(os.Stderr, "  # gh api repos/{owner}/{repo}/code-scanning/sarifs -f sarif=@results.sarif\n")
	}

	flag.Parse()
	if baselineAuditVerb {
		*f.BaselineAudit = true
	}
	if *f.PerfRules {
		*f.Perf = true
	}

	// Scaffold mode: -new-experiment short-circuits the normal scan path.
	runNewExperimentScaffoldFlag(NewExperimentOpts{
		Name:        *f.NewExperiment,
		Description: *f.NewExperimentDescription,
		Intent:      *f.NewExperimentIntent,
		TargetRules: experiment.ParseCSV(*f.NewExperimentTargetRules),
		WireFile:    *f.NewExperimentWireFile,
	})

	r, code, ok := newRunner(f)
	if !ok {
		return code
	}
	defer r.close()

	if code, err := r.collectFiles(); err != nil {
		return code
	}
	if handled, code := r.filterRules(); handled {
		return code
	}
	r.bootstrapResolver()
	if code, err := r.runOracleIndex(); err != nil {
		return code
	}
	r.printVerboseBanner()
	r.setupAndroidProviders()
	r.setupParseCaches()

	if code, err := r.runProjectAnalysis(); err != nil {
		return code
	}
	r.firCheckAndCollect()

	if handled, code := r.applyBaselinesAndDiff(); handled {
		return code
	}
	if handled, code := r.runFixup(); handled {
		return code
	}

	// close() is idempotent, so invoking it here flushes once before the
	// output writer opens; the runner's defer takes care of any later attempts.
	r.flushCaches()

	if code, err := r.openOutputWriter(); err != nil {
		return code
	}
	if r.w != nil && r.w != os.Stdout {
		defer r.w.Close()
	}

	// Flush again after openOutputWriter. flushCaches is idempotent.
	r.flushCaches()

	r.recordTotalTimingAndStopProfiles()
	return r.outputPhase()
}

// runLegacy remains as a no-op compile-time reference.
//
//nolint:unused

func FilterFixesByLevelColumns(columns *scanner.FindingColumns, registry []*api.Rule, maxLevel rules.FixLevel) (fixableCount, strippedByLevel int) {
	if columns == nil {
		return 0, 0
	}

	ruleLevels := make(map[string]rules.FixLevel, len(registry))
	for _, r := range registry {
		if r == nil {
			continue
		}
		if lvl, ok := rules.GetV2FixLevel(r); ok {
			ruleLevels[r.ID] = lvl
		}
	}

	strippedByLevel = columns.StripTextFixes(func(row int) bool {
		return ruleLevels[columns.RuleAt(row)] > maxLevel
	})
	return columns.CountTextFixes(), strippedByLevel
}

func reportRuleExecutionRanking(w *os.File, stats []rules.RuleExecutionStat, limit int) {
	if len(stats) == 0 {
		return
	}
	if limit <= 0 || limit > len(stats) {
		limit = len(stats)
	}
	fmt.Fprintln(w, "=== rule execution ranking ===")
	fmt.Fprintln(w, "rank  rule                              family    time(ms)  share   calls     avg(ns)")
	for i := 0; i < limit; i++ {
		stat := stats[i]
		fmt.Fprintf(w, "%4d  %-32s  %-8s  %8.3f  %5.1f%%  %7d  %8d\n",
			i+1, stat.Rule, stat.Family, stat.DurationMs, stat.SharePct, stat.Invocations, stat.AvgNs)
	}
	fmt.Fprintln(w, "=== end rule execution ranking ===")
}

// fileTiming is an alias for pipeline.FileTiming so profile-dispatch
// reporting can stay in main.go while the phase owns the capture path.
type fileTiming = pipeline.FileTiming

// reportDispatchProfile prints a distribution analysis of per-file dispatch
// timings. Used to diagnose parallelism-collapse on large corpora.
//
// This function is intentionally verbose and noisy — it's debug output behind
// the -profile-dispatch flag. Shipped on oracle-fixes-integration branch only.
func reportDispatchProfile(timings []fileTiming, workers int, wall time.Duration) {
	if len(timings) == 0 {
		return
	}
	n := len(timings)

	// Totals
	var sumRun, sumQueue, sumLock, sumAgg, sumTotal int64
	var maxRun, maxTotal int64
	for _, t := range timings {
		sumRun += t.RunMs
		sumQueue += t.QueueMs
		sumLock += t.LockMs
		sumAgg += t.AggMs
		sumTotal += t.TotalMs
		if t.RunMs > maxRun {
			maxRun = t.RunMs
		}
		if t.TotalMs > maxTotal {
			maxTotal = t.TotalMs
		}
	}

	// Duration distribution (percentiles of runMs)
	runs := make([]int64, n)
	for i, t := range timings {
		runs[i] = t.RunMs
	}
	sort.Slice(runs, func(i, j int) bool { return runs[i] < runs[j] })
	pct := func(p float64) int64 {
		if n == 0 {
			return 0
		}
		idx := int(float64(n-1) * p)
		if idx < 0 {
			idx = 0
		}
		if idx >= n {
			idx = n - 1
		}
		return runs[idx]
	}

	// Top 20 slowest files
	sorted := make([]int, n)
	for i := range sorted {
		sorted[i] = i
	}
	sort.Slice(sorted, func(i, j int) bool {
		return timings[sorted[i]].RunMs > timings[sorted[j]].RunMs
	})

	fmt.Fprintln(os.Stderr, "")
	fmt.Fprintln(os.Stderr, "=== dispatch profile ===")
	fmt.Fprintf(os.Stderr, "files: %d   workers: %d   wall: %dms\n", n, workers, wall.Milliseconds())
	fmt.Fprintf(os.Stderr, "cumulative runMs: %d   cumulative queueMs: %d   cumulative lockMs: %d   cumulative aggMs: %d   cumulative totalMs: %d\n",
		sumRun, sumQueue, sumLock, sumAgg, sumTotal)
	fmt.Fprintf(os.Stderr, "parallelism (cumRun/wall): %.2fx   ceiling %d\n", float64(sumRun)/float64(wall.Milliseconds()), workers)
	fmt.Fprintln(os.Stderr, "")
	fmt.Fprintln(os.Stderr, "runMs distribution:")
	fmt.Fprintf(os.Stderr, "  p50=%dms  p75=%dms  p90=%dms  p95=%dms  p99=%dms  p99.9=%dms  max=%dms\n",
		pct(0.50), pct(0.75), pct(0.90), pct(0.95), pct(0.99), pct(0.999), pct(1.0))
	fmt.Fprintf(os.Stderr, "  mean=%dms\n", sumRun/int64(n))
	fmt.Fprintln(os.Stderr, "")
	fmt.Fprintf(os.Stderr, "per-file lock-wait (p50/p95/max): %dms / %dms / %dms   (cum %dms)\n",
		percentileInt(lockWaits(timings), 0.50),
		percentileInt(lockWaits(timings), 0.95),
		percentileInt(lockWaits(timings), 1.0),
		sumLock)
	fmt.Fprintf(os.Stderr, "per-file agg-hold  (p50/p95/max): %dms / %dms / %dms   (cum %dms)\n",
		percentileInt(aggHolds(timings), 0.50),
		percentileInt(aggHolds(timings), 0.95),
		percentileInt(aggHolds(timings), 1.0),
		sumAgg)
	fmt.Fprintln(os.Stderr, "")
	fmt.Fprintln(os.Stderr, "top 20 slowest files by runMs:")
	limit := 20
	if n < limit {
		limit = n
	}
	for i := 0; i < limit; i++ {
		t := timings[sorted[i]]
		fmt.Fprintf(os.Stderr, "  %6dms  %7dkb  %4d findings  %s\n",
			t.RunMs, t.Size/1024, t.Findings, t.Path)
	}
	// Sum of top 20 vs total: what fraction of work comes from the tail?
	var topSum int64
	for i := 0; i < limit; i++ {
		topSum += timings[sorted[i]].RunMs
	}
	fmt.Fprintf(os.Stderr, "  top %d account for %d%% of cumulative runMs\n",
		limit, int(topSum*100/sumRun))
	// How long would dispatch take if we had perfect scheduling (largest-first)?
	// Lower bound = max(cumRun/workers, maxFile)
	perfectWall := sumRun / int64(workers)
	if maxRun > perfectWall {
		perfectWall = maxRun
	}
	fmt.Fprintf(os.Stderr, "lower bound (perfect scheduling): wall = max(cumRun/workers, maxFile) = max(%d, %d) = %dms\n",
		sumRun/int64(workers), maxRun, perfectWall)
	fmt.Fprintln(os.Stderr, "=== end dispatch profile ===")
	fmt.Fprintln(os.Stderr, "")
}

// lockWaits extracts LockMs values for percentile computation.
func lockWaits(timings []fileTiming) []int64 {
	out := make([]int64, len(timings))
	for i, t := range timings {
		out[i] = t.LockMs
	}
	return out
}

// aggHolds extracts AggMs values for percentile computation.
func aggHolds(timings []fileTiming) []int64 {
	out := make([]int64, len(timings))
	for i, t := range timings {
		out[i] = t.AggMs
	}
	return out
}

// percentileInt returns the p-th percentile of a slice of ints (not in-place).
func percentileInt(xs []int64, p float64) int64 {
	if len(xs) == 0 {
		return 0
	}
	sorted := make([]int64, len(xs))
	copy(sorted, xs)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i] < sorted[j] })
	idx := int(float64(len(sorted)-1) * p)
	if idx < 0 {
		idx = 0
	}
	if idx >= len(sorted) {
		idx = len(sorted) - 1
	}
	return sorted[idx]
}

func phaseWorkerCount(phase string, maxWorkers, workItems int) int {
	if maxWorkers < 1 {
		maxWorkers = 1
	}
	if workItems < 1 {
		return 1
	}

	workers := maxWorkers
	if workItems < workers {
		workers = workItems
	}

	var phaseCap int
	switch phase {
	case "moduleAwareAnalysis":
		phaseCap = 8
	case "ruleExecution", "parse", "typeIndex", "crossFileAnalysis":
		phaseCap = 16
	default:
		phaseCap = workers
	}
	if phaseCap < 1 {
		phaseCap = 1
	}
	if workers > phaseCap {
		workers = phaseCap
	}
	return workers
}

func countActiveV2(registry []*api.Rule) int {
	count := 0
	for _, r := range registry {
		if rules.IsDefaultActive(r.ID) {
			count++
		}
	}
	return count
}

func filterGeneratedPathStrings(paths []string) []string {
	filtered := paths[:0]
	for _, p := range paths {
		if strings.Contains(filepath.ToSlash(p), "/generated/") {
			continue
		}
		filtered = append(filtered, p)
	}
	return filtered
}

// getChangedLines runs git diff and returns a map of absolute file path → set of changed line numbers.
// getChangedFiles returns the set of absolute file paths that have changed
// since the given git ref. Uses git diff --name-only for robust file discovery.
func getChangedFiles(ref string, scanPaths []string) (map[string]bool, error) {
	// Get changed files: staged + unstaged modifications since ref
	args := []string{"diff", "--name-only", "--diff-filter=ACMR", ref, "--"}
	args = append(args, scanPaths...)
	cmd := exec.CommandContext(context.Background(), "git", args...)
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("git diff %s: %w", ref, err)
	}

	result := make(map[string]bool)
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if line == "" {
			continue
		}
		absPath, _ := filepath.Abs(line)
		if absPath != "" {
			result[absPath] = true
		}
	}
	return result, nil
}
