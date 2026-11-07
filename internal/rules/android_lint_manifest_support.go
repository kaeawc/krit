package rules

import (
	"github.com/kaeawc/krit/internal/manifest"
	"github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
)

// Shared helpers for the Android Lint manifest ports.

// manifestTargetSdk returns the manifest's target SDK, falling back to the owning module's build script, or 0
// when neither declares one.
func manifestTargetSdk(m *manifest.Manifest) int {
	if m.TargetSDK > 0 {
		return m.TargetSDK
	}
	if m.Path == "" {
		return 0
	}
	return lookupManifestGradleInfo(m.Path).targetSdk
}

// manifestMinSdk returns the manifest's minimum SDK, falling back to the owning module's build script, or 1 as
// Lint does when neither declares one.
func manifestMinSdk(m *manifest.Manifest) int {
	if m.MinSDK > 0 {
		return m.MinSDK
	}
	if m.Path != "" {
		if min := lookupManifestGradleInfo(m.Path).minSdk; min > 0 {
			return min
		}
	}
	return 1
}

// manifestIsLibrary reports whether the manifest belongs to a library or test module.
func manifestIsLibrary(m *manifest.Manifest) bool {
	return m.Path != "" && isLibraryOrTestModuleManifest(m.Path)
}

// emitManifestAtValue reports at the value text of an attribute, as Lint's LocationType.VALUE does.
func emitManifestAtValue(ctx *api.Context, a manifest.Attr, msg string) {
	ctx.Emit(scanner.Finding{File: ctx.Manifest.Path, Line: a.Line, Col: a.ValueCol, Message: msg})
}

// emitManifestAtAttr reports at the start of an attribute, name included.
func emitManifestAtAttr(ctx *api.Context, a manifest.Attr, msg string) {
	ctx.Emit(scanner.Finding{File: ctx.Manifest.Path, Line: a.Line, Col: a.Col, Message: msg})
}

// emitManifestAtNode reports at the start tag of an element.
func emitManifestAtNode(ctx *api.Context, n *manifest.Node, msg string) {
	ctx.Emit(scanner.Finding{File: ctx.Manifest.Path, Line: n.Line, Col: n.Col, Message: msg})
}

// registerLintManifestRule registers a Lint port that reads the parsed manifest.
func registerLintManifestRule(rule AndroidRule, impl any, confidence float64, check func(*api.Context)) {
	api.Register(&api.Rule{
		ID: rule.RuleName, Category: rule.RuleSetName, Description: rule.Description(), Sev: api.Severity(rule.Sev),
		Needs: api.NeedsManifest, AndroidDeps: uint32(AndroidDepManifest), Confidence: confidence, Implementation: impl,
		Check: check,
	})
}

// emitManifestAtName reports at the tag name of an element, one column after its `<`, as Lint's getNameLocation does.
func emitManifestAtName(ctx *api.Context, n *manifest.Node, msg string) {
	ctx.Emit(scanner.Finding{File: ctx.Manifest.Path, Line: n.Line, Col: n.Col + 1, Message: msg})
}

// registerLintResourceRule registers a Lint port that reads the parsed resource index.
func registerLintResourceRule(rule AndroidRule, impl interface{ AndroidDependencies() AndroidDataDependency }, confidence float64, check func(*api.Context)) {
	api.Register(&api.Rule{
		ID: rule.RuleName, Category: rule.RuleSetName, Description: rule.Description(), Sev: api.Severity(rule.Sev),
		Needs: api.NeedsResources, AndroidDeps: uint32(impl.AndroidDependencies()), Confidence: confidence, Implementation: impl,
		Check: check,
	})
}
