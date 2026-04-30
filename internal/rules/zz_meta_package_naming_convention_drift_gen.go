// Descriptor metadata for internal/rules/package_naming_convention_drift.go.

package rules

import (
	"github.com/kaeawc/krit/internal/rules/v2"
)

func (r *PackageNamingConventionDriftRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "PackageNamingConventionDrift",
		RuleSet:       "architecture",
		Severity:      "info",
		Description:   "Detects Kotlin source files whose package declaration does not match the directory path under src/main/kotlin.",
		DefaultActive: false,
		FixLevel:      "",
		Confidence:    0.95,
	}
}
