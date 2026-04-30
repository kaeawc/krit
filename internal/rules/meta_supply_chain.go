// Descriptor metadata for internal/rules/supply_chain.go.

package rules

import (
	"github.com/kaeawc/krit/internal/rules/v2"
)

func (r *CompileSdkMismatchAcrossModulesRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "CompileSdkMismatchAcrossModules",
		RuleSet:       "supply-chain",
		Severity:      "warning",
		Description:   "Detects Android modules whose compileSdk is lower than the maximum compileSdk in the project.",
		DefaultActive: true,
		FixLevel:      "",
		Confidence:    0.75,
	}
}
