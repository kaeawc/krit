// Package base holds the rule scaffolding types embedded by every rule
// implementation: BaseRule (name/ruleset/severity/description metadata and
// the Finding emit-helper), FlatDispatchBase (marker for flat-dispatch
// rules), and LineBase (marker for line-pass rules).
//
// This package exists so domain-specific rule subpackages
// (internal/rules/<domain>/) can embed these types without importing the
// main rules package — which would create a cycle, since rules imports the
// subpackages for side-effect registration.
//
// Anything that lives outside the rule struct itself (config glue, exclude
// matching, registry plumbing) stays in the rules package, not here.
package base

import "github.com/kaeawc/krit/internal/scanner"

// BaseRule provides common fields embedded in every rule implementation.
// It carries the canonical name/ruleset/severity/description metadata and
// the Finding helper that rules use to construct emit-boundary
// scanner.Finding values.
//
// The name intentionally stutters as base.BaseRule — the parent rules
// package re-exports it via `type BaseRule = base.BaseRule`, and renaming
// here would break every embed and literal in the codebase (hundreds of
// sites) without changing the rule scaffolding semantics.
//
//nolint:revive // exported: stutter is intentional; see comment above.
type BaseRule struct {
	RuleName    string
	RuleSetName string
	Sev         string
	Desc        string
}

func (b BaseRule) Name() string        { return b.RuleName }
func (b BaseRule) Description() string { return b.Desc }
func (b BaseRule) RuleSet() string     { return b.RuleSetName }
func (b BaseRule) Severity() string    { return b.Sev }

// Finding constructs a scanner.Finding pre-populated with the rule's
// identity. Callers fill the message and any fix. line and col are
// 1-indexed.
func (b BaseRule) Finding(file *scanner.File, line, col int, msg string) scanner.Finding {
	return scanner.Finding{
		File:     file.Path,
		Line:     line,
		Col:      col,
		RuleSet:  b.RuleSetName,
		Rule:     b.RuleName,
		Severity: b.Sev,
		Message:  msg,
	}
}

// FlatDispatchBase is embedded by flat-dispatch rule implementations.
type FlatDispatchBase struct{}

// LineBase is embedded by line-rule implementations.
type LineBase struct{}
