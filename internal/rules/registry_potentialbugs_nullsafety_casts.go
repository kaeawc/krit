package rules

import (
	v2 "github.com/kaeawc/krit/internal/rules/v2"
)

func registerPotentialbugsNullsafetyCastsRules() {

	// --- from potentialbugs_nullsafety_casts.go ---
	{
		r := &UnsafeCastRule{BaseRule: BaseRule{RuleName: "UnsafeCast", RuleSetName: "potential-bugs", Sev: "warning", Desc: "Detects casts that Kotlin reports can never succeed."}}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: v2.Severity(r.Sev),
			NodeTypes: []string{"as_expression"}, Confidence: 0.95, Fix: v2.FixSemantic,
			Needs:  v2.NeedsTypeInfo,
			Oracle: &v2.OracleFilter{Identifiers: []string{" as ", " as?"}},
			OracleCallTargets: &v2.OracleCallTargetFilter{CalleeNames: []string{
				"findFragmentById",
				"findFragmentByTag",
				"findViewById",
				"getSystemService",
				"requireViewById",
			}, LexicalSkipByCallee: map[string][]string{
				"findFragmentById":  {"fragmentManager", "supportFragmentManager", "childFragmentManager", "parentFragmentManager", "FragmentManager"},
				"findFragmentByTag": {"fragmentManager", "supportFragmentManager", "childFragmentManager", "parentFragmentManager", "FragmentManager"},
				"findViewById":      {"view", "root", "itemView", "activity", "dialog", "window", "View"},
				"getSystemService":  {"context", "applicationContext", "requireContext", "activity", "Context"},
				"requireViewById":   {"view", "root", "itemView", "activity", "dialog", "window", "View"},
			}},
			// Consumes compiler CAST_NEVER_SUCCEEDS diagnostics and uses expression
			// types for the conservative local fallback; no declarations traversal
			// needed.
			OracleDeclarationNeeds: &v2.OracleDeclarationProfile{},
			OriginalV1:             r,
			Check:                  r.check,
		})
	}
	{
		r := &CastNullableToNonNullableTypeRule{BaseRule: BaseRule{RuleName: "CastNullableToNonNullableType", RuleSetName: "potential-bugs", Sev: "warning", Desc: "Detects casting a nullable expression to a non-nullable type using 'as Type'."}}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: v2.Severity(r.Sev),
			NodeTypes: []string{"as_expression"}, Confidence: 0.75, Fix: v2.FixSemantic,
			Needs:  v2.NeedsTypeInfo,
			Oracle: &v2.OracleFilter{Identifiers: []string{" as "}},
			// Uses expression-type resolution; no class declarations needed.
			OracleDeclarationNeeds: &v2.OracleDeclarationProfile{},
			OriginalV1:             r,
			Check:                  r.check,
		})
	}
	{
		r := &CastToNullableTypeRule{BaseRule: BaseRule{RuleName: "CastToNullableType", RuleSetName: "potential-bugs", Sev: "warning", Desc: "Detects casts to nullable types like 'as Type?' which always succeed and may hide bugs."}}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: v2.Severity(r.Sev),
			NodeTypes: []string{"as_expression"}, Confidence: 0.75, Fix: v2.FixSemantic, OriginalV1: r,
			Check: r.check,
		})
	}
}
