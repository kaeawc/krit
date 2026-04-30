package rules

import (
	v2 "github.com/kaeawc/krit/internal/rules/v2"
)

func registerSupplyChainRules() {

	// --- from supply_chain.go ---
	{
		r := &CompileSdkMismatchAcrossModulesRule{BaseRule: BaseRule{RuleName: "CompileSdkMismatchAcrossModules", RuleSetName: supplyChainRuleSet, Sev: "warning", Desc: "Detects Android modules whose compileSdk is lower than the maximum compileSdk in the project."}}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: v2.Severity(r.Sev),
			Needs: v2.NeedsModuleIndex, Confidence: r.Confidence(), Implementation: r,
			Check: r.check,
		})
	}
}
