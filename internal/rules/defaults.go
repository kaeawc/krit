package rules

import (
	"sync"

	api "github.com/kaeawc/krit/internal/rules/api"
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
// init() blocks that call api.Register. A sync.Once+init() hook populates
// the map after all rule init() functions complete.
//
// ApplyConfig mutates this map at runtime to reflect YAML overrides.
var DefaultInactive = map[string]bool{}

var defaultInactiveOnce sync.Once

// ensureDefaultInactive populates DefaultInactive from every registered
// rule's effective descriptor (preferring api.Rule fields, falling back
// to MetaProvider) plus the alias list. Callers that read DefaultInactive
// before the rule init() functions have run will see an empty map —
// every rule that needs a baseline (ApplyConfig, IsDefaultActive) must
// call this first.
func ensureDefaultInactive() {
	defaultInactiveOnce.Do(func() {
		// Walk the api.Registry: rules migrated to inline descriptor
		// fields contribute via r.DefaultActive, rules that still
		// implement MetaProvider contribute via the merge in
		// MetaForRule. AllMetaProviders is consulted for any rule
		// not present in the registry.
		seen := make(map[string]struct{}, len(api.Registry))
		for _, r := range api.Registry {
			seen[r.ID] = struct{}{}
			desc, ok := MetaForRule(r)
			if !ok {
				continue
			}
			if !desc.DefaultActive {
				DefaultInactive[desc.ID] = true
			}
		}
		// Cover any rule whose Meta() is published via AllMetaProviders
		// but which has not been seen in the registry walk (e.g. a rule
		// registered under an alias whose canonical ID still lives in
		// the meta index).
		for _, p := range AllMetaProviders() {
			m := p.Meta()
			if _, alreadySeen := seen[m.ID]; alreadySeen {
				continue
			}
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

// Population of DefaultInactive is purely lazy via ensureDefaultInactive:
// every reader (IsDefaultActive, ApplyConfig) calls it on first use, and
// sync.Once makes it idempotent. We deliberately do NOT call
// ensureDefaultInactive() from a package-level init() here — defaults.go
// sorts alphabetically before registry_bootstrap.go, so an init() in this
// file would run before api.Registry is populated and capture a stale
// (empty) snapshot under sync.Once.

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

// ActiveRulesV2 filters api.Registry using config-driven activation. Returns
// rules that are enabled, not disabled, and either in enabledSet,
// allRules=true, or IsDefaultActive.
func ActiveRulesV2(disabledSet, enabledSet map[string]bool, allRules bool) []*api.Rule {
	var out []*api.Rule
	for _, r := range api.Registry {
		if disabledSet[r.ID] {
			continue
		}
		if enabledSet[r.ID] || allRules || IsDefaultActive(r.ID) {
			out = append(out, r)
		}
	}
	return out
}
