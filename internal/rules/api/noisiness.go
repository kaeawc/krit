package api

// Noisiness is a rule's declared false-positive tendency: a coarse
// signal-to-noise label users can filter on without inspecting the
// underlying evidence tier. Most rules are NoisinessNormal — declared
// only when an author has direct evidence the rule reliably (or
// noisily) fires on real code.
//
// Values are ordered cleanest to noisiest so callers can filter with
// comparisons: "strict" gating runs rules with Noisiness <= NoisinessNormal
// and excludes NoisinessNoisy entries. PrecisionUnset (zero) means the
// rule has not declared a tier and consumers should derive one — the
// V2RuleNoisiness helper does this from Precision (heuristic/text-backed
// rules default to NoisinessNoisy; every other tier defaults to
// NoisinessNormal).
type Noisiness uint8

const (
	// NoisinessUnset is the zero value. Resolve via V2RuleNoisiness.
	NoisinessUnset Noisiness = iota
	// NoisinessQuiet marks rules with near-zero observed false-positive
	// rate (e.g. compiler-mirror rules, FQN-resolved type-aware checks).
	NoisinessQuiet
	// NoisinessNormal is the default tier — typical real-world signal.
	NoisinessNormal
	// NoisinessNoisy marks rules known to produce false positives on
	// real code. Excluded by the "strict" preset; users who keep them on
	// should expect to review findings or pair the rule with a baseline.
	NoisinessNoisy
)

// String returns the stable, lowercase label for the noisiness tier.
// Used by CLI output, MCP responses, SARIF properties, and config docs.
func (n Noisiness) String() string {
	switch n {
	case NoisinessQuiet:
		return "quiet"
	case NoisinessNormal:
		return "normal"
	case NoisinessNoisy:
		return "noisy"
	default:
		return "unset"
	}
}

// ParseNoisiness returns the Noisiness matching the given label. Labels
// accepted are exactly the canonical strings emitted by String(), so the
// MCP schema enum and the parser stay in lockstep. Unknown labels (and
// the empty string) return NoisinessUnset, false.
func ParseNoisiness(s string) (Noisiness, bool) {
	switch s {
	case "quiet":
		return NoisinessQuiet, true
	case "normal":
		return NoisinessNormal, true
	case "noisy":
		return NoisinessNoisy, true
	default:
		return NoisinessUnset, false
	}
}

// NoisinessProvider lets tests stub a noisiness value without
// constructing a full Rule. The dispatcher and MetaForRule check for
// this interface on Rule.Implementation before falling back to the
// derived value.
type NoisinessProvider interface {
	Noisiness() Noisiness
}
