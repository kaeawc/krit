package rules

import (
	"github.com/kaeawc/krit/internal/android"
	"github.com/kaeawc/krit/internal/scanner"
	"github.com/kaeawc/krit/internal/typeinfer"
)

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
	"MissingPermission":                true,
	"UnsafeCallOnNullableType":         true,
	"UnsafeCast":                       true,
	"ViewTag":                          true,
	"Wakelock":                         true,
}

// RulePrecision returns the dominant precision class for a rule. The goal is
// to expose whether a rule is primarily syntax/AST based, type-aware,
// project-structure aware, heuristic/text-backed, or policy-driven.
func RulePrecision(r Rule) Precision {
	if r == nil {
		return PrecisionHeuristicTextBacked
	}
	if policyRuleNames[r.Name()] {
		return PrecisionPolicy
	}
	if heuristicRuleNames[r.Name()] {
		return PrecisionHeuristicTextBacked
	}
	if _, ok := r.(interface {
		CheckManifest(m *Manifest) []scanner.Finding
	}); ok {
		return PrecisionProjectStructure
	}
	if _, ok := r.(interface {
		CheckGradle(path string, content string, cfg *android.BuildConfig) []scanner.Finding
	}); ok {
		return PrecisionProjectStructure
	}
	if _, ok := r.(interface {
		CheckParsedFiles(files []*scanner.File) []scanner.Finding
	}); ok {
		return PrecisionProjectStructure
	}
	if _, ok := r.(interface {
		CheckCrossFile(index *scanner.CodeIndex) []scanner.Finding
	}); ok {
		return PrecisionProjectStructure
	}
	if _, ok := r.(interface {
		CheckModuleAware() []scanner.Finding
	}); ok {
		return PrecisionProjectStructure
	}
	if _, ok := r.(interface {
		SetResolver(resolver typeinfer.TypeResolver)
	}); ok {
		return PrecisionTypeAware
	}
	if _, ok := r.(interface {
		NodeTypes() []string
		CheckFlatNode(idx uint32, file *scanner.File) []scanner.Finding
	}); ok {
		return PrecisionASTBacked
	}
	if _, ok := r.(interface {
		AggregateNodeTypes() []string
		CollectFlatNode(idx uint32, file *scanner.File)
		Finalize(file *scanner.File) []scanner.Finding
		Reset()
	}); ok {
		return PrecisionASTBacked
	}
	if _, ok := r.(interface {
		CheckLines(file *scanner.File) []scanner.Finding
	}); ok {
		return PrecisionHeuristicTextBacked
	}
	return PrecisionHeuristicTextBacked
}
