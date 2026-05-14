package api

// Precision is a rule's evidence tier — the dominant signal class the
// rule uses to draw conclusions. It is the single most useful
// signal-to-noise axis: heuristic / text-backed rules tend to be the
// noisiest, AST-backed and project-structure rules trade more context
// for more precision, type-aware rules consume resolver/oracle facts,
// and policy rules report opinions independent of source content.
//
// Values are ordered from noisiest to cleanest so callers can filter
// with comparisons (e.g. "default-active should be Precision >= PrecisionASTBacked").
// Zero (PrecisionUnset) means the rule has not declared a precision tier
// and consumers should derive one — the dispatcher's V2RulePrecision
// helper does this via Needs / NodeTypes / known overrides.
type Precision uint8

const (
	// PrecisionUnset is the zero value. The rule has not declared an
	// explicit tier; derive one from rule shape via V2RulePrecision.
	PrecisionUnset Precision = iota
	// PrecisionHeuristicTextBacked covers regex / lexical scanning rules
	// without AST or type evidence. Highest noise tier.
	PrecisionHeuristicTextBacked
	// PrecisionASTBacked covers rules that base findings on tree-sitter
	// AST structure for the current file.
	PrecisionASTBacked
	// PrecisionProjectStructure covers rules that consult project-wide
	// indexes (cross-file, module graph, manifest, resources, Gradle).
	PrecisionProjectStructure
	// PrecisionTypeAware covers rules that consume resolver or oracle
	// type information.
	PrecisionTypeAware
	// PrecisionPolicy covers rules that report opinions independent of
	// the source under analysis (SDK version policy, version freshness).
	PrecisionPolicy
)

// String returns the stable, kebab-cased label for the precision tier.
// Used by CLI output, MCP responses, SARIF properties, and config docs.
// The labels match the historical string-typed Precision values so
// downstream consumers do not see a wire-format change.
func (p Precision) String() string {
	switch p {
	case PrecisionHeuristicTextBacked:
		return "heuristic/text-backed"
	case PrecisionASTBacked:
		return "ast-backed"
	case PrecisionProjectStructure:
		return "project-structure-aware"
	case PrecisionTypeAware:
		return "type-aware"
	case PrecisionPolicy:
		return "policy"
	default:
		return "unset"
	}
}

// ParsePrecision returns the Precision matching the given label. The
// labels accepted are exactly the canonical strings emitted by String(),
// keeping the MCP schema enum and the parser in lockstep. Unknown labels
// (and the empty string) return PrecisionUnset, false.
func ParsePrecision(s string) (Precision, bool) {
	switch s {
	case "heuristic/text-backed":
		return PrecisionHeuristicTextBacked, true
	case "ast-backed":
		return PrecisionASTBacked, true
	case "project-structure-aware":
		return PrecisionProjectStructure, true
	case "type-aware":
		return PrecisionTypeAware, true
	case "policy":
		return PrecisionPolicy, true
	default:
		return PrecisionUnset, false
	}
}

// PrecisionProvider lets tests stub a precision value without
// constructing a full Rule. The dispatcher and MetaForRule check for
// this interface on Rule.Implementation before falling back to the
// derived value.
type PrecisionProvider interface {
	Precision() Precision
}
