package v2

// v1compat.go provides reverse adapters: v2.Rule → v1 interface wrappers.
// During migration, the existing v1 dispatcher still needs to consume rules.
// These wrappers implement the v1 interfaces by delegating to the v2.Rule's
// Check function.
//
// Two variants exist per rule family:
//   - plain (e.g. V1FlatDispatch) — for rules that don't declare NeedsResolver
//   - type-aware (e.g. V1FlatDispatchTypeAware) — implements SetResolver so
//     the v1 dispatcher wires the resolver through to the rule's hook
//
// Go uses structural typing, so an unconditional SetResolver method on the
// plain wrapper would make every rule look like a TypeAwareRule, which
// breaks precision classification. Splitting the types preserves the
// classification semantics of the original rules.
//
// Once all rules are migrated and the dispatcher is rewritten, these
// wrappers and the v1 interfaces can be deleted.

import (
	"github.com/kaeawc/krit/internal/android"
	"github.com/kaeawc/krit/internal/module"
	"github.com/kaeawc/krit/internal/scanner"
	"github.com/kaeawc/krit/internal/typeinfer"
)

// stampFindings fills in Rule/RuleSet/Severity/File metadata on any
// findings emitted without those fields set.
func stampFindings(ctx *Context, id, category string, sev Severity, path string) {
	for i := range ctx.Findings {
		if ctx.Findings[i].Rule == "" {
			ctx.Findings[i].Rule = id
		}
		if ctx.Findings[i].RuleSet == "" {
			ctx.Findings[i].RuleSet = category
		}
		if ctx.Findings[i].Severity == "" {
			ctx.Findings[i].Severity = string(sev)
		}
		if ctx.Findings[i].File == "" && path != "" {
			ctx.Findings[i].File = path
		}
	}
}

// --- FlatDispatch: plain variant (no SetResolver) ---------------------------

// V1FlatDispatch wraps a v2 node-dispatch Rule into a type that satisfies
// the v1 FlatDispatchRule interface shape (without TypeAwareRule).
type V1FlatDispatch struct {
	R *Rule
}

func (w *V1FlatDispatch) Name() string                             { return w.R.ID }
func (w *V1FlatDispatch) Description() string                      { return w.R.Description }
func (w *V1FlatDispatch) RuleSet() string                          { return w.R.Category }
func (w *V1FlatDispatch) Severity() string                         { return string(w.R.Sev) }
func (w *V1FlatDispatch) Check(_ *scanner.File) []scanner.Finding  { return nil }
func (w *V1FlatDispatch) NodeTypes() []string                      { return w.R.NodeTypes }
func (w *V1FlatDispatch) IsFixable() bool                          { return w.R.Fix != FixNone }
func (w *V1FlatDispatch) Confidence() float64                      { return w.R.Confidence }
func (w *V1FlatDispatch) V1FixLevel() int                          { return int(w.R.Fix) }

func (w *V1FlatDispatch) CheckFlatNode(idx uint32, file *scanner.File) []scanner.Finding {
	ctx := &Context{File: file, Idx: idx}
	if file.FlatTree != nil && int(idx) < len(file.FlatTree.Nodes) {
		node := file.FlatTree.Nodes[idx]
		ctx.Node = &node
	}
	w.R.Check(ctx)
	stampFindings(ctx, w.R.ID, w.R.Category, w.R.Sev, file.Path)
	return ctx.Findings
}

// --- FlatDispatch: type-aware variant (implements SetResolver) --------------

// V1FlatDispatchTypeAware is the TypeAwareRule variant of V1FlatDispatch.
// Used only when the underlying v2 rule declares NeedsResolver.
type V1FlatDispatchTypeAware struct {
	R *Rule
}

func (w *V1FlatDispatchTypeAware) Name() string                             { return w.R.ID }
func (w *V1FlatDispatchTypeAware) Description() string                      { return w.R.Description }
func (w *V1FlatDispatchTypeAware) RuleSet() string                          { return w.R.Category }
func (w *V1FlatDispatchTypeAware) Severity() string                         { return string(w.R.Sev) }
func (w *V1FlatDispatchTypeAware) Check(_ *scanner.File) []scanner.Finding  { return nil }
func (w *V1FlatDispatchTypeAware) NodeTypes() []string                      { return w.R.NodeTypes }
func (w *V1FlatDispatchTypeAware) IsFixable() bool                          { return w.R.Fix != FixNone }
func (w *V1FlatDispatchTypeAware) Confidence() float64                      { return w.R.Confidence }
func (w *V1FlatDispatchTypeAware) V1FixLevel() int                          { return int(w.R.Fix) }

