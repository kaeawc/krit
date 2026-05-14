package rules

import api "github.com/kaeawc/krit/internal/rules/api"

// Hand-written Meta() for NewerVersionAvailableRule. This file is the sole
// source of truth for the rule's descriptor.
//
// Why hand-written:
//   The YAML key recommendedVersions is a []string of specs like
//   "group:name=1.2.3" but the rule's RecommendedVersions field is
//   []libMinVersion — a parsed shape produced by
//   parseRecommendedVersionSpecs. The inventory marks the option as
//   []string (the YAML shape), but the Apply closure needs the value
//   transform. Keeping the transform next to the struct makes the descriptor
//   behavior explicit.

// Meta returns the descriptor for NewerVersionAvailableRule.
func (r *NewerVersionAvailableRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "NewerVersionAvailable",
		RuleSet:       "android-lint",
		DefaultActive: false,
		OptInReason: api.OptInReasonRequiresUserConfig,
		Options: []api.ConfigOption{
			// RecommendedVersions is []libMinVersion on the rule struct;
			// YAML stores a []string of "group:name=version" specs.
			// Parse via parseRecommendedVersionSpecs before assigning
			// the typed rule field.
			api.StringListOption(api.StringListOptionSpec[NewerVersionAvailableRule]{
				Name: "recommendedVersions",
				Apply: func(r *NewerVersionAvailableRule, v []string) {
					r.RecommendedVersions = parseRecommendedVersionSpecs(v)
				},
			}),
		},
	}
}
