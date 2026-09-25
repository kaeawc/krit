package firchecks

// pass.go — the --fir pass: pick the files, run the checkers in the
// project's compile context, and apply the FIR-authoritative verdict.

import (
	"context"
	"fmt"
	"io"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/kaeawc/krit/internal/config"
	"github.com/kaeawc/krit/internal/oracle"
	"github.com/kaeawc/krit/internal/perf"
	"github.com/kaeawc/krit/internal/rules"
	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
)

// PassOptions groups every input RunPass needs.
type PassOptions struct {
	Enabled bool
	// Checker runs the subprocess or daemon invocation. Production callers
	// pass *ProductionFirChecker; tests inject FakeFirChecker. When nil the
	// pass is a no-op even if Enabled is true.
	Checker     FirChecker
	Verbose     bool
	ActiveRules []*api.Rule
	// Config supplies each active rule's options, sent to krit-fir as
	// ruleConfigs so FirRule.config() sees the same options as Go rules.
	Config *config.Config
	// ParsedFiles are the scan's parsed Kotlin files. On warm runs they can
	// be a subset of KotlinPaths (only cache misses are parsed) or path-only
	// stubs; their suppression filters are reused when present.
	ParsedFiles []*scanner.File
	// KotlinPaths is every Kotlin path the scan collected. The checker runs
	// on all of them so a warm run reaches the same verdict as a cold one.
	KotlinPaths []string
	// IncludeGenerated mirrors --include-generated: without it, paths under
	// */generated/* are not checked (the parse phase drops them too).
	IncludeGenerated bool
	// SourceDirs and Classpath are the compile context: the JVM-scoped
	// source roots (oracle.FindSourceDirs) and the configured classpath.
	SourceDirs []string
	Classpath  []string
	Tracker    perf.Tracker
	VerboseOut io.Writer
	// Thorough mirrors --depth=thorough; forwarded to ActiveFirRules.
	Thorough bool
}

// RunPass invokes the FIR checkers and returns base with the
// FIR-authoritative verdict applied (see ApplyVerdict). No-op when
// opts.Enabled is false or no checker is configured.
//
// A checker error leaves base unchanged: FIR is opt-in, so a daemon failure
// must never break the scan. Verbose mode reports the error, the timing, the
// file gating, and per-rule verdict counts.
func RunPass(opts PassOptions, base []scanner.Finding) []scanner.Finding {
	if !opts.Enabled || opts.Checker == nil {
		return base
	}
	active := ActiveFirRules(activeRuleIDs(opts.ActiveRules), opts.Thorough)
	if len(active.Names) == 0 {
		return base
	}
	start := time.Now()
	targets := passTargets(opts.ParsedFiles, opts.KotlinPaths, opts.IncludeGenerated)
	requested, excluded := partitionJVMFiles(targets.paths)

	tracker := opts.Tracker
	if tracker == nil {
		tracker = perf.New(false)
	}
	sub := tracker.Serial("firCheck")
	result, err := opts.Checker.Check(requested, opts.SourceDirs, opts.Classpath, active.Names,
		firRuleConfigs(opts.Config, opts.ActiveRules), testFilesOf(requested, targets.display))
	sub.End()
	verbose := opts.Verbose && opts.VerboseOut != nil
	if err != nil {
		if verbose {
			fmt.Fprintf(opts.VerboseOut, "verbose: FIR checker error: %v\n", err)
		}
		return base
	}

	merged, stats := ApplyVerdict(VerdictInput{
		Go:          base,
		FIR:         result,
		Requested:   requested,
		Excluded:    excluded,
		DisplayPath: targets.display,
		Suppressed:  newSuppressor(targets.files).suppressed,
	})
	if verbose {
		cache := Stats()
		fmt.Fprintf(opts.VerboseOut,
			"verbose: FIR checker in %v (%d findings, %d files requested, cache hits=%d misses=%d)\n",
			time.Since(start).Round(time.Millisecond), len(result.Findings), len(requested), cache.Hits, cache.Misses)
		writeVerdictSummary(opts.VerboseOut, stats)
	}
	return merged
}

