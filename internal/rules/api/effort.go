package api

// Effort estimates the manual-fix difficulty of findings emitted by a
// rule. It is orthogonal to FixLevel — FixLevel describes auto-fix safety
// (whether krit can rewrite the source itself and how confident the
// rewrite is), while Effort describes how much human work the fix takes
// once a developer sits down with the report.
//
// Values are ordered from least to most work so callers can filter with
// comparisons (e.g. `triage --max-effort local` keeps EffortTrivial and
// EffortLocal, drops EffortRefactor and above). Zero (EffortUnset) means
// the rule has not declared an explicit value and consumers should fall
// back to the derived tier via V2RuleEffort.
type Effort uint8

const (
	// EffortUnset is the zero value. Consumers should derive a tier via
	// V2RuleEffort (rules package) when they see this value.
	EffortUnset Effort = iota
	// EffortTrivial: a single-line edit. Renames, missing modifier,
	// stray import, swapping a deprecated call for its replacement.
	EffortTrivial
	// EffortLocal: changes confined to one file. Reordering a few
	// statements, extracting a small helper, adjusting a local API.
	EffortLocal
	// EffortRefactor: the fix touches multiple files. Renaming a public
	// symbol, restructuring a layer boundary, splitting a module.
	EffortRefactor
	// EffortArchitectural: requires design discussion before the fix can
	// even be scoped. Architectural rules, layering violations, deep
	// coupling, policy decisions.
	EffortArchitectural
)

// String returns the stable, kebab-cased label for the effort tier.
// Used by CLI flags, MCP responses, SARIF properties, and config docs.
func (e Effort) String() string {
	switch e {
	case EffortTrivial:
		return "trivial"
	case EffortLocal:
		return "local"
	case EffortRefactor:
		return "refactor"
	case EffortArchitectural:
		return "architectural"
	default:
		return "unset"
	}
}

// ParseEffort returns the Effort matching the given label. The labels
// accepted are exactly the canonical strings emitted by String(), keeping
// CLI flags and the parser in lockstep. Unknown labels (and the empty
// string) return EffortUnset, false.
func ParseEffort(s string) (Effort, bool) {
	switch s {
	case "trivial":
		return EffortTrivial, true
	case "local":
		return EffortLocal, true
	case "refactor":
		return EffortRefactor, true
	case "architectural":
		return EffortArchitectural, true
	default:
		return EffortUnset, false
	}
}

// EffortProvider lets tests stub an Effort value without constructing a
// full Rule. MetaForRule checks for this interface on Rule.Implementation
// before falling back to the derived value, mirroring PrecisionProvider.
type EffortProvider interface {
	Effort() Effort
}

// EffortClassifier produces an Effort for a Rule. The production
// implementation lives in package rules (V2RuleEffort); tests can pass a
// fake to exercise filtering and reporting without depending on the
// derivation heuristics.
type EffortClassifier interface {
	Classify(r *Rule) Effort
}
