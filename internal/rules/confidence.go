package rules

// Confidence tiers classify rule detection precision.
//
// Tier 3 (high): structural checks where the parsed source or resource shape
// leaves little room for false positives.
//
// Tier 2 (medium): heuristic checks, including manifest attribute/resource
// checks and pattern matching that can occasionally depend on project context.
//
// Tier 1 (low): rules that need type information or broader cross-file context
// and may produce false positives when that context is incomplete.
const (
	ConfidenceHigh   = 0.95
	ConfidenceMedium = 0.75
	ConfidenceLow    = 0.60
)
