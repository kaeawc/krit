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

func (r *DependencyLicenseIncompatibleRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "DependencyLicenseIncompatible",
		RuleSet:       "licensing",
		Severity:      "warning",
		Description:   "Detects external dependencies whose license is incompatible with the project's declared license.",
		DefaultActive: false,
		FixLevel:      "",
		Confidence:    0.75,
		Options: []v2.ConfigOption{
			{
				Name:        "projectLicense",
				Type:        v2.OptString,
				Default:     "",
				Description: "SPDX license identifier for the project; dependencies with licenses incompatible with this are flagged.",
				Apply: func(target interface{}, value interface{}) {
					target.(*DependencyLicenseIncompatibleRule).ProjectLicense = value.(string)
				},
			},
		},
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

func (r *LgplStaticLinkingInApkRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "LgplStaticLinkingInApk",
		RuleSet:       "licensing",
		Severity:      "warning",
		Description:   "Detects Android application modules that statically link known-LGPL dependencies into the APK.",
		DefaultActive: false,
		FixLevel:      "",
		Confidence:    0.75,
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
