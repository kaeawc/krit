package rules

// Hand-written Meta() for PublicToInternalLeakyAbstractionRule. The
// generator (krit-gen) excludes this struct via its excludedStructs map,
// so this file is the sole source of truth for the rule's descriptor.
//
// Why hand-written:
//   The YAML option thresholdPercent is an int percent (0-100) but the
//   rule's Threshold field is a float64 fraction (0.0-1.0). The legacy
//   switch in config.go:625-629 performs the transform
//   `rule.Threshold = float64(pct) / 100.0` and only applies it when
//   `pct > 0`. That conditional + transform cannot be expressed by a
//   plain ConfigOption without hand-authoring the Apply closure.

import "github.com/kaeawc/krit/internal/rules/registry"

// Meta returns the descriptor for PublicToInternalLeakyAbstractionRule.
// Mirrors config.go:625-629.
func (r *PublicToInternalLeakyAbstractionRule) Meta() registry.RuleDescriptor {
	return registry.RuleDescriptor{
		ID:            "PublicToInternalLeakyAbstraction",
		RuleSet:       "architecture",
		Severity:      "info",
		Description:   "Flags public classes that are thin wrappers delegating to a single private or internal field, which leak internal abstractions through a nominally public API.",
		DefaultActive: false,
		FixLevel:      "",
		Confidence:    0.7,
		Oracle:        nil,
		Options: []registry.ConfigOption{
			{
				Name:        "thresholdPercent",
				Type:        registry.OptInt,
				Default:     0,
				Description: "",
				// YAML stores an int percent; the rule struct holds a
				// float64 fraction. Only apply when the configured
				// percent is strictly positive — a zero/unset percent
				// leaves the rule's default fraction in place.
				Apply: func(target interface{}, value interface{}) {
					pct, ok := value.(int)
					if !ok || pct <= 0 {
						return
					}
					target.(*PublicToInternalLeakyAbstractionRule).Threshold = float64(pct) / 100.0
				},
			},
		},
	}
}
