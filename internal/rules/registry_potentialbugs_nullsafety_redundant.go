package rules

import (
	v2 "github.com/kaeawc/krit/internal/rules/v2"
)

func registerPotentialbugsNullsafetyRedundantRules() {

	// --- from potentialbugs_nullsafety_redundant.go ---
	{
		r := &UnnecessaryNotNullCheckRule{BaseRule: BaseRule{RuleName: "UnnecessaryNotNullCheck", RuleSetName: "potential-bugs", Sev: "warning", Desc: "Detects unnecessary null checks on expressions that are already non-nullable."}}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: v2.Severity(r.Sev),
			NodeTypes: []string{"equality_expression"}, Confidence: 0.75, Fix: v2.FixIdiomatic, OriginalV1: r,
			Needs: v2.NeedsResolver,
			Check: r.check,
		})
	}
	{
		r := &UnnecessaryNotNullOperatorRule{BaseRule: BaseRule{RuleName: "UnnecessaryNotNullOperator", RuleSetName: "potential-bugs", Sev: "warning", Desc: "Detects the !! operator applied to expressions that are already non-nullable."}}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: v2.Severity(r.Sev),
			NodeTypes: []string{"postfix_expression"}, Confidence: 0.75, Fix: v2.FixIdiomatic,
			Needs:      v2.NeedsResolver,
			TypeInfo:   v2.TypeInfoHint{PreferBackend: v2.PreferResolver, Required: true},
			OriginalV1: r,
			Check:      r.check,
		})
	}
	{
		r := &UnnecessarySafeCallRule{BaseRule: BaseRule{RuleName: "UnnecessarySafeCall", RuleSetName: "potential-bugs", Sev: "warning", Desc: "Detects the ?. safe-call operator applied to expressions that are already non-nullable."}}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: v2.Severity(r.Sev),
			NodeTypes: []string{"navigation_expression"}, Confidence: 0.75, Fix: v2.FixIdiomatic,
			Needs: v2.NeedsResolver, OriginalV1: r,
			Check: r.check,
		})
	}
	{
		r := &NullCheckOnMutablePropertyRule{BaseRule: BaseRule{RuleName: "NullCheckOnMutableProperty", RuleSetName: "potential-bugs", Sev: "warning", Desc: "Detects null checks on mutable var properties that may be changed by another thread between the check and use."}}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: v2.Severity(r.Sev),
			NodeTypes: []string{"equality_expression"}, Needs: v2.NeedsResolver, OriginalV1: r,
			Check: r.check,
		})
	}
	{
		r := &NullableToStringCallRule{BaseRule: BaseRule{RuleName: "NullableToStringCall", RuleSetName: "potential-bugs", Sev: "warning", Desc: "Detects .toString() calls on nullable receivers that may produce the string \"null\"."}}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: v2.Severity(r.Sev),
			NodeTypes: []string{"call_expression", "string_literal"}, Confidence: 0.75,
			Needs:             v2.NeedsTypeInfo,
			Oracle:            &v2.OracleFilter{Identifiers: []string{"toString", "$"}},
			OriginalV1:        r,
			OracleCallTargets: &v2.OracleCallTargetFilter{CalleeNames: []string{"toString"}},
			// Resolves the receiver type to check nullability via expressions map.
			OracleDeclarationNeeds: &v2.OracleDeclarationProfile{},
			Check:                  r.check,
		})
	}
}
