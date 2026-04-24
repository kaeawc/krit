package rules

import v2 "github.com/kaeawc/krit/internal/rules/v2"

// FixLevel indicates how safe an auto-fix is.
type FixLevel int

const (
	// FixCosmetic: whitespace, formatting, comments only. Cannot change behavior.
	FixCosmetic FixLevel = 1
	// FixIdiomatic: idiomatic transforms producing semantically equivalent code.
	FixIdiomatic FixLevel = 2
	// FixSemantic: correct in most cases but could change edge-case behavior
	// (reflection, serialization, binary compatibility, identity semantics).
	FixSemantic FixLevel = 3
)

func (l FixLevel) String() string {
	switch l {
	case FixCosmetic:
		return "cosmetic"
	case FixIdiomatic:
		return "idiomatic"
	case FixSemantic:
		return "semantic"
	default:
		return "unknown"
	}
}

// ParseFixLevel parses a fix level string.
func ParseFixLevel(s string) (FixLevel, bool) {
	switch s {
	case "cosmetic":
		return FixCosmetic, true
	case "idiomatic":
		return FixIdiomatic, true
	case "semantic":
		return FixSemantic, true
	default:
		return 0, false
	}
}

// GetV2FixLevel returns the fix level encoded on a v2 rule. Returns
// (0, false) when the rule is not fixable.
func GetV2FixLevel(r *v2.Rule) (FixLevel, bool) {
	if r == nil {
		return 0, false
	}
	if r.Fix != v2.FixNone {
		return FixLevel(r.Fix), true
	}
	return 0, false
}
