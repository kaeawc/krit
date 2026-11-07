package rules

import (
	api "github.com/kaeawc/krit/internal/rules/api"
)

// V2RuleEffort returns the manual-fix difficulty tier for a rule.
//
// Resolution order mirrors V2RulePrecision:
//  1. Rule.Effort when set (non-zero) — author override.
//  2. Rule.Implementation when it implements api.EffortProvider — lets
//     tests stub a tier without touching the Rule literal.
//  3. Derived from rule shape (Needs / NodeTypes / known override maps).
//
// V2RuleEffort never returns EffortUnset, so callers can use the result
// directly for filtering and dashboards.
func V2RuleEffort(r *api.Rule) api.Effort {
	if r == nil {
		return api.EffortLocal
	}
	if r.Effort != api.EffortUnset {
		return r.Effort
	}
	if r.Implementation != nil {
		if ep, ok := r.Implementation.(api.EffortProvider); ok {
			if e := ep.Effort(); e != api.EffortUnset {
				return e
			}
		}
	}

	if e, ok := effortOverrides[r.ID]; ok {
		return e
	}

	// Rules that emit policy / architectural opinions are not fixable
	// by a developer without a design discussion. The same is true for
	// rules driven by Gradle / manifest config — those typically require
	// coordinating with build owners or release captains.
	if V2RulePrecision(r) == api.PrecisionPolicy {
		return api.EffortArchitectural
	}

	// Project-scope rules report defects whose fix can ripple through
	// many files: dead-code symbols, cross-file references, layer or
	// module-graph violations.
	if r.Needs.HasAny(api.NeedsCrossFile |
		api.NeedsModuleIndex |
		api.NeedsParsedFiles) {
		return api.EffortRefactor
	}

	// A resource, manifest or Gradle finding points at one element in one
	// file, so its fix is file-local unless listed in effortOverrides.
	if r.Needs.HasAny(api.NeedsGradle | api.NeedsManifest | api.NeedsResources) {
		return api.EffortLocal
	}

	// Auto-fixable cosmetic rules are by definition one-line edits even
	// when the user fixes them manually.
	if r.Fix == api.FixCosmetic {
		return api.EffortTrivial
	}

	// Line-pass and aggregate rules are file-local by construction:
	// they collect signal during a single-file walk and finalize per
	// file. Default per-file AST rules also stay file-local.
	return api.EffortLocal
}

// ClassifyEffort is the package-level EffortClassifier implementation
// backed by V2RuleEffort. Useful as a default classifier for callers
// that accept the api.EffortClassifier interface.
type ClassifyEffort struct{}

// Classify implements api.EffortClassifier.
func (ClassifyEffort) Classify(r *api.Rule) api.Effort { return V2RuleEffort(r) }

// effortOverrides lists rules whose fix is a single-attribute or single-line
// edit, or whose tier differs from the one derived from the rule shape.
var effortOverrides = map[string]api.Effort{
	// One attribute added, removed or renamed in a layout, values file or manifest.
	"SpUsageResource":                   api.EffortTrivial,
	"SmallSpResource":                   api.EffortTrivial,
	"PxUsageResource":                   api.EffortTrivial,
	"InOrMmUsageResource":               api.EffortTrivial,
	"RtlHardcodedResource":              api.EffortTrivial,
	"RtlSymmetryResource":               api.EffortTrivial,
	"RtlSuperscriptResource":            api.EffortTrivial,
	"Suspicious0dpResource":             api.EffortTrivial,
	"DisableBaselineAlignmentResource":  api.EffortTrivial,
	"NegativeMarginResource":            api.EffortTrivial,
	"OrientationResource":               api.EffortTrivial,
	"WrongCaseResource":                 api.EffortTrivial,
	"ButtonCaseResource":                api.EffortTrivial,
	"TextFieldsResource":                api.EffortTrivial,
	"WebViewLayoutResource":             api.EffortTrivial,
	"TextViewEditsResource":             api.EffortTrivial,
	"InsecureBaseConfigurationResource": api.EffortTrivial,
	"UnusedNamespaceResource":           api.EffortTrivial,
	"UnusedAttributeResource":           api.EffortTrivial,
	"ObsoleteLayoutParamsResource":      api.EffortTrivial,
	"MissingPrefixResource":             api.EffortTrivial,
	"LayoutImportantForAccessibilityNo": api.EffortTrivial,
	"StringTrailingWhitespace":          api.EffortTrivial,
	"AllowBackupManifest":               api.EffortTrivial,
	"RtlEnabledManifest":                api.EffortTrivial,
	"RtlCompatResource":                 api.EffortTrivial,
	"DebuggableManifest":                api.EffortTrivial,
	"CleartextTraffic":                  api.EffortTrivial,
	"MissingExportedFlag":               api.EffortTrivial,
	"ExportedServiceManifest":           api.EffortTrivial,
	"ExportedReceiverManifest":          api.EffortTrivial,
	"ExportedContentProviderManifest":   api.EffortTrivial,
	"ServiceExportedManifest":           api.EffortTrivial,
	"UnpackedNativeCodeManifest":        api.EffortTrivial,
	"MockLocationManifest":              api.EffortTrivial,
	"ManifestOrderManifest":             api.EffortTrivial,
	"ManifestTypoManifest":              api.EffortTrivial,
	"WrongManifestParentManifest":       api.EffortTrivial,
	// One dependency or repository line in a Gradle script.
	"DependencyFromHttp":    api.EffortTrivial,
	"DependencyFromJcenter": api.EffortTrivial,
	"DependencyFromBintray": api.EffortTrivial,
	"MavenLocal":            api.EffortTrivial,
	"GradleDynamicVersion":  api.EffortTrivial,
	"DynamicVersion":        api.EffortTrivial,
	"RemoteVersion":         api.EffortTrivial,
	"GradlePath":            api.EffortTrivial,
	"GradleDependency":      api.EffortTrivial,
	"NewerVersionAvailable": api.EffortTrivial,
	"ApplyPluginTwice":      api.EffortTrivial,
}
