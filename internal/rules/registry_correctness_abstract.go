package rules

import (
	api "github.com/kaeawc/krit/internal/rules/api"
)

func registerCorrectnessAbstractRules() {

	// --- from correctness_abstract.go ---
	{
		r := &AbstractMemberNotImplementedRule{BaseRule: BaseRule{RuleName: "AbstractMemberNotImplemented", RuleSetName: "potential-bugs", Sev: "error", Desc: "Detects concrete classes that fail to implement abstract members declared on a supertype."}}
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: api.Severity(r.Sev),
			NodeTypes: []string{"class_declaration"}, Confidence: 0.8, Implementation: r,
			Needs: api.NeedsResolver,
			Tags:  []string{"precompile"},
			Check: r.check,
		})
	}
}
