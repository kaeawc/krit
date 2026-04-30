// Descriptor metadata for internal/rules/android_usability.go.

package rules

import (
	"github.com/kaeawc/krit/internal/rules/v2"
)

func (r *InlinedApiRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "InlinedApi",
		RuleSet:       "android-lint",
		Severity:      "warning",
		Description:   "",
		DefaultActive: false,
		FixLevel:      "",
		Confidence:    0.75,
	}
}

func (r *NewApiRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "NewApi",
		RuleSet:       "android-lint",
		Severity:      "error",
		Description:   "",
		DefaultActive: false,
		FixLevel:      "",
		Confidence:    0.75,
	}
}

func (r *OverrideRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "Override",
		RuleSet:       "android-lint",
		Severity:      "error",
		Description:   "",
		DefaultActive: false,
		FixLevel:      "",
		Confidence:    0.75,
	}
}

func (r *UnusedResourcesRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "UnusedResources",
		RuleSet:       "android-lint",
		Severity:      "warning",
		Description:   "",
		DefaultActive: false,
		FixLevel:      "",
		Confidence:    0.75,
	}
}
