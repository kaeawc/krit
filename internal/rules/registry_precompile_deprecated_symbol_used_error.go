package rules

import (
	api "github.com/kaeawc/krit/internal/rules/api"
)

func registerPrecompileDeprecatedSymbolUsedErrorRules() {
	r := &PrecompileDeprecatedSymbolUsedErrorRule{BaseRule: BaseRule{
		RuleName:    "K0403-DeprecatedSymbolUsedError",
		RuleSetName: api.CategoryPrecompile,
		Sev:         string(api.SeverityError),
		Desc:        "Detects references to symbols annotated with @Deprecated(level = DeprecationLevel.ERROR). Mirrors kotlinc's DEPRECATION_ERROR.",
	}}
	api.Register(&api.Rule{
		ID:          r.RuleName,
		Category:    r.RuleSetName,
		Description: r.Desc,
		Sev:         api.Severity(r.Sev),
		NodeTypes:   []string{"call_expression", "navigation_expression", "user_type"},
		Confidence:  r.Confidence(),
		Needs:       api.NeedsOracleMembers | api.NeedsOracleMemberAnnotations,
		Oracle: &api.OracleFilter{
			Identifiers: []string{"Deprecated"},
		},
		OracleDeclarationNeeds: &api.OracleDeclarationProfile{
			ClassShell:        true,
			Members:           true,
			MemberAnnotations: true,
		},
		Implementation: r,
		Check:          r.check,
		DefaultActive:  false,
	})
}
