package api

// Stability is a rule's *output-shape* commitment, distinct from Maturity
// (lifecycle). It tells consumers of krit JSON, SARIF, baseline files,
// dashboards, and CI gates whether the rule's message text, finding
// location, or fix range is safe to pin against between releases.
//
// Values are ordered from least committed (Evolving) to most committed
// (Frozen) so callers can filter with comparisons, e.g.
// "warn for any baselined finding from a rule with Stability < StabilityStable".
// The zero value (StabilityUnset) means the rule has not declared a
// stability tier.
//
// Downgrades from StabilityFrozen → StabilityEvolving require a major
// version bump. Promotions are always allowed.
type Stability uint8

const (
	// StabilityUnset is the zero value. The rule has not declared a
	// stability commitment; consumers should treat this conservatively
	// (assume Evolving).
	StabilityUnset Stability = iota
	// StabilityEvolving means the rule's message text, finding location,
	// or fix range may change between minor versions. Consumers pinning
	// findings (baseline files, CI gates) should expect churn.
	StabilityEvolving
	// StabilityStable means the rule accepts only bug-fix changes to its
	// output shape. Message wording or fix-range tweaks land only when
	// they correct a real defect.
	StabilityStable
	// StabilityFrozen means the rule's message text, finding location,
	// and fix range will not change. Suitable for long-lived baselines
	// and external CI gates.
	StabilityFrozen
)

// String returns the stable kebab-case label for the stability tier.
// The labels appear in MCP responses, baseline-audit output, and SARIF
// properties; treat them as a wire format.
func (s Stability) String() string {
	switch s {
	case StabilityEvolving:
		return "evolving"
	case StabilityStable:
		return "stable"
	case StabilityFrozen:
		return "frozen"
	default:
		return "unset"
	}
}

// ParseStability returns the Stability matching the given label. The
// labels accepted are exactly the canonical strings emitted by String().
// Unknown labels (and the empty string) return StabilityUnset, false.
func ParseStability(s string) (Stability, bool) {
	switch s {
	case "evolving":
		return StabilityEvolving, true
	case "stable":
		return StabilityStable, true
	case "frozen":
		return StabilityFrozen, true
	default:
		return StabilityUnset, false
	}
}

// StabilityProvider lets tests stub a stability value without
// constructing a full Rule. MetaForRule checks for this interface on
// Rule.Implementation before falling back to the rule's declared field.
type StabilityProvider interface {
	Stability() Stability
}
