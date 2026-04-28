package pipeline

import (
	"context"
	"runtime"
	"sort"
	"strings"
	"time"

	"github.com/kaeawc/krit/internal/rules"
	v2 "github.com/kaeawc/krit/internal/rules/v2"
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
		kotlinFiles []*scanner.File
		parseErrs   []error
	)
	parseStart := time.Now()
	_ = in.trackSerial("parse", func() error {
		kotlinFiles, parseErrs = scanner.ScanFilesCached(kotlinPaths, workers, in.ParseCache)
		return nil
	})
	in.logf("verbose: Parsed %d files in %v (%d errors, %d workers)\n",
		len(kotlinFiles), time.Since(parseStart).Round(time.Millisecond), len(parseErrs), workers)

	// Filter generated files unless explicitly requested. Mirrors the
	// main.go behaviour at lines 921-935 — generated dirs contain codegen
	// output that dwarfs hand-written sources and strands workers.
	if !in.IncludeGenerated {
		filtered := kotlinFiles[:0]
		var droppedGenerated int
		for _, f := range kotlinFiles {
			if strings.Contains(f.Path, "/generated/") {
				droppedGenerated++
				continue
			}
			filtered = append(filtered, f)
		}
		kotlinFiles = filtered
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
	for _, f := range kotlinFiles {
		if f.FlatTree == nil {
			continue
		}
		f.Suppression = scanner.BuildSuppressionFilter(f, nil, ruleExcludes, "")
		f.SuppressionIdx = f.Suppression.Annotations()
	}

	caps := unionNeeds(in.ActiveRules)

	// Collect Java files only when at least one active rule needs them:
	// cross-file indexing, parsed-file rules, or direct Java source AST
	// dispatch. Callers that manage Java collection themselves (e.g.
	// inside a later phase tracker scope) pass SkipJavaCollection=true.
	var javaFiles []*scanner.File
	if !in.SkipJavaCollection && (caps.Has(v2.NeedsCrossFile) || caps.Has(v2.NeedsParsedFiles) || NeedsJavaSourceDispatch(in.ActiveRules)) {
		javaPaths, javaErr := scanner.CollectJavaFiles(in.Paths, nil)
		if javaErr != nil {
			// Java indexing is best-effort — surface the error via
			// ParseErrors rather than aborting the whole phase.
			parseErrs = append(parseErrs, javaErr)
		} else if len(javaPaths) > 0 {
			var javaParseErrs []error
			javaFiles, javaParseErrs = scanner.ScanJavaFilesCached(javaPaths, workers, in.ParseCache)
			parseErrs = append(parseErrs, javaParseErrs...)
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

// unionNeeds returns the bitwise union of every active rule's Needs.
// Used to decide whether the phase should collect Java sources, etc.,
// without iterating the rule slice more than once per decision.
func unionNeeds(rules []*v2.Rule) v2.Capabilities {
	var caps v2.Capabilities
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
func NeedsJavaSourceDispatch(rules []*v2.Rule) bool {
	for _, r := range rules {
		if r == nil || len(r.NodeTypes) == 0 || !v2.RuleAppliesToLanguage(r, scanner.LangJava) {
			continue
		}
		if r.Needs.Has(v2.NeedsManifest) || r.Needs.Has(v2.NeedsResources) || r.Needs.Has(v2.NeedsGradle) {
			continue
		}
		return true
	}
	return false
}

// NeedsJavaBeforeDispatch reports whether Java sources must be parsed before
// the normal dispatch/cross-file handoff. Parsed-file rules need the complete
// source set, and Java source rules need Java ASTs for per-file dispatch.
func NeedsJavaBeforeDispatch(rules []*v2.Rule) bool {
	return unionNeeds(rules).Has(v2.NeedsParsedFiles) || NeedsJavaSourceDispatch(rules)
}

// Compile-time check: ParsePhase satisfies Phase[ParseInput, ParseResult].
var _ Phase[ParseInput, ParseResult] = ParsePhase{}
