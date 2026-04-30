package rules

import (
	v2 "github.com/kaeawc/krit/internal/rules/v2"
)

func registerDeadcodeRules() {

	// --- from deadcode.go ---
	{
		r := &DeadCodeRule{
			BaseRule:                BaseRule{RuleName: "DeadCode", RuleSetName: "dead-code", Sev: "warning", Desc: "Detects public or internal symbols that are never referenced from any other file."},
			IgnoreCommentReferences: true,
		}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: v2.Severity(r.Sev),
			Needs: v2.NeedsCrossFile, Confidence: r.Confidence(), Implementation: r,
			Check: r.check,
		})
	}
}
