package scan

import (
	"fmt"
	"os"
	"strings"

	"github.com/kaeawc/krit/internal/oracle"
	"github.com/kaeawc/krit/internal/perf"
	"github.com/kaeawc/krit/internal/scanner"
)

// computeStaleOraclePaths consults the prior findings-bundle manifest to
// decide which .kt paths are KAA-stale relative to the cached
// types.json. Returns nil when no prior manifest exists (true cold
// scan; the freshness gate's lazy-load fast path is safe) or when no
// types.json exists (cold path is going to recompute everything
// anyway). Returns a non-empty slice when one or more files differ in
// stat (size + mtime); the oracle layer will treat those as forced
// misses and route them through a partial JVM reanalyze.
//
// Only the daemon path persists a bundle manifest today, so one-shot
// CLI runs typically return nil here and continue to rely on the
// lazy-load short-circuit. That preserves the historical CLI warm
// latency at the cost of correctness on stale caches — callers that
// need correctness should pass --no-cache-oracle (forces a full
// reanalyze) or use the daemon (which produces a manifest the
// freshness gate can compare against).
func computeStaleOraclePaths(scanPaths []string, kotlinFilePaths []string, tracker perf.Tracker, verbose bool) []string {
	if len(kotlinFilePaths) == 0 {
		return nil
	}
	repoDir := oracle.FindRepoDir(scanPaths)
	if repoDir == "" {
		return nil
	}
	cachedTypes := oracle.CachePath(scanPaths)
	if cachedTypes == "" {
		return nil
	}
	if _, err := os.Stat(cachedTypes); err != nil {
		return nil
	}
	manifestKey := scanner.FindingsBundleManifestKey(repoDir, scanPaths)
	if manifestKey == "" {
		return nil
	}
	prior, ok := scanner.LoadFindingsBundleManifest(repoDir, manifestKey)
	if !ok {
		if verbose {
			fmt.Fprintln(os.Stderr, "verbose: oracle freshness gate: no prior bundle manifest — falling back to lazy load")
		}
		perf.AddEntryDetails(tracker, "freshnessGateNoManifest", 0, nil, nil)
		return nil
	}
	stale := scanner.StaleOracleCandidates(kotlinFilePaths, prior, scanner.StatFile)
	if len(stale) == 0 {
		if verbose {
			fmt.Fprintln(os.Stderr, "verbose: oracle freshness gate: manifest is in sync with current file stats")
		}
		return nil
	}
	if verbose {
		preview := stale
		if len(preview) > 5 {
			preview = preview[:5]
		}
		fmt.Fprintf(os.Stderr, "verbose: oracle freshness gate: %d stale path(s); preview=[%s]\n",
			len(stale), strings.Join(preview, ", "))
	}
	perf.AddEntryDetails(tracker, "freshnessGateStaleCandidates", 0, map[string]int64{
		"stale":     int64(len(stale)),
		"checked":   int64(len(kotlinFilePaths)),
		"priorSize": int64(len(prior.FileStats)),
	}, nil)
	return stale
}
