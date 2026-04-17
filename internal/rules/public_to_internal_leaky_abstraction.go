package rules

import (
	"fmt"

	"github.com/kaeawc/krit/internal/arch"
	"github.com/kaeawc/krit/internal/scanner"
)

// PublicToInternalLeakyAbstractionRule detects public classes that wrap a
// single private/internal delegate and forward most methods to it. Such
// wrappers typically indicate the public type is a leaky facade over an
// internal implementation. Inactive by default.
type PublicToInternalLeakyAbstractionRule struct {
	LineBase
	BaseRule
	Threshold float64
}

// Confidence is 0.70 — the line-based heuristic (single-param constructor +
// single-expression method bodies delegating to the field) has real
// false-positive paths: adapters, type-safe wrappers, and DI holders all
// look similar. Medium confidence keeps reviewers honest.
func (r *PublicToInternalLeakyAbstractionRule) Confidence() float64 { return 0.70 }

func (r *PublicToInternalLeakyAbstractionRule) CheckLines(file *scanner.File) []scanner.Finding {
	leaks := arch.DetectLeakyAbstractions(file.Lines, r.Threshold)
	if len(leaks) == 0 {
		return nil
	}
	findings := make([]scanner.Finding, 0, len(leaks))
	for _, l := range leaks {
		findings = append(findings, scanner.Finding{
			File:     file.Path,
			Line:     l.Line,
			Col:      1,
			RuleSet:  r.RuleSetName,
			Rule:     r.RuleName,
			Severity: r.Sev,
			Message: fmt.Sprintf("Public class %s delegates %d of %d methods to internal field %q; consider exposing the internal type directly or adding real behavior.",
				l.ClassName, l.DelegatingMethods, l.TotalMethods, l.WrappedType),
		})
	}
	return findings
}
