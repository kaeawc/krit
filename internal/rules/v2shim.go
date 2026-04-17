package rules

// v2shim.go provides the WrapAsV2 function that converts any v1 rule
// instance into a v2.Rule. This is the migration shim described in the
// roadmap: existing rules continue to compile and pass tests while the
// dispatcher can operate on v2.Rule values internally.
//
// Once all rules are natively v2, this file and the v1 interfaces can
// be deleted.

import (
	"github.com/kaeawc/krit/internal/android"
	"github.com/kaeawc/krit/internal/module"
	v2 "github.com/kaeawc/krit/internal/rules/v2"
	"github.com/kaeawc/krit/internal/scanner"
	"github.com/kaeawc/krit/internal/typeinfer"
)

// WrapAsV2 converts any v1 Rule into a v2.Rule. The resulting rule
// delegates its Check function to the original v1 implementation.
// This enables the dispatcher to be rewritten around v2.Rule without
// changing any existing rule files.
func WrapAsV2(r Rule) *v2.Rule {
	rule := &v2.Rule{
		ID:          r.Name(),
		Category:    r.RuleSet(),
		Description: r.Description(),
		Sev:         v2.Severity(r.Severity()),
		OriginalV1:  r,
	}

	// Preserve the original Android data dependency so bridge wrappers
	// can return the exact same value (not a hardcoded family constant).
	if adp, ok := r.(AndroidDependencyProvider); ok {
		rule.AndroidDeps = uint32(adp.AndroidDependencies())
	}

	// Transfer optional provider interfaces
	if cp, ok := r.(interface{ Confidence() float64 }); ok {
		rule.Confidence = cp.Confidence()
	}
	// Only transfer the FixLevel when the rule actually declares itself
	// fixable. Some rules define a FixLevel() for categorization but
	// set IsFixable() to false because they can't produce a safe fix on
	// every input. We must preserve that distinction so that the v2
	// wrapper's IsFixable() (which derives from Fix != FixNone) matches
	// the original behavior.
	if fl, ok := r.(interface{ FixLevel() FixLevel }); ok {
		fixable := true
		if fr, ok := r.(FixableRule); ok {
			fixable = fr.IsFixable()
		}
		if fixable {
			rule.Fix = v2.FixLevel(fl.FixLevel())
		}
	}
	if ofp, ok := r.(interface{ OracleFilter() *OracleFilter }); ok {
		if of := ofp.OracleFilter(); of != nil {
			rule.Oracle = &v2.OracleFilter{
				Identifiers: of.Identifiers,
				AllFiles:    of.AllFiles,
			}
		}
	}

	// Wire up resolver hook for rules that implement a SetResolver method
	// so the v1 dispatcher can thread the resolver through to the rule's
	// own SetResolver() method when the hook fires.
	type typeAware interface {
		SetResolver(resolver typeinfer.TypeResolver)
	}
	if ta, ok := r.(typeAware); ok {
		rule.SetResolverHook = func(res typeinfer.TypeResolver) {
			ta.SetResolver(res)
		}
	}

	// Route by rule family (structural interface) — order matters (most specific first)
	if v, ok := r.(interface {
		NodeTypes() []string
		CheckFlatNode(idx uint32, file *scanner.File) []scanner.Finding
	}); ok {
		rule.NodeTypes = v.NodeTypes()
		rule.Check = func(ctx *v2.Context) {
			findings := v.CheckFlatNode(ctx.Idx, ctx.File)
			ctx.Findings = append(ctx.Findings, findings...)
		}
		if _, ok := r.(typeAware); ok {
			rule.Needs |= v2.NeedsResolver
		}
		return rule
	}

	if v, ok := r.(interface {
		AggregateNodeTypes() []string
		CollectFlatNode(idx uint32, file *scanner.File)
		Finalize(file *scanner.File) []scanner.Finding
		Reset()
	}); ok {
		// Aggregate lifecycle (Collect + Finalize).
		// We handle this with a no-op Check; the v2 dispatcher must
		// call CollectFlatNode/Finalize directly via the AggregateAdapter.
		rule.NodeTypes = v.AggregateNodeTypes()
		rule.Check = func(ctx *v2.Context) {} // placeholder
		return rule
	}

	if v, ok := r.(interface {
		CheckLines(file *scanner.File) []scanner.Finding
	}); ok {
		rule.Needs = v2.NeedsLinePass
		if _, ok := r.(typeAware); ok {
			rule.Needs |= v2.NeedsResolver
		}
		rule.Check = func(ctx *v2.Context) {
			findings := v.CheckLines(ctx.File)
			ctx.Findings = append(ctx.Findings, findings...)
		}
		return rule
	}

	if v, ok := r.(interface {
		CheckCrossFile(index *scanner.CodeIndex) []scanner.Finding
	}); ok {
		rule.Needs = v2.NeedsCrossFile
		rule.Check = func(ctx *v2.Context) {
			findings := v.CheckCrossFile(ctx.CodeIndex)
			ctx.Findings = append(ctx.Findings, findings...)
		}
		return rule
	}

	if v, ok := r.(interface {
		SetModuleIndex(pmi *module.PerModuleIndex)
		CheckModuleAware() []scanner.Finding
	}); ok {
		rule.Needs = v2.NeedsModuleIndex
		rule.Check = func(ctx *v2.Context) {
			v.SetModuleIndex(ctx.ModuleIndex)
			findings := v.CheckModuleAware()
			ctx.Findings = append(ctx.Findings, findings...)
		}
		return rule
	}

	if v, ok := r.(interface {
		CheckManifest(m *Manifest) []scanner.Finding
	}); ok {
		rule.Needs = v2.NeedsManifest
		rule.Check = func(ctx *v2.Context) {
			m, _ := ctx.Manifest.(*Manifest)
			findings := v.CheckManifest(m)
			ctx.Findings = append(ctx.Findings, findings...)
		}
		return rule
	}

	if v, ok := r.(interface {
		CheckResources(idx *android.ResourceIndex) []scanner.Finding
	}); ok {
		rule.Needs = v2.NeedsResources
		rule.Check = func(ctx *v2.Context) {
			findings := v.CheckResources(ctx.ResourceIndex)
			ctx.Findings = append(ctx.Findings, findings...)
		}
		return rule
	}

	if v, ok := r.(interface {
		CheckGradle(path string, content string, cfg *android.BuildConfig) []scanner.Finding
	}); ok {
		rule.Needs = v2.NeedsGradle
		rule.Check = func(ctx *v2.Context) {
			findings := v.CheckGradle(ctx.GradlePath, ctx.GradleContent, ctx.GradleConfig)
			ctx.Findings = append(ctx.Findings, findings...)
		}
		return rule
	}

	// Legacy rules — use the old Check() method
	rule.Check = func(ctx *v2.Context) {
		findings := r.Check(ctx.File)
		ctx.Findings = append(ctx.Findings, findings...)
	}
	return rule
}

