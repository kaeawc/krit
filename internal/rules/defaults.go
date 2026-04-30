package rules

import (
	"sync"

	v2 "github.com/kaeawc/krit/internal/rules/v2"
)

// DefaultInactive lists rules that are inactive by default (opt-in).
// DefaultInactive rules are off by default (opt-in). Users enable them
// via config or --all-rules.
//
// The initial contents are computed from every rule's Meta() descriptor via
// AllMetaProviders(), so this file does not carry a hand-maintained map. Adding
// a new opt-in rule requires setting DefaultActive=false in the descriptor.
//
// Population is lazy (via ensureDefaultInactive) because the initializer
// happens at package-init time, which races against the rule-package
// init() blocks that call v2.Register. A sync.Once+init() hook populates
// the map after all rule init() functions complete.
//
// ApplyConfig mutates this map at runtime to reflect YAML overrides.
var DefaultInactive = map[string]bool{}

var defaultInactiveOnce sync.Once

// ensureDefaultInactive populates DefaultInactive from AllMetaProviders
// plus the alias list. Callers that read DefaultInactive before the rule
// init() functions have run will see an empty map — every rule that
// needs a baseline (ApplyConfig, IsDefaultActive) must call this first.
func ensureDefaultInactive() {
	defaultInactiveOnce.Do(func() {
		// AllMetaProviders() is a pure list of zero-value pointers, so it is
		// safe to call at any init phase.
		for _, p := range AllMetaProviders() {
			m := p.Meta()
			if !m.DefaultActive {
				DefaultInactive[m.ID] = true
			}
		}
		// Alias registrations (primary ID vs. registered name) — Meta()
		// represents the primary only, so the alias needs an explicit
		// entry mirroring the primary's default. All 4 aliases below are
		// paired with primaries that are default-inactive, so the list
		// is unconditional.
		for _, name := range aliasDefaultInactive() {
			DefaultInactive[name] = true
		}
	})
}

// init hook — runs after every rule file's init() because zzz_ prefixes
// the rule-bridge file. computeDefaultInactive (sync.Once) is idempotent.
func init() {
	// Populating DefaultInactive in an init() block keeps behavior
	// consistent with the old literal-map map while letting zzz_v2bridge
	// run first to populate v2.Registry if it hasn't yet.
	ensureDefaultInactive()
}

// aliasDefaultInactive returns the alias-registered rule IDs whose
// default-inactive state must be injected manually (because Meta() uses
// the primary ID).
//
// These pair as follows:
//
//	DynamicVersion              -> GradleDynamicVersion
//	NewerVersionAvailable       -> GradleDependency
//	StringInteger               -> StringShouldBeInt
//	GradlePluginCompatibility   -> GradleCompatible
//
// All four primaries are default-inactive in the inventory, so the
// aliases are listed here unconditionally.
func aliasDefaultInactive() []string {
	return []string{
		"GradleCompatible",
		"GradleDependency",
		"GradleDynamicVersion",
		"StringShouldBeInt",
	}
}

// IsDefaultActive returns whether a rule is active by default.
func IsDefaultActive(name string) bool {
	ensureDefaultInactive()
	return !DefaultInactive[name]
}

// ActiveRulesV2 filters v2.Registry using config-driven activation. Returns
// rules that are enabled, not disabled, and either in enabledSet,
// allRules=true, or IsDefaultActive.
func ActiveRulesV2(disabledSet, enabledSet map[string]bool, allRules bool) []*v2.Rule {
	var out []*v2.Rule
	for _, r := range v2.Registry {
		if disabledSet[r.ID] {
			continue
		}
		if enabledSet[r.ID] || allRules || IsDefaultActive(r.ID) {
			out = append(out, r)
		}
	}
	return out
}
