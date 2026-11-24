package firchecks

// pass.go — the FIR pass: pick the files, run the checkers in the
// project's compile context, and apply the FIR-authoritative verdict.

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/kaeawc/krit/internal/android"
	"github.com/kaeawc/krit/internal/config"
	"github.com/kaeawc/krit/internal/fileignore"
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
	// GeneratedSourceDirs are compilation inputs, never checker targets.
	GeneratedSourceDirs []string
	// SourceDirs and Classpath are the compile context: the JVM-scoped
	// source roots (oracle.FindSourceDirs) and the configured classpath.
	SourceDirs []string
	Classpath  []string
	Tracker    perf.Tracker
	// Broker, when set, is the rendezvous with the oracle's compile (#810);
	// the pass records how it ended in the "firCheckBroker" perf entry.
	Broker     *Broker
	VerboseOut io.Writer
	// Thorough mirrors --depth=thorough; forwarded to ActiveFirRules.
	Thorough bool
	// OnDegraded, when set, is called once if the checker fails mid-scan and
	// the results fall back to Go-only findings.
	OnDegraded func(err error)
	// Isolator, when set, rechecks the projects whose files the check gated on
	// a duplicate declaration with each source set compiled on its own: a
	// merged compile of main, test and androidTest reports a class declared in
	// two of them as a redeclaration that the real build never sees.
	Isolator SourceSetIsolator
}

// PendingPass owns an in-flight check. Check is not interruptible through the
// FirChecker interface; Cancel releases callers immediately, and the buffered
// result lets the checker goroutine exit as soon as Check returns.
type PendingPass struct {
	opts                PassOptions
	targets             passTargetSet
	requested, excluded []string
	buildLogicExcluded  int
	rules               []string
	done                chan checkOutcome
	ctx                 context.Context
	cancel              context.CancelFunc
}

type checkOutcome struct {
	result                *Result
	err                   error
	duration              time.Duration
	incrementalDiagnostic string
	isolation             isolationOutcome
}

// Cancel abandons a pending verdict without waiting for a JVM check.
func (p *PendingPass) Cancel() {
	if p != nil && p.cancel != nil {
		p.cancel()
	}
}

// passPlan is the check request a pass sends: the files it checks and the
// rules, options, and file facts that go with them.
type passPlan struct {
	targets             passTargetSet
	requested, excluded []string
	buildLogicExcluded  int
	rules               []string
	configs             RuleConfigs
	facts               FileFacts
}

// planPass computes the request a pass over opts sends, with every input
// copied. False when the pass would not run the checker.
func planPass(opts PassOptions) (passPlan, bool) {
	if !opts.Enabled || opts.Checker == nil {
		return passPlan{}, false
	}
	active := ActiveFirRules(activeRuleIDs(opts.ActiveRules), opts.Thorough)
	if len(active.Names) == 0 {
		return passPlan{}, false
	}
	plan := passPlan{
		targets: passTargets(opts.ParsedFiles, opts.KotlinPaths, opts.IncludeGenerated),
		rules:   slices.Clone(active.Names),
		configs: firRuleConfigs(opts.Config, opts.ActiveRules),
	}
	plan.targets.excludeRoots(opts.GeneratedSourceDirs)
	plan.requested, plan.excluded = partitionJVMFiles(plan.targets.paths)
	for _, path := range plan.excluded {
		if oracle.IsBuildLogicPath(path) {
			plan.buildLogicExcluded++
		}
	}
	for rule, values := range plan.configs {
		copyValues := make(map[string]any, len(values))
		for key, value := range values {
			copyValues[key] = cloneRuleConfigValue(value)
		}
		plan.configs[rule] = copyValues
	}
	plan.facts = fileFactsOf(plan.requested, plan.targets.display)
	return plan, true
}

