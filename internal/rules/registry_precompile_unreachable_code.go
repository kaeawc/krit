package rules

import (
	api "github.com/kaeawc/krit/internal/rules/api"
)

func registerPrecompileUnreachableCodeRules() {
	r := &PrecompileUnreachableCodeRule{BaseRule: BaseRule{
		RuleName:    "K0101-UnreachableCode",
		RuleSetName: api.CategoryPrecompile,
		Sev:         string(api.SeverityError),
		Desc:        "Detects statements that follow an unconditional jump (return, throw, break, continue) in the same block. Mirrors kotlinc's UNREACHABLE_CODE.",
	}}
	api.Register(&api.Rule{
		ID:             r.RuleName,
		Category:       r.RuleSetName,
		Description:    r.Desc,
		Sev:            api.Severity(r.Sev),
		NodeTypes:      []string{"jump_expression"},
		Confidence:     r.Confidence(),
		Implementation: r,
		Check:          r.check,
		DefaultActive:  false,
	})
}
