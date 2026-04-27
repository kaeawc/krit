package rules

import (
	v2 "github.com/kaeawc/krit/internal/rules/v2"
)

func registerDeadcodeModuleRules() {

	// --- from deadcode_module.go ---
	{
		r := &ModuleDeadCodeRule{
			BaseRule: BaseRule{RuleName: "ModuleDeadCode", RuleSetName: "dead-code", Sev: "warning", Desc: "Detects dead code with module-boundary awareness, categorizing symbols as truly dead or could-be-internal."},
		}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: v2.Severity(r.Sev),
			Needs: v2.NeedsModuleIndex, Confidence: r.Confidence(), OriginalV1: r,
			Check: r.check,
		})
	}
}
