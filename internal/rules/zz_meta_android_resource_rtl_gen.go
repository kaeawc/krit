// Descriptor metadata for internal/rules/android_resource_rtl.go.

package rules

import (
	"github.com/kaeawc/krit/internal/rules/v2"
)

func (r *NotSiblingResourceRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "NotSiblingResource",
		RuleSet:       "android-lint",
		Severity:      "error",
		Description:   "",
		DefaultActive: true,
		FixLevel:      "",
		Confidence:    0.75,
	}
}

func (r *RelativeOverlapResourceRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "RelativeOverlapResource",
		RuleSet:       "android-lint",
		Severity:      "warning",
		Description:   "",
		DefaultActive: true,
		FixLevel:      "",
		Confidence:    0.75,
	}
}

func (r *RtlHardcodedResourceRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "RtlHardcodedResource",
		RuleSet:       "android-lint",
		Severity:      "warning",
		Description:   "",
		DefaultActive: true,
		FixLevel:      "",
		Confidence:    0.75,
	}
}

func (r *RtlSuperscriptResourceRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "RtlSuperscriptResource",
		RuleSet:       "android-lint",
		Severity:      "warning",
		Description:   "",
		DefaultActive: true,
		FixLevel:      "",
		Confidence:    0.75,
	}
}

func (r *RtlSymmetryResourceRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "RtlSymmetryResource",
		RuleSet:       "android-lint",
		Severity:      "warning",
		Description:   "",
		DefaultActive: true,
		FixLevel:      "",
		Confidence:    0.75,
	}
}
