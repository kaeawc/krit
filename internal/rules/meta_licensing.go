// Descriptor metadata for internal/rules/licensing.go.

package rules

import (
	"github.com/kaeawc/krit/internal/rules/v2"
)

func (r *CopyrightYearOutdatedRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "CopyrightYearOutdated",
		RuleSet:       "licensing",
		Severity:      "info",
		Description:   "Detects stale copyright years in file header comments.",
		DefaultActive: false,
		FixLevel:      "",
		Confidence:    0.75,
	}
}

func (r *DependencyLicenseUnknownRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "DependencyLicenseUnknown",
		RuleSet:       "licensing",
		Severity:      "info",
		Description:   "Detects external dependencies not present in the embedded license v2.",
		DefaultActive: false,
		FixLevel:      "",
		Confidence:    0.75,
		Options: []v2.ConfigOption{
			{
				Name:        "requireVerification",
				Type:        v2.OptBool,
				Default:     false,
				Description: "Require external dependencies to exist in the embedded license v2.",
				Apply: func(target interface{}, value interface{}) {
					target.(*DependencyLicenseUnknownRule).RequireVerification = value.(bool)
				},
			},
		},
	}
}

func (r *MissingSpdxIdentifierRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "MissingSpdxIdentifier",
		RuleSet:       "licensing",
		Severity:      "info",
		Description:   "Detects file header comments that are missing a SPDX license identifier.",
		DefaultActive: false,
		FixLevel:      "",
		Confidence:    0.75,
	}
}
