package rules

// zzz_v2bridge.go integrates v2 rules into the v1 dispatcher.
//
// Named with a zzz_ prefix so its init() runs AFTER all other rule
// registration init() functions in the package (Go processes init
// functions in filename-alphabetical order). By that point every v2
// rule has been registered via v2.Register(), and this init() bridges
// them into the v1 Registry automatically.

import (
	"sync"

	v2 "github.com/kaeawc/krit/internal/rules/v2"
)

var registerV2Once sync.Once

func init() {
	RegisterV2Rules()
}

// RegisterV2Rules takes all rules from the v2.Registry, wraps them
// into v1-compatible wrappers, and adds them to the appropriate v1
// registration slot (Registry, ManifestRules, ResourceRules, GradleRules).
// Safe to call multiple times — only the first call takes effect.
func RegisterV2Rules() {
	registerV2Once.Do(func() {
		for _, r := range v2.Registry {
			wrapped := v2.ToV1(r)
			// For rule families that need AndroidDependencyProvider or
			// live outside the main Registry, route into the right slot.
			wrapped = wrapV2ForFamilyInterface(wrapped, r)
			// Route by declared capability (v2.Rule.Needs bitfield) rather
			// than by named interface assertion, so this file doesn't need
			// to reference the family-interface type names.
			switch {
			case r.Needs.Has(v2.NeedsManifest):
				if mr, ok := wrapped.(*v2ManifestWrapper); ok {
					RegisterManifest(mr)
				}
			case r.Needs.Has(v2.NeedsResources):
				if rr, ok := wrapped.(*v2ResourceWrapper); ok {
					RegisterResource(rr)
				}
			case r.Needs.Has(v2.NeedsGradle):
				if gr, ok := wrapped.(*v2GradleWrapper); ok {
					RegisterGradle(gr)
				}
			default:
				if rr, ok := wrapped.(Rule); ok {
					Registry = append(Registry, rr)
				}
			}
		}
	})
}
