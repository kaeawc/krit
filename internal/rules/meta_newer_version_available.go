package rules

// Hand-written Meta() for NewerVersionAvailableRule. The generator
// (krit-gen) excludes this struct via its excludedStructs map, so this
// file is the sole source of truth for the rule's descriptor.
//
// Why hand-written:
//   The YAML key recommendedVersions is a []string of specs like
//   "group:name=1.2.3" but the rule's RecommendedVersions field is
//   []libMinVersion — a parsed shape produced by
//   parseRecommendedVersionSpecs. The inventory marks the option as
//   []string (the YAML shape), but the Apply closure needs the value
//   transform. Keeping the transform next to the struct avoids the risk
//   of regenerating and losing the hand-edit previously made in
//   zz_meta_android_gradle_gen.go.

import "github.com/kaeawc/krit/internal/rules/registry"

// Meta returns the descriptor for NewerVersionAvailableRule. The legacy
// case in config.go:617-620 performs the same transform via
// parseRecommendedVersionSpecs; this preserves that behavior.
func (r *NewerVersionAvailableRule) Meta() registry.RuleDescriptor {
	return registry.RuleDescriptor{
		ID:            "NewerVersionAvailable",
		RuleSet:       "android-lint",
		Severity:      "warning",
		Description:   "",
		DefaultActive: false,
		FixLevel:      "",
		Confidence:    0.75,
		Options: []registry.ConfigOption{
			{
				Name:        "recommendedVersions",
				Type:        registry.OptStringList,
				Default:     []string(nil),
				Description: "",
				// RecommendedVersions is []libMinVersion on the rule struct;
				// YAML stores a []string of "group:name=version" specs.
				// Parse via parseRecommendedVersionSpecs to match the legacy
				// switch in config.go:617-620.
				Apply: func(target interface{}, value interface{}) {
					target.(*NewerVersionAvailableRule).RecommendedVersions =
						parseRecommendedVersionSpecs(value.([]string))
				},
			},
		},
	}
}
