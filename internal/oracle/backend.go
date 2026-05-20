package oracle

import (
	"fmt"
	"strings"
)

// Backend identifies which JVM-side daemon krit invokes for the
// type-oracle role: krit-types (the long-standing KAA-backed daemon)
// or krit-fir (the K2/FIR-backed daemon shipped alongside it).
//
// Both backends speak the same analyze / analyzeAll / analyzeFiles /
// analyzeWithDeps RPCs (see tools/krit-fir/.../OracleResponse.kt for
// parity), so the Go-side oracle client deserializes either backend's
// response with the same struct. Switching between them is a JAR
// resolution decision: which daemon do we spawn?
type Backend string

const (
	// BackendKAA is the krit-types daemon — Kotlin Analysis API
	// backed. The historical default and the only backend before
	// krit-fir landed.
	BackendKAA Backend = "kaa"

	// BackendFIR is the krit-fir daemon — K2/FIR backed. Required
	// for rules that opt into Capability.NEEDS_FIR; usable as a
	// drop-in replacement for KAA today thanks to oracle-protocol
	// parity (see PR 2.x series).
	BackendFIR Backend = "fir"

	// DefaultBackend stays KAA until the FIR backend has shipped a
	// CI parity gate. Flipping the default is a separate change.
	DefaultBackend Backend = BackendKAA
)

// ParseBackend accepts the canonical lower-case spellings plus
// common aliases. Unknown values surface as a typed error so callers
// can render a useful message — `--oracle-backend=foo` shouldn't
// silently fall back to a default.
func ParseBackend(raw string) (Backend, error) {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "":
		return DefaultBackend, nil
	case "kaa", "krit-types", "types":
		return BackendKAA, nil
	case "fir", "krit-fir", "k2":
		return BackendFIR, nil
	default:
		return "", fmt.Errorf("unknown oracle backend %q: want one of kaa, fir", raw)
	}
}

// String renders the canonical wire form for the backend (matching
// the value users set on the CLI / in krit.yml). Useful for error
// messages and verbose logging.
func (b Backend) String() string {
	switch b {
	case BackendKAA, BackendFIR:
		return string(b)
	case "":
		return string(DefaultBackend)
	default:
		return string(b)
	}
}

// JarName returns the canonical jar filename the backend expects on
// disk — `krit-types.jar` for KAA, `krit-fir.jar` for FIR. Centralized
// here so the discovery helper, the auto-download installer, and the
// CI parity job share one source of truth.
func (b Backend) JarName() string {
	switch b {
	case BackendFIR:
		return "krit-fir.jar"
	default:
		return "krit-types.jar"
	}
}
