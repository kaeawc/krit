package pipeline

import (
	"context"
	"runtime"
	"sort"
	"strings"
	"time"

	"github.com/kaeawc/krit/internal/rules"
	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
)

// ParsePhase is the first phase of the Krit analysis pipeline. It collects
// and parses Kotlin sources under the configured paths, optionally filters
// out generated files, sorts by content length for LPT scheduling, builds
// a per-file SuppressionIndex so downstream cross-file rules can share the
// same suppression map, and collects Java sources when any active rule
// declares NeedsCrossFile, NeedsParsedFiles, or Java source dispatch.
type ParsePhase struct {
	// Workers overrides the parse worker count. Zero = runtime.NumCPU().
	// When ParseInput.Workers is non-zero, that takes precedence.
	Workers int
}

// Name returns the stable phase identifier used for timing and error tags.
func (ParsePhase) Name() string { return "parse" }

// Run executes the Parse phase.
//
// Steps:
//  1. Collect Kotlin files under in.Paths (minus in.Excludes). If
//     in.KotlinPaths is non-nil, use it directly and skip collection.
//  2. Parse them in parallel.
//  3. Drop files under */generated/* unless IncludeGenerated is true.
//  4. Sort by content length descending (LPT scheduling).
//  5. Populate each file's SuppressionIdx so cross-file rules can consult
//     the same @Suppress map as per-file dispatch.
//  6. When any active rule declares NeedsCrossFile, also collect and parse
//     Java sources for cross-reference indexing. Skipped entirely when
//     in.SkipJavaCollection is true (caller handles Java elsewhere).
//
// Non-fatal per-file parse failures are returned via ParseResult.ParseErrors
// so downstream phases can still run on the files that did parse.
func (p ParsePhase) Run(ctx context.Context, in ParseInput) (ParseResult, error) {
	if err := ctx.Err(); err != nil {
		return ParseResult{}, err
	}

	workers := in.Workers
	if workers <= 0 {
		workers = p.Workers
	}
	if workers <= 0 {
		workers = runtime.NumCPU()
	}

	kotlinPaths := in.KotlinPaths
	if kotlinPaths == nil {
		collected, err := scanner.CollectKotlinFiles(in.Paths, in.Excludes)
		if err != nil {
			return ParseResult{}, err
		}
		kotlinPaths = collected
	}

	var (
		kotlinFiles  []*scanner.File
		parseErrs    []error
		residentHits int
	)
	parseStart := time.Now()
	_ = in.trackSerial("parse", func() error {
		kotlinFiles, parseErrs, residentHits = scanWithResident(ctx, kotlinPaths, workers, in.ParseCache, in.ResidentFiles, scanner.ScanFilesCached)
		return nil
	})
	in.logf("verbose: Parsed %d files in %v (%d errors, %d workers, %d resident hits)\n",
		len(kotlinFiles), time.Since(parseStart).Round(time.Millisecond), len(parseErrs), workers, residentHits)

	// Filter generated Kotlin files unless explicitly requested. Generated
	// dirs contain codegen output that dwarfs hand-written sources and
	// strands workers. When IncludeGeneratedAllowlist is set, files whose
	// paths match a known-safe-generator substring are kept (and tagged
	// File.Generated = true) so the resolver can index them; the
	// dispatcher and rule layer still see them as files but can opt out
	// via File.Generated.
	if !in.IncludeGenerated {
		var droppedGenerated int
		kotlinFiles, droppedGenerated = filterGeneratedSourceFilesWithAllowlist(kotlinFiles, in.IncludeGeneratedAllowlist)
		if droppedGenerated > 0 {
			in.logf("verbose: Skipped %d files in */generated/* dirs (pass --include-generated to re-enable)\n", droppedGenerated)
		}
	}

	// LPT scheduling: longest files first so workers pick up the heavy
	// work before the small-file queue drains.
	sort.Slice(kotlinFiles, func(i, j int) bool {
		return len(kotlinFiles[i].Content) > len(kotlinFiles[j].Content)
	})

	// Build per-file SuppressionFilter once per file. This unifies the
	// four pre-refactor suppression sources (annotations, config
	// excludes, baseline, inline comments) into a single filter that
	// both the dispatcher and the cross-file phase consult, instead of
	// each call site rebuilding the annotation index independently.
	// LSP/MCP callers that construct files directly still get nil
	// fields here and fall through to the dispatcher's lazy build.
	ruleExcludes := rules.GetAllRuleExcludes()
	ruleAliases := rules.AllSuppressionAliases()
	for _, f := range kotlinFiles {
		installSourceSuppression(f, ruleExcludes, ruleAliases)
	}

	caps := unionNeeds(in.ActiveRules)

	// Collect Java files only when at least one active rule needs them:
	// cross-file indexing, parsed-file rules, or direct Java source AST
	// dispatch. Callers that manage Java collection themselves (e.g.
	// inside a later phase tracker scope) pass SkipJavaCollection=true.
	var javaFiles []*scanner.File
	if !in.SkipJavaCollection && (caps.Has(api.NeedsCrossFile) || caps.Has(api.NeedsParsedFiles) || NeedsJavaSourceDispatch(in.ActiveRules)) {
		javaPaths := in.JavaPaths
		var javaErr error
		if javaPaths == nil {
			javaPaths, javaErr = scanner.CollectJavaFiles(in.Paths, in.Excludes)
		}
		if javaErr != nil {
			// Java indexing is best-effort — surface the error via
			// ParseErrors rather than aborting the whole phase.
			parseErrs = append(parseErrs, javaErr)
		} else if len(javaPaths) > 0 {
			var javaParseErrs []error
			javaFiles, javaParseErrs, _ = scanWithResident(ctx, javaPaths, workers, in.ParseCache, in.ResidentFiles, scanner.ScanJavaFilesCached)
			parseErrs = append(parseErrs, javaParseErrs...)
			if !in.IncludeGenerated {
				javaFiles, _ = filterGeneratedSourceFilesWithAllowlist(javaFiles, in.IncludeGeneratedAllowlist)
			}
			for _, f := range javaFiles {
				installSourceSuppression(f, ruleExcludes, ruleAliases)
			}
		}
	}

	return ParseResult{
		Config:      in.Config,
		ActiveRules: in.ActiveRules,
		KotlinFiles: kotlinFiles,
		KotlinPaths: kotlinPaths,
		JavaFiles:   javaFiles,
		Paths:       in.Paths,
		ParseErrors: parseErrs,
	}, nil
}

