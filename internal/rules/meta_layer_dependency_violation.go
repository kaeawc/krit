package rules

// Hand-written Meta() for LayerDependencyViolationRule. This file is the sole
// source of truth for the rule's descriptor.
//
// Why hand-written:
//   LayerDependencyViolationRule calls arch.ParseLayerConfig(cfg) — a
//   whole-config-tree read rather than a single YAML key lookup. The
//   ConfigOption expresses "read key X, assign field Y"; no single Option can
//   represent "read the entire config, parse nested maps, build a
//   *arch.LayerConfig". We use the CustomApply escape hatch instead.

import (
	"github.com/kaeawc/krit/internal/arch"
	api "github.com/kaeawc/krit/internal/rules/api"
)

// Meta returns the descriptor for LayerDependencyViolationRule. No
// per-option config — the rule pulls its layer matrix out of the whole
// config tree inside CustomApply.
func (r *LayerDependencyViolationRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{
		ID:            "LayerDependencyViolation",
		RuleSet:       "architecture",
		DefaultActive: false,
		OptInReason:   api.OptInReasonProjectPolicy,
		Options:       nil,
		CustomApply: api.TypedCustomApply(func(rule *LayerDependencyViolationRule, cfg api.ConfigSource) {
			// Only a real *ConfigAdapter can expose the underlying
			// *config.Config that arch.ParseLayerConfig needs. Fake
			// config sources (used by unit tests and the parity
			// harness) have no layer data to parse, so we no-op —
			// leaving the rule's existing LayerConfig untouched.
			adapter, ok := cfg.(*ConfigAdapter)
			if !ok {
				return
			}
			rule.LayerConfig = arch.ParseLayerConfig(adapter.Unwrap())
		}),
	}
}
