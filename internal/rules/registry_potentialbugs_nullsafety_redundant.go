package rules

import (
	api "github.com/kaeawc/krit/internal/rules/api"
)

func registerPotentialbugsNullsafetyRedundantRules() {

	// --- from potentialbugs_nullsafety_redundant.go ---
	{
		r := &UnnecessaryNotNullCheckRule{BaseRule: BaseRule{RuleName: "UnnecessaryNotNullCheck", RuleSetName: "potential-bugs", Sev: "warning", Desc: "Detects unnecessary null checks on expressions that are already non-nullable."}}
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: api.Severity(r.Sev),
			NodeTypes: []string{"equality_expression"}, Confidence: api.ConfidenceMedium, Fix: api.FixIdiomatic, Implementation: r,
			Needs:         api.NeedsResolver,
			Check:         r.check,
			ExprPositions: r.ExpressionPositions,
		})
	}
	{
		r := &UnnecessaryNotNullOperatorRule{BaseRule: BaseRule{RuleName: "UnnecessaryNotNullOperator", RuleSetName: "potential-bugs", Sev: "warning", Desc: "Detects the !! operator applied to expressions that are already non-nullable."}}
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: api.Severity(r.Sev),
			NodeTypes: []string{"postfix_expression"}, Confidence: api.ConfidenceMedium, Fix: api.FixIdiomatic,
			Needs:          api.NeedsResolver,
			Implementation: r,
			Check:          r.check,
			ExprPositions:  r.ExpressionPositions,
		})
	}
	{
		r := &UnnecessarySafeCallRule{BaseRule: BaseRule{RuleName: "UnnecessarySafeCall", RuleSetName: "potential-bugs", Sev: "warning", Desc: "Detects the ?. safe-call operator applied to expressions that are already non-nullable."}}
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: api.Severity(r.Sev),
			NodeTypes: []string{"navigation_expression"}, Confidence: api.ConfidenceMedium, Fix: api.FixIdiomatic,
			Needs: api.NeedsResolver, Implementation: r,
			Check:         r.check,
			ExprPositions: r.ExpressionPositions,
		})
	}
	{
		r := &NullCheckOnMutablePropertyRule{BaseRule: BaseRule{RuleName: "NullCheckOnMutableProperty", RuleSetName: "potential-bugs", Sev: "warning", Desc: "Detects null checks on mutable var properties that may be changed by another thread between the check and use."}}
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: api.Severity(r.Sev),
			NodeTypes: []string{"equality_expression"}, Needs: api.NeedsResolver, Implementation: r,
			Check: r.check,
		})
	}
	{
		r := &UselessElvisOnNonNullRule{BaseRule: BaseRule{RuleName: "UselessElvisOnNonNull", RuleSetName: "potential-bugs", Sev: "warning", Desc: "Detects the ?: elvis operator applied to expressions that are already non-nullable, making the fallback dead code."}}
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: api.Severity(r.Sev),
			NodeTypes:              []string{"elvis_expression"},
			Confidence:             0.85,
			Fix:                    api.FixSemantic,
			Needs:                  api.NeedsTypeInfo | api.NeedsOracleExprType,
			OracleDeclarationNeeds: &api.OracleDeclarationProfile{},
			Implementation:         r,
			Check:                  r.check,
			ExprPositions:          r.ExpressionPositions,
		})
	}
	{
		r := &NullableToStringCallRule{BaseRule: BaseRule{RuleName: "NullableToStringCall", RuleSetName: "potential-bugs", Sev: "warning", Desc: "Detects .toString() calls on nullable receivers that may produce the string \"null\"."}}
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: api.Severity(r.Sev),
			NodeTypes: []string{"call_expression", "string_literal"}, Confidence: api.ConfidenceMedium,
			Needs:             api.NeedsTypeInfo | api.NeedsOracleCallTargets,
			Oracle:            &api.OracleFilter{Identifiers: []string{"toString", "$"}},
			Implementation:    r,
			OracleCallTargets: &api.OracleCallTargetFilter{CalleeNames: []string{"toString"}},
			// Resolves the receiver type to check nullability via expressions map.
			OracleDeclarationNeeds: &api.OracleDeclarationProfile{},
			Check:                  r.check,
			ExprPositions:          r.ExpressionPositions,
		})
	}
}
