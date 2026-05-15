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

	// Rules that emit policy / architectural opinions are not fixable
	// by a developer without a design discussion. The same is true for
	// rules driven by Gradle / manifest config — those typically require
	// coordinating with build owners or release captains.
	if V2RulePrecision(r) == api.PrecisionPolicy {
		return api.EffortArchitectural
	}

	// Project-scope rules report defects whose fix can ripple through
	// many files: dead-code symbols, cross-file references, layer or
	// module-graph violations, Gradle config drift.
	if r.Needs.HasAny(api.NeedsCrossFile |
		api.NeedsModuleIndex |
		api.NeedsParsedFiles |
		api.NeedsGradle |
		api.NeedsManifest |
		api.NeedsResources) {
		return api.EffortRefactor
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
