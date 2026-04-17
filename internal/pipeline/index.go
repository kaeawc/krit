package pipeline

import (
	"context"
	"runtime"

	"github.com/kaeawc/krit/internal/android"
	"github.com/kaeawc/krit/internal/module"
	v2 "github.com/kaeawc/krit/internal/rules/v2"
	"github.com/kaeawc/krit/internal/scanner"
	"github.com/kaeawc/krit/internal/typeinfer"
)

// IndexPhase builds the non-per-file analysis state downstream phases
// need: a type resolver (with its parallel index), a module graph, and
// the Android project descriptor. Oracle, CodeIndex, and incremental
// cache are populated by callers that pre-build them (LSP caches them
// across edits, the CLI wires them through the IndexInput hooks). This
// phase's job is the cheap, deterministic pieces that always need to run.
type IndexPhase struct {
	// Workers overrides the type-index worker count. Zero = runtime.NumCPU().
	Workers int
	// ScanRoot, if non-empty, is passed to module.DiscoverModules.
	// Empty means "use the first element of ParseResult.Paths, or '.'".
	ScanRoot string
	// SkipModules, when true, leaves ModuleGraph nil. Used by the LSP
	// server where a single open file is analysed without a Gradle
	// project around it.
	SkipModules bool
	// SkipAndroid, when true, leaves AndroidProject nil.
	SkipAndroid bool
}

// IndexInput carries ParseResult plus optional pre-built resolver and
// oracle from callers that already have them (LSP, MCP, --input-types).
// When these are nil, IndexPhase builds fresh state.
type IndexInput struct {
	ParseResult
	// PrebuiltResolver, when non-nil, is used as-is; no IndexFilesParallel
	// call is made. When nil, IndexPhase builds one if any active rule
	// declares NeedsResolver.
	PrebuiltResolver typeinfer.TypeResolver
}

// Name implements Phase.
func (IndexPhase) Name() string { return "index" }

// Run implements Phase.
func (p IndexPhase) Run(ctx context.Context, in IndexInput) (IndexResult, error) {
	if err := ctx.Err(); err != nil {
		return IndexResult{}, err
	}

	result := IndexResult{ParseResult: in.ParseResult}

	caps := unionNeeds(in.ActiveRules)

	// Type resolver
	if in.PrebuiltResolver != nil {
		result.Resolver = in.PrebuiltResolver
	} else if caps.Has(v2.NeedsResolver) {
		r := typeinfer.NewResolver()
		if err := ctx.Err(); err != nil {
			return IndexResult{}, err
		}
		workers := p.Workers
		if workers <= 0 {
			workers = runtime.NumCPU()
		}
		if indexer, ok := interface{}(r).(interface {
			IndexFilesParallel([]*scanner.File, int)
		}); ok {
			indexer.IndexFilesParallel(in.KotlinFiles, workers)
		}
		result.Resolver = r
	}

	// Module graph
	if !p.SkipModules {
		scanRoot := p.ScanRoot
		if scanRoot == "" {
			scanRoot = "."
			if len(in.Paths) > 0 {
				scanRoot = in.Paths[0]
			}
		}
		if graph, err := module.DiscoverModules(scanRoot); err == nil {
			result.ModuleGraph = graph
		}
	}

	// Android project
	if !p.SkipAndroid {
		result.AndroidProject = android.DetectAndroidProject(in.Paths)
	}

	return result, nil
}

// Compile-time check.
var _ Phase[IndexInput, IndexResult] = IndexPhase{}
