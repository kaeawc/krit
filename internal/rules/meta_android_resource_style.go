// Descriptor metadata for internal/rules/android_resource_style.go.

package rules

import (
	api "github.com/kaeawc/krit/internal/rules/api"
)

func (r *AaptCrashRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "AaptCrash",
		RuleSet:       "android-lint",
		DefaultActive: false,
		OptInReason:   api.OptInReasonAndroidOnly,
	}
}

func (r *AlwaysShowActionResourceRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "AlwaysShowActionResource",
		RuleSet:       "android-lint",
		DefaultActive: true,
	}
}

func (r *DisableBaselineAlignmentResourceRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "DisableBaselineAlignmentResource",
		RuleSet:       "android-lint",
		DefaultActive: true,
	}
}

func (r *InOrMmUsageResourceRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "InOrMmUsageResource",
		RuleSet:       "android-lint",
		DefaultActive: true,
	}
}

func (r *InefficientWeightResourceRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "InefficientWeightResource",
		RuleSet:       "android-lint",
		DefaultActive: false,
		OptInReason:   api.OptInReasonAndroidOnly,
	}
}

func (r *MergeRootFrameResourceRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "MergeRootFrameResource",
		RuleSet:       "android-lint",
		DefaultActive: false,
		OptInReason:   api.OptInReasonAndroidOnly,
	}
}

func (r *NegativeMarginResourceRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "NegativeMarginResource",
		RuleSet:       "android-lint",
		DefaultActive: true,
		Options: []api.ConfigOption{
			api.StringListOption(api.StringListOptionSpec[NegativeMarginResourceRule]{
				Name:        "allowedNegativeMargins",
				Description: "Project-approved negative margin patterns. Accepts attr, attr=value, value, ViewType:attr, ViewType:attr=value, or ViewType:*=value.",
				Apply:       func(r *NegativeMarginResourceRule, v []string) { r.AllowedNegativeMargins = v },
			}),
		},
	}
}

func (r *NestedWeightsResourceRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "NestedWeightsResource",
		RuleSet:       "android-lint",
		DefaultActive: true,
	}
}

func (r *ObsoleteLayoutParamsResourceRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "ObsoleteLayoutParamsResource",
		RuleSet:       "android-lint",
		DefaultActive: false,
		OptInReason:   api.OptInReasonAndroidOnly,
	}
}

func (r *OverdrawResourceRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "OverdrawResource",
		RuleSet:       "android-lint",
		DefaultActive: true,
	}
}

func (r *PxUsageResourceRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "PxUsageResource",
		RuleSet:       "android-lint",
		DefaultActive: false,
		OptInReason:   api.OptInReasonAndroidOnly,
	}
}

func (r *SmallSpResourceRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "SmallSpResource",
		RuleSet:       "android-lint",
		DefaultActive: true,
	}
}

func (r *SpUsageResourceRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "SpUsageResource",
		RuleSet:       "android-lint",
		DefaultActive: true,
	}
}

func (r *StateListReachableResourceRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "StateListReachableResource",
		RuleSet:       "android-lint",
		DefaultActive: true,
	}
}

func (r *Suspicious0dpResourceRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "Suspicious0dpResource",
		RuleSet:       "android-lint",
		DefaultActive: true,
	}
}