func (w *V1FlatDispatchTypeAware) CheckFlatNode(idx uint32, file *scanner.File) []scanner.Finding {
	ctx := &Context{File: file, Idx: idx}
	if file.FlatTree != nil && int(idx) < len(file.FlatTree.Nodes) {
		node := file.FlatTree.Nodes[idx]
		ctx.Node = &node
	}
	w.R.Check(ctx)
	stampFindings(ctx, w.R.ID, w.R.Category, w.R.Sev, file.Path)
	return ctx.Findings
}

// SetResolver implements the v1 TypeAwareRule interface. Forwards to the
// v2 rule's registered resolver hook, if any.
func (w *V1FlatDispatchTypeAware) SetResolver(res typeinfer.TypeResolver) {
	if w.R.SetResolverHook != nil {
		w.R.SetResolverHook(res)
	}
}

// --- Line: plain variant ----------------------------------------------------

// V1Line wraps a v2 line-pass Rule into a v1 LineRule.
type V1Line struct {
	R *Rule
}

func (w *V1Line) Name() string                             { return w.R.ID }
func (w *V1Line) Description() string                      { return w.R.Description }
func (w *V1Line) RuleSet() string                          { return w.R.Category }
func (w *V1Line) Severity() string                         { return string(w.R.Sev) }
func (w *V1Line) Check(_ *scanner.File) []scanner.Finding  { return nil }
func (w *V1Line) IsFixable() bool                          { return w.R.Fix != FixNone }
func (w *V1Line) Confidence() float64                      { return w.R.Confidence }
func (w *V1Line) V1FixLevel() int                          { return int(w.R.Fix) }

func (w *V1Line) CheckLines(file *scanner.File) []scanner.Finding {
	ctx := &Context{File: file}
	w.R.Check(ctx)
	stampFindings(ctx, w.R.ID, w.R.Category, w.R.Sev, file.Path)
	return ctx.Findings
}

// --- Line: type-aware variant -----------------------------------------------

// V1LineTypeAware is the TypeAwareRule variant of V1Line.
type V1LineTypeAware struct {
	R *Rule
}

func (w *V1LineTypeAware) Name() string                             { return w.R.ID }
func (w *V1LineTypeAware) Description() string                      { return w.R.Description }
func (w *V1LineTypeAware) RuleSet() string                          { return w.R.Category }
func (w *V1LineTypeAware) Severity() string                         { return string(w.R.Sev) }
func (w *V1LineTypeAware) Check(_ *scanner.File) []scanner.Finding  { return nil }
func (w *V1LineTypeAware) IsFixable() bool                          { return w.R.Fix != FixNone }
func (w *V1LineTypeAware) Confidence() float64                      { return w.R.Confidence }
func (w *V1LineTypeAware) V1FixLevel() int                          { return int(w.R.Fix) }

func (w *V1LineTypeAware) CheckLines(file *scanner.File) []scanner.Finding {
	ctx := &Context{File: file}
	w.R.Check(ctx)
	stampFindings(ctx, w.R.ID, w.R.Category, w.R.Sev, file.Path)
	return ctx.Findings
}

func (w *V1LineTypeAware) SetResolver(res typeinfer.TypeResolver) {
	if w.R.SetResolverHook != nil {
		w.R.SetResolverHook(res)
	}
}

// --- CrossFile --------------------------------------------------------------

// V1CrossFile wraps a v2 cross-file Rule for the v1 CrossFileRule interface.
type V1CrossFile struct {
	R *Rule
}

func (w *V1CrossFile) Name() string                             { return w.R.ID }
func (w *V1CrossFile) Description() string                      { return w.R.Description }
func (w *V1CrossFile) RuleSet() string                          { return w.R.Category }
func (w *V1CrossFile) Severity() string                         { return string(w.R.Sev) }
func (w *V1CrossFile) Check(_ *scanner.File) []scanner.Finding  { return nil }
func (w *V1CrossFile) Confidence() float64                      { return w.R.Confidence }

