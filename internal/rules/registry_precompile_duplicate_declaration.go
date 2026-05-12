package rules

import (
	api "github.com/kaeawc/krit/internal/rules/api"
)

func registerPrecompileDuplicateDeclarationRules() {
	r := &PrecompileDuplicateDeclarationRule{BaseRule: BaseRule{
		RuleName:    "K0206-DuplicateDeclaration",
		RuleSetName: api.CategoryPrecompile,
		Sev:         string(api.SeverityError),
		Desc:        "Detects two top-level fun/class/val declarations in one file with the same name and (for functions) matching parameter type signature. Mirrors kotlinc's CONFLICTING_OVERLOADS / REDECLARATION for the file-local subset.",
	}}
	api.Register(&api.Rule{
		ID:             r.RuleName,
		Category:       r.RuleSetName,
		Description:    r.Desc,
		Sev:            api.Severity(r.Sev),
		NodeTypes:      []string{"source_file"},
		Confidence:     r.Confidence(),
		Implementation: r,
		Check:          r.check,
		DefaultActive:  false,
	})
}
