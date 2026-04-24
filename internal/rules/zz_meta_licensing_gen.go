// Descriptor metadata for internal/rules/licensing.go.

package rules

import (
	"github.com/kaeawc/krit/internal/rules/registry"
)

func (r *CopyrightYearOutdatedRule) Meta() registry.RuleDescriptor {
	return registry.RuleDescriptor{
		ID:            "CopyrightYearOutdated",
		RuleSet:       "licensing",
		Severity:      "info",
		Description:   "Detects stale copyright years in file header comments.",
		DefaultActive: false,
		FixLevel:      "",
		Confidence:    0.75,
	}
}

func (r *DependencyLicenseUnknownRule) Meta() registry.RuleDescriptor {
	return registry.RuleDescriptor{
		ID:            "DependencyLicenseUnknown",
		RuleSet:       "licensing",
		Severity:      "info",
		Description:   "Detects external dependencies not present in the embedded license registry.",
		DefaultActive: false,
		FixLevel:      "",
		Confidence:    0.75,
		Options: []registry.ConfigOption{
			{
				Name:        "requireVerification",
				Type:        registry.OptBool,
				Default:     false,
				Description: "Require external dependencies to exist in the embedded license registry.",
				Apply: func(target interface{}, value interface{}) {
					target.(*DependencyLicenseUnknownRule).RequireVerification = value.(bool)
				},
			},
		},
	}
}

func (r *MissingSpdxIdentifierRule) Meta() registry.RuleDescriptor {
	return registry.RuleDescriptor{
		ID:            "MissingSpdxIdentifier",
		RuleSet:       "licensing",
		Severity:      "info",
		Description:   "Detects file header comments that are missing a SPDX license identifier.",
		DefaultActive: false,
		FixLevel:      "",
		Confidence:    0.75,
	}
}
