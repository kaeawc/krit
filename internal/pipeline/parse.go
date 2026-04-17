package pipeline

import (
	"context"
	"runtime"
	"sort"
	"strings"

	v2 "github.com/kaeawc/krit/internal/rules/v2"
	"github.com/kaeawc/krit/internal/scanner"
)

// ParsePhase is the first phase of the Krit analysis pipeline. It collects
// and parses Kotlin sources under the configured paths, optionally filters
// out generated files, sorts by content length for LPT scheduling, builds
// a per-file SuppressionIndex so downstream cross-file rules can share the
// same suppression map, and collects Java sources when any active rule
// declares NeedsCrossFile.
type ParsePhase struct {
	// Workers overrides the parse worker count. Zero = runtime.NumCPU().
	Workers int
}

// Name returns the stable phase identifier used for timing and error tags.
func (ParsePhase) Name() string { return "parse" }

// Run executes the Parse phase.
//
// Steps:
//  1. Collect Kotlin files under in.Paths (minus in.Excludes).
//  2. Parse them in parallel.
//  3. Drop files under */generated/* unless IncludeGenerated is true.
//  4. Sort by content length descending (LPT scheduling).
//  5. Populate each file's SuppressionIdx so cross-file rules can consult
//     the same @Suppress map as per-file dispatch.
//  6. When any active rule declares NeedsCrossFile, also collect and parse
//     Java sources for cross-reference indexing.
//
// Non-fatal per-file parse failures are returned via ParseResult.ParseErrors
// so downstream phases can still run on the files that did parse.
func (p ParsePhase) Run(ctx context.Context, in ParseInput) (ParseResult, error) {
	if err := ctx.Err(); err != nil {
		return ParseResult{}, err
	}

	workers := p.Workers
	if workers <= 0 {
		workers = runtime.NumCPU()
	}

	kotlinPaths, err := scanner.CollectKotlinFiles(in.Paths, in.Excludes)
	if err != nil {
		return ParseResult{}, err
	}

	kotlinFiles, parseErrs := scanner.ScanFiles(kotlinPaths, workers)

	// Filter generated files unless explicitly requested. Mirrors the
	// main.go behaviour at lines 921-935 — generated dirs contain codegen
	// output that dwarfs hand-written sources and strands workers.
	if !in.IncludeGenerated {
		filtered := kotlinFiles[:0]
		for _, f := range kotlinFiles {
			if strings.Contains(f.Path, "/generated/") {
				continue
			}
			filtered = append(filtered, f)
		}
		kotlinFiles = filtered
	}

	// LPT scheduling: longest files first so workers pick up the heavy
	// work before the small-file queue drains.
	sort.Slice(kotlinFiles, func(i, j int) bool {
		return len(kotlinFiles[i].Content) > len(kotlinFiles[j].Content)
	})

	// Build per-file SuppressionIndex so cross-file / module-aware rules
	// can consult the same @Suppress map the dispatcher uses for per-file
	// rules. LSP/MCP callers that construct files directly still get a
	// nil SuppressionIdx and fall through the "no suppression" path.
	for _, f := range kotlinFiles {
		if f.FlatTree == nil {
			continue
		}
		f.SuppressionIdx = scanner.BuildSuppressionIndexFlat(f.FlatTree, f.Content)
	}

	// Collect Java files only when at least one active rule needs them.
	// All 481 rules already declare their Needs; rules that don't need
	// cross-file data leave this work undone.
	var javaFiles []*scanner.File
	if unionNeeds(in.ActiveRules).Has(v2.NeedsCrossFile) {
		javaPaths, javaErr := scanner.CollectJavaFiles(in.Paths, nil)
		if javaErr != nil {
			// Java indexing is best-effort — surface the error via
			// ParseErrors rather than aborting the whole phase.
			parseErrs = append(parseErrs, javaErr)
		} else if len(javaPaths) > 0 {
			var javaParseErrs []error
			javaFiles, javaParseErrs = scanner.ScanJavaFiles(javaPaths, workers)
			parseErrs = append(parseErrs, javaParseErrs...)
		}
	}

	return ParseResult{
		Config:      in.Config,
		ActiveRules: in.ActiveRules,
		KotlinFiles: kotlinFiles,
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

// Compile-time check: ParsePhase satisfies Phase[ParseInput, ParseResult].
var _ Phase[ParseInput, ParseResult] = ParsePhase{}
