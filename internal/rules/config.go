package rules

import (
	"github.com/kaeawc/krit/internal/config"
	"github.com/kaeawc/krit/internal/rules/registry"
	v2 "github.com/kaeawc/krit/internal/rules/v2"
)

// ApplyConfig applies YAML configuration to all registered rules.
//
// Semantics are owned by the checked-in rule descriptors:
//   - ruleset-level `active: false` short-circuits and marks every rule in
//     the set inactive regardless of rule-level overrides
//   - rule-level `active: true|false` overrides DefaultInactive
//   - per-rule `excludes` lists a set of file globs (detekt-compatible)
//   - every configured option on the rule's Meta() descriptor is applied
//     to the concrete struct via its descriptor Apply closure
//
// The single source of truth for rule metadata is MetaForRule(), which
// reads Meta() from the registered concrete rule when available and falls
// back to the metaByName index for rules without a concrete config target.
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

	for _, r := range v2.Registry {
		ruleName := r.ID
		ruleSetName := r.Category

		// Per-rule excludes (detekt-compatible file exclusion globs)
		// apply to every rule regardless of whether the rule publishes
		// Meta() — the exclusion map is consulted at dispatch time.
		if excludes := cfg.GetStringList(ruleSetName, ruleName, "excludes"); excludes != nil {
			SetRuleExcludes(ruleName, excludes)
		}

		meta, ok := MetaForV2Rule(r)
		if !ok {
			// Alias registration — primary ID is handled on its own
			// Registry pass. Only honor ruleset/rule active overrides.
			applyAliasActiveOverride(cfg, ruleSetName, ruleName)
			continue
		}

		if meta.ID != ruleName {
			// Alias registration — skip; primary ID handles full config.
			applyAliasActiveOverride(cfg, ruleSetName, ruleName)
			continue
		}

		// Use the registered concrete rule when available so option Apply
		// closures can mutate the live rule fields. Fall back to active-only
		// when no concrete pointer is available.
		concrete := r.Implementation
		if _, hasMeta := concrete.(registry.MetaProvider); hasMeta {
			active := registry.ApplyConfig(concrete, meta, adapter)
			if active {
				delete(DefaultInactive, ruleName)
			} else {
				DefaultInactive[ruleName] = true
			}
		} else {
			active := registry.ApplyConfigActiveOnly(meta, adapter)
			if active {
				delete(DefaultInactive, ruleName)
			} else {
				DefaultInactive[ruleName] = true
			}
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
