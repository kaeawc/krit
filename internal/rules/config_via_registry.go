package rules

import (
	"github.com/kaeawc/krit/internal/config"
	"github.com/kaeawc/krit/internal/rules/registry"
)

// RegistryApplyResult is the result of ApplyConfigViaRegistry for a single
// registered rule. The caller reconciles these back against the legacy
// DefaultInactive map during the 3C switch flip.
type RegistryApplyResult struct {
	// Name is the registered rule name (r.Name()).
	Name string

	// MetaID is the ID from the rule's Meta() descriptor, if it implements
	// MetaProvider. Empty when Migrated is false.
	MetaID string

	// Migrated reports whether the rule exposed a Meta() method AND the
	// descriptor ID matched r.Name(). Alias-registered rules (same struct
	// registered under two names) have Meta() but their ID only covers the
	// primary registration; the alias falls through to the legacy path.
	Migrated bool

	// Active is the effective active state after ruleset + rule overrides.
	// Only meaningful when Migrated is true.
	Active bool
}

// ApplyConfigViaRegistry applies cfg to every rule in the global Registry
// using the generated Meta() descriptors. It returns one result per rule in
// registration order so callers can reconcile against the legacy
// DefaultInactive map without scanning the registry twice.
//
// Semantics mirror registry.ApplyConfig:
//
//   - Rules without a Meta() method are reported with Migrated=false and
//     left untouched; the caller should fall back to the legacy switch for
//     them.
//   - Rules whose Meta().ID != r.Name() (alias registrations: the same
//     struct is in Registry twice under a primary name and an alias name)
//     are also reported as Migrated=false. Only the primary registration
//     participates in the registry-driven path; the alias continues to be
//     handled by the legacy switch.
//   - Otherwise the rule's fields are mutated in-place (via the generated
//     Apply closures) and the returned Active reflects the ruleset/rule
//     override chain declared by registry.ApplyConfig.
//
// ApplyConfigViaRegistry does NOT touch DefaultInactive. The caller is
// responsible for merging Active back into the global map when it flips
// the production path over in phase 3C.
func ApplyConfigViaRegistry(cfg *config.Config) []RegistryApplyResult {
	adapter := NewConfigAdapter(cfg)
	results := make([]RegistryApplyResult, 0, len(Registry))

	for _, r := range Registry {
		name := r.Name()
		concrete := Unwrap(r)

		mp, ok := concrete.(registry.MetaProvider)
		if !ok {
			results = append(results, RegistryApplyResult{Name: name, Migrated: false})
			continue
		}

		meta := mp.Meta()
		// Alias registration: the same struct is in Registry under two
		// names. Meta() only represents the primary ID, so we skip the
		// alias so the legacy switch can still handle it.
		if meta.ID != name {
			results = append(results, RegistryApplyResult{
				Name:     name,
				MetaID:   meta.ID,
				Migrated: false,
			})
			continue
		}

		active := registry.ApplyConfig(concrete, meta, adapter)
		results = append(results, RegistryApplyResult{
			Name:     name,
			MetaID:   meta.ID,
			Migrated: true,
			Active:   active,
		})
	}

	return results
}
