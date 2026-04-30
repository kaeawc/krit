// Descriptor metadata for internal/rules/hotspot.go.

package rules

import (
	"github.com/kaeawc/krit/internal/rules/v2"
)

func (r *FanInFanOutHotspotRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "FanInFanOutHotspot",
		RuleSet:       "architecture",
		Severity:      "info",
		Description:   "Detects class-like declarations with unusually high fan-in across the project.",
		DefaultActive: false,
		FixLevel:      "",
		Confidence:    0.75,
		Options: []v2.ConfigOption{
			{
				Name:        "allowedFanIn",
				Aliases:     []string{"threshold"},
				Type:        v2.OptInt,
				Default:     20,
				Description: "Minimum distinct external files referencing a class-like declaration before reporting it as a hotspot.",
				Apply: func(target interface{}, value interface{}) {
					target.(*FanInFanOutHotspotRule).AllowedFanIn = value.(int)
				},
			},
			{
				Name:        "ignoreCommentReferences",
				Type:        v2.OptBool,
				Default:     true,
				Description: "Ignore references that only appear inside comments.",
				Apply: func(target interface{}, value interface{}) {
					target.(*FanInFanOutHotspotRule).IgnoreCommentReferences = value.(bool)
				},
			},
		},
	}
}

func (r *GodClassOrModuleRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "GodClassOrModule",
		RuleSet:       "architecture",
		Severity:      "warning",
		Description:   "Detects source files that import from an unusually broad set of packages, suggesting too many responsibilities.",
		DefaultActive: false,
		FixLevel:      "",
		Confidence:    0.75,
	}
}
