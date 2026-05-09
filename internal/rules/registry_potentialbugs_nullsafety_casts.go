package rules

import (
	api "github.com/kaeawc/krit/internal/rules/api"
)

func registerPotentialbugsNullsafetyCastsRules() {

	// --- from potentialbugs_nullsafety_casts.go ---
	{
		r := &UnsafeCastRule{
			BaseRule:              BaseRule{RuleName: "UnsafeCast", RuleSetName: "potential-bugs", Sev: "warning", Desc: "Detects casts that Kotlin reports can never succeed."},
			CustomPreviewWildcard: true,
		}
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: api.Severity(r.Sev),
			NodeTypes: []string{"as_expression"}, Confidence: 0.95, Fix: api.FixSemantic,
			Needs: api.NeedsTypeInfo |
				api.NeedsOracleCallTargets |
				api.NeedsOracleDiagnostics,
			Oracle: &api.OracleFilter{Identifiers: []string{" as ", " as?"}},
			OracleCallTargets: &api.OracleCallTargetFilter{CalleeNames: []string{
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
			OracleDeclarationNeeds: &api.OracleDeclarationProfile{},
			Tags:                   []string{"precompile"},
			Implementation:         r,
			Check:                  r.check,
		})
	}
	{
		r := &CastNullableToNonNullableTypeRule{BaseRule: BaseRule{RuleName: "CastNullableToNonNullableType", RuleSetName: "potential-bugs", Sev: "warning", Desc: "Detects casting a nullable expression to a non-nullable type using 'as Type'."}}
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: api.Severity(r.Sev),
			NodeTypes: []string{"as_expression"}, Confidence: 0.75, Fix: api.FixSemantic,
			Needs:          api.NeedsResolver,
			TypeInfo:       api.TypeInfoHint{PreferBackend: api.PreferResolver, Required: true},
			Implementation: r,
			Check:          r.check,
		})
	}
	{
		r := &CastToNullableTypeRule{BaseRule: BaseRule{RuleName: "CastToNullableType", RuleSetName: "potential-bugs", Sev: "warning", Desc: "Detects casts to nullable types like 'as Type?' which always succeed and may hide bugs."}}
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: api.Severity(r.Sev),
			NodeTypes: []string{"as_expression"}, Confidence: 0.75, Fix: api.FixSemantic, Implementation: r,
			Check: r.check,
		})
	}
}