// maxGatedFilesListed caps the per-file gating lines in verbose output.
const maxGatedFilesListed = 10

func writeVerdictSummary(w io.Writer, stats VerdictStats) {
	fmt.Fprintf(w, "verbose: FIR verdict: %d authoritative files, %d gated (compiler error or crash), %d excluded (scripts or not in a JVM source set), %d rule errors (checker threw; Go kept for that rule and file)\n",
		stats.AuthoritativeFiles, len(stats.GatedFiles), stats.ExcludedFiles, len(stats.RuleErrors))
	for i, e := range stats.RuleErrors {
		if i == maxGatedFilesListed {
			fmt.Fprintf(w, "verbose: FIR rule error: ... and %d more\n", len(stats.RuleErrors)-maxGatedFilesListed)
			break
		}
		fmt.Fprintf(w, "verbose: FIR rule error: %s: %s: %s\n", e.Rule, e.File, firstLine(e.Message))
	}
	gated := make([]string, 0, len(stats.GatedFiles))
	for path := range stats.GatedFiles {
		gated = append(gated, path)
	}
	sort.Strings(gated)
	for i, path := range gated {
		if i == maxGatedFilesListed {
			fmt.Fprintf(w, "verbose: FIR gated: ... and %d more\n", len(gated)-maxGatedFilesListed)
			break
		}
		fmt.Fprintf(w, "verbose: FIR gated: %s: %s\n", path, firstLine(stats.GatedFiles[path]))
	}
	ruleIDs := make([]string, 0, len(stats.Rules))
	for id := range stats.Rules {
		ruleIDs = append(ruleIDs, id)
	}
	sort.Strings(ruleIDs)
	for _, id := range ruleIDs {
		r := stats.Rules[id]
		fmt.Fprintf(w, "verbose: FIR verdict %s: confirmed=%d go-dropped=%d fir-added=%d enriched-with-fix=%d suppressed=%d files-gated=%d rule-errors=%d\n",
			id, r.Confirmed, r.GoDropped, r.FirAdded, r.EnrichedWithFix, r.Suppressed, len(stats.GatedFiles), r.RuleErrorFiles)
	}
}

func firstLine(s string) string {
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return s[:i]
	}
	return s
}

// passTargetSet is the checker's file list plus how to map it back.
type passTargetSet struct {
	// paths are absolute, sorted, and unique.
	paths []string
	// display maps an absolute path to the scan's own spelling of it.
	display map[string]string
	// files maps an absolute path to its parsed file, when one was parsed.
	files map[string]*scanner.File
}

// passTargets chooses the files to check: every parsed Kotlin file plus
// every collected Kotlin path, minus */generated/* paths unless
// includeGenerated (parsed files already had that filter applied).
func passTargets(parsed []*scanner.File, kotlinPaths []string, includeGenerated bool) passTargetSet {
	set := passTargetSet{display: map[string]string{}, files: map[string]*scanner.File{}}
	add := func(path string) string {
		abs, err := filepath.Abs(path)
		if err != nil {
			abs = path
		}
		if _, ok := set.display[abs]; !ok {
			set.display[abs] = path
			set.paths = append(set.paths, abs)
		}
		return abs
	}
	for _, f := range parsed {
		if f == nil || f.Path == "" {
			continue
		}
		abs := add(f.Path)
		if len(f.Content) > 0 {
			set.files[abs] = f
		}
	}
	for _, path := range kotlinPaths {
		if path == "" || (!includeGenerated && strings.Contains(filepath.ToSlash(path), "/generated/")) {
			continue
		}
		add(path)
	}
	sort.Strings(set.paths)
	return set
}

