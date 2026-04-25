// Descriptor metadata for internal/rules/android_resource_rtl.go.

package rules

import (
	"github.com/kaeawc/krit/internal/rules/registry"
)

func (r *NotSiblingResourceRule) Meta() registry.RuleDescriptor {
	return registry.RuleDescriptor{
		ID:            "NotSiblingResource",
		RuleSet:       "android-lint",
		Severity:      "error",
		Description:   "",
		DefaultActive: true,
		FixLevel:      "",
		Confidence:    0.75,
	}
}

func (r *RelativeOverlapResourceRule) Meta() registry.RuleDescriptor {
	return registry.RuleDescriptor{
		ID:            "RelativeOverlapResource",
		RuleSet:       "android-lint",
		Severity:      "warning",
		Description:   "",
		DefaultActive: true,
		FixLevel:      "",
		Confidence:    0.75,
	}
}

func (r *RtlHardcodedResourceRule) Meta() registry.RuleDescriptor {
	return registry.RuleDescriptor{
		ID:            "RtlHardcodedResource",
		RuleSet:       "android-lint",
		Severity:      "warning",
		Description:   "",
		DefaultActive: true,
		FixLevel:      "",
		Confidence:    0.75,
	}
}

func (r *RtlSuperscriptResourceRule) Meta() registry.RuleDescriptor {
	return registry.RuleDescriptor{
		ID:            "RtlSuperscriptResource",
		RuleSet:       "android-lint",
		Severity:      "warning",
		Description:   "",
		DefaultActive: true,
		FixLevel:      "",
		Confidence:    0.75,
	}
}

func (r *RtlSymmetryResourceRule) Meta() registry.RuleDescriptor {
	return registry.RuleDescriptor{
		ID:            "RtlSymmetryResource",
		RuleSet:       "android-lint",
		Severity:      "warning",
		Description:   "",
		DefaultActive: true,
		FixLevel:      "",
		Confidence:    0.75,
	}
}
