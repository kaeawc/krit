// Descriptor metadata for internal/rules/android_usability.go.

package rules

import api "github.com/kaeawc/krit/internal/rules/api"

func (r *InlinedAPIRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "InlinedApi",
		RuleSet:       "android-lint",
		DefaultActive: false,
	}
}

func (r *NewAPIRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "NewApi",
		RuleSet:       "android-lint",
		DefaultActive: false,
	}
}

func (r *OverrideRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "Override",
		RuleSet:       "android-lint",
		DefaultActive: false,
	}
}

func (r *RequiresApiViolationRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "RequiresApiViolation",
		RuleSet:       "android-lint",
		DefaultActive: false,
	}
}

func (r *UnusedResourcesRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "UnusedResources",
		RuleSet:       "android-lint",
		DefaultActive: false,
	}
}
