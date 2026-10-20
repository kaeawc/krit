package firchecks

import (
	"github.com/kaeawc/krit/internal/gradlemodel"
)

// CompileTarget is the part of a krit-fir compile context that is not a path:
// the JVM target and the project's effective Kotlin compiler settings. Both
// change what the compiler reports, so both belong in every daemon registry
// key, request and cache fingerprint. The zero value means "the embedded
// compiler's defaults", which is also what a missing Gradle model produces.
type CompileTarget struct {
	JvmTarget string
	Kotlin    gradlemodel.KotlinSettings
	// IncrementalDir is where krit-fir may persist its incremental-checking
	// bundle ({repo}/.krit/fir-incremental), so a new daemon or a one-shot run
	// starts warm. Empty disables both reading and writing (--no-cache, no
	// repo). It does not change what the compiler reports, so it is not part
	// of sameCompileTarget, the daemon registry key or any fingerprint.
	IncrementalDir string
}

// firstTarget returns the optional trailing CompileTarget argument.
func firstTarget(targets []CompileTarget) CompileTarget {
	if len(targets) == 0 {
		return CompileTarget{}
	}
	return targets[0]
}

// kotlinArgs is the settings in the compiler-argument wire form.
func (t CompileTarget) kotlinArgs() []string { return t.Kotlin.CompilerArgs() }

// sameCompileTarget reports whether two targets compile identically: the same
// JVM target and the same semantic Kotlin settings.
func sameCompileTarget(a, b CompileTarget) bool {
	return a.JvmTarget == b.JvmTarget && a.Kotlin.Identity() == b.Kotlin.Identity()
}

// registrySuffix is the daemon registry key component for the target.
func (t CompileTarget) registrySuffix() string {
	key := ""
	if t.JvmTarget != "" {
		key += "-jvm" + t.JvmTarget
	}
	if hash := t.Kotlin.IdentityHash(); hash != "" {
		key += "-kt" + hash
	}
	return key
}
