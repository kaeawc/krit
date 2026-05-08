package rules

import (
	api "github.com/kaeawc/krit/internal/rules/api"
)

func registerDeadcodeRules() {

	// --- from deadcode.go ---
	{
		r := &DeadCodeRule{
			BaseRule:                BaseRule{RuleName: "DeadCode", RuleSetName: "dead-code", Sev: "warning", Desc: "Detects public or internal symbols that are never referenced from any other file."},
			IgnoreCommentReferences: true,
		}
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: api.Severity(r.Sev),
			Needs: api.NeedsCrossFile, Confidence: r.Confidence(), Implementation: r,
			Check: r.check,
		})
	}
}
