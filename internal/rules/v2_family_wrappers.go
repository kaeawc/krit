package rules

// v2_family_wrappers.go provides rules-package wrappers that adapt v2.Rule
// values into the v1 family method sets (manifest, resource, gradle,
// module-aware, aggregate). These can't live in the v2 package because
// the method signatures reference types defined in this package
// (*Manifest, AndroidDependencyProvider) — an import cycle.
//
// Each wrapper holds a pointer to the underlying v2 compat wrapper from
// v2/v1compat.go and delegates the Check-family method to it. The
// AndroidDependencyProvider requirement is satisfied by storing an
// AndroidDataDependency value on the wrapper.

import (
	"github.com/kaeawc/krit/internal/android"
	"github.com/kaeawc/krit/internal/module"
	v2 "github.com/kaeawc/krit/internal/rules/v2"
	"github.com/kaeawc/krit/internal/scanner"
)

// v2ManifestWrapper satisfies the v1 manifest-rule method set for a v2.Rule
// that declared NeedsManifest.
type v2ManifestWrapper struct {
	*v2.V1Manifest
	deps AndroidDataDependency
}

func (w *v2ManifestWrapper) CheckManifest(m *Manifest) []scanner.Finding {
	// Delegate to the v2 wrapper's generic-typed method.
	return w.V1Manifest.CheckManifest(m)
}
func (w *v2ManifestWrapper) AndroidDependencies() AndroidDataDependency { return w.deps }

// v2ResourceWrapper satisfies the v1 resource-rule method set.
type v2ResourceWrapper struct {
	*v2.V1Resource
	deps AndroidDataDependency
}

func (w *v2ResourceWrapper) CheckResources(idx *android.ResourceIndex) []scanner.Finding {
	return w.V1Resource.CheckResources(idx)
}
func (w *v2ResourceWrapper) AndroidDependencies() AndroidDataDependency { return w.deps }

// v2GradleWrapper satisfies the v1 gradle-rule method set.
type v2GradleWrapper struct {
	*v2.V1Gradle
	deps AndroidDataDependency
}

func (w *v2GradleWrapper) CheckGradle(path, content string, cfg *android.BuildConfig) []scanner.Finding {
	return w.V1Gradle.CheckGradle(path, content, cfg)
}
func (w *v2GradleWrapper) AndroidDependencies() AndroidDataDependency { return w.deps }

// v2ModuleAwareWrapper satisfies the v1 module-aware rule method set.
type v2ModuleAwareWrapper struct {
	*v2.V1ModuleAware
}

func (w *v2ModuleAwareWrapper) SetModuleIndex(pmi *module.PerModuleIndex) {
	w.V1ModuleAware.SetModuleIndex(pmi)
}
func (w *v2ModuleAwareWrapper) CheckModuleAware() []scanner.Finding {
	return w.V1ModuleAware.CheckModuleAware()
}

// v2AggregateWrapper satisfies the v1 aggregate-rule method set.
type v2AggregateWrapper struct {
	*v2.V1Aggregate
}

// wrapV2ForFamilyInterface takes the ToV1 output from a v2.Rule and,
// if the rule is in a family that needs extra surface (AndroidDependencyProvider),
// wraps it with the matching *v2*Wrapper that satisfies the v1 interface.
// Returns the original wrapper unchanged if it already satisfies a v1 interface
// directly (e.g., V1FlatDispatch, V1Line, V1CrossFile).
func wrapV2ForFamilyInterface(wrapped interface{}, r *v2.Rule) interface{} {
	// Preserve the original rule's Android data dependency where known;
	// fall back to the family constant for rules that didn't declare one.
	deps := AndroidDataDependency(r.AndroidDeps)
	switch w := wrapped.(type) {
	case *v2.V1Manifest:
		if deps == AndroidDepNone {
			deps = AndroidDepManifest
		}
		return &v2ManifestWrapper{V1Manifest: w, deps: deps}
	case *v2.V1Resource:
		if deps == AndroidDepNone {
			deps = AndroidDepResources
		}
		return &v2ResourceWrapper{V1Resource: w, deps: deps}
	case *v2.V1Gradle:
		if deps == AndroidDepNone {
			deps = AndroidDepGradle
		}
		return &v2GradleWrapper{V1Gradle: w, deps: deps}
	case *v2.V1ModuleAware:
		return &v2ModuleAwareWrapper{V1ModuleAware: w}
	case *v2.V1Aggregate:
		return &v2AggregateWrapper{V1Aggregate: w}
	default:
		return wrapped
	}
}