// StartPass starts the checker after snapshotting every input it reads.
func StartPass(ctx context.Context, opts PassOptions) *PendingPass {
	p := &PendingPass{opts: opts}
	plan, ok := planPass(opts)
	if !ok {
		return p
	}
	p.targets, p.requested, p.excluded, p.buildLogicExcluded = plan.targets, plan.requested, plan.excluded, plan.buildLogicExcluded
	p.rules = plan.rules
	requested := slices.Clone(p.requested)
	sourceDirs, classpath, ruleNames := slices.Clone(opts.SourceDirs), slices.Clone(opts.Classpath), plan.rules
	configs, facts := plan.configs, plan.facts
	p.opts.GeneratedSourceDirs = slices.Clone(opts.GeneratedSourceDirs)
	p.ctx, p.cancel = context.WithCancel(ctx)
	p.done = make(chan checkOutcome, 1)
	tracker := opts.Tracker
	if tracker == nil {
		tracker = perf.New(false)
	}
	go func() {
		sub := tracker.Serial("firCheck")
		checkStart := time.Now()
		result, err := opts.Checker.Check(requested, sourceDirs, classpath, ruleNames, configs, facts)
		duration := time.Since(checkStart)
		var isolation isolationOutcome
		if err == nil && result != nil && opts.Isolator != nil {
			isolation = isolateSourceSets(p.ctx, opts, requested, sourceDirs, classpath, ruleNames, configs, facts, result)
			if isolation.gated > 0 {
				perf.AddEntryDetails(sub, "firSourceSetIsolation", isolation.duration, map[string]int64{
					"gated":      int64(isolation.gated),
					"rechecked":  int64(isolation.rechecked),
					"replaced":   int64(isolation.replaced),
					"stillGated": int64(isolation.stillGated),
				}, nil)
			}
		}
		status := "ok"
		if err != nil {
			status = "error"
		}
		incrementalDiagnostic := ""
		if err == nil {
			noCache := false
			if checker, ok := opts.Checker.(*ProductionFirChecker); ok {
				noCache = checker.NoCache
			}
			incrementalDiagnostic = incrementalFIRDiagnostic(result, noCache, os.Getenv("KRIT_FIR_INCREMENTAL"))
		}
		if opts.Broker != nil {
			// Recorded once the pass has its answer, so "shared" means the
			// pass really used the oracle's compile.
			details := map[string]string{"outcome": opts.Broker.Outcome()}
			if result != nil && result.Incremental != nil {
				// How krit-fir's incremental planner served the check: on
				// the shared compile or on the pass's own.
				details["incremental"] = result.Incremental.String()
			}
			perf.AddEntryDetails(sub, "firCheckBroker", 0, nil, details)
		}
		details := map[string]string{"status": status}
		if incrementalDiagnostic != "" {
			details["incremental"] = incrementalDiagnostic
		}
		perf.AddEntryDetails(sub, "firCheckOutcome", 0, map[string]int64{
			"files": int64(len(requested)),
			"rules": int64(len(ruleNames)),
		}, details)
		sub.End()
		p.done <- checkOutcome{result: result, err: err, duration: duration, incrementalDiagnostic: incrementalDiagnostic, isolation: isolation}
	}()
	return p
}

func cloneRuleConfigValue(value any) any {
	switch v := value.(type) {
	case map[string]any:
		copyMap := make(map[string]any, len(v))
		for key, item := range v {
			copyMap[key] = cloneRuleConfigValue(item)
		}
		return copyMap
	case []any:
		copySlice := make([]any, len(v))
		for i, item := range v {
			copySlice[i] = cloneRuleConfigValue(item)
		}
		return copySlice
	default:
		return value
	}
}

// RunPass invokes the FIR checkers and returns base with the
// FIR-authoritative verdict applied (see ApplyVerdict). No-op when
// opts.Enabled is false or no checker is configured.
//
// A checker error leaves base unchanged after preflight: runtime checker
// failures retain Go findings. Verbose mode reports the error, the timing, the
// file gating, and per-rule verdict counts.
func RunPass(opts PassOptions, base []scanner.Finding) []scanner.Finding {
	p := StartPass(context.Background(), opts)
	merged, _ := p.Finish(base)
	return merged
}

