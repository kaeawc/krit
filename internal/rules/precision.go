package rules

// Precision labels the dominant implementation model for a rule.
type Precision string

const (
	PrecisionASTBacked           Precision = "ast-backed"
	PrecisionTypeAware           Precision = "type-aware"
	PrecisionProjectStructure    Precision = "project-structure-aware"
	PrecisionHeuristicTextBacked Precision = "heuristic/text-backed"
	PrecisionPolicy              Precision = "policy"
)

var policyRuleNames = map[string]bool{
	"OldTargetApi":          true,
	"MinSdkTooLow":          true,
	"NewerVersionAvailable": true,
}

var heuristicRuleNames = map[string]bool{
	"ArrayPrimitive":                   true,
	"CouldBeSequence":                  true,
	"GoogleAppIndexingWarningManifest": true,
	"LayoutInflation":                  true,
	"LongMethod":                       true,
	"MagicNumber":                      true,
	"UnsafeCallOnNullableType":         true,
	"UnsafeCast":                       true,
	"ViewTag":                          true,
	"Wakelock":                         true,
}
