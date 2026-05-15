package rules

import api "github.com/kaeawc/krit/internal/rules/api"

// Stability is re-exported from the api package so existing callers
// (`rules.Stability`, `rules.StabilityFrozen`, …) keep compiling.
// New code should reference api.Stability directly.
type Stability = api.Stability

const (
	StabilityUnset    = api.StabilityUnset
	StabilityEvolving = api.StabilityEvolving
	StabilityStable   = api.StabilityStable
	StabilityFrozen   = api.StabilityFrozen
)