// Finish waits for the checker and applies its verdict to completed Go findings.
// Checker errors retain those findings, as RunPass always has.
func (p *PendingPass) Finish(base []scanner.Finding) ([]scanner.Finding, error) {
	if p == nil {
		return base, nil
	}
	opts := p.opts
	if !opts.Enabled || opts.Checker == nil {
		return base, nil
	}
	base = excludeGeneratedFindings(base, opts.GeneratedSourceDirs)
	if p.done == nil {
		return base, nil
	}
	var outcome checkOutcome
	select {
	case outcome = <-p.done:
	case <-p.ctx.Done():
		// The scan keeps Go's findings; record that, so a caller that still
		// reports them does not present them as a complete FIR result.
		err := p.ctx.Err()
		if opts.OnDegraded != nil {
			opts.OnDegraded(fmt.Errorf("FIR pass cancelled: %w", err))
		}
		return base, err
	}
	p.Cancel()
	result, err := outcome.result, outcome.err
	verbose := opts.Verbose && opts.VerboseOut != nil
	if err != nil {
		reportDegraded(opts, err)
		if verbose {
			fmt.Fprintf(opts.VerboseOut, "verbose: FIR checker error: %v\n", err)
		}
		return base, nil
	}
	if lost := lostPassError(result, p.requested, p.rules); lost != nil {
		// The check returned, but ApplyVerdict will keep Go's findings for
		// every file. Report it like a checker error, then still apply the
		// (empty) verdict so verbose mode explains the gating.
		reportDegraded(opts, lost)
	}
	result = withoutGoAuthoritativeRules(result, opts.Config)

	merged, stats := ApplyVerdict(VerdictInput{
		Go:          base,
		FIR:         result,
		Requested:   p.requested,
		Excluded:    p.excluded,
		DisplayPath: p.targets.display,
		Suppressed:  newSuppressor(p.targets.files).suppressed,
	})
	if verbose {
		if outcome.incrementalDiagnostic != "" {
			fmt.Fprintf(opts.VerboseOut, "verbose: %s\n", outcome.incrementalDiagnostic)
		}
		if outcome.isolation.gated > 0 {
			writeIsolationOutcome(opts.VerboseOut, outcome.isolation)
		}
		fmt.Fprintf(opts.VerboseOut,
			"verbose: FIR checker in %v (%d findings, %d files requested)\n",
			outcome.duration.Round(time.Millisecond), len(result.Findings), len(p.requested))
		writeVerdictSummary(opts.VerboseOut, stats, p.buildLogicExcluded)
		for _, warning := range result.Warnings {
			fmt.Fprintf(opts.VerboseOut, "verbose: FIR checker warning: %s\n", warning)
		}
	}
	return merged, nil
}

// reportDegraded makes a pass whose result holds only Go findings visible:
// those findings look complete, so the warning is printed even without -v.
func reportDegraded(opts PassOptions, err error) {
	reporter().Warnf("warning: --fir checker failed (%v); results contain Go-only findings\n", err)
	if opts.OnDegraded != nil {
		opts.OnDegraded(err)
	}
}

// withoutGoAuthoritativeRules drops the rules the config keeps Go-authoritative
// (fir.goAuthoritativeRules) from the rules FIR owns.
func withoutGoAuthoritativeRules(result *Result, cfg *config.Config) *Result {
	if cfg == nil || len(cfg.FIR().GoAuthoritativeRules) == 0 {
		return result
	}
	goAuthoritative := make(map[string]bool)
	for _, rule := range cfg.FIR().GoAuthoritativeRules {
		goAuthoritative[rule] = true
	}
	filtered := make([]string, 0, len(result.Rules))
	for _, rule := range result.Rules {
		if !goAuthoritative[rule] {
			filtered = append(filtered, rule)
		}
	}
	copyResult := *result
	copyResult.Rules = filtered
	return &copyResult
}

