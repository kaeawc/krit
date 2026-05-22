package rules

import (
	api "github.com/kaeawc/krit/internal/rules/api"
)

func registerStyleBracesRules() {

	// --- from style_braces.go ---
	{
		r := &BracesOnIfStatementsRule{BaseRule: BaseRule{RuleName: "BracesOnIfStatements", RuleSetName: "style", Sev: "warning", Desc: "Detects if/else statements that are missing braces around their bodies."}}
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: api.Severity(r.Sev),
			NodeTypes: []string{"if_expression"}, Confidence: api.ConfidenceMedium, Fix: api.FixCosmetic, Implementation: r,
			Check: r.check,
		})
	}
	{
		r := &BracesOnWhenStatementsRule{BaseRule: BaseRule{RuleName: "BracesOnWhenStatements", RuleSetName: "style", Sev: "warning", Desc: "Detects when branches that are missing braces around their bodies."}}
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: api.Severity(r.Sev),
			NodeTypes: []string{"when_entry"}, Confidence: api.ConfidenceMedium, Fix: api.FixCosmetic, Implementation: r,
			Check: r.check,
		})
	}
	{
		r := &MandatoryBracesLoopsRule{BaseRule: BaseRule{RuleName: "MandatoryBracesLoops", RuleSetName: "style", Sev: "warning", Desc: "Detects for, while, and do-while loops that are missing braces around their bodies."}}
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: api.Severity(r.Sev),
			NodeTypes: []string{"for_statement", "while_statement", "do_while_statement"}, Confidence: api.ConfidenceMedium, Fix: api.FixCosmetic, Implementation: r,
			Check: r.check,
		})
	}
}
