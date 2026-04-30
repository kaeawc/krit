package rules

import (
	"github.com/kaeawc/krit/internal/config"
	"github.com/kaeawc/krit/internal/rules/registry"
	v2 "github.com/kaeawc/krit/internal/rules/v2"
)

// RegistryApplyResult is the result of ApplyConfigViaRegistry for a single
// registered rule.
type RegistryApplyResult struct {
	// Name is the registered rule name (r.Name()).
	Name string

	// MetaID is the ID from the rule's Meta() descriptor, if it implements
	// MetaProvider. Empty when Applied is false.
	MetaID string

	// Applied reports whether the rule exposed a Meta() method AND the
	// descriptor ID matched r.Name(). Alias-registered rules (same struct
	// registered under two names) have Meta() but their ID only covers the
	// primary registration.
	Applied bool

	// Active is the effective active state after ruleset + rule overrides.
	// Only meaningful when Applied is true.
	Active bool
}

// ApplyConfigViaRegistry applies cfg to every rule in the global Registry
// using checked-in Meta() descriptors. It returns one result per rule in
// registration order.
//
// Semantics mirror registry.ApplyConfig:
//
//   - Rules without a Meta() method are reported with Applied=false and
//     left untouched.
//   - Rules whose Meta().ID != r.Name() (alias registrations: the same
//     struct is in Registry twice under a primary name and an alias name)
//     are also reported as Applied=false. Only the primary registration
//     participates in the registry-driven path.
//   - Otherwise the rule's fields are mutated in-place (via descriptor Apply
//     closures) and the returned Active reflects the ruleset/rule
//     override chain declared by registry.ApplyConfig.
//
// ApplyConfigViaRegistry does NOT touch DefaultInactive.
func ApplyConfigViaRegistry(cfg *config.Config) []RegistryApplyResult {
	adapter := NewConfigAdapter(cfg)
	results := make([]RegistryApplyResult, 0, len(v2.Registry))

	for _, r := range v2.Registry {
		name := r.ID
		concrete := r.Implementation

		mp, ok := concrete.(registry.MetaProvider)
		if !ok {
			results = append(results, RegistryApplyResult{Name: name, Applied: false})
			continue
		}

		meta := mp.Meta()
		// Alias registration: the same struct is in Registry under two
		// names. Meta() only represents the primary ID, so we skip the alias.
		if meta.ID != name {
			results = append(results, RegistryApplyResult{
				Name:    name,
				MetaID:  meta.ID,
				Applied: false,
			})
			continue
		}

		active := registry.ApplyConfig(concrete, meta, adapter)
		results = append(results, RegistryApplyResult{
			Name:    name,
			MetaID:  meta.ID,
			Applied: true,
			Active:  active,
		})
	}

	return results
}
