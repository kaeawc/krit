package rules

import (
	api "github.com/kaeawc/krit/internal/rules/api"
)

func registerDeadcodeModuleRules() {

	// --- from deadcode_module.go ---
	{
		r := &ModuleDeadCodeRule{
			BaseRule: BaseRule{RuleName: "ModuleDeadCode", RuleSetName: "dead-code", Sev: "warning", Desc: "Detects dead code with module-boundary awareness, categorizing symbols as truly dead or could-be-internal."},
		}
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: api.Severity(r.Sev),
			Needs: api.NeedsModuleIndex, Confidence: r.Confidence(), Implementation: r,
			Check:         r.check,
			DefaultActive: false,
			OptInReason:   api.OptInReasonExpensive,
		})
	}
}
