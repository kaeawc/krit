// Descriptor metadata for internal/rules/android_resource_style.go.

package rules

import (
	"github.com/kaeawc/krit/internal/rules/v2"
)

func (r *AlwaysShowActionResourceRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "AlwaysShowActionResource",
		RuleSet:       "android-lint",
		Severity:      "warning",
		Description:   "",
		DefaultActive: true,
		FixLevel:      "",
		Confidence:    0.75,
	}
}

func (r *DisableBaselineAlignmentResourceRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "DisableBaselineAlignmentResource",
		RuleSet:       "android-lint",
		Severity:      "warning",
		Description:   "",
		DefaultActive: true,
		FixLevel:      "",
		Confidence:    0.75,
	}
}

func (r *InOrMmUsageResourceRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "InOrMmUsageResource",
		RuleSet:       "android-lint",
		Severity:      "warning",
		Description:   "",
		DefaultActive: true,
		FixLevel:      "",
		Confidence:    0.75,
	}
}

func (r *InefficientWeightResourceRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "InefficientWeightResource",
		RuleSet:       "android-lint",
		Severity:      "warning",
		Description:   "",
		DefaultActive: false,
		FixLevel:      "",
		Confidence:    0.75,
	}
}

func (r *MergeRootFrameResourceRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "MergeRootFrameResource",
		RuleSet:       "android-lint",
		Severity:      "warning",
		Description:   "",
		DefaultActive: false,
		FixLevel:      "",
		Confidence:    0.75,
	}
}

func (r *NegativeMarginResourceRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "NegativeMarginResource",
		RuleSet:       "android-lint",
		Severity:      "warning",
		Description:   "",
		DefaultActive: true,
		FixLevel:      "",
		Confidence:    0.75,
		Options: []v2.ConfigOption{
			{
				Name:        "allowedNegativeMargins",
				Type:        v2.OptStringList,
				Default:     []string(nil),
				Description: "Project-approved negative margin patterns. Accepts attr, attr=value, value, ViewType:attr, ViewType:attr=value, or ViewType:*=value.",
				Apply: func(target interface{}, value interface{}) {
					target.(*NegativeMarginResourceRule).AllowedNegativeMargins = value.([]string)
				},
			},
		},
	}
}

func (r *NestedWeightsResourceRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "NestedWeightsResource",
		RuleSet:       "android-lint",
		Severity:      "warning",
		Description:   "",
		DefaultActive: true,
		FixLevel:      "",
		Confidence:    0.75,
	}
}

func (r *ObsoleteLayoutParamsResourceRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "ObsoleteLayoutParamsResource",
		RuleSet:       "android-lint",
		Severity:      "warning",
		Description:   "",
		DefaultActive: false,
		FixLevel:      "",
		Confidence:    0.75,
	}
}

func (r *OverdrawResourceRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "OverdrawResource",
		RuleSet:       "android-lint",
		Severity:      "warning",
		Description:   "",
		DefaultActive: true,
		FixLevel:      "",
		Confidence:    0.75,
	}
}

func (r *PxUsageResourceRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "PxUsageResource",
		RuleSet:       "android-lint",
		Severity:      "warning",
		Description:   "",
		DefaultActive: false,
		FixLevel:      "",
		Confidence:    0.75,
	}
}

func (r *SmallSpResourceRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "SmallSpResource",
		RuleSet:       "android-lint",
		Severity:      "warning",
		Description:   "",
		DefaultActive: true,
		FixLevel:      "",
		Confidence:    0.75,
	}
}

func (r *SpUsageResourceRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "SpUsageResource",
		RuleSet:       "android-lint",
		Severity:      "warning",
		Description:   "",
		DefaultActive: true,
		FixLevel:      "",
		Confidence:    0.75,
	}
}

func (r *StateListReachableResourceRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "StateListReachableResource",
		RuleSet:       "android-lint",
		Severity:      "warning",
		Description:   "",
		DefaultActive: true,
		FixLevel:      "",
		Confidence:    0.75,
	}
}

func (r *Suspicious0dpResourceRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "Suspicious0dpResource",
		RuleSet:       "android-lint",
		Severity:      "warning",
		Description:   "",
		DefaultActive: true,
		FixLevel:      "",
		Confidence:    0,
	}
}