// DefaultKnownSafeGenerators returns path substrings selecting build outputs
// from the well-known annotation processors and Gradle code generators
// whose output the resolver should index so cross-file references into
// generated code (Hilt components, KSP-generated factories, Room *_Impl
// classes, ViewBinding/DataBinding bindings, BuildConfig) resolve in
// source typeinfer. Callers wire this list into ParseInput when they
// want generated symbols to be type-aware without including arbitrary
// machine-written code in linting.
func DefaultKnownSafeGenerators() []string {
	return []string{
		"build/generated/source/kapt/",
		"build/generated/source/kaptKotlin/",
		"build/generated/ksp/",
		"build/generated/source/buildConfig/",
		"build/generated/source/viewBinding/",
		"build/generated/source/dataBinding/",
		"build/generated/data_binding_base_class_source_out/",
		"build/generated/hilt/",
	}
}

func filterGeneratedSourceFilesWithAllowlist(files []*scanner.File, allowlist []string) ([]*scanner.File, int) {
	// Allocate a fresh slice — sibling [:0] filters in the path
	// pipeline (filterGeneratedSourcePaths, filterGeneratedPathStrings)
	// already cause caller-slice corruption when input is aliased. Match
	// that fix here so future callers of this *File filter cannot
	// re-introduce the bug by accident.
	filtered := make([]*scanner.File, 0, len(files))
	var droppedGenerated int
	for _, f := range files {
		if f == nil || !strings.Contains(f.Path, "/generated/") {
			filtered = append(filtered, f)
			continue
		}
		if pathMatchesAnySubstring(f.Path, allowlist) {
			f.Generated = true
			filtered = append(filtered, f)
			continue
		}
		droppedGenerated++
	}
	return filtered, droppedGenerated
}

