package rules

import (
	api "github.com/kaeawc/krit/internal/rules/api"
)

func registerPotentialbugsNullsafetyBangbangRules() {

	// --- from potentialbugs_nullsafety_bangbang.go ---
	{
		r := &UnsafeCallOnNullableTypeRule{
			BaseRule:              BaseRule{RuleName: "UnsafeCallOnNullableType", RuleSetName: "potential-bugs", Sev: "warning", Desc: "Detects usage of the !! not-null assertion operator which may throw NullPointerException."},
			CustomPreviewWildcard: true,
		}
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: api.Severity(r.Sev),
			NodeTypes: []string{"postfix_expression"}, Confidence: api.ConfidenceHigh,
			Implementation: r,
			Check:          r.check,
		})
	}
	{
		r := &MapGetWithNotNullAssertionRule{BaseRule: BaseRule{RuleName: "MapGetWithNotNullAssertionOperator", RuleSetName: "potential-bugs", Sev: "warning", Desc: "Detects map[key]!! usage and suggests getValue() or safe alternatives."}}
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: api.Severity(r.Sev),
			NodeTypes: []string{"postfix_expression"}, Confidence: api.ConfidenceMedium, Fix: api.FixIdiomatic,
			Needs: api.NeedsTypeInfo |
				api.NeedsOracleCallTargets |
				api.NeedsOracleSupertypes |
				api.NeedsOracleMembers,
			Oracle:         &api.OracleFilter{Identifiers: []string{"!!"}},
			Implementation: r,
			Check:          r.check,
		})
	}
}
