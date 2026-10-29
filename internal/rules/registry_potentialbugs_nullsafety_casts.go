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
			NodeTypes: []string{"as_expression"}, Confidence: api.ConfidenceVeryHigh, Fix: api.FixSemantic,
			Needs: api.NeedsTypeInfo |
				api.NeedsOracleCallTargets |
				api.NeedsOracleDiagnostics,
			Oracle:         &api.OracleFilter{Identifiers: []string{" as ", " as?"}},
			Tags:           []string{"precompile"},
			Implementation: r,
			Check:          r.check,
		})
	}
	{
		r := &CastNullableToNonNullableTypeRule{BaseRule: BaseRule{RuleName: "CastNullableToNonNullableType", RuleSetName: "potential-bugs", Sev: "warning", Desc: "Detects casting a nullable expression to a non-nullable type using 'as Type'."}}
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: api.Severity(r.Sev),
			NodeTypes: []string{"as_expression"}, Confidence: api.ConfidenceMedium, Fix: api.FixIdiomatic,
			Needs:          api.NeedsTypeInfo | api.NeedsOracleExprType,
			TypeInfo:       api.TypeInfoHint{PreferBackend: api.PreferAny, Required: true},
			Implementation: r,
			Check:          r.check,
		})
	}
	{
		r := &CastToNullableTypeRule{BaseRule: BaseRule{RuleName: "CastToNullableType", RuleSetName: "potential-bugs", Sev: "warning", Desc: "Detects casts to nullable types like 'as Type?' which always succeed and may hide bugs."}}
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: api.Severity(r.Sev),
			NodeTypes: []string{"as_expression"}, Confidence: api.ConfidenceMedium, Fix: api.FixIdiomatic, Implementation: r,
			Check: r.check,
		})
	}
}
