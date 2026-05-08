package rules

import api "github.com/kaeawc/krit/internal/rules/api"

// Hand-written Meta() for PublicToInternalLeakyAbstractionRule. This file is
// the sole source of truth for the rule's descriptor.
//
// Why hand-written:
//   The YAML option thresholdPercent is an int percent (0-100) but the
//   rule's Threshold field is a float64 fraction (0.0-1.0). The descriptor
//   applies `rule.Threshold = float64(pct) / 100.0` only when `pct > 0`.
//   That conditional + transform cannot be expressed by a plain ConfigOption
//   without hand-authoring the Apply closure.

// Meta returns the descriptor for PublicToInternalLeakyAbstractionRule.
// Mirrors config.go:625-629.
func (r *PublicToInternalLeakyAbstractionRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "PublicToInternalLeakyAbstraction",
		RuleSet:       "architecture",
		DefaultActive: false,
		Options: []api.ConfigOption{
			// YAML stores an int percent; the rule struct holds a
			// float64 fraction. Only apply when the configured
			// percent is strictly positive — a zero/unset percent
			// leaves the rule's default fraction in place.
			api.IntOption(api.IntOptionSpec[PublicToInternalLeakyAbstractionRule]{
				Name:    "thresholdPercent",
				Default: 0,
				Apply: func(r *PublicToInternalLeakyAbstractionRule, pct int) {
					if pct <= 0 {
						return
					}
					r.Threshold = float64(pct) / 100.0
				},
			}),
		},
	}
}