// lostPassError reports a check that returned without error but left FIR no
// verdict to apply, so ApplyVerdict would keep Go's findings for every file:
// krit-fir advertised none of the requested rules although some have a FIR
// checker, or it crashed on or failed to compile every requested file (a
// compiler exception under memory pressure marks every file of the compile
// crashed). Nil when FIR owns at least one file and rule, when nothing was
// requested, or when no requested rule has a FIR checker (every active rule
// ID is sent; the jar advertises the ones it implements).
func lostPassError(result *Result, requested, rules []string) error {
	if len(requested) == 0 {
		return nil
	}
	known := slices.ContainsFunc(rules, IsKnownFIRRule)
	if result == nil {
		if !known {
			return nil
		}
		return fmt.Errorf("krit-fir returned no result for %d files", len(requested))
	}
	if len(result.Rules) == 0 {
		if !known {
			return nil
		}
		return fmt.Errorf("krit-fir ran none of the requested rules on %d files", len(requested))
	}
	crashed, failed := 0, 0
	firstReason := ""
	for _, path := range requested {
		if msg, ok := result.Crashed[path]; ok {
			crashed++
			if firstReason == "" {
				firstReason = "crashed: " + firstLine(msg)
			}
			continue
		}
		msg, ok := result.ErrorFiles[path]
		if !ok {
			return nil
		}
		failed++
		if firstReason == "" {
			firstReason = firstLine(msg)
		}
	}
	return fmt.Errorf("krit-fir had no usable verdict for any of %d files (%d crashed, %d with compiler errors; first: %s)",
		len(requested), crashed, failed, firstReason)
}

// maxGatedFilesListed caps the per-file gating lines in verbose output.
const maxGatedFilesListed = 10

