// Descriptor metadata for internal/rules/hotspot.go.

package rules

import api "github.com/kaeawc/krit/internal/rules/api"

func (r *FanInFanOutHotspotRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "FanInFanOutHotspot",
		RuleSet:       "architecture",
		DefaultActive: false,
		OptInReason: api.OptInReasonThresholdTuning,
		Options: []api.ConfigOption{
			api.IntOption(api.IntOptionSpec[FanInFanOutHotspotRule]{
				Name:        "allowedFanIn",
				Aliases:     []string{"threshold"},
				Default:     20,
				Description: "Minimum distinct external files referencing a class-like declaration before reporting it as a hotspot.",
				Apply:       func(r *FanInFanOutHotspotRule, v int) { r.AllowedFanIn = v },
			}),
			api.BoolOption(api.BoolOptionSpec[FanInFanOutHotspotRule]{
				Name:        "ignoreCommentReferences",
				Default:     true,
				Description: "Ignore references that only appear inside comments.",
				Apply:       func(r *FanInFanOutHotspotRule, v bool) { r.IgnoreCommentReferences = v },
			}),
		},
	}
}

func (r *GodClassOrModuleRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "GodClassOrModule",
		RuleSet:       "architecture",
		DefaultActive: false,
		OptInReason: api.OptInReasonThresholdTuning,
	}
}