// partitionJVMFiles splits files into those the JVM compilation covers and
// the rest, which are never sent to the checker and so keep Go's findings:
// Kotlin scripts (build.gradle.kts and friends compile against APIs the
// module compilation does not have; the oracle never compiles them either,
// and a script error with no location would gate every file) and files in a
// non-JVM Kotlin Multiplatform source set (jsMain, iosMain, ...).
func partitionJVMFiles(files []string) (jvm, excluded []string) {
	for _, path := range files {
		if strings.HasSuffix(path, ".kt") && oracle.InJVMCompilableSourceSet(path) {
			jvm = append(jvm, path)
		} else {
			excluded = append(excluded, path)
		}
	}
	return jvm, excluded
}

// testFilesOf returns the requested paths krit classifies as test sources,
// spelled as requested so krit-fir can match them against its compiled files.
// Each path is classified by the scan's own spelling of it (display), the
// string Go rules pass to scanner.IsTestFile, so a checker skips exactly the
// files the Go rule skips, configured test paths included. Classifying the
// absolute path instead would also match directories above the scan root.
func testFilesOf(requested []string, display map[string]string) []string {
	var out []string
	for _, path := range requested {
		spelling := path
		if d, ok := display[path]; ok {
			spelling = d
		}
		if scanner.IsTestFile(spelling) {
			out = append(out, path)
		}
	}
	return out
}

// suppressor answers VerdictInput.Suppressed from each file's
// SuppressionFilter: the parse phase's filter when the file was parsed, else
// one built from a fresh parse (warm runs parse only cache misses).
type suppressor struct {
	parsed  map[string]*scanner.File
	filters map[string]*scanner.SuppressionFilter
}

func newSuppressor(parsed map[string]*scanner.File) *suppressor {
	return &suppressor{parsed: parsed, filters: map[string]*scanner.SuppressionFilter{}}
}

func (s *suppressor) suppressed(f scanner.Finding) bool {
	filter := s.filterFor(f.File)
	start := -1
	if f.EndByte > f.StartByte {
		start = f.StartByte
	}
	return filter.IsSuppressedAt(f.Rule, f.RuleSet, f.Line, start)
}

func (s *suppressor) filterFor(path string) *scanner.SuppressionFilter {
	abs, err := filepath.Abs(path)
	if err != nil {
		abs = path
	}
	if filter, ok := s.filters[abs]; ok {
		return filter
	}
	var filter *scanner.SuppressionFilter
	switch file := s.parsed[abs]; {
	case file != nil && file.Suppression != nil:
		filter = file.Suppression
	case file != nil && file.FlatTree != nil:
		filter = buildSuppressionFilter(file)
	default:
		if parsed, err := scanner.ParseFile(context.Background(), path); err == nil {
			filter = buildSuppressionFilter(parsed)
		}
	}
	s.filters[abs] = filter
	return filter
}

// buildSuppressionFilter mirrors the parse phase's per-file filter.
func buildSuppressionFilter(file *scanner.File) *scanner.SuppressionFilter {
	return scanner.BuildSuppressionFilter(file, nil, rules.GetAllRuleExcludes(), "").WithRuleAliases(rules.AllSuppressionAliases())
}

// activeRuleIDs flattens a rule slice into its non-nil rule IDs.
func activeRuleIDs(active []*api.Rule) []string {
	out := make([]string, 0, len(active))
	for _, r := range active {
		if r == nil {
			continue
		}
		out = append(out, r.ID)
	}
	return out
}

// firRuleConfigs collects the configured options (rules.<RuleId> minus
// active/excludes) of every active rule. Go has no index of which rules have
// a FIR checker, so it sends options for every active rule, mirroring the
// rule-ID list; rules without options are omitted.
func firRuleConfigs(cfg *config.Config, active []*api.Rule) RuleConfigs {
	var out RuleConfigs
	for _, r := range active {
		if r == nil {
			continue
		}
		opts := cfg.RuleOptions(r.Category, r.ID)
		if len(opts) == 0 {
			continue
		}
		if out == nil {
			out = RuleConfigs{}
		}
		out[r.ID] = opts
	}
	return out
}