func writeVerdictSummary(w io.Writer, stats VerdictStats, buildLogicExcluded ...int) {
	generated := 0
	for _, message := range stats.GatedFiles {
		if generatedSymbolError(firstLine(message)) {
			generated++
		}
	}
	buildLogic := 0
	if len(buildLogicExcluded) > 0 {
		buildLogic = buildLogicExcluded[0]
	}
	fmt.Fprintf(w, "verbose: FIR verdict: %d authoritative files, %d gated (compiler error or crash), %d excluded (scripts or not in a JVM source set), %d rule errors (checker threw; Go kept for that rule and file), %d gated (generated sources), %d excluded (build logic)\n",
		stats.AuthoritativeFiles, len(stats.GatedFiles)-generated, stats.ExcludedFiles-buildLogic, len(stats.RuleErrors), generated, buildLogic)
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
		if r.Confirmed == 0 && r.GoDropped == 0 && r.FirAdded == 0 && r.ConfirmOnlyDropped == 0 && r.EnrichedWithFix == 0 && r.Suppressed == 0 && r.RuleErrorFiles == 0 {
			continue // a rule with no verdicts only adds the shared gated-file count
		}
		fmt.Fprintf(w, "verbose: FIR verdict %s: confirmed=%d go-dropped=%d fir-added=%d enriched-with-fix=%d suppressed=%d files-gated=%d confirm-only-dropped=%d rule-errors=%d\n",
			id, r.Confirmed, r.GoDropped, r.FirAdded, r.EnrichedWithFix, r.Suppressed, len(stats.GatedFiles), r.ConfirmOnlyDropped, r.RuleErrorFiles)
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

func (set *passTargetSet) excludeRoots(roots []string) {
	if len(roots) == 0 {
		return
	}
	kept := set.paths[:0]
	for _, path := range set.paths {
		generated := false
		for _, root := range roots {
			if root == "" {
				continue
			}
			abs, err := filepath.Abs(root)
			if err != nil {
				continue
			}
			if isUnderRoot(path, abs) {
				generated = true
				break
			}
		}
		if generated {
			delete(set.display, path)
			delete(set.files, path)
		} else {
			kept = append(kept, path)
		}
	}
	set.paths = kept
}

func excludeGeneratedFindings(findings []scanner.Finding, roots []string) []scanner.Finding {
	if len(roots) == 0 {
		return findings
	}
	var out []scanner.Finding
	for _, finding := range findings {
		path, err := filepath.Abs(finding.File)
		if err != nil {
			path = finding.File
		}
		generated := false
		for _, root := range roots {
			abs, err := filepath.Abs(root)
			if root != "" && err == nil && isUnderRoot(path, abs) {
				generated = true
				break
			}
		}
		if !generated {
			out = append(out, finding)
		}
	}
	return out
}

func isUnderRoot(path, root string) bool {
	return path == root || strings.HasPrefix(path, root+string(filepath.Separator))
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
		// A documentation sample is never compiled, so the checker must not be handed it.
		if f == nil || f.Path == "" || fileignore.DocSamplePath(f.Path) {
			continue
		}
		abs := add(f.Path)
		if len(f.Content) > 0 {
			set.files[abs] = f
		}
	}
	for _, path := range kotlinPaths {
		if path == "" || fileignore.DocSamplePath(path) || (!includeGenerated && strings.Contains(filepath.ToSlash(path), "/generated/")) {
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
		if strings.HasSuffix(path, ".kt") && oracle.InJVMCompilableSourceSet(path) && !oracle.IsBuildLogicPath(path) {
			jvm = append(jvm, path)
		} else {
			excluded = append(excluded, path)
		}
	}
	return jvm, excluded
}

// fileFactsOf returns the facts sent with a check request about the
// requested files: which ones are test sources (testFilesOf) and the scan's
// own spelling of each file whose spelling differs from the requested
// absolute path. The scan spelling is the path string the Go rules test (a
// relative `samples/proj/src/X.kt` for `krit samples/proj`), so a checker
// that applies a Go path heuristic reads it through FirRule.scanPath instead
// of guessing from the absolute path. The facts also carry each file's
// minSdk / targetSdk, resolved from the scan spelling with
// android.ResolveSDKLevels, the lookup the Go rules run on file.Path, so a
// checker reading FirRule.minSdkFor / targetSdkFor sees the levels the Go
// rule sees.
func fileFactsOf(requested []string, display map[string]string) FileFacts {
	facts := FileFacts{TestFiles: testFilesOf(requested, display)}
	var sdk android.SDKLevelResolver
	for _, path := range requested {
		spelling := path
		if d, ok := display[path]; ok && d != path {
			spelling = d
			if facts.ScanPaths == nil {
				facts.ScanPaths = map[string]string{}
			}
			facts.ScanPaths[path] = d
		}
		if levels := sdk.Resolve(spelling); !levels.IsZero() {
			if facts.SDKLevels == nil {
				facts.SDKLevels = map[string]android.SDKLevels{}
			}
			facts.SDKLevels[path] = levels
		}
	}
	return facts
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
		if r.ID == "TooGenericExceptionCaught" {
			const key = "allowedExceptionNameRegex"
			if value, exists := opts[key]; exists {
				pattern, _ := value.(string)
				if pattern == "" || api.ValidateAnchoredPattern(pattern) != nil {
					delete(opts, key)
				}
			}
		}
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

// generatedSymbolError recognizes the first unresolved generated Android symbol.
func generatedSymbolError(message string) bool {
	lower := strings.ToLower(message)
	if !strings.Contains(lower, "unresolved reference") {
		return false
	}
	if strings.Contains(message, "Unresolved reference 'R'") || strings.Contains(message, "Unresolved reference: 'R'") {
		return true
	}
	for _, field := range strings.Fields(message) {
		symbol := strings.Trim(field, "'\"`:,.;()[]")
		if symbol == "BuildConfig" || strings.HasSuffix(symbol, "Binding") {
			return true
		}
	}
	return false
}
