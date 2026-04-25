// Descriptor metadata for internal/rules/android_usability.go.

package rules

import (
	"github.com/kaeawc/krit/internal/rules/registry"
)

func (r *InlinedApiRule) Meta() registry.RuleDescriptor {
	return registry.RuleDescriptor{
		ID:            "InlinedApi",
		RuleSet:       "android-lint",
		Severity:      "warning",
		Description:   "",
		DefaultActive: false,
		FixLevel:      "",
		Confidence:    0.75,
	}
}

func (r *NewApiRule) Meta() registry.RuleDescriptor {
	return registry.RuleDescriptor{
		ID:            "NewApi",
		RuleSet:       "android-lint",
		Severity:      "error",
		Description:   "",
		DefaultActive: false,
		FixLevel:      "",
		Confidence:    0.75,
	}
}

func (r *OverrideRule) Meta() registry.RuleDescriptor {
	return registry.RuleDescriptor{
		ID:            "Override",
		RuleSet:       "android-lint",
		Severity:      "error",
		Description:   "",
		DefaultActive: false,
		FixLevel:      "",
		Confidence:    0.75,
	}
}

func (r *UnusedResourcesRule) Meta() registry.RuleDescriptor {
	return registry.RuleDescriptor{
		ID:            "UnusedResources",
		RuleSet:       "android-lint",
		Severity:      "warning",
		Description:   "",
		DefaultActive: false,
		FixLevel:      "",
		Confidence:    0.75,
	}
}
