package rules

// Hand-written Meta() for LayerDependencyViolationRule. The generator
// (krit-gen) excludes this struct via its excludedStructs map, so this
// file is the sole source of truth for the rule's descriptor.
//
// Why hand-written:
//   Legacy internal/rules/config.go#applyRuleConfig (case
//   *LayerDependencyViolationRule) calls arch.ParseLayerConfig(cfg) — a
//   whole-config-tree read rather than a single YAML key lookup. The
//   generator's ConfigOption shape expresses "read key X, assign field
//   Y"; no single Option can represent "read the entire config, parse
//   nested maps, build a *arch.LayerConfig". We use the CustomApply
//   escape hatch instead.

import (
	"github.com/kaeawc/krit/internal/arch"
	"github.com/kaeawc/krit/internal/rules/registry"
)

// Meta returns the descriptor for LayerDependencyViolationRule. No
// per-option config — the rule pulls its layer matrix out of the whole
// config tree inside CustomApply.
func (r *LayerDependencyViolationRule) Meta() registry.RuleDescriptor {
	return registry.RuleDescriptor{
		ID:            "LayerDependencyViolation",
		RuleSet:       "architecture",
		Severity:      "warning",
		Description:   "Flags Gradle module dependencies that cross architectural layer boundaries not permitted by the configured layer matrix.",
		DefaultActive: false,
		FixLevel:      "",
		Confidence:    0.95,
		Options:       nil,
		CustomApply: func(target interface{}, cfg registry.ConfigSource) {
			// Only a real *ConfigAdapter can expose the underlying
			// *config.Config that arch.ParseLayerConfig needs. Fake
			// config sources (used by unit tests and the parity
			// harness) have no layer data to parse, so we no-op —
			// leaving the rule's existing LayerConfig untouched.
			adapter, ok := cfg.(*ConfigAdapter)
			if !ok {
				return
			}
			rule := target.(*LayerDependencyViolationRule)
			rule.LayerConfig = arch.ParseLayerConfig(adapter.Unwrap())
		},
	}
}
