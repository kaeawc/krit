package rules

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

import "github.com/kaeawc/krit/internal/rules/v2"

// Meta returns the descriptor for NewerVersionAvailableRule.
func (r *NewerVersionAvailableRule) Meta() v2.RuleDescriptor {
	return v2.RuleDescriptor{
		ID:            "NewerVersionAvailable",
		RuleSet:       "android-lint",
		Severity:      "warning",
		Description:   "",
		DefaultActive: false,
		FixLevel:      "",
		Confidence:    0.75,
		Options: []v2.ConfigOption{
			{
				Name:        "recommendedVersions",
				Type:        v2.OptStringList,
				Default:     []string(nil),
				Description: "",
				// RecommendedVersions is []libMinVersion on the rule struct;
				// YAML stores a []string of "group:name=version" specs.
				// Parse via parseRecommendedVersionSpecs before assigning
				// the typed rule field.
				Apply: func(target interface{}, value interface{}) {
					target.(*NewerVersionAvailableRule).RecommendedVersions =
						parseRecommendedVersionSpecs(value.([]string))
				},
			},
		},
	}
}
