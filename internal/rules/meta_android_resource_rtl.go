// Descriptor metadata for internal/rules/android_resource_rtl.go.

package rules

import api "github.com/kaeawc/krit/internal/rules/api"

func (r *NotSiblingResourceRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "NotSiblingResource",
		RuleSet:       "android-lint",
		DefaultActive: true,
	}
}

func (r *RelativeOverlapResourceRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "RelativeOverlapResource",
		RuleSet:       "android-lint",
		DefaultActive: true,
	}
}

func (r *RtlHardcodedResourceRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "RtlHardcodedResource",
		RuleSet:       "android-lint",
		DefaultActive: true,
	}
}

func (r *RtlSuperscriptResourceRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "RtlSuperscriptResource",
		RuleSet:       "android-lint",
		DefaultActive: true,
	}
}

func (r *RtlSymmetryResourceRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "RtlSymmetryResource",
		RuleSet:       "android-lint",
		DefaultActive: true,
	}
}