// WrapAllAsV2 converts a slice of v1 rules into v2.Rule values. If a
// rule is already a v2 compat wrapper (produced by an earlier call to
// WrapAsV2 → ToV1), it is passed through to its underlying *v2.Rule so
// we don't double-wrap.
func WrapAllAsV2(rules []Rule) []*v2.Rule {
	result := make([]*v2.Rule, len(rules))
	for i, r := range rules {
		if vr := extractV2Rule(r); vr != nil {
			result[i] = vr
		} else {
			result[i] = WrapAsV2(r)
		}
	}
	return result
}

// v2WrappedRule is the interface satisfied by compat wrappers that carry
// a pointer to the underlying v2.Rule (which in turn may hold the
// OriginalV1 pointer). All the family wrappers (*v2.V1FlatDispatch,
// *v2.V1Line, *v2.V1CrossFile, *v2GradleWrapper, etc.) expose the v2.Rule
// via an R field, but the field is on different struct types, so we use
// an interface to extract it generically.
type v2WrappedRule interface {
	v2Rule() *v2.Rule
}

// Unwrap returns the original v1 rule struct if `r` is a wrapper produced
// by the v2 bridge (WrapAsV2 → ToV1), otherwise returns `r` unchanged.
// Tests and advanced callers use this to recover the concrete rule type
// for type assertions and field access.
func Unwrap(r Rule) Rule {
	type hasR interface{ underlying() *v2.Rule }
	// Try the direct wrapper types first.
	if vr := extractV2Rule(r); vr != nil && vr.OriginalV1 != nil {
		if orig, ok := vr.OriginalV1.(Rule); ok {
			return orig
		}
	}
	return r
}

// extractV2Rule looks for an embedded v2 compat wrapper and returns its
// underlying *v2.Rule. Returns nil if r is not a v2 wrapper.
func extractV2Rule(r Rule) *v2.Rule {
	switch w := r.(type) {
	case *v2.V1FlatDispatch:
		return w.R
	case *v2.V1FlatDispatchTypeAware:
		return w.R
	case *v2.V1Line:
		return w.R
	case *v2.V1LineTypeAware:
		return w.R
	case *v2.V1CrossFile:
		return w.R
	case *v2ManifestWrapper:
		return w.V1Manifest.R
	case *v2ResourceWrapper:
		return w.V1Resource.R
	case *v2GradleWrapper:
		return w.V1Gradle.R
	case *v2ModuleAwareWrapper:
		return w.V1ModuleAware.R
	case *v2AggregateWrapper:
		return w.V1Aggregate.R
	}
	return nil
}

// V1OfV2 holds the original v1 rule that was wrapped. This is used
// to extract the original rule for operations that need the v1 interface
// (like a rule's SetResolver method).
type V1OfV2 struct {
	V1   Rule
	V2   *v2.Rule
}