func pathMatchesAnySubstring(path string, substrings []string) bool {
	for _, s := range substrings {
		if s == "" {
			continue
		}
		if strings.Contains(path, s) {
			return true
		}
	}
	return false
}

func installSourceSuppression(f *scanner.File, ruleExcludes map[string][]string, ruleAliases map[string][]string) {
	if f == nil || f.FlatTree == nil {
		return
	}
	// Resident-cache hits arrive with Suppression already built from a
	// prior analyze. Rules-config changes invalidate the whole resident
	// cache (via WorkspaceState.InvalidateAll), so a non-nil Suppression
	// can be trusted for the current run — rebuilding it would walk the
	// flat tree for every file, costing ~50µs each (~700ms on a 13k-file
	// corpus).
	if f.Suppression != nil && f.SuppressionIdx != nil {
		return
	}
	f.Suppression = scanner.BuildSuppressionFilter(f, nil, ruleExcludes, "").WithRuleAliases(ruleAliases)
	f.SuppressionIdx = f.Suppression.Annotations()
}

// unionNeeds returns the bitwise union of every active rule's Needs.
// Used to decide whether the phase should collect Java sources, etc.,
// without iterating the rule slice more than once per decision.
func unionNeeds(rules []*api.Rule) api.Capabilities {
	var caps api.Capabilities
	for _, r := range rules {
		if r == nil {
			continue
		}
		caps |= r.Needs
	}
	return caps
}

// NeedsJavaSourceDispatch reports whether any active per-file source rule is
// declared for Java. Project-resource rule families are intentionally excluded
// because their inputs are provided by later Android/Gradle phases.
func NeedsJavaSourceDispatch(rules []*api.Rule) bool {
	for _, r := range rules {
		if r == nil || len(r.NodeTypes) == 0 || !api.RuleAppliesToLanguage(r, scanner.LangJava) {
			continue
		}
		if r.Needs.Has(api.NeedsManifest) || r.Needs.Has(api.NeedsResources) || r.Needs.Has(api.NeedsGradle) {
			continue
		}
		return true
	}
	return false
}

// NeedsJavaBeforeDispatch reports whether Java sources must be parsed before
// the normal dispatch/cross-file handoff. Parsed-file rules need the complete
// source set, and Java source rules need Java ASTs for per-file dispatch.
func NeedsJavaBeforeDispatch(rules []*api.Rule) bool {
	return unionNeeds(rules).Has(api.NeedsParsedFiles) || NeedsJavaSourceDispatch(rules)
}

// scanWithResident partitions paths into resident-cache hits (returned
// without any disk read) and misses (forwarded to scan). The returned
// file slice preserves the order: resident hits first in input order,
// then freshly-scanned misses. Newly-parsed files are stored back in
// the resident cache so the next call can short-circuit.
//
// A nil cache disables the fast path entirely — the function then
// just forwards every path to scan, matching the pre-#254 behavior.
// Errors from scan are returned unchanged.
func scanWithResident(
	ctx context.Context,
	paths []string,
	workers int,
	pc *scanner.ParseCache,
	cache ResidentFileCache,
	scan func(context.Context, []string, int, *scanner.ParseCache) ([]*scanner.File, []error),
) ([]*scanner.File, []error, int) {
	if cache == nil || len(paths) == 0 {
		files, errs := scan(ctx, paths, workers, pc)
		return files, errs, 0
	}
	hits := make([]*scanner.File, 0, len(paths))
	misses := make([]string, 0)
	for _, p := range paths {
		if f, ok := cache.LookupParsedByPath(p); ok {
			hits = append(hits, f)
			continue
		}
		misses = append(misses, p)
	}
	freshFiles, errs := scan(ctx, misses, workers, pc)
	for _, f := range freshFiles {
		if f == nil {
			continue
		}
		cache.StoreParsed(f.Path, f.Content, f)
	}
	out := make([]*scanner.File, 0, len(hits)+len(freshFiles))
	out = append(out, hits...)
	out = append(out, freshFiles...)
	return out, errs, len(hits)
}

// Compile-time check: ParsePhase satisfies Phase[ParseInput, ParseResult].
var _ Phase[ParseInput, ParseResult] = ParsePhase{}
