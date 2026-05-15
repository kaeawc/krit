package rules

import api "github.com/kaeawc/krit/internal/rules/api"

// Noisiness is re-exported from the api package so existing callers
// (`rules.Noisiness`, `rules.NoisinessNoisy`, …) keep compiling.
// New code should reference api.Noisiness directly.
type Noisiness = api.Noisiness

const (
	NoisinessUnset  = api.NoisinessUnset
	NoisinessQuiet  = api.NoisinessQuiet
	NoisinessNormal = api.NoisinessNormal
	NoisinessNoisy  = api.NoisinessNoisy
)

// V2RuleNoisiness returns the declared false-positive tendency for a
// rule.
//
// Resolution order:
//  1. Rule.Noisiness when set (non-zero) — the rule has overridden the
//     derived tier.
//  2. Rule.Implementation when it implements api.NoisinessProvider —
//     lets tests stub a tier without touching the Rule literal.
//  3. Derived from Precision: PrecisionHeuristicTextBacked maps to
//     NoisinessNoisy; every other precision tier maps to
//     NoisinessNormal. V2RuleNoisiness never returns NoisinessUnset.
func V2RuleNoisiness(r *api.Rule) Noisiness {
	if r == nil {
		return NoisinessNormal
	}
	if r.Noisiness != NoisinessUnset {
		return r.Noisiness
	}
	if r.Implementation != nil {
		if np, ok := r.Implementation.(api.NoisinessProvider); ok {
			if n := np.Noisiness(); n != NoisinessUnset {
				return n
			}
		}
	}
	return NoisinessFromPrecision(V2RulePrecision(r))
}

// NoisinessFromPrecision returns the default noisiness derived from a
// precision tier. PrecisionHeuristicTextBacked maps to NoisinessNoisy;
// every other tier (including PrecisionUnset, which V2RulePrecision
// never produces) maps to NoisinessNormal.
func NoisinessFromPrecision(p api.Precision) Noisiness {
	if p == api.PrecisionHeuristicTextBacked {
		return NoisinessNoisy
	}
	return NoisinessNormal
}
