package rules

import (
	v2 "github.com/kaeawc/krit/internal/rules/v2"
)

func registerStyleBracesRules() {

	// --- from style_braces.go ---
	{
		r := &BracesOnIfStatementsRule{BaseRule: BaseRule{RuleName: "BracesOnIfStatements", RuleSetName: "style", Sev: "warning", Desc: "Detects if/else statements that are missing braces around their bodies."}}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: v2.Severity(r.Sev),
			NodeTypes: []string{"if_expression"}, Confidence: 0.75, Fix: v2.FixCosmetic, OriginalV1: r,
			Check: r.check,
		})
	}
	{
		r := &BracesOnWhenStatementsRule{BaseRule: BaseRule{RuleName: "BracesOnWhenStatements", RuleSetName: "style", Sev: "warning", Desc: "Detects when branches that are missing braces around their bodies."}}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: v2.Severity(r.Sev),
			NodeTypes: []string{"when_entry"}, Confidence: 0.75, Fix: v2.FixCosmetic, OriginalV1: r,
			Check: r.check,
		})
	}
	{
		r := &MandatoryBracesLoopsRule{BaseRule: BaseRule{RuleName: "MandatoryBracesLoops", RuleSetName: "style", Sev: "warning", Desc: "Detects for, while, and do-while loops that are missing braces around their bodies."}}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: v2.Severity(r.Sev),
			NodeTypes: []string{"for_statement", "while_statement", "do_while_statement"}, Confidence: 0.75, Fix: v2.FixCosmetic, OriginalV1: r,
			Check: r.check,
		})
	}
}