func (w *V1CrossFile) CheckCrossFile(index *scanner.CodeIndex) []scanner.Finding {
	ctx := &Context{CodeIndex: index}
	w.R.Check(ctx)
	stampFindings(ctx, w.R.ID, w.R.Category, w.R.Sev, "")
	return ctx.Findings
}

// --- ModuleAware ------------------------------------------------------------

// V1ModuleAware wraps a v2 module-aware Rule for the v1 ModuleAwareRule
// interface.
type V1ModuleAware struct {
	R   *Rule
	pmi *module.PerModuleIndex
}

func (w *V1ModuleAware) Name() string                             { return w.R.ID }
func (w *V1ModuleAware) Description() string                      { return w.R.Description }
func (w *V1ModuleAware) RuleSet() string                          { return w.R.Category }
func (w *V1ModuleAware) Severity() string                         { return string(w.R.Sev) }
func (w *V1ModuleAware) Check(_ *scanner.File) []scanner.Finding  { return nil }
func (w *V1ModuleAware) Confidence() float64                      { return w.R.Confidence }

// SetModuleIndex implements the v1 ModuleAwareRule interface.
func (w *V1ModuleAware) SetModuleIndex(pmi *module.PerModuleIndex) {
	w.pmi = pmi
}

// CheckModuleAware implements the v1 ModuleAwareRule interface by invoking
// the v2 rule's Check with a Context carrying the captured ModuleIndex.
func (w *V1ModuleAware) CheckModuleAware() []scanner.Finding {
	ctx := &Context{ModuleIndex: w.pmi}
	w.R.Check(ctx)
	stampFindings(ctx, w.R.ID, w.R.Category, w.R.Sev, "")
	return ctx.Findings
}

// --- Manifest ---------------------------------------------------------------

// V1Manifest wraps a v2 manifest Rule for the v1 ManifestRule interface.
// The Manifest parameter is typed as interface{} because *rules.Manifest
// is in the package that imports v2 (avoiding a cycle). The rules package
// adapter must type-assert back when it calls Check.
type V1Manifest struct {
	R *Rule
}

func (w *V1Manifest) Name() string                             { return w.R.ID }
func (w *V1Manifest) Description() string                      { return w.R.Description }
func (w *V1Manifest) RuleSet() string                          { return w.R.Category }
func (w *V1Manifest) Severity() string                         { return string(w.R.Sev) }
func (w *V1Manifest) Check(_ *scanner.File) []scanner.Finding  { return nil }
func (w *V1Manifest) Confidence() float64                      { return w.R.Confidence }

// CheckManifest is the generic-typed manifest entry point. Callers in the
// rules package wrap this to satisfy the concrete ManifestRule interface
// (which takes *rules.Manifest).
func (w *V1Manifest) CheckManifest(m interface{}) []scanner.Finding {
	ctx := &Context{Manifest: m}
	w.R.Check(ctx)
	stampFindings(ctx, w.R.ID, w.R.Category, w.R.Sev, "")
	return ctx.Findings
}

// --- Resource ---------------------------------------------------------------

// V1Resource wraps a v2 resource Rule for the v1 ResourceRule interface.
type V1Resource struct {
	R *Rule
}

func (w *V1Resource) Name() string                             { return w.R.ID }
func (w *V1Resource) Description() string                      { return w.R.Description }
func (w *V1Resource) RuleSet() string                          { return w.R.Category }
func (w *V1Resource) Severity() string                         { return string(w.R.Sev) }
func (w *V1Resource) Check(_ *scanner.File) []scanner.Finding  { return nil }
func (w *V1Resource) Confidence() float64                      { return w.R.Confidence }

// CheckResources mirrors the v1 ResourceRule method.
func (w *V1Resource) CheckResources(idx *android.ResourceIndex) []scanner.Finding {
	ctx := &Context{ResourceIndex: idx}
	w.R.Check(ctx)
	stampFindings(ctx, w.R.ID, w.R.Category, w.R.Sev, "")
	return ctx.Findings
}

