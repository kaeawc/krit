package rules

import api "github.com/kaeawc/krit/internal/rules/api"

// Precision is re-exported from the api package so existing callers
// (`rules.Precision`, `rules.PrecisionASTBacked`, …) keep compiling.
// New code should reference api.Precision directly.
type Precision = api.Precision

const (
	PrecisionUnset               = api.PrecisionUnset
	PrecisionASTBacked           = api.PrecisionASTBacked
	PrecisionTypeAware           = api.PrecisionTypeAware
	PrecisionProjectStructure    = api.PrecisionProjectStructure
	PrecisionHeuristicTextBacked = api.PrecisionHeuristicTextBacked
	PrecisionPolicy              = api.PrecisionPolicy
)

// policyRuleNames lists rules whose dominant signal is policy
// (SDK version freshness, target-API recency) rather than source
// content. These rules report opinions independent of code shape, so
// the precision derivation tags them as PrecisionPolicy.
var policyRuleNames = map[string]bool{
	"OldTargetApi":          true,
	"MinSdkTooLow":          true,
	"NewerVersionAvailable": true,
}

// heuristicRuleNames lists rules whose dominant signal is lexical /
// text-shaped rather than AST/type-aware. They are the noisiest tier
// and surfaced separately from AST-backed rules.
var heuristicRuleNames = map[string]bool{
	"ArrayPrimitive":                   true,
	"CouldBeSequence":                  true,
	"GoogleAppIndexingWarningManifest": true,
	"LongMethod":                       true,
	"MagicNumber":                      true,
	"UnsafeCallOnNullableType":         true,
	"UnsafeCast":                       true,
	"ViewTag":                          true,
	"Wakelock":                         true,
}
