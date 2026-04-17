package rules

import (
	"github.com/kaeawc/krit/internal/config"
	"github.com/kaeawc/krit/internal/rules/registry"
)

// ApplyConfig applies YAML configuration to all registered rules.
//
// Semantics match the legacy switch that used to live here (pre-Phase 3D):
//   - ruleset-level `active: false` short-circuits and marks every rule in
//     the set inactive regardless of rule-level overrides
//   - rule-level `active: true|false` overrides DefaultInactive
//   - per-rule `excludes` lists a set of file globs (detekt-compatible)
//   - every configured option on the rule's Meta() descriptor is applied
//     to the concrete struct via its generated Apply closure
//
// The single source of truth for rule metadata is MetaForRule(), which
// reads Meta() from the rule struct when available (via Unwrap) and
// falls back to the generated metaByName index for adapter-wrapped rules
// that dropped the concrete pointer.
func ApplyConfig(cfg *config.Config) {
	if cfg == nil {
		return
	}

	// Ensure the baseline DefaultInactive map is populated before we
	// mutate it below — some callers (e.g. short-lived test harnesses)
	// might invoke ApplyConfig before the init() hook that normally
	// warms the map.
	ensureDefaultInactive()

	adapter := NewConfigAdapter(cfg)

	for _, r := range Registry {
		ruleSet := r.RuleSet()
		ruleName := r.Name()

		// Per-rule excludes (detekt-compatible file exclusion globs)
		// apply to every rule regardless of whether the rule publishes
		// Meta() — the exclusion map is consulted at dispatch time.
		if excludes := cfg.GetStringList(ruleSet, ruleName, "excludes"); excludes != nil {
			SetRuleExcludes(ruleName, excludes)
		}

		meta, ok := MetaForRule(r)
		if !ok {
			// Alias registration — primary ID is handled on its own
			// Registry pass. Only honor ruleset/rule active overrides.
			applyAliasActiveOverride(cfg, ruleSet, ruleName)
			continue
		}

		// If Unwrap yielded the concrete rule struct (a MetaProvider), use
		// the full ApplyConfig path so option Apply closures can mutate
		// it. Otherwise fall back to ApplyConfigActiveOnly — options
		// cannot target a missing pointer.
		concrete := Unwrap(r)
		if _, hasMeta := concrete.(registry.MetaProvider); !hasMeta {
			active := registry.ApplyConfigActiveOnly(meta, adapter)
			if active {
				delete(DefaultInactive, ruleName)
			} else {
				DefaultInactive[ruleName] = true
			}
			continue
		}

		active := registry.ApplyConfig(concrete, meta, adapter)
		if active {
			delete(DefaultInactive, ruleName)
		} else {
			DefaultInactive[ruleName] = true
		}
	}
}

// applyAliasActiveOverride handles the 4 alias-registered rules whose
// Meta().ID does not match Registry Name(). They don't declare config
// options in their primary descriptor — alias users can only toggle
// active/inactive via config.
func applyAliasActiveOverride(cfg *config.Config, ruleSet, ruleName string) {
	if rsActive := cfg.IsRuleSetActive(ruleSet); rsActive != nil && !*rsActive {
		DefaultInactive[ruleName] = true
		return
	}
	if active := cfg.IsRuleActive(ruleSet, ruleName); active != nil {
		if *active {
			delete(DefaultInactive, ruleName)
		} else {
			DefaultInactive[ruleName] = true
		}
	}
}