// --- Gradle -----------------------------------------------------------------

// V1Gradle wraps a v2 gradle Rule for the v1 GradleRule interface.
type V1Gradle struct {
	R *Rule
}

func (w *V1Gradle) Name() string                             { return w.R.ID }
func (w *V1Gradle) Description() string                      { return w.R.Description }
func (w *V1Gradle) RuleSet() string                          { return w.R.Category }
func (w *V1Gradle) Severity() string                         { return string(w.R.Sev) }
func (w *V1Gradle) Check(_ *scanner.File) []scanner.Finding  { return nil }
func (w *V1Gradle) Confidence() float64                      { return w.R.Confidence }

// CheckGradle mirrors the v1 GradleRule method signature.
func (w *V1Gradle) CheckGradle(path, content string, cfg *android.BuildConfig) []scanner.Finding {
	ctx := &Context{
		GradlePath:    path,
		GradleContent: content,
		GradleConfig:  cfg,
	}
	w.R.Check(ctx)
	stampFindings(ctx, w.R.ID, w.R.Category, w.R.Sev, path)
	return ctx.Findings
}

// --- Aggregate --------------------------------------------------------------

// V1Aggregate wraps a v2 aggregate Rule for the v1 AggregateRule interface.
type V1Aggregate struct {
	R       *Rule
	curFile *scanner.File
}

func (w *V1Aggregate) Name() string                             { return w.R.ID }
func (w *V1Aggregate) Description() string                      { return w.R.Description }
func (w *V1Aggregate) RuleSet() string                          { return w.R.Category }
func (w *V1Aggregate) Severity() string                         { return string(w.R.Sev) }
func (w *V1Aggregate) Check(_ *scanner.File) []scanner.Finding  { return nil }
func (w *V1Aggregate) Confidence() float64                      { return w.R.Confidence }
func (w *V1Aggregate) AggregateNodeTypes() []string             { return w.R.NodeTypes }

func (w *V1Aggregate) CollectFlatNode(idx uint32, file *scanner.File) {
	if w.R.Aggregate == nil || w.R.Aggregate.Collect == nil {
		return
	}
	w.curFile = file
	ctx := &Context{File: file, Idx: idx}
	if file.FlatTree != nil && int(idx) < len(file.FlatTree.Nodes) {
		node := file.FlatTree.Nodes[idx]
		ctx.Node = &node
	}
	w.R.Aggregate.Collect(ctx)
}

func (w *V1Aggregate) Finalize(file *scanner.File) []scanner.Finding {
	if w.R.Aggregate == nil || w.R.Aggregate.Finalize == nil {
		return nil
	}
	ctx := &Context{File: file}
	w.R.Aggregate.Finalize(ctx)
	stampFindings(ctx, w.R.ID, w.R.Category, w.R.Sev, file.Path)
	return ctx.Findings
}

func (w *V1Aggregate) Reset() {
	if w.R.Aggregate != nil && w.R.Aggregate.Reset != nil {
		w.R.Aggregate.Reset()
	}
	w.curFile = nil
}

// ToV1 converts a v2.Rule into the appropriate v1 interface wrapper
// based on the rule's declared capabilities.
func ToV1(r *Rule) interface{} {
	switch {
	case r.Needs.Has(NeedsAggregate):
		return &V1Aggregate{R: r}
	case r.Needs.Has(NeedsManifest):
		return &V1Manifest{R: r}
	case r.Needs.Has(NeedsResources):
		return &V1Resource{R: r}
	case r.Needs.Has(NeedsGradle):
		return &V1Gradle{R: r}
	case r.Needs.Has(NeedsModuleIndex):
		return &V1ModuleAware{R: r}
	case r.Needs.Has(NeedsCrossFile):
		return &V1CrossFile{R: r}
	case r.Needs.Has(NeedsLinePass):
		if r.Needs.Has(NeedsResolver) {
			return &V1LineTypeAware{R: r}
		}
		return &V1Line{R: r}
	default:
		if r.Needs.Has(NeedsResolver) {
			return &V1FlatDispatchTypeAware{R: r}
		}
		return &V1FlatDispatch{R: r}
	}
}
