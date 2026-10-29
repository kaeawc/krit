package oracle

import (
	"fmt"
	"strings"
)

// Backend identifies the JVM-side daemon krit invokes for the type-oracle
// role. krit-fir (K2/FIR) is the only backend; the krit-types (Kotlin
// Analysis API) backend was removed. The type stays so cache scopes, daemon
// registry keys and the `--oracle-backend` / `oracle.backend` setting keep a
// single spelling.
type Backend string

const (
	// BackendFIR is the krit-fir daemon — K2/FIR backed.
	BackendFIR Backend = "fir"

	// DefaultBackend is the only backend.
	DefaultBackend Backend = BackendFIR
)

// ParseBackend accepts the krit-fir spellings. The removed krit-types
// spellings get an error saying so, rather than a silent switch.
func ParseBackend(raw string) (Backend, error) {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "", "fir", "krit-fir", "k2":
		return BackendFIR, nil
	case "kaa", "krit-types", "types":
		return "", fmt.Errorf("oracle backend %q was removed: krit-fir is the only type-oracle backend; drop the setting or use fir", raw)
	default:
		return "", fmt.Errorf("unknown oracle backend %q: want fir", raw)
	}
}

// String renders the canonical wire form for the backend.
func (b Backend) String() string {
	if b == "" {
		return string(DefaultBackend)
	}
	return string(b)
}

// JarName returns the canonical jar filename.
func (b Backend) JarName() string { return "krit-fir.jar" }

// ReturnsWholeCompilation reports whether one analysis run returns fresh facts
// for every file in the compilation, whatever subset was requested. krit-fir
// compiles every source file on each request and reports them all.
func (b Backend) ReturnsWholeCompilation() bool { return true }

// CacheApproximation names how the backend's cache entries were produced.
// Entries any other way (written by the removed krit-types backend) never
// satisfy a lookup. An unset backend returns "", which accepts any entry.
func (b Backend) CacheApproximation() string {
	if b == "" {
		return ""
	}
	return ApproximationFIRWholeCompilation
}
