package v2

// adapt.go provides adapter functions that convert v1 rule interface
// implementations into v2.Rule values. This enables incremental migration:
// existing rules can be wrapped without modifying their implementation.

import (
	"github.com/kaeawc/krit/internal/android"
	"github.com/kaeawc/krit/internal/module"
	"github.com/kaeawc/krit/internal/scanner"
	"github.com/kaeawc/krit/internal/typeinfer"
)

// AdaptFlatDispatch wraps a v1 FlatDispatchRule-shaped rule into a v2.Rule.
// The caller provides the rule metadata and the check function signature
// matching the old CheckFlatNode(idx uint32, file *scanner.File) []scanner.Finding.
func AdaptFlatDispatch(id, category, desc string, sev Severity, nodeTypes []string,
	check func(idx uint32, file *scanner.File) []scanner.Finding,
	opts ...AdaptOption,
) *Rule {
	o := applyAdaptOpts(opts)
	return &Rule{
		ID:              id,
		Category:        category,
		Description:     desc,
		Sev:             sev,
		NodeTypes:       nodeTypes,
		Needs:           o.needs,
		Fix:             o.fix,
		Confidence:      o.confidence,
		Oracle:          o.oracle,
		SetResolverHook: o.setResolverHook,
		OriginalV1:      o.originalV1,
		Check: func(ctx *Context) {
			for _, f := range check(ctx.Idx, ctx.File) {
				ctx.Emit(f)
			}
		},
	}
}

// AdaptLine wraps a v1 LineRule-shaped rule into a v2.Rule.
func AdaptLine(id, category, desc string, sev Severity,
	check func(file *scanner.File) []scanner.Finding,
	opts ...AdaptOption,
) *Rule {
	o := applyAdaptOpts(opts)
	o.needs |= NeedsLinePass
	return &Rule{
		ID:              id,
		Category:        category,
		Description:     desc,
		Sev:             sev,
		Needs:           o.needs,
		Fix:             o.fix,
		Confidence:      o.confidence,
		Oracle:          o.oracle,
		SetResolverHook: o.setResolverHook,
		Check: func(ctx *Context) {
			for _, f := range check(ctx.File) {
				ctx.Emit(f)
			}
		},
	}
}

// AdaptCrossFile wraps a v1 CrossFileRule-shaped rule into a v2.Rule.
func AdaptCrossFile(id, category, desc string, sev Severity,
	check func(index *scanner.CodeIndex) []scanner.Finding,
	opts ...AdaptOption,
) *Rule {
	o := applyAdaptOpts(opts)
	o.needs |= NeedsCrossFile
	return &Rule{
		ID:          id,
		Category:    category,
		Description: desc,
		Sev:         sev,
		Needs:       o.needs,
		Fix:         o.fix,
		Confidence:  o.confidence,
		Oracle:      o.oracle,
		Check: func(ctx *Context) {
			for _, f := range check(ctx.CodeIndex) {
				ctx.Emit(f)
			}
		},
	}
}

// AdaptParsedFiles wraps a v1 ParsedFilesRule-shaped rule into a v2.Rule.
func AdaptParsedFiles(id, category, desc string, sev Severity,
	check func(files []*scanner.File) []scanner.Finding,
	opts ...AdaptOption,
) *Rule {
	o := applyAdaptOpts(opts)
	o.needs |= NeedsParsedFiles
	return &Rule{
		ID:          id,
		Category:    category,
		Description: desc,
		Sev:         sev,
		Needs:       o.needs,
		Fix:         o.fix,
		Confidence:  o.confidence,
		Oracle:      o.oracle,
		Check: func(ctx *Context) {
			for _, f := range check(ctx.ParsedFiles) {
				ctx.Emit(f)
			}
		},
	}
}

// AdaptOption configures an adapter.
type AdaptOption func(*adaptOpts)

type adaptOpts struct {
	needs           Capabilities
	fix             FixLevel
	confidence      float64
	oracle          *OracleFilter
	setResolverHook func(typeinfer.TypeResolver)
	originalV1      interface{}
}

func applyAdaptOpts(opts []AdaptOption) adaptOpts {
	var o adaptOpts
	for _, opt := range opts {
		opt(&o)
	}
	return o
}

// AdaptWithNeeds adds capabilities to the adapted rule.
func AdaptWithNeeds(c Capabilities) AdaptOption {
	return func(o *adaptOpts) { o.needs |= c }
}

// AdaptWithFix sets the fix level.
func AdaptWithFix(level FixLevel) AdaptOption {
	return func(o *adaptOpts) { o.fix = level }
}

// AdaptWithConfidence sets the base confidence.
func AdaptWithConfidence(c float64) AdaptOption {
	return func(o *adaptOpts) { o.confidence = c }
}

// AdaptWithOracle sets the oracle filter.
func AdaptWithOracle(f *OracleFilter) AdaptOption {
	return func(o *adaptOpts) { o.oracle = f }
}

// AdaptWithResolverHook stores a callback that the v1 dispatcher can use
// to forward SetResolver calls to the original rule struct. This ensures
// that rules with a captured check closure receive the type resolver.
func AdaptWithResolverHook(fn func(typeinfer.TypeResolver)) AdaptOption {
	return func(o *adaptOpts) { o.setResolverHook = fn }
}

