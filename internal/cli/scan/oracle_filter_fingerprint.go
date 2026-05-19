package scan

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"

	"github.com/kaeawc/krit/internal/oracle"
	"github.com/kaeawc/krit/internal/rules"
	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
)

// oracleFingerprintReport is the JSON payload emitted by
// --oracle-filter-fingerprint. Fields are chosen to be stable across
// checkouts: Fingerprint is hashed over paths relative to the first
// scan path (via oracle.StableFingerprint), not absolute paths.
type oracleFingerprintReport struct {
	RuleSet     string   `json:"ruleSet"`
	TotalFiles  int      `json:"totalFiles"`
	MarkedFiles int      `json:"markedFiles"`
	AllFiles    bool     `json:"allFiles"`
	Fingerprint string   `json:"fingerprint"`
	OracleRules []string `json:"oracleRules"`
	Root        string   `json:"root"`
}

// runOracleFilterFingerprint computes the oracle filter input-set
// fingerprint for the given corpus and prints it as JSON. Returns a
// process exit code. The computation does NOT invoke krit-types — it
// runs only the byte-substring pre-filter, so CI can diff fingerprints
// without a JVM on the runner.
func RunOracleFilterFingerprint(paths []string, files []string, activeRules []*api.Rule, allRules bool) int {
	return WriteOracleFilterFingerprint(os.Stdout, os.Stderr, paths, files, activeRules, allRules)
}

// WriteOracleFilterFingerprint is the no-direct-stderr/stdout variant
// of RunOracleFilterFingerprint. Emits the JSON report to stdoutW and
// any error messages to stderrW; returns the process exit code.
// Pulled out so the daemon's oracle-filter-fingerprint verb can
// capture both streams without writing to the daemon's own stdio.
func WriteOracleFilterFingerprint(stdoutW, stderrW io.Writer, paths []string, files []string, activeRules []*api.Rule, allRules bool) int {
	if len(paths) == 0 {
		fmt.Fprintln(stderrW, "error: --oracle-filter-fingerprint requires at least one path")
		return 2
	}
	root, err := filepath.Abs(paths[0])
	if err != nil {
		fmt.Fprintf(stderrW, "error: resolve root: %v\n", err)
		return 2
	}

	// Fingerprint command runs outside the scan pipeline; the depth flag
	// is not in scope here. Project filters at the default (non-thorough)
	// view so baselines stay stable across depths.
	filterRules := rules.BuildOracleFilterRulesV2(activeRules, false)
	oracleRuleNames := make([]string, 0, len(filterRules))
	for _, r := range filterRules {
		oracleRuleNames = append(oracleRuleNames, r.Name)
	}
	sort.Strings(oracleRuleNames)

	lightFiles := loadRawFiles(files)
	summary := oracle.CollectOracleFiles(filterRules, lightFiles)

	ruleSet := "default"
	if allRules {
		ruleSet = "all-rules"
	}

	report := oracleFingerprintReport{
		RuleSet:     ruleSet,
		TotalFiles:  summary.TotalFiles,
		MarkedFiles: summary.MarkedFiles,
		AllFiles:    summary.AllFiles,
		Fingerprint: oracle.StableFingerprint(summary.Paths, root),
		OracleRules: oracleRuleNames,
		Root:        filepath.ToSlash(filepath.Base(root)),
	}

	enc := json.NewEncoder(stdoutW)
	enc.SetIndent("", "  ")
	if err := enc.Encode(report); err != nil {
		fmt.Fprintf(stderrW, "error: encode: %v\n", err)
		return 2
	}
	return 0
}

// loadRawFiles mirrors pipeline.loadFilesForOracleFilter: read each
// path's raw bytes and wrap in *scanner.File. Unreadable files are
// silently dropped (matching the pipeline path's behavior).
func loadRawFiles(paths []string) []*scanner.File {
	out := make([]*scanner.File, 0, len(paths))
	for _, p := range paths {
		content, err := os.ReadFile(p)
		if err != nil {
			continue
		}
		out = append(out, &scanner.File{Path: p, Content: content})
	}
	return out
}
