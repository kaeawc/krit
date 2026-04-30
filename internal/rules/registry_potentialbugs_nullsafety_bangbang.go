package rules

import (
	v2 "github.com/kaeawc/krit/internal/rules/v2"
)

func registerPotentialbugsNullsafetyBangbangRules() {

	// --- from potentialbugs_nullsafety_bangbang.go ---
	{
		r := &UnsafeCallOnNullableTypeRule{BaseRule: BaseRule{RuleName: "UnsafeCallOnNullableType", RuleSetName: "potential-bugs", Sev: "warning", Desc: "Detects usage of the !! not-null assertion operator which may throw NullPointerException."}}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: v2.Severity(r.Sev),
			NodeTypes: []string{"postfix_expression"}, Confidence: 0.85,
			Implementation: r,
			Check:          r.check,
		})
	}
	{
		r := &MapGetWithNotNullAssertionRule{BaseRule: BaseRule{RuleName: "MapGetWithNotNullAssertionOperator", RuleSetName: "potential-bugs", Sev: "warning", Desc: "Detects map[key]!! usage and suggests getValue() or safe alternatives."}}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: v2.Severity(r.Sev),
			NodeTypes: []string{"postfix_expression"}, Confidence: 0.75, Fix: v2.FixSemantic,
			Needs:  v2.NeedsTypeInfo,
			Oracle: &v2.OracleFilter{Identifiers: []string{"!!"}},
			OracleCallTargets: &v2.OracleCallTargetFilter{
				CalleeNames:         []string{"get"},
				LexicalSkipByCallee: map[string][]string{"get": {"*"}},
			},
			// Checks class hierarchy (ClassShell+Supertypes) to verify Map type,
			// and reads member types via mapMemberType() for navigation expressions.
			OracleDeclarationNeeds: &v2.OracleDeclarationProfile{ClassShell: true, Supertypes: true, Members: true},
			Implementation:         r,
			Check:                  r.check,
		})
	}
}