// AdaptWithOriginalV1 stores a pointer to the underlying v1 rule struct
// on the resulting v2.Rule. This preserves the Unwrap path for adapter-
// wrapped rules that have config options whose Apply closures need to
// target the concrete struct. Without this, AdaptFlatDispatch drops the
// concrete pointer and rules.Unwrap returns a wrapper that doesn't
// implement registry.MetaProvider, so Meta() is unreachable.
func AdaptWithOriginalV1(orig interface{}) AdaptOption {
	return func(o *adaptOpts) { o.originalV1 = orig }
}

// AdaptModuleAware wraps a v1 ModuleAwareRule-shaped check function into
// a v2.Rule. The rule automatically has NeedsModuleIndex added to its
// capabilities. The check closure receives the PerModuleIndex that the
// dispatcher populates on the Context.
func AdaptModuleAware(id, category, desc string, sev Severity,
	check func(pmi *module.PerModuleIndex) []scanner.Finding,
	opts ...AdaptOption,
) *Rule {
	o := applyAdaptOpts(opts)
	o.needs |= NeedsModuleIndex
	return &Rule{
		ID:          id,
		Category:    category,
		Description: desc,
		Sev:         sev,
		Needs:       o.needs,
		Fix:         o.fix,
		Confidence:  o.confidence,
		Oracle:      o.oracle,
		Check: func(ctx *Context) {
			for _, f := range check(ctx.ModuleIndex) {
				ctx.Emit(f)
			}
		},
	}
}

// AdaptManifest wraps a v1 ManifestRule-shaped check function into a
// v2.Rule. The rule automatically has NeedsManifest added to its
// capabilities. The check closure receives the parsed manifest as an
// opaque value; callers are expected to type-assert it back to
// *rules.Manifest (kept as interface{} here to avoid an import cycle
// between internal/rules/v2 and internal/rules).
func AdaptManifest(id, category, desc string, sev Severity,
	check func(manifest interface{}) []scanner.Finding,
	opts ...AdaptOption,
) *Rule {
	o := applyAdaptOpts(opts)
	o.needs |= NeedsManifest
	return &Rule{
		ID:          id,
		Category:    category,
		Description: desc,
		Sev:         sev,
		Needs:       o.needs,
		Fix:         o.fix,
		Confidence:  o.confidence,
		Oracle:      o.oracle,
		Check: func(ctx *Context) {
			for _, f := range check(ctx.Manifest) {
				ctx.Emit(f)
			}
		},
	}
}

// AdaptResource wraps a v1 ResourceRule-shaped check function into a
// v2.Rule. The rule automatically has NeedsResources added to its
// capabilities. The check closure receives the Android resource index
// the dispatcher populates on the Context.
func AdaptResource(id, category, desc string, sev Severity,
	check func(idx *android.ResourceIndex) []scanner.Finding,
	opts ...AdaptOption,
) *Rule {
	o := applyAdaptOpts(opts)
	o.needs |= NeedsResources
	return &Rule{
		ID:          id,
		Category:    category,
		Description: desc,
		Sev:         sev,
		Needs:       o.needs,
		Fix:         o.fix,
		Confidence:  o.confidence,
		Oracle:      o.oracle,
		Check: func(ctx *Context) {
			for _, f := range check(ctx.ResourceIndex) {
				ctx.Emit(f)
			}
		},
	}
}

// AdaptGradle wraps a v1 GradleRule-shaped check function into a
// v2.Rule. The rule automatically has NeedsGradle added to its
// capabilities. The check closure receives the Gradle file path,
// raw content, and parsed BuildConfig from the Context.
func AdaptGradle(id, category, desc string, sev Severity,
	check func(path, content string, cfg *android.BuildConfig) []scanner.Finding,
	opts ...AdaptOption,
) *Rule {
	o := applyAdaptOpts(opts)
	o.needs |= NeedsGradle
	return &Rule{
		ID:          id,
		Category:    category,
		Description: desc,
		Sev:         sev,
		Needs:       o.needs,
		Fix:         o.fix,
		Confidence:  o.confidence,
		Oracle:      o.oracle,
		Check: func(ctx *Context) {
			for _, f := range check(ctx.GradlePath, ctx.GradleContent, ctx.GradleConfig) {
				ctx.Emit(f)
			}
		},
	}
}

// AdaptAggregate wraps a v1 AggregateRule-shaped rule into a v2.Rule.
// Aggregate rules participate in the per-file AST walk via Collect, then
// produce findings in a Finalize post-pass. Reset clears per-file state.
//
// The collectNode closure is called for each matching node and receives
// the flat node index plus the file. The finalize closure is called
// once per file after the walk and returns the findings produced from
// the collected state. The reset closure clears any accumulated state
// before the next file.
func AdaptAggregate(id, category, desc string, sev Severity, nodeTypes []string,
	collectNode func(idx uint32, file *scanner.File),
	finalize func(file *scanner.File) []scanner.Finding,
	reset func(),
	opts ...AdaptOption,
) *Rule {
	o := applyAdaptOpts(opts)
	o.needs |= NeedsAggregate
	return &Rule{
		ID:          id,
		Category:    category,
		Description: desc,
		Sev:         sev,
		NodeTypes:   nodeTypes,
		Needs:       o.needs,
		Fix:         o.fix,
		Confidence:  o.confidence,
		Oracle:      o.oracle,
		Aggregate: &Aggregate{
			Collect: func(ctx *Context) {
				if collectNode != nil {
					collectNode(ctx.Idx, ctx.File)
				}
			},
			Finalize: func(ctx *Context) {
				if finalize != nil {
					for _, f := range finalize(ctx.File) {
						ctx.Emit(f)
					}
				}
			},
			Reset: func() {
				if reset != nil {
					reset()
				}
			},
		},
	}
}
