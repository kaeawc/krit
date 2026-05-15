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

// experimentalRules lists rules whose Maturity is MaturityExperimental.
// Populated by ensureDefaultInactive alongside DefaultInactive so that
// ActiveRulesV2 can re-enable them when --experimental is set without
// flipping deprecated rules at the same time.
var experimentalRules = map[string]bool{}

// deprecatedRules lists rules whose Maturity is MaturityDeprecated. These
// are default-inactive and stay off even when --experimental is set; users
// must name them explicitly via --enable-rules to run them.
var deprecatedRules = map[string]bool{}

// noisyRules lists rules whose effective Noisiness (Rule.Noisiness or
// the value derived from Precision) is NoisinessNoisy. Used by the
// "strict" preset to filter out high-FP rules without disturbing the
// experimental / deprecated lifecycle filters.
var noisyRules = map[string]bool{}

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
			switch r.Maturity {
			case api.MaturityExperimental:
				experimentalRules[r.ID] = true
				DefaultInactive[r.ID] = true
			case api.MaturityDeprecated:
				deprecatedRules[r.ID] = true
				DefaultInactive[r.ID] = true
			}
			if V2RuleNoisiness(r) == NoisinessNoisy {
				noisyRules[r.ID] = true
			}
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

// IsExperimental reports whether a rule's Maturity is MaturityExperimental.
// Returns false for stable, deprecated, or unknown rule IDs.
func IsExperimental(name string) bool {
	ensureDefaultInactive()
	return experimentalRules[name]
}

// IsDeprecated reports whether a rule's Maturity is MaturityDeprecated.
// Returns false for stable, experimental, or unknown rule IDs.
func IsDeprecated(name string) bool {
	ensureDefaultInactive()
	return deprecatedRules[name]
}

// ExpandWithRelated mutates disabledSet in place, adding every rule ID
// that appears in the RelatedRules metadata of any rule already in the
// set. The expansion is non-transitive: only one hop is followed. A
// related ID that is not present in registry is silently skipped — that
// case is already rejected by ValidateRelations at dispatcher
// construction, so a stale entry here means the user passed an unknown
// rule ID via --disable-rules (which is its own diagnostic surface) or
// the registry was filtered after validation.
func ExpandWithRelated(disabledSet map[string]bool, registry []*api.Rule) {
	if len(disabledSet) == 0 {
		return
	}
	// Snapshot the initial set so the one-hop guarantee survives the
	// in-place mutation below — iterating against the live map would
	// promote second-hop relations as new entries are added.
	seed := make(map[string]bool, len(disabledSet))
	for id := range disabledSet {
		seed[id] = true
	}
	for _, r := range registry {
		if r == nil || !seed[r.ID] {
			continue
		}
		for _, related := range r.RelatedRules {
			disabledSet[related] = true
		}
	}
}

// IsNoisy reports whether a rule's effective Noisiness is NoisinessNoisy
// (declared on Rule or derived from Precision). Used by the "strict"
// preset to skip noisy rules; returns false for rules outside the
// registry.
func IsNoisy(name string) bool {
	ensureDefaultInactive()
	return noisyRules[name]
}

// ActiveRulesV2 filters api.Registry using config-driven activation.
//
// A rule is included when it is not in disabledSet AND either:
//   - it is named in enabledSet (explicit user opt-in always wins);
//   - allRules=true (--all-rules);
//   - experimental=true and the rule's Maturity is MaturityExperimental; OR
//   - it is default-active (IsDefaultActive).
//
// Deprecated rules are never included via allRules or experimental — the
// only path that re-enables a deprecated rule is naming it explicitly in
// enabledSet. This keeps deprecated rules from coming back to life when
// users flip broad opt-in flags.
//
// When strict=true, rules whose effective Noisiness is NoisinessNoisy
// are excluded UNLESS the user named them explicitly in enabledSet
// (explicit opt-in always wins). Strict is independent of experimental
// and allRules: --strict --all-rules still drops noisy rules so the
// preset's contract holds.
func ActiveRulesV2(disabledSet, enabledSet map[string]bool, allRules, experimental, strict bool) []*api.Rule {
	ensureDefaultInactive()
	return selectActiveRules(api.Registry, disabledSet, enabledSet, allRules, experimental, strict, experimentalRules, deprecatedRules, noisyRules, DefaultInactive)
}

// selectActiveRules is the registry-agnostic core of ActiveRulesV2,
// extracted so tests can supply a fake registry and fake maturity sets
// without mutating the global api.Registry.
//
// defaultInactive carries the same semantics as the package-level
// DefaultInactive map: presence means the rule is opt-in. noisySet
// holds rules whose effective Noisiness is NoisinessNoisy; when
// strict=true these are excluded unless the user named them in
// enabledSet.
func selectActiveRules(
	reg []*api.Rule,
	disabledSet, enabledSet map[string]bool,
	allRules, experimental, strict bool,
	experimentalSet, deprecatedSet, noisySet, defaultInactive map[string]bool,
) []*api.Rule {
	var out []*api.Rule
	for _, r := range reg {
		if disabledSet[r.ID] {
			continue
		}
		if enabledSet[r.ID] {
			out = append(out, r)
			continue
		}
		if deprecatedSet[r.ID] {
			continue
		}
		if strict && noisySet[r.ID] {
			continue
		}
		if allRules {
			out = append(out, r)
			continue
		}
		if experimental && experimentalSet[r.ID] {
			out = append(out, r)
			continue
		}
		if !defaultInactive[r.ID] {
			out = append(out, r)
		}
	}
	return out
}
